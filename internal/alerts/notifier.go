package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// FeishuNotifier sends notifications via Feishu API (app_id/app_secret)
type FeishuNotifier struct {
	appID      string
	appSecret  string
	userID     string
	httpClient *http.Client
}

// NewFeishuNotifier creates a new Feishu notifier
func NewFeishuNotifier(appID, appSecret, userID string) *FeishuNotifier {
	return &FeishuNotifier{
		appID:     appID,
		appSecret: appSecret,
		userID:    userID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Notify sends a notification to Feishu user
func (n *FeishuNotifier) Notify(ctx context.Context, title, message string) error {
	if n.appID == "" || n.appSecret == "" || n.userID == "" {
		return fmt.Errorf("feishu config incomplete")
	}

	// Get access token
	token, err := n.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Send message
	return n.sendMessage(ctx, token, title, message)
}

func (n *FeishuNotifier) getAccessToken(ctx context.Context) (string, error) {
	url := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"

	body := map[string]string{
		"app_id":     n.appID,
		"app_secret": n.appSecret,
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("failed to get token: %s", result.Msg)
	}

	return result.TenantAccessToken, nil
}

func (n *FeishuNotifier) sendMessage(ctx context.Context, token, title, message string) error {
	url := "https://open.feishu.cn/open-apis/im/v1/messages"

	// Build rich text message
	content := map[string]interface{}{
		"text": fmt.Sprintf("%s\n\n%s", title, message),
	}
	contentJSON, _ := json.Marshal(content)

	body := map[string]interface{}{
		"receive_id":      n.userID,
		"receive_id_type": "open_id",
		"msg_type":        "text",
		"content":         string(contentJSON),
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("failed to send message: %s", result.Msg)
	}

	log.Printf("[Feishu] Message sent to user: %s", n.userID)
	return nil
}

// WebhookNotifier sends notifications via webhook (Feishu/Slack/DingTalk)
type WebhookNotifier struct {
	webhookURL string
	httpClient *http.Client
}

// NewWebhookNotifier creates a new webhook notifier
func NewWebhookNotifier(webhookURL string) *WebhookNotifier {
	return &WebhookNotifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Notify sends a notification via webhook
func (n *WebhookNotifier) Notify(ctx context.Context, title, message string) error {
	if n.webhookURL == "" {
		return fmt.Errorf("webhook url not configured")
	}

	// 自动检测 webhook 类型
	if strings.Contains(n.webhookURL, "feishu.cn") {
		return n.sendFeishuWebhook(title, message)
	} else if strings.Contains(n.webhookURL, "slack.com") {
		return n.sendSlack(title, message)
	} else if strings.Contains(n.webhookURL, "dingtalk.com") || strings.Contains(n.webhookURL, "oapi.dingtalk.com") {
		return n.sendDingTalk(title, message)
	}

	// 默认尝试 Slack 格式
	return n.sendSlack(title, message)
}

// sendFeishuWebhook sends notification to Feishu webhook
func (n *WebhookNotifier) sendFeishuWebhook(title, message string) error {
	card := fmt.Sprintf(`{
		"config": {
			"wide_screen_mode": true
		},
		"header": {
			"title": {
				"tag": "plain_text",
				"content": "%s"
			},
			"template": "blue"
		},
		"elements": [
			{
				"tag": "div",
				"text": {
					"tag": "lark_md",
					"content": "%s"
				}
			}
		]
	}`, title, escapeForJSON(message))

	payload := map[string]interface{}{
		"msg_type": "interactive",
		"card":     json.RawMessage(card),
	}

	return n.doRequest(payload)
}

// sendSlack sends notification to Slack
func (n *WebhookNotifier) sendSlack(title, message string) error {
	payload := map[string]interface{}{
		"text": fmt.Sprintf("*%s*\n%s", title, message),
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": title,
				},
			},
			{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": message,
				},
			},
		},
	}

	return n.doRequest(payload)
}

// sendDingTalk sends notification to DingTalk
func (n *WebhookNotifier) sendDingTalk(title, message string) error {
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": title,
			"text":  fmt.Sprintf("### %s\n%s", title, message),
		},
	}

	return n.doRequest(payload)
}

func (n *WebhookNotifier) doRequest(payload map[string]interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", n.webhookURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func escapeForJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// Notifier is the unified notifier interface
type Notifier interface {
	Notify(ctx context.Context, title, message string) error
}

// NewNotifier creates a new notifier based on Feishu config
func NewNotifier(webhookURL string) Notifier {
	if webhookURL == "" {
		return &DummyNotifier{}
	}
	return NewWebhookNotifier(webhookURL)
}

// NewFeishuNotifierWithConfig creates a Feishu notifier from config
func NewFeishuNotifierWithConfig(appID, appSecret, userID string) Notifier {
	if appID == "" || appSecret == "" || userID == "" {
		return &DummyNotifier{}
	}
	return NewFeishuNotifier(appID, appSecret, userID)
}

// DummyNotifier is a no-op notifier
type DummyNotifier struct{}

func (n *DummyNotifier) Notify(ctx context.Context, title, message string) error {
	log.Printf("[Notifier] %s: %s", title, message)
	return nil
}
