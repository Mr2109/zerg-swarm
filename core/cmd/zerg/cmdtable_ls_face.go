package main

// 命令表 · ls-face 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_ls_face() []*command {
	return []*command{
		// ── A2（2026-09-27）· 件名统一出口的**命令面**（《设计-件名统一出口与调用点判据-v0.1.md》
		//   §4.1 · P-7/P-10 · 任务清单 A2 · 缺口账 `GAP-20260927-216` 的 ②）─────────────
		//   本命令 = A1 出口（`core/internal/gitpaths`）的**唯一**命令面入口：取件名只经
		//   `gitpaths.List`，本命令**零 git argv**；四个面旗标互斥、缺省不猜；`--json` 带
		//   **字节面字段** `path_bytes`（全量小写十六进制，照 git2-rs `path_bytes` 形状）。
		//   实现件：`core/cmd/zerg/family_lsface.go`（★ 2026-09-27 当读：A1 已补出 `FaceOthers`，该件四面全接上出口、末段**无**遗留缺口）。
		{
			path:     []string{"ls-face"},
			kind:     "LsFace",
			summary:  "件名统一出口（A1 `core/internal/gitpaths`）的命令面：**逐字节真名**（转义形态在出口处硬失败 C-5）· 四个面旗标互斥、缺省不猜",
			usage:    lsFaceUsage,
			arity:    "none",
			args:     []string{"（不收位置参数 —— 面由旗标给、路径由 `--prefix <路径>` 给）"},
			fields:   lsFaceFields,
			endpoint: "",
			run:      cmdLsFace,
		},
	}
}
