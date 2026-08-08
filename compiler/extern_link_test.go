package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xiaoqinli/ast"
)

// platformModule declares the host surface once so several modules can share it.
const platformModule = `{
  "kind": "Program",
  "declarations": [
    {
      "kind": "ExternDecl",
      "name": "fetch",
      "params": [{"name": "url", "type": {"kind": "String"}}],
      "returnType": {"kind": "String"},
      "effects": ["network"],
      "grant": ["network"]
    }
  ]
}`

func externEntry(extraDecls string) string {
	return `{
  "kind": "Program",
  "declarations": [
    {"kind": "ImportDecl", "path": "./platform.xql.json", "as": "platform"}` + extraDecls + `,
    {
      "kind": "FunctionDecl",
      "name": "main",
      "params": [],
      "returnType": {"kind": "Void"},
      "effects": ["network"],
      "grant": ["network"],
      "body": [{
        "kind": "VarDecl",
        "name": "body",
        "type": {"kind": "String"},
        "value": {
          "kind": "CallExpr",
          "callee": "fetch",
          "args": [{"kind": "Literal", "valueType": "String", "value": "https://example.com"}]
        }
      }]
    }
  ]
}`
}

func writeExternFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return filepath.Join(dir, "main.xql.json")
}

// TestExternIsInheritedAcrossImports: unlike a function, an extern is not
// namespaced by its module — it names one host function, so declaring it in a
// shared module and importing it must make it callable by that same name.
func TestExternIsInheritedAcrossImports(t *testing.T) {
	entry := writeExternFixture(t, map[string]string{
		"platform.xql.json": platformModule,
		"main.xql.json":     externEntry(""),
	})
	result := CompileFromFile(entry, "go", "")
	if !result.Success {
		t.Fatalf("expected an imported extern to be callable: %s", result.Error)
	}
	if !strings.Contains(string(result.Code), "fetch(") {
		t.Errorf("expected the host call in the output, got:\n%s", result.Code)
	}
}

// TestDuplicateExternIsMergedOnce: the host provides one fetch no matter how
// many modules mention it.
func TestDuplicateExternIsMergedOnce(t *testing.T) {
	dupDecl := `,
    {
      "kind": "ExternDecl",
      "name": "fetch",
      "params": [{"name": "url", "type": {"kind": "String"}}],
      "returnType": {"kind": "String"},
      "effects": ["network"],
      "grant": ["network"]
    }`
	entry := writeExternFixture(t, map[string]string{
		"platform.xql.json": platformModule,
		"main.xql.json":     externEntry(dupDecl),
	})

	node := mustParseFile(t, entry)
	merged, err := FlattenImports(node, entry)
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	count := 0
	for _, d := range merged.(*ast.Program).Decls {
		if ed, ok := d.(*ast.ExternDecl); ok && ed.Name == "fetch" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected identical externs to merge into one, got %d", count)
	}
}

// TestConflictingExternIsRejected: two modules that disagree about a host
// function's grant cannot both be right, and picking one silently would let the
// weaker declaration decide what the capability check enforces.
func TestConflictingExternIsRejected(t *testing.T) {
	conflicting := `,
    {
      "kind": "ExternDecl",
      "name": "fetch",
      "params": [{"name": "url", "type": {"kind": "String"}}],
      "returnType": {"kind": "String"},
      "effects": ["network"],
      "grant": []
    }`
	entry := writeExternFixture(t, map[string]string{
		"platform.xql.json": platformModule,
		"main.xql.json":     externEntry(conflicting),
	})

	result := CompileFromFile(entry, "go", "")
	if result.Success {
		t.Fatal("expected conflicting extern declarations to be rejected")
	}
	if !strings.Contains(result.Error, "conflicting signatures") {
		t.Errorf("expected a conflicting-signature error, got: %s", result.Error)
	}
}

// TestExternNameSurvivesAliasStripping: the linker rewrites alias-qualified
// references to bare names, and must not mistake the extern "platform.log" for
// a reference into the module imported as "platform".
func TestExternNameSurvivesAliasStripping(t *testing.T) {
	hostLog := `{
  "kind": "Program",
  "declarations": [
    {
      "kind": "ExternDecl",
      "name": "platform.log",
      "params": [{"name": "msg", "type": {"kind": "String"}}],
      "returnType": {"kind": "Void"},
      "effects": ["state"],
      "grant": []
    }
  ]
}`
	entry := `{
  "kind": "Program",
  "declarations": [
    {"kind": "ImportDecl", "path": "./platform.xql.json", "as": "platform"},
    {
      "kind": "FunctionDecl",
      "name": "main",
      "params": [],
      "returnType": {"kind": "Void"},
      "effects": ["state"],
      "grant": [],
      "body": [{
        "kind": "ExprStmt",
        "expr": {
          "kind": "CallExpr",
          "callee": "platform.log",
          "args": [{"kind": "Literal", "valueType": "String", "value": "hi"}]
        }
      }]
    }
  ]
}`
	path := writeExternFixture(t, map[string]string{
		"platform.xql.json": hostLog,
		"main.xql.json":     entry,
	})

	result := CompileFromFile(path, "go", "")
	if !result.Success {
		t.Fatalf("compile: %s", result.Error)
	}
	if !strings.Contains(string(result.Code), "platform.log(") {
		t.Errorf("expected the extern name to survive intact, got:\n%s", result.Code)
	}
}
