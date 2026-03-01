package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"hades/internal/longbridge"
)

// AccountInfoInput 账户信息
type AccountInfoInput struct{}

type AccountInfoOutput struct {
	Result string `json:"result" jsonschema:"title=result,description=账户信息"`
}

func NewAccountInfoTool(lb *longbridge.Client) func(ctx context.Context, req *mcp.CallToolRequest, input AccountInfoInput) (*mcp.CallToolResult, AccountInfoOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input AccountInfoInput) (*mcp.CallToolResult, AccountInfoOutput, error) {
		info, err := lb.GetAccountInfo(ctx)
		if err != nil {
			return nil, AccountInfoOutput{Result: fmt.Sprintf("Error: %v", err)}, nil
		}

		result := fmt.Sprintf(`账户信息:
- 账户 ID: %s
- 现金: %.2f %s
- 购买力: %.2f %s
- 冻结金额: %.2f %s
- 负债: %.2f %s
- 市场价值: %.2f %s
- 总资产: %.2f %s
`,
			info.AccountID,
			info.Cash, info.Currency,
			info.BuyingPower, info.Currency,
			info.FrozenCash, info.Currency,
			info.Liability, info.Currency,
			info.MarketValue, info.Currency,
			info.TotalAssets, info.Currency,
		)

		return nil, AccountInfoOutput{Result: result}, nil
	}
}

// PositionsInput 持仓查询
type PositionsInput struct{}

type PositionsOutput struct {
	Result string `json:"result" jsonschema:"title=result,description=持仓信息"`
}

func NewPositionsTool(lb *longbridge.Client) func(ctx context.Context, req *mcp.CallToolRequest, input PositionsInput) (*mcp.CallToolResult, PositionsOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input PositionsInput) (*mcp.CallToolResult, PositionsOutput, error) {
		positions, err := lb.GetPositions(ctx)
		if err != nil {
			return nil, PositionsOutput{Result: fmt.Sprintf("Error: %v", err)}, nil
		}

		if len(positions) == 0 {
			return nil, PositionsOutput{Result: "当前无持仓"}, nil
		}

		var result string
		for _, p := range positions {
			result += fmt.Sprintf(`%s %s:
  数量: %d, 可用: %d, 成本价: %.2f, 当前价: %.2f
  盈亏: %.2f (%.2f%%), 持仓市值: %.2f %s
`,
				p.Symbol, p.Name,
				p.Quantity, p.AvailableQuantity, p.CostPrice, p.ClosePrice,
				p.Pnl, p.PnlRate*100, p.MarketValue, p.Currency,
			)
		}

		return nil, PositionsOutput{Result: result}, nil
	}
}
