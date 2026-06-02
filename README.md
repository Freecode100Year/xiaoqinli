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
