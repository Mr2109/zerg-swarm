// check_unsubstituted_placeholders.go —— 门禁件 `scripts/gates/check-unsubstituted-placeholders.py` 的 **Go 真搬**
// （v2.5.14 · M2-0 · 路C续2 · 候选区间首字母 **q–z**）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据 · 4.1）路C续2 的一枚：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它 / 不选它的理由（4.1 M2-0 = 「无输入物料、纯静态检查的判据（读仓内件即可）」）：
//
//	✓ 选 `check-unsubstituted-placeholders.py`（302 行）：只读**仓内** `docs/**/*.md` 与
//	  `publish/docs/*.md`（**无外部树依赖**：不碰 Zerg-内部文档 文档仓 / 公开树生成物 / 在线面）·
//	  输出**定序**（两条 glob 结果逐条 `sorted()`、最终 `sorted(out)` ⇒ 全序；Go 侧同名升序同序）·
//	  退码 `0/1/2` 三档齐（0 无残留 / 1 有残留 / 2 未知旗标与 `--json` 用法错）·
//	  **纯静态无写面**（除 `/var/tmp` 自检夹具外不写盘 · 不起后台 · 不联网）· 行数适中 ·
//	  **无人正在改**（`git status` 现读不含它）。
//	✗ 不选 `check-slice.py`（2206 行）/ `check-trustroot-witness.py`（816 行）/ `check-verdict-record.py`
//	  （736 行）/ `check-zh-en.py`（691 行）/ `check-zero-side-effect.py`（657 行）：**重件**（行数不适中）。
//	✗ 不选 `check-wallclock-ratchet.py`（232 行）：默认档外带**写通道**（`--write` 就地改基线件 +
//	  append-only 台账）⇒ 不属「**纯**静态无写面」；且要调用方给 `--measured` / `--from-bench`（**输入物料**）。
//	✗ 不选 `check-signature-identity.py`（288 行）：真身面要**现跑 `codesign` 子进程**并读 `bin/`
//	  产出件（外部命令面 + 外部制品）⇒ 不是「纯静态」，且读数随机器签名档变（非确定）。
//	✗ 不选 `check-wired-scripts.py` / `check-writer-guard.py`：前者读**在飞登记面**，后者是**共享件**
//	  （本轮铁律②点名禁改面）；`check-replace-rule-fixtures.py` 要现跑子进程比对 ⇒ 非纯静态。
//	✗ 不选 `who-references.py` / `seal-ledger.py` / `two-states-gate-mutate.py` / `repo-diffstat-check.py`：
//	  都是**仓外/全树遍历 + git 子进程**形态（`os.walk` 序非确定、外部命令面）⇒ 不满足「输出有序 / 纯静态」。
//	✗ 不选其余 q–z 件（`check-service-declaration.py` / `check-symbol-delete-ripple.py` /
//	  `check-two-pass-identity.py` / `check-tty-output-contract.py` / `check-token-argv.py`）：要
//	  `bin/zerg` 现跑或读生成物（外部制品 + 子进程）⇒ 非「纯静态检查」。本轮**未跨区**取样。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-unsubstituted-placeholders.py` 原件（退场规则 4.3：**对拍通过后才**退场）；
//	· **不**改任何登记面（`公开标记.tsv` / `mirror-public-lib.py` / `publish-public.sh` /
//	  `gate-coverage.config.json` / `judge-registry.json` / `dryrun-semantics.json`）；
//	· 入口 `unsubstitutedPlaceholdersGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func unsubstitutedPlaceholdersGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 全过 / 1 有残留 / 2 用法错（未知旗标 · 多余位置参数 · `--json` 无机器面）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ★ 标准声明（一）· **默认根定位**：
//
//	本门 py 侧**不用** `__file__` 上溯 —— `main()` 里 `root = os.path.abspath(a.root) if a.root
//	else find_root(os.getcwd())`，即**缺省根从 `os.getcwd()` 起上溯**；Go 入口同样把缺省起点取
//	**当前工作目录**（`os.Getwd()`）。⇒ **两面同形，无差异**（依据：对拍 C9「cwd=夹具根 · 不给根参数」
//	rc/stdout/stderr 三面逐字节同）。这与前几枚样板（py 从 `__file__` 所在目录上溯）**不同** ——
//	照实声明，不套模板。
//
// ★ 标准声明（二）· **自指检查**：**否** —— 本件判面由两条 glob **写死**在
// `docs/**/*.md` 与 `publish/docs/*.md`（两者都以仓根为基），本搬件落在 `core/cmd/zerg/`、
// 原件落在 `scripts/gates/`，**两面都不在判面内**（判面只吃 `.md`，且不含 `core/`、`scripts/`）。
// 依据（**真判**，不是照抄）：本件**落仓前后**在真仓上跑原件 py，**55 件**判面清单与「命中 0 处」
// 逐字不变（`real_before.txt` / `real_after.txt` 与 `git-status-*.txt` 对读）—— 即本件的落地
// **不改变判面取数**，自指面**不存在**，故无须「标记字面禁反引号」那类处置。
//
// 已对拍面（仓外夹具两态逐字）：全绿（指定根 / 缺省 cwd）· 句面残留 `{counts}`（rc=1 逐件逐行点名）·
// 成对半边（同夹具删占位 ⇒ 0）· 路由模板假红对照（裸 / 行内代码）· 代码区假红对照（围栏 / 行内）·
// 空转（扫面 0 件）· 取不到输入（根不存在）· 读不了件（`chmod 000`）· 未知旗标 · 多余位置参数 ·
// `--root` 缺值 · `--json` 拒收（两面形态）· `--list-rules` · `--self-test` · `-h` / `--help` · 真仓现读。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse 的**其余**用法错正文（`--root=` 的 `ignored explicit argument`、选项缩写 `--ro`
//	   （py `allow_abbrev=True` 会收）/ `--` 分隔符 / `-x` 之类短选项的归并）—— 本件只逐字对齐
//	   `unrecognized arguments` 与 `argument --root: expected one argument` 两档。
//	② `-h/--help` 正文按 **`COLUMNS` 环境变量 / 终端宽度**换行（py `HelpFormatter` 取
//	   `get_terminal_size().columns - 2`）。本件把非 tty（宽度 78）下的正文逐字固化；设了 `COLUMNS`
//	   的调用方读数可能与本件不同（对拍一律在非 tty、`COLUMNS` 未设下做）。
//	③ **异常路径**：件是**目录**（py `IsADirectoryError`）/ 其他 errno（ENOTDIR、ELOOP…）—— 本件
//	   只把 ENOENT / EACCES / EISDIR 三种 errno 逐字翻成 py 的 `[Errno N] 说明: '路径'` 形态，
//	   其余 errno 落 Go 原生错误串（与 py 的 `strerror` 措辞不同）。
//	④ **非 UTF-8 件**：py `open(...,encoding="utf-8",errors="replace")` 按 Unicode「最大子部件」
//	   规则替换；本件同样走最大子部件，但**非法起始字节后跟合法续字节**（如 `C0 80`）的替换个数
//	   与 py 可能不等（本件合一个 U+FFFD，py 两个）⇒ 正文另论。
//	⑤ **符号链接**：py `glob(recursive=True)` 会跟进**目录符号链接**；本件走 `os.ReadDir` 不跟进
//	   （真仓 `docs/` 下现读**无**符号链接 ⇒ 不影响对拍）。
//	⑥ py `str.strip()/lstrip()` 认 `\x1c`–`\x1f` 为空白，Go `unicode.IsSpace` 不认 ⇒ 含这几个
//	   控制字符的件，剥离粒度与打印正文可能不同（缩进/行首判定同理）。
//	⑦ `--self-test` 格 7 的正文里带 `tempfile.mkdtemp` 的**随机后缀绝对路径** ⇒ **原件自身两跑
//	   也不同**（对拍表已按此单列点名：属原件非确定面，不属本件差异）。
//
//	—— ①②③④⑤⑥⑦ 各面均**不在** M2-0 的样板对拍输入集内；接线前须逐面补齐或显式剔除。
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// upProg —— py `argparse` 的 `prog`（= `os.path.basename(sys.argv[0])`）逐字。
const upProg = "check-unsubstituted-placeholders.py"

// upVersion —— py `VERSION` 逐字（`--list-rules` 首行 + `argparse` 的 `description`）。
const upVersion = "check-unsubstituted-placeholders.py v1.0（`Q-166` / 块C §六④ · 2026-09-24 落件）"

// upPATSource —— py `PAT.pattern` 逐字（`re` 只回原串 ⇒ 打印面一致）。
const upPATSource = `\{[A-Za-z_][A-Za-z0-9_.]*\}`

// upMaxPrint —— py `MAX_PRINT` 逐字。
const upMaxPrint = 200

var (
	// upPAT —— py `PAT = re.compile(r"\{[A-Za-z_][A-Za-z0-9_.]*\}")` 逐字。
	upPAT = regexp.MustCompile(upPATSource)
	// upFaceGlobs —— py `FACE_GLOBS` 逐字（顺序即拼接打印顺序）。
	upFaceGlobs = []string{"docs/**/*.md", "publish/docs/*.md"}
)

// upHit —— py 命中项 `(rel, lineno, token, line.strip())` 的对应体。
type upHit struct {
	Rel  string
	Line int
	Tok  string
	Text string
}

// ── argparse 面（逐字对齐 py 3.9 `HelpFormatter` 非 tty（宽 78）下的正文）──────────────
func upUsage() string {
	// 续行缩进 = len("usage: ") + len(prog) + 1 = len(prog) + 8（py `_format_usage` 的 prefix 口径）
	return "usage: " + upProg + " [-h] [--root ROOT] [--self-test]\n" +
		strings.Repeat(" ", len(upProg)+8) + "[--list-rules]\n"
}

func upHelp() string {
	return upUsage() + "\n" + upVersion + "\n\noptional arguments:\n" +
		"  -h, --help    show this help message and exit\n" +
		"  --root ROOT   判据树 / 仓根（默认：cwd 上溯找同时有 docs/ 与 publish/ 的那一层）\n" +
		"  --self-test   只跑自检（合成夹具 · 不碰真目标）\n" +
		"  --list-rules  打印判据 / 判据面 / 排除 / 退码\n"
}

// upRefuseJSON —— py `refuse_json_without_machine_face()` 逐字：**先于** argparse 判
// `--json` / `--json=*`（本门 `JSON_FIELDS = []` ⇒ 无机器读面 ⇒ rc=2）。
func upRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
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
	fmt.Fprintln(stderr, "   字段表：无（本门只出人读面）")
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	return 2, true
}

func upArgError(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, upUsage())
	fmt.Fprintf(stderr, "%s: error: %s\n", upProg, msg)
	return 2
}

// ── 取数面 ──────────────────────────────────────────────────────────────────────
func upIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func upIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// upFindRoot —— py `find_root(start)` 逐字：上溯找同时有 `docs/` 与 `publish/` 的那一层。
func upFindRoot(start string) string {
	d, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if upIsDir(filepath.Join(d, "docs")) && upIsDir(filepath.Join(d, "publish")) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// upGlob —— py `glob.glob(<root>/<pattern>, recursive=True)` 的定序对应体。
//
//	`docs/**/*.md`    ⇒ 递归（**不**进隐藏目录 / 隐藏件：py `include_hidden=False` 同口径）
//	`publish/docs/*.md` ⇒ 只一级（同上跳过隐藏件）
func upGlob(root, pattern string) []string {
	res := []string{}
	switch pattern {
	case "docs/**/*.md":
		upWalkMD(filepath.Join(root, "docs"), &res)
	case "publish/docs/*.md":
		base := filepath.Join(root, "publish", "docs")
		ents, err := os.ReadDir(base)
		if err != nil {
			return res
		}
		for _, e := range ents {
			n := e.Name()
			if strings.HasPrefix(n, ".") || !strings.HasSuffix(n, ".md") {
				continue
			}
			res = append(res, filepath.Join(base, n))
		}
	}
	sort.Strings(res)
	return res
}

func upWalkMD(dir string, res *[]string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		n := e.Name()
		if strings.HasPrefix(n, ".") {
			continue
		}
		p := filepath.Join(dir, n)
		if e.IsDir() {
			upWalkMD(p, res)
			continue
		}
		if strings.HasSuffix(n, ".md") {
			*res = append(*res, p)
		}
	}
}

// upScanFiles —— py `scan_files(root)` 逐字（去重 → 转仓根相对路径 → `sorted`）。
func upScanFiles(root string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, g := range upFaceGlobs {
		for _, p := range upGlob(root, g) {
			if seen[p] || !upIsFile(p) {
				continue
			}
			seen[p] = true
			rel, err := filepath.Rel(root, p)
			if err != nil {
				rel = p
			}
			out = append(out, filepath.ToSlash(rel))
		}
	}
	sort.Strings(out)
	return out
}

// upInlineSpans —— py `inline_spans(line)` 逐字（成对反引号的内侧区间）。
func upInlineSpans(line string) [][2]int {
	spans := [][2]int{}
	i, n := 0, len(line)
	for i < n {
		if line[i] == '`' {
			j := strings.IndexByte(line[i+1:], '`')
			if j < 0 {
				break
			}
			j += i + 1
			spans = append(spans, [2]int{i + 1, j})
			i = j + 1
		} else {
			i++
		}
	}
	return spans
}

// upHitsInText —— py `hits_in_text(rel, text)` 逐字：围栏内 / 行内代码内 / 路由模板（`{` 紧跟 `/`）不计。
func upHitsInText(rel, text string) []upHit {
	hits := []upHit{}
	inFence := false
	for i, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		spans := upInlineSpans(line)
		for _, m := range upPAT.FindAllStringIndex(line, -1) {
			s := m[0]
			skip := false
			for _, sp := range spans {
				if sp[0] <= s && s < sp[1] {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			if s > 0 && line[s-1] == '/' {
				continue
			}
			hits = append(hits, upHit{Rel: rel, Line: i + 1, Tok: line[m[0]:m[1]],
				Text: strings.TrimSpace(line)})
		}
	}
	return hits
}

// upDecodeReplace —— py `open(...,encoding="utf-8",errors="replace")` 的对应体
// （非法序列按 Unicode「最大子部件」合成一个 U+FFFD；合法件走零拷贝路径）。
func upDecodeReplace(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r != utf8.RuneError || size > 1 {
			sb.Write(b[i : i+size])
			i += size
			continue
		}
		j := i + 1
		for j < len(b) && j-i < 3 && b[j]&0xC0 == 0x80 {
			j++
		}
		sb.WriteRune(utf8.RuneError)
		i = j
	}
	return sb.String()
}

// upTranslateNewlines —— py 文本模式读的**通用换行**翻译（`\r\n` → `\n`、裸 `\r` → `\n`）。
func upTranslateNewlines(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// upOSErrText —— py `OSError.__str__` 的逐字形态（`[Errno N] 说明: '路径'`）。
func upOSErrText(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		var errno syscall.Errno
		if errors.As(pe.Err, &errno) {
			switch errno {
			case syscall.ENOENT:
				return fmt.Sprintf("[Errno 2] No such file or directory: '%s'", pe.Path)
			case syscall.EACCES:
				return fmt.Sprintf("[Errno 13] Permission denied: '%s'", pe.Path)
			case syscall.EISDIR:
				return fmt.Sprintf("[Errno 21] Is a directory: '%s'", pe.Path)
			}
		}
	}
	return err.Error()
}

// upJudge —— py `judge(root)` 逐字（自带两处「判不了」打印）。
func upJudge(root string, stdout io.Writer) (int, []string, []upHit) {
	files := upScanFiles(root)
	if len(files) == 0 {
		fmt.Fprintln(stdout, "   ⛔ 判不了：扫面 **0 件**（两条 glob 都没匹配到产物件 —— 空转不许当绿）")
		fmt.Fprintln(stdout, "   ⇒ 本判据是**两值制（0/1）**、不给「没结论」档 ⇒ **按红记（rc=1）**")
		return 1, files, []upHit{}
	}
	hits := []upHit{}
	unreadable := []string{}
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			unreadable = append(unreadable, fmt.Sprintf("%s（%s）", rel, upOSErrText(err)))
			continue
		}
		text := upTranslateNewlines(upDecodeReplace(b))
		hits = append(hits, upHitsInText(rel, text)...)
	}
	if len(unreadable) > 0 {
		for _, u := range unreadable {
			fmt.Fprintf(stdout, "   ⛔ 件读不了：%s\n", u)
		}
		fmt.Fprintln(stdout, "   ⇒ 两值制 ⇒ **按红记（rc=1）**（读不全就判不出真值 · 不给「没结论」档）")
		return 1, files, hits
	}
	if len(hits) > 0 {
		return 1, files, hits
	}
	return 0, files, hits
}

// upTruncRunes —— py `line[:110]`（按**码位**截断）。
func upTruncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// upReport —— py `report(root, rc, files, hits, quiet=False)` 逐字。
//
//	★ 照实：py 侧**丢弃** `judge` 给的 rc（`return 1 if hits else 0`）——
//	  「扫面 0 件 / 件读不了」两态 judge 返 1，经 report 后 rc 仍**回落 0**（正文自相矛盾）。
//	  本件**逐字照搬该行为**（对拍第一优先：先搬得一样，口径问题另单提）。故形参 rc 不参与判决。
func upReport(w io.Writer, root string, _ int, files []string, hits []upHit, quiet bool) int {
	if !quiet {
		fmt.Fprintln(w, "── 判据「未代入占位符 = 0」（产物句面）· 块C §六④ / `Q-166` ──")
		fmt.Fprintf(w, "   判据树 / 仓根：%s\n", root)
		fmt.Fprintf(w, "   判据面（逐字照令）：%s = **%d 件**\n", strings.Join(upFaceGlobs, " + "), len(files))
		fmt.Fprintf(w, "   检测形态（逐字照 §三.3 最小检测句）：%s\n", upPATSource)
		fmt.Fprintln(w, "   三处排除（见件头 · 只排「不是命名占位」的两类）：围栏内 / 行内代码内 / 路由模板（`{` 紧跟 `/`）")
	}
	if len(hits) > 0 {
		if !quiet {
			fmt.Fprintf(w, "   ✗ 命中 **%d 处**（逐件逐行点名）：\n", len(hits))
		}
		for k, h := range hits {
			if k >= upMaxPrint {
				break
			}
			fmt.Fprintf(w, "     %s:%d: %s   ← %s\n", h.Rel, h.Line, h.Tok, upTruncRunes(h.Text, 110))
		}
		if len(hits) > upMaxPrint {
			fmt.Fprintf(w, "     …另有 %d 处（打印上限 %d）\n", len(hits)-upMaxPrint, upMaxPrint)
		}
		if !quiet {
			fmt.Fprintln(w, "   补救：改生成器/模板里那一处（缺 `.format(...)` 就补上，或把该占位删掉）⇒ 重跑 `--emit`")
			fmt.Fprintf(w, "⇒ **不通过（rc=1）**：产物里 %d 处未代入的命名花括号占位\n", len(hits))
		}
		return 1
	}
	if !quiet {
		fmt.Fprintln(w, "   ✓ 命中 **0 处**（本判据两值：0 / 1 ⇒ 不给「没结论」档）")
		fmt.Fprintln(w, "⇒ **全过（rc=0）**：产物句面无未代入的命名花括号占位")
	}
	return 0
}

func upListRules(w io.Writer) {
	fmt.Fprintln(w, upVersion)
	fmt.Fprintf(w, "  判据面（逐字照令）：%s\n", strings.Join(upFaceGlobs, " + "))
	fmt.Fprintf(w, "  检测形态（逐字照 §三.3 最小检测句）：%s\n", upPATSource)
	fmt.Fprintln(w, "  排除（只排「不是命名占位」的两类）：围栏内 / 行内代码内 / 路由模板（`{` 紧跟 `/`）")
	fmt.Fprintln(w, "  退码：0 = 0 处 · 1 = 有残留 或 判不了（两值制 · 不给「没结论」档）")
	fmt.Fprintln(w, "  不许：只做「两侧对拍」（gettext 反例：两侧同洞 ⇒ 照样假绿）")
}

// ── 自检（合成夹具 · 不碰真目标）────────────────────────────────────────────────
func upSelfTest(w io.Writer) int {
	t, err := os.MkdirTemp("", "cunsub-")
	if err != nil {
		fmt.Fprintf(w, "   ⛔ 自检跑不了：造临时目录失败：%v\n", err)
		fmt.Fprintf(w, "   ⇒ 自检不过（断言 0 格 · 失败 1 格）\n")
		return 1
	}
	defer os.RemoveAll(t)

	n, bad := 0, 0
	check := func(what string, got, want interface{}) {
		n++
		ok := fmt.Sprintf("%v", got) == fmt.Sprintf("%v", want)
		if !ok {
			bad++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(w, "   %s %s（实测 %v · 期望 %v）\n", mark, what, got, want)
	}
	fixture := func(name string, files map[string]string) string {
		d := filepath.Join(t, name)
		for rel, text := range files {
			p := filepath.Join(d, filepath.FromSlash(rel))
			_ = os.MkdirAll(filepath.Dir(p), 0o777)
			_ = os.WriteFile(p, []byte(text), 0o644)
		}
		_ = os.MkdirAll(filepath.Join(d, "publish", "docs"), 0o777)
		return d
	}
	appendFile := func(p, text string) {
		f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		_, _ = f.WriteString(text)
		_ = f.Close()
	}

	// 格 1 正控：干净产物 ⇒ 0
	d1 := fixture("clean", map[string]string{"docs/zh/参考-命令行.md": "# 参考\n\n命令 `zerg help` 只读。\n"})
	rc1, f1, h1 := upJudge(d1, w)
	check("格 1 正控（干净句面 ⇒ rc=0）", upReport(w, d1, rc1, f1, h1, true), 0)
	// 格 2 must-fail 负控：句面留一处 `{counts}`（= 漏 `.format()` 的形态）⇒ 1 + 逐一例点名
	p2 := filepath.Join(d1, "publish", "docs", "CLI.zh-CN.md")
	_ = os.WriteFile(p2, []byte("> 本版共 {counts} 条已开放命令。\n"), 0o644)
	rc2, _, h2 := upJudge(d1, w)
	check("格 2 负控（句面 `{counts}` ⇒ rc=1）", rc2, 1)
	name, lineno, tok := "—", 0, "—"
	if len(h2) > 0 {
		name, lineno, tok = h2[0].Rel, h2[0].Line, h2[0].Tok
	}
	check("格 2 负控（点名到 件:行: 占位）",
		fmt.Sprintf("%s:%d: %s", name, lineno, tok), "publish/docs/CLI.zh-CN.md:1: {counts}")
	// 格 2b 成对（区分度）：同一夹具删掉那一处 ⇒ 0
	_ = os.WriteFile(p2, []byte("> 本版共 118 条已开放命令。\n"), 0o644)
	rc2b, _, _ := upJudge(d1, w)
	check("格 2b 成对（同一夹具删掉占位 ⇒ rc=0）", rc2b, 0)
	// 格 3 假红对照：路由模板（真树那 35 处那一类）⇒ 0
	d3 := fixture("route", map[string]string{"docs/zh/模块-主控.md": "- **舰队**: `GET /api/models/{name}`（单枚模型）\n"})
	rc3, _, _ := upJudge(d3, w)
	check("格 3 假红对照（行内代码里的路由模板 ⇒ rc=0）", rc3, 0)
	appendFile(filepath.Join(d3, "docs", "zh", "模块-主控.md"), "- 裸路由模板（无行内代码）：GET /api/logs/{kind} 读日志\n")
	rc3b, _, _ := upJudge(d3, w)
	check("格 3b 假红对照（裸路由模板 `{` 紧跟 `/` ⇒ rc=0）", rc3b, 0)
	// 格 4 假红对照：代码区（围栏 + 行内）里的 `{counts}` ⇒ 0
	d4 := fixture("code", map[string]string{"docs/zh/x.md": "示例：\n\n```text\n{counts} 与 {\"counts\": 3}\n```\n\n行内代码里的 `{counts}` 不算。\n"})
	rc4, _, _ := upJudge(d4, w)
	check("格 4 假红对照（围栏内 / 行内代码里的 `{counts}` ⇒ rc=0）", rc4, 0)
	appendFile(filepath.Join(d4, "docs", "zh", "x.md"), "句面漏代：本版共 {counts} 条。\n")
	rc4b, _, _ := upJudge(d4, w)
	check("格 4b 成对（同一件句面加一处 ⇒ rc=1 · 代码区那条仍然不算）", rc4b, 1)
	// 格 5 空转负控：扫面 0 件 ⇒ 1（两值制 ⇒ 不给「没结论」档）
	d5 := fixture("empty", map[string]string{"docs/zh/空.md": ""})
	_ = os.Remove(filepath.Join(d5, "docs", "zh", "空.md"))
	rc5, _, _ := upJudge(d5, w)
	check("格 5 空转负控（扫面 0 件 ⇒ rc=1 · 两值制不给「没结论」档）", rc5, 1)
	// 格 6 缺根 ⇒ 1（同上口径）
	rc6, _, _ := upJudge(filepath.Join(t, "no-such"), w)
	check("格 6 缺根负控（找不到仓根 ⇒ rc=1）", rc6, 1)
	// 格 7 读不了件 ⇒ 1（用 `chmod 000` 造读不了的 .md 件）
	d7 := fixture("unreadable", map[string]string{"docs/zh/好.md": "干净\n", "docs/zh/坏.md": "读不了\n"})
	p7 := filepath.Join(d7, "docs", "zh", "坏.md")
	_ = os.Chmod(p7, 0)
	rc7, _, _ := upJudge(d7, w)
	check("格 7 读不了负控（件读不了 ⇒ rc=1）", rc7, 1)
	_ = os.Chmod(p7, 0o644)
	// 格 8 真树现跑：只证「判得动」（照实打印现值 · 不要求 0）
	if cwd, err := os.Getwd(); err == nil {
		if root := upFindRoot(cwd); root != "" {
			r8c, f8, h8 := upJudge(root, w)
			rc8 := upReport(w, root, r8c, f8, h8, true)
			fmt.Fprintf(w, "   ✓ 格 8 真树现跑：rc=%d（照实：本格只证「判得动」，不写死期望值）\n", rc8)
		} else {
			fmt.Fprintln(w, "   · 格 8 真树现跑：跳（cwd 上溯找不到仓根）")
		}
	} else {
		fmt.Fprintln(w, "   · 格 8 真树现跑：跳（cwd 上溯找不到仓根）")
	}
	verdict := "全过"
	if bad != 0 {
		verdict = "不过"
	}
	fmt.Fprintf(w, "   ⇒ 自检%s（断言 %d 格 · 失败 %d 格）\n", verdict, n, bad)
	if bad != 0 {
		return 1
	}
	return 0
}

// ── 入口 ───────────────────────────────────────────────────────────────────────
func unsubstitutedPlaceholdersGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc, hit := upRefuseJSON(stderr, argv); hit {
		return rc
	}

	rootOpt, rootGiven := "", false
	selfTest, listRules := false, false
	var extras []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, upHelp())
			return 0
		case a == "--root":
			if i+1 >= len(argv) || (len(argv[i+1]) > 1 && strings.HasPrefix(argv[i+1], "-")) {
				return upArgError(stderr, "argument --root: expected one argument")
			}
			i++
			rootOpt, rootGiven = argv[i], true
		case strings.HasPrefix(a, "--root="):
			rootOpt, rootGiven = a[len("--root="):], true
		case a == "--self-test":
			selfTest = true
		case a == "--list-rules":
			listRules = true
		default:
			extras = append(extras, a)
		}
	}
	if len(extras) > 0 {
		return upArgError(stderr, "unrecognized arguments: "+strings.Join(extras, " "))
	}

	if listRules {
		upListRules(stdout)
		return 0
	}
	if selfTest {
		fmt.Fprintln(stdout, "── 自检（合成夹具 · 不碰真目标）──")
		return upSelfTest(stdout)
	}

	root := ""
	if rootGiven && rootOpt != "" {
		if abs, err := filepath.Abs(rootOpt); err == nil {
			root = abs
		}
	} else if cwd, err := os.Getwd(); err == nil {
		root = upFindRoot(cwd)
	}
	if root == "" || !upIsDir(root) {
		fmt.Fprintln(stdout, "⛔ 判不了：找不到仓根（同时有 `docs/` 与 `publish/` 的那一层）⇒ 两值制 ⇒ 按红记（rc=1）")
		return 1
	}
	rc, files, hits := upJudge(root, stdout)
	return upReport(stdout, root, rc, files, hits, false)
}
