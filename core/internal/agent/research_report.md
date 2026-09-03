# 调研报告: Agent 技能资源吸收

## 1. 调研目标
吸收开源社区优秀 Agent 技能资源，并将其转化为本项目 `core/internal/agent` 的原生工具。

## 2. 调研对象与价值评估
| 资源名称 | 核心能力 | 有用性判断 | 适配策略 |
| :--- | :--- | :--- | :--- |
| **VoltAgent** | 高性能 Agent 执行流 | **高** (普适/高性能) | 转化为 `exec.go` 中的执行上下文优化 |
| **awesomeclaude** | 丰富的 Claude 工具集 | **高** (生态位/可复制) | 转化为 `tools.go` 中的 `DefaultTools` 集合 |
| **microsoft agent-skills** | 标准化技能定义 | **中** (文档化/标准化) | 转化为 `skills/` 目录及 `skill_manager.go` |
| **awesome-mcp** | Model Context Protocol 扩展 | **极高** (未来价值/生态) | 转化为 `mcp/` 服务与 `mcp_manager.go` |

## 3. 设计适配方案
- **核心引擎适配**: 将 `awesomeclaude` 的工具定义逻辑集成到 `core/internal/agent/tools.go`。
- **技能管理适配**: 参考 `microsoft agent-skills` 设计 `core/internal/agent/skills/` 目录，实现技能的加载与分发。
- **协议适配**: 引入 MCP 协议思想，在 `core/internal/agent/mcp/` 中实现插件化工具注册机制。

## 4. 实施进度 (2026-08-20)
- [x] 调研开源社区
- [x] 判断有用性
- [x] 设计适配方案
- [ ] 真正创建 (进行中...)

---
**生成时间**: 2026-08-20
**状态**: 已完成调研与设计阶段
