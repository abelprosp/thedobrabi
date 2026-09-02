package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
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
	Headline      string     `json:"headline"`
	GeneratedAt   string     `json:"generated_at"`
	MajorChanges  []Insight  `json:"major_changes"`
	Risks         []Insight  `json:"risks"`
	Opportunities []Insight  `json:"opportunities"`
	Actions       []string   `json:"recommended_actions"`
	DatasetID     string     `json:"dataset_id,omitempty"`
}

type Insight struct {
	Kind     string         `json:"kind"`
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	Severity string         `json:"severity"`
	Evidence map[string]any `json:"evidence"`
}

func (e *Engine) AnalyzeDataset(ctx context.Context, orgID, wsID, userID, datasetID uuid.UUID, role string) (Brief, error) {
	var modelJSON []byte
	var name string
	err := e.pg.QueryRow(ctx, `
		SELECT d.name, s.model_json FROM datasets d
		JOIN semantic_models s ON s.dataset_id = d.id
		WHERE d.id=$1 AND d.org_id=$2 AND d.workspace_id=$3
	`, datasetID, orgID, wsID).Scan(&name, &modelJSON)
	if err != nil {
		return Brief{}, fmt.Errorf("conjunto ainda não está pronto para análise")
	}
	var model semantic.Model
	_ = json.Unmarshal(modelJSON, &model)

	measure := "revenue"
	if _, ok := semantic.ResolveMeasure(model, measure); !ok {
		if len(model.Measures) == 0 {
			return Brief{}, fmt.Errorf("não há dados suficientes para analisar com fiabilidade")
		}
		measure = model.Measures[0].Name
	}

	now := time.Now().UTC()
	curStart := now.AddDate(0, 0, -30).Format("2006-01-02")
	curEnd := now.Format("2006-01-02")
	prevStart := now.AddDate(0, 0, -60).Format("2006-01-02")
	prevEnd := curStart

	cur, err := e.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
		DatasetID: datasetID.String(), Measures: []string{measure}, Limit: 1,
		TimeRange: &queryeng.TimeRange{Start: curStart, End: curEnd},
	})
	if err != nil {
		return Brief{}, err
	}
	prev, err := e.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
		DatasetID: datasetID.String(), Measures: []string{measure}, Limit: 1,
		TimeRange: &queryeng.TimeRange{Start: prevStart, End: prevEnd},
	})
	if err != nil {
		return Brief{}, err
	}

	curV := num(first(cur.Rows), alias(measure))
	prevV := num(first(prev.Rows), alias(measure))
	delta := 0.0
	if prevV != 0 {
		delta = (curV - prevV) / prevV * 100
	}

	brief := Brief{
		GeneratedAt: now.Format(time.RFC3339),
		DatasetID:   datasetID.String(),
	}
	if delta < 0 {
		brief.Headline = fmt.Sprintf("Analisei %s. %s caiu %.1f%% nos últimos 30 dias.", name, measure, math.Abs(delta))
	} else {
		brief.Headline = fmt.Sprintf("Analisei %s. %s subiu %.1f%% nos últimos 30 dias.", name, measure, delta)
	}

	brief.MajorChanges = append(brief.MajorChanges, Insight{
		Kind: "trend", Severity: sev(delta),
		Title: fmt.Sprintf("%s mudou %.1f%%", measure, delta),
		Body:  fmt.Sprintf("Últimos 30 dias: %.2f vs. 30 dias anteriores: %.2f.", curV, prevV),
		Evidence: map[string]any{"metric": measure, "current": curV, "previous": prevV, "delta_pct": delta, "period": curStart + " → " + curEnd},
	})

	dims := []string{"region", "product", "segment", "channel"}
	var drivers []struct {
		Dim, Val string
		Delta    float64
		Cur      float64
	}
	for _, d := range dims {
		if _, ok := semantic.ResolveDimension(model, d); !ok {
			continue
		}
		crows, err := e.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
			DatasetID: datasetID.String(), Measures: []string{measure}, Dimensions: []string{d}, Limit: 50,
			TimeRange: &queryeng.TimeRange{Start: curStart, End: curEnd},
		})
		if err != nil {
			continue
		}
		prows, err := e.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
			DatasetID: datasetID.String(), Measures: []string{measure}, Dimensions: []string{d}, Limit: 50,
			TimeRange: &queryeng.TimeRange{Start: prevStart, End: prevEnd},
		})
		if err != nil {
			continue
		}
		prevMap := map[string]float64{}
		for _, r := range prows.Rows {
			prevMap[str(r[d])] = num(r, alias(measure))
		}
		for _, r := range crows.Rows {
			k := str(r[d])
			cv := num(r, alias(measure))
			pv := prevMap[k]
			if pv == 0 {
				continue
			}
			dlt := (cv - pv) / pv * 100
			drivers = append(drivers, struct {
				Dim, Val string
				Delta    float64
				Cur      float64
			}{d, k, dlt, cv})
		}
	}
	sort.Slice(drivers, func(i, j int) bool { return math.Abs(drivers[i].Delta) > math.Abs(drivers[j].Delta) })
	for i, d := range drivers {
		if i >= 3 {
			break
		}
		kind := "anomaly"
		if d.Delta > 0 {
			kind = "opportunity"
			brief.Opportunities = append(brief.Opportunities, Insight{
				Kind: kind, Severity: "info",
				Title: fmt.Sprintf("%s · %s cresceu %.1f%%", d.Dim, d.Val, d.Delta),
				Body:  fmt.Sprintf("Esta fatia contribui agora com %.2f de %s.", d.Cur, measure),
				Evidence: map[string]any{"dimension": d.Dim, "value": d.Val, "delta_pct": d.Delta, "current": d.Cur},
			})
		} else {
			brief.MajorChanges = append(brief.MajorChanges, Insight{
				Kind: kind, Severity: "warn",
				Title: fmt.Sprintf("%s · %s caiu %.1f%%", d.Dim, d.Val, math.Abs(d.Delta)),
				Body:  fmt.Sprintf("Investigue preço, mix e churn em %s=%s.", d.Dim, d.Val),
				Evidence: map[string]any{"dimension": d.Dim, "value": d.Val, "delta_pct": d.Delta, "current": d.Cur},
			})
		}
	}

	if _, ok := semantic.ResolveDimension(model, "customer"); ok {
		cust, err := e.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
			DatasetID: datasetID.String(), Measures: []string{measure}, Dimensions: []string{"customer"}, Limit: 20,
			TimeRange: &queryeng.TimeRange{Start: curStart, End: curEnd},
		})
		if err == nil && len(cust.Rows) > 0 {
			var total, top float64
			n := min(5, len(cust.Rows))
			for i, r := range cust.Rows {
				v := num(r, alias(measure))
				total += v
				if i < n {
					top += v
				}
			}
			if total > 0 {
				share := top / total * 100
				if share >= 30 {
					brief.Risks = append(brief.Risks, Insight{
						Kind: "risk", Severity: "warn",
						Title: fmt.Sprintf("Os %d maiores clientes representam %.0f%% de %s", n, share, measure),
						Body:  "Risco de concentração: um número pequeno de clientes domina o volume.",
						Evidence: map[string]any{"share_pct": share, "customers": n, "metric": measure},
					})
				}
			}
		}
	}

	if len(brief.Risks) == 0 && delta < -10 {
		brief.Risks = append(brief.Risks, Insight{
			Kind: "risk", Severity: "warn",
			Title: fmt.Sprintf("A queda de %s pode persistir", measure),
			Body:  "A variação de 30 dias é grande o suficiente para uma revisão executiva de pipeline e churn.",
			Evidence: map[string]any{"delta_pct": delta},
		})
	}

	actions := []string{}
	if delta < 0 {
		actions = append(actions, "Investigue as fatias com maior queda e confirme se é volume, preço ou mix.")
	}
	if len(brief.Opportunities) > 0 {
		actions = append(actions, "Dobre a aposta nas fatias que crescem: replique o playbook nas regiões em atraso.")
	}
	if len(brief.Risks) > 0 {
		actions = append(actions, "Reduza concentração: alargue o pipeline para além das maiores contas.")
	}
	actions = append(actions, "Crie um alerta de variação semanal de "+measure+" acima de 10%.")
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

func sev(delta float64) string {
	if delta <= -15 {
		return "critical"
	}
	if delta < 0 {
		return "warn"
	}
	return "info"
}
