package aiagent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thedobra/thedobra/services/api/internal/semantic"
	"github.com/thedobra/thedobra/services/api/internal/semanticxpr"
)

// validateExpressionAgainstModel guarantees that an AI expression only uses
// fields exposed by the selected semantic model.
func validateExpressionAgainstModel(expression string, model semantic.Model, allowMeasures bool) ([]string, error) {
	parsed, err := semanticxpr.Parse(expression)
	if err != nil {
		return nil, fmt.Errorf("expressão inválida: %w", err)
	}

	columns := map[string]string{}
	measures := map[string]string{}
	for _, dimension := range model.Dimensions {
		if column := strings.TrimSpace(dimension.Column); column != "" {
			columns[normalizeSemanticName(column)] = column
		}
	}
	for _, measure := range model.Measures {
		if column := strings.TrimSpace(measure.Column); column != "" && column != "*" {
			columns[normalizeSemanticName(column)] = column
		}
		if name := strings.TrimSpace(measure.Name); name != "" {
			measures[normalizeSemanticName(name)] = name
		}
	}
	if timeColumn := strings.TrimSpace(model.TimeColumn); timeColumn != "" {
		columns[normalizeSemanticName(timeColumn)] = timeColumn
	}

	used := map[string]bool{}
	var walk func(semanticxpr.Expr) error
	walk = func(expr semanticxpr.Expr) error {
		switch strings.ToUpper(expr.Func) {
		case "COLUMN":
			ref := strings.TrimSpace(expr.Column)
			if ref != "" && ref != "*" {
				actual, ok := columns[normalizeSemanticName(lastReferencePart(ref))]
				if !ok {
					return fmt.Errorf("a coluna %q não existe no modelo semântico", ref)
				}
				used[actual] = true
			}
		case "MEASURE":
			ref := strings.TrimSpace(expr.Column)
			actual, ok := measures[normalizeSemanticName(ref)]
			if !allowMeasures || !ok {
				return fmt.Errorf("a medida %q não existe no modelo semântico", ref)
			}
			used[actual] = true
		}
		if expr.Left != nil {
			if err := walk(*expr.Left); err != nil {
				return err
			}
		}
		if expr.Right != nil {
			if err := walk(*expr.Right); err != nil {
				return err
			}
		}
		for _, arg := range expr.Args {
			if err := walk(arg); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(parsed); err != nil {
		return nil, err
	}

	references := make([]string, 0, len(used))
	for field := range used {
		references = append(references, field)
	}
	sort.Strings(references)
	return references, nil
}

func lastReferencePart(ref string) string {
	if idx := strings.LastIndex(ref, "["); idx >= 0 && strings.HasSuffix(ref, "]") {
		return strings.TrimSuffix(ref[idx+1:], "]")
	}
	if idx := strings.LastIndex(ref, "."); idx >= 0 {
		return ref[idx+1:]
	}
	return ref
}

func normalizeSemanticName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c", "-", "_", " ", "_",
	).Replace(value)
	return value
}
