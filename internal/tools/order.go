package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/longportapp/openapi-go/trade"
	"github.com/shopspring/decimal"
	"hades/internal/longbridge"
)

func NewSubmitOrderTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		symbolI, ok := args["symbol"]
		if !ok {
			return nil, fmt.Errorf("missing symbol parameter")
		}
		symbol, ok := symbolI.(string)
		if !ok {
			return nil, fmt.Errorf("symbol must be a string")
		}

		quantityI, ok := args["quantity"]
		if !ok {
			return nil, fmt.Errorf("missing quantity parameter")
		}
		var quantity uint64
		switch v := quantityI.(type) {
		case float64:
			quantity = uint64(v)
		case int:
			quantity = uint64(v)
		default:
			return nil, fmt.Errorf("quantity must be a number")
		}

		priceI, ok := args["price"]
		var price decimal.Decimal
		if ok {
			switch v := priceI.(type) {
			case float64:
				price = decimal.NewFromFloat(v)
			case int:
				price = decimal.NewFromInt(int64(v))
			}
		}

		orderType := trade.OrderTypeLO
		if orderTypeI, ok := args["order_type"]; ok {
			if ot, ok := orderTypeI.(string); ok {
				orderType = parseOrderType(ot)
			}
		}

		side := trade.OrderSideBuy
		if sideI, ok := args["side"]; ok {
			if s, ok := sideI.(string); ok {
				side = parseSide(s)
			}
		}

		timeInForce := trade.TimeTypeDay
		if tifI, ok := args["time_in_force"]; ok {
			if t, ok := tifI.(string); ok {
				timeInForce = parseTimeInForce(t)
			}
		}

		order := &trade.SubmitOrder{
			Symbol:            symbol,
			OrderType:         orderType,
			Side:              side,
			SubmittedQuantity: quantity,
			TimeInForce:       timeInForce,
			SubmittedPrice:    price,
		}

		orderID, err := lb.SubmitOrder(ctx, order)
		if err != nil {
			return nil, fmt.Errorf("failed to submit order: %v", err)
		}

		return map[string]interface{}{"result": fmt.Sprintf("订单提交成功! 订单ID: %s", orderID)}, nil
	}
}

func NewCancelOrderTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		orderIDI, ok := args["order_id"]
		if !ok {
			return nil, fmt.Errorf("missing order_id parameter")
		}
		orderID, ok := orderIDI.(string)
		if !ok {
			return nil, fmt.Errorf("order_id must be a string")
		}

		err := lb.CancelOrder(ctx, orderID)
		if err != nil {
			return nil, fmt.Errorf("failed to cancel order: %v", err)
		}

		return map[string]interface{}{"result": fmt.Sprintf("订单 %s 已取消", orderID)}, nil
	}
}

func NewOrdersTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		orders, err := lb.GetOrders(ctx, parseOrderStatuses(args["status"]))
		if err != nil {
			return nil, fmt.Errorf("failed to get orders: %v", err)
		}
		if orders == nil || len(orders) == 0 {
			return map[string]interface{}{"result": "暂无订单"}, nil
		}

		var result string
		for _, o := range orders {
			result += fmt.Sprintf("订单ID: %s, 股票: %s, 方向: %s, 状态: %s\n",
				o.OrderId, o.Symbol, o.Side, o.Status)
		}

		return map[string]interface{}{"result": result}, nil
	}
}

func NewOrderDetailTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		orderIDI, ok := args["order_id"]
		if !ok {
			return nil, fmt.Errorf("missing order_id parameter")
		}
		orderID, ok := orderIDI.(string)
		if !ok {
			return nil, fmt.Errorf("order_id must be a string")
		}

		detail, err := lb.GetOrderDetail(ctx, orderID)
		if err != nil {
			return nil, fmt.Errorf("failed to get order detail: %v", err)
		}

		result := fmt.Sprintf("订单ID: %s, 股票: %s, 方向: %s, 状态: %s",
			detail.OrderId, detail.Symbol, detail.Side, detail.Status)

		return map[string]interface{}{"result": result}, nil
	}
}

func NewHistoryExecutionsTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		var symbol string
		if symbolI, ok := args["symbol"]; ok {
			symbol, _ = symbolI.(string)
		}

		startAt, err := parseOptionalUnixMillis(args["start"])
		if err != nil {
			return nil, fmt.Errorf("invalid start parameter: %v", err)
		}

		endAt, err := parseOptionalUnixMillis(args["end"])
		if err != nil {
			return nil, fmt.Errorf("invalid end parameter: %v", err)
		}

		executions, err := lb.GetHistoryExecutions(ctx, symbol, startAt, endAt)
		if err != nil {
			return nil, fmt.Errorf("failed to get history executions: %v", err)
		}
		if executions == nil || len(executions) == 0 {
			return map[string]interface{}{"result": "暂无成交记录"}, nil
		}

		items := make([]map[string]interface{}, 0, len(executions))
		for _, e := range executions {
			if e == nil {
				continue
			}
			items = append(items, map[string]interface{}{
				"trade_done_at":         e.TradeDoneAt.Format("2006-01-02 15:04:05"),
				"trade_done_at_unix_ms": e.TradeDoneAt.UnixMilli(),
				"symbol":                e.Symbol,
				"order_id":              e.OrderId,
				"quantity":              fmt.Sprintf("%v", e.Quantity),
				"price":                 fmt.Sprintf("%v", e.Price),
			})
		}

		query := map[string]interface{}{
			"symbol": symbol,
			"start":  nil,
			"end":    nil,
		}
		if !startAt.IsZero() {
			query["start"] = startAt.UnixMilli()
		}
		if !endAt.IsZero() {
			query["end"] = endAt.UnixMilli()
		}

		return map[string]interface{}{
			"result": map[string]interface{}{
				"summary": fmt.Sprintf("共 %d 笔成交记录", len(items)),
				"query":   query,
				"count":   len(items),
				"items":   items,
			},
		}, nil
	}
}

func NewTodayExecutionsTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		var symbol string
		if symbolI, ok := args["symbol"]; ok {
			symbol, _ = symbolI.(string)
		}

		var orderID string
		if orderIDI, ok := args["order_id"]; ok {
			orderID, _ = orderIDI.(string)
		}

		executions, err := lb.GetTodayExecutions(ctx, symbol, orderID)
		if err != nil {
			return nil, fmt.Errorf("failed to get today executions: %v", err)
		}
		if executions == nil || len(executions) == 0 {
			return map[string]interface{}{"result": "暂无今日成交记录"}, nil
		}

		items := make([]map[string]interface{}, 0, len(executions))
		for _, e := range executions {
			if e == nil {
				continue
			}
			items = append(items, map[string]interface{}{
				"trade_done_at":         e.TradeDoneAt.Format("2006-01-02 15:04:05"),
				"trade_done_at_unix_ms": e.TradeDoneAt.UnixMilli(),
				"symbol":                e.Symbol,
				"order_id":              e.OrderId,
				"quantity":              fmt.Sprintf("%v", e.Quantity),
				"price":                 fmt.Sprintf("%v", e.Price),
			})
		}

		return map[string]interface{}{
			"result": map[string]interface{}{
				"summary": fmt.Sprintf("共 %d 笔今日成交记录", len(items)),
				"query": map[string]interface{}{
					"symbol":   symbol,
					"order_id": orderID,
				},
				"count": len(items),
				"items": items,
			},
		}, nil
	}
}

func parseOrderType(t string) trade.OrderType {
	switch t {
	case "LO", "lo", "limit":
		return trade.OrderTypeLO
	case "ELO", "elo":
		return trade.OrderTypeELO
	case "MO", "mo", "market":
		return trade.OrderTypeMO
	case "AO", "ao":
		return trade.OrderTypeAO
	case "ALO", "alo":
		return trade.OrderTypeALO
	case "ODD", "odd":
		return trade.OrderTypeODD
	case "LIT", "lit":
		return trade.OrderTypeLIT
	case "MIT", "mit":
		return trade.OrderTypeMIT
	case "TSLPAMT", "tslpamt":
		return trade.OrderTypeTSLPAMT
	case "TSLPPCT", "tslppct":
		return trade.OrderTypeTSLPPCT
	case "TSMAMT", "tsmamt":
		return trade.OrderTypeTSMAMT
	case "TSMPCT", "tsmpct":
		return trade.OrderTypeTSMPCT
	case "SLO", "slo":
		return trade.OrderTypeSLO
	default:
		return trade.OrderTypeLO
	}
}

func parseSide(s string) trade.OrderSide {
	switch s {
	case "buy":
		return trade.OrderSideBuy
	case "sell":
		return trade.OrderSideSell
	default:
		return trade.OrderSideBuy
	}
}

func parseTimeInForce(t string) trade.TimeType {
	switch t {
	case "day", "DAY", "Day":
		return trade.TimeTypeDay
	case "gtc", "GTC":
		return trade.TimeTypeGTC
	case "gtd", "GTD":
		return trade.TimeTypeGTD
	default:
		return trade.TimeTypeDay
	}
}

func parseOrderStatuses(raw interface{}) []trade.OrderStatus {
	statuses := splitStringArg(raw)
	if len(statuses) == 0 {
		return nil
	}

	result := make([]trade.OrderStatus, 0, len(statuses))
	for _, status := range statuses {
		if parsed, ok := mapOrderStatus(status); ok {
			result = append(result, parsed)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func mapOrderStatus(value string) (trade.OrderStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all":
		return "", false
	case "filled":
		return trade.OrderFilledStatus, true
	case "cancelled", "canceled":
		return trade.OrderCanceledStatus, true
	case "pending":
		return trade.OrderNewStatus, true
	case "partial_filled", "partial-filled", "partialfilled":
		return trade.OrderPartialFilledStatus, true
	case "rejected", "failed":
		return trade.OrderRejectedStatus, true
	case "expired":
		return trade.OrderExpiredStatus, true
	default:
		return trade.OrderStatus(value), true
	}
}

func parseOptionalUnixMillis(raw interface{}) (time.Time, error) {
	millis, ok, err := parseOptionalInt64(raw)
	if err != nil || !ok {
		return time.Time{}, err
	}
	return time.UnixMilli(millis), nil
}
