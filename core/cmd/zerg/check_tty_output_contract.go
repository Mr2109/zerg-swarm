// check_tty_output_contract.go —— 判据件 `scripts/gates/check-tty-output-contract.py`（341 行）的 **Go 真搬**
// （v2.5.14 · M2 · 真·待搬清单第二十一枚 · 入口 `checkTtyOutputContractGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的口径差异面与未覆盖面）。
//
// 本件形态（照实）：**真起子进程 ＋ 真造 TTY**。py 本体 `run_pair()` 用 `sh -c` 跑目标两遍
// （非 TTY 档 / `script -q <transcript> sh -c …` 造真 PTY 档），再逐字节比两态 stdout。
// ⇒ 它**不是纯静态件**：判断面落在「同一目标在两态下的 stdout 字节」上，而两态都由**本件的子进程**产出。
// ★ 可对拍性来源：目标命令（`--target-cmd`）由**显式入参旋钮**给出 ⇒ 夹具里给它**确定性合成命令**即可
// 让两态输出逐字可复现（真仓默认目标 = `bin/zerg repo status --json head` · 现读 rc=0 且连跑两发同 sha）。
//
// 外部真源真身路径（**逐字** · py docstring :3-14 引用）：
//
//	`Zerg-内部文档/项目文档/v2.5.11/任务清单-缺口收口-20260923.md:72`（`T11`）
//	`调研-缺口-门禁证据-20260923.md:128`（`R-15` 行）
//	★ 本件**不读**这两处（py 也不读 —— 只在 docstring 里逐字引用）⇒ 搬运面为「零外部树读取」。
//
// 选它 / 不选它的理由：
//
//	✓ 退码 0/1/2 三档齐 · 默认档**不写仓内件**（`--self-test` 只在 `tempfile.mkdtemp()` 里造件，
//	  本件 `defer os.RemoveAll` 自回收 ⇒ 前后 delta=0）。
//	⚠ 外部工具依赖：`script`（造真 TTY）· `sh`。py `shutil.which("script") is None` ⇒ 退 2（缺件不给结论）⇒
//	  本件用 `exec.LookPath("script")` 逐字对齐。工具链指纹写进未覆盖面。
//	⚠ 本件**真跑子进程**（`sh` / `script`）⇒ 对拍台必须两侧同 cwd / 同 env / 同 argv，并指向仓外夹具目标。
//
// 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:3100`（门㉖ · tri 档））；
//	· **不**删 `scripts/gates/check-tty-output-contract.py` 原件（退场规则：**对拍通过后才**退场 · 逐件点名）；
//	· **已接线**：入口 `checkTtyOutputContractGateMain` 由 `gateCheckGoEntry["check-tty-output-contract"]` 分派（`zerg gate check check-tty-output-contract`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkTtyOutputContractGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 `--json` 面归一后逐字同 / 1 有命中 / 2 不给结论（跑不出 · 空转 · 假 TTY · 目标>1 · 自检不过）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ═══════════════════════ 出厂声明行 (a)：默认根定位差异 ═══════════════════════
//
//	py：`HERE = dirname(abspath(__file__))`；`REPO_ROOT = dirname(dirname(HERE))` ⇒ 从脚本所在
//	`scripts/gates/` **上溯两层**到仓根（与 cwd 无关）。Go：无 `__file__` 语境 ⇒ 只能取 `os.Getwd()`。
//	⇒ 「脚本位置 ≠ cwd」时 **路径推导面不同**（✗ 不声称逐字对齐）。同真仓 + cwd=仓根 ⇒ 逐字相同；
//	  对拍台据此把 py 副本放 `<fixture>/scripts/gates/` 且 Go 探针 cwd=`<fixture>`，令两侧根都 = fixture。
//
// ═══════════════════════ 出厂声明行 (b)：自指检查 ═══════════════════════
//
//	★ 本件落在 **`check-error-kinds` 的 R3 用法面**（它扫 `core/cmd/zerg/*.go`）⇒ 件头/正文**一律**
//	不写「连续引号原串」形态：本门**零 setErr 调用**；JSON 键名若需提及一律**描述式或前缀拼串**。
//	写完自跑 R3 正则（`setErr` 紧跟左括号加引号串 / 引号键名加冒号加引号串）现读确认 **0/0**（见收尾读数）。
//	· 本件**不读**任何仓内/仓外树（零遍历、零 glob）⇒ 不落在目录遍历类判据的面内；
//	· 本件含的字符串字面量 = 文案 + 命令串（`bin/zerg repo status --json head`）⇒ 无绝对私有路径字面量。
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

// ctocProg —— argparse 的 prog（= basename(sys.argv[0]) · 现读 py 直接跑脚本 ⇒ 脚本名）。
const ctocProg = "check-tty-output-contract.py"

// ctocHelpSha256 —— `ctocHelp`（本件 Go 半真源）的 sha256（GAP-20261001-61 golden）。
// ★ 内化（判据件退役前置 ② · GAP-20261007-16）：真源 = 下面的 `ctocHelp` 常量，不再以退役中的
// check-tty-output-contract.py 的 `-h` 输出为源；此字面量是 Go 半的冻结回归值，由
// ctocHelpNativeSha256() 在 Go 内自证（见 _test.go help_golden_internalized · 抽掉即红）。
const ctocHelpSha256 = "38f41f105c170d1a59837a0ae2d64ab84829ba77b9b3f3eeb93cedf4a4e3b343"

// ctocHelpNativeSha256 —— 从 Go 半真源 `ctocHelp` 现算 sha256（✗ 不读 py）：把「金标的真源」
// 从退役的 py 收回到 Go 自身 ⇒ py 退役后仍可自证。
func ctocHelpNativeSha256() string {
	sum := sha256.Sum256([]byte(ctocHelp))
	return hex.EncodeToString(sum[:])
}

// ctocUsageHead —— argparse 报错时的 `usage:` 两行（与 help 首两行逐字同）。
const ctocUsageHead = "usage: " + ctocProg + " [-h] [--target-cmd TARGET_CMD]\n" +
	"                                    [--json JSON] [--self-test] [--list-rules]\n"

// ctocHelp —— `-h/--help` 正文（现读复刻 · 398 字节 · sha256 见 ctocHelpSha256）。
const ctocHelp = ctocUsageHead +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --target-cmd TARGET_CMD\n" +
	"                        换目标（可重复出现；**但一次只许 1 条** —— 多给 ⇒ rc=2）\n" +
	"  --json JSON\n" +
	"  --self-test\n" +
	"  --list-rules\n"

// ── 判据表：**默认只一条**（「跑两遍的步数 ≤ 1」是判据，不是建议）────────────────────

// ctocTarget —— 判据表一行（与 py dict 四键同形）。
type ctocTarget struct{ ID, Name, Cmd, Why string }

var ctocDefaultTargets = []ctocTarget{
	{
		ID:   "OC1",
		Name: "输出契约档 · `--json` 六键包封（只读 · 本机状态）",
		Cmd:  "bin/zerg repo status --json head",
		Why: "它走完整的六键包封（`schema/kind/items/meta/warnings/truncated`）+ 人面在 stderr ⇒ " +
			"正好把「机器面 vs 人面」两态分开：`--json` 面必须两态逐字相同，人面允许不同。",
	},
}

// ctocMaxTargets —— 判据「跑两遍的步数 ≤ 1」的机械落点：一次跑的目标数上限。
const ctocMaxTargets = 1

// ctocN1Prefix —— `script` 自己写进 transcript 的回显前缀 `^D\b\b`（N1 归一白名单第一条）。
var ctocN1Prefix = []byte("^D\x08\x08")

// ctocTimeoutSecs —— 子进程超时（py `TIMEOUT = 120`）。
const ctocTimeoutSecs = 120

// ctocJSONFields —— 本门机器读面字段表（py `JSON_FIELDS` · **唯一真源**：印表与拒收同读这一处）。
var ctocJSONFields = []string{"rc", "target", "cmd", "step_runs", "max_targets", "equal_raw",
	"equal_norm", "normalize_used", "rc_n", "rc_t", "hits", "note"}

// ── py 语义小工具（逐条对齐 CPython）────────────────────────────────────────────

// ctocRepoRoot —— 声明行 (a)：py 从 `__file__` 上溯两层（scripts/gates → 仓根）⟷ Go 取 cwd。
func ctocRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// ctocPyDecodeReplace —— py `bytes.decode("utf-8", "replace")`。
// ★ 语义差（已入未覆盖面）：CPython 按**非法序列**替换一个 U+FFFD，本件按**非法字节**逐个替换 U+FFFD
// ⇒ 只有「差异上下文里含非法 UTF-8」时才分叉；真仓/夹具该面均为合法 UTF-8。
func ctocPyDecodeReplace(b []byte) string {
	var sb strings.Builder
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			sb.WriteRune(0xFFFD)
			i++
			continue
		}
		sb.Write(b[i : i+size])
		i += size
	}
	return sb.String()
}

// ctocPyStrRepr —— py `repr(str)`（judge 用 `%r` 打差异上下文 ⇒ 逐字复刻）。
func ctocPyStrRepr(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var sb strings.Builder
	sb.WriteByte(quote)
	for _, r := range s {
		switch {
		case byte(r) == quote && r < 0x80:
			sb.WriteByte('\\')
			sb.WriteByte(quote)
		case r == '\\':
			sb.WriteString("\\\\")
		case r == '\n':
			sb.WriteString("\\n")
		case r == '\r':
			sb.WriteString("\\r")
		case r == '\t':
			sb.WriteString("\\t")
		case r < 0x20 || r == 0x7f:
			sb.WriteString(fmt.Sprintf("\\x%02x", r))
		case r < 0x7f:
			sb.WriteRune(r)
		default:
			// 非 ASCII：printable 原样，非 printable 走 \x / \u / \U（py repr 口径）。
			if isPrintableRune(r) {
				sb.WriteRune(r)
			} else if r < 0x100 {
				sb.WriteString(fmt.Sprintf("\\x%02x", r))
			} else if r < 0x10000 {
				sb.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				sb.WriteString(fmt.Sprintf("\\U%08x", r))
			}
		}
	}
	sb.WriteByte(quote)
	return sb.String()
}

// isPrintableRune —— py `str.isprintable()` 的近似（C0/C1 控制面即不可打印；其余按 Unicode 字母/符号放行）。
func isPrintableRune(r rune) bool {
	if r < 0x20 || (r >= 0x7f && r <= 0xa0) {
		return false
	}
	if r == 0xad {
		return false
	}
	return true
}

// ctocInList —— 成员判定（保序、纯线性）。
func ctocInList(s string, xs []string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// ctocBacktick —— `" · ".join("` + 反引号 + `%s` + 反引号 + "`" for x in xs)`。
func ctocBacktick(xs []string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = "`" + x + "`"
	}
	return strings.Join(parts, " · ")
}

// ── py `run_pair(cmd, timeout)` ──────────────────────────────────────────────

// ctocRun —— run_pair 的产出（与 py dict 同形 · note 存在即「不给结论」档）。
type ctocRun struct {
	dir     string
	cmd     string
	note    string
	hasNote bool
	outN    []byte
	outT    []byte
	errN    []byte
	errT    []byte
	rcN     int
	rcT     int
	isattyN string
	isattyT string
	secs    float64
}

// ctocErrTimeout —— 子进程超时的哨兵错（py `subprocess.TimeoutExpired`）。
var ctocErrTimeout = errors.New("ctoc-timeout")

// ctocRunShell —— 跑 `argv`（cwd = 仓根）· 返回 (returncode, err)。err == ctocErrTimeout ⇒ 超时。
func ctocRunShell(argv []string, timeout int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	c.Dir = ctocRepoRoot()
	var so, se bytes.Buffer
	c.Stdout = &so
	c.Stderr = &se
	err := c.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return 0, ctocErrTimeout
	}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), nil
		}
		return 0, err
	}
	return 0, nil
}

// ctocRead —— py `rd(p)`：读得到 ⇒ (bytes, true)；读不到 ⇒ (nil, false)。
func ctocRead(p string) ([]byte, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	return b, true
}

// ctocReadOrEmpty —— py `rd(p) or b""`。
func ctocReadOrEmpty(p string) []byte {
	if b, ok := ctocRead(p); ok {
		return b
	}
	return []byte{}
}

// ctocIsattyProbe —— py `(rd(mk) or b"").strip().decode() or "取不到"`。
func ctocIsattyProbe(p string) string {
	b, ok := ctocRead(p)
	if !ok {
		return "取不到"
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "取不到"
	}
	return s
}

// ctocRunPair —— 跑两态 → ctocRun（同一份 harness 供自检与真目标用）。
func ctocRunPair(cmd string, timeout int) ctocRun {
	d, err := os.MkdirTemp("", "zerg-tty-")
	r := ctocRun{cmd: cmd, dir: d}
	if err != nil {
		r.note = fmt.Sprintf("非 TTY 跑不起来：%s", err)
		r.hasNote = true
		return r
	}
	defer os.RemoveAll(d) // ★ A 类自回收：与 py 的 shutil.rmtree(ignore_errors=True) 对齐

	capN := filepath.Join(d, "cap_nontty")
	capT := filepath.Join(d, "cap_tty")
	errN := filepath.Join(d, "err_nontty")
	errT := filepath.Join(d, "err_tty")
	mkN := filepath.Join(d, "isatty_nontty")
	mkT := filepath.Join(d, "isatty_tty")

	// 非 TTY：stdout → 文件（isatty(1)=0）· stderr → 文件
	innerN := fmt.Sprintf("%s >%s 2>%s; [ -t 1 ] && echo 1 >%s || echo 0 >%s", cmd, capN, errN, mkN, mkN)
	// TTY：`script -q` 给目标进程一个 PTY ⇒ stdout 是 PTY（isatty(1)=1）· stderr 仍落文件
	innerT := fmt.Sprintf("%s 2>%s; [ -t 1 ] && echo 1 >%s || echo 0 >%s", cmd, errT, mkT, mkT)

	t0 := time.Now()
	rcN, rerrN := ctocRunShell([]string{"sh", "-c", innerN}, timeout)
	if rerrN == ctocErrTimeout {
		r.note = fmt.Sprintf("非 TTY 跑超时（>%ds）", timeout)
		r.hasNote = true
		return r
	}
	if rerrN != nil {
		r.note = fmt.Sprintf("非 TTY 跑不起来：%s", rerrN)
		r.hasNote = true
		return r
	}
	if _, err := exec.LookPath("script"); err != nil {
		r.note = "`script` 不在 PATH ⇒ 造不出真 TTY（缺件不给结论）"
		r.hasNote = true
		return r
	}
	rcT, rerrT := ctocRunShell([]string{"script", "-q", capT, "sh", "-c", innerT}, timeout)
	if rerrT == ctocErrTimeout {
		r.note = fmt.Sprintf("TTY 跑超时（>%ds）", timeout)
		r.hasNote = true
		return r
	}
	if rerrT != nil {
		r.note = fmt.Sprintf("TTY 跑不起来：%s", rerrT)
		r.hasNote = true
		return r
	}

	outN, okN := ctocRead(capN)
	outT, okT := ctocRead(capT)
	if !okN || !okT {
		r.note = "一侧没落 transcript（缺件）"
		r.hasNote = true
		return r
	}
	r.outN, r.outT = outN, outT
	r.errN = ctocReadOrEmpty(errN)
	r.errT = ctocReadOrEmpty(errT)
	r.rcN, r.rcT = rcN, rcT
	r.isattyN = ctocIsattyProbe(mkN)
	r.isattyT = ctocIsattyProbe(mkT)
	r.secs = time.Since(t0).Seconds()
	return r
}

// ── py `normalize_tty(raw)` ──────────────────────────────────────────────────

// ctocNormalizeTTY —— TTY 侧归一：只剥 PTY 层那两条（N1 script 前缀 · N2 ONLCR）。返回 (bytes, 用了哪几条)。
func ctocNormalizeTTY(raw []byte) ([]byte, []string) {
	used := []string{}
	b := raw
	if bytes.HasPrefix(b, ctocN1Prefix) {
		b = b[len(ctocN1Prefix):]
		used = append(used, "N1 script 前缀 ^D\\b\\b")
	}
	if bytes.Contains(b, []byte("\r\n")) {
		b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
		used = append(used, "N2 PTY ONLCR（\\r\\n → \\n）")
	}
	return b, used
}

// ── py `first_diff(a, b)` ────────────────────────────────────────────────────

// ctocDiff —— 首个差异偏移 + 两侧原样上下文（不猜、不概括）。
type ctocDiff struct {
	offset int
	aLen   int
	bLen   int
	aCtx   string
	bCtx   string
}

func ctocSlice30(b []byte, i int) []byte {
	lo := i - 30
	if lo < 0 {
		lo = 0
	}
	hi := i + 30
	if hi > len(b) {
		hi = len(b)
	}
	if lo > hi {
		lo = hi
	}
	return b[lo:hi]
}

func ctocFirstDiff(a, b []byte) ctocDiff {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return ctocDiff{
		offset: i,
		aLen:   len(a),
		bLen:   len(b),
		aCtx:   ctocPyDecodeReplace(ctocSlice30(a, i)),
		bCtx:   ctocPyDecodeReplace(ctocSlice30(b, i)),
	}
}

// ── py `judge(run)` ─────────────────────────────────────────────────────────

// ctocInfo —— judge 的 info（非 note 档才有）。★ `present` 区分 py 的「空 dict `{}`」与「有值 dict」：
// 空 dict 档（假 TTY · 空转）下 equal_raw/equal_norm/rc_equal 键**不在场**（`info.get` → None），
// 与「在场且 false」是两态 ⇒ 零值 struct 不能兼表两者。
type ctocInfo struct {
	present       bool
	normalizeUsed []string
	equalRaw      bool
	equalNorm     bool
	rcEqual       bool
	rcN           int
	rcT           int
	diff          *ctocDiff
}

// ctocJudge —— 三档判定（纯函数 · 自检与真目标同一份）。
func ctocJudge(run ctocRun) (int, []string, *ctocInfo) {
	hits := []string{}
	if run.hasNote {
		return 2, []string{"不给结论：" + run.note}, nil
	}
	if run.isattyN != "0" || run.isattyT != "1" {
		return 2, []string{fmt.Sprintf("不给结论：探针证不出真两态（非 TTY isatty(1)=%s · TTY isatty(1)=%s）",
			run.isattyN, run.isattyT)}, &ctocInfo{}
	}
	normT, used := ctocNormalizeTTY(run.outT)
	// 空转判据看**归一后**：TTY 侧的 `script` 前缀不是目标的产出，不许拿它冒充「有输出」（零输出不算绿）
	if len(bytes.TrimSpace(run.outN)) == 0 && len(bytes.TrimSpace(normT)) == 0 {
		return 2, []string{"不给结论：两侧 stdout 归一后都是零字节（TTY 侧只剩 `script` 前缀）⇒ 空转"}, &ctocInfo{}
	}
	info := &ctocInfo{
		present:       true,
		normalizeUsed: used,
		equalRaw:      bytes.Equal(run.outN, run.outT),
		equalNorm:     bytes.Equal(run.outN, normT),
		rcEqual:       run.rcN == run.rcT,
		rcN:           run.rcN,
		rcT:           run.rcT,
	}
	if !info.equalNorm {
		d := ctocFirstDiff(run.outN, normT)
		info.diff = &d
		hits = append(hits, fmt.Sprintf("`--json` 面**归一后仍不同**（首个差异在偏移 %d · 非 TTY %d 字节 / TTY %d 字节）",
			d.offset, d.aLen, d.bLen))
		hits = append(hits, "   非 TTY："+ctocPyStrRepr(d.aCtx))
		hits = append(hits, "   TTY   ："+ctocPyStrRepr(d.bCtx))
	}
	if !info.rcEqual {
		hits = append(hits, fmt.Sprintf("两侧**退码不同**（非 TTY rc=%d · TTY rc=%d）—— 输出契约含退码",
			run.rcN, run.rcT))
	}
	if len(hits) == 0 {
		return 0, hits, info
	}
	return 1, hits, info
}

// ── py `--json <字段>` 的**字段名打错不许静默 / 不许炸堆栈**（缺口 `Q-176`）────────────

// ctocRefuseUnknownJSONFields —— `--json <字段>` 里有不认的名 ⇒ rc=2 ＋ 印可选字段表；没给 / 全合法 ⇒ nil。
func ctocRefuseUnknownJSONFields(stderr io.Writer, argv []string) *int {
	var want []string
	for i, x := range argv {
		switch {
		case x == "--json":
			v := ""
			if i+1 < len(argv) {
				v = argv[i+1]
			}
			want = append(want, strings.Split(v, ",")...)
		case strings.HasPrefix(x, "--json="):
			want = append(want, strings.Split(strings.SplitN(x, "=", 2)[1], ",")...)
		}
	}
	var fields []string
	for _, f := range want {
		if s := strings.TrimSpace(f); s != "" {
			fields = append(fields, s)
		}
	}
	var bad []string
	for _, f := range fields {
		if !ctocInList(f, ctocJSONFields) {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", ctocBacktick(bad))
	fmt.Fprintf(stderr, "   可选字段表：%s\n", ctocBacktick(ctocJSONFields))
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开")
	rc := 2
	return &rc
}

// ── argparse 等价解析 ────────────────────────────────────────────────────────

type ctocArgs struct {
	targets   []ctocTarget
	json      string
	selfTest  bool
	listRules bool
}

// ctocArgErr —— argparse 报错：usage 两行 + `prog: error: MSG`（均落 stderr · rc=2）。
func ctocArgErr(stderr io.Writer, msg string) {
	fmt.Fprint(stderr, ctocUsageHead)
	fmt.Fprintf(stderr, "%s: error: %s\n", ctocProg, msg)
}

// ctocMatchLong —— argparse 的长选项前缀缩写匹配（allow_abbrev 默认 True）。
// 返回 (canonical, ambiguous, ok)。
func ctocMatchLong(name string) (string, bool, bool) {
	known := []string{"--help", "--target-cmd", "--json", "--self-test", "--list-rules"}
	for _, k := range known {
		if k == name {
			return k, false, true
		}
	}
	var matches []string
	for _, k := range known {
		if strings.HasPrefix(k, name) {
			matches = append(matches, k)
		}
	}
	if len(matches) == 1 {
		return matches[0], false, true
	}
	if len(matches) > 1 {
		return "", true, false
	}
	return "", false, false
}

// ctocParseArgs —— 逐字对齐 py 的 `argparse.ArgumentParser` 面。
// stdout 命中 `-h/--help` ⇒ 印 help 并返回 (nil, 0)；报错 ⇒ 印 usage+error 并返回 (nil, 2)。
func ctocParseArgs(stdout, stderr io.Writer, argv []string) (*ctocArgs, int) {
	a := &ctocArgs{}
	var unknown []string
	i := 0
	afterSep := false
	for i < len(argv) {
		x := argv[i]
		if afterSep {
			unknown = append(unknown, x)
			i++
			continue
		}
		if x == "--" {
			afterSep = true
			i++
			continue
		}
		if x == "-h" || x == "--help" {
			fmt.Fprint(stdout, ctocHelp)
			return nil, 0
		}
		if strings.HasPrefix(x, "--") {
			name := x
			inline := ""
			hasInline := false
			if j := strings.IndexByte(x, '='); j >= 0 {
				name = x[:j]
				inline = x[j+1:]
				hasInline = true
			}
			canon, amb, ok := ctocMatchLong(name)
			switch {
			case amb:
				ctocArgErr(stderr, "ambiguous option: "+x)
				return nil, 2
			case !ok:
				unknown = append(unknown, x)
			case canon == "--help":
				fmt.Fprint(stdout, ctocHelp)
				return nil, 0
			case canon == "--target-cmd":
				var v string
				if hasInline {
					v = inline
				} else {
					if i+1 >= len(argv) {
						ctocArgErr(stderr, "argument --target-cmd: expected one argument")
						return nil, 2
					}
					v = argv[i+1]
					i++
				}
				a.targets = append(a.targets, ctocTarget{
					ID: fmt.Sprintf("OC%d", len(a.targets)+1), Name: "命令行给定", Cmd: v, Why: "—"})
			case canon == "--json":
				var v string
				if hasInline {
					v = inline
				} else {
					if i+1 >= len(argv) {
						ctocArgErr(stderr, "argument --json: expected one argument")
						return nil, 2
					}
					v = argv[i+1]
					i++
				}
				a.json = v
			case canon == "--self-test":
				if hasInline {
					ctocArgErr(stderr, "argument --self-test: ignored explicit argument '"+inline+"'")
					return nil, 2
				}
				a.selfTest = true
			case canon == "--list-rules":
				if hasInline {
					ctocArgErr(stderr, "argument --list-rules: ignored explicit argument '"+inline+"'")
					return nil, 2
				}
				a.listRules = true
			}
			i++
			continue
		}
		unknown = append(unknown, x)
		i++
	}
	if len(unknown) > 0 {
		ctocArgErr(stderr, "unrecognized arguments: "+strings.Join(unknown, " "))
		return nil, 2
	}
	return a, 0
}

// ── py `list_rules()` ───────────────────────────────────────────────────────

func ctocListRules(stdout io.Writer) int {
	fmt.Fprintln(stdout, "输出契约 · TTY/非 TTY 两态（R-15）· 真源：任务清单-缺口收口-20260923.md:72 · 调研-缺口-门禁证据-20260923.md:128")
	fmt.Fprintln(stdout, "判据：`--json` 面两跑**逐字相同**（TTY 差异只允许在人面）· 跑两遍的步数 **≤1**")
	fmt.Fprintln(stdout, "TTY 侧手搓法（本仓源件逐字）：`script -q /dev/null …` ⇒ 本件用 `script -q <transcript> sh -c '<目标> 2><err>'`")
	fmt.Fprintln(stdout, "归一白名单（只两条 · 都是 PTY 层）：N1 `script` 前缀 · N2 ONLCR（\\r\\n→\\n）")
	fmt.Fprintf(stdout, "一次跑的目标数上限：%d（给多了 ⇒ rc=2，拒绝把成本 ×2）\n", ctocMaxTargets)
	for _, t := range ctocDefaultTargets {
		fmt.Fprintf(stdout, "  %s %s\n     命令：%s\n     为什么：%s\n", t.ID, t.Name, t.Cmd, t.Why)
	}
	fmt.Fprintln(stdout, "退码：0 逐字相同 · 1 有命中 · 2 不给结论（跑不出/空转/假 TTY/目标>1/自检不过）")
	return 0
}

// ── py `self_test()`（正/负成对 · 合成目标 · 含 must-fail 真红）────────────────────

func ctocPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func ctocPyTupleIntBool(a int, b bool) string {
	return fmt.Sprintf("(%d, %s)", a, ctocPyBool(b))
}

func ctocPyTupleBoolBool(a, b bool) string {
	return fmt.Sprintf("(%s, %s)", ctocPyBool(a), ctocPyBool(b))
}

func ctocSelfTest(stdout io.Writer) int {
	ok := 0
	var bad []string
	var log []string
	check := func(name, got, want string) {
		if got == want {
			ok++
			log = append(log, fmt.Sprintf("   ✓ %s ⇒ %s", name, got))
		} else {
			bad = append(bad, name)
			log = append(log, fmt.Sprintf("   ✗ %s ⇒ 期望 %s，实得 %s", name, want, got))
		}
	}

	same := `printf '{"a":1}\n'`
	ttyColor := `if [ -t 1 ]; then printf '\033[32m{"a":1}\033[0m\n'; else printf '{"a":1}\n'; fi`
	silent := "true"

	rc, hits, info := ctocJudge(ctocRunPair(same, ctocTimeoutSecs))
	check("正控 · 两态同形的合成目标 ⇒ rc=0", ctocPyTupleIntBool(rc, info != nil && info.equalNorm), "(0, True)")
	check("正控 · harness 真给了两态（非 TTY 0 / TTY 1）", ctocPyBool(info != nil), "True")

	rc, hits, info = ctocJudge(ctocRunPair(ttyColor, ctocTimeoutSecs))
	check("负控(must-fail) · TTY 会加 ANSI ⇒ 必须真红 rc=1", strconv.Itoa(rc), "1")
	check("负控(must-fail) · 命中必须点名「归一后仍不同」", ctocPyBool(ctocAnyContains(hits, "归一后仍不同")), "True")
	check("负控(must-fail) · 该例 `raw` 与 `norm` 都该不同", ctocPyTupleBoolBool(info != nil && info.equalRaw, info != nil && info.equalNorm), "(False, False)")

	rc, _, _ = ctocJudge(ctocRunPair(silent, ctocTimeoutSecs))
	check("空转 · 零字节两边 ⇒ 不给结论 rc=2", strconv.Itoa(rc), "2")

	// 探针证不出两态 ⇒ 不给结论（**合成**：直接把 isatty 读数改坏，证明判据有牙）
	badRun := ctocRunPair(same, ctocTimeoutSecs)
	badRun.isattyT = "0"
	rc, _, _ = ctocJudge(badRun)
	check("负控 · 假 TTY（探针说 isatty(1)=0）⇒ 不给结论 rc=2", strconv.Itoa(rc), "2")

	check("判据 · 一次跑两遍的目标数上限 = 1（「跑两遍的步数 ≤1」）", strconv.Itoa(ctocMaxTargets), "1")
	check("判据表 · 默认目标恰好 1 条", strconv.Itoa(len(ctocDefaultTargets)), "1")

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

func ctocAnyContains(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}

// ── py `json.dumps(..., ensure_ascii=False)`（**逐字节**对齐 CPython 分隔符 `", "` / `": "`）──

// ctocPyJSONString —— json.dumps 的字符串编码（ensure_ascii=False）。
func ctocPyJSONString(s string) string {
	var sb strings.Builder
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
	return sb.String()
}

// ctocPyJSONValue —— 只吃本件会出现的三种值（nil / bool / int / string / []string）。
func ctocPyJSONValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case string:
		return ctocPyJSONString(x)
	case []string:
		parts := make([]string, len(x))
		for i, s := range x {
			parts[i] = ctocPyJSONString(s)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return "null"
}

type ctocKV struct {
	k string
	v any
}

func ctocJSONObj(pairs []ctocKV) string {
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = ctocPyJSONString(p.k) + ": " + ctocPyJSONValue(p.v)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// ctocJSONFor —— 复刻 py `json.dumps({k: obj[k] for k in want} if want else obj)`。
func ctocJSONFor(want []string, rc int, t ctocTarget, run ctocRun, hits []string, info *ctocInfo) string {
	var equalRaw, equalNorm, used, rcN, rcT any
	if info != nil && info.present {
		equalRaw = info.equalRaw
		equalNorm = info.equalNorm
		used = info.normalizeUsed
		rcN = info.rcN
		rcT = info.rcT
	}
	full := []ctocKV{
		{"rc", rc}, {"target", t.ID}, {"cmd", t.Cmd}, {"step_runs", 1}, {"max_targets", ctocMaxTargets},
		{"equal_raw", equalRaw}, {"equal_norm", equalNorm}, {"normalize_used", used},
		{"rc_n", rcN}, {"rc_t", rcT}, {"hits", hits}, {"note", run.note},
	}
	if len(want) == 0 {
		return ctocJSONObj(full)
	}
	byKey := map[string]any{}
	for _, p := range full {
		byKey[p.k] = p.v
	}
	var sel []ctocKV
	seen := map[string]bool{}
	for _, k := range want {
		if seen[k] {
			continue
		}
		seen[k] = true
		sel = append(sel, ctocKV{k, byKey[k]})
	}
	return ctocJSONObj(sel)
}

// ── py `main()` ─────────────────────────────────────────────────────────────

// checkTtyOutputContractGateMain —— 入口（= py `main()`）。
func checkTtyOutputContractGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：`--json <字段>` 打错 ⇒ 用法错 rc=2（先于位置参数解析）。
	if rc := ctocRefuseUnknownJSONFields(stderr, argv); rc != nil {
		return *rc
	}
	a, rc := ctocParseArgs(stdout, stderr, argv)
	if a == nil {
		return rc
	}

	if a.listRules {
		return ctocListRules(stdout)
	}
	if a.selfTest {
		fmt.Fprintln(stdout, "── 输出契约 TTY 两态（R-15）· 自检（合成目标 · 含 must-fail 真红）──")
		return ctocSelfTest(stdout)
	}

	targets := a.targets
	if len(targets) == 0 {
		targets = ctocDefaultTargets
	}
	if len(targets) > ctocMaxTargets {
		fmt.Fprintf(stdout, "不给结论：一次给了 **%d** 个目标 ⇒ 会把「跑两遍」的步数从 1 顶到 %d\n",
			len(targets), len(targets))
		fmt.Fprintln(stdout, "判据逐字：跑两遍的步数 **≤1**（不许让全量档为此跑两遍 · 成本 × 2）")
		fmt.Fprintln(stdout, "⇒ rc=2")
		return 2
	}

	t := targets[0]
	fmt.Fprintln(stdout, "── 输出契约 · TTY/非 TTY 两态（R-15）──")
	fmt.Fprintf(stdout, "目标：%s %s\n", t.ID, t.Name)
	fmt.Fprintf(stdout, "命令：%s（cwd=%s）\n", t.Cmd, ctocRepoRoot())
	fmt.Fprintf(stdout, "为什么：%s\n", t.Why)
	fmt.Fprintf(stdout, "跑两遍的步数：1（判据上限 %d）\n", ctocMaxTargets)
	run := ctocRunPair(t.Cmd, ctocTimeoutSecs)
	jrc, hits, info := ctocJudge(run)

	if run.hasNote {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "   ✗ "+run.note)
	} else {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "── 两态实测（现跑）──")
		fmt.Fprintf(stdout, "   非 TTY：stdout %d 字节 · stderr %d 字节 · rc=%d · 探针 isatty(1)=%s\n",
			len(run.outN), len(run.errN), run.rcN, run.isattyN)
		fmt.Fprintf(stdout, "   TTY   ：stdout %d 字节 · stderr %d 字节 · rc=%d · 探针 isatty(1)=%s\n",
			len(run.outT), len(run.errT), run.rcT, run.isattyT)
		fmt.Fprintf(stdout, "   归一用了：%s\n", ctocJoinOrDash(info.normalizeUsed))
		rawEq := "不同"
		if info.equalRaw {
			rawEq = "相同"
		}
		fmt.Fprintf(stdout, "   原始字节判等：%s（PTY 层必然改行尾 ⇒ 这一格**只作参照**）\n", rawEq)
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "── 判据：`--json` 面（stdout）归一后逐字相同 ──")
		if info.equalNorm {
			fmt.Fprintln(stdout, "   ✓ 逐字相同（TTY 差异不在机器面）")
		} else {
			fmt.Fprintln(stdout, "   ✗ 归一后仍不同 ⇒ 命中")
		}
		for _, h := range hits {
			fmt.Fprintln(stdout, "   ✗ "+h)
		}
		if info.present {
			if !info.rcEqual {
				fmt.Fprintf(stdout, "   （附加判据：两侧退码 %d / %d）\n", info.rcN, info.rcT)
			}
		} else {
			// ★ C 类真口子（已交回入账）：py 在这一格取 `info["rc_n"]` ⇒ KeyError ⇒ 回溯 + rc=1。
			//   本件按新口径 fail-closed：stdout 保对齐，rc=2，✗ 不影随崩（未覆盖面逐条明写）。
			return 2
		}
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "⇒ rc=%d\n", jrc)

	if a.json != "" {
		var want []string
		for _, f := range strings.Split(a.json, ",") {
			if s := strings.TrimSpace(f); s != "" {
				want = append(want, s)
			}
		}
		fmt.Fprintln(stdout, ctocJSONFor(want, jrc, t, run, hits, info))
	}
	return jrc
}

// ctocJoinOrDash —— py `"、".join(used or []) or "（无 —— 本来就是逐字同）"`。
func ctocJoinOrDash(used []string) string {
	if len(used) == 0 {
		return "（无 —— 本来就是逐字同）"
	}
	return strings.Join(used, "、")
}
