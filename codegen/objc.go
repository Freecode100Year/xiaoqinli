package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateObjC produces Objective-C source code from the given typed AST.
// The "main" function wraps its body in int main() { @autoreleasepool { ... } return 0; }.
func GenerateObjC(root ast.Node) ([]byte, error) {
	g := &objcGen{
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

	// Emit struct and enum declarations first.
	var header strings.Builder
	header.WriteString("#import <Foundation/Foundation.h>\n\n")

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

	// Emit non-main functions.
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

	// Emit main function.
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		if !first {
			g.writeln("")
		}
		g.writeln("int main() {")
		g.indent++
		g.writeIndent()
		g.writeln("@autoreleasepool {")
		g.indent++
		for _, p := range fd.Params {
			g.varTypes[p.Name] = p.Type.KindName
		}
		for _, stmt := range fd.Body {
			if err := g.emitNode(stmt); err != nil {
				return nil, err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln("}")
		g.writeIndent()
		g.writeln("return 0;")
		g.indent--
		g.writeln("}")
	}

	header.WriteString(g.buf.String())
	return []byte(header.String()), nil
}

type objcGen struct {
	buf      *strings.Builder
	indent   int
	funcRets map[string]string
	varTypes map[string]string
}

func (g *objcGen) write(s string)   { g.buf.WriteString(s) }
func (g *objcGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *objcGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func typeToObjC(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "NSInteger"
	case "Float":
		return "double"
	case "String":
		return "NSString*"
	case "Bool":
		return "BOOL"
	case "Void":
		return "void"
	case "Array":
		return "NSArray*"
	default:
		return t.KindName
	}
}

func (g *objcGen) inferTypeKind(n ast.Node) string {
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

func (g *objcGen) emitNode(n ast.Node) error {
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

func (g *objcGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeln("typedef struct {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln(typeToObjC(f.Type) + " " + f.Name + ";")
	}
	g.indent--
	g.writeln("} " + sd.Name + ";")
	return nil
}

func (g *objcGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeln("typedef NS_ENUM(NSInteger, " + ed.Name + ") {")
	g.indent++
	for i, v := range ed.Variants {
		g.writeIndent()
		if i < len(ed.Variants)-1 {
			g.writeln(ed.Name + v + ",")
		} else {
			g.writeln(ed.Name + v)
		}
	}
	g.indent--
	g.writeln("};")
	return nil
}

func (g *objcGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	for _, p := range fd.Params {
		g.varTypes[p.Name] = p.Type.KindName
	}

	g.writeIndent()
	rt := typeToObjC(fd.ReturnType)
	g.write(rt + " " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(typeToObjC(p.Type) + " " + p.Name)
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

func (g *objcGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *objcGen) emitVarDecl(vd *ast.VarDecl) error {
	g.varTypes[vd.Name] = vd.Type.KindName
	g.writeIndent()
	g.write(typeToObjC(vd.Type) + " " + vd.Name)
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln(";")
	return nil
}

func (g *objcGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *objcGen) emitIf(is *ast.IfStmt) error {
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

func (g *objcGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *objcGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for (NSInteger " + fs.Var + " = ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("; " + fs.Var + " < ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.write("; " + fs.Var + "++)")
	case "each":
		g.write("for (id " + fs.Var + " in ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.write(")")
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

func (g *objcGen) emitMatchExpr(me *ast.MatchExpr) error {
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

func (g *objcGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *objcGen) emitExpr(n ast.Node) error {
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
	case *ast.IfExpr:
		return g.emitIfExpr(node)
	case *ast.Lambda:
		return g.emitLambda(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported expression %s", n.Kind())
	}
}

func (g *objcGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *objcGen) emitLambda(lam *ast.Lambda) error {
	g.write("^(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(typeToObjC(p.Type) + " " + p.Name)
	}
	g.write(") { ")
	for i, stmt := range lam.Body {
		if i > 0 {
			g.write("; ")
		}
		if es, ok := stmt.(*ast.ExprStmt); ok {
			if err := g.emitExpr(es.Expr); err != nil {
				return err
			}
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
		}
	}
	g.write(" }")
	return nil
}

func (g *objcGen) emitBinary(be *ast.BinaryExpr) error {
	if be.Op == "+" && (g.inferTypeKind(be.Left) == "String" || g.inferTypeKind(be.Right) == "String") {
		g.write("[")
		if err := g.emitExpr(be.Left); err != nil {
			return err
		}
		g.write(" stringByAppendingString:")
		if err := g.emitExpr(be.Right); err != nil {
			return err
		}
		g.write("]")
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

func (g *objcGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		if len(ce.Args) == 0 {
			g.write(`NSLog(@"")`)
			return nil
		}
		tk := g.inferTypeKind(ce.Args[0])
		switch tk {
		case "String":
			g.write(`NSLog(@"%@", `)
		case "Float":
			g.write(`NSLog(@"%g", `)
		case "Bool":
			g.write(`NSLog(@"%d", `)
		default:
			g.write(`NSLog(@"%ld", (long)`)
		}
		if err := g.emitExpr(ce.Args[0]); err != nil {
			return err
		}
		g.write(")")
		return nil
	case "printf":
		if len(ce.Args) == 0 {
			return nil
		}
		tk := g.inferTypeKind(ce.Args[0])
		switch tk {
		case "String":
			g.write(`printf("%s", [`)
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(" UTF8String])")
		case "Float":
			g.write(`printf("%g", `)
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(")")
		default:
			g.write(`printf("%ld", (long)`)
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(")")
		}
		return nil
	case "sprintf":
		g.write("[NSString stringWithFormat:@\"%@\", ")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write("]")
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

func (g *objcGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write(fmt.Sprintf("@%q", s))
	case "Int":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%d", int64(f)))
	case "Float":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%g", f))
	case "Bool":
		b, _ := lit.Value.(bool)
		if b {
			g.write("YES")
		} else {
			g.write("NO")
		}
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}

func (g *objcGen) emitStructLit(sl *ast.StructLit) error {
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

func (g *objcGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("@[")
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

func (g *objcGen) emitIndexExpr(ie *ast.IndexExpr) error {
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
