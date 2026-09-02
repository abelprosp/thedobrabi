package ingest

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	_ "github.com/microsoft/go-mssqldb"
	go_ora "github.com/sijms/go-ora/v2"
	_ "github.com/snowflakedb/gosnowflake"
)

func (e *Engine) discoverSQL(ctx context.Context, typ string, cfg SQLConfig) ([]string, error) {
	if typ == "odbc" {
		typ = detectODBCType(cfg)
		if typ == "postgres" && strings.TrimSpace(cfg.URL) != "" {
			conn, err := pgx.Connect(ctx, cfg.URL)
			if err != nil {
				return nil, err
			}
			defer conn.Close(ctx)
			return queryPGTables(ctx, conn)
		}
	}
	switch typ {
	case "postgres", "redshift":
		dsn := postgresDSN(cfg)
		if typ == "odbc" && strings.TrimSpace(cfg.URL) != "" {
			dsn = cfg.URL
		}
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			return nil, err
		}
		defer conn.Close(ctx)
		return queryPGTables(ctx, conn)
	default:
		db, _, err := openSQL(typ, cfg)
		if err != nil {
			return nil, err
		}
		defer db.Close()
		rows, err := db.QueryContext(ctx, discoverQuery(typ))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanStringRows(rows)
	}
}

func queryPGTables(ctx context.Context, conn *pgx.Conn) ([]string, error) {
	rows, err := conn.Query(ctx, `SELECT table_schema||'.'||table_name FROM information_schema.tables WHERE table_type='BASE TABLE' AND table_schema NOT IN ('pg_catalog','information_schema') ORDER BY 1 LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if out == nil {
		out = []string{}
	}
	return out, rows.Err()
}

func discoverQuery(typ string) string {
	switch typ {
	case "mysql", "mariadb":
		return `SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() ORDER BY 1 LIMIT 500`
	case "sqlserver":
		return `SELECT TABLE_SCHEMA + '.' + TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE='BASE TABLE' ORDER BY 1 OFFSET 0 ROWS FETCH NEXT 500 ROWS ONLY`
	case "oracle":
		return `SELECT owner || '.' || table_name FROM all_tables WHERE ROWNUM <= 500 ORDER BY 1`
	case "snowflake":
		return `SELECT table_schema || '.' || table_name FROM information_schema.tables WHERE table_type='BASE TABLE' ORDER BY 1 LIMIT 500`
	default:
		return `SELECT table_schema || '.' || table_name FROM information_schema.tables WHERE table_type='BASE TABLE' ORDER BY 1 LIMIT 500`
	}
}

func scanStringRows(rows *sql.Rows) ([]string, error) {
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if out == nil {
		out = []string{}
	}
	return out, rows.Err()
}

func openSQL(typ string, cfg SQLConfig) (*sql.DB, string, error) {
	if typ == "odbc" {
		typ = detectODBCType(cfg)
	}
	driver, dsn, err := sqlDSN(typ, cfg)
	if err != nil {
		return nil, "", err
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, dsn, err
	}
	db.SetMaxOpenConns(4)
	return db, dsn, nil
}

func sqlDSN(typ string, cfg SQLConfig) (driver, dsn string, err error) {
	switch typ {
	case "mysql", "mariadb":
		return "mysql", mysqlDSN(cfg), nil
	case "sqlserver":
		return "sqlserver", sqlServerDSN(cfg), nil
	case "oracle":
		port := cfg.Port
		if port == 0 {
			port = 1521
		}
		service := cfg.Database
		if service == "" {
			service = "ORCL"
		}
		return "oracle", go_ora.BuildUrl(cfg.Host, port, service, cfg.User, cfg.Password, nil), nil
	case "snowflake":
		return "snowflake", snowflakeDSN(cfg), nil
	case "postgres", "redshift":
		return "pgx", postgresDSN(cfg), fmt.Errorf("use pgx nativo para %s", typ)
	default:
		if strings.TrimSpace(cfg.URL) != "" {
			return detectODBCType(cfg), cfg.URL, nil
		}
		return "", "", fmt.Errorf("tipo SQL desconhecido: %s", typ)
	}
}

func sqlServerDSN(c SQLConfig) string {
	port := c.Port
	if port == 0 {
		port = 1433
	}
	u := &url.URL{
		Scheme: "sqlserver",
		Host:   fmt.Sprintf("%s:%d", c.Host, port),
	}
	if c.User != "" {
		u.User = url.UserPassword(c.User, c.Password)
	}
	q := url.Values{}
	if c.Database != "" {
		q.Set("database", c.Database)
	}
	if c.EffectiveSSLMode() == "disable" {
		q.Set("encrypt", "disable")
	} else {
		q.Set("encrypt", "true")
		q.Set("TrustServerCertificate", "true")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func snowflakeDSN(c SQLConfig) string {
	acc := strings.TrimSpace(c.Account)
	db := strings.TrimSpace(c.Database)
	schema := strings.TrimSpace(c.Schema)
	if schema == "" {
		schema = "PUBLIC"
	}
	dsn := fmt.Sprintf("%s:%s@%s", url.QueryEscape(c.User), url.QueryEscape(c.Password), acc)
	if db != "" {
		dsn += "/" + db
		if schema != "" {
			dsn += "/" + schema
		}
	}
	q := url.Values{}
	if c.Warehouse != "" {
		q.Set("warehouse", c.Warehouse)
	}
	if c.Role != "" {
		q.Set("role", c.Role)
	}
	if enc := q.Encode(); enc != "" {
		dsn += "?" + enc
	}
	return dsn
}

func detectODBCType(cfg SQLConfig) string {
	u := strings.ToLower(cfg.URL)
	switch {
	case strings.Contains(u, "mysql"):
		return "mysql"
	case strings.Contains(u, "sqlserver"), strings.Contains(u, "mssql"), strings.Contains(u, "odbc driver 1"):
		return "sqlserver"
	case strings.Contains(u, "oracle"):
		return "oracle"
	case strings.Contains(u, "snowflake"):
		return "snowflake"
	default:
		return "postgres"
	}
}

func selectStarSQL(typ, table string, limit int) string {
	if limit <= 0 {
		limit = 10000
	}
	switch typ {
	case "sqlserver":
		return fmt.Sprintf("SELECT TOP (%d) * FROM %s", limit, table)
	case "oracle":
		return fmt.Sprintf("SELECT * FROM %s FETCH FIRST %d ROWS ONLY", table, limit)
	default:
		return fmt.Sprintf("SELECT * FROM %s LIMIT %d", table, limit)
	}
}

func applySQLLimit(typ, q string, limit int) string {
	if limit <= 0 {
		return q
	}
	q = strings.TrimRight(strings.TrimSpace(q), ";")
	switch typ {
	case "sqlserver":
		low := strings.ToLower(q)
		if strings.HasPrefix(low, "select") && !strings.Contains(low, " top ") {
			return "SELECT TOP (" + strconv.Itoa(limit) + ") " + strings.TrimSpace(q[6:])
		}
		return q
	case "oracle":
		return q + fmt.Sprintf(" FETCH FIRST %d ROWS ONLY", limit)
	default:
		return q + fmt.Sprintf(" LIMIT %d", limit)
	}
}

func scanSQLQuery(ctx context.Context, db *sql.DB, q string, limit int) ([]string, [][]string, error) {
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	headers, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	if limit <= 0 {
		limit = 10000
	}
	var out [][]string
	for rows.Next() {
		raw := make([]sql.NullString, len(headers))
		ptrs := make([]any, len(headers))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		rec := make([]string, len(headers))
		for i, v := range raw {
			if v.Valid {
				rec[i] = v.String
			}
		}
		out = append(out, rec)
		if len(out) >= limit {
			break
		}
	}
	return headers, out, rows.Err()
}
