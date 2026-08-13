package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateBash produces Bash shell script source code from the given typed AST.
// The "main" function's body is emitted at top level after function definitions.
func GenerateBash(root ast.Node) ([]byte, error) {
	g := &bashGen{
		buf:         &strings.Builder{},
		varTypes:    make(map[string]string),
		funcRets:    make(map[string]string),
		funcParams:  make(map[string][]string),
		structNames: make(map[string]bool),
		refNames:    make(map[string]string),
	}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Return types are needed before any body is walked: a call can appear in
	// an expression whose function is declared further down the file.
	for _, d := range prog.Decls {
		switch decl := d.(type) {
		case *ast.FunctionDecl:
			g.funcRets[decl.Name] = decl.ReturnType.KindName
			kinds := make([]string, len(decl.Params))
			for i, p := range decl.Params {
				kinds[i] = p.Type.KindName
			}
			g.funcParams[decl.Name] = kinds
		case *ast.StructDecl:
			g.structNames[decl.Name] = true
		}
	}

	g.writeln("#!/bin/bash")
	g.writeln("")

	// Emit enum declarations as readonly integer constants.
	for _, d := range prog.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			if err := g.emitEnumDecl(ed); err != nil {
				return nil, err
			}
			g.writeln("")
		}
	}

	// Emit struct declarations (associative array helpers are created at instantiation).
	for _, d := range prog.Decls {
		if sd, ok := d.(*ast.StructDecl); ok {
			if err := g.emitStructDecl(sd); err != nil {
				return nil, err
			}
			g.writeln("")
		}
	}

	// Emit non-main function declarations.
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

	return []byte(g.buf.String()), nil
}

type bashGen struct {
	buf    *strings.Builder
	indent int
	inFunc bool

	// Bash has one data type, the string, and `$(( ))` reads whatever is inside
	// it as arithmetic. Telling a string apart from a number is therefore not a
	// nicety here — `"Hello, " + name` compiled into an arithmetic expansion
	// evaluates to 0. These two maps are the only type information the backend
	// needs to route `+` to concatenation instead.
	varTypes map[string]string
	funcRets map[string]string

	// Struct-typed parameters need a name rather than a value at the call site,
	// so both ends have to agree on which parameters those are.
	funcParams  map[string][]string
	structNames map[string]bool

	// refNames maps a struct parameter to the local nameref standing in for it.
	// `local -n p="$1"` with the caller's variable also called p is a name
	// bound to itself, which bash reports as a circular reference on every
	// use; the local has to be called something else, and every mention of the
	// parameter in the body has to follow.
	refNames map[string]string
}

// varName is the shell name for an XQL variable, which differs from its own
// name only for a parameter reached through a nameref.
func (g *bashGen) varName(name string) string {
	if ref, ok := g.refNames[name]; ok {
		return ref
	}
	return name
}

// inferTypeKind reports the AST type kind of an expression, defaulting to Int
// because arithmetic is the safe assumption for everything bash does with $(( )).
func (g *bashGen) inferTypeKind(n ast.Node) string {
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
func (g *bashGen) isStringConcat(be *ast.BinaryExpr) bool {
	return be.Op == "+" &&
		(g.inferTypeKind(be.Left) == "String" || g.inferTypeKind(be.Right) == "String")
}

func (g *bashGen) write(s string)   { g.buf.WriteString(s) }
func (g *bashGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *bashGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func (g *bashGen) emitNode(n ast.Node) error {
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

func (g *bashGen) emitStructDecl(sd *ast.StructDecl) error {
	// Bash has no structs; emit a comment describing the fields.
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

func (g *bashGen) emitEnumDecl(ed *ast.EnumDecl) error {
	for i, v := range ed.Variants {
		g.writeIndent()
		g.writeln(fmt.Sprintf("readonly %s=%d", v, i))
	}
	return nil
}

func (g *bashGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("case ")
	if err := g.emitExprUnquoted(me.Value); err != nil {
		return err
	}
	g.writeln(" in")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		// Check for wildcard pattern.
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.writeln("*)")
		} else {
			if err := g.emitExprUnquoted(arm.Pattern); err != nil {
				return err
			}
			g.writeln(")")
		}
		g.indent++
		for _, stmt := range arm.Body {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
		g.writeIndent()
		g.writeln(";;")
		g.indent--
	}
	g.indent--
	g.writeIndent()
	g.writeln("esac")
	return nil
}

func (g *bashGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.writeIndent()
	g.writeln(fd.Name + "() {")
	g.indent++
	prevInFunc := g.inFunc
	g.inFunc = true
	// Assign parameters from positional args.
	for i, p := range fd.Params {
		g.varTypes[p.Name] = p.Type.KindName
		g.writeIndent()
		// A struct is an associative array, and bash cannot pass one by value:
		// `manhattan "${p}"` expands to the element keyed 0, which does not
		// exist, so the body computed `( + )` and the shell reported an
		// arithmetic syntax error. A nameref binds the local to the caller's
		// array, which is the one way bash has of handing an array to a
		// function. The call site passes the bare name to match.
		if g.structNames[p.Type.KindName] {
			ref := "xql_ref_" + p.Name
			g.refNames[p.Name] = ref
			g.writeln(fmt.Sprintf("local -n %s=\"$%d\"", ref, i+1))
			continue
		}
		g.writeln(fmt.Sprintf("local %s=\"$%d\"", p.Name, i+1))
	}
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	for _, p := range fd.Params {
		delete(g.refNames, p.Name)
	}
	g.inFunc = prevInFunc
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

// emitReturn hands the value back on stdout, because a shell function's exit
// status is one byte and every call site here reads `$(...)`. The echo is not
// the whole statement, though, and for one release it was: without a `return`
// after it, control fell through to whatever followed. A return inside a loop
// echoed its value and then kept looping, so early_return.xql.json's
// firstOver(20) printed "5 6 7 8 9 0" where every other target printed 5, and
// the caller captured all of it as one string.
//
// main is emitted at top level rather than as a function (see GenerateBash),
// and `return` outside a function is an error in a script bash was told to
// run, so there the statement has to be `exit`.
func (g *bashGen) emitReturn(rs *ast.ReturnStmt) error {
	if rs.Value != nil {
		g.writeIndent()
		g.write("echo ")
		if err := g.emitExpr(rs.Value); err != nil {
			return err
		}
		g.writeln("")
	}
	g.writeIndent()
	if g.inFunc {
		g.writeln("return")
	} else {
		g.writeln("exit 0")
	}
	return nil
}

func (g *bashGen) emitVarDecl(vd *ast.VarDecl) error {
	g.varTypes[vd.Name] = vd.Type.KindName
	g.writeIndent()

	// A struct becomes an associative array, and bash only treats `([k]=v)` as
	// one if the name was declared associative first. Without the -A the
	// subscripts are evaluated as arithmetic — every field name resolves to 0,
	// so `([x]=3 [y]=5)` collapses into a single element and both fields read
	// back as 5.
	assoc := false
	if _, ok := vd.Value.(*ast.StructLit); ok {
		assoc = true
	}
	switch {
	case g.inFunc && assoc:
		g.write("local -A ")
	case g.inFunc:
		g.write("local ")
	case assoc:
		g.write("declare -A ")
	}
	g.write(vd.Name + "=")
	if vd.Value != nil {
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	} else {
		g.write("\"\"")
	}
	g.writeln("")
	return nil
}

func (g *bashGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	if err := g.emitExprUnquoted(as.Target); err != nil {
		return err
	}
	g.write("=")
	if err := g.emitExpr(as.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *bashGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if [[ ")
	if err := g.emitCondExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(" ]]; then")
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
	g.writeln("fi")
	return nil
}

func (g *bashGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while [[ ")
	if err := g.emitCondExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(" ]]; do")
	g.indent++
	for _, s := range ws.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("done")
	return nil
}

func (g *bashGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for ((" + fs.Var + "=")
		if err := g.emitExprUnquoted(fs.Start); err != nil {
			return err
		}
		g.write("; " + fs.Var + "<")
		if err := g.emitExprUnquoted(fs.End); err != nil {
			return err
		}
		g.writeln("; " + fs.Var + "++)); do")
	case "each":
		// The comment here used to say "${arr[@]} style" while the code emitted
		// the bare name, so `for n in nums` iterated the single word "nums" and
		// arithmetic on it evaluated an unset variable to 0. for_each.xql.json
		// summed to 2 rather than 12 — the loop ran once, on nothing.
		//
		// Only an identifier can be expanded this way: it names a bash array.
		// Anything else has no array to expand and is left as it was.
		if id, ok := fs.Iterable.(*ast.Ident); ok {
			g.write("for " + fs.Var + ` in "${` + id.Name + `[@]}"`)
		} else {
			g.write("for " + fs.Var + " in ")
			if err := g.emitExprUnquoted(fs.Iterable); err != nil {
				return err
			}
		}
		g.writeln("; do")
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
	g.writeln("done")
	return nil
}

func (g *bashGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

// emitExprUnquoted writes an expression without surrounding quotes (for assignment targets, arithmetic contexts).
func (g *bashGen) emitExprUnquoted(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Ident:
		g.write(g.varName(node.Name))
		return nil
	case *ast.Literal:
		return g.emitLiteralRaw(node)
	case *ast.MemberExpr:
		if err := g.emitExprUnquoted(node.Object); err != nil {
			return err
		}
		g.write("[" + node.Field + "]")
		return nil
	case *ast.IndexExpr:
		if err := g.emitExprUnquoted(node.Target); err != nil {
			return err
		}
		g.write("[")
		if err := g.emitExprUnquoted(node.Index); err != nil {
			return err
		}
		g.write("]")
		return nil
	default:
		return g.emitExpr(n)
	}
}

// emitCondExpr writes a condition expression suitable for [[ ]] context.
func (g *bashGen) emitCondExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.BinaryExpr:
		op := node.Op
		switch op {
		case "==":
			if err := g.emitExpr(node.Left); err != nil {
				return err
			}
			g.write(" == ")
			return g.emitExpr(node.Right)
		case "!=":
			if err := g.emitExpr(node.Left); err != nil {
				return err
			}
			g.write(" != ")
			return g.emitExpr(node.Right)
		case "<":
			if err := g.emitExpr(node.Left); err != nil {
				return err
			}
			g.write(" -lt ")
			return g.emitExpr(node.Right)
		case ">":
			if err := g.emitExpr(node.Left); err != nil {
				return err
			}
			g.write(" -gt ")
			return g.emitExpr(node.Right)
		case "<=":
			if err := g.emitExpr(node.Left); err != nil {
				return err
			}
			g.write(" -le ")
			return g.emitExpr(node.Right)
		case ">=":
			if err := g.emitExpr(node.Left); err != nil {
				return err
			}
			g.write(" -ge ")
			return g.emitExpr(node.Right)
		case "&&":
			if err := g.emitCondExpr(node.Left); err != nil {
				return err
			}
			g.write(" && ")
			return g.emitCondExpr(node.Right)
		case "||":
			if err := g.emitCondExpr(node.Left); err != nil {
				return err
			}
			g.write(" || ")
			return g.emitCondExpr(node.Right)
		default:
			return g.emitCondValue(n)
		}
	case *ast.UnaryExpr:
		if node.Op == "!" {
			g.write("! ")
			return g.emitCondExpr(node.Operand)
		}
		return g.emitCondValue(n)
	default:
		return g.emitCondValue(n)
	}
}

// emitCondValue writes an expression that is being used as a truth value in its
// own right rather than as an operand of a comparison.
//
// Bash has one data type, and `[[ "$b" ]]` asks whether the string is
// non-empty. A Bool is 1 or 0 here, and "0" is a perfectly non-empty string, so
// every false Bool tested true: bool_logic.xql.json wrote `[[ "${a}" && !
// "${b}" ]]` with b false and printed and-bad, because `!` negated a test that
// had already answered yes. Comparing against 1 asks the question that was
// meant.
func (g *bashGen) emitCondValue(n ast.Node) error {
	if err := g.emitExpr(n); err != nil {
		return err
	}
	if g.inferTypeKind(n) == "Bool" {
		g.write(" == 1")
	}
	return nil
}

func (g *bashGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write("\"${" + g.varName(node.Name) + "}\"")
		return nil
	case *ast.BinaryExpr:
		if g.isStringConcat(node) {
			return g.emitStringConcat(node)
		}
		g.write("$(( ")
		if err := g.emitArithExpr(node); err != nil {
			return err
		}
		g.write(" ))")
		return nil
	case *ast.UnaryExpr:
		if node.Op == "!" {
			g.write("$(( ! ")
			if err := g.emitArithExpr(node.Operand); err != nil {
				return err
			}
			g.write(" ))")
			return nil
		}
		g.write(node.Op)
		return g.emitExpr(node.Operand)
	case *ast.CallExpr:
		return g.emitCall(node)
	case *ast.MemberExpr:
		// Associative array access: ${obj[field]}
		g.write("\"${")
		if ident, ok := node.Object.(*ast.Ident); ok {
			g.write(g.varName(ident.Name))
		} else {
			return fmt.Errorf("XQL_E401: Bash member access requires simple identifier as object")
		}
		g.write("[" + node.Field + "]}\"")
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

func (g *bashGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("$(if [[ ")
	if err := g.emitCondExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" ]]; then echo ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write("; else echo ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write("; fi)")
	return nil
}

func (g *bashGen) emitLambda(lam *ast.Lambda) error {
	return fmt.Errorf("XQL_E401: Bash does not support Lambda expressions")
}

// emitStringConcat writes a string-valued `+` as bash concatenation: one pair
// of double quotes with every operand expanded inside it.
func (g *bashGen) emitStringConcat(be *ast.BinaryExpr) error {
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

// emitInterpolated writes an expression as it must appear *inside* double
// quotes. That rules out the quoted forms emitExpr produces — a `"` there would
// close the string being built — so every case here expands to bare text, a
// parameter expansion, or a command substitution.
func (g *bashGen) emitInterpolated(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		if node.ValueType == "String" {
			s, _ := node.Value.(string)
			g.write(bashEscapeInDoubleQuotes(s))
			return nil
		}
		return g.emitLiteralRaw(node)
	case *ast.Ident:
		g.write("${" + g.varName(node.Name) + "}")
		return nil
	case *ast.MemberExpr:
		ident, ok := node.Object.(*ast.Ident)
		if !ok {
			return fmt.Errorf("XQL_E401: Bash member access requires simple identifier as object")
		}
		g.write("${" + g.varName(ident.Name) + "[" + node.Field + "]}")
		return nil
	case *ast.IndexExpr:
		ident, ok := node.Target.(*ast.Ident)
		if !ok {
			return fmt.Errorf("XQL_E401: Bash index access requires simple identifier as target")
		}
		g.write("${" + g.varName(ident.Name) + "[")
		if err := g.emitExprUnquoted(node.Index); err != nil {
			return err
		}
		g.write("]}")
		return nil
	case *ast.BinaryExpr:
		if g.isStringConcat(node) {
			if err := g.emitInterpolated(node.Left); err != nil {
				return err
			}
			return g.emitInterpolated(node.Right)
		}
		g.write("$(( ")
		if err := g.emitArithExpr(node); err != nil {
			return err
		}
		g.write(" ))")
		return nil
	case *ast.CallExpr:
		// A command substitution starts a fresh quoting context, so the quotes
		// emitExpr puts around the arguments are safe in here.
		callee := node.Callee
		if callee == "sprintf" {
			callee = "printf"
		}
		g.write("$(" + callee)
		for _, arg := range node.Args {
			g.write(" ")
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	default:
		return fmt.Errorf("XQL_E401: unsupported expression %s in string concatenation", n.Kind())
	}
}

// bashEscapeInDoubleQuotes escapes the four characters bash still interprets
// between double quotes.
func bashEscapeInDoubleQuotes(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"$", `\$`,
		"`", "\\`",
	)
	return r.Replace(s)
}

// emitArithExpr writes an expression inside $(( )) arithmetic context.
func (g *bashGen) emitArithExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteralRaw(node)
	case *ast.Ident:
		g.write(g.varName(node.Name))
		return nil
	case *ast.BinaryExpr:
		g.write("(")
		if err := g.emitArithExpr(node.Left); err != nil {
			return err
		}
		op := node.Op
		if op == "&&" {
			op = "&&"
		} else if op == "||" {
			op = "||"
		}
		g.write(" " + op + " ")
		if err := g.emitArithExpr(node.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	case *ast.UnaryExpr:
		g.write(node.Op)
		return g.emitArithExpr(node.Operand)
	default:
		return g.emitExpr(n)
	}
}

func (g *bashGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("(")
	for i, elem := range al.Elements {
		if i > 0 {
			g.write(" ")
		}
		if err := g.emitExpr(elem); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *bashGen) emitIndexExpr(ie *ast.IndexExpr) error {
	g.write("\"${")
	if ident, ok := ie.Target.(*ast.Ident); ok {
		g.write(ident.Name)
	} else {
		return fmt.Errorf("XQL_E401: Bash index access requires simple identifier as target")
	}
	g.write("[")
	if err := g.emitExprUnquoted(ie.Index); err != nil {
		return err
	}
	g.write("]}\"")
	return nil
}

func (g *bashGen) emitStructLit(sl *ast.StructLit) error {
	// Emit as inline associative array declaration.
	g.write("(")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(" ")
		}
		g.write("[" + f.Name + "]=")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *bashGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("echo ")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		} else {
			g.write("\"\"")
		}
		return nil
	case "printf":
		g.write("printf ")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(" ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		return nil
	case "sprintf":
		g.write("\"$(printf ")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(" ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")\"")
		return nil
	default:
		// Call function and capture output via $().
		g.write("$(" + ce.Callee)
		params := g.funcParams[ce.Callee]
		for i, arg := range ce.Args {
			g.write(" ")
			// The nameref on the other side wants the variable's name, not its
			// contents. Only a plain identifier has one to give.
			if i < len(params) && g.structNames[params[i]] {
				id, ok := arg.(*ast.Ident)
				if !ok {
					return fmt.Errorf("XQL_E402: Bash passes a struct by name, "+
						"so the argument to %s must be a variable", ce.Callee)
				}
				g.write(id.Name)
				continue
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	}
}

func (g *bashGen) emitLiteral(lit *ast.Literal) error {
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

func (g *bashGen) emitLiteralRaw(lit *ast.Literal) error {
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
