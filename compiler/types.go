package compiler

import (
	"xiaoqinli/ast"
	"xiaoqinli/evolution"
)

// CodegenStrategyConfig alias for evolution.CodegenStrategyConfig
type CodegenStrategyConfig = evolution.CodegenStrategyConfig

// CompileRequest describes what to compile and how.
type CompileRequest struct {
	// AST is the parsed root node (required).
	AST ast.Node

	// Target language: "go", "rust", "ts", etc. Default: "go".
	Target string

	// OutputPath writes the result to disk when non-empty.
	OutputPath string

	// WorkspacePath is the project root for multi-file validation.
	WorkspacePath string

	// ValidateOnly skips codegen when true.
	ValidateOnly bool

	// StrictCapabilities enables strict capability check for unresolved function calls.
	StrictCapabilities bool
}

// CompileStats provides performance and size metrics for a compilation.
type CompileStats struct {
	DurationMs     int64 `json:"duration_ms"`
	TotalNodes     int   `json:"total_nodes"`
	FunctionCount  int   `json:"function_count"`
	ClassCount     int   `json:"class_count"`
	StructCount    int   `json:"struct_count"`
	GeneratedLines int   `json:"generated_lines"`
	GeneratedBytes int   `json:"generated_bytes"`
}

// CompileResult holds the outcome of a Compile call.
type CompileResult struct {
	Success     bool         `json:"success"`
	Code        []byte       `json:"code,omitempty"`
	Error       string       `json:"error,omitempty"`
	ErrorCode   string       `json:"error_code,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Warnings    []string     `json:"warnings,omitempty"`
	Stats       CompileStats `json:"stats,omitempty"`
}

// Diagnostic is a structured, AI-friendly error report.
type Diagnostic struct {
	Code         string           `json:"code"`
	Message      string           `json:"message"`
	Location     ast.LocationInfo `json:"location,omitempty"`
	Context      string           `json:"context,omitempty"` // snippet of code
	SuggestedFix string           `json:"suggested_fix,omitempty"`
	Level        string           `json:"level"` // "error" | "warning" | "info"
}

// ParseRequest describes an AST parse operation.
type ParseRequest struct {
	Data     []byte // Raw .xql.json bytes.
	FilePath string // File path for error messages.
}

// ParseResult holds the outcome of a ParseAST call.
type ParseResult struct {
	Success     bool         `json:"success"`
	AST         ast.Node     `json:"-"`
	Error       string       `json:"error,omitempty"`
	ErrorCode   string       `json:"error_code,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// ValidateRequest describes a validate-only operation.
type ValidateRequest struct {
	AST                ast.Node
	WorkspacePath      string
	StrictCapabilities bool
}

// ValidateResult holds the outcome of a Validate call.
type ValidateResult struct {
	Success     bool         `json:"success"`
	Error       string       `json:"error,omitempty"`
	ErrorCode   string       `json:"error_code,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Warnings    []string     `json:"warnings,omitempty"`
}
