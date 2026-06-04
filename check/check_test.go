package check

import (
	"strings"
	"testing"

	"xiaoqinli/ast"
)

// helper: parse JSON string to AST node
func mustParse(t *testing.T, src string) ast.Node {
	t.Helper()
	node, err := ast.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return node
}

// --- Type checker tests ---

func TestTypeCheckPass(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "add",
			"params": [
				{"name": "a", "type": {"kind": "Int"}},
				{"name": "b", "type": {"kind": "Int"}}
			],
			"returnType": {"kind": "Int"},
			"effects": ["pure"],
			"grant": [],
			"body": [{
				"kind": "ReturnStmt",
				"value": {"kind": "BinaryExpr", "op": "+",
					"left": {"kind": "Ident", "name": "a"},
					"right": {"kind": "Ident", "name": "b"}}
			}]
		}]
	}`

	root := mustParse(t, src)
	tc := NewTypeChecker()
	if err := tc.Check(root); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestTypeCheckReturnMismatch(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "f",
			"params": [],
			"returnType": {"kind": "Int"},
			"effects": ["pure"],
			"grant": [],
			"body": [{
				"kind": "ReturnStmt",
				"value": {"kind": "Literal", "valueType": "String", "value": "hello"}
			}]
		}]
	}`

	root := mustParse(t, src)
	tc := NewTypeChecker()
	err := tc.Check(root)
	if err == nil {
		t.Fatal("expected type error")
	}
	if !strings.Contains(err.Error(), "return type mismatch") {
		t.Errorf("expected return type mismatch, got: %v", err)
	}
}

func TestTypeCheckVarMismatch(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "f",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": [{
				"kind": "VarDecl",
				"name": "x",
				"type": {"kind": "Int"},
				"value": {"kind": "Literal", "valueType": "String", "value": "oops"}
			}]
		}]
	}`

	root := mustParse(t, src)
	tc := NewTypeChecker()
	err := tc.Check(root)
	if err == nil {
		t.Fatal("expected type error")
	}
	if !strings.Contains(err.Error(), "type mismatch") {
		t.Errorf("expected type mismatch, got: %v", err)
	}
}

func TestTypeCheckArgCountMismatch(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "add",
				"params": [
					{"name": "a", "type": {"kind": "Int"}},
					{"name": "b", "type": {"kind": "Int"}}
				],
				"returnType": {"kind": "Int"},
				"effects": ["pure"],
				"grant": [],
				"body": [{"kind": "ReturnStmt", "value": {"kind": "BinaryExpr", "op": "+",
					"left": {"kind": "Ident", "name": "a"}, "right": {"kind": "Ident", "name": "b"}}}]
			},
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "add", "args": [
						{"kind": "Literal", "valueType": "Int", "value": 1}
					]}
				}]
			}
		]
	}`

	root := mustParse(t, src)
	tc := NewTypeChecker()
	err := tc.Check(root)
	if err == nil {
		t.Fatal("expected arg count error")
	}
	if !strings.Contains(err.Error(), "expects 2 args, got 1") {
		t.Errorf("expected arg count mismatch, got: %v", err)
	}
}

func TestTypeCheckBoolCondition(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "f",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": [{
				"kind": "IfStmt",
				"condition": {"kind": "Literal", "valueType": "Int", "value": 1},
				"then": [],
				"else": []
			}]
		}]
	}`

	root := mustParse(t, src)
	tc := NewTypeChecker()
	err := tc.Check(root)
	if err == nil {
		t.Fatal("expected bool condition error")
	}
	if !strings.Contains(err.Error(), "must be Bool") {
		t.Errorf("expected condition type error, got: %v", err)
	}
}

// --- Effect tests ---

func TestEffectCheckPass(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "add",
			"params": [
				{"name": "a", "type": {"kind": "Int"}},
				{"name": "b", "type": {"kind": "Int"}}
			],
			"returnType": {"kind": "Int"},
			"effects": ["pure"],
			"grant": [],
			"body": [{
				"kind": "ReturnStmt",
				"value": {"kind": "BinaryExpr", "op": "+",
					"left": {"kind": "Ident", "name": "a"},
					"right": {"kind": "Ident", "name": "b"}}
			}]
		}]
	}`

	root := mustParse(t, src)
	if err := CheckEffects(root); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestEffectCheckPureViolation(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "f",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": ["pure"],
			"grant": [],
			"body": [{
				"kind": "ExprStmt",
				"expr": {"kind": "CallExpr", "callee": "println", "args": [
					{"kind": "Literal", "valueType": "String", "value": "hi"}
				]}
			}]
		}]
	}`

	root := mustParse(t, src)
	err := CheckEffects(root)
	if err == nil {
		t.Fatal("expected effect violation")
	}
	if !strings.Contains(err.Error(), "pure") && !strings.Contains(err.Error(), "state") {
		t.Errorf("expected pure/state conflict, got: %v", err)
	}
}

// --- Capability tests ---

func TestCapabilityCheckPass(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "helper",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["io"],
				"body": []
			},
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["io", "network"],
				"body": [{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "helper", "args": []}
				}]
			}
		]
	}`

	root := mustParse(t, src)
	if err := CheckCapabilities(root); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCapabilityCheckFail(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "dangerous",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network"],
				"body": []
			},
			{
				"kind": "FunctionDecl",
				"name": "caller",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "dangerous", "args": []}
				}]
			}
		]
	}`

	root := mustParse(t, src)
	err := CheckCapabilities(root)
	if err == nil {
		t.Fatal("expected capability error")
	}
	if !strings.Contains(err.Error(), "capability") {
		t.Errorf("expected capability error, got: %v", err)
	}
}

func TestEffectTransitivePropagation(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "printer",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": ["state"],
				"grant": [],
				"body": [{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "println", "args": [{"kind": "Literal", "valueType": "String", "value": "hi"}]}
				}]
			},
			{
				"kind": "FunctionDecl",
				"name": "wrapper",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": ["pure"],
				"grant": [],
				"body": [{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "printer", "args": []}
				}]
			}
		]
	}`

	root := mustParse(t, src)
	err := CheckEffects(root)
	if err == nil {
		t.Fatal("expected effect violation: wrapper calls printer but declares pure")
	}
}

func TestForStmtTypeCheck(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": [
				{
					"kind": "VarDecl", "name": "nums",
					"type": {"kind": "Array", "elem": {"kind": "Int"}},
					"value": {"kind": "ArrayLit", "elemType": {"kind": "Int"}, "elements": [
						{"kind": "Literal", "valueType": "Int", "value": 1}
					]}
				},
				{
					"kind": "ForStmt", "form": "range", "var": "i",
					"start": {"kind": "Literal", "valueType": "Int", "value": 0},
					"end": {"kind": "Literal", "valueType": "Int", "value": 5},
					"body": []
				},
				{
					"kind": "ForStmt", "form": "each", "var": "n",
					"iterable": {"kind": "Ident", "name": "nums"},
					"body": []
				}
			]
		}]
	}`
	root := mustParse(t, src)
	tc := NewTypeChecker()
	if err := tc.Check(root); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestForRangeTypeMismatch(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": [{
				"kind": "ForStmt", "form": "range", "var": "i",
				"start": {"kind": "Literal", "valueType": "String", "value": "bad"},
				"end": {"kind": "Literal", "valueType": "Int", "value": 5},
				"body": []
			}]
		}]
	}`
	root := mustParse(t, src)
	tc := NewTypeChecker()
	err := tc.Check(root)
	if err == nil {
		t.Fatal("expected type error for string start in for-range")
	}
	if !strings.Contains(err.Error(), "for-range start must be Int") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestScopeNesting(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": [
				{
					"kind": "VarDecl", "name": "x",
					"type": {"kind": "Int"},
					"value": {"kind": "Literal", "valueType": "Int", "value": 1}
				},
				{
					"kind": "IfStmt",
					"cond": {"kind": "Literal", "valueType": "Bool", "value": true},
					"then": [{
						"kind": "VarDecl", "name": "y",
						"type": {"kind": "String"},
						"value": {"kind": "Literal", "valueType": "String", "value": "hi"}
					}],
					"else": []
				},
				{
					"kind": "AssignStmt",
					"target": "x",
					"value": {"kind": "Literal", "valueType": "Int", "value": 2}
				}
			]
		}]
	}`
	root := mustParse(t, src)
	tc := NewTypeChecker()
	if err := tc.Check(root); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCapabilityInForStmt(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "dangerous",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network"],
				"body": []
			},
			{
				"kind": "FunctionDecl",
				"name": "caller",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [{
					"kind": "ForStmt", "form": "range", "var": "i",
					"start": {"kind": "Literal", "valueType": "Int", "value": 0},
					"end": {"kind": "Literal", "valueType": "Int", "value": 3},
					"body": [{
						"kind": "ExprStmt",
						"expr": {"kind": "CallExpr", "callee": "dangerous", "args": []}
					}]
				}]
			}
		]
	}`
	root := mustParse(t, src)
	err := CheckCapabilities(root)
	if err == nil {
		t.Fatal("expected capability error for call in for loop body")
	}
}

func TestIndexExprTypeInference(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Int"},
			"effects": [],
			"grant": [],
			"body": [
				{
					"kind": "VarDecl", "name": "nums",
					"type": {"kind": "Array", "elem": {"kind": "Int"}},
					"value": {"kind": "ArrayLit", "elemType": {"kind": "Int"}, "elements": [
						{"kind": "Literal", "valueType": "Int", "value": 1}
					]}
				},
				{
					"kind": "ReturnStmt",
					"value": {"kind": "IndexExpr",
						"target": {"kind": "Ident", "name": "nums"},
						"index": {"kind": "Literal", "valueType": "Int", "value": 0}
					}
				}
			]
		}]
	}`
	root := mustParse(t, src)
	tc := NewTypeChecker()
	if err := tc.Check(root); err != nil {
		t.Errorf("expected pass (IndexExpr on Array<Int> returns Int), got: %v", err)
	}
}

// --- RunAll integration test ---

func TestRunAllPass(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": ["state"],
			"grant": ["io"],
			"body": [{
				"kind": "ExprStmt",
				"expr": {"kind": "CallExpr", "callee": "println", "args": [
					{"kind": "Literal", "valueType": "String", "value": "hello"}
				]}
			}]
		}]
	}`

	root := mustParse(t, src)
	if err := RunAll(root); err != nil {
		t.Errorf("expected all checks pass, got: %v", err)
	}
}
