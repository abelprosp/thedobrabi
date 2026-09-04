package queryeng

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/thedobra/thedobra/services/api/internal/config"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

type Engine struct {
	pg      *pgxpool.Pool
	ch      driver.Conn
	rdb     *redis.Client
	cfg     config.Config
	planner *Planner
}

func New(pg *pgxpool.Pool, ch driver.Conn, rdb *redis.Client, cfg config.Config) *Engine {
	return &Engine{pg: pg, ch: ch, rdb: rdb, cfg: cfg, planner: NewPlanner(pg, rdb)}
}

type Request struct {
	DatasetID     string        `json:"dataset_id"`
	Measures      []string      `json:"measures"`
	Dimensions    []string      `json:"dimensions"`
	Filters       []Filter      `json:"filters,omitempty"`
	TimeRange     *TimeRange    `json:"time_range,omitempty"`
	OrderBy       []Order       `json:"order_by,omitempty"`
	Limit         int           `json:"limit,omitempty"`
	Compare       *TimeRange    `json:"compare,omitempty"`
	GlobalFilters []Filter      `json:"global_filters,omitempty"`
	DrillPath     []string      `json:"drill_path,omitempty"`
	Joins         []DatasetJoin `json:"joins,omitempty"`
}

type DatasetJoin struct {
	DatasetID  string `json:"dataset_id"`
	FromColumn string `json:"from_column"`
	ToColumn   string `json:"to_column"`
	Match      string `json:"match,omitempty"` // both | all_left
}

type Filter struct {
	Dimension string `json:"dimension"`
	Op        string `json:"op"` // eq, neq, in, gt, gte, lt, lte, contains
	Value     any    `json:"value"`
}

type TimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type Order struct {
	Field string `json:"field"`
	Dir   string `json:"dir"`
}

type Result struct {
	Columns     []string         `json:"columns"`
	Rows        []map[string]any `json:"rows"`
	SQL         string           `json:"sql"`
	CacheHit    bool             `json:"cache_hit"`
	DurationMs  int64            `json:"duration_ms"`
	RowCount    int              `json:"row_count"`
	BytesRead   int64            `json:"bytes_read,omitempty"`
	Fingerprint string           `json:"fingerprint"`
	Planner     string           `json:"planner,omitempty"`
	Evidence    Evidence         `json:"evidence"`
}

type Evidence struct {
	Dataset    string   `json:"dataset"`
	Table      string   `json:"table"`
	TimeColumn string   `json:"time_column,omitempty"`
	Period     string   `json:"period,omitempty"`
	Metrics    []string `json:"metrics"`
}

func (e *Engine) Execute(ctx context.Context, orgID, wsID, userID uuid.UUID, role string, req Request) (Result, error) {
	if req.Limit <= 0 || req.Limit > e.cfg.QueryRowLimit {
		req.Limit = 900
	}
	meta, err := e.planner.loadDataset(ctx, orgID, wsID, req.DatasetID)
	if err != nil {
		return Result{}, err
	}
	plan := e.planner.choosePlan(meta, req)
	fp := fingerprint(orgID, wsID, userID, req)
	if cached, ok := e.getCache(ctx, fp, plan.CacheTTL); ok {
		cached.CacheHit = true
		cached.Fingerprint = fp
		return cached, nil
	}

	start := time.Now()
	var out Result
	sql, evidence, err := e.buildSQL(ctx, meta, plan, req, orgID, userID, role)
	if err != nil {
		return Result{}, err
	}
	qctx, cancel := context.WithTimeout(ctx, e.cfg.QueryTimeout)
	defer cancel()
	rows, err := e.ch.Query(qctx, sql)
	if err != nil {
		return Result{}, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()
	cols, data, err := collectRows(rows, req.Limit)
	if err != nil {
		if qctx.Err() != nil {
			return Result{}, fmt.Errorf("query timeout")
		}
		return Result{}, fmt.Errorf("query failed: %w", err)
	}
	out = Result{Columns: cols, Rows: data, SQL: sql, Evidence: evidence, Fingerprint: fp, RowCount: len(data), Planner: plan.SourceType}
	out.BytesRead = estimateBytes(cols, data)
	out.DurationMs = time.Since(start).Milliseconds()
	out.Fingerprint = fp
	e.setCache(ctx, fp, out, plan.CacheTTL)
	e.record(ctx, orgID, wsID, userID, fp, req, out.SQL, out, false, plan.SourceType)
	return out, nil
}

func (e *Engine) executeDuckDB(ctx context.Context, meta datasetInfo, plan plan, req Request, orgID, userID uuid.UUID, role string) (Result, error) {
	sql, ev := requestToSimpleSQL(meta, req)
	loader := func(ctx context.Context, limit int) ([]string, []map[string]any, error) {
		return e.ReadRows(ctx, orgID, meta.WorkspaceID, meta.ID.String(), limit)
	}
	exec := NewDuckDBExecutor(loader)
	res, err := exec.Execute(ctx, sql, req.Limit)
	if err != nil {
		return Result{}, err
	}
	// Apply role-based RLS predicates to filter rows in memory.
	if role != "" {
		preds, err := e.planner.rlsPredicates(ctx, meta, userID, role)
		if err == nil && len(preds) > 0 {
			predsSQL := strings.Join(preds, " AND ")
			res.Rows = filterRows(res.Rows, parsePredicates(predsSQL))
		}
	}
	return Result{Columns: res.Columns, Rows: res.Rows, SQL: sql, Evidence: ev, RowCount: len(res.Rows), BytesRead: estimateBytes(res.Columns, res.Rows), Planner: plan.SourceType}, nil
}

func requestToSimpleSQL(meta datasetInfo, req Request) (string, Evidence) {
	ev := Evidence{Dataset: meta.Name, Table: meta.Table, TimeColumn: meta.Model.TimeColumn}
	var parts []string
	var groups []string
	for _, d := range req.Dimensions {
		parts = append(parts, fmt.Sprintf("`%s`", d))
		groups = append(groups, fmt.Sprintf("`%s`", d))
	}
	for _, m := range req.Measures {
		sm, ok := semantic.ResolveMeasure(meta.Model, m)
		if !ok {
			continue
		}
		expr, _ := measureSQL(sm, &meta.Model, "", "")
		parts = append(parts, fmt.Sprintf("%s AS `%s`", expr, sqlOutAlias(m, sm.Name)))
		ev.Metrics = append(ev.Metrics, m+" = "+sm.Expression)
	}
	if len(parts) == 0 {
		parts = append(parts, "*")
	}
	var clauses []string
	if req.TimeRange != nil && meta.Model.TimeColumn != "" {
		clauses = append(clauses, timeFilterSimple(meta.Model.TimeColumn, req.TimeRange.Start, req.TimeRange.End)...)
		ev.Period = req.TimeRange.Start + " → " + req.TimeRange.End
	}
	for _, f := range req.Filters {
		if clause, err := filterClauseSimple(meta.Model, f); err == nil {
			clauses = append(clauses, clause)
		}
	}
	for _, f := range req.GlobalFilters {
		if clause, err := filterClauseSimple(meta.Model, f); err == nil {
			clauses = append(clauses, clause)
		}
	}
	sql := fmt.Sprintf("SELECT %s FROM rows", strings.Join(parts, ", "))
	if len(clauses) > 0 {
		sql += " WHERE " + strings.Join(clauses, " AND ")
	}
	if len(groups) > 0 {
		sql += " GROUP BY " + strings.Join(groups, ", ")
	}
	if len(req.OrderBy) > 0 {
		dir := "ASC"
		if strings.ToUpper(req.OrderBy[0].Dir) == "DESC" {
			dir = "DESC"
		}
		sql += fmt.Sprintf(" ORDER BY `%s` %s", req.OrderBy[0].Field, dir)
	}
	if req.Limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", req.Limit)
	}
	return sql, ev
}

func filterClauseSimple(model semantic.Model, f Filter) (string, error) {
	col := f.Dimension
	if !identOK(col) {
		return "", fmt.Errorf("invalid column")
	}
	switch f.Op {
	case "eq":
		return fmt.Sprintf("`%s` = %s", col, literal(f.Value)), nil
	case "neq":
		return fmt.Sprintf("`%s` != %s", col, literal(f.Value)), nil
	case "gt":
		return fmt.Sprintf("`%s` > %s", col, literal(f.Value)), nil
	case "gte":
		return fmt.Sprintf("`%s` >= %s", col, literal(f.Value)), nil
	case "lt":
		return fmt.Sprintf("`%s` < %s", col, literal(f.Value)), nil
	case "lte":
		return fmt.Sprintf("`%s` <= %s", col, literal(f.Value)), nil
	case "in":
		vs := toSlice(f.Value)
		parts := make([]string, len(vs))
		for i, v := range vs {
			parts[i] = literal(v)
		}
		return fmt.Sprintf("`%s` IN (%s)", col, strings.Join(parts, ", ")), nil
	default:
		return "", fmt.Errorf("unsupported filter op")
	}
}

func (e *Engine) ReadRows(ctx context.Context, orgID, wsID uuid.UUID, datasetID string, limit int) ([]string, []map[string]any, error) {
	if limit <= 0 || limit > 200000 {
		limit = 100000
	}
	meta, err := e.planner.loadDataset(ctx, orgID, wsID, datasetID)
	if err != nil {
		return nil, nil, err
	}
	plan := e.planner.choosePlan(meta, Request{})
	target := plan.TargetTable
	if target == "" {
		target = meta.Table
	}
	sql := fmt.Sprintf("SELECT * EXCEPT(_tenant) FROM %s.`%s` WHERE _tenant = '%s' LIMIT %d",
		e.cfg.ClickHouseDB, target, orgID.String(), limit)
	qctx, cancel := context.WithTimeout(ctx, e.cfg.QueryTimeout)
	defer cancel()
	rows, err := e.ch.Query(qctx, sql)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	cols, data, err := collectRows(rows, limit)
	if err != nil {
		return nil, nil, err
	}
	return cols, data, nil
}

func (e *Engine) Preview(ctx context.Context, orgID, wsID uuid.UUID, datasetID string, limit int) (Result, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	meta, err := e.planner.loadDataset(ctx, orgID, wsID, datasetID)
	if err != nil {
		return Result{}, err
	}
	plan := e.planner.choosePlan(meta, Request{})
	target := plan.TargetTable
	if target == "" {
		target = meta.Table
	}
	sql := fmt.Sprintf("SELECT * EXCEPT(_tenant) FROM %s.`%s` WHERE _tenant = '%s' LIMIT %d",
		e.cfg.ClickHouseDB, target, orgID.String(), limit)
	req := Request{DatasetID: datasetID, Limit: limit}
	return e.runRaw(ctx, orgID, wsID, uuid.Nil, sql, req, "preview")
}

func (e *Engine) runRaw(ctx context.Context, orgID, wsID, userID uuid.UUID, sql string, req Request, planner string) (Result, error) {
	qctx, cancel := context.WithTimeout(ctx, e.cfg.QueryTimeout)
	defer cancel()
	start := time.Now()
	rows, err := e.ch.Query(qctx, sql)
	if err != nil {
		return Result{}, err
	}
	defer rows.Close()
	cols, data, err := collectRows(rows, req.Limit)
	if err != nil {
		return Result{}, err
	}
	out := Result{Columns: cols, Rows: data, SQL: sql, RowCount: len(data), DurationMs: time.Since(start).Milliseconds(), Planner: planner}
	_ = userID
	_ = wsID
	return out, nil
}

func (e *Engine) buildSQL(ctx context.Context, meta datasetInfo, plan plan, req Request, orgID, userID uuid.UUID, role string) (string, Evidence, error) {
	if len(req.Measures) == 0 && len(req.Dimensions) == 0 {
		return "", Evidence{}, fmt.Errorf("escolha pelo menos uma medida ou dimensão")
	}
	model := meta.Model
	selects := make([]string, 0)
	groups := make([]string, 0)
	ev := Evidence{Dataset: meta.Name, Table: meta.Table, TimeColumn: model.TimeColumn}
	target := plan.TargetTable
	if target == "" {
		target = meta.Table
	}

	var joinMeta *datasetInfo
	var join DatasetJoin
	if len(req.Joins) > 0 {
		join = req.Joins[0]
		if join.DatasetID == "" || join.FromColumn == "" || join.ToColumn == "" {
			return "", ev, fmt.Errorf("cruzamento incompleto: indique o conjunto e as colunas que se ligam")
		}
		if !identOK(join.FromColumn) || !identOK(join.ToColumn) {
			return "", ev, fmt.Errorf("coluna de cruzamento inválida")
		}
		loaded, err := e.planner.loadDataset(ctx, orgID, meta.WorkspaceID, join.DatasetID)
		if err != nil {
			return "", ev, fmt.Errorf("não encontrei o conjunto do cruzamento")
		}
		if !columnExists(model, join.FromColumn) {
			return "", ev, fmt.Errorf("a coluna %q não existe neste conjunto", join.FromColumn)
		}
		if !columnExists(loaded.Model, join.ToColumn) {
			return "", ev, fmt.Errorf("a coluna %q não existe no conjunto cruzado", join.ToColumn)
		}
		joinMeta = &loaded
		ev.Dataset = meta.Name + " + " + loaded.Name
	}

	qualify := func(joinField bool, col string) string {
		if joinMeta == nil {
			return "`" + col + "`"
		}
		if joinField {
			return "b.`" + col + "`"
		}
		return "a.`" + col + "`"
	}

	for _, dname := range req.Dimensions {
		joinField := strings.HasPrefix(dname, "join.")
		raw := strings.TrimPrefix(dname, "join.")
		src := model
		if joinField {
			if joinMeta == nil {
				return "", ev, fmt.Errorf("dimensão %q pede cruzamento, mas nenhum conjunto extra foi ligado", dname)
			}
			src = joinMeta.Model
		}
		d, ok := semantic.ResolveDimension(src, raw)
		if !ok {
			if identOK(raw) && columnExists(src, raw) {
				d = semantic.Dimension{Name: raw, Column: raw}
			} else {
				return "", ev, fmt.Errorf("dimensão desconhecida %q", dname)
			}
		}
		if !identOK(d.Column) {
			return "", ev, fmt.Errorf("coluna de dimensão inválida")
		}
		colSQL := qualify(joinField, d.Column)
		expr := dimensionExpr(d, src.TimeColumn, colSQL)
		selects = append(selects, fmt.Sprintf("%s AS `%s`", expr, sqlOutAlias(dname, d.Name)))
		groups = append(groups, expr)
	}

	skipTimeWhere := false
	rangeStart, rangeEnd := "", ""
	if req.TimeRange != nil {
		rangeStart, rangeEnd = req.TimeRange.Start, req.TimeRange.End
	}
	if len(req.Measures) == 0 {
		req.Measures = defaultMeasures(model)
	}
	for _, mname := range req.Measures {
		joinField := strings.HasPrefix(mname, "join.")
		raw := strings.TrimPrefix(mname, "join.")
		src := model
		if joinField {
			if joinMeta == nil {
				return "", ev, fmt.Errorf("medida %q pede cruzamento, mas nenhum conjunto extra foi ligado", mname)
			}
			src = joinMeta.Model
		}
		m, ok := semantic.ResolveMeasure(src, raw)
		if !ok {
			return "", ev, fmt.Errorf("medida desconhecida %q — não invento a definição da métrica", mname)
		}
		if measureUsesTimeIntel(m) {
			skipTimeWhere = true
		}
		expr, err := measureSQL(m, &src, rangeStart, rangeEnd)
		if err != nil {
			return "", ev, err
		}
		if joinMeta != nil {
			aliasTbl := "a"
			if joinField {
				aliasTbl = "b"
			}
			expr = qualifyIdentExpr(expr, aliasTbl)
		}
		selects = append(selects, fmt.Sprintf("%s AS `%s`", expr, sqlOutAlias(mname, m.Name)))
		ev.Metrics = append(ev.Metrics, m.Name+" = "+m.Expression)
	}

	var where []string
	if joinMeta != nil {
		where = append(where, fmt.Sprintf("a._tenant = '%s'", orgID.String()))
	} else {
		where = append(where, fmt.Sprintf("_tenant = '%s'", orgID.String()))
	}
	if req.TimeRange != nil && model.TimeColumn != "" && identOK(model.TimeColumn) && !skipTimeWhere {
		where = append(where, timeFilterSQL(qualify(false, model.TimeColumn), req.TimeRange.Start, req.TimeRange.End)...)
		ev.Period = req.TimeRange.Start + " → " + req.TimeRange.End
	}
	for _, f := range req.Filters {
		clause, err := filterClauseQualified(model, joinMeta, f, qualify)
		if err != nil {
			return "", ev, err
		}
		if clause != "" {
			where = append(where, clause)
		}
	}
	for _, f := range req.GlobalFilters {
		clause, err := filterClauseQualified(model, joinMeta, f, qualify)
		if err != nil {
			return "", ev, err
		}
		if clause != "" {
			where = append(where, clause)
		}
	}
	if len(req.DrillPath) > 0 {
		for i, d := range req.DrillPath {
			if i < len(req.Dimensions) {
				dname := req.Dimensions[i]
				joinField := strings.HasPrefix(dname, "join.")
				raw := strings.TrimPrefix(dname, "join.")
				where = append(where, fmt.Sprintf("%s = %s", qualify(joinField, raw), literal(d)))
			}
		}
	}

	if userID != uuid.Nil {
		rlsPreds, err := e.planner.rlsPredicates(ctx, meta, userID, role)
		if err == nil {
			if joinMeta != nil {
				for i, p := range rlsPreds {
					if strings.Contains(p, "`") {
						rlsPreds[i] = qualifyIdentExpr(p, "a")
					}
				}
			}
			where = append(where, rlsPreds...)
		}
	}

	var b strings.Builder
	if joinMeta != nil {
		joinPlan := e.planner.choosePlan(*joinMeta, Request{})
		joinTable := joinPlan.TargetTable
		if joinTable == "" {
			joinTable = joinMeta.Table
		}
		kind := "INNER JOIN"
		if strings.EqualFold(join.Match, "all_left") {
			kind = "LEFT JOIN"
		}
		fmt.Fprintf(&b, "SELECT %s FROM %s.`%s` AS a %s %s.`%s` AS b ON a.`%s` = b.`%s` AND b._tenant = '%s'",
			strings.Join(selects, ", "),
			e.cfg.ClickHouseDB, target, kind, e.cfg.ClickHouseDB, joinTable,
			join.FromColumn, join.ToColumn, orgID.String())
	} else {
		fmt.Fprintf(&b, "SELECT %s FROM %s.`%s`", strings.Join(selects, ", "), e.cfg.ClickHouseDB, target)
	}
	b.WriteString(" WHERE ")
	b.WriteString(strings.Join(where, " AND "))
	if len(groups) > 0 {
		b.WriteString(" GROUP BY ")
		b.WriteString(strings.Join(groups, ", "))
	}
	if len(req.OrderBy) > 0 {
		ords := make([]string, 0, len(req.OrderBy))
		for _, o := range req.OrderBy {
			field := sqlOutAlias(o.Field, o.Field)
			if !identOK(field) {
				continue
			}
			dir := "ASC"
			if strings.EqualFold(o.Dir, "desc") {
				dir = "DESC"
			}
			ords = append(ords, "`"+field+"` "+dir)
		}
		if len(ords) > 0 {
			b.WriteString(" ORDER BY ")
			b.WriteString(strings.Join(ords, ", "))
		}
	} else if len(req.Measures) > 0 {
		b.WriteString(" ORDER BY " + fmt.Sprintf("`%s` DESC", sqlOutAlias(req.Measures[0], req.Measures[0])))
	}
	lim := req.Limit
	if lim <= 0 {
		lim = 900
	}
	fmt.Fprintf(&b, " LIMIT %d", lim)
	return b.String(), ev, nil
}

func dimensionExpr(d semantic.Dimension, timeCol, colSQL string) string {
	t := strings.ToLower(d.Type)
	if t == "date" || t == "datetime" || (timeCol != "" && strings.EqualFold(d.Column, timeCol)) {
		return fmt.Sprintf("toDate(parseDateTimeBestEffortOrNull(toString(%s)))", colSQL)
	}
	return colSQL
}

func timeFilterSQL(colSQL, start, end string) []string {
	var w []string
	if start != "" {
		w = append(w, fmt.Sprintf("%s >= parseDateTimeBestEffort('%s')", colSQL, sanitizeLiteral(start)))
	}
	if end != "" {
		lit := sanitizeLiteral(end)
		if isDateOnly(lit) {
			w = append(w, fmt.Sprintf("%s < parseDateTimeBestEffort('%s') + INTERVAL 1 DAY", colSQL, lit))
		} else {
			w = append(w, fmt.Sprintf("%s < parseDateTimeBestEffort('%s')", colSQL, lit))
		}
	}
	return w
}

func timeFilterSimple(col, start, end string) []string {
	var w []string
	if start != "" {
		w = append(w, fmt.Sprintf("`%s` >= '%s'", col, sanitizeLiteral(start)))
	}
	if end != "" {
		lit := sanitizeLiteral(end)
		if isDateOnly(lit) {
			w = append(w, fmt.Sprintf("`%s` < '%s'", col, nextDate(lit)))
		} else {
			w = append(w, fmt.Sprintf("`%s` < '%s'", col, lit))
		}
	}
	return w
}

func isDateOnly(s string) bool {
	if len(s) != 10 {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func nextDate(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.AddDate(0, 0, 1).Format("2006-01-02")
}

func sqlOutAlias(requested, fallback string) string {
	name := requested
	if name == "" {
		name = fallback
	}
	plain := strings.TrimPrefix(name, "join.")
	if strings.HasPrefix(requested, "join.") {
		return "join_" + alias(plain)
	}
	return alias(plain)
}

func qualifyIdentExpr(expr, tableAlias string) string {
	var b strings.Builder
	inIdent := false
	for i := 0; i < len(expr); i++ {
		if expr[i] == '`' {
			if !inIdent {
				b.WriteString(tableAlias)
				b.WriteByte('.')
				b.WriteByte('`')
				inIdent = true
			} else {
				b.WriteByte('`')
				inIdent = false
			}
			continue
		}
		b.WriteByte(expr[i])
	}
	return b.String()
}

func filterClauseQualified(model semantic.Model, joinMeta *datasetInfo, f Filter, qualify func(bool, string) string) (string, error) {
	joinField := strings.HasPrefix(f.Dimension, "join.")
	raw := strings.TrimPrefix(f.Dimension, "join.")
	src := model
	if joinField {
		if joinMeta == nil {
			return "", nil
		}
		src = joinMeta.Model
	}
	clause, err := filterClause(src, Filter{Dimension: raw, Op: f.Op, Value: f.Value})
	if err != nil || clause == "" || joinMeta == nil {
		return clause, err
	}
	d, ok := semantic.ResolveDimension(src, raw)
	col := raw
	if ok {
		col = d.Column
	}
	return strings.Replace(clause, "`"+col+"`", qualify(joinField, col), 1), nil
}

func filterClause(model semantic.Model, f Filter) (string, error) {
	d, ok := semantic.ResolveDimension(model, f.Dimension)
	if !ok && !columnExists(model, f.Dimension) {
		return "", nil
	}
	col := f.Dimension
	if ok {
		col = d.Column
	}
	if !identOK(col) || !columnExists(model, col) {
		return "", nil
	}
	return filterSQL(col, f.Op, f.Value)
}

func filterSQL(col, op string, value any) (string, error) {
	switch strings.ToLower(op) {
	case "eq", "=":
		return fmt.Sprintf("`%s` = %s", col, literal(value)), nil
	case "neq", "!=":
		return fmt.Sprintf("`%s` != %s", col, literal(value)), nil
	case "gt":
		return fmt.Sprintf("`%s` > %s", col, literal(value)), nil
	case "gte":
		return fmt.Sprintf("`%s` >= %s", col, literal(value)), nil
	case "lt":
		return fmt.Sprintf("`%s` < %s", col, literal(value)), nil
	case "lte":
		return fmt.Sprintf("`%s` <= %s", col, literal(value)), nil
	case "in":
		vs := toSlice(value)
		parts := make([]string, len(vs))
		for i, v := range vs {
			parts[i] = literal(v)
		}
		return fmt.Sprintf("`%s` IN (%s)", col, strings.Join(parts, ", ")), nil
	case "contains":
		return fmt.Sprintf("positionCaseInsensitiveUTF8(toString(`%s`), %s) > 0", col, literal(value)), nil
	default:
		return "", fmt.Errorf("unsupported filter op")
	}
}

func literal(v any) string {
	switch t := v.(type) {
	case float64, float32, int, int64, json.Number:
		return sanitizeLiteral(fmt.Sprint(t))
	case bool:
		if t {
			return "1"
		}
		return "0"
	case []any:
		parts := make([]string, len(t))
		for i, x := range t {
			parts[i] = literal(x)
		}
		return strings.Join(parts, ", ")
	case []string:
		parts := make([]string, len(t))
		for i, x := range t {
			parts[i] = literal(x)
		}
		return strings.Join(parts, ", ")
	default:
		return "'" + sanitizeLiteral(fmt.Sprint(t)) + "'"
	}
}

func toSlice(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case []string:
		out := make([]any, len(t))
		for i, s := range t {
			out[i] = s
		}
		return out
	default:
		return []any{v}
	}
}

func sanitizeLiteral(s string) string {
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, ";", "")
	s = strings.ReplaceAll(s, "--", "")
	return s
}

func identOK(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func columnExists(model semantic.Model, name string) bool {
	n := strings.ToLower(name)
	if strings.ToLower(model.TimeColumn) == n {
		return true
	}
	for _, d := range model.Dimensions {
		if strings.ToLower(d.Column) == n {
			return true
		}
	}
	for _, m := range model.Measures {
		if strings.ToLower(m.Column) == n {
			return true
		}
	}
	return false
}

func alias(s string) string {
	s = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "_"))
	return s
}

func defaultMeasures(m semantic.Model) []string {
	if name := semantic.PrimaryMeasure(m); name != "" {
		return []string{name}
	}
	if len(m.Measures) == 0 {
		return nil
	}
	return []string{m.Measures[0].Name}
}

func fingerprint(orgID, wsID, userID uuid.UUID, req Request) string {
	b, _ := json.Marshal(req)
	sum := sha256.Sum256(append(append(append(orgID[:], wsID[:]...), userID[:]...), b...))
	return fmt.Sprintf("%x", sum[:])
}

func (e *Engine) getCache(ctx context.Context, fp string, ttl time.Duration) (Result, bool) {
	if e.rdb == nil {
		return Result{}, false
	}
	raw, err := e.rdb.Get(ctx, "q:"+fp).Bytes()
	if err != nil {
		return Result{}, false
	}
	var r Result
	if json.Unmarshal(raw, &r) != nil {
		return Result{}, false
	}
	return r, true
}

func (e *Engine) setCache(ctx context.Context, fp string, r Result, ttl time.Duration) {
	if e.rdb == nil {
		return
	}
	b, _ := json.Marshal(r)
	_ = e.rdb.Set(ctx, "q:"+fp, b, ttl).Err()
}

func (e *Engine) record(ctx context.Context, orgID, wsID, userID uuid.UUID, fp string, req Request, sql string, res Result, cache bool, planner string) {
	qj, _ := json.Marshal(req)
	var uid any
	if userID != uuid.Nil {
		uid = userID
	}
	_, _ = e.pg.Exec(ctx, `
		INSERT INTO query_history (org_id, workspace_id, user_id, fingerprint, query_json, sql_text, duration_ms, row_count, bytes_read, cache_hit, planner_choice, source_type)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, orgID, wsID, uid, fp, qj, sql, res.DurationMs, res.RowCount, res.BytesRead, cache, planner, planner)
}

func collectRows(rows driver.Rows, limit int) ([]string, []map[string]any, error) {
	cols := rows.Columns()
	types := rows.ColumnTypes()
	var out []map[string]any
	for rows.Next() {
		ptrs := make([]any, len(cols))
		for i, t := range types {
			st := t.ScanType()
			if st == nil {
				var s *string
				ptrs[i] = &s
				continue
			}
			ptrs[i] = reflect.New(st).Interface()
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		row := map[string]any{}
		for i, c := range cols {
			row[c] = normalizeValue(deref(ptrs[i]))
		}
		out = append(out, row)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	return cols, out, rows.Err()
}

func deref(p any) any {
	v := reflect.ValueOf(p)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return nil
	}
	return v.Interface()
}

func normalizeValue(v any) any {
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(time.RFC3339)
	case *time.Time:
		if t == nil {
			return nil
		}
		return t.UTC().Format(time.RFC3339)
	default:
		return v
	}
}

func estimateBytes(cols []string, rows []map[string]any) int64 {
	var n int64
	for _, r := range rows {
		for _, c := range cols {
			n += int64(len(fmt.Sprint(r[c])))
		}
	}
	return n
}
