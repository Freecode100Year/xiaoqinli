# Xiaoqinli Error Handbook

All errors follow the `XQL_Exxx` pattern. Errors halt compilation — no partial output is produced.

## Error Code Ranges

| Range       | Category           | Phase      |
|-------------|--------------------|------------|
| `XQL_E1xx`  | Parse / AST errors | Layer 0    |
| `XQL_E2xx`  | Type / effect errors | Layer 1  |
| `XQL_E3xx`  | Capability errors  | Layer 1    |
| `XQL_E4xx`  | Codegen errors     | Layer 2    |

---

## XQL_E1xx — Parse Errors

### XQL_E101: Invalid JSON or missing fields

**Cause:** The input is not valid JSON, or a required field is missing from a node.

**Examples:**
- `XQL_E101: invalid JSON: unexpected end of JSON input`
- `XQL_E101: node missing 'kind' field`
- `XQL_E101: FunctionDecl missing 'name'`
- `XQL_E101: unknown node kind: FooBar`

**Fix:** Ensure the `.xql.json` file is well-formed and every node has the correct `kind` field and required properties.

---

## XQL_E2xx — Type & Effect Errors

### XQL_E201: Type check failed

**Cause:** One or more type mismatches were found.

**Sub-errors include:**
- **Return type mismatch:** Function declares return type `Int` but body returns `String`.
- **Variable type mismatch:** `VarDecl` declares type `Int` but is assigned a `String` expression.
- **Assignment type mismatch:** Assigning a `Float` to a variable declared as `Int`.
- **Argument type mismatch:** Calling `add(1, "hello")` when `add` expects `(Int, Int)`.
- **Argument count mismatch:** `add(1)` when `add` expects 2 args.
- **Operator type error:** Using `+` on `Bool` operands.
- **Condition type error:** `if` or `while` condition is not `Bool`.

**Fix:** Align types across declarations, assignments, returns, and function calls.

### XQL_E203: Effect check failed

**Cause:** A function declares `@effects(["pure"])` but the compiler inferred side effects.

**Example:**
```
function 'compute' declares @effects(["pure"]) but has inferred effect 'state'
```

**Fix:** Either remove the `pure` declaration, or remove the side-effecting call (e.g., `println`) from the function body.

---

## XQL_E3xx — Capability Errors

### XQL_E301: Missing required capability

**Cause:** A capability required by a callee is not present in the caller's `@grant`.

**Example:**
```
XQL_E301: missing required capability: io
```

### XQL_E302: Capability check failed

**Cause:** Function A calls function B, but A's `@grant` does not cover B's `@grant`.

**Example:**
```
function 'main' calls 'writeFile' but lacks required capabilities: [io, filesystem]
```

**Fix:** Add the missing capabilities to the caller's `"grant"` array. Capabilities must propagate up the call chain.

---

## XQL_E4xx — Codegen Errors

### XQL_E401: Codegen failure

**Cause:** The code generator encountered a node it cannot handle, or was asked to generate code for an unsupported target.

**Examples:**
- `XQL_E401: top-level node must be Program or FunctionDecl`
- `XQL_E401: cannot emit statement for node kind MemberExpr`

**Fix:** Ensure the AST structure matches the expected shape. Top-level nodes should only contain `FunctionDecl` inside `Program`.

### XQL_E402: Unsupported type for target

**Cause:** The AST uses a type that the target language cannot cleanly map.

**Examples:**
- `XQL_E402: C++ target does not support Result<T>`
- `XQL_E402: target "lua" does not support Result<T> type`

**Fix:** Remove or replace the unsupported type, or choose a different target that supports it.

### XQL_E403: MQL unsupported feature

**Cause:** The AST uses a language feature that MQL4/MQL5 does not support. MQL targets only generate script-mode skeletons (`OnStart` entry point, `Print` for output). They do not support Map, Option, Result types or for-each loops.

**Examples:**
- `XQL_E403: MQL does not support Map type`
- `XQL_E403: MQL does not support Option type`
- `XQL_E403: MQL does not support Result type`
- `XQL_E403: MQL does not support for-each loops`

**Fix:** Remove the unsupported type or feature from the AST before targeting MQL. Use `Array<T>` with index-based for-range loops instead of for-each.

---

## General Debugging Tips

1. **Run `validate` first:** `xql validate --file program.xql.json` to catch errors before codegen.
2. **Check `kind` fields:** Every node must have the correct `kind` string.
3. **Match types exactly:** `"Int"` ≠ `"int"` — use PascalCase type names.
4. **Propagate grants:** If function A calls B calls C, A needs all of C's capabilities.
5. **Pure means pure:** A `pure` function cannot call `println`, `printf`, or any function with side effects.
