// check_publish_layer_names.go —— 判据件 `scripts/gates/check-publish-layer-names.py`（484 行 ·
// sha256 前16 `012e0e5269466f64`）的 **Go 真搬**（M2 · 发布面层名闭集族 ·
// 入口 `checkPublishLayerNamesGateMain` · 承接 GAP-20261001-13）。
//
// 验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 声明 (a) 默认根定位差异：py 用 `dirname(dirname(abspath(__file__)))` 从**脚本位置**推默认仓根
// （`scripts/gates/…` → 仓根）；Go 无 `__file__` 语境 ⇒ 只能从 **cwd 上溯**找「含
// `publish/mirror-public-lib.py` 的目录」。两态在「脚本位置 == cwd 上溯根」时同构（对拍即按此布置）；
// 缺省面**不声称**逐字对齐（见回执「未覆盖面」）。
//
// 声明 (b) 自指：本件**不落在本判据自己的**扫描面内（本判据只**读**四个源 ＋ 现跑一条只读档，不扫
// 任何以件内容为判面的集合）。但本件落 `core/cmd/zerg/`，**落在他门的扫描面内**（例：
// `scripts/gates/check-hardcoded-private-paths.py` 的 `SCAN_DIRS` 含 `core/`）⇒ 本件正文**零私有卷 /
// 家目录绝对路径字面量**（一律不写绝对路径；夹具字面量只出现「仓相对路径」）。依据：本件内
// `"/"` 起首的路径字面量 = 0 处（路径一律经 `cplnPyJoin` 拼、或仓相对常量）。
//
// 遍历 / 排序点（逐点交代）：
//
//	P1 真源 `SKIP_LABELS`：**无目录遍历**（只读单件）；元组元素序 = 源文件里 tuple 元素的**书写序**。
//	P2 CLI 层名闭集：`regexp.FindAllStringSubmatch` ⇒ 匹配序 = 文件**行序**（左到右）。
//	P3 py 明写 `sorted(true_returns)` / `sorted(true_subs)` / `sorted(observed - set(cli))`
//	   ⇒ Go `sort.Strings` 复现（三处显式排序点）。
//	P4 **输出行序的决定者** = `bad` 列表的**构造序**：
//	   L1(returns 排序序) → L1(subs 排序序) → L3甲(true_labels 书写序) → L3乙(cli_layers 行序)
//	   → L4(observed-cli 排序序) → L4(cli_layers 行序) → L4(探针声明序) → L4 正控 → L2 哨兵(最后追加)。
//	   `true_labels`/`cli_layers` 打印序亦分别是书写序 / 行序（非排序）。零 Go map 迭代入输出。
//
// `os.path.join` 逐处交代：本件三处加入点（`LIB_REL`/`CLI_REL` 拼法、`join(repo_root, LIB_REL)`、
// 夹具 `join(d, dirname(LIB_REL))`）**均非空段 / 无 `..` / 后段非绝对**（三语义差都不触发），
// 但按本批口径**恒同构**：自写 `cplnPyJoin`（照 CPython `posixpath.join` 逐段规则）＋
// `cplnPyDirname`（`posixpath.dirname` 语义）。定向单测把三态各钉一格。
// 正则面：唯一一处 `CLI_LAYER_RE`（`^\s*(publishSetLayer[A-Za-z]+)\s*=\s*"([^"]+)"\s*$` · 多行）
// ⇒ Go `(?m)` 同构；`\s` 的 Unicode 面差异登记未覆盖。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	cplnVersion        = "1.0.0"
	cplnTrueLabelsName = "SKIP_LABELS"
	cplnSentinelID     = "publishSetLayerNone"
	cplnLayerNone      = "—"
)

var (
	cplnLibRel = cplnPyJoin("publish", "mirror-public-lib.py")
	cplnCLIRel = cplnPyJoin("core", "cmd", "zerg", "family_publish_set.go")

	cplnCLILayerRE = regexp.MustCompile(`(?m)^\s*(publishSetLayer[A-Za-z]+)\s*=\s*"([^"]+)"\s*$`)
)

// cplnCliOnlyEntry —— CLI 侧**有意**存在、真源侧**没有同名层**的层名（逐条带锚点 · 现读校验）。
type cplnCliOnlyEntry struct{ Name, LibRel, Anchor, Why string }

var cplnCliOnly = []cplnCliOnlyEntry{
	{"白名单未命中", "publish/mirror-public-lib.py", "# 不在白名单", "真源只读预演对「不在白名单」的件是**静默 continue**（既不「会发」也不计入「跳过」的计数）⇒ 真源侧**没有**这一层的层名；CLI 的 `publish set` 答的是「这件**会不会**发 + 哪一层拦的」⇒ 必须给「白名单未命中」一个可点的层名（否则「不发」这一格答不出拦在哪里）。锚点 = 真源那句 continue 的注释。"},
	{"私有面硬门禁", "publish/mirror-public-lib.py", "私有面门禁：镜像树出现私有路径（硬门禁）", "真源侧的处置点 = `Assets.private_face_hit`（`publish/private-paths.txt`）+ 真跑原位 `:1225-1227` 的**整批中止**（那份清单是硬门禁 ⇒ 优先级最高）；但真源的**层名口径**里没有它（`SKIP_LABELS` 那五格是「跳过」分类，硬门禁是**中止**不是跳过）⇒ CLI 单立一个层名。锚点 = 真源真跑那一处的中止文字。"},
}

// 探针集（逐条给「这一件该命中哪一层」· 现读实证）
var cplnProbes = [][2]string{
	{"README-private.md", "白名单未命中"},
	{"scripts/公开标记.tsv", "EXCLUDES"},
	{"publish/mirror-verify-tree.py", "DROP_EXACT"},
	{"core/zerg-agent2", "DROP_PREFIX"},
	{"publish/mirror-allow.txt", "映射丢弃"},
	{"publish/replace-rules.tsv", "私有面硬门禁"},
}

// 正控（「会发」那一件）：`layer` 必须是 `—`（不命中任何层）
const cplnProbeEmit = "core/cmd/zerg/family_publish_set.go"

const cplnUsageText = "usage: check-publish-layer-names.py [-h] [--repo-root REPO_ROOT] [--bin BIN]\n" +
	"                                    [--no-live] [--json] [--list-rules]\n" +
	"                                    [--self-test]\n" +
	"                                    [--self-test-dir SELF_TEST_DIR]\n"

const cplnHelpText = cplnUsageText +
	"\nCLI 层名闭集 ⟷ 真源层名口径 对拍\n" +
	"\noptional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --repo-root REPO_ROOT\n" +
	"  --bin BIN\n" +
	"  --no-live             只跑 L1–L3（仓外夹具用；缺 `bin/zerg` 时的显式出口）\n" +
	"  --json\n" +
	"  --list-rules\n" +
	"  --self-test\n" +
	"  --self-test-dir SELF_TEST_DIR\n"

const cplnListRules = "判据（四条 · 现读 · 缺口 GAP-20260927-412）\n" +
	"--------------------------------------------\n" +
	"  L1 真源层名闭集：`publish/mirror-public-lib.py` 的 `SKIP_LABELS`（ast 现读）；另判真源自洽\n" +
	"     （`exclude_kind` 的 return 字面量 ⊆ 闭集 · `list_only_report` 的 `skip[…\"]` ⊆ 闭集）\n" +
	"  L2 CLI 层名闭集：`core/cmd/zerg/family_publish_set.go` 的 `publishSetLayer<名> = \"…\"` 常量块\n" +
	"  L3 双向对拍：真源每个层名必须在 CLI 闭集里有对应（相等或 CLI 名 = `真源名（…）`）；\n" +
	"     CLI 每个层名必须对应真源层名，或在 CLI_ONLY 表里（逐条带真源锚点 · 锚点现读校验）\n" +
	"  L4 运行面 ⟷ 声明面：现跑 `publish set <探针> --json` ⇒ 观测集（layer 值 ∪ non_blob_basis\n" +
	"     里出现的闭集名）必须 == CLI 闭集；逐条探针必须打出它期望的那一层；「会发」正控必须 = `—`\n" +
	"\n不假装覆盖：各层的**判定实现**（哪一件该被哪一层拦）不在本件（那是 check-publish-face-sync.py\n" +
	"与 cli_publish_set_test.go 的面）。本件只读四个源 + 现跑一个只读档，不写盘。\n"

// ── py 语义原语（✗ 不用 filepath.Join / strings.TrimSpace 一类）────────────────────────

// cplnPyJoin —— CPython `posixpath.join` 逐段规则直译。
func cplnPyJoin(elems ...string) string {
	if len(elems) == 0 {
		return ""
	}
	path := elems[0]
	for _, b := range elems[1:] {
		if len(b) > 0 && b[0] == '/' {
			path = b
		} else if path == "" || path[len(path)-1] == '/' {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

// cplnPyDirname —— CPython `posixpath.dirname`（= split 的头段，剥尾斜杠）。
func cplnPyDirname(p string) string {
	i := strings.LastIndexByte(p, '/') + 1
	head := p[:i]
	if head != "" {
		allSlash := true
		for k := 0; k < len(head); k++ {
			if head[k] != '/' {
				allSlash = false
				break
			}
		}
		if !allSlash {
			head = strings.TrimRight(head, "/")
		}
	}
	return head
}

// cplnPyStrip —— py `str.strip()`：剥 Unicode 空白 **含** \x1c-\x1f / \x85 / \xa0。
func cplnPyStrip(s string) string {
	rs := []rune(s)
	lo, hi := 0, len(rs)
	for lo < hi && cplnIsPySpace(rs[lo]) {
		lo++
	}
	for hi > lo && cplnIsPySpace(rs[hi-1]) {
		hi--
	}
	return string(rs[lo:hi])
}

func cplnIsPySpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	if r == 0x85 || r == 0xa0 {
		return true
	}
	return unicode.IsSpace(r)
}

// cplnPyCut —— py 切片 `s[:n]` = 前 n 个**码点**。
func cplnPyCut(s string, n int) string {
	if n < 0 {
		n = 0
	}
	cnt := 0
	for i := range s {
		if cnt == n {
			return s[:i]
		}
		cnt++
	}
	return s
}

// cplnPyRepr —— py `repr(str)` 的等价（我们的层名/锚点不含控制字符时逐字同）。
func cplnPyRepr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r == rune(quote) {
				b.WriteByte('\\')
				b.WriteRune(r)
			} else if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, "\\x%02x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// cplnOSErrStr —— py `OSError` 的 `str()` 近似（Errno 13 / 2 覆盖；其余回落 Go 文案）。
func cplnOSErrStr(err error, path string) string {
	if os.IsPermission(err) {
		return "[Errno 13] Permission denied: '" + path + "'"
	}
	if os.IsNotExist(err) {
		return "[Errno 2] No such file or directory: '" + path + "'"
	}
	return err.Error()
}

func cplnStatRegular(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return st.Mode().IsRegular()
}

// ── 极简 Python 词法 / 片段解析（对齐 `ast` 的四处取数面）────────────────────────────

const (
	cplnTokName = iota
	cplnTokStr
	cplnTokNum
	cplnTokOp
	cplnTokNL
)

type cplnTok struct {
	kind int
	text string
	val  string
	isF  bool
	isB  bool
	line int
	col  int
}

func cplnIsIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func cplnIsIdentCont(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func cplnValidStrPrefix(p string) bool {
	switch strings.ToLower(p) {
	case "", "r", "b", "u", "f", "rb", "br", "fr", "rf":
		return true
	}
	return false
}

var cplnMultiOps = []string{"**=", "//=", ">>=", "<<=", "**", "//", ">>", "<<", "<=", ">=", "==", "!=", "->", ":=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^="}

// cplnLex —— 产出 token（括号内换行被吞 ⇒ 逻辑行自动合并）。返回 "" 表示无词法错；
// 非空 = 语法不合的近似判据（未覆盖面：正文与 py SyntaxError 不同）。
func cplnLex(src string) ([]cplnTok, string) {
	var toks []cplnTok
	n := len(src)
	i, line, lineStart := 0, 1, 0
	depth := 0
	for i < n {
		c := src[i]
		col := i - lineStart
		switch {
		case c == '\n':
			if depth == 0 {
				toks = append(toks, cplnTok{kind: cplnTokNL, line: line, col: col})
			}
			i++
			line++
			lineStart = i
			continue
		case c == ' ' || c == '\t' || c == '\f' || c == '\r' || c == '\v':
			i++
			continue
		case c == '#':
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		case c == '\\' && i+1 < n && src[i+1] == '\n':
			i += 2
			line++
			lineStart = i
			continue
		}
		// 字符串（含前缀）
		if c == '\'' || c == '"' {
			val, isF, isB, ni, ok := cplnScanString(src, i, "")
			if !ok {
				return nil, "未闭合的字符串字面量（第 " + fmt.Sprint(line) + " 行）"
			}
			toks = append(toks, cplnTok{kind: cplnTokStr, val: val, isF: isF, isB: isB, line: line, col: col})
			for k := i; k < ni && k < n; k++ {
				if src[k] == '\n' {
					line++
					lineStart = k + 1
				}
			}
			i = ni
			continue
		}
		if r, size := utf8.DecodeRuneInString(src[i:]); cplnIsIdentStart(r) {
			j := i
			lett := []rune{}
			for j < n {
				r2, s2 := utf8.DecodeRuneInString(src[j:])
				if !cplnIsIdentCont(r2) {
					break
				}
				lett = append(lett, r2)
				j += s2
			}
			_ = size
			if j < n && (src[j] == '\'' || src[j] == '"') && len(lett) <= 2 && cplnValidStrPrefix(string(lett)) {
				val, isF, isB, ni, ok := cplnScanString(src, j, string(lett))
				if !ok {
					return nil, "未闭合的字符串字面量（第 " + fmt.Sprint(line) + " 行）"
				}
				toks = append(toks, cplnTok{kind: cplnTokStr, val: val, isF: isF, isB: isB, line: line, col: col})
				for k := i; k < ni && k < n; k++ {
					if src[k] == '\n' {
						line++
						lineStart = k + 1
					}
				}
				i = ni
				continue
			}
			toks = append(toks, cplnTok{kind: cplnTokName, text: src[i:j], line: line, col: col})
			i = j
			continue
		}
		if c >= '0' && c <= '9' {
			j := i
			for j < n {
				cc := src[j]
				if (cc >= '0' && cc <= '9') || (cc >= 'a' && cc <= 'z') || (cc >= 'A' && cc <= 'Z') || cc == '.' || cc == '_' {
					j++
					continue
				}
				break
			}
			toks = append(toks, cplnTok{kind: cplnTokNum, text: src[i:j], line: line, col: col})
			i = j
			continue
		}
		// 运算符
		matched := ""
		for _, op := range cplnMultiOps {
			if strings.HasPrefix(src[i:], op) {
				matched = op
				break
			}
		}
		if matched == "" {
			matched = src[i : i+1]
		}
		switch matched {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth < 0 {
				return nil, "括号不配对（第 " + fmt.Sprint(line) + " 行）"
			}
		}
		toks = append(toks, cplnTok{kind: cplnTokOp, text: matched, line: line, col: col})
		i += len(matched)
	}
	if depth != 0 {
		return nil, "括号未闭合"
	}
	return toks, ""
}

// cplnScanString —— 从 q（引号起点）扫一个字符串；prefix 为字面前缀（可空）。
func cplnScanString(src string, q int, prefix string) (string, bool, bool, int, bool) {
	n := len(src)
	if q >= n {
		return "", false, false, q, false
	}
	low := strings.ToLower(prefix)
	raw := strings.Contains(low, "r")
	isB := strings.Contains(low, "b")
	isF := strings.Contains(low, "f")
	quote := src[q]
	triple := q+2 < n && src[q+1] == quote && src[q+2] == quote
	j := q + 1
	if triple {
		j = q + 3
	}
	var b strings.Builder
	for j < n {
		ch := src[j]
		if ch == '\\' {
			if raw {
				if j+1 < n {
					b.WriteByte('\\')
					b.WriteByte(src[j+1])
					j += 2
					continue
				}
				b.WriteByte('\\')
				j++
				continue
			}
			v, nj := cplnDecodeEscape(src, j)
			b.WriteString(v)
			j = nj
			continue
		}
		if triple {
			if ch == quote && j+2 < n && src[j+1] == quote && src[j+2] == quote {
				return b.String(), isF, isB, j + 3, true
			}
			if r, size := utf8.DecodeRuneInString(src[j:]); size > 0 {
				b.WriteRune(r)
				j += size
				continue
			}
		}
		if ch == quote {
			return b.String(), isF, isB, j + 1, true
		}
		if ch == '\n' {
			return "", false, false, q, false
		}
		if r, size := utf8.DecodeRuneInString(src[j:]); size > 0 {
			b.WriteRune(r)
			j += size
			continue
		}
		j++
	}
	return "", false, false, q, false
}

func cplnDecodeEscape(src string, j int) (string, int) {
	// src[j] == '\\'
	n := len(src)
	k := j + 1
	if k >= n {
		return "\\", k
	}
	e := src[k]
	switch e {
	case 'n':
		return "\n", k + 1
	case 't':
		return "\t", k + 1
	case 'r':
		return "\r", k + 1
	case '\\':
		return "\\", k + 1
	case '\'':
		return "'", k + 1
	case '"':
		return "\"", k + 1
	case 'a':
		return "\a", k + 1
	case 'b':
		return "\b", k + 1
	case 'f':
		return "\f", k + 1
	case 'v':
		return "\v", k + 1
	case '\n':
		return "", k + 1
	case 'x':
		if k+2 < n {
			if v, err := cplnHex(src[k+1 : k+3]); err == nil {
				return string(rune(v)), k + 3
			}
		}
	case 'u':
		if k+4 < n {
			if v, err := cplnHex(src[k+1 : k+5]); err == nil {
				return string(rune(v)), k + 5
			}
		}
	case 'U':
		if k+8 < n {
			if v, err := cplnHex(src[k+1 : k+9]); err == nil {
				return string(rune(v)), k + 9
			}
		}
	default:
		if e >= '0' && e <= '7' {
			m := k
			for m < n && m < k+3 && src[m] >= '0' && src[m] <= '7' {
				m++
			}
			if v, err := cplnOctal(src[k:m]); err == nil {
				return string(rune(v)), m
			}
		}
	}
	return "\\" + string(e), k + 1
}

func cplnHex(s string) (int, error) {
	v := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		default:
			return 0, fmt.Errorf("bad hex")
		}
		v = v*16 + d
	}
	return v, nil
}

func cplnOctal(s string) (int, error) {
	v := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '7' {
			return 0, fmt.Errorf("bad octal")
		}
		v = v*8 + int(s[i]-'0')
	}
	return v, nil
}

// ── 逻辑行 / 片段工具 ────────────────────────────────────────────────────────────

type cplnLine struct {
	toks   []cplnTok
	indent int
}

func cplnLines(toks []cplnTok) []cplnLine {
	var out []cplnLine
	var cur []cplnTok
	for _, t := range toks {
		if t.kind == cplnTokNL {
			if len(cur) > 0 {
				out = append(out, cplnLine{toks: cur, indent: cur[0].col})
				cur = nil
			}
			continue
		}
		cur = append(cur, t)
	}
	if len(cur) > 0 {
		out = append(out, cplnLine{toks: cur, indent: cur[0].col})
	}
	return out
}

func cplnDepths(toks []cplnTok) []int {
	d := make([]int, len(toks))
	cur := 0
	for i, t := range toks {
		d[i] = cur
		if t.kind == cplnTokOp {
			switch t.text {
			case "(", "[", "{":
				cur++
			case ")", "]", "}":
				cur--
			}
		}
	}
	return d
}

func cplnSplitTop(toks []cplnTok, sep string) ([][]cplnTok, bool) {
	d := cplnDepths(toks)
	var parts [][]cplnTok
	cur := []cplnTok{}
	saw := false
	for i, t := range toks {
		if d[i] == 0 && t.kind == cplnTokOp && t.text == sep {
			saw = true
			parts = append(parts, cur)
			cur = []cplnTok{}
			continue
		}
		cur = append(cur, t)
	}
	parts = append(parts, cur)
	return parts, saw
}

func cplnTopOpIdx(toks []cplnTok, want string) []int {
	d := cplnDepths(toks)
	var out []int
	for i, t := range toks {
		if d[i] == 0 && t.kind == cplnTokOp && t.text == want {
			out = append(out, i)
		}
	}
	return out
}

func cplnStripOuterParens(toks []cplnTok) []cplnTok {
	for len(toks) >= 2 && toks[0].kind == cplnTokOp && toks[0].text == "(" {
		cur := 0
		match := -1
		for i, t := range toks {
			if t.kind == cplnTokOp {
				if t.text == "(" {
					cur++
				} else if t.text == ")" {
					cur--
					if cur == 0 {
						match = i
						break
					}
				}
			}
		}
		if match != len(toks)-1 {
			break
		}
		toks = toks[1 : len(toks)-1]
	}
	return toks
}

// cplnStrConst —— 片段是否恰为一个**字符串常量**（含隐式拼接）。
func cplnStrConst(toks []cplnTok) (string, bool) {
	t := cplnStripOuterParens(toks)
	if len(t) == 0 {
		return "", false
	}
	var b strings.Builder
	for _, tk := range t {
		if tk.kind != cplnTokStr || tk.isF || tk.isB {
			return "", false
		}
		b.WriteString(tk.val)
	}
	return b.String(), true
}

// cplnAsTuple —— 片段是否为元组字面量（剥外括号后有顶层逗号；`()` = 空元组）。
func cplnAsTuple(toks []cplnTok) ([][]cplnTok, bool) {
	inner := cplnStripOuterParens(toks)
	if len(inner) == 0 {
		return nil, true
	}
	parts, saw := cplnSplitTop(inner, ",")
	if !saw {
		return nil, false
	}
	if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	return parts, true
}

var cplnStmtKeywords = map[string]bool{
	"if": true, "elif": true, "else": true, "while": true, "for": true, "def": true,
	"class": true, "with": true, "try": true, "except": true, "finally": true,
	"return": true, "yield": true, "import": true, "from": true, "del": true,
	"global": true, "nonlocal": true, "assert": true, "raise": true, "pass": true,
	"break": true, "continue": true, "lambda": true, "await": true, "async": true,
	"match": true, "case": true,
}

// ── 现读两侧（都不是判据，只负责取数）────────────────────────────────────────────

// cplnReadTrueLabels —— 真源层名闭集（`SKIP_LABELS` 现读）⇒ ([]层名, 说明)。
// 失败一律返 **nil**（不返空切片）⇒ 「取不到」与「空集合」分表意（空元组另判错）。
func cplnReadTrueLabels(path string) ([]string, string) {
	if !cplnStatRegular(path) {
		return nil, "真源件不在盘：" + path
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "真源件读不回来（" + cplnOSErrStr(err, path) + "）"
	}
	toks, lexErr := cplnLex(string(data))
	if lexErr != "" {
		return nil, "真源件语法不合 ⇒ 解析不出（" + lexErr + "）"
	}
	for _, ln := range cplnLines(toks) {
		if ln.indent != 0 || len(ln.toks) == 0 || ln.toks[0].kind != cplnTokName {
			continue
		}
		if cplnStmtKeywords[ln.toks[0].text] {
			continue
		}
		eqs := cplnTopOpIdx(ln.toks, "=")
		if len(eqs) == 0 {
			continue
		}
		last := eqs[len(eqs)-1]
		hasColon := false
		{
			d := cplnDepths(ln.toks)
			for i := 0; i < last; i++ {
				if d[i] == 0 && ln.toks[i].kind == cplnTokOp && ln.toks[i].text == ":" {
					hasColon = true
					break
				}
			}
		}
		if hasColon {
			continue
		}
		targetParts := [][]cplnTok{}
		{
			pre := ln.toks[:last]
			byEq, _ := cplnSplitTop(pre, "=")
			for _, seg := range byEq {
				ps, _ := cplnSplitTop(seg, ",")
				targetParts = append(targetParts, ps...)
			}
		}
		matchedTarget := false
		for _, tg := range targetParts {
			if len(tg) == 1 && tg[0].kind == cplnTokName && tg[0].text == cplnTrueLabelsName {
				matchedTarget = true
			}
		}
		if !matchedTarget {
			continue
		}
		elems, isTup := cplnAsTuple(ln.toks[last+1:])
		if !isTup {
			return nil, cplnTrueLabelsName + " 不是元组（真源换了形态 ⇒ 本件拒绝猜）"
		}
		out := []string{}
		for _, el := range elems {
			s, ok := cplnStrConst(el)
			if !ok {
				return nil, cplnTrueLabelsName + " 里有非字符串成员（真源换了形态 ⇒ 本件拒绝猜）"
			}
			out = append(out, s)
		}
		if len(out) == 0 {
			return nil, cplnTrueLabelsName + " 是空元组（真源层名闭集为空 = 无区分度 ⇒ 不给结论）"
		}
		return out, ""
	}
	return nil, "真源件里找不到模块级赋值 " + cplnTrueLabelsName + "（层名口径的单一真源没了）"
}

// cplnBodyLines —— `def <name>` 行的函数体逻辑行（缩进 > def 行缩进，直到不再大于）。
func cplnBodyLines(lines []cplnLine, idx int) []cplnLine {
	defIndent := lines[idx].indent
	var body []cplnLine
	for k := idx + 1; k < len(lines); k++ {
		if lines[k].indent <= defIndent {
			break
		}
		body = append(body, lines[k])
	}
	return body
}

// cplnReadTrueUsage —— 真源自洽取数 ⇒ (exclude_kind 的 return 字面量, list_only_report 的 skip[字面量], 说明)。
func cplnReadTrueUsage(path string) ([]string, []string, string) {
	if !cplnStatRegular(path) {
		return nil, nil, "真源件不在盘：" + path
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, "真源件读不回来（" + cplnOSErrStr(err, path) + "）"
	}
	toks, lexErr := cplnLex(string(data))
	if lexErr != "" {
		return nil, nil, "真源件语法不合 ⇒ 解析不出（" + lexErr + "）"
	}
	lines := cplnLines(toks)
	rets := []string{}
	subs := []string{}
	retsSet := map[string]bool{}
	subsSet := map[string]bool{}
	for idx, ln := range lines {
		if len(ln.toks) < 2 || ln.toks[0].kind != cplnTokName || ln.toks[0].text != "def" || ln.toks[1].kind != cplnTokName {
			continue
		}
		name := ln.toks[1].text
		if name != "exclude_kind" && name != "list_only_report" {
			continue
		}
		body := cplnBodyLines(lines, idx)
		if name == "exclude_kind" {
			for _, bl := range body {
				if len(bl.toks) >= 1 && bl.toks[0].kind == cplnTokName && bl.toks[0].text == "return" {
					if s, ok := cplnStrConst(bl.toks[1:]); ok && !retsSet[s] {
						retsSet[s] = true
						rets = append(rets, s)
					}
				}
			}
		} else {
			for _, bl := range body {
				t := bl.toks
				for k := 0; k+1 < len(t); k++ {
					if t[k].kind != cplnTokName || t[k].text != "skip" {
						continue
					}
					if k > 0 && t[k-1].kind == cplnTokOp && t[k-1].text == "." {
						continue
					}
					if t[k+1].kind != cplnTokOp || t[k+1].text != "[" {
						continue
					}
					d := 0
					end := -1
					for m := k + 1; m < len(t); m++ {
						if t[m].kind != cplnTokOp {
							continue
						}
						switch t[m].text {
						case "(", "[", "{":
							d++
						case ")", "]", "}":
							d--
							if d == 0 {
								end = m
							}
						}
						if end >= 0 {
							break
						}
					}
					if end < 0 {
						continue
					}
					if s, ok := cplnStrConst(t[k+2 : end]); ok && !subsSet[s] {
						subsSet[s] = true
						subs = append(subs, s)
					}
				}
			}
		}
	}
	return rets, subs, ""
}

func cplnReadCLISentinel(path string) (string, bool) {
	if !cplnStatRegular(path) {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	for _, m := range cplnCLILayerRE.FindAllStringSubmatch(string(data), -1) {
		if m[1] == cplnSentinelID {
			return m[2], true
		}
	}
	return "", false
}

// cplnReadCLILayers —— CLI 层名闭集 ⇒ ([]层名, 说明)。哨兵单列（不算层名）。
func cplnReadCLILayers(path string) ([]string, string) {
	if !cplnStatRegular(path) {
		return nil, "CLI 件不在盘：" + path
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "CLI 件读不回来（" + cplnOSErrStr(err, path) + "）"
	}
	out := []string{}
	hasSentinel := false
	for _, m := range cplnCLILayerRE.FindAllStringSubmatch(string(data), -1) {
		if m[1] == cplnSentinelID {
			hasSentinel = true
			continue
		}
		out = append(out, m[2])
	}
	if len(out) == 0 {
		return nil, "CLI 件里一条 `publishSetLayer<名> = \"…\"` 都没解析出来（形态换了 ⇒ 不给结论）"
	}
	if !hasSentinel {
		return nil, "CLI 件里没有哨兵常量 `" + cplnSentinelID + "`（「没命中任何层」那一格的值取不到 ⇒ 不给结论）"
	}
	return out, ""
}

// cplnObservedFromLive —— 运行面观测集 = 各件 `layer` 值 ∪ `non_blob_basis` 里出现的闭集名。
func cplnObservedFromLive(items []any, cliLayers []string) (map[string]bool, map[string]string) {
	seen := map[string]bool{}
	per := map[string]string{}
	for _, it := range items {
		m, _ := it.(map[string]any)
		rel := cplnAnyStrOrEmpty(m["path"])
		layer := cplnAnyStrOrEmpty(m["layer"])
		per[rel] = layer
		if layer != "" && layer != cplnLayerNone {
			seen[layer] = true
		}
		basis := cplnAnyStrOrEmpty(m["non_blob_basis"])
		for _, name := range cliLayers {
			if strings.Contains(basis, name) {
				seen[name] = true
			}
		}
	}
	return seen, per
}

// cplnAnyStrOrEmpty —— py `x or ""` 的近似（nil / 空串 ⇒ ""；字符串原样）。
func cplnAnyStrOrEmpty(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func cplnContains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func cplnSortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cplnSortedCopy(ss []string) []string {
	out := append([]string{}, ss...)
	sort.Strings(out)
	return out
}

// ── 判据本体（唯一一处 · 纯函数：不吃盘、不起进程）────────────────────────────────

func cplnMatched(c, s string) bool {
	return c == s || strings.HasPrefix(c, s+"（")
}

func cplnJudge(trueLabels, trueReturns, trueSubs, cliLayers []string, anchorsOK map[string]bool,
	live bool, observed map[string]bool, per map[string]string) []string {
	bad := []string{}

	for _, name := range cplnSortedCopy(trueReturns) {
		if !cplnContains(trueLabels, name) {
			bad = append(bad, fmt.Sprintf("L1 真源自洽：`exclude_kind` 返回的 %s 不在 %s 里 （真源自己就先漂了：分类口吐了一个闭集外的层名）", cplnPyRepr(name), cplnTrueLabelsName))
		}
	}
	for _, name := range cplnSortedCopy(trueSubs) {
		if !cplnContains(trueLabels, name) {
			bad = append(bad, fmt.Sprintf("L1 真源自洽：只读预演的 `skip[%s]` 不在 %s 里（计数口吐了闭集外的层名）", cplnPyRepr(name), cplnTrueLabelsName))
		}
	}
	for _, s := range trueLabels {
		ok := false
		for _, c := range cliLayers {
			if cplnMatched(c, s) {
				ok = true
				break
			}
		}
		if !ok {
			bad = append(bad, fmt.Sprintf("L3 甲 对拍：真源层名 %s **没有**对应的 CLI 层名（真源增层 / 改名 ⇒ CLI 静默落后）", cplnPyRepr(s)))
		}
	}
	for _, c := range cliLayers {
		matched := false
		for _, s := range trueLabels {
			if cplnMatched(c, s) {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		entryIdx := -1
		for i, e := range cplnCliOnly {
			if e.Name == c {
				entryIdx = i
				break
			}
		}
		if entryIdx < 0 {
			bad = append(bad, fmt.Sprintf("L3 乙 对拍：CLI 层名 %s 在真源闭集（%s）里没有对应、也不在 CLI_ONLY 表里 ⇒ CLI 自造层名（谁都不知道这一层按什么算的）", cplnPyRepr(c), strings.Join(trueLabels, " · ")))
			continue
		}
		if !anchorsOK[c] {
			bad = append(bad, fmt.Sprintf("L3 乙 对拍：CLI 层名 %s 的 CLI_ONLY 锚点现读**找不着**了（%s 里的 %s —— 真源那处处置被删/改了 ⇒ 这条白名单腐化，必须重判）", cplnPyRepr(c), cplnCliOnly[entryIdx].LibRel, cplnPyRepr(cplnCliOnly[entryIdx].Anchor)))
		}
	}

	if !live {
		return bad
	}

	cliSet := map[string]bool{}
	for _, c := range cliLayers {
		cliSet[c] = true
	}
	for _, name := range cplnSortedKeys(observed) {
		if !cliSet[name] {
			bad = append(bad, fmt.Sprintf("L4 运行面：真跑吐出了闭集外的层名 %s（声明了一套、真跑另一套）", cplnPyRepr(name)))
		}
	}
	for _, name := range cliLayers {
		if !observed[name] {
			bad = append(bad, fmt.Sprintf("L4 运行面：CLI 声明的层名 %s 在探针集里**一次都没观测到**（要么探针退化、要么这一层已经吐不出来了）", cplnPyRepr(name)))
		}
	}
	for _, pr := range cplnProbes {
		rel, want := pr[0], pr[1]
		got, ok := per[rel]
		if !ok {
			bad = append(bad, fmt.Sprintf("L4 探针：%s 不在现跑结果里（探针没了 ⇒ 无区分度）", rel))
		} else if got != want {
			bad = append(bad, fmt.Sprintf("L4 探针：%s 实得层名 %s ≠ 期望 %s（探针退化 / 那一层被人挪走了）", rel, cplnPyRepr(got), cplnPyRepr(want)))
		}
	}
	if got, ok := per[cplnProbeEmit]; !ok {
		bad = append(bad, fmt.Sprintf("L4 正控：%s 不在现跑结果里", cplnProbeEmit))
	} else if got != cplnLayerNone {
		bad = append(bad, fmt.Sprintf("L4 正控：%s 实得层名 %s ≠ %s（「会发」那一件不该命中任何层）", cplnProbeEmit, cplnPyRepr(got), cplnPyRepr(cplnLayerNone)))
	}
	return bad
}

// ── 夹具（合成 · 真文件）────────────────────────────────────────────────────────

func cplnMakeFixture(fxRoot, caseName, libSrc, goSrc string, libPatch, goPatch func(string) string) string {
	d := cplnPyJoin(fxRoot, caseName)
	lib := libSrc
	if libPatch != nil {
		lib = libPatch(libSrc)
	}
	goText := goSrc
	if goPatch != nil {
		goText = goPatch(goSrc)
	}
	_ = os.MkdirAll(cplnPyJoin(d, cplnPyDirname(cplnLibRel)), 0o755)
	_ = os.MkdirAll(cplnPyJoin(d, cplnPyDirname(cplnCLIRel)), 0o755)
	_ = os.WriteFile(cplnPyJoin(d, cplnLibRel), []byte(lib), 0o644)
	_ = os.WriteFile(cplnPyJoin(d, cplnCLIRel), []byte(goText), 0o644)
	return d
}

func cplnAddTrueLayer(src string) string {
	return strings.Replace(src, `SKIP_LABELS = ("EXCLUDES"`, `SKIP_LABELS = ("夹具新层", "EXCLUDES"`, 1)
}

func cplnAddCLILayer(src string) string {
	return strings.Replace(src, "\tpublishSetLayerWhitelist = \"白名单未命中\"",
		"\tpublishSetLayerWhitelist = \"白名单未命中\"\n\tpublishSetLayerFixtureX  = \"夹具自造层\"", 1)
}

type cplnCase struct {
	name    string
	libP    func(string) string
	goP     func(string) string
	wantRed []string
}

var cplnCases = []cplnCase{
	{"正控-两侧全合", nil, nil, nil},
	{"负控①-CLI多一层", nil, cplnAddCLILayer, []string{"L3 乙 对拍：CLI 层名 '夹具自造层'"}},
	{"负控②-真源多一层", cplnAddTrueLayer, nil, []string{"L3 甲 对拍：真源层名 '夹具新层'"}},
}

func cplnSelfTest(out io.Writer, fxRoot, repo string) bool {
	if repo == "" {
		repo = cplnDefaultRepoRoot()
	}
	if fxRoot == "" {
		fxRoot = cplnPyJoin(os.TempDir(), "zerg-publish-layer-names-fx")
	}
	libData, err := os.ReadFile(cplnPyJoin(repo, cplnLibRel))
	if err != nil {
		fmt.Fprintf(out, "⛔ 不给结论（rc=2）：自检读不到真源件（%s）\n", cplnOSErrStr(err, cplnPyJoin(repo, cplnLibRel)))
		return false
	}
	goData, err := os.ReadFile(cplnPyJoin(repo, cplnCLIRel))
	if err != nil {
		fmt.Fprintf(out, "⛔ 不给结论（rc=2）：自检读不到 CLI 件（%s）\n", cplnOSErrStr(err, cplnPyJoin(repo, cplnCLIRel)))
		return false
	}
	libSrc := string(libData)
	goSrc := string(goData)
	fmt.Fprintf(out, "── 自检（合成夹具 · 真件副本 · 不碰真目标）──\n夹具根：%s\n", fxRoot)
	total, failed := 0, 0
	for _, cs := range cplnCases {
		d := cplnMakeFixture(fxRoot, cs.name, libSrc, goSrc, cs.libP, cs.goP)
		labels, why1 := cplnReadTrueLabels(cplnPyJoin(d, cplnLibRel))
		cli, why2 := cplnReadCLILayers(cplnPyJoin(d, cplnCLIRel))
		if labels == nil || cli == nil {
			fmt.Fprintf(out, "  ❌ %s：夹具读不回来（%s / %s）\n", cs.name, why1, why2)
			failed++
			total++
			continue
		}
		rets, subs, _ := cplnReadTrueUsage(cplnPyJoin(d, cplnLibRel))
		fxLibData, _ := os.ReadFile(cplnPyJoin(d, cplnLibRel))
		fxLib := string(fxLibData)
		anchors := map[string]bool{}
		for _, e := range cplnCliOnly {
			anchors[e.Name] = strings.Contains(fxLib, e.Anchor)
		}
		bad := cplnJudge(labels, rets, subs, cli, anchors, false, nil, nil)
		var hit []string
		if len(cs.wantRed) > 0 {
			for _, b := range bad {
				for _, w := range cs.wantRed {
					if strings.HasPrefix(b, w) {
						hit = append(hit, b)
						break
					}
				}
			}
		}
		ok := false
		if len(cs.wantRed) > 0 {
			ok = len(hit) > 0
		} else {
			ok = len(bad) == 0
		}
		total++
		if !ok {
			failed++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		expStr := "绿"
		if len(cs.wantRed) > 0 {
			expStr = "红"
		}
		gotStr := "绿"
		if len(bad) > 0 {
			gotStr = "红"
		}
		suffix := ""
		if len(bad) > 0 && !ok {
			suffix = "：" + bad[0]
		}
		fmt.Fprintf(out, "  %s %s（期望%s · 实得%s%s）\n", mark, cs.name, expStr, gotStr, suffix)
	}
	fmt.Fprintf(out, "  自检 %d 格 · 失败 %d 格\n", total, failed)
	return failed == 0
}

// ── 旗标（argparse 同构：长选项前缀消歧 + 三种报错正文）────────────────────────────

type cplnOpts struct {
	repoRoot    string
	repoRootSet bool
	bin         string
	noLive      bool
	jsonOut     bool
	listRules   bool
	selfTest    bool
	selfTestDir string
}

var cplnLongOpts = []string{"--help", "--repo-root", "--bin", "--no-live", "--json", "--list-rules", "--self-test", "--self-test-dir"}

func cplnArgError(stderr io.Writer, msg string) {
	fmt.Fprint(stderr, cplnUsageText)
	fmt.Fprintf(stderr, "check-publish-layer-names.py: error: %s\n", msg)
}

// cplnResolveLong ⇒ (规范名, 歧义候选)；无匹配 ⇒ ("", nil)。
func cplnResolveLong(name string) (string, []string) {
	for _, o := range cplnLongOpts {
		if o == name {
			return o, nil
		}
	}
	var cands []string
	for _, o := range cplnLongOpts {
		if strings.HasPrefix(o, name) {
			cands = append(cands, o)
		}
	}
	if len(cands) == 1 {
		return cands[0], nil
	}
	if len(cands) > 1 {
		return "", cands
	}
	return "", nil
}

func cplnParseArgs(argv []string, stdout, stderr io.Writer) (cplnOpts, int, bool) {
	o := cplnOpts{}
	var unrecognized []string
	i := 0
	for i < len(argv) {
		a := argv[i]
		if a == "--" {
			unrecognized = append(unrecognized, argv[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "--") {
			name := a
			val := ""
			hasVal := false
			if k := strings.IndexByte(a, '='); k >= 0 {
				name = a[:k]
				val = a[k+1:]
				hasVal = true
			}
			opt, ambig := cplnResolveLong(name)
			if len(ambig) > 0 {
				cplnArgError(stderr, "ambiguous option: "+name+" could match "+strings.Join(ambig, ", "))
				return o, 2, true
			}
			if opt == "" {
				unrecognized = append(unrecognized, a)
				i++
				continue
			}
			switch opt {
			case "--help":
				fmt.Fprint(stdout, cplnHelpText)
				return o, 0, true
			case "--repo-root", "--bin", "--self-test-dir":
				if !hasVal {
					if i+1 >= len(argv) {
						cplnArgError(stderr, "argument "+name+": expected one argument")
						return o, 2, true
					}
					val = argv[i+1]
					i++
				}
				switch opt {
				case "--repo-root":
					o.repoRoot = val
					o.repoRootSet = true
				case "--bin":
					o.bin = val
				case "--self-test-dir":
					o.selfTestDir = val
				}
			case "--no-live":
				if hasVal {
					cplnArgError(stderr, "argument "+name+": ignored explicit argument "+cplnPyRepr(val))
					return o, 2, true
				}
				o.noLive = true
			case "--json":
				if hasVal {
					cplnArgError(stderr, "argument "+name+": ignored explicit argument "+cplnPyRepr(val))
					return o, 2, true
				}
				o.jsonOut = true
			case "--list-rules":
				if hasVal {
					cplnArgError(stderr, "argument "+name+": ignored explicit argument "+cplnPyRepr(val))
					return o, 2, true
				}
				o.listRules = true
			case "--self-test":
				if hasVal {
					cplnArgError(stderr, "argument "+name+": ignored explicit argument "+cplnPyRepr(val))
					return o, 2, true
				}
				o.selfTest = true
			}
			i++
			continue
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			if a == "-h" {
				fmt.Fprint(stdout, cplnHelpText)
				return o, 0, true
			}
			unrecognized = append(unrecognized, a)
			i++
			continue
		}
		unrecognized = append(unrecognized, a)
		i++
	}
	if len(unrecognized) > 0 {
		cplnArgError(stderr, "unrecognized arguments: "+strings.Join(unrecognized, " "))
		return o, 2, true
	}
	return o, 0, false
}

// cplnDefaultRepoRoot —— Go 无 `__file__` ⇒ 从 cwd 上溯找含 `publish/mirror-public-lib.py` 的目录。
func cplnDefaultRepoRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	d := cwd
	for {
		if cplnStatRegular(cplnPyJoin(d, cplnLibRel)) {
			return d
		}
		parent := cplnPyDirname(d)
		if parent == d || parent == "" {
			return cwd
		}
		d = parent
	}
}

// ── 输出：JSON（py `json.dumps(..., ensure_ascii=False)` 逐字形态）────────────────

func cplnJSONStr(s string) string {
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

func cplnJSONList(ss []string) string {
	parts := make([]string, len(ss))
	for i, s := range ss {
		parts[i] = cplnJSONStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ── 入口 ─────────────────────────────────────────────────────────────────────

func checkPublishLayerNamesGateMain(stdout, stderr io.Writer, argv []string) int {
	o, rc, handled := cplnParseArgs(argv, stdout, stderr)
	if handled {
		return rc
	}

	if o.listRules {
		fmt.Fprint(stdout, cplnListRules)
		return 0
	}
	if o.selfTest {
		if cplnSelfTest(stdout, o.selfTestDir, o.repoRoot) {
			return 0
		}
		return 2
	}

	repoRoot := o.repoRoot
	if !o.repoRootSet {
		repoRoot = cplnDefaultRepoRoot()
	}

	lib := cplnPyJoin(repoRoot, cplnLibRel)
	goFile := cplnPyJoin(repoRoot, cplnCLIRel)
	trueLabels, why := cplnReadTrueLabels(lib)
	if trueLabels == nil {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", why)
		return 2
	}
	cliLayers, why := cplnReadCLILayers(goFile)
	if cliLayers == nil {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", why)
		return 2
	}
	trueReturns, trueSubs, whyUsage := cplnReadTrueUsage(lib)
	if trueReturns == nil {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", whyUsage)
		return 2
	}

	libData, err := os.ReadFile(lib)
	if err != nil {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：真源件读不回来（%s）\n", cplnOSErrStr(err, lib))
		return 2
	}
	libText := string(libData)
	anchorsOK := map[string]bool{}
	for _, e := range cplnCliOnly {
		anchorsOK[e.Name] = strings.Contains(libText, e.Anchor)
	}
	sentinel, sentinelOK := cplnReadCLISentinel(goFile)

	fmt.Fprintf(stdout, "CLI 层名闭集 ⟷ 真源层名口径 对拍 · v%s\n", cplnVersion)
	fmt.Fprintf(stdout, "  真源闭集（%s · %d）：%s\n", cplnTrueLabelsName, len(trueLabels), strings.Join(trueLabels, " · "))
	fmt.Fprintf(stdout, "  CLI 闭集（publishSetLayer* · %d）：%s\n", len(cliLayers), strings.Join(cliLayers, " · "))

	var observed map[string]bool
	var per map[string]string
	live := false
	itemsLen := 0
	if o.noLive {
		fmt.Fprint(stdout, "  运行面：**未跑**（--no-live：只判 L1–L3）\n")
	} else {
		binz := o.bin
		if binz == "" {
			binz = cplnPyJoin(repoRoot, "bin", "zerg")
		}
		args := []string{"publish", "set"}
		for _, p := range cplnProbes {
			args = append(args, p[0])
		}
		args = append(args, cplnProbeEmit)
		args = append(args, "--json", "path,caliber,will_publish,layer,non_blob_basis")
		cmd := exec.Command(binz, args...)
		cmd.Dir = repoRoot
		cmd.Stdin = nil
		var so, se bytes.Buffer
		cmd.Stdout = &so
		cmd.Stderr = &se
		runErr := cmd.Run()
		liveRC := 0
		if runErr != nil {
			if ee, ok := runErr.(*exec.ExitError); ok {
				liveRC = ee.ExitCode()
			} else {
				fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：现跑 `publish set … --json` 起不来（%s）\n", runErr.Error())
				return 2
			}
		}
		if liveRC != 0 {
			fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：现跑 `publish set … --json` 退码 %d（stderr 前 200 字：%s）\n",
				liveRC, cplnPyCut(cplnPyStrip(se.String()), 200))
			return 2
		}
		var doc map[string]any
		if uerr := json.Unmarshal(so.Bytes(), &doc); uerr != nil {
			fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：现跑输出不是 JSON（%s）\n", uerr.Error())
			return 2
		}
		items := []any{}
		if v, ok := doc["items"]; ok {
			if lst, ok2 := v.([]any); ok2 {
				items = lst
			}
		}
		if len(items) == 0 {
			fmt.Fprint(stdout, "⛔ 不给结论（rc=2）：现跑 `items` 为空（零件 ⇒ 无区分度）\n")
			return 2
		}
		observed, per = cplnObservedFromLive(items, cliLayers)
		live = true
		itemsLen = len(items)
		fmt.Fprintf(stdout, "  运行面（现跑 %d 件）：观测层名 %d 个 —— %s\n",
			itemsLen, len(observed), strings.Join(cplnSortedKeys(observed), " · "))
	}

	bad := cplnJudge(trueLabels, trueReturns, trueSubs, cliLayers, anchorsOK, live, observed, per)
	sentinelVal := sentinel
	if !sentinelOK {
		sentinelVal = ""
	}
	if sentinelVal != cplnLayerNone {
		bad = append(bad, fmt.Sprintf("L2 哨兵：「没命中任何层」那一格的值 %s ≠ 本件认的 %s（`%s` 被人改动了 —— 那一格一改，「会发」的件就会被打上一个层名）",
			cplnPyRepr(sentinelVal), cplnPyRepr(cplnLayerNone), cplnSentinelID))
	}
	if len(bad) > 0 {
		fmt.Fprint(stdout, "⇒ 判红（rc=1）：\n")
		for _, b := range bad {
			fmt.Fprintf(stdout, "  ✗ %s\n", b)
		}
	} else {
		fmt.Fprint(stdout, "  ✓ L1 真源自洽（分类口/计数口只吐闭集里的名字）\n")
		fmt.Fprintf(stdout, "  ✓ L2 CLI 闭集 %d 个（现读常量块）\n", len(cliLayers))
		extra := 0
		for _, c := range cliLayers {
			anyMatch := false
			for _, s := range trueLabels {
				if cplnMatched(c, s) {
					anyMatch = true
					break
				}
			}
			if !anyMatch {
				extra++
			}
		}
		fmt.Fprintf(stdout, "  ✓ L3 双向对拍：真源 %d 个层名全部有 CLI 对应；CLI 侧多出的 %d 个都有真源锚点（现读校验过）\n", len(trueLabels), extra)
		if live {
			fmt.Fprintf(stdout, "  ✓ L4 运行面 == 声明面（观测 %d 个层名 · 探针 %d 枚逐条命中期望 · 「会发」正控 = %s）\n", len(observed), len(cplnProbes), cplnLayerNone)
		}
		fmt.Fprint(stdout, "⇒ 四条都合（rc=0）\n")
	}
	if o.jsonOut {
		var observedJSON string
		if live {
			observedJSON = cplnJSONList(cplnSortedKeys(observed))
		} else {
			observedJSON = "null"
		}
		liveJSON := "false"
		if live {
			liveJSON = "true"
		}
		cliOnlyNames := []string{}
		for _, e := range cplnCliOnly {
			cliOnlyNames = append(cliOnlyNames, e.Name)
		}
		fmt.Fprintf(stdout, "{\"schema\": \"zerg-gate/v1\", \"gate\": \"check-publish-layer-names\", \"version\": %s, \"true_labels\": %s, \"cli_layers\": %s, \"cli_only\": %s, \"live\": %s, \"observed\": %s, \"bad\": %s, \"mode\": \"judge\"}\n",
			cplnJSONStr(cplnVersion), cplnJSONList(trueLabels), cplnJSONList(cliLayers), cplnJSONList(cliOnlyNames), liveJSON, observedJSON, cplnJSONList(bad))
	}
	if len(bad) > 0 {
		return 1
	}
	return 0
}
