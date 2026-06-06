# Xiaoqinli (xql)

**AST-First transpiler for AI agents.** One JSON AST in, idiomatic source code out — 33 languages, single Go binary, zero dependencies.

```
  .xql.json  ──▶  Type Check  ──▶  Effect Inference  ──▶  Capability Enforcement  ──▶  Source Code
  (JSON AST)      Scope Nesting    Transitive Analysis     @grant Verification         (33 targets)
```

AI agents write structured `.xql.json` directly — no parser, no syntax errors. The compiler validates types, effects, and capabilities at compile time, then emits idiomatic code for the chosen target.

## Supported Languages (33)

**Systems** — `go` `rust` `c` `cpp` `zig` `d` `v` `nim`
**JVM/CLR** — `java` `kotlin` `scala` `csharp` `dart`
**Scripting** — `py` `ts` `ruby` `lua` `php` `perl` `julia` `crystal` `awk`
**Functional** — `haskell`
**Shell** — `bash` `powershell` `tcl`
**Legacy/Niche** — `ada` `fortran` `pascal` `objc` `mql4` `mql5`

| # | Target | Flag | Ext | IfExpr | Lambda |
|---|--------|------|-----|--------|--------|
| 1 | Go | `go` | `.go` | IIFE | yes |
| 2 | Rust | `rust` | `.rs` | if-expr | closure |
| 3 | TypeScript | `ts` | `.ts` | ternary | arrow fn |
| 4 | Python | `py` | `.py` | inline if | lambda |
| 5 | C++ | `cpp` | `.cpp` | ternary | `[](){}` |
| 6 | C | `c` | `.c` | ternary | — |
| 7 | Kotlin | `kotlin` | `.kt` | if-expr | `{}` |
| 8 | Swift | `swift` | `.swift` | ternary | closure |
| 9 | Java | `java` | `.java` | ternary | `->` |
| 10 | C# | `csharp` | `.cs` | ternary | `=>` |
| 11 | Scala | `scala` | `.scala` | if-expr | `=>` |
| 12 | Haskell | `haskell` | `.hs` | if-then-else | `\->` |
| 13 | Dart | `dart` | `.dart` | ternary | arrow/block |
| 14 | Lua | `lua` | `.lua` | and/or | `function() end` |
| 15 | Ruby | `ruby` | `.rb` | ternary | `lambda {}` |
| 16 | PHP | `php` | `.php` | ternary | `function(){}` |
| 17 | Zig | `zig` | `.zig` | if-expr | — |
| 18 | Nim | `nim` | `.nim` | if-expr | `proc()` |
| 19 | Julia | `julia` | `.jl` | ternary | `->` |
| 20 | MQL4 | `mql4` | `.mq4` | ternary | — |
| 21 | MQL5 | `mql5` | `.mq5` | ternary | — |
| 22 | Ada | `ada` | `.adb` | if-expr | — |
| 23 | AWK | `awk` | `.awk` | ternary | — |
| 24 | Bash | `bash` | `.sh` | — | — |
| 25 | Crystal | `crystal` | `.cr` | ternary | `->(){}` |
| 26 | D | `d` | `.d` | ternary | `(){}` |
| 27 | Fortran | `fortran` | `.f90` | `merge()` | — |
| 28 | Objective-C | `objc` | `.m` | ternary | block `^(){}` |
| 29 | Pascal | `pascal` | `.pas` | — | — |
| 30 | Perl | `perl` | `.pl` | ternary | `sub {}` |
| 31 | PowerShell | `powershell` | `.ps1` | `$(if)` | scriptblock |
| 32 | Tcl | `tcl` | `.tcl` | `[expr]` | — |
| 33 | V | `v` | `.v` | if-expr | `fn(){}` |

> **—** = returns compile error (language limitation). All 33 targets support the full statement set (VarDecl, IfStmt, WhileStmt, ForStmt, etc.).

### Backend Notes

- **C** — `long` for Int, `_xql_strcat` helper for string concatenation. Rejects Lambda, Option, Map, Result.
- **Haskell** — Pure functions use expression-based if/then/else; IO functions use `do` notation. Rejects mutable patterns (AssignStmt, WhileStmt).
- **Scala** — Wraps code in `object Main`. Uses `val`/`var` based on reassignment analysis.
- **Go** — IfExpr uses IIFE pattern since Go lacks ternary expressions.
- **Fortran** — IfExpr uses `merge()` intrinsic. Rejects Lambda.
- **Bash** — Rejects both IfExpr and Lambda (shell limitations in expression context).
- **Pascal** — Rejects both IfExpr and Lambda (language limitations).
- **MQL4/MQL5** — Script mode only (`OnStart` entry). Rejects Lambda, Map, Option, Result, for-each.
- **D** — Uses `~` for string concatenation, `long` for Int.
- **Objective-C** — Uses Foundation types (`NSString*`), `@autoreleasepool` in main, block syntax for Lambda.

## Quick Start

```bash
go build -o xql .

# Validate AST without generating code
./xql validate --file examples/hello.xql.json

# Compile to any target
./xql compile --file examples/hello.xql.json --target go
./xql compile --file examples/hello.xql.json --target rust   --out main.rs
./xql compile --file examples/hello.xql.json --target py     --out main.py
./xql compile --file examples/hello.xql.json --target ada    --out main.adb
./xql compile --file examples/hello.xql.json --target crystal --out main.cr

# List all supported targets
./xql targets
```

## One AST, Many Languages

Write one `.xql.json`, compile to any target:

```json
{
  "kind": "Program",
  "declarations": [
    {
      "kind": "FunctionDecl",
      "name": "greet",
      "params": [{ "name": "name", "type": { "kind": "String" } }],
      "returnType": { "kind": "String" },
      "effects": ["pure"],
      "grant": [],
      "body": [{
        "kind": "ReturnStmt",
        "value": {
          "kind": "BinaryExpr", "op": "+",
          "left": { "kind": "Literal", "valueType": "String", "value": "Hello, " },
          "right": { "kind": "Ident", "name": "name" }
        }
      }]
    },
    {
      "kind": "FunctionDecl",
      "name": "main",
      "params": [],
      "returnType": { "kind": "Void" },
      "effects": [],
      "grant": [],
      "body": [{
        "kind": "ExprStmt",
        "expr": {
          "kind": "CallExpr", "callee": "println",
          "args": [{
            "kind": "CallExpr", "callee": "greet",
            "args": [{ "kind": "Literal", "valueType": "String", "value": "World" }]
          }]
        }
      }]
    }
  ]
}
```

<details>
<summary><strong>Go</strong></summary>

```go
package main

import "fmt"

func greet(name string) string {
    return "Hello, " + name
}

func main() {
    fmt.Println(greet("World"))
}
```
</details>

<details>
<summary><strong>Rust</strong></summary>

```rust
fn greet(name: &str) -> String {
    return ("Hello, ".to_string() + name);
}

fn main() {
    println!("{}", greet("World"));
}
```
</details>

<details>
<summary><strong>Python</strong></summary>

```python
def greet(name: str) -> str:
    return ("Hello, " + name)

def main() -> None:
    print(greet("World"))

if __name__ == "__main__":
    main()
```
</details>

<details>
<summary><strong>C++</strong></summary>

```cpp
#include <iostream>
#include <string>

std::string greet(std::string name) {
    return ("Hello, " + name);
}

int main() {
    std::cout << greet("World") << std::endl;
    return 0;
}
```
</details>

<details>
<summary><strong>Java</strong></summary>

```java
public class Main {
    static String greet(String name) {
        return ("Hello, " + name);
    }

    public static void main(String[] args) {
        System.out.println(greet("World"));
    }
}
```
</details>

<details>
<summary><strong>Kotlin</strong></summary>

```kotlin
fun greet(name: String): String {
    return ("Hello, " + name)
}

fun main() {
    println(greet("World"))
}
```
</details>

<details>
<summary><strong>Swift</strong></summary>

```swift
func greet(_ name: String) -> String {
    return ("Hello, " + name)
}

print(greet("World"))
```
</details>

<details>
<summary><strong>C</strong></summary>

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static char* _xql_strcat(const char* a, const char* b) {
    size_t la = strlen(a), lb = strlen(b);
    char* r = (char*)malloc(la + lb + 1);
    memcpy(r, a, la);
    memcpy(r + la, b, lb + 1);
    return r;
}

const char* greet(const char* name) {
    return _xql_strcat("Hello, ", name);
}

int main() {
    printf("%s\n", greet("World"));
    return 0;
}
```
</details>

<details>
<summary><strong>Scala</strong></summary>

```scala
object Main {
    def greet(name: String): String = {
        return ("Hello, " + name)
    }

    def main(args: Array[String]): Unit = {
        println(greet("World"))
    }
}
```
</details>

<details>
<summary><strong>Haskell</strong></summary>

```haskell
module Main where

greet :: String -> String
greet name = ("Hello, " ++ name)

main :: IO ()
main = do
    putStrLn (greet "World")
```
</details>

<details>
<summary><strong>Ada</strong></summary>

```ada
with Ada.Text_IO; use Ada.Text_IO;
with Ada.Integer_Text_IO; use Ada.Integer_Text_IO;

function greet(name : String) return String is
begin
    return ("Hello, " & name);
end greet;

procedure Main is
begin
    Put_Line(greet("World"));
end Main;
```
</details>

<details>
<summary><strong>Crystal</strong></summary>

```crystal
def greet(name : String) : String
  return ("Hello, " + name)
end

puts(greet("World"))
```
</details>

<details>
<summary><strong>D</strong></summary>

```d
import std.stdio;

string greet(string name) {
    return ("Hello, " ~ name);
}

void main() {
    writeln(greet("World"));
}
```
</details>

<details>
<summary><strong>Objective-C</strong></summary>

```objc
#import <Foundation/Foundation.h>

NSString* greet(NSString* name) {
    return [@"Hello, " stringByAppendingString:name];
}

int main() {
    @autoreleasepool {
        NSLog(@"%@", greet(@"World"));
    }
    return 0;
}
```
</details>

<details>
<summary><strong>Perl</strong></summary>

```perl
use strict;
use warnings;

sub greet {
    my ($name) = @_;
    return ("Hello, " . $name);
}

print(greet("World") . "\n");
```
</details>

<details>
<summary><strong>V</strong></summary>

```v
fn greet(name string) string {
    return ('Hello, ' + name)
}

fn main() {
    println(greet('World'))
}
```
</details>

<details>
<summary><strong>MQL5</strong></summary>

```mql5
#property strict

string greet(string name) {
    return ("Hello, " + name);
}

void OnStart() {
    Print(greet("World"));
}
```
</details>

## AST Node Reference

### Program

```json
{ "kind": "Program", "declarations": [...] }
```

### Declarations

| Node | Fields | Description |
|------|--------|-------------|
| `FunctionDecl` | `name`, `params[]`, `returnType`, `effects[]`, `grant[]`, `body[]` | Function definition |
| `StructDecl` | `name`, `fields[]` | Struct type definition |
| `EnumDecl` | `name`, `variants[]` | Enum type definition |

### Statements

| Node | Fields | Description |
|------|--------|-------------|
| `VarDecl` | `name`, `type`, `value` | Variable declaration |
| `AssignStmt` | `target`, `value` | Assignment |
| `ReturnStmt` | `value` (optional) | Return from function |
| `IfStmt` | `cond`, `then[]`, `else[]` | Conditional branch |
| `WhileStmt` | `cond`, `body[]` | While loop |
| `ForStmt` | `form`, `var`, `start`/`end` or `iterable`, `body[]` | For loop (range or each) |
| `BreakStmt` | — | Exit innermost loop |
| `ContinueStmt` | — | Skip to next iteration |
| `ExprStmt` | `expr` | Expression as statement |

### Expressions

| Node | Fields | Description |
|------|--------|-------------|
| `Literal` | `valueType`, `value` | Int, Float, String, Bool literal |
| `Ident` | `name` | Variable reference |
| `BinaryExpr` | `op`, `left`, `right` | `+ - * / % == != < > <= >= && \|\|` |
| `UnaryExpr` | `op`, `operand` | `- !` |
| `CallExpr` | `callee`, `args[]` | Function call |
| `MemberExpr` | `object`, `field` | Field access (`obj.x`) |
| `StructLit` | `typeName`, `fields[]` | Struct construction |
| `ArrayLit` | `elemType`, `elements[]` | Array literal |
| `IndexExpr` | `target`, `index` | Index access (`arr[i]`) |
| `MatchExpr` | `value`, `arms[]` | Pattern match / switch |
| `IfExpr` | `cond`, `then`, `else` | Ternary / conditional expression |
| `Lambda` | `params[]`, `returnType`, `body[]` | Anonymous function / closure |

### IfExpr (Ternary Expression)

```json
{
  "kind": "IfExpr",
  "cond": { "kind": "BinaryExpr", "op": ">", "left": ..., "right": ... },
  "then": { "kind": "Literal", "valueType": "String", "value": "big" },
  "else": { "kind": "Literal", "valueType": "String", "value": "small" }
}
```

| Language | Output |
|----------|--------|
| Go | `func() interface{} { if x > 5 { return "big" }; return "small" }()` |
| Rust | `if (x > 5) { "big" } else { "small" }` |
| Python | `("big" if (x > 5) else "small")` |
| C / C++ / Java / C# | `((x > 5) ? "big" : "small")` |
| Haskell | `(if (x > 5) then "big" else "small")` |
| Ada | `(if (x > 5) then "big" else "small")` |
| Fortran | `merge("big", "small", (x > 5))` |
| V | `if (x > 5) { 'big' } else { 'small' }` |
| PowerShell | `$(if (($x -gt 5)) { "big" } else { "small" })` |

### Lambda (Anonymous Function)

```json
{
  "kind": "Lambda",
  "params": [{ "name": "x", "type": { "kind": "Int" } }],
  "returnType": { "kind": "Int" },
  "body": [{ "kind": "ReturnStmt", "value": ... }]
}
```

| Language | Output |
|----------|--------|
| Go | `func(x int) int { return ... }` |
| Rust | `\|x: i64\| -> i64 { return ...; }` |
| Python | `lambda x: ...` |
| TypeScript | `(x: number): number => { return ...; }` |
| C++ | `[](long x) -> long { return ...; }` |
| Java | `(long x) -> { return ...; }` |
| Kotlin | `{ x: Long -> ... }` |
| Swift | `{ (x: Int) -> Int in return ... }` |
| Ruby | `lambda { \|x\| return ... }` |
| Lua | `function(x) return ... end` |
| Crystal | `->(x : Int64) { return ... }` |
| Objective-C | `^(long x) { return ...; }` |

> Lambda is not supported in: C, Zig, MQL4/5, Ada, AWK, Bash, Fortran, Pascal, Tcl.

### StructDecl + StructLit

```json
{
  "kind": "StructDecl",
  "name": "Point",
  "fields": [
    { "name": "x", "type": { "kind": "Int" } },
    { "name": "y", "type": { "kind": "Int" } }
  ]
}
```

```json
{
  "kind": "StructLit",
  "typeName": "Point",
  "fields": [
    { "name": "x", "value": { "kind": "Literal", "valueType": "Int", "value": 10 } },
    { "name": "y", "value": { "kind": "Literal", "valueType": "Int", "value": 20 } }
  ]
}
```

### EnumDecl + MatchExpr

```json
{
  "kind": "EnumDecl",
  "name": "Color",
  "variants": ["Red", "Green", "Blue"]
}
```

```json
{
  "kind": "MatchExpr",
  "value": { "kind": "Ident", "name": "c" },
  "arms": [
    {
      "pattern": { "kind": "Ident", "name": "ColorRed" },
      "body": [{ "kind": "ExprStmt", "expr": { "kind": "CallExpr", "callee": "println", "args": [{ "kind": "Literal", "valueType": "String", "value": "red" }] } }]
    },
    {
      "pattern": { "kind": "Ident", "name": "_" },
      "body": [{ "kind": "ExprStmt", "expr": { "kind": "CallExpr", "callee": "println", "args": [{ "kind": "Literal", "valueType": "String", "value": "other" }] } }]
    }
  ]
}
```

### Built-in Functions

| Function | Effect | Description |
|----------|--------|-------------|
| `println` | state | Print with newline |
| `printf` | state | Formatted print |
| `sprintf` | pure | Formatted string (returns String) |

### ForStmt

**Range form** — iterate from start to end (exclusive):

```json
{
  "kind": "ForStmt", "form": "range", "var": "i",
  "start": { "kind": "Literal", "valueType": "Int", "value": 0 },
  "end": { "kind": "Literal", "valueType": "Int", "value": 10 },
  "body": [...]
}
```

**Each form** — iterate over array elements:

```json
{
  "kind": "ForStmt", "form": "each", "var": "item",
  "iterable": { "kind": "Ident", "name": "items" },
  "body": [...]
}
```

## Static Analysis Pipeline

All checks run before code generation. If any check fails, no code is emitted.

```
  Parse JSON  ──▶  Type Check  ──▶  Effect Check  ──▶  Capability Check  ──▶  Codegen
  (XQL_E1xx)      (XQL_E2xx)      (XQL_E2xx)         (XQL_E3xx)             (XQL_E4xx)
```

| Phase | What it validates |
|-------|-------------------|
| **Type check** | Variable types, function signatures, return types, operator compatibility, array element types, struct field types, index types, IfExpr branch types |
| **Effect inference** | Side effects (`network`/`filesystem`/`state`), purity violations, transitive propagation through call chains, Lambda body effects |
| **Capability check** | `@grant` enforcement — callee capabilities must be subset of caller's |

### Type Inference

- `Array<Int>` indexing returns `Int` (not unknown)
- `for-each` loop variable inherits the array's element type
- Struct field access returns the field's declared type
- `IfExpr` branches must have the same type
- Binary operators propagate types through the expression tree

### Scoped Type Checking

Variables declared inside `if`/`while`/`for` blocks do not leak into the enclosing scope. Each block creates a child scope that inherits parent bindings but isolates its own declarations.

## Type System

| XQL Kind | Go | Rust | C | C++ | Python | Java | Haskell |
|----------|-----|------|---|-----|--------|------|---------|
| `Int` | `int` | `i64` | `long` | `long` | `int` | `long` | `Int` |
| `Float` | `float64` | `f64` | `double` | `double` | `float` | `double` | `Double` |
| `String` | `string` | `String` | `const char*` | `std::string` | `str` | `String` | `String` |
| `Bool` | `bool` | `bool` | `int` | `bool` | `bool` | `boolean` | `Bool` |
| `Void` | — | — | `void` | `void` | `None` | `void` | `()` |
| `Array<T>` | `[]T` | `Vec<T>` | `T[]` | `std::vector<T>` | `list[T]` | `T[]` | `[T]` |
| `Option<T>` | `*T` | `Option<T>` | E402 | `std::optional<T>` | `Optional[T]` | `T` | `Maybe T` |
| `Result<T>` | `(T, error)` | `Result<T,E>` | E402 | E402 | E402 | E402 | — |

## MCP Server

Xiaoqinli runs as a local MCP server for Claude Code, Cursor, and other MCP-compatible editors.

```bash
./xql stdio                      # stdio mode (recommended)
./xql http :8080                 # streamable HTTP mode
./xql http :8080 --mode rest     # REST API mode
```

| Tool | Args | Description |
|------|------|-------------|
| `compile` | `source`, `target` | Compile `.xql.json` AST to target language |
| `validate` | `source` | Validate AST without generating code |
| `targets` | — | List all 33 supported target languages |

### Claude Code Integration

Add to `~/.mcp.json`:

```json
{
  "mcpServers": {
    "xiaoqinli": {
      "command": "/path/to/xql",
      "args": ["stdio"]
    }
  }
}
```

Then Claude Code can directly call `compile` and `validate` as MCP tools:

```
> Use xiaoqinli to compile this AST to Rust
> Validate my .xql.json file
```

### Cursor / VS Code Integration

Add to `.cursor/mcp.json` or VS Code MCP settings:

```json
{
  "mcpServers": {
    "xiaoqinli": {
      "command": "/path/to/xql",
      "args": ["stdio"]
    }
  }
}
```

## Error Codes

| Range | Category | Example |
|-------|----------|---------|
| `XQL_E1xx` | Parse / AST structure | Missing `kind` field, invalid JSON |
| `XQL_E2xx` | Type / effect violations | Return type mismatch, purity violation, IfExpr branch type mismatch |
| `XQL_E3xx` | Capability violations | Missing `@grant` for callee |
| `XQL_E4xx` | Codegen errors | Unsupported node for target (e.g. Lambda in C) |

## Project Structure

```
xiaoqinli/
  main.go                    CLI entry + version (3.0.0)
  ast/
    nodes.go                 24 AST node types + JSON parser
  check/
    types.go                 Type checker + effect system
    capability.go            @grant enforcement
    check.go                 RunAll orchestrator
  codegen/
    golang.go   rust.go      typescript.go   python.go
    cpp.go      c.go         kotlin.go       swift.go
    java.go     csharp.go    scala.go        haskell.go
    dart.go     lua.go       ruby.go         php.go
    zig.go      nim.go       julia.go        mql.go
    ada.go      awk.go       bash.go         crystal.go
    d.go        fortran.go   objc.go         pascal.go
    perl.go     powershell.go  tcl.go        v.go
    util.go                  Generate() dispatcher + shared utilities
    codegen_test.go          222 tests across all 33 backends
    roundtrip_test.go        Compile-and-run verification
  server/
    mcp.go                   MCP server (stdio + HTTP)
    rest.go                  REST API server
  examples/
    hello.xql.json           Hello world
    example.xql.json         Fibonacci + arithmetic
    clock.xql.json           System clock
    loop.xql.json            For-loop + array indexing
    struct.xql.json          Struct declaration + field access
    collections.xql.json     Array literals + indexing
    lambda_ifexpr.xql.json   IfExpr + Lambda demo
```

## Tests

```bash
go test ./...                    # run all 222 tests
go build -o xql . && go test ./... -v   # build + test verbose
```

Round-trip tests automatically skip when the toolchain is not installed:

| Target | Toolchain | Round-trip |
|--------|-----------|------------|
| Go | `go` | compile + run |
| Rust | `rustc` | compile + run |
| Python | `python` | interpret |
| TypeScript | `node` | strip types + run |
| C++ | `g++` | compile (-std=c++17) + run |

## Design Principles

- **Zero dependencies** — Single language (Go), single binary, no third-party imports
- **Deterministic** — Same AST always produces same output
- **Secure by default** — All validation at compile time; `os/exec` never runs user code
- **Two-layer pipeline** — Check (types + effects + capabilities) then codegen, single-pass AST traversal, no IR

## Version History

| Version | Languages | AST Nodes | Highlights |
|---------|-----------|-----------|------------|
| **3.0.0** | 33 | 24 | +12 backends (Ada/AWK/Bash/Crystal/D/Fortran/ObjC/Pascal/Perl/PowerShell/Tcl/V), IfExpr, Lambda |
| 2.5.0 | 21 | 22 | +C/Scala/Haskell, EnumDecl, MatchExpr |
| 2.2.0 | 18 | 20 | +C++/MQL4/MQL5, ArrayLit, IndexExpr, StructDecl |
| 2.0.0 | 15 | 16 | +Dart/Lua/Ruby/PHP/Zig/Nim/Julia, ForStmt |
| 1.0.0 | 8 | 12 | Initial release: Go/Rust/TS/Python/Kotlin/Swift/Java/C# |

## License

MIT
