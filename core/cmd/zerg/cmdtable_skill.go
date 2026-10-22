package main

// 命令表 · skill 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_skill() []*command {
	return []*command{
		// ---- 技能库族（设计稿 `zerg-技能库设计稿-20261017.md` §3 · M1 只读面）----
		// 根只认一处：`statepath.SkillsDir()`（`ZERG_SKILLS_DIR` → `<仓根>/core/internal/agent/skills`）。
		// ✗ 不开 `--root`、✗ 不新增第二个默认根；`--json` 字段表写字面量（`clicFieldsRE` 直读这一行）。
		{
			path:    []string{"skill", "ls"},
			kind:    "SkillLs",
			summary: "技能库清单（**只读**：名字 · 触发描述 · 状态 · 子件数 · 字节数 · 落点；状态把「agent 静默跳过」的件显式标出来）",
			usage:   "zerg skill ls [--json <字段>]",
			arity:   "none",
			args:    []string{"（无位置参数 —— 技能根取唯一解析 `statepath.SkillsDir()` · ✗ 不开 `--root`）"},
			fields:  []string{"name", "description", "state", "files", "bytes", "path"},
			run:     cmdSkillLs,
		},
		{
			path:    []string{"skill", "show"},
			kind:    "SkillShow",
			summary: "看一条技能的正文（**只读**：正文取自 `agent.SkillManager.Load` —— 与 CA 侧 `skill_load` 同一函数，构造上同字节）",
			usage:   "zerg skill show <名> [--json <字段>]",
			arity:   "any",
			args:    []string{"技能名（与 `zerg skill ls` 的 `name` 逐字相同）"},
			fields:  []string{"name", "description", "state", "path", "bytes", "lines", "body"},
			run:     cmdSkillShow,
		},
		// ---- 技能库族 · M2 三条 D2 写面（设计稿 `zerg-技能库设计稿-20261017.md` §3.3–3.5）----
		// 三态照 `doc meta fill` / `script inventory sync`：`--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 ·
		// 写面进 `<状态目录>/skill_audit.jsonl`（审计先落盘）· 写完回读对拍 sha256。
		// ✗ 零新增旗标名（`--desc` / `--from` 早就在 `valueFlagName` 表里）⇒ 门73 两处登记面零改动。
		{
			path:    []string{"skill", "new"},
			kind:    "SkillNew",
			summary: "新建一条技能（**D2 写面**：默认干跑 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 · 同名已存在 ⇒ 14 不覆盖 · 写面进审计）",
			usage:   "zerg skill new <名> (--desc <一句话触发器> | --from <件|->) [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"技能名（`^[a-z0-9][a-z0-9-]{0,63}$` · 同时是目录名）", "（`--desc` 或 `--from` 二选一：前者的那一行会进系统提示，是触发器；后者是仓外整件 · `-` = stdin）"},
			fields:  []string{"result", "name", "path", "before_sha256", "after_sha256", "bytes", "audit_path"},
			danger: &dangerSpec{dangerD2, "技能名",
				"在技能根建一个 `<名>/SKILL.md`（可逆：同名已存在即拒 14 · 写前算 sha256 · 写后回读对拍 · 审计先落盘）",
				"zerg-技能库设计稿-20261017.md §3.3 · 同族先例 doc meta fill（D2）· 写权限只给人的既有教条（ai_boundary.go:209）",
				false, dangerScopeRepo},
			opened: true,
			run:    cmdSkillNew,
		},
		{
			path:    []string{"skill", "edit"},
			kind:    "SkillEdit",
			summary: "改一条技能（**D2 写面**：`--desc` 只改描述行 / `--from <件|->` 整篇替换 · 互斥 · 默认干跑 · 缺 `--yes` ⇒ 2 · 写前备份 ＋ 审计）",
			usage:   "zerg skill edit <名> (--desc <新触发器> | --from <件|->) [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"技能名（与 `zerg skill ls` 的 `name` 逐字相同）", "改法：`--desc`（只改描述行 · 正文一字不动）或 `--from`（整篇 · `-` = stdin）"},
			fields:  []string{"result", "name", "path", "mode", "before_sha256", "after_sha256", "bytes", "backup_path", "audit_path"},
			danger: &dangerSpec{dangerD2, "技能名",
				"改 `<名>/SKILL.md`（可逆：写前备份到 `<状态目录>/skill-backup/<ts>/<名>/` + 写后回读对拍 + 审计先落盘）",
				"zerg-技能库设计稿-20261017.md §3.4 · 同族先例 doc meta fill（D2）",
				false, dangerScopeUnknown}, // 落点由 ZERG_SKILLS_DIR 决定 ⇒ 可能仓内、可能仓外（guard.go:108）
			opened: true,
			run:    cmdSkillEdit,
		},
		{
			path:    []string{"skill", "rm"},
			kind:    "SkillRm",
			summary: "下架一条技能（**D2 写面**：**不硬删** —— 整目录挪进 `<状态目录>/skill-trash/<ts>/` 可逆；默认干跑 · 缺 `--yes` ⇒ 2 · 写面进审计）",
			usage:   "zerg skill rm <名> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"技能名（= 目录名）"},
			fields:  []string{"result", "name", "path", "trash_path", "audit_path"},
			danger: &dangerSpec{dangerD2, "技能名",
				"把 `<名>/` 整目录挪进回收站（可逆：`os.Rename` 同卷即时生效；跨卷 ⇒ 照实报 1，不降级成删除）",
				"zerg-技能库设计稿-20261017.md §3.5 · 同族先例 doc meta fill（D2）",
				false, dangerScopeUnknown},
			opened: true,
			run:    cmdSkillRm,
		},
	}
}
