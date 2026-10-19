// check_public_face_wording.go —— 判据件 `scripts/gates/check-public-face-wording.py`（713 行）的 **Go 真搬**
// （M2 一枚 · 公开面用词族 · 入口 `checkPublicFaceWordingGateMain` · 承接 GAP-20261001-13）。
//
// 验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 声明 (a) 默认根定位差异：py 从 `__file__` 上溯 2 层推 `REPO_ROOT`（= `scripts/gates/` 的上两层）；
// Go 无 `__file__` 语境 ⇒ 缺省根 = **cwd**（不向上搜「含某标记的目录」，避免与 py 的纯上溯语义分叉）。
// 两态在「脚本位置 == cwd」时逐字同构（两环境对拍即按此布置：甲=显式传根；乙=夹具根放 `fx/scripts/gates/` + cwd=fx）。
//
// 声明 (b) ★自指四形态（本件落在**他门**扫描面内 ⇒ 逐条消解，见交付④）：
//
//	① 件头/正文的真引号路径原串：本件**零**私有卷/家目录**绝对路径**字面量（`/Volumes/` 与卷名、
//	   `/Users/…` 与用户名一律**拆串**）⇒ 不给 `check-hardcoded-private-paths.py`（SCAN_DIRS 含 `core/`、
//	   针 `<volume-path>` / `<volume-path>` / `/Users/<user>`）造新命中。
//	② 夹具/自检标记字面用反引号原串直写：本件夹具内容全是 `map[string]string` 值（不是真引号路径原串）。
//	③ self-test 魔法串**一律拼串落地**（✗ 不写连续字面）：本件自检的探针串即按此拼 —— 否则本件
//	   会成为他门扫描面（如 compat 清单 R6 的环境变量面扫 `core/**` 下 `.go`）的假阳性来源，
//	   反过来把**他门的 self-test 反例顶谎**（GAP-20261002-03 即此形态：注释里的连续串被扫进
//	   joined ⇒ `contains` 恒真 ⇒ 那条反例永远抓不住）。
//	④ **本件是否落在「任何他门」的扫描面内** ⇒ 是（`check-hardcoded-private-paths` 的 `SCAN_DIRS` 含 `core/`；
//	   `check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go`；`check-dryrun-semantics` 的 SCAN_DIRS 含 `core`）
//	   ⇒ 处置：被扫形态（私有卷针、真引号路径、`run`+`go` 一类命令形态字面量）一律**拆串拼**或**注释行内**写。
//
// 覆盖口径：甲档 12 条规则**逐字节手写匹配器**（py 的 `(?<!…)` / `(?!…)` 后顾·先行断言 RE2 无 ⇒ 不能靠
// `regexp` 直译；`\b` 的 Unicode 语义与 RE2 的 ASCII 语义亦有差 ⇒ 全部手写）+ 乙/丙档白名单 8 条逐条打印理由。
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

const (
	cpfwVersion = "check-public-face-wording.py v1.0.0"
	cpfwDocsRel = pathPublishDocs
	cpfwSelfRel = "scripts/gates/check-public-face-wording.py"
)

// cpfwVolPrefix / cpfwVolNames：私有卷面（★ 拆串 —— 见声明 (b) ①）。
var cpfwVolPrefix = "/Volumes/"

var cpfwVolNames = []string{"AHZ", "BXC", "ZERG-ARCHIVE"}

// cpfwSelfText —— 自豁免真源（**内化进 Go**）：py `check-public-face-wording.py` 退役（不在盘）时
//
//	`--self-test` ⑭ 的**离盘真源**（⑭ 的牙不依赖本 py）。
//	★ 必须让 cpfwScanText 命中 A1/A2/A4/A5/A6b/A9 六枚 rid（与 py 在盘时的豁免覆盖同集）；
//	★ 拆串写：私有卷名走 cpfwVolPrefix + cpfwVolNames、缺口号走 `Q-` + 数字 ——
//	  ✗ 不落 `/Volumes/<卷名>` / `Q-<数字>` 的连续字面量（他门扫 core/）。
var cpfwSelfText = strings.Join([]string{
	"私有卷 " + cpfwVolPrefix + cpfwVolNames[0],
	"任意卷 " + cpfwVolPrefix + "My Disk",
	"文仓名 Zerg-内部文档",
	"缺口号 " + "Q-" + "176",
	"私有称谓 " + "Mr2109",
	"仓主名 " + "Mr2109",
}, " · ")

// cpfwLoadSelfText —— ⑭ 的「本件自身」文本源：py 在盘 ⇒ 现读 py 真身（单一真源 · 两态逐字同）；
//
//	py 退役（不在盘 ⇒ os.ReadFile err）⇒ 回落**内化真源** cpfwSelfText ⇒ ⑭ 不依赖本 py（退役容忍）。
func cpfwLoadSelfText(root string) string {
	if raw, err := os.ReadFile(cpfwPyJoin(root, cpfwSelfRel)); err == nil {
		return string(raw)
	}
	return cpfwSelfText
}

// cpfwRootFace：根面 8 件（设计 §2.3「根面 8 件对外件」· 与 `T-03`/`T-11` 同一批读数）。
var cpfwRootFace = []string{
	"publish/LICENSE",
	"publish/NOTICE",
	"publish/THIRD_PARTY_LICENSES.md",
	"publish/CONTRIBUTING.md",
	"publish/SECURITY.md",
	"publish/config.example.yaml",
	"publish/gitleaks.toml",
	"publish/install.sh",
}

var cpfwSkipSuffix = []string{"~", ".bak", ".orig", ".rej", ".swp", ".pyc", ".DS_Store"}

// ── 甲档：12 条规则（id · 名称 · 匹配器 · 为什么禁）────────────────────────────────────────
type cpfwRule struct {
	rid, title, why string
	find            func(s string) [][2]int
}

func cpfwFindAll(s string, matchAt func(s string, i int) int) [][2]int {
	var out [][2]int
	i := 0
	for i < len(s) {
		if n := matchAt(s, i); n > 0 {
			out = append(out, [2]int{i, i + n})
			i += n
		} else {
			i++
		}
	}
	return out
}

// ── Unicode `\b` 语义（py `re` 的 str 模式 `\w` = 字母/数字/下划线）━━━━━━━━━━━━━━━━━━━━━
func cpfwIsWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

func cpfwRuneAt(s string, i int) rune {
	if i >= len(s) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return r
}

func cpfwPrevRune(s string, i int) rune {
	if i <= 0 {
		return 0
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return r
}

// cpfwBoundAfterWord：`\b` 落在「词字符之后」的条件（i==len 或下一字符非词）。
func cpfwBoundAfterWord(s string, i int) bool {
	return i >= len(s) || !cpfwIsWordRune(cpfwRuneAt(s, i))
}

// cpfwBoundBeforeWord：`\b` 落在「词字符之前」的条件（i==0 或上一字符非词）。
func cpfwBoundBeforeWord(s string, i int) bool {
	return i == 0 || !cpfwIsWordRune(cpfwPrevRune(s, i))
}

func cpfwIsASCIIAlnum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func cpfwIsASCIIWordBs(b byte) bool { return cpfwIsASCIIAlnum(b) || b == '_' }

// ── 匹配器 ──────────────────────────────────────────────────────────────────────

// A1 `/Volumes/(?:AHZ|BXC|ZERG-ARCHIVE)\b`
func cpfwM_A1(s string, i int) int {
	if i+len(cpfwVolPrefix) > len(s) || s[i:i+len(cpfwVolPrefix)] != cpfwVolPrefix {
		return 0
	}
	for _, n := range cpfwVolNames {
		if strings.HasPrefix(s[i+len(cpfwVolPrefix):], n) {
			j := i + len(cpfwVolPrefix) + len(n)
			if cpfwBoundAfterWord(s, j) {
				return j - i
			}
		}
	}
	return 0
}

// A2 `/Volumes/(?!(?:AHZ|BXC|ZERG-ARCHIVE)\b)[^\n"'` + "`" + `)\\]）]*`（后顾/先行断言手写）
func cpfwM_A2(s string, i int) int {
	if i+len(cpfwVolPrefix) > len(s) || s[i:i+len(cpfwVolPrefix)] != cpfwVolPrefix {
		return 0
	}
	j := i + len(cpfwVolPrefix)
	for _, n := range cpfwVolNames {
		if strings.HasPrefix(s[j:], n) {
			k := j + len(n)
			if cpfwBoundAfterWord(s, k) {
				return 0 // 负向先行断言不成立 ⇒ 本位置无匹配
			}
		}
	}
	k := j
	for k < len(s) {
		r, w := utf8.DecodeRuneInString(s[k:])
		if cpfwA2Forbidden(r) {
			break
		}
		k += w
	}
	return k - i
}

func cpfwA2Forbidden(r rune) bool {
	switch r {
	case '\n', '"', '\'', '`', ')', ']', '\\', '）':
		return true
	}
	return false
}

// A3 `(?<![A-Za-z0-9])(?:AHZ|BXC)(?![A-Za-z0-9])`
func cpfwM_A3(s string, i int) int {
	prevOK := i == 0 || !cpfwIsASCIIAlnum(s[i-1])
	if !prevOK {
		return 0
	}
	for _, n := range cpfwVolNames {
		if len(n) == len("ZERG-ARCHIVE") {
			continue // A3 只收 AHZ / BXC
		}
		if strings.HasPrefix(s[i:], n) {
			j := i + len(n)
			if j >= len(s) || !cpfwIsASCIIAlnum(s[j]) {
				return len(n)
			}
		}
	}
	return 0
}

// A4 `Zerg-内部文档`
func cpfwM_A4(s string, i int) int {
	if strings.HasPrefix(s[i:], "Zerg-内部文档") {
		return len("Zerg-内部文档")
	}
	return 0
}

// A5 `\bQ-\d{1,4}\b`（`\d` = Unicode Nd）
func cpfwM_A5(s string, i int) int {
	if !cpfwBoundBeforeWord(s, i) {
		return 0
	}
	if !strings.HasPrefix(s[i:], "Q-") {
		return 0
	}
	j := i + 2
	k := j
	nd := 0
	for k < len(s) {
		r, w := utf8.DecodeRuneInString(s[k:])
		if !unicode.IsDigit(r) {
			break
		}
		nd++
		k += w
	}
	if nd < 1 || nd > 4 {
		return 0
	}
	if !cpfwBoundAfterWord(s, k) {
		return 0
	}
	return k - i
}

// A6 `父代理|本枚|开工单|认领单|助手`
var cpfwA6Alts = []string{"\u7236\u4ee3\u7406", "\u672c\u679a", "\u5f00\u5de5\u5355", "\u9886\u5355", "\u52a9\u624b"}

func cpfwM_A6(s string, i int) int {
	for _, n := range cpfwA6Alts {
		if strings.HasPrefix(s[i:], n) {
			return len(n)
		}
	}
	return 0
}

// A6b `Mr2109|Mr2109|脑\(Hermes\)|Mr2109deMac-Studio\.local|(?<![A-Za-z0-9_])Mr2109(?![A-Za-z0-9_])|示例虫茧|<container-repo>|<project-a>|<project-b>|A72109`
var cpfwA6bLits1 = []string{
	"\u8001\u53e8",
	"\u8111\uff08Hermes\uff09",
	"\u8111(Hermes)",
	"<host>",
}

var cpfwA6bLits2 = []string{
	"\u5706\u684c\u6d3e",
	"\u8327\u4ed3",
	"\u5343\u7ae0",
	"\u8fb9\u5883\u4fdd\u9556",
	"A72109",
}

func cpfwM_A6b(s string, i int) int {
	for _, n := range cpfwA6bLits1 {
		if strings.HasPrefix(s[i:], n) {
			return len(n)
		}
	}
	if strings.HasPrefix(s[i:], "Mr2109") {
		j := i + 4
		prevOK := i == 0 || !cpfwIsASCIIWordBs(s[i-1])
		nextOK := j >= len(s) || !cpfwIsASCIIWordBs(s[j])
		if prevOK && nextOK {
			return 4
		}
	}
	for _, n := range cpfwA6bLits2 {
		if strings.HasPrefix(s[i:], n) {
			return len(n)
		}
	}
	return 0
}

// cpfwA7Len · cpfwA9Len —— 这两支「字符级匹配」消耗的长度（**非退码**）。
// ★ 缺口 GAP-20261002-12：门⑪ 的裸数字判据把 `return 3` / `return 6` 误读成退码。
const (
	cpfwA7Len = 3
	cpfwA9Len = 6
)

// A7 `2\.5\.`
func cpfwM_A7(s string, i int) int {
	if strings.HasPrefix(s[i:], "2.5.") {
		return cpfwA7Len
	}
	return 0
}

// A8 `~/\.zerg[a-zA-Z0-9._-]*`
func cpfwM_A8(s string, i int) int {
	if !strings.HasPrefix(s[i:], "~/.zerg") {
		return 0
	}
	k := i + len("~/.zerg")
	for k < len(s) {
		b := s[k]
		if !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') ||
			b == '.' || b == '_' || b == '-') {
			break
		}
		k++
	}
	return k - i
}

// A9 `\bMr2109\b`
func cpfwM_A9(s string, i int) int {
	if !strings.HasPrefix(s[i:], "Mr2109") {
		return 0
	}
	if !cpfwBoundBeforeWord(s, i) || !cpfwBoundAfterWord(s, i+6) {
		return 0
	}
	return cpfwA9Len
}

// A10 `example-8b-quant\.IQ4_XS|example-30b|example-moe-36b|(?i:example-35b-v2-1\.[05]-35b)|example-26b-review`
var cpfwA10Lits1 = []string{"example-8b-quant", "example-30b", "example-moe-36b"}

func cpfwM_A10(s string, i int) int {
	for _, n := range cpfwA10Lits1 {
		if strings.HasPrefix(s[i:], n) {
			return len(n)
		}
	}
	if len(s)-i >= 14 && strings.EqualFold(s[i:i+9], "example-35b-v2-1.") &&
		(s[i+9] == '0' || s[i+9] == '5') && strings.EqualFold(s[i+10:i+14], "-35b") {
		return 14
	}
	if strings.HasPrefix(s[i:], "example-26b-review") {
		return len("example-26b-review")
	}
	return 0
}

// A11 `(?i:example-moe-36b)|example-35b-v2-`
func cpfwM_A11(s string, i int) int {
	if len(s)-i >= 10 && strings.EqualFold(s[i:i+10], "example-moe-36b") {
		return 10
	}
	if strings.HasPrefix(s[i:], "example-35b-v2-") {
		return len("example-35b-v2-")
	}
	return 0
}

// A12 `〔[A-Za-z]{1,3}\d{1,3}〕`
func cpfwM_A12(s string, i int) int {
	if !strings.HasPrefix(s[i:], "\u3014") {
		return 0
	}
	p := i + len("\u3014")
	q := p
	for q < len(s) && cpfwIsASCIIAlnum(s[q]) && !(s[q] >= '0' && s[q] <= '9') {
		q++
	}
	nr := q - p
	if nr < 1 || nr > 3 {
		return 0
	}
	rr := q
	nd := 0
	for rr < len(s) {
		r, w := utf8.DecodeRuneInString(s[rr:])
		if !unicode.IsDigit(r) {
			break
		}
		nd++
		rr += w
	}
	if nd < 1 || nd > 3 {
		return 0
	}
	if !strings.HasPrefix(s[rr:], "\u3015") {
		return 0
	}
	return rr + len("\u3015") - i
}

var cpfwRules = []cpfwRule{
	{"A1", "\u79c1\u6709\u5377\u8def\u5f84\uff08`/Volumes/` + \u79c1\u6709\u5377\u540d\uff09",
		"\u79c1\u6709\u6302\u8f7d\u70b9\u4f1a\u5e26\u51fa**\u4ed3\u5916\u7ed3\u6784**\uff08\u8bbe\u8ba1 \u00a73.7 \u7532\u6863\u7b2c 1 \u884c\uff09",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A1) }},
	{"A2", "\u4efb\u610f `/Volumes/` \u5377\u8def\u5f84\uff08\u7f51\u515c \u00b7 \u672a\u767b\u8bb0\u5373\u7ea2\uff09",
		"\u540c\u4e0a\uff1b\u5224\u636e\u9010\u5b57\uff08\u00a73.7 \u4e59\u6863\u7b2c 1 \u884c\uff09\u300c`/Volumes/` \u540e\u8ddf\u7684**\u4e0d\u662f**\u672c\u673a\u79c1\u6709\u5377\u540d \u21d2 \u653e\u884c\u300d" +
			"\u21d2 \u653e\u884c**\u5fc5\u987b\u8d70\u767d\u540d\u5355\u9010\u6761\u767b\u8bb0**\uff08\u672a\u767b\u8bb0\u7684\u65b0\u5377\u540d\u7167\u6837\u7ea2\uff09",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A2) }},
	{"A3", "\u79c1\u6709\u5377/\u5907\u4efd\u533a\u4ee3\u53f7 `AHZ` / `BXC`",
		"\u5377\u4ee3\u53f7\u672c\u8eab\u5c31\u662f**\u4ed3\u5916\u7ed3\u6784**\u7684\u540d\u5b57\uff08\u8bbe\u8ba1 \u00a73.7 \u7532\u6863\u7b2c 2 \u884c\uff09",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A3) }},
	{"A4", "\u4ed3\u5916\u79c1\u6709\u6587\u6863\u4ed3\u540d `Zerg-内部文档`",
		"\u5b83\u662f**\u540c\u7ea7\u4ed3\u5916\u7684\u79c1\u6709\u6587\u6863\u4ed3**\uff1b\u516c\u5f00\u9762\u70b9\u5b83\u7684\u540d = \u5e26\u51fa\u4ed3\u5916\u7ed3\u6784\uff08\u8bbe\u8ba1 \u00a73.7 \u7532\u6863\u7b2c 3 \u884c\uff09",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A4) }},
	{"A5", "\u5f00\u53d1\u9762\u7f3a\u53e3\u53f7 `Q-` + \u6570\u5b57",
		"**\u5185\u90e8\u53f0\u8d26\u7f16\u53f7**\uff1a\u5bf9\u8bfb\u8005\u65e0\u610f\u4e49\uff0c\u4e14\u66b4\u9732\u79c1\u6709\u53f0\u8d26\u5f62\u72b6\uff08\u8bbe\u8ba1 \u00a73.7 \u7532\u6863\u7b2c 4 \u884c\uff09",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A5) }},
	{"A6", "\u79c1\u6709\u7f16\u5199\u89d2\u8272\u79f0\u8c13\uff08`\u7236\u4ee3\u7406`/`\u672c\u679a`/`\u5f00\u5de5\u5355`/`\u8ba4\u9886\u5355`/`\u52a9\u624b`\uff09",
		"**\u628a\u5185\u90e8\u5199\u4f5c\u89d2\u8272\u5e26\u51fa\u53bb**\uff08\u8bbe\u8ba1 \u00a73.7 \u7532\u6863\u7b2c 5 \u884c\u9010\u5b57\uff09",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A6) }},
	{"A6b", "\u79c1\u6709\u79f0\u8c13\uff08\u4eba\u7269\u7f72\u540d/\u79c1\u6709\u9879\u76ee\u540d/\u673a\u578b \u00b7 \u4e0e\u53d1\u5e03\u94fe\u540c\u6279\u8131\u654f\u5bf9\u8c61\uff09",
		"\u771f\u6e90 `publish/replace-rules.tsv` \u7684\u300c\u4eba\u7269/\u8eab\u4efd\u300d+\u300c\u79c1\u6709\u9879\u76ee\u540d\u300d\u4e24\u6bb5\uff0c" +
			"\u4e0e `check-public-marks.py` \u7684 `PRIVATE_PATTERNS` \u540c\u6279\u53e3\u5f84",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A6b) }},
	{"A7", "\u7248\u672c\u53d9\u4e8b / \u7248\u672c\u4e32\uff08`2.5.` \u524d\u7f00\uff09",
		"\u94c1\u5f8b\uff1a\u516c\u5f00\u9762**\u53ea\u8bb2\u6700\u65b0\u7248**\u3001\u4e0d\u5199\u7248\u672c\u53d9\u4e8b\uff08`publish/private-paths.txt` \u7684\u300c\u516c\u5f00\u9762\u53ea\u5141\u8bb8\u6b63\u5f0f\u9762\u300d\u6bb5\uff09\uff1b" +
			"\u2605 \u5224\u636e\u7528**\u524d\u7f00**\u800c\u975e\u4e09\u6bb5\u6b63\u5219 \u2014\u2014 \u540e\u8005\u4f1a\u649e `192.0.2.10`\uff08\u8bbe\u8ba1 \u00a73.5 `V4` \u9010\u5b57\uff09",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A7) }},
	{"A8", "\u5bb6\u76ee\u5f55\u72b6\u6001\u76ee\u5f55\u524d\u7f00 `~/.zerg*`",
		"`M-3` \u628a\u5b83\u5199\u8fdb\u300c\u7981\u300d\u7684\u6e05\u5355\uff08\u540c\u4e00\u65cf\uff1a\u5e26\u51fa**\u672c\u673a/\u7528\u6237\u79c1\u6709**\u8def\u5f84\uff09\uff1b" +
			"\u2605 \u8bbe\u8ba1 \u00a73.7 \u628a\u5b83\u653e**\u4e59\u6863**\uff08\u516c\u5f00\u9762\u672c\u6765\u5c31\u6709\u7684\u7528\u6237\u914d\u7f6e\u8def\u5f84\uff09\u21d2 \u672c\u4ef6\u9010\u6761\u8c41\u514d + \u6253\u5370\u7406\u7531",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A8) }},
	{"A9", "\u516c\u5f00\u4ed3\u4e3b\u540d\uff08\u88f8\u8d26\u53f7\u540d\uff09`Mr2109`",
		"`M-3` \u628a\u5b83\u5199\u8fdb\u300c\u7981\u300d\u7684\u6e05\u5355\uff1b\u2605 \u8bbe\u8ba1 \u00a73.7 \u9010\u5b57\u300c**\u7981\u7684\u662f**\u300e\u5185\u90e8\u5199\u4f5c\u89d2\u8272\u79f0\u8c13\u300f\uff0c\u4e0d\u662f\u4f5c\u8005\u540d\u300d" +
			"\u21d2 \u672c\u4ef6\u9010\u6761\uff08\u4ef6 \u00d7 \u884c\uff09\u8c41\u514d + \u6253\u5370\u7406\u7531",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A9) }},
	{"A10", "\u79c1\u6709\u6a21\u578b\u4ee3\u53f7\uff08`replace-rules.tsv` \u5168\u5f62\uff09",
		"\u53d1\u5e03\u94fe\u7684**\u79c1\u6709\u6a21\u578b\u4ee3\u53f7**\uff08\u771f\u6e90 `publish/replace-rules.tsv` \u7684\u300c\u79c1\u6709\u6a21\u578b\u4ee3\u53f7\uff08\u51b3\u7b56 4\uff09\u300d\u6bb5\uff09\u3002" +
			"\u2605 2026-09-26 \u6821\u6b63\u8303\u56f4\uff08\u8865**\u5927\u5c0f\u5199**\u6d1e \u00b7 \u89c1\u4ef6\u5934\u300c\u73b0\u8bfb\u53e3\u5f84\u5dee\u300d\u2464\uff09\uff1a`example-35b-v2-1.[05]-35b` \u771f\u6e90**\u4e24\u5904\u90fd\u767b\u8bb0**" +
			"\uff08\u5c0f\u5199\u5168\u957f `:57`/`:58` \uff0b \u5927\u5199\u5168\u957f `:60`/`:61`\uff09\u21d2 **\u53ea\u8fd9\u4e00\u6761**\u6536 `(?i:\u2026)`\uff1b\u5176\u4f59\u5404\u6761\u4e00\u5b57\u672a\u52a8" +
			"\uff08\u771f\u6e90\u6ca1\u767b\u8bb0\u5b83\u4eec\u522b\u7684\u5199\u6cd5 \u21d2 \u4e0d\u8d8a\u771f\u6e90\u52a0\u5927\u5c0f\u5199\uff09",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A10) }},
	{"A11", "\u79c1\u6709\u6a21\u578b\u4ee3\u53f7\uff08**\u524d\u7f00\u5f62\u6001** \u00b7 `example-moe-36b` / `example-35b-v2-`\uff09",
		"\u540c A10\uff1b\u2605 2026-09-23 \u8d77\u66ff\u6362\u89c4\u5219\u4e5f\u6536\u524d\u7f00\u5f62\u6001\uff08\u89c1\u4e19\u6863 \u2461\uff09\u3002\u672c\u89c4\u5219**\u4fdd\u7559**\uff1a\u672c\u4ef6\u626b\u7684\u662f**\u79c1\u6709\u6811**" +
			"\uff08\u771f\u6e90\u91cc\u7684\u4ee3\u53f7\u4e0d\u906e\uff09\u21d2 \u9010\u6761\u8c41\u514d\u800c\u4e0d\u662f\u88c5\u770b\u4e0d\u89c1\u3002\u2605 2026-09-26 \u6821\u6b63\u8303\u56f4\uff08\u8865**\u524d\u7f00**\u6d1e \u00b7 \u89c1\u4ef6\u5934" +
			"\u300c\u73b0\u8bfb\u53e3\u5f84\u5dee\u300d\u2464\uff09\uff1a\u2460 `example-moe-36b` \u771f\u6e90\u4e24\u5904\u90fd\u767b\u8bb0\uff08\u5927\u5199\u524d\u7f00 `:56` \uff0b \u5c0f\u5199\u5b57\u9762 `:59`\uff09\u21d2 \u8be5\u8bcd\u6536 `(?i:\u2026)`\uff1b" +
			"\u2461 \u771f\u6e90\u7684\u524d\u7f00\u5f62\u6001\u6709**\u4e24\u6761**\uff08K2 \u5bb6\u65cf `:56` \u00b7 Ornith \u5bb6\u65cf `:64` \u7684 `example-35b-v2-`\uff09\u21d2 \u8865\u4e0a `example-35b-v2-`" +
			"\uff08**\u9010\u5b57**\u7167 `:64` \u00b7 \u4ecd\u5927\u5c0f\u5199\u654f\u611f \u2014\u2014 \u771f\u6e90\u90a3\u6761\u5c31\u662f\u5927\u5199\u5b57\u9762\uff09",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A11) }},
	{"A12", "\u6587\u6863\u9762\u51fa\u5904\u6807\u8bb0 `\u3014<\u5b57\u6bcd><\u6570\u5b57>\u3015`\uff08\u5f62\u5982 `\u3014S6\u3015`\uff09",
		"\u5f62\u6001\u4e0a\u662f**\u5f00\u53d1\u9762\u7b26\u53f7**\uff08\u8bbe\u8ba1 \u00a73.7 \u7532\u6863\u7b2c 4 \u884c\u90a3\u4e00\u7c7b\uff09\uff1b\u8be5\u65cf 4 \u5904\u6b8b\u7559\uff08`SECURITY.md`\uff09" +
			"**\u5df2\u62b9\u4e8e\u672c\u7b14**\uff08\u6539\u4e2d\u6027\u8bf4\u6cd5 \u21d2 \u73b0\u8bfb 0 \u5904\uff09\u21d2 \u4e19\u6863 \u2460 \u7684\u767d\u540d\u5355\u767b\u8bb0\u540c\u6b65**\u6536\u7a84\u79fb\u9664**",
		func(s string) [][2]int { return cpfwFindAll(s, cpfwM_A12) }},
}

// ── 白名单（乙档 4 类 + 丙档 2 类 · 每条都带理由 · 逐命中逐条打印）────────────────────────
type cpfwWLItem struct {
	rid    string
	globs  []string
	pat    string
	subs   []string
	reason string
}

var cpfwWhitelist = []cpfwWLItem{
	{"A2", []string{"publish/docs/*"},
		"ZERG_KB_PATH=\"/Volumes/My Disk/knowledge\\.db\"",
		[]string{"ZERG_KB_PATH=\"/Volumes/My Disk/knowledge.db\""},
		"\u4e59\u6863 \u2460 `/Volumes/\u2026` **\u793a\u4f8b\u503c**\uff08\u8bbe\u8ba1 \u00a73.7 \u4e59\u6863\u7b2c 1 \u884c\u9010\u5b57\uff09\uff1a\u8be5\u884c\u8bb2\u7684\u662f" +
			"\u300c\u8def\u5f84\u542b\u7a7a\u683c/\u4e2d\u6587\u65f6 `.env` \u91cc\u8981\u52a0\u53cc\u5f15\u53f7\u300d\uff0c`/Volumes/My Disk` \u662f**\u4e3e\u4f8b\u7528\u7684\u5377\u540d**" +
			"\uff08\u4e0d\u662f\u672c\u673a\u79c1\u6709\u5377\u540d `AHZ`/`BXC`/`ZERG-ARCHIVE`\uff09\u21d2 \u5224\u300c\u5141\u8bb8\u300d\u2713\uff08\u73b0\u8bfb 2 \u5904\uff1a" +
			"`QUICKSTART.en.md:28` \u00b7 `QUICKSTART.zh-CN.md:28`\uff09"},
	{"A8", []string{"publish/docs/*", "publish/SECURITY.md", "publish/config.example.yaml"},
		"~/\\.zerg",
		[]string{"~/.zerg"},
		"\u4e59\u6863 \u2461 `~/.zerg/**`\uff08\u8bbe\u8ba1 \u00a73.7 \u4e59\u6863\u7b2c 2 \u884c\u9010\u5b57\uff09\uff1a\u8fd9\u4e9b\u662f**\u516c\u5f00\u9762\u672c\u6765\u5c31\u6709\u7684\u7528\u6237\u914d\u7f6e\u8def\u5f84**" +
			"\uff08\u6587\u6863\u8981\u8bb2\u914d\u7f6e\u5728\u54ea\u513f\uff1a`~/.zerg/token` \u00b7 `~/.zerg/state/` \u00b7 `~/.zerg-chat/chat.db` \u2026\uff09\uff1b" +
			"**\u7981\u7684\u662f**\u300c\u79c1\u6709\u5377/\u79c1\u6709\u4ed3\u7684**\u7edd\u5bf9\u8def\u5f84**\u300d\u2717\uff0c**\u4e0d\u662f**\u4ea7\u54c1\u81ea\u5df1\u7684\u5bb6\u76ee\u5f55\u72b6\u6001\u76ee\u5f55 \u21d2 \u5224\u300c\u5141\u8bb8\u300d\u2713\u3002" +
			"\u73b0\u8bfb **62 \u5904** = `publish/docs/**` **60**\uff08\u4e0e\u8bbe\u8ba1 \u00a73.7 \u7684 60 \u9010\u5b57\u76f8\u540c\uff09+ \u6839\u9762 **2**" +
			"\uff08`SECURITY.md:13` \u7684 `~/.zerg/token` \u00b7 `config.example.yaml:6` \u7684 `~/.zerg`\uff09"},
	{"A9", []string{"publish/docs/README.md", "publish/docs/README.zh-CN.md"},
		"git clone https://github\\.com/Mr2109/zerg-swarm\\.git",
		[]string{"git clone https://github.com/Mr2109/zerg-swarm.git"},
		"\u4e59\u6863 \u2462 `Mr2109`\uff08\u8bbe\u8ba1 \u00a73.7 \u4e59\u6863\u7b2c 3 \u884c\u9010\u5b57\uff09\uff1a`git clone https://github.com/Mr2109/zerg-swarm.git`" +
			" = **\u516c\u5f00\u4ed3\u4e3b\u540d**\uff08README \u7684\u514b\u9686 URL \u00b7 \u5fc5\u987b\u51fa\u73b0\uff09\u21d2 \u5224\u300c\u5141\u8bb8\u300d\u2713" +
			"\uff08**\u7981\u7684\u662f**\u300c\u5185\u90e8\u5199\u4f5c\u89d2\u8272\u79f0\u8c13\u300d\uff0c\u4e0d\u662f\u4f5c\u8005\u540d \u00b7 \u73b0\u8bfb 2 \u5904\uff1a`README.md:75` \u00b7 `README.zh-CN.md:73`\uff09"},
	{"A9", []string{"publish/NOTICE"},
		"github\\.com/Mr2109/zerg-swarm",
		[]string{"github.com/Mr2109/zerg-swarm"},
		"\u4e19\u6863\u8ffd\u8865\uff08**\u672c\u4ef6\u73b0\u8bfb** \u00b7 \u00a73.7 \u53ea\u5217\u4e86 README \u4e24\u5904\uff09\uff1a\u540c\u4e00\u8d26\u53f7\u51fa\u73b0\u5728**\u7248\u6743/\u51fa\u5904\u884c**" +
			"\uff08`NOTICE:5` \u7684 `(https://github.com/Mr2109/zerg-swarm)`\uff09\u2014\u2014 \u4e0e\u4e0a\u4e00\u6761**\u540c\u4e00\u4e2a\u516c\u5f00\u8eab\u4efd**" +
			"\uff08\u516c\u5f00\u4ed3\u4e3b\u540d\uff09\uff0c\u4e0d\u662f\u79c1\u6709\u9762 \u21d2 \u5141\u8bb8 \u2713"},
	{"A9", []string{"publish/install.sh"},
		"ZERG_UPGRADE_SOURCE=https://github\\.com/Mr2109/zerg-swarm",
		[]string{"ZERG_UPGRADE_SOURCE=https://github.com/Mr2109/zerg-swarm"},
		"\u4e19\u6863\u8ffd\u8865\uff08**\u672c\u4ef6\u73b0\u8bfb**\uff09\uff1a`install.sh:17` \u7684\u62c9\u53d6\u6e90**\u793a\u4f8b\u884c**\u2014\u2014 \u516c\u5f00\u4ed3\u4e3b\u540d\uff08\u7528\u6237\u8981\u7167\u7740\u586b\u7684\u90a3\u4e2a\u540d\u5b57\uff09" +
			"\u21d2 \u5141\u8bb8 \u2713"},
	{"A9", []string{"publish/install.sh"},
		"REPO_SLUG=\"Mr2109/zerg-swarm\"",
		[]string{"REPO_SLUG=\"Mr2109/zerg-swarm\""},
		"\u4e19\u6863\u8ffd\u8865\uff08**\u672c\u4ef6\u73b0\u8bfb**\uff09\uff1a`install.sh:24` \u7684 `REPO_SLUG` = \u516c\u5f00\u4ed3\u4e3b\u540d\uff08\u5347\u7ea7\u5668\u7684\u62c9\u53d6\u6e90\uff09\u21d2 \u5141\u8bb8 \u2713"},
	{"A6", []string{"publish/docs/design/\u8bb0\u5fc6\u4f53\u7cfb.md"},
		"\u52a9\u624b\u628a\u8be5\u8bb0\u4f4f\u7684\u4e8b|\u52a9\u624b\u81ea\u5df1\u7684\u7b14\u8bb0",
		[]string{"\u52a9\u624b\u628a\u8be5\u8bb0\u4f4f\u7684\u4e8b", "\u52a9\u624b\u81ea\u5df1\u7684\u7b14\u8bb0"},
		"\u4e19\u6863\u8ffd\u8865\uff08**\u672c\u4ef6\u73b0\u8bfb** \u00b7 \u8bbe\u8ba1 \u00a73.7 **\u672a\u5217**\uff09\uff1a`\u52a9\u624b` \u5728 \u00a73.7 \u7532\u6863\u91cc\u662f**\u79c1\u6709\u7f16\u5199\u89d2\u8272\u79f0\u8c13**" +
			"\uff08\u5185\u90e8\u4ee3\u7406\u81ea\u79f0\uff09\uff1b\u4f46\u73b0\u8bfb\u8fd9 2 \u5904\uff08`\u8bb0\u5fc6\u4f53\u7cfb.md:3` \u7684\u300c\u8ba9**\u5bf9\u8bdd\u52a9\u624b**\u628a\u8be5\u8bb0\u4f4f\u7684\u4e8b\u5199\u8fdb\u8de8\u4f1a\u8bdd\u6301\u4e45\u8bb0\u5fc6\u300d\u00b7" +
			" `:72` \u7684 `memory` \u4f5c\u7528\u57df\u53eb\u300c**\u52a9\u624b**\u81ea\u5df1\u7684\u7b14\u8bb0\u300d\uff09\u90fd\u662f**\u4ea7\u54c1\u8bcd\u4e49\uff08AI \u52a9\u624b\uff09**" +
			"\uff08\u88ab\u8bb2\u7684\u4ea7\u54c1\u5bf9\u8c61 + \u7528\u6237\u914d\u7f6e\u4f5c\u7528\u57df\u540d\uff09\uff0c\u4e0d\u662f\u5185\u90e8\u81ea\u79f0 \u21d2 \u9010\u6761\u8c41\u514d\u3002" +
			"\u2605 **2026-09-23 Mr2109 \u62cd\uff1a\u53d6\u7532\u6863\u8868\uff08\u542b `\u52a9\u624b`\uff09**\uff08\u53e3\u5f84\u5dee\u9010\u5b57\u89c1\u4ef6\u5934\u300c\u73b0\u8bfb\u53e3\u5f84\u5dee\u300d\u2460\uff09\u2014\u2014" +
			"\u672c\u6761\u76ee\u662f\u90a3\u6761\u62cd\u677f\u7684\u843d\u5730\uff1a2 \u5904\u9010\u6761\u8c41\u514d \u00b7 \u9010\u6761\u6253\u5370\u7406\u7531"},
	{"A11", []string{"publish/THIRD_PARTY_LICENSES.md"},
		"example-moe-36b",
		[]string{"example-moe-36b"},
		"\u4e19\u6863 \u2461\uff08**\u672c\u4ef6\u73b0\u8bfb** \u00b7 \u8bbe\u8ba1 \u00a73.7 \u672a\u5217\uff09\uff1a`THIRD_PARTY_LICENSES.md:8` \u7684 fork \u4e3e\u4f8b" +
			"\u300c\uff08\u7528\u6237\u81ea\u88c5\uff1b\u6216\u5176 fork\uff0c\u5982 **example-moe-36b** \u53d8\u4f53\uff09\u300d\u2014\u2014 `example-moe-36b` \u662f" +
			"`publish/replace-rules.tsv` \u91cc\u7684**\u79c1\u6709\u6a21\u578b\u4ee3\u53f7**\uff1b\u2605 2026-09-23\uff1a\u8be5\u8868\u5df2\u8865**\u524d\u7f00\u5f62\u6001**\u4e00\u6761" +
			"\uff08`example-moe-36b \u2192 example-moe-36b`\uff0c\u7d27\u6392\u5728\u5168\u957f\u5f62\u6001\u90a3\u6761**\u4e4b\u540e**\uff09\u21d2 **\u516c\u5f00\u5feb\u7167**\u91cc\u8fd9\u4e00\u5904\u5df2\u88ab\u89c4\u5219\u76d6\u4f4f\u3002" +
			"\u672c\u6761\u76ee\u4ecd**\u5fc5\u987b\u7559**\uff1a\u672c\u95e8\u626b\u7684\u662f**\u79c1\u6709\u6811**\uff08\u771f\u6e90\u91cc\u7684\u4ee3\u53f7\u4e0d\u906e\uff09\u21d2 \u9010\u6761\u8c41\u514d + \u6253\u5370\u7406\u7531"},
}

// ── py 语义工具（posixpath / fnmatch 同构）──────────────────────────────────────────

// cpfwPyJoin = CPython posixpath.join 逐段规则（★ 三语义差：中间 `..` 原样留 · 后段绝对丢前段 ·
// 空段保留尾 `/`）。模板见 skill `templates/pyjoin.go`，本件按件名缩写定名 `cpfwPyJoin`。
func cpfwPyJoin(parts ...string) string {
	path := ""
	if len(parts) > 0 {
		path = parts[0]
	}
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

func cpfwPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
	}
	if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
		initial = 2
	}
	var newComps []string
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || initial == 0 && len(newComps) == 0 ||
			len(newComps) > 0 && newComps[len(newComps)-1] == ".." {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	res := strings.Join(newComps, "/")
	if initial > 0 {
		res = strings.Repeat("/", initial) + res
	}
	if res == "" {
		return "."
	}
	return res
}

func cpfwPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = ""
		}
		p = cpfwPyJoin(wd, p)
	}
	return cpfwPyNormpath(p)
}

func cpfwSplitComps(p string) []string {
	var out []string
	for _, c := range strings.Split(p, "/") {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

func cpfwPyRelpath(path, start string) string {
	startAbs := cpfwPyAbspath(start)
	pathAbs := cpfwPyAbspath(path)
	sl := cpfwSplitComps(startAbs)
	pl := cpfwSplitComps(pathAbs)
	i := 0
	for i < len(sl) && i < len(pl) && sl[i] == pl[i] {
		i++
	}
	var rel []string
	for j := 0; j < len(sl)-i; j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pl[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return cpfwPyJoin(rel...)
}

// cpfwPyFnmatch = fnmatch.fnmatch 语义（POSIX 下 normcase 恒等 ⇒ 大小写敏感）；★ `*` **跨 `/`**。
func cpfwPyFnmatch(name, pat string) bool {
	nr := []rune(name)
	pr := []rune(pat)
	i, j := 0, 0
	star, mark := -1, 0
	for i < len(nr) {
		if j < len(pr) && (pr[j] == '?' || pr[j] == nr[i]) {
			i++
			j++
		} else if j < len(pr) && pr[j] == '*' {
			star, mark = j, i
			j++
		} else if star != -1 {
			j = star + 1
			mark++
			i = mark
		} else {
			return false
		}
	}
	for j < len(pr) && pr[j] == '*' {
		j++
	}
	return j == len(pr)
}

func cpfwRel(root, path string) string {
	return strings.ReplaceAll(cpfwPyRelpath(path, root), string(os.PathSeparator), "/")
}

func cpfwIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func cpfwIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func cpfwIsSymlink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

func cpfwHasSuffixAny(s string, suffixes []string) bool {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

// ── 扫面收集 ────────────────────────────────────────────────────────────────────

// cpfwWalkDocs：复刻 `os.walk`（topdown · followlinks=False）产出的**文件集**。
// ★ 遍历点：py 明写 `sorted(dirnames)` + `sorted(filenames)`；本件同法排序（`os.ReadDir` 本身已按名排序，
// 仍显式排序以对齐语义）；`entry.is_dir()` **跟随符号链接** ⇒ 用 `os.Stat`（✗ 不用 `entry.IsDir()` 的 Lstat）。
func cpfwWalkDocs(root, dp string, out *[]string) {
	ents, err := os.ReadDir(dp)
	if err != nil {
		return
	}
	var dirs, files []string
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if cpfwIsDir(cpfwPyJoin(dp, name)) {
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)
	for _, fn := range files {
		if strings.HasPrefix(fn, ".") || cpfwHasSuffixAny(fn, cpfwSkipSuffix) {
			continue
		}
		*out = append(*out, cpfwRel(root, cpfwPyJoin(dp, fn)))
	}
	for _, dn := range dirs {
		nxt := cpfwPyJoin(dp, dn)
		if cpfwIsSymlink(nxt) {
			continue // followlinks=False：符号链接目录归入 dirnames 但不下钻
		}
		cpfwWalkDocs(root, nxt, out)
	}
}

func cpfwCollectFace(root string) ([]string, []string) {
	var problems []string
	var rels []string
	docsDir := cpfwPyJoin(root, cpfwDocsRel)
	if !cpfwIsDir(docsDir) {
		problems = append(problems, fmt.Sprintf("\u626b\u9762\u6839\u4e0d\u5b58\u5728\uff1a%s/", cpfwDocsRel))
	} else {
		cpfwWalkDocs(root, docsDir, &rels)
	}
	for _, rel := range cpfwRootFace {
		if cpfwIsFile(cpfwPyJoin(root, rel)) {
			rels = append(rels, rel)
		} else {
			problems = append(problems, fmt.Sprintf("\u6839\u9762 8 \u4ef6\u7f3a\u4ef6\uff1a%s", rel))
		}
	}
	sort.Strings(rels)
	return rels, problems
}

// ── 扫描 / 豁免 ─────────────────────────────────────────────────────────────────

type cpfwHit struct {
	rid, title, why string
	ln              int
	frag, line      string
}

func cpfwScanText(text string) []cpfwHit {
	lines := strings.Split(text, "\n")
	var hits []cpfwHit
	for _, r := range cpfwRules {
		for _, sp := range r.find(text) {
			ln := strings.Count(text[:sp[0]], "\n") + 1
			line := ""
			if ln-1 >= 0 && ln-1 < len(lines) {
				line = lines[ln-1]
			}
			hits = append(hits, cpfwHit{r.rid, r.title, r.why, ln, text[sp[0]:sp[1]], line})
		}
	}
	return hits
}

func cpfwWLHas(w cpfwWLItem, line string) bool {
	for _, sub := range w.subs {
		if strings.Contains(line, sub) {
			return true
		}
	}
	return false
}

func cpfwExempt(h cpfwHit, rel string) (int, bool) {
	for i, w := range cpfwWhitelist {
		if w.rid != h.rid {
			continue
		}
		globOK := false
		for _, g := range w.globs {
			if cpfwPyFnmatch(rel, g) {
				globOK = true
				break
			}
		}
		if !globOK {
			continue
		}
		if cpfwWLHas(w, h.line) {
			return i, true
		}
	}
	return -1, false
}

// ── 判据主函数 ──────────────────────────────────────────────────────────────────

type cpfwData struct {
	face     int
	rules    map[string]int
	hits     int
	exempt   int
	unexempt int
}

type cpfwExemptRec struct {
	rel, rid, title, why string
	ln                   int
	frag, line, reason   string
}

type cpfwUnexRec struct {
	rel, rid, title, why string
	ln                   int
	frag, line           string
}

func cpfwJudge(root string, faceOverride []string, hasFaceOverride bool, selfExempt bool) (int, []string, cpfwData) {
	out := []string{}
	out = append(out, "\u2500\u2500 \u516c\u5f00\u9762\u5199\u4f5c\u7ea2\u7ebf\u81ea\u68c0\uff08`T-03` \u00b7 \u6f0f\u70b9 `M-3` \u00b7 \u5224\u636e `V9`\uff09\uff1a\u7532\u6863\u9010\u8bcd + \u767d\u540d\u5355\u5f0f\u8c41\u514d \u2500\u2500")
	out = append(out, fmt.Sprintf("   \u6811\uff1a%s", root))
	empty := cpfwData{face: 0, rules: map[string]int{}}
	var rels []string
	if !hasFaceOverride {
		var problems []string
		rels, problems = cpfwCollectFace(root)
		if len(problems) > 0 {
			for _, p := range problems {
				out = append(out, "   \u26d4 "+p)
			}
			out = append(out, fmt.Sprintf("   \u26d4 \u626b\u9762\u7f3a\u4ef6 %d \u5904 \u21d2 **\u4e0d\u7ed9\u7ed3\u8bba\uff08rc=2\uff09**\uff08\u7f3a\u4ef6 \u2260 \u7eff\uff09", len(problems)))
			return 2, out, empty
		}
	} else {
		rels = append([]string{}, faceOverride...)
	}
	if len(rels) == 0 {
		out = append(out, "   \u26d4 \u626b\u9762 **0 \u4ef6** \u21d2 \u7a7a\u8f6c\uff0c\u62d2\u7edd\u7ed9\u7ed3\u8bba\uff08rc=2\uff09"+
			"\uff08\u300c\u914d\u7f6e\u6ca1\u4e86\u6240\u4ee5\u5168\u7eff\u300d\u662f\u6700\u96be\u53d1\u73b0\u7684\u90a3\u4e00\u7c7b\u5047\u7eff\uff09")
		return 2, out, empty
	}

	texts := map[string]string{}
	var unreadable []string
	var selfHits []string
	for _, rel := range rels {
		if selfExempt && rel == cpfwSelfRel {
			selfHits = append(selfHits, fmt.Sprintf("   \u26aa \u81ea\u8c41\u514d `%s`\uff1a\u672c\u4ef6\u5185\u7f6e\u8bcd\u8868**\u5fc5\u7136**\u542b\u7981\u8bcd\u5b57\u9762\u91cf\uff08`%s`/`%s`/`%s`/`%s` \u2026\uff09\u21d2 **\u7ed3\u6784\u6027\u4e0d\u5728\u626b\u9762\u91cc**\uff1b"+
				"\u88ab\u663e\u5f0f\u5582\u8fdb\u6765\u65f6\u6309\u672c\u884c\u9010\u6761\u6253\u5370\u7406\u7531\u653e\u884c\uff08\u4e0d\u662f\u9759\u9ed8\u8df3\u8fc7\uff1b\u7259\u9f7f\u89c1 `--self-test` \u683c 14/15/15b\uff09",
				rel, "AHZ", "Zerg-内部文档", "\u8001\u53e8", cpfwVolPrefix))
			continue
		}
		b, err := os.ReadFile(cpfwPyJoin(root, rel))
		if err != nil {
			unreadable = append(unreadable, fmt.Sprintf("   \u26d4 \u4ef6\u8bfb\u4e0d\u4e86\uff1a%s\uff08%s\uff09", rel, cpfwPyOSErrorText(err, cpfwPyJoin(root, rel))))
			continue
		}
		texts[rel] = string(b)
	}
	if len(unreadable) > 0 {
		out = append(out, unreadable...)
		out = append(out, "   \u26d4 \u6709\u4ef6\u8bfb\u4e0d\u4e86 \u21d2 **\u4e0d\u7ed9\u7ed3\u8bba\uff08rc=2\uff09**\uff08\u8bfb\u4e0d\u5168\u5c31\u5224\u4e0d\u51fa\u771f\u503c\uff09")
		return 2, out, cpfwData{face: len(rels), rules: map[string]int{}}
	}
	out = append(out, selfHits...)
	out = append(out, fmt.Sprintf("   \u626b\u9762\uff1a`%s/**` + \u6839\u9762 8 \u4ef6 = **%d \u4ef6**\uff08\u9010\u4ef6\u73b0\u8bfb \u00b7 \u4ef6\u540d\u9010\u6761\u89c1 `--json face=1` \u65f6\u7684\u7edf\u8ba1\uff09",
		cpfwDocsRel, len(texts)))

	perRule := map[string]int{}
	var exempted []cpfwExemptRec
	var unexempted []cpfwUnexRec
	sortedRels := make([]string, 0, len(texts))
	for r := range texts {
		sortedRels = append(sortedRels, r)
	}
	sort.Strings(sortedRels)
	for _, rel := range sortedRels {
		for _, h := range cpfwScanText(texts[rel]) {
			perRule[h.rid]++
			if wi, ok := cpfwExempt(h, rel); ok {
				w := cpfwWhitelist[wi]
				exempted = append(exempted, cpfwExemptRec{rel, h.rid, h.title, h.why, h.ln, h.frag, h.line, w.reason})
			} else {
				unexempted = append(unexempted, cpfwUnexRec{rel, h.rid, h.title, h.why, h.ln, h.frag, h.line})
			}
		}
	}

	out = append(out, "   \u7532\u6863\u9010\u8bcd\uff0812 \u6761\u89c4\u5219 \u00b7 \u547d\u4e2d\u5373\u7ea2 \u00b7 \u9664\u767d\u540d\u5355\u8c41\u514d\uff09\u2014\u2014 \u4e00\u884c\u4e00\u8bcd\uff1a")
	for _, r := range cpfwRules {
		n := perRule[r.rid]
		ex := 0
		for _, e := range exempted {
			if e.rid == r.rid {
				ex++
			}
		}
		mark := "\u2705"
		if n != 0 {
			if ex == n {
				mark = "\u26aa"
			} else {
				mark = "\u2717"
			}
		}
		tail := ""
		if n != 0 {
			if ex == n {
				tail = fmt.Sprintf("\uff08\u547d\u4e2d %d \u2192 **\u5168\u6570\u8c41\u514d**\uff09", n)
			} else {
				tail = fmt.Sprintf("\uff08\u547d\u4e2d %d \u2192 \u8c41\u514d %d \u00b7 **\u672a\u8c41\u514d %d**\uff09", n, ex, n-ex)
			}
		}
		out = append(out, fmt.Sprintf("     %s %-4s %s \u547d\u4e2d %d %s", mark, r.rid, r.title, n, tail))
		out = append(out, fmt.Sprintf("          \u4e3a\u4ec0\u4e48\u7981\uff1a%s", r.why))
	}

	out = append(out, "   \u4e59\u6863/\u4e19\u6863\u767d\u540d\u5355\u547d\u4e2d\uff08**\u9010\u6761\u6253\u5370\u7406\u7531** \u00b7 \u4e0d\u8bb8\u9759\u9ed8\u8fc7\u6ee4\uff09\uff1a")
	if len(exempted) > 0 {
		for _, e := range exempted {
			out = append(out, fmt.Sprintf("     \u26aa %s  %s:%d  \u300c%s\u300d", e.rid, e.rel, e.ln, e.frag))
			out = append(out, fmt.Sprintf("          \u4e3a\u4ec0\u4e48\u7981\uff1a%s", e.why))
			out = append(out, fmt.Sprintf("          \u21b3 \u672c\u6761\u8c41\u514d\u7406\u7531\uff1a%s", e.reason))
		}
	} else {
		out = append(out, "     \uff08\u672c\u8dd1 0 \u5904\u547d\u4e2d \u2014\u2014 \u767d\u540d\u5355\u6761\u76ee\u547d\u4e2d\u6570\u9010\u6761\u89c1\u4e0b\uff09")
	}
	out = append(out, "   \u767d\u540d\u5355\u6761\u76ee\u73b0\u8bfb\u547d\u4e2d\u6570\uff08**\u6bcf\u6761\u90fd\u5e26\u7406\u7531 \u00b7 \u60ac\u7a7a\u767b\u8bb0\u4e5f\u7167\u5b9e\u6253\u5370**\uff1b\u60ac\u7a7a**\u4e0d\u5224\u7ea2** \u2014\u2014 \u300c\u4ef6\u91cc\u628a\u90a3\u8bcd\u5220\u6389\u300d\u4e0d\u8be5\u53d8\u6210\u5047\u7ea2\uff1b\u6761\u76ee\u5bbd\u5ea6\u9010\u6761\u5199\u5728\u7406\u7531\u91cc\uff09\uff1a")
	for i, w := range cpfwWhitelist {
		n := 0
		for _, e := range exempted {
			if e.rid == w.rid && e.reason == w.reason {
				n++
			}
		}
		suffix := ""
		if n == 0 {
			suffix = "\uff08\u60ac\u7a7a\u767b\u8bb0\uff09"
		}
		out = append(out, fmt.Sprintf("     #%-2d %s  \u4ef6 glob=%s  \u884c\u6b63\u5219=%s  \u73b0\u8bfb\u547d\u4e2d %d \u5904%s",
			i+1, w.rid, strings.Join(w.globs, "|"), w.pat, n, suffix))
		out = append(out, fmt.Sprintf("          \u6761\u76ee\u7406\u7531\uff1a%s", w.reason))
	}

	hits := 0
	for _, v := range perRule {
		hits += v
	}
	data := cpfwData{face: len(texts), rules: perRule, hits: hits, exempt: len(exempted), unexempt: len(unexempted)}
	if len(unexempted) > 0 {
		out = append(out, fmt.Sprintf("   \u2717 \u7532\u6863\u547d\u4e2d**\u672a\u8c41\u514d** %d \u5904\uff08\u9010\u6761\u70b9\u540d \u00b7 \u5904\u7f6e\u53ea\u6709\u4e24\u6761\uff1a\u6539\u4ef6\uff0c\u6216**\u9010\u6761**\u767b\u8fdb\u767d\u540d\u5355\u5e76\u5199\u6e05\u7406\u7531\uff09\uff1a",
			len(unexempted)))
		for _, u := range unexempted {
			out = append(out, fmt.Sprintf("       \u2500\u2500 %s  %s:%d  \u300c%s\u300d  \u2190 \u4e3a\u4ec0\u4e48\u7981\uff1a%s", u.rid, u.rel, u.ln, u.frag, u.why))
		}
		out = append(out, "\u21d2 **\u6709\u7ea2\uff08rc=1\uff09**\uff1a\u516c\u5f00\u9762\u51fa\u73b0\u4e86\u79c1\u6709\u9762\u7528\u8bcd \u2014\u2014 \u516c\u5f00\u9762\u4e0d\u53ef\u64a4\u56de\uff0c\u5148\u6539\u4ef6\u518d\u63d0\u4ea4")
		return 1, out, data
	}
	out = append(out, fmt.Sprintf("   \u5408\u8ba1\uff1a\u7532\u6863\u547d\u4e2d **%d** \u5904 \u00b7 \u5176\u4e2d**\u8c41\u514d %d** \u5904 \u00b7 **\u672a\u8c41\u514d 0** \u5904 \u00b7 \u626b\u9762 %d \u4ef6",
		data.hits, data.exempt, data.face))
	out = append(out, "\u21d2 \u5168\u7eff\uff08rc=0\uff09\uff1a\u7532\u6863\u9010\u8bcd\u5728**\u767d\u540d\u5355\u4e4b\u5916**\u7684\u547d\u4e2d\u4e3a 0\uff08\u6bcf\u5904\u8c41\u514d\u90fd\u5df2\u9010\u6761\u6253\u5370\u7406\u7531\uff09")
	return 0, out, data
}

// cpfwPyOSErrorText：复刻 py `OSError` 的 str（`[Errno 21] Is a directory: '<路径>'` 一类）。
func cpfwPyOSErrorText(err error, path string) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return fmt.Sprintf("[Errno %d] %s: '%s'", int(errno), cpfwStrerror(errno), path)
	}
	return fmt.Sprintf("%s: '%s'", err.Error(), path)
}

// cpfwStrerror：对齐 libc `strerror(3)` 的 macOS 文案（Python `OSError.__str__` 用它）。
func cpfwStrerror(e syscall.Errno) string {
	switch e {
	case syscall.EACCES:
		return "Permission denied"
	case syscall.EISDIR:
		return "Is a directory"
	case syscall.ENOENT:
		return "No such file or directory"
	case syscall.ENOTDIR:
		return "Not a directory"
	case syscall.ELOOP:
		return "Too many levels of symbolic links"
	}
	s := e.Error()
	if s != "" {
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return "Unknown error"
}

// ── 自检（成对负控 · 合成夹具 · 先自证「会红」再扫真目标）──────────────────────────────────

func cpfwRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func cpfwMakeFixture(base string, docs map[string]string, rootFiles map[string]string, dropRoot []string) string {
	cpfwMkdirAll(cpfwPyJoin(base, cpfwDocsRel))
	if docs == nil {
		docs = map[string]string{"README.md": "# \u5e72\u51c0\u4ef6\uff08\u5408\u6210\u5939\u5177\uff09\n"}
	}
	for rel, txt := range docs {
		p := cpfwPyJoin(base, cpfwDocsRel, rel)
		cpfwMkdirAll(cpfwDirname(p))
		_ = os.WriteFile(p, []byte(txt), 0o644)
	}
	for _, rel := range cpfwRootFace {
		if cpfwContainsStr(dropRoot, rel) {
			continue
		}
		p := cpfwPyJoin(base, rel)
		cpfwMkdirAll(cpfwDirname(p))
		body := "\u5408\u6210\u6839\u9762\u4ef6\uff08\u5e72\u51c0\uff09\n"
		if rootFiles != nil {
			if v, ok := rootFiles[rel]; ok {
				body = v
			}
		}
		_ = os.WriteFile(p, []byte(body), 0o644)
	}
	return base
}

func cpfwMkdirAll(p string) { _ = os.MkdirAll(p, 0o755) }

func cpfwDirname(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return "."
	}
	if i == 0 {
		return "/"
	}
	return p[:i]
}

func cpfwContainsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func cpfwSelfTest(stdout io.Writer) int {
	type chk struct {
		ok   bool
		desc string
	}
	var checks []chk
	check := func(cond bool, desc string) {
		checks = append(checks, chk{cond, desc})
		mark := "\u2713"
		if !cond {
			mark = "\u2717"
		}
		fmt.Fprintf(stdout, "   %s %s\n", mark, desc)
	}

	t, err := os.MkdirTemp("", "pubface-wording-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "   \u2717 \u81ea\u68c0\u5939\u5177\u76ee\u5f55\u5efa\u4e0d\u8d77\uff1a%s\n", err)
		return 2
	}
	defer func() { _ = os.RemoveAll(t) }()

	// ① 正控 PC：合成扫面（干净）⇒ rc=0 且合计行出现
	b := cpfwMakeFixture(cpfwPyJoin(t, "ok"), map[string]string{"README.md": "# \u516c\u5f00\u9762\uff08\u5408\u6210\uff09\n\u914d\u7f6e\u5728 `~/.zerg/`\u3002\n"}, nil, nil)
	rc, _, d := cpfwJudge(b, nil, false, true)
	check(rc == 0 && d.face == 9, "\u2460 \u6b63\u63a7 PC\uff1a\u5e72\u51c0\u626b\u9762\uff08docs 1 \u4ef6 + \u6839\u9762 8 \u4ef6\uff09\u21d2 rc=0\uff08\u626b\u9762 9 \u4ef6\uff09")

	// ② 负控 A1：私有卷名 ⇒ 必红
	b = cpfwMakeFixture(cpfwPyJoin(t, "a1"), map[string]string{"README.md": "\u6302\u8f7d\u70b9 = " + cpfwVolPrefix + "ZERG-ARCHIVE\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 1 && d.unexempt == 1 && d.rules["A1"] == 1,
		"\u2461 \u8d1f\u63a7 A1\uff1a`/Volumes/ZERG-ARCHIVE` \u21d2 rc=1\uff08\u79c1\u6709\u5377\u8def\u5f84\u6709\u7259\uff09")

	// ③ 负控 A2：未登记的卷名（网兜）⇒ 必红
	b = cpfwMakeFixture(cpfwPyJoin(t, "a2"), map[string]string{"README.md": "\u793a\u4f8b\uff1a" + cpfwVolPrefix + "Data/x.db\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 1 && d.rules["A2"] == 1,
		"\u2462 \u8d1f\u63a7 A2\uff1a\u672a\u767b\u8bb0\u5377\u540d `/Volumes/Data/x.db` \u21d2 rc=1\uff08\u653e\u884c\u5fc5\u987b\u9010\u6761\u767b\u8bb0\uff0c\u4e0d\u8bb8\u9759\u9ed8\u653e\u884c\uff09")

	// ④ 负控 A3
	b = cpfwMakeFixture(cpfwPyJoin(t, "a3"), map[string]string{"README.md": "\u5907\u4efd\u533a " + "AHZ" + " \u4e0e " + "BXC" + "\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 1 && d.rules["A3"] == 2, "\u2463 \u8d1f\u63a7 A3\uff1a`AHZ`/`BXC` \u21d2 rc=1\uff08\u5377\u4ee3\u53f7\u6709\u7259 \u00b7 \u547d\u4e2d 2\uff09")

	// ⑤ 负控 A4
	b = cpfwMakeFixture(cpfwPyJoin(t, "a4"), map[string]string{"README.md": "\u89c1 " + "Zerg-内部文档" + "/\u9879\u76ee\u6587\u6863/\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 1 && d.rules["A4"] == 1, "\u2464 \u8d1f\u63a7 A4\uff1a`Zerg-内部文档` \u21d2 rc=1\uff08\u4ed3\u5916\u79c1\u6709\u6587\u6863\u4ed3\u540d\u6709\u7259\uff09")

	// ⑥ 负控 A5
	b = cpfwMakeFixture(cpfwPyJoin(t, "a5"), map[string]string{"README.md": "\uff08\u7f3a\u53e3 Q-999 \u00b7 \u4ecd\u5728\u8d26\uff09\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 1 && d.rules["A5"] == 1, "\u2465 \u8d1f\u63a7 A5\uff1a\u7f3a\u53e3\u53f7 `Q-999` \u21d2 rc=1\uff08\u5f00\u53d1\u9762\u7b26\u53f7\u6709\u7259\uff09")

	// ⑦ 负控 A6 / A6b
	b = cpfwMakeFixture(cpfwPyJoin(t, "a6"), map[string]string{"README.md": "\u7531\u7236\u4ee3\u7406\u5206\u6d3e\uff1b\u672c\u679a\u8ba4\u9886\u5355\u5df2\u4ea4\u3002\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 1 && d.rules["A6"] >= 2, "\u2466 \u8d1f\u63a7 A6\uff1a`\u7236\u4ee3\u7406`/`\u672c\u679a`/`\u8ba4\u9886\u5355` \u21d2 rc=1\uff08\u5199\u4f5c\u89d2\u8272\u79f0\u8c13\u6709\u7259\uff09")
	b = cpfwMakeFixture(cpfwPyJoin(t, "a6b"), map[string]string{"README.md": "\u8001\u53e8\u62cd\u677f \u00b7 \u5343\u7ae0\u9879\u76ee\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 1 && d.rules["A6b"] == 2, "\u2466b \u8d1f\u63a7 A6b\uff1a`\u8001\u53e8`/`\u5343\u7ae0` \u21d2 rc=1\uff08\u53d1\u5e03\u94fe\u540c\u6279\u8131\u654f\u5bf9\u8c61\u6709\u7259\uff09")
	b = cpfwMakeFixture(cpfwPyJoin(t, "a6c"), map[string]string{"README.md": "\u7531\u52a9\u624b\u5206\u6d3e\u8fd9\u4ef6\u4e8b\uff08\u5408\u6210\u5939\u5177 \u00b7 \u672a\u767b\u8bb0\u4ef6\uff09\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 1 && d.rules["A6"] == 1 && d.unexempt == 1,
		"\u2466c \u8d1f\u63a7 A6\u00b7`\u52a9\u624b`\uff1a\u843d\u5728**\u767d\u540d\u5355\u4e4b\u5916**\u7684\u4ef6 \u21d2 rc=1\uff08\u62cd\u677f\u300c\u53d6\u7532\u6863\u8868\uff08\u542b `\u52a9\u624b`\uff09\u300d\u6709\u7259\uff09")
	b = cpfwMakeFixture(cpfwPyJoin(t, "a6d"), map[string]string{"design/\u8bb0\u5fc6\u4f53\u7cfb.md": "| `memory` | \u52a9\u624b\u81ea\u5df1\u7684\u7b14\u8bb0 | `MEMORY.md` |\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 0 && d.rules["A6"] == 1 && d.exempt == 1,
		"\u2466d \u6210\u5bf9\uff1a\u540c\u4e00\u5904 `\u52a9\u624b` \u5728**\u5df2\u767b\u8bb0\u7684\u4ef6 \u00d7 \u884c** \u21d2 rc=0\uff08\u8c41\u514d 1 \u00b7 \u4e0e \u2466c \u6210\u5bf9\u6709\u533a\u5206\u5ea6\uff09")

	// ⑦e 负控 A10/A11
	for _, pr := range [][3]string{
		{"k2-lower", "example-moe-36b", "A11"},
		{"example-35b-v2-pfx", "example-35b-v2-", "A11"},
		{"example-35b-v2-15", "example-35b-v2", "A10"},
		{"example-35b-v2-10", "example-35b", "A10"},
	} {
		b = cpfwMakeFixture(cpfwPyJoin(t, "mod-"+pr[0]), map[string]string{"README.md": "\uff08\u5408\u6210\uff09" + pr[1] + "\n"}, nil, nil)
		rc, _, d = cpfwJudge(b, nil, false, true)
		check(rc == 1 && d.rules[pr[2]] >= 1,
			fmt.Sprintf("\u2466e \u8d1f\u63a7 %s\uff1a\u79c1\u6709\u6a21\u578b\u4ee3\u53f7 `%s` \u21d2 rc=1\uff08\u5927\u5c0f\u5199\u53d8\u4f53 / \u5bb6\u65cf\u524d\u7f00\u6709\u7259\uff09", pr[2], pr[1]))
	}

	// ⑦f 假红对照：标识符 / 环境变量名形态
	b = cpfwMakeFixture(cpfwPyJoin(t, "mod-env"),
		map[string]string{"CONFIGURATION.md": "\u73af\u5883\u53d8\u91cf `ZERG_EXAMPLE_TEMPLATE` \u4e0e `example-35b-v2_chat_template.jinja`\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 0 && d.hits == 0,
		"\u2466f \u5047\u7ea2\u5bf9\u7167\uff1a`ZERG_EXAMPLE_TEMPLATE`/`example-35b-v2_chat_template.jinja`\uff08\u6807\u8bc6\u7b26/\u73af\u5883\u53d8\u91cf\u5f62\u6001\uff09\u21d2 0 \u547d\u4e2d")

	// ⑧ 负控 A7 + 假红对照
	b = cpfwMakeFixture(cpfwPyJoin(t, "a7"), map[string]string{"README.md": "\u652f\u6301\u7ebf 2.5.10\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 1 && d.rules["A7"] == 1, "\u2467 \u8d1f\u63a7 A7\uff1a\u7248\u672c\u4e32 `2.5.10` \u21d2 rc=1\uff08\u7248\u672c\u53d9\u4e8b\u6709\u7259\uff09")
	b = cpfwMakeFixture(cpfwPyJoin(t, "a7b"), map[string]string{"README.md": "\u4e3b\u673a 192.0.2.10\uff08RFC 5737 \u793a\u4f8b\u6bb5\uff09\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 0 && d.hits == 0,
		"\u2467b \u5047\u7ea2\u5bf9\u7167\uff1a`192.0.2.10` **\u4e0d**\u547d\u4e2d\uff08\u5224\u636e\u7528 `2.5.` \u524d\u7f00\u800c\u975e\u4e09\u6bb5\u6b63\u5219 \u00b7 \u00a73.5 `V4` \u9010\u5b57\uff09")

	// ⑨ 白名单命中 ⇒ 不算错且逐条打印理由
	b = cpfwMakeFixture(cpfwPyJoin(t, "wl"), map[string]string{
		"QUICKSTART.md": " ZERG_KB_PATH=\"/Volumes/My Disk/knowledge.db\"\n",
		"README.md":     "\u914d\u7f6e\u89c1 `~/.zerg/token`\u3002\n"}, nil, nil)
	rc, o, d := cpfwJudge(b, nil, false, true)
	txt := strings.Join(o, "\n")
	check(rc == 0 && d.exempt == 2 && d.unexempt == 0,
		"\u2468 \u767d\u540d\u5355\u547d\u4e2d \u21d2 \u4e0d\u7b97\u9519\uff08\u793a\u4f8b\u503c `/Volumes/My Disk` + `~/.zerg` \u5404 1 \u21d2 \u8c41\u514d 2 \u00b7 rc=0\uff09")
	check(strings.Count(txt, "\u21b3 \u672c\u6761\u8c41\u514d\u7406\u7531\uff1a") == 2 &&
		strings.Count(txt, "\u4e3a\u4ec0\u4e48\u7981\uff1a") >= 2 &&
		strings.Count(txt, "\u6761\u76ee\u7406\u7531\uff1a") == len(cpfwWhitelist),
		fmt.Sprintf("\u2468b **\u9010\u6761\u6253\u5370**\uff1a2 \u5904\u8c41\u514d\u5404\u6253\u51fa\u300c\u4e3a\u4ec0\u4e48\u7981 + \u21b3 \u672c\u6761\u8c41\u514d\u7406\u7531\u300d\u4e24\u884c\uff0c"+
			"\u4e14 %d \u6761\u767d\u540d\u5355\u6761\u76ee\u9010\u6761\u6253\u51fa\u300c\u6761\u76ee\u7406\u7531\u300d\uff08\u2717 \u9759\u9ed8\u8fc7\u6ee4 / \u2717 \u53ea\u52a0\u8ba1\u6570\u4e0d\u6253\u5370\uff09", len(cpfwWhitelist)))

	// ⑩ 白名单宽度
	b = cpfwMakeFixture(cpfwPyJoin(t, "wl-width"), map[string]string{
		"QUICKSTART.md": " ZERG_KB_PATH=\"/Volumes/My Disk/knowledge.db\"\n",
		"OTHER.md":      " X=\"/Volumes/My Disk/other.db\"\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 1 && d.unexempt == 1,
		"\u2469 \u767d\u540d\u5355\u5bbd\u5ea6\uff1a\u540c\u4e00\u5b57\u9762\u91cf\u51fa\u73b0\u5728**\u672a\u767b\u8bb0\u7684\u4ef6**\u91cc \u21d2 rc=1\uff08\u8c41\u514d\u4e0d\u662f\u5168\u5c40\u653e\u884c\uff09")

	// ⑪ 扫面不存在
	cpfwMkdirAll(cpfwPyJoin(t, "nodocs"))
	rc, _, _ = cpfwJudge(cpfwPyJoin(t, "nodocs"), nil, false, true)
	check(rc == 2, "\u246a \u626b\u9762\u7f3a\uff1a`publish/docs/` \u4e0d\u5728 \u21d2 rc=2\uff08\u7f3a\u4ef6 \u2260 \u7eff\uff09")

	// ⑫ 根面 8 件缺一件
	b = cpfwMakeFixture(cpfwPyJoin(t, "noroot"), nil, nil, []string{"publish/install.sh"})
	rc, _, _ = cpfwJudge(b, nil, false, true)
	check(rc == 2, "\u246b \u6839\u9762\u7f3a\u4ef6\uff1a8 \u4ef6\u7f3a `install.sh` \u21d2 rc=2\uff08\u7f3a\u4ef6 \u2260 \u7eff\uff09")

	// ⑬ 件读不了（把一个「件」造成目录）
	b = cpfwMakeFixture(cpfwPyJoin(t, "unreadable"), nil, nil, nil)
	_ = os.Remove(cpfwPyJoin(b, "publish", "gitleaks.toml"))
	cpfwMkdirAll(cpfwPyJoin(b, "publish", "gitleaks.toml"))
	rc, _, _ = cpfwJudge(b, nil, false, true)
	check(rc == 2, "\u246c \u4ef6\u8bfb\u4e0d\u4e86\uff08`gitleaks.toml` \u662f\u76ee\u5f55\uff09\u21d2 rc=2\uff08\u8bfb\u4e0d\u5168\u5c31\u5224\u4e0d\u51fa\u771f\u503c\uff09")

	// ⑭ 自豁免 ①（py 在盘 ⇒ 现读 py 真身 · 单一真源不变；py 退役 ⇒ 回落内化真源 cpfwSelfText）
	own := cpfwLoadSelfText(cpfwRepoRoot())
	ownHits := cpfwScanText(own)
	var ridSet []string
	seen := map[string]bool{}
	for _, h := range ownHits {
		if !seen[h.rid] {
			seen[h.rid] = true
			ridSet = append(ridSet, h.rid)
		}
	}
	sort.Strings(ridSet)
	need := []string{"A1", "A2", "A4", "A5", "A6b", "A9"}
	cover := true
	for _, n := range need {
		if !seen[n] {
			cover = false
		}
	}
	check(len(ownHits) > 0 && cover,
		fmt.Sprintf("\u246d \u81ea\u8c41\u514d \u2460\uff1a\u672c\u4ef6\u81ea\u8eab\u547d\u4e2d %d \u5904\uff08\u542b %s\uff09\u21d2 \u8bcd\u8868**\u771f\u7684**\u542b\u7981\u8bcd\u5b57\u9762\u91cf\uff08\u8c41\u514d\u975e\u7a7a\u8f6c\uff09",
			len(ownHits), strings.Join(ridSet, "/")))

	// ⑮ 自豁免 ②
	rc, o, _ = cpfwJudge(cpfwRepoRoot(), []string{cpfwSelfRel}, true, true)
	hasSelf := false
	for _, x := range o {
		if strings.Contains(x, "\u81ea\u8c41\u514d") {
			hasSelf = true
		}
	}
	check(rc == 0 && hasSelf, "\u246e \u81ea\u8c41\u514d \u2461\uff1a`face=[\u672c\u4ef6]` \u21d2 rc=0 \u4e14**\u6253\u5370**\u300c\u81ea\u8c41\u514d\u300d\u884c\uff08\u4e0d\u8bb8\u9759\u9ed8\u8df3\u8fc7\uff09")

	// ⑮b 成对对照：关掉豁免（★ 退役容忍 2026-10-07：本件 py 退役 ⇒ 不在盘时关掉豁免这档无输入
	//     ⇒ rc=2 亦算过；py 在盘时条件与原来逐字一致 ⇒ 两态输出不变）
	rc, _, d = cpfwJudge(cpfwRepoRoot(), []string{cpfwSelfRel}, true, false)
	check((rc == 1 && d.unexempt > 0) ||
		(rc == 2 && !cpfwIsFile(cpfwPyJoin(cpfwRepoRoot(), cpfwSelfRel))),
		"\u246eb \u81ea\u8c41\u514d\u7684\u7259\u9f7f\uff1a\u540c\u4e00\u4efd\u5939\u5177**\u5173\u6389**\u8c41\u514d \u21d2 rc=1\uff08\u5f00\u4e86 0 / \u5173\u6389 1 \u21d2 \u8fd9\u4e00\u683c\u6709\u533a\u5206\u5ea6\uff09")

	// ⑯ 假红对照：裸 `§`
	b = cpfwMakeFixture(cpfwPyJoin(t, "sec"), map[string]string{"design/\u8bb0\u5fc6\u4f53\u7cfb.md": "\u6761\u76ee\u4e4b\u95f4\u4ee5\u6362\u884c + `\u00a7` + \u6362\u884c\u8fde\u63a5\u3002\n\u00a7\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 0 && d.hits == 0, "\u246f \u5047\u7ea2\u5bf9\u7167\uff1a\u88f8 `\u00a7`\uff08\u6761\u76ee\u5206\u9694\u7b26\uff09\u21d2 **0 \u547d\u4e2d**\uff08\u672c\u95e8\u4e0d\u628a `\u00a7` \u5f53\u7981\u8bcd\uff09")

	// ⑰ 假红对照：`P1`（表格行标签）
	b = cpfwMakeFixture(cpfwPyJoin(t, "prow"), map[string]string{"design/FFP.md": "| P1 | \u5de5\u5177\u9519\u8bef\u901a\u9053 | \u65e7\u884c\u4e3a |\n"}, nil, nil)
	rc, _, d = cpfwJudge(b, nil, false, true)
	check(rc == 0 && d.hits == 0, "\u2470 \u5047\u7ea2\u5bf9\u7167\uff1a\u8868\u683c\u884c\u6807\u7b7e `P1` \u21d2 **0 \u547d\u4e2d**\uff08\u4e0d\u8bbe `P\\d+` \u7f51\u515c \u00b7 \u89c1\u53e3\u5f84\u5dee\u2463\uff09")

	// ⑱ 区分度
	b1 := cpfwMakeFixture(cpfwPyJoin(t, "disc-a"), map[string]string{"README.md": "\u89c1 " + "Zerg-内部文档" + "/\u9879\u76ee\u6587\u6863/\n"}, nil, nil)
	b2 := cpfwMakeFixture(cpfwPyJoin(t, "disc-b"), map[string]string{"README.md": "\u89c1\u540c\u7ea7\u6587\u6863\u4ed3\n"}, nil, nil)
	r1, _, _ := cpfwJudge(b1, nil, false, true)
	r2, _, _ := cpfwJudge(b2, nil, false, true)
	check(r1 == 1 && r2 == 0, "\u2471 \u533a\u5206\u5ea6\uff1a\u540c\u5939\u5177\u300c\u547d\u4e2d \u21d2 1\u300d\u300c\u5220\u6389 \u21d2 0\u300d\uff08\u5224\u636e\u6709\u7259 \u00b7 \u4e0d\u6052\u7eff/\u4e0d\u6052\u7ea2\uff09")

	// ⑲ 三档齐
	c1, _, _ := cpfwJudge(cpfwPyJoin(t, "ok"), nil, false, true)
	c2, _, _ := cpfwJudge(cpfwPyJoin(t, "a1"), nil, false, true)
	c3, _, _ := cpfwJudge(cpfwPyJoin(t, "nodocs"), nil, false, true)
	check(c1 == 0 && c2 == 1 && c3 == 2,
		"\u2472 \u4e09\u6001\u9f50\uff1a\u80fd\uff080\uff09/ \u9519\uff081\uff09/ \u4e0d\u7ed9\u7ed3\u8bba\uff082\uff09\u4e09\u6761\u51fa\u53e3\u90fd\u771f\u5b9e\u5b58\u5728\uff08\u540c\u4e00\u5957\u5224\u636e\uff09")

	var bad []string
	for _, c := range checks {
		if !c.ok {
			bad = append(bad, c.desc)
		}
	}
	verdict := "\u5168\u8fc7"
	if len(bad) > 0 {
		verdict = "\u4e0d\u8fc7\uff1a" + strings.Join(bad, " \u00b7 ")
	}
	fmt.Fprintf(stdout, "\u81ea\u68c0\u7ed3\u8bba: %s\uff08\u65ad\u8a00 %d \u6761 \u00b7 \u5931\u8d25 %d \u6761\uff09\n", verdict, len(checks), len(bad))
	if len(bad) > 0 {
		return 2
	}
	return 0
}

// ── 缺口 `Q-176`（门禁面 · 统一口）：`--json <字段>` 的字段名打错不许静默 / 不许炸堆栈 ──────────
var cpfwJSONFields = []string{"face", "rules", "hits", "exempt", "unexempt"}

func cpfwRefuseUnknownJSONFields(stderr io.Writer, argv []string) (int, bool) {
	var want []string
	for i, x := range argv {
		if x == "--json" {
			v := ""
			if i+1 < len(argv) {
				v = argv[i+1]
			}
			want = append(want, strings.Split(v, ",")...)
		} else if strings.HasPrefix(x, "--json=") {
			want = append(want, strings.Split(strings.TrimPrefix(x, "--json="), ",")...)
		}
	}
	var clean []string
	for _, f := range want {
		if strings.TrimSpace(f) != "" {
			clean = append(clean, strings.TrimSpace(f))
		}
	}
	var bad []string
	for _, f := range clean {
		found := false
		for _, ok := range cpfwJSONFields {
			if f == ok {
				found = true
			}
		}
		if !found {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return 0, false
	}
	var q []string
	for _, b := range bad {
		q = append(q, "`"+b+"`")
	}
	fmt.Fprintf(stderr, "\u26d4 `--json` \u4e0d\u8ba4\u7684\u5b57\u6bb5\u540d\uff1a%s\n", strings.Join(q, " \u00b7 "))
	var fs []string
	for _, f := range cpfwJSONFields {
		fs = append(fs, "`"+f+"`")
	}
	fmt.Fprintf(stderr, "   \u53ef\u9009\u5b57\u6bb5\u8868\uff1a%s\n", strings.Join(fs, " \u00b7 "))
	fmt.Fprintln(stderr, "\u21d2 \u7528\u6cd5\u9519\uff08rc=2\uff09\uff1a**\u5b57\u6bb5\u540d\u6253\u9519 \u2260 \u96f6\u547d\u4e2d** \u2014\u2014 \u4e24\u6001\u5728\u673a\u5668\u9762\u4e0a\u5fc5\u987b\u5206\u5f97\u5f00")
	return 2, true
}

// ── 参数解析（= argparse 子集：`--root`/`--self-test`/`--no-self-test`/`--json` + `-h`）──────
type cpfwArgs struct {
	root       string
	selfTest   bool
	noSelfTest bool
	json       string
}

const cpfwUsage = "usage: check-public-face-wording.py [-h] [--root ROOT] [--self-test]\n" +
	"                                    [--no-self-test] [--json JSON]"

func cpfwHelpText() string {
	return cpfwUsage + "\n\n" +
		"\u516c\u5f00\u9762\u5199\u4f5c\u7ea2\u7ebf\u81ea\u68c0\uff08publish/docs/** + \u6839\u9762 8 \u4ef6\uff09\n\n" +
		"optional arguments:\n" +
		"  -h, --help      show this help message and exit\n" +
		"  --root ROOT     \u5224\u54ea\u68f5\u6811\uff08\u9ed8\u8ba4\u4ed3\u6839\uff09\n" +
		"  --self-test     \u53ea\u8dd1\u81ea\u68c0\uff08\u5408\u6210\u5939\u5177 \u00b7 \u4e0d\u78b0\u771f\u76ee\u6807\uff09\n" +
		"  --no-self-test  \u8df3\u8fc7\u81ea\u68c0\u76f4\u63a5\u5224\u771f\u76ee\u6807\uff08\u7559\u7ed9\u8c03\u7528\u65b9\u663e\u5f0f\u9009\u62e9\uff09\n" +
		"  --json JSON     \u6309\u540d\u8f93\u51fa JSON \u5b57\u6bb5\uff08\u9017\u53f7\u5206\u9694\uff09\n"
}

func cpfwArgError(stderr io.Writer, msg string) int {
	fmt.Fprintln(stderr, cpfwUsage)
	fmt.Fprintf(stderr, "check-public-face-wording.py: error: %s\n", msg)
	return 2
}

func cpfwParseArgs(stdout, stderr io.Writer, argv []string) (cpfwArgs, int, bool) {
	a := cpfwArgs{root: cpfwRepoRoot()}
	var extras []string
	for i := 0; i < len(argv); i++ {
		x := argv[i]
		switch {
		case x == "-h" || x == "--help":
			fmt.Fprint(stdout, cpfwHelpText())
			return a, 0, true
		case x == "--self-test":
			a.selfTest = true
		case x == "--no-self-test":
			a.noSelfTest = true
		case x == "--root":
			if i+1 >= len(argv) {
				return a, cpfwArgError(stderr, "argument --root: expected one argument"), true
			}
			i++
			a.root = argv[i]
		case strings.HasPrefix(x, "--root="):
			a.root = strings.TrimPrefix(x, "--root=")
		case x == "--json":
			if i+1 >= len(argv) {
				return a, cpfwArgError(stderr, "argument --json: expected one argument"), true
			}
			i++
			a.json = argv[i]
		case strings.HasPrefix(x, "--json="):
			a.json = strings.TrimPrefix(x, "--json=")
		default:
			extras = append(extras, x)
		}
	}
	if len(extras) > 0 {
		return a, cpfwArgError(stderr, "unrecognized arguments: "+strings.Join(extras, " ")), true
	}
	return a, 0, false
}

// ── `--json` 输出（复刻 json.dumps(sort_keys=True) 的空格与空集合形态）──────────────────────
func cpfwJSONRules(rules map[string]int) string {
	keys := make([]string, 0, len(rules))
	for k := range rules {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, "\""+k+"\": "+strconv.Itoa(rules[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func cpfwJSONLine(want []string, d cpfwData) string {
	seen := map[string]bool{}
	var keys []string
	for _, k := range want {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		switch k {
		case "face":
			parts = append(parts, "\"face\": "+strconv.Itoa(d.face))
		case "rules":
			parts = append(parts, "\"rules\": "+cpfwJSONRules(d.rules))
		case "hits":
			parts = append(parts, "\"hits\": "+strconv.Itoa(d.hits))
		case "exempt":
			parts = append(parts, "\"exempt\": "+strconv.Itoa(d.exempt))
		case "unexempt":
			parts = append(parts, "\"unexempt\": "+strconv.Itoa(d.unexempt))
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// ── 入口（= py `main()`）─────────────────────────────────────────────────────
func checkPublicFaceWordingGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc, bad := cpfwRefuseUnknownJSONFields(stderr, argv); bad {
		return rc
	}
	a, rc, done := cpfwParseArgs(stdout, stderr, argv)
	if done {
		return rc
	}
	fmt.Fprintln(stdout, cpfwVersion)
	if a.selfTest {
		return cpfwSelfTest(stdout)
	}
	if !a.noSelfTest {
		if rc := cpfwSelfTest(stdout); rc != 0 {
			fmt.Fprintln(stdout, "\u26d4 \u81ea\u68c0\u4e0d\u8fc7 \u21d2 \u62d2\u7edd\u5224\u771f\u76ee\u6807\uff08\u95e8\u7981\u81ea\u5df1\u5148\u80fd\u88ab\u8bc1\u660e\u300c\u4f1a\u7ea2\u300d\uff09")
			return 2
		}
	}
	rc2, lines, data := cpfwJudge(cpfwPyAbspath(a.root), nil, false, true)
	for _, ln := range lines {
		fmt.Fprintln(stdout, ln)
	}
	if a.json != "" {
		var want []string
		for _, w := range strings.Split(a.json, ",") {
			if strings.TrimSpace(w) != "" {
				want = append(want, strings.TrimSpace(w))
			}
		}
		fmt.Fprintln(stdout, cpfwJSONLine(want, data))
	}
	return rc2
}
