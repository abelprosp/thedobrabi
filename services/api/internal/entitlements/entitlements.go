package entitlements

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	PlanEssencial = "essencial"
	PlanPro       = "pro"
	PlanCompleto  = "completo"

	TierBasic = "basic"
	TierPlus  = "plus"
	TierAll   = "all"
)

var platformEmails = map[string]bool{
	"redobrai@gmail.com": true,
	"redorai@gmail.com":  true,
}

func IsPlatformEmail(email string) bool {
	return platformEmails[strings.ToLower(strings.TrimSpace(email))]
}

type Limits struct {
	Users         int
	Datasets      int
	Queries       int
	AI            int
	Dashboards    int
	Connectors    int
	ConnectorTier string
	Whitelabel    bool
	Plan          string
	Name          string
	PriceBRL      int
	Trial         bool
	TrialEndsAt   *time.Time
	DaysLeft      int
}

type PlanDef struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	PriceBRL      int      `json:"price_brl"`
	Period        string   `json:"period"`
	Users         int      `json:"users"`
	Datasets      int      `json:"datasets"`
	Queries       int      `json:"queries"`
	AI            int      `json:"ai"`
	Dashboards    int      `json:"dashboards"`
	Connectors    int      `json:"connectors"`
	ConnectorTier string   `json:"connector_tier"`
	Whitelabel    bool     `json:"whitelabel"`
	Highlights    []string `json:"highlights"`
}

func Catalog() []PlanDef {
	return []PlanDef{
		{
			ID: PlanEssencial, Name: "Essencial", PriceBRL: 97, Period: "mês",
			Users: 3, Datasets: 10, Queries: 20000, AI: 0, Dashboards: 5, Connectors: 5,
			ConnectorTier: TierBasic, Whitelabel: false,
			Highlights: []string{"5 dashboards", "5 conectores básicos (CSV, Excel, JSON, Postgres, MySQL)", "Sem créditos de IA", "Até 3 utilizadores"},
		},
		{
			ID: PlanPro, Name: "Pro", PriceBRL: 129, Period: "mês",
			Users: 10, Datasets: 40, Queries: 100000, AI: 1500, Dashboards: 25, Connectors: 15,
			ConnectorTier: TierPlus, Whitelabel: false,
			Highlights: []string{"25 dashboards", "15 conectores (básicos + ERPs, APIs e Google Sheets)", "1.500 créditos de IA / mês", "Até 10 utilizadores"},
		},
		{
			ID: PlanCompleto, Name: "Completo", PriceBRL: 297, Period: "mês",
			Users: -1, Datasets: -1, Queries: -1, AI: -1, Dashboards: -1, Connectors: -1,
			ConnectorTier: TierAll, Whitelabel: true,
			Highlights: []string{"Dashboards e conectores ilimitados", "IA ilimitada", "White-label (logo e nome)", "Todos os conectores"},
		},
	}
}

func NormalizePlan(plan string) string {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case PlanPro, "growth":
		return PlanPro
	case PlanCompleto, "business", "enterprise":
		return PlanCompleto
	default:
		return PlanEssencial
	}
}

func DefFor(plan string) PlanDef {
	id := NormalizePlan(plan)
	for _, p := range Catalog() {
		if p.ID == id {
			return p
		}
	}
	return Catalog()[0]
}

func ForPlan(plan string) Limits {
	d := DefFor(plan)
	return Limits{
		Users: d.Users, Datasets: d.Datasets, Queries: d.Queries, AI: d.AI,
		Dashboards: d.Dashboards, Connectors: d.Connectors, ConnectorTier: d.ConnectorTier,
		Whitelabel: d.Whitelabel, Plan: d.ID, Name: d.Name, PriceBRL: d.PriceBRL,
	}
}

var basicConnectors = map[string]bool{
	"csv": true, "xlsx": true, "json": true, "postgres": true, "mysql": true,
	"manual": true,
}

var plusConnectors = map[string]bool{
	"csv": true, "xlsx": true, "json": true, "pdf": true, "postgres": true, "mysql": true, "manual": true,
	"mariadb": true, "supabase": true, "sqlserver": true, "rest": true, "url": true,
	"asaas": true, "conta_azul": true, "google_analytics": true, "google_sheets": true, "stripe": true,
}

func ConnectorAllowed(tier, typ string) bool {
	t := strings.ToLower(strings.TrimSpace(typ))
	switch strings.ToLower(tier) {
	case TierAll:
		return true
	case TierPlus:
		return plusConnectors[t]
	default:
		return basicConnectors[t]
	}
}

type TrialFeatures struct {
	Plan          string `json:"plan,omitempty"`
	AI            *bool  `json:"ai,omitempty"`
	Whitelabel    *bool  `json:"whitelabel,omitempty"`
	Dashboards    *int   `json:"dashboards,omitempty"`
	Connectors    *int   `json:"connectors,omitempty"`
	ConnectorTier string `json:"connector_tier,omitempty"`
}

func applyTrial(base Limits, raw []byte) Limits {
	var f TrialFeatures
	_ = json.Unmarshal(raw, &f)
	out := base
	if f.Plan != "" {
		out = ForPlan(f.Plan)
	}
	if f.AI != nil {
		if *f.AI {
			if out.AI == 0 {
				out.AI = ForPlan(PlanPro).AI
			}
		} else {
			out.AI = 0
		}
	}
	if f.Whitelabel != nil {
		out.Whitelabel = *f.Whitelabel
	}
	if f.Dashboards != nil {
		out.Dashboards = *f.Dashboards
	}
	if f.Connectors != nil {
		out.Connectors = *f.Connectors
	}
	if f.ConnectorTier != "" {
		out.ConnectorTier = f.ConnectorTier
	}
	out.Trial = true
	return out
}

type Service struct{ pg *pgxpool.Pool }

func New(pg *pgxpool.Pool) *Service { return &Service{pg: pg} }

func (s *Service) StoredPlan(ctx context.Context, orgID uuid.UUID) string {
	var plan string
	_ = s.pg.QueryRow(ctx, `SELECT plan FROM organizations WHERE id=$1`, orgID).Scan(&plan)
	return NormalizePlan(plan)
}

func (s *Service) Plan(ctx context.Context, orgID uuid.UUID) string {
	return s.Limits(ctx, orgID).Plan
}

func (s *Service) Limits(ctx context.Context, orgID uuid.UUID) Limits {
	var plan string
	var trialEnds *time.Time
	var features []byte
	_ = s.pg.QueryRow(ctx, `
		SELECT plan, trial_ends_at, trial_features FROM organizations WHERE id=$1
	`, orgID).Scan(&plan, &trialEnds, &features)
	lim := ForPlan(plan)
	if trialEnds != nil && trialEnds.After(time.Now()) {
		lim = applyTrial(lim, features)
		lim.TrialEndsAt = trialEnds
		d := int(time.Until(*trialEnds).Hours() / 24)
		if d < 0 {
			d = 0
		}
		lim.DaysLeft = d + 1
		if time.Until(*trialEnds) < time.Hour {
			lim.DaysLeft = 1
		}
	}
	return lim
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
			return fmt.Errorf("limite mensal de consultas do plano %s atingido (%d)", lim.Name, lim.Queries)
		}
	case "dataset":
		if lim.Datasets < 0 {
			return nil
		}
		var n int
		_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM datasets WHERE org_id=$1`, orgID).Scan(&n)
		if n >= lim.Datasets {
			return fmt.Errorf("limite de conjuntos do plano %s atingido (%d)", lim.Name, lim.Datasets)
		}
	case "dashboard":
		if lim.Dashboards < mar0(lim.Dashboards) && lim.Dashboards >= 0 {
			var n int
			_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM dashboards WHERE org_id=$1`, orgID).Scan(&n)
			if n >= lim.Dashboards {
				return fmt.Errorf("o plano %s inclui %d dashboards. Peça um upgrade ou um teste de 7 dias", lim.Name, lim.Dashboards)
			}
		}
	case "connector":
		if lim.Connectors >= 0 {
			var n int
			_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM data_sources WHERE org_id=$1`, orgID).Scan(&n)
			if n >= lim.Connectors {
				return fmt.Errorf("o plano %s inclui %d conectores. Peça um upgrade para ligar mais fontes", lim.Name, lim.Connectors)
			}
		}
	case "ai":
		if lim.AI == 0 {
			return fmt.Errorf("o plano %s não inclui créditos de IA. O plano Pro (R$ 129) já traz créditos", lim.Name)
		}
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
			return fmt.Errorf("limite mensal de mensagens IA do plano %s atingido (%d)", lim.Name, lim.AI)
		}
	case "user":
		if lim.Users < 0 {
			return nil
		}
		var n int
		_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM organization_members WHERE org_id=$1`, orgID).Scan(&n)
		if n >= lim.Users {
			return fmt.Errorf("limite de utilizadores do plano %s atingido (%d)", lim.Name, lim.Users)
		}
	case "whitelabel":
		if !lim.Whitelabel {
			return fmt.Errorf("white-label está no plano Completo (R$ 297)")
		}
	}
	return nil
}

func mar0(n int) int { return n } // keep Check dashboard readable; dashboards >= 0 is the real gate

func (s *Service) CheckConnector(ctx context.Context, orgID uuid.UUID, typ string) error {
	lim := s.Limits(ctx, orgID)
	if !ConnectorAllowed(lim.ConnectorTier, typ) {
		return fmt.Errorf("o conector «%s» não está no plano %s. No Essencial só CSV, Excel, JSON, Postgres e MySQL", typ, lim.Name)
	}
	return s.Check(ctx, orgID, "connector")
}

func (s *Service) Snapshot(ctx context.Context, orgID, wsID uuid.UUID) map[string]any {
	lim := s.Limits(ctx, orgID)
	var queries, datasets, ai, users, dashboards, connectors int
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM query_history WHERE org_id=$1 AND created_at > date_trunc('month', now())`, orgID).Scan(&queries)
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM datasets WHERE org_id=$1`, orgID).Scan(&datasets)
	_ = s.pg.QueryRow(ctx, `
		SELECT COUNT(*) FROM ai_messages m JOIN ai_conversations c ON c.id=m.conversation_id
		WHERE c.org_id=$1 AND m.created_at > date_trunc('month', now())
	`, orgID).Scan(&ai)
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM organization_members WHERE org_id=$1`, orgID).Scan(&users)
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM dashboards WHERE org_id=$1`, orgID).Scan(&dashboards)
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM data_sources WHERE org_id=$1`, orgID).Scan(&connectors)
	_ = wsID
	out := map[string]any{
		"plan":            lim.Plan,
		"plan_name":       lim.Name,
		"price_brl":       lim.PriceBRL,
		"trial":           lim.Trial,
		"trial_days_left": lim.DaysLeft,
		"whitelabel":      lim.Whitelabel,
		"connector_tier":  lim.ConnectorTier,
		"queries":         queries,
		"datasets":        datasets,
		"ai_messages":     ai,
		"users":           users,
		"dashboards":      dashboards,
		"connectors":      connectors,
		"limits": map[string]int{
			"users": lim.Users, "datasets": lim.Datasets, "queries": lim.Queries, "ai": lim.AI,
			"dashboards": lim.Dashboards, "connectors": lim.Connectors,
		},
	}
	if lim.TrialEndsAt != nil {
		out["trial_ends_at"] = lim.TrialEndsAt.UTC().Format(time.RFC3339)
	}
	return out
}
