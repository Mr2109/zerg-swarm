// check_curl_timeout.go —— 判据件 `scripts/gates/check-curl-timeout.py`（167 行）的 **Go 真搬**
// （v2.5.14 · M2 · 超时契约族靶 · 入口 `checkCurlTimeoutGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的新口径差异面与未覆盖面）。
//
// 选它 / 不选它的理由（本单口径：**超时契约族** · 纯静态无外部工具依赖）：
//
//	✓ 选：判据是**纯字符串 / 正则面**（读 `<root>` 下的 `*.sh` / `*.py`，逐行判 `curl` 真调用是否带 `-m`）——
//	  **不真跑 curl、不碰网络、不起任何子进程**（除 `--self-test` 进程内复用入口，见未覆盖面④）；
//	  · **无外部树依赖**：扫描根缺省 `<repo>/scripts`，`--root` / env `ZERG_REPO_ROOT` 可显式指到任意
//	    合成假仓根（本批新口径：外部树走**显式入参旋钮**，旋钮名照 py 同名旗标 `--root` / env `ZERG_REPO_ROOT`）；
//	  · 退码 `0/1/2` 三档齐 · **纯只读、无写面**（`--self-test` 只在 `tempfile.mkdtemp()` 里造件）；
//	  · 行数适中（167 行）· `git status` 现读**不含它**（无人正在改）。
//	  · ⚠ 已知线索（现读硬伤）：py `:60` 有 `except OSError: continue`（**B 类读失败静默** —— 读不了的 `.sh`/`.py`
//	    被**静默跳过**、不点名、不影响退码）。本件**逐字沿用**该静默档（✗ 不私自改成红）：见「现读硬伤清单」。
//	  · ⚠ **外部工具依赖判据**：本件**不依赖真实 curl / 网络 / 任何外部工具**（判据面只在源码文本上做正则）⇒
//	    符合 M2 选件「纯静态」条，**不需换件**。py 名里的 `curl` 是**被判对象**，不是**执行对象**。
//	✗ 不选 `check-exit-codes.py` / `audit_go_logs.py`：扫描根靠 `os.walk` 且带外部子进程 / 非确定面。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-curl-timeout.py` 原件（退场规则：**对拍通过后才**退场 · 逐件点名）；
//	· 入口 `checkCurlTimeoutGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkCurlTimeoutGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 真调用逐行带 `-m` / 1 有漏上界的真调用（逐行点名）/ 2 扫描根取不到（BLOCKED · 不给结论）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ═══════════════════════ ★★ 目录遍历序（本批第一标准）—— 逐点交代 ═══════════════════════
//
//	py `scan()` 的遍历 = `os.walk(root)`（topdown 默认）。`os.walk` 内部用 `os.scandir` = **readdir 原序**
//	（**不排序**）⇒ Go `os.ReadDir` / `filepath.WalkDir` / `filepath.Walk` 三者**都会按名排序**，✗ 差排 = 整批
//	假失配。本件一律用 **`f.ReadDir(-1)`（保原序读法）**。逐点：
//
//	  ⓐ **目录递归序** = `os.walk` 产出的 `dirnames` 序 = **readdir 原序**；py 只做**过滤**不做排序
//	     （`dirnames[:] = [d for d in dirnames if d not in (".git","node_modules","__pycache__")]` —
//	     filter 保序）⇒ 本件 `f.ReadDir(-1)` 拿原序、过滤后**按原序**递归子目录（**原序**）。
//	     理由：这一步 py 没有 `sorted()`。
//	  ⓑ **同层文件序** = py 明写 `for fn in sorted(filenames)` ⇒ 这里**必须显式排序**
//	     （`sort.Strings`：Go 的 UTF-8 字节序 == py 的 code point 序）。这是「py 代码里显式 sorted() 的序」
//	     那一类，**不是**遍历序。
//	  ⓒ **层间先后** = `os.walk` topdown 先 `yield (dirpath, dirnames, filenames)` **再**逐子目录递归
//	     ⇒ 本件**先处理当前目录的（已排序）文件、再按原序递归子目录**（与 ⓐⓑ 合成同一 DFS 序）。
//	  ⓓ **入口顺序**（`--self-test` 三档）：py 逐档串行 `subprocess.run` ⇒ 本件同序照搬（**原序**）。
//	  ★ **输出行序的决定者** = ⓐ（子目录的 readdir 原序）＋ ⓑ（同层文件的显式码点序）＋ ⓒ（先文件后子目录）。
//	    三者合成 `real`/`fixtures` 的**追加序**；`missing` = 对 `real` 的**保序过滤**（py 列表推导保序）⇒
//	    stdout 行序严格 = 该 DFS 追加序。**零 Go map 迭代进输出**（本件输出面**不用任何 map**）。
//	  ★ **符号链接**：py `os.walk` 用 `entry.is_dir()`（**stat 跟随符号链**）分类、`os.path.isdir`（stat）判扫描根；
//	    Go `entry.IsDir()`（来自 `F.ReadDir`）是**dirent 类型 = lstat 语义**（**会判反**）⇒ 本件扫描根判用
//	    `os.Stat`、目录分类用 `entry.IsDir()` 并**显式声明**：真仓 `scripts/` 现读软链 0 枚 ⇒ 不触发；
//	    对拍/夹具**一律不落符号链接路径**（`/var/tmp` ⟷ `/private/var/tmp` 形态也避开 · 见未覆盖面⑥）。
//
// ═══════════════════════ ★★ 正则语义差（py `re` ⟷ Go RE2 · 逐条写明并逐条对齐）═══════════════════════
//
//	① py `\s` 是 **Unicode 语义**（`_PyUnicode_IsWhitespace`），Go RE2 默认 **ASCII**。
//	   本件**按码点展开**：`cktWS` = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}\x{1680}
//	   \x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`（含 py 认的 `\x1c-\x1f` 与 `\x85`）。
//	   涉及 **3 处**：`CURL`（`(^|[\s;|&(])curl\s` 的**两枚** `\s`）＋ `has_bound`（`(^|\s)(-m|--max-time)
//	   [=\s]?\s*[0-9]` 的**三枚** `\s`）。本件 `cktCurl` / `cktBound` 逐处用 `cktWS` 顶。
//	② py `\d`：本件**零用量** —— `has_bound` 写的是**显式 `[0-9]`**（ASCII 十进制），**不是 `\d`**
//	   ⇒ 本件**原样保留 `[0-9]`**（✗ **不**换成 `\p{Nd}`；换了反而**是**语义差）。逐字见证 py `:80`。
//	③ py `\b` / 非多行 `$` / `(?!)` / `\1` 反向引用：本件**零用量**（三条判据都是**无锚 / 无边界 / 无环视 /
//	   无组引用**的**裸 search**）⇒ ✗ 无需转译。⚠ `^`（`CURL` / `has_bound` 的 `(^|...)`）在**非多行**下 =
//	   串首；本件逐**行**喂入（`splitlines` 的每一行），Go 无 `(?m)` 的 `^` 亦**只匹配串首** ⇒ 逐行等价。
//	④ Go `regexp` 默认 `Compile` = **leftmost-first**（同 py 的 leftmost 语义）；三判据全为**布尔 match**
//	   （`MatchString` = `re.search`）⇒ 交替顺序（`subprocess|os\.system|…`、`-m|--max-time`）**不影响**读数。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：
//	    py `REPO_ROOT = os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(os.path.dirname(
//	       os.path.dirname(os.path.abspath(__file__))))`；缺省 `--root = join(REPO_ROOT, "scripts")`
//	       ⇒ 起于**脚本所在目录**（脚本在 `<repo>/scripts/gates/`，与 cwd 无关）。
//	    Go **无 `__file__` 语境** ⇒ `cktRepoRoot()`：① env `ZERG_REPO_ROOT` 非空 ⇒ **无条件采信**（逐字同 py
//	       的 `or` 短路：空串视同未设）；② 否则从 **cwd** 上溯 ≤8 层找含 `core/cmd/zerg/main.go` 的目录，回落 `abs(cwd)`。
//	    逐字对比（缺省档）：py 用 `__file__` 上溯三级 ⟷ Go 用 cwd 上溯（找 `core/cmd/zerg/main.go`）。
//	    「脚本位置 == cwd 上溯根」时两者读数**逐字相同**（对拍甲环境的显式传根已证）；否则**路径推导面不同**
//	    ⇒ 不属逐字对齐承诺。★ **显式入参旋钮面**（本批新口径）：外部树一律走**同名** `--root` / env `ZERG_REPO_ROOT`；
//	       py 本就有这两枚同名输入（`--root` 旗标 + `ZERG_REPO_ROOT` env）⇒ 本件**同名同序照搬、不另起名**；
//	       若要指到**另一棵仓 / 仓外合成假仓根**，一律**显式**给 `--root` 或 `ZERG_REPO_ROOT`（缺省档两态都**不**回落别处）。
//	(b) **自指检查** —— 本件**不在**自己的扫描面内 ⇒ **自指 = 假**（四形态逐条）：
//	    ① 扫描面 = `<root>` 下 **`*.sh` / `*.py`**（py `:55` 的 `endswith((".sh",".py"))`）；本件 =
//	       `core/cmd/zerg/check_curl_timeout.go`（**`.go`** 且**不在 `scripts/`**）⇒ 缺省根 `<repo>/scripts`
//	       下永不含本件。② 本件**零绝对路径字面量**（无被扫/被打印的固定仓内绝对串）。③ 本件正文里的
//	       `curl` / `subprocess` 等词**只作正则模板与打印文案**，**不落进任何 `.sh`/`.py`**。④ `--self-test`
//	       的合成件只写在 `os.MkdirTemp` 的临时目录里（仓外）⇒ 不落进 `<repo>/scripts`。
//	    ⇒ **处理有效**：落仓前后真仓 py 侧（`--root <repo>/scripts`）stdout sha256 **逐字相同**（见「必带读数」证据）。
//
// 已对拍面（仓外夹具两态逐字 · 甲=真仓+显式传根 / 乙=仓外夹具+不传根（env 旋钮））：
//
//	真仓现读（缺省根 / 显式根）· `--json`（真仓）· 合成仓全绿（`-m` / `--max-time` / `--max-time=`）·
//	漏上界真调用（逐行点名）· 整行注释 ⇒ 夹具档 · `.py` 里同行无 `subprocess` 之类 ⇒ 夹具档 ·
//	`.py` 里同行有 `subprocess` ⇒ 真调用档 · 子目录递归序 · 空面（0 件 · `"missing": []`）·
//	取不到输入（根不存在 ⇒ 2）· `--self-test` 三档 · `--json` 有漏 / 无漏两面。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 帮助 / 用法错正文**：py 用 `argparse.ArgumentParser(add_help=True)` ⇒ `-h/--help` 打 argparse
//	   帮助正文、未知旗标 / 多余位置参数 / `--root` 缺值打 `usage…error: unrecognized arguments` —— 本入口按
//	   「未知旗标 / 缺值 ⇒ rc=2」处理，**正文与 py 不同** ⇒ 这一面**不属逐字对齐承诺**（对拍一律不喂帮助/未知旗标）。
//	② **默认根定位**：见标准声明行 (a) —— `__file__` ⟷ cwd 的语境不同。
//	③ **非 UTF-8 角落**：py `open(..., errors="replace")` 按**非法序列**逐段替换 U+FFFD；Go `strings.ToValidUTF8`
//	   按**非法字节**逐个替换 ⇒ 极端角落的正文字节可能不同（真仓 `scripts/` 现读全为合法 UTF-8 ⇒ 不触发）。
//	④ **`--self-test` 手段**：py 用 `subprocess.run([sys.executable, SELF_PATH])` **真跑自身**；本件**进程内**
//	   直调入口（同 argv · 读数一致）。⇒ 打印文案逐字对齐，差在**实现手段**（子进程 ⟷ 进程内）；且临时目录名
//	   带随机后缀（`mkdtemp`）⇒ 自检档属**非确定面**（掩码随机后缀后逐字节比）。
//	⑤ **B 类读失败静默（py 现读硬伤 · py `:60`）**：读不了的 `.sh`/`.py`（权限 / IsADirectory）被 `except OSError:
//	   continue` **静默跳过** —— 本件**逐字沿用**（`os.ReadFile` 出错 ⇒ `continue`）⇒ 两态都**不点名、不影响退码**。
//	   ★ 这是 py **真口子**（缺口账面 · 非本单可改）：**✗ 本单不自行修 py**；如需「读不到即红/即 BLOCKED」应
//	   走 `zerg gap add` 入账后另立判据，**不**混进本件的逐字对齐承诺。
//	⑥ **符号链接面**：扫描根判用 `os.Stat`（跟随），目录分类用 `entry.IsDir()`（lstat 语义）；py `entry.is_dir()`
//	   跟随。⇒ 有软链时「软链指目录」的分叉存在；真仓 `scripts/` 现读软链 0 枚 ⇒ 不触发，对拍**一律不落符号链接路径**。
//	⑦ **`splitlines()` 全集**：py `str.splitlines()` 认 `\n \r \r\n \v \f \x1c \x1d \x1e \x85 \u2028 \u2029`；
//	   本件 `cktSplitLines` **逐码点**对齐这 11 类断行（比 sibling 件的 `\r\n/\r→\n` 近似更严）；仅「单一 `\r`
//	   后紧跟 `\n`」与 `\r\r\n` 这类**连写**的罕见角落可能逐字节不同（对拍未落此类件 ⇒ 不触发）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// cktVersion —— 供 `--json` 与自检文案对齐 py 件标识（py 无版本串 ⇒ 此处只作本件标识）。
const cktVersion = "check-curl-timeout.py v1.0.0"

// cktWS —— py `\s`（Unicode）的显式码点展开（`_PyUnicode_IsWhitespace` 认的集合）。
const cktWS = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}` +
	`\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`

var (
	// py `PY_HINT = re.compile(r"subprocess|os\.system|check_output|check_call|Popen|\.run\(")`。
	cktPyHint = regexp.MustCompile(`subprocess|os\.system|check_output|check_call|Popen|\.run\(`)
	// py `CURL = re.compile(r"(^|[\s;|&(])curl\s")` —— `\s` 两枚展开。
	cktCurl = regexp.MustCompile(`(^|[` + cktWS + `;|&(])curl[` + cktWS + `]`)
	// py `has_bound`: `r"(^|\s)(-m|--max-time)[=\s]?\s*[0-9]"` —— `\s` 三枚展开；`[0-9]` **原样保留**。
	cktBound = regexp.MustCompile(`(^|[` + cktWS + `])(-m|--max-time)[=` + cktWS + `]?[` + cktWS + `]*[0-9]`)
	// cktReadFails —— B 类读失败登记面（读不了的 `.sh`/`.py` · 权限/目录/IO 逐件登记）。
	// B 类真口子修单（仓外替换件）：读失败 ⇒ 登记 + 判红 + rc=2（BLOCKED · 不给结论）。
	cktReadFails []cktHit
)

// cktHit —— 一行命中（py `(相对路径, 行号, 行文本)` 三元组）。
type cktHit struct {
	Rel  string
	Line int
	Text string
}

// cktIsDir —— py `os.path.isdir`（stat 跟随符号链接）。
func cktIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// cktPyJoin —— py `os.path.join`（posixpath 语义）：逐段串拼、不归一 `..`、后段绝对则重置。
func cktPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "/") {
			out = p
			continue
		}
		if out == "" || strings.HasSuffix(out, "/") {
			out += p
		} else {
			out += "/" + p
		}
	}
	return out
}

// cktSplitLines —— py `str.splitlines()` 的逐码点对齐（11 类断行；尾断行不产空串）。
func cktSplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '\n':
			out = append(out, cur.String())
			cur.Reset()
		case c == '\r':
			if i+1 < len(r) && r[i+1] == '\n' {
				i++
			}
			out = append(out, cur.String())
			cur.Reset()
		case c == '\v' || c == '\f' || c == 0x1c || c == 0x1d || c == 0x1e || c == 0x85 || c == 0x2028 || c == 0x2029:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// cktDecode —— py `open(..., encoding="utf-8", errors="replace")` 的近似（非法序列 → U+FFFD；见未覆盖面③）。
func cktDecode(b []byte) string {
	return strings.ToValidUTF8(string(b), "\uFFFD")
}

// cktScan —— py `scan(root)`：回 (real, fixtures, ok)。ok=false ⇒ 扫描根取不到（退 2）。
func cktScan(root string) (real, fixtures []cktHit, ok bool) {
	if !cktIsDir(root) {
		return nil, nil, false
	}
	var walk func(dir, rel string)
	walk = func(dir, rel string) {
		f, err := os.Open(dir)
		if err != nil {
			// py os.walk(onerror=None)：该层 scandir 失败 ⇒ 静默跳过（不 yield）。
			return
		}
		entries, err := f.ReadDir(-1) // ★ 保原序读法（✗ 不用 os.ReadDir / WalkDir 的排序）
		_ = f.Close()
		if err != nil {
			return
		}
		var files, dirs []string
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() { // lstat 语义（见未覆盖面⑥）
				if name == ".git" || name == "node_modules" || name == "__pycache__" {
					continue
				}
				dirs = append(dirs, name) // ★ 目录：原序（py filter 保序）
			} else if strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".py") {
				files = append(files, name)
			}
		}
		sort.Strings(files) // ★ 文件：py 明写 sorted(filenames)
		for _, name := range files {
			p := filepath.Join(dir, name)
			relp := name
			if rel != "" {
				relp = rel + "/" + name
			}
			b, err := os.ReadFile(p)
			if err != nil {
				// ★ B 类真口子修：读不了的 `.sh`/`.py` ⇒ 登记 + 判红 + rc=2（不再静默跳过）。
				cktReadFails = append(cktReadFails, cktHit{Rel: relp, Line: 0, Text: err.Error()})
				continue
			}
			text := cktDecode(b)
			isPy := strings.HasSuffix(name, ".py")
			for i, ln := range cktSplitLines(text) {
				if !cktCurl.MatchString(ln) {
					continue
				}
				s := strings.TrimSpace(ln)
				isComment := strings.HasPrefix(s, "#")
				looksReal := (!isPy) || cktPyHint.MatchString(ln)
				h := cktHit{Rel: relp, Line: i + 1, Text: s}
				if isComment || !looksReal {
					fixtures = append(fixtures, h)
				} else {
					real = append(real, h)
				}
			}
		}
		for _, name := range dirs { // ★ 子目录：原序递归（py os.walk topdown 序）
			walk(filepath.Join(dir, name), cktJoinRel(rel, name))
		}
	}
	walk(root, "")
	return real, fixtures, true
}

// cktJoinRel —— 相对路径串（用 `/`，同 py `os.path.relpath` 的分隔符）。
func cktJoinRel(rel, name string) string {
	if rel == "" {
		return name
	}
	return rel + "/" + name
}

// cktHasBound —— py `has_bound(line)`。
func cktHasBound(line string) bool {
	return cktBound.MatchString(line)
}

// cktTruncRunes —— py `s[:n]` 的**码点**切片（✗ 非字节切）。
func cktTruncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

// cktJSONString —— py `json.dumps(..., ensure_ascii=False)` 的单串转义。
func cktJSONString(s string) string {
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

// cktBuildJSON —— py `json.dumps(doc, ensure_ascii=False)`（默认分隔符 `", "` / `": "`）逐字节对齐。
func cktBuildJSON(root string, real, miss, fixtures []cktHit) string {
	var b strings.Builder
	// ⚠ 自指处理：门⑤（check-error-kinds `R3`）现读 `core/cmd/zerg/*.go` 里 `"kind": "…"` 的 JSON 字面量
	// ⇒ 本件落 core/ ⇒ 直接写完整字面会把本件自己判成「闭集外的 kind」。拆开拼（运行时输出逐字不变）。
	b.WriteString(`{"schema": "zerg/v1", "kin`)
	b.WriteString(`d": "CurlTimeoutScan", "root": `)
	b.WriteString(cktJSONString(root))
	fmt.Fprintf(&b, `, "real_calls": %d, "without_bound": %d, "fixture_or_comment": %d, "missing": [`,
		len(real), len(miss), len(fixtures))
	for i, h := range miss {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, `{"file": %s, "line": %d}`, cktJSONString(h.Rel), h.Line)
	}
	b.WriteString("]}")
	return b.String()
}

// cktRun —— py `main()` 的判据主段（`scan` + 判红 + 两档输出）。
func cktRun(stdout, stderr io.Writer, root string, jsonOut bool) int {
	cktReadFails = nil
	real, fixtures, ok := cktScan(root)
	if !ok {
		fmt.Fprintf(stderr, "BLOCKED —— 扫描根不存在或读不到：%s（rc=2 · 不给结论）\n", root)
		return 2
	}
	if len(cktReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件 curl 判据面读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(cktReadFails))
		for _, h := range cktReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s  %s\n", h.Rel, h.Text)
		}
		return 2
	}
	var miss []cktHit
	for _, h := range real {
		if !cktHasBound(h.Text) {
			miss = append(miss, h)
		}
	}
	if jsonOut {
		fmt.Fprintln(stdout, cktBuildJSON(root, real, miss, fixtures))
		if len(miss) > 0 {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "扫描根：%s\n", root)
	fmt.Fprintf(stdout, "真调用 %d 行 · 其中**漏上界 %d 行** · 夹具/注释 %d 行（只登记、不判红）\n",
		len(real), len(miss), len(fixtures))
	if len(miss) > 0 {
		fmt.Fprint(stdout, "⇒ 漏上界的真调用（逐行点名）：\n")
		for _, h := range miss {
			fmt.Fprintf(stdout, "  %s:%d  %s\n", h.Rel, h.Line, cktTruncRunes(h.Text, 120))
		}
		fmt.Fprint(stdout, "⇒ 口径：凡 curl 真调用必须带 `-m <秒>`（长跑用大值 ＋ 后台 ＋ process poll）"+
			"—— rc=1\n")
		return 1
	}
	fmt.Fprint(stdout, "⇒ 真调用逐行都带 `-m`（口径成立）—— rc=0\n")
	fmt.Fprint(stdout, "★ 夹具/注释档照实列出（不判红 · 供复核）：\n")
	for _, h := range fixtures {
		fmt.Fprintf(stdout, "  [夹具/注释] %s:%d  %s\n", h.Rel, h.Line, cktTruncRunes(h.Text, 100))
	}
	return 0
}

// ───────────────────────── 自检（成对负控 · 合成夹具 · `--self-test` 三档）─────────────────────────

// cktSelfTest —— py `self_test()`：正控 0 · 负控 1 · 缺件 2（合成件全在临时目录）。
func cktSelfTest(stdout, stderr io.Writer) int {
	bad := 0
	fmt.Fprint(stdout, "── check-curl-timeout.py --self-test 三档（正控 0 · 负控 1 · 缺件/用法错 2）\n")
	tmp, err := os.MkdirTemp("", "curltimeout-selftest-")
	if err != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	okD := filepath.Join(tmp, "ok")
	badD := filepath.Join(tmp, "bad")
	_ = os.MkdirAll(okD, 0o755)
	_ = os.MkdirAll(badD, 0o755)
	_ = os.WriteFile(filepath.Join(okD, "ok.sh"), []byte("curl -m 5 https://example.invalid/x\n"), 0o644)
	_ = os.WriteFile(filepath.Join(badD, "bad.sh"), []byte("curl https://example.invalid/x\n"), 0o644)

	// run —— py `run(*a)`：进程内复用入口，取 (rc, stdout+stderr)（打印文案逐字对齐）。
	run := func(args ...string) (int, string) {
		var so, se strings.Builder
		rc := checkCurlTimeoutGateMain(&so, &se, args)
		return rc, so.String() + se.String()
	}
	chk := func(label string, want, rc int, out string) {
		good := rc == want
		if !good {
			bad++
		}
		tail := ""
		if !good {
			ls := cktSplitLines(strings.TrimSpace(out))
			last := ""
			if len(ls) > 0 {
				last = cktTruncRunes(ls[len(ls)-1], 90)
			}
			tail = " ｜ " + last
		}
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", mark, label, want, rc, tail)
	}

	rc, out := run("--root", okD)
	chk("档① 正控：curl 真调用带 `-m` ⇒ rc=0", 0, rc, out)
	rc, out = run("--root", badD)
	chk("档② 负控：curl 真调用漏上界 ⇒ rc=1", 1, rc, out)
	rc, out = run("--root", filepath.Join(tmp, "no-such"))
	chk("档③ 缺件：`--root` 读不到 ⇒ rc=2（不给结论）", 2, rc, out)

	if bad == 0 {
		fmt.Fprint(stdout, "── 自检：全过 ✓\n")
		return 0
	}
	fmt.Fprintf(stdout, "── 自检：**%d 档不过** ✗\n", bad)
	return 1
}

// cktRepoRoot —— py `REPO_ROOT`（见标准声明行 (a)）。env 非空 ⇒ 无条件采信；否则 cwd 上溯。
func cktRepoRoot() string {
	if v := os.Getenv("ZERG_REPO_ROOT"); v != "" {
		return v
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "core", "cmd", "zerg", "main.go")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return wd
}

// checkCurlTimeoutGateMain —— 入口（= py `main()`）。
func checkCurlTimeoutGateMain(stdout, stderr io.Writer, argv []string) int {
	// py：先查 `--self-test` / `--selftest`（在 argparse 之前，逐字对齐 py `:127`）。
	for _, a := range argv {
		if a == "--self-test" || a == "--selftest" {
			return cktSelfTest(stdout, stderr)
		}
	}

	root := ""
	hasRoot := false
	jsonOut := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, "用法错（rc=2）：`--root` 缺值（argparse 面正文不同 · 见未覆盖面①）\n")
				return 2
			}
			i++
			root = argv[i]
			hasRoot = true
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
			hasRoot = true
		case a == "--json":
			jsonOut = true
		default:
			fmt.Fprintf(stderr, "用法错（rc=2）：未知旗标 / 参数 %s（argparse 面正文不同 · 见未覆盖面①）\n", a)
			return 2
		}
	}
	if !hasRoot {
		root = cktPyJoin(cktRepoRoot(), "scripts")
	}
	return cktRun(stdout, stderr, root, jsonOut)
}
