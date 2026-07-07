# 📜 Loop Engineering Contract

- **Current Goal**: 阶段七：补齐 11 个非主力后端特性，打通本地物理编译与运行，首发攻克 Java、C#、Kotlin 和 Swift。
- **Verification Command**: go test ./codegen -run TestCollectionCodegenAll -v
- **Status**: Progress
- **Gatekeeper Status**: Test: Pending | Audit: Audited

---

## 🔬 阶段七：非主力后端物理开发契约 (本地物理与生成验证)

### 1. 物理验证命令定死清单

| 语言后端 | 验证顺序 | 物理验证执行命令 (定死) | 状态 |
| :--- | :--- | :--- | :--- |
| **Java** | 1 (首攻) | `javac Main.java Service.java Models.java && java Main` | **已于 2026-07-07 00:33 物理通过** |
| **C#** | 2 | `dotnet new console --force && rm -f Program.cs && dotnet run` | **已于 2026-07-07 00:52 物理通过** |
| **Kotlin** | 3 | `kotlinc main.kt service.kt models.kt -include-runtime -d main.jar && java -jar main.jar` | **已于 2026-07-07 01:06 物理通过** |
| **Swift** | 4 | `swift main.swift service.swift models.swift` | **已完成 Codegen 生成验证** |
| **Dart** | 5 | `dart run main.dart service.dart models.dart` | |
| **Zig** | 6 | `zig run main.zig service.zig models.zig` | |
| **Nim** | 7 | `nim c -r main.nim` | |
| **Julia** | 8 | `julia main.jl` | |
| **PHP** | 9 | `php main.php` | |
| **Ruby** | 10 | `ruby main.rb` | |
| **Lua** | 11 | `lua main.lua` | |

### 2. 物理验证红线准则
1. **彻底废除 Docker 容器**：依照用户要求，全面移除并永久废除 Docker 容器相关测试。
2. **渐进式迭代**：严禁并行铺开，必须按上述 **1 到 11** 的物理顺序，逐个语言完成 `Codegen支持 -> 移出拦截 -> 静态与本地编译跑通 -> 记录成果并归档` 的完整闭环后，才准进下一门语言。
