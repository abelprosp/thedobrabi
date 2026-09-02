package ingest

import (
	"fmt"
	"strings"
	"unicode"
)

// SourceSelection is an optional subset of tables, columns and joins.
// Absent selection keeps the previous sync behaviour (table/query on the config).
type SourceSelection struct {
	Tables []SelectedTable `json:"tables,omitempty"`
	Joins  []SelectedJoin  `json:"joins,omitempty"`
}

type SelectedTable struct {
	Schema  string   `json:"schema,omitempty"`
	Name    string   `json:"name"`
	Columns []string `json:"columns,omitempty"`
}

type SelectedJoin struct {
	LeftTable   string `json:"left_table"`
	LeftColumn  string `json:"left_column"`
	RightTable  string `json:"right_table"`
	RightColumn string `json:"right_column"`
	// Match is "both" (INNER) or "all_left" (LEFT). Stored technically; the UI uses human language.
	Match string `json:"match,omitempty"`
}

func (s SourceSelection) empty() bool {
	return len(s.Tables) == 0
}

func tableKey(schema, name string) string {
	name = strings.TrimSpace(name)
	schema = strings.TrimSpace(schema)
	if schema == "" {
		return name
	}
	return schema + "." + name
}

func (t SelectedTable) Key() string {
	return tableKey(t.Schema, t.Name)
}

func splitTableKey(full string) (schema, name string) {
	full = strings.TrimSpace(full)
	if i := strings.LastIndex(full, "."); i > 0 {
		return full[:i], full[i+1:]
	}
	return "", full
}

func identOK(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 128 {
		return false
	}
	for i, r := range s {
		if r == '_' || r == '$' {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		if r == '.' && i > 0 && i < len(s)-1 {
			continue
		}
		return false
	}
	return true
}

func identPartOK(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 128 || strings.Contains(s, ".") {
		return false
	}
	return identOK(s)
}

func quoteIdent(typ, name string) string {
	name = strings.TrimSpace(name)
	switch typ {
	case "mysql", "mariadb":
		return "`" + strings.ReplaceAll(name, "`", "") + "`"
	case "sqlserver":
		return "[" + strings.ReplaceAll(name, "]", "") + "]"
	default:
		return `"` + strings.ReplaceAll(name, `"`, "") + `"`
	}
}

func quoteTableSQL(typ, schema, name string) string {
	if schema == "" {
		return quoteIdent(typ, name)
	}
	return quoteIdent(typ, schema) + "." + quoteIdent(typ, name)
}

func tableAlias(schema, name string) string {
	if schema == "" || strings.EqualFold(schema, "public") || strings.EqualFold(schema, "dbo") {
		return name
	}
	return schema + "_" + name
}

func sqlAlias(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		out = "col"
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func joinKindSQL(match string) string {
	if strings.EqualFold(strings.TrimSpace(match), "all_left") || strings.EqualFold(match, "left") {
		return "LEFT JOIN"
	}
	return "INNER JOIN"
}

func (s SourceSelection) datasetName() string {
	var parts []string
	for _, t := range s.Tables {
		parts = append(parts, HumanizeIdent(t.Name))
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 2 {
		return parts[0] + " + " + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " e " + parts[len(parts)-1]
}

func applySelection(typ string, cfg SQLConfig) (SQLConfig, error) {
	if cfg.Selection == nil || cfg.Selection.empty() {
		return cfg, nil
	}
	q, err := buildSelectionSQL(typ, *cfg.Selection, cfg.RowLimit())
	if err != nil {
		return cfg, err
	}
	cfg.Query = q
	return cfg, nil
}

func buildSelectionSQL(typ string, sel SourceSelection, limit int) (string, error) {
	if len(sel.Tables) == 0 {
		return "", fmt.Errorf("escolha pelo menos uma lista de dados")
	}
	type tbl struct {
		schema, name, alias, key string
		cols                     []string
	}
	seenAlias := map[string]int{}
	tables := make([]tbl, 0, len(sel.Tables))
	byKey := map[string]tbl{}
	for _, t := range sel.Tables {
		if !identPartOK(t.Name) {
			return "", fmt.Errorf("nome de lista inválido")
		}
		if t.Schema != "" && !identPartOK(t.Schema) {
			return "", fmt.Errorf("nome de grupo inválido")
		}
		alias := sqlAlias(tableAlias(t.Schema, t.Name))
		if !identPartOK(alias) {
			return "", fmt.Errorf("nome de lista inválido")
		}
		if n := seenAlias[alias]; n > 0 {
			alias = fmt.Sprintf("%s_%d", alias, n+1)
		}
		seenAlias[alias]++
		cols := make([]string, 0, len(t.Columns))
		for _, c := range t.Columns {
			if !identPartOK(c) {
				return "", fmt.Errorf("nome de campo inválido")
			}
			cols = append(cols, c)
		}
		item := tbl{schema: t.Schema, name: t.Name, alias: alias, key: t.Key(), cols: cols}
		tables = append(tables, item)
		byKey[item.key] = item
		if t.Schema == "" {
			byKey[item.name] = item
		}
	}

	if len(tables) > 1 && len(sel.Joins) == 0 {
		return "", fmt.Errorf("várias listas sem cruzamento")
	}

	var selectCols []string
	prefixed := len(tables) > 1
	usedAlias := map[string]int{}
	for _, t := range tables {
		if len(t.cols) == 0 {
			if prefixed {
				selectCols = append(selectCols, quoteIdent(typ, t.alias)+".*")
			} else {
				selectCols = append(selectCols, "*")
			}
			continue
		}
		for _, c := range t.cols {
			expr := quoteIdent(typ, t.alias) + "." + quoteIdent(typ, c)
			as := c
			if prefixed {
				as = sqlAlias(t.name + "_" + c)
			}
			if n := usedAlias[as]; n > 0 {
				as = fmt.Sprintf("%s_%d", as, n+1)
			}
			usedAlias[as]++
			selectCols = append(selectCols, expr+" AS "+quoteIdent(typ, as))
		}
	}
	if len(selectCols) == 0 {
		return "", fmt.Errorf("escolha pelo menos um campo")
	}

	from := quoteTableSQL(typ, tables[0].schema, tables[0].name) + " AS " + quoteIdent(typ, tables[0].alias)
	joined := map[string]bool{tables[0].key: true}
	var joinSQL []string
	pending := append([]SelectedJoin(nil), sel.Joins...)
	for len(pending) > 0 {
		progress := false
		next := pending[:0]
		for _, j := range pending {
			if !identOK(j.LeftTable) || !identOK(j.RightTable) || !identPartOK(j.LeftColumn) || !identPartOK(j.RightColumn) {
				return "", fmt.Errorf("cruzamento inválido")
			}
			left, okL := byKey[j.LeftTable]
			right, okR := byKey[j.RightTable]
			if !okL || !okR {
				ls, ln := splitTableKey(j.LeftTable)
				rs, rn := splitTableKey(j.RightTable)
				if !okL {
					left, okL = byKey[tableKey(ls, ln)]
				}
				if !okR {
					right, okR = byKey[tableKey(rs, rn)]
				}
			}
			if !okL || !okR {
				return "", fmt.Errorf("cruzamento refere uma lista que não foi escolhida")
			}
			leftIn, rightIn := joined[left.key], joined[right.key]
			if leftIn && rightIn {
				progress = true
				continue
			}
			if !leftIn && !rightIn {
				next = append(next, j)
				continue
			}
			var add tbl
			var onLeft, onRight tbl
			if leftIn {
				onLeft, onRight, add = left, right, right
			} else {
				onLeft, onRight, add = right, left, left
			}
			joinSQL = append(joinSQL, fmt.Sprintf("%s %s AS %s ON %s.%s = %s.%s",
				joinKindSQL(j.Match),
				quoteTableSQL(typ, add.schema, add.name),
				quoteIdent(typ, add.alias),
				quoteIdent(typ, onLeft.alias), quoteIdent(typ, ternary(leftIn, j.LeftColumn, j.RightColumn)),
				quoteIdent(typ, onRight.alias), quoteIdent(typ, ternary(leftIn, j.RightColumn, j.LeftColumn)),
			))
			joined[add.key] = true
			progress = true
		}
		if !progress {
			return "", fmt.Errorf("não foi possível ligar todas as listas — verifique os cruzamentos")
		}
		pending = next
	}
	if len(tables) > 1 {
		for _, t := range tables {
			if !joined[t.key] {
				return "", fmt.Errorf("a lista %s ficou de fora do cruzamento", HumanizeIdent(t.name))
			}
		}
	}

	q := "SELECT " + strings.Join(selectCols, ", ") + " FROM " + from
	if len(joinSQL) > 0 {
		q += " " + strings.Join(joinSQL, " ")
	}
	if limit <= 0 {
		limit = 10000
	}
	switch typ {
	case "sqlserver":
		// TOP must sit after SELECT. Rebuild.
		q = "SELECT TOP (" + fmt.Sprintf("%d", limit) + ") " + strings.Join(selectCols, ", ") + " FROM " + from
		if len(joinSQL) > 0 {
			q += " " + strings.Join(joinSQL, " ")
		}
	default:
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	return q, nil
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func joinInMemory(leftHeaders []string, leftRows [][]string, leftCol string, rightHeaders []string, rightRows [][]string, rightCol string, allLeft bool) ([]string, [][]string, error) {
	li, ri := -1, -1
	for i, h := range leftHeaders {
		if h == leftCol {
			li = i
			break
		}
	}
	for i, h := range rightHeaders {
		if h == rightCol {
			ri = i
			break
		}
	}
	if li < 0 || ri < 0 {
		return nil, nil, fmt.Errorf("campo de cruzamento não encontrado")
	}
	idx := map[string][]int{}
	for i, row := range rightRows {
		if ri >= len(row) {
			continue
		}
		k := row[ri]
		idx[k] = append(idx[k], i)
	}
	headers := append(append([]string{}, leftHeaders...), rightHeaders...)
	var out [][]string
	for _, lrow := range leftRows {
		key := ""
		if li < len(lrow) {
			key = lrow[li]
		}
		matches := idx[key]
		if len(matches) == 0 {
			if allLeft {
				rec := make([]string, len(headers))
				copy(rec, lrow)
				out = append(out, rec)
			}
			continue
		}
		for _, mi := range matches {
			rec := make([]string, 0, len(headers))
			rec = append(rec, padRow(lrow, len(leftHeaders))...)
			rec = append(rec, padRow(rightRows[mi], len(rightHeaders))...)
			out = append(out, rec)
		}
	}
	return headers, out, nil
}

func padRow(row []string, n int) []string {
	out := make([]string, n)
	copy(out, row)
	return out
}

var humanWords = map[string]string{
	"customer": "Cliente", "customers": "Clientes", "cliente": "Cliente", "clientes": "Clientes",
	"order": "Pedido", "orders": "Pedidos", "pedido": "Pedido", "pedidos": "Pedidos",
	"sale": "Venda", "sales": "Vendas", "venda": "Venda", "vendas": "Vendas",
	"product": "Produto", "products": "Produtos", "produto": "Produto", "produtos": "Produtos",
	"user": "Utilizador", "users": "Utilizadores", "usuario": "Utilizador", "usuarios": "Utilizadores",
	"invoice": "Fatura", "invoices": "Faturas", "fatura": "Fatura", "faturas": "Faturas",
	"payment": "Pagamento", "payments": "Pagamentos", "pagamento": "Pagamento", "pagamentos": "Pagamentos",
	"item": "Item", "items": "Itens", "itens": "Itens",
	"category": "Categoria", "categories": "Categorias", "categoria": "Categoria", "categorias": "Categorias",
	"address": "Morada", "addresses": "Moradas", "endereco": "Morada", "enderecos": "Moradas", "morada": "Morada",
	"employee": "Colaborador", "employees": "Colaboradores",
	"supplier": "Fornecedor", "suppliers": "Fornecedores", "fornecedor": "Fornecedor", "fornecedores": "Fornecedores",
	"stock": "Stock", "inventory": "Inventário", "inventario": "Inventário",
	"transaction": "Transacção", "transactions": "Transacções", "transacao": "Transacção",
	"account": "Conta", "accounts": "Contas", "conta": "Conta", "contas": "Contas",
	"company": "Empresa", "companies": "Empresas", "empresa": "Empresa", "empresas": "Empresas",
	"contact": "Contacto", "contacts": "Contactos", "contacto": "Contacto", "contactos": "Contactos",
	"lead": "Lead", "leads": "Leads",
	"id": "Código", "name": "Nome", "nome": "Nome", "email": "E-mail",
	"phone": "Telefone", "telefone": "Telefone", "mobile": "Telemóvel",
	"created": "Criado", "updated": "Actualizado", "created_at": "Criado em", "updated_at": "Actualizado em",
	"total": "Total", "amount": "Valor", "valor": "Valor", "price": "Preço", "preco": "Preço",
	"qty": "Quantidade", "quantity": "Quantidade", "quantidade": "Quantidade",
	"date": "Data", "data": "Data", "status": "Estado", "estado": "Estado",
	"description": "Descrição", "descricao": "Descrição", "title": "Título", "titulo": "Título",
	"type": "Tipo", "tipo": "Tipo", "city": "Cidade", "cidade": "Cidade",
	"country": "País", "pais": "País", "zip": "Código postal",
	"at": "em",
}

var dropPrefixes = []string{"fct_", "fact_", "dim_", "stg_", "raw_", "vw_", "tb_", "tbl_"}

// HumanizeIdent turns schema.fct_sales / customer_id into a readable Portuguese label.
func HumanizeIdent(raw string) string {
	s := strings.TrimSpace(raw)
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	if s == "" {
		return raw
	}
	lower := strings.ToLower(s)
	for _, p := range dropPrefixes {
		if strings.HasPrefix(lower, p) {
			s = s[len(p):]
			lower = strings.ToLower(s)
			break
		}
	}
	if label, ok := humanWords[lower]; ok {
		return label
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	if len(parts) == 0 {
		return s
	}
	out := make([]string, 0, len(parts))
	for i, p := range parts {
		lp := strings.ToLower(p)
		if mapped, ok := humanWords[lp]; ok {
			if i > 0 {
				out = append(out, strings.ToLower(mapped))
			} else {
				out = append(out, mapped)
			}
			continue
		}
		out = append(out, titleWord(p))
	}
	return strings.Join(out, " ")
}

func titleWord(s string) string {
	s = strings.ToLower(s)
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func GuidedSQLType(typ string) bool {
	switch typ {
	case "postgres", "mysql", "mariadb", "sqlserver", "supabase":
		return true
	default:
		return false
	}
}
