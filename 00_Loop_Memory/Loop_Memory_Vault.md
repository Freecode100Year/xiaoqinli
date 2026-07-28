# 🧠 Loop Engineering Memory Vault

## 🚫 Avoid-List (踩坑禁区 - 禁止重复尝试)
<!-- 执行 Agent 在验证失败后追加,格式建议: -->
<!-- - [YYYY-MM-DD] 不要做 XX,原因: YY -->

## 🛠️ Proven Patterns (已验证成功的模式)
<!-- 审计通过后可从中提炼可复用的实现模式 -->

## 🔄 Current Task Execution Log
### Iteration History
<!-- 执行 Agent 每轮追加: -->
<!-- - **Iter N**: <改了什么> -> *Result: <测试结果>* -->
- **Iter Stage 8**: 1) 收敛 `compiler.allTargetInfos` 与 `TargetInfo` 为 43+ 目标语言唯一数据源；2) 重构 `server/mcp.go` 中的 `toolTargets()`，删除死锁与漂移硬编码；3) 补全 `LoadLocalState` / `SaveLocalState` 及 `evolution.LoadEvolutionState` 与 Write-Through 持久化落盘到 `.xql/`；4) 配置 `.gitignore` 排除 `.xql/`；5) 修复 `sync.RWMutex` 重入死锁 bug。 -> *Result: go test ./... 100% 物理测试 PASS，xql.exe 成功同步全局二进制* [STATUS: READY FOR AUDIT]
