package apihttp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/aiagent"
	"github.com/thedobra/thedobra/services/api/internal/apps"
	"github.com/thedobra/thedobra/services/api/internal/authn"
	"github.com/thedobra/thedobra/services/api/internal/billing"
	"github.com/thedobra/thedobra/services/api/internal/cdc"
	"github.com/thedobra/thedobra/services/api/internal/ctxkey"
	"github.com/thedobra/thedobra/services/api/internal/entitlements"
	"github.com/thedobra/thedobra/services/api/internal/flow"
	"github.com/thedobra/thedobra/services/api/internal/gateway"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
	"github.com/thedobra/thedobra/services/api/internal/ingest"
	"github.com/thedobra/thedobra/services/api/internal/intelligence"
	"github.com/thedobra/thedobra/services/api/internal/lineage"
	"github.com/thedobra/thedobra/services/api/internal/notify"
	"github.com/thedobra/thedobra/services/api/internal/platform"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
	"github.com/thedobra/thedobra/services/api/internal/rls"
	"github.com/thedobra/thedobra/services/api/internal/scheduler"
	"github.com/thedobra/thedobra/services/api/internal/scim"
	"github.com/thedobra/thedobra/services/api/internal/sso"
)

type Server struct {
	deps     *platform.Deps
	auth     *authn.Service
	ingest   *ingest.Engine
	query    *queryeng.Engine
	intel    *intelligence.Engine
	ai       *aiagent.Agent
	sso      *sso.Service
	billing  *billing.Service
	lineage  *lineage.Service
	cdc      *cdc.Engine
	scim     *scim.Server
	ent      *entitlements.Service
	notify   *notify.Service
	flow     *flow.Store
	flowEng  *flow.Engine
	apps     *apps.Store
	gateway  *gateway.Store
	rls      *rls.Store
	sched    *scheduler.Store
	schedRun *scheduler.Runner
}

func New(deps *platform.Deps) http.Handler {
	auth := authn.New(deps.PG, deps.Cfg.JWTSecret, deps.Cfg.EncryptionKey)
	ing := ingest.New(deps.PG, deps.CH, deps.Minio, deps.Cfg, deps.Bus)
	q := queryeng.New(deps.PG, deps.CH, deps.Redis, deps.Cfg)
	intel := intelligence.New(deps.PG, q)
	flowStore := flow.NewStore(deps.PG)
	s := &Server{
		deps:    deps,
		auth:    auth,
		ingest:  ing,
		query:   q,
		intel:   intel,
		ai:      aiagent.New(deps.PG, q, intel, deps.Cfg, deps.Redis),
		sso:     sso.New(deps.Cfg, deps.PG, deps.Redis, auth),
		billing: billing.New(deps.Cfg, deps.PG),
		lineage: lineage.New(deps.PG),
		cdc:     cdc.New(deps.PG, ing, deps.Cfg.EncryptionKey, deps.Log, deps.Bus),
		scim:    scim.New(deps.PG),
		ent:     entitlements.New(deps.PG),
		notify:  notify.New(deps.Cfg, deps.PG, deps.Log),
		flow:    flowStore,
		flowEng: flow.NewEngine(flowStore, &flowIngester{ing: ing}, &flowLineage{svc: lineage.New(deps.PG)}),
		apps:    apps.NewStore(deps.PG),
		gateway: gateway.NewStore(deps.PG),
		rls:     rls.NewStore(deps.PG),
	}
	s.sched = scheduler.NewStore(deps.PG)
	s.schedRun = scheduler.NewRunner(s.sched, deps.Log, scheduler.Jobs{
		Connector: s.runScheduledConnector,
		Flow:      s.runScheduledFlow,
		Dataset:   s.runScheduledDataset,
	})
	go s.cdc.RunLoop(context.Background())
	go s.schedRun.RunLoop(context.Background())

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{deps.Cfg.WebOrigin, "http://localhost:3000", "http://localhost:3001", "http://localhost:3010"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Workspace-Id"},
		AllowCredentials: true,
	}))

	r.Get("/healthz", s.health)
	r.Get("/readyz", s.ready)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", s.register)
		r.Post("/auth/login", s.login)
		r.Post("/auth/mfa/verify", s.mfaVerify)
		r.Post("/auth/forgot", s.forgotPassword)
		r.Post("/auth/reset", s.resetPassword)
		r.Post("/invites/{token}/accept", s.acceptInvite)
		r.Get("/public/dashboards/{token}", s.publicDashboard)
		r.Post("/auth/refresh", s.refresh)
		r.Post("/auth/logout", s.logout)
		r.Get("/auth/oauth/providers", s.oauthProviders)
		r.Get("/auth/oauth/{provider}/start", s.oauthStart)
		r.Get("/auth/oauth/{provider}/callback", s.oauthCallback)
		r.Get("/auth/saml/{org}/metadata", s.samlMetadata)
		r.Get("/auth/saml/{org}/login", s.samlLogin)
		r.Post("/auth/saml/{org}/acs", s.samlACS)
		r.Post("/billing/webhook", s.stripeWebhook)

		r.Group(func(r chi.Router) {
			r.Use(s.authMw)
			r.Get("/auth/me", s.me)
			r.Post("/onboarding/next", s.onboardingNext)
			r.Post("/onboarding/complete", s.onboardingComplete)
			r.Post("/onboarding/reset", s.onboardingReset)
			r.Post("/auth/mfa/enroll", s.mfaEnroll)
			r.Post("/auth/mfa/confirm", s.mfaConfirm)
			r.Post("/auth/mfa/disable", s.mfaDisable)
			r.Get("/organizations/current", s.orgCurrent)
			r.Get("/members", s.listMembers)
			r.Post("/members/invite", s.inviteMember)
			r.Patch("/members/{id}", s.patchMember)
			r.Get("/workspaces", s.workspaces)
			r.Post("/workspaces", s.createWorkspace)
			r.Post("/workspaces/{id}/switch", s.switchWorkspace)

			r.Get("/overview", s.overview)

			r.Get("/data-sources", s.listSources)
			r.Post("/data-sources", s.createSource)
			r.Get("/data-sources/{id}", s.getSource)
			r.Patch("/data-sources/{id}", s.patchSource)
			r.Delete("/data-sources/{id}", s.deleteSource)
			r.Post("/data-sources/{id}/discover", s.discoverSource)
			r.Post("/data-sources/{id}/sync", s.syncSource)
			r.Post("/data-sources/{id}/test", s.testSource)
			r.Get("/connectors/catalog", s.connectorsCatalog)

			r.Get("/sync-schedules", s.listSchedules)
			r.Post("/sync-schedules", s.upsertSchedule)
			r.Get("/sync-schedules/{id}", s.getSchedule)
			r.Patch("/sync-schedules/{id}", s.patchSchedule)
			r.Delete("/sync-schedules/{id}", s.deleteSchedule)
			r.Post("/sync-schedules/{id}/pause", s.pauseSchedule)
			r.Post("/sync-schedules/{id}/resume", s.resumeSchedule)
			r.Post("/sync-schedules/{id}/run", s.runScheduleNow)
			r.Get("/sync-schedules/{id}/runs", s.listScheduleRuns)

			r.Get("/datasets", s.listDatasets)
			r.Post("/datasets/upload", s.uploadDataset)
			r.Post("/datasets/demo", s.demoDataset)
			r.Get("/datasets/{id}", s.getDataset)
			r.Delete("/datasets/{id}", s.deleteDataset)
			r.Get("/datasets/{id}/preview", s.previewDataset)
			r.Get("/datasets/{id}/quality", s.datasetQuality)

			r.Get("/semantic-models", s.listSemantic)
			r.Get("/semantic-models/{id}", s.getSemantic)
			r.Put("/semantic-models/{id}", s.putSemantic)

			r.Post("/queries", s.runQuery)
			r.Get("/queries/history", s.queryHistory)

			r.Get("/dashboards", s.listDashboards)
			r.Post("/dashboards", s.createDashboard)
			r.Post("/dashboards/ai", s.aiDashboard)
			r.Get("/dashboards/{id}", s.getDashboard)
			r.Put("/dashboards/{id}", s.putDashboard)
			r.Post("/dashboards/{id}/share", s.shareDashboard)

			r.Get("/ai/config", s.aiConfig)
			r.Post("/ai/ask", s.ask)
			r.Post("/ai/generate-dashboard", s.generateDashboard)
			r.Get("/ai/conversations", s.conversations)

			r.Get("/insights", s.insights)
			r.Post("/insights/refresh", s.refreshInsights)

			r.Get("/alerts", s.listAlerts)
			r.Post("/alerts", s.createAlert)
			r.Post("/alerts/{id}/evaluate", s.evalAlert)

			r.Get("/reports", s.listReports)
			r.Post("/reports", s.createReport)
			r.Get("/reports/{id}", s.getReport)
			r.Put("/reports/{id}", s.updateReport)
			r.Delete("/reports/{id}", s.deleteReport)
			r.Post("/reports/{id}/generate", s.generateReport)

			r.Get("/metrics", s.metrics)
			r.Get("/audit", s.auditLogs)
			r.Get("/usage", s.usage)
			r.Get("/billing/config", s.billingConfig)
			r.Get("/billing/status", s.billingStatus)
			r.Post("/billing/checkout", s.billingCheckout)
			r.Post("/billing/portal", s.billingPortal)

			r.Get("/sso/connections", s.listSSO)
			r.Post("/sso/connections", s.createSSO)
			r.Post("/sso/scim-token", s.createSCIMToken)

			r.Get("/lineage", s.getLineage)
			r.Get("/cdc", s.listCDC)
			r.Post("/cdc/enable", s.enableCDC)
			r.Get("/lake", s.lakeObjects)

			r.Get("/flows", s.listFlows)
			r.Post("/flows", s.createFlow)
			r.Get("/flows/{id}", s.getFlow)
			r.Put("/flows/{id}", s.updateFlow)
			r.Delete("/flows/{id}", s.deleteFlow)
			r.Get("/flows/{id}/steps", s.listFlowSteps)
			r.Post("/flows/{id}/steps", s.createFlowStep)
			r.Put("/flows/{id}/steps/{stepId}", s.updateFlowStep)
			r.Delete("/flows/{id}/steps/{stepId}", s.deleteFlowStep)
			r.Post("/flows/{id}/runs", s.runFlow)
			r.Get("/flows/{id}/runs", s.listFlowRuns)
			r.Get("/flows/runs/{runId}", s.getFlowRun)
			r.Get("/flows/runs/{runId}/logs", s.getFlowRunLogs)

			r.Get("/datasets/{id}/rls", s.listDatasetRLS)
			r.Post("/datasets/{id}/rls", s.createDatasetRLS)
			r.Put("/datasets/{id}/rls/{rid}", s.updateDatasetRLS)
			r.Delete("/datasets/{id}/rls/{rid}", s.deleteDatasetRLS)
			r.Patch("/datasets/{id}/storage-mode", s.patchDatasetStorageMode)

			r.Get("/semantic-models/{id}/hierarchies", s.listHierarchies)
			r.Post("/semantic-models/{id}/hierarchies", s.createHierarchy)
			r.Delete("/semantic-models/{id}/hierarchies/{hid}", s.deleteHierarchy)
			r.Get("/semantic-models/{id}/relationships", s.listRelationships)
			r.Post("/semantic-models/{id}/relationships", s.createRelationship)
			r.Delete("/semantic-models/{id}/relationships/{rid}", s.deleteRelationship)
			r.Post("/semantic-models/{id}/validate-measure", s.validateMeasure)

			r.Post("/ai/generate-sql", s.generateSQL)
			r.Post("/ai/generate-measure", s.generateMeasure)
			r.Post("/ai/generate-visual", s.generateVisual)

			r.Get("/apps", s.listApps)
			r.Post("/apps", s.createApp)
			r.Get("/apps/{id}", s.getApp)
			r.Put("/apps/{id}", s.updateApp)
			r.Delete("/apps/{id}", s.deleteApp)
			r.Post("/apps/{id}/content", s.setAppContent)
			r.Post("/apps/{id}/publish", s.publishApp)
			r.Get("/apps/{id}/open", s.openApp)

			r.Get("/gateway/instances", s.listGatewayInstances)
			r.Post("/gateway/tokens", s.generateGatewayToken)
		})
		// Public routes (no auth required).
		r.Group(func(r chi.Router) {
			r.Get("/apps/public/{token}", s.publicApp)
		})

		r.Post("/gateway/heartbeat", s.gatewayHeartbeat)
	})

	r.Route("/scim/v2", func(r chi.Router) {
		r.Use(s.scim.Auth(s.deps.PG))
		s.scim.Routes(r)
	})
	return r
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, 200, map[string]any{"status": "ok", "service": "thedobra-api"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	checks := map[string]string{}
	if err := s.deps.PG.Ping(ctx); err != nil {
		checks["postgres"] = err.Error()
	} else {
		checks["postgres"] = "ok"
	}
	if err := s.deps.CH.Ping(ctx); err != nil {
		checks["clickhouse"] = err.Error()
	} else {
		checks["clickhouse"] = "ok"
	}
	if err := s.deps.Redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = err.Error()
	} else {
		checks["redis"] = "ok"
	}
	ok := checks["postgres"] == "ok" && checks["clickhouse"] == "ok" && checks["redis"] == "ok"
	status := 200
	if !ok {
		status = 503
	}
	httpx.JSON(w, status, map[string]any{"ready": ok, "checks": checks})
}

func (s *Server) authMw(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			httpx.Error(w, 401, "unauthorized", "token em falta")
			return
		}
		p, err := s.auth.ParseAccess(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			httpx.Error(w, 401, "unauthorized", "token inválido")
			return
		}
		if ws := r.Header.Get("X-Workspace-Id"); ws != "" {
			id, err := uuid.Parse(ws)
			if err == nil && id != p.WorkspaceID {
				np, err := s.auth.Principal(r.Context(), p.UserID, id)
				if err == nil {
					p = np
				}
			}
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxkey.UserID, p.UserID)
		ctx = context.WithValue(ctx, ctxkey.OrgID, p.OrgID)
		ctx = context.WithValue(ctx, ctxkey.WorkspaceID, p.WorkspaceID)
		ctx = context.WithValue(ctx, ctxkey.Role, p.Role)
		ctx = context.WithValue(ctx, ctxkey.Email, p.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func principal(r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, string) {
	return r.Context().Value(ctxkey.UserID).(uuid.UUID),
		r.Context().Value(ctxkey.OrgID).(uuid.UUID),
		r.Context().Value(ctxkey.WorkspaceID).(uuid.UUID),
		r.Context().Value(ctxkey.Role).(string)
}

func (s *Server) audit(r *http.Request, action, rtype string, rid uuid.UUID, meta map[string]any) {
	var uid, org, ws uuid.UUID
	if v, ok := r.Context().Value(ctxkey.UserID).(uuid.UUID); ok {
		uid = v
	}
	if v, ok := r.Context().Value(ctxkey.OrgID).(uuid.UUID); ok {
		org = v
	}
	if v, ok := r.Context().Value(ctxkey.WorkspaceID).(uuid.UUID); ok {
		ws = v
	}
	b, _ := json.Marshal(meta)
	if len(b) == 0 {
		b = []byte("{}")
	}
	var orgArg, wsArg, uidArg any
	if org != uuid.Nil {
		orgArg = org
	}
	if ws != uuid.Nil {
		wsArg = ws
	}
	if uid != uuid.Nil {
		uidArg = uid
	}
	_, _ = s.deps.PG.Exec(r.Context(), `
		INSERT INTO audit_logs (org_id, workspace_id, user_id, action, resource_type, resource_id, ip, user_agent, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, orgArg, wsArg, uidArg, action, rtype, nullableUUID(rid), r.RemoteAddr, r.UserAgent(), b)
}

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		OrgName  string `json:"organization"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid_json", "invalid body")
		return
	}
	p, tok, err := s.auth.Register(r.Context(), body.Name, body.Email, body.Password, body.OrgName)
	if err != nil {
		httpx.Error(w, 400, "register_failed", err.Error())
		return
	}
	s.audit(r.WithContext(context.WithValue(context.WithValue(context.WithValue(r.Context(), ctxkey.UserID, p.UserID), ctxkey.OrgID, p.OrgID), ctxkey.WorkspaceID, p.WorkspaceID)), "LOGIN", "user", p.UserID, nil)
	httpx.JSON(w, 201, map[string]any{"tokens": tok, "user": p})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid_json", "invalid body")
		return
	}
	p, tok, err := s.auth.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		httpx.Error(w, 401, "invalid_credentials", err.Error())
		return
	}
	if tok.TokenType == "mfa_required" {
		httpx.JSON(w, 200, map[string]any{"mfa_required": true, "mfa_token": tok.AccessToken})
		return
	}
	ctx := r.Context()
	ctx = context.WithValue(ctx, ctxkey.UserID, p.UserID)
	ctx = context.WithValue(ctx, ctxkey.OrgID, p.OrgID)
	ctx = context.WithValue(ctx, ctxkey.WorkspaceID, p.WorkspaceID)
	s.audit(r.WithContext(ctx), "LOGIN", "user", p.UserID, nil)
	httpx.JSON(w, 200, map[string]any{"tokens": tok, "user": p})
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid_json", "invalid body")
		return
	}
	p, tok, err := s.auth.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		httpx.Error(w, 401, "invalid_refresh", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"tokens": tok, "user": p})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = httpx.Decode(r, &body)
	s.auth.Logout(r.Context(), body.RefreshToken)
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	p, err := s.auth.Principal(r.Context(), uid, ws)
	if err != nil {
		httpx.Error(w, 401, "unauthorized", "sessão expirada")
		return
	}
	_ = org
	_ = role
	httpx.JSON(w, 200, p)
}

func (s *Server) orgCurrent(w http.ResponseWriter, r *http.Request) {
	_, org, _, _ := principal(r)
	var name, slug, plan string
	err := s.deps.PG.QueryRow(r.Context(), `SELECT name, slug, plan FROM organizations WHERE id=$1`, org).Scan(&name, &slug, &plan)
	if err != nil {
		httpx.Error(w, 404, "not_found", "organização não encontrada")
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": org, "name": name, "slug": slug, "plan": plan})
}

func (s *Server) workspaces(w http.ResponseWriter, r *http.Request) {
	_, org, _, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `SELECT id, name, slug, created_at FROM workspaces WHERE org_id=$1 ORDER BY created_at`, org)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	type ws struct {
		ID        uuid.UUID `json:"id"`
		Name      string    `json:"name"`
		Slug      string    `json:"slug"`
		CreatedAt time.Time `json:"created_at"`
	}
	var out []ws
	for rows.Next() {
		var x ws
		if err := rows.Scan(&x.ID, &x.Name, &x.Slug, &x.CreatedAt); err != nil {
			httpx.Error(w, 500, "scan_failed", err.Error())
			return
		}
		out = append(out, x)
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) createWorkspace(w http.ResponseWriter, r *http.Request) {
	uid, org, _, role := principal(r)
	if role != "owner" && role != "admin" {
		httpx.Error(w, 403, "forbidden", "permissão insuficiente")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Name == "" {
		httpx.Error(w, 400, "invalid", "nome obrigatório")
		return
	}
	slug := strings.ToLower(strings.ReplaceAll(body.Name, " ", "-"))
	var id uuid.UUID
	err := s.deps.PG.QueryRow(r.Context(), `INSERT INTO workspaces (org_id, name, slug) VALUES ($1,$2,$3) RETURNING id`, org, body.Name, slug).Scan(&id)
	if err != nil {
		httpx.Error(w, 400, "create_failed", err.Error())
		return
	}
	_ = uid
	httpx.JSON(w, 201, map[string]any{"id": id, "name": body.Name, "slug": slug})
}

func (s *Server) switchWorkspace(w http.ResponseWriter, r *http.Request) {
	uid, _, _, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "bad id")
		return
	}
	p, err := s.auth.Principal(r.Context(), uid, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", "espaço de trabalho não encontrado")
		return
	}
	pair, err := s.auth.Issue(r.Context(), p)
	if err != nil {
		httpx.Error(w, 500, "token_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"tokens": pair, "user": p})
}
