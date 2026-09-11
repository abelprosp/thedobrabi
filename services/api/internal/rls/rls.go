package rls

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Rule is a row-level security predicate attached to a dataset.
type Rule struct {
	ID         uuid.UUID `json:"id"`
	DatasetID  uuid.UUID `json:"dataset_id"`
	Role       string    `json:"role"`
	ColumnName string    `json:"column_name"`
	Expression string    `json:"expression"`
}

// Store persists and retrieves RLS rules.
type Store struct{ pg *pgxpool.Pool }

func NewStore(pg *pgxpool.Pool) *Store { return &Store{pg: pg} }

func (s *Store) List(ctx context.Context, orgID, wsID, datasetID uuid.UUID) ([]Rule, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id, dataset_id, role, column_name, expression
		FROM dataset_rls
		WHERE org_id=$1 AND workspace_id=$2 AND dataset_id=$3
		ORDER BY created_at
	`, orgID, wsID, datasetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.DatasetID, &r.Role, &r.ColumnName, &r.Expression); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) Create(ctx context.Context, orgID, wsID, datasetID uuid.UUID, role, column, expr string) (uuid.UUID, error) {
	if role == "" {
		role = "viewer"
	}
	if err := ValidateRule(role, column, expr); err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	_, err := s.pg.Exec(ctx, `
		INSERT INTO dataset_rls (id, org_id, workspace_id, dataset_id, role, column_name, expression)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, id, orgID, wsID, datasetID, role, column, expr)
	return id, err
}

func (s *Store) Update(ctx context.Context, orgID, wsID, id uuid.UUID, role, column, expr string) error {
	if err := ValidateRule(role, column, expr); err != nil {
		return err
	}
	ct, err := s.pg.Exec(ctx, `
		UPDATE dataset_rls SET role=$1, column_name=$2, expression=$3, updated_at=now()
		WHERE id=$4 AND org_id=$5 AND workspace_id=$6
	`, role, column, expr, id, orgID, wsID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

var rlsIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateRule accepts only a small predicate language. RLS rules are later
// embedded in ClickHouse SQL, so arbitrary SQL must never be stored.
func ValidateRule(role, column, expr string) error {
	if role != "owner" && role != "admin" && role != "analyst" && role != "viewer" {
		return fmt.Errorf("função de RLS inválida")
	}
	column = strings.TrimSpace(column)
	expr = strings.TrimSpace(expr)
	if !rlsIdentifier.MatchString(column) {
		return fmt.Errorf("coluna de RLS inválida")
	}
	if expr == "" || len(expr) > 500 {
		return fmt.Errorf("expressão de RLS inválida")
	}
	upper := strings.ToUpper(expr)
	for _, forbidden := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "UNION", "JOIN", "FROM", "--", "/*", "*/", ";"} {
		if strings.Contains(upper, forbidden) {
			return fmt.Errorf("expressão de RLS contém SQL não permitido")
		}
	}
	if strings.Contains(upper, " OR ") || strings.Contains(upper, "||") {
		return fmt.Errorf("expressões OR não são permitidas em regras de RLS")
	}
	for _, r := range expr {
		if !(r == '_' || r == '-' || r == '+' || r == '*' || r == '/' || r == '(' || r == ')' ||
			r == ',' || r == '.' || r == '=' || r == '!' || r == '<' || r == '>' ||
			r == '\'' || r == '"' || r == ' ' || r == '\t' ||
			r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
			return fmt.Errorf("caractere não permitido na expressão de RLS")
		}
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, orgID, wsID, id uuid.UUID) error {
	_, err := s.pg.Exec(ctx, `DELETE FROM dataset_rls WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, orgID, wsID)
	return err
}

// Predicates returns SQL WHERE clauses for the given user and role. It replaces current_user_id()
// with the quoted user UUID, and tenant_id/org_id placeholders with the org UUID.
func Predicates(rules []Rule, orgID, userID uuid.UUID, role string) []string {
	var out []string
	for _, r := range rules {
		if r.Role != "" && r.Role != role && r.Role != "viewer" {
			continue
		}
		expr := r.Expression
		expr = strings.ReplaceAll(expr, "current_user_id()", "'"+userID.String()+"'")
		expr = strings.ReplaceAll(expr, "current_org_id()", "'"+orgID.String()+"'")
		expr = strings.ReplaceAll(expr, "tenant_id", "_tenant")
		if strings.Contains(expr, "=") && r.ColumnName != "" && !strings.Contains(expr, r.ColumnName) {
			expr = fmt.Sprintf("`%s` %s", r.ColumnName, expr)
		}
		out = append(out, expr)
	}
	return out
}

// ApplyToSQL injects RLS predicates into an existing SQL query after the WHERE clause.
// It is a simple string manipulator that assumes the query already has a WHERE clause.
func ApplyToSQL(sql string, predicates []string) string {
	if len(predicates) == 0 {
		return sql
	}
	pred := strings.Join(predicates, " AND ")
	// Append to existing WHERE clause. If there is an ORDER BY or GROUP BY, insert before them.
	lower := strings.ToLower(sql)
	idxOrder := strings.Index(lower, " order by ")
	idxLimit := strings.Index(lower, " limit ")
	insertAt := len(sql)
	if idxOrder >= 0 && idxOrder < insertAt {
		insertAt = idxOrder
	}
	if idxLimit >= 0 && idxLimit < insertAt {
		insertAt = idxLimit
	}
	before := sql[:insertAt]
	after := sql[insertAt:]
	return before + " AND " + pred + after
}
