package main

// 命令表 · gate 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_gate() []*command {
	return []*command{
		// ---- 批 A · S3 门禁直通六条（**只转发、不翻译** · §6.3 S3 判据③）----
		// ★ 缺口 `GAP-20260927-226` want①：`name` 走**可选字段层**（`optFields`，与 `code find` 的
		// `count` 同一体例）—— 步名的第二个键（同族 `gate find`/`gate explain` 的 `name` 逐字同名）；
		// **不进**上面这份帮助/导出面的冻结表 ⇒ 帮助面与命令页一行不动。
		{
			path:    []string{"gate", "ls"},
			summary: "门禁步骤表（逐行等于脚本 --list；薄壳不另写一份）",
			usage:   "zerg gate ls [--json <字段>] [--fingerprint]",
			fields:  []string{"total", "scope", "mode", "step"},
			// ★ 缺口 GAP-20260926-102（批⑤ · 2026-10-13）：`reds`（最近一趟非 PASS 的步名清单）同走**可选字段层**。
			// ★ 缺口 `GAP-20260928-68`（本片 · 2026-10-14）：`id`（每步的**稳定短 id** = 步名派生的内容指纹 ·
			//  ✗ 不含行号/序号 ⇒ 步骤表插/删一行后其余步的 id 逐字不变）同走**可选字段层** ⇒
			//  `zerg gate ls --json id` 点得出来（件内那一侧是 `gate.go` 的 `gateLsOptFields`，两表逐字一致
			//  由 `TestGateLsOptFieldsParity` 钉住 · ✗ 不进冻结 `fields:` ⇒ 帮助面与命令页一行不动）。
			// ★ 缺口 `GAP-20260929-22`（本片 · 2026-10-14）：`cmd`（每步**可直接粘贴的整条命令**，含正确
			//  引号；单引号逐字 / 内嵌单引号按 `'\''` 闭合 · 见 `gate.go` 的 `gateLsPasteCmd`）同走**可选字段层**
			//  ⇒ `zerg gate ls --json cmd` 点得出来；✗ 不进冻结 `fields:`（帮助面与命令页一行不动 · 同 `name`/`id`）。
			optFields:   []string{"name", "gates_total", "fast", "reds", "id", "cmd"},
			endpoint:    "",
			passthrough: true,
			run:         cmdGate,
		},
		{
			path:        []string{"gate", "run"},
			summary:     "跑门禁（旗标逐字透传；退码原样转出，不翻译；`--step` 只跑一道门）",
			usage:       "zerg gate run [--scope <s> | --fast] [--outdir <目录>] [--dry-run] … | zerg gate run --step <步名> [--dry-run] [--self-test] [--only-step] [--json <字段>] [--verify-live] [--show-log] [-- <只读旗标>…（如 --root <路径>）]",
			arity:       "any",
			args:        []string{"脚本旗标（原样透传）"},
			fields:      gateRunStepFields,
			endpoint:    "",
			passthrough: true,
			run:         cmdGate,
		},
		{
			path:    []string{"gate", "show"},
			kind:    "GateShow",
			summary: "看某一步要跑的命令串（脚本 --emit-cmd）；`--json` 另给四格（scope/mode/判据/日志路径）",
			usage:   "zerg gate show <步名> [--json <字段>]",
			arity:   "any",
			args:    []string{"步名（与 --list 里逐字相同；`--json` 面是**精确匹配**，子串不给结论）"},
			// 四格 = `Q-061`/`B-3` 的可核条件逐字（`scope` / `mode` / 判据 / 日志路径）。
			// 默认面（不给 `--json`）仍是 `--emit-cmd` 直取口 —— 见 `family_gate_show.go` 的文件头。
			fields:      gateShowFields,
			endpoint:    "",
			passthrough: true,
			run:         cmdGate,
		},
		{
			path:    []string{"gate", "find"},
			kind:    "GateFind",
			summary: "按**门件名/命令串片段**找步骤名（门件名 ⇒ 步名 的桥）· 只读",
			usage:   "zerg gate find <片段> [--json <字段>] | zerg gate find --uncertain [--json <字段>]",
			arity:   "any",
			args:    []string{"片段（门件名或命令串里的一段，例：check-gate-coverage）"},
			// 九格 = 与 `gate explain` 同义字段逐字同名（判据/日志/出处在本仓只有一处口径）+ 第八格
			// `next`（照抄即走的下一步）+ 第九格 `wiring`（三处接线面 precommit / all.sh / real-gates.sh
			// 的现读结论 —— 「零命中」与「不存在」的分别就靠它）。★ 本命令**不走脚本**：只读脚本里的
			// `add_step` 声明 ⇒ 与 `gate results` 同规（命令面分支），不改任何脚本退码。
			fields: gateFindFields,
			// ★ 本单（缺口 `GAP-20260929-28` 剩面）：可选字段层两批 —— ① `command_kind` /
			//   `command_resolved`（第五格形态 + 真身的人面两格 · 与 `gate explain` 同款）；
			//   ② `line` / `kind` / `why` / `raw`（`gate find --uncertain` 那张正门表的四格）。
			//   都不进冻结 `fields:` ⇒ 不重生命令页 / 帮助面；点得出即用。
			optFields: []string{"command_kind", "command_resolved", "line", "kind", "why", "raw"},
			endpoint:  "",
			run:       cmdGate,
		},
		{
			path:    []string{"gate", "results"},
			kind:    "GateResults",
			summary: "读**现成**一趟门禁产物的四数（通过/失败/不给结论/只报告 + 步数与总退码）· 只读",
			usage:   "zerg gate results [--last] [--dir <目录>] [--json <字段>]",
			arity:   "none",
			args:    []string{"（不收位置参数：那一趟由 `--last`（缺省即最近一趟）或 `--dir <目录>` 指）"},
			// 逐条五格 = 步名/状态/退码/耗时/日志路径（缺口 `Q-111`/`B-8` 的机器面）。
			fields: gateResultsFields,
			// ★ 缺口 GAP-20260927-227（批⑤ · 2026-10-13）：`source` 走**可选字段层**（`optFields`）——
			//   「本页读的是**哪一趟**产物」的机读直读口（= 那一趟目录，与 `meta.run_dir`/`meta.results_tsv`
			//   同源），登在出口白名单、`--json` 点得出来，但不进 `fields` 冻结表 ⇒ 不重生命令页。
			optFields: []string{"source"},
			endpoint:  "",
			run:       cmdGate,
		},
		// ---- Q-125（本枚 · 只增）：棘轮基线的**只读呈现面** `zerg gate baseline show` ----
		// 病根：命令树里没有 `baseline` 这个节点 ⇒ `zerg gate baseline show` 落到「族名/中间节点」
		//   分支（rc=2），而读棘轮现状只有 `gate check baseline-ratchet --view`（名词 = 判据件名）。
		// 本动作挂既有 `gate` 族（中间段 `baseline` · 三段路径 `resolve()` 已支持），**不新开族**。
		// ★ 读路径 = `baseline_ratchet.go` 的 brConditions/brCollect/brLoadLedger/brLastEntryFor
		//   （与 `gate check baseline-ratchet` **同一引擎** ⇒ 不许另造第二套基线读取）。
		// ★ 不走脚本（`gateActionNeedsScript` 已把 `baseline` 归入「不 exec 自举件」那一档）⇒
		//   公开树（排除 `scripts/gates/**`）里照常可跑。
		// ★ `fields:` 必须写成 `[]string{…}` **字面量**（契约脚本 `FIELDS_RE` 只认字面量）。
		//   八格里后两格（`ledger_value` / `delta`）是本动作的**派生列**，前六格与 `--view --json`
		//   的 rows 同名同义 ⇒ 两个机器面可逐字段对拍。
		{
			path:     []string{"gate", "baseline", "show"},
			kind:     "GateBaselineShow",
			summary:  "只读：棘轮基线**现读值 × 台账基线值 × 差**（逐条件 · 不带名 = 全部）· 读路径 = 同族 `gate check baseline-ratchet` 的同一套引擎（✗ 不另造第二套读取）",
			usage:    "zerg gate baseline show [<条件名>] [--json <字段>] [--root <目录>]",
			arity:    "any",
			args:     []string{"条件名（可省；省 = 全部条件。点错名 ⇒ 2 + 可用清单 = 条件表现读）"},
			fields:   []string{"name", "gate", "file", "value", "ledger_value", "delta", "verdict", "verdict_text"},
			endpoint: "",
			run:      cmdGate,
		},
		{
			path:        []string{"gate", "self-test"},
			summary:     "门禁自检（合成步骤 · 不碰真目标）",
			usage:       "zerg gate self-test",
			endpoint:    "",
			passthrough: true,
			run:         cmdGate,
		},
		{
			path:     []string{"gate", "self-test-summary"},
			kind:     "GateSelfTestSummary",
			summary:  "**判据自检的统一机器面**（缺口 `GAP-20260926-183` · 路 B）：一个入口跑齐进程内判据的自检、**集中解析末行** ⇒ 三态（通过/失败/判不了）＋ 两读数（格数/失败数）",
			usage:    "zerg gate self-test-summary [--json <字段>] [--self-test]",
			arity:    "none",
			args:     []string{"（不收位置参数：名录 = 进程内判据臂 `gateCheckGoEntry` · 逐件跑 `--self-test` 并解析末行；解析不出 ⇒ 判不了）"},
			fields:   []string{"count", "pass", "fail", "unknown", "grids", "fails", "entries"},
			endpoint: "",
			run:      cmdGateSelfTestSummary,
		},
		// ---- v2.5.14 · M1 首项：判据转发壳 `zerg gate check <判据名>`（设计-判据 CLI 化-v1.7 §9.1）----
		// 形状：**动词做命令、名词做参数** ⇒ 判据名是**参数**，不是 189 条平级命令（§5.3）。
		// 走 `cmdGate` 的新动作分支（`gate.go` 的 `case "check"` → `family_gate_check.go`）：
		// 判据名 ⇒ 件 的**保守**解析（名录真源 = 现读 `scripts/gates/` 目录列举）＋ 逐字透传退码与两路输出；
		// 不在册 ⇒ 拒并给可用名；件自身崩 ⇒ `8` ＋ `detail=judge_broken` ＋ `raw_rc`（**零新占号**）。
		// 透传型（同 `gate run`/`show`）：子件的旗标与位置参数逐字交给子件 ⇒ 命令面不另判旗标面。
		{
			path:     []string{"gate", "check"},
			kind:     "GateCheck",
			summary:  "跑**一件判据件**（判据名 ⇒ scripts/gates/*.py|*.sh 的保守解析 · 逐字透传退码与输出；不在册 ⇒ 拒并给可用名；件自身崩 ⇒ 8 不给结论；**写工具**件强制危险档 · 缺 --yes ⇒ 2）",
			usage:    "zerg gate check <判据名> [--list-selfcases] [--dry-run | --yes] [子件参数…]",
			arity:    "any",
			args:     []string{"判据名（＝ scripts/gates/ 下件名去扩展名 · 逐字相同；✗ 不模糊匹配）"},
			endpoint: "",
			// ★ `--json` 字段表（M1 尾 · 全局字段表闸）：与 `family_gate_check.go` 的
			//   `gateCheckFullTable` 机器面**同源** —— 现读那一处 `enc.Encode(map[string]any{…})`
			//   的四个键（`count` / `resolved` / `dry_run` / `entries`），一字不多一字不少。
			//   没有它，`zerg gate check --json` 会被命令面当「说明面」拒成 2
			//   （逐字：``zerg gate check` 没有 --json 字段表（它是说明面）`）。
			// ★ 必须写成 `[]string{…}` **字面量**（契约脚本的 `FIELDS_RE` 只认字面量；
			//   抽成变量 = 该条被读成「没有字段表」）—— 与 `net probe` / `publish tree has` 同一条纪律。
			fields: []string{"count", "resolved", "dry_run", "entries"},
			// ★ 本笔（缺口：`zerg gate check <判据名> -h` 被命令树截成族帮助）：`-h` 短形**透传给子件**
			//   ⇒ 子件自己的 usage 在 CLI 上可达；`--help` 长形仍是本条的族帮助（判据在 `passthroughChildHelp`）。
			//   只本条开：`gate run` 这类「跑真门」的透传命令**不许**开，否则 `-h` 会变成真干一次。
			childHelp: true,
			// ★ 写工具危险档（M1 批B）：件头 `#zerg-class: write D2|D3` ⇒ 壳侧 exec 前强制确认档。
			// 取**最坏档 D3** 登记（写工具里最坏是不可逆；命令树一格只能写一档 ⇒ 取最坏）；
			// `ChecksConfirm: true` = 实现真读 `--confirm`（family_gate_check.go 的写工具闸 · 真读才印）。
			danger:      &dangerSpec{dangerD3, "判据名", "跑一件**写工具**（会真改盘：换件/落临时副本/落锁）；只读件不受影响", "本单 · 同族先例 §九 M3 C1 · family_gate_matrix.go:12-15", true, dangerScopeUnknown},
			opened:      true,
			passthrough: true,
			run:         cmdGate,
		},
		// ---- T09/T11 正门两条（本枚 · **只增**）----
		// ★ 两条都**不走** `cmdGate`：`cmdGate` 的动作分派长在 `gate.go`（本枚不许改的第三件之外），
		//   而 `resolve()` 的 3 段前缀试探（见其函数头）已能把 `gate selfreport diff` 精确命中 ⇒
		//   直接把 `run` 指向本枚新增的两个处理器即可（零改既有分派）。
		{
			path:     []string{"gate", "selfreport", "diff"},
			kind:     "GateSelfreportDiff",
			summary:  "自报件 × 独立对拍件**六格逐格**对拍（query_ts/computed_at/source/digest/verifier/verdict）· 只读 · 判不了不给结论",
			usage:    "zerg gate selfreport diff --source <件> --against <件> [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数：自报方与对拍方分别由 `--source` / `--against` 指 —— 两枚同名即用法错，禁自报面自打分）"},
			fields:   gateSelfreportFields,
			endpoint: "",
			run:      cmdGateSelfreportDiff,
		},
		{
			path:     []string{"gate", "four-states"},
			kind:     "GateFourStates",
			summary:  "四态亲测：现役件／改件 × 不带／带 `--yes` 各跑一次，四格逐格比对 · 只读面必须逐字节相同、写面不许恒同",
			usage:    "zerg gate four-states --current <命令串> --patched <命令串> [--yes-flag <旗标>] [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数：两条命令串由 `--current` / `--patched` 指；命令串按**空白切词**，不做 shell 展开）"},
			fields:   gateFourStatesFields,
			endpoint: "",
			run:      cmdGateFourStates,
		},
		{
			path:     []string{"gate", "reap"},
			danger:   &dangerSpec{dangerD3, "（陈旧/卡死门跑根）", "逐枚 pid 发 SIGTERM→SIGKILL 终止陈旧/卡死门跑根**及其门谱系** ＋ 追加审计 reap-audit.jsonl（真在飞的门跑绝不碰）", "GAP-20261005-35 · 同族先例 agent reap（D3）· gate 族写工具件走 gate check D3", false, dangerScopeSystem},
			opened:   true,
			kind:     "GateReap",
			summary:  "回收**陈旧/卡死门跑根**（只收祖链已断或死趟残儿的根**及其门谱系**；真在飞的门跑绝不碰）· 每个收回动作入审计",
			usage:    "zerg gate reap --stale [--dry-run | --yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数：`--dry-run` 只列清单、`--yes` 才真收）"},
			fields:   gateReapFields,
			endpoint: "",
			run:      cmdGateReap,
		},
		{
			path:     []string{"gate", "ps"},
			kind:     "GatePS",
			summary:  "只读：门进程 + 门锁的现读快照（进程段复用 gateReapScan · 锁段读 <锁目录>/gate-run.lock）· 立即返回",
			usage:    "zerg gate ps [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数）"},
			fields:   gatePSFields,
			endpoint: "",
			run:      cmdGatePS,
		},
		{
			path:     []string{"gate", "progress"},
			kind:     "GateProgress",
			summary:  "**只读快照**：看正在跑那一趟门禁的活进度（八格：趟号步数/正跑哪步/已耗时/静默秒/四数/是否在跑/趟归属/advancing）· 纯只读 · 立即返回 · 绝不阻塞（设计-门进度可查-v0.3 §三/§十） · 判活**只判第 3 层**（当前步日志 mtime 推进 ＋ 谱系 pcpu>0）：快照不做长采样，**完整三层取或**只在有时间窗口的 `gate wait` 落（缺口 `GAP-20261002-16`）",
			usage:    "zerg gate progress [--dir <目录> | --last] [--list] [--limit <n>] [--silent-threshold <秒>] [--json <字段>]（`--last` **按最新写入挑**：`results.tsv` 的 mtime 最新 · 与 `zerg gate results --last` 同口径；点名那一趟用 `--dir`）",
			arity:    "none",
			args:     []string{"（不收位置参数：那一趟由 `--dir` / `--last` 指 —— 非 TTY 下多趟活 ⇒ 列清单 + 退 2，绝不弹提示）"},
			fields:   []string{"run_dir", "finished", "alive", "root_pid", "origin_kind", "origin", "steps_done", "steps_total", "steps_source_sha", "current_step", "current_step_name", "current_secs", "elapsed_secs", "last_activity_ts", "silent_secs", "silent_threshold", "pass", "fail", "blocked", "report", "exit_code", "slowest_steps", "advancing", "deepest_pid", "deepest_cmd", "deepest_secs", "depth_limit_reached"},
			endpoint: "",
			run:      cmdGateProgress,
		},
		{
			path:     []string{"gate", "wait"},
			kind:     "GateWait",
			summary:  "**有界阻塞**等门跑收工（与 progress 共享同一引擎）；`--timeout` **默认有限**（不学 kubectl 的 0=无穷）；`--exit-status` 把门禁的红透成退码（设计-门进度可查-v0.3 §十.6） · 判活**自第二帧起判完整三层取或**（活动计数 / 最深层 PID 换过 / 当前步日志 mtime 推进 · §十.1）：与 `gate progress` 的快照口径（只判第 3 层 ＋ 谱系 pcpu>0）不同（缺口 `GAP-20261002-16`）",
			usage:    "zerg gate wait [--dir <目录> | --last] [--interval <秒>] [--timeout <秒>] [--exit-status] [--json <字段>]（`--last` **按最新写入挑**：`results.tsv` 的 mtime 最新 · 与 `zerg gate results --last` 同口径；点名那一趟用 `--dir`）",
			arity:    "none",
			args:     []string{"（不收位置参数：那一趟由 `--dir` / `--last` 指）"},
			fields:   []string{"run_dir", "finished", "alive", "root_pid", "origin_kind", "origin", "steps_done", "steps_total", "steps_source_sha", "current_step", "current_step_name", "current_secs", "elapsed_secs", "last_activity_ts", "silent_secs", "silent_threshold", "pass", "fail", "blocked", "report", "exit_code", "slowest_steps", "advancing", "deepest_pid", "deepest_cmd", "deepest_secs", "depth_limit_reached", "waited_secs", "timed_out", "interval", "timeout"},
			endpoint: "",
			run:      cmdGateWait,
		},
		{
			path:     []string{"gate", "seal", "verify"},
			kind:     "GateSealVerify",
			summary:  "核章档（第四档）章账**只读**核验：九格齐备 + 断言 executed 计数 0 + 失效规则（head/worktree_fp 与现读不符 ⇒ stale；步不在现读步集 ⇒ gap）· 三态逐字输出（未真跑（核章）/stale/gap）",
			usage:    "zerg gate seal verify [<章账>] [--json <字段>]",
			arity:    "any",
			args:     []string{"章账（append-only JSONL · 可省：缺省 scripts/gates/seal/ledger.jsonl）"},
			fields:   gateSealVerifyFields,
			endpoint: "",
			run:      cmdGateSealVerify,
		},
		{
			path:    []string{"gate", "explain"},
			kind:    "GateExplain",
			summary: "读懂某一步到底在判什么（scope/模式/判据/退码口径/日志路径/出处文件:行 —— **精确匹配**步名）",
			usage:   "zerg gate explain <步名> [--json <字段>]",
			arity:   "any",
			args:    []string{"步名（与 `zerg gate ls` 逐字相同）"},
			fields:  []string{"scope", "mode", "criterion", "verdict", "exit", "log", "source", "command", "script_say"},
			// ★ 本单（缺口 `GAP-20260929-28` 剩面 · 2026-10-14）：`command_kind`（第五格形态：
			//   字面命令 / 变量间接 / 命令替换）与 `command_resolved`（真身：变量定义行 + 可能值集 /
			//   命令替换点名）走**可选字段层**（`optFields`）—— 它们由 `family_gate_explain.go` 一处算、
			//   与 `gate find` 同款 ⇒ `--json command_kind` 点得出来，但**不进**冻结 `fields:`
			//   ⇒ 帮助面与命令页一行不动（与 `gate results` 的 `source` 同体例）。
			optFields: []string{"command_kind", "command_resolved"},
			endpoint:  "",
			run:       cmdGateExplain,
		},
		// ── `danger`（**D2**）：本表项**真写盘**（修前无 `danger` ⇒ `zerg help` 把它判成安全档 ✗）
		//   定档理由（逐条读 `family_gate_matrix.go:cmdGateMatrix` 得）：`--out <件>` 那一支
		//   `os.WriteFile` 写一份逐格 TSV —— 只**新建/覆盖产出件**；不删件、不改矩阵真源
		//   `testdata/cli-matrix.json`、不改任何仓内真值 ⇒ 可逆（删掉/换回落点件即回原状）
		//   ⇒ **D2**（`--yes` 即可），不是 D3。
		//   ★ 三态面**已接**（2026-09-27 · `family_gate_matrix.go` 的 `gateMatrixOutPlan` + 三道判，
		//   全在 `os.WriteFile` **之前**）⇒ `usage` 串的互斥形态 `[--dry-run | --yes]` = **全树 19 条真判「两枚同给 ⇒ 2」命令的准形**
		//   （**2026-09-27 反向对齐**：`0b832b3f` 曾按「同给 ⇒ `--dry-run` 优先」把另 18 处收成 `[--dry-run] [--yes]`；`48cfef8a` 后实现改判 2 ⇒ 那 18 处已回改到本形态）：
		//   `--dry-run` ⇒ 计划件走 stdout + rc=0 且**一个字节都不落**（不建件、不建目录）；
		//   缺 `--yes` ⇒ 计划件走 stderr + rc=2（fail-closed）；`--dry-run` 与 `--yes` 同给 ⇒ rc=2。
		//   先看计划件：`zerg gate matrix --out <件> --dry-run`（同族先例 `zerg gap export`）。
		{
			path:    []string{"gate", "matrix"},
			kind:    "GateMatrix",
			summary: "命令面自己的 must-fail 矩阵（逐格可读可导 —— 新增命令照着它补格）",
			usage:   "zerg gate matrix [--out <件>] [--dry-run | --yes] [--json <字段>]",
			fields:  []string{"command", "case", "want_rc", "why"},
			danger: &dangerSpec{dangerD2, "落点件（`--out`）",
				"往 `--out <件>` 写一份逐格 TSV（command/case/want_rc/want_stdout_bytes/argv/why）；矩阵真源 `testdata/cli-matrix.json` 与仓内真值一个字不动（可逆：删掉落点件即回原状）",
				"缺口-命令面-20260921 §十一 P0-5 · 同族写面先例 `zerg gap export`（`--out` 真写盘）· §九 M3 C1", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGateMatrix,
		},
		{
			path:     []string{"gate", "bench"},
			kind:     "GateBench",
			summary:  "量门禁耗时（逐趟 real/user/sys + 中位/最差 + 门禁身份 sha256）",
			usage:    "zerg gate bench [--fast | --scope <s>…] [--repeat n] [--json <字段>]",
			fields:   []string{"run", "real_ms", "user_ms", "sys_ms", "rc", "log"},
			endpoint: "",
			run:      cmdGateBench,
		},
	}
}
