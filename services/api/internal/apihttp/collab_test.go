package apihttp

import "testing"

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
