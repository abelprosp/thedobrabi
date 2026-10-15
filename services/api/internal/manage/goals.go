package manage

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

func (s *Service) ListGoalPaces(ctx context.Context, orgID, wsID, userID uuid.UUID, role string) ([]GoalPace, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id, dataset_id, metric_name, name, target_value, period, COALESCE(owner_id::text,''), status,
			starts_at, ends_at
		FROM metric_goals
		WHERE org_id=$1 AND workspace_id=$2 AND status IN ('active','paused')
		ORDER BY updated_at DESC
		LIMIT 20
	`, orgID, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		id, dataset                         uuid.UUID
		metric, name, period, owner, status string
		target                              float64
		starts, ends                        *time.Time
	}
	var list []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.id, &x.dataset, &x.metric, &x.name, &x.target, &x.period, &x.owner, &x.status, &x.starts, &x.ends); err != nil {
			return nil, err
		}
		list = append(list, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	out := []GoalPace{}
	now := nowBR()
	for _, g := range list {
		start, end := periodBounds(now, g.period)
		if g.starts != nil {
			start = *g.starts
		}
		if g.ends != nil {
			end = *g.ends
		}
		currency := moneyish("", g.metric+" "+g.name)
		current := 0.0
		res, qerr := s.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
			DatasetID: g.dataset.String(),
			Measures:  []string{g.metric},
			TimeRange: &queryeng.TimeRange{Start: formatDate(start), End: formatDate(end)},
			Limit:     1,
		})
		if qerr != nil {
			res, qerr = s.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
				DatasetID: g.dataset.String(), Measures: []string{g.metric}, Limit: 1,
			})
		}
		if qerr == nil && len(res.Rows) > 0 {
			if v, ok := res.Rows[0][g.metric]; ok {
				current = toFloat(v)
			} else {
				for _, v := range res.Rows[0] {
					current = toFloat(v)
					break
				}
			}
		}
		var modelJSON []byte
		_ = s.pg.QueryRow(ctx, `SELECT COALESCE(model_json,'{}'::jsonb) FROM semantic_models WHERE dataset_id=$1`, g.dataset).Scan(&modelJSON)
		var model semantic.Model
		_ = json.Unmarshal(modelJSON, &model)
		if m, ok := semantic.ResolveMeasure(model, g.metric); ok {
			currency = moneyish(m.Format, m.Name)
		}
		days := BusinessDaysRemaining(now, end)
		pct, rem, daily, reached, narrative, hypo := paceNarrative(current, g.target, days, currency)
		out = append(out, GoalPace{
			ID: g.id.String(), Name: g.name, MetricName: g.metric, DatasetID: g.dataset.String(),
			TargetValue: g.target, CurrentValue: current, Remaining: rem, Pct: pct,
			Period: g.period, PeriodStart: formatDate(start), PeriodEnd: formatDate(end),
			RemainingDays: days, DailyPace: daily, Reached: reached, Currency: currency,
			Status: g.status, Narrative: narrative, Hypothesis: hypo, OwnerID: g.owner,
		})
	}
	return out, nil
}

func (s *Service) CreateGoal(ctx context.Context, orgID, wsID, userID uuid.UUID, datasetID, metric, name string, target float64, period, ownerID, starts, ends string) (uuid.UUID, error) {
	ds, err := uuid.Parse(datasetID)
	if err != nil || strings.TrimSpace(name) == "" || strings.TrimSpace(metric) == "" {
		return uuid.Nil, errInvalid("conjunto, métrica e nome são obrigatórios")
	}
	if period == "" {
		period = "monthly"
	}
	id := uuid.New()
	var owner any
	if ownerID != "" {
		oid, perr := uuid.Parse(ownerID)
		if perr != nil {
			return uuid.Nil, errInvalid("responsável inválido")
		}
		owner = oid
	}
	var startVal, endVal any
	if t, ok := parseDate(starts); ok {
		startVal = dateOnly(t)
	}
	if t, ok := parseDate(ends); ok {
		endVal = dateOnly(t)
	}
	_, err = s.pg.Exec(ctx, `
		INSERT INTO metric_goals
			(id, org_id, workspace_id, dataset_id, metric_name, name, target_value, period, owner_id, created_by, starts_at, ends_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, id, orgID, wsID, ds, metric, name, target, period, owner, userID, startVal, endVal)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

type invalidError string

func errInvalid(s string) error      { return invalidError(s) }
func (e invalidError) Error() string { return string(e) }
