package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

type JobResult struct {
	Mode string
	Rows *int64
	Err  error
}

type Jobs struct {
	Connector func(ctx context.Context, orgID, wsID, userID, sourceID uuid.UUID, table string, incremental bool) (JobResult, error)
	Flow      func(ctx context.Context, orgID, wsID, userID, flowID uuid.UUID) (JobResult, error)
	Dataset   func(ctx context.Context, orgID, wsID, userID, datasetID uuid.UUID, incremental bool) (JobResult, error)
}

type Runner struct {
	store *Store
	log   *slog.Logger
	jobs  Jobs
}

func NewRunner(store *Store, log *slog.Logger, jobs Jobs) *Runner {
	return &Runner{store: store, log: log, jobs: jobs}
}

func (r *Runner) RunLoop(ctx context.Context) {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	r.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.tick(ctx)
		}
	}
}

func (r *Runner) tick(ctx context.Context) {
	due, err := r.store.ClaimDue(ctx, 8)
	if err != nil {
		if r.log != nil {
			r.log.Warn("sync_schedules.claim", "err", err)
		}
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, sc := range due {
		sc := sc
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			r.Execute(ctx, sc, false)
		}()
	}
	wg.Wait()
}

func (r *Runner) Execute(parent context.Context, sc Schedule, manual bool) {
	_ = manual
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 10*time.Minute)
	defer cancel()
	defer func() {
		if rec := recover(); rec != nil && r.log != nil {
			r.log.Error("sync_schedule.panic", "id", sc.ID, "panic", rec)
			msg := fmt.Sprint(rec)
			r.store.Finish(ctx, sc.ID, "error", "full", msg, nil)
		}
	}()
	uid := uuid.Nil
	if sc.CreatedBy != nil {
		uid = *sc.CreatedBy
	}
	var res JobResult
	var err error
	switch sc.Kind {
	case "connector":
		if r.jobs.Connector != nil {
			res, err = r.jobs.Connector(ctx, sc.OrgID, sc.WorkspaceID, uid, sc.TargetID, sc.TableName, sc.Incremental)
		} else {
			err = errNoJob
		}
	case "flow":
		if r.jobs.Flow != nil {
			res, err = r.jobs.Flow(ctx, sc.OrgID, sc.WorkspaceID, uid, sc.TargetID)
		} else {
			err = errNoJob
		}
	case "dataset":
		if r.jobs.Dataset != nil {
			res, err = r.jobs.Dataset(ctx, sc.OrgID, sc.WorkspaceID, uid, sc.TargetID, sc.Incremental)
		} else {
			err = errNoJob
		}
	default:
		err = errNoJob
	}
	status := "ok"
	errMsg := ""
	mode := res.Mode
	if mode == "" {
		mode = "full"
	}
	if err != nil {
		status = "error"
		errMsg = err.Error()
		if r.log != nil {
			r.log.Warn("sync_schedule.run", "id", sc.ID, "kind", sc.Kind, "err", err)
		}
	}
	r.store.Finish(ctx, sc.ID, status, mode, errMsg, res.Rows)
}

var errNoJob = errString("executor em falta")

type errString string

func (e errString) Error() string { return string(e) }
