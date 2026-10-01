// check_placeholder_residue.go —— 门禁件 `scripts/gates/check-placeholder-residue.py`（250 行）的
// **Go 真搬**（M2 路C 第 5 枚 · 判据真跑子进程型 · 甲案口径）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案把判据实现面分两段走 —— M1 = 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面
//	（转发壳）；M2 = 按价值真搬 Go（落点 `core/cmd/zerg/`）。本件 = M2 路C 的一枚：
//	验收判据 = 对同一件跑两态（py 原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**。
//
//	★ 甲案口径（Mr2109 已拍）：判据件允许**真跑子进程**，条件 = **输入确定 ⇒ 输出可逐字对拍**。
//	本件正属此类：它真跑 `git rev-list` / `git grep`（子进程）扫**全历史**。
//
// ★ 输入确定性（本件最要紧的一条）：
//
//	本件判据的输入 = **所给 git 仓的提交树**（`git grep <形态> <commit>` 读的是**树对象**，
//	不是工作树）⇒ 判据**不吃工作树 dirty**。真仓工作树虽 dirty（`git status` 现读 176 行），
//	但本件扫 `HEAD` 树 ⇒ dirty 不进读数。**输入锁定指纹 = HEAD sha**：
//	`6aa1120e50d7ec03ad0b25a9b5c2548d0040bcad`（真仓 `git rev-parse HEAD`）。
//	⇒ 对拍时两侧（py 原件 / 本件）**必须面对同一个 `--git-dir` / 同一 HEAD**。
//
// ★ 工具链指纹（现读 · 版本面差写进未覆盖面）：
//
//	`git version 2.50.1 (Apple Git-155)` · 路径 `/usr/bin/git`（另有 `/opt/homebrew/bin/git`）。
//	本件走 `exec.Command("git", ...)`（PATH 首命中）；py 侧 `subprocess.run(["git", ...])` 同。
//
// ★ 选它（M2 路C 第 5 枚 · 候选区间 a–e / f–p 抽空后按「真跑子进程型」点名取件）：
//
//	· 判据面**只读**（`--self-test` 只在 `tempfile.mkdtemp()` 里造仓；默认档零写面）；
//	· 输出**确定**（`git grep` 行序 = git 自身定序；`sorted(...)` / 稳定排序复刻）；
//	· 退码 `0/1/2` 三档齐（0 干净 / 1 有残留 / 2 不给结论或用法错）。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkPlaceholderResidueGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 干净 / 1 有残留 / 2 不给结论（含用法错档）
//	  · 输出 —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ★ 标准声明 (a) —— **默认根定位差异（不适用 · 逐字无差）**：
//
//	py 不读 `__file__` 定仓（除 `--self-test` 的 `SELF_PATH = os.path.abspath(__file__)` 用于
//	**再 spawn 自己**）。判据的 git 目录 = **位置参数**（缺省 `.`）。⇒ py 与 Go 的默认根
//	**同源于同一个位置参数**（缺省 `.` 相对 **cwd**）—— 两面同形，无差异。
//	唯一的 `__file__` 用法（self-test 再 spawn）在 Go 里改为**进程内直调入口**（见下）。
//
// ★ 标准声明 (b) —— **自指检查（四形态）**：
//
//	① 本件落在**自己**的扫描面内（`git grep` 扫 git 跟踪的全树）⇒ 本件正文**不得**出现
//	   完整占位符形态（`py:20` 也是拆开写：「否则会自己命中自己」）。本件照做：占位符由
//	   `cprPlaceholder()` **拼接构造**（源码文本里没有连成一串的它）；自检/对拍夹具里的形态
//	   一律**运行时**由该函数取用，正文里不落整串。
//	② 本件落在 `check-hardcoded-private-paths.py` 扫描面内（其 `SCAN_DIRS` 含 `core/`）——
//	   命中词是三条私有绝对路径。本件正文**不写**这三串（自检夹具里那段正则前缀也拆开写）。
//	③ 本件落在 `check-error-kinds.py` 的**用法面**内（扫 `core/cmd/zerg/*.go` · 非 `_test.go`）：
//	   命中形 = 「错误登记调用的字符串实参」与「JSON 里 kind 键的字符串值」两形。本件正文不含这两形
//	   （★ 用法面字面一律**描述式** —— ✗ 落任何会被该门正则命中的调用/键名原串：前例已有件头注释
//	   写被扫形态 ⇒ 让他门凭空多报两处）。
//	④ 本件落在 `check-wall-layer.py` 扫描面内（扫 `core/cmd/zerg/*.go` · 非 `_test.go`）：
//	   命中形 = 层级常量赋值 与 命令面层级标记。本件正文不含这两形。
//	★ 落仓后**实跑**上面三门，证 rc 未因本件翻红（见回执）。
//
// ★ 遍历面 —— **不适用**（给理由）：py 本件**零 `os.walk` / 零 `os.listdir` / 零 `glob`**；
//
//	文件清单**完全来自 `git grep` 的输出**（git 自身定序）。⇒ 无「保 readdir 原序 / 先文件后下钻」
//	这一面。输出行序的**决定者** = `git grep` 的自身输出序（逐 commit）＋ 末尾那一次
//	`sorted(files.items(), key=-len(hits))`（稳定排序 ⇒ 同数保**首次遇到序**）。
//
// ★ Join / 词法面 —— **逐处现读**（结果：主逻辑零 `os.path.join`）：
//
//	主扫描路径**零路径拼接**（`git -C <gitdir> ...` 直传字符串）。`os.path.join` 只出现在
//	`self_test()` 的夹具构造里：`os.path.join(tmp, name)` / `os.path.join(d, "说明.md")` /
//	`os.path.join(nogit, "说明.md")` —— 三处后段都是**单段相对名**（无 `..`、非绝对、非空段）
//	⇒ 与 `filepath.Join` **逐字同值**，三类语义差**均不触发**。
//	词法面：`os.path.abspath(__file__)`（SELF_PATH）—— 本件不复刻「再 spawn 自己」，改进程内直调，
//	故**不涉**；`normpath/relpath/realpath/dirname` **零调用** ⇒ 双前导斜杠那类差异本件**不涉**。
//	★ 尽管如此，依标准仍附 `PyJoin`/`PyNormpath`/`PyDirname` 同构实现并各钉一格单测（恒同构最省心）。
//
// ★ 正则面 —— 三条正则**逐字对读**：
//
//	① `LINE_RE = ^(?:([0-9a-f]{7,40}):)?(.*?):(\d+):(.*)$`（re.S）⇒ Go `(?s)` 同构；
//	   `\d` 是 py **str** 语义（认 Unicode 数字 `\p{Nd}`）⇒ 用 `\p{Nd}`。
//	② `RESIDUE_RE = <escaped PH>[ \t]*([^\s\"'`\)]{1,})`⇒ `\s` 是 py Unicode 空白类，
//	   **展开为显式集**（含 `\x1c-\x1f` / `\x85` / `\xa0` / `\u1680` / `\u2000-\u200a` /
//	   `\u2028-\u2029` / `\u202f` / `\u205f` / `\u3000`，✗ 不含 `\u180e`）＋ `"` `'` 反引号 `)`。
//	   `{1,}` = `+`；`re.escape` 与 Go `regexp.QuoteMeta` 对 `-` 的转义与否**语义等价**（类外 `-` 非元字符）。
//	③ `PATHY_RE = re.compile(r"/")` ⇒ `strings.Contains(g, "/")`（非正则更省）。
//	★ 这三条**不含**环视 / 反向引用 / `\b` / `(?i:)` ⇒ 无需手写扫描器。
//
// ★ 符号链接面 —— **不适用（给理由）**：py 本件**零 `is_dir`/`isfile`/`exists`/`islink`**；
//
//	它不遍历盘、只扫 git 树 ⇒ 无符号链接判定面。
//
// ★ JSON / null 面 —— **不适用（给理由）**：本件**无 JSON 入参**（读的是 argv 与 git 输出）。
//
//	`--json` 在本门是**拒收档**（无机器读面 ⇒ rc=2）。⇒ 「缺键 / 显式 null」三态在**入参面**
//	不存在；本件改用 **argv 真值面**三形态代替（见测试件：缺省 / 空串 / 显式传值）。
//
// ★ py 真口子（A/B/C/D 四类 · 照实交回入账）：
//
//	C 类（最外层无兜底）：py `__main__` 裸 `sys.exit(...)`；`scan()` 里 `content_of()` 对
//	  LINE_RE 不匹配的非空行返回 `(None,None,None)`，随后 `is_residue(None)` 会 `TypeError`
//	  ——以 `git grep` 的输出形态该路**不可达**（每行恒匹配）；本件落「不匹配即跳过」并写未覆盖面。
//	A 类（tempdir 泄漏）：py `self_test()` 有 `finally: shutil.rmtree` ⇒ **本件本就回收**；
//	  本件 Go `defer os.RemoveAll(tmp)` 同效果；差异不可见（已实测 delta=0）。
//	B 类（读失败静默少判）：py 本件**无** `except OSError: continue` 形。
//	D 类（死旗标/假承诺）：`--all` 真被 `scan()` 读取（非死旗标）。
//
// 已对拍面（两环境 ≥18 格 · 三面逐字：rc / stdout / stderr）：
//
//	甲（真仓 + 显式传参 · py 走真仓原件绝对路径）：真仓 HEAD（rc=0）· 缺省根（cwd=仓根）·
//	传绝对路径 · 未知旗标（rc=2）· `--json`（rc=2）· `--json=stdout`（rc=2）· `--self-test`。
//	乙（仓外夹具 + 每夹具自放 py 副本）：全绿仓 · 残留仓（rc=1）· 定义列假红对照（rc=0）·
//	无 .git 物化树（取不到 ⇒ rc=2）· 同正文真仓成对（rc=1）· 读不了的 git 目录 · `--all` 多提交仓 ·
//	第二位置参数（非 `--all` ⇒ 只印 mode、扫 HEAD）· argv 三形态（缺省/空串/显式）· 绝对路径格。
//	★ 掩码面：**零掩码**（理由：本件输出**不含**时间戳 / 相对时间 / 随机路径 —— self-test 的
//	  `mkdtemp` 后缀只在**失败尾巴**里出现，全过时不进 stdout；已实测两环境独立两次运行逐字节同）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **非 UTF-8 的 git 跟踪件内容**：py `git grep` 用 `encoding="utf-8", errors="replace"`
//	   解码；本件 `exec` 收**原始字节**转字符串 ⇒ 非法序列两侧正文可能不同（真仓现读全为合法 UTF-8）。
//	② **`git` 不在 PATH / 起不来**：py `subprocess.run` 抛 `FileNotFoundError`（C 类 ⇒ traceback rc=1）；
//	   本件落 rc=2（fail-closed · 不给结论）。此格只对齐 rc 之外的两侧不同（正文另论）。
//	③ **`rev-list main` 的 `main` 分支缺席**（如夹具默认 `master`）：两侧都走 rc≠0 ⇒ 判不了（已对拍 rc=2＋正文）。
//	④ `LINE_RE` 不匹配的非空行：见上 C 类（不可达）。
//	⑤ `--self-test` 的**实现手段**差：py 用 `subprocess.run([sys.executable, SELF_PATH])` 再 spawn 自己；
//	   本件**进程内**直调入口（打印文案逐字对齐，差在手段）。
//	⑥ `--self-test` 里 `git init` 的**默认分支名**随跑测机 git 配置（`init.defaultBranch`）变；
//	   两态各自 `git init -q`（都不指定分支）⇒ 同机同值 ⇒ 不影响对拍。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// ── 常量（与 py 逐字同源）────────────────────────────────────────────────────────────────
const (
	cprProg            = "check-placeholder-residue.py"
	cprExitOK          = 0
	cprExitFail        = 1
	cprExitBlocked     = 2
	cprCodeTrackedName = "TRACKED_UNKNOWN"
)

// cprUsage —— py `USAGE` 逐字（含 prog 名硬化 = 原件基名）。
const cprUsage = "用法：python3 scripts/gates/check-placeholder-residue.py <git目录> [<commit>|--all] [--self-test]"

// cprPlaceholder —— py `PLACEHOLDER = "<volume-" + "path>"` 逐字同源。
//
// ★ 自指纪律：本件落在**自己**的扫描面内（`git grep` 扫 git 跟踪的全树）⇒ 正文**不得**出现
// 连成一串的完整占位符形态（py 件头逐字：「否则会自己命中自己」）。这里**拼接构造**。
func cprPlaceholder() string { return "<volume-" + "path>" }

// cprWSClass —— py `\s`（str 模式 = Unicode 空白）的**显式展开**（✗ 不用 Go RE2 的 ASCII `\s`）。
//
// 成员（现读坐实）：`\t \n \v \f \r` · `\x1c-\x1f` · 空格 · `\x85` · `\xa0` · `\u1680` ·
// `\u2000-\u200a` · `\u2028 \u2029` · `\u202f` · `\u205f` · `\u3000`（✗ `\u180e` 不属）。
const cprWSClass = `\t\n\v\f\r\x1c-\x1f \x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

var (
	// cprLineRE —— py `LINE_RE = re.compile(r"^(?:([0-9a-f]{7,40}):)?(.*?):(\d+):(.*)$", re.S)`。
	// `\d`（str）= `\p{Nd}`（认 Unicode 数字）；`(?s)` = `re.S`。
	cprLineRE = regexp.MustCompile(`(?s)^(?:([0-9a-f]{7,40}):)?(.*?):(\p{Nd}+):(.*)$`)
	// cprResidueRE —— py `RESIDUE_RE = re.compile(re.escape(PH) + r"[ \t]*([^\s\"'`+"`"+r"\)]{1,})")`。
	// 类排除 = py Unicode 空白集 ＋ `"` `'` 反引号 `)`。`\x60` = 反引号（避源码里落原字符）。
	cprResidueRE = regexp.MustCompile(
		regexp.QuoteMeta(cprPlaceholder()) + "[ \t]*([^" + cprWSClass + "\"'\x60)]+)")
)

// ── 判据辅助 ─────────────────────────────────────────────────────────────────────────────

// cprPyPad —— py `%-Ns`（左对齐、按**码点**补空格）。
func cprPyPad(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

// cprPyBool —— py `str(bool)` ⇒ "True"/"False"（✗ 不是 Go 的 "true"/"false"）。
func cprPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// cprTruncRunes —— py `s[:n]`（按**码点**截断）。
func cprTruncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// cprIsDefinitionField —— py `is_definition_field(text, m)`：占位符**自成列**（前为行首/制表符，
// 后为制表符/行尾）⇒ 定义面（规则表替换列等模式文本，非残留）。按**码点**判邻位。
func cprIsDefinitionField(rs []rune, startIdx int) bool {
	after := startIdx + len([]rune(cprPlaceholder()))
	before := startIdx == 0 || rs[startIdx-1] == '\t'
	afterOK := after >= len(rs) || rs[after] == '\t'
	return before && afterOK
}

// cprIsResidue —— py `is_residue(text)`：返回 (是否残留, 命中片段=group(0))。
func cprIsResidue(text string) (bool, string) {
	rs := []rune(text)
	for _, m := range cprResidueRE.FindAllStringSubmatchIndex(text, -1) {
		startRune := utf8.RuneCountInString(text[:m[0]])
		if cprIsDefinitionField(rs, startRune) {
			continue
		}
		if strings.Contains(text[m[2]:m[3]], "/") { // PATHY_RE = 余部含斜杠 ⇒ 路径样
			return true, text[m[0]:m[1]]
		}
	}
	return false, ""
}

// cprContentOf —— py `content_of(line)`：解 `<commit>:<path>:<lineno>:<content>`（group2/3/4）。
func cprContentOf(line string) (path, lineno, text string) {
	m := cprLineRE.FindStringSubmatch(line)
	if m == nil {
		return "", "", ""
	}
	return m[2], m[3], m[4]
}

// ── py 词法同构（本件主逻辑零 `os.path.join`，仅备齐以恒同构）──────────────────────────────

// cprPyJoin —— CPython `posixpath.join` 逐段规则（吃 `..` 原样留 / 后段绝对丢前段 / 零段保留尾斜杠）。
func cprPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
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

// cprPyNormpath —— CPython `posixpath.normpath` 逐字（★ 保留**恰两个**前导斜杠 `//a/b`
// ⟷ `filepath.Clean` 折成 `/a/b`；三个及以上折成一个）。
func cprPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initial = 2 // POSIX 允许恰两个前导斜杠
		}
	}
	comps := strings.Split(path, "/")
	var out []string
	for _, c := range comps {
		if c == "" || c == "." {
			continue
		}
		// py: comp != '..' or (not initial and not out) or (out and out[-1] == '..')
		if c != ".." || (initial == 0 && len(out) == 0) || (len(out) > 0 && out[len(out)-1] == "..") {
			out = append(out, c)
		} else if len(out) > 0 {
			out = out[:len(out)-1]
		}
	}
	res := strings.Join(out, "/")
	if initial > 0 {
		res = strings.Repeat("/", initial) + res
	}
	if res == "" {
		return "."
	}
	return res
}

// cprPyDirname —— CPython `posixpath.dirname`（= `posixpath.split` 的头部，**剥尾斜杠**；
// 头部全是斜杠时保留原样，如 `split('/a')` ⇒ head='/'）。
func cprPyDirname(p string) string {
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return ""
	}
	head := p[:i+1]
	if head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// ── scan() ───────────────────────────────────────────────────────────────────────────────

// cprFileHits —— 命中文件（保**首次遇到序**，供末尾稳定排序复刻 py dict 插入序）。
type cprFileHits struct {
	path string
	hits []string
}

// cprRunGit —— 真跑 git 子进程；返回 (退出码, stdout)。起不来 ⇒ code=-1（见未覆盖面②）。
func cprRunGit(args ...string) (int, string) {
	cmd := exec.Command("git", args...)
	var so, se strings.Builder
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return code, so.String()
}

// cprScan —— py `scan(gitdir, all_commits=False, commit=None)` 逐字。
//
// 注意（逐字复刻）：py 调用点为 `scan(target, all_commits=(mode == "--all"))`，**不传 commit**
// ⇒ commit 常量 "HEAD"。第二位置参数（非 `--all`）**只影响打印的 mode**，不影响扫描提交。
func cprScan(gitdir string, allCommits bool) (int, []cprFileHits, string) {
	commit := "HEAD"
	var commits []string
	if allCommits {
		code, out := cprRunGit("-C", gitdir, "-c", "core.quotepath=false", "rev-list", "main")
		if code != 0 {
			return 0, nil, fmt.Sprintf(
				"判不了：仓根不是 git 仓 / git 不可用 ⇒ `git rev-list main` 非零退出（rc=%d）", code)
		}
		commits = strings.Fields(out) // py r.stdout.split()
	} else {
		commits = []string{commit}
	}
	total := 0
	order := []string{}
	idx := map[string]int{}
	var files []cprFileHits
	for _, c := range commits {
		code, out := cprRunGit("-C", gitdir, "-c", "core.quotepath=false", "grep", "-n", "-I",
			cprPlaceholder(), c, "--", ".")
		if code >= 2 {
			return 0, nil, fmt.Sprintf(
				"判不了：仓根不是 git 仓 / git 不可用 ⇒ `git grep` 非零退出（rc=%d · commit=%s）", code, c)
		}
		for _, ln := range strings.Split(out, "\n") {
			if strings.TrimSpace(ln) == "" { // py: if not ln.strip()
				continue
			}
			path, lineno, text := cprContentOf(ln)
			hit, frag := cprIsResidue(text)
			if hit {
				total++
				entry := fmt.Sprintf("%s:%s", lineno, cprTruncRunes(frag, 60))
				if j, ok := idx[path]; ok {
					files[j].hits = append(files[j].hits, entry)
				} else {
					idx[path] = len(order)
					order = append(order, path)
					files = append(files, cprFileHits{path: path, hits: []string{entry}})
				}
			}
		}
	}
	return total, files, ""
}

// ── selftest()（主逻辑前的**自检前言** · 与 py 逐字）──────────────────────────────────────

// cprSelftest —— py `selftest()`：打印 7 例内检表 ＋ 「自测: …」，返回 bad==0。
//
// ★ py 与 Go 的 `%-42s` 按**码点**补空格；`True/False` 是 py `str(bool)`；`frag[:40]` 按码点。
func cprSelftest(stdout io.Writer) bool {
	ph := cprPlaceholder()
	type cprCase struct {
		text string
		want bool
		why  string
	}
	cases := []cprCase{
		{"regex:" + ph + "\t" + ph + "\t说明", false, "规则表定义（模式文本，非残留）"},
		{ph + " 中文字目录/示例.json", true, "截断残留（带空格续部）"},
		{"配置见 " + ph + " 即可", false, "正常正文：后面是普通词"},
		{ph, false, "纯占位符，无余部"},
		{ph + "/中文字目录/示例", true, "无空格直连的续部"},
		{ph + "中文字目录/示例.md", true, "无空格直连的中文目录（含斜杠，形态成立）"},
		{ph + "\t私有卷路径（引号/反引号/换行）", false, "规则表定义列（占位符自成列 · 说明含斜杠）"},
	}
	bad := 0
	for _, c := range cases {
		got, frag := cprIsResidue(c.text)
		ok := got == c.want
		if !ok {
			bad++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintln(stdout, "   "+mark+" want="+cprPyPad(cprPyBool(c.want), 5)+
			" got="+cprPyPad(cprPyBool(got), 5)+" | "+cprPyPad(c.why, 42)+" | "+cprTruncRunes(frag, 40))
	}
	if bad == 0 {
		fmt.Fprintln(stdout, "   自测: 全过")
	} else {
		fmt.Fprintf(stdout, "   自测: %d 条失败\n", bad)
	}
	return bad == 0
}

// ── Q-176：`--json` 而本门**无机器读面** ⇒ 逐字说明后 rc=2（先于一切）──────────────────────

// cprJSONFields —— py `JSON_FIELDS`（空 · 唯一真源：印表与拒收同读这一处）。
var cprJSONFields = []string{}

// cprRefuseJSON —— py `refuse_json_without_machine_face()`：给了 `--json`/`--json=*` ⇒ rc=2。
func cprRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
	var hit []string
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
	if len(cprJSONFields) > 0 {
		fs := make([]string, 0, len(cprJSONFields))
		for _, f := range cprJSONFields {
			fs = append(fs, "`"+f+"`")
		}
		fmt.Fprintf(stderr, "   字段表：%s\n", strings.Join(fs, " · "))
	} else {
		fmt.Fprint(stderr, "   字段表：无（本门只出人读面）\n")
	}
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return 2, true
}

// ── `Q-176` 第五批：`--self-test` 三档自检（正控 0 · 负控 1 · 用法错 2）──────────────────────

// cprSelfTest —— py `self_test()`：在**临时 git 仓**里造件，真跑本入口取真退出码，退出即清。
//
// ★ 手段差（见未覆盖面⑤）：py `[sys.executable, SELF_PATH, target]` **再 spawn 自己**；
// 本件**进程内**直调 `checkPlaceholderResidueGateMain`。打印文案逐字同（子进程 stdout 被
// py `capture_output=True` 吞掉、全过时不进父 stdout）。
func cprSelfTest(stdout io.Writer) int {
	tmp, err := os.MkdirTemp("", "placeholder-residue-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "── %s --self-test 三档（正控 0 · 负控 1 · 用法错 2）\n", cprProg)
		fmt.Fprintln(stdout, "── 自检：**1 档不过** ✗")
		return 1
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	bad := 0
	ph := cprPlaceholder()

	chk := func(label string, want, rc int, out string) {
		good := rc == want
		if !good {
			bad++
		}
		tail := ""
		if !good {
			last := ""
			trimmed := strings.TrimSpace(out)
			if trimmed != "" {
				parts := strings.Split(trimmed, "\n")
				last = parts[len(parts)-1]
			}
			if len(last) > 90 {
				last = string([]rune(last)[:90])
			}
			tail = " ｜ " + last
		}
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", mark, label, want, rc, tail)
	}

	gitrun := func(d string, args ...string) {
		base := []string{"-c", "core.quotepath=false", "-C", d, "-c", "user.email=selftest@invalid",
			"-c", "user.name=selftest", "-c", "commit.gpgsign=false"}
		cprRunGit(append(base, args...)...)
	}
	repo := func(name, body string) string {
		d := cprPyJoin(tmp, name)
		_ = os.MkdirAll(d, 0o755)
		gitrun(d, "init", "-q")
		_ = os.WriteFile(cprPyJoin(d, "说明.md"), []byte(body), 0o644)
		gitrun(d, "add", "-A")
		gitrun(d, "commit", "-q", "-m", "fixture")
		return d
	}
	run := func(target string, args ...string) (int, string) {
		var b strings.Builder
		rc := checkPlaceholderResidueGateMain(&b, &b, append([]string{target}, args...))
		return rc, b.String()
	}

	fmt.Fprintf(stdout, "── %s --self-test 三档（正控 0 · 负控 1 · 用法错 2）\n", cprProg)
	clean := repo("clean", "路径见 "+ph+" 即可（后面是普通词，无余部）\n")
	rc, out := run(clean, "HEAD")
	chk("档① 正控：纯占位符、余部不是路径样 ⇒ 无残留", 0, rc, out)

	dirty := repo("dirty", "配置在 "+ph+" 中文字目录/示例.json 里\n")
	rc, out = run(dirty, "HEAD")
	chk("档② 负控：占位符后仍带路径样余部（截断残留）", 1, rc, out)

	// ★ 自指纪律：夹具里的完整形态一律**运行时**由 ph 拼出（正文不落整串）。
	caseD := repo("case", "跑 `"+ph+"\t"+ph+"\t说明`（规则表定义，形态文本非残留）\n")
	rc, out = run(caseD, "HEAD")
	chk("档①′ 成对正控：规则表定义行 ⇒ **不许红**（防假红）", 0, rc, out)

	// 自指纪律：私有正则前缀**拆开写**（本件落在 check-hardcoded-private-paths 的扫描面内）。
	desc := repo("desc", "regex:"+"/Volumes/"+`(AHZ|BXC)*`+"\t"+ph+"\t私有卷路径（停在引号/反引号/换行）\n")
	rc, out = run(desc, "HEAD")
	chk("档①″ 成对正控：定义列说明**含斜杠** ⇒ 不许红（现读假红的病灶）", 0, rc, out)

	rc, out = run(clean, "--no-such-flag")
	chk("档③ 用法错：未知旗标（输入错 ≠ 零命中）", 2, rc, out)

	rc, out = run("--json")
	chk("档③′ 用法错：`--json` 而本门无机器读面（`Q-176` 统一口）", 2, rc, out)

	// 成对负控：**取不到**（无 .git 物化树）⇒ 判不了 rc=2（不再静默当通过）。
	nogit := cprPyJoin(tmp, "no-git-tree")
	_ = os.MkdirAll(nogit, 0o755)
	_ = os.WriteFile(cprPyJoin(nogit, "说明.md"), []byte("配置在 "+ph+" 中文字目录/示例.json 里\n"), 0o644)
	rc, out = run(nogit, "HEAD")
	chk("档④ 负控：无 .git 物化树（`git grep` 取不到）⇒ 判不了 rc=2，**不再静默当通过**", 2, rc, out)

	paired := repo("dirty2", "配置在 "+ph+" 中文字目录/示例.json 里\n")
	rc, out = run(paired, "HEAD")
	chk("档④′ 成对正控：同一段正文在真 git 仓 ⇒ rc=1（证明档④ 不是恒 rc=2）", 1, rc, out)

	if bad == 0 {
		fmt.Fprintln(stdout, "── 自检：全过 ✓")
		return 0
	}
	fmt.Fprintf(stdout, "── 自检：**%d 档不过** ✗\n", bad)
	return 1
}

// ── 入口 ─────────────────────────────────────────────────────────────────────────────────

// checkPlaceholderResidueGateMain —— 本件唯一入口（= py `__main__`）。
func checkPlaceholderResidueGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc, hit := cprRefuseJSON(stderr, argv); hit {
		return rc
	}
	for _, a := range argv {
		if a == "--self-test" {
			return cprSelfTest(stdout)
		}
	}
	var unknown []string
	for _, a := range argv {
		if strings.HasPrefix(a, "-") && a != "--all" {
			unknown = append(unknown, a)
		}
	}
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "✗ 用法错（rc=2）：未知旗标 %s\n", strings.Join(unknown, " "))
		fmt.Fprintf(stderr, "  %s\n", cprUsage)
		return cprExitBlocked
	}
	if !cprSelftest(stdout) { // py: 自检前言恒打 stdout；不过 ⇒ rc=2
		fmt.Fprintln(stderr, "自测未过 ⇒ 脚本本身有问题，拒绝出结论")
		return cprExitBlocked
	}
	target := "."
	if len(argv) > 0 {
		target = argv[0]
	}
	mode := "HEAD"
	if len(argv) > 1 {
		mode = argv[1]
	}
	total, files, errMsg := cprScan(target, mode == "--all")
	if errMsg != "" {
		fmt.Fprintf(stderr, "✗ 不给结论（rc=2）：%s ⇒ **取不到 ≠ 零命中**（两态不许同形）；code=%s\n",
			errMsg, cprCodeTrackedName)
		return cprExitBlocked
	}
	if total == 0 {
		fmt.Fprintf(stdout, "结果: 通过——未发现残留（扫 %s mode=%s）\n", target, mode)
		return cprExitOK
	}
	fmt.Fprintf(stdout, "结果: 不通过——发现 %d 处残留（%d 个文件）：\n", total, len(files))
	// py: sorted(files.items(), key=lambda x: -len(x[1]))[:15] —— **稳定排序** ⇒ 同数保首次遇到序。
	sorted := make([]cprFileHits, len(files))
	copy(sorted, files)
	sort.SliceStable(sorted, func(i, j int) bool { return len(sorted[i].hits) > len(sorted[j].hits) })
	if len(sorted) > 15 {
		sorted = sorted[:15]
	}
	for _, f := range sorted {
		fmt.Fprintf(stdout, "   %s\n", f.path)
		lim := len(f.hits)
		if lim > 3 {
			lim = 3
		}
		for _, h := range f.hits[:lim] {
			fmt.Fprintf(stdout, "      %s\n", h)
		}
	}
	return cprExitFail
}
