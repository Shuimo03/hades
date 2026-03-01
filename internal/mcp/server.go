package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HTTPServer struct {
	server   *mcp.Server
	router   *http.ServeMux
	handlers map[string]interface{}
}

func NewHTTPServer(name, version string) *HTTPServer {
	server := mcp.NewServer(&mcp.Implementation{Name: name, Version: version}, nil)
	return &HTTPServer{
		server:   server,
		router:   http.NewServeMux(),
		handlers: make(map[string]interface{}),
	}
}

func (s *HTTPServer) AddTool(tool *mcp.Tool, handler interface{}) {
	s.handlers[tool.Name] = handler
	mcp.AddTool(s.server, tool, handler)
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only accept POST for JSON-RPC
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonrpc.WriteError(w, jsonrpc.ErrorCodeParseError, "Failed to read request body")
		return
	}

	// Handle MCP protocol messages
	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" || contentType == "" {
		s.handleJSONRPC(w, r, body)
	} else {
		jsonrpc.WriteError(w, jsonrpc.ErrorCodeInvalidRequest, "Unsupported content type")
	}
}

func (s *HTTPServer) handleJSONRPC(w http.ResponseWriter, r *http.Request, body []byte) {
	var req jsonrpc.Request
	if err := json.Unmarshal(body, &req); err != nil {
		jsonrpc.WriteError(w, jsonrpc.ErrorCodeParseError, err.Error())
		return
	}

	// Handle initialize request (MCP handshake)
	if req.Method == "initialize" {
		s.handleInitialize(w, req)
		return
	}

	// Handle tools/list
	if req.Method == "tools/list" {
		s.handleToolsList(w, req)
		return
	}

	// Handle tools/call
	if req.Method == "tools/call" {
		s.handleToolsCall(w, r, req)
		return
	}

	// Handle ping
	if req.Method == "ping" {
		s.handlePing(w, req)
		return
	}

	// Unknown method
	jsonrpc.WriteError(w, jsonrpc.ErrorCodeMethodNotFound, fmt.Sprintf("Method not found: %s", req.Method))
}

func (s *HTTPServer) handleInitialize(w http.ResponseWriter, req jsonrpc.Request) {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    s.server.Implementation.Name,
			"version": s.server.Implementation.Version,
		},
	}

	resp := jsonrpc.Response{
		ID:     req.ID,
		Result: result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handleToolsList(w http.ResponseWriter, req jsonrpc.Request) {
	// Get registered tools from server
	tools := s.server.Tools()

	result := map[string]interface{}{
		"tools": tools,
	}

	resp := jsonrpc.Response{
		ID:     req.ID,
		Result: result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handleToolsCall(w http.ResponseWriter, r *http.Request, req jsonrpc.Request) {
	// Parse request params
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if req.Params != nil {
		if data, ok := req.Params.(map[string]interface{}); ok {
			params.Name, _ = data["name"].(string)
			params.Arguments, _ = data["arguments"].(map[string]interface{})
		}
	}

	if params.Name == "" {
		jsonrpc.WriteError(w, jsonrpc.ErrorCodeInvalidParams, "Missing tool name")
		return
	}

	// Find handler
	handler, ok := s.handlers[params.Name]
	if !ok {
		jsonrpc.WriteError(w, jsonrpc.ErrorCodeMethodNotFound, fmt.Sprintf("Tool not found: %s", params.Name))
		return
	}

	// Execute tool via MCP server
	ctx := context.Background()
	callReq := &mcp.CallToolRequest{
		Params: &mcp.CallToolParams{
			Name:      params.Name,
			Arguments: params.Arguments,
		},
	}

	result, err := s.server.CallTool(ctx, callReq, handler)
	if err != nil {
		jsonrpc.WriteError(w, jsonrpc.ErrorCodeInternalError, err.Error())
		return
	}

	resp := jsonrpc.Response{
		ID:     req.ID,
		Result: result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handlePing(w http.ResponseWriter, req jsonrpc.Request) {
	resp := jsonrpc.Response{
		ID:     req.ID,
		Result: nil,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) Run(ctx context.Context, host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	log.Printf("MCP server starting on %s", addr)
	return http.ListenAndServe(addr, s.router)
}
