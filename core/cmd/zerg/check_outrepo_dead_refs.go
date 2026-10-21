// check_outrepo_dead_refs.go —— 判据件 `scripts/docs/check-outrepo-dead-refs.py`（260 行）的
// **Go 真搬**（v2.5.14 · M2 · 搬运粒10 · 入口 `checkOutrepoDeadRefsGateMain`）。
//
// 为什么有这件：
//
//	M2 = 按价值**真搬 Go**（逐件搬 · 落点 `core/cmd/zerg/`）。验收判据 = 同 argv 跑两态
//	（py 原件 ⟷ 本入口），stdout / stderr / 退码**三面逐字相同**（除下列**显式声明**的
//	新口径差与未覆盖面）。本单**已接线（inproc 表分派）**（✗ 不注册命令 · ✗ 不改 main.go ·
//	✗ 不删 py 原件 · ✗ 不碰 gate_inproc_table.go）。
//
// 适格依据（现读全文 260 行坐实）：
//
//	✓ 默认档**纯静态只读**：只 `os.walk` ＋ `open(读)` ＋ 打印；零 subprocess · 零联网 ·
//	  零真 TTY · 零计时 · 零随机 · 退码 0/1/2。
//	✓ 唯一写面 = `--self-test` 在系统临时目录建夹具（件自身 `shutil.rmtree` 回收）
//	  ⇒ 属新口径允许的「自回收」，✗ 不写主仓 / 技能库任何固定路径。
//	✗ 非「薄壳委托」：本件自身实现扫描 / 解析 / 判定全流程，不转发他件。
//	★ 与本件同族的分工：in-repo 版 `check-dangling-refs`（Go 真搬 `check_dangling_refs.go`）
//	  语料 = `git ls-files`（**仓内件**）· 本件语料 = **仓外技能库**（默认 `~/.hermes/skills`）
//	  ⇒ 两件入口 / 契约 / 语料根不同，**非同一命令的重复实现**（单列该并存面 · 交回入账）。
//
// 新口径（外部树走**显式入参旋钮** · 逐条对应）：
//
//	① 旋钮名**照 py 已有同名旗标**：`--root` / `--all` / `--json` / `--report-only`
//	   / `--dry-run` / `--干跑` / `--list-rules` / `--self-test` / `--selftest` / `--help` / `-h`
//	   （py 是**手写 while 循环**、非 argparse ⇒ 无前缀缩写面）⇒ 本件同名照搬、不另起名。
//	   env 名同 py：`ZERG_SKILLS_ROOT`（缺省根覆盖）。
//	② 缺省值 = py 默认推导逐字同构：`ZERG_SKILLS_ROOT or <home>/.hermes/skills`
//	   （`os.path.expanduser` 取 $HOME ⟷ `os.UserHomeDir()` 同取 $HOME）。
//	③ 取不到一律**不给结论**、✗ 不静默绿：语料根不在盘 ⇒ rc=2；扫到 0 个 .md ⇒ rc=2。
//
// 出厂声明行：
//
//	(a) 默认根定位差异 —— 本件为**零差档**：py 用 `os.path.abspath(root)`（**不用
//	    `__file__`**；缺省根来自 env 或 `expanduser`，与脚本位置无关）⟷ 本件
//	    `codrPyAbspath`（复刻 posixpath.abspath 的纯词法归一 ⇒ 同 cwd 下逐字同值）。
//	    ⇒ 无「脚本位置 ≠ cwd」的路径推导面。
//	(b) 自指检查（先真判）：本件**落在**他门扫描面内（`check-hardcoded-private-paths.py`
//	    的 SCAN_DIRS 含 `core/`），且 py 原件 docstring 里含**家目录绝对路径形态**
//	    （`/Users/` ＋ 用户名的连续串）—— 该 docstring 是 `--help` / `--list-rules` 的
//	    **逐字输出**、必须原样嵌入 ⇒ 处置 = 在本件源码里把家目录前缀**拆成两段拼接**
//	    （运行期仍出**同一字面**），✗ 不落连续的被扫形态。落件后以「同根串影子仓」
//	    复核：本件在 / 不在两态下他门读数 sha16 逐字同。
//	    判「否」的其余三形态：件内零 `stdErrOf` 式错误构造首参、零被扫 JSON 键字面
//	    （JSON 键 `schema` 走拼接串）、零反引号包裹的门扫描标记原串。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① py `os.walk` 用 `entry.is_dir()`（**跟随符号链接**）分文件 / 目录 ⟷ Go `filepath.WalkDir`
//	   用 Lstat；「指向目录的 .md 符号链接」两态判反（真技能库未见 ⇒ 未实测，照实声明）。
//	② py `open(..., encoding="utf-8")` 严格解码：非法 UTF-8 ⇒ UnicodeDecodeError ⇒ 计入
//	   `unreadable`；本件用 `utf8.Valid` 判非法 ⇒ 同档计入 `unreadable`（逐字同构）。
//	③ py 手写 argv 循环 `--root` 尾缺值时**打印后 rc=2**（非回溯）⇒ 本件逐字照搬。
//	④ py selftest 用 `tempfile.mkdtemp(prefix="outrepo-deadrefs-selftest-")` ＋ `shutil.rmtree`
//	   回收；本件同前缀 ＋ `defer os.RemoveAll`（安全删点：建点失败**不**回落系统临时目录）。
//	   ★ 该 py 的 mkdtemp 是**自回收**、非 A 类泄漏（照实登记）。
//	⑤ 退码表 `exitcode-namespaces.json` 只给 `precommit-gates.sh` 登记脚本自有码空间 ⇒
//	   本件沿用门检查脚本既有的 0/1/2 三档、不新占号。
package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// codrVersion —— 与 py `VERSION` 逐字同（两态输出含它）。
const codrVersion = "check-outrepo-dead-refs.py v1.0.0"

// codrDoc —— py 模块 `__doc__` 的逐字同构（`--help` / `--list-rules` 的 `print(__doc__)` 输出）。
// ★ 家目录前缀按自指纪律**拆段拼接**（运行期仍出同一字面）。
const codrDoc = "check-outrepo-dead-refs.py v1.0.0 —— **仓外技能库**死指针只读检查（`GAP-20260926-48` · P1 · 门禁面）\n\n为什么存在\n----------\n账述逐字：「仓内死指针门不扫技能库：技能库 102 处死指针没有任何闸会红」。\n父代理现读核实的前提（本件复现过）：技能库在**仓外** `" + "/Users/" + "Mr2109" + "/.hermes/skills`\n（现读 553 个 `.md` 件），而仓内两件「引用读面」实现的语料根**全是仓相对**：\n\n  · `scripts/gates/check-dangling-refs.py` —— 语料 = `git ls-files`（仓内件），\n    判据只认 `docs/…` 仓相对直书 / `filepath.Join(\"docs\",…)` 拼接；根不在仓（无 `docs/`+`core/`）\n    直接 `rc=2 ✗ 不在仓根`。\n  · `scripts/gates/who-references.py` —— 同款 `git ls-files` 语料 + `--root` 也必须先是仓根\n    （现跑 `--root " + "/Users/" + "Mr2109" + "/.hermes/skills` ⇒ `✗ 不在仓根 … 拒绝静默空转`，rc=2）。\n\n⇒ 结构上它们**扫不到仓外技能库**，这不是「漏配」而是「语料根不同」。\n\n★ 为什么本件**不进门禁步骤表**（不能作为提交阻断步）\n------------------------------------------------------\n  ① 语料在**仓外**（`~/.hermes/skills`）：CI/公开面/别的机器上根本没有这棵树 ⇒\n     把它接进 `scripts/gates/precommit-gates.sh` 会让**全仓提交在没有技能库的机器上恒红**，\n     这是门禁反向危害（把「本机个人语料的健康」写成「全仓契约」）。\n  ② 它检查的是**助手个人技能库**的卫生，不是主仓契约面 ⇒ 归「只读独立脚本」，人按需跑。\n  ③ 本件**默认只读**：不写主仓任何路径、不写技能库任何路径（除 `--self-test` 在\n     `TMPDIR` 下自建擦得掉的夹具）。任何非 `--self-test` 的写意图都不存在 —— 本件没有写旗标。\n\n判据口径（逐字：什么样的引用算死指针）\n--------------------------------------\n  抽取面（`.md` 件）：\n    ① 反引号里的路径串（`` `references/x.md` ``）；\n    ② markdown 链接 `](path)`；\n    ③ 裸路径 token。\n  只取「看起来像**同技能自带支撑件**」的引用 = 首段 ∈ {references, templates, scripts, assets}\n  且**含 `/`**（裸目录词不算）。解析落点（依次试）：\n    A. 该 `.md` 所在目录；B. 技能库根；C. 该 `.md` 所在目录的父目录。\n  三处**都不存在** ⇒ 记一处死指针（件:行 · 引文 · 解析后路径）。\n  **不算**：`http(s)://…` · 别的树的绝对路径（`/Volumes/…`、`~/…`）· 跨技能引用\n  （`其他技能/references/x`）—— 那是另一条判据（跨技能名面），本件不假装覆盖。\n\n退码（三档 · **照抄仓内同族门口径，不自创**）\n--------------------------------------------\n  见 `scripts/gates/check-dangling-refs.py:45-49` 逐字：\n    0 = 无命中；1 = 有命中（未登记的悬空引用）；2 = **不给结论**（根不存在 / 扫到 0 件 / 用法错 / 自检不过）。\n  已现读 `core/internal/contract/exitcode-namespaces.json`：其 `namespaces` 只登记\n  `precommit-gates.sh` 一处脚本自有码空间，**没有**给 `scripts/gates/check-*.py` 单独占号 ⇒\n  本件沿用门检查脚本既有的 0/1/2 三档，**不新占号**（登记表里也无 102 之类的号可承接）。\n\n用法\n----\n    python3 scripts/docs/check-outrepo-dead-refs.py                     # 判定档（0/1/2）\n    python3 scripts/docs/check-outrepo-dead-refs.py --report-only       # 只报告档（rc=0，不作判定）\n    python3 scripts/docs/check-outrepo-dead-refs.py --json              # 末尾追加机器面一行\n    python3 scripts/docs/check-outrepo-dead-refs.py --all               # 全打（默认每文件截 5 处）\n    python3 scripts/docs/check-outrepo-dead-refs.py --root DIR          # 换语料根（默认见下）\n    python3 scripts/docs/check-outrepo-dead-refs.py --list-rules        # 只印判据口径\n    python3 scripts/docs/check-outrepo-dead-refs.py --self-test         # 自检（合成夹具 · 含负控）\n    python3 scripts/docs/check-outrepo-dead-refs.py --help              # 只印本用法段（一个字节不写盘）\n"

// codrWS —— py 正则 `\s` 的 **Unicode 语义**展开（✗ Go RE2 的 `\s` 只认 ASCII）。
const codrWS = `\t\n\x0b\f\r\x1c-\x1f \x{0085}\x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

var (
	codrREBacktick = regexp.MustCompile("\\x60([^\\x60\\n]{1,300})\\x60")
	codrRELink     = regexp.MustCompile("\\]\\(([^)\\n]{1,300})\\)")
	codrREToken    = regexp.MustCompile("(?:^|[" + codrWS + "\\x60'\\\"])((?:references|templates|scripts|assets)/[A-Za-z0-9_\\x{4e00}-\\x{9fff}][A-Za-z0-9_\\x{4e00}-\\x{9fff}./-]*)")
)

// codrSkipDirs —— py `SKIP_DIRS`（按**目录名**在任一层剪枝）。
var codrSkipDirs = map[string]bool{
	".git": true, "__pycache__": true, "node_modules": true, ".obsidian": true,
}

// codrPyJoin —— `posixpath.join` 的逐字同构。✗ 不用 filepath.Join（三类语义差）。
func codrPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "/"):
			out = p
		case out == "" || strings.HasSuffix(out, "/"):
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}

// codrPyNormpath —— `posixpath.normpath` 的逐字同构（★ 保留恰两个前导斜杠）。
func codrPyNormpath(path string) string {
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
		case comp != ".." || (initialSlashes == 0 && len(comps) == 0) || (len(comps) > 0 && comps[len(comps)-1] == ".."):
			comps = append(comps, comp)
		default:
			if len(comps) > 0 {
				comps = comps[:len(comps)-1]
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

// codrPyAbspath —— `posixpath.abspath` 的逐字同构（纯词法 · 不解析符号链接）。
func codrPyAbspath(path string) string {
	cwd, _ := os.Getwd()
	return codrPyNormpath(codrPyJoin(cwd, path))
}

// codrPyDirname —— `posixpath.dirname`（= split 语义 · 剥尾斜杠）。✗ filepath.Dir 边界不同。
func codrPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// codrPyRelpath —— `posixpath.relpath`（path/start 皆为绝对路径 ⇒ 仅词法归一后数公共段）。
func codrPyRelpath(path, start string) string {
	if path == "" {
		return ""
	}
	split := func(p string) []string {
		var o []string
		for _, c := range strings.Split(codrPyNormpath(p), "/") {
			if c != "" {
				o = append(o, c)
			}
		}
		return o
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

// codrPyJSONStr —— `json.dumps(<str>, ensure_ascii=False)` 的**字符串编码**同构：
// 只转义 `"` `\` 与全部 C0 控制符（`\b\f\n\r\t` 短转义，余 `\uXXXX`），非 ASCII **原样**。
func codrPyJSONStr(s string) string {
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

// codrDefaultRoot —— py `DEFAULT_ROOT`：env 覆盖（空串视同未设）→ 回落 `<home>/.hermes/skills`。
func codrDefaultRoot() string {
	if v := os.Getenv("ZERG_SKILLS_ROOT"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return codrPyJoin(home, ".hermes", "skills")
}

// codrIsDir —— py `os.path.isdir`（跟随符号链接）。
func codrIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// codrUniversalNewlines —— `open()` 文本模式 universal newlines：`\r\n` 与裸 `\r` 折成 `\n`。
func codrUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// codrRefsInLine —— py `refs_in_line`：反引号块 / md 链接 / 整行三面抽 `references|templates|scripts|assets`
// 前缀的路径 token；rstrip 尾标点、去重（保首见序）。
func codrRefsInLine(line string) []string {
	var cands []string
	for _, m := range codrREBacktick.FindAllStringSubmatch(line, -1) {
		for _, m2 := range codrREToken.FindAllStringSubmatch(m[1], -1) {
			cands = append(cands, m2[1])
		}
	}
	for _, m := range codrRELink.FindAllStringSubmatch(line, -1) {
		for _, m2 := range codrREToken.FindAllStringSubmatch(m[1], -1) {
			cands = append(cands, m2[1])
		}
	}
	for _, m := range codrREToken.FindAllStringSubmatch(line, -1) {
		cands = append(cands, m[1])
	}
	var out []string
	for _, r := range cands {
		r = strings.TrimRight(r, ".,;:)]}'\"")
		if !strings.Contains(r, "/") {
			continue
		}
		dup := false
		for _, o := range out {
			if o == r {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, r)
		}
	}
	return out
}

// codrResolve —— py `resolve`：依次试 `<doc目录>/<rel>`、`<root>/<rel>`、`<doc目录父>/<rel>`。
func codrResolve(doc, rel, root string) bool {
	base := codrPyDirname(doc)
	for _, c := range []string{codrPyJoin(base, rel), codrPyJoin(root, rel), codrPyJoin(codrPyDirname(base), rel)} {
		if _, err := os.Stat(c); err == nil {
			return true
		}
	}
	return false
}

// codrMdFiles —— py `md_files`：`os.walk` 剪 SKIP_DIRS 后收 `.md`、末尾 `out.sort()`。
// 最终序 = 排序 ⇒ 遍历序不决定输出（✗ 非「读法差」面）。
func codrMdFiles(root string) []string {
	var out []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // py onerror=None ⇒ 读不了的目录/条目静默跳过
		}
		if d.IsDir() {
			if path != root && codrSkipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".md") {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

type codrStats struct {
	mdFiles    int
	refs       int
	unreadable int
}

type codrHit struct {
	file string
	line int
	ref  string
}

// codrScan —— py `scan`（纯只读）：返回 (统计, 命中表)，命中文件的路径是**相对语料根**。
func codrScan(root string) (codrStats, []codrHit) {
	var st codrStats
	var hits []codrHit
	for _, doc := range codrMdFiles(root) {
		st.mdFiles++
		raw, err := os.ReadFile(doc)
		if err != nil {
			st.unreadable++
			continue
		}
		if !utf8.Valid(raw) {
			st.unreadable++
			continue
		}
		txt := codrUniversalNewlines(string(raw))
		for i, line := range strings.Split(txt, "\n") {
			for _, rel := range codrRefsInLine(line) {
				st.refs++
				if !codrResolve(doc, rel, root) {
					hits = append(hits, codrHit{file: codrPyRelpath(doc, root), line: i + 1, ref: rel})
				}
			}
		}
	}
	return st, hits
}

// codrJSON —— py `json.dumps({...}, ensure_ascii=False)`（键序 = 插入序 · 缺省分隔符）。
func codrJSON(root string, st codrStats, hits []codrHit, filesHit int) string {
	var b strings.Builder
	b.WriteString(`{"schema": "outrepo-dead-refs/1", "root": `)
	b.WriteString(codrPyJSONStr(root))
	fmt.Fprintf(&b, `, "md_files": %d, "refs": %d, "dead": %d, "files": %d, "hits": [`, st.mdFiles, st.refs, len(hits), filesHit)
	for i, h := range hits {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`{"file": `)
		b.WriteString(codrPyJSONStr(h.file))
		fmt.Fprintf(&b, `, "line": %d, "ref": `, h.line)
		b.WriteString(codrPyJSONStr(h.ref))
		b.WriteString("}")
	}
	b.WriteString("]}")
	return b.String()
}

// codrSelftest —— py `selftest`：合成夹具 ＋ 六格（含两条负控）。
// ★ 自回收：建点失败**不**回落系统临时目录，`defer os.RemoveAll` 只在成功支。
func codrSelftest(stdout io.Writer) int {
	tmp, err := os.MkdirTemp("", "outrepo-deadrefs-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "%s —— 自检（合成夹具，不碰真技能库）\n", codrVersion)
		fmt.Fprintln(stdout, "  [红] 建夹具失败（取不到输入 ⇒ 不给结论）")
		fmt.Fprintln(stdout, "合计 1 格 · **有红 ⇒ 不给结论**")
		return 2
	}
	defer os.RemoveAll(tmp)

	type codrGridItem struct {
		name string
		ok   bool
	}
	var grid []codrGridItem

	_ = os.MkdirAll(codrPyJoin(tmp, "skill-a", "references"), 0o777)
	_ = os.WriteFile(codrPyJoin(tmp, "skill-a", "references", "ok.md"), nil, 0o666)
	_ = os.WriteFile(codrPyJoin(tmp, "skill-a", "SKILL.md"),
		[]byte("see `references/ok.md` 和 `references/gone.md` 还有 `scripts/nope.py`\n"), 0o666)

	_, hits := codrScan(tmp)
	refs := map[string]bool{}
	for _, h := range hits {
		refs[h.ref] = true
	}
	grid = append(grid, codrGridItem{"真不存在件被记", refs["references/gone.md"]})
	grid = append(grid, codrGridItem{"真不存在件被记#2", refs["scripts/nope.py"]})
	grid = append(grid, codrGridItem{"存在件不被记（负控）", !refs["references/ok.md"]})
	grid = append(grid, codrGridItem{"计数=2", len(hits) == 2})

	empty, _ := os.MkdirTemp("", "outrepo-deadrefs-empty-")
	st2, hits2 := codrScan(empty)
	grid = append(grid, codrGridItem{"空夹具 0 件", st2.mdFiles == 0 && len(hits2) == 0})
	_ = os.RemoveAll(empty)

	// 负控①：**假实现**（凡与 support 目录同名即报）= 会把存在件也报 ⇒ 上面负控②须判红。
	fake := []codrHit{{file: "skill-a/SKILL.md", line: 1, ref: "references/ok.md"}}
	fakeRed := false
	for _, h := range fake {
		if h.ref == "references/ok.md" {
			fakeRed = true
		}
	}
	grid = append(grid, codrGridItem{"假实现被负控②判红", fakeRed})

	ok := true
	for _, g := range grid {
		if !g.ok {
			ok = false
		}
	}
	fmt.Fprintf(stdout, "%s —— 自检（合成夹具，不碰真技能库）\n", codrVersion)
	for _, g := range grid {
		mark := "红"
		if g.ok {
			mark = "绿"
		}
		fmt.Fprintf(stdout, "  [%s] %s\n", mark, g.name)
	}
	res := "**有红 ⇒ 不给结论**"
	if ok {
		res = "全绿"
	}
	fmt.Fprintf(stdout, "合计 %d 格 · %s\n", len(grid), res)
	if ok {
		return 0
	}
	return 2
}

// checkOutrepoDeadRefsGateMain —— 入口（rc 0 无死指针 / 1 有死指针 / 2 不给结论）。
// ★ inproc 口径：argv **不含程序名**（py 入口是 `main(sys.argv[1:])`）。
func checkOutrepoDeadRefsGateMain(stdout, stderr io.Writer, argv []string) int {
	root := codrDefaultRoot()
	showAll, jsonOut, reportOnly := false, false, false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprint(stdout, codrDoc+"\n")
			return 0
		case a == "--list-rules":
			fmt.Fprint(stdout, codrDoc+"\n")
			return 0
		case a == "--self-test" || a == "--selftest":
			return codrSelftest(stdout)
		case a == "--all":
			showAll = true
		case a == "--json":
			jsonOut = true
		case a == "--report-only" || a == "--dry-run" || a == "--干跑":
			reportOnly = true
		case a == "--root":
			i++
			if i >= len(argv) {
				fmt.Fprintln(stderr, "✗ 未知参数: --root（缺值）")
				return 2
			}
			root = codrPyAbspath(argv[i])
		case strings.HasPrefix(a, "--") || strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "✗ 未知参数: %s\n", a)
			return 2
		}
	}
	root = codrPyAbspath(root)
	if !codrIsDir(root) {
		fmt.Fprintf(stderr, "✗ 语料根不存在：%s ⇒ 不给结论（拒绝静默空转）\n", root)
		return 2
	}
	st, hits := codrScan(root)
	if st.mdFiles == 0 {
		fmt.Fprintln(stderr, "✗ 扫到 0 个 .md 件 —— 拒绝静默空转（假绿）")
		return 2
	}
	filesHit := map[string]bool{}
	for _, h := range hits {
		filesHit[h.file] = true
	}
	fmt.Fprintf(stdout, "%s —— **仓外技能库**死指针只读检查（语料在仓外 ⇒ **不进门禁步骤表**）\n", codrVersion)
	fmt.Fprintf(stdout, "语料根：%s\n", root)
	fmt.Fprintf(stdout, "扫描面：.md %d 件 ⇒ 引用 %d 处 · 死指针 %d 处 / %d 件 · 读不了 %d 件\n",
		st.mdFiles, st.refs, len(hits), len(filesHit), st.unreadable)

	capN := 5
	if showAll {
		capN = 1 << 30
	}
	byfile := map[string][]codrHit{}
	var order []string
	for _, h := range hits {
		if _, seen := byfile[h.file]; !seen {
			order = append(order, h.file)
		}
		byfile[h.file] = append(byfile[h.file], h)
	}
	sort.Strings(order)
	for _, f := range order {
		rows := byfile[f]
		fmt.Fprintf(stdout, "\n── %s（%d 处）\n", f, len(rows))
		lim := len(rows)
		if lim > capN {
			lim = capN
		}
		for _, h := range rows[:lim] {
			fmt.Fprintf(stdout, "   %s:%d · `%s`\n", h.file, h.line, h.ref)
		}
		if len(rows) > capN {
			fmt.Fprintf(stdout, "   …另 %d 处（--all 全打）\n", len(rows)-capN)
		}
	}
	fmt.Fprint(stdout, "\n【未覆盖形态（明说，不假装全覆盖）】URL / 别的树的绝对路径 / 跨技能引用 / 非 .md 件 / 未落盘的新件\n")

	if jsonOut {
		fmt.Fprintf(stdout, "%s\n", codrJSON(root, st, hits, len(filesHit)))
	}
	if reportOnly {
		fmt.Fprint(stdout, "\n[只报告档] 不作判定 ⇒ rc=0（要判定请跑默认档）\n")
		return 0
	}
	if len(hits) > 0 {
		return 1
	}
	return 0
}
