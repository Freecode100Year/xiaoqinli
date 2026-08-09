package compiler

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A backend that cannot express a construct is supposed to reject it rather
// than degrade it silently. Result<T, E> was the place that promise leaked:
// only a handful of backends ever implemented it, and the rest inherited the
// appearance of support from the dispatcher. They emitted `Result.ok(users)`
// and `res.unwrap()` verbatim — a module that does not exist in Haskell, a
// missing command in PowerShell, an undeclared symbol nearly everywhere else —
// and codegen reported success for all of it.
//
// Twenty-two targets were in that state. They were found by asking one
// question of every backend at once, which is what this test keeps asking:
// if the output names Result, something in the output had better define it.

// resultDefinition matches a plausible declaration of a Result type in any of
// the surface syntaxes these backends emit.
var resultDefinition = regexp.MustCompile(
	`(?i)\b(class|struct|type|def|fn|func|function|record|data|interface|sub|proc|namespace|module|enum)\s+Result\b` +
		`|Result\s*=\s*(\{|function|class)`)

// nativeResultTargets reference a Result they did not have to define, so the
// question above does not apply to them. Both are in the executed tier: their
// generated programs are compiled, run, and their stdout checked in CI, which
// is stronger evidence than this test could produce.
var nativeResultTargets = map[string]string{
	"rust": "std::result::Result is in the prelude",
	"zig":  "the backend emits a separate result.zig module alongside main.zig",
}

func TestNoBackendFakesResultSupport(t *testing.T) {
	entry := filepath.Join("..", "examples", "e2e_workspace", "main.xql")

	targets := GetSupportedTargets()
	sort.Strings(targets)

	accepted := 0
	for _, target := range targets {
		res := CompileFromFile(entry, target, "")
		if !res.Success {
			// Declining is the correct outcome for a backend without Result.
			// The matrix test asserts the refusal carries XQL_E402.
			continue
		}
		accepted++

		code := string(res.Code)
		for _, content := range res.Files {
			code += "\n" + string(content)
		}

		references := strings.Contains(code, "Result.") || strings.Contains(code, "unwrap")
		if !references {
			// Lowered to something native — Go returns a tuple, Julia its own
			// shape. Nothing to check.
			continue
		}
		if why, ok := nativeResultTargets[target]; ok {
			if !references {
				t.Errorf("%s is listed as using a native Result but references none; the entry is stale (%s)", target, why)
			}
			continue
		}
		if !resultDefinition.MatchString(code) {
			t.Errorf("%s compiles a program using Result<T, E> and emits references to it, "+
				"but defines no Result anywhere in its output — the generated code cannot work. "+
				"Either implement Result for this backend or add it to unsupportedResultTargets so it "+
				"declines with XQL_E402 instead of producing something broken.", target)
		}
	}

	if accepted == 0 {
		t.Fatal("no target accepted the Result example, so this test verified nothing")
	}
}

// TestNativeResultTargetsAreAdvertised keeps the allowlist from outliving the
// targets it excuses.
func TestNativeResultTargetsAreAdvertised(t *testing.T) {
	advertised := map[string]bool{}
	for _, flag := range GetSupportedTargets() {
		advertised[flag] = true
	}
	for flag := range nativeResultTargets {
		if !advertised[flag] {
			t.Errorf("nativeResultTargets excuses %q, which is not an advertised target", flag)
		}
		v, ok := VerificationFor(flag)
		if !ok || v.Tier != TierExecuted {
			t.Errorf("%q is excused from the Result definition check because its programs are executed; "+
				"it is not in the executed tier, so the excuse no longer holds", flag)
		}
	}
}
