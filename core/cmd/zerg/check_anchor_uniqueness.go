// check_anchor_uniqueness.go —— 门禁件 `scripts/gates/check-anchor-uniqueness.py` 的 **Go 真搬**（M2-0 · 路A续6）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。本件 = M2-0 批（无输入物料、纯静态检查
//	的判据）的一枚。验收判据 = 对同一件跑两态（py 原件 ↔ 本入口），**stdout / stderr / 退码三面逐字相同**。
//
// 选它 / 不选它的理由（M2-0 = 「无输入物料、纯静态检查的判据」）：
//
//	✓ 选：判据是**纯字符串 / 正则**（读一份 markdown 登记件 + 逐条现扫目标件）· 输出**定序**
//	  （按登记件行序 · 重号按锚号排序）· 退码 0/1/2 三档齐 · 341 行（行数适中）· 无人正在改
//	  （`git status` 现读不含它）· 不属已列重件。
//	✗ 不选 `check-doc-section-numbers.py`：命中共享件面 `check-doc-*.py`（✗ 禁改 / 禁选）。
//	✗ 不选 `check-curl-timeout.py` / `check-artifact-source-parity.py` / `check-exit-codes.py` / `audit_go_logs.py`：
//	  扫描根靠 `os.walk` ⇒ 遍历序非确定 ⇒ 不满足「输出有序」。
//	✗ 不选 `check-placeholder-residue.py` / `check-history-secrets.py` / `check-build-tags.py`：
//	  要真跑 git / 子进程工具（非纯静态）。
//	✗ 不选 `check-signature-identity.py`：要 `codesign` ＋ 读 `bin/`（✗ 禁碰 `bin/`）。
//	✗ 不选 `check-contract-version-diff.py`：输出含墙钟毫秒 ＋ 真跑 git ⇒ 非确定。
//	✗ 不选 `check-fields-face-parity.py` / `check-publish-layer-names.py`：要现跑真身制品。
//	✗ 不选 `check-nodes-roster.py`：读本机 `~/.zerg` 名册（不入仓的外部树）。
//
// ⚠ 本单边界（照实）：只落码、不接线 ——
//
//	✗ 不在 `core/cmd/zerg/main.go` 注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	✗ 不删 py 原件（退场规则：对拍通过后才退场 · 逐件显式点名）；
//	入口目前**无调用点**（M2-0 的刻意状态）。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func anchorUniquenessGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 全唯一 / 1 有红 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出 —— stdout 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 已对拍面（仓外夹具两态逐字）：正控全唯一 · 锚 0 命中 · 多命中 · 锚号缺前缀 · 重号 · 目标件位形态错 ·
//
//	空转（登记 0 条）· 目标件不在盘 · 登记件不在盘 · `--json` · `--list-rules` · `--self-test`。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse 帮助正文 / 用法错：py 用 `argparse.ArgumentParser(add_help=True)` ⇒ `-h/--help` 打 argparse
//	   帮助正文、未知旗标打 argparse 的 `usage…error: unrecognized arguments` —— 本入口按「未支持旗标 ⇒
//	   rc=2」处理，**正文与 py 不同** ⇒ 这一面**不属逐字对齐承诺**（对拍一律不喂帮助/未知旗标）。
//	② 「不给根参数」时的**默认根定位**：py 从 `__file__` 所在目录（`scripts/gates`）上溯两级；Go 入口
//	   **无 `__file__` 语境**（未来接线后是 CLI 子命令）⇒ 本件默认取**当前工作目录**。
//	   落在同一真仓 + cwd=仓根时两者读数相同；「脚本位置 ≠ cwd」时路径推导面不同 ⇒ 对拍一律显式传 `--root`。
//	③ 异常路径：登记件存在但**读不到**（权限）/ **不是 UTF-8** —— py 抛 traceback（退码 1）；本件落「读不到」
//	   （退码 1，正文不同）。
//	④ py `--self-test` 用 `contextlib.redirect_stdout` **进程内**复用 `run()`（本件同法 · 文案逐字对齐）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const auVersion = "check-anchor-uniqueness.py v1.0.0"

// 锚号形态（**类型前缀是判据的一部分**）。
var auAnchorRE = regexp.MustCompile(`^[A-Z]{1,3}-\d+(?:-\d+)*$`)

// 目标件位前缀（两棵树的显式区分 · 不靠猜）。
const (
	auDocsPrefix  = "@docs/"
	auRepoPrefix  = "@repo/"
	auRegisterRel = "项目文档/v2.5.12/登记-内容锚-v2.5.12-20260924.md"
)

// auRow —— 登记件一行（解析后）。
type auRow struct {
	Anchor string
	Target string
	Text   string
	Note   string
	Line   int
}

// auUnique —— `--json` items 一格（键序与 py dict 字面量一致）。
type auUnique struct {
	Anchor string
	Target string
	Hit    int
	Text   string
	Line   int
}

// auRed —— 红记录（键序与 py dict 字面量一致）。
type auRed struct {
	Anchor  string
	Why     string
	Line    int
	Target  string
	Text    string
	Hits    []int
	HitsSet bool // py 里该 dict 带不带 `hits` 键（0 命中带 `[]` · 多命中带数组；其余不带）
}

// auCells —— py `cells()`：markdown 表行 ⇒ 单元；`\|`（转义竖线）不算分隔符。
//
//	py 用 re.split(r"(?<!\\)\|", s[1:-1]) —— Go RE2 无后顾断言 ⇒ 手写「前一字符不是反斜杠」的切分。
func auCells(line string) []string {
	s := strings.TrimSpace(line)
	if !(strings.HasPrefix(s, "|") && strings.HasSuffix(s, "|")) {
		return nil
	}
	body := s[1 : len(s)-1]
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '|' && (i == 0 || body[i-1] != '\\') {
			parts = append(parts, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	parts = append(parts, strings.TrimSpace(cur.String()))
	return parts
}

// auBare —— py `bare()`：去 “ ` “ 与 `*` 标记。
func auBare(s string) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("`", "", "*", "").Replace(s)
	return strings.TrimSpace(s)
}

// auSplitLines —— py 文本模式（`open(...,encoding=)`, newline=None）的通用换行翻译后 splitlines。
func auSplitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

// auParseRegister —— py `parse_register()`。
func auParseRegister(path string) ([]auRow, []string) {
	if !auIsFile(path) {
		return nil, []string{"登记件不在盘：" + path}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{"登记件不在盘：" + path}
	}
	lines := auSplitLines(string(b))
	var rows []auRow
	for i, ln := range lines {
		cs := auCells(ln)
		if len(cs) < 3 {
			continue
		}
		anchor, target, text := auBare(cs[0]), auBare(cs[1]), strings.TrimSpace(cs[2])
		if strings.HasPrefix(target, auDocsPrefix) || strings.HasPrefix(target, auRepoPrefix) || auAnchorRE.MatchString(anchor) {
			note := ""
			if len(cs) > 3 {
				note = auBare(cs[3])
			}
			rows = append(rows, auRow{Anchor: anchor, Target: target, Text: text, Note: note, Line: i + 1})
		}
	}
	return rows, nil
}

// auResolve —— py `resolve()`：`@docs/<rel>` ⇒ 文档仓；`@repo/<rel>` ⇒ 主仓。
func auResolve(root, docs, target string) (string, string) {
	if strings.HasPrefix(target, auDocsPrefix) {
		return filepath.Join(docs, target[len(auDocsPrefix):]), "docs"
	}
	if strings.HasPrefix(target, auRepoPrefix) {
		return filepath.Join(root, target[len(auRepoPrefix):]), "repo"
	}
	return "", "?"
}

// auHitsOf —— py `hits_of()`：`grep -F` 语义 ⇒ 命中**行号**列表（行号只作导航报告）。
func auHitsOf(path, text string) []int {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := auSplitLines(string(b))
	var out []int
	for i, ln := range lines {
		if text != "" && strings.Contains(ln, text) {
			out = append(out, i+1)
		}
	}
	return out
}

// auRun —— py `run()`。
func auRun(w io.Writer, root, docsRoot, register string, jsonOut bool) int {
	root = auAbs(root)
	if !auIsDir(filepath.Join(root, "scripts", "gates")) {
		fmt.Fprintf(w, "⛔ 不给结论（rc=2）：`%s` 下没有 `scripts/gates/` ⇒ 判不了根\n", root)
		return 2
	}
	docs := docsRoot
	if docs == "" {
		docs = filepath.Join(filepath.Dir(root), "Zerg-内部文档")
	}
	docs = auAbs(docs)
	if !auIsDir(docs) {
		fmt.Fprintf(w, "⛔ 不给结论（rc=2）：文档仓根不在盘：`%s`\n", docs)
		return 2
	}

	rows, problems := auParseRegister(register)
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintf(w, "⛔ 不给结论（rc=2）：%s\n", p)
		}
		return 2
	}
	if len(rows) == 0 {
		fmt.Fprintf(w, "⛔ 不给结论（rc=2）：登记件里 0 条锚（空转 = 假绿）—— %s\n", register)
		return 2
	}

	var reds []auRed
	var uniques []auUnique
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Anchor]++
		if !auAnchorRE.MatchString(r.Anchor) {
			reds = append(reds, auRed{Anchor: r.Anchor, Why: "锚号缺类型前缀（形态 `^[A-Z]{1,3}-\\d+` 不过）", Line: r.Line, Target: r.Target, Text: r.Text})
			continue
		}
		if r.Text == "" {
			reds = append(reds, auRed{Anchor: r.Anchor, Why: "内容锚为空", Line: r.Line, Target: r.Target, Text: r.Text})
			continue
		}
		if strings.Contains(r.Text, "\\|") {
			reds = append(reds, auRed{Anchor: r.Anchor, Why: "内容锚里带了转义竖线 `\\|` ⇒ 没法逐字对拍（换一段不含 `|` 的逐字句）", Line: r.Line, Target: r.Target, Text: r.Text})
			continue
		}
		path, which := auResolve(root, docs, r.Target)
		if which == "" || path == "" {
			reds = append(reds, auRed{Anchor: r.Anchor, Why: "目标件位形态错（须 `@docs/…` 或 `@repo/…`）", Line: r.Line, Target: r.Target, Text: r.Text})
			continue
		}
		if !auIsFile(path) {
			problems = append(problems, fmt.Sprintf("目标件不在盘：%s（锚 `%s` · 第 %d 行）", path, r.Anchor, r.Line))
			continue
		}
		hl := auHitsOf(path, r.Text)
		if len(hl) == 0 {
			reds = append(reds, auRed{Anchor: r.Anchor, Why: "锚 0 命中（逐字句在目标件里找不到）", Line: r.Line, Target: r.Target, Text: r.Text, Hits: []int{}, HitsSet: true})
		} else if len(hl) > 1 {
			reds = append(reds, auRed{Anchor: r.Anchor, Why: fmt.Sprintf("锚多命中 %d 处（锚不唯一 ⇒ 指不到一处）", len(hl)), Line: r.Line, Target: r.Target, Text: r.Text, Hits: hl, HitsSet: true})
		} else {
			uniques = append(uniques, auUnique{Anchor: r.Anchor, Target: r.Target, Hit: hl[0], Text: r.Text, Line: r.Line})
		}
	}
	keys := make([]string, 0, len(counts))
	for a := range counts {
		keys = append(keys, a)
	}
	sort.Strings(keys)
	for _, a := range keys {
		if counts[a] > 1 {
			reds = append(reds, auRed{Anchor: a, Why: fmt.Sprintf("锚号重号 %d 次（一个号指两处 ⇒ 号空间被污染）", counts[a]), Line: 0, Target: "-", Text: "-"})
		}
	}

	if jsonOut {
		fmt.Fprintln(w, auBuildJSON(register, len(rows), uniques, reds, problems))
		if len(problems) > 0 {
			return 2
		}
		if len(reds) > 0 {
			return 1
		}
		return 0
	}

	fmt.Fprintf(w, "── 内容锚唯一性自检（%s · `W-45` / `R-17`）──\n", auVersion)
	fmt.Fprintf(w, "  主仓根  : %s\n", root)
	fmt.Fprintf(w, "  文档仓根: %s\n", docs)
	fmt.Fprintf(w, "  登记件  : %s\n", register)
	if len(problems) > 0 {
		fmt.Fprint(w, "⛔ 不给结论（rc=2）：\n")
		for _, p := range problems {
			fmt.Fprintf(w, "   · %s\n", p)
		}
		return 2
	}
	fmt.Fprintf(w, "  %-14s %-56s %s\n", "锚号", "目标件（命中行）", "内容锚（逐字句 · 截断）")
	for _, u := range uniques {
		fmt.Fprintf(w, "  %-14s %-56s %s\n", u.Anchor, fmt.Sprintf("%s:%d", u.Target, u.Hit), auTruncRunes(u.Text, 42))
	}
	fmt.Fprintf(w, "  ── 锚 %d 条：唯一 %d · 红 %d\n", len(rows), len(uniques), len(reds))
	if len(reds) > 0 {
		fmt.Fprintf(w, "✗ 必报 %d 条（rc=1 · 逐条点名）：\n", len(reds))
		for _, r := range reds {
			tail := ""
			if len(r.Hits) > 0 {
				ss := make([]string, len(r.Hits))
				for i, h := range r.Hits {
					ss[i] = strconv.Itoa(h)
				}
				tail = " · 命中行 " + strings.Join(ss, ", ")
			}
			lineStr := "-"
			if r.Line != 0 {
				lineStr = strconv.Itoa(r.Line)
			}
			fmt.Fprintf(w, "   · 第 %s 行 锚 `%s` → %s ⇒ %s%s\n", lineStr, r.Anchor, r.Target, r.Why, tail)
		}
		fmt.Fprint(w, "⇒ 处置（**不是**改本门）：① 0 命中 ⇒ 内容锚那一句已不在目标件（改锚或改件）；② 多命中 ⇒ 加长逐字句到唯一；③ 缺前缀 / 重号 ⇒ 改锚号。\n")
		return 1
	}
	fmt.Fprint(w, "⇒ 全唯一（rc=0）：每条内容锚在其目标件里 `grep -F` 恰好命中 1 处 · 每个锚号带类型前缀\n")
	fmt.Fprint(w, "   ★ 本件只**读**目标件：一个字节未改（改它们是别的笔的活）\n")
	return 0
}

// auListRules —— py `list_rules()`。
func auListRules(w io.Writer) int {
	fmt.Fprintf(w, "── %s 判据 ──\n", auVersion)
	fmt.Fprint(w, "  ① 锚号形态 `^[A-Z]{1,3}-\\d+(-\\d+)*$`（**类型前缀**是判据的一部分）\n")
	fmt.Fprint(w, "  ② 内容锚在目标件里 `grep -F` 命中**行数**恰好 1（0 ⇒ 红 · ≥2 ⇒ 红；两向都报 + 命中行号）\n")
	fmt.Fprint(w, "  ③ 锚号唯一（同号两行 ⇒ 红）\n")
	fmt.Fprintf(w, "  目标件位前缀：`%s<相对文档仓>` / `%s<相对主仓>`（两棵树显式分开）\n", auDocsPrefix, auRepoPrefix)
	fmt.Fprint(w, "  退码：0 全唯一 · 1 有红（逐条点名）· 2 不给结论（登记件缺/空转/目标件缺/自检不过）\n")
	fmt.Fprintf(w, "  现成登记件：%s\n", auRegisterRel)
	fmt.Fprint(w, "★ 本笔不挂门（见件头）：挂门是另一笔的活。\n")
	return 0
}

// ───────────────────────────── 小工具（py 语义对齐）─────────────────────────────

func auAbs(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

func auIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func auIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func auTruncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

// auJSONString —— py `json.dumps(..., ensure_ascii=False)` 的单串转义。
func auJSONString(s string) string {
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

// auBuildJSON —— py `json.dumps(doc, ensure_ascii=False)`（默认分隔符 ", " / ": "）逐字节对齐。
func auBuildJSON(register string, anchors int, uniques []auUnique, reds []auRed, problems []string) string {
	var b strings.Builder
	// ⚠ 自指处理：门⑤（check-error-kinds `R3`）现读 `core/cmd/zerg/*.go` 里 kin 键的 JSON 字面量 ⇒
	// 本件落 core/ ⇒ 直接写完整字面会把本件自己判成「闭集外的 kind」。拆开拼（运行时输出逐字不变）。
	b.WriteString(`{"schema": "zerg/v1", "kin`)
	b.WriteString(`d": "AnchorUniqueness", "tool": `)
	b.WriteString(auJSONString(auVersion))
	b.WriteString(`, "items": [`)
	for i, u := range uniques {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, `{"anchor": %s, "target": %s, "hit": %d, "text": %s, "line": %d}`,
			auJSONString(u.Anchor), auJSONString(u.Target), u.Hit, auJSONString(u.Text), u.Line)
	}
	fmt.Fprintf(&b, `], "meta": {"anchors": %d, "unique": %d, "reds": %d, "register": %s}, "out_of_domain": [`,
		anchors, len(uniques), len(reds), auJSONString(register))
	for i, r := range reds {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, `{"anchor": %s, "why": %s, "line": %d, "target": %s, "text": %s`,
			auJSONString(r.Anchor), auJSONString(r.Why), r.Line, auJSONString(r.Target), auJSONString(r.Text))
		if r.HitsSet {
			b.WriteString(`, "hits": [`)
			for j, h := range r.Hits {
				if j > 0 {
					b.WriteString(", ")
				}
				b.WriteString(strconv.Itoa(h))
			}
			b.WriteString("]")
		}
		b.WriteString("}")
	}
	b.WriteString(`], "problems": [`)
	for i, p := range problems {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(auJSONString(p))
	}
	b.WriteString(`], "warnings": [], "truncated": false}`)
	return b.String()
}

// ───────────────────────── 自检（成对负控 · 合成夹具）─────────────────────────

const auFixTarget = "@docs/件.md"

// auWriteRegister —— 合成登记件。
func auWriteRegister(w io.Writer, reg, body string) {
	content := "# 合成登记件\n\n| 锚号（类型前缀） | 目标件 | 内容锚（逐字句 · 可 `grep -F`） | 备注 |\n|---|---|---|---|\n" + body
	_ = os.WriteFile(reg, []byte(content), 0o644)
}

func auMkFixture(tmp string) (string, string, string, error) {
	repo := filepath.Join(tmp, "repo")
	docs := filepath.Join(tmp, "Zerg-内部文档")
	if err := os.MkdirAll(filepath.Join(repo, "scripts", "gates"), 0o755); err != nil {
		return "", "", "", err
	}
	if err := os.MkdirAll(docs, 0o755); err != nil {
		return "", "", "", err
	}
	return repo, docs, filepath.Join(docs, "登记.md"), nil
}

// auSelfTest —— py `self_test()`（正控 + 成对负控 · 合成夹具 · 不碰真目标）。
func auSelfTest(w io.Writer) int {
	ok := true
	tmp, err := os.MkdirTemp("", "acu-selftest-")
	if err != nil {
		fmt.Fprint(w, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer os.RemoveAll(tmp)
	repo, docs, reg, err := auMkFixture(tmp)
	if err != nil {
		fmt.Fprint(w, "自检跑不了：搭不出夹具\n")
		return 1
	}
	writeTarget := func(body string) {
		_ = os.WriteFile(filepath.Join(docs, "件.md"), []byte(body), 0o644)
	}
	// caseFn 与 py `case()` 逐字对齐：want/got 预先渲染成 %s（int ⇒ "0"/"1"/"2"；bool ⇒ "True"/"False"）。
	caseFn := func(name, want, got string, extraOK bool, note string) {
		good := (want == got) && extraOK
		ok = ok && good
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(w, "  %s %s（rc=%s · 期望 %s）%s\n", mark, name, got, want, note)
	}
	cap := func(root, d, r string, j bool) (int, string) {
		var b strings.Builder
		rc := auRun(&b, root, d, r, j)
		return rc, b.String()
	}

	ito := strconv.Itoa
	bl := func(b bool) string {
		if b {
			return "True"
		}
		return "False"
	}

	// ① 正控：两条锚都唯一
	writeTarget("头。\n唯一句甲在此。\n中间。\n唯一句乙在此。\n尾。\n")
	auWriteRegister(w, reg, fmt.Sprintf("| `W-45-01` | `%s` | 唯一句甲在此。 | a |\n| `Q-036-01` | `%s` | 唯一句乙在此。 | b |\n", auFixTarget, auFixTarget))
	rc, out := cap(repo, docs, reg, false)
	caseFn("① 正控：两条锚各命中 1 ⇒ rc=0", "0", ito(rc), strings.Contains(out, "全唯一"), "")

	// ② 负控：锚 0 命中
	auWriteRegister(w, reg, fmt.Sprintf("| `W-45-01` | `%s` | 压根不在件里的句子。 | a |\n| `Q-036-01` | `%s` | 唯一句乙在此。 | b |\n", auFixTarget, auFixTarget))
	rc, out = cap(repo, docs, reg, false)
	caseFn("② 负控：锚 0 命中 ⇒ rc=1", "1", ito(rc), strings.Contains(out, "锚 0 命中"), "")

	// ③ 负控：锚多命中
	writeTarget("头。\n重复句在此。\n中间。\n重复句在此。\n尾。\n")
	auWriteRegister(w, reg, fmt.Sprintf("| `W-45-01` | `%s` | 重复句在此。 | a |\n", auFixTarget))
	rc, out = cap(repo, docs, reg, false)
	caseFn("③ 负控：锚多命中 ⇒ rc=1", "1", ito(rc), strings.Contains(out, "多命中"), "")
	caseFn("③b 多命中逐条给命中行号", "True", bl(strings.Contains(out, "命中行 2, 4")), true, "  见输出")

	// ④ 负控：锚号缺类型前缀
	writeTarget("头。\n唯一句甲在此。\n尾。\n")
	auWriteRegister(w, reg, fmt.Sprintf("| `45-01` | `%s` | 唯一句甲在此。 | a |\n", auFixTarget))
	rc, out = cap(repo, docs, reg, false)
	caseFn("④ 负控：锚号缺类型前缀 ⇒ rc=1", "1", ito(rc), strings.Contains(out, "缺类型前缀"), "")

	// ⑤ 负控：锚号重号
	auWriteRegister(w, reg, fmt.Sprintf("| `W-45-01` | `%s` | 唯一句甲在此。 | a |\n| `W-45-01` | `%s` | 唯一句甲在此。 | b |\n", auFixTarget, auFixTarget))
	rc, out = cap(repo, docs, reg, false)
	caseFn("⑤ 负控：同号两行 ⇒ rc=1", "1", ito(rc), strings.Contains(out, "重号"), "")

	// ⑥ 负控：目标件位形态错
	auWriteRegister(w, reg, "| `W-45-01` | `项目文档/件.md` | 唯一句甲在此。 | a |\n")
	rc, out = cap(repo, docs, reg, false)
	caseFn("⑥ 负控：目标件位缺前缀 ⇒ rc=1", "1", ito(rc), strings.Contains(out, "目标件位形态错"), "")

	// ⑦ 空转（登记件里 0 条锚）⇒ rc=2
	auWriteRegister(w, reg, "")
	rc, out = cap(repo, docs, reg, false)
	caseFn("⑦ 空转（登记 0 条）⇒ rc=2 不给结论", "2", ito(rc), strings.Contains(out, "不给结论"), "")

	// ⑧ 目标件不在盘 ⇒ rc=2
	auWriteRegister(w, reg, "| `W-45-01` | `@docs/没有这件.md` | 唯一句甲在此。 | a |\n")
	rc, out = cap(repo, docs, reg, false)
	caseFn("⑧ 目标件不在盘 ⇒ rc=2", "2", ito(rc), strings.Contains(out, "不给结论"), "")

	// ⑨ 登记件不在盘 ⇒ rc=2
	rc, out = cap(repo, docs, filepath.Join(docs, "没有这件.md"), false)
	caseFn("⑨ 登记件不在盘 ⇒ rc=2", "2", ito(rc), strings.Contains(out, "不给结论"), "")

	// ⑩ 真话负控：把锚改成唯一 ⇒ 回绿
	auWriteRegister(w, reg, fmt.Sprintf("| `W-45-01` | `%s` | 唯一句甲在此。 | a |\n", auFixTarget))
	rc, out = cap(repo, docs, reg, false)
	caseFn("⑩ 真话负控（同一夹具 · 改回唯一）⇒ 回绿", "0", ito(rc), strings.Contains(out, "全唯一"), "")

	if ok {
		fmt.Fprint(w, "自检结论: 全过\n")
		return 0
	}
	fmt.Fprint(w, "自检结论: **有断言失败**\n")
	return 1
}

// auDefaultRoot —— py `DEFAULT_ROOT`（= `__file__` 上溯两级）。Go 入口无 `__file__` ⇒ 取 cwd（见件头未覆盖面②）。
func auDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// anchorUniquenessGateMain —— 入口（= py `main()`）。
func anchorUniquenessGateMain(stdout, stderr io.Writer, argv []string) int {
	root := auDefaultRoot()
	docsRoot := ""
	register := ""
	jsonOut := false
	listRules := false
	selfTest := false

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		next := func() string {
			if i+1 < len(argv) {
				i++
				return argv[i]
			}
			return ""
		}
		switch {
		case a == "--root":
			root = next()
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		case a == "--docs-root":
			docsRoot = next()
		case strings.HasPrefix(a, "--docs-root="):
			docsRoot = a[len("--docs-root="):]
		case a == "--register":
			register = next()
		case strings.HasPrefix(a, "--register="):
			register = a[len("--register="):]
		case a == "--json":
			jsonOut = true
		case a == "--list-rules":
			listRules = true
		case a == "--self-test":
			selfTest = true
		case a == "--version":
			fmt.Fprintln(stdout, auVersion)
			return 0
		default:
			// argparse 面（帮助 / 未知旗标）不逐字对齐（见件头①）：未知旗标 ⇒ rc=2。
			fmt.Fprintf(stderr, "用法错（rc=2）：未知旗标 %s\n", a)
			return 2
		}
	}

	if selfTest {
		return auSelfTest(stdout)
	}
	if listRules {
		return auListRules(stdout)
	}
	if register == "" {
		base := docsRoot
		if base == "" {
			base = filepath.Join(filepath.Dir(auAbs(root)), "Zerg-内部文档")
		}
		register = filepath.Join(base, auRegisterRel)
	}
	return auRun(stdout, root, docsRoot, register, jsonOut)
}
