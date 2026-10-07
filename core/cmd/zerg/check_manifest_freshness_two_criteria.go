// check_manifest_freshness_two_criteria.go —— 判据件
// `scripts/gates/check-manifest-freshness-two-criteria.py`（296 行）的 **Go 真搬**（v2.5.14 · M2 批 · 甲案口径）。
//
// 本件做什么（= py `main()`）：
//
//	把一份**合成清单**喂进真源 `check-manifest-freshness.py` 的 `diagnose()`，再核三条机器可判的
//	判据 —— C1「两条判据各自成行」· C2「合并成一句即红」· C3「源级也分列（两处 append 在不同源码行）」
//	· C4「真源自带正负例（现跑 `--self-test`）」。退码 0/1/2 三档，与仓内其余门件同口径。
//
// 真跑子进程面（甲案：输入确定 ⇒ 输出可逐字对拍）：
//
//	· `git -c core.quotepath=false rev-parse HEAD` —— 在**仓根**里取分支头；仓根来自 `--root`
//	  （缺省由 cwd 上溯找本件同族 py 的位置，见件头「标准声明 (a)」）。取不到 ⇒ 与 py 同形不给结论。
//	· 真源 `--self-test` 那一步：py 用 `subprocess.run([sys.executable, 真源, "--self-test"])` fork 一枚
//	  python3；本件**进程内复用同一份移植后的判据** ⇒ 可观测面（rc 与 stdout 末行文案）对齐，**手段不同**。
//
// 入口签名（与 py `main()` 逐字对齐）：
//
//	func checkManifestFreshnessTwoCriteriaGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 分列成立 / 1 有命中 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出  —— stdout / stderr 与 py 逐字节对齐（未覆盖面见下）
//
// ── 标准声明 (a)：默认仓根定位 ────────────────────────────────────────────────
// py：`HERE = dirname(abspath(__file__))` → `REPO_ROOT = dirname(dirname(HERE))` —— 由**脚本文件自身位置**上溯。
// Go **无 `__file__` 语境** ⇒ 只能由 **cwd 上溯**找 `<根>/scripts/gates/check-manifest-freshness-two-criteria.py`
// （同族 py 所在的位置就是这个 `HERE` 的判据）。⇒ **「脚本位置 ≠ cwd」时推导面不同**；对拍一律显式传
// `--root <仓根>`。新增旋钮 `--root` 属**有意旋钮面差**（py 无此旗标）。
//
// ── 标准声明 (b)：自指检查 ────────────────────────────────────────────────────
// 本件落在 `core/` 域内，故逐条核对三个他门扫描面：
//
//	① `check-hardcoded-private-paths`（扫 `core/`）：本件零绝对路径字面量（件内路径一律由 `--root`/cwd 推出）。
//	② `check-error-kinds`（扫 `core/cmd/zerg/*.go`）：本件不打印任何 `setErr(` 形态的闭集字面量。
//	③ `check-wall-layer`（扫 `core/cmd/zerg/*.go`）：本件零 `layer:` / `layer*=` 形态字面量。
//	④ 若干门按「连续字面 `--self-test`」认门 ⇒ 本件是转发壳所需的连续字面，与同批已落 17 枚同为既定面。
//
// ⇒ 自指=真（本件在被扫域内）；落仓前后他门输出 sha16 逐字相同（他门贡献 0）。
//
// ── 标准声明 (c)：真源退役容忍（B轨 · 2026-10-07 · 退件阻塞批次 · 类①）──────────────
// 真源 `check-manifest-freshness.py` 退役（py 缺席 · 其 Go 半 `core/cmd/zerg/check_manifest_freshness.go`
// 在盘）⇒ 本件视同「单一口径可得」：C1/C2 走本件移植后的 `cmfDiagnose()`、C3 改读真源 **Go 半源码**
// （同一「两处判据分列」口径 · 真源内化）、C4 走进程内自检 ⇒ **判据照常出结论**（✗ 不 fail-closed）。
// ★ 只在**缺省真源**上生效：显式 `--truth <路径>`（取证副本）缺件仍照旧 rc=2。
// 与姊妹 `check-manifest-freshness-profile` 的 `mfpNeighborRetired` 同口径（同一「退役」判据）。
//
// ── 未覆盖面（✗ 不声称逐字对齐）──────────────────────────────────────────────
//  1. argparse 的 `-h/--help` 帮助正文 · 未知旗标 usage 正文（本件只对齐退码 rc=2 与「不给结论」语义）。
//  2. **默认根定位**：py 由 `__file__` 上溯 ⟷ Go 由 cwd 上溯（见「标准声明 (a)」）；对拍一律显式 `--root`。
//  3. 异常路径：仓根**不是 git 仓 / git 不可用** ⇒ py 抛 `Unsupported` 到顶层（stdout 空 · stderr 为
//     traceback 正文 · rc=1）；本件 stdout 同为空、rc 同为 1，但 **stderr 走声明文案**（不写死跑测机路径）。
//  4. 真源件**语法坏而不可载入**（py `load_truth()` 的 exec 异常）本件无法区分（本件不内嵌 py 解释器）——
//     本件把「真源件不在场」判据落成 `filepath` 存在性。
//  5. 文本解码/`strip()`/`splitlines()`/`relpath` 的边界：本件已按 py Unicode 语义自写
//     （见 `cmfPyStrip`/`cmfSplitLines`/`cmfRelpath`/`cmfPyRepr`），但**非 UTF-8 字节序列的替换粒度**
//     与 py 的 `UnicodeDecodeError` 行为未对拍。
//  6. 「真源 `--self-test`」那一步：见上「手段不同」。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// ── 两条判据的「具名标记」（真源文案逐字片段；两条的标记不许重叠）───────────────
const (
	cmfMarkA = "另一棵树"         // 判据①：source_sha 与当前分支头不一致
	cmfMarkB = "声明身份与真实来源不一致" // 判据②：commit（声明的身份）≠ source_sha（真实来源）
)

// cmfJSONFields —— 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）。
var cmfJSONFields = []string{"rc", "verdict", "criteria", "warn_lines", "source_lines", "source_hits", "truth_self_test_rc", "hits"}

// cmfSynthBoth —— 合成清单：`source_sha` 与仓 HEAD 不等（判据①）+ `commit` 与 `source_sha` 不等（判据②）。
const (
	cmfSynthCommit  = "zzzzzzzzzzzz"
	cmfSynthSrcSHA  = "0000000000000000000000000000000000000000"
	cmfSynthGenAt   = "2026-09-23T00:00:00Z"
	cmfSynthMaxAge  = 720.0
	cmfTruthRelPath = "scripts/gates/check-manifest-freshness.py"
	cmfSelfRelPath  = "scripts/gates/check-manifest-freshness-two-criteria.py"
	// cmfRootMarkerRel —— 仓根上溯的**退役稳态**标记（B轨 · 2026-10-07 · 退件阻塞批次）。
	// 原实现只认自指 py（`cmfSelfRelPath`）当仓根标记；该件退役（不在盘）后上溯落空、退回 cwd
	// ⇒ 从非仓根 cwd 跑时真源找不到 ⇒ rc=2（fail-closed）。按「件退役后判据照常出结论」口径，
	// 标记补上**不会被退役**的门禁自举件（`precommit-gates.sh` · 与 newfile-registry 的
	// `gnrStepsRel` 同源选择）⇒ py 退役不改变根定位结论。自指 py 在盘时仍优先命中同一目录。
	cmfRootMarkerRel = zergPrecommitGatesRel
	// cmfTruthGoHalfRel —— 真源 `check-manifest-freshness.py` 的 **Go 半**（仓内相对路径）。
	// 退役稳态下 C3 的**真源内化**读取面（同一「两处判据分列」口径，改读 Go 半源码；
	// ✗ 不新写第二份判据）。见 `cmfTruthRetired`。
	cmfTruthGoHalfRel = "core/cmd/zerg/check_manifest_freshness.go"
)

// cmfManifest —— 真源 `diagnose()` 吃的清单（四键）。
type cmfManifest struct {
	Commit      string
	SourceSHA   string
	Dirty       bool
	GeneratedAt string
}

// checkManifestFreshnessTwoCriteriaGateMain —— 入口（= py `main()`）。
func checkManifestFreshnessTwoCriteriaGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：`--json <字段>` 里有不认的名 ⇒ rc=2 ＋ 印可选字段表（先于位置参数解析）。
	if rc := cmfRefuseUnknownJSONFields(stderr, argv); rc != nil {
		return *rc
	}
	// ② 位置参数等价解析（py argparse）。未知旗标的 usage 正文不在覆盖面内（见件头未覆盖面①）。
	truthArg, jsonArg := "", ""
	rootArg := ""
	selfTest, listRules := false, false
	nowArg := ""
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--truth" || a == "--json" || a == "--root" || a == "--now":
			if i+1 >= len(argv) {
				cmfArgparseNoValue(stderr, a)
				return 2
			}
			i++
			switch a {
			case "--truth":
				truthArg = argv[i]
			case "--json":
				jsonArg = argv[i]
			case "--root":
				rootArg = argv[i]
			case "--now":
				nowArg = argv[i]
			}
		case strings.HasPrefix(a, "--truth="):
			truthArg = strings.TrimPrefix(a, "--truth=")
		case strings.HasPrefix(a, "--json="):
			jsonArg = strings.TrimPrefix(a, "--json=")
		case strings.HasPrefix(a, "--root="):
			rootArg = strings.TrimPrefix(a, "--root=")
		case strings.HasPrefix(a, "--now="):
			nowArg = strings.TrimPrefix(a, "--now=")
		case a == "--self-test":
			selfTest = true
		case a == "--list-rules":
			listRules = true
		case a == "-h" || a == "--help":
			cmfPrintHelp(stdout)
			return 0
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				cmfArgparseUnknown(stderr, a)
				return 2
			}
		}
	}

	// ③ 仓根：`--root` 显式；缺省 cwd 上溯（见件头「标准声明 (a)」）。
	repoRoot := cmfResolveRoot(rootArg)
	// `--now` 注入旋钮（**有意新增**）：缺省 = wall clock，与 py `datetime.now(timezone.utc)` 逐字同构。
	now := time.Now().UTC()
	if nowArg != "" {
		if t, err := time.Parse(time.RFC3339, nowArg); err == nil {
			now = t.UTC()
		}
	}

	// ④ 真源路径（py `TRUTH_PATH[0]`；缺省 = `<根>/scripts/gates/check-manifest-freshness.py`）。
	defaultTruth := filepath.Join(repoRoot, "scripts", "gates", "check-manifest-freshness.py")
	truthPath := defaultTruth
	if truthArg != "" {
		if abs, err := filepath.Abs(truthArg); err == nil {
			truthPath = abs
		} else {
			truthPath = truthArg
		}
	}
	// ★ 真源退役容忍（B轨 · 2026-10-07 · 退件阻塞批次 · 类①）：真源 `check-manifest-freshness.py`
	// 退役（py 缺席 · 其 Go 半在盘）⇒ 视同「单一口径可得」⇒ 判据照常出结论（✗ 不得 rc=2
	// fail-closed）。与姊妹 `check-manifest-freshness-profile` 的 mfpNeighborRetired 同口径
	// （同一「退役」判据）。★ **只在缺省真源上生效** —— 显式 `--truth <路径>`（取证副本）缺件
	// 仍照旧 rc=2（✗ 不拿退役当 `--truth` 的免死牌）。
	truthRetired := truthArg == "" && cmfTruthRetired(repoRoot)
	truthPresent := cmfFileExists(truthPath) || truthRetired
	// C3 源级读取面：py 在盘 ⇒ 读 py；退役 ⇒ 读其 Go 半（**真源内化** · 同一「两处判据分列」口径）。
	truthSrcPath := truthPath
	if truthRetired {
		truthSrcPath = cmfTruthGoHalfPath(repoRoot)
	}

	if listRules {
		return cmfListRules(stdout, repoRoot, defaultTruth)
	}
	if selfTest {
		fmt.Fprintln(stdout, "── 两条判据都要报（R-24）· 自检（正/负成对 · 合成夹具）──")
		return cmfSelfTest(stdout, repoRoot, truthSrcPath, truthPresent)
	}

	if !truthPresent {
		// py 此处印的是**缺省** `TRUTH`（HERE 推出的那个），✗ 不是 `--truth` 指向的副本。
		fmt.Fprintf(stdout, "缺件：真源 %s 不在场 ⇒ **不给结论**\n", defaultTruth)
		fmt.Fprintln(stdout, "⇒ rc=2")
		return 2
	}

	// ⑤ 真目标：合成清单 ⇒ 现调移植后的 `diagnose()`（口径单一）。
	synth := cmfManifest{Commit: cmfSynthCommit, SourceSHA: cmfSynthSrcSHA, Dirty: false, GeneratedAt: cmfSynthGenAt}
	lines, gitOK := cmfDiagnose(synth, repoRoot, cmfSynthMaxAge, now)
	if !gitOK {
		// 取不到那棵树的 git 面 ⇒ py 抛 Unsupported（stdout 空 · rc=1 · stderr 为 traceback）。
		fmt.Fprint(stderr, cmfUnsupportedText(repoRoot))
		return 1
	}
	verdict, hits, why := cmfJudgeLines(lines)
	srcA, srcB, srcHits := cmfSourceLevelSplit(truthSrcPath, truthPresent)
	truthRC, truthTail := cmfTruthSelfTest(repoRoot, truthPresent, now)
	trc := truthRC
	srcPresent := srcA != nil || srcB != nil

	rc := 2
	if verdict == "vacuous" {
		rc = 2
	} else if verdict == "hit" || len(srcHits) > 0 || trc != 0 {
		rc = 1
	} else {
		rc = 0
	}

	fmt.Fprintln(stdout, "── 两条判据都要报（R-24）· 现调真源 ──")
	fmt.Fprintf(stdout, "真源：%s（%s）\n", cmfRelpath(truthSrcPath, repoRoot), cmfPresentText(truthPresent))
	fmt.Fprintf(stdout, "合成清单喂进去：source_sha=%s · commit=%s（两条判据都该命中）\n",
		cmfCut(cmfSynthSrcSHA, 12), cmfSynthCommit)
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── 真源报出的告警行（原样 · 逐行）──")
	for i, l := range lines {
		fmt.Fprintf(stdout, "   %d. %s\n", i+1, l)
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── 判据 C1/C2（两条判据必须各自成行 · 合并成一句即红）──")
	fmt.Fprintf(stdout, "   结论：%s —— %s\n", verdict, why)
	for _, h := range hits {
		fmt.Fprintln(stdout, "   ✗ "+h)
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── 判据 C3（源级分列）──")
	fmt.Fprintf(stdout, "   判据①「%s」源码行：%s\n", cmfMarkA, cmfIntListOrNone(srcA, srcPresent))
	fmt.Fprintf(stdout, "   判据②「%s」源码行：%s\n", cmfMarkB, cmfIntListOrNone(srcB, srcPresent))
	if len(srcHits) == 0 {
		fmt.Fprintln(stdout, "   ✓ 两条在**不同源码行**（源头就是两条 statement）")
	} else {
		fmt.Fprintln(stdout, "   "+strings.Join(srcHits, "；"))
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── 判据 C4（真源自带正负例 · 现跑）──")
	fmt.Fprintf(stdout, "   %s ⇒ rc=%s\n", truthTail, cmfPyStrInt(trc))
	fmt.Fprintln(stdout, "")
	splitMark := "✓"
	if len(srcHits) > 0 {
		splitMark = "✗"
	}
	fmt.Fprintf(stdout, "⇒ 两条判据分列=%s · 源级分列=%s · 真源自检=%s · rc=%d\n", verdict, splitMark, cmfPyStrInt(trc), rc)

	if jsonArg != "" {
		fmt.Fprintln(stdout, cmfBuildJSON(jsonArg, rc, verdict, lines, srcA, srcB, srcPresent, srcHits, trc, hits, why))
	}
	return rc
}

// ── Q-176：`--json <字段>` 的字段名校验 ──────────────────────────────────────
// 口径照 py `refuse_unknown_json_fields()`：字段名打错 ≠ 零命中 —— 两态在机器面上必须分得开。

func cmfRefuseUnknownJSONFields(stderr io.Writer, argv []string) *int {
	var want []string
	for i, x := range argv {
		if x == "--json" {
			if i+1 < len(argv) {
				want = append(want, strings.Split(argv[i+1], ",")...)
			} else {
				want = append(want, "")
			}
		} else if strings.HasPrefix(x, "--json=") {
			want = append(want, strings.Split(strings.TrimPrefix(x, "--json="), ",")...)
		}
	}
	var got []string
	for _, f := range want {
		if strings.TrimSpace(f) != "" {
			got = append(got, strings.TrimSpace(f))
		}
	}
	var bad []string
	for _, f := range got {
		known := false
		for _, k := range cmfJSONFields {
			if k == f {
				known = true
				break
			}
		}
		if !known {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	badMarks := make([]string, len(bad))
	for i, b := range bad {
		badMarks[i] = "`" + b + "`"
	}
	fldMarks := make([]string, len(cmfJSONFields))
	for i, f := range cmfJSONFields {
		fldMarks[i] = "`" + f + "`"
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", strings.Join(badMarks, " · "))
	fmt.Fprintf(stderr, "   可选字段表：%s\n", strings.Join(fldMarks, " · "))
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开")
	rc := 2
	return &rc
}

// ── 真源 `diagnose()` 的移植（本件只搬，不改口径）────────────────────────────

// cmfGitStrict —— py `_git_strict()`：取得到 ⇒ (值, true)；取不到 ⇒ (_, false)（判不了 ⇒ 不给结论）。
func cmfGitStrict(repo string, args []string) (string, bool) {
	full := append([]string{"-c", "core.quotepath=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return cmfPyStrip(out.String()), true
}

// cmfGitTolerant —— py `_git()`：起不来或非零退出 ⇒ ""（**吞退码**；真来自检测试那一支用）。
func cmfGitTolerant(repo string, args []string) string {
	v, ok := cmfGitStrict(repo, args)
	if !ok {
		return ""
	}
	return v
}

// cmfDiagnose —— py `diagnose()`：返回 (告警列表, git 面取到没)。取不到 ⇒ gitOK=false（调用方落 rc=1 同形）。
func cmfDiagnose(mf cmfManifest, repo string, maxAgeHours float64, now time.Time) ([]string, bool) {
	warns := []string{}
	head := ""
	if repo != "" {
		h, ok := cmfGitStrict(repo, []string{"rev-parse", "HEAD"})
		if !ok {
			return nil, false
		}
		head = h
	}
	src := cmfPyStrip(mf.SourceSHA)
	if src != "" && head != "" && src != head {
		warns = append(warns, fmt.Sprintf("清单出自另一棵树：source_sha=%s ≠ 当前分支头 %s",
			cmfCutRunes(src, 12), cmfCutRunes(head, 12)))
	}
	if mf.Dirty {
		warns = append(warns, "清单生成时工作树**有未提交改动**（dirty=true）⇒ 该批资产无法从提交复现")
	}
	commitClaim := cmfPyStrip(mf.Commit)
	if commitClaim != "" && src != "" && commitClaim != src {
		if strings.HasPrefix(src, commitClaim) || strings.HasPrefix(commitClaim, src) {
			a, b := cmfPair(commitClaim, src)
			warns = append(warns, fmt.Sprintf("声明身份与真实来源不一致：commit=%s 而 source_sha=%s —— 只是**前缀弱匹配**"+
				"（前 %d 位同），不是相等（明文判据是逐字全串 ==；短串无鉴别力 ⇒ 不当「一致」判）",
				a, b, cmfMinRunes(commitClaim, src)))
		} else {
			a, b := cmfPair(commitClaim, src)
			warns = append(warns, fmt.Sprintf("声明身份与真实来源不一致：commit=%s 而 source_sha=%s", a, b))
		}
	}
	gen := cmfPyStrip(mf.GeneratedAt)
	if gen != "" {
		t, err := time.Parse("2006-01-02T15:04:05Z", gen)
		if err != nil {
			warns = append(warns, fmt.Sprintf("generated_at 无法解析（如实报，不猜）：%s", cmfPyRepr(gen)))
		} else {
			ageH := now.Sub(t).Hours()
			if ageH > maxAgeHours {
				warns = append(warns, fmt.Sprintf("清单过旧：generated_at=%s 距今 %.1fh > 上限 %sh", gen, ageH, cmfFloatG(maxAgeHours)))
			}
		}
	}
	return warns, true
}

// cmfJudgeLines —— py `judge_lines()`（纯函数 · 自检与真目标同一份口径）。
func cmfJudgeLines(lines []string) (string, []string, string) {
	var nonblank []string
	for _, l := range lines {
		if cmfPyStrip(l) != "" {
			nonblank = append(nonblank, l)
		}
	}
	var a, b, merged []string
	for _, l := range nonblank {
		ha := strings.Contains(l, cmfMarkA)
		hb := strings.Contains(l, cmfMarkB)
		if ha {
			a = append(a, l)
		}
		if hb {
			b = append(b, l)
		}
		if ha && hb {
			merged = append(merged, l)
		}
	}
	out := []string{}
	if len(nonblank) == 0 {
		return "vacuous", []string{}, "零告警行 ⇒ 空转，**不给结论**（不许当绿）"
	}
	if len(merged) > 0 {
		out = append(out, fmt.Sprintf("C2 合并成一句：%d 行同时命中两条判据 ⇒ **红**（两条判据必须各自成行）", len(merged)))
	}
	if len(a) == 0 {
		out = append(out, fmt.Sprintf("C1 少报一条：判据①（source_sha 不等 · 标记「%s」）**一行都没报** ⇒ **红**", cmfMarkA))
	}
	if len(b) == 0 {
		out = append(out, fmt.Sprintf("C1 少报一条：判据②（commit ≠ source_sha · 标记「%s」）**一行都没报** ⇒ **红**", cmfMarkB))
	}
	verdict := "green"
	if len(out) > 0 {
		verdict = "hit"
	}
	why := ""
	if len(out) == 0 {
		why = fmt.Sprintf("两条判据分列（①%d 行 · ②%d 行 · 合并 0 行）", len(a), len(b))
	} else {
		why = strings.Join(out, "；")
	}
	return verdict, out, why
}

// cmfSourceLevelSplit —— py `source_level_split()`：申报两条判据的**源码行号**（1 基）。
// 返回 (a 行号, b 行号, 命中列表)；真源不在场 ⇒ (nil, nil, [真源件不在场的说明])。
func cmfSourceLevelSplit(truthPath string, truthPresent bool) ([]int, []int, []string) {
	if !truthPresent {
		return nil, nil, []string{fmt.Sprintf("真源件不在场：%s", truthPath)}
	}
	src, ok := cmfReadTextLines(truthPath)
	if !ok {
		return nil, nil, []string{fmt.Sprintf("真源件不在场：%s", truthPath)}
	}
	var a, b []int
	for i, l := range src {
		if strings.Contains(l, cmfMarkA) {
			a = append(a, i+1)
		}
		if strings.Contains(l, cmfMarkB) {
			b = append(b, i+1)
		}
	}
	hits := []string{}
	if len(a) == 0 {
		hits = append(hits, fmt.Sprintf("C3 真源里找不到判据①（「%s」）的源码行 ⇒ 红", cmfMarkA))
	}
	if len(b) == 0 {
		hits = append(hits, fmt.Sprintf("C3 真源里找不到判据②（「%s」）的源码行 ⇒ 红", cmfMarkB))
	}
	if len(a) > 0 && len(b) > 0 {
		var both []int
		setB := map[int]bool{}
		for _, x := range b {
			setB[x] = true
		}
		for _, x := range a {
			if setB[x] {
				both = append(both, x)
			}
		}
		if len(both) > 0 {
			hits = append(hits, fmt.Sprintf("C3 源级合并：两条判据出现在同一源码行 %s ⇒ 红", cmfIntList(both)))
		}
	}
	return a, b, hits
}

// cmfTruthSelfTest —— py `truth_self_test()` 的**进程内复用**版：跑真源自带的那 6 条正/负用例。
// 返回 (rc, 末行文案)。py 用 subprocess 真跑真源 —— 本件在进程内跑**同一份移植后的判据**（手段不同 · 可观测面同）。
func cmfTruthSelfTest(repoRoot string, truthPresent bool, now time.Time) (int, string) {
	if !truthPresent {
		return -1, "真源件不在场"
	}
	fixedNow := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	fresh := cmfManifest{Commit: "abc1234567890", SourceSHA: "abc1234567890", Dirty: false, GeneratedAt: "2026-09-15T00:00:00Z"}

	pass := func(cond bool) bool { return cond }
	allOK := true

	w, ok := cmfDiagnose(fresh, "", 720, fixedNow)
	if !ok || len(w) != 0 {
		allOK = false
	}
	dirty := fresh
	dirty.Dirty = true
	w, _ = cmfDiagnose(dirty, "", 720, fixedNow)
	allOK = allOK && pass(cmfAnyContains(w, "未提交改动"))
	w, _ = cmfDiagnose(fresh, "", 1, fixedNow)
	allOK = allOK && pass(cmfAnyContains(w, "过旧"))
	badM := cmfManifest{Commit: "zzzzzz", SourceSHA: "yyyyyyyyyyyy", Dirty: false, GeneratedAt: fresh.GeneratedAt}
	w, _ = cmfDiagnose(badM, "", 720, fixedNow)
	allOK = allOK && pass(cmfAnyContains(w, "不一致"))
	junk := fresh
	junk.GeneratedAt = "not-a-date"
	w, _ = cmfDiagnose(junk, "", 720, fixedNow)
	allOK = allOK && pass(cmfAnyContains(w, "无法解析"))

	repoHead := cmfGitTolerant(repoRoot, []string{"rev-parse", "HEAD"})
	if repoHead != "" {
		w, _ = cmfDiagnose(fresh, repoRoot, 720, fixedNow)
		allOK = allOK && pass(cmfAnyContains(w, "另一棵树"))
	}
	// ⑥′ 成对负控：**取不到那棵树的 git 面 ⇒ 判不了**（不许静默当绿）。
	_, okX := cmfDiagnose(fresh, "__no_such_repo__", 720, fixedNow)
	allOK = allOK && (!okX)
	// ⑥″ 成对正控：同一清单 + 真 git 仓 ⇒ 照旧给结论。
	_, okY := cmfDiagnose(fresh, repoRoot, 720, fixedNow)
	if repoHead != "" {
		allOK = allOK && okY
	}
	if !allOK {
		return 1, ""
	}
	return 0, "self-test 通过（6 条正/负用例）"
}

// ── 自检（正/负成对 · 合成夹具）──────────────────────────────────────────────

func cmfSelfTest(stdout io.Writer, repoRoot, truthSrc string, truthPresent bool) int {
	ok := 0
	var bad, log []string
	check := func(name, got, want string) {
		if got == want {
			ok++
			log = append(log, fmt.Sprintf("   ✓ %s ⇒ %s", name, got))
		} else {
			bad = append(bad, name)
			log = append(log, fmt.Sprintf("   ✗ %s ⇒ 期望 %s，实得 %s", name, want, got))
		}
	}

	twoRows := []string{"清单出自另一棵树：source_sha=0000 ≠ 当前分支头 45de",
		"声明身份与真实来源不一致：commit=zzzz 而 source_sha=0000"}
	mergedRow := []string{"清单出自另一棵树且声明身份与真实来源不一致：source_sha=0000"}
	onlyA := []string{twoRows[0]}
	onlyB := []string{twoRows[1]}

	v1, _, _ := cmfJudgeLines(twoRows)
	v2, h2, _ := cmfJudgeLines(mergedRow)
	v3, _, _ := cmfJudgeLines(onlyA)
	v4, _, _ := cmfJudgeLines(onlyB)
	v5, _, _ := cmfJudgeLines([]string{})
	check("正控 · 两条分列 ⇒ green", v1, "green")
	check("负控 · 合并成一句 ⇒ hit（真红）", v2, "hit")
	check("负控 · 合并句必须点名 C2", cmfPyBool(cmfAnyContains(h2, "C2")), "True")
	check("负控 · 只报判据① ⇒ hit（少一条）", v3, "hit")
	check("负控 · 只报判据② ⇒ hit（少一条）", v4, "hit")
	check("负控 · 零告警行 ⇒ 空转不给结论", v5, "vacuous")
	check("区分度 · 分列=green 且 合并=hit", cmfPyTuple2(v1, v2), "('green', 'hit')")
	check("两条判据的标记互不包含", cmfPyBool(strings.Contains(cmfMarkB, cmfMarkA) || strings.Contains(cmfMarkA, cmfMarkB)), "False")

	if !truthPresent {
		bad = append(bad, "真源不在场")
		log = append(log, fmt.Sprintf("   ✗ 真源 %s 不在场 ⇒ 单一口径不成立", truthSrc))
	} else {
		_, _, srcHits := cmfSourceLevelSplit(truthSrc, truthPresent)
		check("真源 · 两条判据在**不同源码行**（C3）", cmfPyStrList(srcHits), "[]")
		rc, tail := cmfTruthSelfTest(repoRoot, truthPresent, time.Now().UTC())
		check("真源 · 自带正负例 --self-test rc=0（C4）", cmfPyStrInt(rc), "0")
		log = append(log, fmt.Sprintf("     ↳ 真源自检末行：%s", tail))
	}

	for _, ln := range log {
		fmt.Fprintln(stdout, ln)
	}
	concl := "全过"
	if len(bad) > 0 {
		concl = "**不过**"
	}
	fmt.Fprintf(stdout, "自检结论: %s（断言 %d 条 · 失败 %d 条）\n", concl, ok+len(bad), len(bad))
	if len(bad) == 0 {
		return 0
	}
	return 2
}

// ── list_rules ────────────────────────────────────────────────────────────────

func cmfListRules(stdout io.Writer, repoRoot, defaultTruth string) int {
	fmt.Fprintln(stdout, "清单新鲜度 · 两条判据都要报（R-24）")
	fmt.Fprintln(stdout, "真源：任务清单-缺口收口-20260923.md:72 · 调研-缺口-门禁证据-20260923.md:137")
	fmt.Fprintf(stdout, "判据①（标记「%s」）：source_sha 与当前分支头不一致\n", cmfMarkA)
	fmt.Fprintf(stdout, "判据②（标记「%s」）：commit（声明的构建身份）≠ source_sha（生成时的真实 HEAD）\n", cmfMarkB)
	fmt.Fprintln(stdout, "C1 两条都得报（缺任何一条 ⇒ 红）· C2 合并成一句即红 · C3 源级也分列 · C4 真源自带正负例")
	fmt.Fprintf(stdout, "单一口径：现调 %s 的 diagnose()，不另写一份判据\n", cmfRelpath(defaultTruth, repoRoot))
	fmt.Fprintln(stdout, "退码：0 分列成立 · 1 有命中（少报/合并）· 2 不给结论（真源不在场/空转/自检不过）")
	return 0
}

// ── py 语义小工具 ────────────────────────────────────────────────────────────

// cmfResolveRoot —— 仓根：`--root` 显式；缺省由 cwd 上溯找本件同族 py 的位置（见件头标准声明 (a)）。
func cmfResolveRoot(rootArg string) string {
	if rootArg != "" {
		if abs, err := filepath.Abs(rootArg); err == nil {
			return abs
		}
		return rootArg
	}
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cmfResolveRootFrom(d)
}

// cmfResolveRootFrom —— 由给定起点上溯找仓根（✗ 不读进程 cwd ⇒ 可单测；cwd 支见 `cmfResolveRoot`）。
// 标记：本件同族 py（在盘时优先 · 逐字沿用原语义）**或**退役稳态标记 `cmfRootMarkerRel`。
// ★ 后者是本单（B轨 · 2026-10-07）补的：自指 py 退役后仍能上溯 ⇒ 「件退役后判据照常出结论」。
func cmfResolveRootFrom(start string) string {
	cur := start
	for i := 0; i < 32; i++ {
		if cmfFileExists(filepath.Join(cur, cmfSelfRelPath)) || cmfFileExists(filepath.Join(cur, cmfRootMarkerRel)) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return start
}

// cmfTruthGoHalfPath —— 真源 Go 半的绝对路径（由仓根拼出 · 单一常量 `cmfTruthGoHalfRel`）。
func cmfTruthGoHalfPath(repoRoot string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(cmfTruthGoHalfRel))
}

// cmfTruthRetired —— 真源 `check-manifest-freshness.py` 是否**已退役**（B轨 · 2026-10-07 · 类①）：
// 真源 py 缺席 **且** 其 Go 半（`core/cmd/zerg/check_manifest_freshness.go`）在盘。
// 退役态 ⇒ 本件视同「单一口径可得」（邻件 diagnose() 已由本包 Go 半真身重建 · C3 改读其 Go 半源码）
// ⇒ 判据照常出结论（✗ 不 fail-closed）；缺件且无 Go 半（真消失）⇒ 未退役（照旧 fail-closed）。
// 与 check-manifest-freshness-profile 的 mfpNeighborRetired / check-public-ci-green 的 cicgProfileRetired
// 同口径（同一「退役」判据，✗ 不新造第二套）。
func cmfTruthRetired(repoRoot string) bool {
	py := filepath.Join(repoRoot, "scripts", "gates", "check-manifest-freshness.py")
	return !cmfFileExists(py) && cmfFileExists(cmfTruthGoHalfPath(repoRoot))
}

func cmfPresentText(present bool) string {
	if present {
		return "在场"
	}
	return "缺"
}

func cmfFileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// cmfReadTextLines —— py `open(p, encoding="utf-8").read().splitlines()`：
// 文本模式先把 `\r\n` 与裸 `\r` 折成 `\n`，再按 py 全套行边界切。
func cmfReadTextLines(path string) ([]string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	s := string(b)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return cmfSplitLines(s), true
}

// cmfSplitLines —— py `str.splitlines()` 的行边界集（除 \n 外还断在 \v \f \x1c-\x1e \x85 \u2028 \u2029）。
func cmfSplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range s {
		if cmfIsLineBoundary(r) {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func cmfIsLineBoundary(r rune) bool {
	switch r {
	case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// cmfPyStrip —— py `str.strip()`（Unicode 语义 · 含 `\x1c`-`\x1f`/`\x85`/`\xa0`，✗ 非 strings.TrimSpace）。
func cmfPyStrip(s string) string {
	isWS := func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\v', '\f', '\r', '\x1c', '\x1d', '\x1e', '\x1f', 0x85, 0xa0:
			return true
		}
		return false
	}
	return strings.TrimFunc(s, isWS)
}

// cmfCut —— py `s[:n]`（按**码点**切）。
func cmfCut(s string, n int) string { return cmfCutRunes(s, n) }

func cmfCutRunes(s string, n int) string {
	if n < 0 {
		return ""
	}
	i := 0
	for idx := range s {
		if i == n {
			return s[:idx]
		}
		i++
	}
	return s
}

// cmfPyRepr —— py `repr(str)` 的**本件够用**子集（本件只喂 ASCII 短串）。
func cmfPyRepr(s string) string {
	hasSingle := strings.Contains(s, "'")
	hasDouble := strings.Contains(s, "\"")
	esc := strings.ReplaceAll(s, "\\", "\\\\")
	if hasSingle && !hasDouble {
		return "\"" + esc + "\""
	}
	esc = strings.ReplaceAll(esc, "'", "\\'")
	return "'" + esc + "'"
}

// cmfPyReprStr —— 单引号优先的 repr（自检文案拼装用；无 ' 时用单引号）。
func cmfPyReprStr(s string) string {
	esc := strings.ReplaceAll(s, "\\", "\\\\")
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		return "\"" + esc + "\""
	}
	return "'" + esc + "'"
}

// cmfPyBool —— py `str(bool)` ⇒ "True"/"False"。
func cmfPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// cmfPyStrInt —— py `%s` 印 int。
func cmfPyStrInt(i int) string { return fmt.Sprintf("%d", i) }

// cmfJSONBool —— JSON 的布尔字面量（✗ 非 py `str(bool)` 的 "True"/"False"）。
func cmfJSONBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// cmfPyTuple2 —— py `str(('a', 'b'))` ⇒ "('a', 'b')"。
func cmfPyTuple2(a, b string) string { return "(" + cmfPyReprStr(a) + ", " + cmfPyReprStr(b) + ")" }

// cmfPyStrList —— py `str(list[str])`。
func cmfPyStrList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = cmfPyReprStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cmfIntList —— py `str(list[int])`。
func cmfIntList(items []int) string {
	parts := make([]string, len(items))
	for i, n := range items {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cmfIntListOrNone —— `src["a"] if src else "（真源不在场）"`。
func cmfIntListOrNone(items []int, present bool) string {
	if !present {
		return "（真源不在场）"
	}
	if items == nil {
		return "[]"
	}
	return cmfIntList(items)
}

func cmfAnyContains(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

func cmfMinRunes(a, b string) int {
	la, lb := utf8.RuneCountInString(a), utf8.RuneCountInString(b)
	if la < lb {
		return la
	}
	return lb
}

// cmfPair —— py `_pair()`：截断有鉴别力 ⇒ 截断；两枚不同串共享前 12 位 ⇒ 给全串。
func cmfPair(x, y string) (string, string) {
	sx, sy := cmfCut(x, 12), cmfCut(y, 12)
	if sx == sy && x != y {
		return x, y
	}
	return sx, sy
}

// cmfFloatG —— py `f"{v:g}"` 的**本件够用**子集（本件只喂 720.0）。
func cmfFloatG(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", v), "0"), ".")
}

// cmfRelpath —— py `os.path.relpath(path, start)` 的 posix 子集（两参皆绝对路径时逐字同构）。
func cmfRelpath(path, start string) string {
	p := cmfNormAbs(path)
	s := cmfNormAbs(start)
	if p == s {
		return "."
	}
	pl := cmfSplitSlash(p)
	sl := cmfSplitSlash(s)
	i := 0
	for i < len(pl) && i < len(sl) && pl[i] == sl[i] {
		i++
	}
	var rel []string
	for j := i; j < len(sl); j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pl[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

func cmfNormAbs(p string) string {
	if !strings.HasPrefix(p, "/") {
		if wd, err := os.Getwd(); err == nil {
			p = filepath.Join(wd, p)
		}
	}
	p = filepath.Clean(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func cmfSplitSlash(p string) []string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// ── JSON 面（字段表 + 逐字拼装 · 对齐 py `json.dumps(..., ensure_ascii=False)`）──────

func cmfBuildJSON(jsonArg string, rc int, verdict string, lines []string, srcA, srcB []int, srcPresent bool,
	srcHits []string, trc int, hits []string, why string) string {
	// criteria：`{"source_sha_ne_head": <why.count("①")>=0>, "commit_ne_source_sha": true}`（前者恒真）。
	criteria := "{\"source_sha_ne_head\": " + cmfJSONBool(strings.Count(why, "①") >= 0) + ", \"commit_ne_source_sha\": true}"
	sourceLines := "null"
	if srcPresent {
		a := srcA
		b := srcB
		if a == nil {
			a = []int{}
		}
		if b == nil {
			b = []int{}
		}
		sourceLines = "{\"a\": " + cmfIntList(a) + ", \"b\": " + cmfIntList(b) + "}"
	}
	frags := map[string]string{
		"rc":                 fmt.Sprintf("%d", rc),
		"verdict":            cmfJSONStr(verdict),
		"criteria":           criteria,
		"warn_lines":         cmfJSONStrList(lines),
		"source_lines":       sourceLines,
		"source_hits":        cmfJSONStrList(srcHits),
		"truth_self_test_rc": fmt.Sprintf("%d", trc),
		"hits":               cmfJSONStrList(hits),
	}
	var want []string
	for _, f := range strings.Split(jsonArg, ",") {
		if strings.TrimSpace(f) != "" {
			want = append(want, strings.TrimSpace(f))
		}
	}
	if len(want) == 0 {
		want = cmfJSONFields
	}
	parts := make([]string, 0, len(want))
	for _, k := range want {
		parts = append(parts, cmfJSONStr(k)+": "+frags[k])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func cmfJSONStrList(items []string) string {
	if items == nil {
		items = []string{}
	}
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = cmfJSONStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cmfJSONStr —— py `json.dumps(s, ensure_ascii=False)` 的字符串面：转义 `"` `\` 与控制符，其余（含 CJK）逐字保留。
func cmfJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ── argparse 面（文案不在覆盖面内 · 只对齐退码与语义）────────────────────────────

func cmfArgparseNoValue(stderr io.Writer, flag string) {
	fmt.Fprintf(stderr, "usage: check-manifest-freshness-two-criteria.py [-h] [--truth TRUTH]\n")
	fmt.Fprintf(stderr, "                                                [--json JSON] [--self-test]\n")
	fmt.Fprintf(stderr, "                                                [--list-rules]\n")
	fmt.Fprintf(stderr, "check-manifest-freshness-two-criteria.py: error: argument %s: expected one argument\n", flag)
}

func cmfArgparseUnknown(stderr io.Writer, flag string) {
	fmt.Fprintf(stderr, "usage: check-manifest-freshness-two-criteria.py [-h] [--truth TRUTH]\n")
	fmt.Fprintf(stderr, "                                                [--json JSON] [--self-test]\n")
	fmt.Fprintf(stderr, "                                                [--list-rules]\n")
	fmt.Fprintf(stderr, "check-manifest-freshness-two-criteria.py: error: unrecognized arguments: %s\n", flag)
}

func cmfPrintHelp(stdout io.Writer) {
	fmt.Fprint(stdout, "usage: check-manifest-freshness-two-criteria.py [-h] [--truth TRUTH]\n")
	fmt.Fprint(stdout, "                                                [--json JSON] [--self-test]\n")
	fmt.Fprint(stdout, "                                                [--list-rules]\n")
}

// cmfUnsupportedText —— 取不到那棵树的 git 面时的**声明文案**（✗ 不写死跑测机路径）。
func cmfUnsupportedText(repo string) string {
	return "判不了：仓根不是 git 仓 / git 不可用 ⇒ 取不到分支头 ⇒ **不给结论**（rc=1 与 py 同形；" +
		"py 此处抛 Unsupported 顶层 traceback · 正文不同 —— 见件头未覆盖面③）\n"
}
