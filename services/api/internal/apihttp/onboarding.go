package apihttp

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/authn"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
)

type onboardingTask struct {
	ID   string `json:"id"`
	Done bool   `json:"done"`
}

type onboardingStatus struct {
	authn.OnboardingState
	Tasks         []onboardingTask `json:"tasks"`
	JustCompleted bool             `json:"just_completed"`
}

// onboardingTasksFor detecta o progresso a partir do que existe na conta, para o guia marcar-se sozinho.
func (s *Server) onboardingTasksFor(ctx context.Context, uid, org, ws uuid.UUID, role string) []onboardingTask {
	exists := func(q string, args ...any) bool {
		var ok bool
		_ = s.deps.PG.QueryRow(ctx, q, args...).Scan(&ok)
		return ok
	}
	data := exists(`SELECT EXISTS(SELECT 1 FROM datasets WHERE org_id=$1 AND workspace_id=$2 AND status='ready')`, org, ws)
	dashboard := exists(`SELECT EXISTS(SELECT 1 FROM dashboards WHERE org_id=$1 AND workspace_id=$2)`, org, ws)
	ask := exists(`
		SELECT EXISTS(
			SELECT 1 FROM ai_conversations c JOIN ai_messages m ON m.conversation_id=c.id
			WHERE c.org_id=$1 AND c.user_id=$2
		)`, org, uid)
	automate := exists(`
		SELECT EXISTS(SELECT 1 FROM alerts WHERE org_id=$1 AND workspace_id=$2)
		    OR EXISTS(SELECT 1 FROM reports WHERE org_id=$1 AND workspace_id=$2)
		    OR EXISTS(SELECT 1 FROM sync_schedules WHERE org_id=$1 AND workspace_id=$2 AND enabled)`, org, ws)
	team := exists(`
		SELECT (SELECT COUNT(*) FROM organization_members WHERE org_id=$1) > 1
		    OR EXISTS(SELECT 1 FROM organization_invites WHERE org_id=$1 AND accepted_at IS NULL AND expires_at > now())`, org)

	switch role {
	case "viewer":
		return []onboardingTask{{"dashboard", dashboard}, {"ask", ask}}
	case "analyst":
		return []onboardingTask{{"data", data}, {"dashboard", dashboard}, {"ask", ask}, {"automate", automate}}
	default:
		return []onboardingTask{{"data", data}, {"dashboard", dashboard}, {"ask", ask}, {"automate", automate}, {"team", team}}
	}
}

func (s *Server) onboardingGet(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	st, err := s.auth.GetOnboarding(r.Context(), uid)
	if err != nil {
		httpx.Error(w, 400, "onboarding_failed", err.Error())
		return
	}
	out := onboardingStatus{OnboardingState: st}
	if !st.Completed {
		out.Tasks = s.onboardingTasksFor(r.Context(), uid, org, ws, role)
		allDone := len(out.Tasks) > 0
		for _, t := range out.Tasks {
			allDone = allDone && t.Done
		}
		if allDone && s.auth.CompleteOnboarding(r.Context(), uid) == nil {
			out.Step, out.Completed, out.Dismissed, out.JustCompleted = "done", true, true, true
		}
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) onboardingStep(w http.ResponseWriter, r *http.Request) {
	uid, _, _, _ := principal(r)
	var body struct {
		Step string `json:"step"`
	}
	_ = httpx.Decode(r, &body)
	if err := s.auth.SetOnboardingStep(r.Context(), uid, body.Step); err != nil {
		httpx.Error(w, 400, "onboarding_failed", err.Error())
		return
	}
	s.onboardingState(w, r, uid)
}

func (s *Server) onboardingDismiss(w http.ResponseWriter, r *http.Request) {
	uid, _, _, _ := principal(r)
	if err := s.auth.DismissOnboarding(r.Context(), uid); err != nil {
		httpx.Error(w, 400, "onboarding_failed", err.Error())
		return
	}
	s.onboardingState(w, r, uid)
}

func (s *Server) onboardingComplete(w http.ResponseWriter, r *http.Request) {
	uid, _, _, _ := principal(r)
	if err := s.auth.CompleteOnboarding(r.Context(), uid); err != nil {
		httpx.Error(w, 400, "onboarding_failed", err.Error())
		return
	}
	s.onboardingState(w, r, uid)
}

func (s *Server) onboardingReset(w http.ResponseWriter, r *http.Request) {
	uid, _, _, _ := principal(r)
	if err := s.auth.ResetOnboarding(r.Context(), uid); err != nil {
		httpx.Error(w, 400, "onboarding_failed", err.Error())
		return
	}
	s.onboardingState(w, r, uid)
}

func (s *Server) onboardingState(w http.ResponseWriter, r *http.Request, uid uuid.UUID) {
	st, err := s.auth.GetOnboarding(r.Context(), uid)
	if err != nil {
		httpx.Error(w, 400, "onboarding_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, st)
}
