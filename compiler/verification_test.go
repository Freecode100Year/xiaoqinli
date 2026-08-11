package compiler

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// updateGolden rewrites the README tables from the registry instead of failing.
var updateGolden = flag.Bool("update", false, "rewrite generated blocks in the READMEs from targetVerification")

// A registry that can drift from the code is just a second place to be wrong.
// These tests tie every claim in verification.go to something observable: an
// advertised flag, a test function that exists, a toolchain CI installs, a row
// in the README.

func TestEveryAdvertisedTargetDeclaresVerification(t *testing.T) {
	for _, flag := range GetSupportedTargets() {
		if _, ok := VerificationFor(flag); !ok {
			t.Errorf("target %q is advertised but declares no verification tier — "+
				"add it to targetVerification and say honestly how it is checked", flag)
		}
	}

	advertised := map[string]bool{}
	for _, flag := range GetSupportedTargets() {
		advertised[flag] = true
	}
	for _, tier := range []VerificationTier{TierSmoke, TierCompiled, TierExecuted} {
		for _, flag := range TargetsAtTier(tier) {
			if !advertised[flag] {
				t.Errorf("targetVerification has an entry for %q, which is not an advertised target", flag)
			}
		}
	}
}

// TestExecutedTierNamesARealTest catches the registry claiming evidence from a
// harness that was renamed or deleted. Without it, a tier outlives the test
// that produced it. Both tiers above smoke are checked: either can go stale.
func TestExecutedTierNamesARealTest(t *testing.T) {
	sources := readTestSources(t)

	var claimants []string
	claimants = append(claimants, TargetsAtTier(TierExecuted)...)
	claimants = append(claimants, TargetsAtTier(TierCompiled)...)

	for _, flag := range claimants {
		v, _ := VerificationFor(flag)
		if v.Harness == "" {
			t.Errorf("%s claims the %s tier but names no harness", flag, v.Tier)
			continue
		}
		// "TestLinkedPipelineE2E/Go" -> the function, then the subtest name.
		fn, sub, _ := strings.Cut(v.Harness, "/")
		if !strings.Contains(sources, "func "+fn+"(") {
			t.Errorf("%s claims evidence from %s, but no such test function exists", flag, fn)
			continue
		}
		if sub != "" && !strings.Contains(sources, `"`+sub+`"`) {
			t.Errorf("%s claims evidence from subtest %q of %s, but nothing declares that case name",
				flag, sub, fn)
		}
	}
}

// TestExecutedTierRequiresAToolchain keeps the tier meaningful: running a
// program needs something to run it with, and CI can only install what the
// registry names.
func TestExecutedTierRequiresAToolchain(t *testing.T) {
	for _, flag := range TargetsAtTier(TierExecuted) {
		v, _ := VerificationFor(flag)
		if v.Toolchain == "" {
			t.Errorf("%s claims the executed tier but names no toolchain, so CI cannot guarantee it ran", flag)
		}
	}
	if len(RequiredToolchains()) == 0 {
		t.Fatal("no target is executed anywhere — the executed tier has become decoration")
	}
}

// TestCIInstallsEveryRequiredToolchain is the one that would have caught the
// long-standing gap: the E2E table listed eleven targets while the workflow
// installed six, so five subtests skipped on every run and reported as passes.
func TestCIInstallsEveryRequiredToolchain(t *testing.T) {
	workflow := readFile(t, filepath.Join("..", ".github", "workflows", "e2e-backends.yml"))

	// Toolchains the runner image already provides need no install step.
	preinstalled := map[string]bool{
		"python3": true,
		"rustc":   true,
		"php":     true,
		"ruby":    true,

		// The conformance corpus runs in these too. All of them ship on
		// ubuntu-latest, which is why that corpus costs one apt package.
		"node": true,
		"gcc":  true,
		"g++":  true,
		"perl": true,
		"bash": true,
		"gawk": true,
		"pwsh": true,

		// gfortran ships with the image too — the compiled tier has leaned on
		// that since it existed, and the corpus only adds a run step.
		"gfortran": true,
	}
	// What the workflow calls the thing it installs, when that differs from the
	// executable name the test looks for.
	installedAs := map[string]string{
		"javac":   "setup-java",
		"dotnet":  "setup-dotnet",
		"kotlinc": "setup-kotlin",
		"swiftc":  "setup-swift",
		"dart":    "setup-dart",
		"zig":     "setup-zig",
		"julia":   "setup-julia",
		"go":      "setup-go",
		"lua":     "lua5.4",
		"tsx":     "tsx",
		"tclsh":   "tcl",
	}

	for _, tool := range RequiredToolchains() {
		if preinstalled[tool] {
			continue
		}
		needle, ok := installedAs[tool]
		if !ok {
			t.Errorf("no idea how CI would install %q — add it to installedAs, and to the workflow", tool)
			continue
		}
		if !strings.Contains(workflow, needle) {
			t.Errorf("the executed tier needs %q but e2e-backends.yml never installs it (looked for %q). "+
				"Either add the step or drop that target out of TierExecuted — a tier CI cannot run is a claim, not evidence.",
				tool, needle)
		}
	}

	if !strings.Contains(workflow, "XQL_E2E_REQUIRE") {
		t.Error("e2e-backends.yml must set XQL_E2E_REQUIRE, or a missing toolchain silently skips and the run still reports green")
	}
}

// TestReadmeCoverageTableMatchesRegistry keeps the published claim and the
// checked fact in sync. The android Result defect survived a release because
// those were two unrelated pieces of text.
func TestReadmeCoverageTableMatchesRegistry(t *testing.T) {
	const begin = "<!-- verification:begin -->"
	const end = "<!-- verification:end -->"

	for _, name := range []string{"README.md", "README.zh-CN.md"} {
		path := filepath.Join("..", name)
		readme := readFile(t, path)

		block, ok := extractBlock(readme, begin, end)
		if !ok {
			t.Fatalf("%s has no verification block — restore the %s / %s markers", name, begin, end)
		}
		want := RenderVerificationTable()
		if strings.TrimSpace(block) == strings.TrimSpace(want) {
			continue
		}
		if *updateGolden {
			i := strings.Index(readme, begin)
			j := strings.Index(readme[i:], end) + i
			updated := readme[:i+len(begin)] + want + readme[j:]
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				t.Fatalf("rewrite %s: %v", name, err)
			}
			t.Logf("regenerated the verification table in %s", name)
			continue
		}
		t.Errorf("%s's verification table is stale. Regenerate it with:\n"+
			"    go test ./compiler -run TestReadmeCoverageTableMatchesRegistry -update\n"+
			"want:\n%s\n\ngot:\n%s", name, want, block)
	}
}

func extractBlock(s, begin, end string) (string, bool) {
	i := strings.Index(s, begin)
	if i < 0 {
		return "", false
	}
	j := strings.Index(s[i:], end)
	if j < 0 {
		return "", false
	}
	return s[i+len(begin) : i+j], true
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// readTestSources concatenates every _test.go file in the repository, so a
// harness can be found wherever it lives.
func readTestSources(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr == nil {
			b.Write(content)
			b.WriteByte('\n')
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk for test sources: %v", err)
	}
	if b.Len() == 0 {
		t.Fatal("found no test sources to check harness names against")
	}
	return b.String()
}

// TestRequiredToolchainsIsStable guards the ordering RenderVerificationTable
// depends on, so regenerating the README does not produce spurious diffs.
func TestRequiredToolchainsIsStable(t *testing.T) {
	first := RequiredToolchains()
	for i := 0; i < 5; i++ {
		if got := RequiredToolchains(); !equalStrings(first, got) {
			t.Fatalf("RequiredToolchains is not deterministic: %v then %v", first, got)
		}
	}
	if !sort.StringsAreSorted(first) {
		t.Errorf("RequiredToolchains should be sorted, got %v", first)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
