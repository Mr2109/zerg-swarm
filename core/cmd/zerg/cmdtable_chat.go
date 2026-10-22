package main

// 命令表 · chat 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_chat() []*command {
	return []*command{
		// ---- `zerg chat` 族（**人类正门** · `P-1` 乙案 · 设计稿 v0.3 §5）----
		// 首版 **7 条**子命令（`F-1` 拍定 = `new`/`ls`/`send`/`show`/`abort`/`steer`/`rm`）。
		// 形态 = **薄壳**（打请求 + 渲染）：语义全在服务端（`core/internal/api/chat_handlers.go` ·
		// `core/internal/chat/**`），壳里**一个字都不重写**（§0 第一原则 · §9.1 第 3 条）；
		// 实现件 = `core/cmd/zerg/family_chat.go`（本族的 HTTP 出口 / 事件读法 / 错误分档全在它里）。
		// ★ ✗ **不登记**（`O-5`/`O-6` 已给理由）：`send-tool` / `regenerate` / `message_edit` / `search`
		//   —— 真敲它们是「未知命令」（退码 2），✗ 不是「有登记无实现」。
		// ★ 裸 `zerg chat`（交互 REPL 入口 · §5.5 ①）—— **2026-10-17 Mr2109 拍定：登记裸条 · 按 TTY 分流**
		//   （本条**订正**同批早先那句「裸条归 T9 ⇒ 不登记 ⇒ 否则 +8 ≠ 255」）：
		//   **有 TTY ⇒ 进 REPL**（骨架归 T9）· **无 TTY ⇒ 退 2 ＋ 提示走 stderr** · **`--help` ⇒ rc=0**；
		//   先例 = 现读全树唯一的 **`help`**（裸条 ＋ 同族动作**并存**）。⇒ **命令树 path 数 248 → 256**：
		//   **+8** 条 = 7 条子命令 ＋ 裸条 `chat`。★ 早先写的 **255**（= 只算 7 条）是 T9 勘定给的旧值、
		//   **已被本拍定作废** —— 随动面（门⑪ `commands` / 门78 `count` / cli-matrix / 命令页）一律按 **256** 刷。
		{
			path:    []string{"chat"},
			kind:    "Chat",
			summary: "对话正门：**裸跑按 TTY 分流**（有 TTY ⇒ 进交互 REPL · 无 TTY ⇒ 退 2 ＋ 提示）· 7 条子命令见下行（§5.5 ①）",
			usage:   "zerg chat [--session <会话id> | --new] [<子命令>]",
			arity:   "any",
			args:    []string{"子命令（裸跑 = 交互 REPL · 无 TTY ⇒ 退 2）"},
			run:     cmdChatRoot,
		},
		{
			path:     []string{"chat", "new"},
			kind:     "ChatNew",
			summary:  "新建一个对话会话（人类正门的起手式 · `POST /api/chat/sessions` · §5.1 第 1 行）",
			usage:    "zerg chat new [--model 名] [--title 题] [--source 名] [--json <字段>]",
			arity:    "none",
			fields:   chatNewFields,
			endpoint: "POST /api/chat/sessions",
			run:      cmdChatNew,
		},
		{
			path:     []string{"chat", "ls"},
			kind:     "ChatLs",
			summary:  "列会话（`GET /api/chat/sessions`；归档默认隐藏 · `--archived` 含 · §5.1 第 2 行）",
			usage:    "zerg chat ls [--archived] [--source 名] [--limit N] [--json <字段>]",
			arity:    "none",
			fields:   chatLsFields,
			endpoint: "GET /api/chat/sessions",
			run:      cmdChatLs,
		},
		{
			path:     []string{"chat", "send"},
			kind:     "ChatSend",
			summary:  "对话主写面（SSE 流式 · `POST /api/chat/sessions/{id}/send`；`--json`/非 TTY/`--no-stream` ⇒ 非流式 · §7.5）",
			usage:    "zerg chat send [--prompt-file <路径> | <提示> | -] [--session <id>] [--new] [--last] [--last-in-cwd] [--model 名] [--images <路径>…] [--no-stream] [--markdown] [--wait] [--json <字段>] [--timeout <时长>]",
			arity:    "any",
			args:     []string{"提示（`-` = 从 stdin 读 · 与 `--prompt-file` 三选一 · 零 shell 解释 · §5.5 ③）"},
			fields:   chatSendFields,
			endpoint: "POST /api/chat/sessions/{id}/send",
			run:      cmdChatSend,
		},
		{
			path:     []string{"chat", "show"},
			kind:     "ChatShow",
			summary:  "读回会话与消息（`GET /{id}` · `--window <mid>` 走 `GET /{id}/window` 召回窗口 · §5.1 第 4 行）",
			usage:    "zerg chat show [<id>] [--session <id>] [--last] [--last-in-cwd] [--limit N] [--window <mid>] [--json <字段>]",
			arity:    "any",
			args:     []string{"会话 id（完整 id · 可省 —— 与 `--session`/`--last` 三选一）"},
			fields:   chatShowFields,
			endpoint: "GET /api/chat/sessions/{id}",
			run:      cmdChatShow,
		},
		{
			path:     []string{"chat", "abort"},
			kind:     "ChatAbort",
			summary:  "中断运行中的轮次（`POST /{id}/abort`；已流出内容由服务端落库附「已中断」· §7.4）",
			usage:    "zerg chat abort <id> [--json <字段>]",
			arity:    "any",
			args:     []string{"会话 id（完整 id）"},
			fields:   chatAbortFields,
			endpoint: "POST /api/chat/sessions/{id}/abort",
			run:      cmdChatAbort,
		},
		{
			path:     []string{"chat", "steer"},
			kind:     "ChatSteer",
			summary:  "生成中插话（`POST /{id}/steer` · 挂**下一次工具结果**；`turn_running=false` ⇒ 服务端不入库 · `O-4`）",
			usage:    "zerg chat steer <id> <文字> [--json <字段>]",
			arity:    "any",
			args:     []string{"会话 id（完整 id）", "插话文字"},
			fields:   chatSteerFields,
			endpoint: "POST /api/chat/sessions/{id}/steer",
			run:      cmdChatSteer,
		},
		{
			path:     []string{"chat", "rm"},
			kind:     "ChatRm",
			summary:  "删会话（**写面 fail-closed**：`--dry-run` 出计划件 · 缺 `--yes` ⇒ 2 · `DELETE /{id}` · §5.1 第 7 行）",
			usage:    "zerg chat rm <id> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"会话 id（完整 id）"},
			fields:   chatRmFields,
			danger:   &dangerSpec{dangerD2, "会话 id", "**硬删**一个会话与其消息（服务端 `DeleteSession` ⇒ `deleted:true`）—— **不可逆**：删掉的消息取不回来", "设计稿 §5.1 第 7 行 · §5.2（写面 fail-closed）· 同族先例 `gateway breakers`（D2）", false, dangerScopeRemote},
			opened:   true,
			endpoint: "DELETE /api/chat/sessions/{id}",
			run:      cmdChatRm,
		},
		// ---- `zerg chat` 机器正门 ＋ 会话别名（2026-10-17 本枚 · 命令树 264 → 268 · **+4** 条 path）----
		// ★ 为什么加这四条（用户原话「怎么更方便长期对话和多任务对话」）：
		//   ① **长期对话的真痛点 = 记不住 `chat_xxx` 那串 id** ⇒ 会话别名（`alias set|ls|rm`），
		//      ✗ 不是再加一层 `--session`；
		//   ② **多任务的真痛点 = 一个任务一条线、互不串** ⇒ 同一套别名天然解决（`ask --as <名>`）；
		//   ③ **机器调用的真痛点 = 一次进一次出、正文即结果** ⇒ `chat ask`（✗ 不靠流式 ✗ 不靠二次 `show`）。
		// ★ 随动面已同批刷（门⑪ 计数 / 门69 冻结常量 / cli-matrix / newcmd-paths / error-detail / 命令页四件）。
		{
			path:     []string{"chat", "ask"},
			kind:     "ChatAsk",
			summary:  "机器正门：**一次进一次出**（`--as <名>` 有别名就续上、没有就新建**并登记**；默认只把成稿正文打上 stdout · ✗ 不靠流式 · ✗ 不靠二次 `show`）",
			usage:    "zerg chat ask --as <名> [--prompt-file <路径> | <题> | -] [--stream] [--json <字段>] [--timeout <时长>]",
			arity:    "any",
			args:     []string{"提示（与 `--prompt-file` 三选一 · `-` = 从 stdin 读）"},
			fields:   chatAskFields,
			endpoint: "POST /api/chat/sessions（无别名时新建）· POST /api/chat/sessions/{id}/send",
			run:      cmdChatAsk,
		},
		{
			path:    []string{"chat", "alias", "set"},
			kind:    "ChatAliasSet",
			summary: "登记会话别名（名字 → 完整会话 id · 落 `<状态目录>/chat-aliases.json` · 写走**独占文件锁**）",
			usage:   "zerg chat alias set <名> --session <会话id> [--json <字段>]",
			arity:   "any",
			args:    []string{"别名（非空 · ≤64 字符 · ✗ 不含空白/控制字符）"},
			fields:  chatAliasSetFields,
			run:     cmdChatAliasSet,
		},
		{
			path:    []string{"chat", "alias", "ls"},
			kind:    "ChatAliasLs",
			summary: "列会话别名（默认表格式 · `--json` 挑字段 · 读走共享锁快照 · 按名字排序）",
			usage:   "zerg chat alias ls [--json <字段>]",
			arity:   "none",
			fields:  chatAliasLsFields,
			run:     cmdChatAliasLs,
		},
		{
			path:    []string{"chat", "alias", "rm"},
			kind:    "ChatAliasRm",
			summary: "删一条会话别名（名字不在表里 ⇒ **2** · 会话本身不动 · 写走独占文件锁）",
			usage:   "zerg chat alias rm <名> [--json <字段>]",
			arity:   "any",
			args:    []string{"别名"},
			fields:  chatAliasRmFields,
			run:     cmdChatAliasRm,
		},
	}
}
