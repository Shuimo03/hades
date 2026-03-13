package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// DebugMode controls debug logging.
var DebugMode bool

func SetDebug(enabled bool) {
	DebugMode = enabled
}

type ToolHandler func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)

type HTTPServer struct {
	server  *sdkmcp.Server
	handler http.Handler
}

func NewHTTPServer(name, version string) *HTTPServer {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    name,
		Version: version,
	}, nil)

	if DebugMode {
		server.AddReceivingMiddleware(debugMiddleware)
	}

	handler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return server
	}, nil)

	return &HTTPServer{
		server:  server,
		handler: handler,
	}
}

func (s *HTTPServer) AddTool(name, description string, schema map[string]interface{}, handler ToolHandler) {
	inputSchema := schema
	if len(inputSchema) == 0 {
		inputSchema = map[string]interface{}{"type": "object"}
	}

	s.server.AddTool(&sdkmcp.Tool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		args, errResult := decodeArguments(req)
		if errResult != nil {
			return errResult, nil
		}

		result, err := handler(ctx, args)
		if err != nil {
			return toolErrorResult(err), nil
		}

		return toolSuccessResult(result), nil
	})
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func decodeArguments(req *sdkmcp.CallToolRequest) (map[string]interface{}, *sdkmcp.CallToolResult) {
	args := map[string]interface{}{}
	if req == nil || req.Params == nil || len(req.Params.Arguments) == 0 {
		return args, nil
	}

	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, toolErrorResult(fmt.Errorf("invalid tool arguments: %w", err))
	}

	return args, nil
}

func toolSuccessResult(result map[string]interface{}) *sdkmcp.CallToolResult {
	content := []sdkmcp.Content{}
	if result != nil {
		content = append(content, &sdkmcp.TextContent{Text: formatToolResult(result)})
	}

	return &sdkmcp.CallToolResult{
		Content:           content,
		StructuredContent: result,
	}
}

func toolErrorResult(err error) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: err.Error()},
		},
		IsError: true,
	}
}

func debugMiddleware(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
	return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
		log.Printf("[MCP DEBUG] Request method=%s", method)
		result, err := next(ctx, method, req)
		if err != nil {
			log.Printf("[MCP DEBUG] Response method=%s error=%v", method, err)
			return nil, err
		}
		log.Printf("[MCP DEBUG] Response method=%s ok", method)
		return result, nil
	}
}

func formatToolResult(result map[string]interface{}) string {
	if text, ok := result["result"].(string); ok && len(result) == 1 {
		return text
	}

	if value, ok := result["result"]; ok && len(result) == 1 {
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err == nil {
			return string(encoded)
		}
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", result)
	}
	return string(encoded)
}
