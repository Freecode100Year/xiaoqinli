package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateZig produces Zig source code from the given typed AST.
func GenerateZig(root ast.Node) ([]byte, error) {
	g := &zigGen{buf: &strings.Builder{}, funcReturns: make(map[string]string)}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Collect function return types for format specifier inference.
	for _, d := range prog.Decls {
		if fd, ok := d.(*ast.FunctionDecl); ok {
			g.funcReturns[fd.Name] = fd.ReturnType.KindName
		}
	}

	for i, d := range prog.Decls {
		if i > 0 {
			g.writeln("")
		}
		if err := g.emitNode(d); err != nil {
			return nil, err
		}
	}

	var out strings.Builder
	if g.needStd {
		out.WriteString("const std = @import(\"std\");\n\n")
	}
	out.WriteString(g.buf.String())
	return []byte(out.String()), nil
}

type zigGen struct {
	buf       *strings.Builder
	indent    int
	muts      map[string]bool
	needStd   bool
	scope     map[string]string // variable/param name → type kind
	funcReturns map[string]string // function name → return type kind
}

func (g *zigGen) write(s string)   { g.buf.WriteString(s) }
func (g *zigGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *zigGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func typeToZig(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "i64"
	case "Float":
		return "f64"
	case "String":
		return "[]const u8"
	case "Bool":
		return "bool"
	case "Void":
		return "void"
	case "Array":
		if t.Elem != nil {
			return "[]const " + typeToZig(*t.Elem)
		}
		return "[]const u8"
	case "Option":
		if t.Elem != nil {
			return "?" + typeToZig(*t.Elem)
		}
		return "?*anyopaque"
	case "Result":
		if t.OkType != nil {
			return typeToZig(*t.OkType) + "!void"
		}
		return "anyerror!void"
	default:
		return t.KindName
	}
}

func (g *zigGen) emitNode(n ast.Node) error {
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
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *zigGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("const " + sd.Name + " = struct {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(f.Name + ": " + typeToZig(f.Type) + ",")
	}
	g.indent--
	g.writeIndent()
	g.writeln("};")
	return nil
}

func (g *zigGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)
	g.scope = make(map[string]string)
	for _, p := range fd.Params {
		g.scope[p.Name] = p.Type.KindName
	}

	g.writeIndent()
	if fd.Name == "main" {
		g.write("pub ")
	}
	g.write("fn " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + ": " + typeToZig(p.Type))
	}
	g.write(") " + typeToZig(fd.ReturnType))
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

func (g *zigGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *zigGen) emitVarDecl(vd *ast.VarDecl) error {
	if g.scope != nil {
		g.scope[vd.Name] = vd.Type.KindName
	}
	g.writeIndent()
	if g.muts[vd.Name] {
		g.write("var ")
	} else {
		g.write("const ")
	}
	g.write(vd.Name + ": " + typeToZig(vd.Type))
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln(";")
	return nil
}

func (g *zigGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *zigGen) emitIf(is *ast.IfStmt) error {
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

func (g *zigGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *zigGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("var " + fs.Var + ": i64 = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.writeln(";")
		g.writeIndent()
		g.write("while (" + fs.Var + " < ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(") : (" + fs.Var + " += 1) {")
	case "each":
		g.write("for (")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(") |" + fs.Var + "| {")
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

func (g *zigGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *zigGen) emitExpr(n ast.Node) error {
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
			op = "++"
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

func (g *zigGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("if (")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(") ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" else ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	return nil
}

func (g *zigGen) emitLambda(lam *ast.Lambda) error {
	return fmt.Errorf("XQL_E401: Zig does not support Lambda expressions")
}

func (g *zigGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("&[_]" + typeToZig(al.ElemType) + "{ ")
	for i, elem := range al.Elements {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(elem); err != nil {
			return err
		}
	}
	g.write(" }")
	return nil
}

func (g *zigGen) emitIndexExpr(ie *ast.IndexExpr) error {
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write("[@intCast(")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write(")]")
	return nil
}

func (g *zigGen) emitStructLit(sl *ast.StructLit) error {
	g.write(sl.TypeName + "{ ")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write("." + f.Name + " = ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(" }")
	return nil
}

// exprIsString checks whether an expression evaluates to a String type.
func (g *zigGen) exprIsString(n ast.Node) bool {
	switch node := n.(type) {
	case *ast.Literal:
		return node.ValueType == "String"
	case *ast.Ident:
		if g.scope != nil {
			return g.scope[node.Name] == "String"
		}
		return false
	case *ast.CallExpr:
		if node.Callee == "sprintf" {
			return true
		}
		if rt, ok := g.funcReturns[node.Callee]; ok {
			return rt == "String"
		}
		return false
	case *ast.BinaryExpr:
		if node.Op == "+" {
			return g.exprIsString(node.Left) || g.exprIsString(node.Right)
		}
		return false
	default:
		return false
	}
}

// zigFmtSpec returns "{s}" for string-typed expressions, "{}" for others.
func (g *zigGen) zigFmtSpec(n ast.Node) string {
	if g.exprIsString(n) {
		return "{s}"
	}
	return "{}"
}

func (g *zigGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.needStd = true
		g.write(`std.debug.print("`)
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(" ")
			}
			g.write(g.zigFmtSpec(arg))
		}
		g.write(`\n", .{`)
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(", ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write("})")
		return nil
	case "printf":
		g.needStd = true
		g.write(`std.debug.print("`)
		if len(ce.Args) > 0 {
			g.write(g.zigFmtSpec(ce.Args[0]))
		}
		g.write(`", .{`)
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write("})")
		return nil
	case "sprintf":
		g.needStd = true
		g.write(`std.fmt.comptimePrint("`)
		if len(ce.Args) > 0 {
			g.write(g.zigFmtSpec(ce.Args[0]))
		}
		g.write(`", .{`)
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write("})")
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

func (g *zigGen) emitLiteral(lit *ast.Literal) error {
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
