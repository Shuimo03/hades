package main

import (
	"hades/internal/longbridge"
	"hades/internal/mcp"
	"hades/internal/okx"
	"hades/internal/tools"
)

func registerAccountAndTradeTools(server *mcp.HTTPServer, lb *longbridge.Client, okxClient *okx.Client) {
	server.AddTool("get_account_info", "获取账户信息", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewAccountInfoTool(lb))

	server.AddTool("get_positions", "获取持仓信息", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewPositionsTool(lb))

	orderSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "股票代码",
			},
			"order_type": map[string]interface{}{
				"type":        "string",
				"description": "订单类型: LO,EO,SC,AO,LOC,ELOC",
			},
			"side": map[string]interface{}{
				"type":        "string",
				"description": "交易方向: buy,sell",
			},
			"quantity": map[string]interface{}{
				"type":        "integer",
				"description": "数量",
			},
			"price": map[string]interface{}{
				"type":        "number",
				"description": "价格",
			},
			"time_in_force": map[string]interface{}{
				"type":        "string",
				"description": "有效期: day,gtc,gtd",
			},
		},
	}
	server.AddTool("submit_order", "提交订单", orderSchema, tools.NewSubmitOrderTool(lb))

	server.AddTool("cancel_order", "取消订单", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"order_id": map[string]interface{}{
				"type":        "string",
				"description": "订单ID",
			},
		},
	}, tools.NewCancelOrderTool(lb))

	server.AddTool("get_orders", "查询订单列表", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"type":        "string",
				"description": "订单状态，可传 all,filled,cancelled,pending,failed 或 SDK 原始状态值",
			},
		},
	}, tools.NewOrdersTool(lb))

	server.AddTool("get_order_detail", "查询订单详情", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"order_id": map[string]interface{}{
				"type":        "string",
				"description": "订单ID",
			},
		},
	}, tools.NewOrderDetailTool(lb))

	server.AddTool("get_history_executions", "查询历史成交", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "股票代码(可选)",
			},
			"start": map[string]interface{}{
				"type":        "integer",
				"description": "开始时间戳(毫秒)",
			},
			"end": map[string]interface{}{
				"type":        "integer",
				"description": "结束时间戳(毫秒)",
			},
		},
	}, tools.NewHistoryExecutionsTool(lb))

	server.AddTool("get_today_executions", "查询今日成交", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "股票代码(可选)",
			},
			"order_id": map[string]interface{}{
				"type":        "string",
				"description": "订单ID(可选)",
			},
		},
	}, tools.NewTodayExecutionsTool(lb))

	if okxClient == nil {
		return
	}

	server.AddTool("okx_get_ticker", "获取 OKX 单币对行情", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"inst_id": map[string]interface{}{
				"type":        "string",
				"description": "交易对，如 BTC-USDT",
			},
		},
	}, tools.NewOkxTickerTool(okxClient))

	server.AddTool("okx_get_orderbook", "获取 OKX 盘口", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"inst_id": map[string]interface{}{
				"type":        "string",
				"description": "交易对，如 BTC-USDT",
			},
			"sz": map[string]interface{}{
				"type":        "integer",
				"description": "盘口档数，默认 10",
			},
		},
	}, tools.NewOkxOrderBookTool(okxClient))

	server.AddTool("okx_get_trades", "获取 OKX 最近成交", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"inst_id": map[string]interface{}{
				"type":        "string",
				"description": "交易对，如 BTC-USDT",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "数量，默认 20",
			},
		},
	}, tools.NewOkxTradesTool(okxClient))

	server.AddTool("okx_get_candles", "获取 OKX K 线", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"inst_id": map[string]interface{}{
				"type":        "string",
				"description": "交易对，如 BTC-USDT",
			},
			"bar": map[string]interface{}{
				"type":        "string",
				"description": "周期，如 1m, 5m, 1H, 1D",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "数量，默认 100",
			},
		},
	}, tools.NewOkxCandlesTool(okxClient))

	server.AddTool("okx_get_balances", "获取 OKX 账户资产", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewOkxBalancesTool(okxClient))

	okxOrderSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"inst_id": map[string]interface{}{
				"type":        "string",
				"description": "交易对，如 BTC-USDT",
			},
			"td_mode": map[string]interface{}{
				"type":        "string",
				"description": "交易模式，默认 cash",
			},
			"side": map[string]interface{}{
				"type":        "string",
				"description": "buy 或 sell",
			},
			"ord_type": map[string]interface{}{
				"type":        "string",
				"description": "订单类型，如 limit, market",
			},
			"sz": map[string]interface{}{
				"type":        "string",
				"description": "下单数量",
			},
			"px": map[string]interface{}{
				"type":        "string",
				"description": "价格，市价单可不填",
			},
		},
	}
	server.AddTool("okx_place_order", "OKX 下单", okxOrderSchema, tools.NewOkxPlaceOrderTool(okxClient))

	server.AddTool("okx_cancel_order", "OKX 撤单", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"inst_id": map[string]interface{}{"type": "string", "description": "交易对，如 BTC-USDT"},
			"ord_id":  map[string]interface{}{"type": "string", "description": "订单 ID"},
		},
	}, tools.NewOkxCancelOrderTool(okxClient))

	server.AddTool("okx_get_open_orders", "OKX 当前挂单", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"inst_id": map[string]interface{}{"type": "string", "description": "交易对，可选"},
		},
	}, tools.NewOkxOpenOrdersTool(okxClient))

	server.AddTool("okx_get_order_history", "OKX 历史订单", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"inst_id": map[string]interface{}{"type": "string", "description": "交易对，可选"},
			"limit":   map[string]interface{}{"type": "integer", "description": "数量，默认 50"},
		},
	}, tools.NewOkxOrderHistoryTool(okxClient))
}
