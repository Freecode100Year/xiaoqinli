package check

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xiaoqinli/ast"
	"xiaoqinli/vfs"
)

// Effect represents inferred side-effects of a function.
type Effect string

const (
	EffectPure       Effect = "pure"
	EffectNetwork    Effect = "network"
	EffectFilesystem Effect = "filesystem"
	EffectState      Effect = "state"
)

// FunctionTypeName is the type a lambda expression has. It exists so a variable
// holding a lambda can be recognised at a call site; the lambda's own return
// type is carried in TypeExpr.Elem.
const FunctionTypeName = "Function"

// isFunctionValue reports whether an inferred type is a lambda. XQL has no
// syntax for a function type, so a lambda can only ever be compared against
// whatever name the author invented for the parameter holding it — "Callback",
// "Handler" — and reporting a mismatch there is noise, not a check.
func isFunctionValue(name string) bool { return name == FunctionTypeName }

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
	funcTable     map[string]*ast.FunctionDecl
	externTable   map[string]*ast.ExternDecl
	externMethods map[string]*ast.ExternDecl
	structTable   map[string]*ast.StructDecl
	enumTable     map[string]*ast.EnumDecl
	classTable    map[string]*ast.ClassDecl
	errors        []string
	currentReturn ast.TypeExpr
	currentFunc   *ast.FunctionDecl

	// Multi-file Workspace support
	currentFile string
	workspace   *vfs.Workspace
	imports     map[string]*TypeChecker

	// program retains this checker's own Program so later passes (notably the
	// capability check) can walk imported modules, not just the entry file.
	program *ast.Program
	Diagnostics []Diagnostic
}

// NewTypeChecker creates a new TypeChecker.
func NewTypeChecker() *TypeChecker {
	return &TypeChecker{
		funcTable:     make(map[string]*ast.FunctionDecl),
		externTable:   make(map[string]*ast.ExternDecl),
		externMethods: make(map[string]*ast.ExternDecl),
		structTable:   make(map[string]*ast.StructDecl),
		enumTable:     make(map[string]*ast.EnumDecl),
		classTable:    make(map[string]*ast.ClassDecl),
		imports:       make(map[string]*TypeChecker),
	}
}

// Check performs type checking on the provided AST root.
// Returns an error describing all type issues found.
func (tc *TypeChecker) Check(root ast.Node) error {
	if root == nil {
		return fmt.Errorf("root node is nil")
	}

	if prog, ok := root.(*ast.Program); ok {
		visiting := make(map[string]bool)
		if tc.currentFile != "" {
			visiting[tc.currentFile] = true
		}
		if err := tc.loadImports(prog, visiting); err != nil {
			tc.addError(err.Error())
			return err
		}
	}

	// First pass: collect all function declarations.
	tc.collectFunctions(root)
	tc.inheritExterns(map[*TypeChecker]bool{tc: true})
	tc.checkExternShadowing()

	// 跨模块全局符号命名冲突检测
	if err := tc.checkGlobalSymbolConflicts(root); err != nil {
		tc.addError(err.Error())
		return err
	}

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
	if n == nil {
		return
	}
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			tc.collectFunctions(d)
		}
	case *ast.FunctionDecl:
		tc.funcTable[node.Name] = node
	case *ast.ExternDecl:
		if node.Method {
			tc.externMethods[node.Name] = node
		} else {
			tc.externTable[node.Name] = node
		}
	case *ast.StructDecl:
		tc.structTable[node.Name] = node
	case *ast.ClassDecl:
		tc.classTable[node.Name] = node
	case *ast.EnumDecl:
		tc.enumTable[node.Name] = node
	}
}

// methodExtern resolves a qualified callee against the declared host methods by
// its final segment, so "res.json" matches an extern method named "json".
// Returns nil when no method extern claims the call.
func (tc *TypeChecker) methodExtern(callee string) *ast.ExternDecl {
	idx := strings.LastIndex(callee, ".")
	if idx < 0 {
		return nil
	}
	return tc.externMethods[callee[idx+1:]]
}

// inheritExterns pulls extern declarations up from imported modules. Unlike
// functions and types, an extern is not namespaced by its module: it names one
// host function, so a module that declares `fetch` makes it callable by its
// importers under that same name. Redeclarations that disagree are an error;
// identical ones are simply the same host function seen twice.
func (tc *TypeChecker) inheritExterns(visited map[*TypeChecker]bool) {
	for _, depTC := range tc.imports {
		if visited[depTC] {
			continue
		}
		visited[depTC] = true
		depTC.inheritExterns(visited)
		tc.mergeExterns(tc.externTable, depTC.externTable)
		tc.mergeExterns(tc.externMethods, depTC.externMethods)
	}
}

func (tc *TypeChecker) mergeExterns(dst, src map[string]*ast.ExternDecl) {
	for name, ed := range src {
		existing, ok := dst[name]
		if !ok {
			dst[name] = ed
			continue
		}
		if !existing.SignatureEquals(ed) {
			tc.addError(fmt.Sprintf(
				"extern '%s' is declared with conflicting signatures across modules", name))
		}
	}
}

// checkExternShadowing rejects a program that declares a name both as an extern
// and as a real function: the two would disagree about who provides the body,
// and call resolution would silently pick one of them.
func (tc *TypeChecker) checkExternShadowing() {
	for name := range tc.externTable {
		if _, ok := tc.funcTable[name]; ok {
			tc.addError(fmt.Sprintf(
				"extern '%s' is also declared as a function in the same program", name))
		}
	}
}

// checkExternCall validates a call against an extern signature and yields its
// declared return type. An extern that omits "params" declares an unchecked
// signature, so arity and argument types are not enforced — the host owns them.
func (tc *TypeChecker) checkExternCall(node *ast.CallExpr, ed *ast.ExternDecl, scope map[string]ast.TypeExpr) ast.TypeExpr {
	if !ed.HasParams {
		for _, arg := range node.Args {
			tc.inferType(arg, scope)
		}
		return ed.ReturnType
	}
	if len(node.Args) != len(ed.Params) {
		tc.addError(fmt.Sprintf(
			"extern '%s' expects %d args, got %d",
			ed.Name, len(ed.Params), len(node.Args)))
		return ed.ReturnType
	}
	for i, arg := range node.Args {
		argType := tc.inferType(arg, scope)
		paramType := ed.Params[i].Type.KindName
		if argType.KindName != "" && paramType != "" && argType.KindName != paramType && !isFunctionValue(argType.KindName) {
			tc.addError(fmt.Sprintf(
				"extern '%s' arg %d: expected %s, got %s",
				ed.Name, i, paramType, argType.KindName))
		}
	}
	return ed.ReturnType
}

func (tc *TypeChecker) addError(msg string) {
	tc.errors = append(tc.errors, msg)

	code := "XQL_E201"
	fix := "Check code correctness or schema compatibility."

	if strings.Contains(msg, "expects") && strings.Contains(msg, "args") {
		code = "XQL_E201"
		fix = "Adjust the number of arguments to match the function signature."
	} else if (strings.Contains(msg, "expected") && strings.Contains(msg, "got")) || strings.Contains(msg, "type mismatch") {
		code = "XQL_E201"
		fix = "Change the expression or argument value to match the expected type."
	} else if strings.Contains(msg, "undefined function") {
		code = "XQL_E201"
		fix = "Ensure the function is declared or imported properly."
	} else if strings.Contains(msg, "undefined variable") {
		code = "XQL_E201"
		fix = "Declare the variable with 'var' before using it."
	} else if strings.Contains(msg, "circular import") {
		code = "XQL_E402"
		fix = "Remove the circular reference between modules."
	} else if strings.Contains(msg, "is defined in multiple files") {
		code = "XQL_E202"
		fix = "Rename one of the conflicting global symbols."
	} else if strings.Contains(msg, "is also declared as a function") {
		code = "XQL_E202"
		fix = "Drop either the ExternDecl or the FunctionDecl; a name is provided by the host or by this program, not both."
	} else if strings.Contains(msg, "conflicting signatures across modules") {
		code = "XQL_E202"
		fix = "Declare the extern once and import it, or make every declaration identical."
	} else if strings.Contains(msg, "missing required capability") || strings.Contains(msg, "lacks required capabilities") {
		code = "XQL_E301"
		capName := "required capability"
		if strings.Contains(msg, "lacks required capabilities: ") {
			parts := strings.SplitN(msg, "lacks required capabilities: ", 2)
			if len(parts) == 2 {
				capName = parts[1]
			}
		} else if strings.Contains(msg, "missing required capability: ") {
			parts := strings.SplitN(msg, "missing required capability: ", 2)
			if len(parts) == 2 {
				capName = parts[1]
			}
		}
		fix = fmt.Sprintf("Add the missing capability name to the caller function's @grant list. Example: grant: [\"%s\"]", capName)
	} else if strings.Contains(msg, "has inferred effect") {
		code = "XQL_E203"
		fix = "Remove the pure annotation or remove the side-effecting code."
	}

	tc.Diagnostics = append(tc.Diagnostics, Diagnostic{
		Code:         code,
		Message:      msg,
		SuggestedFix: fix,
	})
}

func (tc *TypeChecker) checkNode(n ast.Node) {
	if n == nil {
		return
	}
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

	oldReturn := tc.currentReturn
	tc.currentReturn = fd.ReturnType
	defer func() { tc.currentReturn = oldReturn }()

	oldFunc := tc.currentFunc
	tc.currentFunc = fd
	defer func() { tc.currentFunc = oldFunc }()

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
	if n == nil {
		return
	}
	switch node := n.(type) {
	case *ast.ReturnStmt:
		if node.Value != nil {
			exprType := tc.inferType(node.Value, scope)
			expected := tc.currentReturn.KindName
			if expected != "" && expected != "Void" && exprType.KindName != "" && exprType.KindName != expected {
				tc.addError(fmt.Sprintf(
					"function '%s': return type mismatch, expected %s but got %s",
					fn.Name, expected, exprType.KindName))
			}
		}
	case *ast.VarDecl:
		if node.Value != nil {
			valType := tc.inferType(node.Value, scope)
			if node.Type.KindName != "" && valType.KindName != "" && valType.KindName != node.Type.KindName && !isFunctionValue(valType.KindName) {
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
				if valType.KindName != "" && declaredType.KindName != "" && valType.KindName != declaredType.KindName && !isFunctionValue(valType.KindName) {
					tc.addError(fmt.Sprintf(
						"assignment to '%s': type mismatch, expected %s but got %s",
						ident.Name, declaredType.KindName, valType.KindName))
				}
			}
		} else if targetType.KindName != "" && valType.KindName != "" && targetType.KindName != valType.KindName && !isFunctionValue(valType.KindName) {
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
	case *ast.SwitchStmt:
		valType := tc.inferType(node.Value, scope)
		for _, c := range node.Cases {
			if c.Value != nil {
				caseType := tc.inferType(c.Value, scope)
				if valType.KindName != "" && caseType.KindName != "" && valType.KindName != caseType.KindName {
					tc.addError(fmt.Sprintf("switch case value type mismatch, expected %s, got %s", valType.KindName, caseType.KindName))
				}
			}
			caseScope := forkScope(scope)
			for _, s := range c.Body {
				tc.checkStmt(s, fn, caseScope)
			}
		}
	}
}

// inferType infers the full type of an expression, preserving generic info.
func (tc *TypeChecker) inferType(n ast.Node, scope map[string]ast.TypeExpr) ast.TypeExpr {
	none := ast.TypeExpr{}
	if n == nil {
		return none
	}
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
	case *ast.NewExpr:
		for _, arg := range node.Args {
			tc.inferType(arg, scope)
		}
		return ast.TypeExpr{KindName: ""}
	case *ast.AwaitExpr:
		return tc.inferType(node.Expr, scope)
	case *ast.CallExpr:
		if node.Callee == "Result.ok" {
			var okType ast.TypeExpr
			if len(node.Args) > 0 {
				okType = tc.inferType(node.Args[0], scope)
			} else {
				okType = ast.TypeExpr{KindName: "Void"}
			}
			return ast.TypeExpr{KindName: "Result", OkType: &okType}
		}
		if node.Callee == "Result.err" {
			var errType ast.TypeExpr
			if len(node.Args) > 0 {
				errType = tc.inferType(node.Args[0], scope)
			} else {
				errType = ast.TypeExpr{KindName: "Void"}
			}
			return ast.TypeExpr{KindName: "Result", ErrType: &errType}
		}
		if strings.Contains(node.Callee, ".") {
			parts := strings.Split(node.Callee, ".")
			if len(parts) == 2 {
				objName := parts[0]
				method := parts[1]
				if objType, ok := scope[objName]; ok && objType.KindName == "Result" {
					if method == "unwrap" {
						if objType.OkType != nil {
							return *objType.OkType
						}
						return ast.TypeExpr{KindName: "Void"}
					}
					if method == "unwrapErr" {
						if objType.ErrType != nil {
							return *objType.ErrType
						}
						return ast.TypeExpr{KindName: "Void"}
					}
				}
			}
		}
		// A local binding shadows every global name, so a variable holding a
		// lambda is resolved before functions, externs, and builtins.
		if t, ok := scope[node.Callee]; ok && t.KindName == FunctionTypeName {
			for _, arg := range node.Args {
				tc.inferType(arg, scope)
			}
			if t.Elem != nil {
				return *t.Elem
			}
			return none
		}
		if ed, ok := tc.externTable[node.Callee]; ok {
			return tc.checkExternCall(node, ed, scope)
		}
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
					if argType.KindName != "" && paramType != "" && argType.KindName != paramType && !isFunctionValue(argType.KindName) {
						tc.addError(fmt.Sprintf(
							"function '%s' arg %d: expected %s, got %s",
							node.Callee, i, paramType, argType.KindName))
					}
				}
			}
			return fd.ReturnType
		}
		if strings.Contains(node.Callee, ".") {
			parts := strings.Split(node.Callee, ".")
			if len(parts) == 2 {
				alias := parts[0]
				funcName := parts[1]
				if depTC, ok := tc.imports[alias]; ok {
					if fd, ok := depTC.funcTable[funcName]; ok {
						if len(node.Args) != len(fd.Params) {
							tc.addError(fmt.Sprintf(
								"function '%s' expects %d args, got %d",
								node.Callee, len(fd.Params), len(node.Args)))
						} else {
							for i, arg := range node.Args {
								argType := tc.inferType(arg, scope)
								paramType := fd.Params[i].Type.KindName
								if argType.KindName != "" && paramType != "" && !typesMatch(paramType, argType.KindName, alias) && !isFunctionValue(argType.KindName) {
									tc.addError(fmt.Sprintf(
										"function '%s' arg %d: expected %s, got %s",
										node.Callee, i, paramType, argType.KindName))
								}
							}
						}
						return fd.ReturnType
					}
				}
			}
		}
		// Method externs are matched last: a qualified call is far more likely
		// to be a module reference, and only when nothing else claims it does
		// the final segment get read as a host method.
		if ed := tc.methodExtern(node.Callee); ed != nil {
			return tc.checkExternCall(node, ed, scope)
		}
		tc.addError(fmt.Sprintf("undefined function: %s", node.Callee))
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
		} else if cd, ok := tc.classTable[objType.KindName]; ok {
			for _, f := range cd.Fields {
				if f.Name == node.Field {
					return f.Type
				}
			}
			tc.addError(fmt.Sprintf("class '%s' has no field '%s'", objType.KindName, node.Field))
		} else if strings.Contains(objType.KindName, ".") {
			parts := strings.Split(objType.KindName, ".")
			if len(parts) == 2 {
				alias := parts[0]
				localTypeName := parts[1]
				if depTC, ok := tc.imports[alias]; ok {
					if sd, ok := depTC.structTable[localTypeName]; ok {
						for _, f := range sd.Fields {
							if f.Name == node.Field {
								return f.Type
							}
						}
						tc.addError(fmt.Sprintf("struct '%s' has no field '%s'", objType.KindName, node.Field))
					} else if cd, ok := depTC.classTable[localTypeName]; ok {
						for _, f := range cd.Fields {
							if f.Name == node.Field {
								return f.Type
							}
						}
						tc.addError(fmt.Sprintf("class '%s' has no field '%s'", objType.KindName, node.Field))
					}
				}
			}
		}
		return none
	case *ast.StructLit:
		sd, ok := tc.structTable[node.TypeName]
		if !ok {
			// Treat as external/dynamic struct (like standard library/web types).
			// We check the fields' values recursively but do not enforce schema rules.
			for _, fi := range node.Fields {
				tc.inferType(fi.Value, scope)
			}
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
			} else if valType.KindName != "" && valType.KindName != expectedType && !isFunctionValue(valType.KindName) {
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
	case *ast.ArrayLiteral:
		expectedElem := node.ElemType.KindName
		for i, elem := range node.Elements {
			elemType := tc.inferType(elem, scope)
			if expectedElem != "" && elemType.KindName != "" && elemType.KindName != expectedElem {
				tc.addError(fmt.Sprintf("array element %d: expected %s, got %s", i, expectedElem, elemType.KindName))
			}
		}
		return ast.TypeExpr{KindName: "Array", Elem: &node.ElemType}
	case *ast.MapLiteral:
		expectedKey := node.KeyType.KindName
		expectedVal := node.ValueType.KindName
		for i, entry := range node.Entries {
			kt := tc.inferType(entry.Key, scope)
			vt := tc.inferType(entry.Value, scope)
			if expectedKey != "" && kt.KindName != "" && kt.KindName != expectedKey {
				tc.addError(fmt.Sprintf("map key %d: expected %s, got %s", i, expectedKey, kt.KindName))
			}
			if expectedVal != "" && vt.KindName != "" && vt.KindName != expectedVal {
				tc.addError(fmt.Sprintf("map value %d: expected %s, got %s", i, expectedVal, vt.KindName))
			}
		}
		return ast.TypeExpr{KindName: "Map", KeyType: &node.KeyType, Elem: &node.ValueType}
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
	case *ast.IfExpr:
		condType := tc.inferType(node.Cond, scope)
		if condType.KindName != "" && condType.KindName != "Bool" {
			tc.addError(fmt.Sprintf("IfExpr condition must be Bool, got %s", condType.KindName))
		}
		thenType := tc.inferType(node.Then, scope)
		elseType := tc.inferType(node.Else, scope)
		if thenType.KindName != "" && elseType.KindName != "" && thenType.KindName != elseType.KindName {
			tc.addError(fmt.Sprintf("IfExpr branches must have same type, got %s and %s", thenType.KindName, elseType.KindName))
		}
		if thenType.KindName != "" {
			return thenType
		}
		return elseType
	case *ast.Lambda:
		lambdaScope := forkScope(scope)
		for _, p := range node.Params {
			lambdaScope[p.Name] = p.Type
		}
		oldReturn := tc.currentReturn
		tc.currentReturn = node.ReturnType
		defer func() { tc.currentReturn = oldReturn }()

		for _, s := range node.Body {
			tc.checkStmt(s, tc.currentFunc, lambdaScope)
		}
		// A lambda has a type, so a variable bound to one can be called by name
		// instead of being reported as an undefined function. Elem carries the
		// lambda's return type so the call site still types.
		retType := node.ReturnType
		return ast.TypeExpr{KindName: FunctionTypeName, Elem: &retType}
	default:
		return none
	}
}

func (tc *TypeChecker) checkBinaryOp(op string, leftT, rightT ast.TypeExpr) ast.TypeExpr {
	if op == "==" || op == "!=" || op == "===" || op == "!==" || op == "<" || op == ">" || op == "<=" || op == ">=" {
		return ast.TypeExpr{KindName: "Bool"}
	}
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
	return InferEffectsWithTC(root, NewTypeChecker())
}

func InferEffectsWithTC(root ast.Node, tc *TypeChecker) map[string][]Effect {
	if root == nil {
		return nil
	}
	funcBodies := make(map[string][]ast.Node)
	collectFuncBodies(root, funcBodies)

	result := make(map[string][]Effect)
	resolving := make(map[string]bool)
	for name := range funcBodies {
		resolveEffects(name, funcBodies, result, resolving, tc)
	}
	return result
}

func collectFuncBodies(n ast.Node, out map[string][]ast.Node) {
	if n == nil {
		return
	}
	switch node := n.(type) {
	case *ast.Program:
		for _, d := range node.Decls {
			collectFuncBodies(d, out)
		}
	case *ast.FunctionDecl:
		out[node.Name] = node.Body
	}
}

func resolveEffects(name string, funcBodies map[string][]ast.Node, result map[string][]Effect, resolving map[string]bool, tc *TypeChecker) []Effect {
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
			collectEffects(s, seen, funcBodies, result, resolving, tc)
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

func collectEffects(n ast.Node, seen map[Effect]bool, funcBodies map[string][]ast.Node, result map[string][]Effect, resolving map[string]bool, tc *TypeChecker) {
	if n == nil {
		return
	}
	switch node := n.(type) {
	case *ast.CallExpr:
		if ed, ok := tc.externTable[node.Callee]; ok {
			// A host call's effects are whatever it declares; there is no body
			// to infer them from, which is the whole point of declaring them.
			for _, e := range ed.Effects {
				if Effect(e) != EffectPure {
					seen[Effect(e)] = true
				}
			}
		} else if bi, ok := builtinFuncs[node.Callee]; ok {
			for _, e := range bi.Effects {
				seen[e] = true
			}
		} else if _, ok := funcBodies[node.Callee]; ok {
			for _, e := range resolveEffects(node.Callee, funcBodies, result, resolving, tc) {
				if e != EffectPure {
					seen[e] = true
				}
			}
		} else if strings.Contains(node.Callee, ".") {
			parts := strings.Split(node.Callee, ".")
			if len(parts) == 2 {
				alias := parts[0]
				funcName := parts[1]
				if depTC, ok := tc.imports[alias]; ok {
					if fd, ok := depTC.funcTable[funcName]; ok {
						isPure := false
						for _, eff := range fd.Effects {
							if eff == "pure" {
								isPure = true
							}
						}
						if !isPure {
							seen[Effect("impure")] = true
						}
					}
				}
			}
			if ed := tc.methodExtern(node.Callee); ed != nil {
				for _, e := range ed.Effects {
					if Effect(e) != EffectPure {
						seen[Effect(e)] = true
					}
				}
			}
		}
		for _, arg := range node.Args {
			collectEffects(arg, seen, funcBodies, result, resolving, tc)
		}
	case *ast.ExprStmt:
		collectEffects(node.Expr, seen, funcBodies, result, resolving, tc)
	case *ast.ReturnStmt:
		if node.Value != nil {
			collectEffects(node.Value, seen, funcBodies, result, resolving, tc)
		}
	case *ast.VarDecl:
		if node.Value != nil {
			collectEffects(node.Value, seen, funcBodies, result, resolving, tc)
		}
	case *ast.AssignStmt:
		collectEffects(node.Target, seen, funcBodies, result, resolving, tc)
		collectEffects(node.Value, seen, funcBodies, result, resolving, tc)
	case *ast.IfStmt:
		collectEffects(node.Cond, seen, funcBodies, result, resolving, tc)
		for _, s := range node.Then {
			collectEffects(s, seen, funcBodies, result, resolving, tc)
		}
		for _, s := range node.Else {
			collectEffects(s, seen, funcBodies, result, resolving, tc)
		}
	case *ast.WhileStmt:
		collectEffects(node.Cond, seen, funcBodies, result, resolving, tc)
		for _, s := range node.Body {
			collectEffects(s, seen, funcBodies, result, resolving, tc)
		}
	case *ast.ForStmt:
		if node.Start != nil {
			collectEffects(node.Start, seen, funcBodies, result, resolving, tc)
		}
		if node.End != nil {
			collectEffects(node.End, seen, funcBodies, result, resolving, tc)
		}
		if node.Iterable != nil {
			collectEffects(node.Iterable, seen, funcBodies, result, resolving, tc)
		}
		for _, s := range node.Body {
			collectEffects(s, seen, funcBodies, result, resolving, tc)
		}
	case *ast.BinaryExpr:
		collectEffects(node.Left, seen, funcBodies, result, resolving, tc)
		collectEffects(node.Right, seen, funcBodies, result, resolving, tc)
	case *ast.UnaryExpr:
		collectEffects(node.Operand, seen, funcBodies, result, resolving, tc)
	case *ast.NewExpr:
		for _, arg := range node.Args {
			collectEffects(arg, seen, funcBodies, result, resolving, tc)
		}
	case *ast.AwaitExpr:
		collectEffects(node.Expr, seen, funcBodies, result, resolving, tc)
	case *ast.StructLit:
		for _, fi := range node.Fields {
			collectEffects(fi.Value, seen, funcBodies, result, resolving, tc)
		}
	case *ast.MemberExpr:
		collectEffects(node.Object, seen, funcBodies, result, resolving, tc)
	case *ast.ArrayLit:
		for _, elem := range node.Elements {
			collectEffects(elem, seen, funcBodies, result, resolving, tc)
		}
	case *ast.IndexExpr:
		collectEffects(node.Target, seen, funcBodies, result, resolving, tc)
		collectEffects(node.Index, seen, funcBodies, result, resolving, tc)
	case *ast.MatchExpr:
		collectEffects(node.Value, seen, funcBodies, result, resolving, tc)
		for _, arm := range node.Arms {
			for _, s := range arm.Body {
				collectEffects(s, seen, funcBodies, result, resolving, tc)
			}
		}
	case *ast.IfExpr:
		collectEffects(node.Cond, seen, funcBodies, result, resolving, tc)
		collectEffects(node.Then, seen, funcBodies, result, resolving, tc)
		collectEffects(node.Else, seen, funcBodies, result, resolving, tc)
	case *ast.Lambda:
		for _, s := range node.Body {
			collectEffects(s, seen, funcBodies, result, resolving, tc)
		}
	}
}

// CheckEffects verifies that declared effects match inferred effects.
func CheckEffects(root ast.Node) error {
	return CheckEffectsWithTC(root, NewTypeChecker())
}

func CheckEffectsWithTC(root ast.Node, tc *TypeChecker) error {
	if root == nil {
		return fmt.Errorf("root node is nil")
	}
	inferred := InferEffectsWithTC(root, tc)
	var errs []string

	checkEffectsNode(root, inferred, &errs)

	for _, e := range errs {
		tc.addError(e)
	}

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
	if n == nil {
		return
	}
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

func (tc *TypeChecker) loadImports(prog *ast.Program, visiting map[string]bool) error {
	for _, d := range prog.Decls {
		if imp, ok := d.(*ast.ImportDecl); ok {
			depPath := resolveRelativePath(tc.currentFile, imp.Path)
			if visiting[depPath] {
				return fmt.Errorf("XQL_E402: circular import detected: %s", depPath)
			}
			var depData []byte
			var err error
			if tc.workspace != nil && tc.workspace.Exists(depPath) {
				depData, err = tc.workspace.Read(depPath)
			} else {
				depData, err = os.ReadFile(depPath)
			}
			if err != nil {
				return fmt.Errorf("XQL_E404: failed to load import %q (%s): %w", imp.Path, depPath, err)
			}
			depProgNode, err := ast.Parse(depData)
			if err != nil {
				return fmt.Errorf("XQL_E101: failed to parse import %q: %w", imp.Path, err)
			}
			depProg, ok := depProgNode.(*ast.Program)
			if !ok {
				return fmt.Errorf("XQL_E101: imported file %q is not a valid Program", imp.Path)
			}
			depTC := NewTypeChecker()
			depTC.currentFile = depPath
			depTC.workspace = tc.workspace
			depTC.program = depProg
			nextVisiting := make(map[string]bool)
			for k, v := range visiting {
				nextVisiting[k] = v
			}
			nextVisiting[depPath] = true
			if err := depTC.loadImports(depProg, nextVisiting); err != nil {
				return err
			}
			depTC.collectFunctions(depProg)
			// A module gets the same extern visibility as the entry file: it
			// can call what its own imports declare. Without this an extern is
			// only usable from the file the compiler happens to be invoked on,
			// and a module calling a host function it imported is reported as
			// an undefined function.
			depTC.inheritExterns(map[*TypeChecker]bool{depTC: true})
			depTC.checkExternShadowing()
			depTC.checkNode(depProg)
			if len(depTC.errors) > 0 {
				msg := fmt.Sprintf("XQL_E201: type check failed in import %q:\n", imp.Path)
				for _, e := range depTC.errors {
					msg += "  - " + e + "\n"
				}
				return errors.New(msg)
			}
			if _, exists := tc.imports[imp.As]; exists {
				return fmt.Errorf("XQL_E202: duplicate import alias %q", imp.As)
			}
			tc.imports[imp.As] = depTC
		}
	}
	return nil
}

func resolveRelativePath(current, target string) string {
	if current == "" {
		return filepath.Clean(target)
	}
	dir := filepath.Dir(current)
	return filepath.Clean(filepath.Join(dir, target))
}

func typesMatch(expected, actual string, alias string) bool {
	if expected == actual {
		return true
	}
	if actual == alias+"."+expected {
		return true
	}
	if expected == alias+"."+actual {
		return true
	}
	return false
}

func (tc *TypeChecker) checkGlobalSymbolConflicts(root ast.Node) error {
	definedSymbols := make(map[string]string)
	collect := func(progTC *TypeChecker, filename string) error {
		for s := range progTC.funcTable {
			if orig, exists := definedSymbols[s]; exists && orig != filename {
				return fmt.Errorf("XQL_E202: global symbol %q is defined in multiple files: %s and %s", s, orig, filename)
			}
			definedSymbols[s] = filename
		}
		for s := range progTC.structTable {
			if orig, exists := definedSymbols[s]; exists && orig != filename {
				return fmt.Errorf("XQL_E202: global symbol %q is defined in multiple files: %s and %s", s, orig, filename)
			}
			definedSymbols[s] = filename
		}
		for s := range progTC.classTable {
			if orig, exists := definedSymbols[s]; exists && orig != filename {
				return fmt.Errorf("XQL_E202: global symbol %q is defined in multiple files: %s and %s", s, orig, filename)
			}
			definedSymbols[s] = filename
		}
		for s := range progTC.enumTable {
			if orig, exists := definedSymbols[s]; exists && orig != filename {
				return fmt.Errorf("XQL_E202: global symbol %q is defined in multiple files: %s and %s", s, orig, filename)
			}
			definedSymbols[s] = filename
		}
		return nil
	}

	currentFile := tc.currentFile
	if currentFile == "" {
		currentFile = "main.xql"
	}
	if err := collect(tc, currentFile); err != nil {
		return err
	}

	for alias, depTC := range tc.imports {
		depFile := depTC.currentFile
		if depFile == "" {
			depFile = alias + ".xql"
		}
		if err := collect(depTC, depFile); err != nil {
			return err
		}
	}
	return nil
}
