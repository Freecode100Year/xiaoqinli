package codegen

import (
	"strings"
	"testing"
)

// The file is match_arms_test.go and not match_arm_test.go because `arm` is a
// GOARCH: go treats a file ending in _arm.go as built only on that
// architecture, so the singular name compiles into nothing on every machine
// anyone runs the suite on, silently and without an error to read.
//
// A match whose arms assign to a variable declared before it. examples/
// match_arms.xql.json is the same shape, and the conformance suite runs it
// wherever a toolchain exists; this file is for the backends that suite skips
// on a developer machine, where the only evidence available is the text.
//
// The node had never appeared in the corpus. Thirty-five backends translate it,
// and the count of those that had ever compiled one was zero.
const matchArmAssign = `{
	"kind": "Program",
	"declarations": [
		{
			"kind": "FunctionDecl",
			"name": "classify",
			"params": [{"name": "n", "type": {"kind": "Int"}}],
			"returnType": {"kind": "String"},
			"effects": [],
			"body": [
				{
					"kind": "VarDecl",
					"name": "tag",
					"type": {"kind": "String"},
					"value": {"kind": "Literal", "valueType": "String", "value": "none"}
				},
				{
					"kind": "MatchExpr",
					"value": {"kind": "Ident", "name": "n"},
					"arms": [
						{
							"pattern": {"kind": "Literal", "valueType": "Int", "value": 1},
							"body": [{
								"kind": "AssignStmt",
								"target": "tag",
								"value": {"kind": "Literal", "valueType": "String", "value": "one"}
							}]
						},
						{
							"pattern": {"kind": "Ident", "name": "_"},
							"body": [{
								"kind": "AssignStmt",
								"target": "tag",
								"value": {"kind": "Literal", "valueType": "String", "value": "many"}
							}]
						}
					]
				},
				{"kind": "ReturnStmt", "value": {"kind": "Ident", "name": "tag"}}
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
					"args": [{"kind": "CallExpr", "callee": "classify", "args": [{"kind": "Literal", "valueType": "Int", "value": 1}]}]
				}
			}]
		}
	]
}`

// collectMutables is what the immutable-by-default backends ask before choosing
// between `let` and `let mut`, `val` and `var`, `const` and `let`. It walks
// statements, and a MatchExpr is a statement in every backend that emits one —
// the name says expression, and scanExpr had always handled it, which is why
// nobody noticed the statement walk had no case for it.
//
// The result was uniform across the matrix: the variable an arm assigned to was
// declared immutable, and then assigned to. Nine backends, one missing case.
func TestMatchArmAssignmentDeclaredMutable(t *testing.T) {
	root := mustParse(t, matchArmAssign)

	cases := []struct {
		target string
		want   string
		reject string
	}{
		{"rust", "let mut tag", "let tag"},
		{"ts", "let tag", "const tag"},
		{"js", "let tag", "const tag"},
		{"java", "String tag =", "final String tag"},
		{"kotlin", "var tag", "val tag"},
		{"swift", "var tag", "let tag"},
		{"dart", "String tag =", "final tag"},
		{"zig", "var tag", "const tag"},
		{"nim", "var tag", "let tag"},
	}

	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			out, err := Generate(root, tc.target)
			if err != nil {
				t.Fatalf("%s declined a match it advertises support for: %v", tc.target, err)
			}
			src := string(out)
			if !strings.Contains(src, tc.want) {
				t.Errorf("%s should declare tag with %q:\n%s", tc.target, tc.want, src)
			}
			if strings.Contains(src, tc.reject) {
				t.Errorf("%s declared tag %q and then assigned to it in an arm:\n%s", tc.target, tc.reject, src)
			}
		})
	}
}

// Four backends whose match is not a switch, and each was wrong in its own way.
// None has a toolchain on the machine this was written on, so the assertions
// are on the shape of the text and the conformance suite is what runs them.
func TestMatchArmLowering(t *testing.T) {
	root := mustParse(t, matchArmAssign)

	cases := []struct {
		target string
		want   []string
		reject []string
		why    string
	}{
		{
			target: "bash",
			want:   []string{`case "${n}" in`},
			reject: []string{"case n in"},
			why:    "the subject of a case is a value; the bare name matches the literal text \"n\", which takes the wildcard every time",
		},
		{
			target: "bat",
			want:   []string{") else ("},
			reject: []string{"rem default"},
			why:    "the default arm needs its own else block, or it runs as the tail of the previous arm's",
		},
		{
			target: "elixir",
			want:   []string{"tag = case n do"},
			why:    "a case arm is a scope, so what it rebinds has to be bound back on the way out",
		},
		{
			target: "ocaml",
			want:   []string{"-> begin", "end"},
			why:    "every statement ends in `;`, and `;` before the next arm's `|` is a syntax error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			out, err := Generate(root, tc.target)
			if err != nil {
				t.Fatalf("%s declined the match: %v", tc.target, err)
			}
			src := string(out)
			for _, want := range tc.want {
				if !strings.Contains(src, want) {
					t.Errorf("%s should emit %q — %s:\n%s", tc.target, want, tc.why, src)
				}
			}
			for _, reject := range tc.reject {
				if strings.Contains(src, reject) {
					t.Errorf("%s still emits %q — %s:\n%s", tc.target, reject, tc.why, src)
				}
			}
		})
	}
}

// Assigning a literal to a String is the fifth place rust has to own a &str,
// and the first four were each found by a corpus program. This one has no
// match in it: the defect is in the assignment, and the match is only what
// finally put an assignment under a String declaration.
func TestRustAssignsOwnedString(t *testing.T) {
	const assignLiteral = `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": ["state"],
			"grant": ["io"],
			"body": [
				{
					"kind": "VarDecl",
					"name": "s",
					"type": {"kind": "String"},
					"value": {"kind": "Literal", "valueType": "String", "value": "a"}
				},
				{
					"kind": "AssignStmt",
					"target": "s",
					"value": {"kind": "Literal", "valueType": "String", "value": "b"}
				},
				{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "println", "args": [{"kind": "Ident", "name": "s"}]}
				}
			]
		}]
	}`

	out, err := Generate(mustParse(t, assignLiteral), "rust")
	if err != nil {
		t.Fatalf("rust declined a string assignment: %v", err)
	}
	if src := string(out); !strings.Contains(src, `s = "b".to_string()`) {
		t.Errorf("rust assigned a &str to a String, which rustc refuses:\n%s", src)
	}
}
