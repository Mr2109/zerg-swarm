---
name: ca-troubleshooting
description: CA（虫族 Agent）执行时的常见坑与规避——路径语义/工作区残留/任务量控制/轮次参数/MCP连接/MCP调用参数。Use when: CA 执行任务遇到工具失败、路径错误、上下文爆炸、MCP 问题，或给 CA 派活前预防踩坑。
---

# CA 排障护栏（CA 执行坑——失败→护栏——Steve-Evolving 模式）

> 来源：docs/优化清单.md（2026-08-14 实测坑——CA 干活累积）
> 用途：CA 执行任务时自查 + 派活者预防——避免重复踩坑

## 护栏清单（禁止/必须）

```
1. 【路径语义】工具路径相对「当前工作区」（workdir）——不是项目根
   ——workdir 已是目标目录时用相对名（memory.go）或 .（当前目录）
   ——不要拼完整项目路径（core/internal/agent/xxx.go 会重复拼接找不到文件）
   ——不确定先 ls . 看当前目录
   坑: 75 次 tool_exec_failed 全"路径不存在"（CA 拼 core 重复）

2. 【工作区残留】CA 写文件可能产生残留（internal/api/internal/api/ 嵌套 + .zerg/）
   ——验收时检查工作区无残留（rm -rf 残留目录）
   ——.zerg/ .hermes/ data/ 不入 git（.gitignore）

3. 【任务量控制】派活明确"最少数量"（写 3-5 个测试就够——不是越多越好）
   ——CA 会超额执行（"至少3个"→写 22 个→轮次耗尽没验证）
   ——大输出任务预留验证轮次（默认 100——不显式传小值）

4. 【轮次参数】CLI 默认 max_turns=100——派大任务不传小值（20 会不够）
   ——简单任务才传小值（10-15）

5. 【MCP 连接】-mcp 参数 stdio 型用 | 分隔（name:cmd|arg|arg）
   ——路径含空格必须 | 分隔（Fields 会拆坏——transport closed）

6. 【MCP 调用参数】codegraph_search 必须传 query（非空）——不能空想列全部
   ——调 codegraph 前先 tool_search 发现（deferred 不预加载）
   ——查代码第一优先 codegraph（比 read/grep 高效百倍）
```

## 工具选择原则（什么时候用什么）

```
- 查代码/死代码/调用关系 → tool_search 搜 codegraph → mcp_codegraph_*
- 查经验/坑/历史 → tool_search 搜 kb → mcp_kb_kb_search（执行任务前先查——避免踩坑）
- 查网络/调研 → tool_search 搜 anysearch → mcp_anysearch_*
- 加载技能 → skill_load（任务匹配 SKILL.md description）
```

## 修复模式（失败后）

```
1. 先诊断（看错误原文——不猜）
2. 路径错 → 用 ls . 确认当前目录 → 相对名
3. 工具失败 → 检查参数（query 非空/schema 匹配）
4. MCP 连接失败 → 检查 | 分隔/路径空格
5. 上下文爆 → 分页（read offset/limit）或 tool_search 精确工具
```
