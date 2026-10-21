// check_docs.go —— 判据件 `scripts/gates/check_docs.py`（191 行）的 **Go 真搬**
// （v2.5.14 · M2 · 文档双语门禁 · 入口 `checkDocsGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的
//	新口径差异面与未覆盖面）。
//
// 选它 / 不选它的理由（本单口径：纯静态只读 · M2 剩余 A 类中最小件）：
//
//	✓ 选：纯静态**只读**（读 `<root>` 下的 `*.md` / `*.markdown` 文本）—— 不联网、不碰
//	  外部工具、不取任何运行期读数；退码 `0/1/2` 三档齐；无写面（`--self-test` 只在临时目录里造件）。
//	· **外部树依赖走显式入参旋钮**：py 本就有**位置参数 `<仓库根>`**（无同名旗标/env）
//	  ⇒ 本件照搬该位置参数当**唯一旋钮**（名照 py 缺省面；py 无同名旗标 ⇒ 见标准声明 (a) 的口径差台账）。
//	· `git status` 现读**不含它**（无人正在改）。
//	✗ 不选同族更小件：本单「A 类纯静态 6 件」中它行数最小（191 行）。
//
// 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `main.go` 里注册命令（✗ 不进命令树 · `add_step` **未挂**（`precommit-gates.sh` 现读零命中））；
//	· **不**删 `scripts/gates/check_docs.py` 原件；
//	· **已接线**：入口 `checkDocsGateMain` 由 `gateCheckGoEntry["check_docs"]` 分派（`zerg gate check check_docs`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkDocsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 通过 / 1 有 error（`--strict` 时含 warn）/ 2 用法错 或 读不到输入（不给结论）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ═══════════════════════ ★★ 逐处现读交代（Join / 词法面 / 正则 / 符号链接 / 遍历）═══════════════════════
//
// ── ① `os.path.join` ⟷ `filepath.Join` 三语义差（逐处现读是否触发）──────────────
//
//	py 现读 `os.path.join` 共 **7 处**，逐处交代（一律走自写 `cdocPyJoin`，✗ 不用 filepath.Join）：
//	  · `:32` find_repo_root  `join(d, ".git")` / `join(d, "README.md")` —— 后段皆**相对常量**、
//	    无空段 ⇒ **三语义差均不触发**；照写自写件保恒同构。
//	  · `:44` doc_files       `join(base, f)` —— `f` 是 readdir 返回的**基名**（非空、非绝对、
//	    无 `..`）⇒ 不触发。
//	  · `:88`-`:89` self_test `join(root, fn)` / `join(tmp, name)` —— 后段**相对常量**「不触发」；
//	    ⚠ 但 `:88` 的 `name` 是 `mk()` 的入参（`"good"` / `"bad"`，**相对常量**）⇒ 不触发。
//	  · `:131` G1 `join(os.path.dirname(f), "README.md")` —— 后段相对常量 ⇒ 不触发。
//	  · `:133` 走**串切片**而非 join（`f[:-len(CJK_SUFFIX)] + EN_SUFFIX`）⇒ 逐字照搬切片。
//	  · `:168` G3 ★★ **`join(os.path.dirname(p), t)` 里的 `t` = 文内链接目标（**运行期数据**）**：
//	    它是本件**唯一一处运行期可能触发第 ② 类（后段绝对 ⇒ 丢前段）** 的 join ——
//	    py 写的是 `](…)` 里原样抓来的串，`t` 完全可能是 `/abs/x.md`（`http(s)/mailto/#` 只被前缀
//	    过滤，**绝对路径不在过滤列**）⇒ py `join("/repo/docs", "/abs/x.md")` == `"/abs/x.md"`
//	    （**丢弃前段**），而 `filepath.Join` 会拼成 `"/repo/docs/abs/x.md"`（**症状：件永不判失效**）。
//	    同类还含 `..`（py 原样留串、`filepath.Join` 归一）与空目标（`t` 至少 1 字符，`[^)#\s]+` ⇒ 空段不触发）。
//	    ⇒ **本件自写 `cdocPyJoin` ＋ `cdocPyNormpath` 逐字复刻**，并用定向单测把三态各钉一格。
//
// ── ② 词法面逐处现读（`normpath` / `abspath` / `relpath` / `realpath`）──────────────
//
//	· `os.path.abspath` **3 处**（`:30` `:61` `:119`）⇒ 自写 `cdocPyAbspath(cwd, p)`（**纯词法**：
//	  join 后 normpath 一次；✗ 不用 `filepath.Abs`：后者会 Clean 掉 `//`、与 py 极端串不同）。
//	  ★ cwd **由调用方传入**（py 的 cwd 来自 `os.getcwd()`；Go `os.Getwd()` 在 $PWD 不可信时会
//	  解析真路径 ⇒ 夹具落符号链接路径时两态不同 ⇒ 整批假失配）—— 见「符号链接面」。
//	· `os.path.normpath` **1 处**（`:168`）⇒ 自写 `cdocPyNormpath`（保留「开头斜杠数」细节）。
//	· `os.path.relpath` **6 处**（`:137` `:147` `:157` `:162` `:173` `:175`）⇒ 自写 `cdocPyRelpath`
//	  （去空段 → 数公共段 → `..` 前缀 + 余段）；✗ 不用 `filepath.Rel`（`..` 前缀算法不同）。
//	· `os.path.realpath`：**零用量**（全件现读零命中）⇒ 不涉「悬空链接回词法路径」那条。
//	· `os.path.dirname` **4 处**（`:131` `:168` ×2 + `:133` 隐含）⇒ 自写 `cdocPyDirname`
//	  （posixpath.split 语义：**剥尾斜杠**、全斜杠头保原样）。
//
// ── ③ 正则（唯一一处 `re`，不硬译）──────────────────────────────────────────────
//
//	py `:26` `LINK = re.compile(r'\]\(([^)#\s]+)(#[^)]*)?\)')`。
//	· **无锚 / 无边界 / 无环视 / 无反向引用 / 无捕获组引用**（只取 `group(1)`）⇒ ✗ 无需手写扫描器。
//	· `\s`（**在否定字符类里**）是 **Unicode 语义** ⇒ 按码点展开为 `cdocWS`（含 py 认的
//	  `\x1c-\x1f` 与 `\x85`），✗ 不用 Go RE2 的 ASCII `\s`。涉及 **1 处**（该字符类）。
//	· `[^)]`（组 2）与 `\]\(` 字面量：**逐字原样**，无 `\d` / `\b` / `(?i:)` 面。
//	· 末段 `\)` ⇒ ✗ 不用 `filepath.Match` / `fnmatch`（本件**零 glob/fnmatch 用量**，现读零命中）。
//	· Go `regexp.Compile` 为 **leftmost-first**（与 py leftmost 同）且此模式无交替 ⇒ 等价。
//	· `m.group(1).strip()` ⇒ `strings.TrimSpace`（py `str.strip()` 与 Go `unicode.IsSpace` 的
//	  极小差：`\x1c-\x1f` 只被 py 剥 ⇒ 见未覆盖面⑤）。
//
// ── ④ 符号链接面（判据判定用的是哪把尺）──────────────────────────────────────────
//
//	· py `os.path.isdir` / `os.path.isfile` / `os.path.exists` **一律跟随**符号链接（stat 语义）
//	  ⇒ Go 侧**一律 `os.Stat`**（✗ 非 `DirEntry.IsDir()`、✗ 非 `filepath.WalkDir` 的 dirent 判定）。
//	· py `os.walk(followlinks=False)` 用 `entry.is_dir()`（**跟随**）把条目分进 `dirs`/`files`，
//	  但**下钻**前判 `os.path.islink(new_path)` ⇒ 链接目录**录入不下钻**。本件 `cdocDocFiles` 逐字复刻：
//	  分类用 `os.Stat`（跟随）、下钻前用 `os.Lstat`（`cdocIsLink`）拦掉链接。
//	· py `os.path.islink` 零用量（说明：`os.walk` 内部用的不是它）；不存在悬空链接误判面。
//	  ★ 真仓现读：`scripts/gates/` 下软链 0 枚 ⇒ 不触发；夹具**所在路径**一律落**非符号链接路径**
//	  （`/private/var/tmp/…`，**不用** `/var/tmp` ⟷ `/private/var/tmp` 那种形态）；
//	  夹具**内部**可按需造链接条目（判 symlink 面时正当）。
//
// ── ⑤ 目录遍历序（★ 保原序读法 ＋ 产出序）─────────────────────────────────────────
//
//	py `doc_files`（`:38-45`）= `os.walk(root)`（topdown 默认）+ `dirs[:]` 过滤 + **末尾 `sorted(out)`**。
//	  ⓐ **同层读取序** = `os.walk` 内部 `os.scandir` = **readdir 原序**（**不排序**）
//	     ⇒ 本件用 **`f.ReadDir(-1)`**（✗ `os.ReadDir` / `filepath.WalkDir` / `filepath.Walk` 三者**都会排序**）。
//	  ⓑ **层内分区序** = py 先把 `scandir` 结果分成 `dirs`/`files`、**先 yield 当前层全部文件**，**再按
//	     `dirs` 原序下钻** ⇒ 本件**先收当前层文件、再按 dirs 原序递归**（✗ 边遍历边下钻：子目录夹在
//	     readdir 原序中间时会**早钻** ⇒ 行序假差）。
//	  ⓒ **`dirs[:]` 过滤**（`.git` / `target` / `node_modules` / `vendor`）= **保序过滤**、**不下钻被剔目录**。
//	  ⓓ ★★ **输出行序的决定者 = `:45` 末尾那句全局 `sorted(out)`**（py **明写 `sorted(...)`** ⇒ 本件用
//	     `sort.Strings` 复现：Go 的 UTF-8 字节序 == py 的 code point 序）。⇒ ⓐⓑⓒ 三者的**保序**只为
//	     **控制流逐字同构**；`doc_files` 的**返回值序**由 `sorted` 决定，`files` 的一切下游（G1/G2/G3/G4
//	     与 `pair` 追加序）都从它派生 ⇒ **stdout 行序严格 = `sorted` 后的 `files` 序**。**零 Go map 迭代进输出**
//	     （本件输出面**不用任何 map**；`pairs` / `errs` / `warns` 全为**切片**，保 py 插入序）。
//	  ⓔ **`--self-test` 三档** = py 逐档**串行** `subprocess.run` ⇒ 本件同序照搬（进程内直调，见未覆盖面④）。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：
//	    py `find_repo_root()`（`:29-35`）起于 `abspath(dirname(__file__))`（**脚本所在目录**，与 cwd 无关），
//	      上溯 ≤6 层找含 `.git` 目录或 `README.md` 文件的目录，落空回落 `os.getcwd()`。
//	    Go **无 `__file__` 语境** ⇒ `cdocFindRepoRoot(cwd)`：从 **cwd** 起**同一判据（`.git` 目录 或
//	      `README.md` 文件，`os.Stat` 跟随）** 上溯 **6** 层，落空回落 `cwd`（＝ py 的 `os.getcwd()`）。
//	    ⇒ 「脚本位置 == cwd 上溯根」时两者**逐字相同**（对拍乙环境已证：py `__file__` 上溯 == Go cwd 上溯 == fixture 根）；
//	      否则**路径推导面不同**（如本件从仓根跑、而 py 在 `scripts/gates/` 下 ⇒ 两者都上溯到仓根 ⇒ 仍同）。
//	    ★ **显式入参旋钮面**：py 的**唯一**外部树旋钮 = **位置参数 `<仓库根>`**（`sys.argv` 里首个非 `-` 项）；
//	      py **无**同名旗标 / **无**同名 env ⇒ 本件**同名同序照搬该位置参数**（✗ 不另起 `--root`，✗ 不加 env）——
//	      这是**有意**的旋钮面：外部树一律**显式**给位置参数（缺省档两态都走 `find_repo_root` 推导）。
//	      （旋钮面差：py 位置参数 = 本件位置参数，**合同**；无 env 面差 —— 见未覆盖面⑦。）
//	(b) **自指检查** —— 本件**不在**自己的扫描面内 ⇒ **自指 = 假**（四形态逐条）：
//	    ① **扫描面** = `<root>` 下 **`*.md` / `*.markdown`**（py `:43` 的 `endswith((".md",".markdown"))`）；
//	       本件 = `core/cmd/zerg/check_docs.go`（**`.go`** 且**不在**任何 `.md`/`.markdown` 面内）
//	       ⇒ 缺省根下**永不含本件**（真仓 `core/` 下 `.go` 件不匹配 `.md` 后缀）。
//	    ② 本件**零命中词私路径字面量**（`/Volumes/…` / `/Users/…` 形态现读 **0** 处）—— 与
//	       `check-hardcoded-private-paths.py`（其 `SCAN_DIRS` 含 `core/`）同族纪律；注释行亦无。
//	    ③ 本件正文里出现的判定关键词（后缀常量 / 正则模板 / 打印文案）**不落进任何 `.md` 件** ⇒
//	       不会给自己造出「新的被扫件」。
//	    ④ `--self-test` 的合成 `.md` 件只写在**临时目录**里（仓外）⇒ 不落进 `<root>`。
//	    ★ **「一旦被跟踪」前瞻消解**（新落件此刻未被 `git ls-files` 跟踪，故按跟踪后逐条排）：
//	      · `check-hardcoded-private-paths.py`：本件**无**该门的三枚命中词（私有绝对路径字面量）
//	        —— 代码行与注释行皆 **0** ⇒ 贡献 **0 处**。
//	      · `check-error-kinds.py` R3（扫 `core/cmd/zerg/*.go` non-test）：本件**无**用法面 kind
//	        字面量（既无 setErr 调用形态、也无 JSON 的 kind 键形态）⇒ 贡献 **0 个**。
//	      · `check-wall-layer.py`（扫同一批 `.go`）：本件**无**层级常量（`layer` 前缀那三条）、
//	        **无**命令树里的 `layer` 键标记 ⇒ 贡献 **0 处**。
//	        ★ 血证（自指第四形态 · 本单实测）：初版正文里写了该键的**带引号字面形态** ⇒ 该门 R2
//	        命中 1 处、真仓由 rc=0 翻 rc=1 ⇒ 已改**描述式**表述；改后实跑该门 rc=0，且 stdout
//	        sha256 与落件前**逐字节同**（`f0c3df521bda4666…`）。
//	      ⇒ **遗留悬空字面计数 = 0 / 0 / 0**（三面各 0）。
//
// 已对拍面（仓外夹具两态逐字 · 甲=真仓+显式传参 / 乙=仓外夹具+每夹具自放 py 副本·不传根）：
//
//	真仓现读（缺省根）· 显式**绝对路径**传根 · 全绿快照布局（README 双语对 + G3 链接）· 失效链接（G3）·
//	缺英文版（G1 正）· 缺中文版（G1 反）· 标题层级不一致（G2）· 代码块数不一致（G2）· 非快照布局（提示 + 跳 G3）·
//	空面（0 件 · 双语对 0）· `--strict`（仅 warn ⇒ 0 ⟷ 1）· 未知旗标（用法错 rc=2）· 读不了的件（chmod 000）·
//	`--self-test` 三档 · **子目录夹在 readdir 原序中间**的夹具格。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **用法 / 帮助正文**：py 无 argparse，未知旗标打的是**它自己的**中文用法错（本件**逐字复刻** stdout/stderr
//	   两行文案）⇒ 该面已对齐；但 py 无 `-h/--help`（`-h` 会被当未知旗标 ⇒ rc=2）⇒ 本件同（`-h` 亦 rc=2）。
//	② **默认根定位**：见标准声明行 (a) —— `__file__` ⟷ cwd 的语境不同。
//	③ **非 UTF-8 角落**：py `open(..., errors="replace")` 按**非法序列**逐段替换 U+FFFD；Go `strings.ToValidUTF8`
//	   按**非法字节**逐个替换 ⇒ 极端角落的正文字节可能不同（真仓 `.md` 现读全为合法 UTF-8 ⇒ 不触发）。
//	④ **`--self-test` 手段**：py 用 `subprocess.run([sys.executable, SELF_PATH])` **真跑自身**；本件**进程内**
//	   直调入口（同 argv · 读数一致）。⇒ 打印文案逐字对齐，差在**实现手段**（子进程 ⟷ 进程内）。
//	   ★ 临时目录名带随机后缀（`mkdtemp`）⇒ 自检档属**非确定面**（掩码随机后缀后逐字节比；随机后缀**不进**
//	   任何打印行 ⇒ 实测两态打印面逐字节同）。
//	⑤ **`str.strip()` 全集**：py 剥 `\x1c-\x1f`，Go `strings.TrimSpace` 不剥 ⇒ 链接目标含这四个控制符时不同（不触发）。
//	⑥ **读失败档（真口子 · C 类＋B 类混合 · py 现读硬伤）**：py `open()` 无兜底、`__main__` 裸 `sys.exit(main())`
//	   ⇒ 读不了的 `.md`（权限 000 / 悬空链接 / 是目录）抛未捕异常 = **回溯 + rc=1**；`os.walk` 走 `onerror=None`
//	   ⇒ **读不了的目录被静默跳过**（B 类）。本件按**新口径 fail-closed**：读不到即 **rc=2 + stderr 具名**（不给结论）；
//	   **目录读失败逐字沿用 py 静默**（不点名、不影响退码）。⇒ 该格**只保 rc 之外的正文不同**（单列，不混进逐字承诺）。
//	⑦ **旋钮面**：py 只有位置参数，**无同名旗标 / 无 env** ⇒ 本件不另设旗标/env（有意不留第二旋钮面）。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// cdocVersion —— 本件标识（py 无版本串 ⇒ 此处只作本件标识，不进任何输出面）。
const cdocVersion = "check-docs.go v1.0.0"

const (
	// cdocCJKSuffix / cdocENSuffix —— py `:24-25` 的两个后缀常量。
	cdocCJKSuffix = ".zh-CN.md"
	cdocENSuffix  = ".en.md"
	// cdocWS —— py `\s`（Unicode，`_PyUnicode_IsWhitespace` 认的集合）的显式码点展开。
	cdocWS = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}` +
		`\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`
)

// cdocLink —— py `:26` `LINK = re.compile(r'\]\(([^)#\s]+)(#[^)]*)?\)')`。`\s` 一枚已展开。
var cdocLink = regexp.MustCompile(`\]\(([^)#` + cdocWS + `]+)(#[^)]*)?\)`)

// cdocSkipDirs —— py `:41` 的 `dirs[:]` 过滤集（**保序**过滤，不下钻）。
var cdocSkipDirs = []string{".git", "target", "node_modules", "vendor"}

// cdocFindings —— 一条 err/warn（py 三元组 `(code, 相对路径, 消息)`，**切片保插入序**）。
type cdocFindings struct {
	Code string
	Rel  string
	Msg  string
}

// ─────────────────────────── py 词法面同构件（✗ 一律不用 filepath.*）───────────────────────────

// cdocPyJoin 复刻 posixpath.join —— 按 CPython 逐段规则直译（★ 三类语义差一次写全）。
// ✗ 别手工「跳过空段」，那会把 "a/" 变成 "a"。
func cdocPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "/"): // ② 后段绝对 ⇒ 重置前面所有段
			out = p
		case out == "" || strings.HasSuffix(out, "/"): // 首段 / 上一段落点自带分隔符（含 ③ 空段）
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}

// cdocPyNormpath 复刻 posixpath.normpath。★ 保留「开头斜杠数」：`//a` 不归一成 `/a`。
func cdocPyNormpath(path string) string {
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
			// 跳过空段与当前目录
		case comp != ".." || (initialSlashes == 0 && len(comps) == 0) || (len(comps) > 0 && comps[len(comps)-1] == ".."):
			comps = append(comps, comp)
		default:
			if len(comps) > 0 {
				comps = comps[:len(comps)-1] // 弹掉上一段
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

// cdocPyAbspath 复刻 posixpath.abspath：**纯词法**（不碰盘、不解析符号链接）。
// ★★ cwd 由调用方传入（✗ 别在函数里 os.Getwd()：$PWD 不可信时会解析真路径 ⇒ 夹具落符号链接路径时假失配）。
func cdocPyAbspath(cwd, path string) string {
	return cdocPyNormpath(cdocPyJoin(cwd, path))
}

// cdocPyDirname 复刻 posixpath.dirname（= posixpath.split 的头段：剥尾斜杠、全斜杠头保原样）。
func cdocPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// cdocPyRelpath 复刻 posixpath.relpath（start 传已 abspath 的 root）。
func cdocPyRelpath(path, start string) string {
	split := func(p string) []string {
		var out []string
		for _, c := range strings.Split(p, "/") {
			if c != "" {
				out = append(out, c)
			}
		}
		return out
	}
	s, p := split(start), split(path)
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
		return "."
	}
	return strings.Join(rel, "/")
}

// ─────────────────────────── py stat / walk 面同构件 ───────────────────────────

// cdocExists —— py `os.path.exists`（stat 跟随符号链接；悬空链接 ⇒ false）。
func cdocExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// cdocStatIsDir —— py `os.path.isdir` / `entry.is_dir()`（**stat 跟随**符号链接）。
func cdocStatIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// cdocIsLink —— `os.path.islink`（**lstat** 语义）。os.walk(followlinks=False) 的下钻前置判据。
func cdocIsLink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

// cdocRecognized —— py `f.endswith((".md", ".markdown"))`。
func cdocRecognized(name string) bool {
	return strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".markdown")
}

// cdocDocFiles 复刻 py `doc_files`（`:38-45`）：os.walk(followlinks=False) + dirs 保序过滤 + 末尾 sorted。
// ⓐ 同层读取用 `f.ReadDir(-1)`（readdir 原序）· ⓑ 先收当前层文件再按 dirs 原序下钻 · ⓒ dirs 保序过滤 ·
// ⓓ 末尾 `sort.Strings` = **输出行序的决定者**（py 明写 `sorted(out)`）。
func cdocDocFiles(root string) []string {
	var out []string
	var walk func(dir string)
	walk = func(dir string) {
		f, err := os.Open(dir)
		if err != nil {
			return // py os.walk(onerror=None) ⇒ 读不了的目录**静默跳过**（B 类，逐字沿用）
		}
		entries, err := f.ReadDir(-1)
		_ = f.Close()
		if err != nil {
			return
		}
		var dirs, files []string
		for _, e := range entries {
			if cdocStatIsDir(cdocPyJoin(dir, e.Name())) { // ★ 分类跟随符号链接（stat），✗ 非 e.IsDir()
				dirs = append(dirs, e.Name())
			} else {
				files = append(files, e.Name())
			}
		}
		kept := make([]string, 0, len(dirs))
		for _, d := range dirs { // ⓒ 保序过滤
			skip := false
			for _, s := range cdocSkipDirs {
				if d == s {
					skip = true
					break
				}
			}
			if !skip {
				kept = append(kept, d)
			}
		}
		for _, fn := range files { // ⓑ 先收当前层全部文件
			if cdocRecognized(fn) {
				out = append(out, cdocPyJoin(dir, fn))
			}
		}
		for _, d := range kept { // ⓑ 再按 dirs 原序下钻；链接目录**录入不下钻**
			p := cdocPyJoin(dir, d)
			if !cdocIsLink(p) {
				walk(p)
			}
		}
	}
	walk(root)
	sort.Strings(out) // ⓓ ★★ 输出行序的决定者
	return out
}

// ─────────────────────────── 读取 / 正文面 ───────────────────────────

// cdocReadText —— py `open(path, encoding="utf-8", errors="replace").read()`。
// 读失败 ⇒ 返回 err（本件按新口径 fail-closed rc=2，✗ 不静默、✗ 不回落空串）。
func cdocReadText(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return strings.ToValidUTF8(string(b), "\uFFFD"), nil
}

// cdocSplitLines —— py 文本迭代的**通用换行**语义（`\r\n` / `\r` 皆视作断行）。
func cdocSplitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

// cdocHeadingShape —— py `heading_shape`（`:48-56`）：标题层级序列 + 代码块数。
func cdocHeadingShape(path string) ([]int, int, error) {
	txt, err := cdocReadText(path)
	if err != nil {
		return nil, 0, err
	}
	levels := []int{}
	blocks := 0
	for _, line := range cdocSplitLines(txt) {
		if strings.HasPrefix(line, "#") {
			levels = append(levels, len(line)-len(strings.TrimLeft(line, "#")))
		}
		if strings.HasPrefix(line, "```") {
			blocks++
		}
	}
	return levels, blocks / 2, nil
}

// cdocPyListRepr —— py `str(list)` 的逐字同构（`%s` 作用于 list 时的形态）。
func cdocPyListRepr(xs []int) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, fmt.Sprintf("%d", x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cdocFindRepoRoot —— py `find_repo_root`（`:29-35`）的 cwd 版（见标准声明 (a)）。
func cdocFindRepoRoot(cwd string) string {
	d := cwd
	for i := 0; i < 6; i++ {
		if cdocStatIsDir(cdocPyJoin(d, ".git")) || cdocIsFile(cdocPyJoin(d, "README.md")) {
			return d
		}
		d = cdocPyDirname(d)
		if d == "" {
			break
		}
	}
	return cwd
}

// cdocIsFile —— py `os.path.isfile`（stat 跟随符号链接）。
func cdocIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// ─────────────────────────── 主检查体 ───────────────────────────

// cdocMain —— 主检查体（py `main()` 去掉 self-test 分派后的正文，含用法错判）。cwd 由调用方传入。
func cdocMain(argv []string, cwd string, stdout, stderr io.Writer) int {
	// ── 未知旗标（py `main` 的首道判：**不许**静默忽略 ⇒ rc=2，判在一切读盘之前）──
	var unknown []string
	for _, a := range argv {
		if strings.HasPrefix(a, "-") && a != "--strict" {
			unknown = append(unknown, a)
		}
	}
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "用法错：本件不认的旗标 %s ⇒ rc=2（**不许**静默忽略）\n", strings.Join(unknown, " "))
		fmt.Fprintln(stderr, "用法：python3 scripts/gates/check_docs.py [仓库根] [--strict]")
		return 2
	}
	pos := make([]string, 0, len(argv))
	for _, a := range argv {
		if !strings.HasPrefix(a, "-") {
			pos = append(pos, a)
		}
	}
	strict := false
	for _, a := range argv {
		if a == "--strict" {
			strict = true
		}
	}
	root := ""
	if len(pos) > 0 {
		root = cdocPyAbspath(cwd, pos[0])
	} else {
		root = cdocFindRepoRoot(cwd)
	}

	files := cdocDocFiles(root)
	// ★ 空集守卫（GAP-20260927-108）：空域 ⇒ fail-closed（扫面 0 件 ⇒ rc=2，不许把「空转」当绿）
	if len(files) == 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 空域：%s 下扫到 0 个 *.md/*.markdown 文档件 ⇒ rc=2（不给结论；空转 ≠ 绿）\n", root)
		return 2
	}

	type pair struct{ zh, en string }
	var pairs []pair
	var errs, warns []cdocFindings

	// G1 正向：每个 *.zh-CN.md 必须有对应 *.en.md（README.zh-CN.md ↔ README.md）
	for _, f := range files {
		b := cdocBase(f)
		if strings.HasSuffix(b, cdocCJKSuffix) {
			zh := f
			var en string
			if b == "README.zh-CN.md" {
				en = cdocPyJoin(cdocPyDirname(f), "README.md")
			} else {
				en = f[:len(f)-len(cdocCJKSuffix)] + cdocENSuffix
			}
			if cdocExists(en) {
				pairs = append(pairs, pair{zh, en})
			} else {
				errs = append(errs, cdocFindings{"G1", cdocPyRelpath(zh, root),
					fmt.Sprintf("缺少对应英文版 %s", cdocBase(en))})
			}
		}
	}
	// G1 反向：*.en.md 必须有中文版
	for _, f := range files {
		if strings.HasSuffix(cdocBase(f), cdocENSuffix) {
			zh := f[:len(f)-len(cdocENSuffix)] + cdocCJKSuffix
			if !cdocExists(zh) {
				errs = append(errs, cdocFindings{"G1", cdocPyRelpath(f, root),
					fmt.Sprintf("缺少对应中文版 %s", cdocBase(zh))})
			}
		}
	}

	// G2 结构对齐 / G3 链接 / G4 源声明
	snapshot := cdocExists(cdocPyJoin(root, "README.zh-CN.md"))
	if !snapshot {
		fmt.Fprintln(stdout, "提示: 当前不是快照布局（未在根找到 README.zh-CN.md）→ 跳过 G3 链接检查（链接按快照布局书写）")
	}
	for _, pr := range pairs {
		zh, en := pr.zh, pr.en
		lz, bz, err := cdocHeadingShape(zh)
		if err != nil {
			fmt.Fprintf(stderr, "读不了输入件: %s: %v\n", zh, err)
			return 2
		}
		le, be, err := cdocHeadingShape(en)
		if err != nil {
			fmt.Fprintf(stderr, "读不了输入件: %s: %v\n", en, err)
			return 2
		}
		if !cdocIntsEqual(lz, le) {
			errs = append(errs, cdocFindings{"G2", cdocPyRelpath(en, root),
				fmt.Sprintf("标题层级序列不一致：中文 %d 个 %s / 英文 %d 个 %s",
					len(lz), cdocPyListRepr(cdocHeadN(lz, 12)), len(le), cdocPyListRepr(cdocHeadN(le, 12)))})
		}
		if bz != be {
			errs = append(errs, cdocFindings{"G2", cdocPyRelpath(en, root),
				fmt.Sprintf("代码块数不一致：中文 %d / 英文 %d", bz, be)})
		}
		if snapshot {
			for _, p := range []string{zh, en} {
				rel := cdocPyRelpath(p, root)
				txt, err := cdocReadText(p)
				if err != nil {
					fmt.Fprintf(stderr, "读不了输入件: %s: %v\n", p, err)
					return 2
				}
				for _, m := range cdocLink.FindAllStringSubmatch(txt, -1) {
					t := strings.TrimSpace(m[1])
					if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") ||
						strings.HasPrefix(t, "mailto:") || strings.HasPrefix(t, "#") {
						continue
					}
					if !cdocExists(cdocPyNormpath(cdocPyJoin(cdocPyDirname(p), t))) {
						errs = append(errs, cdocFindings{"G3", rel, fmt.Sprintf("失效链接: %s", t)})
					}
				}
			}
		}
		zt, err := cdocReadText(zh)
		if err != nil {
			fmt.Fprintf(stderr, "读不了输入件: %s: %v\n", zh, err)
			return 2
		}
		et, err := cdocReadText(en)
		if err != nil {
			fmt.Fprintf(stderr, "读不了输入件: %s: %v\n", en, err)
			return 2
		}
		if !strings.Contains(et, "Source of truth") {
			warns = append(warns, cdocFindings{"G4", cdocPyRelpath(en, root), "英文版缺少 'Source of truth' 源声明"})
		}
		if !strings.Contains(zt, "唯一真相源") {
			warns = append(warns, cdocFindings{"G4", cdocPyRelpath(zh, root), "中文版缺少 '唯一真相源' 声明"})
		}
	}

	fmt.Fprintf(stdout, "仓库根: %s\n", root)
	fmt.Fprintf(stdout, "双语对: %d 对\n", len(pairs))
	for _, e := range errs {
		fmt.Fprintf(stdout, "  ✗ [%s] %s — %s\n", e.Code, e.Rel, e.Msg)
	}
	for _, w := range warns {
		fmt.Fprintf(stdout, "  ! [%s] %s — %s\n", w.Code, w.Rel, w.Msg)
	}
	if len(errs) == 0 && len(warns) == 0 {
		fmt.Fprintln(stdout, "  ✓ 配对完整 / 标题对齐 / 链接有效 / 源声明齐备")
	}
	failed := len(errs) > 0 || (strict && len(warns) > 0)
	res := "通过"
	if failed {
		res = "失败"
	}
	fmt.Fprintf(stdout, "\n结果: %s（error %d / warn %d）\n", res, len(errs), len(warns))
	if failed {
		return 1
	}
	return 0
}

// cdocBase —— py `os.path.basename`（posixpath：剥尾斜杠后取末段）。
func cdocBase(p string) string {
	i := strings.LastIndex(p, "/")
	return p[i+1:]
}

// cdocIntsEqual —— py 列表 `==`（逐元素）。
func cdocIntsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// cdocHeadN —— py `xs[:n]`（**保序**前 n 项）。
func cdocHeadN(xs []int, n int) []int {
	if len(xs) <= n {
		return xs
	}
	return xs[:n]
}

// ─────────────────────────── 自检（成对负控 · 合成夹具 · 不碰真目标）───────────────────────────

// cdocTmpBase —— py `_tmp_base`：`TMPDIR` 指名且是目录 ⇒ 用它；否则交回系统默认。
func cdocTmpBase() string {
	d := os.Getenv("TMPDIR")
	if d != "" && cdocStatIsDir(d) {
		return d
	}
	return os.TempDir()
}

// cdocStChk —— py `_st_chk`（`:70-76`）：比退码、不合则打「末行[:90]」尾巴，返回是否合。
func cdocStChk(bad *int, label string, want, rc int, out string, stdout io.Writer) bool {
	good := rc == want
	if !good {
		*bad++
	}
	tail := ""
	if !good {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		last := ""
		if len(lines) > 0 {
			last = lines[len(lines)-1]
		}
		if len(last) > 90 {
			last = last[:90]
		}
		tail = " ｜ " + last
	}
	fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", map[bool]string{true: "✓", false: "✗"}[good], label, want, rc, tail)
	return good
}

// cdocSelfTest —— py `self_test`（`:79-107`）：三档（正控 0 · 负控 1 · 用法错 2），finally 清临时目录。
func cdocSelfTest(stdout, stderr io.Writer) int {
	bad := 0
	fmt.Fprintln(stdout, "── check_docs.py --self-test 三档（正控 0 · 负控 1 · 用法错 2）")
	tmp, err := os.MkdirTemp(cdocTmpBase(), "docs-selftest-")
	if err != nil {
		fmt.Fprintf(stderr, "建临时目录失败: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tmp) // ★ 自回收：与 py `finally: shutil.rmtree(tmp, ignore_errors=True)` 同效

	mk := func(name, zhBody, enBody string) (string, error) {
		root := cdocPyJoin(tmp, name)
		if err := os.MkdirAll(root, 0o755); err != nil {
			return "", err
		}
		for _, kv := range [][2]string{{"guide.zh-CN.md", zhBody}, {"guide.en.md", enBody}} {
			if err := os.WriteFile(cdocPyJoin(root, kv[0]), []byte(kv[1]), 0o644); err != nil {
				return "", err
			}
		}
		return root, nil
	}

	good, err := mk("good", "# 标题\n\n## 小节\n\n正文\n", "# Title\n\n## Section\n\nbody\n")
	if err != nil {
		fmt.Fprintf(stderr, "建夹具失败: %v\n", err)
		return 1
	}
	badRoot, err := mk("bad", "# 标题\n\n## 小节\n\n正文\n", "# Title\n\n正文\n")
	if err != nil {
		fmt.Fprintf(stderr, "建夹具失败: %v\n", err)
		return 1
	}

	run := func(a ...string) (int, string) {
		var so, se strings.Builder
		rc := cdocMain(a, tmp, &so, &se)
		return rc, so.String() + se.String()
	}

	rc, out := run(good)
	cdocStChk(&bad, "档① 正控：双语配对 + 标题层级对齐 ⇒ rc=0", 0, rc, out, stdout)
	rc, out = run(badRoot)
	cdocStChk(&bad, "档② 负控：英文版少一个标题层级 ⇒ G2 判红 rc=1", 1, rc, out, stdout)
	rc, out = run("--no-such-flag")
	cdocStChk(&bad, "档③ 用法错：不认的旗标 ⇒ rc=2（不静默忽略）", 2, rc, out, stdout)

	if bad == 0 {
		fmt.Fprintln(stdout, "── 自检：全过 ✓")
		return 0
	}
	fmt.Fprintf(stdout, "── 自检：**%d 档不过** ✗\n", bad)
	return 1
}

// ─────────────────────────── 入口 ───────────────────────────

// checkDocsGateMain —— 与 py `main()`（`:110-187`）对齐的入口（先分派 `--self-test`，其后 = `cdocMain`）。
func checkDocsGateMain(stdout, stderr io.Writer, argv []string) int {
	for _, a := range argv {
		if a == "--self-test" || a == "--selftest" {
			return cdocSelfTest(stdout, stderr)
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "取不到工作目录: %v\n", err)
		return 2
	}
	return cdocMain(argv, cwd, stdout, stderr)
}
