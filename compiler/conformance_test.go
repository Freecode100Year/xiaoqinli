package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	// -7/2, -7%2, 7/-2, 7%-2. The sign rules are a second divergence hiding
	// behind the first: the languages that floor their division also give `%`
	// the sign of the divisor, so Python, Ruby, Lua, Tcl and Perl answered -4
	// and 1 where C, Go, Java, Rust, JavaScript, awk and bash answer -3 and
	// -1. Truncation is the majority and now the rule; those five emit a
	// helper rather than their native operator.
	"negative_arithmetic.xql.json": {"-3", "-1", "-3", "1"},
}

// conformanceRunner says how to turn one target's output into a running
// process. Placeholders: {tool} the working executable, {file} the generated
// source, {bin} an absolute path for a compiled artefact.
type conformanceRunner struct {
	target string
	ext    string
	tools  []string
	steps  [][]string

	// base overrides the "prog" stem, for a toolchain that derives a name from
	// the file rather than being told one.
	base string

	// pre runs before the generated source is written, for a target that needs a
	// project around the file rather than just the file. `dotnet new console`
	// writes its own Program.cs, so it has to happen first and be overwritten;
	// scaffolding after the write would delete the program under test.
	pre [][]string

	// probe replaces the --version / version handshake FirstWorking uses. lua
	// answers -v and errors on --version, so without this it looks absent on a
	// runner that has it.
	probe []string

	// onlyOS restricts a runner to one GOOS. cmd.exe exists nowhere but
	// Windows and CI is Linux, so such a runner can never be the evidence
	// behind a published tier — it is a check for whoever develops on that
	// platform, and TestConformanceRunnersAreExecutedTier exempts it from the
	// executed-tier rule for exactly that reason.
	onlyOS string
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

	// Windows PowerShell answers neither `--version` nor `version`, so
	// FirstWorking will not pick it up and this skips on Windows; ubuntu-latest
	// ships pwsh, which does, so CI runs it.
	{target: "powershell", ext: ".ps1", tools: []string{"pwsh", "powershell"},
		steps: [][]string{{"{tool}", "-NoProfile", "-NonInteractive", "-File", "{file}"}}},

	// ruby, lua, php and julia were executed-tier already, but only through the
	// dogfood workspace — which has no arithmetic in it at all. Their integer
	// division was changed twice with nothing running the result. lua needs its
	// own probe: it answers -v and errors on --version.
	{target: "ruby", ext: ".rb", tools: []string{"ruby"},
		steps: [][]string{{"{tool}", "{file}"}}},
	{target: "lua", ext: ".lua", tools: []string{"lua", "lua5.4"}, probe: []string{"-v"},
		steps: [][]string{{"{tool}", "{file}"}}},
	{target: "php", ext: ".php", tools: []string{"php"},
		steps: [][]string{{"{tool}", "{file}"}}},
	{target: "julia", ext: ".jl", tools: []string{"julia"},
		steps: [][]string{{"{tool}", "{file}"}}},

	{target: "c", ext: ".c", tools: []string{"gcc", "clang"},
		steps: [][]string{{"{tool}", "-o", "{bin}", "{file}"}, {"{bin}"}}},
	{target: "cpp", ext: ".cpp", tools: []string{"g++", "clang++"},
		steps: [][]string{{"{tool}", "-o", "{bin}", "{file}"}, {"{bin}"}}},

	// rust was executed-tier through the dogfood workspace alone, and that
	// workspace has neither a range loop nor integer division — the two things
	// this backend was changed for. -Awarnings because an unused variable is a
	// remark about the generated style, not about whether the program means what
	// the AST says.
	//
	// On Windows with the msvc toolchain this needs a real link.exe: an MSYS or
	// mingw bin directory earlier on PATH supplies coreutils' `link`, which
	// rustc then invokes and which fails on the second object file. Running the
	// suite as RUSTUP_TOOLCHAIN=stable-x86_64-pc-windows-gnu picks the linker
	// that machine does have. CI is Linux and unaffected.
	{target: "rust", ext: ".rs", tools: []string{"rustc"},
		steps: [][]string{{"{tool}", "-Awarnings", "-o", "{bin}", "{file}"}, {"{bin}"}}},

	// fortran leaves the compiled tier here. gfortran was already syntax-checking
	// every example; the step that was missing is the one that runs the result.
	{target: "fortran", ext: ".f90", tools: []string{"gfortran"},
		steps: [][]string{{"{tool}", "-o", "{bin}", "{file}"}, {"{bin}"}}},

	// ts, like rust, had only the dogfood workspace behind it. tsx is what CI
	// installs and what TestLinkedPipelineE2E already uses.
	{target: "ts", ext: ".ts", tools: []string{"tsx"},
		steps: [][]string{{"{tool}", "{file}"}}},

	// The six below close the last of the gap this file was written for: every
	// target the registry calls executed now runs the corpus, not just the
	// dogfood workspace. None of their toolchains exists on the machine these
	// runners were written on — dotnet here is the runtime with no SDK — so CI
	// is what decides whether they are right. XQL_E2E_REQUIRE=1 means a missing
	// one fails rather than skips, which is the only reason writing them blind
	// is honest: they cannot report a pass they did not earn.

	// java runs the file directly. Single-file source mode has been in the
	// launcher since 11, it compiles in memory, and it lifts the requirement
	// that the file be named after the class — so this needs no javac step and
	// no `base`, even though every generated program declares `class Main`.
	{target: "java", ext: ".java", tools: []string{"java"},
		steps: [][]string{{"{tool}", "{file}"}}},

	// csharp is the one target that needs a project rather than a file, and so
	// the only user of pre. Taking the framework from `dotnet new` rather than
	// writing a .csproj here keeps the test from pinning a TargetFramework that
	// the workflow's SDK version would eventually outgrow.
	{target: "csharp", ext: ".cs", base: "Program", tools: []string{"dotnet"},
		pre:   [][]string{{"{tool}", "new", "console", "--force", "-o", "."}},
		steps: [][]string{{"{tool}", "run", "--verbosity", "quiet"}}},

	// kotlinc answers -version, not --version, so it needs its own probe for the
	// same reason lua does. Building a self-contained jar is what the dogfood
	// workspace already does in CI, so it is the invocation known to work there.
	{target: "kotlin", ext: ".kt", tools: []string{"kotlinc"}, probe: []string{"-version"},
		steps: [][]string{{"{tool}", "{file}", "-include-runtime", "-d", "prog.jar"},
			{"java", "-jar", "prog.jar"}}},

	{target: "swift", ext: ".swift", tools: []string{"swift"},
		steps: [][]string{{"{tool}", "{file}"}}},
	{target: "dart", ext: ".dart", tools: []string{"dart"},
		steps: [][]string{{"{tool}", "run", "{file}"}}},

	// zig prints through std.debug.print, which writes to stderr. CombinedOutput
	// is what makes that comparable; nothing here asserts a stream.
	{target: "zig", ext: ".zig", tools: []string{"zig"},
		steps: [][]string{{"{tool}", "run", "{file}"}}},

	// Windows only, and therefore never CI evidence — bat stays at the smoke
	// tier. It is here because it is the only place the batch backend can be
	// run at all, and running it found `echo (a / b)` putting that text on
	// stdout: echo evaluates nothing, so arithmetic needs `set /a` first.
	{target: "bat", ext: ".bat", tools: []string{"cmd"}, onlyOS: "windows",
		steps: [][]string{{"{tool}", "/c", "{file}"}}},
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
			tool := ""
			if r.onlyOS != "" {
				// A plain skip, not e2e.Missing: XQL_E2E_REQUIRE promises the
				// toolchains a *published tier* needs, and this runner backs no
				// tier. Failing CI over a missing cmd.exe on Linux would be
				// demanding evidence nobody claimed.
				if runtime.GOOS != r.onlyOS {
					t.Skipf("%s runs only on %s, and this is %s", r.target, r.onlyOS, runtime.GOOS)
				}
				for _, name := range r.tools {
					if _, err := exec.LookPath(name); err == nil {
						tool = name
						break
					}
				}
			} else if len(r.probe) > 0 {
				for _, name := range r.tools {
					path, err := exec.LookPath(name)
					if err != nil {
						continue
					}
					if exec.Command(path, r.probe...).Run() == nil {
						tool = name
						break
					}
				}
			} else {
				tool = e2e.FirstWorking(r.tools...)
			}
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
	base := r.base
	if base == "" {
		base = "prog"
	}
	src := filepath.Join(dir, base+r.ext)
	// Naming the artefact .exe everywhere keeps one code path: Windows needs the
	// suffix and Unix does not care what an executable is called.
	bin := filepath.Join(dir, "prog_bin.exe")

	subst := strings.NewReplacer("{tool}", tool, "{file}", src, "{bin}", bin)
	run := func(step []string) ([]byte, error) {
		args := make([]string, len(step))
		for i, a := range step {
			args[i] = subst.Replace(a)
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		// A generated program reads nothing; leaving Stdin nil hands it the null
		// device, so an awk rule outside BEGIN cannot hang the run.
		return cmd.CombinedOutput()
	}

	// Scaffolding runs first and its output is discarded — it is the template's
	// chatter, not the program's.
	for _, step := range r.pre {
		if combined, err := run(step); err != nil {
			return nil, errWithOutput(err, combined)
		}
	}

	if err := os.WriteFile(src, code, 0o644); err != nil {
		t.Fatalf("write %s: %v", src, err)
	}

	var out []byte
	for _, step := range r.steps {
		combined, err := run(step)
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
		if r.onlyOS != "" {
			// CI is Linux. A runner that cannot run there produces no evidence
			// for the published tier, whatever it proves on a developer's
			// machine, so the registry must not have been raised on its account.
			if v, ok := VerificationFor(r.target); ok && v.Tier == TierExecuted {
				t.Errorf("%s is only run on %s, which CI is not, so it cannot be published as executed",
					r.target, r.onlyOS)
			}
			continue
		}
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
