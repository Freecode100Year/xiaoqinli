package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateJava produces Java source code from the given typed AST.
// All functions are emitted as static methods inside a public class Main.
func GenerateJava(root ast.Node) ([]byte, error) {
	g := &javaGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	g.indent = 1
	for i, d := range prog.Decls {
		if i > 0 {
			g.writeln("")
		}
		if err := g.emitNode(d); err != nil {
			return nil, err
		}
	}

	var out strings.Builder
	if g.needList {
		out.WriteString("import java.util.ArrayList;\nimport java.util.List;\n\n")
	}
	out.WriteString("public class Main {\n")
	out.WriteString(g.buf.String())
	out.WriteString("}\n")

	return []byte(out.String()), nil
}

type javaGen struct {
	buf      *strings.Builder
	indent   int
	muts     map[string]bool
	needList bool
}

func (g *javaGen) write(s string)   { g.buf.WriteString(s) }
func (g *javaGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *javaGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func (g *javaGen) typeStr(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "long"
	case "Float":
		return "double"
	case "String":
		return "String"
	case "Bool":
		return "boolean"
	case "Void":
		return "void"
	case "Array":
		g.needList = true
		if t.Elem != nil {
			return "List<" + g.boxedTypeStr(*t.Elem) + ">"
		}
		return "List<Object>"
	case "Option":
		if t.Elem != nil {
			return g.boxedTypeStr(*t.Elem)
		}
		return "Object"
	case "Result":
		if t.OkType != nil {
			return g.boxedTypeStr(*t.OkType)
		}
		return "Object"
	default:
		return t.KindName
	}
}

func (g *javaGen) boxedTypeStr(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "Long"
	case "Float":
		return "Double"
	case "Bool":
		return "Boolean"
	default:
		return g.typeStr(t)
	}
}

// --- Node emitters ---

func (g *javaGen) emitNode(n ast.Node) error {
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

func (g *javaGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.write("static record " + sd.Name + "(")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(g.typeStr(f.Type) + " " + f.Name)
	}
	g.writeln(") {}")
	return nil
}

func (g *javaGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)

	g.writeIndent()
	if fd.Name == "main" {
		g.writeln("public static void main(String[] args) {")
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

func (g *javaGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *javaGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if !g.muts[vd.Name] {
		g.write("final ")
	}
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

func (g *javaGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	g.write(as.Target + " = ")
	if err := g.emitExpr(as.Value); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *javaGen) emitIf(is *ast.IfStmt) error {
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

func (g *javaGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *javaGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

// --- Expression emitters ---

func (g *javaGen) emitExpr(n ast.Node) error {
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
	default:
		return fmt.Errorf("XQL_E401: unsupported expression %s", n.Kind())
	}
}

func (g *javaGen) emitStructLit(sl *ast.StructLit) error {
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

func (g *javaGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("System.out.println(")
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
		g.write("System.out.print(")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "sprintf":
		g.write("String.valueOf(")
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

func (g *javaGen) emitLiteral(lit *ast.Literal) error {
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
