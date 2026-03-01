package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ToolHandler func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)

type HTTPServer struct {
	name    string
	version string
	router  *http.ServeMux
	tools   map[string]ToolHandler
	schemas map[string]map[string]interface{}
}

func NewHTTPServer(name, version string) *HTTPServer {
	return &HTTPServer{
		name:    name,
		version: version,
		router:  http.NewServeMux(),
		tools:   make(map[string]ToolHandler),
		schemas: make(map[string]map[string]interface{}),
	}
}

func (s *HTTPServer) AddTool(name, description string, schema map[string]interface{}, handler ToolHandler) {
	s.tools[name] = handler
	s.schemas[name] = map[string]interface{}{
		"name":        name,
		"description": description,
		"inputSchema": schema,
	}
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, -32700, "Failed to read request body")
		return
	}

	s.handleJSONRPC(w, body)
}

func (s *HTTPServer) writeError(w http.ResponseWriter, code int64, message string) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handleJSONRPC(w http.ResponseWriter, body []byte) {
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		s.writeError(w, -32700, err.Error())
		return
	}

	if req.JSONRPC != "2.0" {
		s.writeError(w, -32600, "Invalid JSON-RPC version")
		return
	}

	switch req.Method {
	case "initialize":
		s.handleInitialize(w, req.ID)
	case "ping":
		s.handlePing(w, req.ID)
	case "tools/list":
		s.handleToolsList(w, req.ID)
	case "tools/call":
		s.handleToolsCall(w, req.ID, req.Params)
	default:
		s.writeError(w, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func (s *HTTPServer) handleInitialize(w http.ResponseWriter, id interface{}) {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    s.name,
			"version": s.version,
		},
	}

	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handlePing(w http.ResponseWriter, id interface{}) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  nil,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handleToolsList(w http.ResponseWriter, id interface{}) {
	tools := make([]map[string]interface{}, 0, len(s.tools))
	for _, schema := range s.schemas {
		tools = append(tools, schema)
	}

	result := map[string]interface{}{
		"tools": tools,
	}

	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handleToolsCall(w http.ResponseWriter, id interface{}, params json.RawMessage) {
	var paramsStruct struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments,omitempty"`
	}

	if err := json.Unmarshal(params, &paramsStruct); err != nil {
		s.writeError(w, -32600, err.Error())
		return
	}

	if paramsStruct.Name == "" {
		s.writeError(w, -32600, "Missing tool name")
		return
	}

	handler, ok := s.tools[paramsStruct.Name]
	if !ok {
		s.writeError(w, -32601, fmt.Sprintf("Tool not found: %s", paramsStruct.Name))
		return
	}

	// Execute tool
	ctx := context.Background()
	result, err := handler(ctx, paramsStruct.Arguments)
	if err != nil {
		s.writeError(w, -32603, err.Error())
		return
	}

	// Format result as MCP tool response
	content := make([]map[string]interface{}, 0)
	if result != nil {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": fmt.Sprintf("%v", result),
		})
	}

	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"content": content,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
