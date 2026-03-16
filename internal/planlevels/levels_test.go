package planlevels

import "testing"

func TestBuyZoneCapsDeepSupportForBullishTrend(t *testing.T) {
	low, high := BuyZone("bullish", 116, 66, 65)
	if low < 100 || high < 100 {
		t.Fatalf("buy zone too far from current price: low=%.2f high=%.2f", low, high)
	}
	if high >= 116 {
		t.Fatalf("buy zone should stay below current price, got high=%.2f", high)
	}
}

func TestBuyZoneKeepsReasonableSupportWhenNearCurrent(t *testing.T) {
	low, high := BuyZone("bearish", 33.32, 30.58, 30.26)
	if low > 31 || high > 31.5 {
		t.Fatalf("buy zone should stay anchored near nearby support: low=%.2f high=%.2f", low, high)
	}
}

func TestTakeProfitUsesNearestResistance(t *testing.T) {
	target := TakeProfit(333.61, 353.14, 414.61)
	if target >= 400 {
		t.Fatalf("take profit should use nearer resistance, got %.2f", target)
	}
}
