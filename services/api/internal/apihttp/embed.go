package apihttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
)

func (s *Server) createDashboardEmbed(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	var n int
	_ = s.deps.PG.QueryRow(r.Context(), `SELECT COUNT(*) FROM dashboards WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&n)
	if n == 0 {
		httpx.Error(w, 404, "not_found", "dashboard não encontrado")
		return
	}
	var body struct {
		ExpiresDays *int `json:"expires_days"`
	}
	_ = httpx.Decode(r, &body)
	jti, err := cryptoenc.RandomToken(18)
	if err != nil {
		httpx.Error(w, 500, "token", err.Error())
		return
	}
	var expires *time.Time
	if body.ExpiresDays != nil && *body.ExpiresDays > 0 {
		t := time.Now().UTC().Add(time.Duration(*body.ExpiresDays) * 24 * time.Hour)
		expires = &t
	}
	_, err = s.deps.PG.Exec(r.Context(), `
		INSERT INTO dashboard_embeds (org_id, workspace_id, dashboard_id, token, created_by, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, org, ws, id, jti, uid, expires)
	if err != nil {
		httpx.Error(w, 400, "embed", err.Error())
		return
	}
	httpx.JSON(w, 201, s.embedPayload(r.Context(), org, id, jti, expires))
}

func (s *Server) listDashboardEmbeds(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	rows, err := s.deps.PG.Query(r.Context(), `
		SELECT token, created_at, expires_at, revoked_at
		FROM dashboard_embeds
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
		var jti string
		var created time.Time
		var expires, revoked *time.Time
		if err := rows.Scan(&jti, &created, &expires, &revoked); err != nil {
			continue
		}
		item := s.embedPayload(r.Context(), org, id, jti, expires)
		item["jti"] = jti
		item["created_at"] = created
		item["revoked_at"] = revoked
		item["active"] = revoked == nil && (expires == nil || expires.After(time.Now()))
		out = append(out, item)
	}
	httpx.JSON(w, 200, out)
}

func (s *Server) revokeDashboardEmbed(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	jti := s.resolveEmbedJTI(chi.URLParam(r, "token"))
	ct, err := s.deps.PG.Exec(r.Context(), `
		UPDATE dashboard_embeds SET revoked_at=now()
		WHERE dashboard_id=$1 AND org_id=$2 AND workspace_id=$3 AND token=$4 AND revoked_at IS NULL
	`, id, org, ws, jti)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Error(w, 404, "not_found", "embed não encontrado")
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) embedPayload(ctx context.Context, org, dash uuid.UUID, jti string, expires *time.Time) map[string]any {
	tok := s.signEmbedJWT(org, dash, jti, expires)
	origin := s.orgWebOrigin(ctx, org)
	url := origin + "/embed/" + tok
	out := map[string]any{
		"token":  tok,
		"jti":    jti,
		"url":    url,
		"iframe": `<iframe src="` + url + `" width="100%" height="640" frameborder="0" allowfullscreen></iframe>`,
		"script": `<script src="` + origin + `/embed.js" data-token="` + tok + `"></script>`,
	}
	if expires != nil {
		out["expires_at"] = expires.UTC()
	}
	return out
}

func (s *Server) signEmbedJWT(org, dash uuid.UUID, jti string, expires *time.Time) string {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"typ":     "embed",
		"jti":     jti,
		"dash_id": dash.String(),
		"org_id":  org.String(),
		"iat":     now.Unix(),
	}
	if expires != nil {
		claims["exp"] = expires.Unix()
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.deps.Cfg.JWTSecret)
	if err != nil {
		return jti
	}
	return tok
}

func (s *Server) resolveEmbedJTI(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.Count(raw, ".") != 2 {
		return raw
	}
	parsed, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errEmbedGone
		}
		return s.deps.Cfg.JWTSecret, nil
	})
	if err != nil || !parsed.Valid {
		return raw
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return raw
	}
	if typ, _ := claims["typ"].(string); typ != "" && typ != "embed" {
		return raw
	}
	if jti, _ := claims["jti"].(string); jti != "" {
		return jti
	}
	return raw
}

func (s *Server) lookupEmbed(ctx context.Context, tok string) (org, ws, dash uuid.UUID, layout []byte, name, desc string, err error) {
	jti := s.resolveEmbedJTI(tok)
	var expires, revoked *time.Time
	err = s.deps.PG.QueryRow(ctx, `
		SELECT e.org_id, e.workspace_id, d.id, d.name, d.description, d.layout_json, e.expires_at, e.revoked_at
		FROM dashboard_embeds e JOIN dashboards d ON d.id=e.dashboard_id
		WHERE e.token=$1
	`, jti).Scan(&org, &ws, &dash, &name, &desc, &layout, &expires, &revoked)
	if err != nil {
		return
	}
	if revoked != nil || (expires != nil && !expires.After(time.Now())) {
		err = errEmbedGone
	}
	return
}

type embedGone string

func (e embedGone) Error() string { return string(e) }

const errEmbedGone embedGone = "embed expirado ou revogado"

func (s *Server) publicEmbed(w http.ResponseWriter, r *http.Request) {
	tok := chi.URLParam(r, "token")
	org, _, id, layout, name, desc, err := s.lookupEmbed(r.Context(), tok)
	if err != nil {
		httpx.Error(w, 404, "not_found", "embed não encontrado")
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

func (s *Server) publicEmbedQuery(w http.ResponseWriter, r *http.Request) {
	tok := chi.URLParam(r, "token")
	org, ws, _, layout, _, _, err := s.lookupEmbed(r.Context(), tok)
	if err != nil {
		httpx.Error(w, 404, "not_found", "embed não encontrado")
		return
	}
	var req queryeng.Request
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, 400, "invalid", "consulta inválida")
		return
	}
	allowed := allowedDatasetIDs(layout)
	if req.DatasetID == "" {
		httpx.Error(w, 400, "invalid", "conjunto em falta")
		return
	}
	if _, ok := allowed[req.DatasetID]; !ok {
		httpx.Error(w, 403, "forbidden", "conjunto não faz parte deste embed")
		return
	}
	for _, j := range req.Joins {
		if j.DatasetID == "" {
			continue
		}
		if _, ok := allowed[j.DatasetID]; !ok {
			httpx.Error(w, 403, "forbidden", "conjunto não faz parte deste embed")
			return
		}
	}
	res, err := s.query.Execute(r.Context(), org, ws, uuid.Nil, "viewer", req)
	if err != nil {
		httpx.Error(w, 400, "query_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, res)
}

func (s *Server) publicEmbedAnalyze(w http.ResponseWriter, r *http.Request) {
	tok := chi.URLParam(r, "token")
	org, ws, _, layout, _, _, err := s.lookupEmbed(r.Context(), tok)
	if err != nil {
		httpx.Error(w, 404, "not_found", "embed não encontrado")
		return
	}
	s.analyzePublicDashboard(w, r, org, ws, layout)
}

func (s *Server) orgBrand(ctx context.Context, org uuid.UUID) (string, string) {
	var name, logo string
	_ = s.deps.PG.QueryRow(ctx, `SELECT COALESCE(brand_name,''), COALESCE(brand_logo_url,'') FROM organizations WHERE id=$1`, org).Scan(&name, &logo)
	return name, logo
}

func (s *Server) orgWebOrigin(ctx context.Context, org uuid.UUID) string {
	var domain string
	_ = s.deps.PG.QueryRow(ctx, `SELECT COALESCE(custom_domain,'') FROM organizations WHERE id=$1`, org).Scan(&domain)
	if host := normalizeDomain(domain); host != "" {
		return "https://" + host
	}
	return s.deps.Cfg.WebOrigin
}

func normalizeDomain(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return ""
	}
	return host
}

func (s *Server) allowCORSOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	if origin == s.deps.Cfg.WebOrigin ||
		strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "https://localhost:") ||
		strings.HasPrefix(origin, "http://127.0.0.1:") || strings.HasPrefix(origin, "https://127.0.0.1:") {
		return true
	}
	host := normalizeDomain(origin)
	if host == "" {
		return false
	}
	var n int
	_ = s.deps.PG.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM organizations WHERE custom_domain=$1 OR custom_domain=$2
	`, host, "www."+host).Scan(&n)
	return n > 0
}
