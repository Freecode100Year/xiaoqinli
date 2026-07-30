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
	"xiaoqinli/evolution"
)

const (
	// Version is the current version of the xiaoqinli compiler package.
	Version = "3.36.0"

	// MaxASTBytes is the maximum allowed size for an AST payload (mirrors ast.MaxASTBytes).
	MaxASTBytes = 2 << 20 // 2 MB
)

// TargetInfo defines metadata for a supported target language.
type TargetInfo struct {
	Flag string `json:"flag"`
	Ext  string `json:"ext"`
	Name string `json:"name"`
}

// allTargetInfos is the single source of truth for all supported target backends.
var allTargetInfos = []TargetInfo{
	{Flag: "go", Ext: ".go", Name: "Go"},
	{Flag: "rust", Ext: ".rs", Name: "Rust"},
	{Flag: "ts", Ext: ".ts", Name: "TypeScript"},
	{Flag: "py", Ext: ".py", Name: "Python"},
	{Flag: "cpp", Ext: ".cpp", Name: "C++"},
	{Flag: "c", Ext: ".c", Name: "C"},
	{Flag: "java", Ext: ".java", Name: "Java"},
	{Flag: "csharp", Ext: ".cs", Name: "C#"},
	{Flag: "kotlin", Ext: ".kt", Name: "Kotlin"},
	{Flag: "swift", Ext: ".swift", Name: "Swift"},
	{Flag: "scala", Ext: ".scala", Name: "Scala"},
	{Flag: "haskell", Ext: ".hs", Name: "Haskell"},
	{Flag: "dart", Ext: ".dart", Name: "Dart"},
	{Flag: "lua", Ext: ".lua", Name: "Lua"},
	{Flag: "ruby", Ext: ".rb", Name: "Ruby"},
	{Flag: "php", Ext: ".php", Name: "PHP"},
	{Flag: "zig", Ext: ".zig", Name: "Zig"},
	{Flag: "nim", Ext: ".nim", Name: "Nim"},
	{Flag: "julia", Ext: ".jl", Name: "Julia"},
	{Flag: "mql4", Ext: ".mq4", Name: "MQL4"},
	{Flag: "mql5", Ext: ".mq5", Name: "MQL5"},
	{Flag: "ada", Ext: ".adb", Name: "Ada"},
	{Flag: "awk", Ext: ".awk", Name: "AWK"},
	{Flag: "bash", Ext: ".sh", Name: "Bash"},
	{Flag: "crystal", Ext: ".cr", Name: "Crystal"},
	{Flag: "d", Ext: ".d", Name: "D"},
	{Flag: "fortran", Ext: ".f90", Name: "Fortran"},
	{Flag: "objc", Ext: ".m", Name: "Objective-C"},
	{Flag: "pascal", Ext: ".pas", Name: "Pascal"},
	{Flag: "perl", Ext: ".pl", Name: "Perl"},
	{Flag: "powershell", Ext: ".ps1", Name: "PowerShell"},
	{Flag: "tcl", Ext: ".tcl", Name: "Tcl"},
	{Flag: "v", Ext: ".v", Name: "V"},
	{Flag: "ocaml", Ext: ".ml", Name: "OCaml"},
	{Flag: "fsharp", Ext: ".fs", Name: "F#"},
	{Flag: "elixir", Ext: ".ex", Name: "Elixir"},
	{Flag: "clojure", Ext: ".clj", Name: "Clojure"},
	{Flag: "vala", Ext: ".vala", Name: "Vala"},
	{Flag: "groovy", Ext: ".groovy", Name: "Groovy"},
	{Flag: "bat", Ext: ".bat", Name: "Batch"},
	{Flag: "shortcut", Ext: ".shortcut", Name: "Apple Shortcuts"},
	{Flag: "chrome", Ext: ".crx.json", Name: "Chrome Extension"},
	{Flag: "tccli", Ext: ".sh", Name: "Tencent Cloud CLI"},
	{Flag: "android", Ext: ".kt", Name: "Android (Gradle APK Project)"},
	{Flag: "ios", Ext: ".swift", Name: "iOS (Swift Package Manager Project)"},
}

// GetVersion returns the compiler library version.
func GetVersion() string { return Version }

// GetSupportedTargets returns all supported target language flags.
func GetSupportedTargets() []string {
	out := make([]string, len(allTargetInfos))
	for i, t := range allTargetInfos {
		out[i] = t.Flag
	}
	return out
}

// GetSupportedTargetInfos returns full metadata for all target languages.
func GetSupportedTargetInfos() []TargetInfo {
	out := make([]TargetInfo, len(allTargetInfos))
	copy(out, allTargetInfos)
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
	if len(req.Data) > MaxASTBytes {
		return ParseResult{
			Error:       fmt.Sprintf("XQL_E413: AST payload too large %d > %d", len(req.Data), MaxASTBytes),
			ErrorCode:   "XQL_E413",
			Diagnostics: []Diagnostic{{Code: "XQL_E413", Message: "AST payload too large", Level: "error"}},
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

func determineStrictCapabilities(strictCaps, disableStrictCaps bool) bool {
	if disableStrictCaps {
		return false
	}
	return true
}

// Validate runs all semantic checks without generating code.
func Validate(req ValidateRequest) ValidateResult {
	if req.AST == nil {
		err := errors.New("XQL_E001: AST is nil")
		return ValidateResult{
			Error:       err.Error(),
			ErrorCode:   "XQL_E001",
			Diagnostics: wrapDiag(err),
		}
	}
	strictCaps := determineStrictCapabilities(req.StrictCapabilities, req.DisableStrictCapabilities)
	if err := check.RunAllWithOptions(req.AST, "", nil, check.CheckOptions{StrictCapabilities: strictCaps}); err != nil {
		diags := wrapDiag(err)
		return ValidateResult{
			Error:       formatDiagError(err, diags),
			ErrorCode:   extractCode(err.Error()),
			Diagnostics: diags,
		}
	}
	return ValidateResult{Success: true}
}

// Compile runs the full pipeline: validate → codegen → optional write.
func Compile(req CompileRequest) CompileResult {
	start := time.Now()
	if req.AST == nil {
		err := errors.New("XQL_E001: AST is nil")
		diags := wrapDiag(err)
		return CompileResult{
			Error:       formatDiagError(err, diags),
			ErrorCode:   "XQL_E001",
			Diagnostics: diags,
		}
	}
	if req.Target == "" {
		req.Target = "go"
	}

	// Phase 1: validate.
	strictCaps := determineStrictCapabilities(req.StrictCapabilities, req.DisableStrictCapabilities)
	if err := check.RunAllWithOptions(req.AST, "", nil, check.CheckOptions{StrictCapabilities: strictCaps}); err != nil {
		diags := wrapDiag(err)
		return CompileResult{
			Error:       formatDiagError(err, diags),
			ErrorCode:   extractCode(err.Error()),
			Diagnostics: diags,
		}
	}
	if req.ValidateOnly {
		return CompileResult{Success: true}
	}

	// Phase 2: codegen.
	proj, err := codegen.GenerateProject(req.AST, req.Target)
	if err != nil {
		msg := fmt.Sprintf("XQL_E401: codegen error: %v", err)
		return CompileResult{
			Error: msg, ErrorCode: "XQL_E401",
			Diagnostics: []Diagnostic{{
				Code: "XQL_E401", Message: msg, Level: "error",
			}},
		}
	}
	code := proj.MainCode

	// Phase 3: optional disk write.
	if req.OutputPath != "" {
		if len(proj.Files) > 0 {
			for relPath, content := range proj.Files {
				fullPath := filepath.Join(req.OutputPath, relPath)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					return CompileResult{Error: err.Error(), ErrorCode: "XQL_E402"}
				}
				if err := os.WriteFile(fullPath, content, 0644); err != nil {
					return CompileResult{Error: err.Error(), ErrorCode: "XQL_E402"}
				}
			}
		} else {
			if wErr := writeOutput(code, req.OutputPath, req.Target); wErr != nil {
				return CompileResult{
					Error: wErr.Error(), ErrorCode: "XQL_E402",
				}
			}
		}
	}

	// Compute Stats
	stats := computeStats(req.AST, code, time.Since(start))

	return CompileResult{
		Success: true,
		Code:    code,
		Files:   proj.Files,
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
				_ = f
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
		clean := filepath.Clean(name)
		if strings.Contains(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("XQL_E403: path escape in bundle: %s", name)
		}
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
		p := filepath.Join(dir, clean)
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
			fix := d.SuggestedFix
			if learned := InspectDiagnosticFixes(d.Code); len(learned) > 0 {
				fix = learned[len(learned)-1].SuggestedFix
			}
			out[i] = Diagnostic{
				Code:         d.Code,
				Message:      d.Message,
				Location:     d.Location,
				SuggestedFix: fix,
				Level:        "error",
			}
		}
		return out
	}
	code := extractCode(err.Error())
	fix := ""
	if learned := InspectDiagnosticFixes(code); len(learned) > 0 {
		fix = learned[len(learned)-1].SuggestedFix
	}
	return []Diagnostic{{
		Code:         code,
		Message:      err.Error(),
		SuggestedFix: fix,
		Level:        "error",
	}}
}

func formatDiagError(err error, diags []Diagnostic) string {
	if len(diags) > 0 {
		data, mErr := json.MarshalIndent(diags, "", "  ")
		if mErr == nil {
			return string(data)
		}
	}
	return err.Error()
}

// InspectSpec retrieves the latest language specification profile for target language.
func InspectSpec(target string) (*codegen.LanguageProfile, error) {
	return codegen.InspectLanguageProfile(target)
}

// UpdateSpec updates or registers a target language specification profile locally.
func UpdateSpec(profile codegen.LanguageProfile) (*codegen.LanguageProfile, error) {
	res, err := codegen.UpdateLanguageProfile(profile)
	if err == nil {
		_ = SaveLocalState(DefaultStateDir)
	}
	return res, err
}

// DefaultStateDir is the local, project-scoped directory where xql persists
// self-evolved language specs and evolution memory across process restarts.
const DefaultStateDir = ".xql"

// LoadLocalState loads previously self-evolved language specs and evolution memory
// from dirPath if available. A missing directory or file is not an error.
func LoadLocalState(dirPath string) error {
	if dirPath == "" {
		dirPath = DefaultStateDir
	}
	_ = codegen.LoadProfilesFromFile(filepath.Join(dirPath, "profiles.json"))
	_ = evolution.LoadEvolutionState(filepath.Join(dirPath, "evolution"))
	return nil
}

// SaveLocalState persists the current in-memory language specs and evolution memory
// to dirPath so agent-supplied updates survive process restarts.
func SaveLocalState(dirPath string) error {
	if dirPath == "" {
		dirPath = DefaultStateDir
	}
	_ = codegen.SaveProfilesToFile(filepath.Join(dirPath, "profiles.json"))
	_ = evolution.SaveEvolutionState(filepath.Join(dirPath, "evolution"))
	return nil
}

// GetAllSpecs returns all 42+ registered target language profiles.
func GetAllSpecs() map[string]*codegen.LanguageProfile {
	return codegen.ListAllLanguageProfiles()
}

// Diagnostic Memory Wrappers
func RecordDiagnosticFix(code, errCtx, astPattern, fix string) *evolution.DiagnosticFixRecord {
	return evolution.RecordDiagnosticFix(code, errCtx, astPattern, fix)
}

func InspectDiagnosticFixes(code string) []*evolution.DiagnosticFixRecord {
	return evolution.InspectDiagnosticFixes(code)
}

// TreeSitter Mapping Wrappers
func UpdateTreeSitterMapping(m evolution.TreeSitterMapping) *evolution.TreeSitterMapping {
	return evolution.UpdateTreeSitterMapping(m)
}

func InspectTreeSitterMapping(target string) (*evolution.TreeSitterMapping, error) {
	return evolution.InspectTreeSitterMapping(target)
}

// Security Policy Wrappers
func InspectSecurityPolicy() evolution.SecurityPolicyConfig {
	return evolution.InspectSecurityPolicy()
}

func UpdateSecurityPolicy(policy evolution.SecurityPolicyConfig) evolution.SecurityPolicyConfig {
	return evolution.UpdateSecurityPolicy(policy)
}

// Stdlib Matrix Wrappers
func InspectStdlibMatrix(target string) (*evolution.StdlibAPIMatrix, error) {
	return evolution.InspectStdlibMatrix(target)
}

func UpdateStdlibMatrix(m evolution.StdlibAPIMatrix) *evolution.StdlibAPIMatrix {
	return evolution.UpdateStdlibMatrix(m)
}

// Codegen Strategy Wrappers
func InspectCodegenStrategy(target string) *evolution.CodegenStrategyConfig {
	return evolution.InspectCodegenStrategy(target)
}

func UpdateCodegenStrategy(s evolution.CodegenStrategyConfig) *evolution.CodegenStrategyConfig {
	return evolution.UpdateCodegenStrategy(s)
}

// Universal Skill Evolution Wrappers
func DiagnoseAndFillSkillGap(taskContext, missingCapability string) *evolution.DynamicSkill {
	return evolution.DiagnoseAndFillSkillGap(taskContext, missingCapability)
}
