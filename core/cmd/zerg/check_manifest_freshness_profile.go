// check_manifest_freshness_profile.go —— 判据件
// `scripts/gates/check-manifest-freshness-profile.py`（353 行）的 **Go 真搬**（v2.5.14 · M2 批 · 甲案口径）。
//
// 本件做什么（= py `main()`）：
//
//	清单新鲜度**分档**（R-23）。两条判据（J1 `manifest.source_sha == 发布提交 HEAD` · J2 `manifest.dirty == false`）
//	逐条分列、不合并；分档只决定「拦不拦」—— dev 档命中只告警（rc 恒 0），public 档命中即红（rc=1）。
//	三条 stale 判据**现调**真源 `check-manifest-freshness.py` 的 `diagnose()`（单一口径，不是本件另抄一份）。
//	退码 0/1/2，与仓内其余门件同口径。
//
// 跨件复用面（★ 本件的核心坑 · 如实交代）：
//
//	py 本件用 `importlib.util.spec_from_file_location` 现载**邻件** `scripts/gates/check-manifest-freshness.py`，
//	取的**真身**是两个东西：① `_git(repo, ["rev-parse","HEAD"])`（`git_head()` 用）· ② `diagnose(mf, repo, age)`
//	（`stale_warnings()` 用）。而 `diagnose` 内部真正取分支头走的是**邻件里的另一支** `_git_strict()`
//	（不吞退码版，取不到即抛 `Unsupported`）—— 即「`_git`（吞退码）／`_git_strict`（不吞）」这对双子。
//	本件按「输入 = 两者」口径把邻件 sha256 一并钉住（见回执）；Go 侧把 `_git`/`_git_strict`/`diagnose`/`_pair`
//	按邻件源码逐字重建（`mfpGitTolerant`/`mfpGitStrict`/`mfpDiagnose`/`mfpPair`）。
//	★ 邻件真身读法：`grep` 同名件**不够** —— 必须看 `importlib` 载的到底是哪一件、调的是哪几个函数
//	（本件经现读确认：载的是 `check-manifest-freshness.py`，调 `_git` 与 `diagnose` 两支）。
//
// wall-clock 面（★ 有意旋钮）：
//
//	邻件 `diagnose()` 第 94 行 `now = now or datetime.datetime.now(datetime.timezone.utc)` —— 本件经
//	`stale_warnings()` 现调它时**不传 now** ⇒ py 侧真吃 **wall clock**（非 mtime）：凡清单带 `generated_at`
//	且超龄，判词里就带 `距今 %.1fh`。⇒ Go 侧新增 `--now <RFC3339>` 注入旋钮**确定化**（缺省 = wall clock，
//	与 py `datetime.now(timezone.utc)` 逐字同构）；py 无此旗标 ⇒ **单列为有意旋钮面差**。
//	★ 用注入旋钮而非掩码：旋钮可使同一档两发逐字节稳定，掩码会把真差一起掩掉。
//
// 入口签名（与 py `main()` 逐字对齐）：
//
//	func checkManifestFreshnessProfileGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 无阻断 / 1 公开制品档有阻断命中 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出  —— stdout / stderr 与 py 逐字节对齐（未覆盖面见下）
//
// ── 标准声明 (a)：默认仓根定位 ────────────────────────────────────────────────
// py：`HERE = dirname(abspath(__file__))` → `REPO_ROOT = dirname(dirname(HERE))` —— 由**脚本文件自身位置**上溯。
// Go **无 `__file__` 语境** ⇒ 只能由 **cwd 上溯**找 `<根>/scripts/gates/check-manifest-freshness-profile.py`
// （同族 py 所在位置即这个 `HERE` 的判据）。⇒ **「脚本位置 ≠ cwd」时推导面不同**；对拍一律显式传
// `--repo <仓根>`。`--repo` 已存在于 py（缺省 = `REPO_ROOT`），非新增旋钮。
// ★ 另：邻件（真源）的落点也由这个「上溯根」推出（py 从 `HERE` 载它），**不**跟 `--repo` 走。
//
// ── 标准声明 (b)：自指检查 ────────────────────────────────────────────────────
// 本件落在 `core/` 域内，故逐条核对三个他门扫描面（按「一旦被跟踪」消解）：
//
//	① `check-hardcoded-private-paths`（扫 `core/` · 命中词 `<volume-path>`·`<volume-path>`·`~`）：
//	   本件零私有绝对路径字面量（件内路径一律由 `--repo`/cwd 推出）⇒ 遗留悬空字面量 **0**。
//	② `check-error-kinds`（扫 `core/cmd/zerg/*.go` · R3 认「setErr 首参字面」与「kind 键取值字面」两类形）：
//	   本件不打印任何闭集 kind 字面量 ⇒ 该门 R3 命中数不变（0 处）· 遗留 **0**。
//	③ `check-wall-layer`（扫 `core/cmd/zerg/*.go` · R1 认层级常量形 · R2 认命令树 `layer` 标记形）：
//	   本件零层级常量形 / 零命令树 `layer` 标记形 ⇒ 遗留 **0**。
//	④ 若干门按「连续字面 `--self-test`」认门 ⇒ 本件是转发壳所需的连续字面（与同批已落 ~30 枚同为既定面）。
//	⇒ 自指=真（本件在被扫域内）；落仓前后他门输出 sha16 逐字相同（本件对它们的贡献 = 0）。
//
// ── 未覆盖面（✗ 不声称逐字对齐）──────────────────────────────────────────────
//  1. `-h/--help` 帮助正文与 argparse 的 usage / error 正文：本件已按 80 列冻结形态逐字复刻
//     （`mfpUsage`/`mfpHelpText`），凡 `COLUMNS` 非 80 或 argparse 版本差异处不声称对齐。
//  2. **默认根定位**：py 由 `__file__` 上溯 ⟷ Go 由 cwd 上溯（见「标准声明 (a)」）；对拍一律显式 `--repo`。
//  3. **JSON 解析错误正文**：清单件坏 JSON 时 py 打 `json.JSONDecodeError` 的 cp 报文
//     （如 `Expecting property name enclosed in double quotes: line 1 column 2 (char 1)`）；
//     本件落「清单件 JSON 解析失败（<Go 的 error 正文>）⇒ **不给结论**」—— rc/stdout 前缀同形，**正文不同**。
//  4. **最外层无兜底档（C 类真口子）**：清单里 `source_sha` 是**真值非串**（如 `123`）时 py `judge()`
//     裸 `AttributeError`（未捕 ⇒ 回溯 + rc=1）；本件按新口径 **fail-closed rc=2 ＋ 具名文案**（口径差）。
//  5. argparse 的**前缀缩写**（`--prof dev` 一类）与「旗标值以 `-` 开头」的歧义档：本件只认全名与 `--k=v`。
//  6. 非 UTF-8 字节序列的替换粒度（py `open(encoding="utf-8")` 的 `UnicodeDecodeError`）未对拍。
//  7. py `json` 默认收 `NaN`/`Infinity`；本件走严格 JSON（该档未对拍）。
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// ── 常量 ─────────────────────────────────────────────────────────────────────

const (
	mfpSelfRel  = pathFreshnessProfilePy
	mfpNeiRel   = pathFreshnessPy
	mfpSelfBase = "check-manifest-freshness-profile.py"
)

// mfpJSONFields —— 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）。
var mfpJSONFields = []string{"profile", "manifest", "release_head", "rc", "hits", "stale"}

// mfpProfileZh —— 分档中文名（与 py `PROFILE_ZH` 逐字同构）。
var mfpProfileZh = map[string]string{"dev": "开发机", "public": "公开制品"}

// mfpHitsID 判据 id（两条 · 逐条分列）。
const (
	mfpJ1 = "J1"
	mfpJ2 = "J2"
)

// mfpHit —— 一条判据的结论。hit: true=命中 · false=成立 · nil=不给结论。
type mfpHit struct {
	id   string
	name string
	hit  *bool
	note string
}

func mfpBool(b bool) *bool { return &b }

// mfpEvalResult —— `evaluate()` 的出口。
type mfpEvalResult struct {
	hits    []mfpHit
	stale   []string
	staleNo bool // true = 真源不在场（py 的 `stale is None`）
}

// ── 入口 ─────────────────────────────────────────────────────────────────────

// checkManifestFreshnessProfileGateMain —— 入口（= py `main()`）。
func checkManifestFreshnessProfileGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：`--json <字段>` 里有不认的名 ⇒ rc=2 ＋ 印可选字段表（先于位置参数解析）。
	if rc := mfpRefuseUnknownJSONFields(stderr, argv); rc != nil {
		return *rc
	}
	// ② 位置参数等价解析（py argparse）。
	opt, rcBad := mfpParseArgs(stderr, argv)
	if rcBad != nil {
		if *rcBad == 0 {
			mfpPrintHelp(stdout)
		}
		return *rcBad
	}

	selfRoot := mfpResolveRoot()
	// `--now` 注入旋钮（有意新增）：缺省 = wall clock，与 py `datetime.now(timezone.utc)` 逐字同构。
	now := time.Now().UTC()
	if opt.now != "" {
		if t, err := time.Parse(time.RFC3339, opt.now); err == nil {
			now = t.UTC()
		}
	}

	neiPath := mfpPyJoin(mfpPyJoin(selfRoot, "scripts"), "gates")
	neiPath = mfpPyJoin(neiPath, "check-manifest-freshness.py")
	// ★ 退役容忍（B轨 · 2026-10-07 · 退件阻塞批次 · 类①）：邻件退役（py 缺席 · 其 Go 半在盘）
	// ⇒ 视同「口径可得」（本 Go 半已把邻件 diagnose()/git_head() 真身重建）⇒ 判据照常出结论
	// （✗ 不得 rc=2 fail-closed）。件在盘 ⇒ 逐字沿用原语义；真消失（无 Go 半）⇒ 仍 fail-closed。
	neiPresent := mfpFileExists(neiPath) || mfpNeighborRetired(selfRoot)

	if opt.listRules {
		return mfpListRules(stdout)
	}
	if opt.selfTest {
		fmt.Fprintln(stdout, "── 清单新鲜度分档（R-23）· 自检（正/负成对 · 合成夹具）──")
		return mfpSelfTest(stdout, selfRoot, neiPresent, now)
	}

	// ③ 不给结论之一：清单件读不到。
	if opt.manifest == "" || !mfpFileExists(opt.manifest) {
		shown := opt.manifest
		if shown == "" {
			shown = "（未给 --manifest）"
		}
		fmt.Fprintf(stdout, "缺件：清单件读不到（%s）⇒ **不给结论**（缺件不许静默放绿）\n", shown)
		fmt.Fprintln(stdout, "⇒ rc=2")
		return 2
	}
	// ④ 不给结论之二：JSON 坏（正文差异见件头未覆盖面③）。
	mf, ok := mfpLoadManifest(opt.manifest)
	if !ok {
		fmt.Fprintf(stdout, "清单件 JSON 解析失败（%s）⇒ **不给结论**\n", mfpLoadErr)
		fmt.Fprintln(stdout, "⇒ rc=2")
		return 2
	}

	repo := opt.repo
	repoSet := opt.repoSet
	if !repoSet {
		repo = selfRoot
	}

	releaseHead := mfpPyStrip(opt.releaseHead)
	var headSrc string
	if releaseHead == "" {
		releaseHead = mfpGitHead(repo, neiPresent)
		shown := "取不到"
		if releaseHead != "" {
			shown = mfpCut(releaseHead, 12)
		}
		headSrc = fmt.Sprintf("缺省取值 = 仓 HEAD（%s）", shown)
	} else {
		headSrc = "显式给出 --release-head"
	}
	// ⑤ 不给结论之三：取不到发布提交 HEAD。
	if releaseHead == "" {
		fmt.Fprintf(stdout, "取不到发布提交 HEAD（--release-head 未给 · 仓 %s 也读不到）⇒ 清单文件：%s\n",
			repo, opt.manifest)
		fmt.Fprintln(stdout, "⇒ rc=2（**不给结论**：不假装绿，也不假装红）")
		return 2
	}

	ev, judgeErr := mfpEvaluate(mf, repo, releaseHead, opt.maxAgeHours, now, neiPresent)
	if judgeErr != nil {
		// C 类真口子（py 裸回溯 · rc=1）⇒ 新口径 fail-closed rc=2（见件头未覆盖面④）。
		fmt.Fprintf(stderr, "✗ 本件不给结论（rc=2）：py 此处裸回溯（%s）⇒ 新口径 fail-closed（见件头未覆盖面④）\n",
			judgeErr.Error())
		return 2
	}
	rc := mfpRcOf(opt.profile, ev)

	fmt.Fprintf(stdout, "── 清单新鲜度分档（R-23）· 档=%s（%s）──\n", opt.profile, mfpProfileZh[opt.profile])
	fmt.Fprintf(stdout, "清单文件：%s\n", opt.manifest)
	fmt.Fprintf(stdout, "发布提交 HEAD：%s（%s）\n", mfpCut(releaseHead, 12), headSrc)
	if opt.profile == "public" {
		fmt.Fprintln(stdout, "判定：**阻断**（rc=1，命中即红）")
	} else {
		fmt.Fprintln(stdout, "判定：**只告警**（rc 不拦，命中不红）")
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── 两条判据（逐条分列 · 不合并）──")
	anyHard := false
	for _, h := range ev.hits {
		var tag string
		switch {
		case h.hit == nil:
			tag = "不给结论"
		case *h.hit:
			if opt.profile == "public" {
				tag = "阻断命中"
			} else {
				tag = "告警命中"
			}
			anyHard = true
		default:
			tag = "成立"
		}
		fmt.Fprintf(stdout, "   [%s] %s —— %s：%s\n", h.id, tag, h.name, h.note)
	}
	if anyHard {
		if opt.profile == "public" {
			fmt.Fprintln(stdout, "   ⇒ 公开制品档：命中**不是 stale 告警**，是**红**（公开面「用 A 打包、声称是 B」不可逆）")
		} else {
			fmt.Fprintln(stdout, "   ⇒ 开发机档：命中**只告警**（分发通道可能落后于分支 ⇒ 放宽窗口而非关掉检查）· rc 不拦")
		}
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── 三条 stale 判据（现调 check-manifest-freshness.py 的 diagnose() · 单一口径 · 语义一字未改）──")
	switch {
	case ev.staleNo:
		fmt.Fprintln(stdout, "   （真源 check-manifest-freshness.py 不在场 ⇒ 照实报「取不到」，不自己编一份）")
	case len(ev.stale) == 0:
		fmt.Fprintln(stdout, "   无（清单新鲜）")
	default:
		for _, w := range ev.stale {
			fmt.Fprintln(stdout, "   - "+w)
		}
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "⇒ 档=%s · 判据 %d 条（命中 %d）· rc=%d\n",
		opt.profile, len(ev.hits), mfpCountHard(ev.hits), rc)

	if opt.json != "" {
		fmt.Fprintln(stdout, mfpBuildJSON(opt.json, opt.profile, opt.manifest, releaseHead, rc, ev))
	}
	return rc
}

// ── 判据（逐条一个 id · 逐条一句人话 · 不许合并）───────────────────────────────

// mfpPair —— py `_pair()`：截断有鉴别力 ⇒ 截断；两枚不同串共享前 12 位 ⇒ 给全串。
func mfpPair(x, y string) (string, string) {
	sx, sy := mfpCut(x, 12), mfpCut(y, 12)
	if sx == sy && x != y {
		return x, y
	}
	return sx, sy
}

// mfpJudge —— py `judge()`：返回两条判据（J1/J2 独立成行）。
func mfpJudge(mf any, releaseHead string) ([]mfpHit, error) {
	out := []mfpHit{}
	var src string
	var dirty any
	if m, isDict := mf.(map[string]any); isDict {
		v, err := mfpPyOrEmptyStr(m["source_sha"])
		if err != nil {
			return nil, err
		}
		src = mfpPyStrip(v)
		dirty = m["dirty"]
	}
	// J1：manifest.source_sha == 发布提交 HEAD
	switch {
	case src == "":
		out = append(out, mfpHit{mfpJ1, "source_sha 等于发布提交 HEAD", mfpBool(true),
			"manifest 里 **没有** source_sha（字段缺失 ⇒ 不成立 ⇒ 宁拒不可猜）"})
	case releaseHead == "":
		out = append(out, mfpHit{mfpJ1, "source_sha 等于发布提交 HEAD", nil,
			"取不到发布提交 HEAD ⇒ 这一条**不给结论**"})
	case src == releaseHead:
		a, b := mfpPair(src, releaseHead)
		out = append(out, mfpHit{mfpJ1, "source_sha 等于发布提交 HEAD", mfpBool(false),
			fmt.Sprintf("source_sha=%s == 发布提交 HEAD=%s", a, b)})
	case strings.HasPrefix(releaseHead, src) || strings.HasPrefix(src, releaseHead):
		a, b := mfpPair(src, releaseHead)
		out = append(out, mfpHit{mfpJ1, "source_sha 等于发布提交 HEAD", mfpBool(true),
			fmt.Sprintf("source_sha=%s ≠ 发布提交 HEAD=%s —— 只是**前缀弱匹配**（前 %d 位同），不是相等"+
				"（明文判据是逐字全串 ==；短串无鉴别力 ⇒ 不当「相等」判）", a, b, mfpMinRunes(src, releaseHead))})
	default:
		a, b := mfpPair(src, releaseHead)
		out = append(out, mfpHit{mfpJ1, "source_sha 等于发布提交 HEAD", mfpBool(true),
			fmt.Sprintf("source_sha=%s ≠ 发布提交 HEAD=%s（**用 A 打包、声称是 B**）", a, b)})
	}
	// J2：dirty == false
	switch {
	case dirty == nil:
		out = append(out, mfpHit{mfpJ2, "dirty == false", mfpBool(true),
			"manifest 里 **没有** dirty（字段缺失 ⇒ 不成立 ⇒ 宁拒不可猜）"})
	case dirty == false:
		out = append(out, mfpHit{mfpJ2, "dirty == false", mfpBool(false),
			"dirty=false（出清单时工作树干净）"})
	default:
		out = append(out, mfpHit{mfpJ2, "dirty == false", mfpBool(true),
			fmt.Sprintf("dirty=%s（出清单时工作树**有未提交改动** ⇒ 这批资产无法从提交复现）", mfpPyRepr(dirty))})
	}
	return out, nil
}

// mfpRcOf —— py `rc_of()`：分档只决定「拦不拦」。
func mfpRcOf(profile string, ev mfpEvalResult) int {
	hard := mfpCountHard(ev.hits)
	unknown := 0
	for _, h := range ev.hits {
		if h.hit == nil {
			unknown++
		}
	}
	if profile == "public" && hard > 0 {
		return 1
	}
	if unknown > 0 {
		return 2
	}
	return 0
}

func mfpCountHard(hits []mfpHit) int {
	n := 0
	for _, h := range hits {
		if h.hit != nil && *h.hit {
			n++
		}
	}
	return n
}

// mfpEvaluate —— py `evaluate()`：两条判据 + 现调真源的 stale。
func mfpEvaluate(mf any, repo, releaseHead string, maxAge float64, now time.Time, neiPresent bool) (mfpEvalResult, error) {
	hits, err := mfpJudge(mf, releaseHead)
	if err != nil {
		return mfpEvalResult{}, err
	}
	res := mfpEvalResult{hits: hits}
	// stale_warnings()
	if !neiPresent {
		res.staleNo = true
		return res, nil
	}
	w, derr := mfpDiagnose(mf, repo, maxAge, now)
	if derr != nil {
		res.stale = []string{fmt.Sprintf("（现调 check-manifest-freshness.py 的 diagnose() 抛异常：%s）", derr.Error())}
		return res, nil
	}
	res.stale = w
	return res, nil
}

// ── 邻件 `check-manifest-freshness.py` 的真身重建（只搬，不改口径）─────────────

// mfpUnsupportedMsg —— 邻件 `Unsupported` 的两条具名文案（与邻件源码逐字同构）。
func mfpUnsupportedMsgStart(exc string) string {
	return fmt.Sprintf("判不了：仓根不是 git 仓 / git 不可用 ⇒ 取不到分支头（%s）", exc)
}

func mfpUnsupportedMsgRC(args []string, rc int) string {
	return fmt.Sprintf("判不了：仓根不是 git 仓 / git 不可用 ⇒ `git %s` 非零退出（rc=%d）", strings.Join(args, " "), rc)
}

// mfpGitStrict —— 邻件 `_git_strict()`：取得到 ⇒ (值, true)；取不到 ⇒ (_, false)（判不了 ⇒ 不给结论）。
func mfpGitStrict(repo string, args []string) (string, bool) {
	full := append([]string{"-c", "core.quotepath=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return mfpUnsupportedMsgRC(args, ee.ExitCode()), false
		}
		return mfpUnsupportedMsgStart(err.Error()), false
	}
	return mfpPyStrip(out.String()), true
}

// mfpGitTolerant —— 邻件 `_git()`：起不来或非零退出 ⇒ ""（**吞退码**）。
func mfpGitTolerant(repo string, args []string) string {
	full := append([]string{"-c", "core.quotepath=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return ""
	}
	return mfpPyStrip(out.String())
}

// mfpGitHead —— py `git_head()`：真源不在场或仓不是目录 ⇒ ""。
func mfpGitHead(repo string, neiPresent bool) string {
	if !neiPresent || !mfpIsDir(repo) {
		return ""
	}
	return mfpGitTolerant(repo, []string{"rev-parse", "HEAD"})
}

// mfpDiagnose —— 邻件 `diagnose()`：返回 (告警列表, 异常)。异常非空 ⇒ 调用方落「抛异常」文案。
func mfpDiagnose(mf any, repo string, maxAgeHours float64, now time.Time) ([]string, error) {
	warns := []string{}
	head := ""
	if repo != "" {
		h, ok := mfpGitStrict(repo, []string{"rev-parse", "HEAD"})
		if !ok {
			return nil, fmt.Errorf("%s", h)
		}
		head = h
	}
	m, isDict := mf.(map[string]any)
	if !isDict {
		return nil, fmt.Errorf("'%s' object has no attribute 'get'", mfpPyTypeName(mf))
	}
	src, err := mfpPyOrEmptyStr(m["source_sha"])
	if err != nil {
		return nil, err
	}
	src = mfpPyStrip(src)
	if src != "" && head != "" && src != head {
		warns = append(warns, fmt.Sprintf("清单出自另一棵树：source_sha=%s ≠ 当前分支头 %s",
			mfpCut(src, 12), mfpCut(head, 12)))
	}
	if b, ok := m["dirty"].(bool); ok && b {
		warns = append(warns, "清单生成时工作树**有未提交改动**（dirty=true）⇒ 该批资产无法从提交复现")
	}
	commitClaim, err := mfpPyOrEmptyStr(m["commit"])
	if err != nil {
		return nil, err
	}
	commitClaim = mfpPyStrip(commitClaim)
	if commitClaim != "" && src != "" && commitClaim != src {
		if strings.HasPrefix(src, commitClaim) || strings.HasPrefix(commitClaim, src) {
			a, b := mfpPair(commitClaim, src)
			warns = append(warns, fmt.Sprintf("声明身份与真实来源不一致：commit=%s 而 source_sha=%s —— 只是**前缀弱匹配**"+
				"（前 %d 位同），不是相等（明文判据是逐字全串 ==；短串无鉴别力 ⇒ 不当「一致」判）",
				a, b, mfpMinRunes(commitClaim, src)))
		} else {
			a, b := mfpPair(commitClaim, src)
			warns = append(warns, fmt.Sprintf("声明身份与真实来源不一致：commit=%s 而 source_sha=%s", a, b))
		}
	}
	gen, err := mfpPyOrEmptyStr(m["generated_at"])
	if err != nil {
		return nil, err
	}
	gen = mfpPyStrip(gen)
	if gen != "" {
		t, perr := time.Parse("2006-01-02T15:04:05Z", gen)
		if perr != nil {
			warns = append(warns, fmt.Sprintf("generated_at 无法解析（如实报，不猜）：%s", mfpPyReprStr(gen)))
		} else {
			ageH := now.Sub(t).Hours()
			if ageH > maxAgeHours {
				warns = append(warns, fmt.Sprintf("清单过旧：generated_at=%s 距今 %.1fh > 上限 %sh",
					gen, ageH, mfpPyFloatG(maxAgeHours)))
			}
		}
	}
	return warns, nil
}

// ── 自检（正/负成对 · 合成夹具 · 不碰真目标）────────────────────────────────────

// mfpFreshManifest —— py `FRESH`（合成清单）。
func mfpFreshManifest() map[string]any {
	return map[string]any{
		"commit":       "abc1234567890",
		"source_sha":   "abc1234567890",
		"dirty":        false,
		"generated_at": "2026-09-23T00:00:00Z",
	}
}

func mfpSelfTest(stdout io.Writer, selfRoot string, neiPresent bool, now time.Time) int {
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
	head := "abc1234567890"
	evalNoGit := func(mf map[string]any, rh string) mfpEvalResult {
		ev, _ := mfpEvaluate(mf, "", rh, 720, now, neiPresent)
		return ev
	}
	hitsFlags := func(ev mfpEvalResult) string {
		parts := make([]string, len(ev.hits))
		for i, h := range ev.hits {
			parts[i] = mfpPyReprBool(h.hit)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}

	// ① 干净且新鲜 ⇒ 两档都无命中（rc=0）
	ev := evalNoGit(mfpFreshManifest(), head)
	check("正控 · 干净清单 · J1/J2 全成立", hitsFlags(ev), "[False, False]")
	check("正控 · 干净清单 · dev 档 rc", strconv.Itoa(mfpRcOf("dev", ev)), "0")
	check("正控 · 干净清单 · public 档 rc", strconv.Itoa(mfpRcOf("public", ev)), "0")

	// ② 公开制品档：source_sha 不等 ⇒ **必须红**
	ev = evalNoGit(mfpWith(mfpFreshManifest(), "source_sha", "ffffffffffff"), head)
	check("负控 · 公开制品档 · source_sha 不等 ⇒ rc", strconv.Itoa(mfpRcOf("public", ev)), "1")
	check("负控 · 公开制品档 · 同一输入 · dev 档 rc（只告警 ⇒ 不拦）", strconv.Itoa(mfpRcOf("dev", ev)), "0")
	check("负控 · 两条判据分列（J1 命中 · J2 仍成立）", hitsFlags(ev), "[True, False]")

	// ③ 公开制品档：dirty=true ⇒ 必须红；dev ⇒ 仍 0
	ev = evalNoGit(mfpWith(mfpFreshManifest(), "dirty", true), head)
	check("负控 · 公开制品档 · dirty=true ⇒ rc", strconv.Itoa(mfpRcOf("public", ev)), "1")
	check("负控 · 同一输入 · dev 档 rc", strconv.Itoa(mfpRcOf("dev", ev)), "0")

	// ④ 两条同时命中 ⇒ 仍是**两条**
	ev = evalNoGit(mfpWith(mfpWith(mfpFreshManifest(), "source_sha", "ffffffffffff"), "dirty", true), head)
	var ids []string
	for _, h := range ev.hits {
		if h.hit != nil && *h.hit {
			ids = append(ids, h.id)
		}
	}
	check("负控 · 两条同时命中 ⇒ 报出两条 id", mfpPyStrList(ids), "['J1', 'J2']")

	// ⑤ 字段缺失 ⇒ 公开制品档红（宁拒不可猜）· dev 档不拦
	ev = evalNoGit(map[string]any{"commit": "abc1234567890"}, head)
	check("负控 · 字段缺失 ⇒ 公开制品档 rc", strconv.Itoa(mfpRcOf("public", ev)), "1")
	check("负控 · 字段缺失 ⇒ dev 档 rc", strconv.Itoa(mfpRcOf("dev", ev)), "0")

	// ⑥ 取不到发布提交 HEAD ⇒ 那一条**不给结论**（rc=2）
	ev = evalNoGit(mfpFreshManifest(), "")
	check("负控 · 取不到 HEAD ⇒ 不给结论（rc=2）", strconv.Itoa(mfpRcOf("dev", ev)), "2")

	// ⑦ 三条 stale 判据**现调真源**（不是自己抄一份）
	if neiPresent {
		w, err := mfpDiagnose(mfpWith(mfpFreshManifest(), "generated_at", "2026-09-23T00:00:00Z"), "", 1, now)
		isList := err == nil && w != nil
		check("单一口径 · 现调真源 diagnose() 可调用", mfpPyBool(isList), "True")
		check("单一口径 · 真源在场（不是本件自己抄一份判据）", mfpPyBool(neiPresent), "True")
	} else {
		bad = append(bad, "真源 check-manifest-freshness.py 不在场")
		log = append(log, "   ✗ 真源 check-manifest-freshness.py 不在场 ⇒ 单一口径不成立")
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

func mfpWith(m map[string]any, k string, v any) map[string]any {
	out := map[string]any{}
	for kk, vv := range m {
		out[kk] = vv
	}
	out[k] = v
	return out
}

// mfpListRules —— py `list_rules()`。
func mfpListRules(stdout io.Writer) int {
	fmt.Fprintln(stdout, "清单新鲜度分档（R-23）· 真源：任务清单-缺口收口-20260923.md:72 · 调研-缺口-门禁证据-20260923.md:136/167")
	fmt.Fprintln(stdout, "判据（两条 · 逐条分列 · 合并成一句即失去一条）：")
	fmt.Fprintln(stdout, "  J1 manifest.source_sha == 发布提交 HEAD（--release-head；缺省 = 仓 HEAD）")
	fmt.Fprintln(stdout, "  J2 manifest.dirty == false")
	fmt.Fprintln(stdout, "分档（只决定「拦不拦」，不决定「判不判」）：")
	fmt.Fprintln(stdout, "  dev    开发机  ⇒ 命中写成**告警**，rc 不拦（恒 0）")
	fmt.Fprintln(stdout, "  public 公开制品 ⇒ 命中写成**阻断**，rc=1")
	fmt.Fprintln(stdout, "三条 stale 判据：现调 scripts/gates/check-manifest-freshness.py 的 diagnose()（单一口径，不抄一份）")
	fmt.Fprintln(stdout, "退码：0 无阻断 · 1 公开制品档有阻断命中 · 2 不给结论（缺件 / JSON 坏 / 取不到 HEAD / 自检不过）")
	return 0
}

// ── Q-176：`--json <字段>` 的字段名校验 ──────────────────────────────────────

func mfpRefuseUnknownJSONFields(stderr io.Writer, argv []string) *int {
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
		if s := mfpPyStrip(f); s != "" {
			got = append(got, s)
		}
	}
	var bad []string
	for _, f := range got {
		known := false
		for _, k := range mfpJSONFields {
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
	fldMarks := make([]string, len(mfpJSONFields))
	for i, f := range mfpJSONFields {
		fldMarks[i] = "`" + f + "`"
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", strings.Join(badMarks, " · "))
	fmt.Fprintf(stderr, "   可选字段表：%s\n", strings.Join(fldMarks, " · "))
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开")
	rc := 2
	return &rc
}

// ── JSON 面（逐字拼装 · 对齐 py `json.dumps(..., ensure_ascii=False)`）────────

func mfpBuildJSON(jsonArg, profile, manifest, releaseHead string, rc int, ev mfpEvalResult) string {
	obj := map[string]string{
		"profile":      mfpJSONStr(profile),
		"manifest":     mfpJSONStr(manifest),
		"release_head": mfpJSONStr(releaseHead),
		"rc":           strconv.Itoa(rc),
		"hits":         mfpJSONHits(ev.hits),
		"stale":        mfpJSONStale(ev),
	}
	var want []string
	for _, f := range strings.Split(jsonArg, ",") {
		if s := mfpPyStrip(f); s != "" {
			want = append(want, s)
		}
	}
	if len(want) == 0 {
		want = mfpJSONFields
	}
	seen := map[string]bool{}
	var parts []string
	for _, k := range want {
		if seen[k] {
			continue
		}
		seen[k] = true
		parts = append(parts, mfpJSONStr(k)+": "+obj[k])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func mfpJSONHits(hits []mfpHit) string {
	parts := make([]string, len(hits))
	for i, h := range hits {
		hitLit := "null"
		if h.hit != nil {
			if *h.hit {
				hitLit = "true"
			} else {
				hitLit = "false"
			}
		}
		parts[i] = "{\"id\": " + mfpJSONStr(h.id) + ", \"name\": " + mfpJSONStr(h.name) +
			", \"hit\": " + hitLit + ", \"note\": " + mfpJSONStr(h.note) + "}"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func mfpJSONStale(ev mfpEvalResult) string {
	if ev.staleNo {
		return "null"
	}
	parts := make([]string, len(ev.stale))
	for i, s := range ev.stale {
		parts[i] = mfpJSONStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// mfpJSONStr —— py `json.dumps(s, ensure_ascii=False)` 的字符串面。
func mfpJSONStr(s string) string {
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

// ── 清单件读取 ───────────────────────────────────────────────────────────────

// mfpLoadErr —— 最近一次 `mfpLoadManifest` 的失败正文（Go 侧 error 文案）。
var mfpLoadErr string

// mfpLoadManifest —— py `json.load(open(path, encoding="utf-8"))`：
// 文本模式先折 `\r\n`/裸 `\r` 为 `\n`，再按严格 JSON 解；数字保留字面（判 int/float）。
func mfpLoadManifest(path string) (any, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		mfpLoadErr = mfpOSErrPy(err, path)
		return nil, false
	}
	s := string(b)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if strings.TrimSpace(s) == "" {
		mfpLoadErr = "Expecting value: line 1 column 1 (char 0)"
		return nil, false
	}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		mfpLoadErr = err.Error()
		return nil, false
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		mfpLoadErr = "Extra data"
		return nil, false
	}
	return v, true
}

// mfpOSErrPy —— 对齐 py `OSError.__str__` 的形态 `[Errno N] <strerror>: '<path>'`（本件够用子集）。
func mfpOSErrPy(err error, path string) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		if errno, ok := pe.Err.(syscall.Errno); ok {
			msg, known := mfpStrerror[int(errno)]
			if !known {
				msg = pe.Err.Error()
			}
			return fmt.Sprintf("[Errno %d] %s: '%s'", int(errno), msg, path)
		}
	}
	return err.Error()
}

// mfpStrerror —— C `strerror` 的常用档（py 用大写句式；Go `errno.Error()` 是小写，故另立一表）。
var mfpStrerror = map[int]string{
	1:  "Operation not permitted",
	2:  "No such file or directory",
	13: "Permission denied",
	20: "Not a directory",
	21: "Is a directory",
	40: "Too many levels of symbolic links",
}

// ── argparse 面（80 列冻结形态）──────────────────────────────────────────────

type mfpOpts struct {
	profile     string
	manifest    string
	releaseHead string
	repo        string
	repoSet     bool
	maxAgeHours float64
	json        string
	selfTest    bool
	listRules   bool
	now         string
}

var mfpUsageLines = []string{
	"usage: check-manifest-freshness-profile.py [-h] [--profile {dev,public}]",
	"                                           [--manifest MANIFEST]",
	"                                           [--release-head RELEASE_HEAD]",
	"                                           [--repo REPO]",
	"                                           [--max-age-hours MAX_AGE_HOURS]",
	"                                           [--json JSON] [--self-test]",
	"                                           [--list-rules]",
}

func mfpUsageText() string { return strings.Join(mfpUsageLines, "\n") + "\n" }

func mfpArgError(stderr io.Writer, msg string) {
	fmt.Fprint(stderr, mfpUsageText())
	fmt.Fprintf(stderr, "%s: error: %s\n", mfpSelfBase, msg)
}

// mfpParseArgs —— 等价 argparse：返回 (opts, rc)。rc==0 表示走 help。
// rc != nil 且 rc != 0 ⇒ 已印错误，调用方直接 return。
func mfpParseArgs(stderr io.Writer, argv []string) (mfpOpts, *int) {
	o := mfpOpts{profile: "dev", maxAgeHours: 720.0}
	need := func(i int, name string) (string, bool) {
		if i+1 >= len(argv) {
			mfpArgError(stderr, fmt.Sprintf("argument %s: expected one argument", name))
			return "", false
		}
		v := argv[i+1]
		if strings.HasPrefix(v, "--") {
			mfpArgError(stderr, fmt.Sprintf("argument %s: expected one argument", name))
			return "", false
		}
		return v, true
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			z := 0
			return o, &z
		case a == "--self-test":
			o.selfTest = true
		case a == "--list-rules":
			o.listRules = true
		case a == "--profile" || strings.HasPrefix(a, "--profile="):
			v, ok := mfpVal(argv, &i, a, "--profile", "profile", stderr)
			if !ok {
				z := 2
				return o, &z
			}
			if v != "dev" && v != "public" {
				mfpArgError(stderr, fmt.Sprintf("argument --profile: invalid choice: '%s' (choose from 'dev', 'public')", v))
				z := 2
				return o, &z
			}
			o.profile = v
		case a == "--manifest" || strings.HasPrefix(a, "--manifest="):
			v, ok := mfpVal(argv, &i, a, "--manifest", "manifest", stderr)
			if !ok {
				z := 2
				return o, &z
			}
			o.manifest = v
		case a == "--release-head" || strings.HasPrefix(a, "--release-head="):
			v, ok := mfpVal(argv, &i, a, "--release-head", "release-head", stderr)
			if !ok {
				z := 2
				return o, &z
			}
			o.releaseHead = v
		case a == "--repo" || strings.HasPrefix(a, "--repo="):
			v, ok := mfpVal(argv, &i, a, "--repo", "repo", stderr)
			if !ok {
				z := 2
				return o, &z
			}
			o.repo = v
			o.repoSet = true
		case a == "--json" || strings.HasPrefix(a, "--json="):
			v, ok := mfpVal(argv, &i, a, "--json", "json", stderr)
			if !ok {
				z := 2
				return o, &z
			}
			o.json = v
		case a == "--now" || strings.HasPrefix(a, "--now="):
			v, ok := mfpVal(argv, &i, a, "--now", "now", stderr)
			if !ok {
				z := 2
				return o, &z
			}
			o.now = v
		case a == "--max-age-hours" || strings.HasPrefix(a, "--max-age-hours="):
			v, ok := mfpVal(argv, &i, a, "--max-age-hours", "max-age-hours", stderr)
			if !ok {
				z := 2
				return o, &z
			}
			f, ferr := strconv.ParseFloat(v, 64)
			if ferr != nil {
				mfpArgError(stderr, fmt.Sprintf("argument --max-age-hours: invalid float value: '%s'", v))
				z := 2
				return o, &z
			}
			o.maxAgeHours = f
		case strings.HasPrefix(a, "-") && a != "-":
			mfpArgError(stderr, fmt.Sprintf("unrecognized arguments: %s", a))
			z := 2
			return o, &z
		default:
			mfpArgError(stderr, fmt.Sprintf("unrecognized arguments: %s", a))
			z := 2
			return o, &z
		}
		_ = need
	}
	return o, nil
}

// mfpVal —— 取旗标值：`--k v` 或 `--k=v`。
func mfpVal(argv []string, i *int, a, name, errName string, stderr io.Writer) (string, bool) {
	if strings.HasPrefix(a, name+"=") {
		return strings.TrimPrefix(a, name+"="), true
	}
	if *i+1 >= len(argv) {
		mfpArgError(stderr, fmt.Sprintf("argument %s: expected one argument", name))
		return "", false
	}
	v := argv[*i+1]
	if strings.HasPrefix(v, "--") {
		mfpArgError(stderr, fmt.Sprintf("argument %s: expected one argument", name))
		return "", false
	}
	*i++
	return v, true
}

func mfpPrintHelp(stdout io.Writer) {
	fmt.Fprint(stdout, mfpUsageText())
	fmt.Fprint(stdout, "\noptional arguments:\n")
	fmt.Fprint(stdout, "  -h, --help            show this help message and exit\n")
	fmt.Fprint(stdout, "  --profile {dev,public}\n")
	fmt.Fprint(stdout, "  --manifest MANIFEST\n")
	fmt.Fprint(stdout, "  --release-head RELEASE_HEAD\n")
	fmt.Fprint(stdout, "                        发布提交 sha（公开制品档应显式给；缺省 = 仓 HEAD）\n")
	fmt.Fprint(stdout, "  --repo REPO\n")
	fmt.Fprint(stdout, "  --max-age-hours MAX_AGE_HOURS\n")
	fmt.Fprint(stdout, "  --json JSON           机器面字段（逗号分隔：hits,stale,rc,profile）\n")
	fmt.Fprint(stdout, "  --self-test\n")
	fmt.Fprint(stdout, "  --list-rules\n")
}

// ── py 语义小工具 ────────────────────────────────────────────────────────────

// mfpResolveRoot —— 仓根：由 cwd 上溯找本件同族 py 的位置（见件头标准声明 (a)）。
func mfpResolveRoot() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	cur := d
	for i := 0; i < 32; i++ {
		if mfpFileExists(mfpPyJoin(cur, mfpSelfRel)) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return d
}

// mfpPyJoin —— CPython `posixpath.join` 的逐段规则（三类语义差：`..` 留串 · 绝对段丢前段 · 空段形态）。
// 本件调用点两段皆相对、无 `..`、无空段 ⇒ 恒同构；仍自写以对齐口径（✗ 不用 filepath.Join）。
func mfpPyJoin(a, b string) string {
	if strings.HasPrefix(b, "/") {
		return b
	}
	if a == "" {
		return b
	}
	if b == "" {
		return a + "/"
	}
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

func mfpFileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// mfpNeighborRetired —— 邻件 `check-manifest-freshness.py` 是否**已退役**（B轨 · 2026-10-07 · 类①）：
// 邻件 py 缺席 **且** 其 Go 半（`core/cmd/zerg/check_manifest_freshness.go`）在盘。
// 退役态 ⇒ 本件视同「单一口径可得」（本 Go 半已把邻件 diagnose()/git_head() 真身重建）⇒ 判据照常
// 出结论（✗ 不 fail-closed）；缺件且无 Go 半（真消失）⇒ 未退役（照旧 fail-closed，⑫ 格不变）。
// 与 check_public_ci_green 的 cicgProfileRetired 同口径（同一「退役」判据，✗ 不新造第二套）。
func mfpNeighborRetired(selfRoot string) bool {
	py := mfpPyJoin(mfpPyJoin(mfpPyJoin(selfRoot, "scripts"), "gates"), "check-manifest-freshness.py")
	goHalf := mfpPyJoin(mfpPyJoin(mfpPyJoin(selfRoot, "core"), "cmd"), mfpPyJoin("zerg", "check_manifest_freshness.go"))
	return !mfpFileExists(py) && mfpFileExists(goHalf)
}

func mfpIsDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// mfpPyStrip —— py `str.strip()`（Unicode 语义 · 含 `\x1c`-`\x1f`/`\x85`/`\xa0`）。
func mfpPyStrip(s string) string {
	isWS := func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\v', '\f', '\r', '\x1c', '\x1d', '\x1e', '\x1f', 0x85, 0xa0:
			return true
		}
		return false
	}
	return strings.TrimFunc(s, isWS)
}

// mfpCut —— py `s[:n]`（按**码点**切）。
func mfpCut(s string, n int) string {
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

func mfpMinRunes(a, b string) int {
	la, lb := utf8.RuneCountInString(a), utf8.RuneCountInString(b)
	if la < lb {
		return la
	}
	return lb
}

// mfpPyOrEmptyStr —— py `(v or "")`：v 是假值 ⇒ ""；v 是串 ⇒ v；否则抛 `AttributeError`（同 py）。
func mfpPyOrEmptyStr(v any) (string, error) {
	if v == nil {
		return "", nil
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	if !mfpPyTruthy(v) {
		return "", nil
	}
	return "", fmt.Errorf("'%s' object has no attribute 'strip'", mfpPyTypeName(v))
}

// mfpPyTypeName —— py 侧类型名（AttributeError 文案用）。
func mfpPyTypeName(v any) string {
	switch t := v.(type) {
	case nil:
		return "NoneType"
	case bool:
		return "bool"
	case string:
		return "str"
	case json.Number:
		s := t.String()
		if strings.ContainsAny(s, ".eE") {
			return "float"
		}
		return "int"
	case []any:
		return "list"
	case map[string]any:
		return "dict"
	}
	return "object"
}

// mfpPyTruthy —— py 真值判定（JSON 值域）。
func mfpPyTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return true
		}
		return f != 0
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	}
	return true
}

// mfpPyReprBool —— py `str(bool)` 面（hit 为 None ⇒ "None"）。
func mfpPyReprBool(b *bool) string {
	if b == nil {
		return "None"
	}
	if *b {
		return "True"
	}
	return "False"
}

func mfpPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// mfpPyReprStr —— py `repr(str)` 的本件够用子集。
func mfpPyReprStr(s string) string {
	hasSingle := strings.Contains(s, "'")
	hasDouble := strings.Contains(s, "\"")
	esc := mfpPyReprEscape(s)
	if hasSingle && !hasDouble {
		return "\"" + esc + "\""
	}
	esc = strings.ReplaceAll(esc, "'", "\\'")
	return "'" + esc + "'"
}

func mfpPyReprEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 || r == 0x7f {
				b.WriteString(fmt.Sprintf("\\x%02x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// mfpPyRepr —— py `repr(v)` 的 JSON 值域子集（dirty %r 用）。
func mfpPyRepr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case string:
		return mfpPyReprStr(t)
	case json.Number:
		return mfpPyReprNumber(t)
	case []any:
		parts := make([]string, len(t))
		for i, x := range t {
			parts[i] = mfpPyRepr(x)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		parts := make([]string, 0, len(t))
		for k, x := range t {
			parts = append(parts, mfpPyReprStr(k)+": "+mfpPyRepr(x))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return fmt.Sprintf("%v", v)
}

// mfpPyReprNumber —— py `repr(int|float)`：整数原样；浮点用最短往返（本件够用子集）。
func mfpPyReprNumber(n json.Number) string {
	s := n.String()
	if !strings.ContainsAny(s, ".eE") {
		return s
	}
	f, err := n.Float64()
	if err != nil {
		return s
	}
	if f == float64(int64(f)) && f >= -1e15 && f <= 1e15 {
		return strconv.FormatInt(int64(f), 10) + ".0"
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// mfpPyStrList —— py `str(list[str])`。
func mfpPyStrList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = mfpPyReprStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// mfpPyFloatG —— py `f"{v:g}"` 的本件够用子集。
func mfpPyFloatG(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}
