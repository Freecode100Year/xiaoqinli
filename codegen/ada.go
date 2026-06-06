package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateAda produces Ada source code from the given typed AST.
// The "main" function's body is emitted inside a `procedure Main is begin ... end Main;` block.
func GenerateAda(root ast.Node) ([]byte, error) {
	g := &adaGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Header with imports.
	var header strings.Builder
	header.WriteString("with Ada.Text_IO; use Ada.Text_IO;\n")
	header.WriteString("with Ada.Integer_Text_IO; use Ada.Integer_Text_IO;\n")
	header.WriteString("\n")

	// Emit type declarations (structs and enums).
	for _, d := range prog.Decls {
		switch node := d.(type) {
		case *ast.StructDecl:
			if err := g.emitStructDecl(node); err != nil {
				return nil, err
			}
			g.writeln("")
		case *ast.EnumDecl:
			if err := g.emitEnumDecl(node); err != nil {
				return nil, err
			}
			g.writeln("")
		}
	}

	// Emit non-main functions/procedures.
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name == "main" {
			continue
		}
		if err := g.emitFunctionDecl(fd); err != nil {
			return nil, err
		}
		g.writeln("")
	}

	// Emit main procedure.
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		if err := g.emitMainBlock(fd); err != nil {
			return nil, err
		}
	}

	return []byte(header.String() + g.buf.String()), nil
}

type adaGen struct {
	buf    *strings.Builder
	indent int
}

func (g *adaGen) write(s string)   { g.buf.WriteString(s) }
func (g *adaGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *adaGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func typeToAda(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "Long_Integer"
	case "Float":
		return "Long_Float"
	case "String":
		return "String"
	case "Bool":
		return "Boolean"
	case "Void":
		return ""
	case "Array":
		if t.Elem != nil {
			return "array of " + typeToAda(*t.Elem)
		}
		return "array of Long_Integer"
	default:
		return t.KindName
	}
}

func (g *adaGen) emitMainBlock(fd *ast.FunctionDecl) error {
	g.writeln("procedure Main is")

	// Collect and emit variable declarations.
	vars := collectVarDecls(fd.Body)
	if len(vars) > 0 {
		g.indent++
		for _, vd := range vars {
			g.writeIndent()
			g.writeln(vd.Name + " : " + typeToAda(vd.Type) + ";")
		}
		g.indent--
	}

	g.writeln("begin")
	g.indent++
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.indent--
	g.writeln("end Main;")
	return nil
}

func (g *adaGen) emitNode(n ast.Node) error {
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
		g.writeln("exit;")
		return nil
	case *ast.ContinueStmt:
		return fmt.Errorf("XQL_E401: Ada does not natively support continue")
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

func (g *adaGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("type " + sd.Name + " is record")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(f.Name + " : " + typeToAda(f.Type) + ";")
	}
	g.indent--
	g.writeIndent()
	g.writeln("end record;")
	return nil
}

func (g *adaGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.writeln("type " + ed.Name + " is (" + strings.Join(ed.Variants, ", ") + ");")
	return nil
}

func (g *adaGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	rt := typeToAda(fd.ReturnType)
	isProcedure := rt == ""

	g.writeIndent()
	if isProcedure {
		g.write("procedure " + fd.Name + "(")
	} else {
		g.write("function " + fd.Name + "(")
	}
	for i, p := range fd.Params {
		if i > 0 {
			g.write("; ")
		}
		g.write(p.Name + " : " + typeToAda(p.Type))
	}
	if isProcedure {
		g.writeln(") is")
	} else {
		g.writeln(") return " + rt + " is")
	}

	// Collect and emit variable declarations.
	vars := collectVarDecls(fd.Body)
	if len(vars) > 0 {
		g.indent++
		for _, vd := range vars {
			g.writeIndent()
			g.writeln(vd.Name + " : " + typeToAda(vd.Type) + ";")
		}
		g.indent--
	}

	g.writeIndent()
	g.writeln("begin")
	g.indent++
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("end " + fd.Name + ";")
	return nil
}

func (g *adaGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *adaGen) emitVarDecl(vd *ast.VarDecl) error {
	// Variable declaration is in the declaration section; here we only emit the assignment if there is a value.
	if vd.Value != nil {
		g.writeIndent()
		g.write(vd.Name + " := ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
		g.writeln(";")
	}
	return nil
}

func (g *adaGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	if err := g.emitExpr(as.Target); err != nil {
		return err
	}
	g.write(" := ")
	if err := g.emitExpr(as.Value); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *adaGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if ")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(" then")
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
	g.writeln("end if;")
	return nil
}

func (g *adaGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while ")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(" loop")
	g.indent++
	for _, s := range ws.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("end loop;")
	return nil
}

func (g *adaGen) emitForStmt(fs *ast.ForStmt) error {
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
		g.writeln(" - 1 loop")
	case "each":
		g.write("for " + fs.Var + " of ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(" loop")
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
	g.writeln("end loop;")
	return nil
}

func (g *adaGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *adaGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("case ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(" is")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.writeln("when others =>")
		} else {
			g.write("when ")
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.writeln(" =>")
		}
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
	g.writeln("end case;")
	return nil
}

func (g *adaGen) emitExpr(n ast.Node) error {
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
		case "!=":
			op = "/="
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

func (g *adaGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("(if ")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" then ")
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

func (g *adaGen) emitLambda(lam *ast.Lambda) error {
	return fmt.Errorf("XQL_E401: Ada does not support Lambda expressions")
}

func (g *adaGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("(")
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

func (g *adaGen) emitIndexExpr(ie *ast.IndexExpr) error {
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write("(")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *adaGen) emitStructLit(sl *ast.StructLit) error {
	g.write("(" + sl.TypeName + "'(")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + " => ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write("))")
	return nil
}

func (g *adaGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		if len(ce.Args) == 0 {
			g.write("New_Line")
			return nil
		}
		g.write("Put_Line(")
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
		g.write("Put(")
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
			g.write("Long_Integer'Image(")
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(")")
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

func (g *adaGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write("\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\"")
	case "Int":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%d", int64(f)))
	case "Float":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%g", f))
	case "Bool":
		b, _ := lit.Value.(bool)
		if b {
			g.write("True")
		} else {
			g.write("False")
		}
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}
