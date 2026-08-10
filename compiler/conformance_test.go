package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"xiaoqinli/internal/e2e"
)

// One AST, forty-six targets — the claim is not that forty-six backends emit
// something, it is that they emit the *same program*. Nothing tested that.
//
// TestExampleTargetMatrix asks whether codegen returns bytes. TestCompiledTier
// asks whether a compiler accepts those bytes. Both were satisfied by programs
// that ran and printed the wrong answer, and several did:
//
//   - examples/loop.xql.json sums nums[0..4] and prints 15. ForStmt's range form
//     is documented exclusive of its end (ast/nodes.go), and twenty-three
//     backends implemented it that way. Nine did not: go, rust, ts, js, py,
//     java, csharp and swift emitted an inclusive loop, so the program read one
//     past the end of a five-element array — a panic in Go, an IndexError in
//     Python, NaN in JavaScript. Kotlin emitted `for (i in 0L <= 5L)`, which is
//     not a range at all and does not compile.
//   - bash compiled `"Hello, " + name` into an arithmetic expansion, which
//     evaluates to 0, and built struct literals as indexed arrays, so every
//     field collapsed onto index 0 and struct.xql.json printed 5 twice.
//   - tcl passed the same concatenation to `expr`, which aborts on a
//     non-numeric operand.
//
// Eight of those backends are in the executed tier. The dogfood workspace they
// run has no range loop and no string concatenation, and it asserts that the
// output *contains* two names rather than what the program printed — so none of
// it was visible.
//
// This test closes that: known programs, known stdout, compared exactly, in
// every language a toolchain here can actually run.

// conformanceExpect maps an example to the lines it must print, in order. An
// example belongs here only once its output is a fact about the program rather
// than about the machine — clock.xql.json prints timestamps and cannot join.
var conformanceExpect = map[string][]string{
	"hello.xql.json":         {"Hello, World"},
	"loop.xql.json":          {"15"},
	"struct.xql.json":        {"3", "5"},
	"collections.xql.json":   {"10", "30"},
	"lambda_ifexpr.xql.json": {"big"},
	"example.xql.json":       {"8", "55"},

	// 7 / 2, 7 % 2, 7 * 2 + 1. The division is the whole point: `/` between
	// two Ints is integer division, and roughly a third of the target
	// languages make `/` float division instead. Before this, `7 / 2` printed
	// 3 in Go, C, Ruby and Tcl and 3.5 in Python, JavaScript, Perl, awk, Lua,
	// PHP, Julia and Dart, and did not compile at all in Haskell or Zig.
	"int_arithmetic.xql.json": {"3", "1", "15"},

	// A while loop with an early exit, and string equality. Perl's `==` is
	// numeric, so it numified both operands to 0 and answered yes to every
	// string comparison; C compared `const char *` addresses, so two equal
	// strings built differently came out unequal; and C++ would not compile
	// `"ab" + "c"` at all, that being pointer arithmetic on two char arrays.
	"control_flow.xql.json":   {"21"},
	"string_compare.xql.json": {"differ", "eq", "ne-ok"},
}

// conformanceRunner says how to turn one target's output into a running
// process. Placeholders: {tool} the working executable, {file} the generated
// source, {bin} an absolute path for a compiled artefact.
type conformanceRunner struct {
	target string
	ext    string
	tools  []string
	steps  [][]string
}

// conformanceRunners holds only targets whose stdout has been compared against
// the rest of the matrix. Adding one is cheap and is the point — a target
// listed here is a target that can no longer quietly disagree with the other
// forty-five. The registry in verification.go must call every one of them
// executed, which TestConformanceRunnersAreExecutedTier enforces.
var conformanceRunners = []conformanceRunner{
	{target: "go", ext: ".go", tools: []string{"go"},
		steps: [][]string{{"{tool}", "run", "{file}"}}},
	{target: "py", ext: ".py", tools: []string{"python3", "python"},
		steps: [][]string{{"{tool}", "{file}"}}},
	{target: "js", ext: ".js", tools: []string{"node"},
		steps: [][]string{{"{tool}", "{file}"}}},
	{target: "perl", ext: ".pl", tools: []string{"perl"},
		steps: [][]string{{"{tool}", "{file}"}}},
	{target: "bash", ext: ".sh", tools: []string{"bash"},
		steps: [][]string{{"{tool}", "{file}"}}},
	{target: "tcl", ext: ".tcl", tools: []string{"tclsh"},
		steps: [][]string{{"{tool}", "{file}"}}},

	// gawk, not awk: the compiled tier already depends on gawk being present,
	// and mawk differs enough to be a separate claim.
	{target: "awk", ext: ".awk", tools: []string{"gawk"},
		steps: [][]string{{"{tool}", "-f", "{file}"}}},

	{target: "c", ext: ".c", tools: []string{"gcc", "clang"},
		steps: [][]string{{"{tool}", "-o", "{bin}", "{file}"}, {"{bin}"}}},
	{target: "cpp", ext: ".cpp", tools: []string{"g++", "clang++"},
		steps: [][]string{{"{tool}", "-o", "{bin}", "{file}"}, {"{bin}"}}},
}

func TestCrossTargetConformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping toolchain-driven checks in -short mode")
	}

	corpus := exampleCorpus(t)
	names := make([]string, 0, len(conformanceExpect))
	for name := range conformanceExpect {
		if _, ok := corpus[name]; !ok {
			t.Fatalf("conformanceExpect names %q, which is not in the example corpus", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)

	for _, r := range conformanceRunners {
		t.Run(r.target, func(t *testing.T) {
			tool := e2e.FirstWorking(r.tools...)
			if tool == "" {
				e2e.Missing(t, "none of %v is on PATH, so %s was generated but never run",
					r.tools, r.target)
				return
			}

			ran := 0
			for _, name := range names {
				res := CompileFromFile(corpus[name], r.target, "")
				if !res.Success {
					// Declining is the matrix's business, and it already asserts
					// that every rejection is a deliberate XQL_E402.
					continue
				}

				got, err := runConformance(t, r, tool, res.Code)
				if err != nil {
					t.Errorf("%s compiled %s, but running it failed: %v", r.target, name, err)
					continue
				}
				want := conformanceExpect[name]
				if !equalStrings(got, want) {
					t.Errorf("%s ran %s and printed %q; every other target prints %q\n--- generated ---\n%s",
						r.target, name, got, want, res.Code)
					continue
				}
				ran++
			}

			if ran == 0 {
				t.Errorf("%s ran no example at all, so this target verified nothing", r.target)
			}
		})
	}
}

// runConformance writes the generated source to a scratch directory, walks the
// runner's steps, and returns the last step's stdout as trimmed non-empty lines.
// Blank lines and trailing whitespace are dropped because they are formatting,
// not behaviour: Fortran pads, PowerShell does not, and neither says anything
// about whether the translation is faithful.
func runConformance(t *testing.T, r conformanceRunner, tool string, code []byte) ([]string, error) {
	t.Helper()

	dir := t.TempDir()
	src := filepath.Join(dir, "prog"+r.ext)
	if err := os.WriteFile(src, code, 0o644); err != nil {
		t.Fatalf("write %s: %v", src, err)
	}
	// Naming the artefact .exe everywhere keeps one code path: Windows needs the
	// suffix and Unix does not care what an executable is called.
	bin := filepath.Join(dir, "prog_bin.exe")

	subst := strings.NewReplacer("{tool}", tool, "{file}", src, "{bin}", bin)

	var out []byte
	for _, step := range r.steps {
		args := make([]string, len(step))
		for i, a := range step {
			args[i] = subst.Replace(a)
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		// A generated program reads nothing; leaving Stdin nil hands it the null
		// device, so an awk rule outside BEGIN cannot hang the run.
		combined, err := cmd.CombinedOutput()
		if err != nil {
			return nil, errWithOutput(err, combined)
		}
		out = combined
	}
	return nonEmptyLines(string(out)), nil
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

type outputError struct {
	err    error
	output string
}

func (e *outputError) Error() string { return e.err.Error() + "\n" + e.output }

func errWithOutput(err error, out []byte) error {
	return &outputError{err: err, output: string(out)}
}

// TestConformanceRunnersAreExecutedTier keeps the registry honest in the
// direction that matters: this test is what produces the executed-tier evidence
// for the targets below, so a runner here and a weaker tier there would
// understate what CI checks — and dropping a runner while leaving the tier
// alone would overstate it.
func TestConformanceRunnersAreExecutedTier(t *testing.T) {
	for _, r := range conformanceRunners {
		v, ok := VerificationFor(r.target)
		if !ok {
			t.Errorf("conformanceRunners covers %q, which declares no verification at all", r.target)
			continue
		}
		if v.Tier != TierExecuted {
			t.Errorf("%s is run and its stdout asserted, but the registry calls it %q — "+
				"that is the executed tier by definition", r.target, v.Tier)
		}
		if !strings.HasPrefix(v.Harness, "TestCrossTargetConformance/") &&
			!strings.Contains(v.Harness, "E2E") {
			t.Errorf("%s claims evidence from %q, but this test is what runs it", r.target, v.Harness)
		}
	}
}

// TestConformanceCorpusIsWorthRunning stops the corpus from decaying into a
// single hello-world: the bugs this test exists to catch were in loops, structs
// and string handling, not in printing a constant.
func TestConformanceCorpusIsWorthRunning(t *testing.T) {
	if len(conformanceExpect) < 5 {
		t.Fatalf("the conformance corpus has shrunk to %d programs; it is meant to span "+
			"loops, structs, collections and expressions", len(conformanceExpect))
	}
	if _, ok := conformanceExpect["loop.xql.json"]; !ok {
		t.Error("loop.xql.json must stay in the corpus — the off-by-one range loop that " +
			"eight backends shipped is exactly what it pins")
	}
}
