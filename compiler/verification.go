package compiler

import (
	"fmt"
	"sort"
	"strings"
)

// The transpiler advertises 38 targets. What that sentence is worth depends
// entirely on how each one was checked, and those checks are not equal: some
// backends have their output compiled and run against an expected stdout,
// others have only ever been asked to produce bytes.
//
// It advertised 46 until eight of them were removed rather than described. ada,
// clojure, fsharp, mql4, mql5, objc, scala and v had never had their output
// read by a compiler for the language they claimed to emit, and ada's had been
// read and rejected — gnat refuses an unconstrained `String` declaration, an
// `array of T` without bounds, and `%`, which opens a string literal in Ada
// rather than taking a remainder. A target nobody can check is a claim, and the
// honest way to stop making it is to stop shipping it.
//
// Until this registry existed the difference lived in prose, and prose drifts.
// The android backend spent a release emitting Kotlin that could not compile —
// a `Result<T, E>` colliding with the stdlib's `kotlin.Result<out T>` — while
// the README described android as working, because nothing tied the claim to a
// test. VerificationTier makes the claim a value the test suite can check.

// VerificationTier is how strong the evidence for a target is.
type VerificationTier int

const (
	// TierSmoke means codegen was asked for output and returned some without
	// erroring. Nothing has established that the output is valid in the target
	// language, let alone that it behaves correctly.
	TierSmoke VerificationTier = iota

	// TierCompiled means a real toolchain accepted the generated source. Syntax
	// and types are known good; runtime behaviour is not.
	TierCompiled

	// TierExecuted means the generated program was compiled, run, and its
	// stdout checked. This is the only tier that says the translation preserves
	// meaning.
	TierExecuted
)

func (t VerificationTier) String() string {
	switch t {
	case TierExecuted:
		return "executed"
	case TierCompiled:
		return "compiled"
	default:
		return "smoke"
	}
}

// Label is the human-readable evidence description used in the README table.
func (t VerificationTier) Label() string {
	switch t {
	case TierExecuted:
		return "compiled and run, stdout asserted"
	case TierCompiled:
		return "compiled by a real toolchain"
	default:
		return "codegen returns output; never compiled"
	}
}

// TargetVerification records how a single target is checked, and by what.
type TargetVerification struct {
	// Tier is the strongest evidence CI produces for this target.
	Tier VerificationTier

	// Harness names the test that produces it, so a reader can go look.
	Harness string

	// Toolchain is the executable CI must install for that harness to run at
	// all. Empty when the tier needs no external toolchain.
	Toolchain string

	// Note explains anything a reader would otherwise get wrong.
	Note string
}

// targetVerification maps every advertised target flag to its evidence.
// TestEveryAdvertisedTargetDeclaresVerification keeps it exhaustive.
var targetVerification = map[string]TargetVerification{
	// Executed: the workspace dogfood example is compiled and run, and its
	// stdout must contain the names the program prints.
	"go":     {TierExecuted, "TestLinkedPipelineE2E/Go", "go", ""},
	"py":     {TierExecuted, "TestLinkedPipelineE2E/Python", "python3", ""},
	"rust":   {TierExecuted, "TestLinkedPipelineE2E/Rust", "rustc", ""},
	"ts":     {TierExecuted, "TestLinkedPipelineE2E/TypeScript", "tsx", ""},
	"java":   {TierExecuted, "TestLocalE2EWorkspaceDogfood/Java", "javac", ""},
	"csharp": {TierExecuted, "TestLocalE2EWorkspaceDogfood/CSharp", "dotnet", ""},
	"kotlin": {TierExecuted, "TestLocalE2EWorkspaceDogfood/Kotlin", "kotlinc", ""},
	"swift":  {TierExecuted, "TestLocalE2EWorkspaceDogfood/Swift", "swiftc", ""},
	"dart":   {TierExecuted, "TestLocalE2EWorkspaceDogfood/Dart", "dart", ""},
	"zig":    {TierExecuted, "TestLocalE2EWorkspaceDogfood/Zig", "zig", ""},
	"php":    {TierExecuted, "TestLocalE2EWorkspaceDogfood/PHP", "php", ""},
	"ruby":   {TierExecuted, "TestLocalE2EWorkspaceDogfood/Ruby", "ruby", ""},
	"lua":    {TierExecuted, "TestLocalE2EWorkspaceDogfood/Lua", "lua", ""},
	"julia":  {TierExecuted, "TestLocalE2EWorkspaceDogfood/Julia", "julia", ""},

	// Executed by the conformance corpus rather than the workspace dogfood.
	// None of these backends implements Result<T>, so the workspace refuses
	// them, and until the corpus existed that left them unrun. Every one was
	// wrong about something the moment its output was executed — see the header
	// of conformance_test.go.
	"js":   {TierExecuted, "TestCrossTargetConformance/js", "node", "rejects Result<T>"},
	"c":    {TierExecuted, "TestCrossTargetConformance/c", "gcc", "rejects Result<T>"},
	"cpp":  {TierExecuted, "TestCrossTargetConformance/cpp", "g++", "rejects Result<T>"},
	"perl": {TierExecuted, "TestCrossTargetConformance/perl", "perl", "rejects Result<T>"},
	"bash": {TierExecuted, "TestCrossTargetConformance/bash", "bash", "rejects Result<T>"},
	"tcl":  {TierExecuted, "TestCrossTargetConformance/tcl", "tclsh", "rejects Result<T>"},
	"awk":  {TierExecuted, "TestCrossTargetConformance/awk", "gawk", "rejects Result<T> and struct literals"},

	// pwsh runs the corpus; the parse-only case in compiledTierCases stays,
	// because it covers the examples with no assertable stdout.
	"powershell": {TierExecuted, "TestCrossTargetConformance/powershell", "pwsh", "rejects Result<T>"},

	// fortran leaves the compiled tier: gfortran was already checking its
	// syntax, and running the result found the backend handing five-character
	// arguments to dummies declared 256 long — a read past the end of the actual
	// argument that printed heap next to "Hello, World".
	"fortran": {TierExecuted, "TestCrossTargetConformance/fortran", "gfortran", "rejects for-each loops"},

	// The tier that used to be compiled-only. A check-only toolchain and one
	// that produces a running binary are the same install, so these joined the
	// corpus as soon as anyone asked them to; the compiled tier entries below
	// stay, because they cover the examples with no assertable stdout.
	"nim": {TierExecuted, "TestCrossTargetConformance/nim", "nim", "rejects Result<T>"},

	// The three functional backends carry one more caveat than the rest, and it
	// is the same caveat three times: each lowers a loop to a form that runs to
	// the end — mapM_, a `for ... done` whose body must be unit, Enum.reduce —
	// so break, continue and a return from inside a loop are all early exits
	// none of them has. They emitted all three anyway until a corpus program
	// contained one.
	"haskell": {TierExecuted, "TestCrossTargetConformance/haskell", "ghc", "rejects Result<T>, and break, continue or return from inside a loop"},
	"ocaml":   {TierExecuted, "TestCrossTargetConformance/ocaml", "ocaml", "rejects Result<T>, and break, continue or return from inside a loop"},
	"crystal": {TierExecuted, "TestCrossTargetConformance/crystal", "crystal", "rejects Result<T>"},
	"d":       {TierExecuted, "TestCrossTargetConformance/d", "gdc", "rejects Result<T>"},
	"pascal":  {TierExecuted, "TestCrossTargetConformance/pascal", "fpc", "rejects for-each loops"},
	"elixir":  {TierExecuted, "TestCrossTargetConformance/elixir", "elixir", "rejects Result<T>, and break, continue or return from inside a loop"},
	"vala":    {TierExecuted, "TestCrossTargetConformance/vala", "valac", "rejects Result<T>"},
	"groovy":  {TierExecuted, "TestCrossTargetConformance/groovy", "groovy", "rejects Result<T>"},

	// Compiled where a check-only toolchain exists, smoke where none does.
	// A smoke entry means codegen ran over the full example corpus without
	// erroring or panicking — and that no compiler for the language has ever
	// seen the result.
	"tccli": {TierCompiled, "TestCompiledTier/tccli", "bash", "emits Tencent Cloud CLI shell; no arithmetic, comparisons or structs"},

	// A bundle rather than a program, so the conformance corpus cannot reach
	// it — but an extension is a manifest and some JavaScript, and both have
	// parsers. node reads the scripts; the manifest is checked for the keys MV3
	// will not load without. What no CI can say is whether Chrome accepts the
	// result.
	"chrome": {TierCompiled, "TestChromeBundle", "node", "emits an extension bundle; the JavaScript is parsed, the browser is not consulted; rejects Result<T>"},

	// ios builds. `swift build` over the generated package is what
	// TestLocalE2EProjectScaffolds does, and CI installs swift, so this is a
	// real toolchain accepting real output — it was only ever called smoke
	// because a Swift package has no stdout to assert.
	"ios": {TierCompiled, "TestLocalE2EProjectScaffolds/iOS", "swift", "SwiftPM package; `swift build` succeeds, nothing is run"},

	// Assembling an APK needs an SDK whose version triple drifts with the
	// runner image, and the generated Kotlin imports androidx, which kotlinc
	// alone cannot resolve. So the structure is checked and the source leans on
	// the kotlin executed tier.
	"android": {TierSmoke, "TestAndroidScaffoldStructure", "", "Gradle scaffold; structure checked, never assembled"},

	// The shortcut backend emits JSON — the registry called it a plist for a
	// long time, and nothing had read the bytes to notice. TestShortcutBundle
	// checks that it parses and that every action carries a namespaced
	// identifier. Only Apple's Shortcuts app can say more, and it has no
	// command-line form.
	//
	// Which is why the loop limits below are stated rather than approximated.
	// A smoke-tier backend has nothing that could catch a wrong translation,
	// so the only defence it has is declining what it cannot say: Shortcuts'
	// Repeat takes a count or a list and there is no action that leaves one
	// early, and this backend used to answer `while` with Repeat 1000 and
	// `break` with a comment.
	//
	// `return` was the fourth and it was missed the first time, because it does
	// not look like a jump: it emitted the value and the Repeat carried on to
	// the end. What the loops do have now is a loop variable — Repeat Index and
	// Repeat Item, which nothing had read, so every `i` inside a generated
	// Repeat named a variable no action ever set.
	"shortcut": {TierSmoke, "TestShortcutBundle", "", "emits a Shortcuts workflow as JSON; structure checked, never imported; rejects Result<T>, while, and break, continue or return inside a loop"},

	// cmd.exe exists on one platform, so this one is earned on a Windows runner
	// rather than the Linux one every other target uses. The exclusion is real
	// and named: `set /a` is 32-bit signed with no wider arithmetic anywhere in
	// the interpreter, so int_width.xql.json is the single corpus program bat
	// is excused from, and conformance_test.go prints that exclusion on
	// every run.
	"bat": {TierExecuted, "TestCrossTargetConformance/bat", "cmd", "rejects struct literals and for-each; `set /a` is 32-bit, so int_width is excused"},
}

// RenderVerificationTable renders the coverage table both READMEs embed. It is
// generated rather than written so the published claim cannot drift from the
// tier the tests actually enforce.
func RenderVerificationTable() string {
	var b strings.Builder

	executed := TargetsAtTier(TierExecuted)
	compiled := TargetsAtTier(TierCompiled)
	smoke := TargetsAtTier(TierSmoke)

	b.WriteString("\n| Evidence | Targets | What was checked |\n")
	b.WriteString("|---|---|---|\n")
	row := func(tier VerificationTier, flags []string) {
		if len(flags) == 0 {
			return
		}
		fmt.Fprintf(&b, "| **%s** (%d) | %s | %s |\n",
			tier.String(), len(flags), "`"+strings.Join(flags, "` `")+"`", tier.Label())
	}
	row(TierExecuted, executed)
	row(TierCompiled, compiled)
	row(TierSmoke, smoke)

	fmt.Fprintf(&b, "\nThe executed tier needs %d toolchains, which CI installs or inherits from the\n",
		len(RequiredToolchains()))
	b.WriteString("runner image, and it sets `XQL_E2E_REQUIRE=1` so a missing one fails the run\n")
	b.WriteString("instead of skipping quietly.\n\n")

	// Anything with a caveat is worth naming, whatever its tier.
	var noted []string
	for _, tier := range []VerificationTier{TierExecuted, TierCompiled, TierSmoke} {
		for _, flag := range TargetsAtTier(tier) {
			if v := targetVerification[flag]; v.Note != "" {
				noted = append(noted, fmt.Sprintf("- `%s` — %s", flag, v.Note))
			}
		}
	}
	if len(noted) > 0 {
		sort.Strings(noted)
		b.WriteString("Per-target caveats:\n\n")
		b.WriteString(strings.Join(noted, "\n"))
		b.WriteString("\n")
	}

	return b.String()
}

// VerificationFor returns the recorded evidence for a target flag.
func VerificationFor(flag string) (TargetVerification, bool) {
	v, ok := targetVerification[flag]
	return v, ok
}

// TargetsAtTier lists the advertised flags verified at exactly the given tier,
// sorted, so callers get a stable order to print.
func TargetsAtTier(tier VerificationTier) []string {
	var out []string
	for flag, v := range targetVerification {
		if v.Tier == tier {
			out = append(out, flag)
		}
	}
	sort.Strings(out)
	return out
}

// RequiredToolchains lists every executable CI has to install for the
// executed tier to actually run rather than skip. The E2E workflow is
// generated from this, and TestCIInstallsEveryRequiredToolchain checks it.
func RequiredToolchains() []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range targetVerification {
		if v.Tier != TierExecuted || v.Toolchain == "" {
			continue
		}
		if !seen[v.Toolchain] {
			seen[v.Toolchain] = true
			out = append(out, v.Toolchain)
		}
	}
	sort.Strings(out)
	return out
}
