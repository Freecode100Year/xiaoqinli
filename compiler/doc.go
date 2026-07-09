// Package compiler exports xiaoqinli's compile pipeline as a reusable library.
//
// xiaoqinli is an AST-First transpiler supporting 42+ target languages.
// This package wraps the internal ast → check → codegen pipeline into
// clean public functions that external projects (e.g. AgentCLI) can import.
//
// Quick start:
//
//	result := compiler.CompileFromFile("app.xql.json", "go", "")
//	if !result.Success {
//	    log.Fatal(result.Error)
//	}
//	fmt.Println(string(result.Code))
package compiler
