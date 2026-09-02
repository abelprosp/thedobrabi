package ingest

import (
	"encoding/json"
	"testing"
)

func TestStripWidgetsForDataset(t *testing.T) {
	keepID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	dropID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	layout, _ := json.Marshal(map[string]any{
		"widgets": []any{
			map[string]any{"id": "1", "type": "kpi", "query": map[string]any{"dataset_id": dropID, "measures": []any{"valor"}}},
			map[string]any{"id": "2", "type": "bar", "query": map[string]any{"dataset_id": keepID, "measures": []any{"valor"}}},
			map[string]any{"id": "3", "type": "text", "text": "ok"},
		},
	})
	next, changed := stripDatasetFromLayoutJSON(layout, dropID)
	if !changed {
		t.Fatal("expected layout change")
	}
	var parsed struct {
		Widgets []map[string]any `json:"widgets"`
	}
	if err := json.Unmarshal(next, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Widgets) != 2 {
		t.Fatalf("expected 2 widgets, got %d", len(parsed.Widgets))
	}
	if parsed.Widgets[0]["id"] != "2" {
		t.Fatalf("expected remaining bar widget first, got %#v", parsed.Widgets[0]["id"])
	}
}

func TestStripDatasetFromPagesJSON(t *testing.T) {
	dropID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	pages, _ := json.Marshal([]any{
		map[string]any{
			"name": "Página 1",
			"widgets": []any{
				map[string]any{"id": "1", "query": map[string]any{"dataset_id": dropID}},
				map[string]any{"id": "2", "query": map[string]any{"dataset_id": "other"}},
			},
		},
	})
	next, changed := stripDatasetFromPagesJSON(pages, dropID)
	if !changed {
		t.Fatal("expected pages change")
	}
	var parsed []struct {
		Widgets []map[string]any `json:"widgets"`
	}
	if err := json.Unmarshal(next, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 || len(parsed[0].Widgets) != 1 {
		t.Fatalf("expected 1 remaining widget, got %#v", parsed)
	}
}

func TestUniqueSanitizedSlugs(t *testing.T) {
	got := uniqueSanitizedSlugs("redorai_financeiro-abcd1234", "redorai-financeiro")
	foundUnsuffixed := false
	for _, s := range got {
		if s == "redorai_financeiro" {
			foundUnsuffixed = true
		}
	}
	if !foundUnsuffixed {
		t.Fatalf("expected unsuffixed slug, got %v", got)
	}
}
