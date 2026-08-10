package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateElixir produces Elixir source code from the given typed AST.
// The "main" function's body is wrapped in a Main module.
func GenerateElixir(root ast.Node) ([]byte, error) {
	g := &exGen{buf: &strings.Builder{}}
	g.types = newTypeKinds(root)

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Emit enum declarations as module-level comments / atom definitions
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

	// Emit struct declarations (as modules with defstruct or map constructors)
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

	// Collect all function declarations
	var funcs []*ast.FunctionDecl
	for _, d := range prog.Decls {
		if fd, ok := d.(*ast.FunctionDecl); ok {
			funcs = append(funcs, fd)
		}
	}

	// Wrap all functions in defmodule Main
	if len(funcs) > 0 {
		if !first {
			g.writeln("")
		}
		g.writeln("defmodule Main do")
		g.indent++
		for i, fd := range funcs {
			if i > 0 {
				g.writeln("")
			}
			if err := g.emitFunctionDecl(fd); err != nil {
				return nil, err
			}
		}
		g.indent--
		g.writeln("end")
		g.writeln("Main.main()")
	}

	var out strings.Builder
	if g.needSprintf {
		out.WriteString("defmodule XqlHelper do\n")
		out.WriteString("  def sprintf(fmt, args) do\n")
		out.WriteString("    {result, _} = Enum.reduce(args, {fmt, 0}, fn arg, {s, _} ->\n")
		out.WriteString("      {String.replace(s, ~r/%[sdfo]/, to_string(arg), global: false), 0}\n")
		out.WriteString("    end)\n")
		out.WriteString("    result\n")
		out.WriteString("  end\n")
		out.WriteString("end\n\n")
	}
	out.WriteString(g.buf.String())
	return []byte(out.String()), nil
}

type exGen struct {
	types       *typeKinds
	buf         *strings.Builder
	indent      int
	needSprintf bool
	loopCount   int
}

func (g *exGen) write(s string)   { g.buf.WriteString(s) }
func (g *exGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *exGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("  ")
	}
}

func (g *exGen) emitNode(n ast.Node) error {
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
		return g.emitWhileStmt(node)
	case *ast.ForStmt:
		return g.emitForStmt(node)
	case *ast.BreakStmt:
		return fmt.Errorf("XQL_E402: Elixir does not support BreakStmt")
	case *ast.ContinueStmt:
		return fmt.Errorf("XQL_E402: Elixir does not support ContinueStmt")
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

func (g *exGen) emitEnumDecl(ed *ast.EnumDecl) error {
	// Elixir uses atoms for enums; emit a comment documenting the variants.
	g.writeIndent()
	g.write("# Enum " + ed.Name + ": ")
	for i, v := range ed.Variants {
		if i > 0 {
			g.write(", ")
		}
		g.write(":" + strings.ToLower(v))
	}
	g.writeln("")
	return nil
}

func (g *exGen) emitStructDecl(sd *ast.StructDecl) error {
	// Emit as a module with defstruct
	g.writeIndent()
	g.writeln("defmodule " + sd.Name + " do")
	g.indent++
	g.writeIndent()
	g.write("defstruct [")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(":" + f.Name)
	}
	g.writeln("]")
	g.indent--
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *exGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.types.noteParams(fd)
	g.writeIndent()
	g.write("def " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name)
	}
	g.writeln(") do")
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

func (g *exGen) emitReturn(rs *ast.ReturnStmt) error {
	g.writeIndent()
	if rs.Value == nil {
		g.writeln("nil")
		return nil
	}
	if err := g.emitExpr(rs.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *exGen) emitVarDecl(vd *ast.VarDecl) error {
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

func (g *exGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *exGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if ")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(" do")
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

func (g *exGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for " + fs.Var + " <- ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("..(")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(" - 1) do")
	case "each":
		g.write("for " + fs.Var + " <- ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(" do")
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

func (g *exGen) emitWhileStmt(ws *ast.WhileStmt) error {
	loopName := fmt.Sprintf("xql_loop%d", g.loopCount)
	g.loopCount++
	loopFn := loopName + "_fn"

	g.writeIndent()
	g.writeln(loopName + " = fn " + loopFn + " ->")
	g.indent++

	if lit, ok := ws.Cond.(*ast.Literal); ok && lit.ValueType == "Bool" && lit.Value == true {
		for _, s := range ws.Body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.writeIndent()
		g.writeln(loopFn + ".(" + loopFn + ")")
	} else {
		g.writeIndent()
		g.write("if ")
		if err := g.emitExpr(ws.Cond); err != nil {
			return err
		}
		g.writeln(" do")
		g.indent++
		for _, s := range ws.Body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.writeIndent()
		g.writeln(loopFn + ".(" + loopFn + ")")
		g.indent--
		g.writeIndent()
		g.writeln("end")
	}

	g.indent--
	g.writeIndent()
	g.writeln("end")
	g.writeIndent()
	g.writeln(loopName + ".(" + loopName + ")")
	return nil
}

func (g *exGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *exGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("case ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(" do")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		// Wildcard pattern
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.write("_ ->")
		} else {
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.write(" ->")
		}
		g.writeln("")
		g.indent++
		if len(arm.Body) == 0 {
			g.writeIndent()
			g.writeln("nil")
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
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *exGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		return g.emitBinaryExpr(node)
	case *ast.UnaryExpr:
		return g.emitUnaryExpr(node)
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
	case *ast.MatchExpr:
		return g.emitMatchExprInline(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported expression %s", n.Kind())
	}
}

func (g *exGen) emitBinaryExpr(be *ast.BinaryExpr) error {
	op := be.Op
	// Map operators to Elixir equivalents
	switch op {
	case "/":
		// Elixir's `/` always returns a float; div/2 is integer division.
		if !g.types.isIntDivision(be) {
			break
		}
		g.write("div(")
		if err := g.emitExpr(be.Left); err != nil {
			return err
		}
		g.write(", ")
		if err := g.emitExpr(be.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	case "%":
		op = "rem"
		// rem(a, b) form
		g.write("rem(")
		if err := g.emitExpr(be.Left); err != nil {
			return err
		}
		g.write(", ")
		if err := g.emitExpr(be.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	case "&&":
		op = "and"
	case "||":
		op = "or"
	case "!=":
		op = "!="
	case "+":
		// Check for string concatenation
		if containsStringExpr(be.Left) || containsStringExpr(be.Right) {
			g.write("(")
			if err := g.emitExpr(be.Left); err != nil {
				return err
			}
			g.write(" <> ")
			if err := g.emitExpr(be.Right); err != nil {
				return err
			}
			g.write(")")
			return nil
		}
	}
	g.write("(")
	if err := g.emitExpr(be.Left); err != nil {
		return err
	}
	g.write(" " + op + " ")
	if err := g.emitExpr(be.Right); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *exGen) emitUnaryExpr(ue *ast.UnaryExpr) error {
	op := ue.Op
	switch op {
	case "!":
		op = "not "
	}
	g.write(op)
	return g.emitExpr(ue.Operand)
}

func (g *exGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("if(")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(", do: ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(", else: ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *exGen) emitLambda(lam *ast.Lambda) error {
	g.write("fn ")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name)
	}
	g.write(" -> ")
	if len(lam.Body) == 1 {
		if rs, ok := lam.Body[0].(*ast.ReturnStmt); ok && rs.Value != nil {
			if err := g.emitExpr(rs.Value); err != nil {
				return err
			}
			g.write(" end")
			return nil
		}
		if es, ok := lam.Body[0].(*ast.ExprStmt); ok {
			if err := g.emitExpr(es.Expr); err != nil {
				return err
			}
			g.write(" end")
			return nil
		}
	}
	g.writeln("")
	g.indent++
	for _, stmt := range lam.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.write("end")
	return nil
}

func (g *exGen) emitArrayLit(al *ast.ArrayLit) error {
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

func (g *exGen) emitIndexExpr(ie *ast.IndexExpr) error {
	g.write("Enum.at(")
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write(", ")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *exGen) emitStructLit(sl *ast.StructLit) error {
	g.write("%{")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + ": ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write("}")
	return nil
}

func (g *exGen) emitMatchExprInline(me *ast.MatchExpr) error {
	g.write("case ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(" do")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.write("_ ->")
		} else {
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.write(" ->")
		}
		g.writeln("")
		g.indent++
		if len(arm.Body) == 0 {
			g.writeIndent()
			g.writeln("nil")
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
	g.writeIndent()
	g.write("end")
	return nil
}

func (g *exGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		if len(ce.Args) == 0 {
			g.write(`IO.puts("")`)
			return nil
		}
		g.write("IO.puts(")
		if err := g.emitExpr(ce.Args[0]); err != nil {
			return err
		}
		g.write(")")
		return nil
	case "printf":
		if len(ce.Args) >= 2 {
			g.needSprintf = true
			g.write("IO.write(XqlHelper.sprintf(")
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
			g.write("IO.write(")
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
			g.write("XqlHelper.sprintf(")
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

func (g *exGen) emitLiteral(lit *ast.Literal) error {
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
