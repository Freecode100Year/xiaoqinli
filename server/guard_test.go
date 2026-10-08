package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListenAddrDefaultsToLoopback(t *testing.T) {
	cases := map[string]string{":8080": "127.0.0.1:8080", "0.0.0.0:9": "0.0.0.0:9", "[::1]:1": "[::1]:1"}
	for in, want := range cases {
		if got := ListenAddr(in); got != want {
			t.Errorf("ListenAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGuardRejectsForeignOrigin(t *testing.T) {
	g := &guard{}
	h := g.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for origin, want := range map[string]int{
		"":                       200,
		"http://localhost:3000":  200,
		"http://127.0.0.1":       200,
		"https://evil.example":   403,
		"http://localhost.evil.": 403,
	} {
		req := httptest.NewRequest("POST", "/mcp", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Origin %q: status %d, want %d", origin, rec.Code, want)
		}
	}
}

func TestGuardRequiresToken(t *testing.T) {
	g := &guard{token: "s3cret"}
	h := g.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for auth, want := range map[string]int{"": 401, "Bearer wrong": 401, "Bearer s3cret": 200} {
		req := httptest.NewRequest("POST", "/compile", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Authorization %q: status %d, want %d", auth, rec.Code, want)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	if rec.Code != 200 {
		t.Errorf("/health without a token: status %d, want 200", rec.Code)
	}
}

func TestReadOnlyServerRefusesStateChanges(t *testing.T) {
	t.Setenv(TokenEnv, "")
	s := &MCPServer{guard: newGuard("0.0.0.0:8080")}
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"codegen_strategy_update","arguments":{"target":"py","optimization_flags":{"strategy_tag":"x\nimport os"}}}}`
	rec := httptest.NewRecorder()
	s.handleHTTPMCP(rec, httptest.NewRequest("POST", "/mcp", strings.NewReader(body)))
	if !strings.Contains(rec.Body.String(), "disabled") {
		t.Fatalf("a read-only server ran a mutating tool: %s", rec.Body.String())
	}
}
