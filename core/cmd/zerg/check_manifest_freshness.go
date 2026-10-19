// check_manifest_freshness.go —— 判据件
// `scripts/gates/check-manifest-freshness.py`（226 行）的 **Go 真搬**（v2.5.14 · M2 批 · 甲案口径）。
//
// 本件做什么（= py `main()`）：
//
//	读一份 manifest.json，按三条 stale 判据 + 一条身份线索出告警；缺省**只告警不拒绝**（rc 恒 0），
//	`--strict` 才把告警升成 rc=1；取不到那棵树的 git 面 ⇒ **不给结论**（rc=2 · `TRACKED_UNKNOWN` 口径）。
//
// ── 被邻件复用的导出面（后续收口批接线用 · 名字 ↔ py 函数 · 语义）──────────────
//
//	· `mfGit`       ← py `_git(repo, args) -> str`        吞异常与退码 ⇒ 取不到一律 ""（**报警用**）。
//	· `mfGitStrict` ← py `_git_strict(repo, args) -> str` 取不到 ⇒ (判词, false)（**判据用**，不吞）。
//	· `mfPair`      ← py `_pair(x, y) -> (a, b)`          12 位截断**有鉴别力**时截断，两串共享前 12 位 ⇒ 给全串。
//	· `mfDiagnose`  ← py `diagnose(mf, repo, max_age_hours, now=None) -> list`  返回 (告警列表, 异常)。
//	· `mfJudge`     ← py `judge(mf, repo, max_age_hours, strict=False, now=None) -> (rc, warns, err)`。
//
// 邻件现读复用面（收口批据此接线）：`check-manifest-freshness-profile.py:130` 现调 `_git()`、
// `:138` 现调 `diagnose()`；`check-manifest-freshness-two-criteria.py:246` 现调 `diagnose()`；
// `_git_strict`/`_pair`/`judge` 为同源可调面（本件一并搬出，待接线）。
//
// 入口签名（与 py `main()` 逐字对齐）：
//
//	func mfMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 告警/无告警 · 1（`--strict` 且有告警）· 2 不给结论 · 64 用法错（逐字对 py）
//	  · 输出  —— stdout / stderr 与 py 逐字节对齐（未覆盖面见下）
//
// ── 标准声明 (a)：默认仓根定位 ────────────────────────────────────────────────
// py：`--repo` 缺省 = `dirname(abspath(__file__)) + "/../.."` —— 由**脚本文件自身位置**上溯。
// Go **无 `__file__` 语境** ⇒ 由 **cwd 上溯**找 `<根>/scripts/gates/check-manifest-freshness.py`
// 所在的那棵树（同族 py 的位置就是这个 `HERE` 的判据）。⇒「脚本位置 ≠ cwd」时推导面不同；对拍一律显式传 `--repo`。
//
// ── 未覆盖面（✗ 不声称逐字对齐）──────────────────────────────────────────────
//  1. argparse 的 `-h/--help` 帮助正文 · 未知旗标 / 缺值的 usage 正文（本件只对齐退码 rc=2 与「不给结论」语义）。
//  2. **默认根定位**：py 由 `__file__` 上溯 ⟷ Go 由 cwd 上溯（见「标准声明 (a)」）；对拍一律显式 `--repo`。
//  3. 异常路径文案：仓根**不是 git 仓 / git 不可用** ⇒ py 抛 `Unsupported`，其文案在**取不到分支头**那一支里
//     内嵌 `OSError` 的**本机路径**（如 `[Errno 2] No such file or directory: ”`）；
//     本件同形给出 `判不了：…（%s）` 骨架但 `%s` 用**声明文案**（不写死跑测机路径）⇒ 这一格只对齐 rc，不对齐 stderr 正文。
//  4. manifest 里 `source_sha`/`commit`/`generated_at` 是**非字符串**时：py 走 `(x or "").strip()`
//     的属性/取值路径（0/False/None ⇒ ""；其余非串 ⇒ AttributeError 崩），本件把「非串且真」判成**取不到**（不给结论），
//     不模拟 AttributeError 的 traceback。
//  5. `generated_at` 时间串的解析边界：py `strptime("%Y-%m-%dT%H:%M:%SZ")` 的宽松度 ⟷ Go `time.Parse` 的严格度未逐一穷举。
//  6. 文本解码 / `strip()` / 非 UTF-8 字节序列的替换粒度未对拍（`mfPyStrip` 按 Go `TrimSpace` 语义）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// mfCodeTrackedUnknown —— py `CODE_TRACKED_UNKNOWN`（同族词表：git 面取不到用这一个）。
const mfCodeTrackedUnknown = "TRACKED_UNKNOWN"

// 退码三档（优先级 2 > 1 > 0，与 check-doc-meta/name/coverage 同口径；另加 64 用法错）。
const (
	mfExitOK      = 0
	mfExitFail    = 1
	mfExitBlocked = 2
	mfExitUsage   = 64
)

// mfJSONFields —— 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）。py `JSON_FIELDS = []`。
var mfJSONFields = []string{}

// mfSelfRel —— 由 cwd 上溯定位仓根用的标记（同族 py 的仓内相对路径）。
const mfSelfRel = pathFreshnessPy

// mfRootMarkerRel —— 仓根上溯的**退役稳态**标记（B轨 · 2026-10-07 · 退件阻塞批次）。
// 原实现只认自指 py（mfSelfRel）当仓根标记；该件退役（不在盘）后上溯落空 ⇒ 从非仓根 cwd
// 跑时真源找不到 ⇒ rc=2（fail-closed）。按「件退役后判据照常出结论」口径，标记补上
// **不会被退役**的门禁自举件（precommit-gates.sh · 与 check-manifest-freshness-two-criteria
// 的 cmfRootMarkerRel 同源选择）⇒ py 退役不改变根定位结论。自指 py 在盘时仍优先命中同一目录。
const mfRootMarkerRel = zergPrecommitGatesRel

// mfEmptyManifestMsg —— ★ 空清单（`{}` / 四键全缺）⇒ **没输入可判**：✗ 不许报「新鲜」。
// ★ 口径（`GAP-20260927-108` · 2026-10-05 拍板）：**rc 契约一字不动**（本门是报告档：缺省退码恒 0、
// 仅 `--strict` 拦）—— 只把文案从「✓ 清单新鲜（无告警）」改成**明说「没输入可判」**；`--strict` 下空清单照旧**拦**（升 rc=1）。
// 逐字节须与 py `check-manifest-freshness.py` 的 `main()` 同串（对拍面）。
const mfEmptyManifestMsg = "⚠ 空清单：没输入可判 —— 本清单未携带任何判据字段（source_sha / dirty / commit / generated_at 全缺）⇒ 不报「新鲜」"

// mfIsEmptyManifest —— 清单里**有没有可判的输入**：非 dict ⇒ false（走既有属性错路径，不算「空」）；
// dict ⇒ 四键（source_sha / dirty / commit / generated_at）**一个都不在**才算空。逐字对齐 py 的同一判定。
func mfIsEmptyManifest(mf any) bool {
	m, ok := mf.(map[string]any)
	if !ok {
		return false
	}
	for _, k := range []string{"source_sha", "dirty", "commit", "generated_at"} {
		if _, present := m[k]; present {
			return false
		}
	}
	return true
}

// ── Unsupported：判不了 ⇒ 一律 rc=2（不给结论），绝不猜。py `Unsupported(Exception)`。───────

type mfUnsupported struct{ msg string }

func (e mfUnsupported) Error() string { return e.msg }

// ── git 面：吞退码的报警探针 / 不吞退码的判据探针 ────────────────────────────────

// mfGit —— py `_git(repo, args) -> str`：起不来或非零退出 ⇒ ""（**吞退码**）。
// ★ py 侧 `cwd=""` 会抛 FileNotFoundError（实测）⇒ 被 `except` 吞成 ""；此处同形：repo=="" ⇒ ""。
func mfGit(repo string, args []string) string {
	if repo == "" {
		return ""
	}
	full := append([]string{"-c", "core.quotepath=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return ""
	}
	return mfPyStrip(out.String())
}

// mfGitStrict —— py `_git_strict(repo, args) -> str`：取得到 ⇒ (值, true)；取不到 ⇒ (判词, false)。
// 判词与 py 的两条具名文案逐字同构（只把内嵌的 OSError 正文换成声明文案，见「未覆盖面 3」）。
func mfGitStrict(repo string, args []string) (string, bool) {
	full := append([]string{"-c", "core.quotepath=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Sprintf("判不了：仓根不是 git 仓 / git 不可用 ⇒ `git %s` 非零退出（rc=%d）",
				strings.Join(args, " "), ee.ExitCode()), false
		}
		return fmt.Sprintf("判不了：仓根不是 git 仓 / git 不可用 ⇒ 取不到分支头（%s）", err.Error()), false
	}
	return mfPyStrip(out.String()), true
}

// ── 出口成对打印：12 位截断有鉴别力时截断；共享前 12 位 ⇒ 给全串。py `_pair(x, y)`。────────

func mfPair(x, y string) (string, string) {
	sx, sy := mfCut(x, 12), mfCut(y, 12)
	if sx == sy && x != y {
		return x, y
	}
	return sx, sy
}

// ── 三条 stale 判据 + 一条身份线索 ─────────────────────────────────────────────

// mfDiagnose —— py `diagnose(mf, repo, max_age_hours, now=None) -> list`：返回 (告警列表, 异常)。
// 异常非空 ⇒ 判不了（取不到那棵树的 git 面，`GAP-20260927-105`）—— 不许让 `head=""` 把判据①静默吞掉。
func mfDiagnose(mf any, repo string, maxAgeHours float64, now time.Time) ([]string, error) {
	warns := []string{}
	head := ""
	if repo != "" {
		h, ok := mfGitStrict(repo, []string{"rev-parse", "HEAD"})
		if !ok {
			return nil, mfUnsupported{h}
		}
		head = h
	}
	m, isDict := mf.(map[string]any)
	if !isDict {
		return nil, fmt.Errorf("'%s' object has no attribute 'get'", mfPyTypeName(mf))
	}
	src, ok := mfPyOrEmptyStr(m["source_sha"])
	if !ok {
		return nil, fmt.Errorf("'%s' object has no attribute 'strip'", mfPyTypeName(m["source_sha"]))
	}
	src = mfPyStrip(src)
	if src != "" && head != "" && src != head {
		warns = append(warns, fmt.Sprintf("清单出自另一棵树：source_sha=%s ≠ 当前分支头 %s",
			mfCut(src, 12), mfCut(head, 12)))
	}
	if b, isB := m["dirty"].(bool); isB && b {
		warns = append(warns, "清单生成时工作树**有未提交改动**（dirty=true）⇒ 该批资产无法从提交复现")
	}
	commitClaim, ok := mfPyOrEmptyStr(m["commit"])
	if !ok {
		return nil, fmt.Errorf("'%s' object has no attribute 'strip'", mfPyTypeName(m["commit"]))
	}
	commitClaim = mfPyStrip(commitClaim)
	if commitClaim != "" && src != "" && commitClaim != src {
		if strings.HasPrefix(src, commitClaim) || strings.HasPrefix(commitClaim, src) {
			a, b := mfPair(commitClaim, src)
			warns = append(warns, fmt.Sprintf("声明身份与真实来源不一致：commit=%s 而 source_sha=%s —— 只是**前缀弱匹配**"+
				"（前 %d 位同），不是相等（明文判据是逐字全串 ==；短串无鉴别力 ⇒ 不当「一致」判）",
				a, b, mfMinRunes(commitClaim, src)))
		} else {
			a, b := mfPair(commitClaim, src)
			warns = append(warns, fmt.Sprintf("声明身份与真实来源不一致：commit=%s 而 source_sha=%s", a, b))
		}
	}
	gen, ok := mfPyOrEmptyStr(m["generated_at"])
	if !ok {
		return nil, fmt.Errorf("'%s' object has no attribute 'strip'", mfPyTypeName(m["generated_at"]))
	}
	gen = mfPyStrip(gen)
	if gen != "" {
		t, perr := time.Parse("2006-01-02T15:04:05Z", gen)
		if perr != nil {
			warns = append(warns, fmt.Sprintf("generated_at 无法解析（如实报，不猜）：%s", mfPyReprStr(gen)))
		} else {
			ageH := now.Sub(t).Hours()
			if ageH > maxAgeHours {
				warns = append(warns, fmt.Sprintf("清单过旧：generated_at=%s 距今 %.1fh > 上限 %sh",
					gen, ageH, mfPyFloatG(maxAgeHours)))
			}
		}
	}
	return warns, nil
}

// mfJudge —— py `judge(...) -> (rc, warns, err)`。`err` 非空 ⇒ 判不了（rc=`mfExitBlocked`）。
func mfJudge(mf any, repo string, maxAgeHours float64, strict bool, now time.Time) (int, []string, string) {
	warns, err := mfDiagnose(mf, repo, maxAgeHours, now)
	if err != nil {
		return mfExitBlocked, nil, err.Error()
	}
	if len(warns) == 0 {
		return mfExitOK, nil, ""
	}
	if strict {
		return mfExitFail, warns, ""
	}
	return mfExitOK, warns, ""
}

// ── Q-176：本门**没有**机器读面 ⇒ 给了 `--json` 不许沉默（退码 = 用法错 rc=2）──────────────

// mfRefuseJSONWithoutMachineFace —— py `refuse_json_without_machine_face(argv=None)`：
// 给了 `--json` ⇒ 逐字说明后返 *rc=2；没给 ⇒ nil（原路照走）。
func mfRefuseJSONWithoutMachineFace(argv []string, stderr io.Writer) *int {
	var hit []string
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return nil
	}
	marked := make([]string, len(hit))
	for i, h := range hit {
		marked[i] = "`" + h + "`"
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(marked, " · "))
	fieldTbl := "无（本门只出人读面）"
	if len(mfJSONFields) > 0 {
		fs := make([]string, len(mfJSONFields))
		for i, f := range mfJSONFields {
			fs[i] = "`" + f + "`"
		}
		fieldTbl = strings.Join(fs, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fieldTbl)
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	rc := mfExitBlocked
	return &rc
}

// ── 默认仓根：由 cwd 上溯（见件头「标准声明 (a)」）─────────────────────────────

func mfDefaultRepo() string {
	d, err := os.Getwd()
	if err != nil {
		return ""
	}
	return mfRepoFrom(d)
}

// mfRepoFrom —— 由给定起点上溯找仓根（✗ 不读进程 cwd ⇒ 可单测；cwd 支见 mfDefaultRepo）。
// 标记：自指 py（在盘时优先 · 逐字沿用原语义）**或**退役稳态标记 mfRootMarkerRel
// （★ B轨 · 2026-10-07：自指 py 退役后仍能上溯 ⇒ 「件退役后判据照常出结论」）。
func mfRepoFrom(start string) string {
	cur := start
	for i := 0; i < 32; i++ {
		if _, e1 := os.Stat(filepath.Join(cur, mfSelfRel)); e1 == nil {
			return cur
		}
		if _, e2 := os.Stat(filepath.Join(cur, mfRootMarkerRel)); e2 == nil {
			return cur
		}
		p := filepath.Dir(cur)
		if p == cur {
			break
		}
		cur = p
	}
	return ""
}

// ── --self-test：6 条正/负用例（照 py `main()` 的自检逐条搬）────────────────────

func mfSelfTest(stdout io.Writer, stderr io.Writer) int {
	now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	fresh := map[string]any{
		"commit":       "abc1234567890",
		"source_sha":   "abc1234567890",
		"dirty":        false,
		"generated_at": "2026-09-15T00:00:00Z",
	}
	fail := func(msg string) int {
		fmt.Fprintf(stderr, "self-test 失败：%s\n", msg)
		return mfExitFail
	}
	// 负用例①：干净且新鲜 ⇒ 零告警
	if w, err := mfDiagnose(fresh, "", 720, now); err != nil || len(w) != 0 {
		return fail("干净清单不应报警")
	}
	// 正用例②：dirty ⇒ 必须报
	if w, _ := mfDiagnose(mfWith(fresh, "dirty", true), "", 720, now); !mfAnyContains(w, "未提交改动") {
		return fail("dirty 必须报")
	}
	// 正用例③：过旧 ⇒ 必须报
	if w, _ := mfDiagnose(fresh, "", 1, now); !mfAnyContains(w, "过旧") {
		return fail("过旧必须报")
	}
	// 正用例④：声明身份与真实来源不一致 ⇒ 必须报
	bad := mfWith(mfWith(fresh, "commit", "zzzzzz"), "source_sha", "yyyyyyyyyyyy")
	if w, _ := mfDiagnose(bad, "", 720, now); !mfAnyContains(w, "不一致") {
		return fail("身份不一致必须报")
	}
	// 正用例⑤：generated_at 垃圾 ⇒ 如实报解析不了（不猜）
	if w, _ := mfDiagnose(mfWith(fresh, "generated_at", "not-a-date"), "", 720, now); !mfAnyContains(w, "无法解析") {
		return fail("垃圾日期必须如实报")
	}
	// 正用例⑥ / ⑥′ / ⑥″：取不到 git 面 ⇒ 判不了；真 git 仓 ⇒ 照旧给结论（成对）
	root := mfDefaultRepo()
	repoHead := mfGit(root, []string{"rev-parse", "HEAD"})
	if repoHead != "" {
		if w, _ := mfDiagnose(fresh, root, 720, now); !mfAnyContains(w, "另一棵树") {
			return fail("source_sha 与给定 HEAD 不符必须报")
		}
	}
	rcX, _, errX := mfJudge(fresh, "__no_such_repo__", 720, false, now)
	if rcX != mfExitBlocked || errX == "" {
		return fail("无 .git 树 ⇒ 必须 rc=2（判不了），不许静默当绿")
	}
	rcY, _, errY := mfJudge(fresh, root, 720, false, now)
	if repoHead != "" {
		if rcY != mfExitOK || errY != "" {
			return fail("真 git 仓 ⇒ 照旧给结论（与 ⑥′ 成对）")
		}
	}
	fmt.Fprintln(stdout, "self-test 通过（6 条正/负用例）")
	return mfExitOK
}

// ── 入口：py `main()` 逐字对齐 ─────────────────────────────────────────────────

func mfMain(stdout, stderr io.Writer, argv []string) int {
	if rc := mfRefuseJSONWithoutMachineFace(argv, stderr); rc != nil {
		return *rc
	}
	var manifest, repo string
	repoSet := false
	maxAge := 720.0
	strict := false
	selfTest := false
	bad := false
	reportBad := func(name string) {
		fmt.Fprintf(stderr, "zerg: 参数解析失败（%s）—— 未覆盖面 1：argparse usage 正文不对齐，只对齐退码 rc=2\n", name)
		bad = true
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--self-test":
			selfTest = true
		case a == "--strict":
			strict = true
		case a == "--repo":
			if i+1 >= len(argv) {
				reportBad("--repo")
			} else {
				i++
				repo = argv[i]
				repoSet = true
			}
		case strings.HasPrefix(a, "--repo="):
			repo = strings.TrimPrefix(a, "--repo=")
			repoSet = true
		case a == "--max-age-hours":
			if i+1 >= len(argv) {
				reportBad("--max-age-hours")
			} else {
				i++
				v, err := strconv.ParseFloat(argv[i], 64)
				if err != nil {
					reportBad("--max-age-hours")
				} else {
					maxAge = v
				}
			}
		case strings.HasPrefix(a, "--max-age-hours="):
			v, err := strconv.ParseFloat(strings.TrimPrefix(a, "--max-age-hours="), 64)
			if err != nil {
				reportBad("--max-age-hours")
			} else {
				maxAge = v
			}
		case a == "-h" || a == "--help":
			// 未覆盖面 1：py argparse 的 help 正文不对齐；此处按「用法错」档收（rc=2）。
			reportBad("-h/--help")
		case strings.HasPrefix(a, "-") && a != "-":
			reportBad(a)
		default:
			manifest = a
		}
	}
	if bad {
		return mfExitBlocked
	}
	if !repoSet {
		repo = mfDefaultRepo()
	}
	if selfTest {
		return mfSelfTest(stdout, stderr)
	}
	if manifest == "" {
		fmt.Fprintln(stdout, "用法：check-manifest-freshness.py <manifest.json> [--repo …] [--max-age-hours N] [--strict]")
		return mfExitUsage
	}
	if fi, err := os.Stat(manifest); err != nil || fi.IsDir() {
		fmt.Fprintln(stdout, "用法：check-manifest-freshness.py <manifest.json> [--repo …] [--max-age-hours N] [--strict]")
		return mfExitUsage
	}
	b, err := os.ReadFile(manifest)
	if err != nil {
		fmt.Fprintln(stdout, "用法：check-manifest-freshness.py <manifest.json> [--repo …] [--max-age-hours N] [--strict]")
		return mfExitUsage
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var mf any
	if err := dec.Decode(&mf); err != nil {
		fmt.Fprintf(stderr, "zerg: manifest 不是合法 JSON：%v\n", err)
		return mfExitBlocked
	}
	rc, warns, errMsg := mfJudge(mf, repo, maxAge, strict, time.Now().UTC())
	if errMsg != "" {
		fmt.Fprintf(stderr, "✗ 不给结论（rc=2）：%s ⇒ **取不到 ≠ 零命中**（两态不许同形）；code=%s\n",
			errMsg, mfCodeTrackedUnknown)
		return mfExitBlocked
	}
	// ★ 空清单（`mfEmptyManifestMsg` 见件头）：**没输入可判** ⇒ ✗ 不许把「零告警」读成「新鲜」。
	//   rc 契约一字不动（报告档）：缺省照落 rc（=mfExitOK）、`--strict` 下空清单**拦**（升 mfExitFail）。
	if len(warns) == 0 && mfIsEmptyManifest(mf) {
		fmt.Fprintln(stdout, mfEmptyManifestMsg)
		if strict {
			return mfExitFail
		}
		return rc
	}
	if len(warns) == 0 {
		fmt.Fprintln(stdout, "✓ 清单新鲜（无告警）")
		return rc
	}
	fmt.Fprintf(stdout, "⚠ 清单新鲜度告警 %d 条（**只告警不拒绝**，依据：分发通道可能落后于分支，放宽窗口而非关掉检查）：\n", len(warns))
	for _, w := range warns {
		fmt.Fprintln(stdout, "   - "+w)
	}
	return rc
}

// ── py 语义小工具（与同批已落 17 枚同口径）─────────────────────────────────────

func mfWith(m map[string]any, k string, v any) map[string]any {
	out := map[string]any{}
	for kk, vv := range m {
		out[kk] = vv
	}
	out[k] = v
	return out
}

func mfAnyContains(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}

// mfPyStrip —— py `str.strip()`（按 Go TrimSpace 语义；见「未覆盖面 6」）。
func mfPyStrip(s string) string { return strings.TrimSpace(s) }

// mfCut —— py `s[:n]`（Unicode 码点截断，非字节）。
func mfCut(s string, n int) string {
	if n < 0 {
		return s
	}
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}

// mfMinRunes —— py `min(len(a), len(b))`（长度是码点数）。
func mfMinRunes(a, b string) int {
	la, lb := utf8.RuneCountInString(a), utf8.RuneCountInString(b)
	if la < lb {
		return la
	}
	return lb
}

// mfPyFloatG —— py `f"{v:g}"`。
func mfPyFloatG(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// mfPyTruthy —— py `bool(v)` 的常用子集。
func mfPyTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return true
		}
		return f != 0
	default:
		return true
	}
}

// mfPyOrEmptyStr —— py `(v or "")` 后接 `.strip()` 的**取值**面：取得到 ⇒ (串, true)；非串且真 ⇒ (_, false)。
func mfPyOrEmptyStr(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", true
	case string:
		return x, true
	default:
		if !mfPyTruthy(v) {
			return "", true
		}
		return "", false
	}
}

// mfPyTypeName —— py `type(v).__name__` 的常用子集。
func mfPyTypeName(v any) string {
	switch v.(type) {
	case nil:
		return "NoneType"
	case bool:
		return "bool"
	case string:
		return "str"
	case json.Number:
		return "int"
	case float64:
		return "float"
	case []any:
		return "list"
	case map[string]any:
		return "dict"
	default:
		return fmt.Sprintf("%T", v)
	}
}

// mfPyReprStr —— py `repr(str)`：缺省单引号；串内含 `'` 而不含 `"` ⇒ 改用双引号。
func mfPyReprStr(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case byte(r) == quote && r < utf8.RuneSelf:
			b.WriteByte('\\')
			b.WriteByte(quote)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}
