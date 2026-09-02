package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
	"github.com/thedobra/thedobra/services/api/internal/quality"
	"github.com/thedobra/thedobra/services/api/internal/schemax"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

const (
	maxManualColumns = 40
	maxManualRows    = 20000
)

type ManualColumn struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required,omitempty"`
	Options  []string `json:"options,omitempty"`
	Hint     string   `json:"hint,omitempty"`
}

type ManualRowView struct {
	ID        uuid.UUID      `json:"id"`
	Values    map[string]any `json:"values"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type ManualTableView struct {
	SourceID uuid.UUID       `json:"source_id"`
	Name     string          `json:"name"`
	Columns  []ManualColumn  `json:"columns"`
	Rows     []ManualRowView `json:"rows"`
}

func normalizeManualType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "number", "int", "integer", "float", "numeric":
		return "number"
	case "date":
		return "date"
	case "datetime", "timestamp":
		return "datetime"
	case "boolean", "bool", "checkbox":
		return "boolean"
	case "select", "list", "enum":
		return "select"
	default:
		return "text"
	}
}

func slugKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case r == 'á' || r == 'à' || r == 'ã' || r == 'â':
			b.WriteByte('a')
			lastUnderscore = false
		case r == 'é' || r == 'ê':
			b.WriteByte('e')
			lastUnderscore = false
		case r == 'í':
			b.WriteByte('i')
			lastUnderscore = false
		case r == 'ó' || r == 'ô' || r == 'õ':
			b.WriteByte('o')
			lastUnderscore = false
		case r == 'ú' || r == 'ü':
			b.WriteByte('u')
			lastUnderscore = false
		case r == 'ç':
			b.WriteByte('c')
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "col"
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = "c_" + out
	}
	return out
}

func NormalizeManualColumns(cols []ManualColumn) ([]ManualColumn, error) {
	if len(cols) == 0 {
		return nil, fmt.Errorf("defina pelo menos uma coluna")
	}
	if len(cols) > maxManualColumns {
		return nil, fmt.Errorf("máximo de %d colunas", maxManualColumns)
	}
	used := map[string]int{}
	out := make([]ManualColumn, 0, len(cols))
	for _, c := range cols {
		label := strings.TrimSpace(c.Label)
		if label == "" {
			continue
		}
		key := slugKey(c.Key)
		if key == "col" {
			key = slugKey(label)
		}
		n := used[key]
		used[key] = n + 1
		if n > 0 {
			key = key + "_" + strconv.Itoa(n+1)
			used[key] = 1
		}
		typ := normalizeManualType(c.Type)
		opts := make([]string, 0, len(c.Options))
		seen := map[string]bool{}
		for _, o := range c.Options {
			o = strings.TrimSpace(o)
			if o == "" || seen[o] {
				continue
			}
			seen[o] = true
			opts = append(opts, o)
		}
		if typ == "select" && len(opts) == 0 {
			return nil, fmt.Errorf("a coluna «%s» é uma lista e precisa de opções", label)
		}
		out = append(out, ManualColumn{
			Key:      key,
			Label:    label,
			Type:     typ,
			Required: c.Required,
			Options:  opts,
			Hint:     strings.TrimSpace(c.Hint),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("defina pelo menos uma coluna com nome")
	}
	return out, nil
}

func validateManualValue(col ManualColumn, v any) (any, error) {
	if v == nil {
		if col.Required {
			return nil, fmt.Errorf("«%s» é obrigatório", col.Label)
		}
		return nil, nil
	}
	s := strings.TrimSpace(cellString(v))
	if s == "" {
		if col.Required {
			return nil, fmt.Errorf("«%s» é obrigatório", col.Label)
		}
		return nil, nil
	}
	switch col.Type {
	case "number":
		n, err := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), ",", "."), 64)
		if err != nil {
			return nil, fmt.Errorf("«%s» deve ser um número", col.Label)
		}
		return n, nil
	case "boolean":
		ls := strings.ToLower(s)
		return ls == "true" || ls == "sim" || ls == "yes" || ls == "1", nil
	case "select":
		ok := false
		for _, o := range col.Options {
			if o == s {
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("«%s» deve ser uma das opções definidas", col.Label)
		}
		return s, nil
	case "date":
		if _, err := time.Parse("2006-01-02", s); err != nil {
			if t, err2 := time.Parse(time.RFC3339, s); err2 == nil {
				return t.Format("2006-01-02"), nil
			}
			return nil, fmt.Errorf("«%s» deve ser uma data (AAAA-MM-DD)", col.Label)
		}
		return s, nil
	case "datetime":
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			if _, err2 := time.Parse("2006-01-02 15:04", s); err2 != nil {
				if _, err3 := time.Parse("2006-01-02", s); err3 != nil {
					return nil, fmt.Errorf("«%s» deve ser data e hora", col.Label)
				}
			}
		}
		return s, nil
	default:
		return s, nil
	}
}

func validateManualRow(cols []ManualColumn, values map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for _, c := range cols {
		v, err := validateManualValue(c, values[c.Key])
		if err != nil {
			return nil, err
		}
		if v != nil {
			out[c.Key] = v
		}
	}
	return out, nil
}

func cellString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

func manualColType(t string) schemax.ColType {
	switch t {
	case "number":
		return schemax.TypeFloat
	case "date":
		return schemax.TypeDate
	case "datetime":
		return schemax.TypeDateTime
	case "boolean":
		return schemax.TypeBool
	default:
		return schemax.TypeString
	}
}

func manualSchemaxCols(cols []ManualColumn) []schemax.Column {
	out := make([]schemax.Column, len(cols))
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Key
	}
	names = schemax.UniqueNames(names)
	for i, c := range cols {
		out[i] = schemax.Column{
			Name:       names[i],
			SourceName: c.Label,
			Type:       manualColType(c.Type),
		}
		out[i].Role = schemax.GuessRole(out[i])
	}
	return out
}

func (e *Engine) saveSourceConfig(ctx context.Context, orgID, wsID, sourceID uuid.UUID, cfg SQLConfig) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	enc, err := cryptoenc.Encrypt(e.cfg.EncryptionKey, string(raw))
	if err != nil {
		return err
	}
	tag, err := e.pg.Exec(ctx, `UPDATE data_sources SET config_enc=$1, updated_at=now() WHERE id=$2 AND org_id=$3 AND workspace_id=$4`,
		enc, sourceID, orgID, wsID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("fonte não encontrada")
	}
	return nil
}

func (e *Engine) GetManualTable(ctx context.Context, orgID, wsID, sourceID uuid.UUID) (ManualTableView, error) {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return ManualTableView{}, err
	}
	if typ != "manual" {
		return ManualTableView{}, fmt.Errorf("esta fonte não é uma planilha manual")
	}
	var name string
	_ = e.pg.QueryRow(ctx, `SELECT name FROM data_sources WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, sourceID, orgID, wsID).Scan(&name)
	rows, err := e.pg.Query(ctx, `
		SELECT id, payload, created_at, updated_at
		FROM manual_rows
		WHERE org_id=$1 AND workspace_id=$2 AND source_id=$3
		ORDER BY created_at DESC
		LIMIT $4
	`, orgID, wsID, sourceID, maxManualRows)
	if err != nil {
		return ManualTableView{}, err
	}
	defer rows.Close()
	outRows := []ManualRowView{}
	for rows.Next() {
		var r ManualRowView
		var raw []byte
		if err := rows.Scan(&r.ID, &raw, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return ManualTableView{}, err
		}
		_ = json.Unmarshal(raw, &r.Values)
		if r.Values == nil {
			r.Values = map[string]any{}
		}
		outRows = append(outRows, r)
	}
	if cfg.Columns == nil {
		cfg.Columns = []ManualColumn{}
	}
	return ManualTableView{SourceID: sourceID, Name: name, Columns: cfg.Columns, Rows: outRows}, rows.Err()
}

func (e *Engine) SaveManualSchema(ctx context.Context, orgID, wsID, userID, sourceID uuid.UUID, cols []ManualColumn) (ManualTableView, error) {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return ManualTableView{}, err
	}
	if typ != "manual" {
		return ManualTableView{}, fmt.Errorf("esta fonte não é uma planilha manual")
	}
	norm, err := NormalizeManualColumns(cols)
	if err != nil {
		return ManualTableView{}, err
	}
	cfg.Columns = norm
	if cfg.Table == "" {
		cfg.Table = "planilha"
	}
	if err := e.saveSourceConfig(ctx, orgID, wsID, sourceID, cfg); err != nil {
		return ManualTableView{}, err
	}
	if _, err := e.PublishManual(ctx, orgID, wsID, userID, sourceID); err != nil {
		return ManualTableView{}, err
	}
	return e.GetManualTable(ctx, orgID, wsID, sourceID)
}

func (e *Engine) InsertManualRow(ctx context.Context, orgID, wsID, userID, sourceID uuid.UUID, values map[string]any) (ManualRowView, error) {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return ManualRowView{}, err
	}
	if typ != "manual" {
		return ManualRowView{}, fmt.Errorf("esta fonte não é uma planilha manual")
	}
	if len(cfg.Columns) == 0 {
		return ManualRowView{}, fmt.Errorf("defina as colunas antes de preencher o formulário")
	}
	var n int
	if err := e.pg.QueryRow(ctx, `SELECT COUNT(*) FROM manual_rows WHERE source_id=$1 AND org_id=$2`, sourceID, orgID).Scan(&n); err != nil {
		return ManualRowView{}, err
	}
	if n >= maxManualRows {
		return ManualRowView{}, fmt.Errorf("limite de %d linhas atingido", maxManualRows)
	}
	payload, err := validateManualRow(cfg.Columns, values)
	if err != nil {
		return ManualRowView{}, err
	}
	raw, _ := json.Marshal(payload)
	id := uuid.New()
	var created, updated time.Time
	err = e.pg.QueryRow(ctx, `
		INSERT INTO manual_rows (id, org_id, workspace_id, source_id, payload, created_by)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING created_at, updated_at
	`, id, orgID, wsID, sourceID, raw, userID).Scan(&created, &updated)
	if err != nil {
		return ManualRowView{}, err
	}
	if _, err := e.PublishManual(ctx, orgID, wsID, userID, sourceID); err != nil {
		return ManualRowView{}, err
	}
	return ManualRowView{ID: id, Values: payload, CreatedAt: created, UpdatedAt: updated}, nil
}

func (e *Engine) UpdateManualRow(ctx context.Context, orgID, wsID, userID, sourceID, rowID uuid.UUID, values map[string]any) (ManualRowView, error) {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return ManualRowView{}, err
	}
	if typ != "manual" {
		return ManualRowView{}, fmt.Errorf("esta fonte não é uma planilha manual")
	}
	payload, err := validateManualRow(cfg.Columns, values)
	if err != nil {
		return ManualRowView{}, err
	}
	raw, _ := json.Marshal(payload)
	var created, updated time.Time
	err = e.pg.QueryRow(ctx, `
		UPDATE manual_rows SET payload=$1, updated_at=now()
		WHERE id=$2 AND source_id=$3 AND org_id=$4 AND workspace_id=$5
		RETURNING created_at, updated_at
	`, raw, rowID, sourceID, orgID, wsID).Scan(&created, &updated)
	if err != nil {
		return ManualRowView{}, fmt.Errorf("linha não encontrada")
	}
	if _, err := e.PublishManual(ctx, orgID, wsID, userID, sourceID); err != nil {
		return ManualRowView{}, err
	}
	return ManualRowView{ID: rowID, Values: payload, CreatedAt: created, UpdatedAt: updated}, nil
}

func (e *Engine) DeleteManualRow(ctx context.Context, orgID, wsID, userID, sourceID, rowID uuid.UUID) error {
	typ, _, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return err
	}
	if typ != "manual" {
		return fmt.Errorf("esta fonte não é uma planilha manual")
	}
	tag, err := e.pg.Exec(ctx, `
		DELETE FROM manual_rows WHERE id=$1 AND source_id=$2 AND org_id=$3 AND workspace_id=$4
	`, rowID, sourceID, orgID, wsID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("linha não encontrada")
	}
	_, err = e.PublishManual(ctx, orgID, wsID, userID, sourceID)
	return err
}

func (e *Engine) fetchManualRows(ctx context.Context, orgID, wsID, sourceID uuid.UUID, cfg SQLConfig) ([]string, [][]string, error) {
	if len(cfg.Columns) == 0 {
		return nil, nil, fmt.Errorf("defina as colunas da planilha")
	}
	headers := make([]string, len(cfg.Columns))
	for i, c := range cfg.Columns {
		headers[i] = c.Label
		if headers[i] == "" {
			headers[i] = c.Key
		}
	}
	pgRows, err := e.pg.Query(ctx, `
		SELECT payload FROM manual_rows
		WHERE org_id=$1 AND workspace_id=$2 AND source_id=$3
		ORDER BY created_at ASC
		LIMIT $4
	`, orgID, wsID, sourceID, maxManualRows)
	if err != nil {
		return nil, nil, err
	}
	defer pgRows.Close()
	var out [][]string
	for pgRows.Next() {
		var raw []byte
		if err := pgRows.Scan(&raw); err != nil {
			return nil, nil, err
		}
		var payload map[string]any
		_ = json.Unmarshal(raw, &payload)
		rec := make([]string, len(cfg.Columns))
		for i, c := range cfg.Columns {
			rec[i] = cellString(payload[c.Key])
		}
		out = append(out, rec)
	}
	return headers, out, pgRows.Err()
}

func schemaSignature(cols []schemax.Column) string {
	var b strings.Builder
	for _, c := range cols {
		b.WriteString(c.Name)
		b.WriteByte(':')
		b.WriteString(string(c.Type))
		b.WriteByte(';')
	}
	return b.String()
}

func (e *Engine) RecreateDataset(ctx context.Context, orgID, wsID, datasetID uuid.UUID, name string, cols []schemax.Column, rows [][]string) (int64, error) {
	if len(cols) == 0 {
		return 0, fmt.Errorf("nenhuma coluna")
	}
	var table string
	err := e.pg.QueryRow(ctx, `SELECT clickhouse_table FROM datasets WHERE id=$1 AND org_id=$2 AND workspace_id=$3`,
		datasetID, orgID, wsID).Scan(&table)
	if err != nil {
		return 0, fmt.Errorf("conjunto não encontrado")
	}
	if !safeCHTable(table) {
		return 0, fmt.Errorf("tabela inválida")
	}
	if e.ch != nil {
		_ = e.ch.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s.`%s`", e.cfg.ClickHouseDB, table))
		if err := e.createTable(ctx, table, cols); err != nil {
			return 0, err
		}
	}
	n, err := e.insertRows(ctx, table, orgID, cols, rows)
	if err != nil {
		return 0, err
	}
	q := quality.Analyze(cols, rows[:min(len(rows), 50000)])
	for i := range cols {
		if cols[i].Role == "" {
			cols[i].Role = schemax.GuessRole(cols[i])
		}
	}
	schemaJSON, _ := json.Marshal(cols)
	qualityJSON, _ := json.Marshal(q)
	model := semantic.Suggest(name, cols)
	model.DatasetID = datasetID.String()
	modelJSON, _ := json.Marshal(model)
	_, _ = e.pg.Exec(ctx, `UPDATE datasets SET schema_json=$2, quality_score=$3, quality_json=$4, row_count=$5, status='ready', updated_at=now() WHERE id=$1`,
		datasetID, schemaJSON, q.Score, qualityJSON, n)
	_, _ = e.pg.Exec(ctx, `UPDATE semantic_models SET model_json=$2, name=$3, updated_at=now() WHERE dataset_id=$1`,
		datasetID, modelJSON, model.Name)
	_ = e.writeLake(ctx, orgID, wsID, datasetID, headersFromCols(cols), rows)
	return n, nil
}

func headersFromCols(cols []schemax.Column) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		if c.SourceName != "" {
			out[i] = c.SourceName
		} else {
			out[i] = c.Name
		}
	}
	return out
}

func (e *Engine) ingestRowsTyped(ctx context.Context, orgID, wsID, userID uuid.UUID, name, kind string, cols []schemax.Column, rows [][]string) (Result, error) {
	if len(cols) == 0 {
		return Result{}, fmt.Errorf("no columns")
	}
	for i := range cols {
		if cols[i].Name == "" {
			cols[i].Name = schemax.SanitizeIdent(cols[i].SourceName)
		}
		if cols[i].Role == "" {
			cols[i].Role = schemax.GuessRole(cols[i])
		}
	}
	q := quality.Analyze(cols, rows[:min(len(rows), 50000)])
	dsID := uuid.New()
	table := "ds_" + strings.ReplaceAll(dsID.String(), "-", "")
	slug := schemax.SanitizeIdent(name) + "-" + dsID.String()[:8]
	schemaJSON, _ := json.Marshal(cols)
	qualityJSON, _ := json.Marshal(q)
	jobID := uuid.New()

	_, err := e.pg.Exec(ctx, `
		INSERT INTO datasets (id, org_id, workspace_id, name, slug, clickhouse_table, schema_json, quality_score, quality_json, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ingesting')
	`, dsID, orgID, wsID, name, slug, table, schemaJSON, q.Score, qualityJSON)
	if err != nil {
		return Result{}, err
	}
	_, _ = e.pg.Exec(ctx, `INSERT INTO ingestion_jobs (id, org_id, workspace_id, dataset_id, kind, status, started_at) VALUES ($1,$2,$3,$4,$5,'running',now())`,
		jobID, orgID, wsID, dsID, kind)

	if err := e.createTable(ctx, table, cols); err != nil {
		e.fail(ctx, dsID, jobID, err)
		return Result{}, err
	}
	inserted, err := e.insertRows(ctx, table, orgID, cols, rows)
	if err != nil {
		e.fail(ctx, dsID, jobID, err)
		return Result{}, err
	}
	model := semantic.Suggest(name, cols)
	model.DatasetID = dsID.String()
	modelJSON, _ := json.Marshal(model)
	_, err = e.pg.Exec(ctx, `INSERT INTO semantic_models (org_id, workspace_id, dataset_id, name, status, model_json) VALUES ($1,$2,$3,$4,'published',$5)`,
		orgID, wsID, dsID, model.Name, modelJSON)
	if err != nil {
		e.fail(ctx, dsID, jobID, err)
		return Result{}, err
	}
	_, _ = e.pg.Exec(ctx, `UPDATE datasets SET status='ready', row_count=$2, updated_at=now() WHERE id=$1`, dsID, inserted)
	_, _ = e.pg.Exec(ctx, `UPDATE ingestion_jobs SET status='completed', finished_at=now() WHERE id=$1`, jobID)
	_ = userID
	return Result{DatasetID: dsID, JobID: jobID, Name: name, Table: table, RowCount: inserted, Schema: cols, Quality: q, Semantic: model, Status: "ready"}, nil
}

func (e *Engine) PublishManual(ctx context.Context, orgID, wsID, userID, sourceID uuid.UUID) (Result, error) {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return Result{}, err
	}
	if typ != "manual" {
		return Result{}, fmt.Errorf("esta fonte não é uma planilha manual")
	}
	if len(cfg.Columns) == 0 {
		return Result{}, fmt.Errorf("defina as colunas da planilha")
	}
	headers, rows, err := e.fetchManualRows(ctx, orgID, wsID, sourceID, cfg)
	if err != nil {
		return Result{}, err
	}
	cols := manualSchemaxCols(cfg.Columns)
	name := cfg.Table
	if name == "" {
		_ = e.pg.QueryRow(ctx, `SELECT name FROM data_sources WHERE id=$1`, sourceID).Scan(&name)
	}
	if name == "" {
		name = "Planilha manual"
	}

	existing, err := e.LatestDatasetForSource(ctx, orgID, wsID, sourceID)
	if err == nil && existing != uuid.Nil {
		var schemaJSON []byte
		_ = e.pg.QueryRow(ctx, `SELECT schema_json FROM datasets WHERE id=$1`, existing).Scan(&schemaJSON)
		var old []schemax.Column
		_ = json.Unmarshal(schemaJSON, &old)
		if schemaSignature(old) == schemaSignature(cols) {
			n, err := e.ReplaceDataset(ctx, orgID, wsID, existing, headers, rows)
			if err != nil {
				return Result{}, err
			}
			e.LinkDataset(ctx, orgID, sourceID, existing)
			_ = e.writeLake(ctx, orgID, wsID, existing, headers, rows)
			return Result{DatasetID: existing, Name: name, RowCount: n, Status: "ready", Schema: cols}, nil
		}
		n, err := e.RecreateDataset(ctx, orgID, wsID, existing, name, cols, rows)
		if err != nil {
			return Result{}, err
		}
		e.LinkDataset(ctx, orgID, sourceID, existing)
		return Result{DatasetID: existing, Name: name, RowCount: n, Status: "ready", Schema: cols}, nil
	}

	res, err := e.ingestRowsTyped(ctx, orgID, wsID, userID, name, "manual", cols, rows)
	if err != nil {
		return Result{}, err
	}
	e.LinkDataset(ctx, orgID, sourceID, res.DatasetID)
	_ = e.writeLake(ctx, orgID, wsID, res.DatasetID, headers, rows)
	return res, nil
}
