package alerts

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// ExecutionWindow represents a scheduled execution window
type ExecutionWindow struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Schedule   string    `json:"schedule"` // cron expression
	Strategy   string    `json:"strategy"`
	WebhookURL string    `json:"webhook_url,omitempty"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	LastRun    time.Time `json:"last_run,omitempty"`
}

// ExecutionWindowManager manages execution windows
type ExecutionWindowManager struct {
	mu          sync.RWMutex
	windows     map[string]*ExecutionWindow
	storagePath string
	callback    func(window *ExecutionWindow) // callback when window triggers
}

// NewExecutionWindowManager creates a new execution window manager
func NewExecutionWindowManager(storagePath string) *ExecutionWindowManager {
	return &ExecutionWindowManager{
		windows:     make(map[string]*ExecutionWindow),
		storagePath: storagePath,
	}
}

// SetCallback sets the callback function for window triggers
func (m *ExecutionWindowManager) SetCallback(fn func(window *ExecutionWindow)) {
	m.callback = fn
}

// Load loads windows from storage
func (m *ExecutionWindowManager) Load() error {
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

	var windows []*ExecutionWindow
	if err := json.Unmarshal(data, &windows); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, w := range windows {
		m.windows[w.ID] = w
	}

	log.Printf("[ExecutionWindow] Loaded %d windows", len(windows))
	return nil
}

// Save saves windows to storage
func (m *ExecutionWindowManager) Save() error {
	if m.storagePath == "" {
		return nil
	}

	m.mu.RLock()
	windows := make([]*ExecutionWindow, 0, len(m.windows))
	for _, w := range m.windows {
		windows = append(windows, w)
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(windows, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.storagePath, data, 0644)
}

// Create creates a new execution window
func (m *ExecutionWindowManager) Create(window *ExecutionWindow) error {
	// Validate cron expression
	if _, err := cron.ParseStandard(window.Schedule); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	if window.ID == "" {
		window.ID = fmt.Sprintf("window_%d", time.Now().UnixNano())
	}
	if window.CreatedAt.IsZero() {
		window.CreatedAt = time.Now()
	}
	if window.Enabled {
		window.Enabled = true
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.windows[window.ID] = window
	log.Printf("[ExecutionWindow] Created: %s (%s)", window.Name, window.Schedule)

	go m.Save()
	return nil
}

// Get returns a window by ID
func (m *ExecutionWindowManager) Get(id string) (*ExecutionWindow, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w, ok := m.windows[id]
	return w, ok
}

// List returns all windows
func (m *ExecutionWindowManager) List() []*ExecutionWindow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	windows := make([]*ExecutionWindow, 0, len(m.windows))
	for _, w := range m.windows {
		windows = append(windows, w)
	}
	return windows
}

// Update updates a window
func (m *ExecutionWindowManager) Update(id string, updates map[string]interface{}) (*ExecutionWindow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	window, ok := m.windows[id]
	if !ok {
		return nil, fmt.Errorf("window not found: %s", id)
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		window.Name = name
	}
	if schedule, ok := updates["schedule"].(string); ok {
		// Validate cron expression
		if _, err := cron.ParseStandard(schedule); err != nil {
			return nil, fmt.Errorf("invalid cron expression: %w", err)
		}
		window.Schedule = schedule
	}
	if strategy, ok := updates["strategy"].(string); ok {
		window.Strategy = strategy
	}
	if webhookURL, ok := updates["webhook_url"].(string); ok {
		window.WebhookURL = webhookURL
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		window.Enabled = enabled
	}

	log.Printf("[ExecutionWindow] Updated: %s", id)
	go m.Save()

	return window, nil
}

// Delete deletes a window
func (m *ExecutionWindowManager) Delete(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.windows[id]; !ok {
		return false
	}

	delete(m.windows, id)
	log.Printf("[ExecutionWindow] Deleted: %s", id)

	go m.Save()
	return true
}

// Trigger triggers a window callback
func (m *ExecutionWindowManager) Trigger(id string) {
	m.mu.RLock()
	window, ok := m.windows[id]
	m.mu.RUnlock()

	if !ok {
		return
	}

	window.LastRun = time.Now()

	if m.callback != nil {
		m.callback(window)
	}
}
