package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateFortran produces modern Fortran (F90+) source code from the given typed AST.
// The "main" function's body is emitted inside a `program main ... end program main` block.
func GenerateFortran(root ast.Node) ([]byte, error) {
	g := &fortranGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Find and emit the main program block.
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		if err := g.emitMainBlock(fd, prog); err != nil {
			return nil, err
		}
	}

	return []byte(g.buf.String()), nil
}

type fortranGen struct {
	buf    *strings.Builder
	indent int
}

func (g *fortranGen) write(s string)   { g.buf.WriteString(s) }
func (g *fortranGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *fortranGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func typeToFortran(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		// Default `integer` is 32-bit in every Fortran compiler anyone uses, so
		// 2147483647 + 1 wrapped to -2147483648 and 100000 * 100000 to
		// 1410065408 — silently, since the arithmetic is legal. Int is 64-bit
		// in this AST; integer(8) is the kind that says so.
		return "integer(8)"
	case "Float":
		return "real(8)"
	case "String":
		return "character(len=256)"
	case "Bool":
		return "logical"
	case "Void":
		return ""
	case "Array":
		if t.Elem != nil {
			return typeToFortran(*t.Elem) + ", dimension(:), allocatable"
		}
		return "integer(8), dimension(:), allocatable"
	default:
		return "type(" + t.KindName + ")"
	}
}

// fortranParamType is typeToFortran for a dummy argument, where a character's
// length must be assumed rather than fixed.
//
// A local String is `character(len=256)` because it has to be some length, and
// assignment blank-pads it. A *dummy argument* declared that way is a different
// thing entirely: Fortran passes character length as a hidden argument and the
// callee believes the declaration, so `greet('World')` — five characters
// against a dummy declared 256 — let the body read 251 bytes past the end of
// the actual argument. That is what it did: hello.xql.json printed
// "Hello, World" followed by a page of heap, including fragments of libm's
// error-message table. gfortran -fsyntax-only cannot see it, which is why the
// defect survived the compiled tier.
//
// len=* takes the length from the caller, which is what every other backend
// means by passing a string.
func fortranParamType(t ast.TypeExpr) string {
	if t.KindName == "String" {
		return "character(len=*)"
	}
	return typeToFortran(t)
}

func (g *fortranGen) emitMainBlock(fd *ast.FunctionDecl, prog *ast.Program) error {
	g.writeln("program main")
	g.indent++
	g.writeIndent()
	g.writeln("implicit none")

	// Emit type declarations (structs and enums) inside the program.
	for _, d := range prog.Decls {
		switch node := d.(type) {
		case *ast.StructDecl:
			g.writeln("")
			if err := g.emitStructDecl(node); err != nil {
				return err
			}
		case *ast.EnumDecl:
			g.writeln("")
			if err := g.emitEnumDecl(node); err != nil {
				return err
			}
		}
	}

	// Collect and emit variable declarations.
	vars := collectVarDecls(fd.Body)
	forVars := collectForVars(fd.Body)
	declaredNames := map[string]bool{}
	for _, vd := range vars {
		declaredNames[vd.Name] = true
		g.writeIndent()
		g.writeln(typeToFortran(vd.Type) + " :: " + vd.Name)
	}
	for _, name := range forVars {
		if !declaredNames[name] {
			g.writeIndent()
			// integer(8) like every other Int here: a loop bound is an Int
			// expression, and a counter one kind narrower than the values it
			// is compared against is the same overflow one step removed.
			g.writeln("integer(8) :: " + name)
		}
	}

	g.writeln("")
	// Emit executable statements.
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}

	// Emit contained functions/subroutines.
	hasFuncs := false
	for _, d := range prog.Decls {
		fdn, ok := d.(*ast.FunctionDecl)
		if !ok || fdn.Name == "main" {
			continue
		}
		if !hasFuncs {
			g.writeln("")
			g.writeIndent()
			g.writeln("contains")
			g.writeln("")
			hasFuncs = true
		}
		if err := g.emitFunctionDecl(fdn); err != nil {
			return err
		}
		g.writeln("")
	}

	g.indent--
	g.writeln("end program main")
	return nil
}

func (g *fortranGen) emitNode(n ast.Node) error {
	switch node := n.(type) {
	case *ast.FunctionDecl:
		return g.emitFunctionDecl(node)
	case *ast.ReturnStmt:
		return g.emitReturn(node)
	case *ast.VarDecl:
		return g.emitVarDecl(node)
	case *ast.AssignStmt:
		return g.emitAssign(node)
	case *ast.IfStmt:
		return g.emitIf(node)
	case *ast.WhileStmt:
		return g.emitWhile(node)
	case *ast.ForStmt:
		return g.emitForStmt(node)
	case *ast.BreakStmt:
		g.writeIndent()
		g.writeln("exit")
		return nil
	case *ast.ContinueStmt:
		g.writeIndent()
		g.writeln("cycle")
		return nil
	case *ast.ExprStmt:
		return g.emitExprStmt(node)
	case *ast.StructDecl:
		return g.emitStructDecl(node)
	case *ast.EnumDecl:
		return g.emitEnumDecl(node)
	case *ast.MatchExpr:
		return g.emitMatchExpr(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *fortranGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("type :: " + sd.Name)
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(typeToFortran(f.Type) + " :: " + f.Name)
	}
	g.indent--
	g.writeIndent()
	g.writeln("end type " + sd.Name)
	return nil
}

func (g *fortranGen) emitEnumDecl(ed *ast.EnumDecl) error {
	// Modern Fortran uses integer parameters for enum-like behavior.
	for i, v := range ed.Variants {
		g.writeIndent()
		g.writeln(fmt.Sprintf("integer, parameter :: %s_%s = %d", ed.Name, v, i))
	}
	return nil
}

func (g *fortranGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	rt := typeToFortran(fd.ReturnType)
	isSubroutine := rt == ""

	g.writeIndent()
	// Fortran 90 and 95 require the RECURSIVE prefix before a procedure may
	// call itself; without it gfortran refuses with "cannot be called
	// recursively". The keyword needs a RESULT clause, which functions here
	// always have, and costs nothing on a procedure that never recurses — so
	// rather than analysing the call graph, declare it and be correct.
	if isSubroutine {
		g.write("recursive subroutine " + fd.Name + "(")
	} else {
		g.write("recursive function " + fd.Name + "(")
	}
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name)
	}
	if isSubroutine {
		g.writeln(")")
	} else {
		g.writeln(") result(res)")
	}

	g.indent++
	g.writeIndent()
	g.writeln("implicit none")

	// Emit parameter type declarations.
	for _, p := range fd.Params {
		g.writeIndent()
		g.writeln(fortranParamType(p.Type) + ", intent(in) :: " + p.Name)
	}

	// Emit result type declaration for functions.
	if !isSubroutine {
		g.writeIndent()
		g.writeln(rt + " :: res")
	}

	// Collect and emit variable declarations.
	vars := collectVarDecls(fd.Body)
	forVars := collectForVars(fd.Body)
	declaredNames := map[string]bool{}
	for _, vd := range vars {
		declaredNames[vd.Name] = true
		g.writeIndent()
		g.writeln(typeToFortran(vd.Type) + " :: " + vd.Name)
	}
	for _, name := range forVars {
		if !declaredNames[name] {
			g.writeIndent()
			// integer(8) like every other Int here: a loop bound is an Int
			// expression, and a counter one kind narrower than the values it
			// is compared against is the same overflow one step removed.
			g.writeln("integer(8) :: " + name)
		}
	}

	g.writeln("")
	// Emit executable statements.
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}

	g.indent--
	g.writeIndent()
	if isSubroutine {
		g.writeln("end subroutine " + fd.Name)
	} else {
		g.writeln("end function " + fd.Name)
	}
	return nil
}

func (g *fortranGen) emitReturn(rs *ast.ReturnStmt) error {
	if rs.Value != nil {
		g.writeIndent()
		g.write("res = ")
		if err := g.emitExpr(rs.Value); err != nil {
			return err
		}
		g.writeln("")
	}
	g.writeIndent()
	g.writeln("return")
	return nil
}

func (g *fortranGen) emitVarDecl(vd *ast.VarDecl) error {
	// Variable declaration is in the declaration section; here we only emit the assignment if there is a value.
	if vd.Value != nil {
		g.writeIndent()
		g.write(vd.Name + " = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
		g.writeln("")
	}
	return nil
}

func (g *fortranGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	if err := g.emitExpr(as.Target); err != nil {
		return err
	}
	g.write(" = ")
	if err := g.emitExpr(as.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *fortranGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if (")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(") then")
	g.indent++
	for _, s := range is.Then {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	if len(is.Else) > 0 {
		g.writeIndent()
		g.writeln("else")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
	}
	g.writeIndent()
	g.writeln("end if")
	return nil
}

func (g *fortranGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("do while (")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(")")
	g.indent++
	for _, s := range ws.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("end do")
	return nil
}

func (g *fortranGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("do " + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write(", ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(" - 1")
	case "each":
		return fmt.Errorf("XQL_E402: Fortran target does not support for-each loops")
	default:
		return fmt.Errorf("XQL_E401: unsupported for-loop form %q", fs.Form)
	}
	g.indent++
	for _, s := range fs.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("end do")
	return nil
}

func (g *fortranGen) emitExprStmt(es *ast.ExprStmt) error {
	if ce, ok := es.Expr.(*ast.CallExpr); ok {
		switch ce.Callee {
		case "println", "printf", "sprintf":
			g.writeIndent()
			if err := g.emitExpr(es.Expr); err != nil {
				return err
			}
			g.writeln("")
			return nil
		}
	}
	g.writeIndent()
	g.write("call ")
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *fortranGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("select case (")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(")")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.writeln("case default")
		} else {
			g.write("case (")
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.writeln(")")
		}
		g.indent++
		for _, s := range arm.Body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
	}
	g.indent--
	g.writeIndent()
	g.writeln("end select")
	return nil
}

func (g *fortranGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		if node.Op == "%" {
			g.write("mod(")
			if err := g.emitExpr(node.Left); err != nil {
				return err
			}
			g.write(", ")
			if err := g.emitExpr(node.Right); err != nil {
				return err
			}
			g.write(")")
			return nil
		}
		if node.Op == "+" && containsStringExpr(node) {
			return g.emitConcat(node)
		}
		g.write("(")
		if err := g.emitExpr(node.Left); err != nil {
			return err
		}
		op := node.Op
		switch op {
		case "&&":
			op = ".and."
		case "||":
			op = ".or."
		case "!=":
			op = "/="
		case "==":
			op = "=="
		}
		g.write(" " + op + " ")
		if err := g.emitExpr(node.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	case *ast.UnaryExpr:
		if node.Op == "!" {
			g.write(".not. ")
		} else {
			g.write(node.Op)
		}
		return g.emitExpr(node.Operand)
	case *ast.CallExpr:
		return g.emitCall(node)
	case *ast.MemberExpr:
		if err := g.emitExpr(node.Object); err != nil {
			return err
		}
		g.write("%" + node.Field)
		return nil
	case *ast.StructLit:
		return g.emitStructLit(node)
	case *ast.ArrayLit:
		return g.emitArrayLit(node)
	case *ast.IndexExpr:
		return g.emitIndexExpr(node)
	case *ast.IfExpr:
		return g.emitIfExpr(node)
	case *ast.Lambda:
		return g.emitLambda(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported expression %s", n.Kind())
	}
}

func (g *fortranGen) emitIfExpr(ie *ast.IfExpr) error {
	// MERGE requires both arms to have the same type, kind and — for
	// characters — the same length, so merge('big', 'small', c) is rejected
	// outright. Fortran characters are fixed-length and blank-padded anyway, so
	// padding the shorter literal to match is what the language would have done
	// on assignment; it is not a change in meaning.
	thenLit, thenOK := stringLiteralOf(ie.Then)
	elseLit, elseOK := stringLiteralOf(ie.Else)
	if thenOK && elseOK && len(thenLit) != len(elseLit) {
		width := len(thenLit)
		if len(elseLit) > width {
			width = len(elseLit)
		}
		g.write("merge(")
		g.write(fortranPaddedString(thenLit, width))
		g.write(", ")
		g.write(fortranPaddedString(elseLit, width))
		g.write(", ")
		if err := g.emitExpr(ie.Cond); err != nil {
			return err
		}
		g.write(")")
		return nil
	}

	g.write("merge(")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(", ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(", ")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(")")
	return nil
}

// emitConcat writes `//` with every non-literal operand trimmed.
//
// A Fortran String is a fixed-length buffer and assignment blank-pads it, so a
// variable holding "x" is really "x" followed by 255 spaces. Concatenating that
// produces 257 characters, and assigning them back to a 256-character variable
// throws away everything past the padding — so `s = s + "ab"` in a loop left s
// as "x" no matter how many times it ran. string_build.xql.json printed x where
// every other target printed xababab.
//
// trim drops the padding, which is the only reading of these strings that
// matches what the other backends do. It also means a String whose value ends
// in a space cannot survive a concatenation here — but a fixed-length buffer
// could not tell that space from its own padding in the first place.
func (g *fortranGen) emitConcat(node *ast.BinaryExpr) error {
	g.write("(")
	if err := g.emitConcatOperand(node.Left); err != nil {
		return err
	}
	g.write(" // ")
	if err := g.emitConcatOperand(node.Right); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *fortranGen) emitConcatOperand(n ast.Node) error {
	// A literal has no padding to trim, and a nested concatenation has already
	// trimmed its own operands.
	if lit, ok := n.(*ast.Literal); ok && lit.ValueType == "String" {
		return g.emitLiteral(lit)
	}
	if be, ok := n.(*ast.BinaryExpr); ok && be.Op == "+" && containsStringExpr(be) {
		return g.emitConcat(be)
	}
	g.write("trim(")
	if err := g.emitExpr(n); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *fortranGen) emitLambda(lam *ast.Lambda) error {
	return fmt.Errorf("XQL_E401: Fortran does not support Lambda expressions")
}

func (g *fortranGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("(/")
	for i, elem := range al.Elements {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(elem); err != nil {
			return err
		}
	}
	g.write("/)")
	return nil
}

func (g *fortranGen) emitIndexExpr(ie *ast.IndexExpr) error {
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write("(")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write(" + 1)")
	return nil
}

func (g *fortranGen) emitStructLit(sl *ast.StructLit) error {
	g.write(sl.TypeName + "(")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + "=")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *fortranGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("print *, ")
		if len(ce.Args) == 0 {
			g.write("''")
		} else {
			for i, arg := range ce.Args {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
		}
		return nil
	case "printf":
		g.write("write(*, '(A)', advance='no') ")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		return nil
	case "sprintf":
		// Fortran: write to internal string. Limited support.
		if len(ce.Args) > 0 {
			g.write("transfer(")
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(", '')")
		}
		return nil
	default:
		g.write(ce.Callee + "(")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(", ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	}
}

func (g *fortranGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write("'" + strings.ReplaceAll(s, "'", "''") + "'")
	case "Int":
		// `_8` is the kind suffix, and it is required rather than tidy: Fortran
		// checks argument kinds exactly, so `add(3, 5)` against a dummy declared
		// integer(8) is "Type mismatch in argument 'a': passed INTEGER(4) to
		// INTEGER(8)" and does not compile. The same suffix keeps `100000 *
		// 100000` from overflowing in the default kind before it is stored.
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%d_8", int64(f)))
	case "Float":
		f, _ := lit.Value.(float64)
		s := fmt.Sprintf("%g", f)
		if !strings.Contains(s, ".") && !strings.Contains(s, "e") && !strings.Contains(s, "E") {
			s += ".0"
		}
		g.write(s)
	case "Bool":
		b, _ := lit.Value.(bool)
		if b {
			g.write(".true.")
		} else {
			g.write(".false.")
		}
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}

// stringLiteralOf reports the value of a string literal node.
func stringLiteralOf(n ast.Node) (string, bool) {
	lit, ok := n.(*ast.Literal)
	if !ok || lit.ValueType != "String" {
		return "", false
	}
	s, ok := lit.Value.(string)
	return s, ok
}

// fortranPaddedString renders a string literal blank-padded to width, quoting
// it the way the rest of this backend does.
func fortranPaddedString(s string, width int) string {
	padded := s + strings.Repeat(" ", width-len(s))
	return "'" + strings.ReplaceAll(padded, "'", "''") + "'"
}
