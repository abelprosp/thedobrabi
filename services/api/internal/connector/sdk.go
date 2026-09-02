package connector

import "context"

// Interface is the Connector SDK. New sources implement this without
// changing the ingestion core.
type Interface interface {
	Connect(ctx context.Context) error
	Authenticate(ctx context.Context) error
	DiscoverSchema(ctx context.Context) ([]Table, error)
	Estimate(ctx context.Context, table string) (Estimate, error)
	Sync(ctx context.Context, table string, emit func(batch [][]string) error) error
	IncrementalSync(ctx context.Context, table string, cursor string, emit func(batch [][]string) error) (newCursor string, err error)
	Disconnect(ctx context.Context) error
}

type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
}

type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Estimate struct {
	Rows      int64 `json:"rows"`
	Bytes     int64 `json:"bytes"`
	Truncated bool  `json:"truncated"`
}
