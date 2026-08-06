package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xiaoqinli/ast"
	"xiaoqinli/codegen"
)

// TestAdvertisedTargetsAreGeneratable keeps allTargetInfos honest: every target
// the CLI and the MCP "targets" tool advertise must reach a real codegen
// backend. Without this, a backend can exist while never being advertised (as
// happened with "js"), or a flag can be advertised with no backend behind it.
func TestAdvertisedTargetsAreGeneratable(t *testing.T) {
	prog := &ast.Program{Decls: []ast.Node{
		&ast.FunctionDecl{
			Name:       "main",
			ReturnType: ast.TypeExpr{KindName: "Void"},
			Body:       []ast.Node{},
		},
	}}

	for _, info := range GetSupportedTargetInfos() {
		if _, err := codegen.GenerateProject(prog, info.Flag); err != nil {
			t.Errorf("advertised target %q (%s) has no working backend: %v", info.Flag, info.Name, err)
		}
	}

	// Backends reachable through Generate must also be advertised, apart from
	// the documented aliases that intentionally share another flag's entry.
	aliases := map[string]bool{
		"javascript":   true, // alias of js
		"tencentcloud": true, // alias of tccli
		"apk":          true, // alias of android
		"swift-pkg":    true, // alias of ios
	}
	advertised := make(map[string]bool)
	for _, f := range GetSupportedTargets() {
		advertised[f] = true
	}
	for _, flag := range []string{
		"go", "rust", "ts", "js", "javascript", "kotlin", "swift", "py", "java", "csharp",
		"dart", "lua", "ruby", "php", "zig", "nim", "julia", "cpp", "mql4", "mql5", "c",
		"scala", "haskell", "ocaml", "fsharp", "ada", "awk", "bash", "crystal", "d",
		"fortran", "objc", "pascal", "perl", "powershell", "tcl", "v", "elixir", "clojure",
		"vala", "groovy", "bat", "shortcut", "chrome", "tccli", "tencentcloud", "android",
		"apk", "ios", "swift-pkg",
	} {
		if !advertised[flag] && !aliases[flag] {
			t.Errorf("target %q has a codegen backend but is not advertised in allTargetInfos", flag)
		}
	}
}

// helloJSON is the minimal valid XQL AST for testing.
const helloJSON = `{
  "kind": "Program",
  "declarations": [
    {
      "kind": "FunctionDecl",
      "name": "greet",
      "params": [{"name": "name", "type": {"kind": "String"}}],
      "returnType": {"kind": "String"},
      "effects": ["pure"],
      "grant": [],
      "body": [
        {
          "kind": "ReturnStmt",
          "value": {
            "kind": "BinaryExpr",
            "op": "+",
            "left": {"kind": "Literal", "valueType": "String", "value": "Hello, "},
            "right": {"kind": "Ident", "name": "name"}
          }
        }
      ]
    },
    {
      "kind": "FunctionDecl",
      "name": "main",
      "params": [],
      "returnType": {"kind": "Void"},
      "effects": [],
      "grant": [],
      "body": [
        {
          "kind": "ExprStmt",
          "expr": {
            "kind": "CallExpr",
            "callee": "println",
            "args": [
              {
                "kind": "CallExpr",
                "callee": "greet",
                "args": [{"kind": "Literal", "valueType": "String", "value": "World"}]
              }
            ]
          }
        }
      ]
    }
  ]
}`

// --- ParseAST ---

func TestParseAST_Success(t *testing.T) {
	res := ParseAST(ParseRequest{Data: []byte(helloJSON)})
	if !res.Success {
		t.Fatalf("expected success, got error: %s", res.Error)
	}
	if res.AST == nil {
		t.Fatal("expected non-nil AST")
	}
}

func TestParseAST_EmptyData(t *testing.T) {
	res := ParseAST(ParseRequest{Data: nil})
	if res.Success {
		t.Fatal("expected failure for empty data")
	}
	if res.ErrorCode != "XQL_E001" {
		t.Fatalf("expected XQL_E001, got %s", res.ErrorCode)
	}
}

func TestParseAST_InvalidJSON(t *testing.T) {
	res := ParseAST(ParseRequest{Data: []byte("{bad json}")})
	if res.Success {
		t.Fatal("expected failure for invalid JSON")
	}
	if len(res.Diagnostics) == 0 {
		t.Fatal("expected diagnostics")
	}
}

// --- Validate ---

func TestValidate_Success(t *testing.T) {
	pr := ParseAST(ParseRequest{Data: []byte(helloJSON)})
	if !pr.Success {
		t.Fatalf("parse failed: %s", pr.Error)
	}
	vr := Validate(ValidateRequest{AST: pr.AST})
	if !vr.Success {
		t.Fatalf("validate failed: %s", vr.Error)
	}
}

func TestValidate_NilAST(t *testing.T) {
	vr := Validate(ValidateRequest{AST: nil})
	if vr.Success {
		t.Fatal("expected failure for nil AST")
	}
}

// --- Compile ---

func TestCompile_Go(t *testing.T) {
	pr := ParseAST(ParseRequest{Data: []byte(helloJSON)})
	if !pr.Success {
		t.Fatalf("parse failed: %s", pr.Error)
	}
	cr := Compile(CompileRequest{AST: pr.AST, Target: "go"})
	if !cr.Success {
		t.Fatalf("compile failed: %s", cr.Error)
	}
	if len(cr.Code) == 0 {
		t.Fatal("expected non-empty code output")
	}
	code := string(cr.Code)
	if !contains(code, "func greet") {
		t.Fatalf("Go output missing 'func greet': %s", code[:min(200, len(code))])
	}
	if cr.Stats.DurationMs == 0 && cr.Stats.GeneratedBytes == 0 {
		t.Error("expected non-zero compilation stats")
	}
}

func TestCompile_ValidateOnly(t *testing.T) {
	pr := ParseAST(ParseRequest{Data: []byte(helloJSON)})
	if !pr.Success {
		t.Fatalf("parse failed: %s", pr.Error)
	}
	cr := Compile(CompileRequest{AST: pr.AST, ValidateOnly: true})
	if !cr.Success {
		t.Fatalf("validate-only compile failed: %s", cr.Error)
	}
	if len(cr.Code) != 0 {
		t.Fatal("expected nil code for validate-only")
	}
}

func TestCompile_MultipleTargets(t *testing.T) {
	targets := []string{"go", "rust", "ts", "py", "java"}
	pr := ParseAST(ParseRequest{Data: []byte(helloJSON)})
	if !pr.Success {
		t.Fatalf("parse failed: %s", pr.Error)
	}
	for _, tgt := range targets {
		t.Run(tgt, func(t *testing.T) {
			cr := Compile(CompileRequest{AST: pr.AST, Target: tgt})
			if !cr.Success {
				t.Fatalf("compile to %s failed: %s", tgt, cr.Error)
			}
			if len(cr.Code) == 0 {
				t.Fatalf("empty output for %s", tgt)
			}
		})
	}
}

func TestCompile_DefaultTarget(t *testing.T) {
	pr := ParseAST(ParseRequest{Data: []byte(helloJSON)})
	if !pr.Success {
		t.Fatalf("parse failed: %s", pr.Error)
	}
	cr := Compile(CompileRequest{AST: pr.AST})
	if !cr.Success {
		t.Fatalf("default target compile failed: %s", cr.Error)
	}
}

func TestCompile_NilAST(t *testing.T) {
	cr := Compile(CompileRequest{AST: nil, Target: "go"})
	if cr.Success {
		t.Fatal("expected failure for nil AST")
	}
}

func TestCompile_WriteOutput(t *testing.T) {
	pr := ParseAST(ParseRequest{Data: []byte(helloJSON)})
	if !pr.Success {
		t.Fatalf("parse failed: %s", pr.Error)
	}
	outPath := filepath.Join(t.TempDir(), "out.go")
	cr := Compile(CompileRequest{AST: pr.AST, Target: "go", OutputPath: outPath})
	if !cr.Success {
		t.Fatalf("compile failed: %s", cr.Error)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("output file is empty")
	}
}

func TestCompileFromFile_Hello(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.xql.json")
	if err := os.WriteFile(path, []byte(helloJSON), 0644); err != nil {
		t.Fatal(err)
	}
	cr := CompileFromFile(path, "go", "")
	if !cr.Success {
		t.Fatalf("compile from file failed: %s", cr.Error)
	}
	if len(cr.Code) == 0 {
		t.Fatal("expected non-empty code")
	}
}

func TestCompileFromFile_NotFound(t *testing.T) {
	cr := CompileFromFile("/nonexistent/file.xql.json", "go", "")
	if cr.Success {
		t.Fatal("expected failure for missing file")
	}
	if cr.ErrorCode != "XQL_E404" {
		t.Fatalf("expected XQL_E404, got %s", cr.ErrorCode)
	}
}

func TestGetVersion(t *testing.T) {
	v := GetVersion()
	if v != Version {
		t.Fatalf("expected %s, got %s", Version, v)
	}
}

func TestDynamicSkillGapFilling(t *testing.T) {
	sk := DiagnoseAndFillSkillGap("unit_test_context", "gpu_acceleration")
	if sk == nil || sk.Name != "gpu_acceleration" {
		t.Fatalf("expected skill gpu_acceleration, got %v", sk)
	}
}

func TestCompilerEvolutionBridge(t *testing.T) {
	// Diagnostic memory
	RecordDiagnosticFix("XQL_E301", "Ungranted capability", "FunctionDecl", "Add @grant([io]) to declaration")
	fixes := InspectDiagnosticFixes("XQL_E301")
	if len(fixes) == 0 || fixes[0].SuggestedFix == "" {
		t.Errorf("expected diagnostic fix record via compiler bridge")
	}

	// Security policy
	pol := InspectSecurityPolicy()
	if pol.MaxEffectLevel == "" {
		t.Errorf("expected security policy max effect level")
	}

	// Codegen strategy
	strat := InspectCodegenStrategy("py")
	if strat.Target != "py" {
		t.Errorf("expected strategy target py")
	}
}

func TestAutoAttachLearnedFixToDiagnostic(t *testing.T) {
	// Record a learned fix strategy for XQL_E001
	_ = RecordDiagnosticFix("XQL_E001", "AST is nil", "ASTPattern", "Provide valid AST root node")

	// Trigger validate failure with nil AST
	res := Validate(ValidateRequest{AST: nil})
	if res.Success {
		t.Fatal("expected validate failure for nil AST")
	}
	if len(res.Diagnostics) == 0 {
		t.Fatal("expected diagnostic report")
	}
	if res.Diagnostics[0].SuggestedFix != "Provide valid AST root node" {
		t.Errorf("expected learned SuggestedFix to be automatically attached, got: %s", res.Diagnostics[0].SuggestedFix)
	}
}

func TestLearnedDiagnosticFixOverridesPrePopulatedDefault(t *testing.T) {
	// Record a learned fix strategy for XQL_E201 (Type Check Failure)
	learnedFix := "Custom Learned Fix Strategy: cast String to Int explicitly"
	_ = RecordDiagnosticFix("XQL_E201", "return type mismatch", "ReturnStmt", learnedFix)

	// Create an AST with type mismatch where check pre-populates a default SuggestedFix
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "badReturn",
			"params": [],
			"returnType": {"kind": "Int"},
			"effects": ["pure"],
			"grant": [],
			"body": [{
				"kind": "ReturnStmt",
				"value": {"kind": "Literal", "valueType": "String", "value": "invalid_int"}
			}]
		}]
	}`
	pr := ParseAST(ParseRequest{Data: []byte(src)})
	if !pr.Success {
		t.Fatalf("ParseAST failed: %s", pr.Error)
	}

	vr := Validate(ValidateRequest{AST: pr.AST})
	if vr.Success {
		t.Fatal("expected validate failure for type mismatch")
	}
	if len(vr.Diagnostics) == 0 {
		t.Fatal("expected diagnostic report")
	}

	if vr.Diagnostics[0].SuggestedFix != learnedFix {
		t.Errorf("expected learned fix '%s' to override pre-populated default, got: '%s'", learnedFix, vr.Diagnostics[0].SuggestedFix)
	}

	if !contains(vr.Error, learnedFix) {
		t.Errorf("expected vr.Error string to contain learned fix '%s', got: '%s'", learnedFix, vr.Error)
	}
}

func TestGetSupportedTargets(t *testing.T) {
	targets := GetSupportedTargets()
	if len(targets) < 40 {
		t.Fatalf("expected 40+ targets, got %d", len(targets))
	}
	targets[0] = "MODIFIED"
	fresh := GetSupportedTargets()
	if fresh[0] == "MODIFIED" {
		t.Fatal("GetSupportedTargets returned a reference, not a copy")
	}
}

func TestStrictCapabilitiesDefaultTrue(t *testing.T) {
	// Program with unresolved call 'externalCall'
	unresolvedCapJSON := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [
					{
						"kind": "ExprStmt",
						"expr": {
							"kind": "CallExpr",
							"callee": "println",
							"args": [
								{
									"kind": "Literal",
									"valueType": "String",
									"value": "hello"
								}
							]
						}
					}
				]
			}
		]
	}`

	pr := ParseAST(ParseRequest{Data: []byte(unresolvedCapJSON)})
	if !pr.Success {
		t.Fatalf("ParseAST failed: %v", pr.Error)
	}

	// 1. Default CompileRequest (StrictCapabilities defaults to true) -> should succeed for builtin println
	crDefault := Compile(CompileRequest{AST: pr.AST, Target: "go"})
	if !crDefault.Success {
		t.Fatalf("expected Compile with builtin call to succeed, got: %s", crDefault.Error)
	}

	// Unresolved call JSON
	unresolvedCallJSON := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [
					{
						"kind": "ExprStmt",
						"expr": {
							"kind": "CallExpr",
							"callee": "unresolvedExternalCall",
							"args": []
						}
					}
				]
			}
		]
	}`

	prUnresolved := ParseAST(ParseRequest{Data: []byte(unresolvedCallJSON)})
	if !prUnresolved.Success {
		t.Fatalf("ParseAST failed: %v", prUnresolved.Error)
	}

	// 2. Default Compile (StrictCapabilities = true) on unresolved call -> triggers XQL_E303
	crStrict := Compile(CompileRequest{AST: prUnresolved.AST, Target: "go"})
	if crStrict.Success {
		t.Fatalf("expected Compile on unresolved call to fail by default under StrictCapabilities, but succeeded")
	}
	if !strings.Contains(crStrict.Error, "XQL_E303") {
		t.Errorf("expected XQL_E303 error, got: %s", crStrict.Error)
	}

	// 3. DisableStrictCapabilities = true -> bypasses XQL_E303 capability check
	crBypass := Compile(CompileRequest{AST: prUnresolved.AST, Target: "go", DisableStrictCapabilities: true})
	if crBypass.Success && strings.Contains(crBypass.Error, "XQL_E303") {
		t.Errorf("DisableStrictCapabilities=true should bypass XQL_E303, got: %s", crBypass.Error)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchStr(s, sub)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
