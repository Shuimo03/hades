package main

import (
	"hades/internal/alerts"
	"hades/internal/config"
	"hades/internal/longbridge"
	"hades/internal/mcp"
	"hades/internal/okx"
	"hades/internal/scheduler"
)

func registerServerTools(server *mcp.HTTPServer, cfg *config.Config, lb *longbridge.Client, okxClient *okx.Client, alertMgr *alerts.Manager, execWindowMgr *alerts.ExecutionWindowManager, sched *scheduler.Scheduler) {
	registerMarketTools(server, lb)
	registerAnalysisTools(server, lb)
	registerAccountAndTradeTools(server, lb, okxClient)
	registerWorkflowTools(server, cfg, lb, alertMgr, execWindowMgr, sched)
}
