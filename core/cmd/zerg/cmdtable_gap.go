package main

// 命令表 · gap 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_gap() []*command {
	return []*command{
		// ── `gap` 族（缺口 · 设计稿 `设计-命令面-gap族-v1.0-20260923.md` §二/§五）────────────
		//   三条：`ls` 只读 · `add` 写（留证据）· `verify` 写（改状态）。**不新增顶层命令**；
		//   真源 = `<状态目录>/zerg-cli-gaps.jsonl`（不在任何仓里）；审计进 `dev edit` 同一件。
		//   ★ 矩阵 13 格与基线计数**同批**（`H-2`：分批 ⇒ 门⑬ `R4` 判幽灵调用）。
		{
			path:     []string{"gap", "ls"},
			kind:     "GapList",
			summary:  "缺口账（只读面：不写真源、不写审计）· 可按状态/优先级/影响面收窄 · 账内闭集外的值 ⇒ 判红并**点名到行**",
			usage:    "zerg gap ls [--state <仍缺|已派|已立项|已解|回归|不做>…] [--prio P0|P1|P2] [--impact <命令面|门禁面|文档面|公开面|换件面|归档面>] [--unit <件路径|目录前缀>] [--module <模块前缀>] [--grep <关键词>] [--qid <Q-NNN>] [--untriaged] [--solved-kind <实现|销案|前提不成立|部分|未分类>] [--json <字段>]",
			fields:   gapListFields,
			endpoint: "",
			run:      cmdGapLs,
			// ★ `GAP-20260928-163`（2026-10-10 · 本枚）：`truncated`/`hits`/`total` 走**可选字段层**
			//   （`optFields`）—— 跑级截断自报的直读口，让**全量档与收窄档同尺**（两档都点得出这
			//   三格，同源 = 与 `meta.total`/`meta.hits`/`inv.markTruncated()` 同一份
			//   `total`/`hits`/`cut`）；登在出口白名单里、`--json` 点得出来，但**不进** `fields`
			//   那份冻结帮助 / 文档导出面字段表（与 `code find` 的 `count` 同一体例）⇒ 不造
			//   「表列着、面不产出」的反例，也无需重生成命令页。
			//   ★ 本枚（缺口 `GAP-20260928-54`）：`triage_batch` 同样走**可选字段层**（`optFields`）—— 分诊批次
			//   标记的直读口，登在出口白名单、`--json` 点得出来，但**不进** `fields` 那份冻结帮助 /
			//   文档导出面字段表（与 `truncated`/`hits`/`total`、`code find` 的 `count` 同一体例）
			//   ⇒ 无需重生成命令页，也不造「表列着、面不产出」的反例。
			// ★ 缺口 GAP-20260927-322（批⑤ · 2026-10-13）：`frozen`（账真源**读面冻结锚**：sha16 · 行数 ·
			//   读时点）同走**可选字段层** —— 登在出口白名单、`--json` 点得出来，但不进 `fields` 冻结表。
			optFields: []string{"truncated", "hits", "total", "triage_batch", "file", "frozen",
				// ★ 缺口 `GAP-20260926-70`（2026-10-23 · 本枚）：`qid` / `qid_source` —— 缺口 ↔ 版账
				//   `Q-NNN` 的机器可读对应（反查轴 `--qid <Q-NNN>` 的逐条回吐口）。同走**可选字段层**
				//   ⇒ 点得出来、不进 `fields` 冻结表（无需重生成命令页）；老行没有这两格 ⇒ 空串不出键。
				"qid", "qid_source"},
		},
		// ★ `gap status` —— **排行面**（批1 第四片 · 只读面 · 2026-09-28）：按件（`--by unit` · 缺省）
		//   或按模块（`--by module`）分桶排行；**批4 第二片**再加 `--by tier`（按判据分级分桶 ——
		//   真判据 / 占位 / 无 三桶，列 = 条数 + 其中仍缺 + 其中已解，表尾固定两行：守恒式自校
		//   「三桶之和 == 总账」（不等 ⇒ 判红 1 并打印两数）+ 占位桶只报告（不退码））。
		//   实现件 = `family_gap.go:cmdGapStatus`。
		//   排序两键（都写进表头）：① 未闭降序 ② 近邻量化 P0/P1/P2 降序；对账等式自校
		//   （Σ(有件桶未闭) + 无件桶 == 账内仍缺）；带 `query_ts` / `ledger_sha16`（两个时刻的排行才可比）。
		{
			path:     []string{"gap", "status"},
			kind:     "GapStatus",
			summary:  "缺口账**排行面**（只读）：按件（`--by unit` · 缺省）/ 按模块（`--by module`）/ 按**判据分级**（`--by tier`）／按**入账日**（`--by day` · 新增走 found_at 轴、销案走 solved_at 轴、同给净）分桶。按**命令族**（`--by family` · 走账自带 `want_family` 原值成桶）分桶。件轴／模块轴：逐桶给 **未闭 / 已解 / 净**（可为负）+ 近邻量化（P0/P1/P2 计数）· 排序键 = ① 未闭降序 ② 近邻量化降序（都写进表头）· 对账等式自校（Σ(有件桶未闭) + 无件桶 == 账内仍缺 · 不成立 ⇒ `warnings` 点名差数）。**tier 轴**：行 = 三桶闭集（真判据 / 占位 / 无 —— `verify_tier` 已落库则用它，未落库则现读 `verify_cmd` 现算：空串/无键 ⇒ 无 · 含「占位」字样 ⇒ 占位 · 其余且能被命令树解析 ⇒ 真判据）· 列 = **条数 / 其中仍缺 / 其中已解** · 表尾固定两行：① 守恒式自校「三桶之和 == 总账」（两数各自现算 · 不等 ⇒ **判红 1** 并打印两数）② 占位桶**只报告不阻断**（不退码 · 先量分布）。全轴带 `query_ts` / `ledger_sha16`（不同时刻两个排行才可比）· 截断自报（`--top` 一页硬顶 = `gapLsRowCap`）· 读不到真源 ⇒ 8",
			usage:    "zerg gap status [--by <unit|module|tier|day|family>] [--top <N>] [--bucket <桶名>…] [--state <仍缺|已派|已立项|已解|回归|不做>…] [--prio P0|P1|P2] [--impact <命令面|门禁面|文档面|公开面|换件面|归档面>] [--unit <件路径|目录前缀>] [--module <模块前缀>] [--grep <关键词>] [--json <字段>]",
			fields:   gapStatusFields,
			endpoint: "",
			run:      cmdGapStatus,
		},
		// ★ `gap plan` —— **作业单六件**（批3 第二片 · 只读面 · 2026-09-28 · 设计稿 §3.C `C1` + §11.3）。
		//   选择器 = 第一个位置参数（件路径或模块目录前缀 · 与 `gap ls --unit/--module` 同口径）；
		//   `--module` 声明按模块面算（前导匹配）；`--top N` 一页硬顶（缺省 20 · 上限同 `gapLsRowCap`）。
		//   输出六件（设计稿 §11.3 逐字）：① 该桶全部未闭（按 prio 排序）② 出口判据 = 该桶未闭归零
		//   （回显现读未闭数）③ 建议允许面（件清单）④ 禁碰面（建议值 + 理由）⑤ 占用状态（桶内 `已派`
		//   条逐条点名卵号 · 判据 2 `M-32`）⑥ 派单模板骨架（目标/允许面/禁碰面/出口判据/时限 · 无占位符）。
		//   ★ **零未闭 ≠ 空件**（桶内有条目而 `仍缺==0` ⇒ 照出六件、真报未闭 0、退码 0）；选择器零命中 ⇒ 1。
		//   实现件 = `family_gap.go:cmdGapPlan`。
		{
			path:     []string{"gap", "plan"},
			kind:     "GapPlan",
			summary:  "作业单**六件**（只读面 · 一字不写真源、不写审计）：把一件（或一模块）的未闭账变成可直接丢给子代理的派单骨架 —— ① 该桶全部未闭（按 prio 排序 P0→P1→P2 · 同 prio 按缺口号 · `--top` 一页硬顶）② 出口判据 = 该桶未闭归零（回显现读未闭数）③ 建议允许面（该桶涉及的件清单）④ 禁碰面（建议值 + 理由）⑤ 占用状态（桶内 `已派` 条逐条点名卵号 · 判据 2 `M-32`）⑥ 派单模板骨架（目标/允许面/禁碰面/出口判据/时限 · 无占位符）· 选择器 = 第一个位置参数（件路径或模块目录前缀）或 `--unit`/`--module`；`--module` 声明按模块面算（前导匹配 `gapLsModuleHit`）· **零未闭 ≠ 空件**（桶内有条目而 `仍缺==0` ⇒ 真报未闭 0、退码 0）；选择器零命中 ⇒ 1；缺选择器/`--top` 非正整数 ⇒ 2；读不到真源 ⇒ 8",
			usage:    "zerg gap plan <件路径|模块目录前缀> [--module <模块前缀>] [--unit <件路径|目录前缀>] [--top <N>] [--allow-drifted] [--json <字段>]",
			arity:    "any",
			args:     []string{"选择器（件路径或模块目录前缀 · **恰好一个** —— 与 `gap ls --unit/--module` 同口径，含「无件」闭集取值）"},
			fields:   gapPlanFields,
			opened:   true,
			endpoint: "",
			run:      cmdGapPlan,
		},
		// ★ 批3 第四片（2026-09-28）：`gap receipt check` / `gap receipt apply` —— **回执收件面**。
		//   设计出处：§三十四 `O-12`（卵回执结构规范化：`closed_ids[]` 必填 · 取代散文）+
		//   施工清单 3-4（回执缺栏即拒收 · 闭案与账一一对应）。实现件 = `family_gap.go`。
		//   ★ 回执形状本片取定（设计稿只钉死 `closed_ids` 一名）：`egg` / `readings[]`（每条 `gap_id`+`result`）/
		//   `conclusion` / `closed_ids[]`（键必须在 · 可为空数组）—— 缺栏 ⇒ 2 并点名缺哪栏。
		{
			path:     []string{"gap", "receipt", "check"},
			kind:     "GapReceiptCheck",
			summary:  "回执**收件体检**（只读面 · 一字不写真源、不写审计）：按设计稿 `O-12` 逐条验 —— 四必填栏（卵号 `egg` · 逐条读数 `readings[]`（每条含 `gap_id`+`result`）· 结论 `conclusion` · 闭案号 `closed_ids[]`（**可为空数组，但键必须在**））缺栏 ⇒ 2 并点名缺哪栏 · 闭案与账一一对应（`closed_ids` 每个号必须在账里且当前 `state=已派`，否则点名冲突 ⇒ 2）· 反面（账里 `已派` 但回执没提）⇒ 列为「未结派单」告警（**不阻断退码**，只报告）· 读不到回执件或真源 ⇒ 8",
			usage:    "zerg gap receipt check <回执件路径> [--json <字段>]",
			arity:    "any",
			args:     []string{"回执件路径（**恰好一条** JSON 件）"},
			fields:   gapReceiptCheckFields,
			endpoint: "",
			run:      cmdGapReceiptCheck,
		},
		// ★ 缺口账 `GAP-20260928-184` 第三面（2026-10-10）：`gap receipt new` —— **卵闭案回执的生产者**。
		//   病：`gap receipt check` 要四栏卵闭案回执形状，而全仓只有 `check`/`apply`、**零产生口**
		//   （存量 receipts 全是 `dev receipt` 轮级交接回执族）。治：立合法产生口（同族 `dev receipt new` 体例）·
		//   落件沿用 receipts 目录但文件名前缀 `GR-`、件内四栏自证 ⇒ 与 `<R-…>` 族可自我区分；
		//   `check` 亦按签名键（`round_id`）判族（错族显式点名，不当「缺 4 栏」）。实现件 = `family_gap.go`。
		{
			path:     []string{"gap", "receipt", "new"},
			kind:     "GapReceiptNew",
			summary:  "卵闭案回执**生产者**（写面 · D2 档 · 缺 `--yes` fail-closed 2）：落一件卵闭案回执（`{egg, readings[], conclusion, closed_ids[]}`）到 receipts 目录（文件名前缀 `GR-`）—— 四栏与 `gap receipt check` 的收件判据**同一把尺**（生产者不许产出 check 必拒的件）· `--dry-run` 出计划件 · `--json` 走同族信封",
			usage:    "zerg gap receipt new --egg <卵号> --reading <缺口号=结果>… --conclusion <结论> [--closed-id <GAP id>]… [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"（不收位置参数 —— 四栏走旗标）"},
			fields:   gapReceiptNewFields,
			danger:   &dangerSpec{dangerD2, "卵闭案回执件", "落一件卵闭案回执（状态目录 `receipts/GR-<卵号>-<时刻>.json`）—— 逐条读数 + 结论 + 闭案号（可逆：删该件即回原态）", "缺口账 GAP-20260928-184 第三面（缺口账与自进化 · 2026-10-10）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapReceiptNew,
		},
		{
			path:     []string{"gap", "receipt", "apply"},
			kind:     "GapReceiptApply",
			summary:  "回执**闭案落账**（写面 · 缺 `--yes` fail-closed 2）：把 `closed_ids` 逐条改 `已解`（**走现有改态路径 `gapRewriteOne` · 不另开写路**）· 缺 `--evidence` ⇒ 2 · 有冲突号（账里非 `已派`）⇒ 2 且**不动账** · 成败后回显逐条改前→改后计数 · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap receipt apply <回执件路径> --evidence <一句话> --yes [--by <谁>] [--json <字段>]",
			arity:    "any",
			args:     []string{"回执件路径（**恰好一条** JSON 件）"},
			fields:   gapReceiptApplyFields,
			danger:   &dangerSpec{dangerD2, "回执件（`closed_ids`）", "把回执 `closed_ids[]` 里的号在真源逐条改 `已解`（`state`/`solved_at`/`solved_evidence`）+ 审计逐条一行（可逆：照审计那一格回写）；缺栏 / 冲突号 ⇒ 2（一个字节都不写）", "批3 第四片（缺口账与自进化 · 2026-09-28）· 设计稿 §三十四 `O-12` + 施工清单 3-4 · 同族写面先例 `gap set-state`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapReceiptApply,
		},
		// ★ `gap show` —— **单条取全文**（2026-09-28 · 缺口账 `GAP-20260926-15` + 同族 `GAP-20260927-243`）：
		//   只读面（不登记 `danger` ⇒ 按只读幂等档列）· 点名面复用同族 `gapTargetOne`（**账内没有这个 id ⇒ 2**，
		//   与 `set-state` / `note` 同一口径）· 读不到真源 ⇒ 8。实现件 = `family_gap.go:cmdGapShow`。
		{
			path:    []string{"gap", "show"},
			kind:    "GapShow",
			summary: "取一条缺口的**完整正文**（只读面 · 一字不截）：正文 / 手搓记录 / 复现 / 判据 / 已解证据 / 口径注全量出 —— `gap ls` 那一行只印摘要前 40 显示宽、`--json` 全量直出又会被读方截断（缺口账 `GAP-20260927-243`）· 账内没有这个 id ⇒ 用法错 2（同族口径 · 不跨族抄 `dev proposal show` 的 1）· 读不到真源 ⇒ 8",
			usage:   "zerg gap show <GAP id|件路径> [--json <字段>]（位置参数给一条件路径 = 按件反查：该件在账里被点到的条目逐条列出 · 件不在仓里 ⇒ 2 · 仓根取不到 ⇒ 8）",
			arity:   "any",
			args:    []string{"缺口 id（**恰好一条** —— 本面只取一条）"},
			fields:  gapShowFields,
			optFields: []string{"hints",
				// ★ 缺口 `GAP-20260926-70`（2026-10-23 · 本枚）：`qid` / `qid_source` —— 单条面的
				//   同一份对应格（与 `gap ls` 同源同形 · 同走可选字段层）。老行 ⇒ 空串不出键。
				"qid", "qid_source"},
			endpoint: "",
			run:      cmdGapShow,
		},
		// ★ `gap hints` —— **正门提示面**（只读 · 2026-10-10 · 缺口 `GAP-20261010-26` ＝ 父
		//   `GAP-20260929-09` 的第二形态「正门 gap/contract 子命令出口」）。逐条给「怎么核 /
		//   怎么复现 / 下一步」的落点提示；真源 = 账内既有 `verify_cmd`/`repro_cmd`（✗ 不新增
		//   真源键）。只读面（不登记 `danger`）· 读不到真源 ⇒ 8。实现件 = `family_gap.go:cmdGapHints`。
		{
			path:     []string{"gap", "hints"},
			kind:     "GapHints",
			summary:  "缺口**正门提示面**（只读面 · 一字不写真源、不写审计）：逐条给「怎么核 / 怎么复现 / 下一步」的落点提示（真源 = 账内既有 `verify_cmd`/`repro_cmd`，✗ 不新增真源键）· `gap show <id> --json hints` 给同一条的同一份提示（走可选字段层）· 读不到真源 ⇒ 8",
			usage:    "zerg gap hints [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：范围 = 整账 —— 要收窄走 `gap ls`）"},
			fields:   gapHintsFields,
			endpoint: "",
			run:      cmdGapHints,
		},
		{
			path:     []string{"gap", "add"},
			kind:     "GapAdd",
			summary:  "记一条缺口（写面 · 留证据）：**手搓记录 + 验证命令是两件必填**（防呆⑤）· 同 fp 同内容 ⇒ 幂等命中 0 · 同 fp 内容不同 ⇒ `14` · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap add --symptom <一句> --handmade <命令原样> --impact <六值之一> --want-family <族> --want-action <动作> [--want-argv <段>…] [--prio P0|P1|P2] [--state <仍缺|已立项|不做>] --repro-cmd <命令> --verify-cmd <命令> [--depends-on <fp>…] [--qid <Q-NNN> [--qid-source <出处锚>]] [--note <注>|--notes <注>|--evidence <注>…] [--by <谁>] [--dry-run] [--yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：全部走旗标）"},
			fields:   gapAddFields,
			danger:   &dangerSpec{dangerD2, "缺口 fp", "往真源（`<状态目录>/zerg-cli-gaps.jsonl`）追加一行 + 审计一行（可逆：删那一行 / 审计历史行不删）；审计写不进 ⇒ 真源一行不写", "设计-命令面-gap族-v1.0-20260923.md §二.2 · §四 · `H-10`（不要人签：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapAdd,
		},
		{
			path:     []string{"gap", "verify"},
			kind:     "GapVerify",
			summary:  "跑判据（`verify_cmd`）改缺口状态（写面）：过 ⇒ `已解` + `solved_evidence`·**已解现缺** ⇒ 记 `回归` 并退 1 · `--dry-run` 只跑只印（恒 0）",
			usage:    "zerg gap verify [<GAP id>…] [--all] [--by <谁>] [--dry-run] [--yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（可重复；与 `--all` 不许同给）"},
			fields:   gapVerifyFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state` / `solved_evidence` / `last_verified_at` + 审计一行（可逆：照审计那一格回写）；判红（`回归`）只在真跑那一态可达", "设计-命令面-gap族-v1.0-20260923.md §二.3 · §三 · `H-10`（判据是机器给的：跑命令看 rc）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapVerify,
		},
		// ★ 批4 第一片（2026-09-28）：`gap verify-one <GAP id>` —— **跑判据即写**（一条一次）。
		//   病（设计稿 §11.4 批4 · §11.2 批2 D1/D2）：`gap verify` 是**批量面**（点名多条 / `--all`），
		//   且只有 `--yes` 那一态写 `last_verified_at` ⇒ 「跑判据即写」没落地（现读账里该格仅 1 条）。
		//   本面 = 一条一次：跑该条的 `verify_cmd`，把 **last_verified_at / last_verify_rc /
		//   verify_tier** 三格写回该条（走本族唯一收口 `gapRewriteOne`：审计先落盘 → 整件重写 →
		//   读回对拍；只重写目标那一行）。
		//   ⚠ 命令名取定 = `verify-one`：`gap verify` 这一格**已被判据批量面占用**（形状/退码一字不许动）
		//   ⇒ 不占用同一条命令树路径（回执 CLI 缺口栏如实点名）。
		//   四条硬纪律：① `--dry-run` 只出计划件（**不跑判据**、不写账 · 恒 0）② 缺 `--yes` 真写 ⇒ fail-closed 2
		//   ③ 该条**无 `verify_cmd`** ⇒ 2 并点名（不当绿）④ 带 shell 元字符（管道/重定向/分号/与或）的判据
		//   **一律拒跑并点名**（不当绿也不当红）。
		{
			path:     []string{"gap", "verify-one"},
			kind:     "GapVerifyOne",
			summary:  "跑一条缺口的判据（`verify_cmd`）**一条一次**并**把结果写回该条**（写面）：写 `last_verified_at`（现取时刻）· `last_verify_rc`（真退码）· `verify_tier`（真判据/占位/无）· 走本族唯一收口 `gapRewriteOne`（只重写目标那一行 · 审计进 `edit_audit.jsonl`）· `--dry-run` 只出计划件（不跑判据 · 恒 0）· 缺 `--yes` ⇒ 2 · 无 `verify_cmd` ⇒ 2 点名 · 判据带 shell 元字符 ⇒ 拒跑点名",
			usage:    "zerg gap verify-one <GAP id> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本面一条一次）"},
			fields:   gapVerifyOneFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `last_verified_at` / `last_verify_rc` / `verify_tier` + 审计一行（可逆：照审计那一格回写）；**本面不改 `state`**（改态是 `gap verify` / `set-state` 的事）", "设计-缺口账与自进化-v2.0-20260928.md §11.4 批4 · §11.2 批2 D1/D2 · 同族写面先例 `设计-命令面-gap族-v1.0-20260923.md §二.2/§三/§四`", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapVerifyOne,
		},
		// ★ 批4 第三片（2026-09-28）：`gap regress <件…>` —— **分层回归巡检**（只读面）。
		//   设计出处：`设计-缺口账与自进化-v2.0-20260928.md` §11.4 批4 D3「回归巡检」+ §附录
		//   「巡检条款措辞限缩（35.2 源四）」。病（§11.2 批4 · §11.4）：`verify-one` 一条一次 ⇒
		//   改一件之后无法快速知道「我碰过的那件名下账里那些条还站不站得住」；全跑贵且慢（`M-34`）。
		//   本面 = **只跑被点名件名下**的账（`unit` **精确等值**该件 · 不含模块前缀面），逐条跑其
		//   `verify_cmd`（进程内走同一 `run` 入口 · 不拼 shell 串），逐件汇总六数：
		//   条数 / 实跑 / 通过 / 失败 / 拒跑 / 未跑（被 `--limit` 截掉的 · 缺省 20）。
		//   ★ **声称纪律（本片核心）**：表尾**逐字**声明「本次只覆盖 modification-traversing 级
		//   （只跑了与被改件直接相关的账）；这不等于 safe，也不覆盖非确定性/环境依赖场景」。
		//   某件零条 ⇒ 明写「零条（不是通过）」。
		//   ★ 安全边界：只跑账里那一格命令本身；含 shell 元字符（管道/重定向/分号/与或/反引号/`$`/
		//   括号）⇒ **一律拒跑并点名**（不当绿也不当红）。本面**全只读**（不写账、不写审计、不改态）。
		//   退码：0 全过（含零条 / 全是拒跑）· 1 有条判红（`verify_cmd` rc≠0）· 2 用法错（缺件 /
		//   件不在仓 / `--limit` 非非负整数 / `--json` 不给字段）· 8 真源读不到 / 仓根取不到。
		//   实现件 = `family_gap.go:cmdGapRegress`。
		{
			path:     []string{"gap", "regress"},
			kind:     "GapRegress",
			summary:  "缺口账**分层回归巡检**（只读面 · 一字不写真源、不写审计、不改任何 `state`）：只跑被点名件名下（`unit` **精确等值**该件 · 不含模块前缀面）账内各条的 `verify_cmd` —— ① 汇总六数：件 → 条数 / 实跑 / 通过 / 失败 / 拒跑 / 未跑（被 `--limit` 截掉的 · 缺省 20）② 状态分栏（仍缺/已派/已立项/已解/回归/不做 · 不分状态全列）③ **声称纪律**：表尾逐字声明「只覆盖 modification-traversing 级（只跑了与被改件直接相关的账）；这不等于 safe，也不覆盖非确定性/环境依赖场景」④ 某件零条 ⇒ 明写「零条（不是通过）」。安全边界：只跑账里那一格命令本身 —— 含 shell 元字符（管道/重定向/分号/与或/反引号/`$`/括号）⇒ **一律拒跑并点名**（不当绿也不当红）；写面/无判据/占位 ⇒ 拒跑。件不在仓 ⇒ 2 并点名；`--limit` 非非负整数 ⇒ 2；读不到真源 / 仓根取不到 ⇒ 8。",
			usage:    "zerg gap regress <件路径>… [--limit <N>] [--dry-run] [--json <字段>]",
			arity:    "any",
			args:     []string{"件路径（仓内相对路径 · **一个或多个** —— 只这些件名下的账会被跑）"},
			fields:   gapRegressFields,
			endpoint: "",
			run:      cmdGapRegress,
		},
		// ★ 本枚（缺口账 `GAP-20260927-248`）：`gap reread <GAP id>…|--all` —— **判据复跑**（只读面）。
		//   病（`GAP-20260927-248` 逐字）：`solved_evidence` 是**一次性文本**（"现有 75 条非空"）而非
		//   **可重放结果** ⇒ 「已解」条目**是否已回归**机器判不出（只能靠 `gap verify` 现跑，而它自己
		//   又不设防 —— 见 `GAP-20260927-247`）。治法 = 重跑 `solved_evidence` 里的**带键标读数**
		//   （`cmd=<命令> · rc=<真退码> · reading=<关键读数一行>`），与记下的 `rc` / `reading` 逐条对拍：
		//   rc 变 ⇒ `reading_stale`；rc 一致再看 `件:行` 引用的**行内容**（✗ 不判绝对行号 —— 行号会漂）。
		//   ★ 口径（只报告 · 不对拍不进退出码 —— 免得存量陈旧把它变成恒红）：四条读数闭集
		//   `ok` / `reading_stale` / `not_rereadable` / `skipped`；**安全铁律** = 取值命令落在**危险档**
		//   （写面）/ 含 shell 元字符 / 解析不出 / 是本命令自己（自递归）⇒ **一律不执行**（判不准就丢
		//   `not_rereadable`）。★ 只读动作 ⇒ **不挂危险档**（本面只报告 · 一字节都不写真源、不写审计）。
		//   退码：0（含有 `reading_stale`）· 2 用法错（缺目标 / `--all` 与 id 同给 / `--limit` 非非负整数 /
		//   `--json` 不给字段 / 账内没有该 id）· 8 真源读不到。实现件 = `family_gap_reread.go:cmdGapReread`。
		//   ★ 2026-10-05（缺口账 `GAP-20261005-60` 剩面 · 本枚）：新增 `--include-open` —— 把**未闭**
		//   （`state == 仍缺`）条目的 `verify_cmd` 纳入同一套判活读数（可跑 ⇒ ok；元字符 / 占位 / 自递归 /
		//   空 / 解析不出 ⇒ not_rereadable · **不执行**）；解析面在 `parseInvocation` 专用 case 上户口，
		//   dispatch 面按 usage 声明收口（与 `--top` 同形：别族给它会仍退 2，不静默吞）。
		{
			path:     []string{"gap", "reread"},
			kind:     "GapReread",
			summary:  "缺口账**判据复跑**（只读面 · 一字不写真源、不写审计、不改任何 `state`）：重跑 `solved_evidence` 里的**带键标读数**（`cmd=<命令> · rc=<真退码> · reading=<关键读数一行>`）并与记下的 `rc` / `reading` 对拍 —— rc 变 ⇒ `reading_stale`；rc 一致再看 `件:行` 引用的**行内容**（✗ 不判绝对行号 —— 行号会漂）· 四条读数闭集 `ok` / `reading_stale` / `not_rereadable` / `skipped` · **安全铁律**：取值命令落在**危险档**（写面）/ 含 shell 元字符 / 解析不出 / 是本命令自己（自递归）⇒ **一律不执行**（判不准就记 `not_rereadable`）· 表尾声称「**只报告** —— `reading_stale` / `not_rereadable` **均不进退出码**」（免得存量陈旧读数把它变成恒红）· `--include-open` 把**未闭**条（`state == 仍缺`）的 `verify_cmd` 纳入同一套判活（可跑 ⇒ ok；元字符 / 占位 / 自递归 / 空 / 解析不出 ⇒ `not_rereadable` · **不执行**）· 缺目标 / `--all` 与 id 同给 / `--limit` 非非负整数 / `--json` 不给字段 / 账内没有该 id ⇒ 2；读不到真源 ⇒ 8",
			usage:    "zerg gap reread [<GAP id>…] [--all] [--include-open] [--limit <N>] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**一条或多条**；与 `--all` 不许同给）"},
			fields:   gapRereadFields,
			endpoint: "",
			run:      cmdGapReread,
		},
		{
			path:     []string{"gap", "export"},
			kind:     "GapExport",
			summary:  "缺口真源 → 「结转账 markdown 片段」导出面（**只打 stdout 时零写盘**：不写真源、不写审计；**`--out <目录>` 那一支会拆件落盘** ⇒ 本表项按**写面**登记 `D2`）· **只列未闭**（`已解`/`不做` 不进正文，只进件头计数）· 件头 = 仪表盘（条数/未闭/已闭/逐面计数/最老未闭天数）· 单行 ≤200 字符 · 读不到真源 ⇒ 退 8（fail-closed）",
			usage:    "zerg gap export [--out <目录>] [--json <字段>] [--dry-run | --yes]",
			fields:   gapExportFields,
			danger:   &dangerSpec{dangerD2, "落点目录（`--out`）", "往 `--out <目录>` 拆件落盘（索引件 1 + 页件 N：`缺口总账-<日期>.md` / `-<面>-pNN.md`）；真源 `zerg-cli-gaps.jsonl` 与审计 `edit_audit.jsonl` **一个字不动**（可逆：删掉新落的那些件即回到原状）", "缺口账（2026-09-27）· `ddf57491` 接上 `--out` 真写盘 ⇒ 旧表项无 `danger`，本族写命令一直被 `zerg help` 判成**安全档** ✗ · 同族写面先例 `设计-命令面-gap族-v1.0-20260923.md §二.2/§三/§四` · `H-10`（`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapExport,
		},
		{
			// ★ 缺口 `GAP-20261013-20`（F30 接线 · 2026-10-15）：`gap diff` —— 缺口账**差集面**（只读对拍）。
			// 实现件 = `family_gap_diff.go:cmdGapDiff`。两份**外部件**（账 jsonl / `gap export` 人面片段）逐格对拍
			// ⇒ 三类差（仅左 · 仅右 · 同 id 不同体 · 逐条点名到 id ＋ 哪一格）。退码 **0/1/2**：
			// 0 = 无差异 · 1 = 有差异（逐条点名）· 2 = 输入取不到 / 格式不对 / 用法错（**不给结论**）。
			// ★ 与同族「真源读不到 ⇒ 8」**不同码**：本面是两份**外部件**的对拍，缺件 / 坏件属**前置/用法面**
			// （派单口径 = 2 ⇒ `exitUsage`）。用法串须写明 `--out`（本命令认它）。
			// ★ `--out <件路径>` 落**报告件**（D2 写面 · ✗ 不写真源、不写审计）：`--dry-run` ⇒ 计划件走 stdout
			// ＋ 一个字节不落（rc 仍照真跑 0/1）；缺 `--yes` ⇒ 2（fail-closed）。
			path:    []string{"gap", "diff"},
			kind:    "GapDiff",
			summary: "缺口账**差集面**（只读）：两份账 / 两份导出对拍 ⇒ 三类差（仅左 · 仅右 · 同 id 不同体），逐条点名到 id ＋ 哪一格",
			usage:   "zerg gap diff <左件> <右件> [--json <字段>] [--out <件路径> [--dry-run|--yes]]",
			arity:   "any",
			args:    []string{"左件 · 右件（各一份账 jsonl 或 `gap export` 人面片段）"},
			fields:  gapDiffFields,
			run:     cmdGapDiff,
		},
		// ★ 两条**状态写面**（2026-09-26 · 缺口账 `Q-239`：「`gap` 族只有 4 个动作 ⇒ 改态/写口径
		//   没有正门」）。照同族写面（`add` / `verify`）的式样：`--dry-run` 恒 0 / 缺 `--yes`
		//   fail-closed 2 / 审计先落盘（写不进就不写真源）/ `--by`-`--yes`-`--json` 习惯逐字沿用。
		//   实现件 = `family_gap_state.go`（本族**只这两条**用 `--evidence` / `--text` 两枚旗标）。
		{
			path:     []string{"gap", "set-state"},
			kind:     "GapSetState",
			summary:  "改一条缺口的状态（写面 · 留证据）：`--state` 只认**三值闭集**（`仍缺` / `已解` / `不做`）—— 闭集外 ⇒ 2 并逐字印闭集 · 转 `已解` 写 `solved_at` + `solved_evidence` · **销案两态对拍**（批4 第四片 · 设计稿 §11.5 · 表 `D6`）：改到 `已解` 时**新账**（缺口号内日期 ≥ `2026-09-28`）**必须**带改前/改后两条读数（`--before-read` / `--after-read` · 每条形如 `cmd=<命令> · rc=<真退码> · reading=<关键读数一行>`）—— 缺哪一段就退 2 并**逐段点名**（改前读数 / 改后读数）· **存量账**（号内日期 < 阈值）豁免但**不静默**：表尾逐字印「本发为存量豁免（未带两态对拍）」· 改**非** `已解` 的状态 ⇒ 本闸不适用（行为逐字不变）· 同 id 同态同证据 ⇒ 「无变化」0（不写）· 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap set-state <GAP id> --state <仍缺|已解|不做> --evidence <一句话> [--solved-kind <实现|销案|前提不成立|部分>] [--remainder <残面描述>] [--before-cmd <命令串>] [--after-cmd <命令串>] [--judge-cmd <命令串>] [--before-read <cmd=… · rc=… · reading=…>] [--after-read <cmd=… · rc=… · reading=…>] [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本动作不做批量改态）"},
			fields:   gapSetStateFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state` / `solved_at` / `solved_evidence` + 审计一行（可逆：照审计那一格回写）；`--state` 闭集外 ⇒ 2（一个字节都不写）", "缺口账 `Q-239` · 同族写面先例 `设计-命令面-gap族-v1.0-20260923.md §二.2/§三/§四` · `H-10`（`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapSetState,
		},
		// ★ 2026-10-09（缺口账 `GAP-20261008-06`/`-07` · 「改 unit / 改 want 均无正门」）：`gap set-unit`
		//   —— **改件面三键（`unit` / `module` / `unit_source`）的正门**。病源：退役后旧 `unit` 指向已退场件
		//   ⇒ 只能往门75 存量豁免表加行（**只增不删**）绕过；有本门后这类条可**归位**。实现件 =
		//   `family_gap_state.go:cmdGapSetUnit`。形制照同族五写面：走唯一收口 `gapRewriteOne`（只重写目标那一行）·
		//   `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2（D2）· `--evidence` 必填（落成 `notes` 一条注）·
		//   `--unit` 不在仓内 ⇒ 默认拒 2（须显式 `--allow-missing-unit`）。
		{
			path:     []string{"gap", "set-unit"},
			kind:     "GapSetUnit",
			summary:  "改一条缺口的**件面三键**（`unit` / `module` / `unit_source` · 写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：走族内唯一收口 `gapRewriteOne`（只重写目标那一行 · 其余行逐字节不变 · 行序不变 · 末行换行不变）· `--unit` 必填（仓内相对路径）· `--module` 缺省由 `unit` 目录前缀推（`gapModuleOfUnit`）· `--unit-source` 缺省由 `unit` 形态推（兜底档 `hand` / 仓内路径 `auto` · 闭集 `auto|hand`）· `--evidence` 必填（**改件留证** —— 落成该条 `notes` 末尾一条注）· ★ **`--unit` 不在仓内（`git ls-files` 现读对不上）⇒ 默认拒 2**（一个字节都不写），须显式 `--allow-missing-unit` 才放行（兜底档 `无件(命令面)`/`拟(x)` 不受此闸）· 同 id 同三键 ⇒ 「无变化」0 · 回执回显「改前 → 改后」三键 · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap set-unit <GAP id> --unit <件路径> [--module <模块前缀>] [--unit-source <auto|hand>] [--allow-missing-unit] --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本动作不做批量）"},
			fields:   gapSetUnitFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源那一行的 `unit` / `module` / `unit_source` 三键 + 往 `notes` 追加一条改件留证注 + 审计一行（可逆：照审计那一格回写 · 留证注删掉即回原状；其余键名与值逐字节不变 · 行序不变 · 末行换行不变）；`--unit` 不在仓内且未带 `--allow-missing-unit` / 缺 `--unit` / 缺 `--evidence` / 收到 `--state` ⇒ 2（一个字节都不写）", "缺口账 `GAP-20261008-06`/`-07`（「改 unit 无正门」· 治法）· 同族写面先例 `gap set-state` / `gap note` / `gap assign` / `gap verify-one`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapSetUnit,
		},

		// ★ 2026-10-23（`GAP-20261008-07` · 「want 无 CLI 写面」的治法）：`gap set-want`
		//   —— **改 want 三格（`want_family` / `want_action` / `want_argv`）的正门**。
		//   病源：同族五写面（`set-state` / `note` / `set-unit` / `judge set` / `assign`）都不碰 want 两格
		//   ⇒ 拍板换了方向只能手改盘上真源（绕审计 / 绕账写锁 / 绕一切校验）。
		//   实现件 = `family_gap_state.go:cmdGapSetWant`。形制照同族 `set-unit`：走族内唯一收口
		//   `gapRewriteOne`（写前取锁走**写面落点** `gapLedgerWritePath` · 只重写目标那一行）。
		{
			path:     []string{"gap", "set-want"},
			kind:     "GapSetWant",
			summary:  "改一条缺口的 **want 三格**（`want_family` / `want_action` / `want_argv` · 写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：走唯一收口 `gapRewriteOne`（写前取锁走写面落点 · 只重写目标那一行 · 其余行逐字节不变）· 族名闭集 = 命令树顶层族 ∪ 账内现用原值 · `--evidence` 必填 · `state` 一个字节不动 · 只读借用档硬拒 2",
			usage:    "zerg gap set-want <GAP id> --want-family <族> --want-action <动作> [--want-argv <段>…] --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本动作不做批量）"},
			fields:   gapSetWantFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `want_family` / `want_action` / `want_argv` 三格 + 往 `notes` 追加一条改口径留痕注 + 审计一行（可逆：照审计那一格回写；`state` 逐字不动 · 其余行逐字节不变）", "缺口账 `GAP-20261008-07`（want 无写面 · 治法）· 同族写面先例 `gap set-unit` / `gap judge set`", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapSetWant,
		},
		// ★ 2026-10-10（缺口账 `GAP-20261010-34`）：`gap judge set` —— 账内 ~99 条判据「无鉴别力 /
		//   与 want 无关」，全卡在缺这个「改判据」写面（现读只有 `set-state` 改态、`set-unit` 改件面三键，
		//   两键 `verify_cmd` / `repro_cmd` 无正门）。形制照同族 `set-unit`：走唯一收口 `gapRewriteOne` ·
		//   `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2（D2）· `--evidence` 必填；校验逐字复用 `gap add`
		//   的既有助手（样板值闸 `gapBoilerplateWhy` + 命令树解析面 `gapJudgeArgv`）。实现件 = `family_gap.go:cmdGapJudgeSet`。
		{
			path:     []string{"gap", "judge", "set"},
			kind:     "GapJudgeSet",
			summary:  "改一条缺口的**判据两键**（`verify_cmd` / `repro_cmd` · 写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：走族内唯一收口 `gapRewriteOne`（只重写目标那一行 · 其余行逐字节不变 · 行序不变 · 末行换行不变）· `--verify-cmd` 必填（命令树里真有的命令 —— 逐字复用 `gap add` 的样板值闸 `gapBoilerplateWhy` + 解析面 `gapJudgeArgv`：通用内省型 / 解析不到 / 写面 / 自递归 ⇒ 拒 2）· `--repro-cmd` 缺省保留旧值（同 `gap add` 口径只过样板值闸）· `--evidence` 必填（改判据留证 —— 落成该条 `notes` 末尾一条注）· 同 id 同两键 ⇒ 「无变化」0 · 回执回显「改前 → 改后」两键 · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap judge set <GAP id> --verify-cmd <命令串> [--repro-cmd <命令串>] --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本动作不做批量）"},
			fields:   gapJudgeSetFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `verify_cmd` / `repro_cmd` 两键 + 往 `notes` 追加一条改判据留证注 + 审计一行（可逆：照审计那一格回写 · 留证注删掉即回原状；其余键名与值逐字节不变 · 行序不变 · 末行换行不变）；`--verify-cmd` 是样板值 / 解析不到命令树 / 缺 `--verify-cmd` / 缺 `--evidence` / 收到 `--state` ⇒ 2（一个字节都不写）", "缺口账 `GAP-20261010-34`（账内判据无鉴别力 · 治法的一半）· 同族写面先例 `gap set-state` / `gap set-unit`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapJudgeSet,
		},
		// ★ 批3 第五片（2026-09-28）：`gap bulk set-state` —— **族级批量改态**（一次改 N 条）。
		//   病源 = 缺口账 `GAP-20260926-97`（逐字：想要 `zerg gap-batch-state` · 「一次只能改一个 GAP id
		//   （结账/收版时逐条改态成本高）」）；本片 = 该条点名的治法，**不另开写路**：逐条仍走
		//   `family_gap_state.go:gapRewriteOne` 那一个收口（审计先落盘 → 整件重写 → 写后读回对拍）。
		//   四条硬纪律：① 全量预检先行（任一不合规 ⇒ 整批拒 2 + 逐条点名 + **一字不写**）
		//   ② 逐条改态、逐条写**自己那一行**的 evidence ③ 任一条写失败 ⇒ 当场停手 + 如实报「已改 m/未改 n」
		//   ④ `--dry-run` 恒 0 / 缺 `--yes` fail-closed 2（D2）。实现件 = `family_gap_state.go:cmdGapBulkSetState`。
		{
			path:     []string{"gap", "bulk", "set-state"},
			kind:     "GapBulkSetState",
			summary:  "族级**批量改态**（一次改 N 条 · 写面）：第一位置参 = 清单件（JSONL · 每行至少 `id` 与 `evidence` 两键）· `--state` 闭集同单条（`仍缺` / `已解` / `不做`）· **全量预检先行**：任一条缺 `id`/`evidence`、或 `id` 不在账、或该条当前 `state` 不可改（必须是闭集三值之一 —— `已派`/`已立项`/`回归` 不归本面）⇒ **整批拒**（退 2 · 逐条点名到行 · **一字不写**）· 全过后才真写：逐条改态、逐条写自己的 `evidence`、审计逐条一行（走现有单条改态路径 `gapRewriteOne`，不另开写路）· 任一条写失败 ⇒ **当场停手**并如实报「已改 m 条 / 未改 n 条」· 表尾回显总数 / 逐条改前 → 改后 / 失败条数 · `--dry-run` 恒 0 零副作用 · 缺 `--yes` fail-closed 2",
			usage:    "zerg gap bulk set-state <清单件.jsonl> --state <仍缺|已解|不做> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"清单件路径（**恰好一个** · JSONL：每行至少 `id` 与 `evidence` 两键）"},
			fields:   gapBulkSetStateFields,
			danger:   &dangerSpec{dangerD2, "整批缺口 id（清单件点名的那几条）", "按清单逐条改真源里的 `state` / `solved_at` / `solved_evidence`（逐条写自己的 evidence）+ 审计**逐条**一行（可逆：照审计那一格逐条回写）；任一条不合规 ⇒ 2 且**一个字节都不写**（预检在真写之前）", "批3 第五片（缺口账与自进化 · 2026-09-28）· 缺口账 `GAP-20260926-97`（本片治法逐字）· 同族写面先例 `gap set-state` / `gap assign`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapBulkSetState,
		},
		{
			path:     []string{"gap", "note"},
			kind:     "GapNote",
			summary:  "给一条缺口追加一条**口径/上下文注**（写面）：落真源那一行的 `notes` 数组（`<时刻> · <谁>：<文本>`）· **不改 `state`**（形状里根本没有 `--state` ⇒ 收到即拒 2）· 同 by 同 text 已在位 ⇒ 「无变化」0（不写）· 审计进 `edit_audit.jsonl`（写不进就不写真源）· `--retract <n|指纹>` 给已落的注打**作废标记**（`notes_void` 追加 · **禁真删**：原文逐字节留在 `notes` 里 · 读面默认不显示作废项 · `--json void_notes` 带原文出来）",
			usage:    "zerg gap note <GAP id> [--text <一句话> | --retract <n|指纹> | --supersede <n>] [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条**）"},
			fields:   gapNoteFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "往真源那一行的 `notes` 数组追加一条（可逆：删那一格的那一条）+ 审计一行；`state` 逐字不动（改前改后现算对拍）；`--retract <n|指纹>` 只**追加**一条作废标记（`notes_void` · 可逆：删标记那一条）—— **禁真删任何注**，读面按标记过滤", "缺口账 `Q-239` · 同族写面先例同 `gap set-state` · 「注不改态」是本条与 `set-state` 的分工线", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapNote,
		},
		// ★ 批3 第一片（2026-09-28）：`gap assign`（作业单 + 派单登记）—— 让状态机的 `已派` **首次真启用**
		//   （现读 0 条）。设计出处：`设计-缺口账与自进化-v2.0-20260928.md` §11.3（派单登记 = 写 `已派` +
		//   `notes` 记卵号与回执绝对路径）+ §4 判据 2（自派：`已派` 条数 == 在飞卵数 · 作业单必须带占用状态）
		//   + §8 `M-32`（「同改文件只许一路」⇒ `assign` 前先查占用）。
		//   ★ 写面**只走现有改态路径**（`family_gap_state.go:gapRewriteOne` 那一个收口 · 不另开写路）：
		//   `state` → `已派` 与 `notes` 追加登记**同一次**重写、一条审计行。六栏缺一栏 ⇒ 2 并点名缺哪栏。
		//   实现件 = `family_gap_state.go:cmdGapAssign`（写面）/ `family_gap.go:cmdGapAssignLs`（只读面）。
		{
			path:     []string{"gap", "assign"},
			kind:     "GapAssign",
			summary:  "派单登记（写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：把该条 `state` 改 **`已派`**（走现有改态路径 `gapRewriteOne` · 不另开写路）并在 `notes` 末尾追加**一行机读登记**（`派单登记 egg=… · receipt=… · allow=… · forbid=… · occupies=… · criterion=… · deadline=…`）· **作业单六栏缺栏拒发**（① `--allow` 允许面 ② `--forbid` 禁碰面 ③ `--occupies` 占用件 ④ `--criterion` 出口判据 ⑤ `--deadline` 时限 ⑥ 该条全文 —— 缺一栏 ⇒ 2 并点名缺哪栏）· 预检「可派状态」= `仍缺` / `已立项`（非此 ⇒ 2 并点名当前 state）· `M-32` 占用预检（同件已被另一 `已派` 占住 ⇒ 2）· 同卵号登记已在位 ⇒ 「无变化」0",
			usage:    "zerg gap assign <GAP id> --egg <卵号> --allow <件清单> --forbid <面> --occupies <件|无> --criterion <出口判据> --deadline <绝对日期> [--receipt <回执绝对路径>] [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 一次只派一条 · 本动作不做批量）"},
			fields:   gapAssignFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state`（→ `已派`）+ 往 `notes` 追加一行派单登记 + 审计一行（可逆：照审计那一格回写 · 登记那一行删掉即回原状）；六栏缺一栏 / 非可派状态 ⇒ 2（一个字节都不写）", "批3 第一片（缺口账与自进化 · 2026-09-28）· 设计稿 §11.3 + §4 判据 2 + §8 `M-32` · 同族写面先例 `gap set-state` / `gap note`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapAssign,
		},
		{
			path:     []string{"gap", "assign", "ls"},
			kind:     "GapAssignLs",
			summary:  "派单**读面**（只读 · 一字不写真源、不写审计）：列 `已派` 条与它们的作业单登记（六栏 + 卵号 + 回执 + 登记时刻）· `--egg <卵号>` 收窄 · `--aging`（按登记时刻与时限算超时多少 · **降序**最久的在最前 · 时限算不出的单列「时限不明」**不当零**）· `--stale`（只列超时限未收的派单 · 无一条 ⇒ rc 0 且明写「无超时派单」）· 表尾**固定**回显对账口径行（`已派 N 条 —— 请与平台在飞清单逐条比对` · N 现读现算），有超时条再点名它们的缺口号 + 卵号 + 超时时长（**只报告不自动断言** —— 在飞不在本 CLI 真源内）· `--json` 走同族信封（`meta.total` / `meta.hits` / `query` / `query_ts` / `ledger_sha16` / `assign_total` / `overdue_count` / `reconcile` + 截断自报）· 零命中 ⇒ 1（`已派` 0 条不是绿）· 真源读不到 ⇒ 8。判据 2「自派」的计数落点",
			usage:    "zerg gap assign ls [--egg <卵号>] [--aging] [--stale] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：收窄走 `--egg`）"},
			fields:   gapAssignListFields,
			opened:   true,
			endpoint: "",
			run:      cmdGapAssignLs,
		},
		// ★ 三段链中段（2026-10-05 · 缺口账 `GAP-20261005-61` · 设计稿 `~/.zerg/state/设计-gap立项正门-20261005.md` §2 乙案 / §3 / §7 U1）：
		//   `仍缺 ──project──▶ 已立项（立项待拍） ──assign──▶ 已派（派单）`。甲案「扩 `set-state` / `bulk`
		//   的 `gapStateSettable` 闭集」已否 ⇒ 本面**只增两条动作**，`set-state` / `bulk` 一字不动 ✗。
		//   实现件 = `family_gap_state.go:cmdGapProject` / `cmdGapUnproject`（共用 `gapProjectRun`）。
		{
			path:     []string{"gap", "project"},
			kind:     "GapProject",
			summary:  "立项待拍（写面 · 三段链中段）：源 {`仍缺`, `不做`} → 目标**恒** `已立项`（**不收 `--state`** —— 目标态由动作名唯一确定，甲案「扩 set-state 闭集」已否）· `--evidence` **必填**（改态留证）· 转入 `已立项` **不取 `solved_at`**（非终态）· 同 id 同态同证据 ⇒ 「无变化」0（不写）· `--dry-run` 恒 0 零副作用 · 缺 `--yes` fail-closed 2 · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap project <GAP id> --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本动作不做批量）"},
			fields:   gapProjectFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state`（→ `已立项`）+ `solved_evidence`（**不碰** `solved_at` / `solved_kind`）+ 审计一行（可逆：照审计那一格回写）；源态不在源闭集 / 收到 `--state` / 缺 `--evidence` ⇒ 2（一个字节都不写）", "缺口账 `GAP-20261005-61` · 设计稿 `设计-gap立项正门-20261005.md` §2 乙案 / §3 三段链 / §7 U1 · 同族写面先例 `gap set-state`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapProject,
		},
		{
			path:     []string{"gap", "unproject"},
			kind:     "GapUnproject",
			summary:  "撤立项（写面 · 三段链回退面 §7 U1）：源 {`已立项`} → 目标**恒** `仍缺`（**不收 `--state`**）· `--evidence` **必填** · 非 `已立项` 条 ⇒ 按源闭集拒执 2（逐字印当前 state + 源闭集）· 同 id 同态同证据 ⇒ 「无变化」0 · `--dry-run` 恒 0 零副作用 · 缺 `--yes` fail-closed 2 · 审计进 `edit_audit.jsonl`（写不进就不写真源）",
			usage:    "zerg gap unproject <GAP id> --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"缺口 id（**恰好一条** —— 本动作不做批量）"},
			fields:   gapProjectFields,
			danger:   &dangerSpec{dangerD2, "缺口 id", "改真源里的 `state`（→ `仍缺`）+ `solved_evidence`（**不碰** `solved_at` / `solved_kind`）+ 审计一行（可逆：照审计那一格回写）；源态非 `已立项` / 收到 `--state` / 缺 `--evidence` ⇒ 2（一个字节都不写）", "缺口账 `GAP-20261005-61` · 设计稿 `设计-gap立项正门-20261005.md` §2 乙案 / §7 U1（三段链回退面）· 同族写面先例 `gap set-state`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapUnproject,
		},
		// ★ `gap backfill-unit` —— **存量回填三键**（2026-09-28 · 批1 第二片）：第一片只把 `unit` / `module` /
		//   `unit_source` 落进**新落行**（`gap add`），存量 891 条账里没有 ⇒ 件面 / 模块分类在存量面上**无数据**。
		//   本面：`--dry-run` 只印「可抽到 N + 落兜底 M」（**N + M == 账内总条数**）+ 前 5 条预览（只读 · 恒 0）；
		//   `--yes` 逐条**只加三键**（其余键名与值**逐字节不变** · 行序不变 · 末行换行不变 · 已带三键**跳过** ⇒ 幂等）。
		//   抽取值形态逐字复用第一片的 `gapUnitExtract`；退码照现有表（0 成事 / 2 缺 `--yes` / 8 读不到或写不进）。
		//   实现件 = `family_gap.go:cmdGapBackfillUnit`。
		{
			path:     []string{"gap", "backfill-unit"},
			kind:     "GapBackfillUnit",
			summary:  "给**存量**缺口账回填件面三键（`unit` / `module` / `unit_source`）：`--dry-run` 恒 0 · 只读 · 零副作用 —— 印「**可抽到** N + **落兜底** M」（**N + M == 账内总条数**）+ 前 5 条待改预览（id + unit/module）· `--yes` 逐条**只加三键**（其余键名与值**逐字节不变** · 行序不变 · 末行换行不变）· 已带三键**跳过** ⇒ **幂等**（再跑 ⇒ 「新增 0 行改动」）· 抽取值形态逐字复用 `gapUnitExtract`（甲档全等 / 乙档路径子串 / 兜底手写 `拟(x)` 或 `无件(命令面)` + module `无件` + `unit_source=hand`）· 审计进 `edit_audit.jsonl`（写不进 ⇒ 真源一个字节不改）",
			usage:    "zerg gap backfill-unit [--dry-run | --yes] [--force-recompute] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：范围 = **整本账** · 本片不收窄）"},
			fields:   gapBackfillFields,
			danger:   &dangerSpec{dangerD2, "整本缺口账（`<状态目录>/zerg-cli-gaps.jsonl`）", "逐条**只加** `unit` / `module` / `unit_source` 三键（其余键名与值逐字节不变 · 行序不变 · 末行换行不变）+ 审计一行（可逆：删掉新加的那三键即回原状；`--dry-run` 那一态零副作用）", "批1 第二片（存量 891 条回填 · 2026-09-28）· 同族写面先例 `gap add` / `gap verify` / `gap set-state` / `gap note` · `设计-命令面-gap族-v1.0-20260923.md §三 三态纪律` · `H-10`（`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapBackfillUnit,
		},
		// ★ 批2 第一片（2-1 / 2-2 / 2-4）：候选池独立件 —— `gap idea` 两条（**新件** `<状态目录>/zerg-cli-gap-candidates.jsonl`）。
		//   设计出处：`设计-缺口账与自进化-v2.0-20260928.md` §11.3（独立文件 · 不进主账 · 七格字段 · ttl_days=7）
		//   + §9.1 第 1/2 条（立候选池 · `source` 四值 `guard|gate|egg|human`）+ 施工清单 §3 的 2-1/2-2/2-4。
		//   ★ **`gap add` 仍是唯一入账**：这两条一个字都不写 `zerg-cli-gaps.jsonl`（判据：`idea add` 后账行数不变）。
		//   实现件 = `family_gap.go:cmdGapIdeaLs` / `cmdGapIdeaAdd`。
		{
			path:     []string{"gap", "idea", "ls"},
			kind:     "GapIdeaLs",
			summary:  "候选池**最小读面**（只读 · 不写池、不写真源、不写审计）：列池内候选（`cid` / `source` / `status` / 落池时刻 / 症状）· `--source` 收窄（四值闭集 `guard|gate|egg|human`，闭集外 ⇒ 2 并点名闭集）· `--json` 走同族信封（`meta.total` / `meta.hits` / `query_ts` / `pool_sha16` + 截断自报）· 池件不在盘上 ⇒ 8（「读不到」不当绿）· 零命中 ⇒ 1",
			usage:    "zerg gap idea ls [--source <guard|gate|egg|human>] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：收窄走 `--source`）"},
			fields:   gapIdeaListFields,
			endpoint: "",
			run:      cmdGapIdeaLs,
		},
		{
			path:     []string{"gap", "idea", "add"},
			kind:     "GapIdeaAdd",
			summary:  "人随手记 ⇒ **落候选池**（写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：落一行 `<状态目录>/zerg-cli-gap-candidates.jsonl`（`cid`/`source`/`raw`/`created_at`/`ttl_days=7`/`status=pending`/`fp`）· **真源一字不动**（`gap add` 仍是唯一入账）· 同 source 同 fp ⇒ 幂等 0 · 顺带做**七日归档**（落池超七天的条目搬入同目录 `…archive-<日期>.jsonl` · 只搬超期条）· `--source` 缺省 `human`，闭集外 ⇒ 2 并点名闭集",
			usage:    "zerg gap idea add --symptom <一句> [--source <guard|gate|egg|human>] [--unit <件路径>] [--gate-step <步>] [--log-path <件>] [--rc <N>] [--dry-run | --yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：全部走旗标）"},
			fields:   gapIdeaFields,
			danger:   &dangerSpec{dangerD2, "候选池件（`<状态目录>/zerg-cli-gap-candidates.jsonl`）", "往**候选池**追加一行（可逆：删那一行）+ 顺带把落池超七天的条目搬进同目录归档件；**真源 `zerg-cli-gaps.jsonl` 与审计一个字节都不动**（本面不碰真源 —— 入账仍走 `gap add`）", "设计-缺口账与自进化-v2.0-20260928.md §11.3 + §9.1 第 1/2 条 · 施工清单 `任务清单-缺口账自进化-施工-20260928.md` §3 的 2-1/2-4 · 同族写面先例 `gap add`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaAdd,
		},
		// ★ 批2 第二片（2-3 出口面 · 2026-09-28）：候选池三条出口 + 一条读数（设计稿 §11.3「出口」+ §4 判据 1）。
		//   ★ **`promote` 复用 `gap add` 的既有校验（不许绕）**（§11.3 黑体）⇒ 与 `cmdGapAdd` 共用
		//   `family_gap.go:gapAddApply`（六必填 / impact 闭集 / prio 缺省 / 人面禁写 state / 判据命令树解析 /
		//   同 fp 幂等与 `14` / 审计先落盘 / 读回对拍）—— 实现件 = `cmdGapIdeaPromote`。
		{
			path:     []string{"gap", "idea", "show"},
			kind:     "GapIdeaShow",
			summary:  "候选池**单条全文**（只读 · 一字不截）：cid / 来源 / 状态 / 指纹 / 落池时刻 / 存活 / 点名件 / 症状 / **提升后的 `gap_id`**（`pool` 与 `ledger` 两边点名）· 池内没有这个 cid ⇒ 2（同族 `gap show` 口径）· 池件不在盘 ⇒ 8",
			usage:    "zerg gap idea show <cid> [--json <字段>]",
			arity:    "any",
			args:     []string{"候选 id（**恰好一条** —— 本面只取一条）"},
			fields:   gapIdeaShowFields,
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaShow,
		},
		{
			path:     []string{"gap", "idea", "promote"},
			kind:     "GapIdeaPromote",
			summary:  "池内一条候选 ⇒ **正式账**（写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：**复用 `gap add` 的既有校验与落账路径**（`gapAddApply` —— 六必填 / `impact` 六值闭集 / 人面禁写 `state` / 判据命令树解析 / 同 fp 幂等与 `14` / 审计先落盘 / 读回对拍，一条不绕 · 设计稿 §11.3 黑体）· 只有 `status=pending` 可提升（其余 ⇒ 2 并点名当前 status）· 提升后池行**不删**：`status=promoted` + 记 `gap_id`（先落账、后改池行）",
			usage:    "zerg gap idea promote <cid> --handmade <命令原样> --impact <六值之一> --want-family <族> --want-action <动作> [--want-argv <段>…] [--prio P0|P1|P2] [--state <仍缺|已立项|不做>] --repro-cmd <命令> --verify-cmd <命令> [--note <注>|--notes <注>|--evidence <注>…] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"候选 id（**恰好一条**）；其余六必填与本族 `gap add` 逐字同款（症状取自池件那一格 `raw`）"},
			fields:   gapIdeaPromoteFields,
			danger:   &dangerSpec{dangerD2, "候选 id（池内一条）+ 缺口 fp", "走 `gap add` 那条路往真源（`<状态目录>/zerg-cli-gaps.jsonl`）追加一行 + 审计一行（可逆：删那一行 / 审计历史行不删），并把池件那一行置 `status=promoted` + 记 `gap_id`（池行**不删**）；池件写不进 ⇒ 退 8 并如实报「正式账已落」", "设计-缺口账与自进化-v2.0-20260928.md §11.3（`promote` **复用 `gap add` 的既有校验（不许绕）**）· 批2 第二片（2026-09-28）· 同族写面先例 `gap add`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaPromote,
		},
		{
			path:     []string{"gap", "idea", "discard"},
			kind:     "GapIdeaDiscard",
			summary:  "池内一条候选 ⇒ `status=expired`（写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：**不是删件** —— 池行不删、其余行逐字节不动、真源一个字节不碰 · 只有 `status=pending` 可作废（其余 ⇒ 2 并点名当前 status）",
			usage:    "zerg gap idea discard <cid> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"候选 id（**恰好一条**）"},
			fields:   gapIdeaDiscardFields,
			danger:   &dangerSpec{dangerD2, "候选池件（`<状态目录>/zerg-cli-gap-candidates.jsonl`）点名那一行", "把池内该行 `status` 置 `expired`（可逆：照 `gap idea show` 的读数改回 `pending`）；池行**不删**、真源 `zerg-cli-gaps.jsonl` 与审计**一个字节不动**", "设计-缺口账与自进化-v2.0-20260928.md §11.3（出口面 · `status` 四值 `pending|promoted|expired|merged`）· 批2 第二片（2026-09-28）· 同族写面先例 `gap add` / `gap idea add`", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaDiscard,
		},
		{
			path:     []string{"gap", "idea", "stats"},
			kind:     "GapIdeaStats",
			summary:  "（设计稿 §4 判据 1）读数面（**只报告、不进退码**）：池内各 `status` 计数 + **来源分布**（`source` 四值）+ **候选→入账转换率**（`promoted / (promoted+expired+pending)`，分母逐字写出）· 只读（不写池、不写真源、不写审计）· 池件不在盘 ⇒ 8",
			usage:    "zerg gap idea stats [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：范围 = 整池）"},
			fields:   gapIdeaStatsFields,
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaStats,
		},
		// ★ 批2 第三片（2-3 守卫腿 · 2026-10-13）：守卫拦截**回收面**。口径已拍（缺口
		//   `GAP-20260928-174`）：**落点定 control 层**（`core/internal/control/guardlog.go` 记事件）
		//   · **回收面走 `gap idea sweep --source guard`**（拦截点不内联写池）。实现件 = `cmdGapIdeaSweep`。
		{
			path:     []string{"gap", "idea", "sweep"},
			kind:     "GapIdeaSweep",
			summary:  "把**守卫拦截事件**（控制层 `<状态目录>/zerg-cli-gap-guard-events.jsonl`）回收成候选池条目（写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）：逐事件造一行 `source=guard` / `status=pending`（幂等靠池内 `fp` —— 同 source 同 fp ⇒ 不新增）· `--source` 只认 `guard`（闭集外 ⇒ 2 并点名；gate 腿由门禁脚本直写入池 · egg/human 腿走 `gap idea add`）· 拦截点**不内联写池**（落点见 `core/internal/control/guardlog.go`）· **真源一字不动** · 顺带做七日归档 · 没事件 ⇒ 1（零命中）· 池件写不进 ⇒ 8",
			usage:    "zerg gap idea sweep [--source <guard>] [--dry-run | --yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：全部走旗标）"},
			fields:   gapIdeaSweepFields,
			danger:   &dangerSpec{dangerD2, "候选池件（`<状态目录>/zerg-cli-gap-candidates.jsonl`）", "往**候选池**追加「池里还没有的」守卫拦截候选（可逆：删那些行）+ 顺带把落池超七天的条目搬进同目录归档件；**真源 `zerg-cli-gaps.jsonl` 与守卫事件件一个字节都不动**（本面只读事件件；入账仍走 `gap add`）", "`设计-缺口账与自进化-v2.0-20260928.md` §11.3「三类入口」之二（守卫拦）· 缺口 `GAP-20260928-174`（已拍：落点定 control 层 · 回收面走本命令）· 同族写面先例 `gap idea add`（`H-10`：`--yes` 是命令行确认档，不是批准件）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdGapIdeaSweep,
		},
	}
}
