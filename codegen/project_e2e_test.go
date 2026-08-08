// Package codegen_test holds the end-to-end checks for the multi-file project
// scaffolds. They live in an external test package because they drive the
// whole pipeline through the compiler, and compiler imports codegen.
package codegen_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"xiaoqinli/compiler"
)

// TestLocalE2EProjectScaffolds checks the targets that emit a whole project
// tree rather than a single source file.
//
// They are compiled through compiler.CompileFromFile rather than the codegen
// entry point directly: the entry program imports two modules, and it is the
// linker that merges them into the self-contained Program a scaffold needs.
// Calling the backend with an unlinked AST yields sources referencing
// models.Config and service.fetchUsers, which no toolchain can resolve.
//
// iOS is built for real. Android is verified structurally — see
// TestAndroidScaffoldStructure for why.
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
			name:     "iOS",
			target:   "ios",
			checkCmd: "swift",
			runCmd:   "swift build",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate first, and only then decide whether a real build is
			// possible: generation needs no toolchain, so skipping before it
			// would leave these backends with no coverage at all on a machine
			// that lacks the SDK.
			tmpDir := t.TempDir()
			result := compiler.CompileFromFile(entry, tc.target, tmpDir)
			if !result.Success {
				t.Fatalf("compile to %s failed: %s", tc.target, result.Error)
			}
			if len(result.Files) == 0 {
				t.Fatalf("target %q produced no project files", tc.target)
			}

			if _, err := exec.LookPath(tc.checkCmd); err != nil {
				t.Skipf("Local toolchain %q not found in PATH. Skipping physical build verification.", tc.checkCmd)
			}

			args := strings.Fields(tc.runCmd)
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = tmpDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Step %q failed: %v\nOutput: %s\n%s",
					tc.runCmd, err, out, dumpGenerated(tmpDir, result.Files))
			}
		})
	}
}

// TestAndroidScaffoldStructure verifies the generated Android project without
// assembling an APK.
//
// A real `gradle assembleDebug` depends on an Android SDK, a network, and a
// compatible AGP/Gradle/Kotlin version triple that drifts with the runner
// image — none of which says anything about whether the transpiler is
// correct. The Kotlin the backend emits is already executed for real by
// TestLocalE2EWorkspaceDogfood/Kotlin, so what is left worth checking here is
// that the scaffold is internally consistent: every file present, and every
// resource the manifest names actually generated.
//
// KNOWN GAP: because nothing compiles this Kotlin, the backend's broken
// Result<T> handling is not caught here. androidGen emits no Result type of
// its own, so `Result<List<User>, String>` resolves to kotlin.Result<out T>
// and the build fails with "One type argument expected". This is recorded in
// the README's limitations table; do not mistake a green run here for a
// buildable app when the program uses Result.
func TestAndroidScaffoldStructure(t *testing.T) {
	entry := filepath.Join("..", "examples", "e2e_workspace", "main.xql")

	tmpDir := t.TempDir()
	result := compiler.CompileFromFile(entry, "android", tmpDir)
	if !result.Success {
		t.Fatalf("compile to android failed: %s", result.Error)
	}

	required := []string{
		"build.gradle",
		"settings.gradle",
		"gradle.properties",
		"app/build.gradle",
		"app/src/main/AndroidManifest.xml",
		"app/src/main/java/com/xql/app/MainActivity.kt",
	}
	for _, name := range required {
		if _, ok := result.Files[name]; !ok {
			t.Errorf("scaffold is missing %s", name)
		}
		if _, err := os.Stat(filepath.Join(tmpDir, name)); err != nil {
			t.Errorf("%s was not written to disk: %v", name, err)
		}
	}

	// androidx dependencies do not resolve without this, and the build dies at
	// checkDebugAarMetadata.
	if props := string(result.Files["gradle.properties"]); !strings.Contains(props, "android.useAndroidX=true") {
		t.Errorf("gradle.properties must enable AndroidX, got:\n%s", props)
	}

	// Java and Kotlin must agree on a bytecode target or Gradle refuses to
	// compile the module.
	appGradle := string(result.Files["app/build.gradle"])
	if !strings.Contains(appGradle, "compileOptions") || !strings.Contains(appGradle, "kotlinOptions") {
		t.Errorf("app/build.gradle must pin both JVM targets, got:\n%s", appGradle)
	}

	// Every @drawable/@layout/@string the manifest names has to exist, or aapt
	// fails with "resource not found" — which is how the missing launcher icon
	// went unnoticed.
	manifest := string(result.Files["app/src/main/AndroidManifest.xml"])
	refs := regexp.MustCompile(`@(drawable|layout|mipmap|string)/([A-Za-z0-9_]+)`).FindAllStringSubmatch(manifest, -1)
	if len(refs) == 0 {
		t.Error("expected the manifest to reference at least one resource")
	}
	for _, ref := range refs {
		kind, name := ref[1], ref[2]
		if !resourceExists(result.Files, kind, name) {
			t.Errorf("manifest references @%s/%s but the scaffold generates no such resource", kind, name)
		}
	}

	// The linker should have merged the imported modules, leaving no qualified
	// references for a compiler to choke on.
	kotlin := string(result.Files["app/src/main/java/com/xql/app/MainActivity.kt"])
	for _, alias := range []string{"models.", "service."} {
		if strings.Contains(kotlin, alias) {
			t.Errorf("MainActivity.kt still references unlinked %q:\n%s", alias, kotlin)
		}
	}
}

// resourceExists reports whether the scaffold generates the named resource,
// either as its own file or as an entry in a values file such as strings.xml.
func resourceExists(files map[string][]byte, kind, name string) bool {
	for path, content := range files {
		if !strings.HasPrefix(path, "app/src/main/res/") {
			continue
		}
		if strings.HasPrefix(filepath.Base(path), name+".") &&
			strings.Contains(path, "/"+kind) {
			return true
		}
		if strings.Contains(path, "/values/") &&
			strings.Contains(string(content), `name="`+name+`"`) {
			return true
		}
	}
	return false
}

// dumpGenerated prints the sources the compiler produced, and only those.
// Walking the directory instead would sweep up whatever the build wrote into
// build/ — dex files, packaged archives — and a few megabytes of binary noise
// in the log truncates away the results of any test that ran afterwards.
func dumpGenerated(root string, files map[string][]byte) string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("\n=== DEBUG: Generated Files Content ===\n")
	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			b.WriteString(fmt.Sprintf("--- File: %s (unreadable: %v) ---\n", name, err))
			continue
		}
		b.WriteString(fmt.Sprintf("--- File: %s ---\n%s\n", name, string(content)))
	}
	b.WriteString("======================================\n")
	return b.String()
}
