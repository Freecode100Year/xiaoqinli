package codegen

import "xiaoqinli/ast"

// A backend that emits `a / b` verbatim has delegated the meaning of division
// to whatever the target language happens to think it means, and the target
// languages do not agree. Given two Ints, `7 / 2` is 3 in Go, C, Rust, Java,
// Ruby and Tcl, and 3.5 in Python, JavaScript, Perl, awk, Lua, PHP, Julia and
// Dart. In Haskell and Zig it does not compile at all — `/` is not defined on
// their integer types.
//
// Deciding that needs one fact: are both operands Ints? Four backends already
// carry their own `inferTypeKind` to answer questions like it. typeKinds is
// that same idea in one place, so a backend gains the answer by declaring a
// field and calling three methods rather than by copying forty lines.
//
// The maps are filled as the emitter walks, not up front, because a parameter
// named `n` in one function and a String named `n` in the next are different
// variables and the later declaration is the one in scope.
type typeKinds struct {
	vars  map[string]string
	funcs map[string]string
}

func newTypeKinds(root ast.Node) *typeKinds {
	t := &typeKinds{
		vars:  make(map[string]string),
		funcs: make(map[string]string),
	}
	// Return types are collected up front: a call can name a function declared
	// further down the file, and unlike variables those names do not shadow.
	if prog, ok := root.(*ast.Program); ok {
		for _, d := range prog.Decls {
			switch decl := d.(type) {
			case *ast.FunctionDecl:
				t.funcs[decl.Name] = decl.ReturnType.KindName
			case *ast.ExternDecl:
				t.funcs[decl.Name] = decl.ReturnType.KindName
			}
		}
	}
	return t
}

// noteParams records a function's parameter types as it is entered.
func (t *typeKinds) noteParams(fd *ast.FunctionDecl) {
	if t == nil || fd == nil {
		return
	}
	for _, p := range fd.Params {
		t.vars[p.Name] = p.Type.KindName
	}
}

// noteVar records a declared variable's type.
func (t *typeKinds) noteVar(vd *ast.VarDecl) {
	if t == nil || vd == nil {
		return
	}
	t.vars[vd.Name] = vd.Type.KindName
}

// kindOf reports the AST type kind of an expression, or "" when it cannot tell.
// Callers must treat "" as "not known to be an Int": every decision this type
// drives is a departure from the plain operator, and taking one on a guess is
// worse than leaving the operator alone.
func (t *typeKinds) kindOf(n ast.Node) string {
	if t == nil {
		return ""
	}
	switch node := n.(type) {
	case *ast.Literal:
		return node.ValueType
	case *ast.Ident:
		return t.vars[node.Name]
	case *ast.CallExpr:
		if node.Callee == "sprintf" {
			return "String"
		}
		return t.funcs[node.Callee]
	case *ast.BinaryExpr:
		switch node.Op {
		case "==", "!=", "<", ">", "<=", ">=", "&&", "||":
			return "Bool"
		case "+":
			if t.kindOf(node.Left) == "String" || t.kindOf(node.Right) == "String" {
				return "String"
			}
		}
		if l := t.kindOf(node.Left); l != "" {
			return l
		}
		return t.kindOf(node.Right)
	case *ast.UnaryExpr:
		if node.Op == "!" {
			return "Bool"
		}
		return t.kindOf(node.Operand)
	case *ast.IndexExpr:
		// An element of an Int array is an Int; anything else is unknown.
		if arr := t.kindOf(node.Target); arr == "Array" {
			return ""
		}
		return ""
	default:
		return ""
	}
}

// isIntDivision reports whether a `/` divides one Int by another, which is the
// only case where a backend should reach for its language's integer division.
// Unknown operands answer false: emitting integer division over something that
// turns out to be a Float would silently truncate it.
func (t *typeKinds) isIntDivision(be *ast.BinaryExpr) bool {
	return t.isIntOp(be, "/")
}

// isIntRemainder reports whether a `%` takes the remainder of one Int by
// another. It matters for the same reason division does, one layer down: the
// languages that floor their division also give `%` the sign of the divisor,
// so -7 % 2 is 1 in Python, Ruby, Lua, Tcl and Perl and -1 in C, Go, Java,
// Rust, JavaScript, awk and bash.
func (t *typeKinds) isIntRemainder(be *ast.BinaryExpr) bool {
	return t.isIntOp(be, "%")
}

func (t *typeKinds) isIntOp(be *ast.BinaryExpr, op string) bool {
	if t == nil || be == nil || be.Op != op {
		return false
	}
	return t.kindOf(be.Left) == "Int" && t.kindOf(be.Right) == "Int"
}
