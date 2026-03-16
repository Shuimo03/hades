package planlevels

func BuyZone(trend string, currentPrice, dailySupport, weeklySupport float64) (float64, float64) {
	support := supportAnchor(trend, currentPrice, dailySupport, weeklySupport)
	if support == 0 {
		return round2(currentPrice * 0.98), round2(currentPrice * 1.01)
	}

	switch trend {
	case "bullish":
		return round2(support * 0.995), round2(support * 1.02)
	case "neutral":
		return round2(support * 0.99), round2(support * 1.01)
	default:
		return round2(support * 0.97), round2(support * 0.99)
	}
}

func StopLoss(trend string, currentPrice, dailySupport, weeklySupport float64) float64 {
	support := supportAnchor(trend, currentPrice, dailySupport, weeklySupport)
	if support == 0 {
		return round2(currentPrice * 0.95)
	}
	return round2(support * 0.97)
}

func TakeProfit(currentPrice, dailyResistance, weeklyResistance float64) float64 {
	resistance := resistanceAnchor(currentPrice, dailyResistance, weeklyResistance)
	if resistance == 0 {
		return round2(currentPrice * 1.08)
	}
	return round2(resistance * 0.99)
}

func supportAnchor(trend string, currentPrice float64, supports ...float64) float64 {
	if currentPrice <= 0 {
		return 0
	}

	best := 0.0
	for _, support := range supports {
		if support <= 0 || support > currentPrice {
			continue
		}
		if support > best {
			best = support
		}
	}

	floor := currentPrice * (1 - maxPullbackPct(trend))
	switch {
	case best == 0:
		best = floor
	case best < floor:
		best = floor
	}
	return best
}

func resistanceAnchor(currentPrice float64, resistances ...float64) float64 {
	if currentPrice <= 0 {
		return 0
	}

	best := 0.0
	for _, resistance := range resistances {
		if resistance <= currentPrice {
			continue
		}
		if best == 0 || resistance < best {
			best = resistance
		}
	}

	cap := currentPrice * 1.18
	switch {
	case best == 0:
		best = currentPrice * 1.08
	case best > cap:
		best = cap
	}
	return best
}

func maxPullbackPct(trend string) float64 {
	switch trend {
	case "bullish":
		return 0.08
	case "neutral":
		return 0.12
	default:
		return 0.15
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
