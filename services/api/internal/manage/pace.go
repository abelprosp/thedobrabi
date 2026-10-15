package manage

import (
	"fmt"
	"math"
	"strings"
	"time"

	_ "time/tzdata"
)

func locBR() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.UTC
	}
	return loc
}

func nowBR() time.Time {
	return time.Now().In(locBR())
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.ParseInLocation("2006-01-02", s, locBR()); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.In(locBR()), true
	}
	return time.Time{}, false
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(locBR()).Format("2006-01-02")
}

func isoWeekPeriod(t time.Time) string {
	t = t.In(locBR())
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

// BusinessDaysRemaining counts Mon–Fri from from (inclusive) to to (inclusive).
func BusinessDaysRemaining(from, to time.Time) int {
	from = dateOnly(from)
	to = dateOnly(to)
	if to.Before(from) {
		return 0
	}
	n := 0
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		w := d.Weekday()
		if w != time.Saturday && w != time.Sunday {
			n++
		}
	}
	return n
}

func dateOnly(t time.Time) time.Time {
	t = t.In(locBR())
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, locBR())
}

func periodBounds(now time.Time, period string) (start, end time.Time) {
	now = dateOnly(now)
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "daily":
		return now, now
	case "weekly":
		wd := int(now.Weekday())
		if wd == 0 {
			wd = 7
		}
		start = now.AddDate(0, 0, -(wd - 1))
		return start, start.AddDate(0, 0, 6)
	case "quarterly":
		q := ((int(now.Month()) - 1) / 3) * 3
		start = time.Date(now.Year(), time.Month(q+1), 1, 12, 0, 0, 0, locBR())
		end = start.AddDate(0, 3, -1)
		return start, end
	case "yearly":
		start = time.Date(now.Year(), 1, 1, 12, 0, 0, 0, locBR())
		end = time.Date(now.Year(), 12, 31, 12, 0, 0, 0, locBR())
		return start, end
	default: // monthly
		start = time.Date(now.Year(), now.Month(), 1, 12, 0, 0, 0, locBR())
		end = start.AddDate(0, 1, -1)
		return start, end
	}
}

func cadenceWindow(cadence string) time.Duration {
	switch cadence {
	case "daily":
		return 26 * time.Hour
	case "monthly":
		return 32 * 24 * time.Hour
	default:
		return 8 * 24 * time.Hour
	}
}

func suggestCadence(updatedAt time.Time, hasTime bool) string {
	if updatedAt.IsZero() {
		return "weekly"
	}
	age := time.Since(updatedAt)
	if age > 20*24*time.Hour {
		return "monthly"
	}
	if hasTime && age < 48*time.Hour {
		return "daily"
	}
	return "weekly"
}

func formatNum(v float64) string {
	if math.Abs(v) >= 1000 {
		return strings.ReplaceAll(fmt.Sprintf("%.0f", v), ".", "")
	}
	if math.Abs(v-math.Round(v)) < 0.05 {
		return fmt.Sprintf("%.0f", v)
	}
	return strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1)
}

func moneyish(format, name string) bool {
	s := strings.ToLower(format + " " + name)
	return strings.Contains(s, "currenc") ||
		strings.Contains(s, "brl") ||
		strings.Contains(s, "receita") ||
		strings.Contains(s, "fatur") ||
		strings.Contains(s, "valor") ||
		strings.Contains(s, "preco") ||
		strings.Contains(s, "preço") ||
		strings.Contains(s, "despesa") ||
		strings.Contains(s, "r$")
}

func paceNarrative(current, target float64, days int, currency bool) (pct, remaining, daily float64, reached bool, text, hypo string) {
	if target == 0 {
		return 0, 0, 0, false, "A meta ainda não tem um valor-alvo.", ""
	}
	pct = current / math.Abs(target) * 100
	remaining = target - current
	reached = remaining <= 0
	prefix := ""
	if currency {
		prefix = "R$ "
	}
	if reached {
		text = fmt.Sprintf("Você já ultrapassou a meta (%.0f%%). O resultado actual é %s%s.", math.Min(pct, 999), prefix, formatNum(current))
		return pct, remaining, 0, true, text, ""
	}
	if days <= 0 {
		text = fmt.Sprintf("Você atingiu %.0f%% da meta. O período acabou e ainda faltam %s%s.", pct, prefix, formatNum(remaining))
		hypo = "Hipótese: sem dias úteis restantes, o objectivo deste período já não é recuperável só com o ritmo actual."
		return pct, remaining, remaining, false, text, hypo
	}
	daily = remaining / float64(days)
	text = fmt.Sprintf("Você atingiu %.0f%% da meta. Para alcançar o objectivo, precisa de uma média de %s%s por dia útil restante.", pct, prefix, formatNum(daily))
	hypo = "Hipótese: se o ritmo recente se repetir e nada mudar no mix, a meta fica em risco."
	if pct >= 80 {
		hypo = "Hipótese: o ritmo actual chega perto da meta; uma variação pequena no fecho do período pode decidir o resultado."
	}
	return pct, remaining, daily, false, text, hypo
}
