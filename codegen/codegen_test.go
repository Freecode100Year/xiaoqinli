package codegen

import (
	"strings"
	"testing"

	"xiaoqinli/ast"
)

func mustParse(t *testing.T, src string) ast.Node {
	t.Helper()
	node, err := ast.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return node
}

// addFibMain is a full program with add, fibonacci, main
const addFibMain = `{
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
			"body": [{
				"kind": "ReturnStmt",
				"value": {"kind": "BinaryExpr", "op": "+",
					"left": {"kind": "Ident", "name": "a"},
					"right": {"kind": "Ident", "name": "b"}}
			}]
		},
		{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": ["state"],
			"grant": ["io"],
			"body": [
				{
					"kind": "VarDecl",
					"name": "result",
					"type": {"kind": "Int"},
					"value": {"kind": "CallExpr", "callee": "add", "args": [
						{"kind": "Literal", "valueType": "Int", "value": 3},
						{"kind": "Literal", "valueType": "Int", "value": 5}
					]}
				},
				{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "println", "args": [
						{"kind": "Ident", "name": "result"}
					]}
				}
			]
		}
	]
}`

// whileProgram tests while loop + assignment (mutability)
const whileProgram = `{
	"kind": "Program",
	"declarations": [{
		"kind": "FunctionDecl",
		"name": "count",
		"params": [],
		"returnType": {"kind": "Int"},
		"effects": ["pure"],
		"grant": [],
		"body": [
			{
				"kind": "VarDecl",
				"name": "i",
				"type": {"kind": "Int"},
				"value": {"kind": "Literal", "valueType": "Int", "value": 0}
			},
			{
				"kind": "WhileStmt",
				"condition": {"kind": "BinaryExpr", "op": "<",
					"left": {"kind": "Ident", "name": "i"},
					"right": {"kind": "Literal", "valueType": "Int", "value": 10}},
				"body": [{
					"kind": "AssignStmt",
					"target": "i",
					"value": {"kind": "BinaryExpr", "op": "+",
						"left": {"kind": "Ident", "name": "i"},
						"right": {"kind": "Literal", "valueType": "Int", "value": 1}}
				}]
			},
			{
				"kind": "ReturnStmt",
				"value": {"kind": "Ident", "name": "i"}
			}
		]
	}]
}`

// boolProgram tests boolean literals and logical operators
const boolProgram = `{
	"kind": "Program",
	"declarations": [{
		"kind": "FunctionDecl",
		"name": "check",
		"params": [{"name": "x", "type": {"kind": "Bool"}}],
		"returnType": {"kind": "Bool"},
		"effects": ["pure"],
		"grant": [],
		"body": [{
			"kind": "ReturnStmt",
			"value": {"kind": "BinaryExpr", "op": "&&",
				"left": {"kind": "Ident", "name": "x"},
				"right": {"kind": "Literal", "valueType": "Bool", "value": true}}
		}]
	}]
}`

// stringConcatProgram tests string concatenation (Rust needs & on rhs)
const stringConcatProgram = `{
	"kind": "Program",
	"declarations": [{
		"kind": "FunctionDecl",
		"name": "greet",
		"params": [{"name": "name", "type": {"kind": "String"}}],
		"returnType": {"kind": "String"},
		"effects": ["pure"],
		"grant": [],
		"body": [{
			"kind": "ReturnStmt",
			"value": {"kind": "BinaryExpr", "op": "+",
				"left": {"kind": "Literal", "valueType": "String", "value": "Hello, "},
				"right": {"kind": "Ident", "name": "name"}}
		}]
	}]
}`

// --- Go codegen ---

func TestGenerateGo(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateGo(root)
	if err != nil {
		t.Fatalf("GenerateGo: %v", err)
	}
	code := string(out)

	checks := []string{
		"package main",
		"func add(a int, b int) int",
		"func main()",
		"fmt.Println(result)",
		"return a + b",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Go output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateGoWhile(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateGo(root)
	if err != nil {
		t.Fatalf("GenerateGo: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "for i < 10") {
		t.Errorf("Go while should use 'for', got:\n%s", code)
	}
}

// --- Rust codegen ---

func TestGenerateRust(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateRust(root)
	if err != nil {
		t.Fatalf("GenerateRust: %v", err)
	}
	code := string(out)

	checks := []string{
		"fn add(a: i64, b: i64) -> i64",
		"fn main()",
		"println!(",
		"let result: i64",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Rust output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateRustMutability(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateRust(root)
	if err != nil {
		t.Fatalf("GenerateRust: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "let mut i") {
		t.Errorf("Rust should use 'let mut' for reassigned var, got:\n%s", code)
	}
}

func TestGenerateRustStringConcat(t *testing.T) {
	root := mustParse(t, stringConcatProgram)
	out, err := GenerateRust(root)
	if err != nil {
		t.Fatalf("GenerateRust: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "+ &") {
		t.Errorf("Rust string concat should use '+ &' for rhs, got:\n%s", code)
	}
}

// --- TypeScript codegen ---

func TestGenerateTypeScript(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateTypeScript(root)
	if err != nil {
		t.Fatalf("GenerateTypeScript: %v", err)
	}
	code := string(out)

	checks := []string{
		"function add(a: number, b: number): number",
		"function main(): void",
		"console.log(result)",
		"main();",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("TS output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateTSMutability(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateTypeScript(root)
	if err != nil {
		t.Fatalf("GenerateTypeScript: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "let i: number") {
		t.Errorf("TS should use 'let' for reassigned var, got:\n%s", code)
	}
}

// --- Kotlin codegen ---

func TestGenerateKotlin(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateKotlin(root)
	if err != nil {
		t.Fatalf("GenerateKotlin: %v", err)
	}
	code := string(out)

	checks := []string{
		"fun add(a: Long, b: Long): Long",
		"fun main()",
		"println(result)",
		"3L",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Kotlin output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateKotlinMutability(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateKotlin(root)
	if err != nil {
		t.Fatalf("GenerateKotlin: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "var i: Long") {
		t.Errorf("Kotlin should use 'var' for reassigned var, got:\n%s", code)
	}
}

// --- Swift codegen ---

func TestGenerateSwift(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateSwift(root)
	if err != nil {
		t.Fatalf("GenerateSwift: %v", err)
	}
	code := string(out)

	checks := []string{
		"func add(_ a: Int, _ b: Int) -> Int",
		"print(result)",
		// main body at top level, not wrapped in func
		"let result: Int",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Swift output missing %q\n---\n%s", c, code)
		}
	}
	// Swift main should NOT have "func main"
	if strings.Contains(code, "func main") {
		t.Errorf("Swift should emit main body at top level, not func main:\n%s", code)
	}
}

func TestGenerateSwiftMutability(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateSwift(root)
	if err != nil {
		t.Fatalf("GenerateSwift: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "var i: Int") {
		t.Errorf("Swift should use 'var' for reassigned var, got:\n%s", code)
	}
}

// --- Python codegen ---

func TestGeneratePython(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GeneratePython(root)
	if err != nil {
		t.Fatalf("GeneratePython: %v", err)
	}
	code := string(out)

	checks := []string{
		"def add(a: int, b: int) -> int:",
		"def main() -> None:",
		"print(result)",
		`if __name__ == "__main__":`,
		"    main()",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Python output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGeneratePythonBoolOps(t *testing.T) {
	root := mustParse(t, boolProgram)
	out, err := GeneratePython(root)
	if err != nil {
		t.Fatalf("GeneratePython: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, " and ") {
		t.Errorf("Python should map '&&' to 'and', got:\n%s", code)
	}
	if !strings.Contains(code, "True") {
		t.Errorf("Python should emit 'True' for bool, got:\n%s", code)
	}
}

// --- Java codegen ---

func TestGenerateJava(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateJava(root)
	if err != nil {
		t.Fatalf("GenerateJava: %v", err)
	}
	code := string(out)

	checks := []string{
		"public class Main {",
		"static long add(long a, long b)",
		"public static void main(String[] args)",
		"System.out.println(result)",
		"return (a + b);",
		"3L",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Java output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateJavaMutability(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateJava(root)
	if err != nil {
		t.Fatalf("GenerateJava: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "long i") {
		t.Errorf("Java should declare mutable var without final, got:\n%s", code)
	}
	if strings.Contains(code, "final long i") {
		t.Errorf("Java should NOT use 'final' for reassigned var, got:\n%s", code)
	}
	if !strings.Contains(code, "while (") {
		t.Errorf("Java should emit 'while' loop, got:\n%s", code)
	}
}

func TestGenerateJavaStringConcat(t *testing.T) {
	root := mustParse(t, stringConcatProgram)
	out, err := GenerateJava(root)
	if err != nil {
		t.Fatalf("GenerateJava: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "static String greet(String name)") {
		t.Errorf("Java should map String type correctly, got:\n%s", code)
	}
	if !strings.Contains(code, `"Hello, " + name`) {
		t.Errorf("Java string concat should use +, got:\n%s", code)
	}
}

// --- C# codegen ---

func TestGenerateCSharp(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateCSharp(root)
	if err != nil {
		t.Fatalf("GenerateCSharp: %v", err)
	}
	code := string(out)

	checks := []string{
		"using System;",
		"class Program {",
		"static long add(long a, long b)",
		"static void Main()",
		"Console.WriteLine(result)",
		"return (a + b);",
		"3L",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("C# output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateCSharpMutability(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateCSharp(root)
	if err != nil {
		t.Fatalf("GenerateCSharp: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "long i") {
		t.Errorf("C# should declare var with type, got:\n%s", code)
	}
	if !strings.Contains(code, "while (") {
		t.Errorf("C# should emit 'while' loop, got:\n%s", code)
	}
}

func TestGenerateCSharpStringConcat(t *testing.T) {
	root := mustParse(t, stringConcatProgram)
	out, err := GenerateCSharp(root)
	if err != nil {
		t.Fatalf("GenerateCSharp: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "static string greet(string name)") {
		t.Errorf("C# should map string type (lowercase), got:\n%s", code)
	}
	if !strings.Contains(code, `"Hello, " + name`) {
		t.Errorf("C# string concat should use +, got:\n%s", code)
	}
}

// --- collectMutables tests ---

func TestCollectMutables(t *testing.T) {
	stmts := []ast.Node{
		&ast.VarDecl{Name: "x"},
		&ast.AssignStmt{Target: "x", Value: &ast.Literal{ValueType: "Int", Value: float64(1)}},
		&ast.VarDecl{Name: "y"},
		&ast.IfStmt{
			Cond: &ast.Literal{ValueType: "Bool", Value: true},
			Then: []ast.Node{
				&ast.AssignStmt{Target: "y", Value: &ast.Literal{ValueType: "Int", Value: float64(2)}},
			},
		},
		&ast.VarDecl{Name: "z"},
	}

	muts := collectMutables(stmts)
	if !muts["x"] {
		t.Error("x should be mutable")
	}
	if !muts["y"] {
		t.Error("y should be mutable (assigned in if)")
	}
	if muts["z"] {
		t.Error("z should not be mutable")
	}
}
