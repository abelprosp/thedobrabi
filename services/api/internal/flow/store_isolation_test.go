package flow

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Guardrail: step/run store methods must take org/ws and filter via the parent flow.
func TestStepAndRunSQLEnforcesTenant(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	storeSrc, err := os.ReadFile(filepath.Join(dir, "store.go"))
	if err != nil {
		t.Fatal(err)
	}
	engineSrc, err := os.ReadFile(filepath.Join(dir, "engine.go"))
	if err != nil {
		t.Fatal(err)
	}
	combined := string(storeSrc) + "\n" + string(engineSrc)

	sigs := []string{
		"func (s *Store) ListSteps(ctx context.Context, orgID, wsID, flowID uuid.UUID)",
		"func (s *Store) CreateStep(ctx context.Context, orgID, wsID uuid.UUID, st Step)",
		"func (s *Store) UpdateStep(ctx context.Context, orgID, wsID, flowID, stepID uuid.UUID, st Step)",
		"func (s *Store) DeleteStep(ctx context.Context, orgID, wsID, flowID, stepID uuid.UUID)",
		"func (s *Store) CreateRun(ctx context.Context, orgID, wsID uuid.UUID, run Run)",
		"func (s *Store) ListRuns(ctx context.Context, orgID, wsID, flowID uuid.UUID)",
		"func (s *Store) GetRunLogs(ctx context.Context, orgID, wsID, runID uuid.UUID)",
		"func (s *Store) GetRun(ctx context.Context, orgID, wsID, runID uuid.UUID)",
		"func (s *Store) UpdateRun(ctx context.Context, orgID, wsID, runID uuid.UUID",
		"func (s *Store) SetOutputDataset(ctx context.Context, orgID, wsID, flowID, datasetID uuid.UUID)",
		"func (s *Store) Delete(ctx context.Context, orgID, wsID, id uuid.UUID)",
		"func (e *Engine) Execute(ctx context.Context, orgID, wsID, runID, userID uuid.UUID",
	}
	for _, sig := range sigs {
		if !strings.Contains(combined, sig) {
			t.Errorf("missing tenant-scoped signature: %s", sig)
		}
	}

	// Child-resource SQL must reference flows + both tenant columns.
	needFlowsJoin := []string{
		"ListSteps", "CreateStep", "UpdateStep", "DeleteStep",
		"CreateRun", "ListRuns", "GetRunLogs", "GetRun", "UpdateRun",
	}
	for _, name := range needFlowsJoin {
		body := sliceMethod(combined, "func (s *Store) "+name+"(")
		if body == "" {
			t.Errorf("could not locate Store.%s", name)
			continue
		}
		if !strings.Contains(body, "org_id") || !strings.Contains(body, "workspace_id") {
			t.Errorf("Store.%s must filter org_id and workspace_id", name)
		}
		if !strings.Contains(body, "flows") {
			t.Errorf("Store.%s must go through flows for tenant isolation", name)
		}
	}

	delBody := sliceMethod(combined, "func (s *Store) Delete(")
	if !strings.Contains(delBody, "org_id") || !strings.Contains(delBody, "workspace_id") {
		t.Error("Store.Delete must filter org_id and workspace_id")
	}
}

func sliceMethod(src, marker string) string {
	i := strings.Index(src, marker)
	if i < 0 {
		return ""
	}
	rest := src[i:]
	// Take until the next Store/Engine method or EOF, capped for safety.
	next := strings.Index(rest[len(marker):], "\nfunc (")
	if next < 0 {
		if len(rest) > 2500 {
			return rest[:2500]
		}
		return rest
	}
	end := len(marker) + next
	return rest[:end]
}
