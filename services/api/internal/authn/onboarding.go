package authn

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var onboardingSteps = []string{
	"welcome",
	"connect_data",
	"create_metric",
	"build_dashboard",
	"ask_ai",
	"invite_team",
	"done",
}

func (s *Service) NextOnboardingStep(ctx context.Context, orgID uuid.UUID, step string) (string, error) {
	if step == "" {
		var current string
		err := s.pg.QueryRow(ctx, `SELECT onboarding_step FROM organizations WHERE id=$1`, orgID).Scan(&current)
		if err != nil {
			return "", err
		}
		step = current
	}
	next := nextStep(step)
	_, err := s.pg.Exec(ctx, `
		UPDATE organizations SET onboarding_step=$2, onboarding_seen=TRUE, updated_at=now()
		WHERE id=$1 AND onboarding_completed_at IS NULL
	`, orgID, next)
	return next, err
}

func (s *Service) CompleteOnboarding(ctx context.Context, orgID uuid.UUID) error {
	_, err := s.pg.Exec(ctx, `
		UPDATE organizations SET onboarding_step='done', onboarding_completed_at=now(), onboarding_seen=TRUE, updated_at=now()
		WHERE id=$1
	`, orgID)
	return err
}

func (s *Service) ResetOnboarding(ctx context.Context, orgID uuid.UUID) error {
	_, err := s.pg.Exec(ctx, `
		UPDATE organizations SET onboarding_step='welcome', onboarding_completed_at=NULL, onboarding_seen=FALSE, updated_at=now()
		WHERE id=$1
	`, orgID)
	return err
}

func (s *Service) GetOnboardingStep(ctx context.Context, orgID uuid.UUID) (string, bool, bool, error) {
	var step string
	var completedAt *time.Time
	var seen bool
	err := s.pg.QueryRow(ctx, `
		SELECT onboarding_step, onboarding_completed_at, onboarding_seen
		FROM organizations WHERE id=$1
	`, orgID).Scan(&step, &completedAt, &seen)
	if err != nil {
		return "", false, false, err
	}
	return step, completedAt != nil, seen, nil
}

func nextStep(current string) string {
	for i, s := range onboardingSteps {
		if s == current && i+1 < len(onboardingSteps) {
			return onboardingSteps[i+1]
		}
	}
	if current == "" {
		return onboardingSteps[0]
	}
	return current
}

func (s *Service) UpdateOnboardingStep(ctx context.Context, orgID uuid.UUID, step string) error {
	valid := false
	for _, s := range onboardingSteps {
		if s == step {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("passo de onboarding inválido")
	}
	_, err := s.pg.Exec(ctx, `
		UPDATE organizations SET onboarding_step=$2, onboarding_seen=TRUE, updated_at=now()
		WHERE id=$1
	`, orgID, step)
	return err
}
