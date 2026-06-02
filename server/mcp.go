// Package server implements MCP, REST, and Skills serving for the xiaoqinli transpiler.
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"xiaoqinli/ast"
	"xiaoqinli/check"
	"xiaoqinli/codegen"
	"xiaoqinli/vfs"
)

// Session holds per-connection state for MCP sessions.
type Session struct {
	VFS *vfs.Workspace
}

// MCPServer implements the Model Context Protocol over stdio and streamable HTTP.
type MCPServer struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// NewMCPServer creates a new MCPServer.
func NewMCPServer() *MCPServer {
	return &MCPServer{
		sessions: make(map[string]*Session),
	}
}

func (s *MCPServer) getSession(id string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		sess = &Session{VFS: vfs.New()}
		s.sessions[id] = sess
	}
	return sess
}

// ---------------------------------------------------------------------------
// JSON-RPC types (subset for MCP)
// ---------------------------------------------------------------------------

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// stdio mode
// ---------------------------------------------------------------------------

// ServeStdio runs the MCP server in stdio mode, reading JSON-RPC from stdin.
func (s *MCPServer) ServeStdio() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 4*1024*1024), 64*1024*1024)
	enc := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			}
			enc.Encode(resp)
			continue
		}

		if req.ID == nil {
			s.handleNotification(&req)
			continue
		}

		resp := s.safeHandleRequest(&req)
		enc.Encode(resp)
	}
	return scanner.Err()
}

func (s *MCPServer) safeHandleRequest(req *jsonRPCRequest) (resp jsonRPCResponse) {
	defer func() {
		if r := recover(); r != nil {
			resp = jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: -32000, Message: fmt.Sprintf("internal error: %v", r)},
			}
		}
	}()
	return s.handleRequest(req)
}

// ---------------------------------------------------------------------------
// streamable HTTP mode
// ---------------------------------------------------------------------------

// ServeHTTP runs the MCP server as a streamable HTTP endpoint.
func (s *MCPServer) ServeHTTP(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleHTTPMCP)
	fmt.Fprintf(os.Stderr, "MCP HTTP listening on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *MCPServer) handleHTTPMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "parse error"},
		})
		return
	}

	if req.ID == nil {
		s.handleNotification(&req)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	resp := s.handleRequest(&req)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------------------------
// request dispatch
// ---------------------------------------------------------------------------

func (s *MCPServer) handleNotification(req *jsonRPCRequest) {
	// Notifications (no ID) require no response. Log for debugging only.
	switch req.Method {
	case "notifications/initialized", "notifications/cancelled":
		// expected lifecycle notifications — no action needed
	}
}

func (s *MCPServer) handleRequest(req *jsonRPCRequest) jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "ping":
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "prompts/list":
		return s.handlePromptsList(req)
	case "prompts/get":
		return s.handlePromptsGet(req)
	default:
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: "method not found: " + req.Method},
		}
	}
}

func (s *MCPServer) handleInitialize(req *jsonRPCRequest) jsonRPCResponse {
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"serverInfo": map[string]string{
				"name":    "xiaoqinli",
				"version": "2.1.0",
			},
			"capabilities": map[string]interface{}{
				"tools":   map[string]interface{}{},
				"prompts": map[string]interface{}{},
			},
		},
	}
}

func (s *MCPServer) handleToolsList(req *jsonRPCRequest) jsonRPCResponse {
	tools := []map[string]interface{}{
		{
			"name":        "compile",
			"description": "Compile .xql.json AST to target language source code",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]string{
						"type":        "string",
						"description": "The .xql.json AST as a JSON string",
					},
					"target": map[string]string{
						"type":        "string",
						"description": "Target language: go | rust | ts | kotlin | swift | py | java | csharp (default: go)",
					},
				},
				"required": []string{"source"},
			},
		},
		{
			"name":        "validate",
			"description": "Validate .xql.json AST without generating code",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]string{
						"type":        "string",
						"description": "The .xql.json AST as a JSON string",
					},
				},
				"required": []string{"source"},
			},
		},
	}
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"tools": tools},
	}
}

func (s *MCPServer) handleToolsCall(req *jsonRPCRequest) jsonRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "invalid params"},
		}
	}

	var args struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	json.Unmarshal(params.Arguments, &args)
	if args.Target == "" {
		args.Target = "go"
	}

	switch params.Name {
	case "compile":
		return s.toolCompile(req.ID, args.Source, args.Target)
	case "validate":
		return s.toolValidate(req.ID, args.Source)
	default:
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "unknown tool: " + params.Name},
		}
	}
}

func (s *MCPServer) toolCompile(id interface{}, source, target string) jsonRPCResponse {
	root, err := ast.Parse([]byte(source))
	if err != nil {
		return toolErrorResult(id, err.Error())
	}
	if err := check.RunAll(root); err != nil {
		return toolErrorResult(id, err.Error())
	}

	output, err := codegen.Generate(root, target)
	if err != nil {
		return toolErrorResult(id, err.Error())
	}

	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": string(output)},
			},
		},
	}
}

func (s *MCPServer) toolValidate(id interface{}, source string) jsonRPCResponse {
	root, err := ast.Parse([]byte(source))
	if err != nil {
		return toolErrorResult(id, err.Error())
	}
	if err := check.RunAll(root); err != nil {
		return toolErrorResult(id, err.Error())
	}
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "ok: all checks passed"},
			},
		},
	}
}

func toolErrorResult(id interface{}, msg string) jsonRPCResponse {
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"isError": true,
			"content": []map[string]string{
				{"type": "text", "text": msg},
			},
		},
	}
}
