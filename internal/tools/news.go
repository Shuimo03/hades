package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/longportapp/openapi-go/quote"
	"hades/internal/longbridge"
	"hades/internal/planlevels"
)

func NewStockNewsTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		symbol, ok := args["symbol"].(string)
		if !ok || strings.TrimSpace(symbol) == "" {
			return nil, fmt.Errorf("missing or invalid symbol parameter")
		}

		count, hasCount, err := parseOptionalInt(args["count"])
		if err != nil {
			return nil, fmt.Errorf("count must be a number: %v", err)
		}
		if !hasCount || count <= 0 {
			count = 5
		}

		items, err := lb.GetStockNews(ctx, strings.TrimSpace(symbol))
		if err != nil {
			return nil, fmt.Errorf("failed to get stock news: %v", err)
		}
		if len(items) == 0 {
			return map[string]interface{}{
				"result": map[string]interface{}{
					"symbol": symbol,
					"count":  0,
					"items":  []map[string]interface{}{},
				},
			}, nil
		}

		resultItems := make([]map[string]interface{}, 0, min(count, len(items)))
		for _, item := range items {
			if item == nil {
				continue
			}
			resultItems = append(resultItems, stockNewsItemMap(item))
			if len(resultItems) >= count {
				break
			}
		}

		return map[string]interface{}{
			"result": map[string]interface{}{
				"symbol": symbol,
				"count":  len(resultItems),
				"items":  resultItems,
			},
		}, nil
	}
}

func NewWatchlistPlanTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		symbols, source, err := resolveSymbolsFromArgs(ctx, lb, args)
		if err != nil {
			return nil, err
		}

		newsCount, hasNewsCount, err := parseOptionalInt(args["news_count"])
		if err != nil {
			return nil, fmt.Errorf("news_count must be a number: %v", err)
		}
		if !hasNewsCount || newsCount <= 0 {
			newsCount = 3
		}

		lookback, hasLookback, err := parseOptionalInt32(args["lookback"])
		if err != nil {
			return nil, fmt.Errorf("lookback must be a number: %v", err)
		}
		if !hasLookback || lookback < 60 {
			lookback = 120
		}

		items := make([]map[string]interface{}, 0, len(symbols))
		for _, symbol := range symbols {
			weekly, err := analyzeTrendPeriod(ctx, lb, symbol, "1w", 60, quote.CandlestickTradeSessionNormal)
			if err != nil {
				return nil, err
			}
			daily, err := analyzeTrendPeriod(ctx, lb, symbol, "1d", lookback, quote.CandlestickTradeSessionNormal)
			if err != nil {
				return nil, err
			}

			overallTrend, overallScore := combineTrendSnapshots([]trendSnapshot{weekly, daily})
			signals, risks := combineMessages([]trendSnapshot{weekly, daily})
			newsItems, err := lb.GetStockNews(ctx, symbol)
			if err != nil {
				return nil, fmt.Errorf("failed to get news for %s: %v", symbol, err)
			}

			resultItem := buildWatchlistPlanItem(symbol, weekly, daily, overallTrend, overallScore, signals, risks, newsItems, newsCount)
			items = append(items, resultItem)
		}

		sort.Slice(items, func(i, j int) bool {
			left, _ := items[i]["score"].(int)
			right, _ := items[j]["score"].(int)
			return left > right
		})

		return map[string]interface{}{
			"result": map[string]interface{}{
				"analysis_of": "watchlist_plan",
				"count":       len(items),
				"source":      source,
				"items":       items,
			},
		}, nil
	}
}

func buildWatchlistPlanItem(symbol string, weekly, daily trendSnapshot, overallTrend string, overallScore int, signals, risks []string, newsItems []*longbridge.StockNewsItem, newsCount int) map[string]interface{} {
	buyZoneLow, buyZoneHigh := planlevels.BuyZone(overallTrend, daily.LastClose, daily.Support, weekly.Support)
	stopLoss := planlevels.StopLoss(overallTrend, daily.LastClose, daily.Support, weekly.Support)
	takeProfit := planlevels.TakeProfit(daily.LastClose, daily.Resistance, weekly.Resistance)
	action := buildActionPlan(overallTrend, overallScore, daily.LastClose, buyZoneHigh, stopLoss, takeProfit)

	topNews := make([]map[string]interface{}, 0, min(newsCount, len(newsItems)))
	newsSignals := make([]string, 0, newsCount)
	for _, item := range newsItems {
		if item == nil {
			continue
		}
		topNews = append(topNews, stockNewsItemMap(item))
		newsSignals = append(newsSignals, summarizeNewsHeadline(item))
		if len(topNews) >= newsCount {
			break
		}
	}

	allSignals := dedupeStrings(append(signals, newsSignals...))

	return map[string]interface{}{
		"symbol":        symbol,
		"trend":         overallTrend,
		"score":         overallScore,
		"action":        action,
		"buy_zone_low":  buyZoneLow,
		"buy_zone_high": buyZoneHigh,
		"stop_loss":     stopLoss,
		"take_profit":   takeProfit,
		"signals":       allSignals,
		"risks":         risks,
		"suggestion":    buildWatchlistSuggestion(action, allSignals, risks),
		"weekly": map[string]interface{}{
			"trend":      weekly.Trend,
			"score":      weekly.Score,
			"last_close": weekly.LastClose,
			"support":    weekly.Support,
			"resistance": weekly.Resistance,
		},
		"daily": map[string]interface{}{
			"trend":      daily.Trend,
			"score":      daily.Score,
			"last_close": daily.LastClose,
			"support":    daily.Support,
			"resistance": daily.Resistance,
		},
		"news": topNews,
	}
}

func stockNewsItemMap(item *longbridge.StockNewsItem) map[string]interface{} {
	return map[string]interface{}{
		"title":          item.Title,
		"description":    item.Description,
		"url":            item.URL,
		"published_at":   item.PublishedAt,
		"comments_count": item.CommentsCount,
		"likes_count":    item.LikesCount,
		"shares_count":   item.SharesCount,
	}
}

func summarizeNewsHeadline(item *longbridge.StockNewsItem) string {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return ""
	}
	if publishedAt := strings.TrimSpace(item.PublishedAt); publishedAt != "" {
		if ts, err := time.Parse(time.RFC3339, publishedAt); err == nil {
			return fmt.Sprintf("资讯: %s (%s)", title, ts.Format("01-02 15:04"))
		}
	}
	return "资讯: " + title
}

func buildActionPlan(trend string, score int, lastClose, buyZoneHigh, stopLoss, takeProfit float64) string {
	switch {
	case trend == "bullish" && score >= 70 && lastClose <= buyZoneHigh:
		return "watch_pullback_buy"
	case trend == "bullish" && score >= 70:
		return "wait_pullback"
	case trend == "neutral" && takeProfit > lastClose:
		return "watch_breakout"
	case trend == "bearish" && lastClose > stopLoss:
		return "reduce_or_wait"
	default:
		return "observe"
	}
}

func buildWatchlistSuggestion(action string, signals, risks []string) string {
	switch action {
	case "watch_pullback_buy":
		return "周日共振偏强，优先等待回踩买入区间，不追高。"
	case "wait_pullback":
		return "趋势仍偏强，但当前位置不够舒适，等回踩再考虑。"
	case "watch_breakout":
		return "当前更适合等突破确认或支撑企稳后再行动。"
	case "reduce_or_wait":
		return "走势偏弱，先以观察或减仓思路为主，避免盲目加仓。"
	default:
		if len(risks) > len(signals) {
			return "信息面和走势都不够顺畅，先观察。"
		}
		return "维持观察，等待更明确的价格信号。"
	}
}

func minNonZero(left, right float64) float64 {
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

func maxNonZero(left, right float64) float64 {
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
