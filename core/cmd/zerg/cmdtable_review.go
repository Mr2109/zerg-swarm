package main

// 命令表 · review 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_review() []*command {
	return []*command{
		// ---- 脑确认复查（批 B3 · 2026-10-20）：独立入口 `core/cmd/zerg-review` 归入命令面 ----
		// 位置参数与旗标**逐字**保留源件（`<workdir> <issue-id|list> [approve|reject]` +
		// 一枚 `-repo` ⇒ 命令面写作 `--repo`），只把 Go `flag` 的单横线写成双横线。
		// **双轨期**：`core/cmd/zerg-review` 二进制一字不动（本族只加一条入口）。
		// 实现在 `family_review.go`（件头逐条写清与源件的六处差异）。
		// 写面（真 merge / 真丢分支 / 改 issue 状态）但**不进危险表**，理由与 `sched run` 那一格同：
		// 它是操作者对**自己指定的**工作目录/仓库当场做的确认动作（源件本来就这么用），宿主是本机、
		// 目标是命令行给的那个路径，✗ 无远端效应；再加 `--yes`/`--dry-run` 会改源件行为 ⇒ 双轨期不加。
		{
			path:     []string{"review"},
			kind:     "Review",
			summary:  "脑确认复查（C4 流程：`approve` 把分支 merge 回 main 并置 issue done · `reject` 丢分支并置 retry · `list` 列在等确认的单）",
			usage:    "zerg review <workdir> <issue-id|list> [approve|reject] [--repo <路径>]",
			arity:    "any",
			args:     []string{"<workdir>", "<issue-id|list>", "[approve|reject]"},
			endpoint: "",
			run:      cmdReview,
		},
	}
}
