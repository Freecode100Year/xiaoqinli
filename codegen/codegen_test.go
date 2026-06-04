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

// structProgram tests struct declaration, construction, and field access
const structProgram = `{
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
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": ["state"],
			"grant": ["io"],
			"body": [
				{
					"kind": "VarDecl",
					"name": "p",
					"type": {"kind": "Point"},
					"value": {
						"kind": "StructLit",
						"typeName": "Point",
						"fields": [
							{"name": "x", "value": {"kind": "Literal", "valueType": "Int", "value": 3}},
							{"name": "y", "value": {"kind": "Literal", "valueType": "Int", "value": 5}}
						]
					}
				},
				{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "println", "args": [
						{"kind": "MemberExpr", "object": {"kind": "Ident", "name": "p"}, "field": "x"}
					]}
				}
			]
		}
	]
}`

// collectionProgram tests array literal creation and index access
const collectionProgram = `{
	"kind": "Program",
	"declarations": [{
		"kind": "FunctionDecl",
		"name": "main",
		"params": [],
		"returnType": {"kind": "Void"},
		"effects": ["state"],
		"grant": ["io"],
		"body": [
			{
				"kind": "VarDecl",
				"name": "nums",
				"type": {"kind": "Array", "elem": {"kind": "Int"}},
				"value": {
					"kind": "ArrayLit",
					"elemType": {"kind": "Int"},
					"elements": [
						{"kind": "Literal", "valueType": "Int", "value": 10},
						{"kind": "Literal", "valueType": "Int", "value": 20},
						{"kind": "Literal", "valueType": "Int", "value": 30}
					]
				}
			},
			{
				"kind": "ExprStmt",
				"expr": {"kind": "CallExpr", "callee": "println", "args": [
					{"kind": "IndexExpr",
					 "target": {"kind": "Ident", "name": "nums"},
					 "index": {"kind": "Literal", "valueType": "Int", "value": 0}}
				]}
			}
		]
	}]
}`

// helloProgram tests println with a string-returning function call
const helloProgram = `{
	"kind": "Program",
	"declarations": [
		{
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
		},
		{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": [{
				"kind": "ExprStmt",
				"expr": {"kind": "CallExpr", "callee": "println", "args": [
					{"kind": "CallExpr", "callee": "greet", "args": [
						{"kind": "Literal", "valueType": "String", "value": "World"}
					]}
				]}
			}]
		}
	]
}`

// ifElseProgram tests if/else code generation
const ifElseProgram = `{
	"kind": "Program",
	"declarations": [{
		"kind": "FunctionDecl",
		"name": "abs",
		"params": [{"name": "x", "type": {"kind": "Int"}}],
		"returnType": {"kind": "Int"},
		"effects": ["pure"],
		"grant": [],
		"body": [{
			"kind": "IfStmt",
			"condition": {"kind": "BinaryExpr", "op": "<",
				"left": {"kind": "Ident", "name": "x"},
				"right": {"kind": "Literal", "valueType": "Int", "value": 0}},
			"then": [{
				"kind": "ReturnStmt",
				"value": {"kind": "UnaryExpr", "op": "-",
					"operand": {"kind": "Ident", "name": "x"}}
			}],
			"else": [{
				"kind": "ReturnStmt",
				"value": {"kind": "Ident", "name": "x"}
			}]
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
	// param String → &str, so concat uses "+" directly (no extra &)
	if !strings.Contains(code, "name: &str") {
		t.Errorf("Rust String param should be &str, got:\n%s", code)
	}
	if !strings.Contains(code, `"Hello, ".to_string() + name`) {
		t.Errorf("Rust string concat should use .to_string() on lhs + bare &str param, got:\n%s", code)
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

// --- Generate dispatcher ---

func TestGenerateDispatcher(t *testing.T) {
	root := mustParse(t, addFibMain)
	targets := []string{"go", "rust", "ts", "kotlin", "swift", "py", "java", "csharp", "dart", "lua", "ruby", "php", "zig", "nim", "julia"}
	for _, tgt := range targets {
		out, err := Generate(root, tgt)
		if err != nil {
			t.Errorf("Generate(%q) error: %v", tgt, err)
			continue
		}
		if len(out) == 0 {
			t.Errorf("Generate(%q) returned empty output", tgt)
		}
	}
	_, err := Generate(root, "brainfuck")
	if err == nil {
		t.Error("Generate should fail for unsupported target")
	}
}

// --- Dart codegen ---

func TestGenerateDart(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateDart(root)
	if err != nil {
		t.Fatalf("GenerateDart: %v", err)
	}
	code := string(out)

	checks := []string{
		"int add(int a, int b)",
		"void main()",
		"print(result)",
		"return (a + b);",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Dart output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateDartMutability(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateDart(root)
	if err != nil {
		t.Fatalf("GenerateDart: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "int i") {
		t.Errorf("Dart should declare mutable var without final, got:\n%s", code)
	}
	if strings.Contains(code, "final int i") {
		t.Errorf("Dart should NOT use 'final' for reassigned var, got:\n%s", code)
	}
}

func TestGenerateDartIfElse(t *testing.T) {
	root := mustParse(t, ifElseProgram)
	out, err := GenerateDart(root)
	if err != nil {
		t.Fatalf("GenerateDart: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "if (") || !strings.Contains(code, "} else {") {
		t.Errorf("Dart should emit 'if (...) { ... } else { ... }', got:\n%s", code)
	}
}

// --- Lua codegen ---

func TestGenerateLua(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateLua(root)
	if err != nil {
		t.Fatalf("GenerateLua: %v", err)
	}
	code := string(out)

	checks := []string{
		"function add(a, b)",
		"print(result)",
		"return (a + b)",
		"local result",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Lua output missing %q\n---\n%s", c, code)
		}
	}
	if strings.Contains(code, "function main") {
		t.Errorf("Lua should emit main body at top level, not function main:\n%s", code)
	}
}

func TestGenerateLuaStringConcat(t *testing.T) {
	root := mustParse(t, stringConcatProgram)
	out, err := GenerateLua(root)
	if err != nil {
		t.Fatalf("GenerateLua: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "..") {
		t.Errorf("Lua string concat should use '..', got:\n%s", code)
	}
}

func TestGenerateLuaIfElse(t *testing.T) {
	root := mustParse(t, ifElseProgram)
	out, err := GenerateLua(root)
	if err != nil {
		t.Fatalf("GenerateLua: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "if ") || !strings.Contains(code, " then") || !strings.Contains(code, "else") {
		t.Errorf("Lua should emit 'if...then...else...end', got:\n%s", code)
	}
}

func TestGenerateLuaBoolOps(t *testing.T) {
	root := mustParse(t, boolProgram)
	out, err := GenerateLua(root)
	if err != nil {
		t.Fatalf("GenerateLua: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, " and ") {
		t.Errorf("Lua should map '&&' to 'and', got:\n%s", code)
	}
}

func TestGenerateLuaWhile(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateLua(root)
	if err != nil {
		t.Fatalf("GenerateLua: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "while") || !strings.Contains(code, "do") {
		t.Errorf("Lua should use 'while...do', got:\n%s", code)
	}
}

// --- Ruby codegen ---

func TestGenerateRuby(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateRuby(root)
	if err != nil {
		t.Fatalf("GenerateRuby: %v", err)
	}
	code := string(out)

	checks := []string{
		"def add(a, b)",
		"puts(result)",
		"return (a + b)",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Ruby output missing %q\n---\n%s", c, code)
		}
	}
	if strings.Contains(code, "def main") {
		t.Errorf("Ruby should emit main body at top level, not def main:\n%s", code)
	}
}

func TestGenerateRubyWhile(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateRuby(root)
	if err != nil {
		t.Fatalf("GenerateRuby: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "while") || !strings.Contains(code, "end") {
		t.Errorf("Ruby should use 'while...end', got:\n%s", code)
	}
}

func TestGenerateRubyIfElse(t *testing.T) {
	root := mustParse(t, ifElseProgram)
	out, err := GenerateRuby(root)
	if err != nil {
		t.Fatalf("GenerateRuby: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "if ") || !strings.Contains(code, "else") || !strings.Contains(code, "end") {
		t.Errorf("Ruby should emit 'if...else...end', got:\n%s", code)
	}
}

// --- PHP codegen ---

func TestGeneratePHP(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GeneratePHP(root)
	if err != nil {
		t.Fatalf("GeneratePHP: %v", err)
	}
	code := string(out)

	checks := []string{
		"<?php",
		"function add(int $a, int $b): int",
		"echo $result",
		"return ($a + $b);",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("PHP output missing %q\n---\n%s", c, code)
		}
	}
	if strings.Contains(code, "function main") {
		t.Errorf("PHP should emit main body at top level, not function main:\n%s", code)
	}
}

func TestGeneratePHPStringConcat(t *testing.T) {
	root := mustParse(t, stringConcatProgram)
	out, err := GeneratePHP(root)
	if err != nil {
		t.Fatalf("GeneratePHP: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, " . ") {
		t.Errorf("PHP string concat should use '.', got:\n%s", code)
	}
}

func TestGeneratePHPIfElse(t *testing.T) {
	root := mustParse(t, ifElseProgram)
	out, err := GeneratePHP(root)
	if err != nil {
		t.Fatalf("GeneratePHP: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "if (") || !strings.Contains(code, "} else {") {
		t.Errorf("PHP should emit 'if (...) { ... } else { ... }', got:\n%s", code)
	}
}

// --- Zig codegen ---

func TestGenerateZig(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateZig(root)
	if err != nil {
		t.Fatalf("GenerateZig: %v", err)
	}
	code := string(out)

	checks := []string{
		"fn add(a: i64, b: i64) i64",
		"pub fn main()",
		"std.debug.print",
		"const std = @import(\"std\");",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Zig output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateZigIfElse(t *testing.T) {
	root := mustParse(t, ifElseProgram)
	out, err := GenerateZig(root)
	if err != nil {
		t.Fatalf("GenerateZig: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "if (") || !strings.Contains(code, "} else {") {
		t.Errorf("Zig should emit 'if (...) { ... } else { ... }', got:\n%s", code)
	}
}

func TestGenerateZigBoolOps(t *testing.T) {
	root := mustParse(t, boolProgram)
	out, err := GenerateZig(root)
	if err != nil {
		t.Fatalf("GenerateZig: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, " and ") {
		t.Errorf("Zig should map '&&' to 'and', got:\n%s", code)
	}
}

func TestGenerateZigMutability(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateZig(root)
	if err != nil {
		t.Fatalf("GenerateZig: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "var i: i64") {
		t.Errorf("Zig should use 'var' for reassigned var, got:\n%s", code)
	}
}

func TestGenerateZigStringConcat(t *testing.T) {
	root := mustParse(t, stringConcatProgram)
	out, err := GenerateZig(root)
	if err != nil {
		t.Fatalf("GenerateZig: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "++") {
		t.Errorf("Zig string concat should use '++', got:\n%s", code)
	}
}

func TestGenerateZigStringFmtSpec(t *testing.T) {
	// hello.xql.json pattern: println(greet("World")) where greet returns String
	root := mustParse(t, helloProgram)
	out, err := GenerateZig(root)
	if err != nil {
		t.Fatalf("GenerateZig: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, `{s}`) {
		t.Errorf("Zig should use {s} for string args in println, got:\n%s", code)
	}
	// Integer printing should still use {}
	root2 := mustParse(t, addFibMain)
	out2, err := GenerateZig(root2)
	if err != nil {
		t.Fatalf("GenerateZig: %v", err)
	}
	code2 := string(out2)
	if !strings.Contains(code2, `{}\n`) {
		t.Errorf("Zig should use {} for int args in println, got:\n%s", code2)
	}
}

// --- Nim codegen ---

func TestGenerateNim(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateNim(root)
	if err != nil {
		t.Fatalf("GenerateNim: %v", err)
	}
	code := string(out)

	checks := []string{
		"proc add(a: int64, b: int64): int64 =",
		"echo result",
		"return (a + b)",
		"let result: int64",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Nim output missing %q\n---\n%s", c, code)
		}
	}
	if strings.Contains(code, "proc main") {
		t.Errorf("Nim should emit main body at top level, not proc main:\n%s", code)
	}
}

func TestGenerateNimIfElse(t *testing.T) {
	root := mustParse(t, ifElseProgram)
	out, err := GenerateNim(root)
	if err != nil {
		t.Fatalf("GenerateNim: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "if ") || !strings.Contains(code, "else:") {
		t.Errorf("Nim should emit 'if ...:\\n...else:\\n...', got:\n%s", code)
	}
}

func TestGenerateNimBoolOps(t *testing.T) {
	root := mustParse(t, boolProgram)
	out, err := GenerateNim(root)
	if err != nil {
		t.Fatalf("GenerateNim: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, " and ") {
		t.Errorf("Nim should map '&&' to 'and', got:\n%s", code)
	}
}

func TestGenerateNimMutability(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateNim(root)
	if err != nil {
		t.Fatalf("GenerateNim: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "var i: int64") {
		t.Errorf("Nim should use 'var' for reassigned var, got:\n%s", code)
	}
}

func TestGenerateNimStringConcat(t *testing.T) {
	root := mustParse(t, stringConcatProgram)
	out, err := GenerateNim(root)
	if err != nil {
		t.Fatalf("GenerateNim: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, " & ") {
		t.Errorf("Nim string concat should use '&', got:\n%s", code)
	}
}

// --- Julia codegen ---

func TestGenerateJulia(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateJulia(root)
	if err != nil {
		t.Fatalf("GenerateJulia: %v", err)
	}
	code := string(out)

	checks := []string{
		"function add(a::Int64, b::Int64)::Int64",
		"function main()",
		"println(result)",
		"main()",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Julia output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateJuliaIfElse(t *testing.T) {
	root := mustParse(t, ifElseProgram)
	out, err := GenerateJulia(root)
	if err != nil {
		t.Fatalf("GenerateJulia: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "if ") || !strings.Contains(code, "else") || !strings.Contains(code, "end") {
		t.Errorf("Julia should emit 'if...else...end', got:\n%s", code)
	}
}

func TestGenerateJuliaStringConcat(t *testing.T) {
	root := mustParse(t, stringConcatProgram)
	out, err := GenerateJulia(root)
	if err != nil {
		t.Fatalf("GenerateJulia: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, " * ") {
		t.Errorf("Julia string concat should use '*', got:\n%s", code)
	}
}

// --- Struct codegen ---

func TestStructCodegenAll(t *testing.T) {
	root := mustParse(t, structProgram)

	type check struct {
		target string
		expect []string
	}
	cases := []check{
		{"go", []string{"type Point struct", "x int", "Point{x: 3"}},
		{"rust", []string{"struct Point", "x: i64", "Point {"}},
		{"ts", []string{"interface Point", "x: number", "x: 3"}},
		{"kotlin", []string{"data class Point", "val x: Long", "Point(x ="}},
		{"swift", []string{"struct Point", "var x: Int", "Point(x:"}},
		{"py", []string{"@dataclass", "class Point", "x: int", "Point(x="}},
		{"java", []string{"record Point", "long x", "new Point("}},
		{"csharp", []string{"record Point", "long x", "new Point("}},
		{"dart", []string{"class Point", "final int x", "Point(x:"}},
		{"lua", []string{"x = 3"}},
		{"ruby", []string{"Point = Struct.new", "Point.new("}},
		{"php", []string{"class Point", "public int $x", "new Point("}},
		{"zig", []string{"const Point = struct", "x: i64", "Point{"}},
		{"nim", []string{"type Point = object", "x: int64", "Point(x:"}},
		{"julia", []string{"struct Point", "x::Int64", "Point("}},
	}

	for _, tc := range cases {
		t.Run("struct_"+tc.target, func(t *testing.T) {
			out, err := Generate(root, tc.target)
			if err != nil {
				t.Fatalf("Generate(%q): %v", tc.target, err)
			}
			code := string(out)
			for _, s := range tc.expect {
				if !strings.Contains(code, s) {
					t.Errorf("%s output missing %q\n---\n%s", tc.target, s, code)
				}
			}
		})
	}
}

// --- Collection codegen ---

func TestCollectionCodegenAll(t *testing.T) {
	root := mustParse(t, collectionProgram)

	type check struct {
		target string
		expect []string
	}
	cases := []check{
		{"go", []string{"[]int{10, 20, 30}", "nums[0]"}},
		{"rust", []string{"vec![10, 20, 30]", "nums[0 as usize]"}},
		{"ts", []string{"[10, 20, 30]", "nums[0]"}},
		{"kotlin", []string{"listOf(10L, 20L, 30L)", "nums[(0L).toInt()]"}},
		{"swift", []string{"[10, 20, 30]", "nums[0]"}},
		{"py", []string{"[10, 20, 30]", "nums[0]"}},
		{"java", []string{"java.util.List.of(10L, 20L, 30L)", "nums.get("}},
		{"csharp", []string{"new List<long>", "{ 10L, 20L, 30L }", "nums[(int)"}},
		{"dart", []string{"[10, 20, 30]", "nums[0]"}},
		{"lua", []string{"{10, 20, 30}", "nums[0]"}},
		{"ruby", []string{"[10, 20, 30]", "nums[0]"}},
		{"php", []string{"[10, 20, 30]", "$nums[0]"}},
		{"zig", []string{"&[_]i64{", "nums[@intCast(0)]"}},
		{"nim", []string{"@[10'i64", "nums[0'i64]"}},
		{"julia", []string{"Int64[Int64(10)", "nums[Int64(0)]"}},
	}

	for _, tc := range cases {
		t.Run("collection_"+tc.target, func(t *testing.T) {
			out, err := Generate(root, tc.target)
			if err != nil {
				t.Fatalf("Generate(%q): %v", tc.target, err)
			}
			code := string(out)
			for _, s := range tc.expect {
				if !strings.Contains(code, s) {
					t.Errorf("%s output missing %q\n---\n%s", tc.target, s, code)
				}
			}
		})
	}
}

// --- Result type rejection ---

func TestResultTypeRejection(t *testing.T) {
	// Program using Result<Int> return type
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "tryParse",
			"params": [{"name": "s", "type": {"kind": "String"}}],
			"returnType": {"kind": "Result", "okType": {"kind": "Int"}},
			"effects": [],
			"grant": [],
			"body": [{
				"kind": "ReturnStmt",
				"value": {"kind": "Literal", "valueType": "Int", "value": 42}
			}]
		}]
	}`
	root := mustParse(t, src)

	// These targets should reject Result
	rejectTargets := []string{"ts", "dart", "nim", "julia", "lua", "ruby"}
	for _, tgt := range rejectTargets {
		_, err := Generate(root, tgt)
		if err == nil {
			t.Errorf("Generate(%q) should reject Result type, but succeeded", tgt)
		} else if !strings.Contains(err.Error(), "XQL_E402") {
			t.Errorf("Generate(%q) error should be XQL_E402, got: %v", tgt, err)
		}
	}

	// These targets should accept Result
	acceptTargets := []string{"go", "rust", "kotlin", "swift", "py", "java", "csharp", "php", "zig"}
	for _, tgt := range acceptTargets {
		_, err := Generate(root, tgt)
		if err != nil {
			t.Errorf("Generate(%q) should accept Result type, got error: %v", tgt, err)
		}
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
