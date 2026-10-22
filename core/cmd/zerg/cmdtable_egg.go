package main

// 命令表 · egg 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_egg() []*command {
	return []*command{
		// ---- 批 D · T-47 虫卵 / 虫茧两族（I / J 族 · §3.4 · §7.1 P6 · N4 硬占位表）----
		{
			path:     []string{"egg", "ls"},
			kind:     "Egg",
			summary:  "卵 × 设备矩阵（只读投影：host / model / state 三格 · 照现成端点包装）",
			usage:    "zerg egg ls [--json <字段>]",
			fields:   eggFields,
			endpoint: "GET /api/fleet/models + GET /api/fleet/status + GET /api/models/{name}（现成端点包装 · 不新开一条路）",
			run:      cmdEggLs,
			// ★ `GAP-20260925-15`（Face A · 2026-10-10）：看门狗读数两格（`last_output_ago_s` / `watchdog_verdict`）
			//   走**可选字段层**（`optFields`）—— 登在出口白名单里、`--json` 点得出来，但**不进** `eggFields`
			//   那一份**冻结**的帮助 / 文档导出面字段表 ⇒ 无需重生成命令页（与 `code find` 的 `count` 同一体例）。
			optFields: eggOptionalFields,
		},
		{
			path:     []string{"egg", "show"},
			kind:     "Egg",
			summary:  "单枚卵的现状（只读投影：host / model / state 三格 · 同一份真源）",
			usage:    "zerg egg show <卵 id> [--json <字段>]",
			arity:    "any",
			args:     []string{"卵 id（<模型 id>@<主机>，照 `egg ls` 的 egg_id 那一格）"},
			fields:   eggFields,
			endpoint: "GET /api/fleet/models + GET /api/fleet/status + GET /api/models/{name}（现成端点包装 · 不新开一条路）",
			run:      cmdEggShow,
			// ★ `GAP-20260925-15`（Face A · 2026-10-10）：同 `egg ls` —— 看门狗读数两格走**可选字段层**
			//   （`optFields`：登在出口白名单、`--json` 点得出来，但不进冻结帮助 / 文档导出面字段表）。
			optFields: eggOptionalFields,
		},
		// ★ 缺口 `GAP-20261018-13`（第二层：体检正门）：卵的**三面对账**一条命令看全 ——
		//   声明态（主控 `/api/models/{name}`）⟷ 进程态/端口态（子端 `/eggs`，含驻留账本的
		//   `residency[]`）⟷ GTT 占用 ＋ 上次装载时刻，并**逐项点名**不一致。
		// 退码 = 0 全一致 / 1 有不一致 / 8 判不了（件头 `family_egg_doctor.go` 的「退码约定」）；
		// 只用既有端点、只 GET ⇒ 非危险档（`danger == nil`）、无干跑档（本命令零写面）。
		{
			path:    []string{"egg", "doctor"},
			kind:    "EggDoctor",
			summary: "卵体检（只读三面对账：声明态 / 进程态 / 端口态 / GTT / 上次装载时刻 ＋ 不一致项逐条点名 · 退码 0 一致 / 1 不一致 / 8 判不了）",
			usage:   "zerg egg doctor [--machine <机>] [--json <字段>]",
			arity:   "any",
			args:    []string{"机器（`--machine`，可省：缺省体检全机群）"},
			fields:  eggDoctorFields,
			endpoint: "GET /api/fleet/models + GET /api/fleet/status + GET /api/models/{name} ＋ 子端 GET /eggs" +
				"（现成端点包装 · 不新开一条路 · 只读）",
			run: cmdEggDoctor,
		},
		{
			path:     []string{"egg", "run"},
			danger:   &dangerSpec{dangerD2, "卵 id", "把该卵的模型**装到**那台机上（控制面 `POST /api/control/load` · 占内存/显存槽 · 单槽机串行）—— 可逆：`egg stop` 即回原态", "GAP-20261005-38 · 同族先例 model start / agent load（D2）", false, dangerScopeRemote},
			opened:   true,
			kind:     "EggRun",
			summary:  "把卵的模型**装到**那台机上（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/control/load`）",
			usage:    "zerg egg run <卵 id> [--machine <机>] [--timeout <时长>] [--wait | --no-wait] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"卵 id（`<模型 id>@<主机>`，照 `egg ls` 的 egg_id 那一格）", "机器（`--machine`，可省：缺省用卵档案里那台机）"},
			fields:   eggActFields,
			endpoint: "POST /api/control/load（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdEggRun,
		},
		{
			path:     []string{"egg", "stop"},
			danger:   &dangerSpec{dangerD2, "卵 id", "把那台机上的该模型**卸掉**（控制面 `POST /api/control/unload` · **在跑任务受影响**）", "GAP-20261005-38 · 同族先例 model stop / agent unload", false, dangerScopeRemote},
			opened:   true,
			kind:     "EggStop",
			summary:  "卸载一枚卵 —— 把那台机上的该模型**卸掉**（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/control/unload`）",
			usage:    "zerg egg stop <卵 id> [--machine <机>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"卵 id（`<模型 id>@<主机>`，照 `egg ls` 的 egg_id 那一格）", "机器（`--machine`，可省：缺省用卵档案里那台机）"},
			fields:   eggActFields,
			endpoint: "POST /api/control/unload（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdEggStop,
		},
		// ---- 卵开工 · 卵7①（2026-10-15）：卵族**具名入口** —— `zerg egg <info|verify|gc|expand|release>` ----
		// 落件 = `core/cmd/zerg/family_egg.go`（薄壳 · 逐字转发已落能力 —— ✗ 命令层不重写逻辑）：
		//   · `verify` → `zerg_egg_verify.go` 的 `zegvVerifyEntry`（逐段并行 sha256 ＋ 内容寻址校验缓存 ·
		//     卵3① 已落 —— 此前只有 passthrough 的 `cocoon verify` 打得到）；
		//   · `info` / `gc` / `expand` / `release` → `core/internal/store/eggstore`（名册 / 引用图 / GC /
		//     CoW 展开 / 解引用 · 卵2 已落 —— 该包件头自注逐字「本包纯库件、零 CLI 接线」⇒ 命令面此前打不着）。
		// ★ 卵库根 = `--root <目录>`（**已在旗标名字表上** ⇒ ✗ 不新开旗标；缺省 `~/.zerg/eggs`）。
		// ★ 危险档照同族先例 `core db`（`family_core_db.go`）：`gc` = D3（真删不可逆 ⇒ `--confirm=<卵库根>`
		//   与 `--yes` **同时到**）· `expand` / `release` = D2（`--yes`）· 三条各有 `--dry-run` 计划件（零副作用）。
		// ★ 族名口径（延续 main.go 里既有那条自注）：`egg` **既有**的 `ls`/`show`/`run`/`stop`/`pin`/`unpin`
		//   是【虫卵（沙箱 / 容器）投影面】（远端只读投影 ＋ 控制面写面），与本次这五条**不同对象** ——
		//   本批只**增**五条（✗ 不动既有任何一条的用法串与退码）。`ls` 那一格**已被占用**
		//   ⇒ 卵库名册清单落 `zerg egg info` 的无参档（名册态），✗ 不新造同路径第二条。
		{
			path:    []string{"egg", "info"},
			kind:    "EggInfo",
			summary: "卵库档案 / 名册态（只读：给卵名 ⇒ 名册行 ＋ 引用格；不给 ⇒ 全册清单 —— 转发 `eggstore` 的名册与引用图）",
			usage:   "zerg egg info [<卵名>] [--root <卵库根>] [--json <字段>]",
			arity:   "any",
			args:    []string{"卵名（可省 ⇒ 报卵库名册态）"},
			fields:  []string{"action", "root", "root_source", "egg", "sha256", "state", "path", "bytes", "refs", "complete", "blocks", "added_at", "updated_at", "count", "verdict"},
			run:     cmdEggInfo,
		},
		{
			path:    []string{"egg", "verify"},
			kind:    "EggVerify",
			summary: "校验一枚卵包（逐段并行 sha256 ＋ 内容寻址校验缓存 —— 转发 `zerg_egg_verify.go` 的 `zegvVerifyEntry` · ✗ 命令层不重写判据）",
			usage:   "zerg egg verify <卵包.egg> [--json <字段>]",
			arity:   "any",
			args:    []string{"卵包 .egg 件"},
			fields:  []string{"action", "path", "size_bytes", "segments", "data_bytes", "align", "cache_hit", "problem_count", "problems", "verdict"},
			run:     cmdEggVerify,
		},
		{
			path:    []string{"egg", "gc"},
			danger:  &dangerSpec{dangerD3, "卵库根", "真删卵件（**只在引用计数归零且态可回收时**）—— 删前 `--dry-run` 出计划件；件被删即不可逆（须重取）", "设计 §16.2 · §13.1 · §14.4 M1 · 任务单 卵2② / 卵7① · 同族先例 `core db`（D3 ＋ `--confirm`）", true, dangerScopeOutRepo},
			opened:  true,
			kind:    "EggGc",
			summary: "卵库回收（引用计数归零才真删 · 报「已释放」只认 df；D3：`--dry-run` 零副作用 · 真跑要 `--confirm=<卵库根>` ＋ `--yes`）",
			usage:   "zerg egg gc [--root <卵库根>] [--dry-run | --confirm=<卵库根> --yes] [--json <字段>]",
			arity:   "any",
			fields:  []string{"action", "root", "root_source", "dry_run", "deleted", "deleted_count", "pinned", "pinned_count", "df_before", "df_after", "freed_bytes_df", "freed_source", "nominal_bytes_not_freed", "nominal_note", "note", "verdict"},
			run:     cmdEggGC,
		},
		{
			path:    []string{"egg", "expand"},
			danger:  &dangerSpec{dangerD2, "卵名", "展开一枚卵到落点（CoW 整文件 clone ＋ 登记同一条引用 ⇒ 卵的引用计数 ＋1）—— 可逆：`egg release` 删件并解引用即回原态", "设计 §13.1 · §16.2 · 任务单 卵2② · 同族先例 `core db backup`（D2 ＋ `--yes`）", false, dangerScopeOutRepo},
			opened:  true,
			kind:    "EggExpand",
			summary: "展开一枚卵（先登记引用 → 再 CoW 整文件落件 · 展开物纳入**同一**引用计数；D2 写面：`--dry-run` 出计划件 · 真跑要 `--yes`）",
			usage:   "zerg egg expand <卵名> <引用者 id> <落点> [--kind expansion|clone] [--root <卵库根>] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"卵名", "引用者 id", "落点（展开物 / 副本的落盘路径）"},
			fields:  []string{"action", "root", "root_source", "egg", "ref", "kind", "dst", "tier", "cow", "bytes", "refs_after", "refs_now", "dur_ms", "fell_back", "dry_run", "dst_exists", "egg_known", "egg_state", "egg_complete", "egg_src", "note", "verdict"},
			run:     cmdEggExpand,
		},
		{
			path:    []string{"egg", "release"},
			danger:  &dangerSpec{dangerD2, "卵名", "删掉展开物 / 副本件并解掉那一条引用（`eggstore.ReleaseExpansion` · 先删件 → 再解引用）—— 可逆：`egg expand` 重新展开即回原态", "设计 §16.2 · §14.4 M1 · 任务单 卵2② · 同族先例 `core db prune`（D2 ＋ `--yes`）", false, dangerScopeOutRepo},
			opened:  true,
			kind:    "EggRelease",
			summary: "删前解引用（删副本件 ＋ 解引用 · **只有归零后才允许 `egg gc` 真删**；D2 写面：`--dry-run` 出计划件 · 真跑要 `--yes`）",
			usage:   "zerg egg release <卵名> <引用者 id> [<副本路径>] [--root <卵库根>] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"卵名", "引用者 id", "副本路径（可省 ⇒ 只解引用、不删件）"},
			fields:  []string{"action", "root", "root_source", "egg", "ref", "path", "released", "refs_left", "refs_now", "zero", "df_before", "df_after", "freed_bytes_df", "freed_source", "nominal_note", "path_exists", "dry_run", "note", "verdict"},
			run:     cmdEggRelease,
		},
		// ---- F52（2026-10-15）：`egg sign` **具名入口** —— 把卵5 已落的签名能力接进命令树 ----
		// 账面出处：`GAP-20261015-27` 的 `want_family=egg` / `want_action=sign`（族名与动作名照账面**逐字**）。
		// 真修件 = `core/cmd/zerg/egg_sign.go`（卵5 · 提案 `DEV-2958` 已落提交面）；命令面壳 = 该件 `cmdEggSign`。
		// ★ 判据一字不落：argv 组装 / 前置探针 / 自检面全在 `egg_sign.go`（本笔**只接线** —— ✗ 不改那件任何判据语义）。
		// ★ 退码闭集（照卵5 口径 · ✗ 不新增裸数字）：0 真跑签成 / 1 真跑了但未成（判决只看退码）/
		//   2 不给结论（工具缺失 / 版本不符 / 子进程起不来 / 用法错 —— 本库无 cosign ⇒ 恒走这一档）。
		// ★ ✗ 不新增旗标（usage 只列既有的 `--self-test`（全局布尔 · 谁用谁读）与 `--json` ⇒ 门73 旗标名表零改动 ·
		//   `usageDeclaresFlag` 无新增落点）。位置参数 1–3 枚（照同族 `egg expand` / `egg release` 的多位置参数先例）。
		{
			path:    []string{"egg", "sign"},
			kind:    "EggSign",
			summary: "为卵签名（cosign keyed · **调用形态真跑**：工具面前置探针 → argv 组装 → 转发 cosign）· 工具缺失 / 版本不符 ⇒ 退 2 不给结论（本库未装 cosign ⇒ 恒这一档）· 退码 0 签成 / 1 未成 / 2 不给结论 · `--self-test` 跑卵5 四态＋负控（9 项）",
			usage:   "zerg egg sign <卵包.egg> [<子钥件> [<bundle 件>]] [--self-test] [--json <字段>]",
			arity:   "any",
			args:    []string{"卵包 .egg 件（必给）", "子钥件（cosign 的 key 取值）", "bundle 落件（cosign 的 bundle 取值）"},
			fields:  []string{"egg", "key", "bundle", "argv", "rc", "verdict"},
			run:     cmdEggSign,
		},
		{
			path:    []string{"egg", "pin"},
			summary: "钉住一枚卵（危险 D2 · 本版未开放）",
			usage:   "zerg egg pin <卵 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD2, "卵 id", "把该卵标成在孵（**同一时刻至多一枚**，会挤掉别的）", "§3.4 I 族 · 开工单 T-47", true, dangerScopeRemote},
			run:     cmdGuarded,
		},
		{
			path:    []string{"egg", "unpin"},
			summary: "解钉一枚卵（危险 D2 · 本版未开放）",
			usage:   "zerg egg unpin <卵 id> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"卵 id"},
			danger:  &dangerSpec{dangerD2, "卵 id", "取消在孵标记（原本占有单槽的卵会被换下）", "§3.4 I 族 · 开工单 T-47", true, dangerScopeRemote},
			run:     cmdGuarded,
		},
	}
}
