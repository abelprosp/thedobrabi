package apihttp

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
	"github.com/thedobra/thedobra/services/api/internal/ingest"
)

func (s *Server) updateDatasetFile(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if role == "viewer" {
		httpx.Error(w, 403, "forbidden", "sem permissão para alterar conjuntos")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		httpx.Error(w, 400, "invalid", "multipart obrigatório")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, 400, "invalid", "ficheiro obrigatório")
		return
	}
	defer file.Close()
	mode := r.FormValue("mode")
	if mode == "" {
		mode = "append"
	}
	res, err := s.ingest.UpdateDatasetFromFile(r.Context(), org, ws, uid, id, mode, hdr.Filename, file)
	if err != nil {
		httpx.Error(w, 400, "update_failed", err.Error())
		return
	}
	s.audit(r, "DATASET_UPDATED", "dataset", id, map[string]any{"mode": res.Mode, "file": hdr.Filename, "added": res.Added})
	httpx.JSON(w, 200, res)
}

func (s *Server) updateDatasetRows(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if role == "viewer" {
		httpx.Error(w, 403, "forbidden", "sem permissão para alterar conjuntos")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	var body struct {
		Mode string           `json:"mode"`
		Rows []map[string]any `json:"rows"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "json inválido")
		return
	}
	mode := strings.ToLower(strings.TrimSpace(body.Mode))
	if mode == "" {
		mode = "append"
	}
	limit := ingest.MaxJSONAppendRows
	if mode == "replace" {
		limit = ingest.MaxJSONReplaceRows
	}
	if len(body.Rows) == 0 {
		httpx.Error(w, 400, "invalid", "indique pelo menos uma linha")
		return
	}
	if len(body.Rows) > limit {
		httpx.Error(w, 400, "invalid", "demasiadas linhas neste pedido")
		return
	}
	res, err := s.ingest.UpdateDatasetFromMaps(r.Context(), org, ws, id, mode, body.Rows)
	if err != nil {
		httpx.Error(w, 400, "update_failed", err.Error())
		return
	}
	s.audit(r, "DATASET_UPDATED", "dataset", id, map[string]any{"mode": res.Mode, "added": res.Added})
	httpx.JSON(w, 200, res)
}

func (s *Server) listDatasetRows(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	limit := ingest.MaxInlineEditRows
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	if limit <= 0 {
		limit = ingest.MaxInlineEditRows
	}
	if limit > ingest.MaxJSONReplaceRows {
		limit = ingest.MaxJSONReplaceRows
	}
	cols, rows, err := s.query.ReadRows(r.Context(), org, ws, id, limit)
	if err != nil {
		httpx.Error(w, 400, "rows_failed", err.Error())
		return
	}
	keep := make([]string, 0, len(cols))
	for _, c := range cols {
		if strings.HasPrefix(c, "_") {
			continue
		}
		keep = append(keep, c)
	}
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		item := make(map[string]any, len(keep))
		for _, c := range keep {
			item[c] = row[c]
		}
		out[i] = item
	}
	httpx.JSON(w, 200, map[string]any{"columns": keep, "rows": out, "row_count": len(out)})
}
