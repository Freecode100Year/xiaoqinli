# Xiaoqinli (xql)

[![Go Report Card](https://goreportcard.com/badge/github.com/Freecode100Year/xiaoqinli)](https://goreportcard.com/report/github.com/Freecode100Year/xiaoqinli)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](#开源协议)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Freecode100Year/xiaoqinli)](go.mod)

**一份 AST，38 个目标语言。** AST-First 转译器，在编译期完成类型检查、效果推断与基于能力的安全校验。

*[English](README.md)*

---

## 什么是 AST-First

你写的不是文本源码，而是一个 `.xql.json` 文件——一棵显式的抽象语法树：

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

编译器对它做类型检查、效果推断、能力校验、链接被导入的模块，然后生成 38 个后端中任意一个的源码。

**为什么这样设计？** 写一棵 AST 而不是 38 种方言，能让语义在所有目标间保持一致。而对于 LLM 驱动的代码生成，输出一棵**写错就会被编译器拒绝**的结构化树，远比输出 38 种只在运行时才暴露问题的表层语法可靠。

---

## 快速开始

```bash
go build -o xql .
```

只做校验、不生成任何代码：

```console
$ ./xql validate --file examples/hello.xql.json
ok: all checks passed
```

把同一份 AST 编译到不同语言：

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

用 `--out` 写入文件，用 `./xql targets` 列出全部后端。

---

## 能力安全

每个函数都要声明自己执行的效果与被授予的能力。调用方必须持有被调用方能力的超集，且校验发生在**生成任何代码之前**。

假设 `readSecret` 声明了 `"grant": ["io"]`，而 `main` 以空 grant 列表调用它：

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

能力字符串支持通配符（`network:*`）。校验覆盖整个导入图而不只是入口文件——把越权代码挪进模块并不能绕过检查。

内置效果：`pure`、`network`、`filesystem`、`state`。

`compile` 默认开启严格能力检查，可用 `--no-strict-caps` 放宽。

---

## 宿主函数

完全不与外界交互的程序算不上程序。对宿主平台的调用——`fetch`、`time.Sleep`、`document.createElement`——用 `ExternDecl` 声明：

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

名字与被调用方逐字匹配，所以 `time.Sleep`、`document.head.appendChild` 这类带点的名字就按调用时的写法声明。extern 没有函数体：编译器只生成调用本身，不生成任何别的东西。

能力安全的价值正体现在这里。宿主调用是真正伸向外部世界的那一条边，任何调用方都必须持有该 extern 的授权：

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

声明的 `effects` 同样会向上传播：标注为 `pure` 的函数若调用了带 `network` 效果的 extern，会被 `XQL_E203` 拒绝。

**`targets`** 把 extern 限定在真正提供它的后端上。把调用浏览器 API 的程序编译到根本没听说过该 API 的目标，会直接报编译错误（`XQL_E402`），而不是产出运行时才崩的代码。省略该字段表示所有目标都可用。

**`params`** 可以整个省略，表示签名不做检查，用于变参或重载的宿主函数。一旦声明了 params，参数个数与类型就与普通调用一样受检。

**方法。** 当接收者是编译器无法定型的运行时值时——`res.json()`、`hud.classList.add()`——改为声明方法，它可匹配任意接收者：

```json
{ "kind": "ExternDecl", "name": "json", "method": true, "effects": ["network"], "grant": ["network"] }
```

接收者不受验证，但每个调用点仍会强制检查授权。

extern 不按模块划分命名空间：把一个平台的接口面声明一次并导入即可，名字照样可直接调用。多个模块以完全相同的方式声明同一个 extern 时会被合并；声明不一致则被拒绝（`XQL_E202`）。

---

## 支持的目标平台（38 种）

| 分类 | 目标 |
|---|---|
| 系统级 | `go` `rust` `c` `cpp` `zig` `nim` `d` `fortran` `pascal` |
| JVM / .NET | `java` `kotlin` `groovy` `csharp` |
| 脚本 | `py` `js` `ts` `ruby` `php` `perl` `lua` `tcl` `awk` |
| 函数式 | `haskell` `ocaml` `elixir` `julia` `crystal` |
| Apple 生态 | `swift` `ios` `shortcut` |
| Shell | `bash` `powershell` `bat` |
| 移动 / Web | `android` `chrome` |
| 领域专用 | `vala` `tccli` |

`android` 与 `ios` 输出的是多文件工程脚手架（Gradle / Swift Package Manager），而非单个源文件。

**每个目标各自的验证强度。** 宣称支持三十八个后端，并不说明其中任何一个的成色。
下表由 `compiler/verification.go` 生成，一旦它与测试实际强制的等级脱节，测试就会失败。

<!-- verification:begin -->
| Evidence | Targets | What was checked |
|---|---|---|
| **executed** (32) | `awk` `bash` `c` `cpp` `crystal` `csharp` `d` `dart` `elixir` `fortran` `go` `groovy` `haskell` `java` `js` `julia` `kotlin` `lua` `nim` `ocaml` `pascal` `perl` `php` `powershell` `py` `ruby` `rust` `swift` `tcl` `ts` `vala` `zig` | compiled and run, stdout asserted |
| **compiled** (1) | `tccli` | compiled by a real toolchain |
| **smoke** (5) | `android` `bat` `chrome` `ios` `shortcut` | codegen returns output; never compiled |

The executed tier needs 32 toolchains, which CI installs or inherits from the
runner image, and it sets `XQL_E2E_REQUIRE=1` so a missing one fails the run
instead of skipping quietly.

Per-target caveats:

- `android` — Gradle scaffold; structure checked, never assembled
- `awk` — rejects Result<T> and struct literals
- `bash` — rejects Result<T>
- `bat` — rejects struct literals
- `c` — rejects Result<T>
- `chrome` — emits an extension bundle; rejects Result<T>
- `cpp` — rejects Result<T>
- `crystal` — rejects Result<T>
- `d` — rejects Result<T>
- `elixir` — rejects Result<T>
- `fortran` — rejects for-each loops
- `groovy` — rejects Result<T>
- `haskell` — rejects Result<T>
- `ios` — SwiftPM scaffold; built only when swift is present
- `js` — rejects Result<T>
- `nim` — rejects Result<T>
- `ocaml` — rejects Result<T>
- `pascal` — rejects for-each loops
- `perl` — rejects Result<T>
- `powershell` — rejects Result<T>
- `shortcut` — emits an Apple Shortcuts plist, not source; rejects Result<T>
- `tccli` — emits Tencent Cloud CLI shell; no arithmetic, comparisons or structs
- `tcl` — rejects Result<T>
- `vala` — rejects Result<T>
<!-- verification:end -->

**已知限制。** 后端无法表达的构造会被明确拒绝，而不是悄悄降级：

| 构造 | 拒绝它的目标 |
|---|---|
| `Result<T, E>` | 除下列 16 个以外的全部目标 |
| for-each 循环 | `fortran` `pascal` |
| 结构体字面量 | `bat` `awk` `tccli` |
| 算术、比较、数组 | `tccli` |

`Result<T, E>` 是实现率最低的构造，只有 16 个后端真正支持：

`go` `rust` `ts` `py` `java` `csharp` `kotlin` `swift` `dart` `lua` `ruby` `php` `zig` `julia` `android` `ios`

另外 27 个此前会「编译成功」，产出的却是引用了自己从未定义的 `Result` 的代码——`Result.ok(users)` 和
`res.unwrap()` 原封不动照抄 AST，在 Haskell 里指向不存在的模块，在 PowerShell 里指向不存在的命令。它们现在
直接拒绝。这不是收回能力，那些产物本来就跑不起来。详见
[docs/breaking_changes.md](docs/breaking_changes.md)。

其余目标均携带真实的 `Result` 语义。`kotlin` 与 `android` 后端会把自己的双参数 `Result<T, E>` 发到生成文件所在的包里，以遮蔽默认导入的 `kotlin.Result<out T>`；缺了它，Gradle 构建会报 "One type argument expected"。

CI 不会真的打出 APK——那需要 Android SDK，以及随 runner 镜像漂移的 AGP/Gradle/Kotlin 版本组合。`android` 脚手架按结构验证，它发出的 Kotlin 则由 `kotlin` 端到端测试负责验证，那条链路会真正编译并运行同样的生成构造。

---

## 多文件工程

把程序拆到多个文件，用 `ImportDecl` 连接：

```json
{ "kind": "ImportDecl", "path": "./models.xql", "as": "models" }
```

编译时链接器会解析导入图，把所有模块合并成一个自足的 `Program`，并剥离已失去意义的别名限定符。因此后端永远只看到一个扁平程序——正是它们已被充分测试的路径。

导入成环会报 `XQL_E402`，跨文件符号冲突在合并前就被拒绝。可运行的三文件示例见 `examples/e2e_workspace/`。

`ExternDecl` 是别名剥离与命名空间划分的例外：宿主名字是一个逐字的符号，导入了声明模块的任何模块都能以同一名字调用它。

---

## 接入方式

### 命令行

```
xql compile --file <path.xql.json> --target <lang> [--out <output>] [--no-strict-caps]
xql validate --file <path.xql.json>
xql targets                          列出所有支持的目标语言
xql stdio                            MCP stdio 模式
xql http [<:port>] [--mode rest]     MCP / REST HTTP 模式（默认 :8080）
```

退出码：`0` 成功 · `1` 校验失败 · `2` 编译错误 · `3` 参数错误。

### MCP 服务端

`xql stdio` 通过 stdin/stdout 讲 Model Context Protocol，Agent 可以直接驱动编译器。`xql http` 以 HTTP 提供同一组工具。

已暴露的工具：`compile`、`validate`、`targets`、`specs_inspect`、`specs_update`、`stdlib_matrix_inspect`、`stdlib_matrix_update`、`treesitter_mapping_inspect`、`treesitter_mapping_update`、`diagnostic_memory_inspect`、`diagnostic_memory_record`、`security_policy_inspect`、`codegen_strategy_inspect`、`codegen_strategy_update`、`skills_diagnose_and_fill`、`agent_search_query`、`agent_search_autoupdate`。

### REST API

`xql http :8080 --mode rest`

| 方法 | 端点 | 用途 |
|---|---|---|
| POST | `/compile` | 把 AST 编译到目标语言 |
| POST | `/validate` | 只跑全部语义检查 |
| GET/POST | `/specs` | 查看 / 更新语言画像 |
| GET/POST | `/codegen/strategy` | 查看 / 更新代码生成策略 |
| GET/POST | `/evolution/diagnostics` | 查看 / 记录学习到的修复经验 |
| GET/POST | `/api/v1/search` | 查询 Agent 检索索引 |
| POST | `/api/v1/search/autoupdate` | 重建检索索引 |
| GET | `/skills/` | 获取内置技能文档 |
| GET | `/health` | 存活状态与版本 |
| GET | `/metrics` | Prometheus 指标 |

除非用 `-tags metrics` 构建，否则 `/metrics` 返回的是占位响应。

---

## Docker

```bash
docker compose up --build
```

镜像会构建静态二进制，并随附 Python、Node.js、Go 工具链，使生成的代码可以在沙箱内直接执行。MCP HTTP 服务监听 `:8080`。

---

## 项目结构

```
xiaoqinli/
  main.go          命令行入口
  ast/             AST 节点定义、JSON 解析器、稳定二进制编解码
  check/           类型检查器、效果推断、能力校验器
  compiler/        公共库 API：解析 → 检查 → 链接 → 代码生成
  codegen/         38 个语言后端与派发
  evolution/       自演化状态：诊断记忆、技能、检索索引
  server/          MCP（stdio + HTTP）与 REST 服务端
  vfs/             会话级内存文件系统
  skills/          内置技能文档（go:embed）
  remedy/          缺陷修复辅助
  examples/        示例 .xql.json 程序
  docs/            架构决策记录
```

---

## 开发

```bash
go build ./...                # 构建全部
go vet ./...                  # 静态分析
go test ./...                 # 完整测试套件
go test -tags metrics ./...   # 启用 Prometheus 指标
```

`codegen/local_e2e_test.go` 中的端到端套件会编译示例工程并用**真实工具链实际运行**产物（Ruby、Lua、PHP、Java……）。缺少某语言工具链时会自动跳过，因此本地环境不全也能跑出全绿。

`compiler/conformance_test.go` 问的是更难的那个问题：不是每个后端有没有生成**一个**程序，而是它们生成的是不是**同一个**程序。它拿一批输出已知的示例，在本机能跑的每种语言里实际运行，逐行比对 stdout。「一份 AST，38 个目标」只能是这个意思。八个后端的 range 循环多跑一轮——Go 里 panic、Python 里 `IndexError`、JavaScript 里 `NaN`，而 C 和 Lua 打印的是对的——正是靠它才终于暴露出来。

以库的形式引入编译器：

```go
import "xiaoqinli/compiler"

result := compiler.CompileFromFile("app.xql.json", "go", "")
if !result.Success {
    log.Fatal(result.Error)
}
fmt.Println(string(result.Code))
```

---

## 开源协议

本项目采用 MIT 开源协议。
