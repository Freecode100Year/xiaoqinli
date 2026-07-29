# 📜 Loop Engineering Contract

- **Current Goal**: 阶段十三：报错摘要文本 (content[0].text) 与结构化 Diagnostics 修复建议完全对齐 v3.32.0 (1) 重构 compiler.formatDiagError 与 server.toolErrorResult，在返回编译/校验错误时，将经过 wrapDiag 覆盖后的最新 SuggestedFix 重新格式化并同步更新至 content[0].text 摘要文本与 Error 字符串；(2) 物理解决 Agent/人类在默认视图下看到的仍然是旧文案的问题，实现 100% 体验与文本对齐。
- **Verification Command**: go test ./...
- **Status**: Complete
- **Gatekeeper Status**: Test: Passed | Audit: Audited

---

## 🔬 阶段七：非主力后端物理开发契约 (Docker 隔离)

### 1. 物理验证命令定死清单

| 语言后端 | 验证顺序 | Docker 验证镜像 | 物理验证执行命令 (定死) |
| :--- | :--- | :--- | :--- |
| **Java** | 1 (首攻) | `eclipse-temurin:17-alpine` | `javac Main.java Service.java Models.java && java Main` | **已于 2026-07-07 00:33 物理通过** |
| **C#** | 2 | `mcr.microsoft.com/dotnet/sdk:7.0-alpine` | `dotnet new console --force && rm -f Program.cs && dotnet run` | **已于 2026-07-07 00:52 物理通过** |
| **Kotlin** | 3 | `zenika/kotlin:alpine` | `kotlinc main.kt service.kt models.kt -include-runtime -d main.jar && java -jar main.jar` | **已于 2026-07-07 01:06 物理通过** |
| **Swift** | 4 | `swift:5.8-slim` | `swift main.swift service.swift models.swift` | **已于 2026-07-07 01:23 物理通过** |
| **Dart** | 5 | `dart:stable-alpine` | `dart run main.dart service.dart models.dart` | **已于 2026-07-08 15:06 物理跑通** |
| **Zig** | 6 | `ziglang/zig:0.11.0-alpine` | `zig run main.zig service.zig models.zig` | **已于 2026-07-08 15:19 物理跑通** |
| **Nim** | 7 | `nimlang/nim:alpine` | `nim c -r main.nim` | **已于 2026-07-26 22:12 物理跑通** |
| **Julia** | 8 | `julia:alpine` | `julia main.jl` | **已于 2026-07-26 22:12 物理跑通** |
| **PHP** | 9 | `php:8.2-alpine` | `php main.php` | **已于 2026-07-26 22:12 物理跑通** |
| **Ruby** | 10 | `ruby:3.2-alpine` | `ruby main.rb` | **已于 2026-07-26 22:12 物理跑通** |
| **Lua** | 11 | `lua:5.4-alpine` | `lua main.lua` | **已于 2026-07-26 22:12 物理跑通** |

### 2. 物理验证红线准则
1. **严格的 Build Tag 隔离**：所有的 Docker E2E 测试均被移入 `codegen/docker_e2e_test.go`，前置头部标记 `//go:build docker_e2e`。保证日常 `go test ./...` 速度不受拖累。
2. **渐进式迭代**：严禁并行铺开，严禁一次性写完统一测试。必须按上述 **1 (首攻 Java) 到 11 (Lua)** 的物理顺序，逐个语言完成 `Codegen支持 -> 移出拦截 -> 物理跑通 -> 记录成果并归档` 的完整闭环后，才准进下一门语言。
3. **日志归档**：物理验证通过后，在 `Loop_Contracts.md` 下记录该语言物理通过的 commit 及时间。
