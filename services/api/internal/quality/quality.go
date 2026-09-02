package quality

import (
	"fmt"
	"math"
	"strings"

	"github.com/thedobra/thedobra/services/api/internal/schemax"
)

type Report struct {
	Score      float64      `json:"score"`
	RowCount   int          `json:"row_count"`
	NullPct    float64      `json:"null_pct"`
	DupPct     float64      `json:"duplicate_pct"`
	InvalidPct float64      `json:"invalid_pct"`
	Issues     []Issue      `json:"issues"`
	Columns    []ColumnStat `json:"columns"`
}

type Issue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Column  string `json:"column,omitempty"`
}

type ColumnStat struct {
	Name        string  `json:"name"`
	NullPct     float64 `json:"null_pct"`
	Cardinality int     `json:"cardinality"`
}

func Analyze(cols []schemax.Column, rows [][]string) Report {
	n := len(rows)
	rep := Report{RowCount: n, Columns: make([]ColumnStat, len(cols))}
	if n == 0 {
		rep.Score = 0
		rep.Issues = []Issue{{Code: "empty", Message: "Dataset has no rows"}}
		return rep
	}

	nullCells := 0
	invalid := 0
	seen := map[string]int{}
	dupRows := 0
	uniques := make([]map[string]struct{}, len(cols))
	nulls := make([]int, len(cols))
	for i := range cols {
		uniques[i] = map[string]struct{}{}
		rep.Columns[i].Name = cols[i].Name
	}

	for _, row := range rows {
		key := strings.Join(row, "\x1f")
		seen[key]++
		if seen[key] == 2 {
			dupRows++
		}
		for i := range cols {
			var v string
			if i < len(row) {
				v = strings.TrimSpace(row[i])
			}
			if v == "" {
				nulls[i]++
				nullCells++
				continue
			}
			uniques[i][v] = struct{}{}
			if cols[i].Type != schemax.TypeString && schemax.ParseValue(cols[i].Type, v) == nil {
				invalid++
			}
		}
	}

	totalCells := n * max(len(cols), 1)
	rep.NullPct = pct(nullCells, totalCells)
	rep.InvalidPct = pct(invalid, totalCells)
	rep.DupPct = pct(dupRows, n)

	for i := range cols {
		card := len(uniques[i])
		rep.Columns[i].Cardinality = card
		rep.Columns[i].NullPct = pct(nulls[i], n)
		cols[i].Cardinality = card
		cols[i].Nullable = nulls[i] > 0
		if rep.Columns[i].NullPct > 20 {
			rep.Issues = append(rep.Issues, Issue{
				Code:    "high_nulls",
				Column:  cols[i].Name,
				Message: cols[i].Name + " has high null rate",
			})
		}
	}

	score := 100.0
	score -= math.Min(40, rep.NullPct*1.2)
	score -= math.Min(25, rep.DupPct*2)
	score -= math.Min(20, rep.InvalidPct*4)
	if n < 10 {
		score -= 10
	}
	rep.Score = math.Round(math.Max(0, score)*100) / 100

	if rep.NullPct > 0 {
		rep.Issues = append(rep.Issues, Issue{Code: "nulls", Message: fmt.Sprintf("%.2f%% null values", rep.NullPct)})
	}
	if rep.DupPct > 0 {
		rep.Issues = append(rep.Issues, Issue{Code: "duplicates", Message: fmt.Sprintf("%.2f%% duplicate rows", rep.DupPct)})
	}
	if rep.InvalidPct > 0 {
		rep.Issues = append(rep.Issues, Issue{Code: "invalid", Message: fmt.Sprintf("%.2f%% invalid values", rep.InvalidPct)})
	}
	return rep
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return math.Round(float64(a)*10000/float64(b)) / 100
}
