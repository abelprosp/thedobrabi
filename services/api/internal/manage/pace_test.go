package manage

import (
	"testing"
	"time"
)

func TestBusinessDaysRemaining(t *testing.T) {
	loc := locBR()
	from := time.Date(2026, 10, 15, 12, 0, 0, 0, loc) // Thursday
	to := time.Date(2026, 10, 20, 12, 0, 0, 0, loc)   // Tuesday
	// Thu, Fri, Mon, Tue = 4
	if n := BusinessDaysRemaining(from, to); n != 4 {
		t.Fatalf("got %d want 4", n)
	}
	if n := BusinessDaysRemaining(to, from); n != 0 {
		t.Fatalf("inverted got %d", n)
	}
}

func TestPeriodBoundsMonthly(t *testing.T) {
	now := time.Date(2026, 10, 15, 8, 0, 0, 0, locBR())
	start, end := periodBounds(now, "monthly")
	if start.Day() != 1 || end.Day() != 31 || start.Month() != time.October {
		t.Fatalf("bounds %s %s", start, end)
	}
}

func TestPaceNarrative(t *testing.T) {
	pct, rem, daily, reached, text, hypo := paceNarrative(680, 1000, 10, true)
	if reached || pct < 67 || pct > 69 || rem != 320 || daily != 32 {
		t.Fatalf("pct=%v rem=%v daily=%v reached=%v", pct, rem, daily, reached)
	}
	if text == "" || hypo == "" {
		t.Fatal("expected narrative and hypothesis")
	}
}
