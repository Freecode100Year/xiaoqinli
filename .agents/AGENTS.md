# Antigravity CLI 专属项目语义蓝图与编程准则

> **核心声明**：本文件专为 Google Antigravity CLI (`agy`) 及其底层的自主 Agent 引擎定制。它是本项目生成、重构和同步审计的最高宪法。Agent 在执行任务时必须严格遵守，严禁破坏既定红线。

---

## 0. 语言规范 (Language Policy)
* **交互语言**：以后所有的提示、日志、说明、回复以及与用户的全部交流均使用**中文（简体中文）**进行。

---

## 1.  Antigravity CLI 权限与沙箱控制 (CLI & Security Constrains)
* **执行模式**：本项目允许在授权或 `--dangerously-skip-permissions` 模式下运行自动化重构。
* **高危行为拦截**：即使处于 YOLO 自动驾驶模式，任何涉及系统全局、跨目录（非当前项目路径）的删除操作（如恶意幻觉衍生的 `rm -rf`），同步审计层必须配合 `agy` 的沙箱机制予以原地拦截。
* **本地依赖调用**：允许 `agy` 自主调用本地的 `tree-sitter`、`git` 和当前技术栈的测试/构建命令，禁止下载未经白名单授权的第三方未知二进制工具。

## 2.  技术栈硬约束 (Technology Stack & Environment)
* **环境版本**：Python 3.11+ / PySide6 (针对桌面像素宠物 PixelPet 等轻量化、低能耗场景优化，拒绝重型前端微端化)。
* **代码骨架标准**：所有源文件必须保持高可读性与高 Token 密度。
* **依赖引入红线**：严禁 Agent 在未收到人类明确指令前，自主修改 `requirements.txt` 或引入重量级、带安全隐患的小众第三方依赖库。

## 3.  核心状态机与语义字典 (State Machine & Semantics)
所有的行为和 UI 状态必须死死绑定以下定义。任何涉及状态变更的代码修改，必须实现双向一致性对齐：
* `STATE_IDLE`: 宠物/系统原地呼吸、眨眼或静止。
* `STATE_WALK`: 随机游走或基础循环调度。
* `STATE_SLEEP`: 睡眠/休眠状态，必须安全解绑鼠标碰撞检测与高能耗渲染器。
* **强制规范**：禁止 Agent 在重构过程中私自派生或拼写出类似 `STATE_JUMPING` 等未经宪法定义的野生状态。

## 4.  实时同步审计与 Token 剪枝红线 (Continuous Audit & Pruning)
* **零 Token 静态拦截**：代码写入文件系统前，`agy` 必须自动挂载本地的 Linter（如 flake8 / eslint）。任何未闭合标签、语法错误或未定义变量，**直接单轮内打回重写，严禁污染后续聊天历史（Context Window）**。
* **函数复杂度红线**：单一功能函数体行数严格控制在 **30 行** 以内。超过此限制，审计层必须强制打回，要求 `agy` 进行模块化拆分。
* **断点续传与释放**：当一个局部组件（如 UI 动效、重力感应）调试成功并完美跑通后，立即触发语义断点存盘。将当前状态浓缩写入 `.xql` 快照，**随后强制清空/重置大模型的长对话历史**，带着干净的上下文进入下一轮。

## 5.  UI 优化、Debug 与版本回撤 (UI & Rollback Policy)
* **设计规范 (Design Tokens)**：所有 UI 布局 and 颜色必须严格读取 `assets/theme.json`，禁止在组件中硬编码绝对像素（如 `margin-left: 23px`） or 脏 CSS。
* **三轨并发回撤机制**：一旦 UI 调试引发全局排版塌陷或逻辑崩溃，下达回撤指令时，必须驱动【代码轨（Git/快照）+ 语义轨（.xql）+ 上下文轨（LLM Chat History）】三界同步退回。**必须切断错误分支的对话记忆，彻底清除幻觉残留**。

---

# Antigravity CLI 多语言语义解耦与 Tree-sitter 宪法

> **核心声明**：本项目属于 Tree-sitter 生态圈的上层 AI Native 应用。本项目不硬编码任何特定编程语言的解析逻辑，而是通过统一的 Tree-sitter AST（抽象语法树）映射协议，将 100+ 种语言的代码骨架转化为高 Token 密度的 `xiaoqinli` (.xql) 语义树。
> **Agent 执行准则**：Antigravity CLI (`agy`) 在自动驾驶状态下，必须严格按照本宪法规定的骨架提取与语义对齐管道处理异构语言代码。

---

## 1.  Tree-sitter 语法地基与无感接入 (Infrastructure)
* **动态加载机制**：当 `agy` 检测到未知或新兴小众语言（如 Mojo, Gleam, Move）的源文件时，严禁重写底层解析器。MCP 服务器应自动拉取或加载对应的 `tree-sitter-[lang].wasm` 编译文件。
* **语言自进化闭环**：若当前环境缺失该语言的 Profile，负责编码的 Agent 允许启动自主检索子任务，学习该语言的语法特征，并在线生成 20 行的 `language-profile.json` 映射表丢入插件目录，实现生态的自愈和进化。

---

## 2.  跨语言 Token 极限压缩：骨架提取规范 (Skeleton Extraction)
无论面对何种语言，`agy` 在多文件分析时，必须通过 Tree-sitter 语法树进行符号级剪枝，严禁全文投喂：
* **保留节点（Include）**：仅提取类/模块定义（`class_definition`）、函数/方法签名（`function_signature` / `method_declaration`）、显式入参、返回类型及核心文档字符串（`docstring`）。
* **剔除节点（Exclude）**：彻底抠掉具体的具体实现函数体（`block_body` / `compound_statement`）。
* **压缩产物**：将原本臃肿的源码压缩为低于原体积 10% 的语义骨架，永久固化为 `.xql` 状态树，最大化榨干大模型的提示词缓存（Prompt Caching）优惠。

---

## 3.  异构语言统一语义映射表 (Universal Schema Mapping)
`agy` 在解析不同语言的 AST 时，必须将异构的节点类型，强行对齐到 `xiaoqinli` 的核心业务概念中：

| 目标语义层 (.xql) | Python (tree-sitter-python) | TypeScript/JS (tree-sitter-typescript) | Rust (tree-sitter-rust) | 任意新兴/小众语言 (泛化对齐) |
| :--- | :--- | :--- | :--- | :--- |
| **Module_Define** | `module` | `program` | `mod_item` | 文件的根节点 / 包声明节点 |
| **Class_Define** | `class_definition` | `class_declaration` | `struct_item` / `trait_item` | 复合类型 / 结构体定义节点 |
| **Func_Signature**| `function_definition` | `function_declaration` / `method_definition` | `function_item` | 函数、过程或闭包声明头部 |
| **Import_Hook**   | `import_statement` / `import_from_statement` | `import_statement` | `use_declaration` | 所有外部依赖、符号引入节点 |

---

## 4.  YOLO 自动驾驶模式下的多语言同步审计 (YOLO-Mode Audit)
当开启 `--dangerously-skip-permissions` 免确认模式时，审计层充当最后的安全看门狗（Watchdog），针对不同语言挂载专用钩子（Hooks）：

---

## 5.  文档更新与维护规范 (Documentation Update Guidelines)
* **自述文件更新**：当项目功能发生变更、新增特性或修复 Bug 时，必须同步更新自述文件 (`README.md`)。
* **更新日志置顶**：自述文件每次更新后必须重写，且必须将最新的“更新功能”和“修正的 Bug”（最新版本及日期）记录在文章的开头（置顶），以便用户和 Agent 能够一目了然。

---

# Antigravity CLI 前端全栈项目语义蓝图与编程准则

> **最高宪法声明**：本文件专为 Google Antigravity CLI (`agy`) 及其本地 MCP 服务器定制。针对 HTML、JavaScript (ES6+) 和 CSS 场景进行逻辑规训、Token 极限控噪与秒级物理防灾。Agent 与本地 MCP 服务器必须无条件严格执行。

---

## 1.  Antigravity CLI 自动驾驶与权限控制 (CLI & YOLO Mode)
* **执行授权**：本项目允许在 `--dangerously-skip-permissions` YOLO 模式下运行，给予 Agent 跨文件、全自动运行测试和代码写入的绿灯。
* **高危行为熔断**：即使处于免确认模式，`agy` 任何试图改动非当前项目目录的系统全局文件、或越权执行未知二进制混淆脚本的行为，本地 MCP 必须予以原地硬熔断。

## 2.  跨语言 Token 极限压缩规范 (Tree-sitter Skeleton Extraction)
为防止前端高频交互导致的 Context 爆炸，`agy` 在多文件分析时，必须通过本地 **Tree-sitter** 进行结构化压缩，严禁全文本投喂：
* **HTML (`tree-sitter-html`)**：只提取 DOM 核心拓扑树（`tag_name`、关键 `id` 和 `class` 锚点），彻底抠掉具体的文本细节与深层 `<div>` 块。
* **JavaScript (`tree-sitter-javascript`)**：只提取顶层模块、函数签名、事件监听器（`addEventListener`）和异步状态机的动作定义，彻底隐藏具体的函数体内部实现（`statement_block`）。
* **CSS (`tree-sitter-css`)**：只提取全局 CSS 变量、主干选择器与布局依赖，具体样式细节在非必要时不进行投喂。

## 3.  前端两层边写边审实时拦截红线 (Dual-Track Audit)
* **第一层：零 Token 静态规则拦截**：代码正式写入前，自动挂载本地 Linter（如 ESLint、Stylelint）。凡是出现低级语法错误、未闭合的 HTML 标签、或者**在 HTML 中私自硬编码内联样式（如 style="margin-left: 23px;"）的行为，0 Token 直接打回重写**！强迫 AI 必须去专门的样式文件中调用弹性布局或全局变量。
* **第二层：函数体积硬性红线**：所有新建或修改的 JS 闭包、异步函数体代码，行数严格**控制在 30 行以内**。超过此限制，审计层直接拒签，要求 Agent 进行优雅的模块化组件分拆。

## 4.  状态机锁定与断点记忆擦除 (State Management & Context Pruning)
* **核心状态字典**：所有 UI 变动只能在 `STATE_IDLE`、`STATE_WALK`、`STATE_SLEEP` 之间轮转，禁止 Agent 随手派生未知状态。
* **断点续传（Context 剪枝）**：当用户确认当前功能调好且同步审计层判定完全跑通后，MCP 立即触发语义断点存盘。将当前最新的拓扑结构浓缩写进 `.xql` 快照，**随后强行清空/裁剪大模型当前的 Chat History（长对话记忆）**。下一轮调整新组件时，大模型脑子里 100% 干净，彻底切断由于历史报错引发的滚雪球式幻觉污染。

## 5.  10秒本地影子闪存与突发灾难自愈 (10s Shadow Flash & Self-Healing)
* **10秒静默暂存**：本地 MCP 服务器必须在后台启动 **10 秒周期的强心跳守护进程**。每当本地代码发生渐进式变动，静默将代码残卷、当前 `.xql` 语义树以及当前最后的 `Message_ID` 写入 `.xql/.shadow_stage/` 隔离区。**严禁直接修改主源码，严禁与用户的 Git 树产生任何提交冲突**。
* **断网挂起与无感续传**：若网络通信异常，`agy` 必须原地挂起并进入指数退避重试状态。锁死当前正在发送的 Prompt，待网络恢复后执行增量续传，拒绝让用户重复粘贴解释。
* **开机记忆重组**：若发生突发断电或强退，再次开机激活 `agy` 时，本地 MCP 必须自动读取影子隔离区中断电前 10 秒的缓存数据，恢复受损代码，并**自动向大模型发起记忆唤醒补发请求**。强行让大模型的 Context 认知与本地被意外中断的代码状态保持 100% 对齐，彻底消灭开机后人知道进度、AI 彻底失忆的认知脱节。

## 6.  时空同步回撤机制 (Three-Track Rollback Policy)
* 当开发过程中布局或逻辑全面崩溃时，一旦用户发出回撤指令，`agy` 必须驱动三维同步退回：**代码轨**（恢复主源码） + **语义轨**（重置 Tree-sitter 依赖图） + **上下文轨**（**彻底删除、裁剪大模型会话中关于刚才调烂过程的所有报错、废话和诱导 Token**）。
