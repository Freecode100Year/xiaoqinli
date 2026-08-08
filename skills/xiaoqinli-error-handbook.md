# Xiaoqinli Error Handbook

All errors follow the `XQL_Exxx` pattern. Errors halt compilation — no partial output is produced.

## Error Code Ranges

| Range | Category | Phase |
|-------|----------|-------|
| `XQL_E1xx` | Parse / AST errors | Layer 0 |
| `XQL_E2xx` | Type / effect errors | Layer 1 |
| `XQL_E3xx` | Capability errors | Layer 1 |
| `XQL_E4xx` | Codegen errors | Layer 2 |

---

## XQL_E1xx — Parse Errors

### XQL_E101: Invalid JSON or missing fields

**Cause:** The input is not valid JSON, or a required field is missing from a node.

**Examples:**
- `XQL_E101: invalid JSON: unexpected end of JSON input`
- `XQL_E101: node missing 'kind' field`
- `XQL_E101: FunctionDecl missing 'name'`
- `XQL_E101: IfExpr missing 'cond'`
- `XQL_E101: IfExpr missing 'then'`
- `XQL_E101: Lambda param is not an object`
- `XQL_E101: ExternDecl missing 'name'`
- `XQL_E101: ExternDecl "fetch" must not have a body; the host provides the implementation`
- `XQL_E101: extern method "res.json" must be named by the method alone; the receiver is not part of the name`
- `XQL_E101: unknown node kind: FooBar`

**Fix:** Ensure the `.xql.json` file is well-formed and every node has the correct `kind` field and required properties.

### XQL_E404: File not found

**Cause:** The specified `.xql.json` file does not exist.

**Fix:** Check the file path passed to `--file`.

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
- **IfExpr condition type:** IfExpr `cond` must be `Bool`.
- **IfExpr branch type mismatch:** IfExpr `then` and `else` branches must have the same type.
- **Struct field mismatch:** StructLit field value type doesn't match StructDecl field type.
- **Missing struct field:** StructLit is missing a field declared in StructDecl.
- **Array element type:** Array element type doesn't match declared `elemType`.
- **Index type:** Array index must be `Int`.
- **Undefined function:** The callee is not a declared function, a builtin, an
  imported symbol, or a declared extern. Calls into the host platform (`fetch`,
  `time.Sleep`, `document.createElement`) must be declared with `ExternDecl`.
- **Extern argument count / type mismatch:** The call does not match the extern's
  declared signature. Omit `params` entirely to declare a signature the compiler
  does not police.
- **For-range bounds:** `start` and `end` must be `Int`.
- **For-each iterable:** Must be `Array` type.

**Fix:** Align types across declarations, assignments, returns, and function calls.

### XQL_E202: Conflicting global symbol

**Cause:** One name is defined twice in a way the merged program cannot resolve.

**Examples:**
```
XQL_E202: global symbol "load" is defined in multiple files: models.xql and service.xql
extern 'fetch' is also declared as a function in the same program
extern 'fetch' is declared with conflicting signatures across modules
```

**Fix:** Rename the colliding symbol; drop either the `ExternDecl` or the
`FunctionDecl` (a name is provided by the host or by this program, not both);
declare a shared extern once and import it, or make every declaration identical.

### XQL_E203: Effect check failed

**Cause:** A function declares `@effects(["pure"])` but the compiler inferred side effects. An `ExternDecl`'s declared `effects` propagate to its callers, so calling a `network` extern makes the caller impure.

**Example:**
```
function 'compute' declares @effects(["pure"]) but has inferred effect 'state'
```

**Fix:** Either remove the `pure` declaration, or remove the side-effecting call (e.g., `println`) from the function body. Note: effects propagate through IfExpr, Lambda bodies, and all call chains.

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

**Cause:** The code generator encountered a node it cannot handle for the chosen target.

**Examples:**
- `XQL_E401: top-level node must be Program or FunctionDecl`
- `XQL_E401: cannot emit statement for node kind MemberExpr`
- `XQL_E401: C does not support Lambda expressions`
- `XQL_E401: Zig does not support Lambda expressions`
- `XQL_E401: MQL does not support Lambda expressions`
- `XQL_E401: Ada does not support Lambda expressions`
- `XQL_E401: AWK does not support Lambda expressions`
- `XQL_E401: Bash does not support Lambda expressions`
- `XQL_E401: Fortran does not support Lambda expressions`
- `XQL_E401: Pascal does not support Lambda expressions`
- `XQL_E401: Tcl does not support Lambda expressions`
- `XQL_E401: Bash does not support IfExpr in expression context`
- `XQL_E401: Pascal does not support IfExpr (ternary expressions)`
- `XQL_E401: Haskell does not support AssignStmt`
- `XQL_E401: Haskell does not support WhileStmt`

**Fix:** Avoid using unsupported nodes for that target, or choose a different target language.

### XQL_E402: Unsupported type for target

**Cause:** The AST uses a type that the target language cannot cleanly map.

**Examples:**
- `XQL_E402: target "js" does not support Result<T> type`
- `XQL_E402: C does not support Option type`
- `XQL_E402: Fortran target does not support for-each loops`
- `XQL_E402: extern "fetch" is declared only for targets [js ts chrome] and is not available in "go"`

**Affected targets for Result<T>:** js, c, cpp, nim, mql4, mql5.

**Fix:** Remove or replace the unsupported type, or choose a different target.
For the extern case, compile to a target the host actually provides, or widen
the extern's `targets` list — but only if that host really does provide it.

### XQL_E403: MQL unsupported feature

**Cause:** The AST uses a feature that MQL4/MQL5 does not support. MQL targets only generate script-mode skeletons (`OnStart` entry point, `Print` for output).

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
6. **Check Lambda support:** Not all targets support Lambda — see the support matrix in README.
7. **IfExpr branch types:** Both branches of IfExpr must return the same type.
