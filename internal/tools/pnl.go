package tools

import (
	"context"
	"fmt"

	"github.com/longportapp/openapi-go/quote"
	"github.com/longportapp/openapi-go/trade"
	"hades/internal/longbridge"
)

type positionSnapshot struct {
	Position          *trade.StockPosition
	Quantity          float64
	AvailableQuantity float64
	CostPrice         float64
	LastPrice         float64
	CostBasis         float64
	MarketValue       float64
	UnrealizedPnL     float64
	UnrealizedPnLPct  float64
	HasQuote          bool
}

func buildPositionSnapshots(ctx context.Context, lb *longbridge.Client) ([]positionSnapshot, error) {
	positionChannels, err := lb.GetPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %v", err)
	}

	positions := flattenPositions(positionChannels)
	if len(positions) == 0 {
		return []positionSnapshot{}, nil
	}

	symbols := make([]string, 0, len(positions))
	for _, position := range positions {
		if position == nil {
			continue
		}
		symbols = append(symbols, position.Symbol)
	}

	quotes, err := lb.GetQuote(ctx, symbols)
	if err != nil {
		return nil, fmt.Errorf("failed to get quotes for positions: %v", err)
	}

	quoteMap := make(map[string]*quote.SecurityQuote, len(quotes))
	for _, item := range quotes {
		if item == nil {
			continue
		}
		quoteMap[item.Symbol] = item
	}

	snapshots := make([]positionSnapshot, 0, len(positions))
	for _, position := range positions {
		if position == nil {
			continue
		}

		quantity := parseStringFloat(position.Quantity)
		availableQuantity := parseStringFloat(position.AvailableQuantity)
		costPrice := decimalToFloat(position.CostPrice)
		lastPrice := costPrice
		hasQuote := false
		if item := quoteMap[position.Symbol]; item != nil && item.LastDone != nil {
			lastPrice = decimalToFloat(item.LastDone)
			hasQuote = true
		}

		costBasis := quantity * costPrice
		marketValue := quantity * lastPrice
		unrealizedPnL := marketValue - costBasis
		unrealizedPnLPct := 0.0
		if costBasis > 0 {
			unrealizedPnLPct = unrealizedPnL / costBasis * 100
		}

		snapshots = append(snapshots, positionSnapshot{
			Position:          position,
			Quantity:          quantity,
			AvailableQuantity: availableQuantity,
			CostPrice:         round2(costPrice),
			LastPrice:         round2(lastPrice),
			CostBasis:         round2(costBasis),
			MarketValue:       round2(marketValue),
			UnrealizedPnL:     round2(unrealizedPnL),
			UnrealizedPnLPct:  round2(unrealizedPnLPct),
			HasQuote:          hasQuote,
		})
	}

	return snapshots, nil
}

func summarizePositionSnapshots(snapshots []positionSnapshot) map[string]interface{} {
	totalCostBasis := 0.0
	totalMarketValue := 0.0
	totalUnrealizedPnL := 0.0
	profitable := 0
	losing := 0
	missingQuotes := 0

	for _, snapshot := range snapshots {
		totalCostBasis += snapshot.CostBasis
		totalMarketValue += snapshot.MarketValue
		totalUnrealizedPnL += snapshot.UnrealizedPnL
		if !snapshot.HasQuote {
			missingQuotes++
			continue
		}
		switch {
		case snapshot.UnrealizedPnL > 0:
			profitable++
		case snapshot.UnrealizedPnL < 0:
			losing++
		}
	}

	totalUnrealizedPnLPct := 0.0
	if totalCostBasis > 0 {
		totalUnrealizedPnLPct = totalUnrealizedPnL / totalCostBasis * 100
	}

	return map[string]interface{}{
		"positions_count":        len(snapshots),
		"cost_basis":             round2(totalCostBasis),
		"market_value":           round2(totalMarketValue),
		"unrealized_pnl":         round2(totalUnrealizedPnL),
		"unrealized_pnl_pct":     round2(totalUnrealizedPnLPct),
		"profitable_positions":   profitable,
		"losing_positions":       losing,
		"missing_quotes":         missingQuotes,
		"realized_pnl_status":    "官方接口未直接返回已实现盈亏，当前结果仅统计持仓浮动盈亏。",
		"realized_metrics_ready": false,
	}
}
