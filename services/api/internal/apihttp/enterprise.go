package apihttp

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
	"github.com/thedobra/thedobra/services/api/internal/scim"
	"github.com/thedobra/thedobra/services/api/internal/sso"
)

func (s *Server) oauthProviders(w http.ResponseWriter, r *http.Request) {
	p := s.sso.Providers()
	httpx.JSON(w, 200, map[string]any{
		"google":     p["google"],
		"github":     p["github"],
		"oidc":       p["oidc"],
		"saml":       p["saml"],
		"public_url": s.deps.Cfg.PublicURL,
	})
}

func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	u, err := s.sso.StartURL(r.Context(), provider)
	if err != nil {
		httpx.Error(w, 400, "oauth", err.Error())
		return
	}
	http.Redirect(w, r, u, http.StatusFound)
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	_, pair, err := s.sso.Finish(r.Context(), provider, r.URL.Query().Get("code"), r.URL.Query().Get("state"))
	if err != nil {
		http.Redirect(w, r, s.deps.Cfg.WebOrigin+"/login?erro="+err.Error(), http.StatusFound)
		return
	}
	sso.RedirectWithTokens(w, r, s.deps.Cfg.WebOrigin, pair)
}

func (s *Server) samlMetadata(w http.ResponseWriter, r *http.Request) {
	org := chi.URLParam(r, "org")
	entity := s.deps.Cfg.PublicURL + "/api/v1/auth/saml/" + org
	acs := entity + "/acs"
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write([]byte(sso.SPMetadata(entity, acs)))
}

func (s *Server) samlLogin(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "org")
	var xmlMeta, issuer string
	err := s.deps.PG.QueryRow(r.Context(), `
		SELECT COALESCE(metadata_xml,''), COALESCE(issuer,'') FROM sso_connections sc
		JOIN organizations o ON o.id = sc.org_id
		WHERE o.slug=$1 AND sc.kind='saml' AND sc.enabled LIMIT 1
	`, slug).Scan(&xmlMeta, &issuer)
	if err != nil {
		httpx.Error(w, 404, "saml", "conexão SAML não encontrada para esta organização")
		return
	}
	idp := sso.ExtractIDPSSO(xmlMeta)
	if idp == "" {
		idp = issuer
	}
	if idp == "" {
		httpx.Error(w, 400, "saml", "metadata do IdP sem SSO URL")
		return
	}
	entity := s.deps.Cfg.PublicURL + "/api/v1/auth/saml/" + slug
	u, err := sso.AuthnRequestRedirect(idp, entity+"/acs", entity)
	if err != nil {
		httpx.Error(w, 400, "saml", err.Error())
		return
	}
	http.Redirect(w, r, u, http.StatusFound)
}

func (s *Server) samlACS(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpx.Error(w, 400, "saml", "form inválido")
		return
	}
	email, name, sub, err := sso.ParseSAMLResponse(r.FormValue("SAMLResponse"))
	if err != nil {
		httpx.Error(w, 400, "saml", err.Error())
		return
	}
	_, pair, err := s.auth.UpsertSSO(r.Context(), email, name, "saml", sub)
	if err != nil {
		httpx.Error(w, 400, "saml", err.Error())
		return
	}
	sso.RedirectWithTokens(w, r, s.deps.Cfg.WebOrigin, pair)
}

func (s *Server) listSSO(w http.ResponseWriter, r *http.Request) {
	_, org, _, _ := principal(r)
	list, err := s.sso.ListConnections(r.Context(), org)
	if err != nil {
		httpx.Error(w, 500, "sso", err.Error())
		return
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) createSSO(w http.ResponseWriter, r *http.Request) {
	_, org, _, role := principal(r)
	if role != "owner" && role != "admin" {
		httpx.Error(w, 403, "forbidden", "apenas admin")
		return
	}
	var body struct {
		Kind         string   `json:"kind"`
		Name         string   `json:"name"`
		Issuer       string   `json:"issuer"`
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		MetadataXML  string   `json:"metadata_xml"`
		Domains      []string `json:"domains"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Name == "" {
		httpx.Error(w, 400, "invalid", "nome obrigatório")
		return
	}
	id, err := s.sso.SaveConnection(r.Context(), org, body.Kind, body.Name, body.Issuer, body.ClientID, body.ClientSecret, body.MetadataXML, body.Domains)
	if err != nil {
		httpx.Error(w, 400, "sso", err.Error())
		return
	}
	httpx.JSON(w, 201, map[string]any{"id": id})
}

func (s *Server) createSCIMToken(w http.ResponseWriter, r *http.Request) {
	_, org, _, role := principal(r)
	if role != "owner" && role != "admin" {
		httpx.Error(w, 403, "forbidden", "apenas admin")
		return
	}
	tok, err := scim.CreateToken(r.Context(), s.deps.PG, org, "scim")
	if err != nil {
		httpx.Error(w, 400, "scim", err.Error())
		return
	}
	httpx.JSON(w, 201, map[string]any{"token": tok, "base_url": s.deps.Cfg.PublicURL + "/scim/v2"})
}

func (s *Server) billingConfig(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, 200, s.billing.PublicConfig())
}

func (s *Server) billingStatus(w http.ResponseWriter, r *http.Request) {
	_, org, _, _ := principal(r)
	httpx.JSON(w, 200, s.billing.Status(r.Context(), org))
}

func (s *Server) billingCheckout(w http.ResponseWriter, r *http.Request) {
	uid, org, _, _ := principal(r)
	var body struct {
		Plan string `json:"plan"`
	}
	_ = httpx.Decode(r, &body)
	if body.Plan == "" {
		body.Plan = "growth"
	}
	p, _ := s.auth.Principal(r.Context(), uid, uuid.Nil)
	url, err := s.billing.Checkout(r.Context(), org, p.Email, body.Plan)
	if err != nil {
		httpx.Error(w, 400, "stripe", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"url": url})
}

func (s *Server) billingPortal(w http.ResponseWriter, r *http.Request) {
	_, org, _, _ := principal(r)
	url, err := s.billing.Portal(r.Context(), org)
	if err != nil {
		httpx.Error(w, 400, "stripe", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"url": url})
}

func (s *Server) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if err := s.billing.HandleWebhook(r); err != nil {
		httpx.Error(w, 400, "webhook", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) getLineage(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	g, err := s.lineage.Graph(r.Context(), org, ws)
	if err != nil {
		httpx.Error(w, 500, "lineage", err.Error())
		return
	}
	httpx.JSON(w, 200, g)
}

func (s *Server) listCDC(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	list, err := s.cdc.List(r.Context(), org, ws)
	if err != nil {
		httpx.Error(w, 500, "cdc", err.Error())
		return
	}
	httpx.JSON(w, 200, list)
}

func (s *Server) enableCDC(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	var body struct {
		DataSourceID string `json:"data_source_id"`
		DatasetID    string `json:"dataset_id"`
		Table        string `json:"table"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "body")
		return
	}
	src, err := uuid.Parse(body.DataSourceID)
	if err != nil {
		httpx.Error(w, 400, "invalid", "data_source_id")
		return
	}
	var ds uuid.UUID
	if body.DatasetID != "" {
		ds, _ = uuid.Parse(body.DatasetID)
	}
	if err := s.cdc.Enable(r.Context(), org, ws, src, ds, body.Table); err != nil {
		httpx.Error(w, 400, "cdc", err.Error())
		return
	}
	httpx.JSON(w, 201, map[string]any{"ok": true})
}
