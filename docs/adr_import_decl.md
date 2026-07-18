# ADR 002: 多文件项目跨文件 Import 及 Capability 链追踪设计

## 状态
Accepted

## 背景
xiaoqinli 之前仅支持单文件（AST Root）的转译与静态分析。为了支持具有实际工程规模的多文件项目，需要建立跨文件的类型、符号以及 Capability 检查通道，同时在代码生成层（Codegen）将多文件转换为各后端的目标多文件模块结构。

## 决策

### 1. AST 节点定义 (`ImportDecl`)
- **JSON 表示法**：
  ```json
  {
      "kind": "ImportDecl",
      "path": "./utils.xql.json",
      "as": "utils"
  }
  ```
- **属性**：
  - `path`: 相对路径，用于在 Workspace/VFS/磁盘上定位被导入的文件。
  - `as`: 命名空间别名（Alias），在当前文件中使用该别名引用被导入文件中的符号。

### 2. 符号表跨文件解析 (Workspace Scope)
- **依赖收集与循环 Import 检测**：
  - 在对单个 `*ast.Program` 执行类型检查前，遍历所有 `ImportDecl`，并使用 DFS（深度优先搜索）递归加载并解析依赖树。
  - 维护一个 `visiting` 状态栈。如果发现当前加载的文件已经在 `visiting` 栈中，直接抛出 `XQL_E402` 循环依赖错误，拒绝编译，防止死循环。
- **符号跨文件引用**：
  - 将 `TypeChecker` 扩展为包含 `imports map[string]*TypeChecker` 属性。
  - 当在 `main.xql` 中解析如 `utils.foo` 这样的跨模块调用时，类型系统会穿透到 `imports["utils"]` 对应的符号表进行存在性与签名验证。
  - 跨模块的 struct / class 类型表达式 `utils.Point` 将通过别名路由解析为被导入模块中的实际结构。

### 3. Capability 跨文件调用图分析
- 扩展 `check/capability.go` 里的能力验证分析。
- 多文件依赖下，函数跨模块调用必须在 Capability 上存在包含关系。例如：若 `main.xql` 里的 `main()` (无任何 grant) 调用了 `utils.xql` 里的 `netCall()` (需要 `@grant network`)，即使是在多文件边界外，仍然构成能力泄漏。
- 在 `CheckCapabilities` 进行分析时，通过跨文件的 `funcTable` 去追溯被调用函数的 grant 属性，保障 Capability 的全图安全。

### 4. Codegen 落地策略
- **Go 后端**：
  - 输出为多个 Go 文件，放在同一个 package（或不同 package）下。由于 Go 不支持在同一个 package 里同名符号冲突，如果 import 的是 `./utils.xql`，我们将其编译为独立文件。在同一目录下，若是同一个包 `package main`，它们是共享符号的。为保持极简，我们直接让各生成的 `.go` 文件的 header 统一属于同一 `package main`（或者根据 import 的 as 生成相对引用的 module 导入）。对于 xiaoqinli 本身，最简洁且不易出错的方法是将导入的别名在生成时平铺为同包内的本地调用，或者真正输出为 Go 的 `import` 引用。为了让多文件完全可编译，我们可以生成多文件，并根据 as 字段决定如何输出。
- **TypeScript / Python / Rust 后端**：
  - **TS**：生成 `import * as utils from "./utils";`。
  - **Python**：生成 `import utils`。
  - **Rust**：在 `main.rs` 里生成 `mod utils;`，并使用 `utils::foo`。

## 后果
- 实现了 Workspace 级别的安全分析。
- 保证了在所有主流目标语言后端上的**语义一致性**与**多文件模块结构**的合理性。
