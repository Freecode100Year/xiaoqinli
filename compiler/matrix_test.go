package compiler

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The advertised claim is one AST, thirty-eight targets. The test that used to
// stand behind it compiled an empty `main` to each backend, which every
// backend manages. This one runs the whole example corpus through every
// target and pins the result of all 460 combinations.
//
// Two things are asserted, and the second matters more than the first:
//
//   - A target that compiles an example must keep compiling it. A regression
//     shows up as a new rejection.
//   - A target that refuses an example must refuse it deliberately — code
//     XQL_E402, "this backend cannot express that construct". Anything else is
//     a crash wearing an error message, and the codes exist to tell those
//     apart.
//
// When a backend gains a construct, its entry here shrinks; the test failing
// is how you find out the docs need the same edit.

// rejected lists, per example, the targets that decline to compile it. Empty
// or absent means every target handles it.
var expectedRejections = map[string][]string{
	// This example returns Result<T, E>, which turns out to be the construct
	// fewest backends actually implement. Only twelve do; the rest declined
	// once a sweep established that they were emitting references to a Result
	// they never defined. See TestNoBackendFakesResultSupport.
	"e2e_workspace/main.xql": {
		"bat",               // no struct literals
		"fortran", "pascal", // no for-each

		// No Result<T>. Everything outside this list and the twelve that
		// implement it should be treated as unexamined, not as working.
		"awk", "bash", "c", "chrome", "cpp", "crystal", "d",
		"elixir", "groovy", "haskell", "js", "nim",
		"ocaml", "perl", "powershell", "shortcut", "tccli",
		"tcl", "vala",
	},

	// A struct literal is the one thing the batch-file backend cannot fake.
	// tccli emits Tencent Cloud CLI invocations and has no way to represent a
	// struct; it used to compile one to an empty string.
	// awk joins them: it lowered the literal to 0, losing the field values, and
	// then indexed a scalar. Arrays cannot cross an awk function boundary, so
	// partial support would only move the failure.
	"struct.xql.json": {"awk", "bat", "tccli"},

	// tccli has no arithmetic, no comparisons, no arrays, no lambdas. It was
	// accepting all of them and emitting shell that either printed blank lines
	// or passed a minus sign as a command-line argument.
	"collections.xql.json":         {"tccli"},
	"loop.xql.json":                {"tccli"},
	"int_arithmetic.xql.json":      {"tccli"},
	"negative_arithmetic.xql.json": {"tccli"},
	"string_compare.xql.json":      {"tccli"},
	"lambda_ifexpr.xql.json":       {"tccli"},
	"example.xql.json":             {"tccli"},
	"nested_loop.xql.json":         {"tccli"},
	"int_width.xql.json":           {"tccli"},

	// The each form, which is the line the matrix splits along here. c has no
	// array length to iterate; fortran and pascal declare the limit outright;
	// and bat has no array *values* at all — it fakes arrays with variables
	// named nums[0], nums[1], which indexing can reach by writing the index
	// into the name and iteration cannot reach at all.
	"for_each.xql.json": {"bat", "c", "fortran", "pascal", "tccli"},

	// `!` is a UnaryExpr, which the batch and tccli backends have never
	// emitted; both decline it rather than guess.
	"bool_logic.xql.json":   {"bat", "tccli"},
	"precedence.xql.json":   {"tccli"},
	"string_build.xql.json": {"tccli"},

	// A struct crossing a function boundary. awk arrays cannot be passed by
	// value or returned at all, which is why awk already declines the struct
	// example; bat has no struct literals; tccli has no member access.
	"struct_arg.xql.json": {"awk", "bat", "tccli"},

	// `break` is the construct that splits the matrix along a different line
	// than Result does: a language without an early loop exit cannot fake one.
	// All six declined with XQL_E401 until this example existed, which claimed
	// a compiler bug for what is a capability limit — the distinction the two
	// codes exist to draw.
	"control_flow.xql.json": {
		"bat", "elixir", "haskell", "ocaml",
		"tccli", // no while loop either
	},

	// The chrome examples declare host externs restricted to browser targets,
	// so every backend that cannot provide them is refused by design. This is
	// XQL_E402 doing exactly the job it was added for.
	"chrome_network.xql.json": allTargetsExcept("chrome"),
	"chrome_volume.xql.json":  allTargetsExcept("chrome", "js", "ts"),

	// clock.xql.json declares a Go-only time extern.
	"clock.xql.json": allTargetsExcept("go"),
}

func allTargetsExcept(keep ...string) []string {
	kept := map[string]bool{}
	for _, k := range keep {
		kept[k] = true
	}
	var out []string
	for _, flag := range GetSupportedTargets() {
		if !kept[flag] {
			out = append(out, flag)
		}
	}
	sort.Strings(out)
	return out
}

// exampleCorpus returns every example, keyed by the name used above.
func exampleCorpus(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "examples")
	out := map[string]string{}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("cannot read examples: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".xql.json") {
			out[e.Name()] = filepath.Join(root, e.Name())
		}
	}
	out["e2e_workspace/main.xql"] = filepath.Join(root, "e2e_workspace", "main.xql")

	if len(out) < 5 {
		t.Fatalf("expected the example corpus to have real content, found %d", len(out))
	}
	return out
}

func TestExampleTargetMatrix(t *testing.T) {
	corpus := exampleCorpus(t)
	targets := GetSupportedTargets()
	sort.Strings(targets)

	names := make([]string, 0, len(corpus))
	for name := range corpus {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		path := corpus[name]

		want := map[string]bool{}
		for _, flag := range expectedRejections[name] {
			want[flag] = true
		}

		t.Run(name, func(t *testing.T) {
			var got []string
			for _, target := range targets {
				res := CompileFromFile(path, target, "")
				if res.Success {
					if want[target] {
						t.Errorf("%s now compiles to %s — the backend gained a construct, "+
							"so drop it from expectedRejections and from the README's limitation table",
							name, target)
					}
					if len(res.Code) == 0 && len(res.Files) == 0 {
						t.Errorf("%s -> %s reported success but produced nothing", name, target)
					}
					continue
				}

				got = append(got, target)
				if !want[target] {
					t.Errorf("%s no longer compiles to %s: %s", name, target, res.Error)
					continue
				}
				// A declared limitation is XQL_E402. Any other code here means
				// the backend fell over rather than declining.
				if res.ErrorCode != "XQL_E402" {
					t.Errorf("%s -> %s should decline with XQL_E402, got %s: %s",
						name, target, res.ErrorCode, res.Error)
				}
			}

			for flag := range want {
				if !containsStr(got, flag) {
					t.Errorf("expectedRejections lists %s for %s, but it compiled fine", flag, name)
				}
			}
		})
	}
}

// TestMatrixCoversEveryAdvertisedTarget stops the matrix from quietly shrinking:
// GetSupportedTargets is what the matrix iterates, so a target dropped from
// there would vanish from the test with nothing to show for it.
func TestMatrixCoversEveryAdvertisedTarget(t *testing.T) {
	if n := len(GetSupportedTargets()); n < 38 {
		t.Fatalf("the matrix should cover all 38 advertised targets, GetSupportedTargets returned %d", n)
	}
}

func containsStr(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}
