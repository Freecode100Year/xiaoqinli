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

func TestParseForStmtRange(t *testing.T) {
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
				"start": {"kind": "Literal", "valueType": "Int", "value": 0},
				"end": {"kind": "Literal", "valueType": "Int", "value": 10},
				"body": []
			}]
		}]
	}`
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	fd := root.(*Program).Decls[0].(*FunctionDecl)
	fs := fd.Body[0].(*ForStmt)
	if fs.Form != "range" || fs.Var != "i" {
		t.Errorf("expected range/i, got %s/%s", fs.Form, fs.Var)
	}
}

func TestParseForStmtInvalidForm(t *testing.T) {
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
				"kind": "ForStmt", "form": "bad", "var": "i",
				"start": {"kind": "Literal", "valueType": "Int", "value": 0},
				"end": {"kind": "Literal", "valueType": "Int", "value": 10},
				"body": []
			}]
		}]
	}`
	_, err := Parse([]byte(src))
	if err == nil {
		t.Fatal("expected error for invalid form")
	}
}

func TestParseBreakContinue(t *testing.T) {
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
				{"kind": "BreakStmt"},
				{"kind": "ContinueStmt"}
			]
		}]
	}`
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	fd := root.(*Program).Decls[0].(*FunctionDecl)
	if _, ok := fd.Body[0].(*BreakStmt); !ok {
		t.Error("expected BreakStmt")
	}
	if _, ok := fd.Body[1].(*ContinueStmt); !ok {
		t.Error("expected ContinueStmt")
	}
}

func TestParseAssignStmtStringTarget(t *testing.T) {
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
				"kind": "AssignStmt",
				"target": "x",
				"value": {"kind": "Literal", "valueType": "Int", "value": 1}
			}]
		}]
	}`
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	fd := root.(*Program).Decls[0].(*FunctionDecl)
	as := fd.Body[0].(*AssignStmt)
	ident, ok := as.Target.(*Ident)
	if !ok {
		t.Fatal("expected string target to parse as Ident")
	}
	if ident.Name != "x" {
		t.Errorf("expected 'x', got %q", ident.Name)
	}
}

func TestParseAssignStmtNodeTarget(t *testing.T) {
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
				"kind": "AssignStmt",
				"target": {
					"kind": "IndexExpr",
					"target": {"kind": "Ident", "name": "arr"},
					"index": {"kind": "Literal", "valueType": "Int", "value": 0}
				},
				"value": {"kind": "Literal", "valueType": "Int", "value": 99}
			}]
		}]
	}`
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	fd := root.(*Program).Decls[0].(*FunctionDecl)
	as := fd.Body[0].(*AssignStmt)
	if _, ok := as.Target.(*IndexExpr); !ok {
		t.Fatal("expected IndexExpr target")
	}
}

func TestParseUnknownKind(t *testing.T) {
	src := `{"kind": "FooBar"}`
	_, err := Parse([]byte(src))
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}
