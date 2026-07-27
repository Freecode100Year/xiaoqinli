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
				"type":       "object",
				"properties": map[string]interface{}{},
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
	targets := []map[string]string{
		{"flag": "go", "ext": ".go", "name": "Go"},
		{"flag": "rust", "ext": ".rs", "name": "Rust"},
		{"flag": "ts", "ext": ".ts", "name": "TypeScript"},
		{"flag": "py", "ext": ".py", "name": "Python"},
		{"flag": "cpp", "ext": ".cpp", "name": "C++"},
		{"flag": "c", "ext": ".c", "name": "C"},
		{"flag": "java", "ext": ".java", "name": "Java"},
		{"flag": "csharp", "ext": ".cs", "name": "C#"},
		{"flag": "kotlin", "ext": ".kt", "name": "Kotlin"},
		{"flag": "swift", "ext": ".swift", "name": "Swift"},
		{"flag": "scala", "ext": ".scala", "name": "Scala"},
		{"flag": "haskell", "ext": ".hs", "name": "Haskell"},
		{"flag": "dart", "ext": ".dart", "name": "Dart"},
		{"flag": "lua", "ext": ".lua", "name": "Lua"},
		{"flag": "ruby", "ext": ".rb", "name": "Ruby"},
		{"flag": "php", "ext": ".php", "name": "PHP"},
		{"flag": "zig", "ext": ".zig", "name": "Zig"},
		{"flag": "nim", "ext": ".nim", "name": "Nim"},
		{"flag": "julia", "ext": ".jl", "name": "Julia"},
		{"flag": "mql4", "ext": ".mq4", "name": "MQL4"},
		{"flag": "mql5", "ext": ".mq5", "name": "MQL5"},
		{"flag": "ada", "ext": ".adb", "name": "Ada"},
		{"flag": "awk", "ext": ".awk", "name": "AWK"},
		{"flag": "bash", "ext": ".sh", "name": "Bash"},
		{"flag": "crystal", "ext": ".cr", "name": "Crystal"},
		{"flag": "d", "ext": ".d", "name": "D"},
		{"flag": "fortran", "ext": ".f90", "name": "Fortran"},
		{"flag": "objc", "ext": ".m", "name": "Objective-C"},
		{"flag": "pascal", "ext": ".pas", "name": "Pascal"},
		{"flag": "perl", "ext": ".pl", "name": "Perl"},
		{"flag": "powershell", "ext": ".ps1", "name": "PowerShell"},
		{"flag": "tcl", "ext": ".tcl", "name": "Tcl"},
		{"flag": "v", "ext": ".v", "name": "V"},
		{"flag": "ocaml", "ext": ".ml", "name": "OCaml"},
		{"flag": "fsharp", "ext": ".fs", "name": "F#"},
		{"flag": "elixir", "ext": ".ex", "name": "Elixir"},
		{"flag": "clojure", "ext": ".clj", "name": "Clojure"},
		{"flag": "vala", "ext": ".vala", "name": "Vala"},
		{"flag": "groovy", "ext": ".groovy", "name": "Groovy"},
		{"flag": "bat", "ext": ".bat", "name": "Batch"},
		{"flag": "shortcut", "ext": ".shortcut", "name": "Apple Shortcuts"},
		{"flag": "chrome", "ext": ".crx.json", "name": "Chrome Extension"},
	}
	lines := make([]string, len(targets))
	for i, t := range targets {
		lines[i] = t["flag"] + " (" + t["name"] + ", " + t["ext"] + ")"
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

func toolErrorResult(id interface{}, errMsg string, diagnostics []compiler.Diagnostic) jsonRPCResponse {
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
