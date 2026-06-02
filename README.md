# Xiaoqinli (xql)

**AST-First transpiler for AI agents.** One JSON AST in, idiomatic source code out — 15 languages, single Go binary, zero dependencies.

```
                        ┌──────────────────────────────────┐
  .xql.json  ──────────▶│  Type Check                      │
  (JSON AST)            │  Effect Inference                 │──▶  Source Code
                        │  Capability Enforcement (@grant)  │     (15 languages)
                        └──────────────────────────────────┘
```

AI agents write structured `.xql.json` directly — no text parser, no syntax errors. The compiler validates types, effects, and capabilities at compile time, then emits idiomatic code for the chosen target.

## Supported Languages

| Target | CLI flag | String concat | Entry point | Mutability |
|--------|----------|---------------|-------------|------------|
| Go | `go` | `+` | `func main()` | all mutable |
| Rust | `rust` | `+ &` | `fn main()` | `let` / `let mut` |
| TypeScript | `ts` | `+` | `main()` call | `const` / `let` |
| Kotlin | `kotlin` | `+` | `fun main()` | `val` / `var` |
| Swift | `swift` | `+` | top-level body | `let` / `var` |
| Python | `py` | `+` | `if __name__` guard | all mutable |
| Java | `java` | `+` | `public static void main` | `final` / bare |
| C# | `csharp` | `+` | `static void Main()` | all mutable |
| Dart | `dart` | `+` | `void main()` | `final` / bare |
| Lua | `lua` | `..` | top-level body | `local` |
| Ruby | `ruby` | `+` | top-level body | all mutable |
| PHP | `php` | `.` | top-level body | `$` prefix |
| Zig | `zig` | `++` | `pub fn main()` | `const` / `var` |
| Nim | `nim` | `&` | top-level body | `let` / `var` |
| Julia | `julia` | `*` | `main()` call | all mutable |

## Quick Start

```bash
go build -o xql .

# Validate without generating code
./xql validate --file examples/hello.xql.json

# Compile to any target
./xql compile --file examples/hello.xql.json --target go
./xql compile --file examples/hello.xql.json --target rust   --out main.rs
./xql compile --file examples/hello.xql.json --target ts     --out main.ts
./xql compile --file examples/hello.xql.json --target kotlin --out main.kt
./xql compile --file examples/hello.xql.json --target swift  --out main.swift
./xql compile --file examples/hello.xql.json --target py     --out main.py
./xql compile --file examples/hello.xql.json --target java   --out Main.java
./xql compile --file examples/hello.xql.json --target csharp --out Program.cs
./xql compile --file examples/hello.xql.json --target dart   --out main.dart
./xql compile --file examples/hello.xql.json --target lua    --out main.lua
./xql compile --file examples/hello.xql.json --target ruby   --out main.rb
./xql compile --file examples/hello.xql.json --target php    --out main.php
./xql compile --file examples/hello.xql.json --target zig    --out main.zig
./xql compile --file examples/hello.xql.json --target nim    --out main.nim
./xql compile --file examples/hello.xql.json --target julia  --out main.jl
```

## One AST, Fifteen Languages

Write one `.xql.json`, compile to any target. Here's `hello.xql.json`:

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
fn greet(name: String) -> String {
    return (String::from("Hello, ") + &name);
}

fn main() {
    println!("{}", greet(String::from("World")));
}
```
</details>

<details>
<summary><strong>TypeScript</strong></summary>

```typescript
function greet(name: string): string {
    return ("Hello, " + name);
}

function main(): void {
    console.log(greet("World"));
}

main();
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
<summary><strong>C#</strong></summary>

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
<summary><strong>Dart</strong></summary>

```dart
String greet(String name) {
    return ("Hello, " + name);
}

void main() {
    print(greet("World"));
}
```
</details>

<details>
<summary><strong>Lua</strong></summary>

```lua
function greet(name)
    return ("Hello, " .. name)
end

print(greet("World"))
```
</details>

<details>
<summary><strong>Ruby</strong></summary>

```ruby
def greet(name)
  return ("Hello, " + name)
end

puts(greet("World"))
```
</details>

<details>
<summary><strong>PHP</strong></summary>

```php
<?php

function greet(string $name): string {
    return ("Hello, " . $name);
}

echo greet("World") . "\n";
```
</details>

<details>
<summary><strong>Zig</strong></summary>

```zig
const std = @import("std");

fn greet(name: []const u8) []const u8 {
    return ("Hello, " ++ name);
}

pub fn main() void {
    std.debug.print("{}\n", .{greet("World")});
}
```
</details>

<details>
<summary><strong>Nim</strong></summary>

```nim
proc greet(name: string): string =
  return ("Hello, " & name)

echo greet("World")
```
</details>

<details>
<summary><strong>Julia</strong></summary>

```julia
function greet(name::String)::String
    return ("Hello, " * name)
end

function main()
    println(greet("World"))
end

main()
```
</details>

## Three Static Checks

All checks run before code generation. If any check fails, no code is emitted.

| Phase | What it validates | Error codes |
|-------|-------------------|-------------|
| **Type check** | Variable types, function signatures, return types, operator compatibility | `XQL_E2xx` |
| **Effect inference** | Side effects (`network` / `filesystem` / `state`), purity violations | `XQL_E2xx` |
| **Capability check** | `@grant` enforcement — callee capabilities must be subset of caller's | `XQL_E3xx` |

```json
"effects": ["pure"],
"grant": ["io", "network"]
```

A `pure` function cannot call `println` or any function with side effects. If function A calls function B, A's `grant` must cover all of B's `grant`.

## Type System

| XQL Kind | Go | Rust | TypeScript | Kotlin | Swift | Python |
|----------|-----|------|------------|--------|-------|--------|
| `Int` | `int` | `i64` | `number` | `Long` | `Int` | `int` |
| `Float` | `float64` | `f64` | `number` | `Double` | `Double` | `float` |
| `String` | `string` | `String` | `string` | `String` | `String` | `str` |
| `Bool` | `bool` | `bool` | `boolean` | `Boolean` | `Bool` | `bool` |
| `Void` | *(none)* | *(none)* | `void` | `Unit` | *(none)* | `None` |
| `Array<T>` | `[]T` | `Vec<T>` | `T[]` | `List<T>` | `[T]` | `list[T]` |
| `Option<T>` | `*T` | `Option<T>` | `T \| null` | `T?` | `T?` | `Optional[T]` |
| `Result<T>` | `(T, error)` | `Result<T,E>` | — | — | — | — |

| XQL Kind | Java | C# | Dart | PHP | Zig | Nim | Julia |
|----------|------|-----|------|-----|-----|-----|-------|
| `Int` | `long` | `long` | `int` | `int` | `i64` | `int64` | `Int64` |
| `Float` | `double` | `double` | `double` | `float` | `f64` | `float64` | `Float64` |
| `String` | `String` | `string` | `String` | `string` | `[]const u8` | `string` | `String` |
| `Bool` | `boolean` | `bool` | `bool` | `bool` | `bool` | `bool` | `Bool` |
| `Void` | `void` | `void` | `void` | `void` | `void` | *(none)* | `Nothing` |
| `Array<T>` | `List<T>` | `List<T>` | `List<T>` | `array` | — | `seq[T]` | `Vector{T}` |
| `Option<T>` | `T` (boxed) | `T?` | `T?` | `?T` | `?T` | `Option[T]` | `Union{T,Nothing}` |

Lua and Ruby are dynamically typed — no type annotations are emitted.

## .xql.json Node Reference

**Program:** `{ "kind": "Program", "declarations": [...] }`

**Declarations:**
- `FunctionDecl` — `name`, `params[]` (`{name, type}`), `returnType`, `effects[]`, `grant[]`, `body[]`

**Statements:**
- `VarDecl` — `name`, `type`, `value`
- `AssignStmt` — `target`, `value`
- `ReturnStmt` — `value` (optional)
- `IfStmt` — `condition`, `then[]`, `else[]`
- `WhileStmt` — `condition`, `body[]`
- `ExprStmt` — `expr`

**Expressions:**
- `Literal` — `valueType` (`Int` / `Float` / `String` / `Bool`), `value`
- `Ident` — `name`
- `BinaryExpr` — `op` (`+` `-` `*` `/` `%` `==` `!=` `<` `>` `<=` `>=` `&&` `||`), `left`, `right`
- `UnaryExpr` — `op` (`-` `!`), `operand`
- `CallExpr` — `callee`, `args[]`
- `MemberExpr` — `object`, `field`

**Built-in functions:** `println` (effect: state), `printf` (effect: state), `sprintf` (pure)

## MCP Server

Xiaoqinli runs as a local MCP server for Claude Code, Cursor, and other MCP-compatible editors.

```bash
./xql stdio                      # stdio mode (recommended)
./xql http :8080                 # streamable HTTP mode
./xql http :8080 --mode rest     # REST API mode
```

**Setup** — add to your MCP config:

```json
{
  "xiaoqinli": {
    "command": "/path/to/xql",
    "args": ["stdio"]
  }
}
```

**Tools:**

| Tool | Args | Description |
|------|------|-------------|
| `compile` | `source`, `target` | Compile `.xql.json` AST to target language (default: go) |
| `validate` | `source` | Validate AST without generating code |

**Prompts:**

| Prompt | Description |
|--------|-------------|
| `xiaoqinli-usage-guide` | Full `.xql.json` format reference |
| `xiaoqinli-error-handbook` | All `XQL_Exxx` error codes with causes and fixes |

## Error Codes

| Range | Category | Phase |
|-------|----------|-------|
| `XQL_E1xx` | Parse / AST structure | Input |
| `XQL_E2xx` | Type / effect violations | Static check |
| `XQL_E3xx` | Capability violations | Static check |
| `XQL_E4xx` | Codegen errors | Code generation |

Use PascalCase type names in the AST (`Int`, not `int`). Run `xql validate` to catch errors before codegen.

## Project Structure

```
xiaoqinli/
  main.go                    CLI + version (2.2.0)
  ast/
    nodes.go                 AST node definitions + JSON parser
    hash.go                  Content-addressable hashing (SHA-256)
  check/
    types.go                 Type checker + transitive effect inference
    capability.go            Capability checker (@grant)
    check.go                 RunAll orchestrator
  codegen/
    golang.go  rust.go       Go, Rust backends
    typescript.go  kotlin.go TypeScript, Kotlin backends
    swift.go  python.go      Swift, Python backends
    java.go  csharp.go       Java, C# backends
    dart.go  lua.go          Dart, Lua backends
    ruby.go  php.go          Ruby, PHP backends
    zig.go  nim.go  julia.go Zig, Nim, Julia backends
    util.go                  Generate() dispatcher + shared utilities
    codegen_test.go          48 tests across all 15 backends
  server/
    mcp.go                   MCP server (stdio + streamable HTTP)
    rest.go                  REST API server
    skills.go                Skills dispatcher
  vfs/
    workspace.go             In-memory virtual filesystem
  skills/
    *.md                     Skill documents (embedded via go:embed)
  examples/
    hello.xql.json           Hello world
    example.xql.json         Fibonacci + arithmetic
    clock.xql.json           System clock with live output
```

## Tests

```bash
go test ./... -v
```

48 tests covering AST parsing, type/effect/capability checking, and code generation for all 15 backends.

## Design Principles

- **Minimal** — Single language (Go), single binary, zero third-party dependencies
- **Secure** — All validation at compile time, deterministic output, no runtime uncertainty
- **Fast** — 2-layer pipeline (check then codegen), single-pass AST traversal, no IR

## License

MIT
