package main

// 命令表 · repo 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_repo() []*command {
	return []*command{
		{
			path:     []string{"repo", "status"},
			kind:     "RepoStatus",
			summary:  "看仓脏没脏 / HEAD 在哪 / 有没有别人在写它（手敲 git status 的替身）· `--root` 给 ≥2 次 ⇒ **多仓汇总**（一条只读命令出两仓 HEAD + 脏件数；写旗标一律拒 2）· **三态面**：行面七栏 `head/branch/path/status/untracked/mtime/sha256`，其中 `untracked` 栏只答「**未被忽略的真未跟踪**」（**件数随仓变 · 不在这里写死任何数** —— 写了就会烂；要数就现跑一条，按行面 `path` 前缀数「`untracked`=是」的行）；**被忽略**（`.gitignore` 命中）态现读**无栏** ✗",
			usage:    "zerg repo status [--root <仓根>] [--json <字段>]",
			fields:   []string{"head", "branch", "path", "status", "untracked", "mtime", "sha256", "head_files"},
			endpoint: "",
			run:      cmdRepoStatus,
			// ★ 缺口 `GAP-20260927-14`（2026-10-14 · 真修 ①）：`total` 走**可选字段层**（`optFields`，
			//  与 `code find` 的 `count` 同一体例：登在出口白名单、`--json total` 点得出来，但**不进**
			//  上面那份 `fields:` 冻结表 ⇒ 帮助面 / 命令页一行不动、`合法字段:` 那条错误面也一字不变）。
			//  逐行落一格 `total` = 本跑**真合计处数**（与包封 `meta.total` 同源同值）。
			optFields: []string{"total"},
		},

		// ---- GAP-20260926-78（2026-10-23）：`repo uncommitted` —— **未提交件分类面**（只读 · 零写盘）
		// `git diff --stat` 对 `??` 件恒空、空输出还退 0 ⇒ 数不出「工作树有几件未提交」；
		// 本面真源 = `git status --porcelain=v1 -z -uall`，逐件给 kind/artifact/mtime/bucket/source。
		{
			path:     []string{"repo", "uncommitted"},
			kind:     "RepoUncommitted",
			summary:  "**未提交件分类面**（只读 · 缺口 `GAP-20260926-78`）：把工作树里所有未提交件逐件摊成表（kind/artifact/mtime/bucket/source），来源按现内容 sha256 对审计账 `after_sha256` 反查 ⇒ 本笔/他路/存量/来源不明四档",
			usage:    "zerg repo uncommitted [--root <仓根>] [--since <时长|RFC3339>] [--me <提案号|人>] [--audit <审计账>]… [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数：仓根由 `--root` 或 env `ZERG_REPO` / cwd 上溯定）"},
			fields:   repoUncommittedFields,
			endpoint: "",
			run:      cmdRepoUncommitted,
		},
		// ---- T20 正门一条（本枚 · **只增**）----
		// 「报行数三格」的机检面：判一份 diffstat 报没报齐 `原始 diff / 增 / 减`。
		// ★ 不退 `cmdRepo` 分派：本族 `repo status` 已是**直挂处理器**（`run: cmdRepoStatus`）⇒
		//   照它同规把 `run` 指向本枚新增的 `cmdRepoDiffstat`（零改既有分派 · 零改 `repo.go`）。
		// ★ 写旗标一律拒（`repoWriteWords`）—— 与 `repo status` 同一道只读牙（同族同守）。
		{
			path:     []string{"repo", "diffstat"},
			kind:     "RepoDiffstat",
			summary:  "报数三格机检（`原始 diff` / `增` / `减`）：三格齐 ⇒ 退 0；**只给净数（或缺任一格）⇒ 未报 ⇒ 判不了（退 2）** · 件读不到 ⇒ 判不了（退 8）· 只读",
			usage:    "zerg repo diffstat --report <报告件> [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数：报告件由 `--report <件>` 指）"},
			fields:   repoDiffstatFields,
			endpoint: "",
			run:      cmdRepoDiffstat,
		},
		// ---- D3b 第三步（2026-09-21）：**提交面**（缺口-命令面 §九 I4）----
		// 按文件名暂存（禁 `git add -A`）· 过快速档才放行 · 禁 `--no-verify` · 提交信息模板。
		{
			path:    []string{"repo", "commit"},
			kind:    "RepoCommit",
			summary: "提交：**按文件名逐件暂存**（禁 `git add -A`）· 或**点名单路径**（`--only <路径…>`：索引面允许非空、别人的暂存只许多不许少）· **过快速档才放行** · 禁 `--no-verify`（例外走 `--waive <步名> --reason <…>` 并进审计）",
			usage:   "zerg repo commit --message <题> (--file <件>… | --only <路径>[ --only <路径>]… | --removed <路径>…) [--proposal <提案 id>] [--by <谁>] [--trace <id>] [--criterion <判据>] [--waive <步名> --reason <理由>] [--allow-stale-gates <一句话理由>] [--no-fast] [--json <字段>] [--dry-run] [--yes]",
			arity:   "any",
			args:    []string{"提交主题（--message）", "逐件点名（--file · 可重复）或点名单路径（--only · 可重复 · 可 `--only=<路径>`）"},
			fields:  repoCommitFields,
			danger:  &dangerSpec{dangerD2, "提交主题", "把点名的件提交（可逆：`git reset --soft HEAD~1`）；**先跑快速档**，rc≠0 不提交（要带账放行得 `--waive <步名> --reason <…>`）", "缺口-命令面 §九 I4 · §九 M3 C5 · D3b 第三步 · 缺口 Q-104", false, dangerScopeRepo},
			// `opened`: 真跑已开放（`--yes` 就执行 —— 默认模式逐件暂存 / `--only` 模式点名单路径；D2 可逆）。
			opened:   true,
			endpoint: "",
			run:      cmdRepoCommit,
		},
	}
}
