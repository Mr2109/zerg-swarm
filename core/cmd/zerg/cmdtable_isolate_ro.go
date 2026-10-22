package main

// 命令表 · isolate-ro 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_isolate_ro() []*command {
	return []*command{
		// ---- 只读隔离（`GAP-20260926-20` P0 · 2026-09-26）：脚本面的**CLI 正门** · **顶层命令** ----
		// 底层原件 = `scripts/dev/isolate-ro.sh`（只转发、不翻译；退码 0/1/2 原样转出）。
		// ★ 真搬后本件自足（✗ 不再 exec 脚本 · ✗ 无「递不到脚本退 8」档）：旗标 / 退码 / 文案逐档照原件。
		// ★ 落**顶层**（父代理一拍）：① 与已登记名一致 —— 技能参考 `references/safe-isolation.md`
		//   与缺口账写的就是 `zerg isolate-ro new|verify`；② 不撞 standing 判据 —— `dev` 族被钉成
		//   「只增改环 · 读环一条都不新增」（`TestDevFamilyIsACHangedOnlyRingNoNewReadCommands`
		//   + 四道闸 `G2` 的七条闭集），那条属性是**有意的**；隔离是工具类正门、不是 `dev` 族的读环。
		// 名字分家：`new` = 造副本（写只落在副本/`--dst`）· `verify` = 只读逐件 sha256 验回。
		{
			path:        []string{"isolate-ro"},
			summary:     "只读隔离：「在别处试」的**硬拷贝**副本（new）/ 逐件 sha256 验回没碰真仓（verify）· 真搬 · 行为落 Go —— 不再 exec scripts/dev/isolate-ro.sh",
			usage:       "zerg isolate-ro new <真仓> [--dst <目录>] [--include <glob>]… [--exclude <路径>]… [--all] [--reject-symlinks] [--no-manifest] [--quiet] | zerg isolate-ro verify <副本> <真仓> [--max-examples N] [--quiet] | zerg isolate-ro --self-test",
			arity:       "any",
			args:        []string{"动作：new（造副本 · 真仓只读）| verify（只读对拍）", "旗标与退码逐档照 scripts/dev/isolate-ro.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdIsolateRO,
		},
	}
}
