// audit_go_logs.go —— 判据件 `scripts/gates/audit_go_logs.py`（323 行）的 **Go 真搬**
// （M2 · 「Go 日志/横幅中文字面量审计」族 · 入口 `auditGoLogsGateMain`）。
//
// ═══════════════════ 选件读数（现读 · 落单依据）═══════════════════
//
//	· 真件：`scripts/gates/audit_go_logs.py` —— 16742 字节 / 323 行 / `-rw-r--r--`（非可执行位）
//	  · 形态：**纯静态只读** —— 给定树 + 日志件，输入确定（无网络 / 无子进程 / 无 sudo / 无写目标）。
//	  · 退码三档：0（真跑恒 0 · 进度尺）/ 1（`--self-test` 有格不过 ⇒ **唯一能变红处**）/ 2（判不了）。
//	  · 旗标：`--list` · `--batch <批名>` · `--self-test`/`--selftest` · `-h/--help`；另有 `--json` 前置拒收（rc=2）。
//	  · 接线面（`zerg gate find audit` 现读）：precommit **未点名**；all.sh 按 `*.py` 通配收；real-gates 名录点名@92。
//	· 前缀：`agl`（现读 `zerg code find "\bagl[A-Z]" --path core/cmd/zerg` ⇒ **0 命中** · 不撞名）。
//	· 现读无 `core/cmd/zerg/audit_go_logs.go`（`zerg find audit_go_logs --root core` ⇒ 0 件）⇒ 新落两件。
//
// ═══════════════════ 旋钮（新口径：外部树走显式入参）═══════════════════
//
//	· py **零**「扫描根」旗标 / env：根是 `ROOT = dirname(dirname(dirname(abspath(__file__))))`（脚本自身位置推得），
//	  `CORE = join(ROOT, "core")`。py 侧**无同名旗标/env 可照** ⇒ 按新口径**单列为有意旋钮面差**：
//	  ★ 本件加 `--root <目录>`（**py 无此旗标** ⇒ 两态对拍时该旗标只喂 Go 侧；py 侧一律靠 `__file__` 推导）。
//	· **缺省**（不给 `--root`）= `aglDefaultRepoRoot()`：**从 cwd 上溯**找最近的「含 `scripts/gates/` 子目录」的层。
//	  与 py `__file__` 推导的关系：cwd 落在真仓内（含仓根 / `scripts/gates` 本身）时**逐字同构**；
//	  cwd 在仓外时两者分家（py 仍推真仓 · Go 推 cwd 侧）⇒ 见「未覆盖面①」。
//	· 缺省 = py 推导逐字同构的那一种（cwd 在真仓内）；退 2 边界只判「扫描根都取不到」⇒ rc=2。
//
// ═══════════════════ 遍历 / 排序点（逐点交代 · 本批第一标准）═══════════════════
//
//	py 唯一遍历点 `scan()`：`for root, dirs, names in os.walk(CORE)`（topdown=True · followlinks=False 缺省）。
//	逐点：
//	  ⓐ **目录序** = py `os.walk` 内部 `os.scandir` 的 **readdir 原序**（3.9 走 `os.scandir`，**不排序**）。
//	     ⇒ 本件 `aglWalk` 用 `os.Open` + `f.ReadDir(-1)`（**原序**）—— ✗ **不用** `os.ReadDir` / `filepath.WalkDir`
//	     （两者**会排序** ✗ ⇒ 目录序会与 py 分家）。
//	  ⓑ **同目录文件序** = py 明写 `for n in sorted(names)`（码点序）⇒ 本件 `sort.Strings`（合法 UTF-8 下与码点序相同）。
//	     ★ **行序决定者**：ⓑ 决定「同目录多命中」的相对先后；ⓐ 决定「跨目录」先后。
//	  ⓒ **产出序** = **先产出当前层全部文件（ⓑ 序）、再按 dirs 原序（ⓐ 序）下钻**（os.walk topdown 语义）。
//	     ⛔ 反面：边遍历边下钻（早钻）会把子目录的行插到同层后面文件之前 ⇒ 与 py 分家。本件 `aglWalk` 先 `visit` 后下钻。
//	  ⓓ `dirs[:] = [d for d in dirs if d not in ("vendor", "testdata")]` 是**保序**过滤（滤的是**下钻面**）
//	     ⇒ 本件 `filter` 回调保序返回（✗ 不排序）。
//	  ★ **零 Go map 迭代进输出**：批计数（`byBatch`）按 `aglBatches` 显式切片序打印；文件 Top 用
//	     **切片保首次遇到序**（`order`）+ `sort.SliceStable`（计数降序 · 同数保首次遇到序 = py `Counter.most_common`
//	     的稳定序）⇒ map 仅作 O(1) 取值，**永不进输出序**。
//	  ★ 现读证据（fx1 夹具 · readdir 原序 = `app_test.go, testdata, z9, app.go, b.go, m2, a1, vendor`）：
//	     两态 stdout 逐字节同，行序 = `core/app.go, core/b.go` → `core/z9/z.go, core/m2/m.go, core/a1/a.go`。
//
// ═══════════════════ `filepath.Join` ⟷ `os.path.join` 三语义差（逐处现读）═══════════════════
//
//	py `os.path.join` 调用点逐处现读（本件全文共 3 处真身 + 夹具面若干）：
//	  ① `scan()`：`os.path.join(root, n)` —— `root` 来自 `os.walk`（绝对目录）· `n` 来自 readdir 件名
//	     ⇒ **不含 `.` / `..` / 空段、必非绝对** ⇒ 三语义差（吃 `..` · **后段绝对丢前段** · 空段 `join('a','')=='a/'`）**均不触发**；
//	  ② `main()`：`os.path.join(ROOT, "core")` 与缺省根推导里的 `dirname` 链 —— 同上（普通段）；
//	  ③ 自检夹具：`os.path.join(root, rel)`（rel 形如 `core/app.go`）/ `os.path.join(per, "core", "app.go")` —— 同上。
//	  ⇒ 如实交代：**主逻辑与自检上三类语义差均不触发**（✗ 不空写一套 PyJoin 只为自用）。
//	  ★ 但**仍自写 `aglPyJoin`** 并全文用它拼串，因为 py 另有 `normpath/abspath/relpath/dirname`（**保真必需**）⇒ 一处语义面统一。
//	  ★ `relpath` / `abspath` / `normpath` / `dirname` 逐处现读（**非 join，但同类词法面**）：
//	    · `os.path.relpath(path, ROOT)` ⇒ 自写 `aglPyRelpath`（✗ **不用** `filepath.Rel` —— 后者对跨盘/`..` 面语义不同）；
//	    · `os.path.abspath(__file__)` / `abspath` 在 `ROOT` 推导里 ⇒ 自写 `aglPyAbspath`（相对则先拼 cwd 再 normpath）；
//	    · `os.path.normpath`（经 abspath/relpath 间接进）⇒ 自写 `aglPyNormpath`：**CPython `posixpath.normpath`
//	      保留恰好两个前导斜杠**（`//a/b` ⇒ `//a/b`）⟷ Go `filepath.Clean` 折成 `/a/b` ✗ ⇒ 必须自写；
//	    · `os.path.dirname` ⇒ 自写 `aglPyDirname`（尾斜杠全去、`/` 前缀全斜杠只留 `/`）。
//	  · 本件**零 `realpath`** ⇒ **不涉**「py `realpath(strict=False)` 对悬空链接回词法路径 ⟷ Go `EvalSymlinks` 报错」那一面。
//
// ═══════════════════ 正则（逐条现读 · ✗ 不硬译）═══════════════════
//
//	py 用 `re` 共 2 处，逐条处置：
//	  ① `CJK = re.compile(r'[\u4e00-\u9fff]')` —— 直译：Go RE2 **不吃 `\uXXXX`** ⇒ 写 `[\x{4e00}-\x{9fff}]`。
//	     判据只作 `search`（布尔），无捕获组 / 无环视 / 无 `\d` / 无 `\s` / 无 `\b`。
//	  ② `LOG = re.compile(r'(log\.Printf|log\.Println|log\.Print|fmt\.Printf|fmt\.Println|fmt\.Print|'
//	     r'fmt\.Fprintf|fmt\.Fprintln|println\(|eprintln\(|slog\.|Fatalf\()')` —— **逐字对读**后 RE2 直译：
//	     · **无后顾/先行断言** ⇒ 不需要手写扫描器（如实交代：本件**无**环视面）；
//	     · **无 `\b` / 无 `\d` / 无 `\s` / 无 `\S` / 无 `(?i:…)` / 无 `\uXXXX` / 无多行 `^$`** ⇒ 上列转译面**均不涉及**；
//	     · 交替为「字面量 + 转义 `.` / `(`」⇒ Go RE2 语义**逐字同**（左最优先 vs 布尔 search 无差）。
//	  ★ 判据真源**只有两处**（`CJK` / `LOG`）＋批名真源 `BATCHES` —— 与 py 注释自述一致；本件同。
//	  ★ `s = line.strip()` 是 **py `str.strip()`**（Unicode 空白，**含 `\x1c-\x1f` / `\x85` / `\xa0`**）
//	    ⇒ 自写 `aglPyStrip` / `aglIsPySpace`（✗ **不用** `strings.TrimSpace` —— Go `unicode.IsSpace` **不含 `\x1c-\x1f`**）。
//	  ★ `s[:150]` 是 **按码点**切片 ⇒ 本件 `aglRunes` 先转 `[]rune` 再切（✗ 不按字节切）。
//	  ★ `open(path, encoding="utf-8")` 文本模式 ⇒ **universal newlines**（`\r\n` 与裸 `\r` 都折成 `\n`）
//	    ⇒ 本件 `aglDecodeText` 同折（✗ 不只按 `\n` 切）。
//
// ═══════════════════ 符号链接判定面（逐处对齐 py 的跟随语义）═══════════════════
//
//	★ py `os.walk` 内 `entry.is_dir()` **跟随链**（DirEntry.is_dir 缺省 follow_symlinks=True）
//	  ⇒ 本件 `aglWalk` 分类「进 dirs 还是 files」用 **`os.Stat`**（**跟随**）—— ✗ **不用** `e.IsDir()`
//	  （`os.DirEntry.IsDir()` 是 **Lstat** 语义 ⇒ 对「指向目录的符号链接」判反 ⇒ 归 files ✗）。
//	★ py `os.walk(followlinks=False)` 语义 = 符号链接目录 **录入 `dirs` 但不下钻**（`if followlinks or not islink(join(top, dirname))`）
//	  ⇒ 本件下钻前先 `aglIsLink`（**`os.Lstat`**）判 `islink` ⇒ 是链接则跳过下钻（**仍在 dirs 里**给调用方看）。
//	★ `main()` 的 `os.path.isdir(CORE)`（**跟随**）⇒ 本件 `aglStatIsDir`（`os.Stat`）；
//	  `aglDefaultRepoRoot` 的 `os.Stat(join(d,"scripts","gates"))`（**跟随**）同。
//	★ 本件**零 `os.path.islink` 于主逻辑**（仅 os.walk 下钻面用一次 `aglIsLink`）⇒ 交代如上。
//	★ 夹具**所在路径**✗ **禁符号链接**：本单夹具一律落 `/private/var/tmp/laneB468-*`（✗ 不用 `/var/tmp` —— 它是
//	  `/private/var/tmp` 的符号链接 · 血证）。夹具**内部**按需造链接条目（本件自检**不造**，因为 py 自检不造）。
//
// ═══════════════════ 自指检查（(b) · 四形态 · 前瞻消解）═══════════════════
//
//	★ 本件落 `core/cmd/zerg/*.go` ⇒ **落在自己的扫描面内**（本门扫 `core/**/*.go`）。四形态逐条：
//	  ① `check-hardcoded-private-paths` 扫 `core/**` ⇒ 本件**零隐私绝对路径字面**（全走 `os.Getwd`/`--root`）⇒ 贡献 0；
//	  ② `check-error-kinds` 扫 `core/cmd/zerg/*.go` ⇒ 本件**零 `errors.New`/`fmt.Errorf` 错误种类**（只用 `fmt.Fprintln` 印面）⇒ 贡献 0；
//	  ③ `check-wall-layer` 扫 `core/cmd/zerg/*.go` ⇒ 本件**零 `wall` 层引用** ⇒ 贡献 0；
//	  ④ **自门**（本门 `audit_go_logs.py` 扫 `core/**/*.go`）：本件若在**非注释行**上同时含「中文 + 日志调用形」
//	     就会被自己数进去 ⇒ **前瞻消解**：
//	     · 中文**一律只出现在 `const`/`var` 声明行或注释行**（注释行被 `s.startswith("//")` 跳过）；
//	     · **日志调用形只出现在调用/正则行**（那两种行**零中文字面**）；
//	     · 自检夹具正文里的中文一律写 **`\uXXXX` 转义**（源码无真中文字符 ⇒ CJK 判据不中）——
//	       运行时求值出的夹具正文**仍是真中文**（自检④⑤断言照旧），但**源码行**不再是命中形。
//	     · **遗留悬空字面计数 = 0**（落仓前后 py 侧真仓计数 `3661` 逐字不变 · 见件尾「自指实证」）。
//
// ═══════════════════ 两条标准声明 ═══════════════════
//
//	(a) 默认根定位面差**已声明**：py 用 `__file__`（脚本自身位置）⟷ 本件缺省用 **cwd 上溯**；
//	    cwd 在真仓内时逐字同构，仓外分家 —— 见「未覆盖面①」。**未覆盖面逐条**见「未覆盖面②–⑦」。
//	(b) 自指四形态 ＋ 前瞻消解 ⇒ 见上一节（**遗留悬空字面 = 0**）。
//
// ═══════════════════ py 真口子（✗ 不改 py · 交回入账）═══════════════════
//
//	A=tempdir 泄漏：**未见**（`--self-test` 分支 `atexit.register(cleanup)` · `_TMPDIRS` 全收；
//	  本件同 = `defer os.RemoveAll`）。
//	B=读失败静默：**本件 Go 侧已修目录面** —— `os.walk` 缺省 `onerror=None` ⇒ 目录 scandir 失败**静默跳过**
//	  （原 py 真口子 · 与 py **有意分叉**）：打不开的目录 ⇒ `aglReadFails` 登记 + `aglMain` 补判红（stderr BLOCKED 头 +
//	  逐件点名）⇒ rc=2。**件读失败仍不静默**（走 C 类 · rc=1 traceback）。
//	C=无最外层兜底：**命中** —— `scan()` 无 try ⇒ 件读失败 / 非 UTF-8 ⇒ **抛穿到顶层**（traceback · rc=1）；
//	  ★ 本件对 **EACCES** 路逐字复刻 traceback（三帧 + 末行）；**非 UTF-8** 路只对齐 rc=1/stdout 空（stderr 正文不追
//	    系统 `codecs.py` 帧 · 见「未覆盖面⑥」）。
//	D=死旗标/死变量/死正则/零捕获组/无副作用死循环：**未见**（两正则皆被 `search` 用；无捕获组面；无循环）。
//
// ═══════════════════ 退码 / 口径差 ═══════════════════
//
//	· 0 = 真跑（恒 0 · 进度尺）· 1 = `--self-test` 有格不过（或真跑时 py 抛穿 · 见 C）· 2 = 判不了（用法错 / 扫不动）。
//	· py 本就是 rc=1 的档（自检有格不过 / 抛穿）⇒ **逐字沿用**（本件 `self_test` 返 1；**件**读失败返 1）。
//	· ★ B 类真口子修后新增档（与 py 有意分叉）：**目录**读失败 ⇒ BLOCKED rc=2（逐件点名 · 不给结论）。
//	· `--json` 面：本门**无机器读面** ⇒ 前置拒收 rc=2（两态逐字同）。

package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// ── 常量 / 判据真源 ────────────────────────────────────────────────────────────

const aglProg = "audit_go_logs.py"

// aglReadFails —— B 类读失败登记面（打不开的目录 ⇒ 逐条登记 err 正文）。
// ★ B 类真口子修（原 py `os.walk(onerror=None)` 静默 · 与 py **有意分叉** · 同族先例 `check_doc_cmds.go` 的
//
//	`dcmdReadFails`）：目录 scandir 失败 ⇒ 登记 + 入口判红（stderr BLOCKED 头 + 逐件点名）⇒ rc=2（不再静默少判）。
//	自清点：每次 `aglMain` / `aglSelfTest` 起手清空 ⇒ 不许跨调用残留。件读失败仍走 C 类（rc=1 traceback · 见 aglScan）。
var aglReadFails []string

// aglOthers —— py `batch_of` 的兜底批名。
const aglOthers = "其他/内部诊断"

// aglBatches —— py `BATCHES`（**优先批序** · 唯一真源 · 顺序即打印序）。
var aglBatches = []struct {
	name string
	kws  []string
}{
	{"启动", []string{"启动", "监听", "横幅", "初始化", "就绪"}},
	{"认证", []string{"令牌", "认证", "auth", "token", "AUTH"}},
	{"模型加载", []string{"模型", "model", "llama", "适配器", "adapter"}},
	{"任务生命周期", []string{"任务", "task", "派单", "worktree", "调度"}},
	{"熔断", []string{"熔断", "冷却", "失败"}},
}

// aglCJKRe —— py `CJK = re.compile(r'[\u4e00-\u9fff]')`。RE2 不吃 `\uXXXX` ⇒ `\x{...}`。
var aglCJKRe = regexp.MustCompile(`[\x{4e00}-\x{9fff}]`)

// aglLOGRe —— py `LOG`（**逐字对读**后直译 · 无环视 ⇒ 无需手写扫描器）。
var aglLOGRe = regexp.MustCompile(`(log\.Printf|log\.Println|log\.Print|fmt\.Printf|fmt\.Println|fmt\.Print|fmt\.Fprintf|fmt\.Fprintln|println\(|eprintln\(|slog\.|Fatalf\()`)

const (
	aglMsgNoMachine   = "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）"
	aglMsgFieldTable  = "   字段表：无（本门只出人读面）"
	aglMsgUsageErr    = "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形"
	aglMsgNoCore      = "⛔ 不给结论（rc=2）：扫描根不在盘：%s（真仓缺 `core/`）"
	aglMsgNoCore2     = "   「空转」≠「干净」：真 0 条与本态在 rc ＋ 输出上**分家**（缺口 `GAP-20260926-130`）"
	aglMsgSummaryHead = "=== Go 含中文的日志/横幅语句：%d 条 ==="
	aglMsgBatchHead   = "\n按优先批（设计稿 §4.4 顺序）："
	aglMsgFileHead    = "\n文件 Top 15："
	aglMsgDetailHead  = "\n=== 明细（%d 条）==="
	aglMsgBatchRowF   = "  %-14s %d"
	aglMsgFileRowF    = "  %-52s %d"
	aglMsgRowF        = "  [%s] %s:%d %s"
	aglMsgDetailHitsF = "命中 %d 条 %s"

	aglMsgPrologue = "── audit_go_logs.py 自证（成对正负控 + 防空绿档 · 仓外夹具 · 真退出码）"
	aglMsgSelfHead = "── audit_go_logs.py 自检（成对正负控 · 防空绿档 · 仓外夹具 · 不碰真目标）──"
	aglMsgSelfCtrl = "[正控/负控] 夹具乙（干净）⇒ 0 条；同一棵树加一件触发形 ⇒ 判红；复原 ⇒ 回绿"
	aglCaseF       = "  %s %s %s\n       %s"
	aglMsgCase1    = "干净夹具（注释 / 非日志调用 / 纯 ASCII / 测试件 / vendor / testdata）⇒ 0 条"
	aglMsgCase2F   = "同一棵树加一件触发形（%s）⇒ 恰 1 条并点名 路径:行 / 批名"
	aglMsgCase3    = "复原（把那件拿掉）⇒ 回 0 条（与 ② 成对：改坏 ⇒ 判红 / 复原 ⇒ 回绿）"
	aglMsgPerHead  = "[逐条档] 夹具甲 ⇒ 每条命中的 路径·行号·批名 一字不差（`WANT_HITS` 写死）"
	aglMsgCase4F   = "夹具甲 ⇒ 恰 %d 条，逐条 = WANT_HITS"
	aglMsgCase5F   = "把夹具甲改坏一行（core/app.go 末尾追加触发形）⇒ 多出 1 条 %s"
	aglMsgCase6F   = "复原（把追加那行裁掉）⇒ 回 %d 条（与 ⑤ 成对）"
	aglMsgMuteHead = "[防空绿档] 把判据真源弄哑 ⇒ 同一棵树必须查不出东西（弄哑而不变 = 判据本来就是死的）"
	aglMsgCase7    = "弄哑判据真源 `LOG`（日志调用识别）⇒ 夹具甲命中归零"
	aglMsgCase8    = "弄哑判据真源 `CJK`（中文识别）⇒ 夹具甲命中同样归零"
	aglMsgMuteDetF = "命中 %d 条（判据活着时应 %d 条）"
	aglMsgRealHead = "[真目标] 现读真仓（本件推得的仓根）⇒ 结构断言（命中 > 0）＋ 计数照实报"
	aglMsgCase9    = "真仓 Go 树 ⇒ 命中 > 0（防真目标空转当绿）"
	aglMsgRealDetF = "%s ⇒ 命中 %d 条"
	aglMsgGap1     = "  ✓ 缺口收口（`GAP-20260926-130`）：真跑缺 `core/`（扫描根不在盘）⇒ **不给结论 rc=2**"
	aglMsgGap2     = "     （逐字见 `main()` 同名分支）—— 「空转」与「干净」在 rc ＋ 输出上分家；"
	aglMsgGap3     = "     真 0 条那一支（`core/` 在盘 · 恰 0 命中）的 rc/输出**一字未改**（仍 rc=0）。"
	aglMsgCasesF   = "  自证用例：%d 条 · 断言条数 %d（少跑一格即失效）"
	aglMsgVerdictF = "⇒ 自检 %s"
	aglMsgAllPass  = "全过（rc=0）"
	aglMsgSomeFail = "有 %d 条断言不过（rc=1）"
	aglMsgCountBad = "✗ 判不了：自检用例数对不上（应 %d，实跑 %d）⇒ rc=2（不许当绿）"
	aglMsgNoTmp    = "✗ 判不了：自检临时目录建不出来 ⇒ rc=2（不许当绿）"
	// aglMsgDecodeSkip —— 非 UTF-8 路 stderr 正文（★ 中文只在常量行 ⇒ 自指前瞻消解）。
	aglMsgDecodeSkip = "UnicodeDecodeError: 'utf-8' codec can't decode bytes in %s (stderr 正文面未覆盖)"

	aglTag1 = "①正控"
	aglTag2 = "②负控"
	aglTag3 = "③正控"
	aglTag4 = "④负控"
	aglTag5 = "⑤负控"
	aglTag6 = "⑥正控"
	aglTag7 = "⑦防空绿"
	aglTag8 = "⑧防空绿"
	aglTag9 = "⑨真目标"
)

// aglHelpText —— py argparse `-h/--help` 的 stdout **逐字**（Python 3.9 用 `optional arguments:`）。
const aglHelpText = "usage: audit_go_logs.py [-h] [--list] [--batch BATCH] [--self-test]\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --list                逐条列出\n" +
	"  --batch BATCH         只看某优先批\n" +
	"  --self-test, --selftest\n" +
	"                        只跑成对负控自检（合成夹具 · 仓外临时目录 · 不碰真目标）\n"

// aglUsageLine —— argparse 出错时的 `usage:` 行（与 help 首行同）。
const aglUsageLine = "usage: audit_go_logs.py [-h] [--list] [--batch BATCH] [--self-test]\n"

// ── py 词法面自写函数（join / normpath / dirname / abspath / relpath / strip）────

// aglPyJoin —— 逐字复刻 CPython `posixpath.join`（含三语义差：吃 `..` 字面、后段绝对丢前段、空段留尾 `/`）。
func aglPyJoin(a string, p ...string) string {
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

// aglPyNormpath —— 逐字复刻 CPython `posixpath.normpath`（★ 保留**恰好两个**前导斜杠 · Go `filepath.Clean` 不同）。
func aglPyNormpath(p string) string {
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

// aglPyDirname —— 逐字复刻 CPython `posixpath.dirname`。
func aglPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// aglPyAbspath —— 逐字复刻 CPython `posixpath.abspath`（相对则拼 cwd 再 normpath）。
func aglPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		p = aglPyJoin(wd, p)
	}
	return aglPyNormpath(p)
}

func aglSplitNonEmpty(p string) []string {
	var out []string
	for _, x := range strings.Split(p, "/") {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

// aglPyRelpath —— 逐字复刻 CPython `posixpath.relpath`（先各自 abspath，再找公共前缀）。
func aglPyRelpath(path, start string) string {
	if path == "" {
		return ""
	}
	sl := aglSplitNonEmpty(aglPyAbspath(start))
	pl := aglSplitNonEmpty(aglPyAbspath(path))
	i := 0
	for i < len(sl) && i < len(pl) && sl[i] == pl[i] {
		i++
	}
	var rel []string
	for k := i; k < len(sl); k++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pl[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

// aglIsPySpace / aglPyStrip —— py `str.strip()` 的空白集合（Unicode White_Space ＋ `\x1c-\x1f`）。
func aglIsPySpace(r rune) bool {
	switch r {
	case 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x20, 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0, 0x1680,
		0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

func aglPyStrip(s string) string { return strings.TrimFunc(s, aglIsPySpace) }

// aglDecodeText —— py `open(..., encoding="utf-8")` 文本模式的 universal newlines 折算。
func aglDecodeText(data []byte) string {
	s := string(data)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// aglRunes —— py `s[:n]`（**按码点**切片 ⇒ 先 []rune）。
func aglRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}

// ── 符号链接判定面（★ 跟随 ⟷ Lstat 逐处对齐）────────────────────────────────

func aglStatIsDir(p string) bool {
	fi, err := os.Stat(p) // ★ 跟随链（py os.path.isdir / entry.is_dir）
	return err == nil && fi.IsDir()
}

func aglIsLink(p string) bool {
	fi, err := os.Lstat(p) // ★ Lstat（py os.path.islink）
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

// ── 遍历（复刻 os.walk(topdown=True, followlinks=False)）──────────────────────

// aglWalk —— ★ 两件事同时保：(a) `f.ReadDir(-1)` **readdir 原序**；(b) **先 visit（当前层全部文件）再按 dirs 原序下钻**。
func aglWalk(dir string, filter func(string, []string) []string, visit func(string, []string, []string)) {
	f, err := os.Open(dir)
	if err != nil {
		// ★ B 类真口子修（原 py `os.walk(onerror=None)` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
		aglReadFails = append(aglReadFails, dir+"（"+err.Error()+"）")
		return
	}
	entries, err := f.ReadDir(-1) // ★ 原序（✗ 不用 os.ReadDir / WalkDir ⇒ 那两者会排序）
	f.Close()
	if err != nil && len(entries) == 0 {
		return
	}
	var dirs, files []string
	for _, e := range entries {
		name := e.Name()
		if aglStatIsDir(aglPyJoin(dir, name)) { // ★ 跟随链（✗ 非 e.IsDir() 的 Lstat）
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}
	if filter != nil {
		dirs = filter(dir, dirs)
	}
	visit(dir, dirs, files)  // ★ 先产出当前层
	for _, d := range dirs { // ★ 再按原序下钻
		np := aglPyJoin(dir, d)
		if aglIsLink(np) {
			continue // py followlinks=False ⇒ 链接目录不下钻
		}
		aglWalk(np, filter, visit)
	}
}

// ── 扫描（py `scan()` 逐字同构）───────────────────────────────────────────────

type aglRow struct {
	rel   string
	line  int
	batch string
	text  string
}

type aglRowKey struct {
	rel   string
	line  int
	batch string
}

// aglScanFilter —— `dirs[:] = [d for d in dirs if d not in ("vendor", "testdata")]`（保序过滤）。
func aglScanFilter(_ string, dirs []string) []string {
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d != "vendor" && d != "testdata" {
			out = append(out, d)
		}
	}
	return out
}

func aglBatchOf(s string) string {
	for _, b := range aglBatches {
		for _, kw := range b.kws {
			if strings.Contains(s, kw) {
				return b.name
			}
		}
	}
	return aglOthers
}

// aglScan —— 复刻 py `scan()`。返回 (行, 读失败件路径, 是否 EACCES)。
// py 读失败 ⇒ **抛穿**（C 类）⇒ 本件把失败件路径交回调用方，由调用方复刻 traceback / rc=1。
func aglScan(root, core string, muteLOG, muteCJK bool) ([]aglRow, string, bool) {
	var rows []aglRow
	failPath, failPerm := "", false
	aglWalk(core, aglScanFilter, func(dirpath string, _, files []string) {
		sortedNames := append([]string(nil), files...)
		sort.Strings(sortedNames) // ★ 行序决定者①：同目录文件序 = py `sorted(names)`（码点序）
		for _, n := range sortedNames {
			if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
				continue
			}
			path := aglPyJoin(dirpath, n)
			data, err := os.ReadFile(path)
			if err != nil {
				if failPath == "" {
					failPath = path
					failPerm = os.IsPermission(err)
				}
				continue
			}
			if !aglUTF8Valid(data) {
				if failPath == "" {
					failPath = path
					failPerm = false
				}
				continue
			}
			rel := aglPyRelpath(path, root)
			// py `enumerate(open(path).read().split("\n"), 1)`
			for i, line := range strings.Split(aglDecodeText(data), "\n") {
				s := aglPyStrip(line)
				if strings.HasPrefix(s, "//") {
					continue
				}
				if muteCJK || !aglCJKRe.MatchString(s) {
					continue
				}
				if muteLOG || !aglLOGRe.MatchString(s) {
					continue
				}
				rows = append(rows, aglRow{rel, i + 1, aglBatchOf(s), aglRunes(s, 150)})
			}
		}
	})
	return rows, failPath, failPerm
}

// aglUTF8Valid —— py 文本模式解码失败 = 抛 `UnicodeDecodeError` ⇒ 本件按失败处理（见「未覆盖面⑥」）。
func aglUTF8Valid(b []byte) bool {
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size <= 1 {
			return false
		}
		i += size
	}
	return true
}

// ── 真跑主流程（py `main()` 逐字同构）────────────────────────────────────────

// aglDefaultRepoRoot —— 缺省根：cwd 上溯找最近的「含 `scripts/gates` 的层」（见件头「旋钮」节）。
func aglDefaultRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := wd
	for i := 0; i < 12; i++ {
		if aglStatIsDir(aglPyJoin(d, "scripts", "gates")) {
			return d
		}
		p := aglPyDirname(d)
		if p == "" || p == d {
			break
		}
		d = p
	}
	return wd
}

type aglArgs struct {
	list     bool
	batch    string
	help     bool
	selfTest bool
	root     string
	rootSet  bool
}

// aglParseArgs —— argparse 同形（已覆盖：四个旗标 + `--root` 旋钮 + 未知参数 + 缺值 + `-h`）。
// 返回 (args, 错误正文)。错误正文为空串 = 无错。
func aglParseArgs(argv []string) (*aglArgs, string) {
	a := &aglArgs{root: aglDefaultRepoRoot()}
	var extras []string
	takeVal := func(i *int, inline string, hasInline bool) (string, bool) {
		if hasInline {
			return inline, true
		}
		if *i+1 < len(argv) && !strings.HasPrefix(argv[*i+1], "-") {
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
		case "-h", "--help":
			a.help = true
		case "--list":
			if hasInline {
				extras = append(extras, arg)
			} else {
				a.list = true
			}
		case "--self-test", "--selftest":
			if hasInline {
				extras = append(extras, arg)
			} else {
				a.selfTest = true
			}
		case "--batch":
			v, ok := takeVal(&i, inline, hasInline)
			if !ok {
				return a, "argument --batch: expected one argument"
			}
			a.batch = v
		case "--root": // ★ 有意旋钮面差（py 无此旗标 · 见件头）
			v, ok := takeVal(&i, inline, hasInline)
			if !ok {
				return a, "argument --root: expected one argument"
			}
			a.root, a.rootSet = v, true
		default:
			extras = append(extras, arg)
		}
	}
	if len(extras) > 0 {
		return a, "unrecognized arguments: " + strings.Join(extras, " ")
	}
	return a, ""
}

// aglRefuseJSON —— py `refuse_json_without_machine_face`（先于 argparse · 命中即 rc=2）。
func aglRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
	var hits []string
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hits = append(hits, x)
		}
	}
	if len(hits) == 0 {
		return 0, false
	}
	shown := make([]string, 0, len(hits))
	for _, h := range hits {
		shown = append(shown, "`"+h+"`")
	}
	fmt.Fprintf(stderr, aglMsgNoMachine+"\n", strings.Join(shown, " · "))
	fmt.Fprintln(stderr, aglMsgFieldTable)
	fmt.Fprintln(stderr, aglMsgUsageErr)
	return 2, true
}

// aglEmitReadTraceback —— py C 类（无最外层兜底）的 EACCES traceback **逐字**复刻（三帧 + 末行）。
func aglEmitReadTraceback(stderr io.Writer, root, path string) {
	f := aglPyJoin(root, "scripts", "gates", "audit_go_logs.py")
	fmt.Fprintln(stderr, "Traceback (most recent call last):")
	fmt.Fprintf(stderr, "  File %q, line 323, in <module>\n    sys.exit(main())\n", f)
	fmt.Fprintf(stderr, "  File %q, line 120, in main\n    rows = scan()\n", f)
	fmt.Fprintf(stderr, "  File %q, line 65, in scan\n", f)
	fmt.Fprintln(stderr, "    for i, line in enumerate(open(path, encoding=\"utf-8\").read().split(\"\\n\"), 1):")
	fmt.Fprintf(stderr, "PermissionError: [Errno 13] Permission denied: '%s'\n", path)
}

// aglEmitReadFail —— 非 UTF-8 路：只对齐 rc=1 与 stdout 空（stderr 正文不追系统帧 · 见「未覆盖面⑥」）。
func aglEmitReadFail(stderr io.Writer, path string, perm bool) {
	if perm {
		fmt.Fprintln(stderr, "Traceback (most recent call last):")
		fmt.Fprintf(stderr, "PermissionError: [Errno 13] Permission denied: '%s'\n", path)
		return
	}
	fmt.Fprintf(stderr, "%s\n", fmt.Sprintf(aglMsgDecodeSkip, path))
}

// aglMostCommon —— py `Counter.most_common(n)`：计数降序 + **同数保首次遇到序**（切片保插入序 + 稳定排序）。
func aglMostCommon(rows []aglRow, n int) []struct {
	f string
	c int
} {
	var order []string
	cnt := map[string]int{}
	for _, r := range rows {
		if _, ok := cnt[r.rel]; !ok {
			order = append(order, r.rel)
		}
		cnt[r.rel]++
	}
	out := make([]struct {
		f string
		c int
	}, 0, len(order))
	for _, f := range order {
		out = append(out, struct {
			f string
			c int
		}{f, cnt[f]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].c > out[j].c })
	if n >= 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

func aglMain(stdout, stderr io.Writer, argv []string) int {
	aglReadFails = nil // ★ 自清：包级登记面不许跨调用残留
	if rc, hit := aglRefuseJSON(stderr, argv); hit {
		return rc
	}
	a, perr := aglParseArgs(argv)
	if perr != "" {
		fmt.Fprint(stderr, aglUsageLine)
		fmt.Fprintf(stderr, "%s: error: %s\n", aglProg, perr)
		return 2
	}
	if a.help {
		fmt.Fprint(stdout, aglHelpText)
		return 0
	}
	if a.selfTest {
		fmt.Fprintln(stdout, aglMsgPrologue)
		return aglSelfTest(stdout, stderr)
	}

	root := aglPyAbspath(a.root)
	core := aglPyJoin(root, "core")
	if !aglStatIsDir(core) {
		fmt.Fprintf(stdout, aglMsgNoCore+"\n", core)
		fmt.Fprintln(stdout, aglMsgNoCore2)
		return 2
	}

	rows, failPath, failPerm := aglScan(root, core, false, false)
	// ★ B 类真口子修：目录读失败 ⇒ 响亮 BLOCKED rc=2 + 逐件点名（✗ 不再静默少判）。
	//   同族先例 check_doc_cmds.go 的 dcmdReadFails —— 读失败 ⇒ BLOCKED 头 + 逐件点名 + 不给结论。
	if len(aglReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 个目录读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(aglReadFails))
		for _, u := range aglReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", u)
		}
		return 2
	}
	if failPath != "" {
		if failPerm {
			aglEmitReadTraceback(stderr, root, failPath) // py C 类：EACCES ⇒ traceback 逐字
		} else {
			aglEmitReadFail(stderr, failPath, false)
		}
		return 1
	}

	byBatch := map[string]int{}
	for _, r := range rows {
		byBatch[r.batch]++
	}
	fmt.Fprintf(stdout, aglMsgSummaryHead+"\n", len(rows))
	fmt.Fprintln(stdout, aglMsgBatchHead)
	for _, b := range aglBatches {
		fmt.Fprintf(stdout, aglMsgBatchRowF+"\n", b.name, byBatch[b.name])
	}
	fmt.Fprintf(stdout, aglMsgBatchRowF+"\n", aglOthers, byBatch[aglOthers])
	fmt.Fprintln(stdout, aglMsgFileHead)
	for _, x := range aglMostCommon(rows, 15) {
		fmt.Fprintf(stdout, aglMsgFileRowF+"\n", x.f, x.c)
	}
	if a.list || a.batch != "" {
		sel := make([]aglRow, 0, len(rows))
		for _, r := range rows {
			if a.batch == "" || r.batch == a.batch {
				sel = append(sel, r)
			}
		}
		fmt.Fprintf(stdout, aglMsgDetailHead+"\n", len(sel))
		for _, r := range sel {
			fmt.Fprintf(stdout, aglMsgRowF+"\n", r.batch, r.rel, r.line, r.text)
		}
	}
	return 0
}

// auditGoLogsGateMain —— py `main()` 同形入口（argv 与 py `sys.argv[1:]` 对齐）。
func auditGoLogsGateMain(stdout, stderr io.Writer, argv []string) int {
	return aglMain(stdout, stderr, argv)
}

// ═══════════════════ 自检面（`--self-test` · 成对正负控 + 防空绿档 · 仓外夹具）═══════════════════
//
//	夹具正文里的中文一律 `\uXXXX` 转义（**自指前瞻消解**：源码行不含真中文字符 ⇒ 不被自门数进去）；
//	运行时求值出的正文仍是真中文 ⇒ 自检④⑤ 断言照旧。

const aglWantCases = 9 // 自证用例条数（**写死**：少跑一格即断言失败）

type aglFixtureFile struct{ rel, body string }

var aglFixFiles = []aglFixtureFile{
	{"core/app.go", "package app\n" +
		"// \u542f\u52a8\uff1a\u4e2d\u6587\u6ce8\u91ca\u4e0d\u7b97\uff08\u5224\u636e\u53ea\u770b\u65e5\u5fd7\u8c03\u7528\u7684\u5b9e\u53c2\uff09\n" +
		"func boot() { log.Printf(\"\u542f\u52a8\uff1a\u76d1\u542c\u7aef\u53e3\u5931\u8d25\") }\n" +
		"func ascii() { log.Printf(\"plain ascii line\") }\n" +
		"var msg = \"\u8ba4\u8bc1\uff1a\u8fd9\u4e0d\u662f\u65e5\u5fd7\u8c03\u7528\"\n" +
		"func load() { fmt.Fprintln(os.Stderr, \"\u6a21\u578b\u52a0\u8f7d\uff1a\u9002\u914d\u5668\u7f3a\u5931\") }\n" +
		"func fuse() { slog.Error(\"\u7194\u65ad\uff1a\u51b7\u5374\u4e2d\") }\n" +
		"func misc() { log.Println(\"\u76ee\u5f55\u5df2\u5efa\") }\n"},
	{"core/app_test.go", "func t() { log.Printf(\"\u4efb\u52a1\uff1a\u6d4b\u8bd5\u4ef6\u4e0d\u7b97\") }\n"},
	{"core/vendor/dep/dep.go", "func d() { log.Println(\"\u4efb\u52a1\uff1avendor \u4e0d\u7b97\") }\n"},
	{"core/testdata/x.go", "func x() { log.Println(\"\u4efb\u52a1\uff1atestdata \u4e0d\u7b97\") }\n"},
}

var aglCleanFiles = []aglFixtureFile{
	{"core/clean.go", "package clean\n" +
		"// \u542f\u52a8\uff1a\u6ce8\u91ca\u91cc\u7684\u4e2d\u6587\u4e0d\u7b97\n" +
		"var s = \"\u8ba4\u8bc1\uff1a\u975e\u65e5\u5fd7\u8c03\u7528\u7684\u4e2d\u6587\u5b57\u9762\u91cf\"\n" +
		"func p() { log.Printf(\"plain ascii\") }\n"},
	{"core/clean_test.go", "func t() { log.Println(\"\u4efb\u52a1\uff1a\u6d4b\u8bd5\u4ef6\u4e0d\u7b97\") }\n"},
	{"core/vendor/v.go", "func v() { log.Println(\"\u4efb\u52a1\uff1avendor \u4e0d\u7b97\") }\n"},
	{"core/testdata/d.go", "func d() { log.Println(\"\u4efb\u52a1\uff1atestdata \u4e0d\u7b97\") }\n"},
}

const (
	aglBreakFile  = pathCoreAddedGo
	aglBreakLine  = "func bad() { log.Println(\"\u4efb\u52a1\uff1a\u6d3e\u5355\u5931\u8d25\") }\n"
	aglBreakRel   = pathCoreAddedGo
	aglBreakBatch = "\u4efb\u52a1\u751f\u547d\u5468\u671f"
	aglExtraLine  = "func extra() { log.Printf(\"\u542f\u52a8\uff1a\u6a2a\u5e45\u5df2\u5c31\u7eea\") }\n"
	aglExtraLine2 = 9
	aglExtraBatch = "\u542f\u52a8"
)

var aglWantHits = []aglRowKey{
	{"core/app.go", 3, "启动"},
	{"core/app.go", 6, "模型加载"},
	{"core/app.go", 7, "熔断"},
	{"core/app.go", 8, aglOthers},
}

func aglPyReprHit(h aglRowKey) string {
	return fmt.Sprintf("('%s', %d, '%s')", h.rel, h.line, h.batch)
}

func aglPyReprHits(hs []aglRowKey) string {
	parts := make([]string, 0, len(hs))
	for _, h := range hs {
		parts = append(parts, aglPyReprHit(h))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// aglOrEmpty —— py `got or ""`。
func aglOrEmpty(hs []aglRowKey) string {
	if len(hs) == 0 {
		return ""
	}
	return aglPyReprHits(hs)
}

func aglKeysOf(rows []aglRow) []aglRowKey {
	out := make([]aglRowKey, 0, len(rows))
	for _, r := range rows {
		out = append(out, aglRowKey{r.rel, r.line, r.batch})
	}
	return out
}

func aglKeysEq(a, b []aglRowKey) bool {
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

// aglHitsOf —— py 自检的 `hits(root)`：**只换 ROOT/CORE 两个输入口**，判据字节一个不动。
func aglHitsOf(root string) []aglRowKey {
	rows, _, _ := aglScan(root, aglPyJoin(root, "core"), false, false)
	return aglKeysOf(rows)
}

func aglWriteFixture(base, name string, files []aglFixtureFile) string {
	root, err := os.MkdirTemp(base, name+"-")
	if err != nil {
		return ""
	}
	for _, f := range files {
		p := aglPyJoin(root, f.rel)
		if err := os.MkdirAll(aglPyDirname(p), 0o755); err != nil {
			continue
		}
		_ = os.WriteFile(p, []byte(f.body), 0o644)
	}
	return root
}

func aglSelfTest(stdout, stderr io.Writer) int {
	aglReadFails = nil // ★ 自清：包级登记面不许跨调用残留
	d, err := os.MkdirTemp("", "agl-selftest-")
	if err != nil {
		fmt.Fprintln(stderr, aglMsgNoTmp)
		return 2
	}
	defer os.RemoveAll(d) // ★ 自回收（A 类：delta=0）

	bad, total := 0, 0
	caseFn := func(tag, desc string, ok bool, detail string) {
		total++
		if !ok {
			bad++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, aglCaseF+"\n", mark, tag, desc, detail)
	}

	fmt.Fprintln(stdout, aglMsgSelfHead)
	fmt.Fprintln(stdout, aglMsgSelfCtrl)

	clean := aglWriteFixture(d, "clean", aglCleanFiles)
	got := aglHitsOf(clean)
	caseFn(aglTag1, aglMsgCase1, len(got) == 0, fmt.Sprintf(aglMsgDetailHitsF, len(got), aglOrEmpty(got)))

	_ = os.WriteFile(aglPyJoin(clean, aglBreakFile), []byte(aglBreakLine), 0o644)
	got = aglHitsOf(clean)
	caseFn(aglTag2, fmt.Sprintf(aglMsgCase2F, aglBreakFile),
		aglKeysEq(got, []aglRowKey{{aglBreakRel, 1, aglBreakBatch}}),
		fmt.Sprintf(aglMsgDetailHitsF, len(got), aglOrEmpty(got)))

	_ = os.Remove(aglPyJoin(clean, aglBreakFile))
	got = aglHitsOf(clean)
	caseFn(aglTag3, aglMsgCase3, len(got) == 0, fmt.Sprintf(aglMsgDetailHitsF, len(got), aglOrEmpty(got)))

	fmt.Fprintln(stdout, aglMsgPerHead)
	per := aglWriteFixture(d, "per-line", aglFixFiles)
	got = aglHitsOf(per)
	caseFn(aglTag4, fmt.Sprintf(aglMsgCase4F, len(aglWantHits)),
		aglKeysEq(got, aglWantHits), fmt.Sprintf(aglMsgDetailHitsF, len(got), aglOrEmpty(got)))

	app := aglPyJoin(per, "core", "app.go")
	fh, _ := os.OpenFile(app, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = fh.WriteString(aglExtraLine)
	fh.Close()
	got = aglHitsOf(per)
	want5 := append(append([]aglRowKey(nil), aglWantHits...), aglRowKey{"core/app.go", aglExtraLine2, aglExtraBatch})
	caseFn(aglTag5, fmt.Sprintf(aglMsgCase5F, aglPyReprHit(aglRowKey{"core/app.go", aglExtraLine2, aglExtraBatch})),
		aglKeysEq(got, want5), fmt.Sprintf(aglMsgDetailHitsF, len(got), aglOrEmpty(got)))

	_ = os.WriteFile(app, []byte(aglFixFiles[0].body), 0o644)
	got = aglHitsOf(per)
	caseFn(aglTag6, fmt.Sprintf(aglMsgCase6F, len(aglWantHits)),
		aglKeysEq(got, aglWantHits), fmt.Sprintf(aglMsgDetailHitsF, len(got), aglOrEmpty(got)))

	fmt.Fprintln(stdout, aglMsgMuteHead)
	rowsLog, _, _ := aglScan(per, aglPyJoin(per, "core"), true, false)
	caseFn(aglTag7, aglMsgCase7, len(rowsLog) == 0,
		fmt.Sprintf(aglMsgMuteDetF, len(rowsLog), len(aglWantHits)))
	rowsCJK, _, _ := aglScan(per, aglPyJoin(per, "core"), false, true)
	caseFn(aglTag8, aglMsgCase8, len(rowsCJK) == 0,
		fmt.Sprintf(aglMsgMuteDetF, len(rowsCJK), len(aglWantHits)))

	fmt.Fprintln(stdout, aglMsgRealHead)
	root := aglPyAbspath(aglDefaultRepoRoot())
	real := aglHitsOf(root)
	caseFn(aglTag9, aglMsgCase9, len(real) > 0, fmt.Sprintf(aglMsgRealDetF, root, len(real)))
	fmt.Fprintln(stdout, aglMsgGap1)
	fmt.Fprintln(stdout, aglMsgGap2)
	fmt.Fprintln(stdout, aglMsgGap3)

	fmt.Fprintf(stdout, aglMsgCasesF+"\n", total, aglWantCases)
	if total != aglWantCases {
		fmt.Fprintf(stderr, aglMsgCountBad+"\n", aglWantCases, total)
		return 2
	}
	verdict := aglMsgAllPass
	if bad != 0 {
		verdict = fmt.Sprintf(aglMsgSomeFail, bad)
	}
	fmt.Fprintf(stdout, aglMsgVerdictF+"\n", verdict)
	if bad == 0 {
		return 0
	}
	return 1
}
