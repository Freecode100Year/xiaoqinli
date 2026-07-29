# 📜 Loop Engineering Contract

- **Current Goal**: 阶段十二：Codegen 物理推导式分支生成与 learned Diagnostic 修复覆盖 v3.31.0 (1) 在 codegen/python.go 中重构 emitForStmt，真正的物理分支逻辑：PreferComprehension 为 true 时生成列表推导式 extend([elem for item in iterable])，为 false 时生成传统 3 行 for 循环 + append；(2) 修正 compiler.wrapDiag 中 fix == "" 逻辑漏洞，只要 InspectDiagnosticFixes 中有学习到的修法建议，优先覆盖默认兜底文案；(3) 全量物理单元测试 100% 跑通闭环。
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
