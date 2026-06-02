package ast

import "testing"

func TestParseHello(t *testing.T) {
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
				"kind": "ExprStmt",
				"expr": {
					"kind": "CallExpr",
					"callee": "println",
					"args": [{"kind": "Literal", "valueType": "String", "value": "hello"}]
				}
			}]
		}]
	}`
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	prog, ok := root.(*Program)
	if !ok {
		t.Fatal("expected *Program")
	}
	if len(prog.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(prog.Decls))
	}
	fd := prog.Decls[0].(*FunctionDecl)
	if fd.Name != "main" {
		t.Errorf("expected 'main', got %q", fd.Name)
	}
}

func TestParseIfStmtCond(t *testing.T) {
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
				"kind": "IfStmt",
				"cond": {"kind": "Literal", "valueType": "Bool", "value": true},
				"then": [],
				"else": []
			}]
		}]
	}`
	_, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseIfStmtCondition(t *testing.T) {
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
				"kind": "IfStmt",
				"condition": {"kind": "Literal", "valueType": "Bool", "value": true},
				"then": [],
				"else": []
			}]
		}]
	}`
	_, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseIfStmtMissingCond(t *testing.T) {
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
				"kind": "IfStmt",
				"then": [],
				"else": []
			}]
		}]
	}`
	_, err := Parse([]byte(src))
	if err == nil {
		t.Fatal("expected error for missing cond")
	}
}

func TestParseWhileStmtMissingCond(t *testing.T) {
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
				"kind": "WhileStmt",
				"body": []
			}]
		}]
	}`
	_, err := Parse([]byte(src))
	if err == nil {
		t.Fatal("expected error for missing cond")
	}
}

func TestParseUnknownKind(t *testing.T) {
	src := `{"kind": "FooBar"}`
	_, err := Parse([]byte(src))
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}
