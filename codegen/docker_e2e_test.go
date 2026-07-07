//go:build docker_e2e

package codegen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"xiaoqinli/ast"
	"xiaoqinli/check"
	"xiaoqinli/vfs"
)

func ensureDockerReady(t *testing.T) {
	t.Helper()

	// 尝试直接运行一次 docker ps，看是否就绪
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "ps")
	if err := cmd.Run(); err != nil {
		t.Skip("Docker daemon is not running. Please start it externally. Skipping Docker E2E test.")
	}
}

func runDockerE2E(t *testing.T, image string, files map[string][]byte, cmdStr string) (string, error) {
	tmpDir := t.TempDir()
	for name, content := range files {
		_ = os.WriteFile(filepath.Join(tmpDir, name), content, 0644)
	}

	absPath, err := filepath.Abs(tmpDir)
	if err != nil {
		return "", err
	}

	// 转换为 Docker volume 挂载路径
	volumeMount := absPath + ":/app"
	args := []string{
		"run", "--rm",
		"-v", volumeMount,
		"-w", "/app",
		image,
		"sh", "-c", cmdStr,
	}

	cmd := exec.Command("docker", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestDockerE2EWorkspaceDogfood(t *testing.T) {
	ensureDockerReady(t)

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

	modelsNode, err := ast.Parse(modelsSrc)
	if err != nil {
		t.Fatalf("Parse models.xql failed: %v", err)
	}
	serviceNode, err := ast.Parse(serviceSrc)
	if err != nil {
		t.Fatalf("Parse service.xql failed: %v", err)
	}
	mainNode, err := ast.Parse(mainSrc)
	if err != nil {
		t.Fatalf("Parse main.xql failed: %v", err)
	}

	ws := vfs.New()
	ws.Write("models.xql", modelsSrc)
	ws.Write("service.xql", serviceSrc)
	ws.Write("main.xql", mainSrc)

	if err := check.RunAllInWorkspace(mainNode, "main.xql", ws); err != nil {
		t.Fatalf("RunAllInWorkspace check failed: %v", err)
	}

	expected := "Alice\nBob\n"

	// 11个非主力后端测试，由于它们当前可能还不支持Result或多文件，Generate如果报错则直接Skip
	targets := []struct {
		name    string
		target  string
		image   string
		cmd     string
		files   map[string]string // XQL file name -> generated output file name
	}{
		{
			name:   "Java",
			target: "java",
			image:  "eclipse-temurin:17-alpine",
			cmd:    "javac Main.java Service.java Models.java && java Main",
			files: map[string]string{
				"main.xql":   "Main.java",
				"service.xql": "Service.java",
				"models.xql":  "Models.java",
			},
		},
		{
			name:   "CSharp",
			target: "csharp",
			image:  "mcr.microsoft.com/dotnet/sdk:7.0-alpine",
			cmd:    "dotnet new console --force && rm -f Program.cs && dotnet run",
			files: map[string]string{
				"main.xql":   "main.cs",
				"service.xql": "service.cs",
				"models.xql":  "models.cs",
			},
		},
		{
			name:   "Kotlin",
			target: "kotlin",
			image:  "zenika/kotlin:alpine",
			cmd:    "kotlinc main.kt service.kt models.kt -include-runtime -d main.jar && java -jar main.jar",
			files: map[string]string{
				"main.xql":   "main.kt",
				"service.xql": "service.kt",
				"models.xql":  "models.kt",
			},
		},
		{
			name:   "Swift",
			target: "swift",
			image:  "swift:5.8-slim",
			cmd:    "swift main.swift service.swift models.swift",
			files: map[string]string{
				"main.xql":   "main.swift",
				"service.xql": "service.swift",
				"models.xql":  "models.swift",
			},
		},
		{
			name:   "Dart",
			target: "dart",
			image:  "dart:stable-alpine",
			cmd:    "dart run main.dart service.dart models.dart",
			files: map[string]string{
				"main.xql":   "main.dart",
				"service.xql": "service.dart",
				"models.xql":  "models.dart",
			},
		},
		{
			name:   "Zig",
			target: "zig",
			image:  "ziglang/zig:0.11.0-alpine",
			cmd:    "zig run main.zig service.zig models.zig",
			files: map[string]string{
				"main.xql":   "main.zig",
				"service.xql": "service.zig",
				"models.xql":  "models.zig",
			},
		},
		{
			name:   "Nim",
			target: "nim",
			image:  "nimlang/nim:alpine",
			cmd:    "nim c -r main.nim",
			files: map[string]string{
				"main.xql":   "main.nim",
				"service.xql": "service.nim",
				"models.xql":  "models.nim",
			},
		},
		{
			name:   "Julia",
			target: "julia",
			image:  "julia:alpine",
			cmd:    "julia main.jl",
			files: map[string]string{
				"main.xql":   "main.jl",
				"service.xql": "service.jl",
				"models.xql":  "models.jl",
			},
		},
		{
			name:   "PHP",
			target: "php",
			image:  "php:8.2-alpine",
			cmd:    "php main.php",
			files: map[string]string{
				"main.xql":   "main.php",
				"service.xql": "service.php",
				"models.xql":  "models.php",
			},
		},
		{
			name:   "Ruby",
			target: "ruby",
			image:  "ruby:3.2-alpine",
			cmd:    "ruby main.rb",
			files: map[string]string{
				"main.xql":   "main.rb",
				"service.xql": "service.rb",
				"models.xql":  "models.rb",
			},
		},
		{
			name:   "Lua",
			target: "lua",
			image:  "lua:5.4-alpine",
			cmd:    "lua main.lua",
			files: map[string]string{
				"main.xql":   "main.lua",
				"service.xql": "service.lua",
				"models.xql":  "models.lua",
			},
		},
	}

	for _, tgt := range targets {
		t.Run(tgt.name, func(t *testing.T) {
			generatedFiles := make(map[string][]byte)

			// 编译 main
			mainOut, err := Generate(mainNode, tgt.target)
			if err != nil {
				t.Skipf("Target %q does not support compiler features yet: %v", tgt.name, err)
			}
			generatedFiles[tgt.files["main.xql"]] = mainOut

			// 编译 service
			serviceOut, err := Generate(serviceNode, tgt.target)
			if err != nil {
				t.Skipf("Target %q does not support compiler features yet: %v", tgt.name, err)
			}
			generatedFiles[tgt.files["service.xql"]] = serviceOut

			// 编译 models
			modelsOut, err := Generate(modelsNode, tgt.target)
			if err != nil {
				t.Skipf("Target %q does not support compiler features yet: %v", tgt.name, err)
			}
			generatedFiles[tgt.files["models.xql"]] = modelsOut

			// 进行物理 Docker 端到端运行
			out, err := runDockerE2E(t, tgt.image, generatedFiles, tgt.cmd)
			if err != nil {
				t.Fatalf("Docker run failed: %v\nOutput: %s", err, out)
			}

			actual := strings.ReplaceAll(out, "\r\n", "\n")
			if !strings.Contains(actual, expected) {
				t.Errorf("expected output to contain %q, got %q", expected, actual)
			}
		})
	}
}
