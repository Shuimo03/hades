package tools

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/longportapp/openapi-go/trade"
	"hades/internal/longbridge"
)

type executionLot struct {
	Quantity float64
	Price    float64
}

type executionEvent struct {
	OrderID  string
	Symbol   string
	Side     trade.OrderSide
	Quantity float64
	Price    float64
	DoneAt   time.Time
}

func buildRealizedPnLSummary(ctx context.Context, lb *longbridge.Client, start, end time.Time) (map[string]interface{}, error) {
	statuses := []trade.OrderStatus{
		trade.OrderFilledStatus,
		trade.OrderPartialFilledStatus,
	}

	orders, hasMoreOrders, err := lb.GetHistoryOrders(ctx, "", statuses, time.Time{}, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get history orders for realized pnl: %v", err)
	}
	executions, err := lb.GetHistoryExecutions(ctx, "", time.Time{}, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get history executions for realized pnl: %v", err)
	}

	orderByID := make(map[string]*trade.Order, len(orders))
	for _, order := range orders {
		if order == nil || order.OrderId == "" {
			continue
		}
		orderByID[order.OrderId] = order
	}

	events := make([]executionEvent, 0, len(executions))
	missingOrderRefs := 0
	for _, execution := range executions {
		if execution == nil || execution.OrderId == "" {
			continue
		}
		order := orderByID[execution.OrderId]
		if order == nil {
			missingOrderRefs++
			continue
		}
		qty := parseStringFloat(execution.Quantity)
		if qty <= 0 {
			continue
		}
		events = append(events, executionEvent{
			OrderID:  execution.OrderId,
			Symbol:   execution.Symbol,
			Side:     order.Side,
			Quantity: qty,
			Price:    decimalFloat(execution.Price),
			DoneAt:   execution.TradeDoneAt,
		})
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].DoneAt.Equal(events[j].DoneAt) {
			if events[i].Symbol == events[j].Symbol {
				return events[i].OrderID < events[j].OrderID
			}
			return events[i].Symbol < events[j].Symbol
		}
		return events[i].DoneAt.Before(events[j].DoneAt)
	})

	inventory := make(map[string][]executionLot)
	periodRealizedPnL := 0.0
	grossProfit := 0.0
	grossLoss := 0.0
	winningTrades := 0
	losingTrades := 0
	breakevenTrades := 0
	closedTrades := 0
	unmatchedSellQuantity := 0.0
	dayPnL := make(map[string]float64)

	for _, event := range events {
		if event.DoneAt.After(end) {
			continue
		}

		switch event.Side {
		case trade.OrderSideBuy:
			inventory[event.Symbol] = append(inventory[event.Symbol], executionLot{
				Quantity: event.Quantity,
				Price:    event.Price,
			})
		case trade.OrderSideSell:
			remaining := event.Quantity
			realizedForEvent := 0.0
			lots := inventory[event.Symbol]

			for remaining > 0 && len(lots) > 0 {
				matched := minFloat(remaining, lots[0].Quantity)
				realizedForEvent += (event.Price - lots[0].Price) * matched
				remaining -= matched
				lots[0].Quantity -= matched
				if lots[0].Quantity <= 1e-9 {
					lots = lots[1:]
				}
			}
			inventory[event.Symbol] = lots

			if remaining > 0 {
				unmatchedSellQuantity += remaining
			}

			if !event.DoneAt.Before(start) {
				realizedForEvent = round2(realizedForEvent)
				periodRealizedPnL += realizedForEvent
				if realizedForEvent > 0 {
					grossProfit += realizedForEvent
					winningTrades++
				} else if realizedForEvent < 0 {
					grossLoss += realizedForEvent
					losingTrades++
				} else {
					breakevenTrades++
				}
				closedTrades++
				dayKey := event.DoneAt.Format("2006-01-02")
				dayPnL[dayKey] += realizedForEvent
			}
		}
	}

	winningDays := 0
	losingDays := 0
	flatDays := 0
	for _, pnl := range dayPnL {
		switch {
		case pnl > 0:
			winningDays++
		case pnl < 0:
			losingDays++
		default:
			flatDays++
		}
	}

	winRate := 0.0
	if closedTrades > 0 {
		winRate = float64(winningTrades) / float64(closedTrades) * 100
	}

	profitFactor := 0.0
	if grossLoss < 0 {
		profitFactor = grossProfit / -grossLoss
	}

	averageWin := 0.0
	if winningTrades > 0 {
		averageWin = grossProfit / float64(winningTrades)
	}

	averageLoss := 0.0
	if losingTrades > 0 {
		averageLoss = grossLoss / float64(losingTrades)
	}

	expectancy := 0.0
	if closedTrades > 0 {
		expectancy = periodRealizedPnL / float64(closedTrades)
	}

	status := "estimated_fifo_excluding_fees"
	if hasMoreOrders || missingOrderRefs > 0 || unmatchedSellQuantity > 0 {
		status = "estimated_fifo_incomplete"
	}

	return map[string]interface{}{
		"realized_pnl":            round2(periodRealizedPnL),
		"realized_pnl_status":     status,
		"closed_trades":           closedTrades,
		"winning_trades":          winningTrades,
		"losing_trades":           losingTrades,
		"breakeven_trades":        breakevenTrades,
		"win_rate":                round2(winRate),
		"gross_profit":            round2(grossProfit),
		"gross_loss":              round2(grossLoss),
		"profit_factor":           round2(profitFactor),
		"average_win":             round2(averageWin),
		"average_loss":            round2(averageLoss),
		"expectancy":              round2(expectancy),
		"winning_days":            winningDays,
		"losing_days":             losingDays,
		"flat_days":               flatDays,
		"missing_order_refs":      missingOrderRefs,
		"has_more_history_orders": hasMoreOrders,
		"unmatched_sell_quantity": round2(unmatchedSellQuantity),
		"method":                  "FIFO based on history executions and history orders, excluding fees",
		"metrics_complete":        !(hasMoreOrders || missingOrderRefs > 0 || unmatchedSellQuantity > 0),
	}, nil
}

func mergePnLSummaries(unrealized map[string]interface{}, realized map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(unrealized)+len(realized))
	for key, value := range unrealized {
		merged[key] = value
	}
	for key, value := range realized {
		merged[key] = value
	}
	return merged
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}
