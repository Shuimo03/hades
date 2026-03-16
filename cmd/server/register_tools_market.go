package main

import (
	"hades/internal/longbridge"
	"hades/internal/mcp"
	"hades/internal/tools"
)

func registerMarketTools(server *mcp.HTTPServer, lb *longbridge.Client) {
	server.AddTool("get_quote", "获取股票实时行情", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbols": map[string]interface{}{
				"type":        "string",
				"description": "股票代码，用逗号分隔，如: 700.HK,AAPL.US",
			},
		},
	}, tools.NewQuoteTool(lb))

	server.AddTool("get_quote_info", "获取股票基本信息", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbols": map[string]interface{}{
				"type":        "string",
				"description": "股票代码，用逗号分隔",
			},
		},
	}, tools.NewQuoteInfoTool(lb))

	server.AddTool("get_depth", "获取股票买卖盘口", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "股票代码，如: 700.HK",
			},
			"size": map[string]interface{}{
				"type":        "integer",
				"description": "盘口深度，默认10",
			},
		},
	}, tools.NewDepthTool(lb))

	server.AddTool("get_trades", "获取股票分时成交", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "股票代码",
			},
			"start": map[string]interface{}{
				"type":        "integer",
				"description": "开始时间戳(毫秒)",
			},
			"end": map[string]interface{}{
				"type":        "integer",
				"description": "结束时间戳(毫秒)",
			},
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "返回数量，默认100",
			},
		},
	}, tools.NewTradesTool(lb))

	server.AddTool("get_candlesticks", "获取股票K线数据", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "股票代码，如: 700.HK",
			},
			"period": map[string]interface{}{
				"type":        "string",
				"description": "K线周期: 1m,5m,15m,30m,1h,2h,4h,1d,1w,1M",
			},
			"start": map[string]interface{}{
				"type":        "integer",
				"description": "开始时间戳(毫秒)",
			},
			"end": map[string]interface{}{
				"type":        "integer",
				"description": "结束时间戳(毫秒)",
			},
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "返回数量，默认100",
			},
			"size": map[string]interface{}{
				"type":        "integer",
				"description": "返回数量别名，等同 count",
			},
			"trade_session": map[string]interface{}{
				"type":        "string",
				"description": "K线交易时段: regular 仅常规盘, all 包含盘前/盘后/夜盘",
			},
		},
	}, tools.NewCandlesticksTool(lb))

	server.AddTool("get_stock_news", "获取个股相关资讯", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "股票代码，如: AAPL.US",
			},
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "返回资讯数量，默认5",
			},
		},
	}, tools.NewStockNewsTool(lb))

	server.AddTool("generate_watchlist_plan", "生成关注股下周操作计划", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbols": map[string]interface{}{
				"type":        "string",
				"description": "股票代码，逗号分隔，如: AAPL.US,TSLA.US,1810.HK",
			},
			"group_name": map[string]interface{}{
				"type":        "string",
				"description": "关注组名称，如: us, hk, 半导体",
			},
			"group_id": map[string]interface{}{
				"type":        "string",
				"description": "关注组 ID，如: 2522760",
			},
			"news_count": map[string]interface{}{
				"type":        "integer",
				"description": "每只股票附带的资讯数量，默认3",
			},
			"lookback": map[string]interface{}{
				"type":        "integer",
				"description": "日线回看K线数量，默认120",
			},
		},
	}, tools.NewWatchlistPlanTool(lb))

	server.AddTool("get_watchlist_groups", "获取关注组列表", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewWatchlistGroupsTool(lb))
}
