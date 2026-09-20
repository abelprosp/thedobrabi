package apihttp

import (
	"testing"

	"github.com/thedobra/thedobra/services/api/internal/queryeng"
)

func TestBuildPublicWidgetQuery_UsesSavedConfig(t *testing.T) {
	layout := []byte(`{
		"widgets": [
			{"id":"w1","type":"bar","query":{"dataset_id":"ds-1","measures":["receita"],"dimensions":["mes"],"filters":[{"dimension":"pais","op":"eq","value":"PT"}]}},
			{"id":"s1","type":"slicer","query":{"dataset_id":"ds-1","dimensions":["cliente"]}}
		]
	}`)
	req, _, err := buildPublicWidgetQuery(layout, publicQueryInput{
		WidgetID: "w1",
		Filters: []queryeng.Filter{
			{Dimension: "cliente", Op: "eq", Value: "Acme"},
			{Dimension: "secret_col", Op: "eq", Value: "x"},
			{Dimension: "mes", Op: "gt", Value: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.DatasetID != "ds-1" || len(req.Measures) != 1 || req.Measures[0] != "receita" {
		t.Fatalf("unexpected measures/dataset: %#v", req)
	}
	if len(req.Dimensions) != 1 || req.Dimensions[0] != "mes" {
		t.Fatalf("visitor must not override dimensions: %#v", req.Dimensions)
	}
	if !req.DisableAutoJoins {
		t.Fatal("expected DisableAutoJoins")
	}
	if req.Joins == nil {
		t.Fatal("joins should be non-nil empty slice")
	}
	// fixed filter + published slicer filter only
	if len(req.Filters) != 2 {
		t.Fatalf("expected 2 filters, got %#v", req.Filters)
	}
	dims := map[string]bool{}
	for _, f := range req.Filters {
		dims[f.Dimension] = true
	}
	if !dims["pais"] || !dims["cliente"] || dims["secret_col"] || dims["mes"] {
		t.Fatalf("unexpected filters %#v", req.Filters)
	}
}

func TestBuildPublicWidgetQuery_RejectsUnknownWidget(t *testing.T) {
	layout := []byte(`{"widgets":[{"id":"w1","type":"bar","query":{"dataset_id":"ds-1","measures":["m"]}}]}`)
	_, _, err := buildPublicWidgetQuery(layout, publicQueryInput{WidgetID: "nope"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildPublicWidgetQuery_DrillUsesHierarchy(t *testing.T) {
	layout := []byte(`{
		"widgets": [{
			"id":"w1","type":"bar",
			"hierarchy":["regiao","cidade"],
			"query":{"dataset_id":"ds-1","measures":["receita"],"dimensions":["regiao"]}
		}]
	}`)
	req, _, err := buildPublicWidgetQuery(layout, publicQueryInput{
		WidgetID:  "w1",
		DrillPath: []string{"Norte"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Dimensions) != 1 || req.Dimensions[0] != "cidade" {
		t.Fatalf("expected cidade dim, got %#v", req.Dimensions)
	}
	found := false
	for _, f := range req.Filters {
		if f.Dimension == "regiao" && f.Value == "Norte" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected hierarchy filter, got %#v", req.Filters)
	}
}

func TestValidateAllowedDatasets_RejectsAutoJoinLeak(t *testing.T) {
	err := queryeng.ValidateAllowedDatasets(queryeng.Request{
		DatasetID: "ds-1",
		Joins:     []queryeng.DatasetJoin{{DatasetID: "ds-secret"}},
		AllowedDatasets: map[string]struct{}{
			"ds-1": {},
		},
	})
	if err == nil {
		t.Fatal("expected unauthorized join to fail")
	}
}
