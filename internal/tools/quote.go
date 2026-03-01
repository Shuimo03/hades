package tools

import (
	"context"
	"fmt"

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
			result += fmt.Sprintf("%s: 最新价=%s, 开盘=%s, 最高=%s, 最低=%s, 成交量=%d\n",
				q.Symbol, q.LastDone, q.Open, q.High, q.Low, q.Volume)
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

		trades, err := lb.GetTrades(ctx, symbol, 100)
		if err != nil {
			return nil, fmt.Errorf("failed to get trades: %v", err)
		}
		if len(trades) == 0 {
			return map[string]interface{}{"result": "未获取到成交数据"}, nil
		}

		var result string
		for _, t := range trades {
			result += fmt.Sprintf("时间: %d, 价格: %s, 成交量: %d\n",
				t.Timestamp, t.Price, t.Volume)
		}

		return map[string]interface{}{"result": result}, nil
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
