package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateD produces D source code from the given typed AST.
// The "main" function is emitted as void main() { ... }.
func GenerateD(root ast.Node) ([]byte, error) {
	g := &dGen{
		buf:      &strings.Builder{},
		funcRets: make(map[string]string),
		varTypes: make(map[string]string),
	}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	for _, d := range prog.Decls {
		if fd, ok := d.(*ast.FunctionDecl); ok {
			g.funcRets[fd.Name] = fd.ReturnType.KindName
		}
	}

	// Emit struct and enum declarations first.
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

	// Emit non-main functions.
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

	// Emit main function.
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

	// Build output with imports.
	var out strings.Builder
	out.WriteString("import std.stdio;\n")
	if g.needConv {
		out.WriteString("import std.conv;\n")
	}
	if g.needFmt {
		out.WriteString("import std.format;\n")
	}
	out.WriteString("\n")
	out.WriteString(g.buf.String())
	return []byte(out.String()), nil
}

type dGen struct {
	buf      *strings.Builder
	indent   int
	needConv bool
	needFmt  bool
	funcRets map[string]string
	varTypes map[string]string
}

func (g *dGen) write(s string)   { g.buf.WriteString(s) }
func (g *dGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *dGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func typeToD(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "long"
	case "Float":
		return "double"
	case "String":
		return "string"
	case "Bool":
		return "bool"
	case "Void":
		return "void"
	case "Array":
		if t.Elem != nil {
			return typeToD(*t.Elem) + "[]"
		}
		return "long[]"
	default:
		return t.KindName
	}
}

func (g *dGen) inferTypeKind(n ast.Node) string {
	switch node := n.(type) {
	case *ast.Literal:
		return node.ValueType
	case *ast.Ident:
		if t, ok := g.varTypes[node.Name]; ok {
			return t
		}
		return "Int"
	case *ast.CallExpr:
		if node.Callee == "sprintf" {
			return "String"
		}
		if rt, ok := g.funcRets[node.Callee]; ok {
			return rt
		}
		return "Int"
	case *ast.BinaryExpr:
		if node.Op == "+" && (g.inferTypeKind(node.Left) == "String" || g.inferTypeKind(node.Right) == "String") {
			return "String"
		}
		switch node.Op {
		case "==", "!=", "<", ">", "<=", ">=", "&&", "||":
			return "Bool"
		}
		return g.inferTypeKind(node.Left)
	case *ast.MemberExpr:
		return ""
	case *ast.IndexExpr:
		return ""
	case *ast.UnaryExpr:
		if node.Op == "!" {
			return "Bool"
		}
		return g.inferTypeKind(node.Operand)
	default:
		return "Int"
	}
}

func (g *dGen) emitNode(n ast.Node) error {
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

func (g *dGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("struct " + sd.Name + " {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(typeToD(f.Type) + " " + f.Name + ";")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *dGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.writeln("enum " + ed.Name + " {")
	g.indent++
	for i, v := range ed.Variants {
		g.writeIndent()
		if i < len(ed.Variants)-1 {
			g.writeln(v + ",")
		} else {
			g.writeln(v)
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *dGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	for _, p := range fd.Params {
		g.varTypes[p.Name] = p.Type.KindName
	}

	g.writeIndent()
	if fd.Name == "main" {
		g.writeln("void main() {")
	} else {
		rt := typeToD(fd.ReturnType)
		g.write(rt + " " + fd.Name + "(")
		for i, p := range fd.Params {
			if i > 0 {
				g.write(", ")
			}
			g.write(typeToD(p.Type) + " " + p.Name)
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

func (g *dGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *dGen) emitVarDecl(vd *ast.VarDecl) error {
	g.varTypes[vd.Name] = vd.Type.KindName
	g.writeIndent()
	g.write(typeToD(vd.Type) + " " + vd.Name)
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln(";")
	return nil
}

func (g *dGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *dGen) emitIf(is *ast.IfStmt) error {
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

func (g *dGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *dGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("foreach (" + fs.Var + "; ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("..")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.write(")")
	case "each":
		g.write("foreach (" + fs.Var + "; ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.write(")")
	default:
		return fmt.Errorf("XQL_E401: unknown ForStmt form %q", fs.Form)
	}
	g.writeln(" {")
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

func (g *dGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("switch (")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(") {")
	g.indent++
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
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *dGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *dGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		return g.emitBinary(node)
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

func (g *dGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *dGen) emitLambda(lam *ast.Lambda) error {
	g.write("(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(typeToD(p.Type) + " " + p.Name)
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
			g.write("return ")
			if err := g.emitExpr(rs.Value); err != nil {
				return err
			}
			g.write(";")
		} else {
			if err := g.emitExpr(stmt); err != nil {
				return err
			}
		}
	}
	g.write(" }")
	return nil
}

func (g *dGen) emitBinary(be *ast.BinaryExpr) error {
	if be.Op == "+" && (g.inferTypeKind(be.Left) == "String" || g.inferTypeKind(be.Right) == "String") {
		g.write("(")
		if err := g.emitExpr(be.Left); err != nil {
			return err
		}
		g.write(" ~ ")
		if err := g.emitExpr(be.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	}
	g.write("(")
	if err := g.emitExpr(be.Left); err != nil {
		return err
	}
	g.write(" " + be.Op + " ")
	if err := g.emitExpr(be.Right); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *dGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("writeln(")
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
			g.write("writef(")
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
			g.write("write(")
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
			g.needFmt = true
			g.write("format(")
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
			g.needConv = true
			g.write("to!string(")
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(")")
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

func (g *dGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write(fmt.Sprintf("%q", s))
	case "Int":
		// D's long is 64-bit everywhere, so the variables were right and the
		// arithmetic was not: `100000 * 100000` is int times int, and it
		// overflowed to 1410065408 before the long ever saw it.
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

func (g *dGen) emitStructLit(sl *ast.StructLit) error {
	g.write(sl.TypeName + "(")
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

func (g *dGen) emitArrayLit(al *ast.ArrayLit) error {
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

func (g *dGen) emitIndexExpr(ie *ast.IndexExpr) error {
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
