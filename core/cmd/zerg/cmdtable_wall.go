package main

// 命令表 · wall 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_wall() []*command {
	return []*command{
		// ---- L1 批2（2026-10-08）：`zerg wall` 真动作分派（茧壁证据 2 · 壳转账）＋ 茧壁变异 2（危险档 · 未接）----
		// 依据：`~/.zerg/state/沙登茧壁组-接线预扫-20261008.tsv` 批2（茧壁证据 ×2 ＋ 茧壁变异 ×2）·
		// `待拍清单-AlltoGo-20261008.md` P3（`wall` 撞名 ⇒ 帮助主题已改名 `enclosure` 腾出本族名）·
		// P5（两个变异器**真改仓内件** ⇒ 入危险档 D2/写仓 ＋ 还原台账纪律）。
		// 形态（两条路，都照仓里既有先例、不自己创式样）：
		//   · `wall evidence <two-states|macos>` = **壳转账**（底层唯一实现 = `scripts/wall/<名>.py` ·
		//     真跑 cargo+verify ⇒ 非纯 Go 可化）：命令面只做入口 ＋ 转发，✗ 不重写脚本任何一段。
		//     实现体宿主 = `ui_start.go`（门78 面④ 要求实现件已在 `scripts/公开标记.tsv` ⇒ 复用**已登记**件
		//     作宿主 · 本单铁律禁碰该表 —— 同 `cmdSandboxProbe` 那条理由）。
		//   · `wall mutate <bridge|macos>` = **危险档 D2（写仓）· 本版已开放**：两个变异器**真改仓内件**
		//     （改坏 Rust/Go ⇒ 门必须红 ⇒ 还原）⇒ 依 P5 入危险档，✗ **不得默认裸跑** —— `run: cmdWallMutate`
		//     （`wall_mutate.go` · **壳转账**：`--dry-run` 出计划件退 0 / 缺 --yes fail-closed 退 2 / 确认档齐
		//     转发 `scripts/wall/{wall-bridge,wall-macos}-mutate.py` 真跑 · 脚本退码原样转出）。★ **还原台账
		//     纪律**（跑前落备份 · 跑后逐件核 sha · 回执报「变异期间碰过哪些件、怎么复原」）随
		//     `dangerSpec.Effect` 逐字登记，由脚本内 `scripts/docs/inplace-window.py` 落地。
		{
			path:        []string{"wall", "evidence"},
			kind:        "WallEvidence",
			layer:       "host",
			summary:     "茧壁活体回执生成器（two-states / macos · 底层只调 scripts/wall/<名>.py —— 命令面不重写逻辑 · 真跑 cargo+verify）",
			usage:       "zerg wall evidence <two-states|macos> [<透传旗标…>]",
			arity:       "any",
			args:        []string{"子动作名（逐字 · 见用法串的闭集）", "旗标与退码逐字来自 scripts/wall/<名>.py（原样透传 · 0 全绿 / 1 断言红 / 2 硬失败）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdWallEvidence,
		},
		{
			path:    []string{"wall", "mutate"},
			kind:    "WallMutate",
			layer:   "host",
			danger:  &dangerSpec{dangerD2, "变异目标（bridge / macos）", "真改仓内件做变异验证（改坏 wall/src/**.rs 与迁移桥两侧 ⇒ 门必须红 ⇒ 还原）；**还原台账纪律**：跑前落备份（`scripts/docs/inplace-window.py` arm）、跑后逐件核 sha、回执报「变异期间碰过哪些件、怎么复原」", "待拍清单 P5（茧壁变异器入危险档 D2/写仓）· 同族先例 gate check（D3）· 两个变异器件头（inplace-window 复原口径）", false, dangerScopeRepo},
			opened:  true,
			summary: "茧壁变异验证（bridge / macos · **危险档 D2/写仓 · 本版已开放**：真改仓内件 ⇒ 缺 --yes fail-closed 退 2；`--dry-run` 只出计划件）",
			usage:   "zerg wall mutate <bridge|macos> [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"子动作名（逐字 · bridge / macos）", "确认档：`--dry-run` 出计划件 / `--yes` 才执行（缺 ⇒ fail-closed 退 2）"},
			run:     cmdWallMutate,
		},
	}
}
