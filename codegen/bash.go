package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateBash produces Bash shell script source code from the given typed AST.
// The "main" function's body is emitted at top level after function definitions.
func GenerateBash(root ast.Node) ([]byte, error) {
	g := &bashGen{buf: &strings.Builder{}}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
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
}

func (g *bashGen) write(s string)   { g.buf.WriteString(s) }
func (g *bashGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *bashGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

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
		g.writeIndent()
		g.writeln(fmt.Sprintf("local %s=\"$%d\"", p.Name, i+1))
	}
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.inFunc = prevInFunc
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *bashGen) emitReturn(rs *ast.ReturnStmt) error {
	g.writeIndent()
	if rs.Value == nil {
		g.writeln("return")
		return nil
	}
	g.write("echo ")
	if err := g.emitExpr(rs.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *bashGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if g.inFunc {
		g.write("local ")
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
		g.write("for " + fs.Var + " in ")
		// For arrays, emit with "${arr[@]}" style.
		if err := g.emitExprUnquoted(fs.Iterable); err != nil {
			return err
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
		g.write(node.Name)
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
			return g.emitExpr(n)
		}
	case *ast.UnaryExpr:
		if node.Op == "!" {
			g.write("! ")
			return g.emitCondExpr(node.Operand)
		}
		return g.emitExpr(n)
	default:
		return g.emitExpr(n)
	}
}

func (g *bashGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write("\"${" + node.Name + "}\"")
		return nil
	case *ast.BinaryExpr:
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
			g.write(ident.Name)
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
	return fmt.Errorf("XQL_E401: Bash does not support IfExpr in expression context")
}

func (g *bashGen) emitLambda(lam *ast.Lambda) error {
	return fmt.Errorf("XQL_E401: Bash does not support Lambda expressions")
}

// emitArithExpr writes an expression inside $(( )) arithmetic context.
func (g *bashGen) emitArithExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteralRaw(node)
	case *ast.Ident:
		g.write(node.Name)
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
		for _, arg := range ce.Args {
			g.write(" ")
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
