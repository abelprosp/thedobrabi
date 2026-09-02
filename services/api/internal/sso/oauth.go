package sso

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/thedobra/thedobra/services/api/internal/authn"
	"github.com/thedobra/thedobra/services/api/internal/config"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type Service struct {
	cfg  config.Config
	pg   *pgxpool.Pool
	rdb  *redis.Client
	auth *authn.Service
}

func New(cfg config.Config, pg *pgxpool.Pool, rdb *redis.Client, auth *authn.Service) *Service {
	return &Service{cfg: cfg, pg: pg, rdb: rdb, auth: auth}
}

func (s *Service) Providers() map[string]bool {
	return map[string]bool{
		"google": s.cfg.GoogleClientID != "" && s.cfg.GoogleClientSecret != "",
		"github": s.cfg.GitHubClientID != "" && s.cfg.GitHubClientSecret != "",
		"oidc":   s.cfg.OIDCIssuer != "" && s.cfg.OIDCClientID != "",
		"saml":   true,
	}
}

func (s *Service) oauthConfig(provider string) (*oauth2.Config, error) {
	switch provider {
	case "google":
		if s.cfg.GoogleClientID == "" {
			return nil, fmt.Errorf("OAuth Google não está configurado")
		}
		return &oauth2.Config{
			ClientID:     s.cfg.GoogleClientID,
			ClientSecret: s.cfg.GoogleClientSecret,
			RedirectURL:  s.cfg.PublicURL + "/api/v1/auth/oauth/google/callback",
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		}, nil
	case "github":
		if s.cfg.GitHubClientID == "" {
			return nil, fmt.Errorf("OAuth GitHub não está configurado")
		}
		return &oauth2.Config{
			ClientID:     s.cfg.GitHubClientID,
			ClientSecret: s.cfg.GitHubClientSecret,
			RedirectURL:  s.cfg.PublicURL + "/api/v1/auth/oauth/github/callback",
			Scopes:       []string{"user:email", "read:user"},
			Endpoint:     github.Endpoint,
		}, nil
	case "oidc":
		if s.cfg.OIDCIssuer == "" {
			return nil, fmt.Errorf("OIDC não está configurado")
		}
		ep, err := discoverOIDC(s.cfg.OIDCIssuer)
		if err != nil {
			return nil, err
		}
		return &oauth2.Config{
			ClientID:     s.cfg.OIDCClientID,
			ClientSecret: s.cfg.OIDCClientSecret,
			RedirectURL:  s.cfg.PublicURL + "/api/v1/auth/oauth/oidc/callback",
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     ep.Endpoint,
		}, nil
	default:
		return nil, fmt.Errorf("provedor desconhecido")
	}
}

type oidcDiscovery struct {
	Endpoint oauth2.Endpoint
	Userinfo string
}

func discoverOIDC(issuer string) (oidcDiscovery, error) {
	u := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	resp, err := http.Get(u)
	if err != nil {
		return oidcDiscovery{}, fmt.Errorf("descoberta OIDC falhou: %w", err)
	}
	defer resp.Body.Close()
	var doc struct {
		Auth     string `json:"authorization_endpoint"`
		Token    string `json:"token_endpoint"`
		Userinfo string `json:"userinfo_endpoint"`
	}
	if json.NewDecoder(resp.Body).Decode(&doc) != nil || doc.Auth == "" {
		return oidcDiscovery{}, fmt.Errorf("metadata OIDC inválido")
	}
	return oidcDiscovery{
		Endpoint: oauth2.Endpoint{AuthURL: doc.Auth, TokenURL: doc.Token},
		Userinfo: doc.Userinfo,
	}, nil
}

func (s *Service) StartURL(ctx context.Context, provider string) (string, error) {
	cfg, err := s.oauthConfig(provider)
	if err != nil {
		return "", err
	}
	state := randomState()
	if s.rdb != nil {
		_ = s.rdb.Set(ctx, "oauth:"+state, provider, 10*time.Minute).Err()
	}
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOnline), nil
}

func (s *Service) Finish(ctx context.Context, provider, code, state string) (authn.Principal, authn.TokenPair, error) {
	if s.rdb != nil {
		got, err := s.rdb.Get(ctx, "oauth:"+state).Result()
		if err != nil || got != provider {
			return authn.Principal{}, authn.TokenPair{}, fmt.Errorf("estado OAuth inválido")
		}
		_ = s.rdb.Del(ctx, "oauth:"+state).Err()
	}
	cfg, err := s.oauthConfig(provider)
	if err != nil {
		return authn.Principal{}, authn.TokenPair{}, err
	}
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return authn.Principal{}, authn.TokenPair{}, fmt.Errorf("troca de código OAuth falhou: %w", err)
	}
	email, name, sub, err := fetchProfile(ctx, provider, cfg, tok, s.cfg.OIDCIssuer)
	if err != nil {
		return authn.Principal{}, authn.TokenPair{}, err
	}
	return s.auth.UpsertSSO(ctx, email, name, provider, sub)
}

func fetchProfile(ctx context.Context, provider string, cfg *oauth2.Config, tok *oauth2.Token, oidcIssuer string) (email, name, sub string, err error) {
	client := cfg.Client(ctx, tok)
	switch provider {
	case "google", "oidc":
		userinfo := "https://openidconnect.googleapis.com/v1/userinfo"
		if provider == "oidc" {
			if raw, ok := tok.Extra("id_token").(string); ok {
				email, name, sub = parseJWTClaims(raw)
				if email != "" {
					return email, name, sub, nil
				}
			}
			if meta, e := discoverOIDC(oidcIssuer); e == nil && meta.Userinfo != "" {
				userinfo = meta.Userinfo
			}
		}
		resp, e := client.Get(userinfo)
		if e != nil {
			return "", "", "", e
		}
		defer resp.Body.Close()
		var p struct {
			Sub   string `json:"sub"`
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		if json.NewDecoder(resp.Body).Decode(&p) != nil {
			return "", "", "", fmt.Errorf("perfil OIDC inválido")
		}
		return p.Email, p.Name, p.Sub, nil
	case "github":
		resp, e := client.Get("https://api.github.com/user")
		if e != nil {
			return "", "", "", e
		}
		defer resp.Body.Close()
		var p struct {
			ID    int    `json:"id"`
			Login string `json:"login"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		if json.NewDecoder(resp.Body).Decode(&p) != nil {
			return "", "", "", fmt.Errorf("perfil GitHub inválido")
		}
		email = p.Email
		if email == "" {
			er, e2 := client.Get("https://api.github.com/user/emails")
			if e2 == nil {
				defer er.Body.Close()
				var emails []struct {
					Email   string `json:"email"`
					Primary bool   `json:"primary"`
				}
				_ = json.NewDecoder(er.Body).Decode(&emails)
				for _, x := range emails {
					if x.Primary {
						email = x.Email
						break
					}
				}
				if email == "" && len(emails) > 0 {
					email = emails[0].Email
				}
			}
		}
		name = p.Name
		if name == "" {
			name = p.Login
		}
		return email, name, fmt.Sprintf("%d", p.ID), nil
	default:
		return "", "", "", fmt.Errorf("provedor desconhecido")
	}
}

func parseJWTClaims(idToken string) (email, name, sub string) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return
	}
	var c map[string]any
	if json.Unmarshal(raw, &c) != nil {
		return
	}
	email, _ = c["email"].(string)
	name, _ = c["name"].(string)
	sub, _ = c["sub"].(string)
	return
}

func randomState() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

type Connection struct {
	ID      uuid.UUID `json:"id"`
	Kind    string    `json:"kind"`
	Name    string    `json:"name"`
	Enabled bool      `json:"enabled"`
	Issuer  string    `json:"issuer,omitempty"`
	Domains []string  `json:"domains"`
}

func (s *Service) ListConnections(ctx context.Context, orgID uuid.UUID) ([]Connection, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, kind, name, enabled, COALESCE(issuer,''), domains FROM sso_connections WHERE org_id=$1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Connection{}
	for rows.Next() {
		var c Connection
		if err := rows.Scan(&c.ID, &c.Kind, &c.Name, &c.Enabled, &c.Issuer, &c.Domains); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) SaveConnection(ctx context.Context, orgID uuid.UUID, kind, name, issuer, clientID, clientSecret, metadataXML string, domains []string) (uuid.UUID, error) {
	id := uuid.New()
	enc := ""
	if clientSecret != "" {
		enc, _ = cryptoenc.Encrypt(s.cfg.EncryptionKey, clientSecret)
	}
	_, err := s.pg.Exec(ctx, `
		INSERT INTO sso_connections (id, org_id, kind, name, issuer, client_id, client_secret_enc, metadata_xml, domains)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, id, orgID, kind, name, issuer, clientID, enc, metadataXML, domains)
	return id, err
}

func RedirectWithTokens(w http.ResponseWriter, r *http.Request, webOrigin string, pair authn.TokenPair) {
	u, _ := url.Parse(strings.TrimRight(webOrigin, "/") + "/auth/callback")
	q := u.Query()
	q.Set("access_token", pair.AccessToken)
	q.Set("refresh_token", pair.RefreshToken)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func ReadBody(r io.Reader) []byte {
	b, _ := io.ReadAll(r)
	return b
}
