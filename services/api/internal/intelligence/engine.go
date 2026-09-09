package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
	"github.com/thedobra/thedobra/services/api/internal/schemax"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

type Engine struct {
	pg    *pgxpool.Pool
	query *queryeng.Engine
}

func New(pg *pgxpool.Pool, q *queryeng.Engine) *Engine {
	return &Engine{pg: pg, query: q}
}

type Brief struct {
	Headline      string    `json:"headline"`
	GeneratedAt   string    `json:"generated_at"`
	MajorChanges  []Insight `json:"major_changes"`
	Risks         []Insight `json:"risks"`
	Opportunities []Insight `json:"opportunities"`
	Actions       []string  `json:"recommended_actions"`
	DatasetID     string    `json:"dataset_id,omitempty"`
}

type Insight struct {
	Kind     string         `json:"kind"`
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	Severity string         `json:"severity"`
	Evidence map[string]any `json:"evidence"`
}

func (e *Engine) AnalyzeDataset(ctx context.Context, orgID, wsID, userID, datasetID uuid.UUID, role string) (Brief, error) {
	var modelJSON, schemaJSON []byte
	var name string
	err := e.pg.QueryRow(ctx, `
		SELECT d.name, COALESCE(s.model_json, '{}'::jsonb), COALESCE(d.schema_json, '[]'::jsonb)
		FROM datasets d
		LEFT JOIN semantic_models s ON s.dataset_id = d.id
		WHERE d.id=$1 AND d.org_id=$2 AND d.workspace_id=$3
	`, datasetID, orgID, wsID).Scan(&name, &modelJSON, &schemaJSON)
	if err != nil {
		return Brief{}, fmt.Errorf("conjunto ainda não está pronto para análise")
	}
	var model semantic.Model
	_ = json.Unmarshal(modelJSON, &model)
	if len(model.Measures) == 0 {
		var cols []schemax.Column
		_ = json.Unmarshal(schemaJSON, &cols)
		if len(cols) > 0 {
			model = semantic.Suggest(name, cols)
		}
	}
	measure := semantic.PrimaryMeasure(model)
	if measure == "" {
		return Brief{}, fmt.Errorf("não há métrica no modelo para analisar")
	}
	timeDim := pickTimeDimension(model)
	catDims := pickCategoryDimensions(model, timeDim, 3)
	dsID := datasetID.String()
	now := time.Now().UTC()
	brief := Brief{GeneratedAt: now.Format(time.RFC3339), DatasetID: dsID}

	var curV, prevV, delta float64
	var trendFromSeries bool
	if timeDim != "" {
		series, err := e.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
			DatasetID: dsID, Measures: []string{measure}, Dimensions: []string{timeDim}, Limit: 36,
		})
		if err != nil {
			return Brief{}, err
		}
		vals := seriesValues(series.Rows, measure)
		if len(vals) >= 2 {
			trendFromSeries = true
			prevV, curV = vals[len(vals)-2], vals[len(vals)-1]
			if prevV != 0 {
				delta = (curV - prevV) / math.Abs(prevV) * 100
			}
			n := min(3, len(vals)/2)
			if n >= 1 && len(vals) >= n*2 {
				var recent, older float64
				for i := 0; i < n; i++ {
					recent += vals[len(vals)-1-i]
					older += vals[len(vals)-1-n-i]
				}
				if older != 0 {
					delta = (recent - older) / math.Abs(older) * 100
					curV, prevV = recent, older
				}
			}
		}
	}
	if !trendFromSeries {
		tot, err := e.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
			DatasetID: dsID, Measures: []string{measure}, Limit: 1,
		})
		if err != nil {
			return Brief{}, err
		}
		curV = measureValue(first(tot.Rows), measure)
	}

	if trendFromSeries {
		if delta < 0 {
			brief.Headline = fmt.Sprintf("Analisei %s. %s caiu %.1f%% no recorte recente da série temporal.", name, measure, math.Abs(delta))
		} else {
			brief.Headline = fmt.Sprintf("Analisei %s. %s subiu %.1f%% no recorte recente da série temporal.", name, measure, delta)
		}
		brief.MajorChanges = append(brief.MajorChanges, Insight{
			Kind: "trend", Severity: sev(delta),
			Title:    fmt.Sprintf("%s mudou %.1f%%", measure, delta),
			Body:     fmt.Sprintf("Período recente: %s vs. período anterior: %s.", fmtNum(curV), fmtNum(prevV)),
			Evidence: map[string]any{"metric": measure, "current": curV, "previous": prevV, "delta_pct": delta, "dimension": timeDim},
		})
	} else {
		brief.Headline = fmt.Sprintf("Analisei %s. %s totaliza %s no conjunto.", name, measure, fmtNum(curV))
		brief.MajorChanges = append(brief.MajorChanges, Insight{
			Kind: "trend", Severity: "info",
			Title:    fmt.Sprintf("%s está em %s", measure, fmtNum(curV)),
			Body:     "Não há dimensão de tempo fiável neste modelo, por isso a leitura é o total visível — não uma variação de período.",
			Evidence: map[string]any{"metric": measure, "current": curV},
		})
	}

	for _, d := range catDims {
		rows, err := e.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
			DatasetID: dsID, Measures: []string{measure}, Dimensions: []string{d}, Limit: 40,
		})
		if err != nil || len(rows.Rows) == 0 {
			continue
		}
		var total float64
		type pair struct {
			Val string
			Cur float64
		}
		var parts []pair
		for _, r := range rows.Rows {
			k := dimValue(r, d)
			if k == "" {
				continue
			}
			cv := measureValue(r, measure)
			parts = append(parts, pair{k, cv})
			total += cv
		}
		sort.Slice(parts, func(i, j int) bool { return parts[i].Cur > parts[j].Cur })
		if total != 0 && len(parts) > 0 {
			top := parts[0]
			n := min(5, len(parts))
			var topN float64
			for i := 0; i < n; i++ {
				topN += parts[i].Cur
			}
			share := topN / math.Abs(total) * 100
			if share >= 55 && len(parts) >= 3 {
				brief.Risks = append(brief.Risks, Insight{
					Kind: "risk", Severity: "warn",
					Title:    fmt.Sprintf("%s concentra %.0f%% nos %d maiores valores de %s", measure, share, n, d),
					Body:     fmt.Sprintf("A fatia maior é «%s» com %s. Vale confirmar se essa dependência é intencional.", top.Val, fmtNum(top.Cur)),
					Evidence: map[string]any{"dimension": d, "value": top.Val, "share_pct": share, "metric": measure},
				})
			}
			if len(parts) >= 2 {
				best, worst := parts[0], parts[len(parts)-1]
				brief.Opportunities = append(brief.Opportunities, Insight{
					Kind: "opportunity", Severity: "info",
					Title:    fmt.Sprintf("%s lidera em %s", best.Val, d),
					Body:     fmt.Sprintf("«%s» soma %s. O menor valor visível é «%s» (%s) — compare mix, preço e volume.", best.Val, fmtNum(best.Cur), worst.Val, fmtNum(worst.Cur)),
					Evidence: map[string]any{"dimension": d, "leader": best.Val, "laggard": worst.Val},
				})
			}
		}
	}

	if len(brief.Risks) == 0 && trendFromSeries && delta < -10 {
		brief.Risks = append(brief.Risks, Insight{
			Kind: "risk", Severity: "warn",
			Title:    fmt.Sprintf("A queda de %s pode persistir", measure),
			Body:     "A variação recente da série é grande o suficiente para revisão de volume, preço e mix.",
			Evidence: map[string]any{"delta_pct": delta},
		})
	}

	actions := []string{}
	if trendFromSeries && delta < 0 {
		actions = append(actions, "Abra a série temporal e confirme se a queda é de volume, preço ou mix.")
	}
	if len(brief.Opportunities) > 0 {
		actions = append(actions, "Compare as fatias que lideram com as que ficam atrás e replique o que funciona.")
	}
	if len(brief.Risks) > 0 {
		actions = append(actions, "Reduza concentração: não deixe o resultado depender de poucas categorias.")
	}
	actions = append(actions, "Crie um alerta se "+measure+" variar mais de 10% entre períodos.")
	brief.Actions = actions

	e.persist(ctx, orgID, wsID, datasetID, brief)
	return brief, nil
}

func (e *Engine) persist(ctx context.Context, orgID, wsID, datasetID uuid.UUID, brief Brief) {
	all := append(append(brief.MajorChanges, brief.Risks...), brief.Opportunities...)
	for _, in := range all {
		ev, _ := json.Marshal(in.Evidence)
		_, _ = e.pg.Exec(ctx, `
			INSERT INTO insights (org_id, workspace_id, dataset_id, kind, title, body, evidence_json, severity)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		`, orgID, wsID, datasetID, in.Kind, in.Title, in.Body, ev, in.Severity)
	}
}

func (e *Engine) List(ctx context.Context, orgID, wsID uuid.UUID) ([]Insight, error) {
	rows, err := e.pg.Query(ctx, `
		SELECT kind, title, body, severity, evidence_json FROM insights
		WHERE org_id=$1 AND workspace_id=$2 ORDER BY created_at DESC LIMIT 50
	`, orgID, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Insight
	for rows.Next() {
		var in Insight
		var ev []byte
		if err := rows.Scan(&in.Kind, &in.Title, &in.Body, &in.Severity, &ev); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(ev, &in.Evidence)
		out = append(out, in)
	}
	return out, rows.Err()
}

func first(rows []map[string]any) map[string]any {
	if len(rows) == 0 {
		return map[string]any{}
	}
	return rows[0]
}

func num(row map[string]any, key string) float64 {
	v, ok := row[key]
	if !ok {
		for _, x := range row {
			return toF(x)
		}
		return 0
	}
	return toF(v)
}

func toF(v any) float64 {
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

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func alias(s string) string {
	return semanticAlias(s)
}

func semanticAlias(s string) string {
	out := ""
	for _, r := range s {
		if r == ' ' {
			out += "_"
		} else if r >= 'A' && r <= 'Z' {
			out += string(r + 32)
		} else {
			out += string(r)
		}
	}
	return out
}

func foldIdent(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a",
		"é", "e", "ê", "e", "í", "i",
		"ó", "o", "ô", "o", "õ", "o",
		"ú", "u", "ç", "c",
		" ", "_", "-", "_",
	)
	return repl.Replace(s)
}

func dimLooksTime(d semantic.Dimension, timeCol string) bool {
	t := strings.ToLower(strings.TrimSpace(d.Type))
	if t == "date" || t == "datetime" || t == "timestamp" {
		return true
	}
	if timeCol != "" && (strings.EqualFold(d.Column, timeCol) || strings.EqualFold(d.Name, timeCol)) {
		return true
	}
	blob := strings.ToUpper(d.Expression)
	if strings.Contains(blob, "TOMONTH") || strings.Contains(blob, "YEARMONTH") || strings.Contains(blob, "TODATE") || strings.Contains(blob, "YEAR(") {
		return true
	}
	parts := strings.FieldsFunc(foldIdent(d.Name+"_"+d.Column), func(r rune) bool { return r == '_' })
	for _, p := range parts {
		switch p {
		case "date", "data", "datetime", "mes", "month", "ano", "year", "dia", "day", "semana", "week":
			return true
		}
	}
	return false
}

func pickTimeDimension(model semantic.Model) string {
	if model.TimeColumn != "" {
		if d, ok := semantic.ResolveDimension(model, model.TimeColumn); ok {
			if d.Name != "" {
				return d.Name
			}
			return d.Column
		}
		return model.TimeColumn
	}
	for _, d := range model.Dimensions {
		if dimLooksTime(d, model.TimeColumn) {
			if d.Name != "" {
				return d.Name
			}
			return d.Column
		}
	}
	return ""
}

func pickCategoryDimensions(model semantic.Model, timeDim string, maxN int) []string {
	out := make([]string, 0, maxN)
	for _, d := range model.Dimensions {
		key := d.Name
		if key == "" {
			key = d.Column
		}
		if key == "" || strings.EqualFold(key, timeDim) || dimLooksTime(d, model.TimeColumn) {
			continue
		}
		out = append(out, key)
		if len(out) >= maxN {
			break
		}
	}
	return out
}

func measureValue(row map[string]any, measure string) float64 {
	if row == nil || measure == "" {
		return 0
	}
	if v, ok := row[measure]; ok {
		return toF(v)
	}
	want := foldIdent(measure)
	for k, v := range row {
		if foldIdent(k) == want {
			return toF(v)
		}
	}
	return 0
}

func dimValue(row map[string]any, dim string) string {
	if row == nil {
		return ""
	}
	if v, ok := row[dim]; ok {
		return strings.TrimSpace(str(v))
	}
	want := foldIdent(dim)
	for k, v := range row {
		if foldIdent(k) == want {
			return strings.TrimSpace(str(v))
		}
	}
	return ""
}

func seriesValues(rows []map[string]any, measure string) []float64 {
	out := make([]float64, 0, len(rows))
	for _, r := range rows {
		out = append(out, measureValue(r, measure))
	}
	return out
}

func fmtNum(v float64) string {
	if math.Abs(v) >= 1000 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func sev(delta float64) string {
	if delta <= -15 {
		return "critical"
	}
	if delta < 0 {
		return "warn"
	}
	return "info"
}
