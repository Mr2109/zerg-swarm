// inplace_window.go —— 公共件 `scripts/docs/inplace-window.py`（392 行）的 **Go 真搬**
// （A 档批 2 · 文档面 · 公共件搬运 · 入口 `ipwMain(stdout, stderr io.Writer, argv []string) int`）。
//
// ★ inproc 入口（本批新增 · 缺口 GAP-20260927-112 · 为「可接」补）：`ipwGateMain(stdout, stderr io.Writer, argv []string) int`
// —— 名带 `GateMain`、签名逐字同 `gate_inproc_table.go` 的 `gateCheckGoEntry` 元素体例（接线批可挂
// `gateCheckGoEntry["inplace-window"]`；★ 本件仍**不接线** —— 不碰 `gate_inproc_table.go`，归下一批）。
// argv 契约：**不含程序名**（首 token 即真参数，✗ 不照 py `sys.argv` 口径吃 `argv[0]`）；
// 本件无 `os.Executable()` 自起重入 ⇒ 无「自检重入须带子命令前缀」那一坑。入口仅转发 `ipwMain`（单实现）。
//
// 真源：`<主仓>/scripts/docs/inplace-window.py`（现读 392 行 · 缺口 `GAP-20260927-112`）。
// 本件形态：**「原地改写窗口」公共件** —— 形状① 库（arm/mutate/verify_outside/restore ＋ 上下文窗口）·
// 形状② CLI（`arm` / `status` / `restore` / `sweep` 四原语子命令）。两侧共用同一份复原口径。
//
// 逐字节对齐要点（照 py 逐条）：
//
//	① py 的 `argparse` 帮助/usage/错误正文按现读原文**逐字嵌入**（本机门跑侧 `/usr/bin/python3`
//	   现读 **3.9.6** ⇒ 段名印 `optional arguments:`、choices 印 `('arm', 'status', 'restore', 'sweep')`）。
//	   `prog` = `basename(sys.argv[0])` = `inplace-window.py`（py 以脚本文件直跑时的现读值）；
//	   ≤3.9/≥3.10 差异属环境依赖 ⇒ 已列入未覆盖面。
//	② 路径一律 **posixpath 同构**（自写 `ipwPyJoin`/`ipwPyNormpath`/`ipwPySplit`/`ipwPyAbspath`/
//	   `ipwPyDirname`/`ipwPyBasename`，✗ 不用 `filepath.Join`/`filepath.Clean`/`filepath.Dir`）。
//	③ 台账 JSON 照 py `json.dumps(..., ensure_ascii=False)` 的**默认分隔符 `", "` / `": "`** ＋
//	   **键按 dict 插入序** ⇒ 自写有序编码（✗ 不用 `encoding/json` 的 map 形态/转义 HTML）。
//	   float 照 py `repr()`：整值补 `.0`（`1.0` 不许写成 `1`）。
//	④ 空集合 `[]` ≠ `null`：py 恒产 `[]`（json.dumps 空列表）⇒ 本件显式置非 nil 空切片。
//	⑤ 非确定面：`ts`（`time.time()`）· `pid`（`os.getpid()`）· 未给 `--run-id` 时的 `uuid4().hex[:16]`
//	   —— 三者照 py 现读形状落盘/打印；对拍时按声明**归一该 token**（见回执与测试件）。
//	⑥ 文件模式：前像 tmp → `fsync` → `os.replace`；复原用 `mkstemp(dir=d, prefix=".inplace-restore-")`
//	   ⇒ 复原后 target 模式 = `0600`（mkstemp 默认），Go 侧 `os.CreateTemp` 同为 `0600`。
//
// 两条标准声明行：
//
//	(a) **默认根定位差异**：py 的 root 只来自 `$ZERG_STATE_DIR`（缺则 `~/.zerg/state`，**不**用 `__file__`）
//	    ⇒ 本件与 py 的默认根**同源同构**（都读同一 env / 同一 HOME 回落），✗ 无 `__file__` 口径差。
//	    唯一差异：本件由 cwd 取 `abspath` 的相对 target 基底（`os.getcwd()`），与 py `os.getcwd()` 逐字同。
//	(b) **自指检查**：本件扫描面 = `$ZERG_STATE_DIR/inplace-window` 的台账与调用方点名的 target；
//	    本件（落 `core/cmd/zerg/`）**不在自身的扫描/判据面内**。量化：全件零绝对私有路径字面量
//	    （零 `/Volumes`、零 `/Users`）、退码仅 0/1/2/3（⊆ 真源退码表已登记集）、零「JSON 键名连续引号原串」。
//
// 本单边界（照实）：**只落码、不接线** —— 不注册命令、不删 py 原件、无调用点（M2 刻意状态）。
// 形状①（库面）的**信号面**（INT/TERM/HUP/QUIT ⇒ 复原本次 run 再退出；SIGKILL 明写兜不住）
// 未设对拍格（真信号需跨进程、不可逐字对拍）⇒ 见回执未覆盖面。
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const ipwProg = "inplace-window.py"
const ipwLedgerName = "ledger.jsonl"

// ── argparse 面（现读原文逐字嵌入）────────────────────────────────────────────

const ipwTopUsage = "usage: inplace-window.py [-h] [--json] [--self-test]\n" +
	"                         {arm,status,restore,sweep} ..."

const ipwArmUsage = "usage: inplace-window.py arm [-h] [--run-id RUN_ID] target"
const ipwStatusUsage = "usage: inplace-window.py status [-h]"
const ipwRestoreUsage = "usage: inplace-window.py restore [-h] run_id"
const ipwSweepUsage = "usage: inplace-window.py sweep [-h] [--heal]"

const ipwTopHelp = ipwTopUsage + "\n\n" +
	"原地改写窗口公共件（GAP-20260927-112）\n\n" +
	"positional arguments:\n" +
	"  {arm,status,restore,sweep}\n\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --json\n" +
	"  --self-test\n"

const ipwArmHelp = ipwArmUsage + "\n\n" +
	"positional arguments:\n" +
	"  target\n\n" +
	"optional arguments:\n" +
	"  -h, --help       show this help message and exit\n" +
	"  --run-id RUN_ID\n"

const ipwStatusHelp = ipwStatusUsage + "\n\n" +
	"optional arguments:\n" +
	"  -h, --help  show this help message and exit\n"

const ipwRestoreHelp = ipwRestoreUsage + "\n\n" +
	"positional arguments:\n" +
	"  run_id\n\n" +
	"optional arguments:\n" +
	"  -h, --help  show this help message and exit\n"

const ipwSweepHelp = ipwSweepUsage + "\n\n" +
	"optional arguments:\n" +
	"  -h, --help  show this help message and exit\n" +
	"  --heal\n"

func ipwSubHelp(cmd string) string {
	switch cmd {
	case "arm":
		return ipwArmHelp
	case "status":
		return ipwStatusHelp
	case "restore":
		return ipwRestoreHelp
	case "sweep":
		return ipwSweepHelp
	}
	return ipwTopHelp
}

// ipwUsageErr —— argparse `error()` 正文：`<usage>\n<prog>: error: <msg>\n`（rc=2）。
type ipwUsageErr struct {
	usage, prog, msg string
}

func (e *ipwUsageErr) Error() string {
	return e.usage + "\n" + e.prog + ": error: " + e.msg + "\n"
}

// ipwError —— py `RuntimeError` 的对应物（带退码；主线 catch 后印 `inplace-window：<msg>`）。
type ipwError struct {
	msg string
	rc  int
}

func (e *ipwError) Error() string { return e.msg }

// ── py 字面原语 ────────────────────────────────────────────────────────────────

// ipwPyRepr —— py `%r` / `repr(str)` 的最简同构（单引号包裹；仅转义 `\` 与 `'`）。
func ipwPyRepr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "'", `\'`)
	return "'" + s + "'"
}

// ipwQuote —— py `json.dumps(str, ensure_ascii=False)` 的字符串面（✗ 不转义 `<>&`；控制符走 \u00xx）。
func ipwQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ipwFloatRepr —— py `repr(float)`：整值补 `.0`（`1.0` ⇒ `"1.0"`，✗ `"1"`）。
func ipwFloatRepr(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// ipwKV —— 有序 JSON 对象的一格（键按插入序）。
type ipwKV struct {
	K string
	V interface{}
}

// ipwObj —— 有序 JSON 对象。
type ipwObj struct{ F []ipwKV }

// ipwArr —— JSON 数组（**空集显式 `[]`**，✗ 不落 `null`）。
type ipwArr []interface{}

func ipwEnc(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return ipwQuote(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return ipwFloatRepr(t)
	case ipwArr:
		var b strings.Builder
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(ipwEnc(e))
		}
		b.WriteByte(']')
		return b.String()
	case *ipwObj:
		var b strings.Builder
		b.WriteByte('{')
		for i, kv := range t.F {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(ipwQuote(kv.K))
			b.WriteString(": ")
			b.WriteString(ipwEnc(kv.V))
		}
		b.WriteByte('}')
		return b.String()
	}
	return "null"
}

// ── posixpath 同构（✗ filepath.*）─────────────────────────────────────────────

// ipwPyJoin —— CPython `posixpath.join(a, b)`（后段绝对 ⇒ 丢前段；空段语义照原样）。
func ipwPyJoin(a, b string) string {
	if strings.HasPrefix(b, "/") {
		return b
	}
	if a == "" || strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// ipwPyNormpath —— CPython `posixpath.normpath`（★ 保留**恰两个**前导斜杠）。
func ipwPyNormpath(p string) string {
	if p == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(p, "/") {
		initial = 1
		if strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///") {
			initial = 2
		}
	}
	var out []string
	for _, c := range strings.Split(p, "/") {
		if c == "" || c == "." {
			continue
		}
		if c != ".." || (initial == 0 && len(out) == 0) || (len(out) > 0 && out[len(out)-1] == "..") {
			out = append(out, c)
		} else if len(out) > 0 {
			out = out[:len(out)-1]
		}
	}
	res := strings.Join(out, "/")
	if initial > 0 {
		res = strings.Repeat("/", initial) + res
	}
	if res == "" {
		return "."
	}
	return res
}

// ipwPySplit —— CPython `posixpath.split(p)`（剥尾斜杠）。
func ipwPySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

func ipwPyDirname(p string) string {
	h, _ := ipwPySplit(p)
	return h
}

func ipwPyBasename(p string) string {
	_, t := ipwPySplit(p)
	return t
}

// ipwPyAbspath —— CPython `os.path.abspath` = `normpath(join(getcwd(), path))`。
func ipwPyAbspath(p string) string {
	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}
	return ipwPyNormpath(ipwPyJoin(wd, p))
}

// ipwPyStrip —— py `str.strip()` 的近似同构（补上 unicode.IsSpace 不剥的 \x1c-\x1f）。
func ipwPyStrip(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		if r < 0x80 {
			switch r {
			case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f:
				return true
			}
			return false
		}
		switch r {
		case 0x85, 0xa0, 0x1680, 0x2007, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
			return true
		}
		return r >= 0x2000 && r <= 0x200a
	})
}

// ── 状态目录 / 台账 ───────────────────────────────────────────────────────────

// ipwStateDir —— `$ZERG_STATE_DIR`（缺/空 ⇒ `~/.zerg/state`）＋ `/inplace-window`；解析不来 ⇒ rc=2。
func ipwStateDir() (string, *ipwError) {
	d := os.Getenv("ZERG_STATE_DIR")
	if d == "" {
		home, _ := os.UserHomeDir()
		d = ipwPyJoin(ipwPyJoin(home, ".zerg"), "state")
	}
	if d == "" || d == "/" {
		return "", &ipwError{"状态目录解析不来（ZERG_STATE_DIR 为空/根）⇒ 拒跑（rc=2）", 2}
	}
	root := ipwPyJoin(d, "inplace-window")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", &ipwError{fmt.Sprintf("状态目录建不起：%s（%s）", root, err.Error()), 1}
	}
	return root, nil
}

func ipwLedgerPath(root string) string { return ipwPyJoin(root, ipwLedgerName) }

func ipwIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func ipwSha(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// ipwPidAlive —— py `_pid_alive`：ESRCH ⇒ false；EPERM 及其余 ⇒ true。
func ipwPidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil || err == syscall.EPERM {
		return true
	}
	if err == syscall.ESRCH {
		return false
	}
	return true
}

func ipwNow() float64 { return float64(time.Now().UnixNano()) / 1e9 }

// ipwReadLedger —— 逐行读台账（universal newlines ＋ `.strip()`；坏行 = 不吞 ⇒ 上抛）。
func ipwReadLedger(root string) ([]map[string]interface{}, error) {
	p := ipwLedgerPath(root)
	if !ipwIsFile(p) {
		return []map[string]interface{}{}, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	out := []map[string]interface{}{}
	for _, ln := range strings.Split(text, "\n") {
		ln = ipwPyStrip(ln)
		if ln == "" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func ipwAppend(root string, fields []ipwKV) error {
	f, err := os.OpenFile(ipwLedgerPath(root), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.WriteString(f, ipwEnc(&ipwObj{F: fields})+"\n"); err != nil {
		return err
	}
	return f.Sync()
}

func ipwStr(m map[string]interface{}, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func ipwInt(m map[string]interface{}, k string) int {
	if v, ok := m[k]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return 0
}

// ipwIntOrNil —— `e.get(k)` 原始值；数字取整值（`999999` 不许印成 `999999.0`），缺/非数 ⇒ 原样/nil。
func ipwIntOrNil(m map[string]interface{}, k string) interface{} {
	v, ok := m[k]
	if !ok {
		return nil
	}
	if f, ok := v.(float64); ok {
		if f == math.Trunc(f) {
			return int64(f)
		}
		return f
	}
	return v
}

func ipwHasRestoreEvent(led []map[string]interface{}, runID string) bool {
	for _, x := range led {
		if ipwStr(x, "event") == "restore" && ipwStr(x, "run_id") == runID {
			return true
		}
	}
	return false
}

// ipwActiveForTarget —— 回占用该 target 的存活窗口（同 target 串行判据）。
func ipwActiveForTarget(root, targetAbs, excludeRun string) ([]map[string]interface{}, error) {
	led, err := ipwReadLedger(root)
	if err != nil {
		return nil, err
	}
	live := []map[string]interface{}{}
	for _, e := range led {
		if ipwStr(e, "event") != "arm" {
			continue
		}
		if ipwStr(e, "target") != targetAbs {
			continue
		}
		rid := ipwStr(e, "run_id")
		if rid == excludeRun {
			continue
		}
		if ipwHasRestoreEvent(led, rid) {
			continue
		}
		if ipwPidAlive(ipwInt(e, "pid")) {
			live = append(live, e)
		}
	}
	return live, nil
}

// ipwRandomRunID —— py `uuid.uuid4().hex[:16]`（前 8 字节；byte6 高半字节 = 版本 4）。
func ipwRandomRunID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	return hex.EncodeToString(b[:])
}

// ipwArm —— 落前像 ＋ 登记。回 run_id。同 target 有存活窗口 ⇒ rc=3；target 缺 ⇒ rc=2。
func ipwArm(root, target, runID string, haveRunID bool) (string, *ipwError) {
	targetAbs := ipwPyAbspath(target)
	rid := runID
	if !haveRunID {
		rid = ipwRandomRunID()
	}
	live, err := ipwActiveForTarget(root, targetAbs, rid)
	if err != nil {
		return "", &ipwError{err.Error(), 1}
	}
	if len(live) > 0 {
		return "", &ipwError{fmt.Sprintf("同 target 已有存活改写窗口 ⇒ 并发第二跑被拒（rc=3）：%s", targetAbs), 3}
	}
	if !ipwIsFile(targetAbs) {
		return "", &ipwError{fmt.Sprintf("target 不在盘，无可前像：%s ⇒ rc=2", targetAbs), 2}
	}
	preDir := ipwPyJoin(root, "pre")
	if err := os.MkdirAll(preDir, 0o755); err != nil {
		return "", &ipwError{err.Error(), 1}
	}
	preimage := ipwPyJoin(preDir, fmt.Sprintf("%s.%s.pre", rid, ipwPyBasename(targetAbs)))
	data, err := os.ReadFile(targetAbs)
	if err != nil {
		return "", &ipwError{err.Error(), 1}
	}
	tmp := preimage + ".tmp"
	if err := ipwWriteSync(tmp, data); err != nil {
		return "", &ipwError{err.Error(), 1}
	}
	if err := os.Rename(tmp, preimage); err != nil {
		return "", &ipwError{err.Error(), 1}
	}
	sum := sha256.Sum256(data)
	rec := []ipwKV{
		{"event", "arm"},
		{"run_id", rid},
		{"target", targetAbs},
		{"preimage", preimage},
		{"target_sha", hex.EncodeToString(sum[:])},
		{"pid", int64(os.Getpid())},
		{"ts", ipwNow()},
	}
	if err := ipwAppend(root, rec); err != nil {
		return "", &ipwError{err.Error(), 1}
	}
	return rid, nil
}

func ipwWriteSync(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ipwRestore —— 按台账复原该 run 的全部窗口。前像缺/sha 不符 ⇒ rc=2（不给结论，不静默覆盖）。
func ipwRestore(runID, root string) (int, *ipwError) {
	led, err := ipwReadLedger(root)
	if err != nil {
		return 0, &ipwError{err.Error(), 1}
	}
	arms := []map[string]interface{}{}
	for _, e := range led {
		if ipwStr(e, "event") == "arm" && ipwStr(e, "run_id") == runID {
			arms = append(arms, e)
		}
	}
	if len(arms) == 0 {
		return 0, &ipwError{fmt.Sprintf("台账里没有该 run 的窗口：%s ⇒ rc=2", runID), 2}
	}
	restored := 0
	for _, e := range arms {
		pre := ipwStr(e, "preimage")
		if !ipwIsFile(pre) {
			return 0, &ipwError{fmt.Sprintf("前像缺件（SIGKILL 双失风险）：%s ⇒ rc=2", pre), 2}
		}
		sha, err := ipwSha(pre)
		if err != nil {
			return 0, &ipwError{err.Error(), 1}
		}
		if sha != ipwStr(e, "target_sha") {
			return 0, &ipwError{fmt.Sprintf("前像 sha 与登记不符：%s ⇒ rc=2（不给结论，不静默覆盖）", pre), 2}
		}
		data, err := os.ReadFile(pre)
		if err != nil {
			return 0, &ipwError{err.Error(), 1}
		}
		tgt := ipwStr(e, "target")
		d := ipwPyDirname(tgt)
		if d == "" {
			d = "."
		}
		tmpf, err := os.CreateTemp(d, ".inplace-restore-")
		if err != nil {
			return 0, &ipwError{err.Error(), 1}
		}
		tmp := tmpf.Name()
		if _, err := tmpf.Write(data); err != nil {
			tmpf.Close()
			return 0, &ipwError{err.Error(), 1}
		}
		if err := tmpf.Sync(); err != nil {
			tmpf.Close()
			return 0, &ipwError{err.Error(), 1}
		}
		if err := tmpf.Close(); err != nil {
			return 0, &ipwError{err.Error(), 1}
		}
		if err := os.Rename(tmp, tgt); err != nil {
			return 0, &ipwError{err.Error(), 1}
		}
		if dfd, derr := os.Open(d); derr == nil {
			_ = dfd.Sync()
			_ = dfd.Close()
		}
		if err := ipwAppend(root, []ipwKV{
			{"event", "restore"}, {"run_id", runID}, {"target", tgt}, {"ts", ipwNow()},
		}); err != nil {
			return 0, &ipwError{err.Error(), 1}
		}
		restored++
	}
	return restored, nil
}

// ── 形状①：库面（arm ＋ 信号复原 ＋ 窗口写入/核字节）───────────────────────────
//
// ★ 未设对拍格（信号面需跨进程真发信号、且 SIGKILL 明写兜不住）⇒ 见回执未覆盖面。

// ipwInstallHandlers —— 装 INT/TERM/HUP/QUIT handler：触发时 killpg 同组 ＋ 复原 run 再按默认语义退出。
func ipwInstallHandlers(runID string) chan os.Signal {
	sigs := []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sigs...)
	go func() {
		sig := <-ch
		signum, _ := sig.(syscall.Signal)
		pgid := syscall.Getpgrp()
		_ = syscall.Kill(-pgid, signum)
		if root, e := ipwStateDir(); e == nil {
			if _, err := ipwRestore(runID, root); err != nil {
				fmt.Fprintf(os.Stderr, "inplace-window：信号 %d 复原失败：%s（靠 status/sweep 人工兜）\n", signum, err.Error())
			} else {
				fmt.Fprintf(os.Stderr, "inplace-window：收到信号 %d ⇒ 已复原 run %s 的全部窗口\n", signum, runID)
			}
		}
		signal.Reset(sig)
		_ = syscall.Kill(os.Getpid(), signum)
	}()
	return ch
}

func ipwUninstallHandlers(ch chan os.Signal) {
	if ch != nil {
		signal.Stop(ch)
	}
}

// ipwWindow —— 形状① 上下文窗口：arm ＋ 装 handler；调用方 defer 复原；返回复原闭包。
func ipwWindow(target, runID string) (string, func(), *ipwError) {
	root, e := ipwStateDir()
	if e != nil {
		return "", nil, e
	}
	rid, e := ipwArm(root, target, runID, runID != "")
	if e != nil {
		return "", nil, e
	}
	ch := ipwInstallHandlers(rid)
	done := func() {
		_, _ = ipwRestore(rid, root)
		ipwUninstallHandlers(ch)
	}
	return rid, done, nil
}

// ipwWindowWriteBytes —— 窗口内真改（flush ＋ fsync）。
func ipwWindowWriteBytes(target string, data []byte) error {
	f, err := os.OpenFile(target, os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ipwWindowVerifyOutside —— 窗口外消费方看到的字节 = 现盘内容；给 expect 则比 sha。
func ipwWindowVerifyOutside(target, expect string) (string, bool) {
	s, err := ipwSha(target)
	if err != nil {
		return "", false
	}
	if expect == "" {
		return s, true
	}
	return s, s == expect
}

// ── 形状②：CLI 派发 ───────────────────────────────────────────────────────────

func ipwStatus(stdout io.Writer, root string, asJSON bool) int {
	led, err := ipwReadLedger(root)
	if err != nil {
		fmt.Fprintf(stdout, "inplace-window：%s\n", err.Error())
		return 1
	}
	rows := []interface{}{}
	for _, e := range led {
		if ipwStr(e, "event") != "arm" {
			continue
		}
		restored := ipwHasRestoreEvent(led, ipwStr(e, "run_id"))
		alive := !restored && ipwPidAlive(ipwInt(e, "pid"))
		rows = append(rows, &ipwObj{F: []ipwKV{
			{"run_id", ipwStr(e, "run_id")},
			{"target", ipwStr(e, "target")},
			{"pid", ipwIntOrNil(e, "pid")},
			{"alive", alive},
			{"restored", restored},
		}})
	}
	if asJSON {
		fmt.Fprintln(stdout, ipwEnc(&ipwObj{F: []ipwKV{{"windows", ipwArr(rows)}}}))
		return 0
	}
	fmt.Fprintf(stdout, "inplace-window 台账：%d 条窗口\n", len(rows))
	for _, e := range led {
		if ipwStr(e, "event") != "arm" {
			continue
		}
		restored := ipwHasRestoreEvent(led, ipwStr(e, "run_id"))
		alive := !restored && ipwPidAlive(ipwInt(e, "pid"))
		flag := "★疑似残留（pid 已死）"
		if alive {
			flag = "存活"
		} else if restored {
			flag = "已复原"
		}
		fmt.Fprintf(stdout, "  %s target=%s pid=%s ⇒ %s\n",
			ipwStr(e, "run_id"), ipwStr(e, "target"), ipwEncValue(ipwIntOrNil(e, "pid")), flag)
	}
	return 0
}

// ipwEncValue —— `%s` 语义：int64 ⇒ 十进制、nil ⇒ `None`、float ⇒ py repr。
func ipwEncValue(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return ipwFloatRepr(t)
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	}
	return fmt.Sprintf("%v", v)
}

func ipwSweep(stdout io.Writer, root string, heal, asJSON bool) int {
	led, err := ipwReadLedger(root)
	if err != nil {
		fmt.Fprintf(stdout, "inplace-window：%s\n", err.Error())
		return 1
	}
	stale := []map[string]interface{}{}
	for _, e := range led {
		if ipwStr(e, "event") != "arm" {
			continue
		}
		restored := ipwHasRestoreEvent(led, ipwStr(e, "run_id"))
		if !restored && !ipwPidAlive(ipwInt(e, "pid")) {
			stale = append(stale, e)
		}
	}
	type doneRec struct{ rid, res string }
	done := []doneRec{}
	if heal {
		for _, e := range stale {
			rid := ipwStr(e, "run_id")
			if n, err := ipwRestore(rid, root); err != nil {
				done = append(done, doneRec{rid, "失败：" + err.Error()})
			} else {
				done = append(done, doneRec{rid, strconv.Itoa(n)})
			}
		}
	}
	if asJSON {
		staleIDs := ipwArr{}
		for _, s := range stale {
			staleIDs = append(staleIDs, ipwStr(s, "run_id"))
		}
		healedIDs := ipwArr{}
		for _, d := range done {
			healedIDs = append(healedIDs, d.rid)
		}
		fmt.Fprintln(stdout, ipwEnc(&ipwObj{F: []ipwKV{{"stale", staleIDs}, {"healed", healedIDs}}}))
		return 0
	}
	fmt.Fprintf(stdout, "inplace-window sweep：疑似残留（持有者已死、未复原）= %d 条\n", len(stale))
	for _, e := range stale {
		fmt.Fprintf(stdout, "  ★ %s target=%s（前像 %s）\n", ipwStr(e, "run_id"), ipwStr(e, "target"), ipwStr(e, "preimage"))
	}
	if heal {
		for _, d := range done {
			fmt.Fprintf(stdout, "  ✓ 已复原 %s ⇒ %s\n", d.rid, d.res)
		}
	} else if len(stale) > 0 {
		fmt.Fprint(stdout, "  加 --heal 复原；SIGKILL 场景只能靠本列表 + 人工。\n")
	}
	return 0
}

// ipwSelfTest —— py `_self_test`（自建自回收：mkdtemp ⇒ finally rmtree）。
func ipwSelfTest(stdout io.Writer) int {
	ok := true
	td, err := os.MkdirTemp("", "")
	if err != nil {
		fmt.Fprint(stdout, "自检 FAIL\n")
		return 1
	}
	defer os.RemoveAll(td)
	old := os.Getenv("ZERG_STATE_DIR")
	_ = os.Setenv("ZERG_STATE_DIR", ipwPyJoin(td, "state"))
	defer func() { _ = os.Setenv("ZERG_STATE_DIR", old) }()

	tgt := ipwPyJoin(td, "t.txt")
	if err := os.WriteFile(tgt, []byte("ORIG"), 0o644); err != nil {
		fmt.Fprint(stdout, "自检 FAIL\n")
		return 1
	}
	root, e := ipwStateDir()
	if e != nil {
		fmt.Fprint(stdout, "自检 FAIL\n")
		return 1
	}
	rid, e := ipwArm(root, tgt, "", false)
	if e != nil {
		fmt.Fprint(stdout, "自检 FAIL\n")
		return 1
	}
	if err := ipwWindowWriteBytes(tgt, []byte("MUT")); err != nil {
		fmt.Fprint(stdout, "自检 FAIL\n")
		return 1
	}
	if b, _ := os.ReadFile(tgt); string(b) != "MUT" {
		fmt.Fprint(stdout, "自检 FAIL\n")
		return 1
	}
	if _, e := ipwRestore(rid, root); e != nil {
		fmt.Fprint(stdout, "自检 FAIL\n")
		return 1
	}
	if b, _ := os.ReadFile(tgt); string(b) != "ORIG" {
		fmt.Fprint(stdout, "  ✗ 正控失败：窗口退出未复原\n")
		ok = false
	}
	led, rerr := ipwReadLedger(root)
	armN, restoreN := 0, 0
	if rerr == nil {
		for _, x := range led {
			switch ipwStr(x, "event") {
			case "arm":
				armN++
			case "restore":
				restoreN++
			}
		}
	}
	if armN < 1 || restoreN < 1 {
		fmt.Fprintf(stdout, "  ✗ 台账缺 arm/restore：arm=%d restore=%d\n", armN, restoreN)
		ok = false
	}
	if _, e := ipwRestore("no-such-run", root); e == nil {
		fmt.Fprint(stdout, "  ✗ 负控失败：未知 run 未抛\n")
		ok = false
	}
	if ok {
		fmt.Fprint(stdout, "自检 PASS\n")
		return 0
	}
	fmt.Fprint(stdout, "自检 FAIL\n")
	return 1
}

// ipwMain —— 入口（argv **不含程序名**，照同包 inproc 口径）。
func ipwMain(stdout, stderr io.Writer, argv []string) int {
	args, helpText, perr := ipwParseArgs(argv)
	if helpText != "" {
		io.WriteString(stdout, helpText)
		return 0
	}
	if perr != nil {
		io.WriteString(stderr, perr.Error())
		return 2
	}
	if args.selfTest {
		return ipwSelfTest(stdout)
	}
	root, serr := ipwStateDir()
	if serr != nil {
		fmt.Fprintf(stderr, "inplace-window：%s\n", serr.Error())
		return serr.rc
	}
	switch args.cmd {
	case "arm":
		rid, e := ipwArm(root, args.target, args.runID, args.haveRunID)
		if e != nil {
			fmt.Fprintf(stderr, "inplace-window：%s\n", e.Error())
			return e.rc
		}
		if args.json {
			fmt.Fprintln(stdout, rid)
		} else {
			fmt.Fprintf(stdout, "已 arm run_id=%s target=%s\n", rid, args.target)
		}
		return 0
	case "restore":
		n, e := ipwRestore(args.positional, root)
		if e != nil {
			fmt.Fprintf(stderr, "inplace-window：%s\n", e.Error())
			return e.rc
		}
		if args.json {
			fmt.Fprintf(stdout, "{\"restored\": %d}\n", n)
		} else {
			fmt.Fprintf(stdout, "已复原 %d 个窗口\n", n)
		}
		return 0
	case "status":
		return ipwStatus(stdout, root, args.json)
	case "sweep":
		return ipwSweep(stdout, root, args.heal, args.json)
	}
	io.WriteString(stdout, ipwTopHelp)
	return 2
}

// ipwGateMain —— **inproc 入口**（名带 `GateMain`；签名逐字同 `gate_inproc_table.go` 的
// `gateCheckGoEntry` 元素类型 `func(stdout, stderr io.Writer, argv []string) int` ⇒ 接线批可直接
// 把 `gateCheckGoEntry["inplace-window"]` 指向本函数）。
//
// 契约面（两条已知坑的处置）：
//
//	① argv **不含程序名**（inproc 口径）：首 token 即真参数 —— 本入口整段转交 `ipwMain`，而
//	   `ipwMain` 从不消费 `argv[0]` ⇒ 契约满足；✗ 不按 py `sys.argv` 口径丢首 token。
//	② 自检重入：本件 `--self-test`（`ipwSelfTest`）**不重起自己**（无 `os.Executable()`）⇒
//	   不存在「重入 argv 须前置子命令前缀」那一坑，无需注入点。
//
// 单一实现：本入口只转发 `ipwMain`（✗ 不另写第二套解析/判据体）。
func ipwGateMain(stdout, stderr io.Writer, argv []string) int {
	return ipwMain(stdout, stderr, argv)
}

// ── 参数解析（argparse 同构：唯一前缀缩写 · `--`/`-` 视作位置参数 · 缺必填先于未识别）──

type ipwArgs struct {
	json       bool
	selfTest   bool
	cmd        string
	target     string
	haveTarget bool
	runID      string
	haveRunID  bool
	positional string
	havePos    bool
	heal       bool
}

func ipwSplitEq(tok string) (string, string, bool) {
	if i := strings.Index(tok, "="); i >= 0 {
		return tok[:i], tok[i+1:], true
	}
	return tok, "", false
}

// ipwLongPrefix —— argparse 唯一前缀匹配：`--j` 匹 `--json`（len≥3，✗ 裸 `--`）。
func ipwLongPrefix(name, full string) bool {
	if !strings.HasPrefix(name, "--") || len(name) < 3 {
		return false
	}
	return strings.HasPrefix(full, name)
}

func ipwParseArgs(argv []string) (*ipwArgs, string, error) {
	args := &ipwArgs{}
	var unrecognized []string
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if strings.HasPrefix(tok, "-") && tok != "-" && tok != "--" {
			name, val, hasVal := ipwSplitEq(tok)
			switch {
			case name == "-h" || ipwLongPrefix(name, "--help"):
				return args, ipwTopHelp, nil
			case ipwLongPrefix(name, "--json"):
				if hasVal {
					return nil, "", &ipwUsageErr{ipwTopUsage, ipwProg, "argument --json: ignored explicit argument " + ipwPyRepr(val)}
				}
				args.json = true
			case ipwLongPrefix(name, "--self-test"):
				if hasVal {
					return nil, "", &ipwUsageErr{ipwTopUsage, ipwProg, "argument --self-test: ignored explicit argument " + ipwPyRepr(val)}
				}
				args.selfTest = true
			default:
				unrecognized = append(unrecognized, tok)
			}
			continue
		}
		args.cmd = tok
		if tok != "arm" && tok != "status" && tok != "restore" && tok != "sweep" {
			return nil, "", &ipwUsageErr{ipwTopUsage, ipwProg,
				"argument cmd: invalid choice: " + ipwPyRepr(tok) + " (choose from 'arm', 'status', 'restore', 'sweep')"}
		}
		sub, htext, herr := ipwParseSub(args, argv[i+1:])
		if htext != "" {
			return args, htext, nil
		}
		if herr != nil {
			return nil, "", herr
		}
		unrecognized = append(unrecognized, sub...)
		break
	}
	if len(unrecognized) > 0 {
		return nil, "", &ipwUsageErr{ipwTopUsage, ipwProg, "unrecognized arguments: " + strings.Join(unrecognized, " ")}
	}
	return args, "", nil
}

// ipwParseSub —— 子命令解析；回（未识别串表 · 子帮助正文 · 立即错误）。
func ipwParseSub(args *ipwArgs, rest []string) ([]string, string, error) {
	var usage, prog string
	switch args.cmd {
	case "arm":
		usage, prog = ipwArmUsage, ipwProg+" arm"
	case "status":
		usage, prog = ipwStatusUsage, ipwProg+" status"
	case "restore":
		usage, prog = ipwRestoreUsage, ipwProg+" restore"
	case "sweep":
		usage, prog = ipwSweepUsage, ipwProg+" sweep"
	}
	var unrecognized []string
	for i := 0; i < len(rest); i++ {
		tok := rest[i]
		if strings.HasPrefix(tok, "-") && tok != "-" && tok != "--" {
			name, val, hasVal := ipwSplitEq(tok)
			if name == "-h" || ipwLongPrefix(name, "--help") {
				return nil, ipwSubHelp(args.cmd), nil
			}
			if args.cmd == "arm" && ipwLongPrefix(name, "--run-id") {
				if hasVal {
					args.runID = val
				} else {
					if i+1 >= len(rest) {
						return nil, "", &ipwUsageErr{usage, prog, "argument --run-id: expected one argument"}
					}
					i++
					args.runID = rest[i]
				}
				args.haveRunID = true
				continue
			}
			if args.cmd == "sweep" && ipwLongPrefix(name, "--heal") {
				if hasVal {
					return nil, "", &ipwUsageErr{usage, prog, "argument --heal: ignored explicit argument " + ipwPyRepr(val)}
				}
				args.heal = true
				continue
			}
			unrecognized = append(unrecognized, tok)
			continue
		}
		switch args.cmd {
		case "arm":
			if !args.haveTarget {
				args.target = tok
				args.haveTarget = true
			} else {
				unrecognized = append(unrecognized, tok)
			}
		case "restore":
			if !args.havePos {
				args.positional = tok
				args.havePos = true
			} else {
				unrecognized = append(unrecognized, tok)
			}
		default:
			unrecognized = append(unrecognized, tok)
		}
	}
	if args.cmd == "arm" && !args.haveTarget {
		return nil, "", &ipwUsageErr{usage, prog, "the following arguments are required: target"}
	}
	if args.cmd == "restore" && !args.havePos {
		return nil, "", &ipwUsageErr{usage, prog, "the following arguments are required: run_id"}
	}
	return unrecognized, "", nil
}
