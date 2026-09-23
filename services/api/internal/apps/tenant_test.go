package apps

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestOrgACannotAttachOrgBDashboard(t *testing.T) {
	dashA := uuid.New()
	dashB := uuid.New()
	// Org A workspace only owns dashA (result of tenant-scoped SELECT).
	foundInOrgA := map[uuid.UUID]struct{}{dashA: {}}

	if err := RejectMissingIDs([]uuid.UUID{dashB}, foundInOrgA); !errors.Is(err, ErrNotInWorkspace) {
		t.Fatalf("org A attaching org B dashboard: got %v, want ErrNotInWorkspace", err)
	}
	if err := RejectMissingIDs([]uuid.UUID{dashA, dashB}, foundInOrgA); !errors.Is(err, ErrNotInWorkspace) {
		t.Fatalf("mixed attach with foreign id: got %v, want ErrNotInWorkspace", err)
	}
	if err := RejectMissingIDs([]uuid.UUID{dashA}, foundInOrgA); err != nil {
		t.Fatalf("own dashboard should attach: %v", err)
	}
	if err := RejectMissingIDs(nil, foundInOrgA); err != nil {
		t.Fatalf("empty list should be ok: %v", err)
	}
}

func TestOrgACannotAttachOrgBReport(t *testing.T) {
	repA := uuid.New()
	repB := uuid.New()
	foundInOrgA := map[uuid.UUID]struct{}{repA: {}}

	if err := RejectMissingIDs([]uuid.UUID{repB}, foundInOrgA); !errors.Is(err, ErrNotInWorkspace) {
		t.Fatalf("org A attaching org B report: got %v, want ErrNotInWorkspace", err)
	}
}
