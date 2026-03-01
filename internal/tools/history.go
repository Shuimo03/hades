package tools

import (
	"context"
	"fmt"

	"github.com/longportapp/openapi-go/quote"
	"hades/internal/longbridge"
)

func NewCandlesticksTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		symbolI, ok := args["symbol"]
		if !ok {
			return nil, fmt.Errorf("missing symbol parameter")
		}
		symbol, ok := symbolI.(string)
		if !ok {
			return nil, fmt.Errorf("symbol must be a string")
		}

		period := quote.Period(1) // Default to daily
		if periodI, ok := args["period"]; ok {
			periodStr, ok := periodI.(string)
			if ok {
				period = parsePeriod(periodStr)
			}
		}

		count := int32(100)
		if countI, ok := args["count"]; ok {
			switch v := countI.(type) {
			case float64:
				count = int32(v)
			case int:
				count = int32(v)
			}
		}

		candles, err := lb.GetCandlesticks(ctx, symbol, period, count)
		if err != nil {
			return nil, fmt.Errorf("failed to get candlesticks: %v", err)
		}

		var result string
		for _, c := range candles {
			result += fmt.Sprintf("时间: %d, 开盘: %v, 最高: %v, 最低: %v, 收盘: %v, 成交量: %d\n",
				c.Timestamp, c.Open, c.High, c.Low, c.Close, c.Volume)
		}

		return map[string]interface{}{"result": result}, nil
	}
}

func parsePeriod(p string) quote.Period {
	switch p {
	case "1m":
		return quote.Period(3) // 1 minute
	case "5m":
		return quote.Period(4) // 5 minutes
	case "15m":
		return quote.Period(5) // 15 minutes
	case "30m":
		return quote.Period(6) // 30 minutes
	case "1h":
		return quote.Period(7) // 1 hour
	case "2h":
		return quote.Period(8) // 2 hours
	case "4h":
		return quote.Period(9) // 4 hours
	case "1d", "day":
		return quote.Period(1) // 1 day
	case "1w", "week":
		return quote.Period(10) // 1 week
	case "1M", "month":
		return quote.Period(11) // 1 month
	default:
		return quote.Period(1)
	}
}
