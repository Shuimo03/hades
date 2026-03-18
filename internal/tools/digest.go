package tools

import (
	"context"
	"fmt"
	"strings"

	"hades/internal/longbridge"
)

func NewTradingDigestTool(lb *longbridge.Client) func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		periodRaw, _ := args["period"].(string)
		period := reviewPeriod(strings.ToLower(strings.TrimSpace(periodRaw)))
		switch period {
		case reviewPeriodDaily, reviewPeriodWeekly, reviewPeriodMonthly, reviewPeriodYearly:
		default:
			period = reviewPeriodDaily
		}

		reviewResult, err := buildPeriodicReviewResult(ctx, lb, args, period)
		if err != nil {
			return nil, err
		}

		digest := map[string]interface{}{
			"analysis_of":         "trading_digest",
			"period":              period,
			"review":              reviewResult,
			"summary":             reviewResult["summary"],
			"action_items":        buildDigestActions(reviewResult, nil, nil),
			"watchlist_plan":      nil,
			"portfolio_risk":      nil,
			"execution_checklist": nil,
		}

		portfolioResult, err := NewPortfolioRiskTool(lb)(ctx, map[string]interface{}{})
		if err != nil {
			return nil, err
		}
		if result, ok := portfolioResult["result"].(map[string]interface{}); ok {
			digest["portfolio_risk"] = result
		}

		if _, ok := args["symbols"]; ok || args["group_name"] != nil || args["group_id"] != nil {
			planResult, err := NewWatchlistPlanTool(lb)(ctx, map[string]interface{}{
				"symbols":    args["symbols"],
				"group_name": args["group_name"],
				"group_id":   args["group_id"],
				"news_count": args["news_count"],
				"lookback":   args["lookback"],
			})
			if err != nil {
				return nil, err
			}
			if result, ok := planResult["result"].(map[string]interface{}); ok {
				digest["watchlist_plan"] = result
				digest["action_items"] = buildDigestActions(reviewResult, result, digest["portfolio_risk"])
			}
		} else {
			digest["action_items"] = buildDigestActions(reviewResult, nil, digest["portfolio_risk"])
		}

		watchlistPlan, _ := digest["watchlist_plan"].(map[string]interface{})
		portfolioRisk, _ := digest["portfolio_risk"].(map[string]interface{})
		digest["execution_checklist"] = buildExecutionChecklist(reviewResult, watchlistPlan, portfolioRisk)

		return map[string]interface{}{"result": digest}, nil
	}
}

func buildDigestActions(review map[string]interface{}, watchlistPlan interface{}, portfolio interface{}) []string {
	actions := make([]string, 0, 8)

	if review != nil {
		if risks, ok := review["risks"].([]string); ok {
			for _, risk := range risks {
				if strings.Contains(risk, "过度交易") {
					actions = append(actions, "下个交易周期优先减少无计划交易，先按既定买卖点执行。")
				}
				if strings.Contains(risk, "亏损持仓") || strings.Contains(risk, "趋势偏弱") {
					actions = append(actions, "优先处理弱势或亏损持仓，严格执行止损和减仓。")
				}
			}
		}
	}

	if portfolioMap, ok := portfolio.(map[string]interface{}); ok {
		if portfolioActions, ok := portfolioMap["portfolio_actions"].([]string); ok {
			actions = append(actions, portfolioActions...)
		}
	}

	if planMap, ok := watchlistPlan.(map[string]interface{}); ok {
		if items, ok := planMap["items"].([]map[string]interface{}); ok {
			for _, item := range items {
				action, _ := item["action"].(string)
				symbol, _ := item["symbol"].(string)
				modeLabel, _ := item["mode_label"].(string)
				conclusion, _ := item["conclusion"].(string)
				switch action {
				case "watch_pullback_buy":
					actions = append(actions, fmt.Sprintf("%s 当前为%s模式，等待回踩确认后再执行。", symbol, modeLabel))
				case "watch_breakout":
					actions = append(actions, fmt.Sprintf("%s 当前为%s模式，只接受确认突破，不预判追价。", symbol, modeLabel))
				case "range_trade":
					actions = append(actions, fmt.Sprintf("%s 当前为%s模式，只在下沿尝试，不追高。", symbol, modeLabel))
				case "reduce_or_wait":
					actions = append(actions, fmt.Sprintf("%s 当前走势偏弱，优先观察或降低参与度。", symbol))
				case "event_wait":
					actions = append(actions, fmt.Sprintf("%s 处于事件模式，机械挂单降权，先等事件后确认。", symbol))
				default:
					if conclusion != "" {
						actions = append(actions, fmt.Sprintf("%s: %s", symbol, conclusion))
					}
				}
				if len(actions) >= 6 {
					break
				}
			}
		}
	}

	if len(actions) == 0 {
		actions = append(actions, "维持当前计划，优先执行已有买入区、止盈位和止损位。")
	}
	return dedupeStrings(actions)
}

func buildExecutionChecklist(review map[string]interface{}, watchlistPlan map[string]interface{}, portfolio map[string]interface{}) map[string]interface{} {
	buyCandidates := make([]map[string]interface{}, 0, 6)
	positionActions := make([]map[string]interface{}, 0, 8)
	riskControls := make([]string, 0, 6)
	priorityOrder := make([]string, 0, 8)

	if watchlistPlan != nil {
		if items, ok := watchlistPlan["items"].([]map[string]interface{}); ok {
			for _, item := range items {
				action, _ := item["action"].(string)
				symbol, _ := item["symbol"].(string)
				switch action {
				case "watch_pullback_buy", "watch_breakout", "range_trade", "event_wait":
					buyCandidates = append(buyCandidates, map[string]interface{}{
						"symbol":           symbol,
						"mode":             item["mode"],
						"mode_label":       item["mode_label"],
						"action":           action,
						"buy_zone_low":     item["buy_zone_low"],
						"buy_zone_high":    item["buy_zone_high"],
						"stop_loss":        item["stop_loss"],
						"take_profit":      item["take_profit"],
						"take_profit_2":    item["take_profit_2"],
						"breakout_confirm": item["breakout_confirm"],
						"breakout_exit":    item["breakout_exit"],
						"chase_limit":      item["chase_limit"],
						"rr":               item["rr"],
						"rr_qualified":     item["rr_qualified"],
						"entry_condition":  item["entry_condition"],
						"invalidation":     item["invalidation"],
						"target_summary":   item["target_summary"],
						"suggestion":       item["suggestion"],
					})
				}
			}
		}
	}

	if review != nil {
		if positions, ok := review["positions"].(map[string]interface{}); ok {
			if items, ok := positions["items"].([]map[string]interface{}); ok {
				for _, item := range items {
					symbol, _ := item["symbol"].(string)
					score, _ := item["score"].(int)
					trend, _ := item["trend"].(string)
					unrealizedPnLPct, _ := item["unrealized_pnl_pct"].(float64)

					action := "hold_and_monitor"
					note := "维持持仓，继续跟踪关键位和止损位。"
					switch {
					case trend == "bearish" || score <= 40:
						action = "reduce_or_exit"
						note = "走势偏弱，优先考虑减仓或退出。"
					case unrealizedPnLPct >= 10:
						action = "trim_or_raise_stop"
						note = "已有较好浮盈，考虑分批止盈或上移止损。"
					case unrealizedPnLPct <= -5:
						action = "review_stop_loss"
						note = "亏损扩大，优先检查止损纪律。"
					}

					positionActions = append(positionActions, map[string]interface{}{
						"symbol":             symbol,
						"trend":              trend,
						"score":              score,
						"unrealized_pnl_pct": unrealizedPnLPct,
						"action":             action,
						"note":               note,
					})
				}
			}
		}
	}

	if portfolio != nil {
		if actions, ok := portfolio["portfolio_actions"].([]string); ok {
			riskControls = append(riskControls, actions...)
		}
		if risks, ok := portfolio["portfolio_risks"].([]string); ok {
			for _, risk := range risks {
				if strings.Contains(risk, "集中度") {
					priorityOrder = append(priorityOrder, "先控制组合集中度，再考虑新增仓位。")
				}
				if strings.Contains(risk, "回撤") {
					priorityOrder = append(priorityOrder, "先处理回撤和弱势持仓，再安排新开仓。")
				}
			}
		}
	}

	if len(priorityOrder) == 0 {
		switch {
		case len(positionActions) > 0:
			priorityOrder = append(priorityOrder, "先处理现有持仓的止盈止损和仓位管理。")
		case len(buyCandidates) > 0:
			priorityOrder = append(priorityOrder, "先等待关注股进入买入区，再逐个执行。")
		default:
			priorityOrder = append(priorityOrder, "维持观察，等待更明确的交易触发条件。")
		}
	}

	return map[string]interface{}{
		"buy_candidates":   buyCandidates,
		"position_actions": positionActions,
		"risk_controls":    dedupeStrings(riskControls),
		"priority_order":   dedupeStrings(priorityOrder),
	}
}
