package check

import (
	"strings"
	"testing"

	"xiaoqinli/ast"
)

func parseForTest(src string) (ast.Node, error) { return ast.Parse([]byte(src)) }

// externProgram builds a one-function program that calls the host through the
// given extern declarations. grant is main's capability list.
func externProgram(externs, grant, body string) string {
	return `{
		"kind": "Program",
		"declarations": [` + externs + `,
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": ["network"],
				"grant": [` + grant + `],
				"body": [` + body + `]
			}
		]
	}`
}

const fetchExtern = `{
	"kind": "ExternDecl",
	"name": "fetch",
	"params": [{"name": "url", "type": {"kind": "String"}}],
	"returnType": {"kind": "String"},
	"effects": ["network"],
	"grant": ["network"]
}`

const callFetch = `{
	"kind": "ExprStmt",
	"expr": {
		"kind": "CallExpr",
		"callee": "fetch",
		"args": [{"kind": "Literal", "valueType": "String", "value": "https://example.com"}]
	}
}`

// TestExternResolvesHostCall covers the reason ExternDecl exists: without a
// declaration, any call into the host is an undefined function and the program
// cannot be compiled at all.
func TestExternResolvesHostCall(t *testing.T) {
	root := mustParse(t, externProgram(fetchExtern, `"network"`, callFetch))
	if err := RunAllWithOptions(root, "", nil, CheckOptions{StrictCapabilities: true}); err != nil {
		t.Fatalf("expected extern-declared host call to check, got: %v", err)
	}
}

func TestUndeclaredHostCallStillRejected(t *testing.T) {
	root := mustParse(t, externProgram(fetchExtern, `"network"`, strings.Replace(callFetch, `"fetch"`, `"fetchTypo"`, 1)))
	err := RunAllWithOptions(root, "", nil, CheckOptions{})
	if err == nil {
		t.Fatal("expected an undeclared host call to be rejected")
	}
	if !strings.Contains(err.Error(), "undefined function") {
		t.Errorf("expected 'undefined function', got: %v", err)
	}
}

// TestExternGrantIsEnforced is the security property: a host call is the edge
// that reaches the outside world, so the caller must hold its grant.
func TestExternGrantIsEnforced(t *testing.T) {
	root := mustParse(t, externProgram(fetchExtern, ``, callFetch))
	err := RunAllWithOptions(root, "", nil, CheckOptions{})
	if err == nil {
		t.Fatal("expected a missing capability to be rejected")
	}
	if !strings.Contains(err.Error(), "XQL_E301") || !strings.Contains(err.Error(), "network") {
		t.Errorf("expected XQL_E301 naming the network capability, got: %v", err)
	}
}

func TestExternArityIsChecked(t *testing.T) {
	body := `{"kind": "ExprStmt", "expr": {"kind": "CallExpr", "callee": "fetch", "args": []}}`
	root := mustParse(t, externProgram(fetchExtern, `"network"`, body))
	err := RunAllWithOptions(root, "", nil, CheckOptions{})
	if err == nil || !strings.Contains(err.Error(), "expects 1 args") {
		t.Fatalf("expected an arity error, got: %v", err)
	}
}

// TestExternWithoutParamsSkipsArityCheck: omitting "params" declares a
// signature the compiler does not police, for host functions that are variadic
// or overloaded.
func TestExternWithoutParamsSkipsArityCheck(t *testing.T) {
	extern := `{
		"kind": "ExternDecl",
		"name": "fetch",
		"effects": ["network"],
		"grant": ["network"]
	}`
	body := `{"kind": "ExprStmt", "expr": {"kind": "CallExpr", "callee": "fetch", "args": [
		{"kind": "Literal", "valueType": "String", "value": "a"},
		{"kind": "Literal", "valueType": "String", "value": "b"}
	]}}`
	root := mustParse(t, externProgram(extern, `"network"`, body))
	if err := RunAllWithOptions(root, "", nil, CheckOptions{StrictCapabilities: true}); err != nil {
		t.Fatalf("expected an unchecked extern signature to accept any arity, got: %v", err)
	}
}

func TestExternReturnTypeFlows(t *testing.T) {
	body := `{
		"kind": "VarDecl",
		"name": "n",
		"type": {"kind": "Int"},
		"value": {"kind": "CallExpr", "callee": "fetch", "args": [{"kind": "Literal", "valueType": "String", "value": "u"}]}
	}`
	root := mustParse(t, externProgram(fetchExtern, `"network"`, body))
	err := RunAllWithOptions(root, "", nil, CheckOptions{})
	if err == nil || !strings.Contains(err.Error(), "declared Int but assigned String") {
		t.Fatalf("expected the extern's declared String return type to be used, got: %v", err)
	}
}

// TestDottedExternIsNotReadAsModule guards the resolution order: "time.Sleep"
// is one host name, not function Sleep in module time.
func TestDottedExternIsNotReadAsModule(t *testing.T) {
	extern := `{
		"kind": "ExternDecl",
		"name": "time.Sleep",
		"params": [{"name": "d", "type": {"kind": "Int"}}],
		"returnType": {"kind": "Void"},
		"effects": ["state"],
		"grant": ["clock"]
	}`
	body := `{"kind": "ExprStmt", "expr": {"kind": "CallExpr", "callee": "time.Sleep", "args": [
		{"kind": "Literal", "valueType": "Int", "value": 1}
	]}}`
	root := mustParse(t, externProgram(extern, `"clock"`, body))
	if err := RunAllWithOptions(root, "", nil, CheckOptions{StrictCapabilities: true}); err != nil {
		t.Fatalf("expected a dotted extern to resolve verbatim, got: %v", err)
	}
}

// TestExternMethodMatchesAnyReceiver covers host methods, where the receiver is
// a runtime value the compiler cannot type but the method still carries a grant.
func TestExternMethodMatchesAnyReceiver(t *testing.T) {
	extern := `{
		"kind": "ExternDecl",
		"name": "json",
		"effects": ["network"],
		"grant": ["network"],
		"method": true
	}`
	body := `{"kind": "ExprStmt", "expr": {"kind": "CallExpr", "callee": "res.json", "args": []}}`

	root := mustParse(t, externProgram(extern, `"network"`, body))
	if err := RunAllWithOptions(root, "", nil, CheckOptions{StrictCapabilities: true}); err != nil {
		t.Fatalf("expected an extern method to resolve on any receiver, got: %v", err)
	}

	root = mustParse(t, externProgram(extern, ``, body))
	err := RunAllWithOptions(root, "", nil, CheckOptions{})
	if err == nil || !strings.Contains(err.Error(), "XQL_E301") {
		t.Fatalf("expected an extern method's grant to be enforced, got: %v", err)
	}
}

func TestExternMethodNameMustNotBeQualified(t *testing.T) {
	src := `{"kind": "Program", "declarations": [{
		"kind": "ExternDecl", "name": "res.json", "method": true
	}]}`
	if _, err := parseForTest(src); err == nil {
		t.Fatal("expected a qualified extern method name to be rejected at parse time")
	}
}

func TestExternMustNotHaveBody(t *testing.T) {
	src := `{"kind": "Program", "declarations": [{
		"kind": "ExternDecl", "name": "fetch", "body": []
	}]}`
	if _, err := parseForTest(src); err == nil {
		t.Fatal("expected an ExternDecl with a body to be rejected at parse time")
	}
}

// TestExternCannotShadowFunction: a name is provided by the host or by this
// program, never both.
func TestExternCannotShadowFunction(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{"kind": "ExternDecl", "name": "helper", "effects": [], "grant": []},
			{
				"kind": "FunctionDecl", "name": "helper", "params": [],
				"returnType": {"kind": "Void"}, "effects": [], "grant": [], "body": []
			}
		]
	}`
	root := mustParse(t, src)
	err := RunAllWithOptions(root, "", nil, CheckOptions{})
	if err == nil || !strings.Contains(err.Error(), "also declared as a function") {
		t.Fatalf("expected an extern/function name clash to be rejected, got: %v", err)
	}
}

// TestExternEffectsReachTheCaller: a pure-declared function that calls a
// network extern is not pure.
func TestExternEffectsReachTheCaller(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [` + fetchExtern + `,
			{
				"kind": "FunctionDecl", "name": "main", "params": [],
				"returnType": {"kind": "Void"}, "effects": ["pure"], "grant": ["network"],
				"body": [` + callFetch + `]
			}
		]
	}`
	root := mustParse(t, src)
	err := RunAllWithOptions(root, "", nil, CheckOptions{})
	if err == nil || !strings.Contains(err.Error(), "XQL_E203") {
		t.Fatalf("expected the extern's declared effect to break purity, got: %v", err)
	}
}

// TestLambdaBoundNameIsCallable: a lambda stored in a variable and called by
// that name used to be reported as an undefined function.
func TestLambdaBoundNameIsCallable(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl", "name": "main", "params": [],
			"returnType": {"kind": "Void"}, "effects": [], "grant": [],
			"body": [
				{
					"kind": "VarDecl", "name": "double",
					"value": {
						"kind": "Lambda",
						"params": [{"name": "x", "type": {"kind": "Int"}}],
						"returnType": {"kind": "Int"},
						"body": [{"kind": "ReturnStmt", "value": {"kind": "Ident", "name": "x"}}]
					}
				},
				{
					"kind": "VarDecl", "name": "n", "type": {"kind": "Int"},
					"value": {"kind": "CallExpr", "callee": "double", "args": [
						{"kind": "Literal", "valueType": "Int", "value": 2}
					]}
				}
			]
		}]
	}`
	root := mustParse(t, src)
	if err := RunAllWithOptions(root, "", nil, CheckOptions{StrictCapabilities: true}); err != nil {
		t.Fatalf("expected a lambda-bound name to be callable, got: %v", err)
	}
}
