package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"hades/internal/alerts"
	"hades/internal/longbridge"
)

func NewExceptionReviewTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		snapshots, err := buildPositionSnapshots(ctx, lb)
		if err != nil {
			return nil, err
		}

		if len(snapshots) == 0 {
			return map[string]interface{}{
				"result": map[string]interface{}{
					"analysis_of":     "exception_review",
					"has_exceptions":  false,
					"positions_count": 0,
					"items":           []map[string]interface{}{},
					"summary":         "当前无持仓，无需波段例外复盘。",
				},
			}, nil
		}

		items := make([]map[string]interface{}, 0, len(snapshots))
		risks := make([]string, 0, len(snapshots))

		for _, snapshot := range snapshots {
			symbol := snapshot.Position.Symbol
			metrics, err := alerts.BuildAlertPlanMetrics(ctx, lb, symbol, longbridge.QuoteSessionScopeRegular)
			if err != nil {
				return nil, fmt.Errorf("build plan metrics for %s: %w", symbol, err)
			}

			reasons, severity := exceptionReasons(snapshot, metrics)
			if len(reasons) == 0 {
				continue
			}

			items = append(items, map[string]interface{}{
				"symbol":             symbol,
				"symbol_name":        snapshot.Position.SymbolName,
				"mode":               metrics.Mode,
				"mode_label":         metrics.ModeLabel,
				"decision":           swingDecision(metrics),
				"severity":           severity,
				"current_price":      metrics.CurrentPrice,
				"cost_price":         snapshot.CostPrice,
				"unrealized_pnl_pct": snapshot.UnrealizedPnLPct,
				"stop_loss":          metrics.StopLoss,
				"take_profit":        metrics.TakeProfit,
				"take_profit_2":      metrics.TakeProfit2,
				"conclusion":         metrics.Conclusion,
				"entry_condition":    metrics.EntryCondition,
				"invalidation":       metrics.InvalidationCondition,
				"target_summary":     metrics.TargetSummary,
				"status":             metrics.Status,
				"status_reason":      metrics.StatusReason,
				"event_notes":        metrics.EventNotes,
				"reasons":            reasons,
				"position_context":   metrics.PositionContext,
			})

			risks = append(risks, fmt.Sprintf("%s: %s", symbol, strings.Join(reasons, "；")))
		}

		sort.Slice(items, func(i, j int) bool {
			left, _ := items[i]["severity"].(int)
			right, _ := items[j]["severity"].(int)
			return left > right
		})

		summary := "当前持仓未触发例外条件，按周复盘节奏跟踪即可。"
		if len(items) > 0 {
			topSymbol, _ := items[0]["symbol"].(string)
			summary = fmt.Sprintf("波段例外复盘：当前有 %d 个持仓需要处理，优先检查 %s。", len(items), topSymbol)
		}

		return map[string]interface{}{
			"result": map[string]interface{}{
				"analysis_of":     "exception_review",
				"has_exceptions":  len(items) > 0,
				"positions_count": len(snapshots),
				"count":           len(items),
				"items":           items,
				"risks":           risks,
				"summary":         summary,
			},
		}, nil
	}
}

func exceptionReasons(snapshot positionSnapshot, metrics *alerts.AlertPlanMetrics) ([]string, int) {
	reasons := make([]string, 0, 5)
	severity := 0

	switch metrics.Status {
	case "invalidated":
		reasons = append(reasons, "计划已失效，需要立即重算")
		severity += 5
	case "defer":
		reasons = append(reasons, "事件窗口抬升波动，计划已降权")
		severity += 4
	}

	if metrics.EventRisk {
		reasons = append(reasons, "事件风险模式，避免机械执行")
		severity += 3
	}

	if metrics.StopLoss > 0 && metrics.CurrentPrice <= metrics.StopLoss*1.01 {
		reasons = append(reasons, "价格已接近失效位")
		severity += 4
	}
	if metrics.TakeProfit > 0 && metrics.CurrentPrice >= metrics.TakeProfit*0.99 {
		reasons = append(reasons, "价格已接近 TP1，适合检查锁盈")
		severity += 3
	}
	if snapshot.UnrealizedPnLPct <= -5 {
		reasons = append(reasons, fmt.Sprintf("浮亏扩大到 %.2f%%", snapshot.UnrealizedPnLPct))
		severity += 3
	}
	if snapshot.UnrealizedPnLPct >= 10 {
		reasons = append(reasons, fmt.Sprintf("浮盈达到 %.2f%%，应检查移动止盈", snapshot.UnrealizedPnLPct))
		severity += 2
	}

	return dedupeStrings(reasons), severity
}

func swingDecision(metrics *alerts.AlertPlanMetrics) string {
	switch {
	case metrics.Status == "invalidated":
		return "invalidated"
	case metrics.IsHeld && (metrics.Mode == "event" || metrics.StopLoss > 0 && metrics.CurrentPrice <= metrics.StopLoss*1.01):
		return "reduce"
	case metrics.IsHeld:
		return "hold"
	default:
		return "wait"
	}
}
