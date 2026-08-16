package codegen

import (
	"strings"
	"testing"
)

// An enum, and a reference to one of its variants. examples/enum_match.xql.json
// is the same program; this file asserts the text for the targets whose
// toolchains are not on a given developer machine, and for the two — shortcut
// and chrome — that have no toolchain anywhere.
//
// EnumDecl had an emitter in thirty-five backends. A *reference* to a variant
// had never appeared in the corpus, so the two sides had never been compared:
// the declaration picked a spelling and `Color.Red` went out through
// emitMemberExpr, which is written for field access on a value. See
// docs/adr_enum_ref.md.
const enumRefProgram = `{
	"kind": "Program",
	"declarations": [
		{"kind": "EnumDecl", "name": "Color", "variants": ["Red", "Green", "Blue"]},
		{
			"kind": "FunctionDecl",
			"name": "describe",
			"params": [{"name": "c", "type": {"kind": "Color"}}],
			"returnType": {"kind": "String"},
			"effects": [],
			"body": [
				{
					"kind": "VarDecl",
					"name": "tag",
					"type": {"kind": "String"},
					"value": {"kind": "Literal", "valueType": "String", "value": "none"}
				},
				{
					"kind": "MatchExpr",
					"value": {"kind": "Ident", "name": "c"},
					"arms": [
						{
							"pattern": {"kind": "MemberExpr", "object": {"kind": "Ident", "name": "Color"}, "field": "Red"},
							"body": [{
								"kind": "AssignStmt",
								"target": "tag",
								"value": {"kind": "Literal", "valueType": "String", "value": "red"}
							}]
						},
						{
							"pattern": {"kind": "Ident", "name": "_"},
							"body": [{
								"kind": "AssignStmt",
								"target": "tag",
								"value": {"kind": "Literal", "valueType": "String", "value": "other"}
							}]
						}
					]
				},
				{"kind": "ReturnStmt", "value": {"kind": "Ident", "name": "tag"}}
			]
		},
		{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": ["state"],
			"grant": ["io"],
			"body": [{
				"kind": "ExprStmt",
				"expr": {
					"kind": "CallExpr",
					"callee": "println",
					"args": [{
						"kind": "CallExpr",
						"callee": "describe",
						"args": [{"kind": "MemberExpr", "object": {"kind": "Ident", "name": "Color"}, "field": "Green"}]
					}]
				}
			}]
		}
	]
}`

// Every target that accepts the program, and the spelling its own emitEnumDecl
// obliges it to use. `Color.Red` is in the reject column wherever the backend
// does not define a Color to have a Red on — which was all twenty-two of the
// wrong ones, spelled identically and wrong for a different reason each time.
func TestEnumVariantReferenceMatchesDeclaration(t *testing.T) {
	root := mustParse(t, enumRefProgram)

	cases := []struct {
		target string
		want   []string
		reject []string
	}{
		{target: "go", want: []string{"ColorRed", "describe(ColorGreen)"}, reject: []string{"Color.Red", "Color.red"}},
		{target: "c", want: []string{"case Color_Red:", "describe(Color_Green)"}, reject: []string{"Color.Red"}},
		{target: "rust", want: []string{"Color::Red =>", "describe(Color::Green)"}, reject: []string{"Color.Red"}},
		{target: "cpp", want: []string{"case Color::Red:"}, reject: []string{"Color.Red"}},

		// The variants are i64 constants, so the parameter is i64 too — the
		// signature used to name a type this backend never declares.
		{target: "zig", want: []string{"ColorRed =>", "c: i64"}, reject: []string{"Color.Red", "c: Color"}},

		// `@enum Color Red Green Blue` binds the variants in the enclosing
		// scope; Color is a type, not a module.
		{target: "julia", want: []string{"Red", "describe(Green)"}, reject: []string{"Color.Red"}},

		// A switch label has to be unqualified and everything else qualified,
		// which is why java is the one backend that spells a variant two ways.
		{target: "java", want: []string{"case Red:", "describe(Color.Green)"}, reject: []string{"case Color.Red:"}},

		// Integer constants at file scope, so the parameter is int.
		{target: "php", want: []string{"case Red:", "describe(Green)", "int $c"}, reject: []string{"Color $c", "Color->"}},

		{target: "lua", want: []string{"describe(Green)"}, reject: []string{"Color.Red"}},
		{target: "perl", want: []string{"describe(Green)"}, reject: []string{"$Color->{Red}"}},
		{target: "awk", want: []string{"describe(Green)"}, reject: []string{"Color[\"Red\"]"}},

		// A proc does not inherit globals, so the variant is fully qualified;
		// and a switch does not substitute variables in its patterns, so the
		// match lowers to an if chain.
		{target: "tcl", want: []string{"$::Red", "describe $::Green"}, reject: []string{"dict get $Color"}},
		{target: "bash", want: []string{"${Green}", "$Red)"}, reject: []string{"Color[Red]"}},
		{target: "powershell", want: []string{"[Color]::Red", "describe ([Color]::Green)"}, reject: []string{"$Color.Red"}},

		{target: "ruby", want: []string{"Color::Red"}, reject: []string{"Color.Red"}},
		{target: "crystal", want: []string{"Color::Red"}, reject: []string{"Color.Red"}},

		// A variant constructor is a name in the enclosing scope, not a module
		// path.
		{target: "ocaml", want: []string{"| Red ->"}, reject: []string{"Color.Red"}},
		{target: "pascal", want: []string{"Red:"}, reject: []string{"Color.Red"}},

		// integer(8), because Int is 64-bit here and a plain `integer` is
		// INTEGER(4) — gfortran rejects the call as a type mismatch.
		{target: "fortran", want: []string{"integer(8), parameter :: Color_Red", "describe(Color_Green)"}, reject: []string{"type(Color)", "Color%"}},

		// The declaration is a comment; the atom is the value.
		{target: "elixir", want: []string{":red ->", "describe(:green)"}, reject: []string{"Color.Red"}},

		// Shortcuts sets one variable per variant, named Color_Red.
		{target: "shortcut", want: []string{"Color_Green"}, reject: []string{`"VariableName": "Color"`}},

		// The targets that were right all along, kept here so that staying
		// right is also asserted.
		{target: "py", want: []string{"case Color.Red:"}},
		{target: "js", want: []string{"case Color.Red:"}},
		{target: "ts", want: []string{"case Color.Red:"}},
		{target: "csharp", want: []string{"case Color.Red:"}},
		{target: "kotlin", want: []string{"Color.Red ->"}},
		{target: "swift", want: []string{"case Color.Red:"}},
		{target: "dart", want: []string{"case Color.Red:"}},
		{target: "nim", want: []string{"of Color.Red:"}},
		{target: "d", want: []string{"case Color.Red:"}},
		{target: "vala", want: []string{"case Color.Red:"}},
		{target: "groovy", want: []string{"case Color.Red:"}},
	}

	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			out, err := Generate(root, tc.target)
			if err != nil {
				t.Fatalf("%s declined a program it advertises support for: %v", tc.target, err)
			}
			src := string(out)
			for _, want := range tc.want {
				if !strings.Contains(src, want) {
					t.Errorf("%s should reference the variant as %q:\n%s", tc.target, want, src)
				}
			}
			for _, reject := range tc.reject {
				if strings.Contains(src, reject) {
					t.Errorf("%s emits %q, which its own enum declaration does not define:\n%s", tc.target, reject, src)
				}
			}
		})
	}
}

// A struct field that happens to share a name with an enum variant is still a
// field. enumRef only fires when the object is a bare identifier naming a
// declared enum, and this is the program that says so.
func TestEnumRefDoesNotSwallowFieldAccess(t *testing.T) {
	const shadowed = `{
		"kind": "Program",
		"declarations": [
			{"kind": "EnumDecl", "name": "Color", "variants": ["Red"]},
			{
				"kind": "StructDecl",
				"name": "Paint",
				"fields": [{"name": "Red", "type": {"kind": "Int"}, "visibility": "public"}]
			},
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": ["state"],
				"grant": ["io"],
				"body": [
					{
						"kind": "VarDecl",
						"name": "p",
						"type": {"kind": "Paint"},
						"value": {
							"kind": "StructLit",
							"typeName": "Paint",
							"fields": [{"name": "Red", "value": {"kind": "Literal", "valueType": "Int", "value": 7}}]
						}
					},
					{
						"kind": "ExprStmt",
						"expr": {
							"kind": "CallExpr",
							"callee": "println",
							"args": [{"kind": "MemberExpr", "object": {"kind": "Ident", "name": "p"}, "field": "Red"}]
						}
					}
				]
			}
		]
	}`

	out, err := Generate(mustParse(t, shadowed), "go")
	if err != nil {
		t.Fatalf("go declined a field access: %v", err)
	}
	if src := string(out); !strings.Contains(src, "p.Red") {
		t.Errorf("p.Red is a field on a value, not the enum variant ColorRed:\n%s", src)
	}
}
