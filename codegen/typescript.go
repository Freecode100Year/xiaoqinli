package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateTypeScript produces TypeScript source code from the given typed AST.
func GenerateTypeScript(root ast.Node) ([]byte, error) {
	return generateJSTarget(root, false)
}

// GenerateJavaScript produces pure JavaScript source code from the given typed AST.
func GenerateJavaScript(root ast.Node) ([]byte, error) {
	return generateJSTarget(root, true)
}

func generateJSTarget(root ast.Node, isJS bool) ([]byte, error) {
	g := &tsGen{buf: &strings.Builder{}, isJS: isJS}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Emit enum declarations first.
	for _, d := range prog.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			if err := g.emitEnumDecl(ed); err != nil {
				return nil, err
			}
			g.writeln("")
		}
	}

	hasMain := false
	for i, d := range prog.Decls {
		if _, ok := d.(*ast.EnumDecl); ok {
			continue
		}
		if i > 0 {
			g.writeln("")
		}
		if err := g.emitNode(d); err != nil {
			return nil, err
		}
		if fd, ok := d.(*ast.FunctionDecl); ok && fd.Name == "main" {
			hasMain = true
		}
	}

	if hasMain {
		g.writeln("")
		g.writeln("main();")
	}

	var out strings.Builder
	if g.needSprintf {
		if isJS {
			out.WriteString("function _xql_sprintf(fmt, ...args) {\n")
		} else {
			out.WriteString("function _xql_sprintf(fmt: string, ...args: unknown[]): string {\n")
		}
		out.WriteString("    let i = 0;\n")
		out.WriteString("    return fmt.replace(/%[sdfo]/g, () => String(args[i++]));\n")
		out.WriteString("}\n\n")
	}
	out.WriteString(g.buf.String())
	return []byte(out.String()), nil
}

type tsGen struct {
	buf         *strings.Builder
	indent      int
	muts        map[string]bool
	needSprintf bool
	isJS        bool
}

func (g *tsGen) write(s string)   { g.buf.WriteString(s) }
func (g *tsGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *tsGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func typeToTS(t ast.TypeExpr) string {
	if t.KindName == "" {
		return "any"
	}
	switch t.KindName {
	case "Int", "Float":
		return "number"
	case "String":
		return "string"
	case "Bool":
		return "boolean"
	case "Void":
		return "void"
	case "Array":
		if t.Elem != nil {
			return typeToTS(*t.Elem) + "[]"
		}
		return "any[]"
	case "Option":
		if t.Elem != nil {
			return typeToTS(*t.Elem) + " | null"
		}
		return "any | null"
	case "Result":
		if t.OkType != nil {
			return typeToTS(*t.OkType)
		}
		return "any"
	default:
		return t.KindName
	}
}

// --- Node emitters ---

func (g *tsGen) emitNode(n ast.Node) error {
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

func (g *tsGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	if g.isJS {
		g.write("const " + ed.Name + " = { ")
		for i, v := range ed.Variants {
			if i > 0 {
				g.write(", ")
			}
			g.write(v + ": " + fmt.Sprintf("%q", v))
		}
		g.writeln(" };")
	} else {
		g.write("enum " + ed.Name + " { ")
		for i, v := range ed.Variants {
			if i > 0 {
				g.write(", ")
			}
			g.write(v)
		}
		g.writeln(" }")
	}
	return nil
}

func (g *tsGen) emitMatchExpr(me *ast.MatchExpr) error {
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

func (g *tsGen) emitStructDecl(sd *ast.StructDecl) error {
	if g.isJS {
		return nil
	}
	g.writeIndent()
	g.writeln("interface " + sd.Name + " {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(f.Name + ": " + typeToTS(f.Type) + ";")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *tsGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)

	g.writeIndent()
	if strings.HasPrefix(fd.Name, "onRequest") {
		g.write("export ")
	}
	if hasAwait(fd.Body) {
		g.write("async ")
	}
	g.write("function " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name)
		if !g.isJS {
			g.write(": " + typeToTS(p.Type))
		}
	}
	g.write(")")

	if !g.isJS {
		g.write(": " + typeToTS(fd.ReturnType))
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

func (g *tsGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *tsGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if g.muts[vd.Name] {
		g.write("let ")
	} else {
		g.write("const ")
	}
	g.write(vd.Name)
	if !g.isJS {
		g.write(": " + typeToTS(vd.Type))
	}
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln(";")
	return nil
}

func (g *tsGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *tsGen) emitIf(is *ast.IfStmt) error {
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

func (g *tsGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *tsGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	if fs.Form == "range" {
		g.write("for (let " + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("; " + fs.Var + " < ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.write("; " + fs.Var + "++")
	} else {
		g.write("for (const " + fs.Var + " of ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
	}
	g.writeln(") {")
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

func (g *tsGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

// --- Expression emitters ---

func (g *tsGen) emitExpr(n ast.Node) error {
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
	case *ast.NewExpr:
		return g.emitNewExpr(node)
	case *ast.AwaitExpr:
		return g.emitAwaitExpr(node)
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

func (g *tsGen) emitArrayLit(al *ast.ArrayLit) error {
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

func (g *tsGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *tsGen) emitStructLit(sl *ast.StructLit) error {
	g.write("{ ")
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

func (g *tsGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("console.log(")
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
			g.needSprintf = true
			g.write("process.stdout.write(_xql_sprintf(")
			for i, arg := range ce.Args {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write("))")
		} else {
			g.write("process.stdout.write(String(")
			if len(ce.Args) > 0 {
				if err := g.emitExpr(ce.Args[0]); err != nil {
					return err
				}
			}
			g.write("))")
		}
		return nil
	case "sprintf":
		if len(ce.Args) >= 2 {
			g.needSprintf = true
			g.write("_xql_sprintf(")
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
			g.write("String(")
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(")")
		} else {
			g.write("\"\"")
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

func (g *tsGen) emitNewExpr(ne *ast.NewExpr) error {
	g.write("new " + ne.Callee + "(")
	for i, arg := range ne.Args {
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

func (g *tsGen) emitAwaitExpr(ae *ast.AwaitExpr) error {
	g.write("await ")
	return g.emitExpr(ae.Expr)
}

func (g *tsGen) emitLiteral(lit *ast.Literal) error {
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

func (g *tsGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *tsGen) emitLambda(lam *ast.Lambda) error {
	if hasAwait(lam.Body) {
		g.write("async ")
	}
	g.write("(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name)
		if !g.isJS {
			g.write(": " + typeToTS(p.Type))
		}
	}
	g.write(")")
	if !g.isJS {
		g.write(": " + typeToTS(lam.ReturnType))
	}
	g.write(" => {")
	if len(lam.Body) == 0 {
		g.write("}")
		return nil
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
	g.write("}")
	return nil
}

func hasAwait(stmts []ast.Node) bool {
	for _, s := range stmts {
		if hasAwaitNode(s) {
			return true
		}
	}
	return false
}

func hasAwaitNode(n ast.Node) bool {
	if n == nil {
		return false
	}
	switch node := n.(type) {
	case *ast.AwaitExpr:
		return true
	case *ast.ExprStmt:
		return hasAwaitNode(node.Expr)
	case *ast.ReturnStmt:
		return hasAwaitNode(node.Value)
	case *ast.VarDecl:
		return hasAwaitNode(node.Value)
	case *ast.AssignStmt:
		return hasAwaitNode(node.Target) || hasAwaitNode(node.Value)
	case *ast.IfStmt:
		for _, s := range node.Then {
			if hasAwaitNode(s) {
				return true
			}
		}
		for _, s := range node.Else {
			if hasAwaitNode(s) {
				return true
			}
		}
	case *ast.WhileStmt:
		for _, s := range node.Body {
			if hasAwaitNode(s) {
				return true
			}
		}
	case *ast.ForStmt:
		if hasAwaitNode(node.Start) || hasAwaitNode(node.End) || hasAwaitNode(node.Iterable) {
			return true
		}
		for _, s := range node.Body {
			if hasAwaitNode(s) {
				return true
			}
		}
	case *ast.BinaryExpr:
		return hasAwaitNode(node.Left) || hasAwaitNode(node.Right)
	case *ast.UnaryExpr:
		return hasAwaitNode(node.Operand)
	case *ast.CallExpr:
		for _, arg := range node.Args {
			if hasAwaitNode(arg) {
				return true
			}
		}
	case *ast.NewExpr:
		for _, arg := range node.Args {
			if hasAwaitNode(arg) {
				return true
			}
		}
	case *ast.MemberExpr:
		return hasAwaitNode(node.Object)
	case *ast.StructLit:
		for _, f := range node.Fields {
			if hasAwaitNode(f.Value) {
				return true
			}
		}
	case *ast.ArrayLit:
		for _, elem := range node.Elements {
			if hasAwaitNode(elem) {
				return true
			}
		}
	case *ast.IndexExpr:
		return hasAwaitNode(node.Target) || hasAwaitNode(node.Index)
	case *ast.IfExpr:
		return hasAwaitNode(node.Cond) || hasAwaitNode(node.Then) || hasAwaitNode(node.Else)
	case *ast.MatchExpr:
		if hasAwaitNode(node.Value) {
			return true
		}
		for _, arm := range node.Arms {
			for _, s := range arm.Body {
				if hasAwaitNode(s) {
					return true
				}
			}
		}
	}
	return false
}
