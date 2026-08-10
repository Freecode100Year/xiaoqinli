package codegen

import (
	"fmt"
	"strings"

	"xiaoqinli/ast"
)

// GenerateClojure produces Clojure source code from the given typed AST.
// The "main" function is emitted as (defn -main [] ...) with a (-main) call.
func GenerateClojure(root ast.Node) ([]byte, error) {
	g := &cljGen{buf: &strings.Builder{}}
	g.types = newTypeKinds(root)

	prog, ok := root.(*ast.Program)
	if !ok {
		return nil, fmt.Errorf("XQL_E401: top-level node must be Program")
	}

	first := true
	for _, d := range prog.Decls {
		switch node := d.(type) {
		case *ast.FunctionDecl:
			if node.Name == "main" {
				continue
			}
			if !first {
				g.writeln("")
			}
			if err := g.emitFunctionDecl(node); err != nil {
				return nil, err
			}
			first = false
		case *ast.StructDecl:
			if !first {
				g.writeln("")
			}
			g.emitStructDecl(node)
			first = false
		case *ast.EnumDecl:
			if !first {
				g.writeln("")
			}
			g.emitEnumDecl(node)
			first = false
		}
	}

	for _, d := range prog.Decls {
		fd, ok := d.(*ast.FunctionDecl)
		if !ok || fd.Name != "main" {
			continue
		}
		g.mutables = collectMutables(fd.Body)
		if !first {
			g.writeln("")
		}
		g.writeln("(defn -main []")
		g.indent++
		for _, stmt := range fd.Body {
			if err := g.emitNode(stmt); err != nil {
				return nil, err
			}
		}
		g.indent--
		g.writeln(")")
		g.writeln("")
		g.writeln("(-main)")
	}

	return []byte(g.buf.String()), nil
}

type cljGen struct {
	types    *typeKinds
	buf      *strings.Builder
	indent   int
	mutables map[string]bool
}

func (g *cljGen) write(s string)   { g.buf.WriteString(s) }
func (g *cljGen) writeln(s string) { g.buf.WriteString(s); g.buf.WriteByte('\n') }
func (g *cljGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("  ")
	}
}

func (g *cljGen) emitNode(n ast.Node) error {
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
		return g.emitWhileStmt(node)
	case *ast.ForStmt:
		return g.emitForStmt(node)
	case *ast.BreakStmt:
		return fmt.Errorf("XQL_E401: Clojure does not support break")
	case *ast.ContinueStmt:
		return fmt.Errorf("XQL_E401: Clojure does not support continue")
	case *ast.ExprStmt:
		return g.emitExprStmt(node)
	case *ast.StructDecl:
		g.emitStructDecl(node)
		return nil
	case *ast.EnumDecl:
		g.emitEnumDecl(node)
		return nil
	case *ast.MatchExpr:
		return g.emitMatchExpr(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported node %s", n.Kind())
	}
}

func (g *cljGen) emitFunctionDecl(fd *ast.FunctionDecl) error {
	g.types.noteParams(fd)
	g.mutables = collectMutables(fd.Body)
	g.writeIndent()
	g.write("(defn " + fd.Name + " [")
	for i, p := range fd.Params {
		if i > 0 {
			g.write(" ")
		}
		g.write(p.Name)
	}
	g.writeln("]")
	g.indent++
	for _, stmt := range fd.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln(")")
	return nil
}

func (g *cljGen) emitReturn(rs *ast.ReturnStmt) error {
	if rs.Value == nil {
		g.writeIndent()
		g.writeln("nil")
		return nil
	}
	g.writeIndent()
	if err := g.emitExpr(rs.Value); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *cljGen) emitVarDecl(vd *ast.VarDecl) error {
	g.types.noteVar(vd)
	g.writeIndent()
	if g.mutables[vd.Name] {
		g.write("(def " + vd.Name + "Ref (atom ")
		if vd.Value != nil {
			if err := g.emitExpr(vd.Value); err != nil {
				return err
			}
		} else {
			g.write("nil")
		}
		g.writeln("))")
	} else {
		g.write("(def " + vd.Name)
		if vd.Value != nil {
			g.write(" ")
			if err := g.emitExpr(vd.Value); err != nil {
				return err
			}
		}
		g.writeln(")")
	}
	return nil
}

func (g *cljGen) emitIf(is *ast.IfStmt) error {
	g.writeIndent()
	g.write("(if ")
	if err := g.emitExpr(is.Cond); err != nil {
		return err
	}
	g.writeln("")
	g.indent++
	// then branch
	g.writeIndent()
	g.writeln("(do")
	g.indent++
	for _, s := range is.Then {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln(")")
	// else branch
	if len(is.Else) > 0 {
		g.writeIndent()
		g.writeln("(do")
		g.indent++
		for _, s := range is.Else {
			if err := g.emitNode(s); err != nil {
				return err
			}
		}
		g.indent--
		g.writeIndent()
		g.writeln(")")
	}
	g.indent--
	g.writeIndent()
	g.writeln(")")
	return nil
}

func (g *cljGen) emitForStmt(fs *ast.ForStmt) error {
	g.writeIndent()
	switch fs.Form {
	case "range":
		g.write("(doseq [" + fs.Var + " (range ")
		if err := g.emitExpr(fs.Start); err != nil {
			return err
		}
		g.write(" ")
		if err := g.emitExpr(fs.End); err != nil {
			return err
		}
		g.writeln(")]")
	case "each":
		g.write("(doseq [" + fs.Var + " ")
		if err := g.emitExpr(fs.Iterable); err != nil {
			return err
		}
		g.writeln("]")
	default:
		return fmt.Errorf("XQL_E401: unsupported for-loop form %q", fs.Form)
	}
	g.indent++
	for _, s := range fs.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln(")")
	return nil
}

func (g *cljGen) emitExprStmt(es *ast.ExprStmt) error {
	g.writeIndent()
	if err := g.emitExpr(es.Expr); err != nil {
		return err
	}
	g.writeln("")
	return nil
}

func (g *cljGen) emitAssign(as *ast.AssignStmt) error {
	if ident, ok := as.Target.(*ast.Ident); ok && g.mutables[ident.Name] {
		g.writeIndent()
		g.write("(reset! " + ident.Name + "Ref ")
		if err := g.emitExpr(as.Value); err != nil {
			return err
		}
		g.writeln(")")
		return nil
	}
	return fmt.Errorf("XQL_E401: Clojure does not support AssignStmt for non-mutable targets")
}

func (g *cljGen) emitWhileStmt(ws *ast.WhileStmt) error {
	g.writeIndent()
	g.write("(while ")
	if err := g.emitExpr(ws.Cond); err != nil {
		return err
	}
	g.writeln("")
	g.indent++
	for _, s := range ws.Body {
		if err := g.emitNode(s); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.writeln(")")
	return nil
}

func (g *cljGen) emitStructDecl(sd *ast.StructDecl) {
	g.writeIndent()
	g.write("(defrecord " + sd.Name + " [")
	for i, f := range sd.Fields {
		if i > 0 {
			g.write(" ")
		}
		g.write(f.Name)
	}
	g.writeln("])")
}

func (g *cljGen) emitEnumDecl(ed *ast.EnumDecl) {
	g.writeIndent()
	g.write(";; enum " + ed.Name + ": ")
	for i, v := range ed.Variants {
		if i > 0 {
			g.write(", ")
		}
		g.write(":" + v)
	}
	g.writeln("")
}

func (g *cljGen) emitExpr(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Literal:
		return g.emitLiteral(node)
	case *ast.Ident:
		if g.mutables[node.Name] {
			g.write("@" + node.Name + "Ref")
		} else {
			g.write(node.Name)
		}
		return nil
	case *ast.BinaryExpr:
		return g.emitBinaryExpr(node)
	case *ast.UnaryExpr:
		return g.emitUnaryExpr(node)
	case *ast.CallExpr:
		return g.emitCall(node)
	case *ast.MemberExpr:
		g.write("(:" + node.Field + " ")
		if err := g.emitExpr(node.Object); err != nil {
			return err
		}
		g.write(")")
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
	case *ast.MatchExpr:
		return g.emitMatchExpr(node)
	default:
		return fmt.Errorf("XQL_E401: unsupported expression %s", n.Kind())
	}
}

func (g *cljGen) emitBinaryExpr(be *ast.BinaryExpr) error {
	op := be.Op
	switch op {
	case "/":
		// Clojure's `/` on two integers yields an exact Ratio: (/ 7 2) is 7/2,
		// and that is what it prints. quot is the truncating division.
		if g.types.isIntDivision(be) {
			op = "quot"
		}
	case "+":
		if containsStringExpr(be) {
			g.write("(str ")
			if err := g.emitExpr(be.Left); err != nil {
				return err
			}
			g.write(" ")
			if err := g.emitExpr(be.Right); err != nil {
				return err
			}
			g.write(")")
			return nil
		}
	case "==":
		op = "="
	case "!=":
		op = "not="
	case "&&":
		op = "and"
	case "||":
		op = "or"
	case "%":
		op = "mod"
	}
	g.write("(" + op + " ")
	if err := g.emitExpr(be.Left); err != nil {
		return err
	}
	g.write(" ")
	if err := g.emitExpr(be.Right); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *cljGen) emitUnaryExpr(ue *ast.UnaryExpr) error {
	switch ue.Op {
	case "!":
		g.write("(not ")
		if err := g.emitExpr(ue.Operand); err != nil {
			return err
		}
		g.write(")")
	case "-":
		g.write("(- ")
		if err := g.emitExpr(ue.Operand); err != nil {
			return err
		}
		g.write(")")
	default:
		g.write("(" + ue.Op + " ")
		if err := g.emitExpr(ue.Operand); err != nil {
			return err
		}
		g.write(")")
	}
	return nil
}

func (g *cljGen) emitCall(ce *ast.CallExpr) error {
	switch ce.Callee {
	case "println":
		g.write("(println ")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(" ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "printf":
		g.write("(printf ")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(" ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	case "sprintf":
		g.write("(format ")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(" ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	default:
		g.write("(" + ce.Callee + " ")
		for i, arg := range ce.Args {
			if i > 0 {
				g.write(" ")
			}
			if err := g.emitExpr(arg); err != nil {
				return err
			}
		}
		g.write(")")
		return nil
	}
}

func (g *cljGen) emitStructLit(sl *ast.StructLit) error {
	g.write("(->" + sl.TypeName + " ")
	for i, f := range sl.Fields {
		if i > 0 {
			g.write(" ")
		}
		if err := g.emitExpr(f.Value); err != nil {
			return err
		}
	}
	g.write(")")
	return nil
}

func (g *cljGen) emitArrayLit(al *ast.ArrayLit) error {
	g.write("[")
	for i, elem := range al.Elements {
		if i > 0 {
			g.write(" ")
		}
		if err := g.emitExpr(elem); err != nil {
			return err
		}
	}
	g.write("]")
	return nil
}

func (g *cljGen) emitIndexExpr(ie *ast.IndexExpr) error {
	g.write("(nth ")
	if err := g.emitExpr(ie.Target); err != nil {
		return err
	}
	g.write(" ")
	if err := g.emitExpr(ie.Index); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *cljGen) emitIfExpr(ie *ast.IfExpr) error {
	g.write("(if ")
	if err := g.emitExpr(ie.Cond); err != nil {
		return err
	}
	g.write(" ")
	if err := g.emitExpr(ie.Then); err != nil {
		return err
	}
	g.write(" ")
	if err := g.emitExpr(ie.Else); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *cljGen) emitLambda(lam *ast.Lambda) error {
	g.write("(fn [")
	for i, p := range lam.Params {
		if i > 0 {
			g.write(" ")
		}
		g.write(p.Name)
	}
	g.writeln("]")
	g.indent++
	for _, stmt := range lam.Body {
		if err := g.emitNode(stmt); err != nil {
			return err
		}
	}
	g.indent--
	g.writeIndent()
	g.write(")")
	return nil
}

func (g *cljGen) emitMatchExpr(me *ast.MatchExpr) error {
	g.write("(case ")
	if err := g.emitExpr(me.Value); err != nil {
		return err
	}
	g.writeln("")
	g.indent++
	for _, arm := range me.Arms {
		g.writeIndent()
		// Check for wildcard/default pattern
		isDefault := false
		if ident, ok := arm.Pattern.(*ast.Ident); ok && ident.Name == "_" {
			isDefault = true
		}
		if !isDefault {
			if err := g.emitExpr(arm.Pattern); err != nil {
				return err
			}
			g.write(" ")
		}
		// Emit body: if single expression, inline; otherwise wrap in do
		if len(arm.Body) == 1 {
			if rs, ok := arm.Body[0].(*ast.ReturnStmt); ok && rs.Value != nil {
				if err := g.emitExpr(rs.Value); err != nil {
					return err
				}
			} else if es, ok := arm.Body[0].(*ast.ExprStmt); ok {
				if err := g.emitExpr(es.Expr); err != nil {
					return err
				}
			} else {
				if err := g.emitExpr(arm.Body[0]); err != nil {
					return err
				}
			}
		} else {
			g.write("(do ")
			for i, s := range arm.Body {
				if i > 0 {
					g.write(" ")
				}
				if rs, ok := s.(*ast.ReturnStmt); ok && rs.Value != nil {
					if err := g.emitExpr(rs.Value); err != nil {
						return err
					}
				} else if es, ok := s.(*ast.ExprStmt); ok {
					if err := g.emitExpr(es.Expr); err != nil {
						return err
					}
				} else {
					if err := g.emitExpr(s); err != nil {
						return err
					}
				}
			}
			g.write(")")
		}
		g.writeln("")
	}
	g.indent--
	g.writeIndent()
	g.write(")")
	return nil
}

func (g *cljGen) emitLiteral(lit *ast.Literal) error {
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
