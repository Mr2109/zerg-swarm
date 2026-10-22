package main

// 命令表 · route 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_route() []*command {
	return []*command{
		// ── 「显式指定机器」族（`route` · 2026-09-24 · 单独定制 > 路由默认规则）──────────────
		//   现场（逐字）：子代理**没法把模型钉到指定机器** —— 网关择优默认挑本机（请求落在 Mr2109 的
		//   `llama-server`），而人要的是该机型；当时只有「手工 `POST /api/control/unload {"machine":"Mr2109"}`
		//   绕路」这一条路。设计稿：`Zerg-内部文档/项目文档/v2.5.12/设计-指定机器路由-v1.0-20260924.md`。
		//   覆盖表 = `<状态目录>/route_pins.json`（**不在任何仓里** · 三件套里最要紧的「不永久改变默认」
		//   就落在那张表的 TTL 上）；选机那道门（`core/internal/gateway/route_pin.go`）**只读**它。
		//   ★ 三条命令都**不新增顶层命令**语义面之外的东西：命令树 +3，旗标面只加 `--machine` 一枚。
		{
			path: []string{"route", "pin"},
			kind: "RoutePin",
			// ★ 字段表必须写成 `[]string{…}` **字面量** —— 与 `net probe` / `publish tree has`
			//   / `ci green` 三条**同一条纪律**（契约脚本的 `FIELDS_RE` 只认字面量）。
			summary: "把**某个模型**钉到**某台机器**上（带 TTL 的覆盖表 · 命中即用 · 撤回一行 `route unpin`）",
			usage:   "zerg route pin --model <模型> --machine <机器> --ttl <时长> [--by <谁>] [--note <…>] [--dry-run | --yes] [--json <字段>]",
			arity:   "none",
			args:    []string{"（无位置参数：模型走 `--model`、机器走 `--machine`）"},
			fields:  []string{"model", "machine", "expires_at", "remaining_s", "state"},
			danger: &dangerSpec{dangerD2, "模型",
				"把这一条写进覆盖表（该模型的选机**命中即用**该机器）—— 可逆：`route unpin` 或等它自己到期",
				"设计-指定机器路由-v1.0-20260924.md §4 · 单独定制 > 路由默认规则（个别要求一次性 + TTL）", false, dangerScopeOutRepo},
			// `opened`: 真跑已开放（`--yes` 就写；`--dry-run` 零副作用）。
			opened:   true,
			endpoint: "",
			run:      cmdRoutePin,
		},
		{
			path:    []string{"route", "ls"},
			kind:    "RouteLs",
			summary: "看覆盖表现在钉着什么（**只读**：不碰盘、不探主控、零网络 · 过期行也显出来）",
			usage:   "zerg route ls [--model <模型>] [--json <字段>]",
			arity:   "none",
			args:    []string{"（无位置参数：过滤走 `--model`）"},
			// 与上面 `route pin` 那一条**同一份五格**（同一个写法只有一个来源）。
			fields: []string{"model", "machine", "expires_at", "remaining_s", "state"},
			// `opened`: 只读面直接开放（不写、不探网络 ⇒ 无危险档，与 `route pin/unpin` 的 D2 成对）。
			opened:   true,
			endpoint: "",
			run:      cmdRouteLs,
		},
		{
			path: []string{"route", "unpin"},
			kind: "RouteUnpin",
			// ★ 字段表同样必须是字面量（同上）。
			summary: "撒掉一条钉（不给 `--model` ⇒ **撒全部** · 一行撤回 · 幂等：没撒到 ⇒ 0 + `changed=false`）",
			usage:   "zerg route unpin [--model <模型>] [--dry-run | --yes] [--json <字段>]",
			arity:   "none",
			args:    []string{"（无位置参数：点名走 `--model`）"},
			fields:  []string{"model", "machine", "expires_at", "remaining_s", "state"},
			danger: &dangerSpec{dangerD2, "模型",
				"把点名的行从覆盖表里去掉（该模型的选机**立刻回默认择优**）—— 可逆：再 `route pin` 一次",
				"设计-指定机器路由-v1.0-20260924.md §4 · 「一行撤回」那条约束的落点", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdRouteUnpin,
		},
	}
}
