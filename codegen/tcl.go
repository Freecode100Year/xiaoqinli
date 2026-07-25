package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateTcl produces Tcl source code from the given typed AST.
// The "main" function's body is emitted at top level after proc definitions.
func GenerateTcl(root ast.Node) ([]byte, error) {
	g := &tclGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	first := true

	// Emit enum declarations as constants.
	for _, d := range prog.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			if !first {
				g.writeln("")
			}
			if err := g.emitEnumDecl(ed); err != nil {
				return nil, err
			}
			first = false
		}
	}

	// Emit struct declarations (comment only; Tcl uses dicts).
	for _, d := range prog.Decls {
		if sd, ok := d.(*ast.StructDecl); ok {
			if !first {
				g.writeln("")
			}
			if err := g.emitStructDecl(sd); err != nil {
				return nil, err
			}
			first = false
		}
	}

	// Emit non-main proc declarations.
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name == "main" {
			continue
		}
		if !first {
			g.writeln("")
		}
		if err := g.emitFunctionDecl(fd); err != nil {
			return nil, err
		}
		first = false
	}

	// Emit main body at top level.
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		if !first {
			g.writeln("")
		}
		for _, stmt := range fd.Body {
			if err := g.emitNode(stmt); err != nil {
				return nil, err
			}
		}
	}

	return []byte(g.buf.String()), nil
}

type tclGen struct {
	buf    *strings.Builder
	indent int
}

func (g *tclGen) write(s string)   { g.buf.WriteString(s) }
func (g *tclGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *tclGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func (g *tclGen) emitNode(n ast.Node) error {
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
		g.writeln("break")
		return nil
	case *ast.ContinueStmt:
		g.writeIndent()
		g.writeln("continue")
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

func (g *tclGen) emitStructDecl(sd *ast.StructDecl) error {
	// Tcl uses dicts for structs; emit a comment describing the fields.
	g.writeIndent()
	g.write("# struct " + sd.Name + ": ")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name)
	}
	g.writeln("")
	return nil
}

func (g *tclGen) emitEnumDecl(ed *ast.EnumDecl) error {
	for i, v := range ed.Variants {
		g.writeIndent()
		g.writeln(fmt.Sprintf("set %s %d", v, i))
	}
	return nil
}

func (g *tclGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("switch $")
	if ident, ok := me.Value.(*ast.Ident); ok {
		g.write(ident.Name)
	} else {
		g.write("[")
		if err := g.emitExprInline(me.Value); err != nil {
			return err
		}
		g.write("]")
	}
	g.writeln(" {")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.writeln("default {")
		} else {
			if err := g.emitExprInline(arm.Pattern); err != nil {
				return err
			}
			g.writeln(" {")
		}
		g.indent++
		for _, stmt := range arm.Body {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln("}")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *tclGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.writeIndent()
	g.write("proc " + fd.Name + " {")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(" ")
		}
		g.write(p.Name)
	}
	g.writeln("} {")
	g.indent++
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *tclGen) emitReturn(rs *ast.ReturnStmt) error {
	g.writeIndent()
	if rs.Value == nil {
		g.writeln("return")
		return nil
	}
	g.write("return ")
	if err := g.emitExprInline(rs.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *tclGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	g.write("set " + vd.Name + " ")
	if vd.Value != nil {
		if err := g.emitExprInline(vd.Value); err != nil {
			return err
		}
	} else {
		g.write("\"\"")
	}
	g.writeln("")
	return nil
}

func (g *tclGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	// Handle member access (dict set) and index access.
	switch target := as.Target.(type) {
	case *ast.MemberExpr:
		if ident, ok := target.Object.(*ast.Ident); ok {
			g.write("dict set " + ident.Name + " " + target.Field + " ")
		} else {
			g.write("set ")
			if err := g.emitExprInline(as.Target); err != nil {
				return err
			}
			g.write(" ")
		}
	case *ast.IndexExpr:
		if ident, ok := target.Target.(*ast.Ident); ok {
			g.write("lset " + ident.Name + " ")
			if err := g.emitExprInline(target.Index); err != nil {
				return err
			}
			g.write(" ")
		} else {
			g.write("set ")
			if err := g.emitExprInline(as.Target); err != nil {
				return err
			}
			g.write(" ")
		}
	case *ast.Ident:
		g.write("set " + target.Name + " ")
	default:
		g.write("set ")
		if err := g.emitExprInline(as.Target); err != nil {
			return err
		}
		g.write(" ")
	}
	if err := g.emitExprInline(as.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *tclGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if {")
	if err := g.emitCondExpr(is.Cond); err != nil {
		return err
	}
	g.writeln("} {")
	g.indent++
	for _, s := range is.Then {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	if len(is.Else) > 0 {
		g.writeIndent()
		g.writeln("} else {")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
	}
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *tclGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while {")
	if err := g.emitCondExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln("} {")
	g.indent++
	for _, s := range ws.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *tclGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for {set " + fs.Var + " ")
		if err := g.emitExprInline(fs.Start); err != nil {
			return err
		}
		g.write("} {$" + fs.Var + " < ")
		if err := g.emitExprInline(fs.End); err != nil {
			return err
		}
		g.writeln("} {incr " + fs.Var + "} {")
	case "each":
		g.write("foreach " + fs.Var + " ")
		if err := g.emitExprInline(fs.Iterable); err != nil {
			return err
		}
		g.writeln(" {")
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
	g.writeln("}")
	return nil
}

func (g *tclGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExprInline(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

// emitCondExpr writes an expression in a Tcl condition context (inside {}).
func (g *tclGen) emitCondExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.BinaryExpr:
		if err := g.emitCondExpr(node.Left); err != nil {
			return err
		}
		g.write(" " + node.Op + " ")
		return g.emitCondExpr(node.Right)
	case *ast.UnaryExpr:
		g.write(node.Op)
		return g.emitCondExpr(node.Operand)
	case *ast.Ident:
		g.write("$" + node.Name)
		return nil
	case *ast.Literal:
		return g.emitLiteral(node)
	default:
		return g.emitExprInline(n)
	}
}

// emitExprInline writes an expression for inline use (not wrapped in $).
func (g *tclGen) emitExprInline(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write("$" + node.Name)
		return nil
	case *ast.BinaryExpr:
		g.write("[expr {")
		if err := g.emitCondExpr(node); err != nil {
			return err
		}
		g.write("}]")
		return nil
	case *ast.UnaryExpr:
		if node.Op == "!" {
			g.write("[expr {!")
			if err := g.emitCondExpr(node.Operand); err != nil {
				return err
			}
			g.write("}]")
			return nil
		}
		g.write("[expr {" + node.Op)
		if err := g.emitCondExpr(node.Operand); err != nil {
			return err
		}
		g.write("}]")
		return nil
	case *ast.CallExpr:
		return g.emitCall(node)
	case *ast.MemberExpr:
		if ident, ok := node.Object.(*ast.Ident); ok {
			g.write("[dict get $" + ident.Name + " " + node.Field + "]")
		} else {
			g.write("[dict get ")
			if err := g.emitExprInline(node.Object); err != nil {
				return err
			}
			g.write(" " + node.Field + "]")
		}
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

func (g *tclGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("[expr {")
	if err := g.emitCondExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" ? ")
	if err := g.emitCondExpr(ie.Then); err != nil {
		return err
	}
	g.write(" : ")
	if err := g.emitCondExpr(ie.Else); err != nil {
		return err
	}
	g.write("}]")
	return nil
}

func (g *tclGen) emitLambda(lam *ast.Lambda) error {
	return fmt.Errorf("XQL_E401: Tcl does not support Lambda expressions")
}

func (g *tclGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("[list")
	for _, elem := range al.Elements {
		g.write(" ")
		if err := g.emitExprInline(elem); err != nil {
			return err
		}
	}
	g.write("]")
	return nil
}

func (g *tclGen) emitIndexExpr(ie *ast.IndexExpr) error {
	g.write("[lindex ")
	if err := g.emitExprInline(ie.Target); err != nil {
		return err
	}
	g.write(" ")
	if err := g.emitExprInline(ie.Index); err != nil {
		return err
	}
	g.write("]")
	return nil
}

func (g *tclGen) emitStructLit(sl *ast.StructLit) error {
	g.write("[dict create")
	for _, f := range sl.Fields {
		g.write(" " + f.Name + " ")
		if err := g.emitExprInline(f.Value); err != nil {
			return err
		}
	}
	g.write("]")
	return nil
}

func (g *tclGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("puts ")
		if len(ce.Args) > 0 {
			if err := g.emitExprInline(ce.Args[0]); err != nil {
				return err
			}
		} else {
			g.write("\"\"")
		}
		return nil
	case "printf":
		if len(ce.Args) >= 2 {
			g.write("puts -nonewline [format ")
			for _, arg := range ce.Args {
				g.write(" ")
				if err := g.emitExprInline(arg); err != nil {
					return err
				}
			}
			g.write("]")
		} else {
			g.write("puts -nonewline ")
			if len(ce.Args) > 0 {
				if err := g.emitExprInline(ce.Args[0]); err != nil {
					return err
				}
			}
		}
		return nil
	case "sprintf":
		if len(ce.Args) >= 2 {
			g.write("[format ")
			for _, arg := range ce.Args {
				g.write(" ")
				if err := g.emitExprInline(arg); err != nil {
					return err
				}
			}
			g.write("]")
		} else {
			g.write("[format \"%s\" ")
			if len(ce.Args) > 0 {
				if err := g.emitExprInline(ce.Args[0]); err != nil {
					return err
				}
			}
			g.write("]")
		}
		return nil
	default:
		g.write("[" + ce.Callee)
		for _, arg := range ce.Args {
			g.write(" ")
			if err := g.emitExprInline(arg); err != nil {
				return err
			}
		}
		g.write("]")
		return nil
	}
}

func (g *tclGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write(fmt.Sprintf("%q", s))
	case "Int":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%d", int64(f)))
	case "Float":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%g", f))
	case "Bool":
		b, _ := lit.Value.(bool)
		if b {
			g.write("1")
		} else {
			g.write("0")
		}
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}
