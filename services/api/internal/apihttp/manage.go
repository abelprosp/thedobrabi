package apihttp

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
	"github.com/thedobra/thedobra/services/api/internal/manage"
)

func (s *Server) manageHome(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	out, err := s.manage.Home(r.Context(), org, ws, uid, role)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	s.manageHome(w, r)
}

func (s *Server) patchManageCadence(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if !requireAnalyst(w, role) {
		return
	}
	var body struct {
		Cadence       string   `json:"cadence"`
		WatchMeasures []string `json:"watch_measures"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if err := s.manage.SetCadence(r.Context(), org, ws, body.Cadence, body.WatchMeasures); err != nil {
		httpx.Error(w, 400, "invalid", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"cadence": body.Cadence, "watch_measures": body.WatchMeasures})
}

func (s *Server) manageRadar(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	var ds uuid.UUID
	if err := s.deps.PG.QueryRow(r.Context(), `SELECT id FROM datasets WHERE org_id=$1 AND workspace_id=$2 AND status='ready' ORDER BY updated_at DESC LIMIT 1`, org, ws).Scan(&ds); err == nil {
		if brief, err := s.intel.AnalyzeDataset(r.Context(), org, ws, uid, ds, role); err == nil {
			_ = s.manage.SyncFromBrief(r.Context(), org, ws, ds, brief)
		}
	}
	list, err := s.manage.Radar(r.Context(), org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) manageRadarFeedback(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if !requireAnalyst(w, role) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "descoberta inválida")
		return
	}
	var body struct {
		Feedback string `json:"feedback"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if err := s.manage.Feedback(r.Context(), org, ws, id, body.Feedback); err != nil {
		httpx.Error(w, 400, "invalid", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id, "feedback": body.Feedback})
}

func (s *Server) manageListActions(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	list, err := s.manage.ListActions(r.Context(), org, ws, r.URL.Query().Get("status"))
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) manageCreateAction(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if !requireAnalyst(w, role) {
		return
	}
	var body manage.CreateActionInput
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	a, err := s.manage.CreateAction(r.Context(), org, ws, uid, role, body)
	if err != nil {
		httpx.Error(w, 400, "invalid", err.Error())
		return
	}
	s.audit(r, "MANAGE_ACTION_CREATED", "manage_action", uuid.MustParse(a.ID), map[string]any{"title": a.Title})
	httpx.JSON(w, 201, a)
}

func (s *Server) manageGetAction(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "acção inválida")
		return
	}
	a, reviews, err := s.manage.GetAction(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"action": a, "reviews": reviews})
}

func (s *Server) managePatchAction(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if !requireAnalyst(w, role) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "acção inválida")
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if err := s.manage.PatchAction(r.Context(), org, ws, id, body.Status); err != nil {
		httpx.Error(w, 400, "invalid", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": id, "status": body.Status})
}

func (s *Server) manageReviewAction(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if !requireAnalyst(w, role) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "acção inválida")
		return
	}
	var body manage.ReviewActionInput
	_ = httpx.Decode(r, &body)
	rev, err := s.manage.ReviewAction(r.Context(), org, ws, uid, role, id, body)
	if err != nil {
		httpx.Error(w, 400, "invalid", err.Error())
		return
	}
	httpx.JSON(w, 201, rev)
}

func (s *Server) manageListReviews(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	list, err := s.manage.ListReviews(r.Context(), org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) manageCreateReview(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if !requireAnalyst(w, role) {
		return
	}
	var body manage.SaveReviewInput
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	rev, err := s.manage.SaveReview(r.Context(), org, ws, uid, body)
	if err != nil {
		httpx.Error(w, 400, "invalid", err.Error())
		return
	}
	httpx.JSON(w, 201, rev)
}

func (s *Server) manageGetReview(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "revisão inválida")
		return
	}
	rev, err := s.manage.GetReview(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", "revisão não encontrada")
		return
	}
	httpx.JSON(w, 200, rev)
}

func (s *Server) publicWeekReview(w http.ResponseWriter, r *http.Request) {
	tok := strings.TrimSpace(chi.URLParam(r, "token"))
	out, err := s.manage.PublicReview(r.Context(), tok)
	if err != nil {
		httpx.Error(w, 404, "not_found", err.Error())
		return
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) manageListMemories(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	list, err := s.manage.ListMemories(r.Context(), org, ws, 50)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) manageCreateMemory(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if !requireAnalyst(w, role) {
		return
	}
	var body struct {
		DatasetID  string `json:"dataset_id"`
		Note       string `json:"note"`
		ChartKey   string `json:"chart_key"`
		OccurredOn string `json:"occurred_on"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	m, err := s.manage.CreateMemory(r.Context(), org, ws, uid, body.DatasetID, body.Note, body.ChartKey, body.OccurredOn)
	if err != nil {
		httpx.Error(w, 400, "invalid", err.Error())
		return
	}
	httpx.JSON(w, 201, m)
}

func (s *Server) manageDeleteMemory(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if !requireAnalyst(w, role) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "memória inválida")
		return
	}
	if err := s.manage.DeleteMemory(r.Context(), org, ws, id); err != nil {
		httpx.Error(w, 404, "not_found", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) manageListGoals(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	list, err := s.manage.ListGoalPaces(r.Context(), org, ws, uid, role)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) manageCreateGoal(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if !requireAnalyst(w, role) {
		return
	}
	var body struct {
		DatasetID  string  `json:"dataset_id"`
		MetricName string  `json:"metric_name"`
		Name       string  `json:"name"`
		Target     float64 `json:"target_value"`
		Period     string  `json:"period"`
		OwnerID    string  `json:"owner_id"`
		StartsAt   string  `json:"starts_at"`
		EndsAt     string  `json:"ends_at"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	id, err := s.manage.CreateGoal(r.Context(), org, ws, uid, body.DatasetID, body.MetricName, body.Name, body.Target, body.Period, body.OwnerID, body.StartsAt, body.EndsAt)
	if err != nil {
		httpx.Error(w, 400, "invalid", err.Error())
		return
	}
	httpx.JSON(w, 201, map[string]any{"id": id})
}
