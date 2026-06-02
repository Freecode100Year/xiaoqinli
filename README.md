# Xiaoqinli (xql)

**AST-First transpiler for AI agents.** Single Go binary, zero dependencies.

AI agents write structured `.xql.json` (AST) directly — no text parser needed, no syntax errors possible. The compiler validates types, effects, and capabilities at compile time, then emits idiomatic source code in 15 languages.

```
.xql.json ──▶ [ Type Check ──▶ Effect Check ──▶ Capability Check ] ──▶ Code Generation
                              all checks pass?                          ▼
                              no → error + halt                   Go / Rust / TS / Kotlin / Swift
                                                                  Python / Java / C# / Dart / Lua
                                                                  Ruby / PHP / Zig / Nim / Julia
```

## Quick Start

```bash
go build -o xql .

./xql validate --file examples/hello.xql.json
./xql compile  --file examples/hello.xql.json --target go
./xql compile  --file examples/hello.xql.json --target rust   --out main.rs
./xql compile  --file examples/hello.xql.json --target ts     --out main.ts
./xql compile  --file examples/hello.xql.json --target kotlin --out main.kt
./xql compile  --file examples/hello.xql.json --target swift  --out main.swift
./xql compile  --file examples/hello.xql.json --target py     --out main.py
./xql compile  --file examples/hello.xql.json --target java   --out Main.java
./xql compile  --file examples/hello.xql.json --target csharp --out Program.cs
./xql compile  --file examples/hello.xql.json --target dart   --out main.dart
./xql compile  --file examples/hello.xql.json --target lua    --out main.lua
./xql compile  --file examples/hello.xql.json --target ruby   --out main.rb
./xql compile  --file examples/hello.xql.json --target php    --out main.php
./xql compile  --file examples/hello.xql.json --target zig    --out main.zig
./xql compile  --file examples/hello.xql.json --target nim    --out main.nim
./xql compile  --file examples/hello.xql.json --target julia  --out main.jl
```

## One AST, Fifteen Languages

Write one `.xql.json`, compile to any target:

**hello.xql.json**
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
<summary><strong>Go</strong> — <code>xql compile --target go</code></summary>

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
<summary><strong>Rust</strong> — <code>xql compile --target rust</code></summary>

```rust
fn greet(name: String) -> String {
    return (String::from("Hello, ") + &name);
}

fn main() {
    println!("{}", greet(String::from("World")));
}
```
</details>

<details>
<summary><strong>Java</strong> — <code>xql compile --target java</code></summary>

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
<summary><strong>C#</strong> — <code>xql compile --target csharp</code></summary>

```csharp
using System;

class Program {
    static string greet(string name) {
        return ("Hello, " + name);
    }

    static void Main() {
        Console.WriteLine(greet("World"));
    }
}
```
</details>

<details>
<summary><strong>Python</strong> — <code>xql compile --target py</code></summary>

```python
def greet(name: str) -> str:
    return ("Hello, " + name)


def main() -> None:
    print(greet("World"))


if __name__ == "__main__":
    main()
```
</details>

Also supports **TypeScript** (`ts`), **Kotlin** (`kotlin`), **Swift** (`swift`), **Dart** (`dart`), **Lua** (`lua`), **Ruby** (`ruby`), **PHP** (`php`), **Zig** (`zig`), **Nim** (`nim`), **Julia** (`julia`).

## Three Static Checks

All checks run at compile time. Code generation only proceeds if all three pass.

| Check | What it does | Error codes |
|-------|-------------|-------------|
| **Type check** | Validates variable types, function signatures, return types, operator compatibility | `XQL_E2xx` |
| **Effect inference** | Infers side effects (network / filesystem / state), catches purity violations | `XQL_E2xx` |
| **Capability check** | Enforces `@grant` — callee capabilities must be subset of caller's | `XQL_E3xx` |

```json
"effects": ["pure"],
"grant": ["io", "network"]
```

A `pure` function cannot call `println` or any function with side effects. If function A calls function B, A's `grant` must cover all of B's `grant`.

## Type System

| Kind | Go | Rust | TS | Kotlin | Swift | Python | Java | C# | Dart | Lua | Ruby | PHP | Zig | Nim | Julia |
|------|-----|------|-----|--------|-------|--------|------|-----|------|-----|------|-----|-----|-----|-------|
| `Int` | `int` | `i64` | `number` | `Long` | `Int` | `int` | `long` | `long` | `int` | — | — | `int` | `i64` | `int64` | `Int64` |
| `Float` | `float64` | `f64` | `number` | `Double` | `Double` | `float` | `double` | `double` | `double` | — | — | `float` | `f64` | `float64` | `Float64` |
| `String` | `string` | `String` | `string` | `String` | `String` | `str` | `String` | `string` | `String` | — | — | `string` | `[]const u8` | `string` | `String` |
| `Bool` | `bool` | `bool` | `boolean` | `Boolean` | `Bool` | `bool` | `boolean` | `bool` | `bool` | — | — | `bool` | `bool` | `bool` | `Bool` |
| `Void` | *(none)* | *(none)* | `void` | `Unit` | *(none)* | `None` | `void` | `void` | `void` | — | — | `void` | `void` | — | `Nothing` |
| `Array` | `[]T` | `Vec<T>` | `T[]` | `List<T>` | `[T]` | `list[T]` | `List<T>` | `List<T>` | `List<T>` | — | — | `array` | — | `seq[T]` | `Vector{T}` |
| `Option` | `*T` | `Option<T>` | `T\|null` | `T?` | `T?` | `Optional[T]` | `T` | `T?` | `T?` | — | — | `?T` | `?T` | `Option[T]` | `Union{T,Nothing}` |

## .xql.json Node Reference

**Declarations:**
- `Program` — `{ "kind": "Program", "declarations": [...] }`
- `FunctionDecl` — `name`, `params[]`, `returnType`, `effects[]`, `grant[]`, `body[]`

**Statements:**
- `VarDecl` — `name`, `type`, `value`
- `AssignStmt` — `target`, `value`
- `ReturnStmt` — `value` (optional)
- `IfStmt` — `cond` (or `condition`), `then[]`, `else[]`
- `WhileStmt` — `cond` (or `condition`), `body[]`
- `ExprStmt` — `expr`

**Expressions:**
- `Literal` — `valueType` (`Int` / `Float` / `String` / `Bool`), `value`
- `Ident` — `name`
- `BinaryExpr` — `op` (`+` `-` `*` `/` `%` `==` `!=` `<` `>` `<=` `>=` `&&` `||`), `left`, `right`
- `UnaryExpr` — `op` (`-` `!`), `operand`
- `CallExpr` — `callee`, `args[]`
- `MemberExpr` — `object`, `field`

**Built-in functions:** `println` (state), `printf` (state), `sprintf` (pure)

## MCP Integration

Xiaoqinli runs as a local MCP server for Claude Code, Cursor, and other MCP-compatible editors.

```bash
./xql stdio                      # stdio mode
./xql http :8080                 # streamable HTTP mode
./xql http :8080 --mode rest     # REST API mode
```

**Setup** — add to `~/.mcp.json`:

```json
{
  "xiaoqinli": {
    "command": "/path/to/xql",
    "args": ["stdio"]
  }
}
```

| Tool | Description |
|------|-------------|
| `compile` | Compile `.xql.json` AST to target language. Args: `source`, `target` (default: go) |
| `validate` | Validate `.xql.json` AST without generating code. Args: `source` |

| Prompt | Description |
|--------|-------------|
| `xiaoqinli-usage-guide` | Full `.xql.json` format reference |
| `xiaoqinli-error-handbook` | All `XQL_Exxx` error codes with causes and fixes |

## Error Codes

| Range | Category | Phase |
|-------|----------|-------|
| `XQL_E1xx` | Parse / AST errors | Input |
| `XQL_E2xx` | Type / effect errors | Static check |
| `XQL_E3xx` | Capability errors | Static check |
| `XQL_E4xx` | Codegen errors | Code generation |

Run `xql validate` first to catch errors before codegen. Use PascalCase type names (`Int`, not `int`).

## Project Structure

```
xiaoqinli/
  main.go                 CLI entry point + version
  ast/
    nodes.go              AST node definitions + JSON parser
    hash.go               Content-addressable hashing (SHA-256)
  check/
    types.go              Type checker + transitive effect inference
    capability.go         Capability checker (@grant)
    check.go              RunAll orchestrator
  codegen/
    golang.go             Go backend
    rust.go               Rust backend
    typescript.go         TypeScript backend
    kotlin.go             Kotlin backend
    swift.go              Swift backend
    python.go             Python backend
    java.go               Java backend
    csharp.go             C# backend
    dart.go               Dart backend
    lua.go                Lua backend
    ruby.go               Ruby backend
    php.go                PHP backend
    zig.go                Zig backend
    nim.go                Nim backend
    julia.go              Julia backend
    util.go               Generate() dispatcher + shared utilities
    codegen_test.go       37 unit tests across all backends
  server/
    mcp.go                MCP server (stdio + HTTP) with panic recovery
    rest.go               REST API server
    skills.go             Skills dispatcher
  vfs/
    workspace.go          In-memory virtual filesystem
  skills/
    *.md                  Skill documents (embedded via go:embed)
  examples/
    hello.xql.json        Hello world
    example.xql.json      Fibonacci + arithmetic
    clock.xql.json        System clock with live output
```

## Tests

```bash
go test ./... -v
```

37 tests covering AST parsing, type/effect/capability checking, and code generation for all 15 backends.

## Design Principles

- **Minimal** — Single language (Go), single binary, zero third-party dependencies
- **Secure** — All validation at compile time, deterministic output, no runtime uncertainty
- **Fast** — 2-layer pipeline (check then codegen), single-pass AST traversal, no IR

## License

MIT
