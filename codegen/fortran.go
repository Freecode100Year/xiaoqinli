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
func (g *fortranGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func typeToFortran(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "integer"
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
		return "integer, dimension(:), allocatable"
	default:
		return "type(" + t.KindName + ")"
	}
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
	if len(vars) > 0 {
		for _, vd := range vars {
			g.writeIndent()
			g.writeln(typeToFortran(vd.Type) + " :: " + vd.Name)
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
	if isSubroutine {
		g.write("subroutine " + fd.Name + "(")
	} else {
		g.write("function " + fd.Name + "(")
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
		g.writeln(typeToFortran(p.Type) + ", intent(in) :: " + p.Name)
	}

	// Emit result type declaration for functions.
	if !isSubroutine {
		g.writeIndent()
		g.writeln(rt + " :: res")
	}

	// Collect and emit variable declarations.
	vars := collectVarDecls(fd.Body)
	for _, vd := range vars {
		g.writeIndent()
		g.writeln(typeToFortran(vd.Type) + " :: " + vd.Name)
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
		if op == "+" && containsStringExpr(node) {
			op = "//"
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
	g.write(")")
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
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%d", int64(f)))
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
