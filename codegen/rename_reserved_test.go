package codegen

import (
	"os"
	"strings"
	"testing"

	"xiaoqinli/ast"
)

// examples/reserved_names.xql.json is the corpus side of this: a function
// named `end`, a parameter named `in`, a variable named `class`, run through
// every backend the conformance suite can execute. These tests ask what
// running it cannot — that the pass leaves the caller's tree alone, that it
// renames the declaration and every reference together, and that it keeps its
// hands off the names the program does not own.

func loadReservedExample(t *testing.T) ast.Node {
	t.Helper()
	data, err := os.ReadFile("../examples/reserved_names.xql.json")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	root, err := ast.Parse(data)
	if err != nil {
		t.Fatalf("parse example: %v", err)
	}
	return root
}

// Generate is called once per target over one parsed AST. A rename that
// mutated the tree would leave the next target compiling the previous
// target's renames — python would emit `class_` because pascal had been asked
// for first.
func TestRenameDoesNotMutateCallerAST(t *testing.T) {
	root := loadReservedExample(t)

	if _, err := Generate(root, "py"); err != nil {
		t.Fatalf("py: %v", err)
	}

	names := map[string]bool{}
	walkNodes(root, func(n ast.Node) {
		if fd, ok := n.(*ast.FunctionDecl); ok {
			names[fd.Name] = true
			for _, p := range fd.Params {
				names[p.Name] = true
			}
		}
	})
	if !names["end"] || !names["in"] {
		t.Fatalf("compiling renamed the caller's AST: names are now %v", names)
	}
}

// The declaration and the references have to move together. A rename that
// reached one and not the other would produce a program that parses and does
// not link, which is worse than the one that did not parse.
func TestRenameReachesDeclarationAndReferences(t *testing.T) {
	root := loadReservedExample(t)

	code, err := Generate(root, "py")
	if err != nil {
		t.Fatalf("py: %v", err)
	}
	out := string(code)

	for _, want := range []string{"def end(", "in_", "class_"} {
		if !strings.Contains(out, want) {
			t.Fatalf("python output is missing %q:\n%s", want, out)
		}
	}
	// `class` is a keyword in Python and `end` is not, so exactly one of the
	// two functions should have been touched.
	for _, unwanted := range []string{"end_", " class ", "class ="} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("python output contains %q, which should have been renamed or left alone:\n%s", unwanted, out)
		}
	}
}

// Pascal is the language that started this: `label` is a keyword there and an
// ordinary name everywhere else, and fpc refused the whole file over it.
func TestRenameRewritesPascalKeyword(t *testing.T) {
	data, err := os.ReadFile("../examples/switch_stmt.xql.json")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	root, err := ast.Parse(data)
	if err != nil {
		t.Fatalf("parse example: %v", err)
	}

	code, err := Generate(root, "pascal")
	if err != nil {
		t.Fatalf("pascal: %v", err)
	}
	out := string(code)
	if !strings.Contains(out, "function label_(") {
		t.Fatalf("pascal still declares a function named label:\n%s", out)
	}
	if strings.Contains(out, "label_(1)") == false {
		t.Fatalf("pascal renamed the declaration and not the calls:\n%s", out)
	}

	// Case does not matter in Pascal, so a program that spells it `Label`
	// collides just as hard.
	upper, err := ast.Parse([]byte(strings.ReplaceAll(string(data), `"label"`, `"Label"`)))
	if err != nil {
		t.Fatalf("parse example: %v", err)
	}
	code, err = Generate(upper, "pascal")
	if err != nil {
		t.Fatalf("pascal: %v", err)
	}
	if !strings.Contains(string(code), "function Label_(") {
		t.Fatalf("a case-insensitive language missed a capitalised keyword:\n%s", code)
	}
}

// An extern is the name of something the host provides. Renaming it would
// break the call rather than fix it, so a collision there is the program's to
// resolve, not this pass's.
func TestRenameLeavesExternsAlone(t *testing.T) {
	root := &ast.Program{Decls: []ast.Node{
		&ast.ExternDecl{Name: "class", HasParams: true, ReturnType: ast.TypeExpr{KindName: "Void"}, Grant: []string{"io"}},
		&ast.FunctionDecl{
			Name:       "main",
			ReturnType: ast.TypeExpr{KindName: "Void"},
			Grant:      []string{"io"},
			Body: []ast.Node{
				&ast.ExprStmt{Expr: &ast.CallExpr{Callee: "class"}},
			},
		},
	}}

	renames := collectReservedRenames(root, reservedByLanguage["py"], "_", false)
	if to, ok := renames["class"]; ok {
		t.Fatalf("an extern was renamed to %q", to)
	}
}

// A struct field is a name the program does own, and it is spelled in two
// places — the declaration and every access through a dot.
func TestRenameRewritesStructFields(t *testing.T) {
	root := &ast.Program{Decls: []ast.Node{
		&ast.StructDecl{Name: "Box", Fields: []ast.StructField{
			{Name: "class", Type: ast.TypeExpr{KindName: "Int"}},
		}},
		&ast.FunctionDecl{
			Name:       "main",
			ReturnType: ast.TypeExpr{KindName: "Void"},
			Grant:      []string{"io"},
			Body: []ast.Node{
				&ast.VarDecl{
					Name: "b",
					Type: ast.TypeExpr{KindName: "Box"},
					Value: &ast.StructLit{TypeName: "Box", Fields: []ast.StructFieldInit{
						{Name: "class", Value: &ast.Literal{ValueType: "Int", Value: float64(1)}},
					}},
				},
				&ast.ExprStmt{Expr: &ast.CallExpr{Callee: "println", Args: []ast.Node{
					&ast.MemberExpr{Object: &ast.Ident{Name: "b"}, Field: "class"},
				}}},
			},
		},
	}}

	code, err := Generate(root, "java")
	if err != nil {
		t.Fatalf("java: %v", err)
	}
	out := string(code)
	// `class_` contains `class`, so the check has to name the punctuation that
	// would follow the field had it not been renamed.
	if strings.Contains(out, "long class;") || strings.Contains(out, "b.class)") {
		t.Fatalf("java output still spells a field `class`:\n%s", out)
	}
	if !strings.Contains(out, "class_") {
		t.Fatalf("java output never renamed the field:\n%s", out)
	}
}

// A target whose output is JSON or a CLI invocation has no keyword table, and
// should come back the same node it went in as.
func TestRenameSkipsTargetsWithoutKeywords(t *testing.T) {
	root := loadReservedExample(t)
	for _, target := range []string{"shortcut", "tccli"} {
		if out := renameReservedForTarget(root, target); out != root {
			t.Fatalf("%s has no keyword table but its tree was rewritten", target)
		}
	}
}

// The replacement has to be a name the program is not already using, and one
// the language does not reserve either.
func TestRenameAvoidsCollidingWithAnExistingName(t *testing.T) {
	root := &ast.Program{Decls: []ast.Node{
		&ast.FunctionDecl{
			Name:       "main",
			ReturnType: ast.TypeExpr{KindName: "Void"},
			Grant:      []string{"io"},
			Body: []ast.Node{
				&ast.VarDecl{Name: "class", Type: ast.TypeExpr{KindName: "Int"}, Value: &ast.Literal{ValueType: "Int", Value: float64(1)}},
				&ast.VarDecl{Name: "class_", Type: ast.TypeExpr{KindName: "Int"}, Value: &ast.Literal{ValueType: "Int", Value: float64(2)}},
			},
		},
	}}

	renames := collectReservedRenames(root, reservedByLanguage["py"], "_", false)
	if renames["class"] != "class__" {
		t.Fatalf("rename picked %q, which the program already uses", renames["class"])
	}
}

// Nim rejects a trailing underscore and compares identifiers ignoring the
// underscores it does accept, so the default suffix is both illegal and
// pointless there. CI found this one: "invalid token: trailing underscore".
func TestRenameSuffixFollowsTheLanguage(t *testing.T) {
	root := loadReservedExample(t)

	code, err := Generate(root, "nim")
	if err != nil {
		t.Fatalf("nim: %v", err)
	}
	out := string(code)
	if strings.Contains(out, "_(") || strings.Contains(out, "_:") {
		t.Fatalf("nim output renamed something to a trailing underscore:\n%s", out)
	}
	if !strings.Contains(out, "endX") {
		t.Fatalf("nim output never renamed `end`, which is a keyword there:\n%s", out)
	}
}

// PHP is the case where a name is taken without being reserved: `end` is a
// builtin, and redeclaring one is a fatal error rather than a parse error.
func TestRenameCoversPHPBuiltins(t *testing.T) {
	root := loadReservedExample(t)

	code, err := Generate(root, "php")
	if err != nil {
		t.Fatalf("php: %v", err)
	}
	if strings.Contains(string(code), "function end(") {
		t.Fatalf("php redeclares the builtin end():\n%s", code)
	}
}
