package tools

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"hades/internal/okx"
)

// OKX market tools

func NewOkxTickerTool(client *okx.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		instIDI, ok := args["inst_id"]
		if !ok {
			return nil, fmt.Errorf("missing inst_id parameter")
		}
		instID, ok := instIDI.(string)
		if !ok {
			return nil, fmt.Errorf("inst_id must be a string")
		}

		ticker, err := client.GetTicker(ctx, instID)
		if err != nil {
			return nil, fmt.Errorf("failed to get ticker: %v", err)
		}
		if ticker == nil {
			return map[string]interface{}{"result": "未获取到行情"}, nil
		}

		result := fmt.Sprintf("%s 最新价=%s 买一=%s/%s 卖一=%s/%s 24h高=%s 低=%s 量=%s", ticker.InstID, ticker.Last, ticker.BidPx, ticker.BidSz, ticker.AskPx, ticker.AskSz, ticker.High24h, ticker.Low24h, ticker.Vol24h)
		return map[string]interface{}{"result": result, "raw": ticker}, nil
	}
}

func NewOkxOrderBookTool(client *okx.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		instIDI, ok := args["inst_id"]
		if !ok {
			return nil, fmt.Errorf("missing inst_id parameter")
		}
		instID, ok := instIDI.(string)
		if !ok {
			return nil, fmt.Errorf("inst_id must be a string")
		}
		depth, _, err := parseOptionalInt(args["depth"])
		if err != nil {
			return nil, fmt.Errorf("depth must be a number")
		}

		book, err := client.GetOrderBook(ctx, instID, depth)
		if err != nil {
			return nil, fmt.Errorf("failed to get orderbook: %v", err)
		}
		if book == nil {
			return map[string]interface{}{"result": "未获取到盘口"}, nil
		}

		return map[string]interface{}{"result": map[string]interface{}{"asks": book.Asks, "bids": book.Bids, "ts": book.Ts}}, nil
	}
}

func NewOkxTradesTool(client *okx.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		instIDI, ok := args["inst_id"]
		if !ok {
			return nil, fmt.Errorf("missing inst_id parameter")
		}
		instID, ok := instIDI.(string)
		if !ok {
			return nil, fmt.Errorf("inst_id must be a string")
		}
		limit, _, err := parseOptionalInt(args["limit"])
		if err != nil {
			return nil, fmt.Errorf("limit must be a number")
		}

		trades, err := client.GetTrades(ctx, instID, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to get trades: %v", err)
		}
		if len(trades) == 0 {
			return map[string]interface{}{"result": "暂无成交"}, nil
		}

		lines := make([]string, 0, len(trades))
		for _, t := range trades {
			ts, _ := strconv.ParseInt(t.Ts, 10, 64)
			lines = append(lines, fmt.Sprintf("时间:%s 方向:%s 价格:%s 数量:%s", time.UnixMilli(ts).Format("2006-01-02 15:04:05"), t.Side, t.Px, t.Sz))
		}
		return map[string]interface{}{"result": joinLines(lines)}, nil
	}
}

func NewOkxCandlesTool(client *okx.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		instID, _ := args["inst_id"].(string)
		if instID == "" {
			return nil, fmt.Errorf("missing inst_id parameter")
		}
		bar, _ := args["bar"].(string)
		limit, _, err := parseOptionalInt(args["limit"])
		if err != nil {
			return nil, fmt.Errorf("limit must be a number")
		}

		candles, err := client.GetCandles(ctx, instID, bar, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to get candles: %v", err)
		}
		if len(candles) == 0 {
			return map[string]interface{}{"result": "暂无K线数据"}, nil
		}

		return map[string]interface{}{"result": map[string]interface{}{"inst_id": instID, "bar": bar, "count": len(candles), "items": candles}}, nil
	}
}

// Account
func NewOkxBalancesTool(client *okx.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		balances, err := client.GetBalances(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get balances: %v", err)
		}
		if len(balances) == 0 {
			return map[string]interface{}{"result": "暂无可用资产"}, nil
		}
		return map[string]interface{}{"result": balances}, nil
	}
}

// Trade / order
func NewOkxPlaceOrderTool(client *okx.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		instID, _ := args["inst_id"].(string)
		side, _ := args["side"].(string)
		ordType, _ := args["ord_type"].(string)
		szRaw, _ := args["sz"].(string)
		if instID == "" || side == "" || ordType == "" || szRaw == "" {
			return nil, fmt.Errorf("inst_id/side/ord_type/sz are required")
		}
		px, _ := args["px"].(string)
		tdMode := "cash"
		if v, ok := args["td_mode"].(string); ok && v != "" {
			tdMode = v
		}
		req := okx.PlaceOrderRequest{InstID: instID, TdMode: tdMode, Side: side, OrdType: ordType, Sz: szRaw, Px: px}
		if cl, ok := args["cl_ord_id"].(string); ok && cl != "" {
			req.ClOrdID = cl
		}
		resp, err := client.PlaceOrder(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to place order: %v", err)
		}
		if resp == nil {
			return map[string]interface{}{"result": "下单返回为空"}, nil
		}
		return map[string]interface{}{"result": fmt.Sprintf("下单成功 ordId=%s", resp.OrdID), "raw": resp}, nil
	}
}

func NewOkxCancelOrderTool(client *okx.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		instID, _ := args["inst_id"].(string)
		ordID, _ := args["ord_id"].(string)
		clOrdID, _ := args["cl_ord_id"].(string)
		if instID == "" {
			return nil, fmt.Errorf("inst_id is required")
		}
		if ordID == "" && clOrdID == "" {
			return nil, fmt.Errorf("ord_id or cl_ord_id is required")
		}
		req := okx.CancelOrderRequest{InstID: instID, OrdID: ordID, ClOrdID: clOrdID}
		resp, err := client.CancelOrder(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to cancel order: %v", err)
		}
		if resp == nil {
			return map[string]interface{}{"result": "取消返回为空"}, nil
		}
		return map[string]interface{}{"result": fmt.Sprintf("已提交撤单 ordId=%s", resp.OrdID), "raw": resp}, nil
	}
}

func NewOkxOpenOrdersTool(client *okx.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		instType, _ := args["inst_type"].(string)
		orders, err := client.GetOpenOrders(ctx, instType)
		if err != nil {
			return nil, fmt.Errorf("failed to get open orders: %v", err)
		}
		if len(orders) == 0 {
			return map[string]interface{}{"result": "暂无挂单"}, nil
		}
		return map[string]interface{}{"result": map[string]interface{}{"count": len(orders), "items": orders}}, nil
	}
}

func NewOkxOrderHistoryTool(client *okx.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		instType, _ := args["inst_type"].(string)
		limit, _, err := parseOptionalInt(args["limit"])
		if err != nil {
			return nil, fmt.Errorf("limit must be a number")
		}
		orders, err := client.GetOrderHistory(ctx, instType, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to get order history: %v", err)
		}
		if len(orders) == 0 {
			return map[string]interface{}{"result": "暂无历史订单"}, nil
		}
		return map[string]interface{}{"result": map[string]interface{}{"count": len(orders), "items": orders}}, nil
	}
}
