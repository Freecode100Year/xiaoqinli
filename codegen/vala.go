package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateVala produces Vala source code from the given typed AST.
// Structs become public classes, enums become public enums, and
// the "main" function is emitted as `static int main(string[] args)`.
func GenerateVala(root ast.Node) ([]byte, error) {
	g := &valaGen{buf: &strings.Builder{}}
	g.types = newTypeKinds(root)

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// First pass: emit struct/enum declarations at top level.
	first := true
	for _, d := range prog.Decls {
		switch node := d.(type) {
		case *ast.StructDecl:
			if !first {
				g.writeln("")
			}
			if err := g.emitStructDecl(node); err != nil {
				return nil, err
			}
			first = false
		case *ast.EnumDecl:
			if !first {
				g.writeln("")
			}
			if err := g.emitEnumDecl(node); err != nil {
				return nil, err
			}
			first = false
		}
	}

	// Second pass: emit non-main functions.
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

	// Third pass: emit main function.
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		if !first {
			g.writeln("")
		}
		if err := g.emitMainFunction(fd); err != nil {
			return nil, err
		}
	}

	return []byte(g.buf.String()), nil
}

type valaGen struct {
	types  *typeKinds
	buf    *strings.Builder
	indent int
}

func (g *valaGen) write(s string)   { g.buf.WriteString(s) }
func (g *valaGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *valaGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func typeToVala(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "int64"
	case "Float":
		return "double"
	case "String":
		return "string"
	case "Bool":
		return "bool"
	case "Void":
		return "void"
	case "Array":
		if t.Elem != nil {
			return typeToVala(*t.Elem) + "[]"
		}
		return "string[]"
	case "Option":
		if t.Elem != nil {
			return typeToVala(*t.Elem) + "?"
		}
		return "string?"
	case "Result":
		if t.OkType != nil {
			return typeToVala(*t.OkType)
		}
		return "Object"
	default:
		return t.KindName
	}
}

// --- Node emitters ---

func (g *valaGen) emitNode(n ast.Node) error {
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
		return g.emitMatchStmt(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *valaGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("public class " + sd.Name + " {")
	g.indent++

	// Properties
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln("public " + typeToVala(f.Type) + " " + f.Name + " { get; set; }")
	}

	// Constructor
	g.writeln("")
	g.writeIndent()
	g.write("public " + sd.Name + "(")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(typeToVala(f.Type) + " " + f.Name)
	}
	g.writeln(") {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln("this." + f.Name + " = " + f.Name + ";")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")

	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *valaGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.writeln("public enum " + ed.Name + " {")
	g.indent++
	for i, v := range ed.Variants {
		g.writeIndent()
		if i < len(ed.Variants)-1 {
			g.writeln(v + ",")
		} else {
			g.writeln(v)
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *valaGen) emitMainFunction(fd *ast.FunctionDecl) error {
	g.writeIndent()
	g.writeln("static int main(string[] args) {")
	g.indent++
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.writeIndent()
	g.writeln("return 0;")
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *valaGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.types.noteParams(fd)
	g.writeIndent()
	rt := typeToVala(fd.ReturnType)
	g.write("static " + rt + " " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(typeToVala(p.Type) + " " + p.Name)
	}
	g.writeln(") {")
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

func (g *valaGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *valaGen) emitVarDecl(vd *ast.VarDecl) error {
	g.types.noteVar(vd)
	g.writeIndent()
	if vd.Value != nil {
		g.write(typeToVala(vd.Type) + " " + vd.Name + " = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	} else {
		g.write(typeToVala(vd.Type) + " " + vd.Name + " = " + valaDefaultValue(vd.Type))
	}
	g.writeln(";")
	return nil
}

func valaDefaultValue(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "0"
	case "Float":
		return "0.0"
	case "String":
		return `""`
	case "Bool":
		return "false"
	default:
		return "null"
	}
}

func (g *valaGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *valaGen) emitIf(is *ast.IfStmt) error {
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
	g.writeIndent()

	if len(is.Else) > 0 {
		g.writeln("} else {")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
	}
	g.writeln("}")
	return nil
}

func (g *valaGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *valaGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	if fs.Form == "range" {
		g.write("for (int64 " + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("; " + fs.Var + " < ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln("; " + fs.Var + "++) {")
	} else if fs.Form == "each" {
		g.write("foreach (var " + fs.Var + " in ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(") {")
	} else {
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
	g.writeln("}")
	return nil
}

func (g *valaGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *valaGen) emitMatchStmt(me *ast.MatchExpr) error {
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
			g.writeln("default:")
		} else {
			g.write("case ")
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.writeln(":")
		}
		g.indent++
		for _, stmt := range arm.Body {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
		g.writeIndent()
		g.writeln("break;")
		g.indent--
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

// --- Expression emitters ---

func (g *valaGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		// Vala compiles to C, and it hands C the literals as written: `int64
		// wide = (100000 * 100000)` becomes int arithmetic assigned to a gint64,
		// so it printed 1410065408. The variables were never the problem — the
		// declared type is already int64 — so only literal operands are cast,
		// which leaves subscripts and array bounds as ints, where Vala wants
		// them.
		emit := func(operand ast.Node) error {
			if widenIntLiteral(g.types, node, operand) {
				g.write("(int64) ")
			}
			return g.emitExpr(operand)
		}
		g.write("(")
		if err := emit(node.Left); err != nil {
			return err
		}
		g.write(" " + node.Op + " ")
		if err := emit(node.Right); err != nil {
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

func (g *valaGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *valaGen) emitLambda(lam *ast.Lambda) error {
	g.write("(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name)
	}
	g.write(") => {")
	for i, stmt := range lam.Body {
		if i > 0 {
			g.write(" ")
		} else {
			g.write(" ")
		}
		if es, ok := stmt.(*ast.ExprStmt); ok {
			if err := g.emitExpr(es.Expr); err != nil {
				return err
			}
			g.write(";")
		} else if rs, ok := stmt.(*ast.ReturnStmt); ok && rs.Value != nil {
			g.write("return ")
			if err := g.emitExpr(rs.Value); err != nil {
				return err
			}
			g.write(";")
		} else {
			if err := g.emitExpr(stmt); err != nil {
				return err
			}
			g.write(";")
		}
	}
	g.write(" }")
	return nil
}

func (g *valaGen) emitMatchExpr(me *ast.MatchExpr) error {
	// Vala doesn't have switch as an expression; we emit an inline workaround
	// using a ternary chain.
	for i, arm := range me.Arms {
		isWildcard := false
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			isWildcard = true
		}
		if isWildcard {
			// default arm — emit the body expression
			if len(arm.Body) > 0 {
				if es, ok := arm.Body[0].(*ast.ExprStmt); ok {
					if err := g.emitExpr(es.Expr); err != nil {
						return err
					}
				} else if rs, ok := arm.Body[0].(*ast.ReturnStmt); ok && rs.Value != nil {
					if err := g.emitExpr(rs.Value); err != nil {
						return err
					}
				}
			}
			// Close all open parens from prior arms
			for j := 0; j < i; j++ {
				g.write(")")
			}
			return nil
		}

		g.write("(")
		if err := g.emitExpr(me.Value); err != nil {
			return err
		}
		g.write(" == ")
		if err := g.emitExpr(arm.Pattern); err != nil {
			return err
		}
		g.write(" ? ")
		if len(arm.Body) > 0 {
			if es, ok := arm.Body[0].(*ast.ExprStmt); ok {
				if err := g.emitExpr(es.Expr); err != nil {
					return err
				}
			} else if rs, ok := arm.Body[0].(*ast.ReturnStmt); ok && rs.Value != nil {
				if err := g.emitExpr(rs.Value); err != nil {
					return err
				}
			}
		}
		g.write(" : ")
	}
	// If no wildcard arm, emit null as fallback
	g.write("null")
	for i := 0; i < len(me.Arms); i++ {
		g.write(")")
	}
	return nil
}

func (g *valaGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("{")
	for i, elem := range al.Elements {
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

func (g *valaGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *valaGen) emitStructLit(sl *ast.StructLit) error {
	g.write("new " + sl.TypeName + "(")
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

func (g *valaGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		if len(ce.Args) == 0 {
			g.write(`stdout.printf("\n")`)
			return nil
		}
		g.write("stdout.printf(\"%s\\n\", ")
		if err := g.emitExpr(ce.Args[0]); err != nil {
			return err
		}
		if !stringValued(g.types, ce.Args[0]) {
			g.write(".to_string()")
		}
		g.write(")")
		return nil
	case "printf":
		g.write("stdout.printf(")
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
		if len(ce.Args) >= 2 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(".printf(")
			for i, arg := range ce.Args[1:] {
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
			g.write(".to_string()")
		} else {
			g.write(`""`)
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

func (g *valaGen) emitLiteral(lit *ast.Literal) error {
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
