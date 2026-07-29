package codegen

import (
	"path/filepath"
	"strings"
	"testing"
	"xiaoqinli/ast"
)

func TestCodegenStrategyUpdateAndGenerate(t *testing.T) {
	// 1. Update strategy for Python with custom header comment flag
	strat := UpdateCodegenStrategy(CodegenStrategyConfig{
		Target:              "py",
		PreferComprehension: true,
		InlineThreshold:     45,
		OptimizationFlags:   map[string]string{"header_comment": "true", "strategy_tag": "BenchScore98.5"},
		BenchmarkScore:      98.5,
	})

	if strat.Target != "py" {
		t.Errorf("expected target py, got %s", strat.Target)
	}

	// 2. Generate Python code and verify strategy header tag is included
	root := &ast.Program{
		Decls: []ast.Node{
			&ast.FunctionDecl{
				Name:       "main",
				Params:     []ast.Param{},
				ReturnType: ast.TypeExpr{KindName: "Void"},
				Body:       []ast.Node{},
			},
		},
	}

	codeBytes, err := GeneratePython(root)
	if err != nil {
		t.Fatalf("GeneratePython error: %v", err)
	}

	code := string(codeBytes)
	if !strings.Contains(code, "# Codegen Strategy: BenchScore98.5") {
		t.Errorf("expected generated python code to contain strategy tag comment, got:\n%s", code)
	}

	// 3. Test persistence file save and load
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "codegen_strategies.json")
	if err := SaveStrategiesToFile(filePath); err != nil {
		t.Fatalf("SaveStrategiesToFile error: %v", err)
	}

	if err := LoadStrategiesFromFile(filePath); err != nil {
		t.Fatalf("LoadStrategiesFromFile error: %v", err)
	}

	reInspected := InspectCodegenStrategy("py")
	if reInspected.BenchmarkScore != 98.5 {
		t.Errorf("expected re-inspected BenchmarkScore 98.5, got %f", reInspected.BenchmarkScore)
	}
}
