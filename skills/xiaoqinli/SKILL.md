---
name: xiaoqinli
description: "Universal AI Agent Architecture, Schema, and Integration Guide for Xiaoqinli (xql) AST-First Transpiler v3.24.0."
version: 3.24.0
author: Freecode100Year
platforms: [linux, macos, windows]
metadata:
  hermes:
    tags: [xql, xiaoqinli, compiler, architecture, transpiler, go, mcp, agents]
---

# Xiaoqinli (xql) 极简安全转译器 v3.24.0 — 自我更新进化后强制 Debug 与自检宪法

> **定位**：AST-First 安全转译器（Transpiler）· 单 Go 二进制 · 零依赖 · 无运行时 · 极简 / 安全 / 高性能
> **版本**：v3.24.0 (自我更新进化后强制 Debug 物理防灾版)
> **支持 Agent & 模型**：Google Antigravity, Kimi Code (Moonshot), Qwen Code (Qwen2.5-Coder), DeepSeek Coder/V3, GLM Coding (GLM-4), Official Tencent Cloud CLI (tccli), Claude Code, Cursor, Windsurf, Aider 等

---

## 🤖 全 AI Agent 框架强对齐协议 (Multi-Agent Alignment Protocol)

所有对接 `xiaoqinli` 的 AI Agent 系统必须遵循以下核心规训：

1. **编译器内核为唯一真相来源（Compiler Core Single Source of Truth）**：
   - 所有的类型检查、副作用 Effect 审计与 `@grant` 权限校验由原生 Go 编译器闭环，禁止 Agent 在运行期使用 LLM 调用或盲目猜度。
2. **AST-First 强约束**：
   - AI Agent 必须直接输出 `.xql.json` 格式的结构化 JSON AST，从物理根源上消灭排版错乱与词法解析错误。
3. **生成前最新语言特性检索与本地自我更新协议 (Pre-Retrieval & Self-Updating Spec Protocol)**：
   - 在为目标语言（Python 3.12+/3.13+ 及其他 42+ 语言）生成代码前，Agent 必须通过 MCP `specs_inspect` 或 REST `/specs` 检索最新语言特性 Profile。
   - 若检测到新语法/新规范，Agent 可主动调用 `specs_update` 或更新本地 Profile JSON 实现动态自我演进，确保生成的代码始终适用于最新的语言版本与特性。
4. **零崩溃与死循环物理拦截宪法 (Zero-Crash & Deadloop Boundary Protocol)**：
   - 所有的自我更新、动态演化与语法树解析必须封装 `SafeExecute` (Panic Shield)，出现非预期异常时优雅降级并输出结构化错误，保证程序 0 Panic Crash。
   - 所有的自我纠错重试与图闭环挂载 `LoopBreaker`，设置 `MaxSelfEvolutionRetries = 3` 与 `MaxRecursionDepth = 64` 物理阻断，拦截任何可能引发无限循环的图回路。
5. **自我更新进化后强制 Debug 校验协议 (Post-Evolution Mandatory Auto-Debug Protocol)**：
   - **任何时候触发自我更新迭代后**，系统/Agent 必须自动启动 Debug 自检流程：执行 `gofmt` 格式化、`go test ./...` 100% 物理测试通过、重新编译 `xql.exe` 并覆盖宿主机全局路径 `C:\Users\sj929\go\bin\xql.exe`、更新 `README.md` 并提交 GitHub！
6. **云原生 CLI & 大模型原生生态强对齐 (Cloud Native CLI & LLM Model Alignment)**：
   - **Official Tencent Cloud CLI (`tccli`)**：原生支持将 `.xql.json` 编译导出为腾讯云官方 CLI 自动化运维与云原生资源编排 Bash 脚本（Target: `tccli`）。
   - **Kimi Code / Qwen Code / DeepSeek Coder / GLM Coding**：全面支持长上下文 Prompt Caching、FIM (Fill-In-Middle) 补全、Tool Calling/MCP 标准对接与结构化 JSON AST 直接生成。
7. **目标后端 Tier 级治理分工**：
   - **Tier A (核心主力)**：`Go` | `Rust` | `TypeScript` | `Python` | `C++` | `Java` | `C#` | `Zig` | `Tencent Cloud CLI (tccli)` (保证 100% 物理跑通)
   - **Tier B (主流扩展)**：`Swift` | `Kotlin` | `Dart` | `PHP` | `Ruby` | `Lua` | `Shell/Bash` | `PowerShell`
   - **Tier C (长尾小众)**：`Ada` | `Bat` | `Tcl` | `Fortran` | `Pascal` 等 (稳态 Freeze)

---

## 设计宪法：三要求优先于一切

本项目的每个设计决策都必须同时满足以下三条，违反任意一条的特性一律不进核心（功能丰富度永远让位于三要求，做减法不做加法） ：

* **极简（Minimal）**：单语言、单二进制、最少工具、最少依赖。
* **安全（Secure）**：所有检查在编译期完成，产出物确定，无运行期不确定性。
* **高性能（Fast）**：转译器无运行时开销，毫秒级响应。

---

## v1.4 vs v2.0 减法清单

| 砍掉的特性 | 原因 | 最终去向 |
|---|---|---|
| **TypeScript 编译器核心** | 双语言栈违背极简，维护两套构建+桥接层太重 | 全部用 Go 重写 |
| **Aether VM + 字节码** | 转译器不需要运行时，多一层就慢一层 | 删除，直接 codegen |
| **认知原语 `predict/embed/<=>`** | 运行期调 LLM = 不确定性 + 网络依赖 + token 成本，与安全/高性能冲突 | 移到 v3 可选插件，不进核心 |
| **自愈引擎 healer** | 运行期改代码 = 不可控，违背产出确定原则 | 删除 |
| **LSP** | 非核心，优先级低 (P3) | 推迟到工具链成熟后 |
| **IR DAG 中中间层** | 多余的转译层 | 删除，AST 直接到目标代码 |
| **10 个 packages** | 过度模块化 | 收敛到 6 个 Go 包 |

> **保留并强化**：AST-First 存储、内容寻址、类型+效果检查、能力安全、MCP/REST/Skills 三通道接入。

---

## 1. 核心定位

Xiaoqinli 是一个极简、安全、高性能的 **AST-First 转译器**。

* **输入**：AI 直接编写结构化的 `.xql.json`（物理上天然无语法错误）。
* **处理**：编译器做类型与能力的静态检查。
* **输出**：直接输出目标语言源码（Go / Rust / TS / Kotlin / Swift / Python / Java / C# / Dart / Lua / Ruby / PHP / Zig / Nim / Julia）。
* **原则**：**无 VM，无运行时，无运行期 LLM 调用。**

### 1.1 是与不是 (Is / Is Not)

* **是**：转译器 (source → source) | AST-First (代码即结构化数据) | 编译期全静态检查 | AI Agent 的编程目标 | 单 Go 二进制。
* **不是**：解释器/虚拟机 | 文本编程语言 | 运行期动态检查 | 人类日常手写语言 | 多语言工具链。

---

## 2. 转译流水线 (2-Layer Pipeline)

从 5 层简化为 2 个转译层：

1. **输入**：`.xql.json` (AST，AI 直接写的结构化数据)
2. **层 1：静态检查**
   * 类型检查
   * 效果推断
   * 能力检查 (`@grant`)
   * *全部通过才进入下一层*
3. **层 2：代码生成**
   * AST 直接遍历 → 目标语言源码字符串
4. **输出**：42+ 种目标语言/平台源码（Go / Rust / TS / Python / C++ / Java / C# / Zig / Kotlin / Swift / Dart / Lua / Ruby / PHP / Nim / Julia / Scala / Haskell / Bash / PowerShell / Chrome Extension 等）

> **对比 v1.4**：删除了 IR DAG、字节码、VM 三层。中间不落任何中间表示，AST 检查通过后直接 codegen，走最快路径。

---

## 3. 单二进制结构 (Minimal Codebase)

整个项目收敛为单一 Go module，仅包含 **6 个 Go 包**：

```text
xiaoqinli/                      # 单一 Go module
  main.go                     # 入口：模式路由（stdio / http / cli）
  ast/
    nodes.go                # AST 节点定义
    hash.go                 # 内容寻址哈希
  check/
    types.go                # 类型检查 + 效果推断
    capability.go           # 能力检查（@grant 验证）
  codegen/
    golang.go               # Go
    rust.go                 # Rust
    typescript.go           # TypeScript
    kotlin.go               # Kotlin
    swift.go                # Swift
    python.go               # Python
    java.go                 # Java
    csharp.go               # C#
    dart.go                 # Dart
    lua.go                  # Lua
    ruby.go                 # Ruby
    php.go                  # PHP
    zig.go                  # Zig
    nim.go                  # Nim
    julia.go                # Julia
    util.go                 # Generate() dispatcher + shared utilities
  server/
    mcp.go                  # MCP（stdio + streamable HTTP）
    rest.go                 # REST API
    skills.go               # Skills 分发（prompts/* + /skills/*）
  vfs/
    workspace.go            # 会话内存文件系统
  skills/                     # Skill markdown（编译进二进制，go:embed）
    xiaoqinli-usage-guide.md
    xiaoqinli-error-handbook.md
```

* **真正零第三方依赖**：流式 HTTP 和 SSE 均基于 Go 标准库 `net/http` 实现。
* **部署极简**：Skills 采用 `go:embed` 打进二进制，无额外文件依赖，`go build` 即可交付。

---

## 4. 安全：编译期静态保证

```
【核心原则】
编译器接受 .xql.json，输出确定的目标代码。
没有运行期 LLM、没有运行期 code 改写、没有动态加载。
给定相同输入，永远产出相同输出（确定性 = 可审计）。
```

### 三道静态检查（全部在层 1 拦截）

1. **类型检查**：所有变量、函数签名、返回值类型在编译期验证。类型不匹配抛出 `XQL_E2xx` 错误，拒绝 codegen。
2. **效果推断**：自动推断函数的副作用（network / filesystem / state）。声明 `@effects(["pure"])` 的纯函数若含副作用引发编译错误。保证可缓存、可并行、可推理。
3. **能力检查（能力安全机制）**：函数通过 `@grant([...])` 声明能力。被调函数的能力必须是调用者的**子集**（能力继承规则）。使用未声明的能力直接引发 `XQL_E3xx` 编译熔断。

---

## 5. 高性能：无运行时设计

1. **无运行时**：转译器输出源码后即结束生命周期，不参与目标代码的执行。
2. **单次遍历**：AST  → 目标代码是一次深度优先遍历 + 字符串拼接。
3. **无中间表示**：删除中间层，最快路径直达目标。
4. **Go 语言实现**：编译型语言，短生命周期请求，无 GC 停顿影响。
5. **内存 VFS**：源码常驻内存空间，除显式持久化外，无磁盘 I/O 损耗。

---

## 6. .xql 语言规范：双视图 design

* **核心决策**：为保持极简，**编译器只接受 `.xql.json` (AST) 作为输入**。
* **人类视图**：`.xql`（人类文本视图）是 AST 单向渲染的**只读**产物。
* **不内置 Parser**：编译器**不内置** `.xql` ──> AST 的逆向解析器。
  1. AI Agent 作为主要用户，直接产出 AST，天然无需文本解析。
  2. 省掉 parser 包等于极致精简，并从根源上消除了文本语法错误。
  3. *注：若未来需要人类手写，解析器将作为 P3 级别独立工具开发，不进入核心二进制。*

---

## 7. MCP / REST / Skills 三通道接入

保留 v1.4 的三通道接入能力：

* **MCP (Model Context Protocol)**：面向 Claude Code / Codex / Antigravity / Hermes / Cursor 等。
  * `stdio`：本地接入
  * `streamable HTTP`：远程接入 (VPS)
* **REST**：面向 Aider / 脚本 / 任何标准 HTTP 客户端。
* **Skills**：通过 MCP `prompts/*` + REST `/skills/*` 进行通用全 Agent 技能分发。
