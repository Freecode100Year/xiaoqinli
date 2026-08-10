package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateJulia produces Julia source code from the given typed AST.
// A main() call is appended at the bottom if main is defined.
func GenerateJulia(root ast.Node) ([]byte, error) {
	g := &jlGen{buf: &strings.Builder{}, imports: make(map[string]bool)}
	g.types = newTypeKinds(root)

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Result helper, emitted unconditionally. Fields are untyped (Any) rather
	// than a true parametric struct: Julia can't infer the unused branch's
	// type parameter from a single-argument constructor call (resultOk(v)
	// has no way to know E), and the source language doesn't need static
	// precision here — this mirrors the interface{}-based Result already
	// used by the Go backend.
	g.writeln("mutable struct Result")
	g.writeln("    isOk::Bool")
	g.writeln("    val::Any")
	g.writeln("    errVal::Any")
	g.writeln("end")
	g.writeln("resultOk(v) = Result(true, v, nothing)")
	g.writeln("resultErr(e) = Result(false, nothing, e)")
	g.writeln("xqlUnwrap(r::Result) = r.val")
	g.writeln("xqlUnwrapErr(r::Result) = r.errVal")
	g.writeln("")

	// Emit every include up front so the alias table is complete before any
	// declaration that might reference an imported symbol is generated.
	// include() splices the imported file into this same scope, so the alias
	// qualifier is then dropped everywhere (see stripImportAlias).
	hasImport := false
	for _, d := range prog.Decls {
		if id, ok := d.(*ast.ImportDecl); ok {
			if err := g.emitImportDecl(id); err != nil {
				return nil, err
			}
			hasImport = true
		}
	}
	if hasImport {
		g.writeln("")
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
		switch d.(type) {
		case *ast.EnumDecl, *ast.ImportDecl:
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
		g.writeln("main()")
	}

	return []byte(g.buf.String()), nil
}

type jlGen struct {
	types   *typeKinds
	buf     *strings.Builder
	indent  int
	imports map[string]bool
}

// stripImportAlias removes a leading import-alias qualifier from a symbol name.
// XQL writes cross-module references as "models.User", but include() splices
// the imported file into the current scope rather than creating a module named
// "models", so Julia needs plain "User". Names whose prefix is not a declared
// alias are returned unchanged, leaving field access such as "config.retries"
// intact.
func (g *jlGen) stripImportAlias(name string) string {
	idx := strings.Index(name, ".")
	if idx <= 0 {
		return name
	}
	if g.imports[name[:idx]] {
		return name[idx+1:]
	}
	return name
}

func (g *jlGen) write(s string)   { g.buf.WriteString(s) }
func (g *jlGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *jlGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func (g *jlGen) typeToJulia(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "Int64"
	case "Float":
		return "Float64"
	case "String":
		return "String"
	case "Bool":
		return "Bool"
	case "Void":
		return "Nothing"
	case "Array":
		if t.Elem != nil {
			return "Vector{" + g.typeToJulia(*t.Elem) + "}"
		}
		return "Vector{Any}"
	case "Option":
		if t.Elem != nil {
			return "Union{" + g.typeToJulia(*t.Elem) + ", Nothing}"
		}
		return "Union{Any, Nothing}"
	case "Result":
		return "Result"
	default:
		return g.stripImportAlias(t.KindName)
	}
}

func (g *jlGen) emitNode(n ast.Node) error {
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

func (g *jlGen) emitImportDecl(id *ast.ImportDecl) error {
	g.writeIndent()
	path := id.Path

	alias := id.As
	if alias == "" {
		base := path
		if idx := strings.LastIndexAny(base, "/\\"); idx != -1 {
			base = base[idx+1:]
		}
		alias = strings.TrimSuffix(base, ".xql")
	}
	if alias != "" {
		g.imports[alias] = true
	}

	if strings.HasSuffix(path, ".xql") {
		path = path[:len(path)-4] + ".jl"
	}
	g.writeln(fmt.Sprintf("include(%q)", path))
	return nil
}

func (g *jlGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.write("@enum " + ed.Name + " ")
	for i, v := range ed.Variants {
		if i > 0 {
			g.write(" ")
		}
		g.write(v)
	}
	g.writeln("")
	return nil
}

func (g *jlGen) emitMatchExpr(me *ast.MatchExpr) error {
	first := true
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.writeln("else")
		} else {
			if first {
				g.write("if ")
			} else {
				g.write("elseif ")
			}
			if err := g.emitExpr(me.Value); err != nil {
				return err
			}
			g.write(" == ")
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.writeln("")
		}
		g.indent++
		for _, s := range arm.Body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
		first = false
	}
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *jlGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("struct " + sd.Name)
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(f.Name + "::" + g.typeToJulia(f.Type))
	}
	g.indent--
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *jlGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.types.noteParams(fd)
	g.writeIndent()
	g.write("function " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + "::" + g.typeToJulia(p.Type))
	}
	g.write(")")
	rt := g.typeToJulia(fd.ReturnType)
	if rt != "" && rt != "Nothing" {
		g.write("::" + rt)
	}
	g.writeln("")
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

func (g *jlGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *jlGen) emitVarDecl(vd *ast.VarDecl) error {
	g.types.noteVar(vd)
	g.writeIndent()
	g.write(vd.Name + "::" + g.typeToJulia(vd.Type))
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln("")
	return nil
}

func (g *jlGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *jlGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if ")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln("")
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

func (g *jlGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while ")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln("")
	g.indent++
	for _, s := range ws.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *jlGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for " + fs.Var + " in ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write(":(")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(" - 1)")
	case "each":
		g.write("for " + fs.Var + " in ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln("")
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

func (g *jlGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *jlGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		// Julia's `/` promotes to Float64 even for two Int64s.
		if g.types.isIntDivision(node) {
			g.write("div(")
			if err := g.emitExpr(node.Left); err != nil {
				return err
			}
			g.write(", ")
			if err := g.emitExpr(node.Right); err != nil {
				return err
			}
			g.write(")")
			return nil
		}
		g.write("(")
		if err := g.emitExpr(node.Left); err != nil {
			return err
		}
		op := node.Op
		if op == "+" && containsStringExpr(node) {
			op = "*"
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
		return g.emitArrayLit(g.typeToJulia(node.ElemType), node.Elements)
	case *ast.ArrayLiteral:
		return g.emitArrayLit(g.typeToJulia(node.ElemType), node.Elements)
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

func (g *jlGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *jlGen) emitLambda(lam *ast.Lambda) error {
	g.write("(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name)
	}
	g.write(") -> ")
	if len(lam.Body) == 1 {
		if rs, ok := lam.Body[0].(*ast.ReturnStmt); ok && rs.Value != nil {
			return g.emitExpr(rs.Value)
		}
		if es, ok := lam.Body[0].(*ast.ExprStmt); ok {
			return g.emitExpr(es.Expr)
		}
	}
	g.writeln("begin")
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

func (g *jlGen) emitArrayLit(elemTypeStr string, elements []ast.Node) error {
	g.write(elemTypeStr + "[")
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

func (g *jlGen) emitIndexExpr(ie *ast.IndexExpr) error {
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write("[(")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write(") + 1]")
	return nil
}

func (g *jlGen) emitStructLit(sl *ast.StructLit) error {
	g.write(g.stripImportAlias(sl.TypeName) + "(")
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

func (g *jlGen) emitCall(ce *ast.CallExpr) error {
	// Result is a struct, not a namespace, so Julia has no "Result.ok(v)"
	// call syntax; and res.unwrap()/res.unwrapErr() are free functions here
	// (see the Result helper emitted at the top of the file), not struct
	// methods, so they can't use Julia's obj.field(...) call sugar either.
	// Both need rewriting to plain function calls before the generic
	// passthrough below.
	callee := g.stripImportAlias(ce.Callee)
	switch callee {
	case "Result.ok":
		callee = "resultOk"
	case "Result.err":
		callee = "resultErr"
	default:
		if idx := strings.LastIndex(callee, "."); idx > 0 {
			obj, method := callee[:idx], callee[idx+1:]
			switch method {
			case "unwrap":
				g.write("xqlUnwrap(" + obj + ")")
				return nil
			case "unwrapErr":
				g.write("xqlUnwrapErr(" + obj + ")")
				return nil
			}
		}
	}
	switch callee {
	case "println":
		g.write("println(")
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
		g.write("print(")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "sprintf":
		g.write("string(")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	default:
		g.write(callee + "(")
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

func (g *jlGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write(fmt.Sprintf("%q", s))
	case "Int":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("Int64(%d)", int64(f)))
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
