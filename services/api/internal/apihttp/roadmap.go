package apihttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
	"github.com/thedobra/thedobra/services/api/internal/mljobs"
	"github.com/thedobra/thedobra/services/api/internal/quality"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
	"github.com/thedobra/thedobra/services/api/internal/schemax"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

func (s *Server) datasetHealth(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	var name, status string
	var lastSync, updated *time.Time
	var rows int64
	var score *float64
	var quality, schema, model []byte
	err = s.deps.PG.QueryRow(r.Context(), `
		SELECT d.name, d.status, d.last_sync_at, d.updated_at, d.row_count,
			COALESCE(d.quality_score, 0), COALESCE(d.quality_json, '{}'::jsonb),
			COALESCE(d.schema_json, '[]'::jsonb),
			COALESCE(sm.model_json, '{}'::jsonb)
		FROM datasets d
		LEFT JOIN semantic_models sm ON sm.dataset_id=d.id
		WHERE d.id=$1 AND d.org_id=$2 AND d.workspace_id=$3
	`, id, org, ws).Scan(&name, &status, &lastSync, &updated, &rows, &score, &quality, &schema, &model)
	if err != nil {
		httpx.Error(w, 404, "not_found", "conjunto não encontrado")
		return
	}

	var latestRun struct {
		Status     string     `json:"status"`
		Rows       *int64     `json:"rows_affected,omitempty"`
		DurationMS *int64     `json:"duration_ms,omitempty"`
		Error      *string    `json:"error,omitempty"`
		StartedAt  time.Time  `json:"started_at"`
		FinishedAt *time.Time `json:"finished_at,omitempty"`
	}
	_ = s.deps.PG.QueryRow(r.Context(), `
		SELECT status, rows_affected, duration_ms, error, started_at, finished_at
		FROM dataset_refresh_runs
		WHERE dataset_id=$1 AND org_id=$2 AND workspace_id=$3
		ORDER BY started_at DESC LIMIT 1
	`, id, org, ws).Scan(&latestRun.Status, &latestRun.Rows, &latestRun.DurationMS, &latestRun.Error, &latestRun.StartedAt, &latestRun.FinishedAt)

	var columns []any
	var measures, dimensions int
	_ = json.Unmarshal(schema, &columns)
	var semanticModel semantic.Model
	if json.Unmarshal(model, &semanticModel) == nil {
		measures = len(semanticModel.Measures)
		dimensions = len(semanticModel.Dimensions)
	}
	freshness := "unknown"
	if lastSync != nil {
		age := time.Since(lastSync.UTC())
		switch {
		case age <= 24*time.Hour:
			freshness = "fresh"
		case age <= 7*24*time.Hour:
			freshness = "stale"
		default:
			freshness = "outdated"
		}
	}
	httpx.JSON(w, 200, map[string]any{
		"id": id, "name": name, "status": status, "row_count": rows,
		"quality_score": score, "quality": json.RawMessage(quality),
		"schema_columns": len(columns), "semantic_measures": measures,
		"semantic_dimensions": dimensions, "last_sync_at": lastSync,
		"updated_at": updated, "freshness": freshness, "latest_refresh": latestRun,
		"checks": []map[string]any{
			{"key": "data", "label": "Dados carregados", "ok": rows > 0},
			{"key": "quality", "label": "Qualidade aceitável", "ok": score != nil && *score >= 70},
			{"key": "semantic", "label": "Modelo semântico pronto", "ok": measures > 0 || dimensions > 0},
			{"key": "freshness", "label": "Dados atualizados", "ok": freshness == "fresh"},
		},
	})
}

func (s *Server) refreshDatasetQuality(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if !canWrite(role) {
		httpx.Error(w, 403, "forbidden", "apenas analistas ou administradores podem recalcular a qualidade")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	var schema []byte
	if err := s.deps.PG.QueryRow(r.Context(), `SELECT schema_json FROM datasets WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&schema); err != nil {
		httpx.Error(w, 404, "not_found", "conjunto não encontrado")
		return
	}
	var columns []schemax.Column
	if err := json.Unmarshal(schema, &columns); err != nil || len(columns) == 0 {
		httpx.Error(w, 400, "profile_failed", "esquema do conjunto inválido")
		return
	}
	headers, rows, err := s.query.ReadRows(r.Context(), org, ws, id.String(), 50000)
	if err != nil {
		httpx.Error(w, 400, "profile_failed", err.Error())
		return
	}
	values := make([][]string, 0, len(rows))
	for _, row := range rows {
		record := make([]string, len(headers))
		for i, header := range headers {
			if row[header] != nil {
				record[i] = fmt.Sprint(row[header])
			}
		}
		values = append(values, record)
	}
	report := quality.Analyze(columns, values)
	raw := mustJSON(report)
	_, err = s.deps.PG.Exec(r.Context(), `UPDATE datasets SET quality_score=$1, quality_json=$2, updated_at=now() WHERE id=$3 AND org_id=$4 AND workspace_id=$5`, report.Score, raw, id, org, ws)
	if err != nil {
		httpx.Error(w, 500, "profile_failed", err.Error())
		return
	}
	s.audit(r, "DATASET_PROFILE_REFRESHED", "dataset", id, map[string]any{"user_id": uid, "rows_sampled": len(values), "score": report.Score})
	httpx.JSON(w, 200, report)
}

func (s *Server) listDatasetJobs(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT id, data_source_id, kind, status, progress_json, error, started_at, finished_at, created_at
		FROM ingestion_jobs
		WHERE dataset_id=$1 AND org_id=$2 AND workspace_id=$3
		ORDER BY created_at DESC LIMIT 50
	`, id, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var jobID uuid.UUID
		var dataSourceID *uuid.UUID
		var kind, status string
		var progress []byte
		var message *string
		var started, finished, created *time.Time
		if err := rows.Scan(&jobID, &dataSourceID, &kind, &status, &progress, &message, &started, &finished, &created); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		out = append(out, map[string]any{"id": jobID, "data_source_id": dataSourceID, "kind": kind, "status": status, "progress": json.RawMessage(progress), "error": message, "started_at": started, "finished_at": finished, "created_at": created})
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) datasetForecast(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	id := chi.URLParam(r, "id")
	model, name, err := s.ai.LoadModel(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", err.Error())
		return
	}
	measure := semantic.PrimaryMeasure(model)
	if measure == "" || model.TimeColumn == "" {
		httpx.Error(w, 400, "forecast_unavailable", "o conjunto precisa de uma métrica e uma dimensão de tempo")
		return
	}
	res, err := s.query.Execute(r.Context(), org, ws, uid, role, queryeng.Request{
		DatasetID: id, Measures: []string{measure}, Dimensions: []string{model.TimeColumn}, Limit: 90,
	})
	if err != nil || len(res.Rows) < 3 {
		httpx.Error(w, 400, "forecast_unavailable", "são necessários pelo menos três pontos históricos")
		return
	}
	series := make([]float64, 0, len(res.Rows))
	for _, row := range res.Rows {
		for _, value := range row {
			switch number := value.(type) {
			case float64:
				series = append(series, number)
			case int64:
				series = append(series, float64(number))
			case int:
				series = append(series, float64(number))
			}
		}
	}
	if len(series) < 3 {
		httpx.Error(w, 400, "forecast_unavailable", "não foi possível extrair uma série numérica")
		return
	}
	horizon := 6
	if raw := r.URL.Query().Get("horizon"); raw != "" {
		if _, scanErr := fmt.Sscanf(raw, "%d", &horizon); scanErr != nil || horizon < 1 || horizon > 24 {
			horizon = 6
		}
	}
	forecast := mljobs.Forecast(r.Context(), s.deps.Redis, series, horizon)
	httpx.JSON(w, 200, map[string]any{
		"dataset_id": id, "dataset_name": name, "measure": measure,
		"time_column": model.TimeColumn, "historical": series,
		"forecast": forecast, "source": "mljobs",
	})
}

func (s *Server) queryObservability(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	var total, slow, cached int
	var avgDuration *float64
	err := s.deps.PG.QueryRow(r.Context(), `
		SELECT COUNT(*),
			COUNT(*) FILTER (WHERE COALESCE(duration_ms,0) >= 1000),
			COUNT(*) FILTER (WHERE cache_hit),
			AVG(duration_ms)
		FROM query_history
		WHERE org_id=$1 AND workspace_id=$2 AND created_at >= now() - interval '30 days'
	`, org, ws).Scan(&total, &slow, &cached, &avgDuration)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT id, fingerprint, COALESCE(duration_ms,0), COALESCE(row_count,0),
			cache_hit, created_at, query_json
		FROM query_history
		WHERE org_id=$1 AND workspace_id=$2 AND created_at >= now() - interval '30 days'
		ORDER BY COALESCE(duration_ms,0) DESC, created_at DESC LIMIT 20
	`, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	slowest := []map[string]any{}
	for rows.Next() {
		var id uuid.UUID
		var fingerprint string
		var duration, rowCount int
		var cacheHit bool
		var created time.Time
		var query []byte
		if err := rows.Scan(&id, &fingerprint, &duration, &rowCount, &cacheHit, &created, &query); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		slowest = append(slowest, map[string]any{"id": id, "fingerprint": fingerprint, "duration_ms": duration, "row_count": rowCount, "cache_hit": cacheHit, "created_at": created, "query": json.RawMessage(query)})
	}
	httpx.JSON(w, 200, map[string]any{
		"window": "30d", "total_queries": total, "slow_queries": slow,
		"cache_hits": cached, "average_duration_ms": avgDuration, "slowest": slowest,
	})
}

func (s *Server) relationshipSuggestions(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	modelID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "modelo inválido")
		return
	}
	var datasetID uuid.UUID
	if err := s.deps.PG.QueryRow(r.Context(), `SELECT dataset_id FROM semantic_models WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, modelID, org, ws).Scan(&datasetID); err != nil {
		httpx.Error(w, 404, "not_found", "modelo não encontrado")
		return
	}
	var baseSchema []byte
	if err := s.deps.PG.QueryRow(r.Context(), `SELECT schema_json FROM datasets WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, datasetID, org, ws).Scan(&baseSchema); err != nil {
		httpx.Error(w, 404, "not_found", "conjunto não encontrado")
		return
	}
	var baseCols []schemax.Column
	_ = json.Unmarshal(baseSchema, &baseCols)
	rows, err := s.deps.PG.Query(r.Context(), `SELECT id,name,schema_json FROM datasets WHERE id<>$1 AND org_id=$2 AND workspace_id=$3 AND status='ready'`, datasetID, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	type candidate struct {
		DatasetID   uuid.UUID `json:"dataset_id"`
		DatasetName string    `json:"dataset_name"`
		FromColumn  string    `json:"from_column"`
		ToColumn    string    `json:"to_column"`
		Confidence  string    `json:"confidence"`
		Reason      string    `json:"reason"`
	}
	out := []candidate{}
	for rows.Next() {
		var id uuid.UUID
		var name string
		var raw []byte
		if err := rows.Scan(&id, &name, &raw); err != nil {
			continue
		}
		var cols []schemax.Column
		_ = json.Unmarshal(raw, &cols)
		for _, left := range baseCols {
			if left.Name == "" {
				continue
			}
			for _, right := range cols {
				if right.Name == "" || !sameRelationshipColumn(left.Name, right.Name) {
					continue
				}
				confidence := "medium"
				if left.Name == right.Name {
					confidence = "high"
				}
				out = append(out, candidate{DatasetID: id, DatasetName: name, FromColumn: left.Name, ToColumn: right.Name, Confidence: confidence, Reason: "nomes de coluna compatíveis"})
			}
		}
	}
	httpx.JSON(w, 200, out)
}

func sameRelationshipColumn(left, right string) bool {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	if left == right {
		return true
	}
	left = strings.TrimSuffix(left, "_id")
	right = strings.TrimSuffix(right, "_id")
	return left == right && left != ""
}

func (s *Server) listMetricGoals(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT id, dataset_id, metric_name, name, target_value, period, owner_id, status, metadata, created_at, updated_at
		FROM metric_goals WHERE org_id=$1 AND workspace_id=$2
		ORDER BY updated_at DESC
	`, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, datasetID uuid.UUID
		var metric, name, period, status string
		var target float64
		var owner *uuid.UUID
		var metadata []byte
		var created, updated time.Time
		if err := rows.Scan(&id, &datasetID, &metric, &name, &target, &period, &owner, &status, &metadata, &created, &updated); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		out = append(out, map[string]any{
			"id": id, "dataset_id": datasetID, "metric_name": metric, "name": name,
			"target_value": target, "period": period, "owner_id": owner, "status": status,
			"metadata": json.RawMessage(metadata), "created_at": created, "updated_at": updated,
		})
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT id, kind, title, body, url, read_at, created_at
		FROM notifications
		WHERE user_id=$1 AND org_id=$2 AND workspace_id=$3
		ORDER BY created_at DESC LIMIT 50
	`, uid, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	unread := 0
	for rows.Next() {
		var id uuid.UUID
		var kind, title, body string
		var url *string
		var readAt *time.Time
		var created time.Time
		if err := rows.Scan(&id, &kind, &title, &body, &url, &readAt, &created); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		if readAt == nil {
			unread++
		}
		out = append(out, map[string]any{"id": id, "kind": kind, "title": title, "body": body, "url": url, "read_at": readAt, "created_at": created})
	}
	httpx.JSON(w, 200, map[string]any{"items": out, "unread": unread})
}

func (s *Server) readNotification(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "notificação inválida")
		return
	}
	ct, err := s.deps.PG.Exec(r.Context(), `UPDATE notifications SET read_at=COALESCE(read_at,now()) WHERE id=$1 AND user_id=$2 AND org_id=$3 AND workspace_id=$4`, id, uid, org, ws)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Error(w, 404, "not_found", "notificação não encontrada")
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id, "read": true})
}

func (s *Server) readAllNotifications(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	if _, err := s.deps.PG.Exec(r.Context(), `UPDATE notifications SET read_at=COALESCE(read_at,now()) WHERE user_id=$1 AND org_id=$2 AND workspace_id=$3 AND read_at IS NULL`, uid, org, ws); err != nil {
		httpx.Error(w, 500, "update_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"read": true})
}

func (s *Server) createMetricGoal(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if !canWrite(role) {
		httpx.Error(w, 403, "forbidden", "apenas analistas ou administradores podem criar metas")
		return
	}
	var body struct {
		DatasetID  string          `json:"dataset_id"`
		MetricName string          `json:"metric_name"`
		Name       string          `json:"name"`
		Target     float64         `json:"target_value"`
		Period     string          `json:"period"`
		OwnerID    string          `json:"owner_id"`
		Metadata   json.RawMessage `json:"metadata"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.DatasetID == "" || body.MetricName == "" || body.Name == "" {
		httpx.Error(w, 400, "invalid", "dataset_id, metric_name e name são obrigatórios")
		return
	}
	datasetID, err := uuid.Parse(body.DatasetID)
	if err != nil || body.Target == 0 {
		httpx.Error(w, 400, "invalid", "conjunto ou meta inválida")
		return
	}
	model, _, err := s.ai.LoadModel(r.Context(), org, ws, body.DatasetID)
	if err != nil {
		httpx.Error(w, 400, "invalid_metric", err.Error())
		return
	}
	measure, ok := semantic.ResolveMeasure(model, body.MetricName)
	if !ok {
		httpx.Error(w, 400, "invalid_metric", "a métrica não existe no modelo semântico")
		return
	}
	period := body.Period
	if period == "" {
		period = "monthly"
	}
	if body.Metadata == nil {
		body.Metadata = []byte(`{}`)
	}
	var owner *uuid.UUID
	if body.OwnerID != "" {
		id, parseErr := uuid.Parse(body.OwnerID)
		if parseErr != nil {
			httpx.Error(w, 400, "invalid", "owner_id inválido")
			return
		}
		owner = &id
	}
	id := uuid.New()
	_, err = s.deps.PG.Exec(r.Context(), `
		INSERT INTO metric_goals
			(id, org_id, workspace_id, dataset_id, metric_name, name, target_value, period, owner_id, metadata, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, id, org, ws, datasetID, measure.Name, body.Name, body.Target, period, owner, body.Metadata, uid)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	s.audit(r, "METRIC_GOAL_CREATED", "metric_goal", id, map[string]any{"dataset_id": datasetID, "metric": measure.Name})
	httpx.JSON(w, 201, map[string]any{"id": id, "metric_name": measure.Name})
}

func (s *Server) patchMetricGoal(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if !canWrite(role) {
		httpx.Error(w, 403, "forbidden", "sem permissão para editar metas")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	var body struct {
		Target *float64 `json:"target_value"`
		Status string   `json:"status"`
		Owner  string   `json:"owner_id"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	owner := any(nil)
	if body.Owner != "" {
		parsed, parseErr := uuid.Parse(body.Owner)
		if parseErr != nil {
			httpx.Error(w, 400, "invalid", "owner_id inválido")
			return
		}
		owner = parsed
	}
	ct, err := s.deps.PG.Exec(r.Context(), `
		UPDATE metric_goals SET
			target_value=COALESCE($1,target_value),
			status=CASE WHEN $2='' THEN status ELSE $2 END,
			owner_id=COALESCE($3,owner_id), updated_at=now()
		WHERE id=$4 AND org_id=$5 AND workspace_id=$6
	`, body.Target, body.Status, owner, id, org, ws)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Error(w, 404, "not_found", "meta não encontrada")
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id})
}

func (s *Server) listMetricCertifications(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "modelo inválido")
		return
	}
	var datasetID uuid.UUID
	if err := s.deps.PG.QueryRow(r.Context(), `SELECT dataset_id FROM semantic_models WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&datasetID); err != nil {
		httpx.Error(w, 404, "not_found", "modelo não encontrado")
		return
	}
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT id, metric_name, description, status, certified_by, created_at, updated_at
		FROM metric_certifications WHERE dataset_id=$1 AND org_id=$2 AND workspace_id=$3
		ORDER BY metric_name
	`, datasetID, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var cid uuid.UUID
		var metric, description, status string
		var by *uuid.UUID
		var created, updated time.Time
		if err := rows.Scan(&cid, &metric, &description, &status, &by, &created, &updated); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		out = append(out, map[string]any{"id": cid, "metric_name": metric, "description": description, "status": status, "certified_by": by, "created_at": created, "updated_at": updated})
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) createMetricCertification(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if !canWrite(role) {
		httpx.Error(w, 403, "forbidden", "sem permissão para certificar métricas")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "modelo inválido")
		return
	}
	var body struct {
		Metric      string `json:"metric_name"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Metric == "" {
		httpx.Error(w, 400, "invalid", "metric_name obrigatório")
		return
	}
	var datasetID uuid.UUID
	var modelJSON []byte
	if err := s.deps.PG.QueryRow(r.Context(), `SELECT dataset_id, model_json FROM semantic_models WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&datasetID, &modelJSON); err != nil {
		httpx.Error(w, 404, "not_found", "modelo não encontrado")
		return
	}
	var model semantic.Model
	_ = json.Unmarshal(modelJSON, &model)
	measure, ok := semantic.ResolveMeasure(model, body.Metric)
	if !ok {
		httpx.Error(w, 400, "invalid_metric", "a métrica não existe no modelo semântico")
		return
	}
	if body.Status == "" {
		body.Status = "certified"
	}
	_, err = s.deps.PG.Exec(r.Context(), `
		INSERT INTO metric_certifications
			(org_id, workspace_id, dataset_id, metric_name, certified_by, description, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (dataset_id, metric_name) DO UPDATE SET
			certified_by=EXCLUDED.certified_by, description=EXCLUDED.description,
			status=EXCLUDED.status, updated_at=now()
	`, org, ws, datasetID, measure.Name, uid, body.Description, body.Status)
	if err != nil {
		httpx.Error(w, 400, "save_failed", err.Error())
		return
	}
	httpx.JSON(w, 201, map[string]any{"metric_name": measure.Name, "status": body.Status})
}

func (s *Server) patchMetricCertification(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if !canWrite(role) {
		httpx.Error(w, 403, "forbidden", "sem permissão")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "certificationID"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	var body struct {
		Status      string `json:"status"`
		Description string `json:"description"`
	}
	_ = httpx.Decode(r, &body)
	ct, err := s.deps.PG.Exec(r.Context(), `UPDATE metric_certifications SET status=COALESCE(NULLIF($1,''),status), description=COALESCE(NULLIF($2,''),description), updated_at=now() WHERE id=$3 AND org_id=$4 AND workspace_id=$5`, body.Status, body.Description, id, org, ws)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Error(w, 404, "not_found", "certificação não encontrada")
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id})
}

func (s *Server) listDashboardComments(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	dashboardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "dashboard inválido")
		return
	}
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT c.id, c.widget_id, c.body, c.resolved, c.user_id, COALESCE(u.name,u.email,''), c.created_at, c.updated_at
		FROM dashboard_comments c LEFT JOIN users u ON u.id=c.user_id
		WHERE c.dashboard_id=$1 AND c.org_id=$2 AND c.workspace_id=$3
		ORDER BY c.created_at DESC
	`, dashboardID, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, userID uuid.UUID
		var widgetID *string
		var body, author string
		var resolved bool
		var created, updated time.Time
		if err := rows.Scan(&id, &widgetID, &body, &resolved, &userID, &author, &created, &updated); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		out = append(out, map[string]any{"id": id, "widget_id": widgetID, "body": body, "resolved": resolved, "user_id": userID, "author": author, "created_at": created, "updated_at": updated})
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) createDashboardComment(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	dashboardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "dashboard inválido")
		return
	}
	var body struct {
		WidgetID string `json:"widget_id"`
		Body     string `json:"body"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Body == "" {
		httpx.Error(w, 400, "invalid", "comentário obrigatório")
		return
	}
	id := uuid.New()
	_, err = s.deps.PG.Exec(r.Context(), `INSERT INTO dashboard_comments (id,org_id,workspace_id,dashboard_id,widget_id,user_id,body) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7)`, id, org, ws, dashboardID, body.WidgetID, uid, body.Body)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	httpx.JSON(w, 201, map[string]any{"id": id})
}

func (s *Server) patchDashboardComment(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "commentID"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "comentário inválido")
		return
	}
	var body struct {
		Resolved *bool  `json:"resolved"`
		Body     string `json:"body"`
	}
	_ = httpx.Decode(r, &body)
	ownerClause := "AND user_id=$6"
	args := []any{body.Body, body.Resolved, id, org, ws, uid}
	if role == "owner" || role == "admin" {
		ownerClause = ""
		args = args[:5]
	}
	ct, err := s.deps.PG.Exec(r.Context(), fmt.Sprintf(`UPDATE dashboard_comments SET body=COALESCE(NULLIF($1,''),body), resolved=COALESCE($2,resolved), updated_at=now() WHERE id=$3 AND org_id=$4 AND workspace_id=$5 %s`, ownerClause), args...)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Error(w, 404, "not_found", "comentário não encontrado")
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id})
}

func (s *Server) listDashboardVersions(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	dashboardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "dashboard inválido")
		return
	}
	rows, err := s.deps.PG.Query(r.Context(), `SELECT id,version,name,description,created_by,created_at FROM dashboard_versions WHERE dashboard_id=$1 AND org_id=$2 AND workspace_id=$3 ORDER BY version DESC`, dashboardID, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, by uuid.UUID
		var version int
		var name, description string
		var created time.Time
		if err := rows.Scan(&id, &version, &name, &description, &by, &created); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		out = append(out, map[string]any{"id": id, "version": version, "name": name, "description": description, "created_by": by, "created_at": created})
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) restoreDashboardVersion(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if !canWrite(role) {
		httpx.Error(w, 403, "forbidden", "sem permissão para restaurar versões")
		return
	}
	dashboardID, err := uuid.Parse(chi.URLParam(r, "id"))
	version := 0
	if err == nil {
		_, err = fmt.Sscanf(chi.URLParam(r, "version"), "%d", &version)
	}
	if err != nil || version < 1 {
		httpx.Error(w, 400, "invalid", "versão inválida")
		return
	}
	var name, description string
	var layout []byte
	if err := s.deps.PG.QueryRow(r.Context(), `SELECT name,description,layout_json FROM dashboard_versions WHERE dashboard_id=$1 AND org_id=$2 AND workspace_id=$3 AND version=$4`, dashboardID, org, ws, version).Scan(&name, &description, &layout); err != nil {
		httpx.Error(w, 404, "not_found", "versão não encontrada")
		return
	}
	ct, err := s.deps.PG.Exec(r.Context(), `UPDATE dashboards SET name=$1,description=$2,layout_json=$3,updated_at=now() WHERE id=$4 AND org_id=$5 AND workspace_id=$6`, name, description, layout, dashboardID, org, ws)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Error(w, 404, "not_found", "dashboard não encontrado")
		return
	}
	s.audit(r, "DASHBOARD_VERSION_RESTORED", "dashboard", dashboardID, map[string]any{"version": version, "user_id": uid})
	httpx.JSON(w, 200, map[string]any{"id": dashboardID, "version": version})
}
