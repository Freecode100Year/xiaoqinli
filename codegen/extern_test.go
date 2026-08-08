package codegen

import (
	"strings"
	"testing"

	"xiaoqinli/ast"
)

// externCallProgram is a program that declares one host function and calls it.
func externCallProgram(targets []string) ast.Node {
	return &ast.Program{Decls: []ast.Node{
		&ast.ExternDecl{
			Name:       "hostPing",
			ReturnType: ast.TypeExpr{KindName: "Void"},
			Targets:    targets,
		},
		&ast.FunctionDecl{
			Name:       "main",
			ReturnType: ast.TypeExpr{KindName: "Void"},
			Body: []ast.Node{
				&ast.ExprStmt{Expr: &ast.CallExpr{Callee: "hostPing"}},
			},
		},
	}}
}

// TestExternIsNotEmitted: an extern is a promise about the host, so backends
// must emit the call and nothing else — no stub, no declaration.
func TestExternIsNotEmitted(t *testing.T) {
	for _, target := range []string{"go", "py", "rust", "js", "ruby"} {
		t.Run(target, func(t *testing.T) {
			code, err := Generate(externCallProgram(nil), target)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			out := string(code)
			if !strings.Contains(out, "hostPing") {
				t.Errorf("expected the host call to be emitted, got:\n%s", out)
			}
			if strings.Contains(out, "ExternDecl") || strings.Contains(out, "extern") {
				t.Errorf("expected no extern declaration in the output, got:\n%s", out)
			}
		})
	}
}

// TestExternTargetRestrictionIsEnforced: compiling a browser API to a host that
// has never heard of it is a compile error, not output that fails at runtime.
func TestExternTargetRestrictionIsEnforced(t *testing.T) {
	_, err := Generate(externCallProgram([]string{"js", "chrome"}), "go")
	if err == nil {
		t.Fatal("expected a target-restricted extern to be rejected")
	}
	if !strings.Contains(err.Error(), "XQL_E402") || !strings.Contains(err.Error(), "hostPing") {
		t.Errorf("expected XQL_E402 naming the extern, got: %v", err)
	}

	if _, err := Generate(externCallProgram([]string{"js", "chrome"}), "js"); err != nil {
		t.Errorf("expected a listed target to be accepted, got: %v", err)
	}
	// "javascript" is the same backend as "js" under a second name.
	if _, err := Generate(externCallProgram([]string{"js"}), "javascript"); err != nil {
		t.Errorf("expected the js alias to be accepted, got: %v", err)
	}
}

// TestUnusedExternDoesNotRestrictTarget: only a call reaches the host, so a
// declaration for some other platform is not this target's problem.
func TestUnusedExternDoesNotRestrictTarget(t *testing.T) {
	prog := &ast.Program{Decls: []ast.Node{
		&ast.ExternDecl{Name: "browserOnly", Targets: []string{"js"}},
		&ast.FunctionDecl{
			Name:       "main",
			ReturnType: ast.TypeExpr{KindName: "Void"},
			Body: []ast.Node{
				&ast.ExprStmt{Expr: &ast.CallExpr{
					Callee: "println",
					Args:   []ast.Node{&ast.Literal{ValueType: "String", Value: "hi"}},
				}},
			},
		},
	}}
	if _, err := Generate(prog, "go"); err != nil {
		t.Fatalf("expected an unused extern to be ignored, got: %v", err)
	}
}
