package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"xiaoqinli/internal/e2e"
)

// Everything that actually ran a generated program used to live in
// codegen/local_e2e_test.go, which calls GenerateJava, GenerateKotlin and so on
// one module at a time. That path is worth testing — it is how the target
// language's own module system gets exercised — but it is not the path the CLI
// takes, and it skips the pipeline's own checks. Nim sat in that table for
// months even though `CompileFromFile` refuses nim for this program: the
// backend has no Result<T>, and calling the generator directly walked straight
// past the validation that says so.
//
// This test drives the documented pipeline instead — parse, check, link,
// codegen, exactly as `xql compile --file` does — and then builds and runs what
// comes out. It covers the four targets a reader is most likely to reach for
// first, none of which had ever been executed anywhere.

// canaryFails reports why the local toolchain cannot build a trivial program of
// its own, or "" when it can.
//
// Without this a broken environment reads as a compiler bug. A Git Bash PATH
// puts coreutils `link` ahead of MSVC's `link.exe`, so rustc emits correct code
// and then fails to link it — a failure about the machine that would otherwise
// be reported against the backend.
func canaryFails(t *testing.T, tool, outName, src string, extra map[string]string, steps []string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range extra {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write canary %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, outName), []byte(src), 0o644); err != nil {
		t.Fatalf("write canary: %v", err)
	}
	for _, step := range steps {
		fields := strings.Fields(strings.ReplaceAll(step, "{tool}", tool))
		cmd := exec.Command(fields[0], fields[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return step + ": " + err.Error() + "\n" + string(out)
		}
	}
	return ""
}

func TestLinkedPipelineE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping toolchain-driven E2E in -short mode")
	}

	entry := filepath.Join("..", "examples", "e2e_workspace", "main.xql")

	cases := []struct {
		name string
		// target is the compile flag.
		target string
		// out is the filename the generated source is written as.
		out string
		// tools are acceptable executables; the first present is used, and its
		// name replaces {tool} in steps.
		tools []string
		// extra files the toolchain needs to see the source as a program.
		extra map[string]string
		// steps run in order in the temp directory.
		steps []string
		// canary is a trivial program in the target language. If the toolchain
		// cannot build this, the environment is broken, not the backend.
		canary string
	}{
		{
			name:   "Go",
			target: "go",
			out:    "main.go",
			tools:  []string{"go"},
			// Module mode refuses to run a bare file outside a module.
			extra:  map[string]string{"go.mod": "module xqle2e\n\ngo 1.21\n"},
			steps:  []string{"{tool} run main.go"},
			canary: "package main\n\nfunc main() { println(\"ok\") }\n",
		},
		{
			name:   "Python",
			target: "py",
			out:    "main.py",
			tools:  []string{"python3", "python"},
			steps:  []string{"{tool} main.py"},
			canary: "print(\"ok\")\n",
		},
		{
			name:   "Rust",
			target: "rust",
			out:    "main.rs",
			tools:  []string{"rustc"},
			steps:  []string{"{tool} main.rs -o main_rs", "./main_rs"},
			canary: "fn main() { println!(\"ok\"); }\n",
		},
		{
			name:   "TypeScript",
			target: "ts",
			out:    "main.ts",
			tools:  []string{"tsx", "ts-node"},
			steps:  []string{"{tool} main.ts"},
			canary: "console.log(\"ok\");\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Compile through the same entry point the CLI uses, so a target the
			// pipeline would refuse cannot quietly be tested anyway.
			res := CompileFromFile(entry, tc.target, "")
			if !res.Success {
				t.Fatalf("pipeline refused %s: %s", tc.target, res.Error)
			}
			if len(strings.TrimSpace(string(res.Code))) == 0 {
				t.Fatalf("%s compiled to empty output", tc.target)
			}
			if err := os.WriteFile(filepath.Join(tmpDir, tc.out), res.Code, 0o644); err != nil {
				t.Fatalf("write %s: %v", tc.out, err)
			}
			for name, content := range tc.extra {
				if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644); err != nil {
					t.Fatalf("write %s: %v", name, err)
				}
			}

			tool := e2e.FirstWorking(tc.tools...)
			if tool == "" {
				msg := "none of %v is on PATH, so the generated program was produced but never run"
				if e2e.Required() {
					t.Fatalf("XQL_E2E_REQUIRE is set, so this must not be skipped: "+msg, tc.tools)
				}
				t.Skipf(msg, tc.tools)
			}

			if why := canaryFails(t, tool, tc.out, tc.canary, tc.extra, tc.steps); why != "" {
				msg := "the local %s toolchain cannot build a hello-world, so nothing here would be evidence about the backend:\n%s"
				if e2e.Required() {
					t.Fatalf("XQL_E2E_REQUIRE is set, so this must not be skipped: "+msg, tc.name, why)
				}
				t.Skipf(msg, tc.name, why)
			}

			var last string
			for _, step := range tc.steps {
				fields := strings.Fields(strings.ReplaceAll(step, "{tool}", tool))
				cmd := exec.Command(fields[0], fields[1:]...)
				cmd.Dir = tmpDir
				out, err := cmd.CombinedOutput()
				last = string(out)
				if err != nil {
					t.Fatalf("step %q failed: %v\noutput:\n%s\ngenerated %s:\n%s",
						step, err, last, tc.out, res.Code)
				}
			}

			// The program prints the names in models.xql. Checking stdout is what
			// separates "it compiled" from "it means the same thing".
			for _, want := range []string{"Alice", "Bob"} {
				if !strings.Contains(last, want) {
					t.Errorf("expected %q in output, got:\n%s", want, last)
				}
			}
		})
	}
}

// TestLinkedPipelineErrPathPython covers the branch the happy path never takes.
// The generated Result exposes unwrap_err, and until recently call sites still
// said unwrapErr, so every Err return was an AttributeError that no test could
// see because no test ever produced an Err.
func TestLinkedPipelineErrPathPython(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping toolchain-driven E2E in -short mode")
	}

	res := CompileFromFile(filepath.Join("..", "examples", "e2e_workspace", "main.xql"), "py", "")
	if !res.Success {
		t.Fatalf("python codegen failed: %s", res.Error)
	}
	if strings.Contains(string(res.Code), ".unwrapErr(") {
		t.Errorf("generated Python calls unwrapErr, but the emitted Result defines unwrap_err")
	}
	if !strings.Contains(string(res.Code), "def unwrap_err") {
		t.Fatalf("generated Python has no unwrap_err to call:\n%s", res.Code)
	}

	tool := e2e.FirstWorking("python3", "python")
	if tool == "" {
		if e2e.Required() {
			t.Fatal("XQL_E2E_REQUIRE is set, so this must not be skipped: no python on PATH")
		}
		t.Skip("no python on PATH")
	}

	// Drive the Err branch directly: the example only ever returns Ok.
	probe := string(res.Code) + "\n" +
		"_e = Result.err(\"boom\")\n" +
		"assert not _e.is_ok\n" +
		"print(\"ERRPATH\", _e.unwrap_err())\n"

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "errpath.py")
	if err := os.WriteFile(path, []byte(probe), 0o644); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	cmd := exec.Command(tool, "errpath.py")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Err path failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(string(out), "ERRPATH boom") {
		t.Errorf("expected the Err branch to unwrap cleanly, got:\n%s", out)
	}
}
