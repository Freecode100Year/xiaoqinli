package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"xiaoqinli/compiler"
)

// RESTServer serves a lightweight REST API for compile and validate.
type RESTServer struct{}

// NewRESTServer creates a new RESTServer.
func NewRESTServer() *RESTServer { return &RESTServer{} }

// Serve starts the REST API on the given address.
func (s *RESTServer) Serve(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/compile", s.handleCompile)
	mux.HandleFunc("/validate", s.handleValidate)
	mux.HandleFunc("/skills/", handleSkillsREST)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": compiler.Version})
	})

	// Prometheus metrics endpoint
	mux.Handle("/metrics", GlobalMetrics.PrometheusHandler())

	fmt.Fprintf(os.Stderr, "REST API listening on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

type compileRequest struct {
	Source string `json:"source"` // raw .xql.json string
	Target string `json:"target"` // go | rust | ts | kotlin | swift | py | java | csharp | dart | lua | ruby | php | zig | nim | julia
}

type compileResponse struct {
	Ok     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (s *RESTServer) handleCompile(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	success := true
	target := ""
	defer func() {
		GlobalMetrics.RecordCompile(target, time.Since(start).Seconds(), success)
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, MaxMCPMessageBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		success = false
		writeJSON(w, http.StatusBadRequest, compileResponse{Error: "bad request"})
		return
	}

	var req compileRequest
	if err := json.Unmarshal(body, &req); err != nil {
		success = false
		writeJSON(w, http.StatusBadRequest, compileResponse{Error: "invalid JSON request"})
		return
	}
	target = req.Target
	if target == "" {
		target = "go"
	}

	pRes := compiler.ParseAST(compiler.ParseRequest{Data: []byte(req.Source)})
	if !pRes.Success {
		success = false
		writeJSON(w, http.StatusOK, compileResponse{Error: pRes.Error})
		return
	}

	res := compiler.Compile(compiler.CompileRequest{
		AST:    pRes.AST,
		Target: target,
	})

	if !res.Success {
		success = false
		writeJSON(w, http.StatusOK, compileResponse{Error: res.Error})
		return
	}

	writeJSON(w, http.StatusOK, compileResponse{Ok: true, Output: string(res.Code)})
}

func (s *RESTServer) handleValidate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	success := true
	target := ""
	defer func() {
		GlobalMetrics.RecordCompile(target, time.Since(start).Seconds(), success)
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, MaxMCPMessageBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		success = false
		writeJSON(w, http.StatusBadRequest, compileResponse{Error: "bad request"})
		return
	}

	var req compileRequest
	if err := json.Unmarshal(body, &req); err != nil {
		success = false
		writeJSON(w, http.StatusBadRequest, compileResponse{Error: "invalid JSON request"})
		return
	}

	pRes := compiler.ParseAST(compiler.ParseRequest{Data: []byte(req.Source)})
	if !pRes.Success {
		success = false
		writeJSON(w, http.StatusOK, compileResponse{Error: pRes.Error})
		return
	}

	res := compiler.Validate(compiler.ValidateRequest{
		AST: pRes.AST,
	})

	if !res.Success {
		success = false
		writeJSON(w, http.StatusOK, compileResponse{Error: res.Error})
		return
	}

	writeJSON(w, http.StatusOK, compileResponse{Ok: true, Output: "all checks passed"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
