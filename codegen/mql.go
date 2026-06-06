package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateMQL4 produces MQL4 source code (.mq4) from the given typed AST.
func GenerateMQL4(root ast.Node) ([]byte, error) {
	return generateMQL(root, "mql4")
}

// GenerateMQL5 produces MQL5 source code (.mq5) from the given typed AST.
func GenerateMQL5(root ast.Node) ([]byte, error) {
	return generateMQL(root, "mql5")
}

func generateMQL(root ast.Node, dialect string) ([]byte, error) {
	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Validate: reject Map, Option, Result types up front.
	if err := validateMQLTypes(root, dialect); err != nil {
		return nil, err
	}

	// Validate: reject for-each loops.
	if err := validateMQLForEach(prog); err != nil {
		return nil, err
	}

	g := &mqlGen{buf: &strings.Builder{}, dialect: dialect}

	// Separate declarations: structs and non-main functions go before OnStart,
	// the main function becomes OnStart.
	var structs []ast.Node
	var funcs []ast.Node
	var mainFunc *ast.FunctionDecl

	for _, d := range prog.Decls {
		switch node := d.(type) {
		case *ast.StructDecl:
			structs = append(structs, node)
		case *ast.FunctionDecl:
			if node.Name == "main" {
				mainFunc = node
			} else {
				funcs = append(funcs, node)
			}
		default:
			funcs = append(funcs, d)
		}
	}

	// Emit structs first.
	for i, s := range structs {
		if i > 0 {
			g.writeln("")
		}
		if err := g.emitNode(s); err != nil {
			return nil, err
		}
	}

	if len(structs) > 0 && (len(funcs) > 0 || mainFunc != nil) {
		g.writeln("")
	}

	// Emit non-main functions.
	for i, f := range funcs {
		if i > 0 {
			g.writeln("")
		}
		if err := g.emitNode(f); err != nil {
			return nil, err
		}
	}

	if len(funcs) > 0 && mainFunc != nil {
		g.writeln("")
	}

	// Emit main as OnStart.
	if mainFunc != nil {
		if err := g.emitMainAsOnStart(mainFunc); err != nil {
			return nil, err
		}
	}

	// Build final output with header.
	var out strings.Builder
	out.WriteString("#property strict\n\n")
	out.WriteString(g.buf.String())

	return []byte(out.String()), nil
}

// validateMQLTypes checks for unsupported types: Map, Option, Result.
func validateMQLTypes(root ast.Node, dialect string) error {
	var err error
	walkTypes(root, func(t ast.TypeExpr, context string) {
		if err != nil {
			return
		}
		switch t.KindName {
		case "Map":
			err = fmt.Errorf("XQL_E403: MQL does not support Map type (used in %s)", context)
		case "Option":
			err = fmt.Errorf("XQL_E403: MQL does not support Option type (used in %s)", context)
		case "Result":
			err = fmt.Errorf("XQL_E403: MQL does not support Result type (used in %s)", context)
		}
	})
	return err
}

// validateMQLForEach walks the AST for any for-each loops which MQL cannot express.
func validateMQLForEach(prog *ast.Program) error {
	for _, d := range prog.Decls {
		if err := scanForEach(d); err != nil {
			return err
		}
	}
	return nil
}

func scanForEach(n ast.Node) error {
	switch node := n.(type) {
	case *ast.FunctionDecl:
		for _, s := range node.Body {
			if err := scanForEach(s); err != nil {
				return err
			}
		}
	case *ast.ForStmt:
		if node.Form == "each" {
			return fmt.Errorf("XQL_E403: MQL does not support for-each loops")
		}
		for _, s := range node.Body {
			if err := scanForEach(s); err != nil {
				return err
			}
		}
	case *ast.IfStmt:
		for _, s := range node.Then {
			if err := scanForEach(s); err != nil {
				return err
			}
		}
		for _, s := range node.Else {
			if err := scanForEach(s); err != nil {
				return err
			}
		}
	case *ast.WhileStmt:
		for _, s := range node.Body {
			if err := scanForEach(s); err != nil {
				return err
			}
		}
	}
	return nil
}

type mqlGen struct {
	buf     *strings.Builder
	indent  int
	dialect string // "mql4" or "mql5"
}

func (g *mqlGen) write(s string)   { g.buf.WriteString(s) }
func (g *mqlGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *mqlGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

// typeStr returns the MQL base type string for a TypeExpr.
// For Array types it returns the element type; the caller handles [] syntax.
func (g *mqlGen) typeStr(t ast.TypeExpr) (string, error) {
	switch t.KindName {
	case "Int":
		return "long", nil
	case "Float":
		return "double", nil
	case "String":
		return "string", nil
	case "Bool":
		return "bool", nil
	case "Void":
		return "void", nil
	case "Array":
		if t.Elem != nil {
			return g.typeStr(*t.Elem)
		}
		return "long", nil // default element type
	case "Map":
		return "", fmt.Errorf("XQL_E403: MQL does not support Map type")
	case "Option":
		return "", fmt.Errorf("XQL_E403: MQL does not support Option type")
	case "Result":
		return "", fmt.Errorf("XQL_E403: MQL does not support Result type")
	default:
		// User-defined struct type.
		return t.KindName, nil
	}
}

func (g *mqlGen) isArrayType(t ast.TypeExpr) bool {
	return t.KindName == "Array"
}

// --- Node emitters ---

func (g *mqlGen) emitNode(n ast.Node) error {
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
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *mqlGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("struct " + sd.Name + " {")
	g.indent++
	for _, f := range sd.Fields {
		ts, err := g.typeStr(f.Type)
		if err != nil {
			return err
		}
		g.writeIndent()
		if g.isArrayType(f.Type) {
			g.writeln(ts + " " + f.Name + "[];")
		} else {
			g.writeln(ts + " " + f.Name + ";")
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln("};")
	return nil
}

func (g *mqlGen) emitMainAsOnStart(fd *ast.FunctionDecl) error {
	g.writeIndent()
	g.writeln("void OnStart() {")
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

func (g *mqlGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	if fd.Name == "main" {
		return g.emitMainAsOnStart(fd)
	}

	rt, err := g.typeStr(fd.ReturnType)
	if err != nil {
		return err
	}

	g.writeIndent()
	if g.isArrayType(fd.ReturnType) {
		// MQL does not support returning arrays directly; emit as void.
		// This is a simplification for the language skeleton.
		g.write("void " + fd.Name + "(")
	} else {
		g.write(rt + " " + fd.Name + "(")
	}

	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		pts, err := g.typeStr(p.Type)
		if err != nil {
			return err
		}
		if g.isArrayType(p.Type) {
			g.write(pts + " &" + p.Name + "[]")
		} else {
			g.write(pts + " " + p.Name)
		}
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

func (g *mqlGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *mqlGen) emitVarDecl(vd *ast.VarDecl) error {
	ts, err := g.typeStr(vd.Type)
	if err != nil {
		return err
	}

	g.writeIndent()
	if g.isArrayType(vd.Type) {
		g.write(ts + " " + vd.Name + "[]")
		if vd.Value != nil {
			g.write(" = ")
			if err := g.emitExpr(vd.Value); err != nil {
				return err
			}
		}
		g.writeln(";")
	} else {
		g.write(ts + " " + vd.Name)
		if vd.Value != nil {
			g.write(" = ")
			if err := g.emitExpr(vd.Value); err != nil {
				return err
			}
		}
		g.writeln(";")
	}
	return nil
}

func (g *mqlGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *mqlGen) emitIf(is *ast.IfStmt) error {
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

func (g *mqlGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *mqlGen) emitForStmt(fs *ast.ForStmt) error {
	if fs.Form == "each" {
		return fmt.Errorf("XQL_E403: MQL does not support for-each loops")
	}

	g.writeIndent()
	g.write("for (long " + fs.Var + " = ")
	if err := g.emitExpr(fs.Start); err != nil {
		return err
	}
	g.write("; " + fs.Var + " < ")
	if err := g.emitExpr(fs.End); err != nil {
		return err
	}
	g.writeln("; " + fs.Var + "++) {")

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

func (g *mqlGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

// --- Expression emitters ---

func (g *mqlGen) emitExpr(n ast.Node) error {
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
		g.write(" " + node.Op + " ")
		if err := g.emitExpr(node.Right); err != nil {
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
	default:
		return fmt.Errorf("XQL_E401: unsupported expression %s", n.Kind())
	}
}

func (g *mqlGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *mqlGen) emitLambda(lam *ast.Lambda) error {
	return fmt.Errorf("XQL_E401: MQL does not support Lambda expressions")
}

func (g *mqlGen) emitArrayLit(al *ast.ArrayLit) error {
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

func (g *mqlGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *mqlGen) emitStructLit(sl *ast.StructLit) error {
	g.write("{")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write("}")
	return nil
}

func (g *mqlGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println", "printf":
		g.write("Print(")
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
			// Emit the argument directly; MQL Print handles implicit conversion.
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
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

func (g *mqlGen) emitLiteral(lit *ast.Literal) error {
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
			g.write("true")
		} else {
			g.write("false")
		}
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}
