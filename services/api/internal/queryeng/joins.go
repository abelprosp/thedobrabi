package queryeng

import (
	"strconv"
	"strings"

	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

func joinAlias(i int) string {
	if i < 0 {
		return "a"
	}
	return string(rune('b' + i))
}

// parseJoinRef maps "join.col" / "join.0.col" / "join.1.col" onto a join index.
func parseJoinRef(name string) (idx int, raw string, isJoin bool) {
	if !strings.HasPrefix(name, "join.") {
		return -1, name, false
	}
	rest := strings.TrimPrefix(name, "join.")
	if i := strings.IndexByte(rest, '.'); i > 0 {
		if n, err := strconv.Atoi(rest[:i]); err == nil && n >= 0 {
			return n, rest[i+1:], true
		}
	}
	return 0, rest, true
}

func sqlOutAlias(requested, fallback string) string {
	name := requested
	if name == "" {
		name = fallback
	}
	idx, raw, isJoin := parseJoinRef(name)
	if !isJoin {
		return alias(name)
	}
	if idx <= 0 {
		return "join_" + alias(raw)
	}
	return "join" + strconv.Itoa(idx) + "_" + alias(raw)
}

func filterClauseQualifiedN(model semantic.Model, joins []datasetInfo, f Filter, qualify func(int, string) string) (string, error) {
	idx, raw, isJoin := parseJoinRef(f.Dimension)
	src := model
	if isJoin {
		if idx < 0 || idx >= len(joins) {
			return "", nil
		}
		src = joins[idx].Model
	}
	clause, err := filterClause(src, Filter{Dimension: raw, Op: f.Op, Value: f.Value})
	if err != nil || clause == "" {
		return clause, err
	}
	d, ok := semantic.ResolveDimension(src, raw)
	col := raw
	if ok {
		col = d.Column
	}
	qIdx := -1
	if isJoin {
		qIdx = idx
	}
	return strings.Replace(clause, "`"+col+"`", qualify(qIdx, col), 1), nil
}
