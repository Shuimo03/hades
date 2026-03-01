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

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	server := mcpserver.NewHTTPServer("longbridge-mcp", "v1.0.0")

	// Register quote tools
	server.AddTool(&mcp.Tool{
		Name:        "get_quote",
		Description: "获取股票实时行情",
	}, tools.NewQuoteTool(lb))

	server.AddTool(&mcp.Tool{
		Name:        "get_quote_info",
		Description: "获取股票基本信息",
	}, tools.NewQuoteInfoTool(lb))

	server.AddTool(&mcp.Tool{
		Name:        "get_depth",
		Description: "获取股票买卖盘口",
	}, tools.NewDepthTool(lb))

	server.AddTool(&mcp.Tool{
		Name:        "get_trades",
		Description: "获取股票分时成交",
	}, tools.NewTradesTool(lb))

	// Register historical data tools
	server.AddTool(&mcp.Tool{
		Name:        "get_candlesticks",
		Description: "获取股票K线数据",
	}, tools.NewCandlesticksTool(lb))

	// Register account tools
	server.AddTool(&mcp.Tool{
		Name:        "get_account_info",
		Description: "获取账户信息",
	}, tools.NewAccountInfoTool(lb))

	server.AddTool(&mcp.Tool{
		Name:        "get_positions",
		Description: "获取持仓信息",
	}, tools.NewPositionsTool(lb))

	// Register order tools
	server.AddTool(&mcp.Tool{
		Name:        "submit_order",
		Description: "提交订单",
	}, tools.NewSubmitOrderTool(lb))

	server.AddTool(&mcp.Tool{
		Name:        "cancel_order",
		Description: "取消订单",
	}, tools.NewCancelOrderTool(lb))

	server.AddTool(&mcp.Tool{
		Name:        "get_orders",
		Description: "查询订单列表",
	}, tools.NewOrdersTool(lb))

	server.AddTool(&mcp.Tool{
		Name:        "get_order_detail",
		Description: "查询订单详情",
	}, tools.NewOrderDetailTool(lb))

	server.AddTool(&mcp.Tool{
		Name:        "get_history_executions",
		Description: "查询历史成交",
	}, tools.NewHistoryExecutionsTool(lb))

	// Setup HTTP handler
	http.Handle("/mcp/", server)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort)
	log.Printf("MCP server starting on %s", addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
