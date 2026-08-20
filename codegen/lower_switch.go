package codegen

import "xiaoqinli/ast"

// SwitchStmt is sugar for MatchExpr, and this file is where it stops being a
// second node.
//
// Thirteen backends emit a native switch. The other twenty-five had no
// emitSwitchStmt at all: five of them (lua, ruby, php, nim, julia) reached
// their statement emitter's default arm and returned "unsupported node
// SwitchStmt", and the remaining twenty were refused up front by
// validateNodesForTarget. Writing twenty-five new emitters would have meant
// twenty-five new places to be wrong about a construct — the trade this corpus
// keeps catching — when every one of those targets already carries a MatchExpr
// emitter that examples/match_arms.xql.json and examples/enum_match.xql.json
// exercise on every run.
//
// So a target without a native switch is handed a match instead. The two nodes
// hold the same three things — a discriminant, a list of value/body pairs, and
// a fallback — and the rewrite is total: after it, no SwitchStmt reaches a
// backend that cannot spell one.
//
// The rewrite is functional rather than in-place. Generate is called once per
// target over an AST the caller usually keeps (the conformance suite parses one
// file and compiles it to all thirty-eight), so mutating the tree would leave
// the next target compiling a program the previous one rewrote. Nodes on the
// path to a switch are shallow-copied; everything else is shared.
var nativeSwitchTargets = map[string]bool{
	"go": true, "rust": true, "ts": true, "js": true, "py": true,
	"java": true, "csharp": true, "kotlin": true, "swift": true,
	"dart": true, "zig": true, "android": true, "ios": true,
}

// lowerSwitchForTarget returns root with every SwitchStmt rewritten to a
// MatchExpr, unless target emits switches itself.
func lowerSwitchForTarget(root ast.Node, target string) ast.Node {
	if nativeSwitchTargets[targetAlias(target)] {
		return root
	}
	lowered, _ := lowerSwitch(root)
	return lowered
}

// switchToMatch turns one switch into one match.
//
// The default case becomes a `_` arm and moves to the end. Moving it is safe —
// every other case matches a value, so the default is taken exactly when none
// of them is, wherever it was written — and it is necessary for the backends
// whose match is ordered: an arm after a wildcard is dead code in Rust, OCaml
// and Haskell alike.
//
// A switch with no default becomes a match with no wildcard, which is what the
// source program said. Nothing is invented to fill it.
func switchToMatch(ss *ast.SwitchStmt) *ast.MatchExpr {
	me := &ast.MatchExpr{Value: ss.Value}
	var defaultBody []ast.Node
	haveDefault := false
	for _, c := range ss.Cases {
		if c.Value == nil {
			// A second default is unreachable in every language that has the
			// construct. The first one wins here too, rather than the last one
			// silently replacing it.
			if !haveDefault {
				defaultBody = c.Body
				haveDefault = true
			}
			continue
		}
		me.Arms = append(me.Arms, ast.MatchArm{Pattern: c.Value, Body: c.Body})
	}
	if haveDefault {
		me.Arms = append(me.Arms, ast.MatchArm{Pattern: &ast.Ident{Name: "_"}, Body: defaultBody})
	}
	return me
}

// lowerSwitch rewrites n, reporting whether anything below it changed. A false
// second return means the first is the node that came in.
func lowerSwitch(n ast.Node) (ast.Node, bool) {
	if n == nil {
		return nil, false
	}
	switch node := n.(type) {
	case *ast.Program:
		decls, changed := lowerSwitchList(node.Decls)
		if !changed {
			return node, false
		}
		cp := *node
		cp.Decls = decls
		return &cp, true

	case *ast.FunctionDecl:
		body, changed := lowerSwitchList(node.Body)
		if !changed {
			return node, false
		}
		cp := *node
		cp.Body = body
		return &cp, true

	case *ast.Lambda:
		body, changed := lowerSwitchList(node.Body)
		if !changed {
			return node, false
		}
		cp := *node
		cp.Body = body
		return &cp, true

	case *ast.SwitchStmt:
		value, _ := lowerSwitch(node.Value)
		cases := make([]ast.SwitchCase, len(node.Cases))
		for i, c := range node.Cases {
			cv, _ := lowerSwitch(c.Value)
			cb, _ := lowerSwitchList(c.Body)
			cases[i] = ast.SwitchCase{Value: cv, Body: cb}
		}
		return switchToMatch(&ast.SwitchStmt{Value: value, Cases: cases}), true

	case *ast.MatchExpr:
		value, changed := lowerSwitch(node.Value)
		arms := make([]ast.MatchArm, len(node.Arms))
		for i, arm := range node.Arms {
			body, ch := lowerSwitchList(arm.Body)
			if ch {
				changed = true
			}
			arms[i] = ast.MatchArm{Pattern: arm.Pattern, Body: body}
		}
		if !changed {
			return node, false
		}
		return &ast.MatchExpr{Value: value, Arms: arms}, true

	case *ast.IfStmt:
		cond, changed := lowerSwitch(node.Cond)
		then, ch1 := lowerSwitchList(node.Then)
		els, ch2 := lowerSwitchList(node.Else)
		if !changed && !ch1 && !ch2 {
			return node, false
		}
		return &ast.IfStmt{Cond: cond, Then: then, Else: els}, true

	case *ast.WhileStmt:
		cond, changed := lowerSwitch(node.Cond)
		body, ch := lowerSwitchList(node.Body)
		if !changed && !ch {
			return node, false
		}
		return &ast.WhileStmt{Cond: cond, Body: body}, true

	case *ast.ForStmt:
		start, c1 := lowerSwitch(node.Start)
		end, c2 := lowerSwitch(node.End)
		iter, c3 := lowerSwitch(node.Iterable)
		body, c4 := lowerSwitchList(node.Body)
		if !c1 && !c2 && !c3 && !c4 {
			return node, false
		}
		cp := *node
		cp.Start, cp.End, cp.Iterable, cp.Body = start, end, iter, body
		return &cp, true

	case *ast.ReturnStmt:
		value, changed := lowerSwitch(node.Value)
		if !changed {
			return node, false
		}
		return &ast.ReturnStmt{Value: value}, true

	case *ast.ExprStmt:
		expr, changed := lowerSwitch(node.Expr)
		if !changed {
			return node, false
		}
		return &ast.ExprStmt{Expr: expr}, true

	case *ast.VarDecl:
		value, changed := lowerSwitch(node.Value)
		if !changed {
			return node, false
		}
		cp := *node
		cp.Value = value
		return &cp, true

	case *ast.AssignStmt:
		target, c1 := lowerSwitch(node.Target)
		value, c2 := lowerSwitch(node.Value)
		if !c1 && !c2 {
			return node, false
		}
		return &ast.AssignStmt{Target: target, Value: value}, true

	case *ast.BinaryExpr:
		left, c1 := lowerSwitch(node.Left)
		right, c2 := lowerSwitch(node.Right)
		if !c1 && !c2 {
			return node, false
		}
		cp := *node
		cp.Left, cp.Right = left, right
		return &cp, true

	case *ast.UnaryExpr:
		operand, changed := lowerSwitch(node.Operand)
		if !changed {
			return node, false
		}
		cp := *node
		cp.Operand = operand
		return &cp, true

	case *ast.CallExpr:
		args, changed := lowerSwitchList(node.Args)
		if !changed {
			return node, false
		}
		cp := *node
		cp.Args = args
		return &cp, true

	case *ast.NewExpr:
		args, changed := lowerSwitchList(node.Args)
		if !changed {
			return node, false
		}
		cp := *node
		cp.Args = args
		return &cp, true

	case *ast.MemberExpr:
		object, changed := lowerSwitch(node.Object)
		if !changed {
			return node, false
		}
		cp := *node
		cp.Object = object
		return &cp, true

	case *ast.IndexExpr:
		target, c1 := lowerSwitch(node.Target)
		index, c2 := lowerSwitch(node.Index)
		if !c1 && !c2 {
			return node, false
		}
		cp := *node
		cp.Target, cp.Index = target, index
		return &cp, true

	case *ast.IfExpr:
		cond, c1 := lowerSwitch(node.Cond)
		then, c2 := lowerSwitch(node.Then)
		els, c3 := lowerSwitch(node.Else)
		if !c1 && !c2 && !c3 {
			return node, false
		}
		cp := *node
		cp.Cond, cp.Then, cp.Else = cond, then, els
		return &cp, true

	case *ast.AwaitExpr:
		expr, changed := lowerSwitch(node.Expr)
		if !changed {
			return node, false
		}
		cp := *node
		cp.Expr = expr
		return &cp, true

	case *ast.ArrayLit:
		elems, changed := lowerSwitchList(node.Elements)
		if !changed {
			return node, false
		}
		cp := *node
		cp.Elements = elems
		return &cp, true

	case *ast.ArrayLiteral:
		elems, changed := lowerSwitchList(node.Elements)
		if !changed {
			return node, false
		}
		cp := *node
		cp.Elements = elems
		return &cp, true

	case *ast.MapLiteral:
		changed := false
		entries := make([]ast.MapEntry, len(node.Entries))
		for i, e := range node.Entries {
			k, c1 := lowerSwitch(e.Key)
			v, c2 := lowerSwitch(e.Value)
			if c1 || c2 {
				changed = true
			}
			entries[i] = ast.MapEntry{Key: k, Value: v}
		}
		if !changed {
			return node, false
		}
		cp := *node
		cp.Entries = entries
		return &cp, true

	case *ast.StructLit:
		changed := false
		fields := make([]ast.StructFieldInit, len(node.Fields))
		for i, f := range node.Fields {
			v, c := lowerSwitch(f.Value)
			if c {
				changed = true
			}
			fields[i] = ast.StructFieldInit{Name: f.Name, Value: v}
		}
		if !changed {
			return node, false
		}
		cp := *node
		cp.Fields = fields
		return &cp, true
	}
	return n, false
}

// lowerSwitchList rewrites a statement list, sharing the input slice when
// nothing in it changed.
func lowerSwitchList(list []ast.Node) ([]ast.Node, bool) {
	changed := false
	out := make([]ast.Node, len(list))
	for i, s := range list {
		ns, ch := lowerSwitch(s)
		out[i] = ns
		if ch {
			changed = true
		}
	}
	if !changed {
		return list, false
	}
	return out, true
}
