package alerts

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/longportapp/openapi-go/quote"
	"hades/internal/longbridge"
)

func BuildAlertPlanContext(ctx context.Context, lb *longbridge.Client, symbol string, scope longbridge.QuoteSessionScope) string {
	metrics, err := BuildAlertPlanMetrics(ctx, lb, symbol, scope)
	if err != nil {
		return ""
	}

	lines := []string{
		"",
		"交易计划:",
		fmt.Sprintf("1. 当前模式: %s", metrics.ModeLabel),
		fmt.Sprintf("2. 结论: %s", metrics.Conclusion),
		fmt.Sprintf("3. 入场条件: %s", metrics.EntryCondition),
		fmt.Sprintf("4. 失效条件: %s", metrics.InvalidationCondition),
		fmt.Sprintf("5. 目标与 RR: %s", metrics.TargetSummary),
	}
	if details := metrics.PositionContext; strings.TrimSpace(details) != "" {
		lines = append(lines, fmt.Sprintf("附注: %s", details))
	}
	if details := metrics.StatusReason; strings.TrimSpace(details) != "" {
		lines = append(lines, fmt.Sprintf("状态: %s", details))
	}

	return strings.Join(lines, "\n")
}

type AlertPlanMetrics struct {
	CurrentPrice          float64
	CurrentPriceSession   longbridge.QuoteSession
	Trend                 string
	Score                 int
	Mode                  string
	ModeLabel             string
	Status                string
	StatusReason          string
	BuyLow                float64
	BuyHigh               float64
	StopLoss              float64
	TakeProfit            float64
	TakeProfit2           float64
	RR                    float64
	RRQualified           bool
	Conclusion            string
	EntryCondition        string
	InvalidationCondition string
	TargetSummary         string
	BreakoutConfirm       float64
	BreakoutExit          float64
	ChaseLimit            float64
	VolumeRequirement     string
	EventRisk             bool
	EventNotes            []string
	Overextended          bool
	PositionContext       string
	Action                string
	IsHeld                bool
	CostPrice             float64
	Daily                 map[string]interface{}
	Weekly                map[string]interface{}
}

type planSnapshot struct {
	LastClose    float64
	MA10         float64
	MA20         float64
	MA60         float64
	Support      float64
	Resistance   float64
	Score        int
	ChangePct    float64
	VolumeRatio  float64
	PriceAbove20 bool
	PriceAbove60 bool
}

func BuildAlertPlanMetrics(ctx context.Context, lb *longbridge.Client, symbol string, scope longbridge.QuoteSessionScope) (*AlertPlanMetrics, error) {
	if strings.TrimSpace(symbol) == "" {
		return nil, fmt.Errorf("missing symbol")
	}

	tradeSession := longbridge.CandlestickTradeSessionFromScope(scope)
	daily, err := analyzePlanPeriod(ctx, lb, symbol, quote.PeriodDay, 120, tradeSession)
	if err != nil {
		return nil, err
	}
	weekly, err := analyzePlanPeriod(ctx, lb, symbol, quote.PeriodWeek, 60, tradeSession)
	if err != nil {
		return nil, err
	}

	effectiveQuote := longbridge.EffectiveQuote{}
	quotes, err := lb.GetQuote(ctx, []string{symbol})
	if err == nil && len(quotes) > 0 && quotes[0] != nil {
		effectiveQuote = longbridge.ResolveEffectiveQuote(quotes[0], scope)
	}

	currentPrice := daily.LastClose
	if effectiveQuote.HasQuote && effectiveQuote.Price > 0 {
		currentPrice = effectiveQuote.Price
	}

	trend, score := combinePlanTrend(daily, weekly)

	isHeld := false
	costPrice := 0.0
	if positionChannels, err := lb.GetPositions(ctx); err == nil {
		for _, channel := range positionChannels {
			if channel == nil {
				continue
			}
			for _, position := range channel.Positions {
				if position == nil || position.Symbol != symbol || position.CostPrice == nil {
					continue
				}
				isHeld = true
				costPrice, _ = position.CostPrice.Float64()
				break
			}
			if isHeld {
				break
			}
		}
	}

	var news []*longbridge.StockNewsItem
	if items, err := lb.GetStockNews(ctx, symbol); err == nil {
		news = items
	}
	plan := deriveStrategyPlan(symbol, currentPrice, trend, score, daily, weekly, isHeld, costPrice, news)

	return &AlertPlanMetrics{
		CurrentPrice:          round2(currentPrice),
		CurrentPriceSession:   effectiveQuote.Session,
		Trend:                 trend,
		Score:                 score,
		Mode:                  string(plan.Mode),
		ModeLabel:             modeLabel(plan.Mode),
		Status:                plan.Status,
		StatusReason:          plan.StatusReason,
		BuyLow:                plan.BuyLow,
		BuyHigh:               plan.BuyHigh,
		StopLoss:              plan.InvalPrice,
		TakeProfit:            plan.TP1,
		TakeProfit2:           plan.TP2,
		RR:                    plan.RR,
		RRQualified:           plan.RRQualified,
		Conclusion:            plan.Conclusion,
		EntryCondition:        plan.EntryCondition,
		InvalidationCondition: plan.InvalCondition,
		TargetSummary:         plan.TargetSummary,
		BreakoutConfirm:       plan.BreakoutConfirm,
		BreakoutExit:          plan.BreakoutExit,
		ChaseLimit:            plan.ChaseLimit,
		VolumeRequirement:     plan.VolumeRequirement,
		EventRisk:             plan.EventRisk,
		EventNotes:            plan.EventNotes,
		Overextended:          plan.Overextended,
		PositionContext:       plan.PositionSummary,
		Action:                plan.Action,
		IsHeld:                isHeld,
		CostPrice:             round2(costPrice),
		Daily:                 snapshotToMap(daily),
		Weekly:                snapshotToMap(weekly),
	}, nil
}

func analyzePlanPeriod(ctx context.Context, lb *longbridge.Client, symbol string, period quote.Period, count int32, tradeSession quote.CandlestickTradeSession) (planSnapshot, error) {
	candles, err := lb.GetCandlesticksWithTradeSession(ctx, symbol, period, count, tradeSession)
	if err != nil {
		return planSnapshot{}, err
	}
	closes := make([]float64, 0, len(candles))
	highs := make([]float64, 0, len(candles))
	lows := make([]float64, 0, len(candles))
	for _, candle := range candles {
		if candle == nil || candle.Close == nil || candle.High == nil || candle.Low == nil {
			continue
		}
		closeValue, _ := candle.Close.Float64()
		highValue, _ := candle.High.Float64()
		lowValue, _ := candle.Low.Float64()
		closes = append(closes, closeValue)
		highs = append(highs, highValue)
		lows = append(lows, lowValue)
	}
	if len(closes) < 20 {
		return planSnapshot{}, fmt.Errorf("not enough data")
	}

	lastClose := closes[len(closes)-1]
	ma10 := averageFloat(closes[maxInt(0, len(closes)-10):])
	ma20 := averageFloat(closes[maxInt(0, len(closes)-20):])
	ma60 := averageFloat(closes[maxInt(0, len(closes)-minInt(len(closes), 60)):])
	changeBaseIndex := maxInt(0, len(closes)-20)
	changePct := 0.0
	if base := closes[changeBaseIndex]; base > 0 {
		changePct = (lastClose - base) / base * 100
	}
	support := minFloatSlice(lows[maxInt(0, len(lows)-20):])
	resistance := maxFloatSlice(highs[maxInt(0, len(highs)-20):])
	latestVolume := 0.0
	if len(candles) > 0 {
		latestVolume = float64(candles[len(candles)-1].Volume)
	}
	avgVolume20 := 0.0
	if len(candles) > 0 {
		totalVolume := 0.0
		volumeSamples := 0
		for _, candle := range candles[maxInt(0, len(candles)-20):] {
			if candle == nil {
				continue
			}
			totalVolume += float64(candle.Volume)
			volumeSamples++
		}
		if volumeSamples > 0 {
			avgVolume20 = totalVolume / float64(volumeSamples)
		}
	}
	volumeRatio := 0.0
	if avgVolume20 > 0 {
		volumeRatio = latestVolume / avgVolume20
	}

	score := 50
	priceAbove20 := lastClose > ma20
	priceAbove60 := lastClose > ma60
	if priceAbove20 {
		score += 10
	} else {
		score -= 10
	}
	if priceAbove60 {
		score += 10
	} else {
		score -= 10
	}
	if ma20 > ma60 {
		score += 10
	} else {
		score -= 10
	}

	return planSnapshot{
		LastClose:    round2(lastClose),
		MA10:         round2(ma10),
		MA20:         round2(ma20),
		MA60:         round2(ma60),
		Support:      round2(support),
		Resistance:   round2(resistance),
		Score:        clamp(score, 0, 100),
		ChangePct:    round2(changePct),
		VolumeRatio:  round2(volumeRatio),
		PriceAbove20: priceAbove20,
		PriceAbove60: priceAbove60,
	}, nil
}

func combinePlanTrend(daily, weekly planSnapshot) (string, int) {
	score := clamp((daily.Score+weekly.Score)/2, 0, 100)
	if daily.PriceAbove20 && daily.PriceAbove60 && weekly.PriceAbove20 && score >= 60 {
		return "bullish", score
	}
	if !daily.PriceAbove20 && !weekly.PriceAbove20 && score <= 40 {
		return "bearish", score
	}
	return "neutral", score
}

func summarizeNews(title, publishedAt string) string {
	if publishedAt != "" {
		if ts, err := time.Parse(time.RFC3339, publishedAt); err == nil {
			return fmt.Sprintf("%s (%s)", title, ts.Format("01-02 15:04"))
		}
	}
	return title
}

func averageFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func minFloatSlice(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	minValue := values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
	}
	return minValue
}

func maxFloatSlice(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maxValue := values[0]
	for _, value := range values[1:] {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func minNonZeroFloat(left, right float64) float64 {
	switch {
	case left <= 0:
		return right
	case right <= 0:
		return left
	case left < right:
		return left
	default:
		return right
	}
}

func maxNonZeroFloat(left, right float64) float64 {
	switch {
	case left <= 0:
		return right
	case right <= 0:
		return left
	case left > right:
		return left
	default:
		return right
	}
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func snapshotToMap(snapshot planSnapshot) map[string]interface{} {
	return map[string]interface{}{
		"last_close":    snapshot.LastClose,
		"ma10":          snapshot.MA10,
		"ma20":          snapshot.MA20,
		"ma60":          snapshot.MA60,
		"support":       snapshot.Support,
		"resistance":    snapshot.Resistance,
		"score":         snapshot.Score,
		"change_pct":    snapshot.ChangePct,
		"volume_ratio":  snapshot.VolumeRatio,
		"price_above20": snapshot.PriceAbove20,
		"price_above60": snapshot.PriceAbove60,
	}
}

func modeLabel(mode planMode) string {
	switch mode {
	case planModePullback:
		return "回踩"
	case planModeBreakout:
		return "突破"
	case planModeEvent:
		return "事件"
	default:
		return "区间"
	}
}
