package apihttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/aiagent"
	"github.com/thedobra/thedobra/services/api/internal/connector"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
	"github.com/thedobra/thedobra/services/api/internal/ingest"
	"github.com/thedobra/thedobra/services/api/internal/notify"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
	"github.com/thedobra/thedobra/services/api/internal/schemax"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	var datasets int
	_ = s.deps.PG.QueryRow(r.Context(), `SELECT COUNT(*) FROM datasets WHERE org_id=$1 AND workspace_id=$2 AND status='ready'`, org, ws).Scan(&datasets)
	var dsID uuid.UUID
	err := s.deps.PG.QueryRow(r.Context(), `SELECT id FROM datasets WHERE org_id=$1 AND workspace_id=$2 AND status='ready' ORDER BY updated_at DESC LIMIT 1`, org, ws).Scan(&dsID)
	out := map[string]any{"datasets": datasets, "brief": nil}
	if err == nil {
		brief, err := s.intel.AnalyzeDataset(r.Context(), org, ws, uid, dsID, role)
		if err == nil {
			out["brief"] = brief
		}
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) listSources(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT ds.id, ds.name, ds.type, ds.status, ds.last_sync_at, ds.created_at,
			sch.enabled, sch.frequency, sch.next_run_at, sch.last_run_at, sch.last_status
		FROM data_sources ds
		LEFT JOIN sync_schedules sch
			ON sch.target_id = ds.id AND sch.kind = 'connector' AND sch.org_id = ds.org_id AND sch.workspace_id = ds.workspace_id
		WHERE ds.org_id=$1 AND ds.workspace_id=$2 ORDER BY ds.created_at DESC
	`, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	type schedInfo struct {
		Enabled    bool       `json:"enabled"`
		Frequency  string     `json:"frequency"`
		NextRunAt  *time.Time `json:"next_run_at,omitempty"`
		LastRunAt  *time.Time `json:"last_run_at,omitempty"`
		LastStatus string     `json:"last_status,omitempty"`
	}
	type src struct {
		ID          uuid.UUID  `json:"id"`
		Name        string     `json:"name"`
		Type        string     `json:"type"`
		Status      string     `json:"status"`
		LastSyncAt  *time.Time `json:"last_sync_at"`
		CreatedAt   time.Time  `json:"created_at"`
		Implemented bool       `json:"implemented"`
		Preview     bool       `json:"preview"`
		Schedule    *schedInfo `json:"schedule,omitempty"`
	}
	var out []src
	for rows.Next() {
		var x src
		var en *bool
		var freq, lastStatus *string
		var next, last *time.Time
		if err := rows.Scan(&x.ID, &x.Name, &x.Type, &x.Status, &x.LastSyncAt, &x.CreatedAt, &en, &freq, &next, &last, &lastStatus); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		x.Implemented = connector.Implemented(x.Type)
		x.Preview = !x.Implemented
		if en != nil && freq != nil {
			st := ""
			if lastStatus != nil {
				st = *lastStatus
			}
			x.Schedule = &schedInfo{Enabled: *en, Frequency: *freq, NextRunAt: next, LastRunAt: last, LastStatus: st}
		}
		out = append(out, x)
	}
	if out == nil {
		out = []src{}
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) createSource(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	var body struct {
		Name   string          `json:"name"`
		Type   string          `json:"type"`
		Config json.RawMessage `json:"config"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	body.Type = connector.Canonical(body.Type)
	if !connector.Known(body.Type) {
		httpx.Error(w, 400, "invalid_type", "tipo de conector desconhecido")
		return
	}
	var cfg ingest.SQLConfig
	if len(body.Config) > 0 {
		if err := json.Unmarshal(body.Config, &cfg); err != nil {
			httpx.Error(w, 400, "invalid", "config inválida")
			return
		}
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		if it := connector.ByID(body.Type); it != nil {
			name = it.Label
		} else {
			name = body.Type
		}
	}
	id, err := s.ingest.SaveDataSource(r.Context(), org, ws, uid, name, body.Type, cfg)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	s.audit(r, "DATA_SOURCE_CREATED", "data_source", id, map[string]any{"type": body.Type})
	item := connector.ByID(body.Type)
	out := map[string]any{"id": id, "type": body.Type, "name": name, "implemented": connector.Implemented(body.Type)}
	if item != nil && item.Preview {
		out["preview"] = true
		out["message"] = connector.PreviewMessage(body.Type)
	}
	httpx.JSON(w, 201, out)
}

func (s *Server) discoverSource(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	tables, err := s.ingest.Discover(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 400, "discover_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, tables)
}

func (s *Server) syncSource(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var body struct {
		Table       string                  `json:"table"`
		Name        string                  `json:"name"`
		StorageMode string                  `json:"storage_mode"`
		Selection   *ingest.SourceSelection `json:"selection"`
	}
	_ = httpx.Decode(r, &body)
	res, err := s.ingest.SyncSourceWithSelection(r.Context(), org, ws, uid, id, body.Table, body.Name, body.Selection)
	if err != nil {
		httpx.Error(w, 400, "sync_failed", err.Error())
		return
	}
	if body.StorageMode != "" {
		_, _ = s.deps.PG.Exec(r.Context(), `UPDATE datasets SET storage_mode=$1 WHERE id=$2`, body.StorageMode, res.DatasetID)
	}
	s.audit(r, "DATASET_CREATED", "dataset", res.DatasetID, map[string]any{"source": id})
	s.lineage.RecordIngest(r.Context(), org, ws, id, res.DatasetID, res.Name, res.Name)
	for _, m := range res.Semantic.Measures {
		s.lineage.RecordMetric(r.Context(), org, ws, res.DatasetID, m.Name, m.Expression)
	}
	httpx.JSON(w, 201, res)
}

func (s *Server) listDatasets(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT d.id, d.name, d.slug, d.status, d.row_count, d.quality_score, d.created_at, d.updated_at,
			COALESCE(d.storage_mode, 'import'), d.data_source_id, src.type, src.name
		FROM datasets d
		LEFT JOIN data_sources src ON src.id = d.data_source_id
		WHERE d.org_id=$1 AND d.workspace_id=$2
		ORDER BY d.updated_at DESC
	`, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	type ds struct {
		ID           uuid.UUID  `json:"id"`
		Name         string     `json:"name"`
		Slug         string     `json:"slug"`
		Status       string     `json:"status"`
		RowCount     int64      `json:"row_count"`
		QualityScore *float64   `json:"quality_score"`
		CreatedAt    time.Time  `json:"created_at"`
		UpdatedAt    time.Time  `json:"updated_at"`
		StorageMode  string     `json:"storage_mode"`
		SourceID     *uuid.UUID `json:"source_id,omitempty"`
		SourceType   *string    `json:"source_type,omitempty"`
		SourceName   *string    `json:"source_name,omitempty"`
	}
	out := []ds{}
	for rows.Next() {
		var x ds
		if err := rows.Scan(&x.ID, &x.Name, &x.Slug, &x.Status, &x.RowCount, &x.QualityScore, &x.CreatedAt, &x.UpdatedAt, &x.StorageMode, &x.SourceID, &x.SourceType, &x.SourceName); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		out = append(out, x)
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) uploadDataset(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	if err := s.ent.Check(r.Context(), org, "dataset"); err != nil {
		httpx.Error(w, 402, "quota", err.Error())
		return
	}
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		httpx.Error(w, 400, "invalid", "multipart obrigatório")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, 400, "invalid", "ficheiro obrigatório")
		return
	}
	defer file.Close()
	name := r.FormValue("name")
	if name == "" {
		name = hdr.Filename
	}
	res, err := s.ingest.IngestFile(r.Context(), org, ws, uid, name, hdr.Filename, file)
	if err != nil {
		httpx.Error(w, 400, "ingest_failed", err.Error())
		return
	}
	s.audit(r, "DATASET_CREATED", "dataset", res.DatasetID, map[string]any{"file": hdr.Filename})
	s.lineage.RecordIngest(r.Context(), org, ws, res.DatasetID, res.DatasetID, hdr.Filename, name)
	for _, m := range res.Semantic.Measures {
		s.lineage.RecordMetric(r.Context(), org, ws, res.DatasetID, m.Name, m.Expression)
	}
	if sid := r.FormValue("data_source_id"); sid != "" {
		if parsed, err := uuid.Parse(sid); err == nil {
			s.ingest.LinkDataset(r.Context(), org, parsed, res.DatasetID)
		}
	}
	httpx.JSON(w, 201, res)
}

func (s *Server) demoDataset(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	if err := s.ent.Check(r.Context(), org, "dataset"); err != nil {
		httpx.Error(w, 402, "quota", err.Error())
		return
	}
	res, err := s.ingest.LoadDemo(r.Context(), org, ws, uid, 25000)
	if err != nil {
		httpx.Error(w, 500, "demo_failed", err.Error())
		return
	}
	s.audit(r, "DATASET_CREATED", "dataset", res.DatasetID, map[string]any{"demo": true})
	s.lineage.RecordIngest(r.Context(), org, ws, res.DatasetID, res.DatasetID, "Demo de vendas", res.Name)
	for _, m := range res.Semantic.Measures {
		s.lineage.RecordMetric(r.Context(), org, ws, res.DatasetID, m.Name, m.Expression)
	}
	httpx.JSON(w, 201, res)
}

func (s *Server) deleteDataset(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if role == "viewer" {
		httpx.Error(w, 403, "forbidden", "sem permissão para excluir conjuntos")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	if err := s.ingest.DeleteDataset(r.Context(), org, ws, id); err != nil {
		httpx.Error(w, 400, "delete_failed", err.Error())
		return
	}
	s.lineage.DeleteForDataset(r.Context(), org, ws, id)
	s.sched.DeleteForTarget(r.Context(), "dataset", id)
	s.audit(r, "DATASET_DELETED", "dataset", id, nil)
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) getDataset(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var name, slug, status, table, storageMode string
	var sourceTable, sourceQuery *string
	var sourceID *uuid.UUID
	var sourceType, sourceName *string
	var rows int64
	var qscore *float64
	var schema, quality, model []byte
	err = s.deps.PG.QueryRow(r.Context(), `
		SELECT d.name, d.slug, d.status, d.clickhouse_table, d.storage_mode, d.source_table, d.source_query, d.row_count, d.quality_score,
			d.schema_json, d.quality_json, COALESCE(s.model_json,'{}'::jsonb),
			d.data_source_id, src.type, src.name
		FROM datasets d
		LEFT JOIN semantic_models s ON s.dataset_id=d.id
		LEFT JOIN data_sources src ON src.id = d.data_source_id
		WHERE d.id=$1 AND d.org_id=$2 AND d.workspace_id=$3
	`, id, org, ws).Scan(&name, &slug, &status, &table, &storageMode, &sourceTable, &sourceQuery, &rows, &qscore, &schema, &quality, &model, &sourceID, &sourceType, &sourceName)
	if err != nil {
		httpx.Error(w, 404, "not_found", "conjunto não encontrado")
		return
	}
	model = s.hydrateAndPersistModel(r.Context(), org, ws, id, name, schema, model)
	httpx.JSON(w, 200, map[string]any{
		"id": id, "name": name, "slug": slug, "status": status, "clickhouse_table": table,
		"storage_mode": storageMode, "source_table": sourceTable, "source_query": sourceQuery,
		"source_id": sourceID, "source_type": sourceType, "source_name": sourceName,
		"row_count": rows, "quality_score": qscore,
		"schema": json.RawMessage(schema), "quality": json.RawMessage(quality),
		"semantic_model": json.RawMessage(model),
	})
}

func (s *Server) previewDataset(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	res, err := s.query.Preview(r.Context(), org, ws, chi.URLParam(r, "id"), 50)
	if err != nil {
		httpx.Error(w, 400, "preview_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, res)
}

func (s *Server) datasetQuality(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var q []byte
	err := s.deps.PG.QueryRow(r.Context(), `SELECT quality_json FROM datasets WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&q)
	if err != nil {
		httpx.Error(w, 404, "not_found", "conjunto não encontrado")
		return
	}
	httpx.JSON(w, 200, json.RawMessage(q))
}

func (s *Server) listSemantic(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT COALESCE(s.id, d.id), d.id, COALESCE(s.name, d.name), COALESCE(s.status, 'published'),
			COALESCE(s.model_json, '{}'::jsonb), d.schema_json, d.name
		FROM datasets d
		LEFT JOIN semantic_models s ON s.dataset_id = d.id
		WHERE d.org_id=$1 AND d.workspace_id=$2
		ORDER BY d.updated_at DESC
	`, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, ds uuid.UUID
		var name, status, dsName string
		var model, schema []byte
		if err := rows.Scan(&id, &ds, &name, &status, &model, &schema, &dsName); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		model = hydrateModelJSON(schema, model, dsName)
		out = append(out, map[string]any{"id": id, "dataset_id": ds, "name": name, "status": status, "model": json.RawMessage(model)})
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) getSemantic(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var ds uuid.UUID
	var name, status string
	var model []byte
	err := s.deps.PG.QueryRow(r.Context(), `SELECT dataset_id, name, status, model_json FROM semantic_models WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&ds, &name, &status, &model)
	if err != nil {
		httpx.Error(w, 404, "not_found", "modelo não encontrado")
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id, "dataset_id": ds, "name": name, "status": status, "model": json.RawMessage(model)})
}

func (s *Server) putSemantic(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var model semantic.Model
	if err := httpx.Decode(r, &model); err != nil {
		httpx.Error(w, 400, "invalid", "invalid model")
		return
	}
	raw, _ := json.Marshal(model)
	ct, err := s.deps.PG.Exec(r.Context(), `UPDATE semantic_models SET model_json=$1, status='published', updated_at=now() WHERE id=$2 AND org_id=$3 AND workspace_id=$4`, raw, id, org, ws)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Error(w, 404, "not_found", "modelo não encontrado")
		return
	}
	httpx.JSON(w, 200, model)
}

func (s *Server) runQuery(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if err := s.ent.Check(r.Context(), org, "query"); err != nil {
		httpx.Error(w, 402, "quota", err.Error())
		return
	}
	var req queryeng.Request
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, 400, "invalid", "invalid query")
		return
	}
	res, err := s.query.Execute(r.Context(), org, ws, uid, role, req)
	if err != nil {
		httpx.Error(w, 400, "query_failed", err.Error())
		return
	}
	s.audit(r, "QUERY_EXECUTED", "query", uuid.Nil, map[string]any{"fingerprint": res.Fingerprint, "ms": res.DurationMs})
	httpx.JSON(w, 200, res)
}

func (s *Server) queryHistory(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT id, fingerprint, duration_ms, row_count, cache_hit, created_at, sql_text
		FROM query_history WHERE org_id=$1 AND workspace_id=$2 ORDER BY created_at DESC LIMIT 50
	`, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	type qh struct {
		ID          uuid.UUID `json:"id"`
		Fingerprint string    `json:"fingerprint"`
		DurationMs  *int      `json:"duration_ms"`
		RowCount    *int      `json:"row_count"`
		CacheHit    bool      `json:"cache_hit"`
		CreatedAt   time.Time `json:"created_at"`
		SQL         *string   `json:"sql"`
	}
	out := []qh{}
	for rows.Next() {
		var x qh
		if err := rows.Scan(&x.ID, &x.Fingerprint, &x.DurationMs, &x.RowCount, &x.CacheHit, &x.CreatedAt, &x.SQL); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		out = append(out, x)
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) listDashboards(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `SELECT id, name, description, updated_at FROM dashboards WHERE org_id=$1 AND workspace_id=$2 ORDER BY updated_at DESC`, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	type d struct {
		ID          uuid.UUID `json:"id"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
		UpdatedAt   time.Time `json:"updated_at"`
	}
	out := []d{}
	for rows.Next() {
		var x d
		if err := rows.Scan(&x.ID, &x.Name, &x.Description, &x.UpdatedAt); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		out = append(out, x)
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) createDashboard(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	var body struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Layout      json.RawMessage `json:"layout"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Name == "" {
		httpx.Error(w, 400, "invalid", "nome obrigatório")
		return
	}
	if len(body.Layout) == 0 {
		body.Layout = []byte(`{"widgets":[]}`)
	}
	id := uuid.New()
	_, err := s.deps.PG.Exec(r.Context(), `
		INSERT INTO dashboards (id, org_id, workspace_id, name, description, layout_json, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, id, org, ws, body.Name, body.Description, body.Layout, uid)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	s.audit(r, "DASHBOARD_CREATED", "dashboard", id, nil)
	s.lineage.RecordDashboard(r.Context(), org, ws, id, uuid.Nil, body.Name)
	httpx.JSON(w, 201, map[string]any{"id": id})
}

func (s *Server) getDashboard(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var name, desc string
	var layout []byte
	var updated time.Time
	err := s.deps.PG.QueryRow(r.Context(), `SELECT name, description, layout_json, updated_at FROM dashboards WHERE id=$1 AND org_id=$2 AND workspace_id=$3`,
		id, org, ws).Scan(&name, &desc, &layout, &updated)
	if err != nil {
		httpx.Error(w, 404, "not_found", "dashboard não encontrado")
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id, "name": name, "description": desc, "layout": json.RawMessage(layout), "updated_at": updated})
}

func (s *Server) putDashboard(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var body struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Layout      json.RawMessage `json:"layout"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	ct, err := s.deps.PG.Exec(r.Context(), `UPDATE dashboards SET name=$1, description=$2, layout_json=$3, updated_at=now() WHERE id=$4 AND org_id=$5 AND workspace_id=$6`,
		body.Name, body.Description, body.Layout, id, org, ws)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Error(w, 404, "not_found", "dashboard não encontrado")
		return
	}
	s.audit(r, "DASHBOARD_UPDATED", "dashboard", id, nil)
	s.lineage.RecordDashboard(r.Context(), org, ws, id, uuid.Nil, body.Name)
	httpx.JSON(w, 200, map[string]any{"id": id})
}

func (s *Server) deleteDashboard(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if role == "viewer" {
		httpx.Error(w, 403, "forbidden", "sem permissão para excluir dashboards")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	ct, err := s.deps.PG.Exec(r.Context(), `DELETE FROM dashboards WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws)
	if err != nil {
		httpx.Error(w, 400, "delete_failed", err.Error())
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, 404, "not_found", "dashboard não encontrado")
		return
	}
	s.audit(r, "DASHBOARD_DELETED", "dashboard", id, nil)
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) aiDashboard(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	var body struct {
		Prompt    string `json:"prompt"`
		DatasetID string `json:"dataset_id"`
	}
	_ = httpx.Decode(r, &body)
	ds := body.DatasetID
	if ds == "" {
		var id uuid.UUID
		if err := s.deps.PG.QueryRow(r.Context(), `SELECT id FROM datasets WHERE org_id=$1 AND workspace_id=$2 AND status='ready' ORDER BY updated_at DESC LIMIT 1`, org, ws).Scan(&id); err != nil {
			httpx.Error(w, 400, "no_dataset", "ligue um conjunto de dados primeiro")
			return
		}
		ds = id.String()
	}
	var modelJSON []byte
	var dsName string
	_ = s.deps.PG.QueryRow(r.Context(), `SELECT d.name, s.model_json FROM datasets d JOIN semantic_models s ON s.dataset_id=d.id WHERE d.id=$1`, ds).Scan(&dsName, &modelJSON)
	var model semantic.Model
	_ = json.Unmarshal(modelJSON, &model)
	widgets := autoWidgets(ds, model)
	layout, _ := json.Marshal(map[string]any{"widgets": widgets})
	name := "Dashboard executivo"
	if body.Prompt != "" {
		name = "IA · " + body.Prompt
		if len(name) > 60 {
			name = name[:60]
		}
	}
	id := uuid.New()
	_, err := s.deps.PG.Exec(r.Context(), `INSERT INTO dashboards (id, org_id, workspace_id, name, description, layout_json, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		id, org, ws, name, "Gerado pela TheDobra a partir do conjunto "+dsName, layout, uid)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	s.audit(r, "DASHBOARD_CREATED", "dashboard", id, map[string]any{"ai": true})
	if dsID, err := uuid.Parse(ds); err == nil {
		s.lineage.RecordDashboard(r.Context(), org, ws, id, dsID, name)
	}
	httpx.JSON(w, 201, map[string]any{"id": id, "name": name})
}

func (s *Server) generateDashboard(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	if err := s.ent.Check(r.Context(), org, "ai"); err != nil {
		httpx.Error(w, 402, "quota", err.Error())
		return
	}
	var req aiagent.GenerateDashboardRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if req.Prompt == "" {
		httpx.Error(w, 400, "invalid", "prompt obrigatório")
		return
	}
	gen, err := s.ai.GenerateDashboard(r.Context(), org, ws, uid, req)
	if err != nil {
		httpx.Error(w, 400, "generate_failed", err.Error())
		return
	}
	layout, _ := json.Marshal(map[string]any{"widgets": gen.Widgets})
	id := uuid.New()
	_, err = s.deps.PG.Exec(r.Context(), `
		INSERT INTO dashboards (id, org_id, workspace_id, name, description, layout_json, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, id, org, ws, gen.Name, gen.Description, layout, uid)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	s.audit(r, "DASHBOARD_CREATED", "dashboard", id, map[string]any{"ai": true, "source": gen.Source, "dataset_id": gen.DatasetID})
	if dsID, err := uuid.Parse(gen.DatasetID); err == nil {
		s.lineage.RecordDashboard(r.Context(), org, ws, id, dsID, gen.Name)
	}
	httpx.JSON(w, 201, map[string]any{"id": id, "name": gen.Name, "url": "/dashboards/" + id.String(), "source": gen.Source})
}

func autoWidgets(datasetID string, model semantic.Model) []map[string]any {
	meas := []string{}
	for i, m := range model.Measures {
		if i >= 3 {
			break
		}
		meas = append(meas, m.Name)
	}
	dims := []string{}
	for _, d := range model.Dimensions {
		switch d.Column {
		case "region", "product", "channel", "segment", "customer":
			dims = append(dims, d.Column)
		}
	}
	widgets := []map[string]any{}
	x := 0
	for i, m := range meas {
		widgets = append(widgets, map[string]any{
			"id": fmt.Sprintf("kpi-%d", i), "type": "kpi", "title": m,
			"layout": map[string]int{"x": x, "y": 0, "w": 4, "h": 2},
			"query":  map[string]any{"dataset_id": datasetID, "measures": []string{m}, "limit": 1},
		})
		x += 4
	}
	if len(dims) > 0 && len(meas) > 0 {
		widgets = append(widgets, map[string]any{
			"id": "bar-1", "type": "bar", "title": meas[0] + " by " + dims[0],
			"layout": map[string]int{"x": 0, "y": 2, "w": 6, "h": 4},
			"query":  map[string]any{"dataset_id": datasetID, "measures": []string{meas[0]}, "dimensions": []string{dims[0]}, "limit": 12},
		})
	}
	if model.TimeColumn != "" && len(meas) > 0 {
		widgets = append(widgets, map[string]any{
			"id": "line-1", "type": "line", "title": meas[0] + " over time",
			"layout": map[string]int{"x": 6, "y": 2, "w": 6, "h": 4},
			"query":  map[string]any{"dataset_id": datasetID, "measures": []string{meas[0]}, "dimensions": []string{model.TimeColumn}, "limit": 90},
		})
	}
	if len(dims) > 1 && len(meas) > 0 {
		widgets = append(widgets, map[string]any{
			"id": "table-1", "type": "table", "title": "Breakdown",
			"layout": map[string]int{"x": 0, "y": 6, "w": 12, "h": 4},
			"query":  map[string]any{"dataset_id": datasetID, "measures": meas[:min(2, len(meas))], "dimensions": dims[:min(2, len(dims))], "limit": 20},
		})
	}
	return widgets
}

func (s *Server) aiConfig(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, 200, map[string]any{"openai_configured": s.deps.Cfg.OpenAIKey != ""})
}

func (s *Server) ask(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if err := s.ent.Check(r.Context(), org, "ai"); err != nil {
		httpx.Error(w, 402, "quota", err.Error())
		return
	}
	var req aiagent.AskRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	ans, err := s.ai.Ask(r.Context(), org, ws, uid, role, req)
	if err != nil {
		httpx.Error(w, 400, "ai_failed", err.Error())
		return
	}
	s.audit(r, "AI_QUERY_EXECUTED", "ai", uuid.Nil, map[string]any{"conversation_id": ans.ConversationID})
	httpx.JSON(w, 200, ans)
}

func (s *Server) conversations(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `SELECT id, title, created_at FROM ai_conversations WHERE org_id=$1 AND workspace_id=$2 AND user_id=$3 ORDER BY created_at DESC LIMIT 30`, org, ws, uid)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	type c struct {
		ID        uuid.UUID `json:"id"`
		Title     string    `json:"title"`
		CreatedAt time.Time `json:"created_at"`
	}
	out := []c{}
	for rows.Next() {
		var x c
		_ = rows.Scan(&x.ID, &x.Title, &x.CreatedAt)
		out = append(out, x)
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) insights(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	list, err := s.intel.List(r.Context(), org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) refreshInsights(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	var id uuid.UUID
	if err := s.deps.PG.QueryRow(r.Context(), `SELECT id FROM datasets WHERE org_id=$1 AND workspace_id=$2 AND status='ready' ORDER BY updated_at DESC LIMIT 1`, org, ws).Scan(&id); err != nil {
		httpx.Error(w, 400, "no_dataset", "nenhum conjunto disponível")
		return
	}
	brief, err := s.intel.AnalyzeDataset(r.Context(), org, ws, uid, id, role)
	if err != nil {
		httpx.Error(w, 400, "intel_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, brief)
}

func (s *Server) listAlerts(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `SELECT id, name, condition_json, channels, enabled, last_triggered_at FROM alerts WHERE org_id=$1 AND workspace_id=$2 ORDER BY created_at DESC`, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var name string
		var cond, ch []byte
		var enabled bool
		var last *time.Time
		if err := rows.Scan(&id, &name, &cond, &ch, &enabled, &last); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		out = append(out, map[string]any{"id": id, "name": name, "condition": json.RawMessage(cond), "channels": json.RawMessage(ch), "enabled": enabled, "last_triggered_at": last})
	}
	if out == nil {
		out = []map[string]any{}
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) createAlert(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	var body struct {
		Name      string          `json:"name"`
		Condition json.RawMessage `json:"condition"`
		Channels  json.RawMessage `json:"channels"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Name == "" {
		httpx.Error(w, 400, "invalid", "nome obrigatório")
		return
	}
	if len(body.Channels) == 0 {
		body.Channels = []byte(`["realtime"]`)
	}
	id := uuid.New()
	_, err := s.deps.PG.Exec(r.Context(), `INSERT INTO alerts (id, org_id, workspace_id, name, condition_json, channels) VALUES ($1,$2,$3,$4,$5,$6)`,
		id, org, ws, body.Name, body.Condition, body.Channels)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	httpx.JSON(w, 201, map[string]any{"id": id})
}

func (s *Server) evalAlert(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var cond []byte
	err := s.deps.PG.QueryRow(r.Context(), `SELECT condition_json FROM alerts WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&cond)
	if err != nil {
		httpx.Error(w, 404, "not_found", "alerta não encontrado")
		return
	}
	var c struct {
		DatasetID string  `json:"dataset_id"`
		Measure   string  `json:"measure"`
		Op        string  `json:"op"`
		Value     float64 `json:"value"`
	}
	_ = json.Unmarshal(cond, &c)
	res, err := s.query.Execute(r.Context(), org, ws, uid, role, queryeng.Request{DatasetID: c.DatasetID, Measures: []string{c.Measure}, Limit: 1})
	if err != nil {
		httpx.Error(w, 400, "eval_failed", err.Error())
		return
	}
	var val float64
	if len(res.Rows) > 0 {
		for _, v := range res.Rows[0] {
			switch t := v.(type) {
			case float64:
				val = t
			case int64:
				val = float64(t)
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
	}
	if triggered {
		_, _ = s.deps.PG.Exec(r.Context(), `UPDATE alerts SET last_triggered_at=now(), last_value=$2 WHERE id=$1`, id, mustJSON(map[string]any{"value": val}))
		var ch []byte
		var name string
		_ = s.deps.PG.QueryRow(r.Context(), `SELECT name, channels FROM alerts WHERE id=$1`, id).Scan(&name, &ch)
		var channels []string
		_ = json.Unmarshal(ch, &channels)
		s.notify.Deliver(r.Context(), id, channels, notify.Message{
			Title: name, Body: fmt.Sprintf("Alerta disparado: valor=%.2f", val), URL: s.deps.Cfg.WebOrigin + "/alerts",
		})
	}
	httpx.JSON(w, 200, map[string]any{"triggered": triggered, "value": val})
}

func (s *Server) listReports(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `SELECT id, name, cadence, last_generated_at, updated_at FROM reports WHERE org_id=$1 AND workspace_id=$2 ORDER BY updated_at DESC`, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var name, cadence string
		var last, updated *time.Time
		_ = rows.Scan(&id, &name, &cadence, &last, &updated)
		out = append(out, map[string]any{"id": id, "name": name, "cadence": cadence, "last_generated_at": last, "updated_at": updated})
	}
	if out == nil {
		out = []map[string]any{}
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) getReport(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var name, cadence string
	var pages, lastContent []byte
	var last, updated *time.Time
	err := s.deps.PG.QueryRow(r.Context(), `
		SELECT name, cadence, pages_json, last_generated_at, last_content_json, updated_at
		FROM reports WHERE id=$1 AND org_id=$2 AND workspace_id=$3
	`, id, org, ws).Scan(&name, &cadence, &pages, &last, &lastContent, &updated)
	if err != nil {
		httpx.Error(w, 404, "not_found", "relatório não encontrado")
		return
	}
	if pages == nil {
		pages = []byte("[]")
	}
	httpx.JSON(w, 200, map[string]any{
		"id": id, "name": name, "cadence": cadence,
		"pages":             json.RawMessage(pages),
		"last_generated_at": last,
		"last_content":      json.RawMessage(lastContent),
		"updated_at":        updated,
	})
}

func (s *Server) createReport(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	var body struct {
		Name    string          `json:"name"`
		Cadence string          `json:"cadence"`
		Pages   json.RawMessage `json:"pages"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Name == "" {
		httpx.Error(w, 400, "invalid", "nome obrigatório")
		return
	}
	if body.Cadence == "" {
		body.Cadence = "weekly"
	}
	if len(body.Pages) == 0 {
		body.Pages = []byte(`[{"name":"Página 1","widgets":[]}]`)
	}
	id := uuid.New()
	_, err := s.deps.PG.Exec(r.Context(), `INSERT INTO reports (id, org_id, workspace_id, name, cadence, pages_json) VALUES ($1,$2,$3,$4,$5,$6)`, id, org, ws, body.Name, body.Cadence, body.Pages)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	s.lineage.RecordReport(r.Context(), org, ws, id, uuid.Nil, body.Name)
	httpx.JSON(w, 201, map[string]any{"id": id})
}

func (s *Server) updateReport(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var body struct {
		Name    string          `json:"name"`
		Cadence string          `json:"cadence"`
		Pages   json.RawMessage `json:"pages"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if len(body.Pages) == 0 {
		body.Pages = []byte("[]")
	}
	ct, err := s.deps.PG.Exec(r.Context(), `
		UPDATE reports SET name=$1, cadence=$2, pages_json=$3, updated_at=now()
		WHERE id=$4 AND org_id=$5 AND workspace_id=$6
	`, body.Name, body.Cadence, body.Pages, id, org, ws)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Error(w, 404, "not_found", "relatório não encontrado")
		return
	}
	s.audit(r, "REPORT_UPDATED", "report", id, nil)
	httpx.JSON(w, 200, map[string]any{"id": id})
}

func (s *Server) deleteReport(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	_, err := s.deps.PG.Exec(r.Context(), `DELETE FROM reports WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws)
	if err != nil {
		httpx.Error(w, 500, "delete_failed", err.Error())
		return
	}
	s.audit(r, "REPORT_DELETED", "report", id, nil)
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) generateReport(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var ds uuid.UUID
	if err := s.deps.PG.QueryRow(r.Context(), `SELECT id FROM datasets WHERE org_id=$1 AND workspace_id=$2 AND status='ready' ORDER BY updated_at DESC LIMIT 1`, org, ws).Scan(&ds); err != nil {
		httpx.Error(w, 400, "no_dataset", "nenhum conjunto disponível")
		return
	}
	brief, err := s.intel.AnalyzeDataset(r.Context(), org, ws, uid, ds, role)
	if err != nil {
		httpx.Error(w, 400, "report_failed", err.Error())
		return
	}
	content := map[string]any{
		"executive_summary":   brief.Headline,
		"performance":         brief.MajorChanges,
		"risks":               brief.Risks,
		"opportunities":       brief.Opportunities,
		"recommended_actions": brief.Actions,
		"generated_at":        time.Now().UTC(),
	}
	raw, _ := json.Marshal(content)
	_, _ = s.deps.PG.Exec(r.Context(), `UPDATE reports SET last_generated_at=now(), last_content_json=$2 WHERE id=$1 AND org_id=$3 AND workspace_id=$4`, id, raw, org, ws)
	httpx.JSON(w, 200, content)
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	s.listSemantic(w, r)
}

func (s *Server) auditLogs(w http.ResponseWriter, r *http.Request) {
	_, org, _, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `SELECT id, action, resource_type, created_at FROM audit_logs WHERE org_id=$1 ORDER BY created_at DESC LIMIT 100`, org)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var action, rt string
		var at time.Time
		_ = rows.Scan(&id, &action, &rt, &at)
		out = append(out, map[string]any{"id": id, "action": action, "resource_type": rt, "created_at": at})
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) auditList(w http.ResponseWriter, r *http.Request) { s.auditLogs(w, r) }

func (s *Server) usage(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	httpx.JSON(w, 200, s.ent.Snapshot(r.Context(), org, ws))
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func hydrateModelJSON(schema, model []byte, name string) []byte {
	var cols []schemax.Column
	_ = json.Unmarshal(schema, &cols)
	var m semantic.Model
	_ = json.Unmarshal(model, &m)
	if m.Name == "" {
		m.Name = name + " · modelo"
	}
	m = semantic.Hydrate(m, cols)
	raw, _ := json.Marshal(m)
	return raw
}

func (s *Server) hydrateAndPersistModel(ctx context.Context, org, ws, datasetID uuid.UUID, name string, schema, model []byte) []byte {
	var orig semantic.Model
	_ = json.Unmarshal(model, &orig)
	raw := hydrateModelJSON(schema, model, name)
	if len(orig.Measures) > 0 && len(orig.Dimensions) > 0 {
		return raw
	}
	var hydrated semantic.Model
	_ = json.Unmarshal(raw, &hydrated)
	if len(hydrated.Measures) == 0 {
		return raw
	}
	_, _ = s.deps.PG.Exec(ctx, `
		INSERT INTO semantic_models (org_id, workspace_id, dataset_id, name, status, model_json)
		VALUES ($1,$2,$3,$4,'published',$5)
		ON CONFLICT (dataset_id) DO UPDATE SET model_json=EXCLUDED.model_json, name=EXCLUDED.name, status='published', updated_at=now()
	`, org, ws, datasetID, hydrated.Name, raw)
	return raw
}
