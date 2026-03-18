package tools

import (
	"testing"
	"time"
)

func TestDefaultPeriodStartDailyUsesRolling24Hours(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	now := time.Date(2026, 3, 17, 7, 30, 0, 0, location)
	start := defaultPeriodStart(now, reviewPeriodDaily)

	want := time.Date(2026, 3, 16, 7, 30, 0, 0, location)
	if !start.Equal(want) {
		t.Fatalf("defaultPeriodStart() = %s, want %s", start, want)
	}
}

func TestDefaultPeriodStartWeeklyStillUsesWeekBoundary(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	now := time.Date(2026, 3, 19, 7, 30, 0, 0, location)
	start := defaultPeriodStart(now, reviewPeriodWeekly)

	want := time.Date(2026, 3, 16, 0, 0, 0, 0, location)
	if !start.Equal(want) {
		t.Fatalf("defaultPeriodStart() = %s, want %s", start, want)
	}
}
