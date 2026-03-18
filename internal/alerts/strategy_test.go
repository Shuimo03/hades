package alerts

import (
	"strings"
	"testing"
	"time"

	"hades/internal/longbridge"
)

func TestDeriveStrategyPlanSwitchesToBreakoutWhenOldPullbackIsInvalid(t *testing.T) {
	plan := deriveStrategyPlan(
		"AMD.US",
		124.00,
		"bullish",
		72,
		planSnapshot{
			LastClose:   121.00,
			MA10:        119.00,
			MA20:        116.50,
			MA60:        109.00,
			Support:     113.00,
			Resistance:  120.00,
			Score:       72,
			VolumeRatio: 1.8,
		},
		planSnapshot{
			LastClose:   118.00,
			MA10:        115.00,
			MA20:        111.00,
			MA60:        103.00,
			Support:     108.00,
			Resistance:  122.00,
			Score:       68,
			VolumeRatio: 1.2,
		},
		false,
		0,
		nil,
	)

	if plan.Mode != planModeBreakout {
		t.Fatalf("expected breakout mode, got %s", plan.Mode)
	}
	if !plan.OldPlanInvalidated {
		t.Fatalf("expected old pullback plan to be invalidated")
	}
	if !strings.Contains(plan.EntryCondition, "突破确认") {
		t.Fatalf("expected breakout entry condition, got %q", plan.EntryCondition)
	}
}

func TestDeriveStrategyPlanUsesEventModeWhenNewsKeywordsPresent(t *testing.T) {
	publishedAt := time.Now().Add(-6 * time.Hour).Format(time.RFC3339)
	plan := deriveStrategyPlan(
		"NVDA.US",
		140.00,
		"bullish",
		76,
		planSnapshot{
			LastClose:   138.00,
			MA10:        136.00,
			MA20:        133.00,
			MA60:        124.00,
			Support:     130.00,
			Resistance:  141.00,
			Score:       76,
			VolumeRatio: 1.6,
		},
		planSnapshot{
			LastClose:   134.00,
			MA10:        131.00,
			MA20:        128.00,
			MA60:        118.00,
			Support:     125.00,
			Resistance:  144.00,
			Score:       70,
			VolumeRatio: 1.1,
		},
		false,
		0,
		[]*longbridge.StockNewsItem{
			{Title: "Nvidia GTC keynote expected to drive AI announcements", PublishedAt: publishedAt},
		},
	)

	if plan.Mode != planModeEvent {
		t.Fatalf("expected event mode, got %s", plan.Mode)
	}
	if !plan.EventRisk {
		t.Fatalf("expected event risk to be true")
	}
	if !strings.Contains(plan.Conclusion, "事件窗口") {
		t.Fatalf("expected event conclusion, got %q", plan.Conclusion)
	}
}

func TestDeriveStrategyPlanRejectsLowRRPullback(t *testing.T) {
	plan := deriveStrategyPlan(
		"1810.HK",
		35.20,
		"bullish",
		66,
		planSnapshot{
			LastClose:   35.10,
			MA10:        34.90,
			MA20:        34.70,
			MA60:        33.80,
			Support:     34.80,
			Resistance:  37.20,
			Score:       66,
			VolumeRatio: 0.9,
		},
		planSnapshot{
			LastClose:   34.80,
			MA10:        34.30,
			MA20:        33.90,
			MA60:        32.70,
			Support:     33.80,
			Resistance:  37.60,
			Score:       62,
			VolumeRatio: 0.8,
		},
		false,
		0,
		nil,
	)

	if plan.Mode != planModePullback {
		t.Fatalf("expected pullback mode, got %s", plan.Mode)
	}
	if plan.RRQualified {
		t.Fatalf("expected RR to be unqualified")
	}
	if plan.Status != "observe" {
		t.Fatalf("expected observe status, got %s", plan.Status)
	}
}
