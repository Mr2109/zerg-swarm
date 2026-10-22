package main

// 命令表 · deploy 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_deploy() []*command {
	return []*command{
		// ---- ② 类 CLI 工具子命令（命令树登记 · 2026-10-08）────────────────────────────────
		// 三条 CLI 工具子命令（✗ 不是判据 ⇒ ✗ 不进 `gate_inproc_table.go`，那会伪造判据名）；
		// 形状照同族先例（`passthrough: true` + `run` 直调实现件）。
		// ★ 三条均已**真搬**（行为落 Go · ✗ 不再 exec 底层脚本）：`deploy_launch.go` / `doc_site_build.go`
		//   / `ui_start.go` —— 底层脚本退役（移出仓）后两态三面逐字同（✗ 不再退 `8`）。
		//   ★ `deploy launch` 含**意图件落盘** ⇒ 是**写面** ⇒ 登记 `danger` + `opened`（fail-closed）。
		// ---- A 档 批 4（机群/发布面）· A13：机群发布的 **CLI 正门**（`zerg deploy launch` · 新族 `deploy` · 真搬）----
		// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 4 · A13。
		// 底层原件 = `deploy/scripts/deploy-launch.sh`（真跑 `ssh` 到节点跑发布器 · OS 原生/联网面）。
		// ★ 真搬后本件自足（✗ 不再 exec 脚本 · ✗ 无「递不到脚本退 8」档）：旗标 / 退码 / 文案逐档照原件。
		// 写面（意图件落盘 `~/.zerg/deploy_launch/`）⇒ 默认档 fail-closed（缺 `--yes` 退 2 · 零写）。
		{
			path:        []string{"deploy", "launch"},
			danger:      &dangerSpec{dangerD3, "发布目标主机", "真发布到节点（`ssh` 到节点跑发布器 · 会写主控侧意图件并触发节点侧发布 —— 不可逆动作的邻居）", "GAP-20260927-140 ① · A档批4 A13（真搬后成写面 · 门100 写面登记）", false, dangerScopeRemote},
			opened:      true,
			summary:     "机群发布「意图件 + 回读」（真搬 · 行为落 Go —— 默认档 fail-closed：缺 --yes 退 2 零写）",
			usage:       "zerg deploy launch [--host <主机> --file <意图件> --expect-sha256 <sha> | --help] [--dry-run | --yes]",
			arity:       "any",
			args:        []string{"旗标与退码逐档照 deploy/scripts/deploy-launch.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdDeployLaunch,
		},
	}
}
