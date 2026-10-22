package main

// 命令表 · audit 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_audit() []*command {
	return []*command{
		// ---- `GAP-20260928-98` 族（2026-09-28 · laneGS · 用户拍「按推荐」= laneGJ 设计稿 §二 乙档）----
		// 「工作树脏件 ↔ 审计行」的**对拍面**：分母 = `git status --porcelain=v1` 的**已跟踪 `M`/`A` 行**
		// （**不由审计定** —— 审计定的分母会让「零审计行的件」永远无对拍 = 最硬的假绿面）；
		// 对拍 = 工作树 `sha256` ↔ 审计里该件**最新**一条 `edit` 行的 `after_sha256`。
		// ★ **纯只读 · 只报告档（起手档）**：有未覆盖件仍退 0（「无记录」是状态不是错）· 不进任何必跑路径。
		// ★ 与 `repo status` 的口径**不相抵**：判「哪些件脏」的真源**仍是 git porcelain**（family_repo.go 那句
		//   「不自己比 mtime/sha」）；本命令比的是**审计行 vs 工作树身份**（两枚既有 sha 面的对拍），
		//   不是自己造第二套判脏口径 —— 候选实现面 = 只读命令 `core/cmd/zerg/family_audit.go`。
		{
			path:    []string{"audit", "cover"},
			kind:    "AuditCover",
			summary: "对拍「工作树脏件 ↔ 审计行」：分母 = `git status --porcelain=v1` 的**已跟踪 `M`/`A` 行**（真源是 git 自己的面 · **不由审计定**）· 逐件比「工作树 `sha256` ↔ 审计里该件最新一条 `edit` 行的 `after_sha256`」· 四判词 `有记录/无记录/记录过期/读不到` · 逐条点名未覆盖件（件·状态·工作树 sha16·审计有无行）+ 计数 · **只报告档**：有未覆盖仍退 0（不进任何必跑路径）· `8` = 读不到/**分母 0 判不了**（不当绿）· 与 `GAP-20260928-79` 族对偶（79 = 有行但回指不到授权物 · 本格 = 压根没有行）",
			usage:   auditCoverUsage,
			arity:   "none",
			args:    []string{"（不收位置参数 —— 面由 `--file <件>` 收窄、仓根由 `--root <仓根>` 给）"},
			fields:  auditCoverFields,
			// ★ 缺口 GAP-20260926-56（批⑤ · 2026-10-13）：`verified` 走**可选字段层**（`optFields`，与
			//   `code find` 的 `count` 同一体例）—— 「该件有没有改动记录」的机读直读口，登在出口白名单、
			//   `--json` 点得出来，但不进 `fields` 那份冻结帮助 / 文档导出面字段表 ⇒ 不重生命令页。
			optFields: []string{"verified"},
			endpoint:  "",
			run:       cmdAuditCover,
		},
		// ---- `GAP-20261010-20`（2026-10-17）：**仓内硬编码机器地址**的扇盘清单（只读 · L0）----
		// 拍定「做新命令、**不**扩既有断言」：既有 `check-hardcoded-private-paths` 是**门**（判红用），
		// 本命令是**扇盘工具**（给人看清单：件:行:内容 + 总数 + 按目录分组计），默认 rc=0；
		// 只有显式 `--fail-on-hit` 才把命中 > 0 透成 1。★ 规则表/白名单**复用**判据门那两张（不立第二份）。
		{
			path:     []string{"audit", "hardcoded-addr"},
			kind:     "AuditHardcodedAddr",
			summary:  "仓内「硬编码机器地址」**扇盘清单**（只读 · 零写盘 · L0）：扫 `core/` `agent/` `shared/` `scripts/`（不含 `ui/`/依赖树/点目录），逐处点名 `件:行:内容` + 命中总数 + **按目录分组计（处·件）**；规则与值白名单**直接复用**判据门 `check-hardcoded-private-paths` 的三族规则（`192.168.x` / 集群 `10.0.x` / 机器主机名 `*.local`；回环与文档段是声明过的合法写法 ⇒ 不报；裸端口数字不判）· 排除 `*_test.go` 夹具 / 注释行 / Rust 夹具段 / 二进制件 · 豁免件 `scripts/audit-addr-allow.txt`（一行 `路径:子串` · 写坏的行 ⇒ 2，不许静默失效）· **默认清单档：有命中仍退 0**；`--fail-on-hit` ⇒ 命中 > 0 退 1（供门/CI）· `8` = 仓根解析不到 / 扫描面 0 件（空转 ≠ 干净）",
			usage:    auditAddrUsage,
			arity:    "none",
			args:     []string{"（不收位置参数 —— 仓根由 `--root <仓根>` 给）"},
			fields:   auditAddrFields,
			endpoint: "",
			run:      cmdAuditHardcodedAddr,
		},
	}
}
