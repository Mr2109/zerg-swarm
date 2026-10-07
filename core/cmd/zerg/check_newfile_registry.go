// check_newfile_registry.go —— 门76 判据件 `scripts/gates/check-newfile-registry.py`（946 行 · 新版）的 **Go 真搬**。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2 新口径（外部树不再出局 ⇒ 改走**显式入参旋钮**）下的一枚：门76 = 新件登记六面总闸。
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**。
//
// 选它 / 不选它的理由：
//
//	✓ 选：只读六张登记面 + 现读盘上件集（**读**登记面合规 · 禁的是**改**）· 输出**定序**
//	  （所有集合处一律 `sorted`）· 退码 `0/1/2` 三档齐 · 行数适中 · **无人正在改**。
//	✓ 外部树（仓外台账 tsv / 仓外冻结基线）按新口径走**显式入参旋钮**（`--inventory` / `--baseline`，
//	  名照 py 同名旗标），缺省 = py 默认推导逐字同构（`--inventory` 缺省 = 同级 `Zerg-内部文档/项目文档/*/清单-脚本现状-*.tsv`
//	  取 sorted 末件；该串**不含 `..` 段** ⇒ `filepath.Join` 与 `os.path.join` 逐字同，**不做串拼**）。
//
// ★★ 唯一「不逐字可搬」的面（照实写在这里，**不粉饰**）：
//
//	① **面② 的「现读载入判据件」面 = Python `importlib` 动态载入任意 `.py` 并调用其 `scan_hits(root)`**
//	   —— Go **无等价能力**（Go 不能在进程内执行任意 Python 模块）。本件的处置：**以 `python3` 驱动
//	   子进程逐字复刻** py 的 `load_dry_module()` + `collect_paths()` + `dry_registry()` 三步
//	   （驱动脚本内嵌于本件 `gnrDryDriverPy`），**✗ 不另抄一份门⑩ 的扫描正则**（单一真源仍是那个 `.py`）。
//	   代价：本件**保留一枚解释器依赖**（仅面② 一处）；`python3` 缺席 / 驱动不可判 ⇒ 面② 按
//	   **判不了（fail-closed rc=2）** 走，✗ 不静默绿。**这一面交回父代理入账**（见件尾「交回」）。
//	② **py `--dump-baseline` 生成口自身是真口子（`GAP-20261001-21`）**：它把**当前树**的现读件集写成
//	   冻结基线 ⇒ 下一趟分母恒 `0` ⇒ 只能得 `rc=2`。**本件逐字保留该行为**（✗ 不自行修），交回入账。
//
// ★ 标准声明行（两条 · 逐条照实）
//
//	(a) **默认根定位差异**：py `REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))`
//	    ⇒ 默认根 = **脚本所在目录上溯三级**（件住 `<root>/scripts/gates/` · 与 cwd 无关）。Go **无 `__file__`
//	    语境**，改用**同语义**的可执行文件位置：Go 二进制住 `<root>/bin/`（比 py 件**浅一层**）
//	    ⇒ 映射 = `os.Executable()` 上溯**两级**（`gnrRootFromExe`）。回退两路（都先验仓根标记 · ✗ 不误判临时目录）：
//	    ② `runtime.Caller(0)` 的**本件源码位置**上溯（测试二进制住临时目录时走这条）；③ cwd 上溯（前两路皆判不了
//	    时的兜底 · 与旧行为同）。⇒ 「同一条命令在任何目录跑同结果」成立，这一面**属逐字对齐承诺**。
//	(b) **自指检查（四形态逐条判）**：
//	    ① 本件落在门76 **自己**的扫描面内？——**否**。本件的集合定义 = `scan_ledger_face(root)`，它只遍历
//	       `<root>/scripts/**`；本件落 `core/cmd/zerg/check_newfile_registry.go` ⇒ **不在**该面内
//	       ⇒ 不进 ①②③⑤ 任何一格（已对拍：真仓 甲 格分母 3 件，与 py 逐字同）。
//	    ② 本件正文含**真引号的路径原串**会不会把它自己扫红？——本件**扫描面只判「件名在不在表里」**，
//	       不读任何被扫件正文 ⇒ **无**此患（零「写进被扫面的标记字面」）。
//	    ③ 本件是否落在**他门**的扫描面内？——**是（门⑩）**：`scripts/gates/check-dryrun-semantics.py` 的
//	       `SCAN_DIRS` 含 `core/`、`SCAN_EXTS` 含 `*.go`、模式 `dr[y]... run`（此处**故意拆写**）。
//	       **处理**：本件全文**零处**出现该许可模式（判据件的件名/登记表名在本件里**一律不落字面** ——
//	       面② 的判据件名由**步骤表现读**得出）⇒ 两态读数逐字相同。
//	    ④ 本件是否落在 `check-hardcoded-private-paths`（`SCAN_DIRS` 含 `core/`）面内？——**否**：本件零绝对
//	       私有路径字面（家目录走 `os.UserHomeDir()`，✗ 不写 `/Users/...`）。
//	    ⇒ 综合：自指 = **否**（唯一相邻面 = 门⑩，已按 ③ 处置并留证）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 帮助 / usage 正文**（`-h/--help` · 未知旗标 · 缺值 · 位置参数）：本件落 rc=0 / 2 与 py 同，
//	   且 `gnrUsage` / `gnrHelp` **逐字内嵌现读 py 正文**（含新旗标 `--json SECTIONS` / `--yes` 与两段
//	   位置），但 argparse 的**自动折行算法**不逐字复刻（只按 py 现读输出**照抄换行位置**）。
//	   ★ **这一面已由机械拦阻兜住**（✗ 不再靠人记住同步）：`check_newfile_registry_test.go` 的
//	   `TestGnrHelpParityWithPy` 现跑 `/usr/bin/python3 <py 件> -h|--help|--zzz` 与 `gnrHelp` **逐字节**对拍
//	   （stdout + stderr + rc 三面），并钉住 py 正文 sha256（无 python3 时的退路 · 见测件头）。
//	② **默认根定位**：见上 (a)。★ 已改为**与 py 同语义**（exe 上溯两级 + 源码位置回退 ⇒ 与 cwd 无关）。
//	   唯一残留差：正式制品用 `-trimpath` 构建 ⇒ `runtime.Caller` 的**源码回退面**失效，但**主路**
//	   `os.Executable()` 不受 `-trimpath` 影响 ⇒ 正式制品仍逐字同；只有「exe 不在 `<root>/bin/` 且又走了
//	   源码回退」这一组合下才会变（现役布局不触发）。
//	③ **非 UTF-8 输入的字面替换粒度**：py `errors="replace"` 与 Go `strings.ToValidUTF8` 对**连续非法字节**
//	   的 U+FFFD 分割粒度可能不同（真仓六张表皆合法 UTF-8 ⇒ 现读零影响）。
//	④ **面② 的判据件载入例外**：py 在 `spec.loader.exec_module` / `scan_hits(root)` 抛异常时 = **未捕异常
//	   ⇒ 回溯 + rc=1**；本件按新口径 **fail-closed rc=2**（正文亦不同）。
//	⑤ **面② 的 `__pycache__` 副产物**：py 走 `importlib` 会把字节码缓存写到判据件旁（真仓现读确见
//	   `scripts/gates/__pycache__`）；本件驱动**关掉字节码写入**（`sys.dont_write_bytecode`）⇒ 盘上副产物不同，
//	   读数面不受影响。
//	⑥ **`--self-test` 夹具的 `mkdtemp` 随机后缀**：py 自检把夹具落在 `<TMPDIR>/gnr-selftest-XXXXXXXX`
//	   并把**该路径印进正文**（`仓根：…`）⇒ 同一实现连跑两发亦不同。本件同样打印该路径 ⇒ 该格属**非确定面**，
//	   掩码该段后逐字节可比。
//	⑦ 本件**不做** py 的 `import glob as globmod` 那种 `**` 递归语义（`glob.glob` 缺省 `recursive=False`
//	   亦不递归）⇒ 两侧同为「单层 glob」，现读无差。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func checkNewfileRegistryGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 判到了（报告档；`--strict` 下 R1/R2/R3/R5/R7 任一 > 0 ⇒ 1） / 2 不给结论
//	      ★ rc=2 现读含两条**用法错**（照 py `main()` 判序）：`--yes` 不与 `--dump-baseline` 连用 ·
//	        `--json` 与 `--dump-baseline` 同用 · `--json` 断面名不认 / 为空。
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见上「未覆盖面」）
//
// 默认干跑（写面最小 · 比业界写器保守）
//
//	本件默认**不写任何件**（只读报告 / 干跑）。唯一的写面 = `--dump-baseline`（生成冻结基线 · 仓外生成件），
//	且**只有显式 `--yes` 才真落**：不带 `--yes` 时它只把计划（落点 + 将写件数）打到 stdout、
//	**一个字节都不写**。这是本件与「默认真写」的写器（业界常见形态）**刻意不同**的保守面 ——
//	目的是「先看计划、再显式确认落点」。（`--yes` 不与 `--dump-baseline` 连用 ⇒ 用法错 rc=2；其余旗标一律只读。）
//
// 机器面（`--json`）
//
//	`--json <断面>`：断面为逗号分隔、只认 `faces` 与 `self`；给了就**只出 JSON**（人面向 stdout 让位）。
//	JSON 恒含：`expected_faces`（本件十面名）· `reported_faces`（本次实际报出的面）· `counts`（五态计数
//	PASS/FAIL/BLOCKED/REPORT/UNKNOWN）· `all_green`（判到了且 0 条缺登记）；`faces` / `self` 断面分别加
//	逐面详情与自指详情。断面名不认 / 为空 ⇒ 用法错 rc=2（不给结论）。`--json` 不与 `--dump-baseline` 同用 ⇒ 用法错 rc=2。
//
// 交回父代理入账（✗ 本件不自行修）：
//
//	· `GAP-20261001-21`：`--dump-baseline` 生成口打**当前树** ⇒ 分母恒 0（逐字保留，见上 ★★②）。
//	· **面② 的解释器依赖**（见上 ★★①）：本件是 M2 已落件里**唯一**保留 `python3` 子进程的一枚。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// ── 常量（逐字照 py）───────────────────────────────────────────────────────
const (
	gnrSelfRel     = "scripts/gates/check-newfile-registry.py"
	gnrMarksRel    = "scripts/公开标记.tsv"
	gnrStepsRel    = "scripts/gates/precommit-gates.sh"
	gnrCovRel      = "scripts/gates/gate-coverage.config.json"
	gnrCovJudgeRel = "scripts/gates/check-gate-coverage.py"
	gnrPubRel      = "scripts/build/publish-public.sh"
	gnrMirrorRel   = "publish/mirror-public-lib.py"
	// 面⑨ 真源：`.sh` 门面冻结清册在 real-gates.sh 里（与门65 真跑档那一面同源）
	gnrShGatesRel = "scripts/gates/real-gates.sh"
	gnrShGatesDir = "scripts/gates"
	gnrLedgerSib  = "Zerg-内部文档"
	gnrStepMark   = "干跑"
	gnrAddStep    = "add_step"
	gnrJudgeRE    = `scripts/[A-Za-z0-9_/.-]+\.py`
	gnrExcludesRE = `(?s)EXCLUDES=\((.*?)\n\)`
	gnrDropRE     = `(?s)DROP_EXACT\s*=\s*\{(.*?)\n\}`
	gnrShGatesRE  = `(?s)SH_GATES=\((.*?)\)`
)

var gnrLedgerGlob = filepath.Join("项目文档", "*", "清单-脚本现状-*.tsv")

// gnrScriptExcludes —— 逐字对齐 `script_inventory_test.go:28-32` 的 13 个排除面。
var gnrScriptExcludes = []string{
	"target", "node_modules", "dist", "bin", "vendor", "data", ".git", ".venv", "venv",
	".build", "zerg-wt", "_history", "__pycache__",
}

var (
	gnrJudgeRECompiled  = regexp.MustCompile(gnrJudgeRE)
	gnrExcludesCompiled = regexp.MustCompile(gnrExcludesRE)
	gnrDropCompiled     = regexp.MustCompile(gnrDropRE)
	gnrShGatesCompiled  = regexp.MustCompile(gnrShGatesRE)
)

// ── 机器面：门76 的**十面**（逐字对齐 py `FACES` · 顺序 = 打印顺序）────────────
// 为什么是这十面：六张登记表（①–⑥）＋ **⑧家目录批准件**（host-bound · 机器相关面）＋
// **⑨.sh门面清册**（盘上 `scripts/gates/*.sh` ⇄ `real-gates.sh` 的 `SH_GATES` 双向差集）＋
// 口径差栏（参照读数）＋ 自指（本门对自身的登记情形）。
// ★ 面⑦ 处**故意留空档** —— 该号在 `gnrRunSync` 里已绑「判据注册表」（`zerg gate check check-judge-registry --emit`）⇒ 新面**取⑨、不占⑦**（✗ 别顺手把 ⑦ 填了：会与那条正门指路撞号）。
// ★ `gnrExpectedFaces()` **跑前先生成**这张清单；任何**未产出记录**的面由 `gnrFaceRecords` 合成
// `state=UNKNOWN · reason_code=MISSING_FACE · decided=false` 并**退码钉 2**（禁静默跳过缺面）。
var gnrFaces = []string{
	"①公开标记表",
	"②干跑登记",
	"③门①两张真源",
	"④脚本台账",
	"⑤两器排除表",
	"⑥失败文案面",
	"⑧家目录批准件",
	"⑨.sh门面清册",
	"口径差栏",
	"自指",
}

// 五态（`precommit-gates.sh` 的步骤级四档 PASS/FAIL/BLOCKED/REPORT ＋ 自报未知态 UNKNOWN）：
//
//	PASS    = 判到了且该面 0 条缺登记（R4/R6 = 无发现）
//	FAIL    = 判到了且该面 > 0 条缺登记（R1/R2/R3/R5）
//	BLOCKED = 该面自己的真源读不到 / 面判不了（面级不给结论）
//	REPORT  = 只报不判的面（R4/R6 旁栏不进退码 · 口径差栏/自指 = 参照/自指读数）
//	UNKNOWN = 整跑在**面级判读之前**就短路（分母级判不了：扫面不在盘 / 分母 0 / 基线不在盘）⇒ 该面结论未知
//	★ **HOST-REQUIRED** = **机器相关面（host-bound）**：本机家目录态、不进仓、不可移植 ⇒ 既不 PASS
//	  也不硬 FAIL。聚合时**不计入 pass 也不计入 fail**，但必须**显式列出并暴露计数**。
var gnrFacesStates = []string{"PASS", "FAIL", "BLOCKED", "REPORT", "UNKNOWN", "HOST-REQUIRED"}

// ★ **机器相关面（host-bound）**：真身在**本机家目录**（`~/.zerg/state/approvals/dev_edit.json`），
// 不进仓、随机器而变 ⇒ 报 HOST-REQUIRED / reason_code=SKIPPED-HOST-BOUND，聚合时不计 pass/fail。
var gnrHostBoundFaces = []string{"⑧家目录批准件"}

// gnrApprovalsRel —— py `APPROVALS_REL = os.path.join(".zerg","state","approvals","dev_edit.json")` 的字面。
// ★ py 侧仅作「真身在哪」的声明面（正文从不读它）⇒ 本件逐字保留该声明，✗ 不擅自把它变成读面。
const gnrApprovalsRel = ".zerg/state/approvals/dev_edit.json"

// 面级**原因码**（机器面 `face_records[<面>].reason_code` 的闭集）
const (
	gnrReasonOK               = "OK"
	gnrReasonMissingEntry     = "MISSING_ENTRY"      // 判到了且该面 > 0 条缺登记（FAIL）
	gnrReasonSourceUnreadable = "SOURCE_UNREADABLE"  // 该面真源读不到（BLOCKED）
	gnrReasonReportOnly       = "REPORT_ONLY"        // 只报不判（REPORT）
	gnrReasonUnjudged         = "UNJUDGED"           // 整跑面级判读前短路（UNKNOWN）
	gnrReasonMissingFace      = "MISSING_FACE"       // ★ 未产出记录的面（聚合器合成 · 退码钉 2）
	gnrReasonHostBound        = "SKIPPED-HOST-BOUND" // ★ 机器相关面未跑（HOST-REQUIRED）
)

// gnrDryDriverPy —— py `load_dry_module()` + `collect_paths()` + `dry_registry()` 的**逐字复刻驱动**。
// argv：1=判据件绝对路径 · 2=仓根 · 3=步骤表里抽出的**仓根相对**判据件名（仅用于例外文案）。
// 输出：一行 JSON（**唯一** stdout 内容；模块自身的打印被临时导去 stderr）。
const gnrDryDriverPy = `import importlib.util, io, json, os, sys

sys.dont_write_bytecode = True
cand, root, relp_arg = sys.argv[1], sys.argv[2], sys.argv[3]
res = {"rel": None, "hits_ok": False, "hits": [], "reg_ok": False, "reg": [], "problem": None}
_real = sys.stdout
sys.stdout = sys.stderr


def _emit():
    sys.stdout = _real
    sys.stdout.write(json.dumps(res, ensure_ascii=False))
    sys.stdout.flush()
    raise SystemExit(0)


spec = None
try:
    spec = importlib.util.spec_from_file_location("_gnr_dry_judge", cand)
except Exception:
    spec = None
if spec is None or spec.loader is None:
    res["problem"] = "判据件载入不了：%s" % (relp_arg,)
    _emit()

mod = importlib.util.module_from_spec(spec)
try:
    spec.loader.exec_module(mod)
except Exception as exc:
    res["problem"] = "判据件载入不了：%s（%s）" % (relp_arg, exc)
    _emit()

try:
    raw = mod.scan_hits(root)
except Exception as exc:
    res["problem"] = "判据件扫描函数不可判：%s" % (exc,)
    _emit()

if raw is None or (isinstance(raw, tuple) and len(raw) > 0 and raw[0] is None):
    why = raw[1] if isinstance(raw, tuple) and len(raw) > 1 else raw
    res["problem"] = "扫描空转 / 不可判：%s" % (why,)
    _emit()

hits = set()


def _walk(x):
    if isinstance(x, str):
        hits.add(x)
    elif isinstance(x, (set, frozenset, list, tuple)):
        for i in x:
            _walk(i)
    elif isinstance(x, dict):
        for k in x:
            _walk(k)


_walk(raw)
res["hits_ok"] = True
res["hits"] = sorted(hits)

relp = getattr(mod, "REGISTRY_REL", None)
if isinstance(relp, str):
    res["rel"] = relp
if not relp:
    res["problem"] = "判据件未声明登记表路径（REGISTRY_REL）"
    _emit()

path = os.path.join(root, relp)
if not os.path.isfile(path):
    res["problem"] = "登记表不在盘：%s" % (relp,)
    _emit()
try:
    with open(path, encoding="utf-8", errors="replace") as fh:
        data = json.loads(fh.read())
except ValueError as exc:
    res["problem"] = "登记表解析失败：%s" % (exc,)
    _emit()
except OSError:
    res["problem"] = "登记表不在盘：%s" % (relp,)
    _emit()

reg = set()
for e in (data.get("件") or []):
    if isinstance(e, dict) and e.get("file") and isinstance(e["file"], str):
        reg.add(e["file"])
if reg:
    res["reg_ok"] = True
    res["reg"] = sorted(reg)
_emit()
`

// ── 基础工具（逐字对齐 py 语义）─────────────────────────────────────────────

func gnrIsDir(p string) bool { st, err := os.Stat(p); return err == nil && st.IsDir() }

func gnrIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// gnrReadText —— py `open(path, encoding="utf-8", errors="replace").read()`。
func gnrReadText(p string) (string, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	return strings.ToValidUTF8(string(b), "\uFFFD"), true
}

// gnrPyErrText —— py `str(OSError)` 的近似（用于「基线在盘但读不出」那格 · 逐字对齐 errno 正文）。
//
//	CPython：`[Errno 13] Permission denied: '<path>'`（首字母大写 · 单引号路径）。
//	Go os.ReadFile 返回 *PathError ⇒ 取其 syscall.Errno（`permission denied`）首字母大写即与 C `strerror` 同串。
func gnrPyErrText(err error, path string) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		msg := errno.Error()
		if msg != "" {
			r := []rune(msg)
			r[0] = unicode.ToUpper(r[0])
			msg = string(r)
		}
		return fmt.Sprintf("[Errno %d] %s: '%s'", int(errno), msg, path)
	}
	return err.Error()
}

// gnrSplitLines —— py `str.splitlines()` 的换行集（✗ 不是 `strings.Split(s, "\n")`）。
func gnrSplitLines(s string) []string {
	out := []string{}
	var cur strings.Builder
	for i := 0; i < len(s); {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if gnrIsLineBreak(r) {
			out = append(out, cur.String())
			cur.Reset()
			i += sz
			if r == '\r' && i < len(s) && s[i] == '\n' {
				i++
			}
			continue
		}
		cur.WriteRune(r)
		i += sz
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func gnrIsLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', 0x0b, 0x0c, 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// gnrRel —— py `os.path.relpath(p, root).replace(os.sep, "/")`。
func gnrRel(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(r)
}

// gnrTruthy —— py 真值语义（用于 json 取值）。
func gnrTruthy(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case float64:
		return x != 0
	case []interface{}:
		return len(x) > 0
	case map[string]interface{}:
		return len(x) > 0
	}
	return true
}

// gnrPyStr —— py `"%s" % v` 的形态（None ⇒ "None"）。
func gnrPyStr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return x
	case bool:
		if x {
			return "True"
		}
		return "False"
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	case []interface{}:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, gnrPyStr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return fmt.Sprintf("%v", v)
}

// gnrStripQ —— py `s.strip('"').strip("'")`。
func gnrStripQ(s string) string {
	s = strings.Trim(s, `"`)
	return strings.Trim(s, "'")
}

func gnrContains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// ── 分母：两套现读口径 ─────────────────────────────────────────────────────
// ★ 遍历/排序点交代：py `os.walk` 里**明写** `dirnames[:] = sorted(...)` 与 `for name in sorted(filenames)`，
//
//	所以**遍历序**是「目录名 sorted 的 DFS × 文件名单层 sorted」；但本函数**返回集合**，而**输出行序**的
//	决定者是 `audit()` 里的 `sorted(ledger_face)` / `sorted(ledger_face - base)`（res["roster"] /
//	new_files / 对差 三处）⇒ **遍历序不决定任何一行输出**。本件按 Go 侧 `f.ReadDir(-1)`（**原序**）
//	收集进 map，序不进输出。✗ 不用 `os.ReadDir`（它会替我们排序、且与本函数无关）。
func gnrScanLedgerFace(root string) map[string]bool {
	base := filepath.Join(root, "scripts")
	if !gnrIsDir(base) {
		return nil
	}
	out := map[string]bool{}
	var walk func(dir string)
	walk = func(dir string) {
		fh, err := os.Open(dir)
		if err != nil {
			return
		}
		ents, err := fh.ReadDir(-1) // 原序：✗ 不用排序版
		fh.Close()
		if err != nil {
			return
		}
		for _, e := range ents {
			name := e.Name()
			p := filepath.Join(dir, name)
			lst, lerr := os.Lstat(p)
			if lerr != nil {
				continue
			}
			isLink := lst.Mode()&os.ModeSymlink != 0
			st := lst
			if isLink {
				if s2, e2 := os.Stat(p); e2 == nil {
					st = s2
				}
			}
			if st.IsDir() {
				// py `os.walk(followlinks=False)`：符号链接目录进 dirnames 但**不下降**、也不当文件处理。
				if isLink {
					continue
				}
				if gnrContains(gnrScriptExcludes, name) {
					continue
				}
				walk(p)
				continue
			}
			isScript := strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".py")
			if !isScript {
				if st.Mode().Perm()&0o111 == 0 { // py `os.access(p, os.X_OK)`
					continue
				}
				txt, ok := gnrReadText(p)
				if !ok {
					continue
				}
				if !strings.HasPrefix(txt, "#!") {
					continue
				}
			}
			out[gnrRel(root, p)] = true
		}
	}
	walk(base)
	return out
}

type gnrGate1Face struct {
	set     bool
	files   map[string]bool
	rawData map[string]interface{}
}

// gnrScanGate1Face —— 门① 候选面口径（`gate-coverage.config.json` 的 `scan.script_globs` + `nosuffix_dir`）。
func gnrScanGate1Face(root string) gnrGate1Face {
	cfg := filepath.Join(root, gnrCovRel)
	if !gnrIsFile(cfg) {
		return gnrGate1Face{}
	}
	txt, ok := gnrReadText(cfg)
	if !ok {
		return gnrGate1Face{}
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(txt), &data); err != nil {
		return gnrGate1Face{}
	}
	scan, _ := data["scan"].(map[string]interface{})
	if scan == nil {
		scan = map[string]interface{}{}
	}
	globs := []string{}
	if g, ok := scan["script_globs"].([]interface{}); ok {
		for _, v := range g {
			if s, ok := v.(string); ok {
				globs = append(globs, s)
			}
		}
	}
	if len(globs) == 0 {
		return gnrGate1Face{rawData: data}
	}
	comps := map[string]bool{}
	if ex, ok := data["exclude"].(map[string]interface{}); ok {
		if pc, ok := ex["path_components"].([]interface{}); ok {
			for _, v := range pc {
				if s, ok := v.(string); ok {
					comps[s] = true
				}
			}
		}
	}
	out := map[string]bool{}
	for _, g := range globs {
		matches, err := filepath.Glob(filepath.Join(root, g))
		if err != nil {
			continue
		}
		for _, p := range matches {
			if !gnrIsFile(p) {
				continue
			}
			r := gnrRel(root, p)
			if gnrAnyComp(r, comps) {
				continue
			}
			out[r] = true
		}
	}
	if ns, ok := scan["nosuffix_dir"].(string); ok && ns != "" {
		d := filepath.Join(root, ns)
		if gnrIsDir(d) {
			if ents, err := os.ReadDir(d); err == nil {
				names := make([]string, 0, len(ents))
				for _, e := range ents {
					names = append(names, e.Name())
				}
				sort.Strings(names) // py `sorted(os.listdir(d))` 明写
				for _, name := range names {
					p := filepath.Join(d, name)
					if gnrIsFile(p) && !strings.Contains(name, ".") {
						out[gnrRel(root, p)] = true
					}
				}
			}
		}
	}
	return gnrGate1Face{set: true, files: out, rawData: data}
}

func gnrAnyComp(rel string, comps map[string]bool) bool {
	for _, seg := range strings.Split(rel, "/") {
		if comps[seg] {
			return true
		}
	}
	return false
}

// ── 六张表：逐张现读 ───────────────────────────────────────────────────────
// gnrLoadMarks —— ① `公开标记.tsv` ⇒ {rel: public}；读不到 / 0 行 ⇒ nil。
// ★ py dict 的**插入序**本件用 `keys` 切片保序；本件对 marks **只做按键查值**（✗ 零 map 迭代）。
func gnrLoadMarks(root string) (map[string]string, []string) {
	p := filepath.Join(root, gnrMarksRel)
	if !gnrIsFile(p) {
		return nil, nil
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil, nil
	}
	rows := map[string]string{}
	keys := []string{}
	for i, line := range gnrSplitLines(txt) {
		if strings.TrimSpace(line) == "" || i == 0 {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 2 {
			continue
		}
		k := strings.TrimSpace(cols[0])
		if _, seen := rows[k]; !seen {
			keys = append(keys, k)
		}
		rows[k] = strings.TrimSpace(cols[1])
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows, keys
}

// gnrLoadSteps —— ③a 步骤表：(add_step 行列表, 全文, 是否拿到)。
func gnrLoadSteps(root string) ([]string, string, bool) {
	p := filepath.Join(root, gnrStepsRel)
	if !gnrIsFile(p) {
		return nil, "", false
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil, "", false
	}
	lines := []string{}
	for _, ln := range gnrSplitLines(txt) {
		if strings.Contains(ln, gnrAddStep) {
			lines = append(lines, ln)
		}
	}
	return lines, txt, true
}

// gnrWlItem —— 门① 白名单条目（只取三格 · 其余不关心）。
type gnrWlItem struct {
	path   interface{}
	reason interface{}
	date   interface{}
}

// gnrLoadWhitelistScripts —— ③b：返回 (条目列表, 问题列表)；缺 ⇒ (nil, [问题])。
func gnrLoadWhitelistScripts(root string) ([]gnrWlItem, []string) {
	p := filepath.Join(root, gnrCovRel)
	if !gnrIsFile(p) {
		return nil, []string{"面③ 判不了：" + gnrCovRel + " 不在盘"}
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil, []string{"面③ 判不了：" + gnrCovRel + " 不在盘"}
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(txt), &data); err != nil {
		return nil, []string{"面③ 判不了：" + gnrCovRel + " 解析失败（" + err.Error() + "）"}
	}
	items := []gnrWlItem{}
	if wl, ok := data["whitelist"].(map[string]interface{}); ok {
		if arr, ok := wl["scripts"].([]interface{}); ok {
			for _, v := range arr {
				m, ok := v.(map[string]interface{})
				if !ok {
					items = append(items, gnrWlItem{})
					continue
				}
				items = append(items, gnrWlItem{path: m["path"], reason: m["reason"], date: m["date"]})
			}
		}
	}
	problems := []string{}
	for _, it := range items {
		if !gnrTruthy(it.reason) || !gnrTruthy(it.date) {
			problems = append(problems, "面③ 白名单条目缺 reason/date："+gnrPyStr(it.path))
		}
	}
	return items, problems
}

// gnrLoadExcludes —— ⑤a `EXCLUDES=( … )`（活跃条目）。
func gnrLoadExcludes(root string) map[string]bool {
	p := filepath.Join(root, gnrPubRel)
	if !gnrIsFile(p) {
		return nil
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil
	}
	m := gnrExcludesCompiled.FindStringSubmatch(txt)
	if m == nil {
		return nil
	}
	out := map[string]bool{}
	for _, ln := range gnrSplitLines(m[1]) {
		if i := strings.Index(ln, "#"); i >= 0 { // ★ ② 剥行内注释（消假阳性 · 与 gnrLoadDropExact 同法）
			ln = ln[:i]
		}
		s := strings.TrimSpace(ln)
		if s == "" {
			continue
		}
		s = gnrStripQ(s)
		if s != "" {
			out[s] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// gnrLoadDropExact —— ⑤b `DROP_EXACT = { … }`。
func gnrLoadDropExact(root string) map[string]bool {
	p := filepath.Join(root, gnrMirrorRel)
	if !gnrIsFile(p) {
		return nil
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil
	}
	m := gnrDropCompiled.FindStringSubmatch(txt)
	if m == nil {
		return nil
	}
	out := map[string]bool{}
	for _, ln := range gnrSplitLines(m[1]) {
		s := strings.TrimSpace(strings.SplitN(ln, "#", 2)[0])
		s = strings.TrimRight(s, ",")
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out[gnrStripQ(s)] = true
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// gnrSortedSetDiff —— py `sorted(a - b)`（集合差 · 逐元素码位序排序）。
func gnrSortedSetDiff(a, b map[string]bool) []string {
	out := []string{}
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// gnrSortedKeys —— py `sorted(<set>)`（集合 → 排序切片）。
func gnrSortedKeys(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// gnrScanGatesSh —— py `scan_gates_sh(root)`：⑨ 盘上面 = `scripts/gates/*.sh` 的 basename 集
// （**非递归** · 与 real-gates.sh 的 `${GATEDIR}/*.sh` 同口径）。面不在 / 0 件 ⇒ nil。
func gnrScanGatesSh(root string) map[string]bool {
	d := filepath.Join(root, gnrShGatesDir)
	if !gnrIsDir(d) {
		return nil
	}
	out := map[string]bool{}
	names := []string{}
	ents, err := os.ReadDir(d)
	if err != nil {
		return nil
	}
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names) // py `sorted(os.listdir(d))`
	for _, name := range names {
		p := filepath.Join(d, name)
		if gnrIsFile(p) && strings.HasSuffix(name, ".sh") {
			out[name] = true
		}
	}
	if len(out) == 0 {
		return nil // py `return out or None`
	}
	return out
}

// gnrLoadShGates —— py `load_sh_gates(root)`：⑨ 清册面 = `real-gates.sh` 的 `SH_GATES=( … )` 冻结清册
// （逐只点名 · 支撑反斜杠续行）。缺 / 0 条 / 解析不了 ⇒ nil。
func gnrLoadShGates(root string) map[string]bool {
	p := filepath.Join(root, gnrShGatesRel)
	if !gnrIsFile(p) {
		return nil
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil
	}
	m := gnrShGatesCompiled.FindStringSubmatch(txt)
	if m == nil {
		return nil
	}
	out := map[string]bool{}
	for _, ln := range gnrSplitLines(m[1]) {
		s := strings.TrimSpace(strings.SplitN(ln, "#", 2)[0]) // py `ln.split("#")[0].strip()`
		for _, tok := range strings.Fields(s) {               // py `s.split()`
			tok = strings.TrimSpace(gnrStripQ(tok)) // py `tok.strip('"').strip("'").strip()`
			if strings.HasSuffix(tok, ".sh") {
				out[tok] = true
			}
		}
	}
	if len(out) == 0 {
		return nil // py `return out or None`
	}
	return out
}

// gnrLoadLedgerRows —— ④ 仓外台账 tsv 行集（**显式入参旋钮 `--inventory`**）。
func gnrLoadLedgerRows(root, inv string, invSet bool) (map[string]bool, string, bool) {
	path := inv
	if invSet && inv != "" && !filepath.IsAbs(inv) {
		path = filepath.Join(root, inv)
	}
	if !invSet {
		sib := filepath.Join(filepath.Dir(gnrAbs(root)), gnrLedgerSib)
		pat := filepath.Join(sib, gnrLedgerGlob)
		cand, _ := filepath.Glob(pat)
		sort.Strings(cand) // py `sorted(globmod.glob(...))`
		if len(cand) > 0 {
			path = cand[len(cand)-1]
		} else {
			path = ""
		}
	}
	if path == "" || !gnrIsFile(path) {
		return nil, path, false
	}
	txt, ok := gnrReadText(path)
	if !ok {
		return nil, path, false
	}
	rows := map[string]bool{}
	for i, line := range gnrSplitLines(txt) {
		if strings.TrimSpace(line) == "" || i == 0 {
			continue
		}
		rows[strings.TrimSpace(strings.SplitN(line, "\t", 2)[0])] = true
	}
	if len(rows) == 0 {
		return nil, path, true
	}
	return rows, path, true
}

func gnrAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

// gnrLoadWording —— ⑥ 门① 判据件的「处置」文案是否已指到白名单档。
func gnrLoadWording(root string) (*bool, []string, bool) {
	p := filepath.Join(root, gnrCovJudgeRel)
	if !gnrIsFile(p) {
		return nil, nil, false
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil, nil, false
	}
	lines := gnrSplitLines(txt)
	type blk struct {
		b []string
		s string
	}
	blocks := []blk{}
	idx := []int{}
	for j, ln := range lines {
		if strings.Contains(ln, "处置") {
			idx = append(idx, j)
		}
	}
	for _, i := range idx {
		lo := i - 1
		if lo < 0 {
			lo = 0
		}
		hi := i + 4
		if hi > len(lines) {
			hi = len(lines)
		}
		seg := make([]string, 0, hi-lo)
		for _, ln := range lines[lo:hi] {
			seg = append(seg, strings.TrimSpace(ln))
		}
		blob := strings.Join(seg, " ")
		if strings.Contains(blob, "处置") {
			blocks = append(blocks, blk{seg, blob})
		}
	}
	rel := []blk{}
	for _, b := range blocks {
		if strings.Contains(b.s, gnrAddStep) || strings.Contains(b.s, "挂门") {
			rel = append(rel, b)
		}
	}
	if len(rel) == 0 {
		return nil, nil, false
	}
	fixed := true
	for _, b := range rel {
		if !(strings.Contains(b.s, "白名单") && strings.Contains(b.s, "reason")) {
			fixed = false
		}
	}
	out := []string{}
	for _, b := range rel {
		pick := ""
		for _, x := range b.b {
			if strings.Contains(x, "处置") {
				pick = x
				break
			}
		}
		if pick == "" && len(b.b) > 0 {
			pick = b.b[0]
		}
		out = append(out, pick)
	}
	if len(out) > 4 {
		out = out[:4]
	}
	return &fixed, out, true
}

// ── 查缺档（三合并式 · 判红机制 · 隔离工作树）─────────────────────────────────
// ★ **判红三合并式**：三条**任一非空即红** ——
//
//	  ① tracked 有改动 :  `git diff --exit-code HEAD -- <paths>`   （rc=1 ⇒ 有）
//	  ② untracked      :  `git ls-files --others --exclude-standard -- <paths>`
//	  ③ ignored        :  `git ls-files --others --ignored --exclude-standard -- <paths>`
//	★ **禁只用 `git diff` 判红** —— 它看不见 untracked（②）与 ignored（③）⇒ 那两格必须单列现读。
//	★ 若改用 `git status --porcelain`，**必须显式 `--untracked-files=all`**；本件**优先 ls-files --others** 那条路。
//
// ★ **隔离**：`git worktree add -f -q <tmp> HEAD` 取 HEAD 参照；**退出必清**（Go defer 复刻 py try/finally）。
// ★ **生成前先清空输出目录**（防旧件残留看着一致）—— 见 `gnrRunCheckMissing` 的 `--outdir` 分支。
var gnrChkMissingPathsDefault = []string{"scripts"}

// gnrGitUnreadable —— py `_git` 的 `rc=None`（git 起不来 / OSError）在本件里的哨兵值。
const gnrGitUnreadable = -1

// gnrRcOrNone —— py `"%s" % rc` 的形态（None ⇒ "None"）。
func gnrRcOrNone(rc int) string {
	if rc == gnrGitUnreadable {
		return "None"
	}
	return strconv.Itoa(rc)
}

// gnrPyStrBlank —— py `ln.strip() == ""`（py str.strip 剥的空白集含 \x1c-\x1f / \x85 / \xa0，
// 比 Go `unicode.IsSpace` 更宽 ⇒ 显式补齐这几枚，✗ 不用裸 `strings.TrimSpace`）。
func gnrPyStrBlank(s string) bool {
	for _, r := range s {
		if r == '	' || r == '\n' || r == '\v' || r == '\f' || r == '\r' || r == ' ' ||
			r == 0x1c || r == 0x1d || r == 0x1e || r == 0x1f || r == 0x85 || r == 0xa0 || unicode.IsSpace(r) {
			continue
		}
		return false
	}
	return true
}

// gnrGitLines —— py `[ln for ln in p.stdout.splitlines() if ln.strip()]`。
func gnrGitLines(s string) []string {
	out := []string{}
	for _, ln := range gnrSplitLines(s) {
		if !gnrPyStrBlank(ln) {
			out = append(out, ln)
		}
	}
	return out
}

// gnrGit —— py `_git(root, args)`：跑一条 git 命令。
// 返回 (rc, out, errs)：rc=gnrGitUnreadable 表示「git 起不来（OSError）」；out = 非空行（保持 git 原序）。
// ★ 行序决定者：本函数**不排序**（同 py）；三条列表各自在调用方 `sort.Strings`（等价 py `sorted`）。
func gnrGit(root string, args ...string) (int, []string, string) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return gnrGitUnreadable, []string{}, err.Error() // 起不来（OSError 等价）
		}
		return cmd.ProcessState.ExitCode(), gnrGitLines(outb.String()), strings.TrimSpace(errb.String())
	}
	return 0, gnrGitLines(outb.String()), strings.TrimSpace(errb.String())
}

// gnrGitWorktreeKind —— py `git_worktree_kind(root)`：'yes' / 'no'（非 git 工作树）/ 'unreadable'。
func gnrGitWorktreeKind(root string) string {
	_, stErr := os.Stat(filepath.Join(root, ".git")) // py os.path.exists（跟随符号链接）
	hasDotGit := stErr == nil
	rc, _, _ := gnrGit(root, "rev-parse", "--is-inside-work-tree")
	if rc == gnrGitUnreadable {
		return "unreadable" // git 起不来
	}
	if rc == 0 {
		return "yes"
	}
	if hasDotGit {
		return "unreadable"
	}
	return "no"
}

// gnrWorktreeAdd —— py `_worktree_add(root)`：`git worktree add -f -q <tmp>/co HEAD`。
// 返回 (工作树路径, 错误文案)；失败时路径为空串、并已清临时父目录。
// ★ 自回收：只在**自己建的**临时目录上清理（建不起来这一支直接返回，不挂任何清理）。
func gnrWorktreeAdd(root string) (string, string) {
	tmp, err := os.MkdirTemp("", "gnr-chkmiss-")
	if err != nil {
		return "", "`git worktree add` 起不来 / 失败：临时目录建不了（" + err.Error() + "）"
	}
	wt := filepath.Join(tmp, "co")
	rc, _, errs := gnrGit(root, "worktree", "add", "-f", "-q", wt, "HEAD")
	if rc == gnrGitUnreadable || rc != 0 {
		os.RemoveAll(tmp)
		return "", "`git worktree add` 起不来 / 失败（rc=" + gnrRcOrNone(rc) + "）：" + errs
	}
	return wt, ""
}

// gnrWorktreeRemove —— py `_worktree_remove(root, wt)`：★ 退出必清（移除工作树 + 删临时父目录 · 幂等）。
func gnrWorktreeRemove(root, wt string) {
	gnrGit(root, "worktree", "remove", "--force", wt)
	os.RemoveAll(filepath.Dir(wt))
}

// gnrChkRes —— py `worktree_three_way` 的读数结构。
type gnrChkRes struct {
	applicable       bool
	unreadable       bool
	reason           *string // None ⇒ nil
	paths            []string
	trackedModified  []string
	untracked        []string
	ignored          []string
	red              bool
	noData           bool
	fileSetSize      int
	headTrackedCount *int // None ⇒ nil
	worktree         *string
	worktreeCleaned  bool
}

// gnrSortedCopy —— py `sorted(list)` 的副本（✗ 不改原切片）。
func gnrSortedCopy(l []string) []string {
	out := append([]string{}, l...)
	sort.Strings(out)
	return out
}

// gnrWorktreeThreeWay —— 查缺档**判红**：三合并式（tracked 改动 ∥ untracked ∥ ignored）任一非空 ⇒ red=true。
//
//	· unreadable=True ⇒ 真源不可读 ⇒ 调用方退码 **2**；
//	· noData=True     ⇒ **零执行探针**（空件集）⇒ 非绿、退码 **2**；
//	· applicable=False ⇒ root 根本不是 git 工作树 ⇒ 三合并式不适用（不当绿也不当红）。
//
// ★ 隔离：在 HEAD 建一枚 worktree 当参照，**退出必清**；清理结果写进读数。
func gnrWorktreeThreeWay(root string, paths []string) *gnrChkRes {
	res := &gnrChkRes{
		applicable:      true,
		paths:           append([]string{}, paths...),
		trackedModified: []string{},
		untracked:       []string{},
		ignored:         []string{},
	}
	kind := gnrGitWorktreeKind(root)
	if kind == "no" {
		res.applicable = false
		first := "?"
		if len(paths) > 0 {
			first = paths[0]
		}
		s := "非 git 工作树（`" + first + "` 不受版本控制）⇒ 三合并式不适用"
		res.reason = &s
		return res
	}
	if kind == "unreadable" {
		res.unreadable = true
		s := "git 真源不可读（`.git` 在、但 `git rev-parse` 失败）"
		res.reason = &s
		return res
	}
	rel := []string{}
	for _, p := range paths {
		if p != "" {
			rel = append(rel, p)
		}
	}
	// ① tracked 有改动（--exit-code：0=干净 / 1=有改动；其余 = 判不了）
	rc, out, errs := gnrGit(root, append([]string{"diff", "--name-only", "--exit-code", "HEAD", "--"}, rel...)...)
	if rc == gnrGitUnreadable || (rc != 0 && rc != 1) {
		res.unreadable = true
		s := "`git diff --exit-code HEAD` rc=" + gnrRcOrNone(rc) + "（应为 0/1）：" + errs
		res.reason = &s
		return res
	}
	res.trackedModified = gnrSortedCopy(out)
	// ② untracked（★ 优先 ls-files --others；③ ignored 同族）
	rc, out, errs = gnrGit(root, append([]string{"ls-files", "--others", "--exclude-standard", "--"}, rel...)...)
	if rc == gnrGitUnreadable || rc != 0 {
		res.unreadable = true
		s := "`git ls-files --others` rc=" + gnrRcOrNone(rc) + "：" + errs
		res.reason = &s
		return res
	}
	res.untracked = gnrSortedCopy(out)
	rc, out, errs = gnrGit(root, append([]string{"ls-files", "--others", "--ignored", "--exclude-standard", "--"}, rel...)...)
	if rc == gnrGitUnreadable || rc != 0 {
		res.unreadable = true
		s := "`git ls-files --others --ignored` rc=" + gnrRcOrNone(rc) + "：" + errs
		res.reason = &s
		return res
	}
	res.ignored = gnrSortedCopy(out)
	// 文件集大小（判「零执行探针」：空件集 ⇒ no_data、非绿）
	rc, out, _ = gnrGit(root, append([]string{"ls-files", "--"}, rel...)...)
	tracked := []string{}
	if rc == 0 {
		tracked = out
	}
	seen := map[string]bool{}
	for _, f := range tracked {
		seen[f] = true
	}
	for _, f := range res.untracked {
		seen[f] = true
	}
	res.fileSetSize = len(seen)
	res.red = len(res.trackedModified) > 0 || len(res.untracked) > 0 || len(res.ignored) > 0
	if res.fileSetSize == 0 && !res.red {
		res.noData = true
	}
	// ★ 隔离：HEAD worktree 当参照 · **退出必清**（py try/finally ≡ Go defer）
	wt, werr := gnrWorktreeAdd(root)
	if wt == "" {
		res.unreadable = true
		res.reason = &werr
		return res
	}
	res.worktree = &wt
	defer func() {
		gnrWorktreeRemove(root, wt)
		res.worktreeCleaned = true
	}()
	wtrc, wout, _ := gnrGit(wt, append([]string{"ls-files", "--"}, rel...)...) // HEAD 参照件集（对照读数）
	if wtrc == 0 {
		n := len(wout)
		res.headTrackedCount = &n
	}
	return res
}

// gnrChkMissingRC —— 查缺档退码：不适用/干净 ⇒ 0 · **红 ⇒ 1** · 判不了 / 空件集 ⇒ **2**（绝不 0）。
func gnrChkMissingRC(res *gnrChkRes) int {
	if !res.applicable {
		return 0
	}
	if res.unreadable || res.noData {
		return 2
	}
	if res.red {
		return 1
	}
	return 0
}

// gnrStrPtrIface / gnrIntPtrIface —— 指针 → JSON 值（nil ⇒ untyped nil ⇒ null）。
func gnrStrPtrIface(p *string) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func gnrIntPtrIface(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// gnrStrPtr / gnrIntPtrStr —— 指针 → 人面向 `%s`（nil ⇒ "None"）。
func gnrStrPtr(p *string) string {
	if p == nil {
		return "None"
	}
	return *p
}

func gnrIntPtrStr(p *int) string {
	if p == nil {
		return "None"
	}
	return strconv.Itoa(*p)
}

// gnrJSortKeys —— py `json.dumps(..., sort_keys=True)` 的键序（按码位升序；ASCII 键 ⇒ 字节序等价）。
func gnrJSortKeys(obj gnrJObj) gnrJObj {
	out := append(gnrJObj{}, obj...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].k < out[j].k })
	return out
}

// gnrRunCheckMissing —— py `run_check_missing()`：三合并式查缺档（只读 · 隔离工作树 · 生成前清空输出目录）。
//
//	退码：0 = 不适用/干净 · 1 = **红**（三合并式任一非空）· 2 = **判不了 / 空件集**（fail-closed）。
//	★ 遍历序/键序决定者：三条列表各自 sorted；JSON 顶层键 = obj 插入序（stdout）／`gnrJSortKeys`（写盘件）。
func gnrRunCheckMissing(stdout io.Writer, root string, paths []string, outdir string, asJSON bool) int {
	sel := []string{}
	for _, p := range paths {
		if strings.TrimSpace(p) != "" {
			sel = append(sel, strings.TrimSpace(p))
		}
	}
	if len(sel) == 0 {
		sel = append([]string{}, gnrChkMissingPathsDefault...)
	}
	res := gnrWorktreeThreeWay(root, sel)
	rc := gnrChkMissingRC(res)
	var outdirVal interface{}
	if outdir != "" {
		outdirVal = gnrAbs(outdir)
	}
	obj := gnrJObj{
		{"schema", "zerg/gate-json/v1"},
		{"tool", "check-newfile-registry.py"},
		{"gate", "门76 查缺档（三合并式）"},
		{"repo", root},
		{"paths", sel},
		{"rc", rc},
		{"applicable", res.applicable},
		{"red", res.red},
		{"no_data", res.noData},
		{"unreadable", res.unreadable},
		{"reason", gnrStrPtrIface(res.reason)},
		{"tracked_modified", res.trackedModified},
		{"untracked", res.untracked},
		{"ignored", res.ignored},
		{"file_set_size", res.fileSetSize},
		{"head_tracked_count", gnrIntPtrIface(res.headTrackedCount)},
		{"worktree", gnrStrPtrIface(res.worktree)},
		{"worktree_cleaned", res.worktreeCleaned},
		{"outdir", outdirVal},
	}
	if outdir != "" {
		// ★ 生成前先**清空输出目录**（防旧件残留看着一致）
		absd := gnrAbs(outdir)
		os.RemoveAll(absd)
		os.MkdirAll(absd, 0o755)
		var sb strings.Builder
		gnrJSONValue(&sb, gnrJSortKeys(obj), 0)
		sb.WriteString("\n")
		os.WriteFile(filepath.Join(absd, "查缺档.json"), []byte(sb.String()), 0o644)
		var tb strings.Builder
		tb.WriteString("状态	路径\n")
		for _, leg := range gnrChkLegs(res) {
			for _, it := range leg.items {
				tb.WriteString(leg.name + "	" + it + "\n")
			}
		}
		os.WriteFile(filepath.Join(absd, "查缺档.tsv"), []byte(tb.String()), 0o644)
	}
	if asJSON {
		var sb strings.Builder
		gnrJSONValue(&sb, obj, 0)
		sb.WriteString("\n")
		fmt.Fprint(stdout, sb.String())
		return rc
	}
	fmt.Fprintln(stdout, "门76 查缺档（三合并式 · 只读 · 隔离工作树）")
	fmt.Fprintf(stdout, "  仓根：%s\n", root)
	fmt.Fprintf(stdout, "  判据路径：%s\n", strings.Join(sel, ", "))
	fmt.Fprintf(stdout, "  三合并式：① tracked 改动 %d · ② untracked %d · ③ ignored %d（任一非空即红）\n",
		len(res.trackedModified), len(res.untracked), len(res.ignored))
	for _, leg := range gnrChkLegs(res) {
		for _, it := range leg.items {
			fmt.Fprintf(stdout, "     ✗ %s %s\n", leg.name, it)
		}
	}
	if !res.applicable {
		fmt.Fprintf(stdout, "  ⇒ 三合并式**不适用**（非 git 工作树）⇒ 退码 %d\n", rc)
		return rc
	}
	if res.unreadable {
		fmt.Fprintf(stdout, "  ⇒ 判不了（真源不可读）：%s ⇒ **不给结论（rc=2）**\n", gnrStrPtr(res.reason))
		return rc
	}
	if res.noData {
		fmt.Fprint(stdout, "  ⇒ **零执行探针**（空件集，file_set_size=0）⇒ 无数据标志置位、**非绿（rc=2）**\n")
		return rc
	}
	if res.red {
		fmt.Fprint(stdout, "  ⇒ **红**：工作树相对 HEAD 有分歧（见上）⇒ rc=1\n")
		return rc
	}
	fmt.Fprintf(stdout, "  ⇒ 判到了 —— 三合并式逐条 0 件（file_set_size=%d · HEAD 参照 %s 件 · 工作树已清 %s）\n",
		res.fileSetSize, gnrIntPtrStr(res.headTrackedCount), gnrBoolZh(res.worktreeCleaned, "是", "否"))
	return rc
}

// gnrChkLeg —— 三合并式的三条腿（py 的元组序列 · 保序）。
type gnrChkLeg struct {
	name  string
	items []string
}

func gnrChkLegs(res *gnrChkRes) []gnrChkLeg {
	return []gnrChkLeg{
		{"tracked改动", res.trackedModified},
		{"untracked", res.untracked},
		{"ignored", res.ignored},
	}
}

// ── 面②：现读载入判据件（★ 唯一解释器依赖面 · 见件头 ★★①）────────────────────
type gnrDry struct {
	hitsOK  bool
	hits    map[string]bool
	regOK   bool
	reg     map[string]bool
	rel     string
	relSet  bool
	problem string
}

type gnrDryOut struct {
	Rel     *string  `json:"rel"`
	HitsOK  bool     `json:"hits_ok"`
	Hits    []string `json:"hits"`
	RegOK   bool     `json:"reg_ok"`
	Reg     []string `json:"reg"`
	Problem *string  `json:"problem"`
}

// gnrDryGoJudge —— 面② 的**进程内判据件提供器**：已 Go 真搬的判据件按「仓根相对件名」查表，
// 命中即**进程内**取数（✗ 不起 python3）；未搬的仍走 gnrRunDryDriver 回退（解释器依赖在
// 现行判据集下不可达 —— 门⑩ 是步骤表里**唯一**带「干跑」标记的判据件，且它已搬 Go）。
//
// ★ 本件的自指防线（见件头 (b)③）：门⑩ 判据件的件名**不得**以连续字面落地（否则本件被
// 门⑩ 的 `dry.?run|DryRun` 扫成未登记站点 ⇒ 真仓 R1 假红）⇒ 表键**拼串**构造。
var gnrDryGoJudge = map[string]func(root string) gnrDry{
	"scripts/gates/check-" + "dry" + "run" + "-semantics.py": gnrDryInProc,
}

// gnrDryGoCLIKey —— CLI 形 `zerg gate check <键>` → **进程内**判据件的映射。2026-10-07 检查点4 起
// 步骤表直调 py 改走 CLI：门⑩ 那条「干跑」步的命令**不再是** `python3 …check-<…>.py`，而是
// `zerg gate check check-<…>` ⇒ 解析器须按**键名**落到同一已搬 Go 的判据件（✗ 不因 py 形消失而判不了）。
// ★ 键与 `gnrDryGoJudge` 的 `.py` 名同源（同一判据件的两种指法）；键字面按件头 (b)③ **拼串**构造，
// 不落连续的判据件名词干（否则本件被门⑩ 的扫描口径扫成未登记站点 ⇒ 真仓 R1 假红）。
var gnrDryGoCLIKey = map[string]func(root string) gnrDry{
	"check-" + "dry" + "run" + "-semantics": gnrDryInProc,
}

// gnrDryCLIKeyRE —— 从步骤行现读 CLI 形 `zerg gate check <键>` 的键名（正则源串不含「干跑令牌」字面）。
var gnrDryCLIKeyRE = regexp.MustCompile(`\bzerg\s+gate\s+check\s+([A-Za-z0-9_-]+)`)

// gnrDryCLIKey —— 抽键名（不中 ⇒ 空串）。
func gnrDryCLIKey(ln string) string {
	if m := gnrDryCLIKeyRE.FindStringSubmatch(ln); m != nil {
		return m[1]
	}
	return ""
}

// gnrDryInProc —— 进程内调用已搬 Go 的判据件（当前唯一项 = 门⑩ 干跑语义）。
// 取数面与 py 驱动逐字对齐：`hits` = `scan_hits(root)` 四元组里**全部字符串**
// （files ∪ sites ∪ refs）· `rel` = 判据件的登记表相对路径 · `reg` = 登记表 `件[].file` 集合。
func gnrDryInProc(root string) gnrDry {
	hits, errText := drsScan(root)
	if hits == nil {
		return gnrDry{problem: "判据件扫描函数不可判：" + errText}
	}
	union := map[string]bool{}
	for f := range hits.files {
		union[f] = true
	}
	for f := range hits.sites {
		union[f] = true
	}
	for f := range hits.refs {
		union[f] = true
	}
	d := gnrDry{hitsOK: true, hits: union, rel: drsRegistryRel, relSet: true}
	reg, ok := gnrDryLoadReg(root, drsRegistryRel)
	if !ok {
		d.problem = "登记表不在盘：" + drsRegistryRel
		return d
	}
	if len(reg) == 0 {
		return d // py：`reg` 为空 ⇒ `reg_ok` 保持 False（仅非空才置真）
	}
	d.regOK = true
	d.reg = reg
	return d
}

// gnrDryLoadReg —— 读 门⑩ 登记表的 `件[].file` 集合（py 驱动 `reg = set(...)` 段逐字对齐）。
func gnrDryLoadReg(root, rel string) (map[string]bool, bool) {
	text, ok := gnrReadText(filepath.Join(root, filepath.FromSlash(rel)))
	if !ok {
		return nil, false
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return nil, false
	}
	reg := map[string]bool{}
	if arr, ok := data["件"].([]interface{}); ok {
		for _, e := range arr {
			m, ok := e.(map[string]interface{})
			if !ok {
				continue
			}
			if s, ok := m["file"].(string); ok && s != "" {
				reg[s] = true
			}
		}
	}
	return reg, true
}

// gnrLoadDryJudge —— py `load_dry_module()` + `collect_paths()` + `dry_registry()` 的合体。
func gnrLoadDryJudge(root string) gnrDry {
	_, text, ok := gnrLoadSteps(root)
	if !ok {
		return gnrDry{problem: "步骤表读不到"}
	}
	for _, ln := range gnrSplitLines(text) {
		if !strings.Contains(ln, gnrAddStep) || !strings.Contains(ln, gnrStepMark) {
			continue
		}
		m := gnrJudgeRECompiled.FindString(ln)
		if m == "" {
			// ★ 步骤表直调 py 已改 CLI 形 `zerg gate check <键>`（2026-10-07 检查点4）：行里**没有**
			//   `.py` 落点时，按**键名**落到已搬 Go 的进程内判据件（✗ 不因 py 形消失而判不了）。
			//   键名不中映射表 ⇒ 落到本函数末的 fail-closed（✗ 不静默跳过）。
			if key := gnrDryCLIKey(ln); key != "" {
				if fn, ok := gnrDryGoCLIKey[key]; ok {
					return fn(root)
				}
			}
			continue
		}
		cand := filepath.Join(root, m)
		if !gnrIsFile(cand) {
			continue
		}
		// ★ 面② 去 python3：已搬 Go 的判据件走**进程内**（✗ 不 spawn 解释器）。
		if fn, ok := gnrDryGoJudge[m]; ok {
			return fn(root)
		}
		out, errText := gnrRunDryDriver(cand, root, m)
		if out == nil {
			msg := "判据件载入不了：" + m
			if errText != "" {
				msg = msg + "（" + errText + "）"
			}
			return gnrDry{problem: msg}
		}
		d := gnrDry{}
		if out.Rel != nil {
			d.rel, d.relSet = *out.Rel, true
		}
		if out.HitsOK {
			d.hitsOK = true
			d.hits = map[string]bool{}
			for _, h := range out.Hits {
				d.hits[h] = true
			}
		}
		if out.RegOK {
			d.regOK = true
			d.reg = map[string]bool{}
			for _, r := range out.Reg {
				d.reg[r] = true
			}
		}
		if out.Problem != nil {
			d.problem = *out.Problem
		}
		return d
	}
	return gnrDry{problem: "步骤表里找不到「" + gnrStepMark + "」那条步的判据件（.py 或 `zerg gate check <键>` 两形皆不中）"}
}

func gnrRunDryDriver(cand, root, relArg string) (*gnrDryOut, string) {
	cmd := exec.Command("python3", "-c", gnrDryDriverPy, cand, root, relArg)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, strings.TrimSpace(errb.String())
	}
	var o gnrDryOut
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &o); err != nil {
		return nil, strings.TrimSpace(errb.String())
	}
	return &o, ""
}

// ── 主判据 ────────────────────────────────────────────────────────────────
type gnrNJ struct{ face, item, why string }

type gnrCounts struct {
	R1, R2, R3, R5 int
	R7             int
	R4             *int
	R6             *int
}

type gnrRes struct {
	counts     *gnrCounts
	ledgerCnt  *int
	gate1Cnt   *int
	diff       []string
	diffSet    bool
	problems   []string
	notJudged  []gnrNJ
	roster     []string
	newFiles   []string
	items      map[string][]string
	weak       []string
	fourth     []string
	dryNA      []string
	dryHits    map[string]bool
	dryReg     map[string]bool
	dryHitsOK  bool
	ledPath    string
	ledRows    *int
	wording    *bool
	wLines     []string
	marks      map[string]string
	faceJudged map[string]bool
	// ⑨ `.sh` 门面清册：盘上 `scripts/gates/*.sh` ⇄ `real-gates.sh` 的 `SH_GATES`（双 nil 语义 = 判不了）。
	shDisk    []string
	shRoster  []string
	shMissing []string
	shExtra   []string
	// ★ 甲/乙 分档：分母 = 非空扫描面 ∖ 在盘基线 ⇒ 差集为空是正常态（rc=0）；
	//   扫描面自身为空（零执行探针）⇒ denScanEmpty ⇒ 仍判不了（rc=2）。
	denFromBaseline bool
	denScanEmpty    bool
}

// gnrBaselineFingerprint —— ③ B1-3：现读件集的稳定指纹（排序逐行 · 纯内容派生 · 禁 mtime / 禁读盘序）。
func gnrBaselineFingerprint(face map[string]bool) string {
	h := sha256.New()
	names := []string{}
	for k := range face {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, f := range names {
		h.Write([]byte(f))
		h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func gnrIsHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// gnrParseBaselineHeader —— 基线 v2 首行注释头（件数 / 指纹）。无头（v1 旧件 / 合成夹具）⇒ ok=false 照读。
func gnrParseBaselineHeader(text string) (int, string, bool) {
	lines := gnrSplitLines(text)
	if len(lines) > 3 {
		lines = lines[:3]
	}
	for _, ln := range lines {
		if !strings.HasPrefix(ln, "#") {
			continue
		}
		i := strings.Index(ln, "件数=")
		if i < 0 {
			continue
		}
		rest := ln[i+len("件数="):]
		j := 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			j++
		}
		if j == 0 {
			continue
		}
		n, _ := strconv.Atoi(rest[:j])
		fp := ""
		if k := strings.Index(rest, "指纹="); k >= 0 {
			tail := rest[k+len("指纹="):]
			m := 0
			for m < len(tail) && gnrIsHexDigit(tail[m]) {
				m++
			}
			fp = tail[:m]
		}
		return n, fp, true
	}
	return 0, "", false
}

// gnrScanLedgerTwice —— ③ 双跑 cmp 自证（确定性闸）：两次扫描不等 ⇒ 判不了（禁部分扫描写整树）。
func gnrScanLedgerTwice(root string) (map[string]bool, bool) {
	a := gnrScanLedgerFace(root)
	b := gnrScanLedgerFace(root)
	if a == nil || b == nil || len(a) != len(b) {
		return a, false
	}
	for k := range a {
		if !b[k] {
			return a, false
		}
	}
	return a, true
}

// gnrDumpBaseline —— ③ B1-3 冻结基线生成口：双跑 cmp 自证 + v2 头（件数+指纹）+ 只许下调 + 原子写（temp+fsync+rename）。
func gnrDumpBaseline(stdout, stderr io.Writer, root, target string, yes bool) int {
	face, det := gnrScanLedgerTwice(root)
	if face == nil {
		fmt.Fprintf(stderr, "✗ 扫面不在盘：%s ⇒ 不写基线\n", filepath.Join(root, "scripts"))
		return 2
	}
	if !det {
		fmt.Fprintln(stderr, "✗ 双跑 cmp 不一致（扫描非确定）⇒ 拒写（禁部分扫描写整树）")
		return 2
	}
	fp := gnrBaselineFingerprint(face)
	oldCnt, hasOld := -1, false
	if gnrIsFile(target) {
		if txt, ok := gnrReadText(target); ok {
			if c, _, ok2 := gnrParseBaselineHeader(txt); ok2 {
				oldCnt, hasOld = c, true
			}
		}
	}
	if !yes {
		fmt.Fprintln(stdout, "── 冻结基线生成档（**干跑** · 未写件）")
		fmt.Fprintf(stdout, "   目标件 : %s\n", target)
		fmt.Fprintf(stdout, "   将写   : %d 件（现读台账口径件集逐行 · 首行注释 + 逐件一行）\n", len(face))
		if hasOld && len(face) > oldCnt {
			fmt.Fprintf(stderr, "   ✗ 基线只许下调：新 %d > 旧 %d ⇒ 加 `--yes` 也拒写\n", len(face), oldCnt)
			return 2
		}
		fmt.Fprintln(stdout, "   ⇒ **干跑未写**：加 `--yes` 才落（本件默认不写件）")
		return 0
	}
	if hasOld && len(face) > oldCnt {
		fmt.Fprintf(stderr, "✗ 基线只许下调：新 %d 件 > 旧 %d 件 ⇒ 拒写（fail-closed）\n", len(face), oldCnt)
		return 2
	}
	dir := filepath.Dir(gnrAbs(target))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "✗ 基线写不了：%s\n", err)
		return 2
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# 门76 冻结基线 v2 件数=%d 指纹=%s（现读件集 · 生成件 · 禁手维）\n", len(face), fp))
	names := []string{}
	for k := range face {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, f := range names {
		sb.WriteString(f + "\n")
	}
	tmp, err := os.CreateTemp(dir, ".gnr-base-")
	if err != nil {
		fmt.Fprintf(stderr, "✗ 基线写不了：%s\n", err)
		return 2
	}
	tmpName := tmp.Name()
	if _, werr := tmp.WriteString(sb.String()); werr == nil {
		err = tmp.Sync()
	} else {
		err = werr
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmpName, target)
	}
	if err != nil {
		os.Remove(tmpName)
		fmt.Fprintf(stderr, "✗ 基线写不了：%s\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "基线已写：%s（%d 件）\n", target, len(face))
	return 0
}

// gnrRunSync —— py `run_sync()`：④ B1-4 补齐档（默认干跑 · 只写纯派生面 · 面③⑤⑧ 只报「该跑哪条正门」）。
func gnrRunSync(stdout, stderr io.Writer, root string, yes bool, baseline string) int {
	face, det := gnrScanLedgerTwice(root)
	if face == nil {
		fmt.Fprintln(stderr, "✗ 扫面不在盘 ⇒ 不给结论（rc=2）")
		return 2
	}
	if !det {
		fmt.Fprintln(stderr, "✗ 双跑 cmp 不一致（扫描非确定）⇒ 拒写（禁部分扫描写整树）")
		return 2
	}
	base := map[string]bool{}
	if baseline != "" && gnrIsFile(baseline) {
		txt, ok := gnrReadText(baseline)
		if !ok {
			fmt.Fprintf(stderr, "✗ 基线在盘但读不出（%s）⇒ fail-closed 拒写 rc=2\n", baseline)
			return 2
		}
		for _, ln := range gnrSplitLines(txt) {
			s := strings.TrimSpace(ln)
			if s != "" && !strings.HasPrefix(s, "#") {
				base[s] = true
			}
		}
	}
	newf := []string{}
	for k := range face {
		if !base[k] {
			newf = append(newf, k)
		}
	}
	sort.Strings(newf)
	marks, _ := gnrLoadMarks(root)
	miss := []string{}
	for _, f := range newf {
		if marks == nil {
			miss = append(miss, f)
			continue
		}
		if _, ok := marks[f]; !ok {
			miss = append(miss, f)
		}
	}
	mode := "干跑 · 未写件"
	if yes {
		mode = "真写"
	}
	fmt.Fprintf(stdout, "── 补齐档 `--sync`（%s）──\n", mode)
	fmt.Fprintf(stdout, "   面① 公开标记表：补占位行 %d 件\n", len(miss))
	fmt.Fprintln(stdout, "   面④ 脚本台账：该跑正门 `zerg script inventory sync --docs-root <Zerg-内部文档> --yes`（本档不代跑）")
	fmt.Fprintln(stdout, "   面⑦ 判据注册表：该跑正门 `zerg gate check check-judge-registry --emit`（本档不代跑）")
	fmt.Fprintln(stdout, "   面③ 门①两张真源：该跑正门 = 人加 add_step 或 whitelist.scripts（要拍档 · 本档不替人拍）")
	fmt.Fprintln(stdout, "   面⑤ 两器排除表：该跑正门 = public=no 才要，EXCLUDES + DROP_EXACT 两器同批（本档不代改）")
	fmt.Fprintln(stdout, "   面⑧ 批准件 scope：该跑正门 = `zerg approve new --tool dev_edit`（人签 · 非交互拒）")
	fmt.Fprintln(stdout, "   面⑨ .sh 门面清册：该跑正门 = 同批把新 `.sh` 门件登记进 `scripts/gates/real-gates.sh` 的 `SH_GATES`（人拍 · 本档不代改）")
	if !yes {
		fmt.Fprintln(stdout, "   ⇒ **干跑未写**：加 `--yes` 才落（只写纯派生面；面③⑤⑧ 只报该跑哪条正门）")
		return 0
	}
	if len(miss) == 0 {
		fmt.Fprintln(stdout, "   ⇒ 计划件 0 ⇒ 零写入")
		return 0
	}
	p := filepath.Join(root, gnrMarksRel)
	dir := filepath.Dir(gnrAbs(p))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "✗ 写不了：%s\n", err)
		return 2
	}
	var body strings.Builder
	if gnrIsFile(p) {
		if txt, ok := gnrReadText(p); ok {
			body.WriteString(txt)
		}
	}
	for _, f := range miss {
		body.WriteString(f + "\tno\t待补理由（--sync 占位 · 人补）\n")
	}
	tmp, err := os.CreateTemp(dir, ".gnr-marks-")
	if err != nil {
		fmt.Fprintf(stderr, "✗ 写不了：%s\n", err)
		return 2
	}
	tmpName := tmp.Name()
	if _, werr := tmp.WriteString(body.String()); werr == nil {
		err = tmp.Sync()
	} else {
		err = werr
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmpName, p)
	}
	if err != nil {
		os.Remove(tmpName)
		fmt.Fprintf(stderr, "✗ 写不了：%s\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "   已写 面① 占位行 %d 件（原子写 · temp+fsync+rename）\n", len(miss))
	return 0
}

func gnrAudit(root string, newFiles []string, newSet bool, inv string, invSet bool, baseline string) *gnrRes {
	res := &gnrRes{items: map[string][]string{}, problems: []string{}, notJudged: []gnrNJ{}}
	ledgerFace := gnrScanLedgerFace(root)
	if ledgerFace == nil {
		res.problems = append(res.problems, "扫面 0 件：`scripts` 不在盘 ⇒ 空转 = 假绿")
		return res
	}
	roster := []string{}
	for k := range ledgerFace {
		roster = append(roster, k)
	}
	sort.Strings(roster)
	res.roster = roster

	g1 := gnrScanGate1Face(root)
	n := len(ledgerFace)
	res.ledgerCnt = &n
	if g1.set {
		m := len(g1.files)
		res.gate1Cnt = &m
	} else if g1.rawData == nil {
		res.gate1Cnt = nil
	} else {
		res.gate1Cnt = nil
	}
	if g1.set {
		diff := []string{}
		for k := range ledgerFace {
			if !g1.files[k] {
				diff = append(diff, k)
			}
		}
		for k := range g1.files {
			if !ledgerFace[k] {
				diff = append(diff, k)
			}
		}
		sort.Strings(diff)
		res.diff = diff
		res.diffSet = true
	}

	// 分母
	if !newSet {
		if baseline == "" || !gnrIsFile(baseline) {
			shown := baseline
			if shown == "" {
				shown = gnrDefaultBaseline()
			}
			res.problems = append(res.problems, "分母判不了：冻结基线不在盘（"+shown+"）⇒ 不给结论；先跑 `--dump-baseline <仓外路径>`")
			return res
		}
		base := map[string]bool{}
		btxt, berr := os.ReadFile(baseline)
		if berr != nil { // ★ ① 基线在盘但读不出 ⇒ fail-closed（py 未捕获 OSError 曾静默绿）
			res.problems = append(res.problems, fmt.Sprintf(
				"分母判不了：冻结基线在盘但读不出（%s：%s）⇒ fail-closed 不给结论（rc=2）", baseline, gnrPyErrText(berr, baseline)))
			return res
		}
		txt := strings.ToValidUTF8(string(btxt), "\uFFFD")
		bcnt, _, bHasHdr := gnrParseBaselineHeader(txt) // ★ ③ B1-3：v2 头（件数+指纹）
		for _, ln := range gnrSplitLines(txt) {
			s := strings.TrimSpace(ln)
			if s != "" && !strings.HasPrefix(s, "#") {
				base[s] = true
			}
		}
		if bHasHdr && len(ledgerFace) < bcnt { // ★ ③ 过期即红（件在减少 ⇒ 基线已不描述本树）
			res.problems = append(res.problems, fmt.Sprintf(
				"基线过期（v2 头声明 %d 件 ⟷ 现读扫面 %d 件 ⇒ 件在减少）⇒ 判红 rc=2；先跑 `--dump-baseline <仓外路径> --yes` 重打",
				bcnt, len(ledgerFace)))
		}
		nf := []string{}
		for k := range ledgerFace {
			if !base[k] {
				nf = append(nf, k)
			}
		}
		sort.Strings(nf)
		res.newFiles = nf
		// ★ 甲/乙 分档（分母 = 非空扫描面 ∖ 在盘基线）。
		res.denFromBaseline = true
		res.denScanEmpty = len(ledgerFace) == 0
	} else {
		seen := map[string]bool{}
		nf := []string{}
		for _, f := range newFiles {
			if !seen[f] {
				seen[f] = true
				nf = append(nf, f)
			}
		}
		sort.Strings(nf)
		res.newFiles = nf
		res.denFromBaseline = false
		res.denScanEmpty = len(ledgerFace) == 0
	}

	marks, _ := gnrLoadMarks(root)
	dry := gnrLoadDryJudge(root)
	dryErr := dry.problem
	var dryHits, dryReg map[string]bool
	dryRel := ""
	dryRelSet := false
	dryProblem := dryErr
	if dry.hitsOK && dry.regOK {
		dryHits, dryReg = dry.hits, dry.reg
		dryRel, dryRelSet = dry.rel, dry.relSet
		dryProblem = ""
	} else if dry.hitsOK {
		dryHits = dry.hits
		dryRel, dryRelSet = dry.rel, dry.relSet
		if dry.problem == "" {
			dryProblem = ""
		} else {
			dryProblem = dry.problem
		}
	}
	stepLines, stepText, stepOK := gnrLoadSteps(root)
	wlItems, wlProblems := gnrLoadWhitelistScripts(root)
	excludes := gnrLoadExcludes(root)
	drops := gnrLoadDropExact(root)
	ledRows, ledPath, _ := gnrLoadLedgerRows(root, inv, invSet)
	fixed, wLines, wordingOK := gnrLoadWording(root)
	shDisk := gnrScanGatesSh(root)
	shRoster := gnrLoadShGates(root)
	shMissing, shExtra := []string{}, []string{}
	if shDisk != nil && shRoster != nil {
		shMissing = gnrSortedSetDiff(shRoster, shDisk) // py `sorted(sh_roster - sh_disk)`（清册点名 · 盘上无）
		shExtra = gnrSortedSetDiff(shDisk, shRoster)   // py `sorted(sh_disk - sh_roster)`（盘上有 · 未进清册）
	}

	// 机器面：逐面「真源读到了没有」（面自身判不了 ⇒ BLOCKED；读到但整跑无结论 ⇒ UNKNOWN）
	res.faceJudged = map[string]bool{
		"①公开标记表":   marks != nil,
		"②干跑登记":    dryHits != nil && dryReg != nil,
		"③门①两张真源":  stepLines != nil && wlItems != nil,
		"④脚本台账":    ledRows != nil,
		"⑤两器排除表":   excludes != nil && drops != nil,
		"⑥失败文案面":   fixed != nil,
		"⑨.sh门面清册": shDisk != nil && shRoster != nil,
	}

	type faceCheck struct {
		name string
		ok   bool
		why  string
	}
	why2 := dryProblem
	if dryRelSet && dryRel != "" {
		why2 = dryRel
	} else if dryProblem == "" {
		why2 = "门⑩ 登记表"
	}
	for _, fc := range []faceCheck{
		{"①公开标记表", marks != nil, gnrMarksRel},
		{"②干跑登记", dry.hitsOK && dry.regOK, why2},
		{"③门①两张真源", stepOK && wlItems != nil, gnrStepsRel},
		{"⑤两器排除表", excludes != nil && drops != nil, gnrPubRel},
		{"⑨.sh门面清册", shDisk != nil && shRoster != nil, gnrShGatesRel},
	} {
		if !fc.ok {
			res.problems = append(res.problems, "面"+fc.name+" 判不了："+fc.why+" 读不到 / 解析出 0 条（真源读不全 ⇒ 绿就是假绿）")
		}
	}
	res.problems = append(res.problems, wlProblems...)
	if ledRows == nil {
		item := ledPath
		if item == "" {
			item = "（同级 " + gnrLedgerSib + " 下无 清单-脚本现状-*.tsv）"
		}
		res.notJudged = append(res.notJudged, gnrNJ{"④台账面", item,
			"仓外真源 · 本门**不写**（唯一写入口 `zerg script inventory sync` = 危险档 D2 写面 · 需人签）"})
	}
	if !wordingOK {
		res.notJudged = append(res.notJudged, gnrNJ{"⑥文案面", gnrCovJudgeRel, "判据件不在盘 / 找不着处置文案"})
	}

	r1, r2, r3, r5 := []string{}, []string{}, []string{}, []string{}
	weak, fourth, dryNA := []string{}, []string{}, []string{}
	wlPaths := map[string]bool{}
	for _, it := range wlItems {
		if s, ok := it.path.(string); ok {
			wlPaths[s] = true
		}
	}
	for _, f := range res.newFiles {
		// ①
		if marks != nil {
			if _, ok := marks[f]; !ok {
				r1 = append(r1, f)
			}
		}
		// ②
		if dryHits != nil && dryReg != nil {
			if dryHits[f] && !dryReg[f] {
				r2 = append(r2, f)
			} else if !dryHits[f] {
				dryNA = append(dryNA, f)
			}
		}
		// ③
		if stepLines != nil && wlItems != nil {
			strong, weakm := false, false
			for _, ln := range stepLines {
				if strings.Contains(ln, f) {
					strong = true
				}
				if strings.Contains(ln, filepath.Base(f)) {
					weakm = true
				}
			}
			if wlPaths[f] {
				// pass
			} else if strong {
				// pass
			} else if weakm {
				weak = append(weak, f)
			} else if stepText != "" && strings.Contains(stepText, filepath.Base(f)) {
				fourth = append(fourth, f)
			} else {
				r3 = append(r3, f)
			}
		}
		// ⑤
		if marks != nil && excludes != nil && drops != nil {
			if marks[f] == "no" && !(excludes[f] && drops[f]) {
				r5 = append(r5, f)
			}
		}
	}
	var r4 []string
	var r4n *int
	if ledRows != nil {
		r4 = []string{}
		for _, f := range res.newFiles {
			if !ledRows[f] {
				r4 = append(r4, f)
			}
		}
		m := len(r4)
		r4n = &m
	}
	var r6n *int
	if wordingOK {
		v := 0
		if !*fixed {
			v = 1
		}
		r6n = &v
	}
	res.counts = &gnrCounts{R1: len(r1), R2: len(r2), R3: len(r3), R4: r4n, R5: len(r5), R6: r6n,
		R7: len(shMissing) + len(shExtra)}
	res.items["R1"], res.items["R2"], res.items["R3"] = r1, r2, r3
	if r4 == nil {
		r4 = []string{}
	}
	res.items["R4"], res.items["R5"] = r4, r5
	res.items["R7"] = append(append([]string{}, shMissing...), shExtra...)
	if shDisk == nil {
		res.shDisk = nil // py `None if sh_disk is None`
	} else {
		res.shDisk = gnrSortedKeys(shDisk)
	}
	if shRoster == nil {
		res.shRoster = nil // py `None if sh_roster is None`
	} else {
		res.shRoster = gnrSortedKeys(shRoster)
	}
	res.shMissing, res.shExtra = shMissing, shExtra
	res.weak, res.fourth, res.dryNA = weak, fourth, dryNA
	res.dryHits, res.dryReg = dryHits, dryReg
	res.dryHitsOK = dryHits != nil
	if ledRows != nil {
		m := len(ledRows)
		res.ledRows = &m
	}
	res.ledPath = ledPath
	res.wording, res.wLines = fixed, wLines
	res.marks = marks
	return res
}

func gnrDefaultBaseline() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "~"
	}
	return filepath.Join(home, ".zerg", "newfile-registry-baseline.tsv")
}

// ── 面清单 / 面记录（**跑前先生成 expected_faces** · 缺面必须合成 · 禁静默跳过）────────
// gnrExpectedFaces —— py `expected_faces()`：★ **跑前先生成** expected_faces 清单（唯一真源 = gnrFaces）。
func gnrExpectedFaces() []string { return append([]string{}, gnrFaces...) }

// gnrReasonFor —— py `_reason_for(state)`：五态 → 原因码（闭集见模块常量）。
func gnrReasonFor(state string) string {
	switch state {
	case "PASS":
		return gnrReasonOK
	case "FAIL":
		return gnrReasonMissingEntry
	case "BLOCKED":
		return gnrReasonSourceUnreadable
	case "REPORT":
		return gnrReasonReportOnly
	}
	return gnrReasonUnjudged
}

// gnrJLookup —— gnrJObj 里按 key 取 string 值（py `f in states` 的存在性 + 取值）。
func gnrJLookup(obj gnrJObj, k string) (string, bool) {
	for _, f := range obj {
		if f.k == k {
			if s, ok := f.v.(string); ok {
				return s, true
			}
			return "", true
		}
	}
	return "", false
}

// gnrJRecField —— 面记录（gnrJObj）里取某字段串值。
func gnrJRecField(v interface{}, k string) string {
	if o, ok := v.(gnrJObj); ok {
		if s, ok2 := gnrJLookup(o, k); ok2 {
			return s
		}
	}
	return ""
}

// gnrJRecBool —— 面记录（gnrJObj）里取某字段 bool 值（缺 ⇒ false）。
func gnrJRecBool(v interface{}, k string) bool {
	if o, ok := v.(gnrJObj); ok {
		for _, f := range o {
			if f.k == k {
				b, _ := f.v.(bool)
				return b
			}
		}
	}
	return false
}

// gnrRecValue —— 面记录表（gnrJObj）里按面名取该面的记录（缺 ⇒ nil）。
func gnrRecValue(recs gnrJObj, k string) interface{} {
	for _, f := range recs {
		if f.k == k {
			return f.v
		}
	}
	return nil
}

// gnrB2i —— bool → 0/1（py 自检里 `1 if cond else 0` 的 got 形态）。
func gnrB2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// gnrFaceRecords —— py `face_records(res, states, faces)`：逐面**记录**（state / reason_code / decided）。
//
//	· host-bound 面（gnrHostBoundFaces）⇒ HOST-REQUIRED · SKIPPED-HOST-BOUND · decided=false（不计 pass/fail）；
//	· 已产出记录的面 ⇒ 照 gnrBuildFaceStates 的态 + 原因码；
//	· **未产出记录的面** ⇒ 合成 UNKNOWN · MISSING_FACE · decided=false（退码由 gnrVerdict 钉 2）。
func gnrFaceRecords(res *gnrRes, states gnrJObj, faces []string) gnrJObj {
	if faces == nil {
		faces = gnrExpectedFaces()
	}
	if states == nil {
		states = gnrBuildFaceStates(res)
	}
	out := gnrJObj{}
	for _, f := range faces {
		if gnrContains(gnrHostBoundFaces, f) {
			out = append(out, gnrJField{f, gnrJObj{
				{"state", "HOST-REQUIRED"}, {"reason_code", gnrReasonHostBound}, {"decided", false}}})
			continue
		}
		if st, ok := gnrJLookup(states, f); ok {
			decided := st == "PASS" || st == "FAIL" || st == "REPORT"
			out = append(out, gnrJField{f, gnrJObj{
				{"state", st}, {"reason_code", gnrReasonFor(st)}, {"decided", decided}}})
			continue
		}
		out = append(out, gnrJField{f, gnrJObj{
			{"state", "UNKNOWN"}, {"reason_code", gnrReasonMissingFace}, {"decided", false}}})
	}
	return out
}

// gnrMissingFaces —— py `missing_faces(res, faces)`：未产出记录的面（reason_code=MISSING_FACE）名单 · sorted。
func gnrMissingFaces(res *gnrRes, faces []string) []string {
	recs := gnrFaceRecords(res, nil, faces)
	out := []string{}
	for _, fld := range recs {
		if gnrJRecField(fld.v, "reason_code") == gnrReasonMissingFace {
			out = append(out, fld.k)
		}
	}
	sort.Strings(out)
	return out
}

// ── 出口判决（**唯一真源** · 人面向与机器面共用）────────────────────────────
// gnrVerdict —— 返回 (rc, bad, unjudged)。0 = 判到了 · 1 = 缺登记（仅 --strict）· 2 = 不给结论。
//
//	与 `gnrRun` 的出口分支**同一套数**（`bad` / `unjudged` 只由 res 派生、无副作用）——
//	改一处必须两处同源，禁各算一套（`--json` 短路不得改变退码）。
//	★ **缺面（MISSING_FACE）⇒ 退码钉 2**（fail-closed：缺面不许当齐）。faces=nil ⇒ 用 gnrExpectedFaces()。
func gnrVerdict(res *gnrRes, strict bool, faces []string) (int, int, int) {
	if res.counts == nil {
		return 2, 0, 0
	}
	c := res.counts
	bad := c.R1 + c.R2 + c.R3 + c.R5 + c.R7 // R4 / R6 = 单列旁栏（写死边界：不进退码）；R7 = ⑨.sh门面清册
	unjudged := len(res.notJudged) + len(res.problems)
	if c.R4 == nil {
		unjudged++
	}
	denOK := res.denFromBaseline && !res.denScanEmpty // ★ 甲/乙 分档
	if unjudged > 0 || (len(res.newFiles) == 0 && !denOK) {
		return 2, bad, unjudged
	}
	if len(gnrMissingFaces(res, faces)) > 0 { // ★ 缺面 ⇒ 退码钉 2
		return 2, bad, unjudged
	}
	if bad > 0 {
		if strict {
			return 1, bad, unjudged
		}
		return 0, bad, unjudged
	}
	return 0, bad, unjudged
}

// gnrRun —— py `run()`：输出 + 退码。strict=阻断档。
func gnrRun(stdout io.Writer, root string, newFiles []string, newSet bool, inv string, invSet bool, strict bool, baseline string, baselineSet bool) int {
	res := gnrAudit(root, newFiles, newSet, inv, invSet, baseline)
	rc, bad, unjudged := gnrVerdict(res, strict, nil) // 出口判决唯一真源（同 `gnrRunJSON`）
	fmt.Fprintln(stdout, "门76 新件登记完整性（件级 × 六张登记表 ＋ .sh 门面清册 · **只报告**）")
	fmt.Fprintf(stdout, "  仓根：%s\n", root)
	if res.ledgerCnt != nil {
		fmt.Fprintf(stdout, "  口径差栏：台账口径 %s 件 · 门①候选面 %s 件 · 对差 %s 件（参照读数 · 不并入判红）\n",
			gnrPyStr(float64(*res.ledgerCnt)), gnrCountStr(res.gate1Cnt), gnrDiffStr(res))
	} else {
		fmt.Fprintf(stdout, "  口径差栏：台账口径 %s 件 · 门①候选面 %s 件 · 对差 %s 件（参照读数 · 不并入判红）\n",
			"None", "None", "判不了")
	}
	for i, f := range res.diff {
		if i >= 40 {
			break
		}
		fmt.Fprintf(stdout, "     · 口径差 %s\n", f)
	}
	if res.counts == nil {
		for _, p := range res.problems {
			fmt.Fprintf(stdout, "  ✗ %s\n", p)
		}
		fmt.Fprintln(stdout, "  判据结论：**判不了 ⇒ 不给结论（rc=2）**")
		return rc
	}
	c := res.counts
	fmt.Fprintf(stdout, "  分母（现读新件 ∖ 冻结基线）：%d 件\n", len(res.newFiles))
	for i, f := range res.newFiles {
		if i >= 40 {
			break
		}
		fmt.Fprintf(stdout, "     - %s\n", f)
	}
	fmt.Fprintf(stdout, "  面① 公开标记表缺登记：%d\n", c.R1)
	for _, f := range res.items["R1"] {
		fmt.Fprintf(stdout, "     ✗ %s  ← 表里无此行（补 `scripts/公开标记.tsv` 一件一行 · public=no 也要写理由）\n", f)
	}
	fmt.Fprintf(stdout, "  面② 干跑登记缺件：%d（口径 = 门⑩ 判据件自己的扫描函数（现读载入）· 不适用 %d 件）\n", c.R2, len(res.dryNA))
	for _, f := range res.items["R2"] {
		fmt.Fprintf(stdout, "     ✗ %s  ← 正文命中干跑口径却未登记（补该登记表的 件[].file）\n", f)
	}
	fmt.Fprintf(stdout, "  面③ 门① 两张真源缺件：%d（弱挂载 %d 件 · 通用语法步兜住 %d 件 —— 均不判红）\n", c.R3, len(res.weak), len(res.fourth))
	for _, f := range res.items["R3"] {
		fmt.Fprintf(stdout, "     ✗ %s  ← 既不在 add_step 点名、也不在 whitelist.scripts（补 reason+date 白名单档）\n", f)
	}
	for _, f := range res.weak {
		fmt.Fprintf(stdout, "     · 弱 %s  ← 仅 basename 出现在 add_step 行（疑点 · 不判红）\n", f)
	}
	if c.R4 == nil {
		fmt.Fprintf(stdout, "  面④ 脚本台账：**判不了**（真源 %s）\n", gnrPathOr(res.ledPath))
	} else {
		fmt.Fprintf(stdout, "  面④ 脚本台账缺登记：%d（台账 %s 行 · **单列旁栏 · 不进退码** · 写入口 = 仓外 `zerg script inventory sync` D2 需人签）\n",
			*c.R4, gnrPyStr(float64(*res.ledRows)))
		for _, f := range res.items["R4"] {
			fmt.Fprintf(stdout, "     ✗ %s  ← 台账 tsv 无此行（治法 = 人签跑 sync · 本门只读不写）\n", f)
		}
	}
	for _, nj := range res.notJudged {
		fmt.Fprintf(stdout, "     ✗判不了 %s · %s —— %s\n", nj.face, nj.item, nj.why)
	}
	fmt.Fprintf(stdout, "  面⑤ 公开链两器排除表：%d（仅判 public=no 的件 · 须两器同批）\n", c.R5)
	for _, f := range res.items["R5"] {
		fmt.Fprintf(stdout, "     ✗ %s  ← 表判 no，两器排除表没收它（`EXCLUDES` 与 `DROP_EXACT` 两处都要）\n", f)
	}
	if c.R6 == nil {
		fmt.Fprintln(stdout, "  面⑥ 失败文案面：**判不了**（判据件不在盘）")
	} else {
		fmt.Fprintf(stdout, "  面⑥ 失败文案面（**单列旁栏 · 不进退码**）：%d\n", *c.R6)
		for _, ln := range res.wLines {
			mark := "✗"
			if res.wording != nil && *res.wording {
				mark = "✓"
			}
			fmt.Fprintf(stdout, "     %s %s\n", mark, ln)
		}
		if res.wording != nil && !*res.wording {
			fmt.Fprintln(stdout, "     ⇒ 文案仍只指 add_step；对「语料在仓外」与「写盘型」两类件是禁路 ⇒ 应改指白名单档（reason+date）")
		}
	}
	// ★ 面⑨ .sh 门面清册（盘上 `scripts/gates/*.sh` ⇄ `SH_GATES` 冻结清册 · 双向差集任一非空即 FAIL）
	fmt.Fprintf(stdout, "  面⑨ .sh 门面清册（盘上 `scripts/gates/*.sh` ⇄ `SH_GATES` 冻结清册 · **双向差集任一非空即 FAIL**）：%d\n", c.R7)
	if res.shDisk == nil || res.shRoster == nil {
		fmt.Fprintf(stdout, "     ✗ 面判不了（真源 %s 读不到 / 解析出 0 条）\n", gnrShGatesRel)
	} else {
		fmt.Fprintf(stdout, "     盘上 %d 件 · 清册 %d 件\n", len(res.shDisk), len(res.shRoster))
		for _, f := range res.shMissing {
			fmt.Fprintf(stdout, "     ✗ %s  ← 清册点名但盘上没有（悬空登记：从 `SH_GATES` 撤名 或 补回盘上那件）\n", f)
		}
		for _, f := range res.shExtra {
			fmt.Fprintf(stdout, "     ✗ %s  ← 盘上有但没进 `SH_GATES`（须同批登记进 `real-gates.sh` · GAP-20261003-06）\n", f)
		}
	}
	selfIn := gnrContains(res.newFiles, gnrSelfRel)
	fmt.Fprintf(stdout, "  ★ 自指：本门自身 %s —— 在分母内：%s · 面①：%s · 面②：%s · 面③：%s · 面④：见上「面④」那几行（台账 tsv 里暂无它 · 唯一写入口是仓外 D2 写面 · 需人签）· 面⑤：%s\n",
		gnrSelfRel, gnrBoolZh(selfIn, "是", "否（本趟不在新件集）"),
		gnrSelfMark(res.marks), gnrSelfDry(res.dryHits), gnrSelfInList(res.items["R3"]), gnrSelfInList(res.items["R5"]))
	for _, p := range res.problems {
		fmt.Fprintf(stdout, "  ✗ %s\n", p)
	}
	denOK := res.denFromBaseline && !res.denScanEmpty // ★ 甲/乙 分档
	if unjudged > 0 || (len(res.newFiles) == 0 && !denOK) {
		fmt.Fprintln(stdout, "  判据结论：**判不了 / 分母 0 ⇒ 不给结论（rc=2）**（真的没有 ≠ 判不了）")
		return rc
	}
	if len(res.newFiles) == 0 {
		fmt.Fprintln(stdout, "  判据结论：判到了 —— **全树扫过、确定性无新件**（现读件集 ∖ 冻结基线 = 0 件 · 正常态 · rc=0）")
		return rc
	}
	if bad > 0 {
		fmt.Fprintf(stdout, "  判据结论：**%d 条缺登记**（逐条点名如上）；报告档 rc=%d（只报告 · 不接闸）\n", bad, rc)
		return rc
	}
	fmt.Fprintln(stdout, "  判据结论：判到了 —— 五张登记表逐面 **0 条缺登记**（面④/⑥ 见上）")
	return rc
}

func gnrCountStr(p *int) string {
	if p == nil {
		return "None"
	}
	return gnrPyStr(float64(*p))
}

func gnrDiffStr(res *gnrRes) string {
	if !res.diffSet {
		return "判不了"
	}
	return gnrPyStr(float64(len(res.diff)))
}

func gnrPathOr(p string) string {
	if p == "" {
		return "不在盘"
	}
	return p
}

func gnrBoolZh(b bool, t, f string) string {
	if b {
		return t
	}
	return f
}

func gnrSelfMark(marks map[string]string) string {
	if marks != nil {
		if v, ok := marks[gnrSelfRel]; ok {
			return "已登记（public=" + v + "）"
		}
	}
	return "未登记"
}

func gnrSelfDry(hits map[string]bool) string {
	if hits != nil && hits[gnrSelfRel] {
		return "需登记"
	}
	return "不适用（正文不在该口径的扫描面内）"
}

func gnrSelfInList(list []string) string {
	if gnrContains(list, gnrSelfRel) {
		return "缺登记"
	}
	return "不在缺登记名单"
}

// ── 机器面（`--json`）：九面 × 五态（不改人面向一字）────────────────────────
// ★ 输出行序的决定者：JSON 的键序 = 本件构造 gnrJObj 时的**插入序**（对齐 py
// `json.dumps(..., sort_keys=False)` 的 dict 插入序）；`face_states` = `gnrFaces` 序（9 键）、
// `counts` = `gnrFacesStates` 序（6 键）、顶层 = 本函数 `obj` 的写入序。`reported_faces` 走
// `sort.Strings`（py `sorted`，Unicode 码位序 ≡ UTF-8 字节序）。★ **零 Go map 迭代进输出**：
// 所有 map 只做按键查值；要成列时先收进切片再 `sort`。
type gnrJField struct {
	k string
	v interface{}
}
type gnrJObj []gnrJField

func gnrIndent(n int) string { return strings.Repeat("  ", n) }

// gnrJSONStr —— Python `json.dumps(ensure_ascii=False)` 的串转义（✗ 不转义非 ASCII / `/`）。
func gnrJSONStr(sb *strings.Builder, s string) {
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString("\\\"")
		case '\\':
			sb.WriteString("\\\\")
		case '\n':
			sb.WriteString("\\n")
		case '\r':
			sb.WriteString("\\r")
		case '\t':
			sb.WriteString("\\t")
		case '\b':
			sb.WriteString("\\b")
		case '\f':
			sb.WriteString("\\f")
		default:
			if r < 0x20 {
				sb.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
}

// gnrJSONValue —— 复刻 `json.dumps(obj, ensure_ascii=False, indent=2, sort_keys=False)` 的排版。
//
//	空 dict/列表 ⇒ `{}` / `[]`；非空 ⇒ 逐项换行 + 2 空格缩进；`nil`（Go 侧显式 nil 接口 / 无值）
//	⇒ `null`；`[]string(nil)` ⇒ `null`（区分「键缺 ⇒ null」与「空表 ⇒ []」—— 与 py `None ⟷ []` 同）。
func gnrJSONValue(sb *strings.Builder, v interface{}, ind int) {
	switch x := v.(type) {
	case nil:
		sb.WriteString("null")
	case bool:
		if x {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case int:
		sb.WriteString(strconv.Itoa(x))
	case string:
		gnrJSONStr(sb, x)
	case []string:
		if x == nil {
			sb.WriteString("null")
			return
		}
		sb.WriteString("[")
		if len(x) == 0 {
			sb.WriteString("]")
			return
		}
		sb.WriteString("\n")
		for i, s := range x {
			sb.WriteString(gnrIndent(ind + 1))
			gnrJSONStr(sb, s)
			if i < len(x)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		sb.WriteString(gnrIndent(ind))
		sb.WriteString("]")
	case gnrJObj:
		if len(x) == 0 {
			sb.WriteString("{}")
			return
		}
		sb.WriteString("{\n")
		for i, f := range x {
			sb.WriteString(gnrIndent(ind + 1))
			gnrJSONStr(sb, f.k)
			sb.WriteString(": ")
			gnrJSONValue(sb, f.v, ind+1)
			if i < len(x)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		sb.WriteString(gnrIndent(ind))
		sb.WriteString("}")
	default:
		sb.WriteString("null")
	}
}

func gnrJGet(obj gnrJObj, k string) string {
	for _, f := range obj {
		if f.k == k {
			if s, ok := f.v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func gnrIntOrNil(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func gnrStrOrNil(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func gnrListOrNil(l []string) interface{} {
	if l == nil {
		return nil
	}
	return l
}

// gnrCntIface —— counts 里的某一个计数（缺失 / `None` ⇒ nil ⇒ JSON null）。
func gnrCntIface(c *gnrCounts, which string) interface{} {
	if c == nil {
		return nil
	}
	switch which {
	case "R1":
		return c.R1
	case "R2":
		return c.R2
	case "R3":
		return c.R3
	case "R4":
		if c.R4 == nil {
			return nil
		}
		return *c.R4
	case "R5":
		return c.R5
	case "R6":
		if c.R6 == nil {
			return nil
		}
		return *c.R6
	case "R7":
		return c.R7
	}
	return nil
}

func gnrFaceCount(res *gnrRes, name string) (int, bool) {
	c := res.counts
	if c == nil {
		return 0, false
	}
	switch name {
	case "①公开标记表":
		return c.R1, true
	case "②干跑登记":
		return c.R2, true
	case "③门①两张真源":
		return c.R3, true
	case "④脚本台账":
		if c.R4 == nil {
			return 0, false
		}
		return *c.R4, true
	case "⑤两器排除表":
		return c.R5, true
	case "⑥失败文案面":
		if c.R6 == nil {
			return 0, false
		}
		return *c.R6, true
	case "⑨.sh门面清册":
		return c.R7, true
	}
	return 0, false
}

// gnrBuildFaceStates —— ①–⑥ + ⑨ + 口径差栏 + 自指 → 五态（PASS/FAIL/BLOCKED/REPORT/UNKNOWN）。口径见 `gnrFacesStates` 注释。
//
//	★ **host-bound 面（⑧家目录批准件）不在这里出态** —— 由 `gnrFaceRecords` 统一给
//	  `HOST-REQUIRED · SKIPPED-HOST-BOUND`（不计 pass/fail）；未产出记录的面由 `gnrFaceRecords` 合成缺面。
func gnrBuildFaceStates(res *gnrRes) gnrJObj {
	unknownAll := res.counts == nil
	st := gnrJObj{}
	for _, name := range gnrFaces[:6] {
		if unknownAll {
			st = append(st, gnrJField{name, "UNKNOWN"})
			continue
		}
		if !res.faceJudged[name] {
			st = append(st, gnrJField{name, "BLOCKED"}) // 该面自己的真源读不到 / 面判不了
			continue
		}
		n, _ := gnrFaceCount(res, name)
		report := name == "④脚本台账" || name == "⑥失败文案面"
		switch {
		case report && n != 0:
			st = append(st, gnrJField{name, "REPORT"}) // 旁栏不进退码：有发现记 REPORT
		case n != 0:
			st = append(st, gnrJField{name, "FAIL"})
		default:
			st = append(st, gnrJField{name, "PASS"})
		}
	}
	// ⑨.sh门面清册：整跑短路 ⇒ UNKNOWN；清册真源读不到 / 盘面扫不到 ⇒ BLOCKED；否则按 R7 计数定 FAIL/PASS
	if unknownAll {
		st = append(st, gnrJField{"⑨.sh门面清册", "UNKNOWN"})
	} else if !res.faceJudged["⑨.sh门面清册"] {
		st = append(st, gnrJField{"⑨.sh门面清册", "BLOCKED"})
	} else if n, _ := gnrFaceCount(res, "⑨.sh门面清册"); n != 0 {
		st = append(st, gnrJField{"⑨.sh门面清册", "FAIL"})
	} else {
		st = append(st, gnrJField{"⑨.sh门面清册", "PASS"})
	}
	// 参照读数 · 不并入判红；台账口径扫面缺 ⇒ 参照读数也判不了
	if res.ledgerCnt == nil {
		st = append(st, gnrJField{"口径差栏", "BLOCKED"})
	} else {
		st = append(st, gnrJField{"口径差栏", "REPORT"})
	}
	// 自指依赖分母/面③⑤ ⇒ 整跑短路时未知
	if unknownAll {
		st = append(st, gnrJField{"自指", "UNKNOWN"})
	} else {
		st = append(st, gnrJField{"自指", "REPORT"})
	}
	return st
}

func gnrDiffFace(res *gnrRes) interface{} {
	if res.ledgerCnt == nil {
		return nil
	}
	var diff interface{}
	if res.diffSet {
		if res.diff == nil {
			diff = []string{}
		} else {
			diff = res.diff
		}
	}
	return gnrJObj{
		{"台账口径", *res.ledgerCnt},
		{"门①候选面", gnrIntOrNil(res.gate1Cnt)},
		{"对差", diff},
	}
}

// gnrFaceDetail —— `--json faces` 断面：逐面详情（state / judged / 缺登记数 / 命中件名）。
func gnrFaceDetail(res *gnrRes, st gnrJObj) gnrJObj {
	c := res.counts
	out := gnrJObj{}
	add := func(name string, obj gnrJObj) {
		obj = append(obj, gnrJField{"judged", res.faceJudged[name]})
		obj = append(obj, gnrJField{"state", gnrJGet(st, name)})
		out = append(out, gnrJField{name, obj})
	}
	add("①公开标记表", gnrJObj{
		{"缺登记", gnrCntIface(c, "R1")},
		{"items", gnrListOrNil(res.items["R1"])},
	})
	add("②干跑登记", gnrJObj{
		{"缺登记", gnrCntIface(c, "R2")},
		{"items", gnrListOrNil(res.items["R2"])},
		{"不适用件", gnrListOrNil(res.dryNA)},
	})
	add("③门①两张真源", gnrJObj{
		{"缺登记", gnrCntIface(c, "R3")},
		{"items", gnrListOrNil(res.items["R3"])},
		{"弱挂载", gnrListOrNil(res.weak)},
		{"通用语法步兜住", gnrListOrNil(res.fourth)},
	})
	add("④脚本台账", gnrJObj{
		{"缺登记", gnrCntIface(c, "R4")},
		{"items", gnrListOrNil(res.items["R4"])},
		{"台账路径", gnrStrOrNil(res.ledPath)},
		{"台账行数", gnrIntOrNil(res.ledRows)},
	})
	add("⑤两器排除表", gnrJObj{
		{"缺登记", gnrCntIface(c, "R5")},
		{"items", gnrListOrNil(res.items["R5"])},
	})
	add("⑥失败文案面", gnrJObj{
		{"未改文案", gnrCntIface(c, "R6")},
		{"文案", gnrListOrNil(res.wLines)},
	})
	add("⑨.sh门面清册", gnrJObj{
		{"缺登记", gnrCntIface(c, "R7")},
		{"items", gnrListOrNil(res.items["R7"])},
		{"盘上", gnrListOrNil(res.shDisk)},
		{"清册", gnrListOrNil(res.shRoster)},
		{"清册悬空", gnrListOrNil(res.shMissing)},
		{"盘上未登记", gnrListOrNil(res.shExtra)},
	})
	out = append(out, gnrJField{"口径差栏", gnrDiffFace(res)})
	return out
}

// gnrSelfDetail —— `--json self` 断面：本门对自身八面的登记情形（逐字对齐人面向「★ 自指」那一行）。
func gnrSelfDetail(res *gnrRes) gnrJObj {
	face1 := "未登记"
	if res.marks != nil {
		if v, ok := res.marks[gnrSelfRel]; ok {
			face1 = "已登记（public=" + v + "）"
		}
	}
	face2 := "不适用（正文不在该口径的扫描面内）"
	if res.dryHits != nil && res.dryHits[gnrSelfRel] {
		face2 = "需登记"
	}
	face3 := "不在缺登记名单"
	if gnrContains(res.items["R3"], gnrSelfRel) {
		face3 = "缺登记"
	}
	face5 := "不在缺登记名单"
	if gnrContains(res.items["R5"], gnrSelfRel) {
		face5 = "缺登记"
	}
	return gnrJObj{
		{"path", gnrSelfRel},
		{"在分母内", gnrContains(res.newFiles, gnrSelfRel)},
		{"面①", face1},
		{"面②", face2},
		{"面③", face3},
		{"面⑤", face5},
	}
}

// gnrVerdictText —— py `_verdict_text()`（仅机器面 `verdict` 字段用）。
func gnrVerdictText(res *gnrRes, rc int) string {
	if res.counts == nil {
		return "判不了 ⇒ 不给结论（rc=2）"
	}
	c := res.counts
	bad := c.R1 + c.R2 + c.R3 + c.R5 + c.R7
	unjudged := len(res.notJudged) + len(res.problems)
	if c.R4 == nil {
		unjudged++
	}
	denOK := res.denFromBaseline && !res.denScanEmpty
	if unjudged > 0 || (len(res.newFiles) == 0 && !denOK) {
		return "判不了 / 分母 0 ⇒ 不给结论（rc=2）"
	}
	if len(res.newFiles) == 0 {
		return "判到了 —— 全树 ∖ 冻结基线 = 0 件新件（正常态；五张登记表无新件可判）"
	}
	if bad > 0 {
		return fmt.Sprintf("%d 条缺登记（报告档 rc=%d）", bad, rc)
	}
	return "判到了 —— 五张登记表逐面 0 条缺登记"
}

// gnrRunJSON —— py `run_json()`：`--json <断面>` 只出机器面 JSON（人面向 stdout 让位）+ 与报告档同源退码。
// ★ 面清单 = `gnrExpectedFaces()`（跑前先生成）；逐面记录由 `gnrFaceRecords` 统一给
// （面⑧ host-bound ⇒ HOST-REQUIRED · 不计 pass/fail；未产出记录的面 ⇒ MISSING_FACE 合成）。
func gnrRunJSON(stdout, stderr io.Writer, root string, newFiles []string, newSet bool, inv string, invSet bool, strict bool, baseline, selector string) int {
	sel := []string{}
	for _, s := range strings.Split(selector, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			sel = append(sel, s)
		}
	}
	unknown := []string{}
	hasFaces, hasSelf := false, false
	for _, s := range sel {
		if s != "faces" && s != "self" {
			unknown = append(unknown, s)
		}
		if s == "faces" {
			hasFaces = true
		}
		if s == "self" {
			hasSelf = true
		}
	}
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "⛔ 用法错（rc=2）：`--json` 只认断面 faces,self；不认：%s\n", strings.Join(unknown, ","))
		return 2
	}
	if len(sel) == 0 {
		fmt.Fprint(stderr, "⛔ 用法错（rc=2）：`--json` 需要断面（faces,self 任一）\n")
		return 2
	}
	res := gnrAudit(root, newFiles, newSet, inv, invSet, baseline)
	rc, _, _ := gnrVerdict(res, strict, nil)
	faces := gnrExpectedFaces() // ★ 跑前先生成 expected_faces 清单
	states := gnrBuildFaceStates(res)
	records := gnrFaceRecords(res, states, faces)
	// counts：初值 = gnrFacesStates 序（6 键 · 全 0）；遇表外态则按 py dict 语义追加到末尾。
	cntKeys := append([]string{}, gnrFacesStates...)
	cntVals := map[string]int{}
	for _, s := range cntKeys {
		cntVals[s] = 0
	}
	for _, fld := range records {
		st := gnrJRecField(fld.v, "state")
		if _, ok := cntVals[st]; !ok {
			cntKeys = append(cntKeys, st)
			cntVals[st] = 0
		}
		cntVals[st]++
	}
	countsObj := gnrJObj{}
	for _, s := range cntKeys {
		countsObj = append(countsObj, gnrJField{s, cntVals[s]})
	}
	// missing / host_bound / reported —— 全部 sorted（输出行序决定者 = 这三处 sort）
	missing := []string{}
	hostBound := []string{}
	reported := []string{}
	for _, fld := range records {
		rf := gnrJRecField(fld.v, "reason_code")
		if rf == gnrReasonMissingFace {
			missing = append(missing, fld.k)
		} else {
			reported = append(reported, fld.k)
		}
		if rf == gnrReasonHostBound {
			hostBound = append(hostBound, fld.k)
		}
	}
	sort.Strings(missing)
	sort.Strings(hostBound)
	sort.Strings(reported)
	// face_states = {f: records[f].state for f in faces}（键序 = gnrFaces 序 · ✗ 零 map 迭代）
	faceStates := gnrJObj{}
	recState := map[string]string{}
	for _, fld := range records {
		recState[fld.k] = gnrJRecField(fld.v, "state")
	}
	for _, f := range faces {
		faceStates = append(faceStates, gnrJField{f, recState[f]})
	}
	allGreen := rc == 0 && cntVals["FAIL"] == 0 && cntVals["BLOCKED"] == 0 && cntVals["UNKNOWN"] == 0
	obj := gnrJObj{
		{"schema", "zerg/gate-json/v1"},
		{"tool", "check-newfile-registry.py"},
		{"gate", "门76 新件登记完整性"},
		{"repo", root},
		{"selector", sel},
		{"strict", strict},
		{"rc", rc},
		{"expected_faces", faces},
		{"reported_faces", reported},
		{"face_states", faceStates},
		{"face_records", records},
		{"counts", countsObj},
		{"host_required_faces", hostBound},
		{"host_required_count", len(hostBound)},
		{"missing_faces", missing},
		{"missing_count", len(missing)},
		{"fully_exercised", len(missing) == 0 && len(hostBound) == 0},
		{"all_green", allGreen},
		{"verdict", gnrVerdictText(res, rc)},
	}
	if hasFaces {
		obj = append(obj, gnrJField{"faces", gnrFaceDetail(res, states)})
	}
	if hasSelf {
		obj = append(obj, gnrJField{"self", gnrSelfDetail(res)})
	}
	var sb strings.Builder
	gnrJSONValue(&sb, obj, 0)
	sb.WriteString("\n")
	fmt.Fprint(stdout, sb.String())
	return rc
}

// ── 自检（合成夹具 · 成对 · 不扫真仓）──────────────────────────────────────
const (
	gnrStubRegRel   = "core/internal/contract/semantics-stub.json"
	gnrStubJudgeRel = "scripts/gates/check-semantics-stub.py"
	gnrProbeRel     = "scripts/sandbox/new-probe.sh"
)

var gnrFixtureStub = "REGISTRY_REL = \"" + gnrStubRegRel + "\"\n\n\ndef scan_hits(root):\n    return {\"" + gnrProbeRel + "\"}, None\n"

const gnrFixtureMarks = "文件\tpublic\t理由\nscripts/old.sh\tno\t夹具存量件\n"

const gnrFixtureCov = `{
  "scan": {"script_globs": ["scripts/*.sh", "scripts/*.py", "scripts/sandbox/*.sh"],
           "nosuffix_dir": "scripts"},
  "exclude": {"path_components": ["__pycache__", "_history"]},
  "whitelist": {"scripts": []}
}
`

const gnrFixtureDry = `{
  "_扫描命令": "grep -r -E \"（口径 = 判据件的扫描函数）\" --include=*.sh --include=*.py scripts core",
  "件": [{"file": "scripts/old.sh"}]
}
`

const gnrFixtureSteps = `#!/usr/bin/env bash
add_step "gates" tri-report "门76 新件登记完整性（只报告）" python3 scripts/gates/check-newfile-registry.py
add_step "gates" tri "门⑩ 干跑：一套语义（阻断）" scripts/gates/check-semantics-stub.py
add_step "gates" tri "public: 公开面标记表" python3 scripts/gates/check-public-marks.py
`

const gnrFixturePub = `#!/usr/bin/env bash
EXCLUDES=(
  "tools/ocr/venv"
)
`

const gnrFixtureMirror = `DROP_EXACT = {
    "core/internal/o1.json",
}
`

// gnrFixtureShGates —— ⑨ 面清册桩（与盘上 `scripts/gates/*.sh` 逐只相等 ⇒ 两向皆空 · 逐字照 py `build_fixture`）。
const gnrFixtureShGates = "SH_GATES=(precommit-gates.sh real-gates.sh)\n"

const gnrFixtureCovJudge = `#!/usr/bin/env python3
def report():
    print("        ⇒ 处置：**真修** —— 该件与本批代码**同批挂门**"
          "（` + "`scripts/gates/precommit-gates.sh`" + ` 的 ` + "`add_step`" + ` 按名调用它）；"
          "新件**不许**蹭基线棘轮。")
`

// gnrFailer —— 让夹具既能给 `*testing.T` 用、也能给自检（t=nil）用。
type gnrFailer interface{ Fatal(...interface{}) }

// gnrBuildFixture —— py `build_fixture()` 的逐字同构（六张表最小真形状 + 一件存量件 + 可选一件新件）。
func gnrBuildFixture(t gnrFailer, tmp, probe string, register map[string]bool) {
	mk := func(rel, body string) {
		p := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			if t != nil {
				t.Fatal(err)
			}
			return
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			if t != nil {
				t.Fatal(err)
			}
		}
	}
	for _, d := range []string{"scripts", "scripts/sandbox", "scripts/build", "scripts/gates", "publish",
		"core/internal/contract"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			if t != nil {
				t.Fatal(err)
			}
			return
		}
	}
	mk(filepath.Join("scripts", "old.sh"), "#!/usr/bin/env bash\necho old\n")
	marks, cov, dry, pub, mir := gnrFixtureMarks, gnrFixtureCov, gnrFixtureDry, gnrFixturePub, gnrFixtureMirror
	if probe != "" {
		mk(probe, "#!/usr/bin/env bash\necho probe\n")
		if register["①"] {
			marks += probe + "\tno\t夹具新件\n"
		}
		if register["②"] {
			dry = strings.Replace(dry, `{"file": "scripts/old.sh"}`,
				`{"file": "scripts/old.sh"}, {"file": "`+probe+`"}`, 1)
		}
		if register["③"] {
			cov = strings.Replace(cov, `"whitelist": {"scripts": []}`,
				`"whitelist": {"scripts": [{"path": "`+probe+`", "reason": "夹具", "date": "2026-09-28"}]}`, 1)
		}
		if register["⑤"] {
			pub = strings.Replace(pub, "\n)", "\n  \""+probe+"\"\n)", 1)
			mir = strings.Replace(mir, "\n}", "\n    \""+probe+"\",\n}", 1)
		}
	}
	mk(gnrMarksRel, marks)
	mk(gnrCovRel, cov)
	mk(gnrStubRegRel, dry)
	mk(gnrStubJudgeRel, gnrFixtureStub)
	mk(gnrStepsRel, gnrFixtureSteps)
	mk(gnrPubRel, pub)
	mk(gnrMirrorRel, mir)
	mk(gnrCovJudgeRel, gnrFixtureCovJudge)
	// ⑨ 面真源桩：`scripts/gates/*.sh` 现盘 = {precommit-gates.sh, real-gates.sh} ⇒ 清册逐只点名、两向皆空
	mk(gnrShGatesRel, gnrFixtureShGates)
}

type gnrTestCase struct {
	name    string
	want    int
	got     int
	ok      bool
	newF    []string
	newSet  bool
	inv     string
	invSet  bool
	strict  bool
	base    string
	baseSet bool
}

// gnrSelfTest —— py `run_self_test()` 的逐字同构（8 格 · 输出与 py 逐字对齐，仅夹具随机后缀不同）。
func gnrSelfTest(stdout io.Writer) int {
	checks := []gnrTestCase{}
	probe := gnrProbeRel
	run1 := func(name string, want int, newF []string, newSet bool, inv string, invSet bool, strict bool, base string, baseSet bool, setup func(tmp string)) {
		tmp, err := os.MkdirTemp("", "gnr-selftest-")
		if err != nil {
			return
		}
		defer os.RemoveAll(tmp)
		setup(tmp)
		rc := gnrRun(stdout, tmp, newF, newSet, inv, invSet, strict, base, baseSet)
		checks = append(checks, gnrTestCase{name: name, want: want, got: rc, ok: rc == want})
	}
	allFive := func(tmp string) {
		gnrBuildFixture(nil, tmp, probe, map[string]bool{"①": true, "②": true, "③": true, "⑤": true})
		os.WriteFile(filepath.Join(tmp, "inv.tsv"), []byte("件\n"+probe+"\n"), 0o644)
	}
	unregistered := func(tmp string) {
		gnrBuildFixture(nil, tmp, probe, nil)
		os.WriteFile(filepath.Join(tmp, "inv.tsv"), []byte("件\n"+probe+"\n"), 0o644)
	}
	noLedger := func(tmp string) {
		gnrBuildFixture(nil, tmp, probe, map[string]bool{"①": true, "②": true, "③": true, "⑤": true})
		os.WriteFile(filepath.Join(tmp, "inv-absent.tsv"), []byte("件\n"), 0o644)
	}
	noProbe := func(tmp string) {
		gnrBuildFixture(nil, tmp, "", nil)
		os.WriteFile(filepath.Join(tmp, "inv.tsv"), []byte("件\n"), 0o644)
	}
	weakOnly := func(tmp string) {
		gnrBuildFixture(nil, tmp, probe, map[string]bool{"①": true, "②": true, "⑤": true})
		f, _ := os.OpenFile(filepath.Join(tmp, gnrStepsRel), os.O_APPEND|os.O_WRONLY, 0o644)
		if f != nil {
			f.WriteString("add_step \"build\" tri \"构建\" bash scripts/build/new-probe.sh\n")
			f.Close()
		}
		os.WriteFile(filepath.Join(tmp, "inv.tsv"), []byte("件\n"+probe+"\n"), 0o644)
	}
	run1("正控① · 五表登记齐（报告档）⇒ 0", 0, []string{probe}, true, "inv.tsv", true, false, "", false, allFive)
	run1("正控② · 五表登记齐（阻断档）⇒ 0", 0, []string{probe}, true, "inv.tsv", true, true, "", false, allFive)
	run1("负控① · 新件**缺登记**（报告档不阻断）⇒ 0 但逐条点名", 0, []string{probe}, true, "inv.tsv", true, false, "", false, unregistered)
	run1("负控② · 新件缺登记（阻断档）⇒ 1 红", 1, []string{probe}, true, "inv.tsv", true, true, "", false, unregistered)
	run1("判不了① · **分母 0 件**（没有新件）⇒ 2 不许绿", 2, []string{}, true, "", false, false, "", false, noProbe)
	run1("判不了② · 面④ 仓外台账真源不在 ⇒ 2（含 ⑤ 已登记 ⇒ 不许报绿）", 2, []string{probe}, true, "路径不存在.tsv", true, false, "", false, noLedger)
	run1("边界① · 基线不在盘 ⇒ 2（不替基线猜分母）", 2, nil, false, "", false, false, "", false, noProbe)
	run1("边界② · 仅 basename 弱挂载 ⇒ 不判红（不计入六表缺登记）", 0, []string{probe}, true, "inv.tsv", true, true, "", false, weakOnly)

	// ── 查缺档 / host-bound / 缺面（合成夹具 · 成对）──────────────────────────
	addCheck := func(name string, want, got int, ok bool) {
		checks = append(checks, gnrTestCase{name: name, want: want, got: got, ok: ok})
	}
	// (A) host-bound 面 + 缺面聚合（纯数据 · 不碰 git）
	if tmpA, errA := os.MkdirTemp("", "gnr-selftest-"); errA == nil {
		defer os.RemoveAll(tmpA)
		gnrBuildFixture(nil, tmpA, probe, map[string]bool{"①": true, "②": true, "③": true, "⑤": true})
		os.WriteFile(filepath.Join(tmpA, "inv.tsv"), []byte("件\n"+probe+"\n"), 0o644)
		resA := gnrAudit(tmpA, []string{probe}, true, "inv.tsv", true, "")
		recsA := gnrFaceRecords(resA, nil, nil)
		f8 := gnrRecValue(recsA, "⑧家目录批准件")
		st8 := gnrJRecField(f8, "state")
		r8 := gnrJRecField(f8, "reason_code")
		d8 := gnrJRecBool(f8, "decided")
		addCheck("查缺档·面⑧ host-bound ⇒ state=HOST-REQUIRED", 1, gnrB2i(st8 == "HOST-REQUIRED"), st8 == "HOST-REQUIRED")
		addCheck("查缺档·面⑧ reason_code=SKIPPED-HOST-BOUND", 1, gnrB2i(r8 == gnrReasonHostBound), r8 == gnrReasonHostBound)
		addCheck("查缺档·面⑧ decided=false（不当绿也不当红）", 1, gnrB2i(!d8), !d8)
		cntA := map[string]int{}
		for _, fld := range recsA {
			cntA[gnrJRecField(fld.v, "state")]++
		}
		addCheck("查缺档·HOST-REQUIRED 单列计数=1（不计入 PASS/FAIL）", 1,
			gnrB2i(cntA["HOST-REQUIRED"] == 1 && cntA["FAIL"] == 0),
			cntA["HOST-REQUIRED"] == 1 && cntA["FAIL"] == 0)
		bogus := "⑨合成缺面"
		recsMissing := gnrFaceRecords(resA, nil, append(gnrExpectedFaces(), bogus))
		bm := gnrRecValue(recsMissing, bogus)
		sm := gnrJRecField(bm, "state")
		rm := gnrJRecField(bm, "reason_code")
		dm := gnrJRecBool(bm, "decided")
		addCheck("缺面·未产出记录的面 ⇒ 合成 UNKNOWN", 1, gnrB2i(sm == "UNKNOWN"), sm == "UNKNOWN")
		addCheck("缺面·reason_code=MISSING_FACE", 1, gnrB2i(rm == gnrReasonMissingFace), rm == gnrReasonMissingFace)
		addCheck("缺面·decided=false", 1, gnrB2i(!dm), !dm)
		rcM, _, _ := gnrVerdict(resA, true, append(gnrExpectedFaces(), bogus))
		addCheck("缺面·退码钉 2（缺面不许当齐）", 2, rcM, rcM == 2)
		rcOK, _, _ := gnrVerdict(resA, true, nil)
		addCheck("正控·无缺面 ⇒ 不因聚合器变 rc", 0, rcOK, rcOK == 0)
		addCheck("正控·缺面注入 ⇒ 2 · 不注入 ⇒ 0（区分度）", 2, rcM, rcM == 2 && rcOK == 0)
	}

	// (B) 三合并式四负控（真 git 夹具 · 仓外）
	gitFixture := func(root string) {
		os.MkdirAll(filepath.Join(root, "scripts"), 0o755)
		os.WriteFile(filepath.Join(root, "scripts", "a.sh"), []byte("#!/usr/bin/env bash\necho a\n"), 0o644)
		env := append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		for _, a := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "-m", "init"}} {
			cmd := exec.Command("git", append([]string{"-C", root}, a...)...)
			cmd.Env = env
			cmd.Run()
		}
	}
	// 负控① 未跟踪新件（不 git add）⇒ 红；并证「git diff 单用看不见它」
	if tmpG, eG := os.MkdirTemp("", "gnr-selftest-"); eG == nil {
		defer os.RemoveAll(tmpG)
		gitFixture(tmpG)
		os.WriteFile(filepath.Join(tmpG, "scripts", "new-probe.sh"), []byte("#!/usr/bin/env bash\necho new\n"), 0o644)
		rU := gnrWorktreeThreeWay(tmpG, []string{"scripts"})
		rcU := gnrChkMissingRC(rU)
		addCheck("负控①·未跟踪新件（不 git add）⇒ 红 rc=1", 1, rcU,
			rcU == 1 && len(rU.untracked) == 1 && rU.untracked[0] == "scripts/new-probe.sh")
		drc, dout, _ := gnrGit(tmpG, "diff", "--name-only", "--exit-code", "HEAD", "--", "scripts")
		addCheck("负控①·git diff 单用看不见 untracked（rc=0 · 洞真被 ② 补上）", 0, drc,
			drc == 0 && len(dout) == 0)
	}
	// 负控② 被 .gitignore 命中的路径 ⇒ 红（ignored 捕获）· 绝不 rc=0
	if tmpI, eI := os.MkdirTemp("", "gnr-selftest-"); eI == nil {
		defer os.RemoveAll(tmpI)
		gitFixture(tmpI)
		os.WriteFile(filepath.Join(tmpI, ".gitignore"), []byte("scripts/ignored/\n"), 0o644)
		os.MkdirAll(filepath.Join(tmpI, "scripts", "ignored"), 0o755)
		os.WriteFile(filepath.Join(tmpI, "scripts", "ignored", "x.sh"), []byte("#!/usr/bin/env bash\necho x\n"), 0o644)
		rI := gnrWorktreeThreeWay(tmpI, []string{"scripts"})
		rcI := gnrChkMissingRC(rI)
		addCheck("负控②·.gitignore 命中 ⇒ 红（ignored 捕获）· 绝不 rc=0", 1, rcI,
			rcI == 1 && len(rI.ignored) == 1 && rI.ignored[0] == "scripts/ignored/x.sh")
	}
	// 负控③ 零执行探针（空件集）⇒ no_data 置位 · 非绿 rc=2
	if tmpE, eE := os.MkdirTemp("", "gnr-selftest-"); eE == nil {
		defer os.RemoveAll(tmpE)
		gitFixture(tmpE)
		rE := gnrWorktreeThreeWay(tmpE, []string{"scripts-nonexistent"})
		rcE := gnrChkMissingRC(rE)
		addCheck("负控③·空件集（零执行探针）⇒ no_data + rc=2", 2, rcE,
			rcE == 2 && rE.noData && rE.fileSetSize == 0)
	}
	// 负控④ 真源不可读 ⇒ rc=2 · 绝不 rc=0
	if tmpX, eX := os.MkdirTemp("", "gnr-selftest-"); eX == nil {
		defer os.RemoveAll(tmpX)
		os.WriteFile(filepath.Join(tmpX, ".git"), []byte("gitdir: /nonexistent-gnr-selftest\n"), 0o644)
		rX := gnrWorktreeThreeWay(tmpX, []string{"scripts"})
		rcX := gnrChkMissingRC(rX)
		addCheck("负控④·真源不可读（.git 破损）⇒ rc=2 · 绝不 0", 2, rcX,
			rcX == 2 && rX.unreadable)
	}
	// 隔离：worktree 退出必清 · 生成前清空输出目录
	if tmpW, eW := os.MkdirTemp("", "gnr-selftest-"); eW == nil {
		if outd, eO := os.MkdirTemp("", "gnr-selftest-out-"); eO == nil {
			defer os.RemoveAll(tmpW)
			defer os.RemoveAll(outd)
			gitFixture(tmpW)
			os.WriteFile(filepath.Join(outd, "旧件残留.txt"), []byte("stale\n"), 0o644)
			rW := gnrWorktreeThreeWay(tmpW, []string{"scripts"})
			_, lout, _ := gnrGit(tmpW, "worktree", "list")
			addCheck("隔离·worktree 退出必清（list 仅 1 条 = 主树）", 1, len(lout),
				rW.worktreeCleaned && len(lout) == 1)
			rcW := gnrRunCheckMissing(io.Discard, tmpW, []string{"scripts"}, outd, false)
			_, statErr := os.Stat(filepath.Join(outd, "旧件残留.txt"))
			_, jsonErr := os.Stat(filepath.Join(outd, "查缺档.json"))
			addCheck("隔离·干净夹具 ⇒ rc=0 · 生成前清空输出目录（旧件被清）", 0, rcW,
				rcW == 0 && os.IsNotExist(statErr) && jsonErr == nil)
		}
	}

	// (C) ★ 甲/乙 分档（成对负控）：--dump-baseline 打当前树后**不再必然 rc=2** ——
	//     「全树扫过（面非空）∖ 在盘基线 = 0 件」= 正常态 ⇒ 0；「基线不在盘 / 显式空件集 / 扫描面为空」⇒ 2（零执行 ✗ 不放宽）。
	if tmpD, errD := os.MkdirTemp("", "gnr-selftest-"); errD == nil {
		defer os.RemoveAll(tmpD)
		gnrBuildFixture(nil, tmpD, probe, map[string]bool{"①": true, "②": true, "③": true, "⑤": true})
		os.WriteFile(filepath.Join(tmpD, "inv.tsv"), []byte("件\n"+probe+"\n"), 0o644)
		baseD := filepath.Join(tmpD, "baseline.tsv")
		if f, e := os.Create(baseD); e == nil {
			f.WriteString("# 夹具冻结基线（现读件集快照）\n")
			faceD := gnrScanLedgerFace(tmpD)
			keys := []string{}
			for k := range faceD {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				f.WriteString(k + "\n")
			}
			f.Close()
		}
		rcA := gnrRun(io.Discard, tmpD, nil, false, "inv.tsv", true, false, baseD, true)
		addCheck("甲·全树扫过 ∖ 在盘基线 = 0 件 ⇒ 0（正常态 · dump 后不再必然 2）", 0, rcA, rcA == 0)
		rcB := gnrRun(io.Discard, tmpD, nil, false, "inv.tsv", true, false, filepath.Join(tmpD, "no-such-baseline.tsv"), true)
		addCheck("乙·基线不在盘（真源拿不到）⇒ 2（未放宽）", 2, rcB, rcB == 2)
		rcZ := gnrRun(io.Discard, tmpD, []string{}, true, "inv.tsv", true, false, "", false)
		addCheck("乙·显式空件集（零执行探针）⇒ 2（未放宽）", 2, rcZ, rcZ == 2)
		zero := 0
		mkRes := func(scanEmpty bool) *gnrRes {
			return &gnrRes{counts: &gnrCounts{R4: &zero}, newFiles: []string{}, notJudged: []gnrNJ{},
				problems: []string{}, items: map[string][]string{}, denFromBaseline: true, denScanEmpty: scanEmpty}
		}
		rcSE, _, _ := gnrVerdict(mkRes(true), false, nil)
		rcSN, _, _ := gnrVerdict(mkRes(false), false, nil)
		addCheck("乙·扫描面为空（基线在盘但面空 = 零执行）⇒ 2（未放宽）", 2, rcSE, rcSE == 2)
		addCheck("区分度·同一 res 形状：面非空 ⇒ 0 · 面空 ⇒ 2（甲/乙 真分开）", 0, rcSN, rcSN == 0 && rcSE == 2)
	}

	// (D) ★ B1-5 成对负控（本片新增：① OSError 在盘但读不出 · ② 行内注释 · ③ 基线 v2 过期/只许下调/指纹读回 · ④ --sync 两态）
	if tmpS, errS := os.MkdirTemp("", "gnr-selftest-"); errS == nil {
		defer os.RemoveAll(tmpS)
		gnrBuildFixture(nil, tmpS, probe, map[string]bool{"①": true, "②": true, "③": true, "⑤": true})
		os.WriteFile(filepath.Join(tmpS, "inv.tsv"), []byte("件\n"+probe+"\n"), 0o644)
		nScan := len(gnrScanLedgerFace(tmpS))

		// ① 基线在盘但读不出（chmod 000）⇒ 2 fail-closed ⟷ 同件可读 ⇒ 0（成对）
		baseRO := filepath.Join(tmpS, "base-ro.tsv")
		os.WriteFile(baseRO, []byte(fmt.Sprintf("# 门76 冻结基线 v2 件数=%d 指纹=deadbeefdeadbeef\n", nScan)), 0o644)
		os.Chmod(baseRO, 0)
		rcRO := gnrRun(io.Discard, tmpS, nil, false, "inv.tsv", true, false, baseRO, true)
		addCheck("B1-5·① 基线在盘但读不出（chmod 000）⇒ 2（fail-closed · 未捕获 OSError 曾静默绿）", 2, rcRO, rcRO == 2)
		os.Chmod(baseRO, 0o644)
		rcROk := gnrRun(io.Discard, tmpS, nil, false, "inv.tsv", true, false, baseRO, true)
		addCheck("B1-5·① 成对正控：同基线可读且件数吻合 ⇒ 0（读得出 ⇒ 判得了）", 0, rcROk, rcROk == 0)

		// ② load_excludes 剥行内注释（消假阳性）
		if tmpL, errL := os.MkdirTemp("", "gnr-selftest-"); errL == nil {
			os.MkdirAll(filepath.Join(tmpL, filepath.Dir(gnrPubRel)), 0o755)
			os.WriteFile(filepath.Join(tmpL, gnrPubRel),
				[]byte("EXCLUDES=(\n  \"a.sh\"   # 行内注释\n  \"b.sh\"\n)\n"), 0o644)
			exL := gnrLoadExcludes(tmpL)
			os.RemoveAll(tmpL)
			okL := len(exL) == 2 && exL["a.sh"] && exL["b.sh"]
			addCheck("B1-5·② load_excludes 剥行内注释 ⇒ {a.sh,b.sh}（消假阳性）", 1, gnrB2i(okL), okL)
		}
		// ② 成对负控：原样不剥 ⇒ 注释尾混入件名（假阳性 · 证明本修必要）
		if tmpL2, errL2 := os.MkdirTemp("", "gnr-selftest-"); errL2 == nil {
			os.MkdirAll(filepath.Join(tmpL2, filepath.Dir(gnrPubRel)), 0o755)
			os.WriteFile(filepath.Join(tmpL2, gnrPubRel),
				[]byte("EXCLUDES=(\n  \"a.sh\"   # 行内注释\n  \"b.sh\"\n)\n"), 0o644)
			naive := map[string]bool{}
			if txt2, okT := gnrReadText(filepath.Join(tmpL2, gnrPubRel)); okT {
				if i := strings.Index(txt2, "EXCLUDES=("); i >= 0 {
					rest := txt2[i+len("EXCLUDES=("):]
					if j := strings.Index(rest, ")"); j >= 0 {
						for _, q := range strings.Split(rest[:j], "\n") {
							s := strings.TrimSpace(q)
							if s == "" || strings.HasPrefix(s, "#") {
								continue
							}
							naive[gnrStripQ(s)] = true
						}
					}
				}
			}
			os.RemoveAll(tmpL2)
			anyHash := false
			for x := range naive {
				if strings.Contains(x, "#") {
					anyHash = true
				}
			}
			addCheck("B1-5·② 负控：原样不剥注释 ⇒ 注释尾混入件名（假阳性 · 证明本修必要）", 1, gnrB2i(anyHash), anyHash)
		}

		// ③ 基线 v2 过期即红（声明件数 > 现读扫面）
		baseEX := filepath.Join(tmpS, "base-expired.tsv")
		os.WriteFile(baseEX, []byte(fmt.Sprintf("# 门76 冻结基线 v2 件数=%d 指纹=0000000000000000\n", nScan+100)), 0o644)
		rcEX := gnrRun(io.Discard, tmpS, nil, false, "inv.tsv", true, false, baseEX, true)
		addCheck(fmt.Sprintf("B1-5·③ 基线 v2 过期（声明 %d > 现读 %d）⇒ 2（过期即红）", nScan+100, nScan), 2, rcEX, rcEX == 2)

		// ③ 基线 v2 头（件数+指纹）可解析读回 + dump 只许下调拒写
		baseSmall := filepath.Join(tmpS, "base-small.tsv")
		os.WriteFile(baseSmall, []byte("# 门76 冻结基线 v2 件数=1 指纹=1111111111111111\n"), 0o644)
		if txtS, okS := gnrReadText(baseSmall); okS {
			cS, fS, _ := gnrParseBaselineHeader(txtS)
			okHdr := cS == 1 && fS == "1111111111111111"
			addCheck("B1-5·③ 基线 v2 头（件数/指纹）读回一致", 1, gnrB2i(okHdr), okHdr)
		}
		// ★ 对齐 py `_quiet`（只吞 stdout · stderr 漏到真 stderr）：干跑拒写的 stderr 判词
		//   必须进进程 stderr（此前喂 io.Discard ⇒ 与 py 分叉）。
		rcDown2 := gnrDumpBaseline(io.Discard, os.Stderr, tmpS, baseSmall, false)
		addCheck(fmt.Sprintf("B1-5·③ 基线只许下调（旧 1 < 现读 %d）⇒ 干跑即拒写 2", nScan), 2, rcDown2, rcDown2 == 2)

		// ④ --sync 干跑零写入 ⟷ --yes 真写（成对 · 原子写 · 无残留）
		marksP := filepath.Join(tmpS, gnrMarksRel)
		shaFile := func(p string) string {
			b, err := os.ReadFile(p)
			if err != nil {
				return ""
			}
			h := sha256.Sum256(b)
			return hex.EncodeToString(h[:])
		}
		sha0 := shaFile(marksP)
		baseS := filepath.Join(tmpS, "base-sync.tsv")
		os.WriteFile(baseS, []byte(fmt.Sprintf("# 门76 冻结基线 v2 件数=%d 指纹=cafecafecafecafe\n", nScan)), 0o644)
		rcSD := gnrRunSync(io.Discard, io.Discard, tmpS, false, baseS)
		sha1 := shaFile(marksP)
		addCheck("B1-5·④ `--sync` 干跑 ⇒ 0 且零写入（marks 表 sha 逐字同）", 0, rcSD, rcSD == 0 && sha0 == sha1)
		rcSY := gnrRunSync(io.Discard, io.Discard, tmpS, true, baseS)
		sha2 := shaFile(marksP)
		addCheck("B1-5·④ `--sync --yes` ⇒ 0 且真写（marks 表 sha 变 · 与干跑成对）", 0, rcSY, rcSY == 0 && sha0 != sha2)
		resid := 0
		if ents, errR := os.ReadDir(tmpS); errR == nil {
			for _, e := range ents {
				if strings.HasPrefix(e.Name(), ".gnr-marks-") {
					resid++
				}
			}
		}
		addCheck("B1-5·④ 原子写无残留临时件（temp+fsync+rename）", 0, resid, resid == 0)
	}

	// (E) ★ 面⑨ `.sh` 门面清册 成对正负控（盘上 scripts/gates/*.sh ⇄ real-gates.sh 的 SH_GATES 双向差集）
	shGatesFixture := func(root string, extraDisk, extraRoster []string) {
		gnrBuildFixture(nil, root, probe, map[string]bool{"①": true, "②": true, "③": true, "⑤": true})
		os.WriteFile(filepath.Join(root, "inv.tsv"), []byte("件\n"+probe+"\n"), 0o644)
		roster := append([]string{"precommit-gates.sh", "real-gates.sh"}, extraRoster...)
		os.WriteFile(filepath.Join(root, gnrShGatesRel),
			[]byte("SH_GATES=("+strings.Join(roster, " ")+")\n"), 0o644)
		for _, e := range extraDisk {
			os.WriteFile(filepath.Join(root, gnrShGatesDir, e),
				[]byte("#!/usr/bin/env bash\necho g\n"), 0o644)
		}
	}
	shRun := func(extraDisk, extraRoster []string) *gnrRes {
		root, err := os.MkdirTemp("", "gnr-selftest-")
		if err != nil {
			return nil
		}
		defer os.RemoveAll(root)
		shGatesFixture(root, extraDisk, extraRoster)
		return gnrAudit(root, []string{probe}, true, "inv.tsv", true, "")
	}
	resP := shRun(nil, nil)
	resE := shRun([]string{"ghost-on-disk.sh"}, nil)
	resM := shRun(nil, []string{"ghost-in-roster.sh"})
	if resP != nil && resE != nil && resM != nil {
		mkSt := func(r *gnrRes) string { return gnrJGet(gnrBuildFaceStates(r), "⑨.sh门面清册") }
		stP, stE, stM := mkSt(resP), mkSt(resE), mkSt(resM)
		okP := stP == "PASS" && len(resP.shMissing) == 0 && len(resP.shExtra) == 0
		addCheck("⑨·正控 盘上 ⇄ SH_GATES 相等（两向皆空）state=PASS", 1, gnrB2i(okP), okP)
		rcP, _, _ := gnrVerdict(resP, true, nil)
		addCheck("⑨·正控 相等 ⇒ strict rc=0（不误红）", 0, rcP, rcP == 0)
		okE := stE == "FAIL" && len(resE.shExtra) == 1 && resE.shExtra[0] == "ghost-on-disk.sh" && len(resE.shMissing) == 0
		addCheck("⑨·负控甲 盘上有件没进清册 ⇒ FAIL（缺登记）", 1, gnrB2i(okE), okE)
		rcEs, _, _ := gnrVerdict(resE, true, nil)
		addCheck("⑨·负控甲 缺登记 ⇒ strict rc=1", 1, rcEs, rcEs == 1)
		rcEr, _, _ := gnrVerdict(resE, false, nil)
		addCheck("⑨·负控甲 报告档 rc=0（面 FAIL 但只报告 · 与 strict 成对）", 0, rcEr, rcEr == 0)
		okM := stM == "FAIL" && len(resM.shMissing) == 1 && resM.shMissing[0] == "ghost-in-roster.sh" && len(resM.shExtra) == 0
		addCheck("⑨·负控乙 清册点名盘上无 ⇒ FAIL（悬空登记）", 1, gnrB2i(okM), okM)
		rcMs, _, _ := gnrVerdict(resM, true, nil)
		addCheck("⑨·负控乙 悬空 ⇒ strict rc=1", 1, rcMs, rcMs == 1)
		okD := stP == "PASS" && stE == "FAIL"
		addCheck("⑨·区分度 同一夹具只差一件：相等 ⇒ PASS · 盘上多一件 ⇒ FAIL（成对）", 1, gnrB2i(okD), okD)
	}

	fmt.Fprintln(stdout, "── 自检（合成夹具 · 成对 · 不扫真仓）──")
	bad := 0
	for _, c := range checks {
		mark := "✗"
		if c.ok {
			mark = "✓"
		}
		fmt.Fprintf(stdout, "  %s %s —— 要 %d · 得 %d\n", mark, c.name, c.want, c.got)
		if !c.ok {
			bad++
		}
	}
	concl := "全过"
	if bad != 0 {
		concl = "**不过**"
	}
	fmt.Fprintf(stdout, "自检结论: %s（断言 %d 条 · 失败 %d）\n", concl, len(checks), bad)
	if bad == 0 {
		return 0
	}
	return 1
}

// ── 默认根（见件头 (a)）────────────────────────────────────────────────────
// gnrIsRootDir —— 仓根标记：同时含 `scripts/` 目录与 `scripts/gates/precommit-gates.sh`
// （与本件依赖的两处真源同源 ⇒ 只在**本件真能判**的树上认根 · ✗ 不把临时目录误当仓根）。
func gnrIsRootDir(dir string) bool {
	return gnrIsDir(filepath.Join(dir, "scripts")) &&
		gnrIsFile(filepath.Join(dir, gnrStepsRel))
}

// gnrRootFromExe —— py `REPO` 的同语义映射（★ 两边文件住**不同深度**）：
//
//	py  REPO = dirname^3(abspath(__file__)) —— 件住 <root>/scripts/gates/<件>.py（三级）
//	Go  bin/zerg 住 <root>/bin/zerg（两级）⇒ dirname^2(exe) == <root>
//
// 标记不在盘 ⇒ ok=false（**不得误判**：临时目录里的测试二进制会落到这条 ⇒ 交回退路）。
func gnrRootFromExe(exe string) (string, bool) {
	root := filepath.Dir(filepath.Dir(exe)) // <root>/bin/<名> ⇒ <root>
	if gnrIsRootDir(root) {
		return root, true
	}
	return "", false
}

// gnrSelfFileAbs —— 本件源码的绝对路径（编译期落进制品的 `__file__` 等价面）。
// 在**包初始化时取一次**（此刻 cwd 仍是运行入口目录 · ✗ 不与后续 cwd 变化耦合）。
//
//	· `go test` / 本机 `go build`（无 `-trimpath`）⇒ 绝对路径；
//	· 正式制品用 `-trimpath` 构建 ⇒ 相对路径（`runtime.Caller` 的已知行为）⇒ 该回退面失效；
//	  正式制品的**主路**是 `gnrRootFromExe`（`-trimpath` 不影响 `os.Executable()`）⇒ 不受影响。
var gnrSelfFileAbs = func() string {
	if _, f, _, ok := runtime.Caller(0); ok {
		if filepath.IsAbs(f) {
			return f
		}
		if a, err := filepath.Abs(f); err == nil {
			return a
		}
	}
	return ""
}()

// gnrRootFromSource —— 回退①：测试二进制住临时目录 ⇒ exe 上溯不达仓根 ⇒ 从**本件源码所在目录**
// 上溯找仓根标记（与 cwd 无关 ⇒ 仍满足「同一条命令在任何目录同结果」）。
func gnrRootFromSource() (string, bool) {
	if gnrSelfFileAbs == "" {
		return "", false
	}
	d := filepath.Dir(gnrSelfFileAbs) // <root>/core/cmd/zerg
	for i := 0; i < 8; i++ {
		if gnrIsRootDir(d) {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", false
}

// gnrDefaultRoot —— py `REPO`（与 cwd 无关）的 Go **同语义**实现。
// 次序：① exe 上溯两级（正式制品 `bin/zerg` 走这条 · 与 py `__file__` 上溯三级同结果）；
// ② 本件源码位置回退（测试二进制走这条）；③ cwd 上溯（前两路皆判不了时的兜底 · 与旧行为同）；
// ④ 最后回落 cwd（判不了不猜 —— 与旧行为逐字同）。
func gnrDefaultRoot() string {
	if exe, err := os.Executable(); err == nil {
		cands := []string{exe}
		if resolved, e2 := filepath.EvalSymlinks(exe); e2 == nil && resolved != exe {
			cands = append(cands, resolved)
		}
		for _, c := range cands {
			if r, ok := gnrRootFromExe(c); ok {
				return r
			}
		}
	}
	if r, ok := gnrRootFromSource(); ok {
		return r
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := wd
	for i := 0; i < 8; i++ {
		if gnrIsRootDir(d) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return wd
}

const gnrUsage = "usage: check-newfile-registry.py [-h] [--repo REPO] [--baseline BASELINE]\n" +
	"                                 [--dump-baseline DUMP_BASELINE]\n" +
	"                                 [--inventory INVENTORY] [--new-file NEW_FILE]\n" +
	"                                 [--strict] [--check-missing] [--paths PATHS]\n" +
	"                                 [--outdir OUTDIR] [--json SECTIONS] [--yes]\n" +
	"                                 [--self-test] [--sync]\n"

// gnrHelp —— py `argparse --help` 的 stdout 逐字节内嵌（含末尾换行 · 新旗标 `--check-missing` /
// `--paths` / `--outdir` · 与旧旗标 `--json SECTIONS` / `--yes`）。
// ★ 机械拦阻（✗ 不再靠人记住同步）：测件 `TestGnrHelpParityWithPy` 现跑 `/usr/bin/python3 <py 件> -h|--help|--zzz`
// 与 `gnrHelp` 逐字节对拍（stdout+stderr+rc 三面）并钉住正文 sha256 ⇒ py 正文一改，该断言必红。
const gnrHelp = gnrUsage +
	"\n" +
	"门76 新件登记完整性：件级 × 六张登记表 ＋ .sh 门面清册（只报告 · 起手不接闸）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --repo REPO           被查仓根（默认本仓）\n" +
	"  --baseline BASELINE   冻结基线（仓外生成件 · --dump-baseline 生成）\n" +
	"  --dump-baseline DUMP_BASELINE\n" +
	"                        把现读件集写成基线（仓外路径）后退出；**默认干跑不写**，带 `--yes` 才真落\n" +
	"  --inventory INVENTORY\n" +
	"                        面④ 仓外台账 tsv（不给 ⇒ 面④ 判不了 · 需人签）\n" +
	"  --new-file NEW_FILE   直接指分母（可重复 · 两态取证用）\n" +
	"  --strict              阻断档：R1–R5 任一 > 0 ⇒ rc=1（默认报告档 rc=0）\n" +
	"  --check-missing       查缺档（只读）：三合并式（tracked改动 ∥ untracked ∥\n" +
	"                        ignored）**任一非空即红** · 隔离 HEAD 工作树（退出必清）· 退码 0/1/2\n" +
	"  --paths PATHS         查缺档判据路径（逗号分隔 · 默认 scripts）\n" +
	"  --outdir OUTDIR       查缺档报告落点：**生成前先清空该目录**（防旧件残留看着一致）· 只跟 `--check-missing`\n" +
	"                        连用\n" +
	"  --json SECTIONS       机器面 JSON（逗号分隔断面：faces = 登记面逐面状态（六张登记表 ＋ ⑨.sh门面清册）；self\n" +
	"                        = 自指那一段） · 给了就只出 JSON（人面向 stdout 让位）\n" +
	"  --yes                 写面确认：只有 `--dump-baseline` 会写件，且**默认干跑不写**；带 `--yes`\n" +
	"                        才真落（本件比「默认真写」的写器更保守）\n" +
	"  --self-test           只跑合成夹具自检（不扫真仓）\n" +
	"  --sync                补齐档（B1-4）：默认干跑；只写纯派生面（面① 占位行）；面③⑤⑧ 只报「该跑哪条正门」；显式\n" +
	"                        `--yes` 才真写且 write-temp+fsync+rename\n"

func gnrUsageErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, gnrUsage)
	fmt.Fprintf(stderr, "check-newfile-registry.py: error: %s\n", msg)
	return 2
}

// checkNewfileRegistryGateMain —— 入口（= py `main()`）。
func checkNewfileRegistryGateMain(stdout, stderr io.Writer, argv []string) int {
	repo, repoSet := "", false
	baseline, baselineSet := "", false
	dump, dumpSet := "", false
	inv, invSet := "", false
	jsonSel, jsonSet := "", false
	paths, outdir := "", ""
	checkMissing := false
	yes := false
	var newFiles []string
	strict, selfTest, help, sync := false, false, false, false
	unrec := []string{}
	need := func(i int) (string, int, bool) {
		if i+1 < len(argv) {
			return argv[i+1], i + 1, true
		}
		return "", i, false
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			help = true
		case a == "--self-test":
			selfTest = true
		case a == "--sync":
			sync = true
		case a == "--strict":
			strict = true
		case a == "--check-missing":
			checkMissing = true
		case a == "--paths":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --paths: expected one argument")
			}
			paths, i = v, ni
		case strings.HasPrefix(a, "--paths="):
			paths = a[len("--paths="):]
		case a == "--outdir":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --outdir: expected one argument")
			}
			outdir, i = v, ni
		case strings.HasPrefix(a, "--outdir="):
			outdir = a[len("--outdir="):]
		case a == "--yes":
			yes = true
		case a == "--repo":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --repo: expected one argument")
			}
			repo, repoSet, i = v, true, ni
		case strings.HasPrefix(a, "--repo="):
			repo, repoSet = a[len("--repo="):], true
		case a == "--baseline":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --baseline: expected one argument")
			}
			baseline, baselineSet, i = v, true, ni
		case strings.HasPrefix(a, "--baseline="):
			baseline, baselineSet = a[len("--baseline="):], true
		case a == "--dump-baseline":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --dump-baseline: expected one argument")
			}
			dump, dumpSet, i = v, true, ni
		case strings.HasPrefix(a, "--dump-baseline="):
			dump, dumpSet = a[len("--dump-baseline="):], true
		case a == "--inventory":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --inventory: expected one argument")
			}
			inv, invSet, i = v, true, ni
		case strings.HasPrefix(a, "--inventory="):
			inv, invSet = a[len("--inventory="):], true
		case a == "--new-file":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --new-file: expected one argument")
			}
			newFiles = append(newFiles, v)
			i = ni
		case strings.HasPrefix(a, "--new-file="):
			newFiles = append(newFiles, a[len("--new-file="):])
		case a == "--json":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --json: expected one argument")
			}
			jsonSel, jsonSet, i = v, true, ni
		case strings.HasPrefix(a, "--json="):
			jsonSel, jsonSet = a[len("--json="):], true
		case strings.HasPrefix(a, "-") && a != "-":
			unrec = append(unrec, a)
		default:
			unrec = append(unrec, a)
		}
	}
	if help {
		fmt.Fprint(stdout, gnrHelp)
		return 0
	}
	if len(unrec) > 0 {
		return gnrUsageErr(stderr, "unrecognized arguments: "+strings.Join(unrec, " "))
	}
	// 用法错（rc=2 · fail-closed）：旗标组合不合法，先判、不执行任何动作（判序照 py `main()`）。
	if yes && !dumpSet && !sync {
		fmt.Fprint(stderr, "⛔ 用法错（rc=2）：`--yes` 只跟 `--dump-baseline`/`--sync` 连用（本件**默认干跑不写**）\n")
		return 2
	}
	if sync && (dumpSet || checkMissing) {
		fmt.Fprint(stderr, "⛔ 用法错（rc=2）：`--sync` 不与 `--dump-baseline`/`--check-missing` 同用\n")
		return 2
	}
	if jsonSet && dumpSet {
		fmt.Fprint(stderr, "⛔ 用法错（rc=2）：`--json` 只跟报告档连用（不与 `--dump-baseline` 同用）\n")
		return 2
	}
	if outdir != "" && !checkMissing {
		fmt.Fprint(stderr, "⛔ 用法错（rc=2）：`--outdir` 只跟 `--check-missing` 连用\n")
		return 2
	}
	if checkMissing && dumpSet {
		fmt.Fprint(stderr, "⛔ 用法错（rc=2）：`--check-missing` 不与 `--dump-baseline` 同用\n")
		return 2
	}
	if selfTest {
		return gnrSelfTest(stdout)
	}
	if !repoSet {
		repo = gnrDefaultRoot()
	}
	root := gnrAbs(repo)
	if sync { // ④ B1-4 补齐档：只写纯派生面；面③⑤⑧ 只报「该跑哪条正门」
		if jsonSet {
			fmt.Fprint(stderr, "⛔ 用法错（rc=2）：`--sync` 不与 `--json` 同用（补齐档无人面 JSON 断面）\n")
			return 2
		}
		return gnrRunSync(stdout, stderr, root, yes, baseline)
	}
	if checkMissing {
		// `--check-missing` 的机器面断面只认 `chk`（非空且逐字）
		csel := []string{}
		for _, s := range strings.Split(jsonSel, ",") {
			if t := strings.TrimSpace(s); t != "" {
				csel = append(csel, t)
			}
		}
		badSel := len(csel) == 0
		for _, s := range csel {
			if s != "chk" {
				badSel = true
			}
		}
		if jsonSet && badSel {
			fmt.Fprintf(stderr, "⛔ 用法错（rc=2）：`--check-missing` 的 `--json` 只认断面 chk；不认：%s\n", jsonSel)
			return 2
		}
		cpaths := []string{}
		for _, p := range strings.Split(paths, ",") {
			if t := strings.TrimSpace(p); t != "" {
				cpaths = append(cpaths, t)
			}
		}
		return gnrRunCheckMissing(stdout, root, cpaths, outdir, jsonSet)
	}
	if !baselineSet {
		baseline = gnrDefaultBaseline()
	}
	if dumpSet {
		return gnrDumpBaseline(stdout, stderr, root, dump, yes)
	}
	nfSet := len(newFiles) > 0
	if jsonSet {
		return gnrRunJSON(stdout, stderr, root, newFiles, nfSet, inv, invSet, strict, baseline, jsonSel)
	}
	return gnrRun(stdout, root, newFiles, nfSet, inv, invSet, strict, baseline, baselineSet)
}
