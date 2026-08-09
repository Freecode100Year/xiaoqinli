// Package e2e holds the shared rules for tests that drive real language
// toolchains. It exists because three test packages need the same contract and
// a contract stated three times is a contract that will eventually differ.
//
// It imports testing, as internal/testenv does in the standard library: nothing
// outside _test.go files imports this package.
package e2e

import (
	"os"
	"os/exec"
	"testing"
)

// EnvRequire is the variable CI sets to forbid skipping.
const EnvRequire = "XQL_E2E_REQUIRE"

// Required reports whether a missing toolchain must fail rather than skip.
//
// A skipped Go subtest is reported as a pass. On a developer machine that is
// correct — nobody has fourteen language toolchains installed. In CI it is how
// a release came to advertise green tests while the pipeline was red: the
// subtests that would have caught it never ran, and a skip looks like a pass in
// the summary. Setting XQL_E2E_REQUIRE=1 makes "green" mean the generated
// programs were actually built and run.
func Required() bool {
	return os.Getenv(EnvRequire) != ""
}

// Missing skips the test, or fails it when the environment promised the
// toolchain would be present.
func Missing(t testing.TB, format string, args ...interface{}) {
	t.Helper()
	if Required() {
		t.Fatalf(EnvRequire+" is set, so this must not be skipped: "+format, args...)
	}
	t.Skipf(format, args...)
}

// FirstWorking returns the first named executable that is present and actually
// runs, so a case can accept either python3 or python without pretending they
// are different verifications. It returns "" when none works.
//
// Being on PATH is not the same as working: Windows ships a python3 stub that
// resolves happily and then exits 9009. `go` spells its probe `go version`
// while everything else takes --version, so both are tried.
func FirstWorking(names ...string) string {
	for _, n := range names {
		path, err := exec.LookPath(n)
		if err != nil {
			continue
		}
		if exec.Command(path, "--version").Run() != nil &&
			exec.Command(path, "version").Run() != nil {
			continue
		}
		return n
	}
	return ""
}
