package tools

import (
	"context"
	"fmt"
	"hades/internal/longbridge"
	"time"
)

func NewAccountInfoTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		balances, err := lb.GetAccountInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get account info: %v", err)
		}
		if len(balances) == 0 {
			return map[string]interface{}{"result": "未获取到账户信息"}, nil
		}

		var result string
		for _, b := range balances {
			result += fmt.Sprintf(`账户信息:
- 货币: %s
- 总现金: %v
- 风险等级: %s
- 净资产: %v
`,
				b.Currency, b.TotalCash, b.RiskLevel, b.NetAssets,
			)
			// Add cash info
			for _, ci := range b.CashInfos {
				result += fmt.Sprintf(`  %s:
    可用现金: %v, 冻结现金: %v, 待结现金: %v
`,
					ci.Currency, ci.AvailableCash, ci.FrozenCash, ci.SettlingCash,
				)
			}
		}

		return map[string]interface{}{"result": result}, nil
	}
}

func NewPositionsTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		snapshots, err := buildPositionSnapshots(ctx, lb)
		if err != nil {
			return nil, err
		}
		if len(snapshots) == 0 {
			return map[string]interface{}{
				"result": map[string]interface{}{
					"count":   0,
					"summary": summarizePositionSnapshots(snapshots),
					"items":   []map[string]interface{}{},
				},
			}, nil
		}

		items := make([]map[string]interface{}, 0, len(snapshots))
		for _, snapshot := range snapshots {
			position := snapshot.Position
			items = append(items, map[string]interface{}{
				"symbol":             position.Symbol,
				"symbol_name":        position.SymbolName,
				"quantity":           round2(snapshot.Quantity),
				"available_quantity": round2(snapshot.AvailableQuantity),
				"currency":           position.Currency,
				"market":             fmt.Sprintf("%v", position.Market),
				"cost_price":         snapshot.CostPrice,
				"last_price":         snapshot.LastPrice,
				"cost_basis":         snapshot.CostBasis,
				"market_value":       snapshot.MarketValue,
				"unrealized_pnl":     snapshot.UnrealizedPnL,
				"unrealized_pnl_pct": snapshot.UnrealizedPnLPct,
				"has_quote":          snapshot.HasQuote,
				"price_session":      snapshot.PriceSession,
				"price_timestamp":    snapshot.PriceTimestamp,
				"price_time":         formatQuoteTimestamp(snapshot.PriceTimestamp),
			})
		}

		return map[string]interface{}{
			"result": map[string]interface{}{
				"count":   len(items),
				"summary": summarizePositionSnapshots(snapshots),
				"items":   items,
			},
		}, nil
	}
}

func formatQuoteTimestamp(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.UnixMilli(ts).Format("2006-01-02 15:04:05")
}
