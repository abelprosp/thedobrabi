package apihttp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
	}
	_ = httpx.Decode(r, &body)
	p, tok, err := s.auth.AcceptInvite(r.Context(), token, body.Name, body.Password)
	if err != nil {
		httpx.Error(w, 400, "invite", err.Error())
		return
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
	if role != "owner" && role != "admin" {
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
	if role != "owner" && role != "admin" {
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
	_, err = s.deps.PG.Exec(r.Context(), `UPDATE organization_members SET role=$3 WHERE org_id=$1 AND user_id=$2`, org, id, body.Role)
	if err != nil {
		httpx.Error(w, 400, "update_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) shareDashboard(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id")
		return
	}
	tok, err := cryptoenc.RandomToken(18)
	if err != nil {
		httpx.Error(w, 500, "token", err.Error())
		return
	}
	_, err = s.deps.PG.Exec(r.Context(), `
		INSERT INTO dashboard_shares (org_id, workspace_id, dashboard_id, token, created_by) VALUES ($1,$2,$3,$4,$5)
	`, org, ws, id, tok, uid)
	if err != nil {
		httpx.Error(w, 400, "share", err.Error())
		return
	}
	httpx.JSON(w, 201, map[string]any{"url": s.deps.Cfg.WebOrigin + "/share/" + tok, "token": tok})
}

func (s *Server) publicDashboard(w http.ResponseWriter, r *http.Request) {
	tok := chi.URLParam(r, "token")
	var id, org, ws uuid.UUID
	var name, desc string
	var layout []byte
	err := s.deps.PG.QueryRow(r.Context(), `
		SELECT d.id, d.name, d.description, d.layout_json, s.org_id, s.workspace_id
		FROM dashboard_shares s JOIN dashboards d ON d.id=s.dashboard_id
		WHERE s.token=$1
	`, tok).Scan(&id, &name, &desc, &layout, &org, &ws)
	if err != nil {
		httpx.Error(w, 404, "not_found", "partilha não encontrada")
		return
	}
	var parsed struct {
		Widgets []map[string]any `json:"widgets"`
	}
	_ = json.Unmarshal(layout, &parsed)
	for i, wgt := range parsed.Widgets {
		qraw, ok := wgt["query"]
		if !ok || qraw == nil {
			continue
		}
		b, _ := json.Marshal(qraw)
		var req queryeng.Request
		if json.Unmarshal(b, &req) != nil || req.DatasetID == "" {
			continue
		}
		res, err := s.query.Execute(r.Context(), org, ws, uuid.Nil, "viewer", req)
		if err != nil {
			parsed.Widgets[i]["error"] = err.Error()
			continue
		}
		parsed.Widgets[i]["result"] = res
	}
	httpx.JSON(w, 200, map[string]any{"id": id, "name": name, "description": desc, "layout": parsed})
}

func (s *Server) lakeObjects(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT id, dataset_id, stage, object_key, bytes, created_at
		FROM lake_objects WHERE org_id=$1 AND workspace_id=$2 ORDER BY created_at DESC LIMIT 50
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
