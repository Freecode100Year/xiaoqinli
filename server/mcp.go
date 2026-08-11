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
	"time"

	"xiaoqinli/codegen"
	"xiaoqinli/compiler"
	"xiaoqinli/evolution"
	"xiaoqinli/remedy"
)

// MaxMCPMessageBytes is the maximum allowed size for a single MCP message (both stdio and HTTP).
const MaxMCPMessageBytes = 2 << 20 // 2 MB

// MCPServer implements the Model Context Protocol over stdio and streamable HTTP.
type MCPServer struct{}

// NewMCPServer creates a new MCPServer.
func NewMCPServer() *MCPServer {
	return &MCPServer{}
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
	scanner.Buffer(make([]byte, 0, MaxMCPMessageBytes), MaxMCPMessageBytes)
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

	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, MaxMCPMessageBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

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

	resp := s.safeHandleRequest(&req)
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
	default:
		log.Printf("MCP: unknown notification method: %s", req.Method)
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
				"version": compiler.Version,
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
						"description": "Target language: go | rust | ts | py | cpp | c | java | csharp | kotlin | swift | haskell | ocaml | elixir | dart | lua | ruby | php | zig | nim | julia | awk | bash | bat | crystal | d | fortran | pascal | perl | powershell | tcl | vala | groovy | shortcut | chrome | tccli | android | ios (default: go)",
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
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "stdlib_matrix_inspect",
			"description": "Retrieve the known deprecated-vs-recommended standard library API mapping for a target language, so codegen can avoid emitting deprecated stdlib calls.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": map[string]string{
						"type":        "string",
						"description": "Target language identifier (py, go, ts, rust, etc.)",
					},
				},
				"required": []string{"target"},
			},
		},
		{
			"name":        "stdlib_matrix_update",
			"description": "Register or update deprecated-vs-recommended standard library API mappings for a target language locally.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": map[string]string{
						"type":        "string",
						"description": "Target language identifier (e.g. py, go, ts)",
					},
					"deprecated_apis": map[string]interface{}{
						"type":                 "object",
						"description":          "Map of deprecated API name to reason/replacement, e.g. {\"os.getcwd\": \"use os.Getwd\"}",
						"additionalProperties": map[string]string{"type": "string"},
					},
					"recommended_apis": map[string]interface{}{
						"type":                 "object",
						"description":          "Map of feature to the currently recommended API, e.g. {\"http client\": \"net/http.Client\"}",
						"additionalProperties": map[string]string{"type": "string"},
					},
				},
				"required": []string{"target"},
			},
		},
		{
			"name":        "treesitter_mapping_inspect",
			"description": "Retrieve AST node and keyword mapping for a target language Tree-sitter WASM grammar.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": map[string]string{
						"type":        "string",
						"description": "Target language identifier (e.g. mojo, gleam)",
					},
				},
				"required": []string{"target"},
			},
		},
		{
			"name":        "treesitter_mapping_update",
			"description": "Register or update a Tree-sitter WASM grammar mapping for AST node synthesis locally.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"language": map[string]string{
						"type":        "string",
						"description": "Language name (e.g. Mojo, Gleam)",
					},
					"target": map[string]string{
						"type":        "string",
						"description": "Target language identifier (e.g. mojo, gleam)",
					},
					"node_mappings": map[string]interface{}{
						"type":                 "object",
						"description":          "Tree-sitter AST node to xql AST node mapping",
						"additionalProperties": map[string]string{"type": "string"},
					},
					"keyword_mapping": map[string]interface{}{
						"type":                 "object",
						"description":          "Grammar keyword mapping",
						"additionalProperties": map[string]string{"type": "string"},
					},
				},
				"required": []string{"language", "target"},
			},
		},
		{
			"name":        "specs_inspect",
			"description": "Retrieve modern language specification profile & latest version features for any target language (e.g. py, go, ts, rust) before code generation.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": map[string]string{
						"type":        "string",
						"description": "Target language identifier (py, go, ts, rust, etc.)",
					},
				},
				"required": []string{"target"},
			},
		},
		{
			"name":        "specs_update",
			"description": "Self-update or register a target language modern specification profile locally.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": map[string]string{
						"type":        "string",
						"description": "Target language identifier (e.g. py, go, ts)",
					},
					"latest_version": map[string]string{
						"type":        "string",
						"description": "Latest language version (e.g. 3.12+)",
					},
					"modern_features": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "List of latest language features & syntax rules",
					},
				},
				"required": []string{"target"},
			},
		},
		{
			"name":        "diagnostic_memory_inspect",
			"description": "Retrieve learned compiler diagnostic fix patterns and proven suggested fixes for an error code (e.g. XQL_E201, XQL_E301).",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code": map[string]string{
						"type":        "string",
						"description": "Compiler error code (e.g. XQL_E201)",
					},
				},
				"required": []string{"code"},
			},
		},
		{
			"name":        "diagnostic_memory_record",
			"description": "Record a proven diagnostic fix pattern into local memory for self-evolution error resolution.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code":          map[string]string{"type": "string"},
					"error_context": map[string]string{"type": "string"},
					"ast_pattern":   map[string]string{"type": "string"},
					"suggested_fix": map[string]string{"type": "string"},
				},
				"required": []string{"code", "suggested_fix"},
			},
		},
		{
			"name":        "security_policy_inspect",
			"description": "Inspect dynamic environment capability grant rules and sandbox bounds.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "skills_diagnose_and_fill",
			"description": "Self-diagnose detected capability gap during task execution and auto-generate dynamic skill module to fill shortboard.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_context": map[string]string{
						"type":        "string",
						"description": "Description of current task context or failing scenario",
					},
					"missing_capability": map[string]string{
						"type":        "string",
						"description": "Name or key of detected missing capability/shortboard",
					},
				},
				"required": []string{"missing_capability"},
			},
		},
		{
			"name":        "agent_search_query",
			"description": "Query AI Agent Search Engine for knowledge, skills, diagnostic memory, policies, and language specs.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]string{
						"type":        "string",
						"description": "Search keyword or query term",
					},
					"category": map[string]string{
						"type":        "string",
						"description": "Optional category filter: skill | diagnostic | policy | spec",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "agent_search_autoupdate",
			"description": "Trigger automatic re-indexing and update of AI Agent Search Engine.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "codegen_strategy_inspect",
			"description": "Inspect codegen performance strategy configuration for a target language backend.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": map[string]string{
						"type":        "string",
						"description": "Target language flag (e.g. py, go, ts, rust)",
					},
				},
				"required": []string{"target"},
			},
		},
		{
			"name":        "codegen_strategy_update",
			"description": "Update codegen performance strategy configuration and benchmark feedback for a target backend.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": map[string]string{
						"type":        "string",
						"description": "Target language flag (e.g. py)",
					},
					"prefer_comprehension": map[string]string{
						"type":        "boolean",
						"description": "Whether to prefer list/array comprehensions",
					},
					"inline_threshold": map[string]string{
						"type":        "number",
						"description": "Function inlining threshold",
					},
					"optimization_flags": map[string]interface{}{
						"type":        "object",
						"description": "Optimization key-value flags",
					},
					"benchmark_score": map[string]string{
						"type":        "number",
						"description": "Measured benchmark score",
					},
				},
				"required": []string{"target"},
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

	if params.Name == "compile" || params.Name == "validate" {
		if ok, err := remedy.ProbeValidateDeferredSchema(params.Arguments, []string{"source"}); !ok || err != nil {
			return jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: -32602, Message: "probe validation failed: " + err.Error()},
			}
		}
	}

	var args struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "invalid tool arguments"},
		}
	}
	if args.Target == "" {
		args.Target = "go"
	}

	switch params.Name {
	case "compile":
		return s.toolCompile(req.ID, args.Source, args.Target)
	case "validate":
		return s.toolValidate(req.ID, args.Source)
	case "targets":
		return s.toolTargets(req.ID)
	case "stdlib_matrix_inspect":
		return s.toolStdlibMatrixInspect(req.ID, args.Target)
	case "stdlib_matrix_update":
		return s.toolStdlibMatrixUpdate(req.ID, params.Arguments)
	case "treesitter_mapping_inspect":
		return s.toolTreeSitterMappingInspect(req.ID, args.Target)
	case "treesitter_mapping_update":
		return s.toolTreeSitterMappingUpdate(req.ID, params.Arguments)
	case "specs_inspect":
		return s.toolSpecsInspect(req.ID, args.Target)
	case "specs_update":
		return s.toolSpecsUpdate(req.ID, params.Arguments)
	case "diagnostic_memory_inspect":
		return s.toolDiagnosticMemoryInspect(req.ID, params.Arguments)
	case "diagnostic_memory_record":
		return s.toolDiagnosticMemoryRecord(req.ID, params.Arguments)
	case "security_policy_inspect":
		return s.toolSecurityPolicyInspect(req.ID)
	case "skills_diagnose_and_fill":
		return s.toolSkillsDiagnoseAndFill(req.ID, params.Arguments)
	case "agent_search_query":
		return s.toolAgentSearchQuery(req.ID, params.Arguments)
	case "agent_search_autoupdate":
		return s.toolAgentSearchAutoUpdate(req.ID)
	case "codegen_strategy_inspect":
		return s.toolCodegenStrategyInspect(req.ID, params.Arguments)
	case "codegen_strategy_update":
		return s.toolCodegenStrategyUpdate(req.ID, params.Arguments)
	default:
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "unknown tool: " + params.Name},
		}
	}
}

func (s *MCPServer) toolCompile(id interface{}, source, target string) jsonRPCResponse {
	start := time.Now()
	success := true
	defer func() {
		GlobalMetrics.RecordToolsCall("compile", time.Since(start).Seconds(), success)
	}()

	pRes := compiler.ParseAST(compiler.ParseRequest{Data: []byte(source)})
	if !pRes.Success {
		success = false
		return toolErrorResult(id, pRes.Error, pRes.Diagnostics)
	}

	res := compiler.Compile(compiler.CompileRequest{
		AST:    pRes.AST,
		Target: target,
	})
	if !res.Success {
		success = false
		return toolErrorResult(id, res.Error, res.Diagnostics)
	}

	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": string(res.Code)},
			},
		},
	}
}

func (s *MCPServer) toolValidate(id interface{}, source string) jsonRPCResponse {
	start := time.Now()
	success := true
	defer func() {
		GlobalMetrics.RecordToolsCall("validate", time.Since(start).Seconds(), success)
	}()

	pRes := compiler.ParseAST(compiler.ParseRequest{Data: []byte(source)})
	if !pRes.Success {
		success = false
		return toolErrorResult(id, pRes.Error, pRes.Diagnostics)
	}

	res := compiler.Validate(compiler.ValidateRequest{
		AST: pRes.AST,
	})
	if !res.Success {
		success = false
		return toolErrorResult(id, res.Error, res.Diagnostics)
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

func (s *MCPServer) toolTargets(id interface{}) jsonRPCResponse {
	targetInfos := compiler.GetSupportedTargetInfos()
	lines := make([]string, len(targetInfos))
	for i, t := range targetInfos {
		lines[i] = t.Flag + " (" + t.Name + ", " + t.Ext + ")"
	}
	text := "Supported target languages:\n"
	for _, l := range lines {
		text += "  " + l + "\n"
	}
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": text},
			},
		},
	}
}

func (s *MCPServer) toolStdlibMatrixInspect(id interface{}, target string) jsonRPCResponse {
	m, err := compiler.InspectStdlibMatrix(target)
	if err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &rpcError{Code: -32602, Message: err.Error()},
		}
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": string(b)},
			},
		},
	}
}

func (s *MCPServer) toolStdlibMatrixUpdate(id interface{}, rawArgs json.RawMessage) jsonRPCResponse {
	var input struct {
		Target          string            `json:"target"`
		DeprecatedAPIs  map[string]string `json:"deprecated_apis"`
		RecommendedAPIs map[string]string `json:"recommended_apis"`
	}
	if err := json.Unmarshal(rawArgs, &input); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &rpcError{Code: -32602, Message: "invalid stdlib_matrix_update parameters"},
		}
	}

	m := evolution.StdlibAPIMatrix{
		Target:          input.Target,
		DeprecatedAPIs:  input.DeprecatedAPIs,
		RecommendedAPIs: input.RecommendedAPIs,
	}
	updated := compiler.UpdateStdlibMatrix(m)

	b, _ := json.MarshalIndent(updated, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": fmt.Sprintf("Successfully updated stdlib matrix:\n%s", string(b))},
			},
		},
	}
}

func (s *MCPServer) toolTreeSitterMappingInspect(id interface{}, target string) jsonRPCResponse {
	m, err := compiler.InspectTreeSitterMapping(target)
	if err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &rpcError{Code: -32602, Message: err.Error()},
		}
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": string(b)},
			},
		},
	}
}

func (s *MCPServer) toolTreeSitterMappingUpdate(id interface{}, rawArgs json.RawMessage) jsonRPCResponse {
	var input struct {
		Language       string            `json:"language"`
		Target         string            `json:"target"`
		NodeMappings   map[string]string `json:"node_mappings"`
		KeywordMapping map[string]string `json:"keyword_mapping"`
	}
	if err := json.Unmarshal(rawArgs, &input); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &rpcError{Code: -32602, Message: "invalid treesitter_mapping_update parameters"},
		}
	}

	m := evolution.TreeSitterMapping{
		Language:       input.Language,
		Target:         input.Target,
		NodeMappings:   input.NodeMappings,
		KeywordMapping: input.KeywordMapping,
	}
	updated := compiler.UpdateTreeSitterMapping(m)

	b, _ := json.MarshalIndent(updated, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": fmt.Sprintf("Successfully updated tree-sitter mapping:\n%s", string(b))},
			},
		},
	}
}

func (s *MCPServer) toolSpecsInspect(id interface{}, target string) jsonRPCResponse {
	spec, err := compiler.InspectSpec(target)
	if err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &rpcError{Code: -32602, Message: err.Error()},
		}
	}
	b, _ := json.MarshalIndent(spec, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": string(b)},
			},
		},
	}
}

func (s *MCPServer) toolSpecsUpdate(id interface{}, rawArgs json.RawMessage) jsonRPCResponse {
	var input struct {
		Target         string            `json:"target"`
		LatestVersion  string            `json:"latest_version"`
		ModernFeatures []string          `json:"modern_features"`
		CodegenOptions map[string]string `json:"codegen_options"`
	}
	if err := json.Unmarshal(rawArgs, &input); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &rpcError{Code: -32602, Message: "invalid specs_update parameters"},
		}
	}

	prof := codegen.LanguageProfile{
		Target:         input.Target,
		LatestVersion:  input.LatestVersion,
		ModernFeatures: input.ModernFeatures,
		CodegenOptions: input.CodegenOptions,
	}

	updated, err := compiler.UpdateSpec(prof)
	if err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &rpcError{Code: -32602, Message: err.Error()},
		}
	}

	b, _ := json.MarshalIndent(updated, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": fmt.Sprintf("Successfully updated spec profile:\n%s", string(b))},
			},
		},
	}
}

func (s *MCPServer) toolDiagnosticMemoryInspect(id interface{}, rawArgs json.RawMessage) jsonRPCResponse {
	var input struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rawArgs, &input)

	fixes := compiler.InspectDiagnosticFixes(input.Code)
	b, _ := json.MarshalIndent(fixes, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": string(b)},
			},
		},
	}
}

func (s *MCPServer) toolDiagnosticMemoryRecord(id interface{}, rawArgs json.RawMessage) jsonRPCResponse {
	var input struct {
		Code         string `json:"code"`
		ErrorContext string `json:"error_context"`
		ASTPattern   string `json:"ast_pattern"`
		SuggestedFix string `json:"suggested_fix"`
	}
	if err := json.Unmarshal(rawArgs, &input); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &rpcError{Code: -32602, Message: "invalid diagnostic_memory_record args"},
		}
	}

	rec := compiler.RecordDiagnosticFix(input.Code, input.ErrorContext, input.ASTPattern, input.SuggestedFix)
	b, _ := json.MarshalIndent(rec, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": fmt.Sprintf("Recorded diagnostic fix:\n%s", string(b))},
			},
		},
	}
}

func (s *MCPServer) toolSecurityPolicyInspect(id interface{}) jsonRPCResponse {
	pol := compiler.InspectSecurityPolicy()
	b, _ := json.MarshalIndent(pol, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": string(b)},
			},
		},
	}
}

func (s *MCPServer) toolSkillsDiagnoseAndFill(id interface{}, rawArgs json.RawMessage) jsonRPCResponse {
	var input struct {
		TaskContext       string `json:"task_context"`
		MissingCapability string `json:"missing_capability"`
	}
	if err := json.Unmarshal(rawArgs, &input); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &rpcError{Code: -32602, Message: "invalid skills_diagnose_and_fill parameters"},
		}
	}

	sk := evolution.DiagnoseAndFillSkillGap(input.TaskContext, input.MissingCapability)
	b, _ := json.MarshalIndent(sk, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": fmt.Sprintf("Successfully diagnosed and registered self-evolved skill module:\n%s", string(b))},
			},
		},
	}
}

func (s *MCPServer) toolAgentSearchQuery(id interface{}, rawArgs json.RawMessage) jsonRPCResponse {
	var input struct {
		Query    string `json:"query"`
		Category string `json:"category"`
	}
	if err := json.Unmarshal(rawArgs, &input); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &rpcError{Code: -32602, Message: "invalid agent_search_query parameters"},
		}
	}

	se := evolution.GetSearchEngine()
	results := se.Query(input.Query, input.Category)
	b, _ := json.MarshalIndent(results, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": string(b)},
			},
		},
	}
}

func (s *MCPServer) toolAgentSearchAutoUpdate(id interface{}) jsonRPCResponse {
	se := evolution.GetSearchEngine()
	count := se.AutoUpdateIndex()
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": fmt.Sprintf("AI Agent Search Engine auto-updated successfully. Total indexed entries: %d", count)},
			},
		},
	}
}

func (s *MCPServer) toolCodegenStrategyInspect(id interface{}, rawArgs json.RawMessage) jsonRPCResponse {
	var input struct {
		Target string `json:"target"`
	}
	_ = json.Unmarshal(rawArgs, &input)
	if input.Target == "" {
		input.Target = "py"
	}

	strat := compiler.InspectCodegenStrategy(input.Target)
	b, _ := json.MarshalIndent(strat, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": string(b)},
			},
		},
	}
}

func (s *MCPServer) toolCodegenStrategyUpdate(id interface{}, rawArgs json.RawMessage) jsonRPCResponse {
	var input struct {
		Target              string            `json:"target"`
		PreferComprehension bool              `json:"prefer_comprehension"`
		InlineThreshold     int               `json:"inline_threshold"`
		OptimizationFlags   map[string]string `json:"optimization_flags"`
		BenchmarkScore      float64           `json:"benchmark_score"`
	}
	if err := json.Unmarshal(rawArgs, &input); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &rpcError{Code: -32602, Message: "invalid codegen_strategy_update parameters"},
		}
	}
	if input.Target == "" {
		input.Target = "py"
	}

	strat := compiler.UpdateCodegenStrategy(compiler.CodegenStrategyConfig{
		Target:              input.Target,
		PreferComprehension: input.PreferComprehension,
		InlineThreshold:     input.InlineThreshold,
		OptimizationFlags:   input.OptimizationFlags,
		BenchmarkScore:      input.BenchmarkScore,
	})

	b, _ := json.MarshalIndent(strat, "", "  ")
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": fmt.Sprintf("Successfully updated codegen strategy profile:\n%s", string(b))},
			},
		},
	}
}

func toolErrorResult(id interface{}, errMsg string, diagnostics []compiler.Diagnostic) jsonRPCResponse {
	if len(diagnostics) > 0 {
		if b, err := json.MarshalIndent(diagnostics, "", "  "); err == nil {
			errMsg = string(b)
		}
	}
	res := map[string]interface{}{
		"isError": true,
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": errMsg,
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
