package ingest

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/thedobra/thedobra/services/api/internal/config"
	"github.com/thedobra/thedobra/services/api/internal/platform"
	"github.com/thedobra/thedobra/services/api/internal/quality"
	"github.com/thedobra/thedobra/services/api/internal/schemax"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
	"github.com/xuri/excelize/v2"
)

type Engine struct {
	pg    *pgxpool.Pool
	ch    driver.Conn
	minio *minio.Client
	cfg   config.Config
	bus   platform.EventBus
}

func New(pg *pgxpool.Pool, ch driver.Conn, minio *minio.Client, cfg config.Config, bus platform.EventBus) *Engine {
	return &Engine{pg: pg, ch: ch, minio: minio, cfg: cfg, bus: bus}
}

type Result struct {
	DatasetID uuid.UUID        `json:"dataset_id"`
	JobID     uuid.UUID        `json:"job_id"`
	Name      string           `json:"name"`
	Table     string           `json:"clickhouse_table"`
	RowCount  int64            `json:"row_count"`
	Schema    []schemax.Column `json:"schema"`
	Quality   quality.Report   `json:"quality"`
	Semantic  semantic.Model   `json:"semantic_model"`
	Status    string           `json:"status"`
}

func (e *Engine) IngestFile(ctx context.Context, orgID, wsID, userID uuid.UUID, name, filename string, r io.Reader) (Result, error) {
	raw, err := io.ReadAll(io.LimitReader(r, 512<<20)) // 512MB hard cap for sync upload; stream connectors handle more
	if err != nil {
		return Result{}, err
	}
	kind := detectKind(filename, raw)
	var rows [][]string
	var headers []string
	switch kind {
	case "xlsx":
		headers, rows, err = parseXLSX(raw)
	case "json":
		headers, rows, err = parseJSON(raw)
	case "parquet":
		headers, rows, err = parseParquet(raw)
	case "pdf":
		headers, rows, err = parsePDF(raw)
	case "ofx":
		headers, rows, err = parseOFX(raw)
	default:
		headers, rows, err = parseCSV(raw)
	}
	if err != nil {
		return Result{}, err
	}
	if len(headers) == 0 {
		return Result{}, fmt.Errorf("no columns detected")
	}

	cols := make([]schemax.Column, len(headers))
	names := schemax.UniqueNames(headers)
	sampleN := min(len(rows), 2000)
	for i, n := range names {
		values := make([]string, 0, sampleN)
		for _, row := range rows[:sampleN] {
			if i < len(row) {
				values = append(values, row[i])
			}
		}
		t := schemax.InferType(values)
		cols[i] = schemax.Column{Name: n, SourceName: headers[i], Type: t}
	}

	q := quality.Analyze(cols, rows[:min(len(rows), 50000)])
	for i := range cols {
		cols[i].Role = schemax.GuessRole(cols[i])
	}

	dsID := uuid.New()
	table := "ds_" + strings.ReplaceAll(dsID.String(), "-", "")
	slug := schemax.SanitizeIdent(name)
	if slug == "" || slug == "col" {
		slug = "dataset_" + table[len(table)-8:]
	}

	jobID := uuid.New()
	schemaJSON, _ := json.Marshal(cols)
	qualityJSON, _ := json.Marshal(q)
	now := time.Now()

	tx, err := e.pg.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO datasets (id, org_id, workspace_id, name, slug, clickhouse_table, schema_json, quality_score, quality_json, status, row_count)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ingesting',0)
	`, dsID, orgID, wsID, name, slug+"-"+dsID.String()[:8], table, schemaJSON, q.Score, qualityJSON)
	if err != nil {
		return Result{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO ingestion_jobs (id, org_id, workspace_id, dataset_id, kind, status, started_at)
		VALUES ($1,$2,$3,$4,$5,'running', now())
	`, jobID, orgID, wsID, dsID, kind)
	if err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}

	_ = e.bus.Publish(ctx, "data.ingestion", platform.Event{
		Type: "data.ingestion", OrgID: orgID.String(), WorkspaceID: wsID.String(),
		IdempotencyKey: jobID.String(),
		Payload:        map[string]any{"dataset_id": dsID.String(), "kind": kind},
	})

	rawKey := path.Join("bronze", "company_id="+orgID.String(), "dataset_id="+dsID.String(), now.Format("20060102T150405"), filename)
	if e.minio != nil {
		_, _ = e.minio.PutObject(ctx, e.cfg.MinioBucket, rawKey, bytes.NewReader(raw), int64(len(raw)), minio.PutObjectOptions{
			ContentType: "application/octet-stream",
		})
	}

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
	_, err = e.pg.Exec(ctx, `
		INSERT INTO semantic_models (org_id, workspace_id, dataset_id, name, status, model_json)
		VALUES ($1,$2,$3,$4,'published',$5)
	`, orgID, wsID, dsID, model.Name, modelJSON)
	if err != nil {
		e.fail(ctx, dsID, jobID, err)
		return Result{}, err
	}

	_, _ = e.pg.Exec(ctx, `
		UPDATE datasets SET status='ready', row_count=$2, size_bytes=$3, updated_at=now() WHERE id=$1
	`, dsID, inserted, len(raw))
	_, _ = e.pg.Exec(ctx, `
		UPDATE ingestion_jobs SET status='completed', finished_at=now(), progress_json=$2 WHERE id=$1
	`, jobID, mustJSON(map[string]any{"rows": inserted, "raw_key": rawKey}))

	_ = e.bus.Publish(ctx, "data.completed", platform.Event{
		Type: "data.completed", OrgID: orgID.String(), WorkspaceID: wsID.String(),
		IdempotencyKey: jobID.String() + ":done",
		Payload:        map[string]any{"dataset_id": dsID.String(), "rows": inserted},
	})
	_ = e.bus.Publish(ctx, "dataset.created", platform.Event{
		Type: "dataset.created", OrgID: orgID.String(), WorkspaceID: wsID.String(),
		Payload: map[string]any{"dataset_id": dsID.String()},
	})

	_ = e.writeLake(ctx, orgID, wsID, dsID, names, rows)

	_ = userID
	return Result{
		DatasetID: dsID, JobID: jobID, Name: name, Table: table,
		RowCount: inserted, Schema: cols, Quality: q, Semantic: model, Status: "ready",
	}, nil
}

// IngestRowsFromMaps materializes a slice of row maps into a new ClickHouse dataset.
// It is used by the Dobra Flow engine to persist transformed data.
func (e *Engine) IngestRowsFromMaps(ctx context.Context, orgID, wsID, userID uuid.UUID, name string, headers []string, rows []map[string]any) (Result, error) {
	if len(headers) == 0 || len(rows) == 0 {
		return Result{}, fmt.Errorf("headers and rows are required")
	}
	stringRows := make([][]string, len(rows))
	for i, r := range rows {
		rec := make([]string, len(headers))
		for j, h := range headers {
			rec[j] = fmt.Sprint(r[h])
		}
		stringRows[i] = rec
	}
	return e.ingestRows(ctx, orgID, wsID, userID, name, "flow", headers, stringRows)
}

func (e *Engine) fail(ctx context.Context, dsID, jobID uuid.UUID, err error) {
	_, _ = e.pg.Exec(ctx, `UPDATE datasets SET status='failed', updated_at=now() WHERE id=$1`, dsID)
	_, _ = e.pg.Exec(ctx, `UPDATE ingestion_jobs SET status='failed', error=$2, finished_at=now() WHERE id=$1`, jobID, err.Error())
}

func (e *Engine) createTable(ctx context.Context, table string, cols []schemax.Column) error {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE IF NOT EXISTS %s.`%s` (\n", e.cfg.ClickHouseDB, table)
	b.WriteString("  `_tenant` UUID,\n")
	b.WriteString("  `_ingested_at` DateTime64(3) DEFAULT now64(3)")
	for _, c := range cols {
		fmt.Fprintf(&b, ",\n  `%s` %s", c.Name, schemax.ClickHouseType(c.Type))
	}
	b.WriteString("\n) ENGINE = MergeTree\nPARTITION BY toYYYYMM(_ingested_at)\nORDER BY (_tenant, _ingested_at)\nSETTINGS index_granularity = 8192")
	return e.ch.Exec(ctx, b.String())
}

func (e *Engine) insertRows(ctx context.Context, table string, tenant uuid.UUID, cols []schemax.Column, rows [][]string) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	colNames := make([]string, 0, len(cols)+1)
	colNames = append(colNames, "_tenant")
	for _, c := range cols {
		colNames = append(colNames, c.Name)
	}
	quoted := make([]string, len(colNames))
	for i, n := range colNames {
		quoted[i] = "`" + n + "`"
	}
	sql := fmt.Sprintf("INSERT INTO %s.`%s` (%s)", e.cfg.ClickHouseDB, table, strings.Join(quoted, ","))
	return e.insertRowsTenant(ctx, tenant, cols, rows, sql)
}

func (e *Engine) insertRowsTenant(ctx context.Context, tenant uuid.UUID, cols []schemax.Column, rows [][]string, sql string) (int64, error) {
	const batchSize = 5000
	var inserted int64
	for start := 0; start < len(rows); start += batchSize {
		end := min(start+batchSize, len(rows))
		batch, err := e.ch.PrepareBatch(ctx, sql)
		if err != nil {
			return inserted, err
		}
		for _, row := range rows[start:end] {
			vals := make([]any, 0, len(cols)+1)
			vals = append(vals, tenant)
			for i, c := range cols {
				var raw string
				if i < len(row) {
					raw = row[i]
				}
				v := schemax.ParseValue(c.Type, raw)
				switch c.Type {
				case schemax.TypeBool:
					if v == nil {
						vals = append(vals, nil)
					} else if v.(bool) {
						vals = append(vals, uint8(1))
					} else {
						vals = append(vals, uint8(0))
					}
				default:
					vals = append(vals, v)
				}
			}
			if err := batch.Append(vals...); err != nil {
				return inserted, err
			}
		}
		if err := batch.Send(); err != nil {
			return inserted, err
		}
		inserted += int64(end - start)
	}
	return inserted, nil
}

func parseCSV(raw []byte) ([]string, [][]string, error) {
	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	all, err := r.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(all) == 0 {
		return nil, nil, fmt.Errorf("empty csv")
	}
	return all[0], all[1:], nil
}

func parseXLSX(raw []byte) ([]string, [][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil, fmt.Errorf("xlsx has no sheets")
	}
	data, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, nil, err
	}
	if len(data) == 0 {
		return nil, nil, fmt.Errorf("empty sheet")
	}
	return data[0], data[1:], nil
}

func detectKind(filename string, raw []byte) string {
	ln := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(ln, ".xlsx"), strings.HasSuffix(ln, ".xls"):
		return "xlsx"
	case strings.HasSuffix(ln, ".json"), strings.HasSuffix(ln, ".ndjson"):
		return "json"
	case strings.HasSuffix(ln, ".parquet"):
		return "parquet"
	case strings.HasSuffix(ln, ".pdf"):
		return "pdf"
	case strings.HasSuffix(ln, ".ofx"), strings.HasSuffix(ln, ".qfx"):
		return "ofx"
	case strings.HasSuffix(ln, ".csv"), strings.HasSuffix(ln, ".tsv"):
		return "csv"
	}
	if len(raw) >= 2 && raw[0] == 0x50 && raw[1] == 0x4B {
		return "xlsx"
	}
	trim := bytes.TrimSpace(raw)
	if len(trim) > 0 && (trim[0] == '[' || trim[0] == '{') {
		return "json"
	}
	return "csv"
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (e *Engine) AppendToDataset(ctx context.Context, orgID, wsID, datasetID uuid.UUID, headers []string, rows [][]string) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	var table string
	var schemaJSON []byte
	err := e.pg.QueryRow(ctx, `SELECT clickhouse_table, schema_json FROM datasets WHERE id=$1 AND org_id=$2 AND workspace_id=$3`,
		datasetID, orgID, wsID).Scan(&table, &schemaJSON)
	if err != nil {
		return 0, fmt.Errorf("conjunto não encontrado para CDC")
	}
	var cols []schemax.Column
	_ = json.Unmarshal(schemaJSON, &cols)
	if len(cols) == 0 {
		return 0, fmt.Errorf("esquema vazio")
	}
	idx := map[string]int{}
	for i, h := range headers {
		idx[strings.ToLower(h)] = i
	}
	aligned := make([][]string, len(rows))
	for r, row := range rows {
		rec := make([]string, len(cols))
		for i, c := range cols {
			j, ok := idx[strings.ToLower(c.Name)]
			if !ok {
				j, ok = idx[strings.ToLower(c.SourceName)]
			}
			if ok && j < len(row) {
				rec[i] = row[j]
			}
		}
		aligned[r] = rec
	}
	n, err := e.insertRows(ctx, table, orgID, cols, aligned)
	if err != nil {
		return 0, err
	}
	_, _ = e.pg.Exec(ctx, `UPDATE datasets SET row_count=row_count+$2, updated_at=now() WHERE id=$1`, datasetID, n)
	return n, nil
}

func safeCHTable(s string) bool {
	if s == "" || len(s) > 80 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func (e *Engine) ReplaceDataset(ctx context.Context, orgID, wsID, datasetID uuid.UUID, headers []string, rows [][]string) (int64, error) {
	if len(headers) == 0 {
		return 0, fmt.Errorf("nenhuma coluna devolvida — verifique o recurso e as credenciais")
	}
	var table string
	var schemaJSON []byte
	err := e.pg.QueryRow(ctx, `SELECT clickhouse_table, schema_json FROM datasets WHERE id=$1 AND org_id=$2 AND workspace_id=$3`,
		datasetID, orgID, wsID).Scan(&table, &schemaJSON)
	if err != nil {
		return 0, fmt.Errorf("conjunto não encontrado")
	}
	if !safeCHTable(table) {
		return 0, fmt.Errorf("tabela inválida")
	}
	var cols []schemax.Column
	_ = json.Unmarshal(schemaJSON, &cols)
	if len(cols) == 0 {
		return 0, fmt.Errorf("esquema vazio")
	}
	idx := map[string]int{}
	for i, h := range headers {
		idx[strings.ToLower(h)] = i
	}
	aligned := make([][]string, len(rows))
	for r, row := range rows {
		rec := make([]string, len(cols))
		for i, c := range cols {
			j, ok := idx[strings.ToLower(c.Name)]
			if !ok {
				j, ok = idx[strings.ToLower(c.SourceName)]
			}
			if ok && j < len(row) {
				rec[i] = row[j]
			}
		}
		aligned[r] = rec
	}
	if err := e.ch.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE IF EXISTS %s.`%s`", e.cfg.ClickHouseDB, table)); err != nil {
		return 0, err
	}
	n, err := e.insertRows(ctx, table, orgID, cols, aligned)
	if err != nil {
		return 0, err
	}
	_, _ = e.pg.Exec(ctx, `UPDATE datasets SET row_count=$2, status='ready', updated_at=now() WHERE id=$1`, datasetID, n)
	return n, nil
}

func (e *Engine) writeLake(ctx context.Context, orgID, wsID, datasetID uuid.UUID, headers []string, rows [][]string) error {
	if e.minio == nil {
		return nil
	}
	prefix := path.Join("company_id="+orgID.String(), "dataset_id="+datasetID.String())
	silver := e.csvBytes(headers, rows[:min(len(rows), 200000)])
	gold := e.csvBytes(headers, rows[:min(len(rows), 50000)])
	now := time.Now().UTC().Format("20060102T150405")
	type stage struct {
		name, key string
		body      []byte
	}
	stages := []stage{
		{"silver", path.Join("silver", prefix, now+".csv"), silver},
		{"gold", path.Join("gold", prefix, now+".csv"), gold},
	}
	for _, st := range stages {
		_, err := e.minio.PutObject(ctx, e.cfg.MinioBucket, st.key, bytes.NewReader(st.body), int64(len(st.body)), minio.PutObjectOptions{ContentType: "text/csv"})
		if err != nil {
			continue
		}
		_, _ = e.pg.Exec(ctx, `INSERT INTO lake_objects (org_id, workspace_id, dataset_id, stage, object_key, bytes) VALUES ($1,$2,$3,$4,$5,$6)`,
			orgID, wsID, datasetID, st.name, st.key, len(st.body))
	}
	return nil
}

func (e *Engine) csvBytes(headers []string, rows [][]string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write(headers)
	_ = w.WriteAll(rows)
	w.Flush()
	return buf.Bytes()
}

func (e *Engine) ReadSQLIncremental(ctx context.Context, orgID, wsID, sourceID uuid.UUID, table, cursorCol, cursor string, limit int) ([]string, [][]string, string, error) {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return nil, nil, "", err
	}
	if table != "" {
		cfg.Table = table
	}
	if !tableIdentOK(cfg.Table) || !tableIdentOK(cursorCol) {
		return nil, nil, "", fmt.Errorf("identificador inválido")
	}
	if limit <= 0 {
		limit = 10000
	}
	q := fmt.Sprintf("SELECT * FROM %s WHERE %s > $1 ORDER BY %s LIMIT %d", cfg.Table, cursorCol, cursorCol, limit)
	if typ != "postgres" && typ != "supabase" {
		q = fmt.Sprintf("SELECT * FROM %s WHERE %s > ? ORDER BY %s LIMIT %d", cfg.Table, cursorCol, cursorCol, limit)
	}
	cfg.Query = ""
	headers, rows, err := e.readSQLBound(ctx, typ, cfg, q, cursor)
	if err != nil {
		return nil, nil, "", err
	}
	next := cursor
	if len(rows) > 0 {
		ci := 0
		for i, h := range headers {
			if strings.EqualFold(h, cursorCol) || strings.EqualFold(h, strings.TrimPrefix(cursorCol, cfg.Table+".")) {
				ci = i
				break
			}
		}
		next = rows[len(rows)-1][ci]
	}
	return headers, rows, next, nil
}
