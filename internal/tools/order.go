package tools

import (
	"context"
	"fmt"

	"github.com/longportapp/openapi-go/trade"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shopspring/decimal"
	"hades/internal/longbridge"
)

// SubmitOrderInput 提交订单
type SubmitOrderInput struct {
	Symbol      string  `json:"symbol" jsonschema:"title=symbol,description=股票代码，如: 700.HK"`
	OrderType   string  `json:"order_type" jsonschema:"title=order_type,description=订单类型: LO(限价),EO(增强限价),SC(市价),AO(竞价),LOC(竞价限价),ELOC(增强竞价限价)"`
	Side        string  `json:"side" jsonschema:"title=side,description=交易方向: buy(买入),sell(卖出)"`
	Quantity    int     `json:"quantity" jsonschema:"title=quantity,description=数量"`
	Price       float64 `json:"price" jsonschema:"title=price,description=价格，SC市价单填0"`
	TimeInForce string  `json:"time_in_force" jsonschema:"title=time_in_force,description=有效期: day(当日有效),ioc(立即成交剩余取消),fok(全部成交或取消)"`
	Remark      string  `json:"remark" jsonschema:"title=remark,description=备注(可选)"`
}

type SubmitOrderOutput struct {
	Result string `json:"result" jsonschema:"title=result,description=订单提交结果"`
}

func NewSubmitOrderTool(lb *longbridge.Client) func(ctx context.Context, req *mcp.CallToolRequest, input SubmitOrderInput) (*mcp.CallToolResult, SubmitOrderOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input SubmitOrderInput) (*mcp.CallToolResult, SubmitOrderOutput, error) {
		orderType := parseOrderType(input.OrderType)
		if orderType == 0 {
			return nil, SubmitOrderOutput{Result: "Error: invalid order type"}, nil
		}

		side := parseSide(input.Side)
		if side == 0 {
			return nil, SubmitOrderOutput{Result: "Error: invalid side"}, nil
		}

		tif := parseTimeInForce(input.TimeInForce)
		if tif == 0 {
			tif = trade.TimeTypeDay
		}

		order := &trade.SubmitOrderRequest{
			Symbol:            input.Symbol,
			OrderType:         orderType,
			Side:              side,
			SubmittedQuantity: input.Quantity,
			TimeInForce:       tif,
			SubmittedPrice:    decimal.NewFromFloat(input.Price),
		}

		if input.Remark != "" {
			order.Remark = &input.Remark
		}

		orderID, err := lb.SubmitOrder(ctx, order)
		if err != nil {
			return nil, SubmitOrderOutput{Result: fmt.Sprintf("Error: %v", err)}, nil
		}

		return nil, SubmitOrderOutput{Result: fmt.Sprintf("订单提交成功! 订单ID: %s", orderID)}, nil
	}
}

// CancelOrderInput 取消订单
type CancelOrderInput struct {
	OrderID string `json:"order_id" jsonschema:"title=order_id,description=订单ID"`
}

type CancelOrderOutput struct {
	Result string `json:"result" jsonschema:"title=result,description=取消结果"`
}

func NewCancelOrderTool(lb *longbridge.Client) func(ctx context.Context, req *mcp.CallToolRequest, input CancelOrderInput) (*mcp.CallToolResult, CancelOrderOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CancelOrderInput) (*mcp.CallToolResult, CancelOrderOutput, error) {
		err := lb.CancelOrder(ctx, input.OrderID)
		if err != nil {
			return nil, CancelOrderOutput{Result: fmt.Sprintf("Error: %v", err)}, nil
		}

		return nil, CancelOrderOutput{Result: fmt.Sprintf("订单 %s 已取消", input.OrderID)}, nil
	}
}

// OrdersInput 查询订单
type OrdersInput struct {
	Status string `json:"status" jsonschema:"title=status,description=订单状态: all(全部),filled(已成交),cancelled(已取消),pending(待成交),failed(失败)"`
}

type OrdersOutput struct {
	Result string `json:"result" jsonschema:"title=result,description=订单列表"`
}

func NewOrdersTool(lb *longbridge.Client) func(ctx context.Context, req *mcp.CallToolRequest, input OrdersInput) (*mcp.CallToolResult, OrdersOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input OrdersInput) (*mcp.CallToolResult, OrdersOutput, error) {
		orders, err := lb.GetOrders(ctx, input.Status)
		if err != nil {
			return nil, OrdersOutput{Result: fmt.Sprintf("Error: %v", err)}, nil
		}

		if len(orders) == 0 {
			return nil, OrdersOutput{Result: "暂无订单"}, nil
		}

		var result string
		for _, o := range orders {
			result += fmt.Sprintf(`订单ID: %s
  股票: %s, 方向: %s, 类型: %s
  数量: %d, 价格: %.2f, 状态: %s
  已成交: %d, 成交价: %.2f
  时间: %d
`,
				o.OrderID, o.Symbol, o.Side, o.OrderType,
				o.SubmittedQuantity, o.SubmittedPrice, o.Status,
				o.FilledQuantity, o.FilledPrice, o.SubmitTime,
			)
		}

		return nil, OrdersOutput{Result: result}, nil
	}
}

// OrderDetailInput 订单详情
type OrderDetailInput struct {
	OrderID string `json:"order_id" jsonschema:"title=order_id,description=订单ID"`
}

type OrderDetailOutput struct {
	Result string `json:"result" jsonschema:"title=result,description=订单详情"`
}

func NewOrderDetailTool(lb *longbridge.Client) func(ctx context.Context, req *mcp.CallToolRequest, input OrderDetailInput) (*mcp.CallToolResult, OrderDetailOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input OrderDetailInput) (*mcp.CallToolResult, OrderDetailOutput, error) {
		detail, err := lb.GetOrderDetail(ctx, input.OrderID)
		if err != nil {
			return nil, OrderDetailOutput{Result: fmt.Sprintf("Error: %v", err)}, nil
		}

		result := fmt.Sprintf(`订单详情:
  订单ID: %s
  股票: %s, 市场: %s
  方向: %s, 类型: %s
  数量: %d, 价格: %.2f
  状态: %s, 有效期: %s
  已成交: %d, 成交价: %.2f
  提交时间: %d
  更新时间: %d
`,
			detail.OrderID, detail.Symbol, detail.Market,
			detail.Side, detail.OrderType,
			detail.SubmittedQuantity, detail.SubmittedPrice,
			detail.Status, detail.TimeInForce,
			detail.FilledQuantity, detail.FilledPrice,
			detail.SubmitTime, detail.UpdateTime,
		)

		return nil, OrderDetailOutput{Result: result}, nil
	}
}

// HistoryExecutionsInput 历史成交
type HistoryExecutionsInput struct {
	Symbol string `json:"symbol" jsonschema:"title=symbol,description=股票代码(可选)"`
	Start  int64  `json:"start" jsonschema:"title=start,description=开始时间戳(毫秒)"`
	End    int64  `json:"end" jsonschema:"title=end,description=结束时间戳(毫秒)"`
}

type HistoryExecutionsOutput struct {
	Result string `json:"result" jsonschema:"title=result,description=历史成交记录"`
}

func NewHistoryExecutionsTool(lb *longbridge.Client) func(ctx context.Context, req *mcp.CallToolRequest, input HistoryExecutionsInput) (*mcp.CallToolResult, HistoryExecutionsOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input HistoryExecutionsInput) (*mcp.CallToolResult, HistoryExecutionsOutput, error) {
		executions, err := lb.GetHistoryExecutions(ctx, input.Symbol, input.Start, input.End)
		if err != nil {
			return nil, HistoryExecutionsOutput{Result: fmt.Sprintf("Error: %v", err)}, nil
		}

		if len(executions) == 0 {
			return nil, HistoryExecutionsOutput{Result: "暂无成交记录"}, nil
		}

		var result string
		for _, e := range executions {
			result += fmt.Sprintf(`%s %s:
  成交ID: %s, 订单ID: %s
  数量: %d, 价格: %.2f, 金额: %.2f
  时间: %d
`,
				e.Symbol, e.TradeSide, e.ExecutionID, e.OrderID,
				e.Quantity, e.Price, e.Amount,
				e.ExecutionTime,
			)
		}

		return nil, HistoryExecutionsOutput{Result: result}, nil
	}
}

func parseOrderType(t string) trade.OrderType {
	switch t {
	case "LO":
		return trade.OrderTypeLO
	case "EO":
		return trade.OrderTypeEO
	case "SC":
		return trade.OrderTypeSC
	case "AO":
		return trade.OrderTypeAO
	case "LOC":
		return trade.OrderTypeLOC
	case "ELOC":
		return trade.OrderTypeELOC
	default:
		return 0
	}
}

func parseSide(s string) trade.OrderSide {
	switch s {
	case "buy":
		return trade.OrderSideBuy
	case "sell":
		return trade.OrderSideSell
	default:
		return 0
	}
}

func parseTimeInForce(t string) trade.TimeInForceType {
	switch t {
	case "day":
		return trade.TimeTypeDay
	case "ioc":
		return trade.TimeTypeIOC
	case "fok":
		return trade.TimeTypeFOK
	default:
		return 0
	}
}
