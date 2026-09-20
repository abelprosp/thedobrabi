package apihttp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thedobra/thedobra/services/api/internal/queryeng"
)

// publicQueryInput is the only visitor-controlled surface for public/embed queries.
// Measures, dimensions, joins, and fixed filters come from the saved widget.
type publicQueryInput struct {
	WidgetID      string               `json:"widget_id"`
	Filters       []queryeng.Filter    `json:"filters,omitempty"`
	GlobalFilters []queryeng.Filter    `json:"global_filters,omitempty"`
	DrillPath     []string             `json:"drill_path,omitempty"`
	TimeRange     *queryeng.TimeRange  `json:"time_range,omitempty"`
	Limit         int                  `json:"limit,omitempty"`
}

type layoutWidget struct {
	ID        string           `json:"id"`
	Type      string           `json:"type"`
	Query     *queryeng.Request `json:"query"`
	Hierarchy []string         `json:"hierarchy"`
	Config    map[string]any   `json:"config"`
}

func parseLayoutWidgets(layout []byte) []layoutWidget {
	var parsed struct {
		Widgets []layoutWidget `json:"widgets"`
	}
	if json.Unmarshal(layout, &parsed) != nil {
		return nil
	}
	for i := range parsed.Widgets {
		if parsed.Widgets[i].ID == "" {
			parsed.Widgets[i].ID = fmt.Sprintf("w-%d", i)
		}
	}
	return parsed.Widgets
}

func findLayoutWidget(layout []byte, widgetID string) (layoutWidget, bool) {
	widgetID = strings.TrimSpace(widgetID)
	if widgetID == "" {
		return layoutWidget{}, false
	}
	for _, w := range parseLayoutWidgets(layout) {
		if w.ID == widgetID {
			return w, true
		}
	}
	return layoutWidget{}, false
}

// publishedFilterDims are interactive dimensions exposed by slicers on the same dataset,
// plus hierarchy levels of the target widget (for drill-down).
func publishedFilterDims(layout []byte, target layoutWidget) map[string]struct{} {
	out := map[string]struct{}{}
	ds := ""
	if target.Query != nil {
		ds = target.Query.DatasetID
	}
	for _, w := range parseLayoutWidgets(layout) {
		if w.Type != "slicer" || w.Query == nil {
			continue
		}
		if ds != "" && w.Query.DatasetID != "" && w.Query.DatasetID != ds {
			continue
		}
		for _, d := range w.Query.Dimensions {
			if d != "" {
				out[d] = struct{}{}
			}
		}
	}
	for _, d := range target.Hierarchy {
		if d != "" {
			out[d] = struct{}{}
		}
	}
	return out
}

func filterAllowed(f queryeng.Filter, allowed map[string]struct{}) bool {
	if f.Dimension == "" {
		return false
	}
	if _, ok := allowed[f.Dimension]; !ok {
		return false
	}
	op := strings.ToLower(strings.TrimSpace(f.Op))
	switch op {
	case "eq", "in", "":
		return true
	default:
		return false
	}
}

// buildPublicWidgetQuery constructs a queryeng.Request from the saved widget,
// allowing only published interactive filters / drill path from the visitor.
func buildPublicWidgetQuery(layout []byte, in publicQueryInput) (queryeng.Request, map[string]struct{}, error) {
	w, ok := findLayoutWidget(layout, in.WidgetID)
	if !ok {
		return queryeng.Request{}, nil, fmt.Errorf("widget não encontrado nesta partilha")
	}
	if w.Query == nil || w.Query.DatasetID == "" {
		return queryeng.Request{}, nil, fmt.Errorf("widget sem consulta configurada")
	}

	req := *w.Query
	// Explicit empty joins: never auto-inject relationships for public queries.
	if req.Joins == nil {
		req.Joins = []queryeng.DatasetJoin{}
	}
	req.DisableAutoJoins = true
	req.Compare = nil
	req.GlobalFilters = nil
	req.DrillPath = nil

	allowed := allowedDatasetIDs(layout)
	req.AllowedDatasets = allowed

	published := publishedFilterDims(layout, w)
	fixed := append([]queryeng.Filter{}, req.Filters...)
	var interactive []queryeng.Filter
	for _, f := range append(append([]queryeng.Filter{}, in.Filters...), in.GlobalFilters...) {
		if !filterAllowed(f, published) {
			continue
		}
		if w.Type == "slicer" && len(w.Query.Dimensions) > 0 && f.Dimension == w.Query.Dimensions[0] {
			continue
		}
		if f.Op == "" {
			f.Op = "eq"
		}
		interactive = append(interactive, f)
	}

	// Drill-down: only along the widget's saved hierarchy; dimensions come from hierarchy.
	if len(in.DrillPath) > 0 && len(w.Hierarchy) > 0 {
		path := in.DrillPath
		if len(path) >= len(w.Hierarchy) {
			path = path[:len(w.Hierarchy)-1]
		}
		level := len(path)
		if level < len(w.Hierarchy) {
			req.Dimensions = []string{w.Hierarchy[level]}
		}
		for i, v := range path {
			fixed = append(fixed, queryeng.Filter{Dimension: w.Hierarchy[i], Op: "eq", Value: v})
		}
	}

	req.Filters = append(fixed, interactive...)
	if in.TimeRange != nil {
		req.TimeRange = in.TimeRange
	}
	if in.Limit > 0 {
		req.Limit = in.Limit
	}

	applyPublicWidgetShape(&req, w)

	if _, ok := allowed[req.DatasetID]; !ok {
		return queryeng.Request{}, nil, fmt.Errorf("conjunto não faz parte desta partilha")
	}
	for _, j := range req.Joins {
		if j.DatasetID == "" {
			continue
		}
		if _, ok := allowed[j.DatasetID]; !ok {
			return queryeng.Request{}, nil, fmt.Errorf("conjunto não faz parte desta partilha")
		}
	}
	return req, allowed, nil
}

func applyPublicWidgetShape(req *queryeng.Request, w layoutWidget) {
	switch w.Type {
	case "kpi", "kpi_goal", "gauge", "metric_group":
		req.Dimensions = nil
	case "ranking":
		if len(req.Measures) > 0 {
			dir := "DESC"
			if cfgStr(w.Config, "rankOrder") == "asc" {
				dir = "ASC"
			}
			req.OrderBy = []queryeng.Order{{Field: req.Measures[0], Dir: dir}}
		}
		n := 10
		if v, ok := w.Config["rankLimit"].(float64); ok && v > 0 {
			n = int(v)
		}
		if n < 3 {
			n = 3
		}
		if n > 50 {
			n = 50
		}
		req.Limit = n
	case "heatmap":
		if len(req.Dimensions) > 2 {
			req.Dimensions = req.Dimensions[:2]
		}
	case "slicer", "scatter":
		// keep saved shape
	default:
		if w.Type != "" && w.Type != "table" && w.Type != "big_table" && w.Type != "decomposition_tree" {
			if len(req.Dimensions) > 1 {
				req.Dimensions = req.Dimensions[:1]
			}
		}
	}
	if len(req.Joins) == 0 {
		req.Measures = filterJoinRefs(req.Measures)
		req.Dimensions = filterJoinRefs(req.Dimensions)
	}
}

func cfgStr(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	s, _ := cfg[key].(string)
	return s
}

func filterJoinRefs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if strings.HasPrefix(id, "join.") {
			continue
		}
		out = append(out, id)
	}
	return out
}
