package apihttp

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/connector"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
	"github.com/thedobra/thedobra/services/api/internal/ingest"
)

func (s *Server) connectorsCatalog(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, 200, map[string]any{
		"groups": connector.Groups(),
		"items":  connector.Catalog(),
	})
}

func (s *Server) getSource(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	view, err := s.ingest.GetDataSource(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 404, "not_found", err.Error())
		return
	}
	httpx.JSON(w, 200, view)
}

func (s *Server) patchSource(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	var body struct {
		Selection *ingest.SourceSelection `json:"selection"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	if body.Selection == nil {
		httpx.Error(w, 400, "invalid", "indique a escolha de dados")
		return
	}
	if err := s.ingest.SaveSelection(r.Context(), org, ws, id, body.Selection); err != nil {
		httpx.Error(w, 400, "patch_failed", err.Error())
		return
	}
	view, err := s.ingest.GetDataSource(r.Context(), org, ws, id)
	if err != nil {
		httpx.JSON(w, 200, map[string]any{"ok": true})
		return
	}
	httpx.JSON(w, 200, view)
}

func (s *Server) deleteSource(w http.ResponseWriter, r *http.Request) {
	_, org, ws, role := principal(r)
	if role == "viewer" {
		httpx.Error(w, 403, "forbidden", "sem permissão para excluir fontes")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	if err := s.ingest.DeleteDataSource(r.Context(), org, ws, id); err != nil {
		httpx.Error(w, 400, "delete_failed", err.Error())
		return
	}
	s.sched.DeleteForTarget(r.Context(), "connector", id)
	s.audit(r, "DATA_SOURCE_DELETED", "data_source", id, nil)
	httpx.JSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) testSource(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	res, err := s.ingest.TestConnection(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 400, "test_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, res)
}

func (s *Server) getManualTable(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	view, err := s.ingest.GetManualTable(r.Context(), org, ws, id)
	if err != nil {
		httpx.Error(w, 400, "manual_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, view)
}

func (s *Server) putManualSchema(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if role == "viewer" {
		httpx.Error(w, 403, "forbidden", "sem permissão para alterar a planilha")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	var body struct {
		Columns []ingest.ManualColumn `json:"columns"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	view, err := s.ingest.SaveManualSchema(r.Context(), org, ws, uid, id, body.Columns)
	if err != nil {
		httpx.Error(w, 400, "schema_failed", err.Error())
		return
	}
	s.audit(r, "MANUAL_SCHEMA_SAVED", "data_source", id, map[string]any{"columns": len(view.Columns)})
	httpx.JSON(w, 200, view)
}

func (s *Server) postManualRow(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if role == "viewer" {
		httpx.Error(w, 403, "forbidden", "sem permissão para preencher a planilha")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	var body struct {
		Values map[string]any `json:"values"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	row, err := s.ingest.InsertManualRow(r.Context(), org, ws, uid, id, body.Values)
	if err != nil {
		httpx.Error(w, 400, "row_failed", err.Error())
		return
	}
	s.audit(r, "MANUAL_ROW_CREATED", "data_source", id, map[string]any{"row_id": row.ID})
	httpx.JSON(w, 201, row)
}

func (s *Server) patchManualRow(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if role == "viewer" {
		httpx.Error(w, 403, "forbidden", "sem permissão para editar linhas")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	rowID, err := uuid.Parse(chi.URLParam(r, "rowId"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "linha inválida")
		return
	}
	var body struct {
		Values map[string]any `json:"values"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, 400, "invalid", "corpo inválido")
		return
	}
	row, err := s.ingest.UpdateManualRow(r.Context(), org, ws, uid, id, rowID, body.Values)
	if err != nil {
		httpx.Error(w, 400, "row_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, row)
}

func (s *Server) deleteManualRow(w http.ResponseWriter, r *http.Request) {
	uid, org, ws, role := principal(r)
	if role == "viewer" {
		httpx.Error(w, 403, "forbidden", "sem permissão para excluir linhas")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "id inválido")
		return
	}
	rowID, err := uuid.Parse(chi.URLParam(r, "rowId"))
	if err != nil {
		httpx.Error(w, 400, "invalid", "linha inválida")
		return
	}
	if err := s.ingest.DeleteManualRow(r.Context(), org, ws, uid, id, rowID); err != nil {
		httpx.Error(w, 400, "row_failed", err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"ok": true})
}
