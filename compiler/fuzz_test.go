package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzPipeline feeds arbitrary input through parse, check and every backend.
// A backend may refuse a program; it may not panic on one the checker passed.
func FuzzPipeline(f *testing.F) {
	files, _ := filepath.Glob("../examples/*.xql.json")
	for _, p := range files {
		b, _ := os.ReadFile(p)
		f.Add(b)
	}
	targets := GetSupportedTargets()
	f.Fuzz(func(t *testing.T, data []byte) {
		pr := ParseAST(ParseRequest{Data: data})
		if !pr.Success {
			return
		}
		Validate(ValidateRequest{AST: pr.AST})
		for _, tg := range targets {
			Compile(CompileRequest{AST: pr.AST, Target: tg})
		}
	})
}
