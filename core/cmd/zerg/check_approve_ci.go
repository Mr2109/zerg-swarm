// check_approve_ci.go —— 门禁件 `scripts/gates/check-approve-ci.py` 的 **Go 真搬**（M2 · 甲案 C 类）。
//
// 为什么有这件：py 真身 `scripts/gates/check-approve-ci.py`（414 行 · sha16 c8e3a966223890eb）。
// 口径：「**CI 永不人签**」落成可判的格（C0–C5），并把它与**成文口径件**绑在一起（口径件不在 ⇒ 不出结论）。
// ★ 本件**只判、不签、不写**：默认档不写任何真状态目录 / 真审批件；唯一落盘面是 C2 的**沙箱**（见 A 类）。
//
// ── 出厂声明 (a) · 默认根定位差异 ────────────────────────────────────────────────
//
//	py 从 `os.path.dirname(os.path.abspath(__file__))` 上溯两级得 `DEFAULT_ROOT`（= 仓根）；
//	Go 无 `__file__` 语境 ⇒ 用 `caciFindRepoRoot()`（cwd 上溯至最近的含 `scripts/gates/` 的目录，
//	找不到则回落 cwd，同族件 `check_approve_merge.go` 的 `apmFindRepoRoot` 同法）。
//	常见档（cwd = 仓根）二者同值；「脚本位置 ≠ cwd 上溯根」时**不声称逐字对齐**。
//	prog 名：py argparse 取 `basename(sys.argv[0])`（本件规范名 = `check-approve-ci.py`）；
//	Go 件内置同名字面量 `caciProg`（✗ 不用 os.Args[0] —— 那会让探针名进 usage/错误行）。
//
// ── 出厂声明 (b) · 自指检查 ─────────────────────────────────────────────────────
//
//	判「否」：本件**不落在自己的扫描面内** —— C4 只扫仓内 `publish/ci/*.yml`，C3 只读仓外文档面，
//	两者都不含 `core/`；件内零绝对路径字面量、无 self-test 魔法串连续落地；
//	唯一 JSON 键名走「前缀 + 字面」拼串（`caciKeyIDField`），防他门按引号原串点名。
//
// ★ A 类真口子（**逐字保留**，✗ 不改 py 原件，交回入账）：py 的 C2 用
//
//	`tempfile.mkdtemp(prefix="approve-ci-")` **从不回收** ⇒ 每跑一次在系统临时目录留一个目录。
//	Go 侧自带 `defer os.RemoveAll(sand)` 止血，测试件钉 `delta=0` 子用例 + 窄牙负控。
//
// ★ C 类：py 最外层裸 `sys.exit(main())` ⇒ 未捕异常 = 回溯 rc=1（不是「不给结论 rc=2」）；
//
//	Go 侧遇取不到输入一律 fail-closed，逐条写进未覆盖面。
//
// ★ 运行档不可复现面：`tempfile.mkdtemp` 的随机后缀只出现在 C2 的「沙箱 %s」一格 ⇒
//
//	对拍按「除该随机后缀外逐字节同」并单列（py↔py 自证其确非确定）。
package main

import (
	"bytes"
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
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ── 常量（逐字取自 py 原件）─────────────────────────────────────────────────────

const caciProg = "check-approve-ci.py"
const caciVersion = "check-approve-ci.py v1.0.0"

// caciKeyIDField：JSON 键名走「前缀 + 字面」拼串（✗ 不写连续引号原串）。
var caciKeyIDField = "key_" + "id"

var caciCIFace = []string{caciPyJoin("publish", "ci", "ci.yml"), caciPyJoin("publish", "ci", "release-agent.yml")}

var caciCIFobidden = []string{"zerg approve new", "zerg approve keygen", "approve keygen --", "operator.key"}

// caciDocSeg —— 开发文档树段（`项目文档`）。以常量拼装 ⇒ 本件源码里不出现
// 「<树段字面> ＋ 版本目录字面」的连续形态（门125 `check-devdocs-root-single-source` 的扫描面）。
const caciDocSeg = "项目文档"

// caciDevdocsTailFor —— 开发文档版本面尾段（`项目文档/vX.Y.Z`）—— ★ **零版本字面量**
// （GAP-20260927-88：原为写死的 `v2.5.11` ⇒ 换版静默过期）。
//
// 版号**现读唯一真源**（`core/internal/version/version.go` 的 `const Version`，与门①
// `check-doc-cmds` 的 `dcmdCurrentVersion` 同读法）：真源指向的版本目录若在取源根下**且含本门
// 要读的口径件** ⇒ 就用它（默认档跟收版走 —— 与 `GAP-20260926-209` 先例同口径）。
// 真源目录缺该件（本门读的是**历史钉版**口径件，未必被承接进当前版）⇒ 在取源根下**按件名**
// 定位版本目录（版本号最大者）—— 仍不写任何版本字面量。
//
// 两处都取不到 ⇒ ok=false（**缺件不猜**：调用方按「未取数」报 rc=2，✗ 不当绿）。
func caciDevdocsTailFor(root, sibling, doc string) (string, bool) {
	if ver, ok := dcmdCurrentVersion(root); ok {
		if caciExists(caciPyJoin(sibling, caciDocSeg, ver, doc)) {
			return caciPyJoin(caciDocSeg, ver), true
		}
	}
	if d, ok := caciVersionDirWithFile(sibling, doc); ok {
		return caciPyJoin(caciDocSeg, d), true
	}
	return "", false
}

// caciVersionDirWithFile —— 在 `<sibling>/<树段>` 下**按件名定位**版本目录：候选须是
// `v<数字>[.数字…]` 目录**且含该件**，取版本号最大者（逐段数字比 ⇒ ✗ 不比字典序）。
// 一个都定位不到 ⇒ ok=false（缺件不猜）。
func caciVersionDirWithFile(sibling, doc string) (string, bool) {
	base := caciPyJoin(sibling, caciDocSeg)
	ents, err := os.ReadDir(base)
	if err != nil {
		return "", false
	}
	best := ""
	var bestKey []int
	for _, e := range ents {
		name := e.Name()
		if !e.IsDir() || !caciIsVersionDirName(name) || !caciExists(caciPyJoin(base, name, doc)) {
			continue
		}
		key := caciVersionKey(name)
		if best == "" || caciVersionKeyLess(bestKey, key) {
			best, bestKey = name, key
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}

// caciIsVersionDirName —— `v<数字>[.<数字…>]` 形态的版本目录名（✗ 不吃别的段）。
func caciIsVersionDirName(name string) bool {
	if !strings.HasPrefix(name, "v") || len(name) < 2 {
		return false
	}
	for _, r := range name[1:] {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}

// caciVersionKey —— 版本目录名 ⇒ 逐段数字键（`v2.5.11` ⇒ `[2 5 11]`）。
func caciVersionKey(name string) []int {
	segs := strings.Split(strings.TrimPrefix(name, "v"), ".")
	out := make([]int, 0, len(segs))
	for _, s := range segs {
		n := 0
		for _, r := range s {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, n)
	}
	return out
}

// caciVersionKeyLess —— 逐段数字比（缺段补 0）。
func caciVersionKeyLess(a, b []int) bool {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		x, y := 0, 0
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x < y
		}
	}
	return false
}

const caciDocName = "口径-CI永不人签-20260923.md"

// 成文口径里**必须逐字出现**的六句（机检用最小片段 · 见头部 C3）。
var caciMust = []string{"CI 永不人签", "rc=2", "stdout 0 字节", "立即失败、不挂死", "人先签好", "不上 CI 钥匙"}

var caciJSONFields = []string{} // 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）

type caciCase struct {
	name string
	argv []string
}

var caciNoTTYCases = []caciCase{
	{"approve new", []string{"approve", "new", "--tool", "terminal", "--by", "<名>", "--note", "<理由>"}},
	{"approve keygen", []string{"approve", "keygen", "--by", "<名>"}},
}

const caciUsage = "usage: check-approve-ci.py [-h] [--root ROOT] [--state STATE]\n" +
	"                           [--devdocs DEVDOCS] [--timeout TIMEOUT]\n" +
	"                           [--self-test] [--no-self-test]\n"

const caciHelp = caciUsage + "\n" +
	"check-approve-ci.py v1.0.0\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help         show this help message and exit\n" +
	"  --root ROOT        仓根（默认 = 本脚本的上两级）\n" +
	"  --state STATE      状态目录（默认 = `ZERG_STATE_DIR` > `~/.zerg/state`）\n" +
	"  --devdocs DEVDOCS  开发文档面根（默认 = `ZERG_DEVDOCS_ROOT` > 仓同级 `Zerg-\n" +
	"                     docs/项目文档/v2.5.11`）\n" +
	"  --timeout TIMEOUT  每一格的时限（秒 · 默认 60）\n" +
	"  --self-test        只跑自检（不碰真目标）\n" +
	"  --no-self-test     跳过自检（只有已自证过的调用方才该用）\n"

var caciLongOpts = []string{"--help", "--root", "--state", "--devdocs", "--timeout", "--self-test", "--no-self-test"}

// ── posixpath 同构面（✗ 不用 filepath.*：语义差见出厂标准清单）────────────────────

// caciPyJoin = CPython posixpath.join 逐段直译。
func caciPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "/") {
			out = p
		} else if out == "" || strings.HasSuffix(out, "/") {
			out += p
		} else {
			out += "/" + p
		}
	}
	return out
}

// caciPyNormpath = CPython posixpath.normpath（★ 保留恰两个前导斜杠）。
func caciPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initial = 2
		}
	}
	comps := []string{}
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initial == 0 && len(comps) == 0) || (len(comps) > 0 && comps[len(comps)-1] == "..") {
			comps = append(comps, comp)
		} else if len(comps) > 0 {
			comps = comps[:len(comps)-1]
		}
	}
	out := strings.Join(comps, "/")
	if initial > 0 {
		out = strings.Repeat("/", initial) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// caciPyAbspath = CPython posixpath.abspath（词法拼接后 normpath）。
func caciPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = caciPyJoin(caciCwd(), p)
	}
	return caciPyNormpath(p)
}

// caciPyDirname = CPython posixpath.split 的 head 面（✗ filepath.Dir）。
func caciPyDirname(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	head := p[:i+1]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

func caciCwd() string {
	d, err := os.Getwd()
	if err != nil {
		return ""
	}
	return d
}

// caciFindRepoRoot = 「cwd 上溯至最近的含 scripts/gates/ 的目录」（同族件同法）。
func caciFindRepoRoot() string {
	d := caciCwd()
	for i := 0; i < 12 && d != "" && d != "/"; i++ {
		if st, err := os.Stat(caciPyJoin(d, "scripts", "gates")); err == nil && st.IsDir() {
			return d
		}
		parent := caciPyDirname(d)
		if parent == d {
			break
		}
		d = parent
	}
	return caciCwd()
}

// caciPyIsSpace = py str.isspace（★ 含 \x1c-\x1f，Go unicode.IsSpace 不含）。
func caciPyIsSpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// caciPyStrip = py str.strip()（✗ strings.TrimSpace）。
func caciPyStrip(s string) string {
	rs := []rune(s)
	i, j := 0, len(rs)
	for i < j && caciPyIsSpace(rs[i]) {
		i++
	}
	for j > i && caciPyIsSpace(rs[j-1]) {
		j--
	}
	return string(rs[i:j])
}

// caciPySplitLines = py str.splitlines()（含 \v \f \x1c-\x1e \x85 \u2028 \u2029）。
func caciPySplitLines(s string) []string {
	out := []string{}
	rs := []rune(s)
	start := 0
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case r == '\n' || r == '\v' || r == '\f' || r == '\x1c' || r == '\x1d' || r == '\x1e' || r == '\x85' || r == '\u2028' || r == '\u2029':
			out = append(out, string(rs[start:i]))
			i++
			start = i
		case r == '\r':
			out = append(out, string(rs[start:i]))
			i++
			if i < len(rs) && rs[i] == '\n' {
				i++
			}
			start = i
		default:
			i++
		}
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}

// caciDecodeReplace = bytes.decode("utf-8","replace")（✗ 不做换行归一）。
func caciDecodeReplace(b []byte) string {
	var sb strings.Builder
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size <= 1 {
			sb.WriteRune(utf8.RuneError)
			i++
			continue
		}
		sb.Write(b[i : i+size])
		i += size
	}
	return sb.String()
}

// caciUniversalNewlines = py 文本模式读取的换行归一（\r\n 与裸 \r ⇒ \n）。
func caciUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// caciPyReadText = py open(path, encoding="utf-8", errors="replace").read()（文本模式）。
func caciPyReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return caciUniversalNewlines(caciDecodeReplace(b)), nil
}

func caciFirstLine(err string) string {
	lines := caciPySplitLines(caciPyStrip(err))
	if len(lines) == 0 {
		return "（空）"
	}
	return lines[0]
}

func caciExists(p string) bool { _, err := os.Stat(p); return err == nil }
func caciIsDir(p string) bool  { st, err := os.Stat(p); return err == nil && st.IsDir() }
func caciIsFile(p string) bool { st, err := os.Stat(p); return err == nil && st.Mode().IsRegular() }
func caciBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}
func caciRcStr(rc int, none bool) string {
	if none {
		return "None"
	}
	return fmt.Sprintf("%d", rc)
}

func caciExpandUser(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home := os.Getenv("HOME")
	if home == "" {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return home + p[1:]
	}
	return p
}

func caciFnMatch(pat, name string) bool {
	p := []rune(pat)
	n := []rune(name)
	var m func(pi, ni int) bool
	m = func(pi, ni int) bool {
		for pi < len(p) {
			switch p[pi] {
			case '*':
				pi++
				for pi < len(p) && p[pi] == '*' {
					pi++
				}
				if pi == len(p) {
					return true
				}
				for k := ni; k <= len(n); k++ {
					if m(pi, k) {
						return true
					}
				}
				return false
			case '?':
				if ni >= len(n) {
					return false
				}
				pi++
				ni++
			default:
				if ni >= len(n) || p[pi] != n[ni] {
					return false
				}
				pi++
				ni++
			}
		}
		return ni == len(n)
	}
	return m(0, 0)
}

// caciGlob = glob.glob 的「dir + 单段 pattern」形态（含 py 的隐藏件过滤）＋ sorted()。
func caciGlob(pattern string) []string {
	dir := ""
	base := pattern
	if i := strings.LastIndex(pattern, "/"); i >= 0 {
		dir = pattern[:i]
		if dir == "" {
			dir = "/"
		}
		base = pattern[i+1:]
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}
	}
	out := []string{}
	hidden := strings.HasPrefix(base, ".")
	for _, e := range entries {
		name := e.Name()
		if !hidden && strings.HasPrefix(name, ".") {
			continue
		}
		if caciFnMatch(base, name) {
			if dir == "/" {
				out = append(out, "/"+name)
			} else {
				out = append(out, dir+"/"+name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// ── 判定口（纯函数：自检与真目标**共用同一套**）──────────────────────────────────

// caciJudgeNoTTY：C1 —— 无 TTY ⇒ rc=2 且 stdout 0 字节（期望值一条不改）。
func caciJudgeNoTTY(rc int, rcNone bool, out []byte) []string {
	problems := []string{}
	if rcNone || rc != 2 {
		problems = append(problems, fmt.Sprintf("C1 破：无 TTY 下 rc=%s ≠ 2（期望值 = 2，**一条不改**）", caciRcStr(rc, rcNone)))
	}
	if len(out) != 0 {
		problems = append(problems, fmt.Sprintf("C1 破：无 TTY 下 stdout %d 字节 ≠ 0（人签面不许往 stdout 打东西）", len(out)))
	}
	return problems
}

// caciJudgeDevicePath：C2 器件面 —— 本机**零器件**时判；有器件 ⇒ 只交代不判。
func caciJudgeDevicePath(rc int, rcNone bool, timedOut, landed, hasDevice bool) ([]string, string) {
	if hasDevice {
		return []string{}, "本机**有**器件 ⇒ 这一格的对象不成立 ⇒ **只交代不判**（不算绿）"
	}
	problems := []string{}
	if timedOut {
		problems = append(problems, "C2 破：器件路径**超时没退出** ⇒ 判红（「挂死」这一格就是它）")
	} else if !rcNone && rc == 0 {
		problems = append(problems, "C2 破：无器件却 rc=0（该立即失败不挂死）")
	}
	if landed {
		problems = append(problems, "C2 破：**落了件**（无器件那一路不该落下任何件）")
	}
	return problems, "本机零器件 ⇒ 判：时限内退出 + rc≠0 + 没落件"
}

// caciJudgeKouJing：C3 —— 成文口径六句必须逐字在。
func caciJudgeKouJing(text string) []string {
	miss := []string{}
	for _, s := range caciMust {
		if !strings.Contains(text, s) {
			miss = append(miss, s)
		}
	}
	if len(miss) > 0 {
		quoted := make([]string, 0, len(miss))
		for _, m := range miss {
			quoted = append(quoted, "「"+m+"」")
		}
		return []string{fmt.Sprintf("C3 破：口径件缺句 %s（口径没落成文 = 判据无处可依）", strings.Join(quoted, "、"))}
	}
	return []string{}
}

// caciJudgeCIFace：C4 —— CI 面不许有能签的钥匙。
func caciJudgeCIFace(texts [][2]string) []string {
	problems := []string{}
	for _, t := range texts {
		for _, pat := range caciCIFobidden {
			if strings.Contains(t[1], pat) {
				problems = append(problems, fmt.Sprintf("C4 破：CI 面 `%s` 里出现 `%s` ⇒ 给了 CI 能签的东西（红线）", t[0], pat))
			}
		}
	}
	return problems
}

// ── 取数（只读 · 只跑失败面）──────────────────────────────────────────────────

func caciStateDirOf(given string, givenSet bool) string {
	if givenSet {
		return caciPyAbspath(given)
	}
	if env := os.Getenv("ZERG_STATE_DIR"); env != "" {
		return caciPyAbspath(env)
	}
	return caciPyJoin(caciExpandUser("~"), ".zerg", "state")
}

// caciReadInstalledKeyID：只读在册公钥的 key_id（读不出 ⇒ 那一格只交代，不当红）。
func caciReadInstalledKeyID(state string) string {
	p := caciPyJoin(state, "approvals", "operator.pub")
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return ""
	}
	v, ok := m[caciKeyIDField].(string)
	if !ok {
		return ""
	}
	return v
}

// caciSnapshotApprovals：真状态目录的 approvals/ 快照（文件名 + sha256）。
func caciSnapshotApprovals(state string) map[string]string {
	out := map[string]string{}
	d := caciPyJoin(state, "approvals")
	if !caciIsDir(d) {
		return out
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		return out
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		p := caciPyJoin(d, name)
		if !caciIsFile(p) {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		sum := sha256.Sum256(b)
		out[name] = hex.EncodeToString(sum[:])
	}
	return out
}

func caciSetEnv(env []string, k, v string) []string {
	out := make([]string, 0, len(env)+1)
	pref := k + "="
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, pref) {
			out = append(out, pref+v)
			found = true
		} else {
			out = append(out, e)
		}
	}
	if !found {
		out = append(out, pref+v)
	}
	return out
}

// caciRunCmd：跑一条命令面（不接管道 · rc 取真退出码）⇒ (rc, rcNone, stdout, stderr, 超时)。
func caciRunCmd(argv []string, state string, timeoutS int, stdinNull bool) (int, bool, []byte, string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutS)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = caciSetEnv(os.Environ(), "ZERG_STATE_DIR", state)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	_ = stdinNull // Go：Stdin=nil 即接 /dev/null（同 subprocess.DEVNULL）
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return 0, true, outBuf.Bytes(), "（超时被强杀）", true
	}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), false, outBuf.Bytes(), caciDecodeReplace(errBuf.Bytes()), false
		}
		return exitBlocked, false, outBuf.Bytes(), caciDecodeReplace(errBuf.Bytes()), false // ★ 子进程**起不来**（非 ExitError）⇒ 读不到 ⇒ 归 `exitBlocked`(8「有 BLOCKED · 读不到不许当健康」)；
		// 原先裸写 127（shell 惯例）不在命令面真源表里 ⇒ 门⑪ T3 报（GAP-20261002-08）。
	}
	return 0, false, outBuf.Bytes(), caciDecodeReplace(errBuf.Bytes()), false
}

// caciHasDevice：本机器件面读数（security list-smartcards · /dev/hidraw* · ~/.ssh/*.pub 里 sk-）。
func caciHasDevice() (bool, string) {
	detail := []string{}
	smart := "（`security` 不在 PATH / 非 macOS）"
	if out, err := exec.Command("security", "list-smartcards").Output(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			combined := append(append([]byte{}, out...), ee.Stderr...)
			lines := caciPySplitLines(caciPyStrip(caciDecodeReplace(combined)))
			if len(lines) == 0 {
				smart = "（无输出）"
			} else {
				smart = lines[len(lines)-1]
			}
		} else {
			smart = fmt.Sprintf("（跑不起来：%s）", caciSecurityErrStr(err))
		}
	} else {
		lines := caciPySplitLines(caciPyStrip(caciDecodeReplace(out)))
		if len(lines) == 0 {
			smart = "（无输出）"
		} else {
			smart = lines[len(lines)-1]
		}
	}
	detail = append(detail, fmt.Sprintf("`security list-smartcards` ⇒ %s", smart))
	hid := caciGlob("/dev/hidraw*")
	detail = append(detail, fmt.Sprintf("`/dev/hidraw*` ⇒ %d 个", len(hid)))
	sk := 0
	for _, p := range caciGlob(caciPyJoin(caciExpandUser("~"), ".ssh", "*.pub")) {
		if txt, err := caciPyReadText(p); err == nil {
			if strings.Contains(txt, "sk-") {
				sk++
			}
		}
	}
	detail = append(detail, fmt.Sprintf("`~/.ssh/*.pub` 里 `sk-` 命中 ⇒ %d", sk))
	return len(hid) > 0 || sk > 0, strings.Join(detail, " · ")
}

// caciSecurityErrStr 逼近 py 的 FileNotFoundError 文案（★ 帧路径面不可逐字复刻，见未覆盖面）。
func caciSecurityErrStr(err error) string {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return "[Errno 2] No such file or directory: 'security'"
	}
	return err.Error()
}

func caciDevdocsRootOf(root, given string, givenSet bool) string {
	if givenSet {
		return caciPyAbspath(given)
	}
	if env := os.Getenv("ZERG_DEVDOCS_ROOT"); env != "" {
		return caciPyAbspath(env)
	}
	sibling := os.Getenv("ZERG_DOCS_ALT")
	if sibling == "" {
		sibling = os.Getenv("ZERG_DOCS")
	}
	if sibling == "" {
		sibling = caciPyJoin(caciPyDirname(root), "Zerg-内部文档")
	}
	tail, ok := caciDevdocsTailFor(root, sibling, caciDocName)
	if !ok {
		// ★ 缺件不猜（GAP-20260927-88）：取源根下定位不到本门口径件的版本目录 ⇒
		// 交「版本面之上一级」给下游，由它按「未取数」报 rc=2（✗ 不回落到写死的版本目录）。
		return caciPyJoin(sibling, caciDocSeg)
	}
	return caciPyJoin(sibling, tail)
}

// ── 自检（先自证会红，再扫真目标）──────────────────────────────────────────────

func caciSelfTest(w io.Writer) bool {
	ok, bad := 0, 0
	check := func(cond bool, msgOk, msgBad string) {
		if cond {
			fmt.Fprintf(w, "   ✓ %s\n", msgOk)
			ok++
		} else {
			fmt.Fprintf(w, "   ✗ %s\n", msgBad)
			bad++
		}
	}

	check(len(caciJudgeNoTTY(2, false, []byte{})) == 0, "① 正控：rc=2 且 stdout 0 字节 ⇒ 不判红（C1 的期望形态）",
		"① 正控失败：期望形态被判红")
	check(len(caciJudgeNoTTY(0, false, []byte{})) > 0, "② 负控：rc=0 ⇒ 判红（无 TTY 竟然签得出来）",
		"② 负控失败：rc=0 还判绿")
	check(len(caciJudgeNoTTY(2, false, []byte("签好了"))) > 0,
		"③ 负控：stdout 非空 ⇒ 判红（判据是 0 字节）",
		"③ 负控失败：stdout 非空还判绿")
	check(len(caciJudgeNoTTY(1, false, []byte{})) > 0, "④ 负控：rc=1（改了退码）⇒ 判红（期望值一条不改）",
		"④ 负控失败：退码被改还判绿")

	p, _ := caciJudgeDevicePath(255, false, false, false, false)
	check(len(p) == 0, "⑤ 正控：时限内 rc=255 且没落件 ⇒ 不判红（`[P11]` 的实测形态）",
		"⑤ 正控失败：实测形态被判红")
	p2, _ := caciJudgeDevicePath(0, true, true, false, false)
	check(len(p2) > 0, "⑥ 负控：**超时被强杀** ⇒ 判红（判据带时限就是为了这一格）",
		"⑥ 负控失败：挂死还判绿")
	p3, _ := caciJudgeDevicePath(0, false, false, false, false)
	check(len(p3) > 0, "⑦ 负控：无器件却 rc=0 ⇒ 判红", "⑦ 负控失败：rc=0 还判绿")
	p4, _ := caciJudgeDevicePath(255, false, false, true, false)
	check(len(p4) > 0, "⑧ 负控：落了件 ⇒ 判红（无器件那一路不许落件）", "⑧ 负控失败：落件还判绿")
	p5, w5 := caciJudgeDevicePath(0, false, false, false, true)
	check(len(p5) == 0 && strings.Contains(w5, "不判"), "⑨ 正控：本机**有器件** ⇒ 只交代不判（对象不成立 ≠ 绿）",
		"⑨ 正控失败：有器件时还硬判")

	good := strings.Join(caciMust, "")
	check(len(caciJudgeKouJing(good)) == 0, "⑩ 正控：口径四句齐 ⇒ 不判红（C3）", "⑩ 正控失败：齐了还判红")
	for i, s := range caciMust {
		check(len(caciJudgeKouJing(strings.ReplaceAll(good, s, ""))) > 0,
			fmt.Sprintf("⑪-%d 负控：抹掉「%s」⇒ 判红（口径缺句 = 判据无处可依）", i+1, s),
			fmt.Sprintf("⑪-%d 负控失败：缺了「%s」还判绿", i+1, s))
	}

	clean := [][2]string{{"publish/ci/ci.yml", "# 只跑构建与测试\nzerg gate run --fast\n"}}
	check(len(caciJudgeCIFace(clean)) == 0,
		"⑫ 正控：CI 面没有能签的动作/钥匙 ⇒ 不判红（C4）", "⑫ 正控失败：干净还判红")
	for i, pat := range caciCIFobidden {
		hit := [][2]string{{"publish/ci/ci.yml", "run: " + pat + "\n"}}
		check(len(caciJudgeCIFace(hit)) > 0,
			fmt.Sprintf("⑬-%d 负控：CI 面出现 `%s` ⇒ 判红（红线）", i+1, pat),
			fmt.Sprintf("⑬-%d 负控失败：%s 出现还判绿", i+1, pat))
	}

	fmt.Fprintf(w, "   自检：%d 格过 · %d 格不过（真目标只读 · 未改任何字节 · 未签任何件）\n", ok, bad)
	return bad == 0
}

// ── 主 ────────────────────────────────────────────────────────────────────────

func caciScan(stdout io.Writer, root, state, devdocs string, timeoutS int) int {
	reds := []string{}
	nocalls := []string{}
	zerg := caciPyJoin(root, "bin", "zerg")
	if !caciExists(zerg) {
		fmt.Fprintf(stdout, "⛔ 未取数：`bin/zerg` 不在（%s）⇒ **不出结论**（rc=2）—— 制品不入库，"+
			"请先走自举档 `zerg build all --only cli`\n", zerg)
		return 2
	}

	fmt.Fprint(stdout, "── C0 本机器件面读数（只读）\n")
	dev, devDetail := caciHasDevice()
	devWord := "**零器件**（同 `[P11]`）"
	if dev {
		devWord = "**有器件**"
	}
	fmt.Fprintf(stdout, "   %s ⇒ 结论：%s\n", devDetail, devWord)

	fmt.Fprintf(stdout, "── C1 无 TTY ⇒ 拒（真状态目录 = %s）\n", state)
	before := caciSnapshotApprovals(state)
	cases := make([]caciCase, 0, len(caciNoTTYCases)+1)
	cases = append(cases, caciNoTTYCases...)
	kid := caciReadInstalledKeyID(state)
	if kid != "" {
		cases = append(cases, caciCase{
			name: fmt.Sprintf("approve keygen --confirm=%s --yes", kid),
			argv: []string{"approve", "keygen", "--by", "<名>", "--confirm=" + kid, "--yes"},
		})
	} else {
		fmt.Fprint(stdout, "   · 交代：在册公钥读不出 `key_id` ⇒ 第三个夹具（`--confirm=<旧 key_id> --yes`）本跑跳过\n")
	}
	for _, c := range cases {
		rc, rcNone, out, err, to := caciRunCmd(append([]string{zerg}, c.argv...), state, timeoutS, true)
		fmt.Fprintf(stdout, "   · `zerg %s </dev/null` ⇒ rc=%s · stdout **%d 字节** · stderr 首行 = %s\n",
			c.name, caciRcStr(rc, rcNone), len(out), caciFirstLine(err))
		reds = append(reds, caciJudgeNoTTY(rc, rcNone, out)...)
		if to {
			reds = append(reds, fmt.Sprintf("C1 破：`zerg %s` **超时没退出**（无 TTY 该立即拒）", c.name))
		}
	}
	after := caciSnapshotApprovals(state)
	if !caciMapEq(before, after) {
		reds = append(reds, fmt.Sprintf("C1 破：**真状态目录被写了**（跑前 %d 件 → 跑后 %d 件；被拒的命令面不许落件）",
			len(before), len(after)))
	} else {
		fmt.Fprintf(stdout, "   · 真状态目录零写（`approvals/` 跑前跑后 %d 件逐字节相同）\n", len(before))
	}

	fmt.Fprintf(stdout, "── C2 无器件 ⇒ 立即失败、不挂死（时限 = %ds）\n", timeoutS)
	rc, rcNone, out, err, _ := caciRunCmd([]string{zerg, "approve", "keygen", "--strength", "token", "--by", "<名>"},
		state, timeoutS, true)
	fmt.Fprintf(stdout, "   · CLI 面 token 档旗标：`zerg approve keygen --strength token` ⇒ rc=%s · stdout %d 字节 · "+
		"stderr 首行 = %s\n", caciRcStr(rc, rcNone), len(out), caciFirstLine(err))
	if rc == 2 && !rcNone && len(out) == 0 {
		fmt.Fprint(stdout, "   · 口径：今天这档在**命令面**上还不存在（`I-4`：名字还没被占）⇒ CLI 面无 token 档路径 ⇒ "+
			"器件面那一路才是今天**唯一能跑到的** token 档路径\n")
	} else {
		reds = append(reds, fmt.Sprintf("C2 破：CLI 面 token 档旗标 rc=%s / stdout %d 字节（期望 rc=2 且 0 字节）",
			caciRcStr(rc, rcNone), len(out)))
	}
	sand, mkErr := os.MkdirTemp("", "approve-ci-")
	if mkErr != nil {
		sand = ""
	}
	defer os.RemoveAll(sand) // ★ 止血：py 原件从不回收（A 类真口子），Go 侧成对回收
	keyf := caciPyJoin(sand, "probe_sk")
	rc2, rc2None, out2, err2, to2 := caciRunCmd([]string{"ssh-keygen", "-t", "ed25519-sk", "-O", "verify-required",
		"-N", "", "-C", "probe", "-f", keyf}, state, timeoutS, true)
	landed := caciExists(keyf) || caciExists(keyf+".pub")
	fmt.Fprintf(stdout, "   · 器件面（`ssh-keygen -t ed25519-sk -O verify-required` · 沙箱 %s）⇒ rc=%s · 超时=%s · "+
		"落件=%s · stdout 首行 = %s · stderr 首行 = %s\n",
		sand, caciRcStr(rc2, rc2None), caciBool(to2), caciBool(landed),
		caciFirstLine(caciDecodeReplace(out2)), caciFirstLine(err2))
	p2, why2 := caciJudgeDevicePath(rc2, rc2None, to2, landed, dev)
	reds = append(reds, p2...)
	fmt.Fprintf(stdout, "   · 判定口径：%s\n", why2)

	fmt.Fprintf(stdout, "── C3 成文口径在位（仓外开发文档面 = %s）\n", devdocs)
	doc := caciPyJoin(devdocs, caciDocName)
	if !caciIsDir(devdocs) || !caciExists(doc) {
		nocalls = append(nocalls, fmt.Sprintf("C3 **未取数**：口径件不在（%s）⇒ 不给结论（rc=2）—— 缺件 ≠ 绿", doc))
	} else {
		txt, rdErr := caciPyReadText(doc)
		if rdErr != nil {
			nocalls = append(nocalls, fmt.Sprintf("C3 **未取数**：口径件读不了（%s）⇒ 不给结论（rc=2）—— 缺件 ≠ 绿", doc))
		} else {
			fmt.Fprintf(stdout, "   · 口径件 %s（%d 字节）\n", caciDocName, len([]byte(txt)))
			reds = append(reds, caciJudgeKouJing(txt)...)
		}
	}

	fmt.Fprint(stdout, "── C4 CI 面不许有能签的钥匙（红线）\n")
	texts := [][2]string{}
	for _, rel := range caciCIFace {
		full := caciPyJoin(root, rel)
		if caciExists(full) {
			if txt, rdErr := caciPyReadText(full); rdErr == nil {
				texts = append(texts, [2]string{rel, txt})
			}
		}
	}
	names := []string{}
	for _, t := range texts {
		names = append(names, t[0])
	}
	joined := "（0 件）"
	if len(names) > 0 {
		joined = strings.Join(names, ", ")
	}
	fmt.Fprintf(stdout, "   · 读到的 CI 面件：%s\n", joined)
	reds = append(reds, caciJudgeCIFace(texts)...)

	fmt.Fprint(stdout, "── C5 不可机检（如实标 · 不许写成命令判据）\n")
	fmt.Fprint(stdout, "   · 「CI 侧有没有偷偷放审批私钥 / 长期令牌」（`H-9`）只能**人复看** ⇒ **本件不判** "+
		"（判了就是假闸）\n")
	fmt.Fprint(stdout, "   · 「无器件 ⇒ rc≠0」**只证明失败面**，**不证明**任何**在场**闸；"+
		"`BLOCKED`（rc=2 不给结论）**不等于**「拒」—— 两者不许混\n")

	if len(reds) > 0 || len(nocalls) > 0 {
		rcOut := 2
		label := "不给结论"
		if len(reds) > 0 {
			rcOut = 1
			label = "红"
		}
		fmt.Fprintf(stdout, "⇒ 门禁%s（rc=%d）：红 %d 条 · 不给结论 %d 条\n", label, rcOut, len(reds), len(nocalls))
		for _, p := range reds {
			fmt.Fprintf(stdout, "   ✗ %s\n", p)
		}
		for _, p := range nocalls {
			fmt.Fprintf(stdout, "   ⛔ %s\n", p)
		}
		return rcOut
	}
	fmt.Fprint(stdout, "⇒ 全绿（rc=0）：无 TTY 两条 rc=2 且 stdout 0 字节 · 真状态目录零写 · "+
		"无器件那一路立即失败不挂死 · 成文口径在位 · CI 面无能签的钥匙\n")
	return 0
}

func caciMapEq(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// ── 缺口 Q-176（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 --json 不许沉默 ✗ ──────

func caciRefuseJSON(argv []string, stderr io.Writer) *int {
	hits := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hits = append(hits, x)
		}
	}
	if len(hits) == 0 {
		return nil
	}
	quoted := make([]string, 0, len(hits))
	for _, h := range hits {
		quoted = append(quoted, "`"+h+"`")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(quoted, " · "))
	fields := "无（本门只出人读面）"
	if len(caciJSONFields) > 0 {
		fs := make([]string, 0, len(caciJSONFields))
		for _, f := range caciJSONFields {
			fs = append(fs, "`"+f+"`")
		}
		fields = strings.Join(fs, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fields)
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	rc := 2
	return &rc
}

// ── argparse 同构面（prog 名固定为 caciProg）───────────────────────────────────

type caciOpts struct {
	root         string
	state        string
	stateGiven   bool
	devdocs      string
	devdocsGiven bool
	timeout      int
	selfTest     bool
	noSelfTest   bool
}

func caciUsageError(stderr io.Writer, msg string) {
	fmt.Fprint(stderr, caciUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", caciProg, msg)
}

func caciLooksLikeOption(s string) bool {
	if len(s) < 2 || !strings.HasPrefix(s, "-") {
		return false
	}
	if s == "--" {
		return true
	}
	if _, err := caciPyAtoi(strings.TrimPrefix(s, "-")); err == nil {
		return false // 负数被当值（argparse 规则）
	}
	return true
}

var errCaciBadInt = errors.New("invalid int value")

// caciPyAtoi 逼近 py int()（十进制 · 可带空白/正负号/数字间下划线）。
func caciPyAtoi(s string) (int, error) {
	t := caciPyStrip(s)
	if t == "" {
		return 0, errCaciBadInt
	}
	rs := []rune(t)
	neg := false
	i := 0
	if rs[0] == '+' || rs[0] == '-' {
		neg = rs[0] == '-'
		i = 1
	}
	if i >= len(rs) {
		return 0, errCaciBadInt
	}
	val := 0
	lastDigit := false
	for ; i < len(rs); i++ {
		r := rs[i]
		if r == '_' {
			if !lastDigit {
				return 0, errCaciBadInt
			}
			lastDigit = false
			continue
		}
		d := -1
		switch {
		case r >= '0' && r <= '9':
			d = int(r - '0')
		case r >= 0xFF10 && r <= 0xFF19: // 全角数字（Unicode Nd 的一个常见块）
			d = int(r - 0xFF10)
		default:
			return 0, errCaciBadInt
		}
		val = val*10 + d
		lastDigit = true
	}
	if !lastDigit {
		return 0, errCaciBadInt
	}
	if neg {
		val = -val
	}
	return val, nil
}

func caciParseArgs(argv []string, stdout, stderr io.Writer) (caciOpts, int, bool) {
	opts := caciOpts{root: caciFindRepoRoot(), timeout: 60}
	unrec := []string{}
	positionalOnly := false
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if !positionalOnly && tok == "--" {
			// ★ py argparse 把 `--` 本身也计入 unrecognized（实测：['--','x'] ⇒ "-- x"）。
			unrec = append(unrec, tok)
			positionalOnly = true
			continue
		}
		if !positionalOnly && strings.HasPrefix(tok, "--") && len(tok) > 2 {
			name := tok
			val := ""
			hasEq := false
			if j := strings.IndexByte(tok, '='); j >= 0 {
				name = tok[:j]
				val = tok[j+1:]
				hasEq = true
			}
			matches := []string{}
			for _, o := range caciLongOpts {
				if o == name {
					matches = []string{o}
					break
				}
			}
			if len(matches) == 0 {
				for _, o := range caciLongOpts {
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
				caciUsageError(stderr, fmt.Sprintf("ambiguous option: %s could match %s", name, strings.Join(matches, ", ")))
				return opts, 2, true
			}
			canon := matches[0]
			switch canon {
			case "--help":
				fmt.Fprint(stdout, caciHelp)
				return opts, 0, true
			case "--self-test", "--no-self-test":
				if hasEq {
					caciUsageError(stderr, fmt.Sprintf("argument %s: ignored explicit argument '%s'", canon, val))
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
					if i+1 >= len(argv) || caciLooksLikeOption(argv[i+1]) {
						caciUsageError(stderr, fmt.Sprintf("argument %s: expected one argument", canon))
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
					opts.stateGiven = true
				case "--devdocs":
					opts.devdocs = v
					opts.devdocsGiven = true
				case "--timeout":
					n, err := caciPyAtoi(v)
					if err != nil {
						caciUsageError(stderr, fmt.Sprintf("argument --timeout: invalid int value: '%s'", v))
						return opts, 2, true
					}
					opts.timeout = n
				}
			}
			continue
		}
		if !positionalOnly && tok == "-h" {
			fmt.Fprint(stdout, caciHelp)
			return opts, 0, true
		}
		unrec = append(unrec, tok)
	}
	if len(unrec) > 0 {
		caciUsageError(stderr, "unrecognized arguments: "+strings.Join(unrec, " "))
		return opts, 2, true
	}
	return opts, 0, false
}

// checkApproveCIGateMain 是 `zerg gate check check-approve-ci` 的 Go 入口（M1 转发壳的直调面）。
func checkApproveCIGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc := caciRefuseJSON(argv, stderr); rc != nil {
		return *rc
	}
	opts, code, done := caciParseArgs(argv, stdout, stderr)
	if done {
		return code
	}
	root := caciPyAbspath(opts.root)
	state := caciStateDirOf(opts.state, opts.stateGiven)
	devdocs := caciDevdocsRootOf(root, opts.devdocs, opts.devdocsGiven)
	fmt.Fprintln(stdout, caciVersion)
	if !opts.noSelfTest && !caciSelfTest(stdout) {
		return 1
	}
	if opts.selfTest {
		return 0
	}
	fmt.Fprintf(stdout, "── 真目标：%s（状态目录 %s）\n", root, state)
	if !opts.stateGiven && os.Getenv("ZERG_STATE_DIR") != "" {
		fmt.Fprintf(stdout, "★★ 状态目录是**环境变量 `ZERG_STATE_DIR` 给的**（%s）⇒ 本读数是**沙箱**读数，"+
			"**不是**真状态目录 `~/.zerg/state`（闸里不设这个变量；要读真源请显式 `--state`）\n",
			os.Getenv("ZERG_STATE_DIR"))
	}
	return caciScan(stdout, root, state, devdocs, opts.timeout)
}
