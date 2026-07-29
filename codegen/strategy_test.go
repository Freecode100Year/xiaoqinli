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

func TestCodegenStrategyBranchComprehensionVsLoop(t *testing.T) {
	// Construct an AST representing: for x in items: res.append(x)
	astForLoop := &ast.Program{
		Decls: []ast.Node{
			&ast.FunctionDecl{
				Name:       "process",
				Params:     []ast.Param{},
				ReturnType: ast.TypeExpr{KindName: "Void"},
				Body: []ast.Node{
					&ast.ForStmt{
						Form:     "each",
						Var:      "x",
						Iterable: &ast.Ident{Name: "items"},
						Body: []ast.Node{
							&ast.ExprStmt{
								Expr: &ast.CallExpr{
									Callee: "res.append",
									Args:   []ast.Node{&ast.Ident{Name: "x"}},
								},
							},
						},
					},
				},
			},
		},
	}

	// 1. Enable PreferComprehension = true
	UpdateCodegenStrategy(CodegenStrategyConfig{
		Target:              "py",
		PreferComprehension: true,
	})

	codeTrueBytes, err := GeneratePython(astForLoop)
	if err != nil {
		t.Fatalf("GeneratePython error: %v", err)
	}
	codeTrue := string(codeTrueBytes)

	if !strings.Contains(codeTrue, "res.extend([x for x in items])") {
		t.Errorf("expected list comprehension in Python output when PreferComprehension=true, got:\n%s", codeTrue)
	}

	// 2. Disable PreferComprehension = false
	UpdateCodegenStrategy(CodegenStrategyConfig{
		Target:              "py",
		PreferComprehension: false,
	})

	codeFalseBytes, err := GeneratePython(astForLoop)
	if err != nil {
		t.Fatalf("GeneratePython error: %v", err)
	}
	codeFalse := string(codeFalseBytes)

	if !strings.Contains(codeFalse, "for x in items:") || !strings.Contains(codeFalse, "res.append(x)") {
		t.Errorf("expected standard for loop in Python output when PreferComprehension=false, got:\n%s", codeFalse)
	}
}
