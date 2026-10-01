// check_public_tree_selfcheck.go —— 判据件 `scripts/gates/check-public-tree-selfcheck.py`（575 行 ·
// 现货 sha16 `35357dee58d24315`）的 **Go 真搬**（甲案 C 类 · M2 · 入口 `checkPublicTreeSelfcheckGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的口径差面与未覆盖面）。
//
// 本件判的什么：公开树（`whitelist ∪ replace-rules` 的**产物**）跟私有全树不是同一份东西 ——
// 本机永远看不见只在公开树上成立的红 ⇒ 拿公开树当本机一份 checkout **跑一遍关键测试子集**
// （P1 `go test ./internal/obs/redact/` · P2 `python3 ui/scripts/check-i18n.py`）⇒ 逐条点名。
// 外加「双方 HEAD sha 对应」判据（本机 HEAD ⟷ 公开树 HEAD；不等 ⇒ 明确「不对应」且**不当绿**）。
//
// ═══════════════════════ 对外交互面（逐条：遍历 / 子进程 / 写面 / 外部真源）═══════════════════════
//
//	★ **遍历面：本件零目录遍历**（全文无 `walk` / `listdir` / `scandir` / `glob` / `ReadDir` /
//	  `WalkDir`）—— py 原件同样零遍历（只 `os.path.exists` 逐个点名 + 子进程）。⇒ 遍历序问题**不适用**，
//	  给的是「不适用 + 零命中」的代码级理由，不是「碰不到」。
//	★ **子进程面（运行时真起，共 4 类 · 逐点行号）**：
//	    · py `:192` `read_head()` 起 `git -C <tree> rev-parse [--short] HEAD`
//	    · py `:254` `materialize()` 起 `git clone --depth 1 -q <repo> <dest>`（**唯一联网点** · 缺省目标
//	      是公开仓 `DEFAULT_REPO`）＋ `:265` 再起一次 `git -C <dest> rev-parse HEAD`
//	    · py `:240` `run_probe()` 起**探针本体**（P1 起 `go`；P2 起 `sys.executable` 跑 py 件）
//	    · py `:359-362` `self_test()` 起 `git init` / `git commit` / `git rev-parse` 造**合成夹具**
//	  ⇒ 本件逐点照搬同一子进程面（Go `os/exec`），**cwd 与 argv 逐字同构**。
//	★ **写面：默认档零写面**；唯一写面是 `--sha-file <件>`（显式旗标 · 落一份 JSON），
//	  与 py `:492` 同点同形。临时树的落点在 `<tempdir>`（py `:459` `tempfile.mkdtemp`；`--dest` 可覆盖）。
//	★ **外部真源真身（本件输出序/判据的决定者，逐字）**：
//	    · 公开树来源真源 = `DEFAULT_REPO` = `https://github.com/Mr2109/zerg-swarm.git`（py `:90`）；
//	      本机**没有任何在册的镜像树落点**（不扫本仓、不扫 `PUB_T`）⇒ 缺省档真跑 = **真 clone 联网**。
//	    · `--tree <目录>` 是**复用已物料化公开树**的旋钮（py `:416`）⇒ 离线/对拍一律走它。
//	    · `REPO_ROOT`（本机侧根）= py `:87-88` 由 `__file__` 上溯三层（`scripts/gates` → 仓根）——
//	      见下面「出厂声明 (a)」。
//	★ **输出序的决定者**：本件输出行序**逐点**由 ① `PROBES` 表的**静态顺序**（P1→P2 · py `:508` 串行 for）
//	  ＋ ② 每行内部各段是**常量模板拼接**（无集合、无 map、**零 Go map 迭代进输出**）
//	  ＋ ③ `dedupe()`（py `:217`）的**首次出现序**（`order` 列表保序、`seen` 只当计数桶）决定
//	  ⇒ **没有任何一处依赖目录遍历序或 map 迭代序**。
//
// ═══════════════════════ 出厂声明 (a)：默认根定位差异（✗ 不声称逐字对齐）═══════════════════════
//
//	py 有 `__file__` ⇒ `REPO_ROOT = dirname(dirname(dirname(abspath(__file__))))`（脚本位置**上溯三层**）。
//	Go 无 `__file__` 语境 ⇒ 本件用 **cwd 上溯**（找第一个含 `scripts/gates/` 的祖先目录；找不到回落 cwd），
//	并给显式旋钮 `ZERG_PST_REPO_ROOT`。⇒ 「脚本位置 ≠ cwd」时两态该面**不同值**（✗ 不保证逐字对齐）；
//	对拍与日常一律显式传 `--local-head`（py 自带该旋钮）绕开这一面。
//
// ═══════════════════════ 出厂声明 (b)：自指检查 —— 本件**不落在自己扫描的面内** ═══════════════════════
//
//	本件扫描面 = **仓外**的公开树（缺省 `git clone` 到 `<tempdir>`；或 `--tree <仓外目录>`）
//	⟷ 它**不读本仓任何件**（零 `git ls-files` / 零 `git grep` / 零本仓路径拼接）⇒ **自指形态 ①③ 不适用**。
//	量化依据：全件对 `core/` `scripts/` 一类本仓相对路径的**读点 0 处**；唯二的路径字面是公开仓 URL
//	（`DEFAULT_REPO`，与 `core/cmd/zerg/check_public_face_wording.go:458` **逐字同串** ⇒ 已被该门当作
//	「被判对象」持有，非新形态）与探针内的 `core` / `ui/...`（相对 `--tree` 的**外部树内**路径）。
//	④ 「本件是否落在他门扫描面内」：`check-hardcoded-private-paths.py` 的三枚 needle =
//	  `<volume-path>` · `<volume-path>` · `~`（其 `SCAN_DIRS` 含 `core`）——
//	  本件把私有路径原串一律写成**拼串形态**（`/Users/` + 尾串），落仓后实测该三枚 needle 在件内
//	  **命中 0**（✗ 不是「碰不到」）。`check-error-kinds.py` 的 R3 只扫 `setErr("…` / `kind="…`
//	  两种字面 —— 本件**零命中**（全件无 `setErr(` / 无 `kind=`）。
//	  `check-public-face-wording.py` 的扫描面是 `ROOT_FACE` 具名件 + `docs/` walk —— 与 `core/` 不交。
//
// ═══════════════════════ 对拍注入旋钮（✗ 不靠掩码 ⇒ 给确定性旋钮）═══════════════════════
//
//	py 输出里有**两处时刻字段**（`%.1fs` 耗时 · `--sha-file` 的 `when`）＋ `self_test()` 会造
//	`git commit`（提交 sha 随提交时刻漂）。本件的确定化法 = **两侧同一注入**：
//
//	  · env `ZERG_PST_FROZEN=1`（本件）⟷ 仓外 `sitecustomize.py` 把 `time.time` 钉成常量（py 侧）
//	    ⇒ 两态的 `%.1fs` 一律打印 `0.0`（**注入确定性，不是掩码**）；
//	  · env `ZERG_PST_WHEN=<串>`（本件）⟷ 同法钉 `time.strftime`（py 侧）⇒ `--sha-file` 的 `when` 两侧同值；
//	  · `GIT_AUTHOR_DATE` / `GIT_COMMITTER_DATE` 由**环境**注入 ⇒ `self_test()` 的合成提交 sha 两侧**同值**。
//
//	⚠ 这三个 env 旋钮是**本件单方**的（py 原件不读它们）⇒ 属**声明口径差**：默认档（不设旋钮）两态
//	  在「耗时秒数」与「合成提交 sha」两处**天然不同**（已实测 py 自身连跑两发也不同 ⇒ 非本件缺陷）。
//
// 入口签名（与 py `main()` 对齐）：
//
//	func checkPublicTreeSelfcheckGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 全绿 / 1 有命中 / 2 不给结论（clone 失败 · 缺件 · 空转 · 自检不过 · 未证）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「对拍」节 · 未覆盖面逐条另列）
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// ── 常量（py `:90-91`）────────────────────────────────────────────────────────────
const cptsDefaultRepo = "https://github.com/Mr2109/zerg-swarm.git"
const cptsDefaultTimeout = 900

// py `:178-180` 三值（唯一出口）
const cptsCorrespond = "对应"
const cptsUnrespond = "不对应"
const cptsUnproven = "未证"

// py `:388`：本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）
var cptsJSONFields = []string{"schema", "tree", "head", "local_head", "correspondence", "correspondence_why", "note", "rows"}

// ═══════════════════════ py 语义原语（✗ 不用 Go 同名标准库 · 逐个自写同构）═══════════════════════

// cptsWSClass = py `\s` 的**码点展开**（CPython Unicode 空白表：含 `\x1c-\x1f` 与 `\x85`；
// Go RE2 的 `\s` 是 ASCII 语义 ⇒ ✗ 直接用）。✗ 不含 `\u180e`（3.9 已不是空白）。
const cptsWSClass = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`

func cptsIsPySpace(r rune) bool {
	switch r {
	case 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x1C, 0x1D, 0x1E, 0x1F, 0x20, 0x85, 0xA0,
		0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// cptsPyStrip = py `str.strip()`（**Unicode 空白** · 含 `\x1c-\x1f`/`\x85`/`\xa0`）⟷ ✗ `strings.TrimSpace`。
func cptsPyStrip(s string) string {
	runes := []rune(s)
	i, j := 0, len(runes)
	for i < j && cptsIsPySpace(runes[i]) {
		i++
	}
	for j > i && cptsIsPySpace(runes[j-1]) {
		j--
	}
	return string(runes[i:j])
}

// cptsSplitLines = py `str.splitlines()` 的分隔符集（`\n` `\r` `\r\n` `\v` `\f` `\x1c-\x1e` `\x85` `\u2028` `\u2029`）。
func cptsSplitLines(s string) []string {
	out := []string{}
	var cur strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); {
		r := runes[i]
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			out = append(out, cur.String())
			cur.Reset()
			i++
		case '\r':
			if i+1 < len(runes) && runes[i+1] == '\n' {
				i++
			}
			out = append(out, cur.String())
			cur.Reset()
			i++
		default:
			cur.WriteRune(r)
			i++
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// cptsTrunc = py 切片 `s[:n]`（**按码点**，✗ 按 byte）。
func cptsTrunc(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// cptsPyJoin = CPython `posixpath.join` 逐段规则（py `:235` `:237` 用到）。
func cptsPyJoin(parts ...string) string {
	path := ""
	if len(parts) > 0 {
		path = parts[0]
	}
	for _, b := range parts[1:] {
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

// cptsPyNormpath = CPython `posixpath.normpath`（含「恰两个前导斜杠保留」那条 · py `:237` 用到）。
func cptsPyNormpath(path string) string {
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
	newComps := []string{}
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initialSlashes == 0 && len(newComps) == 0) ||
			(len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	out := strings.Join(newComps, "/")
	if initialSlashes > 0 {
		out = strings.Repeat("/", initialSlashes) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// cptsPyDirname = CPython `posixpath.split()[0]`（剥尾斜杠 · 与 `filepath.Dir` 不同）。
func cptsPyDirname(p string) string {
	i := strings.LastIndexByte(p, '/') + 1
	head := p[:i]
	if head != "" && strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// cptsPyRepr = py `repr(str)`（自检面 `%r` 用 · 单/双引号择一 · 控制符转义 · 非 ASCII **不转义**）。
func cptsPyRepr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
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
			if r == rune(quote) {
				b.WriteByte('\\')
				b.WriteRune(r)
			} else if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, "\\x%02x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// cptsPyJSONStr = py `json.dumps(s, ensure_ascii=False)` 的字符串编码（✗ Go `encoding/json`
// 的 `\u003c` HTML 转义与 `ensure_ascii=True` 默认都不同）。
func cptsPyJSONStr(s string) string {
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

// cptsWhich = `shutil.which`（py `:185` `:232` `:251`）。
func cptsWhich(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}

// cptsPyExists = `os.path.exists`（**stat 跟随符号链接** · 任何 OSError ⇒ false）。
func cptsPyExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// cptsPythonBin —— P2 探针 argv[0] 对应 py `:112` 的 `sys.executable or "python3"`。
// ★ py 有 `sys.executable`（**跑本件的解释器绝对路径**）、Go 无 ⟹ 这是**声明口径差**：
//
//	缺省 `python3`（照 py 的回落串）；env `ZERG_PST_PYTHON` 可显式指定（对拍时两侧同钉 `/usr/bin/python3`）。
func cptsPythonBin() string {
	if v := os.Getenv("ZERG_PST_PYTHON"); v != "" {
		return v
	}
	return "python3"
}

// ── 注入旋钮（见件头「对拍注入旋钮」节）────────────────────────────────────────────
func cptsFrozen() bool { return os.Getenv("ZERG_PST_FROZEN") != "" }

func cptsNow() time.Time {
	if cptsFrozen() {
		return time.Unix(0, 0)
	}
	return time.Now()
}

func cptsWhen() string {
	if v := os.Getenv("ZERG_PST_WHEN"); v != "" {
		return v
	}
	return time.Now().Format("2006-01-02T15:04:05-0700")
}

// cptsRepoRoot —— 出厂声明 (a)：py 由 `__file__` 上溯三层 ⟷ Go 由 cwd 上溯找 `scripts/gates`。
func cptsRepoRoot() string {
	if v := os.Getenv("ZERG_PST_REPO_ROOT"); v != "" {
		return v
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	d := cwd
	for {
		if cptsPyExists(cptsPyJoin(d, "scripts", "gates")) {
			return d
		}
		parent := cptsPyDirname(d)
		if parent == d || parent == "" {
			break
		}
		d = parent
	}
	return cwd
}

// ═══════════════════════ 探针表（py `:94-119` · 判据写在一处）═══════════════════════

type cptsProbe struct {
	id        string
	name      string
	cwd       string
	argv      []string
	needsArgv []string
	needsTree []string
	marks     []string
	failLines []string
	why       string
}

func cptsProbes() []cptsProbe {
	return []cptsProbe{
		{
			id:        "P1",
			name:      "redact 组（脱敏必要条件门 ↔ 公开面替换）",
			cwd:       "core",
			argv:      []string{"go", "test", "./internal/obs/redact/", "-count=1"},
			needsArgv: []string{"go"},
			needsTree: []string{"core"},
			marks:     []string{"LEAK:", "夹具失效"},
			failLines: []string{"--- FAIL", "FAIL\t"},
			why: "①②两类红都在这里：替换把 `/Users/" + "Mr2109` 改成 `~/` ⇒ 必要条件门只认 /Users/、/home/ ⇒ LEAK；" +
				"`Mr2109`→`Mr2109` 多 2 字节 ⇒ 夹具长度假设被打穿 ⇒ 夹具失效。",
		},
		{
			id:        "P2",
			name:      "ui i18n 门禁（公开面文案）",
			cwd:       ".",
			argv:      []string{cptsPythonBin(), "ui/scripts/check-i18n.py"},
			needsArgv: []string{},
			needsTree: []string{"ui/scripts/check-i18n.py", "ui/locales", "ui/src"},
			marks:     []string{"[ERROR]"},
			failLines: []string{},
			why:       "公开面文案铁律的机械尺；本次 run 唯一的「真红」（G4 两条中文字面量）就是它抓到的。",
		},
	}
}

func cptsProbeByID(pid string) cptsProbe {
	for _, p := range cptsProbes() {
		if p.id == pid {
			return p
		}
	}
	return cptsProbe{}
}

// ── 自检夹具（py `:122-150`）────────────────────────────────────────────────────
const cptsMF1Text = "--- FAIL: TestFailClosed_OverBudget (0.00s)\n" +
	"    failclosed_test.go:71: LEAK: 秘密 \"~/projects/zerg/vault.key\" 以投影 \"~/projects/zerg/vault.key\" " +
	"残留在 {\"err\":\"open ~/projects/zerg/vault.key failed\"}\n"

const cptsMF2Text = "--- FAIL: TestGateReverseControlOnEscapeArtifact (0.00s)\n" +
	"    limits_test.go:464: 夹具失效：blob=\"cGVlciBNcjIxMDkgcnVuIGhlcmUgb2s=\"（len=32）\n"

const cptsMF3Text = "=== i18n 门禁 ===\n" +
	"  键数 zh=570 en=570 ｜ 静态引用 411 ｜ 剩余中文字面量 11 行（基线 9）\n" +
	"  [ERROR] G4 出现未登记的中文 UI 字面量（2 行，须抽成 i18n 键或登记进基线）：\n" +
	"  [ERROR]       ui/src/app.rs:1829 main_view | } else if trust == \"正式\" {\n" +
	"→ 门禁失败（3 项）\n"

const cptsPC1Text = "ok  \tgithub.com/Mr2109/zerg-swarm/core/internal/obs/redact\t0.571s\n"

const cptsPC2Text = "=== i18n 门禁 ===\n  键数 zh=570 en=570 ｜ 静态引用 411 ｜ 剩余中文字面量 9 行（基线 9）\n→ 门禁通过\n"

type cptsSelfCase struct {
	name string
	pid  string
	text string
	rc   int
	want string
}

func cptsSelfTestCases() []cptsSelfCase {
	return []cptsSelfCase{
		{"MF1 替换打穿夹具（redact 必要条件门只认 /Users/、/home/ ⇒ ~/ 形态整段跳过）", "P1", cptsMF1Text, 1, "hit"},
		{"MF2 夹具长度漂（Mr2109→Mr2109 多 2 字节 ⇒ base64 带 = 打穿 len<24 守卫）", "P1", cptsMF2Text, 1, "hit"},
		{"PC1 正控：干净输出", "P1", cptsPC1Text, 0, "green"},
		{"VAC1 空转：零输出 + rc=0（**不许当绿**）", "P1", "", 0, "vacuous"},
		{"MF3 i18n 真红（今日 run 原样）", "P2", cptsMF3Text, 1, "hit"},
		{"PC2 正控：i18n 通过输出", "P2", cptsPC2Text, 0, "green"},
		{"VAC2 空转：零输出 + rc=0", "P2", "", 0, "vacuous"},
	}
}

// ═══════════════════════ 判决面（py `:161-173` · 唯一判据出口）═══════════════════════

func cptsJudge(text string, rc int, p cptsProbe) (string, []string, []string) {
	lines := cptsSplitLines(text)
	named := []string{}
	fails := []string{}
	for _, ln := range lines {
		if cptsHasAny(ln, p.marks) {
			named = append(named, ln)
		}
	}
	for _, ln := range lines {
		if cptsHasAny(ln, p.failLines) {
			fails = append(fails, ln)
		}
	}
	if cptsPyStrip(text) == "" {
		return "vacuous", named, fails
	}
	if len(named) > 0 || len(fails) > 0 || rc != 0 {
		return "hit", named, fails
	}
	return "green", named, fails
}

func cptsHasAny(s string, subs []string) bool {
	for _, m := range subs {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// ── 双方 HEAD sha（py `:200-214` · 唯一出口）────────────────────────────────────
func cptsJudgeHeads(local, public string) (string, string) {
	local, public = cptsPyStrip(local), cptsPyStrip(public)
	if local == "" || public == "" {
		lack := "本机 HEAD 读不到"
		if local != "" {
			lack = "公开树 HEAD 读不到"
		}
		return cptsUnproven, lack + " ⇒ 两枚 sha 不齐，**给不出「对应/不对应」**（缺件 ≠ 绿）"
	}
	if local == public {
		return cptsCorrespond, "双方 HEAD sha 逐字相同"
	}
	return cptsUnrespond, fmt.Sprintf("本机 HEAD %s ≠ 公开树 HEAD %s —— clone 的是**当前公开树**，不是本次要比的快照",
		cptsTrunc(local, 12), cptsTrunc(public, 12))
}

// ── dedupe（py `:217-227`）─────────────────────────────────────────────────────
// 正则逐字：py `r"\s*([^\s:]+:\d+):"` ⇒ ① `\s` 展开为 cptsWSClass（含 `\x1c-\x1f`）
// ② `\d` 是 **Unicode Nd**（✗ 不是 ASCII `[0-9]`）⇒ 展开为 `\p{Nd}`。
var cptsDedupeRe = regexp.MustCompile(`^[` + cptsWSClass + `]*([^` + cptsWSClass + `:]+:\p{Nd}+):`)

type cptsDedupeEnt struct {
	count int
	text  string
}

func cptsDedupe(lines []string, capN int) ([]cptsDedupeEnt, int) {
	order := []string{}
	seen := map[string]*cptsDedupeEnt{}
	for _, ln := range lines {
		key := ""
		if m := cptsDedupeRe.FindStringSubmatch(ln); m != nil {
			key = m[1]
		} else {
			key = cptsTrunc(cptsPyStrip(ln), 80)
		}
		if _, ok := seen[key]; !ok {
			seen[key] = &cptsDedupeEnt{0, cptsTrunc(cptsPyStrip(ln), 200)}
			order = append(order, key)
		}
		seen[key].count++
	}
	out := []cptsDedupeEnt{}
	for i, k := range order {
		if i >= capN {
			break
		}
		out = append(out, *seen[k])
	}
	return out, len(order)
}

// ── read_head（py `:183-197`）─────────────────────────────────────────────────
func cptsReadHead(tree string, full bool) string {
	if tree == "" || cptsWhich("git") == "" {
		return ""
	}
	argv := []string{"git", "-C", tree, "rev-parse"}
	if !full {
		argv = append(argv, "--short")
	}
	argv = append(argv, "HEAD")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var ob, eb bytes.Buffer
	cmd.Stdout = &ob
	cmd.Stderr = &eb
	if err := cmd.Run(); err != nil {
		return ""
	}
	return cptsPyStrip(ob.String())
}

// ── run_probe（py `:229-246`）─────────────────────────────────────────────────
func cptsRunProbe(tree string, p cptsProbe, timeout int) (*int, string, float64, string) {
	for _, a := range p.needsArgv {
		if cptsWhich(a) == "" {
			return nil, "", 0.0, "工具不在 PATH：" + a
		}
	}
	for _, rel := range p.needsTree {
		if !cptsPyExists(cptsPyJoin(tree, rel)) {
			return nil, "", 0.0, "树里缺件：" + rel
		}
	}
	cwd := cptsPyNormpath(cptsPyJoin(tree, p.cwd))
	t0 := cptsNow()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.argv[0], p.argv[1:]...)
	cmd.Dir = cwd
	var ob, eb bytes.Buffer
	cmd.Stdout = &ob
	cmd.Stderr = &eb
	err := cmd.Run()
	secs := cptsNow().Sub(t0).Seconds()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, "", secs, fmt.Sprintf("超时（>%ds）", timeout)
	}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			rc := ee.ExitCode()
			// py `subprocess` 对被信号终止的子件给**负信号号**（✗ 不是 -1）
			if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				rc = -int(ws.Signal())
			}
			return &rc, ob.String() + eb.String(), secs, ""
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, "", secs, fmt.Sprintf("超时（>%ds）", timeout)
		}
		return nil, "", secs, "起不来：" + err.Error()
	}
	rc := 0
	return &rc, ob.String() + eb.String(), secs, ""
}

// ── materialize（py `:250-270` · **唯一联网点**）────────────────────────────────
func cptsMaterialize(repo, dest string, timeout int) (*string, string) {
	if cptsWhich("git") == "" {
		return nil, "git 不在 PATH ⇒ 给不出公开树"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-q", repo, dest)
	var ob, eb bytes.Buffer
	cmd.Stdout = &ob
	cmd.Stderr = &eb
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Sprintf("clone 超时（>%ds）：%s", timeout, repo)
	}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Sprintf("clone 失败（rc=%d）：%s", ee.ExitCode(), cptsTrunc(cptsPyStrip(eb.String()), 300))
		}
		return nil, "clone 起不来：" + err.Error()
	}
	head := ""
	if h := cptsReadHead(dest, true); h != "" {
		head = h
	}
	hs := head
	return &hs, ""
}

// ── 自检（py `:274-381` · 成对负控 · 含真执行面/取数面不许哑化）────────────────────
func cptsSelfTest(stdout io.Writer) int {
	ok := 0
	bad := []string{}
	for _, c := range cptsSelfTestCases() {
		probe := cptsProbeByID(c.pid)
		got, named, fails := cptsJudge(c.text, c.rc, probe)
		if got == c.want {
			ok++
			fmt.Fprintf(stdout, "   ✓ %s ⇒ %s\n", c.name, got)
		} else {
			bad = append(bad, c.name)
			fmt.Fprintf(stdout, "   ✗ %s ⇒ 期望 %s，实得 %s（具名红因 %d 条 · FAIL 行 %d 条）\n",
				c.name, c.want, got, len(named), len(fails))
		}
	}
	// 区分度：同一段文本在 must-fail 用例里必须红、在正控里必须绿（否则判据没牙）
	mf1, _, _ := cptsJudge(cptsMF1Text, 1, cptsProbeByID("P1"))
	pc1, _, _ := cptsJudge(cptsPC1Text, 0, cptsProbeByID("P1"))
	if mf1 == "hit" && pc1 == "green" {
		ok++
		fmt.Fprint(stdout, "   ✓ 区分度：同一探针在 MF1 ⇒ hit、在 PC1 ⇒ green（判据有牙，不是恒绿/恒红）\n")
	} else {
		bad = append(bad, "区分度")
		fmt.Fprintf(stdout, "   ✗ 区分度：MF1 ⇒ %s、PC1 ⇒ %s（判据没有证明自己有区分能力）\n", mf1, pc1)
	}
	// 具名红因必须与探针绑定
	if cptsHasAny("夹具失效", cptsProbeByID("P1").marks) && !cptsHasAny("夹具失效", cptsProbeByID("P2").marks) {
		ok++
		fmt.Fprint(stdout, "   ✓ 具名红因与探针绑定：『夹具失效』只在 P1 的 marks 里（换到 P2 不命中）\n")
	} else {
		bad = append(bad, "具名红因绑定")
	}
	// 两类红的 must-fail 各自点名
	kindsAllHit := true
	for _, c := range cptsSelfTestCases()[:2] {
		v, _, _ := cptsJudge(c.text, c.rc, cptsProbeByID(c.pid))
		if v != "hit" {
			kindsAllHit = false
		}
	}
	if kindsAllHit {
		ok++
		fmt.Fprint(stdout, "   ✓ must-fail 两类分列：替换打穿夹具 / 夹具长度漂 各自被点名（都不是靠 rc 蹭红）\n")
	} else {
		bad = append(bad, "两类 must-fail 分列")
	}
	// 双方 HEAD sha 判据：正控 / 负控 / 缺一侧
	a40 := strings.Repeat("a", 40)
	b40 := strings.Repeat("b", 40)
	vPos, _ := cptsJudgeHeads(a40, a40)
	vNeg, vNegWhy := cptsJudgeHeads(a40, b40)
	vLack, vLackWhy := cptsJudgeHeads("", b40)
	if vPos == cptsCorrespond {
		ok++
		fmt.Fprint(stdout, "   ✓ 正控：双方同 sha ⇒ **对应**（本机 aaaa… = 公开树 aaaa…）\n")
	} else {
		bad = append(bad, "双方 HEAD sha 正控（同 sha）")
		fmt.Fprintf(stdout, "   ✗ 正控：同 sha 却判成 %s\n", vPos)
	}
	if vNeg == cptsUnrespond {
		ok++
		fmt.Fprintf(stdout, "   ✓ 负控：**喂一个假 HEAD**（公开树 bbbb… ≠ 本机 aaaa…）⇒ **不对应**（%s）\n", vNegWhy)
	} else {
		bad = append(bad, "双方 HEAD sha 负控（假 HEAD）")
		fmt.Fprintf(stdout, "   ✗ 负控：假 HEAD 却判成 %s —— 「不对应」没进判决（判据没牙）\n", vNeg)
	}
	if vLack == cptsUnproven {
		ok++
		fmt.Fprintf(stdout, "   ✓ 缺一侧 sha（本机读不到）⇒ **未证**（不给结论，不许当绿）：%s\n", vLackWhy)
	} else {
		bad = append(bad, "双方 HEAD sha 缺一侧 ⇒ 未证")
	}
	// ★ 回归自测格：真探针执行面 `run_probe()` / 取数面 `read_head()` 不许哑化
	sxRoot, mkErr := os.MkdirTemp("", "selfcheck-selftest-")
	if mkErr != nil {
		sxRoot = ""
	}
	if sxRoot != "" {
		defer os.RemoveAll(sxRoot)
	}
	sxProbe := cptsProbe{
		id:        "SX",
		name:      "自检受控探针",
		cwd:       ".",
		argv:      []string{cptsPythonBin(), "-c", "print('LEAK: selftest-synthetic')"},
		needsArgv: []string{},
		needsTree: []string{},
		marks:     []string{"LEAK:"},
		failLines: []string{"--- FAIL"},
		why:       "自检受控探针（合成输出 · 不碰真目标/不走网）",
	}
	type sxCase struct {
		ok   bool
		why  string
		gone bool
	}
	sxCases := []sxCase{}
	if sxRoot == "" {
		sxCases = append(sxCases, sxCase{false, "真执行面 `run_probe()` 哑化/异常（MkdirTempError: " + mkErr.Error() + "）⇒ 不许静默绿", true})
	} else {
		rc, text, _, _ := cptsRunProbe(sxRoot, sxProbe, 60)
		rcRepr := "None"
		rcOK := false
		if rc != nil {
			rcRepr = fmt.Sprintf("%d", *rc)
			rcOK = *rc == 0
		}
		v, _, _ := cptsJudge(text, cptsRCOr(rc, 1), sxProbe)
		sxCases = append(sxCases,
			sxCase{rcOK, fmt.Sprintf("run_probe 真跑合成命令 ⇒ rc=0（现读 %s）", rcRepr), false},
			sxCase{strings.Contains(text, "LEAK:"), fmt.Sprintf("run_probe 真回传子进程输出（现读 %s）", cptsPyRepr(cptsTrunc(text, 80))), false},
			sxCase{v == "hit", fmt.Sprintf("判决面对**真执行面**输出真判 `hit`（现读 %s）", v), false},
		)
	}
	// 取数面：真 git 夹具
	sxEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
	sxOK := true
	sxWhy := ""
	sxGot := ""
	if sxRoot == "" {
		sxOK = false
		sxWhy = "取数面 `read_head()` 哑化/异常（MkdirTempError: " + mkErr.Error() + "）⇒ 不许静默绿"
	} else {
		initCmd := exec.Command("git", "init", "-q", "-b", "main", sxRoot)
		initCmd.Env = sxEnv
		if err := initCmd.Run(); err != nil {
			sxOK = false
			sxWhy = "取数面 `read_head()` 哑化/异常（" + cptsErrName(err) + ": " + err.Error() + "）⇒ 不许静默绿"
		} else {
			cmtCmd := exec.Command("git", "-C", sxRoot, "commit", "-q", "--allow-empty", "-m", "fx")
			cmtCmd.Env = sxEnv
			if err := cmtCmd.Run(); err != nil {
				sxOK = false
				sxWhy = "取数面 `read_head()` 哑化/异常（" + cptsErrName(err) + ": " + err.Error() + "）⇒ 不许静默绿"
			} else {
				revCmd := exec.Command("git", "-C", sxRoot, "rev-parse", "HEAD")
				revCmd.Env = sxEnv
				var rb bytes.Buffer
				revCmd.Stdout = &rb
				_ = revCmd.Run()
				sxWant := cptsPyStrip(rb.String())
				sxGot = cptsReadHead(sxRoot, true)
				sxOK = sxGot == sxWant && utf8.RuneCountInString(sxGot) == 40
				sxWhy = fmt.Sprintf("read_head 真读出 40 位 HEAD（现读 %s）", cptsPyRepr(sxGot))
			}
		}
	}
	sxCases = append(sxCases, sxCase{sxOK, sxWhy, true})
	for _, c := range sxCases {
		if c.ok {
			ok++
			fmt.Fprintf(stdout, "   ✓ %-32s | %s\n", "runprobe/readhead-truly-live", c.why)
		} else {
			bad = append(bad, "runprobe/readhead-truly-live")
			fmt.Fprintf(stdout, "   ✗ %-32s | %s\n", "runprobe/readhead-truly-live", c.why)
		}
	}
	concl := "全过"
	if len(bad) > 0 {
		concl = "**不过**"
	}
	fmt.Fprintf(stdout, "自检结论: %s（断言 %d 条 · 失败 %d 条）\n", concl, ok+len(bad), len(bad))
	if len(bad) > 0 {
		return 2
	}
	return 0
}

func cptsRCOr(rc *int, d int) int {
	if rc == nil {
		return d
	}
	return *rc
}

func cptsErrName(err error) string {
	if err == nil {
		return ""
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return "CalledProcessError"
	}
	var e2 *exec.Error
	if errors.As(err, &e2) {
		return "OSError"
	}
	return fmt.Sprintf("%T", err)
}

// ═══════════════════════ argparse 面（usage / 帮助 / 报错 · 逐字同构）═══════════════════════

// cptsProg —— py 的 `prog` = `os.path.basename(sys.argv[0])` = 本判据件名。
// ★ Go 无 `sys.argv[0]` ⇒ 硬钉该串（**声明口径差**：只有「用别的文件名调 py 原件」才会不同）。
const cptsProg = "check-public-tree-selfcheck.py"

type cptsAction struct {
	usageInv   string
	helpInv    string
	longName   string
	shortName  string
	takesValue bool
	help       string
}

func cptsActions() []cptsAction {
	return []cptsAction{
		{usageInv: "[-h]", helpInv: "-h, --help", longName: "--help", shortName: "-h", takesValue: false,
			help: "show this help message and exit"},
		{usageInv: "[--repo REPO]", helpInv: "--repo REPO", longName: "--repo", takesValue: true,
			help: "公开树来源（默认 " + cptsDefaultRepo + "）"},
		{usageInv: "[--tree TREE]", helpInv: "--tree TREE", longName: "--tree", takesValue: true,
			help: "复用已物料化的公开树（给了就不 clone）"},
		{usageInv: "[--dest DEST]", helpInv: "--dest DEST", longName: "--dest", takesValue: true,
			help: "clone 落点（默认 mktemp）"},
		{usageInv: "[--keep]", helpInv: "--keep", longName: "--keep", takesValue: false,
			help: "留下临时树（取证用；默认删）"},
		{usageInv: "[--timeout TIMEOUT]", helpInv: "--timeout TIMEOUT", longName: "--timeout", takesValue: true,
			help: "每个探针/整次 clone 的超时秒数"},
		{usageInv: "[--max-hits MAX_HITS]", helpInv: "--max-hits MAX_HITS", longName: "--max-hits", takesValue: true,
			help: "具名红因最多列几组（去重后 · 默认 12）"},
		{usageInv: "[--max-fail-lines MAX_FAIL_LINES]", helpInv: "--max-fail-lines MAX_FAIL_LINES",
			longName: "--max-fail-lines", takesValue: true, help: "FAIL 行最多列几组（去重后 · 默认 8）"},
		{usageInv: "[--json JSON]", helpInv: "--json JSON", longName: "--json", takesValue: true,
			help: "机器面字段（逗号分隔）"},
		{usageInv: "[--self-test]", helpInv: "--self-test", longName: "--self-test", takesValue: false,
			help: "只跑自检（合成夹具 · 不碰真目标 · 不走网）"},
		{usageInv: "[--local-head LOCAL_HEAD]", helpInv: "--local-head LOCAL_HEAD", longName: "--local-head", takesValue: true,
			help: "显式给本机 HEAD sha（缺省现读 `git -C <仓根> rev-parse HEAD`）"},
		{usageInv: "[--public-head PUBLIC_HEAD]", helpInv: "--public-head PUBLIC_HEAD", longName: "--public-head", takesValue: true,
			help: "显式给公开树 HEAD sha（缺省现读物料化那棵树的 HEAD）"},
		{usageInv: "[--shas-only]", helpInv: "--shas-only", longName: "--shas-only", takesValue: false,
			help: "**只**核双方 HEAD sha（不跑探针）"},
		{usageInv: "[--sha-file SHA_FILE]", helpInv: "--sha-file SHA_FILE", longName: "--sha-file", takesValue: true,
			help: "把双方 sha + 对应判据落成一份 JSON（记录用 · 缺省只打印）"},
		{usageInv: "[--list-rules]", helpInv: "--list-rules", longName: "--list-rules", takesValue: false,
			help: "打判据 / 探针 / 档位"},
	}
}

// cptsTermWidth = `shutil.get_terminal_size().columns - 2`（非 tty ⇒ COLUMNS 或 80 回落）。
func cptsTermWidth() int {
	cols := 80
	if v := os.Getenv("COLUMNS"); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			cols = n
		}
	}
	return cols - 2
}

func parsePositiveInt(s string) (int, error) {
	n := 0
	if s == "" {
		return 0, errors.New("empty")
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, errors.New("nan")
		}
		n = n*10 + int(s[i]-'0')
	}
	if n <= 0 {
		return 0, errors.New("nonpositive")
	}
	return n, nil
}

// cptsWrapUsage 逐字复刻 CPython argparse `_format_usage` 的折行（prog 短 ⇒ 与 optionals 同行）。
func cptsWrapUsage() string {
	prefix := "usage: "
	width := cptsTermWidth()
	acts := cptsActions()
	parts := []string{}
	for _, a := range acts {
		parts = append(parts, a.usageInv)
	}
	optUsage := strings.Join(parts, " ")
	usage := cptsProg + " " + optUsage
	if len(prefix)+len(usage) <= width {
		return prefix + usage + "\n\n"
	}
	textWidth := width
	indent := strings.Repeat(" ", len(prefix)+len(cptsProg)+1)
	lines := cptsGetLines(append([]string{cptsProg}, parts...), indent, prefix, textWidth)
	return prefix + strings.Join(lines, "\n") + "\n\n"
}

// cptsGetLines = CPython `_format_usage` 内嵌的 `get_lines()`。
func cptsGetLines(parts []string, indent string, prefix string, textWidth int) []string {
	lines := []string{}
	line := []string{}
	lineLen := 0
	if prefix != "" {
		lineLen = len(prefix) - 1
	} else {
		lineLen = len(indent) - 1
	}
	for _, part := range parts {
		if lineLen+1+len(part) > textWidth && len(line) > 0 {
			lines = append(lines, indent+strings.Join(line, " "))
			line = []string{}
			lineLen = len(indent) - 1
		}
		line = append(line, part)
		lineLen += 1 + len(part)
	}
	if len(line) > 0 {
		lines = append(lines, indent+strings.Join(line, " "))
	}
	if prefix != "" && len(lines) > 0 {
		lines[0] = lines[0][len(indent):]
	}
	return lines
}

// cptsWrapHelp = formatter `_split_lines`：空白串归一 + 折到 width（本件的帮助串全 ≤ width ⇒ 单行）。
func cptsWrapHelp(text string, width int) []string {
	norm := strings.Join(strings.FieldsFunc(text, cptsIsPySpace), " ")
	norm = cptsPyStrip(norm)
	if utf8.RuneCountInString(norm) <= width {
		return []string{norm}
	}
	words := strings.Split(norm, " ")
	lines := []string{}
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
			continue
		}
		if utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(w) <= width {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// cptsHelpSections 生成帮助正文（usage 段 + optional arguments 组）。
func cptsHelpSections() (string, string) {
	width := cptsTermWidth()
	acts := cptsActions()
	maxInv := 0
	for _, a := range acts {
		if n := utf8.RuneCountInString(a.helpInv); n > maxInv {
			maxInv = n
		}
	}
	helpPosition := maxInv + 2
	if helpPosition > 24 {
		helpPosition = 24
	}
	actionWidth := helpPosition - 2 - 2
	helpWidth := width - helpPosition
	if helpWidth < 11 {
		helpWidth = 11
	}
	var sb strings.Builder
	for _, a := range acts {
		invLen := utf8.RuneCountInString(a.helpInv)
		helpLines := cptsWrapHelp(a.help, helpWidth)
		if invLen <= actionWidth {
			sb.WriteString("  " + a.helpInv + strings.Repeat(" ", actionWidth-invLen) + "  ")
			sb.WriteString(helpLines[0] + "\n")
			for _, h := range helpLines[1:] {
				sb.WriteString(strings.Repeat(" ", helpPosition) + h + "\n")
			}
		} else {
			sb.WriteString("  " + a.helpInv + "\n")
			sb.WriteString(strings.Repeat(" ", helpPosition) + helpLines[0] + "\n")
			for _, h := range helpLines[1:] {
				sb.WriteString(strings.Repeat(" ", helpPosition) + h + "\n")
			}
		}
	}
	return cptsWrapUsage(), sb.String()
}

// cptsHelp = `formatter.format_help()`：长空行折叠 ⇒ 去首尾换行 + 单个尾换行。
func cptsHelp() string {
	usage, body := cptsHelpSections()
	raw := usage + "\noptional arguments:\n" + body + "\n"
	for strings.Contains(raw, "\n\n\n") {
		raw = strings.ReplaceAll(raw, "\n\n\n", "\n\n")
	}
	// `_long_break_matcher` = `re.compile(r'\n\n\n+')` ⇒ 三连以上折成两连（上一步已等价）。
	return strings.Trim(raw, "\n") + "\n"
}

// cptsUsageShort = `format_usage()`（错误面用 · 单个尾换行）。
func cptsUsageShort() string {
	return strings.Trim(cptsWrapUsage(), "\n") + "\n"
}

// ── 参数解析（py `:414-431` `argparse` 面）──────────────────────────────────────
type cptsArgs struct {
	repo         string
	tree         string
	dest         string
	keep         bool
	timeout      int
	maxHits      int
	maxFailLines int
	jsonFields   string
	selfTest     bool
	localHead    string
	publicHead   string
	shasOnly     bool
	shaFile      string
	listRules    bool
}

func cptsDefaults() *cptsArgs {
	return &cptsArgs{
		repo:         cptsDefaultRepo,
		timeout:      cptsDefaultTimeout,
		maxHits:      12,
		maxFailLines: 8,
	}
}

// cptsParse 返回 (args, rc, done)。done=true ⇒ 已经产出终态输出与退码（帮助 / 用法错）。
func cptsParse(stdout, stderr io.Writer, argv []string) (*cptsArgs, int, bool) {
	a := cptsDefaults()
	acts := cptsActions()
	positionals := []string{}
	i := 0
	cptsErr := func(msg string) (*cptsArgs, int, bool) {
		fmt.Fprint(stderr, cptsUsageShort())
		fmt.Fprintf(stderr, "%s: error: %s\n", cptsProg, msg)
		return nil, 2, true
	}
	takeValue := func(name, inline string, hasInline bool, i *int) (string, bool) {
		if hasInline {
			return inline, true
		}
		if *i < len(argv) {
			next := argv[*i]
			if !(strings.HasPrefix(next, "-") && next != "-" && !cptsLooksLikeNegativeNumber(next)) {
				*i++
				return next, true
			}
		}
		return "", false
	}
	for i < len(argv) {
		x := argv[i]
		i++
		if x == "--" {
			positionals = append(positionals, argv[i:]...)
			break
		}
		if strings.HasPrefix(x, "--") && len(x) > 2 {
			name := x[2:]
			inline := ""
			hasInline := false
			if k := strings.IndexByte(name, '='); k >= 0 {
				inline = name[k+1:]
				hasInline = true
				name = name[:k]
			}
			matched := ""
			for _, act := range acts {
				if act.longName == "--"+name {
					matched = act.longName
					break
				}
			}
			if matched == "" {
				cands := []string{}
				for _, act := range acts {
					if strings.HasPrefix(act.longName, "--"+name) {
						cands = append(cands, act.longName)
					}
				}
				if len(cands) == 1 {
					matched = cands[0]
				} else if len(cands) > 1 {
					return cptsErr("ambiguous option: --" + name + " could match " + strings.Join(cands, ", "))
				}
			}
			if matched == "" {
				return cptsErr("unrecognized arguments: " + x)
			}
			var act cptsAction
			for _, c := range acts {
				if c.longName == matched {
					act = c
				}
			}
			if act.longName == "--help" {
				fmt.Fprint(stdout, cptsHelp())
				return nil, 0, true
			}
			if !act.takesValue {
				if hasInline {
					return cptsErr("argument " + act.longName + ": ignored explicit argument '" + inline + "'")
				}
				cptsSetFlag(a, act.longName)
				continue
			}
			val, ok := takeValue(act.longName, inline, hasInline, &i)
			if !ok {
				return cptsErr("argument " + act.longName + ": expected one argument")
			}
			if r, ok2 := cptsSetValue(a, act.longName, val); !ok2 {
				return cptsErr(r)
			}
			continue
		}
		if strings.HasPrefix(x, "-") && x != "-" {
			if x == "-h" {
				fmt.Fprint(stdout, cptsHelp())
				return nil, 0, true
			}
			return cptsErr("unrecognized arguments: " + x)
		}
		positionals = append(positionals, x)
	}
	if len(positionals) > 0 {
		return cptsErr("unrecognized arguments: " + strings.Join(positionals, " "))
	}
	return a, 0, false
}

func cptsLooksLikeNegativeNumber(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if (s[i] < '0' || s[i] > '9') && s[i] != '.' {
			return false
		}
	}
	return true
}

func cptsSetFlag(a *cptsArgs, name string) {
	switch name {
	case "--keep":
		a.keep = true
	case "--self-test":
		a.selfTest = true
	case "--shas-only":
		a.shasOnly = true
	case "--list-rules":
		a.listRules = true
	}
}

// cptsSetValue 返回 (错误文案, 成功)。错误文案与 argparse 逐字同构。
func cptsSetValue(a *cptsArgs, name, val string) (string, bool) {
	atoi := func() (int, bool) {
		n, err := parseSignedInt(val)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	switch name {
	case "--repo":
		a.repo = val
	case "--tree":
		a.tree = val
	case "--dest":
		a.dest = val
	case "--timeout":
		n, ok := atoi()
		if !ok {
			return "argument --timeout: invalid int value: '" + val + "'", false
		}
		a.timeout = n
	case "--max-hits":
		n, ok := atoi()
		if !ok {
			return "argument --max-hits: invalid int value: '" + val + "'", false
		}
		a.maxHits = n
	case "--max-fail-lines":
		n, ok := atoi()
		if !ok {
			return "argument --max-fail-lines: invalid int value: '" + val + "'", false
		}
		a.maxFailLines = n
	case "--json":
		a.jsonFields = val
	case "--local-head":
		a.localHead = val
	case "--public-head":
		a.publicHead = val
	case "--sha-file":
		a.shaFile = val
	}
	return "", true
}

func parseSignedInt(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty")
	}
	neg := false
	i := 0
	if s[0] == '+' || s[0] == '-' {
		neg = s[0] == '-'
		i = 1
	}
	if i >= len(s) {
		return 0, errors.New("nan")
	}
	n := 0
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, errors.New("nan")
		}
		n = n*10 + int(s[i]-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}

// ── 缺口 `Q-176`：`--json <字段>` 里不认的名 ⇒ rc=2 + 印可选字段表（py `:391-407`）──────────
func cptsRefuseUnknownJSONFields(stderr io.Writer, argv []string) *int {
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
	fields := []string{}
	for _, f := range want {
		if cptsPyStrip(f) != "" {
			fields = append(fields, cptsPyStrip(f))
		}
	}
	bad := []string{}
	for _, f := range fields {
		if !cptsContains(cptsJSONFields, f) {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	quoted := func(xs []string) string {
		out := make([]string, 0, len(xs))
		for _, x := range xs {
			out = append(out, "`"+x+"`")
		}
		return strings.Join(out, " · ")
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", quoted(bad))
	fmt.Fprintf(stderr, "   可选字段表：%s\n", quoted(cptsJSONFields))
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开\n")
	rc := 2
	return &rc
}

func cptsContains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// ═══════════════════════ 主流程（py `:410-571`）═══════════════════════════════════

func checkPublicTreeSelfcheckGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc := cptsRefuseUnknownJSONFields(stderr, argv); rc != nil {
		return *rc
	}
	args, rc, done := cptsParse(stdout, stderr, argv)
	if done {
		return rc
	}

	if args.listRules {
		fmt.Fprint(stdout, "⑭ 公开树自洽检查（③+⑦ 合并）· 只报告档起步 · 不进 FAST_STEPS\n")
		fmt.Fprint(stdout, "真源：Zerg-内部文档/项目文档/v2.5.11/设计-CI适配-v1.1.md §三 交叉验证点 2+5 · §五 第一批 ③\n")
		for _, p := range cptsProbes() {
			fmt.Fprintf(stdout, "  %s %s —— cwd=%s · %s\n", p.id, p.name, p.cwd, strings.Join(p.argv, " "))
			fmt.Fprintf(stdout, "     具名红因：%s\n", strings.Join(p.marks, " · "))
			fmt.Fprintf(stdout, "     为什么：%s\n", p.why)
		}
		fmt.Fprint(stdout, "退码：0 全绿 · 1 有命中 · 2 不给结论（clone 失败/缺件/空转/自检不过）\n")
		fmt.Fprint(stdout, "双方 HEAD sha（§十三 M4 · A3）：打印 + 记录两枚 sha；**不相等 ⇒ 明确输出「不对应」**；"+
			"探针全绿也**不当绿**（rc≥1）；任一读不到 ⇒ 「未证」· rc=2；`--shas-only` 只核 sha\n")
		return 0
	}

	if args.selfTest {
		fmt.Fprint(stdout, "── 公开树自洽检查 · 自检（成对负控 · 合成夹具）──\n")
		return cptsSelfTest(stdout)
	}

	tStart := cptsNow()
	tree, note, head := "", "", ""
	if args.tree != "" {
		tree = cptsAbs(args.tree)
		if !cptsIsDir(tree) {
			fmt.Fprintf(stdout, "缺件：--tree %s 不是目录 ⇒ 不给结论\n", tree)
			fmt.Fprint(stdout, "⇒ rc=2\n")
			return 2
		}
		note = "复用已物料化的公开树（未 clone）"
	} else {
		dest := args.dest
		if dest == "" {
			d, err := os.MkdirTemp("", "zerg-selfcheck-")
			if err != nil {
				fmt.Fprintf(stdout, "公开树拿不到：clone 起不来：%s\n", err.Error())
				fmt.Fprint(stdout, "⇒ 缺件 ≠ 绿：本门不给结论（rc=2）—— 不许把「clone 失败」当通过\n")
				return 2
			}
			dest = d
		}
		// ★ A 类止血：py `:459-465` 在 clone 失败档**提前 return，不走 `:539` 的 finally** ⇒ mkdtemp
		//   出来的临时树**永久残留**（生产路径泄漏）。本件把回收提前到**全部出口**（输出面零差异）。
		defer func() {
			if !args.keep {
				_ = os.RemoveAll(dest)
			}
		}()
		gotHead, errMsg := cptsMaterialize(args.repo, dest, args.timeout)
		if gotHead == nil {
			fmt.Fprintf(stdout, "公开树拿不到：%s\n", errMsg)
			fmt.Fprint(stdout, "⇒ 缺件 ≠ 绿：本门不给结论（rc=2）—— 不许把「clone 失败」当通过\n")
			return 2
		}
		tree = dest
		head = *gotHead
		note = "git clone --depth 1 " + args.repo
	}

	// ── 双方 HEAD sha ───────────────────────────────────────────────────────────
	localHead := cptsPyStrip(args.localHead)
	if localHead == "" {
		localHead = cptsReadHead(cptsRepoRoot(), true)
	}
	publicHead := ""
	pubSrc := ""
	if cptsPyStrip(args.publicHead) != "" {
		publicHead, pubSrc = cptsPyStrip(args.publicHead), "显式 --public-head"
	} else if args.tree != "" {
		publicHead, pubSrc = cptsReadHead(tree, true), fmt.Sprintf("现读 git -C %s rev-parse HEAD", tree)
	} else {
		publicHead, pubSrc = head, "现读 clone 落地树的 HEAD"
	}
	corr, corrWhy := cptsJudgeHeads(localHead, publicHead)
	fmt.Fprint(stdout, "── 双方 HEAD sha（§十三 M4 · 波A A3）──\n")
	localSrc := fmt.Sprintf("现读 git -C %s rev-parse HEAD", cptsRepoRoot())
	if args.localHead != "" {
		localSrc = "显式 --local-head"
	}
	fmt.Fprintf(stdout, "   本机 HEAD  ：%s   ← %s\n", cptsOr(localHead, "（读不到）"), localSrc)
	fmt.Fprintf(stdout, "   公开树 HEAD：%s   ← %s\n", cptsOr(publicHead, "（读不到）"), pubSrc)
	fmt.Fprintf(stdout, "   对应判据   ：%s —— %s\n", corr, corrWhy)
	if corr == cptsUnrespond {
		fmt.Fprint(stdout, "   ⚠ **不对应** ⇒ 本次物料化的**不是「要和本机比的那一份」**：下面探针的读数**不作数**（只当旁证）。\n")
	} else if corr == cptsUnproven {
		fmt.Fprint(stdout, "   ⚠ **未证** ⇒ 两枚 sha 不齐 ⇒ 给不出「对应/不对应」（缺件 ≠ 绿）。\n")
	}
	if args.shaFile != "" {
		payload := "{\n" +
			" \"schema\": " + cptsPyJSONStr("zerg-public-tree-selfcheck-shas/1") + ",\n" +
			" \"local_head\": " + cptsPyJSONStr(localHead) + ",\n" +
			" \"public_head\": " + cptsPyJSONStr(publicHead) + ",\n" +
			" \"correspondence\": " + cptsPyJSONStr(corr) + ",\n" +
			" \"why\": " + cptsPyJSONStr(corrWhy) + ",\n" +
			" \"tree\": " + cptsPyJSONStr(tree) + ",\n" +
			" \"note\": " + cptsPyJSONStr(note) + ",\n" +
			" \"when\": " + cptsPyJSONStr(cptsWhen()) + "\n}"
		if err := os.WriteFile(args.shaFile, []byte(payload), 0o644); err != nil {
			fmt.Fprintf(stdout, "   记录写不进去（%s）⇒ 不给结论 · rc=2\n", err.Error())
			return 2
		}
		fmt.Fprintf(stdout, "   已记录双方 sha → %s\n", args.shaFile)
	}
	if args.shasOnly {
		rcMap := map[string]int{cptsCorrespond: 0, cptsUnrespond: 1, cptsUnproven: 2}
		rc := rcMap[corr]
		fmt.Fprintf(stdout, "⇒ --shas-only：**只核双方 HEAD sha**（不跑探针）⇒ %s · rc=%d\n", corr, rc)
		return rc
	}

	type cptsRow struct {
		probe, name, verdict, rc, seconds, note string
		hits                                    int
	}
	rows := []cptsRow{}
	verdicts := []string{}
	for _, p := range cptsProbes() {
		rcP, text, secs, why := cptsRunProbe(tree, p, args.timeout)
		if rcP == nil {
			rows = append(rows, cptsRow{p.id, p.name, "vacuous", "", fmt.Sprintf("%.1f", secs), why, 0})
			verdicts = append(verdicts, "vacuous")
			continue
		}
		v, named, fails := cptsJudge(text, *rcP, p)
		verdicts = append(verdicts, v)
		rows = append(rows, cptsRow{p.id, p.name, v, fmt.Sprintf("%d", *rcP), fmt.Sprintf("%.1f", secs), "", len(named) + len(fails)})
		fmt.Fprintf(stdout, "── %s %s ──\n", p.id, p.name)
		fmt.Fprintf(stdout, "   命令：%s（cwd=%s）· rc=%d · %.1fs\n", strings.Join(p.argv, " "), p.cwd, *rcP, secs)
		grp, total := cptsDedupe(named, args.maxHits)
		if len(grp) > 0 {
			fmt.Fprintf(stdout, "   ✗ 具名红因：%d 行 → 去重 %d 组（列前 %d 组）\n", len(named), total, len(grp))
			for _, g := range grp {
				fmt.Fprintf(stdout, "      [%d×] %s\n", g.count, g.text)
			}
		}
		gf, tf := cptsDedupe(fails, args.maxFailLines)
		if len(gf) > 0 {
			fmt.Fprintf(stdout, "   ✗ FAIL 行：%d 行 → 去重 %d 组（列前 %d 组）\n", len(fails), tf, len(gf))
			for _, g := range gf {
				fmt.Fprintf(stdout, "      [%d×] %s\n", g.count, g.text)
			}
		}
		if v == "green" {
			fmt.Fprint(stdout, "   ✓ 无具名红因 · rc=0\n")
		} else if v == "vacuous" {
			fmt.Fprint(stdout, "   ⚠ 零输出 ⇒ 空转：**不给结论**（不当绿）\n")
		}
		fmt.Fprintf(stdout, "   为什么这条探针在：%s\n", p.why)
	}

	if args.jsonFields != "" {
		var b strings.Builder
		b.WriteString("{\"schema\": " + cptsPyJSONStr("zerg-public-tree-selfcheck/1"))
		b.WriteString(", \"tree\": " + cptsPyJSONStr(tree))
		b.WriteString(", \"head\": " + cptsPyJSONStr(publicHead))
		b.WriteString(", \"local_head\": " + cptsPyJSONStr(localHead))
		b.WriteString(", \"correspondence\": " + cptsPyJSONStr(corr))
		b.WriteString(", \"correspondence_why\": " + cptsPyJSONStr(corrWhy))
		b.WriteString(", \"note\": " + cptsPyJSONStr(note))
		b.WriteString(", \"rows\": [")
		for i, r := range rows {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("{\"probe\": " + cptsPyJSONStr(r.probe))
			b.WriteString(", \"name\": " + cptsPyJSONStr(r.name))
			b.WriteString(", \"verdict\": " + cptsPyJSONStr(r.verdict))
			b.WriteString(", \"rc\": " + cptsPyJSONStr(r.rc))
			b.WriteString(", \"seconds\": " + cptsPyJSONStr(r.seconds))
			b.WriteString(fmt.Sprintf(", \"hits\": %d", r.hits))
			b.WriteString(", \"note\": " + cptsPyJSONStr(r.note) + "}")
		}
		b.WriteString("]}")
		fmt.Fprintln(stdout, b.String())
	}

	green, hit, vac := 0, 0, 0
	for _, v := range verdicts {
		switch v {
		case "green":
			green++
		case "hit":
			hit++
		case "vacuous":
			vac++
		}
	}
	fmt.Fprint(stdout, "── 公开树自洽检查 · 小结 ──\n")
	fmt.Fprintf(stdout, "   树：%s（%s）\n", tree, note)
	fmt.Fprintf(stdout, "   双方 HEAD：本机 %s ｜ 公开树 %s ｜ %s\n", cptsOr(localHead, "（读不到）"), cptsOr(publicHead, "（读不到）"), corr)
	fmt.Fprintf(stdout, "   探针 %d 个：绿 %d · 命中 %d · 不给结论 %d · 全程 %.1fs\n",
		len(verdicts), green, hit, vac, cptsNow().Sub(tStart).Seconds())
	if green == 0 && hit == 0 {
		fmt.Fprint(stdout, "⇒ 一个探针都没跑成 ⇒ 不给结论 · rc=2\n")
		return 2
	}
	if vac > 0 {
		fmt.Fprint(stdout, "⇒ 有探针空转/跑不起来（不给结论 ⇒ 不当绿）· rc=2\n")
		return 2
	}
	if hit > 0 {
		fmt.Fprint(stdout, "⇒ 有命中（只报告档：挂闸那一步按 tri-report 落 REPORT）· rc=1\n")
		return 1
	}
	if corr != cptsCorrespond {
		rc := 1
		if corr == cptsUnproven {
			rc = 2
		}
		kind := "全绿"
		if hit > 0 {
			kind = "有命中"
		}
		fmt.Fprintf(stdout, "⇒ 探针%s，但双方 HEAD **%s** ⇒ 本轮**不当绿**（读数不对应任何一次发布）· rc=%d\n", kind, corr, rc)
		return rc
	}
	fmt.Fprintf(stdout, "⇒ 全绿 · rc=0（双方 HEAD %s ⇒ 读数对应）\n", cptsCorrespond)
	return 0
}

func cptsOr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// cptsAbs = `os.path.abspath`（**纯词法**：拼 cwd 即返回、✗ 不 Clean · py `:452`）。
func cptsAbs(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	return cptsPyJoin(cwd, p)
}

// cptsIsDir = `os.path.isdir`（stat 跟随符号链接）。
func cptsIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
