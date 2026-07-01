package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

func GenerateDart(root ast.Node) ([]byte, error) {
	g := &dartGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Emit enum declarations first.
	first := true
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

	for _, d := range prog.Decls {
		if _, ok := d.(*ast.EnumDecl); ok {
			continue
		}
		if !first {
			g.writeln("")
		}
		if err := g.emitNode(d); err != nil {
			return nil, err
		}
		first = false
	}

	var out strings.Builder
	if g.needSprintf {
		out.WriteString("String _xqlSprintf(String fmt, List<Object> args) {\n")
		out.WriteString("    var sb = StringBuffer();\n")
		out.WriteString("    int ai = 0;\n")
		out.WriteString("    for (int i = 0; i < fmt.length; i++) {\n")
		out.WriteString("        if (fmt[i] == '%' && i + 1 < fmt.length && 'sdfo'.contains(fmt[i + 1])) {\n")
		out.WriteString("            sb.write(args[ai++]);\n")
		out.WriteString("            i++;\n")
		out.WriteString("        } else {\n")
		out.WriteString("            sb.write(fmt[i]);\n")
		out.WriteString("        }\n")
		out.WriteString("    }\n")
		out.WriteString("    return sb.toString();\n")
		out.WriteString("}\n\n")
	}
	out.WriteString(g.buf.String())
	return []byte(out.String()), nil
}

type dartGen struct {
	buf         *strings.Builder
	indent      int
	muts        map[string]bool
	needSprintf bool
}

func (g *dartGen) write(s string)   { g.buf.WriteString(s) }
func (g *dartGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *dartGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func typeToDart(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "int"
	case "Float":
		return "double"
	case "String":
		return "String"
	case "Bool":
		return "bool"
	case "Void":
		return "void"
	case "Array":
		if t.Elem != nil {
			return "List<" + typeToDart(*t.Elem) + ">"
		}
		return "List<dynamic>"
	case "Option":
		if t.Elem != nil {
			return typeToDart(*t.Elem) + "?"
		}
		return "dynamic"
	case "Result":
		if t.OkType != nil {
			return typeToDart(*t.OkType)
		}
		return "dynamic"
	default:
		return t.KindName
	}
}

func (g *dartGen) emitNode(n ast.Node) error {
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
		g.writeln("break;")
		return nil
	case *ast.ContinueStmt:
		g.writeIndent()
		g.writeln("continue;")
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

func (g *dartGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.write("enum " + ed.Name + " { ")
	for i, v := range ed.Variants {
		if i > 0 {
			g.write(", ")
		}
		g.write(v)
	}
	g.writeln(" }")
	return nil
}

func (g *dartGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("switch (")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(") {")
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.writeln("default:")
		} else {
			g.write("case ")
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.writeln(":")
		}
		g.indent++
		for _, s := range arm.Body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.writeIndent()
		g.writeln("break;")
		g.indent--
	}
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *dartGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("class " + sd.Name + " {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln("final " + typeToDart(f.Type) + " " + f.Name + ";")
	}
	g.writeIndent()
	g.write(sd.Name + "({")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write("required this." + f.Name)
	}
	g.writeln("});")
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *dartGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)
	g.writeIndent()
	rt := typeToDart(fd.ReturnType)
	g.write(rt + " " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(typeToDart(p.Type) + " " + p.Name)
	}
	g.writeln(") {")
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

func (g *dartGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *dartGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if g.muts[vd.Name] {
		g.write(typeToDart(vd.Type) + " " + vd.Name)
	} else {
		g.write("final " + typeToDart(vd.Type) + " " + vd.Name)
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

func (g *dartGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	if err := g.emitExpr(as.Target); err != nil {
		return err
	}
	g.write(" = ")
	if err := g.emitExpr(as.Value); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *dartGen) emitIf(is *ast.IfStmt) error {
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

func (g *dartGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *dartGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for (int " + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("; " + fs.Var + " < ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln("; " + fs.Var + "++) {")
	case "each":
		g.write("for (final " + fs.Var + " in ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(") {")
	default:
		return fmt.Errorf("XQL_E401: unknown ForStmt form %q", fs.Form)
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

func (g *dartGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *dartGen) emitExpr(n ast.Node) error {
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

func (g *dartGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("(")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" ? ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" : ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *dartGen) emitLambda(lam *ast.Lambda) error {
	g.write("(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(typeToDart(p.Type) + " " + p.Name)
	}
	g.write(")")
	if len(lam.Body) == 1 {
		if rs, ok := lam.Body[0].(*ast.ReturnStmt); ok && rs.Value != nil {
			g.write(" => ")
			return g.emitExpr(rs.Value)
		}
		if es, ok := lam.Body[0].(*ast.ExprStmt); ok {
			g.write(" => ")
			return g.emitExpr(es.Expr)
		}
	}
	g.writeln(" {")
	g.indent++
	for _, stmt := range lam.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.write("}")
	return nil
}

func (g *dartGen) emitArrayLit(al *ast.ArrayLit) error {
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

func (g *dartGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *dartGen) emitStructLit(sl *ast.StructLit) error {
	g.write(sl.TypeName + "(")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + ": ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *dartGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("print(")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(` + " " + `)
			}
			if i == 0 && len(ce.Args) > 1 {
				g.write(`"" + `)
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "printf":
		if len(ce.Args) >= 2 {
			g.needSprintf = true
			g.write("stdout.write(_xqlSprintf(")
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(", [")
			for i, arg := range ce.Args[1:] {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write("]))")
		} else {
			g.write("stdout.write(")
			if len(ce.Args) > 0 {
				if err := g.emitExpr(ce.Args[0]); err != nil {
					return err
				}
			}
			g.write(")")
		}
		return nil
	case "sprintf":
		if len(ce.Args) >= 2 {
			g.needSprintf = true
			g.write("_xqlSprintf(")
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(", [")
			for i, arg := range ce.Args[1:] {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write("])")
		} else if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(".toString()")
		} else {
			g.write(`""`)
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

func (g *dartGen) emitLiteral(lit *ast.Literal) error {
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
		g.write(fmt.Sprintf("%t", b))
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}
