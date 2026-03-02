package tools

import (
	"context"
	"fmt"
	"time"

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
		if len(candles) == 0 {
			return map[string]interface{}{"result": "未获取到K线数据"}, nil
		}

		// Use structured JSON format for better parsing
		var result []map[string]interface{}
		for _, c := range candles {
			// Timestamp is in milliseconds
			t := time.Unix(c.Timestamp/1000, 0)
			result = append(result, map[string]interface{}{
				"datetime": t.Format("2006-01-02 15:04:05"),
				"open":     c.Open,
				"high":     c.High,
				"low":      c.Low,
				"close":    c.Close,
				"volume":   c.Volume,
			})
		}

		return map[string]interface{}{"result": result}, nil
	}
}

func parsePeriod(p string) quote.Period {
	switch p {
	case "1m", "1min":
		return quote.Period(3) // 1 minute
	case "5m", "5min":
		return quote.Period(4) // 5 minutes
	case "15m", "15min":
		return quote.Period(5) // 15 minutes
	case "30m", "30min":
		return quote.Period(6) // 30 minutes
	case "1h", "1hour":
		return quote.Period(7) // 1 hour
	case "2h", "2hour":
		return quote.Period(8) // 2 hours
	case "4h", "4hour":
		return quote.Period(9) // 4 hours
	case "1d", "day", "daily":
		return quote.Period(1) // 1 day
	case "1w", "week", "weekly":
		return quote.Period(2) // 1 week (changed from 10)
	case "1M", "month", "monthly":
		return quote.Period(11) // 1 month
	default:
		return quote.Period(1)
	}
}
