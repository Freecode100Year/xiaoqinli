package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GeneratePython produces Python source code from the given typed AST.
func GeneratePython(root ast.Node) ([]byte, error) {
	g := &pyGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	hasMain := false
	for i, d := range prog.Decls {
		if i > 0 {
			g.writeln("")
			g.writeln("")
		}
		if err := g.emitNode(d); err != nil {
			return nil, err
		}
		if fd, ok := d.(*ast.FunctionDecl); ok && fd.Name == "main" {
			hasMain = true
		}
	}

	if hasMain {
		g.writeln("")
		g.writeln("")
		g.writeln("if __name__ == \"__main__\":")
		g.writeln("    main()")
	}
	return []byte(g.buf.String()), nil
}

type pyGen struct {
	buf    *strings.Builder
	indent int
}

func (g *pyGen) write(s string)   { g.buf.WriteString(s) }
func (g *pyGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *pyGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func typeToPython(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "int"
	case "Float":
		return "float"
	case "String":
		return "str"
	case "Bool":
		return "bool"
	case "Void":
		return "None"
	case "Array":
		if t.Elem != nil {
			return "list[" + typeToPython(*t.Elem) + "]"
		}
		return "list"
	case "Option":
		if t.Elem != nil {
			return typeToPython(*t.Elem) + " | None"
		}
		return "object | None"
	case "Result":
		if t.OkType != nil {
			return "tuple[" + typeToPython(*t.OkType) + ", Exception | None]"
		}
		return "tuple[object, Exception | None]"
	default:
		return t.KindName
	}
}

// --- Node emitters ---

func (g *pyGen) emitNode(n ast.Node) error {
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
	case *ast.ExprStmt:
		return g.emitExprStmt(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *pyGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.writeIndent()
	g.write("def " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + ": " + typeToPython(p.Type))
	}
	g.write(")")

	rt := typeToPython(fd.ReturnType)
	g.write(" -> " + rt)
	g.writeln(":")

	g.indent++
	if len(fd.Body) == 0 {
		g.writeIndent()
		g.writeln("pass")
	} else {
		for _, stmt := range fd.Body {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
	}
	g.indent--
	return nil
}

func (g *pyGen) emitReturn(rs *ast.ReturnStmt) error {
	g.writeIndent()
	if rs.Value == nil {
		g.writeln("return")
		return nil
	}
	g.write("return ")
	if err := g.emitExpr(rs.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *pyGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	g.write(vd.Name + ": " + typeToPython(vd.Type))
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln("")
	return nil
}

func (g *pyGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	g.write(as.Target + " = ")
	if err := g.emitExpr(as.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *pyGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if ")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(":")
	g.indent++
	if len(is.Then) == 0 {
		g.writeIndent()
		g.writeln("pass")
	} else {
		for _, s := range is.Then {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
	}
	g.indent--

	if len(is.Else) > 0 {
		g.writeIndent()
		g.writeln("else:")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
	}
	return nil
}

func (g *pyGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while ")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(":")
	g.indent++
	if len(ws.Body) == 0 {
		g.writeIndent()
		g.writeln("pass")
	} else {
		for _, s := range ws.Body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
	}
	g.indent--
	return nil
}

func (g *pyGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

// --- Expression emitters ---

func (g *pyGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		g.write("(")
		if err := g.emitExpr(node.Left); err != nil {
			return err
		}
		op := node.Op
		if op == "&&" {
			op = "and"
		} else if op == "||" {
			op = "or"
		}
		g.write(" " + op + " ")
		if err := g.emitExpr(node.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	case *ast.UnaryExpr:
		op := node.Op
		if op == "!" {
			op = "not "
		}
		g.write(op)
		return g.emitExpr(node.Operand)
	case *ast.CallExpr:
		return g.emitCall(node)
	case *ast.MemberExpr:
		if err := g.emitExpr(node.Object); err != nil {
			return err
		}
		g.write("." + node.Field)
		return nil
	default:
		return fmt.Errorf("XQL_E401: unsupported expression %s", n.Kind())
	}
}

func (g *pyGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("print(")
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
	case "printf":
		g.write("print(")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(", ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(", end=\"\")")
		return nil
	case "sprintf":
		g.write("str(")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(")")
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

func (g *pyGen) emitLiteral(lit *ast.Literal) error {
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
			g.write("True")
		} else {
			g.write("False")
		}
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}
