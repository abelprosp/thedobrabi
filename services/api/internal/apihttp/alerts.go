package apihttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
	"github.com/thedobra/thedobra/services/api/internal/notify"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
)

const alertCooldown = 15 * time.Minute

type alertCondition struct {
	DatasetID string  `json:"dataset_id"`
	Measure   string  `json:"measure"`
	Op        string  `json:"op"`
	Value     float64 `json:"value"`
}

func (s *Server) evalAlert(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	triggered, val, err := s.evaluateAlert(r.Context(), org, ws, uid, role, id, true)
	if err != nil {
		if err.Error() == "alerta não encontrado" {
			httpx.Error(w, 404, "not_found", err.Error())
			return
		}
		httpx.Error(w, 400, "eval_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"triggered": triggered, "value": val})
}

func (s *Server) runAlertLoop(ctx context.Context) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	s.tickAlerts(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.tickAlerts(ctx)
		}
	}
}

func (s *Server) tickAlerts(ctx context.Context) {
	locked, err := s.deps.Redis.SetNX(ctx, "thedobra:leader:alerts", "api", 55*time.Second).Result()
	if err != nil || !locked {
		return
	}
	defer s.deps.Redis.Del(ctx, "thedobra:leader:alerts")
	rows, err := s.deps.PG.Query(ctx, `
		SELECT id, org_id, workspace_id FROM alerts
		WHERE enabled AND (last_triggered_at IS NULL OR last_triggered_at < now() - interval '15 minutes')
	`)
	if err != nil {
		if s.deps.Log != nil {
			s.deps.Log.Warn("alerts.tick", "err", err)
		}
		return
	}
	defer rows.Close()
	type row struct{ id, org, ws uuid.UUID }
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.org, &r.ws); err == nil {
			list = append(list, r)
		}
	}
	for _, a := range list {
		_, _, err := s.evaluateAlert(ctx, a.org, a.ws, uuid.Nil, "analyst", a.id, false)
		if err != nil && s.deps.Log != nil {
			s.deps.Log.Warn("alerts.eval", "id", a.id, "err", err)
		}
	}
}

func (s *Server) evaluateAlert(ctx context.Context, org, ws, uid uuid.UUID, role string, id uuid.UUID, force bool) (bool, float64, error) {
	var cond []byte
	var enabled bool
	var last *time.Time
	err := s.deps.PG.QueryRow(ctx, `
		SELECT condition_json, enabled, last_triggered_at
		FROM alerts WHERE id=$1 AND org_id=$2 AND workspace_id=$3
	`, id, org, ws).Scan(&cond, &enabled, &last)
	if err != nil {
		return false, 0, fmt.Errorf("alerta não encontrado")
	}
	if !force && !enabled {
		return false, 0, nil
	}
	if !force && last != nil && time.Since(*last) < alertCooldown {
		return false, 0, nil
	}
	var c alertCondition
	if err := json.Unmarshal(cond, &c); err != nil || c.DatasetID == "" || c.Measure == "" {
		return false, 0, fmt.Errorf("condição de alerta inválida")
	}
	res, err := s.query.Execute(ctx, org, ws, uid, role, queryeng.Request{
		DatasetID: c.DatasetID, Measures: []string{c.Measure}, Limit: 1,
	})
	if err != nil {
		return false, 0, err
	}
	var val float64
	if len(res.Rows) > 0 {
		for _, v := range res.Rows[0] {
			switch t := v.(type) {
			case float64:
				val = t
			case int64:
				val = float64(t)
			case int:
				val = float64(t)
			case json.Number:
				val, _ = t.Float64()
			}
			break
		}
	}
	triggered := false
	switch c.Op {
	case "<":
		triggered = val < c.Value
	case ">":
		triggered = val > c.Value
	case "<=":
		triggered = val <= c.Value
	case ">=":
		triggered = val >= c.Value
	case "=":
		triggered = val == c.Value
	}
	if triggered {
		_, _ = s.deps.PG.Exec(ctx, `UPDATE alerts SET last_triggered_at=now(), last_value=$2 WHERE id=$1`, id, mustJSON(map[string]any{"value": val}))
		var ch []byte
		var name string
		_ = s.deps.PG.QueryRow(ctx, `SELECT name, channels FROM alerts WHERE id=$1`, id).Scan(&name, &ch)
		var channels []string
		_ = json.Unmarshal(ch, &channels)
		s.notify.Deliver(ctx, id, channels, notify.Message{
			Title: name, Body: fmt.Sprintf("Alerta disparado: valor=%.2f", val), URL: s.deps.Cfg.WebOrigin + "/alerts",
		})
	}
	return triggered, val, nil
}
