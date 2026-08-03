package ast

import (
	"testing"
)

func TestParseMinimalProgram(t *testing.T) {
	input := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": []
			}
		]
	}`

	node, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	prog, ok := node.(*Program)
	if !ok {
		t.Fatal("expected *Program")
	}
	if len(prog.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(prog.Decls))
	}
	fd, ok := prog.Decls[0].(*FunctionDecl)
	if !ok {
		t.Fatal("expected *FunctionDecl")
	}
	if fd.Name != "main" {
		t.Errorf("expected name 'main', got %q", fd.Name)
	}
	if fd.ReturnType.KindName != "Void" {
		t.Errorf("expected return type Void, got %q", fd.ReturnType.KindName)
	}
}

func TestParseLiteral(t *testing.T) {
	input := `{
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
				"value": {"kind": "Literal", "valueType": "Int", "value": 42}
			}]
		}]
	}`

	node, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	prog := node.(*Program)
	fd := prog.Decls[0].(*FunctionDecl)
	ret := fd.Body[0].(*ReturnStmt)
	lit := ret.Value.(*Literal)
	if lit.ValueType != "Int" {
		t.Errorf("expected Int, got %s", lit.ValueType)
	}
	if v, ok := lit.Value.(float64); !ok || v != 42 {
		t.Errorf("expected 42, got %v", lit.Value)
	}
}

func TestParseAllNodeKinds(t *testing.T) {
	input := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "test",
			"params": [{"name": "x", "type": {"kind": "Int"}}],
			"returnType": {"kind": "Void"},
			"effects": ["state"],
			"grant": ["io"],
			"body": [
				{
					"kind": "VarDecl",
					"name": "a",
					"type": {"kind": "Int"},
					"value": {"kind": "Literal", "valueType": "Int", "value": 0}
				},
				{
					"kind": "AssignStmt",
					"target": "a",
					"value": {"kind": "BinaryExpr", "op": "+",
						"left": {"kind": "Ident", "name": "a"},
						"right": {"kind": "Literal", "valueType": "Int", "value": 1}}
				},
				{
					"kind": "IfStmt",
					"condition": {"kind": "BinaryExpr", "op": ">",
						"left": {"kind": "Ident", "name": "a"},
						"right": {"kind": "Literal", "valueType": "Int", "value": 0}},
					"then": [{"kind": "ExprStmt", "expr": {
						"kind": "CallExpr", "callee": "println",
						"args": [{"kind": "Ident", "name": "a"}]
					}}],
					"else": []
				},
				{
					"kind": "WhileStmt",
					"condition": {"kind": "BinaryExpr", "op": "<",
						"left": {"kind": "Ident", "name": "a"},
						"right": {"kind": "Literal", "valueType": "Int", "value": 10}},
					"body": [{
						"kind": "AssignStmt",
						"target": "a",
						"value": {"kind": "BinaryExpr", "op": "+",
							"left": {"kind": "Ident", "name": "a"},
							"right": {"kind": "Literal", "valueType": "Int", "value": 1}}
					}]
				},
				{
					"kind": "ReturnStmt"
				}
			]
		}]
	}`

	node, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	prog := node.(*Program)
	fd := prog.Decls[0].(*FunctionDecl)
	if len(fd.Body) != 5 {
		t.Fatalf("expected 5 body stmts, got %d", len(fd.Body))
	}

	if _, ok := fd.Body[0].(*VarDecl); !ok {
		t.Error("body[0] should be VarDecl")
	}
	if _, ok := fd.Body[1].(*AssignStmt); !ok {
		t.Error("body[1] should be AssignStmt")
	}
	if _, ok := fd.Body[2].(*IfStmt); !ok {
		t.Error("body[2] should be IfStmt")
	}
	if _, ok := fd.Body[3].(*WhileStmt); !ok {
		t.Error("body[3] should be WhileStmt")
	}
	if _, ok := fd.Body[4].(*ReturnStmt); !ok {
		t.Error("body[4] should be ReturnStmt")
	}
}

func TestParseTypeExprComplex(t *testing.T) {
	input := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "f",
			"params": [
				{"name": "arr", "type": {"kind": "Array", "elem": {"kind": "Int"}}},
				{"name": "opt", "type": {"kind": "Option", "elem": {"kind": "String"}}},
				{"name": "res", "type": {"kind": "Result", "okType": {"kind": "Int"}, "errType": {"kind": "String"}}}
			],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": []
		}]
	}`

	node, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	fd := node.(*Program).Decls[0].(*FunctionDecl)

	if fd.Params[0].Type.KindName != "Array" || fd.Params[0].Type.Elem.KindName != "Int" {
		t.Errorf("expected Array<Int>, got %+v", fd.Params[0].Type)
	}
	if fd.Params[1].Type.KindName != "Option" || fd.Params[1].Type.Elem.KindName != "String" {
		t.Errorf("expected Option<String>, got %+v", fd.Params[1].Type)
	}
	if fd.Params[2].Type.KindName != "Result" || fd.Params[2].Type.OkType.KindName != "Int" {
		t.Errorf("expected Result<Int, String>, got %+v", fd.Params[2].Type)
	}
}

func TestParseUnaryAndMember(t *testing.T) {
	input := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "f",
			"params": [{"name": "obj", "type": {"kind": "String"}}],
			"returnType": {"kind": "Bool"},
			"effects": ["pure"],
			"grant": [],
			"body": [{
				"kind": "ReturnStmt",
				"value": {
					"kind": "UnaryExpr", "op": "!",
					"operand": {
						"kind": "MemberExpr",
						"object": {"kind": "Ident", "name": "obj"},
						"field": "isEmpty"
					}
				}
			}]
		}]
	}`

	node, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	fd := node.(*Program).Decls[0].(*FunctionDecl)
	ret := fd.Body[0].(*ReturnStmt)
	unary := ret.Value.(*UnaryExpr)
	if unary.Op != "!" {
		t.Errorf("expected '!', got %q", unary.Op)
	}
	member := unary.Operand.(*MemberExpr)
	if member.Field != "isEmpty" {
		t.Errorf("expected 'isEmpty', got %q", member.Field)
	}
}

// TestParseStripsUTF8BOM guards against a regression to the byte-order mark
// that PowerShell's `-Encoding utf8` and other Windows tools prepend to text
// files: encoding/json treats a leading BOM as invalid input rather than
// stripping it, which used to reject an otherwise well-formed .xql.json file
// with a misleading "invalid character 'ï'" error.
func TestParseStripsUTF8BOM(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	input := append(bom, []byte(`{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": []
		}]
	}`)...)

	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed on BOM-prefixed input: %v", err)
	}
	prog, ok := node.(*Program)
	if !ok || len(prog.Decls) != 1 {
		t.Fatalf("expected a single-decl Program, got %#v", node)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseMissingKind(t *testing.T) {
	_, err := Parse([]byte(`{"name": "test"}`))
	if err == nil {
		t.Fatal("expected error for missing kind")
	}
}

func TestHashContent(t *testing.T) {
	data := []byte(`{"kind":"Program"}`)
	h1 := HashContent(data)
	h2 := HashContent(data)
	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if len(h1) != 64 {
		t.Errorf("SHA256 hex should be 64 chars, got %d", len(h1))
	}

	h3 := HashContent([]byte(`{"kind":"Other"}`))
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
}
