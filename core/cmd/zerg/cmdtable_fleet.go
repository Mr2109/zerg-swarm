package main

// 命令表 · fleet 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_fleet() []*command {
	return []*command{
		// ── 命令树登记批（2026-10-09）：`zerg fleet logs` 入口（原占位只到 `fleet ls`）──────
		// 真身 = `fleet_logs.go` 的 `fleetLogsGateMain`（`gateway/scripts/fleet_logs.py` 的 Go 真搬 ·
		// 四子命令 query/trace/errors/stats · 打主控 :8580 只读端点 · 已 inproc 登记
		// `gate_inproc_table.go:166` 键 `fleet-logs`）。入口 `cmdFleetLogs`（宿主 `ui_start.go` ·
		// 门78 面④ 同 `mcp serve` 理由）**只转发**：✗ 不重写它任何一段 · 退码原样转出
		// （0 成功 / 1 调用或 HTTP 失败 / 2 用法错）。
		{
			path:        []string{"fleet", "logs"},
			kind:        "FleetLogs",
			layer:       "host",
			summary:     "机群日志查询（query/trace/errors/stats · 打主控 :8580 只读端点 · 真身 `fleet_logs.go:fleetLogsGateMain`；入口只转发）",
			usage:       "zerg fleet logs [--controller <地址>] [--token-file <件>] <query|trace|errors|stats> [旗标…]",
			arity:       "any",
			args:        []string{"子命令与旗标逐字来自 gateway/scripts/fleet_logs.py（原样透传 · 0 成功 / 1 调用或 HTTP 失败 / 2 用法错）"},
			passthrough: true,
			run:         cmdFleetLogs,
		},
		// ---- E1（缺失项 `GAP-20260927-172` 之正解 · 稿 E v2 §2 `F1′`）：`zerg fleet render` ----
		// 真源（名册 `~/.zerg/contexts/<档>.yaml` 的 `nodes:`）⇒ 生成投影（`gateway/fleet.yaml` 的
		// `fleet:` 段与 `updates.nodes`）—— 消「两处手抄 ⇒ 漂移」。真身 = `family_fleet.go:cmdFleetRender`。
		// 三档：`--preview-only`（缺省·只打印·零写盘）/ `--emit`＋`--yes`（真写·D2 = 同目录临时件＋rename
		// 原子写＋读回校＋不过即回滚 · 先例 `zerg model add`）/ `--expect-no-changes`（只读断言·现读≠生成 ⇒ 非 0）。
		// ★ 只碰投影件的两段**数据行** ⇒ 不碰真源、不碰 `models:` 与注释行（门④判据 B 的投影源声明）。
		{
			path:     []string{"fleet", "render"},
			kind:     "FleetRender",
			layer:    "host",
			summary:  "由**真源**（名册 `~/.zerg/contexts/<档>.yaml` 的 `nodes:`）生成**投影**（`gateway/fleet.yaml` 的 `fleet:`/`updates.nodes`）—— 消「两处手抄 ⇒ 漂移」（缺失项 `GAP-20260927-172`）；`--preview-only`（缺省·只打印·零写盘）/ `--emit`＋`--yes`（真写·D2 原子写+读回校+回滚，只碰两段数据行）/ `--expect-no-changes`（只读断言·现读≠生成 ⇒ 非 0）",
			usage:    "zerg fleet render [--preview-only] [--expect-no-changes] [--emit] [--roster <名册件>] [--path <投影件>] [--context <档名>] [--json <字段>] [--dry-run | --yes]",
			arity:    "any",
			args:     []string{"（无位置参数）"},
			fields:   []string{"action", "roster", "fleet", "nodes", "drift", "changed", "result"},
			danger:   &dangerSpec{dangerD2, "投影件（`gateway/fleet.yaml` 的 `fleet:`/`updates.nodes` 两段数据行）", "`--emit`＋`--yes` 时写投影件 `gateway/fleet.yaml` 的两段数据行（同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚；✗ 不改真源 `~/.zerg/contexts/…`、✗ 不碰 `models:`/注释行）；可逆：由名册重渲即回名册态", "缺失项 `GAP-20260927-172` · 稿 E v2 §2 `F1′` · 同族写面先例 zerg model add（D2）· §九 M3 C1", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdFleetRender,
		},

		// ---- GAP-20260926-92（2026-10-23）：`fleet topology` —— **部署拓扑只读面**
		// （名册真源 ⟷ 投影 对拍 ＋ 逐机判「远端自持与否」）。实现件 = `family_fleet_topology.go`；
		// ★ 只读：零网络 / 零 ssh / 零写盘 —— ✗ 不替代 `agent ls` 观测面。
		{
			path:     []string{"fleet", "topology"},
			kind:     "FleetTopology",
			layer:    "host",
			summary:  "部署拓扑只读面（名册真源 ⟷ 投影 对拍 ＋ 逐机判「远端自持与否」：自持=components 含 core ＋ fleet 回执 converged；推送=agent deploy 台账 verified 事件）",
			usage:    "zerg fleet topology [--roster <名册件>] [--path <投影件>] [--root <仓根>] [--context <档名>] [--expect-no-changes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数）"},
			fields:   []string{"machine", "components", "root", "ssh", "self_host", "channel", "last_self_update", "self_update_converged", "last_push", "last_push_file", "verdict", "drift"},
			endpoint: "",
			run:      cmdFleetTopology,
		},
	}
}
