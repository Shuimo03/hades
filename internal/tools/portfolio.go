package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/longportapp/openapi-go/quote"
	"hades/internal/longbridge"
)

func NewPortfolioRiskTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		snapshots, err := buildPositionSnapshots(ctx, lb)
		if err != nil {
			return nil, err
		}

		totalMarketValue := 0.0
		for _, snapshot := range snapshots {
			totalMarketValue += snapshot.MarketValue
		}

		items := make([]map[string]interface{}, 0, len(snapshots))
		topWeight := 0.0
		top3Weight := 0.0
		weakCount := 0
		losingCount := 0
		for _, snapshot := range snapshots {
			weight := 0.0
			if totalMarketValue > 0 {
				weight = snapshot.MarketValue / totalMarketValue * 100
			}
			if weight > topWeight {
				topWeight = weight
			}
			if snapshot.UnrealizedPnL < 0 {
				losingCount++
			}

			daily, err := analyzeTrendPeriod(ctx, lb, snapshot.Position.Symbol, "1d", 120, quote.CandlestickTradeSessionNormal)
			if err != nil {
				return nil, err
			}
			hourly, err := analyzeTrendPeriod(ctx, lb, snapshot.Position.Symbol, "1h", 120, quote.CandlestickTradeSessionNormal)
			if err != nil {
				return nil, err
			}
			trend, score := combineTrendSnapshots([]trendSnapshot{daily, hourly})
			signals, risks := combineMessages([]trendSnapshot{daily, hourly})
			if score <= 40 {
				weakCount++
			}

			items = append(items, map[string]interface{}{
				"symbol":             snapshot.Position.Symbol,
				"symbol_name":        snapshot.Position.SymbolName,
				"market_value":       snapshot.MarketValue,
				"weight_pct":         round2(weight),
				"unrealized_pnl":     snapshot.UnrealizedPnL,
				"unrealized_pnl_pct": snapshot.UnrealizedPnLPct,
				"trend":              trend,
				"score":              score,
				"signals":            signals,
				"risks":              risks,
			})
		}

		sort.Slice(items, func(i, j int) bool {
			left, _ := items[i]["weight_pct"].(float64)
			right, _ := items[j]["weight_pct"].(float64)
			return left > right
		})
		for i := 0; i < len(items) && i < 3; i++ {
			weight, _ := items[i]["weight_pct"].(float64)
			top3Weight += weight
		}

		portfolioRisks := make([]string, 0, 6)
		if topWeight >= 35 {
			portfolioRisks = append(portfolioRisks, fmt.Sprintf("单一持仓占比 %.2f%%，集中度偏高。", topWeight))
		}
		if top3Weight >= 70 {
			portfolioRisks = append(portfolioRisks, fmt.Sprintf("前 3 大持仓合计占比 %.2f%%，组合分散度不足。", top3Weight))
		}
		if losingCount >= 3 {
			portfolioRisks = append(portfolioRisks, fmt.Sprintf("当前有 %d 个亏损持仓，需关注组合回撤。", losingCount))
		}
		if weakCount >= 2 {
			portfolioRisks = append(portfolioRisks, fmt.Sprintf("当前有 %d 个弱势持仓，建议复核仓位和止损。", weakCount))
		}
		if len(portfolioRisks) == 0 {
			portfolioRisks = append(portfolioRisks, "当前组合未见明显结构性失衡，但仍需继续跟踪集中度和回撤。")
		}

		return map[string]interface{}{
			"result": map[string]interface{}{
				"analysis_of":       "portfolio_risk",
				"positions_count":   len(items),
				"top_weight_pct":    round2(topWeight),
				"top3_weight_pct":   round2(top3Weight),
				"losing_positions":  losingCount,
				"weak_positions":    weakCount,
				"portfolio_risks":   portfolioRisks,
				"portfolio_actions": buildPortfolioActions(portfolioRisks),
				"items":             items,
			},
		}, nil
	}
}

func buildPortfolioActions(risks []string) []string {
	actions := make([]string, 0, 4)
	for _, risk := range risks {
		switch {
		case containsAny(risk, "单一持仓占比", "前 3 大持仓"):
			actions = append(actions, "优先限制新增仓位继续集中到同一标的或同一风格。")
		case containsAny(risk, "亏损持仓", "弱势持仓"):
			actions = append(actions, "优先复核止损位和减仓顺序，避免亏损仓位继续拖累。")
		}
	}
	if len(actions) == 0 {
		actions = append(actions, "维持当前组合结构，重点跟踪高权重仓位和关键止损位。")
	}
	return dedupeStrings(actions)
}

func containsAny(text string, candidates ...string) bool {
	for _, candidate := range candidates {
		if candidate != "" && strings.Contains(text, candidate) {
			return true
		}
	}
	return false
}
