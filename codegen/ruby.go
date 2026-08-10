package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateRuby produces Ruby source code from the given typed AST.
// The "main" function's body is emitted at top level.
func GenerateRuby(root ast.Node) ([]byte, error) {
	strat := InspectCodegenStrategy("ruby")
	g := &rbGen{buf: &strings.Builder{}, strat: strat, imports: make(map[string]bool)}
	g.types = newTypeKinds(root)

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	if strat != nil && (strat.OptimizationFlags["header_comment"] == "true" || strat.OptimizationFlags["strategy_tag"] != "" || strat.PreferComprehension) {
		tag := strat.OptimizationFlags["strategy_tag"]
		if tag == "" {
			tag = fmt.Sprintf("PreferComprehension=%v, InlineThreshold=%d, Score=%.1f", strat.PreferComprehension, strat.InlineThreshold, strat.BenchmarkScore)
		}
		g.writeln("# Codegen Strategy: " + tag)
		g.writeln("")
	}

	// Result helper class, emitted unconditionally like Lua's Result table.
	// Callee/field passthrough (Result.ok, Result.err, res.unwrap, res.isOk)
	// only needs matching method names, not shared identity, so reopening
	// this class in every generated file when a program is split across
	// several .rb files via require_relative is harmless.
	g.writeln("class Result")
	g.writeln("  def self.ok(v)")
	g.writeln("    new(true, v, nil)")
	g.writeln("  end")
	g.writeln("  def self.err(e)")
	g.writeln("    new(false, nil, e)")
	g.writeln("  end")
	g.writeln("  def initialize(is_ok, val, err_val)")
	g.writeln("    @isOk = is_ok")
	g.writeln("    @val = val")
	g.writeln("    @err_val = err_val")
	g.writeln("  end")
	g.writeln("  def isOk")
	g.writeln("    @isOk")
	g.writeln("  end")
	g.writeln("  def unwrap")
	g.writeln("    @val")
	g.writeln("  end")
	g.writeln("  def unwrapErr")
	g.writeln("    @err_val")
	g.writeln("  end")
	g.writeln("end")
	g.writeln("")

	// Collect imports upfront and emit require_relative for multi-file symbol
	// resolution. require_relative pulls the imported file's classes and
	// methods into the same top-level scope, so the alias prefix is dropped
	// from call sites and struct literals (see stripImportAlias).
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

	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		if !first {
			g.writeln("")
		}
		for _, stmt := range fd.Body {
			if err := g.emitNode(stmt); err != nil {
				return nil, err
			}
		}
	}

	preamble := ""
	if g.needIntDiv {
		preamble = `def _xql_idiv(a, b)
  q = a / b
  q += 1 if q < 0 && q * b != a
  q
end

def _xql_irem(a, b)
  a - b * _xql_idiv(a, b)
end

`
	}
	return []byte(preamble + g.buf.String()), nil
}

type rbGen struct {
	types      *typeKinds
	needIntDiv bool
	buf        *strings.Builder
	indent     int
	strat      *CodegenStrategyConfig
	imports    map[string]bool
}

// stripImportAlias removes a leading import-alias qualifier from a symbol
// name. XQL writes cross-module references as "models.User"; require_relative
// flattens everything into one top-level scope, so Ruby needs plain "User".
// Names whose prefix is not a known import alias are returned unchanged, which
// keeps ordinary method calls such as "res.unwrap" intact.
func (g *rbGen) stripImportAlias(name string) string {
	idx := strings.Index(name, ".")
	if idx == -1 {
		return name
	}
	if g.imports[name[:idx]] {
		return name[idx+1:]
	}
	return name
}

func (g *rbGen) write(s string)   { g.buf.WriteString(s) }
func (g *rbGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *rbGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("  ")
	}
}

func (g *rbGen) emitNode(n ast.Node) error {
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
		g.writeln("next")
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

func (g *rbGen) emitImportDecl(id *ast.ImportDecl) error {
	g.writeIndent()
	path := id.Path
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, ".\\")

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
		path = path[:len(path)-4] + ".rb"
	}
	g.writeln(fmt.Sprintf("require_relative %q", path))
	return nil
}

func (g *rbGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.writeln("module " + ed.Name)
	g.indent++
	for i, v := range ed.Variants {
		g.writeIndent()
		g.writeln(fmt.Sprintf("%s = %d", v, i))
	}
	g.indent--
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *rbGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("case ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln("")
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.writeln("else")
		} else {
			g.write("when ")
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
	}
	g.writeIndent()
	g.writeln("end")
	return nil
}

func (g *rbGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.write(sd.Name + " = Struct.new(")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(":" + f.Name)
	}
	g.writeln(")")
	return nil
}

func (g *rbGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.types.noteParams(fd)
	g.writeIndent()
	g.write("def " + fd.Name)
	if len(fd.Params) > 0 {
		g.write("(")
		for i, p := range fd.Params {
			if i > 0 {
				g.write(", ")
			}
			g.write(p.Name)
		}
		g.write(")")
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

func (g *rbGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *rbGen) emitVarDecl(vd *ast.VarDecl) error {
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

func (g *rbGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *rbGen) emitIf(is *ast.IfStmt) error {
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

func (g *rbGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *rbGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	if fs.Form == "each" && g.strat != nil && g.strat.PreferComprehension && len(fs.Body) == 1 {
		if es, ok := fs.Body[0].(*ast.ExprStmt); ok {
			if call, ok := es.Expr.(*ast.CallExpr); ok && (call.Callee == "push" || strings.HasSuffix(call.Callee, ".push") || strings.HasSuffix(call.Callee, ".concat")) {
				parts := strings.Split(call.Callee, ".")
				if len(parts) == 2 && len(call.Args) == 1 {
					targetVar := parts[0]
					g.write(targetVar + " += ")
					if err := g.emitExpr(fs.Iterable); err != nil {
						return err
					}
					g.write(".map { |" + fs.Var + "| ")
					if err := g.emitExpr(call.Args[0]); err != nil {
						return err
					}
					g.writeln(" }")
					return nil
				}
			}
		}
	}

	switch fs.Form {
	case "range":
		g.write("(")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("...")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(").each do |" + fs.Var + "|")
	case "each":
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(".each do |" + fs.Var + "|")
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

func (g *rbGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *rbGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		// Ruby's Integer#/ floors and its % takes the sign of the divisor, so
		// -7 / 2 is -4 and -7 % 2 is 1 where C and Go say -3 and -1.
		if g.types.isIntDivision(node) || g.types.isIntRemainder(node) {
			g.needIntDiv = true
			if node.Op == "%" {
				g.write("_xql_irem(")
			} else {
				g.write("_xql_idiv(")
			}
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

func (g *rbGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *rbGen) emitLambda(lam *ast.Lambda) error {
	g.write("lambda { ")
	if len(lam.Params) > 0 {
		g.write("|")
		for i, p := range lam.Params {
			if i > 0 {
				g.write(", ")
			}
			g.write(p.Name)
		}
		g.write("| ")
	}
	if len(lam.Body) == 1 {
		if rs, ok := lam.Body[0].(*ast.ReturnStmt); ok && rs.Value != nil {
			if err := g.emitExpr(rs.Value); err != nil {
				return err
			}
			g.write(" }")
			return nil
		}
		if es, ok := lam.Body[0].(*ast.ExprStmt); ok {
			if err := g.emitExpr(es.Expr); err != nil {
				return err
			}
			g.write(" }")
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
	g.write("}")
	return nil
}

func (g *rbGen) emitArrayLit(elements []ast.Node) error {
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

func (g *rbGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *rbGen) emitStructLit(sl *ast.StructLit) error {
	g.write(g.stripImportAlias(sl.TypeName) + ".new(")
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

func (g *rbGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("puts(")
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
			g.write("print(")
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
		} else if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(".to_s")
		} else {
			g.write(`""`)
		}
		return nil
	default:
		g.write(g.stripImportAlias(ce.Callee) + "(")
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

func (g *rbGen) emitLiteral(lit *ast.Literal) error {
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
