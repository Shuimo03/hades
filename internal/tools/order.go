package tools

import (
	"context"
	"fmt"

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

		orderType := trade.OrderType(1) // LO
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
		orders, err := lb.GetOrders(ctx)
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

		executions, err := lb.GetHistoryExecutions(ctx, symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get history executions: %v", err)
		}
		if executions == nil || len(executions) == 0 {
			return map[string]interface{}{"result": "暂无成交记录"}, nil
		}

		var result string
		for _, e := range executions {
			result += fmt.Sprintf("股票: %s, 订单ID: %s, 数量: %s, 价格: %v\n",
				e.Symbol, e.OrderId, e.Quantity, e.Price)
		}

		return map[string]interface{}{"result": result}, nil
	}
}

func parseOrderType(t string) trade.OrderType {
	switch t {
	case "LO":
		return trade.OrderType(1)
	case "EO":
		return trade.OrderType(2)
	case "SC":
		return trade.OrderType(3)
	case "AO":
		return trade.OrderType(4)
	case "LOC":
		return trade.OrderType(5)
	case "ELOC":
		return trade.OrderType(6)
	default:
		return trade.OrderType(1)
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
	case "day":
		return trade.TimeTypeDay
	default:
		return trade.TimeTypeDay
	}
}
