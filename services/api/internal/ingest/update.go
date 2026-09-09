package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/thedobra/thedobra/services/api/internal/schemax"
)

const (
	MaxJSONAppendRows  = 5000
	MaxJSONReplaceRows = 20000
	MaxInlineEditRows  = 2000
)

type DatasetPatch struct {
	DatasetID uuid.UUID `json:"dataset_id"`
	Mode      string    `json:"mode"`
	Kind      string    `json:"kind"`
	Added     int64     `json:"added"`
	RowCount  int64     `json:"row_count"`
}

type mutableDataset struct {
	Table       string
	Cols        []schemax.Column
	StorageMode string
	SourceType  string
	RowCount    int64
	Status      string
}

func (e *Engine) loadMutableDataset(ctx context.Context, orgID, wsID, datasetID uuid.UUID) (mutableDataset, error) {
	var ds mutableDataset
	var schemaJSON []byte
	var sourceType *string
	err := e.pg.QueryRow(ctx, `
		SELECT d.clickhouse_table, d.schema_json, COALESCE(d.storage_mode, 'import'), COALESCE(d.status, ''), COALESCE(d.row_count, 0), src.type
		FROM datasets d
		LEFT JOIN data_sources src ON src.id = d.data_source_id
		WHERE d.id=$1 AND d.org_id=$2 AND d.workspace_id=$3
	`, datasetID, orgID, wsID).Scan(&ds.Table, &schemaJSON, &ds.StorageMode, &ds.Status, &ds.RowCount, &sourceType)
	if err != nil {
		return ds, fmt.Errorf("conjunto não encontrado")
	}
	if sourceType != nil {
		ds.SourceType = strings.ToLower(*sourceType)
	}
	if strings.EqualFold(ds.StorageMode, "direct_query") {
		return ds, fmt.Errorf("este conjunto consulta a fonte em tempo real — não dá para editar as linhas aqui")
	}
	if ds.SourceType == "manual" {
		return ds, fmt.Errorf("este conjunto é uma planilha manual — edite as linhas no conector")
	}
	if ds.Status != "" && ds.Status != "ready" {
		return ds, fmt.Errorf("o conjunto ainda não está pronto para receber dados")
	}
	if !safeCHTable(ds.Table) {
		return ds, fmt.Errorf("tabela inválida")
	}
	if err := json.Unmarshal(schemaJSON, &ds.Cols); err != nil || len(ds.Cols) == 0 {
		return ds, fmt.Errorf("esquema vazio")
	}
	return ds, nil
}

func alignToSchema(cols []schemax.Column, headers []string, rows [][]string) ([][]string, int) {
	idx := map[string]int{}
	for i, h := range headers {
		key := strings.ToLower(strings.TrimSpace(h))
		if key != "" {
			idx[key] = i
		}
	}
	mapped := 0
	for _, c := range cols {
		if _, ok := idx[strings.ToLower(c.Name)]; ok {
			mapped++
			continue
		}
		if c.SourceName != "" {
			if _, ok := idx[strings.ToLower(c.SourceName)]; ok {
				mapped++
			}
		}
	}
	aligned := make([][]string, len(rows))
	for r, row := range rows {
		rec := make([]string, len(cols))
		for i, c := range cols {
			j, ok := idx[strings.ToLower(c.Name)]
			if !ok && c.SourceName != "" {
				j, ok = idx[strings.ToLower(c.SourceName)]
			}
			if ok && j < len(row) {
				rec[i] = strings.TrimSpace(row[j])
			}
		}
		aligned[r] = rec
	}
	return aligned, mapped
}

func mapsToTable(cols []schemax.Column, rows []map[string]any) (headers []string, data [][]string) {
	headers = make([]string, len(cols))
	for i, c := range cols {
		headers[i] = c.Name
	}
	data = make([][]string, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		rec := make([]string, len(cols))
		empty := true
		for i, c := range cols {
			v, ok := lookupMapValue(row, c.Name)
			if !ok && c.SourceName != "" && !strings.EqualFold(c.SourceName, c.Name) {
				v, ok = lookupMapValue(row, c.SourceName)
			}
			if !ok {
				continue
			}
			s := stringifyCell(v)
			rec[i] = s
			if s != "" {
				empty = false
			}
		}
		if empty {
			continue
		}
		data = append(data, rec)
	}
	return headers, data
}

func lookupMapValue(row map[string]any, key string) (any, bool) {
	if v, ok := row[key]; ok {
		return v, true
	}
	want := strings.ToLower(strings.TrimSpace(key))
	for k, v := range row {
		if strings.HasPrefix(k, "_") {
			continue
		}
		if strings.ToLower(strings.TrimSpace(k)) == want {
			return v, true
		}
	}
	return nil, false
}

func stringifyCell(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func (e *Engine) UpdateDatasetFromFile(ctx context.Context, orgID, wsID, userID, datasetID uuid.UUID, mode, filename string, r io.Reader) (DatasetPatch, error) {
	raw, err := io.ReadAll(io.LimitReader(r, 512<<20))
	if err != nil {
		return DatasetPatch{}, err
	}
	kind, headers, rows, err := parseUploadedBytes(filename, raw)
	if err != nil {
		return DatasetPatch{}, err
	}
	out, err := e.applyDatasetPatch(ctx, orgID, wsID, datasetID, mode, kind, headers, rows)
	if err != nil {
		return DatasetPatch{}, err
	}
	e.recordDatasetPatch(ctx, orgID, wsID, datasetID, out.Mode, kind, filename, raw, headers, rows, out.Added)
	_ = userID
	return out, nil
}

func (e *Engine) UpdateDatasetFromMaps(ctx context.Context, orgID, wsID, datasetID uuid.UUID, mode string, rows []map[string]any) (DatasetPatch, error) {
	ds, err := e.loadMutableDataset(ctx, orgID, wsID, datasetID)
	if err != nil {
		return DatasetPatch{}, err
	}
	headers, data := mapsToTable(ds.Cols, rows)
	out, err := e.applyDatasetPatchOn(ctx, orgID, wsID, datasetID, ds, mode, "rows", headers, data)
	if err != nil {
		return DatasetPatch{}, err
	}
	e.recordDatasetPatch(ctx, orgID, wsID, datasetID, out.Mode, "rows", "linhas.json", nil, headers, data, out.Added)
	return out, nil
}

func (e *Engine) applyDatasetPatch(ctx context.Context, orgID, wsID, datasetID uuid.UUID, mode, kind string, headers []string, rows [][]string) (DatasetPatch, error) {
	ds, err := e.loadMutableDataset(ctx, orgID, wsID, datasetID)
	if err != nil {
		return DatasetPatch{}, err
	}
	return e.applyDatasetPatchOn(ctx, orgID, wsID, datasetID, ds, mode, kind, headers, rows)
}

func (e *Engine) applyDatasetPatchOn(ctx context.Context, orgID, wsID, datasetID uuid.UUID, ds mutableDataset, mode, kind string, headers []string, rows [][]string) (DatasetPatch, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "append" && mode != "replace" {
		return DatasetPatch{}, fmt.Errorf("modo deve ser append ou replace")
	}
	aligned, mapped := alignToSchema(ds.Cols, headers, rows)
	if mapped == 0 {
		return DatasetPatch{}, fmt.Errorf("nenhuma coluna corresponde ao conjunto — use os mesmos nomes (%s)", columnNames(ds.Cols))
	}
	if mode == "append" && len(aligned) == 0 {
		return DatasetPatch{}, fmt.Errorf("não há linhas para acrescentar")
	}
	if mode == "replace" && len(aligned) == 0 {
		return DatasetPatch{}, fmt.Errorf("não há linhas — a substituição deixaria o conjunto vazio")
	}

	var n int64
	var err error
	if mode == "append" {
		n, err = e.insertRows(ctx, ds.Table, orgID, ds.Cols, aligned)
		if err != nil {
			return DatasetPatch{}, err
		}
		_, _ = e.pg.Exec(ctx, `UPDATE datasets SET row_count=row_count+$2, status='ready', updated_at=now() WHERE id=$1`, datasetID, n)
	} else {
		if err := e.ch.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE IF EXISTS %s.`%s`", e.cfg.ClickHouseDB, ds.Table)); err != nil {
			return DatasetPatch{}, err
		}
		n, err = e.insertRows(ctx, ds.Table, orgID, ds.Cols, aligned)
		if err != nil {
			return DatasetPatch{}, err
		}
		_, _ = e.pg.Exec(ctx, `UPDATE datasets SET row_count=$2, status='ready', updated_at=now() WHERE id=$1`, datasetID, n)
	}

	var total int64
	_ = e.pg.QueryRow(ctx, `SELECT COALESCE(row_count,0) FROM datasets WHERE id=$1`, datasetID).Scan(&total)
	return DatasetPatch{
		DatasetID: datasetID,
		Mode:      mode,
		Kind:      kind,
		Added:     n,
		RowCount:  total,
	}, nil
}

func columnNames(cols []schemax.Column) string {
	names := make([]string, 0, len(cols))
	for _, c := range cols {
		if c.SourceName != "" && c.SourceName != c.Name {
			names = append(names, c.SourceName)
			continue
		}
		names = append(names, c.Name)
	}
	if len(names) > 8 {
		names = names[:8]
	}
	return strings.Join(names, ", ")
}

func (e *Engine) recordDatasetPatch(ctx context.Context, orgID, wsID, datasetID uuid.UUID, mode, kind, filename string, raw []byte, headers []string, rows [][]string, added int64) {
	jobID := uuid.New()
	jobKind := kind + "_" + mode
	_, _ = e.pg.Exec(ctx, `
		INSERT INTO ingestion_jobs (id, org_id, workspace_id, dataset_id, kind, status, started_at, finished_at, progress_json)
		VALUES ($1,$2,$3,$4,$5,'completed',now(),now(),$6)
	`, jobID, orgID, wsID, datasetID, jobKind, mustJSON(map[string]any{"rows": added, "mode": mode, "file": filename}))
	if e.minio != nil && len(raw) > 0 {
		rawKey := path.Join("bronze", "company_id="+orgID.String(), "dataset_id="+datasetID.String(), time.Now().UTC().Format("20060102T150405"), filename)
		_, _ = e.minio.PutObject(ctx, e.cfg.MinioBucket, rawKey, bytes.NewReader(raw), int64(len(raw)), minio.PutObjectOptions{
			ContentType: "application/octet-stream",
		})
	}
	_ = e.writeLake(ctx, orgID, wsID, datasetID, headers, rows)
}
