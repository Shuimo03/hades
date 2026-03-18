package alerts

import (
	"fmt"
	"strings"
	"time"

	"hades/internal/longbridge"
	"hades/internal/planlevels"
)

type planMode string

const (
	planModePullback planMode = "pullback"
	planModeBreakout planMode = "breakout"
	planModeRange    planMode = "range"
	planModeEvent    planMode = "event"
)

const minExecutableRR = 1.8

type derivedPlan struct {
	Mode                 planMode
	Status               string
	StatusReason         string
	Conclusion           string
	EntryCondition       string
	InvalCondition       string
	TargetSummary        string
	BuyLow               float64
	BuyHigh              float64
	InvalPrice           float64
	TP1                  float64
	TP2                  float64
	RR                   float64
	RRQualified          bool
	BreakoutConfirm      float64
	BreakoutExit         float64
	ChaseLimit           float64
	VolumeRequirement    string
	EventRisk            bool
	EventNotes           []string
	Overextended         bool
	Action               string
	PositionSummary      string
	OldPlanInvalidated   bool
	OldPlanInvalidReason string
}

func deriveStrategyPlan(symbol string, currentPrice float64, trend string, score int, daily, weekly planSnapshot, isHeld bool, costPrice float64, news []*longbridge.StockNewsItem) derivedPlan {
	buyLow, buyHigh := planlevels.BuyZone(trend, currentPrice, daily.Support, weekly.Support)
	baseStop := planlevels.StopLoss(trend, currentPrice, daily.Support, weekly.Support)
	baseTP1 := planlevels.TakeProfit(currentPrice, daily.Resistance, weekly.Resistance)
	baseTP2 := secondaryTarget(currentPrice, daily.Resistance, weekly.Resistance, baseTP1)
	baseRR := calcRR(buyHigh, baseStop, baseTP1)
	baseRRQualified := baseRR >= minExecutableRR

	eventRisk, eventNotes := detectEventRisk(news)
	nearestResistance := minNonZeroFloat(daily.Resistance, weekly.Resistance)
	if nearestResistance <= 0 {
		nearestResistance = round2(currentPrice * 1.06)
	}
	upperResistance := maxNonZeroFloat(daily.Resistance, weekly.Resistance)
	if upperResistance <= 0 {
		upperResistance = round2(currentPrice * 1.12)
	}

	roomToResistancePct := pctDelta(currentPrice, nearestResistance)
	distMA10Pct := pctDistance(currentPrice, daily.MA10)
	distMA20Pct := pctDistance(currentPrice, daily.MA20)
	overextended := currentPrice > daily.MA20*1.08 || currentPrice > buyHigh*1.03
	breakoutConfirm := round2(maxNonZeroFloat(nearestResistance*1.002, currentPrice))
	breakoutExit := round2(maxNonZeroFloat(nearestResistance*0.985, daily.MA20*0.99))
	if breakoutExit >= breakoutConfirm {
		breakoutExit = round2(breakoutConfirm * 0.985)
	}
	chaseLimit := round2(breakoutConfirm * 1.02)
	breakoutTP1 := round2(breakoutConfirm + (breakoutConfirm-breakoutExit)*2)
	breakoutTP2 := round2(breakoutConfirm + (breakoutConfirm-breakoutExit)*3.2)
	breakoutRR := calcRR(breakoutConfirm, breakoutExit, breakoutTP1)

	rangeLow, rangeHigh := tightenRangeZone(daily.Support, nearestResistance, currentPrice)
	rangeStop := round2(rangeLow * 0.985)
	rangeTP1 := round2((rangeLow + nearestResistance) / 2)
	rangeTP2 := round2(nearestResistance * 0.995)
	rangeRR := calcRR(rangeHigh, rangeStop, rangeTP2)

	pullbackCandidate := (trend == "bullish" || score >= 65) &&
		distMA10Pct <= 3.5 &&
		distMA20Pct <= 5.0 &&
		roomToResistancePct >= 5 &&
		!eventRisk
	breakoutCandidate := !eventRisk &&
		(score >= 60 || trend == "bullish") &&
		daily.VolumeRatio >= 1.2 &&
		(currentPrice >= nearestResistance*0.995 || currentPrice > buyHigh*1.01 || currentPrice >= baseTP1)
	rangeCandidate := trend == "neutral" || score < 60 || roomToResistancePct < 5

	mode := planModeRange
	switch {
	case eventRisk:
		mode = planModeEvent
	case breakoutCandidate:
		mode = planModeBreakout
	case pullbackCandidate:
		mode = planModePullback
	case rangeCandidate:
		mode = planModeRange
	}

	volumeRequirement := breakoutVolumeRequirement(daily.VolumeRatio)
	positionSummary := positionContext(isHeld, costPrice, currentPrice)
	oldPlanInvalidated := !isHeld && (currentPrice >= buyHigh*1.03 || currentPrice >= nearestResistance*0.995 || currentPrice <= baseStop || eventRisk)
	oldPlanInvalidReason := ""
	switch {
	case !isHeld && (currentPrice >= buyHigh*1.03 || currentPrice >= nearestResistance*0.995):
		oldPlanInvalidReason = "当前价已脱离原买入区或接近原阻力，旧回踩计划作废"
	case !isHeld && currentPrice <= baseStop:
		oldPlanInvalidReason = "当前价已跌破原计划失效位，旧计划作废"
	case !isHeld && eventRisk:
		oldPlanInvalidReason = "事件窗口抬升波动，旧机械计划降权"
	}

	plan := derivedPlan{
		Mode:                 mode,
		Status:               "active",
		BuyLow:               buyLow,
		BuyHigh:              buyHigh,
		InvalPrice:           baseStop,
		TP1:                  baseTP1,
		TP2:                  baseTP2,
		RR:                   round2(baseRR),
		RRQualified:          baseRRQualified,
		BreakoutConfirm:      breakoutConfirm,
		BreakoutExit:         breakoutExit,
		ChaseLimit:           chaseLimit,
		VolumeRequirement:    volumeRequirement,
		EventRisk:            eventRisk,
		EventNotes:           eventNotes,
		Overextended:         overextended,
		PositionSummary:      positionSummary,
		OldPlanInvalidated:   oldPlanInvalidated,
		OldPlanInvalidReason: oldPlanInvalidReason,
	}

	switch mode {
	case planModePullback:
		plan.Status = planStatus(baseRRQualified, oldPlanInvalidated)
		plan.StatusReason = firstNonEmpty(oldPlanInvalidReason, rrReason(baseRRQualified))
		plan.Conclusion = pullbackConclusion(isHeld, baseRRQualified, oldPlanInvalidated)
		plan.EntryCondition = fmt.Sprintf("仅在 %.2f-%.2f 回踩区承接，最好缩量回踩后重新站稳 MA10/MA20；%s。", buyLow, buyHigh, meanReversionVolumeRequirement(daily.VolumeRatio))
		plan.InvalCondition = fmt.Sprintf("跌破 %.2f 则回踩逻辑失效；若事件临近，也不做机械挂单。", baseStop)
		plan.TargetSummary = formatTargetSummary(baseTP1, baseTP2, baseRR, baseRRQualified)
		plan.Action = pullbackAction(isHeld, baseRRQualified, oldPlanInvalidated)
	case planModeBreakout:
		plan.Status = planStatus(breakoutRR >= minExecutableRR && !overextended, oldPlanInvalidated)
		plan.StatusReason = firstNonEmpty(oldPlanInvalidReason, breakoutStatusReason(overextended))
		plan.BuyLow = 0
		plan.BuyHigh = 0
		plan.InvalPrice = breakoutExit
		plan.TP1 = breakoutTP1
		plan.TP2 = breakoutTP2
		plan.RR = round2(breakoutRR)
		plan.RRQualified = breakoutRR >= minExecutableRR && !overextended
		plan.Conclusion = breakoutConclusion(isHeld, overextended)
		plan.EntryCondition = fmt.Sprintf("只有放量站稳 %.2f 上方才算突破确认，追价不超过 %.2f；量能要求 %s。", breakoutConfirm, chaseLimit, volumeRequirement)
		plan.InvalCondition = fmt.Sprintf("跌回 %.2f 下方视为假突破，空仓不追，持仓改用移动止盈。", breakoutExit)
		plan.TargetSummary = formatTargetSummary(breakoutTP1, breakoutTP2, breakoutRR, plan.RRQualified)
		plan.Action = breakoutAction(isHeld, overextended)
	case planModeEvent:
		plan.Status = "defer"
		plan.StatusReason = strings.Join(eventNotes, "；")
		plan.Conclusion = eventConclusion(isHeld)
		plan.EntryCondition = fmt.Sprintf("只接受事件后确认或极小仓位试单；若参与，也要满足价格确认和量能不低于 %s。", volumeRequirement)
		plan.InvalCondition = fmt.Sprintf("事件前不做机械新开仓；若冲高回落并失守 %.2f，则视为无效。", maxNonZeroFloat(baseStop, breakoutExit))
		plan.TargetSummary = formatEventTargetSummary(baseTP1, baseTP2, baseRRQualified, baseRR)
		plan.Action = eventAction(isHeld)
	default:
		plan.Status = planStatus(rangeRR >= minExecutableRR, oldPlanInvalidated)
		plan.StatusReason = firstNonEmpty(oldPlanInvalidReason, rrReason(rangeRR >= minExecutableRR))
		plan.BuyLow = rangeLow
		plan.BuyHigh = rangeHigh
		plan.InvalPrice = rangeStop
		plan.TP1 = rangeTP1
		plan.TP2 = rangeTP2
		plan.RR = round2(rangeRR)
		plan.RRQualified = rangeRR >= minExecutableRR
		plan.Conclusion = rangeConclusion(isHeld, trend, rangeRR >= minExecutableRR)
		plan.EntryCondition = fmt.Sprintf("仅在 %.2f-%.2f 低吸区考虑试单，未回到区间下沿不出手。", rangeLow, rangeHigh)
		plan.InvalCondition = fmt.Sprintf("跌破 %.2f 退出区间思路；若放量突破 %.2f，则切换到突破模式。", rangeStop, nearestResistance)
		plan.TargetSummary = formatTargetSummary(rangeTP1, rangeTP2, rangeRR, plan.RRQualified)
		plan.Action = rangeAction(isHeld, trend, rangeRR >= minExecutableRR)
	}

	return plan
}

func detectEventRisk(news []*longbridge.StockNewsItem) (bool, []string) {
	if len(news) == 0 {
		return false, nil
	}

	type keywordGroup struct {
		tag      string
		keywords []string
	}

	groups := []keywordGroup{
		{tag: "财报/指引", keywords: []string{"earnings", "guidance", "revenue", "profit warning", "results", "财报", "业绩", "指引"}},
		{tag: "产品/大会", keywords: []string{"gtc", "launch", "event", "conference", "keynote", "发布会", "大会", "新品"}},
		{tag: "宏观", keywords: []string{"fomc", "cpi", "nonfarm", "payroll", "fed", "利率", "通胀", "非农"}},
		{tag: "地缘/油价", keywords: []string{"middle east", "hormuz", "supply disruption", "oil", "crude", "sanction", "attack", "冲突", "油价", "霍尔木兹", "制裁"}},
	}

	tags := make([]string, 0, 4)
	seen := make(map[string]struct{})
	now := time.Now()

	for _, item := range news {
		if item == nil {
			continue
		}
		title := strings.ToLower(strings.TrimSpace(item.Title))
		if title == "" {
			continue
		}
		if !isRecentEventHeadline(item.PublishedAt, now) {
			continue
		}
		for _, group := range groups {
			for _, keyword := range group.keywords {
				if strings.Contains(title, keyword) {
					if _, ok := seen[group.tag]; !ok {
						seen[group.tag] = struct{}{}
						tags = append(tags, group.tag)
					}
					break
				}
			}
		}
	}

	return len(tags) > 0, tags
}

func isRecentEventHeadline(publishedAt string, now time.Time) bool {
	if strings.TrimSpace(publishedAt) == "" {
		return true
	}
	ts, err := time.Parse(time.RFC3339, publishedAt)
	if err != nil {
		return true
	}
	return now.Sub(ts) <= 72*time.Hour
}

func secondaryTarget(currentPrice, dailyResistance, weeklyResistance, primary float64) float64 {
	target := maxNonZeroFloat(dailyResistance, weeklyResistance)
	switch {
	case target <= 0:
		return round2(maxNonZeroFloat(primary*1.08, currentPrice*1.12))
	case round2(target*0.995) <= primary:
		return round2(maxNonZeroFloat(primary*1.06, currentPrice*1.12))
	default:
		return round2(target * 0.995)
	}
}

func tightenRangeZone(support, resistance, currentPrice float64) (float64, float64) {
	baseLow := maxNonZeroFloat(support*0.995, currentPrice*0.96)
	baseHigh := minNonZeroFloat(support*1.015, resistance*0.94)
	if baseHigh <= 0 || baseHigh <= baseLow {
		baseHigh = round2(currentPrice * 0.99)
	}
	if baseLow <= 0 || baseLow >= baseHigh {
		baseLow = round2(baseHigh * 0.985)
	}
	return round2(baseLow), round2(baseHigh)
}

func calcRR(entry, invalidation, target float64) float64 {
	risk := entry - invalidation
	if entry <= 0 || invalidation <= 0 || target <= 0 || risk <= 0 {
		return 0
	}
	return (target - entry) / risk
}

func pctDistance(price, reference float64) float64 {
	if price <= 0 || reference <= 0 {
		return 0
	}
	return mathAbs(price-reference) / reference * 100
}

func pctDelta(price, target float64) float64 {
	if price <= 0 || target <= 0 {
		return 0
	}
	return (target - price) / price * 100
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func breakoutVolumeRequirement(volumeRatio float64) string {
	if volumeRatio >= 1.5 {
		return "当前量能已达 1.5x 以上 20 日均量，需继续维持"
	}
	return "至少达到 1.5x 20 日均量"
}

func meanReversionVolumeRequirement(volumeRatio float64) string {
	if volumeRatio >= 1.5 {
		return "当前量能偏大，等回踩缩量后再看"
	}
	return "回踩阶段尽量缩量，反弹时再恢复到均量上方"
}

func planStatus(rrQualified bool, invalidated bool) string {
	switch {
	case invalidated:
		return "invalidated"
	case rrQualified:
		return "active"
	default:
		return "observe"
	}
}

func rrReason(rrQualified bool) string {
	if rrQualified {
		return ""
	}
	return fmt.Sprintf("RR 低于 %.1f，暂不作为可执行新单", minExecutableRR)
}

func breakoutStatusReason(overextended bool) string {
	if overextended {
		return "已明显拉伸，空仓不追"
	}
	return ""
}

func formatTargetSummary(tp1, tp2, rr float64, qualified bool) string {
	status := "不合格"
	if qualified {
		status = "合格"
	}
	return fmt.Sprintf("TP1 %.2f / TP2 %.2f / RR %.2f (%s)", tp1, tp2, rr, status)
}

func formatEventTargetSummary(tp1, tp2 float64, qualified bool, rr float64) string {
	if !qualified {
		return fmt.Sprintf("事件模式不提供机械新开仓建议；参考 TP1 %.2f / TP2 %.2f / RR %.2f", tp1, tp2, rr)
	}
	return fmt.Sprintf("参考 TP1 %.2f / TP2 %.2f / RR %.2f，但事件窗口内需降权执行", tp1, tp2, rr)
}

func pullbackConclusion(isHeld, rrQualified, invalidated bool) string {
	switch {
	case invalidated && !isHeld:
		return "原回踩计划已失效，空仓不按旧买点执行。"
	case !rrQualified:
		return "回踩结构在，但 RR 不足，先观望。"
	case isHeld:
		return "持仓仍按回踩结构跟踪，不盲目加仓，优先守失效位。"
	default:
		return "可低吸，但只接受回踩确认，不追高。"
	}
}

func breakoutConclusion(isHeld, overextended bool) string {
	switch {
	case isHeld:
		return "已脱离原买入区，持仓改用移动止盈，不再沿用旧回踩计划。"
	case overextended:
		return "当前远离原买入区且拉伸偏大，空仓暂不追。"
	default:
		return "进入突破观察，只在确认后跟随，不预判。"
	}
}

func rangeConclusion(isHeld bool, trend string, rrQualified bool) string {
	switch {
	case isHeld && trend == "bearish":
		return "区间偏弱，只减仓不加仓。"
	case isHeld:
		return "按区间思路管理，靠近上沿减仓，不追高加仓。"
	case !rrQualified:
		return "上下空间有限且 RR 不足，观望为主。"
	default:
		return "低吸高抛可以考虑，但只做区间下沿，不追高。"
	}
}

func eventConclusion(isHeld bool) string {
	if isHeld {
		return "事件波动高，优先轻仓管理或减少隔夜，不按静态计划死扛。"
	}
	return "事件窗口内不做机械挂单，只接受小仓位确认单。"
}

func pullbackAction(isHeld, rrQualified, invalidated bool) string {
	switch {
	case invalidated && !isHeld:
		return "observe"
	case !rrQualified:
		return "observe"
	case isHeld:
		return "hold_pullback"
	default:
		return "watch_pullback_buy"
	}
}

func breakoutAction(isHeld, overextended bool) string {
	switch {
	case isHeld:
		return "manage_breakout_winner"
	case overextended:
		return "watch_breakout"
	default:
		return "watch_breakout"
	}
}

func rangeAction(isHeld bool, trend string, rrQualified bool) string {
	switch {
	case isHeld && trend == "bearish":
		return "reduce_or_wait"
	case isHeld:
		return "range_manage"
	case !rrQualified:
		return "observe"
	default:
		return "range_trade"
	}
}

func eventAction(isHeld bool) string {
	if isHeld {
		return "event_reduce_risk"
	}
	return "event_wait"
}

func positionContext(isHeld bool, costPrice, currentPrice float64) string {
	if !isHeld || costPrice <= 0 {
		return "空仓视角"
	}
	switch {
	case currentPrice > costPrice:
		return fmt.Sprintf("持仓视角: 成本 %.2f，上方盈利中", round2(costPrice))
	case currentPrice < costPrice:
		return fmt.Sprintf("持仓视角: 成本 %.2f，当前位置低于成本", round2(costPrice))
	default:
		return fmt.Sprintf("持仓视角: 成本 %.2f，接近成本线", round2(costPrice))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
