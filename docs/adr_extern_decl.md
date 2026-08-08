# ADR 003: 宿主函数声明（ExternDecl）

## 状态
Accepted

## 背景

在引入本决策之前，AST 中没有任何方式表达「这个函数由宿主平台提供」。类型检查器把所有无法在 `funcTable`、内建表或导入图中解析的调用一律判为 `XQL_E201: undefined function`，能力检查器则在严格模式下报 `XQL_E303: cannot verify capability for unresolved call`。

后果是转译器的本质功能在真实程序上不成立：

- 任何调用 `fetch`、`time.Sleep`、`document.createElement` 的程序都无法通过校验，因而无法编译到任何目标；仓库自带的 `examples/clock.xql.json`、`examples/chrome_network.xql.json`、`examples/chrome_volume.xql.json` 三个示例在全部 46 个目标上都失败。
- 更严重的是安全性倒挂：宿主调用恰恰是程序真正接触外部世界的唯一边界，而能力系统对它完全无话可说。能力检查只覆盖了本程序内部函数之间的调用——即最不需要被约束的那一部分。
- 唯一的绕过方式是 `--no-strict-caps`，即用关闭安全检查来换取可编译性。

## 决策

### 1. AST 节点定义（`ExternDecl`）

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

- `name`：与 `CallExpr.callee` **逐字**匹配。因此 `time.Sleep`、`document.head.appendChild` 按调用时的原样声明，不做任何分段解释。
- `params`：**可整体省略**。省略表示签名不受检查（变参 / 重载的宿主函数）；显式给出（含 `[]`）则按普通调用检查参数个数与类型。二者的区别由 `HasParams` 记录，`"params": []` 是一个受检的零参签名。
- `returnType`：省略表示返回类型未知，调用点不参与类型比较。
- `effects` / `grant`：extern 没有函数体可供推断，因此二者都是声明式的。
- `targets`：非空时限定该 extern 只在列出的后端可用。
- `method`：见下文。
- 携带 `body` 字段的 `ExternDecl` 在解析期即被拒绝（`XQL_E101`）——实现由宿主提供，声明中不应出现。

### 2. 解析顺序

调用解析按「局部绑定 → 精确 extern → 内建 → 本模块函数 → 导入模块 → extern 方法」的顺序进行：

- 局部优先，使得绑定了 Lambda 的变量可以按名字调用（此前会被误判为 undefined function）。
- 精确 extern 早于导入解析，保证 `time.Sleep` 被当作一个宿主名字，而不是「模块 time 中的 Sleep」。
- extern 方法放在最后：带点的调用更可能是模块引用，只有在无人认领时才把末段读作宿主方法。

### 3. 宿主方法（`"method": true`）

`res.json()`、`hud.classList.add()` 的接收者是编译器无法定型的运行时值。此时声明方法本身：

```json
{ "kind": "ExternDecl", "name": "json", "method": true, "effects": ["network"], "grant": ["network"] }
```

匹配规则是「callee 的最后一段等于 name」。接收者不受验证——这是诚实的取舍，编译器确实无从验证它——但**授权仍在每个调用点被强制检查**，即安全相关的那一半得以保留。方法名不允许带点（`XQL_E101`）。

### 4. 跨模块语义

extern 不按模块划分命名空间：它指称的是宿主上**唯一**的那个函数，因此

- 导入模块中声明的 extern 会被提升到导入方，按原名可调用；
- 多个模块以完全相同的签名声明同一 extern 时合并为一条；
- 签名不一致时报 `XQL_E202`。若放任其一静默胜出，实际生效的将是较弱的那份授权声明，能力检查会被悄然削弱。
- 链接器的别名剥离对 extern 名字整体豁免：`platform.log` 是一个逐字的宿主符号，不是「导入为 platform 的模块中的 log」。

### 5. 代码生成

`ExternDecl` 在分发层（`codegen.Generate` / `codegen.GenerateProject`）统一剥离，46 个后端都看不到该节点，无需任何改动。剥离前先校验 `targets`：

- 只有**被实际调用**的 extern 才受目标限制约束——未被调用的其它平台声明不构成本目标的问题；
- 被调用且目标不在列表内时报 `XQL_E402`。这把 extern 唯一可能出错的方式（把浏览器 API 编译到根本没有该 API 的宿主）从「运行时才崩的产物」变成了编译期错误。

## 影响

- 三个此前在全部 46 个目标上失败的示例恢复可编译，且其宿主调用现在都携带显式授权。
- 能力检查首次覆盖了真正的外部边界。`examples/clock.xql.json` 中删去 `main` 的 `clock` 授权即报 `XQL_E301`。
- 顺带修正了两处会导致静默错误产物的缺陷：`codegen.walkNodes` 遗漏 `ExprStmt` 分支（使目标能力校验漏检表达式语句中的构造）；Go 后端会丢弃任意二段式限定调用的前缀（`platform.log(...)` 被生成为 `log(...)`）。
