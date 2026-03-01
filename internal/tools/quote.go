package tools

import (
	"context"
	"fmt"

	"github.com/longportapp/openapi-go/quote"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"hades/internal/longbridge"
)

type QuoteInput struct {
	Symbols string `json:"symbols" jsonschema:"title=symbols,description=股票代码，用逗号分隔，如: 700.HK,AAPL.US"`
}

type QuoteOutput struct {
	Result string `json:"result" jsonschema:"title=result,description=行情结果"`
}

func NewQuoteTool(lb *longbridge.Client) func(ctx context.Context, req *mcp.CallToolRequest, input QuoteInput) (*mcp.CallToolResult, QuoteOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input QuoteInput) (*mcp.CallToolResult, QuoteOutput, error) {
		symbols := splitSymbols(input.Symbols)
		quotes, err := lb.GetQuote(ctx, symbols)
		if err != nil {
			return nil, QuoteOutput{Result: fmt.Sprintf("Error: %v", err)}, nil
		}

		var result string
		for _, q := range quotes {
			result += fmt.Sprintf("%s: 最新价=%.2f, 开盘=%.2f, 最高=%.2f, 最低=%.2f, 成交量=%d, 成交额=%.2f\n",
				q.Symbol, q.LastDone, q.Open, q.High, q.Low, q.Volume, q.Turnover)
		}

		return nil, QuoteOutput{Result: result}, nil
	}
}

func splitSymbols(s string) []string {
	if s == "" {
		return nil
	}
	var symbols []string
	for _, sym := range splitAndTrim(s) {
		if sym != "" {
			symbols = append(symbols, sym)
		}
	}
	return symbols
}

func splitAndTrim(s string) []string {
	result := make([]string, 0)
	current := ""
	for _, c := range s {
		if c == ',' || c == ' ' || c == ';' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// QuoteInfoInput 获取股票基本信息
type QuoteInfoInput struct {
	Symbols string `json:"symbols" jsonschema:"title=symbols,description=股票代码，用逗号分隔"`
}

type QuoteInfoOutput struct {
	Result string `json:"result" jsonschema:"title=result,description=股票基本信息"`
}

func NewQuoteInfoTool(lb *longbridge.Client) func(ctx context.Context, req *mcp.CallToolRequest, input QuoteInfoInput) (*mcp.CallToolResult, QuoteInfoOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input QuoteInfoInput) (*mcp.CallToolResult, QuoteInfoOutput, error) {
		symbols := splitSymbols(input.Symbols)
		infos, err := lb.GetQuoteInfo(ctx, symbols)
		if err != nil {
			return nil, QuoteInfoOutput{Result: fmt.Sprintf("Error: %v", err)}, nil
		}

		var result string
		for _, info := range infos {
			result += fmt.Sprintf("%s: 名称=%s, 市场=%s, 货币=%s, 类型=%s\n",
				info.Symbol, info.Name, info.Exchange, info.Currency, info.SecurityType)
		}

		return nil, QuoteInfoOutput{Result: result}, nil
	}
}

// DepthInput 买卖盘口
type DepthInput struct {
	Symbol string `json:"symbol" jsonschema:"title=symbol,description=股票代码，如: 700.HK"`
	Size   int    `json:"size" jsonschema:"title=size,description=盘口深度，默认10"`
}

type DepthOutput struct {
	Result string `json:"result" jsonschema:"title=result,description=买卖盘口信息"`
}

func NewDepthTool(lb *longbridge.Client) func(ctx context.Context, req *mcp.CallToolRequest, input DepthInput) (*mcp.CallToolResult, DepthOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DepthInput) (*mcp.CallToolResult, DepthOutput, error) {
		size := input.Size
		if size <= 0 {
			size = 10
		}
		depth, err := lb.GetDepth(ctx, input.Symbol, size)
		if err != nil {
			return nil, DepthOutput{Result: fmt.Sprintf("Error: %v", err)}, nil
		}

		result := fmt.Sprintf("%s 买卖盘口:\n", input.Symbol)
		result += "--- 卖盘 (Ask) ---\n"
		for i := len(depth.Asks) - 1; i >= 0; i-- {
			a := depth.Asks[i]
			result += fmt.Sprintf("价格: %.2f, 数量: %d\n", a.Price, a.Quantity)
		}
		result += "--- 买盘 (Bid) ---\n"
		for _, b := range depth.Bids {
			result += fmt.Sprintf("价格: %.2f, 数量: %d\n", b.Price, b.Quantity)
		}

		return nil, DepthOutput{Result: result}, nil
	}
}

// TradesInput 分时成交
type TradesInput struct {
	Symbol string `json:"symbol" jsonschema:"title=symbol,description=股票代码"`
	Start  int64  `json:"start" jsonschema:"title=start,description=开始时间戳(毫秒)"`
	End    int64  `json:"end" jsonschema:"title=end,description=结束时间戳(毫秒)"`
	Count  int    `json:"count" jsonschema:"title=count,description=返回数量，默认100"`
}

type TradesOutput struct {
	Result string `json:"result" jsonschema:"title=result,description=分时成交数据"`
}

func NewTradesTool(lb *longbridge.Client) func(ctx context.Context, req *mcp.CallToolRequest, input TradesInput) (*mcp.CallToolResult, TradesOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input TradesInput) (*mcp.CallToolResult, TradesOutput, error) {
		count := input.Count
		if count <= 0 {
			count = 100
		}
		trades, err := lb.GetTrades(ctx, input.Symbol, input.Start, input.End, count)
		if err != nil {
			return nil, TradesOutput{Result: fmt.Sprintf("Error: %v", err)}, nil
		}

		var result string
		for _, t := range trades {
			side := "买"
			if t.Side == quote.TradeSideSell {
				side = "卖"
			}
			result += fmt.Sprintf("时间: %d, 价格: %.2f, 数量: %d, 方向: %s\n",
				t.Timestamp, t.Price, t.Quantity, side)
		}

		return nil, TradesOutput{Result: result}, nil
	}
}
