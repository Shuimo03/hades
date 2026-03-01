package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hades/internal/alerts"
	"hades/internal/longbridge"
)

// NewCreateSignalAlertTool creates a tool for creating signal alerts
func NewCreateSignalAlertTool(lb *longbridge.Client, mgr *alerts.Manager) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		symbol, ok := args["symbol"].(string)
		if !ok || symbol == "" {
			return nil, fmt.Errorf("missing or invalid symbol parameter")
		}

		alertTypeStr, ok := args["alert_type"].(string)
		if !ok || alertTypeStr == "" {
			return nil, fmt.Errorf("missing or invalid alert_type parameter")
		}

		conditionStr, ok := args["condition"].(string)
		if !ok || conditionStr == "" {
			return nil, fmt.Errorf("missing or invalid condition parameter")
		}

		threshold, ok := args["threshold"].(float64)
		if !ok {
			return nil, fmt.Errorf("missing or invalid threshold parameter")
		}

		note := ""
		if n, ok := args["note"].(string); ok {
			note = n
		}

		alert := &alerts.Alert{
			Symbol:    symbol,
			AlertType: alerts.AlertType(alertTypeStr),
			Condition: alerts.AlertCondition(conditionStr),
			Threshold: threshold,
			Note:      note,
			Enabled:   true,
		}

		if err := mgr.Create(alert); err != nil {
			return nil, fmt.Errorf("failed to create alert: %v", err)
		}

		return map[string]interface{}{
			"result": fmt.Sprintf("✅ 提醒创建成功: %s %s %s %.2f",
				alert.Symbol, alert.Condition, alert.AlertType, alert.Threshold),
		}, nil
	}
}

// NewListSignalAlertsTool creates a tool for listing signal alerts
func NewListSignalAlertsTool(mgr *alerts.Manager) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		alertList := mgr.List()

		if len(alertList) == 0 {
			return map[string]interface{}{"result": "暂无信号提醒"}, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📊 信号提醒 (%d 个):\n\n", len(alertList)))

		for _, a := range alertList {
			status := "✅"
			if !a.Enabled {
				status = "❌"
			}
			if a.Triggered {
				status = "🔔"
			}

			sb.WriteString(fmt.Sprintf("%s %s | %s | %s | %.2f\n",
				status, a.Symbol, a.AlertType, a.Condition, a.Threshold))
			if a.Note != "" {
				sb.WriteString(fmt.Sprintf("   备注: %s\n", a.Note))
			}
		}

		return map[string]interface{}{"result": sb.String()}, nil
	}
}

// NewDeleteSignalAlertTool creates a tool for deleting signal alerts
func NewDeleteSignalAlertTool(mgr *alerts.Manager) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		alertID, ok := args["alert_id"].(string)
		if !ok || alertID == "" {
			return nil, fmt.Errorf("missing or invalid alert_id parameter")
		}

		if !mgr.Delete(alertID) {
			return nil, fmt.Errorf("alert not found: %s", alertID)
		}

		return map[string]interface{}{"result": fmt.Sprintf("✅ 已删除提醒: %s", alertID)}, nil
	}
}

// NewEnableSignalAlertTool creates a tool for enabling/disabling signal alerts
func NewEnableSignalAlertTool(mgr *alerts.Manager) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		alertID, ok := args["alert_id"].(string)
		if !ok || alertID == "" {
			return nil, fmt.Errorf("missing or invalid alert_id parameter")
		}

		enabled, ok := args["enabled"].(bool)
		if !ok {
			return nil, fmt.Errorf("missing or invalid enabled parameter")
		}

		if !mgr.Enable(alertID, enabled) {
			return nil, fmt.Errorf("alert not found: %s", alertID)
		}

		status := "启用"
		if !enabled {
			status = "禁用"
		}

		return map[string]interface{}{"result": fmt.Sprintf("✅ 提醒已%s: %s", status, alertID)}, nil
	}
}

// NewCheckAlertsTool creates a tool for manually checking alerts
func NewCheckAlertsTool(lb *longbridge.Client, mgr *alerts.Manager) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		mgr.CheckAll(ctx)
		return map[string]interface{}{"result": "✅ 触发检查完成"}, nil
	}
}

// NewCreateExecutionWindowTool creates a tool for creating execution windows
func NewCreateExecutionWindowTool(mgr *alerts.ExecutionWindowManager) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		name, ok := args["name"].(string)
		if !ok || name == "" {
			return nil, fmt.Errorf("missing or invalid name parameter")
		}

		schedule, ok := args["schedule"].(string)
		if !ok || schedule == "" {
			return nil, fmt.Errorf("missing or invalid schedule parameter")
		}

		strategy, ok := args["strategy"].(string)
		if !ok || strategy == "" {
			return nil, fmt.Errorf("missing or invalid strategy parameter")
		}

		webhookURL := ""
		if w, ok := args["webhook_url"].(string); ok {
			webhookURL = w
		}

		window := &alerts.ExecutionWindow{
			Name:       name,
			Schedule:   schedule,
			Strategy:   strategy,
			WebhookURL: webhookURL,
			Enabled:    true,
		}

		if err := mgr.Create(window); err != nil {
			return nil, fmt.Errorf("failed to create execution window: %v", err)
		}

		return map[string]interface{}{
			"result": fmt.Sprintf("✅ 执行窗口创建成功: %s\n时间: %s\n策略: %s",
				window.Name, window.Schedule, window.Strategy),
		}, nil
	}
}

// NewListExecutionWindowsTool creates a tool for listing execution windows
func NewListExecutionWindowsTool(mgr *alerts.ExecutionWindowManager) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		windows := mgr.List()

		if len(windows) == 0 {
			return map[string]interface{}{"result": "暂无执行窗口"}, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("⏰ 执行窗口 (%d 个):\n\n", len(windows)))

		for _, w := range windows {
			status := "✅"
			if !w.Enabled {
				status = "❌"
			}

			sb.WriteString(fmt.Sprintf("%s %s\n", status, w.Name))
			sb.WriteString(fmt.Sprintf("   时间: %s\n", w.Schedule))
			sb.WriteString(fmt.Sprintf("   策略: %s\n", w.Strategy))
			if !w.LastRun.IsZero() {
				sb.WriteString(fmt.Sprintf("   上次运行: %s\n", w.LastRun.Format("2006-01-02 15:04")))
			}
			sb.WriteString("\n")
		}

		return map[string]interface{}{"result": sb.String()}, nil
	}
}

// NewDeleteExecutionWindowTool creates a tool for deleting execution windows
func NewDeleteExecutionWindowTool(mgr *alerts.ExecutionWindowManager) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		windowID, ok := args["window_id"].(string)
		if !ok || windowID == "" {
			return nil, fmt.Errorf("missing or invalid window_id parameter")
		}

		if !mgr.Delete(windowID) {
			return nil, fmt.Errorf("window not found: %s", windowID)
		}

		return map[string]interface{}{"result": fmt.Sprintf("✅ 已删除执行窗口: %s", windowID)}, nil
	}
}
