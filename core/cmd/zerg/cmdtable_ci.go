package main

// 命令表 · ci 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_ci() []*command {
	return []*command{
		{
			// 组4 §二.4 `W-51`（`研-禁:134` `R-21` 归绿核心）· 任务单序133 · 与组3 `Q-025` 同条：
			// 归绿判据逐字「指定 commit 的必需检查集合全部 `success`、且没有一个 `skipped`」，
			// 读数带 `head_sha` —— 「**人贴屏不算判据**」⇒ 本命令是那张图的**只读**机器面
			// （读本机盘上两件：check-run 记录件 + 必需集合声明件 · 无网络面、无凭据面）。
			// ★ 「必需集合」的来处是**声明件**（`source` 闭集 `github_settings`/`unverified`）；
			//   `unverified` ⇒ 读数照出，但**口径行**明写「与 GitHub 侧 settings 的对齐未核」
			//   （源件逐字「未核（要 GitHub 侧 settings 才看得到）」）—— 两件事不许并成一个词。
			path: []string{"ci", "green"},
			kind: "CiGreen",
			// ★ 字段表必须写成 `[]string{…}` **字面量** —— 与上面 `net probe` / `publish tree has`
			//   两条**同一条纪律**（契约脚本的 `FIELDS_RE` 只认字面量；抽成变量 = 该条被读成
			//   「没有字段表」）。
			summary:  "公开面 CI「归绿」**只读**读数（必需集合逐名 `success` 且集合内 `skipped`=0 · 带 `head_sha` · 声明集合的「必需」那格未核 ⇒ 口径行里印 · 有一条判不出 ⇒ 8 · **不给结论**）",
			usage:    "zerg ci green --run <记录件> [--decl <声明件>] [--json <字段>]",
			fields:   []string{"head_sha", "caliber", "decl_source", "required", "skipped", "verdict"},
			endpoint: "",
			run:      cmdCiGreen,
		},
	}
}
