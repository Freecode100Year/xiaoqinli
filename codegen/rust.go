package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateRust produces Rust source code from the given typed AST.
func GenerateRust(root ast.Node) ([]byte, error) {
	g := &rsGen{buf: &strings.Builder{}}

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

type rsGen struct {
	buf    *strings.Builder
	indent int
	muts   map[string]bool
}

func (g *rsGen) write(s string)   { g.buf.WriteString(s) }
func (g *rsGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *rsGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func typeToRust(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "i64"
	case "Float":
		return "f64"
	case "String":
		return "String"
	case "Bool":
		return "bool"
	case "Void":
		return ""
	case "Array":
		if t.Elem != nil {
			return "Vec<" + typeToRust(*t.Elem) + ">"
		}
		return "Vec<String>"
	case "Option":
		if t.Elem != nil {
			return "Option<" + typeToRust(*t.Elem) + ">"
		}
		return "Option<String>"
	case "Result":
		ok := "String"
		if t.OkType != nil {
			ok = typeToRust(*t.OkType)
		}
		return "Result<" + ok + ", Box<dyn std::error::Error>>"
	default:
		return t.KindName
	}
}

// --- Node emitters ---

func (g *rsGen) emitNode(n ast.Node) error {
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

func (g *rsGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)

	g.writeIndent()
	g.write("fn " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + ": " + typeToRust(p.Type))
	}
	g.write(")")

	rt := typeToRust(fd.ReturnType)
	if rt != "" {
		g.write(" -> " + rt)
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

func (g *rsGen) emitReturn(rs *ast.ReturnStmt) error {
	g.writeIndent()
	if rs.Value == nil {
		g.writeln("return;")
		return nil
	}
	g.write("return ")
	if err := g.emitExpr(rs.Value); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *rsGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if g.muts[vd.Name] {
		g.write("let mut ")
	} else {
		g.write("let ")
	}
	g.write(vd.Name)
	rt := typeToRust(vd.Type)
	if rt != "" {
		g.write(": " + rt)
	}
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln(";")
	return nil
}

func (g *rsGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	g.write(as.Target + " = ")
	if err := g.emitExpr(as.Value); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *rsGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if ")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(" {")
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

func (g *rsGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while ")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(" {")
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

func (g *rsGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

// --- Expression emitters ---

func (g *rsGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		isStrConcat := node.Op == "+" && (containsStringExpr(node.Left) || containsStringExpr(node.Right))
		g.write("(")
		if err := g.emitExpr(node.Left); err != nil {
			return err
		}
		g.write(" " + node.Op + " ")
		if isStrConcat {
			g.write("&")
		}
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

func (g *rsGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		return g.emitRustPrint("println!", ce.Args)
	case "printf":
		return g.emitRustPrint("print!", ce.Args)
	case "sprintf":
		return g.emitRustPrint("format!", ce.Args)
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

func (g *rsGen) emitRustPrint(macro string, args []ast.Node) error {
	g.write(macro + "(\"")
	for i := range args {
		if i > 0 {
			g.write(" ")
		}
		g.write("{}")
	}
	g.write("\"")
	for _, arg := range args {
		g.write(", ")
		if err := g.emitExpr(arg); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *rsGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write(fmt.Sprintf("String::from(%q)", s))
	case "Int":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%d", int64(f)))
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
