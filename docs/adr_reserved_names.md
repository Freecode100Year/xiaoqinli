# ADR: 标识符与目标语言保留字的冲突在 AST 层解决

## 状态

已采纳。首次由 `examples/switch_stmt.xql.json` 施加约束，`examples/reserved_names.xql.json` 是专门的语料。

## 背景

`examples/switch_stmt.xql.json` 里的函数叫 `label`。这在 XQL 里是一个普通名字，在 Pascal 里是关键字。
CI 上 fpc 在第 3 行就整份拒绝：

```
prog.pas(3,10) Fatal: Syntax error, "identifier" expected but "LABEL" found
```

`validate` 说这份 AST 没问题——它确实没问题。问题不在 AST 里，也不在某一个后端里：**38 个后端全都把
标识符原样写出去**，于是「在一种语言里普通、在另一种语言里保留」的名字，会产出一个连解析都过不去的文件。

这不是一个已知但没修的小毛病，而是一条横跨全部后端的缺陷：它对每一个目标语言都成立，只是要等到语料里
恰好出现那一个词才会暴露。`label` 撞的是 Pascal，`class` 会撞 Java/Python/Ruby 等十几个，`in` 会撞
Python/Rust/Kotlin/Swift/Lua/PHP 等。

## 决定

**不在 38 个发射器里各做一次转义，而是在 codegen 入口做一次 AST 改写。**

冲突是 (程序, 目标语言) 这一对的性质，不是某个后端的性质——一次树改写就同时修好全部后端，这与
`codegen/lower_switch.go` 的取舍是同一笔账。实现分两个文件：

- `codegen/reserved.go`：每种语言的关键字表。收的是**语法保留的词**，不是标准库碰巧用到的名字——
  多改会白白搅动产物，少改则留下一个解析不了的程序。Pascal、Fortran、批处理、PowerShell 大小写不敏感，
  单列在 `caseInsensitiveLanguages` 里，否则 `Label` 这种写法会漏网。JSON / CLI 类目标（`shortcut`、
  `tccli`）没有关键字表，整棵树原样返回。
- `codegen/rename_reserved.go`：收集程序**自己声明**的名字，与关键字表求交，把声明处和每一处引用一起改名，
  后缀 `_`（若与程序里已有的名字或另一个关键字再撞，继续加 `_`）。

只改程序自己声明的名字。`extern` 声明的是宿主提供的符号，`println` 这类内置由后端自己发射——改这两种名字
是把调用改坏，不是把它修好。所以字段名分成两个集合：`values`（表达式位置：函数、变量、参数、循环变量、
类型、枚举）与 `members`（点号之后：结构体/类字段、枚举变体），`MemberExpr.Field` 只在程序自己声明过该
字段时才动，宿主对象上的方法名不受影响。

改写同样是**函数式**的，理由与 switch 降级一样：`Generate` 对同一棵 AST 每个目标调用一次，原地改写会让
下一个目标编译到上一个目标的改名结果——先编 Pascal 再编 Python，Python 就会莫名其妙发射 `label_`。

## 后果

- 一个名字与目标语言关键字冲突的程序，现在能编译到全部 37 个目标（`tccli` 拒的是例子里的 `>`，不是名字）。
- 新语料 `examples/reserved_names.xql.json`：函数 `end`、参数 `in`、变量 `class`。它不测任何构造，
  只测「后端拿到一个自己拼不出来的名字之后还能不能工作」。
- 顺带暴露并修掉了 Haskell 的一个盲点：纯函数体里的 `let` 后面从来没有 `in`——在这个语料之前，
  语料里的纯函数体全都是单个 return，GHC 从没见过带局部绑定的纯函数。同时 `emitPureBranch` 里
  「不是 VarDecl 就静默丢掉」的分支改成报 `XQL_E402`：纯 let 只装得下绑定和结果，装不下语句序列，
  静默丢弃会把程序变成一个默默少做事的程序。
- Java 的 `emitSwitchStmt` 一并改掉：Java 的 switch 选择器不能是 `long`，而这个 AST 里 `Int` 是 64 位。
  它现在把 switch 交给 `emitMatchExpr`，Int 走 if/else 链、枚举走原生 switch 的判断只留一处。

## 已知边界

- 关键字表是人写的，不是从各语言 grammar 自动导出的。漏一个词，症状仍然是「产物不解析」，
  修法是往表里补一个词——而不是再去改某个后端。
- 一个用户函数与某个 `extern` 同名、且该名字又是关键字时，改名会让两者不再一致。这种冲突是程序自己的问题，
  这个 pass 不去猜。
