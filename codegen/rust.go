package codegen

import (
	"fmt"
	"path/filepath"
	"strings"

	"xiaoqinli/ast"
)

// GenerateRust produces Rust source code from the given typed AST.
func GenerateRust(root ast.Node) ([]byte, error) {
	g := &rsGen{
		buf:         &strings.Builder{},
		funcReturns: make(map[string]string),
		imports:     make(map[string]bool),
		structTable: make(map[string]*ast.StructDecl),
	}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	hasMain := false
	for _, d := range prog.Decls {
		if fd, ok := d.(*ast.FunctionDecl); ok {
			g.funcReturns[fd.Name] = fd.ReturnType.KindName
			if fd.Name == "main" {
				hasMain = true
			}
		}
		if id, ok := d.(*ast.ImportDecl); ok {
			g.imports[id.As] = true
		}
		if sd, ok := d.(*ast.StructDecl); ok {
			g.structTable[sd.Name] = sd
		}
	}
	g.isMain = hasMain

	for i, d := range prog.Decls {
		if i > 0 {
			g.writeln("")
		}
		if err := g.emitNode(d); err != nil {
			return nil, err
		}
	}

	return []byte(g.buf.String()), nil
}

type rsGen struct {
	buf         *strings.Builder
	indent      int
	muts        map[string]bool
	isMain      bool
	scope       map[string]string // variable/param name → type kind
	funcReturns map[string]string // function name → return type kind
	currentFunc *ast.FunctionDecl
	imports     map[string]bool // import alias set
	structTable map[string]*ast.StructDecl
}

func (g *rsGen) write(s string)   { g.buf.WriteString(s) }
func (g *rsGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *rsGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func typeToRust(t ast.TypeExpr) string {
	return typeToRustInner(t, false)
}

func typeToRustParam(t ast.TypeExpr) string {
	return typeToRustInner(t, true)
}

func typeToRustInner(t ast.TypeExpr, param bool) string {
	switch t.KindName {
	case "Int":
		return "i64"
	case "Float":
		return "f64"
	case "String":
		if param {
			return "&str"
		}
		return "String"
	case "Bool":
		return "bool"
	case "Void":
		return ""
	case "Array":
		if t.Elem != nil {
			return "Vec<" + typeToRust(*t.Elem) + ">"
		}
		return "Vec<String>"
	case "Option":
		if t.Elem != nil {
			return "Option<" + typeToRust(*t.Elem) + ">"
		}
		return "Option<String>"
	case "Result":
		ok := "String"
		if t.OkType != nil {
			ok = typeToRust(*t.OkType)
		}
		errT := "Box<dyn std::error::Error>"
		if t.ErrType != nil {
			errT = typeToRust(*t.ErrType)
		}
		return "Result<" + ok + ", " + errT + ">"
	case "Map":
		kt := "String"
		vt := "String"
		if t.KeyType != nil {
			kt = typeToRust(*t.KeyType)
		}
		if t.Elem != nil {
			vt = typeToRust(*t.Elem)
		}
		return "std::collections::HashMap<" + kt + ", " + vt + ">"
	default:
		name := t.KindName
		if strings.Contains(name, ".") {
			name = strings.ReplaceAll(name, ".", "::")
		}
		return name
	}
}

// --- Node emitters ---

func (g *rsGen) emitNode(n ast.Node) error {
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
	case *ast.ClassDecl:
		return g.emitClassDecl(node)
	case *ast.EnumDecl:
		return g.emitEnumDecl(node)
	case *ast.MatchExpr:
		return g.emitMatchExpr(node)
	case *ast.SwitchStmt:
		return g.emitSwitchStmt(node)
	case *ast.ImportDecl:
		return g.emitImportDecl(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *rsGen) emitImportDecl(id *ast.ImportDecl) error {
	path := id.Path
	base := filepath.Base(path)
	if strings.HasSuffix(base, ".xql.json") {
		base = strings.TrimSuffix(base, ".xql.json")
	} else if strings.HasSuffix(base, ".xql") {
		base = strings.TrimSuffix(base, ".xql")
	}
	g.writeIndent()
	if g.isMain {
		g.writeln(fmt.Sprintf("pub mod %s;", base))
	} else {
		g.writeln(fmt.Sprintf("use crate::%s;", base))
	}
	return nil
}

func (g *rsGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeIndent()
	g.writeln("enum " + ed.Name + " {")
	g.indent++
	for _, v := range ed.Variants {
		g.writeIndent()
		g.writeln(v + ",")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *rsGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("match ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(" {")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.write("_")
		} else {
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
		}
		g.writeln(" => {")
		g.indent++
		for _, stmt := range arm.Body {
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

func (g *rsGen) emitStructDecl(sd *ast.StructDecl) error {
	g.writeIndent()
	g.writeln("#[derive(Debug, Clone)]")
	g.writeIndent()
	g.writeln("pub struct " + sd.Name + " {")
	g.indent++
	for _, f := range sd.Fields {
		g.writeIndent()
		prefix := ""
		if f.Visibility != "private" {
			prefix = "pub "
		}
		g.writeln(prefix + f.Name + ": " + typeToRust(f.Type) + ",")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *rsGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.muts = collectMutables(fd.Body)
	g.currentFunc = fd
	g.scope = make(map[string]string)
	for _, p := range fd.Params {
		g.scope[p.Name] = p.Type.KindName
	}

	g.writeIndent()
	g.write("pub fn " + fd.Name + "(")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + ": " + typeToRustParam(p.Type))
	}
	g.write(")")

	rt := typeToRust(fd.ReturnType)
	if rt != "" {
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

func (g *rsGen) emitReturn(rs *ast.ReturnStmt) error {
	g.writeIndent()
	if rs.Value == nil {
		g.writeln("return;")
		return nil
	}
	g.write("return ")
	needsOwned := g.currentFunc != nil && g.currentFunc.ReturnType.KindName == "String"
	if needsOwned {
		if err := g.emitOwnedExpr(rs.Value); err != nil {
			return err
		}
	} else if err := g.emitExpr(rs.Value); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

func (g *rsGen) emitVarDecl(vd *ast.VarDecl) error {
	if g.scope != nil {
		g.scope[vd.Name] = vd.Type.KindName
	}
	g.writeIndent()
	if g.muts[vd.Name] {
		g.write("let mut ")
	} else {
		g.write("let ")
	}
	g.write(vd.Name)
	rt := typeToRust(vd.Type)
	if rt != "" {
		g.write(": " + rt)
	}
	if vd.Value != nil {
		g.write(" = ")
		if vd.Type.KindName == "String" {
			if err := g.emitOwnedExpr(vd.Value); err != nil {
				return err
			}
		} else if err := g.emitExpr(vd.Value); err != nil {
			return err
		}
	}
	g.writeln(";")
	return nil
}

func (g *rsGen) emitAssign(as *ast.AssignStmt) error {
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

func (g *rsGen) emitIf(is *ast.IfStmt) error {
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

func (g *rsGen) emitWhile(ws *ast.WhileStmt) error {
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

func (g *rsGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		// for i in start..=end {
		g.write("for " + fs.Var + " in ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("..=")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
	case "each":
		// for v in &iterable {
		g.write("for " + fs.Var + " in &")
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

func (g *rsGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln(";")
	return nil
}

// exprIsStrLiteral checks if the top-level expression is a bare string literal (&str).
func exprIsStrLiteral(n ast.Node) bool {
	if lit, ok := n.(*ast.Literal); ok {
		return lit.ValueType == "String"
	}
	return false
}

// emitOwnedExpr emits an expression, converting &str literals to owned String via .to_string().
func (g *rsGen) emitOwnedExpr(n ast.Node) error {
	if exprIsStrLiteral(n) {
		if err := g.emitExpr(n); err != nil {
			return err
		}
		g.write(".to_string()")
		return nil
	}
	return g.emitExpr(n)
}

// isStrRef checks if an expression is already a &str reference (e.g., a function parameter).
func (g *rsGen) isStrRef(n ast.Node) bool {
	if ident, ok := n.(*ast.Ident); ok {
		if g.currentFunc != nil {
			for _, p := range g.currentFunc.Params {
				if p.Name == ident.Name && p.Type.KindName == "String" {
					return true // param String → &str, already a reference
				}
			}
		}
	}
	return false
}

// --- Expression emitters ---

func (g *rsGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		isStrConcat := node.Op == "+" && (containsStringExpr(node.Left) || containsStringExpr(node.Right))
		g.write("(")
		if isStrConcat {
			if err := g.emitOwnedExpr(node.Left); err != nil {
				return err
			}
		} else if err := g.emitExpr(node.Left); err != nil {
			return err
		}
		g.write(" " + node.Op + " ")
		if isStrConcat && !g.isStrRef(node.Right) {
			g.write("&")
		}
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
		if node.Field == "isOk" {
			if err := g.emitExpr(node.Object); err != nil {
				return err
			}
			g.write(".is_ok()")
			return nil
		} else if node.Field == "isErr" {
			if err := g.emitExpr(node.Object); err != nil {
				return err
			}
			g.write(".is_err()")
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

func (g *rsGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("vec![")
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

func (g *rsGen) emitIndexExpr(ie *ast.IndexExpr) error {
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write("[")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write(" as usize]")
	return nil
}

func (g *rsGen) emitStructLit(sl *ast.StructLit) error {
	typeName := sl.TypeName
	if strings.Contains(typeName, ".") {
		typeName = strings.ReplaceAll(typeName, ".", "::")
	}
	g.write(typeName + " { ")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + ": ")

		isStringLit := false
		if lit, ok := f.Value.(*ast.Literal); ok && lit.ValueType == "String" {
			isStringLit = true
		}
		if isStringLit {
			if err := g.emitOwnedExpr(f.Value); err != nil {
				return err
			}
		} else {
			if err := g.emitExpr(f.Value); err != nil {
				return err
			}
		}
	}
	g.write(" }")
	return nil
}

func (g *rsGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		return g.emitRustPrint("println!", ce.Args)
	case "printf":
		return g.emitRustPrint("print!", ce.Args)
	case "sprintf":
		return g.emitRustPrint("format!", ce.Args)
	case "Result.ok":
		g.write("Ok(")
		if len(ce.Args) > 0 {
			isStringLit := false
			if lit, ok := ce.Args[0].(*ast.Literal); ok && lit.ValueType == "String" {
				isStringLit = true
			}
			if isStringLit {
				if err := g.emitOwnedExpr(ce.Args[0]); err != nil {
					return err
				}
			} else {
				if err := g.emitExpr(ce.Args[0]); err != nil {
					return err
				}
			}
		}
		g.write(")")
		return nil
	case "Result.err":
		g.write("Err(")
		if len(ce.Args) > 0 {
			isStringLit := false
			if lit, ok := ce.Args[0].(*ast.Literal); ok && lit.ValueType == "String" {
				isStringLit = true
			}
			if isStringLit {
				if err := g.emitOwnedExpr(ce.Args[0]); err != nil {
					return err
				}
			} else {
				if err := g.emitExpr(ce.Args[0]); err != nil {
					return err
				}
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
					callee = parts[0] + "::" + parts[1]
				} else {
					obj := parts[0]
					method := parts[1]
					if method == "unwrapErr" {
						callee = obj + ".unwrap_err"
					}
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

func (g *rsGen) emitRustPrint(macro string, args []ast.Node) error {
	g.write(macro + "(\"")
	for i := range args {
		if i > 0 {
			g.write(" ")
		}
		g.write("{}")
	}
	g.write("\"")
	for _, arg := range args {
		g.write(", ")
		if err := g.emitExpr(arg); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *rsGen) emitLiteral(lit *ast.Literal) error {
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

func (g *rsGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("if ")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" { ")
	if err := g.emitOwnedExpr(ie.Then); err != nil {
		return err
	}
	g.write(" } else { ")
	if err := g.emitOwnedExpr(ie.Else); err != nil {
		return err
	}
	g.write(" }")
	return nil
}

func (g *rsGen) emitLambda(lam *ast.Lambda) error {
	g.write("|")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(", ")
		}
		g.write(p.Name + ": " + typeToRustParam(p.Type))
	}
	g.write("|")
	retType := typeToRust(lam.ReturnType)
	if retType != "" {
		g.write(" -> " + retType)
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

func (g *rsGen) emitClassDecl(cd *ast.ClassDecl) error {
	g.writeIndent()
	g.writeln("#[derive(Debug, Clone)]")
	g.writeIndent()
	g.writeln("struct " + cd.Name + " {")
	g.indent++
	for _, f := range cd.Fields {
		g.writeIndent()
		prefix := ""
		if f.Visibility == "public" {
			prefix = "pub "
		}
		g.writeln(prefix + f.Name + ": " + typeToRust(f.Type) + ",")
	}
	g.indent--
	g.writeIndent()
	g.writeln("}")
	return nil
}

func (g *rsGen) emitSwitchStmt(ss *ast.SwitchStmt) error {
	g.writeIndent()
	g.write("match ")
	if err := g.emitExpr(ss.Value); err != nil {
		return err
	}
	g.writeln(" {")
	g.indent++
	for _, c := range ss.Cases {
		g.writeIndent()
		if c.Value != nil {
			if err := g.emitExpr(c.Value); err != nil {
				return err
			}
		} else {
			g.write("_")
		}
		g.writeln(" => {")
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

func (g *rsGen) emitArrayLiteral(al *ast.ArrayLiteral) error {
	g.write("vec![")
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

func (g *rsGen) emitMapLiteral(ml *ast.MapLiteral) error {
	g.write("std::collections::HashMap::from([")
	for i, entry := range ml.Entries {
		if i > 0 {
			g.write(", ")
		}
		g.write("(")
		if err := g.emitExpr(entry.Key); err != nil {
			return err
		}
		g.write(", ")
		if err := g.emitExpr(entry.Value); err != nil {
			return err
		}
		g.write(")")
	}
	g.write("])")
	return nil
}
