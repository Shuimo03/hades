package alerts

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/longportapp/openapi-go/quote"
	"hades/internal/longbridge"
	"hades/internal/planlevels"
)

func BuildAlertPlanContext(ctx context.Context, lb *longbridge.Client, symbol string, scope longbridge.QuoteSessionScope) string {
	metrics, err := buildAlertPlanMetrics(ctx, lb, symbol, scope)
	if err != nil {
		return ""
	}

	priceLabel := "当前价"
	if metrics.CurrentPriceSession != "" {
		priceLabel = fmt.Sprintf("当前价(%s)", longbridge.QuoteSessionDisplayName(metrics.CurrentPriceSession))
	}
	lines := []string{
		"",
		"交易计划参考:",
		fmt.Sprintf("- %s: %.2f", priceLabel, round2(metrics.CurrentPrice)),
		fmt.Sprintf("- 周期趋势: %s (评分: %d)", metrics.Trend, metrics.Score),
		fmt.Sprintf("- 关注买入区: %.2f - %.2f", metrics.BuyLow, metrics.BuyHigh),
		fmt.Sprintf("- 止损参考: %.2f", metrics.StopLoss),
		fmt.Sprintf("- 止盈参考: %.2f", metrics.TakeProfit),
		fmt.Sprintf("- 操作建议: %s", metrics.Action),
	}
	if metrics.IsHeld {
		lines = append(lines, fmt.Sprintf("- 当前持仓成本: %.2f", metrics.CostPrice))
	}

	if news, err := lb.GetStockNews(ctx, symbol); err == nil && len(news) > 0 {
		lines = append(lines, "- 近期资讯:")
		count := 0
		for _, item := range news {
			if item == nil || strings.TrimSpace(item.Title) == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("  - %s", summarizeNews(item.Title, item.PublishedAt)))
			count++
			if count >= 2 {
				break
			}
		}
	}

	return strings.Join(lines, "\n")
}

type AlertPlanMetrics struct {
	CurrentPrice        float64
	CurrentPriceSession longbridge.QuoteSession
	Trend               string
	Score               int
	BuyLow              float64
	BuyHigh             float64
	StopLoss            float64
	TakeProfit          float64
	Action              string
	IsHeld              bool
	CostPrice           float64
}

type planSnapshot struct {
	LastClose    float64
	Support      float64
	Resistance   float64
	Score        int
	PriceAbove20 bool
	PriceAbove60 bool
}

func buildAlertPlanMetrics(ctx context.Context, lb *longbridge.Client, symbol string, scope longbridge.QuoteSessionScope) (*AlertPlanMetrics, error) {
	if strings.TrimSpace(symbol) == "" {
		return nil, fmt.Errorf("missing symbol")
	}

	quotes, err := lb.GetQuote(ctx, []string{symbol})
	if err != nil || len(quotes) == 0 || quotes[0] == nil {
		return nil, fmt.Errorf("missing quote")
	}
	effectiveQuote := longbridge.ResolveEffectiveQuote(quotes[0], scope)
	if !effectiveQuote.HasQuote {
		return nil, fmt.Errorf("missing quote")
	}

	currentPrice := effectiveQuote.Price
	tradeSession := longbridge.CandlestickTradeSessionFromScope(scope)
	daily, err := analyzePlanPeriod(ctx, lb, symbol, quote.PeriodDay, 120, tradeSession)
	if err != nil {
		return nil, err
	}
	weekly, err := analyzePlanPeriod(ctx, lb, symbol, quote.PeriodWeek, 60, tradeSession)
	if err != nil {
		return nil, err
	}

	trend, score := combinePlanTrend(daily, weekly)
	buyLow, buyHigh := planlevels.BuyZone(trend, currentPrice, daily.Support, weekly.Support)
	stopLoss := planlevels.StopLoss(trend, currentPrice, daily.Support, weekly.Support)
	takeProfit := planlevels.TakeProfit(currentPrice, daily.Resistance, weekly.Resistance)

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

	return &AlertPlanMetrics{
		CurrentPrice:        round2(currentPrice),
		CurrentPriceSession: effectiveQuote.Session,
		Trend:               trend,
		Score:               score,
		BuyLow:              buyLow,
		BuyHigh:             buyHigh,
		StopLoss:            stopLoss,
		TakeProfit:          takeProfit,
		Action:              buildPlanAction(trend, score, currentPrice, buyHigh, stopLoss, takeProfit, isHeld, costPrice),
		IsHeld:              isHeld,
		CostPrice:           round2(costPrice),
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
	ma20 := averageFloat(closes[maxInt(0, len(closes)-20):])
	ma60 := averageFloat(closes[maxInt(0, len(closes)-minInt(len(closes), 60)):])
	support := minFloatSlice(lows[maxInt(0, len(lows)-20):])
	resistance := maxFloatSlice(highs[maxInt(0, len(highs)-20):])

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
		Support:      round2(support),
		Resistance:   round2(resistance),
		Score:        clamp(score, 0, 100),
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

func buildPlanAction(trend string, score int, currentPrice, buyHigh, stopLoss, takeProfit float64, isHeld bool, costPrice float64) string {
	if isHeld {
		switch {
		case currentPrice >= takeProfit:
			return "已接近止盈位，优先考虑分批止盈或上移止损。"
		case currentPrice <= stopLoss:
			return "已逼近止损位，优先控制风险，避免继续拖延。"
		case trend == "bearish":
			return "你已持有该标的，走势转弱，优先考虑减仓或防守。"
		case costPrice > 0 && currentPrice > costPrice:
			return "仍在盈利区，可继续跟踪，但建议把止损抬到成本附近。"
		default:
			return "你已持有该标的，先按仓位计划管理，不建议再盲目加仓。"
		}
	}

	switch {
	case trend == "bullish" && score >= 70 && currentPrice <= buyHigh:
		return "可等回踩买入区分批关注，不追高。"
	case trend == "bullish" && score >= 70:
		return "趋势偏强，等待更接近支撑位再介入。"
	case trend == "neutral" && takeProfit > currentPrice:
		return "区间震荡，优先等突破或回踩确认。"
	case trend == "bearish" && currentPrice > stopLoss:
		return "走势偏弱，先观察或减仓，不急于加仓。"
	default:
		return "先观察，等待更明确的价格信号。"
	}
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
