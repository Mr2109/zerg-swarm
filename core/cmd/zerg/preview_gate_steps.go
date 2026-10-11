// preview_gate_steps.go —— 判据件 `scripts/gates/preview-gate-steps.py`（526 行 · sha16 4e26cb04907c93d2）的
// Go 真搬（M2 一枚 · 门禁步骤表只读预览族 · 入口 `pgsGateMain` · 承接 GAP-20261001-13）。
//
// 验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 声明 (a) 默认根定位差异：py 的根来自 `--root`（缺省 `.`）经 `os.path.abspath` 归一；本件无 `__file__` 语境，
// 同样只按 `--root` + cwd 推导 ⇒ 两态在「cwd 相同」时同构。py 的 `--list-cmd` 缺省会起真子进程
// （`bash scripts/gates/precommit-gates.sh --list`）—— 本件如实复刻；对拍一律走显式 `--list-cache` 夹具，
// ✗ 不指真仓（本波只读、不写）。
//
// 声明 (b) 自指：本件落 `core/cmd/zerg/`。正文只出现「仓相对路径字面量」（`scripts/gates/precommit-gates.sh`）
// 与 argparse 帮助正文，**零**私有卷/家目录绝对路径字面量 ⇒ 不给 `check-hardcoded-private-paths.py`
// （SCAN_DIRS 含 `core/`）一类他门造新命中（影子仓 sha16 对照见交付回执）。
//
// 覆盖口径（逐条对到 py 行号）：
//
//	· `run_list`(L67) 现跑 `--list`（真起子进程 · 快照旋钮 `--list-cache` 可完全旁路）
//	· `parse_list`(L79) 现跑输出解析 + 计数自证
//	· `parse_proposal`(L127) 声明式步骤表（TSV/JSON 两形 · 模式闭集）
//	· `diff_steps`(L180) / `build_diff`(L204) 逐条 diff（增删 + 模式/scope 变化 + scope 注记）
//	· `do_template`(L234) 提案骨架（只印不写）
//	· `write_intent_refusal`(L256) / main 写意图三档（一律 rc=2 · 一字不写）
//	· `--self-test`(L362) 合成件成对负控（自回收 tempdir）
//
// 未覆盖面（✗ 不声称逐字对齐 · 逐条）：
//
//		· ① argparse 面：py 支持**长选项前缀缩写**（`--prop` ⇒ `--proposal`）与 `--opt=值`；本件实现**精确长旗标
//		  + `--opt=值` 等号形 + 缺值报话 + 未知旗标报话**，**不实现前缀缩写**（✗ 不假装）。`-h/--help` 正文按
//		  80 列**逐字固化**（`/usr/bin/python3` 3.9.6 的 `optional arguments:` 文案，已对拍）。
//		· ② `--list-cmd` 的 py 口子：py `list(cmd or DEFAULT)` 对**字符串**会拆成**单字符列表** ⇒ 传 `--list-cmd X`
//		  时 py 会拿每个字符当可执行名（`FileNotFoundError`）。本件**逐字复刻该口子**（按码点拆串），并复刻
//		  `[Errno 2] No such file or directory: '<c0>'` 文案；其余 errno 文案按 strerror 同构。
//	  · ③ JSON 解码失败正文：复刻 py `json.JSONDecodeError` 的常见类（`Expecting value` / `Expecting
//	    property name enclosed in double quotes` / `Expecting ':' delimiter` / `Expecting ',' delimiter` /
//	    `Extra data` + `line L column C (char N)`，位置按 py 码点口径换算）；**未覆盖**：截断输入
//	    （Go `unexpected end of JSON input`）等非常见类 ⇒ 该档回落 Go 原文（只保 rc=2 与「⛔ 提案解不动：」前缀）。
//		· ④ 非 UTF-8 输入件：py `open(encoding='utf-8')` 抛 `UnicodeDecodeError`（非 OSError）⇒ 回溯 + rc=1；
//		  本件按「读不了 ⇒ rc=2 fail-closed ＋ 具名文案」处置（该格只保 rc 之外不同，已声明）。
//		· ⑤ `subprocess.TimeoutExpired`（1800s 超时）档不可复现 ⇒ 本件保留该分支与文案，**未实测**。
//		· ⑥ C 类最外层无兜底：py 是裸 `sys.exit(main())` ⇒ main 内任何未捕异常 = 回溯 rc=1；本件 fail-closed rc=2。
//		· ⑦ `--self-test` 档含随机 tempdir 名（`mkdtemp`）⇒ 逐字节比对需**掩码该段**（掩码规则见回执）。
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
)

const pgsProg = "preview-gate-steps.py"

// pgsDefaultTarget —— py 第 58 行 `DEFAULT_TARGET`。
const pgsDefaultTarget = "scripts/gates/precommit-gates.sh"

// pgsDefaultListCmd —— py 第 59 行 `DEFAULT_LIST_CMD`。
var pgsDefaultListCmd = []string{"bash", "scripts/gates/precommit-gates.sh", "--list"}

// pgsModeClosed —— py 第 60 行 `MODE_CLOSED`（闸的档位闭集）。
var pgsModeClosed = []string{"rc", "empty", "tri", "tri-report"}

// pgsUsage —— argparse 80 列下的 usage 块（错误档与 `-h` 共用 · 逐字照 py 现读）。
const pgsUsage = "usage: preview-gate-steps.py [-h] [--proposal 件] [--template] [--list-cache 件]\n" +
	"                             [--list-cmd 串] [--target TARGET] [--apply]\n" +
	"                             [--write] [--commit] [--yes] [--approval 件]\n" +
	"                             [--root ROOT] [--json] [--self-test]\n"

// pgsHelp —— `-h`/`--help` 全文（Python 3.9 argparse · 80 列 · 逐字固化）。
const pgsHelp = pgsUsage + "\n" +
	"optional arguments:\n" +
	"  -h, --help       show this help message and exit\n" +
	"  --proposal 件     提案（声明式步骤表：`.tsv` / `.json`）\n" +
	"  --template       把现状打成提案骨架（TSV · **只印不写**）\n" +
	"  --list-cache 件   复用一次现跑 `--list` 的快照（避免重跑自检）\n" +
	"  --list-cmd 串     覆盖现跑命令（默认 `bash scripts/gates/precommit-gates.sh --list`）\n" +
	"  --target TARGET  写面目标（默认提交闸脚本 · 本波**只读**）\n" +
	"  --apply          写意图（本波一律 rc=2：写面要人签）\n" +
	"  --write          同 `--apply`\n" +
	"  --commit         同 `--apply`\n" +
	"  --yes            写意图的前置（缺 ⇒ rc=2）\n" +
	"  --approval 件     人签批准件（缺 ⇒ rc=2）\n" +
	"  --root ROOT      仓根（默认当前目录）\n" +
	"  --json           机器可读输出\n" +
	"  --self-test      合成件成对负控（不碰真目标）\n"

// ── py 路径语义家族（照 CPython posixpath 直译 · ✗ 不用 filepath.Join/Abs）─────────
func pgsPyJoin(parts ...string) string {
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

func pgsPyNormpath(path string) string {
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

// pgsPyAbspath —— posixpath.abspath = normpath(join(cwd, path))（纯词法 · 不碰盘）。
func pgsPyAbspath(cwd, path string) string {
	return pgsPyNormpath(pgsPyJoin(cwd, path))
}

// ── py 字符串原语 ───────────────────────────────────────────────────────────
// pgsPyIsSpace —— py `str.isspace()`：含 `\x1c-\x1f`（Go `unicode.IsSpace` 不含）。
func pgsPyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

func pgsPyStrip(s string) string {
	return strings.TrimRightFunc(strings.TrimLeftFunc(s, pgsPyIsSpace), pgsPyIsSpace)
}

func pgsPyLStrip(s string) string {
	return strings.TrimLeftFunc(s, pgsPyIsSpace)
}

// pgsPySplitLines —— py `str.splitlines()`（依 CPython 的行界集；尾行界不产空串）。
func pgsPySplitLines(s string) []string {
	out := []string{}
	rs := []rune(s)
	start := 0
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		isBreak := false
		switch r {
		case '\n', '\v', '\f', '\r', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			isBreak = true
		}
		if !isBreak {
			continue
		}
		out = append(out, string(rs[start:i]))
		if r == '\r' && i+1 < len(rs) && rs[i+1] == '\n' {
			i++
		}
		start = i + 1
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}

// pgsPySplitNone —— py `str.split(None, maxsplit)`（空白连跑为界 · 掐头去尾 · 至多 maxsplit 次）。
func pgsPySplitNone(s string, maxsplit int) []string {
	rs := []rune(s)
	out := []string{}
	i, n := 0, len(rs)
	for i < n && pgsPyIsSpace(rs[i]) {
		i++
	}
	for i < n {
		start := i
		for i < n && !pgsPyIsSpace(rs[i]) {
			i++
		}
		out = append(out, string(rs[start:i]))
		if maxsplit > 0 && len(out) == maxsplit {
			for i < n && pgsPyIsSpace(rs[i]) {
				i++
			}
			if i < n {
				out = append(out, string(rs[i:]))
			}
			return out
		}
		for i < n && pgsPyIsSpace(rs[i]) {
			i++
		}
	}
	return out
}

// pgsPyRepr —— py `repr(str)`（单引号优先 · 含 `'` 无 `"` 时改双引号 · 控制符转义）。
func pgsPyRepr(s string) string {
	q := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
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
			if byte(r) == q && r < 0x80 {
				b.WriteByte('\\')
				b.WriteByte(q)
			} else if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, "\\x%02x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(q)
	return b.String()
}

// pgsPyStr —— py `str(v)`（JSON 取值面）。
func pgsPyStr(v any) string {
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
	case float64:
		return strconv.FormatFloat(x, 'r', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

func pgsPyTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case json.Number:
		f, err := x.Float64()
		return err == nil && f != 0
	case float64:
		return x != 0
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	default:
		return true
	}
}

// pgsPyReprAny —— `%r` 的取值面（够自检用）。
func pgsPyReprAny(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return pgsPyRepr(x)
	case bool:
		return pgsReprBool(x)
	case json.Number:
		return x.String()
	default:
		return pgsPyStr(v)
	}
}

func pgsReprBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// ── 数字面（py `int()` 认 Unicode Nd）─────────────────────────────────────────
func pgsIsDigit(r rune) bool { return unicode.IsDigit(r) }

func pgsDigitVal(r rune) (int, bool) {
	if !unicode.IsDigit(r) {
		return 0, false
	}
	start := r
	for unicode.IsDigit(start - 1) {
		start--
	}
	v := int(r - start)
	if v < 0 || v > 9 {
		return 0, false
	}
	return v, true
}

// ── sha256 / 文件面 ──────────────────────────────────────────────────────────
func pgsSHA256File(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
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
			return "", false
		}
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

func pgsIsFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

// pgsOsError —— 复刻 py `OSError` 的 `str()`（`[Errno N] <strerror>: '<path>'`）。
func pgsOsError(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		var eno syscall.Errno
		if errors.As(pe.Err, &eno) {
			return fmt.Sprintf("[Errno %d] %s: '%s'", int(eno), pgsUpperFirst(eno.Error()), pe.Path)
		}
	}
	return err.Error()
}

// pgsUpperFirst —— libc `strerror` 首字母大写（Go `Errno.Error()` 小写 ⇒ 逐字对齐用）。
func pgsUpperFirst(s string) string {
	for _, r := range s {
		return string(unicode.ToUpper(r)) + s[len(string(r)):]
	}
	return s
}

// ── `--list` 真源（py L67/L79/L109）──────────────────────────────────────────
func pgsListCmdResolved(listCmd string) []string {
	if listCmd == "" {
		return append([]string{}, pgsDefaultListCmd...)
	}
	out := []string{}
	for _, r := range listCmd { // py `list(<str>)`：按码点拆
		out = append(out, string(r))
	}
	return out
}

// pgsRunList —— py L67 `run_list`（真起子进程 · 只读）。
func pgsRunList(root, listCmd string) (string, string) {
	return pgsRunListArgv(root, pgsListCmdResolved(listCmd))
}

// pgsRunListArgv —— 同 `pgsRunList`，但收**已解析**的 argv（机读面现跑复用同一支 · 不另起第二套）。
func pgsRunListArgv(root string, cmd []string) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 1800*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	c.Dir = root
	var ob, eb strings.Builder
	c.Stdout = &ob
	c.Stderr = &eb
	err := c.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Sprintf("现跑命令超时：%s", strings.Join(cmd, " "))
		}
		if _, lerr := exec.LookPath(cmd[0]); lerr != nil {
			return "", fmt.Sprintf("现跑命令起不来（%s）：[Errno 2] No such file or directory: '%s'", strings.Join(cmd, " "), cmd[0])
		}
		return "", fmt.Sprintf("现跑命令起不来（%s）：%s", strings.Join(cmd, " "), err.Error())
	}
	return pgsUniversalNL(ob.String()) + "\n" + pgsUniversalNL(eb.String()), ""
}

// pgsUniversalNL —— py `text=True` 的 universal newlines（`\r\n`/`\r` ⇒ `\n`）。
func pgsUniversalNL(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// pgsListHeaderReMatch —— py L84 `LIST_HEADER_RE.search`（`──\s*步骤清单` · `\s` Unicode）。
func pgsListHeaderReMatch(ln string) bool {
	rs := []rune(ln)
	for i := 0; i+1 < len(rs); i++ {
		if rs[i] != '─' || rs[i+1] != '─' {
			continue
		}
		j := i + 2
		for j < len(rs) && pgsPyIsSpace(rs[j]) {
			j++
		}
		if strings.HasPrefix(string(rs[j:]), "步骤清单") {
			return true
		}
	}
	return false
}

// pgsListTotalReMatch —— py L62 `LIST_TOTAL_RE.match`（`^共\s*(\d+)\s*步\s*$`）。
func pgsListTotalReMatch(s string) (int, bool) {
	rs := []rune(s)
	if len(rs) == 0 || rs[0] != '共' {
		return 0, false
	}
	i := 1
	for i < len(rs) && pgsPyIsSpace(rs[i]) {
		i++
	}
	start := i
	val := 0
	for i < len(rs) && pgsIsDigit(rs[i]) {
		d, ok := pgsDigitVal(rs[i])
		if !ok {
			return 0, false
		}
		val = val*10 + d
		i++
	}
	if i == start {
		return 0, false
	}
	for i < len(rs) && pgsPyIsSpace(rs[i]) {
		i++
	}
	if i >= len(rs) || rs[i] != '步' {
		return 0, false
	}
	i++
	for i < len(rs) && pgsPyIsSpace(rs[i]) {
		i++
	}
	if i != len(rs) {
		return 0, false
	}
	return val, true
}

// pgsParseList —— py L79 `parse_list` ⇒ (steps, total, hasTotal, err)。★ steps 恒非 nil（空 ≠ 取不到）。
func pgsParseList(text string) ([]pgsStep, int, bool, string) {
	steps := []pgsStep{}
	lines := pgsPySplitLines(text)
	start := -1
	for i, ln := range lines {
		if pgsListHeaderReMatch(ln) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return steps, 0, false, "`--list` 输出里找不到「步骤清单」段（真源取不到）"
	}
	total, hasTotal := 0, false
	for _, ln := range lines[start:] {
		s := pgsPyStrip(ln)
		if s == "" {
			continue
		}
		if m, ok := pgsListTotalReMatch(s); ok {
			total, hasTotal = m, true
			break
		}
		parts := pgsPySplitNone(s, 2)
		if len(parts) != 3 {
			return steps, 0, false, fmt.Sprintf("步骤行解不动（要 `scope 模式 名称` 三格）：%s", pgsPyRepr(ln))
		}
		steps = append(steps, pgsStep{Scope: parts[0], Mode: parts[1], Name: parts[2]})
	}
	if !hasTotal {
		return steps, 0, false, "`--list` 输出里没有「共 N 步」那一行（计数自证做不了）"
	}
	if total != len(steps) {
		return steps, 0, false, fmt.Sprintf("计数自证不过：现印「共 %d 步」但逐行解出 %d 条 ⇒ 不许当绿", total, len(steps))
	}
	return steps, total, true, ""
}

// pgsRawJSONStr —— 从机读面一格（RawMessage）取字符串格；缺格/非串 ⇒ ("", false)。
func pgsRawJSONStr(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// pgsParseListJSON —— 读步骤表**机读面**（`precommit-gates.sh --list --json` 的单对象
// `{"count":N,"steps":[{"scope":..,"mode":..,"name":..},…]}` · 缺口 GAP-20260926-119 落的口子）。
//
// ★ GAP-20261005-30 残面（本片）：改前 `parse_list` 只认人面 —— 抠「步骤清单」表头 + 「共 N 步」
// 计数行；闸脚本**文案一字改**即不可机检（且人面是三个消费面的隐式契约）。此处与**同族正解**同形
// （`gate_report.go` 的 `grtParseDanglingJSON` 读 `check-dangling-refs --json` 机读面 · `--json` 成
// 单对象带 schema 标记）：**机读面优先，人面只作退化路**；机读面在场而形状坏/计数自证不过 ⇒ **响亮失败**
// （✗ 不许静默退回人面把读数当绿）。
//
// 返回 (steps, total, isMachine, err)：
//
//	· isMachine=false          ⇒ 本输出里**没有**机读面（交 `parse_list` 人面退化路）；
//	· isMachine=true, err==""  ⇒ 机读面读成（`total` = `count`，已自证 == len(steps)）；
//	· isMachine=true, err!=""  ⇒ 机读面在场但自证不过 ⇒ 不许当绿（err 是判词）。
func pgsParseListJSON(text string) ([]pgsStep, int, bool, string) {
	for _, ln := range pgsPySplitLines(text) {
		s := pgsPyStrip(ln)
		if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
			continue
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal([]byte(s), &doc); err != nil {
			continue // 不是 JSON 对象行 ⇒ 不是机读面
		}
		rawCount, okC := doc["count"]
		rawSteps, okS := doc["steps"]
		if !okC || !okS {
			continue // 有 JSON 对象但不是步表机读面（缺 `count`/`steps` 鉴别键）
		}
		// 到此即认定「机读面在场」—— 后面任何读不动都判**响亮失败**，✗ 不落回人面。
		var count int
		if err := json.Unmarshal(rawCount, &count); err != nil {
			return []pgsStep{}, 0, true, "步表机读面（--list --json）的 `count` 读不出整数 ⇒ 不给结论"
		}
		var arr []map[string]json.RawMessage
		if err := json.Unmarshal(rawSteps, &arr); err != nil {
			return []pgsStep{}, 0, true, "步表机读面（--list --json）的 `steps` 读不出数组 ⇒ 不给结论"
		}
		steps := []pgsStep{}
		for i, it := range arr {
			nm, okN := pgsRawJSONStr(it["name"])
			if !okN {
				return []pgsStep{}, 0, true, fmt.Sprintf("步表机读面第 %d 条缺 `name` 格（或不是串）⇒ 不给结论", i+1)
			}
			sc, _ := pgsRawJSONStr(it["scope"])
			md, _ := pgsRawJSONStr(it["mode"])
			steps = append(steps, pgsStep{Scope: sc, Mode: md, Name: nm})
		}
		if count != len(steps) {
			return steps, count, true, fmt.Sprintf("步表机读面计数自证不过：`count`=%d 但 `steps` 解出 %d 条 ⇒ 不许当绿", count, len(steps))
		}
		// ★ 缺口 `GAP-20261010-13`（本片）：机读面新增顶格 `gates_total` 的**自证** —— 与 `count`
		//   同形（形状坏 ⇒ 响亮失败，✗ 不静默退回人面把读数当绿）。缺格 ⇒ 不判（向后兼容：旧机读面
		//   /合成夹具没有这一格，行为与改前逐字相同）。
		if rawGT, okGT := doc["gates_total"]; okGT {
			var gt int
			if err := json.Unmarshal(rawGT, &gt); err != nil {
				return []pgsStep{}, 0, true, "步表机读面（--list --json）的 `gates_total` 读不出整数 ⇒ 不给结论"
			}
			if gt != len(steps) {
				return steps, count, true, fmt.Sprintf("步表机读面自证不过：`gates_total`=%d 但 `steps` 解出 %d 条 ⇒ 不许当绿", gt, len(steps))
			}
		}
		return steps, count, true, ""
	}
	return []pgsStep{}, 0, false, ""
}

// pgsListCmdMachineFace —— 现跑真源的**机读面 argv**：只在**默认真源**（`--list-cmd` 未覆盖）时，
// 在末枚 `--list` 后接一枚 `--json` ⇒ `{"count":N,"steps":[…]}`（`precommit-gates.sh` 的 `--list` 机器面）。
// ★ `--list-cmd` 被用户覆盖那条口子按**码点拆串**（py `list(<str>)` 的复刻 · 见件头声明②）⇒ 那不保证认
// `--json`，故**不追加**（让现跑输出自然落回人面退化路 · 行为与改前逐字相同）。
func pgsListCmdMachineFace(listCmd string) ([]string, bool) {
	if listCmd != "" {
		return pgsListCmdResolved(listCmd), false
	}
	return append(append([]string{}, pgsDefaultListCmd...), "--json"), true
}

func pgsTotalRepr(total int, hasTotal bool) string {
	if !hasTotal {
		return "None"
	}
	return strconv.Itoa(total)
}

func pgsTotalOr0(total int, hasTotal bool) int {
	if !hasTotal {
		return 0
	}
	return total
}

// pgsLoadCurrent —— py L109 `load_current`。
//
// ★ GAP-20261005-30 残面（本片）：现状真源取数**先取机读面**（`--list --json` 的
// `{"count":N,"steps":[…]}`），取不到才退化到人面 `parse_list`（抠「步骤清单」表头 + 「共 N 步」
// 计数行）。机读面在场而形状坏/自证不过 ⇒ **响亮失败**（rc=2 文案），✗ 不许静默退回人面把读数当绿。
func pgsLoadCurrent(root string, a *pgsArgs) ([]pgsStep, string, string) {
	if a.listCache != "" {
		data, err := os.ReadFile(a.listCache)
		if err != nil {
			return []pgsStep{}, "", "--list-cache 读不到：" + pgsOsError(err)
		}
		if steps, total, isM, perr := pgsParseListJSON(string(data)); isM {
			if perr != "" {
				return []pgsStep{}, "", perr
			}
			return steps, fmt.Sprintf("快照 `%s`（机读面：共 %d 步）", a.listCache, total), ""
		}
		steps, total, hasTotal, perr := pgsParseList(string(data))
		src := fmt.Sprintf("快照 `%s`（共 %d 步）", a.listCache, pgsTotalOr0(total, hasTotal))
		return steps, src, perr
	}
	if mcmd, ok := pgsListCmdMachineFace(a.listCmd); ok {
		if mtext, mrerr := pgsRunListArgv(root, mcmd); mrerr == "" {
			if steps, total, isM, perr := pgsParseListJSON(mtext); isM {
				if perr != "" {
					return []pgsStep{}, "", perr
				}
				return steps, fmt.Sprintf("现跑 `%s`（机读面：共 %d 步）", strings.Join(mcmd, " "), total), ""
			}
		}
	}
	text, rerr := pgsRunList(root, a.listCmd)
	if rerr != "" {
		return []pgsStep{}, "", rerr
	}
	steps, total, hasTotal, perr := pgsParseList(text)
	src := fmt.Sprintf("现跑 `%s`（共 %s 步）", strings.Join(pgsListCmdResolved(a.listCmd), " "), pgsTotalRepr(total, hasTotal))
	return steps, src, perr
}

// ── 提案（py L127 `parse_proposal`）──────────────────────────────────────────
type pgsRawItem struct {
	scope, mode, name any
	i                 int
}

func pgsInClosed(mode string) bool {
	for _, m := range pgsModeClosed {
		if m == mode {
			return true
		}
	}
	return false
}

func pgsParseProposal(path string) ([]pgsStep, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "提案读不到：" + pgsOsError(err)
	}
	text := string(data)
	var raw []pgsRawItem
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		var doc any
		dec := json.NewDecoder(strings.NewReader(text))
		dec.UseNumber()
		derr := dec.Decode(&doc)
		if derr == nil {
			var trailing any
			if e2 := dec.Decode(&trailing); e2 != io.EOF {
				derr = pgsJSONExtraData{offset: int(dec.InputOffset())}
			}
		}
		if derr != nil {
			return nil, "提案 JSON 解不动：" + pgsPyJSONErr(text, derr)
		}
		var items any
		if m, ok := doc.(map[string]any); ok {
			items = m["steps"]
		} else {
			items = doc
		}
		arr, ok := items.([]any)
		if !ok || len(arr) == 0 {
			return nil, "提案 JSON 里没有 `steps` 列表（或它是空的）"
		}
		for idx, it := range arr {
			i := idx + 1
			switch v := it.(type) {
			case map[string]any:
				raw = append(raw, pgsRawItem{v["scope"], v["mode"], v["name"], i})
			case string:
				parts := pgsItemParts(v)
				raw = append(raw, pgsRawItem{pgsPartAny(parts, 0), pgsPartAny(parts, 1), pgsPartAny(parts, 2), i})
			default:
				return nil, fmt.Sprintf("提案 JSON 第 %d 项不是对象也不是串", i)
			}
		}
	} else {
		for idx, ln := range pgsPySplitLines(text) {
			i := idx + 1
			if strings.HasPrefix(pgsPyLStrip(ln), "#") {
				continue
			}
			s := strings.TrimRight(ln, "\n")
			if pgsPyStrip(s) == "" {
				continue
			}
			parts := pgsItemParts(s)
			if len(parts) != 3 {
				return nil, fmt.Sprintf("提案第 %d 行解不动（要 `scope ⇥ mode ⇥ 名称` 三格）：%s", i, pgsPyRepr(ln))
			}
			raw = append(raw, pgsRawItem{parts[0], parts[1], pgsPyStrip(parts[2]), i})
		}
	}
	steps := []pgsStep{}
	seen := map[string]int{}
	for _, it := range raw {
		if !pgsPyTruthy(it.scope) || !pgsPyTruthy(it.mode) || !pgsPyTruthy(it.name) {
			return nil, fmt.Sprintf("提案第 %d 项缺格（scope/mode/名称 三格都要有）", it.i)
		}
		ms := pgsPyStr(it.mode)
		if !pgsInClosed(ms) {
			return nil, fmt.Sprintf("提案第 %d 项的模式名在**闭集外**：%s（闭集 = %s —— 表外模式名在闸里会被硬红 ⇒ 本件也不放它过）",
				it.i, pgsPyReprAny(it.mode), strings.Join(pgsModeClosed, " / "))
		}
		ns := pgsPyStr(it.name)
		if prev, dup := seen[ns]; dup {
			return nil, fmt.Sprintf("提案里名称重了：%s（第 %d 项与第 %d 项 —— 名称是步骤的身份，重名判不了）",
				pgsPyReprAny(it.name), it.i, prev)
		}
		seen[ns] = it.i
		steps = append(steps, pgsStep{Scope: pgsPyStr(it.scope), Mode: ms, Name: ns})
	}
	if len(steps) == 0 {
		return nil, "提案里 0 步（空表 = 判不了，不许当「全删」）"
	}
	return steps, ""
}

func pgsItemParts(s string) []string {
	if strings.Contains(s, "\t") {
		return strings.Split(s, "\t")
	}
	return pgsPySplitNone(s, 2)
}

func pgsPartAny(parts []string, i int) any {
	if len(parts) > i {
		return parts[i]
	}
	return nil
}

// pgsJSONExtraData —— py `json.loads` 对「首值之后还有非空白」判 `Extra data`。
type pgsJSONExtraData struct{ offset int }

func (pgsJSONExtraData) Error() string { return "Extra data" }

// pgsPyJSONErr —— 复刻 py `json.JSONDecodeError` 正文（`<cat>: line L column C (char N)`）。
// 未覆盖：截断输入（Go `unexpected end of JSON input`）等非常见类 ⇒ 回落 Go 原文（见件头未覆盖面③）。
func pgsPyJSONErr(text string, err error) string {
	if strings.TrimSpace(text) == "" {
		return "Expecting value: line 1 column 1 (char 0)"
	}
	cat, ch := "", 0
	if ed, ok := err.(pgsJSONExtraData); ok {
		cat = "Extra data"
		ch = pgsSkipWS(text, ed.offset)
	} else {
		var serr *json.SyntaxError
		if errors.As(err, &serr) {
			msg := serr.Error()
			switch {
			case strings.Contains(msg, "looking for beginning of value"):
				cat = "Expecting value"
			case strings.Contains(msg, "looking for beginning of object key string"):
				cat = "Expecting property name enclosed in double quotes"
			case strings.Contains(msg, "after object key:value pair"):
				cat = "Expecting ',' delimiter"
			case strings.Contains(msg, "after object key"):
				cat = "Expecting ':' delimiter"
			case strings.Contains(msg, "after array element"):
				cat = "Expecting ',' delimiter"
			case strings.Contains(msg, "after top-level value"):
				cat = "Extra data"
			}
			if cat != "" {
				// Go 的 SyntaxError.Offset 指**越过错字之后**的位置 ⇒ 回退一个码点对齐 py `pos`。
				ch = pgsByteToCharOffset(text, int(serr.Offset)) - 1
				if ch < 0 {
					ch = 0
				}
			}
		}
	}
	if cat == "" {
		return err.Error()
	}
	line, col := pgsLineCol(text, ch)
	return fmt.Sprintf("%s: line %d column %d (char %d)", cat, line, col, ch)
}

// pgsByteToCharOffset —— 字节偏移 ⇒ 码点偏移（py 的 pos 是字符下标）。
func pgsByteToCharOffset(s string, byteOff int) int {
	if byteOff < 0 {
		byteOff = 0
	}
	if byteOff > len(s) {
		byteOff = len(s)
	}
	return utf8.RuneCountInString(s[:byteOff])
}

// pgsSkipWS —— 从字节偏移起跳过 py `WHITESPACE`（`[ 	\n\r]*`）后的**码点**下标。
func pgsSkipWS(s string, byteOff int) int {
	i := pgsByteToCharOffset(s, byteOff)
	rs := []rune(s)
	for i < len(rs) && (rs[i] == ' ' || rs[i] == '	' || rs[i] == '\n' || rs[i] == '\r') {
		i++
	}
	return i
}

// pgsLineCol —— py：`line = count('\n',0,pos)+1` · `col = pos - rfind('\n',0,pos)`。
func pgsLineCol(s string, ch int) (int, int) {
	rs := []rune(s)
	if ch > len(rs) {
		ch = len(rs)
	}
	line, last := 1, -1
	for i := 0; i < ch; i++ {
		if rs[i] == '\n' {
			line++
			last = i
		}
	}
	return line, ch - last
}

// ── 逐条 diff（py L180/L204）─────────────────────────────────────────────────
type pgsStep struct {
	Scope string `json:"scope"`
	Mode  string `json:"mode"`
	Name  string `json:"name"`
}

type pgsChange struct {
	A, B pgsStep
	Why  string
}

func pgsDiffSteps(cur, prop []pgsStep) (added, removed []pgsStep, changed []pgsChange, notes []string) {
	added = []pgsStep{}
	removed = []pgsStep{}
	changed = []pgsChange{}
	notes = []string{}
	cm := map[string]pgsStep{}
	var cmOrder []string
	for _, s := range cur {
		if _, ok := cm[s.Name]; !ok {
			cmOrder = append(cmOrder, s.Name)
		}
		cm[s.Name] = s
	}
	pm := map[string]pgsStep{}
	var pmOrder []string
	for _, s := range prop {
		if _, ok := pm[s.Name]; !ok {
			pmOrder = append(pmOrder, s.Name)
		}
		pm[s.Name] = s
	}
	for _, n := range pmOrder {
		if _, ok := cm[n]; !ok {
			added = append(added, pm[n])
		}
	}
	for _, n := range cmOrder {
		if _, ok := pm[n]; !ok {
			removed = append(removed, cm[n])
		}
	}
	for _, n := range pmOrder {
		a, ok := cm[n]
		if !ok {
			continue
		}
		b := pm[n]
		if a.Mode != b.Mode {
			changed = append(changed, pgsChange{a, b, "模式变化"})
		} else if a.Scope != b.Scope {
			changed = append(changed, pgsChange{a, b, "scope 变化（同一道门归到别的 scope）"})
		}
	}
	curScopes := map[string]bool{}
	for _, s := range cur {
		curScopes[s.Scope] = true
	}
	noteSet := map[string]bool{}
	for _, s := range prop {
		if !curScopes[s.Scope] {
			keys := []string{}
			for k := range curScopes {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			noteSet[fmt.Sprintf("提案引入了现表里没有的 scope：`%s`（现表闭集 = %s）—— 那要改 `build_steps` 的 `case` 分支（本件只预览，不判；线要不要接见波④）",
				s.Scope, strings.Join(keys, " · "))] = true
		}
	}
	for k := range noteSet {
		notes = append(notes, k)
	}
	sort.Strings(notes)
	return
}

// pgsPadRight —— py `%-Ns`（按**码点**补空格）。
func pgsPadRight(s string, w int) string {
	n := len([]rune(s))
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

func pgsBuildDiff(cur, prop []pgsStep, curSrc, propPath string) (added, removed []pgsStep, changed []pgsChange, notes, L []string) {
	L = []string{}
	L = append(L, "── 门禁步骤表只读预览（`R-02` · **不写任何件**）──")
	L = append(L, fmt.Sprintf("   现状真源：%s", curSrc))
	L = append(L, fmt.Sprintf("   提案：`%s`（%d 步）", propPath, len(prop)))
	added, removed, changed, notes = pgsDiffSteps(cur, prop)
	L = append(L, fmt.Sprintf("   +%d 步（**将新增**）", len(added)))
	for _, s := range added {
		L = append(L, fmt.Sprintf("     + %s %s %s", pgsPadRight(s.Scope, 8), pgsPadRight(s.Mode, 11), s.Name))
	}
	L = append(L, fmt.Sprintf("   -%d 步（**将删**）", len(removed)))
	for _, s := range removed {
		L = append(L, fmt.Sprintf("     - %s %s %s", pgsPadRight(s.Scope, 8), pgsPadRight(s.Mode, 11), s.Name))
	}
	L = append(L, fmt.Sprintf("   ~%d 处（**模式变化 / scope 变化**）", len(changed)))
	for _, c := range changed {
		if c.A.Mode != c.B.Mode {
			L = append(L, fmt.Sprintf("     ~ 「%s」模式 %s → %s（scope %s）", c.A.Name, c.A.Mode, c.B.Mode, c.A.Scope))
		} else {
			L = append(L, fmt.Sprintf("     ~ 「%s」scope %s → %s（模式 %s 未变）", c.A.Name, c.A.Scope, c.B.Scope, c.A.Mode))
		}
	}
	for _, n := range notes {
		L = append(L, fmt.Sprintf("   ⚠ %s", n))
	}
	net := len(prop) - len(cur)
	L = append(L, fmt.Sprintf("   ⇒ 逐条 diff：**+%d 步 / -%d 步 / 模式变化 %d 处**（净 %+d）",
		len(added), len(removed), len(changed), net))
	L = append(L, fmt.Sprintf("   ⇒ 步数：现状 %d → 提案 %d", len(cur), len(prop)))
	if len(added) == 0 && len(removed) == 0 && len(changed) == 0 {
		L = append(L, "   ⇒ 逐字相同：提案与现状**一步不差**")
	}
	return
}

// ── 提案骨架 / 写意图（py L234/L256）────────────────────────────────────────
func pgsDoTemplate(root string, a *pgsArgs, stdout io.Writer) int {
	steps, src, err := pgsLoadCurrent(root, a)
	if err != "" {
		fmt.Fprintf(stdout, "⛔ 现状真源取不到：%s ⇒ 不给结论（rc=2）\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "# 门禁步骤表提案骨架（现跑真源 = %s · **只印不写**）\n", src)
	fmt.Fprintf(stdout, "# 一行一步：scope ⇥ mode ⇥ 名称；模式闭集 = %s\n", strings.Join(pgsModeClosed, " / "))
	fmt.Fprint(stdout, "# 改完存成分件，`--proposal <件>` 就能出逐条 diff；**写面要人签（本波不做）** ✗\n")
	for _, s := range steps {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", s.Scope, s.Mode, s.Name)
	}
	fmt.Fprintf(stdout, "# 共 %d 步\n", len(steps))
	return 0
}

func pgsWriteIntentRefusal(a *pgsArgs, targetPath string) (int, []string) {
	out := []string{}
	if !a.yes {
		out = append(out, "判①：**未带 `--yes`** ⇒ 不写")
	}
	if a.approval == "" {
		out = append(out, "判②：**无人签批准件**（没给 `--approval <件>`）⇒ 不写")
	} else if !pgsIsFile(a.approval) {
		out = append(out, fmt.Sprintf("判②：批准件不在（`%s` 读不到）⇒ **没人签** ⇒ 不写", a.approval))
	}
	if a.yes && a.approval != "" && pgsIsFile(a.approval) {
		out = append(out, "判③：`--yes` 与批准件都在 ⇒ 但**写面本波不做**（只读半件 · 加步 = 动所有人的必经之路 ⇒ 要人签）⇒ 人签核验器未落地 ⇒ **fail-closed 不写**")
	}
	out = append(out, fmt.Sprintf("目标件（一字未动）：`%s`", targetPath))
	return 2, out
}

// ── JSON 机器面（复刻 py json.dumps(ensure_ascii=False, indent=2)）────────────
type pgsKV struct {
	K string
	V any
}
type pgsObj []pgsKV
type pgsArr []any

func pgsJStr(s string) string {
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

func pgsJSONEncode(v any, lvl int) string {
	ind := func(n int) string { return strings.Repeat("  ", n) }
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return pgsJStr(x)
	case int:
		return strconv.Itoa(x)
	case pgsObj:
		if len(x) == 0 {
			return "{}"
		}
		parts := make([]string, 0, len(x))
		for _, kv := range x {
			parts = append(parts, ind(lvl+1)+pgsJStr(kv.K)+": "+pgsJSONEncode(kv.V, lvl+1))
		}
		return "{\n" + strings.Join(parts, ",\n") + "\n" + ind(lvl) + "}"
	case pgsArr:
		if len(x) == 0 {
			return "[]"
		}
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, ind(lvl+1)+pgsJSONEncode(e, lvl+1))
		}
		return "[\n" + strings.Join(parts, ",\n") + "\n" + ind(lvl) + "]"
	default:
		return pgsJStr(fmt.Sprint(v))
	}
}

func pgsStepsToArr(steps []pgsStep) pgsArr {
	out := pgsArr{}
	for _, s := range steps {
		out = append(out, pgsObj{{"scope", s.Scope}, {"mode", s.Mode}, {"name", s.Name}})
	}
	return out
}

func pgsChangesToArr(ch []pgsChange) pgsArr {
	out := pgsArr{}
	for _, c := range ch {
		out = append(out, pgsObj{
			{"name", c.B.Name}, {"from_mode", c.A.Mode}, {"to_mode", c.B.Mode},
			{"from_scope", c.A.Scope}, {"to_scope", c.B.Scope},
		})
	}
	return out
}

func pgsStringsToArr(ss []string) pgsArr {
	out := pgsArr{}
	for _, s := range ss {
		out = append(out, s)
	}
	return out
}

// ── argv 解析（复刻 argparse 的常用面 · 无前缀缩写）────────────────────────────
type pgsArgs struct {
	proposal  string
	template  bool
	listCache string
	listCmd   string
	target    string
	apply     bool
	write     bool
	commit    bool
	yes       bool
	approval  string
	root      string
	json      bool
	selfTest  bool
}

func pgsArgError(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, pgsUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", pgsProg, msg)
	return 2
}

func pgsParseArgs(stdout, stderr io.Writer, argv []string) (*pgsArgs, int, bool) {
	a := &pgsArgs{target: pgsDefaultTarget, root: "."}
	unrecognized := []string{}
	i := 0
	for i = 0; i < len(argv); i++ {
		arg := argv[i]
		name, inline, hasInline := arg, "", false
		if strings.HasPrefix(arg, "--") {
			if eq := strings.Index(arg, "="); eq >= 0 {
				name, inline, hasInline = arg[:eq], arg[eq+1:], true
			}
		}
		needVal := func() (string, bool) {
			if i+1 < len(argv) {
				i++
				return argv[i], true
			}
			pgsArgError(stderr, "argument "+name+": expected one argument")
			return "", false
		}
		switch name {
		case "-h", "--help":
			fmt.Fprint(stdout, pgsHelp)
			return nil, 0, true
		case "--proposal", "--list-cache", "--list-cmd", "--target", "--approval", "--root":
			v := ""
			if hasInline {
				v = inline
			} else {
				var ok bool
				if v, ok = needVal(); !ok {
					return nil, 2, true
				}
			}
			switch name {
			case "--proposal":
				a.proposal = v
			case "--list-cache":
				a.listCache = v
			case "--list-cmd":
				a.listCmd = v
			case "--target":
				a.target = v
			case "--approval":
				a.approval = v
			case "--root":
				a.root = v
			}
		case "--template", "--apply", "--write", "--commit", "--yes", "--json", "--self-test":
			if hasInline {
				return nil, pgsArgError(stderr, "argument "+name+": ignored explicit argument '"+inline+"'"), true
			}
			switch name {
			case "--template":
				a.template = true
			case "--apply":
				a.apply = true
			case "--write":
				a.write = true
			case "--commit":
				a.commit = true
			case "--yes":
				a.yes = true
			case "--json":
				a.json = true
			case "--self-test":
				a.selfTest = true
			}
		default:
			unrecognized = append(unrecognized, arg)
		}
	}
	if len(unrecognized) > 0 {
		pgsArgError(stderr, "unrecognized arguments: "+strings.Join(unrecognized, " "))
		return nil, 2, true
	}
	return a, 0, false
}

// ── 入口（py L272 `main`）────────────────────────────────────────────────────
func pgsGateMain(stdout, stderr io.Writer, argv []string) int {
	a, rc, done := pgsParseArgs(stdout, stderr, argv)
	if done {
		return rc
	}
	if a.selfTest {
		fmt.Fprint(stdout, "── preview-gate-steps 自检（合成件 · 不碰真目标）──\n")
		return pgsSelfTest(stdout)
	}
	cwd, _ := os.Getwd()
	root := pgsPyAbspath(cwd, a.root)
	target := pgsPyJoin(root, a.target)
	if !pgsIsFile(target) {
		fmt.Fprintf(stdout, "⛔ 目标件不在：%s ⇒ 不给结论（rc=2）\n", a.target)
		return 2
	}
	shaPre, ok := pgsSHA256File(target)
	if !ok {
		fmt.Fprintf(stdout, "⛔ 目标件读不了：%s ⇒ 不给结论（rc=2）\n", a.target)
		return 2
	}
	intent := a.apply || a.write || a.commit || a.yes || a.approval != ""
	if intent {
		rc, lines := pgsWriteIntentRefusal(a, a.target)
		for _, ln := range lines {
			fmt.Fprintf(stdout, "   ⛔ %s\n", ln)
		}
		fmt.Fprintf(stdout, "⇒ **不写**（rc=%d）：写面要人签 ⇒ 本波只做**只读半件**\n", rc)
		shaPost, _ := pgsSHA256File(target)
		verdict := "**变了 ⇒ 判红** ✗"
		if shaPost == shaPre {
			verdict = "一字不变 ✓"
		}
		fmt.Fprintf(stdout, "   只读性自证：目标件 `%s` sha256 %s…（跑前） = %s…（跑后）⇒ %s\n",
			a.target, shaPre[:12], shaPost[:12], verdict)
		return rc
	}
	if a.template && a.proposal == "" {
		return pgsDoTemplate(root, a, stdout)
	}
	if a.proposal == "" {
		steps, src, err := pgsLoadCurrent(root, a)
		if err != "" {
			fmt.Fprintf(stdout, "⛔ 现状真源取不到：%s ⇒ 不给结论（rc=2）\n", err)
			return 2
		}
		fmt.Fprint(stdout, "── 门禁步骤表（只读预览 · 无提案）──\n")
		fmt.Fprintf(stdout, "   真源：%s\n", src)
		fmt.Fprintf(stdout, "   ⇒ %d 步（要看差请给 `--proposal <件>`；要提案骨架用 `--template`）\n", len(steps))
		fmt.Fprintf(stdout, "   ⇒ 只读（目标件 sha256 %s… · 一字未动）\n", shaPre[:12])
		return 0
	}
	steps, src, err := pgsLoadCurrent(root, a)
	if err != "" {
		fmt.Fprintf(stdout, "⛔ 现状真源取不到：%s ⇒ 不给结论（rc=2）\n", err)
		return 2
	}
	prop, perr := pgsParseProposal(a.proposal)
	if perr != "" {
		fmt.Fprintf(stdout, "⛔ 提案解不动：%s ⇒ 不给结论（rc=2）\n", perr)
		return 2
	}
	added, removed, changed, notes, lines := pgsBuildDiff(steps, prop, src, a.proposal)
	if !a.json {
		for _, ln := range lines {
			fmt.Fprintln(stdout, ln)
		}
	} else {
		obj := pgsObj{
			{"id", "gate-steps-diff.v1"},
			{"current_source", src},
			{"proposal", a.proposal},
			{"steps_current", len(steps)},
			{"steps_proposal", len(prop)},
			{"added", pgsStepsToArr(added)},
			{"removed", pgsStepsToArr(removed)},
			{"changed", pgsChangesToArr(changed)},
			{"notes", pgsStringsToArr(notes)},
		}
		fmt.Fprintln(stdout, pgsJSONEncode(obj, 0))
	}
	shaPost, _ := pgsSHA256File(target)
	verdict := "**变了 ⇒ 判红** ✗"
	if shaPost == shaPre {
		verdict = "一字不变 ✓"
	}
	fmt.Fprintf(stdout, "   只读性自证：目标件 `%s` sha256 %s… → %s… ⇒ %s\n", a.target, shaPre[:12], shaPost[:12], verdict)
	fmt.Fprint(stdout, "⇒ 只读预览跑成（rc=0）：差是事实；**写面要人签 ⇒ 本波不做** ✗\n")
	return 0
}

// ── `--self-test`（py L362）─────────────────────────────────────────────────
const pgsFXList = `自检结论: 全过（断言 3 条 · 失败 0 条）
── 步骤清单（scope 模式 名称）──
   go    empty  gofmt -l core
   gates tri    门① 覆盖（阻断）
   gates tri    门② 版本源（阻断）
   docs  tri-report docs: glossary 术语表 zh+en（只报告）
共 4 步
`

type pgsSel struct {
	stdout io.Writer
	fails  []string
}

func (s *pgsSel) caseEq(name string, ok bool, gotRepr, wantRepr string) {
	if ok {
		fmt.Fprintf(s.stdout, "   ✓ %s\n", name)
		return
	}
	fmt.Fprintf(s.stdout, "   ✗ %s（got=%s want=%s）\n", name, gotRepr, wantRepr)
	s.fails = append(s.fails, name)
}

func pgsRStepDict(s pgsStep) string {
	return "{'scope': " + pgsPyRepr(s.Scope) + ", 'mode': " + pgsPyRepr(s.Mode) + ", 'name': " + pgsPyRepr(s.Name) + "}"
}

func pgsRStrs(ss []string) string {
	parts := []string{}
	for _, s := range ss {
		parts = append(parts, pgsPyRepr(s))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func pgsRTupleRepr(vals ...string) string { return "(" + strings.Join(vals, ", ") + ")" }

func pgsSelfTest(stdout io.Writer) int {
	s := &pgsSel{stdout: stdout}
	tmp, err := os.MkdirTemp("", "zerg-preview-steps-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "自检结论: **有失败**（失败 1 条）\n")
		return 2
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	cur := pgsPyJoin(tmp, "list.txt")
	_ = os.WriteFile(cur, []byte(pgsFXList), 0o644)

	// ① 真源解析 + 计数自证
	steps, total, hasTotal, perr := pgsParseList(pgsFXList)
	ok1 := len(steps) == 4 && hasTotal && total == 4 && perr == ""
	s.caseEq("① `--list` 解出 4 步", ok1,
		pgsRTupleRepr(strconv.Itoa(len(steps)), pgsTotalRepr(total, hasTotal), pgsRNilOrStr(perr)),
		"(4, 4, None)")
	var got2 pgsStep
	if len(steps) > 1 {
		got2 = steps[1]
	}
	want2 := pgsStep{Scope: "gates", Mode: "tri", Name: "门① 覆盖（阻断）"}
	// ★ ② 冻结下限（棘轮 · 只许增/降）：`--list` 现读解出 4 步（取值＝当次现读基线）——
	//   结构静默变小（解出的条数 < 此下限）⇒ 本格判红，✗ 不许静默绿。
	const pgsListFloor = 4
	if n := len(steps); n < pgsListFloor {
		s.caseEq("① 三格逐条解对：步数 ≥ 冻结下限", false, strconv.Itoa(n), strconv.Itoa(pgsListFloor))
	}
	s.caseEq("① 三格逐条解对（含中文名里的空格）", len(steps) > 1 && got2 == want2,
		pgsRStepDict(got2), pgsRStepDict(want2))
	_, _, _, e3 := pgsParseList(strings.Replace(pgsFXList, "共 4 步", "共 5 步", 1))
	s.caseEq("① 负控：现印步数与逐行解出的条数不符 ⇒ 判不了", e3 != "", pgsReprBool(e3 != ""), "True")
	_, _, _, e4 := pgsParseList("nothing here\n")
	s.caseEq("① 负控：没有「步骤清单」段 ⇒ 判不了", e4 != "", pgsReprBool(e4 != ""), "True")

	// ② 提案：TSV 两形
	pTSV := pgsPyJoin(tmp, "p.tsv")
	_ = os.WriteFile(pTSV, []byte("# 提案：加一步 · 删一步 · 改一档\n"+
		"go\tempty\tgofmt -l core\n"+
		"gates\ttri\t门① 覆盖（阻断）\n"+
		"gates\ttri\t门② 版本源（阻断）\n"+
		"gates\ttri-report\t门㉑ 新门：只报告起步\n"), 0o644)
	prop, perr2 := pgsParseProposal(pTSV)
	ok2 := len(prop) == 4 && perr2 == ""
	gotLen := "None"
	if len(prop) > 0 {
		gotLen = strconv.Itoa(len(prop))
	}
	s.caseEq("② TSV 提案解出 4 步", ok2, pgsRTupleRepr(gotLen, pgsRNilOrStr(perr2)), "(4, None)")
	steps0, _, _, _ := pgsParseList(pgsFXList)
	added, removed, _, _ := pgsDiffSteps(steps0, prop)
	s.caseEq("② 逐条 diff：+1 步", pgsAddedNames(added),
		pgsRStrs(pgsStepNames(added)), pgsRStrs([]string{"门㉑ 新门：只报告起步"}))
	s.caseEq("② 逐条 diff：-1 步", pgsRemovedNames(removed),
		pgsRStrs(pgsStepNames(removed)), pgsRStrs([]string{"docs: glossary 术语表 zh+en（只报告）"}))

	// ③ 模式变化
	pMode := pgsPyJoin(tmp, "mode.tsv")
	_ = os.WriteFile(pMode, []byte("go\tempty\tgofmt -l core\n"+
		"gates\ttri-report\t门① 覆盖（阻断）\n"+
		"gates\ttri\t门② 版本源（阻断）\n"+
		"docs\ttri-report\tdocs: glossary 术语表 zh+en（只报告）\n"), 0o644)
	prop2, _ := pgsParseProposal(pMode)
	a2, r2, c2, _ := pgsDiffSteps(steps0, prop2)
	s.caseEq("③ 模式变化被点出（1 处）", len(c2) == 1, strconv.Itoa(len(c2)), "1")
	got3 := pgsRTupleRepr("", "", "")
	if len(c2) > 0 {
		got3 = pgsRTupleRepr(pgsPyRepr(c2[0].Why), pgsPyRepr(c2[0].A.Mode), pgsPyRepr(c2[0].B.Mode))
	}
	s.caseEq("③ 变化那一步点名到「模式变化」", len(c2) == 1 && c2[0].Why == "模式变化" && c2[0].A.Mode == "tri" && c2[0].B.Mode == "tri-report",
		got3, "('模式变化', 'tri', 'tri-report')")
	s.caseEq("③ 模式变化**不算**增删（+0/-0）", len(a2) == 0 && len(r2) == 0,
		pgsRTupleRepr(strconv.Itoa(len(a2)), strconv.Itoa(len(r2))), "(0, 0)")

	// ④ scope 变化
	pScope := pgsPyJoin(tmp, "scope.tsv")
	_ = os.WriteFile(pScope, []byte("go\tempty\tgofmt -l core\n"+
		"release\ttri\t门① 覆盖（阻断）\n"+
		"gates\ttri\t门② 版本源（阻断）\n"+
		"docs\ttri-report\tdocs: glossary 术语表 zh+en（只报告）\n"), 0o644)
	prop3, _ := pgsParseProposal(pScope)
	_, _, c3, _ := pgsDiffSteps(steps0, prop3)
	got4 := pgsRTupleRepr("", "")
	if len(c3) > 0 {
		got4 = pgsRTupleRepr(strconv.Itoa(len(c3)), pgsPyRepr(c3[0].Why))
	}
	s.caseEq("④ scope 变化被点出", len(c3) == 1 && c3[0].Why == "scope 变化（同一道门归到别的 scope）",
		got4, "(1, 'scope 变化（同一道门归到别的 scope）')")

	// ⑤ 逐字相同
	pSame := pgsPyJoin(tmp, "same.tsv")
	_ = os.WriteFile(pSame, []byte("go\tempty\tgofmt -l core\n"+
		"gates\ttri\t门① 覆盖（阻断）\n"+
		"gates\ttri\t门② 版本源（阻断）\n"+
		"docs\ttri-report\tdocs: glossary 术语表 zh+en（只报告）\n"), 0o644)
	prop4, _ := pgsParseProposal(pSame)
	a4, r4, c4, _ := pgsDiffSteps(steps0, prop4)
	s.caseEq("⑤ 逐字相同 ⇒ +0/-0/变化 0", len(a4) == 0 && len(r4) == 0 && len(c4) == 0,
		pgsRTupleRepr(strconv.Itoa(len(a4)), strconv.Itoa(len(r4)), strconv.Itoa(len(c4))), "(0, 0, 0)")

	// ⑥ 负控：模式名在闭集外
	pBadMode := pgsPyJoin(tmp, "badmode.tsv")
	_ = os.WriteFile(pBadMode, []byte("gates\t阻断\t门① 覆盖（阻断）\n"), 0o644)
	_, pe6 := pgsParseProposal(pBadMode)
	s.caseEq("⑥ 负控：模式名在闭集外 ⇒ 判不了", pe6 != "", pgsReprBool(pe6 != ""), "True")

	// ⑦ 负控：缺列 / 重名 / 空表
	pMiss := pgsPyJoin(tmp, "miss.tsv")
	_ = os.WriteFile(pMiss, []byte("gates\ttri\n"), 0o644)
	_, pe7a := pgsParseProposal(pMiss)
	s.caseEq("⑦ 负控：缺格 ⇒ 判不了", pe7a != "", pgsReprBool(pe7a != ""), "True")
	pDup := pgsPyJoin(tmp, "dup.tsv")
	_ = os.WriteFile(pDup, []byte("gates\ttri\t门① 覆盖（阻断）\ngates\ttri\t门① 覆盖（阻断）\n"), 0o644)
	_, pe7b := pgsParseProposal(pDup)
	s.caseEq("⑦ 负控：重名 ⇒ 判不了", pe7b != "", pgsReprBool(pe7b != ""), "True")
	pEmpty := pgsPyJoin(tmp, "empty.tsv")
	_ = os.WriteFile(pEmpty, []byte(""), 0o644)
	_, pe7c := pgsParseProposal(pEmpty)
	s.caseEq("⑦ 负控：空表 ⇒ 判不了（不许当「全删」）", pe7c != "", pgsReprBool(pe7c != ""), "True")
	pJSON := pgsPyJoin(tmp, "p.json")
	_ = os.WriteFile(pJSON, []byte(`{"id": "proposal.v1", "steps": [{"scope": "gates", "mode": "tri", "name": "门① 覆盖（阻断）"}]}`), 0o644)
	pjSteps, pjErr := pgsParseProposal(pJSON)
	s.caseEq("⑦ JSON 提案同解", pjErr == "" && len(pjSteps) == 1, strconv.Itoa(len(pjSteps)), "1")
	pJSONBad := pgsPyJoin(tmp, "bad.json")
	_ = os.WriteFile(pJSONBad, []byte("{不是 json"), 0o644)
	_, pe7d := pgsParseProposal(pJSONBad)
	s.caseEq("⑦ 负控：JSON 解不动 ⇒ 判不了", pe7d != "", pgsReprBool(pe7d != ""), "True")

	// ⑧ 写意图三档
	tgt := pgsPyJoin(tmp, "fake-gate.sh")
	_ = os.WriteFile(tgt, []byte("#!/usr/bin/env bash\nadd_step gates \"门①\" tri . true\n"), 0o644)
	sh, _ := pgsSHA256File(tgt)
	mk := func(f func(*pgsArgs)) *pgsArgs {
		x := &pgsArgs{target: "fake-gate.sh", root: tmp}
		if f != nil {
			f(x)
		}
		return x
	}
	rc8a, lines8a := pgsWriteIntentRefusal(mk(func(x *pgsArgs) { x.apply = true }), "fake-gate.sh")
	s.caseEq("⑧ 写意图（未带 --yes）⇒ rc=2", rc8a == 2, strconv.Itoa(rc8a), "2")
	s.caseEq("⑧ 判词点明「未带 --yes」", pgsAnyContains(lines8a, "未带 `--yes`"), pgsReprBool(pgsAnyContains(lines8a, "未带 `--yes`")), "True")
	rc8b, lines8b := pgsWriteIntentRefusal(mk(func(x *pgsArgs) { x.apply = true; x.yes = true }), "fake-gate.sh")
	s.caseEq("⑧ 写意图（有 --yes · 无批准件）⇒ rc=2", rc8b == 2, strconv.Itoa(rc8b), "2")
	s.caseEq("⑧ 判词点明「无人签批准件」", pgsAnyContains(lines8b, "无人签批准件"), pgsReprBool(pgsAnyContains(lines8b, "无人签批准件")), "True")
	apFile := pgsPyJoin(tmp, "approval.json")
	_ = os.WriteFile(apFile, []byte("{\"signed_by\": \"没人\", \"note\": \"合成件\"}\n"), 0o644)
	rc8c, lines8c := pgsWriteIntentRefusal(mk(func(x *pgsArgs) { x.apply = true; x.yes = true; x.approval = apFile }), "fake-gate.sh")
	s.caseEq("⑧ 写意图（--yes + 批准件都在）⇒ **仍 rc=2**（写面本波不做 · fail-closed）", rc8c == 2, strconv.Itoa(rc8c), "2")
	s.caseEq("⑧ 判词点明「写面本波不做」", pgsAnyContains(lines8c, "写面本波不做"), pgsReprBool(pgsAnyContains(lines8c, "写面本波不做")), "True")
	sh8, _ := pgsSHA256File(tgt)
	s.caseEq("⑧ 三档都跑完：目标件 sha256 一字不变", sh8 == sh, "sha256("+sh8[:12]+")", "sha256("+sh[:12]+")")

	// ⑩ 名称里的 `#` 不被截断
	pHash := pgsPyJoin(tmp, "hash.tsv")
	_ = os.WriteFile(pHash, []byte("# 整行注释\n"+
		"gates\ttri\tscripts/*（无后缀 + 首行 #!）按 shebang 语法\n"), 0o644)
	prop5, perr5 := pgsParseProposal(pHash)
	got10 := pgsPyRepr(perr5)
	if len(prop5) > 0 {
		got10 = pgsPyRepr(prop5[0].Name)
	}
	s.caseEq("⑩ 名称里的 `#` 不被截断", len(prop5) > 0 && prop5[0].Name == "scripts/*（无后缀 + 首行 #!）按 shebang 语法",
		got10, pgsPyRepr("scripts/*（无后缀 + 首行 #!）按 shebang 语法"))

	// ⑨ 只读预览真跑一遍
	rc9 := pgsGateMain(stdout, io.Discard, []string{"--root", tmp, "--list-cache", cur, "--proposal", pTSV, "--target", "fake-gate.sh"})
	s.caseEq("⑨ 只读预览 ⇒ rc=0", rc9 == 0, strconv.Itoa(rc9), "0")
	sh9, _ := pgsSHA256File(tgt)
	s.caseEq("⑨ 只读预览跑完：目标件 sha256 一字不变", sh9 == sh, "sha256("+sh9[:12]+")", "sha256("+sh[:12]+")")

	if len(s.fails) == 0 {
		fmt.Fprint(stdout, "自检结论: 全过（失败 0 条）\n")
		return 0
	}
	fmt.Fprintf(stdout, "自检结论: **有失败**（失败 %d 条）\n", len(s.fails))
	return 2
}

func pgsRNilOrStr(s string) string {
	if s == "" {
		return "None"
	}
	return pgsPyRepr(s)
}

func pgsStepNames(steps []pgsStep) []string {
	out := []string{}
	for _, s := range steps {
		out = append(out, s.Name)
	}
	return out
}

func pgsAddedNames(added []pgsStep) bool {
	return len(added) == 1 && added[0].Name == "门㉑ 新门：只报告起步"
}

func pgsRemovedNames(removed []pgsStep) bool {
	return len(removed) == 1 && removed[0].Name == "docs: glossary 术语表 zh+en（只报告）"
}

func pgsAnyContains(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}
