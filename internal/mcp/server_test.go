package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHTTPServerSupportsStreamableHTTP(t *testing.T) {
	server := NewHTTPServer("test-server", "v1.0.0")
	server.AddTool("echo", "Echoes a name", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		name, _ := args["name"].(string)
		return map[string]interface{}{"result": "hello " + name}, nil
	})

	mux := http.NewServeMux()
	mux.Handle("/mcp", server)
	mux.Handle("/mcp/", server)

	httpServer := newTestHTTPServer(t, mux)
	defer httpServer.Close()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{
		Name:    "test-client",
		Version: "v1.0.0",
	}, nil)

	session, err := client.Connect(context.Background(), &sdkmcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/mcp",
	}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer session.Close()

	toolsResult, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(toolsResult.Tools) != 1 || toolsResult.Tools[0].Name != "echo" {
		t.Fatalf("ListTools() got %+v, want one echo tool", toolsResult.Tools)
	}

	callResult, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "echo",
		Arguments: map[string]any{
			"name": "codex",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if callResult.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", callResult)
	}
	if len(callResult.Content) != 1 {
		t.Fatalf("CallTool() content len = %d, want 1", len(callResult.Content))
	}
	text, ok := callResult.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("CallTool() content type = %T, want *mcp.TextContent", callResult.Content[0])
	}
	if text.Text != "hello codex" {
		t.Fatalf("CallTool() text = %q, want %q", text.Text, "hello codex")
	}
}

func TestHTTPServerSupportsPrompts(t *testing.T) {
	server := NewHTTPServer("test-server", "v1.0.0")
	server.AddPrompt(&sdkmcp.Prompt{
		Name:        "greet",
		Description: "Returns a greeting prompt",
		Arguments: []*sdkmcp.PromptArgument{
			{Name: "name", Required: true},
		},
	}, func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		return &sdkmcp.GetPromptResult{
			Description: "Greeting prompt",
			Messages: []*sdkmcp.PromptMessage{
				{
					Role:    "user",
					Content: &sdkmcp.TextContent{Text: "hello " + req.Params.Arguments["name"]},
				},
			},
		}, nil
	})

	mux := http.NewServeMux()
	mux.Handle("/mcp", server)
	mux.Handle("/mcp/", server)

	httpServer := newTestHTTPServer(t, mux)
	defer httpServer.Close()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{
		Name:    "test-client",
		Version: "v1.0.0",
	}, nil)

	session, err := client.Connect(context.Background(), &sdkmcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/mcp",
	}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer session.Close()

	prompts, err := session.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListPrompts() error = %v", err)
	}
	if len(prompts.Prompts) != 1 || prompts.Prompts[0].Name != "greet" {
		t.Fatalf("ListPrompts() got %+v, want one greet prompt", prompts.Prompts)
	}

	result, err := session.GetPrompt(context.Background(), &sdkmcp.GetPromptParams{
		Name: "greet",
		Arguments: map[string]string{
			"name": "codex",
		},
	})
	if err != nil {
		t.Fatalf("GetPrompt() error = %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("GetPrompt() messages len = %d, want 1", len(result.Messages))
	}
	text, ok := result.Messages[0].Content.(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("GetPrompt() content type = %T, want *mcp.TextContent", result.Messages[0].Content)
	}
	if text.Text != "hello codex" {
		t.Fatalf("GetPrompt() text = %q, want %q", text.Text, "hello codex")
	}
}

func newTestHTTPServer(t *testing.T, handler http.Handler) (server *httptest.Server) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			message := strings.TrimSpace(fmt.Sprint(r))
			if strings.Contains(message, "bind: operation not permitted") {
				t.Skipf("skipping HTTP MCP test in restricted sandbox: %s", message)
			}
			panic(r)
		}
	}()

	return httptest.NewServer(handler)
}
