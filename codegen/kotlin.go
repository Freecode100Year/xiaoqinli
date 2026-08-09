package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateKotlin produces Kotlin source code from the given typed AST.
func GenerateKotlin(root ast.Node) ([]byte, error) {
	g := &ktGen{
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

	// Determine package name
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
	packageName := "models"
	if hasMain {
		packageName = "main"
	} else if hasService {
		packageName = "service"
	}
	g.packageName = packageName

	// Emit package declaration
	g.writeln("package " + packageName)
	g.writeln("")

	// Emit imports
	for imp := range g.imports {
		g.writeln("import " + imp + ".*")
	}
	if packageName != "main" && g.needResult {
		g.writeln("import main.Result")
	}
	g.writeln("")

	// Inject custom Result class at main package top-level
	if g.needResult && packageName == "main" {
		g.writeln(kotlinResultClass)
		g.writeln("")
	}

	// Emit enum declarations first.
	first := true
	for _, d := range prog.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			if !first {
				g.writeln("")
			}
			if err := g.emitEnumDecl(ed); err != nil {
				return nil, err
			}
			first = false
		}
	}

	for _, d := range prog.Decls {
		if _, ok := d.(*ast.EnumDecl); ok {
			continue
		}
		if _, ok := d.(*ast.ImportDecl); ok {
			continue
		}
		if !first {
			g.writeln("")
		}
		if err := g.emitNode(d); err != nil {
			return nil, err
		}
		first = false
	}

	return []byte(g.buf.String()), nil
}

type ktGen struct {
	buf         *strings.Builder
	indent      int
	muts        map[string]bool
	needResult  bool
	packageName string
	imports     map[string]bool
}

func (g *ktGen) write(s string)   { g.buf.WriteString(s) }
func (g *ktGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *ktGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

// kotlinResultClass is XQL's own Result, emitted by both the kotlin and the
// android backends. Kotlin's stdlib ships a single-parameter kotlin.Result<out
// T> in every file's default imports, so a two-parameter Result<T, E> only
// resolves if the generated file declares one in its own package.
const kotlinResultClass = `class Result<out T, out E> private constructor(
    val val_: T?,
    val err: E?,
    val isOk: Boolean
) {
    companion object {
        fun <T, E> ok(v: T): Result<T, E> = Result(v, null, true)
        fun <T, E> err(e: E): Result<T, E> = Result(null, e, false)
    }
    fun unwrap(): T {
        if (!isOk) throw RuntimeException("Called unwrap on Err Result")
        return val_!!
    }
    fun unwrapErr(): E {
        if (isOk) throw RuntimeException("Called unwrapErr on Ok Result")
        return err!!
    }
}`

func typeToKotlin(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "Long"
	case "Float":
		return "Double"
	case "String":
		return "String"
	case "Bool":
		return "Boolean"
	case "Void":
		return "Unit"
	case "Array":
		if t.Elem != nil {
			return "List<" + typeToKotlin(*t.Elem) + ">"
		}
		return "List<Any>"
	case "Option":
		if t.Elem != nil {
			return typeToKotlin(*t.Elem) + "?"
		}
		return "Any?"
	case "Result":
		// Kotlin compiler maps main.Result automatically due to our imports
		okType := "Any"
		errType := "Any"
		if t.OkType != nil {
			okType = typeToKotlin(*t.OkType)
		}
		if t.ErrType != nil {
			errType = typeToKotlin(*t.ErrType)
		}
		return "Result<" + okType + ", " + errType + ">"
	default:
		return t.KindName
	}
}

func (g *ktGen) defaultValue(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "0L"
	case "Float":
		return "0.0"
	case "Bool":
		return "false"
	case "String":
		return `""`
	case "Array":
		return "emptyList()"
	default:
		return "null"
	}
}

// --- Node emitters ---

func (g *ktGen) emitNode(n ast.Node) error {
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
	case *ast.ClassDecl:
		return g.emitClassDecl(node)
	case *ast.SwitchStmt:
		return g.emitSwitchStmt(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *ktGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.write("enum class " + ed.Name + " { ")
	for i, v := range ed.Variants {
		if i > 0 {
			g.write(", ")
		}
		g.write(v)
	}
	g.writeln(" }")
	return nil
}

func (g *ktGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("when (")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(") {")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.write("else")
		} else {
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
		}
		g.writeln(" -> {")
		g.indent++
		for _, s := range arm.Body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln("}")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *ktGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.write("data class " + sd.Name + "(")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write("val " + f.Name + ": " + typeToKotlin(f.Type))
	}
	g.writeln(")")
	return nil
}

func (g *ktGen) emitClassDecl(cd *ast.ClassDecl) error {
	g.writeIndent()
	g.writeln("public class " + cd.Name + " {")
	g.indent++
	for _, f := range cd.Fields {
		g.writeIndent()
		vis := f.Visibility
		if vis == "" {
			vis = "public"
		}
		g.writeln(vis + " var " + f.Name + ": " + typeToKotlin(f.Type) + " = " + g.defaultValue(f.Type))
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *ktGen) emitSwitchStmt(ss *ast.SwitchStmt) error {
	g.writeIndent()
	g.write("when (")
	if err := g.emitExpr(ss.Value); err != nil {
		return err
	}
	g.writeln(") {")
	g.indent++
	for _, c := range ss.Cases {
		g.writeIndent()
		if c.Value != nil {
			if err := g.emitExpr(c.Value); err != nil {
				return err
			}
		} else {
			g.write("else")
		}
		g.writeln(" -> {")
		g.indent++
		for _, stmt := range c.Body {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln("}")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *ktGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)

	g.writeIndent()
	g.write("fun " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + ": " + typeToKotlin(p.Type))
	}
	g.write(")")

	rt := typeToKotlin(fd.ReturnType)
	if rt != "" && rt != "Unit" {
		g.write(": " + rt)
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

func (g *ktGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *ktGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if g.muts[vd.Name] {
		g.write("var ")
	} else {
		g.write("val ")
	}
	g.write(vd.Name + ": " + typeToKotlin(vd.Type))
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln("")
	return nil
}

func (g *ktGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *ktGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if ")
	if err := g.emitCondition(is.Cond); err != nil {
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

func (g *ktGen) emitWhile(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("while ")
	if err := g.emitCondition(ws.Cond); err != nil {
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

func (g *ktGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	if fs.Form == "range" {
		g.write("for (" + fs.Var + " in ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write(" <= ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(") {")
	} else {
		g.write("for (" + fs.Var + " in ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(") {")
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

func (g *ktGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

// --- Expression emitters ---

func (g *ktGen) emitExpr(n ast.Node) error {
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

func (g *ktGen) emitCondition(cond ast.Node) error {
	if _, ok := cond.(*ast.BinaryExpr); ok {
		return g.emitExpr(cond)
	}
	g.write("(")
	if err := g.emitExpr(cond); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *ktGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("if ")
	if err := g.emitCondition(ie.Cond); err != nil {
		return err
	}
	g.write(" ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" else ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	return nil
}

func (g *ktGen) emitLambda(lam *ast.Lambda) error {
	g.write("{ ")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + ": " + typeToKotlin(p.Type))
	}
	if len(lam.Params) > 0 {
		g.write(" -> ")
	}
	for i, stmt := range lam.Body {
		if i > 0 {
			g.write("; ")
		}
		if es, ok := stmt.(*ast.ExprStmt); ok {
			if err := g.emitExpr(es.Expr); err != nil {
				return err
			}
		} else if rs, ok := stmt.(*ast.ReturnStmt); ok && rs.Value != nil {
			if err := g.emitExpr(rs.Value); err != nil {
				return err
			}
		} else {
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
	}
	g.write(" }")
	return nil
}

func (g *ktGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("listOf(")
	for i, elem := range al.Elements {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(elem); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *ktGen) emitArrayLiteral(al *ast.ArrayLiteral) error {
	g.write("listOf(")
	for i, elem := range al.Elements {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(elem); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *ktGen) emitMapLiteral(ml *ast.MapLiteral) error {
	g.write("mapOf(")
	for i, entry := range ml.Entries {
		if i > 0 {
			g.write(", ")
		}
		if err := g.emitExpr(entry.Key); err != nil {
			return err
		}
		g.write(" to ")
		if err := g.emitExpr(entry.Value); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *ktGen) emitIndexExpr(ie *ast.IndexExpr) error {
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write("[(")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write(").toInt()]")
	return nil
}

func (g *ktGen) emitStructLit(sl *ast.StructLit) error {
	// Class name mapping for type checking
	g.write(typeToKotlin(ast.TypeExpr{KindName: sl.TypeName}) + "(")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + " = ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *ktGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("println(")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(` + " " + `)
			}
			if i == 0 && len(ce.Args) > 1 {
				g.write(`"" + `)
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "printf":
		if len(ce.Args) >= 2 {
			g.write("print(String.format(")
			for i, arg := range ce.Args {
				if i > 0 {
					g.write(", ")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
			}
			g.write("))")
		} else {
			g.write("print(")
			if len(ce.Args) > 0 {
				if err := g.emitExpr(ce.Args[0]); err != nil {
					return err
				}
			}
			g.write(")")
		}
		return nil
	case "sprintf":
		if len(ce.Args) >= 2 {
			g.write("String.format(")
			for i, arg := range ce.Args {
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
			g.write(".toString()")
		} else {
			g.write("\"\"")
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
		// Map type alias package name if any
		callee := ce.Callee
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

func (g *ktGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write(fmt.Sprintf("%q", s))
	case "Int":
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%dL", int64(f)))
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
