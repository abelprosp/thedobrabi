package apihttp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/flow"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
	"github.com/thedobra/thedobra/services/api/internal/scheduler"
)

func canWrite(role string) bool {
	switch role {
	case "owner", "admin", "analyst":
		return true
	default:
		return false
	}
}

func requireWrite(w http.ResponseWriter, role string) bool {
	if canWrite(role) {
		return true
	}
	httpx.Error(w, 403, "forbidden", "apenas analistas ou administradores podem alterar actualizações automáticas")
	return false
}

func (s *Server) listSchedules(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	kind := r.URL.Query().Get("kind")
	var target uuid.UUID
	if t := r.URL.Query().Get("target_id"); t != "" {
		id, err := uuid.Parse(t)
		if err != nil {
			httpx.Error(w, 400, "invalid", "target_id inválido")
			return
		}
		target = id
	}
	list, err := s.sched.List(r.Context(), org, ws, kind, target)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) getSchedule(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	sc, err := s.sched.Get(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", "agendamento não encontrado")
		return
	}
	httpx.JSON(w, 200, sc)
}

func (s *Server) upsertSchedule(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if !requireWrite(w, role) {
		return
	}
	var body scheduler.Input
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if err := s.ensureScheduleTarget(r.Context(), org, ws, body.Kind, body.TargetID); err != nil {
		httpx.Error(w, 400, "invalid_target", err.Error())
		return
	}
	sc, err := s.sched.Upsert(r.Context(), org, ws, uid, body)
	if err != nil {
		httpx.Error(w, 400, "save_failed", err.Error())
		return
	}
	s.audit(r, "SYNC_SCHEDULE_UPSERT", "sync_schedule", sc.ID, map[string]any{"kind": sc.Kind, "target_id": sc.TargetID})
	httpx.JSON(w, 200, sc)
}

func (s *Server) patchSchedule(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if !requireWrite(w, role) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	var body scheduler.Input
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	sc, err := s.sched.Patch(r.Context(), org, ws, id, body)
	if err != nil {
		httpx.Error(w, 400, "save_failed", err.Error())
		return
	}
	s.audit(r, "SYNC_SCHEDULE_UPDATED", "sync_schedule", sc.ID, nil)
	httpx.JSON(w, 200, sc)
}

func (s *Server) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if !requireWrite(w, role) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	if err := s.sched.Delete(r.Context(), org, ws, id); err != nil {
		httpx.Error(w, 404, "not_found", "agendamento não encontrado")
		return
	}
	s.audit(r, "SYNC_SCHEDULE_DELETED", "sync_schedule", id, nil)
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) pauseSchedule(w http.ResponseWriter, r *http.Request) {
	s.setScheduleEnabled(w, r, false)
}

func (s *Server) resumeSchedule(w http.ResponseWriter, r *http.Request) {
	s.setScheduleEnabled(w, r, true)
}

func (s *Server) setScheduleEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	_, org, ws, role := principal(r)
	if !requireWrite(w, role) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	sc, err := s.sched.SetEnabled(r.Context(), org, ws, id, enabled)
	if err != nil {
		httpx.Error(w, 404, "not_found", "agendamento não encontrado")
		return
	}
	action := "SYNC_SCHEDULE_PAUSED"
	if enabled {
		action = "SYNC_SCHEDULE_RESUMED"
	}
	s.audit(r, action, "sync_schedule", sc.ID, nil)
	httpx.JSON(w, 200, sc)
}

func (s *Server) runScheduleNow(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if !requireWrite(w, role) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	sc, err := s.sched.Get(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", "agendamento não encontrado")
		return
	}
	s.schedRun.Execute(r.Context(), sc, true)
	out, err := s.sched.Get(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	s.audit(r, "SYNC_SCHEDULE_RUN", "sync_schedule", id, map[string]any{"status": out.LastStatus})
	httpx.JSON(w, 200, out)
}

func (s *Server) listScheduleRuns(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	list, err := s.sched.ListRuns(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", "agendamento não encontrado")
		return
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) ensureScheduleTarget(ctx context.Context, org, ws uuid.UUID, kind string, target uuid.UUID) error {
	if !scheduler.ValidKind(kind) {
		return fmt.Errorf("tipo inválido")
	}
	if target == uuid.Nil {
		return fmt.Errorf("alvo obrigatório")
	}
	var n int
	switch kind {
	case "connector":
		_ = s.deps.PG.QueryRow(ctx, `SELECT COUNT(*) FROM data_sources WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, target, org, ws).Scan(&n)
		if n == 0 {
			return fmt.Errorf("conector não encontrado")
		}
	case "flow":
		_ = s.deps.PG.QueryRow(ctx, `SELECT COUNT(*) FROM flows WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, target, org, ws).Scan(&n)
		if n == 0 {
			return fmt.Errorf("flow não encontrado")
		}
	case "dataset":
		_ = s.deps.PG.QueryRow(ctx, `SELECT COUNT(*) FROM datasets WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, target, org, ws).Scan(&n)
		if n == 0 {
			return fmt.Errorf("conjunto não encontrado")
		}
	}
	return nil
}

func (s *Server) runScheduledConnector(ctx context.Context, orgID, wsID, userID, sourceID uuid.UUID, table string, incremental bool) (scheduler.JobResult, error) {
	typ, cfgTable, err := s.ingest.SourceMeta(ctx, orgID, wsID, sourceID)
	if err != nil {
		return scheduler.JobResult{}, err
	}
	if table == "" {
		table = cfgTable
	}
	if incremental && scheduler.CDCCapable(typ) {
		ds, dsErr := s.ingest.LatestDatasetForSource(ctx, orgID, wsID, sourceID)
		if dsErr != nil || ds == uuid.Nil {
			if err := s.ent.Check(ctx, orgID, "dataset"); err != nil {
				return scheduler.JobResult{}, err
			}
			res, err := s.ingest.RefreshSource(ctx, orgID, wsID, userID, sourceID, table, "")
			if err != nil {
				return scheduler.JobResult{}, err
			}
			if table != "" {
				_ = s.cdc.Enable(ctx, orgID, wsID, sourceID, res.DatasetID, table)
			}
			s.lineage.RecordIngest(ctx, orgID, wsID, sourceID, res.DatasetID, res.Name, res.Name)
			n := res.RowCount
			return scheduler.JobResult{Mode: "full", Rows: &n}, nil
		}
		if table != "" && !s.cdc.HasCheckpoint(ctx, orgID, wsID, sourceID) {
			_ = s.cdc.Enable(ctx, orgID, wsID, sourceID, ds, table)
		}
		if s.cdc.HasCheckpoint(ctx, orgID, wsID, sourceID) {
			applied, err := s.cdc.PollSource(ctx, orgID, wsID, sourceID)
			if err != nil {
				return scheduler.JobResult{}, err
			}
			n := int64(applied)
			return scheduler.JobResult{Mode: "incremental", Rows: &n}, nil
		}
	}
	if _, err := s.ingest.LatestDatasetForSource(ctx, orgID, wsID, sourceID); err != nil {
		if err := s.ent.Check(ctx, orgID, "dataset"); err != nil {
			return scheduler.JobResult{}, err
		}
	}
	res, err := s.ingest.RefreshSource(ctx, orgID, wsID, userID, sourceID, table, "")
	if err != nil {
		return scheduler.JobResult{}, err
	}
	if res.Created {
		s.lineage.RecordIngest(ctx, orgID, wsID, sourceID, res.DatasetID, res.Name, res.Name)
	}
	n := res.RowCount
	return scheduler.JobResult{Mode: "full", Rows: &n}, nil
}

func (s *Server) runScheduledFlow(ctx context.Context, orgID, wsID, userID, flowID uuid.UUID) (scheduler.JobResult, error) {
	if _, err := s.flow.Get(ctx, orgID, wsID, flowID); err != nil {
		return scheduler.JobResult{}, fmt.Errorf("flow não encontrado")
	}
	runID, err := s.flow.CreateRun(ctx, flow.Run{FlowID: flowID, Status: "pending"})
	if err != nil {
		return scheduler.JobResult{}, err
	}
	reader := func(datasetID string, limit int) ([]string, []map[string]any, error) {
		return s.query.ReadRows(ctx, orgID, wsID, datasetID, limit)
	}
	sum, err := s.flowEng.Execute(ctx, runID, userID, reader)
	n := sum.Rows
	return scheduler.JobResult{Mode: "full", Rows: &n}, err
}

func (s *Server) runScheduledDataset(ctx context.Context, orgID, wsID, userID, datasetID uuid.UUID, incremental bool) (scheduler.JobResult, error) {
	src, _, err := s.ingest.DatasetSource(ctx, orgID, wsID, datasetID)
	if err != nil {
		return scheduler.JobResult{}, err
	}
	return s.runScheduledConnector(ctx, orgID, wsID, userID, src, "", incremental)
}
