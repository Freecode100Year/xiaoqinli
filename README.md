# Xiaoqinli (xql)

[![Go Report Card](https://goreportcard.com/badge/github.com/Freecode100Year/xiaoqinli)](https://goreportcard.com/report/github.com/Freecode100Year/xiaoqinli)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](#license)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Freecode100Year/xiaoqinli)](go.mod)

**One AST, 46 target languages.** An AST-First transpiler with type checking, effect inference, and capability-based security enforced at compile time.

*[中文文档](README.zh-CN.md)*

---

## What is AST-First?

You do not write text source code. You write a `.xql.json` file — an explicit abstract syntax tree:

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
      "body": [
        {
          "kind": "ReturnStmt",
          "value": {
            "kind": "BinaryExpr",
            "op": "+",
            "left": { "kind": "Literal", "valueType": "String", "value": "Hello, " },
            "right": { "kind": "Ident", "name": "name" }
          }
        }
      ]
    }
  ]
}
```

The compiler type-checks it, infers effects, verifies capability grants, links imported modules, and then emits source code for any of 46 backends.

**Why this shape?** Writing one AST instead of 46 dialects keeps semantics identical across targets. And for LLM-driven code generation, emitting a structured tree that a compiler will *reject* when wrong is far more reliable than emitting 46 flavours of surface syntax that only fail at runtime.

---

## Quick start

```bash
go build -o xql .
```

Validate a program without generating anything:

```console
$ ./xql validate --file examples/hello.xql.json
ok: all checks passed
```

Compile the same AST to different languages:

```console
$ ./xql compile --file examples/hello.xql.json --target go
package main

import "fmt"

func greet(name string) string {
    return "Hello, " + name
}

func main() {
    fmt.Println(greet("World"))
}
```

```console
$ ./xql compile --file examples/hello.xql.json --target py
def greet(name: str) -> str:
    return ("Hello, " + name)


def main() -> None:
    print(greet("World"))


if __name__ == "__main__":
    main()
```

Write to a file with `--out`, and list every backend with `./xql targets`.

---

## Capability security

Every function declares the effects it performs and the capabilities it is granted. A caller must hold a superset of its callee's grants, checked **before** any code is generated.

Given `readSecret` declared with `"grant": ["io"]`, and a `main` that calls it with an empty grant list:

```console
$ ./xql compile --file cap_demo.xql.json --target go
[
  {
    "code": "XQL_E301",
    "message": "function 'main' calls 'readSecret' but lacks required capabilities: [io]",
    "suggested_fix": "Add the missing capability name to the caller function's @grant list.",
    "level": "error"
  }
]
$ echo $?
2
```

Capability strings support wildcards (`network:*`). Grants are verified across the whole import graph, not just the entry file — moving privileged code into a module does not bypass the check.

Built-in effects: `pure`, `network`, `filesystem`, `state`.

Strict capability checking is on by default for `compile`; pass `--no-strict-caps` to relax it.

---

## Host functions

A program that never leaves the process is not much of a program. Calls into the
host platform — `fetch`, `time.Sleep`, `document.createElement` — are declared
with `ExternDecl`:

```json
{
  "kind": "ExternDecl",
  "name": "fetch",
  "params": [{ "name": "url", "type": { "kind": "String" } }],
  "returnType": { "kind": "String" },
  "effects": ["network"],
  "grant": ["network"],
  "targets": ["js", "ts", "chrome"]
}
```

The name is matched verbatim against the callee, so dotted names like
`time.Sleep` and `document.head.appendChild` are declared exactly as they are
called. An extern has no body: the compiler emits the call and nothing else.

This is where capability security earns its keep. A host call is the one edge
that actually reaches the outside world, and every caller must hold the extern's
grant:

```console
$ ./xql compile --file clock.xql.json --target go
[
  {
    "code": "XQL_E301",
    "message": "function 'main' calls 'time.Sleep' but lacks required capabilities: [clock]",
    ...
  }
]
```

The declared `effects` also propagate: a function marked `pure` that calls a
`network` extern is rejected with `XQL_E203`.

**`targets`** restricts an extern to the backends whose host actually provides
it. Compiling a program that calls a browser API to a target that has never
heard of it is a compile error (`XQL_E402`) rather than output that only breaks
when someone runs it. Omit the field to allow every target.

**`params`** may be omitted entirely to declare an unchecked signature, for host
functions that are variadic or overloaded. Declared params are checked for
arity and argument types like any other call.

**Methods.** When the receiver is a runtime value the compiler cannot type —
`res.json()`, `hud.classList.add()` — declare the method instead, and it matches
any receiver:

```json
{ "kind": "ExternDecl", "name": "json", "method": true, "effects": ["network"], "grant": ["network"] }
```

The receiver is not verified, but the grant is still enforced at every call site.

Externs are not namespaced by their module: declare a platform's surface once
and import it, and the names stay callable as-is. Modules that declare the same
extern identically are merged; declarations that disagree are rejected
(`XQL_E202`).

---

## Supported targets (46)

| Category | Targets |
|---|---|
| Systems | `go` `rust` `c` `cpp` `zig` `nim` `d` `v` `ada` `fortran` `pascal` |
| JVM / .NET | `java` `kotlin` `scala` `groovy` `clojure` `csharp` `fsharp` |
| Scripting | `py` `js` `ts` `ruby` `php` `perl` `lua` `tcl` `awk` |
| Functional | `haskell` `ocaml` `elixir` `julia` `crystal` |
| Apple | `swift` `objc` `ios` `shortcut` |
| Shell | `bash` `powershell` `bat` |
| Mobile / Web | `android` `chrome` |
| Domain-specific | `mql4` `mql5` `vala` `tccli` |

`android` and `ios` emit multi-file project scaffolds (Gradle / Swift Package Manager) rather than a single source file.

**How each target is verified.** Advertising forty-six backends says nothing
about how well any one of them works. This table is generated from
`compiler/verification.go`, and the test suite fails if it drifts from the tier
the tests actually enforce.

<!-- verification:begin -->
| Evidence | Targets | What was checked |
|---|---|---|
| **executed** (14) | `csharp` `dart` `go` `java` `julia` `kotlin` `lua` `php` `py` `ruby` `rust` `swift` `ts` `zig` | compiled and run, stdout asserted |
| **smoke** (32) | `ada` `android` `awk` `bash` `bat` `c` `chrome` `clojure` `cpp` `crystal` `d` `elixir` `fortran` `fsharp` `groovy` `haskell` `ios` `js` `mql4` `mql5` `nim` `objc` `ocaml` `pascal` `perl` `powershell` `scala` `shortcut` `tccli` `tcl` `v` `vala` | codegen returns output; never compiled |

CI installs 14 toolchains for the executed tier and sets `XQL_E2E_REQUIRE=1`,
so a missing one fails the run instead of skipping quietly.

Per-target caveats:

- `android` — Gradle scaffold; structure checked, never assembled
- `bat` — rejects struct literals
- `c` — rejects Result<T>
- `chrome` — emits an extension bundle
- `cpp` — rejects Result<T>
- `fortran` — rejects for-each loops
- `ios` — SwiftPM scaffold; built only when swift is present
- `js` — rejects Result<T>
- `mql4` — rejects Result<T>, maps, Option, for-each
- `mql5` — rejects Result<T>, maps, Option, for-each
- `nim` — rejects Result<T>
- `pascal` — rejects for-each loops
- `shortcut` — emits an Apple Shortcuts plist, not source
<!-- verification:end -->

**Known limitations.** A backend that cannot express a construct rejects it
rather than silently degrading it:

| Construct | Rejected by |
|---|---|
| `Result<T>` | `js` `c` `cpp` `nim` `mql4` `mql5` |
| for-each loops | `fortran` `pascal` |
| struct literals | `bat` |

All other targets carry real `Result` semantics. The `kotlin` and `android`
backends emit their own two-parameter `Result<T, E>` into the generated file's
package, which shadows the default-imported `kotlin.Result<out T>`; without it
the Gradle build fails with "One type argument expected".

No CI job assembles an APK — that needs an Android SDK and an AGP/Gradle/Kotlin
version triple that drifts with the runner image. The `android` scaffold is
verified structurally, and the Kotlin it emits is verified by the `kotlin`
end-to-end test, which compiles and runs the same generated constructs.

---

## Multi-file projects

Split a program across files and wire them with `ImportDecl`:

```json
{ "kind": "ImportDecl", "path": "./models.xql", "as": "models" }
```

At compile time a linker resolves the import graph, merges every module into a single self-contained `Program`, and strips the now-meaningless alias qualifiers. Backends therefore only ever see a flat program — the path they are already well tested on.

Import cycles are rejected with `XQL_E402`, and cross-file symbol collisions are rejected before merging. See `examples/e2e_workspace/` for a working three-file program.

`ExternDecl` is the exception to alias stripping and to namespacing: a host name
is one verbatim symbol, and every module that imports the declaring module can
call it under that same name.

---

## Integration

### CLI

```
xql compile --file <path.xql.json> --target <lang> [--out <output>] [--no-strict-caps]
xql validate --file <path.xql.json>
xql targets                          List all supported target languages
xql stdio                            MCP stdio mode
xql http [<:port>] [--mode rest]     MCP / REST HTTP mode (default :8080)
```

Exit codes: `0` success · `1` validation failed · `2` compilation error · `3` argument error.

### MCP server

`xql stdio` speaks the Model Context Protocol over stdin/stdout, so an agent can drive the compiler directly. `xql http` serves the same tools over HTTP.

Tools exposed: `compile`, `validate`, `targets`, `specs_inspect`, `specs_update`, `stdlib_matrix_inspect`, `stdlib_matrix_update`, `treesitter_mapping_inspect`, `treesitter_mapping_update`, `diagnostic_memory_inspect`, `diagnostic_memory_record`, `security_policy_inspect`, `codegen_strategy_inspect`, `codegen_strategy_update`, `skills_diagnose_and_fill`, `agent_search_query`, `agent_search_autoupdate`.

### REST API

`xql http :8080 --mode rest`

| Method | Endpoint | Purpose |
|---|---|---|
| POST | `/compile` | Compile an AST to a target language |
| POST | `/validate` | Run all semantic checks only |
| GET/POST | `/specs` | Inspect / update language profiles |
| GET/POST | `/codegen/strategy` | Inspect / update codegen strategy |
| GET/POST | `/evolution/diagnostics` | Inspect / record learned diagnostic fixes |
| GET/POST | `/api/v1/search` | Query the agent search index |
| POST | `/api/v1/search/autoupdate` | Rebuild the search index |
| GET | `/skills/` | Fetch embedded skill documents |
| GET | `/health` | Liveness and version |
| GET | `/metrics` | Prometheus metrics |

`/metrics` returns a stub unless built with `-tags metrics`.

---

## Docker

```bash
docker compose up --build
```

The image builds a static binary and ships it alongside Python, Node.js, and Go toolchains so generated code can be executed in the sandbox. The MCP HTTP server listens on `:8080`.

---

## Project layout

```
xiaoqinli/
  main.go          CLI entry point
  ast/             AST node definitions, JSON parser, stable binary codec
  check/           Type checker, effect inference, capability verifier
  compiler/        Public library API: parse → check → link → codegen
  codegen/         46 language backends + dispatch
  evolution/       Self-evolution state: diagnostic memory, skills, search index
  server/          MCP (stdio + HTTP) and REST servers
  vfs/             Session-scoped in-memory filesystem
  skills/          Embedded skill documents (go:embed)
  remedy/          Defect remediation helpers
  examples/        Sample .xql.json programs
  docs/            Architecture decision records
```

---

## Development

```bash
go build ./...                # build everything
go vet ./...                  # static analysis
go test ./...                 # full test suite
go test -tags metrics ./...   # with Prometheus metrics enabled
```

The end-to-end suite in `codegen/local_e2e_test.go` compiles the sample workspace and *executes* the result with real toolchains (Ruby, Lua, PHP, Java, …). Each language is skipped automatically when its toolchain is absent, so a partial local environment still yields a green run.

Importing the compiler as a library:

```go
import "xiaoqinli/compiler"

result := compiler.CompileFromFile("app.xql.json", "go", "")
if !result.Success {
    log.Fatal(result.Error)
}
fmt.Println(string(result.Code))
```

---

## License

Released under the MIT License.
