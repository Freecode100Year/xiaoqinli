# 📜 Loop Engineering Contract

- **Current Goal**: 阶段七：补齐 11 个非主力后端特性，打通 Docker 隔离物理编译与运行，首发攻克 Java。
- **Verification Command**: go test -tags docker_e2e ./codegen -run TestDockerE2EWorkspaceDogfood -v
- **Status**: Progress
- **Gatekeeper Status**: Test: Pending | Audit: Audited

---

## 🔬 阶段七：非主力后端物理开发契约 (Docker 隔离)

### 1. 物理验证命令定死清单

| 语言后端 | 验证顺序 | Docker 验证镜像 | 物理验证执行命令 (定死) |
| :--- | :--- | :--- | :--- |
| **Java** | 1 (首攻) | `eclipse-temurin:17-alpine` | `javac Main.java Service.java Models.java && java Main` | **已于 2026-07-07 00:33 物理通过** |
| **C#** | 2 | `mcr.microsoft.com/dotnet/sdk:7.0-alpine` | `dotnet new console --force && rm -f Program.cs && dotnet run` | **已于 2026-07-07 00:52 物理通过** |
| **Kotlin** | 3 | `zenika/kotlin:alpine` | `kotlinc main.kt service.kt models.kt -include-runtime -d main.jar && java -jar main.jar` |
| **Swift** | 4 | `swift:5.8-slim` | `swift main.swift service.swift models.swift` |
| **Dart** | 5 | `dart:stable-alpine` | `dart run main.dart service.dart models.dart` |
| **Zig** | 6 | `ziglang/zig:0.11.0-alpine` | `zig run main.zig service.zig models.zig` |
| **Nim** | 7 | `nimlang/nim:alpine` | `nim c -r main.nim` (Nim会自动解析模块导入) |
| **Julia** | 8 | `julia:alpine` | `julia main.jl` (Julia会自动解析模块导入) |
| **PHP** | 9 | `php:8.2-alpine` | `php main.php` (PHP由main主控加载) |
| **Ruby** | 10 | `ruby:3.2-alpine` | `ruby main.rb` (Ruby由main主控加载) |
| **Lua** | 11 | `lua:5.4-alpine` | `lua main.lua` (Lua由main主控加载) |

### 2. 物理验证红线准则
1. **严格的 Build Tag 隔离**：所有的 Docker E2E 测试均被移入 `codegen/docker_e2e_test.go`，前置头部标记 `//go:build docker_e2e`。保证日常 `go test ./...` 速度不受拖累。
2. **渐进式迭代**：严禁并行铺开，严禁一次性写完统一测试。必须按上述 **1 (首攻 Java) 到 11 (Lua)** 的物理顺序，逐个语言完成 `Codegen支持 -> 移出拦截 -> 物理跑通 -> 记录成果并归档` 的完整闭环后，才准进下一门语言。
3. **日志归档**：物理验证通过后，在 `Loop_Contracts.md` 下记录该语言物理通过的 commit 及时间。
