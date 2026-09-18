# 工具履历：session_search

> 版本：**v1.1.0** ｜ 类别：记忆/检索 ｜ 更新：2026-09-11（v1.1.0 加 `include_archived`——见表末履历）

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
- `include_archived`：可选（默认 false）。true 时把**已归档会话**也纳入检索/浏览——归档只是不占活跃视线，不等于"搜不到"


## 变更履历

### v1.0.0（2026-09-10）

**落地**（设计：`docs/01-设计/设计-虫族记忆体系.md` §3.3）

- 执行器 `core/internal/chat/chat_tool_session_search.go`（复用 `SearchMessages` + 新增按 id 取窗口查询 + 最近会话列表）
- 注册：对话族 deferred（`tool_search` 发现）+ CA 族（同一注册通道）
- 与记忆体系的分工：`memory` = **写**（跨会话事实），`session_search` = **读**（历史对话原文，按需取回）

### v1.1.0（2026-09-10）

**新增 `include_archived`（Mr2109拍板：归档会话也要能搜到）**

- store 层：`SearchMessagesArchived(query, limit, includeArchived)` + `searchLikeArchived`（FTS 与 LIKE 降级两条路径）+ `ListSessionsArchived(limit, includeArchived)`
- 兼容：`SearchMessages` / `searchLike` / `ListSessions` 保留原签名（默认排除归档）——UI 搜索端点等既有调用方行为不变
- 工具层：`ssBoolArg` 容错（bool / "true" / "1" / "yes" / "是"）；search 与 browse 两种模式都支持；结果与无命中提示都标注范围（"含已归档"/"已排除归档"），无命中时提示逃生门
- 验收：单测 `chat_include_archived_test.go`（默认不中 / 显式中 / 字符串布尔 / browse 两态 / store 层默认不变）；活体 A/B：UI 搜索端点默认 8 条命中**全部来自活跃会话**（归档探针会话不出现）；模型经 `tool_search → session_search(include_archived=true)` 取回归档会话原文（含"含已归档"标注 + 恢复指针），服务端 `tool_uses.json`/事件文件双证 `session_search` 真实执行
