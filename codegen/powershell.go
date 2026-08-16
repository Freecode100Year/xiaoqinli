package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GeneratePowerShell produces PowerShell source code from the given typed AST.
// The "main" function's body is emitted at top level after function definitions.
func GeneratePowerShell(root ast.Node) ([]byte, error) {
	g := &psGen{buf: &strings.Builder{}}
	g.enums = collectEnums(root)
	g.types = newTypeKinds(root)

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	first := true

	// Emit enum declarations.
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

	// Emit struct declarations as classes.
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

	// Emit non-main function declarations.
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

	// Emit main body at top level.
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

	return []byte(g.buf.String()), nil
}

type psGen struct {
	types  *typeKinds
	buf    *strings.Builder
	indent int
	enums  map[string]*ast.EnumDecl
}

func (g *psGen) write(s string)   { g.buf.WriteString(s) }
func (g *psGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *psGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func typeToPowerShell(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "[long]"
	case "Float":
		return "[double]"
	case "String":
		return "[string]"
	case "Bool":
		return "[bool]"
	case "Void":
		return "[void]"
	case "Array":
		return "[array]"
	default:
		return "[" + t.KindName + "]"
	}
}

func (g *psGen) emitNode(n ast.Node) error {
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

func (g *psGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("class " + sd.Name + " {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(typeToPowerShell(f.Type) + "$" + f.Name)
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *psGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.writeln("enum " + ed.Name + " {")
	g.indent++
	for _, v := range ed.Variants {
		g.writeIndent()
		g.writeln(v)
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *psGen) emitMatchExpr(me *ast.MatchExpr) error {
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
			g.writeln("default {")
		} else if _, isLit := arm.Pattern.(*ast.Literal); isLit {
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.writeln(" {")
		} else {
			// A switch clause that is not a constant has to be a script block.
			// PowerShell matches a clause by value, and `[Color]::Red` in
			// clause position is read as a bareword rather than evaluated — the
			// switch then compared the subject against that text, matched
			// nothing, and every input took the default arm.
			g.write("{ $_ -eq ")
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.writeln(" } {")
		}
		g.indent++
		for _, stmt := range arm.Body {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln("}")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *psGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.types.noteParams(fd)
	g.writeIndent()
	g.write("function " + fd.Name + " {")
	g.writeln("")
	g.indent++
	if len(fd.Params) > 0 {
		g.writeIndent()
		g.write("param(")
		for i, p := range fd.Params {
			if i > 0 {
				g.write(", ")
			}
			g.write(typeToPowerShell(p.Type) + "$" + p.Name)
		}
		g.writeln(")")
	}
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

func (g *psGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *psGen) emitVarDecl(vd *ast.VarDecl) error {
	g.types.noteVar(vd)
	g.writeIndent()
	g.write("$" + vd.Name)
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	} else {
		g.write(" = $null")
	}
	g.writeln("")
	return nil
}

func (g *psGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *psGen) emitIf(is *ast.IfStmt) error {
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
	if len(is.Else) > 0 {
		g.writeIndent()
		g.writeln("} else {")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
	}
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *psGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *psGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for ($" + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("; $" + fs.Var + " -lt ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln("; $" + fs.Var + "++) {")
	case "each":
		g.write("foreach ($" + fs.Var + " in ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(") {")
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
	g.writeln("}")
	return nil
}

func (g *psGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *psGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write("$" + node.Name)
		return nil
	case *ast.BinaryExpr:
		// PowerShell's `/` yields a Double, and casting the result to [int]
		// would round half to even (7/2 -> 4). Truncate is the one that agrees
		// with the other targets.
		if g.types.isIntDivision(node) {
			// The outer parens are not cosmetic. `Write-Output [long][math]::…`
			// parses in argument mode, where PowerShell hands the cast along as
			// a literal and prints "[long][math]::Truncate" followed by 3.5.
			// Wrapped, it is an expression and prints 3.
			g.write("([long][math]::Truncate(")
			if err := g.emitExpr(node.Left); err != nil {
				return err
			}
			g.write(" / ")
			if err := g.emitExpr(node.Right); err != nil {
				return err
			}
			g.write("))")
			return nil
		}
		g.write("(")
		if err := g.emitExpr(node.Left); err != nil {
			return err
		}
		op := node.Op
		switch op {
		case "==":
			op = "-eq"
		case "!=":
			op = "-ne"
		case "<":
			op = "-lt"
		case ">":
			op = "-gt"
		case "<=":
			op = "-le"
		case ">=":
			op = "-ge"
		case "&&":
			op = "-and"
		case "||":
			op = "-or"
		}
		if op == "+" && stringValued(g.types, node) {
			op = "+"
		}
		g.write(" " + op + " ")
		if err := g.emitExpr(node.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	case *ast.UnaryExpr:
		if node.Op == "!" {
			g.write("-not ")
		} else {
			g.write(node.Op)
		}
		return g.emitExpr(node.Operand)
	case *ast.CallExpr:
		return g.emitCall(node)
	case *ast.MemberExpr:
		// PowerShell reaches an enum member through the type literal, and `$Color` is an undefined variable.
		if enum, variant, ok := enumRef(g.enums, node); ok {
			g.write("[" + enum + "]::" + variant)
			return nil
		}
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

func (g *psGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("$(if (")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(") { ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" } else { ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(" })")
	return nil
}

func (g *psGen) emitLambda(lam *ast.Lambda) error {
	g.write("{ ")
	if len(lam.Params) > 0 {
		g.write("param(")
		for i, p := range lam.Params {
			if i > 0 {
				g.write(", ")
			}
			g.write(typeToPowerShell(p.Type) + "$" + p.Name)
		}
		g.write(") ")
	}
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
		} else {
			if err := g.emitExpr(stmt); err != nil {
				return err
			}
		}
	}
	g.write(" }")
	return nil
}

func (g *psGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("@(")
	for i, elem := range al.Elements {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(elem); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *psGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *psGen) emitStructLit(sl *ast.StructLit) error {
	g.write("[" + sl.TypeName + "]@{ ")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write("; ")
		}
		g.write(f.Name + " = ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(" }")
	return nil
}

func (g *psGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("Write-Output ")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		} else {
			g.write("\"\"")
		}
		return nil
	case "printf":
		g.write("Write-Host -NoNewline ")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		return nil
	case "sprintf":
		g.write("[string]::Format(")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	default:
		// Each argument goes in its own parentheses. In command invocation
		// syntax — `describe [Color]::Green` — PowerShell treats an argument
		// that is not a variable or a quoted string as a bareword and passes
		// the text of it: the parameter binder then reported that it could not
		// convert the string "[Color]::Green" to type Color. A variable
		// argument happened to work, which is every argument the corpus had
		// until an enum variant became one, and parenthesising is what makes
		// the general case an expression.
		g.write("(")
		g.write(ce.Callee)
		for _, arg := range ce.Args {
			g.write(" (")
			if err := g.emitExpr(arg); err != nil {
				return err
			}
			g.write(")")
		}
		g.write(")")
		return nil
	}
}

func (g *psGen) emitLiteral(lit *ast.Literal) error {
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
		if b {
			g.write("$true")
		} else {
			g.write("$false")
		}
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}
