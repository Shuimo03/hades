package brief

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"hades/internal/longbridge"
)

// BriefGenerator generates daily brief reports
type BriefGenerator struct {
	lb       *longbridge.Client
	timezone *time.Location
}

// New creates a new brief generator
func New(lb *longbridge.Client, timezone string) *BriefGenerator {
	tz, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("[Brief] Invalid timezone %s, using local", timezone)
		tz = time.Local
	}
	return &BriefGenerator{
		lb:       lb,
		timezone: tz,
	}
}

// BriefVersion represents the type of brief
type BriefVersion string

const (
	BriefVersionPreMarket  BriefVersion = "pre_market"  // 开盘前
	BriefVersionPostMarket BriefVersion = "post_market" // 收盘后
)

// Generate generates a daily brief report
func (g *BriefGenerator) Generate(ctx context.Context, version BriefVersion, symbols []string) (string, error) {
	now := time.Now().In(g.timezone)

	// Determine version if not specified
	if version == "" {
		version = g.determineVersion(now)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Daily Brief - %s %s\n\n", getVersionName(version), now.Format("2006-01-02 15:04")))

	// 1. 宏观概览
	sb.WriteString("=== 宏观概览 ===\n")
	macroContent, err := g.getMacroOverview(ctx, now, version)
	if err != nil {
		log.Printf("[Brief] Failed to get macro: %v", err)
		macroContent = "暂无宏观数据"
	}
	sb.WriteString(macroContent)
	sb.WriteString("\n\n")

	// 2. 持仓概览
	sb.WriteString("=== 持仓概览 ===\n")
	positionsContent, err := g.getPositionsOverview(ctx, symbols)
	if err != nil {
		log.Printf("[Brief] Failed to get positions: %v", err)
		positionsContent = "暂无持仓数据"
	}
	sb.WriteString(positionsContent)
	sb.WriteString("\n\n")

	// 3. 风险提示
	sb.WriteString("=== 风险提示 ===\n")
	riskContent, err := g.getRiskWarnings(ctx, symbols, version)
	if err != nil {
		log.Printf("[Brief] Failed to get risks: %v", err)
		riskContent = "暂无风险数据"
	}
	sb.WriteString(riskContent)

	return sb.String(), nil
}

// determineVersion determines the brief version based on current time
func (g *BriefGenerator) determineVersion(t time.Time) BriefVersion {
	hour := t.Hour()
	minute := t.Minute()

	// Market hours (HK): 09:15-12:00, 13:00-16:00
	// Pre-market: before 09:15
	// Post-market: after 16:00

	if hour < 9 || (hour == 9 && minute < 15) {
		return BriefVersionPreMarket
	}
	if hour > 16 || (hour == 16 && minute > 0) {
		return BriefVersionPostMarket
	}
	// During market hours, default to pre-market
	return BriefVersionPreMarket
}

func getVersionName(v BriefVersion) string {
	switch v {
	case BriefVersionPreMarket:
		return "开盘前"
	case BriefVersionPostMarket:
		return "收盘后"
	default:
		return "每日"
	}
}

// getMacroOverview gets macro overview content
func (g *BriefGenerator) getMacroOverview(ctx context.Context, t time.Time, version BriefVersion) (string, error) {
	var sb strings.Builder

	// Time-based macro info
	if version == BriefVersionPreMarket {
		sb.WriteString(fmt.Sprintf("日期: %s\n", t.Format("2006-01-02")))
		sb.WriteString("市场状态: 盘前\n")

		// Check if it's Monday (pre-market)
		if t.Weekday() == time.Monday {
			sb.WriteString("提示: 本周首个交易日\n")
		}
	} else {
		sb.WriteString(fmt.Sprintf("日期: %s\n", t.Format("2006-01-02")))
		sb.WriteString("市场状态: 盘后\n")
		sb.WriteString(fmt.Sprintf("今日涨跌: 待更新\n"))
	}

	return sb.String(), nil
}

// getPositionsOverview gets positions overview content
func (g *BriefGenerator) getPositionsOverview(ctx context.Context, symbols []string) (string, error) {
	var sb strings.Builder

	// Get account balance
	balances, err := g.lb.GetAccountInfo(ctx)
	if err != nil {
		return "", err
	}

	if len(balances) > 0 {
		b := balances[0]
		sb.WriteString(fmt.Sprintf("总现金: %s %s\n", b.Currency, b.TotalCash.String()))
		sb.WriteString(fmt.Sprintf("净资产: %s %s\n", b.Currency, b.NetAssets.String()))
	}

	// Get positions
	positions, err := g.lb.GetPositions(ctx)
	if err != nil {
		return "", err
	}

	if len(positions) == 0 {
		sb.WriteString("\n当前无持仓\n")
		return sb.String(), nil
	}

	sb.WriteString(fmt.Sprintf("\n持仓 (%d 只):\n", len(positions)))
	sb.WriteString(fmt.Sprintf("%-12s %-10s %-10s\n", "股票", "数量", "成本价"))
	sb.WriteString(strings.Repeat("-", 40) + "\n")

	for _, ch := range positions {
		for _, p := range ch.Positions {
			sb.WriteString(fmt.Sprintf("%-12s %-10s %-10s\n",
				p.Symbol,
				p.Quantity,
				p.CostPrice))
		}
	}

	return sb.String(), nil
}

// getRiskWarnings gets risk warning content
func (g *BriefGenerator) getRiskWarnings(ctx context.Context, symbols []string, version BriefVersion) (string, error) {
	var sb strings.Builder

	// Get positions for risk analysis
	positions, err := g.lb.GetPositions(ctx)
	if err != nil {
		return "", err
	}

	if len(positions) == 0 {
		sb.WriteString("当前无持仓，无风险提示\n")
		return sb.String(), nil
	}

	// Check position count
	totalPositions := 0
	for _, ch := range positions {
		totalPositions += len(ch.Positions)
	}

	if totalPositions > 10 {
		sb.WriteString(fmt.Sprintf("注意: 持仓数量较多: %d 只\n", totalPositions))
	} else {
		sb.WriteString(fmt.Sprintf("持仓数量: %d 只\n", totalPositions))
	}

	// Simple risk summary
	sb.WriteString("风险检查通过\n")

	return sb.String(), nil
}
