package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xiaoqinli/ast"
)

const modelsModule = `{
  "kind": "Program",
  "declarations": [
    {
      "kind": "StructDecl",
      "name": "Config",
      "fields": [{"name": "retries", "type": {"kind": "Int"}}]
    }
  ]
}`

const serviceModule = `{
  "kind": "Program",
  "declarations": [
    {"kind": "ImportDecl", "path": "./models.xql.json", "as": "models"},
    {
      "kind": "FunctionDecl",
      "name": "describe",
      "params": [{"name": "cfg", "type": {"kind": "models.Config"}}],
      "returnType": {"kind": "Void"},
      "effects": ["state"],
      "grant": ["io"],
      "body": [{
        "kind": "ExprStmt",
        "expr": {
          "kind": "CallExpr",
          "callee": "println",
          "args": [{"kind": "Literal", "valueType": "String", "value": "hi"}]
        }
      }]
    }
  ]
}`

const linkEntry = `{
  "kind": "Program",
  "declarations": [
    {"kind": "ImportDecl", "path": "./models.xql.json", "as": "models"},
    {"kind": "ImportDecl", "path": "./service.xql.json", "as": "svc"},
    {
      "kind": "FunctionDecl",
      "name": "main",
      "params": [],
      "returnType": {"kind": "Void"},
      "effects": ["state"],
      "grant": ["io"],
      "body": [
        {
          "kind": "VarDecl",
          "name": "cfg",
          "type": {"kind": "models.Config"},
          "value": {
            "kind": "StructLit",
            "typeName": "models.Config",
            "fields": [{"name": "retries", "value": {"kind": "Literal", "valueType": "Int", "value": 3}}]
          }
        },
        {
          "kind": "ExprStmt",
          "expr": {"kind": "CallExpr", "callee": "svc.describe", "args": [{"kind": "Ident", "name": "cfg"}]}
        }
      ]
    }
  ]
}`

// writeLinkFixture lays out entry + two modules, where the entry and the
// service module both import models (a diamond).
func writeLinkFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range map[string]string{
		"models.xql.json":  modelsModule,
		"service.xql.json": serviceModule,
		"main.xql.json":    linkEntry,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return filepath.Join(dir, "main.xql.json")
}

func mustParseFile(t *testing.T, path string) ast.Node {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	node, err := ast.Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return node
}

// TestFlattenImportsMergesModules checks that every imported declaration ends
// up in the merged Program, that ImportDecl nodes are dropped, and that a
// module reachable twice is merged only once.
func TestFlattenImportsMergesModules(t *testing.T) {
	entry := writeLinkFixture(t)
	linked, err := FlattenImports(mustParseFile(t, entry), entry)
	if err != nil {
		t.Fatalf("FlattenImports: %v", err)
	}
	prog := linked.(*ast.Program)

	structs, funcs := 0, 0
	for _, d := range prog.Decls {
		switch n := d.(type) {
		case *ast.ImportDecl:
			t.Error("ImportDecl survived flattening")
		case *ast.StructDecl:
			structs++
		case *ast.FunctionDecl:
			funcs++
			_ = n
		}
	}
	if structs != 1 {
		t.Errorf("expected Config merged exactly once (diamond import), got %d", structs)
	}
	if funcs != 2 {
		t.Errorf("expected describe + main, got %d functions", funcs)
	}
}

// TestFlattenImportsStripsAliases covers all three qualified-reference forms:
// a call callee, a declared type, and a struct literal's type name.
func TestFlattenImportsStripsAliases(t *testing.T) {
	entry := writeLinkFixture(t)
	linked, err := FlattenImports(mustParseFile(t, entry), entry)
	if err != nil {
		t.Fatalf("FlattenImports: %v", err)
	}
	prog := linked.(*ast.Program)

	var main *ast.FunctionDecl
	var describe *ast.FunctionDecl
	for _, d := range prog.Decls {
		if fd, ok := d.(*ast.FunctionDecl); ok {
			switch fd.Name {
			case "main":
				main = fd
			case "describe":
				describe = fd
			}
		}
	}
	if main == nil || describe == nil {
		t.Fatal("merged program is missing main or describe")
	}

	vd := main.Body[0].(*ast.VarDecl)
	if vd.Type.KindName != "Config" {
		t.Errorf("VarDecl type: want Config, got %q", vd.Type.KindName)
	}
	if sl, ok := vd.Value.(*ast.StructLit); !ok || sl.TypeName != "Config" {
		t.Errorf("StructLit type name not stripped: %+v", vd.Value)
	}
	call := main.Body[1].(*ast.ExprStmt).Expr.(*ast.CallExpr)
	if call.Callee != "describe" {
		t.Errorf("callee: want describe, got %q", call.Callee)
	}
	// Aliases are stripped across modules, not just in the entry file.
	if describe.Params[0].Type.KindName != "Config" {
		t.Errorf("param type in imported module: want Config, got %q", describe.Params[0].Type.KindName)
	}
}

// TestFlattenImportsLeavesNonAliasDotsAlone guards the stripping rule: only a
// declared import alias is a prefix. A method call on a local variable, or a
// Result constructor, must survive untouched.
func TestFlattenImportsLeavesNonAliasDotsAlone(t *testing.T) {
	dir := t.TempDir()
	mod := `{"kind":"Program","declarations":[{"kind":"FunctionDecl","name":"helper","params":[],"returnType":{"kind":"Void"},"effects":[],"grant":[],"body":[]}]}`
	entry := `{
		"kind": "Program",
		"declarations": [
			{"kind": "ImportDecl", "path": "./m.xql.json", "as": "m"},
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [
					{"kind": "ExprStmt", "expr": {"kind": "CallExpr", "callee": "res.unwrap", "args": []}},
					{"kind": "ExprStmt", "expr": {"kind": "CallExpr", "callee": "Result.ok", "args": []}},
					{"kind": "ExprStmt", "expr": {"kind": "CallExpr", "callee": "m.helper", "args": []}}
				]
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(dir, "m.xql.json"), []byte(mod), 0644); err != nil {
		t.Fatal(err)
	}
	entryPath := filepath.Join(dir, "main.xql.json")
	if err := os.WriteFile(entryPath, []byte(entry), 0644); err != nil {
		t.Fatal(err)
	}

	linked, err := FlattenImports(mustParseFile(t, entryPath), entryPath)
	if err != nil {
		t.Fatalf("FlattenImports: %v", err)
	}
	var main *ast.FunctionDecl
	for _, d := range linked.(*ast.Program).Decls {
		if fd, ok := d.(*ast.FunctionDecl); ok && fd.Name == "main" {
			main = fd
		}
	}
	want := []string{"res.unwrap", "Result.ok", "helper"}
	for i, w := range want {
		got := main.Body[i].(*ast.ExprStmt).Expr.(*ast.CallExpr).Callee
		if got != w {
			t.Errorf("callee %d: want %q, got %q", i, w, got)
		}
	}
}

// TestFlattenImportsDetectsCycle ensures a cyclic graph is reported instead of
// recursing forever.
func TestFlattenImportsDetectsCycle(t *testing.T) {
	dir := t.TempDir()
	a := `{"kind":"Program","declarations":[{"kind":"ImportDecl","path":"./b.xql.json","as":"b"}]}`
	b := `{"kind":"Program","declarations":[{"kind":"ImportDecl","path":"./a.xql.json","as":"a"}]}`
	if err := os.WriteFile(filepath.Join(dir, "b.xql.json"), []byte(b), 0644); err != nil {
		t.Fatal(err)
	}
	aPath := filepath.Join(dir, "a.xql.json")
	if err := os.WriteFile(aPath, []byte(a), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := FlattenImports(mustParseFile(t, aPath), aPath)
	if err == nil {
		t.Fatal("expected a circular import error")
	}
	if !strings.Contains(err.Error(), "XQL_E402") {
		t.Errorf("expected XQL_E402, got: %v", err)
	}
}

// TestFlattenImportsNoopWithoutImports keeps single-file programs on the
// untouched path.
func TestFlattenImportsNoopWithoutImports(t *testing.T) {
	root, err := ast.Parse([]byte(helloJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	linked, err := FlattenImports(root, "")
	if err != nil {
		t.Fatalf("FlattenImports: %v", err)
	}
	if linked != root {
		t.Error("expected the original AST to be returned unchanged")
	}
}

// TestCompileMultiFileEmitsImportedDeclarations is the end-to-end guard: the
// generated Go must contain the imported module's declarations, which is
// exactly what used to be missing.
func TestCompileMultiFileEmitsImportedDeclarations(t *testing.T) {
	entry := writeLinkFixture(t)
	res := CompileFromFile(entry, "go", "")
	if !res.Success {
		t.Fatalf("compile failed: %s", res.Error)
	}
	code := string(res.Code)
	for _, want := range []string{"type Config struct", "func describe(", "func main()"} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code missing %q\n---\n%s", want, code)
		}
	}
	if strings.Contains(code, "models.Config") || strings.Contains(code, "svc.describe") {
		t.Errorf("generated code still contains qualified names:\n%s", code)
	}
}
