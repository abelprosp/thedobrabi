package queryeng

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/thedobra/thedobra/services/api/internal/rls"
	"github.com/thedobra/thedobra/services/api/internal/schemax"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
	"github.com/thedobra/thedobra/services/api/internal/semanticxpr"
)

// Planner chooses the execution path for a query based on dataset metadata and storage_mode.
type Planner struct {
	pg  *pgxpool.Pool
	rdb *redis.Client
}

func NewPlanner(pg *pgxpool.Pool, rdb *redis.Client) *Planner {
	return &Planner{pg: pg, rdb: rdb}
}

type datasetInfo struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	WorkspaceID    uuid.UUID
	Name           string
	Table          string
	StorageMode    string
	SourceTable    *string
	SourceQuery    *string
	SchemaJSON     []byte
	ModelJSON      []byte
	Model          semantic.Model
	RowCount       int64
	UpdatedAt      time.Time
	ModelUpdatedAt time.Time
}

type plan struct {
	SourceType    string // clickhouse_import, clickhouse_direct_query, parquet_lake, duckdb
	TargetTable   string
	SourceQuery   string
	CacheTTL      time.Duration
	UseClickHouse bool
}

func (p *Planner) loadDataset(ctx context.Context, orgID, wsID uuid.UUID, datasetID string) (datasetInfo, error) {
	id, err := uuid.Parse(datasetID)
	if err != nil {
		return datasetInfo{}, fmt.Errorf("invalid dataset")
	}
	var m datasetInfo
	m.ID = id
	m.OrgID = orgID
	err = p.pg.QueryRow(ctx, `
		SELECT d.workspace_id, d.name, d.clickhouse_table, d.storage_mode, d.source_table, d.source_query, d.schema_json, d.row_count,
			COALESCE(s.model_json, '{}'::jsonb), d.updated_at, COALESCE(s.updated_at, d.updated_at)
		FROM datasets d
		LEFT JOIN semantic_models s ON s.dataset_id = d.id
		WHERE d.id=$1 AND d.org_id=$2 AND d.workspace_id=$3
	`, id, orgID, wsID).Scan(&m.WorkspaceID, &m.Name, &m.Table, &m.StorageMode, &m.SourceTable, &m.SourceQuery, &m.SchemaJSON, &m.RowCount, &m.ModelJSON, &m.UpdatedAt, &m.ModelUpdatedAt)
	if err != nil {
		return datasetInfo{}, fmt.Errorf("dataset not found")
	}
	_ = json.Unmarshal(m.ModelJSON, &m.Model)
	var cols []schemax.Column
	_ = json.Unmarshal(m.SchemaJSON, &cols)
	m.Model = semantic.Hydrate(m.Model, cols)
	if m.Table == "" || !identOK(m.Table) {
		return datasetInfo{}, fmt.Errorf("invalid table")
	}
	return m, nil
}

func (p *Planner) choosePlan(meta datasetInfo, req Request) plan {
	pl := plan{CacheTTL: 2 * time.Minute, UseClickHouse: true}
	switch meta.StorageMode {
	case "direct_query":
		pl.SourceType = "clickhouse_direct_query"
		if meta.SourceTable != nil && *meta.SourceTable != "" && identOK(*meta.SourceTable) {
			pl.TargetTable = *meta.SourceTable
		} else {
			pl.TargetTable = meta.Table
		}
	case "composite":
		pl.SourceType = "clickhouse_import"
		pl.TargetTable = meta.Table
	case "import":
		fallthrough
	default:
		pl.SourceType = "clickhouse_import"
		pl.TargetTable = meta.Table
	}
	// Always compile aggregations in ClickHouse so CASE/NULLIF/COUNT DISTINCT stay exact.
	if meta.StorageMode == "import" && meta.RowCount > 0 && meta.RowCount < 1000 {
		pl.CacheTTL = 30 * time.Second
	}
	return pl
}

// measureSQL resolves a measure to a SQL fragment. If the measure expression is a DAX-like expression,
// it uses semanticxpr to translate it, including dependent measure references. Otherwise it falls back
// to the simple aggregation mapping.
func measureSQL(m semantic.Measure, model *semantic.Model, rangeStart, rangeEnd string) (string, error) {
	opts := semanticxpr.SQLOptions{RangeStart: rangeStart, RangeEnd: rangeEnd}
	if model != nil {
		opts.TimeColumn = model.TimeColumn
	}
	if m.Expression != "" {
		expr, err := semanticxpr.Parse(m.Expression)
		if err == nil && expr.Func != "COLUMN" && expr.Func != "LITERAL" {
			var resolver semanticxpr.MeasureResolver
			if model != nil {
				resolver = measureResolver(model)
			}
			return expr.ToSQLWithOptions(func(col string) string { return "`" + col + "`" }, resolver, opts)
		}
	}
	agg := strings.ToLower(m.Aggregation)
	if m.Column == "*" && agg == "count" {
		return "COUNT(*)", nil
	}
	if !identOK(m.Column) {
		return "", fmt.Errorf("invalid measure column")
	}
	switch agg {
	case "sum":
		return "SUM(" + semanticxpr.AsFloat64("`"+m.Column+"`") + ")", nil
	case "avg", "average":
		return "AVG(" + semanticxpr.AsFloat64("`"+m.Column+"`") + ")", nil
	case "min":
		return "MIN(" + semanticxpr.AsFloat64("`"+m.Column+"`") + ")", nil
	case "max":
		return "MAX(" + semanticxpr.AsFloat64("`"+m.Column+"`") + ")", nil
	case "count":
		return "COUNT(`" + m.Column + "`)", nil
	case "count_distinct":
		return "uniqExact(`" + m.Column + "`)", nil
	default:
		return "", fmt.Errorf("unsupported aggregation %s", agg)
	}
}

func measureUsesTimeIntel(m semantic.Measure) bool {
	if strings.TrimSpace(m.Expression) == "" {
		return false
	}
	expr, err := semanticxpr.Parse(m.Expression)
	if err != nil {
		return false
	}
	return expr.UsesTimeIntel()
}

func measureResolver(model *semantic.Model) semanticxpr.MeasureResolver {
	return func(name string) (semanticxpr.Expr, error) {
		m, ok := semantic.ResolveMeasure(*model, name)
		if !ok {
			return semanticxpr.Expr{}, fmt.Errorf("measure not found: %s", name)
		}
		return semanticxpr.Parse(m.Expression)
	}
}

// rlsPredicates loads RLS rules for the dataset and returns SQL predicates.
func (p *Planner) rlsPredicates(ctx context.Context, meta datasetInfo, userID uuid.UUID, role string) ([]string, error) {
	rules, err := p.loadRLSRules(ctx, meta)
	if err != nil {
		return nil, err
	}
	return rls.Predicates(rules, meta.OrgID, userID, role), nil
}

func (p *Planner) loadRLSRules(ctx context.Context, meta datasetInfo) ([]rls.Rule, error) {
	rows, err := p.pg.Query(ctx, `
		SELECT role, column_name, expression
		FROM dataset_rls
		WHERE org_id=$1 AND workspace_id=$2 AND dataset_id=$3
		ORDER BY role, column_name, expression
	`, meta.OrgID, meta.WorkspaceID, meta.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []rls.Rule
	for rows.Next() {
		var r rls.Rule
		if err := rows.Scan(&r.Role, &r.ColumnName, &r.Expression); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// permissionFingerprint hashes RLS rules (and join dataset rules) so cache keys
// change when access policies change, without returning hits before RLS is considered.
func (p *Planner) permissionFingerprint(ctx context.Context, meta datasetInfo, joins []DatasetJoin, userID uuid.UUID, role string) (string, error) {
	type ruleKey struct {
		Dataset string `json:"dataset"`
		Role    string `json:"role"`
		Column  string `json:"column"`
		Expr    string `json:"expr"`
	}
	var keys []ruleKey
	appendRules := func(ds datasetInfo) error {
		rules, err := p.loadRLSRules(ctx, ds)
		if err != nil {
			return err
		}
		for _, r := range rules {
			keys = append(keys, ruleKey{Dataset: ds.ID.String(), Role: r.Role, Column: r.ColumnName, Expr: r.Expression})
		}
		return nil
	}
	if err := appendRules(meta); err != nil {
		return "", err
	}
	for _, j := range joins {
		jm, err := p.loadDataset(ctx, meta.OrgID, meta.WorkspaceID, j.DatasetID)
		if err != nil {
			continue
		}
		if err := appendRules(jm); err != nil {
			return "", err
		}
	}
	b, _ := json.Marshal(struct {
		Role  string    `json:"role"`
		User  string    `json:"user"`
		Rules []ruleKey `json:"rules"`
	}{Role: role, User: userID.String(), Rules: keys})
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:]), nil
}
