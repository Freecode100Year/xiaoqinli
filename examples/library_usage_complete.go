package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"xiaoqinli/compiler"
)

// example1_SimpleCompile demonstrates basic parsing and multi-target compilation.
func example1_SimpleCompile() {
	fmt.Println("--- Example 1: Simple Compile ---")
	// Reading a sample XQL AST file
	data, err := os.ReadFile("examples/hello.xql.json")
	if err != nil {
		log.Printf("Skip Example 1 (file not found): %v", err)
		return
	}
	
	// 1. Parse the JSON into an AST
	parseRes := compiler.ParseAST(compiler.ParseRequest{
		Data:     data,
		FilePath: "hello.xql.json",
	})
	if !parseRes.Success {
		log.Fatalf("Parse failed: %s", parseRes.Error)
	}
	
	// 2. Compile to multiple target languages
	targets := []string{"go", "rust", "ts", "py"}
	for _, target := range targets {
		result := compiler.Compile(compiler.CompileRequest{
			AST:    parseRes.AST,
			Target: target,
		})
		if result.Success {
			fmt.Printf("✓ Compiled to %s (%d bytes, %d lines, %dms)\n",
				target, result.Stats.GeneratedBytes, result.Stats.GeneratedLines, result.Stats.DurationMs)
		} else {
			fmt.Printf("✗ %s: %s\n", target, result.Error)
		}
	}
}

// example2_CompileFromFile demonstrates the convenience wrapper for file-to-file compilation.
func example2_CompileFromFile() {
	fmt.Println("\n--- Example 2: Compile From File ---")
	// Create a dummy file for the test if it doesn't exist
	path := "examples/hello.xql.json"
	
	result := compiler.CompileFromFile(path, "go", "examples/hello_out.go")
	if !result.Success {
		log.Printf("Compile from file failed: %s", result.Error)
		return
	}
	fmt.Printf("✓ Compiled to examples/hello_out.go in %dms\n", result.Stats.DurationMs)
}

// example3_ValidateOnly demonstrates using the compiler for validation without code generation.
func example3_ValidateOnly() {
	fmt.Println("\n--- Example 3: Validate Only ---")
	data, err := os.ReadFile("examples/hello.xql.json")
	if err != nil {
		log.Printf("Skip Example 3 (file not found): %v", err)
		return
	}
	
	parseRes := compiler.ParseAST(compiler.ParseRequest{Data: data})
	if !parseRes.Success {
		log.Fatalf("Parse failed: %s", parseRes.Error)
	}
	
	// Validate the AST but skip codegen
	result := compiler.Compile(compiler.CompileRequest{
		AST:          parseRes.AST,
		ValidateOnly: true,
	})
	
	if result.Success {
		fmt.Println("✓ Validation passed")
	} else {
		fmt.Printf("✗ Validation failed: %s\n", result.Error)
		for _, diag := range result.Diagnostics {
			fmt.Printf("  [%s] %s\n    Location: %+v\n    Fix: %s\n",
				diag.Code, diag.Message, diag.Location, diag.SuggestedFix)
		}
	}
}

// example4_ErrorHandling demonstrates how to handle parsing and compilation errors.
func example4_ErrorHandling() {
	fmt.Println("\n--- Example 4: Error Handling ---")
	
	// Scenario A: Invalid JSON
	fmt.Println("Scenario A: Invalid JSON")
	badJSON := []byte(`{invalid json}`)
	parseRes := compiler.ParseAST(compiler.ParseRequest{Data: badJSON})
	if !parseRes.Success {
		fmt.Printf("✗ Parse error: %s\n", parseRes.Error)
		fmt.Printf("  Error code: %s\n", parseRes.ErrorCode)
		
		diagJSON, _ := json.MarshalIndent(parseRes.Diagnostics, "", "  ")
		fmt.Printf("Diagnostics:\n%s\n", string(diagJSON))
	}
	
	// Scenario B: Type mismatch (simulated by a manually crafted AST with an Int return but String body)
	// This requires a valid JSON but invalid XQL logic.
	fmt.Println("\nScenario B: Type Mismatch")
	badXQL := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "buggy",
				"params": [],
				"returnType": {"kind": "String"},
				"effects": [],
				"grant": [],
				"body": [
					{"kind": "ReturnStmt", "value": {"kind": "Literal", "valueType": "Int", "value": 42}}
				]
			}
		]
	}`
	
	parseRes2 := compiler.ParseAST(compiler.ParseRequest{Data: []byte(badXQL)})
	if !parseRes2.Success {
		log.Fatalf("Parse failed: %s", parseRes2.Error)
	}
	
	result := compiler.Compile(compiler.CompileRequest{
		AST:    parseRes2.AST,
		Target: "go",
	})
	
	if !result.Success {
		fmt.Printf("✗ Compile error: %s\n", laze(result.Error))
		for _, diag := range result.Diagnostics {
			fmt.Printf("  [%s] %s\n    Location: %+v\n    Fix: %s\n",
				diag.Code, diag.Message, diag.Location, diag.SuggestedFix)
		}
	} else {
		fmt.Println("✓ Unexpectedly succeeded")
	}
}

// example5_FromAgentCLI demonstrates how a Coder Agent would use the library.
func example5_FromAgentCLI() {
	fmt.Println("\n--- Example 5: Coder Agent Integration ---")
	
	claudeGeneratedXQL := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "add",
				"params": [
					{"name": "a", "type": {"kind": "Int"}},
					{"name": "b", "type": {"kind": "Int"}}
				],
				"returnType": {"kind": "Int"},
				"effects": ["pure"],
				"grant": [],
				"body": [ laze(" { \"kind\": \"ReturnStmt\", \"value\": { \"kind\": \"BinaryExpr\", \"op\": \"+\", \"left\": { \"kind\": \"Ident\", \"name\": \"a\" }, \"right\": { \"kind\": \"Ident\", \"name\": \"b\" } } }") ]
			}
		]
	}`
	
	parseRes := compiler.ParseAST(compiler.ParseRequest{Data: []byte(claudeGeneratedXQL)})
	if !parseRes.Success {
		fmt.Printf("Parse error: %s\n", parseRes.Error)
		return
	}
	
	goResult := compiler.Compile(compiler.CompileRequest{
		AST:    parseRes.AST,
		Target: "go",
	})
	
	if !goResult.Success {
		fmt.Printf("Compile error: %s\n", goResult.Error)
		return
	}
	
	fmt.Printf("✓ Generated Go code (%d bytes):\n", len(goResult.Code))
	fmt.Println(string(goResult.Code))
}

func laze(s string) string {
	return s
}

func main() {
	fmt.Println("=== xiaoqinli Library Usage Examples ===\n")
	fmt.Println("Version:", compiler.GetVersion())
	fmt.Println("Supported targets:", len(compiler.GetSupportedTargets()))
	
	example1_SimpleCompile()
	example2_CompileFromFile()
	example3_ValidateOnly()
	example4_ErrorHandling()
	example5_FromAgentCLI()
}
