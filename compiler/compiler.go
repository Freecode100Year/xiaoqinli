package compiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xiaoqinli/ast"
	"xiaoqinli/check"
	"xiaoqinli/codegen"
)

// Version is the library version (bumped from 3.5.0 for the lib export).
const Version = "3.13.1"

// allTargets mirrors the list in main.go (single source of truth).
var allTargets = []string{
	"go", "rust", "ts", "kotlin", "swift", "py",
	"java", "csharp", "dart", "lua", "ruby", "php",
	"zig", "nim", "julia", "cpp", "c", "scala", "haskell",
	"mql4", "mql5",
	"ocaml", "fsharp", "elixir", "clojure",
	"ada", "awk", "bash", "crystal", "d", "fortran",
	"objc", "pascal", "perl", "powershell", "tcl", "v",
	"vala",
	"groovy",
	"bat",
	"shortcut",
	"chrome",
}

// GetVersion returns the compiler library version.
func GetVersion() string { return Version }

// GetSupportedTargets returns all 42+ target language names.
func GetSupportedTargets() []string {
	out := make([]string, len(allTargets))
	copy(out, allTargets)
	return out
}

// ParseAST parses raw .xql.json bytes into a typed AST tree.
func ParseAST(req ParseRequest) ParseResult {
	if len(req.Data) == 0 {
		return ParseResult{
			Error:     "input data is empty",
			ErrorCode: "XQL_E001",
			Diagnostics: []Diagnostic{{
				Code: "XQL_E001", Message: "input data is empty", Level: "error",
			}},
		}
	}
	root, err := ast.Parse(req.Data)
	if err != nil {
		return ParseResult{
			Error:       err.Error(),
			ErrorCode:   extractCode(err.Error()),
			Diagnostics: wrapDiag(err),
		}
	}
	return ParseResult{Success: true, AST: root}
}

// Validate runs all semantic checks without generating code.
func Validate(req ValidateRequest) ValidateResult {
	if req.AST == nil {
		return ValidateResult{
			Error: "AST is nil", ErrorCode: "XQL_E001",
			Diagnostics: []Diagnostic{{
				Code: "XQL_E001", Message: "AST is nil", Level: "error",
			}},
		}
	}
	if err := check.RunAll(req.AST); err != nil {
		return ValidateResult{
			Error:       err.Error(),
			ErrorCode:   extractCode(err.Error()),
			Diagnostics: wrapDiag(err),
		}
	}
	return ValidateResult{Success: true}
}

// Compile runs the full pipeline: validate → codegen → optional write.
func Compile(req CompileRequest) CompileResult {
	start := time.Now()
	if req.AST == nil {
		return CompileResult{
			Error: "AST is nil", ErrorCode: "XQL_E001",
			Diagnostics: []Diagnostic{{
				Code: "XQL_E001", Message: "AST is nil", Level: "error",
			}},
		}
	}
	if req.Target == "" {
		req.Target = "go"
	}

	// Phase 1: validate.
	if err := check.RunAll(req.AST); err != nil {
		return CompileResult{
			Error:       err.Error(),
			ErrorCode:   extractCode(err.Error()),
			Diagnostics: wrapDiag(err),
		}
	}
	if req.ValidateOnly {
		return CompileResult{Success: true}
	}

	// Phase 2: codegen.
	code, err := codegen.Generate(req.AST, req.Target)
	if err != nil {
		msg := fmt.Sprintf("XQL_E401: codegen error: %v", err)
		return CompileResult{
			Error: msg, ErrorCode: "XQL_E401",
			Diagnostics: []Diagnostic{{
				Code: "XQL_E401", Message: msg, Level: "error",
			}},
		}
	}

	// Phase 3: optional disk write.
	if req.OutputPath != "" {
		if wErr := writeOutput(code, req.OutputPath, req.Target); wErr != nil {
			return CompileResult{
				Error: wErr.Error(), ErrorCode: "XQL_E402",
			}
		}
	}

	// Compute Stats
	stats := computeStats(req.AST, code, time.Since(start))

	return CompileResult{
		Success: true,
		Code:    code,
		Stats:   stats,
	}
}

// CompileFromFile is a convenience wrapper: read file → parse → compile.
func CompileFromFile(path, target, outputPath string) CompileResult {
	data, err := os.ReadFile(path)
	if err != nil {
		msg := fmt.Sprintf("XQL_E404: %v", err)
		return CompileResult{Error: msg, ErrorCode: "XQL_E404"}
	}
	pr := ParseAST(ParseRequest{Data: data, FilePath: path})
	if !pr.Success {
		return CompileResult{
			Error: pr.Error, ErrorCode: pr.ErrorCode,
			Diagnostics: pr.Diagnostics,
		}
	}
	return Compile(CompileRequest{
		AST:           pr.AST,
		Target:        target,
		OutputPath:    outputPath,
		WorkspacePath: filepath.Dir(path),
	})
}

// ---------- internal helpers ----------

func computeStats(root ast.Node, code []byte, duration time.Duration) CompileStats {
	s := CompileStats{
		DurationMs:     duration.Milliseconds(),
		GeneratedBytes: len(code),
		GeneratedLines: strings.Count(string(code), "\n") + 1,
	}

	// Walk AST to count nodes
	var walker func(ast.Node)
	walker = func(n ast.Node) {
		if n == nil {
			return
		}
		s.TotalNodes++
		switch node := n.(type) {
		case *ast.Program:
			for _, d := range node.Decls {
				walker(d)
			}
		case *ast.FunctionDecl:
			s.FunctionCount++
			for _, stmt := range node.Body {
				walker(stmt)
			}
		case *ast.StructDecl:
			s.StructCount++
			for _, f := range node.Fields {
				// Field is not a Node, but we can count them if we want.
				// For now, just count the decl.
			}
		case *ast.ClassDecl:
			s.ClassCount++
		case *ast.ReturnStmt:
			walker(node.Value)
		case *ast.VarDecl:
			walker(node.Value)
		case *ast.AssignStmt:
			walker(node.Target)
			walker(node.Value)
		case *ast.IfStmt:
			walker(node.Cond)
			for _, n := range node.Then {
				walker(n)
			}
			for _, n := range node.Else {
				walker(n)
			}
		case *ast.WhileStmt:
			walker(node.Cond)
			for _, n := range node.Body {
				walker(n)
			}
		case *ast.ForStmt:
			walker(node.Start)
			walker(node.End)
			walker(node.Iterable)
			for _, n := range node.Body {
				walker(n)
			}
		case *ast.ExprStmt:
			walker(node.Expr)
		case *ast.BinaryExpr:
			walker(node.Left)
			walker(node.Right)
		case *ast.UnaryExpr:
			walker(node.Operand)
		case *ast.CallExpr:
			for _, arg := range node.Args {
				walker(arg)
			}
		case *ast.MemberExpr:
			walker(node.Object)
		case *ast.StructLit:
			for _, f := range node.Fields {
				walker(f.Value)
			}
		case *ast.ArrayLit:
			for _, e := range node.Elements {
				walker(e)
			}
		case *ast.IndexExpr:
			walker(node.Target)
			walker(node.Index)
		case *ast.IfExpr:
			walker(node.Cond)
			walker(node.Then)
			walker(node.Else)
		case *ast.NewExpr:
			for _, arg := range node.Args {
				walker(arg)
			}
		case *ast.AwaitExpr:
			walker(node.Expr)
		case *ast.Lambda:
			for _, n := range node.Body {
				walker(n)
			}
		case *ast.MatchExpr:
			walker(node.Value)
			for _, arm := range node.Arms {
				for _, n := range arm.Body {
					walker(n)
				}
			}
		case *ast.SwitchStmt:
			walker(node.Value)
			for _, c := range node.Cases {
				walker(c.Value)
				for _, n := range c.Body {
					walker(n)
				}
			}
		case *ast.MapLiteral:
			for _, e := range node.Entries {
				walker(e.Key)
				walker(e.Value)
			}
		case *ast.ArrayLiteral:
			for _, e := range node.Elements {
				walker(e)
			}
		}
	}
	walker(root)
	return s
}

func writeOutput(code []byte, outPath, target string) error {
	if target == "chrome" {
		return unpackChromeBundle(code, outPath)
	}
	return os.WriteFile(outPath, code, 0644)
}

func unpackChromeBundle(data []byte, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create directory %s: %v", dir, err)
	}
	var bundle map[string]interface{}
	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("invalid bundle: %v", err)
	}
	for name, content := range bundle {
		var fileData []byte
		switch v := content.(type) {
		case string:
			fileData = []byte(v)
		default:
			b, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal %s: %v", name, err)
			}
			fileData = append(b, '\n')
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, fileData, 0644); err != nil {
			return fmt.Errorf("write %s: %v", name, err)
		}
	}
	return nil
}

func extractCode(msg string) string {
	if strings.HasPrefix(msg, "XQL_E") {
		if idx := strings.IndexByte(msg, ':'); idx > 0 {
			return msg[:idx]
		}
		if len(msg) >= 8 {
			return msg[:8]
		}
	}
	return "XQL_E999"
}

func wrapDiag(err error) []Diagnostic {
	var we check.WorkspaceError
	if errors.As(err, &we) {
		out := make([]Diagnostic, len(we.Diagnostics))
		for i, d := range we.Diagnostics {
			out[i] = Diagnostic{
				Code:         d.Code,
				Message:      d.Message,
				Location:     d.Location,
				SuggestedFix: d.SuggestedFix,
				Level:        "error",
			}
		}
		return out
	}
	return []Diagnostic{{
		Code:    extractCode(err.Error()),
		Message: err.Error(),
		Level:   "error",
	}}
}
