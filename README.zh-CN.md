# Xiaoqinli (xql)

[![Go Report Card](https://goreportcard.com/badge/github.com/Freecode100Year/xiaoqinli)](https://goreportcard.com/report/github.com/Freecode100Year/xiaoqinli)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](#开源协议)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Freecode100Year/xiaoqinli)](go.mod)

**一份 AST，46 个目标语言。** AST-First 转译器，在编译期完成类型检查、效果推断与基于能力的安全校验。

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

编译器对它做类型检查、效果推断、能力校验、链接被导入的模块，然后生成 46 个后端中任意一个的源码。

**为什么这样设计？** 写一棵 AST 而不是 46 种方言，能让语义在所有目标间保持一致。而对于 LLM 驱动的代码生成，输出一棵**写错就会被编译器拒绝**的结构化树，远比输出 46 种只在运行时才暴露问题的表层语法可靠。

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

## 支持的目标平台（46 种）

| 分类 | 目标 |
|---|---|
| 系统级 | `go` `rust` `c` `cpp` `zig` `nim` `d` `v` `ada` `fortran` `pascal` |
| JVM / .NET | `java` `kotlin` `scala` `groovy` `clojure` `csharp` `fsharp` |
| 脚本 | `py` `js` `ts` `ruby` `php` `perl` `lua` `tcl` `awk` |
| 函数式 | `haskell` `ocaml` `elixir` `julia` `crystal` |
| Apple 生态 | `swift` `objc` `ios` `shortcut` |
| Shell | `bash` `powershell` `bat` |
| 移动 / Web | `android` `chrome` |
| 领域专用 | `mql4` `mql5` `vala` `tccli` |

`android` 与 `ios` 输出的是多文件工程脚手架（Gradle / Swift Package Manager），而非单个源文件。

**已知限制：** `js` 与 `nim` 不支持 `Result<T>` 类型，遇到时会明确报 `XQL_E402` 而不是悄悄降级。其余目标均携带真实的 `Result` 语义。

---

## 多文件工程

把程序拆到多个文件，用 `ImportDecl` 连接：

```json
{ "kind": "ImportDecl", "path": "./models.xql", "as": "models" }
```

编译时链接器会解析导入图，把所有模块合并成一个自足的 `Program`，并剥离已失去意义的别名限定符。因此后端永远只看到一个扁平程序——正是它们已被充分测试的路径。

导入成环会报 `XQL_E402`，跨文件符号冲突在合并前就被拒绝。可运行的三文件示例见 `examples/e2e_workspace/`。

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
  codegen/         46 个语言后端与派发
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
