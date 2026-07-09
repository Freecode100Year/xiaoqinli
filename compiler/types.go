package compiler

import "xiaoqinli/ast"

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
}

// CompileResult holds the outcome of a Compile call.
type CompileResult struct {
	Success     bool         `json:"success"`
	Code        []byte       `json:"code,omitempty"`
	Error       string       `json:"error,omitempty"`
	ErrorCode   string       `json:"error_code,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Warnings    []string     `json:"warnings,omitempty"`
}

// Diagnostic is a structured, AI-friendly error report.
type Diagnostic struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	Location     string `json:"location,omitempty"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
	Level        string `json:"level"` // "error" | "warning" | "info"
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
	AST           ast.Node
	WorkspacePath string
}

// ValidateResult holds the outcome of a Validate call.
type ValidateResult struct {
	Success     bool         `json:"success"`
	Error       string       `json:"error,omitempty"`
	ErrorCode   string       `json:"error_code,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Warnings    []string     `json:"warnings,omitempty"`
}
