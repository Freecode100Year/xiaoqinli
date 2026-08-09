# 发布说明勘误

已推送的提交信息无法在不重写历史的前提下修改。写错的地方记在这里。

## f516e1e `release: v3.42.0`

原文声称：

> go build, go vet and go test are green under both the default and the
> metrics build tags.

**这句话在写下时不成立。** 它只在作者本机成立——本机缺少多个目标语言的工具链，
相关子测试被 `t.Skip` 跳过，绿灯来自没有真正运行的测试。同一时刻 CI 是红的。

该版本的功能描述（ExternDecl、`targets`、Go 后端限定名、目标支持校验、局部
lambda 调用）都属实，只有测试状态那一句失实。后续提交已修好当时 CI 上的失败，
现在 CI 确实全绿，但那不能追溯地让原话变成真的。

教训：只有 CI 的结果能作为"全绿"的依据。本机跑测试时，被跳过的子测试要当作
未验证，而不是通过。
