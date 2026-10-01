// check_links_lychee.go —— 判据件 `scripts/gates/check-links-lychee.py`（432 行 · sha16 77ec4886c320c11a）
// 的 **Go 真搬**（v2.5.14 · M2 · 甲案 C 类第 14 枚 · 入口 `checkLinksLycheeGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的口径差与未覆盖面）。
//
// 本件在 py 侧的面（现读 · 逐条点号）：
//
//	· 旗标面：`--root`（缺省 = 脚本上两级）· `--devdocs` · `--files` · `--self-test` · `-h/--help`；
//	  另有**argparse 之前**的 `--json` 拦截口（缺口 Q-176：本门无机器读面 ⇒ 给了 `--json` 逐字说明后退 2）。
//	· 遍历面：`git_md_files` = `git -c core.quotepath=false ls-files *.md`（cwd=root）⇒ 行序 = git 自带的
//	  路径序（**不是** readdir 序）；`build_file_list` 把仓根清单与 `--devdocs` 旁树清单**顺序拼接**；
//	  `--files` 给定则**逐字**用它（过滤空行与 `#` 起头行）。★ **输出行序的决定者不是遍历序**，而是
//	  `broken_records` 末尾的 `sorted(rows, key=(rel, str(line), str(column), url))` —— 全局排序，
//	  故 error_map 的键序（dict 插入序）**不进输出**（Go 用 map + 同一 total order 排序即可逐字同）。
//	· 子进程面：① `git`（上面那条，capture_output/text/stdin=DEVNULL，cwd=root）；② `lychee`
//	  （`--offline --no-progress --format json --files-from <临时清单>`，capture_output/text/stdin=DEVNULL，
//	  **无 cwd** ⇒ 继承进程 cwd）；③ `shutil.which("lychee")` 存在性探测。
//	· 写面：唯一一处 = `tempfile.NamedTemporaryFile` 落件清单（用后 `os.unlink`）；`--self-test` 另在
//	  `TemporaryDirectory` 里造夹具。默认档**不写任何仓内件**。
//	· 外部真源真身：`lychee` 二进制（本机 `/opt/homebrew/bin/lychee`；`--offline` ⇒ **零联网**，外链一律
//	  `Excluded`）· `git`（本机 `/usr/bin/git`）。对拍用**仓外幂替身（假 lychee）**验命令串与退码映射，
//	  并在 `--self-test` 档用真 lychee（`--offline`，不联网）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-links-lychee.py` 原件（退场规则：**对拍通过后才**退场）；
//	· 入口 `checkLinksLycheeGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkLinksLycheeGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 判过 / 1 判红（逐条点名）/ 2 不给结论（缺判据 · 空转 0 件 · 登记过期 · 用法错）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `HERE = dirname(abspath(__file__))` ·
//	    `DEFAULT_ROOT = dirname(dirname(HERE))` ⇒ 默认根起于**脚本所在目录**（与 cwd 无关）；
//	    本件**无 `__file__` 语境** ⇒ 从 **cwd** 上溯 ≤8 层找含 `scripts/gates` 的目录、
//	    找不到回落 `abspath(cwd)`。**「脚本位置 == cwd 上溯根」时两者读数逐字相同**（对拍甲/乙两环境已证）；
//	    否则路径推导面不同，不属逐字对齐承诺，对拍一律显式传根或在夹具里让两者重合。
//	(b) **自指检查（先真判 · 逐形态）**：
//	    ① **本件在自己扫描面内吗 ⇒ 否**。本门扫面 = 仓根 `git ls-files *.md` 的 **`.md` 件**；
//	       本件是 `.go`、落 `core/cmd/zerg/`。**量化依据**：`git ls-files *.md` 现读 235 件、逐件
//	       `HasSuffix(".md")`；本件后缀 `.go` ⇒ 恒不入清面。
//	    ② **件头注释里的真引号路径原串**：本件正文只含**被引述的** py 件名（`scripts/gates/check-links-lychee.py`）
//	       与 flag/背引号里的**说明性**文本；扫描面是 `.md` 件 ⇒ 不落地。
//	    ③ **self-test 魔法串连续字面落地**：本门自检夹具全在 `os.MkdirTemp`（仓外）里；无「扫真目标」魔法串。
//	    ④ **本件落在「任何他门」扫描面内吗 ⇒ 是**（`check-hardcoded-private-paths.py` 的 SCAN_DIRS 含
//	       `core/`；`check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go`）⇒ 已按血证处置：
//	       · 正文**零私有绝对路径字面量**（`/Volumes/...`、`/Users/...` 一概不写：真仓根/外部树一律运行时取）；
//	       · **不写** `check-error-kinds` 的被扫错误构造调用原串（无 `setErr` 后接引号那种形态、
//	         也没有 JSON 键 kind 的引号字面形态 —— 两条被扫正则都不命中）；
//	       · 新落件此刻尚未被 `git ls-files` 跟踪 ⇒ 凡以 git 面为准的判据现读命中 0（收口批提交后才真进面）。
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// ── 常量面（与 py 逐字同）────────────────────────────────────────────────────────────────────

const cllProg = "check-links-lychee.py" // py argparse 的 prog = basename(argv[0])
const cllLychee = "lychee"

// cllLycheeArgs —— py `LYCHEE_ARGS` 逐字（`--files-from` 的清单路径运行时追加）。
var cllLycheeArgs = []string{"--offline", "--no-progress", "--format", "json", "--files-from"}

// cllDetailCap —— py `red[:40]` 的明细上限（★ 具名常量：T3 只收「⊆ 真源退码表已登记集」的裸数字）。
const cllDetailCap = 40

// cllBaseOffset —— py `BASE_OFFSET` 逐字（「件在 A 处、链接按 B 基准写」的**收窄判据** · 非豁免名单）。
var cllBaseOffset = map[string][2]string{
	"publish/docs/README.md":       {"publish", "公开树仓根定位"},
	"publish/docs/README.zh-CN.md": {"publish", "公开树仓根定位（与 en 版同基准 · 语言切换互链）"},
}

// cllBaseOffsetOrder —— py `sorted(BASE_OFFSET.items())` 的键序（按串升序 · 与 Go map 迭代序无关）。
var cllBaseOffsetOrder = []string{"publish/docs/README.md", "publish/docs/README.zh-CN.md"}

// cllHelpText —— py argparse `-h/--help` 的 stdout 逐字（Python 3.9 用 `optional arguments:` ·
// 换行宽度按 argparse 的 80 列折行 · prog 取件名基线 `check-links-lychee.py`）。
const cllHelpText = "usage: check-links-lychee.py [-h] [--root ROOT] [--devdocs DEVDOCS]\n" +
	"                             [--files FILES] [--self-test]\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help         show this help message and exit\n" +
	"  --root ROOT        仓根（默认 = 本脚本的上两级）\n" +
	"  --devdocs DEVDOCS  旁树根（给了就并进扫面：域 = 两棵树）\n" +
	"  --files FILES      显式件清单（一行一件 · 覆盖 git 扫面）\n" +
	"  --self-test        只跑自检（成对负控 · 不碰真目标）\n"

// cllUsageLine —— argparse 出错时先印的 `usage:` 段（与 help 首两行同）。
const cllUsageLine = "usage: check-links-lychee.py [-h] [--root ROOT] [--devdocs DEVDOCS]\n" +
	"                             [--files FILES] [--self-test]\n"

// ═════════════════ py 词法面自写函数（join / normpath / dirname / abspath / relpath / strip / splitlines）═════════════════
// ✗ 一律不用 filepath.Join / Abs / Rel（三类语义差见 py-path-semantics）；本件只涉及 join/normpath/
// dirname/abspath/relpath 五个 + 两个字符串原语（strip/splitlines）。

// cllPyJoin —— 逐字复刻 CPython `posixpath.join`（吃 `..` 字面 · 后段绝对丢前段 · 空段留尾 `/`）。
func cllPyJoin(a string, p ...string) string {
	path := a
	for _, b := range p {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path += b
		default:
			path += "/" + b
		}
	}
	return path
}

// cllPyNormpath —— 逐字复刻 CPython `posixpath.normpath`（★ 保留**恰好两个**前导斜杠）。
func cllPyNormpath(p string) string {
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
	var nc []string
	for _, c := range strings.Split(p, "/") {
		if c == "" || c == "." {
			continue
		}
		if c != ".." || (initial == 0 && len(nc) == 0) || (len(nc) > 0 && nc[len(nc)-1] == "..") {
			nc = append(nc, c)
		} else if len(nc) > 0 {
			nc = nc[:len(nc)-1]
		}
	}
	out := strings.Join(nc, "/")
	if initial > 0 {
		out = strings.Repeat("/", initial) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// cllPyDirname —— 逐字复刻 CPython `posixpath.dirname`（= split 的头部，剥尾斜杠）。
func cllPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// cllPyAbspath —— 逐字复刻 CPython `posixpath.abspath`（**纯词法**：不碰盘、不解析符号链接）。
// ★ cwd 必须**由调用方传入**（✗ 不在函数里 os.Getwd()：那会解析真路径 ⇒ 符号链接路径夹具假失配）。
func cllPyAbspath(cwd, p string) string {
	if strings.HasPrefix(p, "/") {
		return cllPyNormpath(p)
	}
	return cllPyNormpath(cllPyJoin(cwd, p))
}

// cllPyRelpath —— 逐字复刻 CPython `posixpath.relpath`（start 缺省 cwd）。ok=false ⇔ py 抛 ValueError（空 path）。
func cllPyRelpath(cwd, path, start string) (string, bool) {
	if path == "" {
		return "", false
	}
	split := func(p string) []string {
		var out []string
		for _, c := range strings.Split(p, "/") {
			if c != "" {
				out = append(out, c)
			}
		}
		return out
	}
	s := split(cllPyAbspath(cwd, start))
	p := split(cllPyAbspath(cwd, path))
	i := 0
	for i < len(s) && i < len(p) && s[i] == p[i] {
		i++
	}
	var rel []string
	for k := 0; k < len(s)-i; k++ {
		rel = append(rel, "..")
	}
	rel = append(rel, p[i:]...)
	if len(rel) == 0 {
		return ".", true
	}
	return strings.Join(rel, "/"), true
}

// cllPyIsSpace —— py `str.isspace()` 的字符集（★ `\x1c-\x1f` 也是空白 · Go unicode.IsSpace 不认）。
func cllPyIsSpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// cllPyStrip —— 逐字复刻 py `str.strip()`（无参 = 剥 py 的 Unicode 空白）。
func cllPyStrip(s string) string {
	return strings.TrimFunc(s, cllPyIsSpace)
}

// cllPySplitLines —— 逐字复刻 py `str.splitlines()`（无 keepends）：\n \r \r\n \v \f \x1c-\x1e \x85 \u2028 \u2029。
func cllPySplitLines(s string) []string {
	out := []string{}
	var cur strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r == '\r' {
			out = append(out, cur.String())
			cur.Reset()
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			continue
		}
		if r == '\n' || r == '\v' || r == '\f' || r == 0x1c || r == 0x1d || r == 0x1e || r == 0x85 || r == 0x2028 || r == 0x2029 {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// cllPySplitFirst —— py `s.split(sep, 1)[0]`（无 sep ⇒ 整串）。
func cllPySplitFirst(s, sep string) string {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i]
	}
	return s
}

// cllStrStripMax —— py `(x or "").strip()[:N]`（★ 切片按**码点** ⇒ 先 []rune 再切）。
func cllStrStripMax(s string, n int) string {
	s = cllPyStrip(s)
	rs := []rune(s)
	if len(rs) > n {
		return string(rs[:n])
	}
	return s
}

// cllUniversalNL —— py 文本模式 / subprocess text=True 的 universal newlines（\r\n 与裸 \r 都折成 \n）。
func cllUniversalNL(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// cllPyUnquote —— 复刻 `urllib.parse.unquote`（默认 utf-8 / errors=replace）。
func cllPyUnquote(s string) string {
	var out strings.Builder
	var buf []byte
	flush := func() {
		if len(buf) > 0 {
			out.WriteString(strings.ToValidUTF8(string(buf), "\uFFFD"))
			buf = buf[:0]
		}
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi, ok1 := cllHexVal(s[i+1])
			lo, ok2 := cllHexVal(s[i+2])
			if ok1 && ok2 {
				buf = append(buf, byte(hi<<4|lo))
				i += 2
				continue
			}
		}
		flush()
		out.WriteByte(s[i])
	}
	flush()
	return out.String()
}

func cllHexVal(b byte) (int, bool) {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0'), true
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10, true
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10, true
	}
	return 0, false
}

// cllPyOSErr —— py `OSError` 的 str 形态 `[Errno N] <strerror>: '<path>'`（只覆盖真仓会碰到的四种）。
func cllPyOSErr(err error) string {
	name := ""
	var pe *os.PathError
	if errors.As(err, &pe) {
		name = pe.Path
	}
	var ee *exec.Error
	if name == "" && errors.As(err, &ee) {
		name = ee.Name
	}
	if name == "" {
		name = cllLychee
	}
	switch {
	case errors.Is(err, os.ErrNotExist), errors.Is(err, exec.ErrNotFound):
		return "[Errno 2] No such file or directory: '" + name + "'"
	case errors.Is(err, os.ErrPermission):
		return "[Errno 13] Permission denied: '" + name + "'"
	case errors.Is(err, syscall.EISDIR):
		return "[Errno 21] Is a directory: '" + name + "'"
	case errors.Is(err, syscall.ENOTDIR):
		return "[Errno 20] Not a directory: '" + name + "'"
	}
	return "[Errno 5] Input/output error: '" + name + "'"
}

// ── JSON 取值面（py `%s` / `int()` 语义）────────────────────────────────────────────────────────

// cllGetDef —— py `d.get(k, default)`（**缺键**走 default；显式 `null` 走 None）。
func cllGetDef(d map[string]any, k string, def any) any {
	if v, ok := d[k]; ok {
		return v
	}
	return def
}

// cllPyStr —— py `str(v)` / `%s`（None ⇒ "None" · 布尔首字母大写 · 数值按 int/float 分形）。
func cllPyStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return x
	case json.Number:
		return cllPyNumStr(x.String())
	case float64:
		return cllPyFloatRepr(x)
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, cllPyRepr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(x))
		for _, k := range keys {
			parts = append(parts, cllPyRepr(k)+": "+cllPyRepr(x[k]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return fmt.Sprintf("%v", v)
}

// cllPyRepr —— 容器内的元素形态（字符串带单引号 · 与 py `repr` 同形）。
func cllPyRepr(v any) string {
	if s, ok := v.(string); ok {
		return "'" + s + "'"
	}
	if n, ok := v.(json.Number); ok {
		return cllPyNumStr(n.String())
	}
	return cllPyStr(v)
}

// cllPyNumStr —— JSON 数字字面量 → py str（整数字面量原样 · 浮点字面量走 py 浮点 repr）。
func cllPyNumStr(lit string) string {
	if strings.ContainsAny(lit, ".eE") {
		f, err := strconv.ParseFloat(lit, 64)
		if err == nil {
			return cllPyFloatRepr(f)
		}
		return lit
	}
	return lit
}

// cllPyFloatRepr —— py `repr(float)` 的常见形态（整数浮点补 `.0`）。
func cllPyFloatRepr(f float64) string {
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	if math.IsNaN(f) {
		return "nan"
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// cllPyIntVal —— py `int(v)`（布尔 ⇒ 0/1 · 数字截断 · 串按基 10 解析含下划线）。errm 非空 = py 会抛异常。
func cllPyIntVal(v any) (int, string) {
	switch x := v.(type) {
	case nil:
		return 0, "TypeError: int() argument must be a string, a bytes-like object or a number, not 'NoneType'"
	case bool:
		if x {
			return 1, ""
		}
		return 0, ""
	case json.Number:
		return cllPyIntParse(x.String())
	case float64:
		return int(x), ""
	case string:
		return cllPyIntParse(x)
	}
	return 0, "TypeError: int() argument must be a string, a bytes-like object or a number"
}

// cllPyIntParse —— py `int(s, 10)`（容忍首尾空白与数字间下划线）。
func cllPyIntParse(s string) (int, string) {
	orig := s
	s = cllPyStrip(s)
	body := s
	neg := false
	if strings.HasPrefix(body, "+") {
		body = body[1:]
	} else if strings.HasPrefix(body, "-") {
		neg = true
		body = body[1:]
	}
	digits := strings.ReplaceAll(body, "_", "")
	if digits == "" {
		return 0, "ValueError: invalid literal for int() with base 10: '" + orig + "'"
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return 0, "ValueError: invalid literal for int() with base 10: '" + orig + "'"
		}
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, "ValueError: invalid literal for int() with base 10: '" + orig + "'"
	}
	if neg {
		n = -n
	}
	return n, ""
}

// cllPyJSONErrMsg —— py `json.loads` 失败时的异常文本（只逐字覆盖**空/纯空白**这一常见档，其余为近似）。
func cllPyJSONErrMsg(s string) string {
	if cllPyStrip(s) == "" {
		return "Expecting value: line 1 column 1 (char 0)"
	}
	return "Expecting value: line 1 column 1 (char 0)"
}

// ── 文件系统谓词（py `os.path.isdir` / `os.path.exists` 都**跟随符号链接** ⇒ 用 os.Stat）─────────────

func cllIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func cllExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func cllIsAbs(p string) bool { return strings.HasPrefix(p, "/") }

// ── 件清单面 ────────────────────────────────────────────────────────────────────────────────

// cllGitMdFiles —— py `git_md_files`（cwd=root · capture · 退码 ≠0 ⇒ 失败文案）。
func cllGitMdFiles(root string) ([]string, string) {
	cmd := exec.Command("git", "-c", "core.quotepath=false", "ls-files", "*.md")
	cmd.Dir = root
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	cmd.Stdin = nil // ⇒ /dev/null（同 py stdin=DEVNULL）
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return nil, "git 跑不了（" + cllPyOSErr(err) + "）"
	}
	rc := 0
	if cmd.ProcessState != nil {
		rc = cmd.ProcessState.ExitCode()
	}
	if rc != 0 {
		return nil, fmt.Sprintf("git ls-files 退 %d（不是 git 仓 / 仓根不对）：%s", rc, cllStrStripMax(cllUniversalNL(se.String()), 200))
	}
	out := []string{}
	for _, ln := range cllPySplitLines(cllUniversalNL(so.String())) {
		s := cllPyStrip(ln)
		if s != "" {
			out = append(out, s)
		}
	}
	return out, ""
}

// cllReadTextFile —— py `open(path, encoding="utf-8")` 的读取面。返回 (文本, OSError 文本, 解码错)。
func cllReadTextFile(path string) (string, string, bool) {
	st, err := os.Stat(path)
	if err != nil {
		return "", cllPyOSErr(err), false
	}
	if st.IsDir() {
		return "", "[Errno 21] Is a directory: '" + path + "'", false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", cllPyOSErr(err), false
	}
	if !utf8.Valid(b) {
		return "", "", true
	}
	return cllUniversalNL(string(b)), "", false
}

// cllBuildFileList —— py `build_file_list`（`--files` 优先；否则 git 清单 [+ 旁树]）。
// ★ 返回非 nil 空切片表「空集合」，err 非空表「取不到」—— 两态分开表意（nil 不兼表两义）。
func cllBuildFileList(root, devdocs, filesFrom, cwd string) ([]string, string) {
	if filesFrom != "" {
		text, oerr, dec := cllReadTextFile(filesFrom)
		if dec {
			return nil, "清单读不了（" + filesFrom + "）：UnicodeDecodeError（非 UTF-8 ⇒ 口径差：py 侧会回溯 rc=1）"
		}
		if oerr != "" {
			return nil, "清单读不了（" + filesFrom + "）：" + oerr
		}
		items := []string{}
		for _, ln := range cllPySplitLines(text) {
			if cllPyStrip(ln) != "" && !strings.HasPrefix(ln, "#") {
				items = append(items, cllPyStrip(ln))
			}
		}
		out := []string{}
		for _, i := range items {
			if cllIsAbs(i) {
				out = append(out, i)
			} else {
				out = append(out, cllPyAbspath(cwd, cllPyJoin(root, i)))
			}
		}
		return out, ""
	}

	files, err := cllGitMdFiles(root)
	if err != "" {
		return nil, err
	}
	out := []string{}
	for _, f := range files {
		out = append(out, cllPyJoin(root, f))
	}
	if devdocs != "" {
		dd := cllPyAbspath(cwd, devdocs)
		if !cllIsDir(dd) {
			return nil, "旁树不是目录（" + dd + "）"
		}
		more, err2 := cllGitMdFiles(dd)
		if err2 != "" {
			return nil, "旁树 " + dd + "：" + err2
		}
		for _, f := range more {
			out = append(out, cllPyJoin(dd, f))
		}
	}
	return out, ""
}

// ── lychee 调用 + 产物解析 ─────────────────────────────────────────────────────────────────────

// cllRunLychee —— py `run_lychee`。返回 (数据, lychee 退码, 错误文案)；错误非空 ⇒ 数据无意义、退码按调用点弃用。
func cllRunLychee(files []string) (map[string]any, int, string) {
	if _, err := exec.LookPath(cllLychee); err != nil {
		return nil, 0, cllLychee + " 不在盘（`which lychee` 找不到）⇒ 缺判据 ⇒ 不给结论"
	}
	tf, err := os.CreateTemp("", "tmp*.txt")
	if err != nil {
		return nil, 0, cllLychee + " 跑不了（" + cllPyOSErr(err) + "）"
	}
	listpath := tf.Name()
	if _, err := tf.WriteString(strings.Join(files, "\n") + "\n"); err != nil {
		tf.Close()
		os.Remove(listpath)
		return nil, 0, cllLychee + " 跑不了（" + cllPyOSErr(err) + "）"
	}
	tf.Close()

	args := append(append([]string{}, cllLycheeArgs...), listpath)
	cmd := exec.Command(cllLychee, args...)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	cmd.Stdin = nil
	runErr := cmd.Run()
	os.Remove(listpath) // py finally: os.unlink(listpath)

	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		return nil, 0, cllLychee + " 跑不了（" + cllPyOSErr(runErr) + "）"
	}
	rc := 0
	if cmd.ProcessState != nil {
		rc = cmd.ProcessState.ExitCode()
	}
	if rc != 0 && rc != 2 {
		return nil, rc, fmt.Sprintf("%s 退 %d（不在 {0, 2} ⇒ 意外错，不是判据结果）：%s",
			cllLychee, rc, cllStrStripMax(cllUniversalNL(se.String()), 200))
	}
	stdout := cllUniversalNL(so.String())

	var v any
	dec := json.NewDecoder(strings.NewReader(stdout))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, rc, cllLychee + " 的 `--format json` 产物解析不了（形状变了 ⇒ 不猜）：" + cllPyJSONErrMsg(stdout)
	}
	data, ok := v.(map[string]any)
	if !ok {
		return nil, rc, "json 缺 `errors` 键（形状变了 ⇒ 不猜）"
	}
	for _, key := range []string{"errors", "error_map"} {
		if _, has := data[key]; !has {
			return nil, rc, "json 缺 `" + key + "` 键（形状变了 ⇒ 不猜）"
		}
	}
	return data, rc, ""
}

// ── 错条目面 ────────────────────────────────────────────────────────────────────────────────

type cllRec struct {
	rel   string // 仓根相对（输出序第一键 · py `rel_in_root(r[0], root)`）
	fname string // error_map 的键（绝对/词法原串）
	line  string // py `str(span.get("line","?"))`
	col   string // py `str(span.get("column","?"))`
	url   string // 去 `file://` 并 unquote 后的目标
}

// cllGetStr — 取值 → py `str()` 形态（缺键走 `?`；显式 null 走 "None"）。
func cllGetStr(m map[string]any, k string) string {
	v, ok := m[k]
	if !ok {
		return "?"
	}
	return cllPyStr(v)
}

// cllBrokenRecords —— py `broken_records`：error_map → 逐条记录，末尾**全局排序**
// （键 = rel, str(line), str(column), url ⇒ 与 error_map 的 dict 插入序无关）。
func cllBrokenRecords(data map[string]any, root, cwd string) ([]cllRec, string) {
	rows := []cllRec{}
	emv, _ := data["error_map"]
	em, ok := emv.(map[string]any)
	if !ok {
		if emv == nil {
			em = map[string]any{}
		} else {
			return nil, "lychee 的 `error_map` 不是对象（口径差：py 侧 `.items()` 会回溯 rc=1）"
		}
	}
	for fname, errsRaw := range em {
		errs, _ := errsRaw.([]any)
		for _, eRaw := range errs {
			e, ok := eRaw.(map[string]any)
			if !ok {
				return nil, "lychee 的 `error_map` 条目不是对象（口径差：py 侧 `.get()` 会回溯 rc=1）"
			}
			var span map[string]any
			if sv, has := e["span"]; has && sv != nil {
				if sm, ok := sv.(map[string]any); ok {
					span = sm
				} else {
					return nil, "lychee 的 `span` 不是对象（口径差：py 侧 `.get()` 会回溯 rc=1）"
				}
			} else {
				span = map[string]any{}
			}
			url := ""
			if uv, has := e["url"]; has {
				url = cllPyStr(uv)
			}
			if strings.HasPrefix(url, "file://") {
				url = cllPyUnquote(url[len("file://"):])
			}
			rel, _ := cllPyRelpath(cwd, fname, root)
			rows = append(rows, cllRec{rel: rel, fname: fname, line: cllGetStr(span, "line"), col: cllGetStr(span, "column"), url: url})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.rel != b.rel {
			return a.rel < b.rel
		}
		if a.line != b.line {
			return a.line < b.line
		}
		if a.col != b.col {
			return a.col < b.col
		}
		return a.url < b.url
	})
	return rows, ""
}

// cllFmtRec —— py `fmt_rec`：`件:行:列 → 目标`（件名按仓根相对印）。
func cllFmtRec(rec cllRec) string {
	return fmt.Sprintf("%s:%s:%s → %s", rec.rel, rec.line, rec.col, rec.url)
}

// cllOffsetOK —— py `offset_ok`：命中「基准偏移登记」⇒ 按登记基准重解析 + **现读存在性**复核。
func cllOffsetOK(rec cllRec, root, cwd string) (string, bool) {
	ent, found := cllBaseOffset[rec.rel]
	if !found {
		return "", false
	}
	base, why := ent[0], ent[1]
	raw, ok := cllPyRelpath(cwd, rec.url, cllPyDirname(rec.fname))
	if !ok {
		return "", false
	}
	cand := cllPyNormpath(cllPyJoin(root, base, cllPySplitFirst(raw, "#")))
	if cllExists(cand) {
		crel, _ := cllPyRelpath(cwd, cand, root)
		return fmt.Sprintf("%s（登记基准 `%s`）⇒ 复核命中 `%s`", why, base, crel), true
	}
	return "", false
}

// cllOffsetRegistryCheck —— py `offset_registry_check`：登记**过期** ⇒ 逐字点名（登记表的牙）。
// 只对「登记件在扫面域内」的条目判（`--files` 取证式单件扫描 / 夹具树 ⇒ 登记不参与本跑）。
func cllOffsetRegistryCheck(root string, files []string, recs []cllRec, cwd string) []string {
	stale := []string{}
	dom := map[string]bool{}
	for _, f := range files {
		dom[cllPyAbspath(cwd, f)] = true
	}
	hit := map[string]bool{}
	for _, r := range recs {
		hit[r.rel] = true
	}
	for _, rel := range cllBaseOffsetOrder {
		ent := cllBaseOffset[rel]
		base := ent[0]
		p := cllPyAbspath(cwd, cllPyJoin(root, rel))
		if !dom[p] {
			continue
		}
		if !cllIsDir(cllPyJoin(root, base)) {
			stale = append(stale, fmt.Sprintf("%s ⇒ **登记基准 `%s` 不是目录**（基准指空 ⇒ 撤或改登记）", rel, base))
			continue
		}
		if !hit[rel] {
			stale = append(stale, fmt.Sprintf("%s ⇒ **在扫面域内却一条原位错都没有**（链接已改回原位可达 / 该件不再需要偏移 ⇒ 请撤登记）", rel))
		}
	}
	return stale
}

// ── 主判据 ──────────────────────────────────────────────────────────────────────────────────

// cllJudge —— py `judge`：**唯一一处**判定逻辑（正控跑真目标、自检跑夹具，都走这里）。返回 (rc, 行清单)。
func cllJudge(root, devdocs, filesFrom string, quiet bool, cwd string) (int, []string) {
	lines := []string{}
	files, err := cllBuildFileList(root, devdocs, filesFrom, cwd)
	if err != "" {
		return 2, []string{"⛔ 不给结论：" + err}
	}
	if len(files) == 0 {
		return 2, []string{"⛔ 不给结论：扫面 **0 件**（空转不许给绿）—— 域 = 仓根 git 跟踪的 `*.md`" +
			"（+ `--devdocs` 旁树）；`--files` 给了空清单同判"}
	}
	data, lycheeRC, err := cllRunLychee(files)
	if err != "" {
		return 2, []string{"⛔ 不给结论：" + err}
	}
	nErr, ierr := cllPyIntVal(cllGetDef(data, "errors", json.Number("0")))
	if ierr != "" {
		return 2, []string{"⛔ 不给结论（口径差）：lychee 的 `errors` 不是整数（py 侧 int() 会回溯 rc=1）"}
	}
	if !quiet {
		lines = append(lines, fmt.Sprintf("扫面 %d 件 · lychee 退 %d · links %s（unique %s · successful %s · excludes %s）",
			len(files), lycheeRC, cllPyStr(cllGetDef(data, "total", nil)),
			cllPyStr(cllGetDef(data, "unique", nil)), cllPyStr(cllGetDef(data, "successful", nil)),
			cllPyStr(cllGetDef(data, "excludes", nil))))
		lines = append(lines, "`--offline` ⇒ 外链一律 `Excluded`（**不检查**）⇒ 「外链不进」是工具语义，不是本件的排除名单")
	}
	recs, rerr := cllBrokenRecords(data, root, cwd)
	if rerr != "" {
		return 2, []string{"⛔ 不给结论：" + rerr}
	}
	stale := cllOffsetRegistryCheck(root, files, recs, cwd)
	if len(stale) > 0 {
		out := append([]string{}, lines...)
		out = append(out, fmt.Sprintf("⛔ 不给结论：**基准偏移登记过期 %d 条** —— 登记指不到现实（逐字点名）：", len(stale)))
		for _, s := range stale {
			out = append(out, "   · "+s)
		}
		return 2, out
	}

	type pair struct {
		rec cllRec
		why string
	}
	exempt := []pair{}
	red := []cllRec{}
	for _, rec := range recs {
		why, ok := cllOffsetOK(rec, root, cwd)
		if ok {
			exempt = append(exempt, pair{rec, why})
		} else {
			red = append(red, rec)
		}
	}
	if len(red) > 0 {
		lines = append(lines, fmt.Sprintf("⇒ 本地断链 **%d 处**（errors %d；另有 **%d 处**为登记假红（基准偏移 · 已逐条复核）⇒ 不计）"+
			"⇒ rc=1（本步起步档 `tri-report` = 只报告、不阻断）", len(red), nErr, len(exempt)))
		shown := red
		if len(shown) > cllDetailCap {
			shown = shown[:cllDetailCap]
		}
		for _, r := range shown {
			lines = append(lines, "   "+cllFmtRec(r))
		}
		if len(red) > cllDetailCap {
			lines = append(lines, fmt.Sprintf("   …（另有 %d 处，见 lychee json 产物）", len(red)-cllDetailCap))
		}
		lines = append(lines, fmt.Sprintf("   ⊕ 已登记假红（基准偏移 · 已复核 · 不计入上面 %d 处）：", len(red)))
		for _, p := range exempt {
			lines = append(lines, fmt.Sprintf("      %s  ⟸ %s", cllFmtRec(p.rec), p.why))
		}
		return 1, lines
	}
	if len(exempt) > 0 {
		lines = append(lines, "   ⊕ 已登记假红（基准偏移 · 已逐条复核 · 不计入断链数）：")
		for _, p := range exempt {
			lines = append(lines, fmt.Sprintf("      %s  ⟸ %s", cllFmtRec(p.rec), p.why))
		}
	}
	tail := ""
	if len(exempt) > 0 {
		tail = fmt.Sprintf("；%d 处为登记假红（基准偏移 · 已复核）", len(exempt))
	}
	lines = append(lines, fmt.Sprintf("⇒ 无本地断链（errors %d%s）· rc=0", nErr, tail))
	return 0, lines
}

// ── 自检（成对负控 · 不碰真目标）────────────────────────────────────────────────────────────────

// cllSelfTestResult —— py `self_test` 的 (bad 文案表, 用例数, 正控数, 负控数)。
type cllSelfTestResult struct {
	bad     []string
	cases   int
	pos     int
	neg     int
	selfErr string
}

// cllSelfTest — py `self_test` 的 8 格（⑴-⑻）逐字复刻。夹具全落 os.MkdirTemp（**回收**在 defer）。
func cllSelfTest(cwd string) cllSelfTestResult {
	res := cllSelfTestResult{bad: []string{}}
	caseOf := func(name string, want, got int, kind string, out []string, needle string) {
		res.cases++
		if kind == "正控" {
			res.pos++
		} else {
			res.neg++
		}
		if want != got {
			res.bad = append(res.bad, fmt.Sprintf("%s ⇒ 实得 rc=%s（要 %s）", name, cllPyStr(json.Number(strconv.Itoa(got))), cllPyStr(json.Number(strconv.Itoa(want)))))
		}
		if needle != "" {
			found := false
			for _, ln := range out {
				if strings.Contains(ln, needle) {
					found = true
					break
				}
			}
			if !found {
				res.bad = append(res.bad, name+"：未印出 `"+needle+"`（逐条点名是本件的一半）")
			}
		}
	}

	t, err := os.MkdirTemp("", "cllft-")
	if err != nil {
		res.selfErr = "自检夹具建不了（" + cllPyOSErr(err) + "）"
		return res
	}
	defer os.RemoveAll(t)

	cllWrite := func(rel, body string) string {
		p := cllPyJoin(t, rel)
		_ = os.MkdirAll(cllPyDirname(p), 0o755)
		_ = os.WriteFile(p, []byte(body), 0o644)
		return p
	}

	// ⑴ 正控：本地链接指到存在的件 ⇒ 0
	cllWrite("good/a.md", "# t\n\n[ok](./b.md)\n")
	cllWrite("good/b.md", "# b\n")
	l1 := cllWrite("l1.txt", cllPyJoin(t, "good", "a.md")+"\n")
	rc, _ := cllJudge(t, "", l1, true, cwd)
	caseOf("⑴ 正控（本地链接都在）⇒ 0", 0, rc, "正控", nil, "")

	// ⑵ 负控：本地链接指向不存在的件 ⇒ 1（探针没牙即红）
	cllWrite("good/c.md", "# c\n\n[bad](./nope.md)\n")
	l2 := cllWrite("l2.txt", cllPyJoin(t, "good", "c.md")+"\n")
	rc, out := cllJudge(t, "", l2, true, cwd)
	caseOf("⑵ 负控（本地断链）⇒ 1", 1, rc, "负控", out, "nope.md")

	// ⑶ 负控：扫面 0 件 ⇒ 2（★ 空转不许给绿）
	l3 := cllWrite("l3.txt", "# 注释行不算件\n\n")
	rc, _ = cllJudge(t, "", l3, true, cwd)
	caseOf("⑶ 负控（空转 0 件）⇒ 2", 2, rc, "负控", nil, "")

	// ⑷ 正控：只放外链 ⇒ 0（`--offline` 的固有语义）
	cllWrite("good/d.md", "# d\n\n[ext](https://example.invalid/x)\n")
	l4 := cllWrite("l4.txt", cllPyJoin(t, "good", "d.md")+"\n")
	rc, _ = cllJudge(t, "", l4, true, cwd)
	caseOf("⑷ 正控（只有外链）⇒ 0", 0, rc, "正控", nil, "")

	// ⑸ 负控：lychee 不在盘 ⇒ 2（缺判据 ≠ 绿）
	oldPath, hadPath := os.LookupEnv("PATH")
	_ = os.Setenv("PATH", "")
	rc, _ = cllJudge(t, "", l1, true, cwd)
	if hadPath {
		_ = os.Setenv("PATH", oldPath)
	} else {
		_ = os.Unsetenv("PATH")
	}
	caseOf("⑸ 负控（lychee 不在盘）⇒ 2", 2, rc, "负控", nil, "")

	// ⑹ 正控（登记豁免口）：逐字复刻真目标形状 publish/docs/README.md（链接按公开树仓根写）
	cllWrite("publish/docs/QUICKSTART.en.md", "# q\n")
	cllWrite("publish/docs/README.md", "# r\n\n见 [q](docs/QUICKSTART.en.md)。\n\n`docs/NEVER-A-FILE.md` 是反引号里的路径。\n\n> 引文：详见 docs/ALSO-NEVER.md 一节。\n")
	l5 := cllWrite("l5.txt", cllPyJoin(t, "publish", "docs", "README.md")+"\n")
	rc, out = cllJudge(t, "", l5, true, cwd)
	caseOf("⑹ 正控（登记件 · 按基准复核命中）⇒ 0", 0, rc, "正控", nil, "")
	if cllContains(out, "NEVER") {
		res.bad = append(res.bad, "⑹ 正控：反引号/引文里的路径被当链接报了（本件不许越界到非链接文本）")
	}

	// ⑺ 负控（豁免口的牙）：登记件里塞一条**按基准也指不到**的链接 ⇒ 照旧判红且点名
	cllWrite("publish/docs/README.md", "# r\n\n见 [q](docs/QUICKSTART.en.md)。\n\n也见 [x](docs/NOPE.md)。\n")
	rc, out = cllJudge(t, "", l5, true, cwd)
	caseOf("⑺ 负控（登记件 · 基准复核落空）⇒ 1", 1, rc, "负控", out, "NOPE.md")

	// ⑻ 负控（登记表的牙 · 登记过期）：另一株夹具树里登记件**原位就可达** ⇒ 判「登记过期」rc=2 并点名
	t2, err2 := os.MkdirTemp("", "cllft-")
	if err2 != nil {
		res.selfErr = "自检夹具建不了（" + cllPyOSErr(err2) + "）"
		return res
	}
	defer os.RemoveAll(t2)
	cllWrite2 := func(rel, body string) string {
		p := cllPyJoin(t2, rel)
		_ = os.MkdirAll(cllPyDirname(p), 0o755)
		_ = os.WriteFile(p, []byte(body), 0o644)
		return p
	}
	cllWrite2("publish/docs/QUICKSTART.en.md", "# q\n")
	cllWrite2("publish/docs/README.md", "# r\n\n见 [q](QUICKSTART.en.md)。\n")
	l6 := cllWrite2("l6.txt", cllPyJoin(t2, "publish", "docs", "README.md")+"\n")
	rc, out = cllJudge(t2, "", l6, true, cwd)
	res.cases++
	res.neg++
	if rc != 2 {
		res.bad = append(res.bad, fmt.Sprintf("⑻ 负控（登记过期：登记件在域内 0 命中）⇒ 2 ⇒ 实得 rc=%d（要 2）", rc))
	}
	if !cllContains(out, "登记过期") && !cllContains(out, "请撤登记") {
		res.bad = append(res.bad, "⑻ 负控：登记过期没被逐字点名（登记表的牙不见了）")
	}
	return res
}

func cllContains(lines []string, needle string) bool {
	for _, ln := range lines {
		if strings.Contains(ln, needle) {
			return true
		}
	}
	return false
}

// cllSelfTestMain —— py `self_test()` 的 stdout/退码面。
func cllSelfTestMain(stdout io.Writer, cwd string) int {
	res := cllSelfTest(cwd)
	if res.selfErr != "" {
		fmt.Fprintln(stdout, "✗ 自检不过（1 格）—— **拒绝扫真目标**：")
		fmt.Fprintln(stdout, "   · "+res.selfErr)
		return 2
	}
	if len(res.bad) > 0 {
		fmt.Fprintf(stdout, "✗ 自检不过（%d 格）—— **拒绝扫真目标**：\n", len(res.bad))
		for _, b := range res.bad {
			fmt.Fprintln(stdout, "   · "+b)
		}
		return 2
	}
	fmt.Fprintf(stdout, "✓ 自检全过（%d 格：正控 %d + 负控 %d · 成对）\n", res.cases, res.pos, res.neg)
	return 0
}

// ── 缺口 `Q-176`（门禁面 · 统一口）：本门**没有**机器读面 ⇒ 给了 `--json` **不许沉默** ────────────

// cllJSONFields —— 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处 · 本门恒空）。
var cllJSONFields = []string{}

// cllRefuseJSON —— py `refuse_json_without_machine_face`：给了 `--json` ⇒ 逐字说明后退 2；没给 ⇒ 继续。
func cllRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
	hit := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return 0, false
	}
	quoted := make([]string, 0, len(hit))
	for _, h := range hit {
		quoted = append(quoted, "`"+h+"`")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(quoted, " · "))
	fieldTable := "无（本门只出人读面）"
	if len(cllJSONFields) > 0 {
		qs := make([]string, 0, len(cllJSONFields))
		for _, f := range cllJSONFields {
			qs = append(qs, "`"+f+"`")
		}
		fieldTable = strings.Join(qs, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fieldTable)
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return 2, true
}

// ── argparse 面（只覆盖本件声明的五个旗标 · ✗ 不实现缩写/`-xy` 合并等 argparse 附加形态）──────────

type cllArgs struct {
	root     string
	devdocs  string
	files    string
	selfTest bool
	help     bool
}

// cllDefaultRoot —— (a) 默认根：从 cwd 上溯 ≤8 层找含 `scripts/gates` 的目录，找不到回落 abspath(cwd)。
func cllDefaultRoot(cwd string) string {
	d := cwd
	for i := 0; i < 8; i++ {
		if cllIsDir(cllPyJoin(d, "scripts", "gates")) {
			return d
		}
		parent := cllPyDirname(d)
		if parent == d || parent == "" {
			break
		}
		d = parent
	}
	return cllPyAbspath(cwd, ".")
}

func cllParseArgs(argv []string, cwd string) (cllArgs, string) {
	a := cllArgs{root: cllDefaultRoot(cwd)}
	extra := []string{}
	i := 0
	for i < len(argv) {
		tok := argv[i]
		if !strings.HasPrefix(tok, "-") || tok == "-" {
			extra = append(extra, tok)
			i++
			continue
		}
		name := tok
		val := ""
		hasVal := false
		if eq := strings.Index(name, "="); eq >= 0 {
			val = name[eq+1:]
			name = name[:eq]
			hasVal = true
		}
		switch name {
		case "-h", "--help":
			// argparse 的 help action 在解析途中即触发（先于「未知参数」汇总）⇒ 立即返回。
			a.help = true
			return a, ""
		case "--root", "--devdocs", "--files":
			if !hasVal {
				if i+1 >= len(argv) || cllLooksLikeFlag(argv[i+1]) {
					return a, "argument " + name + ": expected one argument"
				}
				val = argv[i+1]
				i++
			}
			switch name {
			case "--root":
				a.root = val
			case "--devdocs":
				a.devdocs = val
			case "--files":
				a.files = val
			}
		case "--self-test":
			if hasVal {
				return a, "argument --self-test: ignored explicit argument '" + val + "'"
			}
			a.selfTest = true
		default:
			extra = append(extra, tok)
		}
		i++
	}
	if len(extra) > 0 {
		return a, "unrecognized arguments: " + strings.Join(extra, " ")
	}
	return a, ""
}

// cllLooksLikeFlag —— argparse：值位若看着像选项 ⇒ 「expected one argument」（负数字面量除外）。
func cllLooksLikeFlag(tok string) bool {
	if !strings.HasPrefix(tok, "-") || tok == "-" {
		return false
	}
	if len(tok) >= 2 && tok[1] >= '0' && tok[1] <= '9' {
		return false
	}
	return true
}

// cllMain —— py `main()` 的同形主流程。
func cllMain(stdout, stderr io.Writer, argv []string, cwd string) int {
	if rc, hit := cllRefuseJSON(stderr, argv); hit {
		return rc
	}
	a, perr := cllParseArgs(argv, cwd)
	if perr != "" {
		fmt.Fprint(stderr, cllUsageLine)
		fmt.Fprintf(stderr, "%s: error: %s\n", cllProg, perr)
		return 2
	}
	if a.help {
		fmt.Fprint(stdout, cllHelpText)
		return 0
	}
	if a.selfTest {
		return cllSelfTestMain(stdout, cwd)
	}
	if !cllIsDir(a.root) {
		fmt.Fprintf(stdout, "⛔ 不给结论：仓根不是目录（%s）\n", a.root)
		return 2
	}
	rc, lines := cllJudge(a.root, a.devdocs, a.files, false, cwd)
	for _, ln := range lines {
		fmt.Fprintln(stdout, ln)
	}
	return rc
}

// checkLinksLycheeGateMain —— py `main()` 同形入口（argv 与 py `sys.argv[1:]` 对齐）。
func checkLinksLycheeGateMain(stdout, stderr io.Writer, argv []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return cllMain(stdout, stderr, argv, cwd)
}
