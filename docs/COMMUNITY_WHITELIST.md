# Xiaoqinli (xql) — GitHub 社区白名单 PR 提交模板指南

本文档收录了将 `xiaoqinli` 提交至各大 GitHub 开源 Awesome 列表与官方 Registry 的标准 PR (Pull Request) 模版。

---

## 1. 提交至 Awesome-MCP-Servers (MCP 官方/社区列表)

* **目标仓库**：[punkpeye/awesome-mcp-servers](https://github.com/punkpeye/awesome-mcp-servers) 或 [modelcontextprotocol/servers](https://github.com/modelcontextprotocol/servers)
* **分类**：`Development Tools` 或 `Compilers & Code Generation`

### PR 描述 (PR Description)

```markdown
### Add Xiaoqinli (xql) Transpiler MCP Server

- **Name**: Xiaoqinli (xql)
- **Repository**: https://github.com/Freecode100Year/xiaoqinli
- **Description**: An AST-First, zero-runtime transpiler that converts `.xql.json` ASTs into source code across 38 languages with deterministic compiler-level verification. Supports both stdio and HTTP MCP modes.
- **Language**: Go
- **License**: MIT
```

### README.md 填入项 (Markdown snippet)

```markdown
- [Xiaoqinli (xql)](https://github.com/Freecode100Year/xiaoqinli) - AST-First transpiler for AI Agents supporting 38 target languages with deterministic type, effect, and capability checks (`stdio` & `HTTP`).
```

---

## 2. 提交至 Awesome-Go (Go 语言全球顶级开源列表)

* **目标仓库**：[avelino/awesome-go](https://github.com/avelino/awesome-go)
* **分类**：`Compilers` 或 `Development Tools`

### PR 描述 (PR Description)

```markdown
### Add Xiaoqinli (xql) to Compilers section

* [xiaoqinli](https://github.com/Freecode100Year/xiaoqinli) - AST-First, zero-runtime security transpiler for AI Agents supporting 38 target languages with deterministic static verification.

**Checks:**
- [x] Go module enabled
- [x] Continuous Integration passes
- [x] Includes unit tests and documentation
- [x] Single Go binary with zero third-party runtime dependencies
```

---

## 3. 提交至 Awesome-AI-Agents

* **目标仓库**：[e2b-dev/awesome-ai-agents](https://github.com/e2b-dev/awesome-ai-agents) 或 [WormholeLabs/awesome-ai-agents](https://github.com/WormholeLabs/awesome-ai-agents)
* **分类**：`Developer Tools & Infrastructure`

### Markdown Snippet

```markdown
- [Xiaoqinli](https://github.com/Freecode100Year/xiaoqinli) - Deterministic AST-First transpiler infrastructure built for AI Agents to produce verified, error-free code in 38 targets.
```

---

## 4. GitHub Repository Topics 推荐设置

建议在 GitHub 仓库主页右侧的 **About -> Topics** 中添加以下标签：

`transpiler`, `ast`, `mcp`, `mcp-server`, `compiler`, `golang`, `ai-agent`, `code-generation`, `static-analysis`, `multi-language`
