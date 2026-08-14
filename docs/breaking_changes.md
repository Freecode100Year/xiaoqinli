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

---

## 两个 Int 相除现在是整数除法

**改成什么：** `Int / Int` 在十四个后端由「原样透传 `/`」改为发射该语言的整数除法。`7 / 2` 现在到处都是 `3`。

| 目标 | 之前 | 现在 |
|---|---|---|
| `py` | `a / b` → 3.5 | `a // b` |
| `js` `ts` | `a / b` → 3.5 | `Math.trunc(a / b)` |
| `perl` `awk` | `a / b` → 3.5 | `int(a / b)` |
| `lua` | `a / b` → 3.5 | `a // b` |
| `php` | `$a / $b` → 3.5 | `intdiv($a, $b)` |
| `julia` | `a / b` → Float64 | `div(a, b)` |
| `dart` | `a / b` → double | `a ~/ b` |
| `elixir` | `a / b` → float | `div(a, b)` |
| `clojure` | `(/ a b)` → 有理数 `7/2` | `(quot a b)` |
| `groovy` | `a / b` → BigDecimal | `a.intdiv(b)` |
| `powershell` | `$a / $b` → Double | `[long][math]::Truncate(...)` |
| `pascal` | `a / b` → Real，赋给 Integer 都不合法 | `a div b` |
| `haskell` | `a / b` → **不通过类型检查**（`/` 属于 Fractional） | `` a `quot` b `` |
| `zig` | `a / b` → **编译错误**（有符号整数不许用 `/`） | `@divTrunc(a, b)` |
| `nim` | `a / b` → **编译错误**（`type mismatch: got <int64, int64>`） | `a div b` |

**为什么：** `/` 的含义此前完全取决于目标语言碰巧怎么定义它。同一份 AST，`7 / 2` 在 Go、C、Rust、Java、Ruby、Tcl 里是 3，在上表那些语言里是 3.5，在 Haskell 和 Zig 里根本编译不过。这是一个转译器最不该含糊的地方。

`ast/nodes.go` 现在把这条写死：**两个 Int 相除是整数除法，向零截断**。

**需要你做什么：** 如果你原本靠 `Int / Int` 拿浮点结果（只在上表那些目标上碰巧成立），把操作数改成 `Float`。

**已知残留：** 向零截断与向下取整只在「恰好一个操作数为负」时不同。`py`、`lua`（`//`）和 `ruby`（`/` 原生）是向下取整，其余是截断。conformance 语料只用非负操作数，所以这一段应当视为**未规定**，不要当成已验证。

**未变：** `Float / Float`、`%`、其余算符，以及错误码与退出码。

---

## 字符串比较此前在 perl 和 c 上是坏的

**改成什么：** 涉及 String 的比较运算符，`perl` 改用 `eq`/`ne`/`lt`/`gt`/`le`/`ge`，`c` 改走 `strcmp(a, b) <op> 0`。
`cpp` 的两个字符串**字面量**相加改为 `std::string("ab") + "c"`。

**为什么：**

- **perl** 的 `==` 是数值比较。两个非数字字符串都会 numify 成 0，于是 `"abc" == "xyz"` 为**真**——
  生成的程序里每一次字符串相等判断都回答「相等」。
- **c** 里 String 是 `const char *`，`a == b` 比的是地址。运行期拼出来的 `"ab" + "c"` 与字面量 `"abc"`
  比出来是**不等**；反过来，两个相同的字面量又可能因为编译器做了池化而比出「相等」。两种方向都错。
- **cpp** 的 `"ab" + "c"` 是两个 `const char[]` 做指针算术，**根本编译不过**。之前没暴露是因为语料里没有
  「两个字符串字面量相加」这种写法。

**需要你做什么：** 不需要。之前的行为在这三个目标上都是错的，没有可依赖的语义。

`examples/string_compare.xql.json` 现在把这三件事都钉在 conformance 语料里。

---

## `break` / `continue` 的拒绝码由 `XQL_E401` 改为 `XQL_E402`

**改成什么：** `ada` `bat` `clojure` `elixir` `fsharp` `haskell` `lua` `ocaml` 在遇到自己表达不了的
`break` / `continue` 时，错误码由 `XQL_E401` 改为 `XQL_E402`，共 14 处。

**为什么：** 这违反了本文件开头那条不变式——`XQL_E402` 是「目标表达不了」，`XQL_E401` 是「编译器有 bug，请上报」。
把能力边界报成编译器 bug，会让按 `ErrorCode` 分流的调用方把「该换个目标」误判成「该提 issue」。

之前没被 `TestExampleTargetMatrix` 抓到，只是因为语料里没有任何一个示例用到 `break`。
`examples/control_flow.xql.json` 补上了这个洞。

**需要你做什么：** 如果你匹配 `XQL_E401` 来处理这些拒绝，改成 `XQL_E402`。

**已知残留：** codegen 里还有约 100 处 `XQL_E401: ... does not support ...`，其中一部分同样应该是
`XQL_E402`。它们目前没有任何示例覆盖，因此**未经检验**，这次没有一并改——照着分类扫一遍而不跑一遍，
正是这个项目一直在拆掉的那种做法。

---

## 负数除法与取余不再各说各话

**改成什么：** `Int / Int` 与 `Int % Int` 在 `py` `ruby` `lua` `tcl` `perl` 上改为发射一个 helper，
而不是各自语言的原生算符。

| 表达式 | 之前（这五个） | 现在（全部目标） |
|---|---|---|
| `-7 / 2` | -4 | **-3** |
| `-7 % 2` | 1 | **-1** |
| `7 / -2` | -4 | **-3** |
| `7 % -2` | 1 | **1** |

**为什么：** 上一条把 `Int / Int` 定为「向零截断」时，留了一个已知残留：向下取整的那几个语言在
「恰好一个操作数为负」时结果不同。这条把残留补上了。

符号规则是成对出现的——**凡是除法向下取整的语言，`%` 也取除数的符号**。所以 Python、Ruby、Lua、Tcl、Perl
在这两个算符上一起偏离，而 C、Go、Java、Rust、JavaScript、awk、bash 一起是截断 + 被除数符号。截断是多数，
于是成为规则；这五个后端现在发射 `_xql_idiv` / `_xql_irem`。

代价是这五种语言里的整数除法输出不再是一个裸算符，而是一次函数调用。这是明知的取舍：正确性优先于产物好看。

**需要你做什么：** 如果你的 AST 依赖这五个目标上的向下取整语义（只可能是照着实际行为写的，不是照着规范），
结果会变。规范一侧没有变过——`ast/nodes.go` 一直写的是向零截断。

`examples/negative_arithmetic.xql.json` 把四种符号组合全部钉进 conformance 语料，十一个 runner 逐行比对。

---

## `shortcut` 现在拒绝 `while` / `break` / `continue`

**改成什么：** `shortcut` 后端对以下三个构造由「编译成功」改为以 `XQL_E402` 拒绝：

| 构造 | 之前发射的东西 | 现在 |
|---|---|---|
| `WhileStmt` | `Repeat 1000`，条件被整个丢弃 | `XQL_E402` |
| `BreakStmt` | 一条注释 | `XQL_E402` |
| `ContinueStmt` | 一条注释 | `XQL_E402` |

另外，range 形式的 `for` 若上下界不是字面量，此前会静默按 10 轮生成，现在同样拒绝。

**受影响的示例：** `control_flow.xql.json` 与 `while_accumulate.xql.json` 从 `shortcut` 编译成功变为被拒绝，
`TestExampleTargetMatrix` 的期望表已同步。

**为什么：** Shortcuts 的循环只有 Repeat（给次数）和 Repeat With Each（给列表），没有读条件的形式，
也没有任何提前离开 Repeat 的动作。`Repeat 1000` 不是 `while cond` 的翻译：`while_accumulate.xql.json`
本该跑 3 轮，会跑 1000 轮；`control_flow.xql.json` 唯一的出口是 `break`，本该跑 7 轮，同样会跑 1000 轮。
两者都会「编译成功」。

注释更直接——注释不是跳转，循环照跑不误。这正是 `XQL_E402` 存在的理由，README 那句「后端表达不了就拒绝，
不静默降级」在这里此前是不成立的。

`shortcut` 是 smoke 等级：产物从来没有被 Shortcuts 导入过，也没有命令行工具能导入它。
一个没有任何东西能验证其行为的后端，唯一的防线就是不去说自己说不出的话。

**需要你做什么：** 如果有 AST 依赖 `shortcut` 接受这三个构造，那份产物本来就是错的。
用 range 形式的 `for` 改写循环，或换一个目标。

**漏掉的第四个：** 见下一节——`return` 也离不开 Repeat，而它不长得像跳转，所以上面这一轮没有拒绝它。

---

## `bash` 与 `bat` 的提前 `return` 此前不会返回

**改成什么：** 函数体中间的 `return`（典型是循环里命中条件就返回）在 `bash` 和 `bat` 上现在真的结束函数。

| 目标 | 之前 | 现在 |
|---|---|---|
| `bash` | `echo <值>`，然后继续往下执行 | `echo <值>` 后跟 `return`（`main` 在顶层，用 `exit`） |
| `bat` | 只 `set "_return=..."`，块内不 `exit /b` | 一律 `endlocal` 传值后 `exit /b 0` |

**影响：** `examples/early_return.xql.json` 的 `firstOver(20)` 应当返回 5。此前 `bash` 把 5、6、7、8、9
以及循环后那句 `return 0` 的 0 依次 echo 出来，调用方 `$(firstOver 20)` 捕获到的是 `"5 6 7 8 9 0"`；
`bat` 则被后面的 `set /a "_return=0"` 覆盖成 0。

**为什么之前没发现：** 语料里每一个 `return` 都是所在函数的最后一句。那种形状只要求值送达，
不要求这条语句停下任何东西。

**同批修掉的两处 `bat` 缺陷**（都由同一个示例暴露）：

- `if` 的操作数是算式时会写成 `if (%%i * %%i) GTR !limit!`。cmd 的 `if` 只比较两个词、不做任何求值，
  这一行是语法错误（`* 不应出现在此时`），脚本当场死掉。现在算式先由 `set /a` 落到临时变量。
- 值上下文与条件上下文读标识符时不看 `forVars`，`for /L` 的循环变量被写成 `!i!`（一个从未设置过的
  环境变量，展开为空）而不是 `%%i`。`emitArithExpr` 和 `emitIndexExpr` 早就知道这件事，另两条路径不知道。

**需要你做什么：** 大概率什么都不用做——之前的行为没有一种用法是对的。

---

## `haskell` / `ocaml` / `elixir` 现在拒绝循环体内的 `return`

**改成什么：** 若一个 `ForStmt` 或 `WhileStmt` 的循环体（含其中嵌套的 `if` / `match` / 内层循环）里出现
`ReturnStmt`，这三个后端由「编译成功」改为以 `XQL_E402` 拒绝。

**为什么：** 三者都把循环降级成一个跑到底的形式，没有提前离开的办法：

| 目标 | 循环降级成 | 此前的产物 |
|---|---|---|
| `haskell` | `mapM_` | lambda 返回 `Int`，而 `mapM_` 要的是 action——**编译不过** |
| `ocaml` | `for ... done`（循环体必须是 unit） | 循环体求值为 `int`——**编译不过** |
| `elixir` | `Enum.reduce` | **编译得过、跑得通、答案是错的**：推导式的结果被丢弃 |

`examples/early_return.xql.json` 里 `firstOver(20)` 应当返回 5。elixir 两次调用都返回循环之后那句
`return 0` 的 0，安静地。这三个后端拒绝 `break` 和 `continue` 已经很久了，理由完全相同——
循环体内的 `return` 只是第三种提前退出，此前漏掉了。

**需要你做什么：** 把「循环里命中就返回」改写成「用一个变量记住结果、循环结束后再返回」。
这个形状这三个后端都能表达。

**已知残留：** 判定用 `codegen/util.go` 的 `loopBodyReturns`，它不会走进 `Lambda`——
lambda 里的 `return` 属于 lambda 自己。

**补记（后一次审计）：** 这个预扫描当时只认作为语句直接出现的 `MatchExpr`，不认被 `ExprStmt`
包起来的同一棵树。少认这一层的后果和上表一模一样：`ocaml` 生成 `for ... do (match i with 3 -> limit | _ -> ...) done`，
循环体求值为 int 而非 unit，编译不过；`elixir` 编译得过，`case` 的值被推导式丢掉，返回落空的 0。
现在 `ExprStmt`、`VarDecl` 与 `AssignStmt` 的值位置一并下降，`Lambda` 仍然不进——
值位置是把节点交回同一个 switch，而那个 switch 不认 `Lambda`。

---

## `shortcut` 现在也拒绝 Repeat 里的 `return`，并且循环变量终于有人赋值了

**改成什么：**

| 构造 | 之前发射的东西 | 现在 |
|---|---|---|
| 循环体内的 `ReturnStmt` | 取值的动作，Repeat 照跑到底 | `XQL_E402` |
| range 循环的循环变量 `i` | 什么都没有 | `Repeat Index` 减 1 加 start，存进 `i` |
| each 循环的循环变量 `n` | 什么都没有 | `Repeat Item` 存进 `n` |
| `end < start` 的 range | `Repeat -3 次` | `Repeat 0 次` |
| 不认识的 `ForStmt.Form` | 静默产出空 | `XQL_E401` |

**受影响的示例：** `early_return.xql.json` 与新增的 `each_return.xql.json` 从 `shortcut`
编译成功变为被拒绝，`TestExampleTargetMatrix` 的期望表已同步。

**为什么：** 上一轮拒绝了 `while` / `break` / `continue`，理由是 Repeat 没有任何提前离开的动作。
`return` 是同一件事，只是它带着一个值，看起来不像跳转，于是漏了：`early_return.xql.json` 的
`firstOver(20)` 应当返回 5，这份工作流会把十轮全跑完，再取循环后那句 `return 0` 的 0。两次调用，两个 0。

循环变量是同一场审计翻出来的第二件事，而且更早就在那里了。Repeat 只发布两个变量——`Repeat Index`（从 1 数起）
和 `Repeat Item`——这个后端一个都没读过。`loop.xql.json` 的 `i`、`nested_loop.xql.json` 的 `i` 和 `j`，
在生成的工作流里都是没有任何动作设置过的名字。range 的 start 也一起丢在这里：它被减进了次数里，然后就没了。
这正是 smoke 等级看不见的东西——JSON 解析得过，每个动作的标识符都带命名空间，然后拿一个空变量去算。

**需要你做什么：** 如果有 AST 靠 `shortcut` 接受循环里的 `return`，那份产物本来就是错的。
把它改写成「用变量记住结果、循环后再返回」。已经生成过的工作流值得重新生成一次——
循环变量的绑定是新加的，旧产物里没有。

**未变：** 仍然是 smoke 等级。`TestShortcutLoopVariableIsBound` 能断言的是绑定动作存在、读的是
`Repeat Index` / `Repeat Item`、写的是程序用的那个名字。Shortcuts app 会不会接受这份工作流，
仍然没有任何东西能在 CI 里回答。

---

## `rust` 的 for-each 由绑定引用改为绑定值

**改成什么：** `for n in &nums` 改为 `for n in nums.iter().cloned()`。

**为什么：** `&Vec<T>` 迭代出来的是 `&T`。语料里唯一的 for-each 是 `for_each.xql.json`，
它对元素做的唯一一件事是相加，而 `i64 + &i64` 恰好是引用不用打招呼就能做的少数几件事之一。
其他事都不行：新增的 `each_return.xql.json` 把元素和参数比一下再返回它，rustc 两处都拒绝，
`expected i64, found &i64`。

`.iter().cloned()` 绑定 `T` 且不动原集合——`.clone()` 在切片上做不到这件事，它克隆的是那个引用，
迭代出来还是引用。这个后端发射的元素类型都是 Clone：标量是 Copy，String 是 Clone，
struct 和 enum 都带 `#[derive(Debug, Clone)]`。

**需要你做什么：** 不需要。没有任何程序因此从「能编译」变成「不能编译」——
之前能过 rustc 的形状只有「把元素加起来」，它现在照样过。

**已知残留（不在这次改动里）：** `Vec<String>` 的数组字面量发射成 `vec!["a", "b"]`，
元素是 `&str` 而不是 `String`，rustc 在到达循环之前就拒绝了。这是数组字面量的问题，不是循环的，
语料里还没有任何一个示例是字符串数组——按本项目的规矩，它应该由自己的语料程序带着自己的提交来修。
