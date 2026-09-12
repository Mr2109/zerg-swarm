---
name: code-optimization
description: 虫族代码优化扫描——12 项优化任务清单（死代码/重复/矛盾/依赖/错误处理/日志/并发/一致性/性能/安全/可维护/补测）+ 分阶段执行 + 工作流。Use when: 优化虫族代码、扫描死代码、找重复功能、代码体检、CA 自举优化。
---

# 虫族代码优化扫描

> 让 CA（虫族 Agent）优化虫族项目——自举（终极测试 + 真实价值）
> 来源：docs/常青/优化清单.md（完整细节——需更多信息 read 该文件）

## 12 项优化任务清单

1. 死代码（未被调用函数/字段）→ 报告
2. 重复/矛盾功能 → 报告
3. 最少代码完成相同功能 → 报告
4. 冗余依赖/未用 import → 清理报告
5. 错误处理缺失（吞 error/panic）→ 补全报告
6. 日志/监控缺失 → 补齐报告
7. 并发安全（race/共享变量）→ 修复报告
8. API 一致性（命名/风格）→ 统一报告
9. 性能热点（O(n²)/重复计算）→ 优化报告
10. 安全（路径拼接/shell 注入）→ 加固报告
11. 可维护性（长函数/魔法数字/注释）→ 重构报告
12. 测试覆盖盲区 → 补测报告

## 工作流（每任务循环）

```
1. 拆小任务（单包/单主题——稳）
2. 用工具核查（staticcheck U1000 死代码/dupl 重复/errcheck 错误——工具扫描是候选）
3. 一一核查（工具输出须验证——不信扫描直接结论——排除误报/预留）
4. 记录（问题文档：现象/根因/修复/教训）
5. commit（中文 + 前缀）
```

## 核查要点

```
- staticcheck U1000: 高可信但需核查（跨文件调用/局部遮蔽会误报）
- dupl: 对 switch 模式误报多（阈值 100）
- 工具扫描是候选——必须一一核查（CA 的价值）
- 查代码用 codegraph MCP（mcp_codegraph_codegraph_callers——零调用者=死代码候选）
```

## 查死代码具体流程（v2.5.1——配合 codegraph MCP）

```
1. 先 tool_search 搜索代码查询工具（query=查代码调用关系）——发现 mcp_codegraph_*
2. 列函数清单（codegraph_search 不支持通配符——改用以下方式）:
   - 方式A: mcp_codegraph_codegraph_node(file=文件名, symbolsOnly=true)——单文件符号图
   - 方式B: grep 工具搜 "func " 前缀（pattern="^func "）——列函数定义
   - 方式C: mcp_codegraph_codegraph_explore(query=包名/文件名)——一次拿相关符号
3. 对每个函数调 mcp_codegraph_codegraph_callers（symbol=函数名）
   ——返回空调用者 = 死代码候选
4. 核实（防止误报）: 跨包调用/反射/接口实现——codegraph 可能漏
5. write 报告（文件/函数/行号/依据——调用者数为 0）
```

## 关键工具用法（codegraph MCP——避免试错）

```
- codegraph_search: 按名搜符号（query=具体名——不支持 * 通配符）
- codegraph_node: 读文件(symbolsOnly=true=符号图) 或 单符号详情(含调用轨迹)
- codegraph_callers: 查某函数的调用者（symbol=函数名——零调用者=死代码）
- codegraph_explore: 一次查多个符号/区域（自然语言 query）
```
