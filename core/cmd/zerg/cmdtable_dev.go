package main

// 命令表 · dev 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_dev() []*command {
	return []*command{
		{
			path:    []string{"dev", "release"},
			summary: "发布候选件（危险 D3 · 本版未开放）",
			usage:   "zerg dev release --candidate <候选 id> --confirm=<候选 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD3, "候选 id", "把候选件推上生产面（**只能由人拍板开**；AI 不许自升）", "§17.4 · §九 M18 C4 · 开工单 T-58", true, dangerScopeSystem},
			run:     cmdGuarded,
		},
		{
			path:    []string{"dev", "rollback"},
			summary: "回滚（危险 D3 · 本版未开放）",
			usage:   "zerg dev rollback [--to <目标>] --confirm=<候选 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD3, "候选 id", "把生产面退回某个已知状态（回滚件到期前**永不自动**）", "§17.2 ⑦ · §十五.2 档③ · 开工单 T-58", true, dangerScopeSystem},
			run:     cmdGuarded,
		},
		{
			path:    []string{"dev", "proposal"},
			kind:    "Proposal",
			summary: "提案件通道：只产可审查物（new|list|show|check|revise）· 目标必须回指既有编号 · **判据必须可机检** · `check --deep` 真跑每件判据（报 rc 与命中数）· 「提 ≠ 批」两对字段（subject/approver）· 作废档 `--retract/--void <提案 id>`（打作废标记 · 号不复用 · 落审计一行）",
			usage:   "zerg dev proposal new --title <题（必填）> --target <待办编号（必填 · 只收 D/E/F/G 族编号 · 逐条闭集真源 core/internal/contract/dev-targets.json）> --goal <目标（必填）> --evidence <出处（必填 · 至少一条）—— 两类：① **id 形状**（首部 `GAP-…` / `DEV-…`，如 `GAP-YYYYMMDD-NN`）须**在册**（缺口账 / 在册提案件里查得到）—— 查不到 ⇒ 退码 2「出处 … 在缺口账 / 在册件里查不到 ⇒ 不给结论」；② **描述性证据**（无前缀自由文本）直接收，不要求在册> --rollback <退点件（必填 · 仓外整件路径 —— 退建议照它核 sha256 取改前态；路径形的退点（绝对 / 带目录分隔符）在提案这一步就要求**现读可得**（读不到 / 是目录 ⇒ 当场退 2），非路径形的「方式」文本才留到退建议那一步判）> --criterion <判据（必填 · 一条首词是 zerg、本版真跑得动的命令）> [--criterion-new <前瞻判据（可选 · 一条首词是 zerg、本版**还跑不动**的命令形态 ⇒ 显式标前瞻；本版已能真跑的命令写 --criterion）>] [--by <提出者>] [--file <要改的件（仓内相对路径 —— 与 --rollback 收的**仓外**整件路径是两套口径 · 别混）>]… [--subject <提出者>] [--subject-kind human|ai|egg|ci] [--egg-id <卵 id>] [--approver <批准者>] [--approver-kind human] [--batch <件.json（可机读批量档 · 一次建多枚 · 逐格走单片档同一条闸 · 与单片旗标互斥）>] [--from-file <件.json（单片档 · 读一枚 JSON 对象 = 一片提案的字段 · 与单片旗标互斥）>] [--add-file <路径（仓内相对 · 可重复 · 补进既有提案的 files[]）>…（revise 补件） [--retract <提案 id> | --void <提案 id> --reason <理由>（作废档：打作废标记 · 号不复用 · 落审计一行）] [--json <字段>] [--dry-run | --yes] —— ★ 自由文本字段（题/目标/出处/判据/退点）含反引号或 `$` 时：请用**单引号**包裹、或改走 --from-file <件>；**双引号会被 shell 先展开**（丢字符/误执行）",
			arity:   "any",
			args:    []string{"动作：new | list | show | check | revise", "提案 id（show/check/revise 才要 · `--retract`/`--void` 也点名它）"},
			fields:  proposalFields,
			// 授权侧（`GAP-20260927-140` ①）：`new` 真写提案件（状态目录）而修前无 `danger`。
			// 声明面照 `agent registry` 先例：**整条命令**登记危险档，`list`/`show`/`check` 只读
			// （fail-closed 只落在 `new` 的写盘点上 —— 见 `cmdDevProposalNew` 的 `--yes` 门）。
			danger:   &dangerSpec{dangerD2, "（提案 id）", "落一件提案件（状态目录）—— `new` 写、`list`/`show`/`check` 只读", "GAP-20260927-140 ① · GAP-20260927-90 · 同族先例 agent registry（声明面 · `--list` 只读）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdDevProposal,
		},
		// ---- D3b 第二步（2026-09-21）：**受控写面**（`zerg dev edit` · §17.3 铁律③）----
		// 默认干跑 · 只改提案声明过的件（越界写 ⇒ 2）· 逐条审计（写不进审计就不改件）· D3 档确认。
		{
			path:    []string{"dev", "edit"},
			kind:    "DevEdit",
			summary: "受控写入：只改**提案声明过**的件（越界写 ⇒ 2）· 默认干跑 · 一行一事件的审计（写不进审计就不改件）",
			usage:   "zerg dev edit --proposal <提案 id> --file <仓内相对路径> (--from <件> | --replace <件：补丁 JSON 数组 [{\"old\":…,\"new\":…}] —— 逐格精确替换 · 每格 old 必须在目标件里**恰好命中 1 次**（0 次或多次 ⇒ 拒执 2）> | --replace-inline <同一形态的内联 JSON 串 —— **直给，不必先落件**（缺口 GAP-20261018-22） | --replace-inline-b64 <同一 JSON 的 base64 形态 —— 绕开对话层字符扫描，缺口 GAP-20261018-24>) [--base <件路径|HEAD|sha>] [--by <谁>] [--allow-cross-root <理由>] [--allow-sibling-repo <名>] [--literal-text] [--json <字段>] [--dry-run | --confirm=<本机名> --yes] （真写前置：一枚**人签批准件** <状态目录>/approvals/dev_edit.json —— 由 `zerg approve new --tool dev_edit` 签出 · `zerg approve ls` 可查现状；无件 / 手写件 / 它的 scope 不含本件 ⇒ 一律拒（退 2）；--dry-run 那一态不需要它；`--base` 只与 `--replace` 合用：用**基线内容**当匹配基（`HEAD`=该件的已提交版 · `sha`=某提交版 · 其它=当作仓外前态件路径读），并把基线作为审计的 before ⇒ 已改盘的件可**回放**；回放结果必须与现盘件逐字节相同、否则拒执 2）" + devEditDryRunOnlyNote,
			arity:   "any",
			args:    []string{"提案 id（--proposal）", "要改的件（--file · 必须在提案的 files[] 里）"},
			fields:  devEditFields,
			danger:  &dangerSpec{dangerD3, "本机名", "改仓内件（写工作树）—— 作用域 = 提案声明的件；审计一行一事件；回滚 = 提案退点件（回到 before_sha256；`git revert` 不算退点）", "§17.3 铁律③ · §九 M3 C4/C5 · §4.1 K7 · D3b 第二步", true, dangerScopeRepo},
			// `opened`: 真跑已开放（`--confirm=<本机名> --yes` 齐 + 人签批准件 ⇒ 写工作树）。
			opened:   true,
			endpoint: "",
			run:      cmdDevEdit,
		},
		// ---- GAP-20261010-30：**直写补正门**（`zerg dev reclaim`）----
		// 口径：真前态 = 该件审计末行 `after` → 回盘造前态（`--pre` / `git HEAD`；无审计行 ⇒ 空前态）
		// → 调**既有** `dev edit --from` 净零重落 · 核 sha 逐字同 ＋ 断言审计新增一行。✗ 不放宽任何闸。
		{
			path:     []string{"dev", "reclaim"},
			kind:     "DevReclaim",
			summary:  "**直写补正门**：给被非正门直写的件补审计链（真前态 = 审计末行 after 回盘 → 调既有 `dev edit --from` 净零重落 · 核 sha 逐字同 · 盘上内容逐字节不变）",
			usage:    "zerg dev reclaim <仓内相对件>… --proposal <提案 id> [--pre <件>=<仓外前态件>]… [--by <谁>] [--json <字段>] [--dry-run | --confirm=<本机名> --yes]",
			arity:    "any",
			args:     []string{"要补正的件（仓内相对路径 · 可多给）"},
			fields:   reclaimFields,
			danger:   &dangerSpec{dangerD3, "本机名", "补正审计链：回盘真前态后调 `dev edit --from` 净零重落（盘上内容逐字节不变 · 只补审计链）；真前态取不到 ⇒ 拒", "GAP-20261010-30 · 同族先例 dev edit（D3）", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdDevReclaim,
		},
		// ---- R3 · 缺口 `GAP-20260927-377`（P0）· `dev check --syntax`（类型面只读正门）----
		// 实现件已落（`family_dev_check.go`：`cmdDevCheck` + `devCheckFields`）⇒ 本处只做命令树登记。
		// 只读（零制品 · 制品丢 /dev/null · 不进危险档清单）；无 `--dry-run` ⇒ 干跑语义那一面不适用。
		// ★ 新命令 ⇒ 同批随动：cli-matrix must-fail 格 ＋ cli-contract-baseline ＋ 门69 两半 ＋ 旗标名表。
		{
			path:    []string{"dev", "check"},
			kind:    "DevCheck",
			summary: "类型面只读正门：真跑一次编译/go vet（制品丢 /dev/null · 零制品）—— 施工卵禁跑 go build|vet 时的编译器背书（GAP-20260927-377）",
			usage:   "zerg dev check --syntax [--file <仓内相对 .go 件>]… | [--pkg <包>]… [--vet] [--json <字段>]",
			arity:   "none",
			args:    []string{"（不收位置参数 —— 目标一律走 `--file` / `--pkg`）"},
			fields:  devCheckFields,
			run:     cmdDevCheck,
		},
		// ---- §17.4 第 2/3/5 条：候选区（批 E · T-58 · 真源 core/internal/contract/dev-candidate.json）----
		// 次序（§17.7 逐字「先有判据、再有自动化」）落成机检：build / test 找不到该候选的验收证据单 ⇒ 退码 2。
		{
			path:    []string{"dev", "build"},
			summary: "在**候选区**构建全套件（危险 D2 · 本版未开放；**判据先于自动化**）",
			usage:   "zerg dev build --candidate <候选 id> [--scope go|rust|ui|all] [--dist] [--yes] [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD2, "候选 id", "在候选区编出成套制品（底层就是 Go 真身 scripts/build/cmd/zerg-build-all/main.go —— 不新造第二条构建路）", "§17.4 第 2 条 · §17.6 SD10-b · 开工单 T-58", true, dangerScopeRepo},
			run:     cmdDevBuild,
		},
		{
			path:    []string{"dev", "test"},
			summary: "跑候选件的测试集（危险 D2 · 本版未开放；**判据先于自动化**）",
			usage:   "zerg dev test [--candidate <候选 id> | --pkg <包> [--run <正则>]] [--scope go|rust|ui|all] [--outdir D] [--yes] [--dry-run]",
			arity:   "any",
			args:    []string{"候选 id"},
			danger:  &dangerSpec{dangerD2, "候选 id", "在候选区跑测试集（底层 = make test + 门禁既有步，不新立判据）", "§17.4 第 3 条 · §17.7 次序 · 开工单 T-58", true, dangerScopeRepo},
			run:     cmdDevTest,
		},
		{
			path:    []string{"dev", "verify"},
			kind:    "DevEvidence",
			summary: "合成一份验收证据单（只收证据、**不给「通过」的结论**）· 证据为空 ⇒ 2",
			usage:   "zerg dev verify --candidate <候选 id> [--results <结果表>] [--code-sha <sha>] [--node <名>] [--layer <档>] [--gate [--human-approval <名>]] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"候选 id"},
			fields:  devVerifyFields,
			// 授权侧（`GAP-20260927-140` ①）：真写 `dist/candidates/<id>/evidence.json`（判据物 ·
			// **仓内工作树**）而修前无 `danger` ⇒ 默认档零授权写。fail-closed 落在写盘点（见 `cmdDevVerify`）。
			danger:   &dangerSpec{dangerD2, "候选 id", "写 `dist/candidates/<id>/evidence.json`（判据物 · 仓内候选区）", "GAP-20260927-140 ① · GAP-20260927-90 · dev verify 是零授权写里唯一落仓内工作树者", false, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdDevVerify,
		},
		// ---- §20.3 H3 交接回执（批 E · T-61）----
		// 轮级续做件（`P-119` 的取舍：变更账是版本级史书、回执是轮级续做件，不许混）。
		{
			path:    []string{"dev", "receipt"},
			kind:    "Receipt",
			summary: "交接回执（一轮一页 · 带 trace_id）：new | ls | show；读法 = `zerg context ls --resume`（`new` 写要 `--yes`）",
			usage:   "zerg dev receipt new --what <做了什么> --next <下一步> [--evidence <证据>]… [--blockers <阻碍>]… [--commit <sha>]… [--trace <trace_id>] [--json <字段>] [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"动作：new | ls | show", "轮次 id（只 show 要）"},
			fields:  receiptFields,
			// 授权侧（`GAP-20260927-140` ①）：`new` 真写回执件（状态目录 `receipts/`）而修前无 `danger`。
			// 声明面照 `agent registry` 先例：**整条命令**登记危险档，`ls`/`show` 只读
			// （fail-closed 只落在 `new` 的写盘点上 —— 见 `cmdReceiptNew` 的 `--yes` 门）。
			danger:   &dangerSpec{dangerD2, "（轮次 id）", "落一件回执（状态目录 `receipts/<R-…>.json`）—— `new` 写、`ls`/`show` 只读", "GAP-20260927-140 ① · GAP-20260927-90 · 同族先例 agent registry（声明面）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdDevReceipt,
		},
	}
}
