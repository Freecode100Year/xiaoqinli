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

	// Emit enum declarations first.
	for _, d := range prog.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			if err := g.emitEnumDecl(ed); err != nil {
				return nil, err
			}
			g.writeln("")
		}
	}

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
func (g *phpGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

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
	case *ast.ImportDecl:
		return g.emitImportDecl(node)
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

func (g *phpGen) emitImportDecl(id *ast.ImportDecl) error {
	g.writeIndent()
	path := id.Path
	if strings.HasSuffix(path, ".xql") {
		path = path[:len(path)-4] + ".php"
	}
	g.writeln(fmt.Sprintf("require_once __DIR__ . '/%s';", path))
	return nil
}

func (g *phpGen) emitEnumDecl(ed *ast.EnumDecl) error {
	for i, v := range ed.Variants {
		g.writeIndent()
		g.writeln(fmt.Sprintf("const %s = %d;", v, i))
	}
	return nil
}

func (g *phpGen) emitMatchExpr(me *ast.MatchExpr) error {
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

func (g *phpGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for ($" + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("; $" + fs.Var + " < ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln("; $" + fs.Var + "++) {")
	case "each":
		g.write("foreach (")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(" as $" + fs.Var + ") {")
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
	case *ast.ArrayLit:
		return g.emitArrayLit(node.Elements)
	case *ast.ArrayLiteral:
		return g.emitArrayLit(node.Elements)
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

func (g *phpGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *phpGen) emitLambda(lam *ast.Lambda) error {
	g.write("function(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write("$" + p.Name)
	}
	g.writeln(") {")
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

func (g *phpGen) emitArrayLit(elements []ast.Node) error {
	g.write("[")
	for i, elem := range elements {
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

func (g *phpGen) emitIndexExpr(ie *ast.IndexExpr) error {
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
			g.write("echo ")
			if len(ce.Args) > 0 {
				if err := g.emitExpr(ce.Args[0]); err != nil {
					return err
				}
			}
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
		} else {
			g.write("strval(")
			if len(ce.Args) > 0 {
				if err := g.emitExpr(ce.Args[0]); err != nil {
					return err
				}
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
