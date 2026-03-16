package tools

import (
	"context"
	"fmt"
	"time"

	"hades/internal/longbridge"
)

func NewQuoteTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		symbolsI, ok := args["symbols"]
		if !ok {
			return nil, fmt.Errorf("missing symbols parameter")
		}
		symbolsStr, ok := symbolsI.(string)
		if !ok {
			return nil, fmt.Errorf("symbols must be a string")
		}

		symbols := splitSymbols(symbolsStr)
		quotes, err := lb.GetQuote(ctx, symbols)
		if err != nil {
			return nil, fmt.Errorf("failed to get quote: %v", err)
		}
		if quotes == nil || len(quotes) == 0 {
			return map[string]interface{}{"result": "未获取到行情数据"}, nil
		}

		var result string
		for _, q := range quotes {
			if q == nil {
				continue
			}
			effectiveQuote := longbridge.ResolveEffectiveQuote(q, longbridge.QuoteSessionScopeExtended)
			if !effectiveQuote.HasQuote {
				result += fmt.Sprintf("%s: 未获取到可用行情\n", q.Symbol)
				continue
			}

			openText := "-"
			if effectiveQuote.HasOpen {
				openText = fmt.Sprintf("%.2f", effectiveQuote.Open)
			}
			priceTime := formatQuoteTimestamp(effectiveQuote.TimestampMillis)
			if priceTime == "" {
				priceTime = "-"
			}
			result += fmt.Sprintf("%s: 最新价=%.2f, 时段=%s, 时间=%s, 开盘=%s, 最高=%.2f, 最低=%.2f, 成交量=%d\n",
				q.Symbol,
				effectiveQuote.Price,
				longbridge.QuoteSessionDisplayName(effectiveQuote.Session),
				priceTime,
				openText,
				effectiveQuote.High,
				effectiveQuote.Low,
				effectiveQuote.Volume)
		}

		return map[string]interface{}{"result": result}, nil
	}
}

func NewQuoteInfoTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		symbolsI, ok := args["symbols"]
		if !ok {
			return nil, fmt.Errorf("missing symbols parameter")
		}
		symbolsStr, ok := symbolsI.(string)
		if !ok {
			return nil, fmt.Errorf("symbols must be a string")
		}

		symbols := splitSymbols(symbolsStr)
		infos, err := lb.GetQuoteInfo(ctx, symbols)
		if err != nil {
			return nil, fmt.Errorf("failed to get quote info: %v", err)
		}
		if len(infos) == 0 {
			return map[string]interface{}{"result": "未获取到股票信息"}, nil
		}

		var result string
		for _, info := range infos {
			result += fmt.Sprintf("%s: 名称=%s, 市场=%s, 货币=%s, 每手股数=%d\n",
				info.Symbol, info.NameCn, info.Exchange, info.Currency, info.LotSize)
		}

		return map[string]interface{}{"result": result}, nil
	}
}

func NewDepthTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		symbolI, ok := args["symbol"]
		if !ok {
			return nil, fmt.Errorf("missing symbol parameter")
		}
		symbol, ok := symbolI.(string)
		if !ok {
			return nil, fmt.Errorf("symbol must be a string")
		}

		depth, err := lb.GetDepth(ctx, symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get depth: %v", err)
		}
		if depth == nil {
			return map[string]interface{}{"result": "未获取到深度数据"}, nil
		}

		result := fmt.Sprintf("%s 买卖盘口:\n", symbol)
		result += "--- 卖盘 (Ask) ---\n"
		for i := len(depth.Ask) - 1; i >= 0; i-- {
			a := depth.Ask[i]
			result += fmt.Sprintf("价格: %v, 数量: %d\n", a.Price, a.Volume)
		}
		result += "--- 买盘 (Bid) ---\n"
		for _, b := range depth.Bid {
			result += fmt.Sprintf("价格: %v, 数量: %d\n", b.Price, b.Volume)
		}

		return map[string]interface{}{"result": result}, nil
	}
}

func NewTradesTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		symbolI, ok := args["symbol"]
		if !ok {
			return nil, fmt.Errorf("missing symbol parameter")
		}
		symbol, ok := symbolI.(string)
		if !ok {
			return nil, fmt.Errorf("symbol must be a string")
		}

		startMillis, hasStart, err := parseOptionalInt64(args["start"])
		if err != nil {
			return nil, fmt.Errorf("start must be a number: %v", err)
		}
		endMillis, hasEnd, err := parseOptionalInt64(args["end"])
		if err != nil {
			return nil, fmt.Errorf("end must be a number: %v", err)
		}
		count, ok, err := parseOptionalInt(args["count"])
		if err != nil {
			return nil, fmt.Errorf("count must be a number: %v", err)
		}
		if !ok {
			count = 100
		}
		if count <= 0 {
			return nil, fmt.Errorf("count must be greater than 0")
		}

		trades, err := lb.GetTrades(ctx, symbol, int32(count))
		if err != nil {
			return nil, fmt.Errorf("failed to get trades: %v", err)
		}
		if len(trades) == 0 {
			return map[string]interface{}{"result": "未获取到成交数据"}, nil
		}

		filtered := make([]string, 0, len(trades))
		for _, t := range trades {
			if t == nil {
				continue
			}
			tradeMillis := timestampToMillis(t.Timestamp)
			if hasStart && tradeMillis < startMillis {
				continue
			}
			if hasEnd && tradeMillis > endMillis {
				continue
			}
			filtered = append(filtered, fmt.Sprintf("时间: %s, 价格: %s, 成交量: %d",
				time.UnixMilli(tradeMillis).Format("2006-01-02 15:04:05"), t.Price, t.Volume))
			if len(filtered) >= count {
				break
			}
		}
		if len(filtered) == 0 {
			return map[string]interface{}{"result": "未获取到成交数据"}, nil
		}

		return map[string]interface{}{"result": joinLines(filtered)}, nil
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

func joinLines(lines []string) string {
	result := ""
	for _, line := range lines {
		result += line + "\n"
	}
	return result
}
