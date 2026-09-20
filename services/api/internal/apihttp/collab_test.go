package apihttp

import (
	"testing"
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
	req, _, err := buildPublicWidgetQuery([]byte(`{
		"widgets":[{"id":"w1","type":"bar","query":{"dataset_id":"ds-1","measures":["m"],"joins":[{"dataset_id":"ds-2","from_column":"a","to_column":"b"}]}}]
	}`), publicQueryInput{WidgetID: "w1"})
	if err != nil {
		t.Fatal(err)
	}
	if req.DatasetID != "ds-1" || len(req.Joins) != 1 || req.Joins[0].DatasetID != "ds-2" {
		t.Fatalf("unexpected req %#v", req)
	}
	if _, ok := req.AllowedDatasets["ds-2"]; !ok {
		t.Fatal("join dataset should be in allowlist")
	}
	_, _, err = buildPublicWidgetQuery([]byte(`{
		"widgets":[{"id":"w1","type":"bar","query":{"dataset_id":"ds-1","measures":["m"]}}]
	}`), publicQueryInput{WidgetID: "missing"})
	if err == nil {
		t.Fatal("expected missing widget rejected")
	}
}
