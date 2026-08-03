package check

import (
	"strings"
	"testing"

	"xiaoqinli/vfs"
)

// moduleWithGrantViolation declares privileged() requiring "io" and helper()
// with no grant calling it — a capability violation contained entirely within
// one module.
const moduleWithGrantViolation = `{
	"kind": "Program",
	"declarations": [
		{
			"kind": "FunctionDecl",
			"name": "privileged",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": ["state"],
			"grant": ["io"],
			"body": [{
				"kind": "ExprStmt",
				"expr": {
					"kind": "CallExpr",
					"callee": "println",
					"args": [{"kind": "Literal", "valueType": "String", "value": "x"}]
				}
			}]
		},
		{
			"kind": "FunctionDecl",
			"name": "helper",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": [{
				"kind": "ExprStmt",
				"expr": {"kind": "CallExpr", "callee": "privileged", "args": []}
			}]
		}
	]
}`

const entryImportingModule = `{
	"kind": "Program",
	"declarations": [
		{"kind": "ImportDecl", "path": "./mod.xql", "as": "mod"},
		{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": [{
				"kind": "ExprStmt",
				"expr": {"kind": "CallExpr", "callee": "mod.helper", "args": []}
			}]
		}
	]
}`

// TestCapabilityViolationInsideImportedModule pins the behaviour that a grant
// violation is reported wherever it lives. Previously the capability pass
// walked only the entry Program, so moving offending code into an imported
// module hid it entirely.
func TestCapabilityViolationInsideImportedModule(t *testing.T) {
	// Control: the same module checked as an entry file must fail.
	standalone := mustParse(t, moduleWithGrantViolation)
	if err := CheckCapabilities(standalone); err == nil {
		t.Fatal("control failed: standalone module should report a grant violation")
	}

	ws := vfs.New()
	ws.Write("mod.xql", []byte(moduleWithGrantViolation))
	ws.Write("main.xql", []byte(entryImportingModule))

	prog := mustParse(t, entryImportingModule)
	err := RunAllInWorkspace(prog, "main.xql", ws)
	if err == nil {
		t.Fatal("expected the imported module's grant violation to be reported")
	}
	if !strings.Contains(err.Error(), "XQL_E301") {
		t.Errorf("expected XQL_E301, got: %v", err)
	}
}

// TestCleanImportPassesCapabilityCheck guards against the recursion producing
// false positives on a module whose grants are consistent.
func TestCleanImportPassesCapabilityCheck(t *testing.T) {
	cleanModule := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "helper",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": ["state"],
			"grant": ["io"],
			"body": [{
				"kind": "ExprStmt",
				"expr": {
					"kind": "CallExpr",
					"callee": "println",
					"args": [{"kind": "Literal", "valueType": "String", "value": "ok"}]
				}
			}]
		}]
	}`
	entry := `{
		"kind": "Program",
		"declarations": [
			{"kind": "ImportDecl", "path": "./mod.xql", "as": "mod"},
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": ["state"],
				"grant": ["io"],
				"body": [{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "mod.helper", "args": []}
				}]
			}
		]
	}`

	ws := vfs.New()
	ws.Write("mod.xql", []byte(cleanModule))
	ws.Write("main.xql", []byte(entry))

	prog := mustParse(t, entry)
	if err := RunAllInWorkspace(prog, "main.xql", ws); err != nil {
		t.Errorf("expected a consistent import graph to pass, got: %v", err)
	}
}
