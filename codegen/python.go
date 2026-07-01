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

	var out strings.Builder
	if g.needEnum {
		out.WriteString("from enum import Enum\n\n")
	}
	if g.needDataclass {
		out.WriteString("from dataclasses import dataclass\n\n")
	}
	out.WriteString(g.buf.String())
	return []byte(out.String()), nil
}

type pyGen struct {
	buf           *strings.Builder
	indent        int
	needDataclass bool
	needEnum      bool
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

func (g *pyGen) emitStructDecl(sd *ast.StructDecl) error {
	g.needDataclass = true
	g.writeIndent()
	g.writeln("@dataclass")
	g.writeIndent()
	g.writeln("class " + sd.Name + ":")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(f.Name + ": " + typeToPython(f.Type))
	}
	g.indent--
	return nil
}

func (g *pyGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.needEnum = true
	g.writeIndent()
	g.writeln("class " + ed.Name + "(Enum):")
	g.indent++
	for _, v := range ed.Variants {
		g.writeIndent()
		g.writeln(v + " = " + fmt.Sprintf("%q", v))
	}
	g.indent--
	return nil
}

func (g *pyGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("match ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(":")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		g.write("case ")
		if err := g.emitExpr(arm.Pattern); err != nil {
			return err
		}
		g.writeln(":")
		g.indent++
		if len(arm.Body) == 0 {
			g.writeIndent()
			g.writeln("pass")
		} else {
			for _, stmt := range arm.Body {
				if err := g.emitNode(stmt); err != nil {
					return err
				}
			}
		}
		g.indent--
	}
	g.indent--
	return nil
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

func (g *pyGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for " + fs.Var + " in range(")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write(", ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.write(")")
	case "each":
		g.write("for " + fs.Var + " in ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
	default:
		return fmt.Errorf("XQL_E401: unknown ForStmt form %q", fs.Form)
	}
	g.writeln(":")
	g.indent++
	if len(fs.Body) == 0 {
		g.writeIndent()
		g.writeln("pass")
	} else {
		for _, s := range fs.Body {
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

func (g *pyGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("(")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" if ")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" else ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *pyGen) emitLambda(lam *ast.Lambda) error {
	g.write("lambda ")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name)
	}
	g.write(": ")
	if len(lam.Body) > 0 {
		last := lam.Body[len(lam.Body)-1]
		if es, ok := last.(*ast.ExprStmt); ok {
			if err := g.emitExpr(es.Expr); err != nil {
				return err
			}
		} else if rs, ok := last.(*ast.ReturnStmt); ok && rs.Value != nil {
			if err := g.emitExpr(rs.Value); err != nil {
				return err
			}
		} else {
			if err := g.emitExpr(last); err != nil {
				return err
			}
		}
	} else {
		g.write("None")
	}
	return nil
}

func (g *pyGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("[")
	for i, elem := range al.Elements {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(elem); err != nil {
			return err
		}
	}
	g.write("]")
	return nil
}

func (g *pyGen) emitIndexExpr(ie *ast.IndexExpr) error {
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write("[")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write("]")
	return nil
}

func (g *pyGen) emitStructLit(sl *ast.StructLit) error {
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
		if len(ce.Args) >= 2 {
			g.write("print(")
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(" % (")
			for i, arg := range ce.Args[1:] {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write(",), end=\"\")")
		} else {
			g.write("print(")
			if len(ce.Args) > 0 {
				if err := g.emitExpr(ce.Args[0]); err != nil {
					return err
				}
			}
			g.write(", end=\"\")")
		}
		return nil
	case "sprintf":
		if len(ce.Args) >= 2 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(" % (")
			for i, arg := range ce.Args[1:] {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write(",)")
		} else if len(ce.Args) > 0 {
			g.write("str(")
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(")")
		} else {
			g.write("\"\"")
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
