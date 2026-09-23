package cdc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
	"github.com/thedobra/thedobra/services/api/internal/ingest"
	"github.com/thedobra/thedobra/services/api/internal/platform"
)

// Limitations (append-only watermark CDC — not logical decoding):
//   - Source UPDATEs are appended as new ClickHouse rows (no in-place merge).
//   - Source DELETEs are not propagated.
//   - Schema changes on the source are not followed automatically.
//   - Crash after a successful ClickHouse insert but before recording the applied
//     batch can still briefly re-insert until cdc_applied_batches is written;
//     subsequent retries with the same end-cursor are skipped.
const (
	cursorSep   = "\x1f"
	leaseTTL    = 45 * time.Second
	claimLimit  = 16
)

type Engine struct {
	pg     *pgxpool.Pool
	ingest *ingest.Engine
	cfgKey []byte
	log    *slog.Logger
	bus    platform.EventBus
	owner  string
}

func New(pg *pgxpool.Pool, ing *ingest.Engine, key []byte, log *slog.Logger, bus platform.EventBus) *Engine {
	owner, _ := os.Hostname()
	if owner == "" {
		owner = "api"
	}
	owner = owner + ":" + uuid.NewString()[:8]
	return &Engine{pg: pg, ingest: ing, cfgKey: key, log: log, bus: bus, owner: owner}
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

func (e *Engine) PollSource(ctx context.Context, orgID, wsID, sourceID uuid.UUID) (int, error) {
	claimed, err := e.claimCheckpoints(ctx, claimLimit, &sourceID, orgID, wsID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range claimed {
		if err := e.poll(ctx, c); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (e *Engine) HasCheckpoint(ctx context.Context, orgID, wsID, sourceID uuid.UUID) bool {
	var n int
	_ = e.pg.QueryRow(ctx, `
		SELECT COUNT(*) FROM cdc_checkpoints
		WHERE org_id=$1 AND workspace_id=$2 AND data_source_id=$3 AND status='running'
	`, orgID, wsID, sourceID).Scan(&n)
	return n > 0
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

type checkpointClaim struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	WsID      uuid.UUID
	SourceID  uuid.UUID
	DatasetID uuid.UUID
	Table     string
	Cursor    string
}

func (e *Engine) tick(ctx context.Context) {
	claimed, err := e.claimCheckpoints(ctx, claimLimit, nil, uuid.Nil, uuid.Nil)
	if err != nil {
		if e.log != nil {
			e.log.Warn("cdc claim", "err", err)
		}
		return
	}
	for _, c := range claimed {
		if err := e.poll(ctx, c); err != nil && e.log != nil {
			e.log.Warn("cdc", "err", err, "table", c.Table)
			_, _ = e.pg.Exec(ctx, `UPDATE cdc_checkpoints SET last_error=$2, updated_at=now() WHERE id=$1`, c.ID, err.Error())
		}
	}
}

// claimCheckpoints exclusively leases due CDC checkpoints via FOR UPDATE SKIP LOCKED
// so only one API replica processes a given checkpoint at a time.
func (e *Engine) claimCheckpoints(ctx context.Context, limit int, sourceID *uuid.UUID, orgID, wsID uuid.UUID) ([]checkpointClaim, error) {
	if limit <= 0 {
		limit = claimLimit
	}
	var src any
	if sourceID != nil {
		src = *sourceID
	}
	rows, err := e.pg.Query(ctx, `
		UPDATE cdc_checkpoints c
		SET lease_owner=$2, lease_until=now() + $3::interval, updated_at=now()
		FROM (
			SELECT id FROM cdc_checkpoints
			WHERE status='running'
			  AND (lease_until IS NULL OR lease_until < now())
			  AND ($4::uuid IS NULL OR data_source_id=$4)
			  AND ($5::uuid IS NULL OR org_id=$5)
			  AND ($6::uuid IS NULL OR workspace_id=$6)
			ORDER BY COALESCE(last_event_at, updated_at) ASC NULLS FIRST
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		) due
		WHERE c.id = due.id
		RETURNING c.id, c.org_id, c.workspace_id, c.data_source_id,
			COALESCE(c.dataset_id, '00000000-0000-0000-0000-000000000000'::uuid),
			c.table_name, COALESCE(c.cursor_value, '')
	`, limit, e.owner, leaseTTL.String(), src, nullUUID(orgID), nullUUID(wsID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []checkpointClaim{}
	for rows.Next() {
		var c checkpointClaim
		if err := rows.Scan(&c.ID, &c.OrgID, &c.WsID, &c.SourceID, &c.DatasetID, &c.Table, &c.Cursor); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func nullUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func (e *Engine) poll(ctx context.Context, c checkpointClaim) error {
	var typ, enc string
	if err := e.pg.QueryRow(ctx, `SELECT type, config_enc FROM data_sources WHERE id=$1 AND org_id=$2`, c.SourceID, c.OrgID).Scan(&typ, &enc); err != nil {
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
	if c.Table != "" {
		cfg.Table = c.Table
	}
	if !safeIdent(cfg.Table) {
		return fmt.Errorf("tabela inválida")
	}
	if typ != "postgres" {
		return e.pollViaIngest(ctx, c, cfg.Table)
	}

	if err := ingest.AssertConnectorConfig(cfg); err != nil {
		return err
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
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	cursorCol, pkCol, err := detectCursorColumns(ctx, conn, cfg.Table)
	if err != nil {
		return err
	}
	var lsn string
	_ = conn.QueryRow(ctx, `SELECT COALESCE(pg_current_wal_lsn()::text, '')`).Scan(&lsn)

	cur, curPK := decodeCursor(c.Cursor)
	if cur == "" {
		var maxv string
		q := "SELECT COALESCE(MAX(" + cursorCol + ")::text, '') FROM " + cfg.Table
		if err := conn.QueryRow(ctx, q).Scan(&maxv); err != nil {
			return err
		}
		var n int64
		_ = conn.QueryRow(ctx, "SELECT COUNT(*) FROM "+cfg.Table).Scan(&n)
		init := encodeCursor(maxv, "")
		_, err = e.pg.Exec(ctx, `
			UPDATE cdc_checkpoints
			SET cursor_value=$2, lsn=$3, rows_applied=$4, last_event_at=now(), last_error=NULL, updated_at=now(),
			    lease_until=now()
			WHERE id=$1 AND lease_owner=$5
		`, c.ID, init, lsn, n, e.owner)
		return err
	}

	headers, recs, next, nextPK, err := e.ingest.ReadSQLIncremental(ctx, c.OrgID, c.WsID, c.SourceID, cfg.Table, cursorCol, pkCol, cur, curPK, 10000)
	if err != nil {
		return err
	}
	endCursor := encodeCursor(next, nextPK)
	if endCursor == "" {
		endCursor = c.Cursor
	}
	return e.applyBatch(ctx, c, headers, recs, endCursor, lsn)
}

func (e *Engine) pollViaIngest(ctx context.Context, c checkpointClaim, table string) error {
	col, pk := "id", ""
	cur, curPK := decodeCursor(c.Cursor)
	if cur == "" {
		cur = "0"
		_, err := e.pg.Exec(ctx, `
			UPDATE cdc_checkpoints
			SET cursor_value=$2, last_event_at=now(), updated_at=now(), lease_until=now()
			WHERE id=$1 AND lease_owner=$3
		`, c.ID, encodeCursor(cur, ""), e.owner)
		return err
	}
	headers, recs, next, nextPK, err := e.ingest.ReadSQLIncremental(ctx, c.OrgID, c.WsID, c.SourceID, table, col, pk, cur, curPK, 10000)
	if err != nil {
		return err
	}
	return e.applyBatch(ctx, c, headers, recs, encodeCursor(next, nextPK), "")
}

// applyBatch inserts rows then advances the checkpoint. Checkpoint movement and
// cdc_applied_batches recording happen only after a successful insert (or when
// the same end-cursor was already applied — idempotent retry).
func (e *Engine) applyBatch(ctx context.Context, c checkpointClaim, headers []string, recs [][]string, endCursor, lsn string) error {
	if endCursor == "" {
		endCursor = c.Cursor
	}

	var already bool
	_ = e.pg.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM cdc_applied_batches WHERE checkpoint_id=$1 AND end_cursor=$2)
	`, c.ID, endCursor).Scan(&already)

	applied := int64(0)
	if len(recs) > 0 && !already {
		if c.DatasetID == uuid.Nil {
			return fmt.Errorf("checkpoint sem dataset de destino")
		}
		var err error
		applied, err = e.ingest.AppendToDataset(ctx, c.OrgID, c.WsID, c.DatasetID, headers, recs)
		if err != nil {
			return err
		}
	}

	tx, err := e.pg.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if !already && endCursor != c.Cursor {
		if _, err := tx.Exec(ctx, `
			INSERT INTO cdc_applied_batches (checkpoint_id, end_cursor, rows_applied)
			VALUES ($1,$2,$3)
			ON CONFLICT DO NOTHING
		`, c.ID, endCursor, applied); err != nil {
			return err
		}
	}

	// CAS on cursor_value so a stolen/expired lease cannot clobber a newer watermark.
	tag, err := tx.Exec(ctx, `
		UPDATE cdc_checkpoints
		SET cursor_value=$2, lsn=COALESCE(NULLIF($3,''), lsn),
		    rows_applied=rows_applied+$4, last_event_at=now(), last_error=NULL, updated_at=now(),
		    lease_until=now()
		WHERE id=$1 AND lease_owner=$5 AND COALESCE(cursor_value,'')=$6
	`, c.ID, endCursor, lsn, applied, e.owner, c.Cursor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("checkpoint lease lost or cursor raced")
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if applied > 0 || len(recs) > 0 {
		_ = e.bus.Publish(ctx, "dataset.updated", platform.Event{
			Type: "cdc.apply", OrgID: c.OrgID.String(), WorkspaceID: c.WsID.String(),
			Payload: map[string]any{"table": c.Table, "lsn": lsn, "applied": applied, "cursor": endCursor},
		})
	}
	return nil
}

func encodeCursor(v, pk string) string {
	if pk == "" {
		return v
	}
	return v + cursorSep + pk
}

func decodeCursor(raw string) (v, pk string) {
	if raw == "" {
		return "", ""
	}
	if i := strings.Index(raw, cursorSep); i >= 0 {
		return raw[:i], raw[i+len(cursorSep):]
	}
	return raw, ""
}

func detectCursorColumns(ctx context.Context, conn *pgx.Conn, table string) (cursorCol, pkCol string, err error) {
	schema, name := "public", table
	if i := strings.LastIndex(table, "."); i >= 0 {
		schema, name = table[:i], table[i+1:]
	}
	err = conn.QueryRow(ctx, `
		SELECT column_name FROM information_schema.columns
		WHERE table_schema=$1 AND table_name=$2
		ORDER BY CASE lower(column_name)
			WHEN 'updated_at' THEN 1 WHEN 'updated' THEN 2 WHEN 'modified_at' THEN 3
			WHEN 'modified' THEN 4 WHEN 'id' THEN 5 ELSE 9 END
		LIMIT 1
	`, schema, name).Scan(&cursorCol)
	if err != nil {
		return "", "", fmt.Errorf("sem coluna de cursor (updated_at/id): %w", err)
	}
	if !safeIdent(cursorCol) {
		return "", "", fmt.Errorf("coluna cursor inválida")
	}

	_ = conn.QueryRow(ctx, `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		 AND tc.table_schema = kcu.table_schema
		WHERE tc.table_schema=$1 AND tc.table_name=$2 AND tc.constraint_type='PRIMARY KEY'
		ORDER BY kcu.ordinal_position
		LIMIT 1
	`, schema, name).Scan(&pkCol)
	if pkCol == "" {
		var hasID bool
		_ = conn.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM information_schema.columns
				WHERE table_schema=$1 AND table_name=$2 AND lower(column_name)='id'
			)
		`, schema, name).Scan(&hasID)
		if hasID {
			pkCol = "id"
		}
	}
	if pkCol != "" && !safeIdent(pkCol) {
		pkCol = ""
	}
	if strings.EqualFold(pkCol, cursorCol) {
		pkCol = ""
	}
	return cursorCol, pkCol, nil
}

func safeIdent(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.ContainsAny(s, ";\"'")
}
