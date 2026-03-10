package tools

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/longportapp/openapi-go/quote"
	"github.com/longportapp/openapi-go/trade"
	"hades/internal/longbridge"
)

type trendSnapshot struct {
	Period         string
	Trend          string
	Score          int
	LastClose      float64
	Support        float64
	Resistance     float64
	ChangePct      float64
	VolumeRatio    float64
	Signals        []string
	Risks          []string
	PriceAboveMA20 bool
	PriceAboveMA60 bool
}

func NewTrendAnalysisTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		symbol, ok := args["symbol"].(string)
		if !ok || strings.TrimSpace(symbol) == "" {
			return nil, fmt.Errorf("missing or invalid symbol parameter")
		}

		periods := parseTrendPeriods(args["periods"])
		lookback, ok, err := parseOptionalInt32(args["lookback"])
		if err != nil {
			return nil, fmt.Errorf("lookback must be a number: %v", err)
		}
		if !ok {
			lookback = 120
		}
		if lookback < 30 {
			lookback = 30
		}

		snapshots := make([]trendSnapshot, 0, len(periods))
		for _, periodLabel := range periods {
			snapshot, err := analyzeTrendPeriod(ctx, lb, symbol, periodLabel, lookback)
			if err != nil {
				return nil, err
			}
			snapshots = append(snapshots, snapshot)
		}

		overallTrend, overallScore := combineTrendSnapshots(snapshots)
		signals, risks := combineMessages(snapshots)
		suggestion := buildTrendSuggestion(overallTrend, overallScore, signals, risks)

		result := map[string]interface{}{
			"symbol":      symbol,
			"trend":       overallTrend,
			"score":       overallScore,
			"signals":     signals,
			"risks":       risks,
			"suggestion":  suggestion,
			"periods":     trendSnapshotsToMaps(snapshots),
			"support":     collectPriceLevels(snapshots, true),
			"resistance":  collectPriceLevels(snapshots, false),
			"analysis_of": "trend",
		}

		return map[string]interface{}{"result": result}, nil
	}
}

func NewWatchlistTrendAnalysisTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		symbols := splitSymbolsFromArgs(args["symbols"])
		if len(symbols) == 0 {
			return nil, fmt.Errorf("missing or invalid symbols parameter")
		}

		periods := parseTrendPeriods(args["periods"])
		lookback, ok, err := parseOptionalInt32(args["lookback"])
		if err != nil {
			return nil, fmt.Errorf("lookback must be a number: %v", err)
		}
		if !ok {
			lookback = 120
		}
		if lookback < 30 {
			lookback = 30
		}

		items := make([]map[string]interface{}, 0, len(symbols))
		for _, symbol := range symbols {
			snapshots := make([]trendSnapshot, 0, len(periods))
			for _, periodLabel := range periods {
				snapshot, err := analyzeTrendPeriod(ctx, lb, symbol, periodLabel, lookback)
				if err != nil {
					return nil, err
				}
				snapshots = append(snapshots, snapshot)
			}

			trend, score := combineTrendSnapshots(snapshots)
			signals, risks := combineMessages(snapshots)
			items = append(items, map[string]interface{}{
				"symbol":     symbol,
				"trend":      trend,
				"score":      score,
				"signals":    signals,
				"risks":      risks,
				"suggestion": buildTrendSuggestion(trend, score, signals, risks),
			})
		}

		sort.Slice(items, func(i, j int) bool {
			left, _ := items[i]["score"].(int)
			right, _ := items[j]["score"].(int)
			return left > right
		})

		return map[string]interface{}{
			"result": map[string]interface{}{
				"analysis_of": "watchlist_trends",
				"count":       len(items),
				"items":       items,
			},
		}, nil
	}
}

func NewPositionsTrendAnalysisTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		periods := parseTrendPeriods(args["periods"])
		lookback, ok, err := parseOptionalInt32(args["lookback"])
		if err != nil {
			return nil, fmt.Errorf("lookback must be a number: %v", err)
		}
		if !ok {
			lookback = 120
		}
		if lookback < 30 {
			lookback = 30
		}

		snapshotsWithPnL, err := buildPositionSnapshots(ctx, lb)
		if err != nil {
			return nil, err
		}
		if len(snapshotsWithPnL) == 0 {
			return map[string]interface{}{
				"result": map[string]interface{}{
					"analysis_of": "positions_trends",
					"count":       0,
					"summary":     summarizePositionSnapshots(snapshotsWithPnL),
					"items":       []map[string]interface{}{},
				},
			}, nil
		}

		items := make([]map[string]interface{}, 0, len(snapshotsWithPnL))
		for _, snapshotWithPnL := range snapshotsWithPnL {
			position := snapshotWithPnL.Position

			snapshots := make([]trendSnapshot, 0, len(periods))
			for _, periodLabel := range periods {
				snapshot, err := analyzeTrendPeriod(ctx, lb, position.Symbol, periodLabel, lookback)
				if err != nil {
					return nil, err
				}
				snapshots = append(snapshots, snapshot)
			}

			trend, score := combineTrendSnapshots(snapshots)
			signals, risks := combineMessages(snapshots)
			items = append(items, map[string]interface{}{
				"symbol":             position.Symbol,
				"symbol_name":        position.SymbolName,
				"quantity":           position.Quantity,
				"available_quantity": position.AvailableQuantity,
				"cost_price":         fmt.Sprintf("%v", position.CostPrice),
				"last_price":         snapshotWithPnL.LastPrice,
				"cost_basis":         snapshotWithPnL.CostBasis,
				"market_value":       snapshotWithPnL.MarketValue,
				"unrealized_pnl":     snapshotWithPnL.UnrealizedPnL,
				"unrealized_pnl_pct": snapshotWithPnL.UnrealizedPnLPct,
				"market":             fmt.Sprintf("%v", position.Market),
				"trend":              trend,
				"score":              score,
				"signals":            signals,
				"risks":              risks,
				"suggestion":         buildPositionSuggestion(trend, score, risks),
			})
		}

		sort.Slice(items, func(i, j int) bool {
			left, _ := items[i]["score"].(int)
			right, _ := items[j]["score"].(int)
			return left > right
		})

		return map[string]interface{}{
			"result": map[string]interface{}{
				"analysis_of": "positions_trends",
				"count":       len(items),
				"summary":     summarizePositionSnapshots(snapshotsWithPnL),
				"items":       items,
			},
		}, nil
	}
}

func analyzeTrendPeriod(ctx context.Context, lb *longbridge.Client, symbol, periodLabel string, lookback int32) (trendSnapshot, error) {
	period := parsePeriod(periodLabel)
	candles, err := lb.GetCandlesticks(ctx, symbol, period, lookback)
	if err != nil {
		return unavailableTrendSnapshot(symbol, periodLabel, fmt.Sprintf("K线数据暂不可用: %v", err)), nil
	}
	if len(candles) < 20 {
		return unavailableTrendSnapshot(symbol, periodLabel, "K线数据不足，暂时无法完成趋势分析"), nil
	}

	sanitized := make([]*quote.Candlestick, 0, len(candles))
	for _, candle := range candles {
		if candle != nil {
			sanitized = append(sanitized, candle)
		}
	}
	if len(sanitized) < 20 {
		return unavailableTrendSnapshot(symbol, periodLabel, "有效K线数据不足，暂时无法完成趋势分析"), nil
	}

	sort.Slice(sanitized, func(i, j int) bool {
		return sanitized[i].Timestamp < sanitized[j].Timestamp
	})

	closes := make([]float64, 0, len(sanitized))
	highs := make([]float64, 0, len(sanitized))
	lows := make([]float64, 0, len(sanitized))
	volumes := make([]float64, 0, len(sanitized))
	for _, candle := range sanitized {
		closes = append(closes, decimalToFloat(candle.Close))
		highs = append(highs, decimalToFloat(candle.High))
		lows = append(lows, decimalToFloat(candle.Low))
		volumes = append(volumes, float64(candle.Volume))
	}

	lastClose := closes[len(closes)-1]
	sma5 := average(closes[max(0, len(closes)-5):])
	sma20 := average(closes[max(0, len(closes)-20):])
	sma60 := average(closes[max(0, len(closes)-min(len(closes), 60)):])
	changeBaseIndex := max(0, len(closes)-20)
	changePct := percentChange(closes[changeBaseIndex], lastClose)
	latestVolume := volumes[len(volumes)-1]
	avgVolume20 := average(volumes[max(0, len(volumes)-20):])
	volumeRatio := safeDiv(latestVolume, avgVolume20)
	support := minFloatSlice(lows[max(0, len(lows)-20):])
	resistance := maxFloatSlice(highs[max(0, len(highs)-20):])

	score := 50
	signals := make([]string, 0, 6)
	risks := make([]string, 0, 4)

	priceAboveMA20 := lastClose > sma20
	priceAboveMA60 := lastClose > sma60

	if lastClose > sma20 {
		score += 10
		signals = append(signals, fmt.Sprintf("%s 价格站上 MA20", periodLabel))
	} else {
		score -= 10
		risks = append(risks, fmt.Sprintf("%s 价格跌破 MA20", periodLabel))
	}

	if lastClose > sma60 {
		score += 10
		signals = append(signals, fmt.Sprintf("%s 价格站上 MA60", periodLabel))
	} else {
		score -= 10
		risks = append(risks, fmt.Sprintf("%s 价格位于 MA60 下方", periodLabel))
	}

	if sma20 > sma60 {
		score += 10
		signals = append(signals, fmt.Sprintf("%s MA20 在 MA60 上方", periodLabel))
	} else {
		score -= 10
		risks = append(risks, fmt.Sprintf("%s MA20 未形成上穿 MA60", periodLabel))
	}

	if changePct > 3 {
		score += 8
		signals = append(signals, fmt.Sprintf("%s 最近 20 根上涨 %.2f%%", periodLabel, changePct))
	} else if changePct < -3 {
		score -= 8
		risks = append(risks, fmt.Sprintf("%s 最近 20 根回撤 %.2f%%", periodLabel, math.Abs(changePct)))
	}

	if volumeRatio >= 1.5 {
		score += 6
		signals = append(signals, fmt.Sprintf("%s 成交量放大到 %.2f 倍", periodLabel, volumeRatio))
	} else if volumeRatio <= 0.7 {
		risks = append(risks, fmt.Sprintf("%s 成交量偏弱，仅 %.2f 倍", periodLabel, volumeRatio))
	}

	if lastClose >= resistance*0.99 {
		signals = append(signals, fmt.Sprintf("%s 接近区间压力位 %.2f", periodLabel, resistance))
		risks = append(risks, fmt.Sprintf("%s 追高风险上升", periodLabel))
	}

	if support > 0 && lastClose <= support*1.02 {
		signals = append(signals, fmt.Sprintf("%s 接近区间支撑位 %.2f", periodLabel, support))
	}

	if lastClose < sma5 && lastClose > sma20 {
		risks = append(risks, fmt.Sprintf("%s 短线有回踩压力", periodLabel))
	}

	trend := classifyTrend(score, lastClose, sma20, sma60)

	return trendSnapshot{
		Period:         periodLabel,
		Trend:          trend,
		Score:          clampInt(score, 0, 100),
		LastClose:      round2(lastClose),
		Support:        round2(support),
		Resistance:     round2(resistance),
		ChangePct:      round2(changePct),
		VolumeRatio:    round2(volumeRatio),
		Signals:        dedupeStrings(signals),
		Risks:          dedupeStrings(risks),
		PriceAboveMA20: priceAboveMA20,
		PriceAboveMA60: priceAboveMA60,
	}, nil
}

func unavailableTrendSnapshot(symbol, periodLabel, reason string) trendSnapshot {
	return trendSnapshot{
		Period:      periodLabel,
		Trend:       "neutral",
		Score:       50,
		Signals:     []string{},
		Risks:       []string{fmt.Sprintf("%s %s: %s", symbol, periodLabel, reason)},
		LastClose:   0,
		Support:     0,
		Resistance:  0,
		ChangePct:   0,
		VolumeRatio: 0,
	}
}

func parseTrendPeriods(raw interface{}) []string {
	periods := splitStringArg(raw)
	if len(periods) == 0 {
		return []string{"1d", "1h", "15m"}
	}
	return periods
}

func splitSymbolsFromArgs(raw interface{}) []string {
	return splitStringArg(raw)
}

func flattenPositions(channels []*trade.StockPositionChannel) []*trade.StockPosition {
	positions := make([]*trade.StockPosition, 0)
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		positions = append(positions, channel.Positions...)
	}
	return positions
}

func combineTrendSnapshots(snapshots []trendSnapshot) (string, int) {
	if len(snapshots) == 0 {
		return "neutral", 50
	}

	total := 0
	bullish := 0
	bearish := 0
	for _, snapshot := range snapshots {
		total += snapshot.Score
		switch snapshot.Trend {
		case "bullish":
			bullish++
		case "bearish":
			bearish++
		}
	}

	avgScore := clampInt(total/len(snapshots), 0, 100)
	if bullish > bearish && avgScore >= 60 {
		return "bullish", avgScore
	}
	if bearish > bullish && avgScore <= 40 {
		return "bearish", avgScore
	}
	return "neutral", avgScore
}

func combineMessages(snapshots []trendSnapshot) ([]string, []string) {
	signals := make([]string, 0, len(snapshots)*2)
	risks := make([]string, 0, len(snapshots)*2)
	for _, snapshot := range snapshots {
		signals = append(signals, snapshot.Signals...)
		risks = append(risks, snapshot.Risks...)
	}
	return dedupeStrings(signals), dedupeStrings(risks)
}

func buildTrendSuggestion(trend string, score int, signals, risks []string) string {
	if hasUnavailableTrendData(signals, risks) {
		return "部分周期K线数据暂不可用，当前仅能给出有限趋势判断。"
	}

	switch trend {
	case "bullish":
		if score >= 75 {
			return "多周期趋势偏强，适合等待回踩或放量确认后跟踪。"
		}
		return "趋势偏多，但更适合等待更好的介入位置。"
	case "bearish":
		return "当前走势偏弱，优先控制风险，避免逆势追买。"
	default:
		if len(risks) > len(signals) {
			return "走势震荡偏弱，建议先观察，等待方向明确。"
		}
		return "走势中性，适合结合更高周期或消息面再确认。"
	}
}

func buildPositionSuggestion(trend string, score int, risks []string) string {
	if hasUnavailableTrendData(nil, risks) {
		return "部分周期K线数据暂不可用，先按仓位和止损计划管理持仓。"
	}

	switch trend {
	case "bullish":
		if score >= 75 {
			return "持仓趋势偏强，可继续跟踪，重点观察回踩后的承接。"
		}
		return "持仓趋势偏多，但短线不宜盲目追高。"
	case "bearish":
		if len(risks) > 0 {
			return "持仓走势偏弱，建议重新检查止损位和仓位控制。"
		}
		return "持仓走势偏弱，优先防守。"
	default:
		return "持仓走势中性，适合结合成本价和计划仓位继续观察。"
	}
}

func hasUnavailableTrendData(signals, risks []string) bool {
	for _, item := range signals {
		if strings.Contains(item, "K线数据") {
			return true
		}
	}
	for _, item := range risks {
		if strings.Contains(item, "K线数据") {
			return true
		}
	}
	return false
}

func trendSnapshotsToMaps(snapshots []trendSnapshot) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(snapshots))
	for _, snapshot := range snapshots {
		result = append(result, map[string]interface{}{
			"period":       snapshot.Period,
			"trend":        snapshot.Trend,
			"score":        snapshot.Score,
			"last_close":   snapshot.LastClose,
			"support":      snapshot.Support,
			"resistance":   snapshot.Resistance,
			"change_pct":   snapshot.ChangePct,
			"volume_ratio": snapshot.VolumeRatio,
			"signals":      snapshot.Signals,
			"risks":        snapshot.Risks,
		})
	}
	return result
}

func collectPriceLevels(snapshots []trendSnapshot, support bool) []float64 {
	levels := make([]float64, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if support && snapshot.Support > 0 {
			levels = append(levels, snapshot.Support)
		}
		if !support && snapshot.Resistance > 0 {
			levels = append(levels, snapshot.Resistance)
		}
	}
	sort.Float64s(levels)
	return uniqueRoundedLevels(levels)
}

func uniqueRoundedLevels(levels []float64) []float64 {
	result := make([]float64, 0, len(levels))
	seen := make(map[string]struct{})
	for _, level := range levels {
		key := fmt.Sprintf("%.2f", level)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, round2(level))
	}
	return result
}

func dedupeStrings(items []string) []string {
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func classifyTrend(score int, lastClose, sma20, sma60 float64) string {
	if lastClose > sma20 && sma20 > sma60 && score >= 60 {
		return "bullish"
	}
	if lastClose < sma20 && sma20 < sma60 && score <= 40 {
		return "bearish"
	}
	return "neutral"
}

func decimalToFloat(value interface{ Float64() (float64, bool) }) float64 {
	if value == nil {
		return 0
	}
	v, _ := value.Float64()
	return v
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func percentChange(from, to float64) float64 {
	if from == 0 {
		return 0
	}
	return (to - from) / from * 100
}

func safeDiv(left, right float64) float64 {
	if right == 0 {
		return 0
	}
	return left / right
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

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
