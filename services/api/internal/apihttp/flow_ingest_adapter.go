package apihttp

import (
	"context"

	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/flow"
	"github.com/thedobra/thedobra/services/api/internal/ingest"
)

// flowIngester adapts the ingest engine to the flow Ingester interface.
type flowIngester struct {
	ing *ingest.Engine
}

func (f *flowIngester) IngestRowsFromMaps(ctx context.Context, orgID, wsID, userID uuid.UUID, name string, headers []string, rows []map[string]any) (flow.IngestResult, error) {
	res, err := f.ing.IngestRowsFromMaps(ctx, orgID, wsID, userID, name, headers, rows)
	return flow.IngestResult{DatasetID: res.DatasetID}, err
}
