package codegen

import (
	"fmt"

	"xiaoqinli/ast"
)

// Generate dispatches code generation to the appropriate backend by target name.
func Generate(root ast.Node, target string) ([]byte, error) {
	switch target {
	case "go":
		return GenerateGo(root)
	case "rust":
		return GenerateRust(root)
	case "ts":
		return GenerateTypeScript(root)
	case "kotlin":
		return GenerateKotlin(root)
	case "swift":
		return GenerateSwift(root)
	case "py":
		return GeneratePython(root)
	case "java":
		return GenerateJava(root)
	case "csharp":
		return GenerateCSharp(root)
	default:
		return nil, fmt.Errorf("unsupported target: %s", target)
	}
}

// collectMutables returns the set of variable names that are targets of
// AssignStmt within the given statement list (recursing into if/while).
// Used by languages with immutable-by-default bindings (Rust, TS, Kotlin, Swift).
func collectMutables(stmts []ast.Node) map[string]bool {
	muts := make(map[string]bool)
	scanMutables(stmts, muts)
	return muts
}

func scanMutables(stmts []ast.Node, muts map[string]bool) {
	for _, s := range stmts {
		switch n := s.(type) {
		case *ast.AssignStmt:
			muts[n.Target] = true
		case *ast.IfStmt:
			scanMutables(n.Then, muts)
			scanMutables(n.Else, muts)
		case *ast.WhileStmt:
			scanMutables(n.Body, muts)
		}
	}
}

func containsStringExpr(n ast.Node) bool {
	switch node := n.(type) {
	case *ast.Literal:
		return node.ValueType == "String"
	case *ast.BinaryExpr:
		if node.Op == "+" {
			return containsStringExpr(node.Left) || containsStringExpr(node.Right)
		}
	case *ast.CallExpr:
		if node.Callee == "sprintf" {
			return true
		}
	}
	return false
}
