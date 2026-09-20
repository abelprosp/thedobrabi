package apihttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/aiagent"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
)

func (s *Server) mfaVerify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MFAToken string `json:"mfa_token"`
		Code     string `json:"code"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	p, tok, err := s.auth.FinishMFA(r.Context(), body.MFAToken, body.Code)
	if err != nil {
		httpx.Error(w, 401, "mfa", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"tokens": tok, "user": p})
}

func (s *Server) mfaEnroll(w http.ResponseWriter, r *http.Request) {
	uid, _, _, _ := principal(r)
	p, err := s.auth.Principal(r.Context(), uid, uuid.Nil)
	if err != nil {
		httpx.Error(w, 401, "unauthorized", "sessão expirada")
		return
	}
	secret, url, err := s.auth.BeginEnrollMFA(r.Context(), uid, p.Email)
	if err != nil {
		httpx.Error(w, 400, "mfa", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"secret": secret, "otpauth_url": url})
}

func (s *Server) mfaConfirm(w http.ResponseWriter, r *http.Request) {
	uid, _, _, _ := principal(r)
	var body struct {
		Code string `json:"code"`
	}
	_ = httpx.Decode(r, &body)
	if err := s.auth.ConfirmEnrollMFA(r.Context(), uid, body.Code); err != nil {
		httpx.Error(w, 400, "mfa", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) mfaDisable(w http.ResponseWriter, r *http.Request) {
	uid, _, _, _ := principal(r)
	var body struct {
		Code string `json:"code"`
	}
	_ = httpx.Decode(r, &body)
	if err := s.auth.DisableMFA(r.Context(), uid, body.Code); err != nil {
		httpx.Error(w, 400, "mfa", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	_ = httpx.Decode(r, &body)
	plain, found, _ := s.auth.RequestPasswordReset(r.Context(), body.Email)
	if found {
		link := s.deps.Cfg.WebOrigin + "/reset?token=" + plain
		_ = s.notify.SendMail(body.Email, "Recuperar senha TheDobra", "Abra: "+link)
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if err := s.auth.ResetPassword(r.Context(), body.Token, body.Password); err != nil {
		httpx.Error(w, 400, "reset", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) acceptInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		MFACode  string `json:"mfa_code"`
	}
	_ = httpx.Decode(r, &body)

	// Optional session: existing invitees (incl. SSO) may prove identity via Bearer token.
	// An admin opening invite_url is not the invitee, so this does not auto-login as them.
	sessionUserID := uuid.Nil
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		if sess, err := s.auth.ParseAccess(strings.TrimPrefix(h, "Bearer ")); err == nil {
			sessionUserID = sess.UserID
		}
	}

	p, tok, err := s.auth.AcceptInvite(r.Context(), token, body.Name, body.Password, body.MFACode, sessionUserID)
	if err != nil {
		httpx.Error(w, 400, "invite", err.Error())
		return
	}
	if tok.TokenType == "mfa_required" {
		httpx.JSON(w, 200, map[string]any{"mfa_required": true})
		return
	}
	if verificationToken, verifyErr := s.auth.CreateEmailVerification(r.Context(), p.UserID); verifyErr == nil {
		link := s.deps.Cfg.WebOrigin + "/verify-email?token=" + url.QueryEscape(verificationToken)
		_ = s.notify.SendMail(p.Email, "Confirme o seu e-mail — TheDobra", "Confirme a sua conta TheDobra através deste link:\n\n"+link+"\n\nEsta ligação expira em 24 horas.")
	}
	httpx.JSON(w, 200, map[string]any{"tokens": tok, "user": p})
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) {
	_, org, _, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT u.id, u.email, u.name, om.role, COALESCE(u.active, TRUE)
		FROM organization_members om JOIN users u ON u.id=om.user_id
		WHERE om.org_id=$1 ORDER BY om.created_at
	`, org)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id uuid.UUID
		var email, name, role string
		var active bool
		_ = rows.Scan(&id, &email, &name, &role, &active)
		out = append(out, map[string]any{"id": id, "email": email, "name": name, "role": role, "active": active})
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) inviteMember(w http.ResponseWriter, r *http.Request) {
	uid, org, _, role := principal(r)
	if !isAdmin(role) {
		httpx.Error(w, 403, "forbidden", "apenas admin")
		return
	}
	if err := s.ent.Check(r.Context(), org, "user"); err != nil {
		httpx.Error(w, 402, "quota", err.Error())
		return
	}
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Email == "" {
		httpx.Error(w, 400, "invalid", "e-mail obrigatório")
		return
	}
	plain, err := s.auth.CreateInvite(r.Context(), org, uid, body.Email, body.Role)
	if err != nil {
		httpx.Error(w, 400, "invite", err.Error())
		return
	}
	link := s.deps.Cfg.WebOrigin + "/invite/" + plain
	_ = s.notify.SendMail(body.Email, "Convite TheDobra", "Aceite o convite: "+link)
	httpx.JSON(w, 201, map[string]any{"invite_url": link})
}

func (s *Server) patchMember(w http.ResponseWriter, r *http.Request) {
	_, org, _, role := principal(r)
	if !isAdmin(role) {
		httpx.Error(w, 403, "forbidden", "apenas admin")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id")
		return
	}
	var body struct {
		Role string `json:"role"`
	}
	_ = httpx.Decode(r, &body)
	if body.Role == "" {
		httpx.Error(w, 400, "invalid", "função obrigatória")
		return
	}
	if body.Role != "viewer" && body.Role != "analyst" && body.Role != "admin" {
		httpx.Error(w, 400, "invalid", "função inválida")
		return
	}
	if !isAdmin(role) && body.Role == "admin" {
		httpx.Error(w, 403, "forbidden", "apenas owner pode promover admin")
		return
	}
	var currentRole string
	if err := s.deps.PG.QueryRow(r.Context(), `SELECT role FROM organization_members WHERE org_id=$1 AND user_id=$2`, org, id).Scan(&currentRole); err != nil {
		httpx.Error(w, 404, "not_found", "membro não encontrado")
		return
	}
	if currentRole == "owner" && body.Role != "owner" {
		var owners int
		_ = s.deps.PG.QueryRow(r.Context(), `SELECT COUNT(*) FROM organization_members WHERE org_id=$1 AND role='owner'`, org).Scan(&owners)
		if owners <= 1 {
			httpx.Error(w, 400, "last_owner", "a organização precisa manter um owner")
			return
		}
	}
	_, err = s.deps.PG.Exec(r.Context(), `UPDATE organization_members SET role=$3 WHERE org_id=$1 AND user_id=$2`, org, id, body.Role)
	if err != nil {
		httpx.Error(w, 400, "update_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) shareDashboard(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if !requireAnalyst(w, role) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id")
		return
	}
	var exists bool
	if err := s.deps.PG.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM dashboards WHERE id=$1 AND org_id=$2 AND workspace_id=$3)`, id, org, ws).Scan(&exists); err != nil || !exists {
		httpx.Error(w, 404, "not_found", "dashboard não encontrado")
		return
	}
	var body struct {
		ExpiresDays *int `json:"expires_days"`
	}
	_ = httpx.Decode(r, &body)
	tok, err := cryptoenc.RandomToken(18)
	if err != nil {
		httpx.Error(w, 500, "token", err.Error())
		return
	}
	var expires *time.Time
	days := 90
	if body.ExpiresDays != nil {
		days = *body.ExpiresDays
	}
	if days > 0 {
		t := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)
		expires = &t
	}
	_, err = s.deps.PG.Exec(r.Context(), `
		INSERT INTO dashboard_shares (org_id, workspace_id, dashboard_id, token, created_by, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, org, ws, id, tok, uid, expires)
	if err != nil {
		httpx.Error(w, 400, "share", err.Error())
		return
	}
	out := map[string]any{"url": s.orgWebOrigin(r.Context(), org) + "/share/" + tok, "token": tok}
	if expires != nil {
		out["expires_at"] = expires.UTC()
	}
	httpx.JSON(w, 201, out)
}

func (s *Server) listDashboardShares(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT token, created_at, expires_at, revoked_at
		FROM dashboard_shares
		WHERE dashboard_id=$1 AND org_id=$2 AND workspace_id=$3
		ORDER BY created_at DESC
	`, id, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var tok string
		var created time.Time
		var expires, revoked *time.Time
		if err := rows.Scan(&tok, &created, &expires, &revoked); err != nil {
			continue
		}
		item := map[string]any{
			"token":      tok,
			"url":        s.orgWebOrigin(r.Context(), org) + "/share/" + tok,
			"created_at": created,
			"revoked_at": revoked,
			"active":     revoked == nil && (expires == nil || expires.After(time.Now())),
		}
		if expires != nil {
			item["expires_at"] = expires.UTC()
		}
		out = append(out, item)
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) revokeDashboardShare(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if !requireAnalyst(w, role) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	tok := chi.URLParam(r, "token")
	ct, err := s.deps.PG.Exec(r.Context(), `
		UPDATE dashboard_shares SET revoked_at=now()
		WHERE dashboard_id=$1 AND org_id=$2 AND workspace_id=$3 AND token=$4 AND revoked_at IS NULL
	`, id, org, ws, tok)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Error(w, 404, "not_found", "partilha não encontrada")
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) lookupShare(ctx context.Context, tok string) (org, ws, dash uuid.UUID, layout []byte, name, desc string, err error) {
	var expires, revoked *time.Time
	err = s.deps.PG.QueryRow(ctx, `
		SELECT s.org_id, s.workspace_id, d.id, d.name, d.description, d.layout_json, s.expires_at, s.revoked_at
		FROM dashboard_shares s JOIN dashboards d ON d.id=s.dashboard_id
		WHERE s.token=$1
	`, tok).Scan(&org, &ws, &dash, &name, &desc, &layout, &expires, &revoked)
	if err != nil {
		return
	}
	if revoked != nil || (expires != nil && !expires.After(time.Now())) {
		err = errShareGone
	}
	return
}

type shareGone string

func (e shareGone) Error() string { return string(e) }

const errShareGone shareGone = "partilha expirada ou revogada"

func (s *Server) publicDashboard(w http.ResponseWriter, r *http.Request) {
	tok := chi.URLParam(r, "token")
	org, _, id, layout, name, desc, err := s.lookupShare(r.Context(), tok)
	if err != nil {
		httpx.Error(w, 404, "not_found", "partilha não encontrada")
		return
	}
	var parsed any
	if json.Unmarshal(layout, &parsed) != nil || parsed == nil {
		parsed = map[string]any{"widgets": []any{}}
	}
	brandName, brandLogo := s.orgBrand(r.Context(), org)
	httpx.JSON(w, 200, map[string]any{
		"id": id, "name": name, "description": desc, "layout": parsed,
		"brand_name": brandName, "brand_logo_url": brandLogo,
	})
}

func (s *Server) publicDashboardQuery(w http.ResponseWriter, r *http.Request) {
	tok := chi.URLParam(r, "token")
	org, ws, _, layout, _, _, err := s.lookupShare(r.Context(), tok)
	if err != nil {
		httpx.Error(w, 404, "not_found", "partilha não encontrada")
		return
	}
	var in publicQueryInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, 400, "invalid", "consulta inválida")
		return
	}
	req, _, err := buildPublicWidgetQuery(layout, in)
	if err != nil {
		httpx.Error(w, 403, "forbidden", err.Error())
		return
	}
	res, err := s.query.Execute(r.Context(), org, ws, uuid.Nil, "viewer", req)
	if err != nil {
		httpx.Error(w, 400, "query_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, res)
}

func (s *Server) publicDashboardAnalyze(w http.ResponseWriter, r *http.Request) {
	tok := chi.URLParam(r, "token")
	org, ws, _, layout, _, _, err := s.lookupShare(r.Context(), tok)
	if err != nil {
		httpx.Error(w, 404, "not_found", "partilha não encontrada")
		return
	}
	s.analyzePublicDashboard(w, r, org, ws, layout)
}

func (s *Server) analyzePublicDashboard(w http.ResponseWriter, r *http.Request, org, ws uuid.UUID, layout []byte) {
	if err := s.ent.Check(r.Context(), org, "ai"); err != nil {
		httpx.Error(w, 402, "quota", err.Error())
		return
	}
	var req aiagent.AnalyzeDashboardRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, 400, "invalid", "pedido inválido")
		return
	}
	global := make([]queryeng.Filter, 0, len(req.GlobalFilters))
	for _, f := range req.GlobalFilters {
		global = append(global, queryeng.Filter{Dimension: f.Dimension, Op: f.Op, Value: f.Value})
	}
	kept := make([]aiagent.DashboardWidgetSpec, 0, len(req.Widgets))
	for _, spec := range req.Widgets {
		lw, ok := findLayoutWidget(layout, spec.ID)
		if !ok {
			continue
		}
		built, _, err := buildPublicWidgetQuery(layout, publicQueryInput{
			WidgetID:      spec.ID,
			GlobalFilters: global,
			TimeRange:     req.TimeRange,
		})
		if err != nil {
			continue
		}
		title := spec.Title
		if title == "" {
			title = lw.Type
		}
		kept = append(kept, aiagent.DashboardWidgetSpec{
			ID:     lw.ID,
			Type:   lw.Type,
			Title:  title,
			Query:  built,
			Config: lw.Config,
		})
	}
	req.Widgets = kept
	if len(req.Widgets) == 0 {
		httpx.Error(w, 400, "invalid", "sem visuais para analisar nesta partilha")
		return
	}
	out, err := s.ai.AnalyzeDashboardWidgets(r.Context(), org, ws, uuid.Nil, "viewer", req)
	if err != nil {
		httpx.Error(w, 400, "analyze_failed", err.Error())
		return
	}
	out.AlertSuggestions = nil
	s.audit(r, "AI_DASHBOARD_ANALYZED", "ai", uuid.Nil, map[string]any{"widgets": out.AnalyzedWidgets, "source": out.Source, "public": true})
	httpx.JSON(w, 200, out)
}

func allowedDatasetIDs(layout []byte) map[string]struct{} {
	out := map[string]struct{}{}
	var parsed struct {
		Widgets []map[string]any `json:"widgets"`
	}
	if json.Unmarshal(layout, &parsed) != nil {
		return out
	}
	add := func(id string) {
		if id != "" {
			out[id] = struct{}{}
		}
	}
	for _, wgt := range parsed.Widgets {
		q, _ := wgt["query"].(map[string]any)
		if q == nil {
			continue
		}
		if id, ok := q["dataset_id"].(string); ok {
			add(id)
		}
		joins, _ := q["joins"].([]any)
		for _, raw := range joins {
			j, _ := raw.(map[string]any)
			if j == nil {
				continue
			}
			if id, ok := j["dataset_id"].(string); ok {
				add(id)
			}
		}
	}
	return out
}

func (s *Server) lakeObjects(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	_ = s.ingest.PurgeOrphanClickHouseTables(r.Context())
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT o.id, o.dataset_id, o.stage, o.object_key, o.bytes, o.created_at
		FROM lake_objects o
		JOIN datasets d ON d.id = o.dataset_id
		WHERE o.org_id=$1 AND o.workspace_id=$2
		ORDER BY o.created_at DESC LIMIT 50
	`, org, ws)
	if err != nil {
		httpx.Error(w, 500, "query_failed", err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id uuid.UUID
		var ds *uuid.UUID
		var stage, key string
		var bytes int64
		var at any
		_ = rows.Scan(&id, &ds, &stage, &key, &bytes, &at)
		out = append(out, map[string]any{"id": id, "dataset_id": ds, "stage": stage, "key": key, "bytes": bytes, "created_at": at})
	}
	httpx.JSON(w, 200, out)
}
