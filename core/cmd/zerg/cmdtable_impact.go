package main

// 命令表 · impact 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_impact() []*command {
	return []*command{
		// ---- 变更影响面（设计-变更影响面-v1.6 §7.1/§7.3/§7.4 · 任务单-影响面实施-20260922 §二 `A1`–`A3` · 2026-09-22）----
		// `A1` = 骨架（人面三行 + 六键包封）；`A2` = 六层取数；`A3` = 波纹卡片（四字段 · why 闭集六选一 ·
		// 四级排序 · 按档裁 · §3.7 公开面行 · §3.8 可逆性行 · 两档 `--for-model`/`--for-human`）。
		// 挂干跑与缓存分别属 `A4`/`A5`。不改 `emitEnvelope`、不落审计。
		// ── 2026-09-29 订正：本条**不是**只读 —— 默认档真写 impact 缓存件（现读写点
		//   `family_impact_cache.go:291/:305/:308`，账 `GAP-20260927-140` 现读坐实）；修前那句
		//   「只读 ⇒ 不写 danger / 不写缓存」是**自证式反证断言**（件里写着不写、实现却写）。
		//   定档与执行门**待拍板**（账 `GAP-20260929-31`：登记 ≠ 拦 —— 实测不给 `--yes` 也 rc=0 且真写），
		//   ★ 2026-10-08：`danger` 已挂（D2 · opened）—— Mr2109 拍「默认档要 fail-closed」（账
		//   `GAP-20260927-140` ①）。fail-closed 落在**缓存写盘点**（见 `cmdImpact`）。
		{
			path:    []string{"impact"},
			kind:    "Impact",
			summary: "改一处会牵动谁（判定只读；默认档写 impact 缓存件要 `--yes` ⇒人面三行 + 波纹卡片 ≤12 条/≤1.2k token + 六键包封）",
			usage:   impactUsageLine,
			arity:   "any",
			args:    []string{"目标（仓内件路径 · 或在册契约 id，如 S-g）"},
			fields:  impactFields,
			// 授权侧（`GAP-20260927-140` ①）：默认档 `impactCacheOn` 真写 impact 缓存件（写点现读 =
			// `impactCacheWritePoints` · owner = `impactCacheStore`）而修前无 `danger`。
			danger:   &dangerSpec{dangerD2, "（目标）", "默认档写 impact 缓存件（状态目录 `impact-cache/<sha>/`）", "GAP-20260927-140 ① · GAP-20260927-90 · family_impact_cache.go 写点登记表", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdImpact,
		},
	}
}
