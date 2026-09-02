package flow

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// DatasetReader is a function that returns the source rows for a dataset. It is injected by the API layer
// so the engine package can stay independent of ClickHouse details.
type DatasetReader func(datasetID string, limit int) ([]string, []map[string]any, error)

// IngestResult is the minimal output of materializing rows.
type IngestResult struct {
	DatasetID uuid.UUID `json:"dataset_id"`
}

// Ingester materializes transformed rows into a new dataset.
type Ingester interface {
	IngestRowsFromMaps(ctx context.Context, orgID, wsID, userID uuid.UUID, name string, headers []string, rows []map[string]any) (IngestResult, error)
}

// LineageRecorder records lineage edges from a flow run to a dataset.
type LineageRecorder interface {
	RecordFlowToDataset(ctx context.Context, orgID, wsID, flowID, datasetID uuid.UUID, flowName string)
}

type noopLineage struct{}

func (noopLineage) RecordFlowToDataset(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, string) {
}

// Engine executes Dobra Flow pipelines in memory. It reads source data via an injected reader,
// applies sequential transform/validate/load steps, and emits rows of maps[string]any.
// For MVP, it does not materialize tables; results are stored in the run log and can be loaded
// into a target dataset by the caller (e.g. the API handler) when storage_mode allows.
type Engine struct {
	store   *Store
	ingest  Ingester
	lineage LineageRecorder
}

func NewEngine(store *Store, ingest Ingester, lineage LineageRecorder) *Engine {
	if lineage == nil {
		lineage = noopLineage{}
	}
	return &Engine{store: store, ingest: ingest, lineage: lineage}
}

func (e *Engine) Execute(ctx context.Context, runID uuid.UUID, userID uuid.UUID, reader DatasetReader) (ResultSummary, error) {
	run, err := e.store.GetRun(ctx, runID)
	if err != nil {
		return ResultSummary{}, err
	}
	flow, err := e.store.Get(ctx, run.OrgID, run.WorkspaceID, run.FlowID)
	if err != nil {
		return ResultSummary{}, err
	}
	steps, err := e.store.ListSteps(ctx, flow.ID)
	if err != nil {
		return ResultSummary{}, err
	}
	_ = e.store.UpdateRun(ctx, runID, "running", "", nil)
	log := func(stepID uuid.UUID, level, msg string) {
		_ = e.store.AddLog(ctx, runID, stepID, level, msg, nil)
	}

	var headers []string
	var rows []map[string]any
	loaded := false

	for _, st := range steps {
		if st.Kind != "extract" {
			continue
		}
		dsID := stringVal(st.Config["dataset_id"])
		if dsID == "" {
			continue
		}
		if loaded {
			log(st.ID, "info", "Extract adicional registado — o join completo de duas fontes chega numa próxima versão; a usar a primeira origem")
			continue
		}
		var err error
		headers, rows, err = reader(dsID, 100000)
		if err != nil {
			log(st.ID, "error", err.Error())
			_ = e.store.UpdateRun(ctx, runID, "failed", err.Error(), nil)
			return ResultSummary{}, err
		}
		loaded = true
		log(st.ID, "info", fmt.Sprintf("Extracted %d rows from dataset %s", len(rows), dsID))
	}

	if !loaded && flow.SourceDatasetID != nil {
		var err error
		headers, rows, err = reader(flow.SourceDatasetID.String(), 100000)
		if err != nil {
			_ = e.store.UpdateRun(ctx, runID, "failed", err.Error(), nil)
			return ResultSummary{}, err
		}
		log(uuid.Nil, "info", fmt.Sprintf("Extracted %d rows from source dataset", len(rows)))
	}

	for _, st := range steps {
		if st.Kind == "extract" {
			continue
		}
		if st.Kind == "transform" {
			out, err := applyTransform(st, headers, rows)
			if err != nil {
				log(st.ID, "error", err.Error())
				_ = e.store.UpdateRun(ctx, runID, "failed", err.Error(), int64Ptr(len(rows)))
				return ResultSummary{}, err
			}
			rows = out
			log(st.ID, "info", fmt.Sprintf("Transform %s applied: %d rows remaining", st.Subkind, len(rows)))
			continue
		}
		if st.Kind == "validate" {
			bad, err := applyValidate(st, headers, rows)
			if err != nil {
				log(st.ID, "error", err.Error())
				_ = e.store.UpdateRun(ctx, runID, "failed", err.Error(), int64Ptr(len(rows)))
				return ResultSummary{}, err
			}
			log(st.ID, "info", fmt.Sprintf("Validation: %d bad rows out of %d", bad, len(rows)))
			continue
		}
		if st.Kind == "load" {
			log(st.ID, "info", fmt.Sprintf("Load step: %d rows ready for materialization", len(rows)))
			continue
		}
	}

	var total int64 = int64(len(rows))
	var outputDSID *uuid.UUID
	if len(rows) > 0 && e.ingest != nil {
		res, err := e.ingest.IngestRowsFromMaps(ctx, run.OrgID, run.WorkspaceID, userID, flow.Name+" · output", headers, rows)
		if err != nil {
			log(uuid.Nil, "error", "Materialization failed: "+err.Error())
		} else {
			id := res.DatasetID
			outputDSID = &id
			_ = e.store.SetOutputDataset(ctx, run.FlowID, *outputDSID)
			e.lineage.RecordFlowToDataset(ctx, run.OrgID, run.WorkspaceID, run.FlowID, *outputDSID, flow.Name)
			log(uuid.Nil, "info", fmt.Sprintf("Materialized output dataset %s with %d rows", outputDSID.String(), total))
		}
	}

	_ = e.store.UpdateRun(ctx, runID, "completed", "", &total)
	return ResultSummary{Rows: total, Columns: headers, Sample: sampleRows(rows, 5), OutputDatasetID: outputDSID}, nil
}

type ResultSummary struct {
	Rows            int64            `json:"rows"`
	Columns         []string         `json:"columns"`
	Sample          []map[string]any `json:"sample,omitempty"`
	OutputDatasetID *uuid.UUID       `json:"output_dataset_id,omitempty"`
}

func (s *Store) GetRun(ctx context.Context, runID uuid.UUID) (RunCtx, error) {
	var r RunCtx
	err := s.pg.QueryRow(ctx, `
		SELECT fr.id, fr.flow_id, f.org_id, f.workspace_id
		FROM flow_runs fr JOIN flows f ON f.id=fr.flow_id WHERE fr.id=$1
	`, runID).Scan(&r.RunID, &r.FlowID, &r.OrgID, &r.WorkspaceID)
	return r, err
}

type RunCtx struct {
	RunID       uuid.UUID
	FlowID      uuid.UUID
	OrgID       uuid.UUID
	WorkspaceID uuid.UUID
}

func applyTransform(st Step, headers []string, rows []map[string]any) ([]map[string]any, error) {
	switch st.Subkind {
	case "rename":
		from := stringVal(st.Config["from"])
		to := stringVal(st.Config["to"])
		if from == "" || to == "" {
			return rows, nil
		}
		for _, r := range rows {
			if _, ok := r[from]; ok {
				r[to] = r[from]
				delete(r, from)
			}
		}
		for i, h := range headers {
			if h == from {
				headers[i] = to
			}
		}
		return rows, nil
	case "change_type":
		col := stringVal(st.Config["column"])
		typ := stringVal(st.Config["type"])
		if col == "" {
			return rows, nil
		}
		for _, r := range rows {
			r[col] = coerce(r[col], typ)
		}
		return rows, nil
	case "filter":
		col := stringVal(st.Config["column"])
		op := stringVal(st.Config["op"])
		val := st.Config["value"]
		if col == "" {
			return rows, nil
		}
		var out []map[string]any
		for _, r := range rows {
			if match(r[col], op, val) {
				out = append(out, r)
			}
		}
		return out, nil
	case "fill_null":
		col := stringVal(st.Config["column"])
		fill := st.Config["value"]
		if col == "" {
			return rows, nil
		}
		for _, r := range rows {
			if r[col] == nil || r[col] == "" {
				r[col] = fill
			}
		}
		return rows, nil
	case "dedup":
		cols := stringSlice(st.Config["columns"])
		seen := map[string]bool{}
		var out []map[string]any
		for _, r := range rows {
			key := rowKey(r, cols)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, r)
		}
		return out, nil
	case "conditional":
		col := stringVal(st.Config["column"])
		ifCol := stringVal(st.Config["if_column"])
		op := stringVal(st.Config["op"])
		val := st.Config["value"]
		elseVal := st.Config["else_value"]
		if col == "" || ifCol == "" {
			return rows, nil
		}
		for _, r := range rows {
			if !match(r[ifCol], op, val) {
				r[col] = elseVal
			}
		}
		return rows, nil
	case "aggregate":
		group := stringSlice(st.Config["group_by"])
		aggCol := stringVal(st.Config["agg_column"])
		aggName := stringVal(st.Config["agg_name"])
		aggFn := stringVal(st.Config["agg_fn"])
		if len(group) == 0 || aggCol == "" || aggName == "" {
			return rows, nil
		}
		groups := map[string]map[string]float64{}
		counts := map[string]int{}
		for _, r := range rows {
			gk := rowKey(r, group)
			v := toFloat(r[aggCol])
			if _, ok := groups[gk]; !ok {
				groups[gk] = map[string]float64{"sum": 0, "min": v, "max": v}
				counts[gk] = 0
			}
			groups[gk]["sum"] += v
			groups[gk]["min"] = min(groups[gk]["min"], v)
			groups[gk]["max"] = max(groups[gk]["max"], v)
			counts[gk]++
		}
		var out []map[string]any
		for gk, vals := range groups {
			parts := strings.Split(gk, "\x1f")
			r := map[string]any{}
			for i, c := range group {
				if i < len(parts) {
					r[c] = parts[i]
				}
			}
			switch strings.ToLower(aggFn) {
			case "avg", "average":
				r[aggName] = vals["sum"] / float64(counts[gk])
			case "min":
				r[aggName] = vals["min"]
			case "max":
				r[aggName] = vals["max"]
			default:
				r[aggName] = vals["sum"]
			}
			out = append(out, r)
		}
		return out, nil
	case "append":
		// Append requires a second dataset source; MVP returns a no-op and logs that external merge is needed.
		return rows, nil
	case "join":
		// Join is a placeholder in MVP; requires two datasets. Return rows unchanged.
		return rows, nil
	default:
		return rows, nil
	}
}

func applyValidate(st Step, headers []string, rows []map[string]any) (int, error) {
	col := stringVal(st.Config["column"])
	typ := stringVal(st.Config["type"])
	if col == "" {
		return 0, nil
	}
	bad := 0
	for _, r := range rows {
		switch typ {
		case "not_null":
			if r[col] == nil || r[col] == "" {
				bad++
			}
		case "numeric":
			if _, ok := r[col].(float64); !ok {
				if f, err := strconv.ParseFloat(stringVal(r[col]), 64); err != nil || f == 0 {
					bad++
				}
			}
		case "unique":
			// uniqueness check across rows requires grouping; report always 0 for MVP single pass
		}
	}
	return bad, nil
}

func stringVal(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func stringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, stringVal(x))
		}
		return out
	}
	return []string{stringVal(v)}
}

func toFloat(v any) float64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case float32:
		return float64(t)
	}
	var f float64
	fmt.Sscanf(stringVal(v), "%f", &f)
	return f
}

func coerce(v any, typ string) any {
	s := stringVal(v)
	if s == "" {
		return nil
	}
	switch strings.ToLower(typ) {
	case "int", "integer":
		if n, err := strconv.ParseInt(strings.ReplaceAll(s, ",", ""), 10, 64); err == nil {
			return n
		}
	case "float", "double":
		if f, err := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(s, ",", ""), " ", ""), 64); err == nil {
			return f
		}
	case "bool", "boolean":
		ls := strings.ToLower(s)
		return ls == "true" || ls == "yes" || ls == "1"
	case "string":
		return s
	}
	return v
}

func match(left any, op, right any) bool {
	lv := stringVal(left)
	rv := stringVal(right)
	ln := toFloat(left)
	rn := toFloat(right)
	switch strings.ToLower(stringVal(op)) {
	case "eq", "=", "==":
		return lv == rv
	case "neq", "!=":
		return lv != rv
	case "gt", ">":
		return ln > rn
	case "gte", ">=":
		return ln >= rn
	case "lt", "<":
		return ln < rn
	case "lte", "<=":
		return ln <= rn
	case "contains":
		return strings.Contains(strings.ToLower(lv), strings.ToLower(rv))
	}
	return false
}

func rowKey(r map[string]any, cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = stringVal(r[c])
	}
	return strings.Join(parts, "\x1f")
}

func sampleRows(rows []map[string]any, n int) []map[string]any {
	if n > len(rows) {
		n = len(rows)
	}
	return rows[:n]
}

func int64Ptr(n int) *int64 {
	v := int64(n)
	return &v
}
