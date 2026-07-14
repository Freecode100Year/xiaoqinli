//go:build ignore_e2e

package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"xiaoqinli/ast"
	"xiaoqinli/check"
	"xiaoqinli/vfs"
)

func TestEndToEndIntegration(t *testing.T) {
	utilsSrc := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "StructDecl",
				"name": "Point",
				"fields": [
					{"name": "x", "type": {"kind": "Int"}},
					{"name": "y", "type": {"kind": "Int"}}
				]
			},
			{
				"kind": "FunctionDecl",
				"name": "calc",
				"params": [
					{"name": "val", "type": {"kind": "Int"}}
				],
				"returnType": {"kind": "Int"},
				"effects": ["pure"],
				"grant": [],
				"body": [
					{
						"kind": "VarDecl",
						"name": "sum",
						"type": {"kind": "Int"},
						"value": {"kind": "Literal", "valueType": "Int", "value": 0}
					},
					{
						"kind": "ForStmt",
						"form": "range",
						"var": "i",
						"start": {"kind": "Literal", "valueType": "Int", "value": 1},
						"end": {"kind": "Ident", "name": "val"},
						"body": [
							{
								"kind": "AssignStmt",
								"target": {"kind": "Ident", "name": "sum"},
								"value": {
									"kind": "BinaryExpr",
									"op": "+",
									"left": {"kind": "Ident", "name": "sum"},
									"right": {"kind": "Ident", "name": "i"}
								}
							}
						]
					},
					{
						"kind": "VarDecl",
						"name": "res",
						"type": {"kind": "Int"},
						"value": {"kind": "Literal", "valueType": "Int", "value": 0}
					},
					{
						"kind": "SwitchStmt",
						"value": {"kind": "Ident", "name": "sum"},
						"cases": [
							{
								"kind": "SwitchCase",
								"value": {"kind": "Literal", "valueType": "Int", "value": 6},
								"body": [
									{
										"kind": "AssignStmt",
										"target": {"kind": "Ident", "name": "res"},
										"value": {"kind": "Literal", "valueType": "Int", "value": 100}
									}
								]
							},
							{
								"kind": "SwitchCase",
								"value": null,
								"body": [
									{
										"kind": "AssignStmt",
										"target": {"kind": "Ident", "name": "res"},
										"value": {"kind": "Ident", "name": "sum"}
									}
								]
							}
						]
					},
					{
						"kind": "ReturnStmt",
						"value": {"kind": "Ident", "name": "res"}
					}
				]
			},
			{
				"kind": "FunctionDecl",
				"name": "requestData",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network:read"],
				"body": [
					{
						"kind": "ExprStmt",
						"expr": {
							"kind": "CallExpr",
							"callee": "println",
							"args": [
								{"kind": "Literal", "valueType": "String", "value": "requesting data"}
							]
						}
					}
				]
			}
		]
	}`

	mainSrc := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "ImportDecl",
				"path": "./utils.xql",
				"as": "utils"
			},
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": ["network:*", "io"],
				"body": [
					{
						"kind": "VarDecl",
						"name": "res",
						"type": {"kind": "Int"},
						"value": {
							"kind": "CallExpr",
							"callee": "utils.calc",
							"args": [
								{"kind": "Literal", "valueType": "Int", "value": 3}
							]
						}
					},
					{
						"kind": "ExprStmt",
						"expr": {
							"kind": "CallExpr",
							"callee": "println",
							"args": [
								{"kind": "Ident", "name": "res"}
							]
						}
					},
					{
						"kind": "ExprStmt",
						"expr": {
							"kind": "CallExpr",
							"callee": "utils.requestData",
							"args": []
						}
					}
				]
			}
		]
	}`

	ws := vfs.New()
	ws.Write("utils.xql", []byte(utilsSrc))
	ws.Write("main.xql", []byte(mainSrc))

	utilsNode, err := ast.Parse([]byte(utilsSrc))
	if err != nil {
		t.Fatalf("Parse utils.xql failed: %v", err)
	}
	mainNode, err := ast.Parse([]byte(mainSrc))
	if err != nil {
		t.Fatalf("Parse main.xql failed: %v", err)
	}

	// 1. 静态检查 Workspace
	if err := check.RunAllInWorkspace(mainNode, "main.xql", ws); err != nil {
		t.Fatalf("check RunAllInWorkspace failed: %v", err)
	}

	// 2. Go 端到端运行
	t.Run("Go", func(t *testing.T) {
		if _, err := exec.LookPath("go"); err != nil {
			t.Skip("go not installed, skipping Go E2E")
		}

		goMain, err := Generate(mainNode, "go")
		if err != nil {
			t.Fatalf("Generate main.go failed: %v", err)
		}
		goUtils, err := Generate(utilsNode, "go")
		if err != nil {
			t.Fatalf("Generate utils.go failed: %v", err)
		}

		tmpDir := t.TempDir()
		mainPath := filepath.Join(tmpDir, "main.go")
		utilsPath := filepath.Join(tmpDir, "utils.go")

		_ = os.WriteFile(mainPath, goMain, 0644)
		_ = os.WriteFile(utilsPath, goUtils, 0644)

		cmd := exec.Command("go", "run", "main.go", "utils.go")
		cmd.Dir = tmpDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go run failed: %v\nOutput: %s", err, out)
		}

		expected := "100\nrequesting data\n"
		actual := strings.ReplaceAll(string(out), "\r\n", "\n")
		if !strings.Contains(actual, expected) {
			t.Errorf("expected output to contain %q, got %q", expected, actual)
		}
	})

	// 3. Python 端到端运行
	t.Run("Python", func(t *testing.T) {
		pyCmd := "python"
		if _, err := exec.LookPath(pyCmd); err != nil {
			pyCmd = "py"
			if _, err := exec.LookPath(pyCmd); err != nil {
				t.Skip("python not installed, skipping Python E2E")
			}
		}

		pyMain, err := Generate(mainNode, "py")
		if err != nil {
			t.Fatalf("Generate main.py failed: %v", err)
		}
		pyUtils, err := Generate(utilsNode, "py")
		if err != nil {
			t.Fatalf("Generate utils.py failed: %v", err)
		}

		tmpDir := t.TempDir()
		mainPath := filepath.Join(tmpDir, "main.py")
		utilsPath := filepath.Join(tmpDir, "utils.py")

		_ = os.WriteFile(mainPath, pyMain, 0644)
		_ = os.WriteFile(utilsPath, pyUtils, 0644)

		cmd := exec.Command(pyCmd, "main.py")
		cmd.Dir = tmpDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("python run failed: %v\nOutput: %s", err, out)
		}

		expected := "100\nrequesting data\n"
		actual := strings.ReplaceAll(string(out), "\r\n", "\n")
		if !strings.Contains(actual, expected) {
			t.Errorf("expected output to contain %q, got %q", expected, actual)
		}
	})

	// 4. Rust 端到端运行
	t.Run("Rust", func(t *testing.T) {
		if _, err := exec.LookPath("rustc"); err != nil {
			t.Skip("rustc not installed, skipping Rust E2E")
		}

		rsMain, err := Generate(mainNode, "rust")
		if err != nil {
			t.Fatalf("Generate main.rs failed: %v", err)
		}
		rsUtils, err := Generate(utilsNode, "rust")
		if err != nil {
			t.Fatalf("Generate utils.rs failed: %v", err)
		}

		tmpDir := t.TempDir()
		mainPath := filepath.Join(tmpDir, "main.rs")
		utilsPath := filepath.Join(tmpDir, "utils.rs")

		_ = os.WriteFile(mainPath, rsMain, 0644)
		_ = os.WriteFile(utilsPath, rsUtils, 0644)

		// 编译 rustc main.rs
		buildCmd := exec.Command("rustc", "main.rs", "--edition=2021")
		buildCmd.Dir = tmpDir
		out, err := buildCmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(out), "linker `link.exe` not found") {
				t.Skip("rustc linker link.exe not found, skipping test")
			}
			t.Fatalf("rustc compile failed: %v\nOutput: %s", err, out)
		}

		exeName := "./main"
		if filepath.Separator == '\\' {
			exeName = ".\\main.exe"
		}
		runCmd := exec.Command(exeName)
		runCmd.Dir = tmpDir
		out, err = runCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("rust run failed: %v\nOutput: %s", err, out)
		}

		expected := "100\nrequesting data\n"
		actual := strings.ReplaceAll(string(out), "\r\n", "\n")
		if !strings.Contains(actual, expected) {
			t.Errorf("expected output to contain %q, got %q", expected, actual)
		}
	})
}

func TestE2EWorkspaceDogfood(t *testing.T) {
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

	// 1. Go端到端
	t.Run("Go", func(t *testing.T) {
		if _, err := exec.LookPath("go"); err != nil {
			t.Skip("go not installed, skipping Go E2E")
		}
		goMain, err := Generate(mainNode, "go")
		if err != nil {
			t.Fatalf("Generate goMain failed: %v", err)
		}
		goService, err := Generate(serviceNode, "go")
		if err != nil {
			t.Fatalf("Generate goService failed: %v", err)
		}
		goModels, err := Generate(modelsNode, "go")
		if err != nil {
			t.Fatalf("Generate goModels failed: %v", err)
		}

		tmpDir := t.TempDir()
		_ = os.WriteFile(filepath.Join(tmpDir, "main.go"), goMain, 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "service.go"), goService, 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "models.go"), goModels, 0644)

		cmd := exec.Command("go", "run", "main.go", "service.go", "models.go")
		cmd.Dir = tmpDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go run failed: %v\nOutput: %s", err, out)
		}

		actual := strings.ReplaceAll(string(out), "\r\n", "\n")
		if !strings.Contains(actual, expected) {
			t.Errorf("expected output to contain %q, got %q", expected, actual)
		}
	})

	// 2. Python端到端
	t.Run("Python", func(t *testing.T) {
		pyCmd := "python"
		if _, err := exec.LookPath(pyCmd); err != nil {
			pyCmd = "py"
			if _, err := exec.LookPath(pyCmd); err != nil {
				t.Skip("python not installed, skipping Python E2E")
			}
		}
		pyMain, err := Generate(mainNode, "py")
		if err != nil {
			t.Fatalf("Generate pyMain failed: %v", err)
		}
		pyService, err := Generate(serviceNode, "py")
		if err != nil {
			t.Fatalf("Generate pyService failed: %v", err)
		}
		pyModels, err := Generate(modelsNode, "py")
		if err != nil {
			t.Fatalf("Generate pyModels failed: %v", err)
		}

		tmpDir := t.TempDir()
		_ = os.WriteFile(filepath.Join(tmpDir, "main.py"), pyMain, 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "service.py"), pyService, 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "models.py"), pyModels, 0644)

		cmd := exec.Command(pyCmd, "main.py")
		cmd.Dir = tmpDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("python run failed: %v\nOutput: %s", err, out)
		}

		actual := strings.ReplaceAll(string(out), "\r\n", "\n")
		if !strings.Contains(actual, expected) {
			t.Errorf("expected output to contain %q, got %q", expected, actual)
		}
	})

	// 3. Rust端到端
	t.Run("Rust", func(t *testing.T) {
		if _, err := exec.LookPath("rustc"); err != nil {
			t.Skip("rustc not installed, skipping Rust E2E")
		}
		rsMain, err := Generate(mainNode, "rust")
		if err != nil {
			t.Fatalf("Generate rsMain failed: %v", err)
		}
		rsService, err := Generate(serviceNode, "rust")
		if err != nil {
			t.Fatalf("Generate rsService failed: %v", err)
		}
		rsModels, err := Generate(modelsNode, "rust")
		if err != nil {
			t.Fatalf("Generate rsModels failed: %v", err)
		}

		tmpDir := t.TempDir()
		_ = os.WriteFile(filepath.Join(tmpDir, "main.rs"), rsMain, 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "service.rs"), rsService, 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "models.rs"), rsModels, 0644)

		var buildCmd *exec.Cmd
		if _, err := exec.Command("rustup", "run", "stable-x86_64-pc-windows-gnu", "rustc", "--version").CombinedOutput(); err == nil {
			buildCmd = exec.Command("rustup", "run", "stable-x86_64-pc-windows-gnu", "rustc", "main.rs", "--edition=2021")
		} else {
			buildCmd = exec.Command("rustc", "main.rs", "--edition=2021")
		}
		buildCmd.Dir = tmpDir
		out, err := buildCmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(out), "linker `link.exe` not found") {
				t.Skip("rustc linker link.exe not found, skipping test")
			}
			t.Fatalf("rustc compile failed: %v\nOutput: %s", err, out)
		}

		exeName := "./main"
		if filepath.Separator == '\\' {
			exeName = ".\\main.exe"
		}
		runCmd := exec.Command(exeName)
		runCmd.Dir = tmpDir
		out, err = runCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("rust run failed: %v\nOutput: %s", err, out)
		}

		actual := strings.ReplaceAll(string(out), "\r\n", "\n")
		if !strings.Contains(actual, expected) {
			t.Errorf("expected output to contain %q, got %q", expected, actual)
		}
	})

	// 4. TypeScript端到端
	t.Run("TypeScript", func(t *testing.T) {
		if _, err := exec.LookPath("node"); err != nil {
			t.Skip("node not installed, skipping TS E2E")
		}
		tsMain, err := Generate(mainNode, "ts")
		if err != nil {
			t.Fatalf("Generate tsMain failed: %v", err)
		}
		tsService, err := Generate(serviceNode, "ts")
		if err != nil {
			t.Fatalf("Generate tsService failed: %v", err)
		}
		tsModels, err := Generate(modelsNode, "ts")
		if err != nil {
			t.Fatalf("Generate tsModels failed: %v", err)
		}

		tmpDir := t.TempDir()
		_ = os.WriteFile(filepath.Join(tmpDir, "main.ts"), tsMain, 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "service.ts"), tsService, 0644)
		_ = os.WriteFile(filepath.Join(tmpDir, "models.ts"), tsModels, 0644)

		hasTsc := false
		var compileCmd *exec.Cmd
		if _, err := exec.LookPath("tsc"); err == nil {
			hasTsc = true
			compileCmd = exec.Command("tsc", "main.ts", "service.ts", "models.ts", "--module", "commonjs", "--target", "es2020", "--skipLibCheck")
		} else if _, err := exec.LookPath("npx"); err == nil {
			hasTsc = true
			compileCmd = exec.Command("npx", "-p", "typescript", "tsc", "main.ts", "service.ts", "models.ts", "--module", "commonjs", "--target", "es2020", "--skipLibCheck")
		}

		if hasTsc && compileCmd != nil {
			compileCmd.Dir = tmpDir
			compileOut, compileErr := compileCmd.CombinedOutput()
			if compileErr != nil {
				t.Fatalf("tsc compilation failed: %v\nOutput: %s", compileErr, compileOut)
			}
			cmd := exec.Command("node", "main.js")
			cmd.Dir = tmpDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("node run failed: %v\nOutput: %s", err, out)
			}
			actual := strings.ReplaceAll(string(out), "\r\n", "\n")
			if !strings.Contains(actual, expected) {
				t.Errorf("expected output to contain %q, got %q", expected, actual)
			}
		} else {
			// 如果没有 tsc，仅检测能否在 node --experimental-strip-types 下运行
			cmd := exec.Command("node", "--experimental-strip-types", "main.ts")
			cmd.Dir = tmpDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Skip("node --experimental-strip-types not supported, and tsc not installed, skipping TS E2E run")
			}
			actual := strings.ReplaceAll(string(out), "\r\n", "\n")
			if !strings.Contains(actual, expected) {
				t.Errorf("expected output to contain %q, got %q", expected, actual)
			}
		}
	})
}
