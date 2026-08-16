package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

func GenerateOCaml(root ast.Node) ([]byte, error) {
	g := &ocamlGen{
		types:    newTypeKinds(root),
		buf:      &strings.Builder{},
		funcRets: make(map[string]string),
		varTypes: make(map[string]string),
	}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	for _, d := range prog.Decls {
		if fd, ok := d.(*ast.FunctionDecl); ok {
			g.funcRets[fd.Name] = fd.ReturnType.KindName
		}
	}

	var structs []*ast.StructDecl
	var enums []*ast.EnumDecl
	var funcs []*ast.FunctionDecl
	for _, d := range prog.Decls {
		switch n := d.(type) {
		case *ast.StructDecl:
			structs = append(structs, n)
		case *ast.EnumDecl:
			enums = append(enums, n)
		case *ast.FunctionDecl:
			funcs = append(funcs, n)
		}
	}

	for _, ed := range enums {
		g.emitEnumDecl(ed)
		g.writeln("")
	}

	for _, sd := range structs {
		g.emitStructDecl(sd)
		g.writeln("")
	}

	for i, fd := range funcs {
		if i > 0 {
			g.writeln("")
		}
		if err := g.emitFunctionDecl(fd); err != nil {
			return nil, err
		}
	}

	return []byte(g.buf.String()), nil
}

type ocamlGen struct {
	types    *typeKinds
	buf      *strings.Builder
	indent   int
	funcRets map[string]string
	muts     map[string]bool

	// varTypes remembers the declared type of locals and parameters. Without
	// it the println emitter has to guess which of_string conversion to wrap
	// an identifier in, and its guess was string_of_int.
	varTypes map[string]string

	// valuePosition is set while emitting the final statement of a function
	// that returns something. There the statement is the function's value, not
	// one more step in a sequence, so it must not be followed by ";".
	valuePosition bool
}

// stmtEnd is the separator after a statement: absent in value position,
// where a trailing ";" would promise another expression.
func (g *ocamlGen) stmtEnd() string {
	if g.valuePosition {
		return ""
	}
	return ";"
}

func (g *ocamlGen) write(s string)   { g.buf.WriteString(s) }
func (g *ocamlGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *ocamlGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func typeToOCaml(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "int"
	case "Float":
		return "float"
	case "String":
		return "string"
	case "Bool":
		return "bool"
	case "Void":
		return "unit"
	case "Array":
		if t.Elem != nil {
			return typeToOCaml(*t.Elem) + " list"
		}
		return "int list"
	case "Option":
		if t.Elem != nil {
			return typeToOCaml(*t.Elem) + " option"
		}
		return "int option"
	default:
		return strings.ToLower(t.KindName)
	}
}

func (g *ocamlGen) emitStructDecl(sd *ast.StructDecl) {
	name := strings.ToLower(sd.Name[:1]) + sd.Name[1:]
	g.write("type " + name + " = {")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(";")
		}
		g.write(" " + f.Name + ": " + typeToOCaml(f.Type))
	}
	g.writeln(" }")
}

func (g *ocamlGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeln("type " + strings.ToLower(ed.Name[:1]) + ed.Name[1:] + " = " + strings.Join(ed.Variants, " | "))
	return nil
}

func (g *ocamlGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.types.noteParams(fd)
	g.muts = collectMutables(fd.Body)
	for _, p := range fd.Params {
		if p.Type.KindName != "" {
			g.varTypes[p.Name] = p.Type.KindName
		}
	}

	if fd.Name == "main" {
		// Emit main body, then call at top level
		g.writeln("let rec main () =")
		g.indent++
		for _, stmt := range fd.Body {
			if err := g.emitStmt(stmt); err != nil {
				return err
			}
		}
		g.closeBody(fd.Body, false)
		g.indent--
		g.writeln("")
		g.writeln("let () = main ()")
		return nil
	}

	g.writeIndent()
	// OCaml scopes a plain `let f` so that f is not visible inside its own
	// body, and a self-call fails with "Unbound value f" plus a hint to add
	// rec. It is legal on a function that never recurses, so declare it rather
	// than analyse the call graph — the same call made for Fortran RECURSIVE.
	g.write("let rec " + fd.Name)
	for _, p := range fd.Params {
		g.write(" (" + p.Name + ": " + typeToOCaml(p.Type) + ")")
	}
	if len(fd.Params) == 0 {
		g.write(" ()")
	}
	rt := typeToOCaml(fd.ReturnType)
	g.writeln(" : " + rt + " =")

	returnsValue := rt != "unit"
	g.indent++
	for i, stmt := range fd.Body {
		g.valuePosition = returnsValue && i == len(fd.Body)-1
		if err := g.emitStmt(stmt); err != nil {
			return err
		}
	}
	g.valuePosition = false
	g.closeBody(fd.Body, returnsValue)
	g.indent--
	return nil
}

// closeBody finishes a function body so it is a complete OCaml expression.
//
// Every statement this backend emits ends in ";" except a return, and ";" in
// OCaml separates expressions rather than terminating them. A body ending in
// `print_endline x;` therefore promises another expression and never delivers
// one, so the next top-level `let` is a syntax error — which is why no OCaml
// this backend produced had ever compiled. Closing the sequence with unit
// gives `print_endline x; ()`, which is both valid and the right type for a
// procedure.
func (g *ocamlGen) closeBody(body []ast.Node, returnsValue bool) {
	if returnsValue {
		// The last statement is the function's value. Nothing to add, and the
		// separator after it was already suppressed.
		return
	}
	if len(body) > 0 {
		if _, isReturn := body[len(body)-1].(*ast.ReturnStmt); isReturn {
			return
		}
	}
	g.writeIndent()
	g.writeln("()")
}

func (g *ocamlGen) emitStmt(n ast.Node) error {
	switch node := n.(type) {
	case *ast.ReturnStmt:
		g.writeIndent()
		if node.Value == nil {
			g.writeln("()")
		} else {
			if err := g.emitExpr(node.Value); err != nil {
				return err
			}
			g.writeln("")
		}
		return nil
	case *ast.VarDecl:
		return g.emitVarDecl(node)
	case *ast.AssignStmt:
		return g.emitAssignStmt(node)
	case *ast.ExprStmt:
		g.writeIndent()
		if err := g.emitExpr(node.Expr); err != nil {
			return err
		}
		g.writeln(g.stmtEnd())
		return nil
	case *ast.IfStmt:
		return g.emitIfStmt(node)
	case *ast.WhileStmt:
		return g.emitWhileStmt(node)
	case *ast.ForStmt:
		return g.emitForStmt(node)
	case *ast.BreakStmt:
		return fmt.Errorf("XQL_E402: OCaml does not support break")
	case *ast.ContinueStmt:
		return fmt.Errorf("XQL_E402: OCaml does not support continue")
	case *ast.MatchExpr:
		return g.emitMatchStmt(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *ocamlGen) emitVarDecl(vd *ast.VarDecl) error {
	g.types.noteVar(vd)
	if vd.Type.KindName != "" {
		g.varTypes[vd.Name] = vd.Type.KindName
	}
	g.writeIndent()
	if g.muts[vd.Name] {
		g.write("let " + vd.Name + " = ref ")
		if vd.Value != nil {
			if err := g.emitExpr(vd.Value); err != nil {
				return err
			}
		} else {
			g.write("0")
		}
	} else {
		g.write("let " + vd.Name + " = ")
		if vd.Value != nil {
			if err := g.emitExpr(vd.Value); err != nil {
				return err
			}
		} else {
			g.write("()")
		}
	}
	g.writeln(" in")
	return nil
}

func (g *ocamlGen) emitAssignStmt(as *ast.AssignStmt) error {
	g.writeIndent()
	if ident, ok := as.Target.(*ast.Ident); ok {
		g.write(ident.Name + " := ")
	} else {
		if err := g.emitExpr(as.Target); err != nil {
			return err
		}
		g.write(" <- ")
	}
	if err := g.emitExpr(as.Value); err != nil {
		return err
	}
	g.writeln(g.stmtEnd())
	return nil
}

func (g *ocamlGen) emitIfStmt(is *ast.IfStmt) error {
	outer := g.valuePosition
	g.valuePosition = false
	defer func() { g.valuePosition = outer }()
	g.writeIndent()
	g.write("if ")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(" then begin")
	g.indent++
	for _, s := range is.Then {
		if err := g.emitStmt(s); err != nil {
			return err
		}
	}
	if len(is.Then) == 0 {
		g.writeIndent()
		g.writeln("()")
	}
	g.indent--
	if len(is.Else) > 0 {
		g.writeIndent()
		g.writeln("end else begin")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitStmt(s); err != nil {
				return err
			}
		}
		g.indent--
	}
	g.writeIndent()
	g.valuePosition = outer
	g.writeln("end" + g.stmtEnd())
	return nil
}

func (g *ocamlGen) emitWhileStmt(ws *ast.WhileStmt) error {
	if loopBodyReturns(ws.Body) {
		return fmt.Errorf("XQL_E402: an OCaml while body must evaluate to unit and the loop has no early exit, so a return inside one cannot be expressed")
	}
	outer := g.valuePosition
	g.valuePosition = false
	defer func() { g.valuePosition = outer }()
	g.writeIndent()
	g.write("while ")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(" do")
	g.indent++
	for _, s := range ws.Body {
		if err := g.emitStmt(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.valuePosition = outer
	g.writeln("done" + g.stmtEnd())
	return nil
}

func (g *ocamlGen) emitForStmt(fs *ast.ForStmt) error {
	if loopBodyReturns(fs.Body) {
		return fmt.Errorf("XQL_E402: an OCaml for body must evaluate to unit and the loop has no early exit, so a return inside one cannot be expressed")
	}
	outer := g.valuePosition
	g.valuePosition = false
	defer func() { g.valuePosition = outer }()
	if fs.Form == "range" {
		g.writeIndent()
		g.write("for " + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write(" to ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(" - 1 do")
		g.indent++
		for _, s := range fs.Body {
			if err := g.emitStmt(s); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln("done" + g.stmtEnd())
		return nil
	}
	// each form
	g.writeIndent()
	g.write("List.iter (fun " + fs.Var + " ->")
	g.writeln("")
	g.indent++
	for _, s := range fs.Body {
		if err := g.emitStmt(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.write(") ")
	if err := g.emitExpr(fs.Iterable); err != nil {
		return err
	}
	g.valuePosition = outer
	g.writeln(g.stmtEnd())
	return nil
}

// Each arm's body goes inside begin/end, which is what emitIfStmt does with a
// then-branch and for the same reason: every statement this backend emits ends
// in `;`, and `;` is OCaml's sequence operator, so it needs something after it.
// A trailing one is allowed before `end` or `)` — which is why the last arm
// happened to parse — but not before the `|` of the next arm, so a match with
// two arms was a syntax error and a match with one was not. Nothing in the
// corpus had a match at all, so neither had ever been compiled.
func (g *ocamlGen) emitMatchStmt(me *ast.MatchExpr) error {
	outer := g.valuePosition
	g.valuePosition = false
	defer func() { g.valuePosition = outer }()

	g.writeIndent()
	g.write("(match ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(" with")
	for _, arm := range me.Arms {
		g.writeIndent()
		g.write("| ")
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.write("_")
		} else {
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
		}
		g.writeln(" -> begin")
		g.indent++
		for _, s := range arm.Body {
			if err := g.emitStmt(s); err != nil {
				return err
			}
		}
		if len(arm.Body) == 0 {
			g.writeIndent()
			g.writeln("()")
		}
		g.indent--
		g.writeIndent()
		g.writeln("end")
	}
	g.writeIndent()
	g.valuePosition = outer
	g.writeln(")" + g.stmtEnd())
	return nil
}

func (g *ocamlGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		if g.muts[node.Name] {
			g.write("!" + node.Name)
		} else {
			g.write(node.Name)
		}
		return nil
	case *ast.BinaryExpr:
		return g.emitBinary(node)
	case *ast.UnaryExpr:
		return g.emitUnary(node)
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
	case *ast.MatchExpr:
		return g.emitMatchExpr(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported expression %s", n.Kind())
	}
}

func (g *ocamlGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write(fmt.Sprintf("%q", s))
	case "Int":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%d", int64(f)))
	case "Float":
		f, _ := lit.Value.(float64)
		s := fmt.Sprintf("%g", f)
		if !strings.Contains(s, ".") && !strings.Contains(s, "e") && !strings.Contains(s, "E") {
			s += ".0"
		}
		g.write(s)
	case "Bool":
		b, _ := lit.Value.(bool)
		if b {
			g.write("true")
		} else {
			g.write("false")
		}
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}

func (g *ocamlGen) emitBinary(be *ast.BinaryExpr) error {
	if be.Op == "+" && stringValued(g.types, be) {
		g.write("(")
		if err := g.emitExpr(be.Left); err != nil {
			return err
		}
		g.write(" ^ ")
		if err := g.emitExpr(be.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	}
	op := be.Op
	switch op {
	case "==":
		op = "="
	case "!=":
		op = "<>"
	case "%":
		op = "mod"
	}
	g.write("(")
	if err := g.emitExpr(be.Left); err != nil {
		return err
	}
	g.write(" " + op + " ")
	if err := g.emitExpr(be.Right); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *ocamlGen) emitUnary(ue *ast.UnaryExpr) error {
	switch ue.Op {
	case "!":
		g.write("(not ")
		if err := g.emitExpr(ue.Operand); err != nil {
			return err
		}
		g.write(")")
	case "-":
		g.write("(-")
		if err := g.emitExpr(ue.Operand); err != nil {
			return err
		}
		g.write(")")
	default:
		g.write(ue.Op)
		if err := g.emitExpr(ue.Operand); err != nil {
			return err
		}
	}
	return nil
}

func (g *ocamlGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		if len(ce.Args) == 0 {
			g.write(`print_endline ""`)
			return nil
		}
		isStr := g.isStringExpr(ce.Args[0])
		if isStr {
			g.write("print_endline ")
		} else if g.isBoolExpr(ce.Args[0]) {
			g.write("print_endline (string_of_bool ")
		} else if g.isFloatExpr(ce.Args[0]) {
			g.write("print_endline (string_of_float ")
		} else {
			g.write("print_endline (string_of_int ")
		}
		needParen := g.needParens(ce.Args[0])
		if needParen {
			g.write("(")
		}
		if err := g.emitExpr(ce.Args[0]); err != nil {
			return err
		}
		if needParen {
			g.write(")")
		}
		if !isStr {
			g.write(")")
		}
		return nil
	case "printf":
		if len(ce.Args) >= 2 {
			g.write("Printf.printf ")
			for _, arg := range ce.Args {
				g.write(" ")
				needParen := g.needParens(arg)
				if needParen {
					g.write("(")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
				if needParen {
					g.write(")")
				}
			}
		} else if len(ce.Args) > 0 {
			g.write("Printf.printf \"%s\" ")
			if g.needParens(ce.Args[0]) {
				g.write("(")
			}
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			if g.needParens(ce.Args[0]) {
				g.write(")")
			}
		}
		return nil
	case "sprintf":
		if len(ce.Args) >= 2 {
			g.write("Printf.sprintf ")
			for _, arg := range ce.Args {
				g.write(" ")
				needParen := g.needParens(arg)
				if needParen {
					g.write("(")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
				if needParen {
					g.write(")")
				}
			}
		} else if len(ce.Args) > 0 {
			g.write("Printf.sprintf \"%s\" ")
			if g.needParens(ce.Args[0]) {
				g.write("(")
			}
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			if g.needParens(ce.Args[0]) {
				g.write(")")
			}
		} else {
			g.write(`""`)
		}
		return nil
	default:
		g.write(ce.Callee)
		for _, arg := range ce.Args {
			g.write(" ")
			needParen := g.needParens(arg)
			if needParen {
				g.write("(")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
			if needParen {
				g.write(")")
			}
		}
		return nil
	}
}

func (g *ocamlGen) isBoolExpr(n ast.Node) bool {
	if g.declaredType(n) == "Bool" {
		return true
	}
	switch node := n.(type) {
	case *ast.Literal:
		return node.ValueType == "Bool"
	case *ast.BinaryExpr:
		switch node.Op {
		case "==", "!=", "<", ">", "<=", ">=", "&&", "||":
			return true
		}
	case *ast.UnaryExpr:
		return node.Op == "!"
	}
	return false
}

func (g *ocamlGen) isFloatExpr(n ast.Node) bool {
	if g.declaredType(n) == "Float" {
		return true
	}
	switch node := n.(type) {
	case *ast.Literal:
		return node.ValueType == "Float"
	case *ast.CallExpr:
		if ret, ok := g.funcRets[node.Callee]; ok {
			return ret == "Float"
		}
	}
	return false
}

func (g *ocamlGen) isStringExpr(n ast.Node) bool {
	if g.declaredType(n) == "String" {
		return true
	}
	if stringValued(g.types, n) {
		return true
	}
	if ce, ok := n.(*ast.CallExpr); ok {
		if ret, found := g.funcRets[ce.Callee]; found {
			return ret == "String"
		}
	}
	return false
}

// declaredType reports the recorded type of an identifier, or "" when the node
// is not one or was never declared with a type.
func (g *ocamlGen) declaredType(n ast.Node) string {
	id, ok := n.(*ast.Ident)
	if !ok {
		return ""
	}
	return g.varTypes[id.Name]
}

func (g *ocamlGen) needParens(n ast.Node) bool {
	switch n.(type) {
	case *ast.CallExpr, *ast.BinaryExpr, *ast.UnaryExpr, *ast.IfExpr:
		return true
	default:
		return false
	}
}

func (g *ocamlGen) emitStructLit(sl *ast.StructLit) error {
	g.write("{ ")
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

func (g *ocamlGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("[")
	for i, elem := range al.Elements {
		if i > 0 {
			g.write("; ")
		}
		if err := g.emitExpr(elem); err != nil {
			return err
		}
	}
	g.write("]")
	return nil
}

func (g *ocamlGen) emitIndexExpr(ie *ast.IndexExpr) error {
	g.write("(List.nth ")
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write(" ")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *ocamlGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("(if ")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" then ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" else ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *ocamlGen) emitLambda(lam *ast.Lambda) error {
	g.write("(fun")
	for _, p := range lam.Params {
		g.write(" " + p.Name)
	}
	g.write(" -> ")
	if len(lam.Body) == 1 {
		if rs, ok := lam.Body[0].(*ast.ReturnStmt); ok && rs.Value != nil {
			if err := g.emitExpr(rs.Value); err != nil {
				return err
			}
		} else if es, ok := lam.Body[0].(*ast.ExprStmt); ok {
			if err := g.emitExpr(es.Expr); err != nil {
				return err
			}
		} else {
			if err := g.emitExpr(lam.Body[0]); err != nil {
				return err
			}
		}
	} else {
		g.writeln("")
		g.indent++
		for _, stmt := range lam.Body {
			if err := g.emitStmt(stmt); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
	}
	g.write(")")
	return nil
}

func (g *ocamlGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.write("(match ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.write(" with")
	for _, arm := range me.Arms {
		g.write(" | ")
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.write("_")
		} else {
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
		}
		g.write(" -> ")
		if len(arm.Body) == 1 {
			if es, ok := arm.Body[0].(*ast.ExprStmt); ok {
				if err := g.emitExpr(es.Expr); err != nil {
					return err
				}
				continue
			}
			if rs, ok := arm.Body[0].(*ast.ReturnStmt); ok && rs.Value != nil {
				if err := g.emitExpr(rs.Value); err != nil {
					return err
				}
				continue
			}
		}
		g.writeln("begin")
		g.indent++
		for _, s := range arm.Body {
			if err := g.emitStmt(s); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.write("end")
	}
	g.write(")")
	return nil
}
