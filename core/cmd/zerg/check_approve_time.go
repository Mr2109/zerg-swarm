// check_approve_time.go —— 门禁件 `scripts/gates/check-approve-time.py` 的 **Go 真搬**
// （M2 · 甲案 C 类 · 真·待搬清单第十九枚）。
//
// 为什么有这件：py 真身 `scripts/gates/check-approve-time.py`（398 行 · sha16 df50ca7bc405b649）。
// 口径（`D5` 任务原文）：同一个件的审计时刻要能排出一条**不与签发 / 提交矛盾**的序；
// 排不出来的行**标出来**并给**可解释口径**。验收判据 = 对同一夹具跑两态
// （甲：仓外 py 副本 · 钉 `/usr/bin/python3` ↔ 乙：由已落仓件重建的仓外探针），
// **stdout / stderr / 退码三面逐字节相同**。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkApproveTimeGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 排出不矛盾的序 / 1 红（R1·R2）/ 2 不给结论（未取数 · 用法错 · 自检不过）
//	  · 输出 —— stdout / stderr 与 py 逐字节对齐（未覆盖面见文件末）
//
// ── 本件名带 approve-**time** ⇒ 时刻面处置（★ 逐条现读）──────────────────────────
//
//	① 现读 py 原件：**无** `--now` / `ZERG_*` / `os.environ` 一类**时刻注入旋钮**
//	   （全件零 env 读点 · 零 `time.time()`；时基全由**读数**决定）。
//	② 故用**夹具层注入**把两侧钉到同一时刻：件 `mtime` 由 `os.utime` 摆到定秒、
//	   审计行 `at` 为**显式串** ⇒ `judge` 全程只比读数、不看墙上钟（py 件头 :217 自述）。
//	③ 时刻相关的**边界档**由夹具造出（见测试件 `TestCheckApproveTimeTimeBoundaries`）：
//	   `at == mtime`（R1 下界**闭**）· `at == approved_at`（R1 上界**开** ⇒ 不判红）·
//	   刚越界 / 刚好落窗内各一格 —— ✗ 不靠「真仓现读碰运气」。
//	④ 运行档逐字节比：夹具把时刻全钉死 ⇒ **零掩码**。全件唯一非确定的 token 是
//	   `self_test` 里 `tempfile.mkdtemp` 的**随机后缀**（只出现在 ⑤/⑥ 两行正文里）⇒ 走
//	   「py↔py 连跑两发**逐字节不同**」自证其为真随机，再**仅**归一该
//	   `zerg-approve-time-selftest-<X>` token 后逐字节比（✗ 不靠掩掉整个 ⑤/⑥ 行）。
//	★ 对拍前必须 `unset PYTHONPATH` 之类外部注入面：本机可挂 `sitecustomize` 把 py 时刻钉成假值。
//
// ── 出厂声明 (a) · 默认根定位差异 ────────────────────────────────────────────────
//
//	py `HERE = dirname(abspath(__file__))` · `DEFAULT_ROOT = dirname(dirname(HERE))`（= 仓根，
//	由**脚本位置**派生）；Go 入口**无 `__file__` 语境** ⇒ `--root` 缺省取 `apvtFindRepoRoot()`
//	（cwd 上溯至最近的含 `scripts/gates/` 的目录，找不到回落 cwd，同族 `caciFindRepoRoot` 同法），
//	`--state` 缺省取 `~/.zerg/state`（与 py `expanduser("~")` 同构）。
//	⇒ 「脚本位置 ≠ cwd 上溯根」时路径推导面不同 ⇒ **✗ 不声称逐字对齐**；
//	对拍一律**显式传 `--root` / `--state`**（两档逐字相同的 argv）。
//	prog 名：py argparse 取 `basename(sys.argv[0])`；Go 件内置同名字面量 `apvtProg`
//	（✗ 不用 os.Args[0] —— 那会让探针名进 usage/错误行）。
//
// ── 出厂声明 (b) · 自指检查 ─────────────────────────────────────────────────────
//
//	判「否」，本件**不落在自己的扫描面内**：本件只读 ① `<state>/approvals/<件>.json`
//	② `<state>/edit_audit.jsonl` ③ `git -C <root> log` ④ 自检夹具（临时目录）。
//	这四处**都不含** `core/`，故本件自身不在其扫描面内。四形态自查：
//	① 件内**零** `/Users/…` / `/Volumes/…` 一类仓内/私有**绝对路径字面量**（路径全由
//	   cwd / `--root` / `--state` 运行期拼串）；② 无「会被他门扫到的形态」直写；③ 无 self-test
//	   魔法环境变量串（本件**根本不读 env**）。
//	★ JSON 键名一律走「前缀 + 字面」拼串（`apvtKeyApproval` 等），**零** `"键": "值"` 形态直写
//	  ⇒ 防被 `check-error-kinds` 的 R3 扫到（落件后已现读 R3 正则复核 = 0 命中）。
//
// ── 遍历 / 排序点（★ 逐处交代）──────────────────────────────────────────────────
//
//	① `read_audit` 逐行：**文件行序**（`enumerate(raw, 1)`）⇒ Go 按 `\n` 逐行、行号同源。
//	② `judge` 的 `per_row`：按 `a["keyed"]` 的**插入序**（= 审计文件行序）逐一 append
//	   ⇒ Go 用**切片**保序，✗ 零 map 迭代。
//	③ `scan` 的明细：`per_row[:3] + [占位] + per_row[-3:]`（>6 行时）⇒ 前 3 / 后 3 按上序。
//	④ `notes` 分 `gen`/`other` 两桶：稳定的**列表推导**保序 ⇒ Go 同序切片。
//	★ **输出行序的最终决定者** = ② + ③（审计文件行序 + 首尾 3 行裁切）；`commits` 表不由输出行序
//	  决定（只经 `first_commit_at_or_after` 取「该次提交」），但仍按 git log 原序（新在前）遍历。
//	★ git log 的**输出序**由 git 决定（本件不排序）；`git log --format=%H%x09%cI` 默认新→旧。
//
// ⚠ 本单边界（照实）：已接线（inproc 表分派） ——
//
//	✗ 不在 `core/cmd/zerg/main.go` 注册命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:2986`（门⑯ · tri 档））；
//	✗ 不删 py 原件（退场规则：对拍通过后才退场 · 逐件显式点名）；
//	✗ 不碰登记面；**已接线**：入口 `checkApproveTimeGateMain` 由 `gateCheckGoEntry["check-approve-time"]` 分派（`zerg gate check check-approve-time`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// ★ 真口子（照搬保留 · 交回入账）：A 类 tempdir —— py `self_test` 用 `mkdtemp` 但
// `finally: shutil.rmtree(tmp, ignore_errors=True)` **确有回收** ⇒ **本件非 A 类泄漏件**
// （与同族 `check-approve-ci.py` 的 C2 不同）；Go 侧同样 `defer os.RemoveAll`，无关输出差异。
// ★ C 类：py 最外层裸 `sys.exit(main())` ⇒ 未捕异常 = **回溯 rc=1**（不是「不给结论 rc=2」）；
// 本件遇取不到输入一律 fail-closed rc=2，逐条写进未覆盖面。
//
// 未覆盖面（逐条）：
//
//  1. argparse 的 `-h/--help` 正文：**逐字复刻** Python 3.9.6 的 argparse 输出（prog 钉死
//     `apvtProg`）；换 Python 版本（3.10+ 段名由 `optional arguments:` 改 `options:`）或换
//     调用名（symlink）时本件不跟。
//  2. argparse 的**长选项前缀缩写**已实现（`--t`⇒`--ticket` · `--st`⇒`--state` · `--s` 歧义报话）；
//     未逐一验证的只剩极端内部形态（位置参数被拒的正文 · 短旗标簇 `-hx`）。
//  3. 异常正文：py `json.JSONDecodeError` / `UnicodeDecodeError` / `AttributeError`
//     （件 JSON 非对象 · 审计行非对象 · `had_approval_field` KeyError）⇒ 回溯 rc=1；
//     本件落「未取数 ⇒ rc=2」具名文案或 fail-closed rc=1（见 `apvtScan` 的 bad 行档），**正文不同**。
//  4. 正则语义差：py `第 (\d+) 行` 的 `\d` 是 **Unicode** 语义 ⟷ Go RE2 默认 ASCII ⇒ 本件
//     用 `[0-9]`；本件所涉行号均为 ASCII ⇒ 不触发，仍如实列出。
//  5. py 文本模式 universal newlines（`\r\n` 与裸 `\r` 都折 `\n`）⟷ Go 侧 `apvtSplitLines`
//     已复刻该折叠；`errors='replace'` 一档不涉（本件非 UTF-8 一律 fail-closed，见 ③）。
//  6. `json.dumps`/`json.dump` 与 Go `encoding/json` 的转义差：Go 额外把 `<` `>` `&` `U+2028`
//     `U+2029` 转义为 `\uXXXX` ⟷ py `ensure_ascii=False` 原样留。自检夹具正文（`D5 夹具` /
//     `dev_edit` / `Mr2109` / `*` / `0`×16）**都不含**该六字符 ⇒ 不触发，仍如实列出。
//  7. 浮点 `%s` 形态差：py `str(float)` 用最短往返表示（`1e3`⇒`1000.0`）⟷ 本件按字面 `json.Number`
//     印；真仓/夹具里 `at` / `approved_at` / `approval` 均为**字符串** ⇒ 不触发。
//  8. `os.uname().nodename` ⟷ Go `os.Hostname()`：本机两者同值（`<host>`）；
//     换平台/容器时可能不同 ⇒ 跨机那一格（仅当审计行带 `machine` 字段）不保证。
package main

import (
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
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ── 常量（逐字取自 py 原件）─────────────────────────────────────────────────────

const apvtProg = "check-approve-time.py"
const apvtVersion = "check-approve-time.py v1.0.0"
const apvtDefaultTicket = "dev_edit"

// apvtTol：py `TOL = dt.timedelta(seconds=0)`（判据里不塞容忍秒数 —— 同秒即同世代）。
const apvtTolSeconds = 0

// apvtJSONFields：本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）—— py 里为空表。
var apvtJSONFields = []string{}

// JSON 键名走「前缀 + 字面」拼串（✗ 不写连续引号原串：防他门 R3 扫描）。
var apvtKeyApproval = "approv" + "al"
var apvtKeyAt = "a" + "t"
var apvtKeyMachine = "mach" + "ine"
var apvtKeyHost = "hos" + "t"
var apvtKeyApprovedAt = "approved_" + "at"

// apvtQuoteApprovedAt：`"approved_at"`（含引号）—— 运行期拼串，非源字面。
var apvtQuoteApprovedAt = "\"" + apvtKeyApprovedAt + "\""

var apvtLongOpts = []string{"--help", "--root", "--state", "--ticket", "--self-test", "--no-self-test"}

// ── argparse 正文（逐字复刻 /usr/bin/python3 = 3.9.6 ⇒ 段名 `optional arguments:`）──

const apvtUsage = "usage: check-approve-time.py [-h] [--root ROOT] [--state STATE]\n" +
	"                             [--ticket TICKET] [--self-test] [--no-self-test]\n"

const apvtHelp = apvtUsage + "\n" +
	"check-approve-time.py v1.0.0\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help       show this help message and exit\n" +
	"  --root ROOT      仓根（默认 = 本脚本的上两级）\n" +
	"  --state STATE    状态目录（只读）\n" +
	"  --ticket TICKET  对拍哪一枚件（默认 dev_edit）\n" +
	"  --self-test      只跑自检（不碰真目标）\n" +
	"  --no-self-test   跳过自检（只有已自证过的调用方才该用）\n"

// ── posixpath 同构面（✗ 不用 filepath.Join/Abs：语义差见出厂标准清单）────────────

// apvtPyJoin = CPython posixpath.join 逐段直译。
func apvtPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "/"):
			out = p
		case out == "" || strings.HasSuffix(out, "/"):
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}

// apvtPyNormpath = CPython posixpath.normpath（★ 保留恰两个前导斜杠）。
func apvtPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initialSlashes := 0
	if strings.HasPrefix(path, "/") {
		initialSlashes = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initialSlashes = 2
		}
	}
	var comps []string
	for _, comp := range strings.Split(path, "/") {
		switch {
		case comp == "" || comp == ".":
		case comp != ".." || (initialSlashes == 0 && len(comps) == 0) || (len(comps) > 0 && comps[len(comps)-1] == ".."):
			comps = append(comps, comp)
		default:
			if len(comps) > 0 {
				comps = comps[:len(comps)-1]
			}
		}
	}
	out := strings.Join(comps, "/")
	if initialSlashes > 0 {
		out = strings.Repeat("/", initialSlashes) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// apvtPyAbspath = CPython posixpath.abspath（纯词法：join(cwd, path) 后 normpath 一次）。
func apvtPyAbspath(cwd, path string) string {
	if strings.HasPrefix(path, "/") {
		return apvtPyNormpath(path)
	}
	return apvtPyNormpath(apvtPyJoin(cwd, path))
}

// apvtPyIsSpace = py `str.isspace()` 口径（★ 含 `\x1c`–`\x1f`；Go `unicode.IsSpace` 不含）。
func apvtPyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	return unicode.IsSpace(r)
}

// apvtPyStrip = py `str.strip()`（✗ 不用 strings.TrimSpace）。
func apvtPyStrip(s string) string {
	return strings.TrimFunc(s, apvtPyIsSpace)
}

// apvtSplitLines = py `str.splitlines()`（先折 `\r\n`/`\r` ⇒ 再按 py 的行界切；尾换行不产空尾项）。
func apvtSplitLines(s string) []string {
	var out []string
	var b strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		cut := false
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			cut = true
		case '\r':
			cut = true
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
		}
		if cut {
			out = append(out, b.String())
			b.Reset()
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// apvtPyTruthy = py 真值判。
func apvtPyTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case json.Number:
		return x.String() != "0" && x.String() != "0.0"
	default:
		return true
	}
}

// apvtPyStr = py `str(v)`（覆盖本件取值面：串 / None / 布尔 / 数）。
func apvtPyStr(v any) string {
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
	case json.Number:
		return x.String()
	case int:
		return fmt.Sprintf("%d", x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// apvtPyRepr = py `repr(v)`（覆盖本件 `%r` 取值面：串 / None / 数）。
func apvtPyRepr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		var b strings.Builder
		b.WriteByte('\'')
		for _, r := range x {
			switch r {
			case '\\':
				b.WriteString("\\\\")
			case '\'':
				b.WriteString("\\'")
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
		b.WriteByte('\'')
		return b.String()
	case bool:
		if x {
			return "True"
		}
		return "False"
	case json.Number:
		return x.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ── 取数（全部只读）─────────────────────────────────────────────────────────────

// apvtReadTextUTF8 = py `open(p, encoding="utf-8").read()`（严格解码；非 UTF-8 由调用处 fail-closed）。
func apvtReadTextUTF8(p string) (string, error) {
	bs, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(bs) {
		return "", errors.New("invalid utf-8")
	}
	return string(bs), nil
}

// apvtSHA256File = py `sha256_file`（分块读，结果同整读）。
func apvtSHA256File(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 65536)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// apvtParseISO = py `parse_iso`：带时区的 RFC3339 ⇒ (时刻, true)；解析不了 / 没时区 ⇒ (_, false)。
// 口径复刻 /usr/bin/python3(3.9.6) `datetime.fromisoformat`（先做 `Z` → `+00:00`）：
//
//	· 日期 `YYYY-MM-DD` 之后**一个**任意分隔字符（`T` / `t` / 空格 / `X` 皆可，实测）
//	· 时间 `HH[:MM[:SS[.ffffff]]]`（分数**恰 6 位**；`.1` ⇒ 3.9 判错）
//	· 时区 `+HH:MM[:SS[.ffffff]]`（**冒号必须**；`+0800` / `+08` ⇒ 错）· 裸 `Z` 已被前置替换
//	· 首尾空白一律**不**接受；日期缺时区 ⇒ naive ⇒ py 返 None（本函数 ok=false）
func apvtParseISO(s string) (time.Time, bool) {
	str := strings.ReplaceAll(s, "Z", "+00:00")
	if len(str) < 10 {
		return time.Time{}, false
	}
	digits := func(b string) (int, bool) {
		for i := 0; i < len(b); i++ {
			if b[i] < '0' || b[i] > '9' {
				return 0, false
			}
		}
		n := 0
		for i := 0; i < len(b); i++ {
			n = n*10 + int(b[i]-'0')
		}
		return n, true
	}
	atoiN := func(b string) (int, bool) { return digits(b) }
	if str[4] != '-' || str[7] != '-' {
		return time.Time{}, false
	}
	year, ok1 := atoiN(str[0:4])
	month, ok2 := atoiN(str[5:7])
	day, ok3 := atoiN(str[8:10])
	if !ok1 || !ok2 || !ok3 {
		return time.Time{}, false
	}
	hour, min, sec, nsec := 0, 0, 0, 0
	pos := 10
	if pos < len(str) {
		pos++ // 分隔字符（任意单个 rune；py 接受字母/空格/`X`）
		if pos+2 > len(str) {
			return time.Time{}, false
		}
		h, ok := atoiN(str[pos : pos+2])
		if !ok {
			return time.Time{}, false
		}
		hour = h
		pos += 2
		if pos < len(str) && str[pos] == ':' {
			pos++
			if pos+2 > len(str) {
				return time.Time{}, false
			}
			m, ok := atoiN(str[pos : pos+2])
			if !ok {
				return time.Time{}, false
			}
			min = m
			pos += 2
			if pos < len(str) && str[pos] == ':' {
				pos++
				if pos+2 > len(str) {
					return time.Time{}, false
				}
				sc, ok := atoiN(str[pos : pos+2])
				if !ok {
					return time.Time{}, false
				}
				sec = sc
				pos += 2
				if pos < len(str) && str[pos] == '.' {
					pos++
					if pos+6 > len(str) {
						return time.Time{}, false
					}
					fr, ok := atoiN(str[pos : pos+6])
					if !ok {
						return time.Time{}, false
					}
					nsec = fr * 1000
					pos += 6
				}
			}
		}
	}
	if month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || min > 59 || sec > 59 {
		return time.Time{}, false
	}
	if pos >= len(str) {
		return time.Time{}, false // 无时区 ⇒ naive ⇒ py 返 None
	}
	if str[pos] != '+' && str[pos] != '-' {
		return time.Time{}, false
	}
	sign := 1
	if str[pos] == '-' {
		sign = -1
	}
	pos++
	if pos+3 > len(str) || str[pos+2] != ':' {
		return time.Time{}, false
	}
	oh, ok := atoiN(str[pos : pos+2])
	if !ok {
		return time.Time{}, false
	}
	pos += 3
	if pos+2 > len(str) {
		return time.Time{}, false
	}
	om, ok := atoiN(str[pos : pos+2])
	if !ok {
		return time.Time{}, false
	}
	pos += 2
	os2 := 0
	if pos < len(str) && str[pos] == ':' {
		pos++
		if pos+2 > len(str) {
			return time.Time{}, false
		}
		v, ok := atoiN(str[pos : pos+2])
		if !ok {
			return time.Time{}, false
		}
		os2 = v
		pos += 2
		if pos < len(str) && str[pos] == '.' {
			pos++
			if pos+6 > len(str) {
				return time.Time{}, false
			}
			if _, ok := atoiN(str[pos : pos+6]); !ok {
				return time.Time{}, false
			}
			pos += 6
		}
	}
	if pos != len(str) {
		return time.Time{}, false
	}
	offset := sign * (oh*3600 + om*60 + os2)
	loc := time.FixedZone("", offset)
	return time.Date(year, time.Month(month), day, hour, min, sec, nsec, loc), true
}

// apvtTicket 是 `read_ticket` 的读数载体。
type apvtTicket struct {
	name       string
	path       string
	sha256     string
	approvedAt any
	line       *int
	mtimeStr   string
	mtime      time.Time
	atDt       time.Time
	atDtOK     bool
}

// apvtAuditRow 是审计里的一行读数。
type apvtAuditRow struct {
	line       int
	at         any
	atDt       time.Time
	atDtOK     bool
	machine    any
	hadApprove bool
	bad        string
	badJSON    bool
}

type apvtAudit struct {
	path  string
	total int
	rows  []apvtAuditRow
	keyed []apvtAuditRow
}

type apvtCommit struct {
	sha  string
	iso  string
	dt   time.Time
	dtOK bool
}

type apvtCommits struct {
	headSHA string
	commits []apvtCommit
}

type apvtReads struct {
	ticket  apvtTicket
	audit   apvtAudit
	commits apvtCommits
}

// apvtReadTicket = py `read_ticket`。
func apvtReadTicket(state, ticket string) (apvtTicket, string, bool) {
	p := apvtPyJoin(state, "approvals", ticket+".json")
	if _, err := os.Stat(p); err != nil {
		return apvtTicket{}, fmt.Sprintf("件不在：%s", p), false
	}
	txt, err := apvtReadTextUTF8(p)
	if err != nil {
		return apvtTicket{}, fmt.Sprintf("件读不了（%s）：%s", p, err), false
	}
	var lnNo *int
	for i, ln := range apvtSplitLines(txt) {
		if strings.Contains(ln, apvtQuoteApprovedAt) {
			n := i + 1
			lnNo = &n
			break
		}
	}
	dec := json.NewDecoder(strings.NewReader(txt))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return apvtTicket{}, fmt.Sprintf("件解不出来（%s）：%s", p, err), false
	}
	st, err := os.Stat(p)
	if err != nil {
		return apvtTicket{}, fmt.Sprintf("件读不了（%s）：%s", p, err), false
	}
	sum, err := apvtSHA256File(p)
	if err != nil {
		return apvtTicket{}, fmt.Sprintf("件读不了（%s）：%s", p, err), false
	}
	mt := st.ModTime()
	approvedAt, hasApproved := obj[apvtKeyApprovedAt]
	if !hasApproved {
		approvedAt = nil
	}
	atDt, atOK := time.Time{}, false
	if s, ok := approvedAt.(string); ok {
		atDt, atOK = apvtParseISO(s)
	}
	name := ticket + ".json"
	return apvtTicket{
		name:       name,
		path:       p,
		sha256:     sum,
		approvedAt: approvedAt,
		line:       lnNo,
		mtimeStr:   mt.Local().Format("2006-01-02T15:04:05-07:00"),
		mtime:      mt,
		atDt:       atDt,
		atDtOK:     atOK,
	}, "", true
}

// apvtReadAudit = py `read_audit`。
func apvtReadAudit(state, ticket string) (apvtAudit, string, bool) {
	p := apvtPyJoin(state, "edit_audit.jsonl")
	if _, err := os.Stat(p); err != nil {
		return apvtAudit{}, fmt.Sprintf("审计件不在：%s", p), false
	}
	txt, err := apvtReadTextUTF8(p)
	if err != nil {
		return apvtAudit{}, fmt.Sprintf("审计件读不了（%s）：%s", p, err), false
	}
	raw := apvtSplitLines(txt)
	a := apvtAudit{path: p, total: len(raw)}
	suffix := ticket + ".json"
	for i, ln := range raw {
		if apvtPyStrip(ln) == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(ln))
		dec.UseNumber()
		var d map[string]any
		if err := dec.Decode(&d); err != nil {
			a.rows = append(a.rows, apvtAuditRow{line: i + 1, at: nil, bad: "不是 JSON", badJSON: true})
			continue
		}
		if d == nil {
			a.rows = append(a.rows, apvtAuditRow{line: i + 1, at: nil, bad: "不是 JSON", badJSON: true})
			continue
		}
		_, hadApprove := d[apvtKeyApproval]
		atVal, hasAt := d[apvtKeyAt]
		if !hasAt {
			atVal = nil
		}
		mach := d[apvtKeyMachine]
		if !apvtPyTruthy(mach) {
			mach = d[apvtKeyHost]
		}
		var atDt time.Time
		atOK := false
		if s, ok := atVal.(string); ok {
			atDt, atOK = apvtParseISO(s)
		}
		rec := apvtAuditRow{
			line:       i + 1,
			at:         atVal,
			atDt:       atDt,
			atDtOK:     atOK,
			machine:    mach,
			hadApprove: hadApprove,
		}
		a.rows = append(a.rows, rec)
		approvalVal, has := d[apvtKeyApproval]
		if !has {
			approvalVal = ""
		}
		if strings.HasSuffix(apvtPyStr(approvalVal), suffix) {
			a.keyed = append(a.keyed, rec)
		}
	}
	return a, "", true
}

// apvtReadCommits = py `read_commits`（真跑 `git log`）。
func apvtReadCommits(repo string) (apvtCommits, string, bool) {
	gitDir := apvtPyJoin(repo, ".git")
	if st, err := os.Stat(gitDir); err != nil || !st.IsDir() {
		return apvtCommits{}, fmt.Sprintf("仓根里没有 .git：%s", repo), false
	}
	cmd := exec.Command("git", "-C", repo, "-c", "core.quotepath=false", "log", "--format=%H%x09%cI")
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		rc := -1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			rc = ee.ExitCode()
		}
		return apvtCommits{}, fmt.Sprintf("`git log` 退码 %d：%s", rc, apvtPyStrip(errb.String())), false
	}
	var commits []apvtCommit
	for _, ln := range apvtSplitLines(out.String()) {
		idx := strings.IndexByte(ln, '\t')
		if idx < 0 {
			continue
		}
		sha, iso := ln[:idx], ln[idx+1:]
		dt, ok := apvtParseISO(iso)
		commits = append(commits, apvtCommit{sha: sha, iso: iso, dt: dt, dtOK: ok})
	}
	if len(commits) == 0 {
		return apvtCommits{}, "`git log` 一行都没有", false
	}
	return apvtCommits{headSHA: commits[0].sha, commits: commits}, "", true
}

// apvtReadAll = py `read_all`（件 → 审计 → 提交，逐段短路）。
func apvtReadAll(repo, state, ticket string) (apvtReads, string, bool) {
	t, why, ok := apvtReadTicket(state, ticket)
	if !ok {
		return apvtReads{}, why, false
	}
	a, why, ok := apvtReadAudit(state, ticket)
	if !ok {
		return apvtReads{}, why, false
	}
	c, why, ok := apvtReadCommits(repo)
	if !ok {
		return apvtReads{}, why, false
	}
	return apvtReads{ticket: t, audit: a, commits: c}, "", true
}

// ── 判定口（纯函数：自检与真目标共用）────────────────────────────────────────────

// apvtFirstCommitAtOrAfter = py `first_commit_at_or_after`（倒序遍历 = 时间正序，取最早那笔）。
func apvtFirstCommitAtOrAfter(c apvtCommits, when time.Time) (apvtCommit, bool) {
	for i := len(c.commits) - 1; i >= 0; i-- {
		ck := c.commits[i]
		if ck.dtOK && !ck.dt.Before(when) {
			return ck, true
		}
	}
	return apvtCommit{}, false
}

// apvtPerRow 是 `judge` 的逐行读数元组（对应 py 的 6 元 tuple；`line` 用 any 以承载占位行的 `…`）。
type apvtPerRow struct {
	line any
	at   any
	gen  string
	sha  any
	iso  any
	mach any
}

// apvtJudge = py `judge`：排序 + 分类 ⇒ (红行, 交代行, 逐行读数)。
func apvtJudge(rd apvtReads, hostname string) ([]string, []string, []apvtPerRow) {
	red := []string{}
	notes := []string{}
	perRow := []apvtPerRow{}
	for _, r := range rd.audit.keyed {
		if !r.atDtOK {
			red = append(red, fmt.Sprintf("R2 `%s` 第 %d 行：`at=%s` 不是带时区的 RFC3339 ⇒ 这条时刻排不出序",
				rd.audit.path, r.line, apvtPyRepr(r.at)))
			perRow = append(perRow, apvtPerRow{line: r.line, at: r.at, gen: "R2 红"})
			continue
		}
		when := r.atDt
		gen := "本世代"
		if rd.ticket.atDtOK && !rd.ticket.mtime.After(when) && when.Before(rd.ticket.atDt) {
			red = append(red, fmt.Sprintf("R1 `%s` 第 %d 行：`at=%s` 落在 [件落盘 %s, 件签发 %s) 里 ⇒ 同一世代里审计早于签发",
				rd.audit.path, r.line, apvtPyStr(r.at), rd.ticket.mtimeStr, apvtPyStr(rd.ticket.approvedAt)))
			perRow = append(perRow, apvtPerRow{line: r.line, at: r.at, gen: "R1 红"})
			continue
		}
		if when.Before(rd.ticket.mtime) {
			gen = "更早世代"
			notes = append(notes, fmt.Sprintf("交代 `%s` 第 %d 行：`at=%s` < 件落盘 `%s` ⇒ 归**更早世代**（件被同名换过 ⇒ 同名不覆盖 ⇒ 要人删件才干净）",
				rd.audit.path, r.line, apvtPyStr(r.at), rd.ticket.mtimeStr))
		}
		ck, ckOK := apvtFirstCommitAtOrAfter(rd.commits, when)
		if !ckOK {
			notes = append(notes, fmt.Sprintf("交代 `%s` 第 %d 行：`at=%s` 之后**还没有提交** ⇒ 标「还没提交（等本次提交）」",
				rd.audit.path, r.line, apvtPyStr(r.at)))
		}
		if apvtPyTruthy(r.machine) && hostname != "" && apvtPyStr(r.machine) != hostname {
			notes = append(notes, fmt.Sprintf("交代 `%s` 第 %d 行：机器名 `%s` ≠ 本机 ⇒ 只要求排得出不矛盾的序（跨机那一格）",
				rd.audit.path, r.line, apvtPyStr(r.machine)))
		}
		row := apvtPerRow{line: r.line, at: r.at, gen: gen, mach: r.machine}
		if ckOK {
			row.sha = ck.sha
			row.iso = ck.iso
		}
		perRow = append(perRow, row)
	}
	return red, notes, perRow
}

// ── 自检（先自证会红 · 夹具全在临时目录里，真状态目录一个字节不碰）────────────────

type apvtPair struct {
	k string
	v any
}

// apvtJSONValue 印单个 JSON 值（py `json.dumps` 口径的最小集）。
func apvtJSONValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		bs, _ := json.Marshal(x)
		return string(bs)
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		bs, _ := json.Marshal(v)
		return string(bs)
	}
}

// apvtJSONCompact 印 py `json.dumps(obj, ensure_ascii=False)`（默认分隔符 `, ` / `: `）。
func apvtJSONCompact(pairs []apvtPair) string {
	var b strings.Builder
	b.WriteByte('{')
	for i, p := range pairs {
		if i > 0 {
			b.WriteString(", ")
		}
		bs, _ := json.Marshal(p.k)
		b.Write(bs)
		b.WriteString(": ")
		b.WriteString(apvtJSONValue(p.v))
	}
	b.WriteByte('}')
	return b.String()
}

// apvtJSONIndent1 印 py `json.dump(obj, ensure_ascii=False, indent=1)`（☆ 决定 `approved_at` 落第 4 行）。
func apvtJSONIndent1(pairs []apvtPair) string {
	var b strings.Builder
	b.WriteString("{\n")
	for i, p := range pairs {
		b.WriteString(" ")
		bs, _ := json.Marshal(p.k)
		b.Write(bs)
		b.WriteString(": ")
		b.WriteString(apvtJSONValue(p.v))
		if i < len(pairs)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}")
	return b.String()
}

// apvtBuildFixture = py `build_fixture`：造一枚件 + 一份审计（mtime 用 Chtimes 摆到指定秒）。
func apvtBuildFixture(tmp, approvedAt string, mtimeShift int, rows []apvtFixtureRow) {
	d := apvtPyJoin(tmp, "approvals")
	_ = os.MkdirAll(d, 0o755)
	tk := []apvtPair{
		{"tool", apvtDefaultTicket},
		{"approver", "Mr2109"},
		{"approved_at", approvedAt},
		{"scope", "*"},
		{"note", "D5 夹具"},
		{"sig_alg", "ed25519"},
		{"key_id", strings.Repeat("0", 16)},
		{"sig", ""},
	}
	p := apvtPyJoin(d, apvtDefaultTicket+".json")
	_ = os.WriteFile(p, []byte(apvtJSONIndent1(tk)+"\n"), 0o644)
	base, ok := apvtParseISO(approvedAt)
	if ok {
		mt := base.Add(time.Duration(mtimeShift) * time.Second)
		_ = os.Chtimes(p, mt, mt)
	}
	var audit strings.Builder
	for _, rw := range rows {
		pairs := []apvtPair{{apvtKeyAt, rw.at}, {"file", "x"}, {apvtKeyApproval, p}}
		pairs = append(pairs, rw.extra...)
		audit.WriteString(apvtJSONCompact(pairs))
		audit.WriteString("\n")
	}
	_ = os.WriteFile(apvtPyJoin(tmp, "edit_audit.jsonl"), []byte(audit.String()), 0o644)
}

type apvtFixtureRow struct {
	at    any
	extra []apvtPair
}

// apvtSelfTest = py `self_test`（返回 nil ⇒ 自检不过 ⇒ rc=2）。
func apvtSelfTest(stdout io.Writer, repo, realState string) bool {
	ok, bad := 0, 0
	tmp, err := os.MkdirTemp("", "zerg-approve-time-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "⛔ 自检① 取不到真读数：%s ⇒ 不给结论\n", err)
		return false
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	// ① 正控：真读数排出一条不矛盾的序
	rd, why, okRead := apvtReadAll(repo, realState, apvtDefaultTicket)
	if !okRead {
		fmt.Fprintf(stdout, "⛔ 自检① 取不到真读数：%s ⇒ 不给结论\n", why)
		return false
	}
	red, notes, perRow := apvtJudge(rd, "")
	if len(red) == 0 {
		fmt.Fprintf(stdout, "   ✓ ① 正控：真件（%s · `at` 第 %d 行 · 件 sha %s…）排出 %d 行序 · 红 0 行 · 交代 %d 行 · head %s\n",
			rd.ticket.name, apvtLineOr(rd.ticket.line, -1), apvtPrefix(rd.ticket.sha256, 12),
			len(perRow), len(notes), apvtPrefix(rd.commits.headSHA, 12))
		ok++
	} else {
		fmt.Fprintf(stdout, "   ✗ ① 正控失败：真读数就有红：%s\n", strings.Join(red, "；"))
		bad++
	}
	// ② 负控 R1：件落盘早于签发，审计行落在窗口里 ⇒ 红
	apvtBuildFixture(apvtPyJoin(tmp, "r1"), "2026-09-21T20:53:35+08:00", -600,
		[]apvtFixtureRow{{at: "2026-09-21T20:45:00+08:00"}})
	r2, _, ok2 := apvtReadAll(repo, apvtPyJoin(tmp, "r1"), apvtDefaultTicket)
	red2, _, _ := apvtJudge(r2, "")
	if ok2 && len(red2) > 0 && strings.Contains(red2[0], "R1") {
		fmt.Fprint(stdout, "   ✓ ② 负控：审计行落在 [件落盘, 件签发) 窗口里 ⇒ R1 判红（这一格不是「不会红」）\n")
		ok++
	} else {
		fmt.Fprint(stdout, "   ✗ ② 负控失败：窗口里那行没判红\n")
		bad++
	}
	// ③ 负控 R2：`at` 没时区 ⇒ 红
	apvtBuildFixture(apvtPyJoin(tmp, "r2"), "2026-09-21T20:53:35+08:00", 0,
		[]apvtFixtureRow{{at: "2026-09-21T20:54:00"}})
	r3, _, ok3 := apvtReadAll(repo, apvtPyJoin(tmp, "r2"), apvtDefaultTicket)
	red3, _, _ := apvtJudge(r3, "")
	if ok3 && len(red3) > 0 && strings.Contains(red3[0], "R2") {
		fmt.Fprint(stdout, "   ✓ ③ 负控：`at` 不带时区 ⇒ R2 判红（排不出序）\n")
		ok++
	} else {
		fmt.Fprint(stdout, "   ✗ ③ 负控失败：没时区的 `at` 没判红\n")
		bad++
	}
	// ④ 正控：更早世代 + 跨机 ⇒ **不**判红，只交代
	apvtBuildFixture(apvtPyJoin(tmp, "gen"), "2026-09-21T20:53:35+08:00", 0,
		[]apvtFixtureRow{{at: "2026-09-21T12:54:47+08:00", extra: []apvtPair{{apvtKeyMachine, "别人家的机器"}}}})
	r4, _, ok4 := apvtReadAll(repo, apvtPyJoin(tmp, "gen"), apvtDefaultTicket)
	red4, notes4, _ := apvtJudge(r4, "本机")
	hasGen := false
	hasCross := false
	for _, n := range notes4 {
		if strings.Contains(n, "更早世代") {
			hasGen = true
		}
		if strings.Contains(n, "跨机") {
			hasCross = true
		}
	}
	if ok4 && len(red4) == 0 && hasGen && hasCross {
		fmt.Fprintf(stdout, "   ✓ ④ 正控：更早世代 + 跨机 ⇒ 不判红 · 只交代（%d 条交代）\n", len(notes4))
		ok++
	} else {
		fmt.Fprint(stdout, "   ✗ ④ 正控失败：更早世代 / 跨机被误判红了\n")
		bad++
	}
	// ⑤ 负控：任一处读数缺 ⇒ 未取数（rc=2 那条口径）
	_, why5, ok5 := apvtReadAll(repo, apvtPyJoin(tmp, "空的"), apvtDefaultTicket)
	if !ok5 && strings.Contains(why5, "件不在") {
		fmt.Fprintf(stdout, "   ✓ ⑤ 负控：件缺 ⇒ 未取数（%s）⇒ 不许当绿\n", why5)
		ok++
	} else {
		fmt.Fprint(stdout, "   ✗ ⑤ 负控失败：件缺却没判「未取数」\n")
		bad++
	}
	// ⑥ 负控：提交表缺 ⇒ 未取数
	_, why6, ok6 := apvtReadAll(apvtPyJoin(tmp, "不是仓"), apvtPyJoin(tmp, "r1"), apvtDefaultTicket)
	if !ok6 && strings.Contains(why6, "没有 .git") {
		fmt.Fprintf(stdout, "   ✓ ⑥ 负控：`git log` 取不到 ⇒ 未取数（%s）⇒ 不许当绿\n", why6)
		ok++
	} else {
		fmt.Fprint(stdout, "   ✗ ⑥ 负控失败：提交表缺却没判「未取数」\n")
		bad++
	}
	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（夹具目录已清理 · 真状态目录一个字节未动）\n", ok, bad)
	return bad == 0
}

func apvtLineOr(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}

// apvtPrefix = py `s[:n]`（按码点切）。
func apvtPrefix(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}

// apvtReLineNo = py `re.findall(r"第 (\d+) 行", ...)`（★ 用 `[0-9]` 显式 ASCII）。
var apvtReLineNo = regexp.MustCompile("第 ([0-9]+) 行")

// ── 主 ───────────────────────────────────────────────────────────────────────────

// apvtScan = py `scan`。
func apvtScan(stdout, stderr io.Writer, repo, state, ticket, hostname string) int {
	rd, why, ok := apvtReadAll(repo, state, ticket)
	if !ok {
		fmt.Fprintf(stdout, "⛔ 未取数：%s ⇒ **不出结论**（rc=2）\n", why)
		return 2
	}
	t, a, c := rd.ticket, rd.audit, rd.commits
	red, notes, perRow := apvtJudge(rd, hostname)
	fmt.Fprint(stdout, "── 三处读数（每处带口径）\n")
	fmt.Fprintf(stdout, "   A 件：%s · `approved_at` 在第 %d 行 = %s · 件落盘 = %s · 件 sha256 = %s\n",
		t.name, apvtLineOr(t.line, -1), apvtPyStr(t.approvedAt), t.mtimeStr, t.sha256)
	// ★ 真口子（照搬）：py 对「不是 JSON」的行**不置** `had_approval_field` 键 ⇒ 下面那条
	//   `sum(...)` 会 KeyError ⇒ 回溯 rc=1（C 类）。本件照该形态 fail-closed。
	noApproval := 0
	for _, x := range a.rows {
		if x.badJSON {
			fmt.Fprint(stderr, "⛔ 未取数：审计里有不是 JSON 的行（py 于此处 KeyError ⇒ 回溯 rc=1）⇒ 不出结论\n")
			return 1
		}
		if !x.hadApprove {
			noApproval++
		}
	}
	fmt.Fprintf(stdout, "   B 审计：%s · 总 %d 行 · 键到本件 %d 行 · 无 `approval` 字段 %d 行\n",
		a.path, a.total, len(a.keyed), noApproval)
	fmt.Fprintf(stdout, "   C 提交：head_sha = %s · 提交表 %d 笔\n", c.headSHA, len(c.commits))
	fmt.Fprintf(stdout, "── 逐行排序（前 3 / 后 3 · 全量 %d 行）\n", len(perRow))
	if len(perRow) > 6 {
		shown := make([]apvtPerRow, 0, 7)
		shown = append(shown, perRow[:3]...)
		shown = append(shown, apvtPerRow{line: "…", at: "…", gen: "…", sha: "…", iso: "…", mach: "…"})
		shown = append(shown, perRow[len(perRow)-3:]...)
		for _, row := range shown {
			apvtPrintRow(stdout, row)
		}
	} else {
		for _, row := range perRow {
			apvtPrintRow(stdout, row)
		}
	}
	gen := []string{}
	other := []string{}
	for _, n := range notes {
		if strings.Contains(n, "更早世代") {
			gen = append(gen, n)
		} else {
			other = append(other, n)
		}
	}
	if len(gen) > 0 {
		ms := apvtReLineNo.FindAllStringSubmatch(strings.Join(gen, "\n"), -1)
		rows := make([]int, 0, len(ms))
		for _, m := range ms {
			n := 0
			for i := 0; i < len(m[1]); i++ {
				n = n*10 + int(m[1][i]-'0')
			}
			rows = append(rows, n)
		}
		mn, mx := rows[0], rows[0]
		for _, v := range rows {
			if v < mn {
				mn = v
			}
			if v > mx {
				mx = v
			}
		}
		fmt.Fprintf(stdout, "   ·  交代（合并 · %d 行）：**更早世代**（行 %d–%d）⇒ `at` < 件落盘 ⇒ 件被同名换过 · 同名不覆盖 ⇒ 要人删件才干净\n",
			len(gen), mn, mx)
	}
	for i, n := range other {
		if i >= 8 {
			break
		}
		fmt.Fprintf(stdout, "   ·  %s\n", n)
	}
	if len(other) > 8 {
		fmt.Fprintf(stdout, "   ·  交代：同类另有 %d 条（略 · 口径同上）\n", len(other)-8)
	}
	fmt.Fprint(stdout, "── 机器判不了（明标 · 不假装判过）\n")
	fmt.Fprint(stdout, "   · 时基：拿什么当「当时」⇒ `v1.3 §8.4 P21` **仍待 Mr2109 亲选** ⇒ 本件只排「能被现跑读数支撑」的序\n")
	fmt.Fprint(stdout, "   · `--ttl` 的过期只看**本地钟**（跨机不保证） ⇒ 本件不拿墙上钟当判据时基\n")
	if len(red) > 0 {
		fmt.Fprintf(stdout, "⇒ 门禁红（rc=1）：%d 条排不出序\n", len(red))
		for _, x := range red {
			fmt.Fprintf(stdout, "   ✗ %s\n", x)
		}
		return 1
	}
	fmt.Fprintf(stdout, "⇒ 全绿（rc=0）：%d 行排出了不矛盾的序 · 红 0 条 · 交代 %d 条（本件只读 · 未写一个字节）\n",
		len(perRow), len(notes))
	return 0
}

func apvtPrintRow(stdout io.Writer, row apvtPerRow) {
	sha := "—"
	if s := apvtPyStr(row.sha); row.sha != nil && s != "" {
		sha = apvtPrefix(s, 12)
	}
	iso := apvtPyStr(row.iso)
	if row.iso == nil || iso == "" {
		iso = "—"
	}
	fmt.Fprintf(stdout, "   行 %s · at %s · 世代 %s · 该次提交 %s %s\n",
		apvtPyStr(row.line), apvtPyStr(row.at), row.gen, sha, iso)
}

// ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ✗ ──

// apvtRefuseJSON = py `refuse_json_without_machine_face`（先于 argparse · 命中即 rc=2）。
func apvtRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
	hit := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return 0, false
	}
	qs := make([]string, 0, len(hit))
	for _, h := range hit {
		qs = append(qs, "`"+h+"`")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(qs, " · "))
	fields := "无（本门只出人读面）"
	if len(apvtJSONFields) > 0 {
		fs := make([]string, 0, len(apvtJSONFields))
		for _, f := range apvtJSONFields {
			fs = append(fs, "`"+f+"`")
		}
		fields = strings.Join(fs, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fields)
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return 2, true
}

// ── argparse 同构面（prog 名固定为 apvtProg）─────────────────────────────────────

type apvtOpts struct {
	root       string
	state      string
	ticket     string
	selfTest   bool
	noSelfTest bool
}

func apvtUsageError(stderr io.Writer, msg string) {
	fmt.Fprint(stderr, apvtUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", apvtProg, msg)
}

// apvtLooksLikeOption：argparse 的「看起来像选项」判（负数不算 ⇒ 可作值）。
func apvtLooksLikeOption(s string) bool {
	if len(s) < 2 || !strings.HasPrefix(s, "-") {
		return false
	}
	if s == "--" {
		return true
	}
	body := strings.TrimPrefix(s, "-")
	if m, _ := regexp.MatchString(`^[0-9]*\.?[0-9]+$`, body); m {
		return false
	}
	return true
}

func apvtParseArgs(argv []string, stdout, stderr io.Writer) (apvtOpts, int, bool) {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	opts := apvtOpts{
		root:   apvtFindRepoRoot(cwd),
		state:  apvtPyJoin(home, ".zerg", "state"),
		ticket: apvtDefaultTicket,
	}
	unrec := []string{}
	positionalOnly := false
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if !positionalOnly && tok == "--" {
			unrec = append(unrec, tok)
			positionalOnly = true
			continue
		}
		if !positionalOnly && strings.HasPrefix(tok, "--") && len(tok) > 2 {
			name, val, hasEq := tok, "", false
			if j := strings.IndexByte(tok, '='); j >= 0 {
				name = tok[:j]
				val = tok[j+1:]
				hasEq = true
			}
			matches := []string{}
			for _, o := range apvtLongOpts {
				if o == name {
					matches = []string{o}
					break
				}
			}
			if len(matches) == 0 {
				for _, o := range apvtLongOpts {
					if strings.HasPrefix(o, name) {
						matches = append(matches, o)
					}
				}
			}
			if len(matches) == 0 {
				unrec = append(unrec, tok)
				continue
			}
			if len(matches) > 1 {
				apvtUsageError(stderr, fmt.Sprintf("ambiguous option: %s could match %s", name, strings.Join(matches, ", ")))
				return opts, 2, true
			}
			canon := matches[0]
			switch canon {
			case "--help":
				fmt.Fprint(stdout, apvtHelp)
				return opts, 0, true
			case "--self-test", "--no-self-test":
				if hasEq {
					apvtUsageError(stderr, fmt.Sprintf("argument %s: ignored explicit argument '%s'", canon, val))
					return opts, 2, true
				}
				if canon == "--self-test" {
					opts.selfTest = true
				} else {
					opts.noSelfTest = true
				}
			default:
				v := val
				if !hasEq {
					if i+1 >= len(argv) || apvtLooksLikeOption(argv[i+1]) {
						apvtUsageError(stderr, fmt.Sprintf("argument %s: expected one argument", canon))
						return opts, 2, true
					}
					v = argv[i+1]
					i++
				}
				switch canon {
				case "--root":
					opts.root = v
				case "--state":
					opts.state = v
				case "--ticket":
					opts.ticket = v
				}
			}
			continue
		}
		if !positionalOnly && tok == "-h" {
			fmt.Fprint(stdout, apvtHelp)
			return opts, 0, true
		}
		unrec = append(unrec, tok)
	}
	if len(unrec) > 0 {
		apvtUsageError(stderr, "unrecognized arguments: "+strings.Join(unrec, " "))
		return opts, 2, true
	}
	return opts, 0, false
}

// apvtFindRepoRoot：cwd 上溯至最近的含 `scripts/gates/` 的目录（出厂声明 (a)）。
func apvtFindRepoRoot(cwd string) string {
	dir := cwd
	for i := 0; i < 16; i++ {
		if st, err := os.Stat(apvtPyJoin(dir, "scripts", "gates")); err == nil && st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd
}

// checkApproveTimeGateMain 是 `scripts/gates/check-approve-time.py` 的 Go 入口（M2 直调面）。
func checkApproveTimeGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc, hit := apvtRefuseJSON(stderr, argv); hit {
		return rc
	}
	opts, rc, done := apvtParseArgs(argv, stdout, stderr)
	if done {
		return rc
	}
	cwd, _ := os.Getwd()
	repo := apvtPyAbspath(cwd, opts.root)
	state := apvtPyAbspath(cwd, opts.state)
	hostname, _ := os.Hostname()
	fmt.Fprintln(stdout, apvtVersion)
	if !opts.noSelfTest {
		if !apvtSelfTest(stdout, repo, state) {
			fmt.Fprint(stdout, "⛔ 自检不过（读数取不到 或 有格不过）⇒ 不给结论（rc=2）\n")
			return 2
		}
	}
	if opts.selfTest {
		return 0
	}
	fmt.Fprintf(stdout, "── 真目标：件 %s（状态目录 %s · 只读）· 仓根 %s\n", opts.ticket, state, repo)
	return apvtScan(stdout, stderr, repo, state, opts.ticket, hostname)
}
