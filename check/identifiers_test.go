package check

import (
	"strings"
	"testing"

	"xiaoqinli/ast"
)

func mainWith(t *testing.T, body string) ast.Node {
	t.Helper()
	src := `{"kind":"Program","declarations":[{"kind":"FunctionDecl","name":"main","params":[],
		"returnType":{"kind":"Void"},"effects":["pure"],"grant":[],"body":[` + body + `]}]}`
	n, err := ast.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return n
}

// Each of these passed every check while pure and ungranted, and compiled to
// code that ran a command.
func TestNamesAndOperatorsCannotCarryCode(t *testing.T) {
	cases := map[string]string{
		"var name": `{"kind":"VarDecl","name":"x = __import__('os').system('id')\n    y","type":{"kind":"Int"},
			"value":{"kind":"Literal","valueType":"Int","value":1}}`,
		"operator": `{"kind":"VarDecl","name":"x","type":{"kind":"Int"},"value":{"kind":"BinaryExpr","op":"+ __import__('os').system('id') +",
			"left":{"kind":"Literal","valueType":"Int","value":1},"right":{"kind":"Literal","valueType":"Int","value":2}}}`,
		"literal type": `{"kind":"VarDecl","name":"x","type":{"kind":"Int"},
			"value":{"kind":"Literal","valueType":"Int","value":"0; rm -rf ~"}}`,
		"callee": `{"kind":"ExprStmt","expr":{"kind":"CallExpr","callee":"print(1); os.system","args":[]}}`,
	}
	for name, body := range cases {
		errs := CheckIdentifiers(mainWith(t, body))
		if len(errs) == 0 {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestOrdinaryNamesPass(t *testing.T) {
	body := `{"kind":"VarDecl","name":"_count2","type":{"kind":"Int"},"value":{"kind":"Literal","valueType":"Int","value":1}},
		{"kind":"ExprStmt","expr":{"kind":"CallExpr","callee":"document.body.appendChild","args":[]}}`
	if errs := CheckIdentifiers(mainWith(t, body)); len(errs) != 0 {
		t.Fatalf("rejected: %s", strings.Join(errs, "; "))
	}
}
