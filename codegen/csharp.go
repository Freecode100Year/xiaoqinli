package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateCSharp produces C# source code from the given typed AST.
// All functions are emitted as static methods inside a Program class.
func GenerateCSharp(root ast.Node) ([]byte, error) {
	g := &csGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	g.indent = 1

	// Emit enum declarations first (inside the class, before methods).
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
	out.WriteString("using System;\n")
	if g.needList {
		out.WriteString("using System.Collections.Generic;\n")
	}
	out.WriteString("\nclass Program {\n")
	out.WriteString(g.buf.String())
	out.WriteString("}\n")

	return []byte(out.String()), nil
}

type csGen struct {
	buf      *strings.Builder
	indent   int
	muts     map[string]bool
	needList bool
}

func (g *csGen) write(s string)   { g.buf.WriteString(s) }
func (g *csGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *csGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func (g *csGen) typeStr(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "long"
	case "Float":
		return "double"
	case "String":
		return "string"
	case "Bool":
		return "bool"
	case "Void":
		return "void"
	case "Array":
		g.needList = true
		if t.Elem != nil {
			return "List<" + g.typeStr(*t.Elem) + ">"
		}
		return "List<object>"
	case "Option":
		if t.Elem != nil {
			return g.typeStr(*t.Elem) + "?"
		}
		return "object?"
	case "Result":
		if t.OkType != nil {
			return g.typeStr(*t.OkType)
		}
		return "object"
	default:
		return t.KindName
	}
}

// --- Node emitters ---

func (g *csGen) emitNode(n ast.Node) error {
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

func (g *csGen) emitEnumDecl(ed *ast.EnumDecl) error {
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

func (g *csGen) emitMatchExpr(me *ast.MatchExpr) error {
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

func (g *csGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.write("record " + sd.Name + "(")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(g.typeStr(f.Type) + " " + f.Name)
	}
	g.writeln(");")
	return nil
}

func (g *csGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)

	g.writeIndent()
	if fd.Name == "main" {
		g.writeln("static void Main() {")
	} else {
		rt := g.typeStr(fd.ReturnType)
		g.write("static " + rt + " " + fd.Name + "(")
		for i, p := range fd.Params {
			if i > 0 {
				g.write(", ")
			}
			g.write(g.typeStr(p.Type) + " " + p.Name)
		}
		g.writeln(") {")
	}

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

func (g *csGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *csGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	g.write(g.typeStr(vd.Type) + " " + vd.Name)
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln(";")
	return nil
}

func (g *csGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *csGen) emitIf(is *ast.IfStmt) error {
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

func (g *csGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *csGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	if fs.Form == "range" {
		g.write("for (long " + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("; " + fs.Var + " < ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln("; " + fs.Var + "++) {")
	} else {
		g.write("foreach (var " + fs.Var + " in ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(") {")
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

func (g *csGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

// --- Expression emitters ---

func (g *csGen) emitExpr(n ast.Node) error {
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

func (g *csGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *csGen) emitLambda(lam *ast.Lambda) error {
	g.write("(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(g.typeStr(p.Type) + " " + p.Name)
	}
	g.write(") => {")
	for _, stmt := range lam.Body {
		g.write(" ")
		if es, ok := stmt.(*ast.ExprStmt); ok {
			if err := g.emitExpr(es.Expr); err != nil {
				return err
			}
			g.write(";")
		} else if rs, ok := stmt.(*ast.ReturnStmt); ok && rs.Value != nil {
			g.write("return ")
			if err := g.emitExpr(rs.Value); err != nil {
				return err
			}
			g.write(";")
		} else {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
	}
	g.write(" }")
	return nil
}

func (g *csGen) emitArrayLit(al *ast.ArrayLit) error {
	g.needList = true
	g.write("new List<" + g.typeStr(al.ElemType) + "> { ")
	for i, elem := range al.Elements {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(elem); err != nil {
			return err
		}
	}
	g.write(" }")
	return nil
}

func (g *csGen) emitIndexExpr(ie *ast.IndexExpr) error {
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write("[(int)(")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write(")]")
	return nil
}

func (g *csGen) emitStructLit(sl *ast.StructLit) error {
	g.write("new " + sl.TypeName + "(")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *csGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("Console.WriteLine(")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(" + \" \" + ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "printf":
		g.write("Console.Write(")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "sprintf":
		g.write("Convert.ToString(")
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

func (g *csGen) emitLiteral(lit *ast.Literal) error {
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
