package tools

import (
	"context"
	"fmt"

	"github.com/longportapp/openapi-go/trade"
	"hades/internal/longbridge"
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
		positionChannels, err := lb.GetPositions(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get positions: %v", err)
		}
		if len(positionChannels) == 0 {
			return map[string]interface{}{"result": "当前无持仓"}, nil
		}

		var allPositions []*trade.StockPosition
		for _, ch := range positionChannels {
			allPositions = append(allPositions, ch.Positions...)
		}

		if len(allPositions) == 0 {
			return map[string]interface{}{"result": "当前无持仓"}, nil
		}

		var result string
		for _, p := range allPositions {
			result += fmt.Sprintf(`%s %s:
  数量: %s, 可用: %s
  成本价: %v, 市场: %s
`,
				p.Symbol, p.SymbolName,
				p.Quantity, p.AvailableQuantity,
				p.CostPrice, p.Market,
			)
		}

		return map[string]interface{}{"result": result}, nil
	}
}
