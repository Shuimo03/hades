package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/longportapp/openapi-go/trade"
	"hades/internal/longbridge"
)

type reviewPeriod string

const (
	reviewPeriodDaily   reviewPeriod = "daily"
	reviewPeriodWeekly  reviewPeriod = "weekly"
	reviewPeriodMonthly reviewPeriod = "monthly"
	reviewPeriodYearly  reviewPeriod = "yearly"
)

func NewDailyReviewTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return newPeriodicReviewTool(lb, reviewPeriodDaily)
}

func NewWeeklyReviewTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return newPeriodicReviewTool(lb, reviewPeriodWeekly)
}

func NewMonthlyReviewTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return newPeriodicReviewTool(lb, reviewPeriodMonthly)
}

func NewYearlyReviewTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return newPeriodicReviewTool(lb, reviewPeriodYearly)
}

func newPeriodicReviewTool(lb *longbridge.Client, period reviewPeriod) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		result, err := buildPeriodicReviewResult(ctx, lb, args, period)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"result": result}, nil
	}
}

func buildPeriodicReviewResult(ctx context.Context, lb *longbridge.Client, args map[string]interface{}, period reviewPeriod) (map[string]interface{}, error) {
	location := loadReviewLocation(args["timezone"])

	start, end, err := parseReviewRange(args, location, period)
	if err != nil {
		return nil, err
	}

	accountInfo, err := buildAccountInfo(ctx, lb)
	if err != nil {
		return nil, err
	}

	orders, hasMore, err := lb.GetHistoryOrders(ctx, "", nil, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get history orders: %v", err)
	}

	executions, err := lb.GetHistoryExecutions(ctx, "", start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get history executions: %v", err)
	}

	positionReview, err := buildPositionsReview(ctx, lb, args)
	if err != nil {
		return nil, err
	}

	operations := summarizeOperations(orders, executions)
	activity := summarizeSymbolActivity(executions)
	realizedPnL, err := buildRealizedPnLSummary(ctx, lb, start, end)
	if err != nil {
		return nil, err
	}
	for key, value := range realizedPnL {
		operations[key] = value
	}
	risks := buildPeriodRisks(period, positionReview, operations, hasMore)
	unrealizedPnL, _ := positionReview["pnl_summary"].(map[string]interface{})
	pnl := mergePnLSummaries(unrealizedPnL, realizedPnL)

	return map[string]interface{}{
		"analysis_of": string(period) + "_review",
		"period": map[string]interface{}{
			"start": start.In(location).Format("2006-01-02 15:04:05"),
			"end":   end.In(location).Format("2006-01-02 15:04:05"),
		},
		"account":    accountInfo,
		"operations": operations,
		"activity":   activity,
		"pnl":        pnl,
		"positions":  positionReview,
		"risks":      risks,
		"summary":    buildPeriodSummary(period, operations, positionReview, pnl, risks),
	}, nil
}

func loadReviewLocation(raw interface{}) *time.Location {
	timezone, _ := raw.(string)
	if strings.TrimSpace(timezone) == "" {
		timezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Local
	}
	return location
}

func parseReviewRange(args map[string]interface{}, location *time.Location, period reviewPeriod) (time.Time, time.Time, error) {
	start, err := parseOptionalUnixMillis(args["start"])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start parameter: %v", err)
	}
	end, err := parseOptionalUnixMillis(args["end"])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end parameter: %v", err)
	}

	now := time.Now().In(location)
	if start.IsZero() && end.IsZero() {
		return defaultPeriodStart(now, period), now, nil
	}
	if start.IsZero() {
		start = defaultPeriodStart(end.In(location), period)
	}
	if end.IsZero() {
		end = now
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end must be after start")
	}
	return start, end, nil
}

func defaultPeriodStart(t time.Time, period reviewPeriod) time.Time {
	switch period {
	case reviewPeriodDaily:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case reviewPeriodMonthly:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	case reviewPeriodYearly:
		return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	default:
		return startOfWeek(t)
	}
}

func startOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := t.AddDate(0, 0, -(weekday - 1))
	return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
}

func buildAccountInfo(ctx context.Context, lb *longbridge.Client) (map[string]interface{}, error) {
	balances, err := lb.GetAccountInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %v", err)
	}
	if len(balances) == 0 {
		return map[string]interface{}{}, nil
	}

	balance := balances[0]
	return map[string]interface{}{
		"currency":   balance.Currency,
		"total_cash": fmt.Sprintf("%v", balance.TotalCash),
		"net_assets": fmt.Sprintf("%v", balance.NetAssets),
		"risk_level": fmt.Sprintf("%v", balance.RiskLevel),
	}, nil
}

func buildPositionsReview(ctx context.Context, lb *longbridge.Client, args map[string]interface{}) (map[string]interface{}, error) {
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
	tradeSession, err := parseTradeSessionArg(args["trade_session"])
	if err != nil {
		return nil, err
	}

	snapshotsWithPnL, err := buildPositionSnapshots(ctx, lb)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(snapshotsWithPnL))
	for _, snapshotWithPnL := range snapshotsWithPnL {
		position := snapshotWithPnL.Position

		snapshots := make([]trendSnapshot, 0, len(periods))
		for _, periodLabel := range periods {
			snapshot, err := analyzeTrendPeriod(ctx, lb, position.Symbol, periodLabel, lookback, tradeSession)
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
			"cost_price":         snapshotWithPnL.CostPrice,
			"last_price":         snapshotWithPnL.LastPrice,
			"cost_basis":         snapshotWithPnL.CostBasis,
			"market_value":       snapshotWithPnL.MarketValue,
			"unrealized_pnl":     snapshotWithPnL.UnrealizedPnL,
			"unrealized_pnl_pct": snapshotWithPnL.UnrealizedPnLPct,
			"trend":              trend,
			"score":              score,
			"signals":            signals,
			"risks":              risks,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		left, _ := items[i]["score"].(int)
		right, _ := items[j]["score"].(int)
		return left < right
	})

	return map[string]interface{}{
		"count":       len(items),
		"pnl_summary": summarizePositionSnapshots(snapshotsWithPnL),
		"items":       items,
	}, nil
}

func summarizeOperations(orders []*trade.Order, executions []*trade.Execution) map[string]interface{} {
	filledOrders := 0
	canceledOrders := 0
	rejectedOrders := 0
	buyCount := 0
	sellCount := 0

	for _, order := range orders {
		if order == nil {
			continue
		}
		switch order.Status {
		case trade.OrderFilledStatus, trade.OrderPartialFilledStatus:
			filledOrders++
		case trade.OrderCanceledStatus:
			canceledOrders++
		case trade.OrderRejectedStatus:
			rejectedOrders++
		}

		switch order.Side {
		case trade.OrderSideBuy:
			buyCount++
		case trade.OrderSideSell:
			sellCount++
		}
	}

	return map[string]interface{}{
		"orders_total":      len(orders),
		"executions_total":  len(executions),
		"filled_orders":     filledOrders,
		"canceled_orders":   canceledOrders,
		"rejected_orders":   rejectedOrders,
		"buy_orders":        buyCount,
		"sell_orders":       sellCount,
		"net_order_balance": buyCount - sellCount,
	}
}

func summarizeSymbolActivity(executions []*trade.Execution) []map[string]interface{} {
	type symbolStats struct {
		symbol    string
		trades    int
		quantity  float64
		notional  float64
		lastTrade time.Time
	}

	statsMap := make(map[string]*symbolStats)
	for _, execution := range executions {
		if execution == nil {
			continue
		}

		stat, ok := statsMap[execution.Symbol]
		if !ok {
			stat = &symbolStats{symbol: execution.Symbol}
			statsMap[execution.Symbol] = stat
		}

		stat.trades++
		stat.quantity += parseStringFloat(execution.Quantity)
		stat.notional += parseStringFloat(execution.Quantity) * decimalFloat(execution.Price)
		if execution.TradeDoneAt.After(stat.lastTrade) {
			stat.lastTrade = execution.TradeDoneAt
		}
	}

	items := make([]map[string]interface{}, 0, len(statsMap))
	for _, stat := range statsMap {
		items = append(items, map[string]interface{}{
			"symbol":        stat.symbol,
			"trades":        stat.trades,
			"quantity":      round2(stat.quantity),
			"notional":      round2(stat.notional),
			"last_trade_at": stat.lastTrade.Format("2006-01-02 15:04:05"),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		left, _ := items[i]["trades"].(int)
		right, _ := items[j]["trades"].(int)
		return left > right
	})
	return items
}

func buildPeriodRisks(period reviewPeriod, positionReview map[string]interface{}, operations map[string]interface{}, hasMore bool) []string {
	risks := make([]string, 0, 6)

	if hasMore {
		risks = append(risks, fmt.Sprintf("历史订单结果存在分页，当前%s可能未覆盖全部订单。", reviewPeriodLabel(period)))
	}

	buyOrders, _ := operations["buy_orders"].(int)
	sellOrders, _ := operations["sell_orders"].(int)
	threshold := activityRiskThreshold(period)
	if buyOrders+sellOrders >= threshold {
		risks = append(risks, fmt.Sprintf("%s交易频率较高，需关注是否存在过度交易。", reviewPeriodLabel(period)))
	}

	items, _ := positionReview["items"].([]map[string]interface{})
	weakCount := 0
	for _, item := range items {
		score, _ := item["score"].(int)
		if score <= 40 {
			weakCount++
		}
	}
	if weakCount > 0 {
		risks = append(risks, fmt.Sprintf("当前有 %d 个持仓趋势偏弱，建议检查止损和仓位。", weakCount))
	}

	if pnlSummary, ok := positionReview["pnl_summary"].(map[string]interface{}); ok {
		totalPnL, _ := pnlSummary["unrealized_pnl"].(float64)
		losingPositions, _ := pnlSummary["losing_positions"].(int)
		if totalPnL < 0 {
			risks = append(risks, fmt.Sprintf("当前持仓浮动盈亏为 %.2f，需关注组合回撤。", totalPnL))
		}
		if losingPositions >= 3 {
			risks = append(risks, fmt.Sprintf("当前有 %d 个亏损持仓，建议复核持仓集中度和止损纪律。", losingPositions))
		}
	}

	if realizedPnL, ok := operations["realized_pnl"].(float64); ok && realizedPnL < 0 {
		risks = append(risks, fmt.Sprintf("%s已实现盈亏为 %.2f，建议复核近期卖出纪律。", reviewPeriodLabel(period), realizedPnL))
	}

	if len(risks) == 0 {
		risks = append(risks, fmt.Sprintf("%s未发现明显结构性风险，但仍需结合仓位计划继续跟踪。", reviewPeriodLabel(period)))
	}
	return risks
}

func buildPeriodSummary(period reviewPeriod, operations map[string]interface{}, positionReview map[string]interface{}, pnl map[string]interface{}, risks []string) string {
	ordersTotal, _ := operations["orders_total"].(int)
	executionsTotal, _ := operations["executions_total"].(int)
	buyOrders, _ := operations["buy_orders"].(int)
	sellOrders, _ := operations["sell_orders"].(int)
	count, _ := positionReview["count"].(int)
	unrealizedPnL, _ := pnl["unrealized_pnl"].(float64)
	unrealizedPnLPct, _ := pnl["unrealized_pnl_pct"].(float64)
	realizedPnL, hasRealized := pnl["realized_pnl"].(float64)

	summary := fmt.Sprintf("%s共记录 %d 笔订单、%d 笔成交，买单 %d 笔，卖单 %d 笔。当前持仓 %d 只。",
		reviewPeriodLabel(period), ordersTotal, executionsTotal, buyOrders, sellOrders, count)
	if pnl != nil {
		summary += fmt.Sprintf(" 当前持仓浮动盈亏 %.2f (%.2f%%)。", unrealizedPnL, unrealizedPnLPct)
		if hasRealized {
			summary += fmt.Sprintf(" 区间已实现盈亏 %.2f。", realizedPnL)
		}
	}
	if len(risks) > 0 {
		summary += " 重点风险: " + risks[0]
	}
	return summary
}

func reviewPeriodLabel(period reviewPeriod) string {
	switch period {
	case reviewPeriodDaily:
		return "本日"
	case reviewPeriodMonthly:
		return "本月"
	case reviewPeriodYearly:
		return "本年"
	default:
		return "本周"
	}
}

func activityRiskThreshold(period reviewPeriod) int {
	switch period {
	case reviewPeriodDaily:
		return 8
	case reviewPeriodMonthly:
		return 50
	case reviewPeriodYearly:
		return 200
	default:
		return 20
	}
}

func parseStringFloat(value string) float64 {
	f, _ := parseStringFloat64(value)
	return f
}

func decimalFloat(value interface{ Float64() (float64, bool) }) float64 {
	if value == nil {
		return 0
	}
	f, _ := value.Float64()
	return f
}
