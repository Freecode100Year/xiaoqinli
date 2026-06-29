package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateAwk produces AWK source code from the given typed AST.
// All code is wrapped in a BEGIN { ... } block.
func GenerateAwk(root ast.Node) ([]byte, error) {
	g := &awkGen{buf: &strings.Builder{}, funcBuf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Collect function declarations (non-main) into funcBuf.
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name == "main" {
			continue
		}
		origBuf := g.buf
		g.buf = g.funcBuf
		if g.funcBuf.Len() > 0 {
			g.writeln("")
		}
		if err := g.emitFunctionDecl(fd); err != nil {
			return nil, err
		}
		g.buf = origBuf
	}

	// Emit BEGIN block with enum constants, main body, etc.
	g.writeln("BEGIN {")
	g.indent++

	// Emit enum declarations as integer constants.
	for _, d := range prog.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			if err := g.emitEnumDecl(ed); err != nil {
				return nil, err
			}
		}
	}

	// Emit struct declarations as comments.
	for _, d := range prog.Decls {
		if sd, ok := d.(*ast.StructDecl); ok {
			if err := g.emitStructDecl(sd); err != nil {
				return nil, err
			}
		}
	}

	// Emit main body.
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		for _, stmt := range fd.Body {
			if err := g.emitNode(stmt); err != nil {
				return nil, err
			}
		}
	}

	g.indent--
	g.writeln("}")

	// Append function definitions after BEGIN block.
	var out strings.Builder
	out.WriteString(g.buf.String())
	if g.funcBuf.Len() > 0 {
		out.WriteByte('\n')
		out.WriteString(g.funcBuf.String())
	}

	return []byte(out.String()), nil
}

type awkGen struct {
	buf     *strings.Builder
	funcBuf *strings.Builder
	indent  int
}

func (g *awkGen) write(s string)   { g.buf.WriteString(s) }
func (g *awkGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *awkGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func (g *awkGen) emitNode(n ast.Node) error {
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

func (g *awkGen) emitStructDecl(sd *ast.StructDecl) error {
	// AWK has no struct support; emit a comment.
	g.writeIndent()
	g.write("# struct " + sd.Name + ": ")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name)
	}
	g.writeln("")
	return nil
}

func (g *awkGen) emitEnumDecl(ed *ast.EnumDecl) error {
	for i, v := range ed.Variants {
		g.writeIndent()
		g.writeln(fmt.Sprintf("%s = %d", v, i))
	}
	return nil
}

func (g *awkGen) emitMatchExpr(me *ast.MatchExpr) error {
	// AWK has no switch/case; emit as if/else if chain.
	first := true
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			if !first {
				g.write("} else {")
			} else {
				g.write("if (1) {")
			}
		} else {
			if first {
				g.write("if (")
			} else {
				g.write("} else if (")
			}
			if err := g.emitExpr(me.Value); err != nil {
				return err
			}
			g.write(" == ")
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.write(") {")
		}
		g.writeln("")
		g.indent++
		for _, stmt := range arm.Body {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
		g.indent--
		first = false
	}
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *awkGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.writeIndent()
	g.write("function " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name)
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

func (g *awkGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *awkGen) emitVarDecl(vd *ast.VarDecl) error {
	if al, ok := vd.Value.(*ast.ArrayLit); ok {
		for i, elem := range al.Elements {
			g.writeIndent()
			g.write(fmt.Sprintf("%s[%d] = ", vd.Name, i))
			if err := g.emitExpr(elem); err != nil {
				return err
			}
			g.writeln("")
		}
		return nil
	}
	g.writeIndent()
	g.write(vd.Name + " = ")
	if vd.Value != nil {
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	} else {
		g.write("\"\"")
	}
	g.writeln("")
	return nil
}

func (g *awkGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *awkGen) emitIf(is *ast.IfStmt) error {
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
	if len(is.Else) > 0 {
		g.writeIndent()
		g.writeln("} else {")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
	}
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *awkGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *awkGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for (" + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("; " + fs.Var + " < ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln("; " + fs.Var + "++) {")
	case "each":
		g.write("for (" + fs.Var + " in ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(") {")
	default:
		return fmt.Errorf("XQL_E401: unsupported for-loop form %q", fs.Form)
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

func (g *awkGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *awkGen) emitExpr(n ast.Node) error {
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
		if op == "+" && containsStringExpr(node) {
			// AWK string concatenation: use space
			g.write(" ")
			if err := g.emitExpr(node.Right); err != nil {
				return err
			}
			g.write(")")
			return nil
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
		// Not directly supported in AWK; use simple approach.
		if err := g.emitExpr(node.Object); err != nil {
			return err
		}
		g.write("[\"" + node.Field + "\"]")
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

func (g *awkGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *awkGen) emitLambda(lam *ast.Lambda) error {
	return fmt.Errorf("XQL_E401: AWK does not support Lambda expressions")
}

func (g *awkGen) emitArrayLit(al *ast.ArrayLit) error {
	// AWK doesn't have array literals; emit as a comment-placeholder.
	// In practice, arrays are built by assignment.
	g.write("0")
	return nil
}

func (g *awkGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *awkGen) emitStructLit(sl *ast.StructLit) error {
	// AWK has no struct literals; emit as 0 placeholder.
	g.write("0")
	return nil
}

func (g *awkGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("print ")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(", ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		return nil
	case "printf":
		g.write("printf ")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(", ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		return nil
	case "sprintf":
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

func (g *awkGen) emitLiteral(lit *ast.Literal) error {
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
			g.write("1")
		} else {
			g.write("0")
		}
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}
