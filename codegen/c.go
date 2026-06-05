package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

func GenerateC(root ast.Node) ([]byte, error) {
	g := &cGen{
		buf:      &strings.Builder{},
		funcRets: make(map[string]string),
		varTypes: make(map[string]string),
	}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	if err := validateCTypes(prog); err != nil {
		return nil, err
	}

	for _, d := range prog.Decls {
		if fd, ok := d.(*ast.FunctionDecl); ok {
			g.funcRets[fd.Name] = fd.ReturnType.KindName
		}
	}

	var structs []*ast.StructDecl
	var others []ast.Node
	for _, d := range prog.Decls {
		if sd, ok := d.(*ast.StructDecl); ok {
			structs = append(structs, sd)
		} else {
			others = append(others, d)
		}
	}

	for _, sd := range structs {
		if err := g.emitStructDecl(sd); err != nil {
			return nil, err
		}
		g.writeln("")
	}

	for i, d := range others {
		if i > 0 {
			g.writeln("")
		}
		if err := g.emitNode(d); err != nil {
			return nil, err
		}
	}

	var out strings.Builder
	out.WriteString("#include <stdio.h>\n")
	if g.needStdlib {
		out.WriteString("#include <stdlib.h>\n")
	}
	if g.needString {
		out.WriteString("#include <string.h>\n")
	}
	out.WriteString("\n")

	if g.needStrcat {
		out.WriteString("static char* _xql_strcat(const char* a, const char* b) {\n")
		out.WriteString("    size_t la = strlen(a), lb = strlen(b);\n")
		out.WriteString("    char* r = (char*)malloc(la + lb + 1);\n")
		out.WriteString("    memcpy(r, a, la);\n")
		out.WriteString("    memcpy(r + la, b, lb + 1);\n")
		out.WriteString("    return r;\n")
		out.WriteString("}\n\n")
	}

	out.WriteString(g.buf.String())
	return []byte(out.String()), nil
}

func validateCTypes(prog *ast.Program) error {
	var err error
	for _, d := range prog.Decls {
		walkTypes(d, func(t ast.TypeExpr, context string) {
			if err != nil {
				return
			}
			switch t.KindName {
			case "Option":
				err = fmt.Errorf("XQL_E402: C target does not support Option<T> (used in %s)", context)
			case "Map":
				err = fmt.Errorf("XQL_E402: C target does not support Map<K,V> (used in %s)", context)
			case "Result":
				err = fmt.Errorf("XQL_E402: C target does not support Result<T> (used in %s)", context)
			}
		})
	}
	return err
}

type cGen struct {
	buf        *strings.Builder
	indent     int
	needStdlib bool
	needString bool
	needStrcat bool
	funcRets   map[string]string
	varTypes   map[string]string
}

func (g *cGen) write(s string)   { g.buf.WriteString(s) }
func (g *cGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *cGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func typeToC(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "long"
	case "Float":
		return "double"
	case "String":
		return "const char*"
	case "Bool":
		return "int"
	case "Void":
		return "void"
	case "Array":
		if t.Elem != nil {
			return typeToC(*t.Elem)
		}
		return "long"
	default:
		return "struct " + t.KindName
	}
}

func (g *cGen) inferTypeKind(n ast.Node) string {
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
	case *ast.MemberExpr:
		return ""
	case *ast.IndexExpr:
		return ""
	case *ast.UnaryExpr:
		if node.Op == "!" {
			return "Bool"
		}
		return g.inferTypeKind(node.Operand)
	default:
		return "Int"
	}
}

func (g *cGen) printfFmt(typeKind string) string {
	switch typeKind {
	case "String":
		return "%s"
	case "Float":
		return "%g"
	case "Bool":
		return "%d"
	default:
		return "%ld"
	}
}

func (g *cGen) emitNode(n ast.Node) error {
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

func (g *cGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeln("typedef struct {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(typeToC(f.Type) + " " + f.Name + ";")
	}
	g.indent--
	g.writeln("} " + sd.Name + ";")
	return nil
}

func (g *cGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	for _, p := range fd.Params {
		g.varTypes[p.Name] = p.Type.KindName
	}

	g.writeIndent()
	if fd.Name == "main" {
		g.writeln("int main() {")
	} else {
		rt := typeToC(fd.ReturnType)
		g.write(rt + " " + fd.Name + "(")
		for i, p := range fd.Params {
			if i > 0 {
				g.write(", ")
			}
			g.write(typeToC(p.Type) + " " + p.Name)
		}
		g.writeln(") {")
	}

	g.indent++
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}

	if fd.Name == "main" {
		g.writeIndent()
		g.writeln("return 0;")
	}

	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *cGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *cGen) emitVarDecl(vd *ast.VarDecl) error {
	g.varTypes[vd.Name] = vd.Type.KindName
	g.writeIndent()
	if vd.Type.KindName == "Array" && vd.Value != nil {
		if al, ok := vd.Value.(*ast.ArrayLit); ok {
			elemT := typeToC(vd.Type)
			if vd.Type.Elem != nil {
				elemT = typeToC(*vd.Type.Elem)
			}
			g.write(elemT + " " + vd.Name + "[] = {")
			for i, elem := range al.Elements {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(elem); err != nil {
					return err
				}
			}
			g.writeln("};")
			return nil
		}
	}
	g.write(typeToC(vd.Type) + " " + vd.Name)
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln(";")
	return nil
}

func (g *cGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *cGen) emitIf(is *ast.IfStmt) error {
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

func (g *cGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *cGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for (long " + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("; " + fs.Var + " < ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.write("; " + fs.Var + "++)")
	case "each":
		return fmt.Errorf("XQL_E402: C target does not support for-each loops; use range form with index")
	default:
		return fmt.Errorf("XQL_E401: unknown ForStmt form %q", fs.Form)
	}
	g.writeln(" {")
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

func (g *cGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *cGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		return g.emitBinary(node)
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
	default:
		return fmt.Errorf("XQL_E401: unsupported expression %s", n.Kind())
	}
}

func (g *cGen) emitBinary(be *ast.BinaryExpr) error {
	if be.Op == "+" && (g.inferTypeKind(be.Left) == "String" || g.inferTypeKind(be.Right) == "String") {
		g.needStrcat = true
		g.needStdlib = true
		g.needString = true
		g.write("_xql_strcat(")
		if err := g.emitExpr(be.Left); err != nil {
			return err
		}
		g.write(", ")
		if err := g.emitExpr(be.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	}
	g.write("(")
	if err := g.emitExpr(be.Left); err != nil {
		return err
	}
	g.write(" " + be.Op + " ")
	if err := g.emitExpr(be.Right); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *cGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		if len(ce.Args) == 0 {
			g.write(`printf("\n")`)
			return nil
		}
		tk := g.inferTypeKind(ce.Args[0])
		g.write(`printf("` + g.printfFmt(tk) + `\n", `)
		if err := g.emitExpr(ce.Args[0]); err != nil {
			return err
		}
		g.write(")")
		return nil
	case "printf":
		if len(ce.Args) == 0 {
			return nil
		}
		if len(ce.Args) >= 2 {
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
		}
		tk := g.inferTypeKind(ce.Args[0])
		g.write(`printf("` + g.printfFmt(tk) + `", `)
		if err := g.emitExpr(ce.Args[0]); err != nil {
			return err
		}
		g.write(")")
		return nil
	case "sprintf":
		g.needString = true
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

func (g *cGen) emitLiteral(lit *ast.Literal) error {
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

func (g *cGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeln("typedef enum {")
	g.indent++
	for i, v := range ed.Variants {
		g.writeIndent()
		if i < len(ed.Variants)-1 {
			g.writeln(ed.Name + "_" + v + ",")
		} else {
			g.writeln(ed.Name + "_" + v)
		}
	}
	g.indent--
	g.writeln("} " + ed.Name + ";")
	return nil
}

func (g *cGen) emitMatchExpr(me *ast.MatchExpr) error {
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
			g.writeln("default: {")
		} else {
			g.write("case ")
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.writeln(": {")
		}
		g.indent++
		for _, s := range arm.Body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.writeIndent()
		g.writeln("break;")
		g.indent--
		g.writeIndent()
		g.writeln("}")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *cGen) emitStructLit(sl *ast.StructLit) error {
	g.write("(" + sl.TypeName + "){")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write("." + f.Name + " = ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write("}")
	return nil
}

func (g *cGen) emitArrayLit(al *ast.ArrayLit) error {
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

func (g *cGen) emitIndexExpr(ie *ast.IndexExpr) error {
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
