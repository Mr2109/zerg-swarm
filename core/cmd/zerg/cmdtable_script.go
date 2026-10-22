package main

// 命令表 · script 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_script() []*command {
	return []*command{
		{
			path:     []string{"script", "ls"},
			kind:     "Script",
			summary:  "本机脚本清单（scripts/ · deploy/ · gateway/ 下的脚本件逐件 + 公开标记表）",
			usage:    "zerg script ls [--json <字段>]",
			arity:    "none",
			fields:   []string{"path", "public", "kind", "count_basis", "refs"},
			endpoint: "",
			run:      cmdScriptLs,
		},
		// ---- `A3`（`G-13`/`Q-002`）取「b」案：仓外台账（脚本现状清单）的**唯一写命令** + 写面进审计 ----
		// 已拍口径逐字（`设计-仓外台账写面-A3-b案-v1.0-20260923.md` §二）：台账**留仓外** + 一条唯一写命令
		// （三态照 `repo commit`：`--dry-run` 是**唯一**会返回 0 的那一态 · 缺 `--yes` ⇒ fail-closed 2）+ 审计。
		// ★ 与只读面**分家**：上面那条 `script ls`（`family_h.go cmdScriptLs`）**一字不动** ——
		//   只读面加一枚写旗标就变成第二条写路径（设计稿 §三）。
		{
			path:    []string{"script", "inventory", "sync"},
			kind:    "ScriptInventorySync",
			summary: "把**仓外**台账（脚本现状清单）按现跑重算（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 · 写面进审计）",
			usage:   "zerg script inventory sync [--docs-root <Zerg-内部文档 根>] [--dry-run | --yes] [--by <谁>] [--json <字段>]",
			fields: []string{"result", "target", "rows_live", "rows_added", "rows_removed", "rows_unchanged",
				"declared_line", "inventory_before_sha256", "inventory_after_sha256", "audit_path"},
			danger:   &dangerSpec{dangerD2, "台账件", "把仓外台账的行面与两个 yes/no 列按现跑重算（可逆：写前备份 + 写后读回，复查不过逐字节写回）；**一条命令写**、**不许第二条写路径**", "设计-仓外台账写面-A3-b案-v1.0-20260923.md §二 · `A3` = `G-13`/`Q-002`", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdScriptInventorySync,
		},
		// ---- 退场正门（设计稿 `Zerg-内部文档/01-设计/设计-退场正门-retire-20261006.md` ＋ 拍板 决-01/02/03/09 · 2026-10-07）----
		// 定位：**只管登记面退场**，✗ **不代管物理删除**（`git rm`）——真删交 git 历史＋提案审计（决-01）。
		// 真源**沿用** `core/internal/contract/adopt-retire.json` 的 `retire_ledger`（✗ 不另起第二真源 · 决-02）；
		// 目标件闭集 = **自研面全体 py/sh**（口径**单点真源** = `scriptRetireClosedSetExcludedPrefixes` · ✗ 不在此重抄前缀表以免长歪 · 决-12 · ★ 描述串与 `scriptRetireClosedSetDesc` 同源）；
		// 例外类 = **逐条登记名册 ＋ 理由 ＋ 保留代价栏**（同真源 `retire_exceptions` · 决-09）。
		// 危险档 D2（与 `inventory sync` 同档）：`--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 · 写面进审计
		//   （审计先落盘 → `cp -p` 备份真源 → 原子写 → 写后读回，复查不过逐字节写回）。
		// 退码：0 成功 · 1 拒绝（例外类 / 目标不在闭集）· 2 用法错／不给结论（缺 `--yes`／缺 `--evidence`／
		//   目标不在盘／真源取不到）· 14 冲突（已有同 `object` 条目）。实现在 `family_script_retire.go`。
		{
			path: []string{"script", "retire"},
			kind: "ScriptRetire",
			summary: "退场正门（**只管登记面** · ✗ 不代管 `git rm`）：把一件自研面 py/sh 登记进 `retire_ledger`" +
				"（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 · 写面进审计）；例外类件 / 目标不在闭集 ⇒ 1；" +
				"目标不在盘 / 真源取不到 ⇒ 2；已有同 `object` 条目 ⇒ 14",
			usage: "zerg script retire <件> [--dry-run | --yes] [--evidence <出处>]… [--by <谁>] [--json <字段>]",
			arity: "any",
			args:  []string{"件（仓内相对路径 · " + scriptRetireClosedSetDesc + "）"},
			fields: []string{"result", "object", "state", "closed_set", "in_exceptions",
				"ledger_entries_before", "ledger_entries_after", "ledger_before_sha256", "ledger_after_sha256", "audit_path"},
			danger: &dangerSpec{dangerD2, "登记面（`retire_ledger`）",
				"把一件自研面件登记为「已登记退场」（可逆：写前 `cp -p` 备份 + 写后读回，复查不过逐字节写回）；**只登记 · ✗ 不代管 `git rm`**（真删交 git 历史＋提案审计）",
				"Zerg-内部文档/01-设计/设计-退场正门-retire-20261006.md §一/§二/§三 · 拍板落定-所有稿-20261007.md 决-01/02/03/09", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdScriptRetire,
		},
		{
			path:     []string{"script", "inventory", "verify"},
			kind:     "ScriptInventoryVerify",
			summary:  "只读核对仓外台账与现跑（新增/消失/格值变化任一非空或声明行不等 ⇒ 1 · 全一致 ⇒ 0 · 读不到 ⇒ 8/2 · ✗ 不写件、不收 `--dry-run`/`--yes` —— 要写用 `sync --yes`）",
			usage:    "zerg script inventory verify [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数）"},
			fields:   []string{"result", "target", "rows_live", "rows_added", "rows_removed", "rows_unchanged", "rows_changed", "rows_changed_named", "rows_changed_cells", "declared_line", "declared_line_live", "in_sync"},
			endpoint: "",
			run:      cmdScriptInventoryVerifyFamily,
		},
	}
}
