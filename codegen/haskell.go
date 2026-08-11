package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

func GenerateHaskell(root ast.Node) ([]byte, error) {
	g := &hsGen{
		buf:      &strings.Builder{},
		funcRets: make(map[string]string),
		types:    newTypeKinds(root),
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

	var structs []*ast.StructDecl
	var enums []*ast.EnumDecl
	var funcs []*ast.FunctionDecl
	for _, d := range prog.Decls {
		switch n := d.(type) {
		case *ast.StructDecl:
			structs = append(structs, n)
		case *ast.EnumDecl:
			enums = append(enums, n)
		case *ast.FunctionDecl:
			funcs = append(funcs, n)
		}
	}

	for _, ed := range enums {
		g.emitEnumDecl(ed)
		g.writeln("")
	}

	for _, sd := range structs {
		g.emitStructDecl(sd)
		g.writeln("")
	}

	for i, fd := range funcs {
		if i > 0 {
			g.writeln("")
		}
		if err := g.emitFunctionDecl(fd); err != nil {
			return nil, err
		}
	}

	var out strings.Builder
	out.WriteString("module Main where\n\n")

	if g.needIORef {
		out.WriteString("import Data.IORef\n")
	}
	if g.needPrintf {
		out.WriteString("import Text.Printf\n")
	}
	if g.needIORef || g.needPrintf {
		out.WriteString("\n")
	}

	out.WriteString(g.buf.String())
	return []byte(out.String()), nil
}

type hsGen struct {
	types      *typeKinds
	buf        *strings.Builder
	indent     int
	funcRets   map[string]string
	needIORef  bool
	needPrintf bool
	inIO       bool
	mutables   map[string]bool
	loopCount  int
}

func (g *hsGen) write(s string)   { g.buf.WriteString(s) }
func (g *hsGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *hsGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("    ")
	}
}

func typeToHaskell(t ast.TypeExpr) string {
	switch t.KindName {
	case "Int":
		return "Int"
	case "Float":
		return "Double"
	case "String":
		return "String"
	case "Bool":
		return "Bool"
	case "Void":
		return "()"
	case "Array":
		if t.Elem != nil {
			return "[" + typeToHaskell(*t.Elem) + "]"
		}
		return "[Int]"
	case "Option":
		if t.Elem != nil {
			return "Maybe " + typeToHaskell(*t.Elem)
		}
		return "Maybe Int"
	default:
		return t.KindName
	}
}

func (g *hsGen) isIOFunc(fd *ast.FunctionDecl) bool {
	if fd.Name == "main" {
		return true
	}
	for _, e := range fd.Effects {
		if e == "state" || e == "network" || e == "filesystem" {
			return true
		}
	}
	return false
}

// inferTypeKind answers what kind of value an expression has, which decides
// putStrLn against print. Getting it wrong is visible: `print` goes through
// Show, so a String comes out wearing quotes.
//
// This used to be a local copy of the idea typeKinds implements, and the copy
// answered "" for every Ident — it never learned what a variable held. So
// `println(s)` on a String variable printed "big" rather than big for as long
// as the backend existed, and the shared table that could have said so was
// already built, already fed by noteVar and noteParams, and never asked.
func (g *hsGen) inferTypeKind(n ast.Node) string {
	return g.types.kindOf(n)
}

func (g *hsGen) emitStructDecl(sd *ast.StructDecl) {
	g.writeln("data " + sd.Name + " = " + sd.Name)
	g.indent++
	for i, f := range sd.Fields {
		g.writeIndent()
		prefix := "{ "
		if i > 0 {
			prefix = ", "
		}
		g.writeln(prefix + f.Name + " :: " + typeToHaskell(f.Type))
	}
	g.writeIndent()
	g.writeln("} deriving (Show)")
	g.indent--
}

func (g *hsGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.types.noteParams(fd)
	isIO := g.isIOFunc(fd)
	g.inIO = isIO
	g.mutables = collectMutables(fd.Body)
	if len(g.mutables) > 0 && isIO {
		g.needIORef = true
	}

	g.writeIndent()
	if fd.Name == "main" {
		g.writeln("main :: IO ()")
		g.writeIndent()
		g.writeln("main = do")
	} else {
		g.write(fd.Name + " :: ")
		for _, p := range fd.Params {
			g.write(typeToHaskell(p.Type) + " -> ")
		}
		rt := typeToHaskell(fd.ReturnType)
		if isIO {
			g.writeln("IO " + rt)
		} else {
			g.writeln(rt)
		}

		g.writeIndent()
		g.write(fd.Name)
		for _, p := range fd.Params {
			g.write(" " + p.Name)
		}
		if isIO {
			g.writeln(" = do")
		} else if len(fd.Body) == 1 {
			g.write(" = ")
			return g.emitPureExprBody(fd.Body[0])
		} else {
			g.writeln(" =")
		}
	}

	g.indent++
	for _, stmt := range fd.Body {
		if err := g.emitStmt(stmt); err != nil {
			return err
		}
	}
	g.indent--

	g.inIO = false
	return nil
}

func (g *hsGen) emitPureExprBody(n ast.Node) error {
	if rs, ok := n.(*ast.ReturnStmt); ok && rs.Value != nil {
		if err := g.emitExpr(rs.Value); err != nil {
			return err
		}
		g.writeln("")
		return nil
	}
	g.writeln("")
	g.indent++
	err := g.emitStmt(n)
	g.indent--
	return err
}

func (g *hsGen) emitStmt(n ast.Node) error {
	switch node := n.(type) {
	case *ast.ReturnStmt:
		if g.inIO && len(g.mutables) > 0 && node.Value != nil {
			g.preReadMutables(node.Value)
		}
		g.writeIndent()
		if node.Value == nil {
			if g.inIO {
				g.writeln("return ()")
			} else {
				g.writeln("()")
			}
			return nil
		}
		if g.inIO {
			g.write("return ")
		}
		if err := g.emitExpr(node.Value); err != nil {
			return err
		}
		g.writeln("")
		return nil
	case *ast.VarDecl:
		g.types.noteVar(node)
		if g.inIO && g.mutables[node.Name] {
			g.writeIndent()
			g.write(node.Name + "Ref <- newIORef ")
			if node.Value != nil {
				if err := g.emitExpr(node.Value); err != nil {
					return err
				}
			} else {
				g.write("undefined")
			}
			g.writeln("")
		} else {
			if g.inIO && len(g.mutables) > 0 && node.Value != nil {
				g.preReadMutables(node.Value)
			}
			g.writeIndent()
			g.write("let " + node.Name + " = ")
			if node.Value != nil {
				if err := g.emitExpr(node.Value); err != nil {
					return err
				}
			} else {
				g.write("undefined")
			}
			g.writeln("")
		}
		return nil
	case *ast.ExprStmt:
		if g.inIO && len(g.mutables) > 0 {
			g.preReadMutables(node.Expr)
		}
		g.writeIndent()
		if err := g.emitExpr(node.Expr); err != nil {
			return err
		}
		g.writeln("")
		return nil
	case *ast.IfStmt:
		return g.emitIf(node)
	case *ast.AssignStmt:
		if g.inIO {
			if ident, ok := node.Target.(*ast.Ident); ok && g.mutables[ident.Name] {
				g.preReadMutables(node.Value)
				g.writeIndent()
				g.write("writeIORef " + ident.Name + "Ref (")
				if err := g.emitExpr(node.Value); err != nil {
					return err
				}
				g.writeln(")")
				return nil
			}
		}
		return fmt.Errorf("XQL_E401: Haskell does not support mutable assignment")
	case *ast.WhileStmt:
		if g.inIO {
			return g.emitWhileStmt(node)
		}
		return fmt.Errorf("XQL_E401: Haskell does not support while loops in pure context")
	case *ast.ForStmt:
		return g.emitForStmt(node)
	case *ast.BreakStmt:
		return fmt.Errorf("XQL_E402: Haskell does not support break")
	case *ast.ContinueStmt:
		return fmt.Errorf("XQL_E402: Haskell does not support continue")
	case *ast.MatchExpr:
		return g.emitMatchExpr(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *hsGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("if ")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln("")
	g.indent++

	if g.inIO {
		g.writeIndent()
		g.writeln("then do")
		g.indent++
		for _, s := range is.Then {
			if err := g.emitStmt(s); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln("else do")
		g.indent++
		if len(is.Else) > 0 {
			for _, s := range is.Else {
				if err := g.emitStmt(s); err != nil {
					return err
				}
			}
		} else {
			g.writeIndent()
			g.writeln("return ()")
		}
		g.indent--
	} else {
		g.writeIndent()
		g.write("then ")
		if err := g.emitPureBranch(is.Then); err != nil {
			return err
		}
		g.writeIndent()
		g.write("else ")
		if len(is.Else) > 0 {
			if err := g.emitPureBranch(is.Else); err != nil {
				return err
			}
		} else {
			g.writeln("()")
		}
	}

	g.indent--
	return nil
}

func (g *hsGen) emitPureBranch(body []ast.Node) error {
	if len(body) == 1 {
		if rs, ok := body[0].(*ast.ReturnStmt); ok && rs.Value != nil {
			if err := g.emitExpr(rs.Value); err != nil {
				return err
			}
			g.writeln("")
			return nil
		}
	}
	g.writeln("")
	g.indent++
	lets := body[:len(body)-1]
	last := body[len(body)-1]
	for _, s := range lets {
		if vd, ok := s.(*ast.VarDecl); ok {
			g.writeIndent()
			g.write("let " + vd.Name + " = ")
			if vd.Value != nil {
				if err := g.emitExpr(vd.Value); err != nil {
					return err
				}
			} else {
				g.write("undefined")
			}
			g.writeln("")
		}
	}
	g.writeIndent()
	g.write("in ")
	if rs, ok := last.(*ast.ReturnStmt); ok && rs.Value != nil {
		if err := g.emitExpr(rs.Value); err != nil {
			return err
		}
		g.writeln("")
	} else {
		g.writeln("()")
	}
	g.indent--
	return nil
}

func (g *hsGen) emitForStmt(fs *ast.ForStmt) error {
	if fs.Form == "range" {
		g.writeIndent()
		g.write("mapM_ (\\")
		g.write(fs.Var)
		g.writeln(" -> do")
		g.indent++
		for _, s := range fs.Body {
			if err := g.emitStmt(s); err != nil {
				return err
			}
		}
		// The closing paren stays indented inside the lambda. Layout inserts a
		// statement separator before any token at the enclosing do-block's
		// column, so dedenting to it makes `)` look like a new statement and
		// GHC reports "parse error on input ')'".
		g.writeIndent()
		g.write(") [")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write("..")
		// Haskell ranges are inclusive, XQL is exclusive
		g.write("(")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.write("-1)")
		g.writeln("]")
		g.indent--
		return nil
	}
	// each form
	g.writeIndent()
	g.write("mapM_ (\\")
	g.write(fs.Var)
	g.writeln(" -> do")
	g.indent++
	for _, s := range fs.Body {
		if err := g.emitStmt(s); err != nil {
			return err
		}
	}
	// Kept indented for the same layout reason as the range form above.
	g.writeIndent()
	g.write(") ")
	if err := g.emitExpr(fs.Iterable); err != nil {
		return err
	}
	g.writeln("")
	g.indent--
	return nil
}

func (g *hsGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		return g.emitBinary(node)
	case *ast.UnaryExpr:
		return g.emitUnary(node)
	case *ast.CallExpr:
		return g.emitCall(node)
	case *ast.MemberExpr:
		g.write(node.Field + " ")
		return g.emitExpr(node.Object)
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

func (g *hsGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("(if ")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" then ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" else ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *hsGen) emitLambda(lam *ast.Lambda) error {
	g.write("(\\")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(" ")
		}
		g.write(p.Name)
	}
	g.write(" -> ")
	if len(lam.Body) == 1 {
		if rs, ok := lam.Body[0].(*ast.ReturnStmt); ok && rs.Value != nil {
			if err := g.emitExpr(rs.Value); err != nil {
				return err
			}
		} else if es, ok := lam.Body[0].(*ast.ExprStmt); ok {
			if err := g.emitExpr(es.Expr); err != nil {
				return err
			}
		} else {
			if err := g.emitExpr(lam.Body[0]); err != nil {
				return err
			}
		}
	} else {
		g.write("do ")
		for _, stmt := range lam.Body {
			if err := g.emitStmt(stmt); err != nil {
				return err
			}
		}
	}
	g.write(")")
	return nil
}

func (g *hsGen) emitBinary(be *ast.BinaryExpr) error {
	if be.Op == "+" && (g.inferTypeKind(be.Left) == "String" || g.inferTypeKind(be.Right) == "String") {
		g.write("(")
		if err := g.emitExpr(be.Left); err != nil {
			return err
		}
		g.write(" ++ ")
		if err := g.emitExpr(be.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	}
	op := be.Op
	switch op {
	case "&&":
		op = "&&"
	case "||":
		op = "||"
	case "!=":
		op = "/="
	case "%":
		// `mod` floors, so it takes the divisor's sign: -7 `mod` 2 is 1 where
		// C, Go, Java and Rust answer -1. `rem` is `quot`'s partner — the two
		// have to be chosen together, and the line below already chose `quot`.
		op = "`rem`"
	case "/":
		// `/` belongs to Fractional, so dividing two Ints does not merely give
		// the wrong answer in Haskell — it does not typecheck. `quot`
		// truncates, which is what the other targets do.
		if g.types.isIntDivision(be) {
			op = "`quot`"
		}
	}
	g.write("(")
	if err := g.emitExpr(be.Left); err != nil {
		return err
	}
	g.write(" " + op + " ")
	if err := g.emitExpr(be.Right); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *hsGen) emitUnary(ue *ast.UnaryExpr) error {
	switch ue.Op {
	case "!":
		g.write("(not ")
		if err := g.emitExpr(ue.Operand); err != nil {
			return err
		}
		g.write(")")
	case "-":
		g.write("(negate ")
		if err := g.emitExpr(ue.Operand); err != nil {
			return err
		}
		g.write(")")
	default:
		g.write(ue.Op)
		if err := g.emitExpr(ue.Operand); err != nil {
			return err
		}
	}
	return nil
}

func (g *hsGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		if len(ce.Args) == 0 {
			g.write(`putStrLn ""`)
			return nil
		}
		tk := g.inferTypeKind(ce.Args[0])
		if tk == "String" {
			g.write("putStrLn ")
		} else {
			g.write("print ")
		}
		needParen := g.needParens(ce.Args[0])
		if needParen {
			g.write("(")
		}
		if err := g.emitExpr(ce.Args[0]); err != nil {
			return err
		}
		if needParen {
			g.write(")")
		}
		return nil
	case "printf":
		if len(ce.Args) >= 2 {
			g.needPrintf = true
			g.write("printf ")
			for _, arg := range ce.Args {
				g.write(" ")
				needParen := g.needParens(arg)
				if needParen {
					g.write("(")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
				if needParen {
					g.write(")")
				}
			}
		} else if len(ce.Args) > 0 {
			tk := g.inferTypeKind(ce.Args[0])
			if tk == "String" {
				g.write("putStr ")
			} else {
				g.write("putStr (show ")
			}
			needParen := g.needParens(ce.Args[0])
			if needParen {
				g.write("(")
			}
			if err := g.emitExpr(ce.Args[0]); err != nil {
				return err
			}
			if needParen {
				g.write(")")
			}
			if tk != "String" {
				g.write(")")
			}
		}
		return nil
	case "sprintf":
		if len(ce.Args) >= 2 {
			g.needPrintf = true
			g.write("(printf ")
			for _, arg := range ce.Args {
				g.write(" ")
				needParen := g.needParens(arg)
				if needParen {
					g.write("(")
				}
				if err := g.emitExpr(arg); err != nil {
					return err
				}
				if needParen {
					g.write(")")
				}
			}
			g.write(" :: String)")
		} else if len(ce.Args) > 0 {
			tk := g.inferTypeKind(ce.Args[0])
			if tk == "String" {
				if err := g.emitExpr(ce.Args[0]); err != nil {
					return err
				}
			} else {
				g.write("show ")
				if err := g.emitExpr(ce.Args[0]); err != nil {
					return err
				}
			}
		} else {
			g.write(`""`)
		}
		return nil
	default:
		g.write(ce.Callee)
		for _, arg := range ce.Args {
			g.write(" ")
			needParen := g.needParens(arg)
			if needParen {
				g.write("(")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
			if needParen {
				g.write(")")
			}
		}
		return nil
	}
}

func (g *hsGen) needParens(n ast.Node) bool {
	switch n.(type) {
	case *ast.CallExpr, *ast.BinaryExpr, *ast.UnaryExpr, *ast.MemberExpr:
		return true
	default:
		return false
	}
}

func (g *hsGen) emitLiteral(lit *ast.Literal) error {
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
			g.write("True")
		} else {
			g.write("False")
		}
	default:
		g.write(fmt.Sprintf("%v", lit.Value))
	}
	return nil
}

func (g *hsGen) emitStructLit(sl *ast.StructLit) error {
	g.write(sl.TypeName + " {")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(f.Name + " = ")
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write("}")
	return nil
}

func (g *hsGen) emitArrayLit(al *ast.ArrayLit) error {
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

func (g *hsGen) emitIndexExpr(ie *ast.IndexExpr) error {
	g.write("(")
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write(" !! ")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *hsGen) emitEnumDecl(ed *ast.EnumDecl) error {
	g.writeln("data " + ed.Name + " = " + strings.Join(ed.Variants, " | ") + " deriving (Show, Eq)")
	return nil
}

func (g *hsGen) emitWhileStmt(ws *ast.WhileStmt) error {
	loopName := fmt.Sprintf("xqlLoop%d", g.loopCount)
	g.loopCount++

	g.writeIndent()
	g.writeln("let " + loopName + " = do")
	// Two levels, not one. Haskell's layout rule closes a `let` block at the
	// first line indented no further than the *binding name*, and "let " is four
	// characters — so a body one level in lands in the same column as
	// `xqlLoop0` and `i <- readIORef iRef` is read as a second binding: "parse
	// error on input '<-'". Nothing caught it because control_flow.xql.json is
	// the corpus's only while program and haskell declines its `break`, so this
	// loop had never been compiled.
	g.indent += 2

	if lit, ok := ws.Cond.(*ast.Literal); ok && lit.ValueType == "Bool" && lit.Value == true {
		for _, s := range ws.Body {
			if err := g.emitStmt(s); err != nil {
				return err
			}
		}
		g.writeIndent()
		g.writeln(loopName)
	} else {
		g.preReadMutables(ws.Cond)
		g.writeIndent()
		g.write("if ")
		if err := g.emitExpr(ws.Cond); err != nil {
			return err
		}
		g.writeln("")
		g.indent++
		g.writeIndent()
		g.writeln("then do")
		g.indent++
		for _, s := range ws.Body {
			if err := g.emitStmt(s); err != nil {
				return err
			}
		}
		g.writeIndent()
		g.writeln(loopName)
		g.indent--
		g.writeIndent()
		g.writeln("else return ()")
		g.indent--
	}

	g.indent -= 2
	g.writeIndent()
	g.writeln(loopName)
	return nil
}

func hsCollectIdents(n ast.Node) map[string]bool {
	ids := make(map[string]bool)
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if n == nil {
			return
		}
		switch node := n.(type) {
		case *ast.Ident:
			ids[node.Name] = true
		case *ast.BinaryExpr:
			walk(node.Left)
			walk(node.Right)
		case *ast.UnaryExpr:
			walk(node.Operand)
		case *ast.CallExpr:
			for _, a := range node.Args {
				walk(a)
			}
		case *ast.IndexExpr:
			walk(node.Target)
			walk(node.Index)
		case *ast.MemberExpr:
			walk(node.Object)
		case *ast.IfExpr:
			walk(node.Cond)
			walk(node.Then)
			walk(node.Else)
		case *ast.ArrayLit:
			for _, e := range node.Elements {
				walk(e)
			}
		case *ast.StructLit:
			for _, f := range node.Fields {
				walk(f.Value)
			}
		}
	}
	walk(n)
	return ids
}

func (g *hsGen) preReadMutables(exprs ...ast.Node) {
	seen := make(map[string]bool)
	for _, expr := range exprs {
		if expr == nil {
			continue
		}
		for name := range hsCollectIdents(expr) {
			if g.mutables[name] && !seen[name] {
				seen[name] = true
				g.writeIndent()
				g.writeln(name + " <- readIORef " + name + "Ref")
			}
		}
	}
}

func (g *hsGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("case ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln(" of")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			g.write("_ -> ")
		} else {
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.write(" -> ")
		}
		if len(arm.Body) == 1 {
			if es, ok := arm.Body[0].(*ast.ExprStmt); ok {
				if err := g.emitExpr(es.Expr); err != nil {
					return err
				}
				g.writeln("")
				continue
			}
		}
		if g.inIO {
			g.writeln("do")
		} else {
			g.writeln("")
		}
		g.indent++
		for _, s := range arm.Body {
			if err := g.emitStmt(s); err != nil {
				return err
			}
		}
		g.indent--
	}
	g.indent--
	return nil
}
