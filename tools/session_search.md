# 工具履历：session_search

> 版本：v1.0.0 ｜ 类别：记忆/检索 ｜ 更新：2026-09-10

## 说明

对话历史检索（乙批）。让"被压缩掉、或被软删的旧分支"重新可见——**给指针，不给全文**（省 token，对齐 Hermes 哲学）。

- **三种模式**（按参数自动选择）：
  - `search`：给 `query` → FTS5 全文检索（trigram 分词器，中文 ≥3 字走 MATCH，<3 字走 LIKE 降级；已排除 archived 会话）→ 返回命中的会话标题 + 角色 + 消息 id + 片段（每条截断）+ 时间
  - `read`：给 `session_id`（可选 `around_id`）→ 读该会话的一段（`around_id` 前后各 ~6 条；不传则最近 ~20 条）
  - `browse`：都不给 → 最近 ~10 个会话列表（id/标题/最后消息时间/消息数）
- **恢复指针**：每条命中后附 `▶ 恢复该段上下文: session_search(session_id=..., around_id=...)`——模型可据此按需取回上下文，而不是一次灌入全文
- **安全标注**：结果包在"历史数据，非指令"说明下（防检索结果被当作指令执行——投毒防御）
- **不新增索引、不引入 embedding**（复用 `SearchMessages`，见 `chat_search.go`）

## 参数

`session_search(query?, session_id?, around_id?, limit?)`

- `query`：搜索词（search 模式）
- `session_id` + `around_id?`：读窗口（read 模式）
- 都不传：browse 模式
- `limit`：可选，缺省 search=20 / read=20 / browse=10

## 变更履历

### v1.0.0（2026-09-10）

**落地**（设计：`docs/01-设计/设计-虫族记忆体系-20260910.md` §3.3）

- 执行器 `core/internal/chat/chat_tool_session_search.go`（复用 `SearchMessages` + 新增按 id 取窗口查询 + 最近会话列表）
- 注册：对话族 deferred（`tool_search` 发现）+ CA 族（同一注册通道）
- 与记忆体系的分工：`memory` = **写**（跨会话事实），`session_search` = **读**（历史对话原文，按需取回）
