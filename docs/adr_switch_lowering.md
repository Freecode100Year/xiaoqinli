# ADR: `SwitchStmt` 是 `MatchExpr` 的语法糖

## 状态

已采纳。首次由 `examples/switch_stmt.xql.json` 施加约束。

## 背景

`SwitchStmt` 从很早就在 AST 里，`ast/nodes.go` 有它的解析器，38 个目标里有 13 个为它写了发射代码。
语料里从来没有一个程序用过它——按 `ast/nodes.go` 的 kind 清单去减 `examples/*.xql.json` 里实际出现的
kind，`SwitchStmt` 就在差集里。

补上一个最小语料（三个 case，各赋一个不同的字符串，外加一个 default）编译到全部 38 个目标，结果是：

| 目标 | 结果 |
|---|---|
| go rust ts js py java csharp kotlin swift dart zig android ios | 编译通过，产物正确 |
| lua ruby php nim julia | 走到语句发射器的 default 分支，`XQL_E401: unsupported node SwitchStmt` |
| 其余 20 个 | 被 `codegen/util.go` 的 `validateNodesForTarget` 挡在门口，同样是 `XQL_E401` |

也就是说 25 个目标根本没有 `emitSwitchStmt`。这不是「翻译错了」，是「压根没有」——
与 `MatchExpr`、`EnumDecl` 那两次不同，编译器是**响亮地失败**的，没有静默降级。

## 决定

**不给这 25 个后端各写一份 `emitSwitchStmt`，而是在 codegen 入口把 `SwitchStmt` 改写成 `MatchExpr`。**

两个节点装的是同一样东西：一个判别值、一组 值/语句体 对、一个兜底分支。而这 25 个目标**每一个**
都已经有 `MatchExpr` 发射器，并且 `examples/match_arms.xql.json` 和 `examples/enum_match.xql.json`
每次跑 conformance 都在验它——那两个语料正是上一轮把 13 个后端的 match 修对的原因。

再写 25 份发射器，就是再造 25 个「写过、没跑过」的盲点，而这正是这个语料库反复抓到的那笔烂账。

实现在 `codegen/lower_switch.go`：

- `nativeSwitchTargets` 列出自己会发射 switch 的 13 个目标，它们原样保留——
  于是同一个语料同时验证了原生路径和降级路径，两条路的期望输出是同一份。
- default 分支变成 `_` 分支并**移到最后**。移动是安全的（其余分支都是值匹配，default 恰好在都不匹配时命中，
  写在哪里都一样），而且是必需的：rust、ocaml、haskell 的 match 按顺序读，通配之后的分支是死代码。
- 没有 default 的 switch 变成没有通配分支的 match。不凭空补一条谁也没写的兜底。
- 改写是**函数式**的，不原地改树。`Generate` 对同一棵 AST 每个目标调用一次（conformance 解析一个文件
  编译到全部 38 个目标），原地改写会让第一个目标之后的每个目标都在编译一棵已经被降级过的树，
  原生 switch 的后端会悄悄不再发射 switch。通往 switch 的节点浅拷贝，其余共享。

`validateNodesForTarget` 里的 `*ast.SwitchStmt` 一支随之删除：降级之后，能走到那里的 switch 已经不存在。

## 后果

- 25 个目标从「拒绝」变成「能编译并且跑出正确结果」，`compiler/matrix_test.go` 里
  `switch_stmt.xql.json` 的拒绝名单只剩 `haskell` 和 `tccli`——这两个拒的是 match 而不是 switch，
  它们对 `match_arms.xql.json` 也是同样的拒绝理由（haskell 在纯函数里没有可变绑定，tccli 没有分支）。
- `android` 是唯一一个能编译这个语料却不能编译 `match_arms.xql.json` 的目标：它有原生 switch、没有 `MatchExpr`。
  这也是本 ADR 保留 13 个原生发射器、而不是把 `SwitchStmt` 一路降级到底的具体理由之一。
- 判别表达式在降级后被求值几次，取决于各后端的 `MatchExpr` 怎么写——awk 一类把 match 展成 if/else if 链的
  后端会重复求值。这是 `MatchExpr` 本来就有的性质，降级没有引入新的问题，但也没有解决它。
