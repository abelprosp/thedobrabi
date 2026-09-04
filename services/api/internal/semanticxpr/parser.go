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
	p.skipSpace()
	if p.pos < len(p.input) {
		rest := strings.TrimSpace(p.input[p.pos:])
		up := strings.ToUpper(rest)
		if strings.HasPrefix(up, "FROM ") || strings.HasPrefix(up, "SELECT ") || strings.HasPrefix(up, "AS ") {
			return Expr{}, fmt.Errorf("a medida é só a expressão (SUM, AVG, CASE WHEN). Não uses SELECT, FROM nem AS")
		}
		return Expr{}, fmt.Errorf("texto extra depois da expressão: %s", rest)
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
	if ch == '-' {
		p.pos++
		p.skipSpace()
		if p.peek() >= '0' && p.peek() <= '9' {
			n, err := p.parseNumber()
			if err != nil {
				return Expr{}, err
			}
			n.Column = "-" + n.Column
			return n, nil
		}
		return Expr{}, fmt.Errorf("unexpected character %q at position %d", '-', p.pos-1)
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
	if strings.EqualFold(name, "CASE") {
		return p.parseCase()
	}
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

func (p *parser) lookingAtWord(word string) bool {
	p.skipSpace()
	if p.pos+len(word) > len(p.input) {
		return false
	}
	got := p.input[p.pos : p.pos+len(word)]
	if !strings.EqualFold(got, word) {
		return false
	}
	end := p.pos + len(word)
	return end == len(p.input) || !isIdentChar(p.input[end])
}

func (p *parser) consumeWord(word string) bool {
	if !p.lookingAtWord(word) {
		return false
	}
	p.pos += len(word)
	return true
}

func (p *parser) parseCompareOp() string {
	p.skipSpace()
	rest := p.input[p.pos:]
	for _, op := range []string{"!=", "<>", ">=", "<=", "=", ">", "<"} {
		if strings.HasPrefix(rest, op) {
			p.pos += len(op)
			if op == "<>" {
				return "!="
			}
			return op
		}
	}
	return ""
}

func (p *parser) parseComparison() (Expr, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return Expr{}, err
	}
	op := p.parseCompareOp()
	if op == "" {
		return left, nil
	}
	right, err := p.parsePrimary()
	if err != nil {
		return Expr{}, err
	}
	return Expr{Func: "CMP", Op: op, Left: &left, Right: &right}, nil
}

func (p *parser) parseBoolExpr() (Expr, error) {
	left, err := p.parseComparison()
	if err != nil {
		return Expr{}, err
	}
	for {
		if p.consumeWord("AND") {
			right, err := p.parseComparison()
			if err != nil {
				return Expr{}, err
			}
			l, r := left, right
			left = Expr{Func: "AND", Left: &l, Right: &r}
			continue
		}
		if p.consumeWord("OR") {
			right, err := p.parseComparison()
			if err != nil {
				return Expr{}, err
			}
			l, r := left, right
			left = Expr{Func: "OR", Left: &l, Right: &r}
			continue
		}
		break
	}
	return left, nil
}

func (p *parser) parseCase() (Expr, error) {
	var args []Expr
	for {
		p.skipSpace()
		if p.consumeWord("WHEN") {
			pred, err := p.parseBoolExpr()
			if err != nil {
				return Expr{}, err
			}
			if !p.consumeWord("THEN") {
				return Expr{}, fmt.Errorf("expected THEN after CASE WHEN")
			}
			thenExpr, err := p.parseExpr(0)
			if err != nil {
				return Expr{}, err
			}
			args = append(args, Expr{Func: "WHEN", Left: &pred, Right: &thenExpr})
			continue
		}
		if p.consumeWord("ELSE") {
			elseExpr, err := p.parseExpr(0)
			if err != nil {
				return Expr{}, err
			}
			args = append(args, Expr{Func: "ELSE", Args: []Expr{elseExpr}})
			continue
		}
		if p.consumeWord("END") {
			if len(args) == 0 {
				return Expr{}, fmt.Errorf("CASE requires WHEN ... THEN")
			}
			return Expr{Func: "CASE", Args: args}, nil
		}
		return Expr{}, fmt.Errorf("expected WHEN, ELSE or END in CASE")
	}
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
		var arg Expr
		var err error
		if parentFn == "IF" && len(args) == 0 {
			arg, err = p.parseBoolExpr()
		} else {
			arg, err = p.parseArg(parentFn)
		}
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
	if p.peek() == '*' {
		p.pos++
		return Expr{Func: "COLUMN", Column: "*"}, nil
	}
	if parentFn == "COUNT" && p.lookingAtWord("DISTINCT") {
		p.consumeWord("DISTINCT")
		p.skipSpace()
		if p.peek() == '*' {
			p.pos++
			return Expr{Func: "DISTINCT", Column: "*"}, nil
		}
		col, err := p.parsePrimary()
		if err != nil {
			return Expr{}, err
		}
		return Expr{Func: "DISTINCT", Args: []Expr{col}}, nil
	}
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

// SQLOptions carries model context so time intelligence compiles to real ClickHouse, not stubs.
type SQLOptions struct {
	TimeColumn string
	RangeStart string
	RangeEnd   string
}

// ToSQLWithResolver returns a SQL fragment resolving measure references through the resolver.
func (e Expr) ToSQLWithResolver(columnQuote func(string) string, resolve MeasureResolver) (string, error) {
	return e.ToSQLWithOptions(columnQuote, resolve, SQLOptions{})
}

// ToSQLWithOptions is ToSQLWithResolver plus time-column / period context.
func (e Expr) ToSQLWithOptions(columnQuote func(string) string, resolve MeasureResolver, opts SQLOptions) (string, error) {
	ctx := evalContext{measures: map[string]Expr{}, TimeColumn: opts.TimeColumn, RangeStart: opts.RangeStart, RangeEnd: opts.RangeEnd}
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
	TimeColumn string
	RangeStart string
	RangeEnd   string
}

// UsesTimeIntel reports whether the expression needs a date column and must not be
// constrained by the outer time-range WHERE (it encodes the period itself).
func (e Expr) UsesTimeIntel() bool {
	switch e.Func {
	case "YOY", "MTD", "YTD", "SAMEPERIODLASTYEAR", "DATEADD", "TOTALYTD", "TOTALMTD", "TOTALQTD":
		return true
	}
	if e.Left != nil && e.Left.UsesTimeIntel() {
		return true
	}
	if e.Right != nil && e.Right.UsesTimeIntel() {
		return true
	}
	for _, a := range e.Args {
		if a.UsesTimeIntel() {
			return true
		}
	}
	return false
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
		if e.Op == "/" {
			return fmt.Sprintf("divide(toFloat64(%s), nullIf(toFloat64(%s), 0))", left, right), nil
		}
		return fmt.Sprintf("(%s %s %s)", left, e.Op, right), nil
	case "AND":
		left, err := e.Left.toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		right, err := e.Right.toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s AND %s)", left, right), nil
	case "OR":
		left, err := e.Left.toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		right, err := e.Right.toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s OR %s)", left, right), nil
	case "SUM":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("SUM requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		if e.Args[0].Func == "COLUMN" {
			return fmt.Sprintf("SUM(toFloat64OrZero(%s))", col), nil
		}
		return fmt.Sprintf("SUM(%s)", col), nil
	case "MIN":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("MIN requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		if e.Args[0].Func == "COLUMN" {
			return fmt.Sprintf("MIN(toFloat64OrZero(%s))", col), nil
		}
		return fmt.Sprintf("MIN(%s)", col), nil
	case "MAX":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("MAX requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		if e.Args[0].Func == "COLUMN" {
			return fmt.Sprintf("MAX(toFloat64OrZero(%s))", col), nil
		}
		return fmt.Sprintf("MAX(%s)", col), nil
	case "NULLIF":
		if len(e.Args) < 2 {
			return "", fmt.Errorf("NULLIF requires two arguments")
		}
		a, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		b, err := e.Args[1].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("NULLIF(%s, %s)", a, b), nil
	case "CMP":
		if e.Left == nil || e.Right == nil {
			return "", fmt.Errorf("comparison requires two sides")
		}
		left, err := e.Left.toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		right, err := e.Right.toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s %s %s", left, e.Op, right), nil
	case "CASE":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("CASE requires WHEN ... THEN")
		}
		var b strings.Builder
		b.WriteString("CASE")
		for _, a := range e.Args {
			switch a.Func {
			case "WHEN":
				if a.Left == nil || a.Right == nil {
					return "", fmt.Errorf("CASE WHEN requires a condition and a value")
				}
				pred, err := a.Left.toSQLWithContext(q, ctx)
				if err != nil {
					return "", err
				}
				thenSQL, err := a.Right.toSQLWithContext(q, ctx)
				if err != nil {
					return "", err
				}
				b.WriteString(" WHEN ")
				b.WriteString(pred)
				b.WriteString(" THEN ")
				b.WriteString(thenSQL)
			case "ELSE":
				if len(a.Args) == 0 {
					return "", fmt.Errorf("CASE ELSE requires a value")
				}
				elseSQL, err := a.Args[0].toSQLWithContext(q, ctx)
				if err != nil {
					return "", err
				}
				b.WriteString(" ELSE ")
				b.WriteString(elseSQL)
			default:
				return "", fmt.Errorf("invalid CASE branch %s", a.Func)
			}
		}
		b.WriteString(" END")
		return b.String(), nil
	case "COUNT":
		if len(e.Args) == 0 || e.Args[0].Column == "*" {
			return "COUNT(*)", nil
		}
		if e.Args[0].Func == "DISTINCT" {
			inner := e.Args[0]
			if inner.Column == "*" {
				return "COUNT(*)", nil
			}
			if len(inner.Args) == 0 {
				return "", fmt.Errorf("COUNT DISTINCT requires a column")
			}
			col, err := inner.Args[0].toSQLWithContext(q, ctx)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("uniqExact(%s)", col), nil
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
		if e.Args[0].Func == "COLUMN" {
			return fmt.Sprintf("AVG(toFloat64OrZero(%s))", col), nil
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
		var preds []string
		for _, a := range e.Args[1:] {
			if a.Func == "RAW" && strings.EqualFold(a.Column, "ALL") {
				continue
			}
			if a.Func == "RAW" {
				pred, err := rawPredicateToSQL(a.Column, q)
				if err != nil {
					return "", err
				}
				preds = append(preds, pred)
				if ctx != nil {
					ctx.filterMods = append(ctx.filterMods, pred)
				}
			}
		}
		if len(preds) == 0 {
			return base, nil
		}
		return rewriteAggsWithFilter(base, strings.Join(preds, " AND ")), nil
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
	case "DIVIDE":
		if len(e.Args) < 2 {
			return "", fmt.Errorf("DIVIDE requires numerator and denominator")
		}
		a, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		b, err := e.Args[1].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("divide(toFloat64(%s), nullIf(toFloat64(%s), 0))", a, b), nil
	case "IF":
		if len(e.Args) < 3 {
			return "", fmt.Errorf("IF requires condition, then, else")
		}
		cond, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		th, err := e.Args[1].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		el, err := e.Args[2].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("if(%s, %s, %s)", cond, th, el), nil
	case "COALESCE", "IFNULL":
		if len(e.Args) < 2 {
			return "", fmt.Errorf("%s requires two arguments", e.Func)
		}
		parts := make([]string, 0, len(e.Args))
		for _, a := range e.Args {
			s, err := a.toSQLWithContext(q, ctx)
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		return fmt.Sprintf("coalesce(%s)", strings.Join(parts, ", ")), nil
	case "COUNTROWS":
		return "COUNT(*)", nil
	case "MEDIAN":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("MEDIAN requires a column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("quantileExact(0.5)(%s)", col), nil
	case "TOMONTH", "YEARMONTH":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("%s requires a date column", e.Func)
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("formatDateTime(parseDateTimeBestEffortOrNull(toString(%s)), '%%Y-%%m')", col), nil
	case "YEAR":
		if len(e.Args) == 0 {
			return "", fmt.Errorf("YEAR requires a date column")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("toYear(parseDateTimeBestEffortOrNull(toString(%s)))", col), nil
	case "YOY":
		col, d, err := ctx.timeArgs(e, q)
		if err != nil {
			return "", err
		}
		cur, prev := ctx.periodSums(col, d)
		return fmt.Sprintf("divide((%s) - (%s), nullIf(toFloat64(%s), 0)) * 100", cur, prev, prev), nil
	case "MTD", "TOTALMTD":
		col, d, err := ctx.timeArgs(e, q)
		if err != nil {
			return "", err
		}
		asOf := ctx.asOfSQL()
		return fmt.Sprintf("sumIf(toFloat64OrZero(%s), toYYYYMM(%s) = toYYYYMM(%s) AND %s <= %s)", col, d, asOf, d, asOf), nil
	case "YTD", "TOTALYTD":
		col, d, err := ctx.timeArgs(e, q)
		if err != nil {
			return "", err
		}
		asOf := ctx.asOfSQL()
		return fmt.Sprintf("sumIf(toFloat64OrZero(%s), toYear(%s) = toYear(%s) AND %s <= %s)", col, d, asOf, d, asOf), nil
	case "TOTALQTD":
		col, d, err := ctx.timeArgs(e, q)
		if err != nil {
			return "", err
		}
		asOf := ctx.asOfSQL()
		return fmt.Sprintf("sumIf(toFloat64OrZero(%s), toYear(%s) = toYear(%s) AND toQuarter(%s) = toQuarter(%s) AND %s <= %s)", col, d, asOf, d, asOf, d, asOf), nil
	case "SAMEPERIODLASTYEAR":
		col, d, err := ctx.timeArgs(e, q)
		if err != nil {
			return "", err
		}
		_, prev := ctx.periodSums(col, d)
		return prev, nil
	case "DATEADD":
		if len(e.Args) < 3 {
			return "", fmt.Errorf("DATEADD requires column, interval, unit")
		}
		col, err := e.Args[0].toSQLWithContext(q, ctx)
		if err != nil {
			return "", err
		}
		n := strings.TrimSpace(e.Args[1].Column)
		unit := strings.ToLower(strings.Trim(e.Args[2].Column, "'\""))
		d, err := ctx.dateSQL(q)
		if err != nil {
			return "", err
		}
		shift, err := clickhouseDateShift(n, unit)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("sumIf(toFloat64OrZero(%s), %s >= (%s) AND %s < (%s))", col, d, shift.start, d, shift.end), nil
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

func (ctx *evalContext) dateSQL(q func(string) string) (string, error) {
	if ctx == nil || strings.TrimSpace(ctx.TimeColumn) == "" {
		return "", fmt.Errorf("esta função de tempo precisa de uma coluna de data no modelo semântico")
	}
	if !identOK(ctx.TimeColumn) {
		return "", fmt.Errorf("invalid time column")
	}
	return fmt.Sprintf("parseDateTimeBestEffortOrNull(toString(%s))", q(ctx.TimeColumn)), nil
}

func (ctx *evalContext) asOfSQL() string {
	if ctx != nil && dateOnlyLiteral(ctx.RangeEnd) {
		return fmt.Sprintf("(parseDateTimeBestEffort('%s') + INTERVAL 1 DAY)", sqlDateLit(ctx.RangeEnd))
	}
	if ctx != nil && strings.TrimSpace(ctx.RangeEnd) != "" {
		return fmt.Sprintf("parseDateTimeBestEffort('%s')", sqlDateLit(ctx.RangeEnd))
	}
	return "now()"
}

func (ctx *evalContext) timeArgs(e Expr, q func(string) string) (col, dateSQL string, err error) {
	if len(e.Args) == 0 {
		return "", "", fmt.Errorf("%s requires a column", e.Func)
	}
	col, err = e.Args[0].toSQLWithContext(q, ctx)
	if err != nil {
		return "", "", err
	}
	dateSQL, err = ctx.dateSQL(q)
	if err != nil {
		return "", "", err
	}
	return col, dateSQL, nil
}

func (ctx *evalContext) periodSums(col, dateSQL string) (current, previous string) {
	val := fmt.Sprintf("toFloat64OrZero(%s)", col)
	if ctx != nil && dateOnlyLiteral(ctx.RangeStart) && dateOnlyLiteral(ctx.RangeEnd) {
		start := sqlDateLit(ctx.RangeStart)
		endExcl := fmt.Sprintf("(parseDateTimeBestEffort('%s') + INTERVAL 1 DAY)", sqlDateLit(ctx.RangeEnd))
		startLit := fmt.Sprintf("parseDateTimeBestEffort('%s')", start)
		current = fmt.Sprintf("sumIf(%s, %s >= %s AND %s < %s)", val, dateSQL, startLit, dateSQL, endExcl)
		previous = fmt.Sprintf("sumIf(%s, %s >= addYears(%s, -1) AND %s < addYears(%s, -1))", val, dateSQL, startLit, dateSQL, endExcl)
		return current, previous
	}
	current = fmt.Sprintf("sumIf(%s, toYear(%s) = toYear(now()))", val, dateSQL)
	previous = fmt.Sprintf("sumIf(%s, toYear(%s) = toYear(now()) - 1)", val, dateSQL)
	return current, previous
}

func sqlDateLit(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, ";", "")
	s = strings.ReplaceAll(s, "--", "")
	return s
}

func dateOnlyLiteral(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 10 {
		return false
	}
	matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, s)
	return matched
}

type dateShift struct {
	start string
	end   string
}

func clickhouseDateShift(n, unit string) (dateShift, error) {
	if n == "" {
		n = "-1"
	}
	switch unit {
	case "year", "years", "yy", "yyyy":
		return dateShift{start: fmt.Sprintf("addYears(toStartOfYear(now()), %s)", n), end: fmt.Sprintf("addYears(now(), %s)", n)}, nil
	case "month", "months", "mm":
		return dateShift{start: fmt.Sprintf("addMonths(toStartOfMonth(now()), %s)", n), end: fmt.Sprintf("addMonths(now(), %s)", n)}, nil
	case "day", "days", "dd":
		return dateShift{start: fmt.Sprintf("addDays(now(), %s)", n), end: "now()"}, nil
	default:
		return dateShift{}, fmt.Errorf("DATEADD unit %s not supported (use year, month or day)", unit)
	}
}

func rewriteAggsWithFilter(sql, pred string) string {
	sql = strings.TrimSpace(sql)
	if pred == "" {
		return sql
	}
	type repl struct {
		name string
		ifn  string
	}
	fns := []repl{
		{name: "uniqExact", ifn: "uniqExactIf"},
		{name: "COUNT", ifn: "countIf"},
		{name: "SUM", ifn: "sumIf"},
		{name: "AVG", ifn: "avgIf"},
		{name: "MIN", ifn: "minIf"},
		{name: "MAX", ifn: "maxIf"},
	}
	out := sql
	for _, fn := range fns {
		out = replaceAggCalls(out, fn.name, fn.ifn, pred)
	}
	return out
}

func replaceAggCalls(sql, name, ifn, pred string) string {
	var b strings.Builder
	i := 0
	upper := strings.ToUpper(sql)
	uname := strings.ToUpper(name)
	for i < len(sql) {
		idx := strings.Index(upper[i:], uname)
		if idx < 0 {
			b.WriteString(sql[i:])
			break
		}
		idx += i
		if idx > 0 && isIdentChar(sql[idx-1]) {
			b.WriteString(sql[i : idx+len(name)])
			i = idx + len(name)
			continue
		}
		j := idx + len(name)
		for j < len(sql) && (sql[j] == ' ' || sql[j] == '\t') {
			j++
		}
		if j >= len(sql) || sql[j] != '(' {
			b.WriteString(sql[i : idx+len(name)])
			i = idx + len(name)
			continue
		}
		closeAt, args := matchingParen(sql, j)
		if closeAt < 0 {
			b.WriteString(sql[i:])
			break
		}
		b.WriteString(sql[i:idx])
		trimmed := strings.TrimSpace(args)
		if strings.EqualFold(name, "COUNT") && (trimmed == "*" || trimmed == "") {
			b.WriteString(ifn)
			b.WriteByte('(')
			b.WriteString(pred)
			b.WriteByte(')')
		} else {
			b.WriteString(ifn)
			b.WriteByte('(')
			b.WriteString(args)
			b.WriteString(", ")
			b.WriteString(pred)
			b.WriteByte(')')
		}
		i = closeAt + 1
	}
	return b.String()
}

func matchingParen(s string, open int) (close int, inside string) {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i, s[open+1 : i]
			}
		}
	}
	return -1, ""
}
