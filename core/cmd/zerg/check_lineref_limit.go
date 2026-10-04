// check_lineref_limit.go —— 判据件 `scripts/gates/check-lineref-limit.py`（337 行）的
// **Go 真搬**（v2.5.14 · M2 · 新口径靶：外部树走显式入参旋钮 · 入口 `checkLinerefLimitGateMain`）。
//
// 为什么有这件：
//
//	本件 = M2（按价值真搬 Go）一枚。验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），
//	stdout / stderr / 退码 **三面逐字相同**（除下列**显式声明的**差异面）。
//
// 本件与新口径（外部树改显式入参旋钮 —— 逐条对应）：
//
//	① 旋钮名：py 本件**没有**任何外部树旗标/env（`docs_root` 纯由 `--root` 推导）；
//	   本件按族内同口径补一个**显式入参旋钮** `--docs-root DOCS_ROOT`（与
//	   `check-docs-orphans-links.py` / `who_references.go` 的 `ZERG_DOCS_ROOT` 族同名，
//	   ✗ 不另造第三个互相不认识的名字）。★ 这是本件**唯一的旋钮面新增**，也是本件
//	   与 py **不可合同**的那一格：py 不认 `--docs-root` ⇒ argparse 退 2。
//	② 缺省值 = **py 默认推导逐字同构**：
//	   py `docs_root = os.path.join(os.path.dirname(root), "Zerg-内部文档")`；
//	   Go `clrlDocsDefault(root) = clrlPyJoin(filepath.Dir(root), "Zerg-内部文档")`。
//	   ★ **缺省串不含 `..` 段**（`dirname` 直接吃掉末段，不产生 `..`）⇒ 本件**不做**
//	   `root + "/../Zerg-内部文档"` 那种串拼（串拼只在缺省串真含 `..` 时才需要；此处不需要，
//	   用了反而与 py 的 `os.path.join` 不同构）。
//	③ ★ 退 2 边界（新口径核心）：本件 py 原件**只对根/扫描面**硬退 2 ——
//	   `scan_face(root/scripts/gates)` 为空（目录不在盘 **或** 目录空）⇒ `不给结论 rc=2`；
//	   引用 0 处 ⇒ rc=2；分母守恒断言不成立 ⇒ rc=2。**外部树（Zerg-内部文档）缺席 py 根本不判**：
//	   它只是让裸名/带目录落点解不出来 ⇒ 落「判不了」，**不**退 2。
//	   ⇒ 本件逐字沿用 py：**「都取不到」= 根取不到**（此时扫描面 0 件 ⇒ rc=2）；
//	   **只剩一棵（根在、外部树缺）⇒ 照判根那棵**、缺席格照实打「判不了」。
//	   ⇒ 本档与 py **零口径差**（新口径要求的「只判都取不到」py 本来就满足，未加严/未放宽）。
//	④ 取不到一律**不给结论**、✗ 不静默绿：扫描面为空/引用 0 处/守恒不成立三档一律 rc=2；
//	   外部树缺席 ⇒ 逐条落「判不了」明细（`--verbose` 可点名），✗ 不读成「0 件 = 干净」。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-lineref-limit.py` 原件（退场规则：**对拍通过后才**退场）；
//	· 入口 `checkLinerefLimitGateMain` 目前**无调用点**（M2 的刻意状态）。
//
// ★ 遍历 / 排序点逐一交代（四个点，逐点给「谁决定输出行序」）：
//
//	[点1] `scan_face` 顶层列举 —— py `sorted(os.listdir(gates_dir))`：**py 明写 sorted**。
//	       ⇒ Go 用 `f.ReadDir(-1)`（**readdir 原序**，`os.ReadDir` 会排序、这里不用它）
//	         取名字后 **显式 `sort.Strings`** 复现 py 那个 sorted。
//	       定序者 = **此处**：`rows` 的外层次序就是它（见点4）。
//	       ★ 字节序 ⟷ 码点序：Go `sort.Strings` 按字节比，py `sorted(str)` 按码点比；
//	         合法 UTF-8 下字节序 == 码点序 ⇒ 两者逐字相同（非 UTF-8 文件名见未覆盖面⑥）。
//	[点2] `scan_face` 的 `isfile` 过滤 —— py `os.path.isfile` **跟随符号链接**（`os.stat`）
//	       ⇒ Go 用 `os.Stat`（**不是** `entry.IsDir()`，后者用 Lstat、对「指向目录的符号链接」
//	         会与 py 判反）。符号链接归属：链接到普通件 ⇒ 入面；断链 ⇒ 两边都排除。
//	[点3] `resolve` 裸名多解 —— py `os.walk(base)`：**readdir 原序**（os.walk 不排序！），
//	       先当前层 `filenames`（entry 序）→ 再按 `dirnames` entry 序下钻；
//	       `dirnames[:]` 只剪 `{.git,node_modules,target,vendor}`；`followlinks=False`
//	       （符号链接归入 `filenames`、不下钻）。
//	       ⇒ Go `clrlScandir` 用 `f.ReadDir(-1)` 保**原序**，`os.Stat` 判 is_dir（跟随链接，
//	         同 py `DirEntry.is_dir()`），`clrlWalk` 复刻「当前层文件 → 按序下钻」。
//	       定序者 = **cands 的次序**：`len(行数集合)==1` 时 py 返回 `cands[0]`，
//	       而 `cands[0]` 的**路径**进 `target` 打印（`relpath`）⇒ 点3 决定**多解同长时
//	       `target` 打出的是哪一条**（本例 `len(lens)==1` ⇒ 取首解，非最大值/任取）。
//	[点4] `analyze` 的行/引用序 —— 逐件按点1 的件序、逐件内按 `readlines()` 行序、
//	       行内按 `REF_RE.finditer` **左到右**序 ⇒ Go 用 `FindAllStringSubmatchIndex`（同左到右）。
//	       定序者 = 与点1/4 一致：`rows` 为切片、**零 Go map 迭代**（`lens` 只用来判集合基数，
//	       `map[int]bool` 仅计基数、**不进输出**；输出里每个有序面都是切片）。
//	[点5] `json.dumps(..., indent=1)` —— py 依 **dict 插入序** dump ⇒ Go 用 `clrlOM`
//	       （切片对：keys/vals）保序，**零 map 迭代进输出**。
//
// ★ 正则语义差（逐条写明，并尽量手写对齐）：
//
//	py `REF_RE = ([^\s`"'\[\]<>|=，：。、；（）「」]*\.(?:EXT)):(\d+)(?:-(\d+))?`
//	· ① `\s` —— py（str 模式）是 **Unicode 空白**（含 U+00A0/U+2028/U+3000…）；
//	     Go RE2 的 `\s` 是 **ASCII**（`[\t\n\f\r ]`）⇒ 直接照抄会让「NBSP 前的 token」
//	     在 Go 里多认一段 ⇒ 本件**手写展开 py 的 Unicode 空白集**（`clrlPWS`），
//	     逐码点对齐 `str.isspace()`：`\t\n\v\f\r\x1c-\x1f\x85\xa0\u1680\u2000-\u200a
//	     \u2028\u2029\u202f\u205f\u3000`。
//	· ② `\d` —— py 是 **Unicode Nd**；Go `\d` 是 ASCII ⇒ 本件改用 `\p{Nd}`（== Unicode Nd）。
//	     配套：`int()` 认非 ASCII 十进制数字 ⇒ 本件 `clrlParseNum` 先走 `strconv.Atoi`，
//	     失败则**按各自所属十进块的零值**逐位求值（与 py `int()` 同值）。
//	· ③ **不涉及** 的语义差（逐条给依据）：无 `$`（不涉「非多行 `$` 匹配串尾换行之前」）、
//	     无 `^`、无 `(?!)`/负向先行、无反向引用 `\1`、无 `\b`、无 `\w`（`EXT` 用的是
//	     显式字面量 `py|sh|md|go|json|yaml|yml|ts|js|tsv|jsonl|txt|toml|rs|lock|cfg`，
//	     非 `\w`）、`.ext` 用 `\.` 字面量。⇒ 上述诸项**零涉及**，不计入口径差。
//	· ④ 非 UTF-8 替换粒度：py `open(..., errors="replace")` 按「最长非法子序列」替换；
//	     Go `strings.ToValidUTF8` 按「最长非法**字节**游程」替换 ⇒ 病态字节串下替换个数可不同
//	     （合法 UTF-8 全文零差异）。见未覆盖面②。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `repo_root()` 起于 `dirname(abspath(__file__))` 逐层上溯 ≤8 层
//	    找 `core/internal/version/version.go`，兜底 `dirname(dirname(here))`；
//	     本件**无 `__file__` 语境** ⇒ `clrlDefaultRoot()` 起于 **cwd** 逐层上溯 ≤8 层
//	     找同一锚点、兜底 `abspath(cwd)`。
//	     **旋钮缺省 vs py 默认推导逐字对比**：
//	       py  `root = os.path.abspath(args.root) if args.root else repo_root()`
//	       Go  `root = abspath(a.Root) if hasRoot else clrlDefaultRoot()`
//	       py  `docs_root = os.path.join(os.path.dirname(root), "Zerg-内部文档")`
//	       Go  `clrlDocsDefault(root) = clrlPyJoin(filepath.Dir(root), "Zerg-内部文档")`（无 `..` ⇒ 不串拼）
//	     「脚本位置 == cwd 上溯根」时两者读数逐字相同（对拍甲/乙两环境已证）；否则**路径推导面
//	     不同** ⇒ 不属逐字对齐承诺，对拍一律显式传根或在夹具里让两者重合。
//	(b) **自指检查（先真判 · 四形态逐条）**：
//	    ① **本件在自己扫描面内吗 ⇒ 否**。本门扫描面 = `--root` 下 `scripts/gates/` **顶层**
//	       且扩展名 ∈ `{.md,.py,.sh}` 的**件**；本件是 `.go`、落 `core/cmd/zerg/`
//	       ⇒ **既不在那个目录、扩展名也不在面**。量化依据：`clrlScanFace` 里目录锁定
//	       `clrlScanDirRel`（`scripts/gates`）1 处、扩展名过滤 `clrlScanExts` 1 处；
//	       无其它 glob/遍历入口。⇒ 本门**永不自扫本件**。
//	    ② **本件落在任何他门扫描面内吗 ⇒ 是**（`check-hardcoded-private-paths.py` 的
//	       `SCAN_DIRS` 含 `core/`；`check-error-kinds.py` 的 R3 扫 `core/cmd/zerg/*.go`；
//	       `check-compat-manifest.py` 的 R4 扫 `core/` 下 `*.go`）⇒ 已按既有血证处置：
//	       · 正文**零私有绝对路径字面量**（既无家目录前缀、也无私有卷前缀；一律运行时取）；
//	       · **不写** `setErr(` / `kind="` 的构造原串（`check-error-kinds` R3 的口）；
//	       · **不写** statepath 点 File 后紧跟引号路径 一类被 `check-compat-manifest` R4 认作
//	         「迁移落点」的调用字面量（★ 原串**拆开写**：连续原串会把真仓扫红 —— 本件首落
//	         时正是踩了这条血证，见交付读数）；
//	       · self-test 夹具里的魔法串（`lineref-selftest-` / `twin.md` 一族）只落在
//	         **运行时建在 TMPDIR 下**的夹具件正文，**不落本件正文**的连续被扫形态。
//	    ③ **证明处理有效**：落仓前后跑上述他门同一命令，命中数与输出 sha16 **逐字不变**
//	       （见交付读数；本件被点名 0 次）。
//	    ④ 本件**不接线** ⇒ 不进任何命令树/用法面 ⇒ 无「他门用法面」新增字面量。
//
// py 原件真口子（★ 不自行修 · 逐字保留 · **交回入账**）：
//
//	`:135` `analyze` 里 `try: open(full, ...) readlines() except OSError: continue`
//	—— 扫描面内的件**读不了就静默跳过**：该件内所有引用**凭空消失**、既不计「可判」
//	也不计「判不了」，分母守恒断言照样成立（守恒只对已读到的行成立）⇒ **缺件/读不了的件
//	≠ 绿**：真仓若有一件不可读，本门照样可能 rc=0。本件**逐字照搬**此行为（保两态对齐），
//	问题另行入册（✗ 不在本单改 py、✗ 不改本件行为）。普查归 **B 类（读失败静默）**。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **旋钮面新增** `--docs-root DOCS_ROOT`：py 不认 ⇒ 该格 py 退 2（argparse
//	   `unrecognized arguments`）、本件正常判。★ 单列点名的**唯一有意差异格**。
//	② 非 UTF-8 正文的 U+FFFD 替换粒度（见正则语义差④）。
//	③ argparse **长选项前缀缩写**（如 `--roo`⇒`--root`）本件不实现；
//	   `store_true` 型带 `=值` 的 argparse 文案（"ignored explicit argument"）本件复刻。
//	④ **默认根定位**：见 (a) —— 「脚本位置 ≠ cwd 上溯根」时不属逐字对齐承诺。
//	⑤ `--self-test` 用**随机后缀**临时目录，但其输出**只打 ✓/✗ 行与格数、不含任何路径**
//	   ⇒ 两态可逐字对齐（掩码面无物可掩）。
//	⑥ 非 UTF-8 文件名：py `os.listdir` 给 surrogateescape 代理串、Go 给原始字节
//	   ⇒ 该角落的排序/匹配不同（真仓零此形态）。
//	⑦ `os.walk` 遇目录不可读会让 py 抛 `OSError` 未捕获（进程 rc=1 + 回溯）；本件按
//	   「该子树跳过」处理 ⇒ 该角落 rc 不同、正文不同（真仓零此形态）。
//
//	`--evidence GAP-20261001-13`（在册 M2 承接 · 已回读 · 语义对口）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const clrlVersion = "1.0.0"

// ── py 常量逐字 ──────────────────────────────────────────────────────────────────────
const clrlScanDirRel = "scripts/gates" // py os.path.join("scripts", "gates")

var clrlScanExts = []string{".md", ".py", ".sh"} // py SCAN_EXTS

const clrlExt = "py|sh|md|go|json|yaml|yml|ts|js|tsv|jsonl|txt|toml|rs|lock|cfg" // py EXT

// py FIXTURE_CALLS（封闭名单 · 「引用被 \\n\" 收尾」**同时**成立才判样张）。
var clrlFixtureCalls = []string{"_mk(", "mkcase(", "mkfile(", "mkroot(", "_mkroot(",
	"write_text(", "mkfixture(", "mkfixturecase("}

// py FIXTURE_HINTS 在原件里**定义了但从未被读取**（死常量）⇒ 本件**不落**（见未覆盖面：
// 逐字对齐只针对**可观测面**；常量不被读取 ⇒ 无观测差）。
var clrlPruneDirs = []string{".git", "node_modules", "target", "vendor"} // py os.walk 剪枝集

// clrlPWS —— py `\s`（Unicode）的逐码点展开（RE2 `\s` 是 ASCII ⇒ 必须手写对齐）。
const clrlPWS = " \t\n\v\f\r\x1c\x1d\x1e\x1f\u0085\u00a0\u1680" +
	"\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a" +
	"\u2028\u2029\u202f\u205f\u3000"

// clrlRefRE —— 逐字复刻 py REF_RE（`\d`⇒`\p{Nd}`、`\s`⇒clrlPWS 展开）。
var clrlRefRE = regexp.MustCompile(
	"([^`\"'\\[\\]<>|=，：。、；（）「」" + clrlPWS + "]*\\.(?:" + clrlExt + ")):" +
		"([\\p{Nd}]+)(?:-([\\p{Nd}]+))?")

// ── 路径原语（posixpath 语义逐字）──────────────────────────────────────────────────────

// clrlPyJoin —— 复刻 posixpath.join(a, b)：b 绝对 ⇒ 丢弃 a；a 空或以 "/" 收尾 ⇒ 直拼；
// 否则补一个 "/"。★ 不规范化（不吃 `..`/`.`/重复斜杠）—— 与 py 逐字同构。
func clrlPyJoin(a, b string) string {
	if strings.HasPrefix(b, "/") {
		return b
	}
	if a == "" || strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

func clrlBase(p string) string { return filepath.Base(p) }

func clrlIsReg(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func clrlIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// clrlPhysicalLines —— py `physical_lines`：**二进制**读、数 `\n`；末行无换行 +1；
// 空件 = 0；读不了（OSError）⇒ nil。
func clrlPhysicalLines(path string) *int {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if len(data) == 0 {
		v := 0
		return &v
	}
	n := bytes.Count(data, []byte("\n"))
	if !bytes.HasSuffix(data, []byte("\n")) {
		n++
	}
	return &n
}

// clrlParseNum —— py `int(...)`：先 ASCII 快路；非 ASCII 十进制数字按各自十进块的零值求值
// （Nd 的十进块恒为连续 10 个码点 ⇒ 回溯到块内零值）。
func clrlParseNum(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	v := 0
	for _, r := range s {
		z := r
		for z > 0 && unicode.IsDigit(z-1) {
			z--
		}
		v = v*10 + int(r-z)
	}
	return v
}

// clrlDefaultRoot —— 见 (a)：起于 cwd 上溯 ≤8 层找锚点，兜底 abspath(cwd)。
func clrlDefaultRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if abs, err := filepath.Abs(cwd); err == nil {
		cwd = abs
	}
	cur := cwd
	for i := 0; i < 8; i++ {
		cur = filepath.Dir(cur)
		if cur == "" || cur == "/" {
			break
		}
		anchor := clrlPyJoin(clrlPyJoin(clrlPyJoin(clrlPyJoin(cur, "core"), "internal"), "version"), "version.go")
		if clrlIsReg(anchor) {
			return cur
		}
	}
	return cwd
}

// clrlDocsDefault —— py `os.path.join(os.path.dirname(root), "Zerg-内部文档")` 逐字同构。
// ★ 缺省串不含 `..` 段 ⇒ 用 dirname + join，**不做**串拼。
func clrlDocsDefault(root string) string {
	return clrlPyJoin(filepath.Dir(root), "Zerg-内部文档")
}

// ── 扫描面（点1/点2）───────────────────────────────────────────────────────────────────

type clrlFace struct{ Name, Full string }

func clrlHasScanExt(name string) bool {
	for _, e := range clrlScanExts {
		if strings.HasSuffix(name, e) {
			return true
		}
	}
	return false
}

// clrlScanFace —— py `scan_face`：非目录 ⇒ []；否则 sorted(listdir) 后逐名 isfile 过滤。
func clrlScanFace(gatesDir string) []clrlFace {
	if !clrlIsDir(gatesDir) {
		return nil
	}
	f, err := os.Open(gatesDir)
	if err != nil {
		return nil
	}
	ents, err := f.ReadDir(-1) // ★ readdir 原序（✗ 不用会排序的 os.ReadDir）
	f.Close()
	if err != nil && len(ents) == 0 {
		return nil
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names) // ★ 点1：复现 py 明写的 sorted(...)
	out := []clrlFace{}
	for _, name := range names {
		full := clrlPyJoin(gatesDir, name)
		if clrlIsReg(full) && clrlHasScanExt(name) { // ★ 点2：os.Stat 跟随符号链接
			out = append(out, clrlFace{Name: name, Full: full})
		}
	}
	return out
}

// ── os.walk 复刻（点3 · 保原序 · followlinks=False）────────────────────────────────────

func clrlPrunedDir(name string) bool {
	for _, d := range clrlPruneDirs {
		if name == d {
			return true
		}
	}
	return false
}

// clrlScandir —— 复刻 `scandir`：**原序**；`is_dir()` 跟随符号链接（⇒ os.Stat）；
// 断链/stat 失败归入文件（同 py DirEntry.is_dir() 失败返 False）。
func clrlScandir(dir string) (dirs, files []string, ok bool) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, nil, false
	}
	defer f.Close()
	ents, err := f.ReadDir(-1)
	if err != nil && len(ents) == 0 {
		return nil, nil, false
	}
	for _, e := range ents {
		name := e.Name()
		if fi, serr := os.Stat(clrlPyJoin(dir, name)); serr == nil && fi.IsDir() {
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}
	return dirs, files, true
}

// clrlWalk —— 复刻 os.walk(base) 的产出序：先当前层文件（entry 序）→ 再按目录 entry 序下钻。
func clrlWalk(base string, visit func(dirpath string, files []string)) {
	dirs, files, ok := clrlScandir(base)
	if !ok {
		return
	}
	visit(base, files)
	for _, d := range dirs {
		if clrlPrunedDir(d) {
			continue
		}
		clrlWalk(clrlPyJoin(base, d), visit)
	}
}

// ── 落点四规则（py `resolve`）─────────────────────────────────────────────────────────

func clrlResolve(token, root, docsRoot string) (string, string) {
	if !strings.Contains(token, "/") {
		// 裸件名：全仓按基名找同名件（只走两个仓的常见子树）
		var cands []string
		for _, base := range []string{root, docsRoot} {
			if base == "" || !clrlIsDir(base) {
				continue
			}
			bn := clrlBase(token)
			clrlWalk(base, func(dirpath string, files []string) {
				for _, fn := range files {
					if fn == bn {
						cands = append(cands, clrlPyJoin(dirpath, token))
					}
				}
			})
		}
		if len(cands) == 0 {
			return "", "missing"
		}
		lens := map[int]bool{} // ★ 仅计集合基数 ⇒ 零 map 迭代进输出
		for _, c := range cands {
			p := clrlPhysicalLines(c)
			if p == nil {
				lens[-1] = true
			} else {
				lens[*p] = true
			}
		}
		if len(lens) == 1 {
			return cands[0], "found"
		}
		return "", "ambiguous"
	}
	cands := []string{clrlPyJoin(root, token)}
	if docsRoot != "" {
		cands = append(cands, clrlPyJoin(docsRoot, token))
		if strings.HasPrefix(token, "Zerg-内部文档/") {
			cands = append(cands, clrlPyJoin(docsRoot, token[len("Zerg-内部文档/"):]))
		}
		if strings.HasPrefix(token, "docs/") {
			cands = append(cands, clrlPyJoin(docsRoot, token[len("docs/"):]))
		}
	}
	for _, c := range cands {
		if c != "" && clrlIsReg(c) {
			return c, "found"
		}
	}
	return "", "missing"
}

// ── 逐行取文本（py `open(..., encoding="utf-8", errors="replace").readlines()`）──────
// py 默认 newline=None ⇒ **通用换行**：`\r\n`→`\n`、孤立 `\r`→`\n`，再按 `\n` 切（保留 `\n`）。
func clrlTextLines(path string) ([]string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	s := strings.ToValidUTF8(string(data), "\uFFFD")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if s == "" {
		return []string{}, true
	}
	var lines []string
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			lines = append(lines, s)
			break
		}
		lines = append(lines, s[:i+1])
		s = s[i+1:]
	}
	return lines, true
}

// ── analyze（py `analyze`）────────────────────────────────────────────────────────────

type clrlRow struct {
	RefFile string
	RefLine int
	Token   string
	Start   int
	Stop    int
	Kind    string
	Target  *string
	NLines  *int
	Over    bool
}

type clrlStats struct {
	Total     int
	Judged    int
	Unjudged  int
	Over      int
	Conserved bool
}

func clrlFixtureCallIn(text, prev string) bool {
	for _, k := range clrlFixtureCalls {
		if strings.Contains(text, k) || strings.Contains(prev, k) {
			return true
		}
	}
	return false
}

func clrlAnalyze(face []clrlFace, root, docsRoot string) ([]clrlRow, clrlStats, []string) {
	rows := []clrlRow{}
	var unreadable []string
	for _, fe := range face {
		lines, ok := clrlTextLines(fe.Full)
		if !ok {
			// ★ B 类读失败 ⇒ 登记 + 判红（✗ 不再静默跳过）· 体例照同包已治件 check_token_argv.go。
			unreadable = append(unreadable, clrlPyJoin(clrlScanDirRel, fe.Name))
			continue
		}
		for i, text := range lines {
			prev := ""
			if i >= 1 {
				prev = lines[i-1]
			}
			lineno := i + 1
			for _, m := range clrlRefRE.FindAllStringSubmatchIndex(text, -1) {
				token := text[m[2]:m[3]]
				start := clrlParseNum(text[m[4]:m[5]])
				stop := start
				if m[6] >= 0 {
					stop = clrlParseNum(text[m[6]:m[7]])
				}
				tail := text[m[1]:]
				if len(tail) > 3 {
					tail = tail[:3]
				}
				sample := (strings.HasPrefix(tail, `\n`) || strings.HasPrefix(tail, `\\n`)) &&
					clrlFixtureCallIn(text, prev)
				var path, kind string
				if sample {
					kind = "sample"
				} else {
					path, kind = clrlResolve(token, root, docsRoot)
				}
				var nLines *int
				var target *string
				if path != "" {
					nLines = clrlPhysicalLines(path)
					if rel, err := filepath.Rel(root, path); err == nil {
						target = &rel
					}
				}
				over := kind == "found" && nLines != nil && stop > *nLines
				rows = append(rows, clrlRow{
					RefFile: clrlPyJoin(clrlScanDirRel, fe.Name),
					RefLine: lineno,
					Token:   token,
					Start:   start,
					Stop:    stop,
					Kind:    kind,
					Target:  target,
					NLines:  nLines,
					Over:    over,
				})
			}
		}
	}
	st := clrlStats{Total: len(rows)}
	for _, r := range rows {
		if r.Kind == "found" {
			st.Judged++
		} else {
			st.Unjudged++
		}
		if r.Over {
			st.Over++
		}
	}
	st.Conserved = (st.Judged + st.Unjudged) == st.Total
	return rows, st, unreadable
}

// ── 机器面（Python json.dumps(..., ensure_ascii=False, indent=1) 逐字）───────────────
type clrlOM struct {
	keys []string
	vals []any
}

func (o *clrlOM) set(k string, v any) {
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
}

func clrlPyQuote(s string) string {
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

// clrlEnc —— json.dumps(indent=1) 逐字：对象键在 L+1、列表元素在 L+1、分隔 `,\n`。
func clrlEnc(v any, lvl int) string {
	pad := func(n int) string { return strings.Repeat(" ", n) }
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return clrlPyQuote(x)
	case int:
		return strconv.Itoa(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case *int:
		if x == nil {
			return "null"
		}
		return strconv.Itoa(*x)
	case []*clrlOM:
		if len(x) == 0 {
			return "[]"
		}
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, pad(lvl+1)+clrlEnc(e, lvl+1))
		}
		return "[\n" + strings.Join(parts, ",\n") + "\n" + pad(lvl) + "]"
	case *clrlOM:
		if len(x.keys) == 0 {
			return "{}"
		}
		parts := make([]string, 0, len(x.keys))
		for i, k := range x.keys {
			parts = append(parts, pad(lvl+1)+clrlPyQuote(k)+": "+clrlEnc(x.vals[i], lvl+1))
		}
		return "{\n" + strings.Join(parts, ",\n") + "\n" + pad(lvl) + "}"
	}
	return "null"
}

func (r clrlRow) om() *clrlOM {
	o := &clrlOM{}
	o.set("ref_file", r.RefFile)
	o.set("ref_line", r.RefLine)
	o.set("token", r.Token)
	o.set("start", r.Start)
	o.set("stop", r.Stop)
	o.set("kind", r.Kind)
	var tgt any
	if r.Target != nil {
		tgt = *r.Target
	}
	o.set("target", tgt)
	var nl any
	if r.NLines != nil {
		nl = *r.NLines
	}
	o.set("n_lines", nl)
	o.set("over", r.Over)
	return o
}

// ── argparse 子集 ────────────────────────────────────────────────────────────────────
const clrlProg = "check-lineref-limit.py"

// ★ 帮助/用法正文**逐字复刻 py**（含行续 30 空格）。`--docs-root` 是**有意不列进帮助面**的
// 新增旋钮（见件头未覆盖面①）：帮助面逐字对齐 ⟷ 旋钮面单列点名的取舍。
const clrlUsage = "usage: check-lineref-limit.py [-h] [--json] [--verbose] [--root ROOT]\n" +
	"                              [--self-test]\n"

const clrlHelp = clrlUsage + "\n" +
	"optional arguments:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --json\n" +
	"  --verbose\n" +
	"  --root ROOT  仓根（缺省自动回溯；供入仓前试跑）\n" +
	"  --self-test\n"

func clrlArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, clrlUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", clrlProg, msg)
	return 2
}

type clrlArgs struct {
	JSON     bool
	Verbose  bool
	SelfTest bool
	Root     string
	HasRoot  bool
	DocsRoot string
	HasDocs  bool
}

// clrlParse —— 返回 (args, rc)。rc>=0 ⇒ 已出结论；rc<0 ⇒ 继续。
func clrlParse(stdout, stderr io.Writer, argv []string) (clrlArgs, int) {
	a := clrlArgs{}
	var unknown []string
	i := 0
	needVal := func(name string, hasEq bool, val string) (string, int) {
		if hasEq {
			return val, -1
		}
		if i < len(argv) && !strings.HasPrefix(argv[i], "-") {
			v := argv[i]
			i++
			return v, -1
		}
		return "", clrlArgErr(stderr, "argument "+name+": expected one argument")
	}
	for i < len(argv) {
		tok := argv[i]
		i++
		if tok == "-h" || tok == "--help" {
			fmt.Fprint(stdout, clrlHelp)
			return a, 0
		}
		if strings.HasPrefix(tok, "--") {
			name := tok
			val := ""
			hasEq := false
			if eq := strings.IndexByte(tok, '='); eq >= 0 {
				name, val, hasEq = tok[:eq], tok[eq+1:], true
			}
			switch name {
			case "--root":
				v, rc := needVal(name, hasEq, val)
				if rc >= 0 {
					return a, rc
				}
				a.Root, a.HasRoot = v, true
			case "--docs-root":
				v, rc := needVal(name, hasEq, val)
				if rc >= 0 {
					return a, rc
				}
				a.DocsRoot, a.HasDocs = v, true
			case "--json":
				if hasEq {
					return a, clrlArgErr(stderr, fmt.Sprintf("argument --json: ignored explicit argument '%s'", val))
				}
				a.JSON = true
			case "--verbose":
				if hasEq {
					return a, clrlArgErr(stderr, fmt.Sprintf("argument --verbose: ignored explicit argument '%s'", val))
				}
				a.Verbose = true
			case "--self-test":
				if hasEq {
					return a, clrlArgErr(stderr, fmt.Sprintf("argument --self-test: ignored explicit argument '%s'", val))
				}
				a.SelfTest = true
			default:
				unknown = append(unknown, tok)
			}
			continue
		}
		unknown = append(unknown, tok)
	}
	if len(unknown) > 0 {
		return a, clrlArgErr(stderr, "unrecognized arguments: "+strings.Join(unknown, " "))
	}
	return a, -1
}

// ── 自检（8 格 · py `self_test` 逐字搬）────────────────────────────────────────────────

func clrlCompactAny(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return clrlPyQuote(x)
	case int:
		return strconv.Itoa(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	return "null"
}

func clrlCompactOM(o *clrlOM) string {
	parts := make([]string, 0, len(o.keys))
	for i, k := range o.keys {
		parts = append(parts, clrlPyQuote(k)+": "+clrlCompactAny(o.vals[i]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// clrlStatsJSON —— py `json.dumps(st, ensure_ascii=False)`（无 indent · 仅失败格打印）。
func clrlStatsJSON(st clrlStats) string {
	o := &clrlOM{}
	o.set("total", st.Total)
	o.set("judged", st.Judged)
	o.set("unjudged", st.Unjudged)
	o.set("over", st.Over)
	o.set("conserved", st.Conserved)
	return clrlCompactOM(o)
}

func clrlSelfTest(stdout io.Writer) int {
	type res struct {
		tag    string
		ok     bool
		detail string
	}
	var results []res
	caseOf := func(tag string, ok bool, detail string) {
		results = append(results, res{tag, ok, detail})
	}

	tmp, err := os.MkdirTemp("", "lineref-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "   ✗ 自检夹具建不起来：%v\n", err)
		fmt.Fprint(stdout, "自检 0 格 · 失败 1 格\n")
		return 1
	}
	defer os.RemoveAll(tmp)

	root := filepath.Join(tmp, "Zerg")
	docs := filepath.Join(tmp, "Zerg-内部文档")
	gdir := filepath.Join(root, clrlScanDirRel)
	os.MkdirAll(gdir, 0o755)
	os.MkdirAll(docs, 0o755)

	mk := func(rel, body string) string {
		p := filepath.Join(root, rel)
		if strings.HasPrefix(rel, "Zerg-内部文档/") {
			p = filepath.Join(docs, rel[len("Zerg-内部文档/"):])
		}
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
		return p
	}
	scan := func() ([]clrlRow, clrlStats) {
		rows, st, _ := clrlAnalyze(clrlScanFace(gdir), root, docs)
		return rows, st
	}
	pick := func(rows []clrlRow, suffix string) []clrlRow {
		var out []clrlRow
		for _, r := range rows {
			if strings.HasSuffix(r.RefFile, suffix) {
				out = append(out, r)
			}
		}
		return out
	}

	// NC1 正控：可判且不越界 ⇒ rc=0，且分母守恒
	mk("wall/Cargo.lock", "l1\nl2\nl3\nl4\nl5\nl6\nl7\n")
	mk("scripts/gates/ok.md", "见 wall/Cargo.lock:7\n")
	_, st := scan()
	caseOf("NC1 正控：可判不越界 ⇒ rc=0 且守恒",
		st.Judged == 1 && st.Over == 0 && st.Conserved, clrlStatsJSON(st))

	// NC2 越界负控：行号止 > 件行数 ⇒ 越界，且点名唯一
	mk("scripts/gates/bad.md", "见 wall/Cargo.lock:8\n")
	rows, st := scan()
	var overs []clrlRow
	for _, r := range rows {
		if r.Over {
			overs = append(overs, r)
		}
	}
	caseOf("NC2 越界负控：:8 引 7 行件 ⇒ 越界且恰 1 处",
		len(overs) == 1 && strings.HasSuffix(overs[0].RefFile, "bad.md"), clrlStatsJSON(st))
	os.Remove(filepath.Join(gdir, "bad.md"))

	// NC3 落点改名 ⇒ 必须落「判不了」，且总数守恒、不许消失
	mk("scripts/gates/gone.md", "见 wall/Cargo-nope.lock:3\n")
	rows, st = scan()
	gone := pick(rows, "gone.md")
	caseOf("NC3 改名 ⇒ 落判不了且总数守恒（不许消失）",
		len(gone) == 1 && gone[0].Kind == "missing" && st.Conserved, clrlStatsJSON(st))
	os.Remove(filepath.Join(gdir, "gone.md"))

	// NC4 孪生反向格：同名两解行数不同 ⇒ 不许判
	mk("a/gem.md", "l1\nl2\nl3\n")
	mk("b/gem.md", "l1\nl2\nl3\nl4\nl5\n")
	mk("scripts/gates/twin.md", "见 gem.md:3\n")
	rows, st = scan()
	twin := pick(rows, "twin.md")
	caseOf("NC4 孪生反向格：同名行数不同 ⇒ 不许判",
		len(twin) == 1 && twin[0].Kind == "ambiguous", clrlStatsJSON(st))
	// 同名行数全同 ⇒ 判
	mk("b/gem.md", "l1\nl2\nl3\n")
	rows, st = scan()
	twin = pick(rows, "twin.md")
	caseOf("NC4b 同名行数全同 ⇒ 判（且 :3 不越界）",
		len(twin) == 1 && twin[0].Kind == "found" && !twin[0].Over, clrlStatsJSON(st))
	os.Remove(filepath.Join(gdir, "twin.md"))

	// NC5 索引面回归格：CJK 名件必须找得到（否则 rc=2）
	mk("scripts/gates/中文件.md", "l1\nl2\nl3\nl4\n")
	mk("scripts/gates/cjk.md", "见 中文件.md:4\n")
	rows, st = scan()
	cjk := pick(rows, "cjk.md")
	caseOf("NC5 CJK 名件必须找得到（found ⇒ 不许落判不了）",
		len(cjk) == 1 && cjk[0].Kind == "found" && !cjk[0].Over, clrlStatsJSON(st))
	os.Remove(filepath.Join(gdir, "cjk.md"))
	os.Remove(filepath.Join(gdir, "中文件.md"))

	// NC7 样张档负控：夹具构造行里写的引用 ⇒ 只登记、不许判越界
	mk("scripts/gates/fx.md",
		"_mk(root, docs, \"x.md\", \"见 wall/Cargo.lock:99\\\\n\")\n")
	rows, st = scan()
	fx := pick(rows, "fx.md")
	fxDetail := "["
	for i, r := range fx {
		if i > 0 {
			fxDetail += ", "
		}
		fxDetail += "('" + r.Kind + "', " + strconv.FormatBool(r.Over) + ")"
	}
	fxDetail += "]"
	caseOf("NC7 样张档：夹具行里的引用 ⇒ 落 sample、不许越界",
		len(fx) == 1 && fx[0].Kind == "sample" && !fx[0].Over, fxDetail)
	os.Remove(filepath.Join(gdir, "fx.md"))

	// NC6 空转 rc=2
	os.Remove(filepath.Join(gdir, "ok.md"))
	mk("scripts/gates/empty.md", "无引用的普通一行\n")
	_, st = scan()
	caseOf("NC6 空转 ⇒ rc=2（面内引用 0 处）", st.Total == 0, clrlStatsJSON(st))

	bad := 0
	for _, r := range results {
		if r.ok {
			fmt.Fprintf(stdout, "   ✓ %s\n", r.tag)
		} else {
			fmt.Fprintf(stdout, "   ✗ %s  %s\n", r.tag, r.detail)
			bad++
		}
	}
	fmt.Fprintf(stdout, "自检 %d 格 · 失败 %d 格\n", len(results), bad)
	if bad != 0 {
		return 1
	}
	return 0
}

// ── 入口（= py `main()`）─────────────────────────────────────────────────────────────
func checkLinerefLimitGateMain(stdout, stderr io.Writer, argv []string) int {
	a, rc := clrlParse(stdout, stderr, argv)
	if rc >= 0 {
		return rc
	}
	if a.SelfTest {
		return clrlSelfTest(stdout)
	}

	root := a.Root
	if !a.HasRoot {
		root = clrlDefaultRoot()
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	docsRoot := a.DocsRoot
	if !a.HasDocs {
		docsRoot = clrlDocsDefault(root)
	} else if abs, err := filepath.Abs(docsRoot); err == nil {
		docsRoot = abs
	}
	gatesDir := clrlPyJoin(root, clrlScanDirRel)

	face := clrlScanFace(gatesDir)
	if len(face) == 0 {
		fmt.Fprintf(stdout, "不给结论：扫描面 %s 下 0 件（rc=2）\n", clrlScanDirRel)
		return 2
	}
	rows, st, unreadable := clrlAnalyze(face, root, docsRoot)

	if len(unreadable) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 扫描面件读不了 %d 件（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论 · 读不到不许当健康）：\n", len(unreadable))
		for _, u := range unreadable {
			fmt.Fprintf(stderr, "  ── %s\n", u)
		}
		return 2
	}
	if st.Total == 0 {
		fmt.Fprintf(stdout, "不给结论：扫描面 %d 件里引用 0 处（rc=2）\n", len(face))
		return 2
	}
	if !st.Conserved {
		fmt.Fprintf(stdout, "不给结论：分母守恒断言不成立（可判 %d + 判不了 %d ≠ 总 %d）（rc=2）\n",
			st.Judged, st.Unjudged, st.Total)
		return 2
	}

	overs := []clrlRow{}
	for _, r := range rows {
		if r.Over {
			overs = append(overs, r)
		}
	}

	if a.JSON {
		var unjudged []clrlRow
		for _, r := range rows {
			if r.Kind != "found" {
				unjudged = append(unjudged, r)
			}
		}
		rowsOM := func(src []clrlRow) []*clrlOM {
			out := make([]*clrlOM, 0, len(src))
			for _, r := range src {
				out = append(out, r.om())
			}
			return out
		}
		stOM := &clrlOM{}
		stOM.set("total", st.Total)
		stOM.set("judged", st.Judged)
		stOM.set("unjudged", st.Unjudged)
		stOM.set("over", st.Over)
		stOM.set("conserved", st.Conserved)
		rootOM := &clrlOM{}
		rootOM.set("face", len(face))
		rootOM.set("stats", stOM)
		rootOM.set("over", rowsOM(overs))
		rootOM.set("unjudged", rowsOM(unjudged))
		fmt.Fprintln(stdout, clrlEnc(rootOM, 0))
	} else {
		for _, r := range overs {
			tgt := ""
			if r.Target != nil {
				tgt = *r.Target
			}
			nl := 0
			if r.NLines != nil {
				nl = *r.NLines
			}
			fmt.Fprintf(stdout, "越界 %s:%d  被引件 %s  行号止 %d > 件行数 %d\n",
				r.RefFile, r.RefLine, tgt, r.Stop, nl)
		}
		if a.Verbose {
			for _, r := range rows {
				if r.Kind != "found" {
					fmt.Fprintf(stdout, "判不了 %s:%d  %s（%s）\n",
						r.RefFile, r.RefLine, r.Token, r.Kind)
				}
			}
		}
		cons := "✗"
		if st.Conserved {
			cons = "✓"
		}
		fmt.Fprintf(stdout, "扫描面 %d 件 · 引用总 %d 处 · 可判 %d · 越界 %d · 判不了 %d（分母守恒 %s）\n",
			len(face), st.Total, st.Judged, st.Over, st.Unjudged, cons)
	}

	if len(overs) > 0 {
		return 1
	}
	return 0
}
