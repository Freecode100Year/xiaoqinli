package check

import (
	"fmt"

	"xiaoqinli/ast"
)

// Capability represents a set of granted abilities for a function.
type Capability []string

// Contains checks whether the required capability set is a subset of the
// provided capabilities.
func (c Capability) Contains(required Capability) error {
	set := make(map[string]struct{}, len(c))
	for _, abil := range c {
		set[abil] = struct{}{}
	}
	for _, need := range required {
		if _, ok := set[need]; !ok {
			return fmt.Errorf("XQL_E301: missing required capability: %s", need)
		}
	}
	return nil
}

// VerifyCapability ensures that the caller's capabilities are a superset
// of the callee's required capabilities.
func VerifyCapability(callerCap, requiredCap Capability) error {
	return callerCap.Contains(requiredCap)
}

// CheckCapabilities walks the AST and verifies capability inheritance:
// when function A calls function B, B's @grant must be a subset of A's @grant.
func CheckCapabilities(root ast.Node) error {
	// Collect all function declarations and their grants.
	funcGrants := make(map[string]Capability)
	collectGrants(root, funcGrants)

	var errs []string
	checkCaps(root, funcGrants, &errs)

	if len(errs) > 0 {
		msg := "XQL_E302: capability check failed:\n"
		for _, e := range errs {
			msg += "  - " + e + "\n"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func collectGrants(n ast.Node, out map[string]Capability) {
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			collectGrants(d, out)
		}
	case *ast.FunctionDecl:
		out[node.Name] = Capability(node.Grant)
	}
}

func checkCaps(n ast.Node, funcGrants map[string]Capability, errs *[]string) {
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			checkCaps(d, funcGrants, errs)
		}
	case *ast.FunctionDecl:
		callerCap := Capability(node.Grant)
		for _, stmt := range node.Body {
			checkCapStmt(stmt, node.Name, callerCap, funcGrants, errs)
		}
	}
}

func checkCapStmt(n ast.Node, callerName string, callerCap Capability, funcGrants map[string]Capability, errs *[]string) {
	switch node := n.(type) {
	case *ast.ExprStmt:
		checkCapExpr(node.Expr, callerName, callerCap, funcGrants, errs)
	case *ast.ReturnStmt:
		if node.Value != nil {
			checkCapExpr(node.Value, callerName, callerCap, funcGrants, errs)
		}
	case *ast.VarDecl:
		if node.Value != nil {
			checkCapExpr(node.Value, callerName, callerCap, funcGrants, errs)
		}
	case *ast.AssignStmt:
		checkCapExpr(node.Target, callerName, callerCap, funcGrants, errs)
		checkCapExpr(node.Value, callerName, callerCap, funcGrants, errs)
	case *ast.IfStmt:
		checkCapExpr(node.Cond, callerName, callerCap, funcGrants, errs)
		for _, s := range node.Then {
			checkCapStmt(s, callerName, callerCap, funcGrants, errs)
		}
		for _, s := range node.Else {
			checkCapStmt(s, callerName, callerCap, funcGrants, errs)
		}
	case *ast.WhileStmt:
		checkCapExpr(node.Cond, callerName, callerCap, funcGrants, errs)
		for _, s := range node.Body {
			checkCapStmt(s, callerName, callerCap, funcGrants, errs)
		}
	case *ast.ForStmt:
		if node.Start != nil {
			checkCapExpr(node.Start, callerName, callerCap, funcGrants, errs)
		}
		if node.End != nil {
			checkCapExpr(node.End, callerName, callerCap, funcGrants, errs)
		}
		if node.Iterable != nil {
			checkCapExpr(node.Iterable, callerName, callerCap, funcGrants, errs)
		}
		for _, s := range node.Body {
			checkCapStmt(s, callerName, callerCap, funcGrants, errs)
		}
	case *ast.BreakStmt, *ast.ContinueStmt:
		// No capability checking needed.
	}
}

func checkCapExpr(n ast.Node, callerName string, callerCap Capability, funcGrants map[string]Capability, errs *[]string) {
	switch node := n.(type) {
	case *ast.CallExpr:
		if calleeCap, ok := funcGrants[node.Callee]; ok {
			if err := callerCap.Contains(calleeCap); err != nil {
				*errs = append(*errs, fmt.Sprintf(
					"function '%s' calls '%s' but lacks required capabilities: %v",
					callerName, node.Callee, calleeCap))
			}
		}
		for _, arg := range node.Args {
			checkCapExpr(arg, callerName, callerCap, funcGrants, errs)
		}
	case *ast.BinaryExpr:
		checkCapExpr(node.Left, callerName, callerCap, funcGrants, errs)
		checkCapExpr(node.Right, callerName, callerCap, funcGrants, errs)
	case *ast.UnaryExpr:
		checkCapExpr(node.Operand, callerName, callerCap, funcGrants, errs)
	case *ast.MemberExpr:
		checkCapExpr(node.Object, callerName, callerCap, funcGrants, errs)
	case *ast.StructLit:
		for _, fi := range node.Fields {
			checkCapExpr(fi.Value, callerName, callerCap, funcGrants, errs)
		}
	case *ast.ArrayLit:
		for _, elem := range node.Elements {
			checkCapExpr(elem, callerName, callerCap, funcGrants, errs)
		}
	case *ast.IndexExpr:
		checkCapExpr(node.Target, callerName, callerCap, funcGrants, errs)
		checkCapExpr(node.Index, callerName, callerCap, funcGrants, errs)
	}
}
