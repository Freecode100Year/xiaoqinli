package codegen

import (
	"strings"
	"testing"
)

// A return inside a loop is declined by the backends whose loops run to the end
// — haskell's mapM_, ocaml's unit-bodied `for ... done`, elixir's Enum.reduce —
// and the corpus establishes that for the plain shape. What the corpus cannot
// reach is the same return one node deeper.
//
// loopBodyReturns is a pre-scan, and a pre-scan is only as good as the shapes it
// walks. It knew MatchExpr as a statement and not MatchExpr wrapped in an
// ExprStmt, which is the same tree with one more node on it and is what ocaml
// and elixir happily compiled: ocaml emitted `(match i with 3 -> limit | _ ->
// ...)` in a body that must be unit, and elixir emitted a case expression whose
// value the comprehension discarded before returning the fall-through.
const matchReturnInLoop = `{
	"kind": "Program",
	"declarations": [
		{
			"kind": "FunctionDecl",
			"name": "pick",
			"params": [{"name": "limit", "type": {"kind": "Int"}}],
			"returnType": {"kind": "Int"},
			"effects": [],
			"body": [
				{
					"kind": "ForStmt",
					"form": "range",
					"var": "i",
					"start": {"kind": "Literal", "valueType": "Int", "value": 0},
					"end": {"kind": "Literal", "valueType": "Int", "value": 5},
					"body": [{
						"kind": "ExprStmt",
						"expr": {
							"kind": "MatchExpr",
							"value": {"kind": "Ident", "name": "i"},
							"arms": [
								{
									"pattern": {"kind": "Literal", "valueType": "Int", "value": 3},
									"body": [{"kind": "ReturnStmt", "value": {"kind": "Ident", "name": "limit"}}]
								},
								{"pattern": {"kind": "Ident", "name": "_"}, "body": []}
							]
						}
					}]
				},
				{"kind": "ReturnStmt", "value": {"kind": "Literal", "valueType": "Int", "value": 0}}
			]
		},
		{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": ["state"],
			"grant": ["io"],
			"body": [{
				"kind": "ExprStmt",
				"expr": {
					"kind": "CallExpr",
					"callee": "println",
					"args": [{"kind": "CallExpr", "callee": "pick", "args": [{"kind": "Literal", "valueType": "Int", "value": 7}]}]
				}
			}]
		}
	]
}`

func TestLoopReturnDeclinedThroughExprStmt(t *testing.T) {
	root := mustParse(t, matchReturnInLoop)

	// ocaml and elixir are the two that read this shape and emitted something.
	// haskell declines the MatchExpr itself, one step earlier, which is also a
	// refusal — the test asks for XQL_E402 either way, because the distinction
	// this file cares about is capability limit versus silent mistranslation.
	for _, target := range []string{"haskell", "ocaml", "elixir", "shortcut"} {
		t.Run(target, func(t *testing.T) {
			out, err := Generate(root, target)
			if err == nil {
				t.Fatalf("%s compiled a return it cannot leave a loop with:\n%s", target, out)
			}
			if !strings.Contains(err.Error(), "XQL_E4") {
				t.Errorf("%s refused with %v; a capability limit is XQL_E402", target, err)
			}
		})
	}
}

// The each form is not the range form, and rust is where that mattered: it
// bound `for n in &nums`, so the element arrived as &i64. for_each.xql.json
// only ever added it, and `i64 + &i64` is one of the few things a reference
// does without being asked. Comparing it and returning it are not, and rustc
// refused both with "expected i64, found &i64".
const eachReturnProgram = `{
	"kind": "Program",
	"declarations": [
		{
			"kind": "FunctionDecl",
			"name": "firstBig",
			"params": [{"name": "limit", "type": {"kind": "Int"}}],
			"returnType": {"kind": "Int"},
			"effects": [],
			"body": [
				{
					"kind": "VarDecl",
					"name": "nums",
					"type": {"kind": "Array", "elem": {"kind": "Int"}},
					"value": {
						"kind": "ArrayLit",
						"elemType": {"kind": "Int"},
						"elements": [
							{"kind": "Literal", "valueType": "Int", "value": 3},
							{"kind": "Literal", "valueType": "Int", "value": 8}
						]
					}
				},
				{
					"kind": "ForStmt",
					"form": "each",
					"var": "n",
					"iterable": {"kind": "Ident", "name": "nums"},
					"body": [{
						"kind": "IfStmt",
						"cond": {
							"kind": "BinaryExpr",
							"op": ">",
							"left": {"kind": "Ident", "name": "n"},
							"right": {"kind": "Ident", "name": "limit"}
						},
						"then": [{"kind": "ReturnStmt", "value": {"kind": "Ident", "name": "n"}}],
						"else": []
					}]
				},
				{"kind": "ReturnStmt", "value": {"kind": "Literal", "valueType": "Int", "value": 0}}
			]
		}
	]
}`

func TestRustForEachBindsByValue(t *testing.T) {
	root := mustParse(t, eachReturnProgram)
	out, err := Generate(root, "rust")
	if err != nil {
		t.Fatalf("rust declined a for-each: %v", err)
	}
	code := string(out)
	if strings.Contains(code, "in &nums") {
		t.Errorf("rust binds the element by reference, which only addition survives:\n%s", code)
	}
	if !strings.Contains(code, "nums.iter().cloned()") {
		t.Errorf("rust for-each should bind by value with .iter().cloned():\n%s", code)
	}
}
