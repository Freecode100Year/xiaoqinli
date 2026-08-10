package codegen

import (
	"fmt"
	"path/filepath"
	"strings"

	"xiaoqinli/ast"
)

// GenerateLua produces Lua source code from the given typed AST.
// If the program contains a "main" function, its body is emitted at top level.
// If the program is a module (no "main" function), declarations are exported via a module table 'M'.
func GenerateLua(root ast.Node) ([]byte, error) {
	g := &luaGen{buf: &strings.Builder{}}
	g.types = newTypeKinds(root)

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	hasMain := false
	for _, d := range prog.Decls {
		if fd, ok := d.(*ast.FunctionDecl); ok && fd.Name == "main" {
			hasMain = true
			break
		}
	}

	// Helper for Result type
	g.writeln("local Result = {")
	g.writeln("    ok = function(v) return { isOk = true, val = v, unwrap = function() return v end } end,")
	g.writeln("    err = function(e) return { isOk = false, errVal = e, unwrapErr = function() return e end } end")
	g.writeln("}")
	g.writeln("")

	// Emit import declarations
	for _, d := range prog.Decls {
		if id, ok := d.(*ast.ImportDecl); ok {
			if err := g.emitImportDecl(id); err != nil {
				return nil, err
			}
		}
	}

	if !hasMain {
		g.writeln("local M = {}")
		g.writeln("")
	}

	first := true
	// Emit enum declarations.
	for _, d := range prog.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			if !first {
				g.writeln("")
			}
			if err := g.emitEnumDecl(ed, !hasMain); err != nil {
				return nil, err
			}
			first = false
		}
	}

	// Emit function declarations (non-main).
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name == "main" {
			continue
		}
		if !first {
			g.writeln("")
		}
		if err := g.emitFunctionDecl(fd, !hasMain); err != nil {
			return nil, err
		}
		first = false
	}

	if hasMain {
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
	} else {
		g.writeln("")
		g.writeln("return M")
	}

	return []byte(g.buf.String()), nil
}

type luaGen struct {
	types  *typeKinds
	buf    *strings.Builder
	indent int
}

func (g *luaGen) write(s string)   { g.buf.WriteString(s) }
func (g *luaGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *luaGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func (g *luaGen) emitNode(n ast.Node) error {
	switch node := n.(type) {
	case *ast.ImportDecl:
		return g.emitImportDecl(node)
	case *ast.FunctionDecl:
		return g.emitFunctionDecl(node, false)
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
		return fmt.Errorf("XQL_E401: Lua does not support continue")
	case *ast.ExprStmt:
		return g.emitExprStmt(node)
	case *ast.StructDecl:
		return nil // Lua uses tables; no struct declaration needed.
	case *ast.EnumDecl:
		return g.emitEnumDecl(node, false)
	case *ast.MatchExpr:
		return g.emitMatchExpr(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *luaGen) emitImportDecl(id *ast.ImportDecl) error {
	g.writeIndent()
	path := id.Path
	if strings.HasSuffix(path, ".xql") {
		path = path[:len(path)-4]
	}
	if strings.HasPrefix(path, "./") {
		path = path[2:]
	}
	if strings.HasPrefix(path, ".\\") {
		path = path[2:]
	}
	alias := id.As
	if alias == "" {
		alias = filepath.Base(path)
	}
	g.writeln(fmt.Sprintf("local %s = require(%q)", alias, path))
	return nil
}

func (g *luaGen) emitEnumDecl(ed *ast.EnumDecl, isModule bool) error {
	for i, v := range ed.Variants {
		g.writeIndent()
		if isModule {
			g.writeln(fmt.Sprintf("M.%s = %d", v, i))
		}
		g.writeln(fmt.Sprintf("%s = %d", v, i))
	}
	return nil
}

func (g *luaGen) emitMatchExpr(me *ast.MatchExpr) error {
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
			g.writeln(" then")
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

func (g *luaGen) emitFunctionDecl(fd *ast.FunctionDecl, isModule bool) error {
	g.types.noteParams(fd)
	g.writeIndent()
	funcName := fd.Name
	if isModule {
		g.write("function M." + funcName + "(")
	} else {
		g.write("function " + funcName + "(")
	}
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name)
	}
	g.writeln(")")
	g.indent++
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("end")
	if isModule {
		g.writeIndent()
		g.writeln(fmt.Sprintf("%s = M.%s", funcName, funcName))
	}
	return nil
}

func (g *luaGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *luaGen) emitVarDecl(vd *ast.VarDecl) error {
	g.types.noteVar(vd)
	g.writeIndent()
	g.write("local " + vd.Name)
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln("")
	return nil
}

func (g *luaGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *luaGen) emitIf(is *ast.IfStmt) error {
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
	g.writeln("end")
	return nil
}

func (g *luaGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while ")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(" do")
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

func (g *luaGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for " + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write(", ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(" - 1 do")
	case "each":
		g.write("for _, " + fs.Var + " in ipairs(")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(") do")
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
	g.writeln("end")
	return nil
}

func (g *luaGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *luaGen) emitExpr(n ast.Node) error {
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
			op = "~="
		case "/":
			// Lua 5.3 made `/` float division, so `7 / 2` is 3.5 and prints as
			// "3.5". `//` is the integer one.
			if g.types.isIntDivision(node) {
				op = "//"
			}
		}
		if op == "+" && containsStringExpr(node) {
			op = ".."
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

func (g *luaGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("(")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" and ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" or ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *luaGen) emitLambda(lam *ast.Lambda) error {
	g.write("function(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name)
	}
	g.writeln(")")
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

func (g *luaGen) emitArrayLit(elements []ast.Node) error {
	g.write("{")
	for i, elem := range elements {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(elem); err != nil {
			return err
		}
	}
	g.write("}")
	return nil
}

func (g *luaGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *luaGen) emitStructLit(sl *ast.StructLit) error {
	g.write("{ ")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + " = ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(" }")
	return nil
}

func (g *luaGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("print(")
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
			g.write("io.write(string.format(")
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
			g.write("io.write(")
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
			g.write("string.format(")
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
			g.write("tostring(")
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

func (g *luaGen) emitLiteral(lit *ast.Literal) error {
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
