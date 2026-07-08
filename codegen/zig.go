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

	// Detect Result usage
	walkTypes(root, func(t ast.TypeExpr, context string) {
		if t.KindName == "Result" {
			g.needResult = true
		}
	})

	// Emit import statements first
	for _, d := range prog.Decls {
		if id, ok := d.(*ast.ImportDecl); ok {
			if err := g.emitImportDecl(id); err != nil {
				return nil, err
			}
		}
	}

	// Inject custom Result generic definition if needed
	if g.needResult {
		g.writeln(`fn Result(comptime T: type, comptime E: type) type {
    return struct {
        val: T,
        err: E,
        isOk: bool,

        const Self = @this();
        pub fn unwrap(self: Self) T {
            if (!self.isOk) @panic("Called unwrap on Err Result");
            return self.val;
        }
        pub fn unwrapErr(self: Self) E {
            if (self.isOk) @panic("Called unwrapErr on Ok Result");
            return self.err;
        }
    };
}
`)
	}

	// Emit enum declarations first.
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

	for _, d := range prog.Decls {
		if _, ok := d.(*ast.EnumDecl); ok {
			continue
		}
		if _, ok := d.(*ast.ImportDecl); ok {
			continue
		}
		if !first {
			g.writeln("")
		}
		if err := g.emitNode(d); err != nil {
			return nil, err
		}
		first = false
	}

	var out strings.Builder
	if g.needStd {
		out.WriteString("const std = @import(\"std\");\n\n")
	}
	out.WriteString(g.buf.String())
	return []byte(out.String()), nil
}

type zigGen struct {
	buf         *strings.Builder
	indent      int
	muts        map[string]bool
	needStd     bool
	needResult  bool
	scope       map[string]string // variable/param name → type kind
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
		okType := "void"
		errType := "void"
		if t.OkType != nil {
			okType = typeToZig(*t.OkType)
		}
		if t.ErrType != nil {
			errType = typeToZig(*t.ErrType)
		}
		return "Result(" + okType + ", " + errType + ")"
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
	case *ast.ClassDecl:
		return g.emitClassDecl(node)
	case *ast.EnumDecl:
		return g.emitEnumDecl(node)
	case *ast.MatchExpr:
		return g.emitMatchExpr(node)
	case *ast.SwitchStmt:
		return g.emitSwitchStmt(node)
	case *ast.ImportDecl:
		return g.emitImportDecl(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *zigGen) emitEnumDecl(ed *ast.EnumDecl) error {
	for i, v := range ed.Variants {
		g.writeIndent()
		g.writeln(fmt.Sprintf("pub const %s%s: i64 = %d;", ed.Name, v, i))
	}
	return nil
}

func (g *zigGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("switch (")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(") {")
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.write("else")
		} else {
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
		}
		g.writeln(" => {")
		g.indent++
		for _, s := range arm.Body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln("},")
	}
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *zigGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("pub const " + sd.Name + " = struct {")
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
	g.write("pub fn " + fd.Name + "(")
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
	case *ast.ArrayLiteral:
		return g.emitArrayLiteral(node)
	case *ast.MapLiteral:
		return g.emitMapLiteral(node)
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
		g.write(`std.fmt.allocPrint(std.heap.page_allocator, "`)
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
	case "Result.ok":
		g.write(".{ .val = ")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(", .err = undefined, .isOk = true }")
		return nil
	case "Result.err":
		g.write(".{ .val = undefined, .err = ")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(", .isOk = false }")
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

func (g *zigGen) defaultValue(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "0"
	case "Float":
		return "0.0"
	case "Bool":
		return "false"
	case "String":
		return `""`
	case "Array":
		elemType := "u8"
		if t.Elem != nil {
			elemType = typeToZig(*t.Elem)
		}
		return "&[_]" + elemType + "{}"
	default:
		return "null"
	}
}

func (g *zigGen) emitClassDecl(cd *ast.ClassDecl) error {
	g.writeIndent()
	g.writeln("pub const " + cd.Name + " = struct {")
	g.indent++
	for _, f := range cd.Fields {
		g.writeIndent()
		g.writeln(f.Name + ": " + typeToZig(f.Type) + " = " + g.defaultValue(f.Type) + ",")
	}
	g.indent--
	g.writeIndent()
	g.writeln("};")
	return nil
}

func (g *zigGen) emitSwitchStmt(ss *ast.SwitchStmt) error {
	g.writeIndent()
	g.write("switch (")
	if err := g.emitExpr(ss.Value); err != nil {
		return err
	}
	g.writeln(") {")
	g.indent++
	hasDefault := false
	for _, c := range ss.Cases {
		g.writeIndent()
		if c.Value != nil {
			if err := g.emitExpr(c.Value); err != nil {
				return err
			}
		} else {
			g.write("else")
			hasDefault = true
		}
		g.writeln(" => {")
		g.indent++
		for _, stmt := range c.Body {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln("},")
	}
	if !hasDefault {
		g.writeIndent()
		g.writeln("else => {},")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *zigGen) emitImportDecl(id *ast.ImportDecl) error {
	path := id.Path
	if strings.HasSuffix(path, ".xql.json") {
		path = strings.TrimSuffix(path, ".xql.json") + ".zig"
	} else if strings.HasSuffix(path, ".xql") {
		path = strings.TrimSuffix(path, ".xql") + ".zig"
	}
	g.writeIndent()
	g.writeln(fmt.Sprintf("pub const %s = @import(%q);", id.As, path))
	return nil
}

func (g *zigGen) emitArrayLiteral(al *ast.ArrayLiteral) error {
	elemType := "u8"
	if al.ElemType.KindName != "" {
		elemType = typeToZig(al.ElemType)
	}
	g.write("&[_]" + elemType + "{ ")
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

func (g *zigGen) emitMapLiteral(ml *ast.MapLiteral) error {
	return fmt.Errorf("XQL_E401: Zig target does not support MapLiteral")
}
