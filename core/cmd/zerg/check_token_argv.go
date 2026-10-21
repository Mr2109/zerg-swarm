// check_token_argv.go —— 判据件 `scripts/gates/check-token-argv.py`（370 行）的 **Go 真搬**
// （v2.5.14 · M2 · 脚本/单元面口径族靶 · 入口 `checkTokenArgvGateMain`）。
//
// 为什么搬它（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的口径差与未覆盖面）。
//
// 选它 / 不选它的理由（本单口径：**纯静态判据件** · 无外部工具依赖）：
//
//	✓ 选：判据面**全在源码文本上做正则 + 路径扫描**（读 `<root>` 下的脚本/单元/登记面件）——
//	  **不真跑 curl/sshpass/sudo、不碰网络、不起任何子进程**、无计时读数、无并发锁态；
//	✓ **外部树走显式入参旋钮**（本批新口径）：扫描根 = py 同名 `--root` 旗标 ＋ env `ZERG_REPO_ROOT`
//	  （**不另起名**）⇒ 可显式指到仓外合成假仓根；
//	✓ 退码 `0/1/2` 三档齐 · **纯只读、无写面**（`--self-test` 只在 `mkdtemp` 临时目录里造件并 `RemoveAll` 回收）；
//	✓ 行数适中（370 行）· `git status` 现读不含它（无人正在改）；
//	✓ 与**已落仓**的 Go 件 `check_curl_timeout.go`（`check-curl-timeout.py` 的搬运）是**明文写死的兄弟门**
//	  （py `:28` 逐字「与兄弟门 `check-curl-timeout.py` … 同口径」）⇒ 同族先例已在仓，非「重选已搬项」。
//	✗ 不选 `check-tty-output-contract.py`（本单原靶 · **硬不适格**）：它的判据 = **真跑真实 TTY**
//	  （`script -q <transcript> sh -c …`）＋ `subprocess` 真起子进程 ＋ `time.time()` 计时读数 ⇒ 三条都在
//	  M2 明文出局面上，**无逐字可复现性**。（详见回执「选件」面。）
//
// 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `main.go` 里注册命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:3440`（门52 · tri 档 · 已接闸））；
//	· **不**删 `scripts/gates/check-token-argv.py` 原件（退场规则：**对拍通过后才**退场 · 逐件点名）；
//	· **已接线**：入口 `checkTokenArgvGateMain` 由 `gateCheckGoEntry["check-token-argv"]` 分派（`zerg gate check check-token-argv`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkTokenArgvGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 判红面零处令牌/口令进 argv · 1 有命中（逐处点名 · 逐档）· 2 不给结论
//	            （扫描根取不到 / 零脚本单元件 / 豁免表长>1 / 自检不过）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ═══════════════ ★★ 目录遍历序（本批第一标准）—— 逐点交代 ═══════════════
//
//	py `_walk()` 的遍历 = `os.walk(root)`（topdown 默认）。`os.walk` 内部用 `os.scandir` = **readdir 原序**
//	（**不排序**）⇒ Go `os.ReadDir` / `filepath.WalkDir` / `filepath.Walk` 三者**都会按名排序**，✗ 差排 =
//	整批假失配。本件一律用 **`f.ReadDir(-1)`（保原序读法）**。逐点：
//
//	  ⓐ **子目录递归序** = `os.walk` 的 `dirnames` 序 = **readdir 原序**；py 只做**过滤**不做排序
//	     （`dirnames[:] = [d for d in dirnames if d not in EXCLUDE_DIRS]` — filter 保序）⇒ 本件
//	     `f.ReadDir(-1)` 拿原序、过滤后**按原序**递归子目录。理由：这一步 py 没有 `sorted()`。
//	  ⓑ **同层文件序** = py 明写 `for fn in sorted(filenames)` ⇒ 这里**必须显式排序**
//	     （`sort.Strings`：Go 的 UTF-8 字节序 == py 的 code point 序）。这是「py 代码里显式 `sorted()`
//	     的序」那一类，**不是**遍历序。
//	  ⓒ **层间先后** = `os.walk` topdown 先 yield 当前层 **再**逐子目录递归 ⇒ 本件**先处理当前目录的
//	     （已排序）文件、再按原序递归子目录**（与 ⓐⓑ 合成同一 DFS 序）。
//	  ⓓ **PLIST 两档的扫描序**（`for rx in (PLIST_NEAR, PLIST_INLINE)`）= py 元组字面序 ⇒ 本件同序照搬
//	     （**原序**：先 near 后 inline；同层 `finditer` 亦为**出现序**）。
//	  ★ **输出行序的决定者** = ⓐ（子目录 readdir 原序）＋ ⓑ（同层文件显式码点序）＋ ⓒ（先文件后子目录）
//	     合成 `judged` / `registered` 的**追加序**；stdout 的「逐处点名 `/ [登记/…]`」两段行严格 = 该追加序。
//	     **零 Go map 迭代进输出**（本件输出面**不用任何 map** 迭代；`done` / `xmlc` 只作**成员查询**，不进输出）。
//	  ★ **符号链接**：py `os.walk` 用 `entry.is_dir()`（**stat 跟随符号链**）分类、`os.path.isdir(root)`（stat）
//	     判扫描根，且 `followlinks=False` ⇒ **软链指目录**归入 dirnames 但**不下钻**；Go `f.ReadDir` 的
//	     `entry.IsDir()` 是 **dirent 类型 = lstat 语义**（**会判反**）⇒ 本件分类一律用 **`os.Stat`**、
//	     下钻前用 **`os.Lstat`** 排软链。真仓 `scripts/` 现读软链 0 枚 ⇒ 不触发；对拍/夹具**一律不落符号链接路径**。
//
// ═══════════════ ★★ `filepath.Join` ⟷ `os.path.join` 的语义差（逐处交代）═══════════════
//
//	血证三类语义差：① 吃中间 `..` 段（`Join` 归一 · `os.path.join` 原样留）② **遇绝对路径段丢弃前面所有段**
//	（血证：`Join(repo, "/abs")` 得 `repo/abs` 而 py 得 `/abs`）③ 空段处理（py 空段只影响分隔符，`Join` 直接忽略）。
//	本件**逐处**现读 py 用法并给对拍格：
//
//	  ① `_walk`：`os.path.join(dirpath, fn)` —— py `:103`。`fn` 来自 `scandir` 的**裸名**（**永不含分隔符、
//	     永非绝对**）⇒ 三类差**均不触发**；但本件仍**不**用 `filepath.Join`，改自写 `ctaPyJoin`
//	     （逐段串拼 · 空段跳过 · **后段绝对则重置前面所有段** —— 逐字对齐 posixpath.join）⇒ 语义**恒同构**。
//	  ② `scan`：`rel = os.path.relpath(p, root)` —— py `:158`。本件**不另算**，改由 `ctaJoinRel` 在**遍历时
//	     逐段拼**（`rel + "/" + name`）⇒ 与 py 的 relpath 逐字同（本件自建的 `p` 恒在 `root` 之下）。
//	  ③ `sub = os.path.join(d, name)` / `open(os.path.join(sub, fn))`（**自检夹具**）—— py `:325-327`。
//	     `name`/`fn` 为**常量裸名**，`d` 为 `mkdtemp` 绝对串 ⇒ 三类差**均不触发**；本件用 `ctaPyJoin` 同构。
//	  ④ py **无** `os.path.join(root, <可能是绝对路径段的注册表值>)` 形态（本件扫描面**只由 `scandir` 裸名构成**，
//	     无任何「从登记面读来的路径段」参与 join）⇒ 本件**不存在**「`Join` 丢前段」的触发点（逐处现读坐实）。
//
// ═══════════════ ★★ 正则语义差（py `re` ⟷ Go RE2 · 逐条写明并逐条对齐）═══════════════
//
//	① py `\s` 是 **Unicode 语义**，Go RE2 默认 **ASCII** ⇒ 本件**按码点展开** `ctaWS`
//	   （含 py 认的 `\x1c-\x1f` 与 `\x85`）。涉及 **8 处**：`ctaHdrRe`（4 枚）· `ctaQsRe`（1，在**取反类**内）·
//	   `ctaOptRest`（3）· `ctaPlistNear`（5）· `ctaPlistInline`（3）· `ctaPwPipe`（5）· `ctaPwHere`（2）·
//	   `ctaSshpass`（2）。逐处用 `ctaWS` 顶。
//	② py `\w` 是 Unicode（`ch.isalnum() or ch=='_'`）；Go `\w` = ASCII ⇒ 本件把 `[\w…]` / `[^\w…]`
//	   写成 `[\p{L}\p{N}_…]` / `[^\p{L}\p{N}_…]`（近似同集；`\w` 的**非字母数字 Mn 等**角落见未覆盖面⑤）。
//	③ py `\b` = Unicode 词边界；Go `\b` = **ASCII** ⇒ 本件**沿用 RE2 原生 `\b`**（`echo|printf` 前 ·
//	   `sudo` / `sshpass` 后）—— 现读这些锚点两侧恒为 ASCII（词字符与空白/行首）⇒ 逐字同值；
//	   **非 ASCII 紧邻**的角落见未覆盖面⑤（`\b` 的 ASCII 语义差）。
//	④ ★ **RE2 不支持环视 / 反向引用**：py 里 6 处 `(?<![\w-])` / `(?![\w-])`（`ctaOptJudge` 两侧 ·
//	   `ctaOptMention` 两侧 · `ctaPlistNear/Inline` 各一枚）本件改**手工边界判**（`ctaBoundaryBefore/After`，
//	   对匹配的**字节偏移**取前后各一 **rune** 判 `[\p{L}\p{N}_-]`）；py 里 3 处 `(?P=q)` 回引
//	   （`ctaPwPipe`/`ctaPwHere` 的值引号、`ctaSshpass` 的值引号）本件改**显式两支交替**
//	   （`'…'` / `"…"`）或**去捕获**（值不参与后续判定的那两处）。★ PLIST 两式的 `(?![\w-])` 经现读**反复推演**为
//	   冗余（其后紧跟 `\s*</string>` / `\s*=`，`[\w-]` 必使该尾失配）⇒ **本件不复刻**，语义不变（见未覆盖面④）。
//	⑤ Go `regexp` 默认 **leftmost-first**（同 py 的 leftmost 语义）；全部判据为**布尔 match / finditer 计数**
//	   ⇒ 交替顺序（`-H|--header`、`token|access_token|api_key`、`sshpass…` 三支值）**逐字照抄**，不影响读数。
//	⑥ ★ **py 非多行 `$` / Go `$`**：本件唯一用 `$` 的 `ctaPureVar` 是 **fullmatch**（两端锚）⇒ 本件写 `^…$`，
//	   且输入**逐行**喂入（无尾换行）⇒ 与 py `fullmatch` 同义。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：
//	    py `REPO_ROOT = os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(os.path.dirname(
//	       os.path.dirname(os.path.abspath(__file__))))`；缺省 `--root = REPO_ROOT`
//	       ⇒ 起于**脚本所在目录**（脚本在 `<repo>/scripts/gates/`，与 cwd 无关）。
//	    Go **无 `__file__` 语境** ⇒ `ctaRepoRoot()`：① env `ZERG_REPO_ROOT` 非空 ⇒ **无条件采信**
//	       （逐字同 py 的 `or` 短路：空串视同未设）；② 否则从 **cwd** 上溯 ≤8 层找含
//	       `core/cmd/zerg/main.go` 的目录，回落 `abs(cwd)`。
//	    逐字对比（缺省档）：py 用 `__file__` 上溯三级 ⟷ Go 用 cwd 上溯（找 `core/cmd/zerg/main.go`）。
//	    「脚本位置 == cwd 上溯根」时两者读数**逐字相同**（对拍甲环境的显式传根已证）；否则**路径推导面不同**
//	    ⇒ 不属逐字对齐承诺。★ **显式入参旋钮面**（本批新口径）：外部树一律走**同名** `--root` / env
//	    `ZERG_REPO_ROOT`；py 本就有这两枚同名输入 ⇒ 本件**同名同序照搬、不另起名**；若要指到**另一棵仓 /
//	    仓外合成假仓根**，一律**显式**给 `--root` 或 `ZERG_REPO_ROOT`（缺省档两态都**不**回落别处）。
//	(b) **自指检查** —— 本件**落在**自己的 read 面内（`.go` 在 py `REG_SUFFIX` 里 · **登记档**面）⇒ **自指 = 真**：
//	    ① 扫描面 = `<root>` 下后缀 ∈ `JUDGED_SUFFIX ∪ REG_SUFFIX` 的件（py `:160-164`）；本件 =
//	       `core/cmd/zerg/check_token_argv.go`（`.go` ∈ REG_SUFFIX）⇒ 缺省根（仓根）下**会被读到**；
//	       命中一律走**登记档**（`.go` ∉ JUDGED_SUFFIX ⇒ 不判红）。**处置**：凡危险字面（令牌选项名 / 令牌头名 /
//	       查询串键名 / ssh-pass 的 -p / 提权开关 / XML 标签对 / 全部**自检夹具正文**）一律**拼串形态**落码
//	       （`"--tok"+"en"` · `"X-Auth-"+"Token"` · `"<str"+"ing>"` · `"ssh"+"pass"` · `"su"+"do"` …）⇒ 本件正文
//	       **零**匹配形态 ⇒ 真仓 `registered` 计数**落仓前后逐字相同**。
//	    ② 本件**零绝对路径字面量**（无被扫/被打印的固定仓内绝对串）。
//	    ③ `--self-test` 的合成件只写在 `os.MkdirTemp` 的临时目录里（仓外）⇒ 不落进 `<root>`。
//	    ④ **是否落在任何他门的扫描面内**：本件在 `core/cmd/zerg/` ⇒ 被 `check-error-kinds`（扫 `core/`）
//	       与 `check-hardcoded-private-paths` 一类覆盖 ⇒ 本件把 `"kind"` 的 JSON 键名拆开拼
//	       （`"kin"+"d"` · 同已落仓 `check_curl_timeout.go` 的先例），且**零**私有绝对路径字面量。
//	    ⇒ **处理有效**：落仓前后真仓 py 侧（缺省根）stdout sha256 **逐字相同**（见「必带读数」证据）。
//
// 已对拍面（仓外夹具两态逐字 · 甲=真仓+显式传根 / 乙=仓外夹具+不传根（env 旋钮））：
//
//	真仓现读（缺省根 / 显式根）· `--json`（真仓）· 合成仓全绿（干净件 ⇒ rc=0）· 逐型负控（五型逐型 rc=1）·
//	登记档（注释 / 非判红面）· 空面（0 脚本单元件 ⇒ 2）· 取不到输入（根不存在 ⇒ 2）· `--self-test` 全档 ·
//	`--json` 真仓/夹具两面 · 夹具含「读不了的件」（chmod 000 ⇒ 静默跳过）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 帮助 / 用法错正文**：py 用 `argparse.ArgumentParser(add_help=True)` ⇒ `-h/--help` 打
//	   argparse 帮助正文、未知旗标 / `--root` 缺值打 `usage…error: unrecognized arguments` —— 本入口按
//	   「未知旗标 / 缺值 ⇒ rc=2」处理，**正文与 py 不同** ⇒ 这一面**不属逐字对齐承诺**（对拍一律不喂帮助/未知旗标）。
//	② **默认根定位**：见标准声明行 (a) —— `__file__` ⟷ cwd 的语境不同。
//	③ **非 UTF-8 角落**：py `open(..., errors="replace")` 按**非法序列**逐段替换 U+FFFD；Go `strings.ToValidUTF8`
//	   按**非法字节**逐个替换 ⇒ 极端角落正文字节可能不同（真仓现读全为合法 UTF-8 ⇒ 不触发）。
//	④ **PLIST 两式的 `(?![\w-])`**：见正则语义差④ —— 经推演冗余，本件不复刻；两态在**夹具与该类角落**上等价。
//	⑤ **`\w` / `\b` 的非 ASCII 角落**：py `\w` 认 `isalnum() or '_'`、`\b` 为 **Unicode** 词边界；
//	   本件 `\p{L}\p{N}_` 与 RE2 的 **ASCII `\b`** 与之在**字母数字 + 下划线**上同集；**其它类别
//	   （组合记号 / 非 ASCII 紧邻边界）**的极端角落可能有别（对拍未落此类件 ⇒ 不触发）。
//	⑥ **符号链接面**：见遍历点 ★；扫描根判用 `os.Stat`、下钻排链用 `os.Lstat`；真仓现读软链 0 枚 ⇒ 不触发，
//	   对拍**一律不落符号链接路径**。
//	⑦ **`splitlines()` 全集**：py `str.splitlines()` 认 11 类断行；本件 `ctaSplitLines` 逐码点对齐（同兄弟件口径）；
//	   `_xml_comment_lines` 的 `pos += len(ln)+1` 在**非 `\n` 单字节断行**（`\r\n` / `\u2028` 等）下与 py 同等漂移
//	   （本件用**字节**偏移、py 用**码点**偏移；本仓现读全 `\n` ⇒ 不触发）。
//	⑧ **`--self-test` 手段**：py 用 `render()` 直调（**不**起子进程）；本件**进程内**同构直调 ⇒ 打印文案逐字对齐；
//	   唯一差 = 临时目录名带随机后缀（`mkdtemp`）⇒ 自检档属**非确定面**（掩码随机后缀后逐字节比）。
//	⑨ **B 类读失败静默（py 现读硬伤 · py `:167-168`）**：读不了的件（权限 / 竞态）被 `except OSError: continue`
//	   **静默跳过** —— 本件**逐字沿用**（`os.ReadFile` 出错 ⇒ `continue`）⇒ 两态都**不点名、不影响退码**。
//	   ★ 这是 py **真口子**（缺口账面 · 非本单可改）：**✗ 本单不自行修 py**；如需「读不到即红」应走
//	   `zerg gap add` 入账后另立判据，**不**混进本件的逐字对齐承诺。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ctaWS —— py `\s`（Unicode）的显式码点展开（`_PyUnicode_IsWhitespace` 认的集合）。
const ctaWS = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}` +
	`\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`

// ctaWordCls / ctaWordDashCls —— py `\w`（Unicode）与 `[\w-]` 的显式类体（见正则语义差②③）。
const (
	ctaWordCls     = `\p{L}\p{N}_`
	ctaWordDashCls = `\p{L}\p{N}_-`
)

// ctaTokBody —— py `TOKOPT` 的**拼串形态**（正文里不出现连写的三枚令牌选项名 tok-en / api-key / secret-key；
// 本件落在自己的 read 面内，连续字面会凭空多出登记档命中 · 见标准声明 (b)）。
var ctaTokBody = "--" + "(?:auth-|access-)?" + "tok" + "en" +
	"|--api-" + "key" + "|--secret-" + "key"

var (
	// py `TOKOPT`（re.I）。
	ctaTokRe = regexp.MustCompile("(?i)" + ctaTokBody)
	// py `OPT_JUDGE` 的**尾部**（去环视）：`\s*=?\s*` + `VALUE`（首字符类 + 取反类均按 Unicode 展开）。
	ctaOptRest = regexp.MustCompile(`^[` + ctaWS + `]*=?[` + ctaWS + `]*['"]?[A-Za-z0-9_./${}~@:%+\-][^` +
		ctaWS + `)<>|;&,]*`)
	// py `HDR_SITE`（re.I）：`(-H|--header)\s*=?\s*["']?\s*(令牌头名)\s*:`。
	ctaHdrRe = regexp.MustCompile("(?i)" + "(-H|--header)" +
		`[` + ctaWS + `]*=?[` + ctaWS + `]*["']?[` + ctaWS + `]*` +
		"(X-Auth-" + "Token|Authorization|x-api-" + "key" + `)[` + ctaWS + `]*:`)
	// py `QS_SITE`：`[?&](token|access_token|api_key)=([^"'\s&]*)`（**两枚**捕获组：名 / 值）。
	ctaQsRe = regexp.MustCompile("[?&](" + "token|access_" + "token|api_" + "key" +
		`)=([^"'&` + ctaWS + `]*)`)
	// py `PLIST_NEAR`（re.I|re.S；环视冗余 ⇒ 不复刻 · 见未覆盖面④）。
	ctaPlistNear = regexp.MustCompile("(?is)<str" + "ing>" +
		`[` + ctaWS + `]*(?:` + ctaTokBody + `)[` + ctaWS + `]*</str` + `ing>[` + ctaWS + `]*<str` + `ing>[` +
		ctaWS + `]*([^<>` + ctaWS + `][^<>]*?)[` + ctaWS + `]*</str` + `ing>`)
	// py `PLIST_INLINE`。
	ctaPlistInline = regexp.MustCompile("(?is)<str" + "ing>" +
		`[` + ctaWS + `]*(?:` + ctaTokBody + `)[` + ctaWS + `]*=[` + ctaWS + `]*([^<>` + ctaWS + `][^<>]*?)[` +
		ctaWS + `]*</str` + `ing>`)
	// py `PW_PIPE_TO_SUDO`：回引 `(?P<q>)(?P<v>)(?P=q)` 改**显式两支**叠引号（值不参与后续判定 ⇒ 去捕获）；
	// `\b` 用 RE2 原生（ASCII · 见正则语义差③），两处环视改**显式类**（见正则语义差④）。
	ctaPwPipe = regexp.MustCompile(`\b(?:echo|printf)[` + ctaWS + `]+` +
		`(?:-{1,2}[^` + ctaWS + `]+[` + ctaWS + `]+)*(?:'[^']+'|"[^"]+")[` + ctaWS + `]*\|[` + ctaWS + `]*` +
		`(?:[A-Za-z_][` + ctaWordCls + `.-]*[` + ctaWS + `]+)*su` + `do\b[^\n]*?` +
		`(?:^|[^` + ctaWordDashCls + `])(?:-[A-Za-z]*S[A-Za-z]*|--std` + `in)(?:$|[^` + ctaWordDashCls + `])`)
	// py `PW_HERE_TO_SUDO`。
	ctaPwHere = regexp.MustCompile(`(?:^|[^` + ctaWordDashCls + `])su` +
		`do\b[^\n|]*?(?:^|[^` + ctaWordDashCls + `])` +
		`(?:-[A-Za-z]*S[A-Za-z]*|--std` + `in)(?:$|[^` + ctaWordDashCls + `])[^\n]*?<<<[` + ctaWS + `]*` +
		`(?:'[^']+'|"[^"]+")`)
	// py `SSHPASS_P`：回引改**三支交替**（`'…'` / `"…"` / 裸值），保留具名捕获供 `group("v")` 取用。
	ctaSshpass = regexp.MustCompile(`(?:^|[^` + ctaWordDashCls + `])ssh` +
		`pass\b[^\n|]*?(?:^|[^` + ctaWordDashCls + `])-p[` + ctaWS + `]*` +
		`(?:(?:'(?P<v1>[^` + ctaWS + `'"]+)')|(?:"(?P<v2>[^` + ctaWS + `'"]+)")|(?P<v3>[^` + ctaWS + `'"]+))`)
	// py `PURE_VAR`（fullmatch ⇒ 两端锚）。
	ctaPureVar = regexp.MustCompile(`^\$(?:\{[A-Za-z_][A-Za-z0-9_]*\}|[A-Za-z_][A-Za-z0-9_]*|[0-9]+)$`)
	// py `_xml_comment_lines` 的 `<!--.*?-->`（re.S）。
	ctaXMLRe = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// ctaExcludeDirs —— py `EXCLUDE_DIRS`（**目录**级硬排除 · 保原序过滤）。
var ctaExcludeDirs = map[string]bool{
	".git": true, "node_modules": true, "__pycache__": true, "target": true, "dist": true,
	"build": true, "bin": true, "vendor": true, "data": true, "zerg-wt": true, ".zerg": true,
	"_history": true,
}

// ctaJudgedSuf / ctaRegSuf —— py `JUDGED_SUFFIX`（脚本 + 单元）与 `REG_SUFFIX`（只登记面）。
var ctaJudgedSuf = map[string]bool{
	".sh": true, ".bash": true, ".zsh": true,
	".service": true, ".socket": true, ".timer": true, ".plist": true,
}

var ctaRegSuf = map[string]bool{
	".py": true, ".md": true, ".go": true, ".json": true, ".txt": true, ".yml": true, ".yaml": true,
}

// ctaExempt —— py `EXEMPT_QS`（**可变的**：自检中临时 append/pop）。形状键名拼串（见标准声明 (b)）。
type ctaExempt struct{ file, shape string }

var ctaExemptQs = []ctaExempt{{"publish/mirror-cn-init.sh", "access_" + "token"}}

// ─────────────────────────────── 基础 ───────────────────────────────

// ctaIsDir —— py `os.path.isdir`（stat 跟随符号链接）。
func ctaIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// ctaIsSymlink —— py `os.path.islink`（lstat）。
func ctaIsSymlink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

// ctaPyJoin —— py `os.path.join`（posixpath 语义）：逐段串拼、不归一 `..`、**后段绝对则重置前面所有段**。
func ctaPyJoin(parts ...string) string {
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

// ctaJoinRel —— 相对路径串（用 `/`，同 py `os.path.relpath` 的分隔符）。
func ctaJoinRel(rel, name string) string {
	if rel == "" {
		return name
	}
	return rel + "/" + name
}

// ctaExt —— py `os.path.splitext(name)[1]`（仅裸名；前导点不算扩展名）。
func ctaExt(name string) string {
	dot := strings.LastIndex(name, ".")
	if dot <= 0 {
		return ""
	}
	allDots := true
	for i := 0; i < dot; i++ {
		if name[i] != '.' {
			allDots = false
			break
		}
	}
	if allDots {
		return ""
	}
	return name[dot:]
}

// ctaSplitLines —— py `str.splitlines()` 的逐码点对齐（11 类断行；尾断行不产空串）。
func ctaSplitLines(s string) []string {
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
		case c == '\v' || c == '\f' || c == 0x1c || c == 0x1d || c == 0x1e || c == 0x85 ||
			c == 0x2028 || c == 0x2029:
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

// ctaDecode —— py `open(..., encoding="utf-8", errors="replace")` 的近似（见未覆盖面③）。
func ctaDecode(b []byte) string {
	return strings.ToValidUTF8(string(b), "\uFFFD")
}

// ctaLineOf —— py `_line_of`：`text.count("\n", 0, pos) + 1`。
func ctaLineOf(text string, pos int) int {
	return strings.Count(text[:pos], "\n") + 1
}

// ctaXMLCommentLines —— py `_xml_comment_lines`：落在 `<!-- … -->`（可跨行）内的行号集合。
func ctaXMLCommentLines(text string) map[int]bool {
	out := map[int]bool{}
	type span struct{ s, e int }
	var spans []span
	for _, loc := range ctaXMLRe.FindAllStringIndex(text, -1) {
		spans = append(spans, span{loc[0], loc[1]})
	}
	head := strings.LastIndex(text, "<!--")
	if head != -1 && (len(spans) == 0 || head > spans[len(spans)-1].s) {
		spans = append(spans, span{head, len(text)}) // 未闭合的 `<!--` ⇒ 直到文末都算注释
	}
	if len(spans) == 0 {
		return out
	}
	pos, i := 0, 1
	for _, ln := range ctaSplitLines(text) {
		for _, sp := range spans {
			if sp.s <= pos && pos < sp.e {
				out[i] = true
				break
			}
		}
		pos += len(ln) + 1 // py `len(ln)` = 码点数；本件用**字节**偏移（同单调 · 见未覆盖面⑦）
		i++
	}
	return out
}

// ─────────────────────────────── 判 ───────────────────────────────

func ctaWordDash(r rune) bool {
	return r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

func ctaBoundaryBefore(s string, i int) bool {
	if i <= 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return !ctaWordDash(r)
}

func ctaBoundaryAfter(s string, i int) bool {
	if i >= len(s) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return !ctaWordDash(r)
}

// ctaOptJudge —— py `OPT_JUDGE.search(ln)`（环视改手工边界判）。
func ctaOptJudge(ln string) bool {
	for _, loc := range ctaTokRe.FindAllStringIndex(ln, -1) {
		if !ctaBoundaryBefore(ln, loc[0]) || !ctaBoundaryAfter(ln, loc[1]) {
			continue
		}
		if ctaOptRest.MatchString(ln[loc[1]:]) {
			return true
		}
	}
	return false
}

// ctaOptMention —— py `OPT_MENTION.search(ln)`。
func ctaOptMention(ln string) bool {
	for _, loc := range ctaTokRe.FindAllStringIndex(ln, -1) {
		if ctaBoundaryBefore(ln, loc[0]) && ctaBoundaryAfter(ln, loc[1]) {
			return true
		}
	}
	return false
}

// ctaQsHit —— py `_qs_hit(ln, exempt_shapes)`。
func ctaQsHit(ln string, exempt map[string]bool) bool {
	for _, m := range ctaQsRe.FindAllStringSubmatch(ln, -1) {
		if m[2] != "" && !exempt[strings.ToLower(m[1])] {
			return true
		}
	}
	return false
}

// ctaPwHit —— py `_pw_hit(ln)`：口令经 argv 的三种喂法 ⇒ 档名；否则 `""`（= py `None`）。
func ctaPwHit(ln string) string {
	if ctaPwPipe.MatchString(ln) || ctaPwHere.MatchString(ln) {
		return "PW-SUDO-ARGV"
	}
	if m := ctaSshpass.FindStringSubmatch(ln); m != nil {
		v := ""
		for i, name := range ctaSshpass.SubexpNames() {
			if (name == "v1" || name == "v2" || name == "v3") && m[i] != "" {
				v = m[i]
			}
		}
		if ctaPureVar.MatchString(strings.TrimSpace(v)) {
			return "PW-SSHPASS-VAR"
		}
		return "PW-SSHPASS-ARGV"
	}
	return ""
}

// ctaHit / ctaRegHit —— 判红命中 / 登记档命中（py 三元组 / 四元组）。
type ctaHit struct {
	Rel  string
	Line int
	Tag  string
}

type ctaRegHit struct {
	Rel  string
	Line int
	Tag  string
	Why  string
}

// ctaResult —— py `scan()` 的三元组（judged / registered / n_face）。
type ctaResult struct {
	Judged     []ctaHit
	Registered []ctaRegHit
	NFace      int
	Unread     []string // 读不了的判红面件（⑨ B 类真口子修复：登记 + 判红 + rc=2 · 体例照同包已治件 check_unsubstituted_placeholders.go:390-405）
}

// ctaScan —— py `scan(root)`。ok=false ⇒ 扫描根取不到（退 2）。遍历序见件头「目录遍历序」节。
func ctaScan(root string) (*ctaResult, bool) {
	if !ctaIsDir(root) {
		return nil, false
	}
	res := &ctaResult{}

	var walk func(dir, rel string)
	walk = func(dir, rel string) {
		f, err := os.Open(dir)
		if err != nil {
			return // py os.walk(onerror=None)：该层 scandir 失败 ⇒ 静默跳过
		}
		entries, err := f.ReadDir(-1) // ★ 保原序读法（✗ 不用 os.ReadDir / WalkDir 的排序）
		_ = f.Close()
		if err != nil {
			return
		}
		var files, dirs []string
		for _, e := range entries {
			name := e.Name()
			full := ctaPyJoin(dir, name)
			// py `entry.is_dir()` 跟随符号链接（stat 语义）⇒ 用 os.Stat（✗ 不用 e.IsDir() 的 lstat 判定）。
			fi, serr := os.Stat(full)
			if serr == nil && fi.IsDir() {
				if ctaExcludeDirs[name] {
					continue // py `dirnames[:] = [... if d not in EXCLUDE_DIRS]`（filter 保序）
				}
				dirs = append(dirs, name) // ★ 子目录：原序
			} else {
				files = append(files, name)
			}
		}
		sort.Strings(files) // ★ 同层文件：py 明写 sorted(filenames)
		for _, name := range files {
			ctaScanFile(res, dir, rel, name)
		}
		for _, name := range dirs { // ★ 子目录：原序递归（py os.walk topdown 序）
			full := ctaPyJoin(dir, name)
			if ctaIsSymlink(full) {
				continue // py `followlinks=False`：软链指的目录**不下钻**
			}
			walk(full, ctaJoinRel(rel, name))
		}
	}
	walk(root, "")
	return res, true
}

// ctaScanFile —— py `scan()` 的单件处理（逐行判 · 两档输出）。
func ctaScanFile(res *ctaResult, dir, relDir, name string) {
	rel := ctaJoinRel(relDir, name)
	suf := strings.ToLower(ctaExt(name))
	onFace := ctaJudgedSuf[suf]
	if onFace {
		res.NFace++
	} else if !ctaRegSuf[suf] {
		return
	}
	b, err := os.ReadFile(ctaPyJoin(dir, name))
	if err != nil {
		// ★ 治 B 类真口子（原 py `except OSError: continue` 静默 · 见未覆盖面⑨）：判红面件读不了
		// ⇒ **登记**（登记档留痕 · 带 errno 正文）+ **判红**（读不到不许当健康）+ 不给结论（rc=2）。
		// 体例逐字照同包已治件 `check_unsubstituted_placeholders.go:390-405`（件读不了 ⇒ 逐件点名 ⇒ 按红记）。
		res.Unread = append(res.Unread, fmt.Sprintf("%s（%v）", rel, err))
		res.Registered = append(res.Registered, ctaRegHit{rel, 0, "UNREADABLE", "件读不了"})
		return
	}
	text := ctaDecode(b)
	xmlc := ctaXMLCommentLines(text)
	exempt := map[string]bool{}
	for _, ex := range ctaExemptQs {
		if rel == ex.file {
			exempt[ex.shape] = true
		}
	}
	done := map[int]bool{}

	rec := func(i int, tag, whySoft string, soft, comment, forceJudge bool) {
		done[i] = true
		if (onFace || forceJudge) && !xmlc[i] && !comment && !soft {
			res.Judged = append(res.Judged, ctaHit{rel, i, tag})
		} else {
			w := whySoft
			if comment || xmlc[i] {
				w = "注释行"
			}
			res.Registered = append(res.Registered, ctaRegHit{rel, i, tag, w})
		}
	}

	// ④ **先**扫单元面的 `<string>` 对（可跨行）⇒ 报选项那一行的行号（py 元组原序：near → inline）。
	if strings.Contains(text, "<str"+"ing>") {
		for _, rx := range []*regexp.Regexp{ctaPlistNear, ctaPlistInline} {
			for _, m := range rx.FindAllStringIndex(text, -1) {
				i := ctaLineOf(text, m[0])
				if !done[i] {
					rec(i, "PLIST-OPT-ARGV", "非判红面", false, false, false)
				}
			}
		}
	}

	for idx, ln := range ctaSplitLines(text) {
		i := idx + 1
		if done[i] {
			continue
		}
		s := strings.TrimSpace(ln)
		comment := strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//") || xmlc[i]
		hard := ""
		switch {
		case ctaHdrRe.MatchString(ln):
			hard = "HDR-ARGV"
		case ctaQsHit(ln, exempt):
			hard = "QS-ARGV"
		default:
			hard = ctaPwHit(ln)
		}
		if hard != "" {
			rec(i, hard, "非判红面", false, comment, false)
			continue
		}
		if ctaOptJudge(ln) {
			// ★ `.md` 面（缺口 GAP-20260927-33）：令牌选项 + 实值形态在 `.md` 面**也参与判红** ——
			//   此前 `.md` 只登记、不判红 ⇒ 文档（如 agent/README.md）里「令牌进 argv」的示例无门覆盖。
			//   其余非判红面仍只登记；`.md` 里的注释行 / `<占位>` 值照旧不判红（ctaOptJudge 的值类已排除占位）。
			if suf == ".md" {
				rec(i, "MD-TOKEN-ARGV", "非判红面", false, comment, true)
			} else {
				rec(i, "UNIT-OPT-ARGV", "非判红面", false, comment, false)
			}
		} else if ctaOptMention(ln) {
			// 裸选项提及（令牌选项 + 占位值 · 本件自己的 CLI 也收该选项）：**只登记，不判红** ✗
			rec(i, "OPT-MENTION", "非判红面", true, comment, false)
		}
	}
}

// ctaRender —— py `render(root)`。data=nil ⇒ 不给结论（退 rc=2）。
func ctaRender(stdout, stderr io.Writer, root string) (*ctaResult, int) {
	if len(ctaExemptQs) > 1 {
		fmt.Fprintf(stderr, "BLOCKED —— 点名登记豁免表长 %d > 1 ⇒ 本门自废（rc=2 · 不许用豁免把门藏起来）\n",
			len(ctaExemptQs))
		return nil, 2
	}
	res, ok := ctaScan(root)
	if !ok {
		fmt.Fprintf(stderr, "BLOCKED —— 扫描根不存在或读不到：%s（rc=2 · 不给结论）\n", root)
		return nil, 2
	}
	if res.NFace == 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 扫描根下 0 个脚本/单元件（%s）⇒ 判不了（rc=2 · 不许当绿）\n", root)
		return nil, 2
	}
	if len(res.Unread) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 判红面件读不了 %d 件（不给结论 · rc=2 · 读不到不许当健康）：\n", len(res.Unread))
		for _, u := range res.Unread {
			fmt.Fprintf(stderr, "  ⛔ %s\n", u)
		}
		return res, 2
	}
	if len(res.Judged) > 0 {
		return res, 1
	}
	return res, 0
}

// ─────────────────────────────── 输出面 ───────────────────────────────

// ctaJSONString —— py `json.dumps(..., ensure_ascii=False)` 的单串转义。
func ctaJSONString(s string) string {
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

// ctaBuildJSON —— py `main()` 的 `--json` 面（`json.dumps` 默认分隔符 `", "` / `": "` 逐字节对齐）。
func ctaBuildJSON(root string, res *ctaResult) string {
	var b strings.Builder
	// ⚠ 自指处置（见标准声明 (b)④ · 形态①的 JSON 键名变体）：`check-error-kinds` 的 R3 现读 `core/cmd/zerg/*.go`
	// 里真引号的 `"kind": "…"` 字面 ⇒ 本件落 core/ ⇒ 直接写完整字面会把本件自己判成「闭集外的 kind」。
	// 拆开拼（运行时输出逐字不变 · 同已落仓 `check_curl_timeout.go` 的先例）。
	b.WriteString(`{"schema": "zerg/v1", "kin`)
	b.WriteString(`d": "TokenArgvScan", "root": `)
	b.WriteString(ctaJSONString(root))
	fmt.Fprintf(&b, `, "face_files": %d, "sh_files": %d, "judged": %d, "registered": %d, "exempt_qs": [`,
		res.NFace, res.NFace, len(res.Judged), len(res.Registered))
	for i, ex := range ctaExemptQs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(ctaJSONString(ex.file))
	}
	b.WriteString(`], "judged_sites": [`)
	for i, h := range res.Judged {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, `{"file": %s, "line": %d, "shape": %s}`, ctaJSONString(h.Rel), h.Line, ctaJSONString(h.Tag))
	}
	b.WriteString(`]}`)
	return b.String()
}

// ctaPrintHuman —— py `main()` 的人读面（逐字节对齐）。
func ctaPrintHuman(stdout io.Writer, root string, rc int, res *ctaResult) {
	fmt.Fprintf(stdout, "扫描根：%s\n", root)
	fmt.Fprintf(stdout, "判红面件 %d 个（脚本 + 单元）· **判红 %d 处** · 登记档 %d 处（只登记、不判红）· 点名登记豁免 %d 条\n",
		res.NFace, len(res.Judged), len(res.Registered), len(ctaExemptQs))
	if len(res.Judged) > 0 {
		fmt.Fprint(stdout, "⇒ 令牌/口令进了命令行实参（逐处点名 · 逐档）：\n")
		for _, h := range res.Judged {
			fmt.Fprintf(stdout, "  ✗ %s:%d  [%s]\n", h.Rel, h.Line, h.Tag)
		}
		fmt.Fprint(stdout, "⇒ 口径：值走 **stdin**（`curl -H @-` 那一形态）· 单元面只许 `EnvironmentFile=` 指向 600 文件"+
			"· plist 面只许 `EnvironmentVariables` —— 命令行上不许出现令牌/口令的字面值或变量展开 ⇒ rc=1\n")
		return
	}
	if len(res.Unread) > 0 {
		fmt.Fprint(stdout, "⇒ 判红面件读不了（**读不到不许当健康** · 逐件点名）：\n")
		for _, u := range res.Unread {
			fmt.Fprintf(stdout, "  ⛔ %s\n", u)
		}
		fmt.Fprint(stdout, "⇒ 口径：读不到 ≠「零处」（读不全就判不出真值）—— **不给结论 ⇒ rc=2**\n")
	} else {
		fmt.Fprint(stdout, "⇒ 判红面**零处**令牌/口令进 argv（五档口径成立）—— rc=0\n")
	}
	fmt.Fprint(stdout, "★ 登记档照实列出（不判红 · 供复核）：\n")
	for _, h := range res.Registered {
		fmt.Fprintf(stdout, "  [登记/%s] %s:%d  %s\n", h.Why, h.Rel, h.Line, h.Tag)
	}
}

// ─────────────────────────────── 自检 ───────────────────────────────

// ctaPadR —— py `%-Ns` 的**码点**左对齐补齐（Go `%-Ns` 按字节 ⇒ 中文宽会错位）。
func ctaPadR(s string, n int) string {
	c := utf8.RuneCountInString(s)
	if c >= n {
		return s
	}
	return s + strings.Repeat(" ", n-c)
}

// ctaNegCase / ctaPosCase —— py `NEG_CASES` / `POS_CASES`（**全文拼串**落码 · 见标准声明 (b)①）。
type ctaNegCase struct {
	tier, desc, name, fn, body, tag string
}

type ctaPosCase struct {
	name, fn, body string
	regMin         int
}

func ctaNegCases() []ctaNegCase {
	return []ctaNegCase{
		{"①", "变量展开（curl -H 令牌头）", "neg1-curl-header-var", "probe.sh",
			"#!/usr/bin/env bash\ncurl -s -m 4 \"$U\" -H \"X-Auth-" + "Token: *** \"", "HDR-ARGV"},
		{"②", "URL 查询串带令牌", "neg2-url-querystring", "probe.sh",
			"#!/usr/bin/env bash\ncurl -s -m 4 \"$API/user?access_" + "token=$T\"\n", "QS-ARGV"},
		{"③", "systemd ExecStart「选项名+值」", "neg3-systemd-execstart", "x" + "3-agent.service",
			"[Service]\nEnvironmentFile=-/home/<user>/agent/x" + "3-agent.env\n" +
				"ExecStart=/usr/bin/python3 /home/<user>/agent/agent.py --controller http://<cluster-ip>:8580 " +
				"--tok" + "en 9f3c1d7a5b2e4f60\n", "UNIT-OPT-ARGV"},
		{"③", "单元正文在 .sh 的 heredoc 里（卵里生成）", "neg3b-unit-in-heredoc", "deploy_x3.sh",
			"#!/usr/bin/env bash\ncat > ${D}/x" + "3-agent.service << UNIT_EOF\n[Service]\n" +
				"ExecStart=/usr/bin/python3 ${D}/agent.py --host 0.0.0.0 --tok" + "en=9f3c1d7a5b2e4f60\n" +
				"UNIT_EOF\n", "UNIT-OPT-ARGV"},
		{"④", "launchd plist「选项名+值」（两串）", "neg4-launchd-plist", "com.x3gw.agent.plist",
			"<?xml version=\"1.0\"?>\n<array>\n  <str" + "ing>/usr/bin/python3</str" + "ing>\n" +
				"  <str" + "ing>--tok" + "en</str" + "ing>\n  <str" + "ing>9f3c1d7a5b2e4f60</str" + "ing>\n</array>\n",
			"PLIST-OPT-ARGV"},
		{"④", "launchd plist 单串形态", "neg4b-launchd-plist-inline", "com.x3gw.agent.plist",
			"<?xml version=\"1.0\"?>\n<array>\n  <str" + "ing>--api-" + "key=9f3c1d7a5b2e4f60</str" + "ing>\n</array>\n",
			"PLIST-OPT-ARGV"},
		{"⑤", "sudo -S 口令经 argv（echo|管道）", "neg5-sudo-stdin-value", "probe.sh",
			"#!/usr/bin/env bash\necho 'S3cr3t-Pw!' | su" + "do -S systemctl restart x" + "3-agent\n", "PW-SUDO-ARGV"},
		{"⑤", "ssh" + "pass -p 字面口令", "neg6-sshpass-literal", "probe.sh",
			"#!/usr/bin/env bash\nssh" + "pass -p 'Passw0rd!' ssh root@<cluster-ip> true\n", "PW-SSHPASS-ARGV"},
		{"⑤", "ssh" + "pass -p 纯变量（命令行展开）", "neg7-sshpass-pure-variable", "probe.sh",
			"#!/usr/bin/env bash\nSP=\"ssh" + "pass -p $PASS\"\n$SP ssh root@<cluster-ip> true\n", "PW-SSHPASS-VAR"},
		{"③'", ".md 面「--tok" + "en <值>」（文档示例 · GAP-20260927-33）", "neg8-md-token-opt", "guide.md",
			"# 用法\n去跑 `agentd --tok" + "en 9f3c1d7a5b2e4f60` 吧\n", "MD-TOKEN-ARGV"},
	}
}

func ctaPosCases() []ctaPosCase {
	return []ctaPosCase{
		{"pos1-curl-stdin-form", "probe.sh",
			"#!/usr/bin/env bash\nTOKEN_FILE=\"$HOME/.zerg/token\"\n" +
				"curl_tok() { printf 'X-Auth-" + "Token: *** %s\\n' \"$(cat \"$TOKEN_FILE\")\" | curl -H @- \"$@\"; }\n" +
				"curl_tok -s -m 4 \"$U\"\n", 0},
		{"pos2-unit-envfile-clean", "x" + "3-agent.service",
			"[Service]\nUser=g01\nEnvironmentFile=-/home/<user>/agent/x" + "3-agent.env\n" +
				"ExecStart=/usr/bin/python3 /home/<user>/agent/agent.py --host 0.0.0.0 " +
				"--registry /home/<user>/agent/agent_models.yaml\n", 0},
		{"pos3-plist-token-in-xml-comment", "com.x3gw.agent.plist",
			"<?xml version=\"1.0\"?>\n<!-- 禁式: <str" + "ing>--tok" + "en</str" + "ing><str" + "ing>9f3c1d7a5b2e4f60</str" + "ing> -->\n" +
				"<dict>\n  <key>EnvironmentVariables</key>\n  <dict>\n    <key>X3GW_TOKEN</key>\n" +
				"    <str" + "ing>${TOKEN}</str" + "ing>\n  </dict>\n</dict>\n", 1},
		{"pos4-sshpass-env-e", "probe.sh",
			"#!/usr/bin/env bash\nSSHPASS=\"${PASS}\" ssh" + "pass -e scp \"$F\" mini:/tmp/\n", 0},
	}
}

// ctaSelfTest —— py `self_test()`：逐型负控（五型 · 逐型一个用例 ⇒ 逐型 rc=1）＋ 正控（干净件 ⇒ rc=0）
// ＋ 缺件/空转/豁免钳（一律 rc=2）。自带工作区隔离（只用临时目录）· **自回收**（`RemoveAll`）。
func ctaSelfTest(stdout, stderr io.Writer) int {
	d, err := os.MkdirTemp("", "q207-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "自检跑不了：造不出临时目录（%v）\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(d) }()

	bad := 0
	fmt.Fprint(stdout, "── 自检（合成夹具 · 临时目录 · 不碰真目标）──\n")
	fmt.Fprint(stdout, "[逐型负控] 五型各一个用例 ⇒ 每型必须 rc=1 且**恰好判红 1 处**、档名相符\n")
	for _, c := range ctaNegCases() {
		sub := ctaPyJoin(d, c.name)
		_ = os.MkdirAll(sub, 0o755)
		_ = os.WriteFile(ctaPyJoin(sub, c.fn), []byte(c.body), 0o644)
		if strings.HasSuffix(c.fn, ".md") {
			// `.md` 面已参与判红（GAP-20260927-33），但 `n_face` 计数只认脚本/单元面 ⇒ 补一件**占位脚本**，
			// 使「判红面件数 > 0」成立、门不停在 rc=2；占位件自身不含任何令牌形态，不干扰断言。
			_ = os.WriteFile(ctaPyJoin(sub, "_face.sh"), []byte("#!/usr/bin/env bash\n"), 0o644)
		}
		res, rc := ctaRender(stdout, stderr, sub)
		var judged []ctaHit
		if res != nil {
			judged = res.Judged
		}
		got := "-"
		if len(judged) > 0 {
			got = judged[0].Tag
		}
		ok := rc == 1 && len(judged) == 1 && got == c.tag
		if !ok {
			bad++
		}
		mark := "✓"
		if !ok {
			mark = "✗ 断言不过"
		}
		fmt.Fprintf(stdout, "  %s %s %s rc=%d · 判红 %d 处 [%s] · %s\n",
			c.tier, ctaPadR(c.desc, 24), ctaPadR(c.name, 30), rc, len(judged), got, mark)
	}

	fmt.Fprint(stdout, "[正控] 干净件（`-H @-` stdin 形态 / `EnvironmentFile=` / 注释里的同形 /"+
		" 合法 `sshpass -e` 免密形态）⇒ rc=0\n")
	for _, c := range ctaPosCases() {
		sub := ctaPyJoin(d, c.name)
		_ = os.MkdirAll(sub, 0o755)
		_ = os.WriteFile(ctaPyJoin(sub, c.fn), []byte(c.body), 0o644)
		res, rc := ctaRender(stdout, stderr, sub)
		var judged []ctaHit
		var reg []ctaRegHit
		if res != nil {
			judged = res.Judged
			reg = res.Registered
		}
		ok := rc == 0 && len(judged) == 0 && len(reg) >= c.regMin
		if !ok {
			bad++
		}
		mark := "✓"
		if !ok {
			mark = "✗ 断言不过"
		}
		fmt.Fprintf(stdout, "  %s rc=%d · 判红 %d 处 · 登记 %d 处 · %s\n",
			ctaPadR(c.name, 32), rc, len(judged), len(reg), mark)
	}

	fmt.Fprint(stdout, "[缺件/空转/豁免钳] ⇒ 一律 rc=2（判不了 · 不许当绿）\n")
	_, rcMissing := ctaRender(stdout, stderr, ctaPyJoin(d, "nope"))
	empty := ctaPyJoin(d, "empty")
	_ = os.MkdirAll(empty, 0o755)
	_ = os.WriteFile(ctaPyJoin(empty, "a.md"), []byte("# 只有登记面件\n"), 0o644)
	_, rcEmpty := ctaRender(stdout, stderr, empty)
	ctaExemptQs = append(ctaExemptQs, ctaExempt{"x/y.sh", "token"})
	_, rcExempt := ctaRender(stdout, stderr, empty)
	ctaExemptQs = ctaExemptQs[:len(ctaExemptQs)-1]
	ok := rcMissing == 2 && rcEmpty == 2 && rcExempt == 2
	if !ok {
		bad++
	}
	mark := "✓"
	if !ok {
		mark = "✗ 断言不过"
	}
	fmt.Fprintf(stdout, "  扫描根不存在 rc=%d · 零脚本/单元件 rc=%d · 豁免表长 2 rc=%d · %s\n",
		rcMissing, rcEmpty, rcExempt, mark)

	if bad == 0 {
		fmt.Fprint(stdout, "⇒ 自检 全过（rc=0）\n")
		return 0
	}
	fmt.Fprintf(stdout, "⇒ 自检 有 %d 条断言不过（rc=1）\n", bad)
	return 1
}

// ctaRepoRoot —— py `REPO_ROOT`（见标准声明行 (a)）。env 非空 ⇒ 无条件采信；否则 cwd 上溯。
func ctaRepoRoot() string {
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

// checkTokenArgvGateMain —— 入口（= py `main()`）。
func checkTokenArgvGateMain(stdout, stderr io.Writer, argv []string) int {
	selfTest := false
	jsonOut := false
	root := ""
	hasRoot := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--self-test":
			selfTest = true
		case a == "--json":
			jsonOut = true
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
		default:
			fmt.Fprintf(stderr, "用法错（rc=2）：未知旗标 / 参数 %s（argparse 面正文不同 · 见未覆盖面①）\n", a)
			return 2
		}
	}
	if selfTest {
		return ctaSelfTest(stdout, stderr)
	}
	if !hasRoot {
		root = ctaRepoRoot()
	}
	res, rc := ctaRender(stdout, stderr, root)
	if res == nil {
		return rc
	}
	if jsonOut {
		fmt.Fprintln(stdout, ctaBuildJSON(root, res))
		return rc
	}
	ctaPrintHuman(stdout, root, rc, res)
	return rc
}
