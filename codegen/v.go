package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateV produces V source code from the given typed AST.
// The "main" function is emitted as `fn main() { ... }`.
func GenerateV(root ast.Node) ([]byte, error) {
	g := &vGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	first := true
	for _, d := range prog.Decls {
		switch node := d.(type) {
		case *ast.StructDecl:
			if !first {
				g.writeln("")
			}
			if err := g.emitStructDecl(node); err != nil {
				return nil, err
			}
			first = false
		case *ast.EnumDecl:
			if !first {
				g.writeln("")
			}
			if err := g.emitEnumDecl(node); err != nil {
				return nil, err
			}
			first = false
		}
	}

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

	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		if !first {
			g.writeln("")
		}
		if err := g.emitFunctionDecl(fd); err != nil {
			return nil, err
		}
	}

	return []byte(g.buf.String()), nil
}

type vGen struct {
	buf    *strings.Builder
	indent int
	muts   map[string]bool
}

func (g *vGen) write(s string)   { g.buf.WriteString(s) }
func (g *vGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *vGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func typeToV(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "int"
	case "Float":
		return "f64"
	case "String":
		return "string"
	case "Bool":
		return "bool"
	case "Void":
		return ""
	case "Array":
		if t.Elem != nil {
			return "[]" + typeToV(*t.Elem)
		}
		return "[]string"
	case "Option":
		if t.Elem != nil {
			return "?" + typeToV(*t.Elem)
		}
		return "?string"
	default:
		return t.KindName
	}
}

func (g *vGen) emitNode(n ast.Node) error {
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

func (g *vGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("struct " + sd.Name + " {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(f.Name + " " + typeToV(f.Type))
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *vGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.writeln("enum " + ed.Name + " {")
	g.indent++
	for _, v := range ed.Variants {
		g.writeIndent()
		g.writeln(v)
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *vGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("match ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(" {")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.write("else")
		} else {
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
		}
		g.writeln(" {")
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

func (g *vGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)

	g.writeIndent()
	g.write("fn " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + " " + typeToV(p.Type))
	}
	g.write(")")
	rt := typeToV(fd.ReturnType)
	if rt != "" {
		g.write(" " + rt)
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

func (g *vGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *vGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if g.muts[vd.Name] {
		g.write("mut " + vd.Name)
	} else {
		g.write(vd.Name)
	}
	if vd.Value != nil {
		g.write(" := ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	} else {
		rt := typeToV(vd.Type)
		if rt != "" {
			g.write(" := " + rt + "{}")
		} else {
			g.write(" := 0")
		}
	}
	g.writeln("")
	return nil
}

func (g *vGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *vGen) emitIf(is *ast.IfStmt) error {
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

func (g *vGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("for ")
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

func (g *vGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for " + fs.Var + " in ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write(" .. ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(" {")
	case "each":
		g.write("for " + fs.Var + " in ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(" {")
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

func (g *vGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *vGen) emitExpr(n ast.Node) error {
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
		if node.Op == "!" {
			g.write("!")
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

func (g *vGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("if ")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" { ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" } else { ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(" }")
	return nil
}

func (g *vGen) emitLambda(lam *ast.Lambda) error {
	g.write("fn (")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + " " + typeToV(p.Type))
	}
	g.write(")")
	rt := typeToV(lam.ReturnType)
	if rt != "" {
		g.write(" " + rt)
	}
	g.write(" { ")
	for i, stmt := range lam.Body {
		if i > 0 {
			g.write("; ")
		}
		if es, ok := stmt.(*ast.ExprStmt); ok {
			if err := g.emitExpr(es.Expr); err != nil {
				return err
			}
		} else if rs, ok := stmt.(*ast.ReturnStmt); ok && rs.Value != nil {
			g.write("return ")
			if err := g.emitExpr(rs.Value); err != nil {
				return err
			}
		} else {
			if err := g.emitExpr(stmt); err != nil {
				return err
			}
		}
	}
	g.write(" }")
	return nil
}

func (g *vGen) emitArrayLit(al *ast.ArrayLit) error {
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

func (g *vGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *vGen) emitStructLit(sl *ast.StructLit) error {
	g.write(sl.TypeName + "{ ")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + ": ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(" }")
	return nil
}

func (g *vGen) emitCall(ce *ast.CallExpr) error {
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
	case "sprintf":
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(".str()")
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

func (g *vGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "'", "\\'")
		g.write("'" + s + "'")
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
