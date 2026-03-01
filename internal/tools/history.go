package tools

import (
	"context"
	"fmt"

	"github.com/longportapp/openapi-go/quote"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"hades/internal/longbridge"
)

// CandlesticksInput K线数据
type CandlesticksInput struct {
	Symbol string `json:"symbol" jsonschema:"title=symbol,description=股票代码，如: 700.HK"`
	Period string `json:"period" jsonschema:"title=period,description=K线周期: 1m,5m,15m,30m,1h,2h,4h,1d,1w,1M"`
	Start  int64  `json:"start" jsonschema:"title=start,description=开始时间戳(毫秒)"`
	End    int64  `json:"end" jsonschema:"title=end,description=结束时间戳(毫秒)"`
	Count  int    `json:"count" jsonschema:"title=count,description=返回数量，默认100"`
}

type CandlesticksOutput struct {
	Result string `json:"result" jsonschema:"title=result,description=K线数据"`
}

func NewCandlesticksTool(lb *longbridge.Client) func(ctx context.Context, req *mcp.CallToolRequest, input CandlesticksInput) (*mcp.CallToolResult, CandlesticksOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CandlesticksInput) (*mcp.CallToolResult, CandlesticksOutput, error) {
		period := parsePeriod(input.Period)
		if period == 0 {
			period = quote.CandlestickPeriodDay
		}

		count := input.Count
		if count <= 0 {
			count = 100
		}

		candles, err := lb.GetCandlesticks(ctx, input.Symbol, period, input.Start, input.End, count)
		if err != nil {
			return nil, CandlesticksOutput{Result: fmt.Sprintf("Error: %v", err)}, nil
		}

		var result string
		for _, c := range candles {
			result += fmt.Sprintf("时间: %d, 开盘: %.2f, 最高: %.2f, 最低: %.2f, 收盘: %.2f, 成交量: %d\n",
				c.Timestamp, c.Open, c.High, c.Low, c.Close, c.Volume)
		}

		return nil, CandlesticksOutput{Result: result}, nil
	}
}

func parsePeriod(p string) quote.CandlestickPeriod {
	switch p {
	case "1m":
		return quote.CandlestickPeriodMinute
	case "5m":
		return quote.CandlestickPeriod5Minutes
	case "15m":
		return quote.CandlestickPeriod15Minutes
	case "30m":
		return quote.CandlestickPeriod30Minutes
	case "1h":
		return quote.CandlestickPeriodHour
	case "2h":
		return quote.CandlestickPeriod2Hours
	case "4h":
		return quote.CandlestickPeriod4Hours
	case "1d", "day":
		return quote.CandlestickPeriodDay
	case "1w", "week":
		return quote.CandlestickPeriodWeek
	case "1M", "month":
		return quote.CandlestickPeriodMonth
	default:
		return 0
	}
}
