package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GeneratePerl produces Perl source code from the given typed AST.
// The "main" function's body is emitted at top level after sub definitions.
func GeneratePerl(root ast.Node) ([]byte, error) {
	g := &perlGen{buf: &strings.Builder{}}
	g.enums = collectEnums(root)
	g.types = newTypeKinds(root)

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	g.writeln("use strict;")
	g.writeln("use warnings;")
	g.writeln("")

	// Emit enum declarations as use constant.
	for _, d := range prog.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			if err := g.emitEnumDecl(ed); err != nil {
				return nil, err
			}
			g.writeln("")
		}
	}

	// Emit struct declarations as comments (Perl uses hash refs).
	for _, d := range prog.Decls {
		if sd, ok := d.(*ast.StructDecl); ok {
			if err := g.emitStructDecl(sd); err != nil {
				return nil, err
			}
			g.writeln("")
		}
	}

	// Emit non-main sub declarations.
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

	// Emit main body at top level.
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

	preamble := ""
	if g.needIntRem {
		preamble = "sub _xql_irem { my ($x, $y) = @_; return $x - $y * int($x / $y); }\n\n"
	}
	return []byte(preamble + g.buf.String()), nil
}

type perlGen struct {
	needIntRem bool
	types      *typeKinds
	buf        *strings.Builder
	indent     int
	enums      map[string]*ast.EnumDecl
}

func (g *perlGen) write(s string)   { g.buf.WriteString(s) }
func (g *perlGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *perlGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func (g *perlGen) emitNode(n ast.Node) error {
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
		g.writeln("last;")
		return nil
	case *ast.ContinueStmt:
		g.writeIndent()
		g.writeln("next;")
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

func (g *perlGen) emitStructDecl(sd *ast.StructDecl) error {
	// Perl uses hash refs for structs; emit a comment.
	g.writeIndent()
	g.write("# struct " + sd.Name + ": ")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name)
	}
	g.writeln("")
	return nil
}

func (g *perlGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.write("use constant { ")
	for i, v := range ed.Variants {
		if i > 0 {
			g.write(", ")
		}
		g.write(fmt.Sprintf("%s => %d", v, i))
	}
	g.writeln(" };")
	return nil
}

func (g *perlGen) emitMatchExpr(me *ast.MatchExpr) error {
	// Perl: use if/elsif chain (given/when is experimental).
	first := true
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			if !first {
				g.write("} else {")
			} else {
				g.write("if (1) {")
			}
		} else {
			if first {
				g.write("if (")
			} else {
				g.write("} elsif (")
			}
			if err := g.emitExpr(me.Value); err != nil {
				return err
			}
			g.write(" eq ")
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.write(") {")
		}
		g.writeln("")
		g.indent++
		for _, stmt := range arm.Body {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
		g.indent--
		first = false
	}
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *perlGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.types.noteParams(fd)
	g.writeIndent()
	g.writeln("sub " + fd.Name + " {")
	g.indent++
	if len(fd.Params) > 0 {
		g.writeIndent()
		g.write("my (")
		for i, p := range fd.Params {
			if i > 0 {
				g.write(", ")
			}
			g.write("$" + p.Name)
		}
		g.writeln(") = @_;")
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

func (g *perlGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *perlGen) emitVarDecl(vd *ast.VarDecl) error {
	g.types.noteVar(vd)
	g.writeIndent()
	g.write("my $" + vd.Name)
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln(";")
	return nil
}

func (g *perlGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *perlGen) emitIf(is *ast.IfStmt) error {
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

func (g *perlGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *perlGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for (my $" + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("; $" + fs.Var + " < ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln("; $" + fs.Var + "++) {")
	case "each":
		// Arrays are array *references* in this backend — `my $nums = [2, 4, 6]`
		// — so `for my $n ($nums)` iterated a one-element list holding the
		// reference itself, and `$total + $n` numified it to its address.
		// for_each.xql.json printed 42949701560 instead of 12. @{...} is the
		// dereference that makes it a list of elements.
		g.write("for my $" + fs.Var + " (@{")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln("}) {")
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

func (g *perlGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

// perlStringOps maps each numeric comparison operator to the string operator
// Perl uses for the same question. Perl is the language where these are
// genuinely different operators, not different overloads.
var perlStringOps = map[string]string{
	"==": "eq",
	"!=": "ne",
	"<":  "lt",
	">":  "gt",
	"<=": "le",
	">=": "ge",
}

// isStringComparison reports whether a comparison has a String on either side.
func (g *perlGen) isStringComparison(be *ast.BinaryExpr) bool {
	return g.types.kindOf(be.Left) == "String" || g.types.kindOf(be.Right) == "String"
}

func (g *perlGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write("$" + node.Name)
		return nil
	case *ast.BinaryExpr:
		// Perl's `%` gives the sign of the right operand: -7 % 2 is 1, where C
		// and Go say -1.
		if g.types.isIntRemainder(node) {
			g.needIntRem = true
			g.write("_xql_irem(")
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
		// Perl numbers are floats, so `int()` is what gives `7 / 2` the value 3.
		if g.types.isIntDivision(node) {
			g.write("int(")
			if err := g.emitExpr(node.Left); err != nil {
				return err
			}
			g.write(" / ")
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
		if op == "+" && stringValued(g.types, node) {
			op = "."
		}
		if s, ok := perlStringOps[op]; ok && g.isStringComparison(node) {
			// Perl's `==` is numeric. Comparing two strings with it numifies
			// both to 0, so `"abc" == "xyz"` is true and every string equality
			// in a generated program answered yes.
			op = s
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
		// `use constant { Red => 0 }` makes a bareword, not a hashref field.
		if _, variant, ok := enumRef(g.enums, node); ok {
			g.write(variant)
			return nil
		}
		if err := g.emitExpr(node.Object); err != nil {
			return err
		}
		g.write("->{" + node.Field + "}")
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

func (g *perlGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *perlGen) emitLambda(lam *ast.Lambda) error {
	g.write("sub { ")
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

func (g *perlGen) emitArrayLit(al *ast.ArrayLit) error {
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

func (g *perlGen) emitIndexExpr(ie *ast.IndexExpr) error {
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write("->[")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write("]")
	return nil
}

func (g *perlGen) emitStructLit(sl *ast.StructLit) error {
	g.write("{ ")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + " => ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(" }")
	return nil
}

func (g *perlGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("print(")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(" . ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(` . "\n")`)
		return nil
	case "printf":
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
		return nil
	case "sprintf":
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

func (g *perlGen) emitLiteral(lit *ast.Literal) error {
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
			g.write("1")
		} else {
			g.write("0")
		}
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}
