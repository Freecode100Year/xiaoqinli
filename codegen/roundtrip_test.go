package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

)

// TestRoundTrip generates source code from the hello program, compiles/runs it
// with real toolchains, and verifies stdout. Missing toolchains are skipped.
func TestRoundTrip(t *testing.T) {
	root := mustParse(t, helloProgram)
	expected := "Hello, World"

	cases := []struct {
		target   string
		ext      string
		compiler string       // exec.LookPath key
		run      func(t *testing.T, path string) string
	}{
		{
			target:   "go",
			ext:      ".go",
			compiler: "go",
			run:      runGo,
		},
		{
			target:   "py",
			ext:      ".py",
			compiler: "python",
			run:      runPython,
		},
		{
			target:   "rust",
			ext:      ".rs",
			compiler: "rustc",
			run:      runRust,
		},
		{
			target:   "ts",
			ext:      ".ts",
			compiler: "node",
			run:      runTS,
		},
		{
			target:   "cpp",
			ext:      ".cpp",
			compiler: "g++",
			run:      runCpp,
		},
	}

	for _, tc := range cases {
		t.Run("roundtrip_"+tc.target, func(t *testing.T) {
			if _, err := exec.LookPath(tc.compiler); err != nil {
				t.Skipf("toolchain %q not found, skipping", tc.compiler)
			}

			code, err := Generate(root, tc.target)
			if err != nil {
				t.Fatalf("Generate(%q): %v", tc.target, err)
			}

			dir := t.TempDir()
			path := filepath.Join(dir, "main"+tc.ext)
			if err := os.WriteFile(path, code, 0644); err != nil {
				t.Fatalf("write file: %v", err)
			}

			stdout := tc.run(t, path)
			if !strings.Contains(stdout, expected) {
				t.Errorf("expected stdout to contain %q, got:\n%s", expected, stdout)
			}
		})
	}
}

func runGo(t *testing.T, path string) string {
	t.Helper()
	cmd := exec.Command("go", "run", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, out)
	}
	return string(out)
}

func runPython(t *testing.T, path string) string {
	t.Helper()
	cmd := exec.Command("python", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("python failed: %v\n%s", err, out)
	}
	return string(out)
}

func runRust(t *testing.T, path string) string {
	t.Helper()
	dir := filepath.Dir(path)
	bin := filepath.Join(dir, "main.exe")

	cmd := exec.Command("rustc", path, "-o", bin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rustc failed: %v\n%s", err, out)
	}

	cmd = exec.Command(bin)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rust binary failed: %v\n%s", err, out)
	}
	return string(out)
}

func runCpp(t *testing.T, path string) string {
	t.Helper()
	dir := filepath.Dir(path)
	bin := filepath.Join(dir, "main.exe")

	cmd := exec.Command("g++", "-std=c++17", path, "-o", bin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("g++ failed: %v\n%s", err, out)
	}

	cmd = exec.Command(bin)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cpp binary failed: %v\n%s", err, out)
	}
	return string(out)
}

func runTS(t *testing.T, path string) string {
	t.Helper()
	// TypeScript via node: strip type annotations and run as JS
	// For a simple program, we can just use ts-node or strip types manually.
	// Node >= 22.6 supports --experimental-strip-types, but let's use
	// a simpler approach: generate JS-compatible output by running through node
	// with --experimental-strip-types, or strip types and run.

	// Try node --experimental-strip-types first
	cmd := exec.Command("node", "--experimental-strip-types", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback: strip type annotations manually and run as JS
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read ts file: %v", readErr)
		}
		jsCode := stripTypeAnnotations(string(data))
		jsPath := strings.TrimSuffix(path, ".ts") + ".js"
		if writeErr := os.WriteFile(jsPath, []byte(jsCode), 0644); writeErr != nil {
			t.Fatalf("write js file: %v", writeErr)
		}
		cmd = exec.Command("node", jsPath)
		out, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node failed: %v\n%s", err, out)
		}
	}
	return string(out)
}

// stripTypeAnnotations does a basic removal of TS type annotations for roundtrip test.
// Handles patterns like ": number", ": string", ": void", ": boolean".
func stripTypeAnnotations(ts string) string {
	// Remove type annotations from function params and return types
	result := ts
	// Remove return type annotations (": type {" or "): type {")
	for _, typ := range []string{"number", "string", "void", "boolean", "any"} {
		result = strings.ReplaceAll(result, ": "+typ, "")
	}
	return result
}
