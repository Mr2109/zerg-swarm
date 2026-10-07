// check_public_ci_green.go —— 判据件
// `scripts/gates/check-public-ci-green.py`（421 行 · sha16 0f8f2824013b1d72）的 **Go 真搬**（v2.5.14 · M2 批 · 甲案口径 · 真·待搬清单第十五枚）。
//
// 本件做什么（= py `main()`）：
//
//	公开 CI 归绿**分档**（T10）。四组判据各自成行、不合并：
//	  K1 分档两态 —— dev 档只告警（rc 不拦）· public 档必须阻断（rc=1）。
//	       **单一口径**：现调邻件 `check-manifest-freshness-profile.py` 的两个 `--profile`（本件不另写一份新鲜度判据）。
//	  K2 三条缺项**照原样登记**（逐字常量必须原样出现在输出里 · 报「仍缺 3 条」· 本件不修）。
//	  K3 `R-25` 按兵不动 —— CI 件里 `id-token`/`attestations` 权限 0 处，出现即红。
//	  K4 **附加现读**（不计入三档 · 只记账）：`uses:` 是否全钉 40 位 hex · 顶层 `contents: write` · 件内 job 名统计。
//	退码 0/1/2，与仓内其余门件同口径。
//
// 外部真源（★ 只现读、不重写）：
//
//	① `scripts/gates/check-manifest-freshness-profile.py` —— K1 的**唯一口径真源**，py 由 `subprocess.run([sys.executable, …])` fork。
//	② `publish/ci/ci.yml` · `publish/ci/release-agent.yml` · `.github/workflows/*.yml|*.yaml` —— CI 件的**读面**（只读，零写面）。
//	★ 本件**不联网、不调 gh**：经现读确认，py 全文无 `gh` / `urllib` / `requests` / `curl` / socket 面；
//	  唯一的外部进程面 = 上面那枚**仓内** py 邻件（本地 fork，无网）。
//	★ 本件**零写面**：py 只在 `tempfile.mkdtemp()` 造的仓外临时目录里写合成清单（样张 A / 自检），
//	  仓内 CI 件与仓内任何登记面**一字未动**。
//
// 跨件复用面：
//
//	K1 的 rc 读数**全部**来自邻件；本件只判「档位行为」（dev rc 不许是 1 · public 有命中时 rc 必须是 1）。
//	邻件不在场 ⇒ `run_profile()` 返 `None`（Go 侧 = `(0, false)`），不是 0 也不是 1 —— 而 py 的 `None != 0` 为真
//	⇒ 照样落一条命中（`rc=None`）。★ 这一点本件**逐字照搬**（不当成「取不到就放过」）。
//
// 入口签名（与 py `main()` 逐字对齐）：
//
//	func cicgGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 分档行为对+三条照原样+R-25 未引 / 1 有命中 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出  —— stdout / stderr 与 py 逐字节对齐（未覆盖面见下）
//
// ── 标准声明 (a)：默认仓根定位 ────────────────────────────────────────────────
// py：`HERE = dirname(abspath(__file__))` → `REPO_ROOT = dirname(dirname(HERE))` —— 由**脚本文件自身位置**上溯。
// Go **无 `__file__` 语境** ⇒ 由 **cwd 上溯**找 `<根>/scripts/gates/check-public-ci-green.py`（同族 py 所在位置即这个 `HERE` 的判据）。
// ⇒ **「脚本位置 ≠ cwd」时推导面不同**；对拍一律把两档的 cwd 钉在同一个镜像根。
// ★ 邻件落点同理由「上溯根」推出（py 从 `HERE` join 它）。
//
// ── 标准声明 (b)：解释器口径差（`sys.executable` 面无对应物）────────────────────
// py 用 `sys.executable`（**跑本件的解释器绝对路径**）fork 邻件；Go 无此串 ⇒ 缺省 `python3`，
// env `ZERG_CICG_PYTHON` 可显式指定（对拍时两档同钉 `/usr/bin/python3`）。⇒ **声明口径差**，不是新增行为面。
//
// ── 标准声明 (c)：自指检查 ────────────────────────────────────────────────────
// 本件落在 `core/` 域内，故逐条核对三个他门扫描面（按「一旦被跟踪」消解）：
//
//	① check-hardcoded-private-paths（扫 `core/` · 命中词 `<volume-path>`·`<volume-path>`·`~`）：
//	   本件零私有绝对路径字面量 ⇒ 遗留悬空字面量 **0**。
//	② check-error-kinds（扫 `core/cmd/zerg/*.go` · 认 `setErr` 首参字面与闭集 kind 键取值字面）：
//	   本件不打印任何闭集 kind 字面量 ⇒ 该门命中数不变（0 处）。
//	③ check-wall-layer（扫 `core/cmd/zerg/*.go` · 认层级常量形与命令树 `layer` 标记形）：
//	   本件零层级常量形 / 零命令树 `layer` 标记形 ⇒ 遗留 **0**。
//	⇒ 自指=真（本件在被扫域内）；落仓前后他门输出 sha16 逐字相同（本件对它们的贡献 = 0）。
//
// ── 未覆盖面（✗ 不声称逐字对齐）──────────────────────────────────────────────
//  1. **argparse 前缀缩写**（`--mani x` 一类）与「旗标值以 `-` 开头」的歧义档：本件只认全名与 `--k=v`。
//  2. **默认根定位**：py 由 `__file__` 上溯 ⟷ Go 由 cwd 上溯（见标准声明 (a)）；对拍把两档 cwd 钉在同一镜像根。
//  3. **非 UTF-8 字节序列**：py `open(encoding="utf-8", errors="replace")` 把坏序列换成 U+FFFD；
//     本件按裸字节保留 ⇒ 该档未对拍。
//  4. **`\s` 的 Unicode 语义**：py `re` 的 `\s` 是 Unicode 空白（含 `\xa0`/`\x85` 等）；Go RE2 的 `\s` 只认 ASCII 空白。
//     CI 件（纯 ASCII/中文正文）不触该档；坏档未对拍。
//  5. **`str.lower()` / `str.strip()` 的完整 Unicode 折叠**：本件用 `strings.ToLower` + `unicode.IsSpace`（补 `\x1c`-`\x1f`），
//     与 CPython 全量折叠在极少数码点上可能分叉（CI 件不触）。
//  6. **JSON 字符串转义**：py `json.dumps(ensure_ascii=False)` 与 `cicgJSONStr` 在 `\b`/`\f` 以外逐字同；
//     `\b`/`\f`/U+2028/U+2029 档未对拍（仓内读面不含这些码点）。
//  7. **`tempfile` 落点**：py `tempfile.mkdtemp(prefix="zerg-ci-green-")` ⟷ Go `os.MkdirTemp("", …)` —— 两者的目录名随机段不同，
//     但**该路径从不进 stdout**（样张 A 只印 rc），故对拍面不受影响。
//  8. **`subprocess` 的超时/取不到档**：py `timeout=180` 抛 `TimeoutExpired` ⇒ `run_profile` 返 `None`；
//     本件 `exec.CommandContext` + 180s 同口径，但**该档未对拍**（不真造 180s 挂死）。
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ── 常量（与 py 顶层逐字同构）────────────────────────────────────────────────

const (
	cicgSelfRel        = "scripts/gates/check-public-ci-green.py"
	cicgProfileGateRel = "scripts/gates/check-manifest-freshness-profile.py"
	cicgCIDirNew       = ".github/workflows"
	cicgProg           = "check-public-ci-green.py"
)

// cicgGapsSource —— K2 三条缺项的**原样出处**（py `GAPS_SOURCE` 逐字）。
const cicgGapsSource = "Zerg-内部文档/项目文档/v2.5.11/调研-缺口-门禁证据-20260923.md:22"

// cicgThreeGaps —— K2 三条缺项（**原样** · 只登记不修）。
// ★ 逐字抄自 py 源码字面（含反引号）—— 不是从渲染输出抄的。
var cicgThreeGaps = []cicgGap{
	{id: "缺①", text: "① 人面看不见（`G-10`）"},
	{id: "缺②", text: "② `warnings[]` 恒空"},
	{id: "缺③", text: "③ attestation 今天没出（源件逐字「将来若要出 attestation，也在本 job 内单独加」）"},
}

// cicgAttestTokens —— K3 `R-25` 按兵不动：今天**不引**的权限（出现即红）。
var cicgAttestTokens = []string{"id-token", "attestations"}

// cicgCIFilesAlways —— CI 件的**点名面**两件（发布时镜像成公开树 `.github/workflows/*`）。
var cicgCIFilesAlways = []string{"publish/ci/ci.yml", "publish/ci/release-agent.yml"}

// cicgJSONFields —— 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处 · 与 py `JSON_FIELDS` 序逐字同）。
var cicgJSONFields = []string{"rc", "gaps", "gaps_missing", "profile", "synth", "attest_tokens", "uses_total", "uses_float", "top_write", "ci_files_present", "hits"}

var (
	cicgUsesRe   = regexp.MustCompile(`^-?\s*uses:\s*([^\s#]+)`)
	cicgPinnedRe = regexp.MustCompile(`@[0-9a-f]{40}$`)
	cicgNameRe   = regexp.MustCompile(`(?m)^\s*name:\s*(.+)$`)
)

// cicgSynthManifest —— py `json.dumps({"commit":…,"source_sha":"0"*40,"dirty":True,"generated_at":…})` 的逐字节等价体
// （键序 = 插入序 · 分隔符 = `", "`/`": "` · `True` → `true` · 无尾随换行）。
var cicgSynthManifest = `{"commit": "zzzzzzzzzzzz", "source_sha": "` + strings.Repeat("0", 40) +
	`", "dirty": true, "generated_at": "2026-09-23T00:00:00Z"}`

// ── 自检合成夹具（与 py `FIX_*` 逐字节同构）──────────────────────────────────

const cicgFixClean = `name: 合成干净件
on: {workflow_dispatch: {}}
permissions:
  contents: read
jobs:
  a:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262
`

const cicgFixAttest = `name: 合成越线件（引了 attestation）
on: {workflow_dispatch: {}}
permissions:
  contents: read
  id-token: write
  attestations: write
jobs:
  a:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
`

const cicgFixWrite = `name: 合成顶层提权件
on: {workflow_dispatch: {}}
permissions:
  contents: write
jobs:
  a:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262
`

// ── 类型 ─────────────────────────────────────────────────────────────────────

// cicgGap —— 一条缺项（id + 逐字正文）。
type cicgGap struct {
	id   string
	text string
}

// cicgFile —— 一件 CI 件的现读面。
// ★ 不用 nil 兼表两态：`present=false` = 读不到（py `None`）· `present=true` 且 `text==""` = 在场的空件。
type cicgFile struct {
	rel     string
	text    string
	present bool
}

// cicgJobsEntry —— `jobs` 的**有序**表项（py 是 dict ⇒ 插入序即输出序；Go 用切片 + 索引保住这一序）。
type cicgJobsEntry struct {
	rel   string
	names []string
}

// cicgScan —— `scan_ci()` 的机械读数（纯函数 ⇒ 自检与真目标同一份）。
// ★ 四个列表一律 `[]string{}` 起步：空集合要落成 JSON `[]`，**不许**落成 `null`。
type cicgScan struct {
	attestTokens    []string
	usesTotal       int
	usesFloat       []string
	topWrite        []string
	topReadDeclared []string
	jobs            []cicgJobsEntry
	jobIdx          map[string]int
}

// cicgOpts —— 位置参数等价解析的出口（与 py argparse 的五个旗标一一对应）。
type cicgOpts struct {
	manifest  string
	extraCI   []string
	jsonArg   string
	selfTest  bool
	listRules bool
}

// ── 入口（= py `main()`）────────────────────────────────────────────────────

func cicgGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：`--json <字段>` 里有**不认的名** ⇒ rc=2 ＋ 印可选字段表（先于位置参数解析）。
	if rc := cicgRefuseUnknownJSONFields(stderr, argv); rc != nil {
		return *rc
	}
	// ② 位置参数等价解析（py argparse）。
	opt, rcBad := cicgParseArgs(stderr, argv)
	if rcBad != nil {
		if *rcBad == 0 {
			cicgPrintHelp(stdout)
		}
		return *rcBad
	}

	root := cicgResolveRoot()
	if opt.listRules {
		return cicgListRules(stdout, root)
	}
	if opt.selfTest {
		fmt.Fprintln(stdout, "── 公开 CI 归绿 · 分档（T10）· 自检（合成夹具 · 含 must-fail 真红）──")
		return cicgSelfTest(stdout, root)
	}

	files := cicgCIFiles(root, opt.extraCI)
	present := make([]cicgFile, 0, len(files))
	for _, f := range files {
		if f.present {
			present = append(present, f)
		}
	}
	fmt.Fprintln(stdout, "── 公开 CI 归绿 · 分档（T10）· 现读 ──")
	fmt.Fprintf(stdout, "CI 件面（%d 件在场 / %d 件点名）：\n", len(present), len(files))
	for _, f := range files {
		mark := "✓"
		if !f.present {
			mark = "✗（缺件）"
		}
		fmt.Fprintf(stdout, "   %s %s\n", mark, f.rel)
	}
	if len(present) == 0 {
		fmt.Fprintln(stdout, "⇒ 一件 CI 件都读不到 ⇒ **不给结论** rc=2")
		return 2
	}

	scan := cicgScanCI(present)
	hits := []string{}

	// ── K1 分档两态 ──────────────────────────────────────────────────────────
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── K1 分档两态（开发机只告警 / 公开制品阻断）──")
	fmt.Fprintf(stdout, "   单一口径：现调 %s 的两个 --profile（本件不另写一份新鲜度判据）\n",
		cicgRelPath(root, cicgPyJoin(root, cicgProfileGateRel)))

	// 退役态（B轨 · 2026-10-07）：分档真源件（原 py `check-manifest-freshness-profile.py`）退役
	// （缺席 · 其 Go 半在盘）⇒ K1 分档两态**不过问**（✗ 不 fail-closed）；缺件且无 Go 半仍照旧 fail-closed。
	profileRetired := cicgProfileRetired(root)
	if profileRetired {
		fmt.Fprintf(stdout, "   ⚠ 分档真源件已退役（%s 不在盘 · Go 半接管）⇒ K1 分档两态**不过问**（✗ 不 fail-closed）\n",
			cicgRelPath(root, cicgPyJoin(root, cicgProfileGateRel)))
	}

	// 样张 A：合成清单（J1/J2 双命中）—— 分档的**机械证明**（写在仓外临时目录）。
	tmp, err := os.MkdirTemp("", "zerg-ci-green-")
	if err != nil {
		tmp = os.TempDir()
	} else {
		defer os.RemoveAll(tmp) // ★ 自回收（TMPDIR 前后 delta=0）
	}
	synth := cicgPyJoin(tmp, "manifest.json")
	_ = os.WriteFile(synth, []byte(cicgSynthManifest), 0o644)
	relHead := strings.Repeat("1", 40)
	aDev, aDevOK := cicgRunProfile(root, "dev", synth, []string{"--release-head", relHead})
	aPub, aPubOK := cicgRunProfile(root, "public", synth, []string{"--release-head", relHead})
	fmt.Fprintf(stdout, "   样张 A（合成清单 · J1/J2 双命中）：开发机档 rc=%s（期望 0 · 只告警）· 公开制品档 rc=%s（期望 1 · 阻断）\n",
		cicgRcStr(aDev, aDevOK), cicgRcStr(aPub, aPubOK))
	// ★ py 的 `a_dev != 0` 在 `a_dev is None` 时也为真 ⇒ 逐字照搬（取不到照样落一条命中）。
	if !profileRetired && !(aDevOK && aDev == 0) {
		hits = append(hits, fmt.Sprintf("K1 开发机档**拦了**（rc=%s · 判据：开发机档只告警、rc 不拦）⇒ 红", cicgRcStr(aDev, aDevOK)))
	}
	if !profileRetired && !(aPubOK && aPub == 1) {
		hits = append(hits, fmt.Sprintf("K1 公开制品档**没拦**（rc=%s · 判据：`source_sha != HEAD` 或 `dirty != false` ⇒ 红）⇒ 红", cicgRcStr(aPub, aPubOK)))
	}

	// 样张 B：真清单（照实读数 · 命中不是错，是状态）。
	var devRc, pubRc int
	var devRcOK, pubRcOK bool
	real := opt.manifest
	if real == "" {
		cands := cicgGlobReleaseManifests(root)
		if len(cands) > 0 {
			real = cands[len(cands)-1]
		}
	}
	if real != "" && cicgFileExists(real) {
		devRc, devRcOK = cicgRunProfile(root, "dev", real, nil)
		pubRc, pubRcOK = cicgRunProfile(root, "public", real, nil)
		fmt.Fprintf(stdout, "   样张 B（真清单 %s）：\n", cicgRelPath(root, real))
		devWord := "**拦了** ✗"
		if devRcOK && devRc == 0 {
			devWord = "只告警（不拦）✓"
		}
		pubWord := "未阻断（清单该条判据成立）"
		if pubRcOK && pubRc == 1 {
			pubWord = "阻断 ✓"
		}
		fmt.Fprintf(stdout, "     开发机档 rc=%s ⇒ %s\n", cicgRcStr(devRc, devRcOK), devWord)
		fmt.Fprintf(stdout, "     公开制品档 rc=%s ⇒ %s\n", cicgRcStr(pubRc, pubRcOK), pubWord)
		if devRcOK && devRc == 1 {
			hits = append(hits, "K1 真清单在开发机档被拦（rc=1）⇒ 红（判据：只告警）")
		}
		if !profileRetired && !devRcOK {
			fmt.Fprintln(stdout, "     （分档真源跑不出来 ⇒ 交给 rc=2 那一路）")
		}
	} else {
		fmt.Fprintln(stdout, "   样张 B（真清单）：**没有可用的清单件** ⇒ 照实缺席（不当绿、也不当红）")
	}

	// ── K2 三条缺项照原样登记 ────────────────────────────────────────────────
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── K2 三条缺项照原样登记（**仍缺 3 条** · 本件不修 ✗）──")
	for _, g := range cicgThreeGaps {
		fmt.Fprintf(stdout, "   %s %s\n", g.id, g.text)
	}
	fmt.Fprintf(stdout, "   出处（逐字）：%s\n", cicgGapsSource)
	fmt.Fprintf(stdout, "   ⇒ 仍缺 %d 条（照原样登记 ≠ 已修）\n", len(cicgThreeGaps))
	if cicgGapsSayApplied() {
		hits = append(hits, "K2 三条缺项被写成「已修/已引/已出」⇒ 红（照原样登记，不许美化成已修）")
	}

	// ── K3 R-25 按兵不动 ────────────────────────────────────────────────────
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── K3 `R-25` attestation 按兵不动（今天**不引**）──")
	fmt.Fprintf(stdout, "   权限标记 %s：%d 处\n", strings.Join(cicgAttestTokens, " / "), len(scan.attestTokens))
	for _, x := range scan.attestTokens {
		fmt.Fprintln(stdout, "     ✗ "+x)
	}
	if len(scan.attestTokens) > 0 {
		hits = append(hits, fmt.Sprintf("K3 CI 件里出现了 %s 权限（%d 处）⇒ 红：今天是「按兵不动 ✓ 不引」",
			strings.Join(cicgAttestTokens, "/"), len(scan.attestTokens)))
	} else {
		fmt.Fprintln(stdout, "   ✓ 0 处（与判据逐字一致：`R-25` attestation 按兵不动 ✓ 不引）")
	}

	// ── K4 附加现读（不计入三档）────────────────────────────────────────────
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── K4 附加现读（**不计入三档** · 只记账 ✗）──")
	fmt.Fprintf(stdout, "   `uses:` 共 %d 行 · 未钉 40 位 hex：%d 行\n", scan.usesTotal, len(scan.usesFloat))
	for _, x := range cicgHead(scan.usesFloat, 5) {
		fmt.Fprintln(stdout, "     · "+x)
	}
	fmt.Fprintf(stdout, "   顶层 `contents: write`：%d 处\n", len(scan.topWrite))
	for _, x := range cicgHead(scan.topWrite, 5) {
		fmt.Fprintln(stdout, "     · "+x)
	}
	fmt.Fprintln(stdout, "   job 名（**件内统计** · 不声称「必需检查集合」）：")
	for _, je := range scan.jobs {
		fmt.Fprintf(stdout, "     %s：%d 个 —— %s\n", je.rel, len(je.names), strings.Join(cicgHead(je.names, 8), " · "))
	}

	rc := 0
	if len(hits) > 0 {
		rc = 1
	}
	if len(hits) > 0 && !devRcOK && !pubRcOK && !profileRetired && !cicgFileExists(cicgPyJoin(root, cicgProfileGateRel)) {
		rc = 2
	}
	fmt.Fprintln(stdout, "")
	if len(hits) > 0 {
		fmt.Fprintln(stdout, "── 命中 ──")
		for _, h := range hits {
			fmt.Fprintln(stdout, "   ✗ "+h)
		}
	}
	fmt.Fprintf(stdout, "⇒ rc=%d\n", rc)

	if opt.jsonArg != "" {
		presentRels := make([]string, 0, len(present))
		for _, f := range present {
			presentRels = append(presentRels, f.rel)
		}
		obj := []cicgKV{
			{"rc", strconv.Itoa(rc)},
			{"gaps", cicgJSONStrList(cicgGapTexts())},
			{"gaps_missing", strconv.Itoa(len(cicgThreeGaps))},
			{"profile", "{\"dev\": " + cicgRcJSON(devRc, devRcOK) + ", \"public\": " + cicgRcJSON(pubRc, pubRcOK) + "}"},
			{"synth", "{\"dev\": " + cicgRcJSON(aDev, aDevOK) + ", \"public\": " + cicgRcJSON(aPub, aPubOK) + "}"},
			{"attest_tokens", cicgJSONStrList(scan.attestTokens)},
			{"uses_total", strconv.Itoa(scan.usesTotal)},
			{"uses_float", cicgJSONStrList(scan.usesFloat)},
			{"top_write", cicgJSONStrList(scan.topWrite)},
			{"ci_files_present", cicgJSONStrList(presentRels)},
			{"hits", cicgJSONStrList(hits)},
		}
		want := []string{}
		for _, f := range strings.Split(opt.jsonArg, ",") {
			if s := cicgPyStrip(f); s != "" {
				want = append(want, s)
			}
		}
		fmt.Fprintln(stdout, cicgJSONObj(obj, want))
	}
	return rc
}

// cicgKV —— 有序 JSON 键值对（py 的 dict 保序 ⇒ Go 用切片保住这一序）。
type cicgKV struct{ k, v string }

func cicgJSONObj(obj []cicgKV, want []string) string {
	parts := make([]string, 0, len(obj))
	if len(want) == 0 {
		for _, kv := range obj {
			parts = append(parts, cicgJSONStr(kv.k)+": "+kv.v)
		}
	} else {
		for _, w := range want {
			for _, kv := range obj {
				if kv.k == w {
					parts = append(parts, cicgJSONStr(kv.k)+": "+kv.v)
					break
				}
			}
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// ── 读面 ─────────────────────────────────────────────────────────────────────

// cicgRead —— py `read()`：`open(encoding="utf-8", errors="replace")`；读不到（OSError）⇒ ("", false)。
// ★ 不用 nil / 空串兼表两态：显式 bool（成对钉牙见测件）。
func cicgRead(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// cicgCIFiles —— 私有面两件 + `.github/workflows/*.yml|*.yaml`（新增区）+ `extra` 指定的件。**缺件如实缺**。
func cicgCIFiles(root string, extra []string) []cicgFile {
	out := make([]cicgFile, 0, len(cicgCIFilesAlways)+len(extra))
	for _, rel := range cicgCIFilesAlways {
		t, ok := cicgRead(cicgPyJoin(root, rel))
		out = append(out, cicgFile{rel: rel, text: t, present: ok})
	}
	d := cicgPyJoin(root, cicgCIDirNew)
	var all []string
	for _, g := range []string{"*.yml", "*.yaml"} {
		ms, _ := filepath.Glob(cicgPyJoin(d, g))
		all = append(all, ms...)
	}
	// ★ py 是 `sorted(glob(yml) + glob(yaml))` —— **一次**全排序（不是两段各自排）。
	sort.Strings(all)
	for _, p := range all {
		t, ok := cicgRead(p)
		rel, err := filepath.Rel(root, p)
		if err != nil {
			rel = p
		}
		out = append(out, cicgFile{rel: rel, text: t, present: ok})
	}
	for _, e := range extra {
		p := e
		if !filepath.IsAbs(e) {
			p = cicgPyJoin(root, e)
		}
		t, ok := cicgRead(p)
		// 绝对路径**原样**显示（py：不许拼成 ../../.. 那种难认的相对串）。
		out = append(out, cicgFile{rel: e, text: t, present: ok})
	}
	return out
}

func cicgGlobReleaseManifests(root string) []string {
	ms, _ := filepath.Glob(cicgPyJoin(cicgPyJoin(cicgPyJoin(cicgPyJoin(root, "dist"), "*"), "release"), "manifest.json"))
	sort.Strings(ms)
	return ms
}

// ── K3/K4 的机械读数（纯函数 ⇒ 自检与真目标同一份）──────────────────────────

func cicgScanCI(files []cicgFile) cicgScan {
	r := cicgScan{
		attestTokens:    []string{},
		usesFloat:       []string{},
		topWrite:        []string{},
		topReadDeclared: []string{},
		jobs:            []cicgJobsEntry{},
		jobIdx:          map[string]int{},
	}
	for _, f := range files {
		if !f.present {
			continue
		}
		for _, ln := range cicgSplitLines(f.text) {
			s := cicgPyStrip(ln)
			if strings.HasPrefix(s, "#") {
				continue // 抬头注释里也会出现 `uses:` 字样 —— 那是**规矩**，不是用法
			}
			low := strings.ToLower(s)
			for _, tok := range cicgAttestTokens {
				// 只在**权限声明**语境里认（注释里提到「将来若要出 attestation」不算引用）。
				if strings.HasPrefix(low, tok+":") || strings.HasPrefix(low, "- "+tok) ||
					(strings.Contains(low, "permissions") && strings.Contains(low, tok)) {
					r.attestTokens = append(r.attestTokens, fmt.Sprintf("%s: %s", f.rel, cicgFirstRunes(s, 120)))
				}
			}
			if m := cicgUsesRe.FindStringSubmatch(s); m != nil {
				r.usesTotal++
				if !cicgPinnedRe.MatchString(m[1]) {
					r.usesFloat = append(r.usesFloat, fmt.Sprintf("%s: %s", f.rel, cicgFirstRunes(s, 120)))
				}
			}
			if strings.HasPrefix(low, "permissions:") {
				r.topReadDeclared = append(r.topReadDeclared, fmt.Sprintf("%s: %s", f.rel, cicgFirstRunes(s, 60)))
			}
			if strings.HasPrefix(low, "contents: write") && cicgLeadingWS(ln) <= 2 {
				r.topWrite = append(r.topWrite, fmt.Sprintf("%s: %s", f.rel, cicgFirstRunes(s, 60)))
			}
		}
		for _, m := range cicgNameRe.FindAllStringSubmatch(f.text, -1) {
			nm := cicgPyStrip(m[1])
			if strings.HasPrefix(nm, "ci.yml") || strings.HasPrefix(nm, "release-agent") || strings.HasPrefix(nm, "公开") {
				continue
			}
			i, ok := r.jobIdx[f.rel]
			if !ok {
				r.jobs = append(r.jobs, cicgJobsEntry{rel: f.rel, names: []string{}})
				i = len(r.jobs) - 1
				r.jobIdx[f.rel] = i
			}
			r.jobs[i].names = append(r.jobs[i].names, nm)
		}
	}
	return r
}

// ── K1：现调分档真源 ─────────────────────────────────────────────────────────

// cicgPythonBin —— py `sys.executable` 的 Go 侧对应物（声明口径差 · 见件头标准声明 (b)）。
func cicgPythonBin() string {
	if v := cicgPyStrip(os.Getenv("ZERG_CICG_PYTHON")); v != "" {
		return v
	}
	return "python3"
}

// cicgRunProfile —— 现调分档真源 ⇒ `(rc, 取到了?)`。
// ★ 不用「0 兼表取不到」：返 `(int, bool)`（成对钉牙见测件）。
// 真源不在场 / 跑不起来 ⇒ `(0, false)`（= py 的 `None`）。
func cicgRunProfile(root, profile, manifest string, extra []string) (int, bool) {
	gate := cicgPyJoin(root, cicgProfileGateRel)
	if !cicgFileExists(gate) {
		return 0, false
	}
	argv := []string{cicgPythonBin(), gate, "--profile", profile, "--json", "rc,hits,stale"}
	if manifest != "" {
		argv = append(argv, "--manifest", manifest)
	}
	argv = append(argv, extra...)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = root
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), true
		}
		return 0, false // 起不来 / 超时 ⇒ py 的 `None`
	}
	return 0, true
}

// ── 自检（合成夹具 · 含 must-fail 真红）──────────────────────────────────────

func cicgSelfTest(stdout io.Writer, root string) int {
	okCount := 0
	bad := []string{}
	log := []string{}
	check := func(name, got, want string, eq bool) {
		if eq {
			okCount++
			log = append(log, fmt.Sprintf("   ✓ %s ⇒ %s", name, got))
			return
		}
		bad = append(bad, name)
		log = append(log, fmt.Sprintf("   ✗ %s ⇒ 期望 %s，实得 %s", name, want, got))
	}

	clean := cicgScanCI([]cicgFile{{rel: "合成/clean.yml", text: cicgFixClean, present: true}})
	check("正控 · 干净件 · attestation 权限 0 处", cicgPyReprList(clean.attestTokens), "[]", len(clean.attestTokens) == 0)
	check("正控 · 干净件 · uses 全钉 40 位 hex", cicgPyReprTuple2(clean.usesTotal, clean.usesFloat), "(1, [])",
		clean.usesTotal == 1 && len(clean.usesFloat) == 0)
	check("正控 · 干净件 · 顶层无 write", cicgPyReprList(clean.topWrite), "[]", len(clean.topWrite) == 0)

	att := cicgScanCI([]cicgFile{{rel: "合成/attest.yml", text: cicgFixAttest, present: true}})
	check("负控(must-fail) · 引了 id-token/attestations ⇒ K3 命中（R-25 越线）",
		strconv.Itoa(len(att.attestTokens)), "2", len(att.attestTokens) == 2)
	check("负控(must-fail) · 浮动 tag `@v4` ⇒ K4 附加命中", strconv.Itoa(len(att.usesFloat)), "1", len(att.usesFloat) == 1)

	wr := cicgScanCI([]cicgFile{{rel: "合成/write.yml", text: cicgFixWrite, present: true}})
	check("负控 · 顶层 contents: write ⇒ K4 附加命中", strconv.Itoa(len(wr.topWrite)), "1", len(wr.topWrite) == 1)

	missing := cicgScanCI([]cicgFile{{rel: "合成/缺件.yml", present: false}})
	check("缺件 · 读不到的件不参与（None 不炸）", strconv.Itoa(missing.usesTotal), "0", missing.usesTotal == 0)

	// K2：三条缺项**逐字**且**原样**（不是「已修」）。
	keys := []string{"人面看不见", "`warnings[]` 恒空", "attestation 今天没出"}
	allKeys := true
	for _, k := range keys {
		found := false
		for _, g := range cicgThreeGaps {
			if strings.Contains(g.text, k) {
				found = true
				break
			}
		}
		if !found {
			allKeys = false
		}
	}
	check("K2 · 三条缺项的条数", strconv.Itoa(len(cicgThreeGaps)), "3", len(cicgThreeGaps) == 3)
	check("K2 · 三条逐字键都在", cicgPyBool(allKeys), "True", allKeys)
	sayApplied := cicgGapsSayApplied()
	check("K2 · 三条**没有**任何「已修/已引」字样", cicgPyBool(sayApplied), "False", !sayApplied)

	// K1：分档两态（现调真源 · 合成清单）。
	tmp, err := os.MkdirTemp("", "zerg-ci-green-")
	if err != nil {
		tmp = os.TempDir()
	} else {
		defer os.RemoveAll(tmp) // ★ 自回收（TMPDIR 前后 delta=0）
	}
	mf := cicgPyJoin(tmp, "manifest.json")
	_ = os.WriteFile(mf, []byte(cicgSynthManifest), 0o644)
	relHead := strings.Repeat("1", 40)
	rcDev, rcDevOK := cicgRunProfile(root, "dev", mf, []string{"--release-head", relHead})
	rcPub, rcPubOK := cicgRunProfile(root, "public", mf, []string{"--release-head", relHead})
	check("K1 负控 · 双命中合成清单 · 开发机档 rc（只告警 ⇒ 不拦）", cicgRcStr(rcDev, rcDevOK), "0", rcDevOK && rcDev == 0)
	check("K1 负控 · 双命中合成清单 · 公开制品档 rc（必须阻断）", cicgRcStr(rcPub, rcPubOK), "1", rcPubOK && rcPub == 1)
	diff := cicgRcNE(rcDev, rcDevOK, rcPub, rcPubOK)
	check("K1 · 两档 rc 不同（同输入 · 分档有牙）", cicgPyBool(diff), "True", diff)

	for _, ln := range log {
		fmt.Fprintln(stdout, ln)
	}
	verdict := "全过"
	if len(bad) > 0 {
		verdict = "**不过**"
	}
	fmt.Fprintf(stdout, "自检结论: %s（断言 %d 条 · 失败 %d 条）\n", verdict, okCount+len(bad), len(bad))
	if len(bad) > 0 {
		return 2
	}
	return 0
}

// ── `--list-rules` ───────────────────────────────────────────────────────────

func cicgListRules(stdout io.Writer, root string) int {
	fmt.Fprintln(stdout, "公开 CI 归绿 · 分档（T10）· 真源：任务清单-缺口收口-20260923.md:68-69 · 调研-缺口-门禁证据-20260923.md:22")
	fmt.Fprintln(stdout, "K1 分档两态：开发机档只告警（rc 不拦）· 公开制品档必须阻断（source_sha != HEAD 或 dirty != false ⇒ 红）")
	fmt.Fprintf(stdout, "   —— 单一口径：现调 %s 的两个 --profile\n", cicgRelPath(root, cicgPyJoin(root, cicgProfileGateRel)))
	fmt.Fprintln(stdout, "K2 三条缺项照原样登记（仍缺 3 条 · 本件不修）：")
	for _, g := range cicgThreeGaps {
		fmt.Fprintf(stdout, "   %s %s\n", g.id, g.text)
	}
	fmt.Fprintf(stdout, "   出处（逐字）：%s\n", cicgGapsSource)
	fmt.Fprintf(stdout, "K3 R-25 按兵不动：CI 件里 %s 权限 **0 处**（出现即红）\n", strings.Join(cicgAttestTokens, " / "))
	fmt.Fprintln(stdout, "K4 附加现读（不计入三档）：uses 钉 40 位 hex · 顶层 permissions 无 write")
	fmt.Fprintln(stdout, "★ 不声称「必需检查集合 = 7 个 job」（调研:209 逐字记「未核 · 要 GitHub 侧 settings」）")
	fmt.Fprintf(stdout, "CI 件面：%s + %s/*.yml|*.yaml\n", strings.Join(cicgCIFilesAlways, " · "), cicgCIDirNew)
	fmt.Fprintln(stdout, "退码：0 分档行为对+三条照原样+R-25 未引 · 1 有命中 · 2 不给结论（件读不到/自检不过）")
	return 0
}

// ── 缺口 `Q-176`（门禁面 · 统一口）：`--json <字段>` 的**字段名打错**不许静默 / 不许炸堆栈 ✗ ──

// cicgRefuseUnknownJSONFields —— `--json <字段>` 里有**不认的名** ⇒ rc=2 ＋ 印可选字段表；没给 / 全合法 ⇒ `nil`。
func cicgRefuseUnknownJSONFields(stderr io.Writer, argv []string) *int {
	want := []string{}
	for i, x := range argv {
		if x == "--json" {
			v := ""
			if i+1 < len(argv) {
				v = argv[i+1]
			}
			want = append(want, strings.Split(v, ",")...)
		} else if strings.HasPrefix(x, "--json=") {
			want = append(want, strings.Split(strings.SplitN(x, "=", 2)[1], ",")...)
		}
	}
	bad := []string{}
	for _, f := range want {
		f = cicgPyStrip(f)
		if f == "" {
			continue
		}
		if !cicgInList(cicgJSONFields, f) {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	quoted := make([]string, 0, len(bad))
	for _, b := range bad {
		quoted = append(quoted, "`"+b+"`")
	}
	qf := make([]string, 0, len(cicgJSONFields))
	for _, f := range cicgJSONFields {
		qf = append(qf, "`"+f+"`")
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", strings.Join(quoted, " · "))
	fmt.Fprintf(stderr, "   可选字段表：%s\n", strings.Join(qf, " · "))
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开")
	r := 2
	return &r
}

// ── 位置参数（py argparse 的等价面 · 见件头未覆盖面 1）────────────────────────

func cicgArgError(stderr io.Writer, msg string) {
	fmt.Fprint(stderr, cicgUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", cicgProg, msg)
}

func cicgParseArgs(stderr io.Writer, argv []string) (cicgOpts, *int) {
	opt := cicgOpts{extraCI: []string{}}
	unrec := []string{}
	i := 0
	for i < len(argv) {
		a := argv[i]
		if a == "--" {
			i++
			for ; i < len(argv); i++ {
				unrec = append(unrec, argv[i])
			}
			break
		}
		if a == "-h" || a == "--help" {
			r := 0
			return opt, &r
		}
		if strings.HasPrefix(a, "--") {
			name := a
			val := ""
			hasVal := false
			if k := strings.Index(a, "="); k >= 0 {
				name = a[:k]
				val = a[k+1:]
				hasVal = true
			}
			switch name {
			case "--help":
				r := 0
				return opt, &r
			case "--manifest", "--extra-ci", "--json":
				if !hasVal {
					if i+1 >= len(argv) {
						cicgArgError(stderr, fmt.Sprintf("argument %s: expected one argument", name))
						r := 2
						return opt, &r
					}
					val = argv[i+1]
					i++
				}
				switch name {
				case "--manifest":
					opt.manifest = val
				case "--extra-ci":
					opt.extraCI = append(opt.extraCI, val)
				case "--json":
					opt.jsonArg = val
				}
			case "--self-test", "--list-rules":
				if hasVal {
					cicgArgError(stderr, fmt.Sprintf("argument %s: ignored explicit argument '%s'", name, val))
					r := 2
					return opt, &r
				}
				if name == "--self-test" {
					opt.selfTest = true
				} else {
					opt.listRules = true
				}
			default:
				unrec = append(unrec, a)
			}
			i++
			continue
		}
		unrec = append(unrec, a)
		i++
	}
	if len(unrec) > 0 {
		cicgArgError(stderr, fmt.Sprintf("unrecognized arguments: %s", strings.Join(unrec, " ")))
		r := 2
		return opt, &r
	}
	return opt, nil
}

// cicgUsage / cicgHelpText —— `python3 <原件> -h` 的逐字节读数（`/usr/bin/python3` 3.9.6 · COLUMNS=80 · 非 tty）。
const cicgUsage = "usage: check-public-ci-green.py [-h] [--manifest MANIFEST]\n" +
	"                                [--extra-ci EXTRA_CI] [--json JSON]\n" +
	"                                [--self-test] [--list-rules]\n"

const cicgHelpText = cicgUsage + "\n" +
	"optional arguments:\n" +
	"  -h, --help           show this help message and exit\n" +
	"  --manifest MANIFEST  样张 B 的真清单（缺省：dist/*/release/manifest.json 最新一件）\n" +
	"  --extra-ci EXTRA_CI  额外 CI 件（可重复）—— 用来**真跑负控**（合成 yml），不改仓内 CI 件 ✗\n" +
	"  --json JSON\n" +
	"  --self-test\n" +
	"  --list-rules\n"

func cicgPrintHelp(stdout io.Writer) {
	fmt.Fprint(stdout, cicgHelpText)
}

// ── 小工具（py 语义的 Go 侧对应物）──────────────────────────────────────────

// cicgResolveRoot —— cwd 上溯找仓根（见件头标准声明 (a)）。
//
// ★ 退役态安全（本件退件前置 · 类①）：锚点从「本门自指 py」**放宽**为「`scripts/gates` **目录**」——
// 自指 py 退役（不在盘）后旧锚 `cicgFileExists(<cur>/<cicgSelfRel>)` 恒假 ⇒ 仓根退化成 cwd
// （inproc 档 wd==仓根时侥幸同，但测试件 / 他 cwd 直调档即落错根、静默跳过或读错件）。
// 目录锚不随判据件存亡：py 在盘时两锚同根（逐字同旧）；py 退役时目录锚仍命中真仓根。
func cicgResolveRoot() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	cur := d
	for i := 0; i < 32; i++ {
		if cicgFileExists(cicgPyJoin(cur, cicgSelfRel)) || cicgStatIsDir(cicgPyJoin(cur, "scripts/gates")) {
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

// cicgPyJoin —— CPython `posixpath.join` 的逐段规则（本件调用点两段皆相对、无 `..`、无空段 ⇒ 恒同构）。
func cicgPyJoin(a, b string) string {
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

// cicgGoHalfRel —— py 件名 → 其 Go 半的仓内相对路径（`core/cmd/zerg/<snake>.go`）。
// 退役批口径（B轨 · 2026-10-07）：py 件退役后其 Go 半接管 ⇒ 用「Go 半在盘」当「已退役」的机检信号。
func cicgGoHalfRel(pyPath string) string {
	base := pyPath
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return "core/cmd/zerg/" + strings.ReplaceAll(strings.TrimSuffix(base, ".py"), "-", "_") + ".go"
}

// cicgProfileRetired —— 分档真源件（原 py `check-manifest-freshness-profile.py`）是否**已退役**：
// 该 py 缺席 **且** 其 Go 半（`core/cmd/zerg/check_manifest_freshness_profile.go`）在盘。
// 退役态 ⇒ K1 分档两态不过问（✗ 不 fail-closed）；缺件且无 Go 半（真消失）⇒ 未退役（照旧 fail-closed）。
func cicgProfileRetired(root string) bool {
	return !cicgFileExists(cicgPyJoin(root, cicgProfileGateRel)) &&
		cicgFileExists(cicgPyJoin(root, cicgGoHalfRel(cicgProfileGateRel)))
}

func cicgRelPath(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return r
}

func cicgFileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// cicgStatIsDir —— 目录存在性（退役态仓根目录锚用；跟随链，与 py os.path.isdir 同语义）。
func cicgStatIsDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// cicgSplitLines —— py `str.splitlines()`（含 `\v \f \x1c-\x1e \x85 \u2028 \u2029`）。
func cicgSplitLines(s string) []string {
	out := []string{}
	start := 0
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '\n' {
			out = append(out, s[start:i])
			i++
			start = i
			continue
		}
		if c == '\r' {
			out = append(out, s[start:i])
			i++
			if i < len(s) && s[i] == '\n' {
				i++
			}
			start = i
			continue
		}
		if c == '\v' || c == '\f' || c == '\x1c' || c == '\x1d' || c == '\x1e' {
			out = append(out, s[start:i])
			i++
			start = i
			continue
		}
		if c < 0x80 {
			i++
			continue
		}
		r, sz := utf8.DecodeRuneInString(s[i:])
		if r == '\x85' || r == '\u2028' || r == '\u2029' {
			out = append(out, s[start:i])
			i += sz
			start = i
			continue
		}
		i += sz
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// cicgIsPySpace —— py `str.isspace()` 的近似（`unicode.IsSpace` 补 `\x1c`-`\x1f`）。
func cicgIsPySpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// cicgPyStrip —— py `str.strip()`（无参 ⇒ 两侧空白）。
func cicgPyStrip(s string) string {
	return strings.TrimFunc(s, cicgIsPySpace)
}

// cicgLeadingWS —— py `len(ln) - len(ln.lstrip())`（按**码点**计）。
func cicgLeadingWS(s string) int {
	n := 0
	for _, r := range s {
		if !cicgIsPySpace(r) {
			break
		}
		n++
	}
	return n
}

// cicgFirstRunes —— py `s[:n]`（按**码点**切，不是按字节）。
func cicgFirstRunes(s string, n int) string {
	c := 0
	for i := range s {
		if c == n {
			return s[:i]
		}
		c++
	}
	return s
}

func cicgHead(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func cicgInList(items []string, v string) bool {
	for _, x := range items {
		if x == v {
			return true
		}
	}
	return false
}

func cicgGapTexts() []string {
	out := make([]string, 0, len(cicgThreeGaps))
	for _, g := range cicgThreeGaps {
		out = append(out, g.text)
	}
	return out
}

// cicgGapsSayApplied —— py `any(w in " ".join(texts) for w in ("已修","已引","已出"))`。
func cicgGapsSayApplied() bool {
	joined := strings.Join(cicgGapTexts(), " ")
	for _, w := range []string{"已修", "已引", "已出"} {
		if strings.Contains(joined, w) {
			return true
		}
	}
	return false
}

// cicgRcStr —— py `%s` 对 `int | None` 的读数（None ⇒ "None"）。
func cicgRcStr(rc int, ok bool) string {
	if !ok {
		return "None"
	}
	return strconv.Itoa(rc)
}

// cicgRcNE —— py `rc_dev != rc_pub`（`None` 是**独立取值**，不与任何 int 相等）。
func cicgRcNE(a int, aOK bool, b int, bOK bool) bool {
	if aOK != bOK {
		return true
	}
	if !aOK && !bOK {
		return false
	}
	return a != b
}

func cicgRcJSON(rc int, ok bool) string {
	if !ok {
		return "null"
	}
	return strconv.Itoa(rc)
}

func cicgPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// cicgPyReprList —— py `repr([])` / 一般化到字符串列表（空列表 = "[]"）。
func cicgPyReprList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(items))
	for _, s := range items {
		parts = append(parts, "'"+s+"'")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cicgPyReprTuple2 —— py `repr((n, list))`（本件只用到二元组形态）。
func cicgPyReprTuple2(n int, items []string) string {
	return "(" + strconv.Itoa(n) + ", " + cicgPyReprList(items) + ")"
}

// cicgJSONStr —— py `json.dumps(s, ensure_ascii=False)` 的转义面。
func cicgJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func cicgJSONStrList(items []string) string {
	parts := make([]string, 0, len(items))
	for _, s := range items {
		parts = append(parts, cicgJSONStr(s))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
