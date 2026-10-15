package manage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thedobra/thedobra/services/api/internal/intelligence"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

type Service struct {
	pg    *pgxpool.Pool
	intel *intelligence.Engine
	query *queryeng.Engine
}

func New(pg *pgxpool.Pool, intel *intelligence.Engine, q *queryeng.Engine) *Service {
	return &Service{pg: pg, intel: intel, query: q}
}

func (s *Service) Home(ctx context.Context, orgID, wsID, userID uuid.UUID, role string) (Home, error) {
	out := Home{
		WhatChanged: []Discovery{},
		Attention:   []Discovery{},
		DueReviews:  []Action{},
		Memories:    []Memory{},
	}
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM datasets WHERE org_id=$1 AND workspace_id=$2 AND status='ready'`, orgID, wsID).Scan(&out.Datasets)

	cadenceMode, watch := s.cadenceSettings(ctx, wsID)
	out.CadenceMode = cadenceMode
	out.WatchMeasures = watch

	var dsID uuid.UUID
	var updated time.Time
	var modelJSON []byte
	err := s.pg.QueryRow(ctx, `
		SELECT d.id, d.updated_at, COALESCE(s.model_json, '{}'::jsonb)
		FROM datasets d
		LEFT JOIN semantic_models s ON s.dataset_id = d.id
		WHERE d.org_id=$1 AND d.workspace_id=$2 AND d.status='ready'
		ORDER BY d.updated_at DESC LIMIT 1
	`, orgID, wsID).Scan(&dsID, &updated, &modelJSON)
	hasTime := false
	if err == nil {
		out.DataUpdatedAt = &updated
		var model semantic.Model
		_ = json.Unmarshal(modelJSON, &model)
		hasTime = model.TimeColumn != "" || len(pickTime(model)) > 0
		brief, aerr := s.intel.AnalyzeDataset(ctx, orgID, wsID, userID, dsID, role)
		if aerr == nil {
			out.Brief = &brief
			_ = s.SyncFromBrief(ctx, orgID, wsID, dsID, brief)
		}
	}
	out.SuggestedCadence = suggestCadence(updated, hasTime)
	out.Cadence = cadenceMode
	if cadenceMode == "auto" {
		out.Cadence = out.SuggestedCadence
	}

	radar, _ := s.Radar(ctx, orgID, wsID)
	var material []Discovery
	for _, d := range radar {
		if d.Kind == "stable" {
			continue
		}
		if d.Kind == "trend" || d.Kind == "change" {
			out.WhatChanged = append(out.WhatChanged, d)
		}
		if d.Kind == "risk" || d.Kind == "opportunity" || d.Watching {
			out.Attention = append(out.Attention, d)
		}
		if d.ImpactScore >= 8 {
			material = append(material, d)
		}
	}
	if len(out.WhatChanged) == 0 && len(radar) > 0 {
		for _, d := range radar {
			if d.Kind != "stable" {
				out.WhatChanged = append(out.WhatChanged, d)
				break
			}
		}
	}
	if len(out.WhatChanged) > 3 {
		out.WhatChanged = out.WhatChanged[:3]
	}
	if len(out.Attention) > 4 {
		out.Attention = out.Attention[:4]
	}
	if len(material) == 0 {
		out.Quiet = true
		out.QuietMessage = "Nenhuma mudança relevante desde a última análise. Quando os dados se moverem, o radar volta a priorizar o que importa."
	} else {
		top := material[0]
		for _, d := range material {
			if d.ImpactScore > top.ImpactScore {
				top = d
			}
		}
		cp := top
		out.ActionNow = &cp
	}

	open, due, _ := s.listOpenAndDue(ctx, orgID, wsID)
	out.OpenActions = open
	out.DueReviews = due
	out.WeekReviewReady = s.weekReviewReady(ctx, orgID, wsID, out.Cadence)
	out.Memories, _ = s.ListMemories(ctx, orgID, wsID, 8)
	if len(out.Attention) > 0 {
		out.PastDecisions, _ = s.PastDecisions(ctx, orgID, wsID, out.Attention[0].Title)
	}
	_ = s.maybeNotify(ctx, orgID, wsID, out)
	return out, nil
}

func pickTime(m semantic.Model) string {
	if m.TimeColumn != "" {
		return m.TimeColumn
	}
	for _, d := range m.Dimensions {
		t := strings.ToLower(d.Type)
		if t == "date" || t == "datetime" || t == "timestamp" {
			return d.Name
		}
	}
	return ""
}

func (s *Service) cadenceSettings(ctx context.Context, wsID uuid.UUID) (string, []string) {
	var cadence string
	var raw []byte
	err := s.pg.QueryRow(ctx, `SELECT COALESCE(manage_cadence,'auto'), COALESCE(manage_watch_json,'{}'::jsonb) FROM workspaces WHERE id=$1`, wsID).Scan(&cadence, &raw)
	if err != nil {
		return "auto", nil
	}
	var wrap map[string]any
	_ = json.Unmarshal(raw, &wrap)
	var watch []string
	if arr, ok := wrap["measures"].([]any); ok {
		for _, x := range arr {
			if str, ok := x.(string); ok && str != "" {
				watch = append(watch, str)
			}
		}
	}
	if cadence == "" {
		cadence = "auto"
	}
	return cadence, watch
}

func (s *Service) SetCadence(ctx context.Context, orgID, wsID uuid.UUID, cadence string, watch []string) error {
	cadence = strings.ToLower(strings.TrimSpace(cadence))
	switch cadence {
	case "auto", "daily", "weekly", "monthly":
	default:
		return fmt.Errorf("frequência inválida")
	}
	if watch == nil {
		watch = []string{}
	}
	payload, _ := json.Marshal(map[string]any{"measures": watch})
	_, err := s.pg.Exec(ctx, `UPDATE workspaces SET manage_cadence=$1, manage_watch_json=$2, updated_at=now() WHERE id=$3 AND org_id=$4`, cadence, payload, wsID, orgID)
	return err
}

func fingerprint(kind, title string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(kind + "|" + title))))
	return hex.EncodeToString(h[:16])
}

func nextStep(kind, title, body string) string {
	blob := strings.ToLower(title + " " + body)
	switch {
	case strings.Contains(blob, "vendedor") || strings.Contains(blob, "carteira"):
		return "Comparar carteira e oportunidades"
	case strings.Contains(blob, "despesa") || strings.Contains(blob, "custo"):
		return "Investigar lançamentos"
	case strings.Contains(blob, "meta"):
		return "Calcular o ritmo necessário para alcançá-la"
	case strings.Contains(blob, "estoque") || strings.Contains(blob, "stock") || kind == "opportunity":
		return "Conferir estoque e disponibilidade"
	case kind == "risk":
		return "Investigar a causa e definir um responsável"
	default:
		return "Abrir a análise e decidir se vale uma acção"
	}
}

func impactOf(in intelligence.Insight) float64 {
	if in.Evidence != nil {
		for _, k := range []string{"delta_pct", "share_pct"} {
			switch v := in.Evidence[k].(type) {
			case float64:
				return math.Abs(v)
			case json.Number:
				f, _ := v.Float64()
				return math.Abs(f)
			}
		}
	}
	switch strings.ToLower(in.Severity) {
	case "high", "critical", "alta":
		return 80
	case "warn", "medium", "média", "media":
		return 50
	default:
		return 25
	}
}

func (s *Service) SyncFromBrief(ctx context.Context, orgID, wsID, datasetID uuid.UUID, brief intelligence.Brief) error {
	all := append(append([]intelligence.Insight{}, brief.MajorChanges...), brief.Risks...)
	all = append(all, brief.Opportunities...)
	maxImpact := 0.0
	for _, in := range all {
		kind := in.Kind
		if kind == "" {
			kind = "change"
		}
		if kind == "trend" {
			kind = "change"
		}
		score := impactOf(in)
		if score > maxImpact {
			maxImpact = score
		}
		ev, _ := json.Marshal(in.Evidence)
		fp := fingerprint(kind, in.Title)
		_, err := s.pg.Exec(ctx, `
			INSERT INTO manage_discoveries
				(org_id, workspace_id, dataset_id, kind, title, body, next_step, impact_score, evidence_json, fingerprint, last_seen_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now())
			ON CONFLICT (workspace_id, fingerprint) DO UPDATE SET
				body = EXCLUDED.body,
				next_step = EXCLUDED.next_step,
				impact_score = EXCLUDED.impact_score,
				evidence_json = EXCLUDED.evidence_json,
				dataset_id = EXCLUDED.dataset_id,
				last_seen_at = now()
		`, orgID, wsID, datasetID, kind, in.Title, in.Body, nextStep(kind, in.Title, in.Body), score, ev, fp)
		if err != nil {
			return err
		}
	}
	if maxImpact < 8 {
		title := "Nenhuma mudança relevante neste recorte"
		body := "Os dados não mostram um desvio com evidência suficiente para priorizar agora. Volte quando a próxima actualização chegar."
		fp := fingerprint("stable", "quiet")
		ev, _ := json.Marshal(map[string]any{"quiet": true})
		_, _ = s.pg.Exec(ctx, `
			INSERT INTO manage_discoveries
				(org_id, workspace_id, dataset_id, kind, title, body, next_step, impact_score, evidence_json, fingerprint, last_seen_at)
			VALUES ($1,$2,$3,'stable',$4,$5,'Manter o acompanhamento no ritmo dos dados',0,$6,$7,now())
			ON CONFLICT (workspace_id, fingerprint) DO UPDATE SET last_seen_at=now(), body=EXCLUDED.body, dataset_id=EXCLUDED.dataset_id
		`, orgID, wsID, datasetID, title, body, ev, fp)
	}
	return nil
}

func (s *Service) Radar(ctx context.Context, orgID, wsID uuid.UUID) ([]Discovery, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id, COALESCE(dataset_id::text,''), kind, title, body, next_step, impact_score, evidence_json,
			fingerprint, feedback, watching, created_at, last_seen_at
		FROM manage_discoveries
		WHERE org_id=$1 AND workspace_id=$2
			AND (feedback <> 'not_applicable' OR watching)
			AND last_seen_at > now() - interval '45 days'
		ORDER BY watching DESC, impact_score DESC, last_seen_at DESC
		LIMIT 40
	`, orgID, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Discovery{}
	for rows.Next() {
		d, err := scanDiscovery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDiscovery(row rowScanner) (Discovery, error) {
	var d Discovery
	var ev []byte
	if err := row.Scan(&d.ID, &d.DatasetID, &d.Kind, &d.Title, &d.Body, &d.NextStep, &d.ImpactScore, &ev, &d.Fingerprint, &d.Feedback, &d.Watching, &d.CreatedAt, &d.LastSeenAt); err != nil {
		return d, err
	}
	_ = json.Unmarshal(ev, &d.Evidence)
	return d, nil
}

func (s *Service) Feedback(ctx context.Context, orgID, wsID, id uuid.UUID, feedback string) error {
	feedback = strings.TrimSpace(strings.ToLower(feedback))
	switch feedback {
	case "useful", "not_applicable", "watch", "":
	default:
		return fmt.Errorf("resposta inválida")
	}
	watching := feedback == "watch"
	ct, err := s.pg.Exec(ctx, `
		UPDATE manage_discoveries
		SET feedback=$1, watching=CASE WHEN $2 THEN TRUE ELSE watching END
		WHERE id=$3 AND org_id=$4 AND workspace_id=$5
	`, feedback, watching, id, orgID, wsID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("descoberta não encontrada")
	}
	return nil
}

func (s *Service) ListMemories(ctx context.Context, orgID, wsID uuid.UUID, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id, COALESCE(dataset_id::text,''), note, chart_key, occurred_on::text, created_at
		FROM manage_memories
		WHERE org_id=$1 AND workspace_id=$2
		ORDER BY occurred_on DESC, created_at DESC
		LIMIT $3
	`, orgID, wsID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Memory{}
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.DatasetID, &m.Note, &m.ChartKey, &m.OccurredOn, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Service) CreateMemory(ctx context.Context, orgID, wsID, userID uuid.UUID, datasetID, note, chartKey, occurredOn string) (Memory, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		return Memory{}, fmt.Errorf("escreva o contexto da memória")
	}
	day := dateOnly(nowBR())
	if t, ok := parseDate(occurredOn); ok {
		day = t
	}
	var ds any
	if datasetID != "" {
		id, err := uuid.Parse(datasetID)
		if err != nil {
			return Memory{}, fmt.Errorf("conjunto inválido")
		}
		ds = id
	}
	id := uuid.New()
	_, err := s.pg.Exec(ctx, `
		INSERT INTO manage_memories (id, org_id, workspace_id, dataset_id, created_by, note, chart_key, occurred_on)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, id, orgID, wsID, ds, userID, note, chartKey, day)
	if err != nil {
		return Memory{}, err
	}
	return Memory{ID: id.String(), DatasetID: datasetID, Note: note, ChartKey: chartKey, OccurredOn: formatDate(day), CreatedAt: time.Now()}, nil
}

func (s *Service) DeleteMemory(ctx context.Context, orgID, wsID, id uuid.UUID) error {
	ct, err := s.pg.Exec(ctx, `DELETE FROM manage_memories WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, orgID, wsID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("memória não encontrada")
	}
	return nil
}

func (s *Service) memoriesAround(ctx context.Context, orgID, wsID uuid.UUID, from, to time.Time) []Memory {
	rows, err := s.pg.Query(ctx, `
		SELECT id, COALESCE(dataset_id::text,''), note, chart_key, occurred_on::text, created_at
		FROM manage_memories
		WHERE org_id=$1 AND workspace_id=$2 AND occurred_on BETWEEN $3 AND $4
		ORDER BY occurred_on
	`, orgID, wsID, dateOnly(from), dateOnly(to))
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		var m Memory
		if rows.Scan(&m.ID, &m.DatasetID, &m.Note, &m.ChartKey, &m.OccurredOn, &m.CreatedAt) == nil {
			out = append(out, m)
		}
	}
	return out
}
