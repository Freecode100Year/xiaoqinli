package compiler

import (
	"fmt"
	"sort"
	"strings"
)

// The transpiler advertises 46 targets. What that sentence is worth depends
// entirely on how each one was checked, and those checks are not equal: some
// backends have their output compiled and run against an expected stdout,
// others have only ever been asked to produce bytes.
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

	// Compiled where a check-only toolchain exists, smoke where none does.
	// A smoke entry means codegen ran over the full example corpus without
	// erroring or panicking — and that no compiler for the language has ever
	// seen the result.
	"nim":        {TierCompiled, "TestCompiledTier/nim", "nim", "rejects Result<T>"},
	"mql4":       {TierSmoke, "TestExampleTargetMatrix", "", "rejects Result<T>, maps, Option, for-each"},
	"mql5":       {TierSmoke, "TestExampleTargetMatrix", "", "rejects Result<T>, maps, Option, for-each"},
	"scala":      {TierSmoke, "TestExampleTargetMatrix", "", "rejects Result<T>"},
	"haskell":    {TierCompiled, "TestCompiledTier/haskell", "ghc", "rejects Result<T>"},
	"ocaml":      {TierCompiled, "TestCompiledTier/ocaml", "ocamlc", "rejects Result<T>"},
	"fsharp":     {TierSmoke, "TestExampleTargetMatrix", "", "rejects Result<T>"},
	"ada":        {TierSmoke, "TestExampleTargetMatrix", "", "rejects Result<T>"},
	"crystal":    {TierCompiled, "TestCompiledTier/crystal", "crystal", "rejects Result<T>"},
	"d":          {TierSmoke, "TestExampleTargetMatrix", "", "rejects Result<T>"},
	"fortran":    {TierCompiled, "TestCompiledTier/fortran", "gfortran", "rejects for-each loops"},
	"objc":       {TierSmoke, "TestExampleTargetMatrix", "", "rejects Result<T>"},
	"pascal":     {TierSmoke, "TestExampleTargetMatrix", "", "rejects for-each loops"},
	"powershell": {TierCompiled, "TestCompiledTier/powershell", "pwsh", "rejects Result<T>"},
	"v":          {TierSmoke, "TestExampleTargetMatrix", "", "rejects Result<T>"},
	"elixir":     {TierCompiled, "TestCompiledTier/elixir", "elixir", "rejects Result<T>"},
	"clojure":    {TierSmoke, "TestExampleTargetMatrix", "", "rejects Result<T>"},
	"vala":       {TierSmoke, "TestExampleTargetMatrix", "", "rejects Result<T>"},
	"groovy":     {TierSmoke, "TestExampleTargetMatrix", "", "rejects Result<T>"},
	"bat":        {TierSmoke, "TestExampleTargetMatrix", "", "rejects struct literals"},
	"shortcut":   {TierSmoke, "TestExampleTargetMatrix", "", "emits an Apple Shortcuts plist, not source; rejects Result<T>"},
	"chrome":     {TierSmoke, "TestExampleTargetMatrix", "", "emits an extension bundle; rejects Result<T>"},
	"tccli":      {TierCompiled, "TestCompiledTier/tccli", "bash", "emits Tencent Cloud CLI shell; no arithmetic, comparisons or structs"},

	// Project scaffolds. Assembling these needs an SDK whose version triple
	// drifts with the runner image, so CI checks their structure instead and
	// leans on the kotlin/swift executed tiers for the source they contain.
	"android": {TierSmoke, "TestAndroidScaffoldStructure", "", "Gradle scaffold; structure checked, never assembled"},
	"ios":     {TierSmoke, "TestLocalE2EProjectScaffolds/iOS", "swift", "SwiftPM scaffold; built only when swift is present"},
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
