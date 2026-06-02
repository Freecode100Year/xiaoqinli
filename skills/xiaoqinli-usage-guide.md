# Xiaoqinli Usage Guide

## Overview

Xiaoqinli (xql) is an AST-First transpiler that converts `.xql.json` files into target language source code. It is designed for AI agents to generate safe, verified programs.

## Quick Start

```bash
# Build
go build -o xql .

# Compile .xql.json to Go
xql compile --file program.xql.json --target go

# Write output to file
xql compile --file program.xql.json --target go --out main.go

# Validate only (no codegen)
xql validate --file program.xql.json
```

## .xql.json Format

Every node has a `"kind"` field. The top-level node must be `"Program"` with a `"declarations"` array.

### Type System

Types are objects with a `"kind"` field:

| Kind     | Go mapping  | Description       |
|----------|-------------|-------------------|
| `Int`    | `int`       | Integer           |
| `Float`  | `float64`   | Floating point    |
| `String` | `string`    | UTF-8 string      |
| `Bool`   | `bool`      | Boolean           |
| `Void`   | *(none)*    | No return value   |
| `Array`  | `[]T`       | Array with `elem` |
| `Option` | `*T`        | Nullable with `elem` |
| `Result` | `(T, error)` | With `okType`/`errType` |

### Node Types

**Declarations:**
- `Program` — `{ "kind": "Program", "declarations": [...] }`
- `FunctionDecl` — `{ "kind": "FunctionDecl", "name": "...", "params": [...], "returnType": {...}, "effects": [...], "grant": [...], "body": [...] }`

**Statements:**
- `VarDecl` — `{ "kind": "VarDecl", "name": "...", "type": {...}, "value": {...} }`
- `AssignStmt` — `{ "kind": "AssignStmt", "target": "...", "value": {...} }`
- `ReturnStmt` — `{ "kind": "ReturnStmt", "value": {...} }`
- `IfStmt` — `{ "kind": "IfStmt", "condition": {...}, "then": [...], "else": [...] }`
- `WhileStmt` — `{ "kind": "WhileStmt", "condition": {...}, "body": [...] }`
- `ExprStmt` — `{ "kind": "ExprStmt", "expr": {...} }`

**Expressions:**
- `Literal` — `{ "kind": "Literal", "valueType": "Int", "value": 42 }`
- `Ident` — `{ "kind": "Ident", "name": "x" }`
- `BinaryExpr` — `{ "kind": "BinaryExpr", "op": "+", "left": {...}, "right": {...} }`
- `UnaryExpr` — `{ "kind": "UnaryExpr", "op": "-", "operand": {...} }`
- `CallExpr` — `{ "kind": "CallExpr", "callee": "println", "args": [...] }`
- `MemberExpr` — `{ "kind": "MemberExpr", "object": {...}, "field": "..." }`

### Built-in Functions

| Name      | Effect | Description            |
|-----------|--------|------------------------|
| `println` | state  | Print with newline     |
| `printf`  | state  | Formatted print        |
| `sprintf` | pure   | Formatted string build |

## Safety: @effects and @grant

### Effects

Declare `"effects": ["pure"]` on a function to assert it has no side effects. The compiler verifies this at compile time.

### Capabilities (@grant)

`"grant": ["io", "network"]` declares what capabilities a function may use. When function A calls function B, B's grant must be a **subset** of A's grant.

## Example

```json
{
  "kind": "Program",
  "declarations": [
    {
      "kind": "FunctionDecl",
      "name": "add",
      "params": [
        { "name": "a", "type": { "kind": "Int" } },
        { "name": "b", "type": { "kind": "Int" } }
      ],
      "returnType": { "kind": "Int" },
      "effects": ["pure"],
      "grant": [],
      "body": [
        {
          "kind": "ReturnStmt",
          "value": {
            "kind": "BinaryExpr",
            "op": "+",
            "left": { "kind": "Ident", "name": "a" },
            "right": { "kind": "Ident", "name": "b" }
          }
        }
      ]
    }
  ]
}
```

Compiles to:

```go
package main

func add(a int, b int) int {
    return a + b
}
```
