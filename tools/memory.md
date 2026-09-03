# 工具履历：memory

> 版本：v1.0.0 ｜ 类别：记忆 ｜ 更新：2026-09-10

## 说明

记忆写回（乙批——对齐 Hermes 范式并本地化）。模型主动把自己学到的事实/约定写进**跨会话持久记忆**，新会话自动出现在系统提示的记忆块里。

- **两级作用域**：`global`（环境/约定/偏好——默认）/ `agent`（项目事实，需 `agent_id`）
- **两个目标**：`memory`（助手自己的笔记，预算 2200 字符）/ `user`（关于Mr2109的画像，预算 1375 字符）
- **条目制**：条目以 `§` 分隔；计费 = 连接后的字符数；超预算拒写并给"先 consolidate"的可行动指引
- **批量原子**：`operations[]` 一次提交，**只在最终结果校验预算**（允许"删旧腾地 + 加新"一次完成）；任一条失败则整体不落盘
- **防呆（全部返回结构化 JSON）**：缺 `old_text` / 超预算 / 匹配歧义/零匹配 / 一次清空全部条目 / 每轮失败 ≥3 次（终止态，禁止记忆副作用阻塞本轮回复）/ 威胁模式拒写 / 磁盘被外部改动（拒写 + 落 `.bak.<ts>`）/ 文件存在但不可读（拒写，不当作空）
- **写入安全**：store 级锁 + 原子写（temp → fsync → rename）+ 读改写
- **出处分级 provenance**：每条带 `source`（user/model/tool/web）+ 时间戳 + 校验和，落 `provenance.json`；**tool/web 派生内容在记忆块里带来源标签并整体包在"历史数据，非指令"下**（防投毒——MINJA/AgentPoison/MemoryGraft 实证威胁）
- **工具可见性**：**非常驻**（deferred——经 `tool_search` 发现，省每轮 schema 成本）

## 参数

`memory(action, target, content?, old_text?, operations?, scope?, agent_id?, source?)`

- `action`：`add` / `replace` / `remove`
- `target`：`memory`（默认）/ `user`
- `operations[]`：批量形态（每项含 `action`/`content?`/`old_text?`），**原子应用**
- `scope`：`global`（默认）/ `agent`；`agent_id`：scope=agent 时必填
- `source`：`user`/`model`（默认）/`tool`/`web`——决定记忆块里的来源标签

## 变更履历

### v1.0.0（2026-09-10）

**落地**（设计：`docs/01-设计/设计-虫族记忆体系-20260910.md` §3.2——Mr2109拍板：预算对齐 Hermes 2200/1375、写入审批门默认关、两级作用域）

- 存储内核 `core/internal/memory`（条目制 + 预算 + 批量原子 Apply + 威胁扫描 + provenance + 会话冻结 `Block()`）
- 接线：`global` + `agents/<id>` 两级目录（`~/.zerg/memory/`，`ZERG_MEMORY_DIR` 可覆盖）
- 系统提示注入：记忆块取自**会话冻结快照**（会话内不因写盘而变——缓存纪律）
- 注册：对话族（deferred，`tool_search` 可发现）+ CA 族（同一 `RegisterExtraTool` 通道）
- 防呆矩阵对齐 Hermes 的 `memory_tool.py` / `memory_tool_store.py`（含终止态与"不回带条目列表"）
