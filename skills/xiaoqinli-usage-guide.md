# Xiaoqinli Usage Guide

## Overview

Xiaoqinli (xql) is an AST-First transpiler that converts `.xql.json` files into target language source code. It is designed for AI agents to generate safe, verified programs.

## Quick Start

```bash
# Build
go build -o xql .

# Compile .xql.json to any of 33 targets
xql compile --file program.xql.json --target go
xql compile --file program.xql.json --target rust --out main.rs

# Validate only (no codegen)
xql validate --file program.xql.json

# List all targets
xql targets

# MCP server
xql stdio                      # stdio mode
xql http :8080                 # HTTP mode
xql http :8080 --mode rest     # REST API mode
```

## Supported Targets (33)

**Systems:** `go` `rust` `c` `cpp` `zig` `d` `v` `nim`
**JVM/CLR:** `java` `kotlin` `scala` `csharp` `dart`
**Scripting:** `py` `ts` `ruby` `lua` `php` `perl` `julia` `crystal` `awk`
**Functional:** `haskell`
**Shell:** `bash` `powershell` `tcl`
**Legacy/Niche:** `ada` `fortran` `pascal` `objc` `mql4` `mql5`

## .xql.json Format

Every node has a `"kind"` field. The top-level node must be `"Program"` with a `"declarations"` array.

### Type System

| Kind | Go | Rust | Python | Description |
|------|-----|------|--------|-------------|
| `Int` | `int` | `i64` | `int` | Integer |
| `Float` | `float64` | `f64` | `float` | Floating point |
| `String` | `string` | `String` | `str` | UTF-8 string |
| `Bool` | `bool` | `bool` | `bool` | Boolean |
| `Void` | *(none)* | `()` | `None` | No return value |
| `Array` | `[]T` | `Vec<T>` | `list[T]` | Array with `elem` |
| `Option` | `*T` | `Option<T>` | `Optional[T]` | Nullable with `elem` |
| `Result` | `(T, error)` | `Result<T,E>` | *(E402)* | With `okType`/`errType` |

### Declarations

- `Program` — `{ "kind": "Program", "declarations": [...] }`
- `FunctionDecl` — `{ "kind": "FunctionDecl", "name": "...", "params": [...], "returnType": {...}, "effects": [...], "grant": [...], "body": [...] }`
- `StructDecl` — `{ "kind": "StructDecl", "name": "...", "fields": [{"name": "...", "type": {...}}] }`
- `EnumDecl` — `{ "kind": "EnumDecl", "name": "...", "variants": ["A", "B", "C"] }`

### Statements

- `VarDecl` — `{ "kind": "VarDecl", "name": "...", "type": {...}, "value": {...} }`
- `AssignStmt` — `{ "kind": "AssignStmt", "target": {...}, "value": {...} }`
- `ReturnStmt` — `{ "kind": "ReturnStmt", "value": {...} }`
- `IfStmt` — `{ "kind": "IfStmt", "cond": {...}, "then": [...], "else": [...] }`
- `WhileStmt` — `{ "kind": "WhileStmt", "cond": {...}, "body": [...] }`
- `ForStmt` — `{ "kind": "ForStmt", "form": "range|each", "var": "i", ... }`
- `BreakStmt` — `{ "kind": "BreakStmt" }`
- `ContinueStmt` — `{ "kind": "ContinueStmt" }`
- `ExprStmt` — `{ "kind": "ExprStmt", "expr": {...} }`

### Expressions

- `Literal` — `{ "kind": "Literal", "valueType": "Int|Float|String|Bool", "value": 42 }`
- `Ident` — `{ "kind": "Ident", "name": "x" }`
- `BinaryExpr` — `{ "kind": "BinaryExpr", "op": "+", "left": {...}, "right": {...} }`
- `UnaryExpr` — `{ "kind": "UnaryExpr", "op": "-", "operand": {...} }`
- `CallExpr` — `{ "kind": "CallExpr", "callee": "println", "args": [...] }`
- `MemberExpr` — `{ "kind": "MemberExpr", "object": {...}, "field": "x" }`
- `StructLit` — `{ "kind": "StructLit", "typeName": "Point", "fields": [{"name": "x", "value": {...}}] }`
- `ArrayLit` — `{ "kind": "ArrayLit", "elemType": {"kind": "Int"}, "elements": [...] }`
- `IndexExpr` — `{ "kind": "IndexExpr", "target": {...}, "index": {...} }`
- `MatchExpr` — `{ "kind": "MatchExpr", "value": {...}, "arms": [{"pattern": {...}, "body": [...]}] }`
- `IfExpr` — `{ "kind": "IfExpr", "cond": {...}, "then": {...}, "else": {...} }`
- `Lambda` — `{ "kind": "Lambda", "params": [...], "returnType": {...}, "body": [...] }`

### ForStmt Details

**Range form** (exclusive end):
```json
{ "kind": "ForStmt", "form": "range", "var": "i",
  "start": {"kind": "Literal", "valueType": "Int", "value": 0},
  "end": {"kind": "Literal", "valueType": "Int", "value": 10},
  "body": [...] }
```

**Each form** (iterate array):
```json
{ "kind": "ForStmt", "form": "each", "var": "item",
  "iterable": {"kind": "Ident", "name": "items"},
  "body": [...] }
```

### IfExpr vs IfStmt

`IfStmt` is a statement (appears in body). `IfExpr` is an expression (appears as a value):
```json
{ "kind": "VarDecl", "name": "label", "type": {"kind": "String"},
  "value": {
    "kind": "IfExpr",
    "cond": {"kind": "BinaryExpr", "op": ">", "left": ..., "right": ...},
    "then": {"kind": "Literal", "valueType": "String", "value": "big"},
    "else": {"kind": "Literal", "valueType": "String", "value": "small"}
  }
}
```

Maps to: `cond ? then : else` (most languages), `then if cond else else` (Python), IIFE (Go), `if cond { then } else { else }` (Rust/V).

### Lambda

Anonymous function expression:
```json
{ "kind": "Lambda",
  "params": [{"name": "x", "type": {"kind": "Int"}}],
  "returnType": {"kind": "Int"},
  "body": [{"kind": "ReturnStmt", "value": ...}]
}
```

Not supported in: C, Zig, MQL4/5, Ada, AWK, Bash, Fortran, Pascal, Tcl (returns `XQL_E401`).

### Built-in Functions

| Name | Effect | Description |
|------|--------|-------------|
| `println` | state | Print with newline |
| `printf` | state | Formatted print |
| `sprintf` | pure | Formatted string (returns String) |

## Safety: @effects and @grant

### Effects

Declare `"effects": ["pure"]` on a function to assert it has no side effects. The compiler verifies this at compile time — calling `println` from a pure function produces `XQL_E203`.

### Capabilities (@grant)

`"grant": ["io", "network"]` declares what capabilities a function may use. When function A calls function B, B's grant must be a **subset** of A's grant (`XQL_E302`).

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
