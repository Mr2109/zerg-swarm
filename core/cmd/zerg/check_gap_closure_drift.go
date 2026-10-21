// check_gap_closure_drift.go —— 判据件 `scripts/gates/check-gap-closure-drift.py`（169 行）的 **Go 真搬**
// （v2.5.14 · M2 · 批-3【搬运】粒 6 · 桶 f–j · 入口 `checkGapClosureDriftGateMain` · 承接缺口 `GAP-20261001-13`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的 argparse 面差异
//	与未覆盖面）。
//
// 选它的理由（本单口径：真未搬清单里 **f–j 桶行数最小的纯静态件**）：
//
//	✓ 选：判据是**纯静态只读面** —— 只读 `<ledger>`（一句 jsonl，默认 `~/.zerg/state/zerg-cli-gaps.jsonl`）
//	  逐行 `json.loads`，再对每行 `state` 含「仍缺」者的 `unit` 段做 `os.path.isfile/isdir` 存在性判定。
//	  **不真跑子进程、不碰网络、零计时、零随机（默认档）、默认档零写盘**（`--self-test` 只在临时目录里造件）。
//	· 无外部树依赖：`--ledger` / `--root`（env `ZERG_ROOT`）可显式指到任意合成假仓根与假真源
//	  （本批口径：外部树走显式入参旋钮，旋钮名照 py 同名旗标）。
//	· 退码 0/1/2 三档齐 · 纯只读无写面 · 行数最小（169 行）· `git status` 现读不含它（无人正在改）。
//	· 现读 import 面：argparse / json / os / sys / tempfile —— **零 `subprocess` / `time` / `datetime` /
//	  `random` / `socket` / `urllib`**。`tempfile.TemporaryDirectory`（`with` 形）**自带回收** ⇒ **非** A 类
//	  泄漏形态（逐字保留其自回收语义；单测另钉一条「自检后临时目录 0 残留」的牙）。
//
// 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `gate_inproc_table.go` 表）；
//	· **不**删 `scripts/gates/check-gap-closure-drift.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· **已接线**：入口 `checkGapClosureDriftGateMain` 由 `gateCheckGoEntry["check-gap-closure-drift"]` 分派（`zerg gate check check-gap-closure-drift`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//	· ✗ 不改登记面 · ✗ 不改任何 `family_*.go` / 已落旧件 / `gate_inproc_table.go`。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkGapClosureDriftGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 无漂移 / 1 有漂移（逐条点名）/ 2 缺语料（不给结论）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「未覆盖面」）
//
// ═══════════════════════ ★ 遍历序（本件无目录遍历）—— 逐点交代 ═══════════════════════
//
//	py 本件**零 `os.walk` / `os.listdir` / `glob`**：只 `open()` 单个已知真源、按行扫描，再对每条 unit
//	做**单点** `os.path.isfile/isdir` 判定（无目录枚举）⇒ **无遍历序面**、**零 Go map 迭代进输出**。
//	★ 输出行序的决定者 = py `rows` 列表序（jsonl 物理行序，list 保序）⟷ 命中序 = 逐行 × 逐段首遇序
//	  ⇒ 本件用**切片**（`[]cgcdHit`）保序，✗ 无任何 Go map 迭代进输出（`allow` map 只做 O(1) 查值）。
//
// ═══════════════════════ ★ 声明 (a) 默认根定位差异 ═══════════════════════
//
//	py `--root` 缺省 = `os.environ.get("ZERG_ROOT", os.getcwd())` ⇒ **起于 cwd**（不是 `__file__` 上溯）。
//	Go 无 `__file__` 语境，但本件 py 本就用 cwd ⇒ 走 **`os.LookupEnv("ZERG_ROOT")` + `os.Getwd()`** 逐字同构
//	（`LookupEnv` 保真「键在但空串」档：py `.get(k, default)` 对显式空串回 `""`、Go 亦回 `""` ——
//	✗ 不用 `os.Getenv`，它把「键在空串」与「键缺」并成同形）。
//	⚠ 未覆盖面：py `os.getcwd()` ⟷ Go `os.Getwd()`（后者优先 `$PWD` 若其 `stat` 命中 `.`）在**符号链接路径**
//	下可能不同 ⇒ 对拍/夹具一律落**非符号链接路径**。
//
// ═══════════════════════ ★ 声明 (b) 自指检查 —— 本件**不在**自己的扫描面内（四形态逐条）═══════════════════════
//
//	① 扫描面 = `<ledger>` 单件（判据对象是真源 jsonl，不在仓内）⇒ 本件（`core/cmd/zerg/*.go`）不在内 ⇒ 自指 = 假。
//	② 件头/正文**零真引号路径原串**落进被扫面（本件只把 ledger 路径当**输入**读）。
//	③ self-test 魔法串（`GAP-T-*`）落点 = **临时目录**，不进仓树 ⇒ 真仓 `--self-test` 不受影响。
//	④ 是否落在**他门**扫描面内：本件在 `core/cmd/zerg/*.go` ⇒ 落 `check-error-kinds` 的 R3 面。R3 只认「错误
//	   构造调用的首参字面」（真源 API 调用式，**带左括号**）⇒ 本件**不写该调用式** ⇒ 命中 0。⚠ 也落
//	   `check-hardcoded-private-paths`（其 SCAN_DIRS 含 `core/`）⇒ 本件**不写任何私有绝对路径首段字面**
//	   （★ 此处按**描述式**写，✗ 不把原串当例子写出来，否则「我没写 X」这句本身就含 X）⇒ 命中 0。
//
// ═══════════════════════ ★ py 字符串原语逐点交代 ═══════════════════════
//
//	· `line.strip()` / `(… or "").strip()` / `seg.strip()`：py `str.strip()` 剥的是 py 的 **Unicode 空白**
//	  （含 `\x1c-\x1f` / `\x85` / `\xa0` / `Zs`）⟷ Go `strings.TrimSpace` 用 `unicode.IsSpace`（**不剥**
//	  `\x1c-\x1f`）⇒ 本件自写 `cgcdPyStrip`（`strings.TrimFunc` + `cgcdIsPySpace`），✗ 不用 `TrimSpace`。
//	· `open()` 文本模式 = **universal newlines**（`\r\n` 与裸 `\r` 都折成 `\n`）⇒ `cgcdSplitLedgerLines`
//	  先折行再切（✗ 直接按 `\n` 切会漏裸 `\r` 档）。
//	· 路径拼接：py `os.path.join(root, u)` ⇒ 本件 `cgcdPyJoin` 按 CPython posixpath.join 逐段规则直译
//	  （`u` 以 `/` 开头则**丢弃前面所有段** · 空段保留前缀斜杠 · 尾斜杠直连），✗ 不用 `filepath.Join`。
//	· `os.path.expanduser("~/…")`：`cgcdExpandUser` 照 posixpath 逐条（`~`/`~/` 用 `HOME`；键在空串则
//	  结果回 `/` 形；`~user` 形式**未搬**、如实列入未覆盖面）。
//	· `os.environ.get("ZERG_ROOT", os.getcwd())`：见声明 (a)。
//
// ═══════════════════════ ★ 未覆盖面（逐条如实声明）═══════════════════════
//
//	① **py 版本相关措辞**：`-h` 正文 py≤3.9 印 `optional arguments:`、py≥3.10 印 `options:`；本件按
//	   py3.9（本机 `/usr/bin/python3` = 3.9.6）复刻 ⇒ 门跑环境若用 py≥3.10，帮助面 stdout 分叉（rc 恒 0）。
//	② **usage/帮助折行依赖终端宽**：argparse 按 `COLUMNS` 折行（非 tty 缺省 80）⇒ 本件按宽 78 复刻折点；
//	   换 `COLUMNS`/真 TTY 则 usage 折法分叉（rc 不受影响）。
//	③ **JSON 解析错正文**：py `json.JSONDecodeError` 的 `str(e)` 措辞与 Go 不同 ⇒ 该档只保证 rc=2 与
//	   「真源第 N 行不是合法 JSON：」前缀，**ex 正文不逐字对齐**（已声明为口径差）。
//	④ **非对象 JSON 行**：py `json.loads("5")` 成功、随后 `row.get` 抛 `AttributeError`（**未捕 → 回溯 +
//	   rc=1**，C 类无兜底）；本件按新口径 fail-closed rc=2 ⇒ 该格只保证「不给结论」语义、正文不同。
//	⑤ **unit 为非字符串**：py `(5).strip()` 抛 `AttributeError`（rc=1）；本件回空段（rc 视余行）⇒ 同④处置。
//	⑥ **非 UTF-8 真源**：py `open(encoding="utf-8")` strict 抛 `UnicodeDecodeError`（未捕 → rc=1）；本件
//	   fail-closed rc=2 ⇒ 只保「不给结论」语义。
//	⑦ **`~user` 形 ledger**：本件未搬（回原串）⇒ 该档路径与 py 不同（合成夹具不触发）。
//	⑧ **符号链接路径下的缺省根 / 权限错正文**：见声明 (a) 与 `cgcdOSErr`（只复刻 EACCES/ENOENT 两档）。
//	⑨ **`--self-test` 档非确定面**：临时目录随机后缀 ⇒ 两态 stderr/stdout 掩码该段后逐字节同（已自证）。
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// cgcdProg —— argparse 的 prog 名（= py 原件基名）。
const cgcdProg = "check-gap-closure-drift.py"

// cgcdDesc —— argparse `description`（进 `-h` 正文）。
const cgcdDesc = "缺口「仍缺 ⟷ 落点件在盘」只读漂移门（GAP-20260927-122）"

// cgcdDefaultLedgerRel —— py `os.path.expanduser("~/.zerg/state/zerg-cli-gaps.jsonl")` 的 `~` 后段。
const cgcdDefaultLedgerRel = "/.zerg/state/zerg-cli-gaps.jsonl"

// cgcdPlaceholderUnits —— py `PLACEHOLDER_UNITS`（占位 unit，逐字）。
var cgcdPlaceholderUnits = []string{"无件(命令面)", "无件", "", "-"}

// cgcdIsPlaceholder —— py `raw in PLACEHOLDER_UNITS`。
func cgcdIsPlaceholder(s string) bool {
	for _, p := range cgcdPlaceholderUnits {
		if s == p {
			return true
		}
	}
	return false
}

// cgcdIsPySpace —— py `str.isspace()` 认可的空白（含 `\x1c-\x1f` / `\x85` / `\xa0` / Zs）。
func cgcdIsPySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0,
		0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007,
		0x2008, 0x2009, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return false
}

// cgcdPyStrip —— 复刻 py `str.strip()`（无参：剥 py Unicode 空白两端）。
func cgcdPyStrip(s string) string {
	return strings.TrimFunc(s, cgcdIsPySpace)
}

// cgcdPyJoin —— 复刻 CPython posixpath.join 的逐段规则（✗ 不用 filepath.Join）。
func cgcdPyJoin(a string, parts ...string) string {
	path := a
	for _, b := range parts {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

// cgcdExpandUser —— 复刻 posixpath.expanduser 的 `~` / `~/…` 两档（`~user` 未搬，见未覆盖面⑦）。
func cgcdExpandUser(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	i := strings.Index(p[1:], "/")
	if i < 0 {
		i = len(p)
	} else {
		i++
	}
	if i != 1 {
		return p // `~user` 形式：未搬（如实声明）
	}
	home, ok := os.LookupEnv("HOME")
	if !ok {
		if u, err := user.Current(); err == nil {
			home = u.HomeDir
		}
	}
	home = strings.TrimRight(home, "/")
	out := home + p[i:]
	if out == "" {
		return "/"
	}
	return out
}

// cgcdSplitLedgerLines —— 复刻「universal newlines 下按行迭代」的行集（尾空行由 `if not line` 过滤）。
func cgcdSplitLedgerLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// cgcdOSErr —— py `str(OSError)` 的两档复刻（EACCES / ENOENT）；其余 errno 回 Go 原文（见未覆盖面⑧）。
func cgcdOSErr(err error, path string) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "[Errno 13] Permission denied: '" + path + "'"
	case errors.Is(err, fs.ErrNotExist):
		return "[Errno 2] No such file or directory: '" + path + "'"
	default:
		return err.Error()
	}
}

// cgcdLoadRows —— py `load_rows`：读真源 jsonl ⇒ (rows, errMsg)。任一行解析失败 ⇒ errMsg（fail-closed）。
func cgcdLoadRows(ledger string) ([]map[string]any, string) {
	st, err := os.Stat(ledger)
	if err != nil || st.IsDir() {
		return nil, "真源不存在：" + ledger
	}
	data, err := os.ReadFile(ledger)
	if err != nil {
		return nil, "真源读不到：" + cgcdOSErr(err, ledger)
	}
	if !utf8.Valid(data) {
		return nil, "真源读不到：'" + ledger + "' 非 UTF-8（fail-closed）"
	}
	rows := []map[string]any{}
	for i, ln := range cgcdSplitLedgerLines(string(data)) {
		ln = cgcdPyStrip(ln)
		if ln == "" {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(ln), &v); err != nil {
			return nil, "真源第 " + strconv.Itoa(i+1) + " 行不是合法 JSON：" + err.Error()
		}
		obj, ok := v.(map[string]any)
		if !ok {
			return nil, "真源第 " + strconv.Itoa(i+1) + " 行不是合法 JSON：非对象行（口径差，见未覆盖面④）"
		}
		rows = append(rows, obj)
	}
	return rows, ""
}

// cgcdPyStr —— 复刻 py `str(x)`（仅用于 state 的 `str(r.get("state") or "")` 面）。
func cgcdPyStr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'g', -1, 64)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// cgcdTruthy —— py 真值（`X or Y` 的判定）。
func cgcdTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return t != ""
	case bool:
		return t
	case float64:
		return t != 0
	default:
		return true
	}
}

// cgcdUnitsOf —— py `units_of`：把 `unit` 拆成仓内相对路径列表（去占位 / 去空）。
func cgcdUnitsOf(row map[string]any) []string {
	raw := row["unit"]
	rawStr := ""
	if cgcdTruthy(raw) {
		if s, ok := raw.(string); ok {
			rawStr = s
		} else {
			rawStr = cgcdPyStr(raw) // 非字符串：py 会抛 AttributeError（见未覆盖面⑤），此处收敛
		}
	}
	rawStr = cgcdPyStrip(rawStr)
	if cgcdIsPlaceholder(rawStr) {
		return []string{}
	}
	out := []string{}
	for _, seg := range strings.Split(rawStr, ";") {
		seg = cgcdPyStrip(seg)
		if seg == "" || cgcdIsPlaceholder(seg) {
			continue
		}
		out = append(out, seg)
	}
	return out
}

// cgcdHit —— 一条漂移命中（py `(id, unit_on_disk)`）。
type cgcdHit struct {
	id   string
	unit string
}

// cgcdIDStr —— py `r.get("id") or "?"` 的字符串化（真源 id 恒为字符串；非字符串档见未覆盖面⑤）。
func cgcdIDStr(row map[string]any) string {
	v := row["id"]
	if cgcdTruthy(v) {
		if s, ok := v.(string); ok {
			return s
		}
		return cgcdPyStr(v)
	}
	return "?"
}

// cgcdDriftRows —— py `drift_rows`：回「仍缺且点名件在盘（不在 allow 里）」的命中（切片保序）。
func cgcdDriftRows(rows []map[string]any, root string, allow map[string]bool) []cgcdHit {
	hits := []cgcdHit{}
	for _, r := range rows {
		state := ""
		if cgcdTruthy(r["state"]) {
			state = cgcdPyStr(r["state"])
		}
		if !strings.Contains(state, "仍缺") {
			continue
		}
		rid := cgcdIDStr(r)
		if allow[rid] {
			continue
		}
		for _, u := range cgcdUnitsOf(r) {
			// 只认仓内相对路径（前导 `/` 或 `~` 是仓外路径 —— 那不是「仓内落件」）
			if strings.HasPrefix(u, "/") || strings.HasPrefix(u, "~") {
				continue
			}
			p := cgcdPyJoin(root, u)
			// py `os.path.isfile(p) or os.path.isdir(p)`：跟随符号链接（os.Stat，✗ 非 Lstat）。
			if st, err := os.Stat(p); err == nil && (st.Mode().IsRegular() || st.IsDir()) {
				hits = append(hits, cgcdHit{id: rid, unit: u})
			}
		}
	}
	return hits
}

// cgcdJSONStr —— py `json.dumps(s, ensure_ascii=False)` 的字符串编码（转义集逐字对齐）。
func cgcdJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				b.WriteString(`\u`)
				b.WriteString(fmt.Sprintf("%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// cgcdJSON —— py `json.dumps({...}, ensure_ascii=False)`（缺省分隔符 `, ` / `: `，键序 = 插入序）。
func cgcdJSON(hits []cgcdHit) string {
	var b strings.Builder
	rc := 0
	if len(hits) > 0 {
		rc = 1
	}
	b.WriteString(`{"rc": `)
	b.WriteString(strconv.Itoa(rc))
	b.WriteString(`, "drift_count": `)
	b.WriteString(strconv.Itoa(len(hits)))
	b.WriteString(`, "drift": [`)
	for i, h := range hits {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`{"id": `)
		b.WriteString(cgcdJSONStr(h.id))
		b.WriteString(`, "unit": `)
		b.WriteString(cgcdJSONStr(h.unit))
		b.WriteString(`}`)
	}
	b.WriteString(`]}`)
	return b.String()
}

// cgcdRun —— py `run`：0 无漂移 / 1 有漂移 / 2 缺语料。
func cgcdRun(stdout, stderr io.Writer, ledger, root string, allow map[string]bool, asJSON bool) int {
	rows, errMsg := cgcdLoadRows(ledger)
	if errMsg != "" {
		fmt.Fprintf(stderr, "BLOCKED —— %s ⇒ rc=2 不给结论\n", errMsg)
		return 2
	}
	if len(rows) == 0 {
		fmt.Fprintln(stderr, "BLOCKED —— 真源解析出 0 行（空域）⇒ rc=2 不给结论")
		return 2
	}
	hits := cgcdDriftRows(rows, root, allow)
	if asJSON {
		fmt.Fprintln(stdout, cgcdJSON(hits))
	} else {
		fmt.Fprintln(stdout, "缺口「仍缺 ⟷ 落点件在盘」漂移门（GAP-20260927-122）")
		fmt.Fprintf(stdout, "真源 %s · 读入 %d 行 · 仍缺且点名件在盘 = %d 条\n", ledger, len(rows), len(hits))
		for _, h := range hits {
			fmt.Fprintf(stdout, "  ✗ %s：点名件已在盘 ⇒ %s（账滞后 or 未收口；核后改账或经 --allow 显式降噪）\n", h.id, h.unit)
		}
	}
	if len(hits) > 0 {
		return 1
	}
	return 0
}

// cgcdSelfTest —— py `self_test()`：正控命中 1 / 负控 allow 0 / 负控 缺语料 rc=2 / 正控 rc=1（合成件全在临时目录）。
// ★ 临时目录前缀用 `tmp`（逐字对齐 py `tempfile.TemporaryDirectory()` 的缺省 prefix）⇒ 两态只差随机后缀。
func cgcdSelfTest(stdout, stderr io.Writer) int {
	ok := true
	td, err := os.MkdirTemp("", "tmp")
	if err != nil {
		fmt.Fprintln(stdout, "自检 FAIL（4 格）")
		return 1
	}
	defer func() { _ = os.RemoveAll(td) }() // TemporaryDirectory 语义：自带回收
	root := cgcdPyJoin(td, "repo")
	_ = os.MkdirAll(cgcdPyJoin(root, "core", "cmd", "zerg"), 0o755)
	onDisk := cgcdPyJoin(root, "core", "cmd", "zerg", "landed.go")
	_ = os.WriteFile(onDisk, nil, 0o644)
	ledger := cgcdPyJoin(td, "gaps.jsonl")
	lines := []string{
		`{"id": "GAP-T-1", "state": "仍缺", "unit": "core/cmd/zerg/landed.go"}`,
		`{"id": "GAP-T-2", "state": "仍缺", "unit": "core/cmd/zerg/absent.go"}`,
		`{"id": "GAP-T-3", "state": "已解", "unit": "core/cmd/zerg/landed.go"}`,
		`{"id": "GAP-T-4", "state": "仍缺", "unit": "无件(命令面)"}`,
	}
	_ = os.WriteFile(ledger, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	rows, _ := cgcdLoadRows(ledger)
	h := cgcdDriftRows(rows, root, map[string]bool{})
	gotIDs := []string{}
	for _, x := range h {
		gotIDs = append(gotIDs, x.id)
	}
	if len(gotIDs) != 1 || gotIDs[0] != "GAP-T-1" {
		fmt.Fprintf(stdout, "  ✗ 正控失败：期望命中 [GAP-T-1]，实得 %s\n", cgcdPyReprHits(h))
		ok = false
	}
	if len(cgcdDriftRows(rows, root, map[string]bool{"GAP-T-1": true})) != 0 {
		fmt.Fprintln(stdout, "  ✗ 负控A失败：--allow 未生效")
		ok = false
	}
	if cgcdRun(stdout, stderr, cgcdPyJoin(td, "nope.jsonl"), root, map[string]bool{}, false) != 2 {
		fmt.Fprintln(stdout, "  ✗ 负控B失败：缺语料未回 rc=2")
		ok = false
	}
	if cgcdRun(stdout, stderr, ledger, root, map[string]bool{}, false) != 1 {
		fmt.Fprintln(stdout, "  ✗ 正控 rc 失败：应回 1")
		ok = false
	}
	if ok {
		fmt.Fprintln(stdout, "自检 PASS（4 格）")
		return 0
	}
	fmt.Fprintln(stdout, "自检 FAIL（4 格）")
	return 1
}

// cgcdPyReprHits —— py `%r` 的近似形态（仅用于自检失败档的 diag 文本；正常档不产出）。
func cgcdPyReprHits(hits []cgcdHit) string {
	parts := make([]string, 0, len(hits))
	for _, h := range hits {
		parts = append(parts, "("+cgcdPyReprStr(h.id)+", "+cgcdPyReprStr(h.unit)+")")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cgcdPyReprStr —— py `repr(str)` 的近似（引号 + 常见转义）。
func cgcdPyReprStr(s string) string {
	var b strings.Builder
	quote := '\''
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	b.WriteRune(quote)
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
		case quote:
			b.WriteString("\\" + string(quote))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteRune(quote)
	return b.String()
}

// cgcdUsagePrefix —— argparse usage 的前缀（续行缩进 = 其长度）。
func cgcdUsagePrefix() string {
	return "usage: " + cgcdProg + " "
}

// cgcdUsage —— argparse usage 行（按非 tty 缺省宽 78 复刻折点，见未覆盖面②）。
func cgcdUsage() string {
	prefix := cgcdUsagePrefix()
	line1 := prefix + "[-h] [--ledger LEDGER] [--root ROOT]"
	line2 := strings.Repeat(" ", len(prefix)) + "[--allow ALLOW] [--json] [--self-test]"
	return line1 + "\n" + line2
}

// cgcdArgErr —— argparse 错误档：usage 折行 + `prog: error: msg`（两行均进 stderr），rc=2。
func cgcdArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "%s\n%s: error: %s\n", cgcdUsage(), cgcdProg, msg)
	return 2
}

// cgcdHelp —— argparse `-h/--help` 正文（照 py3.9 措辞复刻）。
func cgcdHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, cgcdUsage())
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, cgcdDesc)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "optional arguments:")
	fmt.Fprintln(stdout, "  -h, --help       show this help message and exit")
	fmt.Fprintln(stdout, "  --ledger LEDGER")
	fmt.Fprintln(stdout, "  --root ROOT")
	fmt.Fprintln(stdout, "  --allow ALLOW")
	fmt.Fprintln(stdout, "  --json")
	fmt.Fprintln(stdout, "  --self-test")
}

// cgcdLongOpts —— 本件全部长旗标（供 argparse 唯一前缀缩写解析）。
var cgcdLongOpts = []string{"--help", "--ledger", "--root", "--allow", "--json", "--self-test"}

// cgcdResolveLong —— argparse 规则：精确名优先，否则取**唯一前缀**；多匹 = 歧义。
func cgcdResolveLong(name string) (string, bool, bool) {
	for _, o := range cgcdLongOpts {
		if o == name {
			return o, false, true
		}
	}
	matches := []string{}
	for _, o := range cgcdLongOpts {
		if strings.HasPrefix(o, name) {
			matches = append(matches, o)
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

// cgcdNegNumRe —— argparse 判「-1 / -1.5 是负数而非旗标」。
var cgcdNegNumRe = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`).MatchString

// cgcdLooksOption —— argparse 判定「下一 token 是否像旗标」（`-` 单字与负数不算）。
func cgcdLooksOption(s string) bool {
	if !strings.HasPrefix(s, "-") || s == "-" {
		return false
	}
	if cgcdNegNumRe(s) {
		return false
	}
	return true
}

// checkGapClosureDriftGateMain —— 入口（= py `main()`，argparse 面逐字复刻）。
func checkGapClosureDriftGateMain(stdout, stderr io.Writer, argv []string) int {
	ledger := ""
	ledgerSet := false
	root := ""
	rootSet := false
	allow := []string{}
	jsonOut := false
	selfTest := false
	unrecognized := []string{}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h":
			cgcdHelp(stdout)
			return 0
		case a == "--":
			// argparse：本程序无 positionals ⇒ `--` 自身也进 extras（实测 py 逐字如此）
			unrecognized = append(unrecognized, a)
			unrecognized = append(unrecognized, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := a, "", false
			if j := strings.Index(a, "="); j >= 0 {
				name, val, hasVal = a[:j], a[j+1:], true
			}
			canon, amb, ok := cgcdResolveLong(name)
			if amb {
				return cgcdArgErr(stderr, "ambiguous option: "+name)
			}
			if !ok {
				unrecognized = append(unrecognized, a)
				continue
			}
			switch canon {
			case "--help":
				if hasVal {
					return cgcdArgErr(stderr, "argument --help: ignored explicit argument '"+val+"'")
				}
				cgcdHelp(stdout)
				return 0
			case "--ledger":
				if hasVal {
					ledger, ledgerSet = val, true
				} else {
					if i+1 >= len(argv) || cgcdLooksOption(argv[i+1]) {
						return cgcdArgErr(stderr, "argument --ledger: expected one argument")
					}
					i++
					ledger, ledgerSet = argv[i], true
				}
			case "--root":
				if hasVal {
					root, rootSet = val, true
				} else {
					if i+1 >= len(argv) || cgcdLooksOption(argv[i+1]) {
						return cgcdArgErr(stderr, "argument --root: expected one argument")
					}
					i++
					root, rootSet = argv[i], true
				}
			case "--allow":
				if hasVal {
					allow = append(allow, val)
				} else {
					if i+1 >= len(argv) || cgcdLooksOption(argv[i+1]) {
						return cgcdArgErr(stderr, "argument --allow: expected one argument")
					}
					i++
					allow = append(allow, argv[i])
				}
			case "--json":
				if hasVal {
					return cgcdArgErr(stderr, "argument --json: ignored explicit argument '"+val+"'")
				}
				jsonOut = true
			case "--self-test":
				if hasVal {
					return cgcdArgErr(stderr, "argument --self-test: ignored explicit argument '"+val+"'")
				}
				selfTest = true
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			unrecognized = append(unrecognized, a)
		default:
			unrecognized = append(unrecognized, a)
		}
	}
	if len(unrecognized) > 0 {
		return cgcdArgErr(stderr, "unrecognized arguments: "+strings.Join(unrecognized, " "))
	}
	if selfTest {
		return cgcdSelfTest(stdout, stderr)
	}
	if !ledgerSet {
		ledger = cgcdExpandUser("~" + cgcdDefaultLedgerRel)
	}
	if !rootSet {
		if v, ok := os.LookupEnv("ZERG_ROOT"); ok {
			root = v
		} else if wd, err := os.Getwd(); err == nil {
			root = wd
		} else {
			root = "."
		}
	}
	allowSet := map[string]bool{}
	for _, a := range allow {
		allowSet[a] = true
	}
	return cgcdRun(stdout, stderr, ledger, root, allowSet, jsonOut)
}
