# 📜 Loop Engineering Contract

- **Current Goal**: 阶段十五：物理验证契约全量对齐与基线重构 v3.37.0 (1) 重构 Loop_Contracts.md，将基于 `codegen/local_e2e_test.go` 的本地 E2E 物理验证作为 11 门非主力语言统一的物理验证基线；(2) 修正物理验证红线准则第 1 条，移除废弃的 Docker E2E 隔离描述，替换为本地轻量级 E2E 物理编译运行守则；(3) 保持跨文件完备性与 100% 诚实一致性。
- **Verification Command**: go test ./...
- **Status**: Complete
- **Gatekeeper Status**: Test: Passed | Audit: Audited

---

## 🔬 阶段七：非主力后端物理开发契约 (Local E2E 物理验证基线)

### 1. 物理验证命令定死清单

| 语言后端 | 验证顺序 | 本地物理验证执行命令 (定死) | 验证状态契约 (全量完备对齐) |
| :--- | :--- | :--- | :--- |
| **Java** | 1 (首攻) | `javac Main.java Service.java Models.java && java Main` | **AST 转换与本地 E2E 物理验证已接入 (`local_e2e_test.go`)** |
| **C#** | 2 | `dotnet new console --force && Remove-Item Program.cs && dotnet run` | **AST 转换与本地 E2E 物理验证已接入 (`local_e2e_test.go`)** |
| **Kotlin** | 3 | `kotlinc main.kt service.kt models.kt -include-runtime -d main.jar && java -jar main.jar` | **AST 转换与本地 E2E 物理验证已接入 (`local_e2e_test.go`)** |
| **Swift** | 4 | `swiftc main.swift service.swift models.swift -o main_out && ./main_out` | **AST 转换与本地 E2E 物理验证已接入 (`local_e2e_test.go`)** |
| **Dart** | 5 | `dart run main.dart service.dart models.dart` | **AST 转换与本地 E2E 物理验证已接入 (`local_e2e_test.go`)** |
| **Zig** | 6 | `zig build-exe main.zig && ./main` | **AST 转换与本地 E2E 物理验证已接入 (`local_e2e_test.go`)** |
| **Nim** | 7 | `nim c -r main.nim` | **AST 转换与本地 E2E 物理验证已接入 (`local_e2e_test.go`)** |
| **Julia** | 8 | `julia main.jl` | **AST 转换与本地 E2E 物理验证已接入 (`local_e2e_test.go`)** |
| **PHP** | 9 | `php main.php` | **AST 转换与本地 E2E 物理验证已接入 (`local_e2e_test.go`)** |
| **Ruby** | 10 | `ruby main.rb` | **AST 转换与本地 E2E 物理验证已接入 (`local_e2e_test.go`)** |
| **Lua** | 11 | `lua main.lua` | **AST 转换与本地 E2E 物理验证已接入 (`local_e2e_test.go`)** |
| **Android** | 12 | `gradlew assembleDebug` | **AST 转换与本地 E2E 物理验证已接入 (`local_e2e_test.go`)** |

### 2. 物理验证红线准则
1. **统一的本地 E2E 物理验证基线**：所有的 E2E 物理编译与运行断言均统一收敛于 `codegen/local_e2e_test.go` (`TestLocalE2EWorkspaceDogfood`)。测试会自动检测当前主机环境是否存在对应语言编译器/运行时：若存在则自动执行真机物理编译与输出断言，若无则优雅 Skip 避让。彻底摒弃对重型 Docker 容器的强依懒，兼顾极速单体单元测试与真实环境物理重测能力。
2. **渐进式迭代**：严禁并行铺开，严禁一次性写完统一测试。必须按上述 **1 (首攻 Java) 到 11 (Lua)** 的物理顺序，逐个语言完成 `Codegen支持 -> 移出拦截 -> 物理跑通 -> 记录成果并归档` 的完整闭环后，才准进下一门语言。
3. **日志归档**：物理验证通过后，在 `Loop_Contracts.md` 下记录该语言物理通过的 commit 及时间。

