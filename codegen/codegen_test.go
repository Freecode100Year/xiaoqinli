package codegen

import (
	"os"
	"path/filepath"
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
		"return (a + b)",
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
	if !strings.Contains(code, "for (i < 10)") {
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
	targets := []string{
		"go", "rust", "ts", "kotlin", "swift", "py",
		"java", "csharp", "dart", "lua", "ruby", "php",
		"zig", "nim", "julia", "cpp", "c", "haskell",
		"ocaml", "elixir",
		"awk", "bash", "crystal", "d", "fortran",
		"pascal", "perl", "powershell", "tcl",
		"vala", "groovy", "bat", "shortcut", "chrome",
	}
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
	// This test used to require `++`, and got it — which is precisely how the
	// defect survived. `++` joins arrays at comptime; handed a slice whose
	// contents are only known at runtime it does not produce a wrong answer, it
	// refuses to build. So the assertion is inverted: the helper must be there,
	// defined as well as called, and `++` must not be.
	if !strings.Contains(code, zigConcatFn+"(") {
		t.Errorf("Zig string concat should call %s, got:\n%s", zigConcatFn, code)
	}
	if !strings.Contains(code, "fn "+zigConcatFn+"(") {
		t.Errorf("%s is called but never defined, got:\n%s", zigConcatFn, code)
	}
	if strings.Contains(code, "++") {
		t.Errorf("`++` cannot concatenate a runtime slice, got:\n%s", code)
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
		{"java", []string{"class Point", "long x", "new Point("}},
		{"csharp", []string{"record Point", "long x", "new Point("}},
		{"dart", []string{"class Point", "final int x", "Point(x:"}},
		{"lua", []string{"x = 3"}},
		{"ruby", []string{"Point = Struct.new", "Point.new("}},
		{"php", []string{"class Point", "public int $x", "new Point("}},
		{"zig", []string{"const Point = struct", "x: i64", "Point{"}},
		{"nim", []string{"type Point = object", "x: int64", "Point(x:"}},
		{"julia", []string{"struct Point", "x::Int64", "Point("}},
		{"cpp", []string{"struct Point {", "long x;", "Point{"}},
		{"chrome", []string{"class Point", "constructor(x, y)", "toString()"}},
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
		{"lua", []string{"{10, 20, 30}", "nums[(0) + 1]"}},
		{"ruby", []string{"[10, 20, 30]", "nums[0]"}},
		{"php", []string{"[10, 20, 30]", "$nums[0]"}},
		{"zig", []string{"&[_]i64{", "nums[@intCast(0)]"}},
		{"nim", []string{"@[10'i64", "nums[0'i64]"}},
		{"julia", []string{"Int64[Int64(10)", "nums[(Int64(0)) + 1]"}},
		{"cpp", []string{"std::vector<long long>{10LL, 20LL, 30LL}", "nums[0LL]"}},
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

	// These targets should reject Result. nim is not a permanent exclusion:
	// Result.ok/Result.err need both the Ok and Err type parameters at the
	// call site, and Nim's generic return-type inference (confirmed against
	// a real Nim 2.2.10 compiler) cannot recover the type that appears only
	// in the declared return type, not in any argument — fixing this needs
	// expected-type context threaded through the Nim backend, which the
	// codegen layer does not currently track.
	rejectTargets := []string{"nim"}
	for _, tgt := range rejectTargets {
		_, err := Generate(root, tgt)
		if err == nil {
			t.Errorf("Generate(%q) should reject Result type, but succeeded", tgt)
		} else if !strings.Contains(err.Error(), "XQL_E402") {
			t.Errorf("Generate(%q) error should be XQL_E402, got: %v", tgt, err)
		}
	}

	// These targets should accept Result. lua, ruby and julia now emit a
	// real Result wrapper instead of collapsing the type; see
	// TestGenerateLuaResultSemantics / TestGenerateRubyResultSemantics /
	// TestGenerateJuliaResultSemantics for physically-verified round trips.
	acceptTargets := []string{"go", "rust", "kotlin", "swift", "py", "java", "csharp", "php", "zig", "ts", "dart", "lua", "ruby", "julia"}
	for _, tgt := range acceptTargets {
		_, err := Generate(root, tgt)
		if err != nil {
			t.Errorf("Generate(%q) should accept Result type, got error: %v", tgt, err)
		}
	}
}

// resultRoundTripProgram exercises Result.ok, Result.err, .isOk, .unwrap and
// .unwrapErr together, matching examples/e2e_workspace/service.xql's shape.
const resultRoundTripProgram = `{
	"kind": "Program",
	"declarations": [{
		"kind": "FunctionDecl",
		"name": "parse",
		"params": [{"name": "n", "type": {"kind": "Int"}}],
		"returnType": {"kind": "Result", "okType": {"kind": "Int"}, "errType": {"kind": "String"}},
		"effects": [],
		"grant": [],
		"body": [{
			"kind": "IfStmt",
			"condition": {"kind": "BinaryExpr", "op": "<",
				"left": {"kind": "Ident", "name": "n"},
				"right": {"kind": "Literal", "valueType": "Int", "value": 0}},
			"then": [{"kind": "ReturnStmt", "value": {"kind": "CallExpr", "callee": "Result.err",
				"args": [{"kind": "Literal", "valueType": "String", "value": "negative"}]}}],
			"else": [{"kind": "ReturnStmt", "value": {"kind": "CallExpr", "callee": "Result.ok",
				"args": [{"kind": "Ident", "name": "n"}]}}]
		}]
	}, {
		"kind": "FunctionDecl",
		"name": "main",
		"params": [],
		"returnType": {"kind": "Void"},
		"effects": ["state"],
		"grant": ["io"],
		"body": [
			{
				"kind": "VarDecl",
				"name": "res",
				"type": {"kind": "Result", "okType": {"kind": "Int"}, "errType": {"kind": "String"}},
				"value": {"kind": "CallExpr", "callee": "parse", "args": [{"kind": "Literal", "valueType": "Int", "value": 5}]}
			},
			{
				"kind": "IfStmt",
				"condition": {"kind": "MemberExpr", "object": {"kind": "Ident", "name": "res"}, "field": "isOk"},
				"then": [{"kind": "ExprStmt", "expr": {"kind": "CallExpr", "callee": "println",
					"args": [{"kind": "CallExpr", "callee": "res.unwrap", "args": []}]}}],
				"else": [{"kind": "ExprStmt", "expr": {"kind": "CallExpr", "callee": "println",
					"args": [{"kind": "CallExpr", "callee": "res.unwrapErr", "args": []}]}}]
			}
		]
	}]
}`

// TestGenerateLuaResultSemantics pins the Lua Result wrapper: physically
// verified against a real Lua 5.4.6 interpreter (see local_e2e_test.go /
// TestLocalE2EWorkspaceDogfood/Lua for the interpreter-backed check).
func TestGenerateLuaResultSemantics(t *testing.T) {
	root := mustParse(t, resultRoundTripProgram)
	out, err := GenerateLua(root)
	if err != nil {
		t.Fatalf("GenerateLua: %v", err)
	}
	code := string(out)
	for _, s := range []string{"Result.ok(", "Result.err(", "res.isOk", "res.unwrap()", "res.unwrapErr()"} {
		if !strings.Contains(code, s) {
			t.Errorf("Lua Result output missing %q\n---\n%s", s, code)
		}
	}
}

// TestGenerateRubyResultSemantics pins the Ruby Result class: physically
// verified against a real Ruby 3.3.12 interpreter.
func TestGenerateRubyResultSemantics(t *testing.T) {
	root := mustParse(t, resultRoundTripProgram)
	out, err := GenerateRuby(root)
	if err != nil {
		t.Fatalf("GenerateRuby: %v", err)
	}
	code := string(out)
	for _, s := range []string{"class Result", "def self.ok", "def self.err", "def isOk", "def unwrap", "def unwrapErr", "Result.ok(", "Result.err(", "res.isOk", "res.unwrap()", "res.unwrapErr()"} {
		if !strings.Contains(code, s) {
			t.Errorf("Ruby Result output missing %q\n---\n%s", s, code)
		}
	}
}

// TestGenerateJuliaResultSemantics pins the Julia Result struct: physically
// verified against a real Julia 1.12.6 interpreter.
func TestGenerateJuliaResultSemantics(t *testing.T) {
	root := mustParse(t, resultRoundTripProgram)
	out, err := GenerateJulia(root)
	if err != nil {
		t.Fatalf("GenerateJulia: %v", err)
	}
	code := string(out)
	for _, s := range []string{"mutable struct Result", "resultOk(v)", "resultErr(e)", "resultOk(", "resultErr(", "res.isOk", "xqlUnwrap(res)", "xqlUnwrapErr(res)"} {
		if !strings.Contains(code, s) {
			t.Errorf("Julia Result output missing %q\n---\n%s", s, code)
		}
	}
	if strings.Contains(code, "Result.ok") || strings.Contains(code, "Result.err") {
		t.Errorf("Julia output still contains unrewritten Result.ok/Result.err callee:\n%s", code)
	}
}

// --- ForStmt codegen across all backends ---

const forRangeProgram = `{
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
				"kind": "ForStmt", "form": "range", "var": "i",
				"start": {"kind": "Literal", "valueType": "Int", "value": 0},
				"end": {"kind": "Literal", "valueType": "Int", "value": 5},
				"body": [{
					"kind": "ExprStmt",
					"expr": {"kind": "CallExpr", "callee": "println", "args": [
						{"kind": "Ident", "name": "i"}
					]}
				}]
			}
		]
	}]
}`

func TestForStmtCodegenAll(t *testing.T) {
	targets := []string{
		"go", "rust", "ts", "kotlin", "swift", "py",
		"java", "csharp", "dart", "lua", "ruby", "php",
		"zig", "nim", "julia", "cpp",
		"c", "haskell",
		"ocaml", "elixir",
		"awk", "bash", "crystal", "d", "fortran",
		"pascal", "perl", "powershell", "tcl",
		"vala", "groovy", "bat", "shortcut", "chrome",
	}
	root := mustParse(t, forRangeProgram)
	for _, target := range targets {
		t.Run("for_"+target, func(t *testing.T) {
			out, err := Generate(root, target)
			if err != nil {
				t.Fatalf("codegen error: %v", err)
			}
			if len(out) == 0 {
				t.Fatal("empty output")
			}
		})
	}
}

const breakContinueProgram = `{
	"kind": "Program",
	"declarations": [{
		"kind": "FunctionDecl",
		"name": "main",
		"params": [],
		"returnType": {"kind": "Void"},
		"effects": ["state"],
		"grant": ["io"],
		"body": [{
			"kind": "WhileStmt",
			"cond": {"kind": "Literal", "valueType": "Bool", "value": true},
			"body": [
				{
					"kind": "IfStmt",
					"cond": {"kind": "Literal", "valueType": "Bool", "value": true},
					"then": [{"kind": "BreakStmt"}],
					"else": [{"kind": "ContinueStmt"}]
				}
			]
		}]
	}]
}`

func TestBreakContinueCodegenAll(t *testing.T) {
	// Excluded: ocaml, haskell, fsharp (no break/continue), bat (limited control flow),
	// lua, ada (no continue), elixir, clojure (functional — no break/continue)
	targets := []string{
		"go", "rust", "ts", "kotlin", "swift", "py",
		"java", "csharp", "dart", "ruby", "php",
		"zig", "nim", "julia", "cpp",
		"c",
		"awk", "bash", "crystal", "d", "fortran",
		"pascal", "perl", "powershell", "tcl",
		"vala", "groovy", "chrome",
	}
	root := mustParse(t, breakContinueProgram)
	for _, target := range targets {
		t.Run("brk_"+target, func(t *testing.T) {
			out, err := Generate(root, target)
			if err != nil {
				t.Fatalf("codegen error: %v", err)
			}
			if len(out) == 0 {
				t.Fatal("empty output")
			}
		})
	}
}

const assignIndexProgram = `{
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
				"kind": "VarDecl", "name": "nums",
				"type": {"kind": "Array", "elem": {"kind": "Int"}},
				"value": {"kind": "ArrayLit", "elemType": {"kind": "Int"}, "elements": [
					{"kind": "Literal", "valueType": "Int", "value": 1},
					{"kind": "Literal", "valueType": "Int", "value": 2}
				]}
			},
			{
				"kind": "AssignStmt",
				"target": {"kind": "IndexExpr",
					"target": {"kind": "Ident", "name": "nums"},
					"index": {"kind": "Literal", "valueType": "Int", "value": 0}
				},
				"value": {"kind": "Literal", "valueType": "Int", "value": 99}
			},
			{
				"kind": "ExprStmt",
				"expr": {"kind": "CallExpr", "callee": "println", "args": [
					{"kind": "IndexExpr",
						"target": {"kind": "Ident", "name": "nums"},
						"index": {"kind": "Literal", "valueType": "Int", "value": 0}
					}
				]}
			}
		]
	}]
}`

func TestAssignIndexCodegenAll(t *testing.T) {
	// Excluded: haskell (functional — no imperative index assignment), fortran, bat (limited array support)
	targets := []string{
		"go", "rust", "ts", "kotlin", "swift", "py",
		"java", "csharp", "dart", "lua", "ruby", "php",
		"zig", "nim", "julia", "cpp",
		"c",
		"ocaml", "elixir",
		"awk", "bash", "crystal", "d",
		"pascal", "perl", "powershell", "tcl",
		"vala", "groovy", "chrome",
	}
	root := mustParse(t, assignIndexProgram)
	for _, target := range targets {
		t.Run("aidx_"+target, func(t *testing.T) {
			out, err := Generate(root, target)
			if err != nil {
				t.Fatalf("codegen error: %v", err)
			}
			if len(out) == 0 {
				t.Fatal("empty output")
			}
		})
	}
}

// --- C++ codegen ---

func TestGenerateCpp(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateCpp(root)
	if err != nil {
		t.Fatalf("GenerateCpp: %v", err)
	}
	code := string(out)

	checks := []string{
		"#include <iostream>",
		"long long add(long long a, long long b)",
		"int main()",
		"std::cout << result << std::endl",
		"return (a + b);",
		"return 0;",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("C++ output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateCppStringConcat(t *testing.T) {
	root := mustParse(t, stringConcatProgram)
	out, err := GenerateCpp(root)
	if err != nil {
		t.Fatalf("GenerateCpp: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "#include <string>") {
		t.Errorf("C++ should include <string>, got:\n%s", code)
	}
	if !strings.Contains(code, "std::string greet(const std::string& name)") {
		t.Errorf("C++ should pass String as const ref, got:\n%s", code)
	}
}

func TestGenerateCppIfElse(t *testing.T) {
	root := mustParse(t, ifElseProgram)
	out, err := GenerateCpp(root)
	if err != nil {
		t.Fatalf("GenerateCpp: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "if (") || !strings.Contains(code, "} else {") {
		t.Errorf("C++ should emit 'if (...) { ... } else { ... }', got:\n%s", code)
	}
}

func TestGenerateCppWhile(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateCpp(root)
	if err != nil {
		t.Fatalf("GenerateCpp: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "while (") {
		t.Errorf("C++ should emit while, got:\n%s", code)
	}
}

func TestGenerateCppResultRejection(t *testing.T) {
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
	_, err := GenerateCpp(root)
	if err == nil {
		t.Fatal("C++ should reject Result type")
	}
	if !strings.Contains(err.Error(), "XQL_E402") {
		t.Errorf("expected XQL_E402, got: %v", err)
	}
}

// --- MQL4/MQL5 codegen ---

func TestGenerateMQLStruct(t *testing.T) {
	root := mustParse(t, structProgram)
	for _, dialect := range []string{} {
		t.Run(dialect, func(t *testing.T) {
			out, err := Generate(root, dialect)
			if err != nil {
				t.Fatalf("Generate(%q): %v", dialect, err)
			}
			code := string(out)
			if !strings.Contains(code, "struct Point {") {
				t.Errorf("MQL should emit struct, got:\n%s", code)
			}
			if !strings.Contains(code, "long x;") {
				t.Errorf("MQL should emit long field, got:\n%s", code)
			}
		})
	}
}

func TestGenerateMQLMapRejection(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [{"name": "m", "type": {"kind": "Map", "keyType": {"kind": "String"}, "elem": {"kind": "Int"}}}],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": []
		}]
	}`
	root := mustParse(t, src)
	for _, dialect := range []string{} {
		_, err := Generate(root, dialect)
		if err == nil {
			t.Errorf("MQL %s should reject Map type", dialect)
		} else if !strings.Contains(err.Error(), "XQL_E402") {
			t.Errorf("expected XQL_E402 for %s, got: %v", dialect, err)
		}
	}
}

func TestGenerateMQLOptionRejection(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [{"name": "x", "type": {"kind": "Option", "elem": {"kind": "Int"}}}],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": []
		}]
	}`
	root := mustParse(t, src)
	for _, dialect := range []string{} {
		_, err := Generate(root, dialect)
		if err == nil {
			t.Errorf("MQL %s should reject Option type", dialect)
		} else if !strings.Contains(err.Error(), "XQL_E402") {
			t.Errorf("expected XQL_E402 for %s, got: %v", dialect, err)
		}
	}
}

func TestGenerateMQLForEachRejection(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "main",
			"params": [],
			"returnType": {"kind": "Void"},
			"effects": [],
			"grant": [],
			"body": [{
				"kind": "ForStmt", "form": "each", "var": "item",
				"iterable": {"kind": "Ident", "name": "items"},
				"body": []
			}]
		}]
	}`
	root := mustParse(t, src)
	for _, dialect := range []string{} {
		_, err := Generate(root, dialect)
		if err == nil {
			t.Errorf("MQL %s should reject for-each loops", dialect)
		} else if !strings.Contains(err.Error(), "XQL_E402") {
			t.Errorf("expected XQL_E402 for %s for-each, got: %v", dialect, err)
		}
	}
}

// --- collectMutables tests ---

func TestCollectMutables(t *testing.T) {
	stmts := []ast.Node{
		&ast.VarDecl{Name: "x"},
		&ast.AssignStmt{Target: &ast.Ident{Name: "x"}, Value: &ast.Literal{ValueType: "Int", Value: float64(1)}},
		&ast.VarDecl{Name: "y"},
		&ast.IfStmt{
			Cond: &ast.Literal{ValueType: "Bool", Value: true},
			Then: []ast.Node{
				&ast.AssignStmt{Target: &ast.Ident{Name: "y"}, Value: &ast.Literal{ValueType: "Int", Value: float64(2)}},
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

func TestCollectMutablesClosure(t *testing.T) {
	stmts := []ast.Node{
		&ast.VarDecl{Name: "y"},
		&ast.VarDecl{
			Name: "lam",
			Value: &ast.Lambda{
				Params: []ast.Param{},
				Body: []ast.Node{
					&ast.VarDecl{Name: "x"},
					&ast.AssignStmt{Target: &ast.Ident{Name: "x"}, Value: &ast.Literal{ValueType: "Int", Value: float64(2)}},
					&ast.AssignStmt{Target: &ast.Ident{Name: "y"}, Value: &ast.Literal{ValueType: "Int", Value: float64(99)}},
				},
			},
		},
	}

	muts := collectMutables(stmts)
	if !muts["y"] {
		t.Error("y should be mutable (assigned inside closure)")
	}
	if muts["x"] {
		t.Error("x should not be mutable globally (it is a local variable inside the lambda)")
	}
}

// --- C codegen ---

func TestGenerateC(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateC(root)
	if err != nil {
		t.Fatalf("GenerateC: %v", err)
	}
	code := string(out)

	checks := []string{
		"#include <stdio.h>",
		"long long add(long long a, long long b)",
		"int main()",
		"printf(",
		"return (a + b);",
		"return 0;",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("C output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateCStringConcat(t *testing.T) {
	root := mustParse(t, stringConcatProgram)
	out, err := GenerateC(root)
	if err != nil {
		t.Fatalf("GenerateC: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "_xql_strcat") {
		t.Errorf("C string concat should use _xql_strcat, got:\n%s", code)
	}
}

func TestGenerateCIfElse(t *testing.T) {
	root := mustParse(t, ifElseProgram)
	out, err := GenerateC(root)
	if err != nil {
		t.Fatalf("GenerateC: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "if (") || !strings.Contains(code, "} else {") {
		t.Errorf("C should emit 'if (...) { ... } else { ... }', got:\n%s", code)
	}
}

func TestGenerateCWhile(t *testing.T) {
	root := mustParse(t, whileProgram)
	out, err := GenerateC(root)
	if err != nil {
		t.Fatalf("GenerateC: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "while (") {
		t.Errorf("C should emit while, got:\n%s", code)
	}
}

func TestGenerateCStruct(t *testing.T) {
	root := mustParse(t, structProgram)
	out, err := GenerateC(root)
	if err != nil {
		t.Fatalf("GenerateC: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "typedef struct") || !strings.Contains(code, "long x;") {
		t.Errorf("C should emit typedef struct, got:\n%s", code)
	}
}

// --- Scala codegen ---

// --- Haskell codegen ---

func TestGenerateHaskell(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateHaskell(root)
	if err != nil {
		t.Fatalf("GenerateHaskell: %v", err)
	}
	code := string(out)

	checks := []string{
		"module Main where",
		"add :: Int -> Int -> Int",
		"main :: IO ()",
		"main = do",
		"print",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Haskell output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateHaskellHello(t *testing.T) {
	root := mustParse(t, helloProgram)
	out, err := GenerateHaskell(root)
	if err != nil {
		t.Fatalf("GenerateHaskell: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "putStrLn") {
		t.Errorf("Haskell should use putStrLn for string output, got:\n%s", code)
	}
	if !strings.Contains(code, "++") {
		t.Errorf("Haskell should use ++ for string concat, got:\n%s", code)
	}
}

// --- EnumDecl + MatchExpr codegen ---

const enumMatchProgram = `{
	"kind": "Program",
	"declarations": [
		{
			"kind": "EnumDecl",
			"name": "Color",
			"variants": ["Red", "Green", "Blue"]
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
					"name": "x",
					"type": {"kind": "Int"},
					"value": {"kind": "Literal", "valueType": "Int", "value": 2}
				},
				{
					"kind": "MatchExpr",
					"value": {"kind": "Ident", "name": "x"},
					"arms": [
						{
							"pattern": {"kind": "Literal", "valueType": "Int", "value": 1},
							"body": [{
								"kind": "ExprStmt",
								"expr": {"kind": "CallExpr", "callee": "println", "args": [
									{"kind": "Literal", "valueType": "String", "value": "one"}
								]}
							}]
						},
						{
							"pattern": {"kind": "Ident", "name": "_"},
							"body": [{
								"kind": "ExprStmt",
								"expr": {"kind": "CallExpr", "callee": "println", "args": [
									{"kind": "Literal", "valueType": "String", "value": "other"}
								]}
							}]
						}
					]
				}
			]
		}
	]
}`

func TestEnumMatchCodegenGo(t *testing.T) {
	root := mustParse(t, enumMatchProgram)
	out, err := GenerateGo(root)
	if err != nil {
		t.Fatalf("GenerateGo: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "type Color int") {
		t.Errorf("Go should emit enum as type + iota, got:\n%s", code)
	}
	if !strings.Contains(code, "switch x") {
		t.Errorf("Go should emit switch for match, got:\n%s", code)
	}
	if !strings.Contains(code, "default:") {
		t.Errorf("Go should emit default for _ pattern, got:\n%s", code)
	}
}

func TestEnumMatchCodegenRust(t *testing.T) {
	root := mustParse(t, enumMatchProgram)
	out, err := GenerateRust(root)
	if err != nil {
		t.Fatalf("GenerateRust: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "enum Color") {
		t.Errorf("Rust should emit enum, got:\n%s", code)
	}
	if !strings.Contains(code, "match x") {
		t.Errorf("Rust should emit match, got:\n%s", code)
	}
}

func TestEnumMatchCodegenPython(t *testing.T) {
	root := mustParse(t, enumMatchProgram)
	out, err := GeneratePython(root)
	if err != nil {
		t.Fatalf("GeneratePython: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "class Color(Enum)") {
		t.Errorf("Python should emit Enum class, got:\n%s", code)
	}
	if !strings.Contains(code, "match x") {
		t.Errorf("Python should emit match statement, got:\n%s", code)
	}
}

func TestEnumMatchCodegenCpp(t *testing.T) {
	root := mustParse(t, enumMatchProgram)
	out, err := GenerateCpp(root)
	if err != nil {
		t.Fatalf("GenerateCpp: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "enum class Color") {
		t.Errorf("C++ should emit enum class, got:\n%s", code)
	}
	if !strings.Contains(code, "switch (x)") {
		t.Errorf("C++ should emit switch, got:\n%s", code)
	}
}

func TestEnumMatchCodegenC(t *testing.T) {
	root := mustParse(t, enumMatchProgram)
	out, err := GenerateC(root)
	if err != nil {
		t.Fatalf("GenerateC: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "typedef enum") {
		t.Errorf("C should emit typedef enum, got:\n%s", code)
	}
	if !strings.Contains(code, "switch (x)") {
		t.Errorf("C should emit switch, got:\n%s", code)
	}
}

func TestEnumMatchCodegenHaskell(t *testing.T) {
	root := mustParse(t, enumMatchProgram)
	out, err := GenerateHaskell(root)
	if err != nil {
		t.Fatalf("GenerateHaskell: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "data Color") {
		t.Errorf("Haskell should emit data Color, got:\n%s", code)
	}
	if !strings.Contains(code, "case") {
		t.Errorf("Haskell should emit case expression, got:\n%s", code)
	}
}

func TestEnumMatchCodegenMultiTarget(t *testing.T) {
	root := mustParse(t, enumMatchProgram)
	targets := []string{
		"go", "rust", "ts", "kotlin", "swift", "py",
		"java", "csharp", "dart", "lua", "ruby", "php",
		"zig", "nim", "julia", "cpp", "c", "haskell",
		"ocaml", "elixir",
		"awk", "bash", "crystal", "d", "fortran",
		"pascal", "perl", "powershell", "tcl",
		"vala", "groovy", "bat", "shortcut", "chrome",
	}
	for _, tgt := range targets {
		t.Run("match_"+tgt, func(t *testing.T) {
			out, err := Generate(root, tgt)
			if err != nil {
				t.Fatalf("Generate(%q): %v", tgt, err)
			}
			if len(out) == 0 {
				t.Fatalf("Generate(%q) returned empty", tgt)
			}
		})
	}
}

func TestHaskellPureIfNoDoBlock(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateHaskell(root)
	if err != nil {
		t.Fatalf("GenerateHaskell: %v", err)
	}
	code := string(out)
	if strings.Contains(code, "then do") {
		if !strings.Contains(code, "main = do") || strings.Count(code, "then do") > 0 {
			lines := strings.Split(code, "\n")
			for _, l := range lines {
				if strings.Contains(l, "then do") {
					t.Errorf("pure function should not use 'then do', found: %s", l)
				}
			}
		}
	}
}

const printfMultiArgProgram = `{
	"kind": "Program",
	"declarations": [{
		"kind": "FunctionDecl",
		"name": "main",
		"params": [],
		"returnType": {"kind": "Void"},
		"effects": ["state"],
		"grant": ["io"],
		"body": [{
			"kind": "VarDecl",
			"name": "name",
			"type": {"kind": "String"},
			"value": {"kind": "Literal", "valueType": "String", "value": "world"}
		}, {
			"kind": "ExprStmt",
			"expr": {
				"kind": "CallExpr",
				"callee": "printf",
				"args": [
					{"kind": "Literal", "valueType": "String", "value": "hello %s"},
					{"kind": "Ident", "name": "name"}
				]
			}
		}]
	}]
}`

func TestCPrintfMultiArg(t *testing.T) {
	root := mustParse(t, printfMultiArgProgram)
	out, err := GenerateC(root)
	if err != nil {
		t.Fatalf("GenerateC: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, `printf("hello %s", name)`) {
		t.Errorf("C printf should pass all args, got:\n%s", code)
	}
}

// --- Chrome Extension codegen ---

func TestGenerateChromeHello(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateChrome(root)
	if err != nil {
		t.Fatalf("GenerateChrome: %v", err)
	}
	code := string(out)
	checks := []string{
		`"manifest_version": 3`,
		`"default_popup": "popup.html"`,
		`<script src=\"popup.js\">`,
		"function add(a, b)",
		"DOMContentLoaded",
		"_xql_print(",
		"try {",
		"} catch (_err)",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Chrome output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateChromeStruct(t *testing.T) {
	root := mustParse(t, structProgram)
	out, err := GenerateChrome(root)
	if err != nil {
		t.Fatalf("GenerateChrome: %v", err)
	}
	code := string(out)
	checks := []string{
		"class Point",
		"constructor(x, y)",
		"this.x = x",
		"toString()",
		"_xql_str(this.x)",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Chrome struct output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateChromePrintf(t *testing.T) {
	root := mustParse(t, printfMultiArgProgram)
	out, err := GenerateChrome(root)
	if err != nil {
		t.Fatalf("GenerateChrome: %v", err)
	}
	code := string(out)
	checks := []string{
		"_xql_printf(",
		`\"hello %s\"`,
		", name)",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Chrome printf output missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateChromeHelpers(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := GenerateChrome(root)
	if err != nil {
		t.Fatalf("GenerateChrome: %v", err)
	}
	code := string(out)
	checks := []string{
		"function _xql_out(s)",
		"function _xql_str(v)",
		"function _xql_print(v)",
		"function _xql_printf(fmt)",
		"Array.isArray(v)",
		"JSON.stringify(v)",
		"'use strict'",
	}
	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("Chrome helpers missing %q\n---\n%s", c, code)
		}
	}
}

func TestGenerateImportDecl(t *testing.T) {
	src := `{
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
				"grant": [],
				"body": [
					{
						"kind": "VarDecl",
						"name": "p",
						"type": {"kind": "utils.Point"},
						"value": null
					},
					{
						"kind": "ExprStmt",
						"expr": {
							"kind": "CallExpr",
							"callee": "utils.netCall",
							"args": []
						}
					}
				]
			}
		]
	}`

	root := mustParse(t, src)

	// Go Target Check
	goOut, err := Generate(root, "go")
	if err != nil {
		t.Fatalf("Generate go failed: %v", err)
	}
	goCode := string(goOut)
	if strings.Contains(goCode, "import") {
		t.Errorf("expected Go code to not contain import, got: %s", goCode)
	}
	if !strings.Contains(goCode, "var p Point") {
		t.Errorf("expected var p Point in Go, got: %s", goCode)
	}
	if !strings.Contains(goCode, "netCall()") {
		t.Errorf("expected netCall() in Go, got: %s", goCode)
	}

	// TypeScript Target Check
	tsOut, err := Generate(root, "ts")
	if err != nil {
		t.Fatalf("Generate ts failed: %v", err)
	}
	tsCode := string(tsOut)
	if !strings.Contains(tsCode, `import * as utils from "./utils";`) {
		t.Errorf("expected import statement in TS, got: %s", tsCode)
	}
	if !strings.Contains(tsCode, "p: utils.Point") {
		t.Errorf("expected p: utils.Point in TS, got: %s", tsCode)
	}
	if !strings.Contains(tsCode, "utils.netCall()") {
		t.Errorf("expected utils.netCall() in TS, got: %s", tsCode)
	}

	// Python Target Check
	pyOut, err := Generate(root, "py")
	if err != nil {
		t.Fatalf("Generate py failed: %v", err)
	}
	pyCode := string(pyOut)
	if !strings.Contains(pyCode, "import utils as utils") {
		t.Errorf("expected import utils in Python, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, "p: utils.Point") {
		t.Errorf("expected p: utils.Point in Python, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, "utils.netCall()") {
		t.Errorf("expected utils.netCall() in Python, got: %s", pyCode)
	}

	// Rust Target Check
	rsOut, err := Generate(root, "rust")
	if err != nil {
		t.Fatalf("Generate rust failed: %v", err)
	}
	rsCode := string(rsOut)
	if !strings.Contains(rsCode, "mod utils;") {
		t.Errorf("expected mod utils in Rust, got: %s", rsCode)
	}
	if !strings.Contains(rsCode, "let p: utils::Point") {
		t.Errorf("expected let p: utils::Point in Rust, got: %s", rsCode)
	}
	if !strings.Contains(rsCode, "utils::netCall()") {
		t.Errorf("expected utils::netCall() in Rust, got: %s", rsCode)
	}
}

func TestLanguageProfileSelfUpdate(t *testing.T) {
	// 1. Inspect default Python spec profile
	pyProf, err := InspectLanguageProfile("py")
	if err != nil {
		t.Fatalf("InspectLanguageProfile py error: %v", err)
	}
	if pyProf.LatestVersion != "3.12+" {
		t.Errorf("expected Python latest_version 3.12+, got %s", pyProf.LatestVersion)
	}

	// 2. Self-update Python spec profile with Python 3.13 features
	updatedProf, err := UpdateLanguageProfile(LanguageProfile{
		Target:        "py",
		Language:      "Python",
		LatestVersion: "3.13+",
		ModernFeatures: []string{
			"PEP 604 union types (T | None)",
			"dataclasses",
			"PEP 703 Free-threaded Python (no-GIL)",
			"JIT compiler support",
		},
	})
	if err != nil {
		t.Fatalf("UpdateLanguageProfile py error: %v", err)
	}
	if updatedProf.LatestVersion != "3.13+" {
		t.Errorf("expected updated version 3.13+, got %s", updatedProf.LatestVersion)
	}

	// 3. Inspect updated Python spec
	reInspected, err := InspectLanguageProfile("py")
	if err != nil {
		t.Fatalf("Re-inspect py error: %v", err)
	}
	if reInspected.LatestVersion != "3.13+" {
		t.Errorf("expected re-inspected version 3.13+, got %s", reInspected.LatestVersion)
	}

	// 4. Ensure all 42+ targets are available in ListAllLanguageProfiles
	all := ListAllLanguageProfiles()
	if len(all) < 42 {
		t.Errorf("expected at least 42 target profiles, got %d", len(all))
	}
}

func TestGenerateTCCLI(t *testing.T) {
	root := mustParse(t, addFibMain)
	out, err := Generate(root, "tccli")
	if err != nil {
		t.Fatalf("Generate tccli error: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "#!/bin/bash") {
		t.Errorf("expected bash shebang in tccli generated script")
	}
	if !strings.Contains(code, "tccli") {
		t.Errorf("expected tccli command check in script")
	}
}

func TestLuaWorkspaceDogfoodCodegen(t *testing.T) {
	servicePath := filepath.Join("..", "examples", "e2e_workspace", "service.xql")
	mainPath := filepath.Join("..", "examples", "e2e_workspace", "main.xql")

	serviceSrc, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("failed to read service.xql: %v", err)
	}
	mainSrc, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("failed to read main.xql: %v", err)
	}

	serviceNode, err := ast.Parse(serviceSrc)
	if err != nil {
		t.Fatalf("parse service.xql failed: %v", err)
	}
	mainNode, err := ast.Parse(mainSrc)
	if err != nil {
		t.Fatalf("parse main.xql failed: %v", err)
	}

	serviceLua, err := GenerateLua(serviceNode)
	if err != nil {
		t.Fatalf("GenerateLua service.xql failed: %v", err)
	}
	mainLua, err := GenerateLua(mainNode)
	if err != nil {
		t.Fatalf("GenerateLua main.xql failed: %v", err)
	}

	svcCode := string(serviceLua)
	if !strings.Contains(svcCode, "local M = {}") || !strings.Contains(svcCode, "return M") {
		t.Errorf("Lua module service.lua must export table M, got:\n%s", svcCode)
	}
	if !strings.Contains(svcCode, "function M.fetchUsers") {
		t.Errorf("Lua module service.lua must contain function M.fetchUsers, got:\n%s", svcCode)
	}

	mainCode := string(mainLua)
	if !strings.Contains(mainCode, `local service = require("service")`) {
		t.Errorf("Lua main.lua must contain require for service, got:\n%s", mainCode)
	}
	if !strings.Contains(mainCode, `service.fetchUsers`) {
		t.Errorf("Lua main.lua must call service.fetchUsers, got:\n%s", mainCode)
	}
}

func TestGenerateAndroidProject(t *testing.T) {
	root := mustParse(t, addFibMain)
	proj, err := GenerateProject(root, "android")
	if err != nil {
		t.Fatalf("GenerateProject android error: %v", err)
	}
	if proj == nil || len(proj.Files) == 0 {
		t.Fatalf("expected multi-file Android project output, got nil or empty")
	}

	requiredFiles := []string{
		"build.gradle",
		"settings.gradle",
		"app/build.gradle",
		"app/src/main/AndroidManifest.xml",
		"app/src/main/res/layout/activity_main.xml",
		"app/src/main/res/values/strings.xml",
		"app/src/main/java/com/xql/app/MainActivity.kt",
	}

	for _, file := range requiredFiles {
		content, ok := proj.Files[file]
		if !ok || len(content) == 0 {
			t.Errorf("missing or empty file in Android project: %s", file)
		}
	}

	ktCode := string(proj.Files["app/src/main/java/com/xql/app/MainActivity.kt"])
	if !strings.Contains(ktCode, "class MainActivity : AppCompatActivity()") {
		t.Errorf("MainActivity.kt should declare MainActivity, got:\n%s", ktCode)
	}
	if !strings.Contains(ktCode, "fun runXqlApp()") {
		t.Errorf("MainActivity.kt should contain runXqlApp, got:\n%s", ktCode)
	}

	// Verify XML structure fixes and UI layout
	stringsXml := string(proj.Files["app/src/main/res/values/strings.xml"])
	if !strings.Contains(stringsXml, "<resources>") {
		t.Errorf("strings.xml must contain opening <resources> tag, got:\n%s", stringsXml)
	}

	layoutXml := string(proj.Files["app/src/main/res/layout/activity_main.xml"])
	if !strings.Contains(layoutXml, "<ScrollView") {
		t.Errorf("activity_main.xml layout must contain ScrollView wrapper, got:\n%s", layoutXml)
	}
}

func TestRubyCodegenStrategyInspection(t *testing.T) {
	_ = UpdateCodegenStrategy(CodegenStrategyConfig{
		Target:              "ruby",
		PreferComprehension: true,
		BenchmarkScore:      98.5,
		OptimizationFlags:   map[string]string{"strategy_tag": "RubyComprehensionMode"},
	})
	defer func() {
		_ = UpdateCodegenStrategy(CodegenStrategyConfig{Target: "ruby", PreferComprehension: false})
	}()

	root := mustParse(t, addFibMain)
	out, err := GenerateRuby(root)
	if err != nil {
		t.Fatalf("GenerateRuby failed: %v", err)
	}
	code := string(out)
	if !strings.Contains(code, "# Codegen Strategy: RubyComprehensionMode") {
		t.Errorf("expected Ruby codegen to emit strategy header, got:\n%s", code)
	}

	// Test comprehension loop branch
	loopProg := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"body": [
					{
						"kind": "VarDecl",
						"name": "res",
						"value": {"kind": "ArrayLit", "elements": []}
					},
					{
						"kind": "ForStmt",
						"form": "each",
						"var": "x",
						"iterable": {"kind": "Ident", "name": "items"},
						"body": [
							{
								"kind": "ExprStmt",
								"expr": {
									"kind": "CallExpr",
									"callee": "res.push",
									"args": [{"kind": "Ident", "name": "x"}]
								}
							}
						]
					}
				]
			}
		]
	}`

	loopNode := mustParse(t, loopProg)
	compOut, _ := GenerateRuby(loopNode)
	compCode := string(compOut)
	if !strings.Contains(compCode, "res += items.map { |x| x }") {
		t.Errorf("expected Ruby comprehension .map output when PreferComprehension=true, got:\n%s", compCode)
	}

	_ = UpdateCodegenStrategy(CodegenStrategyConfig{Target: "ruby", PreferComprehension: false})
	stdOut, _ := GenerateRuby(loopNode)
	stdCode := string(stdOut)
	if !strings.Contains(stdCode, "items.each do |x|") {
		t.Errorf("expected Ruby standard .each output when PreferComprehension=false, got:\n%s", stdCode)
	}
}

func TestGenerateIOSProject(t *testing.T) {
	root := mustParse(t, addFibMain)
	proj, err := GenerateProject(root, "ios")
	if err != nil {
		t.Fatalf("GenerateProject ios error: %v", err)
	}
	if proj == nil || len(proj.Files) == 0 {
		t.Fatalf("expected multi-file iOS project output, got nil or empty")
	}

	requiredFiles := []string{
		"Package.swift",
		"Sources/XqlApp/main.swift",
		"Sources/XqlApp/App.swift",
		"README.md",
	}

	for _, file := range requiredFiles {
		content, ok := proj.Files[file]
		if !ok || len(content) == 0 {
			t.Errorf("missing or empty file in iOS project: %s", file)
		}
	}

	mainSwift := string(proj.Files["Sources/XqlApp/main.swift"])
	if !strings.Contains(mainSwift, "print(result)") {
		t.Errorf("main.swift should contain top-level execution code, got:\n%s", mainSwift)
	}
}

func TestGeneratePHPWorkspaceDogfood(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{"kind": "ImportDecl", "path": "./models.xql", "as": "models"},
			{"kind": "ImportDecl", "path": "./service.xql", "as": "service"},
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"body": [
					{
						"kind": "VarDecl",
						"name": "config",
						"value": {
							"kind": "StructLit",
							"typeName": "models.Config",
							"fields": [{"name": "path", "value": {"kind": "Literal", "valueType": "String", "value": "./config.json"}}]
						}
					},
					{
						"kind": "VarDecl",
						"name": "res",
						"value": {
							"kind": "CallExpr",
							"callee": "service.fetchUsers",
							"args": [{"kind": "Ident", "name": "config"}]
						}
					},
					{
						"kind": "IfStmt",
						"condition": {"kind": "MemberExpr", "object": {"kind": "Ident", "name": "res"}, "field": "isOk"},
						"then": [
							{
								"kind": "VarDecl",
								"name": "users",
								"value": {"kind": "CallExpr", "callee": "res.unwrap", "args": []}
							}
						],
						"else": [
							{
								"kind": "ExprStmt",
								"expr": {"kind": "CallExpr", "callee": "res.unwrapErr", "args": []}
							}
						]
					}
				]
			}
		]
	}`

	root := mustParse(t, src)
	out, err := GeneratePHP(root)
	if err != nil {
		t.Fatalf("GeneratePHP error: %v", err)
	}
	code := string(out)

	// Verify PHP code syntax fixes
	checks := []string{
		"class Result {",
		"require_once __DIR__ . '/models.php';",
		"require_once __DIR__ . '/service.php';",
		"$config = new Config(",
		"$res = fetchUsers($config);",
		"if ($res->isOk) {",
		"$users = $res->unwrap();",
		"$res->unwrapErr();",
	}

	for _, c := range checks {
		if !strings.Contains(code, c) {
			t.Errorf("PHP output missing expected snippet %q\n---\nFull Generated Code:\n%s", c, code)
		}
	}
}
