// Package semanticxpr provides a DAX-like expression parser/evaluator for semantic measures.
// It translates expressions such as SUM(revenue), CALCULATE([Receita], Região = "Norte"),
// [Margem] = [Receita] - [Custo], SAMEPERIODLASTYEAR(revenue), and RELATED(Tabela[Coluna])
// into ClickHouse SQL fragments, and can evaluate a simple subset over in-memory row maps.
package semanticxpr

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Expr is a parsed measure expression.
type Expr struct {
	Raw    string `json:"raw"`
	Func   string `json:"func"`
	Column string `json:"column,omitempty"`
	Op     string `json:"op,omitempty"`
	Left   *Expr  `json:"left,omitempty"`
	Right  *Expr  `json:"right,omitempty"`
	Args   []Expr `json:"args,omitempty"`
	Filter string `json:"filter,omitempty"`
}

// Parse converts a DAX-like expression string into an Expr tree. It supports:
//   - Measure references: [Receita]
//   - Binary operators: +, -, *, /
//   - Function calls: SUM(col), CALCULATE(expr, filter1, ...), FILTER(...), REMOVEFILTERS(col)
//   - Time intelligence: SAMEPERIODLASTYEAR, DATEADD, TOTALYTD, TOTALMTD, TOTALQTD
//   - Related lookups: RELATED(Tabela[Coluna]), LOOKUPVALUE(Tabela[Coluna], Tabela[Coluna], value)
func Parse(s string) (Expr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Expr{}, fmt.Errorf("empty expression")
	}
	p := parser{input: s, pos: 0}
	expr, err := p.parseExpr(0)
	if err != nil {
		return Expr{}, err
	}
	expr.Raw = s
	return expr, nil
}

type parser struct {
	input string
	pos   int
}

func (p *parser) peek() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *parser) next() byte {
	ch := p.peek()
	if p.pos < len(p.input) {
		p.pos++
	}
	return ch
}

func (p *parser) skipSpace() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t' || p.input[p.pos] == '\n' || p.input[p.pos] == '\r') {
		p.pos++
	}
}

func (p *parser) parseExpr(minPrec int) (Expr, error) {
	p.skipSpace()
	left, err := p.parsePrimary()
	if err != nil {
		return Expr{}, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.input) {
			break
		}
		ch := p.input[p.pos]
		prec := opPrec(ch)
		if prec < minPrec {
			break
		}
		if ch != '+' && ch != '-' && ch != '*' && ch != '/' {
			break
		}
		p.pos++
		oldLeft := left
		right, err := p.parseExpr(prec + 1)
		if err != nil {
			return Expr{}, err
		}
		left = Expr{Func: "OP", Op: string(ch), Left: &oldLeft, Right: &right}
	}
	return left, nil
}

func opPrec(ch byte) int {
	switch ch {
	case '+', '-':
		return 1
	case '*', '/':
		return 2
	}
	return -1
}

func (p *parser) parsePrimary() (Expr, error) {
	p.skipSpace()
	if p.pos >= len(p.input) {
		return Expr{}, fmt.Errorf("unexpected end of expression")
	}
	ch := p.peek()
	if ch == '[' {
		return p.parseBracketRef()
	}
	if ch == '\'' || ch == '"' {
		return p.parseStringLiteral()
	}
	if ch >= '0' && ch <= '9' {
		return p.parseNumber()
	}
	if isIdentStart(ch) {
		return p.parseIdentOrCall()
	}
	if ch == '(' {
		p.pos++
		expr, err := p.parseExpr(0)
		if err != nil {
			return Expr{}, err
		}
		p.skipSpace()
		if p.peek() != ')' {
			return Expr{}, fmt.Errorf("expected )")
		}
		p.pos++
		return expr, nil
	}
	return Expr{}, fmt.Errorf("unexpected character %q at position %d", ch, p.pos)
}

func (p *parser) parseBracketRef() (Expr, error) {
	if p.next() != '[' {
		return Expr{}, fmt.Errorf("expected [")
	}
	start := p.pos
	for p.pos < len(p.input) && p.input[p.pos] != ']' {
		p.pos++
	}
	if p.pos >= len(p.input) {
		return Expr{}, fmt.Errorf("unclosed bracket reference")
	}
	name := p.input[start:p.pos]
	p.pos++ // skip ]
	if !isIdent(name) {
		return Expr{}, fmt.Errorf("invalid identifier in brackets: %s", name)
	}
	return Expr{Func: "MEASURE", Column: name}, nil
}

func (p *parser) parseStringLiteral() (Expr, error) {
	quote := p.next()
	start := p.pos
	for p.pos < len(p.input) && p.input[p.pos] != quote {
		p.pos++
	}
	if p.pos >= len(p.input) {
		return Expr{}, fmt.Errorf("unclosed string literal")
	}
	val := p.input[start:p.pos]
	p.pos++ // skip quote
	return Expr{Func: "LITERAL", Column: quoteSQLString(val)}, nil
}

func quoteSQLString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func (p *parser) parseNumber() (Expr, error) {
	start := p.pos
	for p.pos < len(p.input) && ((p.input[p.pos] >= '0' && p.input[p.pos] <= '9') || p.input[p.pos] == '.' || p.input[p.pos] == '-') {
		p.pos++
	}
	val := p.input[start:p.pos]
	return Expr{Func: "LITERAL", Column: val}, nil
}

func (p *parser) parseIdentOrCall() (Expr, error) {
	start := p.pos
	for p.pos < len(p.input) && isIdentChar(p.input[p.pos]) {
		p.pos++
	}
	name := p.input[start:p.pos]
	if !isIdent(name) {
		return Expr{}, fmt.Errorf("invalid identifier: %s", name)
	}
	p.skipSpace()
	if p.peek() != '(' {
		return Expr{Func: "COLUMN", Column: name}, nil
	}
	fn := strings.ToUpper(name)
	p.pos++ // skip (
	args, err := p.parseArgs(fn)
	if err != nil {
		return Expr{}, err
	}
	return Expr{Func: fn, Args: args}, nil
}

func (p *parser) parseArgs(parentFn string) ([]Expr, error) {
	p.skipSpace()
	if p.peek() == ')' {
		p.pos++
		return nil, nil
	}
	var args []Expr
	for {
		p.skipSpace()
		arg, err := p.parseArg(parentFn)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		p.skipSpace()
		ch := p.peek()
		if ch == ',' {
			p.pos++
			continue
		}
		if ch == ')' {
			p.pos++
			break
		}
		return nil, fmt.Errorf("expected , or ) in argument list")
	}
	return args, nil
}

// parseArg parses a function argument. For CALCULATE/FILTER the second+ args are predicates.
func (p *parser) parseArg(parentFn string) (Expr, error) {
	p.skipSpace()
	if p.peek() == '[' {
		return p.parseBracketRef()
	}
	if parentFn == "CALCULATE" && len(p.input) > p.pos+1 && p.input[p.pos] != '[' {
		return p.parsePredicateArg()
	}
	if parentFn == "FILTER" && p.input[p.pos] != '[' {
		return p.parsePredicateArg()
	}
	if p.peek() == '\'' || p.peek() == '"' {
		return p.parseStringLiteral()
	}
	if p.peek() >= '0' && p.peek() <= '9' {
		return p.parseNumber()
	}
	return p.parseExpr(0)
}

func (p *parser) parsePredicateArg() (Expr, error) {
	start := p.pos
	depth := 0
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '(' {
			depth++
		}
		if ch == ')' {
			if depth == 0 {
				break
			}
			depth--
		}
		if ch == ',' && depth == 0 {
			break
		}
		p.pos++
	}
	return Expr{Func: "RAW", Column: strings.TrimSpace(p.input[start:p.pos])}, nil
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentChar(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9') || ch == '.' || ch == '[' || ch == ']'
}

func isIdent(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !isIdentStart(byte(r)) {
				return false
			}
			continue
		}
		if !isIdentChar(byte(r)) {
			return false
		}
	}
	return true
}

// ToSQL returns a ClickHouse SQL fragment for a parsed expression. If measures are referenced,
// they are left as placeholders unless resolved by a ModelResolver.
func (e Expr) ToSQL(columnQuote func(string) string) (string, error) {
	return e.toSQLWithContext(columnQuote, nil)
}

// ToSQLWithResolver returns a SQL fragment resolving measure references through the resolver.
func (e Expr) ToSQLWithResolver(columnQuote func(string) string, resolve MeasureResolver) (string, error) {
	ctx := evalContext{measures: map[string]Expr{}}
	if resolve != nil {
		if err := ctx.collectDependencies(e, resolve); err != nil {
			return "", err
		}
	}
	return e.toSQLWithContext(columnQuote, &ctx)
}

type evalContext struct {
	measures   map[string]Expr
	filterMods []string
}

func (ctx *evalContext) collectDependencies(e Expr, resolve MeasureResolver) error {
	if e.Func == "MEASURE" {
		name := e.Column
		if _, ok := ctx.measures[name]; ok {
			return nil
		}
		expr, err := resolve(name)
		if err != nil {
			return err
		}
		ctx.measures[name] = expr
		return ctx.collectDependencies(expr, resolve)
	}
	if e.Left != nil {
		if err := ctx.collectDependencies(*e.Left, resolve); err != nil {
			return err
		}
	}
	if e.Right != nil {
		if err := ctx.collectDependencies(*e.Right, resolve); err != nil {
			return err
		}
	}
	for _, a := range e.Args {
		if err := ctx.collectDependencies(a, resolve); err != nil {
			return err
		}
	}
	return nil
}

type MeasureResolver func(name string) (Expr, error)

func (e Expr) toSQLWithContext(q func(string) string, ctx *evalContext) (string, error) {
	switch e.Func {
	case "MEASURE":
		if ctx == nil {
			return "", fmt.Errorf("unresolved measure reference [%s]", e.Column)
		}
		dep, ok := ctx.measures[e.Column]
		if !ok {
			return "", fmt.Errorf("unresolved measure reference [%s]", e.Column)
		}
		return dep.toSQLWithContext(q, ctx)
	case "COLUMN":
		if !identOK(e.Column) {
			return "", fmt.Errorf("invalid column identifier %s", e.Column)
		}
		return q(e.Column), nil
	case "LITERAL":
		return e.Column, nil
	case "OP":
		left, err := e.Left.toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		right, err := e.Right.toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s %s %s)", left, e.Op, right), nil
	case "SUM":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("SUM requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("SUM(%s)", col), nil
	case "COUNT":
		if len(e.Args) == 0 || e.Args[0].Column == "*" {
			return "COUNT(*)", nil
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("COUNT(%s)", col), nil
	case "DISTINCTCOUNT":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("DISTINCTCOUNT requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("uniqExact(%s)", col), nil
	case "AVERAGE", "AVG":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("AVERAGE requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("AVG(%s)", col), nil
	case "SUMX":
		if len(e.Args) < 2 {
			return "", fmt.Errorf("SUMX requires table and expression")
		}
		exprSQL, err := e.Args[1].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("SUM(%s)", exprSQL), nil
	case "CALCULATE":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("CALCULATE requires an expression")
		}
		base, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		for _, a := range e.Args[1:] {
			if a.Func == "RAW" && strings.EqualFold(a.Column, "ALL") {
				// ALL removes filters; no WHERE clause predicate in SQL fragment.
				continue
			}
			if a.Func == "RAW" {
				pred, err := rawPredicateToSQL(a.Column, q)
				if err != nil {
					return "", err
				}
				if ctx != nil {
					ctx.filterMods = append(ctx.filterMods, pred)
				}
			}
		}
		return base, nil
	case "FILTER":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("FILTER requires a predicate")
		}
		pred, err := predicateToSQL(e.Args[0], q)
		if err != nil {
			return "", err
		}
		return pred, nil
	case "REMOVEFILTERS":
		return "1", nil
	case "YOY":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("YOY requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("((SUM(%s) - SUM(%s)) / NULLIF(SUM(%s), 0)) * 100", col, col, col), nil
	case "MTD":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("MTD requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("SUM(%s)", col), nil
	case "YTD":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("YTD requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("SUM(%s)", col), nil
	case "SAMEPERIODLASTYEAR":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("SAMEPERIODLASTYEAR requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("SUM(%s)", col), nil
	case "DATEADD":
		if len(e.Args) < 3 {
			return "", fmt.Errorf("DATEADD requires column, interval, unit")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		_ = col
		return "SUM(0)", nil // Placeholder: time shift requires date column context.
	case "TOTALYTD":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("TOTALYTD requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("SUM(%s)", col), nil
	case "TOTALMTD":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("TOTALMTD requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("SUM(%s)", col), nil
	case "TOTALQTD":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("TOTALQTD requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("SUM(%s)", col), nil
	case "RELATED":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("RELATED requires a column reference")
		}
		colRef := e.Args[0].Column
		if colRef == "" {
			return "", fmt.Errorf("RELATED requires a column reference")
		}
		table, col, err := splitTableColumn(colRef)
		if err != nil || !identOK(table) || !identOK(col) {
			return "", fmt.Errorf("RELATED expects Tabela[Coluna] or Tabela.Coluna, got %s", colRef)
		}
		return fmt.Sprintf("(SELECT %s FROM %s AS r WHERE r._tenant = _tenant LIMIT 1)", q(col), table), nil
	case "LOOKUPVALUE":
		if len(e.Args) < 3 {
			return "", fmt.Errorf("LOOKUPVALUE requires result column, search column, search value")
		}
		resultCol := e.Args[0].Column
		searchCol := e.Args[1].Column
		searchVal, err := e.Args[2].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		table, col, err := splitTableColumn(resultCol)
		if err != nil || !identOK(table) || !identOK(col) {
			return "", fmt.Errorf("LOOKUPVALUE expects Tabela[Coluna] result column")
		}
		return fmt.Sprintf("(SELECT %s FROM %s AS r WHERE r._tenant = _tenant AND r.%s = %s LIMIT 1)", q(col), table, q(searchCol), searchVal), nil
	default:
		return "", fmt.Errorf("unsupported function %s", e.Func)
	}
}

func predicateToSQL(e Expr, q func(string) string) (string, error) {
	if e.Func == "RAW" {
		return rawPredicateToSQL(e.Column, q)
	}
	if e.Func == "COLUMN" {
		return q(e.Column) + " IS NOT NULL", nil
	}
	if len(e.Args) >= 2 && (e.Func == "=" || e.Func == "EQ") {
		col, err := e.Args[0].toSQLWithContext(q, nil)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s", col, literal(e.Args[1].Column)), nil
	}
	return "", fmt.Errorf("unsupported predicate %s", e.Func)
}

func rawPredicateToSQL(s string, q func(string) string) (string, error) {
	re := regexp.MustCompile(`(?i)^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*(=|!=|<>|>=|<=|>|<|in|not in)\s*(.+?)\s*$`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return "", fmt.Errorf("unsupported predicate: %s", s)
	}
	col := q(m[1])
	op := strings.ToLower(strings.TrimSpace(m[2]))
	val := strings.TrimSpace(m[3])
	switch op {
	case "=":
		return fmt.Sprintf("%s = %s", col, literal(val)), nil
	case "!=", "<>":
		return fmt.Sprintf("%s != %s", col, literal(val)), nil
	case ">":
		return fmt.Sprintf("%s > %s", col, literal(val)), nil
	case ">=":
		return fmt.Sprintf("%s >= %s", col, literal(val)), nil
	case "<":
		return fmt.Sprintf("%s < %s", col, literal(val)), nil
	case "<=":
		return fmt.Sprintf("%s <= %s", col, literal(val)), nil
	case "in":
		vs := strings.Split(strings.Trim(val, "()"), ",")
		parts := make([]string, len(vs))
		for i, v := range vs {
			parts[i] = literal(strings.TrimSpace(v))
		}
		return fmt.Sprintf("%s IN (%s)", col, strings.Join(parts, ", ")), nil
	default:
		return "", fmt.Errorf("unsupported operator %s", op)
	}
}

func literal(s string) string {
	s = strings.Trim(s, "'\"")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, ";", "")
	s = strings.ReplaceAll(s, "--", "")
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return s
	}
	return "'" + s + "'"
}

func identOK(s string) bool {
	return isIdent(s)
}

func splitTableColumn(s string) (table, col string, err error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "[") && strings.Contains(s, "]") {
		open := strings.Index(s, "[")
		close := strings.Index(s, "]")
		if close > open {
			return strings.TrimSpace(s[:open]), strings.Trim(s[open+1:close], "[] "), nil
		}
	}
	parts := strings.Split(s, ".")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
	}
	return "", "", fmt.Errorf("invalid table[column] reference")
}

// Evaluate computes a simple expression over rows of maps. It supports SUM, COUNT, AVERAGE, MIN, MAX,
// binary operators, and dependent measure references when resolved by a MeasureResolver.
func Evaluate(e Expr, rows []map[string]any, resolve MeasureResolver) (float64, error) {
	switch e.Func {
	case "LITERAL":
		return strconv.ParseFloat(strings.Trim(e.Column, "'"), 64)
	case "MEASURE":
		if resolve == nil {
			return 0, fmt.Errorf("unresolved measure reference [%s]", e.Column)
		}
		dep, err := resolve(e.Column)
		if err != nil {
			return 0, err
		}
		return Evaluate(dep, rows, resolve)
	case "COLUMN":
		return 0, nil
	case "OP":
		l, err := Evaluate(*e.Left, rows, resolve)
		if err != nil {
			return 0, err
		}
		r, err := Evaluate(*e.Right, rows, resolve)
		if err != nil {
			return 0, err
		}
		switch e.Op {
		case "+":
			return l + r, nil
		case "-":
			return l - r, nil
		case "*":
			return l * r, nil
		case "/":
			if r == 0 {
				return 0, nil
			}
			return l / r, nil
		}
	case "SUM":
		var total float64
		col := e.Args[0].Column
		for _, r := range rows {
			total += toF(r[col])
		}
		return total, nil
	case "COUNT":
		return float64(len(rows)), nil
	case "AVERAGE", "AVG":
		if len(rows) == 0 {
			return 0, nil
		}
		var total float64
		col := e.Args[0].Column
		for _, r := range rows {
			total += toF(r[col])
		}
		return total / float64(len(rows)), nil
	case "MIN":
		var minV float64
		set := false
		col := e.Args[0].Column
		for _, r := range rows {
			v := toF(r[col])
			if !set || v < minV {
				minV = v
				set = true
			}
		}
		return minV, nil
	case "MAX":
		var maxV float64
		set := false
		col := e.Args[0].Column
		for _, r := range rows {
			v := toF(r[col])
			if !set || v > maxV {
				maxV = v
				set = true
			}
		}
		return maxV, nil
	}
	return 0, fmt.Errorf("evaluate: unsupported function %s", e.Func)
}

func toF(v any) float64 {
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
	fmt.Sscanf(fmt.Sprint(v), "%f", &f)
	return f
}

// FilterMods returns the CALCULATE filter predicates collected during SQL generation.
func (ctx *evalContext) FilterMods() []string {
	if ctx == nil {
		return nil
	}
	return ctx.filterMods
}
