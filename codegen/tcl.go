package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateTcl produces Tcl source code from the given typed AST.
// The "main" function's body is emitted at top level after proc definitions.
func GenerateTcl(root ast.Node) ([]byte, error) {
	g := &tclGen{
		buf:      &strings.Builder{},
		varTypes: make(map[string]string),
		funcRets: make(map[string]string),
	}
	g.enums = collectEnums(root)
	g.types = newTypeKinds(root)

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Collected before any body is walked: a call can name a proc declared
	// further down the file.
	for _, d := range prog.Decls {
		if fd, ok := d.(*ast.FunctionDecl); ok {
			g.funcRets[fd.Name] = fd.ReturnType.KindName
		}
	}

	first := true

	// Emit enum declarations as constants.
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

	// Emit struct declarations (comment only; Tcl uses dicts).
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

	// Emit non-main proc declarations.
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

	preamble := ""
	if g.needIntDiv {
		preamble = `proc _xql_idiv {a b} {
    set q [expr {$a / $b}]
    if {$q < 0 && $q * $b != $a} { incr q }
    return $q
}

proc _xql_irem {a b} {
    return [expr {$a - $b * [_xql_idiv $a $b]}]
}

`
	}
	return []byte(preamble + g.buf.String()), nil
}

type tclGen struct {
	buf    *strings.Builder
	indent int

	// `expr` is Tcl's only arithmetic context and it is strictly numeric, so a
	// `+` over strings has to be routed to concatenation instead — otherwise
	// `expr {"Hello, " + $name}` aborts with "can't use non-numeric string as
	// operand". These maps carry just enough type information to tell the two
	// apart.
	varTypes map[string]string
	funcRets map[string]string

	types *typeKinds

	// needIntDiv records that the program divides or takes a remainder of two
	// Ints, so the preamble has to define the helpers.
	needIntDiv bool
	enums      map[string]*ast.EnumDecl
}

// inferTypeKind reports the AST type kind of an expression. Int is the default
// because everything Tcl does inside `expr` is arithmetic.
func (g *tclGen) inferTypeKind(n ast.Node) string {
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
	case *ast.UnaryExpr:
		if node.Op == "!" {
			return "Bool"
		}
		return g.inferTypeKind(node.Operand)
	default:
		return ""
	}
}

// isStringConcat reports whether a `+` joins strings rather than numbers.
func (g *tclGen) isStringConcat(be *ast.BinaryExpr) bool {
	return be.Op == "+" &&
		(g.inferTypeKind(be.Left) == "String" || g.inferTypeKind(be.Right) == "String")
}

// emitStringConcat writes a string-valued `+` as a quoted Tcl word with each
// operand substituted into it.
func (g *tclGen) emitStringConcat(be *ast.BinaryExpr) error {
	g.write("\"")
	if err := g.emitInterpolated(be.Left); err != nil {
		return err
	}
	if err := g.emitInterpolated(be.Right); err != nil {
		return err
	}
	g.write("\"")
	return nil
}

// emitInterpolated writes an expression as it must appear inside a quoted word:
// bare text, a `${var}` substitution, or a `[...]` command substitution. The
// quoted forms emitExprInline produces would close the word being built.
func (g *tclGen) emitInterpolated(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		if node.ValueType == "String" {
			s, _ := node.Value.(string)
			g.write(tclEscapeInQuotes(s))
			return nil
		}
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write("${" + node.Name + "}")
		return nil
	case *ast.BinaryExpr:
		if g.isStringConcat(node) {
			if err := g.emitInterpolated(node.Left); err != nil {
				return err
			}
			return g.emitInterpolated(node.Right)
		}
		g.write("[expr {")
		if err := g.emitCondExpr(node); err != nil {
			return err
		}
		g.write("}]")
		return nil
	default:
		// Everything else already emits as a bracketed command substitution,
		// which is exactly what a quoted word wants.
		return g.emitExprInline(n)
	}
}

// tclEscapeInQuotes escapes the characters Tcl still substitutes inside a
// double-quoted word.
func tclEscapeInQuotes(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"$", `\$`,
		"[", `\[`,
		"]", `\]`,
	)
	return r.Replace(s)
}

func (g *tclGen) write(s string)   { g.buf.WriteString(s) }
func (g *tclGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *tclGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func (g *tclGen) emitNode(n ast.Node) error {
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

func (g *tclGen) emitStructDecl(sd *ast.StructDecl) error {
	// Tcl uses dicts for structs; emit a comment describing the fields.
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

func (g *tclGen) emitEnumDecl(ed *ast.EnumDecl) error {
	for i, v := range ed.Variants {
		g.writeIndent()
		g.writeln(fmt.Sprintf("set %s %d", v, i))
	}
	return nil
}

// Tcl does not substitute variables in a switch's pattern words: `switch $c {
// $Red { ... } }` compares the subject against the four characters "$Red" and
// never matches, silently, all the way to the default arm. That is harmless for
// an Int literal — which is what every other match in the corpus is written
// with — and wrong for an enum variant, which is a variable here. A match whose
// patterns are not all literals lowers to an if/elseif chain instead, the way
// perl, lua and awk lower every match.
func (g *tclGen) emitMatchExpr(me *ast.MatchExpr) error {
	for _, arm := range me.Arms {
		if _, ok := arm.Pattern.(*ast.Literal); ok {
			continue
		}
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			continue
		}
		return g.emitMatchAsIfChain(me)
	}

	g.writeIndent()
	g.write("switch $")
	if ident, ok := me.Value.(*ast.Ident); ok {
		g.write(ident.Name)
	} else {
		g.write("[")
		if err := g.emitExprInline(me.Value); err != nil {
			return err
		}
		g.write("]")
	}
	g.writeln(" {")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.writeln("default {")
		} else {
			if err := g.emitExprInline(arm.Pattern); err != nil {
				return err
			}
			g.writeln(" {")
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

// emitMatchAsIfChain is the lowering for a match whose patterns have to be
// evaluated rather than matched as words. The wildcard becomes the else, and a
// match that is nothing but a wildcard emits its body with no `if` around it.
func (g *tclGen) emitMatchAsIfChain(me *ast.MatchExpr) error {
	var wildcard []ast.Node
	haveWildcard := false
	open := false

	emitBody := func(body []ast.Node) error {
		g.indent++
		defer func() { g.indent-- }()
		for _, s := range body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		return nil
	}

	for _, arm := range me.Arms {
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			wildcard = arm.Body
			haveWildcard = true
			continue
		}
		g.writeIndent()
		if open {
			g.write("} elseif {")
		} else {
			g.write("if {")
		}
		if err := g.emitCondExpr(&ast.BinaryExpr{Op: "==", Left: me.Value, Right: arm.Pattern}); err != nil {
			return err
		}
		g.writeln("} {")
		open = true
		if err := emitBody(arm.Body); err != nil {
			return err
		}
	}

	if !open {
		return emitBody(wildcard)
	}
	if haveWildcard {
		g.writeIndent()
		g.writeln("} else {")
		if err := emitBody(wildcard); err != nil {
			return err
		}
	}
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *tclGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.writeIndent()
	g.types.noteParams(fd)
	g.write("proc " + fd.Name + " {")
	for i, p := range fd.Params {
		g.varTypes[p.Name] = p.Type.KindName
		if i > 0 {
			g.write(" ")
		}
		g.write(p.Name)
	}
	g.writeln("} {")
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

func (g *tclGen) emitReturn(rs *ast.ReturnStmt) error {
	g.writeIndent()
	if rs.Value == nil {
		g.writeln("return")
		return nil
	}
	g.write("return ")
	if err := g.emitExprInline(rs.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *tclGen) emitVarDecl(vd *ast.VarDecl) error {
	g.varTypes[vd.Name] = vd.Type.KindName
	g.types.noteVar(vd)
	g.writeIndent()
	g.write("set " + vd.Name + " ")
	if vd.Value != nil {
		if err := g.emitExprInline(vd.Value); err != nil {
			return err
		}
	} else {
		g.write("\"\"")
	}
	g.writeln("")
	return nil
}

func (g *tclGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	// Handle member access (dict set) and index access.
	switch target := as.Target.(type) {
	case *ast.MemberExpr:
		if ident, ok := target.Object.(*ast.Ident); ok {
			g.write("dict set " + ident.Name + " " + target.Field + " ")
		} else {
			g.write("set ")
			if err := g.emitExprInline(as.Target); err != nil {
				return err
			}
			g.write(" ")
		}
	case *ast.IndexExpr:
		if ident, ok := target.Target.(*ast.Ident); ok {
			g.write("lset " + ident.Name + " ")
			if err := g.emitExprInline(target.Index); err != nil {
				return err
			}
			g.write(" ")
		} else {
			g.write("set ")
			if err := g.emitExprInline(as.Target); err != nil {
				return err
			}
			g.write(" ")
		}
	case *ast.Ident:
		g.write("set " + target.Name + " ")
	default:
		g.write("set ")
		if err := g.emitExprInline(as.Target); err != nil {
			return err
		}
		g.write(" ")
	}
	if err := g.emitExprInline(as.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *tclGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if {")
	if err := g.emitCondExpr(is.Cond); err != nil {
		return err
	}
	g.writeln("} {")
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

func (g *tclGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while {")
	if err := g.emitCondExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln("} {")
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

func (g *tclGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for {set " + fs.Var + " ")
		if err := g.emitExprInline(fs.Start); err != nil {
			return err
		}
		g.write("} {$" + fs.Var + " < ")
		if err := g.emitExprInline(fs.End); err != nil {
			return err
		}
		g.writeln("} {incr " + fs.Var + "} {")
	case "each":
		g.write("foreach " + fs.Var + " ")
		if err := g.emitExprInline(fs.Iterable); err != nil {
			return err
		}
		g.writeln(" {")
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

func (g *tclGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExprInline(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

// emitCondExpr writes an expression in a Tcl condition context (inside {}).
func (g *tclGen) emitCondExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.BinaryExpr:
		// Parenthesised, because otherwise the tree is flattened into text and
		// `expr`'s own precedence decides what it meant. `(a + b) * c` and
		// `a + (b * c)` both came out as `$a + $b * $c`, and `a - (b - c)` as
		// `$a - $b - $c`, which is -5 rather than 3. It also puts the `!` below
		// on the whole comparison instead of its left operand: `!$x == 4`
		// negates $x and then compares the result to 4.
		g.write("(")
		if err := g.emitCondExpr(node.Left); err != nil {
			return err
		}
		g.write(" " + node.Op + " ")
		if err := g.emitCondExpr(node.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	case *ast.UnaryExpr:
		g.write(node.Op)
		return g.emitCondExpr(node.Operand)
	case *ast.Ident:
		g.write("$" + node.Name)
		return nil
	case *ast.Literal:
		return g.emitLiteral(node)
	default:
		return g.emitExprInline(n)
	}
}

// emitExprInline writes an expression for inline use (not wrapped in $).
func (g *tclGen) emitExprInline(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write("$" + node.Name)
		return nil
	case *ast.BinaryExpr:
		if g.isStringConcat(node) {
			return g.emitStringConcat(node)
		}
		// Tcl's `expr` floors its integer division and gives `%` the sign of
		// the divisor: -7 / 2 is -4 and -7 % 2 is 1, where C and Go say -3
		// and -1.
		if g.types.isIntDivision(node) || g.types.isIntRemainder(node) {
			g.needIntDiv = true
			if node.Op == "%" {
				g.write("[_xql_irem ")
			} else {
				g.write("[_xql_idiv ")
			}
			if err := g.emitExprInline(node.Left); err != nil {
				return err
			}
			g.write(" ")
			if err := g.emitExprInline(node.Right); err != nil {
				return err
			}
			g.write("]")
			return nil
		}
		g.write("[expr {")
		if err := g.emitCondExpr(node); err != nil {
			return err
		}
		g.write("}]")
		return nil
	case *ast.UnaryExpr:
		if node.Op == "!" {
			g.write("[expr {!")
			if err := g.emitCondExpr(node.Operand); err != nil {
				return err
			}
			g.write("}]")
			return nil
		}
		g.write("[expr {" + node.Op)
		if err := g.emitCondExpr(node.Operand); err != nil {
			return err
		}
		g.write("}]")
		return nil
	case *ast.CallExpr:
		return g.emitCall(node)
	case *ast.MemberExpr:
		// emitEnumDecl writes `set Red 0` at the top level, so a variant is read
		// like any other variable — but a proc has its own scope and does not
		// inherit globals, so it has to be named `$::Red`. Reaching for
		// `dict get $Color Red` indexes a dict nobody ever set.
		if _, variant, ok := enumRef(g.enums, node); ok {
			g.write("$::" + variant)
			return nil
		}
		if ident, ok := node.Object.(*ast.Ident); ok {
			g.write("[dict get $" + ident.Name + " " + node.Field + "]")
		} else {
			g.write("[dict get ")
			if err := g.emitExprInline(node.Object); err != nil {
				return err
			}
			g.write(" " + node.Field + "]")
		}
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

func (g *tclGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("[expr {")
	if err := g.emitCondExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" ? ")
	if err := g.emitCondExpr(ie.Then); err != nil {
		return err
	}
	g.write(" : ")
	if err := g.emitCondExpr(ie.Else); err != nil {
		return err
	}
	g.write("}]")
	return nil
}

func (g *tclGen) emitLambda(lam *ast.Lambda) error {
	return fmt.Errorf("XQL_E401: Tcl does not support Lambda expressions")
}

func (g *tclGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("[list")
	for _, elem := range al.Elements {
		g.write(" ")
		if err := g.emitExprInline(elem); err != nil {
			return err
		}
	}
	g.write("]")
	return nil
}

func (g *tclGen) emitIndexExpr(ie *ast.IndexExpr) error {
	g.write("[lindex ")
	if err := g.emitExprInline(ie.Target); err != nil {
		return err
	}
	g.write(" ")
	if err := g.emitExprInline(ie.Index); err != nil {
		return err
	}
	g.write("]")
	return nil
}

func (g *tclGen) emitStructLit(sl *ast.StructLit) error {
	g.write("[dict create")
	for _, f := range sl.Fields {
		g.write(" " + f.Name + " ")
		if err := g.emitExprInline(f.Value); err != nil {
			return err
		}
	}
	g.write("]")
	return nil
}

func (g *tclGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("puts ")
		if len(ce.Args) > 0 {
			if err := g.emitExprInline(ce.Args[0]); err != nil {
				return err
			}
		} else {
			g.write("\"\"")
		}
		return nil
	case "printf":
		if len(ce.Args) >= 2 {
			g.write("puts -nonewline [format ")
			for _, arg := range ce.Args {
				g.write(" ")
				if err := g.emitExprInline(arg); err != nil {
					return err
				}
			}
			g.write("]")
		} else {
			g.write("puts -nonewline ")
			if len(ce.Args) > 0 {
				if err := g.emitExprInline(ce.Args[0]); err != nil {
					return err
				}
			}
		}
		return nil
	case "sprintf":
		if len(ce.Args) >= 2 {
			g.write("[format ")
			for _, arg := range ce.Args {
				g.write(" ")
				if err := g.emitExprInline(arg); err != nil {
					return err
				}
			}
			g.write("]")
		} else {
			g.write("[format \"%s\" ")
			if len(ce.Args) > 0 {
				if err := g.emitExprInline(ce.Args[0]); err != nil {
					return err
				}
			}
			g.write("]")
		}
		return nil
	default:
		g.write("[" + ce.Callee)
		for _, arg := range ce.Args {
			g.write(" ")
			if err := g.emitExprInline(arg); err != nil {
				return err
			}
		}
		g.write("]")
		return nil
	}
}

func (g *tclGen) emitLiteral(lit *ast.Literal) error {
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
