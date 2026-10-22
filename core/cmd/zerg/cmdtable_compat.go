package main

// 命令表 · compat 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_compat() []*command {
	return []*command{
		// ---- 批 B7（2026-10-20 · `zerg compat list|check|migrate|manifest`）：独立入口
		// `core/cmd/zerg-compat` 归入命令面 ----
		// 源件 243 行 · 四个子命令（`list` / `check` / `migrate` / `manifest`）· 四枚旗标逐字保留
		// （`--state-dir` / `--receipts-dir` / `--dry-run` / `--json`）。
		// ★ 派单原话是「并入**既有** compat 族」—— 而**现读没有这个族**（`zerg help --all` 零命中 ·
		//   本文件 `compat` 零命中）⇒ 无处可并，只能**新开顶层族 `compat`**（族名 = 源件名的主体）。
		//   为什么不挂 `build`（`build --only compat` 是构建档名，语义完全不同）/ `doctor` / `gate`：
		//   理由逐条写在 `family_compat.go` 的件头（照实记一笔，不粉饰派单与现读的差）。
		// 判定与落盘一行不改（仍是 `core/internal/compat` 的 Entries/StatusAll/MigrateAll/
		//   ManifestJSONForDump）；退码按 `exitcodes.go` **重取号**（源件 2/3 ⇒ 命令面 8/8，以
		//   `error.detail` 分家；源件 4「用法错」与命令面 4「未认证」撞号 ⇒ 一律回表取 2）。
		// ★ 写面只有 `migrate`（原地重写状态文件、先备份）：不进危险表 —— 它写的是**本机状态目录**、
		//   落点由 `--state-dir` 或环境变量显式给、每次覆盖前自动备份 `<file>.bak-<ts>`、`--dry-run`
		//   可预演；与 `model record probe`（批 B6）同一条「本机可逆 · 无远端副作用」口径。
		// **双轨期**：`core/cmd/zerg-compat` 二进制一字不动（它自己的 0/1/2/3/4 是**组件自有码空间**，
		//   `exitcode-namespaces.json` 里挂着号）。
		{
			path:     []string{"compat", "list"},
			kind:     "CompatList",
			summary:  "列出跨版本状态文件清单（只读）：逐条给 逻辑名 / 文件 / 当前 schema / 最低可读 / 信封（inband·sidecar）/ 谁盖版本号 / 归属，并打出它将要操作的两个目录",
			usage:    compatListUsage,
			arity:    arityNone,
			args:     []string{"（不收位置参数：目录走 `--state-dir` / `--receipts-dir`）"},
			fields:   []string{"name", "file", "dir", "current_schema", "min_readable", "envelope", "stamps_schema", "owner"},
			endpoint: "",
			opened:   true,
			run:      cmdCompatList,
		},
		{
			path:     []string{"compat", "check"},
			kind:     "CompatCheck",
			summary:  "只读体检：逐文件报现状（不迁移、不备份、不写盘）—— 全部就绪 ⇒ 0 · 有读不动的状态文件 ⇒ 1 · 有待迁移文件或高于本机认知的 schema ⇒ 8（`error.detail` 分家两档）",
			usage:    compatCheckUsage,
			arity:    arityNone,
			args:     []string{"（不收位置参数：目录走 `--state-dir` / `--receipts-dir`）"},
			fields:   []string{"name", "path", "outcome", "on_disk_schema", "current_schema", "min_readable", "envelope", "owner", "error"},
			endpoint: "",
			opened:   true,
			run:      cmdCompatCheck,
		},
		{
			path:     []string{"compat", "migrate"},
			kind:     "CompatMigrate",
			summary:  "执行一次性迁移（幂等 · 可反复跑）：旧版/无 schema 先备份 `<file>.bak-<ts>` 再原地写回并回读校验，校验不过回滚；`--dry-run` 只报将要迁移的文件、不落盘；失败 ⇒ 1",
			usage:    compatMigrateUsage,
			arity:    arityNone,
			args:     []string{"（不收位置参数：目录走 `--state-dir` / `--receipts-dir`）"},
			fields:   []string{"name", "path", "outcome", "error", "on_disk_schema", "current_schema", "min_readable", "envelope", "owner"},
			endpoint: "",
			opened:   true,
			run:      cmdCompatMigrate,
		},
		{
			path:     []string{"compat", "manifest"},
			kind:     "CompatManifest",
			summary:  "打印内嵌清单原文（CI 门禁 / 排障的对照面）—— 直接写原始字节，不再序列化一遍（`<` 不许被转义成 `\\u003c`）；不收任何旗标",
			usage:    compatManifestUsage,
			arity:    arityNone,
			args:     []string{"（不收位置参数、也不收旗标：这一面就是清单原文）"},
			endpoint: "",
			opened:   true,
			run:      cmdCompatManifest,
		},
	}
}
