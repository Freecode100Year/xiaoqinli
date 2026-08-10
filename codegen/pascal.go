package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GeneratePascal produces Pascal source code from the given typed AST.
// The "main" function's body is emitted inside a `program Main; begin ... end.` block.
func GeneratePascal(root ast.Node) ([]byte, error) {
	g := &pascalGen{buf: &strings.Builder{}}
	g.types = newTypeKinds(root)

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Emit main program block (types, functions, and main body all inside).
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		if err := g.emitMainBlock(fd, prog); err != nil {
			return nil, err
		}
	}

	return []byte(g.buf.String()), nil
}

type pascalGen struct {
	types      *typeKinds
	buf        *strings.Builder
	indent     int
	needIfThen bool
}

func (g *pascalGen) write(s string)   { g.buf.WriteString(s) }
func (g *pascalGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *pascalGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func typeToPascal(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "Integer"
	case "Float":
		return "Real"
	case "String":
		return "String"
	case "Bool":
		return "Boolean"
	case "Void":
		return ""
	case "Array":
		if t.Elem != nil {
			return "array of " + typeToPascal(*t.Elem)
		}
		return "array of Integer"
	default:
		return t.KindName
	}
}

func collectVarDecls(stmts []ast.Node) []*ast.VarDecl {
	var vars []*ast.VarDecl
	scanVarDecls(stmts, &vars)
	return vars
}

func scanVarDecls(stmts []ast.Node, vars *[]*ast.VarDecl) {
	for _, s := range stmts {
		switch n := s.(type) {
		case *ast.VarDecl:
			*vars = append(*vars, n)
		case *ast.IfStmt:
			scanVarDecls(n.Then, vars)
			scanVarDecls(n.Else, vars)
		case *ast.WhileStmt:
			scanVarDecls(n.Body, vars)
		case *ast.ForStmt:
			scanVarDecls(n.Body, vars)
		case *ast.MatchExpr:
			for _, arm := range n.Arms {
				scanVarDecls(arm.Body, vars)
			}
		}
	}
}

func collectForVars(stmts []ast.Node) []string {
	seen := map[string]bool{}
	var vars []string
	scanForVars(stmts, &vars, seen)
	return vars
}

func scanForVars(stmts []ast.Node, vars *[]string, seen map[string]bool) {
	for _, s := range stmts {
		switch n := s.(type) {
		case *ast.ForStmt:
			if !seen[n.Var] {
				seen[n.Var] = true
				*vars = append(*vars, n.Var)
			}
			scanForVars(n.Body, vars, seen)
		case *ast.IfStmt:
			scanForVars(n.Then, vars, seen)
			scanForVars(n.Else, vars, seen)
		case *ast.WhileStmt:
			scanForVars(n.Body, vars, seen)
		case *ast.MatchExpr:
			for _, arm := range n.Arms {
				scanForVars(arm.Body, vars, seen)
			}
		}
	}
}

func pascalHasIfExpr(stmts []ast.Node) bool {
	for _, s := range stmts {
		if pascalWalkIfExpr(s) {
			return true
		}
	}
	return false
}

func pascalWalkIfExpr(n ast.Node) bool {
	if n == nil {
		return false
	}
	if _, ok := n.(*ast.IfExpr); ok {
		return true
	}
	switch node := n.(type) {
	case *ast.VarDecl:
		return pascalWalkIfExpr(node.Value)
	case *ast.ExprStmt:
		return pascalWalkIfExpr(node.Expr)
	case *ast.ReturnStmt:
		return pascalWalkIfExpr(node.Value)
	case *ast.AssignStmt:
		return pascalWalkIfExpr(node.Value)
	case *ast.CallExpr:
		for _, a := range node.Args {
			if pascalWalkIfExpr(a) {
				return true
			}
		}
	case *ast.BinaryExpr:
		return pascalWalkIfExpr(node.Left) || pascalWalkIfExpr(node.Right)
	case *ast.UnaryExpr:
		return pascalWalkIfExpr(node.Operand)
	case *ast.IfStmt:
		return pascalHasIfExpr(node.Then) || pascalHasIfExpr(node.Else)
	case *ast.WhileStmt:
		return pascalHasIfExpr(node.Body)
	case *ast.ForStmt:
		return pascalHasIfExpr(node.Body)
	}
	return false
}

func (g *pascalGen) emitMainBlock(fd *ast.FunctionDecl, prog *ast.Program) error {
	for _, d := range prog.Decls {
		if fn, ok := d.(*ast.FunctionDecl); ok {
			if pascalHasIfExpr(fn.Body) {
				g.needIfThen = true
				break
			}
		}
	}

	g.writeln("program Main;")
	// The backend assigns to `Result` inside functions, which only exists in
	// Object Pascal / Delphi mode — fpc's default mode has no such identifier
	// and rejects the assignment. {$H+} makes String an AnsiString, so a
	// string is not silently truncated to 255 characters.
	g.writeln("{$mode objfpc}{$H+}")
	if g.needIfThen {
		g.writeln("uses StrUtils, Math;")
	}

	// Emit type declarations (structs and enums) inside the program block.
	hasTypes := false
	for _, d := range prog.Decls {
		switch d.(type) {
		case *ast.StructDecl, *ast.EnumDecl:
			if !hasTypes {
				g.writeln("type")
				g.indent++
				hasTypes = true
			}
		}
		switch node := d.(type) {
		case *ast.StructDecl:
			if err := g.emitStructDecl(node); err != nil {
				return err
			}
		case *ast.EnumDecl:
			if err := g.emitEnumDecl(node); err != nil {
				return err
			}
		}
	}
	if hasTypes {
		g.indent--
	}

	// Collect and emit var section for main.
	vars := collectVarDecls(fd.Body)
	forVars := collectForVars(fd.Body)
	declaredNames := map[string]bool{}
	for _, vd := range vars {
		declaredNames[vd.Name] = true
	}
	if len(vars) > 0 || len(forVars) > 0 {
		g.writeln("var")
		g.indent++
		for _, vd := range vars {
			g.writeIndent()
			g.writeln(vd.Name + ": " + typeToPascal(vd.Type) + ";")
		}
		for _, name := range forVars {
			if !declaredNames[name] {
				g.writeIndent()
				g.writeln(name + ": Integer;")
			}
		}
		g.indent--
	}

	// Emit non-main functions/procedures.
	for _, d := range prog.Decls {
		fdn, ok := d.(*ast.FunctionDecl)
		if !ok || fdn.Name == "main" {
			continue
		}
		if err := g.emitFunctionDecl(fdn); err != nil {
			return err
		}
		g.writeln("")
	}

	g.writeln("begin")
	g.indent++
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.indent--
	g.writeln("end.")
	return nil
}

func (g *pascalGen) emitNode(n ast.Node) error {
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
	case *ast.EnumDecl:
		return g.emitEnumDecl(node)
	case *ast.MatchExpr:
		return g.emitMatchExpr(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *pascalGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln(sd.Name + " = record")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(f.Name + ": " + typeToPascal(f.Type) + ";")
	}
	g.indent--
	g.writeIndent()
	g.writeln("end;")
	return nil
}

func (g *pascalGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.writeln(ed.Name + " = (" + strings.Join(ed.Variants, ", ") + ");")
	return nil
}

func (g *pascalGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.types.noteParams(fd)
	rt := typeToPascal(fd.ReturnType)
	isProcedure := rt == ""

	g.writeIndent()
	if isProcedure {
		g.write("procedure " + fd.Name + "(")
	} else {
		g.write("function " + fd.Name + "(")
	}
	for i, p := range fd.Params {
		if i > 0 {
			g.write("; ")
		}
		g.write(p.Name + ": " + typeToPascal(p.Type))
	}
	if isProcedure {
		g.writeln(");")
	} else {
		g.writeln("): " + rt + ";")
	}

	// Collect var declarations for the function body.
	vars := collectVarDecls(fd.Body)
	if len(vars) > 0 {
		g.writeIndent()
		g.writeln("var")
		g.indent++
		for _, vd := range vars {
			g.writeIndent()
			g.writeln(vd.Name + ": " + typeToPascal(vd.Type) + ";")
		}
		g.indent--
	}

	g.writeIndent()
	g.writeln("begin")
	g.indent++
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("end;")
	return nil
}

func (g *pascalGen) emitReturn(rs *ast.ReturnStmt) error {
	g.writeIndent()
	if rs.Value == nil {
		g.writeln("exit;")
		return nil
	}
	g.write("Result := ")
	if err := g.emitExpr(rs.Value); err != nil {
		return err
	}
	g.writeln(";")
	g.writeIndent()
	g.writeln("exit;")
	return nil
}

func (g *pascalGen) emitVarDecl(vd *ast.VarDecl) error {
	g.types.noteVar(vd)
	if vd.Value == nil {
		return nil
	}
	if sl, ok := vd.Value.(*ast.StructLit); ok {
		for _, f := range sl.Fields {
			g.writeIndent()
			g.write(vd.Name + "." + f.Name + " := ")
			if err := g.emitExpr(f.Value); err != nil {
				return err
			}
			g.writeln(";")
		}
		return nil
	}
	g.writeIndent()
	g.write(vd.Name + " := ")
	if err := g.emitExpr(vd.Value); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *pascalGen) emitAssign(as *ast.AssignStmt) error {
	g.writeIndent()
	if err := g.emitExpr(as.Target); err != nil {
		return err
	}
	g.write(" := ")
	if err := g.emitExpr(as.Value); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *pascalGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if ")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(" then")
	g.writeIndent()
	g.writeln("begin")
	g.indent++
	for _, s := range is.Then {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	if len(is.Else) > 0 {
		g.writeln("end")
		g.writeIndent()
		g.writeln("else")
		g.writeIndent()
		g.writeln("begin")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln("end;")
	} else {
		g.writeln("end;")
	}
	return nil
}

func (g *pascalGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while ")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(" do")
	g.writeIndent()
	g.writeln("begin")
	g.indent++
	for _, s := range ws.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("end;")
	return nil
}

func (g *pascalGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for " + fs.Var + " := ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write(" to ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(" - 1 do")
	case "each":
		return fmt.Errorf("XQL_E402: Pascal target does not support for-each loops")
	default:
		return fmt.Errorf("XQL_E401: unsupported for-loop form %q", fs.Form)
	}
	g.writeIndent()
	g.writeln("begin")
	g.indent++
	for _, s := range fs.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("end;")
	return nil
}

func (g *pascalGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *pascalGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("case ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(" of")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			// This is the else branch; handle after the arms.
			continue
		}
		if err := g.emitExpr(arm.Pattern); err != nil {
			return err
		}
		g.writeln(":")
		g.writeIndent()
		g.writeln("begin")
		g.indent++
		for _, s := range arm.Body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln("end;")
	}
	// Emit else branch if wildcard exists.
	for _, arm := range me.Arms {
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.indent--
			g.writeIndent()
			g.writeln("else")
			g.indent++
			g.writeIndent()
			g.writeln("begin")
			g.indent++
			for _, s := range arm.Body {
				if err := g.emitNode(s); err != nil {
					return err
				}
			}
			g.indent--
			g.writeIndent()
			g.writeln("end;")
			break
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("end;")
	return nil
}

func (g *pascalGen) emitExpr(n ast.Node) error {
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
		case "==":
			op = "="
		case "!=":
			op = "<>"
		case "%":
			op = "mod"
		case "/":
			// Pascal's `/` is real division and returns a Real, which will not
			// even assign to an Integer. `div` is the integer one.
			if g.types.isIntDivision(node) {
				op = "div"
			}
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

func (g *pascalGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("IfThen(")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(", ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(", ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *pascalGen) emitLambda(lam *ast.Lambda) error {
	return fmt.Errorf("XQL_E401: Pascal does not support Lambda expressions")
}

func (g *pascalGen) emitArrayLit(al *ast.ArrayLit) error {
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

func (g *pascalGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *pascalGen) emitStructLit(sl *ast.StructLit) error {
	// Pascal does not have struct literal syntax directly; use a helper pattern.
	g.write(sl.TypeName + "(")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + ": ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *pascalGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("WriteLn(")
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
		g.write("Write(")
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
		if len(ce.Args) > 0 {
			g.write("IntToStr(")
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
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

func (g *pascalGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write("'" + strings.ReplaceAll(s, "'", "''") + "'")
	case "Int":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%d", int64(f)))
	case "Float":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%g", f))
	case "Bool":
		b, _ := lit.Value.(bool)
		if b {
			g.write("True")
		} else {
			g.write("False")
		}
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}
