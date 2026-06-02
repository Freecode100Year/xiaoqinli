package check

import (
	"fmt"

	"xiaoqinli/ast"
)

// Effect represents inferred side-effects of a function.
type Effect string

const (
	EffectPure       Effect = "pure"
	EffectNetwork    Effect = "network"
	EffectFilesystem Effect = "filesystem"
	EffectState      Effect = "state"
)

// builtinFuncs maps built-in function names to their return types and effects.
var builtinFuncs = map[string]struct {
	ReturnType string
	Effects    []Effect
}{
	"println": {ReturnType: "Void", Effects: []Effect{EffectState}},
	"printf":  {ReturnType: "Void", Effects: []Effect{EffectState}},
	"sprintf": {ReturnType: "String", Effects: nil},
}

// TypeChecker performs static type checking on a typed AST.
type TypeChecker struct {
	// funcTable maps function name to its declaration for cross-reference.
	funcTable map[string]*ast.FunctionDecl
	errors    []string
}

// NewTypeChecker creates a new TypeChecker.
func NewTypeChecker() *TypeChecker {
	return &TypeChecker{
		funcTable: make(map[string]*ast.FunctionDecl),
	}
}

// Check performs type checking on the provided AST root.
// Returns an error describing all type issues found.
func (tc *TypeChecker) Check(root ast.Node) error {
	// First pass: collect all function declarations.
	tc.collectFunctions(root)

	// Second pass: check each function body.
	tc.checkNode(root)

	if len(tc.errors) > 0 {
		msg := "XQL_E201: type check failed:\n"
		for _, e := range tc.errors {
			msg += "  - " + e + "\n"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func (tc *TypeChecker) collectFunctions(n ast.Node) {
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			tc.collectFunctions(d)
		}
	case *ast.FunctionDecl:
		tc.funcTable[node.Name] = node
	}
}

func (tc *TypeChecker) addError(msg string) {
	tc.errors = append(tc.errors, msg)
}

func (tc *TypeChecker) checkNode(n ast.Node) {
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			tc.checkNode(d)
		}
	case *ast.FunctionDecl:
		tc.checkFunctionDecl(node)
	}
}

func (tc *TypeChecker) checkFunctionDecl(fd *ast.FunctionDecl) {
	// Build a local scope with parameter types.
	scope := make(map[string]string) // name → type kind
	for _, p := range fd.Params {
		scope[p.Name] = p.Type.KindName
	}

	for _, stmt := range fd.Body {
		tc.checkStmt(stmt, fd, scope)
	}
}

func (tc *TypeChecker) checkStmt(n ast.Node, fn *ast.FunctionDecl, scope map[string]string) {
	switch node := n.(type) {
	case *ast.ReturnStmt:
		if node.Value != nil {
			exprType := tc.inferType(node.Value, scope)
			expected := fn.ReturnType.KindName
			if expected != "" && expected != "Void" && exprType != "" && exprType != expected {
				tc.addError(fmt.Sprintf(
					"function '%s': return type mismatch, expected %s but got %s",
					fn.Name, expected, exprType))
			}
		}
	case *ast.VarDecl:
		if node.Value != nil {
			valType := tc.inferType(node.Value, scope)
			if node.Type.KindName != "" && valType != "" && valType != node.Type.KindName {
				tc.addError(fmt.Sprintf(
					"variable '%s': type mismatch, declared %s but assigned %s",
					node.Name, node.Type.KindName, valType))
			}
			if node.Type.KindName != "" {
				scope[node.Name] = node.Type.KindName
			} else {
				scope[node.Name] = valType
			}
		} else if node.Type.KindName != "" {
			scope[node.Name] = node.Type.KindName
		}
	case *ast.AssignStmt:
		if declaredType, ok := scope[node.Target]; ok {
			valType := tc.inferType(node.Value, scope)
			if valType != "" && declaredType != "" && valType != declaredType {
				tc.addError(fmt.Sprintf(
					"assignment to '%s': type mismatch, expected %s but got %s",
					node.Target, declaredType, valType))
			}
		}
	case *ast.IfStmt:
		condType := tc.inferType(node.Cond, scope)
		if condType != "" && condType != "Bool" {
			tc.addError(fmt.Sprintf("if condition must be Bool, got %s", condType))
		}
		for _, s := range node.Then {
			tc.checkStmt(s, fn, scope)
		}
		for _, s := range node.Else {
			tc.checkStmt(s, fn, scope)
		}
	case *ast.WhileStmt:
		condType := tc.inferType(node.Cond, scope)
		if condType != "" && condType != "Bool" {
			tc.addError(fmt.Sprintf("while condition must be Bool, got %s", condType))
		}
		for _, s := range node.Body {
			tc.checkStmt(s, fn, scope)
		}
	case *ast.ExprStmt:
		tc.inferType(node.Expr, scope)
	}
}

// inferType infers the type of an expression. Returns the type kind string.
func (tc *TypeChecker) inferType(n ast.Node, scope map[string]string) string {
	switch node := n.(type) {
	case *ast.Literal:
		return node.ValueType
	case *ast.Ident:
		if t, ok := scope[node.Name]; ok {
			return t
		}
		return ""
	case *ast.BinaryExpr:
		leftT := tc.inferType(node.Left, scope)
		rightT := tc.inferType(node.Right, scope)
		return tc.checkBinaryOp(node.Op, leftT, rightT)
	case *ast.UnaryExpr:
		operandT := tc.inferType(node.Operand, scope)
		switch node.Op {
		case "!":
			if operandT != "" && operandT != "Bool" {
				tc.addError(fmt.Sprintf("unary '!' requires Bool, got %s", operandT))
			}
			return "Bool"
		case "-":
			if operandT != "" && operandT != "Int" && operandT != "Float" {
				tc.addError(fmt.Sprintf("unary '-' requires Int or Float, got %s", operandT))
			}
			return operandT
		}
		return operandT
	case *ast.CallExpr:
		// Check built-in functions.
		if bi, ok := builtinFuncs[node.Callee]; ok {
			return bi.ReturnType
		}
		// Check user-defined functions.
		if fd, ok := tc.funcTable[node.Callee]; ok {
			// Verify argument count.
			if len(node.Args) != len(fd.Params) {
				tc.addError(fmt.Sprintf(
					"function '%s' expects %d args, got %d",
					node.Callee, len(fd.Params), len(node.Args)))
			} else {
				// Verify argument types.
				for i, arg := range node.Args {
					argType := tc.inferType(arg, scope)
					paramType := fd.Params[i].Type.KindName
					if argType != "" && paramType != "" && argType != paramType {
						tc.addError(fmt.Sprintf(
							"function '%s' arg %d: expected %s, got %s",
							node.Callee, i, paramType, argType))
					}
				}
			}
			return fd.ReturnType.KindName
		}
		return ""
	case *ast.MemberExpr:
		tc.inferType(node.Object, scope)
		return ""
	default:
		return ""
	}
}

// checkBinaryOp checks type compatibility for a binary operator and returns the result type.
func (tc *TypeChecker) checkBinaryOp(op, leftT, rightT string) string {
	if leftT == "" || rightT == "" {
		return leftT + rightT // return whichever is known
	}

	switch op {
	case "+":
		if leftT == "String" && rightT == "String" {
			return "String"
		}
		if leftT == rightT && (leftT == "Int" || leftT == "Float") {
			return leftT
		}
		if leftT != rightT {
			tc.addError(fmt.Sprintf("operator '+': incompatible types %s and %s", leftT, rightT))
		}
		return leftT
	case "-", "*", "/", "%":
		if leftT != rightT {
			tc.addError(fmt.Sprintf("operator '%s': incompatible types %s and %s", op, leftT, rightT))
		}
		if leftT != "Int" && leftT != "Float" {
			tc.addError(fmt.Sprintf("operator '%s': requires numeric types, got %s", op, leftT))
		}
		return leftT
	case "==", "!=":
		if leftT != rightT {
			tc.addError(fmt.Sprintf("operator '%s': comparing different types %s and %s", op, leftT, rightT))
		}
		return "Bool"
	case "<", ">", "<=", ">=":
		if leftT != rightT {
			tc.addError(fmt.Sprintf("operator '%s': incompatible types %s and %s", op, leftT, rightT))
		}
		return "Bool"
	case "&&", "||":
		if leftT != "Bool" || rightT != "Bool" {
			tc.addError(fmt.Sprintf("operator '%s': requires Bool operands, got %s and %s", op, leftT, rightT))
		}
		return "Bool"
	default:
		return leftT
	}
}

// InferEffects walks the AST and infers the effects of each function.
func InferEffects(root ast.Node) map[string][]Effect {
	result := make(map[string][]Effect)
	inferEffectsNode(root, result)
	return result
}

func inferEffectsNode(n ast.Node, result map[string][]Effect) {
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			inferEffectsNode(d, result)
		}
	case *ast.FunctionDecl:
		effects := inferBodyEffects(node.Body)
		result[node.Name] = effects
	}
}

func inferBodyEffects(stmts []ast.Node) []Effect {
	seen := make(map[Effect]bool)
	for _, s := range stmts {
		collectEffects(s, seen)
	}
	if len(seen) == 0 {
		return []Effect{EffectPure}
	}
	effects := make([]Effect, 0, len(seen))
	for e := range seen {
		effects = append(effects, e)
	}
	return effects
}

func collectEffects(n ast.Node, seen map[Effect]bool) {
	switch node := n.(type) {
	case *ast.CallExpr:
		if bi, ok := builtinFuncs[node.Callee]; ok {
			for _, e := range bi.Effects {
				seen[e] = true
			}
		}
		for _, arg := range node.Args {
			collectEffects(arg, seen)
		}
	case *ast.ExprStmt:
		collectEffects(node.Expr, seen)
	case *ast.ReturnStmt:
		if node.Value != nil {
			collectEffects(node.Value, seen)
		}
	case *ast.VarDecl:
		if node.Value != nil {
			collectEffects(node.Value, seen)
		}
	case *ast.AssignStmt:
		collectEffects(node.Value, seen)
	case *ast.IfStmt:
		collectEffects(node.Cond, seen)
		for _, s := range node.Then {
			collectEffects(s, seen)
		}
		for _, s := range node.Else {
			collectEffects(s, seen)
		}
	case *ast.WhileStmt:
		collectEffects(node.Cond, seen)
		for _, s := range node.Body {
			collectEffects(s, seen)
		}
	case *ast.BinaryExpr:
		collectEffects(node.Left, seen)
		collectEffects(node.Right, seen)
	case *ast.UnaryExpr:
		collectEffects(node.Operand, seen)
	}
}

// CheckEffects verifies that declared effects match inferred effects.
func CheckEffects(root ast.Node) error {
	inferred := InferEffects(root)
	var errs []string

	checkEffectsNode(root, inferred, &errs)

	if len(errs) > 0 {
		msg := "XQL_E203: effect check failed:\n"
		for _, e := range errs {
			msg += "  - " + e + "\n"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func checkEffectsNode(n ast.Node, inferred map[string][]Effect, errs *[]string) {
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			checkEffectsNode(d, inferred, errs)
		}
	case *ast.FunctionDecl:
		// If function declares pure, verify no side effects inferred.
		for _, declared := range node.Effects {
			if declared == "pure" {
				if effects, ok := inferred[node.Name]; ok {
					for _, e := range effects {
						if e != EffectPure {
							*errs = append(*errs, fmt.Sprintf(
								"function '%s' declares @effects([\"pure\"]) but has inferred effect '%s'",
								node.Name, e))
						}
					}
				}
			}
		}
	}
}
