package main

// 命令表 · resource 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_resource() []*command {
	return []*command{
		// ---- 批 D · T-46 H 族四条（§三 H 族 · §7.1 `P10`/`P7`）----
		{
			path:     []string{"resource", "ls"},
			kind:     "Resource",
			summary:  "资源面（投影 /api/resources/ledger 或 /api/resources/{类型}）",
			usage:    "zerg resource ls [<类型>] [--json <字段>]",
			arity:    "any",
			args:     []string{"类型（可省）"},
			fields:   []string{"machine", "mem_known", "mem_total_gb", "mem_available_gb", "vram_known", "vram_total_gb", "vram_used_gb", "vram_free_gb", "gpu_pct", "backend_state", "fit"},
			endpoint: "GET /api/resources/ledger | /api/resources/{type}",
			run:      cmdResourceLs,
		},
		{
			path:     []string{"resource", "ledger"},
			kind:     "Resource",
			summary:  "资源账本（投影 /api/resources/ledger）",
			usage:    "zerg resource ledger [--json <字段>]",
			fields:   []string{"machine", "mem_known", "mem_total_gb", "mem_available_gb", "vram_known", "vram_total_gb", "vram_used_gb", "vram_free_gb", "gpu_pct", "backend_state", "fit"},
			endpoint: "GET /api/resources/ledger",
			run:      cmdResourceLedger,
		},
		// 写面两枚：**同一个执行门**（三态：--dry-run 计划件 / 缺 --yes ⇒ 2 / 齐了才发）
		{
			path:    []string{"resource", "pin"},
			danger:  &dangerSpec{dangerD2, "资源 id", "钉住资源（改资源不可回收标记 · 控制面 `POST /api/resources/pin`）—— 可逆：`resource unpin` 即回原态", "GAP-20261005-38 · 同族先例 task cancel（D2）· §三 H 族", false, dangerScopeRemote},
			opened:  true,
			kind:    "ResourcePin",
			summary: "钉住资源（D2 写面 · --dry-run 零副作用 · 缺 --yes ⇒ 2）",
			usage:   "zerg resource pin <资源 id> --ttl <时长> [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"资源 id"},
			run:     cmdHazardWrite,
		},
		{
			path:    []string{"resource", "unpin"},
			danger:  &dangerSpec{dangerD2, "资源 id", "解钉资源（去掉不可回收标记 · `POST /api/resources/unpin`）—— 可逆：再 `resource pin` 即回原态", "GAP-20261005-38 · 同族先例 task cancel（D2）· §三 H 族", false, dangerScopeRemote},
			opened:  true,
			kind:    "ResourceUnpin",
			summary: "解钉资源（D2 写面 · --dry-run 零副作用 · 缺 --yes ⇒ 2）",
			usage:   "zerg resource unpin <资源 id> [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"资源 id"},
			run:     cmdHazardWrite,
		},
	}
}
