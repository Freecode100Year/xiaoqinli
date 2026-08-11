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

// The gap between "codegen returned bytes" and "a program ran and printed the
// right thing" is wide enough that a backend can sit in it for a long time
// emitting text no compiler would accept. Three did: bash and perl passed
// `Result.ok(x)` and `res.unwrap()` straight through with no runtime behind
// them, and fortran emitted recursive calls without the RECURSIVE keyword its
// own standard requires. All three reported success.
//
// TierCompiled closes that gap for targets where a compiler is cheap to install
// but running the output is not. Every one of these tools ships on the runner
// image already, so the tier costs nothing in CI:
//
//	gcc  g++  gfortran  node  perl  bash  gawk  pwsh
//
// The check is syntax and, where the tool offers it, types — never execution.
// Saying so precisely is the point: this tier means the language accepted the
// text, not that the program is correct.

type compiledTierCase struct {
	// target is the compile flag.
	target string
	// ext is the filename suffix the checker expects. gfortran picks its
	// language from it, so it is not cosmetic.
	ext string
	// tools are acceptable checkers, first working one wins.
	tools []string
	// args are passed before the filename.
	args []string
	// base overrides the "prog" stem, for a compiler that derives a unit name
	// from the file name.
	base string
	// probe replaces the --version / version handshake FirstWorking uses to
	// tell a working executable from a name on PATH. fpc answers neither and
	// exits non-zero, so this tier reported "fpc is not installed" on a runner
	// where fpc 3.2.2 was installed and working.
	probe []string
}

// firstWorking picks the first of the case's tools that is present and runs.
func (tc compiledTierCase) firstWorking() string {
	if len(tc.probe) == 0 {
		return e2e.FirstWorking(tc.tools...)
	}
	for _, name := range tc.tools {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if exec.Command(path, tc.probe...).Run() == nil {
			return name
		}
	}
	return ""
}

// fileName is the stem plus extension the checker will be handed.
func (tc compiledTierCase) fileName() string {
	base := tc.base
	if base == "" {
		base = "prog"
	}
	return base + tc.ext
}

var compiledTierCases = []compiledTierCase{
	{target: "c", ext: ".c", tools: []string{"gcc", "clang"}, args: []string{"-fsyntax-only"}},
	{target: "cpp", ext: ".cpp", tools: []string{"g++", "clang++"}, args: []string{"-fsyntax-only"}},
	{target: "fortran", ext: ".f90", tools: []string{"gfortran"}, args: []string{"-fsyntax-only"}},
	{target: "js", ext: ".js", tools: []string{"node"}, args: []string{"--check"}},
	{target: "perl", ext: ".pl", tools: []string{"perl"}, args: []string{"-c"}},
	{target: "bash", ext: ".sh", tools: []string{"bash"}, args: []string{"-n"}},

	// tccli emits shell, so bash checks it for free. It used to emit `local`
	// at the top level and `[ "$n" <= 1 ]`, neither of which is valid.
	{target: "tccli", ext: ".sh", tools: []string{"bash"}, args: []string{"-n"}},

	// gawk --pretty-print parses the whole program and writes it back out
	// without running it; plain `awk -f` would execute. Confirmed against
	// gawk 5.4: a BEGIN block that prints does not print.
	{target: "awk", ext: ".awk", tools: []string{"gawk"},
		args: []string{"--pretty-print=/dev/null", "-f", "{file}"}},

	// PowerShell has no -n, but it exposes its parser. ParseFile reports
	// syntax errors without executing a line.
	{target: "powershell", ext: ".ps1", tools: []string{"pwsh", "powershell"},
		args: []string{"-NoProfile", "-NonInteractive", "-Command",
			"$e=$null;[void][System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path '{file}'),[ref]$null,[ref]$e);" +
				"if($e.Count){$e|ForEach-Object{Write-Output $_.Message};exit 1}"}},

	// The tier below this line needs an apt install, so unlike the tools above
	// it is not free. Each is still check-only: nim check does not link, ghc
	// -fno-code type-checks and emits nothing, ocamlc -stop-after typing stops
	// before code generation, and crystal build --no-codegen does the semantic
	// pass alone. Elixir is parse-only; see its note below.
	{target: "nim", ext: ".nim", tools: []string{"nim"}, args: []string{"check", "--hints:off", "--verbosity:0"}},
	{target: "haskell", ext: ".hs", tools: []string{"ghc"}, args: []string{"-fno-code", "-v0"}},
	{target: "ocaml", ext: ".ml", tools: []string{"ocamlc"}, args: []string{"-stop-after", "typing"}},
	// Not elixirc: the generated file ends in a top-level Main.main() call, and
	// Elixir evaluates top-level expressions while compiling, so elixirc would
	// run the program. Code.string_to_quoted! only parses.
	{target: "elixir", ext: ".ex", tools: []string{"elixir"},
		args: []string{"-e", `Code.string_to_quoted!(File.read!("{file}"))`}},
	{target: "crystal", ext: ".cr", tools: []string{"crystal"}, args: []string{"build", "--no-codegen", "{file}"}},

	// The four below were smoke-only for one reason: nobody had installed a
	// compiler for them. Each is check-only in the same sense as the group
	// above — gdc has -fsyntax-only, valac --ccode stops at the generated C,
	// fpc -s stops before the assembler, and groovyc only writes classes.
	//
	// Running them found a real defect Pascal had been shipping: it assigned to
	// `Result` without the mode directive that makes `Result` exist.
	//
	// Ada is deliberately not here. gnat rejects three separate things the
	// backend emits — `x : String;` (String is unconstrained, so a declaration
	// without an initialiser is illegal), `array of T` (Ada needs bounds or an
	// index subtype), and `a % b` (`%` opens a string literal in Ada; the
	// operator is `mod` or `rem`). Those are type-system changes, not typos,
	// and a tier claimed before they are made would be a claim.
	{target: "d", ext: ".d", tools: []string{"gdc"},
		args: []string{"-fsyntax-only"}},
	{target: "pascal", ext: ".pas", tools: []string{"fpc"},
		args: []string{"-s", "-vw", "{file}"}, probe: []string{"-iV"}},
	{target: "vala", ext: ".vala", tools: []string{"valac"},
		args: []string{"--ccode", "-d", ".", "{file}"}},
	{target: "groovy", ext: ".groovy", tools: []string{"groovyc"},
		args: []string{"-d", ".", "{file}"}},
}

// TestCompiledTier checks every example a target accepts, not a chosen one.
// Picking the example would let a backend look verified on the one program that
// happens to work; the corpus is what the matrix already reasons about, so the
// two tests agree on which pairs are supposed to compile at all.
func TestCompiledTier(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping toolchain-driven checks in -short mode")
	}

	corpus := exampleCorpus(t)
	names := make([]string, 0, len(corpus))
	for name := range corpus {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, tc := range compiledTierCases {
		t.Run(tc.target, func(t *testing.T) {
			tool := tc.firstWorking()
			if tool == "" {
				e2e.Missing(t, "none of %v is on PATH, so %s output was generated but never checked",
					tc.tools, tc.target)
				return
			}

			checked := 0
			for _, name := range names {
				res := CompileFromFile(corpus[name], tc.target, "")
				if !res.Success {
					// Declining is the matrix's business, and it already asserts
					// that every rejection here is a deliberate XQL_E402.
					continue
				}

				dir := t.TempDir()
				path := filepath.Join(dir, tc.fileName())
				if err := os.WriteFile(path, res.Code, 0o644); err != nil {
					t.Fatalf("write %s: %v", path, err)
				}

				cmd := exec.Command(tool, checkerArgs(tc.args, tc.fileName())...)
				cmd.Dir = dir
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Errorf("%s compiled %s, but %s rejects the result: %v\n%s\n--- generated ---\n%s",
						tc.target, name, tool, err, out, res.Code)
					continue
				}
				checked++
			}

			if checked == 0 {
				t.Errorf("%s accepted no example at all, so this tier verified nothing", tc.target)
			}
		})
	}
}

// checkerArgs substitutes {file} where a checker needs the path in the middle
// of its arguments, and appends it when it does not.
func checkerArgs(args []string, file string) []string {
	out := make([]string, 0, len(args)+1)
	substituted := false
	for _, a := range args {
		if strings.Contains(a, "{file}") {
			substituted = true
			a = strings.ReplaceAll(a, "{file}", file)
		}
		out = append(out, a)
	}
	if !substituted {
		out = append(out, file)
	}
	return out
}

// TestCompiledTierMatchesRegistry keeps the table above and the published tier
// in step. A case here is a floor, not an equality: several targets are also
// run by TestCrossTargetConformance and so are published as executed, while
// their syntax check stays useful — it covers every example in the corpus,
// including the ones with no assertable stdout. What must never happen is the
// registry claiming *less* than this table checks, or claiming the compiled
// tier with nothing here to produce it.
func TestCompiledTierMatchesRegistry(t *testing.T) {
	inTable := map[string]bool{}
	for _, tc := range compiledTierCases {
		inTable[tc.target] = true
		v, ok := VerificationFor(tc.target)
		if !ok {
			t.Errorf("compiledTierCases covers %q, which declares no verification at all", tc.target)
			continue
		}
		if v.Tier < TierCompiled {
			t.Errorf("compiledTierCases compiles %q with a real toolchain but the registry calls it %q; "+
				"one of the two is out of date", tc.target, v.Tier)
		}
	}

	for _, flag := range TargetsAtTier(TierCompiled) {
		if inTable[flag] {
			continue
		}
		// Not every compiled-tier target hands one file to one compiler. chrome
		// is a bundle and ios is a Swift package; their checkers live in
		// TestChromeBundle and TestLocalE2EProjectScaffolds. What must hold for
		// all of them is that the registry names the test that produces the
		// evidence, so a reader can go and find it.
		v, ok := VerificationFor(flag)
		if !ok || strings.TrimSpace(v.Harness) == "" {
			t.Errorf("the registry claims %q is compiled and names no harness — "+
				"a tier nothing runs is a claim, not evidence", flag)
			continue
		}
		if strings.HasPrefix(v.Harness, "TestCompiledTier/") {
			t.Errorf("%q says its evidence is %s, but compiledTierCases has no case for it",
				flag, v.Harness)
		}
	}
}

// TestCompiledTierToolsAreDeclared makes the CI workflow accountable for the
// tier the same way the executed one is.
func TestCompiledTierToolsAreDeclared(t *testing.T) {
	// Every tool in this tier was chosen because ubuntu-latest already has it.
	// If that stops being true the workflow needs an install step, and this is
	// where the reminder belongs.
	preinstalled := map[string]bool{
		"gcc": true, "clang": true,
		"g++": true, "clang++": true,
		"gfortran": true, "node": true,
		"perl": true, "bash": true,
		"gawk": true, "pwsh": true, "powershell": true,
	}
	// Installed by an explicit step in e2e-backends.yml rather than shipped
	// with the image. TestCompiledTierAptToolsAreInstalled checks the step.
	aptInstalled := map[string]bool{
		"nim": true, "ghc": true, "ocamlc": true, "elixir": true, "crystal": true,
		"gdc": true, "fpc": true, "valac": true, "groovyc": true,
	}
	for _, tc := range compiledTierCases {
		for _, tool := range tc.tools {
			if !preinstalled[tool] && !aptInstalled[tool] {
				t.Errorf("%s wants %q, which is not known to ship on the runner image — "+
					"add an install step to e2e-backends.yml and list it here", tc.target, tool)
			}
		}
	}
}

// TestCompiledTierIsNotExecuted guards the distinction the tier names. A step
// that runs the program belongs in TestLinkedPipelineE2E, where stdout is
// actually asserted; smuggling one in here would let a target claim the weaker
// tier while doing the stronger check, or worse, the reverse.
func TestCompiledTierIsNotExecuted(t *testing.T) {
	for _, tc := range compiledTierCases {
		joined := strings.Join(tc.args, " ")
		switch tc.tools[0] {
		case "node":
			if !strings.Contains(joined, "--check") {
				t.Errorf("%s: node without --check runs the program", tc.target)
			}
		case "perl":
			if !strings.Contains(joined, "-c") {
				t.Errorf("%s: perl without -c runs the program", tc.target)
			}
		case "bash":
			if !strings.Contains(joined, "-n") {
				t.Errorf("%s: bash without -n runs the script", tc.target)
			}
		case "gawk":
			if !strings.Contains(joined, "--pretty-print") {
				t.Errorf("%s: gawk without --pretty-print executes the program", tc.target)
			}
		case "pwsh":
			if !strings.Contains(joined, "Parser") {
				t.Errorf("%s: powershell must go through the parser API, not execution", tc.target)
			}
		case "nim":
			if !strings.Contains(joined, "check") {
				t.Errorf("%s: nim needs the check subcommand; `nim c -r` would run it", tc.target)
			}
		case "ghc":
			if !strings.Contains(joined, "-fno-code") {
				t.Errorf("%s: ghc without -fno-code links a binary", tc.target)
			}
		case "ocamlc":
			if !strings.Contains(joined, "typing") {
				t.Errorf("%s: ocamlc must stop after typing", tc.target)
			}
		case "crystal":
			if !strings.Contains(joined, "--no-codegen") {
				t.Errorf("%s: crystal build without --no-codegen produces a binary", tc.target)
			}
		case "elixir":
			if !strings.Contains(joined, "string_to_quoted") {
				t.Errorf("%s: elixir must only parse — compiling evaluates the top-level call", tc.target)
			}
		case "fpc":
			// -s writes the assembly and stops; nothing is assembled or linked.
			if !strings.Contains(joined, "-s") {
				t.Errorf("%s: fpc without -s links a binary", tc.target)
			}
		case "valac":
			// --ccode stops at the generated C rather than invoking the C
			// compiler on it.
			if !strings.Contains(joined, "--ccode") {
				t.Errorf("%s: valac without --ccode compiles and links", tc.target)
			}
		case "groovyc":
			// groovyc only ever writes .class files; `groovy` would run them,
			// which is why the case names the compiler and not the launcher.
			if strings.Contains(joined, "-e") {
				t.Errorf("%s: groovyc with -e evaluates a script", tc.target)
			}
		default:
			if !strings.Contains(joined, "-fsyntax-only") {
				t.Errorf("%s: %v should check syntax only, got args %v", tc.target, tc.tools, tc.args)
			}
		}
	}
}

// TestCompiledTierAptToolsAreInstalled holds the workflow to the same standard
// the executed tier is held to: a tier CI cannot run is a claim, not evidence.
func TestCompiledTierAptToolsAreInstalled(t *testing.T) {
	workflow := readFile(t, filepath.Join("..", ".github", "workflows", "e2e-backends.yml"))

	// The package or install step that provides each executable.
	providedBy := map[string]string{
		"nim":     "nim",
		"ghc":     "ghc",
		"ocamlc":  "ocaml-nox",
		"elixir":  "elixir",
		"crystal": "crystal-lang.org/install.sh",
		"gdc":     "gdc",
		"fpc":     "fp-compiler",
		"valac":   "valac",
		"groovyc": "groovy",
	}
	for _, tc := range compiledTierCases {
		for _, tool := range tc.tools {
			needle, needsInstall := providedBy[tool]
			if !needsInstall {
				continue
			}
			if !strings.Contains(workflow, needle) {
				t.Errorf("the compiled tier needs %q for %s, but e2e-backends.yml never installs it (looked for %q)",
					tool, tc.target, needle)
			}
		}
	}
}
