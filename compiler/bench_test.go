package compiler

import (
	"testing"
	"xiaoqinli/ast"
)

// buildBenchProgram synthesizes a Program with n functions, each containing a
// var decl, an if/else with a binary condition, and a println call, to give
// a realistic-ish mid-size workload for benchmarking the full pipeline and
// individual codegen backends. n=500 lands in the range of a genuinely large
// hand-written source file, not a toy snippet.
func buildBenchProgram(n int) *ast.Program {
	decls := make([]ast.Node, 0, n)
	for i := 0; i < n; i++ {
		fn := &ast.FunctionDecl{
			Name:       "fn" + itoa(i),
			Params:     []ast.Param{{Name: "x", Type: ast.TypeExpr{KindName: "Int"}}},
			ReturnType: ast.TypeExpr{KindName: "Int"},
			Effects:    []string{},
			Grant:      []string{},
			Body: []ast.Node{
				&ast.VarDecl{
					Name:  "acc",
					Type:  ast.TypeExpr{KindName: "Int"},
					Value: &ast.Literal{ValueType: "Int", Value: float64(i)},
				},
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						Op:    ">",
						Left:  &ast.Ident{Name: "x"},
						Right: &ast.Literal{ValueType: "Int", Value: float64(0)},
					},
					Then: []ast.Node{
						&ast.ExprStmt{Expr: &ast.CallExpr{Callee: "println", Args: []ast.Node{&ast.Ident{Name: "acc"}}}},
					},
					Else: []ast.Node{
						&ast.ExprStmt{Expr: &ast.CallExpr{Callee: "println", Args: []ast.Node{&ast.Ident{Name: "x"}}}},
					},
				},
				&ast.ReturnStmt{Value: &ast.Ident{Name: "acc"}},
			},
		}
		decls = append(decls, fn)
	}
	return &ast.Program{Decls: decls}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// BenchmarkCompile_500Fn measures end-to-end Compile() (validate + codegen)
// for a synthetic 500-function program across the main targets, to get a
// real baseline instead of an assumed one.
func BenchmarkCompile_500Fn(b *testing.B) {
	prog := buildBenchProgram(500)
	targets := []string{"go", "py", "rust", "ts"}
	for _, target := range targets {
		b.Run(target, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				res := Compile(CompileRequest{AST: prog, Target: target})
				if !res.Success {
					b.Fatalf("compile failed for %s: %s", target, res.Error)
				}
			}
		})
	}
}

// BenchmarkValidate_500Fn isolates the check/capability/effect pipeline cost
// (everything before codegen) on the same synthetic workload.
func BenchmarkValidate_500Fn(b *testing.B) {
	prog := buildBenchProgram(500)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		res := Validate(ValidateRequest{AST: prog})
		if !res.Success {
			b.Fatalf("validate failed: %s", res.Error)
		}
	}
}

// BenchmarkCompile_ScalingFn measures how Compile() scales with program size
// (50 / 500 / 5000 functions) for the Go target, to see whether cost grows
// linearly or worse.
func BenchmarkCompile_ScalingFn(b *testing.B) {
	sizes := []int{50, 500, 5000}
	for _, n := range sizes {
		prog := buildBenchProgram(n)
		b.Run(itoa(n)+"fn", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				res := Compile(CompileRequest{AST: prog, Target: "go"})
				if !res.Success {
					b.Fatalf("compile failed: %s", res.Error)
				}
			}
		})
	}
}
