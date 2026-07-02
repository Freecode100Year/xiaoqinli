package codegen

import (
	"fmt"

	"xiaoqinli/ast"
)

// Generate dispatches code generation to the appropriate backend by target name.
func Generate(root ast.Node, target string) ([]byte, error) {
	if err := validateTypesForTarget(root, target); err != nil {
		return nil, err
	}
	switch target {
	case "go":
		return GenerateGo(root)
	case "rust":
		return GenerateRust(root)
	case "ts":
		return GenerateTypeScript(root)
	case "js", "javascript":
		return GenerateJavaScript(root)
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
	case "dart":
		return GenerateDart(root)
	case "lua":
		return GenerateLua(root)
	case "ruby":
		return GenerateRuby(root)
	case "php":
		return GeneratePHP(root)
	case "zig":
		return GenerateZig(root)
	case "nim":
		return GenerateNim(root)
	case "julia":
		return GenerateJulia(root)
	case "cpp":
		return GenerateCpp(root)
	case "mql4":
		return GenerateMQL4(root)
	case "mql5":
		return GenerateMQL5(root)
	case "c":
		return GenerateC(root)
	case "scala":
		return GenerateScala(root)
	case "haskell":
		return GenerateHaskell(root)
	case "ocaml":
		return GenerateOCaml(root)
	case "fsharp":
		return GenerateFSharp(root)
	case "ada":
		return GenerateAda(root)
	case "awk":
		return GenerateAwk(root)
	case "bash":
		return GenerateBash(root)
	case "crystal":
		return GenerateCrystal(root)
	case "d":
		return GenerateD(root)
	case "fortran":
		return GenerateFortran(root)
	case "objc":
		return GenerateObjC(root)
	case "pascal":
		return GeneratePascal(root)
	case "perl":
		return GeneratePerl(root)
	case "powershell":
		return GeneratePowerShell(root)
	case "tcl":
		return GenerateTcl(root)
	case "v":
		return GenerateV(root)
	case "elixir":
		return GenerateElixir(root)
	case "clojure":
		return GenerateClojure(root)
	case "vala":
		return GenerateVala(root)
	case "groovy":
		return GenerateGroovy(root)
	case "bat":
		return GenerateBat(root)
	case "shortcut":
		return GenerateShortcut(root)
	case "chrome":
		return GenerateChrome(root)
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
			if ident, ok := n.Target.(*ast.Ident); ok {
				muts[ident.Name] = true
			}
		case *ast.IfStmt:
			scanMutables(n.Then, muts)
			scanMutables(n.Else, muts)
		case *ast.WhileStmt:
			scanMutables(n.Body, muts)
		case *ast.ForStmt:
			scanMutables(n.Body, muts)
		}
	}
}

// unsupportedResultTargets lists targets that silently map Result<T> to just T,
// losing error-handling semantics. These should reject Result types explicitly.
var unsupportedResultTargets = map[string]bool{
	"ts":         true,
	"js":         true,
	"javascript": true,
	"dart":       true,
	"nim":        true,
	"julia":      true,
	"lua":        true,
	"ruby":       true,
}

// validateTypesForTarget walks the AST and returns an error if any type is
// unsupported by the target backend.
func validateTypesForTarget(root ast.Node, target string) error {
	if !unsupportedResultTargets[target] {
		return nil
	}
	var err error
	walkTypes(root, func(t ast.TypeExpr, context string) {
		if err != nil {
			return
		}
		if t.KindName == "Result" {
			err = fmt.Errorf("XQL_E402: target %q does not support Result<T> type (used in %s)", target, context)
		}
	})
	return err
}

// walkTypes traverses the AST and calls fn for each TypeExpr found.
func walkTypes(n ast.Node, fn func(ast.TypeExpr, string)) {
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			walkTypes(d, fn)
		}
	case *ast.FunctionDecl:
		for _, p := range node.Params {
			fn(p.Type, fmt.Sprintf("param '%s' of function '%s'", p.Name, node.Name))
		}
		fn(node.ReturnType, fmt.Sprintf("return type of function '%s'", node.Name))
		for _, s := range node.Body {
			walkTypes(s, fn)
		}
	case *ast.VarDecl:
		fn(node.Type, fmt.Sprintf("variable '%s'", node.Name))
	case *ast.IfStmt:
		for _, s := range node.Then {
			walkTypes(s, fn)
		}
		for _, s := range node.Else {
			walkTypes(s, fn)
		}
	case *ast.WhileStmt:
		for _, s := range node.Body {
			walkTypes(s, fn)
		}
	case *ast.ForStmt:
		for _, s := range node.Body {
			walkTypes(s, fn)
		}
	case *ast.StructDecl:
		for _, f := range node.Fields {
			fn(f.Type, fmt.Sprintf("field '%s' of struct '%s'", f.Name, node.Name))
		}
	case *ast.MatchExpr:
		for _, arm := range node.Arms {
			for _, s := range arm.Body {
				walkTypes(s, fn)
			}
		}
	case *ast.IfExpr:
		walkTypes(node.Cond, fn)
		walkTypes(node.Then, fn)
		walkTypes(node.Else, fn)
	case *ast.Lambda:
		for _, p := range node.Params {
			fn(p.Type, fmt.Sprintf("lambda param '%s'", p.Name))
		}
		fn(node.ReturnType, "lambda return type")
		for _, s := range node.Body {
			walkTypes(s, fn)
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
