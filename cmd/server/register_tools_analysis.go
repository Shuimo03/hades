package main

import (
	"hades/internal/longbridge"
	"hades/internal/mcp"
	"hades/internal/tools"
)

func registerAnalysisTools(server *mcp.HTTPServer, lb *longbridge.Client) {
	trendAnalysisSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "股票代码，如: AAPL.US",
			},
			"periods": map[string]interface{}{
				"type":        "string",
				"description": "分析周期，逗号分隔，如: 1d,1h,15m",
			},
			"lookback": map[string]interface{}{
				"type":        "integer",
				"description": "回看K线数量，默认120",
			},
			"trade_session": map[string]interface{}{
				"type":        "string",
				"description": "K线交易时段: regular 仅常规盘, all 包含盘前/盘后/夜盘",
			},
		},
	}
	server.AddTool("analyze_trend", "分析单只股票走势", trendAnalysisSchema, tools.NewTrendAnalysisTool(lb))

	server.AddTool("analyze_watchlist_trends", "批量分析股票走势", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbols": map[string]interface{}{
				"type":        "string",
				"description": "股票代码，逗号分隔，如: AAPL.US,TSLA.US,700.HK",
			},
			"group_name": map[string]interface{}{
				"type":        "string",
				"description": "关注组名称，如: us, hk, 半导体",
			},
			"group_id": map[string]interface{}{
				"type":        "string",
				"description": "关注组 ID，如: 2522760",
			},
			"periods": map[string]interface{}{
				"type":        "string",
				"description": "分析周期，逗号分隔，如: 1d,1h,15m",
			},
			"lookback": map[string]interface{}{
				"type":        "integer",
				"description": "回看K线数量，默认120",
			},
			"trade_session": map[string]interface{}{
				"type":        "string",
				"description": "K线交易时段: regular 仅常规盘, all 包含盘前/盘后/夜盘",
			},
		},
	}, tools.NewWatchlistTrendAnalysisTool(lb))

	server.AddTool("analyze_positions_trends", "分析当前持仓走势", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"periods": map[string]interface{}{
				"type":        "string",
				"description": "分析周期，逗号分隔，如: 1d,1h,15m",
			},
			"lookback": map[string]interface{}{
				"type":        "integer",
				"description": "回看K线数量，默认120",
			},
			"trade_session": map[string]interface{}{
				"type":        "string",
				"description": "K线交易时段: regular 仅常规盘, all 包含盘前/盘后/夜盘",
			},
		},
	}, tools.NewPositionsTrendAnalysisTool(lb))

	server.AddTool("analyze_portfolio_risk", "分析当前组合风险", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewPortfolioRiskTool(lb))

	reviewSchema := func(description string) map[string]interface{} {
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"start": map[string]interface{}{
					"type":        "integer",
					"description": description,
				},
				"end": map[string]interface{}{
					"type":        "integer",
					"description": "结束时间戳(毫秒)，可选，默认当前时间",
				},
				"timezone": map[string]interface{}{
					"type":        "string",
					"description": "时区，可选，默认 Asia/Shanghai",
				},
				"periods": map[string]interface{}{
					"type":        "string",
					"description": "持仓趋势分析周期，逗号分隔，如: 1d,1h,15m",
				},
				"lookback": map[string]interface{}{
					"type":        "integer",
					"description": "持仓趋势分析回看K线数量，默认120",
				},
				"trade_session": map[string]interface{}{
					"type":        "string",
					"description": "K线交易时段: regular 仅常规盘, all 包含盘前/盘后/夜盘",
				},
			},
		}
	}

	server.AddTool("generate_weekly_review", "生成周复盘", reviewSchema("开始时间戳(毫秒)，可选，默认本周一 00:00"), tools.NewWeeklyReviewTool(lb))
	server.AddTool("generate_daily_review", "生成日复盘", reviewSchema("开始时间戳(毫秒)，可选，默认最近 24 小时"), tools.NewDailyReviewTool(lb))
	server.AddTool("generate_exception_review", "生成波段例外复盘", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewExceptionReviewTool(lb))
	server.AddTool("generate_monthly_review", "生成月复盘", reviewSchema("开始时间戳(毫秒)，可选，默认本月 1 日 00:00"), tools.NewMonthlyReviewTool(lb))
	server.AddTool("generate_yearly_review", "生成年复盘", reviewSchema("开始时间戳(毫秒)，可选，默认当年 1 月 1 日 00:00"), tools.NewYearlyReviewTool(lb))

	server.AddTool("generate_trading_digest", "生成交易摘要与行动清单", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"period": map[string]interface{}{
				"type":        "string",
				"description": "摘要周期: daily, weekly, monthly, yearly，默认 daily",
			},
			"symbols": map[string]interface{}{
				"type":        "string",
				"description": "关注股代码，逗号分隔，可选",
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
				"description": "每只关注股附带的资讯数量，默认3",
			},
			"lookback": map[string]interface{}{
				"type":        "integer",
				"description": "趋势分析回看K线数量，默认120",
			},
			"trade_session": map[string]interface{}{
				"type":        "string",
				"description": "K线交易时段: regular 仅常规盘, all 包含盘前/盘后/夜盘",
			},
			"timezone": map[string]interface{}{
				"type":        "string",
				"description": "时区，可选，默认 Asia/Shanghai",
			},
			"start": map[string]interface{}{
				"type":        "integer",
				"description": "开始时间戳(毫秒)，可选",
			},
			"end": map[string]interface{}{
				"type":        "integer",
				"description": "结束时间戳(毫秒)，可选",
			},
		},
	}, tools.NewTradingDigestTool(lb))
}
