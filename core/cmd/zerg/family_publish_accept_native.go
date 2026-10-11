// family_publish_accept_native.go —— `zerg publish accept` 的 **Go 真身**（收口编排件）。
//
// 病（缺件）：任务单 ㈤（`…/acceptance-prep/20261011-㈤v2.md`）把发布链收口的 8 条验收逐条
// 定死了「怎么跑 / 怎么取数」，但 8 条仍是**散条** —— 收口时靠人拼、AND 靠人算，等于没有
// 一个「收口结论」。（㈤v2 §六 `:189`：「还差的一条 = 收口编排件」。）
//
// 本件把 8 条**逐条 rc AND 成单一结论**，并**固定**三样散件实参（镜像产出/正式镜像树的 `--out`、
// 验-5 的 `gates_total` 冻结值、验-4 的记录值档目录），且与最高准则对齐：结论**从 CLI 打得着**
// （一条命令一个退码）、判据实现**单一真源**（编排件**只调不重判**）。
//
// ★ 同形先例 = `zerg publish preflight`（`family_publish_preflight_native.go`）：照它写 ——
//
//	· **调门**：`paccCallGate` 逐字照 `pfCallGate`（`gateCheckGoEntry[name]` ⇒ `gateCheckRunInproc`，
//	  进程内按名调**同族既有 Go 判据体**，✗ 不 fork、✗ 不复制任何判据实现）。
//	· **聚合**：逐条 `if rc != 0` ⇒ 收口位；三档出口（照 `:456-469`）。
//	· **八条步表**：照 `brConditions`／`gateRunStepFields` 先例，Go 侧**声明式**表 `publishAcceptSteps`
//	  （每格只存「调谁 ＋ 什么 argv ＋ 判据来自㈤v2哪一条」，✗ 不复制判据实现）。
//
// ★ 边界（**单一真源**）：凡**门已判**的只取 rc／读数，✗ 不复制判据、✗ 不改它退码。
//
//	· 验-1 ⇒ 进程内调 `publishVerifyRun`（`family_publish_verify_native.go:293` ＝同族既有 CLI 真身）。
//	· 验-6/7/8 ⇒ `gateCheckGoEntry` 按名**进程内**调（照 `pfCallGate`）。
//	· 验-5 ⇒ `gate ls --json` 的 `gates_total`（经 `gateLsJSON` ⟹ `pgsParseListJSON` 自证）＋ `--self-test`。
//	· **仅** 验-2/3/4 无既有门 ⇒ 才在本件内定义**取数口**（判定仍以 ㈤v2 判据逐字为单一真源）。
//
// ★ 出口三档（只读主档 · ✗ 不许把 2 折成 0）：全 `rc==0` ⇒ `0`（收口）；任一 `rc==1` ⇒ `1`
//
//	（不收口·有红）；任一 `rc==2`／步「判不了／缺件」⇒ `2`（**不给结论**·fail-closed）。
//	—— 本件采 **fail-closed 优先**（有 2 即 2）：缺件/跑不动**绝不**被读成「没病」。
//	★ 照实记一处偏离：先例 `publish preflight` 的 BLOCKED 档走 `exitBlocked`(=8)；本件按设计单
//	  与任务单的**显式**口径落 0/1/2（`paccRCBlocked = 2` ＝ `exitUsage` 「不给结论」同码）。
//
// ★ `--out <件>`（可选写面）＝报告件：照 `gate matrix --out` 逐字 —— D2 档（`--dry-run` ⇒ 计划件走
//
//	stdout＋rc=0 零落盘 / 缺 `--yes` ⇒ 计划件走 stderr＋rc=2 / `--dry-run` 与 `--yes` 同给 ⇒ rc=2）、
//	`--out` 与 `--json` 不许同给 ⇒ rc=2、**原子落件**（同目录临时件 → `os.Rename`，`paccAtomicWrite`）。
//	★ 报告件落**调用方给的路径**（命令面**不预设**入仓 · 照 `devEditArchiveDir` 的「落调用方给的路径」体例）。
//
// 记录值目录（验-4）：照 `devEditArchiveDir`（`family_dev_edit.go:1975-1977`）体例 ——
// `filepath.Join(stateDirOf(), "退点归档-"+<YYYYMMDD>)`，件名压平照 `devEditSafeName`。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// paccGatesTotalFrozen —— 验-5② 的 `gates_total` **冻结初值**（棘轮式：≠ 此值 ⇒ 红）。
//
// ★ 冻结值取值点 = 本单现跑 `bash scripts/gates/precommit-gates.sh --list --json` 的顶层格（211）。
// ★★ 本值**只许按拍-C 现跑校准**（挂门使步数 +1 ⇒ 必须**同批重冻** —— 照 `newcmd-paths-baseline.json`
// 的「冻结值 + 同批重冻」体例 · ✗ 不用 down-only 棘轮：挂门是**有意 +1**，不是只降）。
// ★ 落点说明（照实报）：设计稿 §3.4 主张落 `scripts/gates/publish-accept-baseline.json`，但本单
// **另一路〔第94〕正在改 `scripts/gates/**`** ⇒ 目录冲突 ⇒ 改落本件 **Go 常量**（照 `cscaAuthoritativeBaseline`
// 先例 · `check_structured_count_assert.go:67-70`）；「迁回 `scripts/gates/publish-accept-baseline.json`」
// **列为收口随动**（另一路入场后再迁移）。
const paccGatesTotalFrozen = 211

// paccRCBlocked —— 编排件「判不了 / 缺件」档的**聚合退码**（fail-closed · 与 `exitUsage` 同值）。
const paccRCBlocked = 2

// paccSpawnFail —— shell 惯例退码 127（外部子进程起不来 · 非 ExitError）—— 照 `pfExitSpawnFail`。
const paccSpawnFail = 127

// 取数口旋钮（**自检/夹具专用** · 照 `publish preflight` 的 `ZERG_PUBFACE_ROOT` 同一种旋钮）：
//
//	· paccEnvMirror   = 镜像产出目录（验-2 的「镜像意图 tip」/验-3 的「正式镜像树」· 同一 `--out`）。
//	· paccEnvRollback = 回退演练件（仓内相对路径 · 验-4 的比对对象）。
//	· paccEnvHelpArt  = `check-help-snapshot --runtime-artifact <dir>`（验-7 · 缺省 `/tmp/zerg-help-artifacts`）。
const (
	paccEnvMirror   = "ZERG_ACCEPT_MIRROR"
	paccEnvRollback = "ZERG_ACCEPT_ROLLBACK"
	paccEnvHelpArt  = "ZERG_ACCEPT_HELP_ARTIFACTS"
)

var paccDanglingRe = regexp.MustCompile(`(?m)^(dangling|unreachable|missing|broken|error)`)

// paccStepKind —— 8 条验收各自的「取数口」种类（照设计稿 §3.2 三档）。
type paccStepKind int

const (
	paccKindGate      paccStepKind = iota // 按名进程内调 `gateCheckGoEntry[name]`（照 `pfCallGate`）
	paccKindCLIInproc                     // 进程内调**同族既有 CLI 真身**（验-1 = `publishVerifyRun`）
	paccKindAdapter                       // 无既有门 ⇒ 本件内薄适配器（git / shasum · 判定照 ㈤v2）
)

// paccStep —— 收口编排件**声明式**步表的一格（照 `brConditions`／`gateRunStepFields` 先例）。
//
// ★ 每格只存「调谁 ＋ 什么 argv ＋ 判据来自 ㈤v2 哪一条」，✗ **不复制任何判据实现**（照 `pfCallGate` 件头）。
type paccStep struct {
	id       string                               // 验证条号（验-1…验-8）
	title    string                               // 人面标题
	kind     paccStepKind                         // 取数口种类
	criteria string                               // ★ 判据来自 ㈤v2 哪一条（逐字来源标注）
	gates    [][]string                           // kind==paccKindGate：`{判据名, 子件 argv…}` · 逐条 rc AND
	run      func(root string, out io.Writer) int // 非 gate 档：取数口实现
}

// publishAcceptSteps —— 8 条收口验收的**唯一**声明处（加格不动 `main.go`）。
//
// ★ 判据逐字来源 = 任务单 ㈤v2（`…/acceptance-prep/20261011-㈤v2.md`）；本表**只**登记「调谁/什么 argv」。
var publishAcceptSteps = []paccStep{
	{
		id: "验-1", title: "对拍零分歧", kind: paccKindCLIInproc,
		criteria: "㈤v2 验-1 · 进程内调 publishVerifyRun（= `zerg publish verify` 真身 · `test-publish-parity.sh` 的 Go 真搬）",
		run:      paccRunVerify,
	},
	{
		id: "验-2", title: "`ls-remote` 差集空", kind: paccKindAdapter,
		criteria: "㈤v2 验-2 · 镜像意图 tip == 远端 tip ∧ refs 差集空（**先断言两侧非空** · 防空过）",
		run:      paccRunMirrorTip,
	},
	{
		id: "验-3", title: "镜像 fsck", kind: paccKindAdapter,
		criteria: "㈤v2 验-3 · `git fsck --full` rc=0 ∧ `^(dangling|unreachable|missing|broken|error)` 计数==0（补抓 dangling）",
		run:      paccRunFsck,
	},
	{
		id: "验-4", title: "回退演练 digest 逐字", kind: paccKindAdapter,
		criteria: "㈤v2 验-4 · 现值 `shasum -a 256` == 记录值 `退点归档-<日期>/<件>.sha256`（空==空假 PASS 必红）",
		run:      paccRunRollbackDigest,
	},
	{
		id: "验-5", title: "门本次身", kind: paccKindAdapter,
		criteria: "㈤v2 验-5 · ①release 面挂「两器产出一致性对拍」步（n≥1）②`gates_total`==冻结值 ③`--self-test` rc=0",
		run:      paccRunGateSelf,
	},
	{
		id: "验-6", title: "契约基座", kind: paccKindGate,
		criteria: "㈤v2 验-6 · `check-publish-face-sync` ∧ `check-newcmd-followup` 逐条 rc=0（按名进程内调）",
		gates: [][]string{
			{"check-publish-face-sync"},
			{"check-newcmd-followup"},
		},
	},
	{
		id: "验-7", title: "受影响面复位", kind: paccKindGate,
		criteria: "㈤v2 验-7 · `check-cli-contract` / `check-help-snapshot`（**直接调门取真 rc**） / `check-dryrun-semantics` 逐条 rc=0",
		gates: [][]string{
			{"check-cli-contract"},
			{"check-help-snapshot", "--runtime-artifact", "__HELP_ARTIFACTS__"},
			{"check-dryrun-semantics"},
		},
	},
	{
		id: "验-8", title: "推送不吞错", kind: paccKindAdapter,
		criteria: "㈤v2 验-8 · `check-bypass-idioms --json` ⇒ `unregistered==0 ∧ registry==0 ∧ stale==0`（读登记面 · 不只看 rc）",
		run:      paccRunBypassIdioms,
	},
}

// paccResult —— 一条验收的读数（步定义 + 归一退码 + 该步取数口输出）。
type paccResult struct {
	step paccStep
	rc   int
	log  string
}

// cmdPublishAccept —— `zerg publish accept` 入口（命令树 `run` · 只转发真身）。
func cmdPublishAccept(inv *invocation, stdout, stderr io.Writer) int {
	return publishAcceptRun(inv, stdout, stderr)
}

// publishAcceptRun —— 收口编排件真身：跑 8 条、逐条 rc AND、出三档结论；可选 `--out` 报告件。
func publishAcceptRun(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 跑不动收口（不给结论 · 退码 %d）\n", progName, paccRCBlocked)
		return paccRCBlocked
	}

	// ★ `--out`（写面·报告件）与 `--json`（判面·机读投影）**不许同给** —— 与 `gate matrix --out`
	//   逐字同一条（写面不得兼作判面）。判在**任何 stdout 写入之前**（未过门前不建件、不建目录）。
	if strings.TrimSpace(inv.flagVal("--out")) != "" && inv.jsonGiven {
		inv.setErr("usage", "out_json_conflict", "--out 与 --json 不许同给")
		fmt.Fprintf(stderr, "%s: `--out`（报告件 · 写面）与 `--json`（机读投影 · 判面）**不许同给** ⇒ 用法错 2\n", progName)
		return exitUsage
	}

	// ── 跑 8 条（逐条 rc AND · 每一格标出判据来自 ㈤v2 哪一条）──
	results := make([]paccResult, 0, len(publishAcceptSteps))
	rcs := make([]int, 0, len(publishAcceptSteps))
	for _, st := range publishAcceptSteps {
		var buf bytes.Buffer
		fmt.Fprintf(stderr, "── %s %s ──\n", st.id, st.title)
		fmt.Fprintf(stderr, "   判据来源：%s\n", st.criteria)
		rc := paccRunStep(root, st, &buf)
		rc = paccNormRC(rc)
		io.Copy(stderr, &buf)
		fmt.Fprintf(stderr, "   ⇒ %s rc=%d\n", st.id, rc)
		results = append(results, paccResult{step: st, rc: rc, log: buf.String()})
		rcs = append(rcs, rc)
	}
	overall := paccAggregate(rcs)

	// ── 三档出口（机构面 · ✗ 不许把 2 折成 0）──
	switch overall {
	case exitOK:
		fmt.Fprintln(stderr, "✅ 8 条收口验收全绿 ⇒ 收口（任一红即不收口）")
	case exitFail:
		fmt.Fprintln(stderr, "❌ 有红 ⇒ 不收口")
	case paccRCBlocked:
		fmt.Fprintln(stderr, "⚠ 有步「判不了／缺件」⇒ 不给结论（fail-closed · 不退 0 · 补齐后再跑）")
	}

	// ── `--out <件>`：D2 报告件（三态门 · 原子落件）──
	out := strings.TrimSpace(inv.flagVal("--out"))
	if out != "" {
		report := paccReport(root, results, overall)
		if inv.dryRun {
			paccOutPlan(stdout, out, len(results), "--dry-run")
			fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未落件、未建目录）\n")
			return exitOK
		}
		if !inv.yes {
			paccOutPlan(stderr, out, len(results), "缺 `--yes`（D2 档）")
			inv.setErr("usage", "yes_required", "缺 --yes")
			return exitUsage
		}
		if err := paccAtomicWrite(out, report); err != nil {
			inv.setErr("failed", "out_write_failed", err.Error())
			fmt.Fprintf(stderr, "%s: 报告件写不进去（%v）—— 给一个**已存在**目录里的路径\n", progName, err)
			return exitFail
		}
		fmt.Fprintf(stderr, "%s: 已落收口报告 → %s\n", progName, out)
	}

	// ── 机读/人面投影（stdout · 照 listCmd：`--json <字段>` ⇒ 投影；否则人面表）──
	rows := make([]map[string]string, 0, len(results))
	for _, r := range results {
		rows = append(rows, map[string]string{
			"id":       r.step.id,
			"title":    r.step.title,
			"rc":       strconv.Itoa(r.rc),
			"criteria": r.step.criteria,
		})
	}
	if rc := listCmd(inv, stdout, stderr, []string{"id", "title", "rc"}, rows); rc != exitOK {
		return rc
	}
	return overall
}

// paccRunStep —— 逐条派发（gate 档走 `paccRunGateGroup`；其余走各自取数口）。
func paccRunStep(root string, st paccStep, out io.Writer) int {
	if st.kind == paccKindGate {
		return paccRunGateGroup(root, st.gates, out)
	}
	if st.run == nil {
		fmt.Fprintf(out, "   ⛔ 本步无取数口 ⇒ 判不了（fail-closed）\n")
		return paccRCBlocked
	}
	return st.run(root, out)
}

// paccRunGateGroup —— 按名**进程内**逐条调门（照 `pfCallGate`），**逐条 rc AND**。
func paccRunGateGroup(root string, gates [][]string, out io.Writer) int {
	worst := exitOK
	for _, g := range gates {
		if len(g) == 0 {
			continue
		}
		name := g[0]
		argv := append([]string{}, g[1:]...)
		argv = paccExpandArgv(argv)
		rc, log := paccCallGate(root, name, argv)
		paccTail(out, log, 3, "   ")
		if rc == 0 {
			fmt.Fprintf(out, "   ✅ %s rc=0\n", name)
			continue
		}
		fmt.Fprintf(out, "   ✗ %s rc=%d\n", name, rc)
		worst = paccWorse(worst, paccNormRC(rc))
	}
	return worst
}

// paccExpandArgv —— 把步表里的占位符展开成运行期实参（只此一处）。
func paccExpandArgv(argv []string) []string {
	helpArt := strings.TrimSpace(os.Getenv(paccEnvHelpArt))
	if helpArt == "" {
		helpArt = "/tmp/zerg-help-artifacts"
	}
	out := make([]string, 0, len(argv))
	for _, a := range argv {
		switch a {
		case "__HELP_ARTIFACTS__":
			out = append(out, helpArt)
		default:
			out = append(out, a)
		}
	}
	return out
}

// paccCallGate —— 按名调用同族既有 Go 判据体（进程内 · 与 `zerg gate check <名>` 同一条入口）；
// 逐字照 `publish preflight` 的 `pfCallGate`（`:109-119`）。返回 rc 与 **stdout+stderr 合并**输出。
func paccCallGate(root, name string, argv []string) (int, string) {
	fn, ok := gateCheckGoEntry[name]
	if !ok {
		return paccRCBlocked, fmt.Sprintf("   ⛔ 判据名 %q 不在 inproc 表（`gateCheckGoEntry`）⇒ 判不了（fail-closed）\n", name)
	}
	var buf bytes.Buffer
	rc := gateCheckRunInproc(fn, root, &buf, &buf, argv)
	return rc, buf.String()
}

// ── 验-1：进程内调 `publishVerifyRun`（同族既有 CLI 真身 · ✗ 不复制判据）──

func paccRunVerify(root string, out io.Writer) int {
	_ = root
	var buf bytes.Buffer
	rc := publishVerifyRun(&buf, &buf)
	paccTail(out, buf.String(), 8, "   ")
	// ㈤v2 验-1：脚本 rc + 小结行存在（末行订正：不依赖行位置）。
	// ★ 超时/跑不动（`publishVerifyRun` 到点收整棵树 ⇒ 退 `exitBlocked`）⇒ **判不了**（fail-closed），
	//   ✗ 不当红、✗ 更不当绿 —— 走本件「不给结论」档（`paccRCBlocked`）。
	if rc == exitBlocked {
		fmt.Fprintf(out, "   ⛔ 验-1 跑两器超时/跑不动（rc=%d）⇒ 判不了（fail-closed · ✗ 不当作红/绿）\n", rc)
		return paccRCBlocked
	}
	if rc != 0 {
		fmt.Fprintf(out, "   ✗ 验-1 对拍脚本 rc=%d（红：脚本 rc≠0）\n", rc)
		return 1
	}
	if !regexp.MustCompile(`(?m)^  通过 [0-9]+ 项 · 失败 0 项`).MatchString(buf.String()) {
		fmt.Fprintln(out, "   ✗ 验-1 红：小结行缺失 / 失败≠0")
		return 1
	}
	fmt.Fprintln(out, "   ✅ 验-1 对拍零分歧（rc=0 ∧ 小结行 失败 0 项）")
	return 0
}

// ── 验-2/3/4/5：无既有门 ⇒ 本件内薄适配器（判定照 ㈤v2 判据逐字）──

// paccRunMirrorTip —— 验-2：镜像意图 tip == 远端 tip ∧ refs 差集空（先断言两侧非空）。
func paccRunMirrorTip(root string, out io.Writer) int {
	img := strings.TrimSpace(os.Getenv(paccEnvMirror))
	if img == "" {
		fmt.Fprintf(out, "   ⛔ 验-2 缺件：未给镜像产出目录（%s）⇒ 跑不动 ⇒ fail-closed（缺件≠绿）\n", paccEnvMirror)
		return paccRCBlocked
	}
	if !paccIsDir(img) {
		fmt.Fprintf(out, "   ⛔ 验-2 缺件：镜像产出目录不在盘（%s）⇒ fail-closed\n", img)
		return paccRCBlocked
	}
	r1, intent := paccGitOut(img, "rev-parse", "refs/heads/main")
	if r1 != 0 || intent == "" {
		fmt.Fprintln(out, "   ✗ 验-2 红：镜像意图 tip 取空（取数口 mirror-public.sh:118）")
		return 1
	}
	r2, remote := paccRemoteTip(root, "public-gh")
	if r2 != 0 || remote == "" {
		fmt.Fprintln(out, "   ✗ 验-2 红：远端 tip 取空")
		return 1
	}
	if intent != remote {
		fmt.Fprintf(out, "   ✗ 验-2 红：意图 tip(%s) ≠ 远端 tip(%s)\n", paccShort(intent), paccShort(remote))
		return 1
	}
	L := paccShowRefs(img)
	R := paccRemoteRefs(root, "public-gh")
	if L == "" || R == "" {
		fmt.Fprintln(out, "   ✗ 验-2 红：refs 面一侧为空（空==空）")
		return 1
	}
	if d := paccComm3Count(L, R); d != 0 {
		fmt.Fprintf(out, "   ✗ 验-2 红：refs 差集非空（%d 行）\n", d)
		return 1
	}
	fmt.Fprintln(out, "   ✅ 验-2 意图 tip==远端 tip ∧ refs 差集空")
	return 0
}

// paccRunFsck —— 验-3：`git fsck --full` rc=0 ∧ dangling/unreachable/missing/broken/error 计数==0。
func paccRunFsck(root string, out io.Writer) int {
	_ = root
	tree := strings.TrimSpace(os.Getenv(paccEnvMirror))
	if tree == "" || !paccIsDir(tree) {
		fmt.Fprintf(out, "   ⛔ 验-3 缺件：正式镜像树不在盘（%s）⇒ fail-closed\n", tree)
		return paccRCBlocked
	}
	if rc, _ := paccGitOut(tree, "rev-parse", "--git-dir"); rc != 0 {
		fmt.Fprintf(out, "   ⛔ 验-3 缺件：%s 不是 git 仓（切之后才有对象）⇒ fail-closed\n", tree)
		return paccRCBlocked
	}
	rc, log := paccGitOut(tree, "fsck", "--full")
	if rc != 0 {
		fmt.Fprintf(out, "   ✗ 验-3 红：fsck rc=%d\n", rc)
		paccTail(out, log, 4, "   ")
		return 1
	}
	n := len(paccDanglingRe.FindAllString(log, -1))
	if n != 0 {
		fmt.Fprintf(out, "   ✗ 验-3 红：有 dangling/悬空/缺失对象 %d 行\n", n)
		paccTail(out, paccDanglingMatching(log), 4, "   ")
		return 1
	}
	fmt.Fprintln(out, "   ✅ 验-3 fsck rc=0 ∧ 悬空/缺失计数 0")
	return 0
}

// paccRunRollbackDigest —— 验-4：现值 sha256 == 记录值（`退点归档-<日期>/<件>.sha256` · 退-1 写）。
func paccRunRollbackDigest(root string, out io.Writer) int {
	fileRel := strings.TrimSpace(os.Getenv(paccEnvRollback))
	if fileRel == "" {
		fmt.Fprintf(out, "   ⛔ 验-4 缺件：未给回退演练件（%s）⇒ 跑不动 ⇒ fail-closed\n", paccEnvRollback)
		return paccRCBlocked
	}
	// 记录值件 = <状态目录>/退点归档-<日期>/<safeName>.sha256（照 devEditArchiveDir/devEditSafeName 体例）。
	rec := filepath.Join(devEditArchiveDir(), devEditSafeName(fileRel)+".sha256")
	if fi, err := os.Stat(rec); err != nil || fi.Size() == 0 {
		fmt.Fprintf(out, "   ✗ 验-4 红：记录值缺位（空==空假 PASS）（%s）\n", rec)
		return 1
	}
	now := paccSha256File(filepath.Join(root, fileRel))
	if now == "" {
		now = paccSha256File(fileRel)
	}
	recv := paccFirstField(rec)
	if now == "" || recv == "" || now != recv {
		fmt.Fprintf(out, "   ✗ 验-4 红：现值(%s) ≠ 记录值(%s) ⇒ 演练 FAIL（不给结论）\n", paccShort(now), paccShort(recv))
		return 1
	}
	fmt.Fprintln(out, "   ✅ 验-4 现值 == 记录值")
	return 0
}

// paccRunGateSelf —— 验-5：门本次身（三小条）。
func paccRunGateSelf(root string, out io.Writer) int {
	blocked, fail := false, false

	// ① release 面挂「两器产出一致性对拍」步（㈤v2 验-5 ①）
	if n := paccReleaseStepCount(root, "两器产出一致性对拍"); n < 1 {
		fmt.Fprintf(out, "   ✗ 验-5① release 面未挂「两器产出一致性对拍」步（n=%d）\n", n)
		fail = true
	} else {
		fmt.Fprintf(out, "   ✅ 验-5① release 面挂「两器产出一致性对拍」步 × %d\n", n)
	}

	// ② gates_total == 冻结值（经 `gate ls --json` 机读面 · `pgsParseListJSON` 自证）
	gt, ok, note := paccGatesTotal(root)
	switch {
	case !ok:
		fmt.Fprintf(out, "   ⛔ 验-5② gates_total 读不到（%s）⇒ fail-closed\n", note)
		blocked = true
	case gt != paccGatesTotalFrozen:
		fmt.Fprintf(out, "   ✗ 验-5② gates_total 漂移（得 %d · 冻结 %d）\n", gt, paccGatesTotalFrozen)
		fail = true
	default:
		fmt.Fprintf(out, "   ✅ 验-5② gates_total==%d（冻结值）\n", gt)
	}

	// ③ `--self-test` rc=0
	if rc, log := paccRunScript(root, "scripts/gates/precommit-gates.sh", "--self-test"); rc != 0 {
		fmt.Fprintf(out, "   ✗ 验-5③ `--self-test` rc=%d\n", rc)
		paccTail(out, log, 3, "   ")
		fail = true
	} else {
		fmt.Fprintln(out, "   ✅ 验-5③ `--self-test` rc=0")
	}

	if blocked {
		return paccRCBlocked
	}
	if fail {
		return 1
	}
	return 0
}

// paccRunBypassIdioms —— 验-8：`check-bypass-idioms --json` 登记面 `unregistered==0 ∧ registry==0 ∧ stale==0`。
func paccRunBypassIdioms(root string, out io.Writer) int {
	rc, log := paccCallGate(root, "check-bypass-idioms", []string{"--json"})
	doc := paccLastJSONObject(log)
	if doc == "" {
		fmt.Fprintf(out, "   ⛔ 验-8 缺件：`check-bypass-idioms --json` 未出机读面（rc=%d）⇒ fail-closed\n", rc)
		return paccRCBlocked
	}
	var d map[string]json.RawMessage
	if err := json.Unmarshal([]byte(doc), &d); err != nil {
		fmt.Fprintf(out, "   ⛔ 验-8 缺件：登记面 JSON 解不出（%v）⇒ fail-closed\n", err)
		return paccRCBlocked
	}
	unreg, ok1 := paccJSONInt(d, "unregistered")
	reg, ok2 := paccJSONInt(d, "registry")
	stale, ok3 := paccJSONInt(d, "stale")
	if !ok1 || !ok2 || !ok3 {
		fmt.Fprintln(out, "   ⛔ 验-8 缺件：登记面缺 unregistered/registry/stale 格 ⇒ fail-closed")
		return paccRCBlocked
	}
	fail := false
	if unreg != 0 {
		fmt.Fprintf(out, "   ✗ 验-8 红：未登记命中 %d\n", unreg)
		fail = true
	}
	if reg != 0 {
		fmt.Fprintf(out, "   ✗ 验-8 红：F07 仍登记在册 registry=%d（只证「已登记」不证「已修」）\n", reg)
		fail = true
	}
	if stale != 0 {
		fmt.Fprintf(out, "   ✗ 验-8 红：失效假条目 stale=%d（登记了没对上锚 ⇒ 应删）\n", stale)
		fail = true
	}
	if fail {
		return 1
	}
	fmt.Fprintln(out, "   ✅ 验-8 未登记=0 且登记册已空 ⇒ 真已修")
	return 0
}

// ── 聚合与出口（★ 任一红 ⇒ 1；★ 缺件/跑不动 ⇒ 2 · fail-closed 优先，✗ 不把 2 折成 0）──

// paccAggregate —— 8 条 rc 的 AND。**fail-closed 优先**：有 2 ⇒ 2；否则有 1 ⇒ 1；全 0 ⇒ 0。
func paccAggregate(rcs []int) int {
	blocked, fail := false, false
	for _, rc := range rcs {
		switch paccNormRC(rc) {
		case 0:
		case 1:
			fail = true
		default:
			blocked = true
		}
	}
	if blocked {
		return paccRCBlocked
	}
	if fail {
		return exitFail
	}
	return exitOK
}

// paccNormRC —— 退码归一到 0/1/2（只认 0/1/2；其它 ⇒ 2「判不了」· 照 `publish preflight` 的 `norm`）。
func paccNormRC(rc int) int {
	switch rc {
	case 0, 1, 2:
		return rc
	default:
		return 2
	}
}

// paccWorse —— 两个已归一退码取「更差」者（2 > 1 > 0）。
func paccWorse(a, b int) int {
	if b > a {
		return b
	}
	return a
}

// ── `--out` 报告件（D2 · 三态门 · 原子落件 · 照 `gate matrix --out`）──

func paccReport(root string, results []paccResult, overall int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 收口验收报告（`zerg publish accept` · 判据单一真源 = 任务单 ㈤v2 · 8 条逐条 rc AND）\n")
	fmt.Fprintf(&b, "# 仓根：%s · gates_total 冻结值：%d · 整体：%d（0 收口 / 1 有红 / 2 不给结论）\n",
		root, paccGatesTotalFrozen, overall)
	b.WriteString("id\ttitle\trc\tcriteria\n")
	for _, r := range results {
		fmt.Fprintf(&b, "%s\t%s\t%d\t%s\n", r.step.id, r.step.title, r.rc, r.step.criteria)
	}
	fmt.Fprintf(&b, "整体\t任一红⇒1 · 缺件/跑不动⇒2 · 全绿⇒0\t%d\trc AND\n", overall)
	return b.String()
}

// paccOutPlan —— `--out` 落盘面的**计划件**（`--dry-run` ⇒ 走 stdout · 缺 `--yes` ⇒ 走 stderr）。纯函数。
func paccOutPlan(w io.Writer, out string, n int, why string) {
	fmt.Fprintf(w, "计划件（%s · `zerg publish accept --out %s`）\n", why, out)
	fmt.Fprintf(w, "  落点件   : %s\n", out)
	fmt.Fprintf(w, "  件内容   : 注释头 2 行 + 列名 1 行 + 逐条 %d 行 + 整体 1 行（制表符分隔）\n", n)
	fmt.Fprintf(w, "  未执行   : %s —— 本档**一个字节都不落**（真写要 `--yes`）\n", why)
}

// paccAtomicWrite —— `--out <件>` 的**原子落件**（照 `gateMatrixAtomicWrite` 逐字：同目录临时件 → rename）。
func paccAtomicWrite(out, content string) error {
	dir := filepath.Dir(out)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(out)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, out); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// ── 底层取数助手（git / shasum / 脚本 · 只读）──

func paccGitOut(dir string, args ...string) (int, string) {
	a := append([]string{"-C", dir}, args...)
	cmd := armChildGroup(exec.Command("git", a...)) // ★ 成组 + 登记 runningChild ⇒ 墙钟到点收得到（不漏网）
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	runningChild = cmd
	defer func() { runningChild = nil }()
	err := cmd.Run()
	return paccExitCode(err), strings.TrimSpace(out.String())
}

func paccRemoteTip(root, remote string) (int, string) {
	rc, out := paccGitOut(root, "ls-remote", remote, "refs/heads/main")
	if out == "" {
		return rc, ""
	}
	return rc, strings.TrimSpace(strings.Fields(out)[0])
}

func paccShowRefs(dir string) string {
	_, out := paccGitOut(dir, "show-ref")
	return paccSortedRefNames(out)
}

func paccRemoteRefs(root, remote string) string {
	_, out := paccGitOut(root, "ls-remote", "--refs", remote)
	return paccSortedRefNames(out)
}

// paccSortedRefNames —— 取 `git show-ref`/`ls-remote` 输出的**第 2 列（ref 名）**、排序、拼行（照 ㈤v2 的 awk+sort）。
func paccSortedRefNames(raw string) string {
	names := []string{}
	for _, ln := range strings.Split(raw, "\n") {
		f := strings.Fields(ln)
		if len(f) >= 2 {
			names = append(names, f[1])
		}
	}
	sort.Strings(names)
	return strings.Join(names, "\n")
}

// paccComm3Count —— 两侧 ref 名（已排序 · 换行分隔）的对称差行数（照 ㈤v2 的 `comm -3 … | wc -l`）。
func paccComm3Count(l, r string) int {
	inR := map[string]bool{}
	for _, s := range strings.Split(r, "\n") {
		if s != "" {
			inR[s] = true
		}
	}
	inL := map[string]bool{}
	for _, s := range strings.Split(l, "\n") {
		if s != "" {
			inL[s] = true
		}
	}
	n := 0
	for _, s := range strings.Split(l, "\n") {
		if s != "" && !inR[s] {
			n++
		}
	}
	for _, s := range strings.Split(r, "\n") {
		if s != "" && !inL[s] {
			n++
		}
	}
	return n
}

func paccSha256File(path string) string {
	cmd := armChildGroup(exec.Command("shasum", "-a", "256", path)) // ★ 成组 + 登记 runningChild ⇒ 墙钟到点收得到
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	runningChild = cmd
	defer func() { runningChild = nil }()
	if err := cmd.Run(); err != nil {
		return ""
	}
	return paccFirstFieldText(out.String())
}

func paccFirstField(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return paccFirstFieldText(string(b))
}

func paccFirstFieldText(t string) string {
	f := strings.Fields(strings.TrimSpace(t))
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

func paccRunScript(root, rel string, args ...string) (int, string) {
	cmd := armChildGroup(exec.Command("bash", append([]string{filepath.Join(root, rel)}, args...)...)) // ★ 成组 + 登记 runningChild
	cmd.Dir = root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	runningChild = cmd
	defer func() { runningChild = nil }()
	err := cmd.Run()
	return paccExitCode(err), buf.String()
}

func paccExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return paccSpawnFail
}

// paccGatesTotal —— 经 `zerg gate ls --json gates_total` 机读面取顶层 `gates_total`（进程内调 `gateLsJSON`
// ⟹ `pgsParseListJSON` 自证 · ✗ 不另造第二套解析）。ok=false ⇒ 判不了（fail-closed）。
func paccGatesTotal(root string) (int, bool, string) {
	sinv := &invocation{path: []string{"gate", "ls"}, jsonGiven: true, fields: []string{"gates_total"}}
	var buf bytes.Buffer
	if rc := gateLsJSON(sinv, &buf, &buf, root, filepath.Join(root, gateScriptRel)); rc != exitOK {
		return 0, false, fmt.Sprintf("`gate ls --json gates_total` 退 %d", rc)
	}
	doc := paccLastJSONObject(buf.String())
	if doc == "" {
		return 0, false, "`gate ls --json` 无 JSON 对象"
	}
	var d map[string]json.RawMessage
	if err := json.Unmarshal([]byte(doc), &d); err != nil {
		return 0, false, err.Error()
	}
	gt, ok := paccJSONInt(d, "gates_total")
	if !ok {
		return 0, false, "无 gates_total 格"
	}
	return gt, true, ""
}

// paccReleaseStepCount —— `--list --scope release --plain` 里步名含 `<kw>` 的步数（㈤v2 验-5①）。
func paccReleaseStepCount(root, kw string) int {
	rc, out := paccRunScript(root, "scripts/gates/precommit-gates.sh", "--list", "--scope", "release", "--plain")
	if rc != 0 {
		return 0
	}
	n := 0
	for _, ln := range strings.Split(out, "\n") {
		cols := strings.Split(ln, "\t")
		if len(cols) >= 3 && strings.Contains(cols[2], kw) {
			n++
		}
	}
	return n
}

func paccLastJSONObject(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimSpace(lines[i])
		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			return s
		}
	}
	return ""
}

func paccJSONInt(d map[string]json.RawMessage, key string) (int, bool) {
	raw, ok := d[key]
	if !ok {
		return 0, false
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, false
	}
	return n, true
}

func paccIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func paccShort(s string) string {
	if s == "" {
		return "—"
	}
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// paccTail —— 复刻 `tail -n <n>`（取合并输出末 n 行 · 逐行加前缀）。
func paccTail(w io.Writer, s string, n int, prefix string) {
	if s == "" {
		return
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, ln := range lines {
		fmt.Fprintf(w, "%s%s\n", prefix, ln)
	}
}

func paccDanglingMatching(log string) string {
	var b strings.Builder
	for _, m := range paccDanglingRe.FindAllString(log, -1) {
		b.WriteString(m)
		b.WriteString("\n")
	}
	return b.String()
}
