package check

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// Capability represents a set of granted abilities for a function.
type Capability []string

// Contains checks whether the required capability set is a subset of the
// provided capabilities, supporting scoped capabilities like network:read.
func (c Capability) Contains(required Capability) error {
	for _, need := range required {
		matched := false
		for _, abil := range c {
			if matchCapability(abil, need) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("XQL_E301: missing required capability: %s", need)
		}
	}
	return nil
}

func matchCapability(haveAbil, needAbil string) bool {
	normalize := func(abil string) (string, string) {
		if !strings.Contains(abil, ":") {
			return abil, "*"
		}
		parts := strings.SplitN(abil, ":", 2)
		return parts[0], parts[1]
	}

	haveMod, haveScope := normalize(haveAbil)
	needMod, needScope := normalize(needAbil)

	if haveMod != needMod {
		return false
	}
	if haveScope == "*" {
		return true
	}
	return haveScope == needScope
}

// VerifyCapability ensures that the caller's capabilities are a superset
// of the callee's required capabilities.
func VerifyCapability(callerCap, requiredCap Capability) error {
	return callerCap.Contains(requiredCap)
}

// CheckCapabilities walks the AST and verifies capability inheritance:
// when function A calls function B, B's @grant must be a subset of A's @grant.
func CheckCapabilities(root ast.Node) error {
	return CheckCapabilitiesWithTC(root, NewTypeChecker())
}

func CheckCapabilitiesStrict(root ast.Node) error {
	return CheckCapabilitiesStrictWithTC(root, NewTypeChecker())
}

func CheckCapabilitiesStrictWithTC(root ast.Node, tc *TypeChecker) error {
	return CheckCapabilitiesWithOptions(root, tc, CheckOptions{StrictCapabilities: true})
}

func CheckCapabilitiesWithTC(root ast.Node, tc *TypeChecker) error {
	return CheckCapabilitiesWithOptions(root, tc, CheckOptions{StrictCapabilities: false})
}

func CheckCapabilitiesWithOptions(root ast.Node, tc *TypeChecker, opts CheckOptions) error {
	if root == nil {
		return fmt.Errorf("root node is nil")
	}
	// Collect all function declarations and their grants.
	funcGrants := make(map[string]Capability)
	collectGrants(root, funcGrants)

	var errs []string
	checkCaps(root, funcGrants, &errs, tc, opts)

	// Imported modules are checked too: a grant violation is a violation
	// wherever it lives, and without this it would only be caught when the
	// module happens to be compiled as an entry file.
	checkImportedCaps(tc, &errs, opts, map[*TypeChecker]bool{tc: true})

	for _, e := range errs {
		tc.addError(e)
	}

	if len(errs) > 0 {
		msg := "XQL_E302: capability check failed:\n"
		for _, e := range errs {
			msg += "  - " + e + "\n"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// checkImportedCaps walks the import graph and capability-checks each module
// against its own grants. visited keeps a diamond-shaped import graph from
// reporting the same module twice.
func checkImportedCaps(tc *TypeChecker, errs *[]string, opts CheckOptions, visited map[*TypeChecker]bool) {
	for _, depTC := range tc.imports {
		if visited[depTC] {
			continue
		}
		visited[depTC] = true
		if depTC.program != nil {
			depGrants := make(map[string]Capability)
			collectGrants(depTC.program, depGrants)
			checkCaps(depTC.program, depGrants, errs, depTC, opts)
		}
		checkImportedCaps(depTC, errs, opts, visited)
	}
}

func collectGrants(n ast.Node, out map[string]Capability) {
	if n == nil {
		return
	}
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			collectGrants(d, out)
		}
	case *ast.FunctionDecl:
		out[node.Name] = Capability(node.Grant)
	}
}

func checkCaps(n ast.Node, funcGrants map[string]Capability, errs *[]string, tc *TypeChecker, opts CheckOptions) {
	if n == nil {
		return
	}
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			checkCaps(d, funcGrants, errs, tc, opts)
		}
	case *ast.FunctionDecl:
		callerCap := Capability(node.Grant)
		// Lambdas bound to a local name are callable by that name. Their bodies
		// are walked below under this same function's grant, so calling one
		// needs no capability of its own — but it does need to resolve, or
		// strict mode would report it as unverifiable.
		scoped := funcGrants
		if locals := lambdaLocals(node.Body); len(locals) > 0 {
			scoped = make(map[string]Capability, len(funcGrants)+len(locals))
			for k, v := range funcGrants {
				scoped[k] = v
			}
			for _, name := range locals {
				scoped[name] = Capability{}
			}
		}
		for _, stmt := range node.Body {
			checkCapStmt(stmt, node.Name, callerCap, scoped, errs, tc, opts)
		}
	}
}

// lambdaLocals returns the names of variables in stmts that are bound to a
// lambda, including inside nested blocks.
func lambdaLocals(stmts []ast.Node) []string {
	var out []string
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		switch node := n.(type) {
		case *ast.VarDecl:
			if _, isLambda := node.Value.(*ast.Lambda); isLambda {
				out = append(out, node.Name)
			}
		case *ast.IfStmt:
			for _, s := range node.Then {
				walk(s)
			}
			for _, s := range node.Else {
				walk(s)
			}
		case *ast.WhileStmt:
			for _, s := range node.Body {
				walk(s)
			}
		case *ast.ForStmt:
			for _, s := range node.Body {
				walk(s)
			}
		case *ast.SwitchStmt:
			for _, c := range node.Cases {
				for _, s := range c.Body {
					walk(s)
				}
			}
		case *ast.MatchExpr:
			for _, arm := range node.Arms {
				for _, s := range arm.Body {
					walk(s)
				}
			}
		}
	}
	for _, s := range stmts {
		walk(s)
	}
	return out
}

func checkCapStmt(n ast.Node, callerName string, callerCap Capability, funcGrants map[string]Capability, errs *[]string, tc *TypeChecker, opts CheckOptions) {
	if n == nil {
		return
	}
	switch node := n.(type) {
	case *ast.ExprStmt:
		checkCapExpr(node.Expr, callerName, callerCap, funcGrants, errs, tc, opts)
	case *ast.ReturnStmt:
		if node.Value != nil {
			checkCapExpr(node.Value, callerName, callerCap, funcGrants, errs, tc, opts)
		}
	case *ast.VarDecl:
		if node.Value != nil {
			checkCapExpr(node.Value, callerName, callerCap, funcGrants, errs, tc, opts)
		}
	case *ast.AssignStmt:
		checkCapExpr(node.Target, callerName, callerCap, funcGrants, errs, tc, opts)
		checkCapExpr(node.Value, callerName, callerCap, funcGrants, errs, tc, opts)
	case *ast.IfStmt:
		checkCapExpr(node.Cond, callerName, callerCap, funcGrants, errs, tc, opts)
		for _, s := range node.Then {
			checkCapStmt(s, callerName, callerCap, funcGrants, errs, tc, opts)
		}
		for _, s := range node.Else {
			checkCapStmt(s, callerName, callerCap, funcGrants, errs, tc, opts)
		}
	case *ast.WhileStmt:
		checkCapExpr(node.Cond, callerName, callerCap, funcGrants, errs, tc, opts)
		for _, s := range node.Body {
			checkCapStmt(s, callerName, callerCap, funcGrants, errs, tc, opts)
		}
	case *ast.ForStmt:
		if node.Start != nil {
			checkCapExpr(node.Start, callerName, callerCap, funcGrants, errs, tc, opts)
		}
		if node.End != nil {
			checkCapExpr(node.End, callerName, callerCap, funcGrants, errs, tc, opts)
		}
		if node.Iterable != nil {
			checkCapExpr(node.Iterable, callerName, callerCap, funcGrants, errs, tc, opts)
		}
		for _, s := range node.Body {
			checkCapStmt(s, callerName, callerCap, funcGrants, errs, tc, opts)
		}
	case *ast.BreakStmt, *ast.ContinueStmt:
		// No capability checking needed.
	case *ast.MatchExpr:
		checkCapExpr(node.Value, callerName, callerCap, funcGrants, errs, tc, opts)
		for _, arm := range node.Arms {
			for _, s := range arm.Body {
				checkCapStmt(s, callerName, callerCap, funcGrants, errs, tc, opts)
			}
		}
	case *ast.SwitchStmt:
		checkCapExpr(node.Value, callerName, callerCap, funcGrants, errs, tc, opts)
		for _, c := range node.Cases {
			if c.Value != nil {
				checkCapExpr(c.Value, callerName, callerCap, funcGrants, errs, tc, opts)
			}
			for _, s := range c.Body {
				checkCapStmt(s, callerName, callerCap, funcGrants, errs, tc, opts)
			}
		}
	}
}

func checkCapExpr(n ast.Node, callerName string, callerCap Capability, funcGrants map[string]Capability, errs *[]string, tc *TypeChecker, opts CheckOptions) {
	if n == nil {
		return
	}
	switch node := n.(type) {
	case *ast.CallExpr:
		calleeName := node.Callee
		var calleeCap Capability
		found := false
		// Externs are matched verbatim and resolved first: they are the edge
		// that actually reaches the host, so their declared grant is the one
		// the caller must hold. A dotted name like "time.Sleep" would otherwise
		// be mistaken for a module-qualified call.
		if ed, ok := tc.externTable[calleeName]; ok {
			calleeCap = Capability(ed.Grant)
			found = true
		} else if strings.Contains(calleeName, ".") {
			parts := strings.Split(calleeName, ".")
			if len(parts) == 2 {
				alias := parts[0]
				funcName := parts[1]
				if depTC, ok := tc.imports[alias]; ok {
					if fd, ok := depTC.funcTable[funcName]; ok {
						calleeCap = Capability(fd.Grant)
						found = true
					}
				}
				// Result intrinsics are pure: they return the wrapped value or
				// panic, never performing I/O. The type checker already accepts
				// them on Result-typed receivers (see inferType), so treating
				// them as capability-free keeps both checks consistent. Imports
				// are resolved first, so a real imported function of the same
				// name still uses its declared grant.
				if !found && isResultIntrinsic(calleeName, funcName) {
					calleeCap = Capability{}
					found = true
				}
			}
		} else {
			if cap, ok := funcGrants[calleeName]; ok {
				calleeCap = cap
				found = true
			} else if _, ok := builtinFuncs[calleeName]; ok {
				calleeCap = Capability{}
				found = true
			}
		}
		if !found {
			// Last resort, mirroring the type checker: a qualified call nobody
			// else claims may be a declared host method.
			if ed := tc.methodExtern(calleeName); ed != nil {
				calleeCap = Capability(ed.Grant)
				found = true
			}
		}
		if found {
			if err := callerCap.Contains(calleeCap); err != nil {
				*errs = append(*errs, fmt.Sprintf(
					"function '%s' calls '%s' but lacks required capabilities: %v",
					callerName, calleeName, calleeCap))
			}
		} else if opts.StrictCapabilities {
			*errs = append(*errs, fmt.Sprintf(
				"XQL_E303: cannot verify capability for unresolved call '%s' in function '%s'",
				calleeName, callerName))
		}
		for _, arg := range node.Args {
			checkCapExpr(arg, callerName, callerCap, funcGrants, errs, tc, opts)
		}
	case *ast.BinaryExpr:
		checkCapExpr(node.Left, callerName, callerCap, funcGrants, errs, tc, opts)
		checkCapExpr(node.Right, callerName, callerCap, funcGrants, errs, tc, opts)
	case *ast.UnaryExpr:
		checkCapExpr(node.Operand, callerName, callerCap, funcGrants, errs, tc, opts)
	case *ast.MemberExpr:
		checkCapExpr(node.Object, callerName, callerCap, funcGrants, errs, tc, opts)
	case *ast.StructLit:
		for _, fi := range node.Fields {
			checkCapExpr(fi.Value, callerName, callerCap, funcGrants, errs, tc, opts)
		}
	case *ast.ArrayLit:
		for _, elem := range node.Elements {
			checkCapExpr(elem, callerName, callerCap, funcGrants, errs, tc, opts)
		}
	case *ast.IndexExpr:
		checkCapExpr(node.Target, callerName, callerCap, funcGrants, errs, tc, opts)
		checkCapExpr(node.Index, callerName, callerCap, funcGrants, errs, tc, opts)
	case *ast.NewExpr:
		for _, arg := range node.Args {
			checkCapExpr(arg, callerName, callerCap, funcGrants, errs, tc, opts)
		}
	case *ast.AwaitExpr:
		checkCapExpr(node.Expr, callerName, callerCap, funcGrants, errs, tc, opts)
	case *ast.IfExpr:
		checkCapExpr(node.Cond, callerName, callerCap, funcGrants, errs, tc, opts)
		checkCapExpr(node.Then, callerName, callerCap, funcGrants, errs, tc, opts)
		checkCapExpr(node.Else, callerName, callerCap, funcGrants, errs, tc, opts)
	case *ast.MatchExpr:
		checkCapExpr(node.Value, callerName, callerCap, funcGrants, errs, tc, opts)
		for _, arm := range node.Arms {
			for _, s := range arm.Body {
				checkCapStmt(s, callerName, callerCap, funcGrants, errs, tc, opts)
			}
		}
	case *ast.Lambda:
		for _, s := range node.Body {
			checkCapStmt(s, callerName, callerCap, funcGrants, errs, tc, opts)
		}
	case *ast.ArrayLiteral:
		for _, elem := range node.Elements {
			checkCapExpr(elem, callerName, callerCap, funcGrants, errs, tc, opts)
		}
	case *ast.MapLiteral:
		for _, entry := range node.Entries {
			checkCapExpr(entry.Key, callerName, callerCap, funcGrants, errs, tc, opts)
			checkCapExpr(entry.Value, callerName, callerCap, funcGrants, errs, tc, opts)
		}
	}
}

// isResultIntrinsic reports whether a call is a built-in Result operation.
// These mirror exactly what TypeChecker.inferType recognises: the Result.ok
// and Result.err constructors, and the unwrap/unwrapErr accessors on a
// Result-typed receiver. All are pure and require no capability grant.
func isResultIntrinsic(calleeName, funcName string) bool {
	if calleeName == "Result.ok" || calleeName == "Result.err" {
		return true
	}
	return funcName == "unwrap" || funcName == "unwrapErr"
}
