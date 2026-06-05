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
	funcTable   map[string]*ast.FunctionDecl
	structTable map[string]*ast.StructDecl
	enumTable   map[string]*ast.EnumDecl
	errors      []string
}

// NewTypeChecker creates a new TypeChecker.
func NewTypeChecker() *TypeChecker {
	return &TypeChecker{
		funcTable:   make(map[string]*ast.FunctionDecl),
		structTable: make(map[string]*ast.StructDecl),
		enumTable:   make(map[string]*ast.EnumDecl),
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
	case *ast.StructDecl:
		tc.structTable[node.Name] = node
	case *ast.EnumDecl:
		tc.enumTable[node.Name] = node
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
	scope := make(map[string]ast.TypeExpr)
	for _, p := range fd.Params {
		scope[p.Name] = p.Type
	}

	for _, stmt := range fd.Body {
		tc.checkStmt(stmt, fd, scope)
	}
}

func forkScope(parent map[string]ast.TypeExpr) map[string]ast.TypeExpr {
	child := make(map[string]ast.TypeExpr, len(parent))
	for k, v := range parent {
		child[k] = v
	}
	return child
}

func (tc *TypeChecker) checkStmt(n ast.Node, fn *ast.FunctionDecl, scope map[string]ast.TypeExpr) {
	switch node := n.(type) {
	case *ast.ReturnStmt:
		if node.Value != nil {
			exprType := tc.inferType(node.Value, scope)
			expected := fn.ReturnType.KindName
			if expected != "" && expected != "Void" && exprType.KindName != "" && exprType.KindName != expected {
				tc.addError(fmt.Sprintf(
					"function '%s': return type mismatch, expected %s but got %s",
					fn.Name, expected, exprType.KindName))
			}
		}
	case *ast.VarDecl:
		if node.Value != nil {
			valType := tc.inferType(node.Value, scope)
			if node.Type.KindName != "" && valType.KindName != "" && valType.KindName != node.Type.KindName {
				tc.addError(fmt.Sprintf(
					"variable '%s': type mismatch, declared %s but assigned %s",
					node.Name, node.Type.KindName, valType.KindName))
			}
			if node.Type.KindName != "" {
				scope[node.Name] = node.Type
			} else {
				scope[node.Name] = valType
			}
		} else if node.Type.KindName != "" {
			scope[node.Name] = node.Type
		}
	case *ast.AssignStmt:
		targetType := tc.inferType(node.Target, scope)
		valType := tc.inferType(node.Value, scope)
		if ident, ok := node.Target.(*ast.Ident); ok {
			if declaredType, exists := scope[ident.Name]; exists {
				if valType.KindName != "" && declaredType.KindName != "" && valType.KindName != declaredType.KindName {
					tc.addError(fmt.Sprintf(
						"assignment to '%s': type mismatch, expected %s but got %s",
						ident.Name, declaredType.KindName, valType.KindName))
				}
			}
		} else if targetType.KindName != "" && valType.KindName != "" && targetType.KindName != valType.KindName {
			tc.addError(fmt.Sprintf(
				"assignment type mismatch, expected %s but got %s",
				targetType.KindName, valType.KindName))
		}
	case *ast.IfStmt:
		condType := tc.inferType(node.Cond, scope)
		if condType.KindName != "" && condType.KindName != "Bool" {
			tc.addError(fmt.Sprintf("if condition must be Bool, got %s", condType.KindName))
		}
		thenScope := forkScope(scope)
		for _, s := range node.Then {
			tc.checkStmt(s, fn, thenScope)
		}
		elseScope := forkScope(scope)
		for _, s := range node.Else {
			tc.checkStmt(s, fn, elseScope)
		}
	case *ast.WhileStmt:
		condType := tc.inferType(node.Cond, scope)
		if condType.KindName != "" && condType.KindName != "Bool" {
			tc.addError(fmt.Sprintf("while condition must be Bool, got %s", condType.KindName))
		}
		bodyScope := forkScope(scope)
		for _, s := range node.Body {
			tc.checkStmt(s, fn, bodyScope)
		}
	case *ast.ForStmt:
		bodyScope := forkScope(scope)
		if node.Form == "range" {
			startType := tc.inferType(node.Start, scope)
			if startType.KindName != "" && startType.KindName != "Int" {
				tc.addError(fmt.Sprintf("for-range start must be Int, got %s", startType.KindName))
			}
			endType := tc.inferType(node.End, scope)
			if endType.KindName != "" && endType.KindName != "Int" {
				tc.addError(fmt.Sprintf("for-range end must be Int, got %s", endType.KindName))
			}
			bodyScope[node.Var] = ast.TypeExpr{KindName: "Int"}
		} else {
			iterType := tc.inferType(node.Iterable, scope)
			if iterType.KindName != "" && iterType.KindName != "Array" {
				tc.addError(fmt.Sprintf("for-each iterable must be Array, got %s", iterType.KindName))
			}
			if iterType.Elem != nil {
				bodyScope[node.Var] = *iterType.Elem
			} else {
				bodyScope[node.Var] = ast.TypeExpr{}
			}
		}
		for _, s := range node.Body {
			tc.checkStmt(s, fn, bodyScope)
		}
	case *ast.BreakStmt, *ast.ContinueStmt:
		// No type checking needed.
	case *ast.ExprStmt:
		tc.inferType(node.Expr, scope)
	case *ast.MatchExpr:
		tc.inferType(node.Value, scope)
		for _, arm := range node.Arms {
			armScope := forkScope(scope)
			for _, s := range arm.Body {
				tc.checkStmt(s, fn, armScope)
			}
		}
	}
}

// inferType infers the full type of an expression, preserving generic info.
func (tc *TypeChecker) inferType(n ast.Node, scope map[string]ast.TypeExpr) ast.TypeExpr {
	none := ast.TypeExpr{}
	switch node := n.(type) {
	case *ast.Literal:
		return ast.TypeExpr{KindName: node.ValueType}
	case *ast.Ident:
		if t, ok := scope[node.Name]; ok {
			return t
		}
		return none
	case *ast.BinaryExpr:
		leftT := tc.inferType(node.Left, scope)
		rightT := tc.inferType(node.Right, scope)
		return tc.checkBinaryOp(node.Op, leftT, rightT)
	case *ast.UnaryExpr:
		operandT := tc.inferType(node.Operand, scope)
		switch node.Op {
		case "!":
			if operandT.KindName != "" && operandT.KindName != "Bool" {
				tc.addError(fmt.Sprintf("unary '!' requires Bool, got %s", operandT.KindName))
			}
			return ast.TypeExpr{KindName: "Bool"}
		case "-":
			if operandT.KindName != "" && operandT.KindName != "Int" && operandT.KindName != "Float" {
				tc.addError(fmt.Sprintf("unary '-' requires Int or Float, got %s", operandT.KindName))
			}
			return operandT
		}
		return operandT
	case *ast.CallExpr:
		if bi, ok := builtinFuncs[node.Callee]; ok {
			return ast.TypeExpr{KindName: bi.ReturnType}
		}
		if fd, ok := tc.funcTable[node.Callee]; ok {
			if len(node.Args) != len(fd.Params) {
				tc.addError(fmt.Sprintf(
					"function '%s' expects %d args, got %d",
					node.Callee, len(fd.Params), len(node.Args)))
			} else {
				for i, arg := range node.Args {
					argType := tc.inferType(arg, scope)
					paramType := fd.Params[i].Type.KindName
					if argType.KindName != "" && paramType != "" && argType.KindName != paramType {
						tc.addError(fmt.Sprintf(
							"function '%s' arg %d: expected %s, got %s",
							node.Callee, i, paramType, argType.KindName))
					}
				}
			}
			return fd.ReturnType
		}
		return none
	case *ast.MemberExpr:
		objType := tc.inferType(node.Object, scope)
		if sd, ok := tc.structTable[objType.KindName]; ok {
			for _, f := range sd.Fields {
				if f.Name == node.Field {
					return f.Type
				}
			}
			tc.addError(fmt.Sprintf("struct '%s' has no field '%s'", objType.KindName, node.Field))
		}
		return none
	case *ast.StructLit:
		sd, ok := tc.structTable[node.TypeName]
		if !ok {
			tc.addError(fmt.Sprintf("unknown struct type '%s'", node.TypeName))
			return ast.TypeExpr{KindName: node.TypeName}
		}
		provided := make(map[string]bool)
		for _, fi := range node.Fields {
			provided[fi.Name] = true
			valType := tc.inferType(fi.Value, scope)
			var expectedType string
			for _, sf := range sd.Fields {
				if sf.Name == fi.Name {
					expectedType = sf.Type.KindName
					break
				}
			}
			if expectedType == "" {
				tc.addError(fmt.Sprintf("struct '%s' has no field '%s'", node.TypeName, fi.Name))
			} else if valType.KindName != "" && valType.KindName != expectedType {
				tc.addError(fmt.Sprintf("struct '%s' field '%s': expected %s, got %s",
					node.TypeName, fi.Name, expectedType, valType.KindName))
			}
		}
		for _, sf := range sd.Fields {
			if !provided[sf.Name] {
				tc.addError(fmt.Sprintf("struct '%s' missing field '%s'", node.TypeName, sf.Name))
			}
		}
		return ast.TypeExpr{KindName: node.TypeName}
	case *ast.ArrayLit:
		expectedElem := node.ElemType.KindName
		for i, elem := range node.Elements {
			elemType := tc.inferType(elem, scope)
			if expectedElem != "" && elemType.KindName != "" && elemType.KindName != expectedElem {
				tc.addError(fmt.Sprintf("array element %d: expected %s, got %s", i, expectedElem, elemType.KindName))
			}
		}
		return ast.TypeExpr{KindName: "Array", Elem: &node.ElemType}
	case *ast.IndexExpr:
		targetType := tc.inferType(node.Target, scope)
		indexType := tc.inferType(node.Index, scope)
		if targetType.KindName == "Array" {
			if indexType.KindName != "" && indexType.KindName != "Int" {
				tc.addError(fmt.Sprintf("array index must be Int, got %s", indexType.KindName))
			}
			if targetType.Elem != nil {
				return *targetType.Elem
			}
		}
		return none
	case *ast.MatchExpr:
		tc.inferType(node.Value, scope)
		return none
	default:
		return none
	}
}

func (tc *TypeChecker) checkBinaryOp(op string, leftT, rightT ast.TypeExpr) ast.TypeExpr {
	l, r := leftT.KindName, rightT.KindName
	if l == "" || r == "" {
		if l != "" {
			return leftT
		}
		return rightT
	}

	switch op {
	case "+":
		if l == "String" && r == "String" {
			return leftT
		}
		if l == r && (l == "Int" || l == "Float") {
			return leftT
		}
		if l != r {
			tc.addError(fmt.Sprintf("operator '+': incompatible types %s and %s", l, r))
		}
		return leftT
	case "-", "*", "/", "%":
		if l != r {
			tc.addError(fmt.Sprintf("operator '%s': incompatible types %s and %s", op, l, r))
		}
		if l != "Int" && l != "Float" {
			tc.addError(fmt.Sprintf("operator '%s': requires numeric types, got %s", op, l))
		}
		return leftT
	case "==", "!=":
		if l != r {
			tc.addError(fmt.Sprintf("operator '%s': comparing different types %s and %s", op, l, r))
		}
		return ast.TypeExpr{KindName: "Bool"}
	case "<", ">", "<=", ">=":
		if l != r {
			tc.addError(fmt.Sprintf("operator '%s': incompatible types %s and %s", op, l, r))
		}
		return ast.TypeExpr{KindName: "Bool"}
	case "&&", "||":
		if l != "Bool" || r != "Bool" {
			tc.addError(fmt.Sprintf("operator '%s': requires Bool operands, got %s and %s", op, l, r))
		}
		return ast.TypeExpr{KindName: "Bool"}
	default:
		return leftT
	}
}

// InferEffects walks the AST and infers the effects of each function,
// transitively following user-defined function calls.
func InferEffects(root ast.Node) map[string][]Effect {
	funcBodies := make(map[string][]ast.Node)
	collectFuncBodies(root, funcBodies)

	result := make(map[string][]Effect)
	resolving := make(map[string]bool)
	for name := range funcBodies {
		resolveEffects(name, funcBodies, result, resolving)
	}
	return result
}

func collectFuncBodies(n ast.Node, out map[string][]ast.Node) {
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			collectFuncBodies(d, out)
		}
	case *ast.FunctionDecl:
		out[node.Name] = node.Body
	}
}

func resolveEffects(name string, funcBodies map[string][]ast.Node, result map[string][]Effect, resolving map[string]bool) []Effect {
	if eff, ok := result[name]; ok {
		return eff
	}
	if resolving[name] {
		return nil
	}
	resolving[name] = true

	seen := make(map[Effect]bool)
	if body, ok := funcBodies[name]; ok {
		for _, s := range body {
			collectEffects(s, seen, funcBodies, result, resolving)
		}
	}

	var effects []Effect
	if len(seen) == 0 {
		effects = []Effect{EffectPure}
	} else {
		effects = make([]Effect, 0, len(seen))
		for e := range seen {
			effects = append(effects, e)
		}
	}
	result[name] = effects
	delete(resolving, name)
	return effects
}

func collectEffects(n ast.Node, seen map[Effect]bool, funcBodies map[string][]ast.Node, result map[string][]Effect, resolving map[string]bool) {
	switch node := n.(type) {
	case *ast.CallExpr:
		if bi, ok := builtinFuncs[node.Callee]; ok {
			for _, e := range bi.Effects {
				seen[e] = true
			}
		} else if _, ok := funcBodies[node.Callee]; ok {
			for _, e := range resolveEffects(node.Callee, funcBodies, result, resolving) {
				if e != EffectPure {
					seen[e] = true
				}
			}
		}
		for _, arg := range node.Args {
			collectEffects(arg, seen, funcBodies, result, resolving)
		}
	case *ast.ExprStmt:
		collectEffects(node.Expr, seen, funcBodies, result, resolving)
	case *ast.ReturnStmt:
		if node.Value != nil {
			collectEffects(node.Value, seen, funcBodies, result, resolving)
		}
	case *ast.VarDecl:
		if node.Value != nil {
			collectEffects(node.Value, seen, funcBodies, result, resolving)
		}
	case *ast.AssignStmt:
		collectEffects(node.Target, seen, funcBodies, result, resolving)
		collectEffects(node.Value, seen, funcBodies, result, resolving)
	case *ast.IfStmt:
		collectEffects(node.Cond, seen, funcBodies, result, resolving)
		for _, s := range node.Then {
			collectEffects(s, seen, funcBodies, result, resolving)
		}
		for _, s := range node.Else {
			collectEffects(s, seen, funcBodies, result, resolving)
		}
	case *ast.WhileStmt:
		collectEffects(node.Cond, seen, funcBodies, result, resolving)
		for _, s := range node.Body {
			collectEffects(s, seen, funcBodies, result, resolving)
		}
	case *ast.ForStmt:
		if node.Start != nil {
			collectEffects(node.Start, seen, funcBodies, result, resolving)
		}
		if node.End != nil {
			collectEffects(node.End, seen, funcBodies, result, resolving)
		}
		if node.Iterable != nil {
			collectEffects(node.Iterable, seen, funcBodies, result, resolving)
		}
		for _, s := range node.Body {
			collectEffects(s, seen, funcBodies, result, resolving)
		}
	case *ast.BinaryExpr:
		collectEffects(node.Left, seen, funcBodies, result, resolving)
		collectEffects(node.Right, seen, funcBodies, result, resolving)
	case *ast.UnaryExpr:
		collectEffects(node.Operand, seen, funcBodies, result, resolving)
	case *ast.StructLit:
		for _, fi := range node.Fields {
			collectEffects(fi.Value, seen, funcBodies, result, resolving)
		}
	case *ast.MemberExpr:
		collectEffects(node.Object, seen, funcBodies, result, resolving)
	case *ast.ArrayLit:
		for _, elem := range node.Elements {
			collectEffects(elem, seen, funcBodies, result, resolving)
		}
	case *ast.IndexExpr:
		collectEffects(node.Target, seen, funcBodies, result, resolving)
		collectEffects(node.Index, seen, funcBodies, result, resolving)
	case *ast.MatchExpr:
		collectEffects(node.Value, seen, funcBodies, result, resolving)
		for _, arm := range node.Arms {
			for _, s := range arm.Body {
				collectEffects(s, seen, funcBodies, result, resolving)
			}
		}
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
