package entitlements

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Limits struct {
	Users    int
	Datasets int
	Queries  int
	AI       int
}

func ForPlan(plan string) Limits {
	switch plan {
	case "growth":
		return Limits{Users: 20, Datasets: 50, Queries: 100000, AI: 5000}
	case "business":
		return Limits{Users: 100, Datasets: 200, Queries: 1000000, AI: 25000}
	case "enterprise":
		return Limits{Users: -1, Datasets: -1, Queries: -1, AI: -1}
	default:
		return Limits{Users: 5, Datasets: 10, Queries: 10000, AI: 500}
	}
}

type Service struct{ pg *pgxpool.Pool }

func New(pg *pgxpool.Pool) *Service { return &Service{pg: pg} }

func (s *Service) Plan(ctx context.Context, orgID uuid.UUID) string {
	var plan string
	_ = s.pg.QueryRow(ctx, `SELECT plan FROM organizations WHERE id=$1`, orgID).Scan(&plan)
	if plan == "" {
		return "starter"
	}
	return plan
}

func (s *Service) Limits(ctx context.Context, orgID uuid.UUID) Limits {
	return ForPlan(s.Plan(ctx, orgID))
}

func (s *Service) Check(ctx context.Context, orgID uuid.UUID, kind string) error {
	lim := s.Limits(ctx, orgID)
	switch kind {
	case "query":
		if lim.Queries < 0 {
			return nil
		}
		var n int
		_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM query_history WHERE org_id=$1 AND created_at > date_trunc('month', now())`, orgID).Scan(&n)
		if n >= lim.Queries {
			return fmt.Errorf("limite mensal de consultas do plano atingido (%d)", lim.Queries)
		}
	case "dataset":
		if lim.Datasets < 0 {
			return nil
		}
		var n int
		_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM datasets WHERE org_id=$1`, orgID).Scan(&n)
		if n >= lim.Datasets {
			return fmt.Errorf("limite de conjuntos do plano atingido (%d)", lim.Datasets)
		}
	case "ai":
		if lim.AI < 0 {
			return nil
		}
		var n int
		_ = s.pg.QueryRow(ctx, `
			SELECT COUNT(*) FROM ai_messages m
			JOIN ai_conversations c ON c.id=m.conversation_id
			WHERE c.org_id=$1 AND m.created_at > date_trunc('month', now())
		`, orgID).Scan(&n)
		if n >= lim.AI {
			return fmt.Errorf("limite mensal de mensagens IA do plano atingido (%d)", lim.AI)
		}
	case "user":
		if lim.Users < 0 {
			return nil
		}
		var n int
		_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM organization_members WHERE org_id=$1`, orgID).Scan(&n)
		if n >= lim.Users {
			return fmt.Errorf("limite de utilizadores do plano atingido (%d)", lim.Users)
		}
	}
	return nil
}

func (s *Service) Snapshot(ctx context.Context, orgID, wsID uuid.UUID) map[string]any {
	plan := s.Plan(ctx, orgID)
	lim := ForPlan(plan)
	var queries, datasets, ai, users int
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM query_history WHERE org_id=$1 AND created_at > date_trunc('month', now())`, orgID).Scan(&queries)
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM datasets WHERE org_id=$1`, orgID).Scan(&datasets)
	_ = s.pg.QueryRow(ctx, `
		SELECT COUNT(*) FROM ai_messages m JOIN ai_conversations c ON c.id=m.conversation_id
		WHERE c.org_id=$1 AND m.created_at > date_trunc('month', now())
	`, orgID).Scan(&ai)
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM organization_members WHERE org_id=$1`, orgID).Scan(&users)
	_ = wsID
	return map[string]any{
		"plan": plan, "queries": queries, "datasets": datasets, "ai_messages": ai, "users": users,
		"limits": map[string]int{"users": lim.Users, "datasets": lim.Datasets, "queries": lim.Queries, "ai": lim.AI},
	}
}
