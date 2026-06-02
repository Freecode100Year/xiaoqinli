# Xiaoqinli (xql)

AST-First transpiler for AI agents. Single Go binary, zero dependencies.

AI agents write structured `.xql.json` (AST) directly — no parser needed, no syntax errors possible. The compiler validates types, effects, and capabilities at compile time, then emits idiomatic source code.

## Targets

Go | Rust | TypeScript | Kotlin | Swift | Python

## Quick Start

```bash
go build -o xql .

# Validate
./xql validate --file hello.xql.json

# Compile to Go (default)
./xql compile --file example.xql.json --target go

# Compile to other languages
./xql compile --file example.xql.json --target rust --out main.rs
./xql compile --file example.xql.json --target ts --out main.ts
./xql compile --file example.xql.json --target kotlin --out main.kt
./xql compile --file example.xql.json --target swift --out main.swift
./xql compile --file example.xql.json --target py --out main.py
```

## Use with Claude Code (MCP + Skills)

Xiaoqinli can be used as a local MCP server inside Claude Code, providing `compile` and `validate` tools directly in your conversations.

### Setup

**1. Add MCP server config** — create or edit `~/.mcp.json`:

```json
{
  "xiaoqinli": {
    "command": "/path/to/xql",
    "args": ["stdio"]
  }
}
```

**2. (Optional) Auto-allow tool calls** — add to `~/.claude/settings.local.json` under `permissions.allow`:

```json
"mcp__xiaoqinli__compile",
"mcp__xiaoqinli__validate"
```

**3. (Optional) Add slash commands** — create these files:

- `~/.claude/commands/xql-compile.md` — compile guide + AST reference
- `~/.claude/commands/xql-validate.md` — validate guide + error codes

**4. Restart Claude Code** to load the MCP server.

### Usage in Claude Code

Once connected, you can ask Claude directly:

```
Compile this xql program to Rust:
{ "kind": "Program", "declarations": [...] }
```

Claude will call the `mcp__xiaoqinli__compile` tool automatically.

Available slash commands:
- `/xql-compile` — compile guide with full AST node reference
- `/xql-validate` — validate guide with error code reference

### MCP Tools

| Tool | Description |
|------|-------------|
| `compile` | Compile `.xql.json` AST to target language. Args: `source` (JSON string), `target` (go/rust/ts/kotlin/swift/py) |
| `validate` | Validate `.xql.json` AST without codegen. Args: `source` (JSON string) |

### MCP Prompts (Skills)

| Prompt | Description |
|--------|-------------|
| `xiaoqinli-usage-guide` | Full `.xql.json` format reference, type system, node types, built-in functions |
| `xiaoqinli-error-handbook` | All `XQL_Exxx` error codes with causes and fixes |

## Example

`hello.xql.json`:
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

Compiles to Go:
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

## Three Static Checks

All checks run at compile time. Code generation only proceeds if all three pass.

| Check | What it does | Error codes |
|-------|-------------|-------------|
| **Type check** | Validates variable types, function signatures, return types | `XQL_E2xx` |
| **Effect inference** | Infers side effects (network/filesystem/state). Catches purity violations | `XQL_E2xx` |
| **Capability check** | Enforces `@grant` — callee capabilities must be subset of caller's | `XQL_E3xx` |

## Server Modes

```bash
# MCP stdio (for Claude Code, Cursor, etc.)
./xql stdio

# MCP over HTTP
./xql http :8080

# REST API
./xql http :8080 --mode rest
```

## .xql.json AST Reference

### Type System

| Kind | Go | Rust | TypeScript | Kotlin | Swift | Python |
|------|-----|------|------------|--------|-------|--------|
| `Int` | `int` | `i64` | `number` | `Long` | `Int` | `int` |
| `Float` | `float64` | `f64` | `number` | `Double` | `Double` | `float` |
| `String` | `string` | `String` | `string` | `String` | `String` | `str` |
| `Bool` | `bool` | `bool` | `boolean` | `Boolean` | `Bool` | `bool` |
| `Void` | *(none)* | *(none)* | `void` | `Unit` | *(none)* | `None` |
| `Array` | `[]T` | `Vec<T>` | `T[]` | `List<T>` | `[T]` | `list[T]` |
| `Option` | `*T` | `Option<T>` | `T \| null` | `T?` | `T?` | `Optional[T]` |
| `Result` | `(T, error)` | `Result<T, E>` | — | — | — | — |

### Node Kinds

**Declarations:**
- `Program` — top-level, contains `declarations[]`
- `FunctionDecl` — `name`, `params[]`, `returnType`, `effects[]`, `grant[]`, `body[]`

**Statements:**
- `VarDecl` — `name`, `type`, `value`
- `AssignStmt` — `target`, `value`
- `ReturnStmt` — `value` (optional)
- `IfStmt` — `condition`, `then[]`, `else[]`
- `WhileStmt` — `condition`, `body[]`
- `ExprStmt` — `expr`

**Expressions:**
- `Literal` — `valueType`, `value`
- `Ident` — `name`
- `BinaryExpr` — `op`, `left`, `right`
- `UnaryExpr` — `op`, `operand`
- `CallExpr` — `callee`, `args[]`
- `MemberExpr` — `object`, `field`

### Built-in Functions

| Name | Effect | Description |
|------|--------|-------------|
| `println` | state | Print with newline |
| `printf` | state | Formatted print |
| `sprintf` | pure | Formatted string build |

### Safety Annotations

- `effects: ["pure"]` — compiler verifies no side effects
- `effects: ["state"]` / `["network"]` / `["filesystem"]` — declare side effects
- `grant: ["io", "network"]` — capability declaration; callee must be subset of caller

## Error Codes

| Range | Category | Phase |
|-------|----------|-------|
| `XQL_E1xx` | Parse / AST errors | Input |
| `XQL_E2xx` | Type / effect errors | Static check |
| `XQL_E3xx` | Capability errors | Static check |
| `XQL_E4xx` | Codegen errors | Code generation |

## Project Structure

```
xiaoqinli/
  main.go              # CLI entry point
  ast/
    nodes.go           # AST node definitions + parser
    hash.go            # Content-addressable hashing (SHA-256)
  check/
    types.go           # Type checker + effect inference
    capability.go      # Capability checker (@grant)
    check.go           # RunAll orchestrator
  codegen/
    golang.go          # Go backend
    rust.go            # Rust backend
    typescript.go      # TypeScript backend
    kotlin.go          # Kotlin backend
    swift.go           # Swift backend
    python.go          # Python backend
    util.go            # Shared codegen utilities
  server/
    mcp.go             # MCP server (stdio + HTTP)
    rest.go            # REST API server
    skills.go          # Skills dispatcher
  vfs/
    workspace.go       # In-memory virtual filesystem
  skills/
    *.md               # Skill documents (embedded via go:embed)
```

## Tests

```bash
go test ./... -v
```

## Design Principles

- **Minimal**: Single language, single binary, zero third-party dependencies
- **Secure**: All validation at compile time, deterministic output, no runtime uncertainty
- **Fast**: 2-layer pipeline (check then codegen), single-pass AST traversal, no intermediate representation

## License

MIT
