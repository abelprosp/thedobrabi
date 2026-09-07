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
	tok, err := cryptoenc.RandomToken(18)
	if err != nil {
		httpx.Error(w, 500, "token", err.Error())
		return
	}
	_, err = s.deps.PG.Exec(r.Context(), `
		INSERT INTO dashboard_embeds (org_id, workspace_id, dashboard_id, token, created_by)
		VALUES ($1,$2,$3,$4,$5)
	`, org, ws, id, tok, uid)
	if err != nil {
		httpx.Error(w, 400, "embed", err.Error())
		return
	}
	origin := s.deps.Cfg.WebOrigin
	httpx.JSON(w, 201, map[string]any{
		"token": tok,
		"url":   origin + "/embed/" + tok,
		"iframe": `<iframe src="` + origin + `/embed/` + tok + `" width="100%" height="640" frameborder="0" allowfullscreen></iframe>`,
	})
}

func (s *Server) publicEmbed(w http.ResponseWriter, r *http.Request) {
	tok := chi.URLParam(r, "token")
	var id uuid.UUID
	var name, desc string
	var layout []byte
	err := s.deps.PG.QueryRow(r.Context(), `
		SELECT d.id, d.name, d.description, d.layout_json
		FROM dashboard_embeds e JOIN dashboards d ON d.id=e.dashboard_id
		WHERE e.token=$1
	`, tok).Scan(&id, &name, &desc, &layout)
	if err != nil {
		httpx.Error(w, 404, "not_found", "embed não encontrado")
		return
	}
	var parsed any
	if json.Unmarshal(layout, &parsed) != nil || parsed == nil {
		parsed = map[string]any{"widgets": []any{}}
	}
	httpx.JSON(w, 200, map[string]any{"id": id, "name": name, "description": desc, "layout": parsed})
}

func (s *Server) publicEmbedQuery(w http.ResponseWriter, r *http.Request) {
	tok := chi.URLParam(r, "token")
	var org, ws uuid.UUID
	var layout []byte
	err := s.deps.PG.QueryRow(r.Context(), `
		SELECT e.org_id, e.workspace_id, d.layout_json
		FROM dashboard_embeds e JOIN dashboards d ON d.id=e.dashboard_id
		WHERE e.token=$1
	`, tok).Scan(&org, &ws, &layout)
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
