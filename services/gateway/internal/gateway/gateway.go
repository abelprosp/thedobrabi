package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Source struct {
	Name     string `json:"name" yaml:"name"`
	Type     string `json:"type" yaml:"type"` // postgresql, mysql, mssql
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Database string `json:"database" yaml:"database"`
	User     string `json:"user" yaml:"user"`
	Password string `json:"password" yaml:"password"`
}

type QueryRequest struct {
	SourceName string `json:"source_name"`
	SQL        string `json:"sql"`
	Limit      int    `json:"limit,omitempty"`
}

type QueryResponse struct {
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Error   string           `json:"error,omitempty"`
}

type Agent struct {
	remoteURL  string
	token      string
	instanceID string
	sources    map[string]Source
	dbCache    map[string]*sql.DB
}

func NewAgent(remoteURL, token, instanceID string, sources []Source) *Agent {
	m := make(map[string]Source, len(sources))
	for _, s := range sources {
		m[s.Name] = s
	}
	return &Agent{
		remoteURL:  remoteURL,
		token:      token,
		instanceID: instanceID,
		sources:    m,
		dbCache:    map[string]*sql.DB{},
	}
}

func (a *Agent) Query(ctx context.Context, req QueryRequest) (QueryResponse, error) {
	src, ok := a.sources[req.SourceName]
	if !ok {
		return QueryResponse{}, fmt.Errorf("source %q not found", req.SourceName)
	}
	db, err := a.getDB(ctx, src)
	if err != nil {
		return QueryResponse{}, err
	}
	if req.Limit > 10000 {
		req.Limit = 10000
	}
	rows, err := db.QueryContext(ctx, req.SQL)
	if err != nil {
		return QueryResponse{}, err
	}
	defer rows.Close()
	return collectRows(rows, req.Limit)
}

func (a *Agent) getDB(ctx context.Context, src Source) (*sql.DB, error) {
	if db, ok := a.dbCache[src.Name]; ok {
		if err := db.PingContext(ctx); err != nil {
			db.Close()
			delete(a.dbCache, src.Name)
		} else {
			return db, nil
		}
	}
	var dsn string
	switch src.Type {
	case "postgresql", "postgres":
		if src.Port == 0 {
			src.Port = 5432
		}
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", src.User, src.Password, src.Host, src.Port, src.Database)
	case "mysql":
		if src.Port == 0 {
			src.Port = 3306
		}
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", src.User, src.Password, src.Host, src.Port, src.Database)
	case "mssql", "sqlserver":
		return nil, fmt.Errorf("mssql not implemented in this MVP")
	default:
		return nil, fmt.Errorf("unsupported source type %q", src.Type)
	}
	db, err := sql.Open(src.Type, dsn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	a.dbCache[src.Name] = db
	return db, nil
}

func collectRows(rows *sql.Rows, limit int) (QueryResponse, error) {
	cols, err := rows.Columns()
	if err != nil {
		return QueryResponse{}, err
	}
	var out []map[string]any
	for rows.Next() {
		if limit > 0 && len(out) >= limit {
			break
		}
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return QueryResponse{}, err
		}
		row := map[string]any{}
		for i, c := range cols {
			row[c] = values[i]
		}
		out = append(out, row)
	}
	return QueryResponse{Columns: cols, Rows: out}, rows.Err()
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
