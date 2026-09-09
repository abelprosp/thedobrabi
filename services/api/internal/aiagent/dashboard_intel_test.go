package aiagent

import "testing"

func TestAnalyzeDashboardFallbackEmpty(t *testing.T) {
	out := analyzeDashboardFallback(nil, "")
	if out.Source != "fallback" {
		t.Fatalf("source: %s", out.Source)
	}
	if out.Headline == "" {
		t.Fatal("expected headline")
	}
	if len(out.Insights) == 0 {
		t.Fatal("expected empty-state insight")
	}
}

func TestAnalyzeDashboardFallbackDrop(t *testing.T) {
	snaps := []widgetSnapshot{{
		ID:        "w1",
		Title:     "Receita",
		Type:      "line",
		DatasetID: "ds-1",
		Measures:  []string{"receita"},
		Columns:   []string{"mes", "receita"},
		Rows: []map[string]any{
			{"mes": "jan", "receita": 100.0},
			{"mes": "fev", "receita": 70.0},
		},
		Stats: computeSeriesStats([]string{"mes", "receita"}, []map[string]any{
			{"mes": "jan", "receita": 100.0},
			{"mes": "fev", "receita": 70.0},
		}, []string{"receita"}),
	}}
	out := analyzeDashboardFallback(snaps, "quedas de receita")
	if out.AnalyzedWidgets != 1 {
		t.Fatalf("widgets: %d", out.AnalyzedWidgets)
	}
	foundRisk := false
	for _, in := range out.Insights {
		if in.Kind == "risk" && in.WidgetID == "w1" {
			foundRisk = true
		}
	}
	if !foundRisk {
		t.Fatalf("expected risk insight, got %#v", out.Insights)
	}
	if len(out.AlertSuggestions) == 0 {
		t.Fatal("expected alert suggestion")
	}
	al := out.AlertSuggestions[0]
	if al.Condition.DatasetID != "ds-1" || al.Condition.Measure != "receita" || al.Condition.Op != "<" {
		t.Fatalf("condition: %#v", al.Condition)
	}
	if al.Condition.Value != 70 {
		t.Fatalf("threshold: %v", al.Condition.Value)
	}
}

func TestComputeSeriesStatsChangePct(t *testing.T) {
	st := computeSeriesStats([]string{"d", "v"}, []map[string]any{
		{"d": "a", "v": 50.0},
		{"d": "b", "v": 40.0},
	}, []string{"v"})
	if st == nil {
		t.Fatal("nil stats")
	}
	if st.First != 50 || st.Last != 40 {
		t.Fatalf("first/last: %v %v", st.First, st.Last)
	}
	if st.ChangePct > -19 || st.ChangePct < -21 {
		t.Fatalf("change: %v", st.ChangePct)
	}
	if st.TopCategory != "a" {
		t.Fatalf("top: %s", st.TopCategory)
	}
}

func TestAsFloat(t *testing.T) {
	if v, ok := asFloat(12.5); !ok || v != 12.5 {
		t.Fatalf("float %v %v", v, ok)
	}
	if v, ok := asFloat("1.5"); !ok || v != 1.5 {
		t.Fatalf("string %v %v", v, ok)
	}
	if _, ok := asFloat("abc"); ok {
		t.Fatal("expected fail")
	}
}

func TestNormalizeAlertOp(t *testing.T) {
	if normalizeAlertOp(">") != ">" {
		t.Fatal(">")
	}
	if normalizeAlertOp("eq") != "<" {
		t.Fatal("default")
	}
}
