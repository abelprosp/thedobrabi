package scheduler

import (
	"fmt"
	"strings"
	"time"
)

const (
	Freq15m    = "15m"
	FreqHourly = "hourly"
	FreqDaily  = "daily"
	FreqWeekly = "weekly"
)

func ValidFrequency(f string) bool {
	switch f {
	case Freq15m, FreqHourly, FreqDaily, FreqWeekly:
		return true
	}
	return false
}

func ValidKind(k string) bool {
	switch k {
	case "connector", "flow", "dataset":
		return true
	}
	return false
}

func ValidTimezone(tz string) bool {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return true
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

func LoadLocation(tz string) *time.Location {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		tz = "America/Sao_Paulo"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// NextRun returns the next fire time after `from` (typically now) in UTC.
func NextRun(from time.Time, freq, tz string, hour, weekday int) (time.Time, error) {
	if !ValidFrequency(freq) {
		return time.Time{}, fmt.Errorf("frequência inválida")
	}
	if hour < 0 || hour > 23 {
		hour = 6
	}
	if weekday < 0 || weekday > 6 {
		weekday = 1
	}
	loc := LoadLocation(tz)
	now := from.In(loc)
	switch freq {
	case Freq15m:
		t := now.Truncate(15 * time.Minute).Add(15 * time.Minute)
		if !t.After(now) {
			t = t.Add(15 * time.Minute)
		}
		return t.UTC(), nil
	case FreqHourly:
		t := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc).Add(time.Hour)
		if !t.After(now) {
			t = t.Add(time.Hour)
		}
		return t.UTC(), nil
	case FreqDaily:
		t := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, loc)
		if !t.After(now) {
			t = t.AddDate(0, 0, 1)
		}
		return t.UTC(), nil
	case FreqWeekly:
		t := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, loc)
		for int(t.Weekday()) != weekday || !t.After(now) {
			t = t.AddDate(0, 0, 1)
		}
		return t.UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("frequência inválida")
	}
}

func CDCCapable(typ string) bool {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "postgres", "postgresql", "pg", "mysql", "mariadb", "supabase":
		return true
	}
	return false
}
