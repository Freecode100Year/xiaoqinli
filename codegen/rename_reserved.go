package codegen

import (
	"strings"

	"xiaoqinli/ast"
)

// renameReservedForTarget returns root with every declared name that collides
// with a keyword of target's language renamed, along with every reference to
// it. A program with no collision — the common case — comes back unchanged and
// unwalked.
//
// Only names the program itself declares are renamed. An extern names a
// function the host provides, and a builtin like println names one the backend
// emits; renaming either would break the call rather than fix it. So a program
// that declares nothing called `class` gets no rename even if it calls
// something that is.
func renameReservedForTarget(root ast.Node, target string) ast.Node {
	reserved, suffix, foldCase, ok := reservedFor(target)
	if !ok {
		return root
	}
	renames := collectReservedRenames(root, reserved, suffix, foldCase)
	if len(renames) == 0 {
		return root
	}
	return renameNode(root, renames)
}

// declaredNames is the two-part answer to "what did this program name?".
//
// values are the names that appear in expression position — functions,
// variables, parameters, loop variables, types, enums. members are the names
// that appear after a dot: struct and class fields, and enum variants. The
// split matters because a MemberExpr's field can also be a method an extern
// declared on a host object, which this pass must not touch; only a field the
// program itself declared is eligible.
type declaredNames struct {
	values  map[string]bool
	members map[string]bool
	externs map[string]bool
}

func collectReservedRenames(root ast.Node, reserved map[string]bool, suffix string, foldCase bool) map[string]string {
	decls := declaredNames{
		values:  map[string]bool{},
		members: map[string]bool{},
		externs: map[string]bool{},
	}
	collectDeclared(root, &decls)

	isReserved := func(name string) bool {
		if foldCase {
			return reserved[strings.ToLower(name)] || reserved[name]
		}
		return reserved[name]
	}

	// taken guards the replacement against colliding with a name already in
	// the program, or with the keyword table itself.
	taken := map[string]bool{}
	for n := range decls.values {
		taken[n] = true
	}
	for n := range decls.members {
		taken[n] = true
	}
	for n := range decls.externs {
		taken[n] = true
	}

	renames := map[string]string{}
	for _, set := range []map[string]bool{decls.values, decls.members} {
		for name := range set {
			if _, done := renames[name]; done {
				continue
			}
			if !isReserved(name) || decls.externs[name] {
				continue
			}
			// A trailing underscore is what a person writing the program by
			// hand would have reached for, and it is what every language here
			// takes except the ones renameSuffixByLanguage names.
			candidate := name + suffix
			for taken[candidate] || isReserved(candidate) {
				candidate += suffix
			}
			taken[candidate] = true
			renames[name] = candidate
		}
	}
	return renames
}

// collectDeclared records every name the program binds. It reuses walkNodes
// for the statement and expression tree, and handles the declaration nodes'
// own name fields, which walkNodes does not look at.
func collectDeclared(root ast.Node, out *declaredNames) {
	walkNodes(root, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.ExternDecl:
			out.externs[node.Name] = true
		case *ast.FunctionDecl:
			out.values[node.Name] = true
			for _, p := range node.Params {
				out.values[p.Name] = true
			}
		case *ast.Lambda:
			for _, p := range node.Params {
				out.values[p.Name] = true
			}
		case *ast.VarDecl:
			out.values[node.Name] = true
		case *ast.ForStmt:
			if node.Var != "" {
				out.values[node.Var] = true
			}
		case *ast.StructDecl:
			out.values[node.Name] = true
			for _, f := range node.Fields {
				out.members[f.Name] = true
			}
		case *ast.ClassDecl:
			out.values[node.Name] = true
			for _, f := range node.Fields {
				out.members[f.Name] = true
			}
		case *ast.EnumDecl:
			out.values[node.Name] = true
			for _, v := range node.Variants {
				// A variant is written `Color.Red`, so it is both the thing
				// after a dot and, for the backends that flatten enums to
				// constants, a name in its own right.
				out.members[v] = true
				out.values[v] = true
			}
		}
	})
}

// renameNode rewrites n with renames applied, copying every node on the way
// down. Unlike lowerSwitch this does not try to share unchanged subtrees:
// renameReservedForTarget only calls it when there is at least one rename, and
// a program small enough to compile to thirty-eight targets is small enough to
// copy once.
func renameNode(n ast.Node, renames map[string]string) ast.Node {
	if n == nil {
		return nil
	}
	rename := func(name string) string {
		if to, ok := renames[name]; ok {
			return to
		}
		return name
	}
	list := func(in []ast.Node) []ast.Node {
		if in == nil {
			return nil
		}
		out := make([]ast.Node, len(in))
		for i, s := range in {
			out[i] = renameNode(s, renames)
		}
		return out
	}
	params := func(in []ast.Param) []ast.Param {
		out := make([]ast.Param, len(in))
		for i, p := range in {
			out[i] = ast.Param{Name: rename(p.Name), Type: renameType(p.Type, renames)}
		}
		return out
	}

	switch node := n.(type) {
	case *ast.Program:
		cp := *node
		cp.Decls = list(node.Decls)
		return &cp

	case *ast.ExternDecl:
		// An extern names a host symbol. Its own name stays, and so do its
		// parameter names, which are documentation rather than bindings.
		return node

	case *ast.ImportDecl:
		return node

	case *ast.FunctionDecl:
		cp := *node
		cp.Name = rename(node.Name)
		cp.Params = params(node.Params)
		cp.ReturnType = renameType(node.ReturnType, renames)
		cp.Body = list(node.Body)
		return &cp

	case *ast.Lambda:
		cp := *node
		cp.Params = params(node.Params)
		cp.ReturnType = renameType(node.ReturnType, renames)
		cp.Body = list(node.Body)
		return &cp

	case *ast.StructDecl:
		cp := *node
		cp.Name = rename(node.Name)
		cp.Fields = make([]ast.StructField, len(node.Fields))
		for i, f := range node.Fields {
			cp.Fields[i] = ast.StructField{
				Name:       rename(f.Name),
				Type:       renameType(f.Type, renames),
				Visibility: f.Visibility,
			}
		}
		return &cp

	case *ast.ClassDecl:
		cp := *node
		cp.Name = rename(node.Name)
		cp.Fields = make([]ast.ClassField, len(node.Fields))
		for i, f := range node.Fields {
			nf := f
			nf.Name = rename(f.Name)
			nf.Type = renameType(f.Type, renames)
			cp.Fields[i] = nf
		}
		return &cp

	case *ast.EnumDecl:
		cp := *node
		cp.Name = rename(node.Name)
		cp.Variants = make([]string, len(node.Variants))
		for i, v := range node.Variants {
			cp.Variants[i] = rename(v)
		}
		return &cp

	case *ast.VarDecl:
		cp := *node
		cp.Name = rename(node.Name)
		cp.Type = renameType(node.Type, renames)
		cp.Value = renameNode(node.Value, renames)
		return &cp

	case *ast.AssignStmt:
		return &ast.AssignStmt{
			Target: renameNode(node.Target, renames),
			Value:  renameNode(node.Value, renames),
		}

	case *ast.ReturnStmt:
		return &ast.ReturnStmt{Value: renameNode(node.Value, renames)}

	case *ast.ExprStmt:
		return &ast.ExprStmt{Expr: renameNode(node.Expr, renames)}

	case *ast.IfStmt:
		return &ast.IfStmt{
			Cond: renameNode(node.Cond, renames),
			Then: list(node.Then),
			Else: list(node.Else),
		}

	case *ast.WhileStmt:
		cp := *node
		cp.Cond = renameNode(node.Cond, renames)
		cp.Body = list(node.Body)
		return &cp

	case *ast.ForStmt:
		cp := *node
		cp.Var = rename(node.Var)
		cp.Start = renameNode(node.Start, renames)
		cp.End = renameNode(node.End, renames)
		cp.Iterable = renameNode(node.Iterable, renames)
		cp.Body = list(node.Body)
		return &cp

	case *ast.SwitchStmt:
		cp := *node
		cp.Value = renameNode(node.Value, renames)
		cp.Cases = make([]ast.SwitchCase, len(node.Cases))
		for i, c := range node.Cases {
			cp.Cases[i] = ast.SwitchCase{
				Value: renameNode(c.Value, renames),
				Body:  list(c.Body),
			}
		}
		return &cp

	case *ast.MatchExpr:
		cp := *node
		cp.Value = renameNode(node.Value, renames)
		cp.Arms = make([]ast.MatchArm, len(node.Arms))
		for i, arm := range node.Arms {
			cp.Arms[i] = ast.MatchArm{
				Pattern: renameNode(arm.Pattern, renames),
				Body:    list(arm.Body),
			}
		}
		return &cp

	case *ast.BinaryExpr:
		cp := *node
		cp.Left = renameNode(node.Left, renames)
		cp.Right = renameNode(node.Right, renames)
		return &cp

	case *ast.UnaryExpr:
		cp := *node
		cp.Operand = renameNode(node.Operand, renames)
		return &cp

	case *ast.CallExpr:
		cp := *node
		cp.Callee = renameCallee(node.Callee, renames)
		cp.Args = list(node.Args)
		return &cp

	case *ast.NewExpr:
		cp := *node
		cp.Callee = rename(node.Callee)
		cp.Args = list(node.Args)
		return &cp

	case *ast.MemberExpr:
		cp := *node
		cp.Object = renameNode(node.Object, renames)
		cp.Field = rename(node.Field)
		return &cp

	case *ast.IndexExpr:
		cp := *node
		cp.Target = renameNode(node.Target, renames)
		cp.Index = renameNode(node.Index, renames)
		return &cp

	case *ast.IfExpr:
		cp := *node
		cp.Cond = renameNode(node.Cond, renames)
		cp.Then = renameNode(node.Then, renames)
		cp.Else = renameNode(node.Else, renames)
		return &cp

	case *ast.AwaitExpr:
		cp := *node
		cp.Expr = renameNode(node.Expr, renames)
		return &cp

	case *ast.Ident:
		return &ast.Ident{Name: rename(node.Name)}

	case *ast.Literal:
		return node

	case *ast.ArrayLit:
		cp := *node
		cp.ElemType = renameType(node.ElemType, renames)
		cp.Elements = list(node.Elements)
		return &cp

	case *ast.ArrayLiteral:
		cp := *node
		cp.ElemType = renameType(node.ElemType, renames)
		cp.Elements = list(node.Elements)
		return &cp

	case *ast.MapLiteral:
		cp := *node
		cp.KeyType = renameType(node.KeyType, renames)
		cp.ValueType = renameType(node.ValueType, renames)
		cp.Entries = make([]ast.MapEntry, len(node.Entries))
		for i, e := range node.Entries {
			cp.Entries[i] = ast.MapEntry{
				Key:   renameNode(e.Key, renames),
				Value: renameNode(e.Value, renames),
			}
		}
		return &cp

	case *ast.StructLit:
		cp := *node
		cp.TypeName = rename(node.TypeName)
		cp.Fields = make([]ast.StructFieldInit, len(node.Fields))
		for i, f := range node.Fields {
			cp.Fields[i] = ast.StructFieldInit{
				Name:  rename(f.Name),
				Value: renameNode(f.Value, renames),
			}
		}
		return &cp
	}
	return n
}

// renameCallee renames a call target. A callee can be a dotted path — the
// parser keeps `obj.method` as one string — so each segment is renamed on its
// own, which leaves a host method alone while still renaming a field the
// program declared.
func renameCallee(callee string, renames map[string]string) string {
	if to, ok := renames[callee]; ok {
		return to
	}
	if !strings.Contains(callee, ".") {
		return callee
	}
	segs := strings.Split(callee, ".")
	for i, s := range segs {
		if to, ok := renames[s]; ok {
			segs[i] = to
		}
	}
	return strings.Join(segs, ".")
}

// renameType rewrites the user-defined type names inside a type expression.
// Builtin type names (Int, String, Array) are never in the rename map, because
// nothing declares them.
func renameType(t ast.TypeExpr, renames map[string]string) ast.TypeExpr {
	cp := t
	if to, ok := renames[t.KindName]; ok {
		cp.KindName = to
	}
	cp.Elem = renameTypePtr(t.Elem, renames)
	cp.KeyType = renameTypePtr(t.KeyType, renames)
	cp.OkType = renameTypePtr(t.OkType, renames)
	cp.ErrType = renameTypePtr(t.ErrType, renames)
	return cp
}

func renameTypePtr(t *ast.TypeExpr, renames map[string]string) *ast.TypeExpr {
	if t == nil {
		return nil
	}
	cp := renameType(*t, renames)
	return &cp
}
