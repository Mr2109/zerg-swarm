// family_script_retire.go —— `zerg script retire`：**退场正门**（只管登记面 · ✗ 不代管物理删除）。
//
// 已拍口径（逐字 · 以设计稿为准）：`~/.zerg/state/设计-退场正门-retire-20261006.md`
// ＋ `~/.zerg/state/拍板落定-所有稿-20261007.md`（决-01/02/03/09 · **决-12**）：
//
//	· 决-01：本正门**只做登记面退场**，✗ 不代管 `git rm`（真删交 git 历史＋提案审计）。
//	· 决-02：退场真源**沿用** `core/internal/contract/adopt-retire.json` 的 `retire_ledger`
//	  （✗ 不另起第二真源）；本命令是它的**唯一写面**。
//	· 决-12：目标件闭集 = **自研面**（＝全仓 − `vendor/**` − `tools/ocr/venv/**` 的 py/sh）。
//	  （2026-10-07 Mr2109 拍 · 原文作废：原 `scripts/**`＋`core/**` 只是「退场流程最初扫到的那两棵树」
//	  ✗ ≠ 自研面 ⇒ 闭集该跟大目标的定义一致，✗ 不跟某个流程的历史范围一致。）
//	· 决-09：例外类 = **逐条登记名册 ＋ 理由 ＋ 保留代价栏**（同真源 `retire_exceptions`）。
//
// 命令形状（逐旗标可跑一格 · 声明面 ⟷ 实现面）：
//
//	zerg script retire <件> [--dry-run | --yes] [--evidence <出处>]… [--by <谁>] [--json <字段>]
//
// 退码表（照设计稿 §三 · 与 §五 负控三条同源）：
//
//	0  登记面退场成功（`--dry-run` 计划面 或 `--yes` 真落）
//	1  拒绝（**有据的红**）：目标件不在闭集 · 目标件是例外类（能力依赖外部工具／联网／本机物件）
//	2  用法错 / **不给结论**：缺 `--yes` 且缺 `--dry-run` · `--dry-run` 与 `--yes` 同给 ·
//	   `--yes` 缺 `--evidence`（防无凭退场）· 退场真源取不到 · 目标件**不在盘**
//	14 冲突 / 被占：真源里**已有同 `object`** 退场条目（幂等优先：已在该状态按状态报）
//
// 写面纪律（照设计稿 退场-03「写坏怎么退」＋ 同族先例 `script inventory sync`）：
//
//	审计先落盘 → `cp -p` 备份真源 → 临时件 + fsync + 原子 rename → 写后读回对拍 →
//	复查不过逐字节写回（回退一行：`git checkout <sha> -- core/internal/contract/adopt-retire.json`）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// scriptRetireTruthRel —— 退场真源（**沿用** · 决-02：✗ 不另起第二真源）。
const scriptRetireTruthRel = "core/internal/contract/adopt-retire.json"

// scriptRetireLedgerKey / scriptRetireExceptionKey —— 真源里的两块登记面（逐字照真源现有键名）。
const scriptRetireLedgerKey = "retire_ledger"
const scriptRetireExceptionKey = "retire_exceptions"

// scriptRetireStateWord —— 本正门只写这一档状态（登记面退场）。
//
// ★ 为什么**不**写「已删」：物理删除**不归本门**（决-01）⇒ 写「已删」就是替 `git rm` 许一个空承诺
//
//	（真删还没发生）。本门只把件标成「**已登记退场**」；真删由人／收口批显式做。
const scriptRetireStateWord = "已登记退场"

// scriptRetireEntry / scriptRetireExc —— 两块的条目形状。列面逐字照真源现有条目
// （ledger：object/state/evidence）＋ 决-09 的例外类三列（object/reason/keep_cost）。
type scriptRetireEntry struct {
	Object   string `json:"object"`
	State    string `json:"state"`
	Evidence string `json:"evidence"`
}

type scriptRetireExc struct {
	Object     string `json:"object"`
	Category   string `json:"category"`
	Reason     string `json:"reason"`
	KeepCost   string `json:"keep_cost"`
	ReviewDate string `json:"review_date"`
}

// scriptRetireDoc —— 真源里本命令**读**到的两块（写面走文本级追加 · 见 `scriptRetireAppendToArray`）。
type scriptRetireDoc struct {
	RetireLedger     []scriptRetireEntry `json:"retire_ledger"`
	RetireExceptions []scriptRetireExc   `json:"retire_exceptions"`
}

// scriptRetireClosedSetDesc —— 闭集口径的**唯一描述串**（决-12）。
//
// ★ 报话 / 计划件**全引它**，✗ 不各处手写一份（防「两处各写一套前缀/口径」而长歪）。
const scriptRetireClosedSetDesc = "自研面全体 py/sh（全仓 − `vendor/**` − `tools/ocr/venv/**`）"

// scriptRetireClosedSetExcludedPrefixes —— 闭集的**排除前缀表**（决-12 · ★ 单点真源）。
//
// 闭集 = **自研面** = 全仓 − 这些前缀；余下的 `.py`/`.sh` 件即在闭集。
//
// ★ 只此一处：命令闸（`cmdScriptRetire`）与名册机检（`scriptRetireExceptionErrors`）**共用**
//
//	`scriptRetireInClosedSet`，✗ 不得另开第二份前缀表（判据：全包只此一个 `var …ExcludedPrefixes`）。
//
// 依据 2026-10-07 决-12（Mr2109「按」）：闭集该跟「自研面」（大目标：272 件）这个**目标定义**一致，
//
//	✗ 不跟「退场流程最初扫到的那两棵树」（原 `scripts/**`＋`core/**`）一致。
var scriptRetireClosedSetExcludedPrefixes = []string{"vendor/", "tools/ocr/venv/"}

// scriptRetireInClosedSet —— 目标件闭集判定（决-12）：自研面 py/sh（全仓 − `vendor/**` − `tools/ocr/venv/**`）。
//
// **逐字**只认两条：① ✗ 不落在任一排除前缀下；② 后缀 ∈ {.py, .sh}。
func scriptRetireInClosedSet(rel string) bool {
	rel = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(rel)), "./")
	if rel == "" {
		return false
	}
	for _, p := range scriptRetireClosedSetExcludedPrefixes {
		if strings.HasPrefix(rel, p) {
			return false
		}
	}
	return strings.HasSuffix(rel, ".py") || strings.HasSuffix(rel, ".sh")
}

// ── 例外类名册**逐条机检**（GAP-20261007-03 · 决-09 四列 / 决-12 闭集 / review_date）──────────
//
// ★ 为什么要有这一段：名册条目此前**无任何门核**（`adopt_retire_test.go` 的 arDoc 根本不解析
// `retire_exceptions` 键）⇒ 四列缺一、`object` 落在闭集外、`review_date` 写坏 都无人拦；
// 且闭集外件登进名册后对本正门**无牙**（闭集闸先拒 ⇒ 例外档读不到）。此处把这三条落成机检。

// scriptRetireExcCols —— 决-09 的例外类**四列**（`object` 之外的登记列 · 逐条须齐）。
var scriptRetireExcCols = []string{"category", "reason", "keep_cost", "review_date"}

// scriptRetireExcReviewDateLayout —— 审查日期格式（真源现有条目逐字用 `2006-01-02`）。
const scriptRetireExcReviewDateLayout = "2006-01-02"

// ── 第④格：`reason` 首词闭集（2026-10-08）──────────────────────────────────────
//
// 口径（真源 = `core/internal/contract/adopt-retire.json` boundaries 末两条）：
//
//	`reason` 须写成 `<首词>：<为什么物理上不能 Go 化>`；首词 = **首个全角 `：`（U+FF1A）之前**
//	那段（再 TrimSpace），首词须 ∈ 闭集（**5 档**）。★ 与「闭集内件（决-12）」是**两个面**，✗ 别混。
//
// ★ **判域 ≡ `reason` 首词 ∈ 闭集（乙案 · 2026-10-08 代拍）**：判域只认契约（`boundaries[7]`）已定义语汇的
// `reason` 首词（5 档）；原「显式名单 ∪ `category∈{物理不可化,硬线,自举}`」双支已删（`category` 无契约语汇 ·
// 腿净贡献 0）。下 17 件名单**降级为对照/校验**（✗ 非判域 · 仅断言「名单内件仍在判域」）。
var scriptRetireExcReasonPrefixes = []string{"生态绑定", "OS胶水", "vendor", "硬线", "自举"}

// scriptRetireExcReasonColon —— 首词分隔符 = **全角冒号**（U+FF1A）；✗ 半角 `:` 不算。
const scriptRetireExcReasonColon = "\uFF1A"

// scriptRetireExcReasonDomainObjects —— **对照/校验**名单（现读 17 件 · ✗ **非判域**）。
//
// ★ 名单不再当判域选择子：判域 ≡ `reason` 首词 ∈ 闭集（契约真源）⇒ 名册增删**自动随动**，消掉
// 「添件忘补名单 ⇒ 漏判」这颗雷。此名单只作回归对照（仅校验「名单内件仍在判域」· ✗ 只许随名册瘦身下调）。
var scriptRetireExcReasonDomainObjects = []string{
	"deploy/scripts/deploy-launch.sh",
	"docs/site/export-and-build.sh",
	"mcp/bench_llmlingua2.py",
	"mcp/bench_llmlingua2_v2.py",
	"mcp/test_compress_models.py",
	"mcp/test_selective_context.py",
	"scripts/build/build-wall-mutate.py",
	"scripts/build/negctl-build-wall.sh",
	"scripts/gates/mutation-smoke.sh",
	"scripts/gates/precommit-gates.sh",
	"scripts/gates/two-states-gate-mutate.py",
	"scripts/sandbox-probes/negctl-two-states.sh",
	"scripts/svc/setup-zerg.sh",
	"scripts/wall/wall-bridge-mutate.py",
	"scripts/wall/wall-macos-mutate.py",
	"tools/ocr/ocr_server.py",
	"zerg-evals/backends/bfcl_scorer.py",
}

// scriptRetireExcReasonPrefixOf —— 取 `reason` 首词（首个全角 `：` 之前 · TrimSpace）；无全角冒号/首词空 ⇒ ok=false。
func scriptRetireExcReasonPrefixOf(reason string) (string, bool) {
	i := strings.Index(reason, scriptRetireExcReasonColon)
	if i < 0 {
		return "", false
	}
	w := strings.TrimSpace(reason[:i])
	return w, w != ""
}

// scriptRetireExcReasonPrefixClosed —— 首词是否 ∈ 闭集（逐字相等 · ✗ 不前缀匹配）。
func scriptRetireExcReasonPrefixClosed(w string) bool {
	for _, p := range scriptRetireExcReasonPrefixes {
		if w == p {
			return true
		}
	}
	return false
}

// scriptRetireExcInReasonDomain —— 判域 ≡ `reason` 首词 ∈ 闭集（契约 `boundaries[7]` 单点真源）。
// ★ 乙案：判域只认 `reason` 首词（5 档）；✗ 不看 `category` 列（无契约语汇 · 净贡献 0）。
func scriptRetireExcInReasonDomain(reason string) bool {
	w, ok := scriptRetireExcReasonPrefixOf(strings.TrimSpace(reason))
	return ok && scriptRetireExcReasonPrefixClosed(w)
}

// scriptRetireExcOnRatchetList —— ★ 对照/校验（✗ 非判域）：object 是否在对照名单内。
func scriptRetireExcOnRatchetList(object string) bool {
	o := strings.TrimSpace(object)
	for _, x := range scriptRetireExcReasonDomainObjects {
		if x == o {
			return true
		}
	}
	return false
}

// scriptRetireExcReasonHead —— 报话里回显 reason 的前 40 个字符（按 rune 截 · ✗ 不切坏多字节）。
func scriptRetireExcReasonHead(reason string) string {
	const n = 40
	rs := []rune(reason)
	if len(rs) <= n {
		return reason
	}
	return string(rs[:n]) + "…"
}

// scriptRetireExcElem —— 名册里**一条顶层元素**的原文 ＋ 起行（1-based）。
type scriptRetireExcElem struct {
	Text string
	Line int
}

// scriptRetireExcElements —— 在真源原文里切出 `<key>` 数组的每个**顶层元素**（原文 + 起行）。
// 形状不认识（找不到块 / `[` 没配对的 `]` / 元素 `{` 没配对的 `}`）⇒ fail-closed 返错，✗ 不猜。
func scriptRetireExcElements(raw, key string) ([]scriptRetireExcElem, error) {
	marker := "\"" + key + "\""
	ki := strings.Index(raw, marker)
	if ki < 0 {
		return nil, fmt.Errorf("真源里找不到 %q 块", key)
	}
	ai := strings.Index(raw[ki:], "[")
	if ai < 0 {
		return nil, fmt.Errorf("%q 块后找不到 `[`", key)
	}
	start := ki + ai + 1
	// 数组的配对 `]`（depth 感知 · 字符串感知 —— 与 scriptRetireAppendToArray 同口径）。
	depth, end, inStr, esc := 0, -1, false, false
	for i := start; i < len(raw); i++ {
		c := raw[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '[':
			depth++
		case ']':
			if depth == 0 {
				end = i
			} else {
				depth--
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("%q 块的 `[` 没有配对的 `]`", key)
	}
	lineAt := func(idx int) int { return strings.Count(raw[:idx], "\n") + 1 }
	var out []scriptRetireExcElem
	for i := start; i < end; {
		for i < end && raw[i] != '{' {
			i++
		}
		if i >= end {
			break
		}
		objStart := i
		objDepth, objInStr, objEsc, closeIdx := 0, false, false, -1
		for j := objStart; j < end; j++ {
			c := raw[j]
			if objInStr {
				switch {
				case objEsc:
					objEsc = false
				case c == '\\':
					objEsc = true
				case c == '"':
					objInStr = false
				}
				continue
			}
			switch c {
			case '"':
				objInStr = true
			case '{':
				objDepth++
			case '}':
				objDepth--
				if objDepth == 0 {
					closeIdx = j
				}
			}
			if closeIdx >= 0 {
				break
			}
		}
		if closeIdx < 0 {
			return nil, fmt.Errorf("%q 块里有个元素的 `{` 没有配对的 `}`", key)
		}
		out = append(out, scriptRetireExcElem{Text: raw[objStart : closeIdx+1], Line: lineAt(objStart)})
		i = closeIdx + 1
	}
	return out, nil
}

// scriptRetireExceptionErrors —— 例外类名册**逐条机检**（GAP-20261007-03）：
//
//	① **四列齐**：`category` / `reason` / `keep_cost` / `review_date` 都非空（`object` 亦不许空·主键）；
//	② `object` 在**决-12 闭集**里（自研面 py/sh：全仓 − `vendor/**` − `tools/ocr/venv/**`）；
//	③ `review_date` **可解析**（`2006-01-02`）；
//	④ `reason` **首词 ∈ 闭集**（5 档）—— 判域 ≡ 首词合档（契约真源）；对照名单内件由本格**校验**。
//
// 返回的每条错都带 **`件:行`**；空返回 = 名册形态合。纯函数：只看传入的原文，✗ 不碰盘
// ⇒ 负控可喂合成件。调用方（`cmdScriptRetire`）据此**不给结论**（退码 2）。
func scriptRetireExceptionErrors(truthRel, raw string) []string {
	fail := func(line int, format string, a ...any) string {
		return fmt.Sprintf("%s:%d: %s", truthRel, line, fmt.Sprintf(format, a...))
	}
	elems, err := scriptRetireExcElements(raw, scriptRetireExceptionKey)
	if err != nil {
		return []string{fmt.Sprintf("%s: 名册块形状不认识：%v", truthRel, err)}
	}
	var errs []string
	for _, el := range elems {
		var e scriptRetireExc
		if err := json.Unmarshal([]byte(el.Text), &e); err != nil {
			errs = append(errs, fail(el.Line, "条目解不动（%v）", err))
			continue
		}
		object := strings.TrimSpace(e.Object)
		if object == "" {
			errs = append(errs, fail(el.Line, "object 列是空的（名册以 object 为主键）"))
		}
		col := map[string]string{
			"category":    strings.TrimSpace(e.Category),
			"reason":      strings.TrimSpace(e.Reason),
			"keep_cost":   strings.TrimSpace(e.KeepCost),
			"review_date": strings.TrimSpace(e.ReviewDate),
		}
		for _, c := range scriptRetireExcCols {
			if col[c] == "" {
				errs = append(errs, fail(el.Line, "%s 列是空的（object=%q · 决-09 四列须齐）", c, object))
			}
		}
		if object != "" && !scriptRetireInClosedSet(object) {
			errs = append(errs, fail(el.Line,
				"object=%q 不在决-12 闭集（%s）", object, scriptRetireClosedSetDesc))
		}
		if rd := col["review_date"]; rd != "" {
			if _, perr := time.Parse(scriptRetireExcReviewDateLayout, rd); perr != nil {
				errs = append(errs, fail(el.Line, "review_date=%q 解不动（要 `2006-01-02`）：%v", rd, perr))
			}
		}
		// ④ 判域 ≡ `reason` 首词 ∈ 闭集（真源 boundaries[7] · 5 档）——
		//    ★ 对照名单仅作**校验**（✗ 非判域）：名单内件须仍在判域（首词合档）；名单外件判域随动。
		if scriptRetireExcOnRatchetList(object) {
			if reason := col["reason"]; reason != "" {
				if w, ok := scriptRetireExcReasonPrefixOf(reason); !ok || !scriptRetireExcReasonPrefixClosed(w) {
					errs = append(errs, fail(el.Line,
						"reason 首词 ∉ 闭集（reason=%q）—— 须 `<%s>：<为什么物理上不能 Go 化>`（全角冒号 `：`）",
						scriptRetireExcReasonHead(reason), strings.Join(scriptRetireExcReasonPrefixes, " / ")))
				}
			}
		}
	}
	return errs
}

// ScriptRetireExceptionErrorsForTest —— 名册机检的**测试钩子**（同 `RunForTest` / `RepoRootForTest` 一族：
// 判据体在 `package main`，外部测试包 `main_test` 够不到未导出名 ⇒ 开一个只读出口 · ✗ 不改判据）。
func ScriptRetireExceptionErrorsForTest(truthRel, raw string) []string {
	return scriptRetireExceptionErrors(truthRel, raw)
}

func scriptRetireSHA(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func scriptRetireSHAOfFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return scriptRetireSHA(b)
}

// scriptRetireQuote —— JSON 字符串（**✗ 不做 HTML 转义** —— 真源里 `>`、`&` 一类要逐字留）。
func scriptRetireQuote(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return `""`
	}
	return strings.TrimRight(b.String(), "\n")
}

// scriptRetireRenderEntry —— 一条 ledger 条目按真源现有体例渲染（4 空格缩进 · 键各一行）。
func scriptRetireRenderEntry(e scriptRetireEntry) string {
	return "    {\n" +
		"      \"object\": " + scriptRetireQuote(e.Object) + ",\n" +
		"      \"state\": " + scriptRetireQuote(e.State) + ",\n" +
		"      \"evidence\": " + scriptRetireQuote(e.Evidence) + "\n" +
		"    }"
}

// scriptRetireAppendToArray —— 文本级追加：把 `elemRendered` 追加进真源里 `"<key>": [ … ]` 数组末尾。
//
// ★ 为什么走文本级而**不**整份 `json.Marshal` 回写：真源是**人维护**的（键序固定 · 散文列面），
//
//	整份重序列化会把全件重排/重转义（一个字段不动也刷出一大版 diff）⇒ 只用**最小插入**。
//
// ★ 形状不认识（数组元素结尾不是 `}`）⇒ **拒写**（fail-closed），✗ 不猜。
func scriptRetireAppendToArray(text, key, elemRendered string) (string, error) {
	marker := "\"" + key + "\""
	ki := strings.Index(text, marker)
	if ki < 0 {
		return "", fmt.Errorf("真源里找不到 %q 块", key)
	}
	ai := strings.Index(text[ki:], "[")
	if ai < 0 {
		return "", fmt.Errorf("%q 块后找不到 `[`", key)
	}
	start := ki + ai + 1
	depth, end, inStr, esc := 0, -1, false, false
	for i := start; i < len(text); i++ {
		c := text[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '[':
			depth++
		case ']':
			if depth == 0 {
				end = i
			} else {
				depth--
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return "", fmt.Errorf("%q 块的 `[` 没有配对的 `]`", key)
	}
	inner := text[start:end]
	if strings.TrimSpace(inner) == "" {
		// 空数组：直接放进第一枚（`[` 后换行缩进）
		return text[:start] + "\n" + elemRendered + "\n  " + text[end:], nil
	}
	j := len(inner) - 1
	for j >= 0 {
		c := inner[j]
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			j--
			continue
		}
		break
	}
	if j < 0 || inner[j] != '}' {
		return "", fmt.Errorf("%q 块的元素结尾不是 `}`（形状不认识 ⇒ 拒写）", key)
	}
	pos := start + j + 1
	return text[:pos] + ",\n" + elemRendered + text[pos:], nil
}

// scriptRetireAudit —— 审计行（落同一件 `<状态目录>/edit_audit.jsonl` · **自己的事件名 + 自己的字段名**）。
type scriptRetireAudit struct {
	At      string `json:"at"`
	Event   string `json:"event"`
	Face    string `json:"face"`
	Target  string `json:"target"`
	Before  string `json:"ledger_before_sha256"`
	After   string `json:"ledger_after_sha256"`
	Entries int    `json:"ledger_entries_before"`
	By      string `json:"by"`
	Confirm string `json:"confirm"`
}

func scriptRetireAppendAudit(path string, l scriptRetireAudit) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("审计落点解析不出来（HOME / ZERG_STATE_DIR 都取不到）")
	}
	if strings.TrimSpace(l.At) == "" || strings.TrimSpace(l.Event) == "" || strings.TrimSpace(l.Target) == "" ||
		strings.TrimSpace(l.Before) == "" || strings.TrimSpace(l.After) == "" {
		return fmt.Errorf("审计行缺必需格 ⇒ 这一行不许写")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(l)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// scriptRetireBackupPath —— 写前备份落点（状态目录 · 与同族 `script inventory sync` 同规）。
func scriptRetireBackupPath() string {
	return filepath.Join(statepath.Dir(), "script-retire-backup.json")
}

// scriptRetireBackup —— 备份 = `cp -p` 的等价（逐字节落件 + 保留源文件权限位）。
func scriptRetireBackup(src, dst string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(src); err == nil {
		mode = st.Mode().Perm()
	}
	return os.WriteFile(dst, body, mode)
}

// scriptRetireResidualTemplate —— 残余保留面（件头说明 **模板**）。
//
// ★ 取舍（如实登记）：设计稿 §二 第 ② 项「残余保留面」在本版**只印**（计划面 ＋ 结果面），
//
//	✗ 不另落件 —— 落件会新增登记面（门76 面① / 公开标记表 / 判据注册表）⇒ 爆破半径扩大，
//	与本门「只动登记面」的定位相左；要落件归收口批。
func scriptRetireResidualTemplate(target, evidence string) string {
	return "残余保留面（件头说明 · 模板 —— 本正门只印，✗ 不另落件）：\n" +
		"  # 本件已退场（**登记面**）：" + target + "\n" +
		"  # 退场依据：" + evidence + "\n" +
		"  # ★ 真删交 git 历史＋提案审计（本正门 ✗ 不代管 `git rm`）"
}

// scriptRetireChangeLine —— 变更件一行（设计稿 §二 第 ③ 项 · 同样只印）。
func scriptRetireChangeLine(target, state, evidence string) string {
	return "变更件一行：" + target + " ⇒ " + state + "（evidence=" + evidence + "）"
}

// scriptRetirePrintPlan —— 计划件（`--dry-run` 与「缺 `--yes`」两档**打印同一份**）。
func scriptRetirePrintPlan(out, stderr io.Writer, title, target, state, truthPath string,
	ledgerCount int, evidence []string, by, beforeSHA, afterSHA, backupPath, auditPath string) {
	fmt.Fprintln(out, "计划件（"+title+" · 零副作用 —— 未写件、未写审计）")
	fmt.Fprintf(out, "  真源     : %s\n", truthPath)
	fmt.Fprintf(out, "  目标件   : %s\n", target)
	fmt.Fprintf(out, "  闭集判定 : 在闭集（%s）\n", scriptRetireClosedSetDesc)
	fmt.Fprintf(out, "  例外类   : 否（✗ 不在例外类名册 `%s`）\n", scriptRetireExceptionKey)
	fmt.Fprintf(out, "  既有条目 : %d 条（同 `object` 0 条 —— 无冲突）\n", ledgerCount)
	fmt.Fprintf(out, "  将追加   : {\"object\":%s,\"state\":%s,\"evidence\":%s}\n",
		scriptRetireQuote(target), scriptRetireQuote(state), scriptRetireQuote(orDashList(evidence)))
	fmt.Fprintf(out, "  落 state : %s\n", state)
	fmt.Fprintf(out, "  --by     : %s\n", by)
	fmt.Fprintln(out, scriptRetireResidualTemplate(target, orDashList(evidence)))
	fmt.Fprintln(out, scriptRetireChangeLine(target, state, orDashList(evidence)))
	fmt.Fprintf(out, "  写前 sha : %s\n", beforeSHA)
	fmt.Fprintf(out, "  写后 sha : %s（按**将要写的内容**现算）\n", afterSHA)
	fmt.Fprintf(out, "  备份落点 : %s\n", backupPath)
	fmt.Fprintf(out, "  审计落点 : %s\n", auditPath)
	fmt.Fprintf(stderr, "（%s：只出计划件 · 零副作用 —— 未写件、未写审计）\n", title)
}

// cmdScriptRetire —— `zerg script retire`（退场正门 · D2 写面）。
func cmdScriptRetire(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	// ── 位置参数：恰一件（逐件点名 · ✗ 不批量退）──────────────────────────────
	if len(inv.args) == 0 {
		inv.setErr("usage", "missing_target", "要给一件要退场的件（仓内相对路径 · 自研面 py/sh）")
		fmt.Fprintf(stderr, "%s: `script retire` 要给**一件**要退场的件（例：%s script retire scripts/gates/xxx.py --dry-run）⇒ 退码 2\n",
			progName, progName)
		return exitUsage
	}
	if len(inv.args) > 1 {
		inv.setErr("usage", "too_many_targets", "本命令逐件点名（一次一件 · ✗ 不批量退）")
		fmt.Fprintf(stderr, "%s: 收到 %d 个位置参数 —— 本命令**逐件点名**（一次一件 · 设计稿 §七 Q10 · ✗ 不批量退）⇒ 退码 2\n",
			progName, len(inv.args))
		return exitUsage
	}
	target := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(inv.args[0])), "./")

	// ── 闭集（决-03）：不在闭集 ⇒ 拒执（有据的红 · 退码 1）─────────────────────
	if !scriptRetireInClosedSet(target) {
		msg := fmt.Sprintf("目标件 %q 不在闭集（%s）—— 拒执（有据的红 · 退码 1）", target, scriptRetireClosedSetDesc)
		inv.setErr("declaration_rejected", "out_of_closed_set", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitFail
	}

	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不到退场真源（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	// ── 目标件**必须在盘**（不在盘 ⇒ 不给结论 · 退码 2 · 「空 ≠ 缺」）────────────
	if st, err := os.Stat(filepath.Join(root, target)); err != nil || st.IsDir() {
		inv.setErr("usage", "target_absent", "目标件不在盘 ⇒ 不给结论")
		fmt.Fprintf(stderr, "%s: 目标件不在盘（%s）⇒ **不给结论**（退码 2 —— 「不在盘」≠「已退场」· 空 ≠ 缺）\n", progName, target)
		return exitUsage
	}

	truthPath := filepath.Join(root, scriptRetireTruthRel)
	old, err := os.ReadFile(truthPath)
	if err != nil {
		inv.setErr("usage", "ledger_unreadable", "退场真源读不到 ⇒ 不给结论")
		fmt.Fprintf(stderr, "%s: 退场真源读不到（%s）：%v ⇒ **不给结论**（退码 2）\n", progName, scriptRetireTruthRel, err)
		return exitUsage
	}
	var doc scriptRetireDoc
	if err := json.Unmarshal(old, &doc); err != nil {
		inv.setErr("usage", "ledger_unparsable", "退场真源解不动 ⇒ 不给结论")
		fmt.Fprintf(stderr, "%s: 退场真源解不动（%s）：%v ⇒ **不给结论**（退码 2）\n", progName, scriptRetireTruthRel, err)
		return exitUsage
	}

	// ── 例外类名册**逐条机检**（GAP-20261007-03）：四列齐 ＋ `object` 在决-03 闭集 ＋ `review_date` 可解析 ──
	//  名册形态坏 ⇒ 例外档不可信（可能漏判一件是例外类）⇒ **不给结论**（fail-closed · 退码 2），
	//  ✗ 不静默按坏名册去找例外。每条错带 `件:行`。
	if rosterErrs := scriptRetireExceptionErrors(scriptRetireTruthRel, string(old)); len(rosterErrs) > 0 {
		inv.setErr("declaration_rejected", "exception_roster_malformed", "例外类名册形态坏 ⇒ 不给结论")
		fmt.Fprintf(stderr, "%s: 例外类名册形态坏（`%s` 的 `%s` · %d 处）⇒ **不给结论**（退码 2 —— 名册不可信时例外档读不得）：\n",
			progName, scriptRetireTruthRel, scriptRetireExceptionKey, len(rosterErrs))
		for _, e := range rosterErrs {
			fmt.Fprintf(stderr, "  %s\n", e)
		}
		return exitUsage
	}

	// ── 例外类（决-09）：逐条登记名册 ＋ 理由 ＋ 保留代价栏 ⇒ 拒退（退码 1）──────
	for _, e := range doc.RetireExceptions {
		if strings.TrimSpace(e.Object) == target {
			msg := fmt.Sprintf("目标件 %q 是**例外类**（能力依赖外部工具／联网／本机物件 ⇒ ✗ 不可退）：%s · 保留代价：%s",
				target, e.Reason, e.KeepCost)
			inv.setErr("declaration_rejected", "exception_class", msg)
			fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
			fmt.Fprintf(stderr, "例外类名册在真源 `%s` 的 `%s`（逐条登记：object / reason / keep_cost · 决-09）—— 要退它得先改名册并给理由\n",
				scriptRetireTruthRel, scriptRetireExceptionKey)
			return exitFail
		}
	}

	// ── 冲突（退码 14）：真源里已有同 `object` 退场条目 ─────────────────────────
	for _, e := range doc.RetireLedger {
		if strings.TrimSpace(e.Object) == target {
			msg := fmt.Sprintf("真源里**已有** %q 的退场条目（state=%s）⇒ 冲突／被占（退码 14 · 幂等优先：已在该状态按状态报）",
				target, e.State)
			inv.setErr("conflict", "already_retired", msg)
			fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
			return exitConflict
		}
	}

	// ── 凭据（`--evidence`）：真写必填 ≥1（防无凭退场 · 设计稿 §一）──────────────
	evidence := []string{}
	for _, v := range inv.flagVals("--evidence") {
		if s := strings.TrimSpace(v); s != "" {
			evidence = append(evidence, s)
		}
	}
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = "（未给 —— 取调用方身份）"
	}
	state := scriptRetireStateWord + "（" + time.Now().Format("2006-01-02") + "）"
	evidenceText := strings.Join(evidence, " · ")
	if evidenceText == "" {
		evidenceText = "（未给 —— 干跑面允许留空；`--yes` 真写必填）"
	}
	entry := scriptRetireEntry{Object: target, State: state, Evidence: evidenceText}
	newText, err := scriptRetireAppendToArray(string(old), scriptRetireLedgerKey, scriptRetireRenderEntry(entry))
	if err != nil {
		inv.setErr("usage", "ledger_shape_unknown", "退场真源登记面形状不认识 ⇒ 拒写")
		fmt.Fprintf(stderr, "%s: %v ⇒ **拒写**（退码 2 · 形状不认识时不猜）\n", progName, err)
		return exitUsage
	}
	beforeSHA := scriptRetireSHA(old)
	afterSHA := scriptRetireSHA([]byte(newText))
	backupPath := scriptRetireBackupPath()
	auditPath := editAuditPath()

	// ── `--dry-run`：只出计划件，零副作用 ─────────────────────────────────────
	if inv.dryRun {
		out := stdout
		if inv.jsonGiven {
			out = stderr
		}
		scriptRetirePrintPlan(out, stderr, "--dry-run", target, state, truthPath, len(doc.RetireLedger),
			evidence, by, beforeSHA, afterSHA, backupPath, auditPath)
		if inv.jsonGiven {
			return scriptRetireEmitJSON(inv, stdout, stderr, "dry_run", target, state,
				len(doc.RetireLedger), beforeSHA, afterSHA, auditPath)
		}
		return exitOK
	}

	// ── 缺 `--yes`：fail-closed（D2 档 · 从不提问）──────────────────────────────
	if !inv.yes {
		out := stdout
		if inv.jsonGiven {
			out = stderr
		}
		scriptRetirePrintPlan(out, stderr, "--yes 前的一次预览", target, state, truthPath, len(doc.RetireLedger),
			evidence, by, beforeSHA, afterSHA, backupPath, auditPath)
		if inv.jsonGiven {
			if rc := scriptRetireEmitJSON(inv, stdout, stderr, "need_yes", target, state,
				len(doc.RetireLedger), beforeSHA, afterSHA, auditPath); rc != exitOK {
				return rc
			}
		}
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: `script retire` 是 D2 档 —— **缺 --yes ⇒ 不执行**（照 `repo commit` 的 fail-closed · 从不提问）\n", progName)
		fmt.Fprintf(stderr, "先看计划件：%s script retire %s --dry-run\n", progName, target)
		return exitUsage
	}

	// ── `--yes` 真写：缺 `--evidence` ⇒ 2（防无凭退场 · 设计稿 §一）────────────
	if len(evidence) == 0 {
		inv.setErr("usage", "evidence_required", "--yes 真写缺 --evidence ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: `script retire … --yes` **缺 --evidence ⇒ 不执行**（防无凭退场 · 设计稿 §一）⇒ 退码 2\n", progName)
		return exitUsage
	}

	// 幂等：现读与将写逐字节相同（正常不会发生 —— 已过冲突闸，留作兜底）
	if beforeSHA == afterSHA {
		fmt.Fprintf(stdout, "幂等命中：真源逐字节不变（sha %s）⇒ 不写件、不落审计\n", beforeSHA)
		if inv.jsonGiven {
			return scriptRetireEmitJSON(inv, stdout, stderr, "idempotent", target, state,
				len(doc.RetireLedger), beforeSHA, afterSHA, auditPath)
		}
		return exitOK
	}

	// ---- 真写：审计先落盘 → 备份 → 原子写 → 写后读回对拍 → 复查不过逐字节写回 ----
	audit := scriptRetireAudit{
		At: time.Now().Format(time.RFC3339), Event: "retire_registered", Face: "script retire",
		Target: target, Before: beforeSHA, After: afterSHA, Entries: len(doc.RetireLedger),
		By: by, Confirm: "--yes",
	}
	if err := scriptRetireAppendAudit(auditPath, audit); err != nil {
		inv.setErr("blocked", "audit_unwritable", "审计写不进 ⇒ 不写件")
		fmt.Fprintf(stderr, "%s: 审计写不进（%s）：%v ⇒ **不写件**（没有审计的写 = 之后判不了「谁改的」）\n", progName, auditPath, err)
		return exitBlocked
	}
	if err := scriptRetireBackup(truthPath, backupPath, old); err != nil {
		inv.setErr("blocked", "backup_unwritable", "写前备份落不下来 ⇒ 不写件")
		fmt.Fprintf(stderr, "%s: 写前备份落不下来（%s）：%v ⇒ 不写件（回滚要有可证的落点）\n", progName, backupPath, err)
		return exitBlocked
	}
	if err := scriptInvWriteAtomic(truthPath, []byte(newText)); err != nil {
		inv.setErr("failed", "write_failed", "原子写失败")
		fmt.Fprintf(stderr, "%s: 原子写失败：%v ⇒ 已按写前备份逐字节写回\n", progName, err)
		_ = scriptInvWriteAtomic(truthPath, old)
		return exitFail
	}
	if back := scriptRetireSHAOfFile(truthPath); back != afterSHA {
		fmt.Fprintf(stderr, "%s: 写后读回对拍不等（现算 %s ≠ 期望 %s）⇒ 逐字节写回\n", progName, back, afterSHA)
		_ = scriptInvWriteAtomic(truthPath, old)
		inv.setErr("failed", "readback_mismatch", "写后读回对拍不等 ⇒ 已回滚")
		return exitFail
	}
	fmt.Fprintf(stdout, "已登记退场 %s ⇒ %s\n", target, state)
	fmt.Fprintf(stdout, "真源 %s · 条目 %d → %d · sha %s → %s\n",
		scriptRetireTruthRel, len(doc.RetireLedger), len(doc.RetireLedger)+1, scriptInvShort(beforeSHA), scriptInvShort(afterSHA))
	fmt.Fprintf(stdout, "审计落点 %s（事件 retire_registered · 追加只写）· 备份 %s\n", auditPath, backupPath)
	fmt.Fprintf(stdout, "★ 本正门**只管登记面**：✗ 未删任何件（真删交 git 历史＋提案审计 · 决-01）\n")
	fmt.Fprintln(stdout, scriptRetireResidualTemplate(target, evidenceText))
	fmt.Fprintln(stdout, scriptRetireChangeLine(target, state, evidenceText))
	if inv.jsonGiven {
		return scriptRetireEmitJSON(inv, stdout, stderr, "retired", target, state,
			len(doc.RetireLedger), beforeSHA, afterSHA, auditPath)
	}
	return exitOK
}

// scriptRetireEmitJSON —— 机器面（六键包封由 `selectJSONList` 出）。
func scriptRetireEmitJSON(inv *invocation, stdout, stderr io.Writer, result, target, state string,
	entriesBefore int, beforeSHA, afterSHA, auditPath string) int {
	if rc := requireFields(inv, stderr); rc != exitOK {
		return rc
	}
	row := map[string]string{
		"result":                result,
		"object":                target,
		"state":                 state,
		"closed_set":            "yes",
		"in_exceptions":         "no",
		"ledger_entries_before": fmt.Sprintf("%d", entriesBefore),
		"ledger_entries_after":  fmt.Sprintf("%d", entriesBefore+1),
		"ledger_before_sha256":  beforeSHA,
		"ledger_after_sha256":   afterSHA,
		"audit_path":            auditPath,
	}
	return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, []map[string]string{row})
}
