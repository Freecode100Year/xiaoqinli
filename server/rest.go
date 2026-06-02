package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"xiaoqinli/ast"
	"xiaoqinli/check"
	"xiaoqinli/codegen"
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
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": "2.1.0"})
	})

	fmt.Fprintf(os.Stderr, "REST API listening on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

type compileRequest struct {
	Source string `json:"source"` // raw .xql.json string
	Target string `json:"target"` // go | rust | ts | kotlin | swift | py
}

type compileResponse struct {
	Ok     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (s *RESTServer) handleCompile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, compileResponse{Error: "bad request"})
		return
	}
	defer r.Body.Close()

	var req compileRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, compileResponse{Error: "invalid JSON request"})
		return
	}
	if req.Target == "" {
		req.Target = "go"
	}

	root, err := ast.Parse([]byte(req.Source))
	if err != nil {
		writeJSON(w, http.StatusOK, compileResponse{Error: err.Error()})
		return
	}

	if err := check.RunAll(root); err != nil {
		writeJSON(w, http.StatusOK, compileResponse{Error: err.Error()})
		return
	}

	output, err := codegen.Generate(root, req.Target)
	if err != nil {
		writeJSON(w, http.StatusOK, compileResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, compileResponse{Ok: true, Output: string(output)})
}

func (s *RESTServer) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, compileResponse{Error: "bad request"})
		return
	}
	defer r.Body.Close()

	var req compileRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, compileResponse{Error: "invalid JSON request"})
		return
	}

	root, err := ast.Parse([]byte(req.Source))
	if err != nil {
		writeJSON(w, http.StatusOK, compileResponse{Error: err.Error()})
		return
	}

	if err := check.RunAll(root); err != nil {
		writeJSON(w, http.StatusOK, compileResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, compileResponse{Ok: true, Output: "all checks passed"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
