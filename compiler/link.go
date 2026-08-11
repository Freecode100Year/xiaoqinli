package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xiaoqinli/ast"
)

// FlattenImports resolves the import graph rooted at entryFile and merges every
// reachable module into a single Program.
//
// Backends generate one output file and none of them emit the source of an
// imported module, so a program split across files used to compile to code that
// referenced declarations that were never written. Flattening removes the
// problem at the AST level instead of teaching every backend to walk imports:
// after this pass every backend sees a single self-contained Program with no
// ImportDecl, which is the path they already handle.
//
// Merging is safe because the type checker rejects duplicate global symbols
// across files (checkGlobalSymbolConflicts), so the merged namespace cannot
// collide. Imported declarations are emitted before the entry file's own, in
// dependency order, for targets where declaration order matters.
//
// Qualified references (alias.Fn, alias.Type) are rewritten to their bare names
// because the merged program has a single namespace.
func FlattenImports(root ast.Node, entryFile string) (ast.Node, error) {
	prog, ok := root.(*ast.Program)
	if !ok {
		return root, nil
	}
	if !hasImports(prog) {
		return root, nil
	}
	if entryFile == "" {
		return nil, fmt.Errorf("XQL_E404: cannot resolve imports without an entry file path")
	}

	l := &linker{
		loaded:  make(map[string]bool),
		aliases: make(map[string]bool),
		externs: make(map[string]bool),
	}
	imported, err := l.collect(prog, entryFile, map[string]bool{cleanPath(entryFile): true})
	if err != nil {
		return nil, err
	}

	merged := &ast.Program{}
	merged.Decls = append(merged.Decls, imported...)
	for _, d := range prog.Decls {
		if _, isImport := d.(*ast.ImportDecl); isImport {
			continue
		}
		merged.Decls = append(merged.Decls, d)
	}

	decls, err := l.dedupeExterns(merged.Decls)
	if err != nil {
		return nil, err
	}
	merged.Decls = decls

	for _, d := range merged.Decls {
		l.stripNode(d)
	}
	return merged, nil
}

// dedupeExterns collapses the same host function declared in several modules
// into one declaration, and rejects declarations that disagree. It also records
// every extern name so stripName leaves host calls alone: an extern called
// "models.load" is one verbatim host name, not a reference into module "models".
func (l *linker) dedupeExterns(decls []ast.Node) ([]ast.Node, error) {
	seen := make(map[string]*ast.ExternDecl)
	out := make([]ast.Node, 0, len(decls))
	for _, d := range decls {
		ed, ok := d.(*ast.ExternDecl)
		if !ok {
			out = append(out, d)
			continue
		}
		if !ed.Method {
			// Method externs are matched by their final segment, and the type
			// checker resolves import aliases ahead of them, so alias
			// stripping stays authoritative for those.
			l.externs[ed.Name] = true
		}
		if prev, dup := seen[ed.Name]; dup {
			if !prev.SignatureEquals(ed) {
				return nil, fmt.Errorf(
					"XQL_E202: extern %q is declared with conflicting signatures across modules", ed.Name)
			}
			continue
		}
		seen[ed.Name] = ed
		out = append(out, d)
	}
	return out, nil
}

type linker struct {
	// loaded dedups a module reachable by more than one path (diamond imports).
	loaded map[string]bool
	// aliases holds every import alias seen anywhere in the graph; qualified
	// references are stripped only when the prefix matches one of these, so a
	// variable method call like res.unwrap is left alone.
	aliases map[string]bool
	// externs holds every declared host function name; these are matched
	// verbatim and must survive alias stripping intact.
	externs map[string]bool
}

// collect walks the import graph depth-first and returns the declarations of
// every imported module, dependencies first. visiting carries the current DFS
// path so a cycle is reported rather than followed forever.
func (l *linker) collect(prog *ast.Program, currentFile string, visiting map[string]bool) ([]ast.Node, error) {
	var out []ast.Node
	for _, d := range prog.Decls {
		imp, ok := d.(*ast.ImportDecl)
		if !ok {
			continue
		}
		if imp.As != "" {
			l.aliases[imp.As] = true
		}
		depPath := resolveImportPath(currentFile, imp.Path)
		if visiting[depPath] {
			return nil, fmt.Errorf("XQL_E402: circular import detected: %s", depPath)
		}
		if l.loaded[depPath] {
			continue
		}
		l.loaded[depPath] = true

		data, err := os.ReadFile(depPath)
		if err != nil {
			return nil, fmt.Errorf("XQL_E404: failed to load import %q (%s): %w", imp.Path, depPath, err)
		}
		depNode, err := ast.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("XQL_E101: failed to parse import %q: %w", imp.Path, err)
		}
		depProg, ok := depNode.(*ast.Program)
		if !ok {
			return nil, fmt.Errorf("XQL_E101: imported file %q is not a valid Program", imp.Path)
		}

		nextVisiting := make(map[string]bool, len(visiting)+1)
		for k, v := range visiting {
			nextVisiting[k] = v
		}
		nextVisiting[depPath] = true

		nested, err := l.collect(depProg, depPath, nextVisiting)
		if err != nil {
			return nil, err
		}
		out = append(out, nested...)
		for _, dd := range depProg.Decls {
			if _, isImport := dd.(*ast.ImportDecl); isImport {
				continue
			}
			out = append(out, dd)
		}
	}
	return out, nil
}

func hasImports(prog *ast.Program) bool {
	for _, d := range prog.Decls {
		if _, ok := d.(*ast.ImportDecl); ok {
			return true
		}
	}
	return false
}

func cleanPath(p string) string { return filepath.Clean(p) }

// resolveImportPath mirrors check.resolveRelativePath: imports resolve against
// the directory of the file that declares them.
func resolveImportPath(current, target string) string {
	if current == "" {
		return filepath.Clean(target)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), target))
}

// stripName removes a leading "alias." when alias is a known import alias.
func (l *linker) stripName(name string) string {
	if l.externs[name] {
		return name
	}
	idx := strings.Index(name, ".")
	if idx <= 0 {
		return name
	}
	if l.aliases[name[:idx]] {
		return name[idx+1:]
	}
	return name
}

func (l *linker) stripType(t *ast.TypeExpr) {
	if t == nil {
		return
	}
	t.KindName = l.stripName(t.KindName)
	l.stripType(t.Elem)
	l.stripType(t.KeyType)
	l.stripType(t.OkType)
	l.stripType(t.ErrType)
}

func (l *linker) stripNodes(nodes []ast.Node) {
	for _, n := range nodes {
		l.stripNode(n)
	}
}

func (l *linker) stripParams(params []ast.Param) {
	for i := range params {
		l.stripType(&params[i].Type)
	}
}

// stripNode rewrites qualified references throughout the tree. Every node kind
// that carries a child, a type, or a callee name is handled; anything else is
// a leaf with nothing to rewrite.
func (l *linker) stripNode(n ast.Node) {
	switch node := n.(type) {
	case nil:
		return
	case *ast.Program:
		l.stripNodes(node.Decls)
	case *ast.FunctionDecl:
		l.stripParams(node.Params)
		l.stripType(&node.ReturnType)
		l.stripNodes(node.Body)
	case *ast.ExternDecl:
		// The name is a verbatim host symbol and is never rewritten.
		l.stripParams(node.Params)
		l.stripType(&node.ReturnType)
	case *ast.StructDecl:
		for i := range node.Fields {
			l.stripType(&node.Fields[i].Type)
		}
	case *ast.ClassDecl:
		for i := range node.Fields {
			l.stripType(&node.Fields[i].Type)
		}
	case *ast.VarDecl:
		l.stripType(&node.Type)
		l.stripNode(node.Value)
	case *ast.AssignStmt:
		l.stripNode(node.Target)
		l.stripNode(node.Value)
	case *ast.ReturnStmt:
		l.stripNode(node.Value)
	case *ast.ExprStmt:
		l.stripNode(node.Expr)
	case *ast.IfStmt:
		l.stripNode(node.Cond)
		l.stripNodes(node.Then)
		l.stripNodes(node.Else)
	case *ast.WhileStmt:
		l.stripNode(node.Cond)
		l.stripNodes(node.Body)
	case *ast.ForStmt:
		l.stripNode(node.Start)
		l.stripNode(node.End)
		l.stripNode(node.Iterable)
		l.stripNodes(node.Body)
	case *ast.SwitchStmt:
		l.stripNode(node.Value)
		for i := range node.Cases {
			l.stripNode(node.Cases[i].Value)
			l.stripNodes(node.Cases[i].Body)
		}
	case *ast.MatchExpr:
		l.stripNode(node.Value)
		for i := range node.Arms {
			l.stripNode(node.Arms[i].Pattern)
			l.stripNodes(node.Arms[i].Body)
		}
	case *ast.CallExpr:
		node.Callee = l.stripName(node.Callee)
		l.stripNodes(node.Args)
	case *ast.NewExpr:
		node.Callee = l.stripName(node.Callee)
		l.stripNodes(node.Args)
	case *ast.StructLit:
		node.TypeName = l.stripName(node.TypeName)
		for i := range node.Fields {
			l.stripNode(node.Fields[i].Value)
		}
	case *ast.BinaryExpr:
		l.stripNode(node.Left)
		l.stripNode(node.Right)
	case *ast.UnaryExpr:
		l.stripNode(node.Operand)
	case *ast.MemberExpr:
		l.stripNode(node.Object)
	case *ast.IndexExpr:
		l.stripNode(node.Target)
		l.stripNode(node.Index)
	case *ast.IfExpr:
		l.stripNode(node.Cond)
		l.stripNode(node.Then)
		l.stripNode(node.Else)
	case *ast.AwaitExpr:
		l.stripNode(node.Expr)
	case *ast.ArrayLit:
		l.stripType(&node.ElemType)
		l.stripNodes(node.Elements)
	case *ast.ArrayLiteral:
		l.stripType(&node.ElemType)
		l.stripNodes(node.Elements)
	case *ast.MapLiteral:
		l.stripType(&node.KeyType)
		l.stripType(&node.ValueType)
		for i := range node.Entries {
			l.stripNode(node.Entries[i].Key)
			l.stripNode(node.Entries[i].Value)
		}
	case *ast.Lambda:
		l.stripParams(node.Params)
		l.stripType(&node.ReturnType)
		l.stripNodes(node.Body)
	}
}
