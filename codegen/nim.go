package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateNim produces Nim source code from the given typed AST.
// The "main" function's body is emitted at top level.
func GenerateNim(root ast.Node) ([]byte, error) {
	g := &nimGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	first := true
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
		g.muts = collectMutables(fd.Body)
		for _, stmt := range fd.Body {
			if err := g.emitNode(stmt); err != nil {
				return nil, err
			}
		}
	}

	return []byte(g.buf.String()), nil
}

type nimGen struct {
	buf    *strings.Builder
	indent int
	muts   map[string]bool
}

func (g *nimGen) write(s string)   { g.buf.WriteString(s) }
func (g *nimGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *nimGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("  ") } }

func typeToNim(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "int64"
	case "Float":
		return "float64"
	case "String":
		return "string"
	case "Bool":
		return "bool"
	case "Void":
		return ""
	case "Array":
		if t.Elem != nil {
			return "seq[" + typeToNim(*t.Elem) + "]"
		}
		return "seq[any]"
	case "Option":
		if t.Elem != nil {
			return "Option[" + typeToNim(*t.Elem) + "]"
		}
		return "Option[any]"
	case "Result":
		if t.OkType != nil {
			return typeToNim(*t.OkType)
		}
		return "any"
	default:
		return t.KindName
	}
}

func (g *nimGen) emitNode(n ast.Node) error {
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
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *nimGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("type " + sd.Name + " = object")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(f.Name + ": " + typeToNim(f.Type))
	}
	g.indent--
	return nil
}

func (g *nimGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)

	g.writeIndent()
	g.write("proc " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + ": " + typeToNim(p.Type))
	}
	g.write(")")
	rt := typeToNim(fd.ReturnType)
	if rt != "" {
		g.write(": " + rt)
	}
	g.writeln(" =")
	g.indent++
	if len(fd.Body) == 0 {
		g.writeIndent()
		g.writeln("discard")
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

func (g *nimGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *nimGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if g.muts[vd.Name] {
		g.write("var " + vd.Name + ": " + typeToNim(vd.Type))
	} else {
		g.write("let " + vd.Name + ": " + typeToNim(vd.Type))
	}
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln("")
	return nil
}

func (g *nimGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *nimGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if ")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(":")
	g.indent++
	if len(is.Then) == 0 {
		g.writeIndent()
		g.writeln("discard")
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

func (g *nimGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while ")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(":")
	g.indent++
	if len(ws.Body) == 0 {
		g.writeIndent()
		g.writeln("discard")
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

func (g *nimGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for " + fs.Var + " in ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write(" ..< ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(":")
	case "each":
		g.write("for " + fs.Var + " in ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(":")
	default:
		return fmt.Errorf("XQL_E401: unknown ForStmt form %q", fs.Form)
	}
	g.indent++
	if len(fs.Body) == 0 {
		g.writeIndent()
		g.writeln("discard")
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

func (g *nimGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *nimGen) emitExpr(n ast.Node) error {
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
		switch op {
		case "&&":
			op = "and"
		case "||":
			op = "or"
		}
		if op == "+" && containsStringExpr(node) {
			op = "&"
		}
		g.write(" " + op + " ")
		if err := g.emitExpr(node.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	case *ast.UnaryExpr:
		if node.Op == "!" {
			g.write("not ")
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

func (g *nimGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("(if ")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(": ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" else: ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *nimGen) emitLambda(lam *ast.Lambda) error {
	g.write("proc(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + ": " + typeToNim(p.Type))
	}
	g.write(")")
	rt := typeToNim(lam.ReturnType)
	if rt != "" {
		g.write(": " + rt)
	}
	g.writeln(" =")
	g.indent++
	if len(lam.Body) == 0 {
		g.writeIndent()
		g.write("discard")
	} else {
		for i, stmt := range lam.Body {
			if i > 0 || len(lam.Body) > 1 {
				g.writeIndent()
			}
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
	}
	g.indent--
	return nil
}

func (g *nimGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("@[")
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

func (g *nimGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *nimGen) emitStructLit(sl *ast.StructLit) error {
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

func (g *nimGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("echo ")
		if len(ce.Args) == 1 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
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
		g.write("stdout.write(")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "sprintf":
		g.write("$")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
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

func (g *nimGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write(fmt.Sprintf("%q", s))
	case "Int":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%d'i64", int64(f)))
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
