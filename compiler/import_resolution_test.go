package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const importedModuleJSON = `{
  "kind": "Program",
  "declarations": [
    {
      "kind": "FunctionDecl",
      "name": "helper",
      "params": [],
      "returnType": {"kind": "Void"},
      "effects": ["state"],
      "grant": ["io"],
      "body": [
        {
          "kind": "ExprStmt",
          "expr": {
            "kind": "CallExpr",
            "callee": "println",
            "args": [{"kind": "Literal", "valueType": "String", "value": "from module"}]
          }
        }
      ]
    }
  ]
}`

const entryWithImportJSON = `{
  "kind": "Program",
  "declarations": [
    {"kind": "ImportDecl", "path": "./mod.xql.json", "as": "mod"},
    {
      "kind": "FunctionDecl",
      "name": "main",
      "params": [],
      "returnType": {"kind": "Void"},
      "effects": ["state"],
      "grant": ["io"],
      "body": [
        {
          "kind": "ExprStmt",
          "expr": {"kind": "CallExpr", "callee": "mod.helper", "args": []}
        }
      ]
    }
  ]
}`

// writeImportFixture lays out an entry file importing a sibling module and
// returns the entry file's path.
func writeImportFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	modPath := filepath.Join(dir, "mod.xql.json")
	if err := os.WriteFile(modPath, []byte(importedModuleJSON), 0644); err != nil {
		t.Fatalf("write module: %v", err)
	}
	entryPath := filepath.Join(dir, "main.xql.json")
	if err := os.WriteFile(entryPath, []byte(entryWithImportJSON), 0644); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	return entryPath
}

// TestCompileFromFileResolvesImportsRelativeToEntry verifies imports resolve
// against the entry file's directory rather than the process working
// directory, so an absolute --file path works from anywhere.
func TestCompileFromFileResolvesImportsRelativeToEntry(t *testing.T) {
	entryPath := writeImportFixture(t)

	// Run from a directory that does not contain the module.
	otherDir := t.TempDir()
	restore := chdir(t, otherDir)
	defer restore()

	res := CompileFromFile(entryPath, "go", "")
	if !res.Success {
		t.Fatalf("expected success, got error: %s", res.Error)
	}
}

// TestValidateResolvesImportsViaEntryFile covers the same wiring on the
// Validate path.
func TestValidateResolvesImportsViaEntryFile(t *testing.T) {
	entryPath := writeImportFixture(t)

	data, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}
	pr := ParseAST(ParseRequest{Data: data, FilePath: entryPath})
	if !pr.Success {
		t.Fatalf("parse failed: %s", pr.Error)
	}

	otherDir := t.TempDir()
	restore := chdir(t, otherDir)
	defer restore()

	vr := Validate(ValidateRequest{AST: pr.AST, EntryFile: entryPath})
	if !vr.Success {
		t.Fatalf("expected validation to succeed, got: %s", vr.Error)
	}
}

// TestValidateWithoutEntryFileReportsMissingImport documents that a bare AST
// with no entry context cannot resolve relative imports.
func TestValidateWithoutEntryFileReportsMissingImport(t *testing.T) {
	pr := ParseAST(ParseRequest{Data: []byte(entryWithImportJSON)})
	if !pr.Success {
		t.Fatalf("parse failed: %s", pr.Error)
	}

	otherDir := t.TempDir()
	restore := chdir(t, otherDir)
	defer restore()

	vr := Validate(ValidateRequest{AST: pr.AST})
	if vr.Success {
		t.Fatal("expected failure without entry context")
	}
	if !strings.Contains(vr.Error, "XQL_E404") {
		t.Errorf("expected XQL_E404, got: %s", vr.Error)
	}
}

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return func() { _ = os.Chdir(prev) }
}
