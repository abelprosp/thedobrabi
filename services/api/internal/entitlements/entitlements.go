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
	PlanGratis    = "gratis"
	PlanEssencial = "essencial" // Starter (pago)
	PlanPro       = "pro"
	PlanCompleto  = "completo"

	TierBasic = "basic"
	TierPlus  = "plus"
	TierAll   = "all"

	MB int64 = 1024 * 1024
	GB int64 = 1024 * 1024 * 1024
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
	StorageBytes  int64
	Reports       int
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
	StorageBytes  int64    `json:"storage_bytes"`
	Reports       int      `json:"reports"`
	ConnectorTier string   `json:"connector_tier"`
	Whitelabel    bool     `json:"whitelabel"`
	Highlights    []string `json:"highlights"`
}

func Catalog() []PlanDef {
	return []PlanDef{
		{
			ID: PlanGratis, Name: "Grátis", PriceBRL: 0, Period: "mês",
			Users: 1, Datasets: 2, Queries: 2000, AI: 10, Dashboards: 2, Connectors: 0,
			StorageBytes: 10 * MB, Reports: 0, ConnectorTier: TierBasic, Whitelabel: false,
			Highlights: []string{
				"10 créditos de IA / mês",
				"2 dashboards",
				"2 bases de dados",
				"10 MB de armazenamento",
				"Ficheiros CSV, Excel e JSON",
			},
		},
		{
			ID: PlanEssencial, Name: "Starter", PriceBRL: 97, Period: "mês",
			Users: 5, Datasets: 20, Queries: 20000, AI: 150, Dashboards: -1, Connectors: 4,
			StorageBytes: 5 * GB / 2, Reports: -1, ConnectorTier: TierPlus, Whitelabel: false,
			Highlights: []string{
				"150 créditos de IA / mês",
				"Dashboards ilimitados",
				"20 bases de dados",
				"2,5 GB de armazenamento",
				"4 conectores externos",
				"Relatórios agendados",
			},
		},
		{
			ID: PlanPro, Name: "Pro", PriceBRL: 197, Period: "mês",
			Users: 25, Datasets: 160, Queries: 200000, AI: 1500, Dashboards: -1, Connectors: 32,
			StorageBytes: 25 * GB, Reports: -1, ConnectorTier: TierPlus, Whitelabel: false,
			Highlights: []string{
				"1.500 créditos de IA / mês",
				"Dashboards ilimitados",
				"160 bases de dados",
				"25 GB de armazenamento",
				"32 conectores externos",
				"Relatórios agendados",
			},
		},
		{
			ID: PlanCompleto, Name: "Completo", PriceBRL: 397, Period: "mês",
			Users: -1, Datasets: -1, Queries: -1, AI: -1, Dashboards: -1, Connectors: -1,
			StorageBytes: -1, Reports: -1, ConnectorTier: TierAll, Whitelabel: true,
			Highlights: []string{
				"Créditos de IA ilimitados",
				"Dashboards, bases e conectores ilimitados",
				"Armazenamento ilimitado",
				"Relatórios agendados",
				"White-label (logo e nome)",
				"Todos os conectores",
			},
		},
	}
}

func NormalizePlan(plan string) string {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case PlanGratis, "free", "trial":
		return PlanGratis
	case PlanPro, "growth":
		return PlanPro
	case PlanCompleto, "business", "enterprise":
		return PlanCompleto
	case PlanEssencial, "starter":
		return PlanEssencial
	case "":
		return PlanGratis
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
		Dashboards: d.Dashboards, Connectors: d.Connectors, StorageBytes: d.StorageBytes,
		Reports: d.Reports, ConnectorTier: d.ConnectorTier,
		Whitelabel: d.Whitelabel, Plan: d.ID, Name: d.Name, PriceBRL: d.PriceBRL,
	}
}

func FormatBytes(n int64) string {
	if n < 0 {
		return "ilimitado"
	}
	if n < MB {
		return fmt.Sprintf("%d B", n)
	}
	if n < GB {
		if n%MB == 0 {
			return fmt.Sprintf("%d MB", n/MB)
		}
		return fmt.Sprintf("%.1f MB", float64(n)/float64(MB))
	}
	if n%GB == 0 {
		return fmt.Sprintf("%d GB", n/GB)
	}
	return fmt.Sprintf("%.1f GB", float64(n)/float64(GB))
}

func IsExternalConnector(typ string) bool {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "csv", "xlsx", "json", "pdf", "manual", "":
		return false
	default:
		return true
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
		if lim.Dashboards < 0 {
			return nil
		}
		var n int
		_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM dashboards WHERE org_id=$1`, orgID).Scan(&n)
		if n >= lim.Dashboards {
			return fmt.Errorf("o plano %s inclui %d dashboards. Peça um upgrade ou um teste de 7 dias", lim.Name, lim.Dashboards)
		}
	case "connector":
		if lim.Connectors >= 0 {
			var n int
			_ = s.pg.QueryRow(ctx, `
				SELECT COUNT(*) FROM data_sources
				WHERE org_id=$1 AND lower(type) NOT IN ('csv','xlsx','json','pdf','manual')
			`, orgID).Scan(&n)
			if n >= lim.Connectors {
				if lim.Connectors == 0 {
					return fmt.Errorf("o plano %s não inclui conectores externos. Faça upgrade para o Starter", lim.Name)
				}
				return fmt.Errorf("o plano %s inclui %d conectores externos. Peça um upgrade para ligar mais fontes", lim.Name, lim.Connectors)
			}
		}
	case "storage":
		return s.CheckStorage(ctx, orgID, 0)
	case "report":
		if lim.Reports == 0 {
			return fmt.Errorf("relatórios agendados estão no plano Starter. Faça upgrade para programar envios")
		}
		if lim.Reports < 0 {
			return nil
		}
		var n int
		_ = s.pg.QueryRow(ctx, `
			SELECT COUNT(*) FROM reports
			WHERE org_id=$1 AND lower(cadence) NOT IN ('','once','manual','none')
		`, orgID).Scan(&n)
		if n >= lim.Reports {
			return fmt.Errorf("o plano %s inclui %d relatórios agendados", lim.Name, lim.Reports)
		}
	case "ai":
		if lim.AI == 0 {
			return fmt.Errorf("o plano %s não inclui créditos de IA. O Starter já traz 150 créditos por mês", lim.Name)
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
			return fmt.Errorf("white-label está no plano Completo")
		}
	}
	return nil
}

func (s *Service) CheckStorage(ctx context.Context, orgID uuid.UUID, extra int64) error {
	lim := s.Limits(ctx, orgID)
	if lim.StorageBytes < 0 {
		return nil
	}
	var used int64
	_ = s.pg.QueryRow(ctx, `SELECT COALESCE(SUM(size_bytes), 0) FROM datasets WHERE org_id=$1`, orgID).Scan(&used)
	if used+extra > lim.StorageBytes {
		return fmt.Errorf("o plano %s inclui %s de armazenamento (já usa %s). Faça upgrade ou apague conjuntos", lim.Name, FormatBytes(lim.StorageBytes), FormatBytes(used))
	}
	return nil
}

func (s *Service) CheckConnector(ctx context.Context, orgID uuid.UUID, typ string) error {
	lim := s.Limits(ctx, orgID)
	if !ConnectorAllowed(lim.ConnectorTier, typ) {
		return fmt.Errorf("o conector «%s» não está no plano %s", typ, lim.Name)
	}
	if !IsExternalConnector(typ) {
		return nil
	}
	return s.Check(ctx, orgID, "connector")
}

func (s *Service) Snapshot(ctx context.Context, orgID, wsID uuid.UUID) map[string]any {
	lim := s.Limits(ctx, orgID)
	var queries, datasets, ai, users, dashboards, connectors, reports int
	var storage int64
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM query_history WHERE org_id=$1 AND created_at > date_trunc('month', now())`, orgID).Scan(&queries)
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM datasets WHERE org_id=$1`, orgID).Scan(&datasets)
	_ = s.pg.QueryRow(ctx, `SELECT COALESCE(SUM(size_bytes), 0) FROM datasets WHERE org_id=$1`, orgID).Scan(&storage)
	_ = s.pg.QueryRow(ctx, `
		SELECT COUNT(*) FROM ai_messages m JOIN ai_conversations c ON c.id=m.conversation_id
		WHERE c.org_id=$1 AND m.created_at > date_trunc('month', now())
	`, orgID).Scan(&ai)
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM organization_members WHERE org_id=$1`, orgID).Scan(&users)
	_ = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM dashboards WHERE org_id=$1`, orgID).Scan(&dashboards)
	_ = s.pg.QueryRow(ctx, `
		SELECT COUNT(*) FROM data_sources
		WHERE org_id=$1 AND lower(type) NOT IN ('csv','xlsx','json','pdf','manual')
	`, orgID).Scan(&connectors)
	_ = s.pg.QueryRow(ctx, `
		SELECT COUNT(*) FROM reports
		WHERE org_id=$1 AND lower(cadence) NOT IN ('','once','manual','none')
	`, orgID).Scan(&reports)
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
		"storage_bytes":   storage,
		"ai_messages":     ai,
		"users":           users,
		"dashboards":      dashboards,
		"connectors":      connectors,
		"reports":         reports,
		"limits": map[string]any{
			"users": lim.Users, "datasets": lim.Datasets, "queries": lim.Queries, "ai": lim.AI,
			"dashboards": lim.Dashboards, "connectors": lim.Connectors,
			"storage_bytes": lim.StorageBytes, "reports": lim.Reports,
		},
	}
	if lim.TrialEndsAt != nil {
		out["trial_ends_at"] = lim.TrialEndsAt.UTC().Format(time.RFC3339)
	}
	return out
}
