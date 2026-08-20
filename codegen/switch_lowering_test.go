package codegen

import (
	"os"
	"strings"
	"testing"

	"xiaoqinli/ast"
)

// The corpus program is examples/switch_stmt.xql.json and the conformance suite
// runs it. These tests ask the three things running it cannot: that the rewrite
// leaves the caller's AST alone, that the default arm ends up last whatever
// order it was written in, and that a switch reaches no backend which has no
// word for one.

func loadSwitchExample(t *testing.T) ast.Node {
	t.Helper()
	data, err := os.ReadFile("../examples/switch_stmt.xql.json")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	root, err := ast.Parse(data)
	if err != nil {
		t.Fatalf("parse example: %v", err)
	}
	return root
}

// Generate is called once per target over one parsed AST — the conformance
// suite parses a file and compiles it to all thirty-eight. A rewrite that
// mutated the tree would leave every target after the first compiling a
// program the earlier one had already lowered, and the native-switch backends
// would quietly stop emitting switches.
func TestLoweringDoesNotMutateCallerAST(t *testing.T) {
	root := loadSwitchExample(t)

	before := 0
	walkNodes(root, func(n ast.Node) {
		if _, ok := n.(*ast.SwitchStmt); ok {
			before++
		}
	})
	if before == 0 {
		t.Fatal("the example is supposed to contain a switch")
	}

	if _, err := Generate(root, "lua"); err != nil {
		t.Fatalf("lua: %v", err)
	}

	after := 0
	walkNodes(root, func(n ast.Node) {
		if _, ok := n.(*ast.SwitchStmt); ok {
			after++
		}
	})
	if after != before {
		t.Fatalf("compiling to a lowered target rewrote the caller's AST: %d switches before, %d after", before, after)
	}

	// And the target that has one still emits it, which is the failure the
	// count above would not catch on its own.
	code, err := Generate(root, "go")
	if err != nil {
		t.Fatalf("go: %v", err)
	}
	if !strings.Contains(string(code), "switch n {") {
		t.Errorf("go stopped emitting a native switch:\n%s", code)
	}
}

// Rust, OCaml and Haskell all read match arms in order, so a wildcard written
// anywhere but last makes every arm after it dead code. A switch says nothing
// about where its default is written.
func TestDefaultCaseBecomesTheLastArm(t *testing.T) {
	ss := &ast.SwitchStmt{
		Value: &ast.Ident{Name: "n"},
		Cases: []ast.SwitchCase{
			{Body: []ast.Node{&ast.ReturnStmt{Value: &ast.Literal{ValueType: "Int", Value: int64(0)}}}},
			{Value: &ast.Literal{ValueType: "Int", Value: int64(1)},
				Body: []ast.Node{&ast.ReturnStmt{Value: &ast.Literal{ValueType: "Int", Value: int64(10)}}}},
		},
	}

	me := switchToMatch(ss)
	if len(me.Arms) != 2 {
		t.Fatalf("two cases became %d arms", len(me.Arms))
	}
	if lit, ok := me.Arms[0].Pattern.(*ast.Literal); !ok || lit.Value != int64(1) {
		t.Errorf("first arm is %#v, want the case for 1", me.Arms[0].Pattern)
	}
	id, ok := me.Arms[1].Pattern.(*ast.Ident)
	if !ok || id.Name != "_" {
		t.Errorf("last arm is %#v, want the wildcard the default became", me.Arms[1].Pattern)
	}
}

// A switch with no default is a switch that may match nothing, and lowering it
// must not invent an arm that runs when it does.
func TestSwitchWithoutDefaultGainsNoWildcard(t *testing.T) {
	ss := &ast.SwitchStmt{
		Value: &ast.Ident{Name: "n"},
		Cases: []ast.SwitchCase{
			{Value: &ast.Literal{ValueType: "Int", Value: int64(1)}},
		},
	}
	me := switchToMatch(ss)
	if len(me.Arms) != 1 {
		t.Fatalf("one case became %d arms", len(me.Arms))
	}
	if id, ok := me.Arms[0].Pattern.(*ast.Ident); ok && id.Name == "_" {
		t.Error("a switch with no default was given a wildcard arm")
	}
}

// Twenty-five backends used to refuse this program: five at their statement
// emitter's default arm, twenty at validateNodesForTarget. The two that still
// refuse decline the match rather than the switch, and decline it for
// match_arms.xql.json too.
func TestEveryTargetCompilesASwitch(t *testing.T) {
	root := loadSwitchExample(t)

	declines := map[string]bool{"haskell": true, "tccli": true}

	for _, target := range []string{
		"go", "rust", "ts", "js", "py", "cpp", "c", "java", "csharp", "kotlin",
		"swift", "haskell", "dart", "lua", "ruby", "php", "zig", "nim", "julia",
		"awk", "bash", "crystal", "d", "fortran", "pascal", "perl", "powershell",
		"tcl", "ocaml", "elixir", "vala", "groovy", "bat", "shortcut", "chrome",
		"tccli", "android", "ios",
	} {
		code, err := Generate(root, target)
		if declines[target] {
			if err == nil {
				t.Errorf("%s was expected to decline the match this lowers to, and compiled it", target)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", target, err)
			continue
		}
		if len(code) == 0 {
			t.Errorf("%s compiled the switch to nothing", target)
		}
	}
}
