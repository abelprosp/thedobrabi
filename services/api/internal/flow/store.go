package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Flow struct {
	ID              uuid.UUID       `json:"id"`
	OrgID           uuid.UUID       `json:"org_id"`
	WorkspaceID     uuid.UUID       `json:"workspace_id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Status          string          `json:"status"`
	Schedule        string          `json:"schedule,omitempty"`
	SourceDatasetID *uuid.UUID      `json:"source_dataset_id,omitempty"`
	TargetDatasetID *uuid.UUID      `json:"target_dataset_id,omitempty"`
	OutputDatasetID *uuid.UUID      `json:"output_dataset_id,omitempty"`
	Layout          json.RawMessage `json:"layout,omitempty"`
	CreatedBy       *uuid.UUID      `json:"created_by,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type Step struct {
	ID        uuid.UUID `json:"id"`
	FlowID    uuid.UUID `json:"flow_id"`
	StepOrder int       `json:"step_order"`
	Kind      string    `json:"kind"`    // extract | transform | validate | load
	Subkind   string    `json:"subkind"` // rename, filter, change_type, join, append, aggregate, dedup, fill_null, conditional, sql, source
	Name      string    `json:"name"`
	Config    Config    `json:"config"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Config map[string]any

func (c Config) Value() (string, error) {
	if c == nil {
		return "{}", nil
	}
	b, err := json.Marshal(c)
	return string(b), err
}

type Run struct {
	ID            uuid.UUID  `json:"id"`
	FlowID        uuid.UUID  `json:"flow_id"`
	Status        string     `json:"status"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	RowsProcessed *int64     `json:"rows_processed,omitempty"`
	Error         string     `json:"error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type RunLog struct {
	ID        uuid.UUID      `json:"id"`
	RunID     uuid.UUID      `json:"run_id"`
	StepID    *uuid.UUID     `json:"step_id,omitempty"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type Store struct{ pg *pgxpool.Pool }

func NewStore(pg *pgxpool.Pool) *Store { return &Store{pg: pg} }

func (s *Store) List(ctx context.Context, orgID, wsID uuid.UUID) ([]Flow, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id, org_id, workspace_id, name, description, status, schedule, source_dataset_id, target_dataset_id, output_dataset_id, layout_json, created_by, created_at, updated_at
		FROM flows WHERE org_id=$1 AND workspace_id=$2 ORDER BY updated_at DESC
	`, orgID, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Flow
	for rows.Next() {
		var f Flow
		var layout []byte
		if err := rows.Scan(&f.ID, &f.OrgID, &f.WorkspaceID, &f.Name, &f.Description, &f.Status, &f.Schedule,
			&f.SourceDatasetID, &f.TargetDatasetID, &f.OutputDatasetID, &layout, &f.CreatedBy, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		f.Layout = layout
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) Get(ctx context.Context, orgID, wsID, id uuid.UUID) (Flow, error) {
	var f Flow
	var layout []byte
	err := s.pg.QueryRow(ctx, `
		SELECT id, org_id, workspace_id, name, description, status, schedule, source_dataset_id, target_dataset_id, output_dataset_id, layout_json, created_by, created_at, updated_at
		FROM flows WHERE id=$1 AND org_id=$2 AND workspace_id=$3
	`, id, orgID, wsID).Scan(&f.ID, &f.OrgID, &f.WorkspaceID, &f.Name, &f.Description, &f.Status, &f.Schedule,
		&f.SourceDatasetID, &f.TargetDatasetID, &f.OutputDatasetID, &layout, &f.CreatedBy, &f.CreatedAt, &f.UpdatedAt)
	f.Layout = layout
	return f, err
}

func layoutBytes(raw json.RawMessage) []byte {
	if len(raw) == 0 || string(raw) == "null" {
		return []byte("{}")
	}
	return raw
}

func inferSourceDataset(steps []Step) *uuid.UUID {
	for _, st := range steps {
		if st.Kind != "extract" || st.Config == nil {
			continue
		}
		id, _ := st.Config["dataset_id"].(string)
		if id == "" {
			continue
		}
		u, err := uuid.Parse(id)
		if err == nil {
			return &u
		}
	}
	return nil
}

func (s *Store) Create(ctx context.Context, f Flow) (uuid.UUID, error) {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	layout := layoutBytes(f.Layout)
	_, err := s.pg.Exec(ctx, `
		INSERT INTO flows (id, org_id, workspace_id, name, description, status, schedule, source_dataset_id, target_dataset_id, output_dataset_id, layout_json, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, f.ID, f.OrgID, f.WorkspaceID, f.Name, f.Description, f.Status, f.Schedule, f.SourceDatasetID, f.TargetDatasetID, f.OutputDatasetID, layout, f.CreatedBy)
	return f.ID, err
}

// CreateWithGraph creates a flow and its starter steps in one transaction, then stores a connected canvas layout.
func (s *Store) CreateWithGraph(ctx context.Context, f Flow, steps []Step) (uuid.UUID, error) {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	if f.Status == "" {
		f.Status = "draft"
	}
	if f.SourceDatasetID == nil {
		f.SourceDatasetID = inferSourceDataset(steps)
	}
	err := s.WithTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO flows (id, org_id, workspace_id, name, description, status, schedule, source_dataset_id, target_dataset_id, output_dataset_id, layout_json, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		`, f.ID, f.OrgID, f.WorkspaceID, f.Name, f.Description, f.Status, f.Schedule, f.SourceDatasetID, f.TargetDatasetID, f.OutputDatasetID, []byte("{}"), f.CreatedBy)
		if err != nil {
			return err
		}
		created := make([]Step, 0, len(steps))
		for i, st := range steps {
			if st.ID == uuid.Nil {
				st.ID = uuid.New()
			}
			st.FlowID = f.ID
			if st.StepOrder == 0 {
				st.StepOrder = i + 1
			}
			if st.Kind == "" {
				st.Kind = "transform"
			}
			if st.Config == nil {
				st.Config = Config{}
			}
			raw, err := json.Marshal(st.Config)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO flow_steps (id, flow_id, step_order, kind, subkind, name, config)
				VALUES ($1,$2,$3,$4,$5,$6,$7)
			`, st.ID, st.FlowID, st.StepOrder, st.Kind, st.Subkind, st.Name, raw); err != nil {
				return err
			}
			created = append(created, st)
		}
		layout, err := json.Marshal(DefaultLayout(created))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE flows SET layout_json=$1, updated_at=now() WHERE id=$2`, layout, f.ID)
		return err
	})
	return f.ID, err
}

func (s *Store) Update(ctx context.Context, orgID, wsID, id uuid.UUID, f Flow) error {
	layout := layoutBytes(f.Layout)
	ct, err := s.pg.Exec(ctx, `
		UPDATE flows SET
			name=CASE WHEN $1 <> '' THEN $1 ELSE name END,
			description=$2,
			status=CASE WHEN $3 <> '' THEN $3 ELSE status END,
			schedule=CASE WHEN $4 <> '' THEN $4 ELSE schedule END,
			source_dataset_id=COALESCE($5, source_dataset_id),
			target_dataset_id=COALESCE($6, target_dataset_id),
			output_dataset_id=COALESCE($7, output_dataset_id),
			layout_json=CASE WHEN $8::jsonb <> '{}'::jsonb THEN $8::jsonb ELSE layout_json END,
			updated_at=now()
		WHERE id=$9 AND org_id=$10 AND workspace_id=$11
	`, f.Name, f.Description, f.Status, f.Schedule, f.SourceDatasetID, f.TargetDatasetID, f.OutputDatasetID, layout, id, orgID, wsID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (s *Store) SetOutputDataset(ctx context.Context, flowID, datasetID uuid.UUID) error {
	_, err := s.pg.Exec(ctx, `UPDATE flows SET output_dataset_id=$1, updated_at=now() WHERE id=$2`, datasetID, flowID)
	return err
}

func (s *Store) Delete(ctx context.Context, orgID, wsID, id uuid.UUID) error {
	_, err := s.pg.Exec(ctx, `DELETE FROM flows WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, orgID, wsID)
	return err
}

func (s *Store) ListSteps(ctx context.Context, flowID uuid.UUID) ([]Step, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id, flow_id, step_order, kind, subkind, name, config, created_at, updated_at
		FROM flow_steps WHERE flow_id=$1 ORDER BY step_order
	`, flowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Step
	for rows.Next() {
		var st Step
		var raw []byte
		if err := rows.Scan(&st.ID, &st.FlowID, &st.StepOrder, &st.Kind, &st.Subkind, &st.Name, &raw, &st.CreatedAt, &st.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &st.Config)
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) CreateStep(ctx context.Context, st Step) (uuid.UUID, error) {
	if st.ID == uuid.Nil {
		st.ID = uuid.New()
	}
	raw, _ := json.Marshal(st.Config)
	_, err := s.pg.Exec(ctx, `
		INSERT INTO flow_steps (id, flow_id, step_order, kind, subkind, name, config)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, st.ID, st.FlowID, st.StepOrder, st.Kind, st.Subkind, st.Name, raw)
	return st.ID, err
}

func (s *Store) UpdateStep(ctx context.Context, stepID uuid.UUID, st Step) error {
	raw, _ := json.Marshal(st.Config)
	ct, err := s.pg.Exec(ctx, `
		UPDATE flow_steps SET
			step_order=CASE WHEN $1 = 0 THEN step_order ELSE $1 END,
			kind=$2, subkind=$3, name=$4, config=$5, updated_at=now()
		WHERE id=$6
	`, st.StepOrder, st.Kind, st.Subkind, st.Name, raw, stepID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (s *Store) DeleteStep(ctx context.Context, stepID uuid.UUID) error {
	_, err := s.pg.Exec(ctx, `DELETE FROM flow_steps WHERE id=$1`, stepID)
	return err
}

func (s *Store) CreateRun(ctx context.Context, run Run) (uuid.UUID, error) {
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	err := s.pg.QueryRow(ctx, `
		INSERT INTO flow_runs (id, flow_id, status, started_at)
		VALUES ($1,$2,$3,now()) RETURNING id, created_at
	`, run.ID, run.FlowID, run.Status).Scan(&run.ID, &run.CreatedAt)
	return run.ID, err
}

func (s *Store) UpdateRun(ctx context.Context, runID uuid.UUID, status, errStr string, rows *int64) error {
	_, err := s.pg.Exec(ctx, `
		UPDATE flow_runs SET status=$1, error=$2, rows_processed=$3, finished_at=now()
		WHERE id=$4
	`, status, errStr, rows, runID)
	return err
}

func (s *Store) ListRuns(ctx context.Context, flowID uuid.UUID) ([]Run, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id, flow_id, status, started_at, finished_at, rows_processed, error, created_at
		FROM flow_runs WHERE flow_id=$1 ORDER BY created_at DESC LIMIT 50
	`, flowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		var r Run
		if err := rows.Scan(&r.ID, &r.FlowID, &r.Status, &r.StartedAt, &r.FinishedAt, &r.RowsProcessed, &r.Error, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) AddLog(ctx context.Context, runID, stepID uuid.UUID, level, message string, details map[string]any) error {
	var sid *uuid.UUID
	if stepID != uuid.Nil {
		sid = &stepID
	}
	raw, _ := json.Marshal(details)
	_, err := s.pg.Exec(ctx, `
		INSERT INTO flow_run_logs (run_id, step_id, level, message, details)
		VALUES ($1,$2,$3,$4,$5)
	`, runID, sid, level, message, raw)
	return err
}

func (s *Store) GetRunLogs(ctx context.Context, runID uuid.UUID) ([]RunLog, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id, run_id, step_id, level, message, details, created_at
		FROM flow_run_logs WHERE run_id=$1 ORDER BY created_at
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunLog
	for rows.Next() {
		var l RunLog
		var raw []byte
		if err := rows.Scan(&l.ID, &l.RunID, &l.StepID, &l.Level, &l.Message, &raw, &l.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &l.Details)
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
