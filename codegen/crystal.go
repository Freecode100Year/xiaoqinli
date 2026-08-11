package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateCrystal produces Crystal source code from the given typed AST.
// The "main" function's body is emitted at top level (like Ruby).
func GenerateCrystal(root ast.Node) ([]byte, error) {
	g := &crystalGen{buf: &strings.Builder{}}
	g.types = newTypeKinds(root)

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
		for _, stmt := range fd.Body {
			if err := g.emitNode(stmt); err != nil {
				return nil, err
			}
		}
	}

	return []byte(g.buf.String()), nil
}

type crystalGen struct {
	types  *typeKinds
	buf    *strings.Builder
	indent int
}

func (g *crystalGen) write(s string)   { g.buf.WriteString(s) }
func (g *crystalGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *crystalGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("  ")
	}
}

func typeToCrystal(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "Int64"
	case "Float":
		return "Float64"
	case "String":
		return "String"
	case "Bool":
		return "Bool"
	case "Void":
		return "Nil"
	case "Array":
		if t.Elem != nil {
			return "Array(" + typeToCrystal(*t.Elem) + ")"
		}
		return "Array(String)"
	case "Option":
		if t.Elem != nil {
			return typeToCrystal(*t.Elem) + "?"
		}
		return "String?"
	default:
		return t.KindName
	}
}

func (g *crystalGen) emitNode(n ast.Node) error {
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
		g.writeln("next")
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

func (g *crystalGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("struct " + sd.Name)
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln("property " + f.Name + " : " + typeToCrystal(f.Type))
	}
	g.writeln("")
	g.writeIndent()
	g.write("def initialize(")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write("@" + f.Name + " : " + typeToCrystal(f.Type))
	}
	g.writeln(")")
	g.writeIndent()
	g.writeln("end")
	g.indent--
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *crystalGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.writeln("enum " + ed.Name)
	g.indent++
	for _, v := range ed.Variants {
		g.writeIndent()
		g.writeln(v)
	}
	g.indent--
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *crystalGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("case ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln("")
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.writeln("else")
		} else {
			g.write("when ")
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.writeln("")
		}
		g.indent++
		for _, stmt := range arm.Body {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
		g.indent--
	}
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *crystalGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.types.noteParams(fd)
	g.writeIndent()
	g.write("def " + fd.Name)
	if len(fd.Params) > 0 {
		g.write("(")
		for i, p := range fd.Params {
			if i > 0 {
				g.write(", ")
			}
			g.write(p.Name + " : " + typeToCrystal(p.Type))
		}
		g.write(")")
	}
	rt := typeToCrystal(fd.ReturnType)
	if rt != "" && rt != "Nil" {
		g.write(" : " + rt)
	}
	g.writeln("")
	g.indent++
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *crystalGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *crystalGen) emitVarDecl(vd *ast.VarDecl) error {
	g.types.noteVar(vd)
	g.writeIndent()
	g.write(vd.Name)
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	} else {
		g.write(" = nil")
	}
	g.writeln("")
	return nil
}

func (g *crystalGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *crystalGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if ")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln("")
	g.indent++
	for _, s := range is.Then {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	if len(is.Else) > 0 {
		g.writeIndent()
		g.writeln("else")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
	}
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *crystalGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while ")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln("")
	g.indent++
	for _, s := range ws.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *crystalGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("(")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("...")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(").each do |" + fs.Var + "|")
	case "each":
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(".each do |" + fs.Var + "|")
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
	g.writeln("end")
	return nil
}

func (g *crystalGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *crystalGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		op := node.Op
		// Crystal's Int#/ has returned a Float64 since 0.36 — `7 / 2` is 3.5,
		// and it typechecks, so the compiled tier could never have caught it.
		// `//` is the integer division.
		if g.types.isIntDivision(node) {
			// `//` is the integer division, and it *floors*: -7 // 2 is -4 where
			// C, Go, Java and Rust answer -3. tdiv is the truncating one, and it
			// is what the rest of this matrix means by `/`.
			return g.emitIntMethod("tdiv", node.Left, node.Right)
		}
		// `%` floors to match `//`, so it carried the divisor's sign — -7 % 2
		// was 1 where the majority answer is -1. remainder is `tdiv`'s partner
		// and keeps the dividend's.
		if g.types.isIntRemainder(node) {
			return g.emitIntMethod("remainder", node.Left, node.Right)
		}
		g.write("(")
		if err := g.emitExpr(node.Left); err != nil {
			return err
		}
		g.write(" " + op + " ")
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

func (g *crystalGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *crystalGen) emitLambda(lam *ast.Lambda) error {
	g.write("->(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + " : " + typeToCrystal(p.Type))
	}
	g.write(") { ")
	for i, stmt := range lam.Body {
		if i > 0 {
			g.write("; ")
		}
		if es, ok := stmt.(*ast.ExprStmt); ok {
			if err := g.emitExpr(es.Expr); err != nil {
				return err
			}
		} else if rs, ok := stmt.(*ast.ReturnStmt); ok && rs.Value != nil {
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

func (g *crystalGen) emitArrayLit(al *ast.ArrayLit) error {
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

func (g *crystalGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

// emitIntMethod writes `(left).name(right)`, the call form Crystal's truncating
// integer operators take. The parentheses are not decoration: the left operand
// can be a negative literal, and -7.tdiv(2) parses as -(7.tdiv 2).
func (g *crystalGen) emitIntMethod(name string, left, right ast.Node) error {
	g.write("(")
	if err := g.emitExpr(left); err != nil {
		return err
	}
	g.write(")." + name + "(")
	if err := g.emitExpr(right); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *crystalGen) emitStructLit(sl *ast.StructLit) error {
	g.write(sl.TypeName + ".new(")
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

func (g *crystalGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("puts(")
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
			g.write("printf(")
			for i, arg := range ce.Args {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write(")")
		} else {
			g.write("print(")
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
			g.write("sprintf(")
			for i, arg := range ce.Args {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write(")")
		} else if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(".to_s")
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

func (g *crystalGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write(fmt.Sprintf("%q", s))
	case "Int":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%d_i64", int64(f)))
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
