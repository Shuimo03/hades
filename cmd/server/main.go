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

	"hades/internal/config"
	"hades/internal/longbridge"
	"hades/internal/mcp"
	"hades/internal/tools"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
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

	// Setup HTTP handler
	http.Handle("/mcp/", server)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort)
	log.Printf("MCP server starting on %s", addr)
	log.Printf("MCP endpoint: http://%s/mcp/", addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = ctx

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
	cancel()
}
