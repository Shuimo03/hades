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
			sched.AddJob("daily_brief_pre_market", fmt.Sprintf("0 %s * * 1-5", preTime), func(ctx context.Context) {
				result, err := gen.Generate(ctx, brief.BriefVersionPreMarket, nil)
				if err != nil {
					log.Printf("[DailyBrief] Pre-market failed: %v", err)
					return
				}
				notifier.Notify(ctx, "Daily Brief - 开盘前", result)
			})
		}

		// Post-market brief
		postTime := cfg.DailyBrief.PostMarketTime
		if postTime != "" {
			sched.AddJob("daily_brief_post_market", fmt.Sprintf("0 %s * * 1-5", postTime), func(ctx context.Context) {
				result, err := gen.Generate(ctx, brief.BriefVersionPostMarket, nil)
				if err != nil {
					log.Printf("[DailyBrief] Post-market failed: %v", err)
					return
				}
				notifier.Notify(ctx, "Daily Brief - 收盘后", result)
			})
		}
	}

	// Setup signal alert checker
	if cfg.SignalAlert.Enabled {
		sched.AddJob("signal_alert_check", fmt.Sprintf("@every %ds", cfg.SignalAlert.CheckInterval), func(ctx context.Context) {
			alertMgr.CheckAll(ctx)
		})
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
		},
	}, tools.NewCandlesticksTool(lb))

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
	server.AddTool("submit_order", "提交订单", map[string]interface{}{
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
				"description": "有效期: day,ioc,fok",
			},
		},
	}, tools.NewSubmitOrderTool(lb))

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
				"description": "订单状态: all,filled,cancelled,pending,failed",
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

	// Register Daily Brief tools
	server.AddTool("get_daily_brief", "获取每日交易简报", map[string]interface{}{
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
	}, tools.NewDailyBriefTool(lb, cfg.DailyBrief.Timezone))

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
				"description": "提醒类型: price, volatility, volume",
			},
			"condition": map[string]interface{}{
				"type":        "string",
				"description": "触发条件: above, below, cross_up, cross_down",
			},
			"threshold": map[string]interface{}{
				"type":        "number",
				"description": "触发阈值",
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
	}, tools.NewCreateExecutionWindowTool(execWindowMgr))

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
	}, tools.NewDeleteExecutionWindowTool(execWindowMgr))

	// Setup HTTP handler
	http.Handle("/mcp/", server)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort)
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
