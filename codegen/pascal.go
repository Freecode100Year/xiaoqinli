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

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Emit type declarations (structs and enums).
	for _, d := range prog.Decls {
		switch node := d.(type) {
		case *ast.StructDecl:
			if err := g.emitStructDecl(node); err != nil {
				return nil, err
			}
			g.writeln("")
		case *ast.EnumDecl:
			if err := g.emitEnumDecl(node); err != nil {
				return nil, err
			}
			g.writeln("")
		}
	}

	// Emit non-main functions/procedures.
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

	// Emit main program block.
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		if err := g.emitMainBlock(fd); err != nil {
			return nil, err
		}
	}

	return []byte(g.buf.String()), nil
}

type pascalGen struct {
	buf    *strings.Builder
	indent int
}

func (g *pascalGen) write(s string)   { g.buf.WriteString(s) }
func (g *pascalGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *pascalGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

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

// collectVarDecls gathers all VarDecl nodes from a statement list (non-recursive into nested blocks).
func collectVarDecls(stmts []ast.Node) []*ast.VarDecl {
	var vars []*ast.VarDecl
	for _, s := range stmts {
		if vd, ok := s.(*ast.VarDecl); ok {
			vars = append(vars, vd)
		}
	}
	return vars
}

func (g *pascalGen) emitMainBlock(fd *ast.FunctionDecl) error {
	g.writeln("program Main;")

	// Collect and emit var section for main.
	vars := collectVarDecls(fd.Body)
	if len(vars) > 0 {
		g.writeln("var")
		g.indent++
		for _, vd := range vars {
			g.writeIndent()
			g.writeln(vd.Name + ": " + typeToPascal(vd.Type) + ";")
		}
		g.indent--
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
	g.writeln("type " + sd.Name + " = record")
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
	g.writeln("type " + ed.Name + " = (" + strings.Join(ed.Variants, ", ") + ");")
	return nil
}

func (g *pascalGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
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
	// Variable declaration is in the var section; here we only emit the assignment if there is a value.
	if vd.Value != nil {
		g.writeIndent()
		g.write(vd.Name + " := ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
		g.writeln(";")
	}
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
	g.writeln("end")
	if len(is.Else) > 0 {
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
		// Rewrite the last "end\n" to "end;\n"
		// Actually just append the semicolon properly.
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
		case "!=":
			op = "<>"
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
	return fmt.Errorf("XQL_E401: Pascal does not support IfExpr (ternary expressions)")
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
