package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

func GenerateScala(root ast.Node) ([]byte, error) {
	g := &scalaGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	var structs []*ast.StructDecl
	var enums []*ast.EnumDecl
	var funcs []*ast.FunctionDecl
	for _, d := range prog.Decls {
		switch n := d.(type) {
		case *ast.StructDecl:
			structs = append(structs, n)
		case *ast.EnumDecl:
			enums = append(enums, n)
		case *ast.FunctionDecl:
			funcs = append(funcs, n)
		}
	}

	for _, sd := range structs {
		if err := g.emitStructDecl(sd); err != nil {
			return nil, err
		}
		g.writeln("")
	}

	g.writeln("object Main {")
	g.indent++

	for _, ed := range enums {
		if err := g.emitEnumDecl(ed); err != nil {
			return nil, err
		}
		g.writeln("")
	}

	for i, fd := range funcs {
		if i > 0 {
			g.writeln("")
		}
		if err := g.emitFunctionDecl(fd); err != nil {
			return nil, err
		}
	}

	g.indent--
	g.writeln("}")

	return []byte(g.buf.String()), nil
}

type scalaGen struct {
	buf    *strings.Builder
	indent int
	muts   map[string]bool
}

func (g *scalaGen) write(s string)   { g.buf.WriteString(s) }
func (g *scalaGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *scalaGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func typeToScala(t ast.TypeExpr) string {
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
			return "Array[" + typeToScala(*t.Elem) + "]"
		}
		return "Array[Any]"
	case "Option":
		if t.Elem != nil {
			return "Option[" + typeToScala(*t.Elem) + "]"
		}
		return "Option[Any]"
	case "Result":
		if t.OkType != nil {
			return "Either[Throwable, " + typeToScala(*t.OkType) + "]"
		}
		return "Either[Throwable, Any]"
	case "Map":
		k := "Any"
		v := "Any"
		if t.KeyType != nil {
			k = typeToScala(*t.KeyType)
		}
		if t.Elem != nil {
			v = typeToScala(*t.Elem)
		}
		return "Map[" + k + ", " + v + "]"
	default:
		return t.KindName
	}
}

func (g *scalaGen) emitNode(n ast.Node) error {
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
		g.writeln("// break (scala.util.control.Breaks)")
		return nil
	case *ast.ContinueStmt:
		g.writeIndent()
		g.writeln("// continue (scala.util.control.Breaks)")
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

func (g *scalaGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.write("case class " + sd.Name + "(")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + ": " + typeToScala(f.Type))
	}
	g.writeln(")")
	return nil
}

func (g *scalaGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)

	g.writeIndent()
	if fd.Name == "main" {
		g.writeln("def main(args: Array[String]): Unit = {")
	} else {
		rt := typeToScala(fd.ReturnType)
		g.write("def " + fd.Name + "(")
		for i, p := range fd.Params {
			if i > 0 {
				g.write(", ")
			}
			g.write(p.Name + ": " + typeToScala(p.Type))
		}
		g.write(")")
		if rt != "" && rt != "Unit" {
			g.write(": " + rt)
		}
		g.writeln(" = {")
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

func (g *scalaGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *scalaGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if g.muts[vd.Name] {
		g.write("var ")
	} else {
		g.write("val ")
	}
	g.write(vd.Name + ": " + typeToScala(vd.Type))
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln("")
	return nil
}

func (g *scalaGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *scalaGen) emitIf(is *ast.IfStmt) error {
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

func (g *scalaGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *scalaGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	if fs.Form == "range" {
		g.write("for (" + fs.Var + " <- ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write(" until ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(") {")
	} else {
		g.write("for (" + fs.Var + " <- ")
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

func (g *scalaGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *scalaGen) emitExpr(n ast.Node) error {
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

func (g *scalaGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("(if (")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(") ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" else ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *scalaGen) emitLambda(lam *ast.Lambda) error {
	g.write("(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + ": " + typeToScala(p.Type))
	}
	g.write(") => {")
	for _, stmt := range lam.Body {
		g.write(" ")
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.write(" }")
	return nil
}

func (g *scalaGen) emitCall(ce *ast.CallExpr) error {
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
		if len(ce.Args) >= 2 {
			g.write("print(")
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(".format(")
			for i, arg := range ce.Args[1:] {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write("))")
			return nil
		}
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
			g.write(".toString")
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

func (g *scalaGen) emitLiteral(lit *ast.Literal) error {
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

func (g *scalaGen) emitStructLit(sl *ast.StructLit) error {
	g.write(sl.TypeName + "(")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + " = ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *scalaGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("Array(")
	for i, elem := range al.Elements {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(elem); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *scalaGen) emitIndexExpr(ie *ast.IndexExpr) error {
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write("(")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write(".toInt)")
	return nil
}

func (g *scalaGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.writeln("object " + ed.Name + " extends Enumeration {")
	g.indent++
	for _, v := range ed.Variants {
		g.writeIndent()
		g.writeln("val " + v + " = Value")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *scalaGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(" match {")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		g.write("case ")
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.write("_")
		} else {
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
		}
		g.writeln(" =>")
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
	g.writeln("}")
	return nil
}
