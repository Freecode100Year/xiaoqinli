package codegen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"xiaoqinli/ast"
)

func runLocalE2E(t *testing.T, checkCmd string, runCmd string, files map[string][]byte) {
	// 1. Check if compiler/runtime is available locally
	parts := strings.Fields(checkCmd)
	if len(parts) > 0 {
		_, err := exec.LookPath(parts[0])
		if err != nil {
			t.Skipf("Local compiler/runtime %q not found in PATH. Skipping physical execution verification.", parts[0])
			return
		}
		if parts[0] == "dotnet" {
			out, err := exec.Command("dotnet", "--list-sdks").Output()
			if err != nil || len(strings.TrimSpace(string(out))) == 0 {
				t.Skip("Local .NET SDK not found. Skipping physical execution verification.")
				return
			}
		}
	}

	// 2. Write generated source files to a temporary directory
	tmpDir := t.TempDir()
	for name, content := range files {
		_ = os.WriteFile(filepath.Join(tmpDir, name), content, 0644)
	}

	// 3. Compile & run command steps
	steps := strings.Split(runCmd, ";")
	var lastOutput string
	for _, step := range steps {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}

		// Handle C# Program.cs cleanup cross-platform
		if strings.Contains(step, "Remove-Item") || strings.Contains(step, "Program.cs") && (strings.Contains(step, "rm ") || strings.Contains(step, "Remove-Item")) {
			programPath := filepath.Join(tmpDir, "Program.cs")
			if _, err := os.Stat(programPath); err == nil {
				_ = os.Remove(programPath)
			}
			continue
		}

		args := strings.Fields(step)
		if len(args) == 0 {
			continue
		}

		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		out, err := cmd.CombinedOutput()
		lastOutput = string(out)

		if err != nil {
			var fileDebug strings.Builder
			fileDebug.WriteString("\n=== DEBUG: Generated Files Content ===\n")
			entries, _ := os.ReadDir(tmpDir)
			for _, entry := range entries {
				if entry.Type().IsRegular() {
					content, _ := os.ReadFile(filepath.Join(tmpDir, entry.Name()))
					fileDebug.WriteString(fmt.Sprintf("--- File: %s ---\n%s\n", entry.Name(), string(content)))
				}
			}
			fileDebug.WriteString("======================================\n")
			t.Fatalf("Step %q failed: %v\nOutput: %s\n%s", step, err, lastOutput, fileDebug.String())
		}
	}

	// 4. Assert stdout contains target names from workspace dogfood XQL data
	if !strings.Contains(lastOutput, "Alice") || !strings.Contains(lastOutput, "Bob") {
		t.Fatalf("Output assertion failed. Expected output to contain 'Alice' and 'Bob'. Actual output:\n%s", lastOutput)
	}
}

func TestLocalE2EWorkspaceDogfood(t *testing.T) {
	modelsPath := filepath.Join("..", "examples", "e2e_workspace", "models.xql")
	servicePath := filepath.Join("..", "examples", "e2e_workspace", "service.xql")
	mainPath := filepath.Join("..", "examples", "e2e_workspace", "main.xql")

	modelsSrc, err := os.ReadFile(modelsPath)
	if err != nil {
		t.Fatalf("failed to read models.xql: %v", err)
	}
	serviceSrc, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("failed to read service.xql: %v", err)
	}
	mainSrc, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("failed to read main.xql: %v", err)
	}

	testCases := []struct {
		name     string
		target   string
		checkCmd string
		runCmd   string
		files    map[string]string
	}{
		{
			name:     "Java",
			target:   "java",
			checkCmd: "javac",
			runCmd:   "javac Main.java Service.java Models.java ; java Main",
			files: map[string]string{
				"main.xql":    "Main.java",
				"service.xql": "Service.java",
				"models.xql":  "Models.java",
			},
		},
		{
			name:     "CSharp",
			target:   "csharp",
			checkCmd: "dotnet",
			runCmd:   "dotnet new console --force ; Remove-Item -ErrorAction SilentlyContinue Program.cs ; dotnet run",
			files: map[string]string{
				"main.xql":    "main.cs",
				"service.xql": "service.cs",
				"models.xql":  "models.cs",
			},
		},
		{
			name:     "Kotlin",
			target:   "kotlin",
			checkCmd: "kotlinc",
			runCmd:   "kotlinc main.kt service.kt models.kt -include-runtime -d main.jar ; java -jar main.jar",
			files: map[string]string{
				"main.xql":    "main.kt",
				"service.xql": "service.kt",
				"models.xql":  "models.kt",
			},
		},
		{
			name:     "Swift",
			target:   "swift",
			checkCmd: "swiftc",
			runCmd:   "swiftc main.swift service.swift models.swift -o main_out ; ./main_out",
			files: map[string]string{
				"main.xql":    "main.swift",
				"service.xql": "service.swift",
				"models.xql":  "models.swift",
			},
		},
		{
			name:     "Dart",
			target:   "dart",
			checkCmd: "dart",
			runCmd:   "dart run main.dart service.dart models.dart",
			files: map[string]string{
				"main.xql":    "main.dart",
				"service.xql": "service.dart",
				"models.xql":  "models.dart",
			},
		},
		{
			name:     "Zig",
			target:   "zig",
			checkCmd: "zig",
			runCmd:   "zig build-exe main.zig ; ./main",
			files: map[string]string{
				"main.xql":    "main.zig",
				"service.xql": "service.zig",
				"models.xql":  "models.zig",
				"result.zig":  "result.zig",
			},
		},
		{
			name:     "Nim",
			target:   "nim",
			checkCmd: "nim",
			runCmd:   "nim c -r main.nim",
			files: map[string]string{
				"main.xql":    "main.nim",
				"service.xql": "service.nim",
				"models.xql":  "models.nim",
			},
		},
		{
			name:     "Julia",
			target:   "julia",
			checkCmd: "julia",
			runCmd:   "julia main.jl",
			files: map[string]string{
				"main.xql":    "main.jl",
				"service.xql": "service.jl",
				"models.xql":  "models.jl",
			},
		},
		{
			name:     "PHP",
			target:   "php",
			checkCmd: "php",
			runCmd:   "php main.php",
			files: map[string]string{
				"main.xql":    "main.php",
				"service.xql": "service.php",
				"models.xql":  "models.php",
			},
		},
		{
			name:     "Ruby",
			target:   "ruby",
			checkCmd: "ruby",
			runCmd:   "ruby main.rb",
			files: map[string]string{
				"main.xql":    "main.rb",
				"service.xql": "service.rb",
				"models.xql":  "models.rb",
			},
		},
		{
			name:     "Lua",
			target:   "lua",
			checkCmd: "lua",
			runCmd:   "lua main.lua",
			files: map[string]string{
				"main.xql":    "main.lua",
				"service.xql": "service.lua",
				"models.xql":  "models.lua",
			},
		},
		{
			name:     "Android",
			target:   "android",
			checkCmd: "gradle",
			runCmd:   "gradle assembleDebug",
			files: map[string]string{
				"main.xql": "app/src/main/java/com/xql/app/MainActivity.kt",
			},
		},
		{
			name:     "iOS",
			target:   "ios",
			checkCmd: "swift",
			runCmd:   "swift build",
			files: map[string]string{
				"main.xql": "Sources/XqlApp/main.swift",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 1. Run full codegen in memory to test translation pipeline
			filesMap := make(map[string][]byte)
			for xqlFile, targetFile := range tc.files {
				// result.zig is a shared module generated by a dedicated function
				if xqlFile == "result.zig" {
					filesMap[targetFile] = GenerateZigResultFile()
					continue
				}
				var src []byte
				switch xqlFile {
				case "main.xql":
					src = mainSrc
				case "service.xql":
					src = serviceSrc
				case "models.xql":
					src = modelsSrc
				}

				node, err := ast.Parse(src)
				if err != nil {
					t.Fatalf("parse failed for %s: %v", xqlFile, err)
				}

				var out []byte
				switch tc.target {
				case "java":
					out, err = GenerateJava(node)
				case "csharp":
					out, err = GenerateCSharp(node)
				case "kotlin":
					out, err = GenerateKotlin(node)
				case "swift":
					out, err = GenerateSwift(node)
				case "dart":
					out, err = GenerateDart(node)
				case "zig":
					out, err = GenerateZig(node)
				case "nim":
					out, err = GenerateNim(node)
				case "julia":
					out, err = GenerateJulia(node)
				case "php":
					out, err = GeneratePHP(node)
				case "ruby":
					out, err = GenerateRuby(node)
				case "lua":
					out, err = GenerateLua(node)
				case "android":
					proj, pErr := GenerateAndroidProject(node)
					err = pErr
					if proj != nil {
						for fName, fBytes := range proj.Files {
							filesMap[fName] = fBytes
						}
						out = proj.MainCode
					}
				case "ios":
					proj, pErr := GenerateIOSProject(node)
					err = pErr
					if proj != nil {
						for fName, fBytes := range proj.Files {
							filesMap[fName] = fBytes
						}
						out = proj.MainCode
					}
				}

				if err != nil {
					t.Fatalf("codegen failed for %s: %v", targetFile, err)
				}
				filesMap[targetFile] = out
			}

			// 2. Run physical compile & run verification locally if toolchain exists
			runLocalE2E(t, tc.checkCmd, tc.runCmd, filesMap)
		})
	}
}
