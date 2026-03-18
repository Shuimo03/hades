package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/longportapp/openapi-go/quote"
	"hades/internal/longbridge"
)

// AlertType represents the type of alert
type AlertType string

const (
	AlertTypePrice      AlertType = "price"      // 价格提醒
	AlertTypeVolatility AlertType = "volatility" // 波动率提醒
	AlertTypeTrend      AlertType = "trend"      // 趋势破位
	AlertTypeEarnings   AlertType = "earnings"   // 财报提醒
	AlertTypeVolume     AlertType = "volume"     // 成交量异常
)

// AlertCondition represents the trigger condition
type AlertCondition string

const (
	AlertConditionAbove          AlertCondition = "above"      // 价格高于
	AlertConditionBelow          AlertCondition = "below"      // 价格低于
	AlertConditionCrossUp        AlertCondition = "cross_up"   // 上穿
	AlertConditionCrossDown      AlertCondition = "cross_down" // 下穿
	AlertConditionInBuyZone      AlertCondition = "in_buy_zone"
	AlertConditionNearTakeProfit AlertCondition = "near_take_profit"
	AlertConditionBelowStopLoss  AlertCondition = "below_stop_loss"
)

// Alert represents a signal alert
type Alert struct {
	ID           string         `json:"id"`
	Symbol       string         `json:"symbol"`
	AlertType    AlertType      `json:"alert_type"`
	Condition    AlertCondition `json:"condition"`
	Threshold    float64        `json:"threshold"`
	SessionScope string         `json:"session_scope,omitempty"`
	Note         string         `json:"note,omitempty"`
	Enabled      bool           `json:"enabled"`
	CreatedAt    time.Time      `json:"created_at"`
	LastCheck    time.Time      `json:"last_check,omitempty"`
	Triggered    bool           `json:"triggered,omitempty"`
	TriggeredAt  time.Time      `json:"triggered_at,omitempty"`
}

// Manager manages signal alerts
type Manager struct {
	mu            sync.RWMutex
	alerts        map[string]*Alert
	lb            *longbridge.Client
	quoteScope    longbridge.QuoteSessionScope
	checkInterval time.Duration
	storagePath   string
	callback      func(alert *Alert, message string) // callback when alert triggers
}

// New creates a new alert manager
func New(lb *longbridge.Client, storagePath string, checkInterval time.Duration, quoteScope longbridge.QuoteSessionScope) *Manager {
	return &Manager{
		alerts:        make(map[string]*Alert),
		lb:            lb,
		quoteScope:    quoteScope,
		checkInterval: checkInterval,
		storagePath:   storagePath,
	}
}

// SetCallback sets the callback function for alert triggers
func (m *Manager) SetCallback(fn func(alert *Alert, message string)) {
	m.callback = fn
}

func (m *Manager) QuoteScopeForAlert(alert *Alert) longbridge.QuoteSessionScope {
	if alert != nil {
		if scope, valid := longbridge.ParseQuoteSessionScope(alert.SessionScope); valid && alert.SessionScope != "" {
			return scope
		}
	}
	return m.quoteScope
}

// Load loads alerts from storage
func (m *Manager) Load() error {
	if m.storagePath == "" {
		return nil
	}

	data, err := os.ReadFile(m.storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var alerts []*Alert
	if err := json.Unmarshal(data, &alerts); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range alerts {
		m.alerts[a.ID] = a
	}

	log.Printf("[Alerts] Loaded %d alerts", len(alerts))
	return nil
}

// Save saves alerts to storage
func (m *Manager) Save() error {
	if m.storagePath == "" {
		return nil
	}

	m.mu.RLock()
	alerts := make([]*Alert, 0, len(m.alerts))
	for _, a := range m.alerts {
		alerts = append(alerts, a)
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(alerts, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.storagePath, data, 0644)
}

// Create creates a new alert
func (m *Manager) Create(alert *Alert) error {
	if alert.ID == "" {
		alert.ID = generateID()
	}
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = time.Now()
	}
	alert.Enabled = true

	m.mu.Lock()
	defer m.mu.Unlock()

	m.alerts[alert.ID] = alert
	log.Printf("[Alerts] Created alert: %s for %s", alert.ID, alert.Symbol)

	// Save to storage
	go m.Save()

	return nil
}

// Get returns an alert by ID
func (m *Manager) Get(id string) (*Alert, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, ok := m.alerts[id]
	return alert, ok
}

// List returns all alerts
func (m *Manager) List() []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*Alert, 0, len(m.alerts))
	for _, a := range m.alerts {
		alerts = append(alerts, a)
	}
	return alerts
}

// Delete deletes an alert by ID
func (m *Manager) Delete(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.alerts[id]; !ok {
		return false
	}

	delete(m.alerts, id)
	log.Printf("[Alerts] Deleted alert: %s", id)

	// Save to storage
	go m.Save()

	return true
}

// Enable enables or disables an alert
func (m *Manager) Enable(id string, enabled bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[id]
	if !ok {
		return false
	}

	alert.Enabled = enabled
	alert.Triggered = false // Reset triggered state
	log.Printf("[Alerts] Alert %s enabled: %v", id, enabled)

	// Save to storage
	go m.Save()

	return true
}

// CheckAll checks all enabled alerts
func (m *Manager) CheckAll(ctx context.Context) {
	alerts := m.List()
	if len(alerts) == 0 {
		return
	}

	// Collect symbols to check
	symbols := make(map[string][]*Alert)
	for _, a := range alerts {
		if a.Enabled && !a.Triggered {
			symbols[a.Symbol] = append(symbols[a.Symbol], a)
		}
	}

	if len(symbols) == 0 {
		return
	}

	// Get quotes for all symbols
	symbolList := make([]string, 0, len(symbols))
	for sym := range symbols {
		symbolList = append(symbolList, sym)
	}

	quotes, err := m.lb.GetQuote(ctx, symbolList)
	if err != nil {
		log.Printf("[Alerts] Failed to get quotes: %v", err)
		return
	}

	// Build quote map
	quoteMap := make(map[string]interface{})
	for _, q := range quotes {
		if q != nil {
			quoteMap[q.Symbol] = q
		}
	}

	// Check each alert
	for sym, alertList := range symbols {
		for _, alert := range alertList {
			m.checkAlert(ctx, alert, quoteMap[sym])
		}
	}
}

// checkAlert checks a single alert
func (m *Manager) checkAlert(ctx context.Context, alert *Alert, securityQuote interface{}) {
	q, ok := securityQuote.(*quote.SecurityQuote)
	if !ok || q == nil {
		return
	}

	alert.LastCheck = time.Now()
	message := ""
	alertScope := m.QuoteScopeForAlert(alert)
	effectiveQuote := longbridge.ResolveEffectiveQuote(q, alertScope)

	switch alert.AlertType {
	case AlertTypePrice:
		message = m.checkPriceAlert(alert, effectiveQuote)
	case AlertTypeVolatility:
		if effectiveQuote.Session != longbridge.QuoteSessionRegular {
			return
		}
		message = m.checkVolatilityAlert(alert, q)
	case AlertTypeVolume:
		if effectiveQuote.Session != longbridge.QuoteSessionRegular {
			return
		}
		message = m.checkVolumeAlert(ctx, alert, q)
	case AlertTypeTrend:
		message = m.checkTrendAlert(ctx, alert, effectiveQuote)
	}

	if message != "" && m.callback != nil {
		alert.Triggered = true
		alert.TriggeredAt = time.Now()
		m.callback(alert, message)
		log.Printf("[Alerts] Alert %s triggered: %s", alert.ID, message)
	}
}

func (m *Manager) checkPriceAlert(alert *Alert, q longbridge.EffectiveQuote) string {
	if !q.HasQuote {
		return ""
	}

	price := q.Price
	sessionName := longbridge.QuoteSessionDisplayName(q.Session)

	switch alert.Condition {
	case AlertConditionAbove:
		if price > alert.Threshold {
			return fmt.Sprintf("[价格提醒][%s] %s 达到 %.2f (当前: %.2f)", sessionName, alert.Symbol, alert.Threshold, price)
		}
	case AlertConditionBelow:
		if price < alert.Threshold {
			return fmt.Sprintf("[价格提醒][%s] %s 跌破 %.2f (当前: %.2f)", sessionName, alert.Symbol, alert.Threshold, price)
		}
	}

	return ""
}

func (m *Manager) checkVolatilityAlert(alert *Alert, q *quote.SecurityQuote) string {
	if q.High == nil || q.Low == nil || q.Open == nil {
		return ""
	}

	high, _ := q.High.Float64()
	low, _ := q.Low.Float64()
	open, _ := q.Open.Float64()
	if high == 0 || low == 0 || open == 0 {
		return ""
	}

	// Calculate volatility as percentage
	volatility := (high - low) / open * 100

	if volatility > alert.Threshold {
		return fmt.Sprintf("[波动率提醒] %s 波动率 %.1f%% (最高: %.2f, 最低: %.2f)",
			alert.Symbol, volatility, high, low)
	}

	return ""
}

func (m *Manager) checkVolumeAlert(ctx context.Context, alert *Alert, q *quote.SecurityQuote) string {
	candles, err := m.lb.GetCandlesticks(ctx, alert.Symbol, quote.Period(1), 20)
	if err != nil || len(candles) < 2 {
		return ""
	}

	var total int64
	var samples int64
	for _, candle := range candles[1:] {
		if candle == nil {
			continue
		}
		total += candle.Volume
		samples++
	}
	if samples == 0 {
		return ""
	}

	average := float64(total) / float64(samples)
	if average == 0 {
		return ""
	}

	current := float64(q.Volume)
	multiple := current / average

	switch alert.Condition {
	case AlertConditionAbove:
		if multiple >= alert.Threshold {
			return fmt.Sprintf("[成交量提醒] %s 当前成交量为近 %d 日均量的 %.2f 倍",
				alert.Symbol, samples, multiple)
		}
	case AlertConditionBelow:
		if multiple <= alert.Threshold {
			return fmt.Sprintf("[成交量提醒] %s 当前成交量仅为近 %d 日均量的 %.2f 倍",
				alert.Symbol, samples, multiple)
		}
	}

	return ""
}

func (m *Manager) checkTrendAlert(ctx context.Context, alert *Alert, q longbridge.EffectiveQuote) string {
	if !q.HasQuote {
		return ""
	}

	metrics, err := BuildAlertPlanMetrics(ctx, m.lb, alert.Symbol, m.QuoteScopeForAlert(alert))
	if err != nil {
		return ""
	}

	price := q.Price
	sessionName := longbridge.QuoteSessionDisplayName(q.Session)
	switch alert.Condition {
	case AlertConditionInBuyZone:
		if (metrics.Mode == "pullback" || metrics.Mode == "range") &&
			metrics.RRQualified &&
			metrics.BuyHigh > 0 &&
			price >= metrics.BuyLow && price <= metrics.BuyHigh {
			return fmt.Sprintf("[计划提醒][%s] %s 进入%s计划区间 %.2f - %.2f (当前: %.2f)",
				sessionName, alert.Symbol, metrics.ModeLabel, metrics.BuyLow, metrics.BuyHigh, price)
		}
	case AlertConditionNearTakeProfit:
		bufferPct := alert.Threshold
		if bufferPct <= 0 {
			bufferPct = 1.0
		}
		triggerPrice := metrics.TakeProfit * (1 - bufferPct/100)
		if metrics.IsHeld && price >= triggerPrice {
			return fmt.Sprintf("[持仓提醒][%s] %s 接近 TP1 %.2f (当前: %.2f, 提前 %.2f%% 提醒)",
				sessionName, alert.Symbol, metrics.TakeProfit, price, bufferPct)
		}
	case AlertConditionBelowStopLoss:
		if metrics.IsHeld && price <= metrics.StopLoss {
			return fmt.Sprintf("[失效提醒][%s] %s 跌破失效位 %.2f (当前: %.2f)",
				sessionName, alert.Symbol, metrics.StopLoss, price)
		}
	case AlertConditionCrossUp:
		if metrics.Mode == "breakout" && price >= metrics.BreakoutConfirm {
			return fmt.Sprintf("[突破提醒][%s] %s 接近突破确认价 %.2f (当前: %.2f, 追价上限: %.2f)",
				sessionName, alert.Symbol, metrics.BreakoutConfirm, price, metrics.ChaseLimit)
		}
	case AlertConditionCrossDown:
		if price < metrics.StopLoss {
			return fmt.Sprintf("[趋势提醒][%s] %s 跌破关键失效位 %.2f (当前: %.2f)",
				sessionName, alert.Symbol, metrics.StopLoss, price)
		}
	}

	return ""
}

func generateID() string {
	return fmt.Sprintf("alert_%d", time.Now().UnixNano())
}
