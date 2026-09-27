// family_dev_edit.go —— 受控写面（`zerg dev edit` · D3b 第二步 · §17.3 铁律③「可受控写」）。
//
// 病灶（D3b 逐字）：七环里**唯一「缺」的那一环是「② 改」** —— 判据与取证已全在命令面，
// 而「改源码 / 写文件」今天只有「人/AI 直接编辑 + git」。本件把它变成**一条命令**，且四条硬约束：
//
//	① **作用域 = 提案声明过的件**（§17.3 铁律③①「写只落候选区」的等价物）：
//	   `--proposal <id>` 必给，`--file` 必须在那件提案的 `files[]` 里 ⇒ 没声明过的件**写不进去**（退码 2）。
//	   提案本身就是那次写的「可审查物 + 回滚路径（`rollback_ref`）」。
//	② **默认干跑**（§九 M3 `C4` 三态的第一态）：不带旗标 = `--dry-run`，只出计划件、零副作用；
//	   真写要 D3 档齐 —— `--confirm=<主机名>`（值必须与主机名逐字相同）+ `--yes`。
//	③ **逐条审计**（§九 M3 `C5`：一行一事件 · 追加只写 · **写失败即拒**）：审计行**先落盘**，
//	   落不下就不改件（「写不进日志就不许执行」）。审计记：谁 / 何时 / 改哪件 / 前后 sha256 / 字节数。
//	④ **可回滚**（§17.3 铁律③③）：件在 git 工作树里 ⇒ 回滚路径 = 提案的 `rollback_ref`（件级回滚见 `dev rollback`）。
//
// ③-a（2026-09-21 本枚）**真写要一枚人签批准件**（§17.3 铁律④③ · §九 M18 `C4`② 的落点）：
//
//	`dev edit` 能把工作树改掉这件事，等价于「AI 手里那几件写工具」的能力 —— 所以它**不再靠纪律**，
//	而是与 `approve` 族**同一套验签**（同包的 `verifyTicketState` = `control.VerifyApprovalSignature` 那一个判定口）：
//	**无件 ⇒ 拒**（退码 2）· **手写件（字段齐、没签名）⇒ 拒** · 件批的工具/范围不符 ⇒ 拒。
//	干跑（`--dry-run`）**不需要**批准件 —— 它零副作用；计划面照旧把批准件的判决打出来，好让人先看后签。
//
// ③-b（2026-09-22 本枚 · **真事故级**）**写入前语法自检**：改后的内容先过一遍机检 ——
//
//	`.py` ⇒ `python3 -c "import ast; ast.parse(...)"`（内容走 stdin，永不进 argv）；`.sh`/`.bash` ⇒ `bash -n`。
//	**不过 ⇒ 拒写**（退码 2 · 不落盘、不记审计、目标件一个字节不动）；**判不了**（解释器起不来）⇒
//	退码 8 **不给结论**（不假装检过）。别的扩展名**不适用**（不在口径里 ⇒ 不自造检查器，也不假装检过）。
//	★ 2026-09-24（`待拍清单终版-20260924.md` 条 9 · `序 130`）**扩面到 `.json` / `.yaml` / `.yml`**：
//	走**进程内**支（Go 侧 `encoding/json` ＋ `gopkg.in/yaml.v3 v3.0.1` 已在 `core/go.mod` ⇒
//	**零新依赖**、**零新命令节点**）；四类之外仍**不适用** ✗。
//	为什么：一次 `--replace` 漏闭括号它**照写** ⇒ 写上盘的是一件当场 SyntaxError 的件（改前只有
//	「回读 sha256 对拍」，那对得上恰恰证明**写坏了也照过**）。
//
// ③-c（2026-09-22 本枚 · **A4 挂干跑**）**干跑多一段影响面摘要 + 一枚钩子判决**：
//
//	干跑分支里多出：① 影响面摘要（§八 第 1–3 件）—— 人面三行**只在受影响项 ≥1 时**打
//	（零影响时打三行 = 灌噪声）；② 一枚**钩子判决**（§四.3 分档 · `R28` 裁决：「能找回的只报，
//	找不回的必拦」）—— **改 = 只记 · 符号级删 = 必拦 · 件级（整件）= 只报 + 可找回证据 /
//	不可找回三条任一命中 ⇒ 升为必拦**。判决与取数在 `family_impact_hook.go`（**只读**）。
//
//	真写路径上审计行多**两枚字段**（`impact_digest` + `impact_actual`）：前者 = 波纹摘要的 `sha256`
//	（**只留指纹，不留正文** —— §四.2 第 3 件）；后者属 `B3`（真红对拍），本批**只立字段、不填值** ✗。
//
//	**真写前置一字未动** ✗：人签批准件验签（③-a）· 审计先落盘（③）· 语法自检闸（③-b）三条判据
//	逐字不变；**钩子判决不是放行条件** ✗（卡片铁律 §3.5「提 ≠ 批」）—— 干跑里「必拦」那一档
//	**只让干跑退 2**，`R28` 的「删前波纹门」是**另开一门**（属 `C1`），本批不许塞进真写前置。
//
// 与既有条文的接缝（**不重复立项** ✗）：写面**不新立锁**（§九 M5：真源在持锁者）、**不新立退码**
// （照 §4.1 K3 那张表）、**不改契约**；本件只是「改」这一环的**唯一入口**。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// devEditFields —— `--json` 面的全部字段（K1：机器面先定）。
// ★ 2026-09-27（`GAP-20260927-216` 同族 · 本枚）：末四格 = **退点档**（**只有干跑档填**；真写档一行不加 ✗）。
//
//	病灶（上游刚报的缺口）：退建议此前**只走 stderr 人面** ⇒ `--json` 里没有退点档字段 ⇒ 脚本切不出
//	「可退 / 不可退」与退法那一行（★ 目测能用、程序用不了 ✗）。四格取值**与 stderr 那一块同源** ——
//	同一个 `devEditReturnPoint`（禁人面一套、机器面另一套 ✗ · 禁二次算 ✗）。
//	`rollback_verdict` = 能不能退（闭集 `可退` / `不可退`）· `rollback_tier` = 档位（闭集 `1`/`2`/`3`/`4`）·
//	`rollback_cmd` = 退法那一行（档 1 = `cp -p <退点件> <件>` · 档 3 = `rm <件>` · 档 2/4 **空** ⇒ 不编）·
//	`rollback_why` = 不可退的原因片段（档 1 = 空；2/3/4 逐字 = stderr 那一行里的同一段）。
var devEditFields = []string{"proposal", "file", "mode", "result", "before_sha256", "after_sha256",
	"before_bytes", "after_bytes", "audit_path", "out_path", "approval", "approver",
	"rollback_verdict", "rollback_tier", "rollback_cmd", "rollback_why"}

// replacePair —— `--replace <件>` 里的一枚替换（JSON 数组的一格）。
// 形态：`[{"old":"…","new":"…"}, …]` —— **精确、唯一命中**才许改（模糊替换会静默改错地方）。
type replacePair struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// editAuditLine —— 审计的一行（一行一事件 · 追加只写）。
type editAuditLine struct {
	At           string `json:"at"`
	Event        string `json:"event"`
	Proposal     string `json:"proposal"`
	File         string `json:"file"`
	Mode         string `json:"mode"`
	BeforeSHA256 string `json:"before_sha256"`
	AfterSHA256  string `json:"after_sha256"`
	BeforeBytes  int64  `json:"before_bytes"`
	AfterBytes   int    `json:"after_bytes"`
	By           string `json:"by"`
	Confirm      string `json:"confirm"`
	AuditPath    string `json:"audit_path"`
	Note         string `json:"note,omitempty"`
	// ③-a（2026-09-21）：**批准件**（谁签的、签的哪一件）——「谁让它改的」也要能回读。
	Approval string `json:"approval,omitempty"`
	Approver string `json:"approver,omitempty"`
	// ③-b（2026-09-22）：写入前的**语法自检判决**（过 / 不适用）—— 拒的那一格不落审计（没写就不记账）。
	Syntax string `json:"syntax,omitempty"`
	// ③-c（2026-09-22 · `A4`）：**波纹指纹两枚**（§四.2 —— 与 `impact_digest` 配对的第二枚属 `B3`）。
	//   `impact_digest`  = 波纹摘要的 `sha256`（**只留指纹，不留正文** ✗）
	//   `impact_actual`  = 真红对拍的结果（`B3` 落它 —— 本批**只立字段、不填值** ✗）
	// `omitempty`：取不到波纹（例如仓根解析不到）⇒ 这一格不写，**不写空串冒充「有了」**（「读不到」不当「没有」）。
	ImpactDigest string `json:"impact_digest,omitempty"`
	ImpactActual string `json:"impact_actual,omitempty"`
}

// cmdDevEdit —— `zerg dev edit`：受控写入的唯一入口（默认干跑）。
func cmdDevEdit(inv *invocation, stdout, stderr io.Writer) int {
	// ★ 同给判定（`GAP-20260927-376` · `--dry-run` 与确认档**同给**）：`--dry-run`（只出计划件）与
	//   `--yes`（真写）**同给** = 两道确认档自相矛盾 ⇒ 用法错 2（不给结论）。
	//   修前：本命令走下面那条合并判定 `inv.dryRun || !(inv.confirmGiven && inv.yes)` ⇒ `inv.dryRun`
	//   一真就进干跑支、**退 0**（零写入、审计也不落）—— 人给了 `--confirm=<主机名> --yes` 以为写了，
	//   实际一个字节没落：真写与计划件**退码不可区分、输出同形**（靠 sha256 回读才发现）。
	//   ★ 判定走**共享**那一处（`guard.go` 的 `dryRunYesConflict` —— `D3b` 反向对齐（2026-09-27）的
	//   **唯一共享判定**，全树 19 条真判「同给 ⇒ 2」命令同一口径）⇒ **不另写第二套**（禁同一口径两处实现 ✗）。
	//   ★ 位置 = **命令处理器入口**、在任何分支 / 任何盘面动作之前（与 `cmdGuarded` 同一形状；
	//   本命令的 `usage` 串早已声明互斥：`[--dry-run | --confirm=<本机名> --yes]`）。
	//   ★ 未同给（两枚未同时到）⇒ `dryRunYesConflict` 返回 `exitOK` ⇒ 照原样往下走
	//   （**单给任一档零行为改动**：`--dry-run` 独给仍退 0 · 确认档独给仍走下面原有的闸）。
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	proposalID := strings.TrimSpace(inv.flagVal("--proposal"))
	fileRel := strings.TrimSpace(inv.flagVal("--file"))
	fromPath := strings.TrimSpace(inv.flagVal("--from"))
	replacePath := strings.TrimSpace(inv.flagVal("--replace"))

	// ①② 用法面先判（在任何盘面动作之前）
	if proposalID == "" {
		inv.setErr("usage", "missing_proposal", "缺提案 id")
		fmt.Fprintf(stderr, "%s: `dev edit` 必须给 `--proposal <提案 id>` —— 写**只允许改提案声明过的件**（§17.3 铁律③①）\n", progName)
		fmt.Fprintf(stderr, "先看现有件：%s dev proposal list\n", progName)
		return exitUsage
	}
	if fileRel == "" {
		inv.setErr("usage", "missing_file", "缺件名")
		fmt.Fprintf(stderr, "%s: `dev edit` 要一枚 `--file <仓内相对路径>`（可重复时逐次调用 —— 一次一个件，便于逐件对账）\n", progName)
		return exitUsage
	}
	if (fromPath == "") == (replacePath == "") {
		inv.setErr("usage", "bad_content_source", "内容来源要给且只给一种")
		fmt.Fprintf(stderr, "%s: 内容来源**恰给一种**：`--from <件>`（整件替换）或 `--replace <件>`（逐格精确替换）\n", progName)
		return exitUsage
	}
	if why := declaredFilesWhy([]string{fileRel}); why != "" {
		inv.setErr("usage", "bad_file_path", why)
		fmt.Fprintf(stderr, "%s: %s\n", progName, why)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	outPath := filepath.Join(root, filepath.FromSlash(fileRel))
	if !strings.HasPrefix(outPath, root+string(os.PathSeparator)) {
		inv.setErr("usage", "file_outside_repo", "件不在仓根下")
		fmt.Fprintf(stderr, "%s: `--file %s` 解析后不在仓根下 ⇒ 拒执（越界写 · 退码 2）\n", progName, fileRel)
		return exitUsage
	}

	// ③ 作用域：提案必须存在，且**声明过**这一件
	dir := proposalDir()
	recs, err := loadProposals(dir)
	if err != nil {
		inv.setErr("blocked", "state_dir_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了提案目录 %s：%v ⇒ 不给结论（退码 8）\n", progName, dir, err)
		return exitBlocked
	}
	var prop *proposalRecord
	for i := range recs {
		if recs[i].ID == proposalID {
			prop = &recs[i]
			break
		}
	}
	if prop == nil {
		inv.setErr("usage", "proposal_not_found", "没有这个提案 id")
		fmt.Fprintf(stderr, "%s: 提案目录里没有 %q ⇒ 拒执（写必须有可审查物 ⇒ 先 `%s dev proposal new`）\n",
			progName, proposalID, progName)
		return exitUsage
	}
	if !containsStr(prop.Files, fileRel) {
		inv.setErr("usage", "file_not_declared", "这一件不在提案声明的改件清单里")
		fmt.Fprintf(stderr, "%s: **越界写被拒** —— 件 %s 不在提案 %s 声明的清单里（声明的是：%s）\n",
			progName, fileRel, proposalID, orDashList(prop.Files))
		fmt.Fprintf(stderr, "口径（§17.3 铁律③①）：写只允许改**提案声明过**的件 —— 要改这一件，先提一件声明它的提案\n")
		return exitUsage
	}
	if prop.State != "未决" && prop.State != "已批准" {
		inv.setErr("usage", "proposal_not_open", "提案已否决")
		fmt.Fprintf(stderr, "%s: 提案 %s 的状态是 %q ⇒ 拒执（已否决的件不许照它改码）\n", progName, proposalID, prop.State)
		return exitUsage
	}

	// ④ 算出「改完以后长什么样」+ 前后 sha256/字节数（不落盘的那一半）
	before, err := os.ReadFile(outPath) // 新件（还不存在）⇒ 空内容 + 记明「新建」
	beforeMissing := false
	if err != nil {
		if !os.IsNotExist(err) {
			inv.setErr("failed", "read_target_failed", err.Error())
			fmt.Fprintf(stderr, "%s: 读不了目标件 %s：%v（写失败即拒 · 不吞）\n", progName, outPath, err)
			return exitFail
		}
		beforeMissing = true
		before = []byte{}
	}
	mode := "from"
	after := []byte{}
	if fromPath != "" {
		after, err = os.ReadFile(fromPath)
		if err != nil {
			inv.setErr("usage", "content_unreadable", err.Error())
			fmt.Fprintf(stderr, "%s: 读不了内容来源 %s：%v（退码 2 · 不猜内容）\n", progName, fromPath, err)
			return exitUsage
		}
	} else {
		mode = "replace"
		after, err = applyReplacements(before, replacePath)
		if err != nil {
			inv.setErr("usage", "replace_failed", err.Error())
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			return exitUsage
		}
	}
	// ④-b **写入前语法自检**（③-b）：判的是「改完之后长什么样」，不是原件 —— 干跑也照样判，
	// 好让人先看见（计划面与真写面**同一份判决**，不是一个说绿一个说红）。
	syntaxJudg, syntaxWhy := editSyntaxCheck(fileRel, after)

	// ④-c **逐行 diff**（缺口 #1）：人签闸此前只拿到两枚 16 位短 sha + 字节数 ⇒ 看得出「变了」、
	// 看不出「改了哪几行」。这一块把**行级区间摘要**（`@@ -a,b +c,d @@` · 只行号 + 增减标记）补上 ——
	// ★ 与 sha256 **同一趟**算（`before`/`after` 就在手 ⇒ 不重读盘、不另算一遍）；★ 件正文一个字节
	// 都不进回执（进 = 泄露面 + 巨量输出，两条都犯 ✗）。同一枚 `lineDiff` 给人面两块（计划卡片 / 真写收尾）
	// 用 ⇒ 没有第二处算法。
	lineDiff := devEditLineDiffOf(before, after)

	beforeSHA := sha256Of(before)
	afterSHA := sha256Of(after)
	auditPath := editAuditPath()
	row := map[string]string{
		"proposal": proposalID, "file": fileRel, "mode": mode,
		"before_sha256": shortSHA(beforeSHA), "after_sha256": shortSHA(afterSHA),
		"before_bytes": fmt.Sprintf("%d", len(before)), "after_bytes": fmt.Sprintf("%d", len(after)),
		"audit_path": auditPath, "out_path": outPath, "syntax": syntaxJudg,
	}

	// ③-a 人签批准件（**只读**判定）：干跑也要能看见「有没有、验没验过」——
	// 这一格是**本命令的真写前置**，不是提示语（无件/手写件/范围不符 ⇒ 下面直接拒执）。
	appr := devEditLoadApproval(inv, fileRel)
	row["approval"], row["approval_path"] = appr.Judg, appr.Path

	// ⑤ 干跑（默认那一态）：计划件 + 零副作用 + **`A4` 影响面摘要与钩子判决**
	//    判据（任务单 §二 `A4`）：① 干跑零副作用（前后件字节不变 · 无审计件产生）；
	//    ② 三行**只在受影响项 ≥1 时**打；④ 批准件判据逐字未变（下面那条闸与 ③-a 一字未动）。
	if inv.dryRun || !(inv.confirmGiven && inv.yes) {
		// `A4`：钩子分档（纯判据 + 只读取数 —— 不写任何东西）。判据与取数在 `family_impact_hook.go`。
		hook := impactHookOf(root, fileRel, before, after)
		summary := impactDryRunSummaryOf(root, fileRel)
		if summary.Taken {
			row["impact_digest"] = impactDigestOf(summary.Text)
		} else {
			row["impact_digest"] = "（未取数 —— 不是「没有」）"
		}
		// 影响面摘要打 stderr（人面三行**只在受影响项 ≥1 时**打 —— 零影响时打三行 = 灌噪声）。
		emitImpactDryRunSummary(stderr, summary)
		emitImpactHookBlock(stderr, hook)
		// ★ 退建议（本件口径 · `GAP-20260927-216`）：上面那张卡片的「能退吗」档 2 给的是
		// `git revert <末笔碰本件的提交> -- <件>` —— 它**跑不通**（`git revert` 不吃 pathspec）、
		// 也**取不到改前态**（改前工作树含未提交改动 ⇒ 那一版不在任何提交里）。这一块把真能回来的
		// 那一行给出来（锚 `before_sha256` + 提案退点件），无退点可用时**明说不可退**。
		// ★ 2026-09-27（本枚）**退点档只算一次**：这一枚 `rp` **同时**给人面（下面那块）与机器面
		// （`rollback_*` 四格）用 ⇒ 两个面读**同一格**，没有第二处算法。
		rp := devEditReturnPointOf(root, prop, fileRel, beforeSHA, beforeMissing)
		row["rollback_verdict"] = rp.Verdict()
		row["rollback_tier"] = fmt.Sprintf("%d", rp.Tier)
		row["rollback_cmd"] = rp.Cmd
		row["rollback_why"] = rp.Why
		// ★ `approver` 这一格今天**悬空**：`devEditFields` 列着它、干跑 row 里却没有 ⇒ 任何 `--json`
		// 都在字段面倒下、还报「未知字段 approver」而同一行又把 approver 列成合法字段（本枚实测）。
		// 这里补上**同一枚批准件**读出的批准人（与上面「批准件」那一格同源 · 不另算）。
		// ★ **真写档一字不动** ✗（本单要求：真写档机器面逐字不变 ⇒ 只在这一支里补）。
		row["approver"] = appr.Appr
		emitDevEditRollbackAdvice(stderr, prop, fileRel, beforeSHA, len(before), rp)
		// 指纹（**摘要正文逐字进哈希** ⇒ 第三者可复算；正文本身打到上面那段，不进审计）。
		if summary.Taken {
			fmt.Fprintf(stderr, "%s: 波纹指纹 impact_digest=%s（sha256 of 上面那段摘要正文 · 第三者可复算 · §四.2 只留指纹）\n",
				progName, row["impact_digest"])
		} else {
			fmt.Fprintf(stderr, "%s: 波纹指纹 **未取数** ⇒ 干跑不写指纹（真写那一侧的审计行照「缺摘要」处置并点名）\n", progName)
		}
		// ★ 包封真值（`G-08` 已解 · **同一份取值**，不与上面两行各算一遍）：干跑摘要 + 钩子判决自本批
		// 起进 `meta` 的按需子键（`dry_run` / `impact_digest` / `impact_rows` / `impact_hook`），
		// 拿不到 / 必拦 这两类**真事**进 `warnings[]` —— 顶层仍是那六键（一个键都没加）。
		inv.metaAddJSON("dry_run", "true")
		if summary.Taken {
			inv.metaAddStr("impact_digest", row["impact_digest"])
			inv.metaAddJSON("impact_rows", fmt.Sprintf("%d", summary.Rows))
		} else {
			inv.warnf("影响面摘要**未取数**（不是「没有」）：%s —— 真写那一侧照「缺摘要」处置并点名", summary.Reason)
		}
		inv.metaAddStr("impact_hook", fmt.Sprintf("%s（类=%s）", hook.Tier, hook.Class))
		if hook.Tier == impactHookTierBlock {
			inv.warnf("钩子判决**必拦**（类=%s）：%s", hook.Class, hook.Why)
		}
		if !inv.dryRun {
			// 三态：**没带干跑旗标但确认档不齐** ⇒ 出计划件（fail-closed：从不提问、也从不偷偷写）
			row["result"] = "planned"
			if rc := emitDevEditPlan(stdout, stderr, row, prop, beforeMissing, inv, true, hook, lineDiff); rc != exitOK {
				return rc
			}
			return exitUsage
		}
		row["result"] = "planned"
		if rc := emitDevEditPlan(stdout, stderr, row, prop, beforeMissing, inv, false, hook, lineDiff); rc != exitOK {
			return rc
		}
		// 钩子判决「必拦」⇒ **这一步没通过钩子**：干跑不给放行判决（退码 2 · 与既有的
		//「缺确认档 ⇒ 2」同一档，不新立码）。**真写前置一字未动** ⇒ 这一条**只落在干跑上**
		//（`R28` 的「删前波纹门」是另开一门，属 `C1`）—— 差口在上面那一块里照实点名。
		if hook.Tier == impactHookTierBlock {
			inv.setErr("usage", "impact_hook_block", "钩子判决为必拦（"+hook.Class+"）")
			fmt.Fprintf(stderr, "%s: **钩子判决「必拦」⇒ 干跑退 2**（类=%s）：%s\n", progName, hook.Class, hook.Why)
			fmt.Fprintf(stderr, "%s: 目标件一个字节未动：%s（审计也未落 —— 没写就不记账）\n", progName, outPath)
			fmt.Fprintf(stderr, "%s: 判断口径见上面那两块（`impact_hook` / `impact_criteria` 两行可脚本切分）\n", progName)
			fmt.Fprintf(stderr, "error.kind=usage · detail=impact_hook_block · retryable=false · remedy=fix_usage\n")
			return exitUsage
		}
		return exitOK
	}
	if inv.confirm != planHost() {
		inv.setErr("usage", "confirm_mismatch", "确认值不匹配主机名")
		fmt.Fprintf(stderr, "%s: 确认值不匹配目标（--confirm 给的是 %q，本机主机名是 %q）⇒ 不执行\n",
			progName, inv.confirm, planHost())
		return exitUsage
	}

	// ⑤-b 真写的**语法闸**（③-b）：不过 ⇒ 拒写（退码 2）；判不了 ⇒ 不给结论（退码 8）。
	//     位置在批准件闸**之前**：先看「改成了什么」再看「谁批的」—— 内容本身就是废件时，
	//     让人先改内容，别先去签一枚批废件的批准件。
	if syntaxJudg == "不过" {
		inv.setErr("usage", "syntax_check_failed", "改后的内容语法不过")
		fmt.Fprintf(stderr, "%s: **语法自检不过 ⇒ 拒写**（改后的 %s 解析不了）：\n    %s\n", progName, fileRel, syntaxWhy)
		fmt.Fprintf(stderr, "口径（③-b）：写盘前先自检 —— `.py` 走 `ast.parse`、`.sh` 走 `bash -n`、`.json`/`.yaml` 走 Go 侧解析；**不过的件一个字节都不写**\n")
		fmt.Fprintf(stderr, "  目标件未动：%s（审计也未落 —— 没写就不记账）\n", outPath)
		fmt.Fprintf(stderr, "先看计划件：%s dev edit --proposal %s --file %s … --dry-run（计划面会打出同一份判决）\n", progName, proposalID, fileRel)
		fmt.Fprintf(stderr, "error.kind=usage · detail=syntax_check_failed · retryable=false · remedy=fix_usage\n")
		return exitUsage
	}
	if syntaxJudg == "不判" {
		inv.setErr("blocked", "syntax_check_unavailable", "语法自检跑不起来")
		fmt.Fprintf(stderr, "%s: **语法自检跑不起来 ⇒ 不给结论**（不假装检过）：%s\n", progName, syntaxWhy)
		fmt.Fprintf(stderr, "口径（③-b）：判不了就不写 —— 要么把解释器装上，要么改一个不适用自检的件\n")
		return exitBlocked
	}

	// ⑥ 真写的第一道闸 = **人签批准件**（无件 / 手写件 / 批的范围不含本次改件 ⇒ 退码 2）：
	//   与 `approve` 族同一条验签路（`verifyTicketState` = 消费者那一个判定口），**不另写第二套**。
	if rc := devEditRequireApproval(inv, appr, fileRel, stderr); rc != exitOK {
		return rc
	}

	// ★ 2026-09-28（`GAP-20260928-09` 同族 · 本枚）**字段面先判**（`--json <字段>` 的**名字**校验前移到写盘之前）。
	//   修前这一判**只有一个落点** —— 真写档末尾 `selectJSON(…, devEditFields, row)`（`requireFields`
	//   与 `reportBadField` 都在它里面）：件**已经写完了**才因字段面退 2 ⇒ 实测审计里照落一条
	//   `before_sha256 == after_sha256` 的 `edit` 事件（「拒执」与「真写」从盘面分不开）。★ 口径**一字不新立**：
	//   仍是 §九 M6 I5 的 `reportBadField`（同一句「未知字段 %q」+ 合法字段清单 + `See …--help`）与 `K2` 的
	//   `requireFields`（不给字段 ⇒ 2）；形状与 `family_impact.go` / `family_approve.go` / `family_agent_ops.go` /
	//   `family_route.go` / `family_gate_show.go` / `family_gate_results.go` / `family_eggs_cocoons.go` 那 8 处**逐字同形**
	//   （同一枚口 · 不另写第二套）。
	//   ★ 位置选择：落在**写盘动作之前**、而**不**提到命令入口 —— 上面那些读面分支（干跑档已自带
	//   同一枚字段面判定：`emitDevEditPlan` 传 `inv.fields`）与确认档 / 语法闸 / 批准件闸的**优先级一字不动**：
	//   干跑档倍数与天平都跟修前**逐字同值**，变的只有一件事 —— 真写档不再先写后报。
	//   第一个真写动作 = 下面的 `appendEditAudit`（审计先落盘）⇒ 本块在它之前 ✓（本条下面的两道门都只读）。
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		for _, f := range inv.fields {
			ok := false
			for _, l := range fieldListOf(inv.path) {
				if l == f {
					ok = true
					break
				}
			}
			if !ok {
				inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
				return reportBadField(stderr, inv.path, f)
			}
		}
		// ★ 2026-09-28（`GAP-20260928-09` **残留** · 本枚）**本档必不产出的字段名 ⇒ 写前拒**。
		//   上面那一段只判「名字在不在命令的字段表里」：`approver` / `rollback_verdict` / `rollback_tier` /
		//   `rollback_cmd` / `rollback_why` 五格**在表里**（`devEditFields` 列着），却只由**干跑档**填
		//   （退点四格见干跑支的 `devEditReturnPoint`、`approver` 见干跑支那一行）⇒ 真写档的 row 里没有它们，
		//   收口落在末尾 `selectJSON(…, inv.fields, row)` 里的 `marshalObject` 上 ⇒ **件已经写完了**才退 2
		//   （与修前同病：同一发「既写了件、又退 2 报 usage」）。
		//   口径**一字不新立**：判据就是 `--json` 出口那一枚 `marshalObject` —— 这里拿**本档 row 的原样**先试
		//   一次投影，试出 `bad` 就用**同一枚** `reportBadField`（逐字同句）报，只把位置挪到第一个真写动作
		//   （下面 `appendEditAudit`）之前。`result` 那一格本档**确实产出**（写成功后置 `written`）⇒
		//   探针里照置，好字段名**不误伤**（`proposal` / `file` / `mode` / 两枚 sha / 两枚字节 / 审计与目标路径 /
		//   `approval` / `approval_path` / `syntax` 与 `result` 全在 probe 里 ⇒ 逐字照旧）。
		probe := make(map[string]string, len(row)+1)
		for k, v := range row {
			probe[k] = v
		}
		probe["result"] = "written"
		if _, bad := marshalObject(inv.fields, probe); bad != "" {
			fmt.Fprintf(stderr, "%s: 本档（真写）不产出字段 %q ⇒ **写前拒**（未落盘、未记审计）\n", progName, bad)
			inv.setErr("usage", "json_field_unknown:"+bad, fmt.Sprintf("未知字段 %q", bad))
			return reportBadField(stderr, inv.path, bad)
		}
	}

	// ⑦ 真写：**审计先落盘**（写不进日志就不许执行 · §九 M3 C5）⇒ 再写件 ⇒ 再自检回读 sha256
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = prop.Subject
	}
	if by == "" {
		by = "（未声明）"
	}
	// ⑥-b（`A4`）**波纹指纹**：取一次影响面摘要（**只读** · 默认档），把它的 `sha256` 记进审计行 ——
	// **只留指纹、不留正文** ✗（§四.2 第 3 件）。**未取到数** ⇒ 那一格不写（`omitempty`）+ stderr 点名
	//（「读不到」不当「没有」）；**一律不拦写**（`impact_digest` **不是放行条件** ✗ —— `R14` 只拍「先只记」那一步）。
	impactSum := impactDryRunSummaryOf(root, fileRel)
	digest := ""
	if impactSum.Taken {
		digest = impactDigestOf(impactSum.Text)
	}
	line := editAuditLine{
		At: time.Now().Format(time.RFC3339), Event: "edit", Proposal: proposalID, File: fileRel,
		Mode: mode, BeforeSHA256: beforeSHA, AfterSHA256: afterSHA,
		BeforeBytes: int64(len(before)), AfterBytes: len(after), By: by,
		Confirm: inv.confirm, AuditPath: auditPath,
		Approval: appr.Path, Approver: appr.Appr, Syntax: syntaxJudg,
		ImpactDigest: digest,
	}
	if beforeMissing {
		line.Note = "新建件（写前不存在）"
	}
	if err := appendEditAudit(auditPath, line); err != nil {
		inv.setErr("failed", "audit_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: **审计落不下盘 ⇒ 拒执**（§九 M3 C5「写不进日志就不许执行」）：%v\n", progName, err)
		return exitFail
	}
	if digest == "" {
		fmt.Fprintf(stderr, "%s: ★ 波纹指纹**取不到**（`impact_digest` 那一格没写 —— 「读不到」不当「没有」）：%s\n",
			progName, impactSum.Text)
		fmt.Fprintf(stderr, "%s: 包封的 `warnings[]` 今天恒 `[]` ⇒ 「缺摘要要点名」这一格没有落点（CLI 缺口，`A4` 再点一次名）\n", progName)
	} else {
		fmt.Fprintf(stderr, "%s: 波纹指纹 impact_digest=%s（摘要正文**不进审计** · §四.2 第 3 件；%s）\n",
			progName, digest, impactSum.Reason)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		inv.setErr("failed", "mkdir_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 建不了目录 %s：%v（审计已留痕：%s）\n", progName, filepath.Dir(outPath), err, auditPath)
		return exitFail
	}
	// 序138 的「先复原再报」：**动手写之前**把这一件的现盘样子登记进本趟的墙钟前像本
	// （到点 ⇒ `wallclockRestoreAll` 逐件还原；写前不在盘 ⇒ 到点删残留）。
	// 登记失败不当失败（它只是「到点时多做一件事」的那一格 —— 与 `impact_digest` 取不到同口径：照实不拦）。
	if werr := wallclockRecordPreImage(outPath); werr != nil {
		fmt.Fprintf(stderr, "%s: 写面前像登记不上（%v）⇒ 本件到点时复原不了，照实记一行\n", progName, werr)
	}
	if err := os.WriteFile(outPath, after, 0o644); err != nil {
		inv.setErr("failed", "write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写不进 %s：%v（审计已留痕：%s）\n", progName, outPath, err, auditPath)
		return exitFail
	}
	back, err := os.ReadFile(outPath)
	if err != nil || sha256Of(back) != afterSHA {
		inv.setErr("failed", "writeback_mismatch", "回读与写前算出的 sha256 不一致")
		fmt.Fprintf(stderr, "%s: **回读对拍不一致**（写的是 %s，盘上读到的不一样）⇒ 报失败、不报成功\n", progName, shortSHA(afterSHA))
		return exitFail
	}
	row["result"] = "written"
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		// ★ 投影表 = **用户点名的那几个**（`inv.fields`）—— 与干跑档同一个口（`emitDevEditPlan` 那一处）。
		//   原写法传整张 `devEditFields` ⇒ row 里真写档没有的几格（`approver` / `rollback_*`）把任何 `--json` 在**字段面**
		//   上就顶倒（每一发都是「写完了再退 2」）。字段名的合法性**已在上面那一段判过**。
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, row)
	}
	fmt.Fprintf(stdout, "%s	%s	%s	%s\n", fileRel, shortSHA(beforeSHA), shortSHA(afterSHA), fmt.Sprintf("%d 字节", len(after)))
	// ★ 逐行 diff（缺口 #1）：真写这一档同样给「改了哪几行」——走 **stderr**（人面），
	// stdout 那一行（逐字四格记录）**一个字节不动** ✗（脚本按位切它）。
	emitDevEditLineDiff(stderr, "", lineDiff)
	fmt.Fprintf(stderr, "%s: 已改 %s（提案 %s · 审计 %s）\n", progName, outPath, proposalID, auditPath)
	fmt.Fprintf(stderr, "回滚路径：提案 %s 的退点 = %s（件级回滚见 `%s dev rollback`）\n", proposalID, orDash(prop.Rollback), progName)
	return exitOK
}

// editSyntaxProbe（**外部检查器支**）—— 语法自检的口径之一（扩展名 → 检查器 argv；内容一律走
// **stdin**，永不进 argv）。为什么不按「脚本里有没有 shebang」猜：判据要**机械可判** —— 只看扩展名这一件事。
var editSyntaxProbe = map[string][]string{
	".py":   {"python3", "-c", "import ast, sys; ast.parse(sys.stdin.read())"},
	".sh":   {"bash", "-n"},
	".bash": {"bash", "-n"},
}

// ★ 2026-09-24（`待拍清单终版-20260924.md` 条 9 · `序 130` · 设计 `v1.2` §5 `O-16` 邻位）：语法自检
// **扩面到 `.json` / `.yaml`**。
//
// 为什么走这一支（**进程内**）而不是再往上面那张表加行：`.json` / `.yaml` 的检查器在 Go 侧**已是现成
// 依赖**（`encoding/json` 标准库 · `gopkg.in/yaml.v3 v3.0.1` 已在 `core/go.mod` 的 `require` 块 ⇒
// **零新依赖**）；而走 argv 表就得先造一枚**子命令入口**，那是**新命令节点** ⇒ 命令树 127→128 ＋
// 矩阵随动 ＋ 契约重冻 —— 给一件小检查器换来一整轮契约动作 ✗。⇒ 照 `O-16` 代价栏那个括号里的两支，
// 取**「那张表加一支非命令串」**这一支。
//
// 返回 `nil` = 解析得动（判决「过」）；非 `nil` = `Error()` 原文照转（判决「不过」，逐字不润色）。
var editSyntaxInProc = map[string]func([]byte) error{
	".json": func(b []byte) error {
		var v any
		return json.Unmarshal(b, &v)
	},
	".yaml": yamlParse,
	".yml":  yamlParse,
}

// yamlParse —— `.yaml` / `.yml` 的进程内检查器（`gopkg.in/yaml.v3`）。
//
// 为什么逐文档走 `Decoder` 而不是 `yaml.Unmarshal(..., &any)`：`.yaml` 顶层可以是**多文档**（`---`
// 分段）⇒ 单文档解码会**漏掉第二份起的坏语法**（判成「过」＝假绿）；逐文档解到 `io.EOF` 才与
// 「这一件能不能整件解析」同义。
func yamlParse(b []byte) error {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	for {
		var n yaml.Node
		err := dec.Decode(&n)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// editSyntaxCheck —— 写入前的**语法自检**（③-b）。返回三态判决：
//
//	"过"     —— 检查器跑完且没报（`.py` 解析得动 / `.sh` 语法对 / `.json`·`.yaml` 解得出）
//	"不适用" —— 扩展名不在口径里（本仓可机检的四类之外 ⇒ 不自造检查器，也不假装检过）
//	"不过"   —— 检查器报了（why = 原文，逐字转出，不润色）
//	"不判"   —— 检查器起不来（why = 为什么）⇒ 调用方按**不给结论**处理（不写）
//
// ★ 2026-09-24（条 9 · `序 130`）：**两支口径**——先查**进程内**支（`editSyntaxInProc`：`.json` /
// `.yaml` / `.yml`）再查**外部检查器**支（`editSyntaxProbe`：`.py` / `.sh` / `.bash`）。进程内支
// **没有「不判」这一态**（不依赖任何外部解释器 ⇒ 「起不来」这件事不存在）；其余三态语义**一字未动** ✗。
//
// 内容走 stdin：`.py` 的源码里有引号/反斜杠是常态，拼进 `-c` 的字符串就是**二次转义**的坑；
// stdin 是逐字节的，内容一个字都不改。
func editSyntaxCheck(fileRel string, content []byte) (judg, why string) {
	ext := strings.ToLower(filepath.Ext(fileRel))
	if parse, ok := editSyntaxInProc[ext]; ok {
		if err := parse(content); err != nil {
			return "不过", strings.TrimSpace(err.Error())
		}
		return "过", ""
	}
	probe, ok := editSyntaxProbe[ext]
	if !ok {
		return "不适用", ""
	}
	cmd := exec.Command(probe[0], probe[1:]...)
	cmd.Stdin = bytes.NewReader(content)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if _, isExit := err.(*exec.ExitError); isExit {
			msg := strings.TrimSpace(errb.String())
			if msg == "" {
				msg = err.Error()
			}
			return "不过", msg
		}
		return "不判", err.Error()
	}
	return "过", ""
}

// emitDevEditPlan —— 计划件（干跑与「确认档不齐」两条路共用；零副作用）。
// blocked=true 时，最后一行点明为什么没执行（三态里 fail-closed 的那一格）。
// `A4` 起多一行**钩子判决**（`hook`）：它是**计划的一部分**（这一步的钩子是哪一档），
// 但**不是放行条件** ✗（§3.5 卡片铁律「提 ≠ 批」）。
// ★ 本枚起多一块**逐行 diff**（`lineDiff` · 缺口 #1）：人签闸要能看见「改了哪几行」——
// 只给行号与增减标记，件正文不进这一块（与 `before`/`after` 同一趟算出，见调用点）。
func emitDevEditPlan(stdout, stderr io.Writer, row map[string]string, prop *proposalRecord,
	beforeMissing bool, inv *invocation, blocked bool, hook impactHook, lineDiff devEditLineDiffBlock) int {
	fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未写任何文件、未改任何状态）")
	fmt.Fprintf(stdout, "  动作     : %s dev edit（受控写 · 只改提案声明过的件）\n", progName)
	fmt.Fprintf(stdout, "  提案     : %s（状态 %s · 声明改件 %s）\n", prop.ID, prop.State, orDashList(prop.Files))
	fmt.Fprintf(stdout, "  目标件   : %s%s\n", row["file"], ifStr(beforeMissing, "（**新建**：写前不存在）", ""))
	fmt.Fprintf(stdout, "  内容来源 : %s（mode=%s）\n", ifStr(row["mode"] == "from", "--from <件>", "--replace <件>"), row["mode"])
	fmt.Fprintf(stdout, "  前后 sha : %s → %s\n", row["before_sha256"], row["after_sha256"])
	fmt.Fprintf(stdout, "  前后字节 : %s → %s\n", row["before_bytes"], row["after_bytes"])
	emitDevEditLineDiff(stdout, "  ", lineDiff)
	fmt.Fprintf(stdout, "  审计落点 : %s（一行一事件 · 追加只写 · **写不进审计就不改件**）\n", row["audit_path"])
	fmt.Fprintf(stdout, "  批准件   : %s —— %s\n", row["approval_path"], row["approval"])
	fmt.Fprintf(stdout, "  语法自检 : %s%s\n", row["syntax"], ifStr(row["syntax"] == "不过", " —— **真写会被拒**（`.py`=ast.parse / `.sh`=bash -n / `.json`·`.yaml`=Go 侧解析；先改内容）", ""))
	fmt.Fprintf(stdout, "  影响面钩子: %s（类=%s · §4.3 分档 —— 判据见 stderr 的 `impact_hook` 那两行）\n", hook.Tier, hook.Class)
	fmt.Fprintf(stdout, "  回滚路径 : %s（提案的退点件 —— **退建议见 stderr 那一块**：判据 = 回到 before_sha256；`git revert` 不算退点）\n", orDash(prop.Rollback))
	if blocked {
		fmt.Fprintf(stdout, "  未执行   : D3 档确认不齐 —— 要 `--confirm=%s --yes` 同时到（fail-closed：从不提问）\n", planHost())
	} else if hook.Tier == impactHookTierBlock {
		fmt.Fprintf(stdout, "  未执行   : 钩子判决「必拦」（类=%s）⇒ 干跑退 2 · **真写前置一字未动**（差口见 stderr 那一块）\n", hook.Class)
	}
	if inv.jsonGiven {
		// ★ 2026-09-27（本枚）：投影表传 `inv.fields`（**用户点名的那几个**）而不是 `devEditFields` ——
		// 原写法把整张字段表当投影表 ⇒ ① 用户点名的字段被**忽略**；② 任何 `--json` 都因 row 缺
		// `approver` 在**字段面**就倒下（`--json proposal` 也报「未知字段 approver」· 实测 rc=0 且无包封）。
		// 口径与同族单件出口一致（`family_impact.go:294` 传 `inv.fields`）。rc **不再丢**（见下面 `return exitOK`）。
		if rc := selectJSON(stdout, stderr, inv, inv.path, inv.fields, row); rc != exitOK {
			return rc
		}
	}
	if blocked {
		fmt.Fprintf(stderr, "%s: `dev edit` 是 D3 档（改仓内件）——**缺 `--confirm=<主机名>` 或 `--yes` ⇒ 不执行**（退码 2）\n", progName)
		return exitOK
	}
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未写任何文件）")
	return exitOK
}

// ---- ③-a 人签批准件（与 `approve` 族同一套验签 · 不另立第二套）----

// devEditApproval —— 一枚批准件的**判定结果**（人面一句话 + 机器面两个字段）。
type devEditApproval struct {
	Path string
	OK   bool
	Judg string // 判决（拒因逐字给出 —— 错误文案要给下一步 · K14）
	Appr string // 批准人（验过时才有）
}

// devEditApprovalTool —— 本命令向人要的那枚批准件的工具名（件名 = `<状态目录>/approvals/<工具名>.json`）。
const devEditApprovalTool = "dev_edit"

// devEditLoadApproval —— **只读**判定：读批准件 ⇒ 验签 ⇒ 核对工具名与范围。
// fail-closed 的四格（都对「不算批准」这一侧收）：件不在 / 件读不动 / 签名验不过（含**手写件**）/
// 工具名或范围不符。**读不到不许当通过**（与 `approve show` 的判决同源）。
func devEditLoadApproval(inv *invocation, fileRel string) devEditApproval {
	wantTool := strings.TrimSpace(inv.flagVal("--approval-tool"))
	if wantTool == "" {
		wantTool = devEditApprovalTool
	}
	path := strings.TrimSpace(inv.flagVal("--approval"))
	if path == "" {
		path = filepath.Join(approveDir(), wantTool+".json")
	}
	a := devEditApproval{Path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		a.Judg = "无件（真写要一枚人签批准件）"
		return a
	}
	var tk approvalTicketFile
	if err := json.Unmarshal(b, &tk); err != nil {
		a.Judg = "件读不动（不算批准）：" + err.Error()
		return a
	}
	if st := verifyTicketState(tk); st != "验过" {
		a.Judg = st // 「无签名（不算批准）」= 手写件那一格，逐字转出，不润色
		return a
	}
	if tk.Tool != wantTool {
		a.Judg = fmt.Sprintf("件批的是 %q，不是 %q（不算批准）", tk.Tool, wantTool)
		return a
	}
	if !approvalScopeCovers(tk.Scope, fileRel) {
		a.Judg = fmt.Sprintf("件批的范围 %q 不含本次改的件 %s（不算批准）", tk.Scope, fileRel)
		return a
	}
	a.OK, a.Appr = true, tk.Approver
	a.Judg = fmt.Sprintf("验过（%s · %s · key_id=%s）", tk.Approver, tk.ApprovedAt, tk.KeyID)
	return a
}

// approvalScopeCovers —— 批准件的 `scope` 覆盖不覆盖这一件：`*`（全部）· 逐字同路径 ·
// 以 `/` 结尾的目录前缀（如 `core/cmd/zerg/`）。分隔符认 逗号/分号/空白 —— 逗号分隔的清单逐条比。
// 口径**只认这三种**：不做模糊匹配（模糊范围 = 等于没范围）。
func approvalScopeCovers(scope, fileRel string) bool {
	for _, s := range strings.FieldsFunc(scope, func(r rune) bool {
		return r == ',' || r == ';' || r == '；' || r == ' ' || r == '\t'
	}) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if s == "*" || s == fileRel {
			return true
		}
		if strings.HasSuffix(s, "/") && strings.HasPrefix(fileRel, s) {
			return true
		}
	}
	return false
}

// devEditRequireApproval —— 真写的那道闸：不齐 ⇒ 退 2（并把「下一步」给人 —— 怎么签那枚件）。
func devEditRequireApproval(inv *invocation, a devEditApproval, fileRel string, stderr io.Writer) int {
	if a.OK {
		fmt.Fprintf(stderr, "%s: 批准件 %s ⇒ %s\n", progName, a.Path, a.Judg)
		return exitOK
	}
	inv.setErr("usage", "approval_required", "真写要一枚人签批准件（无件/手写件/范围不含 ⇒ 拒）")
	fmt.Fprintf(stderr, "%s: **真写要一枚人签批准件** —— 与 `approve` 族同一套验签（§17.3 铁律④③ · §九 M18 C4②）\n", progName)
	fmt.Fprintf(stderr, "  批准件落点：%s\n", a.Path)
	fmt.Fprintf(stderr, "  判决      ：%s\n", a.Judg)
	fmt.Fprintf(stderr, "  下一步（**批准要人亲自在终端上敲**，模型够不着）：%s approve new --tool %s --scope %s --by <人名> --note <理由>\n",
		progName, devEditApprovalTool, fileRel)
	return exitUsage
}

// applyReplacements —— 逐格精确替换（每一格的 `old` 必须**恰好命中一次**；否则拒执 —— 不猜、不模糊）。
func applyReplacements(before []byte, replacePath string) ([]byte, error) {
	body, err := os.ReadFile(replacePath)
	if err != nil {
		return nil, fmt.Errorf("读不了替换件 %s：%v", replacePath, err)
	}
	var pairs []replacePair
	if err := json.Unmarshal(body, &pairs); err != nil {
		return nil, fmt.Errorf("替换件 %s 不是 `[{\"old\":…,\"new\":…}]` 形态：%v", replacePath, err)
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("替换件 %s 是空数组 —— 空替换 = 什么也不改 ⇒ 拒执（不给结论）", replacePath)
	}
	cur := string(before)
	for i, p := range pairs {
		if p.Old == "" {
			return nil, fmt.Errorf("第 %d 格的 `old` 是空的 ⇒ 拒执（空模式会命中任意位置 —— 那是模糊替换）", i+1)
		}
		n := strings.Count(cur, p.Old)
		if n != 1 {
			return nil, fmt.Errorf("第 %d 格的 `old` 在件里命中 %d 次（要**恰好 1 次**）⇒ 拒执："+
				"改成更长的上下文，别用会命中多处的模式", i+1, n)
		}
		cur = strings.Replace(cur, p.Old, p.New, 1)
	}
	return []byte(cur), nil
}

// editAuditPath —— 审计落点：`ZERG_EDIT_AUDIT` > `<ZERG_STATE_DIR>/edit_audit.jsonl` > `~/.zerg/state/edit_audit.jsonl`。
func editAuditPath() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_EDIT_AUDIT")); d != "" {
		return d
	}
	if d := strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")); d != "" {
		return filepath.Join(d, "edit_audit.jsonl")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zerg", "state", "edit_audit.jsonl")
}

// appendEditAudit —— 追加一行（O_APPEND · 一行一事件）。**失败即拒**（调用方据此不改件）。
func appendEditAudit(path string, line editAuditLine) error {
	if path == "" {
		return fmt.Errorf("审计落点解析不出来（HOME / ZERG_STATE_DIR 都取不到）")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(line)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// sha256Of / shortSHA —— 现算 sha256（审计与对拍都读它，不缓存、不估计）。
func sha256Of(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func shortSHA(s string) string {
	if len(s) <= 16 {
		return s
	}
	return s[:16]
}

// orDashList / ifStr —— 两枚只在报面上用的小工具（空清单 ⇒ `（未声明）`）。
func orDashList(v []string) string {
	if len(v) == 0 {
		return "（未声明）"
	}
	return strings.Join(v, " · ")
}

func ifStr(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

// ---- 逐行 diff（缺口 #1：写面人签闸此前只看得到两枚短 sha + 字节数）----
//
// 口径（每条都硬）：
//
//	① **只给行号与增减标记**（`@@ -a,b +c,d @@` + `+N -M` 行数）—— **件正文一个字节都不进回执**
//	   （正文进 = 泄露面 + 巨量输出，两条都犯 ✗）。逐行级可见 = 人拿 hunk 头 + 行号就能落到具体行。
//	② **与 sha256 同一趟算**（`before` / `after` 就在手）—— 不重读盘、不另算一遍（禁二次算 ✗）。
//	③ 算法 = **线性空间 LCS**（Hirschberg：前向 / 反向两排 DP 取中点切分 ⇒ 内存 O(列数)），
//	   所以增减行数与 hunk 边界是**精确**的。件大到超预算（`devEditDiffCellBudget` 格）⇒ 退**粗档**，
//	   并**照实标注「粗档」**（不说成精确 · 禁假装算过 ✗）。
type devEditLineDiffBlock struct {
	Added   int      // 逐行新增行数（粗档 = 中段整段，见 Coarse）
	Removed int      // 逐行删除行数（同上）
	Hunks   []string // `@@ -a,b +c,d @@`（只行号 · 正文永不进）
	Coarse  bool     // 粗档（行×行 超预算 ⇒ 只给首尾裁剪后的中段范围）
	More    int      // 因 `devEditDiffMaxHunks` 截掉、没列出的处数
	Same    bool     // 逐字节相同 ⇒ 真写是空动作
}

const (
	// devEditDiffContext —— hunk 头两侧留几行上下文（**只用于合并相邻改动** · 不输出正文）。
	devEditDiffContext = 3
	// devEditDiffCellBudget —— LCS 预算（老行数 × 新行数 的格数）：超了退粗档。
	devEditDiffCellBudget = 4000000
	// devEditDiffMaxHunks —— 回执最多列几处（多了只给处数与总数 —— 回执不灌巨量输出）。
	devEditDiffMaxHunks = 12
)

// devEditDiffOp —— 一条对齐操作（`=` 两侧同 / `-` 只在老 / `+` 只在新）。
type devEditDiffOp struct {
	Kind byte
	Old  int // 老侧 0 基行号（`+` 时 = 插在这儿的老侧位置）
	New  int // 新侧 0 基行号（`-` 时 = 删掉后新侧的对位）
}

// devEditLineDiffOf —— 算人面那一块逐行 diff（**纯函数 · 不碰盘 · 不吐正文**）。
func devEditLineDiffOf(before, after []byte) devEditLineDiffBlock {
	d := devEditLineDiffBlock{}
	if string(before) == string(after) {
		d.Same = true
		return d
	}
	oldLines := devEditSplitLines(before)
	newLines := devEditSplitLines(after)
	if len(oldLines)*len(newLines) > devEditDiffCellBudget {
		// 粗档：首尾裁剪 + 中段整段替换（**照实标注** —— 这不是精确逐行计数）。
		p := 0
		for p < len(oldLines) && p < len(newLines) && oldLines[p] == newLines[p] {
			p++
		}
		s := 0
		for s < len(oldLines)-p && s < len(newLines)-p &&
			oldLines[len(oldLines)-1-s] == newLines[len(newLines)-1-s] {
			s++
		}
		d.Coarse = true
		d.Removed = len(oldLines) - p - s
		d.Added = len(newLines) - p - s
		if d.Removed > 0 || d.Added > 0 {
			d.Hunks = append(d.Hunks, devEditHunkHeader(p, d.Removed, p, d.Added))
		}
		return d
	}
	added, removed, hunks := devEditHunksOf(devEditDiffOps(oldLines, newLines))
	d.Added, d.Removed = added, removed
	if len(hunks) > devEditDiffMaxHunks {
		d.More = len(hunks) - devEditDiffMaxHunks
		hunks = hunks[:devEditDiffMaxHunks]
	}
	d.Hunks = hunks
	return d
}

// emitDevEditLineDiff —— 打人面那一块（indent = 缩进；只给行号与标记 ✗ 正文）。
func emitDevEditLineDiff(w io.Writer, indent string, d devEditLineDiffBlock) {
	switch {
	case d.Same:
		fmt.Fprintf(w, "%s逐行 diff : **逐行无差**（件逐字节相同 ⇒ 真写是空动作，没有一行要复核）\n", indent)
		return
	case d.Coarse:
		fmt.Fprintf(w, "%s逐行 diff : **粗档**（老行×新行 超预算 %d 格 ⇒ 只给首尾裁剪后的**中段范围**，逐行计数不给精确值）：老 -%d 行 / 新 +%d 行\n",
			indent, devEditDiffCellBudget, d.Removed, d.Added)
	case d.Added == 0 && d.Removed == 0:
		fmt.Fprintf(w, "%s逐行 diff : **逐行无差**（行内容逐行相同 —— 差在字节面，例如末行换行 / 行尾空白）⇒ 复核落到 sha256 对拍\n", indent)
		return
	default:
		fmt.Fprintf(w, "%s逐行 diff : +%d -%d 行 · %d 处（只给行号与增减标记 —— **件正文不进回执** ✗）\n",
			indent, d.Added, d.Removed, len(d.Hunks)+d.More)
	}
	for _, h := range d.Hunks {
		fmt.Fprintf(w, "%s  %s\n", indent, h)
	}
	if d.More > 0 {
		fmt.Fprintf(w, "%s  …还有 %d 处未列（本块上限 %d 处 —— 要逐行细看请在本机 `git diff`；本块只给行号）\n",
			indent, d.More, devEditDiffMaxHunks)
	}
}

// devEditHunkHeader —— hunk 头（只行号与计数）。纯插入 / 纯删除那一侧按统一 diff 惯例写 0 基起点。
func devEditHunkHeader(oldStart, oldCount, newStart, newCount int) string {
	if oldCount == 0 {
		oldStart--
	}
	if newCount == 0 {
		newStart--
	}
	return fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, oldCount, newStart, newCount)
}

// devEditSplitLines —— 按 `\n` 切行（**末尾换行不算一行** —— 与「行数」的人面直觉对齐）。
func devEditSplitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	lines := strings.Split(string(b), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// devEditDiffOps —— 对齐摊平：Hirschberg 递归出来的 ops 顺序即行序，这里补上两侧行号。
func devEditDiffOps(a, b []string) []devEditDiffOp {
	var raw []devEditDiffOp
	devEditLCSOps(a, b, &raw)
	ops := make([]devEditDiffOp, 0, len(raw))
	oi, ni := 0, 0
	for _, op := range raw {
		switch op.Kind {
		case '=':
			ops = append(ops, devEditDiffOp{'=', oi, ni})
			oi++
			ni++
		case '-':
			ops = append(ops, devEditDiffOp{'-', oi, ni})
			oi++
		default:
			ops = append(ops, devEditDiffOp{'+', oi, ni})
			ni++
		}
	}
	return ops
}

// devEditLCSOps —— Hirschberg（线性空间 LCS）递归：把 a↔b 的对齐按行序摊平成 ops。
func devEditLCSOps(a, b []string, out *[]devEditDiffOp) {
	switch {
	case len(a) == 0:
		for j := range b {
			*out = append(*out, devEditDiffOp{'+', 0, j})
		}
	case len(b) == 0:
		for i := range a {
			*out = append(*out, devEditDiffOp{'-', i, 0})
		}
	case len(a) == 1:
		k := -1
		for j := range b {
			if b[j] == a[0] {
				k = j
				break
			}
		}
		if k < 0 {
			*out = append(*out, devEditDiffOp{'-', 0, 0})
			for j := range b {
				*out = append(*out, devEditDiffOp{'+', 0, j})
			}
			return
		}
		for j := 0; j < k; j++ {
			*out = append(*out, devEditDiffOp{'+', 0, j})
		}
		*out = append(*out, devEditDiffOp{'=', 0, k})
		for j := k + 1; j < len(b); j++ {
			*out = append(*out, devEditDiffOp{'+', 0, j})
		}
	default:
		mid := len(a) / 2
		f := devEditLCSRow(a[:mid], b)
		r := devEditLCSRow(devEditReversed(a[mid:]), devEditReversed(b))
		best, k := -1, 0
		for j := 0; j <= len(b); j++ {
			if v := f[j] + r[len(b)-j]; v > best {
				best, k = v, j
			}
		}
		devEditLCSOps(a[:mid], b[:k], out)
		devEditLCSOps(a[mid:], b[k:], out)
	}
}

// devEditLCSRow —— 一排 LCS 长度（内存 O(len(b))）：返回「a vs b 各前缀」的结尾那一排。
func devEditLCSRow(a, b []string) []int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := range a {
		for j := range b {
			switch {
			case a[i] == b[j]:
				cur[j+1] = prev[j] + 1
			case prev[j+1] >= cur[j]:
				cur[j+1] = prev[j+1]
			default:
				cur[j+1] = cur[j]
			}
		}
		prev, cur = cur, prev
		for j := range cur {
			cur[j] = 0
		}
	}
	return prev
}

func devEditReversed(s []string) []string {
	out := make([]string, len(s))
	for i := range s {
		out[i] = s[len(s)-1-i]
	}
	return out
}

// devEditHunksOf —— ops ⇒ (增行数, 删行数, hunk 头)：老侧间隔 ≤ 2×上下文的两处改动合一个 hunk。
func devEditHunksOf(ops []devEditDiffOp) (added, removed int, hunks []string) {
	var changed []int
	for i, op := range ops {
		switch op.Kind {
		case '+':
			added++
			changed = append(changed, i)
		case '-':
			removed++
			changed = append(changed, i)
		}
	}
	for i := 0; i < len(changed); {
		j := i
		for j+1 < len(changed) && devEditGapWithin(ops, changed[j], changed[j+1]) {
			j++
		}
		s, e := changed[i], changed[j]
		for n := 0; n < devEditDiffContext && s > 0 && ops[s-1].Kind == '='; n++ {
			s--
		}
		for n := 0; n < devEditDiffContext && e+1 < len(ops) && ops[e+1].Kind == '='; n++ {
			e++
		}
		oldCount, newCount := 0, 0
		for k := s; k <= e; k++ {
			if ops[k].Kind != '+' {
				oldCount++
			}
			if ops[k].Kind != '-' {
				newCount++
			}
		}
		hunks = append(hunks, devEditHunkHeader(ops[s].Old+1, oldCount, ops[s].New+1, newCount))
		i = j + 1
	}
	return added, removed, hunks
}

// devEditGapWithin —— 两条改动之间只隔着 ≤ 2×上下文的**未改行** ⇒ 算同一处（否则拆两处）。
func devEditGapWithin(ops []devEditDiffOp, i, j int) bool {
	same := 0
	for k := i + 1; k < j; k++ {
		if ops[k].Kind == '=' {
			same++
		}
	}
	return j-i-1 == same && same <= 2*devEditDiffContext
}

// ---- ★ 退建议（本件口径 · `GAP-20260927-216` · 2026-09-27 本枚）--------------------------------
//
// 病灶（**上游实测坐实**，不是「不推荐」）：干跑那张卡片的「能退吗」档 2（`family_impact_card.go`
// `impactReversibilityDecide`）给的退法是 `git revert <末笔碰本件的提交> -- <件>`。三条错：
//
//	① **跑不通**：`git revert` **不吃 pathspec** ⇒ `git revert <sha> -- <件>` 实测
//	   `fatal: bad revision '<件>'`（rc=128）；
//	② **取不到改前态**：就算去掉后半截，revert 回到的是**提交记录里**那一版；而「改前工作树」
//	   含**未提交**改动 ⇒ 那一版**不在任何提交里**（`HEAD:<件>` ≠ 改前工作树）⇒ 照它跑完 sha ≠ 改前 sha；
//	③ **锚错了对象**：`git log -1 -- <件>` 只是「末笔碰过本件」，**不保证它是本件的基线** ——
//	   那笔提交可能与本次改动无关（实测那笔提交还顺手改了别的件）。
//
// 真的退点**就在手边**：干跑回执里已经有 `before_sha256`（改前件内容的指纹），提案侧有
// `--rollback <仓外退点件>`。⇒ 退建议改锚这两样，判据只有一条机械可判的：
//
//	**写回之后，件的 sha256 要 == `before_sha256`**（不等 ⇒ 没回到改前态）。
//
// ★ 无退点可用的两格**明说不可退**（新件 = 写前不在盘 ⇒ 无改前内容；退点件 sha 对不上 ⇒ 取不回改前态），
// **禁编一条看起来能的建议** —— 编出来比空着更坏：空着会被读成「没提到」，编了会被照着跑。
//
// 只读：本块读退点件算一遍 sha256 就完事（**一个字节不写**，干跑仍零副作用）。
type devEditReturnPoint struct {
	Tier int    // 1 退点件即改前那一版 · 2 退点件不是改前那一版 · 3 新件（无改前态）· 4 提案没给退点
	Cmd  string // 可执行的那一行（Tier 1 是 `cp -p <退点件> <件>`、Tier 3 是 `rm <件>`）
	// Why = **不可退的原因片段**（Tier 1 为空；2/3/4 逐字给）。★ 它是**唯一**一份原因文本：
	// 人面那一行把它原样嵌进句子、机器面 `rollback_why` 直接取它 ⇒ 不存在第二份措辞。
	Why string
}

// Verdict —— 「能不能退」的两态（机器面 `rollback_verdict` 的**唯一**取法：档 1 ⇒ 可退，其余 ⇒ 不可退）。
// 为什么不写成一个常量对：`rollback_verdict` 与 `rollback_tier` 必须由**同一个** `Tier` 派生
// （两个面各自判一次 = 迟早自相矛盾 ✗）。
func (r devEditReturnPoint) Verdict() string {
	if r.Tier == 1 {
		return "可退"
	}
	return "不可退"
}

// devEditReturnPointOf 现算退点档（**只读**：读退点件 + 算 sha256 → 与改前指纹对拍）。
func devEditReturnPointOf(root string, prop *proposalRecord, fileRel, beforeSHA string, beforeMissing bool) devEditReturnPoint {
	if beforeMissing {
		// 写前不在盘 ⇒ 没有「改前内容」可退回；要回到「写前态」只有删件这一条。
		return devEditReturnPoint{Tier: 3, Cmd: fmt.Sprintf("rm %s", fileRel), Why: "该件写前**不在盘**（新件 · 无改前内容）"}
	}
	rb := strings.TrimSpace(prop.Rollback)
	if rb == "" {
		return devEditReturnPoint{Tier: 4, Why: "没给 `--rollback` 退点件"}
	}
	p := rb
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, filepath.FromSlash(p))
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return devEditReturnPoint{Tier: 2, Why: "退点件读不到：" + err.Error()}
	}
	if got := sha256Of(b); got != beforeSHA {
		return devEditReturnPoint{Tier: 2, Why: fmt.Sprintf("退点件现读 sha256 = %s ≠ 改前态 sha256 = %s（**不是同一版** ⇒ 照它写回取不到改前态）", got, beforeSHA)}
	}
	return devEditReturnPoint{Tier: 1, Cmd: fmt.Sprintf("cp -p %s %s", rb, fileRel)}
}

// emitDevEditRollbackAdvice —— 干跑档的**退建议**（stderr 一块 · 人读 + 逐条理由）。
// 位置：紧跟影响面摘要与钩子判决之后 —— 卡片给的是「波纹」判决，本块给的是「退得回来吗」。
// ★ 2026-09-27（本枚）：`r` 由**调用方算好传进来**（干跑档那一处 `devEditReturnPointOf`）——
// 人面这一段与机器面 `rollback_*` 四格读的是**同一格**（禁二次算 ✗ · 禁两套措辞 ✗）。
func emitDevEditRollbackAdvice(w io.Writer, prop *proposalRecord, fileRel, beforeSHA string, beforeBytes int, r devEditReturnPoint) {
	fmt.Fprintf(w, "%s: ★ 退建议（本件口径 · `GAP-20260927-216`）—— 回到改前态锚 **`before_sha256` + 提案退点件**，不锚 git 提交\n", progName)
	switch r.Tier {
	case 1:
		fmt.Fprintf(w, "  改前态指纹：before_sha256 = %s（%d 字节）—— **回到没回到，让这一个数说话**\n", beforeSHA, beforeBytes)
		fmt.Fprintf(w, "  可执行那一行：%s\n", r.Cmd)
		fmt.Fprintf(w, "  判据      ：`shasum -a 256 %s` 的输出要**逐字等于**上面那个 sha256（不等 ⇒ 没回到改前态）\n", fileRel)
		fmt.Fprintf(w, "  退点件核验：%s（现读 sha256 == 改前态 ⇒ **同一版**）\n", orDash(prop.Rollback))
	case 3:
		fmt.Fprintf(w, "  退建议    ：**不可退** —— %s：`before_sha256` = %s 是**空内容**的指纹，它只说「没有」，**没有一版可以退回**。\n", r.Why, shortSHA(beforeSHA))
		fmt.Fprintf(w, "  写前态      ：不存在 ⇒ 要「回到写前」只有一条：删掉该件 —— `%s`（本命令不会自动执行）\n", r.Cmd)
		fmt.Fprintf(w, "  ★ 禁编：这一格**不存在**「revert 一笔提交回到改前」这回事（写前根本没有这一件、也没有基线）\n")
	case 4:
		fmt.Fprintf(w, "  退建议    ：**不可退（今天没有退点可用）** —— 提案 %s %s，而改前那一版（sha256 = %s · %d 字节）**不在任何提交里**就取不回来 ⇒ **明说不可退**，不编一条看起来能的建议。\n", prop.ID, r.Why, beforeSHA, beforeBytes)
		fmt.Fprintf(w, "  下一步    ：要么**动手写之前**把改前那一版留成退点件（sha256 要对得上 %s）再重跑本命令，要么这一趟别写（`--dry-run` 不改件）\n", shortSHA(beforeSHA))
	default:
		fmt.Fprintf(w, "  退建议    ：**不可退（今天的退点件取不到改前态）** —— 提案 `--rollback` 指 %s：%s ⇒ 照它写回得到的是**另一版**，不是改前态。\n", orDash(prop.Rollback), r.Why)
		fmt.Fprintf(w, "  改前态指纹：before_sha256 = %s（%d 字节）—— 退点件要**逐字节**是这一版才叫退得回来\n", beforeSHA, beforeBytes)
		fmt.Fprintf(w, "  ★ 禁编：不许拿一个 sha 对不上的退点件冒充退点（`cp -p` 跑完 `shasum -a 256` ≠ 上面那个 sha ⇒ 没回到改前态）\n")
	}
	fmt.Fprintf(w, "  ✗ 不用 `git revert <末笔碰本件的提交>`（卡片那一条为什么不算退点）：\n")
	fmt.Fprintf(w, "      · ① 跑不通：`git revert` **不吃 pathspec** —— 卡片的 `git revert <sha> -- %s` 实测 `fatal: bad revision '%s'`（rc=128）\n", fileRel, fileRel)
	fmt.Fprintf(w, "      · ② 取不到改前态：revert 回到的是**提交记录里**那一版，而**改前工作树含未提交改动** ⇒ 那一版不在任何提交里（`HEAD:本件` ≠ 改前工作树）⇒ 跑完 sha ≠ 改前 sha %s\n", shortSHA(beforeSHA))
	fmt.Fprintf(w, "      · ③ 锚错对象：`git log -1 -- <件>` 只是「**末笔碰过本件**」，不保证它是本件的基线（那笔提交可能与本次改动无关）\n")
}
