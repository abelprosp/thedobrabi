// Package queryeng provides a lightweight in-memory query executor used when the planner
// chooses the DuckDB path for small datasets or Parquet lake objects. It is not a full
// DuckDB engine; it executes a subset of SQL (SELECT, WHERE, GROUP BY, ORDER BY, LIMIT)
// over rows loaded from ClickHouse or the lake.
package queryeng

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DuckDBResult is the result of an in-memory query execution.
type DuckDBResult struct {
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
}

// DuckDBExecutor runs simple queries over rows loaded from a source.
type DuckDBExecutor struct {
	loader func(ctx context.Context, limit int) ([]string, []map[string]any, error)
}

func NewDuckDBExecutor(loader func(ctx context.Context, limit int) ([]string, []map[string]any, error)) *DuckDBExecutor {
	return &DuckDBExecutor{loader: loader}
}

// Execute runs a simplified SQL query over the loaded rows. It supports:
// SELECT col1, col2, AGG(col3) ... FROM rows WHERE ... GROUP BY ... ORDER BY ... LIMIT ...
// Aggregations: SUM, AVG, COUNT, MIN, MAX, COUNT(DISTINCT approximated as COUNT after DISTINCT).
func (e *DuckDBExecutor) Execute(ctx context.Context, sql string, limit int) (DuckDBResult, error) {
	cols, rows, err := e.loader(ctx, 0)
	if err != nil {
		return DuckDBResult{}, err
	}
	q, err := parseSimpleSQL(sql)
	if err != nil {
		return DuckDBResult{}, err
	}
	filtered := filterRows(rows, q.where)
	if len(q.aggs) > 0 {
		agg, err := aggregateRows(filtered, cols, q)
		if err != nil {
			return DuckDBResult{}, err
		}
		filtered = agg
	}
	if q.orderby != "" {
		sortRows(filtered, q.orderby, q.orderdir)
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return DuckDBResult{Columns: q.selects, Rows: filtered}, nil
}

type simpleQuery struct {
	selects  []string
	aggs     map[string]aggSpec
	where    []simplePredicate
	groupby  []string
	orderby  string
	orderdir string
	limit    int
}

type aggSpec struct {
	fn   string
	col  string
	name string
}

type simplePredicate struct {
	col string
	op  string
	val any
}

var selectRe = regexp.MustCompile(`(?i)^\s*SELECT\s+(.*?)\s+FROM\s+`)
var whereRe = regexp.MustCompile(`(?i)\s+WHERE\s+(.*?)\s*(?:GROUP BY|ORDER BY|LIMIT|$)`)
var groupRe = regexp.MustCompile(`(?i)\s+GROUP BY\s+(.*?)\s*(?:ORDER BY|LIMIT|$)`)
var orderRe = regexp.MustCompile(`(?i)\s+ORDER BY\s+(.*?)\s*(?:LIMIT|$)`)
var limitRe = regexp.MustCompile(`(?i)\s+LIMIT\s+(\d+)`)

func parseSimpleSQL(sql string) (simpleQuery, error) {
	q := simpleQuery{aggs: map[string]aggSpec{}, orderdir: "ASC"}
	m := selectRe.FindStringSubmatch(sql)
	if m == nil {
		return q, fmt.Errorf("only SELECT ... FROM queries are supported")
	}
	parts := splitComma(m[1])
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if agg := parseAgg(p); agg.fn != "" {
			q.aggs[agg.name] = agg
			q.selects = append(q.selects, agg.name)
		} else {
			name := strings.Trim(p, "`")
			q.selects = append(q.selects, name)
		}
	}
	if m := whereRe.FindStringSubmatch(sql); m != nil {
		q.where = parsePredicates(m[1])
	}
	if m := groupRe.FindStringSubmatch(sql); m != nil {
		q.groupby = splitComma(m[1])
		for i := range q.groupby {
			q.groupby[i] = strings.Trim(q.groupby[i], "`")
		}
	}
	if m := orderRe.FindStringSubmatch(sql); m != nil {
		ob := strings.TrimSpace(m[1])
		if strings.HasSuffix(strings.ToUpper(ob), " DESC") {
			q.orderdir = "DESC"
			ob = strings.TrimSuffix(ob, " DESC")
			ob = strings.TrimSuffix(ob, " desc")
		}
		q.orderby = strings.Trim(ob, "`")
	}
	if m := limitRe.FindStringSubmatch(sql); m != nil {
		q.limit, _ = strconv.Atoi(m[1])
	}
	return q, nil
}

func parseAgg(s string) aggSpec {
	re := regexp.MustCompile(`(?i)^([A-Z][A-Z0-9_]*)\s*\(\s*(?:DISTINCT\s+)?(?:\x60)?([*a-zA-Z_][a-zA-Z0-9_]*)(?:\x60)?\s*\)\s*(?:AS\s+)?(?:\x60)?([a-zA-Z_][a-zA-Z0-9_ ]*)(?:\x60)?$`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return aggSpec{}
	}
	fn := strings.ToUpper(m[1])
	col := m[2]
	name := strings.TrimSpace(strings.Trim(m[3], "\x60"))
	if name == "" {
		name = fn + "_" + col
	}
	name = strings.ReplaceAll(name, " ", "_")
	return aggSpec{fn: fn, col: col, name: name}
}

func parsePredicates(s string) []simplePredicate {
	var preds []simplePredicate
	for _, part := range splitTopLevel(s, "AND") {
		part = strings.TrimSpace(part)
		re := regexp.MustCompile(`^\s*([a-zA-Z_][a-zA-Z0-9_]*|\x60[a-zA-Z_][a-zA-Z0-9_]*\x60)\s*(=|!=|<>|>=|<=|>|<|in|not in)\s*(.+?)\s*$`)
		m := re.FindStringSubmatch(part)
		if m == nil {
			continue
		}
		col := strings.Trim(m[1], "\x60")
		preds = append(preds, simplePredicate{col: col, op: m[2], val: parseValue(m[3])})
	}
	return preds
}

func parseValue(s string) any {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'") {
		return strings.Trim(s, "'")
	}
	if s == "NULL" || s == "null" {
		return nil
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return n
	}
	return s
}

func filterRows(rows []map[string]any, preds []simplePredicate) []map[string]any {
	if len(preds) == 0 {
		return rows
	}
	var out []map[string]any
	for _, r := range rows {
		ok := true
		for _, p := range preds {
			if !evalPred(r[p.col], p.op, p.val) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, r)
		}
	}
	return out
}

func evalPred(left any, op string, right any) bool {
	ln := toF(left)
	rn := toF(right)
	ls := fmt.Sprint(left)
	rs := fmt.Sprint(right)
	switch op {
	case "=":
		return ls == rs
	case "!=", "<>":
		return ls != rs
	case ">":
		return ln > rn
	case ">=":
		return ln >= rn
	case "<":
		return ln < rn
	case "<=":
		return ln <= rn
	}
	return false
}

func aggregateRows(rows []map[string]any, cols []string, q simpleQuery) ([]map[string]any, error) {
	_ = cols
	groups := map[string]map[string]any{}
	counts := map[string]int{}
	for _, r := range rows {
		key := groupKey(r, q.groupby)
		if _, ok := groups[key]; !ok {
			newRow := map[string]any{}
			for _, g := range q.groupby {
				newRow[g] = rowLookup(r, g)
			}
			for name, spec := range q.aggs {
				newRow[name] = aggInit(spec.fn)
			}
			groups[key] = newRow
			counts[key] = 0
		}
		counts[key]++
		for name, spec := range q.aggs {
			v := toF(rowLookup(r, spec.col))
			groups[key][name] = aggCombine(spec.fn, groups[key][name], v)
		}
	}
	var out []map[string]any
	for key, g := range groups {
		n := float64(counts[key])
		if n == 0 {
			n = 1
		}
		for name, spec := range q.aggs {
			if spec.fn == "AVG" || spec.fn == "AVERAGE" {
				g[name] = toF(g[name]) / n
			}
			if spec.fn == "COUNT" && spec.col == "*" {
				g[name] = counts[key]
			}
		}
		out = append(out, g)
	}
	if len(out) == 0 && len(q.aggs) > 0 {
		empty := map[string]any{}
		for name, spec := range q.aggs {
			if spec.fn == "COUNT" {
				empty[name] = 0
			} else {
				empty[name] = 0.0
			}
		}
		out = append(out, empty)
	}
	return out, nil
}

func groupKey(r map[string]any, groupby []string) string {
	if len(groupby) == 0 {
		return ""
	}
	parts := make([]string, len(groupby))
	for i, g := range groupby {
		parts[i] = fmt.Sprint(rowLookup(r, g))
	}
	return strings.Join(parts, "\x1f")
}

func rowLookup(r map[string]any, col string) any {
	if v, ok := r[col]; ok {
		return v
	}
	want := strings.ToLower(strings.Trim(col, "`"))
	for k, v := range r {
		if strings.ToLower(k) == want {
			return v
		}
	}
	return nil
}

func aggInit(fn string) any {
	switch strings.ToUpper(fn) {
	case "MIN":
		return math.MaxFloat64
	case "MAX":
		return -math.MaxFloat64
	case "COUNT":
		return 0
	default:
		return 0.0
	}
}

func aggCombine(fn string, acc any, v float64) any {
	switch strings.ToUpper(fn) {
	case "SUM", "AVG", "AVERAGE":
		return toF(acc) + v
	case "MIN":
		return math.Min(toF(acc), v)
	case "MAX":
		return math.Max(toF(acc), v)
	case "COUNT":
		return toF(acc) + 1
	default:
		return toF(acc) + v
	}
}

func sortRows(rows []map[string]any, col, dir string) {
	less := func(i, j int) bool {
		a, b := rows[i][col], rows[j][col]
		if toF(a) != 0 || toF(b) != 0 {
			if dir == "DESC" {
				return toF(a) > toF(b)
			}
			return toF(a) < toF(b)
		}
		if dir == "DESC" {
			return fmt.Sprint(a) > fmt.Sprint(b)
		}
		return fmt.Sprint(a) < fmt.Sprint(b)
	}
	sort.Slice(rows, less)
}

func splitComma(s string) []string {
	return splitTopLevel(s, ",")
}

func splitTopLevel(s, sep string) []string {
	var out []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth == 0 && strings.HasPrefix(s[i:], sep) {
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + len(sep)
		}
	}
	out = append(out, strings.TrimSpace(s[start:]))
	return out
}

func toF(v any) float64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int64:
		return float64(t)
	case int:
		return float64(t)
	}
	var f float64
	fmt.Sscanf(fmt.Sprint(v), "%f", &f)
	return f
}
