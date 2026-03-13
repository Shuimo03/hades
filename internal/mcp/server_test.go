package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
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

	httpServer := httptest.NewServer(mux)
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
