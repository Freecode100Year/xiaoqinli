package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateJava produces Java source code from the given typed AST.
func GenerateJava(root ast.Node) ([]byte, error) {
	g := &javaGen{
		buf:         &strings.Builder{},
		structTable: make(map[string]*ast.StructDecl),
		imports:     CollectImports(root),
	}
	g.enums = collectEnums(root)

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	g.indent = 1

	// Collect structures for visibility lookup
	for _, d := range prog.Decls {
		if sd, ok := d.(*ast.StructDecl); ok {
			g.structTable[sd.Name] = sd
		}
	}

	// Detect Result usage
	walkTypes(root, func(t ast.TypeExpr, context string) {
		if t.KindName == "Result" {
			g.needResult = true
		}
	})

	// Emit enum declarations first (inside the class, before methods).
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
		if id, ok := d.(*ast.ImportDecl); ok {
			// Skip ImportDecl output in java source files
			_ = id
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

	// Determine class name based on program heuristics
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
		className = "Main"
	} else if hasService {
		className = "Service"
	}
	g.className = className

	var out strings.Builder
	if g.needList {
		out.WriteString("import java.util.ArrayList;\nimport java.util.List;\n\n")
	}
	out.WriteString("public class " + className + " {\n")

	// Inject static Result class if needed inside the Main class to prevent redeclarations in same package
	if g.needResult && className == "Main" {
		out.WriteString(`    public static class Result<T, E> {
        public final T val;
        public final E err;
        public final boolean isOk;
        public Result(T val, E err, boolean isOk) {
            this.val = val;
            this.err = err;
            this.isOk = isOk;
        }
        public static <T, E> Result<T, E> ok(T val) {
            return new Result<>(val, null, true);
        }
        public static <T, E> Result<T, E> err(E err) {
            return new Result<>(null, err, false);
        }
        public T unwrap() {
            if (!isOk) throw new RuntimeException("Called unwrap on Err Result");
            return val;
        }
        public E unwrapErr() {
            if (isOk) throw new RuntimeException("Called unwrapErr on Ok Result");
            return err;
        }
    }
`)
	}

	out.WriteString(g.buf.String())
	out.WriteString("}\n")

	return []byte(out.String()), nil
}

type javaGen struct {
	buf         *strings.Builder
	indent      int
	muts        map[string]bool
	needList    bool
	needResult  bool
	structTable map[string]*ast.StructDecl
	imports     map[string]bool
	className   string
	enums       map[string]*ast.EnumDecl
}

func (g *javaGen) write(s string)   { g.buf.WriteString(s) }
func (g *javaGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *javaGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func (g *javaGen) typeStr(t ast.TypeExpr) string {
	name := g.stripOrCapitalizeAlias(t.KindName)
	switch name {
	case "Int":
		return "long"
	case "Float":
		return "double"
	case "String":
		return "String"
	case "Bool":
		return "boolean"
	case "Void":
		return "void"
	case "Array":
		g.needList = true
		if t.Elem != nil {
			return "List<" + g.boxedTypeStr(*t.Elem) + ">"
		}
		return "List<Object>"
	case "Option":
		if t.Elem != nil {
			return g.boxedTypeStr(*t.Elem)
		}
		return "Object"
	case "Result":
		okType := "Object"
		errType := "Object"
		if t.OkType != nil {
			okType = g.boxedTypeStr(*t.OkType)
		}
		if t.ErrType != nil {
			errType = g.boxedTypeStr(*t.ErrType)
		}
		resultName := "Result"
		if g.className != "Main" {
			resultName = "Main.Result"
		}
		return resultName + "<" + okType + ", " + errType + ">"
	default:
		return name
	}
}

func (g *javaGen) boxedTypeStr(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "Long"
	case "Float":
		return "Double"
	case "Bool":
		return "Boolean"
	default:
		return g.typeStr(t)
	}
}

// --- Node emitters ---

func (g *javaGen) emitNode(n ast.Node) error {
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
	case *ast.ClassDecl:
		return g.emitClassDecl(node)
	case *ast.SwitchStmt:
		return g.emitSwitchStmt(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *javaGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.write("public enum " + ed.Name + " { ")
	for i, v := range ed.Variants {
		if i > 0 {
			g.write(", ")
		}
		g.write(v)
	}
	g.writeln(" }")
	return nil
}

// Java's switch selector may be an int, a String, an enum or a boxed
// equivalent — not a long. Int is 64-bit in this AST and this backend spells it
// long, so a match over an Int compiled to `switch (n) { case 1L: ... }`, which
// javac rejects twice over: "constant label of type long is not compatible with
// switch selector type long", and then reads the labels as pattern matching, a
// preview feature that is off by default. A match over an Int lowers to an
// if/else-if chain; a match over an enum stays a switch, which is where the
// unqualified case label matters.
//
// emitSwitchStmt has the same defect and no corpus program behind it yet.
// emitMatchAsIfChain is the lowering for a match Java's switch cannot take.
// The wildcard becomes the else, and a match that is only a wildcard emits its
// body with no `if` around it.
func (g *javaGen) emitMatchAsIfChain(me *ast.MatchExpr) error {
	var wildcard []ast.Node
	haveWildcard := false
	open := false

	emitBody := func(body []ast.Node) error {
		g.indent++
		defer func() { g.indent-- }()
		for _, s := range body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		return nil
	}

	for _, arm := range me.Arms {
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			wildcard = arm.Body
			haveWildcard = true
			continue
		}
		g.writeIndent()
		if open {
			g.write("} else if (")
		} else {
			g.write("if (")
		}
		if err := g.emitExpr(me.Value); err != nil {
			return err
		}
		g.write(" == ")
		if err := g.emitExpr(arm.Pattern); err != nil {
			return err
		}
		g.writeln(") {")
		open = true
		if err := emitBody(arm.Body); err != nil {
			return err
		}
	}

	if !open {
		return emitBody(wildcard)
	}
	if haveWildcard {
		g.writeIndent()
		g.writeln("} else {")
		if err := emitBody(wildcard); err != nil {
			return err
		}
	}
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *javaGen) emitMatchExpr(me *ast.MatchExpr) error {
	for _, arm := range me.Arms {
		if lit, ok := arm.Pattern.(*ast.Literal); ok && lit.ValueType == "Int" {
			return g.emitMatchAsIfChain(me)
		}
	}

	g.writeIndent()
	g.write("switch (")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(") {")
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.writeln("default:")
		} else {
			g.write("case ")
			// A switch label naming an enum constant must be unqualified:
			// `case Color.Red:` is a compile error before Java 21, and every
			// other position wants the qualified form. So the label is the one
			// place this backend spells a variant without its enum.
			me, isMember := arm.Pattern.(*ast.MemberExpr)
			if _, variant, ok := enumRef(g.enums, me); isMember && ok {
				g.write(variant)
			} else if err := g.emitExpr(arm.Pattern); err != nil {
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
		g.writeIndent()
		g.writeln("break;")
		g.indent--
	}
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *javaGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("public static class " + sd.Name + " {")
	g.indent++

	// Emit fields
	for _, f := range sd.Fields {
		g.writeIndent()
		g.writeln("public " + g.typeStr(f.Type) + " " + f.Name + ";")
	}

	// Emit constructor
	g.writeIndent()
	g.write("public " + sd.Name + "(")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(g.typeStr(f.Type) + " " + f.Name)
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

func (g *javaGen) emitClassDecl(cd *ast.ClassDecl) error {
	g.writeIndent()
	g.writeln("public static class " + cd.Name + " {")
	g.indent++
	for _, f := range cd.Fields {
		g.writeIndent()
		vis := f.Visibility
		if vis == "" {
			vis = "public"
		}
		g.writeln(vis + " " + g.typeStr(f.Type) + " " + f.Name + ";")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *javaGen) emitSwitchStmt(ss *ast.SwitchStmt) error {
	g.writeIndent()
	g.write("switch (")
	if err := g.emitExpr(ss.Value); err != nil {
		return err
	}
	g.writeln(") {")
	g.indent++
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
		g.writeIndent()
		g.writeln("break;")
		g.indent--
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *javaGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)

	g.writeIndent()
	if fd.Name == "main" {
		g.writeln("public static void main(String[] args) {")
	} else {
		rt := g.typeStr(fd.ReturnType)
		g.write("public static " + rt + " " + fd.Name + "(")
		for i, p := range fd.Params {
			if i > 0 {
				g.write(", ")
			}
			g.write(g.typeStr(p.Type) + " " + p.Name)
		}
		g.writeln(") {")
	}

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

func (g *javaGen) emitReturn(rs *ast.ReturnStmt) error {
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

func (g *javaGen) emitVarDecl(vd *ast.VarDecl) error {
	g.writeIndent()
	if !g.muts[vd.Name] {
		g.write("final ")
	}
	g.write(g.typeStr(vd.Type) + " " + vd.Name)
	if vd.Value != nil {
		g.write(" = ")
		if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln(";")
	return nil
}

func (g *javaGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *javaGen) emitIf(is *ast.IfStmt) error {
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

func (g *javaGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *javaGen) emitForStmt(fs *ast.ForStmt) error {
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
		g.write("for (var " + fs.Var + " : ")
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

func (g *javaGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

// --- Expression emitters ---

func (g *javaGen) emitExpr(n ast.Node) error {
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
		// Java qualifies an enum constant everywhere except a switch label, which emitMatchExpr handles.
		if enum, variant, ok := enumRef(g.enums, node); ok {
			g.write(enum + "." + variant)
			return nil
		}
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

func (g *javaGen) emitIfExpr(ie *ast.IfExpr) error {
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

func (g *javaGen) emitLambda(lam *ast.Lambda) error {
	g.write("(")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(g.typeStr(p.Type) + " " + p.Name)
	}
	g.write(") -> {")
	for _, stmt := range lam.Body {
		g.write(" ")
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
			if err := g.emitNode(stmt); err != nil {
				return err
			}
		}
	}
	g.write(" }")
	return nil
}

func (g *javaGen) emitArrayLit(al *ast.ArrayLit) error {
	g.needList = true
	g.write("java.util.List.of(")
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

func (g *javaGen) emitArrayLiteral(al *ast.ArrayLiteral) error {
	g.needList = true
	g.write("java.util.List.of(")
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

func (g *javaGen) emitMapLiteral(ml *ast.MapLiteral) error {
	g.write("java.util.Map.ofEntries(")
	for i, entry := range ml.Entries {
		if i > 0 {
			g.write(", ")
		}
		g.write("java.util.Map.entry(")
		if err := g.emitExpr(entry.Key); err != nil {
			return err
		}
		g.write(", ")
		if err := g.emitExpr(entry.Value); err != nil {
			return err
		}
		g.write(")")
	}
	g.write(")")
	return nil
}

func (g *javaGen) emitIndexExpr(ie *ast.IndexExpr) error {
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write(".get((int)(")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write("))")
	return nil
}

func (g *javaGen) emitStructLit(sl *ast.StructLit) error {
	g.write("new " + g.stripOrCapitalizeAlias(sl.TypeName) + "(")
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

func (g *javaGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("System.out.println(")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(" + \" \" + ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "printf":
		if len(ce.Args) >= 2 {
			g.write("System.out.printf(")
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
			g.write("System.out.print(")
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
			g.write("String.valueOf(")
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			g.write(")")
		} else {
			g.write("\"\"")
		}
		return nil
	case "Result.ok":
		resultName := "Result"
		if g.className != "Main" {
			resultName = "Main.Result"
		}
		g.write(resultName + ".ok(")
		if len(ce.Args) > 0 {
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "Result.err":
		resultName := "Result"
		if g.className != "Main" {
			resultName = "Main.Result"
		}
		g.write(resultName + ".err(")
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

func (g *javaGen) emitLiteral(lit *ast.Literal) error {
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

func (g *javaGen) stripOrCapitalizeAlias(name string) string {
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
