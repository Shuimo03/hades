package main

import (
	"hades/internal/alerts"
	"hades/internal/config"
	"hades/internal/longbridge"
	"hades/internal/mcp"
	"hades/internal/scheduler"
	"hades/internal/tools"
)

func registerWorkflowTools(server *mcp.HTTPServer, cfg *config.Config, lb *longbridge.Client, alertMgr *alerts.Manager, execWindowMgr *alerts.ExecutionWindowManager, sched *scheduler.Scheduler) {
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
			"session_scope": map[string]interface{}{
				"type":        "string",
				"description": "提醒时段: regular 仅常规盘, extended 允许盘前/盘后/夜盘；留空则继承全局 signal_alert.session_scope",
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
				"description": "提醒 ID",
			},
		},
	}, tools.NewDeleteSignalAlertTool(alertMgr))

	server.AddTool("enable_signal_alert", "启用/禁用信号提醒", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"alert_id": map[string]interface{}{
				"type":        "string",
				"description": "提醒 ID",
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

	server.AddTool("create_execution_window", "创建执行窗口", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":        map[string]interface{}{"type": "string", "description": "窗口名称"},
			"schedule":    map[string]interface{}{"type": "string", "description": "cron 表达式"},
			"strategy":    map[string]interface{}{"type": "string", "description": "策略描述"},
			"webhook_url": map[string]interface{}{"type": "string", "description": "可选 Webhook"},
			"enabled":     map[string]interface{}{"type": "boolean", "description": "是否启用"},
		},
	}, tools.NewCreateExecutionWindowTool(execWindowMgr, sched))

	server.AddTool("list_execution_windows", "查询执行窗口列表", map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}, tools.NewListExecutionWindowsTool(execWindowMgr))

	server.AddTool("delete_execution_window", "删除执行窗口", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"window_id": map[string]interface{}{"type": "string", "description": "窗口 ID"},
		},
	}, tools.NewDeleteExecutionWindowTool(execWindowMgr, sched))
}
