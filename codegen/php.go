package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GeneratePHP produces PHP source code from the given typed AST.
// The "main" function's body is emitted at top level after other functions.
func GeneratePHP(root ast.Node) ([]byte, error) {
	g := &phpGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	g.writeln("<?php")
	g.writeln("")

	for _, d := range prog.Decls {
		if sd, ok := d.(*ast.StructDecl); ok {
			if err := g.emitStructDecl(sd); err != nil {
				return nil, err
			}
			g.writeln("")
		}
	}

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

	return []byte(g.buf.String()), nil
}

type phpGen struct {
	buf    *strings.Builder
	indent int
}

func (g *phpGen) write(s string)   { g.buf.WriteString(s) }
func (g *phpGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *phpGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func typeToPHP(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "int"
	case "Float":
		return "float"
	case "String":
		return "string"
	case "Bool":
		return "bool"
	case "Void":
		return "void"
	case "Array":
		return "array"
	case "Option":
		if t.Elem != nil {
			return "?" + typeToPHP(*t.Elem)
		}
		return "mixed"
	case "Result":
		return "mixed"
	default:
		return t.KindName
	}
}

func (g *phpGen) emitNode(n ast.Node) error {
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
	case *ast.StructDecl:
		return g.emitStructDecl(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *phpGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("class " + sd.Name + " {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln("public " + typeToPHP(f.Type) + " $" + f.Name + ";")
	}
	g.writeIndent()
	g.write("public function __construct(")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(typeToPHP(f.Type) + " $" + f.Name)
	}
	g.writeln(") {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln("$this->" + f.Name + " = $" + f.Name + ";")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *phpGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.writeIndent()
	g.write("function " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(typeToPHP(p.Type) + " $" + p.Name)
	}
	g.write(")")
	rt := typeToPHP(fd.ReturnType)
	g.write(": " + rt)
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

func (g *phpGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *phpGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	g.write("$" + vd.Name)
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	} else {
		g.write(" = null")
	}
	g.writeln(";")
	return nil
}

func (g *phpGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	g.write("$" + as.Target + " = ")
	if err := g.emitExpr(as.Value); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *phpGen) emitIf(is *ast.IfStmt) error {
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

func (g *phpGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *phpGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *phpGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write("$" + node.Name)
		return nil
	case *ast.BinaryExpr:
		g.write("(")
		if err := g.emitExpr(node.Left); err != nil {
			return err
		}
		op := node.Op
		if op == "+" && containsStringExpr(node) {
			op = "."
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
		g.write("->" + node.Field)
		return nil
	case *ast.StructLit:
		return g.emitStructLit(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported expression %s", n.Kind())
	}
}

func (g *phpGen) emitStructLit(sl *ast.StructLit) error {
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

func (g *phpGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write(`echo `)
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(` . `)
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(` . "\n"`)
		return nil
	case "printf":
		g.write("echo ")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		return nil
	case "sprintf":
		g.write("strval(")
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

func (g *phpGen) emitLiteral(lit *ast.Literal) error {
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
