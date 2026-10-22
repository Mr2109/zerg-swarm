package main

// 命令表 · config 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_config() []*command {
	return []*command{
		// ★ `danger` 登记（**D2** · 2026-09-27）：本表项**真跑有副作用**，修前无 `danger` ⇒ `zerg help` 把它当安全档 ✗。
		//   定档理由（逐条读 `family_config.go:cmdConfigReload` 得）：本件**零 os 落盘调用**（无 `os.WriteFile` /
		//   `os.MkdirAll` / `os.Create` / `os.Rename` / `os.Remove`）—— 它的效果面是 **HTTP POST**
		//   （`family_config.go:599` `POST /api/config/reload`：让在跑的主控重读名册、路由表即时更新）；
		//   不重启、不停任何进程、已加载模型不受影响；名册件本地解析不过 ⇒ 一个请求都不发（旧配置继续跑）
		//   ⇒ 可逆（再热加载一次即回旧档）⇒ **D2**（`--yes` 即可），不是 D3 —— 同族先例 = `zerg core reload`
		//   表项（`{\"core\", \"reload\"}`：同为「让主控重读配置」的 D2）。
		//   ★ 三态面的两枚真判**都已接**（本单只补登记，不动实现件）：`--dry-run`（`family_config.go:580`）
		//   ⇒ 计划件走 stdout + rc=0 且**一个请求都不发**；缺 `--yes`（`:593`）⇒ 计划件走 stderr + rc=2
		//   （fail-closed）—— 两条都判在 `sendJSON`（`:599`，本件唯一副作用点）**之前** —— 与
		//   `family_gate_matrix.go:99` 那处只差「两枚同给 ⇒ 2」一条（本单不补，见缺口账）。
		//   ★ `opened: true` = 真跑已开放（`--yes` 就发那一次请求；`--dry-run` 一个请求都不发）。
		{
			path:     []string{"config", "reload"},
			kind:     "ConfigReload",
			summary:  "热加载主控配置（照 `nginx -s reload`：**先校验、失败回滚**）—— 名册件本地解析不过 ⇒ **不发请求**（旧配置继续跑）",
			usage:    "zerg config reload [--dry-run] --yes [--json <字段>]",
			arity:    "none",
			args:     []string{"（无名册件参数：走 `--root` / `--path` 或仓根）"},
			fields:   []string{"status", "models", "added", "fleet_nodes", "fingerprint"},
			danger:   &dangerSpec{dangerD2, "（群级：不点名主机）", "让在跑的主控重读名册件（`POST /api/config/reload`）—— 生效面即时（路由表更新）；不重启、不停任何进程、已加载模型不受影响 · 名册件解析不过 ⇒ 一个请求都不发（旧配置继续跑）；可逆：再热加载一次即回旧档", "缺口账 `GAP-20260927-342` · 同族先例 `zerg core reload`（同为 D2）· §三 C 族 · §6.3 S5 · §九 M3 C1", false, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/config/reload（路由已在跑的主控上 ⇒ 主控零改动）",
			run:      cmdConfigReload,
		},
	}
}
