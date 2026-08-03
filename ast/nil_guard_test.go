package ast

import (
	"strings"
	"testing"
)

// wrapInMain embeds a statement JSON fragment into a minimal Program.
func wrapInMain(stmt string) string {
	return `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [` + stmt + `]
			}
		]
	}`
}

// TestParseIfExprAcceptsCondAlias guards the alias accepted by IfStmt and
// WhileStmt: IfExpr must accept "cond" as well as "condition".
func TestParseIfExprAcceptsCondAlias(t *testing.T) {
	for _, field := range []string{"cond", "condition"} {
		t.Run(field, func(t *testing.T) {
			stmt := `{
				"kind": "VarDecl",
				"name": "result",
				"type": {"kind": "String"},
				"value": {
					"kind": "IfExpr",
					"` + field + `": {
						"kind": "BinaryExpr",
						"op": ">",
						"left": {"kind": "Literal", "valueType": "Int", "value": 10},
						"right": {"kind": "Literal", "valueType": "Int", "value": 5}
					},
					"then": {"kind": "Literal", "valueType": "String", "value": "big"},
					"else": {"kind": "Literal", "valueType": "String", "value": "small"}
				}
			}`
			node, err := Parse([]byte(wrapInMain(stmt)))
			if err != nil {
				t.Fatalf("Parse failed for %q: %v", field, err)
			}
			prog := node.(*Program)
			fd := prog.Decls[0].(*FunctionDecl)
			vd, ok := fd.Body[0].(*VarDecl)
			if !ok {
				t.Fatalf("expected *VarDecl, got %T", fd.Body[0])
			}
			ie, ok := vd.Value.(*IfExpr)
			if !ok {
				t.Fatalf("expected *IfExpr, got %T", vd.Value)
			}
			if ie.Cond == nil || ie.Then == nil || ie.Else == nil {
				t.Fatalf("IfExpr has nil child: cond=%v then=%v else=%v", ie.Cond, ie.Then, ie.Else)
			}
		})
	}
}

// TestParseRejectsMissingRequiredChildren ensures a missing required child
// yields an XQL_E101 diagnostic rather than a nil node that panics in codegen.
func TestParseRejectsMissingRequiredChildren(t *testing.T) {
	cases := []struct {
		name string
		stmt string
		want string
	}{
		{"ExprStmt", `{"kind":"ExprStmt"}`, "ExprStmt missing 'expr'"},
		{"AssignStmtValue", `{"kind":"AssignStmt","target":{"kind":"Ident","name":"x"}}`, "AssignStmt missing 'value'"},
		{"SwitchStmt", `{"kind":"SwitchStmt","cases":[]}`, "SwitchStmt missing 'value'"},
		{"BinaryLeft", `{"kind":"ExprStmt","expr":{"kind":"BinaryExpr","op":"+","right":{"kind":"Literal","valueType":"Int","value":1}}}`, "BinaryExpr missing 'left'"},
		{"BinaryRight", `{"kind":"ExprStmt","expr":{"kind":"BinaryExpr","op":"+","left":{"kind":"Literal","valueType":"Int","value":1}}}`, "BinaryExpr missing 'right'"},
		{"UnaryOperand", `{"kind":"ExprStmt","expr":{"kind":"UnaryExpr","op":"-"}}`, "UnaryExpr missing 'operand'"},
		{"MemberObject", `{"kind":"ExprStmt","expr":{"kind":"MemberExpr","field":"f"}}`, "MemberExpr missing 'object'"},
		{"IndexTarget", `{"kind":"ExprStmt","expr":{"kind":"IndexExpr","index":{"kind":"Literal","valueType":"Int","value":0}}}`, "IndexExpr missing 'target'"},
		{"IndexIndex", `{"kind":"ExprStmt","expr":{"kind":"IndexExpr","target":{"kind":"Ident","name":"a"}}}`, "IndexExpr missing 'index'"},
		{"AwaitExpr", `{"kind":"ExprStmt","expr":{"kind":"AwaitExpr"}}`, "AwaitExpr missing 'expr'"},
		{"MatchExpr", `{"kind":"ExprStmt","expr":{"kind":"MatchExpr","arms":[]}}`, "MatchExpr missing 'value'"},
		{"IfExprCond", `{"kind":"ExprStmt","expr":{"kind":"IfExpr","then":{"kind":"Literal","valueType":"Int","value":1},"else":{"kind":"Literal","valueType":"Int","value":2}}}`, "IfExpr missing 'cond'"},
		{"IfExprThen", `{"kind":"ExprStmt","expr":{"kind":"IfExpr","cond":{"kind":"Literal","valueType":"Bool","value":true},"else":{"kind":"Literal","valueType":"Int","value":2}}}`, "IfExpr missing 'then'"},
		{"IfExprElse", `{"kind":"ExprStmt","expr":{"kind":"IfExpr","cond":{"kind":"Literal","valueType":"Bool","value":true},"then":{"kind":"Literal","valueType":"Int","value":1}}}`, "IfExpr missing 'else'"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(wrapInMain(tc.stmt)))
			if err == nil {
				t.Fatalf("expected parse error for %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), "XQL_E101") {
				t.Errorf("expected XQL_E101, got: %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error to mention %q, got: %v", tc.want, err)
			}
		})
	}
}
