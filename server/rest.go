package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"xiaoqinli/codegen"
	"xiaoqinli/compiler"
	"xiaoqinli/evolution"
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
	mux.HandleFunc("/specs", s.handleSpecs)
	mux.HandleFunc("/evolution/diagnostics", s.handleEvolutionDiagnostics)
	mux.HandleFunc("/api/v1/search", s.handleSearch)
	mux.HandleFunc("/api/v1/search/autoupdate", s.handleSearchAutoUpdate)
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

func (s *RESTServer) handleSpecs(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		target := r.URL.Query().Get("target")
		if target != "" {
			spec, err := compiler.InspectSpec(target)
			if err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, spec)
			return
		}
		writeJSON(w, http.StatusOK, compiler.GetAllSpecs())
		return
	}

	if r.Method == http.MethodPost {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
			return
		}
		var prof struct {
			Target         string            `json:"target"`
			Language       string            `json:"language"`
			LatestVersion  string            `json:"latest_version"`
			ModernFeatures []string          `json:"modern_features"`
			CodegenOptions map[string]string `json:"codegen_options"`
		}
		if err := json.Unmarshal(body, &prof); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
			return
		}

		updated, err := compiler.UpdateSpec(codegen.LanguageProfile{
			Target:         prof.Target,
			Language:       prof.Language,
			LatestVersion:  prof.LatestVersion,
			ModernFeatures: prof.ModernFeatures,
			CodegenOptions: prof.CodegenOptions,
		})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *RESTServer) handleEvolutionDiagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		code := r.URL.Query().Get("code")
		fixes := compiler.InspectDiagnosticFixes(code)
		writeJSON(w, http.StatusOK, fixes)
		return
	}

	if r.Method == http.MethodPost {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
			return
		}
		var input struct {
			Code         string `json:"code"`
			ErrorContext string `json:"error_context"`
			ASTPattern   string `json:"ast_pattern"`
			SuggestedFix string `json:"suggested_fix"`
		}
		if err := json.Unmarshal(body, &input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		rec := compiler.RecordDiagnosticFix(input.Code, input.ErrorContext, input.ASTPattern, input.SuggestedFix)
		writeJSON(w, http.StatusOK, rec)
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *RESTServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	kw := r.URL.Query().Get("query")
	cat := r.URL.Query().Get("category")

	se := evolution.GetSearchEngine()
	results := se.Query(kw, cat)
	writeJSON(w, http.StatusOK, results)
}

func (s *RESTServer) handleSearchAutoUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	se := evolution.GetSearchEngine()
	count := se.AutoUpdateIndex()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":            true,
		"indexed_count": count,
		"message":       "AI Agent Search Engine auto-updated successfully",
	})
}
