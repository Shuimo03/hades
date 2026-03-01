package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

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
	AlertConditionAbove    AlertCondition = "above"    // 价格高于
	AlertConditionBelow    AlertCondition = "below"   // 价格低于
	AlertConditionCrossUp  AlertCondition = "cross_up" // 上穿
	AlertConditionCrossDown AlertCondition = "cross_down" // 下穿
)

// Alert represents a signal alert
type Alert struct {
	ID         string       `json:"id"`
	Symbol     string       `json:"symbol"`
	AlertType  AlertType    `json:"alert_type"`
	Condition  AlertCondition `json:"condition"`
	Threshold  float64     `json:"threshold"`
	Note       string      `json:"note,omitempty"`
	Enabled    bool        `json:"enabled"`
	CreatedAt time.Time   `json:"created_at"`
	LastCheck  time.Time   `json:"last_check,omitempty"`
	Triggered  bool        `json:"triggered,omitempty"`
	TriggeredAt time.Time `json:"triggered_at,omitempty"`
}

// Manager manages signal alerts
type Manager struct {
	mu          sync.RWMutex
	alerts      map[string]*Alert
	lb          *longbridge.Client
	checkInterval time.Duration
	storagePath string
	callback    func(alert *Alert, message string) // callback when alert triggers
}

// New creates a new alert manager
func New(lb *longbridge.Client, storagePath string, checkInterval time.Duration) *Manager {
	return &Manager{
		alerts:        make(map[string]*Alert),
		lb:           lb,
		checkInterval: checkInterval,
		storagePath:   storagePath,
	}
}

// SetCallback sets the callback function for alert triggers
func (m *Manager) SetCallback(fn func(alert *Alert, message string)) {
	m.callback = fn
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
	for _, a := range		alerts = append(alerts, a)
 m.alerts {
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
func (m *Manager) checkAlert(ctx context.Context, alert *Alert, quote interface{}) {
	if quote == nil {
		return
	}

	alert.LastCheck = time.Now()
	message := ""

	switch alert.AlertType {
	case AlertTypePrice:
		message = m.checkPriceAlert(alert, quote)
	case AlertTypeVolatility:
		message = m.checkVolatilityAlert(alert, quote)
	case AlertTypeVolume:
		message = m.checkVolumeAlert(alert, quote)
	}

	if message != "" && m.callback != nil {
		alert.Triggered = true
		alert.TriggeredAt = time.Now()
		m.callback(alert, message)
		log.Printf("[Alerts] Alert %s triggered: %s", alert.ID, message)
	}
}

func (m *Manager) checkPriceAlert(alert *Alert, quote interface{}) string {
	// Type assertion to get price
	type quoteInterface interface {
		GetLastDone() interface{}
	}

	q, ok := quote.(quoteInterface)
	if !ok {
		return ""
	}

	lastDone := q.GetLastDone()
	if lastDone == nil {
		return ""
	}

	// Try to convert to float64
	var price float64
	switch v := lastDone.(type) {
	case float64:
		price = v
	default:
		return ""
	}

	switch alert.Condition {
	case AlertConditionAbove:
		if price > alert.Threshold {
			return fmt.Sprintf("🔔 %s 达到 %.2f (当前: %.2f)", alert.Symbol, alert.Threshold, price)
		}
	case AlertConditionBelow:
		if price < alert.Threshold {
			return fmt.Sprintf("🔔 %s 跌破 %.2f (当前: %.2f)", alert.Symbol, alert.Threshold, price)
		}
	}

	return ""
}

func (m *Manager) checkVolatilityAlert(alert *Alert, quote interface{}) string {
	// For volatility, we need high/low from quote
	type volatilityQuote interface {
		GetHigh() interface{}
		GetLow() interface{}
		GetOpen() interface{}
	}

	q, ok := quote.(volatilityQuote)
	if !ok {
		return ""
	}

	// Get values
	getFloat := func(v interface{}) float64 {
		switch val := v.(type) {
		case float64:
			return val
		default:
			return 0
		}
	}

	high := getFloat(q.GetHigh())
	low := getFloat(q.GetLow())
	open := getFloat(q.GetOpen())

	if high == 0 || low == 0 {
		return ""
	}

	// Calculate volatility as percentage
	volatility := (high - low) / open * 100

	if volatility > alert.Threshold {
		return fmt.Sprintf("⚡ %s 波动率飙升 %.1f%% (最高: %.2f, 最低: %.2f)",
			alert.Symbol, volatility, high, low)
	}

	return ""
}

func (m *Manager) checkVolumeAlert(alert *Alert, quote interface{}) string {
	type volumeQuote interface {
		GetVolume() int64
		GetLastDone() interface{}
	}

	q, ok := quote.(volumeQuote)
	if !ok {
		return ""
	}

	// For volume alert, we need to compare with average
	// This is a simplified version - in production, you'd want historical average
	volume := q.GetVolume()

	// Threshold represents multiplier (e.g., 2.0 means 2x average)
	// This would need historical data in production
	_ = alert.Threshold

	// Placeholder - would need to track average volume over time
	return ""
}

func generateID() string {
	return fmt.Sprintf("alert_%d", time.Now().UnixNano())
}
