package schemax

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ColType string

const (
	TypeString   ColType = "string"
	TypeInt      ColType = "int"
	TypeFloat    ColType = "float"
	TypeBool     ColType = "bool"
	TypeDate     ColType = "date"
	TypeDateTime ColType = "datetime"
)

type Column struct {
	Name        string  `json:"name"`
	SourceName  string  `json:"source_name"`
	Type        ColType `json:"type"`
	Nullable    bool    `json:"nullable"`
	Cardinality int     `json:"cardinality,omitempty"`
	Role        string  `json:"role,omitempty"` // dimension | measure | time | id
}

func SanitizeIdent(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "col"
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = "c_" + out
	}
	return out
}

func UniqueNames(headers []string) []string {
	used := map[string]int{}
	out := make([]string, len(headers))
	for i, h := range headers {
		base := SanitizeIdent(h)
		if base == "" {
			base = "col"
		}
		n := used[base]
		used[base] = n + 1
		if n == 0 {
			out[i] = base
		} else {
			out[i] = base + "_" + strconv.Itoa(n+1)
		}
	}
	return out
}

var dateLayouts = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02",
	"2006-01-02 15:04:05",
	"2006/01/02",
	"02/01/2006",
	"01/02/2006",
	"02-01-2006",
}

func InferType(samples []string) ColType {
	nonEmpty := 0
	counts := map[ColType]int{}
	for _, s := range samples {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		nonEmpty++
		counts[classify(s)]++
	}
	if nonEmpty == 0 {
		return TypeString
	}
	// ints are a subset of floats
	if (counts[TypeInt]+counts[TypeFloat])*100/nonEmpty >= 85 && counts[TypeFloat] > 0 {
		return TypeFloat
	}
	order := []ColType{TypeBool, TypeInt, TypeFloat, TypeDateTime, TypeDate}
	for _, t := range order {
		if counts[t]*100/nonEmpty >= 85 {
			return t
		}
	}
	return TypeString
}

func classify(s string) ColType {
	ls := strings.ToLower(s)
	if ls == "true" || ls == "false" || ls == "yes" || ls == "no" || ls == "0" && false {
		return TypeBool
	}
	if ls == "true" || ls == "false" || ls == "yes" || ls == "no" {
		return TypeBool
	}
	if _, err := strconv.ParseInt(strings.ReplaceAll(s, ",", ""), 10, 64); err == nil {
		return TypeInt
	}
	if _, err := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(s, ",", ""), " ", ""), 64); err == nil {
		return TypeFloat
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && !strings.Contains(s, "T") && !strings.Contains(s, ":") {
				return TypeDate
			}
			return TypeDateTime
		}
	}
	return TypeString
}

func ParseValue(t ColType, s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	switch t {
	case TypeInt:
		n, err := strconv.ParseInt(strings.ReplaceAll(s, ",", ""), 10, 64)
		if err != nil {
			return nil
		}
		return n
	case TypeFloat:
		f, err := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(s, ",", ""), " ", ""), 64)
		if err != nil {
			return nil
		}
		return f
	case TypeBool:
		ls := strings.ToLower(s)
		return ls == "true" || ls == "yes" || ls == "1"
	case TypeDate, TypeDateTime:
		for _, l := range dateLayouts {
			if tm, err := time.Parse(l, s); err == nil {
				return tm.UTC()
			}
		}
		return nil
	default:
		return s
	}
}

func ClickHouseType(t ColType) string {
	switch t {
	case TypeInt:
		return "Nullable(Int64)"
	case TypeFloat:
		return "Nullable(Float64)"
	case TypeBool:
		return "Nullable(UInt8)"
	case TypeDate:
		return "Nullable(Date)"
	case TypeDateTime:
		return "Nullable(DateTime64(3))"
	default:
		return "Nullable(String)"
	}
}

var (
	idRe     = regexp.MustCompile(`(^|_)(id|uuid|key)$`)
	timeRe   = regexp.MustCompile(`(date|time|ts|timestamp|created|updated|month|year|day)`)
	moneyRe  = regexp.MustCompile(`(revenue|amount|total|price|cost|profit|sales|fee|gmv|arr|mrr)`)
	qtyRe    = regexp.MustCompile(`(qty|quantity|count|units|volume)`)
)

func GuessRole(col Column) string {
	n := strings.ToLower(col.Name)
	if timeRe.MatchString(n) && (col.Type == TypeDate || col.Type == TypeDateTime || col.Type == TypeString) {
		return "time"
	}
	if idRe.MatchString(n) {
		return "id"
	}
	if col.Type == TypeInt || col.Type == TypeFloat {
		if moneyRe.MatchString(n) || qtyRe.MatchString(n) {
			return "measure"
		}
		if col.Cardinality > 0 && col.Cardinality < 50 {
			return "dimension"
		}
		return "measure"
	}
	return "dimension"
}
