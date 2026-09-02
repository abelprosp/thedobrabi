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

	if !hasMeasure(m, "orders") && m.TimeColumn != "" {
		m.Measures = append([]Measure{{
			Name: "Orders", Column: "*", Aggregation: "count", Expression: "COUNT(*)", Format: "number", Description: "Row count",
		}}, m.Measures...)
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
			Name: "Linhas", Column: "*", Aggregation: "count", Expression: "COUNT(*)", Format: "number",
			Description: "Contagem de linhas",
		})
	}
	if !hasMeasure(*m, "linhas") && !hasMeasure(*m, "count") && !hasMeasure(*m, "orders") {
		m.Measures = append([]Measure{{
			Name: "Linhas", Column: "*", Aggregation: "count", Expression: "COUNT(*)", Format: "number",
			Description: "Contagem de linhas",
		}}, m.Measures...)
	}
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
