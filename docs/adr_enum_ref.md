# ADR: 引用一个 enum 变体

## 状态

已采纳。首次由 `examples/enum_match.xql.json` 施加约束。

## 背景

`EnumDecl` 从很早就在 AST 里，35 个后端都为它写了发射代码。但语料里从来没有一个程序**引用过**某个变体——
`codegen/codegen_test.go` 里那个 "enum + match" 测试程序声明了 `Color` 之后，match 的是一个 `Int`。

于是声明侧和引用侧从未被放在一起看过。声明侧各挑各的拼法：

| 后端 | `emitEnumDecl` 写出来的东西 |
|---|---|
| go | `ColorRed Color = iota` |
| c / fortran | `Color_Red` |
| rust / cpp | `enum Color { Red }` |
| zig | `pub const ColorRed: i64 = 0` |
| julia | `@enum Color Red Green Blue`（变体绑定在外层作用域） |
| php / lua / perl / tcl / awk / bash | 顶层的整数常量 `Red = 0` |
| ruby | `module Color; Red = 0; end` |
| ocaml / pascal | `type color = Red \| Green`、`Color = (Red, Green)` |
| elixir | 一行注释；变体是 atom `:red` |
| shortcut | 一个叫 `Color_Red` 的变量 |
| java / csharp / kotlin / swift / dart / ts / d / vala / groovy / nim / crystal / python | 真正的 enum 类型 |

而引用侧走的是 `emitMemberExpr`——那是为「值上的字段访问」写的。结果 22 个后端发射出自己从未定义过的东西，
各有各的死法：go 写出 `Color.red`（点是 Go 不允许的，小写来自字段可见性规则）、perl 和 tcl 解引用不存在的变量、
bash 索引没人赋过值的数组、elixir 调用它自己没生成的模块、powershell 读 `$Color` 谁也匹配不上。
三个后端连**类型**位置都是错的：fortran 声明 `type(Color)`、zig 写 `c: Color`、php 写 `Color $c`，
这三个名字它们自己都没定义过。

## 决定

**AST 里引用一个 enum 变体的唯一形式是 `MemberExpr{Object: Ident(<enum name>), Field: <variant>}`**，
写成 JSON 就是：

```json
{ "kind": "MemberExpr", "object": { "kind": "Ident", "name": "Color" }, "field": "Green" }
```

这个形式在模式位置和表达式位置是同一个：match 的 arm pattern、函数实参、变量初值都用它。

**后端有义务让引用和自己的声明对上。** `codegen/util.go` 提供 `collectEnums` 与 `enumRef`：
后端在 `Generate` 入口收一份 enum 索引，在 `MemberExpr` 的发射入口问一句「这是不是某个已声明 enum 的变体」，
是的话就按自己 `emitEnumDecl` 用过的拼法写出来。

`enumRef` 只在**对象是一个裸标识符、且这个标识符是已声明的 enum、且字段名是它的变体之一**时才成立。
一个恰好和变体同名的结构体字段不受影响——它的对象不是 enum 名。`TestEnumRefDoesNotSwallowFieldAccess` 守着这条。

**enum 作为类型时**，若后端并不真的声明一个类型（fortran、zig、php 把变体降成整数常量），
类型位置就用该语言的整数类型，而不是把 enum 名原样透传。

**Java 是唯一一个把变体拼成两种样子的后端**：`switch` 的 case 标签必须是非限定名（`case Red:`），
Java 21 之前限定名是编译错误；其余位置必须限定（`Color.Green`）。

## 被否掉的方案

- **在编译前把 `Color.Red` 重写成一个标识符。** 各后端的拼法不同，重写只能是 per-target 的；
  而就地改写会污染同一份 AST 后续目标的编译（矩阵测试会对同一棵树连编 38 次），深拷贝又要为此引入一条新路径。
- **给 AST 加一个 `EnumRef` 节点。** 更显式，但要动 `ast` 包、两套 codec、以及每个后端的 switch——
  代价和现在这条一样，收益只是名字更好看。`MemberExpr` 已经能表达这件事，缺的只是后端去问。

## 影响

`docs/breaking_changes.md` 记了这次的对外变化：22 个后端的产物变了。没有任何程序从「能编译」变成「不能编译」——
之前那些产物本来就跑不起来。
