# 破坏性变更

对外契约发生变化时记在这里。「对外」指调用方可能依赖的东西：错误码、目标是否接受某个程序、CLI 退出码。

---

## `XQL_E403` 不再表示 MQL 功能限制

**改成什么：** MQL4 / MQL5 因不支持某个构造而拒绝编译时，错误码从 `XQL_E403` 变为 `XQL_E402`。

**受影响的消息：**

| 之前 | 现在 |
|---|---|
| `XQL_E403: MQL does not support Map type` | `XQL_E402: MQL does not support Map type` |
| `XQL_E403: MQL does not support Option type` | `XQL_E402: MQL does not support Option type` |
| `XQL_E403: MQL does not support Result type` | `XQL_E402: MQL does not support Result type` |
| `XQL_E403: MQL does not support for-each loops` | `XQL_E402: MQL does not support for-each loops` |

**为什么：** 同一件事——「这个后端表达不了这个构造」——此前按后端不同报三种码：多数后端报 `XQL_E402`，MQL 报 `XQL_E403`，bat 报 `XQL_E401`。想靠 `ErrorCode` 判断「该换个目标了」的调用方做不到这件事。

更糟的是 `XQL_E403` 一码两义：它同时是 `compiler/compiler.go` 里 bundle 写盘时的路径逃逸错误（一个安全检查）。两种含义毫不相干。

现在的不变式是：

- **`XQL_E402`** — 程序没问题，是目标表达不了。换个目标，或改掉那个构造。
- **`XQL_E401`** — 编译器自身出错。这是 bug，请上报。
- **`XQL_E403`** — bundle 路径逃逸，仅此一义。

`TestExampleTargetMatrix` 对全部 460 个「示例 × 目标」组合强制这条不变式，所以它不会再漂回去。

**需要你做什么：** 如果有代码 `switch` 或匹配 `XQL_E403` 来处理 MQL 的能力限制，改成 `XQL_E402`。同时匹配两者是安全的过渡写法——`XQL_E403` 不会再由 MQL 产生。

**未变：** 错误消息文本、退出码（编译错误仍为 `2`）、哪些程序被拒绝。只有码变了。

---

## `Result<T, E>` 的目标支持范围大幅收窄

**改成什么：** 27 个目标现在以 `XQL_E402` 拒绝使用 `Result<T, E>` 的程序，此前它们「编译成功」。

**为什么：** 它们从未真正支持过。只有十二个后端实现了 Result 运行时；其余的把 AST 里的名字原样透传，生成
`Result.ok(users)` 和 `res.unwrap()`——在 Haskell 里是不存在的模块的限定名，在 PowerShell 和 Tcl 里是不存在的命令，
在其余语言里是未声明的符号。codegen 对这些一律报告成功。

也就是说：**这不是能力的收回，而是一个假象的消除。** 之前「成功」编译出来的产物本来就无法运行。现在你会在编译期拿到
一条明确的错误，而不是一个到运行时才崩的文件。

**仍然支持 `Result<T, E>` 的目标（16 个）：**

`go` `rust` `ts` `py` `java` `csharp` `kotlin` `swift` `dart` `lua` `ruby` `php` `zig` `julia` `android` `ios`

其中 14 个属于 executed 层——生成的程序在 CI 里真的编译、运行并核对了 stdout。`android` 与 `ios` 是工程脚手架，
其 Kotlin / Swift 源码由对应的 executed 层目标覆盖。

**需要你做什么：** 如果你在用上述之外的目标编译带 `Result` 的程序，先前拿到的产物是坏的。改用支持的目标，或把
`Result<T, E>` 换成其他错误表达方式。

`TestNoBackendFakesResultSupport` 会对每个后端提同一个问题：输出里引用了 `Result`，那么输出里定义 `Result` 了吗？
只有 `rust`（标准库自带）和 `zig`（生成独立的 `result.zig`）被豁免，且两者都在 executed 层——它们的程序在 CI 里真的
编译、运行并核对了 stdout。

---

## `ForStmt` 的 range 形式在九个后端曾多跑一轮

**改成什么：** `go` `rust` `ts` `js` `py` `java` `csharp` `swift`（以及 `ios`、`android` 两个脚手架）生成的
range 循环由闭区间改为半开区间。`kotlin` 之前生成的 `for (i in 0L <= 5L)` 根本不是区间，改为 `until`。

| 目标 | 之前 | 现在 |
|---|---|---|
| `go` | `for i := 0; i <= 5; i++` | `for i := 0; i < 5; i++` |
| `rust` | `for i in 0..=5` | `for i in 0..5` |
| `ts` `js` | `for (let i = 0; i <= 5; i++)` | `for (let i = 0; i < 5; i++)` |
| `py` | `range(0, (5) + 1)` | `range(0, 5)` |
| `java` `csharp` | `for (long i = 0L; i <= 5L; i++)` | `... i < 5L ...` |
| `swift` `ios` | `for i in 0...5` | `for i in 0..<5` |
| `kotlin` `android` | `for (i in 0L <= 5L)` / `for (i in 0L..5L)` | `for (i in 0L until 5L)` |

**为什么：** `ast/nodes.go` 一直把 range 定义为**不含** `end`，另外 23 个后端也是这么实现的。这九个不是。
仓库自带的 `examples/loop.xql.json` 对一个五元素数组求和，正确答案是 15——它在 Go 里 panic，在 Python 里
`IndexError`，在 JavaScript 里打印 `NaN`，而在 C、Lua、Perl、Tcl 里打印 15。同一份 AST，两种语义。

其中八个属于 executed 层。它们跑的 dogfood 工程里没有 range 循环，断言又只检查 stdout 是否**包含**两个名字，
所以这件事一直没被看见。

**需要你做什么：** 如果你的 AST 是照着上述某个后端的实际行为（而不是照着 range 的定义）写的——比如为了迭代
`0..n` 而把 `end` 写成 `n - 1`——把 `end` 改回 `n`。按定义写的 AST 不受影响，只是终于在这九个目标上也对了。

**未变：** `each` 形式、错误码、退出码。

`TestCrossTargetConformance` 现在用九种语言真的运行这批示例并逐行核对 stdout，这条语义不会再各说各话。
