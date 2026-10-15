package authn

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

var onboardingSteps = []string{
	"welcome",
	"data",
	"dashboard",
	"ask",
	"team",
	"done",
}

type OnboardingState struct {
	Step      string `json:"onboarding_step"`
	Completed bool   `json:"onboarding_completed"`
	Dismissed bool   `json:"onboarding_dismissed"`
}

func (s *Service) GetOnboarding(ctx context.Context, userID uuid.UUID) (OnboardingState, error) {
	var st OnboardingState
	err := s.pg.QueryRow(ctx, `
		SELECT onboarding_step, onboarding_completed_at IS NOT NULL, onboarding_dismissed_at IS NOT NULL
		FROM users WHERE id=$1
	`, userID).Scan(&st.Step, &st.Completed, &st.Dismissed)
	return st, err
}

// SetOnboardingStep guarda o passo atual do assistente; marca também como visto.
func (s *Service) SetOnboardingStep(ctx context.Context, userID uuid.UUID, step string) error {
	if !validOnboardingStep(step) {
		return fmt.Errorf("passo de onboarding inválido")
	}
	if step == "done" {
		return s.CompleteOnboarding(ctx, userID)
	}
	_, err := s.pg.Exec(ctx, `
		UPDATE users SET onboarding_step=$2, onboarding_dismissed_at=COALESCE(onboarding_dismissed_at, now()), updated_at=now()
		WHERE id=$1 AND onboarding_completed_at IS NULL
	`, userID, step)
	return err
}

// DismissOnboarding impede que o assistente volte a abrir sozinho; o guia na visão geral continua.
func (s *Service) DismissOnboarding(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pg.Exec(ctx, `
		UPDATE users SET onboarding_dismissed_at=COALESCE(onboarding_dismissed_at, now()), updated_at=now()
		WHERE id=$1
	`, userID)
	return err
}

func (s *Service) CompleteOnboarding(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pg.Exec(ctx, `
		UPDATE users SET onboarding_step='done',
			onboarding_completed_at=COALESCE(onboarding_completed_at, now()),
			onboarding_dismissed_at=COALESCE(onboarding_dismissed_at, now()),
			updated_at=now()
		WHERE id=$1
	`, userID)
	return err
}

func (s *Service) ResetOnboarding(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pg.Exec(ctx, `
		UPDATE users SET onboarding_step='welcome', onboarding_completed_at=NULL, onboarding_dismissed_at=NULL, updated_at=now()
		WHERE id=$1
	`, userID)
	return err
}

func validOnboardingStep(step string) bool {
	for _, s := range onboardingSteps {
		if s == step {
			return true
		}
	}
	return false
}
