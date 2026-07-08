package codegen

import (
	"fmt"

	"xiaoqinli/ast"
)

// Generate dispatches code generation to the appropriate backend by target name.
func Generate(root ast.Node, target string) ([]byte, error) {
	if err := validateNodesForTarget(root, target); err != nil {
		return nil, err
	}
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
	scanMutables(stmts, muts, nil)
	return muts
}

func scanMutables(stmts []ast.Node, muts map[string]bool, localVars map[string]bool) {
	for _, s := range stmts {
		switch n := s.(type) {
		case *ast.AssignStmt:
			if ident, ok := n.Target.(*ast.Ident); ok {
				if localVars == nil || !localVars[ident.Name] {
					muts[ident.Name] = true
				}
			}
			scanExpr(n.Value, muts, localVars)
		case *ast.IfStmt:
			scanMutables(n.Then, muts, localVars)
			scanMutables(n.Else, muts, localVars)
		case *ast.WhileStmt:
			scanMutables(n.Body, muts, localVars)
		case *ast.ForStmt:
			if localVars != nil {
				forLocals := forkLocalVars(localVars)
				if n.Var != "" {
					forLocals[n.Var] = true
				}
				scanMutables(n.Body, muts, forLocals)
			} else {
				scanMutables(n.Body, muts, nil)
			}
		case *ast.VarDecl:
			if localVars != nil {
				localVars[n.Name] = true
			}
			scanExpr(n.Value, muts, localVars)
		case *ast.ExprStmt:
			scanExpr(n.Expr, muts, localVars)
		case *ast.SwitchStmt:
			for _, c := range n.Cases {
				scanMutables(c.Body, muts, localVars)
			}
		}
	}
}

func scanExpr(expr ast.Node, muts map[string]bool, localVars map[string]bool) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.Lambda:
		lambdaLocals := make(map[string]bool)
		for _, p := range e.Params {
			lambdaLocals[p.Name] = true
		}
		scanMutables(e.Body, muts, lambdaLocals)
	case *ast.CallExpr:
		for _, arg := range e.Args {
			scanExpr(arg, muts, localVars)
		}
	case *ast.BinaryExpr:
		scanExpr(e.Left, muts, localVars)
		scanExpr(e.Right, muts, localVars)
	case *ast.UnaryExpr:
		scanExpr(e.Operand, muts, localVars)
	case *ast.NewExpr:
		for _, arg := range e.Args {
			scanExpr(arg, muts, localVars)
		}
	case *ast.IfExpr:
		scanExpr(e.Cond, muts, localVars)
		scanExpr(e.Then, muts, localVars)
		scanExpr(e.Else, muts, localVars)
	case *ast.ArrayLit:
		for _, elem := range e.Elements {
			scanExpr(elem, muts, localVars)
		}
	case *ast.MatchExpr:
		for _, arm := range e.Arms {
			scanMutables(arm.Body, muts, localVars)
		}
	}
}

func forkLocalVars(parent map[string]bool) map[string]bool {
	child := make(map[string]bool, len(parent))
	for k, v := range parent {
		child[k] = v
	}
	return child
}

// unsupportedResultTargets lists targets that silently map Result<T> to just T,
// losing error-handling semantics. These should reject Result types explicitly.
var unsupportedResultTargets = map[string]bool{
	"js":         true,
	"javascript": true,
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
	case *ast.ClassDecl:
		for _, f := range node.Fields {
			fn(f.Type, fmt.Sprintf("field '%s' of class '%s'", f.Name, node.Name))
		}
	case *ast.SwitchStmt:
		for _, c := range node.Cases {
			for _, s := range c.Body {
				walkTypes(s, fn)
			}
		}
	case *ast.MapLiteral:
		fn(node.KeyType, "map key literal type")
		fn(node.ValueType, "map value literal type")
	case *ast.ArrayLiteral:
		fn(node.ElemType, "array literal element type")
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

func validateNodesForTarget(root ast.Node, target string) error {
	if target == "go" || target == "rust" || target == "ts" || target == "js" || target == "javascript" || target == "py" || target == "java" || target == "csharp" || target == "kotlin" || target == "swift" || target == "dart" || target == "zig" {
		return nil
	}
	var err error
	walkNodes(root, func(n ast.Node) {
		if err != nil {
			return
		}
		if n == nil {
			return
		}
		switch n.(type) {
		case *ast.ClassDecl, *ast.SwitchStmt, *ast.MapLiteral, *ast.ArrayLiteral, *ast.ImportDecl:
			err = fmt.Errorf("XQL_E401: target %q does not implement node kind %s", target, n.Kind())
		}
	})
	return err
}

func walkNodes(n ast.Node, fn func(ast.Node)) {
	if n == nil {
		return
	}
	fn(n)
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			walkNodes(d, fn)
		}
	case *ast.FunctionDecl:
		for _, s := range node.Body {
			walkNodes(s, fn)
		}
	case *ast.ReturnStmt:
		walkNodes(node.Value, fn)
	case *ast.VarDecl:
		walkNodes(node.Value, fn)
	case *ast.AssignStmt:
		walkNodes(node.Target, fn)
		walkNodes(node.Value, fn)
	case *ast.IfStmt:
		walkNodes(node.Cond, fn)
		for _, s := range node.Then {
			walkNodes(s, fn)
		}
		for _, s := range node.Else {
			walkNodes(s, fn)
		}
	case *ast.WhileStmt:
		walkNodes(node.Cond, fn)
		for _, s := range node.Body {
			walkNodes(s, fn)
		}
	case *ast.ForStmt:
		walkNodes(node.Start, fn)
		walkNodes(node.End, fn)
		walkNodes(node.Iterable, fn)
		for _, s := range node.Body {
			walkNodes(s, fn)
		}
	case *ast.StructDecl:
		// no children fields needed for node traversal
	case *ast.ClassDecl:
		// no children fields needed for node traversal
	case *ast.MatchExpr:
		walkNodes(node.Value, fn)
		for _, arm := range node.Arms {
			walkNodes(arm.Pattern, fn)
			for _, s := range arm.Body {
				walkNodes(s, fn)
			}
		}
	case *ast.SwitchStmt:
		walkNodes(node.Value, fn)
		for _, c := range node.Cases {
			walkNodes(c.Value, fn)
			for _, s := range c.Body {
				walkNodes(s, fn)
			}
		}
	case *ast.BinaryExpr:
		walkNodes(node.Left, fn)
		walkNodes(node.Right, fn)
	case *ast.UnaryExpr:
		walkNodes(node.Operand, fn)
	case *ast.CallExpr:
		for _, arg := range node.Args {
			walkNodes(arg, fn)
		}
	case *ast.MemberExpr:
		walkNodes(node.Object, fn)
	case *ast.StructLit:
		for _, f := range node.Fields {
			walkNodes(f.Value, fn)
		}
	case *ast.ArrayLit:
		for _, elem := range node.Elements {
			walkNodes(elem, fn)
		}
	case *ast.ArrayLiteral:
		for _, elem := range node.Elements {
			walkNodes(elem, fn)
		}
	case *ast.MapLiteral:
		for _, entry := range node.Entries {
			walkNodes(entry.Key, fn)
			walkNodes(entry.Value, fn)
		}
	case *ast.IndexExpr:
		walkNodes(node.Target, fn)
		walkNodes(node.Index, fn)
	case *ast.IfExpr:
		walkNodes(node.Cond, fn)
		walkNodes(node.Then, fn)
		walkNodes(node.Else, fn)
	case *ast.NewExpr:
		for _, arg := range node.Args {
			walkNodes(arg, fn)
		}
	case *ast.AwaitExpr:
		walkNodes(node.Expr, fn)
	case *ast.Lambda:
		for _, s := range node.Body {
			walkNodes(s, fn)
		}
	}
}

// CollectImports returns a map containing all the aliases imported in the program.
func CollectImports(root ast.Node) map[string]bool {
	imports := make(map[string]bool)
	prog, ok := root.(*ast.Program)
	if !ok {
		return imports
	}
	for _, d := range prog.Decls {
		if id, ok := d.(*ast.ImportDecl); ok {
			imports[id.As] = true
		}
	}
	return imports
}
