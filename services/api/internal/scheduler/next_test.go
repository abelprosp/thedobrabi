package scheduler

import (
	"testing"
	"time"
)

func TestNextRun15m(t *testing.T) {
	from := time.Date(2026, 9, 2, 12, 7, 0, 0, time.UTC)
	got, err := NextRun(from, Freq15m, "UTC", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 9, 2, 12, 15, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestNextRunDailySaoPaulo(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skip(err)
	}
	from := time.Date(2026, 9, 2, 10, 0, 0, 0, loc)
	got, err := NextRun(from, FreqDaily, "America/Sao_Paulo", 6, 1)
	if err != nil {
		t.Fatal(err)
	}
	// 10:00 BRT is already past 06:00, so next is 3 Sep 06:00 BRT.
	want := time.Date(2026, 9, 3, 6, 0, 0, 0, loc).UTC()
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestNextRunWeekly(t *testing.T) {
	from := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC) // Wednesday
	got, err := NextRun(from, FreqWeekly, "UTC", 9, int(time.Monday))
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestValidFrequency(t *testing.T) {
	if ValidFrequency("cron") {
		t.Fatal("cron should be invalid")
	}
	if !ValidFrequency("hourly") {
		t.Fatal("hourly")
	}
}

func TestCDCCapable(t *testing.T) {
	if !CDCCapable("postgres") || !CDCCapable("mysql") {
		t.Fatal("sql should be cdc capable")
	}
	if CDCCapable("asaas") {
		t.Fatal("saas should not be cdc capable")
	}
}
