package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateKotlin produces Kotlin source code from the given typed AST.
func GenerateKotlin(root ast.Node) ([]byte, error) {
	g := &ktGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	for i, d := range prog.Decls {
		if i > 0 {
			g.writeln("")
		}
		if err := g.emitNode(d); err != nil {
			return nil, err
		}
	}

	return []byte(g.buf.String()), nil
}

type ktGen struct {
	buf    *strings.Builder
	indent int
	muts   map[string]bool
}

func (g *ktGen) write(s string)   { g.buf.WriteString(s) }
func (g *ktGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *ktGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func typeToKotlin(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "Long"
	case "Float":
		return "Double"
	case "String":
		return "String"
	case "Bool":
		return "Boolean"
	case "Void":
		return "Unit"
	case "Array":
		if t.Elem != nil {
			return "List<" + typeToKotlin(*t.Elem) + ">"
		}
		return "List<Any>"
	case "Option":
		if t.Elem != nil {
			return typeToKotlin(*t.Elem) + "?"
		}
		return "Any?"
	case "Result":
		if t.OkType != nil {
			return "Result<" + typeToKotlin(*t.OkType) + ">"
		}
		return "Result<Any>"
	default:
		return t.KindName
	}
}

// --- Node emitters ---

func (g *ktGen) emitNode(n ast.Node) error {
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

func (g *ktGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)

	g.writeIndent()
	g.write("fun " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + ": " + typeToKotlin(p.Type))
	}
	g.write(")")

	rt := typeToKotlin(fd.ReturnType)
	if rt != "" && rt != "Unit" {
		g.write(": " + rt)
	}

	g.writeln(" {")
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

func (g *ktGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *ktGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if g.muts[vd.Name] {
		g.write("var ")
	} else {
		g.write("val ")
	}
	g.write(vd.Name + ": " + typeToKotlin(vd.Type))
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln("")
	return nil
}

func (g *ktGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	g.write(as.Target + " = ")
	if err := g.emitExpr(as.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *ktGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if (")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(") {")
	g.indent++
	for _, s := range is.Then {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()

	if len(is.Else) > 0 {
		g.writeln("} else {")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
	}
	g.writeln("}")
	return nil
}

func (g *ktGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while (")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(") {")
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

func (g *ktGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

// --- Expression emitters ---

func (g *ktGen) emitExpr(n ast.Node) error {
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
		g.write(" " + node.Op + " ")
		if err := g.emitExpr(node.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	case *ast.UnaryExpr:
		g.write(node.Op)
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

func (g *ktGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("println(")
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
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "sprintf":
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(".toString()")
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

func (g *ktGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write(fmt.Sprintf("%q", s))
	case "Int":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%dL", int64(f)))
	case "Float":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%g", f))
	case "Bool":
		b, _ := lit.Value.(bool)
		g.write(fmt.Sprintf("%t", b))
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}
