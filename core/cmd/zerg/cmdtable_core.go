package main

// 命令表 · core 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_core() []*command {
	return []*command{
		{
			path:     []string{"core", "status"},
			kind:     "CoreStatus",
			summary:  "主控现状（投影 /api/core/status）",
			usage:    "zerg core status [--json <字段>]",
			fields:   []string{"ok", "pid", "started_at", "version"},
			endpoint: "GET /api/core/status",
			run:      cmdCoreStatus,
		},
		{
			path:     []string{"core", "logs"},
			kind:     "CoreLogs",
			summary:  "主控日志（`/api/logs` **路由没接** ⇒ 不给结论，退码 8）",
			usage:    "zerg core logs [--json <字段>]",
			fields:   []string{"available", "detail"},
			endpoint: "GET /api/logs（处理器在 handlers.go:966 · 路由没接 ⇒ 现跑 404）",
			run:      cmdCoreLogs,
		},
		{
			path:     []string{"core", "daemon"},
			kind:     "CoreDaemon",
			summary:  "`daemon ls`：本机服务脚本逐件可查（scripts/svc/ 5 件）+ `--declared` 并给声明面两列与幽灵段（与 `doctor` 同源同值）",
			usage:    "zerg core daemon ls [--declared] [--json <字段>]",
			arity:    "any",
			args:     []string{"动作（ls）"},
			fields:   []string{"name", "script", "declared", "note"},
			endpoint: "",
			run:      cmdCoreDaemonLs,
		},
		// ---- 波① · T2 `Q-056`：现值面给人读三格（pid / 起时 / 命令行 · 2026-09-23）----
		// 今天**没有**这条命令（现跑「未知命令 "core ps"」）—— 幽灵/声明差要靠人肉 `ps | grep`
		// 加 `plutil -p` 才看得出（`Q-057` 的病根）。
		{
			path:     []string{"core", "ps"},
			kind:     "CorePs",
			summary:  "现值面逐条读（**三格**：pid / 起时 / 命令行）—— 行面 = 声明件点名的进程特征命中的进程（launchd 声明的 + ghost 幽灵都列）· 只读",
			usage:    "zerg core ps [--root <仓根> | --path <声明件>] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无：现值面是「读全机」不是「要目标」）"},
			fields:   []string{"pid", "start", "command", "kind", "name", "owner"},
			endpoint: "",
			run:      cmdCorePs,
		},

		// ---- GAP-20261021-03（2026-10-23）：`core load` —— 主控当前负载的**只读可见面**
		// （在算什么 / 为谁算 / 还要多久 ＋ CPU 与耗时自测）。实现件 = `family_core_load.go`；
		// ★ 只读：不碰调度、不 kill、不写状态件、不新开路由（复用 `GET /api/core/status` ＋ 本机 `ps` 两帧差分）。
		{
			path:     []string{"core", "load"},
			kind:     "CoreLoad",
			summary:  "主控当前负载（只读投影 ＋ 本机两帧差分测 CPU）：判读 满载/忙/空闲 · `cpu_pct`/`cpu_cores`/`rss_mb`/`uptime` · 逐枚在装任务给 `model`/`machine`/`phase`/`elapsed_s`/`eta_s` · 采样窗口 500ms（非生命期均值）",
			usage:    "zerg core load [--json <字段>]",
			arity:    "none",
			fields:   coreLoadFields,
			endpoint: "GET /api/core/status（＋ 本机 `ps` 两帧差分）",
			run:      cmdCoreLoad,
		},
		// ---- T2④ / T4①（2026-10-14 · 本单）：`core db <check|backup|restore|prune>` —— 虫族库
		// 备份 / 恢复 / 体检 / GFS 轮转的**唯一命令入口**。设计 §五 Q12 逐字：「✗ 不新开 `zerg db` 族」
		// ⇒ 挂**既有 `core` 族**下；登记形制照同族 `core daemon`（一条命令 · 子动作在 `inv.args[0]` 里派发）。
		// 实现件 = `core/cmd/zerg/family_core_db.go`（薄壳 · 逐字转发 `zdb` 已落接口 —— ✗ 命令层重写逻辑）；
		// ★ 与归档**共用同一枚 plist**（`com.zerg.dbbackup` · 先备份后归档）。
		{
			path:     []string{"core", "db"},
			kind:     "CoreDb",
			summary:  "虫族库五动作（`stamp`：**只打标不迁移** —— 只补 `PRAGMA application_id` 身份标（三态：==本标 空动作 / ==0 写一次＋回读自证 / 其它非 0 拒执），✗ 不跑迁移 ⇒ `user_version` 一字不动）（薄壳 · 转发 `core/internal/store/zdb`）：`check` **只读**体检（integrity_check ＋ foreign_key_check ＋ application_id ＋ user_version/page_count/journal_mode —— 真库上跑也一字节不写）· `backup` GFS 快照（checkpoint(TRUNCATE) → VACUUM INTO → 侧车八格 → prune 日14/周8/月12 · 保底 ≥3）· `restore` 从快照就位（VACUUM INTO → 体检 → 逐表行数对拍 → rename · 自证以侧车为准）· `prune` 逐代轮转（**只删匹配命名件**）",
			usage:    "zerg core db <check|backup|prune|stamp> [--db <库路径>] [--dry-run | --yes] [--json <字段>] · zerg core db restore --from <快照> --to <库路径> --confirm=<库路径> --yes [--dry-run] [--json <字段>]",
			arity:    "any",
			args:     []string{"动作：check | backup | restore | prune | stamp"},
			fields:   coreDbFields,
			danger:   &dangerSpec{dangerD3, "库路径", "改虫族群库面：`restore` **覆盖 `--to` 指向的库**（自证 / 体检不过 ⇒ `zdb` 里 fail-closed，rename 之前就退）· `backup` / `prune` 写 / 删 `~/.zerg/backup/` 里的快照（GFS **只删匹配命名件** · 保底 ≥3）；`check` **只读**零写", "设计-虫族数据库-20261013 §四.2/§四.3 · §五 Q12/Q23/Q24 · 任务书 T2④/T4①", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdCoreDb,
		},
		{
			path:    []string{"core", "start"},
			machine: machineHostName,
			summary: "起主控（危险 D3 · 本版未开放）",
			usage:   "zerg core start [--node <机>] --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "起主控进程（会绑端口 8580；已在跑时是**换件**前置）", "§三 C 族 · §7.1 P11 · 开工单 T-45", true, dangerScopeSystem},
			run:     cmdGuarded,
		},
		{
			path:    []string{"core", "restart"},
			machine: machineHostName,
			summary: "重启主控（危险 D3 · 已开放：确认档齐就真执行 `launchctl kickstart -k`）· 审计留痕 + 就绪判据绑自己的 pid",
			usage:   "zerg core restart [--node <机>] --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "停 + 起主控（**整个虫群的控制面会断一会儿**）", "§三 C 族 · 开工单 T-45 · 缺口 Q-103", true, dangerScopeSystem},
			// `opened`: 真跑已开放（`--confirm=<主机名>` 与 `--yes` **同时到**才执行；`--dry-run` ⇒ 计划件 rc=0 ·
			// 缺确认档 ⇒ fail-closed rc=2）。动作只有一条：`launchctl kickstart -k gui/<uid>/com.zerg.core`
			// （定义不一致 / 未装载那两支不走 —— 归换件入口 `scripts/build/zerg-swap-core.sh`）。
			opened: true,
			run:    cmdCoreRestart,
		},
		{
			path:    []string{"core", "stop"},
			machine: machineHostName,
			summary: "停主控（危险 D3 · 本版未开放）",
			usage:   "zerg core stop [--node <机>] --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "停掉主控进程（**整个虫群的控制面会断**）", "§三 C 族 · §7.1 P11 · 开工单 T-45", true, dangerScopeSystem},
			run:     cmdGuarded,
		},
		{
			path:     []string{"core", "reload"},
			danger:   &dangerSpec{dangerD2, "（群级：不点名主机）", "让主控重读配置（规则表/名册 · `POST /api/config/reload`）—— 生效面即时；不重启、不停任何进程、已加载模型不受影响 · 名册件解析不过 ⇒ 一个请求都不发；可逆：再热加载一次即回旧档", "§三 C 族 · §6.3 S5 · 同族已开放先例 `config reload`（同端点 · D2）· §九 M3 C1", false, dangerScopeRemote},
			opened:   true,
			summary:  "重载主控配置（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/config/reload`；先校验名册件，解析不过不发请求）",
			usage:    "zerg core reload [--dry-run] --yes",
			endpoint: "POST /api/config/reload（路由已在跑的主控上 ⇒ 主控零改动）",
			run:      cmdCoreReloadGuarded,
		},
		{
			path:    []string{"core", "update"},
			machine: machineHostName,
			summary: "主控自身换件（危险 D3 · 本版未开放）",
			usage:   "zerg core update [--node <机>] --confirm=<主机名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"主机名"},
			danger:  &dangerSpec{dangerD3, "主机名", "换掉在跑的主控制品（**不可逆**；走 F-3 例外清单 + 验签 + 回执）", "§九 M20 F-3 · §7.1 P12 · 开工单 T-52", true, dangerScopeSystem},
			run:     cmdGuarded,
		},
	}
}
