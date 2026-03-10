package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hades/internal/alerts"
	"hades/internal/brief"
	"hades/internal/config"
	"hades/internal/longbridge"
	"hades/internal/mcp"
	"hades/internal/okx"
	"hades/internal/scheduler"
	"hades/internal/tools"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	logLevel := flag.String("logs", "info", "log level: debug, info, warn, error")
	flag.Parse()

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set debug mode
	if *logLevel == "debug" {
		mcp.SetDebug(true)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("[DEBUG] Debug mode enabled")
	}

	// Validate config
	if cfg.AppKey == "" || cfg.AppSecret == "" || cfg.AccessToken == "" {
		log.Fatal("Missing required configuration: app_key, app_secret, access_token")
	}

	// Create LongBridge client
	lb, err := longbridge.NewClient(cfg.AppKey, cfg.AppSecret, cfg.AccessToken)
	if err != nil {
		log.Fatalf("Failed to create LongBridge client: %v", err)
	}
	defer lb.Close()

	// Create OKX client (optional)
	var okxClient *okx.Client
	if cfg.Okx != nil && cfg.Okx.Enabled {
		if cfg.Okx.APIKey == "" || cfg.Okx.SecretKey == "" || cfg.Okx.Passphrase == "" {
			log.Printf("[OKX] Missing api_key/secret_key/passphrase, skip OKX client")
		} else {
			okxClient = okx.NewClient(cfg.Okx.APIKey, cfg.Okx.SecretKey, cfg.Okx.Passphrase, cfg.Okx.BaseURL)
			log.Printf("[OKX] OKX client initialized")
		}
	}

	// Create scheduler
	sched := scheduler.New()

	// Create alert manager
	alertMgr := alerts.New(lb, "data/alerts.json", time.Duration(cfg.SignalAlert.CheckInterval)*time.Second)
	if err := alertMgr.Load(); err != nil {
		log.Printf("[Alerts] Failed to load alerts: %v", err)
	}

	// Create notifier (Feishu API > Webhook)
	var notifier alerts.Notifier
	if cfg.Feishu != nil && cfg.Feishu.Enabled && cfg.Feishu.AppID != "" && cfg.Feishu.AppSecret != "" && cfg.Feishu.UserID != "" {
		notifier = alerts.NewFeishuNotifierWithConfig(cfg.Feishu.AppID, cfg.Feishu.AppSecret, cfg.Feishu.UserID)
		log.Printf("[Notifier] Using Feishu API (app_id: %s, user_id: %s)", cfg.Feishu.AppID, cfg.Feishu.UserID)
	} else if cfg.SignalAlert != nil && cfg.SignalAlert.WebhookURL != "" {
		notifier = alerts.NewNotifier(cfg.SignalAlert.WebhookURL)
		log.Printf("[Notifier] Using webhook: %s", cfg.SignalAlert.WebhookURL)
	} else {
		notifier = alerts.NewNotifier("")
		log.Printf("[Notifier] No notification configured")
	}

	// Set alert callback
	alertMgr.SetCallback(func(alert *alerts.Alert, message string) {
		if planContext := alerts.BuildAlertPlanContext(context.Background(), lb, alert.Symbol); planContext != "" {
			message += "\n" + planContext
		}
		log.Printf("[Alerts] Triggered: %s", message)
		notifier.Notify(context.Background(), fmt.Sprintf("Signal Alert: %s", alert.Symbol), message)
	})

	// Create execution window manager
	execWindowMgr := alerts.NewExecutionWindowManager("data/execution_windows.json")
	if err := execWindowMgr.Load(); err != nil {
		log.Printf("[ExecutionWindow] Failed to load windows: %v", err)
	}

	// Set execution window callback
	execWindowMgr.SetCallback(func(window *alerts.ExecutionWindow) {
		log.Printf("[ExecutionWindow] Triggered: %s", window.Name)
		// Generate brief for execution window
		gen := brief.New(lb, cfg.DailyBrief.Timezone)
		result, err := gen.Generate(context.Background(), brief.BriefVersionPreMarket, nil)
		if err != nil {
			log.Printf("[ExecutionWindow] Failed to generate brief: %v", err)
			return
		}
		message := fmt.Sprintf("Execution Window: %s\n\n%s", window.Name, result)
		notifier.Notify(context.Background(), "Execution Window", message)
	})

	// Setup Daily Brief jobs
	if cfg.DailyBrief.Enabled {
		timezone := cfg.DailyBrief.Timezone
		if timezone == "" {
			timezone = "Asia/Shanghai"
		}
		gen := brief.New(lb, timezone)

		// Pre-market brief
		preTime := cfg.DailyBrief.PreMarketTime
		if preTime != "" {
			spec, err := weekdayTimeSpec(preTime)
			if err != nil {
				log.Printf("[DailyBrief] Invalid pre-market schedule %q: %v", preTime, err)
			} else if err := sched.AddJob("daily_brief_pre_market", spec, func(ctx context.Context) {
				result, err := gen.Generate(ctx, brief.BriefVersionPreMarket, nil)
				if err != nil {
					log.Printf("[DailyBrief] Pre-market failed: %v", err)
					return
				}
				notifier.Notify(ctx, "Daily Brief - 开盘前", result)
			}); err != nil {
				log.Printf("[DailyBrief] Failed to add pre-market job: %v", err)
			}
		}

		// Post-market brief
		postTime := cfg.DailyBrief.PostMarketTime
		if postTime != "" {
			spec, err := weekdayTimeSpec(postTime)
			if err != nil {
				log.Printf("[DailyBrief] Invalid post-market schedule %q: %v", postTime, err)
			} else if err := sched.AddJob("daily_brief_post_market", spec, func(ctx context.Context) {
				result, err := gen.Generate(ctx, brief.BriefVersionPostMarket, nil)
				if err != nil {
					log.Printf("[DailyBrief] Post-market failed: %v", err)
					return
				}
				notifier.Notify(ctx, "Daily Brief - 收盘后", result)
			}); err != nil {
				log.Printf("[DailyBrief] Failed to add post-market job: %v", err)
			}
		}
	}

	// Setup signal alert checker
	if cfg.SignalAlert.Enabled {
		if err := sched.AddJob("signal_alert_check", fmt.Sprintf("@every %ds", cfg.SignalAlert.CheckInterval), func(ctx context.Context) {
			alertMgr.CheckAll(ctx)
		}); err != nil {
			log.Printf("[Alerts] Failed to add signal alert job: %v", err)
		}
	}

	if cfg.ExecutionWindow.Enabled {
		for _, window := range execWindowMgr.List() {
			if window == nil || !window.Enabled {
				continue
			}
			windowID := window.ID
			if err := sched.AddJob("execution_window_"+windowID, window.Schedule, func(ctx context.Context) {
				execWindowMgr.Trigger(windowID)
			}); err != nil {
				log.Printf("[ExecutionWindow] Failed to restore window %s: %v", window.Name, err)
			}
		}
	}

	// Start scheduler
	sched.Start()

	// Create MCP HTTP server
	server := mcp.NewHTTPServer("longbridge-mcp", "v1.0.0")

	// Register quote tools
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

	// Register historical data tools
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
			"periods": map[string]interface{}{
				"type":        "string",
				"description": "分析周期，逗号分隔，如: 1d,1h,15m",
			},
			"lookback": map[string]interface{}{
				"type":        "integer",
				"description": "回看K线数量，默认120",
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
		},
	}, tools.NewPositionsTrendAnalysisTool(lb))

	server.AddTool("analyze_portfolio_risk", "分析当前组合风险", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewPortfolioRiskTool(lb))

	server.AddTool("generate_weekly_review", "生成周复盘", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"start": map[string]interface{}{
				"type":        "integer",
				"description": "开始时间戳(毫秒)，可选，默认本周一 00:00",
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
		},
	}, tools.NewWeeklyReviewTool(lb))
	server.AddTool("generate_daily_review", "生成日复盘", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"start": map[string]interface{}{
				"type":        "integer",
				"description": "开始时间戳(毫秒)，可选，默认今日 00:00",
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
		},
	}, tools.NewDailyReviewTool(lb))
	server.AddTool("generate_monthly_review", "生成月复盘", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"start": map[string]interface{}{
				"type":        "integer",
				"description": "开始时间戳(毫秒)，可选，默认本月 1 日 00:00",
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
		},
	}, tools.NewMonthlyReviewTool(lb))
	server.AddTool("generate_yearly_review", "生成年复盘", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"start": map[string]interface{}{
				"type":        "integer",
				"description": "开始时间戳(毫秒)，可选，默认当年 1 月 1 日 00:00",
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
		},
	}, tools.NewYearlyReviewTool(lb))

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
			"news_count": map[string]interface{}{
				"type":        "integer",
				"description": "每只关注股附带的资讯数量，默认3",
			},
			"lookback": map[string]interface{}{
				"type":        "integer",
				"description": "趋势分析回看K线数量，默认120",
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

	// Register account tools
	server.AddTool("get_account_info", "获取账户信息", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewAccountInfoTool(lb))

	server.AddTool("get_positions", "获取持仓信息", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewPositionsTool(lb))

	// Register order tools
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

	// Register OKX tools (crypto)
	if okxClient != nil {
		server.AddTool("okx_get_ticker", "获取 OKX 单币对行情", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"inst_id": map[string]interface{}{
					"type":        "string",
					"description": "交易对ID，如: BTC-USDT",
				},
			},
		}, tools.NewOkxTickerTool(okxClient))

		server.AddTool("okx_get_orderbook", "获取 OKX 盘口", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"inst_id": map[string]interface{}{
					"type":        "string",
					"description": "交易对ID，如: BTC-USDT",
				},
				"depth": map[string]interface{}{
					"type":        "integer",
					"description": "深度条数，默认5",
				},
			},
		}, tools.NewOkxOrderBookTool(okxClient))

		server.AddTool("okx_get_trades", "获取 OKX 最近成交", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"inst_id": map[string]interface{}{
					"type":        "string",
					"description": "交易对ID",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回条数，默认100",
				},
			},
		}, tools.NewOkxTradesTool(okxClient))

		server.AddTool("okx_get_candles", "获取 OKX K 线", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"inst_id": map[string]interface{}{
					"type":        "string",
					"description": "交易对ID",
				},
				"bar": map[string]interface{}{
					"type":        "string",
					"description": "K线周期，如 1m/5m/15m/1H/1D",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回条数，默认100",
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
				"inst_id":   map[string]interface{}{"type": "string", "description": "交易对ID"},
				"side":      map[string]interface{}{"type": "string", "description": "buy 或 sell"},
				"ord_type":  map[string]interface{}{"type": "string", "description": "订单类型: limit 或 market"},
				"sz":        map[string]interface{}{"type": "string", "description": "下单数量"},
				"px":        map[string]interface{}{"type": "string", "description": "价格，市价单可省略"},
				"td_mode":   map[string]interface{}{"type": "string", "description": "交易模式，默认 cash"},
				"cl_ord_id": map[string]interface{}{"type": "string", "description": "自定义订单ID，可选"},
			},
		}
		server.AddTool("okx_place_order", "OKX 下单", okxOrderSchema, tools.NewOkxPlaceOrderTool(okxClient))

		server.AddTool("okx_cancel_order", "OKX 撤单", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"inst_id":   map[string]interface{}{"type": "string", "description": "交易对ID"},
				"ord_id":    map[string]interface{}{"type": "string", "description": "订单ID"},
				"cl_ord_id": map[string]interface{}{"type": "string", "description": "自定义订单ID，可选"},
			},
		}, tools.NewOkxCancelOrderTool(okxClient))

		server.AddTool("okx_get_open_orders", "OKX 当前挂单", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"inst_type": map[string]interface{}{"type": "string", "description": "产品类型，默认 SPOT"},
			},
		}, tools.NewOkxOpenOrdersTool(okxClient))

		server.AddTool("okx_get_order_history", "OKX 历史订单", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"inst_type": map[string]interface{}{"type": "string", "description": "产品类型，默认 SPOT"},
				"limit":     map[string]interface{}{"type": "integer", "description": "返回条数，默认50"},
			},
		}, tools.NewOkxOrderHistoryTool(okxClient))
	}

	// Register Daily Brief tools
	dailyBriefSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"version": map[string]interface{}{
				"type":        "string",
				"description": "报告版本: pre_market (开盘前), post_market (收盘后)",
			},
			"symbols": map[string]interface{}{
				"type":        "string",
				"description": "关注的股票代码，用逗号分隔",
			},
		},
	}
	dailyBriefTool := tools.NewDailyBriefTool(lb, cfg.DailyBrief.Timezone)
	server.AddTool("get_daily_brief", "获取每日交易简报", dailyBriefSchema, dailyBriefTool)
	server.AddTool("generate_daily_brief", "获取每日交易简报", dailyBriefSchema, dailyBriefTool)

	// Register Signal Alert tools
	server.AddTool("create_signal_alert", "创建信号提醒", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "股票代码，如: 700.HK",
			},
			"alert_type": map[string]interface{}{
				"type":        "string",
				"description": "提醒类型: price, volatility, volume, trend",
			},
			"condition": map[string]interface{}{
				"type":        "string",
				"description": "触发条件: above, below, cross_up, cross_down, in_buy_zone, near_take_profit, below_stop_loss",
			},
			"threshold": map[string]interface{}{
				"type":        "number",
				"description": "触发阈值；trend 类型可省略，near_take_profit 时可作为提前提醒百分比",
			},
			"note": map[string]interface{}{
				"type":        "string",
				"description": "备注",
			},
		},
	}, tools.NewCreateSignalAlertTool(lb, alertMgr))

	server.AddTool("list_signal_alerts", "查询信号提醒列表", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewListSignalAlertsTool(alertMgr))

	server.AddTool("delete_signal_alert", "删除信号提醒", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"alert_id": map[string]interface{}{
				"type":        "string",
				"description": "提醒ID",
			},
		},
	}, tools.NewDeleteSignalAlertTool(alertMgr))

	server.AddTool("enable_signal_alert", "启用/禁用信号提醒", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"alert_id": map[string]interface{}{
				"type":        "string",
				"description": "提醒ID",
			},
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "是否启用",
			},
		},
	}, tools.NewEnableSignalAlertTool(alertMgr))
	server.AddTool("check_alerts", "手动检查信号提醒", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewCheckAlertsTool(alertMgr))

	// Register Execution Window tools
	server.AddTool("create_execution_window", "创建执行窗口", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "窗口名称",
			},
			"schedule": map[string]interface{}{
				"type":        "string",
				"description": "Cron 表达式，如: 0 9 * * 1-5",
			},
			"strategy": map[string]interface{}{
				"type":        "string",
				"description": "策略描述",
			},
			"webhook_url": map[string]interface{}{
				"type":        "string",
				"description": "Webhook URL (可选)",
			},
		},
	}, tools.NewCreateExecutionWindowTool(execWindowMgr, sched))

	server.AddTool("list_execution_windows", "查询执行窗口列表", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewListExecutionWindowsTool(execWindowMgr))

	server.AddTool("delete_execution_window", "删除执行窗口", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"window_id": map[string]interface{}{
				"type":        "string",
				"description": "窗口ID",
			},
		},
	}, tools.NewDeleteExecutionWindowTool(execWindowMgr, sched))

	// Setup HTTP handler
	http.Handle("/mcp/", server)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("MCP server starting on %s", addr)
	log.Printf("MCP endpoint: http://%s/mcp/", addr)

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	sched.Stop()
}

func weekdayTimeSpec(hhmm string) (string, error) {
	parsed, err := time.Parse("15:04", hhmm)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d %d * * 1-5", parsed.Minute(), parsed.Hour()), nil
}
