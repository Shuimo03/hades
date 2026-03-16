package prompt

import (
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestParsePromptDocument(t *testing.T) {
	doc, err := parsePromptDocument("test.md", []byte(`---
name: greet
title: Greet
description: greeting prompt
result_description: greeting result
system: |
  always be concise
arguments:
  - name: name
    default: codex
---
hello {{.Arg "name"}}`))
	if err != nil {
		t.Fatalf("parsePromptDocument() error = %v", err)
	}

	result, err := renderPrompt(doc, &sdkmcp.GetPromptRequest{
		Params: &sdkmcp.GetPromptParams{
			Arguments: map[string]string{
				"name": "hades",
			},
		},
	})
	if err != nil {
		t.Fatalf("renderPrompt() error = %v", err)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("renderPrompt() messages len = %d, want 2", len(result.Messages))
	}
	if result.Messages[0].Role != sdkmcp.Role("system") {
		t.Fatalf("renderPrompt() first role = %q, want system", result.Messages[0].Role)
	}
	systemText, ok := result.Messages[0].Content.(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("renderPrompt() first content type = %T", result.Messages[0].Content)
	}
	if systemText.Text != "always be concise" {
		t.Fatalf("renderPrompt() first text = %q, want %q", systemText.Text, "always be concise")
	}

	text, ok := result.Messages[1].Content.(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("renderPrompt() second content type = %T", result.Messages[1].Content)
	}
	if text.Text != "hello hades" {
		t.Fatalf("renderPrompt() text = %q, want %q", text.Text, "hello hades")
	}
}

func TestLoadPromptDocumentsFromMarkdown(t *testing.T) {
	docs, err := loadPromptDocuments(".")
	if err != nil {
		t.Fatalf("loadPromptDocuments() error = %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("loadPromptDocuments() len = %d, want 3", len(docs))
	}

	var target *promptDocument
	for i := range docs {
		if docs[i].meta.Name == "signal_alert_workflow" {
			target = &docs[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("loadPromptDocuments() missing signal_alert_workflow")
	}

	def := buildPromptDefinition(target.meta)
	if def.Name != "signal_alert_workflow" {
		t.Fatalf("buildPromptDefinition() name = %q", def.Name)
	}
	if len(def.Arguments) != 3 {
		t.Fatalf("buildPromptDefinition() args len = %d, want 3", len(def.Arguments))
	}

	result, err := renderPrompt(*target, &sdkmcp.GetPromptRequest{
		Params: &sdkmcp.GetPromptParams{
			Arguments: map[string]string{
				"symbol":        "TSLA.US",
				"goal":          "止盈",
				"session_scope": "extended",
			},
		},
	})
	if err != nil {
		t.Fatalf("renderPrompt() error = %v", err)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("renderPrompt() messages len = %d, want 2", len(result.Messages))
	}
	if result.Messages[0].Role != sdkmcp.Role("system") {
		t.Fatalf("renderPrompt() first role = %q, want system", result.Messages[0].Role)
	}

	text, ok := result.Messages[1].Content.(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("renderPrompt() second content type = %T", result.Messages[1].Content)
	}
	if !containsAll(text.Text, "TSLA.US", "止盈", "session_scope=extended") {
		t.Fatalf("renderPrompt() text = %q, want placeholders rendered", text.Text)
	}
}

func TestRenderPromptWithCustomMessages(t *testing.T) {
	doc, err := parsePromptDocument("messages.md", []byte(`---
name: custom_flow
title: Custom Flow
description: custom prompt
messages:
  - role: assistant
    content: |
      我会先确认上下文
  - role: user
    content: |
      请分析 {{.Arg "symbol"}}
arguments:
  - name: symbol
    default: AAPL.US
---
`))
	if err != nil {
		t.Fatalf("parsePromptDocument() error = %v", err)
	}

	result, err := renderPrompt(doc, &sdkmcp.GetPromptRequest{
		Params: &sdkmcp.GetPromptParams{
			Arguments: map[string]string{"symbol": "TSLA.US"},
		},
	})
	if err != nil {
		t.Fatalf("renderPrompt() error = %v", err)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("renderPrompt() messages len = %d, want 2", len(result.Messages))
	}
	if result.Messages[0].Role != sdkmcp.Role("assistant") || result.Messages[1].Role != sdkmcp.Role("user") {
		t.Fatalf("renderPrompt() roles = %q, %q", result.Messages[0].Role, result.Messages[1].Role)
	}
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
