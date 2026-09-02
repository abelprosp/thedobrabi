package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

type InspectColumn struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Label string `json:"label"`
}

type InspectTable struct {
	Schema   string          `json:"schema,omitempty"`
	Name     string          `json:"name"`
	FullName string          `json:"full_name"`
	Label    string          `json:"label"`
	RowCount *int64          `json:"row_count,omitempty"`
	Columns  []InspectColumn `json:"columns"`
}

type InspectFK struct {
	FromTable  string `json:"from_table"`
	FromColumn string `json:"from_column"`
	ToTable    string `json:"to_table"`
	ToColumn   string `json:"to_column"`
}

func inspectToDiscover(tables []InspectTable, fks []InspectFK) DiscoverResult {
	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, t.FullName)
	}
	if names == nil {
		names = []string{}
	}
	if tables == nil {
		tables = []InspectTable{}
	}
	if fks == nil {
		fks = []InspectFK{}
	}
	return DiscoverResult{Tables: names, Catalog: tables, ForeignKeys: fks}
}

func decorateInspect(tables []InspectTable) []InspectTable {
	for i := range tables {
		tables[i].FullName = tableKey(tables[i].Schema, tables[i].Name)
		tables[i].Label = HumanizeIdent(tables[i].Name)
		if tables[i].Columns == nil {
			tables[i].Columns = []InspectColumn{}
		}
		for j := range tables[i].Columns {
			tables[i].Columns[j].Label = HumanizeIdent(tables[i].Columns[j].Name)
		}
	}
	return tables
}

func (e *Engine) inspectSQL(ctx context.Context, typ string, cfg SQLConfig) ([]InspectTable, []InspectFK, error) {
	switch typ {
	case "postgres", "redshift":
		return e.inspectPostgres(ctx, cfg, "")
	case "mysql", "mariadb":
		return e.inspectMySQL(ctx, typ, cfg)
	case "sqlserver":
		return e.inspectSQLServer(ctx, cfg)
	default:
		return nil, nil, fmt.Errorf("inspect não suportado para %s", typ)
	}
}

func (e *Engine) inspectPostgres(ctx context.Context, cfg SQLConfig, onlySchema string) ([]InspectTable, []InspectFK, error) {
	conn, err := pgx.Connect(ctx, postgresDSN(cfg))
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close(ctx)

	colSQL := `
		SELECT c.table_schema, c.table_name, c.column_name, c.data_type
		FROM information_schema.columns c
		JOIN information_schema.tables t
		  ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE t.table_type = 'BASE TABLE'
		  AND c.table_schema NOT IN ('pg_catalog', 'information_schema')`
	args := []any{}
	if onlySchema != "" {
		colSQL += ` AND c.table_schema = $1`
		args = append(args, onlySchema)
	}
	colSQL += ` ORDER BY c.table_schema, c.table_name, c.ordinal_position LIMIT 20000`

	colRows, err := conn.Query(ctx, colSQL, args...)
	if err != nil {
		return nil, nil, err
	}
	tables, err := collectInspectColumns(colRows)
	if err != nil {
		return nil, nil, err
	}

	fkSQL := `
		SELECT tc.table_schema, tc.table_name, kcu.column_name,
			ccu.table_schema, ccu.table_name, ccu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'`
	fkArgs := []any{}
	if onlySchema != "" {
		fkSQL += ` AND tc.table_schema = $1`
		fkArgs = append(fkArgs, onlySchema)
	}
	fkSQL += ` LIMIT 2000`

	var fks []InspectFK
	if fkRows, err := conn.Query(ctx, fkSQL, fkArgs...); err == nil {
		fks, _ = collectInspectFKs(fkRows)
	}

	estSQL := `
		SELECT n.nspname, c.relname, GREATEST(c.reltuples, 0)::bigint
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r' AND n.nspname NOT IN ('pg_catalog', 'information_schema')`
	if estRows, err := conn.Query(ctx, estSQL); err == nil {
		applyEstimates(tables, estRows)
	}

	return decorateInspect(tables), fks, nil
}

func (e *Engine) inspectMySQL(ctx context.Context, typ string, cfg SQLConfig) ([]InspectTable, []InspectFK, error) {
	db, _, err := openSQL(typ, cfg)
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()

	colRows, err := db.QueryContext(ctx, `
		SELECT table_schema, table_name, column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		ORDER BY table_name, ordinal_position
		LIMIT 20000`)
	if err != nil {
		return nil, nil, err
	}
	tables, err := collectSQLInspectColumns(colRows)
	if err != nil {
		return nil, nil, err
	}
	for i := range tables {
		tables[i].Schema = ""
	}

	var fks []InspectFK
	if fkRows, err := db.QueryContext(ctx, `
		SELECT table_name, column_name, referenced_table_name, referenced_column_name
		FROM information_schema.key_column_usage
		WHERE table_schema = DATABASE() AND referenced_table_name IS NOT NULL
		LIMIT 2000`); err == nil {
		defer fkRows.Close()
		for fkRows.Next() {
			var ft, fc, tt, tc string
			if err := fkRows.Scan(&ft, &fc, &tt, &tc); err != nil {
				break
			}
			fks = append(fks, InspectFK{FromTable: ft, FromColumn: fc, ToTable: tt, ToColumn: tc})
		}
	}

	if estRows, err := db.QueryContext(ctx, `
		SELECT table_name, table_rows
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'`); err == nil {
		defer estRows.Close()
		est := map[string]int64{}
		for estRows.Next() {
			var name string
			var n sql.NullInt64
			if err := estRows.Scan(&name, &n); err != nil {
				break
			}
			if n.Valid {
				est[name] = n.Int64
			}
		}
		for i := range tables {
			if n, ok := est[tables[i].Name]; ok {
				nn := n
				tables[i].RowCount = &nn
			}
		}
	}

	if fks == nil {
		fks = []InspectFK{}
	}
	return decorateInspect(tables), fks, nil
}

func (e *Engine) inspectSQLServer(ctx context.Context, cfg SQLConfig) ([]InspectTable, []InspectFK, error) {
	db, _, err := openSQL("sqlserver", cfg)
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()

	colRows, err := db.QueryContext(ctx, `
		SELECT TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, DATA_TYPE
		FROM INFORMATION_SCHEMA.COLUMNS
		ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION
		OFFSET 0 ROWS FETCH NEXT 20000 ROWS ONLY`)
	if err != nil {
		return nil, nil, err
	}
	tables, err := collectSQLInspectColumns(colRows)
	if err != nil {
		return nil, nil, err
	}

	var fks []InspectFK
	if fkRows, err := db.QueryContext(ctx, `
		SELECT
			OBJECT_SCHEMA_NAME(fk.parent_object_id),
			OBJECT_NAME(fk.parent_object_id),
			COL_NAME(fc.parent_object_id, fc.parent_column_id),
			OBJECT_SCHEMA_NAME(fk.referenced_object_id),
			OBJECT_NAME(fk.referenced_object_id),
			COL_NAME(fc.referenced_object_id, fc.referenced_column_id)
		FROM sys.foreign_keys fk
		JOIN sys.foreign_key_columns fc ON fc.constraint_object_id = fk.object_id`); err == nil {
		fks, _ = collectSQLInspectFKs(fkRows)
	}

	if estRows, err := db.QueryContext(ctx, `
		SELECT SCHEMA_NAME(t.schema_id), t.name, SUM(p.rows)
		FROM sys.tables t
		JOIN sys.partitions p ON t.object_id = p.object_id AND p.index_id IN (0, 1)
		GROUP BY t.schema_id, t.name`); err == nil {
		defer estRows.Close()
		est := map[string]int64{}
		for estRows.Next() {
			var schema, name string
			var n sql.NullInt64
			if err := estRows.Scan(&schema, &name, &n); err != nil {
				break
			}
			if n.Valid {
				est[tableKey(schema, name)] = n.Int64
			}
		}
		for i := range tables {
			if n, ok := est[tableKey(tables[i].Schema, tables[i].Name)]; ok {
				nn := n
				tables[i].RowCount = &nn
			}
		}
	}

	return decorateInspect(tables), fks, nil
}

func collectInspectColumns(rows pgx.Rows) ([]InspectTable, error) {
	defer rows.Close()
	byKey := map[string]*InspectTable{}
	var order []string
	for rows.Next() {
		var schema, name, col, typ string
		if err := rows.Scan(&schema, &name, &col, &typ); err != nil {
			return nil, err
		}
		key := tableKey(schema, name)
		t, ok := byKey[key]
		if !ok {
			if len(order) >= 500 {
				continue
			}
			t = &InspectTable{Schema: schema, Name: name, Columns: []InspectColumn{}}
			byKey[key] = t
			order = append(order, key)
		}
		t.Columns = append(t.Columns, InspectColumn{Name: col, Type: typ})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]InspectTable, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out, nil
}

func collectInspectFKs(rows pgx.Rows) ([]InspectFK, error) {
	defer rows.Close()
	var out []InspectFK
	for rows.Next() {
		var fs, ft, fc, ts, tt, tc string
		if err := rows.Scan(&fs, &ft, &fc, &ts, &tt, &tc); err != nil {
			return nil, err
		}
		out = append(out, InspectFK{
			FromTable: tableKey(fs, ft), FromColumn: fc,
			ToTable: tableKey(ts, tt), ToColumn: tc,
		})
	}
	if out == nil {
		out = []InspectFK{}
	}
	return out, rows.Err()
}

func collectSQLInspectColumns(rows *sql.Rows) ([]InspectTable, error) {
	defer rows.Close()
	byKey := map[string]*InspectTable{}
	var order []string
	for rows.Next() {
		var schema, name, col, typ string
		if err := rows.Scan(&schema, &name, &col, &typ); err != nil {
			return nil, err
		}
		key := tableKey(schema, name)
		t, ok := byKey[key]
		if !ok {
			if len(order) >= 500 {
				continue
			}
			t = &InspectTable{Schema: schema, Name: name, Columns: []InspectColumn{}}
			byKey[key] = t
			order = append(order, key)
		}
		t.Columns = append(t.Columns, InspectColumn{Name: col, Type: typ})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]InspectTable, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out, nil
}

func collectSQLInspectFKs(rows *sql.Rows) ([]InspectFK, error) {
	defer rows.Close()
	var out []InspectFK
	for rows.Next() {
		var fs, ft, fc, ts, tt, tc string
		if err := rows.Scan(&fs, &ft, &fc, &ts, &tt, &tc); err != nil {
			return nil, err
		}
		out = append(out, InspectFK{
			FromTable: tableKey(fs, ft), FromColumn: fc,
			ToTable: tableKey(ts, tt), ToColumn: tc,
		})
	}
	if out == nil {
		out = []InspectFK{}
	}
	return out, rows.Err()
}

func applyEstimates(tables []InspectTable, rows pgx.Rows) {
	defer rows.Close()
	est := map[string]int64{}
	for rows.Next() {
		var schema, name string
		var n int64
		if err := rows.Scan(&schema, &name, &n); err != nil {
			return
		}
		est[tableKey(schema, name)] = n
	}
	for i := range tables {
		if n, ok := est[tableKey(tables[i].Schema, tables[i].Name)]; ok {
			nn := n
			tables[i].RowCount = &nn
		}
	}
}

func (e *Engine) inspectSupabase(ctx context.Context, cfg SQLConfig) ([]InspectTable, []InspectFK, error) {
	if supabaseUsePostgres(cfg) {
		cfg = supabasePreparedSQL(cfg)
		return e.inspectPostgres(ctx, cfg, "public")
	}
	if supabaseUseREST(cfg) {
		return e.inspectSupabaseREST(ctx, cfg)
	}
	return nil, nil, supabaseModeError()
}

func (e *Engine) inspectSupabaseREST(ctx context.Context, cfg SQLConfig) ([]InspectTable, []InspectFK, error) {
	names, err := e.discoverSupabaseREST(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	colsByTable := fetchOpenAPIColumns(ctx, cfg)
	tables := make([]InspectTable, 0, len(names))
	for _, n := range names {
		schema, name := splitTableKey(n)
		if name == "" {
			name = n
		}
		cols := colsByTable[name]
		if cols == nil && schema != "" {
			cols = colsByTable[tableKey(schema, name)]
		}
		if cols == nil {
			cols = []InspectColumn{}
		}
		tables = append(tables, InspectTable{Schema: schema, Name: name, Columns: cols})
	}
	return decorateInspect(tables), []InspectFK{}, nil
}

func fetchOpenAPIColumns(ctx context.Context, cfg SQLConfig) map[string][]InspectColumn {
	base := supabaseProjectURL(cfg)
	key := supabaseRESTKey(cfg)
	if base == "" || key == "" {
		return nil
	}
	headers := supabaseRESTHeaders(key)
	headers["Accept"] = "application/openapi+json, application/json"
	raw, status, err := httpJSON(ctx, http.MethodGet, base+"/rest/v1/", headers, nil, "", "")
	if err != nil || status >= 400 {
		return nil
	}
	return extractOpenAPIColumns(raw)
}

type oaProp struct {
	Type   string `json:"type"`
	Format string `json:"format"`
}

type oaSchema struct {
	Properties map[string]oaProp `json:"properties"`
}

func extractOpenAPIColumns(raw []byte) map[string][]InspectColumn {
	out := map[string][]InspectColumn{}
	var spec struct {
		Definitions map[string]oaSchema `json:"definitions"`
		Components  struct {
			Schemas map[string]oaSchema `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return out
	}
	add := func(name string, sch oaSchema) {
		if len(sch.Properties) == 0 {
			return
		}
		cols := make([]InspectColumn, 0, len(sch.Properties))
		for n, p := range sch.Properties {
			typ := p.Type
			if p.Format != "" {
				typ = p.Format
			}
			cols = append(cols, InspectColumn{Name: n, Type: typ})
		}
		out[name] = cols
	}
	for name, def := range spec.Definitions {
		add(name, def)
	}
	for name, def := range spec.Components.Schemas {
		add(name, def)
	}
	return out
}

func restSelectParam(cols []string) string {
	if len(cols) == 0 {
		return "*"
	}
	var parts []string
	for _, c := range cols {
		if identPartOK(c) {
			parts = append(parts, c)
		}
	}
	if len(parts) == 0 {
		return "*"
	}
	return strings.Join(parts, ",")
}
