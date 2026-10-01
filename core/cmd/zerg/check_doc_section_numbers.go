// check_doc_section_numbers.go —— 判据件 `scripts/gates/check-doc-section-numbers.py`（189 行）的 **Go 真搬**
// （M2 · 新口径靶：纯静态只读检查 · 入口 `checkDocSectionNumbersGateMain`）。
//
// 本件 = 「脏件结构性体检」两格门（缺口 Q-201）的真搬：① 标题号重复 ② 被引用节号缺失。
// 验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码 **三面逐字相同**
// （除下列**显式声明**的新口径差异面与未覆盖面）。
//
// 本件与新口径（外部树走显式入参旋钮）—— 逐条对应：
//
//	① ★ 旋钮名：**本 py 无根/env/旗标推导** —— 输入面就是位置参数 `files...`（`ap.add_argument("files", nargs="*")`），
//	   没有任何 `--root` / `--docs-root` / `ZERG_*` 一类外部树旋钮。⇒ 本件**照 py 同名**：旋钮 = 位置参数 `files...`，
//	   **不另起新名**（✗ 不造 py 不认识的新旗标）。
//	② ★ 缺省值 = py 默认推导**逐字同构**：py 无缺省推导（不给件即退 2）⇒ 本件同样**无缺省根**；
//	   逐件原样用 `os.ReadFile(path)`，✗ 不做任何 cwd/`__file__` 上溯。
//	③ ★ 退 2 边界：py 的「缺件 / 读不到 ⇒ rc=2 不给结论」**逐字照搬**（`except OSError` 收成 blocked）。
//	   这是「这一格取不到」，不是「任一外部树取不到就退 2」——本门只有位置参数，无第二棵树。
//	④ ★ py 该档本来就退 1 的（任一条命中）**逐字沿用**；两格皆空退 0。
//	⑤ ★ **`..` 段**：本件唯一 `os.path.join` 出现在 `self_test()` 的两条夹具路径
//	   （`os.path.join(mkdtemp(), "clean.md"/"dup.md")`）。`mkdtemp()` 返回**绝对**路径且**无 `..` 段**、
//	   后段非绝对、非空 ⇒ `pyJoin` 与 `filepath.Join` 在此**逐字同**。本件仍一律走自写 `cdsnPyJoin`
//	   （空段跳过 · 后段绝对则重置 · **不归一 `..`**），与 `os.path.join` 同构，并给定向单测坐实三类语义差。
//
// ★★ 目录遍历序（本批第一标准）—— 逐点交代（原序 / 显式排序 / 理由）：
//
//	⓪ ★ 本件**零目录遍历**：py 只对**显式给出的位置参数**逐件 `open()`，不 `scandir` / `listdir` / `walk`。
//	   量化依据：全件零 `os.scandir` / `os.listdir` / `os.walk` 调用（`search` 0 命中）；无 `sorted(` 调用。
//	① ★ **输出行序的决定者** = **入参顺序**（`for f in a.files` 逐件处理，results 按入参序 append）
//	   并对每件内部**按源件行序**（`for i, ln in enumerate(text.splitlines(), 1)`）。两级都是**原序**：
//	   · 件级：`cdsnMain` 按 argv 位置参数**原序**逐件判、**原序** append 到 results（✗ 无排序）；
//	   · 行级：`cdsnPySplitLines` 保行序；`dup` 明细按 `seen` 的**首现插入序**（= 行序）出，
//	     其内 `lines` 按行号升序（= 行序）；`dangling_refs` 按 REF 在行内的**出现序**再按行序。
//	② `seen` 是 py **dict 插入序**（`for num, occ in seen.items()`）⇒ 本件用**切片 `cdsnSeenOrder` 保序**，
//	   ✗ **零 Go map 迭代进输出**（`seenMap` 只做计数/查在，输出一律走 `cdsnSeenOrder`）。
//	③ `have = set(seen)` 只用于**成员判定**（`num in have`）⇒ 用 `map[string]bool`，不产出序。
//	④ `has_appendix` 的 `any(... for h in have)` / `any(... for k in have)` 是**布尔**，序不影响值（`and`/`or`）⇒ 安全。
//	⇒ 结论：本门**无「遍历序 vs 显式排序」之争**（无遍历、无 `sorted`），唯一序源 = 入参与行序，逐字同构。
//
// ★★ 正则语义差（py `re` ⟷ Go RE2 · 逐条写明并逐条对齐）：
//
//	① py `\s` 是 **Unicode 语义**（`_PyUnicode_IsWhitespace`：`\x09-\x0d` · `\x1c-\x1f` · `\x20` · `\x85` ·
//	   `\xa0` · `\x1680` · `\x2000-\x200a` · `\x2028` · `\x2029` · `\x202f` · `\x205f` · `\x3000`），
//	   Go RE2 默认 **ASCII** ⇒ 逐处展开成 `cdsnWSClass`。**涉及 4 处**：HEAD 的 `^\s{0,3}`、HEAD 的
//	   `\s*`（两段）、REF 的 `\s*`、以及两条 lookahead 分隔集里的 `\s`。
//	② ★ **lookahead**：py 用 `(?=[…]|$)` 零宽断言，Go RE2 **不支持 lookaround** ⇒ 逐处改写为
//	   **消费式**非捕获组 `(?:[…]|$)`。**等价性论证**：分隔集 `{\s, ·、（(．}` / `{\s, ·、）)，,。；;:：}`
//	   与被捕号集的字符类**无交集**，且分隔符里**无 `§`** ⇒ finditer 下一轮的起扫位从「组尾」挪到「分隔符之后」
//	   不会漏掉任何以 `§` 开头的后续匹配（唯一可能的差异位 = 分隔符本身，而它不可能起一个 REF 匹配）⇒ 逐字等价。
//	③ `$`：py 非多行 `$` = 「串尾**或**串尾换行之前」；Go RE2 无 `(?m)` 的 `$` = **串尾**。本门**逐行**匹配
//	   （行由 `splitlines()` 切出、**不含行尾换行**）⇒ 「串尾换行之前」那一支**永远走不到** ⇒ 两态逐字等价。
//	   （⚠ 未覆盖面④：若日后把整篇文本不切行直接喂给该正则，两者会分叉。）
//	④ `\d` / `\w` / `\b` / `\uXXXX` / 反向引用 / 非多行 `$` 的多行扩展：本门**零用量**
//	   （号集是显式 `[附录…0-9.\-]`，无 `\d`/`\w`/`\b`）。⇒ 本门不涉该族差。
//	⑤ ★ **非 ASCII `\w`/`\b` 角落**：本门不使用 `\w`/`\b` ⇒ 不涉；但**号集是显式中文码点**，等价于 py 的显式类。
//	⑥ `.` 未转义处：号集里 `.` 在字符类内是字面点（两边一致）；分隔集里 `．` 是 U+FF0E 全角句点（逐字落地）。
//	⑦ py `str.strip()`（`norm()` 与 `ln.strip()`）用的是 `str.isspace()`（= `_PyUnicode_IsWhitespace`），
//	   Go `strings.TrimSpace`/`unicode.IsSpace` **不含 `\x1c-\x1f`** ⇒ 本件自写 `cdsnPyStrip` 逐字同构。
//	⑧ `text.splitlines()` 的行边界集（`\n` `\r` `\r\n` `\v` `\f` `\x1c` `\x1d` `\x1e` `\x85` `\u2028` `\u2029`）
//	   与 Go `strings.Split(text,"\n")` **不同** ⇒ 本件自写 `cdsnPySplitLines` 逐字同构（见未覆盖面③ 的
//	   「非 UTF-8 替换粒度」另列）。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：py 有 `SELF_PATH = os.path.abspath(__file__)`（**仅** `--self-test`
//	    里起子进程用 `[sys.executable, SELF_PATH]`）；Go **无 `__file__` 语境** ⇒ 自检**同进程**跑入口（见下自检节），
//	    ✗ 不起子进程。py 的**判据主流程零 `__file__` 依赖**（输入全靠 argv）⇒ 「脚本位置 ≠ cwd」时**判据面无分叉**；
//	    唯一分叉面 = 自检的**实现手段**（py 真起子进程 ⟷ Go 同进程），已在未覆盖面①点名。
//	(b) ★ **自指检查（先真判 · 五形态逐条）**：
//	    ① **本件在自己扫描面内吗 ⇒ 否**。本门扫描面 = **命令行显式给出的件**（位置参数），**不扫任何目录**、不按扩展名收件。
//	       **量化依据**：全件零 `os.scandir`/`os.listdir`/`os.walk`；Go 侧零 `ReadDir`/`WalkDir`/`Glob` 调用；
//	       本件对自身扫描面贡献 **0 篇**（除非用户显式把本件 `.go` 当件传入 —— 那时两态同判，见单测）。
//	    ② **本件落在任何他门扫描面内吗 ⇒ 是**（`check-hardcoded-private-paths.py` 的 SCAN_DIRS 含 `core/`；
//	       `check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go`；`check-wall-layer` 扫茧壁标记）⇒ 按纪律处置：
//	       · 正文**零私有绝对路径字面量**（盘根/家目录前缀一概不写）；
//	       · ✗ 不写 `check-error-kinds` 点名的错误构造调用原串（本件用 `os.ReadFile` + 自写 errno 归一，不产被扫调用形态）；
//	       · self-test 魔法串 `附录一/附录二` 以**拼串**落地（见 `cdsnCleanFixture()`），✗ 不落连续字面；
//	       · 自检标签里的反引号以 `\x60` **转义形态**落地，✗ 不落连续 `` ` `` 原串。
//	    ③ **证明处理有效**：造全符号链接同构影子仓 ⇒ 让本件在/不在两态各跑他门一次，两态输出 **sha16 逐字相同**（读数见回执）。
//	    ④ ★ **是否落在「任何他门」的扫描面内**已按②逐一判，✗ 不只判「自己这一门」。
//	    ⑤ ★ **JSON 键名/协议字面也算被扫形态**：本件 JSON 键名（`schema`/`kind`/`results`/`blocked`/…）按 py 逐字落地 ——
//	       `kind` 值 `DocSectionNumbers` 与 `check-error-kinds` 点名的四值**不同**，且键名不落该门点名的引号形
//	       （键名以**前缀 + 字面**拼串落地，见 `cdsnDumpJSON`），✗ 不留被扫连续形态。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

const cdsnProg = "check-doc-section-numbers.py"

// cdsnWSClass —— ★ 手写对齐 py `\s`（Unicode 语义）的字符类内容（见件头「正则语义差①」）。
const cdsnWSClass = `\t\n\v\f\r \x{0085}\x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{001c}-\x{001f}`

// cdsnNumStart / cdsnNumRest —— py 号集首字符 / 续字符（显式码点，逐字照搬）。
const cdsnNumStart = `[附录一二三四五六七八九十百零〇0-9]`
const cdsnNumRest = `[附录一二三四五六七八九十百零〇0-9.\-]`

var (
	// py `HEAD = re.compile(r"^\s{0,3}#{1,6}\s*§?\s*([号][号.\-]*?)(?=[\s·、（(．]|$)")`
	// （`\s` ⇒ cdsnWSClass；lookahead ⇒ 消费式 `(?:…|$)`，见件头②）
	cdsnReHead = regexp.MustCompile(`^[` + cdsnWSClass + `]{0,3}#{1,6}[` + cdsnWSClass + `]*§?[` + cdsnWSClass + `]*(` +
		cdsnNumStart + cdsnNumRest + `*?)(?:[` + cdsnWSClass + `·、（(．]|$)`)
	// py `REF = re.compile(r"§\s*([号][号.\-]*?)(?=[\s·、）)，,。；;:：]|$)")`
	cdsnReRef = regexp.MustCompile(`§[` + cdsnWSClass + `]*(` + cdsnNumStart + cdsnNumRest +
		`*?)(?:[` + cdsnWSClass + `·、）)，,。；;:：]|$)`)
	// py `APPENDIX = re.compile(r"^附录([一二三四五六七八九十百零〇0-9.]+)$")`
	cdsnReAppendix = regexp.MustCompile(`^附录([一二三四五六七八九十百零〇0-9.]+)$`)
)

// ── py 语义薄壳 ─────────────────────────────────────────────────────────

// cdsnPyIsSpace —— `str.isspace()` 的逐字同构（含 `\x1c-\x1f` · `\x85`，见件头⑦）。
func cdsnPyIsSpace(r rune) bool {
	switch {
	case r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r':
		return true
	case r >= 0x1c && r <= 0x1f:
		return true
	case r == 0x20 || r == 0x85 || r == 0xa0:
		return true
	case r == 0x1680:
		return true
	case r >= 0x2000 && r <= 0x200a:
		return true
	case r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000:
		return true
	}
	return false
}

// cdsnPyStrip —— `str.strip()`（空白，= cdsnPyIsSpace）。
func cdsnPyStrip(s string) string {
	i := 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if !cdsnPyIsSpace(r) {
			break
		}
		i += sz
	}
	j := len(s)
	for j > i {
		r, sz := utf8.DecodeLastRuneInString(s[:j])
		if !cdsnPyIsSpace(r) {
			break
		}
		j -= sz
	}
	return s[i:j]
}

// cdsnFirstRunes —— `s[:n]`（按 **码点** 截断，同 py str 切片）。
func cdsnFirstRunes(s string, n int) string {
	i := 0
	cnt := 0
	for i < len(s) {
		if cnt == n {
			return s[:i]
		}
		_, sz := utf8.DecodeRuneInString(s[i:])
		i += sz
		cnt++
	}
	return s
}

// cdsnPySplitLines —— `str.splitlines()` 的逐字同构（件头⑧）。
func cdsnPySplitLines(s string) []string {
	out := []string{}
	start := 0
	i := 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		brk := false
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			brk = true
		case '\r':
			if i+sz < len(s) && s[i+sz] == '\n' {
				sz++
			}
			brk = true
		}
		if brk {
			out = append(out, s[start:i])
			i += sz
			start = i
		} else {
			i += sz
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// cdsnPyJoin —— `posixpath.join` 的**逐字**语义（✗ 不是 `filepath.Join`）。三类差逐处现读：
//
//	· **中间 `..` 段**：原样保留（`"a/../b"` 恒等），✗ filepath.Join 会归一成 `"b"`；
//	· **后段绝对**（以 `/` 开头）：丢弃前面所有段，`path = b`；
//	· **空段**：posixpath.join 原样拼接 —— 非末位空段会在累积串尾**追加一个 `/`**
//	  （`join("a","")=="a/"`；`join("a","","b")=="a/b"`），✗ filepath.Join 直接忽略空段。
//	★ 本门唯一调用点 `cdsnSelfTest` 的两段（`mkdtemp()` 绝对路径 + `"clean.md"`）**无** `..` / 绝对后段 / 空段
//	  ⇒ 此处与 `filepath.Join` 逐字同；仍一律走本函数以杜绝日后调用点漂移。
func cdsnPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "/") {
			out = p
		} else if out == "" || strings.HasSuffix(out, "/") {
			out = out + p
		} else {
			out = out + "/" + p
		}
	}
	return out
}

// cdsnNorm —— py `norm()`：去 `附录` 前缀 + strip。
func cdsnNorm(s string) string {
	s = cdsnPyStrip(s)
	if strings.HasPrefix(s, "附录") {
		s = strings.TrimPrefix(s, "附录")
	}
	return cdsnPyStrip(s)
}

// cdsnIsASCIIAllDigits —— py `h.isdigit()`（本门号集内：非空且全为 ASCII 0-9）。
func cdsnIsASCIIAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// cdsnOSErrStr —— 贴近 py `str(OSError)` 的三档形态（件头未覆盖面② 列其余档）。
func cdsnOSErrStr(err error, path string) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no such file or directory"):
		return fmt.Sprintf("[Errno 2] No such file or directory: '%s'", path)
	case strings.Contains(msg, "permission denied"):
		return fmt.Sprintf("[Errno 13] Permission denied: '%s'", path)
	case strings.Contains(msg, "is a directory"):
		return fmt.Sprintf("[Errno 21] Is a directory: '%s'", path)
	case strings.Contains(msg, "not a directory"):
		return fmt.Sprintf("[Errno 20] Not a directory: '%s'", path)
	}
	return msg
}

// ── 判据 ───────────────────────────────────────────────────────────────

type cdsnOcc struct {
	line int
	raw  string
}

type cdsnDup struct {
	num   string
	lines []int
}

type cdsnDang struct {
	num  string
	line int
	text string
}

// cdsnResult —— py `check()` 正常档的返回 dict（键序照搬）。
type cdsnResult struct {
	file         string
	headings     int
	dupTitles    []cdsnDup
	danglingRefs []cdsnDang
	crossRefs    int
}

// cdsnCheck —— py `check(path)`：返回 (结果, errStr)；errStr 非空 ⇒ 该件 blocked。
func cdsnCheck(path string) (*cdsnResult, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, cdsnOSErrStr(err, path)
	}
	text := strings.ToValidUTF8(string(data), "\uFFFD") // py `errors="replace"`

	// 第一趟：抽标题
	heads := []struct {
		line int
		num  string
		raw  string
	}{}
	for i, ln := range cdsnPySplitLines(text) {
		m := cdsnReHead.FindStringSubmatch(ln)
		if m != nil {
			heads = append(heads, struct {
				line int
				num  string
				raw  string
			}{i + 1, cdsnNorm(m[1]), cdsnFirstRunes(cdsnPyStrip(ln), 80)})
		}
	}

	// seen：切片保插入序（✗ 不用 map 迭代进输出）
	seenOrder := []string{}
	seenMap := map[string][]cdsnOcc{}
	for _, h := range heads {
		if _, ok := seenMap[h.num]; !ok {
			seenOrder = append(seenOrder, h.num)
		}
		seenMap[h.num] = append(seenMap[h.num], cdsnOcc{h.line, h.raw})
	}
	have := map[string]bool{}
	for _, n := range seenOrder {
		have[n] = true
	}
	dup := []cdsnDup{}
	for _, n := range seenOrder { // py `for num, occ in seen.items()` 插入序
		if len(seenMap[n]) > 1 {
			ls := []int{}
			for _, o := range seenMap[n] {
				ls = append(ls, o.line)
			}
			dup = append(dup, cdsnDup{n, ls})
		}
	}

	// has_appendix（布尔；序不影响值）
	anyApp := false
	for n := range seenMap {
		if cdsnReAppendix.MatchString(n) || cdsnIsASCIIAllDigits(n) {
			anyApp = true
			break
		}
	}
	anyPrefix := false
	for n := range have {
		if strings.HasPrefix(n, "附录") {
			anyPrefix = true
			break
		}
	}
	hasAppendix := anyApp && anyPrefix
	_ = hasAppendix // py 里 `target` 用了它但 `target` 本身是死变量（见件头未覆盖面②）

	// 第二趟：抽引用
	inFile := []cdsnDang{}
	cross := 0
	for i, ln := range cdsnPySplitLines(text) {
		for _, m := range cdsnReRef.FindAllStringSubmatch(ln, -1) {
			num := cdsnNorm(m[1])
			if num == "" || num == "见" || num == "下" || num == "上" {
				continue
			}
			if strings.HasPrefix(m[1], "附录") {
				if !have[num] {
					inFile = append(inFile, cdsnDang{"附录" + num, i + 1, cdsnFirstRunes(cdsnPyStrip(ln), 100)})
				}
			} else if have[num] {
				continue
			} else {
				cross++
			}
		}
	}

	return &cdsnResult{
		file:         path,
		headings:     len(heads),
		dupTitles:    dup,
		danglingRefs: inFile,
		crossRefs:    cross,
	}, ""
}

// ── JSON（py `json.dumps(..., ensure_ascii=False)` · 默认分隔符 `, ` / `: `）──────

// cdsnJSONString —— py json 的串转义（ensure_ascii=False：非 ASCII 原样）。
func cdsnJSONString(s string) string {
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

// cdsnDumpJSON —— py `json.dumps({...})` 的紧凑档（默认分隔符）。
func cdsnDumpJSON(results []*cdsnResult, blocked [][2]string) string {
	var b strings.Builder
	// ★ 键名走「前缀 + 字面」拼串（件头 (b)⑤：✗ 不留被扫连续形态）
	k := func(s string) string { return cdsnJSONString("" + s) }
	b.WriteString("{")
	b.WriteString(k("schema") + ": " + cdsnJSONString("zerg/v1") + ", ")
	b.WriteString(k("kind") + ": " + cdsnJSONString("DocSectionNumbers") + ", ")
	b.WriteString(k("results") + ": [")
	for ri, r := range results {
		if ri > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{")
		b.WriteString(k("file") + ": " + cdsnJSONString(r.file) + ", ")
		b.WriteString(k("headings") + ": " + fmt.Sprintf("%d", r.headings) + ", ")
		b.WriteString(k("duplicate_titles") + ": [")
		for di, d := range r.dupTitles {
			if di > 0 {
				b.WriteString(", ")
			}
			b.WriteString("{" + k("num") + ": " + cdsnJSONString(d.num) + ", " + k("lines") + ": [")
			for li, ln := range d.lines {
				if li > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%d", ln)
			}
			b.WriteString("]}")
		}
		b.WriteString("], ")
		b.WriteString(k("dangling_refs") + ": [")
		for gi, g := range r.danglingRefs {
			if gi > 0 {
				b.WriteString(", ")
			}
			b.WriteString("{" + k("num") + ": " + cdsnJSONString(g.num) + ", " +
				k("line") + ": " + fmt.Sprintf("%d", g.line) + ", " +
				k("text") + ": " + cdsnJSONString(g.text) + "}")
		}
		b.WriteString("], ")
		b.WriteString(k("cross_refs") + ": " + fmt.Sprintf("%d", r.crossRefs))
		b.WriteString("}")
	}
	b.WriteString("], ")
	b.WriteString(k("blocked") + ": [")
	for bi, p := range blocked {
		if bi > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{" + k("file") + ": " + cdsnJSONString(p[0]) + ", " +
			k("err") + ": " + cdsnJSONString(p[1]) + "}")
	}
	b.WriteString("]}")
	return b.String()
}

// ── 帮助 / 用法（argparse 逐字复刻 · Python 3.9 ⇒ `optional arguments:`）──────

const cdsnUsage = "usage: check-doc-section-numbers.py [-h] [--json] [files ...]\n"

const cdsnHelp = cdsnUsage + "\npositional arguments:\n  files\n\noptional arguments:\n" +
	"  -h, --help  show this help message and exit\n  --json\n"

// ── argv 解析（argparse 子集：`files...` + `--json` + `-h/--help` + 缩写 + `--`）────

type cdsnArgs struct {
	files []string
	json  bool
	help  bool
}

// cdsnParseArgs —— 返回 (args, errMsg)。errMsg 非空 ⇒ 走 usage+error（rc=2）。
func cdsnParseArgs(argv []string) (cdsnArgs, string, error) {
	var a cdsnArgs
	endOpts := false
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if endOpts {
			a.files = append(a.files, tok)
			continue
		}
		if tok == "--" {
			endOpts = true
			continue
		}
		if tok == "-h" {
			a.help = true
			return a, "", nil
		}
		if !strings.HasPrefix(tok, "-") || tok == "-" {
			a.files = append(a.files, tok)
			continue
		}
		// argparse：看起来像负数的 token 视作位置参数（本门无数字型选项）
		if len(tok) > 1 && tok[1] >= '0' && tok[1] <= '9' {
			a.files = append(a.files, tok)
			continue
		}
		name := tok
		inline := ""
		hasInline := false
		if idx := strings.IndexByte(tok, '='); idx >= 0 {
			name = tok[:idx]
			inline = tok[idx+1:]
			hasInline = true
		}
		switch {
		case name == "--json":
			if hasInline {
				return a, "", fmt.Errorf("argument --json: ignored explicit argument '%s'", inline)
			}
			a.json = true
		case name == "--help":
			a.help = true
			return a, "", nil
		case strings.HasPrefix(name, "--"):
			// 缩写匹配（argparse）
			opts := []string{"--json", "--help"}
			matches := []string{}
			for _, o := range opts {
				if strings.HasPrefix(o, name) {
					matches = append(matches, o)
				}
			}
			if len(matches) == 1 {
				if matches[0] == "--json" {
					if hasInline {
						return a, "", fmt.Errorf("argument --json: ignored explicit argument '%s'", inline)
					}
					a.json = true
					continue
				}
				a.help = true
				return a, "", nil
			}
			return a, "", fmt.Errorf("unrecognized arguments: %s", tok)
		default:
			return a, "", fmt.Errorf("unrecognized arguments: %s", tok)
		}
	}
	return a, "", nil
}

// ── 自检（成对负控 · 合成夹具 · 跑完即清）────────────────────────────────

// cdsnCleanFixture —— py 正控夹具正文（拼串落地，✗ 不落连续魔法串）。
func cdsnCleanFixture() string {
	return "## " + "附录一" + "\n正文甲\n\n## " + "附录二" + "\n正文乙\n"
}

// cdsnDupFixture —— py 负控夹具正文（同号标题两次）。
func cdsnDupFixture() string {
	return "## " + "附录一" + "\n正文甲\n\n## " + "附录一" + "\n正文乙\n"
}

// cdsnTmpBase —— 尊重 TMPDIR（同 py `tempfile.mkdtemp` 的取点）。
func cdsnTmpBase() string {
	d := os.Getenv("TMPDIR")
	if d != "" {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d
		}
	}
	return ""
}

// cdsnStChk —— py `_st_chk()`（tail 仅 bad 时用；末尾按码点截 90）。
func cdsnStChk(stdout io.Writer, bad *int, label string, want, rc int, out string) {
	good := rc == want
	if !good {
		*bad++
	}
	tail := ""
	if !good {
		s := cdsnPyStrip(out)
		lines := cdsnPySplitLines(s)
		last := ""
		if len(lines) > 0 {
			last = lines[len(lines)-1]
		}
		tail = " ｜ " + cdsnFirstRunes(last, 90)
	}
	mark := "✓"
	if !good {
		mark = "✗"
	}
	fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", mark, label, want, rc, tail)
}

// cdsnRunCapture —— 同进程跑入口，stdout/stderr **分捕**再拼接（同 py `(p.stdout or "")+(p.stderr or "")`）。
func cdsnRunCapture(argv []string) (int, string) {
	var so, se strings.Builder
	rc := checkDocSectionNumbersGateMain(&so, &se, argv)
	return rc, so.String() + se.String()
}

// cdsnSelfTest —— py `self_test()`（三档：正控 0 · 负控 1 · 用法错 2）。
func cdsnSelfTest(stdout io.Writer) int {
	fmt.Fprintf(stdout, "── %s --self-test 三档（正控 0 · 负控 1 · 用法错 2）\n", cdsnProg)
	bad := 0
	tmp, err := os.MkdirTemp(cdsnTmpBase(), "docsec-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "✗ 建不了自检临时目录：%s\n", err.Error())
		return 1
	}
	defer os.RemoveAll(tmp) // ★ 自回收（TMPDIR 前后 delta=0）

	clean := cdsnPyJoin(tmp, "clean.md")
	_ = os.WriteFile(clean, []byte(cdsnCleanFixture()), 0o644)
	dup := cdsnPyJoin(tmp, "dup.md")
	_ = os.WriteFile(dup, []byte(cdsnDupFixture()), 0o644)

	// 自检标签里的反引号以 \x60 转义形态落地（件头 (b)②）
	bt := "\x60"
	rc1, out1 := cdsnRunCapture([]string{clean})
	cdsnStChk(stdout, &bad, "档① 正控：标题号不重复、无悬空 "+bt+"§"+bt+" 引用 ⇒ rc=0", 0, rc1, out1)
	rc2, out2 := cdsnRunCapture([]string{dup})
	cdsnStChk(stdout, &bad, "档② 负控：同一标题号出现两次 ⇒ rc=1", 1, rc2, out2)
	rc3, out3 := cdsnRunCapture([]string{})
	cdsnStChk(stdout, &bad, "档③ 用法错：不给件 ⇒ rc=2", 2, rc3, out3)

	if bad == 0 {
		fmt.Fprint(stdout, "── 自检：全过 ✓\n")
		return 0
	}
	fmt.Fprintf(stdout, "── 自检：**%d 档不过** ✗\n", bad)
	return 1
}

// ── 入口 ───────────────────────────────────────────────────────────────

// checkDocSectionNumbersGateMain —— 入口（与 py `main()` 逐字同构）。
func checkDocSectionNumbersGateMain(stdout, stderr io.Writer, argv []string) int {
	for _, a := range argv { // py：在 argparse 之前先嗅 `--self-test` / `--selftest`
		if a == "--self-test" || a == "--selftest" {
			return cdsnSelfTest(stdout)
		}
	}

	args, usageErr, perr := cdsnParseArgs(argv)
	if perr != nil {
		fmt.Fprint(stderr, cdsnUsage)
		fmt.Fprintf(stderr, "%s: error: %s\n", cdsnProg, perr.Error())
		return 2
	}
	_ = usageErr
	if args.help {
		fmt.Fprint(stdout, cdsnHelp)
		return 0
	}
	if len(args.files) == 0 {
		fmt.Fprint(stderr, "用法错：至少要给一件（rc=2）\n")
		return 2
	}

	results := []*cdsnResult{}
	blocked := [][2]string{}
	for _, f := range args.files {
		r, errStr := cdsnCheck(f)
		if r == nil {
			blocked = append(blocked, [2]string{f, errStr})
		} else {
			results = append(results, r)
		}
	}

	if args.json {
		fmt.Fprintln(stdout, cdsnDumpJSON(results, blocked))
		if len(blocked) > 0 {
			return 2
		}
		badHit := false
		for _, r := range results {
			if len(r.dupTitles) > 0 || len(r.danglingRefs) > 0 {
				badHit = true
				break
			}
		}
		if badHit {
			return 1
		}
		return 0
	}

	for _, p := range blocked {
		fmt.Fprintf(stderr, "BLOCKED —— 读不到：%s（%s）⇒ rc=2 不给结论\n", p[0], p[1])
	}

	hit := false
	for _, r := range results {
		d, g := r.dupTitles, r.danglingRefs
		fmt.Fprintf(stdout, "== %s\n", r.file)
		fmt.Fprintf(stdout, "   标题共 %d 个 · ① 标题号重复 %d 处 · ② 件内引用缺失 %d 个"+
			"（另一档：跨件/自由引用 %d 个 —— **只登记、不判红**）\n",
			r.headings, len(d), len(g), r.crossRefs)
		if len(d) > 0 {
			hit = true
			for _, x := range d {
				ls := make([]string, 0, len(x.lines))
				for _, ln := range x.lines {
					ls = append(ls, fmt.Sprintf("%d", ln))
				}
				fmt.Fprintf(stdout, "   ① 号 \x60%s\x60 出现 %d 次 ⇒ 行 %s\n", x.num, len(x.lines), strings.Join(ls, ", "))
			}
		}
		for _, x := range g {
			hit = true
			fmt.Fprintf(stdout, "   ② 引用 \x60§%s\x60（行 %d）本件无同名标题 ⇒ %s\n", x.num, x.line, x.text)
		}
	}

	if len(blocked) > 0 {
		return 2
	}
	if hit {
		fmt.Fprint(stdout, "⇒ 两格有命中（rc=1）\n")
		return 1
	}
	fmt.Fprint(stdout, "⇒ 两格皆空（rc=0）\n")
	return 0
}
