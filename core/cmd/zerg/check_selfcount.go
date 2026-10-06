// check_selfcount.go —— 判据件 `scripts/gates/check-selfcount.py`（314 行）的
// **Go 真搬**（v2.5.14 · M2 · 入口 `checkSelfcountGateMain`）。
//
// 为什么有这件：
//
//	v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳；M2 = 按价值真搬 Go（落点 `core/cmd/zerg/`）。
//	本件 = M2 一枚，验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码 **三面逐字相同**
//	（除下列**显式声明的**差异面）。★ 本件**只落码、不接线**：不注册命令、不删 py 原件、不动登记面。
//
// 旋钮面（照 py 同名旗标 · ✗ 不另起名）：
//
//	py 本就带 `--root`（判哪棵树）· `--json`；且 `--self-test`/`--selftest` 由**前置串判**直接走自检
//	（不走 argparse）⇒ 本件**同名照搬**。py 另读 `ZERG_REPO_ROOT` env 当仓根 ⇒ 本件**同读**
//	（✗ 无 env 就回落 cwd 上溯，见 (a)）。
//
// 缺省推导（逐字同构）：
//
//	py  `REPO_ROOT = os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))`
//	Go  `cscRepoRoot()` = env `ZERG_REPO_ROOT` 非空则用，否则从 **cwd** 上溯找含 `scripts/gates/` 的目录（无 `__file__` 语境）
//	py  `DEFAULT_ROOT = os.path.join(REPO_ROOT, "scripts", "gates")`
//	Go  `cscPyJoin(repoRoot, "scripts", "gates")`（★ 字符串拼接，✗ filepath.Join）
//	py  `SELF_PATH = os.path.abspath(__file__)`；本件取 `cscAbs(cscPyJoin(repoRoot,"scripts","gates","check-selfcount.py"))`
//
// 两条标准声明行：
//
//	(a) **默认根定位差异**：py 从 `__file__` 上溯（与 cwd 无关）⟷ Go 无 `__file__` 语境、只能从 cwd 上溯。
//	    「脚本位置 == cwd 上溯根」时两者读数逐字相同；否则**路径推导面不同** ⇒ 不属逐字对齐承诺。
//	    ★ py 的 `ZERG_REPO_ROOT` env 面本件同读 ⇒ 该面可恢复逐字（对拍未 export 它）。
//	    ★ py 的 `SELF_PATH = abspath(__file__)`（运行中的那个 .py）⟷ Go 无 `__file__` ⇒ 取
//	    `repoRoot/scripts/gates/check-selfcount.py` 的 abspath（本件所替代的原件在缺省根上的位置）。
//	    真仓缺省档两侧跳过的是**同一个物理件** ⇒ 读数同；「把 py 副本放进别处扫描根」这类构造下两者
//	    可能选到不同物理件 ⇒ 列入未覆盖面（本件对拍夹具不含扫描根内的 check-selfcount.py 副本）。
//	(b) **自指检查（先真判 · 四形态逐条）**：
//	    ① **本件在自己扫描面内吗 ⇒ 否**。本门件面 = 扫描根下的 `.py`/`.sh`（缺省根 = `REPO_ROOT/scripts/gates`）；
//	      本件 `.go` 落 `core/cmd/zerg/` ⇒ 既不在根内、也非后缀 ⇒ **不进件面**。py 侧对**自身**自排除
//	      （`SELF_PATH`）；本件同做（见 (a)），保证真仓本门读数**落仓前后逐字相同**（见交付读数）。
//	    ② **本件的测试件在件面内吗 ⇒ 否**（`.go`/`_test.go` 非 `.sh`/`.py`）。
//	    ③ **self-test 魔法串是否连续字面落地 ⇒ 是（但无害，须证）**：自检夹具正文含本门的标注开括号串
//	      （`{{selfcount:…}}`）。该串**只在 `.py`/`.sh` 件面内才有意义**，而本件是 `.go` ⇒ 本门**永不扫到它**；
//	      且本件已按 (a) 自排除真仓 py 原件 ⇒ 真仓缺省档读数 0 标注、rc=0（落仓前后**逐字相同**，见交付读数）。
//	      全库现读：该串除本门 py 原件与其自检夹具外，另仅见于 `scripts/gates/precommit-gates.sh` 的一行
//	      描述式注释（其 kind 为 `...` ⇒ **非法标识符 ⇒ 不判不计**）⇒ 本件落仓**不改任何他门读数**。
//	    ④ **是否落在任何他门扫描面内 ⇒ 会**（`core/cmd/zerg/*.go` 落 `check-error-kinds` 等门的扫描面）
//	      ⇒ 正文**零绝对路径字面量、零 JSON 键名冒号原串**（键名一律 `cscJSONStr(<键名>)` 派生，
//	      ✗ 不写连续的 `"键": "值"` 原串）⇒ 规避 R3 一族自伤。
//
// py 原件真口子（★ 不自行修 · 逐字保留 · **交回入账**）：
//
//	① **B 类读失败静默**（py `run` 的 `except OSError as e`）：件在盘但读不到 ⇒ 进 `blocked` ⇒ 该档 rc=2
//	  （不与「空域」同档，但同属 fail-closed）。本件**逐字照搬**（保两态对齐）。
//	② **C 类最外层无兜底**（py `__main__` 裸 `sys.exit(main())`）：main 内未捕异常 = 回溯 + rc=1。
//	  本件无新增未捕面（读取失败已捕）；其余异常路径按 fail-closed（未覆盖面①）。
//	③ **D 类**：本件无死旗标（`--root`/`--json` 均被读取）。
//	④ **kind 面**：py 对**非标识符** kind（`<kind>`/`…`/`...`）静默跳过（既不判也不记 bad_ann）——
//	  本件逐字照搬（`cscKindRe` 守卫）。
//
// 遍历 / 排序点逐点交代（★ 决定输出行序的那一条已点名）：
//
//	T1 `os.walk(root)`（topdown · followlinks=False）：当前层**文件先出**，再按 `dirnames` 原序下钻；
//	  本件用 `f.ReadDir(-1)`（★ ✗ 不用会排序的 `os.ReadDir` / `WalkDir`）。
//	T2 `dns[:] = [d for d in dns if d not in (".git","node_modules","__pycache__")]`：**目录名**成员判定（保序）。
//	T3 目录分类 = `entry.is_dir()`（**跟随符号链接**）⇒ 本件用 `os.Stat`，✗ 不用 `entry.IsDir()`（Lstat 语义，
//	  对「指向目录的符号链接」会判反）；递归资格另判「非符号链接」（同 `os.walk` followlinks=False）。
//	T4 `for fn in sorted(fns)`：层内文件名排序 —— **★ 但本件末尾有全局 `sorted(files)`**。
//	★★ **输出行序的决定者**：`scan_files` 末尾的 **`return sorted(files)`**（整串路径的 py 字典序 = 码点序，
//	  与 Go `sort.Strings` 的 UTF-8 字节序**同序**，因 UTF-8 保序）；T4 的层内排序与 T1 的遍历序**皆被此全局
//	  排序覆盖** ⇒ 对「文件集」的判定只需**集合**相等（本件仍逐字复刻 T1–T4 以保证集合与 py 同）。
//	★ 本件 py 对 anns / checked / mismatch 均按**扫描顺序**（全局 sorted 后的件序 → 行号升序）；零 map 迭代进输出。
//
// 正则语义差（py `re` ⟷ Go RE2；涉及面逐条写明）：
//
//	R1 `\s`：py Unicode ⟷ Go ASCII ⇒ 译 py 空白类（本件 `cscPyWSClass`）。涉及 STEP_LINE / ANN。
//	R2 `\S`：译作 `[^<py 空白体>]`（STEP_LINE 的 `(\S+)`）。
//	R3 `\b`：py Unicode 词界 ⟷ Go ASCII ⇒ 译消费式 `(?:[^\p{L}\p{N}_]|\A|\z)`（CHECK_LINE 尾）。
//	  剩余分叉：`\b` 的零宽语义被换成消费一位 ⇒ 仅当该位还需被后随子表达式复用时才可能不同
//	  （CHECK_LINE 的 `\b` 在式尾、无后随）⇒ 不触发；仍写进未覆盖面。
//	R4 `\d`：py Unicode 数字 ⟷ Go ASCII ⇒ 译 `\p{Nd}`（`cscFirstIntRe` / `cscAllNdRe`）；且 `int()` 认
//	  Unicode 数字 ⇒ 本件自写 `cscIntStr`（按 `unicode.Nd` 各块零位求值，✗ 不静默折 0）。见未覆盖面②。
//	R5 `$`：py 非多行 `$` = 「串尾或串尾换行之前」⟷ Go `$` 只匹配串尾。KIND_ID 的 `$` 作用于**单个 token**
//	  （无尾换行）⇒ 不触发；仍写进未覆盖面。
//	R6 `errors="replace"` 非 UTF-8：py 按非法**序列**替一个 U+FFFD ⟷ Go `strings.ToValidUTF8` 按无效**字节**
//	  各替一个 ⇒ 粒度不同（真仓件皆合法 UTF-8 ⇒ 不触发），写进未覆盖面③。
//	R7 `str.splitlines()` 的断行符集（含 `\v`/`\f`/`\x1c-\x1e`/`\x85`/`\u2028`/`\u2029`）本件**逐字复刻**
//	  （`cscSplitLines`），✗ 不用 `strings.Split(s, "\n")`。
//
// 子进程面（★ 本件類别判定 · 见下）：
//
//	py `self_test()` 内 `subprocess.run([sys.executable, SELF_PATH] + a, cwd=tmp)` —— 该子进程 = **自 spawn 自身**
//	（跑的是**同一件、同一条 run 管线**，输入是 `tmp` 下的**合成夹具**）；其输出**不随任何「被判对象的行为」变**
//	（被判对象 = 扫描根下的 `.py`/`.sh` 标注，子进程只把合成夹具再判一遍）⇒ 按现行口径**属「自 spawn 自身 / 夹具
//	harness」⇒ 適格成立**（参 `check_bypass_idioms.go` 体例）。本件做法：**改进程内跑同一管线**（`cscSelfTest`
//	内 `cscRun`）并**显式声明**（未覆盖面⑤）—— 在 py 自检全过时逐字节同（py 全过 ⇒ 各档 `tail` 恒空）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse **版本措辞差**：py ≤3.9 印 `optional arguments:` · ≥3.10 印 `options:`（同一格在两条 PATH 上给出
//	  相反答案）。本件帮助正文**钉 py 3.9.6 措辞**（`optional arguments:`；与同包已搬件同钉）。
//	  门跑环境若换 ≥3.10 解释器，则 `-h` 一格仅**该行**差（**环境依赖 · 已声明**，非实现差）。
//	② `int()` 认 Unicode 数字：本件已自写 `cscIntStr` 按 `unicode.Nd` 块求值；但**超长数字串**（>64 位）py 得
//	  任意精度整数、Go `int` 溢出 ⇒ 该极端档未对齐（真仓 `<N>` 皆小整数 ⇒ 不触发）。
//	③ `errors="replace"` 非 UTF-8 替换粒度（见 R6）。
//	④ `files` kind 的 glob：本件自写 `cscGlob`（支持 `*`/`?`/`[...]` 逐段、隐藏名过滤、`**` 未递归展开），
//	  覆盖真仓用法（`glob=*.py`/`*`）；其余 glob 长尾（`recursive=True` 的 `**`、`os.path.split` 的极端空目录段）未穷举。
//	⑤ `--self-test`：py 侧经 `subprocess.run([sys.executable, SELF_PATH] ...)` **真起子进程**，本件改**进程内**跑同一
//	  管线（`cscRun`）并比对 rc/正文 ⇒ 属「第二类·运行时可表达性差」，**在 py 自检全过时逐字节同**。若某档不过，
//	  py 的 `tail` 取「子进程 stdout+stderr 末行」，本件以进程内同一缓冲复刻（同源 ⇒ 应同；仍属未覆盖面）。
//	⑥ 见 (a)：默认根「脚本位置 ≠ cwd 上溯根」时、以及 SELF_PATH 指向不同物理件时，不属逐字对齐承诺。
//	⑦ 见 R3/R5。
//
// `--evidence GAP-20261001-13`（在册 M2 承接 · 先已回读 · 语义对口 · 状态目录 `~/.zerg/state/zerg-cli-gaps.jsonl`）。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ── 常量与数据（逐字照 py）──────────────────────────────────────────────────

const cscProg = "check-selfcount.py"

// cscKinds —— py `KINDS`。
var cscKinds = []string{"steps", "files", "checks"}

// cscExts —— py `EXTS = (".sh", ".py")`。
var cscExts = []string{".sh", ".py"}

// ── py 语义薄壳 ──────────────────────────────────────────────────────────

// cscPyJoin —— `os.path.join` 逐字（保 `..`/后段绝对/空段三类语义）。
func cscPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		if strings.HasPrefix(p, "/") || out == "" {
			out = p
			continue
		}
		if strings.HasSuffix(out, "/") {
			out = out + p
		} else {
			out = out + "/" + p
		}
	}
	return out
}

// cscJoin —— 斜杠拼串（路径只作打印/匹配，✗ 不落盘）。
func cscJoin(a, b string) string {
	if a == "" {
		return b
	}
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// cscPyDir —— `os.path.dirname`（词法；posixpath.split 的剥尾斜杠语义）。
func cscPyDir(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	if i == 0 {
		return "/"
	}
	return p[:i]
}

// cscPySplit —— `os.path.split`（head 保尾斜杠后按 py 规则剥；tail = 末段）。
func cscPySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	tail := p[i:]
	if head != "" && strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

// cscPyBase —— `os.path.basename`。
func cscPyBase(p string) string {
	_, tail := cscPySplit(p)
	return tail
}

// cscNormPath —— `os.path.normpath` 纯词法版（★ 保留恰两个前导斜杠）。
func cscNormPath(p string) string {
	if p == "" {
		return "."
	}
	abs := strings.HasPrefix(p, "/")
	double := strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///")
	out := []string{}
	for _, c := range strings.Split(p, "/") {
		switch c {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
				continue
			}
			if abs {
				continue
			}
			out = append(out, "..")
		default:
			out = append(out, c)
		}
	}
	res := strings.Join(out, "/")
	if abs {
		res = "/" + res
	}
	if res == "" {
		res = "."
	}
	if double {
		res = "/" + res
	}
	return res
}

// cscAbs —— `os.path.abspath`（相对 ⇒ 拼 cwd；再 normpath）。
func cscAbs(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		p = wd + "/" + p
	}
	return cscNormPath(p)
}

// cscRelPath —— `os.path.relpath(path, start)` 纯词法版。
func cscRelPath(path, start string) string {
	p := cscAbs(path)
	s := cscAbs(start)
	if p == s {
		return "."
	}
	pp := []string{}
	if p != "/" {
		pp = strings.Split(strings.TrimPrefix(p, "/"), "/")
	}
	ss := []string{}
	if s != "/" {
		ss = strings.Split(strings.TrimPrefix(s, "/"), "/")
	}
	i := 0
	for i < len(pp) && i < len(ss) && pp[i] == ss[i] {
		i++
	}
	out := []string{}
	for j := i; j < len(ss); j++ {
		out = append(out, "..")
	}
	out = append(out, pp[i:]...)
	if len(out) == 0 {
		return "."
	}
	return strings.Join(out, "/")
}

// cscRepoRoot —— 见 (a)：env `ZERG_REPO_ROOT` 优先；否则 cwd 上溯找含 `scripts/gates/` 的目录。
func cscRepoRoot() string {
	if env := os.Getenv("ZERG_REPO_ROOT"); env != "" {
		return cscAbs(env)
	}
	return cscDefaultRepo()
}

func cscDefaultRepo() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := cscAbs(wd)
	for i := 0; i < 24; i++ {
		if st, e := os.Stat(cscJoin(cscJoin(d, "scripts"), "gates")); e == nil && st.IsDir() {
			return d
		}
		parent := cscPyDir(d)
		if parent == "" || parent == d {
			break
		}
		d = parent
	}
	return cscAbs(wd)
}

// cscIsSymlink —— `os.path.islink`（Lstat）。
func cscIsSymlink(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode()&os.ModeSymlink != 0
}

// cscLexists —— `os.path.lexists`（Lstat 成功即在场，含悬空链接）。
func cscLexists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// cscUniversalNewlines —— py 文本模式默认 `newline=None` 的换行归一（`\r\n`/`\r` ⇒ `\n`）。
func cscUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// cscReadText —— 读件 ⇒ 逐字近似 py `open(..., encoding="utf-8", errors="replace")` + 通用换行。
func cscReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return cscUniversalNewlines(strings.ToValidUTF8(string(b), "\uFFFD")), nil
}

// cscOSErrText —— py `str(OSError)` 的常见三档逐字复刻。
func cscOSErrText(path string, err error) string {
	if pe, ok := err.(*os.PathError); ok {
		switch {
		case os.IsNotExist(pe):
			return "[Errno 2] No such file or directory: '" + path + "'"
		case os.IsPermission(pe):
			return "[Errno 13] Permission denied: '" + path + "'"
		case strings.Contains(pe.Err.Error(), "is a directory"):
			return "[Errno 21] Is a directory: '" + path + "'"
		}
	}
	return err.Error()
}

// cscIsLinebreak —— py `str.splitlines()` 认的断行符（`_PyUnicode_IsLinebreak`）。
func cscIsLinebreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// cscSplitLines —— py `str.splitlines()` 的等价（断行符集逐字复刻；`\r\n` 计一次）。
func cscSplitLines(s string) []string {
	if s == "" {
		return nil
	}
	rs := []rune(s)
	var out []string
	start := 0
	i := 0
	for i < len(rs) {
		if cscIsLinebreak(rs[i]) {
			out = append(out, string(rs[start:i]))
			cr := rs[i] == '\r'
			i++
			if cr && i < len(rs) && rs[i] == '\n' {
				i++
			}
			start = i
			continue
		}
		i++
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}

// cscIsPySpace —— py `str.isspace()` 认的空白（含 `\x1c-\x1f`；✗ 不是 Go 的 `unicode.IsSpace` 全等）。
func cscIsPySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	if r > 0x7f {
		return unicode.IsSpace(r)
	}
	return false
}

// cscPyStrip —— py `str.strip()`（Unicode 空白）。
func cscPyStrip(s string) string {
	return strings.TrimFunc(s, cscIsPySpace)
}

// cscPyRStrip —— py `str.rstrip()`。
func cscPyRStrip(s string) string {
	return strings.TrimRightFunc(s, cscIsPySpace)
}

// cscPyLStrip —— py `str.lstrip()`。
func cscPyLStrip(s string) string {
	return strings.TrimLeftFunc(s, cscIsPySpace)
}

// cscRuneCut —— py 串切片 `s[:n]`（按**码点**，✗ 不是字节）。
func cscRuneCut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// cscHasSuffix —— py `str.endswith(<元组>)`。
func cscHasSuffix(name string, suffixes []string) bool {
	for _, s := range suffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

// cscPySplitFields —— py `str.split()`（无参）：按 py 空白**run** 切、去首尾空段。
func cscPySplitFields(s string) []string {
	var out []string
	var cur strings.Builder
	inTok := false
	for _, r := range s {
		if cscIsPySpace(r) {
			if inTok {
				out = append(out, cur.String())
				cur.Reset()
				inTok = false
			}
			continue
		}
		cur.WriteRune(r)
		inTok = true
	}
	if inTok {
		out = append(out, cur.String())
	}
	return out
}

// ── py 数字原语（R4：`\d` ⇒ `\p{Nd}` + 自写 `int()`）────────────────────────

func cscIsNd(r rune) bool {
	return unicode.Is(unicode.Nd, r)
}

// cscDigitVal —— 求 Unicode 十进制数字 r 的位值（各 Nd 块零位起，按 `(r-Lo)%10`）。
func cscDigitVal(r rune) (int, bool) {
	for _, rg := range unicode.Nd.R16 {
		if rg.Stride == 1 && r >= rune(rg.Lo) && r <= rune(rg.Hi) {
			return int((uint32(r) - uint32(rg.Lo)) % 10), true
		}
	}
	for _, rg := range unicode.Nd.R32 {
		if rg.Stride == 1 && r >= rune(rg.Lo) && r <= rune(rg.Hi) {
			return int((uint32(r) - uint32(rg.Lo)) % 10), true
		}
	}
	return 0, false
}

// cscIntStr —— py `int(<全 Nd 串>)`（严格：串内每字符皆 Nd）。
func cscIntStr(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, r := range s {
		v, ok := cscDigitVal(r)
		if !ok {
			return 0, false
		}
		n = n*10 + v
	}
	return n, true
}

// cscAsInt —— py `_as_int`：`(s or "").strip()` 后 `fullmatch(\d+)` ⇒ int，否则 None。
func cscAsInt(s string) (int, bool) {
	s = cscPyStrip(s)
	if s == "" {
		return 0, false
	}
	if !cscAllNdRe.MatchString(s) {
		return 0, false
	}
	return cscIntStr(s)
}

// ── 正则（py `re` ⇒ Go RE2；R1–R7）──────────────────────────────────────────

const cscPyWSBody = `\t\n\v\f\r \x{001c}-\x{001f}\x{0085}\x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`
const cscPyWSClass = `[` + cscPyWSBody + `]`

// STEP_LINE = py `^\s*add_step\s+(\S+)\s`。
var cscStepRe = regexp.MustCompile(`^` + cscPyWSClass + `*add_step` + cscPyWSClass + `+([^` + cscPyWSBody + `]+)` + cscPyWSClass)

// CHECK_LINE = py `^\s*(?:assert_eq|_st_chk)\b`（R3 消费式词界）。
var cscCheckRe = regexp.MustCompile(`^` + cscPyWSClass + `*(?:assert_eq|_st_chk)(?:[^\p{L}\p{N}_]|\A|\z)`)

// ANN = py `\{\{\s*selfcount\s*:(?P<body>[^}]*)\}\}(?P<after>[^\n]*)`。
var cscAnnRe = regexp.MustCompile(`\{\{` + cscPyWSClass + `*selfcount` + cscPyWSClass + `*:([^}]*)\}\}([^\n]*)`)

// KIND_ID = py `^[A-Za-z_][A-Za-z0-9_]*$`。
var cscKindRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// 行尾第一个整数（py `re.search(r"\d+", after)`）。
var cscFirstIntRe = regexp.MustCompile(`[\p{Nd}]+`)

// `_as_int` 的 fullmatch。
var cscAllNdRe = regexp.MustCompile(`^[\p{Nd}]+$`)

// ── glob（`files` kind：py `glob.glob(os.path.join(d, pat))` 的 len）──────────

var cscMagicRe = regexp.MustCompile(`[*?[]`)

func cscHasMagic(s string) bool {
	return cscMagicRe.MatchString(s)
}

// cscFnmatchTranslate —— py `fnmatch.translate` 的等价（`*`⇒`.*` · `?`⇒`.` · `[...]` 类）。
func cscFnmatchTranslate(pat string) string {
	var b strings.Builder
	rs := []rune(pat)
	n := len(rs)
	i := 0
	for i < n {
		c := rs[i]
		i++
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			j := i
			if j < n && rs[j] == '!' {
				j++
			}
			if j < n && rs[j] == ']' {
				j++
			}
			for j < n && rs[j] != ']' {
				j++
			}
			if j >= n {
				b.WriteString(`\[`)
			} else {
				stuff := string(rs[i:j])
				stuff = strings.ReplaceAll(stuff, `\`, `\\`)
				i = j + 1
				if len(stuff) > 0 && stuff[0] == '!' {
					stuff = "^" + stuff[1:]
				} else if len(stuff) > 0 && (stuff[0] == '^' || stuff[0] == '[') {
					stuff = "\\" + stuff
				}
				b.WriteString("[" + stuff + "]")
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return `(?s:` + b.String() + `)\z`
}

func cscFnmatch(name, pat string) bool {
	re, err := regexp.Compile(cscFnmatchTranslate(pat))
	if err != nil {
		return false
	}
	return re.MatchString(name)
}

// cscGlob1 —— py `_glob1(dir, pattern)`：listdir + 隐藏名过滤 + fnmatch。
func cscGlob1(dir, pat string) []string {
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	ents, err := f.ReadDir(-1)
	_ = f.Close()
	if err != nil {
		return nil
	}
	hidden := strings.HasPrefix(pat, ".")
	var out []string
	for _, e := range ents {
		name := e.Name()
		if !hidden && strings.HasPrefix(name, ".") {
			continue
		}
		if cscFnmatch(name, pat) {
			out = append(out, cscPyJoin(dir, name))
		}
	}
	return out
}

// cscGlob —— py `glob.glob(pattern)`（非 recursive）：末段 `os.path.split` 切分 + 逐段展开。
func cscGlob(pattern string) []string {
	if !cscHasMagic(pattern) {
		if cscLexists(pattern) {
			return []string{pattern}
		}
		return nil
	}
	head, tail := cscPySplit(pattern)
	var out []string
	if !cscHasMagic(head) {
		out = append(out, cscGlob1(head, tail)...)
	} else {
		for _, d := range cscGlob(head) {
			out = append(out, cscGlob1(d, tail)...)
		}
	}
	return out
}

// ── 扫描（T1–T4 + 全局 sorted）───────────────────────────────────────────────

// cscScanFiles —— py `scan_files`（末尾全局 `sorted(files)` 是输出序的决定者）。
func cscScanFiles(root string) []string {
	var files []string
	var walk func(dir string)
	walk = func(dir string) {
		f, err := os.Open(dir)
		if err != nil {
			return // os.walk onerror=None：读不了的目录静默跳过（B 类目录面）
		}
		ents, err := f.ReadDir(-1) // T1 readdir 原序
		_ = f.Close()
		if err != nil {
			return
		}
		var dirs []string
		for _, e := range ents {
			name := e.Name()
			full := cscPyJoin(dir, name)
			st, serr := os.Stat(full) // T3 跟随符号链接 = entry.is_dir()
			if serr == nil && st.IsDir() {
				if name != ".git" && name != "node_modules" && name != "__pycache__" { // T2 目录名成员判定
					dirs = append(dirs, name)
				}
				continue
			}
			if cscHasSuffix(name, cscExts) {
				files = append(files, full)
			}
		}
		for _, d := range dirs {
			full := cscPyJoin(dir, d)
			if cscIsSymlink(full) { // followlinks=False ⇒ 符号链接目录不下钻
				continue
			}
			walk(full)
		}
	}
	walk(root)
	sort.Strings(files) // ★ 输出序决定者
	return files
}

// ── 标注解析 ────────────────────────────────────────────────────────────────

type cscAnn struct {
	file     string
	line     int
	kind     string
	scope    *string
	glob     *string
	declared *int
}

// cscParseAnnotations —— py `parse_annotations`（ANN **search** ⇒ 一行取首个命中）。
func cscParseAnnotations(path, text string) []cscAnn {
	var out []cscAnn
	for i, ln := range cscSplitLines(text) {
		m := cscAnnRe.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		toks := cscPySplitFields(m[1])
		kind := ""
		if len(toks) > 0 {
			kind = toks[0]
		}
		if !cscKindRe.MatchString(kind) {
			// ★ 非标识符 kind（`<kind>`/`…`/`...`）= 语法示例 / 散文提及 ⇒ 直接跳过（既不判也不记 bad_ann）。
			continue
		}
		kv := map[string]string{}
		for _, t := range toks[1:] {
			if idx := strings.Index(t, "="); idx >= 0 {
				kv[t[:idx]] = t[idx+1:]
			}
		}
		var declared *int
		if v, ok := kv["n"]; ok {
			if n, ok2 := cscAsInt(v); ok2 {
				declared = &n
			}
		} else if v, ok := kv[""]; ok { // 形如 `=3` ⇒ 拆成 k="" v="3"
			if n, ok2 := cscAsInt(v); ok2 {
				declared = &n
			}
		}
		if declared == nil {
			if mm := cscFirstIntRe.FindString(m[2]); mm != "" {
				if n, ok2 := cscIntStr(mm); ok2 {
					declared = &n
				}
			}
		}
		ann := cscAnn{file: path, line: i + 1, kind: kind, declared: declared}
		if v, ok := kv["scope"]; ok {
			vv := v
			ann.scope = &vv
		}
		if v, ok := kv["glob"]; ok {
			vv := v
			ann.glob = &vv
		}
		out = append(out, ann)
	}
	return out
}

// ── 现算 ────────────────────────────────────────────────────────────────────

// cscLiveCount —— py `live_count`（本件自己的口现算；未知 kind ⇒ -1）。
func cscLiveCount(a cscAnn, cache map[string]string) int {
	switch a.kind {
	case "steps":
		text := cache[a.file]
		n := 0
		for _, ln := range cscSplitLines(text) {
			m := cscStepRe.FindStringSubmatch(ln)
			if m == nil {
				continue
			}
			s := m[1]
			if a.scope != nil && *a.scope != "" && *a.scope != "all" && *a.scope != "*" {
				if s == *a.scope {
					n++
				}
			} else if s != "self" { // 分母排除合成自检步
				n++
			}
		}
		return n
	case "checks":
		text := cache[a.file]
		n := 0
		for _, ln := range cscSplitLines(text) {
			if cscCheckRe.MatchString(ln) {
				n++
			}
		}
		return n
	case "files":
		d := cscPyDir(a.file)
		pat := "*"
		if a.glob != nil {
			pat = *a.glob
		}
		return len(cscGlob(cscPyJoin(d, pat)))
	}
	return -1
}

// ── 记录类型 ────────────────────────────────────────────────────────────────

type cscRec struct {
	file     string
	line     int
	kind     string
	scope    *string
	glob     *string
	declared int
	live     int
	ok       bool
}

type cscBlockedFile struct {
	file string
	err  string
}

type cscBadAnn struct {
	file string
	line int
	err  string
}

// ── 机器面（逐字节复刻 py `json.dumps(obj, ensure_ascii=False)`）─────────────

// cscJSONStr —— py `json.dumps(str, ensure_ascii=False)` 的字符串格。
func cscJSONStr(s string) string {
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
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// cscStrOrNull —— None ⇒ `null`；字符串 ⇒ 引号格。
func cscStrOrNull(p *string) string {
	if p == nil {
		return "null"
	}
	return cscJSONStr(*p)
}

func cscBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// cscRecJSON —— checked / mismatch 元素（键固定序：file/line/kind/scope/glob/declared/live/ok）。
func cscRecJSON(rec cscRec) string {
	parts := []string{
		cscJSONStr("file") + ": " + cscJSONStr(rec.file),
		cscJSONStr("line") + ": " + strconv.Itoa(rec.line),
		cscJSONStr("kind") + ": " + cscJSONStr(rec.kind),
		cscJSONStr("scope") + ": " + cscStrOrNull(rec.scope),
		cscJSONStr("glob") + ": " + cscStrOrNull(rec.glob),
		cscJSONStr("declared") + ": " + strconv.Itoa(rec.declared),
		cscJSONStr("live") + ": " + strconv.Itoa(rec.live),
		cscJSONStr("ok") + ": " + cscBool(rec.ok),
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// cscJSONPayload —— py `--json` 的载荷（键**固定序**）。
func cscJSONPayload(root string, annN int, checked, mism []cscRec, blocked []cscBlockedFile, badAnn []cscBadAnn) string {
	var ci []string
	for _, rec := range checked {
		ci = append(ci, cscRecJSON(rec))
	}
	var mi []string
	for _, rec := range mism {
		mi = append(mi, cscRecJSON(rec))
	}
	var bi []string
	for _, b := range blocked {
		bi = append(bi, "{"+cscJSONStr("file")+": "+cscJSONStr(b.file)+
			", "+cscJSONStr("err")+": "+cscJSONStr(b.err)+"}")
	}
	var ai []string
	for _, a := range badAnn {
		ai = append(ai, "{"+cscJSONStr("file")+": "+cscJSONStr(a.file)+
			", "+cscJSONStr("line")+": "+strconv.Itoa(a.line)+
			", "+cscJSONStr("err")+": "+cscJSONStr(a.err)+"}")
	}
	parts := []string{
		cscJSONStr("schema") + ": " + cscJSONStr("zerg/v1"),
		cscJSONStr("kind") + ": " + cscJSONStr("SelfCount"),
		cscJSONStr("root") + ": " + cscJSONStr(root),
		cscJSONStr("annotations") + ": " + strconv.Itoa(annN),
		cscJSONStr("checked") + ": [" + strings.Join(ci, ", ") + "]",
		cscJSONStr("mismatch") + ": [" + strings.Join(mi, ", ") + "]",
		cscJSONStr("blocked") + ": [" + strings.Join(bi, ", ") + "]",
		cscJSONStr("bad_annotation") + ": [" + strings.Join(ai, ", ") + "]",
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// cscPyReprStr —— py `%r`（str）：本件只用于 kind（已保证是标识符）⇒ 简单单引号格。
func cscPyReprStr(s string) string {
	quote := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		quote = `"`
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
			if byte(r) == quote[0] {
				b.WriteRune('\\')
				b.WriteRune(r)
			} else if r < 0x20 {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteString(quote)
	return b.String()
}

// ── 用法 / argparse 复刻 ──────────────────────────────────────────────────

const cscUsage = "usage: check-selfcount.py [-h] [--root ROOT] [--json]\n"

const cscHelp = cscUsage + "\n" +
	"optional arguments:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --json\n"

type cscArgs struct {
	root *string
	json bool
}

var cscLongOpts = []string{"--root", "--json", "--help"}

var cscNeedsValue = map[string]bool{"--root": true}

var cscStoreTrue = map[string]bool{"--json": true}

// cscResolveLong —— 长旗标**前缀匹配**（唯一命中 ⇒ 规范名；多命中 ⇒ 歧义）。
func cscResolveLong(tok string) (string, string, bool, error) {
	name := tok
	inline := ""
	hasInline := false
	if i := strings.Index(tok, "="); i >= 0 {
		name = tok[:i]
		inline = tok[i+1:]
		hasInline = true
	}
	if name == "-h" {
		name = "--help"
	} else if strings.HasPrefix(name, "--") {
		matches := []string{}
		for _, o := range cscLongOpts {
			if o == name {
				matches = []string{o}
				break
			}
			if strings.HasPrefix(o, name) {
				matches = append(matches, o)
			}
		}
		if len(matches) == 0 {
			return "", "", false, nil
		}
		if len(matches) > 1 {
			return "", "", false, fmt.Errorf("ambiguous option: %s could match %s", name, strings.Join(matches, ", "))
		}
		name = matches[0]
	} else {
		return "", "", false, nil
	}
	if hasInline {
		return name, inline, true, nil
	}
	return name, "\x00none", false, nil
}

// cscParseArgs —— argparse 等价解析。
func cscParseArgs(argv []string) (*cscArgs, string, bool, error) {
	args := &cscArgs{}
	var unrecognized []string
	endOpts := false
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if endOpts {
			unrecognized = append(unrecognized, tok)
			continue
		}
		if tok == "--" {
			// py argparse：`--` 自身也计入未识别（无 positional 时），且其后所有 token 皆未识别
			unrecognized = append(unrecognized, tok)
			endOpts = true
			continue
		}
		if tok == "-h" || tok == "--help" {
			return nil, cscHelp, true, nil
		}
		if !strings.HasPrefix(tok, "-") || tok == "-" {
			unrecognized = append(unrecognized, tok)
			continue
		}
		name, inline, hasInline, err := cscResolveLong(tok)
		if err != nil {
			return nil, "", false, err
		}
		if name == "" {
			unrecognized = append(unrecognized, tok)
			continue
		}
		val := inline
		if cscStoreTrue[name] && hasInline {
			return nil, "", false, fmt.Errorf("argument %s: ignored explicit argument '%s'", name, inline)
		}
		if inline == "\x00none" {
			if cscNeedsValue[name] {
				if i+1 >= len(argv) || (strings.HasPrefix(argv[i+1], "-") && argv[i+1] != "-" && len(argv[i+1]) > 1) {
					return nil, "", false, fmt.Errorf("argument %s: expected one argument", name)
				}
				i++
				val = argv[i]
			}
		}
		switch name {
		case "--root":
			vv := val
			args.root = &vv
		case "--json":
			args.json = true
		}
	}
	if len(unrecognized) > 0 {
		return nil, "", false, fmt.Errorf("unrecognized arguments: %s", strings.Join(unrecognized, " "))
	}
	return args, "", false, nil
}

// ── 主流程 ────────────────────────────────────────────────────────────────

// cscRunCheck —— py `run(root, as_json)`。
func cscRunCheck(root string, asJSON bool, repoRoot string, stdout, stderr io.Writer) int {
	st, err := os.Stat(root) // os.path.isdir（跟随符号链接）
	if err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "BLOCKED —— 扫描根不存在或读不到：%s（rc=2 · 不给结论）\n", root)
		return 2
	}

	files := cscScanFiles(root)
	if len(files) == 0 { // ★ 空集守卫：空域 ⇒ fail-closed（✗ 不许当「无标注 ⇒ 不判」绿）
		fmt.Fprintf(stderr, "BLOCKED —— 空域：%s 下 0 个 .sh/.py（取不到输入）⇒ rc=2 不给结论\n", root)
		return 2
	}

	selfPath := cscAbs(cscPyJoin(repoRoot, "scripts", "gates", "check-selfcount.py"))
	var anns []cscAnn
	cache := map[string]string{}
	var blocked []cscBlockedFile
	for _, p := range files {
		if cscAbs(p) == selfPath {
			continue // ★ 自排除：本件所替代的 py 原件（见 (a)/(b)①）—— 其自检夹具示例串都落在此。
		}
		text, rerr := cscReadText(p)
		if rerr != nil {
			blocked = append(blocked, cscBlockedFile{file: p, err: cscOSErrText(p, rerr)})
			continue
		}
		cache[p] = text
		anns = append(anns, cscParseAnnotations(p, text)...)
	}

	var checked, mism []cscRec
	var badAnn []cscBadAnn
	for _, a := range anns {
		rel := cscRelPath(a.file, root)
		known := false
		for _, k := range cscKinds {
			if a.kind == k {
				known = true
				break
			}
		}
		if !known {
			badAnn = append(badAnn, cscBadAnn{file: rel, line: a.line,
				err: fmt.Sprintf("kind 未知：%s（合法 %s）", cscPyReprStr(a.kind), strings.Join(cscKinds, "/"))})
			continue
		}
		if a.declared == nil {
			badAnn = append(badAnn, cscBadAnn{file: rel, line: a.line,
				err: "标注缺声明数 N（须 `= <N>` 或行尾给一个整数）"})
			continue
		}
		live := cscLiveCount(a, cache)
		rec := cscRec{file: rel, line: a.line, kind: a.kind, scope: a.scope, glob: a.glob,
			declared: *a.declared, live: live, ok: live == *a.declared}
		checked = append(checked, rec)
		if !rec.ok {
			mism = append(mism, rec)
		}
	}

	rc := 0
	if len(blocked) > 0 || len(badAnn) > 0 {
		rc = 2
	} else if len(mism) > 0 {
		rc = 1
	}

	if asJSON {
		fmt.Fprintln(stdout, cscJSONPayload(root, len(anns), checked, mism, blocked, badAnn))
		return rc
	}

	fmt.Fprintf(stdout, "扫描根：%s\n", root)
	fmt.Fprintf(stdout, "标注 %d 条 · 已判 %d 条 · 不符 %d 条 · 标注不全 %d 条\n",
		len(anns), len(checked), len(mism), len(badAnn))
	for _, rec := range checked {
		mark := "✓"
		if !rec.ok {
			mark = "✗"
		}
		scopeStr := ""
		if rec.scope != nil && *rec.scope != "" {
			scopeStr = " scope=" + *rec.scope
		}
		fmt.Fprintf(stdout, "  %s %s:%d · kind=%s%s · 声明=%d · 现读=%d\n",
			mark, rec.file, rec.line, rec.kind, scopeStr, rec.declared, rec.live)
	}
	for _, b := range badAnn {
		fmt.Fprintf(stderr, "  BLOCKED %s:%d · %s\n", b.file, b.line, b.err)
	}
	for _, b := range blocked {
		fmt.Fprintf(stderr, "  BLOCKED 读不到：%s（%s）\n", b.file, b.err)
	}

	switch {
	case rc == 2:
		fmt.Fprint(stdout, "⇒ 缺语料 / 标注不全 ⇒ rc=2（不给结论）\n")
	case len(mism) > 0:
		fmt.Fprint(stdout, "⇒ 声明数 ⟷ 现读数**对不上**（逐条点名在上）⇒ rc=1\n")
	case len(anns) == 0:
		fmt.Fprint(stdout, "⇒ 无 `{{selfcount:…}}` 标注 ⇒ **不判**（rc=0）\n")
	default:
		fmt.Fprint(stdout, "⇒ 标注数逐条与现读相符 ⇒ rc=0\n")
	}
	return rc
}

// ─────────────────────── 自检（成对负控 · 合成夹具 · 不碰真目标）───────────────────────
//  纪律：合成件全在临时目录（os.MkdirTemp + defer 清理）⇒ 不读不写仓内件 ·
//        **子进程面改进程内**（py 的 subprocess 是自 spawn 自身/夹具 harness，见件头「子进程面」+ 未覆盖面⑤）·
//        stderr 一律经 `stderr`。

// cscStChk —— py `_st_chk`：正控比对 rc；✗ 时附末行 tail（≤90 码点）。
func cscStChk(stdout io.Writer, bad *int, label string, want, rc int, out string) bool {
	good := rc == want
	if !good {
		*bad++
	}
	tail := ""
	if !good {
		lines := cscSplitLines(cscPyStrip(out))
		last := ""
		if len(lines) > 0 {
			last = lines[len(lines)-1]
		}
		tail = " ｜ " + cscRuneCut(last, 90)
	}
	mark := "✓"
	if !good {
		mark = "✗"
	}
	fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", mark, label, want, rc, tail)
	return good
}

// cscSelfTest —— py `self_test()`（十档；返回 0 全过 / 1 有档不过）。
func cscSelfTest(stdout io.Writer) int {
	bad := 0
	fmt.Fprint(stdout, "── check-selfcount.py --self-test 十档（三 kind 各正控 0 · 负控 1 · 缺语料 2）\n")
	tmp, terr := os.MkdirTemp("", "selfcount-selftest-")
	if terr != nil {
		// 建点失败 ⇒ 不进（✗ 无条件 RemoveAll 会误删系统临时目录）
		fmt.Fprintln(stdout, "  ✗ 自检夹具建点失败")
		fmt.Fprint(stdout, "── 自检：**1 档不过** ✗\n")
		return 1
	}
	defer os.RemoveAll(tmp) // ★ 安全删点：仅在 MkdirTemp 成功支

	put := func(sub, name, body string) string {
		d := cscPyJoin(tmp, sub)
		_ = os.MkdirAll(d, 0o755)
		_ = os.WriteFile(cscPyJoin(d, name), []byte(body), 0o644)
		return d
	}

	// ① 正控：scope=gates 声明 = 现读
	dOK := put("ok", "steps.sh",
		"# {{selfcount:steps scope=gates}} = 2\nadd_step gates \"甲\"\nadd_step gates \"乙\"\n")
	// ② 负控：声明 ≠ 现读（同一份件，声明写成 3）
	dBad := put("bad", "steps.sh",
		"# {{selfcount:steps scope=gates}} = 3\nadd_step gates \"甲\"\nadd_step gates \"乙\"\n")
	// ③ 无标注：有 add_step 但无标注 ⇒ 不判
	dNoAnn := put("noann", "steps.sh",
		"add_step gates \"甲\"\nadd_step gates \"乙\"\n")
	// ④ 自检步排除：全集声明 = 1（`self` 不计入分母）
	dExcl := put("excl", "steps.sh",
		"# {{selfcount:steps}} = 1\nadd_step self \"自检-必过-1\"\nadd_step go \"core: build\"\n")
	// ⑤ 自检步排除负控：若误把 self 计入 ⇒ 现读会变 2，声明 1 ⇒ 红（这里声明 2 期望红）
	dExclBad := put("exclbad", "steps.sh",
		"# {{selfcount:steps}} = 2\nadd_step self \"自检-必过-1\"\nadd_step go \"core: build\"\n")
	// ⑥ checks kind：正控（声明 2 = 现读 2）
	dCkOK := put("ckok", "checks.py",
		"# {{selfcount:checks}} = 2\ndef t():\n    assert_eq(\"a\", 1, 1)\n    assert_eq(\"b\", 2, 2)\n")
	// ⑦ checks kind 负控：声明 3 ≠ 现读 2
	dCkBad := put("ckbad", "checks.py",
		"# {{selfcount:checks}} = 3\ndef t():\n    assert_eq(\"a\", 1, 1)\n    assert_eq(\"b\", 2, 2)\n")
	// ⑧ files kind 正控：本目录 2 个 .py（本件 + 同伴）⇒ 声明 2 = 现读 2
	dFlOK := put("flok", "countme.py", "# {{selfcount:files glob=*.py}} = 2\n")
	_ = put("flok", "peer.py", "# 同伴件（供 files kind 计数）\n")
	// ⑨ files kind 负控：声明 5 ≠ 现读 2
	dFlBad := put("flbad", "countme.py", "# {{selfcount:files glob=*.py}} = 5\n")
	_ = put("flbad", "peer.py", "# 同伴件（供 files kind 计数）\n")

	repoRoot := cscRepoRoot()
	run := func(a ...string) (int, string) {
		var o, e strings.Builder
		rc := cscRun(a, repoRoot, &o, &e)
		return rc, o.String() + e.String()
	}
	st := func(label string, want int, a ...string) {
		rc, out := run(a...)
		cscStChk(stdout, &bad, label, want, rc, out)
	}

	st("档① 正控：scope=gates 声明=现读 ⇒ rc=0", 0, "--root", dOK)
	st("档② 负控：声明 3 ≠ 现读 2 ⇒ rc=1（点名）", 1, "--root", dBad)
	st("档③ 无标注：一律不判 ⇒ rc=0", 0, "--root", dNoAnn)
	st("档④ 自检步排除：`self` 不计入分母（声明 1=现读 1）⇒ rc=0", 0, "--root", dExcl)
	st("档⑤ 自检步排除负控：声明 2 ≠ 现读 1（self 已排除）⇒ rc=1", 1, "--root", dExclBad)
	st("档⑥ checks 正控：声明 2=现读 2 ⇒ rc=0", 0, "--root", dCkOK)
	st("档⑦ checks 负控：声明 3 ≠ 现读 2 ⇒ rc=1", 1, "--root", dCkBad)
	st("档⑧ files 正控：声明 2=现读 2 ⇒ rc=0", 0, "--root", dFlOK)
	st("档⑨ files 负控：声明 5 ≠ 现读 2 ⇒ rc=1", 1, "--root", dFlBad)
	st("档⑩ 缺语料：`--root` 不存在 ⇒ rc=2（不给结论）", 2, "--root", cscPyJoin(tmp, "no-such-root"))

	concl := "全过 ✓"
	if bad > 0 {
		concl = fmt.Sprintf("**%d 档不过** ✗", bad)
	}
	fmt.Fprintf(stdout, "── 自检：%s\n", concl)
	if bad == 0 {
		return 0
	}
	return 1
}

// ── 入口 ────────────────────────────────────────────────────────────────

// checkSelfcountGateMain —— 入口（与 py `main()` 逐字同构）。
func checkSelfcountGateMain(stdout, stderr io.Writer, argv []string) int {
	return cscRun(argv, cscRepoRoot(), stdout, stderr)
}

// cscRun —— 进程内主线（自检与入口共用；repoRoot 显式传入以对齐 py 的 REPO_ROOT）。
func cscRun(argv []string, repoRoot string, stdout, stderr io.Writer) int {
	// py：`if "--self-test" in sys.argv[1:] or "--selftest" in sys.argv[1:]`（argparse 之前）
	for _, a := range argv {
		if a == "--self-test" || a == "--selftest" {
			return cscSelfTest(stdout)
		}
	}
	args, helpText, helpWanted, err := cscParseArgs(argv)
	if err != nil {
		fmt.Fprint(stderr, cscUsage)
		fmt.Fprintf(stderr, "%s: error: %s\n", cscProg, err.Error())
		return 2
	}
	if helpWanted {
		fmt.Fprint(stdout, helpText)
		return 0
	}
	root := cscPyJoin(repoRoot, "scripts", "gates") // py `DEFAULT_ROOT`
	if args.root != nil {
		root = *args.root
	}
	return cscRunCheck(root, args.json, repoRoot, stdout, stderr)
}
