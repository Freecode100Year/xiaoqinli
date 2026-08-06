package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRESTServer_HealthAndMetrics(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("/metrics", GlobalMetrics.PrometheusHandler())

	// Test /health
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	// Test /metrics
	reqMetrics := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recMetrics := httptest.NewRecorder()
	mux.ServeHTTP(recMetrics, reqMetrics)

	if recMetrics.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /metrics, got %d", recMetrics.Code)
	}
}

// TestRESTServer_RoutesRegistered asserts every documented endpoint is actually
// wired into the route table. It drives RESTServer.Routes directly rather than
// a locally rebuilt mux, so a handler that exists but was never registered
// fails here instead of silently becoming dead code.
func TestRESTServer_RoutesRegistered(t *testing.T) {
	mux := NewRESTServer().Routes()

	routes := []string{
		"/compile",
		"/validate",
		"/specs",
		"/codegen/strategy",
		"/evolution/diagnostics",
		"/api/v1/search",
		"/api/v1/search/autoupdate",
		"/skills/",
		"/health",
		"/metrics",
	}

	for _, route := range routes {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		h, pattern := mux.Handler(req)
		if h == nil || pattern == "" {
			t.Errorf("route %q is not registered in Routes()", route)
			continue
		}
		// An unregistered path falls through to the mux's 404 handler, which
		// reports an empty pattern; a registered one echoes its own pattern.
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Errorf("route %q resolved to a 404 handler (pattern %q)", route, pattern)
		}
	}
}

func TestRESTServer_CompileAndValidate(t *testing.T) {
	s := NewRESTServer()
	mux := http.NewServeMux()
	mux.HandleFunc("/compile", s.handleCompile)
	mux.HandleFunc("/validate", s.handleValidate)

	// Valid AST JSON
	validSource := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Int"},
				"body": [
					{"kind": "ReturnStmt", "value": {"kind": "Literal", "valueType": "Int", "value": 0}}
				]
			}
		]
	}`

	// Test /compile POST success
	payload, _ := json.Marshal(compileRequest{Source: validSource, Target: "go"})
	req := httptest.NewRequest(http.MethodPost, "/compile", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /compile, got %d", rec.Code)
	}
	var resp compileResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.Ok || resp.Output == "" {
		t.Fatalf("expected successful compile output, got error: %s", resp.Error)
	}

	// Test /validate POST success
	reqVal := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(payload))
	recVal := httptest.NewRecorder()
	mux.ServeHTTP(recVal, reqVal)

	if recVal.Code != http.StatusOK {
		t.Fatalf("expected 200 for /validate, got %d", recVal.Code)
	}
	var respVal compileResponse
	json.NewDecoder(recVal.Body).Decode(&respVal)
	if !respVal.Ok {
		t.Fatalf("expected successful validation, got error: %s", respVal.Error)
	}
}

func TestMCPServer_LifecycleAndTools(t *testing.T) {
	mcp := NewMCPServer()

	// Test initialize
	initReq := &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}
	initResp := mcp.handleRequest(initReq)
	if initResp.Error != nil {
		t.Fatalf("initialize failed: %v", initResp.Error)
	}

	// Test tools/list
	listReq := &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}
	listResp := mcp.handleRequest(listReq)
	if listResp.Error != nil {
		t.Fatalf("tools/list failed: %v", listResp.Error)
	}

	// Test targets tool call
	callArgs, _ := json.Marshal(map[string]interface{}{
		"name":      "targets",
		"arguments": map[string]string{},
	})
	callReq := &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  callArgs,
	}
	callResp := mcp.handleRequest(callReq)
	if callResp.Error != nil {
		t.Fatalf("tools/call targets failed: %v", callResp.Error)
	}
}

func TestSkillsResolution(t *testing.T) {
	skills := ListSkills()
	found := false
	for _, sk := range skills {
		if sk.Name == "xiaoqinli" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected skill 'xiaoqinli' to be listed, but got %v", skills)
	}

	content, ok := GetSkill("xiaoqinli")
	if !ok || content == "" {
		t.Fatalf("expected GetSkill('xiaoqinli') to succeed, got ok=%v", ok)
	}

	// Test MCP prompts/get for xiaoqinli
	mcp := NewMCPServer()
	getArgs, _ := json.Marshal(map[string]interface{}{
		"name": "xiaoqinli",
	})
	req := &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "prompts/get",
		Params:  getArgs,
	}
	resp := mcp.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("prompts/get xiaoqinli failed: %v", resp.Error)
	}
}
