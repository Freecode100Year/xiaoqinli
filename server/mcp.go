// Package server implements MCP, REST, and Skills serving for the xiaoqinli transpiler.
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	log.Println("[MCP] Starting stdio server")
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
			log.Printf("[MCP] Parse error: %v", err)
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			}
			enc.Encode(resp)
			continue
		}

		log.Printf("[MCP] Request: %s (ID: %v)", req.Method, req.ID)

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
			log.Printf("[MCP] Panic recovered: %v", r)
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
	log.Printf("[MCP] HTTP listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *MCPServer) handleHTTPMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("[MCP] HTTP method not allowed: %s", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[MCP] HTTP read error: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("[MCP] HTTP parse error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "parse error"},
		})
		return
	}

	log.Printf("[MCP] HTTP Request: %s (ID: %v)", req.Method, req.ID)

	if req.ID == nil {
		s.handleNotification(&req)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	resp := s.safeHandleRequest(&req)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------------------------
// request dispatch
// ---------------------------------------------------------------------------

func (s *MCPServer) handleNotification(req *jsonRPCRequest) {
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
		log.Printf("[MCP] Unknown method: %s", req.Method)
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
				"version": "3.13.1",
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
			"description": "Compile .xql.json AST to target language source code. Runs type/effect/capability checks before codegen.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]string{
						"type":        "string",
						"description": "The .xql.json AST as a JSON string",
					},
					"target": map[string]string{
						"type":        "string",
						"description": "Target language: go | rust | ts | py | cpp | c | java | csharp | kotlin | swift | scala | haskell | ocaml | fsharp | elixir | clojure | dart | lua | ruby | php | zig | nim | julia | mql4 | mql5 | ada | awk | bash | bat | crystal | d | fortran | objc | pascal | perl | powershell | tcl | v | vala | groovy | shortcut | chrome (default: go)",
					},
				},
				"required": []string{"source"},
			},
		},
		{
			"name":        "validate",
			"description": "Validate .xql.json AST (type check + effect check + capability check) without generating code",
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
		{
			"name":        "targets",
			"description": "List all supported target languages with their flags and file extensions",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{},
			},
		},
	}
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{\"tools\": tools},
	}
}

func (s *MCPServer) handleToolsCall(req *jsonRPCRequest) jsonRPCResponse {
	var params struct {
		Name      string          `json:\"name\"`
		Arguments json.RawMessage `json:\"arguments\"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonRPCResponse{
		JSONRPC: \"2.0\",
		ID:      req.ID,
		Error:   &rpcError{Code: -32602, Message: \"invalid params\"},
	}
	}

	var args struct {
		Source string `json:\"source\"`
		Target string `json:\"target\"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return jsonRPCResponse{
		JSONRPC: \"2.0\",
		ID:      req.ID,
		Error:   &rpcError{Code: -32 laze(\" l")
		}
	}

	return s.toolCompile(req.ID, args.Source, args.Target)
}

func (s *MCPServer) toolCompile(id interface{}, source, target string) jsonRPCResponse {
	root, err := ast.Parse([]byte(source))
	if err != nil {
		return toolErrorResult(id, err)
	}
	if err := check.RunAll(root); err != nil {
		return toolErrorResult(id, err)
	}

	output, err := codegen.Generate(root, target)
	if err != nil {
		return toolErrorResult(id, err)
	}

	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", \"text\": string(output)},
			},
		},
	}
}

func (s *MCPServer) toolValidate(id interface{}, source string) jsonRPCResponse {
	root, err := ast.Parse([]byte(source))
	if err != nil {
		return toolErrorResult(id, err)
	}
	if err := check.RunAll(root); err != nil {
		return toolErrorResult(id, err)
	}
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{s
		\"content\": []map[string]string{
				{\"type\": \"text\", \"text\": \"ok: all checks passed\"},
			},
		},
	}
}

func (s *MCPServer) toolTargets(id interface{}, target string) jsonRPCResponse {
	return jsonRPCResponse{
		JSONRPC: \"2.0\",
		ID:      id,
		Result: map[string]interface{}{
			\"content\": []map[string]string{
				{\"type\": \"text\", \"text\": \"Supported targets: Go, Rust, TS, Py, etc.\},
		},
	}
}

func toolErrorResult(id interface{}, err error) jsonRPCResponse {
	var diagnostics []check.Diagnostic
	if we, ok := err.(check.WorkspaceError); ok {
		diagnostics = we.Diagnostics
	}

	res := map[string]interface{}{
		"isError": true,
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": err.Error(),
			},
		},
	}

	if len(diagnostics) > 0 {
		res["diagnostics"] = diagnostics
	}

	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  res,
	}
}
}
