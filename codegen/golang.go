// Package codegen implements code generation for supported target languages.
package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateGo produces Go source code from the given typed AST.
func GenerateGo(root ast.Node) ([]byte, error) {
	g := &goGen{
		buf:         &strings.Builder{},
		indent:      0,
		needFmt:     false,
		structTable: make(map[string]*ast.StructDecl),
		classTable:  make(map[string]*ast.ClassDecl),
	}

	walkTypes(root, func(t ast.TypeExpr, context string) {
		if t.KindName == "Result" {
			g.needResult = true
		}
	})

	if prog, ok := root.(*ast.Program); ok {
		for _, d := range prog.Decls {
			if sd, ok := d.(*ast.StructDecl); ok {
				g.structTable[sd.Name] = sd
			} else if cd, ok := d.(*ast.ClassDecl); ok {
				g.classTable[cd.Name] = cd
			}
		}
	}

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

	hasMain := false
	if prog, ok := root.(*ast.Program); ok {
		for _, d := range prog.Decls {
			if fd, ok := d.(*ast.FunctionDecl); ok && fd.Name == "main" {
				hasMain = true
			}
		}
	}
	if g.needResult && hasMain {
		out.WriteString(`
type Result struct {
	val  interface{}
	err  interface{}
	IsOk bool
}

func ResultOk(val interface{}) Result {
	return Result{val: val, IsOk: true}
}

func ResultErr(err interface{}) Result {
	return Result{err: err, IsOk: false}
}

func (r Result) Unwrap() interface{} {
	if !r.IsOk {
		panic(r.err)
	}
	return r.val
}

func (r Result) UnwrapErr() interface{} {
	if r.IsOk {
		panic("Called UnwrapErr on Ok Result")
	}
	return r.err
}
`)
	}

	out.WriteString("\n")
	out.WriteString(g.buf.String())
	return []byte(out.String()), nil
}

// goGen holds code-generation state.
type goGen struct {
	buf         *strings.Builder
	indent      int
	needFmt     bool
	needTime    bool
	needOS      bool
	needResult  bool
	currentFunc *ast.FunctionDecl
	structTable map[string]*ast.StructDecl
	classTable  map[string]*ast.ClassDecl
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
	case "Map":
		kt := "interface{}"
		vt := "interface{}"
		if t.KeyType != nil {
			kt = typeToGo(*t.KeyType)
		}
		if t.Elem != nil {
			vt = typeToGo(*t.Elem)
		}
		return "map[" + kt + "]" + vt
	case "Option":
		if t.Elem != nil {
			return "*" + typeToGo(*t.Elem)
		}
		return "interface{}"
	case "Result":
		return "Result"
	default:
		name := t.KindName
		if strings.Contains(name, ".") {
			parts := strings.Split(name, ".")
			if len(parts) == 2 {
				name = parts[1]
			}
		}
		return name
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
	case *ast.ClassDecl:
		return g.emitClassDecl(node)
	case *ast.EnumDecl:
		return g.emitEnumDecl(node)
	case *ast.MatchExpr:
		return g.emitMatchExpr(node)
	case *ast.SwitchStmt:
		return g.emitSwitchStmt(node)
	case *ast.ImportDecl:
		// Go ignores ImportDecl because they share same package
		return nil
	default:
		return fmt.Errorf("XQL_E401: cannot emit statement for node kind %s", n.Kind())
	}
}

func (g *goGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.currentFunc = fd
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
	if retType != "" {
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
		if ce, ok := vd.Value.(*ast.CallExpr); ok && (strings.HasSuffix(ce.Callee, ".unwrap") || strings.HasSuffix(ce.Callee, ".Unwrap")) {
			g.write(".(" + typeToGo(vd.Type) + ")")
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
		// for i := start; i <= end; i++ {
		g.write("for " + fs.Var + " := ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("; " + fs.Var + " <= ")
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
	case *ast.ArrayLiteral:
		return g.emitArrayLiteral(node)
	case *ast.MapLiteral:
		return g.emitMapLiteral(node)
	case *ast.IndexExpr:
		return g.emitIndexExpr(node)
	case *ast.IfExpr:
		return g.emitIfExpr(node)
	case *ast.Lambda:
		return g.emitLambda(node)
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
	if strings.Contains(callee, ".") && !strings.HasPrefix(callee, "fmt.") && !strings.HasPrefix(callee, "time.") && !strings.HasPrefix(callee, "os.") && !strings.HasPrefix(callee, "Result.") {
		parts := strings.Split(callee, ".")
		if len(parts) == 2 {
			obj := parts[0]
			method := parts[1]
			if method == "unwrap" {
				callee = obj + ".Unwrap"
			} else if method == "unwrapErr" {
				callee = obj + ".UnwrapErr"
			} else {
				callee = parts[1]
			}
		}
	}
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
	case "Result.ok":
		callee = "ResultOk"
		g.needResult = true
	case "Result.err":
		callee = "ResultErr"
		g.needResult = true
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
		name := f.Name
		if f.Visibility == "public" {
			name = capitalize(name)
		} else {
			name = uncapitalize(name)
		}
		g.writeln(name + " " + typeToGo(f.Type))
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *goGen) emitStructLit(sl *ast.StructLit) error {
	g.write(stripAlias(sl.TypeName) + "{")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		name := f.Name
		vis := g.getFieldVisibility(name)
		if vis == "public" {
			name = capitalize(name)
		} else {
			name = uncapitalize(name)
		}
		g.write(name + ": ")
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

func (g *goGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.writeln("type " + ed.Name + " int")
	g.writeln("")
	g.writeIndent()
	g.writeln("const (")
	g.indent++
	for i, v := range ed.Variants {
		g.writeIndent()
		if i == 0 {
			g.writeln(ed.Name + v + " " + ed.Name + " = iota")
		} else {
			g.writeln(ed.Name + v)
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln(")")
	return nil
}

func (g *goGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("switch ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(" {")
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
		for _, s := range arm.Body {
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

func (g *goGen) emitMemberExpr(me *ast.MemberExpr) error {
	if err := g.emitExpr(me.Object); err != nil {
		return err
	}
	g.write(".")
	name := me.Field
	if name == "isOk" {
		name = "IsOk"
	} else {
		vis := g.getFieldVisibility(name)
		if vis == "public" {
			name = capitalize(name)
		} else {
			name = uncapitalize(name)
		}
	}
	g.write(name)
	return nil
}

func (g *goGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("func() ")
	retType := g.inferIfExprType(ie)
	g.write(retType)
	g.write(" { if ")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" { return ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" }; return ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(" }()")
	return nil
}

func (g *goGen) inferIfExprType(ie *ast.IfExpr) string {
	switch n := ie.Then.(type) {
	case *ast.Literal:
		switch n.ValueType {
		case "String":
			return "string"
		case "Int":
			return "int"
		case "Float":
			return "float64"
		case "Bool":
			return "bool"
		}
	}
	return "interface{}"
}

func (g *goGen) emitLambda(lam *ast.Lambda) error {
	g.write("func(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + " " + typeToGo(p.Type))
	}
	g.write(")")
	retType := typeToGo(lam.ReturnType)
	if retType != "" {
		g.write(" " + retType)
	}
	g.write(" {")
	if len(lam.Body) == 0 {
		g.write("}")
		return nil
	}
	g.writeln("")
	g.indent++
	for _, stmt := range lam.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.write("}")
	return nil
}

func (g *goGen) emitClassDecl(cd *ast.ClassDecl) error {
	g.writeIndent()
	g.writeln("type " + cd.Name + " struct {")
	g.indent++
	for _, f := range cd.Fields {
		g.writeIndent()
		name := f.Name
		if f.Visibility == "public" {
			name = capitalize(name)
		} else {
			name = uncapitalize(name)
		}
		g.writeln(name + " " + typeToGo(f.Type))
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *goGen) emitSwitchStmt(ss *ast.SwitchStmt) error {
	g.writeIndent()
	g.write("switch ")
	if err := g.emitExpr(ss.Value); err != nil {
		return err
	}
	g.writeln(" {")
	for _, c := range ss.Cases {
		g.writeIndent()
		if c.Value != nil {
			g.write("case ")
			if err := g.emitExpr(c.Value); err != nil {
				return err
			}
			g.writeln(":")
		} else {
			g.writeln("default:")
		}
		g.indent++
		for _, stmt := range c.Body {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
		g.indent--
	}
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *goGen) emitMapLiteral(ml *ast.MapLiteral) error {
	g.write(typeToGo(ast.TypeExpr{KindName: "Map", KeyType: &ml.KeyType, Elem: &ml.ValueType}) + "{")
	for i, entry := range ml.Entries {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(entry.Key); err != nil {
			return err
		}
		g.write(": ")
		if err := g.emitExpr(entry.Value); err != nil {
			return err
		}
	}
	g.write("}")
	return nil
}

func (g *goGen) emitArrayLiteral(al *ast.ArrayLiteral) error {
	g.write(typeToGo(ast.TypeExpr{KindName: "Array", Elem: &al.ElemType}) + "{")
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

func (g *goGen) getFieldVisibility(fieldName string) string {
	for _, sd := range g.structTable {
		for _, f := range sd.Fields {
			if f.Name == fieldName {
				return f.Visibility
			}
		}
	}
	for _, cd := range g.classTable {
		for _, f := range cd.Fields {
			if f.Name == fieldName {
				return f.Visibility
			}
		}
	}
	return ""
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] = r[0] - 'a' + 'A'
	}
	return string(r)
}

func uncapitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	r := []rune(s)
	if r[0] >= 'A' && r[0] <= 'Z' {
		r[0] = r[0] - 'A' + 'a'
	}
	return string(r)
}

func stripAlias(name string) string {
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return name
}
