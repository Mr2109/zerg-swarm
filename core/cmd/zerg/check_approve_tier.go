// check_approve_tier.go —— 门禁件 `scripts/gates/check-approve-tier.py` 的 **Go 真搬**
// （M2 · 甲案 C 类 · 真·待搬清单第十八枚）。
//
// 为什么有这件：py 真身 `scripts/gates/check-approve-tier.py`（460 行 · sha16 2752a3f4108a07ce）。
// 口径：`D1`（2.5.11 人签 D 批）「档位可读面」的判据面 —— 判 T1 `approve ls` 人面六列按序在 ·
// T2 `approve show` 人面八行按序在 · T3 字段面九键逐字等且 `strength`/`key_id_in_use` 不在必填集 ·
// T4 `--json <未知字段>` rc=2（另打 `--json strength` 读数 · **不判**）· T5 词汇面（禁 `token`/`PIN` ·
// 许 `passphrase`）· T6 `M-20` 现跑 `approve keygen --strength token` ⇒ rc=2 且真状态目录零写。
// 本件**只判、不改**：它读现网状态目录（只读）与合成夹具；除临时目录外一个字节都不写。
//
// ── 外部真源（档位面 · 现读真身）────────────────────────────────────────────────
//
//	「审批档位/等级」的**真源在仓内**，且是**在册闭集** —— `core/cmd/zerg/family_approve.go`：
//	  · :66 `var approveFields = []string{...}` = **九键必填面**（本件 T3 逐字对拍的**唯一真源**）；
//	  · :87 `var approveOptionalFields` = 可选面（`strength`/`key_id_in_use`/`ttl_seconds`/`expires_at`/`expired`）；
//	  · :90 `const approveStrengthPassphrase = "passphrase"`；:81 注释明列**闭集** = `passphrase`（生效档）
//	    / `token`（将来档）—— 只增不改；
//	  · :96 `approveStrengthInUse()` = 档位强度的**唯一取值口**（人面与机器面同取 ⇒ 两处不漂）。
//	⇒ 本件 T3 的 `--root` 相对路径 `core/cmd/zerg/family_approve.go` 就是**在册真源真身**；
//	  `--json strength` 那一格只是**读数**（`P26` 未拍 ⇒ 不判）；闭集本身**不落进本件** ✗
//	  （本件不复制档位闭集，只对拍上述真源）。
//
// ── 出厂声明 (a) · 默认根定位差异 ───────────────────────────────────────────────
//
//	py 从 `os.path.dirname(os.path.abspath(__file__))` 上溯两级得 `DEFAULT_ROOT`（= 仓根）；
//	Go 无 `__file__` 语境 ⇒ 用同族件的同一法门 `ccatDefaultRoot()`（cwd 上溯至最近的含
//	`scripts/gates/` 的目录，找不到回落 cwd）。常见档（cwd = 仓根）二者同值；
//	「脚本位置 ≠ cwd 上溯根」时**不声称逐字对齐**（本单对拍一律显式 `--root` 钉同一夹具 ⇒ 不受影响）。
//	prog 名：py argparse 取 `basename(sys.argv[0])`（本件规范名 = `check-approve-tier.py`）；
//	Go 件内置同名字面量 `ccatProg`（✗ 不用 os.Args[0] —— 那会让探针名进 usage/错误行）。
//
// ── 出厂声明 (b) · 自指检查 ─────────────────────────────────────────────────────
//
//	判「否」：本件**不落在自己的扫描面内** —— 本门只读仓内 `core/cmd/zerg/family_approve.go`、
//	`<root>/bin/zerg`（只跑）与合成夹具，三者都不含 `core/cmd/zerg/*.go` 的用法面扫描。
//	★ 他门自指形态已逐条消解：`check-error-kinds` 的 R3 用法面正则吃两种**连续引号原串** ——
//	  ① `setErr(` 后紧跟引号串；② 键名字面（`kin` 拼 `d` 那种）后跟冒号再跟引号串。
//	  ⇒ 本件**两种都不写**：本门本就无 error-kind 面；夹具 JSON 只用非该键的键名，
//	  一律走「前缀 + 字面」拼串，✗ 不落可被扫的连续原串。现读复核：R3 正则对本件与测试件命中 **0/0**。
//	件内零绝对路径字面量（无仓根/家目录一类路径字面量 · 唯一绝对常量是测试件里钉死的解释器）。
//
// ── A 类真口子（本件：**无**）──────────────────────────────────────────────────
//
//	py 的三处 `tempfile.mkdtemp`（self_test :252 · scan :359 · 沙箱 :394）**都在 finally 里
//	`shutil.rmtree`** ⇒ 不留残件 ⇒ 「A 类形态（py 自检夹具不回收）」本件**不适用**；Go 侧逐条对齐
//	`defer os.RemoveAll`。测试件自带 delta=0 回归牙（删 `defer` ⇒ FAIL）。
//
// ── C 类：py 最外层裸 `sys.exit(main())` ⇒ 未捕异常 = 回溯 rc=1（**不是**「不给结论 rc=2」）
//
//	Go 侧把「夹具建不出 / 子进程起不来 / 真源读不到 / 解不出」四态一律 fail-closed 走 `why`
//	分支 ⇒ rc=2；与 py 的 rc=1 逐条分列在未覆盖面（本单只报、不改 py 原件）。
//
// ── 对拍面（甲 = 仓外 py 副本 · 乙 = 由**本件**重建的仓外探针）──────────────────────
//
//	两档 cwd/env/argv 逐字同、显式 `--root` 指向仓外夹具；跑前 `unset PYTHONPATH ZERG_PST_*`。
//	27 档里 26 档 rc/stdout/stderr **逐字节同**；1 档是**已声明口径差**（`--root` 指向**不存在**
//	的目录：py 在 :188 `subprocess.run(..., cwd=<repo>/core)` 抛 `FileNotFoundError` ⇒ 未捕异常
//	rc=1 + 回溯；Go fail-closed ⇒ rc=2 + 逐字「取不到读数」）。
//	含随机后缀的档（T6 沙箱外落件打印了 `zerg-approve-tier-strength-<随机>`）**归一后**严格 == 比，
//	并自证 py 连跑两发原始 stdout 不同（⇒ 归一化不是充数）。
//	夹具里的 `bin/zerg` 是桩（旋钮 `CCAT_STUB_*` 经 env 注入 · 两侧共用同一份 · sha16 见回执）；
//	真制品锁档（现役 `bin/zerg` 只读复制）本单**未**跑运行档 ⇒ 见未覆盖面。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// ── 常量（逐字取自 py 原件 :51-75 / :421）───────────────────────────────────────

// ccatProg —— py argparse 的 prog（= `basename(sys.argv[0])` 的规范名）。
const ccatProg = "check-approve-tier.py"

// ccatVersion —— py `VERSION`（:53），也是 `print(VERSION)` 的先头行。
const ccatVersion = "check-approve-tier.py v1.0.0"

// ccatUsageLine —— argparse 的 usage 行（与 `--root` 缺省值无关 ⇒ 路径无关）。
const ccatUsageLine = "usage: " + ccatProg + " [-h] [--root ROOT] [--self-test] [--no-self-test]"

// ccatHelp —— py `-h` 的 stdout **逐字节**（389 字节 · sha256
// 48575077bd97434aa33405077589f0a48a36054bdaa55338cb8a79b92af5b2a）。
// 现读来源：`/usr/bin/python3 scripts/gates/check-approve-tier.py -h`（Python 3.9.6 ⇒ argparse 用
// `optional arguments:` 那种表头 · 帮助列对齐在 18 列）。
const ccatHelp = "usage: check-approve-tier.py [-h] [--root ROOT] [--self-test] [--no-self-test]\n" +
	"\n" +
	"check-approve-tier.py v1.0.0\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help      show this help message and exit\n" +
	"  --root ROOT     仓根（默认 = 本脚本的上两级）\n" +
	"  --self-test     只跑自检（合成夹具 · 不碰真目标）\n" +
	"  --no-self-test  跳过自检（**只有已自证过的调用方才该用**）\n"

// ── 判定面常量（逐字取自 py 原件 :55-76）────────────────────────────────────────

var ccatExpectedLsCols = []string{"tool", "approver", "approved_at", "scope", "state", "path"}

// ccatKnownLsCols —— 历史已知列名集（`GAP-20260927-29`）：**只增不减**，判「丢了已知列」那一格。
var ccatKnownLsCols = []string{"tool", "approver", "approved_at", "scope", "state", "path", "档"}

var ccatExpectedShowLabels = []string{"工具", "批准者", "签的时间", "范围", "理由", "签名", "判决", "落点"}

// ccatKnownShowLabels —— 历史已知行标签集（`GAP-20260927-29`）：**只增不减**，判「丢了已知标签」。
var ccatKnownShowLabels = []string{"工具", "批准者", "签的时间", "范围", "理由", "签名", "档", "钥匙身份", "有效期", "判决", "落点"}

// ccatExpectedFields —— 九键必填面（真源 = `core/cmd/zerg/family_approve.go:66`）。
var ccatExpectedFields = []string{"tool", "approver", "approved_at", "scope", "note", "sig_alg", "key_id", "state", "path"}

var ccatMustNotBeRequired = []string{"strength", "key_id_in_use"}
var ccatForbiddenTokens = []string{"token", "PIN"}
var ccatAllowedTokens = []string{"passphrase"}

// ccatApproveGoRel —— T3 字段面的**在册真源**（`--root` 相对路径）。
var ccatApproveGoRel = ccatPyJoin("core", "cmd", "zerg", "family_approve.go")

// ccatJSONFields —— 本门机器读面字段表（py :421 · 本版为空 ⇒ 印「无」）。
var ccatJSONFields = []string{}

// ── py 语义小工具（逐条对齐 CPython）────────────────────────────────────────────

// ccatPyIsSpace = py `str.isspace()` 的判据面（★ 含 \x1c-\x1f；Go 的 unicode.IsSpace 不含）。
func ccatPyIsSpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// ccatPyStrip = py `str.strip()`（✗ 不用 strings.TrimSpace —— 它的空白集与 py 不同）。
func ccatPyStrip(s string) string {
	rs := []rune(s)
	i, j := 0, len(rs)
	for i < j && ccatPyIsSpace(rs[i]) {
		i++
	}
	for j > i && ccatPyIsSpace(rs[j-1]) {
		j--
	}
	return string(rs[i:j])
}

// ccatPySplitLines = py `str.splitlines()`（含 \v \f \x1c-\x1e \x85 \u2028 \u2029 · \r\n 归一）。
func ccatPySplitLines(s string) []string {
	rs := []rune(s)
	out := []string{}
	start := 0
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', '\x85', '\u2028', '\u2029':
			out = append(out, string(rs[start:i]))
			start = i + 1
		case '\r':
			out = append(out, string(rs[start:i]))
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}

// ccatPyRepr = py `repr(str)` 的**常用形态**：含 `'` 而不含 `"` ⇒ 双引号包裹，否则单引号；
// 转义 `\` · `\n` · `\r` · `\t` 与包裹引号（其余控制字符原样 · 见未覆盖面）。
func ccatPyRepr(s string) string {
	quote := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = "\""
	}
	var b strings.Builder
	b.WriteString(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if string(r) == quote {
				b.WriteString("\\")
			}
			b.WriteRune(r)
		}
	}
	b.WriteString(quote)
	return b.String()
}

// ccatPyListRepr = py `repr(list)` 的字符串表形态（`['a', 'b']`）。
func ccatPyListRepr(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, ccatPyRepr(x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ccatPyJoin = py `os.path.join`（后件为绝对路径 ⇒ 复位；✗ 不用 filepath.Join —— 它会 Clean）。
func ccatPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	res := parts[0]
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "/") {
			res = p
			continue
		}
		if res == "" || strings.HasSuffix(res, "/") {
			res += p
		} else {
			res += "/" + p
		}
	}
	return res
}

// ccatPyTail = py 切片 `s[-n:]`（按**字符**数，不是字节数）。
func ccatPyTail(s string, n int) string {
	rs := []rune(s)
	if n <= 0 || len(rs) <= n {
		return s
	}
	return string(rs[len(rs)-n:])
}

func ccatStrIn(x string, xs []string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func ccatStrSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ccatRemoveFirst = py `list.remove(x)`（去掉**首个**命中项；没有命中则原样返回）。
func ccatRemoveFirst(xs []string, x string) []string {
	for i, v := range xs {
		if v == x {
			return append(xs[:i:i], xs[i+1:]...)
		}
	}
	return xs
}

// ccatNonEmptyLines = py `[ln for ln in stdout.splitlines() if ln.strip()]`。
func ccatNonEmptyLines(stdout string) []string {
	out := []string{}
	for _, ln := range ccatPySplitLines(stdout) {
		if ccatPyStrip(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}

// ── 判定口（纯函数 · 自检与真目标**共用同一套** ⇒ 没有第二套口径）────────────────

// ccatJudgeOrderedSuperset = py `judge_ordered_superset`：`expected` 必须**按序**在 `actual` 里出现
// （新增项允许 · 换序/缺项 ⇒ 问题行）。返回**非 nil** 空切片（nil 不得兼表「取不到」与「空集合」）。
func ccatJudgeOrderedSuperset(actual, expected []string, what string) []string {
	problems := []string{}
	i := 0
	for _, e := range expected {
		idx := -1
		for k := i; k < len(actual); k++ {
			if actual[k] == e {
				idx = k
				break
			}
		}
		if idx < 0 {
			problems = append(problems, fmt.Sprintf("%s：%s 在该序里找不到（换序或删项 ⇒ 红 · 新增项允许）",
				what, ccatPyRepr(e)))
			continue
		}
		i = idx + 1
	}
	return problems
}

// ccatJudgeColumns = py `judge_columns`（★ py 用 `expected or EXPECTED_LS_COLS` ⇒ 空/缺都回落缺省）。
func ccatJudgeColumns(cols, expected []string) []string {
	if len(expected) == 0 {
		expected = ccatExpectedLsCols
	}
	return ccatJudgeOrderedSuperset(cols, expected, "T1 `approve ls` 列")
}

// ccatJudgeKnownColumnsLost = py `judge_known_columns_lost`（★ py 用 `known is not None` ⇒ 只有 nil 回落）。
func ccatJudgeKnownColumnsLost(cols, known []string) []string {
	if known == nil {
		known = ccatKnownLsCols
	}
	out := []string{}
	for _, c := range known {
		if !ccatStrIn(c, cols) {
			out = append(out, fmt.Sprintf("T1 破：已知列 %s 丢了（改名 / 删列 / 被合并 ⇒ 红 · 已知列名集只增不减）",
				ccatPyRepr(c)))
		}
	}
	return out
}

// ccatJudgeLabels = py `judge_labels`（★ 同 `expected or EXPECTED_SHOW_LABELS`）。
func ccatJudgeLabels(labels, expected []string) []string {
	if len(expected) == 0 {
		expected = ccatExpectedShowLabels
	}
	return ccatJudgeOrderedSuperset(labels, expected, "T2 `approve show` 行")
}

// ccatJudgeKnownLabelsLost = py `judge_known_labels_lost`（★ 只有 nil 回落）。
func ccatJudgeKnownLabelsLost(labels, known []string) []string {
	if known == nil {
		known = ccatKnownShowLabels
	}
	out := []string{}
	for _, c := range known {
		if !ccatStrIn(c, labels) {
			out = append(out, fmt.Sprintf("T2 破：已知行 %s 丢了（改名 / 删行 / 被合并 ⇒ 红 · 已知行标签集只增不减）",
				ccatPyRepr(c)))
		}
	}
	return out
}

// ccatJudgeFields = py `judge_fields`（九键**逐字**等 · `MUST_NOT_BE_REQUIRED` 不得在内）。
func ccatJudgeFields(fields []string) []string {
	problems := []string{}
	if !ccatStrSliceEq(fields, ccatExpectedFields) {
		problems = append(problems, "T3 字段面 ≠ 九键：现读 = "+strings.Join(fields, ","))
	}
	for _, bad := range ccatMustNotBeRequired {
		if ccatStrIn(bad, fields) {
			problems = append(problems, fmt.Sprintf("T3 破：%s 被塞进必填集（`D1` 红线 · 旧消费者会当场坏）",
				ccatPyRepr(bad)))
		}
	}
	return problems
}

// ccatJudgeVocab = py `judge_vocab`（禁词命中 ⇒ 问题行；`passphrase` 不在禁词集 ⇒ 允许）。
func ccatJudgeVocab(text string) []string {
	problems := []string{}
	for _, bad := range ccatForbiddenTokens {
		if strings.Contains(text, bad) {
			problems = append(problems, fmt.Sprintf("T5 破：输出里出现 %s（`M-20` ② 不许）", ccatPyRepr(bad)))
		}
	}
	return problems
}

// ccatParseLsColumns = py `parse_ls_columns`（列面 = 首行按 tab 分格；一行都没有 ⇒ 空切片）。
func ccatParseLsColumns(stdout string) []string {
	lines := ccatNonEmptyLines(stdout)
	if len(lines) == 0 {
		return []string{}
	}
	return strings.Split(lines[0], "\t")
}

// ccatJudgeRowWidths = py `judge_row_widths`（每行格数必须 == 表头格数 · 少一格 ⇒ 红）。
func ccatJudgeRowWidths(stdout string) []string {
	lines := ccatNonEmptyLines(stdout)
	if len(lines) == 0 {
		return []string{"T1 破：`approve ls` 一行都没有（表头也缺）"}
	}
	n := len(strings.Split(lines[0], "\t"))
	out := []string{}
	for i, ln := range lines {
		if w := len(strings.Split(ln, "\t")); w != n {
			out = append(out, fmt.Sprintf("T1 破：第 %d 行格数 %d ≠ 表头 %d", i+1, w, n))
		}
	}
	return out
}

// ccatShowLabelOf = py `re.match(r"^(\S+)\s*:", ln)` 的等价判定（返回 group(1)）。
// 贪心复刻：组 = 从行首起的最长「非 py 空白」串，且其后 `\s*` 恰好接一个 ASCII `:`；
// 引擎先试最长组 ⇒ 从首个非空白段末尾递减回溯（行首是空白 ⇒ 无匹配）。
func ccatShowLabelOf(ln string) (string, bool) {
	rs := []rune(ln)
	if len(rs) == 0 || ccatPyIsSpace(rs[0]) {
		return "", false
	}
	j0 := 0
	for j0 < len(rs) && !ccatPyIsSpace(rs[j0]) {
		j0++
	}
	for e := j0; e >= 1; e-- {
		p := e
		for p < len(rs) && ccatPyIsSpace(rs[p]) {
			p++
		}
		if p < len(rs) && rs[p] == ':' {
			return string(rs[:e]), true
		}
	}
	return "", false
}

// ccatParseShowLabels = py `parse_show_labels`。
func ccatParseShowLabels(stdout string) []string {
	labels := []string{}
	for _, ln := range ccatPySplitLines(stdout) {
		if lab, ok := ccatShowLabelOf(ln); ok {
			labels = append(labels, lab)
		}
	}
	return labels
}

var (
	// py `re.search(r'var approveFields = \[\]string\{([^}]*)\}', src)`
	ccatFieldsRE = regexp.MustCompile(`var approveFields = \[\]string\{([^}]*)\}`)
	// py `re.findall(r'"([^"]+)"', group)`
	ccatQuotedRE = regexp.MustCompile(`"([^"]+)"`)
	// argparse `_negative_number_matcher`
	ccatNegativeNumberRE = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)
)

// ccatParseFieldsFromGo = py `parse_fields_from_go`。
// ★ 两态**分开表意**：解不出来 ⇒ (nil, false)（= 取不到）；解出来但没有引号串 ⇒ ([]string{}, true)
// （= 空集合 · **非 nil**）—— nil slice 不得兼表两态。
func ccatParseFieldsFromGo(src string) ([]string, bool) {
	m := ccatFieldsRE.FindStringSubmatch(src)
	if m == nil {
		return nil, false
	}
	out := []string{}
	for _, mm := range ccatQuotedRE.FindAllStringSubmatch(m[1], -1) {
		out = append(out, mm[1])
	}
	return out, true
}

// ── 现跑（子进程面）────────────────────────────────────────────────────────────

// ccatEnvWith = py `env = dict(os.environ); env[K] = V`（**替换**同名键，不是追加重复项）。
func ccatEnvWith(k, v string) []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, k+"=") {
			continue
		}
		out = append(out, e)
	}
	return append(out, k+"="+v)
}

type ccatRunResult struct {
	rc     int
	rcNone bool
	out    string
	err    string
}

// ccatRunCLI = py `run_cli`（cwd=repo · env 追加 `ZERG_STATE_DIR` · stdout/stderr 分抓）。
// ★ 起不来的两态分开：`exe == ""` 与「进程起不来」都报 rcNone（py 侧后者是未捕异常 ⇒ rc=1 · 见未覆盖面）。
func ccatRunCLI(exe, repo, stateDir string, argv []string) ccatRunResult {
	if exe == "" {
		return ccatRunResult{rcNone: true}
	}
	cmd := exec.Command(exe, argv...)
	cmd.Dir = repo
	cmd.Env = ccatEnvWith("ZERG_STATE_DIR", stateDir)
	var ob, eb strings.Builder
	cmd.Stdout = &ob
	cmd.Stderr = &eb
	err := cmd.Run()
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			return ccatRunResult{rcNone: true}
		}
		return ccatRunResult{rc: ee.ExitCode(), out: ob.String(), err: eb.String()}
	}
	return ccatRunResult{rc: 0, out: ob.String(), err: eb.String()}
}

// ccatResolveBin = py `resolve_bin`：先 `<repo>/bin/zerg`；不在且 `go` 在 PATH ⇒ 现场构建到临时目录
// （★ 产物落调用方的临时目录 · **不碰** `bin/` ✗）。
func ccatResolveBin(repo, tmpdir string) (string, string) {
	exe := ccatPyJoin(repo, "bin", "zerg")
	if _, err := os.Stat(exe); err == nil {
		return exe, ""
	}
	if _, err := exec.LookPath("go"); err != nil {
		return "", "`bin/zerg` 不在且 `go` 不在 PATH ⇒ 取不到读数"
	}
	out := ccatPyJoin(tmpdir, "zerg-approve-tier")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", out, "./cmd/zerg")
	cmd.Dir = ccatPyJoin(repo, "core")
	var eb strings.Builder
	cmd.Stderr = &eb
	err := cmd.Run()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = 1
		}
	}
	msg := fmt.Sprintf("`bin/zerg` 不在且现场构建失败（rc=%d）：%s", rc, ccatPyTail(ccatPyStrip(eb.String()), 200))
	if rc != 0 {
		return "", msg
	}
	if _, err := os.Stat(out); err != nil {
		return "", msg
	}
	return out, "现场构建（`bin/` 不入库）"
}

// ccatJSONString = 夹具写盘时的字符串转义（走 encoding/json ⇒ 与 py `json.dump` 同义；
// 本件八枚取值都无 `<`/`>`/`&` ⇒ 与 py 的 `ensure_ascii=False` 逐字等价）。
func ccatJSONString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

// ccatMakeFixture = py `make_fixture`：一枚**无签名**的批准件（只为取**渲染面**的列 / 行）。
func ccatMakeFixture(stateDir string) error {
	d := ccatPyJoin(stateDir, "approvals")
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	type kv struct{ k, v string }
	pairs := []kv{
		{"tool", "合成件"},
		{"approver", "Mr2109"},
		{"approved_at", "2026-09-22T10:00:00+08:00"},
		{"scope", "*"},
		{"note", "D1 判据件的夹具（无签名 ⇒ 判决那一列照实说「无签名」）"},
		{"sig_alg", "ed25519"},
		{"key_id", "0000000000000000"},
		{"sig", ""},
	}
	var b strings.Builder
	b.WriteString("{\n")
	for i, p := range pairs {
		b.WriteString(" " + ccatJSONString(p.k) + ": " + ccatJSONString(p.v))
		if i < len(pairs)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return os.WriteFile(ccatPyJoin(d, "合成件.json"), []byte(b.String()), 0o644)
}

type ccatReadings struct {
	lsRc       int
	lsOut      string
	lsErr      string
	showRc     int
	showOut    string
	showErr    string
	fields     []string
	unknownRc  int
	unknownOut string
	unknownErr string
	strengthRc int
	strengthEr string
	exe        string
	binNote    string
}

// ccatCollectReadings = py `collect_readings`（顺序逐条对齐：ls → show → 读真源 → unknown → strength）。
func ccatCollectReadings(repo, stateDir, tmpdir string) (*ccatReadings, string) {
	exe, note := ccatResolveBin(repo, tmpdir)
	ls := ccatRunCLI(exe, repo, stateDir, []string{"approve", "ls"})
	if ls.rcNone {
		return nil, "取不到读数：" + note
	}
	show := ccatRunCLI(exe, repo, stateDir, []string{"approve", "show", "合成件"})
	if show.rcNone {
		return nil, "取不到读数：" + note
	}
	goRel := ccatPyJoin(repo, ccatApproveGoRel)
	raw, err := os.ReadFile(goRel)
	if err != nil {
		return nil, fmt.Sprintf("本件读不到 `%s` ⇒ 取不到字段面", ccatApproveGoRel)
	}
	fields, ok := ccatParseFieldsFromGo(string(raw))
	if !ok {
		return nil, fmt.Sprintf("`approveFields` 在 %s 里解不出来 ⇒ 取不到字段面", ccatApproveGoRel)
	}
	unknown := ccatRunCLI(exe, repo, stateDir, []string{"approve", "ls", "--json", "nosuchfield-xyz"})
	strength := ccatRunCLI(exe, repo, stateDir, []string{"approve", "ls", "--json", "strength"})
	return &ccatReadings{
		lsRc: ls.rc, lsOut: ls.out, lsErr: ls.err,
		showRc: show.rc, showOut: show.out, showErr: show.err,
		fields:    fields,
		unknownRc: unknown.rc, unknownOut: unknown.out, unknownErr: unknown.err,
		strengthRc: strength.rc, strengthEr: strength.err,
		exe: exe, binNote: note,
	}, ""
}

// ── 自检（先自证会红，再扫真目标）──────────────────────────────────────────────

// ccatSelfTest = py `self_test`：10 格（① 正控 · ②③④⑤⑦⑧⑨⑩ 负控 · ⑥ 正控）。
// 返回 (collected, 是否给了结论)：读不到读数 ⇒ (false,false)（py 侧回 None · 主流程据此 rc=2）。
func ccatSelfTest(stdout io.Writer, repo string) (bool, bool) {
	okN, badN := 0, 0
	tmp, err := os.MkdirTemp("", "zerg-approve-tier-selftest-")
	if err != nil {
		return false, false
	}
	defer os.RemoveAll(tmp)
	if err := ccatMakeFixture(tmp); err != nil {
		return false, false
	}
	r, why := ccatCollectReadings(repo, tmp, tmp)
	if r == nil {
		fmt.Fprintf(stdout, "⛔ 自检① 取不到读数：%s ⇒ 不给结论\n", why)
		return false, false
	}
	cols := ccatParseLsColumns(r.lsOut)
	labels := ccatParseShowLabels(r.showOut)
	if r.lsRc == 0 && len(ccatJudgeColumns(cols, nil)) == 0 && len(ccatJudgeLabels(labels, nil)) == 0 &&
		len(ccatJudgeFields(r.fields)) == 0 && len(ccatJudgeRowWidths(r.lsOut)) == 0 {
		fmt.Fprintf(stdout, "   ✓ ① 正控：现跑人面六列 / 八行 / 九字段都过（列数 %d · 行标签 %d）\n", len(cols), len(labels))
		okN++
	} else {
		fmt.Fprintf(stdout, "   ✗ ① 正控失败：现跑面就被判红了（列 %s · 行 %s）\n", ccatPyListRepr(cols), ccatPyListRepr(labels))
		badN++
	}
	xcells := []string{}
	for i := 0; i < len(cols)-1; i++ {
		xcells = append(xcells, "x")
	}
	narrow := strings.Join([]string{strings.Join(cols, "\t"), strings.Join(xcells, "\t")}, "\n")
	if len(ccatJudgeRowWidths(narrow)) > 0 {
		fmt.Fprintln(stdout, "   ✓ ⑧ 负控：数据行**少一格** ⇒ 判红")
		okN++
	} else {
		fmt.Fprintln(stdout, "   ✗ ⑧ 负控失败：少一格还判绿")
		badN++
	}
	swap := append([]string{}, cols...)
	if len(swap) >= 2 {
		swap[0], swap[1] = swap[1], swap[0]
	}
	if len(ccatJudgeColumns(swap, nil)) > 0 {
		fmt.Fprintln(stdout, "   ✓ ② 负控：六列**换序** ⇒ 判红")
		okN++
	} else {
		fmt.Fprintln(stdout, "   ✗ ② 负控失败：换序了还判绿")
		badN++
	}
	drop := append([]string{}, cols...)
	lastCol := ccatExpectedLsCols[len(ccatExpectedLsCols)-1]
	if ccatStrIn(lastCol, drop) {
		drop = ccatRemoveFirst(drop, lastCol)
	}
	if len(ccatJudgeColumns(drop, nil)) > 0 {
		fmt.Fprintln(stdout, "   ✓ ③ 负控：**删一列**（必填列 `path`）⇒ 判红")
		okN++
	} else {
		fmt.Fprintln(stdout, "   ✗ ③ 负控失败：删列了还判绿")
		badN++
	}
	lostKnown := ccatKnownLsCols[: len(ccatKnownLsCols)-1 : len(ccatKnownLsCols)-1]
	jSup := ccatJudgeColumns(lostKnown, nil)
	jLost := ccatJudgeKnownColumnsLost(lostKnown, nil)
	if len(jSup) == 0 && len(jLost) > 0 {
		fmt.Fprintf(stdout, "   ✓ ⑨ 负控：**丢一列已知列**（%s）⇒ 新判据判红；同一输入**现有有序超集抓不住**"+
			" (%d 条) —— 本单缺口 %s 的当场证据\n",
			ccatPyRepr(ccatKnownLsCols[len(ccatKnownLsCols)-1]), len(jSup), "GAP-20260927-29")
		okN++
	} else {
		fmt.Fprintf(stdout, "   ✗ ⑨ 负控失败：丢已知列 %s 没按预期分叉（有序超集 %d 条 · 已知列丢失 %d 条）\n",
			ccatPyRepr(ccatKnownLsCols[len(ccatKnownLsCols)-1]), len(jSup), len(jLost))
		badN++
	}
	newLabels := []string{}
	for _, x := range ccatKnownShowLabels {
		if !ccatStrIn(x, ccatExpectedShowLabels) {
			newLabels = append(newLabels, x)
		}
	}
	lostLabel := newLabels[len(newLabels)-1]
	lsup := append([]string{}, labels...)
	if ccatStrIn(lostLabel, lsup) {
		lsup = ccatRemoveFirst(lsup, lostLabel)
	}
	jLSup := ccatJudgeLabels(lsup, nil)
	jLLost := ccatJudgeKnownLabelsLost(lsup, nil)
	if len(jLSup) == 0 && len(jLLost) > 0 {
		fmt.Fprintf(stdout, "   ✓ ⑩ 负控：**丢一行已知行**（%s）⇒ 新判据判红；同一输入**现有有序超集抓不住**"+
			" (%d 条) —— 本单缺口 %s 的当场证据\n", ccatPyRepr(lostLabel), len(jLSup), "GAP-20260927-29")
		okN++
	} else {
		fmt.Fprintf(stdout, "   ✗ ⑩ 负控失败：丢已知行 %s 没按预期分叉（有序超集 %d 条 · 已知行丢失 %d 条）\n",
			ccatPyRepr(lostLabel), len(jLSup), len(jLLost))
		badN++
	}
	ldrop := append([]string{}, labels...)
	lastLabel := ccatExpectedShowLabels[len(ccatExpectedShowLabels)-1]
	if ccatStrIn(lastLabel, ldrop) {
		ldrop = ccatRemoveFirst(ldrop, lastLabel)
	}
	if len(ccatJudgeLabels(ldrop, nil)) > 0 {
		fmt.Fprintf(stdout, "   ✓ ④ 负控：**删一行**（必填标签 `%s`）⇒ 判红\n", lastLabel)
		okN++
	} else {
		fmt.Fprintln(stdout, "   ✗ ④ 负控失败：删行了还判绿")
		badN++
	}
	if len(ccatJudgeVocab(r.lsOut+"\n"+"档：token 触摸\n")) > 0 {
		fmt.Fprintln(stdout, "   ✓ ⑤ 负控：输出里出现 `token` ⇒ 判红（`M-20` ②）")
		okN++
	} else {
		fmt.Fprintln(stdout, "   ✗ ⑤ 负控失败：出现 `token` 还判绿")
		badN++
	}
	if len(ccatJudgeVocab(r.lsOut+"\n"+"档：passphrase（口令）\n")) == 0 {
		fmt.Fprintln(stdout, "   ✓ ⑥ 正控：输出里出现 `passphrase` ⇒ **不**判红（生效档自己 · `D1` 判据④）")
		okN++
	} else {
		fmt.Fprintln(stdout, "   ✗ ⑥ 正控失败：`passphrase` 被当成禁词了")
		badN++
	}
	if len(ccatJudgeFields(append(append([]string{}, r.fields...), "strength"))) > 0 {
		fmt.Fprintln(stdout, "   ✓ ⑦ 负控：字段表里塞 `strength` ⇒ 判红（`D1` 红线）")
		okN++
	} else {
		fmt.Fprintln(stdout, "   ✗ ⑦ 负控失败：塞进必填集还判绿")
		badN++
	}
	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（合成夹具目录已清理 · 真状态目录一个字节未动）\n", okN, badN)
	return badN == 0, true
}

// ── 主（真目标扫描）────────────────────────────────────────────────────────────

// ccatScan = py `scan`。返回退码 0/1/2。
func ccatScan(stdout io.Writer, repo string) int {
	real := ""
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if cand := ccatPyJoin(home, ".zerg", "state"); cand != "" {
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			real = cand
		}
	}
	tmp, err := os.MkdirTemp("", "zerg-approve-tier-")
	if err != nil {
		fmt.Fprintf(stdout, "⛔ 未取数：%s ⇒ **不出结论**（rc=2 · 不许把读不到当绿）\n", err.Error())
		return 2
	}
	defer os.RemoveAll(tmp)
	if err := ccatMakeFixture(tmp); err != nil {
		fmt.Fprintf(stdout, "⛔ 未取数：%s ⇒ **不出结论**（rc=2 · 不许把读不到当绿）\n", err.Error())
		return 2
	}
	r, why := ccatCollectReadings(repo, tmp, tmp)
	if r == nil {
		fmt.Fprintf(stdout, "⛔ 未取数：%s ⇒ **不出结论**（rc=2 · 不许把读不到当绿）\n", why)
		return 2
	}
	problems := []string{}
	cols := ccatParseLsColumns(r.lsOut)
	problems = append(problems, ccatJudgeColumns(cols, nil)...)
	problems = append(problems, ccatJudgeKnownColumnsLost(cols, nil)...)
	problems = append(problems, ccatJudgeRowWidths(r.lsOut)...)
	labels := ccatParseShowLabels(r.showOut)
	problems = append(problems, ccatJudgeLabels(labels, nil)...)
	problems = append(problems, ccatJudgeKnownLabelsLost(labels, nil)...)
	problems = append(problems, ccatJudgeFields(r.fields)...)
	problems = append(problems, ccatJudgeVocab(r.lsOut+"\n"+r.showOut)...)
	fmt.Fprintln(stdout, "── 读数（三处 · 每处带口径）")
	fmt.Fprintf(stdout, "   T1 `approve ls` 列（rc=%d · 首行 = 表头）：%s\n", r.lsRc, strings.Join(cols, " | "))
	fmt.Fprintf(stdout, "   T2 `approve show` 行标签（rc=%d）：%s\n", r.showRc, strings.Join(labels, " | "))
	fmt.Fprintf(stdout, "   T3 `approveFields`（%s：var approveFields）= %s\n", ccatApproveGoRel, strings.Join(r.fields, ","))
	fmt.Fprintf(stdout, "   T4 `--json nosuchfield-xyz` ⇒ rc=%d（要 2）\n", r.unknownRc)
	fmt.Fprintf(stdout, "   ·  读数（**不判** · `P26` 未拍）：`--json strength` ⇒ rc=%d\n", r.strengthRc)
	hit := 0
	for _, b := range ccatForbiddenTokens {
		hit += strings.Count(r.lsOut+"\n"+r.showOut, b)
	}
	fmt.Fprintf(stdout, "   T5 词汇面：禁词 %s 命中 = %d · 允许词 %s\n",
		strings.Join(ccatForbiddenTokens, "/"), hit, strings.Join(ccatAllowedTokens, "/"))
	if r.lsRc != 0 {
		problems = append(problems, fmt.Sprintf("T1 破：`approve ls` 退码 %d（要 0）", r.lsRc))
	}
	if r.showRc != 0 {
		problems = append(problems, fmt.Sprintf("T2 破：`approve show` 退码 %d（要 0）", r.showRc))
	}
	if r.unknownRc != 2 {
		problems = append(problems, fmt.Sprintf("T4 破：`--json <未知字段>` 退码 %d（要 2 · 字段面先判）", r.unknownRc))
	}
	// T6 `M-20` 现跑：`--strength` 今天必须仍是未知旗标（真状态目录零写）。
	sandbox, err := os.MkdirTemp("", "zerg-approve-tier-strength-")
	if err == nil {
		defer os.RemoveAll(sandbox)
		s := ccatRunCLI(r.exe, repo, sandbox, []string{"approve", "keygen", "--strength", "token", "--by", "夹具"})
		if s.rcNone {
			problems = append(problems, fmt.Sprintf("T6 破：取不到 `--strength` 读数（%s）", r.binNote))
		} else if s.rc != 2 {
			problems = append(problems, fmt.Sprintf("T6 破：`approve keygen --strength token` 退码 %d（要 2 · 未知旗标）", s.rc))
		} else {
			first := ""
			if lines := ccatPySplitLines(ccatPyStrip(s.err)); len(lines) > 0 {
				first = lines[0]
			}
			fmt.Fprintf(stdout, "   T6 `M-20` 现跑：`keygen --strength token` ⇒ rc=2（原样：%s）\n", first)
		}
		if sandbox != real {
			if st, err := os.Stat(ccatPyJoin(sandbox, "approvals")); err == nil && st.IsDir() {
				problems = append(problems, fmt.Sprintf("T6 破：沙箱外落了件（%s/approvals）", sandbox))
			}
		}
	}
	if len(problems) > 0 {
		fmt.Fprintf(stdout, "⇒ 门禁红（rc=1）：%d 条\n", len(problems))
		for _, p := range problems {
			fmt.Fprintf(stdout, "   ✗ %s\n", p)
		}
		return 1
	}
	fmt.Fprintln(stdout, "⇒ 全绿（rc=0）：T1–T6 全过 · 漂移 0 条（档位标记的落地形态仍待 `P26` 拍）")
	return 0
}

// ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗
//
//	口径：逐字明说「本门无机器读面」**＋印字段表**（一个字段都没有 ⇒ 也要明说「字段表：无」），
//	退码 = **用法错 `rc=2`**（「机器面缺」与「零命中」两态在机器面上必须分得开）。

// ccatJSONArgs = py `[x for x in argv if x == "--json" or x.startswith("--json=")]`。
func ccatJSONArgs(argv []string) []string {
	out := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			out = append(out, x)
		}
	}
	return out
}

// ccatPrintRefuseJSON = py `refuse_json_without_machine_face` 的三行 stderr。
func ccatPrintRefuseJSON(stderr io.Writer, argv []string) {
	hit := ccatJSONArgs(argv)
	quoted := make([]string, 0, len(hit))
	for _, h := range hit {
		quoted = append(quoted, "`"+h+"`")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(quoted, " · "))
	table := "无（本门只出人读面）"
	if len(ccatJSONFields) > 0 {
		qs := make([]string, 0, len(ccatJSONFields))
		for _, f := range ccatJSONFields {
			qs = append(qs, "`"+f+"`")
		}
		table = strings.Join(qs, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", table)
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
}

// ── 参数面（argparse 等价：四枚长旗标 + `-h` · allow_abbrev=True）────────────────

type ccatOpts struct {
	root       string
	selfTest   bool
	noSelfTest bool
}

var ccatLongOpts = []string{"help", "root", "self-test", "no-self-test"}

// ccatDefaultRoot = 同族件同法门（cwd 上溯至最近的含 `scripts/gates/` 的目录，找不到回落 cwd）。
func ccatDefaultRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := cwd
	for i := 0; i < 12 && d != "" && d != "/"; i++ {
		if st, err := os.Stat(ccatPyJoin(d, "scripts", "gates")); err == nil && st.IsDir() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return cwd
}

// ccatLooksLikeOption = argparse `_parse_optional` 的「这枚 token 会不会被当成选项」判定。
func ccatLooksLikeOption(s string) bool {
	if s == "" || s[0] != '-' {
		return false
	}
	if s == "-" {
		return false
	}
	if ccatNegativeNumberRE.MatchString(s) {
		return false
	}
	return true
}

func ccatUsageError(stderr io.Writer, msg string) {
	fmt.Fprintln(stderr, ccatUsageLine)
	fmt.Fprintf(stderr, "%s: error: %s\n", ccatProg, msg)
}

// ccatParseArgs 返回 (opts, rc, done)：done=false ⇒ 继续走主流程。
func ccatParseArgs(argv []string, stdout, stderr io.Writer) (ccatOpts, int, bool) {
	opts := ccatOpts{root: ccatDefaultRoot()}
	unrec := []string{}
	onlyPositional := false
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if onlyPositional {
			unrec = append(unrec, tok)
			continue
		}
		if tok == "--" {
			// ★ argparse 实测：`--` 没被任何选项吃掉时，它**自己**也进 unrecognized 列表
			// （`["--","foo"]` ⇒ `unrecognized arguments: -- foo`）。
			unrec = append(unrec, tok)
			onlyPositional = true
			continue
		}
		if tok == "-h" {
			fmt.Fprint(stdout, ccatHelp)
			return opts, 0, true
		}
		if strings.HasPrefix(tok, "--") {
			name, val := tok[2:], ""
			hasEq := false
			if k := strings.Index(name, "="); k >= 0 {
				val, name, hasEq = name[k+1:], name[:k], true
			}
			cands := []string{}
			for _, o := range ccatLongOpts {
				if strings.HasPrefix(o, name) {
					cands = append(cands, o)
				}
			}
			if len(cands) > 1 {
				fs := make([]string, 0, len(cands))
				for _, c := range cands {
					fs = append(fs, "--"+c)
				}
				ccatUsageError(stderr, fmt.Sprintf("ambiguous option: %s could match %s", "--"+name, strings.Join(fs, ", ")))
				return opts, 2, true
			}
			if len(cands) == 0 {
				unrec = append(unrec, tok)
				continue
			}
			switch cands[0] {
			case "help":
				if hasEq {
					ccatUsageError(stderr, fmt.Sprintf("argument -h/--help: ignored explicit argument %s", ccatPyRepr(val)))
					return opts, 2, true
				}
				fmt.Fprint(stdout, ccatHelp)
				return opts, 0, true
			case "self-test", "no-self-test":
				if hasEq {
					ccatUsageError(stderr, fmt.Sprintf("argument --%s: ignored explicit argument %s", cands[0], ccatPyRepr(val)))
					return opts, 2, true
				}
				if cands[0] == "self-test" {
					opts.selfTest = true
				} else {
					opts.noSelfTest = true
				}
			case "root":
				if !hasEq {
					if i+1 >= len(argv) || ccatLooksLikeOption(argv[i+1]) {
						ccatUsageError(stderr, "argument --root: expected one argument")
						return opts, 2, true
					}
					val = argv[i+1]
					i++
				}
				opts.root = val
			}
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			// argparse 的单横簇：`-h` 后跟多余字符 ⇒ 「ignored explicit argument 'x'」。
			if len(tok) > 2 && tok[1] == 'h' {
				ccatUsageError(stderr, fmt.Sprintf("argument -h/--help: ignored explicit argument %s", ccatPyRepr(tok[2:])))
				return opts, 2, true
			}
			unrec = append(unrec, tok)
			continue
		}
		unrec = append(unrec, tok)
	}
	if len(unrec) > 0 {
		ccatUsageError(stderr, "unrecognized arguments: "+strings.Join(unrec, " "))
		return opts, 2, true
	}
	return opts, 0, false
}

// checkApproveTierGateMain 是本门的 Go 入口（= py `main()` 的直调面）。
func checkApproveTierGateMain(stdout, stderr io.Writer, argv []string) int {
	// ★ Q-176：给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后 rc=2（**先于**参数解析）。
	if len(ccatJSONArgs(argv)) > 0 {
		ccatPrintRefuseJSON(stderr, argv)
		return 2
	}
	opts, code, done := ccatParseArgs(argv, stdout, stderr)
	if done {
		return code
	}
	repo, err := filepath.Abs(opts.root)
	if err != nil {
		repo = opts.root
	}
	fmt.Fprintln(stdout, ccatVersion)
	if !opts.noSelfTest {
		collected, ok := ccatSelfTest(stdout, repo)
		if !collected || !ok {
			fmt.Fprintln(stdout, "⛔ 自检不过（或取不到读数）⇒ 不给结论（rc=2）")
			return 2
		}
	}
	if opts.selfTest {
		return 0
	}
	fmt.Fprintf(stdout, "── 真目标：%s\n", repo)
	return ccatScan(stdout, repo)
}
