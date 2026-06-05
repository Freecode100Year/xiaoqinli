# Xiaoqinli (xql)

**AST-First transpiler for AI agents.** One JSON AST in, idiomatic source code out — 21 languages, single Go binary, zero dependencies.

```
  .xql.json  ──▶  Type Check  ──▶  Effect Inference  ──▶  Capability Enforcement  ──▶  Source Code
  (JSON AST)      Scope Nesting    Transitive Analysis     @grant Verification         (21 targets)
```

AI agents write structured `.xql.json` directly — no parser, no syntax errors. The compiler validates types, effects, and capabilities at compile time, then emits idiomatic code for the chosen target.

## Supported Languages

| Target | Flag | Extension | Verified | Entry Point |
|--------|------|-----------|----------|-------------|
| Go | `go` | `.go` | yes | `func main()` |
| Rust | `rust` | `.rs` | yes | `fn main()` |
| TypeScript | `ts` | `.ts` | yes | `main()` call |
| Python | `py` | `.py` | yes | `if __name__` |
| C++ | `cpp` | `.cpp` | yes | `int main()` |
| C | `c` | `.c` | — | `int main()` |
| Kotlin | `kotlin` | `.kt` | — | `fun main()` |
| Swift | `swift` | `.swift` | — | top-level |
| Java | `java` | `.java` | — | `public static void main` |
| C# | `csharp` | `.cs` | — | `static void Main` |
| Scala | `scala` | `.scala` | — | `object Main { def main }` |
| Haskell | `haskell` | `.hs` | — | `main :: IO ()` |
| Dart | `dart` | `.dart` | — | `void main()` |
| Lua | `lua` | `.lua` | — | top-level |
| Ruby | `ruby` | `.rb` | — | top-level |
| PHP | `php` | `.php` | — | top-level |
| Zig | `zig` | `.zig` | — | `pub fn main()` |
| Nim | `nim` | `.nim` | — | top-level |
| Julia | `julia` | `.jl` | — | `main()` call |
| MQL4 | `mql4` | `.mq4` | no* | `void OnStart()` |
| MQL5 | `mql5` | `.mq5` | no* | `void OnStart()` |

**Verified** = round-trip tested in CI (generate → compile → run → compare stdout).

> \* **MQL4/MQL5:** Script mode only. Generates language skeleton with `OnStart` entry and `Print` output — no trading API (`OrderSend`, `CTrade`, `OnTick`, `OnCalculate`). Cannot be CI-verified (MetaEditor is closed-source). Map, Option, and Result types emit `XQL_E403`.

**Backend notes:**
- **C** — Uses `long` for Int, `_xql_strcat` helper for string concatenation (`malloc`+`memcpy`). Rejects Option/Map/Result types and for-each loops (`XQL_E402`).
- **Haskell** — Pure functions use expression-based if/then/else; IO functions use `do` notation. Rejects mutable patterns: `AssignStmt`, `WhileStmt`, `BreakStmt`, `ContinueStmt` (`XQL_E401`).
- **Scala** — Wraps all code in `object Main`. Uses `val`/`var` based on reassignment analysis. `Long` for Int with `L` suffixes.

## Quick Start

```bash
go build -o xql .

# Validate AST without generating code
./xql validate --file examples/hello.xql.json

# Compile to any target
./xql compile --file examples/hello.xql.json --target go
./xql compile --file examples/hello.xql.json --target cpp    --out main.cpp
./xql compile --file examples/hello.xql.json --target rust   --out main.rs
./xql compile --file examples/hello.xql.json --target py     --out main.py
./xql compile --file examples/hello.xql.json --target mql5   --out script.mq5
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

## Static Analysis Pipeline

All checks run before code generation. If any check fails, no code is emitted.

```
  Parse JSON  ──▶  Type Check  ──▶  Effect Check  ──▶  Capability Check  ──▶  Codegen
  (XQL_E1xx)      (XQL_E2xx)      (XQL_E2xx)         (XQL_E3xx)             (XQL_E4xx)
```

| Phase | What it validates |
|-------|-------------------|
| **Type check** | Variable types, function signatures, return types, operator compatibility, array element types, struct field types, index expression types |
| **Effect inference** | Side effects (`network`/`filesystem`/`state`), purity violations, transitive propagation through call chains |
| **Capability check** | `@grant` enforcement — callee capabilities must be subset of caller's, checked through all control flow paths including for-loops |

### Type Inference

The type checker tracks full generic type information through expressions:

- `Array<Int>` indexing returns `Int` (not unknown)
- `for-each` loop variable inherits the array's element type
- Struct field access returns the field's declared type
- Binary operators propagate types through the expression tree

### Scoped Type Checking

Variables declared inside `if`/`while`/`for` blocks do not leak into the enclosing scope. Each block creates a child scope that inherits parent bindings but isolates its own declarations.

## Type System

| XQL Kind | Go | Rust | C | C++ | Scala | Haskell | Python | MQL4/5 |
|----------|-----|------|---|-----|-------|---------|--------|--------|
| `Int` | `int` | `i64` | `long` | `long` | `Long` | `Int` | `int` | `long` |
| `Float` | `float64` | `f64` | `double` | `double` | `Double` | `Double` | `float` | `double` |
| `String` | `string` | `String` | `const char*` | `std::string` | `String` | `String` | `str` | `string` |
| `Bool` | `bool` | `bool` | `int` | `bool` | `Boolean` | `Bool` | `bool` | `bool` |
| `Void` | — | — | `void` | `void` | `Unit` | `()` | `None` | `void` |
| `Array<T>` | `[]T` | `Vec<T>` | `T[]` | `std::vector<T>` | `Array[T]` | `[T]` | `list[T]` | `T[]` |
| `Option<T>` | `*T` | `Option<T>` | E402 | `std::optional<T>` | `Option[T]` | `Maybe T` | `Optional[T]` | E403 |
| `Map<K,V>` | `map[K]V` | `HashMap<K,V>` | E402 | `std::unordered_map<K,V>` | `Map[K,V]` | — | `dict[K,V]` | E403 |
| `Result<T>` | `(T, error)` | `Result<T,E>` | E402 | E402 | `Either[Throwable,T]` | — | E402 | E403 |

## .xql.json Node Reference

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
| `AssignStmt` | `target`, `value` | Assignment (target: string, `IndexExpr`, or `MemberExpr`) |
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
| `MatchExpr` | `value`, `arms[]` | Pattern match / switch expression |

### Built-in Functions

| Function | Effect | Maps to |
|----------|--------|---------|
| `println` | state | `fmt.Println` / `println!` / `std::cout << ... << std::endl` / `print()` / `Print()` |
| `printf` | state | `fmt.Printf` / `print!` / `std::cout << ...` / `Print()` |
| `sprintf` | pure | `fmt.Sprintf` / `format!` / `std::to_string` |

## ForStmt

**Range form** — iterate from start to end (exclusive):

```json
{
  "kind": "ForStmt", "form": "range", "var": "i",
  "start": { "kind": "Literal", "valueType": "Int", "value": 0 },
  "end": { "kind": "Literal", "valueType": "Int", "value": 10 },
  "body": [...]
}
```

Generates: `for i := 0; i < 10; i++` (Go) / `for i in 0..10` (Rust) / `for (long i = 0; i < 10; i++)` (C++/MQL)

**Each form** — iterate over array elements:

```json
{
  "kind": "ForStmt", "form": "each", "var": "item",
  "iterable": { "kind": "Ident", "name": "items" },
  "body": [...]
}
```

Generates: `for _, item := range items` (Go) / `for item in &items` (Rust) / `for (const auto& item : items)` (C++)

> MQL4/MQL5 does not support for-each — use range form with index access instead.

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
| `targets` | — | List all 21 supported target languages |

## Error Codes

| Range | Category | Phase | Example |
|-------|----------|-------|---------|
| `XQL_E1xx` | Parse / AST structure | Input | Missing `kind` field, invalid JSON |
| `XQL_E2xx` | Type / effect violations | Static check | Return type mismatch, purity violation |
| `XQL_E3xx` | Capability violations | Static check | Missing `@grant` for callee |
| `XQL_E4xx` | Codegen errors | Code generation | Unsupported node, unsupported type for target |

### Target-Specific Rejections

| Error | Targets | Cause |
|-------|---------|-------|
| `XQL_E402` | C++, TS, Dart, Nim, Julia, Lua, Ruby | `Result<T>` type not supported |
| `XQL_E403` | MQL4, MQL5 | `Map`, `Option`, `Result` type or `for-each` loop not supported |

## Project Structure

```
xiaoqinli/
  main.go                    CLI entry + version (2.5.0)
  ast/
    nodes.go                 AST node definitions + JSON parser
    hash.go                  Content-addressable hashing
  check/
    types.go                 Type checker + scoped inference + effect system
    capability.go            Capability checker (@grant enforcement)
    check.go                 RunAll orchestrator
  codegen/
    golang.go                Go backend
    rust.go                  Rust backend
    typescript.go            TypeScript backend
    python.go                Python backend
    cpp.go                   C++17 backend (auto #include)
    kotlin.go                Kotlin backend
    swift.go                 Swift backend
    java.go                  Java backend
    csharp.go                C# backend
    dart.go                  Dart backend
    lua.go                   Lua backend
    ruby.go                  Ruby backend
    php.go                   PHP backend
    zig.go                   Zig backend
    nim.go                   Nim backend
    julia.go                 Julia backend
    c.go                     C99 backend (printf, _xql_strcat)
    scala.go                 Scala backend (object Main wrapper)
    haskell.go               Haskell backend (pure + IO, do notation)
    mql.go                   MQL4/MQL5 shared backend (script mode)
    util.go                  Generate() dispatcher + shared utilities
    codegen_test.go          124 tests across all 21 backends
    roundtrip_test.go        Compile-and-run verification (Go, Rust, Python, TS, C++)
  server/
    mcp.go                   MCP server (stdio + streamable HTTP)
    rest.go                  REST API server
    skills.go                Skills dispatcher
  vfs/
    workspace.go             In-memory virtual filesystem
  skills/
    *.md                     Embedded skill documents
  examples/
    hello.xql.json           Hello world (greet + println)
    example.xql.json         Fibonacci + arithmetic
    clock.xql.json           System clock with live output
    loop.xql.json            For-loop with array indexing
    struct.xql.json          Struct declaration and field access
    collections.xql.json     Array literals and indexing
```

## Tests

```bash
go test ./... -v              # run all tests (124 tests)
go build -o xql . && go test ./... -v   # build + test
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
- **Deterministic** — Same AST always produces same output, no randomness
- **Secure by default** — All validation at compile time; `os/exec` never runs user code
- **Two-layer pipeline** — Check (types + effects + capabilities) then codegen, single-pass AST traversal, no IR

## License

MIT
