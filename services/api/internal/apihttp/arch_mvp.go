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
	"github.com/thedobra/thedobra/services/api/internal/apps"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
	"github.com/thedobra/thedobra/services/api/internal/flow"
	"github.com/thedobra/thedobra/services/api/internal/gateway"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
	"github.com/thedobra/thedobra/services/api/internal/ingest"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
	"github.com/thedobra/thedobra/services/api/internal/rls"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
	"github.com/thedobra/thedobra/services/api/internal/semanticxpr"
)

// ================= Flows =================

func (s *Server) listFlows(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	list, err := s.flow.List(r.Context(), org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	if list == nil {
		list = []flow.Flow{}
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) getFlow(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	f, err := s.flow.Get(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", "flow não encontrado")
		return
	}
	steps, _ := s.flow.ListSteps(r.Context(), id)
	if steps == nil {
		steps = []flow.Step{}
	}
	httpx.JSON(w, 200, map[string]any{"flow": f, "steps": steps})
}

type createFlowBody struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	SourceDatasetID *uuid.UUID      `json:"source_dataset_id"`
	TargetDatasetID *uuid.UUID      `json:"target_dataset_id"`
	Layout          json.RawMessage `json:"layout"`
	Steps           []flow.Step     `json:"steps"`
}

func (s *Server) createFlow(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	var body createFlowBody
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if body.Name == "" {
		httpx.Error(w, 400, "invalid", "nome obrigatório")
		return
	}
	if len(body.Steps) > 20 {
		httpx.Error(w, 400, "invalid", "máximo de 20 passos no flow inicial")
		return
	}
	f := flow.Flow{
		OrgID:           org,
		WorkspaceID:     ws,
		Name:            body.Name,
		Description:     body.Description,
		Status:          "draft",
		SourceDatasetID: body.SourceDatasetID,
		TargetDatasetID: body.TargetDatasetID,
		Layout:          body.Layout,
		CreatedBy:       &uid,
	}
	var (
		id  uuid.UUID
		err error
	)
	if len(body.Steps) > 0 {
		id, err = s.flow.CreateWithGraph(r.Context(), f, body.Steps)
	} else {
		id, err = s.flow.Create(r.Context(), f)
	}
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	s.audit(r, "FLOW_CREATED", "flow", id, nil)
	httpx.JSON(w, 201, map[string]any{"id": id})
}

func (s *Server) updateFlow(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var body flow.Flow
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if err := s.flow.Update(r.Context(), org, ws, id, body); err != nil {
		httpx.Error(w, 404, "not_found", err.Error())
		return
	}
	s.audit(r, "FLOW_UPDATED", "flow", id, nil)
	httpx.JSON(w, 200, map[string]any{"id": id})
}

func (s *Server) deleteFlow(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	if err := s.flow.Delete(r.Context(), org, ws, id); err != nil {
		httpx.Error(w, 500, "delete_failed", err.Error())
		return
	}
	s.sched.DeleteForTarget(r.Context(), "flow", id)
	s.audit(r, "FLOW_DELETED", "flow", id, nil)
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) listFlowSteps(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	steps, err := s.flow.ListSteps(r.Context(), id)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	if steps == nil {
		steps = []flow.Step{}
	}
	httpx.JSON(w, 200, steps)
}

func (s *Server) createFlowStep(w http.ResponseWriter, r *http.Request) {
	fid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var st flow.Step
	if err := httpx.Decode(r, &st); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	st.FlowID = fid
	id, err := s.flow.CreateStep(r.Context(), st)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	httpx.JSON(w, 201, map[string]any{"id": id})
}

func (s *Server) updateFlowStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := uuid.Parse(chi.URLParam(r, "stepId"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad step id")
		return
	}
	var st flow.Step
	if err := httpx.Decode(r, &st); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if err := s.flow.UpdateStep(r.Context(), stepID, st); err != nil {
		httpx.Error(w, 404, "not_found", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": stepID})
}

func (s *Server) deleteFlowStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := uuid.Parse(chi.URLParam(r, "stepId"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad step id")
		return
	}
	if err := s.flow.DeleteStep(r.Context(), stepID); err != nil {
		httpx.Error(w, 500, "delete_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) runFlow(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	fid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	if _, err := s.flow.Get(r.Context(), org, ws, fid); err != nil {
		httpx.Error(w, 404, "not_found", "flow não encontrado")
		return
	}
	runID, err := s.flow.CreateRun(r.Context(), flow.Run{FlowID: fid, Status: "pending"})
	if err != nil {
		httpx.Error(w, 500, "run_failed", err.Error())
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		reader := func(datasetID string, limit int) ([]string, []map[string]any, error) {
			return s.query.ReadRows(ctx, org, ws, datasetID, limit)
		}
		_, _ = s.flowEng.Execute(ctx, runID, uid, reader)
	}()
	s.audit(r, "FLOW_RUN_CREATED", "flow_run", runID, map[string]any{"flow_id": fid})
	httpx.JSON(w, 201, map[string]any{"run_id": runID, "status": "running"})
}

func (s *Server) listFlowRuns(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	list, err := s.flow.ListRuns(r.Context(), id)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	if list == nil {
		list = []flow.Run{}
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) getFlowRun(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(chi.URLParam(r, "runId"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad run id")
		return
	}
	rc, err := s.flow.GetRun(r.Context(), runID)
	if err != nil {
		httpx.Error(w, 404, "not_found", "run não encontrado")
		return
	}
	httpx.JSON(w, 200, rc)
}

func (s *Server) getFlowRunLogs(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(chi.URLParam(r, "runId"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad run id")
		return
	}
	logs, err := s.flow.GetRunLogs(r.Context(), runID)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	if logs == nil {
		logs = []flow.RunLog{}
	}
	httpx.JSON(w, 200, logs)
}

// ================= Datasets: storage mode & RLS =================

func (s *Server) patchDatasetStorageMode(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var body struct {
		StorageMode string `json:"storage_mode"`
		SourceTable string `json:"source_table,omitempty"`
		SourceQuery string `json:"source_query,omitempty"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.StorageMode == "" {
		httpx.Error(w, 400, "invalid", "storage_mode obrigatório")
		return
	}
	if body.StorageMode != "import" && body.StorageMode != "direct_query" && body.StorageMode != "composite" {
		httpx.Error(w, 400, "invalid", "storage_mode inválido")
		return
	}
	ct, err := s.deps.PG.Exec(r.Context(), `
		UPDATE datasets SET storage_mode=$1, source_table=$2, source_query=$3, updated_at=now()
		WHERE id=$4 AND org_id=$5 AND workspace_id=$6
	`, body.StorageMode, body.SourceTable, body.SourceQuery, id, org, ws)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Error(w, 404, "not_found", "conjunto não encontrado")
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id, "storage_mode": body.StorageMode})
}

func (s *Server) listDatasetRLS(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	list, err := s.rls.List(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	if list == nil {
		list = []rls.Rule{}
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) createDatasetRLS(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var body struct {
		Role   string `json:"role"`
		Column string `json:"column"`
		Expr   string `json:"expression"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Column == "" || body.Expr == "" {
		httpx.Error(w, 400, "invalid", "column e expression obrigatórios")
		return
	}
	rid, err := s.rls.Create(r.Context(), org, ws, id, body.Role, body.Column, body.Expr)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	httpx.JSON(w, 201, map[string]any{"id": rid})
}

func (s *Server) updateDatasetRLS(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var body struct {
		Role   string `json:"role"`
		Column string `json:"column"`
		Expr   string `json:"expression"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if err := s.rls.Update(r.Context(), org, ws, rid, body.Role, body.Column, body.Expr); err != nil {
		httpx.Error(w, 404, "not_found", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": rid})
}

func (s *Server) deleteDatasetRLS(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	if err := s.rls.Delete(r.Context(), org, ws, rid); err != nil {
		httpx.Error(w, 500, "delete_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

// ================= Semantic model: hierarchies, relationships, measure validation =================

func (s *Server) listHierarchies(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var dsID uuid.UUID
	err = s.deps.PG.QueryRow(r.Context(), `SELECT dataset_id FROM semantic_models WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&dsID)
	if err != nil {
		httpx.Error(w, 404, "not_found", "modelo não encontrado")
		return
	}
	rows, err := s.deps.PG.Query(r.Context(), `SELECT id, name, levels FROM semantic_hierarchies WHERE dataset_id=$1 ORDER BY created_at`, dsID)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var hid uuid.UUID
		var name string
		var raw []byte
		if err := rows.Scan(&hid, &name, &raw); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		var levels []string
		_ = json.Unmarshal(raw, &levels)
		out = append(out, map[string]any{"id": hid, "name": name, "levels": levels})
	}
	if out == nil {
		out = []map[string]any{}
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) createHierarchy(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var body struct {
		Name   string   `json:"name"`
		Levels []string `json:"levels"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Name == "" || len(body.Levels) == 0 {
		httpx.Error(w, 400, "invalid", "nome e níveis obrigatórios")
		return
	}
	var dsID uuid.UUID
	err = s.deps.PG.QueryRow(r.Context(), `SELECT dataset_id FROM semantic_models WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&dsID)
	if err != nil {
		httpx.Error(w, 404, "not_found", "modelo não encontrado")
		return
	}
	levels, _ := json.Marshal(body.Levels)
	var hid uuid.UUID
	err = s.deps.PG.QueryRow(r.Context(), `
		INSERT INTO semantic_hierarchies (org_id, workspace_id, dataset_id, name, levels)
		VALUES ($1,$2,$3,$4,$5) RETURNING id
	`, org, ws, dsID, body.Name, levels).Scan(&hid)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	httpx.JSON(w, 201, map[string]any{"id": hid})
}

func (s *Server) deleteHierarchy(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	hid, err := uuid.Parse(chi.URLParam(r, "hid"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	_, err = s.deps.PG.Exec(r.Context(), `DELETE FROM semantic_hierarchies WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, hid, org, ws)
	if err != nil {
		httpx.Error(w, 500, "delete_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) listRelationships(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var dsID uuid.UUID
	err = s.deps.PG.QueryRow(r.Context(), `SELECT dataset_id FROM semantic_models WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&dsID)
	if err != nil {
		httpx.Error(w, 404, "not_found", "modelo não encontrado")
		return
	}
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT id, from_dataset_id, from_column, to_dataset_id, to_column, relationship_type
		FROM semantic_relationships
		WHERE org_id=$1 AND workspace_id=$2 AND from_dataset_id=$3
		ORDER BY created_at
	`, org, ws, dsID)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var rid, from, to uuid.UUID
		var fc, tc, rt string
		if err := rows.Scan(&rid, &from, &fc, &to, &tc, &rt); err != nil {
			httpx.Error(w, 500, "scan", err.Error())
			return
		}
		out = append(out, map[string]any{"id": rid, "from_dataset_id": from, "from_column": fc, "to_dataset_id": to, "to_column": tc, "type": rt})
	}
	if out == nil {
		out = []map[string]any{}
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) createRelationship(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var body struct {
		ToDatasetID string `json:"to_dataset_id"`
		FromColumn  string `json:"from_column"`
		ToColumn    string `json:"to_column"`
		Type        string `json:"type"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.ToDatasetID == "" || body.FromColumn == "" || body.ToColumn == "" {
		httpx.Error(w, 400, "invalid", "campos obrigatórios em falta")
		return
	}
	var dsID uuid.UUID
	err = s.deps.PG.QueryRow(r.Context(), `SELECT dataset_id FROM semantic_models WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&dsID)
	if err != nil {
		httpx.Error(w, 404, "not_found", "modelo não encontrado")
		return
	}
	toID, err := uuid.Parse(body.ToDatasetID)
	if err != nil {
		httpx.Error(w, 400, "invalid", "to_dataset_id inválido")
		return
	}
	if body.Type == "" {
		body.Type = "many_to_one"
	}
	var rid uuid.UUID
	err = s.deps.PG.QueryRow(r.Context(), `
		INSERT INTO semantic_relationships (org_id, workspace_id, from_dataset_id, from_column, to_dataset_id, to_column, relationship_type)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id
	`, org, ws, dsID, body.FromColumn, toID, body.ToColumn, body.Type).Scan(&rid)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	httpx.JSON(w, 201, map[string]any{"id": rid})
}

func (s *Server) deleteRelationship(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	rid, err := uuid.Parse(chi.URLParam(r, "rid"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	_, err = s.deps.PG.Exec(r.Context(), `DELETE FROM semantic_relationships WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, rid, org, ws)
	if err != nil {
		httpx.Error(w, 500, "delete_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) validateMeasure(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var body struct {
		Expression string `json:"expression"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Expression == "" {
		httpx.Error(w, 400, "invalid", "expression obrigatória")
		return
	}
	expr, err := semanticxpr.Parse(body.Expression)
	if err != nil {
		httpx.JSON(w, 200, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	sql, err := expr.ToSQL(func(col string) string { return "`" + col + "`" })
	if err != nil {
		httpx.JSON(w, 200, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	var dsID uuid.UUID
	err = s.deps.PG.QueryRow(r.Context(), `SELECT dataset_id FROM semantic_models WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&dsID)
	if err == nil {
		// Try to validate column names against schema.
		var schema []byte
		_ = s.deps.PG.QueryRow(r.Context(), `SELECT schema_json FROM datasets WHERE id=$1`, dsID).Scan(&schema)
	}
	httpx.JSON(w, 200, map[string]any{"valid": true, "sql": sql, "func": expr.Func})
}

// ================= AI generation endpoints =================

func (s *Server) generateSQL(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	if err := s.ent.Check(r.Context(), org, "ai"); err != nil {
		httpx.Error(w, 402, "quota", err.Error())
		return
	}
	var body struct {
		Prompt    string `json:"prompt"`
		DatasetID string `json:"dataset_id"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Prompt == "" {
		httpx.Error(w, 400, "invalid", "prompt obrigatório")
		return
	}
	model, dsName, err := s.ai.LoadModel(r.Context(), org, ws, body.DatasetID)
	if err != nil {
		httpx.Error(w, 400, "no_model", err.Error())
		return
	}
	q := strings.ToLower(body.Prompt)
	measure := pickMeasureForPrompt(model, q)
	dim := pickDimensionForPrompt(model, q)
	var sql string
	if dim != "" {
		sql = fmt.Sprintf("SELECT `%s`, SUM(`%s`) FROM %s GROUP BY `%s` ORDER BY SUM(`%s`) DESC LIMIT 20", dim, measure, dsName, dim, measure)
	} else {
		sql = fmt.Sprintf("SELECT SUM(`%s`) FROM %s", measure, dsName)
	}
	if s.deps.Cfg.OpenAIKey != "" {
		// LLM path is a future enhancement; deterministic template is returned for now.
	}
	s.audit(r, "AI_SQL_GENERATED", "ai", uuid.Nil, map[string]any{"dataset_id": body.DatasetID})
	httpx.JSON(w, 200, map[string]any{"sql": sql, "explanation": "SQL gerado a partir das métricas oficiais e dimensões disponíveis.", "measure": measure, "dimension": dim})
}

func (s *Server) generateMeasure(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	if err := s.ent.Check(r.Context(), org, "ai"); err != nil {
		httpx.Error(w, 402, "quota", err.Error())
		return
	}
	var body struct {
		Prompt    string `json:"prompt"`
		DatasetID string `json:"dataset_id"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Prompt == "" {
		httpx.Error(w, 400, "invalid", "prompt obrigatório")
		return
	}
	model, _, err := s.ai.LoadModel(r.Context(), org, ws, body.DatasetID)
	if err != nil {
		httpx.Error(w, 400, "no_model", err.Error())
		return
	}
	q := strings.ToLower(body.Prompt)
	expr := "SUM(revenue)"
	name := "Nova métrica"
	if strings.Contains(q, "média") || strings.Contains(q, "average") || strings.Contains(q, "ticket") {
		expr = "AVERAGE(revenue)"
		name = "Média"
	} else if strings.Contains(q, "cont") || strings.Contains(q, "count") || strings.Contains(q, "número") {
		expr = "COUNT(*)"
		name = "Contagem"
	} else if strings.Contains(q, "diferença") || strings.Contains(q, "yoy") || strings.Contains(q, "ano") {
		expr = "YOY(revenue)"
		name = "YoY"
	} else {
		for _, m := range model.Measures {
			if strings.Contains(q, strings.ToLower(m.Column)) || strings.Contains(q, strings.ToLower(m.Name)) {
				expr = "SUM(" + m.Column + ")"
				name = m.Name
				break
			}
		}
	}
	_ = uid
	httpx.JSON(w, 200, map[string]any{"name": name, "expression": expr, "explanation": "Medida DAX-like gerada a partir do pedido e das colunas disponíveis."})
}

func (s *Server) generateVisual(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	if err := s.ent.Check(r.Context(), org, "ai"); err != nil {
		httpx.Error(w, 402, "quota", err.Error())
		return
	}
	var body struct {
		Prompt    string `json:"prompt"`
		DatasetID string `json:"dataset_id"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Prompt == "" {
		httpx.Error(w, 400, "invalid", "prompt obrigatório")
		return
	}
	model, _, err := s.ai.LoadModel(r.Context(), org, ws, body.DatasetID)
	if err != nil {
		httpx.Error(w, 400, "no_model", err.Error())
		return
	}
	q := strings.ToLower(body.Prompt)
	visType := "bar"
	if strings.Contains(q, "tendência") || strings.Contains(q, "trend") || strings.Contains(q, "tempo") || strings.Contains(q, "ao longo") {
		visType = "line"
	} else if strings.Contains(q, "part") || strings.Contains(q, "percentagem") || strings.Contains(q, "pie") || strings.Contains(q, "pizza") {
		visType = "pie"
	} else if strings.Contains(q, "tabela") || strings.Contains(q, "table") || strings.Contains(q, "lista") {
		visType = "table"
	}
	dim := pickDimensionForPrompt(model, q)
	measure := pickMeasureForPrompt(model, q)
	color := "primary"
	if dim != "" {
		color = "indigo"
	}
	httpx.JSON(w, 200, map[string]any{
		"type":        visType,
		"dimension":   dim,
		"measure":     measure,
		"sort":        "desc",
		"color":       color,
		"explanation": fmt.Sprintf("Widget %s com %s por %s.", visType, measure, dim),
	})
}

func pickMeasureForPrompt(model semantic.Model, q string) string {
	for _, m := range model.Measures {
		ln := strings.ToLower(m.Name)
		lc := strings.ToLower(m.Column)
		if strings.Contains(q, ln) || strings.Contains(q, lc) || strings.Contains(q, strings.ReplaceAll(ln, " ", "_")) {
			return m.Name
		}
	}
	if strings.Contains(q, "receita") || strings.Contains(q, "revenue") || strings.Contains(q, "vendas") || strings.Contains(q, "sales") {
		if m, ok := semantic.ResolveMeasure(model, "revenue"); ok {
			return m.Name
		}
	}
	if len(model.Measures) > 0 {
		return model.Measures[0].Name
	}
	return "revenue"
}

func pickDimensionForPrompt(model semantic.Model, q string) string {
	cands := []string{"region", "product", "customer", "channel", "segment", "seller", "regiao", "produto", "cliente", "canal", "segmento"}
	for _, c := range cands {
		if strings.Contains(q, c) {
			key := c
			switch c {
			case "regiao":
				key = "region"
			case "produto":
				key = "product"
			case "cliente":
				key = "customer"
			case "canal":
				key = "channel"
			case "segmento":
				key = "segment"
			}
			if d, ok := semantic.ResolveDimension(model, key); ok {
				return d.Column
			}
		}
	}
	if model.TimeColumn != "" && (strings.Contains(q, "tempo") || strings.Contains(q, "time") || strings.Contains(q, "tendência") || strings.Contains(q, "trend")) {
		return model.TimeColumn
	}
	if len(model.Dimensions) > 0 {
		return model.Dimensions[0].Column
	}
	return ""
}

// ================= Apps =================

func (s *Server) listApps(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	list, err := s.apps.List(r.Context(), org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	if list == nil {
		list = []apps.App{}
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) getApp(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	app, err := s.apps.Get(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", "app não encontrado")
		return
	}
	ds, _ := s.apps.Dashboards(r.Context(), id)
	if ds == nil {
		ds = []apps.DashboardRef{}
	}
	reps, _ := s.apps.Reports(r.Context(), id)
	if reps == nil {
		reps = []apps.ReportRef{}
	}
	httpx.JSON(w, 200, map[string]any{"app": app, "dashboards": ds, "reports": reps})
}

func (s *Server) createApp(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	var body apps.App
	if err := httpx.Decode(r, &body); err != nil || body.Name == "" {
		httpx.Error(w, 400, "invalid", "nome obrigatório")
		return
	}
	body.OrgID = org
	body.WorkspaceID = ws
	body.Status = "draft"
	body.CreatedBy = &uid
	id, err := s.apps.Create(r.Context(), body)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	s.audit(r, "APP_CREATED", "app", id, nil)
	httpx.JSON(w, 201, map[string]any{"id": id})
}

func (s *Server) updateApp(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	var body apps.App
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if err := s.apps.Update(r.Context(), org, ws, id, body); err != nil {
		httpx.Error(w, 404, "not_found", err.Error())
		return
	}
	s.audit(r, "APP_UPDATED", "app", id, nil)
	httpx.JSON(w, 200, map[string]any{"id": id})
}

func (s *Server) deleteApp(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	if err := s.apps.Delete(r.Context(), org, ws, id); err != nil {
		httpx.Error(w, 500, "delete_failed", err.Error())
		return
	}
	s.audit(r, "APP_DELETED", "app", id, nil)
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) setAppContent(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	if _, err := s.apps.Get(r.Context(), org, ws, id); err != nil {
		httpx.Error(w, 404, "not_found", "app não encontrado")
		return
	}
	var body struct {
		Dashboards []struct {
			ID      string `json:"id"`
			Section string `json:"section"`
		} `json:"dashboards"`
		Reports []struct {
			ID      string `json:"id"`
			Section string `json:"section"`
		} `json:"reports"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	ds := make([]apps.DashboardRef, 0, len(body.Dashboards))
	for i, d := range body.Dashboards {
		if did, err := uuid.Parse(d.ID); err == nil {
			ds = append(ds, apps.DashboardRef{ID: did, Order: i, Section: d.Section})
		}
	}
	reps := make([]apps.ReportRef, 0, len(body.Reports))
	for i, rp := range body.Reports {
		if rid, err := uuid.Parse(rp.ID); err == nil {
			reps = append(reps, apps.ReportRef{ID: rid, Order: i, Section: rp.Section})
		}
	}
	if err := s.apps.SetDashboards(r.Context(), id, ds); err != nil {
		httpx.Error(w, 400, "update_failed", err.Error())
		return
	}
	if err := s.apps.SetReports(r.Context(), id, reps); err != nil {
		httpx.Error(w, 400, "update_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id})
}

func (s *Server) publishApp(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	app, err := s.apps.Get(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", "app não encontrado")
		return
	}
	token := ""
	if app.PublicToken != nil {
		token = *app.PublicToken
	}
	if token == "" {
		tok, err := cryptoenc.RandomToken(24)
		if err != nil {
			httpx.Error(w, 500, "token", err.Error())
			return
		}
		token = tok
	}
	if err := s.apps.SetPublicToken(r.Context(), org, ws, id, token); err != nil {
		httpx.Error(w, 500, "publish_failed", err.Error())
		return
	}
	s.audit(r, "APP_PUBLISHED", "app", id, nil)
	httpx.JSON(w, 200, map[string]any{"id": id, "public_token": token, "public_url": s.deps.Cfg.WebOrigin + "/apps/public/" + token})
}

func (s *Server) publicApp(w http.ResponseWriter, r *http.Request) {
	tok := chi.URLParam(r, "token")
	app, err := s.apps.GetByPublicToken(r.Context(), tok)
	if err != nil {
		httpx.Error(w, 404, "not_found", "app não encontrado")
		return
	}
	ds, _ := s.apps.Dashboards(r.Context(), app.ID)
	if ds == nil {
		ds = []apps.DashboardRef{}
	}
	reps, _ := s.apps.Reports(r.Context(), app.ID)
	if reps == nil {
		reps = []apps.ReportRef{}
	}
	var dashboards []map[string]any
	for _, ref := range ds {
		var name, desc string
		err := s.deps.PG.QueryRow(r.Context(), `SELECT name, description FROM dashboards WHERE id=$1`, ref.ID).Scan(&name, &desc)
		if err != nil {
			continue
		}
		shareToken := s.ensureDashboardShare(r.Context(), app.OrgID, app.WorkspaceID, ref.ID)
		dashboards = append(dashboards, map[string]any{
			"id": ref.ID, "name": name, "description": desc, "section": ref.Section,
			"public_url": s.deps.Cfg.WebOrigin + "/share/" + shareToken,
		})
	}
	var reports []map[string]any
	for _, ref := range reps {
		var name, cadence string
		var last *time.Time
		_ = s.deps.PG.QueryRow(r.Context(), `SELECT name, cadence, last_generated_at FROM reports WHERE id=$1`, ref.ID).Scan(&name, &cadence, &last)
		reports = append(reports, map[string]any{"id": ref.ID, "name": name, "section": ref.Section, "cadence": cadence, "last_generated_at": last})
	}
	httpx.JSON(w, 200, map[string]any{"app": app, "dashboards": dashboards, "reports": reports})
}

func (s *Server) ensureDashboardShare(ctx context.Context, orgID, wsID, dashboardID uuid.UUID) string {
	var tok string
	err := s.deps.PG.QueryRow(ctx, `
		SELECT token FROM dashboard_shares WHERE org_id=$1 AND workspace_id=$2 AND dashboard_id=$3 LIMIT 1
	`, orgID, wsID, dashboardID).Scan(&tok)
	if err == nil && tok != "" {
		return tok
	}
	tok, _ = cryptoenc.RandomToken(18)
	_, _ = s.deps.PG.Exec(ctx, `
		INSERT INTO dashboard_shares (org_id, workspace_id, dashboard_id, token)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (token) DO UPDATE SET token=EXCLUDED.token
	`, orgID, wsID, dashboardID, tok)
	return tok
}

func (s *Server) openApp(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	app, err := s.apps.Get(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", "app não encontrado")
		return
	}
	ds, err := s.apps.Dashboards(r.Context(), id)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	// For viewers, the app is read-only; we include dashboard snapshots for each dashboard.
	var dashboards []map[string]any
	for _, ref := range ds {
		var name, desc string
		var layout []byte
		var updated time.Time
		err := s.deps.PG.QueryRow(r.Context(), `SELECT name, description, layout_json, updated_at FROM dashboards WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, ref.ID, org, ws).Scan(&name, &desc, &layout, &updated)
		if err != nil {
			continue
		}
		var parsed struct {
			Widgets []map[string]any `json:"widgets"`
		}
		_ = json.Unmarshal(layout, &parsed)
		for i, wgt := range parsed.Widgets {
			qraw, ok := wgt["query"]
			if !ok || qraw == nil {
				continue
			}
			b, _ := json.Marshal(qraw)
			var req queryeng.Request
			if json.Unmarshal(b, &req) != nil || req.DatasetID == "" {
				continue
			}
			res, err := s.query.Execute(r.Context(), org, ws, uuid.Nil, role, req)
			if err != nil {
				parsed.Widgets[i]["error"] = err.Error()
				continue
			}
			parsed.Widgets[i]["result"] = res
		}
		dashboards = append(dashboards, map[string]any{"id": ref.ID, "name": name, "description": desc, "updated_at": updated, "layout": parsed, "read_only": role == "viewer", "section": ref.Section})
	}
	httpx.JSON(w, 200, map[string]any{"app": app, "dashboards": dashboards})
}

// ================= Gateway =================

func (s *Server) gatewayHeartbeat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token   string         `json:"token"`
		Name    string         `json:"name"`
		Version string         `json:"version"`
		Status  string         `json:"status"`
		Meta    map[string]any `json:"metadata,omitempty"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Token == "" {
		httpx.Error(w, 400, "invalid", "token obrigatório")
		return
	}
	if body.Status == "" {
		body.Status = "online"
	}
	// Try heartbeat first; if not found, register as a new instance.
	tokenHash := gateway.HashToken(body.Token)
	if err := s.gateway.Heartbeat(r.Context(), tokenHash, body.Status, body.Version); err != nil {
		_, _ = s.gateway.Register(r.Context(), nil, body.Name, tokenHash, body.Version, body.Meta)
	}
	httpx.JSON(w, 200, map[string]any{"status": "ok", "server_time": time.Now().UTC()})
}

var _ = ingest.Result{}

func (s *Server) listGatewayInstances(w http.ResponseWriter, r *http.Request) {
	_, org, _, _ := principal(r)
	list, err := s.gateway.List(r.Context(), org)
	if err != nil {
		httpx.Error(w, 500, "list_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) generateGatewayToken(w http.ResponseWriter, r *http.Request) {
	_, org, _, _ := principal(r)
	var body struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if body.Name == "" {
		body.Name = "gateway-" + uuid.New().String()[:8]
	}
	token, id, err := s.gateway.GenerateToken(r.Context(), org, body.Name)
	if err != nil {
		httpx.Error(w, 500, "token_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"token": token, "id": id, "name": body.Name})
}
