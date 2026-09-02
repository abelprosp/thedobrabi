package scim

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	pg *pgxpool.Pool
}

func New(pg *pgxpool.Pool) *Server { return &Server{pg: pg} }

func (s *Server) Routes(r chi.Router) {
	r.Get("/ServiceProviderConfig", s.spc)
	r.Get("/ResourceTypes", s.resourceTypes)
	r.Get("/Schemas", s.schemas)
	r.Get("/Users", s.listUsers)
	r.Post("/Users", s.createUser)
	r.Get("/Users/{id}", s.getUser)
	r.Put("/Users/{id}", s.replaceUser)
	r.Patch("/Users/{id}", s.patchUser)
	r.Delete("/Users/{id}", s.deleteUser)
	r.Get("/Groups", s.listGroups)
	r.Post("/Groups", s.createGroup)
	r.Get("/Groups/{id}", s.getGroup)
	r.Patch("/Groups/{id}", s.patchGroup)
	r.Delete("/Groups/{id}", s.deleteGroup)
}

func (s *Server) Auth(pg *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				scimError(w, 401, "unauthorized")
				return
			}
			token := strings.TrimPrefix(h, "Bearer ")
			var orgID uuid.UUID
			err := pg.QueryRow(r.Context(), `SELECT org_id FROM scim_tokens WHERE token_hash=$1 AND revoked_at IS NULL`, cryptoenc.HashToken(token)).Scan(&orgID)
			if err != nil {
				scimError(w, 401, "unauthorized")
				return
			}
			_, _ = pg.Exec(r.Context(), `UPDATE scim_tokens SET last_used_at=now() WHERE token_hash=$1`, cryptoenc.HashToken(token))
			ctx := context.WithValue(r.Context(), orgKey{}, orgID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type orgKey struct{}

func org(r *http.Request) uuid.UUID { return r.Context().Value(orgKey{}).(uuid.UUID) }

func (s *Server) spc(w http.ResponseWriter, r *http.Request) {
	write(w, 200, map[string]any{
		"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"patch":   map[string]any{"supported": true},
		"filter":  map[string]any{"supported": true, "maxResults": 200},
		"authenticationSchemes": []map[string]any{{
			"type": "oauthbearertoken", "name": "OAuth Bearer Token", "primary": true,
		}},
	})
}

func (s *Server) resourceTypes(w http.ResponseWriter, r *http.Request) {
	write(w, 200, map[string]any{"Resources": []map[string]any{
		{"id": "User", "name": "User", "endpoint": "/Users", "schema": "urn:ietf:params:scim:schemas:core:2.0:User"},
		{"id": "Group", "name": "Group", "endpoint": "/Groups", "schema": "urn:ietf:params:scim:schemas:core:2.0:Group"},
	}})
}

func (s *Server) schemas(w http.ResponseWriter, r *http.Request) {
	write(w, 200, map[string]any{"Resources": []map[string]any{
		{"id": "urn:ietf:params:scim:schemas:core:2.0:User", "name": "User"},
	}})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	orgID := org(r)
	rows, err := s.pg.Query(r.Context(), `
		SELECT u.id, u.email, u.name FROM users u
		JOIN organization_members om ON om.user_id=u.id
		WHERE om.org_id=$1 ORDER BY u.created_at LIMIT 200
	`, orgID)
	if err != nil {
		scimError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	var res []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var email, name string
		_ = rows.Scan(&id, &email, &name)
		res = append(res, scimUser(id, email, name))
	}
	if res == nil {
		res = []map[string]any{}
	}
	write(w, 200, map[string]any{"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"}, "totalResults": len(res), "Resources": res})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	orgID := org(r)
	var body struct {
		UserName    string `json:"userName"`
		DisplayName string `json:"displayName"`
		Active      *bool  `json:"active"`
		Emails      []struct {
			Value string `json:"value"`
		} `json:"emails"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		scimError(w, 400, "invalid")
		return
	}
	email := body.UserName
	if email == "" && len(body.Emails) > 0 {
		email = body.Emails[0].Value
	}
	name := body.DisplayName
	if name == "" {
		name = email
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(uuid.NewString()), 10)
	var id uuid.UUID
	err := s.pg.QueryRow(r.Context(), `INSERT INTO users (email, password_hash, name, auth_provider) VALUES ($1,$2,$3,'scim') RETURNING id`,
		strings.ToLower(email), string(hash), name).Scan(&id)
	if err != nil {
		scimError(w, 409, "usuário já existe")
		return
	}
	_, _ = s.pg.Exec(r.Context(), `INSERT INTO organization_members (org_id, user_id, role) VALUES ($1,$2,'analyst') ON CONFLICT DO NOTHING`, orgID, id)
	write(w, 201, scimUser(id, email, name))
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var email, name string
	err := s.pg.QueryRow(r.Context(), `
		SELECT u.email, u.name FROM users u
		JOIN organization_members om ON om.user_id=u.id
		WHERE u.id=$1 AND om.org_id=$2
	`, id, org(r)).Scan(&email, &name)
	if err != nil {
		scimError(w, 404, "not found")
		return
	}
	write(w, 200, scimUser(id, email, name))
}

func (s *Server) replaceUser(w http.ResponseWriter, r *http.Request) {
	s.patchUser(w, r)
}

func (s *Server) patchUser(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var body struct {
		DisplayName string `json:"displayName"`
		Active      *bool  `json:"active"`
		Operations  []struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value any    `json:"value"`
		} `json:"Operations"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.DisplayName != "" {
		_, _ = s.pg.Exec(r.Context(), `UPDATE users SET name=$2, updated_at=now() WHERE id=$1`, id, body.DisplayName)
	}
	if body.Active != nil {
		_, _ = s.pg.Exec(r.Context(), `UPDATE users SET active=$2, updated_at=now() WHERE id=$1`, id, *body.Active)
	}
	for _, op := range body.Operations {
		path := strings.ToLower(op.Path)
		switch strings.ToLower(op.Op) {
		case "replace":
			if strings.Contains(path, "displayname") || path == "" {
				if m, ok := op.Value.(map[string]any); ok {
					if dn, ok := m["displayName"].(string); ok {
						_, _ = s.pg.Exec(r.Context(), `UPDATE users SET name=$2 WHERE id=$1`, id, dn)
					}
					if a, ok := m["active"].(bool); ok {
						_, _ = s.pg.Exec(r.Context(), `UPDATE users SET active=$2 WHERE id=$1`, id, a)
					}
				}
			}
			if strings.Contains(path, "active") {
				switch v := op.Value.(type) {
				case bool:
					_, _ = s.pg.Exec(r.Context(), `UPDATE users SET active=$2 WHERE id=$1`, id, v)
				case string:
					_, _ = s.pg.Exec(r.Context(), `UPDATE users SET active=$2 WHERE id=$1`, id, v == "true")
				}
			}
		}
	}
	s.getUser(w, r)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	_, _ = s.pg.Exec(r.Context(), `UPDATE users SET active=FALSE, updated_at=now() WHERE id=$1`, id)
	_, _ = s.pg.Exec(r.Context(), `DELETE FROM organization_members WHERE user_id=$1 AND org_id=$2`, id, org(r))
	w.WriteHeader(204)
}

func (s *Server) listGroups(w http.ResponseWriter, r *http.Request) {
	orgID := org(r)
	rows, err := s.pg.Query(r.Context(), `SELECT id, display_name FROM scim_groups WHERE org_id=$1 ORDER BY created_at`, orgID)
	if err != nil {
		scimError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	var res []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var name string
		_ = rows.Scan(&id, &name)
		res = append(res, s.scimGroup(r.Context(), id, name))
	}
	if res == nil {
		res = []map[string]any{}
	}
	write(w, 200, map[string]any{"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"}, "totalResults": len(res), "Resources": res})
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayName string `json:"displayName"`
		Members     []struct {
			Value string `json:"value"`
		} `json:"members"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || body.DisplayName == "" {
		scimError(w, 400, "displayName obrigatório")
		return
	}
	id := uuid.New()
	_, err := s.pg.Exec(r.Context(), `INSERT INTO scim_groups (id, org_id, display_name) VALUES ($1,$2,$3)`, id, org(r), body.DisplayName)
	if err != nil {
		scimError(w, 409, "grupo já existe")
		return
	}
	for _, m := range body.Members {
		uid, e := uuid.Parse(m.Value)
		if e == nil {
			_, _ = s.pg.Exec(r.Context(), `INSERT INTO scim_group_members (group_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, id, uid)
		}
	}
	write(w, 201, s.scimGroup(r.Context(), id, body.DisplayName))
}

func (s *Server) getGroup(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var name string
	err := s.pg.QueryRow(r.Context(), `SELECT display_name FROM scim_groups WHERE id=$1 AND org_id=$2`, id, org(r)).Scan(&name)
	if err != nil {
		scimError(w, 404, "not found")
		return
	}
	write(w, 200, s.scimGroup(r.Context(), id, name))
}

func (s *Server) patchGroup(w http.ResponseWriter, r *http.Request) {
	s.getGroup(w, r)
}

func (s *Server) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	_, _ = s.pg.Exec(r.Context(), `DELETE FROM scim_groups WHERE id=$1 AND org_id=$2`, id, org(r))
	w.WriteHeader(204)
}

func (s *Server) scimGroup(ctx context.Context, id uuid.UUID, name string) map[string]any {
	rows, err := s.pg.Query(ctx, `SELECT user_id FROM scim_group_members WHERE group_id=$1`, id)
	members := []map[string]any{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var uid uuid.UUID
			_ = rows.Scan(&uid)
			members = append(members, map[string]any{"value": uid.String()})
		}
	}
	return map[string]any{
		"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		"id":          id.String(),
		"displayName": name,
		"members":     members,
		"meta":        map[string]any{"resourceType": "Group"},
	}
}

func CreateToken(ctx context.Context, pg *pgxpool.Pool, orgID uuid.UUID, name string) (string, error) {
	raw, err := cryptoenc.RandomToken(32)
	if err != nil {
		return "", err
	}
	_, err = pg.Exec(ctx, `INSERT INTO scim_tokens (org_id, name, token_hash) VALUES ($1,$2,$3)`, orgID, name, cryptoenc.HashToken(raw))
	return raw, err
}

func scimUser(id uuid.UUID, email, name string) map[string]any {
	return map[string]any{
		"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		"id":          id.String(),
		"userName":    email,
		"displayName": name,
		"active":      true,
		"emails":      []map[string]any{{"value": email, "primary": true}},
		"meta":        map[string]any{"resourceType": "User"},
	}
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func scimError(w http.ResponseWriter, status int, d string) {
	write(w, status, map[string]any{
		"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:Error"},
		"detail":  d,
		"status":  fmt.Sprintf("%d", status),
	})
}
