package check

import (
	"strings"
	"testing"

	"xiaoqinli/ast"
	"xiaoqinli/vfs"
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

func TestLambdaTypeCheckMismatch(t *testing.T) {
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
				"kind": "ExprStmt",
				"expr": {
					"kind": "Lambda",
					"params": [{"name": "x", "type": {"kind": "Int"}}],
					"returnType": {"kind": "Int"},
					"body": [{
						"kind": "ReturnStmt",
						"value": {"kind": "Literal", "valueType": "String", "value": "not an int"}
					}]
				}
			}]
		}]
	}`

	root := mustParse(t, src)
	tc := NewTypeChecker()
	err := tc.Check(root)
	if err == nil {
		t.Fatal("expected type error inside lambda")
	}
	if !strings.Contains(err.Error(), "return type mismatch") {
		t.Errorf("expected return type mismatch inside lambda, got: %v", err)
	}
}

func TestLambdaCapabilityCheckFail(t *testing.T) {
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
					"expr": {
						"kind": "Lambda",
						"params": [],
						"returnType": {"kind": "Void"},
						"body": [{
							"kind": "ExprStmt",
							"expr": {"kind": "CallExpr", "callee": "dangerous", "args": []}
						}]
					}
				}]
			}
		]
	}`

	root := mustParse(t, src)
	err := CheckCapabilities(root)
	if err == nil {
		t.Fatal("expected capability error inside lambda")
	}
	if !strings.Contains(err.Error(), "lacks required capabilities") {
		t.Errorf("expected capability error, got: %v", err)
	}
}

func TestNilNodeSafety(t *testing.T) {
	if err := RunAll(nil); err == nil || !strings.Contains(err.Error(), "root node is nil") {
		t.Errorf("RunAll(nil) expected 'root node is nil', got: %v", err)
	}
	tc := NewTypeChecker()
	if err := tc.Check(nil); err == nil || !strings.Contains(err.Error(), "root node is nil") {
		t.Errorf("tc.Check(nil) expected 'root node is nil', got: %v", err)
	}
	if err := CheckEffects(nil); err == nil || !strings.Contains(err.Error(), "root node is nil") {
		t.Errorf("CheckEffects(nil) expected 'root node is nil', got: %v", err)
	}
	if err := CheckCapabilities(nil); err == nil || !strings.Contains(err.Error(), "root node is nil") {
		t.Errorf("CheckCapabilities(nil) expected 'root node is nil', got: %v", err)
	}
	none := tc.inferType(nil, nil)
	if none.KindName != "" || none.Elem != nil {
		t.Errorf("inferType(nil) expected empty TypeExpr, got: %+v", none)
	}
}

func TestVerifyCapabilityMissing(t *testing.T) {
	caller := Capability([]string{"io"})
	required := Capability([]string{"network"})
	err := VerifyCapability(caller, required)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "XQL_E301") {
		t.Errorf("expected XQL_E301, got: %v", err)
	}
}

func TestPureCallsImpure(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "impureFunc",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": ["state"],
				"grant": [],
				"body": [{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "println", "args": []}
				}]
			},
			{
				"kind": "FunctionDecl",
				"name": "pureFunc",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": ["pure"],
				"grant": [],
				"body": [{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "impureFunc", "args": []}
				}]
			}
		]
	}`
	root := mustParse(t, src)
	err := CheckEffects(root)
	if err == nil {
		t.Fatal("expected effect error")
	}
	if !strings.Contains(err.Error(), "XQL_E203") {
		t.Errorf("expected XQL_E203, got: %v", err)
	}
}

func TestCapabilityNotDeclared(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "networkFunc",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network"],
				"body": []
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
					"expr": {"kind": "CallExpr", "callee": "networkFunc", "args": []}
				}]
			}
		]
	}`
	root := mustParse(t, src)
	err := CheckCapabilities(root)
	if err == nil {
		t.Fatal("expected capability error")
	}
	if !strings.Contains(err.Error(), "XQL_E302") {
		t.Errorf("expected XQL_E302, got: %v", err)
	}
}

func TestGrantChainBroken(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "C",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network"],
				"body": []
			},
			{
				"kind": "FunctionDecl",
				"name": "B",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "C", "args": []}
				}]
			},
			{
				"kind": "FunctionDecl",
				"name": "A",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network"],
				"body": [{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "B", "args": []}
				}]
			}
		]
	}`
	root := mustParse(t, src)
	err := CheckCapabilities(root)
	if err == nil {
		t.Fatal("expected capability error due to broken chain")
	}
	if !strings.Contains(err.Error(), "XQL_E302") {
		t.Errorf("expected XQL_E302, got: %v", err)
	}
}

func TestClassFieldTypeInference(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "ClassDecl",
				"name": "User",
				"fields": [
					{"name": "id", "type": {"kind": "Int"}, "visibility": "private"},
					{"name": "name", "type": {"kind": "String"}, "visibility": "public"}
				]
			},
			{
				"kind": "FunctionDecl",
				"name": "f",
				"params": [{"name": "u", "type": {"kind": "User"}}],
				"returnType": {"kind": "String"},
				"effects": [],
				"grant": [],
				"body": [
					{
						"kind": "ReturnStmt",
						"value": {
							"kind": "MemberExpr",
							"object": {"kind": "Ident", "name": "u"},
							"field": "name"
						}
					}
				]
			}
		]
	}`
	root := mustParse(t, src)
	if err := RunAll(root); err != nil {
		t.Errorf("Expected ClassDecl field inference to pass, got: %v", err)
	}
}

func TestSwitchStmtTypeCheck(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "f",
				"params": [{"name": "x", "type": {"kind": "Int"}}],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [
					{
						"kind": "SwitchStmt",
						"value": {"kind": "Ident", "name": "x"},
						"cases": [
							{
								"value": {"kind": "Literal", "valueType": "Int", "value": 1},
								"body": []
							},
							{
								"value": {"kind": "Literal", "valueType": "String", "value": "bad"},
								"body": []
							}
						]
					}
				]
			}
		]
	}`
	root := mustParse(t, src)
	err := RunAll(root)
	if err == nil {
		t.Fatal("expected type mismatch error in switch")
	}
	if !strings.Contains(err.Error(), "switch case value type mismatch") {
		t.Errorf("expected switch case type mismatch error, got: %v", err)
	}
}

func TestMapLiteralTypeCheck(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "f",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [
					{
						"kind": "VarDecl",
						"name": "m",
						"type": {"kind": "Map", "keyType": {"kind": "String"}, "valueType": {"kind": "Int"}},
						"value": {
							"kind": "MapLiteral",
							"keyType": {"kind": "String"},
							"valueType": {"kind": "Int"},
							"entries": [
								{
									"key": {"kind": "Literal", "valueType": "Int", "value": 1},
									"value": {"kind": "Literal", "valueType": "Int", "value": 1}
								}
							]
						}
					}
				]
			}
		]
	}`
	root := mustParse(t, src)
	err := RunAll(root)
	if err == nil {
		t.Fatal("expected key type mismatch error in map literal")
	}
	if !strings.Contains(err.Error(), "map key 0: expected String, got Int") {
		t.Errorf("expected map key mismatch error, got: %v", err)
	}
}

func TestWorkspaceCircularImport(t *testing.T) {
	ws := vfs.New()
	ws.Write("a.xql", []byte(`{
		"kind": "Program",
		"declarations": [
			{
				"kind": "ImportDecl",
				"path": "./b.xql",
				"as": "b"
			}
		]
	}`))
	ws.Write("b.xql", []byte(`{
		"kind": "Program",
		"declarations": [
			{
				"kind": "ImportDecl",
				"path": "./a.xql",
				"as": "a"
			}
		]
	}`))

	progABytes, _ := ws.Read("a.xql")
	progA, err := ast.Parse(progABytes)
	if err != nil {
		t.Fatalf("parse a.xql failed: %v", err)
	}

	err = RunAllInWorkspace(progA, "a.xql", ws)
	if err == nil {
		t.Fatal("expected circular import error")
	}
	if !strings.Contains(err.Error(), "circular import detected") {
		t.Errorf("expected circular import error, got: %v", err)
	}
}

func TestWorkspaceCrossFileCheck(t *testing.T) {
	ws := vfs.New()
	ws.Write("utils.xql", []byte(`{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "netCall",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network"],
				"body": []
			},
			{
				"kind": "FunctionDecl",
				"name": "add",
				"params": [],
				"returnType": {"kind": "Int"},
				"effects": ["pure"],
				"grant": [],
				"body": []
			}
		]
	}`))

	// Case 1: Capability Violation
	ws.Write("main_cap_violation.xql", []byte(`{
		"kind": "Program",
		"declarations": [
			{
				"kind": "ImportDecl",
				"path": "./utils.xql",
				"as": "utils"
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
							"callee": "utils.netCall",
							"args": []
						}
					}
				]
			}
		]
	}`))

	progBytes, _ := ws.Read("main_cap_violation.xql")
	prog, _ := ast.Parse(progBytes)
	err := RunAllInWorkspace(prog, "main_cap_violation.xql", ws)
	if err == nil {
		t.Fatal("expected capability lack error")
	}
	if !strings.Contains(err.Error(), "lacks required capabilities") {
		t.Errorf("expected capability error, got: %v", err)
	}

	// Case 2: Effect Violation
	ws.Write("main_effect_violation.xql", []byte(`{
		"kind": "Program",
		"declarations": [
			{
				"kind": "ImportDecl",
				"path": "./utils.xql",
				"as": "utils"
			},
			{
				"kind": "FunctionDecl",
				"name": "pureFunc",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": ["pure"],
				"grant": [],
				"body": [
					{
						"kind": "ExprStmt",
						"expr": {
							"kind": "CallExpr",
							"callee": "utils.netCall",
							"args": []
						}
					}
				]
			}
		]
	}`))

	progBytes, _ = ws.Read("main_effect_violation.xql")
	prog, _ = ast.Parse(progBytes)
	err = RunAllInWorkspace(prog, "main_effect_violation.xql", ws)
	if err == nil {
		t.Fatal("expected pure function calling impure function error")
	}
	if !strings.Contains(err.Error(), "declares @effects") || !strings.Contains(err.Error(), "but has inferred effect") {
		t.Errorf("expected effect error, got: %v", err)
	}
}

func TestWorkspaceGlobalSymbolConflict(t *testing.T) {
	ws := vfs.New()
	ws.Write("utils.xql", []byte(`{
		"kind": "Program",
		"declarations": [
			{
				"kind": "StructDecl",
				"name": "Point",
				"fields": [
					{"name": "x", "type": {"kind": "Int"}}
				]
			}
		]
	}`))
	ws.Write("models.xql", []byte(`{
		"kind": "Program",
		"declarations": [
			{
				"kind": "StructDecl",
				"name": "Point",
				"fields": [
					{"name": "y", "type": {"kind": "Int"}}
				]
			}
		]
	}`))
	ws.Write("main.xql", []byte(`{
		"kind": "Program",
		"declarations": [
			{
				"kind": "ImportDecl",
				"path": "./utils.xql",
				"as": "utils"
			},
			{
				"kind": "ImportDecl",
				"path": "./models.xql",
				"as": "models"
			}
		]
	}`))

	progBytes, _ := ws.Read("main.xql")
	prog, _ := ast.Parse(progBytes)
	err := RunAllInWorkspace(prog, "main.xql", ws)
	if err == nil {
		t.Fatal("expected duplicate global symbol conflict error")
	}
	if !strings.Contains(err.Error(), "is defined in multiple files") {
		t.Errorf("expected duplicate global symbol error, got: %v", err)
	}
}

func TestScopedCapabilityHierarchy(t *testing.T) {
	src1 := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "netWrite",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network:write"],
				"body": []
			},
			{
				"kind": "FunctionDecl",
				"name": "caller",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network:read"],
				"body": [
					{
						"kind": "ExprStmt",
						"expr": {
							"kind": "CallExpr",
							"callee": "netWrite",
							"args": []
						}
					}
				]
			}
		]
	}`
	prog1, _ := ast.Parse([]byte(src1))
	err1 := RunAll(prog1)
	if err1 == nil {
		t.Fatal("expected capability error (network:read vs network:write)")
	}
	if !strings.Contains(err1.Error(), "lacks required capabilities") {
		t.Errorf("expected capability error, got: %v", err1)
	}

	src2 := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "netWrite",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network:write"],
				"body": []
			},
			{
				"kind": "FunctionDecl",
				"name": "caller",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network:*"],
				"body": [
					{
						"kind": "ExprStmt",
						"expr": {
							"kind": "CallExpr",
							"callee": "netWrite",
							"args": []
						}
					}
				]
			}
		]
	}`
	prog2, _ := ast.Parse([]byte(src2))
	err2 := RunAll(prog2)
	if err2 != nil {
		t.Fatalf("expected capability check to pass, got: %v", err2)
	}
}

func TestDiagnosticSuggestedFix(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "needNet",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network:write"],
				"body": []
			},
			{
				"kind": "FunctionDecl",
				"name": "caller",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [
					{
						"kind": "VarDecl",
						"name": "x",
						"type": {"kind": "Int"},
						"value": {"kind": "Literal", "valueType": "String", "value": "a"}
					},
					{
						"kind": "ExprStmt",
						"expr": {
							"kind": "CallExpr",
							"callee": "needNet",
							"args": []
						}
					}
				]
			}
		]
	}`

	prog, _ := ast.Parse([]byte(src))
	err := RunAll(prog)
	if err == nil {
		t.Fatal("expected errors")
	}

	we, ok := err.(WorkspaceError)
	if !ok {
		t.Fatalf("expected error to be WorkspaceError, got: %T", err)
	}

	if len(we.Diagnostics) < 2 {
		t.Fatalf("expected at least 2 diagnostics, got: %d", len(we.Diagnostics))
	}

	typeFound := false
	capFound := false

	for _, d := range we.Diagnostics {
		if d.Code == "" {
			t.Error("expected Code to be non-empty")
		}
		if d.Message == "" {
			t.Error("expected Message to be non-empty")
		}
		if d.SuggestedFix == "" {
			t.Error("expected SuggestedFix to be non-empty")
		}

		if d.Code == "XQL_E201" && strings.Contains(d.Message, "declared Int but assigned String") {
			typeFound = true
			if !strings.Contains(d.SuggestedFix, "Change the expression") {
				t.Errorf("unexpected SuggestedFix for type mismatch: %s", d.SuggestedFix)
			}
		}

		if d.Code == "XQL_E301" && strings.Contains(d.Message, "lacks required capabilities") {
			capFound = true
			if !strings.Contains(d.SuggestedFix, "Add the missing capability name") {
				t.Errorf("unexpected SuggestedFix for capability mismatch: %s", d.SuggestedFix)
			}
		}
	}

	if !typeFound {
		t.Error("expected type mismatch diagnostic to be found")
	}
	if !capFound {
		t.Error("expected capability lack diagnostic to be found")
	}
}

func TestStrictCapabilityUnresolvedCall(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "danger",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": [{
				"kind": "ExprStmt",
				"expr": {
					"kind": "CallExpr",
					"callee": "totally_unknown_dangerous_syscall_wrapper",
					"args": []
				}
			}]
		}]
	}`

	root := mustParse(t, src)

	// Non-strict check should pass without error
	if err := CheckCapabilities(root); err != nil {
		t.Errorf("expected non-strict CheckCapabilities to pass, got: %v", err)
	}

	// Strict check should fail with XQL_E303
	err := CheckCapabilitiesStrict(root)
	if err == nil {
		t.Fatal("expected strict CheckCapabilities to fail on unresolved call")
	}
	if !strings.Contains(err.Error(), "XQL_E303") {
		t.Errorf("expected error to contain XQL_E303, got: %v", err)
	}
}
