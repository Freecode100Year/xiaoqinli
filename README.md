# Xiaoqinli (xql)

**AST-First transpiler for AI agents.** One JSON AST in, idiomatic source code out — 15 languages, single Go binary, zero dependencies.

```
  .xql.json  ──▶  Type Check  ──▶  Effect Inference  ──▶  Capability Enforcement  ──▶  Source Code
  (JSON AST)      Scope Nesting    Transitive Analysis     @grant Verification         (15 targets)
```

AI agents write structured `.xql.json` directly — no parser, no syntax errors. The compiler validates types, effects, and capabilities at compile time, then emits idiomatic code for the chosen target.

## Supported Languages

| Target | Flag | Verified |
|--------|------|----------|
| Go | `go` | yes |
| Rust | `rust` | yes |
| TypeScript | `ts` | yes |
| Python | `py` | yes |
| Kotlin | `kotlin` | — |
| Swift | `swift` | — |
| Java | `java` | — |
| C# | `csharp` | — |
| Dart | `dart` | — |
| Lua | `lua` | — |
| Ruby | `ruby` | — |
| PHP | `php` | — |
| Zig | `zig` | — |
| Nim | `nim` | — |
| Julia | `julia` | — |

## Quick Start

```bash
go build -o xql .

# Validate without generating code
./xql validate --file examples/hello.xql.json

# Compile to any target
./xql compile --file examples/hello.xql.json --target go
./xql compile --file examples/hello.xql.json --target rust   --out main.rs
./xql compile --file examples/hello.xql.json --target py     --out main.py
```

## One AST, Fifteen Languages

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
<summary><strong>Go output</strong></summary>

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
<summary><strong>Rust output</strong></summary>

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
<summary><strong>Python output</strong></summary>

```python
def greet(name: str) -> str:
    return ("Hello, " + name)

def main() -> None:
    print(greet("World"))

if __name__ == "__main__":
    main()
```
</details>

## Static Analysis Pipeline

All checks run before code generation. If any check fails, no code is emitted.

| Phase | What it validates | Error codes |
|-------|-------------------|-------------|
| **Type check** | Variable types, function signatures, return types, operator compatibility, array element types, struct field types, index expression types | `XQL_E2xx` |
| **Effect inference** | Side effects (`network`/`filesystem`/`state`), purity violations, transitive propagation through call chains | `XQL_E2xx` |
| **Capability check** | `@grant` enforcement — callee capabilities must be subset of caller's, checked through all control flow paths including for-loops | `XQL_E3xx` |

### Type Inference

The type checker tracks full generic type information through expressions:

- `Array<Int>` indexing returns `Int` (not unknown)
- `for-each` loop variable inherits the array's element type
- Struct field access returns the field's declared type
- Binary operators propagate types through the expression tree

### Scoped Type Checking

Variables declared inside `if`/`while`/`for` blocks do not leak into the enclosing scope. Each block creates a child scope that inherits parent bindings but isolates its own declarations.

## Type System

| XQL Kind | Go | Rust | TypeScript | Python | Kotlin | Swift |
|----------|-----|------|------------|--------|--------|-------|
| `Int` | `int` | `i64` | `number` | `int` | `Long` | `Int` |
| `Float` | `float64` | `f64` | `number` | `float` | `Double` | `Double` |
| `String` | `string` | `String` | `string` | `str` | `String` | `String` |
| `Bool` | `bool` | `bool` | `boolean` | `bool` | `Boolean` | `Bool` |
| `Void` | — | — | `void` | `None` | `Unit` | — |
| `Array<T>` | `[]T` | `Vec<T>` | `T[]` | `list[T]` | `List<T>` | `[T]` |
| `Option<T>` | `*T` | `Option<T>` | `T \| null` | `Optional[T]` | `T?` | `T?` |

## .xql.json Node Reference

### Program

```json
{ "kind": "Program", "declarations": [...] }
```

### Declarations

- **FunctionDecl** — `name`, `params[]` (`{name, type}`), `returnType`, `effects[]`, `grant[]`, `body[]`
- **StructDecl** — `name`, `fields[]` (`{name, type}`)

### Statements

- **VarDecl** — `name`, `type`, `value`
- **AssignStmt** — `target` (string or expression node), `value`. Target can be a variable name, `IndexExpr`, or `MemberExpr` for `arr[i] = x` and `obj.field = x`.
- **ReturnStmt** — `value` (optional)
- **IfStmt** — `cond`, `then[]`, `else[]`
- **WhileStmt** — `cond`, `body[]`
- **ForStmt** — `form` (`"range"` or `"each"`), `var`, `start`/`end` (range) or `iterable` (each), `body[]`
- **BreakStmt** — exits the innermost loop
- **ContinueStmt** — skips to next iteration (not supported in Lua)
- **ExprStmt** — `expr`

### Expressions

- **Literal** — `valueType` (`Int`/`Float`/`String`/`Bool`), `value`
- **Ident** — `name`
- **BinaryExpr** — `op` (`+` `-` `*` `/` `%` `==` `!=` `<` `>` `<=` `>=` `&&` `||`), `left`, `right`
- **UnaryExpr** — `op` (`-` `!`), `operand`
- **CallExpr** — `callee`, `args[]`
- **MemberExpr** — `object`, `field`
- **StructLit** — `typeName`, `fields[]` (`{name, value}`)
- **ArrayLit** — `elemType`, `elements[]`
- **IndexExpr** — `target`, `index`

### Built-in Functions

| Function | Effect | Description |
|----------|--------|-------------|
| `println` | state | Print with newline |
| `printf` | state | Formatted print |
| `sprintf` | pure | Formatted string |

## ForStmt Examples

**Range form** — iterate from start to end (exclusive):

```json
{
  "kind": "ForStmt", "form": "range", "var": "i",
  "start": { "kind": "Literal", "valueType": "Int", "value": 0 },
  "end": { "kind": "Literal", "valueType": "Int", "value": 10 },
  "body": [...]
}
```

**Each form** — iterate over elements of an array:

```json
{
  "kind": "ForStmt", "form": "each", "var": "item",
  "iterable": { "kind": "Ident", "name": "items" },
  "body": [...]
}
```

## MCP Server

Xiaoqinli runs as a local MCP server for Claude Code, Cursor, and other MCP-compatible editors.

```bash
./xql stdio                      # stdio mode (recommended)
./xql http :8080                 # streamable HTTP mode
./xql http :8080 --mode rest     # REST API mode
```

**Tools:**

| Tool | Args | Description |
|------|------|-------------|
| `compile` | `source`, `target` | Compile `.xql.json` AST to target language |
| `validate` | `source` | Validate AST without generating code |

## Error Codes

| Range | Category | Phase |
|-------|----------|-------|
| `XQL_E1xx` | Parse / AST structure | Input |
| `XQL_E2xx` | Type / effect violations | Static check |
| `XQL_E3xx` | Capability violations | Static check |
| `XQL_E4xx` | Codegen errors | Code generation |

## Project Structure

```
xiaoqinli/
  main.go                    CLI + version (2.3.0)
  ast/
    nodes.go                 AST node definitions + JSON parser
    hash.go                  Content-addressable hashing
  check/
    types.go                 Type checker + scoped inference + effect system
    capability.go            Capability checker (@grant)
    check.go                 RunAll orchestrator
  codegen/
    golang.go ... julia.go   15 language backends
    util.go                  Generate() dispatcher + shared utilities
    codegen_test.go          Tests across all 15 backends
    roundtrip_test.go        Compile-and-run verification
  server/
    mcp.go                   MCP server (stdio + streamable HTTP)
    rest.go                  REST API server
    skills.go                Skills dispatcher
  vfs/
    workspace.go             In-memory virtual filesystem
  skills/
    *.md                     Embedded skill documents
  examples/
    hello.xql.json           Hello world
    example.xql.json         Fibonacci + arithmetic
    clock.xql.json           System clock with live output
    loop.xql.json            For-loop with array indexing
    struct.xql.json          Struct declaration and literals
    collections.xql.json     Array literals and indexing
```

## Tests

```bash
go test ./... -v
```

## Design Principles

- **Minimal** — Single language (Go), single binary, zero third-party dependencies
- **Secure** — All validation at compile time, deterministic output, no runtime uncertainty
- **Fast** — 2-layer pipeline (check then codegen), single-pass AST traversal, no IR

## License

MIT
