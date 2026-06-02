# Xiaoqinli (xql) 极简架构 v2.0

> **定位**：AST-First 安全转译器（Transpiler） 单 Go 二进制  无运行时  极简 / 安全 / 高性能
> **版本**：v2.0 (减法版)
> **变更说明**：相对 v1.4 大幅瘦身。砍掉双语言栈、VM、字节码、自愈引擎，认知原语移出核心。

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
| **认知原语 `predict/embed/<=**` | 运行期调 LLM = 不确定性 + 网络依赖 + token 成本，与安全/高性能冲突 | 移到 v3 可选插件，不进核心 |
| **自愈引擎 healer** | 运行期改代码 = 不可控，违背产出确定原则 | 删除 |
| **LSP** | 非核心，优先级低 (P3) | 推迟到工具链成熟后 |
| **IR DAG 中中间层** | 多余的转译层 | 删除，AST 直接到目标代码 |
| **10 个 packages** | 过度模块化 | 收敛到 6 个 Go 包 |

> **保留并强化**：AST-First 存储、内容寻址、类型+效果检查、能力安全、MCP/REST/Skills 三通道接入。

---

## 1. 核心定位

Xiaoqinli 是一个极简、安全、高性能的 **AST-First 转译器** 。

* **输入**：AI 直接编写结构化的 `.xql.json`（物理上天然无语法错误）。
* **处理**：编译器做类型与能力的静态检查。
* **输出**：直接输出目标语言源码（Go / Rust / TypeScript / Python）。
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
4. **输出**：`main.go` / `main.rs` / `main.ts` / `main.py`

> **对比 v1.4**：删除了 IR DAG、字节码、VM 三层。中间不落任何中间表示，AST 检查通过后直接 codegen，走最快路径。

---

## 3. 单二进制结构 (Minimal Codebase)

整个项目收敛为单一 Go module，仅包含 **6 个 Go 包** ：

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
    golang.go               # Go（P0 首先实现）
    rust.go                 # Rust（P2）
    typescript.go           # TypeScript（P2）
    python.go               # Python（P2）
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
没有运行期 LLM、没有运行期代码改写、没有动态加载。
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

## 6. .xql 语言规范：双视图设计

* **核心决策**：为保持极简，**编译器只接受 `.xql.json` (AST) 作为输入**。
* **人类视图**：`.xql`（人类文本视图）是 AST 单向渲染的**只读**产物。
* **不内置 Parser**：编译器**不内置** `.xql`  AST 的逆向解析器。
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
```
