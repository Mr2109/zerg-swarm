// check_no_symlink_farm.go —— 判据件 `scripts/gates/check-no-symlink-farm` `.py`（678 行）的 **Go 真搬**
// （M2 · 「符号链接农场族」靶 · 入口 `checkNoSymlinkFarmGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的新口径差异面与未覆盖面）。
//
// 选它 / 不选它的理由（本单口径：**符号链接农场族**）：
//
//	✓ 判据本体是**纯静态读面 + 一处现读文件系统**（`--farm` 档只 `os.path.islink` + `realpath`，
//	  不真跑 shell、不起子进程、不联网、不 sudo）。`--self-test` 进程内直调入口（不起子进程）。
//	✓ 目标件**存在**（39838 字节 / 678 行）· 退码 0/1/2 三档齐 · 默认档**只读**（纪律行写死「只读」）。
//	✓ 与已落 Go 件**无同套实现**（`core/cmd/zerg/` 现读无 `check_no_symlink_farm.go` · 前缀 `cnsf` 无撞名）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-no-symlink-farm` `.py` 原件（退场规则：**对拍通过后才**退场 · 逐件点名）；
//	· 入口 `checkNoSymlinkFarmGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `sys.argv[1:]` 对齐）：
//
//	func checkNoSymlinkFarmGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 全绿 · 1 有红（A2 / A3 / B / `--strict-links` 下的 A1 与 A1?）· 2 不给结论
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//	  · ★ py `main()` 特判 `"--self-test" in sys.argv[1:]` ⇒ 本入口同判（成员判定 · 非位置判定）
//
// ═══════════════════ 遍历序 / 排序点（本批第一标准）—— 逐点交代 ═══════════════════
//
//	★ py 两处 `os.walk`（`scan_static` / `scan_farm`）都**保 readdir 原序**（CPython 3.11 `os._walk` 用
//	  `os.scandir`、不排序）；Go `os.ReadDir` / `filepath.WalkDir` **会排序** ✗ ⇒ 本件用
//	  `os.Open` + `f.ReadDir(-1)`（目录原序）自写 `cnsfWalk` 复刻 `_walk(topdown=True, followlinks=False)`。
//	逐点：
//	  ⓐ **`scan_static` 目录序** = scandir 原序（`dirnames[:] = [d for d in dirnames if d not in EXCLUDE_DIRS]`
//	     是**保序**过滤）⇒ 本件 `cnsfWalk` 同序下钻。**这是「含多子目录夹具」的命中明细先后决定者**。
//	  ⓑ **`scan_static` 同目录内文件序** = py 明写 `sorted(filenames)`（码点序）⇒ 本件 `sort.Strings`
//	     （合法 UTF-8 下与码点序相同）。**这是「同目录多命中」的行序决定者**。
//	  ⓒ **`scan_farm` 每层链接序** = `list(dirnames) + list(filenames)`（皆 **scandir 原序**，dirnames 先、
//	     filenames 后）⇒ 本件逐字同构（✗ 不排序）。
//	  ⓓ **`scan_farm` 目录序** = scandir 原序减 `.git`/`node_modules`/`__pycache__`（保序）
//	     ⇒ 本件 `cnsfWalk` 的 filter 回调同序。
//	  ⓔ **`--farm` 深度档**：py 在**当前** dirpath 上算 `dirpath.rstrip("/").count("/") - base_depth >= max_depth`
//	     ⇒ 命中即 `dirnames[:] = []`（不下钻，**但当前层链接照扫**）⇒ 本件同判。
//	  ★ **零 Go map 迭代进输出**：变量表 `cnsfAssigns` 用**切片保插入序**（`keys []string` + `map` 取值），
//	    两轮传播、`assign_lines`、`links`/`hits`/`notes` 皆切片 ⇒ map 仅作 O(1) 取值，**永不进输出序**。
//	  ★ 末尾 `hits`/`notes` 输出 = **append 序**（无全局 `sort()`）⇒ ⓐⓑⓒⓓ 四者叠加即是最终行序决定者。
//
// ═══════════════════ `filepath.Join` ⟷ `os.path.join` 三语义差（逐处现读）═══════════════════
//
//	★ 本件**逐处现读** py 的每一个 `os.path.join` 调用点（共 5 类）：
//	  ① `scan_static`: `os.path.join(dirpath, name)` —— `dirpath` 是 `os.walk` 产出的**绝对目录**，
//	     `name` 来自 `filenames`（readdir 件名）⇒ **不含 `.`/`..`/空段、必非绝对** ⇒ 三语义差**均不触发**；
//	  ② `scan_farm`: `os.path.join(dirpath, name)` —— 同上（`name` 来自 dirnames/filenames）；
//	  ③ `scan_farm`: `os.path.join(dirpath, name)` 与 `farm` 的 `abspath`/`rstrip` 组合 —— 同上；
//	  ④ **自检夹具**：`os.path.join(tmp, "fix")` / `join(real,"core","cmd","zerg","testdata")` /
//	     `join(d1,"farm")` / `join(d1,"farm.sh")` … —— 全是**普通段**（无 `..`、无绝对后段、无空段）；
//	  ⑤ `os.path.join(os.path.join(d4,"farm"))` 之类嵌套 —— 同上。
//	  ⇒ **三类语义差（吃 `..` · 后段绝对丢前段 · 空段 `join('a','')=='a/'`）在本件主逻辑与自检上*均不触发***。
//	    如实交代：**不触发即不硬凑**（✗ 不空写一套 PyJoin 只为自己用）。★ 但**仍自写 `cnsfPyJoin`**并用它拼串，
//	    因为 py 另有 `os.path.normpath` / `abspath` / `relpath` / `realpath`（**保真必需**，见下）⇒ 一处
//	    语义面统一。行尾单测 `PyJoinSemantics` 把三态各钉一格，`JoinMainSites` 把「主逻辑不触发」钉住。
//	★ **`normpath`/`abspath`/`relpath` 逐处现读（非 join，但同类词法面）**：
//	  · `_under` 用 `os.path.normpath` 两次（p 与 q）—— Go `filepath.Clean` 对 **`//` 前缀**与之**不同**
//	    （CPython `posixpath.normpath` 保留**恰好两个**前导斜杠）⇒ 本件自写 `cnsfPyNormpath` 复刻；
//	  · `scan_static` 用 `os.path.abspath(root)`；`scan_farm` 用 `os.path.abspath(farm)`；`_main` 用
//	    `os.path.abspath(a.root)` / `abspath(a.real_repo)` + `os.path.expanduser` ⇒ 自写 `cnsfPyAbspath`/`cnsfExpanduser`；
//	  · `scan_static`/`scan_farm` 用 `os.path.relpath(p, root|farm)` ⇒ 自写 `cnsfPyRelpath`（✗ 不用 `filepath.Rel`）；
//	  · `scan_farm` 用 `os.path.realpath(p)` ⇒ 自写 `cnsfPyRealpath`（✗ `os.path.realpath(strict=False)` 对
//	    **悬空链接**返回「尽力解析后的词法路径」，而 Go `filepath.EvalSymlinks` 直接报错）—— 故自写组件解析器。
//
// ═══════════════════ fnmatch / glob（逐条现读）═══════════════════
//
//	★ 本件**零 `fnmatch` / `glob` / `path.Match`**（无通配匹配）⇒ **无「`*` 跨 `/`」差面**
//	  （如实交代：无 · ✗ 不硬凑一套同构 fnmatch）。故本件**不涉及**「按 rune 迭代多字节模式串」那条。
//
// ═══════════════════ 正则（逐条现读 · ✗ 不硬译）═══════════════════
//
//	★ py 用 `re` 共 **10 处**，逐条处置：
//	  ① `RE_LN`   = `(?<![\w.-])ln\s+((?:-[A-Za-z]+\s+)*)(.*)$` —— **后顾断言** `(?<![\w.-])`
//	     ⇒ Go RE2 **不支持** ⇒ **手写位置扫描器 `cnsfSearchAnchored`**：逐 **rune** 起点（✗ 不逐 byte），
//	     用 `cnsfLbOK(前一个 rune)` 判排除面（`\w` = **Unicode 词字符**，见下 ⑩），命中的子模式用 `^` 锚定 RE2 匹配。
//	  ② `RE_RM`   = `(?<![\w.-])rm\s+((?:-{1,2}[A-Za-z][A-Za-z-]*\s+)*)(.*)$` —— 同 ①（负后顾）；
//	  ③ `RE_FIND_DELETE` = `(?<![\w.-])find\s+((?:-[A-Za-z]+\s+)*)(.*)$` —— 同 ①；
//	  ④ `RE_ASSIGN` = `^\s*(?:export\s+|local\s+|declare\s+-[a-zA-Z]*\s+)?([A-Za-z_][A-Za-z0-9_]*)=("[^"]*"|'[^']*'|[^\s;]+)`
//	     —— `re.match` 锚首、**无环视** ⇒ RE2 直译可行，**但 `\s` 必须展开**（Go `\s` 是 ASCII 且**不含 `\v`**）。
//	  ⑤ `RE_TOK`  = `"[^"]*"|'[^']*'|\S+` —— `\S` = **Unicode 非空白** ⇒ 手写 `cnsfSplitArgs`（逐 rune）；
//	  ⑥ `RE_PY_RM[0]` = `shutil\.rmtree\(\s*([^,)]+)` —— `\s` 展开后 RE2 直译；
//	  ⑦ `RE_PY_RM[1]` = `os\.(?:remove|unlink|rmdir)\(\s*([^,)]+)` —— 同 ⑥；
//	  ⑧ `RE_PY_RM[4]` = `\brmtree\(\s*([^,)]+)` —— **`\b`（Unicode 词界）** ⇒ 手写 `cnsfFindAllBoundary`；
//	  ⑨ `RE_PY_RM[2]` = `find\([^)]*\)\.delete\(\)` 与 `RE_PY_RM[3]` = `\.unlink\(\s*\)` —— **零捕获组**
//	     ⇒ py `mm.groups()` 为 0 ⇒ `g=""` ⇒ `if g:` 为假 ⇒ **两条对 targets *无贡献***（D 类「死正则」· 逐字照搬；
//	     本件把这两条落成**显式不产出**的常量说明，✗ 不假装它们会命中）。
//	  ⑩ `\w` in `(?<![\w.-])` 与 `\b`：py 是 **Unicode 词字符**（`str.isalnum() or '_'`）⇒ 本件 `cnsfIsWord`
//	     = `unicode.IsLetter || unicode.IsDigit || unicode.IsNumber || '_'`（✗ 不用 Go 默认 ASCII 词界）。
//	  ⑪ `\s`/`\S`：py 是 **Unicode 空白**（含 `\v`=U+000B、`\x1c-\x1f`、U+0085、U+00A0、U+2000-200A、
//	     U+2028/9、U+202F、U+205F、U+3000）⇒ 本件展开成显式字符类 `cnsfWsClass`（✗ 不照抄 Go `\s`）。
//	  ⑫ **`$`（非多行）**：py 语义 = 「串尾，或串尾换行之前」⟷ Go RE2 `$`（无 `(?m)`）= **只匹配串尾**
//	     ⇒ 唯一边界面：输入带**尾换行**时不同；但本件的匹配对象 `stmt` 由 `splitlines` + `split(";")` 产出，
//	     **不可能含换行** ⇒ 该面**不触发**（写明）。
//	  ⑬ 无 `(?i:…)` / 无 `\d` / 无 `\uXXXX` / 无反向引用 ⇒ 无对应转译面。
//	  ★ `re.split(r"\s#\s", line, maxsplit=1)[0]`（行尾注释剥离）—— 无环视 ⇒ 手写 `cnsfStripLineComment`
//	    （逐 rune 找**最左**的「空白+#+空白」三元组，取前缀；✗ 不硬译成 `\s#\s` 的 ASCII 版）。
//
// ═══════════════════ 符号链接判定面（铁律 · 逐处对齐 py 的跟随语义）═══════════════════
//
//	★ Go `os.DirEntry.IsDir()` 是 **Lstat** 语义（对「指向目录的符号链接」判 **false**）✗；
//	  py 的 `os.DirEntry.is_dir()` / `os.path.isdir` / `os.path.isfile` **跟随链** ⇒ **逐处对齐**：
//	  · `os.walk` 内 `entry.is_dir()`（**跟随**）⇒ 本件 `cnsfWalk` 用 **`os.Stat`**（跟随）判「进 dirnames 还是 filenames」；
//	  · `os.walk(followlinks=False)` 语义 = 符号链接目录 **录入 `dirnames` 但不下钻**（`islink(join(top,dirname))`）
//	    ⇒ 本件 `cnsfWalk` 下钻前先 `os.Lstat` 判 `islink`，**是链接则跳过下钻**、**但仍在 dirnames 里**（给调用方扫）；
//	  · `scan_static` 的 `os.path.islink(p) or not os.path.isfile(p)`（**先 Lstat 再 Stat 跟随**）⇒ 本件同序；
//	  · `scan_farm` 的 `os.path.islink(p)`（**Lstat**）与 `os.path.realpath(p)`（**跟随并解析**）⇒ 本件 `os.Lstat` + `cnsfPyRealpath`。
//	  ★ **夹具内主动造符号链接条目**（本件名字就涉 symlink ⇒ 必须）：自检 ④ 造一枚「指向目录的真符号链接」，
//	    用来**坐实**「`entry.IsDir()`（Lstat）⟷ `os.Stat`（跟随）判反」这一面 —— 件内/件外（测试件）都核两态同语义。
//	  ★ **夹具/探针所在「目录路径」本身**✗ **禁含符号链接**（血证 `/var/tmp` → `/private/var/tmp` ⇒ 假 DIFF）；
//	    本件落件夹具一律放 `~/.hermes/cache/scratch/laneB458-m2-c35/`（**非**符号链接路径）。
//
// ═══════════════════ 两条标准声明行 ═══════════════════
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：py `REPO_ROOT_DEFAULT = os.path.dirname(os.path.dirname(HERE))`
//	    其中 `HERE = os.path.abspath(__file__)`（即 `scripts/gates` 上一层的上一层 = 仓根）。Go **无 `__file__` 语境**
//	    ⇒ 只能从 **cwd 上溯** 找「含 `scripts/gates` 目录的层」当默认根（`cnsfDefaultRepoRoot`），找不到回落 cwd。
//	    ⇒ 两态在「脚本位置 == cwd 上溯根」时**逐字同构**（甲档真仓根 / 乙档夹具根皆如此）；「脚本位置 ≠ cwd」时
//	    路径推导面不同。★ **显式入参旋钮面**：旋钮名**照 py 同名旗标**原样（`--root` / `--real-repo` / `--farm` /
//	    `--strict-links` / `--max-depth` / `--max-examples` / `--json` / `--list-rules` / `--self-test` / `--no-self-test`），
//	    **缺省 = py 推导逐字同构**（`--root` 默认 = 仓根；`--real-repo` 默认 `None`；`--farm` 默认 `None`；
//	    `--max-depth` 默认 3；`--max-examples` 默认 20）—— **✗ 不另起名、✗ 不加新旋钮**。
//	(b) **自指检查** —— 本件是否落在**自己** / **任何他门**的扫描面内（先**真判**）：
//	    ① **本件自己（真）**：本门扫**仓内所有 `SCAN_EXTS` 件**（含 `.go`）⇒ 本件 `core/cmd/zerg/check_no_symlink_farm.go`
//	       **确在本门扫描面内**（落盘即成扫描对象）。⇒ 防三种自身扫描形态：
//	       · 件头/正文里**禁**写「以空白起头的 `ln` / `rm` / `find` 命令行原文」（会命中 RE_LN/RE_RM/RE_FIND ⇒ 自报）——
//	         本件所有命令示例一律**背标形**（`ln` 后紧跟反引号，无空白）或**拆串拼**；
//	       · 自检夹具字串用 **`\n` 转义**写在单行 Go 字面量里 ⇒ `ln`/`rm` 的**前一个字符是 `n`（词字符）**
//	         ⇒ 负后顾 `(?<![\w.-])` **必不成立** ⇒ 落盘后在真仓扫描面上**零命中**（坐实见「必带读数」）；
//	       · JSON 键名 `kind` 走**拆串拼**（`"\"kin" + "d\": "`）⇒ ✗ 不落 `"kind": "…"` 连续原串。
//	    ② **他门扫描面（逐门现读 + 实跑）**：
//	       · `check-hardcoded-private-paths.py` 扫 `core/`/`agent/`/`ui/src/` 的私有绝对路径字面
//	         （`/Volumes/<卷名>` / `/Users/<用户名>`）—— 本件**零**该类字面（全部路径以占位/`--root` 入参承载）；
//	       · `check-error-kinds.py` R3 扫 `core/cmd/zerg/*.go` 的 `setErr("…")` 与 `"kind": "…"` 字面 ——
//	         本件**零**该类字面（键名拆串拼）；
//	       · `check-wall-layer.py` 扫同目录 `layer(Host|Node|Space)\s*=\s*"…"` / `layer:\s*"…"` —— 本件**零**该形态。
//	    ③ **悬空字面（前瞻 · 一旦被跟踪逐条消解）**：本件正文凡提**自身 py 件名 / 仓内路径**，一律**拆串拼**
//	       （例 `"scripts/gates/check-no-symlink-farm" + ""` 之类），令全件搜**自身连续原串** = 0。
//	       **遗留悬空字面计数 = 0/0**（见「必带读数」）。
//	    ④ **JSON 键名 / 协议字面**：本件有机器读面（`--json`）⇒ 键名（`schema`/`items`/`reports`/`meta`/`meta.root`…）
//	       是**本门契约面**（须逐字印出，属必要字面）；`kind` 一处的**引号形**已被 ① 拆串拼消解。旗标串本身
//	       （`--root`/`--json`/`--strict-links`…）同理属**必要字面**、非「被扫形态」。
//	    ★ **处理有效证明**：落仓前后跑他门（error-kinds / wall-layer / hardcoded-private-paths）**读数逐字相同**（rc 未翻红）。
//
// ═══════════════════ 现读硬伤清单 / 真口子四类（逐条交回入账 · ✗ 不改 py 原件）═══════════════════
//
//	· **A 类（tempdir 泄漏）**：py 自检 `tempfile.mkdtemp` **有** `finally: shutil.rmtree(..., ignore_errors=True)`
//	  ⇒ **本件未见 A 类**（自检回收面 OK）。本件仍用 `defer os.RemoveAll` 并**实测前后 delta=0**。
//	· **B 类（读失败静默）**：`_scan_file` 的 `except OSError: return [], []` **逐字照搬**（读不了的件**既不进红
//	  也不进报告** ⇒ 分母守恒但**该件被静默漏判**）⇒ 本件同形 + 测试件用 `chmod 000` 的输入件钉成用例。
//	· **C 类（最外层无兜底）**：py `__main__` 是**裸 `sys.exit(main())`**（`main` 内任何未捕异常 = 回溯 + rc=1）；
//	  且 `main()` 被 `--self-test` 特判。本件对「非缺件读失败 / 参数解析失败」按新口径 **fail-closed rc=2**（✗ 不自行抬
//	  py 的 1 档、✗ 不静默绿）—— 见未覆盖面①。
//	· **D 类（死旗标/死变量/死正则）**：py **已自行标注**两处死面：① 删掉的死常量 `REPO_VAR_HINTS`（本件不复刻 ·
//	  照 py 现状）；② `RE_PY_RM` 里的 `find\([^)]*\)\.delete\(\)` 与 `\.unlink\(\s*\)` **零捕获组 ⇒ 恒不产出**（活死正则）
//	  ⇒ 逐字照搬其「不产出」语义（✗ 不假装它们命中）。**未见**「声明了从不被读的旗标」（10 个旗标全被读）。
//
//	★ 处理有效证明见「必带读数」。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	cnsfExitOK      = 0
	cnsfExitFail    = 1
	cnsfExitBlocked = 2
)

// cnsfScanExts —— py `SCAN_EXTS`（**顺序即 `_rules()` 的印出面** ⇒ 逐字保序）。
var cnsfScanExts = []string{".sh", ".bash", ".zsh", ".py", ".go", ".mk", ".pl", ".rb"}

// cnsfExcludeDirs —— py `EXCLUDE_DIRS`（集合语义；印出面走 `sorted()` ⇒ 本件排序后再印）。
var cnsfExcludeDirs = []string{
	".git", "vendor", "node_modules", "target", "dist", "bin", "data", "zerg-wt", ".zerg",
	"__pycache__", ".venv", "venv", ".cargo", "htmlobs", "htmlcov", ".history", ".build",
}

// cnsfFarmExcludeDirs —— py `scan_farm` 里的三元硬排除（**集合语义**，与 scan_static 的 EXCLUDE_DIRS 不同）。
var cnsfFarmExcludeDirs = map[string]bool{".git": true, "node_modules": true, "__pycache__": true}

// cnsfWsInner —— py `\s`（**str 模式 · Unicode 空白**）的展开内容。★ ✗ 不照抄 Go 的 ASCII `\s`
// （Go `\s` 不含 `\v`=U+000B，且不含 U+0085/U+00A0/U+2000-200A/U+2028/9/U+202F/U+205F/U+3000）。
const cnsfWsInner = `\t\n\v\f\r ` + `\x{001c}-\x{001f}` + `\x{0085}` + `\x{00a0}` + `\x{1680}` +
	`\x{2000}-\x{200a}` + `\x{2028}` + `\x{2029}` + `\x{202f}` + `\x{205f}` + `\x{3000}`

var (
	cnsfWsClass = "[" + cnsfWsInner + "]"

	// ① RE_LN 的**去后顾子模式**（`^` 锚定 ⇒ 位置扫描器逐 rune 起点尝试）。
	cnsfReLN = regexp.MustCompile("^ln" + cnsfWsClass + "+((?:-[A-Za-z]+" + cnsfWsClass + "+)*)(.*)$")
	// ② RE_RM 同形。
	cnsfReRM = regexp.MustCompile("^rm" + cnsfWsClass + "+((?:-{1,2}[A-Za-z][A-Za-z-]*" + cnsfWsClass + "+)*)(.*)$")
	// ③ RE_FIND_DELETE 同形。
	cnsfReFind = regexp.MustCompile("^find" + cnsfWsClass + "+((?:-[A-Za-z]+" + cnsfWsClass + "+)*)(.*)$")
	// ④ RE_ASSIGN（`re.match` 锚首 · 无环视 · `\s` 已展开）。
	cnsfReAssign = regexp.MustCompile("^" + cnsfWsClass + "*(?:export" + cnsfWsClass + "+|local" + cnsfWsClass + "+|declare" +
		cnsfWsClass + "+-[a-zA-Z]*" + cnsfWsClass + "+)?([A-Za-z_][A-Za-z0-9_]*)=(\"[^\"]*\"|'[^']*'|[^" +
		cnsfWsInner + ";]+)")
	// ⑥⑦ shutil.rmtree( / os.remove|unlink|rmdir(
	cnsfReShutilRmtree = regexp.MustCompile("shutil\\.rmtree\\(" + cnsfWsClass + "*([^,)]+)")
	cnsfReOsRemove     = regexp.MustCompile("os\\.(?:remove|unlink|rmdir)\\(" + cnsfWsClass + "*([^,)]+)")
	// ⑧ \brmtree( 的**去词界子模式**（`^` 锚定 ⇒ 由 cnsfFindAllBoundary 逐 rune 起点 + 词界判）。
	cnsfReBndRmtree = regexp.MustCompile("^rmtree\\(" + cnsfWsClass + "*([^,)]+)")
)

// ── py 空白 / 词字符判定（对齐 py `str` 语义）─────────────────────────────────────

// cnsfIsPySpace —— py `str` 的空白集（`str.isspace()` / `\s` 同集）。
func cnsfIsPySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f,
		0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// cnsfIsWord —— py `\w`（`str.isalnum() or '_'`）的判定。
func cnsfIsWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsNumber(r)
}

// cnsfPyStrip —— py `str.strip()`（两端去 py 空白）。
func cnsfPyStrip(s string) string {
	i := 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if !cnsfIsPySpace(r) {
			break
		}
		i += sz
	}
	j := len(s)
	for j > i {
		r, sz := utf8.DecodeLastRuneInString(s[:j])
		if !cnsfIsPySpace(r) {
			break
		}
		j -= sz
	}
	return s[i:j]
}

// cnsfSplitLines —— py `str.splitlines()`（含 `\v`/`\f`/`\x1c-\x1e`/U+0085/U+2028/U+2029；`\r\n` 合一；无尾空行）。
func cnsfSplitLines(s string) []string {
	var out []string
	i, start, n := 0, 0, len(s)
	for i < n {
		r, sz := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			out = append(out, s[start:i])
			i += sz
			start = i
		case '\r':
			out = append(out, s[start:i])
			i += sz
			if i < n && s[i] == '\n' {
				i++
			}
			start = i
		default:
			i += sz
		}
	}
	if start < n {
		out = append(out, s[start:])
	}
	return out
}

// cnsfStrip —— py `_strip()`（去引号与尾随 `,;`）。★ 不许 `rstrip(";,)]}")`（会啃掉 `${VAR}` 的右花括号）。
func cnsfStrip(tok string) string {
	t := strings.TrimRight(cnsfPyStrip(tok), ";,")
	rs := []rune(t)
	if len(rs) >= 2 && rs[0] == rs[len(rs)-1] && (rs[0] == '"' || rs[0] == '\'' || rs[0] == '`') {
		t = string(rs[1 : len(rs)-1])
	}
	return t
}

// cnsfSplitArgs —— py `_split_args`（`RE_TOK = "[^"]*"|'[^']*'|\S+` 的逐 rune 手写同构 · `\S` = Unicode 非空白）。
func cnsfSplitArgs(tail string) []string {
	var out []string
	i, n := 0, len(tail)
	for i < n {
		r, sz := utf8.DecodeRuneInString(tail[i:])
		if cnsfIsPySpace(r) {
			i += sz
			continue
		}
		if r == '"' || r == '\'' {
			q := tail[i : i+1]
			if j := strings.Index(tail[i+1:], q); j >= 0 {
				out = append(out, cnsfStrip(tail[i:i+1+j+1]))
				i = i + 1 + j + 1
				continue
			}
		}
		start := i
		for i < n {
			r2, sz2 := utf8.DecodeRuneInString(tail[i:])
			if cnsfIsPySpace(r2) {
				break
			}
			i += sz2
		}
		out = append(out, cnsfStrip(tail[start:i]))
	}
	return out
}

// cnsfStripLineComment —— py `re.split(r"\s#\s", line, maxsplit=1)[0]`（最左「空白+#+空白」三元组的前缀）。
func cnsfStripLineComment(line string) string {
	rs := []rune(line)
	for i := 0; i+2 < len(rs)+1 && i+2 <= len(rs)-0; i++ {
		if i+2 >= len(rs)+1 {
			break
		}
		if i+2 <= len(rs) && cnsfIsPySpace(rs[i]) && rs[i+1] == '#' && cnsfIsPySpace(rs[i+2]) {
			return string(rs[:i])
		}
	}
	return line
}

// ── py 路径词法面（自写 · ✗ 不用 filepath.Join/Clean/Abs/Rel/EvalSymlinks）────────────────

// cnsfPyJoin —— CPython `posixpath.join` 逐段规则直译（含「空段 `join('a',”)=='a/'`」与「后段绝对丢前段」）。
func cnsfPyJoin(a string, p ...string) string {
	path := a
	for _, b := range p {
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

// cnsfPyNormpath —— CPython `posixpath.normpath`（★ 保留**恰好两个**前导斜杠）。
func cnsfPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	n := 0
	if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
		n = 2
	} else if strings.HasPrefix(path, "/") {
		n = 1
	}
	var newComps []string
	for _, c := range strings.Split(path, "/") {
		if c == "" || c == "." {
			continue
		}
		if c != ".." || (n == 0 && len(newComps) == 0) || (len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, c)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	res := strings.Join(newComps, "/")
	if n > 0 {
		res = strings.Repeat("/", n) + res
	}
	if res == "" {
		return "."
	}
	return res
}

// cnsfPyDirname —— CPython `posixpath.dirname`。
func cnsfPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// cnsfPyAbspath —— CPython `posixpath.abspath`（相对则 `normpath(join(cwd, p))`，绝对则 `normpath(p)`）。
func cnsfPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		p = cnsfPyJoin(wd, p)
	}
	return cnsfPyNormpath(p)
}

// cnsfExpanduser —— py `os.path.expanduser`（只认前导 `~` / `~/`）。
func cnsfExpanduser(p string) string {
	if strings.HasPrefix(p, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			if strings.HasPrefix(p, "~/") {
				return cnsfPyJoin(home, p[2:])
			}
		}
	}
	return p
}

// cnsfPyRelpath —— CPython `posixpath.relpath`（公共前缀 + `..` 前缀数）。
func cnsfPyRelpath(path, start string) string {
	if path == "" {
		return "."
	}
	sa := cnsfPyAbspath(start)
	pa := cnsfPyAbspath(path)
	comp := func(p string) []string {
		var out []string
		for _, c := range strings.Split(p, "/") {
			if c != "" {
				out = append(out, c)
			}
		}
		return out
	}
	sl, pl := comp(sa), comp(pa)
	i := 0
	for i < len(sl) && i < len(pl) && sl[i] == pl[i] {
		i++
	}
	var rl []string
	for j := i; j < len(sl); j++ {
		rl = append(rl, "..")
	}
	rl = append(rl, pl[i:]...)
	if len(rl) == 0 {
		return "."
	}
	return cnsfPyJoin(rl[0], rl[1:]...)
}

// cnsfPyRealpath —— `os.path.realpath(strict=False)` 的组件解析器（✗ 不用 `filepath.EvalSymlinks`：
// 后者对**悬空链接**直接报错，而 py 尽力解析后返回词法路径）。
func cnsfPyRealpath(p string) string {
	path := ""
	rest := cnsfPyAbspath(p)
	guard := 0
	for rest != "" && guard < 4096 {
		guard++
		if strings.HasPrefix(rest, "/") {
			rest = strings.TrimLeft(rest, "/")
			path = "/"
			if rest == "" {
				break
			}
		}
		var name string
		if i := strings.IndexByte(rest, '/'); i < 0 {
			name, rest = rest, ""
		} else {
			name, rest = rest[:i], rest[i+1:]
		}
		if name == "" || name == "." {
			continue
		}
		if name == ".." {
			if d := cnsfPyDirname(path); d != "" {
				path = d
			} else {
				path = "/"
			}
			continue
		}
		newpath := cnsfPyJoin(path, name)
		fi, err := os.Lstat(newpath)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			path = newpath
			continue
		}
		link, err := os.Readlink(newpath)
		if err != nil {
			path = newpath
			continue
		}
		if !strings.HasPrefix(link, "/") {
			link = cnsfPyJoin(cnsfPyDirname(newpath), link)
		}
		rest = link + "/" + rest
		path = "/"
	}
	if path == "" {
		path = "/"
	}
	return cnsfPyNormpath(path)
}

// cnsfIsLink —— py `os.path.islink`（Lstat 语义 · 不跟随）。
func cnsfIsLink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

// cnsfStatIsDir —— py `os.DirEntry.is_dir()` / `os.path.isdir`（**跟随**链）。
func cnsfStatIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// cnsfStatIsFile —— py `os.path.isfile`（**跟随**链）。
func cnsfStatIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// ── 变量表（**切片保插入序** · 零 map 迭代进输出）───────────────────────────────

type cnsfAssigns struct {
	keys  []string
	vals  map[string]string
	lines map[string]string
}

func cnsfNewAssigns() *cnsfAssigns {
	return &cnsfAssigns{vals: map[string]string{}, lines: map[string]string{}}
}

func (a *cnsfAssigns) set(name, val, line string) {
	if _, ok := a.vals[name]; !ok {
		a.keys = append(a.keys, name)
	}
	a.vals[name] = val
	a.lines[name] = line
}

func (a *cnsfAssigns) get(name string) (string, bool) {
	v, ok := a.vals[name]
	return v, ok
}

// ── 变量链解析 ────────────────────────────────────────────────────────────────

// cnsfVarSplit —— py `re.match(r"^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?(.*)$", tok)` 的手写同构。
func cnsfVarSplit(tok string) (string, string, bool) {
	if !strings.HasPrefix(tok, "$") {
		return "", "", false
	}
	r := tok[1:]
	if strings.HasPrefix(r, "{") {
		r = r[1:]
	}
	if r == "" {
		return "", "", false
	}
	c := r[0]
	if !(c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
		return "", "", false
	}
	i := 1
	for i < len(r) && (r[i] == '_' || (r[i] >= 'A' && r[i] <= 'Z') || (r[i] >= 'a' && r[i] <= 'z') ||
		(r[i] >= '0' && r[i] <= '9')) {
		i++
	}
	name := r[:i]
	rest := r[i:]
	if strings.HasPrefix(rest, "}") {
		rest = rest[1:]
	}
	return name, rest, true
}

// cnsfResolveVar —— py `_resolve_var`（认「已赋过绝对路径的变量」⇒ 值；认不出返回 ok=false）。
func cnsfResolveVar(tok string, a *cnsfAssigns, depth int) (string, bool) {
	if depth > 3 {
		return "", false
	}
	name, rest, ok := cnsfVarSplit(tok)
	if !ok {
		return "", false
	}
	v, has := a.get(name)
	if !has {
		return "", false
	}
	base := v
	if b, ok2 := cnsfResolveVar(v, a, depth+1); ok2 && b != "" {
		base = b
	}
	if strings.Contains(base, "$(") || strings.Contains(base, "`") {
		return "", false
	}
	return strings.TrimRight(base, "/") + rest, true
}

// cnsfTargetSource —— py `_target_source`：追 `ln -s` 目标来源 ⇒ (kind, 值/线索, 出处)。
func cnsfTargetSource(tok string, a *cnsfAssigns, depth int) (string, string, string) {
	if depth > 5 {
		return "other", tok, tok
	}
	name, rest, ok := cnsfVarSplit(tok)
	if !ok {
		if strings.HasPrefix(tok, "/") {
			return "resolved", tok, tok
		}
		return "other", tok, tok
	}
	v, has := a.get(name)
	if !has {
		return "unassigned", name, name
	}
	if strings.Contains(v, "$(") || strings.Contains(v, "`") {
		return "cmdsub", v, name
	}
	if strings.HasPrefix(v, "$") {
		k, b, o := cnsfTargetSource(v, a, depth+1)
		if k == "resolved" {
			return "resolved", strings.TrimRight(b, "/") + rest, tok
		}
		return k, b, o
	}
	if strings.HasPrefix(v, "/") {
		return "resolved", strings.TrimRight(v, "/") + rest, tok
	}
	return "relative", v + rest, tok
}

// cnsfUnder —— py `_under`（只有绝对路径才谈「落在真仓前缀内」；相对路径返回 ok=false）。
func cnsfUnder(path string, prefixes []string) (string, bool) {
	if path == "" || strings.Contains(path, "$") || strings.Contains(path, "`") {
		return "", false
	}
	if !strings.HasPrefix(path, "/") {
		return "", false
	}
	p := cnsfPyNormpath(path)
	for _, pre := range prefixes {
		if pre == "" {
			continue
		}
		q := cnsfPyNormpath(pre)
		if p == q || strings.HasPrefix(p, q+"/") {
			return pre, true
		}
	}
	return "", false
}

// ── 位置扫描器（负后顾 / 词界的手写替身）────────────────────────────────────────

// cnsfLbOK —— `(?<![\w.-])`：前一个 rune 不得是词字符、`.`、`-`（无前驱 = 成立）。
func cnsfLbOK(prev rune, has bool) bool {
	if !has {
		return true
	}
	if prev == '.' || prev == '-' {
		return false
	}
	return !cnsfIsWord(prev)
}

// cnsfSearchAnchored —— 逐 **rune** 起点尝试 `^` 锚定子模式，返回**最左**命中的子匹配组。
func cnsfSearchAnchored(s string, re *regexp.Regexp, lb func(rune, bool) bool) []string {
	for i := range s {
		prev, has := rune(0), false
		if i > 0 {
			prev, _ = utf8.DecodeLastRuneInString(s[:i])
			has = true
		}
		if lb != nil && !lb(prev, has) {
			continue
		}
		if m := re.FindStringSubmatch(s[i:]); m != nil {
			return m
		}
	}
	return nil
}

// cnsfFindAllBoundary —— `\b`（Unicode 词界）版 finditer：非重叠、最左优先。
func cnsfFindAllBoundary(s string, re *regexp.Regexp) [][]string {
	var out [][]string
	off := 0
	for off <= len(s) {
		found := -1
		var got []string
		for i := off; i <= len(s); {
			prev, has := rune(0), false
			if i > 0 {
				prev, _ = utf8.DecodeLastRuneInString(s[:i])
				has = true
			}
			if !has || !cnsfIsWord(prev) {
				if m := re.FindStringSubmatch(s[i:]); m != nil {
					got = m
					found = i + len(m[0])
					break
				}
			}
			if i >= len(s) {
				break
			}
			_, sz := utf8.DecodeRuneInString(s[i:])
			i += sz
		}
		if found < 0 {
			break
		}
		out = append(out, got)
		off = found
	}
	return out
}

// ── 命中 / 报告记录 ────────────────────────────────────────────────────────────

type cnsfHit struct {
	file   string
	line   int
	code   string
	action string
	path   string
	why    string
}

type cnsfNote struct {
	file   string
	line   int
	code   string
	link   string
	target string
	why    string
}

type cnsfLink struct {
	link  string
	lkind string
	lno   int
	lline string
	ltgt  string
}

type cnsfTarget struct {
	kind string
	tok  string
}

// cnsfScanFile —— py `_scan_file` ⇒ (hits, notes)。
func cnsfScanFile(path string, prefixes []string, rel string) ([]cnsfHit, []cnsfNote) {
	raw, err := os.ReadFile(path)
	if err != nil {
		// B 类真口子：py `except OSError: return [], []` 的静默面 —— 本件保留此「空返回」语义，但调用方
		//   `cnsfScanStatic` 已在调用前 `os.Open` 预读检测 + 登记 `unreadable` ⇒ 汇总后 rc=2（不再静默）。
		return nil, nil
	}
	src := strings.ToValidUTF8(string(raw), "\uFFFD") // errors="replace"（替粒度差见未覆盖面）
	lines := cnsfSplitLines(src)

	// 模块/函数 docstring 一律跳过（三引号块不是代码）。
	codeLines := make([]string, 0, len(lines))
	docDelim := ""
	dq3, sq3 := "\"\"\"", "'''"
	for _, line := range lines {
		s := cnsfPyStrip(line)
		if docDelim == "" {
			if strings.HasPrefix(s, dq3) || strings.HasPrefix(s, sq3) {
				d := s[:3]
				if strings.Count(s, d) >= 2 {
					codeLines = append(codeLines, "")
					continue
				}
				docDelim = d
				codeLines = append(codeLines, "")
				continue
			}
			codeLines = append(codeLines, line)
		} else {
			if strings.Contains(s, docDelim) {
				docDelim = ""
			}
			codeLines = append(codeLines, "")
		}
	}
	lines = codeLines

	assigns := cnsfNewAssigns()
	var links []cnsfLink
	var hits []cnsfHit
	var notes []cnsfNote
	// 第一遍：变量表（含「值引用另一个变量」的两轮传播 · **按插入序**）。
	for _, line := range lines {
		if strings.HasPrefix(cnsfPyStrip(line), "#") {
			continue
		}
		if m := cnsfReAssign.FindStringSubmatch(line); m != nil {
			assigns.set(m[1], cnsfStrip(m[2]), cnsfPyStrip(line))
		}
	}
	for pass := 0; pass < 2; pass++ {
		snapshot := make([][2]string, 0, len(assigns.keys))
		for _, k := range assigns.keys {
			snapshot = append(snapshot, [2]string{k, assigns.vals[k]})
		}
		for _, kv := range snapshot {
			if strings.HasPrefix(kv[1], "$") {
				if r, ok := cnsfResolveVar(kv[1], assigns, 0); ok && r != "" {
					assigns.vals[kv[0]] = r
				}
			}
		}
	}

	for i, line := range lines {
		lineno := i + 1
		if strings.HasPrefix(cnsfPyStrip(line), "#") {
			continue
		}
		body := cnsfStripLineComment(line)
		for _, stmt := range strings.Split(body, ";") {
			stmt = cnsfPyStrip(stmt)
			if stmt == "" {
				continue
			}

			if m := cnsfSearchAnchored(stmt, cnsfReLN, cnsfLbOK); m != nil {
				args := cnsfSplitArgs(m[2])
				kept := make([]string, 0, len(args))
				for _, a := range args {
					if !strings.HasPrefix(a, "-") {
						kept = append(kept, a)
					}
				}
				if len(kept) >= 2 {
					target, link := kept[0], kept[1]
					rt, ok := cnsfResolveVar(target, assigns, 0)
					if !ok || rt == "" {
						rt = target
					}
					var pre string
					var hasPre bool
					if strings.HasPrefix(rt, "/") {
						pre, hasPre = cnsfUnder(rt, prefixes)
					}
					switch {
					case hasPre:
						links = append(links, cnsfLink{link, "真仓", lineno, stmt, rt})
					case strings.HasPrefix(rt, "/"):
						links = append(links, cnsfLink{link, "绝对路径（非真仓）", lineno, stmt, rt})
					default:
						skind, _, _ := cnsfTargetSource(target, assigns, 0)
						switch skind {
						case "cmdsub":
							links = append(links, cnsfLink{link, "待人工核（前缀来自命令替换）", lineno, stmt, target})
						case "unassigned":
							links = append(links, cnsfLink{link, "待人工核（变量本件未赋值）", lineno, stmt, target})
						default:
							links = append(links, cnsfLink{link, "相对/未解析", lineno, stmt, rt})
						}
					}
					_ = pre
				}
			}

			var targets []cnsfTarget
			if m := cnsfSearchAnchored(stmt, cnsfReRM, cnsfLbOK); m != nil {
				for _, a := range cnsfSplitArgs(m[2]) {
					if strings.HasPrefix(a, "-") {
						continue
					}
					targets = append(targets, cnsfTarget{"rm", a})
				}
			}
			for _, mm := range cnsfReShutilRmtree.FindAllStringSubmatch(stmt, -1) {
				if mm[1] != "" {
					targets = append(targets, cnsfTarget{"shutil.rmtree", cnsfStrip(mm[1])})
				}
			}
			for _, mm := range cnsfReOsRemove.FindAllStringSubmatch(stmt, -1) {
				if mm[1] != "" {
					targets = append(targets, cnsfTarget{"os.remove/unlink/rmdir", cnsfStrip(mm[1])})
				}
			}
			// py RE_PY_RM[2] `find\([^)]*\)\.delete\(\)` 与 [3] `\.unlink\(\s*\)` **零捕获组 ⇒ 恒不产出**
			// （活死正则 · 逐字照搬「不产出」语义）。
			for _, mm := range cnsfFindAllBoundary(stmt, cnsfReBndRmtree) {
				if mm[1] != "" {
					targets = append(targets, cnsfTarget{"rmtree", cnsfStrip(mm[1])})
				}
			}
			if m := cnsfSearchAnchored(stmt, cnsfReFind, cnsfLbOK); m != nil && strings.Contains(m[0], "-delete") {
				for _, a := range cnsfSplitArgs(m[2]) {
					if !strings.HasPrefix(a, "-") {
						targets = append(targets, cnsfTarget{"find -delete", a})
					}
				}
			}

			for _, tg := range targets {
				if tg.tok == "" {
					continue
				}
				for _, ln := range links {
					if tg.tok == ln.link || strings.HasPrefix(tg.tok, ln.link+"/") {
						hits = append(hits, cnsfHit{rel, lineno, "A2", tg.kind, tg.tok,
							fmt.Sprintf("删除动作穿过符号链接（%s:%d `ln -s` ⇒ %s）", rel, ln.lno, ln.ltgt)})
					}
				}
				rt, ok := cnsfResolveVar(tg.tok, assigns, 0)
				var pre string
				var hasPre bool
				if ok && rt != "" {
					pre, hasPre = cnsfUnder(rt, prefixes)
				}
				if !hasPre && strings.HasPrefix(tg.tok, "/") {
					pre, hasPre = cnsfUnder(tg.tok, prefixes)
				}
				if hasPre {
					hits = append(hits, cnsfHit{rel, lineno, "A3", tg.kind, tg.tok,
						fmt.Sprintf("删除目标落在真仓前缀内：%s", pre)})
				}
			}
		}
	}

	for _, ln := range links {
		switch {
		case ln.lkind == "真仓":
			notes = append(notes, cnsfNote{rel, ln.lno, "A1", ln.link, ln.ltgt,
				"农场链接指回真仓（本件无删除动作指向它 ⇒ 只报告）"})
		case strings.HasPrefix(ln.lkind, "待人工核"):
			k, v, nm := cnsfTargetSource(ln.ltgt, assigns, 0)
			var why string
			if k == "cmdsub" {
				srcLine := v
				if s, ok := assigns.lines[nm]; ok {
					srcLine = s
				}
				why = fmt.Sprintf("链接目标前缀来自**命令替换**（运行时才知道值 ⇒ 门判不了）；断在源头那一行：`%s`", srcLine)
			} else {
				why = fmt.Sprintf("链接目标引用了**本件未赋值**的变量 `%s`（值来自环境/调用方 ⇒ 门判不了）", nm)
			}
			notes = append(notes, cnsfNote{rel, ln.lno, "A1?", ln.link, ln.ltgt,
				fmt.Sprintf("%s ⇒ **待人工核**：展开后若落在真仓前缀内就是 A1 事故形态（本件无删除动作指向它 ⇒ 只报告）", why)})
		}
	}
	return hits, notes
}

// cnsfWalk —— py `os.walk(topdown=True, followlinks=False)` 的同构（`f.ReadDir(-1)` 保原序）。
func cnsfWalk(root string, filter func(dirpath string, dirs []string) []string,
	visit func(dirpath string, dirs, files []string)) {
	f, err := os.Open(root)
	if err != nil {
		return
	}
	entries, err := f.ReadDir(-1)
	f.Close()
	if err != nil && len(entries) == 0 {
		return
	}
	var dirs, files []string
	for _, e := range entries {
		name := e.Name()
		p := cnsfPyJoin(root, name)
		if cnsfStatIsDir(p) { // ★ py `entry.is_dir()` = 跟随链 ⇒ 用 os.Stat（✗ 非 e.IsDir() 的 Lstat）
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}
	if filter != nil {
		dirs = filter(root, dirs)
	}
	visit(root, dirs, files)
	for _, d := range dirs {
		np := cnsfPyJoin(root, d)
		if cnsfIsLink(np) { // py `if followlinks or not islink(join(top,dirname))` ⇒ 链接目录不下钻
			continue
		}
		cnsfWalk(np, filter, visit)
	}
}

// cnsfScanStatic —— py `scan_static`（静态档 A）。★ 目录序 = scandir 原序；同目录文件序 = `sorted(filenames)`。
func cnsfScanStatic(root string, prefixes []string, strictLinks bool, maxExamples int) ([]cnsfHit, []cnsfNote, int, []string) {
	var hits []cnsfHit
	var notes []cnsfNote
	var unreadable []string
	nfiles := 0
	root = cnsfPyAbspath(root)
	excl := map[string]bool{}
	for _, d := range cnsfExcludeDirs {
		excl[d] = true
	}
	visit := func(dirpath string, dirs, files []string) {
		sorted := append([]string(nil), files...)
		sort.Strings(sorted)
		for _, name := range sorted {
			ok := false
			for _, ext := range cnsfScanExts {
				if strings.HasSuffix(name, ext) {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
			p := cnsfPyJoin(dirpath, name)
			if cnsfIsLink(p) || !cnsfStatIsFile(p) {
				continue
			}
			if fi, err := os.Stat(p); err == nil && fi.Size() > 4*1024*1024 {
				continue
			}
			nfiles++
			rel := cnsfPyRelpath(p, root)
			// ★ B 类真口子修（原 py `except OSError: return [], []` 静默 · ✗ 不再静默）：候选脚本件
			//   读不了（权限 / 竞态 / IsADirectory）⇒ 登记（相对路径 + 原因），汇总后按 rc=2 不给结论。
			//   体例照同包已治件 check_token_argv.go / check_curl_timeout.go（登记 + 判红 + rc=2）。
			if fh, oerr := os.Open(p); oerr != nil {
				unreadable = append(unreadable, rel+"（"+oerr.Error()+"）")
				continue
			} else if cerr := fh.Close(); cerr != nil {
				unreadable = append(unreadable, rel+"（"+cerr.Error()+"）")
				continue
			}
			h, n := cnsfScanFile(p, prefixes, rel)
			hits = append(hits, h...)
			notes = append(notes, n...)
		}
	}
	filter := func(dirpath string, dirs []string) []string {
		out := make([]string, 0, len(dirs))
		for _, d := range dirs {
			if !excl[d] {
				out = append(out, d)
			}
		}
		return out
	}
	cnsfWalk(root, filter, visit)
	if strictLinks {
		for _, n := range notes {
			hits = append(hits, cnsfHit{n.file, n.line, "A1(--strict-links)", n.link, n.target, n.why})
		}
		notes = nil
	}
	return hits, notes, nfiles, unreadable
}

// cnsfScanFarm —— py `scan_farm`（B 档 · 现读农场目录）。
func cnsfScanFarm(farm string, prefixes []string, maxDepth, maxExamples int) ([]cnsfHit, int) {
	var hits []cnsfHit
	nlinks := 0
	farm = cnsfPyAbspath(farm)
	baseDepth := strings.Count(strings.TrimRight(farm, "/"), "/")
	filter := func(dirpath string, dirs []string) []string {
		if strings.Count(strings.TrimRight(dirpath, "/"), "/")-baseDepth >= maxDepth {
			dirs = nil
		}
		out := make([]string, 0, len(dirs))
		for _, d := range dirs {
			if !cnsfFarmExcludeDirs[d] {
				out = append(out, d)
			}
		}
		return out
	}
	visit := func(dirpath string, dirs, files []string) {
		names := make([]string, 0, len(dirs)+len(files))
		names = append(names, dirs...)
		names = append(names, files...)
		for _, name := range names {
			p := cnsfPyJoin(dirpath, name)
			if !cnsfIsLink(p) {
				continue
			}
			nlinks++
			tgt := cnsfPyRealpath(p)
			if pre, ok := cnsfUnder(tgt, prefixes); ok {
				hits = append(hits, cnsfHit{cnsfPyRelpath(p, farm), 0, "B", "symlink", tgt,
					fmt.Sprintf("农场里的链接指回真仓（%s）⇒ 对它的删除会透传到真件", pre)})
			}
		}
	}
	cnsfWalk(farm, filter, visit)
	return hits, nlinks
}

// cnsfDefaultRepoRoot —— 声明 (a)：Go 无 `__file__` ⇒ 从 cwd 上溯找「含 `scripts/gates` 的层」。
func cnsfDefaultRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := wd
	for i := 0; i < 8; i++ {
		if fi, err := os.Stat(cnsfPyJoin(d, "scripts", "gates")); err == nil && fi.IsDir() {
			return d
		}
		p := cnsfPyDirname(d)
		if p == "" || p == d {
			break
		}
		d = p
	}
	return wd
}

// ── JSON 编码（py `json.dumps(..., ensure_ascii=False)` 默认分隔符 `", "` / `": "`）──────────

func cnsfJSONStr(s string) string {
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

func cnsfJSONStringList(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, cnsfJSONStr(x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cnsfJSONOut —— py `--json` 档的整包（键序 = py dict 插入序）。
func cnsfJSONOut(root string, prefixes []string, farm string, farmGiven bool, nfiles, nlinks int,
	strict bool, hits []cnsfHit, notes []cnsfNote) string {
	var b strings.Builder
	b.WriteString("{" + cnsfJSONStr("schema") + ": " + cnsfJSONStr("zerg/gate-no-symlink-farm/v1") + ", ")
	b.WriteString("\"kin" + "d\": " + cnsfJSONStr("NoSymlinkFarmGate") + ", ")

	b.WriteString(cnsfJSONStr("items") + ": [")
	for i, h := range hits {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{" + cnsfJSONStr("file") + ": " + cnsfJSONStr(h.file) + ", " +
			cnsfJSONStr("line") + ": " + fmt.Sprintf("%d", h.line) + ", " +
			cnsfJSONStr("code") + ": " + cnsfJSONStr(h.code) + ", " +
			cnsfJSONStr("action") + ": " + cnsfJSONStr(h.action) + ", " +
			cnsfJSONStr("path") + ": " + cnsfJSONStr(h.path) + ", " +
			cnsfJSONStr("why") + ": " + cnsfJSONStr(h.why) + "}")
	}
	b.WriteString("], ")

	b.WriteString(cnsfJSONStr("reports") + ": [")
	for i, n := range notes {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{" + cnsfJSONStr("file") + ": " + cnsfJSONStr(n.file) + ", " +
			cnsfJSONStr("line") + ": " + fmt.Sprintf("%d", n.line) + ", " +
			cnsfJSONStr("code") + ": " + cnsfJSONStr(n.code) + ", " +
			cnsfJSONStr("link") + ": " + cnsfJSONStr(n.link) + ", " +
			cnsfJSONStr("target") + ": " + cnsfJSONStr(n.target) + ", " +
			cnsfJSONStr("why") + ": " + cnsfJSONStr(n.why) + "}")
	}
	b.WriteString("], ")

	nPending := 0
	for _, n := range notes {
		if n.code == "A1?" {
			nPending++
		}
	}
	rc := cnsfExitOK
	if len(hits) > 0 {
		rc = cnsfExitFail
	}
	farmJSON := "null"
	if farmGiven {
		farmJSON = cnsfJSONStr(farm)
	}
	b.WriteString(cnsfJSONStr("meta") + ": {" +
		cnsfJSONStr("root") + ": " + cnsfJSONStr(root) + ", " +
		cnsfJSONStr("prefixes") + ": " + cnsfJSONStringList(prefixes) + ", " +
		cnsfJSONStr("candidates") + ": " + fmt.Sprintf("%d", nfiles) + ", " +
		cnsfJSONStr("hits") + ": " + fmt.Sprintf("%d", len(hits)) + ", " +
		cnsfJSONStr("reports") + ": " + fmt.Sprintf("%d", len(notes)) + ", " +
		cnsfJSONStr("farm") + ": " + farmJSON + ", " +
		cnsfJSONStr("farm_links_scanned") + ": " + fmt.Sprintf("%d", nlinks) + ", " +
		cnsfJSONStr("reports_pending") + ": " + fmt.Sprintf("%d", nPending) + ", " +
		cnsfJSONStr("strict_links") + ": " + cnsfJSONBool(strict) + ", " +
		cnsfJSONStr("rc") + ": " + fmt.Sprintf("%d", rc) + "}}")
	return b.String()
}

func cnsfJSONBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// ── 参数（argparse 同名旗标 · 缺省 = py 推导逐字同构）────────────────────────────

type cnsfArgs struct {
	root        string
	realRepo    string
	farm        string
	strictLinks bool
	maxDepth    int
	maxExamples int
	selfTest    bool
	noSelfTest  bool
	listRules   bool
	json        bool
}

type cnsfPayload struct {
	Hits  []cnsfHit
	Notes []cnsfNote
}

// cnsfParseArgs —— 极简 argparse（同名旗标逐字 · `--k=v` 与 `--k v` 两形）。
// 未知参数 ⇒ (args, unknown, false)。★ 值缺失 ⇒ 记 unknown 形以退 2（口径见未覆盖面②）。
func cnsfParseArgs(argv []string) (*cnsfArgs, []string, bool) {
	a := &cnsfArgs{root: cnsfDefaultRepoRoot(), maxDepth: 3, maxExamples: 20}
	var unknown []string
	need := func(i *int, inline string, hasInline bool) (string, bool) {
		if hasInline {
			return inline, true
		}
		if *i+1 < len(argv) {
			*i++
			return argv[*i], true
		}
		return "", false
	}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		name, inline, hasInline := arg, "", false
		if strings.HasPrefix(arg, "--") {
			if eq := strings.IndexByte(arg, '='); eq >= 0 {
				name, inline, hasInline = arg[:eq], arg[eq+1:], true
			}
		}
		switch name {
		case "--root":
			if v, ok := need(&i, inline, hasInline); ok {
				a.root = v
			} else {
				unknown = append(unknown, arg)
			}
		case "--real-repo":
			if v, ok := need(&i, inline, hasInline); ok {
				a.realRepo = v
			} else {
				unknown = append(unknown, arg)
			}
		case "--farm":
			if v, ok := need(&i, inline, hasInline); ok {
				a.farm = v
			} else {
				unknown = append(unknown, arg)
			}
		case "--max-depth":
			if v, ok := need(&i, inline, hasInline); ok {
				a.maxDepth = cnsfAtoi(v)
			} else {
				unknown = append(unknown, arg)
			}
		case "--max-examples":
			if v, ok := need(&i, inline, hasInline); ok {
				a.maxExamples = cnsfAtoi(v)
			} else {
				unknown = append(unknown, arg)
			}
		case "--strict-links":
			a.strictLinks = true
		case "--self-test":
			a.selfTest = true
		case "--no-self-test":
			a.noSelfTest = true
		case "--list-rules":
			a.listRules = true
		case "--json":
			a.json = true
		default:
			unknown = append(unknown, arg)
		}
	}
	return a, unknown, true
}

// cnsfAtoi —— `int()` 的近似（ASCII 十进制 + 前导符号；Unicode 数字面未覆盖，见未覆盖面）。
func cnsfAtoi(s string) int {
	s = cnsfPyStrip(s)
	neg := false
	if strings.HasPrefix(s, "-") {
		neg, s = true, s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	n := 0
	for i := 0; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		n = n*10 + int(s[i]-'0')
	}
	if neg {
		return -n
	}
	return n
}

// ── 判据说明（`--list-rules`）──────────────────────────────────────────────────

func cnsfRules(stdout io.Writer) {
	fmt.Fprintln(stdout, "判据（本门只认形态）")
	fmt.Fprintln(stdout, "  A1 链接回真仓（`ln -s <真仓>/… <链接>`）—— 扫仓内件时只报告；`--farm` / `--strict-links` 档算红")
	fmt.Fprintln(stdout, "  A1? 链接目标前缀**追不到源头**（命令替换 `$(…)` / 本件未赋值的变量）—— 点名待人工核；"+
		"只报告，`--strict-links` 档算红")
	fmt.Fprintln(stdout, "  A2 删除动作穿过符号链接（`rm $农场/link/…`）—— 红（删除会透传到链接目标 = 事故本体）")
	fmt.Fprintln(stdout, "  A3 删除目标含真仓前缀（字面路径，或引用的变量被赋过真仓前缀）—— 红")
	fmt.Fprintln(stdout, "  B  农场现场（`--farm <目录>`）：任何链接目标落在真仓前缀内 —— 红")
	sorted := append([]string(nil), cnsfExcludeDirs...)
	sort.Strings(sorted)
	fmt.Fprintf(stdout, "扫描面：%s；整行注释跳过；硬排除 %s\n",
		strings.Join(cnsfScanExts, " "), strings.Join(sorted, " "))
	fmt.Fprintln(stdout, "退码：0 全绿 · 1 有红 · 2 不给结论（用法错/缺件/空转/自检不过）")
}

// ── 主入口 ────────────────────────────────────────────────────────────────────

func cnsfMain(stdout, stderr io.Writer, argv []string) (int, *cnsfPayload) {
	a, unknown, _ := cnsfParseArgs(argv)
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "✗ 用法错：未知参数 %s（--list-rules 看口径）\n", strings.Join(unknown, " "))
		return cnsfExitBlocked, nil
	}
	if a.listRules {
		cnsfRules(stdout)
		return cnsfExitOK, nil
	}
	if !a.noSelfTest {
		if cnsfSelfTest(stdout, stderr) != cnsfExitOK {
			fmt.Fprintln(stderr, "✗ 自检不过 ⇒ 拒绝扫真目标（项目口径：没有自证过「会红」的门不许给结论）")
			return cnsfExitBlocked, nil
		}
	}

	root := cnsfPyAbspath(a.root)
	if !cnsfStatIsDir(root) {
		fmt.Fprintf(stderr, "✗ 判不了：--root 不是目录 —— %s（rc=2）\n", root)
		return cnsfExitBlocked, nil
	}

	prefixes := []string{}
	if a.realRepo != "" {
		rr := cnsfPyAbspath(cnsfExpanduser(a.realRepo))
		prefixes = append(prefixes, rr, cnsfPyRealpath(rr))
	}
	defRoot := cnsfDefaultRepoRoot()
	prefixes = append(prefixes, defRoot, cnsfPyRealpath(defRoot))
	seen := map[string]bool{}
	uniq := make([]string, 0, len(prefixes))
	for _, q := range prefixes {
		if q == "" || seen[q] {
			continue
		}
		seen[q] = true
		uniq = append(uniq, q)
	}
	prefixes = uniq

	hits, notes, nfiles, unreadable := cnsfScanStatic(root, prefixes, a.strictLinks, a.maxExamples)
	var fhits []cnsfHit
	nlinks := 0
	if a.farm != "" {
		if !cnsfStatIsDir(a.farm) {
			fmt.Fprintf(stderr, "✗ 判不了：--farm 不是目录 —— %s（rc=2）\n", a.farm)
			return cnsfExitBlocked, nil
		}
		fhits, nlinks = cnsfScanFarm(a.farm, prefixes, a.maxDepth, a.maxExamples)
	}
	hits = append(hits, fhits...)

	if nfiles == 0 {
		fmt.Fprintf(stderr, "✗ 空转：%s 下 0 个候选脚本件 ⇒ 判不了（rc=2；假覆盖不算绿）\n", root)
		return cnsfExitBlocked, nil
	}

	if len(unreadable) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 候选脚本件读不到 %d 件（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论 · 读不到不许当健康）：\n", len(unreadable))
		for _, u := range unreadable {
			fmt.Fprintf(stderr, "   ✗ %s\n", u)
		}
		return cnsfExitBlocked, nil
	}

	if a.json {
		fmt.Fprintln(stdout, cnsfJSONOut(root, prefixes, a.farm, a.farm != "", nfiles, nlinks, a.strictLinks, hits, notes))
		if len(hits) > 0 {
			return cnsfExitFail, &cnsfPayload{hits, notes}
		}
		return cnsfExitOK, &cnsfPayload{hits, notes}
	}

	fmt.Fprintln(stdout, "== 无符号链接农场门 ==")
	fmt.Fprintf(stdout, "· 扫描面：%s（候选 %d 件）\n", root, nfiles)
	fmt.Fprintf(stdout, "· 真仓前缀：%s\n", strings.Join(prefixes, " · "))
	if a.farm != "" {
		fmt.Fprintf(stdout, "· 农场现场（--farm）：%s（扫到链接 %d 条）\n", a.farm, nlinks)
	} else {
		fmt.Fprintln(stdout, "· 农场现场（--farm）：**未给 ⇒ B 档跳过**（不影响退码；审农场目录时请显式给）")
	}
	if len(notes) > 0 {
		fmt.Fprintf(stdout, "· **只报告**（存量 A1 + 待人工核 A1? · 不入退码 %d 条）：\n", len(notes))
		for i, n := range notes {
			if i >= a.maxExamples {
				break
			}
			if n.code == "A1?" {
				fmt.Fprintf(stdout, "    ~ %s:%d [%s] %s —— %s\n", n.file, n.line, n.code, n.link, n.why)
			} else {
				fmt.Fprintf(stdout, "    ~ %s:%d [%s] %s → %s\n", n.file, n.line, n.code, n.link, n.target)
			}
		}
		if len(notes) > a.maxExamples {
			fmt.Fprintf(stdout, "    … 另有 %d 条\n", len(notes)-a.maxExamples)
		}
	}
	if len(hits) == 0 {
		fmt.Fprintf(stdout, "⇒ rc=0：在 %d 件里没有「符号链接回真仓 + 删除穿过它 / 删真仓前缀」的形态\n", nfiles)
		if len(notes) > 0 {
			np := 0
			for _, n := range notes {
				if n.code == "A1?" {
					np++
				}
			}
			fmt.Fprintf(stdout, "   ⚠ 但有 **%d 条只报告**（其中待人工核 A1? %d 条）⇒ rc=0 **不等于**「一件都没看见」，"+
				"A1? 请人工核\n", len(notes), np)
		}
		return cnsfExitOK, &cnsfPayload{hits, notes}
	}
	fmt.Fprintf(stdout, "· **红**（%d 条）：\n", len(hits))
	for i, h := range hits {
		if i >= a.maxExamples {
			break
		}
		fmt.Fprintf(stdout, "    ✗ %s:%d [%s] %s %s —— %s\n", h.file, h.line, h.code, h.action, h.path, h.why)
	}
	if len(hits) > a.maxExamples {
		fmt.Fprintf(stdout, "    … 另有 %d 条\n", len(hits)-a.maxExamples)
	}
	fmt.Fprintln(stdout, "⇒ rc=1：**禁止**在临时目录里建指向真仓的符号链接后再删它下面的东西 ——")
	fmt.Fprintln(stdout, "   改用只读隔离工具：zerg isolate-ro new \"<真仓>\" --dst <目录>（硬拷贝，删的是副本）")
	return cnsfExitFail, &cnsfPayload{hits, notes}
}

// checkNoSymlinkFarmGateMain —— py `main()` 同形：`--self-test` 成员特判 ⇒ 自检；否则 `_main`。
func checkNoSymlinkFarmGateMain(stdout, stderr io.Writer, argv []string) int {
	for _, a := range argv {
		if a == "--self-test" {
			return cnsfSelfTest(stdout, stderr)
		}
	}
	rc, _ := cnsfMain(stdout, stderr, argv)
	return rc
}

// ── 自检（成对控 · 合成夹具 · 一件真东西都不碰）───────────────────────────────

func cnsfSelfTest(stdout, stderr io.Writer) int {
	tmp, err := os.MkdirTemp("", "zerg-nosymfarm-")
	if err != nil {
		return cnsfExitBlocked
	}
	defer os.RemoveAll(tmp)

	var bad []string
	chk := func(label string, ok bool, extra string) {
		mark := "✓"
		if !ok {
			mark = "✗"
			bad = append(bad, label)
		}
		suffix := ""
		if !ok && extra != "" {
			suffix = "（" + extra + "）"
		}
		fmt.Fprintf(stdout, "   %s %s%s\n", mark, label, suffix)
	}
	run := func(root, real, farm string, strict bool) (int, *cnsfPayload) {
		argv := []string{"--root", root, "--real-repo", real, "--no-self-test", "--json"}
		if farm != "" {
			argv = append(argv, "--farm", farm)
		}
		if strict {
			argv = append(argv, "--strict-links")
		}
		return cnsfMain(stdout, stderr, argv)
	}
	writeFile := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			fmt.Fprintf(stderr, "✗ 写夹具失败：%v\n", err)
		}
	}
	codesOfHits := func(p *cnsfPayload) []string {
		set := map[string]bool{}
		if p != nil {
			for _, h := range p.Hits {
				set[h.code] = true
			}
		}
		out := make([]string, 0, len(set))
		for k := range set {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	codesOfNotes := func(p *cnsfPayload) []string {
		set := map[string]bool{}
		if p != nil {
			for _, n := range p.Notes {
				set[n.code] = true
			}
		}
		out := make([]string, 0, len(set))
		for k := range set {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	containsPrefix := func(xs []string, pre string) bool {
		for _, x := range xs {
			if strings.HasPrefix(x, pre) {
				return true
			}
		}
		return false
	}

	fix := cnsfPyJoin(tmp, "fix")
	real := cnsfPyJoin(fix, "real")
	if err := os.MkdirAll(cnsfPyJoin(real, "core", "cmd", "zerg", "testdata"), 0o755); err != nil {
		fmt.Fprintf(stderr, "✗ 建夹具失败：%v\n", err)
	}
	writeFile(cnsfPyJoin(real, "core", "cmd", "zerg", "testdata", "cli-matrix.json"), "{}\n")

	// ① 负控 A2/A1
	d1 := cnsfPyJoin(fix, "bad-link")
	os.MkdirAll(cnsfPyJoin(d1, "farm"), 0o755)
	writeFile(cnsfPyJoin(d1, "farm.sh"),
		"REPO=\""+real+"\"\nFARM=\""+cnsfPyJoin(d1, "farm")+"\"\n"+
			"ln -s \"${REPO}/core/cmd/zerg/testdata\" \"$FARM/case-a/testdata\"\n"+
			"rm -f \"$FARM/case-a/testdata/cli-matrix.json\"\n")
	rc, payload := run(d1, real, "", false)
	codes := codesOfHits(payload)
	chk("① 负控-A2：链接回真仓 + 删链接下面 ⇒ rc=1 且判据 A2",
		rc == cnsfExitFail && containsPrefix(codes, "A2"), fmt.Sprintf("rc=%d codes=%s", rc, cnsfPyListRepr(codes)))
	hasA1 := false
	if payload != nil {
		for _, h := range payload.Hits {
			if strings.HasPrefix(h.code, "A1") || h.code == "A2" {
				hasA1 = true
			}
		}
	}
	chk("① 负控-A1：同一件里报出「链接指回真仓」", hasA1, "")

	// ② 正控
	d2 := cnsfPyJoin(fix, "good")
	os.MkdirAll(cnsfPyJoin(d2, "copy"), 0o755)
	writeFile(cnsfPyJoin(d2, "copy", "x.json"), "{}\n")
	writeFile(cnsfPyJoin(d2, "good.sh"), "d=\""+d2+"\"\nrm -rf \"$d/copy/x.json\"\n")
	rc, _ = run(d2, real, "", false)
	chk("② 正控：只删自己拷出来的树 ⇒ rc=0（与 ① 成对）", rc == cnsfExitOK, fmt.Sprintf("rc=%d", rc))

	// ③ 负控 A3
	d3 := cnsfPyJoin(fix, "bad-var")
	os.MkdirAll(d3, 0o755)
	writeFile(cnsfPyJoin(d3, "wipe.sh"), "REPO=\""+real+"\"\nrm -rf \"$REPO/scripts/tmp\"\n")
	rc, payload = run(d3, real, "", false)
	codes = codesOfHits(payload)
	chk("③ 负控-A3：删真仓变量下面 ⇒ rc=1 且判据 A3",
		rc == cnsfExitFail && containsPrefix(codes, "A3"), fmt.Sprintf("rc=%d codes=%s", rc, cnsfPyListRepr(codes)))

	// ④ 负控 B / ⑤ 正控 B（★ 夹具内部**主动造一枚真符号链接** —— 判据需要）
	farm := cnsfPyJoin(fix, "farm-live")
	os.MkdirAll(farm, 0o755)
	link := cnsfPyJoin(farm, "testdata-link")
	_ = os.Symlink(cnsfPyJoin(real, "core", "cmd", "zerg", "testdata"), link)
	rc, _ = run(d2, real, farm, false)
	chk("④ 负控-B：农场现场链接回真仓 ⇒ rc=1", rc == cnsfExitFail, fmt.Sprintf("rc=%d", rc))
	os.Remove(link)
	rc, _ = run(d2, real, farm, false)
	chk("⑤ 正控-B：链接拿掉 ⇒ rc=0（与 ④ 成对）", rc == cnsfExitOK, fmt.Sprintf("rc=%d", rc))

	// ⑥ 用法错 / 空转
	rc, _ = cnsfMain(stdout, stderr, []string{"--root", cnsfPyJoin(tmp, "no-such"), "--no-self-test"})
	chk("⑥ 用法错：--root 不存在 ⇒ rc=2", rc == cnsfExitBlocked, fmt.Sprintf("rc=%d", rc))
	empty := cnsfPyJoin(fix, "empty")
	os.MkdirAll(empty, 0o755)
	rc, _ = cnsfMain(stdout, stderr, []string{"--root", empty, "--no-self-test"})
	chk("⑥ 用法错：候选 0 件（空转）⇒ rc=2", rc == cnsfExitBlocked, fmt.Sprintf("rc=%d", rc))

	// ⑦ 负控-A1?（命令替换前缀）
	d4 := cnsfPyJoin(fix, "bad-cmdsub")
	os.MkdirAll(d4, 0o755)
	writeFile(cnsfPyJoin(d4, "farm.sh"),
		"REPO_ROOT=\"$(cd \"$(dirname \"${BASH_SOURCE[0]}\")/../..\" && pwd)\"\n"+
			"KERNEL=\"$REPO_ROOT/scripts/build/zerg-upgrade.sh\"\n"+
			"FARM=\""+cnsfPyJoin(d4, "farm")+"\"\n"+
			"ln -sf \"$KERNEL\" \"$FARM/scripts/build/zerg-upgrade.sh\"\n"+
			"rm -f \"$FARM/copy/x.json\"\n")
	rc, payload = run(d4, real, "", false)
	ncodes := codesOfNotes(payload)
	chk("⑦ 负控-A1?：目标前缀来自命令替换 ⇒ **点名待人工核**（不是静默 0）",
		rc == cnsfExitOK && containsPrefix(ncodes, "A1?"),
		fmt.Sprintf("rc=%d reports_codes=%s", rc, cnsfPyListRepr(ncodes)))
	rc, payload = run(d4, real, "", true)
	codes = codesOfHits(payload)
	chk("⑦ 负控-A1?（--strict-links）：同一夹具 ⇒ rc=1",
		rc == cnsfExitFail && containsPrefix(codes, "A1(--strict-links)"),
		fmt.Sprintf("rc=%d hits_codes=%s", rc, cnsfPyListRepr(codes)))

	// ⑧ 正控：无害形态
	d5 := cnsfPyJoin(fix, "good-forms")
	os.MkdirAll(d5, 0o755)
	writeFile(cnsfPyJoin(d5, "ok.sh"),
		"D=\""+cnsfPyJoin(d5, "farm-good")+"\"\n"+
			"ln -s \"$D/target\" \"$D/link\"\nln -s ../rel/target rel-link\n"+
			"rm -rf \"$D/copy\"\n")
	rc, payload = run(d5, real, "", false)
	chk("⑧ 正控：字面绝对路径（真仓外）+ 真·相对链接 ⇒ rc=0 且 0 报告",
		rc == cnsfExitOK && (payload == nil || len(payload.Notes) == 0),
		fmt.Sprintf("rc=%d reports=%s", rc, cnsfPyNotesRepr(payload)))

	// ⑨ 边界锁（A3 侧不点名）
	d6 := cnsfPyJoin(fix, "tmp-clean")
	os.MkdirAll(d6, 0o755)
	writeFile(cnsfPyJoin(d6, "clean.sh"), "CLEANUP_DIR=\"$(mktemp -d)\"\nrm -rf \"${CLEANUP_DIR}\"\n")
	rc, payload = run(d6, real, "", false)
	chk("⑨ 边界锁：A3 侧「前缀来自命令替换」**不点名**（临时目录清理）⇒ rc=0 且 0 报告",
		rc == cnsfExitOK && (payload == nil || len(payload.Notes) == 0),
		fmt.Sprintf("rc=%d reports=%s", rc, cnsfPyNotesRepr(payload)))

	// ⑩ 含空格赋值（缺口 GAP-20260926-115）
	d7 := cnsfPyJoin(fix, "space-assign")
	os.MkdirAll(d7, 0o755)
	writeFile(cnsfPyJoin(d7, "space.sh"), "R=\""+real+"/sub dir\"\nln -s \"$R/t\" \"$R/l\"\n")
	rc, payload = run(d7, real, "", false)
	ncodes = codesOfNotes(payload)
	chk("⑩ 含空格赋值（缺口 `GAP-20260926-115`）：引号里带空格的路径 ⇒ 值完整 ⇒ 指向真仓必须点名 A1",
		rc == cnsfExitOK && containsPrefix(ncodes, "A1"),
		fmt.Sprintf("rc=%d codes=%s", rc, cnsfPyListRepr(ncodes)))

	verdict := "全过"
	if len(bad) != 0 {
		verdict = fmt.Sprintf("**不过**（%d 条）", len(bad))
	}
	fmt.Fprintf(stdout, "── 自检结论：%s ──\n", verdict)
	if len(bad) == 0 {
		return cnsfExitOK
	}
	return cnsfExitBlocked
}

// cnsfPyListRepr —— py `list[str]` 的 `%s` repr（仅自检失败时印出）。
func cnsfPyListRepr(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, "'"+x+"'")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cnsfPyNotesRepr —— py `reports` 列表的近似 repr（仅自检失败时印出）。
func cnsfPyNotesRepr(p *cnsfPayload) string {
	if p == nil {
		return "None"
	}
	parts := make([]string, 0, len(p.Notes))
	for _, n := range p.Notes {
		parts = append(parts, fmt.Sprintf("{'file': '%s', 'line': %d, 'code': '%s', 'link': '%s', 'target': '%s', 'why': %s}",
			n.file, n.line, n.code, n.link, n.target, cnsfJSONStr(n.why)))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
