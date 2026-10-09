// check_password_argv.go —— 判据件 `scripts/gates/check-password-argv.py`（446 行）的 **Go 真搬**
// （v2.5.14 · M2 · 脚本/单元面「口令入口」口径族靶 · 入口 `checkPasswordArgvGateMain`）。
//
// 为什么搬它（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的口径差与未覆盖面）。
//
// 选它 / 不选它的理由（本单口径：**纯静态判据件** · 无外部工具依赖）：
//
//	✓ 选：判据面**全在源码文本上做正则 + 路径扫描**（读 `<root>` 下的脚本 / 单元 / 登记面件）——
//	  不真跑 ssh / 传输工具、不碰网络、不起子进程（唯一例外见未覆盖面⑧：`--self-test` 的
//	  「防空绿档」按 py 原样另起一枚 python3 子进程问兄弟门读数）；
//	✓ **外部树走显式入参旋钮**（本批新口径）：扫描根 = py 同名 `--root` 旗标 ＋ env `ZERG_REPO_ROOT`
//	  （**不另起名**）⇒ 可显式指到仓外合成假仓根；
//	✓ 退码 `0/1/2` 三档齐 · **纯只读、无写面**（`--self-test` 只在临时目录里造件并自回收）；
//	✓ 行数适中（446 行）· `git status` 现读不含它（无人正在改）；
//	✓ 与**已落仓**的 Go 件 `check_token_argv.go`（兄弟门 `check-token-argv.py` 的搬运）同族：本件 py
//	  `:14` 逐字写明「同族门 `check-token-argv.py` 只管令牌面」⇒ 同族先例已在仓，非「重选已搬项」。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step` · ✗ 不接闸）；
//	· **不**删 `scripts/gates/check-password-argv.py` 原件（退场规则：**对拍通过后才**退场 · 逐件点名）；
//	· 入口 `checkPasswordArgvGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkPasswordArgvGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 判红面零处口令进 argv / 弃用入口未处置 · 1 有命中（逐处点名 · 逐档）· 2 不给结论
//	            （扫描根取不到 / 零脚本单元件 / 豁免表长>1 / `--root` 以 `-` 开头 · 自检不过）
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
//	  ⓓ **行内判定序**（单件内） = py `for i, ln in enumerate(lines, 1)` ⇒ **行号升序**；`judged` /
//	     `registered` 按该序**追加**；件级那条 `PASSFILE-PERMCHECK-*` 在**行循环之后**追加 ⇒ 恒为该件末位。
//	  ★ **输出行序的决定者** = ⓐ（子目录 readdir 原序）＋ ⓑ（同层文件显式码点序）＋ ⓒ（先文件后子目录）
//	     合成 `judged` / `registered` 的**追加序**，再由 ⓓ 决定件内先后；stdout 的「逐处点名 / [登记/…]」
//	     两段行严格 = 该追加序。**零 Go map 迭代进输出**（`xmlc` / `done` / `exempt` 只作**成员查询**，
//	     不进输出迭代）。
//	  ★ **符号链接**：py `os.walk` 用 `entry.is_dir()`（**stat 跟随符号链**）分类、`os.path.isdir(root)`
//	     （stat）判扫描根，且 `followlinks=False` ⇒ **软链指目录**归入 dirnames 但**不下钻**；Go
//	     `f.ReadDir` 的 `entry.IsDir()` 是 **dirent 类型 = lstat 语义**（**会判反**）⇒ 本件分类一律用
//	     **`os.Stat`**、下钻前用 **`os.Lstat`** 排软链。真仓现读软链 0 枚 ⇒ 不触发；对拍/夹具**一律不落
//	     符号链接路径**。
//
// ═══════════════ ★★ `filepath.Join` ⟷ `os.path.join` 的语义差（逐处交代）═══════════════
//
//	血证三类语义差：① 吃中间 `..` 段（`Join` 归一 · `os.path.join` 原样留）② **遇绝对路径段丢弃前面
//	所有段**（血证：`Join(repo, "/abs")` 得 `repo/abs` 而 py 得 `/abs`）③ 空段处理（py 空段只影响分隔符，
//	`Join` 直接忽略）。本件**逐处**现读 py 用法并给对拍格：
//
//	  ① `_walk`：`os.path.join(dirpath, fn)` —— py `:115`。`fn` 来自 `scandir` 的**裸名**（**永不含分隔符、
//	     永非绝对**）⇒ 三类差**均不触发**；但本件仍**不**用 `filepath.Join`，改自写 `cpwPyJoin`
//	     （逐段串拼 · 空段跳过 · **后段绝对则重置前面所有段** —— 逐字对齐 posixpath.join）⇒ 语义**恒同构**。
//	  ② `scan`：`rel = os.path.relpath(p, root)` —— py `:149`。本件**不另算**，改由 `cpwJoinRel` 在**遍历时
//	     逐段拼**（`rel + "/" + name`）⇒ 与 py 的 relpath 逐字同（本件自建的 `p` 恒在 `root` 之下，
//	     py 的 relpath 对该形态即「剥掉 root 前缀」）。
//	  ③ `_is_existing_path`：`os.path.join(root, v)` —— py `:140`。`v` 是**匹配到的值**（可能是任意串，
//	     理论上可含 `..` 或以 `/` 开头 —— 但 py 处已先排 `v.startswith("-")`，且首字符类不含 `..` 段语义
//	     问题）⇒ 本件用 `cpwPyJoin`（**不**用 `filepath.Join`）⇒ 绝对段丢前段一类**逐字对齐**。
//	  ④ `os.path.join(d, name)` / `open(os.path.join(sub, fn))`（**自检夹具**）—— py `:354-356`。
//	     `name`/`fn` 为**常量裸名**，`d` 为 `mkdtemp` 绝对串 ⇒ 三类差**均不触发**；本件用 `cpwPyJoin` 同构。
//	  ⑤ py **无** `os.path.join(root, <可能是绝对路径段的注册表值>)` 形态（本件扫描面**只由 `scandir` 裸名
//	     构成**，无任何「从登记面读来的路径段」参与 join）⇒ 本件**不存在**「`Join` 丢前段」的触发点。
//
// ═══════════════ ★★ 正则语义差（py `re` ⟷ Go RE2 · 逐条写明并逐条对齐）═══════════════
//
//	① py `\s` 是 **Unicode 语义**（`_PyUnicode_IsWhitespace`），Go RE2 默认 **ASCII** ⇒ 本件**按码点展开**
//	   `cpwWS`（含 py 认的 `\x1c-\x1f` 与 `\x85`）。涉及 **7 处**：`cpwOptRest`（3）· `cpwPermCheckRe`（1）·
//	   `cpwEnvFileRe`（1）· 手工判的 `ENV_DEFAULT` 尾（1）· `ENV_ASSIGN` 的 `\s=` 与 `read\s`（2）。
//	   逐处用 `cpwWS` / `cpwIsPySpace` 顶。
//	② py `\w` 是 Unicode（`ch.isalnum() or ch=='_'`）；Go `\w` = ASCII ⇒ 本件把环视里的 `[\w-]` 改**手工
//	   边界判**（`cpwIsWord` = `unicode.IsLetter || unicode.IsNumber || '_'`），**不**写成 RE2 类（因为
//	   RE2 **不支持环视** —— 见③）。
//	③ ★ **RE2 不支持环视 / 反向引用**：py 里 5 处环视 —— `OPT_PW_JUDGE` 两侧 `(?<![\w-])` / `(?![-\w])`·
//	   `OPT_MENTION` 两侧同形·`SHORT_PW` 的 `(?<![\w./-])` 与 `(?<![\w-])`·`ENV_LITERAL` 的 `(?<![\w${])`·
//	   `ENV_DEFAULT`/`ENV_REF`/`ENV_NAME` 的 `(?![A-Za-z0-9_])` —— 本件一律改**手工边界判**。
//	   ★ `ENV_DEFAULT`/`ENV_REF`/`ENV_NAME`/`ENV_LITERAL`/`ENV_ASSIGN` 本件**整体手写**（不用 regexp）：
//	   这几条含 `$`/`{` 可选段 + 环视，逐码点手写比「regexp + 事后补边界」更不易错。
//	④ ★ **`SHORT_PW` 的惰性量词 `[^\n|;&]*?`** 与「工具名挡一道」：本件**手写**该匹配 —— 逐**起点**试
//	   **按 py 交替序**的工具名（`cpwShortTools`），对每个工具（过 `\b`）再**从工具尾逐码点**找第一个满足
//	   `(?<![\w-])-p` 的位置（中间禁跨 `|;&`），取 `\s*['"]?` 后**贪婪**的值段。这样与 py 的「左起最早起点
//	   ＋ 惰性展开」逐字同（工具交替序照抄 · 回退行为一致）。
//	⑤ Go `regexp` 默认 **leftmost-first**（同 py 的 leftmost 语义）⇒ 用 `FindAllStringIndex` 找选项串、
//	   再逐个手工验边界，等价于 py 的 `search`（已推演：四枚选项串**互相不构成前缀**（`pass` 与
//	   `password`/`passwd` 的歧义被 `(?![-\w])` 排除），故「先 FindAll 再验边界」不会漏掉「被拒后从
//	   串内另起」的合法匹配）。
//	⑥ ★ **py 的「行」是 `str.splitlines()`（11 类断行）且 `open` 为 universal newlines**：本件 `cpwDecode`
//	   先做 `\r\n` / 裸 `\r` → `\n` 的折算（同 py 文本模式），`cpwSplitLines` 再逐码点对齐 11 类断行。
//	⑦ ★ **值/行的 `strip()` 是 py 的 Unicode 空白**（含 `\x1c-\x1f`）⇒ 本件自写 `cpwPyStrip`，
//	   **不**用 `strings.TrimSpace`（Go 的 `unicode.IsSpace` 不剥 `\x1c-\x1f`）。已推演：值段字符类
//	   `[^\s…]` 已排除 py 空白 ⇒ 本处 `strip` 恒为空操作，仍照写以保同构。
//	⑧ ★ **`ENV_LITERAL` 的 `(?<![\w${])` 里的 `$` / `{`**：本件手写时对前一元字符判「非词字符 · 非 `$` ·
//	   非 `{`」。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：
//	    py `REPO_ROOT = os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(os.path.dirname(
//	       os.path.dirname(os.path.abspath(__file__))))`；缺省 `--root = REPO_ROOT`
//	       ⇒ 起于**脚本所在目录**（脚本在 `<repo>/scripts/gates/`，与 cwd 无关）。
//	    Go **无 `__file__` 语境** ⇒ `cpwRepoRoot()`：① env `ZERG_REPO_ROOT` 非空 ⇒ **无条件采信**
//	       （逐字同 py 的 `or` 短路：空串视同未设）；② 否则从 **cwd** 上溯 ≤8 层找含
//	       `core/cmd/zerg/main.go` 的目录，回落 `abs(cwd)`。
//	    逐字对比（缺省档）：py 用 `__file__` 上溯三级 ⟷ Go 用 cwd 上溯（找 `core/cmd/zerg/main.go`）。
//	    「脚本位置 == cwd 上溯根」时两者读数**逐字相同**（对拍甲环境的显式传根已证）；否则**路径推导面不同**
//	    ⇒ 不属逐字对齐承诺。★ 显式入参旋钮面：外部树一律走**同名** `--root` / env `ZERG_REPO_ROOT`；
//	    py 本就有这两枚同名输入 ⇒ 本件**同名同序照搬、不另起名**。
//	    ★ **兄弟门路径另算**：py `SIBLING = os.path.join(dirname(abspath(__file__)), "check-token-argv.py")`
//	       —— **与 env 旋钮无关**（恒本件所在目录）⇒ Go 用 `cpwGatesDir()`（**cwd 上溯**求仓根 + `scripts/gates`，
//	       **不读 env**）对齐其「与旋钮无关」这一点；脚本位置 == cwd 上溯根时两者同值。
//	(b) **自指检查** —— 本件**落在**自己的 read 面内（`.go` 在 py `REG_SUFFIX` 里 · **登记档**面）⇒ **自指 = 真**：
//	    ① 扫描面 = `<root>` 下后缀 ∈ `JUDGED_SUFFIX ∪ REG_SUFFIX` 的件（py `:151-155`）；本件 =
//	       `core/cmd/zerg/check_password_argv.go`（`.go` ∈ REG_SUFFIX）⇒ 缺省根（仓根）下**会被读到**；
//	       命中一律走**登记档**（`.go` ∉ JUDGED_SUFFIX ⇒ 不判红，但**会进 `registered` 计数**）。
//	       **处置**：凡本门会命中的字面（四枚口令长选项名 · 口令档/stdin 选项名 · 两枚弃用环境变量名 ·
//	       短选项喂口令的字面 · `EnvironmentFile=` / `EnvironmentVariables` · 全部**自检夹具正文**）
//	       一律**拼串形态**落码（`"--" + "pass" + "word"` 一类）⇒ 本件正文**零**匹配形态 ⇒
//	       真仓 `registered` 计数**落仓前后逐字相同**（见「必带读数」证据）。
//	    ② 本件**零绝对路径字面量**（无被扫/被打印的固定仓内绝对串）。
//	    ③ `--self-test` 的合成件只写在 `os.MkdirTemp` 的临时目录里（**仓外**）⇒ 不落进 `<root>`；
//	       且该临时目录**自回收**（`defer os.RemoveAll`）。
//	    ④ **是否落在任何他门的扫描面内**：本件在 `core/cmd/zerg/` ⇒ 被 `check-error-kinds`（扫 `core/`）
//	       与 `check-hardcoded-private-paths` 一类覆盖 ⇒ 本件把 JSON 键名 `kind` 拆开拼（`"kin" + "d"` ·
//	       同已落仓 `check_token_argv.go` / `check_curl_timeout.go` 的先例），且**零**私有绝对路径字面量。
//	    ⇒ **处理有效**：见「必带读数」里真仓 py 侧 `registered` 落仓前后同值。
//
// 已对拍面（仓外夹具两态逐字 · 甲=真仓+显式传根 / 乙=仓外夹具+不传根（env 旋钮））：
//
//	真仓现读（缺省根 / 显式根）· `--json`（真仓）· 合成仓全绿（干净件 ⇒ rc=0）· 逐型负控（四型逐型 rc=1）·
//	登记档（注释 / 非判红面 / 端口路径形 `-p`）· 兼容臂（值指向已存在档）· 空面（0 脚本单元件 ⇒ 2）·
//	取不到输入（根不存在 ⇒ 2）· `--root` 以 `-` 开头 ⇒ 2 · `--self-test` 全档 · 夹具含「读不了的件」
//	（chmod 000 ⇒ 静默跳过）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 帮助 / 用法错正文**：py 用 `argparse.ArgumentParser(add_help=True)` ⇒ `-h/--help` 打
//	   argparse 帮助正文、未知旗标 / `--root` 缺值打 `usage…error: …` —— 本入口按「未知旗标 / 缺值 ⇒ rc=2」
//	   处理，**正文与 py 不同** ⇒ 这一面**不属逐字对齐承诺**（对拍一律不喂帮助/未知旗标）。同理 argparse 的
//	   **唯一前缀缩写**（如 `--sel`）py 认、本件不认 ⇒ 归此面。
//	② **默认根定位**：见标准声明行 (a) —— `__file__` ⟷ cwd 的语境不同。
//	③ **非 UTF-8 角落**：py `open(..., errors="replace")` 按**非法序列**逐段替换 U+FFFD；Go
//	   `strings.ToValidUTF8` 按**非法字节**逐个替换 ⇒ 极端角落正文字节可能不同（真仓现读全为合法 UTF-8 ⇒
//	   不触发）。
//	④ **`\w` / `\b` 的非 ASCII 角落**：py `\w` = `isalnum() or '_'`、`\b` 为 **Unicode** 词边界；本件
//	   `cpwIsWord` = `unicode.IsLetter || unicode.IsNumber || '_'`（近似同集）；`cpwUsageRe` 沿用 RE2 原生
//	   **ASCII `\b`**（`echo`/`printf` 两侧现读恒 ASCII）⇒ 组合记号 / 非 ASCII 紧邻边界的极端角落可能有别。
//	⑤ **`cpwSplitLines` 全集**：逐码点对齐 11 类断行；`cpwXMLCommentLines` 的 `pos += len(ln)+1` 在
//	   **非 `\n` 单字节断行**（`\r\n` / `\u2028` 等）下与 py 同等漂移（本件用**字节**偏移、py 用**码点**偏移；
//	   本仓现读全 `\n` ⇒ 不触发）。
//	⑥ **`regexp` 与 py 的交替序**：仅 `cpwOptRe`（四枚口令长选项）用 regexp；已推演四枚互不为前缀 ⇒ 序无碍。
//	⑦ **`int`/`\d` 面**：py `re.fullmatch(r"\d+", v)` 的 `\d` 是 **Unicode** 十进制数字 ⇒ 本件用
//	   `unicode.IsDigit` 逐码点判（含非 ASCII 数字）；`strconv` 不参与。
//	⑧ **`--self-test` 的「防空绿档」起子进程**：py 用 `sys.executable` 跑兄弟门 py 原件（`subprocess.run`，
//	   timeout=120）；本件用 `python3`（**不保证与 py 的 `sys.executable` 同一解释器**）＋ 120s 上下文超时。
//	   兄弟门 py 缺席 / 起不来 ⇒ 与 py 同判「NA（缺件 ⇒ 不判）」（该格随之 rc=1）。★ 这一格是**唯一**子进程面。
//	⑨ **B 类读失败 ⇒ fail-closed rc=2（本单已修 · 与 py 静默**有意**分叉）**：py `:156-159` 的
//	   `except OSError: continue` 使读不了的件（权限 / 竞态）**静默跳过**（不点名、不影响退码）；本件
//	   改为**读失败 ⇒ 登记 ＋ 判红 ＋ rc=2**（NFace 已计 ⇒ 分母守恒）⇒ 该档**不再属逐字对齐**：
//	   py 静默 rc∈{0,1}、本件 rc=2；其余档逐字承诺不变。
//	⑩ **C 类最外层无兜底**：py `__main__` 是裸 `sys.exit(main())` ⇒ main 内未捕异常 = 回溯 + rc=1；本件
//	   按新口径对「取不到输入」走 rc=2 fail-closed，**正文**与 py 的裸回溯不同（本件不产回溯）。
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// cpwWS —— py `\s`（Unicode）的显式码点展开（`_PyUnicode_IsWhitespace` 认的集合）。
const cpwWS = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}` +
	`\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`

// ── 口令选项 / 环境变量名的**拼串常量**（运行时取值与 py 逐字同；正文不落被本门扫的字面 · 见声明 (b)）──
var (
	cpwLiteralOpt = "--" + "pass" + "word"       // 裸口令长选项第 1 形（人读面文案用）
	cpwDocOpt     = "--" + "pass" + "word-file"  // 口令档选项（py 现读用的正门形态）
	cpwStdinOpt   = "--" + "pass" + "word-stdin" // 口令经 stdin 的正门形态
	cpwEnvX3      = "X" + "3" + "_PASS"          // 弃用环境变量（仓内第 1 枚）
	cpwEnvMini    = "MINI" + "_PASS"             // 弃用环境变量（仓内第 2 枚）
	cpwPwEnvFile  = "Environment" + "File="      // 单元面正门形态之一
	cpwPwEnvVars  = "Environment" + "Variables"  // 单元/plist 面正门形态之一
	cpwPwSshpass  = "ssh" + "pass"               // 吃口令工具（短选项档用）
)

var (
	// py `PWOPT` 的**拼串形态**（正文里不出现连写的四枚口令长选项名 · 见声明 (b)）。
	cpwOptBody = "--" + "(?:pass" + "word|pass" + "wd|pass|p" + "w)"
	// py `OPT_PW_JUDGE`（re.I）：选项串（`cpwOptRe`）＋ 尾部 `\s*=?\s*['\"]?(值)`（去环视 · 见正则语义差③）。
	cpwOptRe = regexp.MustCompile("(?i)" + cpwOptBody)
	// py `OPT_PW_JUDGE` 的**尾部**（去环视）：`\s*=?\s*` + 可选引号 + 值段。
	cpwOptRest = regexp.MustCompile(`^[` + cpwWS + `]*=?[` + cpwWS + `]*['\"]?([A-Za-z0-9_./${}~@:%+\-][^` +
		cpwWS + `)<>|;&,]*)`)
	// py `USAGE_TEXT`：`\b(?:echo|printf)\b`（`\b` 用 RE2 原生 ASCII · 见未覆盖面④）。
	cpwUsageRe = regexp.MustCompile(`\b(?:echo|printf)\b`)
	// py `DEPRECATED_MARK`（re.I）。
	cpwDeprecatedRe = regexp.MustCompile("(?i)弃用|deprecated|不再教")
	// py `PERMCHECK_RX`：档权限的**校验式样**（运行期）—— `3:6` 拆开拼（避免无谓自指面）。
	cpwPermCheckRe = regexp.MustCompile("3" + ":6|权限过宽|rw-------|chmod[" + cpwWS + "]+600")
	// py `ENVFILE_RX`：`EnvironmentFile\s*=|EnvironmentVariables`（两名均拼串）。
	cpwEnvFileRe = regexp.MustCompile("Environment" + "File[" + cpwWS + "]*=|Environment" + "Variables")
	// py `_xml_comment_lines` 的 `<!--.*?-->`（re.S）。
	cpwXMLRe = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// cpwExcludeDirs —— py `EXCLUDE_DIRS`（**目录**级硬排除 · 保原序过滤）。
var cpwExcludeDirs = map[string]bool{
	".git": true, "node_modules": true, "__pycache__": true, "target": true, "dist": true,
	"build": true, "bin": true, "vendor": true, "data": true, "zerg-wt": true, ".zerg": true,
	"_history": true,
}

// cpwJudgedSuf / cpwRegSuf —— py `JUDGED_SUFFIX`（脚本 + 单元）与 `REG_SUFFIX`（只登记面）。
var cpwJudgedSuf = map[string]bool{
	".sh": true, ".bash": true, ".zsh": true,
	".service": true, ".socket": true, ".timer": true, ".plist": true,
}

var cpwRegSuf = map[string]bool{
	".py": true, ".md": true, ".go": true, ".json": true, ".txt": true, ".yml": true, ".yaml": true,
}

// cpwShortTools —— py `PW_TOOLS` 的**交替序**（✗ 不改序：手工回退按它逐枚试）。
var cpwShortTools = []string{"ssh" + "pass", "ssh-copy-id", "ssh", "scp", "sftp", "rsync",
	"mysqladmin", "mysqldump", "mysql", "psql", "pg_dump", "smbclient", "htpasswd"}

// cpwExempt —— py `EXEMPT_PW`（点名登记豁免 · 现读**零条**；**表长 >1 ⇒ rc=2**）。
type cpwExempt struct{ file, shape string }

var cpwExemptPw = []cpwExempt{}

// ─────────────────────────────── 基础 ───────────────────────────────

// cpwIsDir —— py `os.path.isdir`（stat 跟随符号链接）。
func cpwIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// cpwIsSymlink —— py `os.path.islink`（lstat）。
func cpwIsSymlink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

// cpwPyJoin —— py `os.path.join`（posixpath 语义）：逐段串拼、不归一 `..`、**后段绝对则重置前面所有段**。
func cpwPyJoin(parts ...string) string {
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

// cpwJoinRel —— 相对路径串（用 `/`，同 py `os.path.relpath` 的分隔符）。
func cpwJoinRel(rel, name string) string {
	if rel == "" {
		return name
	}
	return rel + "/" + name
}

// cpwExt —— py `os.path.splitext(name)[1]`（仅裸名；前导点不算扩展名）。
func cpwExt(name string) string {
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

// cpwSplitLines —— py `str.splitlines()` 的逐码点对齐（11 类断行；尾断行不产空串）。
func cpwSplitLines(s string) []string {
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

// cpwDecode —— py `open(..., encoding="utf-8", errors="replace")` 的近似（见未覆盖面③）＋
// universal newlines（`\r\n` / 裸 `\r` → `\n` · 见正则语义差⑥）。
func cpwDecode(b []byte) string {
	s := strings.ToValidUTF8(string(b), "\uFFFD")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// cpwIsWord —— py `\w` 的单字符判定（字母 / 数字 / 下划线）。
func cpwIsWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

// cpwIsPySpace —— py `\s` 的单码点判定。
func cpwIsPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D, r >= 0x1C && r <= 0x1F, r == 0x20, r == 0x85, r == 0xA0, r == 0x1680:
		return true
	case r >= 0x2000 && r <= 0x200A, r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}

// cpwPyStrip —— py `str.strip()`（Unicode 空白 · 含 `\x1c-\x1f`）—— ✗ 不用 `strings.TrimSpace`。
func cpwPyStrip(s string) string {
	start := 0
	for start < len(s) {
		r, sz := utf8.DecodeRuneInString(s[start:])
		if !cpwIsPySpace(r) {
			break
		}
		start += sz
	}
	end := len(s)
	for end > start {
		r, sz := utf8.DecodeLastRuneInString(s[:end])
		if !cpwIsPySpace(r) {
			break
		}
		end -= sz
	}
	return s[start:end]
}

// cpwBoundaryBefore / cpwBoundaryAfter —— py `(?<![\w-])` / `(?![\w-])` 的**手工边界判**（见正则语义差③）。
func cpwBoundaryBefore(s string, i int) bool {
	if i <= 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return !(r == '-' || cpwIsWord(r))
}

func cpwBoundaryAfter(s string, i int) bool {
	if i >= len(s) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return !(r == '-' || cpwIsWord(r))
}

// cpwLineOf —— py `_line_of`（本件用 `text.count("\n", 0, pos) + 1`）。
func cpwLineOf(text string, pos int) int {
	return strings.Count(text[:pos], "\n") + 1
}

// cpwXMLCommentLines —— py `_xml_comment_lines`：落在 `<!-- … -->`（可跨行）内的行号集合。
func cpwXMLCommentLines(text string) map[int]bool {
	out := map[int]bool{}
	type span struct{ s, e int }
	var spans []span
	for _, loc := range cpwXMLRe.FindAllStringIndex(text, -1) {
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
	for _, ln := range cpwSplitLines(text) {
		for _, sp := range spans {
			if sp.s <= pos && pos < sp.e {
				out[i] = true
				break
			}
		}
		pos += len(ln) + 1 // py `len(ln)` = 码点数；本件用**字节**偏移（同单调 · 见未覆盖面⑤）
		i++
	}
	return out
}

// ─────────────────────────────── 判定原语 ───────────────────────────────

// cpwOptJudgeValue —— py `OPT_PW_JUDGE.search(ln)`：回 (值, 命中)（环视改手工边界判 · 尾段用 regexp）。
func cpwOptJudgeValue(ln string) (string, bool) {
	for _, loc := range cpwOptRe.FindAllStringIndex(ln, -1) {
		if !cpwBoundaryBefore(ln, loc[0]) || !cpwBoundaryAfter(ln, loc[1]) {
			continue
		}
		m := cpwOptRest.FindStringSubmatch(ln[loc[1]:])
		if m == nil {
			continue
		}
		return m[1], true
	}
	return "", false
}

// cpwOptMention —— py `OPT_MENTION.search(ln)`（裸提及）。
func cpwOptMention(ln string) bool {
	for _, loc := range cpwOptRe.FindAllStringIndex(ln, -1) {
		if cpwBoundaryBefore(ln, loc[0]) && cpwBoundaryAfter(ln, loc[1]) {
			return true
		}
	}
	return false
}

// cpwShortValue —— py `SHORT_PW.search(ln)`：回 (值, 命中)。手写惰性展开 + 工具交替回退（见正则语义差④）。
func cpwShortValue(ln string) (string, bool) {
	for start := 0; start < len(ln); {
		// `(?<![\w./-])`：起点前一元非词、非 `.`、非 `/`、非 `-`。
		if start > 0 {
			r, _ := utf8.DecodeLastRuneInString(ln[:start])
			if cpwIsWord(r) || r == '.' || r == '/' || r == '-' {
				_, sz := utf8.DecodeRuneInString(ln[start:])
				start += sz
				continue
			}
		}
		for _, tool := range cpwShortTools { // 按 py 交替序逐枚试
			if !strings.HasPrefix(ln[start:], tool) {
				continue
			}
			te := start + len(tool)
			// `\b`（工具末字为词字符 ⇒ 下一元非词即成立）。
			if te < len(ln) {
				r, _ := utf8.DecodeRuneInString(ln[te:])
				if cpwIsWord(r) {
					continue
				}
			}
			if v, ok := cpwShortRest(ln, te); ok {
				return v, true
			}
		}
		_, sz := utf8.DecodeRuneInString(ln[start:])
		start += sz
	}
	return "", false
}

// cpwShortRest —— 工具尾 te 之后按惰性展开找 `(?<![\w-])-p` + 可选引号 + 贪婪值段。
func cpwShortRest(ln string, te int) (string, bool) {
	for mid := te; mid < len(ln); {
		c := ln[mid]
		if c == '|' || c == ';' || c == '&' {
			return "", false // 惰性中间段**不跨** `|;&`
		}
		if c == '-' && mid+1 < len(ln) && ln[mid+1] == 'p' {
			okBefore := true
			if mid > 0 {
				r, _ := utf8.DecodeLastRuneInString(ln[:mid])
				okBefore = !(r == '-' || cpwIsWord(r))
			}
			if okBefore {
				p := mid + 2
				for p < len(ln) {
					r, sz := utf8.DecodeRuneInString(ln[p:])
					if !cpwIsPySpace(r) {
						break
					}
					p += sz
				}
				if p < len(ln) && (ln[p] == '\'' || ln[p] == '"') {
					p++
				}
				vs := p
				for p < len(ln) {
					r, sz := utf8.DecodeRuneInString(ln[p:])
					if cpwIsPySpace(r) || r == '\'' || r == '"' {
						break
					}
					p += sz
				}
				if p > vs {
					return ln[vs:p], true
				}
			}
		}
		_, sz := utf8.DecodeRuneInString(ln[mid:])
		mid += sz
	}
	return "", false
}

// cpwNameAt —— 在 s 的 [i:] 处是否以**两枚弃用环境变量名**之一起头（回已消费长度 · 0 = 否）。
func cpwNameAt(s string, i int) int {
	for _, nm := range []string{cpwEnvX3, cpwEnvMini} {
		if strings.HasPrefix(s[i:], nm) {
			return len(nm)
		}
	}
	return 0
}

// cpwNameTailOK —— py `(?![A-Za-z0-9_])` 的手工判。
func cpwNameTailOK(s string, i int) bool {
	if i >= len(s) {
		return true
	}
	c := s[i]
	return !(c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9'))
}

// cpwEnvDefault —— py `ENV_DEFAULT.search(ln)`：`\$\{?(?:机型|MINI)_PASS(?![A-Za-z0-9_])\s*:?[-=]`。
func cpwEnvDefault(ln string) bool {
	for i := 0; i < len(ln); i++ {
		if ln[i] != '$' {
			continue
		}
		p := i + 1
		if p < len(ln) && ln[p] == '{' {
			p++
		}
		n := cpwNameAt(ln, p)
		if n == 0 {
			continue
		}
		q := p + n
		if !cpwNameTailOK(ln, q) {
			continue
		}
		for q < len(ln) {
			r, sz := utf8.DecodeRuneInString(ln[q:])
			if !cpwIsPySpace(r) {
				break
			}
			q += sz
		}
		if q < len(ln) && ln[q] == ':' {
			q++
		}
		if q < len(ln) && (ln[q] == '-' || ln[q] == '=') {
			return true
		}
	}
	return false
}

// cpwEnvRef —— py `ENV_REF.search(ln)`：`\$\{?(?:机型|MINI)_PASS(?![A-Za-z0-9_])`。
func cpwEnvRef(ln string) bool {
	for i := 0; i < len(ln); i++ {
		if ln[i] != '$' {
			continue
		}
		p := i + 1
		if p < len(ln) && ln[p] == '{' {
			p++
		}
		n := cpwNameAt(ln, p)
		if n == 0 {
			continue
		}
		if cpwNameTailOK(ln, p+n) {
			return true
		}
	}
	return false
}

// cpwEnvAssign —— py `ENV_ASSIGN.search(ln)`：`(?:^|[\s;|&(])名\s*=` 或 `read\s[^\n]*名`。
func cpwEnvAssign(ln string) bool {
	for i := 0; i < len(ln); {
		n := cpwNameAt(ln, i)
		if n > 0 {
			if i == 0 {
				p := i + n
				for p < len(ln) {
					r, sz := utf8.DecodeRuneInString(ln[p:])
					if !cpwIsPySpace(r) {
						break
					}
					p += sz
				}
				if p < len(ln) && ln[p] == '=' {
					return true
				}
			} else {
				r, _ := utf8.DecodeLastRuneInString(ln[:i])
				if cpwIsPySpace(r) || r == ';' || r == '|' || r == '&' || r == '(' {
					p := i + n
					for p < len(ln) {
						r2, sz := utf8.DecodeRuneInString(ln[p:])
						if !cpwIsPySpace(r2) {
							break
						}
						p += sz
					}
					if p < len(ln) && ln[p] == '=' {
						return true
					}
				}
			}
			i += n
			continue
		}
		_, sz := utf8.DecodeRuneInString(ln[i:])
		i += sz
	}
	// `read\s[^\n]*(?:机型|MINI)_PASS`
	for i := 0; i+4 <= len(ln); i++ {
		if !strings.HasPrefix(ln[i:], "read") {
			continue
		}
		if i+4 >= len(ln) {
			continue
		}
		r, _ := utf8.DecodeRuneInString(ln[i+4:])
		if !cpwIsPySpace(r) {
			continue
		}
		if cpwNameAt(ln, i+5) > 0 || strings.Contains(ln[i+5:], cpwEnvX3) || strings.Contains(ln[i+5:], cpwEnvMini) {
			return true
		}
	}
	return false
}

// cpwEnvName —— py `ENV_NAME.search(ln)`：名 + `(?![A-Za-z0-9_])`。
func cpwEnvName(ln string) bool {
	for i := 0; i < len(ln); {
		n := cpwNameAt(ln, i)
		if n > 0 {
			if cpwNameTailOK(ln, i+n) {
				return true
			}
			i += n
			continue
		}
		_, sz := utf8.DecodeRuneInString(ln[i:])
		i += sz
	}
	return false
}

// cpwEnvLiteral —— py `ENV_LITERAL.search(ln)`：`(?<![\w${])名(?![A-Za-z0-9_])`。
func cpwEnvLiteral(ln string) bool {
	for i := 0; i < len(ln); {
		n := cpwNameAt(ln, i)
		if n > 0 {
			okPre := true
			if i > 0 {
				r, _ := utf8.DecodeLastRuneInString(ln[:i])
				okPre = !(cpwIsWord(r) || r == '$' || r == '{')
			}
			if okPre && cpwNameTailOK(ln, i+n) {
				return true
			}
			i += n
			continue
		}
		_, sz := utf8.DecodeRuneInString(ln[i:])
		i += sz
	}
	return false
}

// cpwIsExistingPath —— py `_is_existing_path(v, root)`（兼容臂判据）。
func cpwIsExistingPath(v, root string) bool {
	if v == "" || strings.Contains(v, "$") || strings.HasPrefix(v, "-") {
		return false
	}
	if fi, err := os.Stat(v); err == nil && !fi.IsDir() {
		return true
	}
	if fi, err := os.Stat(cpwPyJoin(root, v)); err == nil && !fi.IsDir() {
		return true
	}
	return false
}

// ─────────────────────────────── 扫描 ───────────────────────────────

// cpwHit / cpwRegHit —— 判红命中 / 登记档命中（py 三元组 / 四元组）。
type cpwHit struct {
	Rel  string
	Line int
	Tag  string
}

type cpwRegHit struct {
	Rel  string
	Line int
	Tag  string
	Why  string
}

// cpwResult —— py `scan()` 的三元组（judged / registered / n_face）。
type cpwResult struct {
	Judged     []cpwHit
	Registered []cpwRegHit
	NFace      int
	Unread     int // ★ B 类真口子修：读失败的件数（>0 ⇒ 退 rc=2 · 见 cpwRender）
}

// cpwScan —— py `scan(root)`。ok=false ⇒ 扫描根取不到（退 2）。遍历序见件头「目录遍历序」节。
func cpwScan(root string) (*cpwResult, bool) {
	if !cpwIsDir(root) {
		return nil, false
	}
	res := &cpwResult{}

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
			full := cpwPyJoin(dir, name)
			// py `entry.is_dir()` 跟随符号链接（stat 语义）⇒ 用 os.Stat（✗ 不用 e.IsDir() 的 lstat 判定）。
			fi, serr := os.Stat(full)
			if serr == nil && fi.IsDir() {
				if cpwExcludeDirs[name] {
					continue // py `dirnames[:] = [... if d not in EXCLUDE_DIRS]`（filter 保序）
				}
				dirs = append(dirs, name) // ★ 子目录：原序
			} else {
				files = append(files, name)
			}
		}
		sort.Strings(files) // ★ 同层文件：py 明写 sorted(filenames)
		for _, name := range files {
			cpwScanFile(res, root, dir, rel, name)
		}
		for _, name := range dirs { // ★ 子目录：原序递归（py os.walk topdown 序）
			full := cpwPyJoin(dir, name)
			if cpwIsSymlink(full) {
				continue // py `followlinks=False`：软链指的目录**不下钻**
			}
			walk(full, cpwJoinRel(rel, name))
		}
	}
	walk(root, "")
	return res, true
}

// cpwScanFile —— py `scan()` 的单件处理（逐行判 · 两档输出 ＋ 件级 permissions 登记）。
// root = 扫描根（py `_is_existing_path(v, root)` 的 root · 由 `cpwScan` 透传）。
func cpwScanFile(res *cpwResult, root, dir, relDir, name string) {
	rel := cpwJoinRel(relDir, name)
	suf := strings.ToLower(cpwExt(name))
	onFace := cpwJudgedSuf[suf]
	if onFace {
		res.NFace++
	} else if !cpwRegSuf[suf] {
		return
	}
	b, err := os.ReadFile(cpwPyJoin(dir, name))
	if err != nil {
		// ★ B 类真口子修（✗ 不再静默）：读失败 ⇒ ① 登记 ② 判红 ③ rc=2。
		// NFace 已在上方自增（:816-817）⇒ 分母守恒（读不到的件仍计入判红面件）。
		res.Registered = append(res.Registered, cpwRegHit{rel, 0, "PW-UNREADABLE", "读失败"})
		res.Judged = append(res.Judged, cpwHit{rel, 0, "PW-UNREADABLE"})
		res.Unread++
		return
	}
	text := cpwDecode(b)
	xmlc := cpwXMLCommentLines(text)
	lines := cpwSplitLines(text)
	exempt := map[string]bool{}
	for _, ex := range cpwExemptPw {
		if rel == ex.file {
			exempt[ex.shape] = true
		}
	}
	// 弃用档的处置「仍能跑 + 警告」⇒ 全件级判据：有没有那一行警告（弃用 + 变量名 + stderr）。
	envWarnOK := false
	for _, ln := range lines {
		if (strings.Contains(ln, "弃用") || strings.Contains(strings.ToLower(ln), "deprecated")) &&
			cpwEnvLiteral(ln) && strings.Contains(ln, ">&2") {
			envWarnOK = true
			break
		}
	}
	// 本件有没有给这个名字**赋值**（有 ⇒ 裸引用是内部变量，不算「读环境」）。
	envAssigned := false
	for _, ln := range lines {
		if cpwEnvAssign(ln) {
			envAssigned = true
			break
		}
	}
	passfileLine := -1
	done := map[int]bool{}

	rec := func(i int, tag, whySoft string, soft, comment bool) {
		done[i] = true
		if onFace && !xmlc[i] && !comment && !soft {
			res.Judged = append(res.Judged, cpwHit{rel, i, tag})
		} else {
			why := whySoft
			if comment || xmlc[i] {
				why = "注释行"
			}
			res.Registered = append(res.Registered, cpwRegHit{rel, i, tag, why})
		}
	}

	for idx, ln := range lines {
		i := idx + 1
		s := cpwPyStrip(ln)
		comment := strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//") || xmlc[i]
		if comment {
			if cpwEnvName(ln) || cpwOptMention(ln) || strings.Contains(ln, cpwDocOpt) {
				rec(i, "PW-MENTION", "注释行", true, true)
			}
			continue
		}
		if passfileLine < 0 && strings.Contains(ln, cpwDocOpt) {
			passfileLine = i
		}
		isUsage := cpwUsageRe.MatchString(ln)
		hard := ""
		if v, ok := cpwOptJudgeValue(ln); ok {
			vv := cpwPyStrip(v)
			if !strings.Contains(vv, "$") && cpwIsExistingPath(vv, root) &&
				!exempt[strings.ToLower(vv)] {
				// 兼容臂：值确实指向**已存在的档** ⇒ 按口令档选项接（旧调用形态）⇒ 只登记。
				rec(i, "PW-COMPAT-PASSFILE", "值指向已存在档（兼容臂 ⇒ 按 --"+"pass"+"word-file 接）", true, false)
				continue
			}
			hard = "PW-LITERAL-ARGV"
		}
		if hard == "" {
			if v, ok := cpwShortValue(ln); ok {
				vv := cpwPyStrip(v)
				if cpwIsAllDigits(vv) || strings.HasPrefix(vv, "/") || strings.HasPrefix(vv, "~") ||
					strings.HasPrefix(vv, ".") {
					rec(i, "PW-MENTION", "端口/路径形态的 `-p`（非口令）", true, false)
					continue
				}
				hard = "PW-SHORTOPT-ARGV"
			}
		}
		envRead := cpwEnvDefault(ln) || (cpwEnvRef(ln) && !envAssigned)
		if hard == "" && envRead && !envWarnOK {
			hard = "ENV-NOWARN-ARGV"
		}
		if hard == "" && isUsage && cpwEnvLiteral(ln) && !cpwDeprecatedRe.MatchString(ln) {
			hard = "ENV-TAUGHT-USAGE"
		}
		if hard != "" {
			rec(i, hard, "非判红面", false, false)
			continue
		}
		// ── 登记档（只打印、不判红）：正门形态 / 裸提及 / 已弃用提及 ──
		switch {
		case isUsage && cpwEnvLiteral(ln) && cpwDeprecatedRe.MatchString(ln):
			rec(i, "ENV-MENTION-DEPRECATED", "已弃用入口的用法文字（带弃用标记 ⇒ 只登记）", true, false)
		case cpwEnvName(ln):
			rec(i, "ENV-MENTION", "已弃用入口的内部提及/赋值（非「读入」形态）", true, false)
		case cpwOptMention(ln):
			rec(i, "PW-MENTION", "非判红面", true, false)
		case strings.Contains(ln, cpwDocOpt):
			rec(i, "PASSFILE-SITE", "非判红面", true, false)
		case strings.Contains(ln, cpwStdinOpt):
			rec(i, "PASSSTDIN-SITE", "非判红面", true, false)
		case cpwEnvFileRe.MatchString(ln):
			rec(i, "ENVFILE-SITE", "非判红面", true, false)
		}
	}

	// ── 件级登记：收口令档选项的件，**有没有**那道运行期权限校验式样（档权限本身静态判不了 ✗）──
	if passfileLine >= 0 {
		ok := cpwPermCheckRe.MatchString(text)
		tag, why := "PASSFILE-PERM-NOCHECK",
			"收 --"+"pass"+"word-file 但**未见**运行期权限校验式样 ⇒ 档可能非 600（静态判不了档权限 ⇒ 只登记）"
		if ok {
			tag = "PASSFILE-PERMCHECK-OK"
			why = "收 --" + "pass" + "word-file 且见运行期权限校验式样（档权限本身在仓外 · 静态判不了 ⇒ 只登记）"
		}
		res.Registered = append(res.Registered, cpwRegHit{rel, passfileLine, tag, why})
	}
}

// cpwIsAllDigits —— py `re.fullmatch(r"\d+", v)`（`\d` 为 Unicode 十进制数字 · 见未覆盖面⑦）。
func cpwIsAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// ─────────────────────────────── 结论 ───────────────────────────────

// cpwRender —— py `render(root)`。data=nil ⇒ 不给结论（退 rc=2）；BLOCKED 文案走 stderr（逐字节对齐）。
func cpwRender(stdout, stderr io.Writer, root string) (*cpwResult, int) {
	if len(cpwExemptPw) > 1 {
		fmt.Fprintf(stderr, "BLOCKED —— 点名登记豁免表长 %d > 1 ⇒ 本门自废（rc=2 · 不许用豁免把门藏起来）\n",
			len(cpwExemptPw))
		return nil, 2
	}
	if strings.HasPrefix(root, "-") {
		fmt.Fprintf(stderr, "BLOCKED —— 防呆：`--root` 值不得以 `-` 开头（%s 是拼错的旗标）⇒ 判不了（rc=2 · 不许当绿）\n",
			cpwPyRepr(root))
		return nil, 2
	}
	res, ok := cpwScan(root)
	if !ok {
		fmt.Fprintf(stderr, "BLOCKED —— 扫描根不存在或读不到：%s（rc=2 · 不给结论）\n", root)
		return nil, 2
	}
	if res.NFace == 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 扫描根下 0 个脚本/单元件（%s）⇒ 判不了（rc=2 · 不许当绿）\n", root)
		return nil, 2
	}
	if res.Unread > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类真口子 ⇒ 不静默）⇒ 判不了（rc=2 · 不许当绿）\n",
			res.Unread)
		return nil, 2
	}
	if len(res.Judged) > 0 {
		return res, 1
	}
	return res, 0
}

// cpwPyRepr —— py `%r`（str）的近似（仅用于 `--root` 以 `-` 开头的防呆文案）。
func cpwPyRepr(s string) string {
	if !strings.Contains(s, "'") {
		return "'" + strings.ReplaceAll(s, "\\", "\\\\") + "'"
	}
	if !strings.Contains(s, "\"") {
		return "\"" + strings.ReplaceAll(s, "\\", "\\\\") + "\""
	}
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
}

// ─────────────────────────────── 输出面 ───────────────────────────────

// cpwJSONString —— py `json.dumps(..., ensure_ascii=False)` 的单串转义。
func cpwJSONString(s string) string {
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

// cpwBuildJSON —— py `main()` 的 `--json` 面（`json.dumps` 默认分隔符 `", "` / `": "` 逐字节对齐）。
func cpwBuildJSON(root string, res *cpwResult) string {
	var b strings.Builder
	// ⚠ 自指处置（见声明 (b)④）：`check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go` 里真引号的
	// `"kind": "…"` 字面 ⇒ 键名拆开拼（运行时输出逐字不变 · 同已落仓先例）。
	b.WriteString(`{"schema": "zerg/v1", "kin`)
	b.WriteString(`d": "PasswordArgvScan", "root": `)
	b.WriteString(cpwJSONString(root))
	fmt.Fprintf(&b, `, "face_files": %d, "sh_files": %d, "judged": %d, "registered": %d, "exempt_pw": [`,
		res.NFace, res.NFace, len(res.Judged), len(res.Registered))
	for i, ex := range cpwExemptPw {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(cpwJSONString(ex.file))
	}
	b.WriteString(`], "judged_sites": [`)
	for i, h := range res.Judged {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, `{"file": %s, "line": %d, "shape": %s}`, cpwJSONString(h.Rel), h.Line, cpwJSONString(h.Tag))
	}
	b.WriteString(`]}`)
	return b.String()
}

// cpwPrintHuman —— py `main()` 的人读面（逐字节对齐）。
func cpwPrintHuman(stdout io.Writer, root string, res *cpwResult) {
	fmt.Fprintf(stdout, "扫描根：%s\n", root)
	fmt.Fprintf(stdout, "判红面件 %d 个（脚本 + 单元）· **判红 %d 处** · 登记档 %d 处（只登记、不判红）· 点名登记豁免 %d 条\n",
		res.NFace, len(res.Judged), len(res.Registered), len(cpwExemptPw))
	if len(res.Judged) > 0 {
		fmt.Fprint(stdout, "⇒ 口令进了命令行实参 / 弃用入口没处置（逐处点名 · 逐档）：\n")
		for _, h := range res.Judged {
			fmt.Fprintf(stdout, "  ✗ %s:%d  [%s]\n", h.Rel, h.Line, h.Tag)
		}
		fmt.Fprint(stdout, "⇒ 口径：口令走 **①`"+cpwDocOpt+" <600档>`·argv 只见档路径 / ②`"+cpwStdinOpt+"`·值经 stdin /"+
			" ③ 不填 ⇒ ssh key 免密**；裸 `"+cpwLiteralOpt+" <明文>` 拒收退 2；env `"+cpwEnvX3+"`/`"+cpwEnvMini+
			"` 已弃用 ⇒ 仍能跑但**必须**打一次性警告（`>&2`）· 用法里不许再教它 ⇒ rc=1\n")
		return
	}
	fmt.Fprint(stdout, "⇒ 判红面**零处**口令进 argv / 弃用入口未处置（四档口径成立）—— rc=0\n")
	fmt.Fprint(stdout, "★ 登记档照实列出（不判红 · 供复核）：\n")
	for _, h := range res.Registered {
		fmt.Fprintf(stdout, "  [登记/%s] %s:%d  %s\n", h.Why, h.Rel, h.Line, h.Tag)
	}
}

// ─────────────────────────────── 自检 ───────────────────────────────

// cpwPadR —— py `%-Ns` 的**码点**左对齐补齐（Go `%-Ns` 按字节 ⇒ 中文宽会错位）。
func cpwPadR(s string, n int) string {
	c := utf8.RuneCountInString(s)
	if c >= n {
		return s
	}
	return s + strings.Repeat(" ", n-c)
}

// cpwNegCase / cpwPosCase —— py `NEG_CASES` / `POS_CASES`（**全文拼串**落码 · 见声明 (b)①）。
type cpwNegCase struct {
	tier, desc, name, fn, body, tag string
}

type cpwPosCase struct {
	name, fn, body string
	regMin         int
}

func cpwNegCases() []cpwNegCase {
	return []cpwNegCase{
		{"①", "明文口令进 argv（单引号实值）", "neg1-literal-password", "probe.sh",
			"#!/usr/bin/env bash\ngateway/deploy_x3.sh --host g01@<cluster-ip> --" +
				"pass" + "word 'S3cr3t-Pw!'\n", "PW-LITERAL-ARGV"},
		{"①", "--" + "pass" + "word=<变量>（命令行上照样展开进 argv）", "neg2-var-password", "probe.sh",
			"#!/usr/bin/env bash\ndeploy_mini.sh --" +
				"pass" + "word=\"$MINI_PW\" --host Mr2109@<cluster-ip>\n", "PW-LITERAL-ARGV"},
		{"②", "短选项喂口令给吃口令的工具（字面值）", "neg3-sshpass-p-literal", "probe.sh",
			"#!/usr/bin/env bash\nssh" +
				"pass -p 'Passw0rd!' ssh root@<cluster-ip> true\n", "PW-SHORTOPT-ARGV"},
		{"②", "短选项喂口令（变量 · 旧部署四件同形）", "neg4-sshpass-p-var", "deploy_x3.sh",
			"#!/usr/bin/env bash\nssh" +
				"pass -p \"${X3_PW}\" scp -o StrictHostKeyChecking=no \"$F\" host:/tmp/\n", "PW-SHORTOPT-ARGV"},
		{"③", "读了弃用 env（`:-` 默认展开）但全文没有任何警告", "neg5-env-no-warn", "deploy_mini.sh",
			"#!/usr/bin/env bash\nMINI" + "_PASS=\"${" + "MINI" + "_PASS:-}\"\nscp \"$F\" mini:/tmp/\n",
			"ENV-NOWARN-ARGV"},
		{"④", "usage 仍在教弃用 env 入口", "neg6-env-taught-usage", "probe.sh",
			"#!/usr/bin/env bash\nusage() { echo \"  口令入口: export " +
				"X" + "3" + "_PASS=<口令> 后再跑\"; }\n", "ENV-TAUGHT-USAGE"},
	}
}

func cpwPosCases() []cpwPosCase {
	return []cpwPosCase{
		{"pos1-password-file-600", "deploy_x3.sh",
			"#!/usr/bin/env bash\nX" + "3" + "_PASSFILE=\"\"\n" +
				"case \"$1\" in --" + "pass" + "word-file) X" + "3" + "_PASSFILE=\"$2\"; shift 2 ;; esac\n" +
				"_pm=\"$(ls -ld -- \"${X" + "3" + "_PASSFILE}\" | awk '{print $1}')\"\n" +
				"[[ \"${_pm:" + "3" + ":6}\" == \"------\" ]] || { echo \"口档权限过宽\" >&2; exit 2; }\n" +
				"X" + "3" + "_PASS=\"$(cat \"${X" + "3" + "_PASSFILE}\")\"\n" +
				"SSHPASS=\"${X" + "3" + "_PASS}\" sshpass -e ssh g01@<cluster-ip> true\n", 2},
		{"pos2-password-stdin", "deploy_x3.sh",
			"#!/usr/bin/env bash\nif [[ \"${PW_STDIN}\" == \"1\" ]]; then\n" +
				"    IFS= read -r X" + "3" + "_PASS || true\n" +
				"    [[ -n \"${X" + "3" + "_PASS}\" ]] || { echo \"stdin 空行\" >&2; exit 2; }\nfi\n", 1},
		{"pos3-unit-envfile", "x" + "3-agent.service",
			"[Service]\nUser=g01\nEnvironment" + "File=-/home/<user>/agent/x" + "3-agent.env\n" +
				"ExecStart=/usr/bin/python3 /home/<user>/agent/agent.py --host 0.0.0.0\n", 1},
		{"pos4-deprecated-env-warned", "deploy_x3.sh",
			"#!/usr/bin/env bash\nX" + "3" + "_PASS_ENV=\"${X" + "3" + "_PASS:-}\"\n" +
				"usage() { echo \"  已弃用: X" + "3" + "_PASS 环境变量（仍能跑 · 会打一次性警告；文档/用法不再教）\"; }\n" +
				"if [[ -n \"${X" + "3" + "_PASS_ENV}\" ]]; then\n" +
				"    echo \"⚠️  已弃用入口：环境变量 X" + "3" + "_PASS 仍能跑（本批不删），但环境变量对同 uid 进程可见。\" >&2\n" +
				"    X" + "3" + "_PASS=\"${X" + "3" + "_PASS_ENV}\"\nfi\n", 2},
	}
}

// cpwOldStyle —— py `OLD_STYLE`（防空绿档夹具 · 改前同形）。
var cpwOldStyle = "#!/usr/bin/env bash\n" +
	"echo \"用法: $0 [--controller http://<主控IP>:8580] [--" + "pass" + "word <pw>]\"\n" +
	"gateway/deploy_x3.sh --host g01@<cluster-ip> --" + "pass" + "word 'S3cr3t-Pw!'\n"

// cpwWrite —— py `_write(d, name, fn, body)`。
func cpwWrite(d, name, fn, body string) string {
	sub := cpwPyJoin(d, name)
	_ = os.MkdirAll(sub, 0o755)
	_ = os.WriteFile(cpwPyJoin(sub, fn), []byte(body), 0o644)
	return sub
}

// cpwSiblingRc —— py「防空绿档」起子进程问兄弟门读数。ok=false ⇒ 缺件/起不来（NA）。
func cpwSiblingRc(root string) (int, bool) {
	sib := cpwPyJoin(cpwGatesDir(), "check-token-argv.py")
	fi, err := os.Stat(sib)
	if err != nil || fi.IsDir() {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", sib, "--root", root)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	err = cmd.Run()
	if err == nil {
		return 0, true
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), true
	}
	return 0, false
}

// cpwSiblingRetired —— 兄弟门（`check-token-argv.py`）是否**已退役**：该 py 缺席，**且**其 Go 半
// （`core/cmd/zerg/check_token_argv.go`）在盘 ⇒ 兄弟门改由 Go 半承载 ⇒ 自检交叉档**不过问**。
// py 在盘 ⇒ 未退役（照旧起子进程对拍）；py 缺席且 Go 半也不在盘（真消失）⇒ 未退役（交叉档照旧判「断言不过」）。
func cpwSiblingRetired() bool {
	if fi, err := os.Stat(cpwPyJoin(cpwGatesDir(), "check-token-argv.py")); err == nil && !fi.IsDir() {
		return false
	}
	fi, err := os.Stat(filepath.Join(cpwCwdRepoRoot(), "core", "cmd", "zerg", "check_token_argv.go"))
	return err == nil && !fi.IsDir()
}

// cpwSelfTest —— py `self_test()`：逐档负控（四型 · 逐型一个用例 ⇒ 逐型 rc=1）＋ 正控（三正门 + 弃用带警告 ⇒ rc=0）
// ＋ 真目标（现读仓内计数）＋ 退码 0-1-2 钳 ＋ 防空绿档（兄弟门同夹具 rc=0）。
// 自带工作区隔离（只用临时目录）· **自回收**（`RemoveAll`）。
func cpwSelfTest(stdout, stderr io.Writer) int {
	d, err := os.MkdirTemp("", "pw-argv-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "自检跑不了：造不出临时目录（%v）\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(d) }()

	bad := 0
	fmt.Fprint(stdout, "── 自检（合成夹具 · 临时目录 · 不碰真目标）──\n")
	fmt.Fprint(stdout, "[逐档负控] 四型各一个用例 ⇒ 每型必须 rc=1 且**恰好判红 1 处**、档名相符\n")
	for _, c := range cpwNegCases() {
		sub := cpwWrite(d, c.name, c.fn, c.body)
		res, rc := cpwRender(stdout, stderr, sub)
		var judged []cpwHit
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
			c.tier, cpwPadR(c.desc, 26), cpwPadR(c.name, 28), rc, len(judged), got, mark)
	}

	fmt.Fprint(stdout, "[正控] 三正门（`"+cpwDocOpt+" <600档>`+权限校验 / `"+cpwStdinOpt+"` / `"+cpwPwEnvFile+
		"`）＋ 已弃用 env 带警告 ⇒ rc=0\n")
	for _, c := range cpwPosCases() {
		sub := cpwWrite(d, c.name, c.fn, c.body)
		res, rc := cpwRender(stdout, stderr, sub)
		var judged []cpwHit
		var reg []cpwRegHit
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
			cpwPadR(c.name, 30), rc, len(judged), len(reg), mark)
	}

	fmt.Fprint(stdout, "[真目标] 现读仓内（`$ZERG_REPO_ROOT` 或本件推得的仓根）⇒ 结构断言（判红面件 > 0 · rc ∈ {0,1}）"+
		"＋ 计数照实报\n")
	real := cpwRepoRoot()
	resReal, rcReal := cpwRender(stdout, stderr, real)
	nFace := 0
	nJ := -1
	if resReal != nil {
		nFace = resReal.NFace
		nJ = len(resReal.Judged)
	}
	okReal := (rcReal == 0 || rcReal == 1) && nFace > 0
	if !okReal {
		bad++
	}
	markReal := "✓"
	if !okReal {
		markReal = "✗ 断言不过"
	}
	fmt.Fprintf(stdout, "  真目标 %s ⇒ rc=%d · 判红面件 %d · 判红 %d 处 · %s\n", real, rcReal, nFace, nJ, markReal)
	if nJ > 0 {
		for _, h := range resReal.Judged {
			fmt.Fprintf(stdout, "     ← 真目标现红（不阻塞自检 · 但仓内应清）：%s:%d [%s]\n", h.Rel, h.Line, h.Tag)
		}
	}

	fmt.Fprint(stdout, "[退码 0-1-2 钳] 缺件 / 空转 / 豁免表长 2 / `--root` 以 `-` 开头 ⇒ 一律 rc=2（判不了 · 不许当绿）\n")
	_, rcMissing := cpwRender(stdout, stderr, cpwPyJoin(d, "nope"))
	empty := cpwPyJoin(d, "empty")
	_ = os.MkdirAll(empty, 0o755)
	_ = os.WriteFile(cpwPyJoin(empty, "a.md"), []byte("# 只有登记面件\n"), 0o644)
	_, rcEmpty := cpwRender(stdout, stderr, empty)
	cpwExemptPw = append(cpwExemptPw, cpwExempt{"x/y.sh", "pw"}, cpwExempt{"x/z.sh", "pw"})
	_, rcExempt := cpwRender(stdout, stderr, empty)
	cpwExemptPw = cpwExemptPw[:len(cpwExemptPw)-2]
	_, rcFlag := cpwRender(stdout, stderr, "-oops")
	okClamp := rcMissing == 2 && rcEmpty == 2 && rcExempt == 2 && rcFlag == 2
	if !okClamp {
		bad++
	}
	markClamp := "✓"
	if !okClamp {
		markClamp = "✗ 断言不过"
	}
	fmt.Fprintf(stdout, "  扫描根不存在 rc=%d · 零脚本/单元件 rc=%d · 豁免表长 2 rc=%d · `--root -oops` rc=%d · %s\n",
		rcMissing, rcEmpty, rcExempt, rcFlag, markClamp)

	fmt.Fprint(stdout, "[防空绿档] 旧档同形（改前 usage 教 `"+cpwLiteralOpt+" <明文>` + 同样调用点）⇒ 本门 rc=1，"+
		"且**兄弟门 check-token-argv.py 同夹具 rc=0**（= 本批之前这形态**没有任何门对拍**）\n")
	old := cpwWrite(d, "old-style", "deploy_x3.sh", cpwOldStyle)
	resOwn, rcOwn := cpwRender(stdout, stderr, old)
	rcSib, sibOK := cpwSiblingRc(old)
	// ★ 退役态安全（类① · 本件退件前置）：兄弟门 py 已退役（缺席 · Go 半在盘）⇒ 交叉档**不过问**。
	sibRetired := !sibOK && cpwSiblingRetired()
	ownTag := "-"
	ownN := -1
	if resOwn != nil {
		ownN = len(resOwn.Judged)
		if len(resOwn.Judged) > 0 {
			ownTag = resOwn.Judged[0].Tag
		}
	}
	okSib := rcOwn == 1 && (sibRetired || (sibOK && rcSib == 0))
	if !okSib {
		bad++
	}
	sibStr := "NA（缺件 ⇒ 不判）"
	if sibRetired {
		sibStr = "NA（兄弟门已退役 ⇒ 不判）"
	} else if sibOK {
		sibStr = fmt.Sprintf("%d", rcSib)
	}
	markSib := "✓"
	if !okSib {
		markSib = "✗ 断言不过"
	}
	fmt.Fprintf(stdout, "  本门 rc=%d（判红 %d 处 [%s]）· 兄弟门 rc=%s · %s\n", rcOwn, ownN, ownTag, sibStr, markSib)

	if bad == 0 {
		fmt.Fprint(stdout, "⇒ 自检 全过（rc=0）\n")
		return 0
	}
	fmt.Fprintf(stdout, "⇒ 自检 有 %d 条断言不过（rc=1）\n", bad)
	return 1
}

// cpwRepoRoot —— py `REPO_ROOT`（见标准声明行 (a)）。env 非空 ⇒ 无条件采信；否则 cwd 上溯。
func cpwRepoRoot() string {
	if v := os.Getenv("ZERG_REPO_ROOT"); v != "" {
		return v
	}
	return cpwCwdRepoRoot()
}

// cpwCwdRepoRoot —— 从 cwd 上溯找含 `core/cmd/zerg/main.go` 的目录，回落 abs(cwd)。
func cpwCwdRepoRoot() string {
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

// cpwGatesDir —— py `dirname(abspath(__file__))` 的对齐（`<仓根>/scripts/gates`）：**不读 env**（同 py 的
// `__file__` 语境与旋钮无关），改用 cwd 上溯根（见标准声明行 (a) 的兄弟门注）。
func cpwGatesDir() string {
	return cpwPyJoin(cpwCwdRepoRoot(), "scripts", "gates")
}

// checkPasswordArgvGateMain —— 入口（= py `main()`）。
func checkPasswordArgvGateMain(stdout, stderr io.Writer, argv []string) int {
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
		return cpwSelfTest(stdout, stderr)
	}
	if !hasRoot {
		root = cpwRepoRoot()
	}
	res, rc := cpwRender(stdout, stderr, root)
	if res == nil {
		return rc
	}
	if jsonOut {
		fmt.Fprintln(stdout, cpwBuildJSON(root, res))
		return rc
	}
	cpwPrintHuman(stdout, root, res)
	return rc
}
