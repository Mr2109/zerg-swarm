package main

// 命令表 · help 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_help() []*command {
	return []*command{
		{
			path:    []string{"help"},
			summary: "帮助（主题见 `zerg help <主题>`；表在 topics.go）· `--json` 给族名清单机器面（族名 + 每族动作数）",
			usage:   "zerg help [<主题>] [--all] [--json <字段>]",
			arity:   "any",
			args:    []string{"主题（可省）"},
			// ★ 缺口 `GAP-20260927-290` / `GAP-20260927-403`：`help` 此前**无 `--json` 面**
			//   （`zerg help --all --json` ⇒ rc=2「没有 --json 字段表（它是说明面）」）⇒ 族名清单只能人读。
			//   本枚给它一档机器面：字段表两格（`family` / `actions`），产出面在 `cmdHelp` 的 `--json` 分支
			//   （`emitHelpFamilies` · 现算自命令树）。★ 必须写成 `[]string{…}` **字面量**（契约脚本的
			//   `FIELDS_RE` 只认字面量；抽成变量 = 该条被判「没有字段表」）。
			//   ★ 缺口 `GAP-20260927-335`（2026-10-10 · 本枚）：`zerg help <族> --json` 的**逐动作机器面**
			//   （某动作「是不是危险档 / 档位几 / 开没开」）—— 字段走**可选字段层**（`optFields`，与
			//   `code find` 的 `count` / `gate ls` 的 `name` 同一体例：登在出口白名单、点得出来，但**不进**
			//   上面那条 `fields:` ⇒ 人面 / `help export` 文档面一行不动、`clicFieldsRE` 照旧只认 `fields:`）。
			//   逐格：`command` = 全路径 · `danger` = 危险档位（D2/D3，**仅危险档出格**；安全档本行缺该键
			//   ⇒ 机器面「无该字段 = 非危险档」）· `opened` = 真跑开没开（`openedForRun`）。产出面见
			//   `cmdHelp` 的族名 `--json` 分支（`emitHelpFamilyActions`）。
			fields:    []string{"family", "actions"},
			optFields: []string{"command", "danger", "opened"},
			run:       cmdHelp,
		},
		{
			path:    []string{"help", "export"},
			kind:    "HelpExport",
			summary: "把命令树导出 markdown 进版本档案目录（产物 · 勿手改）；`--dry-run` 出逐条清单、真写要 `--yes`（默认档 fail-closed）",
			usage:   "zerg help export [--out <目录> | --docs-ver <X.Y.Z>] [--dry-run | --yes] [--json <字段>]",
			fields:  helpExportFields,
			// 授权侧（缺口 `GAP-20260927-140` ① · Mr2109 2026-10-08 拍「默认档要 fail-closed」）：
			// 本条**真写盘**（往版本档案目录落一件导出物）而修前无 `danger` ⇒ 默认档零授权写。
			danger:   &dangerSpec{dangerD2, "（落点目录）", "往版本档案目录落一件导出物（命令面 markdown · 同日幂等覆盖 · 仓外落点）", "GAP-20260927-140 ① · GAP-20260927-135 · 同族先例 doc meta fill（D2）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdHelpExport,
		},
	}
}
