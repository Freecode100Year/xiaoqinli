package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

func GenerateBat(root ast.Node) ([]byte, error) {
	g := &batGen{buf: &strings.Builder{}, savedTmps: make(map[ast.Node]string), forVars: make(map[string]bool)}

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	g.writeln("@echo off")
	g.writeln("setlocal EnableDelayedExpansion")
	g.writeln("")

	// Emit enum constants as set statements.
	for _, d := range prog.Decls {
		if ed, ok := d.(*ast.EnumDecl); ok {
			g.emitEnumDecl(ed)
			g.writeln("")
		}
	}

	// Emit main body at top level.
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		for _, stmt := range fd.Body {
			if err := g.emitNode(stmt); err != nil {
				return nil, err
			}
		}
	}

	// Exit before function labels.
	g.writeln("goto :eof")

	// Emit non-main functions as labels.
	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name == "main" {
			continue
		}
		g.writeln("")
		if err := g.emitFunctionDecl(fd); err != nil {
			return nil, err
		}
	}

	return []byte(g.buf.String()), nil
}

type batGen struct {
	buf       *strings.Builder
	indent    int
	whileID   int
	tmpID     int
	savedTmps map[ast.Node]string
	inBlock   bool
	forVars   map[string]bool
}

func (g *batGen) write(s string)   { g.buf.WriteString(s) }
func (g *batGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *batGen) writeIndent()     { for i := 0; i < g.indent; i++ { g.buf.WriteString("    ") } }

func (g *batGen) emitEnumDecl(ed *ast.EnumDecl) {
	for i, v := range ed.Variants {
		g.writeIndent()
		g.writeln(fmt.Sprintf("set \"%s=%d\"", v, i))
	}
}

func (g *batGen) emitNode(n ast.Node) error {
	switch node := n.(type) {
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
		return fmt.Errorf("XQL_E401: bat does not support break in this context")
	case *ast.ContinueStmt:
		return fmt.Errorf("XQL_E401: bat does not support continue")
	case *ast.ExprStmt:
		return g.emitExprStmt(node)
	case *ast.MatchExpr:
		return g.emitMatchStmt(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *batGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.writeln(":" + fd.Name)
	g.indent++
	g.writeIndent()
	g.writeln("setlocal EnableDelayedExpansion")
	for i, p := range fd.Params {
		g.writeIndent()
		g.writeln(fmt.Sprintf("set \"%s=%%~%d\"", p.Name, i+1))
	}
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	// If the last statement already emitted endlocal+exit (non-block return), skip.
	lastIsReturn := false
	if len(fd.Body) > 0 {
		if _, ok := fd.Body[len(fd.Body)-1].(*ast.ReturnStmt); ok {
			lastIsReturn = true
		}
	}
	if !lastIsReturn {
		g.writeIndent()
		g.writeln("for /f \"delims=\" %%r in (\"!_return!\") do endlocal & set \"_return=%%r\"")
		g.writeIndent()
		g.writeln("exit /b 0")
	}
	g.indent--
	return nil
}

func (g *batGen) emitReturn(rs *ast.ReturnStmt) error {
	if rs.Value == nil {
		g.writeIndent()
		g.writeln("set \"_return=\"")
		if !g.inBlock {
			g.writeIndent()
			g.writeln("for /f \"delims=\" %%r in (\"!_return!\") do endlocal & set \"_return=%%r\"")
			g.writeIndent()
			g.writeln("exit /b 0")
		}
		return nil
	}
	if err := g.emitNestedCalls(rs.Value); err != nil {
		return err
	}
	g.writeIndent()
	if g.isArithExpr(rs.Value) {
		g.write("set /a \"_return=")
		if err := g.emitArithExpr(rs.Value); err != nil {
			return err
		}
		g.writeln("\"")
	} else {
		g.write("set \"_return=")
		if err := g.emitValExpr(rs.Value); err != nil {
			return err
		}
		g.writeln("\"")
	}
	if !g.inBlock {
		g.writeIndent()
		g.writeln("for /f \"delims=\" %%r in (\"!_return!\") do endlocal & set \"_return=%%r\"")
		g.writeIndent()
		g.writeln("exit /b 0")
	}
	return nil
}

func (g *batGen) emitVarDecl(vd *ast.VarDecl) error {
	if vd.Value != nil {
		// Handle IfExpr as if-else block
		if ie, ok := vd.Value.(*ast.IfExpr); ok {
			prev := g.inBlock
			g.inBlock = true
			g.writeIndent()
			g.write("if ")
			if err := g.emitCondExpr(ie.Cond); err != nil {
				return err
			}
			g.writeln(" (")
			g.indent++
			g.writeIndent()
			g.write("set \"" + vd.Name + "=")
			if err := g.emitValExpr(ie.Then); err != nil {
				return err
			}
			g.writeln("\"")
			g.indent--
			g.writeIndent()
			g.writeln(") else (")
			g.indent++
			g.writeIndent()
			g.write("set \"" + vd.Name + "=")
			if err := g.emitValExpr(ie.Else); err != nil {
				return err
			}
			g.writeln("\"")
			g.indent--
			g.writeIndent()
			g.writeln(")")
			g.inBlock = prev
			return nil
		}
		// Handle ArrayLit as pseudo-array
		if al, ok := vd.Value.(*ast.ArrayLit); ok && vd.Type.KindName == "Array" {
			for i, elem := range al.Elements {
				g.writeIndent()
				g.write(fmt.Sprintf("set \"%s[%d]=", vd.Name, i))
				if err := g.emitValExpr(elem); err != nil {
					return err
				}
				g.writeln("\"")
			}
			return nil
		}
		// Pre-emit any nested function calls.
		if err := g.emitNestedCalls(vd.Value); err != nil {
			return err
		}
	}
	g.writeIndent()
	isNum := vd.Type.KindName == "Int" || vd.Type.KindName == "Float"
	if isNum {
		g.write("set /a \"" + vd.Name + "=")
		if vd.Value != nil {
			if err := g.emitArithExpr(vd.Value); err != nil {
				return err
			}
		} else {
			g.write("0")
		}
		g.writeln("\"")
	} else {
		g.write("set \"" + vd.Name + "=")
		if vd.Value != nil {
			if err := g.emitValExpr(vd.Value); err != nil {
				return err
			}
		}
		g.writeln("\"")
	}
	return nil
}

func (g *batGen) emitAssign(as *ast.AssignStmt) error {
	// Pre-emit nested calls.
	if err := g.emitNestedCalls(as.Value); err != nil {
		return err
	}
	g.writeIndent()
	varName := ""
	if ident, ok := as.Target.(*ast.Ident); ok {
		varName = ident.Name
	} else {
		return fmt.Errorf("XQL_E401: bat only supports simple variable assignment targets")
	}
	if containsStringExpr(as.Value) {
		g.write("set \"" + varName + "=")
		if err := g.emitValExpr(as.Value); err != nil {
			return err
		}
		g.writeln("\"")
	} else {
		g.write("set /a \"" + varName + "=")
		if err := g.emitArithExpr(as.Value); err != nil {
			return err
		}
		g.writeln("\"")
	}
	return nil
}

// isArithExpr checks if the expression should use set /a (numeric context).
func (g *batGen) isArithExpr(n ast.Node) bool {
	switch node := n.(type) {
	case *ast.BinaryExpr:
		return !containsStringExpr(node)
	case *ast.Literal:
		return node.ValueType == "Int" || node.ValueType == "Float"
	case *ast.CallExpr:
		return true
	default:
		return false
	}
}

func (g *batGen) emitIf(is *ast.IfStmt) error {
	prev := g.inBlock
	g.inBlock = true
	g.writeIndent()
	g.write("if ")
	if err := g.emitCondExpr(is.Cond); err != nil {
		return err
	}
	g.writeln(" (")
	g.indent++
	for _, s := range is.Then {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	if len(is.Else) > 0 {
		g.writeIndent()
		g.writeln(") else (")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
	}
	g.writeIndent()
	g.writeln(")")
	g.inBlock = prev
	return nil
}

func (g *batGen) emitWhile(ws *ast.WhileStmt) error {
	g.whileID++
	id := g.whileID
	label := fmt.Sprintf("_while_%d", id)
	endLabel := fmt.Sprintf("_endwhile_%d", id)

	g.writeIndent()
	g.writeln(":" + label)
	g.writeIndent()
	g.write("if not ")
	if err := g.emitCondExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln(" goto :" + endLabel)
	g.indent++
	for _, s := range ws.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.writeIndent()
	g.writeln("goto :" + label)
	g.indent--
	g.writeIndent()
	g.writeln(":" + endLabel)
	return nil
}

func (g *batGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("for /L %%")
		g.write(fs.Var)
		g.write(" in (")
		if err := g.emitArithExpr(fs.Start); err != nil {
			return err
		}
		g.write(",1,")
		// end - 1 for exclusive upper bound
		if lit, ok := fs.End.(*ast.Literal); ok {
			f, _ := lit.Value.(float64)
			g.write(fmt.Sprintf("%d", int64(f)-1))
		} else {
			g.write("!_end_tmp!")
		}
		g.writeln(") do (")
		g.forVars[fs.Var] = true
		g.indent++
		for _, s := range fs.Body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
		delete(g.forVars, fs.Var)
		g.writeIndent()
		g.writeln(")")
	case "each":
		g.write("for %%")
		g.write(fs.Var)
		g.write(" in (")
		if err := g.emitValExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln(") do (")
		g.forVars[fs.Var] = true
		g.indent++
		for _, s := range fs.Body {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
		delete(g.forVars, fs.Var)
		g.writeIndent()
		g.writeln(")")
	default:
		return fmt.Errorf("XQL_E401: unsupported for-loop form %q", fs.Form)
	}
	return nil
}

func (g *batGen) emitMatchStmt(me *ast.MatchExpr) error {
	g.writeIndent()
	g.write("set \"_match_val=")
	if err := g.emitValExpr(me.Value); err != nil {
		return err
	}
	g.writeln("\"")
	for i, arm := range me.Arms {
		g.writeIndent()
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			// Default arm — no condition needed (else branch).
			if i > 0 {
				// Already inside an else, just emit body.
			}
			g.writeln("rem default")
			g.indent++
			for _, stmt := range arm.Body {
				if err := g.emitNode(stmt); err != nil {
					return err
				}
			}
			g.indent--
		} else {
			if i > 0 {
				g.write(") else ")
			}
			g.write("if !_match_val! EQU ")
			if err := g.emitValExpr(arm.Pattern); err != nil {
				return err
			}
			g.writeln(" (")
			g.indent++
			for _, stmt := range arm.Body {
				if err := g.emitNode(stmt); err != nil {
					return err
				}
			}
			g.indent--
		}
	}
	if len(me.Arms) > 0 {
		g.writeIndent()
		g.writeln(")")
	}
	return nil
}

func (g *batGen) emitExprStmt(es *ast.ExprStmt) error {
	if ce, ok := es.Expr.(*ast.CallExpr); ok {
		return g.emitCallStmt(ce)
	}
	g.writeIndent()
	if err := g.emitValExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

// emitNestedCalls pre-emits any nested function calls so their results
// are available in !_return! before the outer expression uses them.
// For multiple calls in one expression (e.g. f(a) + f(b)), it saves
// intermediate results in _tmp_N variables.
func (g *batGen) emitNestedCalls(n ast.Node) error {
	switch node := n.(type) {
	case *ast.CallExpr:
		switch node.Callee {
		case "println", "printf", "sprintf":
			for _, arg := range node.Args {
				if err := g.emitNestedCalls(arg); err != nil {
					return err
				}
			}
		default:
			for _, arg := range node.Args {
				if err := g.emitNestedCalls(arg); err != nil {
					return err
				}
			}
			// Pre-compute any arithmetic arguments into temp vars.
			argRefs := make([]string, len(node.Args))
			for i, arg := range node.Args {
				if g.isArithExpr(arg) && !g.isSimpleValue(arg) {
					g.tmpID++
					tmpVar := fmt.Sprintf("_arg_%d", g.tmpID)
					g.writeIndent()
					g.write(fmt.Sprintf("set /a \"%s=", tmpVar))
					if err := g.emitArithExpr(arg); err != nil {
						return err
					}
					g.writeln("\"")
					argRefs[i] = "!" + tmpVar + "!"
				}
			}
			g.writeIndent()
			g.write("call :" + node.Callee)
			for i, arg := range node.Args {
				g.write(" \"")
				if argRefs[i] != "" {
					g.write(argRefs[i])
				} else {
					if err := g.emitValExpr(arg); err != nil {
						return err
					}
				}
				g.write("\"")
			}
			g.writeln("")
		}
	case *ast.BinaryExpr:
		leftHasCall := containsCall(node.Left)
		rightHasCall := containsCall(node.Right)
		if leftHasCall {
			if err := g.emitNestedCalls(node.Left); err != nil {
				return err
			}
			if rightHasCall {
				g.tmpID++
				tmpVar := fmt.Sprintf("_tmp_%d", g.tmpID)
				g.savedTmps[node] = tmpVar
				g.writeIndent()
				g.writeln(fmt.Sprintf("set /a \"%s=!_return!\"", tmpVar))
				if err := g.emitNestedCalls(node.Right); err != nil {
					return err
				}
			}
		} else if rightHasCall {
			if err := g.emitNestedCalls(node.Right); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *batGen) emitIndexExpr(ie *ast.IndexExpr) error {
	ident, ok := ie.Target.(*ast.Ident)
	if !ok {
		return fmt.Errorf("XQL_E401: bat IndexExpr requires simple identifier target")
	}
	g.write("!")
	g.write(ident.Name)
	g.write("[")
	if idx, ok := ie.Index.(*ast.Ident); ok && g.forVars[idx.Name] {
		g.write("%%" + idx.Name)
	} else if lit, ok := ie.Index.(*ast.Literal); ok {
		f, _ := lit.Value.(float64)
		g.write(fmt.Sprintf("%d", int64(f)))
	} else if idx, ok := ie.Index.(*ast.Ident); ok {
		g.write("!" + idx.Name + "!")
	}
	g.write("]!")
	return nil
}

func (g *batGen) emitCallStmt(ce *ast.CallExpr) error {
	// Pre-emit any nested function calls.
	if err := g.emitNestedCalls(ce); err != nil {
		return err
	}

	g.writeIndent()
	switch ce.Callee {
	case "println":
		if len(ce.Args) == 0 {
			g.writeln("echo.")
			return nil
		}
		g.write("echo ")
		if err := g.emitValExpr(ce.Args[0]); err != nil {
			return err
		}
		g.writeln("")
		return nil
	case "printf":
		if len(ce.Args) == 0 {
			return nil
		}
		g.write("<nul set /p \"=")
		if err := g.emitValExpr(ce.Args[0]); err != nil {
			return err
		}
		g.writeln("\"")
		return nil
	default:
		// Already emitted by emitNestedCalls.
		return nil
	}
}

// emitValExpr writes the value of an expression inline (for set, echo, etc).
func (g *batGen) emitValExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write("!" + node.Name + "!")
		return nil
	case *ast.BinaryExpr:
		if node.Op == "+" && containsStringExpr(node) {
			if err := g.emitValExpr(node.Left); err != nil {
				return err
			}
			return g.emitValExpr(node.Right)
		}
		// Numeric — use arithmetic.
		return g.emitArithExpr(n)
	case *ast.CallExpr:
		return g.emitCallCapture(node)
	case *ast.ArrayLit:
		for i, elem := range node.Elements {
			if i > 0 {
				g.write(" ")
			}
			if err := g.emitValExpr(elem); err != nil {
				return err
			}
		}
		return nil
	case *ast.IndexExpr:
		return g.emitIndexExpr(node)
	case *ast.IfExpr:
		return fmt.Errorf("XQL_E401: bat does not support IfExpr in expression context")
	case *ast.Lambda:
		return fmt.Errorf("XQL_E401: bat does not support Lambda")
	default:
		return fmt.Errorf("XQL_E401: unsupported expression %s in bat", n.Kind())
	}
}

// emitCallCapture handles a function call used as a value expression.
// It emits `call :func args` inline and references !_return!.
func (g *batGen) emitCallCapture(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "sprintf":
		if len(ce.Args) > 0 {
			return g.emitValExpr(ce.Args[0])
		}
		return nil
	default:
		// We need to emit the call before using _return,
		// but in value context we just reference _return.
		// The caller should have emitted the call statement.
		g.write("!_return!")
		return nil
	}
}

func (g *batGen) emitCondExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.BinaryExpr:
		switch node.Op {
		case "==":
			if err := g.emitCondVal(node.Left); err != nil {
				return err
			}
			g.write(" EQU ")
			return g.emitCondVal(node.Right)
		case "!=":
			if err := g.emitCondVal(node.Left); err != nil {
				return err
			}
			g.write(" NEQ ")
			return g.emitCondVal(node.Right)
		case "<":
			if err := g.emitCondVal(node.Left); err != nil {
				return err
			}
			g.write(" LSS ")
			return g.emitCondVal(node.Right)
		case ">":
			if err := g.emitCondVal(node.Left); err != nil {
				return err
			}
			g.write(" GTR ")
			return g.emitCondVal(node.Right)
		case "<=":
			if err := g.emitCondVal(node.Left); err != nil {
				return err
			}
			g.write(" LEQ ")
			return g.emitCondVal(node.Right)
		case ">=":
			if err := g.emitCondVal(node.Left); err != nil {
				return err
			}
			g.write(" GEQ ")
			return g.emitCondVal(node.Right)
		default:
			if err := g.emitCondVal(node.Left); err != nil {
				return err
			}
			g.write(" " + node.Op + " ")
			return g.emitCondVal(node.Right)
		}
	case *ast.UnaryExpr:
		if node.Op == "!" {
			g.write("not ")
			return g.emitCondExpr(node.Operand)
		}
		return g.emitCondVal(n)
	default:
		return g.emitCondVal(n)
	}
}

func (g *batGen) emitCondVal(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write("!" + node.Name + "!")
		return nil
	default:
		return g.emitValExpr(n)
	}
}

func (g *batGen) isSimpleValue(n ast.Node) bool {
	switch n.(type) {
	case *ast.Literal, *ast.Ident:
		return true
	default:
		return false
	}
}

func containsCall(n ast.Node) bool {
	switch node := n.(type) {
	case *ast.CallExpr:
		return node.Callee != "sprintf"
	case *ast.BinaryExpr:
		return containsCall(node.Left) || containsCall(node.Right)
	case *ast.UnaryExpr:
		return containsCall(node.Operand)
	default:
		return false
	}
}

// emitArithExpr writes an expression for set /a context.
// Function calls should have been pre-emitted; their results are in _return or _tmp_N.
func (g *batGen) emitArithExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		g.write(node.Name)
		return nil
	case *ast.BinaryExpr:
		g.write("(")
		if tmpVar, ok := g.savedTmps[node]; ok {
			g.write(tmpVar)
		} else if err := g.emitArithExpr(node.Left); err != nil {
			return err
		}
		op := node.Op
		switch op {
		case "&&":
			op = "&"
		case "||":
			op = "|"
		}
		g.write(" " + op + " ")
		if err := g.emitArithExpr(node.Right); err != nil {
			return err
		}
		g.write(")")
		return nil
	case *ast.UnaryExpr:
		g.write(node.Op)
		return g.emitArithExpr(node.Operand)
	case *ast.IndexExpr:
		return g.emitIndexExpr(node)
	case *ast.CallExpr:
		g.write("_return")
		return nil
	default:
		return g.emitValExpr(n)
	}
}

func (g *batGen) emitLiteral(lit *ast.Literal) error {
	switch lit.ValueType {
	case "String":
		s, _ := lit.Value.(string)
		g.write(s)
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
