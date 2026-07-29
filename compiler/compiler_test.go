package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

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
