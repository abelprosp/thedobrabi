package semantic

import (
	"strings"

	"github.com/thedobra/thedobra/services/api/internal/schemax"
)

type Model struct {
	DatasetID     string         `json:"dataset_id"`
	Name          string         `json:"name"`
	TimeColumn    string         `json:"time_column,omitempty"`
	Dimensions    []Dimension    `json:"dimensions"`
	Measures      []Measure      `json:"measures"`
	Hierarchies   []Hierarchy    `json:"hierarchies,omitempty"`
	Relationships []Relationship `json:"relationships,omitempty"`
	Suggested     bool           `json:"suggested"`
}

type Dimension struct {
	Name        string `json:"name"`
	Column      string `json:"column"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type Measure struct {
	Name        string `json:"name"`
	Column      string `json:"column"`
	Aggregation string `json:"aggregation"` // sum, avg, count, min, max, count_distinct, expression
	Expression  string `json:"expression"`
	Format      string `json:"format,omitempty"`
	Description string `json:"description,omitempty"`
	IsDAX       bool   `json:"is_dax,omitempty"`
}

type Hierarchy struct {
	Name   string   `json:"name"`
	Levels []string `json:"levels"` // ordered column names
}

type Relationship struct {
	FromDataset string `json:"from_dataset"`
	FromColumn  string `json:"from_column"`
	ToDataset   string `json:"to_dataset"`
	ToColumn    string `json:"to_column"`
	Type        string `json:"type"` // one_to_one | many_to_one | one_to_many | many_to_many
}

func Suggest(datasetName string, cols []schemax.Column) Model {
	m := Model{Name: datasetName + " · modelo", Suggested: true}
	usedMeasure := map[string]bool{}

	for i := range cols {
		cols[i].Role = schemax.GuessRole(cols[i])
	}

	for _, c := range cols {
		switch c.Role {
		case "time":
			if m.TimeColumn == "" {
				m.TimeColumn = c.Name
			}
			m.Dimensions = append(m.Dimensions, Dimension{
				Name: title(c.Name), Column: c.Name, Type: string(c.Type), Description: "Dimensão de tempo",
			})
		case "measure":
			agg := "sum"
			format := "number"
			ln := strings.ToLower(c.Name)
			if strings.Contains(ln, "rate") || strings.Contains(ln, "pct") || strings.Contains(ln, "margin") {
				agg = "avg"
				format = "percent"
			}
			if strings.Contains(ln, "price") || strings.Contains(ln, "ticket") {
				agg = "avg"
				format = "currency"
			}
			if strings.Contains(ln, "revenue") || strings.Contains(ln, "amount") || strings.Contains(ln, "total") || strings.Contains(ln, "profit") || strings.Contains(ln, "cost") || strings.Contains(ln, "sales") {
				format = "currency"
			}
			expr := strings.ToUpper(agg) + "(" + c.Name + ")"
			if agg == "count" {
				expr = "COUNT(*)"
			}
			m.Measures = append(m.Measures, Measure{
				Name: title(c.Name), Column: c.Name, Aggregation: agg, Expression: expr, Format: format,
			})
			usedMeasure[c.Name] = true
		case "id":
			m.Dimensions = append(m.Dimensions, Dimension{Name: title(c.Name), Column: c.Name, Type: string(c.Type)})
		default:
			m.Dimensions = append(m.Dimensions, Dimension{Name: title(c.Name), Column: c.Name, Type: string(c.Type)})
		}
	}

	// Common derived metrics when columns exist.
	rev := findMeasureCol(m, "revenue", "total", "amount", "sales")
	qty := findCol(cols, "quantity", "qty", "units")
	if rev != "" && qty != "" && !hasMeasure(m, "average_ticket") {
		m.Measures = append(m.Measures, Measure{
			Name: "Average Ticket", Column: rev, Aggregation: "avg",
			Expression: "SUM(" + rev + ") / NULLIF(COUNT(*),0)", Format: "currency",
			Description: "Revenue per order",
		})
	}
	profit := findCol(cols, "profit")
	if rev != "" && profit != "" && !hasMeasure(m, "margin") {
		m.Measures = append(m.Measures, Measure{
			Name: "Margin", Column: profit, Aggregation: "avg",
			Expression: "SUM(" + profit + ") / NULLIF(SUM(" + rev + "),0)", Format: "percent",
			Description: "Profit / Revenue",
		})
	}

	EnsureBasics(&m, cols)
	return m
}

// Hydrate fills an empty or incomplete semantic model from inferred schema columns.
// Used when a connector dataset was stored without measures (IBGE, CSV, REST).
func Hydrate(m Model, cols []schemax.Column) Model {
	if m.Name == "" {
		m.Name = "modelo"
	}
	EnsureBasics(&m, cols)
	return m
}

// EnsureBasics guarantees at least COUNT(*) and SUM of numeric columns, plus
// string/time dimensions. Connector datasets (IPCA, municípios, pagamentos)
// often have no column named revenue/region.
func EnsureBasics(m *Model, cols []schemax.Column) {
	if m == nil {
		return
	}
	usedMeasure := map[string]bool{}
	for _, meas := range m.Measures {
		usedMeasure[normalize(meas.Column)] = true
		usedMeasure[normalize(meas.Name)] = true
	}
	usedDim := map[string]bool{}
	for _, d := range m.Dimensions {
		usedDim[normalize(d.Column)] = true
		usedDim[normalize(d.Name)] = true
	}

	for _, c := range cols {
		role := c.Role
		if role == "" {
			role = schemax.GuessRole(c)
		}
		switch c.Type {
		case schemax.TypeInt, schemax.TypeFloat:
			if role == "id" || role == "dimension" || role == "time" {
				if !usedDim[normalize(c.Name)] {
					m.Dimensions = append(m.Dimensions, Dimension{Name: title(c.Name), Column: c.Name, Type: string(c.Type)})
					usedDim[normalize(c.Name)] = true
				}
				if role == "time" && m.TimeColumn == "" {
					m.TimeColumn = c.Name
				}
				continue
			}
			if !usedMeasure[normalize(c.Name)] {
				agg := "sum"
				format := "number"
				ln := strings.ToLower(c.Name)
				if strings.Contains(ln, "rate") || strings.Contains(ln, "pct") || strings.Contains(ln, "margin") {
					agg = "avg"
					format = "percent"
				}
				expr := strings.ToUpper(agg) + "(" + c.Name + ")"
				m.Measures = append(m.Measures, Measure{
					Name: title(c.Name), Column: c.Name, Aggregation: agg, Expression: expr, Format: format,
				})
				usedMeasure[normalize(c.Name)] = true
			}
		default:
			if !usedDim[normalize(c.Name)] {
				desc := ""
				if role == "time" && m.TimeColumn == "" {
					m.TimeColumn = c.Name
					desc = "Dimensão de tempo"
				}
				m.Dimensions = append(m.Dimensions, Dimension{
					Name: title(c.Name), Column: c.Name, Type: string(c.Type), Description: desc,
				})
				usedDim[normalize(c.Name)] = true
			}
		}
	}

	if len(m.Measures) == 0 {
		m.Measures = append(m.Measures, Measure{
			Name: "N.º de registos", Column: "*", Aggregation: "count", Expression: "COUNT(*)", Format: "number",
			Description: "Contagem de registos — só quando não há valores numéricos",
		})
	}
	preferRealMeasures(m)
}

func isRowCount(m Measure) bool {
	expr := strings.ToLower(strings.ReplaceAll(m.Expression, " ", ""))
	col := strings.TrimSpace(m.Column)
	return col == "*" || expr == "count(*)" || (strings.EqualFold(m.Aggregation, "count") && (col == "*" || col == ""))
}

// preferRealMeasures drops COUNT(*) named Linhas/Orders when the dataset already has SUM/AVG
// of real columns (ex.: valor). "linha" in a P&L file is a dimension, not a row count.
func preferRealMeasures(m *Model) {
	if m == nil || len(m.Measures) == 0 {
		return
	}
	var real, counts []Measure
	for _, meas := range m.Measures {
		if isRowCount(meas) {
			counts = append(counts, meas)
			continue
		}
		real = append(real, meas)
	}
	if len(real) == 0 {
		if len(counts) > 0 {
			counts[0].Name = "N.º de registos"
			m.Measures = counts[:1]
		}
		return
	}
	m.Measures = append(preferredFirst(real), counts...)
}

func preferredFirst(ms []Measure) []Measure {
	score := func(m Measure) int {
		n := normalize(m.Name + " " + m.Column)
		switch {
		case strings.Contains(n, "valor") || strings.Contains(n, "revenue") || strings.Contains(n, "receita") || strings.Contains(n, "amount") || strings.Contains(n, "montante"):
			return 0
		case strings.Contains(n, "total") || strings.Contains(n, "sales") || strings.Contains(n, "gmv"):
			return 1
		default:
			return 2
		}
	}
	out := append([]Measure{}, ms...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if score(out[j]) < score(out[i]) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func PrimaryMeasure(m Model) string {
	for _, meas := range m.Measures {
		if !isRowCount(meas) {
			return meas.Name
		}
	}
	if len(m.Measures) > 0 {
		return m.Measures[0].Name
	}
	return ""
}

func ResolveMeasure(model Model, name string) (Measure, bool) {
	want := normalize(name)
	for _, m := range model.Measures {
		if normalize(m.Name) == want || normalize(m.Column) == want {
			return m, true
		}
	}
	return Measure{}, false
}

func ResolveDimension(model Model, name string) (Dimension, bool) {
	want := normalize(name)
	for _, d := range model.Dimensions {
		if normalize(d.Name) == want || normalize(d.Column) == want {
			return d, true
		}
	}
	return Dimension{}, false
}

func hasMeasure(m Model, name string) bool {
	_, ok := ResolveMeasure(m, name)
	return ok
}

func findMeasureCol(m Model, names ...string) string {
	for _, n := range names {
		if meas, ok := ResolveMeasure(m, n); ok {
			return meas.Column
		}
	}
	return ""
}

func findCol(cols []schemax.Column, names ...string) string {
	for _, c := range cols {
		for _, n := range names {
			if normalize(c.Name) == n {
				return c.Name
			}
		}
	}
	return ""
}

func title(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	return s
}
