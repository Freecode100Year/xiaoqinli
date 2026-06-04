// Package codegen implements code generation for supported target languages.
package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateGo produces Go source code from the given typed AST.
func GenerateGo(root ast.Node) ([]byte, error) {
	g := &goGen{buf: &strings.Builder{}, indent: 0, needFmt: false}

	switch n := root.(type) {
	case *ast.Program:
		for _, d := range n.Decls {
			if err := g.emitNode(d); err != nil {
				return nil, err
			}
			g.writeln("")
		}
	case *ast.FunctionDecl:
		if err := g.emitFunctionDecl(n); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program or FunctionDecl, got %s", root.Kind())
	}

	// Build final output with package header and imports.
	var out strings.Builder
	out.WriteString("package main\n")

	var imports []string
	if g.needFmt {
		imports = append(imports, `"fmt"`)
	}
	if g.needOS {
		imports = append(imports, `"os"`)
	}
	if g.needTime {
		imports = append(imports, `"time"`)
	}
	if len(imports) == 1 {
		out.WriteString("\nimport " + imports[0] + "\n")
	} else if len(imports) > 1 {
		out.WriteString("\nimport (\n")
		for _, imp := range imports {
			out.WriteString("\t" + imp + "\n")
		}
		out.WriteString(")\n")
	}

	out.WriteString("\n")
	out.WriteString(g.buf.String())
	return []byte(out.String()), nil
}

// goGen holds code-generation state.
type goGen struct {
	buf      *strings.Builder
	indent   int
	needFmt  bool
	needTime bool
	needOS   bool
}

func (g *goGen) write(s string) {
	g.buf.WriteString(s)
}

func (g *goGen) writeln(s string) {
	g.buf.WriteString(s)
	g.buf.WriteByte('\n')
}

func (g *goGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

// typeToGo converts a TypeExpr to its Go type string.
func typeToGo(t ast.TypeExpr) string {
	switch t.KindName {
	case "String":
		return "string"
	case "Int":
		return "int"
	case "Float":
		return "float64"
	case "Bool":
		return "bool"
	case "Void":
		return ""
	case "Array":
		if t.Elem != nil {
			return "[]" + typeToGo(*t.Elem)
		}
		return "[]interface{}"
	case "Option":
		if t.Elem != nil {
			return "*" + typeToGo(*t.Elem)
		}
		return "interface{}"
	case "Result":
		// Result<T, E> maps to (T, error) at the function return level,
		// handled specially in emitFunctionDecl.
		if t.OkType != nil {
			return typeToGo(*t.OkType)
		}
		return "interface{}"
	default:
		return t.KindName
	}
}

// --- Emitters ---

func (g *goGen) emitNode(n ast.Node) error {
	switch node := n.(type) {
	case *ast.FunctionDecl:
		return g.emitFunctionDecl(node)
	case *ast.ReturnStmt:
		return g.emitReturnStmt(node)
	case *ast.VarDecl:
		return g.emitVarDecl(node)
	case *ast.AssignStmt:
		return g.emitAssignStmt(node)
	case *ast.IfStmt:
		return g.emitIfStmt(node)
	case *ast.WhileStmt:
		return g.emitWhileStmt(node)
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
	default:
		return fmt.Errorf("XQL_E401: cannot emit statement for node kind %s", n.Kind())
	}
}

func (g *goGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	// func name(params) returnType {
	g.writeIndent()
	g.write("func " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + " " + typeToGo(p.Type))
	}
	g.write(")")

	// Return type.
	retType := typeToGo(fd.ReturnType)
	isResult := fd.ReturnType.KindName == "Result"
	if isResult {
		okT := "interface{}"
		if fd.ReturnType.OkType != nil {
			okT = typeToGo(*fd.ReturnType.OkType)
		}
		g.write(" (" + okT + ", error)")
	} else if retType != "" {
		g.write(" " + retType)
	}

	g.writeln(" {")
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

func (g *goGen) emitReturnStmt(rs *ast.ReturnStmt) error {
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

func (g *goGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if vd.Value != nil {
		g.write(vd.Name + " := ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	} else {
		g.write("var " + vd.Name + " " + typeToGo(vd.Type))
	}
	g.writeln("")
	return nil
}

func (g *goGen) emitAssignStmt(as *ast.AssignStmt) error {
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

func (g *goGen) emitIfStmt(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if ")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(" {")
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

func (g *goGen) emitWhileStmt(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("for ")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(" {")
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

func (g *goGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		// for i := start; i < end; i++ {
		g.write("for " + fs.Var + " := ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("; " + fs.Var + " < ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.write("; " + fs.Var + "++")
	case "each":
		// for _, v := range iterable {
		g.write("for _, " + fs.Var + " := range ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
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

func (g *goGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

// --- Expression emitters ---

func (g *goGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.BinaryExpr:
		return g.emitBinaryExpr(node)
	case *ast.UnaryExpr:
		return g.emitUnaryExpr(node)
	case *ast.CallExpr:
		return g.emitCallExpr(node)
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		if strings.HasPrefix(node.Name, "time.") {
			g.needTime = true
		}
		if strings.HasPrefix(node.Name, "os.") {
			g.needOS = true
		}
		g.write(node.Name)
		return nil
	case *ast.MemberExpr:
		return g.emitMemberExpr(node)
	case *ast.StructLit:
		return g.emitStructLit(node)
	case *ast.ArrayLit:
		return g.emitArrayLit(node)
	case *ast.IndexExpr:
		return g.emitIndexExpr(node)
	default:
		return fmt.Errorf("XQL_E401: cannot emit expression for node kind %s", n.Kind())
	}
}

func (g *goGen) emitBinaryExpr(be *ast.BinaryExpr) error {
	if err := g.emitExpr(be.Left); err != nil {
		return err
	}
	g.write(" " + be.Op + " ")
	return g.emitExpr(be.Right)
}

func (g *goGen) emitUnaryExpr(ue *ast.UnaryExpr) error {
	g.write(ue.Op)
	return g.emitExpr(ue.Operand)
}

func (g *goGen) emitCallExpr(ce *ast.CallExpr) error {
	// Map xql built-in functions to Go equivalents.
	callee := ce.Callee
	switch callee {
	case "println":
		callee = "fmt.Println"
		g.needFmt = true
	case "printf":
		callee = "fmt.Printf"
		g.needFmt = true
	case "sprintf":
		callee = "fmt.Sprintf"
		g.needFmt = true
	}

	if strings.Contains(callee, "time.") {
		g.needTime = true
	}
	if strings.Contains(callee, "os.") {
		g.needOS = true
	}

	g.write(callee + "(")
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

func (g *goGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write(fmt.Sprintf("%q", s))
	case "Int":
		// JSON numbers decode as float64.
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

func (g *goGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("type " + sd.Name + " struct {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(f.Name + " " + typeToGo(f.Type))
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *goGen) emitStructLit(sl *ast.StructLit) error {
	g.write(sl.TypeName + "{")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + ": ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write("}")
	return nil
}

func (g *goGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("[]" + typeToGo(al.ElemType) + "{")
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

func (g *goGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *goGen) emitMemberExpr(me *ast.MemberExpr) error {
	if err := g.emitExpr(me.Object); err != nil {
		return err
	}
	g.write("." + me.Field)
	return nil
}
