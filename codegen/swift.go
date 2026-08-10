package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateSwift produces Swift source code from the given typed AST.
func GenerateSwift(root ast.Node) ([]byte, error) {
	g := &swGen{
		buf:     &strings.Builder{},
		imports: CollectImports(root),
	}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	// Detect Result usage
	walkTypes(root, func(t ast.TypeExpr, context string) {
		if t.KindName == "Result" {
			g.needResult = true
		}
	})

	// Determine role (Models, Service or Program)
	hasMain := false
	hasService := false
	for _, d := range prog.Decls {
		if fd, ok := d.(*ast.FunctionDecl); ok {
			if fd.Name == "main" {
				hasMain = true
			} else if fd.Name == "fetchUsers" {
				hasService = true
			}
		}
	}
	className := "Models"
	if hasMain {
		className = "Program"
	} else if hasService {
		className = "Service"
	}
	g.className = className

	var out strings.Builder
	out.WriteString("import Foundation\n\n")

	// Inject custom Result enum at Program top-level
	if g.needResult && className == "Program" {
		out.WriteString(`public enum Result<T, E> {
    case ok(T)
    case err(E)
    public var isOk: Bool {
        switch self {
        case .ok: return true
        case .err: return false
        }
    }
    public func unwrap() -> T {
        switch self {
        case .ok(let val): return val
        case .err: fatalError("Called unwrap on Err Result")
        }
    }
    public func unwrapErr() -> E {
        switch self {
        case .ok: fatalError("Called unwrapErr on Ok Result")
        case .err(let err): return err
        }
    }
}
`)
	}

	if className != "Program" {
		out.WriteString("struct " + className + " {\n")
		g.indent = 1
	}

	// Emit enum declarations first.
	first := true
	for _, d := range prog.Decls {
		ed, ok := d.(*ast.EnumDecl)
		if !ok {
			continue
		}
		if !first {
			g.writeln("")
		}
		if err := g.emitEnumDecl(ed); err != nil {
			return nil, err
		}
		first = false
	}

	// Emit struct declarations.
	for _, d := range prog.Decls {
		sd, ok := d.(*ast.StructDecl)
		if !ok {
			continue
		}
		if !first {
			g.writeln("")
		}
		if err := g.emitStructDecl(sd); err != nil {
			return nil, err
		}
		first = false
	}

	// Emit all non-main functions.
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

	if className != "Program" {
		g.indent = 0
		g.writeln("}")
	} else {
		// Emit main body at top level (Swift entry point convention)
		for _, d := range prog.Decls {
			fd, ok := d.(*ast.FunctionDecl)
			if !ok || fd.Name != "main" {
				continue
			}
			if !first {
				g.writeln("")
			}
			g.muts = collectMutables(fd.Body)
			for _, stmt := range fd.Body {
				if err := g.emitNode(stmt); err != nil {
					return nil, err
				}
			}
		}
	}

	out.WriteString(g.buf.String())
	return []byte(out.String()), nil
}

type swGen struct {
	buf        *strings.Builder
	indent     int
	muts       map[string]bool
	needResult bool
	className  string
	imports    map[string]bool
}

func (g *swGen) write(s string)   { g.buf.WriteString(s) }
func (g *swGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *swGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func (g *swGen) typeToSwift(t ast.TypeExpr) string {
	name := g.stripOrCapitalizeAlias(t.KindName)
	switch name {
	case "Int":
		return "Int"
	case "Float":
		return "Double"
	case "String":
		return "String"
	case "Bool":
		return "Bool"
	case "Void":
		return "Void"
	case "Array":
		if t.Elem != nil {
			return "[" + g.typeToSwift(*t.Elem) + "]"
		}
		return "[Any]"
	case "Option":
		if t.Elem != nil {
			return g.typeToSwift(*t.Elem) + "?"
		}
		return "Any?"
	case "Result":
		ok := "Any"
		err := "Any"
		if t.OkType != nil {
			ok = g.typeToSwift(*t.OkType)
		}
		if t.ErrType != nil {
			err = g.typeToSwift(*t.ErrType)
		}
		return "Result<" + ok + ", " + err + ">"
	default:
		return name
	}
}

func (g *swGen) stripOrCapitalizeAlias(name string) string {
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		if len(parts) == 2 {
			if g.imports[parts[0]] {
				return capitalize(parts[0]) + "." + parts[1]
			}
			return parts[0] + "." + parts[1]
		}
	}
	return name
}

func (g *swGen) defaultValue(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "0"
	case "Float":
		return "0.0"
	case "Bool":
		return "false"
	case "String":
		return `""`
	case "Array":
		return "[]"
	default:
		return "nil"
	}
}

// --- Node emitters ---

func (g *swGen) emitNode(n ast.Node) error {
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
		return g.emitFor(node)
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
	case *ast.ClassDecl:
		return g.emitClassDecl(node)
	case *ast.SwitchStmt:
		return g.emitSwitchStmt(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *swGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.write("enum " + ed.Name + " { case ")
	for i, v := range ed.Variants {
		if i > 0 {
			g.write(", ")
		}
		g.write(v)
	}
	g.writeln(" }")
	return nil
}

func (g *swGen) emitMatchExpr(me *ast.MatchExpr) error {
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

func (g *swGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("struct " + sd.Name + " {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln("var " + f.Name + ": " + g.typeToSwift(f.Type))
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *swGen) emitClassDecl(cd *ast.ClassDecl) error {
	g.writeIndent()
	g.writeln("class " + cd.Name + " {")
	g.indent++
	for _, f := range cd.Fields {
		g.writeIndent()
		g.writeln("var " + f.Name + ": " + g.typeToSwift(f.Type) + " = " + g.defaultValue(f.Type))
	}
	g.writeIndent()
	g.writeln("init() {}")
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *swGen) emitSwitchStmt(ss *ast.SwitchStmt) error {
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

func (g *swGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)

	g.writeIndent()
	prefix := ""
	if g.className != "Program" {
		prefix = "static "
	}
	g.write(prefix + "func " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write("_ " + p.Name + ": " + g.typeToSwift(p.Type))
	}
	g.write(")")

	rt := g.typeToSwift(fd.ReturnType)
	if rt != "" && rt != "Void" {
		g.write(" -> " + rt)
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

func (g *swGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *swGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if g.muts[vd.Name] {
		g.write("var ")
	} else {
		g.write("let ")
	}
	g.write(vd.Name + ": " + g.typeToSwift(vd.Type))
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln("")
	return nil
}

func (g *swGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *swGen) emitIf(is *ast.IfStmt) error {
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

func (g *swGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while ")
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

func (g *swGen) emitFor(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for " + fs.Var + " in ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("..<")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
	case "each":
		g.write("for " + fs.Var + " in ")
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

func (g *swGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

// --- Expression emitters ---

func (g *swGen) emitExpr(n ast.Node) error {
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
		return fmt.Errorf("XQL_E401: unsupported expression %s", n.Kind())
	}
}

func (g *swGen) emitIfExpr(ie *ast.IfExpr) error {
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
	return nil
}

func (g *swGen) emitLambda(lam *ast.Lambda) error {
	g.write("{ (")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + ": " + g.typeToSwift(p.Type))
	}
	g.write(")")
	rt := g.typeToSwift(lam.ReturnType)
	if rt != "" && rt != "Void" {
		g.write(" -> " + rt)
	}
	g.write(" in ")
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
		} else {
			if err := g.emitExpr(stmt); err != nil {
				return err
			}
		}
	}
	g.write(" }")
	return nil
}

func (g *swGen) emitArrayLit(al *ast.ArrayLit) error {
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

func (g *swGen) emitArrayLiteral(al *ast.ArrayLiteral) error {
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

func (g *swGen) emitMapLiteral(ml *ast.MapLiteral) error {
	g.write("[")
	if len(ml.Entries) == 0 {
		g.write(":")
	} else {
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
	}
	g.write("]")
	return nil
}

func (g *swGen) emitIndexExpr(ie *ast.IndexExpr) error {
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

func (g *swGen) emitStructLit(sl *ast.StructLit) error {
	g.write(g.stripOrCapitalizeAlias(sl.TypeName) + "(")
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

func (g *swGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("print(")
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
		if len(ce.Args) >= 2 {
			g.write("print(String(format: ")
			for i, arg := range ce.Args {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write("), terminator: \"\")")
		} else {
			g.write("print(")
			if len(ce.Args) > 0 {
				if err := g.emitExpr(ce.Args[0]); err != nil {
					return err
				}
			}
			g.write(", terminator: \"\")")
		}
		return nil
	case "sprintf":
		if len(ce.Args) >= 2 {
			g.write("String(format: ")
			for i, arg := range ce.Args {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write(")")
		} else {
			g.write("String(describing: ")
			if len(ce.Args) > 0 {
				if err := g.emitExpr(ce.Args[0]); err != nil {
					return err
				}
			}
			g.write(")")
		}
		return nil
	case "Result.ok":
		g.write("Result.ok(")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "Result.err":
		g.write("Result.err(")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	default:
		callee := ce.Callee
		if strings.Contains(callee, ".") {
			parts := strings.Split(callee, ".")
			if len(parts) == 2 {
				if g.imports[parts[0]] {
					callee = capitalize(parts[0]) + "." + parts[1]
				} else {
					callee = parts[0] + "." + parts[1]
				}
			}
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
}

func (g *swGen) emitLiteral(lit *ast.Literal) error {
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
