package codegen

import (
	"fmt"

	"xiaoqinli/ast"
)

// ProjectOutput holds multi-file project contents when target generates a full directory structure.
type ProjectOutput struct {
	MainCode []byte
	Files    map[string][]byte
}

// GenerateProject dispatches code generation to target, supporting both single-file and multi-file projects.
func GenerateProject(root ast.Node, target string) (*ProjectOutput, error) {
	root, err := prepareExterns(root, target)
	if err != nil {
		return nil, err
	}
	root = lowerSwitchForTarget(root, target)
	root = renameReservedForTarget(root, target)
	if target == "android" || target == "apk" {
		return GenerateAndroidProject(root)
	}
	if target == "ios" || target == "swift-pkg" {
		return GenerateIOSProject(root)
	}
	code, err := Generate(root, target)
	if err != nil {
		return nil, err
	}
	return &ProjectOutput{
		MainCode: code,
	}, nil
}

// Generate dispatches code generation to the appropriate backend by target name.
func Generate(root ast.Node, target string) ([]byte, error) {
	root, err := prepareExterns(root, target)
	if err != nil {
		return nil, err
	}
	root = lowerSwitchForTarget(root, target)
	root = renameReservedForTarget(root, target)
	if err := validateNodesForTarget(root, target); err != nil {
		return nil, err
	}
	if err := validateTypesForTarget(root, target); err != nil {
		return nil, err
	}
	switch target {
	case "ios", "swift-pkg":
		proj, err := GenerateIOSProject(root)
		if err != nil {
			return nil, err
		}
		return proj.MainCode, nil
	case "android", "apk":
		proj, err := GenerateAndroidProject(root)
		if err != nil {
			return nil, err
		}
		return proj.MainCode, nil
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
	case "c":
		return GenerateC(root)
	case "haskell":
		return GenerateHaskell(root)
	case "ocaml":
		return GenerateOCaml(root)
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
	case "pascal":
		return GeneratePascal(root)
	case "perl":
		return GeneratePerl(root)
	case "powershell":
		return GeneratePowerShell(root)
	case "tcl":
		return GenerateTcl(root)
	case "elixir":
		return GenerateElixir(root)
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
	case "tccli", "tencentcloud":
		return GenerateTCCLI(root)
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

// scanMutables walks a statement list. Every node that can hold a statement has
// to be listed here, and every node that can hold an expression has to hand that
// expression to scanExpr — a binding this misses is emitted `const`, `val`,
// `final` or `let` by the dozen-odd backends whose languages are immutable by
// default, and the assignment it missed then fails to compile.
//
// MatchExpr is the reason that paragraph is written down. It is named
// MatchExpr and scanExpr has always handled it, but every backend emits it in
// statement position — it arrives as an element of a body list, not inside one
// — so scanExpr never saw one and this switch had no case for it. Assignments
// in a match arm were therefore invisible: rust, ts, js, java, kotlin, swift,
// dart, scala, zig and nim all declared the target immutable and then assigned
// to it.
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
			scanExpr(n.Cond, muts, localVars)
			scanMutables(n.Then, muts, localVars)
			scanMutables(n.Else, muts, localVars)
		case *ast.WhileStmt:
			scanExpr(n.Cond, muts, localVars)
			scanMutables(n.Body, muts, localVars)
		case *ast.ForStmt:
			scanExpr(n.Start, muts, localVars)
			scanExpr(n.End, muts, localVars)
			scanExpr(n.Iterable, muts, localVars)
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
		case *ast.ReturnStmt:
			scanExpr(n.Value, muts, localVars)
		case *ast.ExprStmt:
			scanExpr(n.Expr, muts, localVars)
		case *ast.SwitchStmt:
			scanExpr(n.Value, muts, localVars)
			for _, c := range n.Cases {
				scanExpr(c.Value, muts, localVars)
				scanMutables(c.Body, muts, localVars)
			}
		case *ast.MatchExpr:
			scanExpr(n, muts, localVars)
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
		scanExpr(e.Value, muts, localVars)
		for _, arm := range e.Arms {
			scanMutables(arm.Body, muts, localVars)
		}
	}
}

// collectEnums indexes a program's enum declarations by name.
//
// Every backend that emits an EnumDecl chooses a spelling for its variants —
// Go prefixes them onto the type, C joins them with an underscore, Rust and C++
// scope them, Julia and OCaml put them in the surrounding namespace — and until
// examples/enum_match.xql.json nothing in the corpus ever referred to one. The
// reference side went through emitMemberExpr, which is written for field access
// on a value, so `Color.Red` came out as `Color.Red` in twenty-two targets
// whose declaration side had called it something else. See docs/adr_enum_ref.md.
func collectEnums(root ast.Node) map[string]*ast.EnumDecl {
	enums := map[string]*ast.EnumDecl{}
	prog, ok := root.(*ast.Program)
	if !ok {
		return enums
	}
	for _, d := range prog.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok && ed.Name != "" {
			enums[ed.Name] = ed
		}
	}
	return enums
}

// enumRef reports whether a MemberExpr names an enum variant rather than a
// field on a value: the object has to be a bare identifier that is a declared
// enum, and the field has to be one of its variants. A struct field that
// happens to share a name with a variant is unaffected, because its object is
// not an enum name.
func enumRef(enums map[string]*ast.EnumDecl, me *ast.MemberExpr) (string, string, bool) {
	if me == nil || len(enums) == 0 {
		return "", "", false
	}
	ident, ok := me.Object.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	ed, ok := enums[ident.Name]
	if !ok {
		return "", "", false
	}
	for _, v := range ed.Variants {
		if v == me.Field {
			return ed.Name, v, true
		}
	}
	return "", "", false
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
//
// lua, ruby and julia are not listed: all three now emit a real Result
// wrapper (a Lua table, a Ruby class, or a Julia struct with resultOk/
// resultErr/xqlUnwrap/xqlUnwrapErr helpers) instead of collapsing the type,
// with CallExpr/MemberExpr rewritten to match. Verified against real Lua
// 5.4.6, Ruby 3.3.12, and Julia 1.12.6 interpreters.
var unsupportedResultTargets = map[string]bool{
	"js":         true,
	"javascript": true,
	"nim":        true,

	// bash and perl were absent from this list, which read as support. Neither
	// backend emits a Result runtime or rewrites the call sites, so they passed
	// `Result.ok(x)` and `res.unwrap()` through verbatim: in bash a command
	// substitution calling a program that does not exist, in perl a bareword
	// that `use strict` refuses outright. Both produced output their own
	// interpreter rejects, and codegen reported success.
	"bash": true,
	"perl": true,

	// awk is the same story again, and the most clear-cut of the three:
	// `res.unwrap()` is not merely wrong there, it does not parse.
	"awk": true,

	// Sweeping every backend for the same shape — does the output reference
	// Result without defining one? — found it in eighteen more. Each emitted
	// `Result.ok(users)` and `res.unwrap()` exactly as the AST spells them,
	// with no runtime anywhere: a qualified name from a module that does not
	// exist in Haskell and F#, a call to a missing command in PowerShell and
	// Tcl, a reference to an undeclared symbol everywhere else.
	//
	// Only a handful of backends ever implemented Result. The rest inherited
	// the appearance of support from the dispatcher, because nothing checked.
	// A backend that cannot express a construct is supposed to reject it
	// rather than degrade it silently, so they do that now.
	"chrome":     true, // emits JavaScript, which is on this list already
	"crystal":    true,
	"d":          true,
	"elixir":     true,
	"groovy":     true,
	"haskell":    true,
	"ocaml":      true,
	"powershell": true,
	"shortcut":   true,
	"tccli":      true,
	"tcl":        true,
	"vala":       true,
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

// stringValued reports whether an expression produces a String. It supersedes
// containsStringExpr, which could only see literals — so `a + b` over two
// String *variables* looked like arithmetic to every backend that asked, and
// most of them ask in order to choose between concatenation and addition. perl
// and awk added the two strings numerically and printed 0, lua raised at run
// time, bat echoed the text "(a + b)", and fortran and rust would not compile
// at all. Every concatenation in the corpus had a literal on one side until
// string_vars.xql.json, which is why none of it was visible.
//
// The type table has been able to answer this the whole time. containsStringExpr
// stays as the fallback for a caller with no table and for sprintf, whose
// result is a String regardless of what its arguments are.
func stringValued(t *typeKinds, n ast.Node) bool {
	if t != nil && t.kindOf(n) == "String" {
		return true
	}
	return containsStringExpr(n)
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

// prepareExterns enforces each extern's target restriction and then removes the
// declarations from the tree.
//
// An extern is a promise about the host, not code: the backend emits the call
// verbatim and nothing else. Removing the declarations centrally means no
// backend has to know the node exists. Checking `targets` first turns the one
// way an extern can go wrong — compiling a browser API to a target whose host
// has never heard of it — into a compile error instead of output that only
// fails when someone runs it.
func prepareExterns(root ast.Node, target string) (ast.Node, error) {
	prog, ok := root.(*ast.Program)
	if !ok {
		return root, nil
	}
	kept := make([]ast.Node, 0, len(prog.Decls))
	stripped := false
	var restricted []*ast.ExternDecl
	for _, d := range prog.Decls {
		ed, isExtern := d.(*ast.ExternDecl)
		if !isExtern {
			kept = append(kept, d)
			continue
		}
		stripped = true
		if !externSupportsTarget(ed, target) {
			restricted = append(restricted, ed)
		}
	}
	// Only a call actually reaches the host, so an unused declaration for some
	// other platform is not this target's problem.
	if len(restricted) > 0 {
		called := calledNames(root)
		for _, ed := range restricted {
			if called[ed.Name] {
				return nil, fmt.Errorf(
					"XQL_E402: extern %q is declared only for targets %v and is not available in %q",
					ed.Name, ed.Targets, target)
			}
		}
	}
	if !stripped {
		return root, nil
	}
	return &ast.Program{Decls: kept}, nil
}

// calledNames returns every callee name invoked anywhere in the program.
func calledNames(root ast.Node) map[string]bool {
	out := make(map[string]bool)
	walkNodes(root, func(n ast.Node) {
		if call, ok := n.(*ast.CallExpr); ok {
			out[call.Callee] = true
		}
	})
	return out
}

func externSupportsTarget(ed *ast.ExternDecl, target string) bool {
	if len(ed.Targets) == 0 {
		return true
	}
	for _, t := range ed.Targets {
		if t == target || targetAlias(t) == targetAlias(target) {
			return true
		}
	}
	return false
}

// targetAlias folds the spellings Generate accepts for one backend so an extern
// declared for "js" is still available when compiling with "javascript".
func targetAlias(target string) string {
	switch target {
	case "javascript":
		return "js"
	case "apk":
		return "android"
	case "swift-pkg":
		return "ios"
	}
	return target
}

func validateNodesForTarget(root ast.Node, target string) error {
	if target == "go" || target == "rust" || target == "ts" || target == "js" || target == "javascript" || target == "py" || target == "java" || target == "csharp" || target == "kotlin" || target == "swift" || target == "dart" || target == "zig" || target == "nim" || target == "julia" || target == "php" || target == "ruby" || target == "lua" {
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
		// SwitchStmt is not in this list: lowerSwitchForTarget has already
		// rewritten it to a MatchExpr for every target that is not in the
		// early return above, so by here there is none left to refuse.
		case *ast.ClassDecl, *ast.MapLiteral, *ast.ArrayLiteral, *ast.ImportDecl:
			err = fmt.Errorf("XQL_E401: target %q does not implement node kind %s", target, n.Kind())
		}
	})
	return err
}

// identUsed reports whether a name is mentioned anywhere in a statement list.
//
// It decides whether a loop variable needs a name at all. A loop that repeats
// something a fixed number of times never mentions its index, and Swift and
// Elixir both warn about that — on stderr, where a warning is indistinguishable
// from the program's own output, and under -Werror where it is fatal.
// string_build.xql.json is such a loop, and it made both of them print their
// complaint alongside the right answer.
func identUsed(name string, body []ast.Node) bool {
	found := false
	for _, s := range body {
		walkNodes(s, func(n ast.Node) {
			if id, ok := n.(*ast.Ident); ok && id.Name == name {
				found = true
			}
		})
	}
	return found
}

// identCount counts how many times a name is mentioned in a subtree, and
// identCountIn does the same across a statement list. The difference between
// the two answers a question a single walk cannot: whether a name is used
// anywhere *outside* a particular node.
func identCount(name string, n ast.Node) int {
	c := 0
	walkNodes(n, func(x ast.Node) {
		if id, ok := x.(*ast.Ident); ok && id.Name == name {
			c++
		}
	})
	return c
}

func identCountIn(name string, nodes []ast.Node) int {
	total := 0
	for _, n := range nodes {
		total += identCount(name, n)
	}
	return total
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
	case *ast.ExprStmt:
		walkNodes(node.Expr, fn)
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

// loopBodyReturns reports whether a loop body contains a `return` — one that
// would have to leave the enclosing function from inside the loop.
//
// Three backends lower a loop to a form with no way out of it: haskell to
// mapM_, ocaml to a `for ... done` whose body has to be unit, elixir to a `for`
// comprehension. For those an early return is not a hard case, it is an
// impossible one, and all three used to emit something anyway.
// early_return.xql.json returns the first i whose square passes a limit, and
// haskell produced a lambda returning Int where mapM_ wants an action, ocaml a
// for body evaluating to int where it must be unit — neither compiles — while
// elixir compiled, threw the comprehension's result away, and printed the
// fall-through 0 for both calls.
//
// It does not descend into a Lambda: a return inside one belongs to the lambda,
// not to the function around the loop. That is also why the value positions
// below are followed by handing the value back to this same switch rather than
// by walking the whole expression: the switch matches statements and MatchExpr
// and nothing else, so a Lambda in value position falls through to false.
//
// A MatchExpr can sit in a body directly or wrapped in an ExprStmt, and for one
// release only the direct form was checked. The wrapped form is the same tree
// with one more node on it, and it walked straight past this guard: ocaml
// emitted `for ... do (match i with 3 -> limit | _ -> ...) done`, which is an
// int where the body must be unit and does not compile, and elixir compiled a
// case expression whose value the comprehension threw away and returned the
// fall-through.
func loopBodyReturns(body []ast.Node) bool {
	for _, s := range body {
		switch node := s.(type) {
		case *ast.ReturnStmt:
			return true
		case *ast.ExprStmt:
			if loopBodyReturns([]ast.Node{node.Expr}) {
				return true
			}
		case *ast.VarDecl:
			if node.Value != nil && loopBodyReturns([]ast.Node{node.Value}) {
				return true
			}
		case *ast.AssignStmt:
			if node.Value != nil && loopBodyReturns([]ast.Node{node.Value}) {
				return true
			}
		case *ast.IfStmt:
			if loopBodyReturns(node.Then) || loopBodyReturns(node.Else) {
				return true
			}
		case *ast.ForStmt:
			if loopBodyReturns(node.Body) {
				return true
			}
		case *ast.WhileStmt:
			if loopBodyReturns(node.Body) {
				return true
			}
		case *ast.MatchExpr:
			for _, arm := range node.Arms {
				if loopBodyReturns(arm.Body) {
					return true
				}
			}
		case *ast.SwitchStmt:
			for _, c := range node.Cases {
				if loopBodyReturns(c.Body) {
					return true
				}
			}
		}
	}
	return false
}
