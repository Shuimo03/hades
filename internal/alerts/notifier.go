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

	"github.com/larksuite/oapi-sdk-go"
	"github.com/larksuite/oapi-sdk-go/core"
	"github.com/larksuite/oapi-sdk-go/service/im/v1"
)

// FeishuNotifier sends notifications to Feishu
type FeishuNotifier struct {
	appID     string
	appSecret string
	client    *lark.Client
	userID    string // 接收消息的用户 open_id
}

// NewFeishuNotifier creates a new Feishu notifier
func NewFeishuNotifier(appID, appSecret, userID string) *FeishuNotifier {
	config := lark.NewConfig(appID, appSecret)
	client := lark.NewClient(config)

	return &FeishuNotifier{
		appID:     appID,
		appSecret: appSecret,
		client:    client,
		userID:    userID,
	}
}

// Notify sends a notification to Feishu
func (n *FeishuNotifier) Notify(ctx context.Context, title, message string) error {
	if n.appID == "" || n.appSecret == "" {
		return fmt.Errorf("feishu app_id or app_secret not configured")
	}

	// 发送富文本卡片消息
	card := n.buildCardMessage(title, message)

	// 先尝试发送文本消息
	content := fmt.Sprintf("%s\n\n%s", title, message)
	resp, err := n.client.Im.MessageCreate(ctx, &lark.CreateMessageReq{
		ReceiveIdType: lark.String("open_id"),
		CreateMessageReqBody: &lark.CreateMessageReqBody{
			ReceiveId: lark.String(n.userID),
			MsgType:   lark.String("interactive"),
			Content:   lark.String(card),
		},
	})

	if err != nil {
		log.Printf("[Feishu] Failed to send message: %v", err)
		// 降级到 webhook
		return err
	}

	if resp.Code != 0 {
		log.Printf("[Feishu] API error: %d - %s", resp.Code, resp.Msg)
		return fmt.Errorf("feishu api error: %s", resp.Msg)
	}

	log.Printf("[Feishu] Message sent successfully")
	return nil
}

func (n *FeishuNotifier) buildCardMessage(title, message string) string {
	// 构建飞书卡片消息 JSON
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

	return card
}

func escapeForJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// WebhookNotifier sends notifications via webhook (Slack/DingTalk)
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

// Notifier is the unified notifier interface
type Notifier interface {
	Notify(ctx context.Context, title, message string) error
}

// NewNotifier creates a new notifier based on configuration
func NewNotifier(webhookURL string) Notifier {
	if webhookURL == "" {
		return &DummyNotifier{}
	}
	return NewWebhookNotifier(webhookURL)
}

// NewFeishuNotifierWithConfig creates a Feishu notifier from config
func NewFeishuNotifierWithConfig(appID, appSecret, userID string) Notifier {
	if appID == "" || appSecret == "" {
		return &DummyNotifier{}
	}
	return &FeishuNotifier{
		appID:     appID,
		appSecret: appSecret,
		client:    lark.NewClient(lark.NewConfig(appID, appSecret)),
		userID:    userID,
	}
}

// DummyNotifier is a no-op notifier
type DummyNotifier struct{}

func (n *DummyNotifier) Notify(ctx context.Context, title, message string) error {
	log.Printf("[Notifier] %s: %s", title, message)
	return nil
}
