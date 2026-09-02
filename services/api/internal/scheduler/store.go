package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Schedule struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	Kind        string     `json:"kind"`
	TargetID    uuid.UUID  `json:"target_id"`
	Enabled     bool       `json:"enabled"`
	Frequency   string     `json:"frequency"`
	HourLocal   int        `json:"hour_local"`
	Weekday     int        `json:"weekday"`
	Timezone    string     `json:"timezone"`
	Incremental bool       `json:"incremental"`
	TableName   string     `json:"table_name,omitempty"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time `json:"next_run_at,omitempty"`
	LastStatus  string     `json:"last_status"`
	LastError   *string    `json:"last_error,omitempty"`
	LastMode    string     `json:"last_mode,omitempty"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	TargetName  string     `json:"target_name,omitempty"`
	TargetType  string     `json:"target_type,omitempty"`
}

type Run struct {
	ID           uuid.UUID  `json:"id"`
	ScheduleID   uuid.UUID  `json:"schedule_id"`
	Status       string     `json:"status"`
	Mode         string     `json:"mode"`
	Error        *string    `json:"error,omitempty"`
	RowsAffected *int64     `json:"rows_affected,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

type Input struct {
	Kind        string    `json:"kind"`
	TargetID    uuid.UUID `json:"target_id"`
	Enabled     *bool     `json:"enabled"`
	Frequency   string    `json:"frequency"`
	HourLocal   *int      `json:"hour_local"`
	Weekday     *int      `json:"weekday"`
	Timezone    string    `json:"timezone"`
	Incremental *bool     `json:"incremental"`
	TableName   string    `json:"table_name"`
}

type Store struct{ pg *pgxpool.Pool }

func NewStore(pg *pgxpool.Pool) *Store { return &Store{pg: pg} }

func (s *Store) List(ctx context.Context, orgID, wsID uuid.UUID, kind string, targetID uuid.UUID) ([]Schedule, error) {
	q := `
		SELECT s.id, s.org_id, s.workspace_id, s.kind, s.target_id, s.enabled, s.frequency,
			s.hour_local, s.weekday, s.timezone, s.incremental, s.table_name,
			s.last_run_at, s.next_run_at, s.last_status, s.last_error, s.last_mode,
			s.created_by, s.created_at, s.updated_at,
			COALESCE(ds.name, f.name, d.name, '') AS target_name,
			COALESCE(ds.type, '', '') AS target_type
		FROM sync_schedules s
		LEFT JOIN data_sources ds ON ds.id = s.target_id AND s.kind = 'connector'
		LEFT JOIN flows f ON f.id = s.target_id AND s.kind = 'flow'
		LEFT JOIN datasets d ON d.id = s.target_id AND s.kind = 'dataset'
		WHERE s.org_id=$1 AND s.workspace_id=$2
	`
	args := []any{orgID, wsID}
	n := 3
	if kind != "" {
		q += fmt.Sprintf(" AND s.kind=$%d", n)
		args = append(args, kind)
		n++
	}
	if targetID != uuid.Nil {
		q += fmt.Sprintf(" AND s.target_id=$%d", n)
		args = append(args, targetID)
	}
	q += " ORDER BY s.updated_at DESC"
	rows, err := s.pg.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *Store) Get(ctx context.Context, orgID, wsID, id uuid.UUID) (Schedule, error) {
	row := s.pg.QueryRow(ctx, `
		SELECT s.id, s.org_id, s.workspace_id, s.kind, s.target_id, s.enabled, s.frequency,
			s.hour_local, s.weekday, s.timezone, s.incremental, s.table_name,
			s.last_run_at, s.next_run_at, s.last_status, s.last_error, s.last_mode,
			s.created_by, s.created_at, s.updated_at,
			COALESCE(ds.name, f.name, d.name, '') AS target_name,
			COALESCE(ds.type, '', '') AS target_type
		FROM sync_schedules s
		LEFT JOIN data_sources ds ON ds.id = s.target_id AND s.kind = 'connector'
		LEFT JOIN flows f ON f.id = s.target_id AND s.kind = 'flow'
		LEFT JOIN datasets d ON d.id = s.target_id AND s.kind = 'dataset'
		WHERE s.id=$1 AND s.org_id=$2 AND s.workspace_id=$3
	`, id, orgID, wsID)
	return scanSchedule(row)
}

func (s *Store) Upsert(ctx context.Context, orgID, wsID, userID uuid.UUID, in Input) (Schedule, error) {
	if !ValidKind(in.Kind) {
		return Schedule{}, fmt.Errorf("tipo inválido")
	}
	if in.TargetID == uuid.Nil {
		return Schedule{}, fmt.Errorf("alvo obrigatório")
	}
	if !ValidFrequency(in.Frequency) {
		return Schedule{}, fmt.Errorf("frequência inválida")
	}
	tz := strings.TrimSpace(in.Timezone)
	if tz == "" {
		tz = "America/Sao_Paulo"
	}
	if !ValidTimezone(tz) {
		return Schedule{}, fmt.Errorf("fuso horário inválido")
	}
	hour := 6
	if in.HourLocal != nil {
		hour = *in.HourLocal
	}
	weekday := 1
	if in.Weekday != nil {
		weekday = *in.Weekday
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	incr := true
	if in.Incremental != nil {
		incr = *in.Incremental
	}
	next, err := NextRun(time.Now(), in.Frequency, tz, hour, weekday)
	if err != nil {
		return Schedule{}, err
	}
	var id uuid.UUID
	err = s.pg.QueryRow(ctx, `
		INSERT INTO sync_schedules (
			org_id, workspace_id, kind, target_id, enabled, frequency, hour_local, weekday,
			timezone, incremental, table_name, next_run_at, last_status, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'idle',$13)
		ON CONFLICT (org_id, workspace_id, kind, target_id) DO UPDATE SET
			enabled=EXCLUDED.enabled,
			frequency=EXCLUDED.frequency,
			hour_local=EXCLUDED.hour_local,
			weekday=EXCLUDED.weekday,
			timezone=EXCLUDED.timezone,
			incremental=EXCLUDED.incremental,
			table_name=EXCLUDED.table_name,
			next_run_at=EXCLUDED.next_run_at,
			updated_at=now()
		RETURNING id
	`, orgID, wsID, in.Kind, in.TargetID, enabled, in.Frequency, hour, weekday, tz, incr, in.TableName, next, userID).Scan(&id)
	if err != nil {
		return Schedule{}, err
	}
	return s.Get(ctx, orgID, wsID, id)
}

func (s *Store) Patch(ctx context.Context, orgID, wsID, id uuid.UUID, in Input) (Schedule, error) {
	cur, err := s.Get(ctx, orgID, wsID, id)
	if err != nil {
		return Schedule{}, err
	}
	if in.Frequency != "" {
		if !ValidFrequency(in.Frequency) {
			return Schedule{}, fmt.Errorf("frequência inválida")
		}
		cur.Frequency = in.Frequency
	}
	if in.Timezone != "" {
		cur.Timezone = in.Timezone
	}
	if in.HourLocal != nil {
		cur.HourLocal = *in.HourLocal
	}
	if in.Weekday != nil {
		cur.Weekday = *in.Weekday
	}
	if in.Enabled != nil {
		cur.Enabled = *in.Enabled
	}
	if in.Incremental != nil {
		cur.Incremental = *in.Incremental
	}
	if in.TableName != "" {
		cur.TableName = in.TableName
	}
	if in.Timezone != "" && !ValidTimezone(in.Timezone) {
		return Schedule{}, fmt.Errorf("fuso horário inválido")
	}
	next, err := NextRun(time.Now(), cur.Frequency, cur.Timezone, cur.HourLocal, cur.Weekday)
	if err != nil {
		return Schedule{}, err
	}
	_, err = s.pg.Exec(ctx, `
		UPDATE sync_schedules SET
			enabled=$1, frequency=$2, hour_local=$3, weekday=$4, timezone=$5,
			incremental=$6, table_name=$7, next_run_at=$8, updated_at=now()
		WHERE id=$9 AND org_id=$10 AND workspace_id=$11
	`, cur.Enabled, cur.Frequency, cur.HourLocal, cur.Weekday, cur.Timezone, cur.Incremental, cur.TableName, next, id, orgID, wsID)
	if err != nil {
		return Schedule{}, err
	}
	return s.Get(ctx, orgID, wsID, id)
}

func (s *Store) SetEnabled(ctx context.Context, orgID, wsID, id uuid.UUID, enabled bool) (Schedule, error) {
	cur, err := s.Get(ctx, orgID, wsID, id)
	if err != nil {
		return Schedule{}, err
	}
	next := cur.NextRunAt
	if enabled {
		n, err := NextRun(time.Now(), cur.Frequency, cur.Timezone, cur.HourLocal, cur.Weekday)
		if err != nil {
			return Schedule{}, err
		}
		next = &n
	}
	_, err = s.pg.Exec(ctx, `
		UPDATE sync_schedules SET enabled=$1, next_run_at=$2, updated_at=now()
		WHERE id=$3 AND org_id=$4 AND workspace_id=$5
	`, enabled, next, id, orgID, wsID)
	if err != nil {
		return Schedule{}, err
	}
	return s.Get(ctx, orgID, wsID, id)
}

func (s *Store) Delete(ctx context.Context, orgID, wsID, id uuid.UUID) error {
	tag, err := s.pg.Exec(ctx, `DELETE FROM sync_schedules WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, orgID, wsID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (s *Store) DeleteForTarget(ctx context.Context, kind string, targetID uuid.UUID) {
	_, _ = s.pg.Exec(ctx, `DELETE FROM sync_schedules WHERE kind=$1 AND target_id=$2`, kind, targetID)
}

func (s *Store) ClaimDue(ctx context.Context, limit int) ([]Schedule, error) {
	if limit <= 0 {
		limit = 8
	}
	rows, err := s.pg.Query(ctx, `
		UPDATE sync_schedules s
		SET last_status='running', updated_at=now()
		FROM (
			SELECT id FROM sync_schedules
			WHERE enabled
			  AND next_run_at IS NOT NULL
			  AND next_run_at <= now()
			  AND (last_status IS DISTINCT FROM 'running' OR updated_at < now() - interval '15 minutes')
			ORDER BY next_run_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		) due
		WHERE s.id = due.id
		RETURNING s.id, s.org_id, s.workspace_id, s.kind, s.target_id, s.enabled, s.frequency,
			s.hour_local, s.weekday, s.timezone, s.incremental, s.table_name,
			s.last_run_at, s.next_run_at, s.last_status, s.last_error, s.last_mode,
			s.created_by, s.created_at, s.updated_at, '', ''
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *Store) Finish(ctx context.Context, id uuid.UUID, status, mode, errMsg string, rows *int64) {
	next := (*time.Time)(nil)
	var freq, tz string
	var hour, weekday int
	_ = s.pg.QueryRow(ctx, `SELECT frequency, timezone, hour_local, weekday FROM sync_schedules WHERE id=$1`, id).
		Scan(&freq, &tz, &hour, &weekday)
	if n, err := NextRun(time.Now(), freq, tz, hour, weekday); err == nil {
		next = &n
	}
	var errArg any
	if errMsg != "" {
		errArg = errMsg
	}
	_, _ = s.pg.Exec(ctx, `
		UPDATE sync_schedules SET
			last_run_at=now(), last_status=$2, last_error=$3, last_mode=$4, next_run_at=$5, updated_at=now()
		WHERE id=$1
	`, id, status, errArg, mode, next)
	_, _ = s.pg.Exec(ctx, `
		INSERT INTO sync_schedule_runs (schedule_id, status, mode, error, rows_affected, finished_at)
		VALUES ($1,$2,$3,$4,$5,now())
	`, id, status, mode, errArg, rows)
}

func (s *Store) ListRuns(ctx context.Context, orgID, wsID, scheduleID uuid.UUID) ([]Run, error) {
	if _, err := s.Get(ctx, orgID, wsID, scheduleID); err != nil {
		return nil, err
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id, schedule_id, status, mode, error, rows_affected, started_at, finished_at
		FROM sync_schedule_runs WHERE schedule_id=$1 ORDER BY started_at DESC LIMIT 30
	`, scheduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		var r Run
		if err := rows.Scan(&r.ID, &r.ScheduleID, &r.Status, &r.Mode, &r.Error, &r.RowsAffected, &r.StartedAt, &r.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSchedule(row rowScanner) (Schedule, error) {
	var sc Schedule
	var lastErr *string
	err := row.Scan(
		&sc.ID, &sc.OrgID, &sc.WorkspaceID, &sc.Kind, &sc.TargetID, &sc.Enabled, &sc.Frequency,
		&sc.HourLocal, &sc.Weekday, &sc.Timezone, &sc.Incremental, &sc.TableName,
		&sc.LastRunAt, &sc.NextRunAt, &sc.LastStatus, &lastErr, &sc.LastMode,
		&sc.CreatedBy, &sc.CreatedAt, &sc.UpdatedAt, &sc.TargetName, &sc.TargetType,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Schedule{}, fmt.Errorf("not found")
		}
		return Schedule{}, err
	}
	sc.LastError = lastErr
	return sc, nil
}
