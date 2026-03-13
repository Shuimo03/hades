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

		startMillis, hasStart, err := parseOptionalInt64(args["start"])
		if err != nil {
			return nil, fmt.Errorf("start must be a number: %v", err)
		}
		endMillis, hasEnd, err := parseOptionalInt64(args["end"])
		if err != nil {
			return nil, fmt.Errorf("end must be a number: %v", err)
		}

		count, ok, err := parseOptionalInt32(args["count"])
		if err != nil {
			return nil, fmt.Errorf("count must be a number: %v", err)
		}
		if !ok {
			count, ok, err = parseOptionalInt32(args["size"])
			if err != nil {
				return nil, fmt.Errorf("size must be a number: %v", err)
			}
		}
		if !ok {
			if hasStart || hasEnd {
				count = 500
			} else {
				count = 100
			}
		}
		if count <= 0 {
			return nil, fmt.Errorf("count must be greater than 0")
		}

		var candles []*quote.Candlestick
		if hasStart || hasEnd {
			startDate := timePointerOrNil(startMillis, hasStart)
			endDate := timePointerOrNil(endMillis, hasEnd)

			candles, err = lb.GetHistoryCandlesticksByDate(ctx, symbol, period, startDate, endDate)
			if err != nil {
				return nil, fmt.Errorf("failed to get history candlesticks: %v", err)
			}
		} else {
			candles, err = lb.GetCandlesticks(ctx, symbol, period, count)
			if err != nil {
				return nil, fmt.Errorf("failed to get candlesticks: %v", err)
			}
		}
		if len(candles) == 0 {
			return map[string]interface{}{"result": "未获取到K线数据"}, nil
		}

		// Use structured JSON format for better parsing
		var result []map[string]interface{}
		for _, c := range candles {
			if c == nil {
				continue
			}
			candleMillis := timestampToMillis(c.Timestamp)
			if hasStart && candleMillis < startMillis {
				continue
			}
			if hasEnd && candleMillis > endMillis {
				continue
			}
			t := time.UnixMilli(candleMillis)
			result = append(result, map[string]interface{}{
				"datetime": t.Format("2006-01-02 15:04:05"),
				"open":     c.Open,
				"high":     c.High,
				"low":      c.Low,
				"close":    c.Close,
				"volume":   c.Volume,
			})
		}
		if len(result) == 0 {
			return map[string]interface{}{"result": "未获取到K线数据"}, nil
		}

		return map[string]interface{}{"result": result}, nil
	}
}

func timePointerOrNil(unixMillis int64, ok bool) *time.Time {
	if !ok {
		return nil
	}
	t := time.UnixMilli(unixMillis)
	return &t
}

func timestampToMillis(ts int64) int64 {
	if ts > 1_000_000_000_000 {
		return ts
	}
	return ts * 1000
}

func parsePeriod(p string) quote.Period {
	switch p {
	case "1m", "1min":
		return quote.PeriodOneMinute
	case "5m", "5min":
		return quote.PeriodFiveMinute
	case "15m", "15min":
		return quote.PeriodFifteenMinute
	case "30m", "30min":
		return quote.PeriodThirtyMinute
	case "1h", "1hour":
		return quote.PeriodSixtyMinute
	case "2h", "2hour":
		return quote.PeriodSixtyMinute
	case "4h", "4hour":
		return quote.PeriodSixtyMinute
	case "1d", "day", "daily":
		return quote.PeriodDay
	case "1w", "week", "weekly":
		return quote.PeriodWeek
	case "1M", "month", "monthly":
		return quote.PeriodMonth
	default:
		return quote.PeriodDay
	}
}
