// check_signature_identity.go — 判据件 `scripts/gates/check-signature-identity.py`（288 行）的 **Go 真搬**
// （v2.5.14 · M2 一枚 · 签名档身份族 · 入口 `checkSignatureIdentityGateMain` · 承接 GAP-20261001-13）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的新口径差异面与未覆盖面）。
//
// 本件属「甲案」那一类：**真跑子进程／吃制品**（跑 macOS 的 `codesign` 取签名档读数）。
// 口径 = **输入确定 ⇒ 输出可逐字对拍**：同一路径上的同一签名的件，`codesign -dvvv` 的读数确定。
//
// 选它 / 不选它的理由：
//
//	✓ 选：判据是**读签名档三格**（`Identifier` / `flags` / `Signature`），只跑系统本机工具 `codesign`、
//	  不联网、不起后台、纯只读（除自检在临时目录里造件）；退码 `0/1/2` 三档齐。
//	✓ **工具链指纹**（本件依赖的唯一外部工具）：`codesign` 走 `/usr/bin/codesign`（同一台机两侧共用）。
//	  该工具**无 `--version`**（现读：`codesign --version` ⇒ `unrecognized option`）⇒ 指纹以**路径 + 输出面**为准。
//	· ⚠ **版本面差风险**：不同 macOS 版本 `codesign -dvvv` 的**行集合**可能不同（例如多/少 `CandidateCDHash` 一族）；
//	  本件**只吃**三格 + `CDHash`/`Authority`/`Executable`，其余行任一态都无视 ⇒ 版本面差**落在未覆盖面**（见下）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-signature-identity.py` 原件（退场规则：**对拍通过后才**退场 · 逐件点名）；
//	· 入口 `checkSignatureIdentityGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkSignatureIdentityGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 三格全过 / 1 有红（三格之一不过 · 或该件没签名）/ 2 不给结论（缺件 / 无工具 / 自检不过）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ═══════════════════════ ★★ 目录遍历序 —— 逐点交代 ═══════════════════════
//
//	★ **本件「不适用」**（无遍历面 · 给理由）：py 全文**不调** `os.walk` / `os.scandir` / `os.listdir` /
//	`glob` —— 它只对**逐个显式点名**的件跑 `os.path.isfile` + `codesign -dvvv`：
//	  ① 默认件集写死两件（`bin/zerg` · `bin/zerg-core`）；② `--piece` 逐枚追加/替换。
//	⇒ 输出行序 = `rels` 的**入参序**（py `a.piece or list(DEFAULT_PIECES)`），**逐件串行、无目录递归**。
//	顺序的**决定者** = argv 里 `--piece` 的**先后**（argparse `action="append"` 保序）+ 默认件的元组序。
//	★ `sorted()`/`sort()` 在本件**零用量** ⇒ 遍历序**不进输出**；本件输出**逐件保入参序**。
//	★ 因此无需造「子目录在 readdir 原序 index1」夹具（无 readdir 原序可言）—— 这条按标准声明**标记不适用**。
//
// ═══════════════════════ ★★ Join 三语义差 / 词法面（逐处现读）═══════════════════════
//
//	py 现读的路径面函数（逐处）：
//	  · `os.path.abspath(a.root)`（`:229`）· `os.path.abspath(__file__)`（`:66`）
//	  · `os.path.isabs(rel)`（`:127`）· `os.path.join(root, rel)`（`:127`）
//	  · `os.path.join(d, "adhoc")` / `join(d,"unsigned")` / `join(d,"不存在这件")`（`:171/:186/:192`）
//	  · `os.path.isfile(p)`（`:129`）· `os.path.dirname`（`:66`:`:67`）
//	  · ★ 词法面现读 **零用量**：`normpath`（不直接调）· `relpath` **零** · `realpath` **零** ·
//	    `islink` **零** · `is_dir` **零**（只用 `isfile`）。
//
//	★★ `filepath.Join` ⟷ `os.path.join` 三语义差 —— **逐处现读是否触发**：
//	  ① **吃 `..`**：`filepath.Join` 会 Clean 掉 `a/../b`；`os.path.join` **原样串拼**。本件两处 join 的
//	     第二段现读 = 常量（`"adhoc"`/`"unsigned"`/`"不存在这件"`）或 `rel`（来自 `--piece`）。
//	     ⇒ `--piece ../x` 时**触发**（py 得 `<root>/../x`、Go `filepath.Join` 会折成 `<root 同级>/x`）。
//	     **本件自写 `csidPyJoin`**（posixpath 语义：逐段串拼、不归一 `..`、后段绝对则重置）⇒ 逐字同构。
//	  ② **后段绝对丢前段**：`os.path.join("/a","/b")=="/b"`。本件 `check_piece` 里当 `rel` 为绝对路径时
//	     py **短路跳过 join**（`rel if os.path.isabs(rel) else join(root,rel)`）⇒ 该格由 `csidPyIsAbs` 顶。
//	     自检里的 join 第二段全为常量（非绝对）⇒ 不触发；**`csidPyJoin` 仍实现**该语义（后段 `/` 开头即重置）。
//	  ③ **空段**：`os.path.join("a","")=="a/"`（py 串拼）；`filepath.Join("a","")=="a"`（Clean 去尾斜杠）。
//	     ⇒ `--piece ""` 时**触发**。本件 `csidPyJoin` 逐字实现**空段保留尾斜杠**（见单测 `空段` 子用例）。
//	  ★ 三语义差本件**全部自写 py 语义函数顶掉**（不靠 `filepath.Join`）。`filepath.Join` 仅用于
//	    **默认根定位**（Go 自有的上溯逻辑，py 无此面 · 见标准声明 (a)）。
//
//	★★ `posixpath.normpath` 保留**恰两个前导斜杠** ⟷ `filepath.Clean` 折成单斜杠：
//	  py `os.path.abspath` 内部调 `normpath`；现读 `normpath("//a/b")=="//a/b"`（POSIX 允许恰两斜杠）、
//	  `normpath("///a/b")=="/a/b"`（三斜杠折一）。Go `filepath.Clean("//a/b")=="/a/b"` ⇒ **必须自写**。
//	  本件自写 `csidPyNormpath`（逐字对齐 CPython 的 `initial_slashes` 语义）＋ `csidPyAbsPath`
//	  （非绝对则 `join(cwd, s)` 再 `normpath`）。⇒ `--root //x/y` 这一格逐字同构（见单测 `normpath 双前导斜杠`）。
//
// ═══════════════════════ ★★ 正则语义差（py `re` ⟷ Go RE2 · 逐条写明并逐条对齐）═══════════════════════
//
//	py 现读**唯一**一条正则（`:75`）：
//	  CODE_DIR_RE = re.compile(r"^CodeDirectory .*?\bflags=0x([0-9a-fA-F]+)\(([^)]*)\)")
//	  用法：`CODE_DIR_RE.match(s)`，`s = line.strip()`（py Unicode strip）。
//
//	① `\b`（`.*?` 与 `flags` 之间的词边界）：py `\b` 用 **Unicode 词字符**（`\w` = 字母/数字/下划线，
//	   按 `str.isalnum()` 或 `_`）。Go RE2 的 `\b` 是 **ASCII** ⇒ **不硬译**：本件**手写扫描器**
//	   `csidMatchCodeDir`，`\b` 用 `csidIsWordRune`（按 Unicode 词字符判）顶掉。逐字对读：next 恒为
//	   `f`（词字符）⇒ 边界成立当且仅当**前一元音**非词字符（或串首）。
//	② `.*?`（非贪婪 · 非 DOTALL）：`.` 不匹配 `\n`；本件逐行喂（`splitlines` 的每一行，无 `\n`）⇒ `.`
//	   ≡ 任意元音。手写扫描器按「从左到右试每一处 `flags=0x`、取首个整体成立者」复刻非贪婪回退。
//	③ `[0-9a-fA-F]+`：**原样保留**（ASCII 十六进制 · ✗ 不换 `\p{Nd}`）。
//	④ `\(` `([^)]*)` `\)`：字面括号 + 到**首个** `)` 前的一切（`[^)]` 天然不含 `)`）。
//	⑤ `^`：`re.match` 锚串首（非多行）；手写扫描器以 `strings.HasPrefix(s,"CodeDirectory ")` 顶。
//	⑥ 环视 `(?=…)` / 反向引用 / `(?i:…)` / `\d` / `\s`：本件**零用量** ⇒ ✗ 无需转译。
//
//	★ `str.strip()` 是 **py Unicode 语义**（`str.isspace()` 的字符集：含 `\x09-\x0d` · `\x1c-\x1f` ·
//	  `\x85` · `\xa0` · `\u1680` · `\u2000-\u200a` · `\u2028` · `\u2029` · `\u202f` · `\u205f` · `\u3000`），
//	  ✗ **非** `strings.TrimSpace`（Go 的 `unicode.IsSpace` 集合**不含** `\x1c-\x1f`，且行为面亦有差）
//	  ⇒ 本件自写 `csidStrip` / `csidIsPySpace` **逐码点**对齐。
//	★ `s[:200]`（`:106` 的错误串截断）按**码点**切 ⇒ 本件 `csidTruncRunes`（✗ 非字节切）。
//	★ `out.splitlines()`（`:92`）按 py **全套行边界** ⇒ 本件 `csidSplitLines` 逐码点对齐 11 类断行。
//	★ `open()` 文本模式：py 走 `subprocess` 的 `text=True`（按本机编码解 `codesign` 的字节）⇒ 见未覆盖面③。
//
// ═══════════════════════ ★★ 符号链接判定面（⑨）═══════════════════════
//
//	py `os.path.isfile(p)`（`:129`）= **stat 跟随符号链**（链指常规件 ⇒ True）。本件**顶用 `os.Stat`**
//	（✗ 非 `os.Lstat` / ✗ 非 `DirEntry.IsDir()`），判 `Mode().IsRegular()`。py **零** `islink` / `is_dir`
//	⇒ 本件亦**零** `os.Lstat`。真仓 `bin/` 现读**软链 0 枚**；对拍夹具**一律不落符号链接路径**
//	（夹具根用 `/private/var/tmp`，✗ 不用 `/var/tmp` 这枚软链形态）。
//
// ═══════════════════════ ★★ JSON 面 ═══════════════════════
//
//	★ **输入面**：本件**不反序列化任何 JSON**（py 只**产出** `--json`，不读 JSON 输入）⇒「区分缺键 vs 显式
//	  null」的 Go 反序列化面**不适用**（给理由：无 JSON 入参、无配置文件、无外部 JSON 树）。
//	★ **输出面**：py `json.dumps({...}, ensure_ascii=False, indent=2)`。⇒ 本件**手写** JSON 生成器
//	  （`csidJSONDoc`）：① 键序照 py dict 插入序（`root`/`rows`/`ok`/`red`/`blocked`；行内 `piece`/`path`/
//	  [三格+`cdhash`]/`reason`/`verdict`）；② 缩进 2 · 空容器 `[]`/`{}` 内联；③ 串转义 = py `ensure_ascii=False`
//	  （✗ 不转义 `<`/`>`/`&` · ✗ 不转义非 ASCII · 控制符按 py 的 `\b\f\n\r\t` + `\uXXXX`）。
//	  ✗ 用 `encoding/json` 会：**乱序**（map）· **转义 HTML**（`<`⇒`\u003c`）· 不保键序 ⇒ 一律手写。
//	★ JSON 里的反斜杠串写**转义安全形态**（`\\` ⇒ `\\\\`）：本件的 `--piece` 可含反斜杠 ⇒ `csidJSONString`
//	  逐字符转义（见单测 `反斜杠不炸单` 子用例）。
//
// ═══════════════════════ ★★ 自指检查（四形态 · 前瞻消解）═══════════════════════
//
//	本件**不在**自己的判据面内（py 判的是 `codesign -dvvv` 的输出面，非文件文本）⇒ **自指 = 假**：
//	  ① 件头/注释里的**说明**不构成任何门触发子串：本件**零**命令树层级标记字面（层级常量后跟冒号与
//	     引号那种）、**零**错误 kind 设值调用字面、**零**机器面 kind 键值对字面、**零**私有绝对路径字面。
//	  ② 本件**零**被打印的固定仓内绝对路径串（仓根一律由 `--root` 现读 / 缺省由 cwd 上溯现读）。
//	  ③ 本件正文里的 `codesign` / `flags` / `adhoc` 等词**只作子进程名与打印文案**，**不落进任何被扫件**。
//	  ④ `--self-test` 的合成件只写在 `os.MkdirTemp` 的**仓外**临时目录（`TMPDIR`）⇒ 不落进仓内任何树。
//	⇒ **对被跟踪的三门贡献 = 0**（见「必带读数」的落仓前后 rc / 输出对比）。
//
// 已对拍面（两态逐字 · 甲=真仓+显式传参 / 乙=仓外夹具+每夹具自放 py 副本）：
//
//	真仓缺省两件（显式 `--root` · py 走真仓原件绝对路径）· `--json`（真仓）· `--list-rules` ·
//	`--piece bin/zerg`（单件替换）· `--piece` 传**绝对路径** · `--piece` 不存在的路径（缺件 ⇒ 2）·
//	`--piece ""`（空段 join 语义）· `--piece ../` 形态（吃 `..` 语义）· 件集为空（缺 `--piece` ⇒ 默认）·
//	`--self-test`（成对负控 7 断言）· `--json --self-test` · 读不了的件（目录 / 无读权限）·
//	`--root //x/y`（normpath 双前导斜杠）· `--no-self-test`。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 帮助 / 用法错正文**：py 用 `argparse.ArgumentParser(add_help=True)` ⇒ `-h/--help` 打
//	   argparse 帮助正文、未知旗标 / 缺值打 `usage…error: …`（stderr）—— 本入口按「未知旗标 / 缺值 ⇒
//	   rc=2」处理，**正文与 py 不同** ⇒ 这一面**不属逐字对齐承诺**（对拍一律不喂帮助 / 未知旗标）。
//	② **默认根定位**：见标准声明行 (a) —— py 由 `__file__` 上溯（脚本在 `<repo>/scripts/gates/`）⟷
//	   Go 无 `__file__` 语境，由 **cwd 上溯**找 `core/cmd/zerg/main.go`。两态在显式 `--root` 下逐字相同。
//	③ **非 UTF-8 角落**：`subprocess` 的 `text=True` 按本机编码解 `codesign` 字节（非法序列按该编码
//	   处理）；本件 `string(raw)` 逐字节透传 ⇒ 极端角落可能不同（`codesign` 输出现读全 ASCII ⇒ 不触发）。
//	④ **`text=True` 的 locale 面**：py 的编码取决于运行环境 `locale.getpreferredencoding()`；本件不做解码
//	   转换。⇒ 声明为未覆盖（真仓 `codesign` 输出 ASCII ⇒ 两态一致）。
//	⑤ **`codesign` 版本面差**：见件头「版本面差风险」——行集合差异本件无视；但若某版把 `flags=` 拼成
//	   非常规形态（本件手写扫描器认 `String "flags=0x…("`），则**该版未覆盖**。
//	⑥ **`subprocess` 的 `FileNotFoundError` 面**：本件 `csidRunCodesign` 的「exec 找不到」分支（对应
//	   py `:88` 的 `except FileNotFoundError`）；现读该分支**不可达**（主流程先 `shutil.which` 拦掉）。
//	⑦ **`--self-test` 的手段**：py 用 `subprocess.run([...])` **真跑** `codesign`（自检亦如此）；本件同
//	   样真跑 `codesign`，但**自检本体进程内**（非再 fork 一个 py）。⇒ 打印文案逐字对齐，差在实现手段。
//	⑧ **`tempfile.mkdtemp` 目录名**：py / Go 的随机目录名不同（**不进任何输出**）⇒ 对拍**零掩码**（见掩码声明）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// csidExit* —— 与 py `EXIT_OK/EXIT_FAIL/EXIT_BLOCKED` 同口径。
const (
	csidExitOK      = 0
	csidExitFail    = 1
	csidExitBlocked = 2
)

// csidDefaultPieces —— py `DEFAULT_PIECES`（逐件点名 · 不扫目录）。
var csidDefaultPieces = []string{"bin/zerg", "bin/zerg-core"}

// csidBadIdentifier / csidBadFlagsToken —— py `BAD_IDENTIFIER` / `BAD_FLAGS_TOKEN`。
const (
	csidBadIdentifier = "a.out"
	csidBadFlagsToken = "adhoc"
)

// csidDvvvKeys —— py `read_dvvv` 里按序试的字段名前缀（`:99`）。
var csidDvvvKeys = []string{"Identifier", "Signature", "CDHash", "Authority", "Executable"}

// csidRules —— py `RULES`（`--list-rules` 的真源）。
var csidRules = [][2]string{
	{"G1", "`Identifier` 不得为 `a.out`（Go linker ad-hoc 形态）"},
	{"G2", "`flags` 不得含 `adhoc`（`0x2(adhoc)` / `0x20002(adhoc,linker-signed)`）"},
	{"G3", "`Signature` 不得为 `adhoc`（该行不出现时这一格不判）"},
}

// ───────────────────── 路径面：自写 py 语义函数（Join 三语义差 / normpath 双前导斜杠）─────────────────────

// csidPyIsAbs —— py `posixpath.isabs`：`s.startswith('/')`。
func csidPyIsAbs(s string) bool { return strings.HasPrefix(s, "/") }

// csidPyJoin —— py `posixpath.join` 的**逐字**语义（逐段串拼 · 不归一 `..` · 后段绝对重置 · 空段留尾斜杠）。
func csidPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
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

// csidPyNormpath —— py `posixpath.normpath` 的**逐字**语义（★ 保留恰两个前导斜杠；三斜杠折一）。
func csidPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initialSlashes := 0
	if strings.HasPrefix(path, "/") {
		initialSlashes = 1
	}
	// POSIX：恰两个前导斜杠保留、三个及以上折成一个。
	if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
		initialSlashes = 2
	}
	var newComps []string
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

// csidPyAbsPath —— py `posixpath.abspath`：非绝对则 `join(cwd, s)`，再 `normpath`。
func csidPyAbsPath(s string) string {
	if !csidPyIsAbs(s) {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		s = csidPyJoin(wd, s)
	}
	return csidPyNormpath(s)
}

// csidIsFile —— py `os.path.isfile`（stat **跟随符号链** · 只认常规件）。
func csidIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// csidRepoRootDefault —— py `REPO_ROOT_DEFAULT`（`dirname^3(abspath(__file__))`）的 **Go 缺省推导**。
// Go 无 `__file__` 语境 ⇒ 从 cwd 上溯找含 `core/cmd/zerg/main.go` 的目录（见标准声明 (a) · 未覆盖面②）。
func csidRepoRootDefault() string {
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

// ───────────────────── 文本面：py str.strip / str.splitlines / 码点截断 ─────────────────────

// csidIsPySpace —— py `str.isspace()` 的字符集（逐码点）。
func csidIsPySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f,
		0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// csidStrip —— py `str.strip()`（Unicode 语义 · ✗ 非 strings.TrimSpace）。
func csidStrip(s string) string {
	return strings.TrimFunc(s, csidIsPySpace)
}

// csidSplitLines —— py `str.splitlines()` 的逐码点对齐（11 类断行；尾断行不产空串）。
func csidSplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '\n' || c == '\v' || c == '\f' || c == 0x1c || c == 0x1d || c == 0x1e ||
			c == 0x85 || c == 0x2028 || c == 0x2029:
			out = append(out, cur.String())
			cur.Reset()
		case c == '\r':
			if i+1 < len(r) && r[i+1] == '\n' {
				i++
			}
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

// csidTruncRunes —— py `s[:n]` 的**码点**切片（✗ 非字节切）。
func csidTruncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

// ───────────────────── 正则面：CODE_DIR_RE 的手写扫描器（`\b` 按 Unicode 词字符）─────────────────────

// csidIsWordRune —— py `\w`（= `str.isalnum()` 或 `_`；即 Unicode 的字母/数字/下划线）。
func csidIsWordRune(r rune) bool {
	if r == '_' {
		return true
	}
	if r < 0x80 {
		return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsDigit(r)
}

func csidIsHexByte(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// csidMatchCodeDir —— py `CODE_DIR_RE.match(s)` 的手写等价（回是否成立 + 两组捕获）。
//
//	^CodeDirectory .*?\bflags=0x([0-9a-fA-F]+)\(([^)]*)\)
//
// `\b` 用 Unicode 词字符（next 恒为 `f` ⇒ 边界当且仅当前一元音非词字符或串首）；
// `.*?` 非贪婪 ⇒ 从左到右取首个整体成立者。
func csidMatchCodeDir(s string) (g1, g2 string, ok bool) {
	const pfx = "CodeDirectory "
	if !strings.HasPrefix(s, pfx) {
		return "", "", false
	}
	start := len(pfx)
	idx := start
	for {
		j := strings.Index(s[idx:], "flags=0x")
		if j < 0 {
			return "", "", false
		}
		pos := idx + j
		// `\b`：前一元音若为词字符 ⇒ 无边界 ⇒ 此处不成。
		if pos > 0 {
			if pr, _ := utf8.DecodeLastRuneInString(s[:pos]); csidIsWordRune(pr) {
				idx = pos + 1
				continue
			}
		}
		hexStart := pos + len("flags=0x")
		k := hexStart
		for k < len(s) && csidIsHexByte(s[k]) {
			k++
		}
		if k == hexStart || k >= len(s) || s[k] != '(' {
			idx = pos + 1
			continue
		}
		closeRel := strings.IndexByte(s[k+1:], ')')
		if closeRel < 0 {
			idx = pos + 1
			continue
		}
		return s[hexStart:k], s[k+1 : k+1+closeRel], true
	}
}

// ───────────────────── 子进程面：codesign ─────────────────────

// csidRunCodesign —— 起 `codesign <args...>`（✗ 不接管道 · 真退出码取子进程的）。found=false ⇒ exec 找不到。
func csidRunCodesign(args ...string) (stdout, stderr string, rc int, found bool) {
	cmd := exec.Command("codesign", args...)
	var so, se strings.Builder
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			return "", "", 0, false
		}
		rc = ee.ExitCode()
	}
	return so.String(), se.String(), rc, true
}

// csidReadDvvv —— py `read_dvvv(path)`：⇒ (字段 dict, unsigned, 报错串)。
//
//	字段 dict 为 nil 且报错串为 "" 的情形不存在；unsigned=true 时字段 dict 为 nil（对应 py 的 `{"unsigned": True,…}`）。
func csidReadDvvv(path string) (fields map[string]string, unsigned bool, errMsg string) {
	so, se, rc, found := csidRunCodesign("-dvvv", path)
	if !found {
		return nil, false, "`codesign` 不在（本机无 codesign ⇒ 判不了）"
	}
	out := so + se
	f := map[string]string{}
	for _, line := range csidSplitLines(out) {
		s := csidStrip(line)
		if g1, g2, ok := csidMatchCodeDir(s); ok {
			f["flags"] = "0x" + g1 + "(" + g2 + ")"
			f["flags_raw"] = g2
			continue
		}
		for _, key := range csidDvvvKeys {
			if strings.HasPrefix(s, key+"=") {
				parts := strings.SplitN(s, "=", 2)
				f[strings.ToLower(key)] = csidStrip(parts[1])
				break
			}
		}
	}
	if rc != 0 && strings.Contains(out, "not signed at all") {
		return nil, true, ""
	}
	if rc != 0 {
		return nil, false, fmt.Sprintf("`codesign -dvvv` 退码 %d：%s", rc, csidTruncRunes(csidStrip(out), 200))
	}
	return f, false, ""
}

// csidJudge —— py `judge(fields)` ⇒ (pass?, 逐条理由)。py 用 `.get(k, "")` ⇒ 缺键与空串同效（此处 map 同义）。
func csidJudge(fields map[string]string) (bool, []string) {
	var bad []string
	ident := fields["identifier"]
	if ident == csidBadIdentifier {
		bad = append(bad, fmt.Sprintf(
			"G1：`Identifier=%s`（Go linker 的 ad-hoc 形态 ⇒ TCC 本地网络授权随 cdhash 静默失效）", ident))
	}
	flags := fields["flags"]
	if strings.Contains(strings.ToLower(flags), csidBadFlagsToken) {
		bad = append(bad, fmt.Sprintf("G2：`flags=%s` 含 `adhoc`（该件不是稳定身份签名）", flags))
	}
	if strings.ToLower(fields["signature"]) == csidBadFlagsToken {
		bad = append(bad, "G3：`Signature=adhoc`")
	}
	return len(bad) == 0, bad
}

// ───────────────────── 行模型（保 py dict 的**插入键序**）─────────────────────

type csidKV struct{ K, V string }

type csidRow struct {
	kvs     []csidKV
	verdict string
}

func (r *csidRow) get(k string) (string, bool) {
	for _, kv := range r.kvs {
		if kv.K == k {
			return kv.V, true
		}
	}
	return "", false
}

// must —— py `row.get(k, def)`。
func (r *csidRow) must(k, def string) string {
	if v, ok := r.get(k); ok {
		return v
	}
	return def
}

// csidCheckPiece —— py `check_piece(root, rel)` ⇒ (verdict ∈ ok/red/blocked, 行)。
func csidCheckPiece(root, rel string) (string, *csidRow) {
	p := rel
	if !csidPyIsAbs(rel) {
		p = csidPyJoin(root, rel)
	}
	r := &csidRow{kvs: []csidKV{{"piece", rel}, {"path", p}}}
	if !csidIsFile(p) {
		r.kvs = append(r.kvs, csidKV{"reason", "被测件不在（缺件 ⇒ 不给结论）"})
		return "blocked", r
	}
	fields, unsigned, errMsg := csidReadDvvv(p)
	if errMsg != "" {
		r.kvs = append(r.kvs, csidKV{"reason", errMsg})
		return "blocked", r
	}
	if unsigned {
		r.kvs = append(r.kvs,
			csidKV{"identifier", ""}, csidKV{"flags", ""},
			csidKV{"reason", "该件**没有签名**（`code object is not signed at all`）"})
		return "red", r
	}
	ok, bad := csidJudge(fields)
	r.kvs = append(r.kvs,
		csidKV{"identifier", fields["identifier"]},
		csidKV{"flags", fields["flags"]},
		csidKV{"signature", fields["signature"]},
		csidKV{"cdhash", fields["cdhash"]})
	if ok {
		r.kvs = append(r.kvs, csidKV{"reason", "三格全过"})
		return "ok", r
	}
	r.kvs = append(r.kvs, csidKV{"reason", strings.Join(bad, " · ")})
	return "red", r
}

// ───────────────────── JSON 面（手写 · 保键序 · py ensure_ascii=False + indent=2）─────────────────────

// csidJSONString —— py `json.dumps(str, ensure_ascii=False)` 的单串转义（✗ 不转义 HTML / 非 ASCII）。
func csidJSONString(s string) string {
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

// csidJV / csidJA —— JSON 编码用的保序对象 / 数组（与行模型的 csidKV 分开：值可为任意类型）。
type csidJV struct {
	K string
	V interface{}
}

// csidJEnc —— py `json.dumps(indent=2)` 的缩进/换行规则：非空 dict/list 逐项独占一行，空容器内联。
func csidJEnc(v interface{}, ind int) string {
	switch x := v.(type) {
	case string:
		return csidJSONString(x)
	case int:
		return strconv.Itoa(x)
	case []csidJV: // 对象（保序）
		if len(x) == 0 {
			return "{}"
		}
		pad := strings.Repeat("  ", ind+1)
		var b strings.Builder
		b.WriteString("{\n")
		for i, kv := range x {
			b.WriteString(pad)
			b.WriteString(csidJSONString(kv.K))
			b.WriteString(": ")
			b.WriteString(csidJEnc(kv.V, ind+1))
			if i < len(x)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(strings.Repeat("  ", ind))
		b.WriteString("}")
		return b.String()
	case []interface{}: // 数组
		if len(x) == 0 {
			return "[]"
		}
		pad := strings.Repeat("  ", ind+1)
		var b strings.Builder
		b.WriteString("[\n")
		for i, e := range x {
			b.WriteString(pad)
			b.WriteString(csidJEnc(e, ind+1))
			if i < len(x)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(strings.Repeat("  ", ind))
		b.WriteString("]")
		return b.String()
	}
	return "null"
}

// csidJSONDoc —— py `json.dumps({"root":…,"rows":…,"ok":…,"red":…,"blocked":…}, ensure_ascii=False, indent=2)`。
func csidJSONDoc(root string, rows []*csidRow, ok, red, blocked int) string {
	var arr []interface{}
	for _, r := range rows {
		kvs := make([]csidJV, 0, len(r.kvs)+1)
		for _, kv := range r.kvs {
			kvs = append(kvs, csidJV{kv.K, kv.V})
		}
		kvs = append(kvs, csidJV{"verdict", r.verdict})
		arr = append(arr, kvs)
	}
	top := []csidJV{
		{"root", root},
		{"rows", arr},
		{"ok", ok},
		{"red", red},
		{"blocked", blocked},
	}
	return csidJEnc(top, 0)
}

// ───────────────────── 自检（成对负控 · 全在 TMPDIR 合成夹具里）─────────────────────

// csidRepr —— py `%r` 的近似（只覆盖自检断言里出现的 str/int/bool）。
func csidRepr(v interface{}) string {
	switch x := v.(type) {
	case string:
		if strings.ContainsRune(x, '\'') && !strings.ContainsRune(x, '"') {
			return `"` + x + `"`
		}
		return "'" + strings.ReplaceAll(x, `\`, `\\`) + "'"
	case int:
		return strconv.Itoa(x)
	case bool:
		if x {
			return "True"
		}
		return "False"
	case []string:
		parts := make([]string, len(x))
		for i, s := range x {
			parts[i] = csidRepr(s)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return fmt.Sprintf("%v", v)
}

func csidReprEq(got, want interface{}) bool {
	return csidRepr(got) == csidRepr(want)
}

// csidCopyBytes —— py `shutil.copy` 的字节级等价（✗ 不搬 st_flags：只搬字节 · 见 py `:172` 注释的缘由）。
func csidCopyBytes(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o755)
}

// csidSelfTest —— py `self_test(verbose)` ⇒ (断言数, 失败列表)。夹具用完即清。
func csidSelfTest(stdout io.Writer, verbose bool) (int, []string) {
	n := 0
	var fails []string
	assertEq := func(name string, got, want interface{}) {
		n++
		if !csidReprEq(got, want) {
			fails = append(fails, fmt.Sprintf("%s：得 %s · 期望 %s", name, csidRepr(got), csidRepr(want)))
			if verbose {
				fmt.Fprintf(stdout, "  ✗ %s：得 %s · 期望 %s\n", name, csidRepr(got), csidRepr(want))
			}
		} else if verbose {
			fmt.Fprintf(stdout, "  ✓ %s\n", name)
		}
	}
	if _, err := exec.LookPath("codesign"); err != nil {
		fmt.Fprint(stdout, "  ⛔ 自检跑不了：本机无 `codesign`\n")
		return 0, []string{"`codesign` 不在 ⇒ 自检不可判"}
	}
	d, err := os.MkdirTemp("", "zerg-sigident-")
	if err != nil {
		fmt.Fprint(stdout, "  ⛔ 自检跑不了：造不出临时目录\n")
		return 0, []string{"造不出临时目录 ⇒ 自检不可判"}
	}
	defer func() { _ = os.RemoveAll(d) }()

	f1 := csidPyJoin(d, "adhoc")
	if err := csidCopyBytes("/bin/echo", f1); err != nil {
		return n, []string{"夹具① 造件失败：" + err.Error()}
	}
	_, _, rc, _ := csidRunCodesign("-s", "-", "--force", f1)
	assertEq("夹具① 造成 ad-hoc（`codesign -s - --force`）rc", rc, 0)
	v, row := csidCheckPiece("/", f1)
	assertEq("夹具① ad-hoc 件 ⇒ **必判红**（负控半边）", v, "red")
	reason, _ := row.get("reason")
	g2hit := false
	for _, b := range strings.Split(reason, " · ") {
		if strings.HasPrefix(b, "G2") {
			g2hit = true
		}
	}
	assertEq("夹具① 红理由落在 G2（flags 含 adhoc）", g2hit, true)

	f2 := "/bin/echo"
	v2, _ := csidCheckPiece("/", f2)
	assertEq("夹具② 原样苹果签名件 ⇒ **必判绿**（正控半边）", v2, "ok")

	f3 := csidPyJoin(d, "unsigned")
	if err := csidCopyBytes("/bin/echo", f3); err != nil {
		return n, []string{"夹具③ 造件失败：" + err.Error()}
	}
	_, _, _, _ = csidRunCodesign("--remove-signature", f3)
	v3, _ := csidCheckPiece("/", f3)
	assertEq("夹具③ 未签件 ⇒ **必判红**（与② 只差「签 / 不签」一个变量）", v3, "red")

	v4, _ := csidCheckPiece("/", csidPyJoin(d, "不存在这件"))
	assertEq("夹具④ 件不在 ⇒ **必 rc=2（不给结论）**，不许与「未签」混成同一个码", v4, "blocked")

	// G1 那一格单独证（真造 `Identifier=a.out` 要 Go linker；此处用合成字段面证判据本身）
	n++
	ok, bad := csidJudge(map[string]string{
		"identifier": csidBadIdentifier,
		"flags":      "0x20002(adhoc,linker-signed)",
		"signature":  "adhoc",
	})
	if ok || len(bad) != 3 {
		fails = append(fails, fmt.Sprintf("G1/G2/G3 合成字段面 ⇒ 期望三格全红，得 %s / %s",
			csidRepr(ok), csidRepr(bad)))
	} else if verbose {
		fmt.Fprint(stdout, "  ✓ G1+G2+G3 合成字段面（`a.out` + linker-signed + `Signature=adhoc`）⇒ 三格全红\n")
	}
	return n, fails
}

// checkSignatureIdentityGateMain —— 入口（= py `main()`）。
func checkSignatureIdentityGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	hasRoot := false
	var pieces []string
	jsonOut := false
	listRules := false
	selfTest := false
	noSelfTest := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprint(stdout, "用法错（rc=2）：`--root` 缺值（argparse 面正文不同 · 见未覆盖面①）\n")
				return csidExitBlocked
			}
			i++
			root = argv[i]
			hasRoot = true
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
			hasRoot = true
		case a == "--piece":
			if i+1 >= len(argv) {
				fmt.Fprint(stdout, "用法错（rc=2）：`--piece` 缺值（argparse 面正文不同 · 见未覆盖面①）\n")
				return csidExitBlocked
			}
			i++
			pieces = append(pieces, argv[i])
		case strings.HasPrefix(a, "--piece="):
			pieces = append(pieces, a[len("--piece="):])
		case a == "--json":
			jsonOut = true
		case a == "--list-rules":
			listRules = true
		case a == "--self-test":
			selfTest = true
		case a == "--no-self-test":
			noSelfTest = true
		default:
			fmt.Fprintf(stdout, "用法错（rc=2）：未知旗标 / 参数 %s（argparse 面正文不同 · 见未覆盖面①）\n", a)
			return csidExitBlocked
		}
	}

	if listRules {
		fmt.Fprint(stdout, "判据（三格 · 逐件独立判）：\n")
		for _, r := range csidRules {
			fmt.Fprintf(stdout, "  %s %s\n", r[0], r[1])
		}
		fmt.Fprintf(stdout, "被测件（默认）：%s\n", strings.Join(csidDefaultPieces, " · "))
		fmt.Fprint(stdout, "退码：0 全绿 · 1 有红（三格之一不过 / 未签）· 2 不给结论（缺件 / 无 codesign / 自检不过）\n")
		fmt.Fprint(stdout, "自检：① ad-hoc 件必红 ② 苹果签名件必绿 ③ 未签件必红 ④ 缺件必 2\n")
		return csidExitOK
	}

	// py：`root = os.path.abspath(a.root) if a.root else REPO_ROOT_DEFAULT`（空串 ⇒ 视同未给）。
	rootAbs := ""
	if hasRoot && root != "" {
		rootAbs = csidPyAbsPath(root)
	} else {
		rootAbs = csidRepoRootDefault()
	}

	if !noSelfTest {
		if !jsonOut {
			fmt.Fprint(stdout, "自检（成对负控 · 合成夹具 · 不碰真目标）：\n")
		}
		n, fails := csidSelfTest(stdout, !jsonOut)
		if len(fails) > 0 {
			fmt.Fprintf(stdout, "⛔ 不给结论：自检未过（%d 条失败）⇒ 先修门自己\n", len(fails))
			for _, f := range fails {
				fmt.Fprint(stdout, "  ✗ "+f+"\n")
			}
			return csidExitBlocked
		}
		if selfTest {
			fmt.Fprintf(stdout, "自检结论: 全过（断言 %d 条 · 失败 0 条）\n", n)
			return csidExitOK
		}
		if !jsonOut {
			fmt.Fprintf(stdout, "自检结论: 全过（断言 %d 条 · 失败 0 条）\n\n", n)
		}
	} else if selfTest {
		fmt.Fprint(stdout, "⛔ --self-test 与 --no-self-test 同给 ⇒ 用法错\n")
		return csidExitBlocked
	}

	if _, err := exec.LookPath("codesign"); err != nil {
		fmt.Fprint(stdout, "⛔ 不给结论（rc=2）：本机无 `codesign` ⇒ 签名档读不了（门侧口径：缺语料不给结论，不许当绿）\n")
		return csidExitBlocked
	}

	rels := pieces
	if len(rels) == 0 {
		rels = append([]string{}, csidDefaultPieces...)
	}
	var rows []*csidRow
	okc, redc, blkc := 0, 0, 0
	for _, rel := range rels {
		v, row := csidCheckPiece(rootAbs, rel)
		row.verdict = v
		rows = append(rows, row)
		switch v {
		case "ok":
			okc++
		case "red":
			redc++
		default:
			blkc++
		}
	}

	if jsonOut {
		fmt.Fprintln(stdout, csidJSONDoc(rootAbs, rows, okc, redc, blkc))
	} else {
		fmt.Fprintf(stdout, "签名档三格门 · 仓根 %s\n", rootAbs)
		fmt.Fprintf(stdout, "  被测件 %d 件：%s\n", len(rels), strings.Join(rels, " · "))
		for _, row := range rows {
			mark := map[string]string{"ok": "✓", "red": "✗", "blocked": "⛔"}[row.verdict]
			piece, _ := row.get("piece")
			fmt.Fprintf(stdout, "  %s %-16s Identifier=%s · flags=%s · Signature=%s\n",
				mark, piece, row.must("identifier", "—"), row.must("flags", "—"), row.must("signature", "—"))
			if cdh := row.must("cdhash", ""); cdh != "" {
				fmt.Fprintf(stdout, "       CDHash=%s\n", cdh)
			}
			fmt.Fprintf(stdout, "       %s\n", row.must("reason", ""))
		}
		fmt.Fprintf(stdout, "  ⇒ 绿 %d · 红 %d · 不给结论 %d\n", okc, redc, blkc)
	}

	if redc > 0 {
		fmt.Fprint(stdout, "⇒ 判红（rc=1）：上面带 ✗ 的件不是稳定身份签名（ad-hoc = TCC 本地网络授权会随 cdhash 静默失效）\n")
		return csidExitFail
	}
	if blkc > 0 {
		fmt.Fprint(stdout, "⇒ 不给结论（rc=2）：上面带 ⛔ 的件读不到（缺件 / 工具不在）\n")
		return csidExitBlocked
	}
	fmt.Fprint(stdout, "⇒ 全过（rc=0）：被测件都不是 ad-hoc 件（`Identifier` ≠ `a.out` · `flags` 不含 `adhoc`）\n")
	return csidExitOK
}
