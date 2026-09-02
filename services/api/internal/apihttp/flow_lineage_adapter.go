package apihttp

import (
	"context"

	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/flow"
	"github.com/thedobra/thedobra/services/api/internal/lineage"
)

// flowLineage adapts the lineage service to the flow LineageRecorder interface.
type flowLineage struct {
	svc *lineage.Service
}

func (f *flowLineage) RecordFlowToDataset(ctx context.Context, orgID, wsID, flowID, datasetID uuid.UUID, flowName string) {
	fid, _ := f.svc.Ensure(ctx, orgID, wsID, "transformation", flowID, flowName, map[string]any{"kind": "flow"})
	ds, _ := f.svc.Ensure(ctx, orgID, wsID, "dataset", datasetID, "dataset", nil)
	if fid != uuid.Nil && ds != uuid.Nil {
		f.svc.Link(ctx, orgID, wsID, fid, ds, "materializa")
	}
}

var _ flow.LineageRecorder = (*flowLineage)(nil)
