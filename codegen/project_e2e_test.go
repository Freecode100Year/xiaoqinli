// Package codegen_test holds the end-to-end checks for the multi-file project
// scaffolds. They live in an external test package because they drive the
// whole pipeline through the compiler, and compiler imports codegen.
package codegen_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"xiaoqinli/compiler"
)

// TestLocalE2EProjectScaffolds builds the generated Android and iOS projects
// with their real toolchains.
//
// Unlike the single-file backends, these targets emit a whole project tree, so
// they are compiled through compiler.Compile rather than the codegen entry
// point directly: the entry program imports two modules, and it is the linker
// that merges them into the self-contained Program a scaffold needs. Calling
// the backend with an unlinked AST yields sources that reference models.Config
// and service.fetchUsers, which no toolchain can resolve.
func TestLocalE2EProjectScaffolds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping toolchain-driven E2E in -short mode")
	}

	entry := filepath.Join("..", "examples", "e2e_workspace", "main.xql")

	cases := []struct {
		name     string
		target   string
		checkCmd string
		runCmd   string
	}{
		{
			name:     "Android",
			target:   "android",
			checkCmd: "gradle",
			runCmd:   "gradle assembleDebug",
		},
		{
			name:     "iOS",
			target:   "ios",
			checkCmd: "swift",
			runCmd:   "swift build",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := exec.LookPath(tc.checkCmd); err != nil {
				t.Skipf("Local toolchain %q not found in PATH. Skipping physical build verification.", tc.checkCmd)
			}
			if tc.checkCmd == "gradle" {
				if os.Getenv("ANDROID_HOME") == "" && os.Getenv("ANDROID_SDK_ROOT") == "" {
					t.Skip("Local Gradle found, but ANDROID_HOME / ANDROID_SDK_ROOT is not set. Skipping physical build verification.")
				}
			}

			tmpDir := t.TempDir()
			result := compiler.CompileFromFile(entry, tc.target, tmpDir)
			if !result.Success {
				t.Fatalf("compile to %s failed: %s", tc.target, result.Error)
			}
			if len(result.Files) == 0 {
				t.Fatalf("target %q produced no project files", tc.target)
			}

			args := strings.Fields(tc.runCmd)
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = tmpDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Step %q failed: %v\nOutput: %s\n%s",
					tc.runCmd, err, out, dumpTree(tmpDir))
			}
		})
	}
}

func dumpTree(root string) string {
	var b strings.Builder
	b.WriteString("\n=== DEBUG: Generated Files Content ===\n")
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || !info.Mode().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		content, _ := os.ReadFile(path)
		b.WriteString(fmt.Sprintf("--- File: %s ---\n%s\n", rel, string(content)))
		return nil
	})
	b.WriteString("======================================\n")
	return b.String()
}
