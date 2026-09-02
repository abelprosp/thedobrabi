package cdc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
	"github.com/thedobra/thedobra/services/api/internal/ingest"
	"github.com/thedobra/thedobra/services/api/internal/platform"
)

type Engine struct {
	pg     *pgxpool.Pool
	ingest *ingest.Engine
	cfgKey []byte
	log    *slog.Logger
	bus    platform.EventBus
}

func New(pg *pgxpool.Pool, ing *ingest.Engine, key []byte, log *slog.Logger, bus platform.EventBus) *Engine {
	return &Engine{pg: pg, ingest: ing, cfgKey: key, log: log, bus: bus}
}

func (e *Engine) Enable(ctx context.Context, orgID, wsID, sourceID, datasetID uuid.UUID, table string) error {
	var ds any
	if datasetID != uuid.Nil {
		ds = datasetID
	}
	_, err := e.pg.Exec(ctx, `
		INSERT INTO cdc_checkpoints (org_id, workspace_id, data_source_id, dataset_id, table_name, status)
		VALUES ($1,$2,$3,$4,$5,'running')
		ON CONFLICT (data_source_id, table_name) DO UPDATE SET status='running', dataset_id=EXCLUDED.dataset_id, updated_at=now()
	`, orgID, wsID, sourceID, ds, table)
	return err
}

func (e *Engine) List(ctx context.Context, orgID, wsID uuid.UUID) ([]map[string]any, error) {
	rows, err := e.pg.Query(ctx, `
		SELECT id, data_source_id, dataset_id, table_name, status, rows_applied, last_event_at, last_error, cursor_value
		FROM cdc_checkpoints WHERE org_id=$1 AND workspace_id=$2 ORDER BY updated_at DESC
	`, orgID, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, src uuid.UUID
		var ds *uuid.UUID
		var table, status string
		var applied int64
		var last *time.Time
		var errMsg, cursor *string
		if err := rows.Scan(&id, &src, &ds, &table, &status, &applied, &last, &errMsg, &cursor); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "data_source_id": src, "dataset_id": ds, "table": table,
			"status": status, "rows_applied": applied, "last_event_at": last, "last_error": errMsg, "cursor": cursor,
		})
	}
	return out, rows.Err()
}

func (e *Engine) RunLoop(ctx context.Context) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.tick(ctx)
		}
	}
}

func (e *Engine) tick(ctx context.Context) {
	rows, err := e.pg.Query(ctx, `SELECT id, org_id, workspace_id, data_source_id, COALESCE(dataset_id, '00000000-0000-0000-0000-000000000000'::uuid), table_name, COALESCE(cursor_value,'') FROM cdc_checkpoints WHERE status='running'`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id, org, ws, src, ds uuid.UUID
		var table, cursor string
		if err := rows.Scan(&id, &org, &ws, &src, &ds, &table, &cursor); err != nil {
			continue
		}
		if err := e.poll(ctx, id, org, ws, src, ds, table, cursor); err != nil && e.log != nil {
			e.log.Warn("cdc", "err", err, "table", table)
			_, _ = e.pg.Exec(ctx, `UPDATE cdc_checkpoints SET last_error=$2, updated_at=now() WHERE id=$1`, id, err.Error())
		}
	}
}

func (e *Engine) poll(ctx context.Context, id, org, ws, src, datasetID uuid.UUID, table, cursor string) error {
	var typ, enc string
	if err := e.pg.QueryRow(ctx, `SELECT type, config_enc FROM data_sources WHERE id=$1 AND org_id=$2`, src, org).Scan(&typ, &enc); err != nil {
		return err
	}
	if typ != "postgres" && typ != "mysql" {
		return fmt.Errorf("CDC cobre PostgreSQL e MySQL")
	}
	plain, err := cryptoenc.Decrypt(e.cfgKey, enc)
	if err != nil {
		return err
	}
	var cfg ingest.SQLConfig
	if err := json.Unmarshal([]byte(plain), &cfg); err != nil {
		return err
	}
	if table != "" {
		cfg.Table = table
	}
	if !safeIdent(cfg.Table) {
		return fmt.Errorf("tabela inválida")
	}
	port := cfg.Port
	if port == 0 {
		port = 5432
	}
	ssl := cfg.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", cfg.User, cfg.Password, cfg.Host, port, cfg.Database, ssl)
	if typ != "postgres" {
		return e.pollViaIngest(ctx, id, org, ws, src, datasetID, cfg.Table, cursor)
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	col, err := detectCursorColumn(ctx, conn, cfg.Table)
	if err != nil {
		return err
	}
	var lsn string
	_ = conn.QueryRow(ctx, `SELECT COALESCE(pg_current_wal_lsn()::text, '')`).Scan(&lsn)

	if cursor == "" {
		var maxv string
		q := "SELECT COALESCE(MAX(" + col + ")::text, '') FROM " + cfg.Table
		if err := conn.QueryRow(ctx, q).Scan(&maxv); err != nil {
			return err
		}
		var n int64
		_ = conn.QueryRow(ctx, "SELECT COUNT(*) FROM "+cfg.Table).Scan(&n)
		_, err = e.pg.Exec(ctx, `
			UPDATE cdc_checkpoints
			SET cursor_value=$2, lsn=$3, rows_applied=$4, last_event_at=now(), last_error=NULL, updated_at=now()
			WHERE id=$1
		`, id, maxv, lsn, n)
		return err
	}

	headers, recs, next, err := e.ingest.ReadSQLIncremental(ctx, org, ws, src, cfg.Table, col, cursor, 10000)
	if err != nil {
		return err
	}
	applied := int64(0)
	if len(recs) > 0 && datasetID != uuid.Nil {
		applied, err = e.ingest.AppendToDataset(ctx, org, ws, datasetID, headers, recs)
		if err != nil {
			return err
		}
	}
	if next == "" {
		next = cursor
	}
	_ = e.bus.Publish(ctx, "dataset.updated", platform.Event{
		Type: "cdc.apply", OrgID: org.String(), WorkspaceID: ws.String(),
		Payload: map[string]any{"table": cfg.Table, "lsn": lsn, "applied": applied, "cursor": next},
	})
	_, err = e.pg.Exec(ctx, `
		UPDATE cdc_checkpoints
		SET cursor_value=$2, lsn=$3, rows_applied=rows_applied+$4, last_event_at=now(), last_error=NULL, updated_at=now()
		WHERE id=$1
	`, id, next, lsn, applied)
	return err
}

func (e *Engine) pollViaIngest(ctx context.Context, id, org, ws, src, datasetID uuid.UUID, table, cursor string) error {
	col := "id"
	if cursor == "" {
		cursor = "0"
		_, err := e.pg.Exec(ctx, `UPDATE cdc_checkpoints SET cursor_value=$2, last_event_at=now(), updated_at=now() WHERE id=$1`, id, cursor)
		return err
	}
	headers, recs, next, err := e.ingest.ReadSQLIncremental(ctx, org, ws, src, table, col, cursor, 10000)
	if err != nil {
		return err
	}
	var applied int64
	if len(recs) > 0 && datasetID != uuid.Nil {
		applied, err = e.ingest.AppendToDataset(ctx, org, ws, datasetID, headers, recs)
		if err != nil {
			return err
		}
	}
	_, err = e.pg.Exec(ctx, `UPDATE cdc_checkpoints SET cursor_value=$2, rows_applied=rows_applied+$3, last_event_at=now(), last_error=NULL, updated_at=now() WHERE id=$1`, id, next, applied)
	return err
}

func detectCursorColumn(ctx context.Context, conn *pgx.Conn, table string) (string, error) {
	schema, name := "public", table
	if i := strings.LastIndex(table, "."); i >= 0 {
		schema, name = table[:i], table[i+1:]
	}
	rows, err := conn.Query(ctx, `
		SELECT column_name FROM information_schema.columns
		WHERE table_schema=$1 AND table_name=$2
		ORDER BY CASE lower(column_name)
			WHEN 'updated_at' THEN 1 WHEN 'updated' THEN 2 WHEN 'modified_at' THEN 3
			WHEN 'modified' THEN 4 WHEN 'id' THEN 5 ELSE 9 END
		LIMIT 1
	`, schema, name)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return "", err
		}
		if !safeIdent(col) {
			return "", fmt.Errorf("coluna cursor inválida")
		}
		return col, nil
	}
	return "", fmt.Errorf("sem coluna de cursor (updated_at/id)")
}

func safeIdent(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.ContainsAny(s, ";\"'")
}
