package apihttp

import (
	"net/http"

	"github.com/thedobra/thedobra/services/api/internal/httpx"
)

func (s *Server) onboardingNext(w http.ResponseWriter, r *http.Request) {
	_, org, _, _ := principal(r)
	var body struct {
		Step string `json:"step"`
	}
	_ = httpx.Decode(r, &body)
	next, err := s.auth.NextOnboardingStep(r.Context(), org, body.Step)
	if err != nil {
		httpx.Error(w, 400, "onboarding_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"onboarding_step": next, "onboarding_completed": next == "done"})
}

func (s *Server) onboardingComplete(w http.ResponseWriter, r *http.Request) {
	_, org, _, _ := principal(r)
	if err := s.auth.CompleteOnboarding(r.Context(), org); err != nil {
		httpx.Error(w, 400, "onboarding_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"onboarding_step": "done", "onboarding_completed": true})
}

func (s *Server) onboardingReset(w http.ResponseWriter, r *http.Request) {
	_, org, _, role := principal(r)
	if role != "owner" && role != "admin" {
		httpx.Error(w, 403, "forbidden", "apenas admin")
		return
	}
	if err := s.auth.ResetOnboarding(r.Context(), org); err != nil {
		httpx.Error(w, 400, "onboarding_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"onboarding_step": "welcome", "onboarding_completed": false})
}
