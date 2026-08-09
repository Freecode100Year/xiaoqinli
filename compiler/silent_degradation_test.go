package compiler

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// "A backend that cannot express a construct rejects it rather than silently
// degrading it" is the README's claim and the reason XQL_E402 exists. Three
// backends were not doing it, each in its own way, and all three reported
// success:
//
//   - tccli turned every unrecognised expression into '' and dropped every
//     unrecognised statement, so a struct literal became an empty string and a
//     whole program could compile down to printing blank lines
//   - android wrote the AST node's kind into a Kotlin comment, leaving
//     `/* IfExpr */` where a value belonged
//   - twenty-two backends emitted Result references with no Result behind them
//
// The first two shapes are what this test looks for. The third has its own
// test, because it needs a different question.

// astKindNames are node kinds a placeholder would name. A backend that writes
// one of these into its output is describing the AST rather than translating
// it.
var astKindNames = []string{
	"StructLit", "IfExpr", "MatchExpr", "Lambda", "ArrayLit", "ArrayLiteral",
	"MapLiteral", "IndexExpr", "MemberExpr", "BinaryExpr", "UnaryExpr",
	"CallExpr", "VarDecl", "AssignStmt", "SwitchStmt", "WhileStmt", "ForStmt",
}

func TestNoBackendEmitsAstPlaceholders(t *testing.T) {
	corpus := exampleCorpus(t)
	names := make([]string, 0, len(corpus))
	for n := range corpus {
		names = append(names, n)
	}
	sort.Strings(names)

	targets := GetSupportedTargets()
	sort.Strings(targets)

	for _, target := range targets {
		for _, name := range names {
			res := CompileFromFile(corpus[name], target, "")
			if !res.Success {
				continue
			}
			code := string(res.Code)
			for _, c := range res.Files {
				code += "\n" + string(c)
			}
			for _, kind := range astKindNames {
				if strings.Contains(code, kind) {
					t.Errorf("%s -> %s emits the AST node name %q into its output, "+
						"which means an unhandled construct was written out as a placeholder "+
						"instead of failing with XQL_E402:\n%s",
						name, target, kind, code)
				}
			}
		}
	}
}

// TestGeneratedOutputIsNotVacuous catches the other half of tccli's behaviour:
// output that is syntactically fine and says nothing, because every expression
// it could not handle collapsed to an empty string.
func TestGeneratedOutputIsNotVacuous(t *testing.T) {
	// A program that prints two struct fields. Any backend accepting it has to
	// mention the values somewhere; tccli used to emit `echo ''` twice.
	path := filepath.Join("..", "examples", "struct.xql.json")

	targets := GetSupportedTargets()
	sort.Strings(targets)

	for _, target := range targets {
		res := CompileFromFile(path, target, "")
		if !res.Success {
			continue
		}
		code := string(res.Code)
		for _, c := range res.Files {
			code += "\n" + string(c)
		}
		// The struct's field values are 3 and 5. A backend that lowered the
		// literal away entirely will not have them.
		if !strings.Contains(code, "3") || !strings.Contains(code, "5") {
			t.Errorf("%s accepted struct.xql.json but its output contains neither field value, "+
				"so the struct literal was lowered to nothing:\n%s", target, code)
		}
	}
}
