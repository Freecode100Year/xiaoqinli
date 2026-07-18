# 🚀 Xiaoqinli v3.13.0 & AgentCLI Kickoff 会议议程

> **日期**：2026-07-11（周六）
> **时长**：60 分钟
> **参与者**：xiaoqinli 维护者、AgentCLI 项目经理、核心开发团队

---

## 📋 议程

### 1. 开场与项目回顾 (10 分钟)

- 🔄 xiaoqinli 从 v3.2.x 到 v3.13.0 的发展历程快速回顾
- 🎯 v3.13.0 库化导出的核心成果

### 2. 方案 B 架构决策确认 (15 分钟)

- 📐 方案 B 的核心理念：**分离关注点**
  - **xiaoqinli** = 纯转译器核心/库
  - **AgentCLI** = 独立仓库，消费 compiler 包
- ✅ 团队投票结果确认
- ⚖️ 与方案 A 的对比和选择理由

### 3. 库化分支技术计划 (15 分钟)

- 📦 `go.mod` 模块路径升级 (`github.com/Freecode100Year/xiaoqinli`)
- 📝 compiler 包公共 API 的 GoDoc 完善
- 🧪 Example 测试覆盖
- 🔍 `ast`/`check`/`codegen` 包的导出策略评估
- 🔒 向后兼容性保证

### 4. AgentCLI 新仓库规划 (10 分钟)

- 🏗️ 仓库命名与组织结构
- ⚙️ 技术栈选择
- 🌐 W1（第一周）网络层开发计划
- 🔗 如何消费 `xiaoqinli/compiler` 包

### 5. 发布时间线与里程碑 (5 分钟)

| 日期 | 里程碑 |
|------|--------|
| 2026-07-14 | xiaoqinli v3.13.0 正式发布 |
| 2026-07-14 | AgentCLI W1 网络层代码审查 |
| 2026-07-14 | 第一周进度同步 |
| 后续 | 迭代节奏待定 |

### 6. 开放讨论 & Q&A (5 分钟)

- ⚠️ 风险识别与缓解
- 📌 行动项分配
- 📅 下次同步时间确认

---

## 📚 会前准备

- [ ] 审阅 `QUICK_REFERENCE.txt`
- [ ] 审阅 `EXECUTIVE_SUMMARY.md`
- [ ] 审阅 `REFACTOR_PLAN.md`（`feature/lib-refactor` 分支）
- [ ] 准备各自负责模块的问题和建议

## 📤 会后交付物

- [ ] 确认最终发布时间线
- [ ] 分配各模块负责人
- [ ] 创建 AgentCLI GitHub 仓库
- [ ] 合并库化分支到 master
