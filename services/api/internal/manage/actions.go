package manage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
)

func (s *Service) ListActions(ctx context.Context, orgID, wsID uuid.UUID, status string) ([]Action, error) {
	q := `
		SELECT a.id, COALESCE(a.discovery_id::text,''), COALESCE(a.dataset_id::text,''),
			COALESCE(a.owner_id::text,''), COALESCE(NULLIF(a.owner_name,''), COALESCE(u.name,'')),
			a.title, a.body, a.measure, a.expected_result, a.baseline_json, a.status,
			a.due_at::text, a.review_at::text, a.created_at, a.updated_at
		FROM manage_actions a
		LEFT JOIN users u ON u.id = a.owner_id
		WHERE a.org_id=$1 AND a.workspace_id=$2
	`
	args := []any{orgID, wsID}
	if status != "" && status != "all" {
		q += ` AND a.status=$3`
		args = append(args, status)
	}
	q += ` ORDER BY COALESCE(a.review_at, a.due_at) ASC NULLS LAST, a.created_at DESC LIMIT 80`
	rows, err := s.pg.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Action{}
	today := formatDate(nowBR())
	for rows.Next() {
		a, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		a.ReviewDue = a.ReviewAt != "" && a.ReviewAt <= today && (a.Status == "open" || a.Status == "in_progress")
		a.DueSoon = a.DueAt != "" && a.DueAt <= today && a.Status != "done" && a.Status != "cancelled"
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Service) GetAction(ctx context.Context, orgID, wsID, id uuid.UUID) (Action, []ActionReview, error) {
	row := s.pg.QueryRow(ctx, `
		SELECT a.id, COALESCE(a.discovery_id::text,''), COALESCE(a.dataset_id::text,''),
			COALESCE(a.owner_id::text,''), COALESCE(NULLIF(a.owner_name,''), COALESCE(u.name,'')),
			a.title, a.body, a.measure, a.expected_result, a.baseline_json, a.status,
			a.due_at::text, a.review_at::text, a.created_at, a.updated_at
		FROM manage_actions a
		LEFT JOIN users u ON u.id = a.owner_id
		WHERE a.id=$1 AND a.org_id=$2 AND a.workspace_id=$3
	`, id, orgID, wsID)
	a, err := scanAction(row)
	if err != nil {
		return Action{}, nil, fmt.Errorf("acção não encontrada")
	}
	reviews, _ := s.listReviews(ctx, orgID, wsID, id)
	if len(reviews) > 0 {
		latest := reviews[0]
		a.LatestReview = &latest
	}
	return a, reviews, nil
}

func scanAction(row rowScanner) (Action, error) {
	var a Action
	var due, review *string
	var ev []byte
	if err := row.Scan(&a.ID, &a.DiscoveryID, &a.DatasetID, &a.OwnerID, &a.OwnerName, &a.Title, &a.Body, &a.Measure, &a.ExpectedResult, &ev, &a.Status, &due, &review, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return a, err
	}
	_ = json.Unmarshal(ev, &a.Baseline)
	if due != nil {
		a.DueAt = strings.TrimSpace(*due)
	}
	if review != nil {
		a.ReviewAt = strings.TrimSpace(*review)
	}
	return a, nil
}

func (s *Service) listOpenAndDue(ctx context.Context, orgID, wsID uuid.UUID) (int, []Action, error) {
	all, err := s.ListActions(ctx, orgID, wsID, "all")
	if err != nil {
		return 0, nil, err
	}
	open := 0
	var due []Action
	for _, a := range all {
		if a.Status == "open" || a.Status == "in_progress" {
			open++
		}
		if a.ReviewDue {
			due = append(due, a)
		}
	}
	return open, due, nil
}

func (s *Service) CreateAction(ctx context.Context, orgID, wsID, userID uuid.UUID, role string, in CreateActionInput) (Action, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return Action{}, fmt.Errorf("a acção precisa de um título")
	}
	id := uuid.New()
	var discovery, dataset, owner any
	if in.DiscoveryID != "" {
		parsed, err := uuid.Parse(in.DiscoveryID)
		if err != nil {
			return Action{}, fmt.Errorf("descoberta inválida")
		}
		discovery = parsed
	}
	if in.DatasetID != "" {
		parsed, err := uuid.Parse(in.DatasetID)
		if err != nil {
			return Action{}, fmt.Errorf("conjunto inválido")
		}
		dataset = parsed
	}
	if in.OwnerID != "" {
		parsed, err := uuid.Parse(in.OwnerID)
		if err != nil {
			return Action{}, fmt.Errorf("responsável inválido")
		}
		owner = parsed
		if in.OwnerName == "" {
			_ = s.pg.QueryRow(ctx, `SELECT name FROM users WHERE id=$1`, parsed).Scan(&in.OwnerName)
		}
	}
	due, _ := parseDate(in.DueAt)
	review, ok := parseDate(in.ReviewAt)
	if !ok && !due.IsZero() {
		review = due.AddDate(0, 0, 7)
	}
	var dueVal, reviewVal any
	if !due.IsZero() {
		dueVal = dateOnly(due)
	}
	if !review.IsZero() {
		reviewVal = dateOnly(review)
	}
	baseline := map[string]any{"captured_at": time.Now().UTC().Format(time.RFC3339)}
	if in.DatasetID != "" && in.Measure != "" {
		if v, err := s.measureNow(ctx, orgID, wsID, userID, role, in.DatasetID, in.Measure); err == nil {
			baseline["value"] = v
			baseline["measure"] = in.Measure
		}
	}
	raw, _ := json.Marshal(baseline)
	_, err := s.pg.Exec(ctx, `
		INSERT INTO manage_actions
			(id, org_id, workspace_id, discovery_id, dataset_id, created_by, owner_id, owner_name,
			 title, body, measure, expected_result, baseline_json, status, due_at, review_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'open',$14,$15)
	`, id, orgID, wsID, discovery, dataset, userID, owner, in.OwnerName, in.Title, in.Body, in.Measure, in.ExpectedResult, raw, dueVal, reviewVal)
	if err != nil {
		return Action{}, err
	}
	_ = s.notifyUsers(ctx, orgID, wsID, "action", "Acção registada: "+in.Title,
		"Acompanhe o resultado na data de revisão.", "/actions", 2)
	a, _, err := s.GetAction(ctx, orgID, wsID, id)
	return a, err
}

func (s *Service) PatchAction(ctx context.Context, orgID, wsID, id uuid.UUID, status string) error {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "open", "in_progress", "reviewed", "done", "cancelled":
	default:
		return fmt.Errorf("estado inválido")
	}
	ct, err := s.pg.Exec(ctx, `UPDATE manage_actions SET status=$1, updated_at=now() WHERE id=$2 AND org_id=$3 AND workspace_id=$4`, status, id, orgID, wsID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("acção não encontrada")
	}
	return nil
}

func (s *Service) ReviewAction(ctx context.Context, orgID, wsID, userID uuid.UUID, role string, actionID uuid.UUID, in ReviewActionInput) (ActionReview, error) {
	a, _, err := s.GetAction(ctx, orgID, wsID, actionID)
	if err != nil {
		return ActionReview{}, err
	}
	var current *float64
	var delta *float64
	indicator := "Não há indicador ligado a esta acção, por isso a revisão cobre só o que foi executado."
	if a.DatasetID != "" && a.Measure != "" {
		if v, qerr := s.measureNow(ctx, orgID, wsID, userID, role, a.DatasetID, a.Measure); qerr == nil {
			current = &v
			base := 0.0
			hasBase := false
			if a.Baseline != nil {
				switch x := a.Baseline["value"].(type) {
				case float64:
					base, hasBase = x, true
				}
			}
			if hasBase && base != 0 {
				d := (v - base) / abs(base) * 100
				delta = &d
				indicator = fmt.Sprintf("O indicador %s passou de %s para %s (%.1f%%).", a.Measure, formatNum(base), formatNum(v), d)
			} else {
				indicator = fmt.Sprintf("O indicador %s está em %s neste momento.", a.Measure, formatNum(v))
			}
		} else {
			indicator = "Não foi possível ler o indicador agora. A evolução fica por confirmar na próxima actualização dos dados."
		}
	}
	effect := "Isto é a evolução observada do indicador no período. Não prova, por si só, que a acção causou a mudança. Use a nota de execução para distinguir o que foi feito do que os números mostram."
	from, to := nowBR().AddDate(0, 0, -21), nowBR()
	if mems := s.memoriesAround(ctx, orgID, wsID, from, to); len(mems) > 0 {
		effect += " Memória de gestão no período: "
		for i, m := range mems {
			if i > 0 {
				effect += " · "
			}
			effect += m.OccurredOn + " — " + m.Note
		}
	}
	id := uuid.New()
	_, err = s.pg.Exec(ctx, `
		INSERT INTO manage_action_reviews
			(id, org_id, workspace_id, action_id, created_by, executed_note, indicator_value, indicator_delta_pct, indicator_narrative, effect_narrative)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, id, orgID, wsID, actionID, userID, strings.TrimSpace(in.ExecutedNote), current, delta, indicator, effect)
	if err != nil {
		return ActionReview{}, err
	}
	_, _ = s.pg.Exec(ctx, `UPDATE manage_actions SET status='reviewed', updated_at=now() WHERE id=$1`, actionID)
	rev := ActionReview{
		ID: id.String(), ActionID: actionID.String(), ExecutedNote: in.ExecutedNote,
		IndicatorValue: current, IndicatorDeltaPct: delta,
		IndicatorNarrative: indicator, EffectNarrative: effect, CreatedAt: time.Now(),
	}
	return rev, nil
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func (s *Service) listReviews(ctx context.Context, orgID, wsID, actionID uuid.UUID) ([]ActionReview, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id, action_id::text, executed_note, indicator_value, indicator_delta_pct, indicator_narrative, effect_narrative, created_at
		FROM manage_action_reviews
		WHERE org_id=$1 AND workspace_id=$2 AND action_id=$3
		ORDER BY created_at DESC
	`, orgID, wsID, actionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ActionReview{}
	for rows.Next() {
		var r ActionReview
		if err := rows.Scan(&r.ID, &r.ActionID, &r.ExecutedNote, &r.IndicatorValue, &r.IndicatorDeltaPct, &r.IndicatorNarrative, &r.EffectNarrative, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) measureNow(ctx context.Context, orgID, wsID, userID uuid.UUID, role, datasetID, measure string) (float64, error) {
	res, err := s.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
		DatasetID: datasetID, Measures: []string{measure}, Limit: 1,
	})
	if err != nil {
		return 0, err
	}
	if len(res.Rows) == 0 {
		return 0, fmt.Errorf("sem valor")
	}
	row := res.Rows[0]
	if v, ok := row[measure]; ok {
		return toFloat(v), nil
	}
	for _, v := range row {
		return toFloat(v), nil
	}
	return 0, fmt.Errorf("sem valor")
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		var f float64
		fmt.Sscanf(t, "%f", &f)
		return f
	default:
		return 0
	}
}
