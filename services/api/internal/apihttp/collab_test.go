package apihttp

import (
	"testing"

	"github.com/thedobra/thedobra/services/api/internal/queryeng"
)

func TestAllowedDatasetIDs(t *testing.T) {
	layout := []byte(`{
		"widgets": [
			{"type":"bar","query":{"dataset_id":"ds-1","measures":["receita"]}},
			{"type":"slicer","query":{"dataset_id":"ds-1","dimensions":["cliente"]}},
			{"type":"kpi","query":{"dataset_id":"ds-2","joins":[{"dataset_id":"ds-3","from_column":"id","to_column":"id"}]}},
			{"type":"text","text":"hello"}
		]
	}`)
	got := allowedDatasetIDs(layout)
	for _, id := range []string{"ds-1", "ds-2", "ds-3"} {
		if _, ok := got[id]; !ok {
			t.Fatalf("missing dataset %s in %#v", id, got)
		}
	}
	if _, ok := got[""]; ok {
		t.Fatal("empty dataset id should not be allowed")
	}
	if len(allowedDatasetIDs([]byte(`{`))) != 0 {
		t.Fatal("invalid json should yield empty set")
	}
}

func TestWidgetQueryAllowed(t *testing.T) {
	allowed := map[string]struct{}{"ds-1": {}, "ds-2": {}}
	if !widgetQueryAllowed(queryeng.Request{DatasetID: "ds-1"}, allowed) {
		t.Fatal("expected ds-1 to be allowed")
	}
	if widgetQueryAllowed(queryeng.Request{DatasetID: "ds-9"}, allowed) {
		t.Fatal("expected unknown dataset to be rejected")
	}
	if widgetQueryAllowed(queryeng.Request{DatasetID: "ds-1", Joins: []queryeng.DatasetJoin{{DatasetID: "ds-9"}}}, allowed) {
		t.Fatal("expected unknown join dataset to be rejected")
	}
	if !widgetQueryAllowed(queryeng.Request{DatasetID: "ds-1", Joins: []queryeng.DatasetJoin{{DatasetID: "ds-2"}}}, allowed) {
		t.Fatal("expected join on shared dataset to be allowed")
	}
}
