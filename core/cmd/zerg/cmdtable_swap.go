package main

// 命令表 · swap 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_swap() []*command {
	return []*command{
		// ---- A 档 批 1（换件面）· A1：主控换件的 **CLI 正门**（`zerg swap core` · 新族 `swap` · 真搬）----
		// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A1（★事故面：把最险的手工换件口收进受控正门）。
		// 底层原件 = `scripts/build/zerg-swap-core.sh`（2026-09-16 事故后固化：三道门禁 ＋ 两道前置闸；
		// 真换生产 `bin/zerg-core` 并重启 launchd ⇒ **动生产 `bin/` ⇒ 冻结面**）。
		// ★ 真搬后本件自足（✗ 不再 exec 脚本 · ✗ 无「递不到脚本退 8」档）：旗标 / 退码 / 文案逐档照原件。
		// ★ 安全线：本族动作真跑会动生产 `bin/` ⇒ ✗ 不在本单真跑（验收只用 `--help`/未知参数/`--self-test-rollback`）。
		{
			path:        []string{"swap", "core"},
			danger:      &dangerSpec{dangerD3, "换件目标", "真换生产件（会动生产 `bin/` 并重启 launchd / UI 进程 —— 不可逆动作的邻居）—— 底层脚本当前**只报告档**（真换件 `--swap` 一律 rc=2 零副作用）", "GAP-20261005-39 · 同族先例 core restart / agent restart（D3）· A档批1 换件面（A2/A3 回执 §四 待 S6 补）", false, dangerScopeSystem},
			opened:      true,
			summary:     "主控换件（★事故面 · 真搬 · 行为落 Go —— 不再 exec scripts/build/zerg-swap-core.sh）",
			usage:       "zerg swap core [--help | --self-test-rollback] [--dry-run | --yes]",
			arity:       "any",
			args:        []string{"旗标与退码逐档照 scripts/build/zerg-swap-core.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdSwapCore,
		},
		// ---- A 档 批 1（换件面）· A2：子端换件的 **CLI 正门**（`zerg swap agent` · 族 `swap` 扩 action · 真搬）----
		// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A2。
		// 底层原件 = `scripts/build/zerg-swap-agent.sh`（只报告档：真跑 `git status`/`codesign`/`launchctl`/`curl`
		// 探针；真换件档未开放 ⇒ `--swap` 一律 rc=2 零副作用）。
		// ★ 真搬后本件自足（✗ 不再 exec 脚本 · ✗ 无「递不到脚本退 8」档）：旗标 / 退码 / 文案逐档照原件。
		// ★ 安全线：本族动作归换件面 ⇒ ✗ 不在本单真跑（验收只用 `--help`/未知参数）。
		{
			path:        []string{"swap", "agent"},
			danger:      &dangerSpec{dangerD3, "换件目标", "真换生产件（会动生产 `bin/` 并重启 launchd / UI 进程 —— 不可逆动作的邻居）—— 底层脚本当前**只报告档**（真换件 `--swap` 一律 rc=2 零副作用）", "GAP-20261005-39 · 同族先例 core restart / agent restart（D3）· A档批1 换件面（A2/A3 回执 §四 待 S6 补）", false, dangerScopeSystem},
			opened:      true,
			summary:     "子端换件（真搬 · 行为落 Go —— 不再 exec scripts/build/zerg-swap-agent.sh；只报告档）",
			usage:       "zerg swap agent [--check | --help | --self-test | --machine <名> [--label <作业名>]] [--dry-run | --yes]",
			arity:       "any",
			args:        []string{"旗标与退码逐档照 scripts/build/zerg-swap-agent.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdSwapAgent,
		},
		// ---- A 档 批 1（换件面）· A3：桌面 UI 换件的 **CLI 正门**（`zerg swap ui` · 族 `swap` 扩 action · 真搬）----
		// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 1 · A3。
		// 底层原件 = `scripts/build/zerg-swap-ui.sh`（只报告档：真跑 `git status`/`codesign`/进程 inode 对拍；
		// 真换件档未开放 ⇒ `--swap` 一律 rc=2 零副作用）。
		// ★ 真搬后本件自足（✗ 不再 exec 脚本 · ✗ 无「递不到脚本退 8」档）：旗标 / 退码 / 文案逐档照原件。
		// ★ 安全线：本族动作归换件面 ⇒ ✗ 不在本单真跑（验收只用 `--help`/未知参数）。
		{
			path:        []string{"swap", "ui"},
			danger:      &dangerSpec{dangerD3, "换件目标", "真换生产件（会动生产 `bin/` 并重启 launchd / UI 进程 —— 不可逆动作的邻居）—— 底层脚本当前**只报告档**（真换件 `--swap` 一律 rc=2 零副作用）", "GAP-20261005-39 · 同族先例 core restart / agent restart（D3）· A档批1 换件面（A2/A3 回执 §四 待 S6 补）", false, dangerScopeSystem},
			opened:      true,
			summary:     "桌面 UI 换件（真搬 · 行为落 Go —— 不再 exec scripts/build/zerg-swap-ui.sh；只报告档）",
			usage:       "zerg swap ui [--check | --help] [--dry-run | --yes]",
			arity:       "any",
			args:        []string{"旗标与退码逐档照 scripts/build/zerg-swap-ui.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdSwapUI,
		},
	}
}
