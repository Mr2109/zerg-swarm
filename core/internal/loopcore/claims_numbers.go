// claims_numbers.go — GAP-20261018-01 第二半（P0）：把正文里的**硬读数**逐字核对真实回执 / 真实文件。
//
// 第一半（output_claims_derive.go 的 `DeriveClaims`）治的是「声称成功那条命令**根本没跑过**」与
// 「那条命令的回执**失败了**」；它拦不住本文件要治的那一类（实测形态，逐字）：
//
//	`zerg doc check-doc-number-live --format json` rc=0 · 扫了 136 件 / 1114 断言 / 陈旧 7 ·
//	`core/cmd/zerg/check_doc_freshness.go:261` 函数体完整
//
// —— 命令那条可能**真跑过且 rc=0**，而配上去的读数与「件:行」是编的。本文件只补这一环。
//
// ── 口径（已拍板，三档分级；✗ 不改口径）──
//
//	① 硬 · `rc=N` —— 只认**紧跟**在一条 `zerg …` 命令之后的那个 `rc=N`（不是把正文里所有 `rc=0`
//	   当断言扫）。取该命令本轮的**真实回执**：`N` 与回执真实 rc **不符** ⇒
//	   `Claim{Text: 该句, ReceiptID: 该回执ID, Quote: "rc=<N>"}` ⇒ 既有 `VerifyClaims` 判
//	   **send_back**（片段与回执原文不符：那条回执里没有 `rc=N` 这行）。
//	   该命令本轮**没有回执**、或回执里判不出 rc ⇒ **不判**（「命令没跑过」归第一半，不在此重复判）。
//
//	② 硬 · `文件:行` —— 正文里形如 `<路径>:<数字>` 的引用（路径含 `/` 或带已知后缀
//	   `.go/.md/.json/.yaml/.yml/.sh/.tsv`）：在 `root` 下读该件，
//	   **不存在 / 不是普通件 / 行号超出文件总行数** ⇒
//	   `Claim{ReceiptID: ClaimNonexistentPrefix + 引用串, Quote: 引用串}` ⇒ **retract**（假回执）。
//	   ★ **只判「存在 ∧ 行号在范围内」**，✗ **不判「该行内容对不对」** —— 后者是语义判断（同一行常被
//	   概括转述），判它会误杀，本批不做（留在语义核查那一层）。
//
//	   ★★ **同句语境判定（GAP-20261018-04）**：这处 `件:行` 只有在**肯定引用**时才算主张 ——
//	   同一句里出现「否定 / 不存在」或「疑问 / 假设」词（见 `numericNonAssertiveWords`）⇒ **不判**：
//	   那是在复述题面、或在说「这个件没有」，不是「我引用了这个件的这一行」。
//	   铁证（实测那一发）：题面自带一个**不存在**的路径，模型老实答「这个文件不在仓里，第 42 行
//	   无从谈起」并把题面的路径原样复述一遍 ⇒ 旧口径判 1 条 retract、正文被追加【降级完成】终局
//	   说明，**诚实回答被误杀**。★ 判据一律**保守优先**（见下「边界」末条）。
//
//	③ 软 · 其它计数（`136 件` / `1114 断言` / `陈旧 7` 这类）—— **只写进 `SoftNotes`**：
//	   ✗ 不生成主张、✗ 不进正文、✗ 不判红。理由：这类数字可能引自缺口账 / 上下文 / 门的历史读数，
//	   一律判红会逼模型**不敢报数**（比漏判更坏）。
//
// ── 判定面（✗ 不另开一套裁决）──
//
// 本文件只负责**造主张**；裁决仍是**同一枚** `VerifyClaims`（receipt.go，一行未改），
// 终局仍是既有那条链（`ApplyOutputClaimFacts` → `ChooseTerminal`，一行未改）。
// run.go 只把这里算出的 `HardFailed` **并进**派生那一批（`mergeNumericClaims`），不新建状态名、不重排优先级。
//
// ── 边界（保守：宁可漏判，不可误杀正常交付）──
//
//	· `root` 为空 ⇒ **只核 rc、不判文件引用**（解析不了根就别猜；零回归）。
//	· 引用是绝对路径 / 含 `..` / 在 URL 里 ⇒ 不判（不是「仓内件:行」这种形态）。
//	· 件读不了（权限等）⇒ 不判（判不出来就不猜，与 `traceExitCode` 同一条纪律）。
//	· ★ **同句带否定 / 疑问 / 假设词 ⇒ 不判**（`numericRefIsAssertion`）—— **保守优先：拿不准就不判**；
//	  宁可放过一个假引用，不可再误杀一段诚实回答（误杀的代价是给真话追加终局说明，比漏判严重得多）。
//	· 软核**恒不影响**产出正文：`SoftNotes` 只进内存结构，谁都不许把它写进 `res.Content`。
package loopcore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// NumericClaimCheck — 正文硬断言的核对结果（纯数据；✗ 不含任何裁决机制）。
//
//	HardFailed — 硬核判红的条目（**复用**既有四态结果类型 `ClaimVerdictResult`，✗ 不新造结构）：
//	             只收 {retract, send_back}（与 `OutputClaimCheck.Failed` 同一闭集口径）。
//	SoftNotes  — 软核记录（其它计数断言）：**只记录、不进正文、不判红**。
type NumericClaimCheck struct {
	HardFailed []ClaimVerdictResult
	SoftNotes  []string
}

// numericMaxSoftNotes — 软核记录条数上限（低基数：软核是观察面，不是判据面；不设上限会随正文长度膨胀）。
const numericMaxSoftNotes = 32

// numericRCAssert — 正文里的 `rc=N` 断言形态（`zerg` 工具回执首行是 `rc=%d`，见 chat_tool_zergcmd.go）。
var numericRCAssert = regexp.MustCompile(`rc=([0-9]+)`)

// numericRCGapAllowed — 「紧跟」允许夹在命令与 `rc=N` 之间的字符（空白 / 排版符 / 常见连接符）。
//
// 只认这些：命令与 `rc=N` 之间一旦出现**实词**（尤其汉字）就不算「紧跟」—— 那可能是
// 「zerg xxx 之前那次 rc=2」这类回忆式叙述，拿它判红就是误杀。
const numericRCGapAllowed = " \t`*_()（）[]{}「」『』【】,，.。:：=->→⇒~|/"

// numericSoftUnit — 软核：`<数字> + 计数单位>`（`136 件` / `1114 断言`）。
var numericSoftUnit = regexp.MustCompile(`[0-9]+\s*(?:件|条|行|断言|个|项|处|次|张|份|步|门|秒)`)

// numericSoftLabel — 软核：`<计数标签> + 数字>`（`陈旧 7` / `命中 3` / `剩余 12`）。
var numericSoftLabel = regexp.MustCompile(`(?:陈旧|新增|命中|失败|覆盖|缺口|待办|剩余|未登记|已登记|已解)\s*[0-9]+`)

var (
	// numericFileRefKnown — `<路径>:<数字>`，路径带已知后缀（含无 `/` 的件名，如 `run.go:12`）。
	// 路径字符集含 `\p{L}` ⇒ 中文路径（`core/xxx/不存在.go`、`Zerg-内部文档/项目文档/x.md`）也认得。
	numericFileRefKnown = regexp.MustCompile(`[\p{L}\p{N}_.\-@+~]+(?:/[\p{L}\p{N}_.\-@+~]+)*\.(?:go|md|json|yaml|yml|sh|tsv):([0-9]+)`)
	// numericFileRefAny — `<路径>:<数字>`，路径含 `/`（后缀不在已知集里也算引用）。
	numericFileRefAny = regexp.MustCompile(`[\p{L}\p{N}_.\-@+~]+(?:/[\p{L}\p{N}_.\-@+~]+)+:([0-9]+)`)
)

// numericNonAssertiveWords — 「同句里出现这些词 ⇒ 该处 `件:行` **不是**在主张这个引用」的词表。
//
// 两组（逐字，改这里必须同批改 claims_numbers_test.go 的四组用例）：
//
//	① 否定 / 不存在 —— 模型在说「这个件没有」；
//	② 疑问 / 假设 —— 模型在复述题面、问要不要、给条件式。
//
// ★ **保守优先：拿不准就不判。** 这张表只增不减，措辞上拿不准的一律按「不是主张」处理 ——
// 宁可放过一个假引用（漏判），不可再误杀一段诚实回答（误杀会把真话追加【降级完成】终局说明）。
var numericNonAssertiveWords = []string{
	// ① 否定 / 不存在
	"不存在", "没有", "不在", "无从谈起", "找不到", "缺失", "查无", "并无", "未找到",
	// ② 疑问 / 假设
	"如果你", "是否需要", "要不要", "假如", "假设", "若是", "如果要", "请确认",
}

// numericRefIsAssertion — 这处 `件:行` 是否属**肯定引用**（该判）；同句带非主张词 ⇒ false（不判）。
//
// ★ 判之前先把 `ref` 本身从同句里摘掉：词表要判的是**行文**，不是路径 —— 路径自己就可能带这些字
// （`core/xxx/不存在.go:12` 是既有用例里的真形态），把路径算进去会把「肯定引用了这个不存在的件」
// 也一起放过（那就成了漏判一条本来就该判的假引用）。
func numericRefIsAssertion(sentence, ref string) bool {
	ctx := sentence
	if ref != "" {
		ctx = strings.ReplaceAll(ctx, ref, " ")
	}
	for _, w := range numericNonAssertiveWords {
		if strings.Contains(ctx, w) {
			return false
		}
	}
	return true
}

// claimSentenceBreak — 句子边界（与 `splitClaimSentences` 同一套：换行与中英硬句读；逗号**不**切）。
//
// ★ 这里另写一份而不是复用该函数：本腿要的是「某个**位置**所在的那一句」，需要边界的字节偏移，
// 而 `splitClaimSentences` 只给切好的串、给不回位置。两处集合必须一致（改一处要同批核另一处）。
func claimSentenceBreak(r rune) bool {
	switch r {
	case '\n', '\r', '。', '！', '？', '；', ';', '!', '?':
		return true
	}
	return false
}

// sentenceAround — 取该字节位置所在的**一句**（裁剪空白；位置越界 ⇒ 空串 ⇒ 判不出来就不判）。
func sentenceAround(content string, pos int) string {
	if pos < 0 || pos > len(content) {
		return ""
	}
	lo, hi := 0, len(content)
	for i := 0; i < pos && i < len(content); {
		r, w := utf8.DecodeRuneInString(content[i:])
		if claimSentenceBreak(r) {
			lo = i + w
		}
		i += w
	}
	for i := pos; i < len(content); {
		r, w := utf8.DecodeRuneInString(content[i:])
		if claimSentenceBreak(r) {
			hi = i
			break
		}
		i += w
	}
	if lo > hi || lo > len(content) {
		return ""
	}
	return strings.TrimSpace(content[lo:hi])
}

// VerifyNumericClaims — 对定稿正文跑硬核（rc / 文件:行）与软核（其它计数）。
//
// 纯函数：不写盘、不改 traces、不改正文。判定一律交既有 `VerifyClaims`（本函数只造 `Claim`）。
func VerifyNumericClaims(content string, traces []Trace, root string) NumericClaimCheck {
	var out NumericClaimCheck
	if strings.TrimSpace(content) == "" {
		return out
	}
	if claims := numericHardClaims(content, traces, root); len(claims) > 0 {
		// 同一枚裁决函数、同一份回执集；本函数只把**命中失败**挑出来（keep/revise 不收）。
		for _, r := range VerifyClaims(claims, MemberReceipts(traces)) {
			switch r.Verdict {
			case VerdictRetract, VerdictSendBack:
				out.HardFailed = append(out.HardFailed, r)
			}
		}
	}
	out.SoftNotes = numericSoftNotes(content)
	return out
}

// mergeNumericClaims — 把硬核命中失败**并进**派生那一批（run.go 的唯一接线点）。
//
// 并的是**已经过 `VerifyClaims` 的裁决结果**（同一枚裁决函数，不是第二套判据）：`Results` 追加、
// `Failed` 追加；`Declared` 置真（有硬核失败就是「有引用载体」，否则 `HasFailure` 之外的观测面会误读成
// 「本轮没有引用」）。无硬核失败 ⇒ **原样返回**（零回归：与接线前逐字节一致）。
func mergeNumericClaims(base OutputClaimCheck, hard []ClaimVerdictResult) OutputClaimCheck {
	if len(hard) == 0 {
		return base
	}
	base.Declared = true
	base.Results = append(base.Results, hard...)
	base.Failed = append(base.Failed, hard...)
	return base
}

// numericHardClaims — 造硬核主张：先逐句核 `rc=N`，再逐条核 `文件:行`（两档都只造「判得出来」的那些）。
func numericHardClaims(content string, traces []Trace, root string) []Claim {
	var out []Claim
	seen := make(map[string]bool, 8)

	// ① rc=N：需要回执可对，先把轨迹里的 zerg 命令串取出来（与第一半同一口径 `traceZergCommand`）。
	cs := make([]claimCand, 0, len(traces))
	for _, t := range traces {
		if cmd := traceZergCommand(t); cmd != "" {
			cs = append(cs, claimCand{cmd: cmd, t: t})
		}
	}
	if len(cs) > 0 && strings.Contains(content, "rc=") {
		for _, raw := range splitClaimSentences(content) {
			s := strings.TrimSpace(raw)
			if s == "" {
				continue
			}
			c, ok := numericRCClaim(s, cs)
			if !ok {
				continue
			}
			key := "rc|" + c.ReceiptID + "|" + c.Quote + "|" + c.Text
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, c)
		}
	}

	// ② 文件:行：逐条现读 root 下的真件（只判存在与行号范围）。
	for _, hit := range scanFileLineRefs(content) {
		if seen["file|"+hit.ref] {
			continue
		}
		seen["file|"+hit.ref] = true
		if c, ok := judgeFileLineRef(hit, root); ok {
			out = append(out, c)
		}
	}
	return out
}

// numericRCClaim — 单句核 `rc=N`：只有「紧跟在该句点名的那条 zerg 命令之后」的那个 N 才算断言。
//
// 返回 (主张, 是否判红)；真话（N == 真实 rc）/ 判不出来 / 命令本轮没跑过 ⇒ false（不判）。
func numericRCClaim(sentence string, cs []claimCand) (Claim, bool) {
	named := zergCommandInSentence(sentence)
	if named == "" {
		return Claim{}, false
	}
	// 命令在句中的**位置**：正文照抄命令时能直接定位；定位不到（多空格/反引号夹在词间等排版变形）
	// ⇒ **不判**（保守：定位不到就说不清「紧跟」是哪一段，宁漏不误杀）。
	idx := strings.Index(sentence, named)
	if idx < 0 {
		return Claim{}, false
	}
	rest := sentence[idx+len(named):]
	m := numericRCAssert.FindStringSubmatchIndex(rest)
	if m == nil {
		return Claim{}, false
	}
	for _, r := range rest[:m[0]] {
		if !strings.ContainsRune(numericRCGapAllowed, r) {
			return Claim{}, false // 命令与 rc=N 之间夹了实词 ⇒ 不是「紧跟」这条命令的读数
		}
	}
	want, err := strconv.Atoi(rest[m[2]:m[3]])
	if err != nil {
		return Claim{}, false
	}
	ns := normalizeForMatch(sentence)
	nn := normalizeForMatch(named)
	hitID := ""
	for _, c := range cs {
		if !traceCommandHit(ns, nn, normalizeForMatch(c.cmd)) {
			continue
		}
		real, ok := traceExitCode(c.t)
		if !ok {
			continue // 回执里判不出 rc ⇒ 不猜
		}
		if real == want {
			return Claim{}, false // 真话：正文读数与回执一致 ⇒ 不判
		}
		if hitID == "" && strings.TrimSpace(c.t.CallID) != "" {
			hitID = c.t.CallID
		}
	}
	if hitID == "" {
		return Claim{}, false // 该命令本轮没有可引用的回执 ⇒ 交第一半（不在此重复判）
	}
	return Claim{Text: sentence, ReceiptID: hitID, Quote: "rc=" + strconv.Itoa(want)}, true
}

// fileRefHit — 一处 `文件:行` 引用及其所在行（所在行只作主张正文，参与判定）。
type fileRefHit struct {
	ref  string // 逐字引用串（`core/x/y.go:12`）
	ctx  string // 引用所在的那一行（裁剪后；只作主张正文）
	sent string // 与 ref **同句**的语境（带否定 / 疑问 / 假设词 ⇒ 不判，见 numericRefIsAssertion）
}

// scanFileLineRefs — 扫出正文里所有 `文件:行` 引用（去重、保序）。
//
// 跳过：URL 里的（`https://h/a.go:1`）、以及 `--flag=core/x:12` 这种紧随 `=` 的旗标取值
// （那是参数、不是「引用了这个件的这一行」）；**同句带否定 / 疑问 / 假设词的也跳过**（判在
// `judgeFileLineRef` 入口，这里先把同句语境一并取好 —— 取不出语境（空串）也不据此跳过）。
func scanFileLineRefs(content string) []fileRefHit {
	var out []fileRefHit
	seen := make(map[string]bool, 8)
	add := func(m []int) {
		if m == nil {
			return
		}
		ref := content[m[0]:m[1]]
		if seen[ref] {
			return
		}
		if insideURL(content, m[0]) || afterAssign(content, m[0]) {
			return
		}
		seen[ref] = true
		out = append(out, fileRefHit{
			ref:  ref,
			ctx:  lineAround(content, m[0]),
			sent: sentenceAround(content, m[0]),
		})
	}
	for _, m := range numericFileRefKnown.FindAllStringIndex(content, -1) {
		add(m)
	}
	for _, m := range numericFileRefAny.FindAllStringIndex(content, -1) {
		add(m)
	}
	return out
}

// insideURL — 该位置是否落在 `scheme://…` 里（URL 的 path:port 不是「件:行」引用）。
func insideURL(content string, start int) bool {
	lo := start - 16
	if lo < 0 {
		lo = 0
	}
	seg := content[lo:start]
	i := strings.LastIndex(seg, "://")
	if i < 0 {
		return false
	}
	return !strings.ContainsAny(seg[i+3:], " \t\n\"'`（）()")
}

// afterAssign — 该位置是否紧跟在 `=` 之后（`--baseline=core/x.go:12` 是参数取值，不是引用）。
func afterAssign(content string, start int) bool {
	i := start - 1
	for i >= 0 && (content[i] == ' ' || content[i] == '\t') {
		i--
	}
	return i >= 0 && content[i] == '='
}

// lineAround — 取该位置所在行（裁剪；上限 200 字，免得主张正文过长）。
func lineAround(content string, pos int) string {
	lo := strings.LastIndexByte(content[:pos], '\n') + 1
	hi := strings.IndexByte(content[pos:], '\n')
	end := len(content)
	if hi >= 0 {
		end = pos + hi
	}
	s := strings.TrimSpace(content[lo:end])
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200])
	}
	return s
}

// judgeFileLineRef — 判一处引用：先判「这是不是**肯定引用**」，再判「件在不在 ∧ 行号在不在范围内」。
//
// 返回 (主张, 是否判红)。判不出来（同句带非主张词 / root 空 / 绝对路径 / 含 `..` / 读不了件）
// ⇒ false（不猜）。
func judgeFileLineRef(hit fileRefHit, root string) (Claim, bool) {
	// ★ 同句语境优先：句子里已经在说「不存在 / 如果你要 / 要不要」⇒ 这是复述题面或否定陈述，
	// 不是「我引用了这个件的这一行」⇒ **不判**（保守优先，见 numericNonAssertiveWords）。
	if !numericRefIsAssertion(hit.sent, hit.ref) {
		return Claim{}, false
	}
	i := strings.LastIndex(hit.ref, ":")
	if i <= 0 {
		return Claim{}, false
	}
	path, lineStr := hit.ref[:i], hit.ref[i+1:]
	lineNo, err := strconv.Atoi(lineStr)
	if err != nil || lineNo < 1 {
		return Claim{}, false
	}
	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") || filepath.IsAbs(path) {
		return Claim{}, false // 不是「root 下的仓内件」形态 ⇒ 不判
	}
	if strings.TrimSpace(root) == "" {
		return Claim{}, false // 没有解析根 ⇒ 只核 rc，不判文件引用（零回归）
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	info, statErr := os.Stat(full)
	if statErr != nil || info.IsDir() {
		return badRefClaim(hit), true // 件不存在（或不是普通件）⇒ 假引用
	}
	total, ok := lineCountOf(full)
	if !ok {
		return Claim{}, false // 读不了 ⇒ 判不出来就不猜
	}
	if lineNo > total {
		return badRefClaim(hit), true // 行号超出总行数 ⇒ 也是假引用
	}
	return Claim{}, false
}

// badRefClaim — 「引用了不存在的东西」的主张：合成一个不可能存在的回执号 ⇒ `VerifyClaims` 判 retract。
func badRefClaim(hit fileRefHit) Claim {
	return Claim{
		Text:      hit.ctx,
		ReceiptID: ClaimNonexistentPrefix + hit.ref,
		Quote:     hit.ref,
	}
}

// lineCountOf — 件总行数（`a\nb\n` = 2 行；末行不带换行也算 1 行）。
func lineCountOf(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	n := bytes.Count(b, []byte("\n"))
	if len(b) > 0 && b[len(b)-1] != '\n' {
		n++
	}
	return n, true
}

// numericSoftNotes — 软核：把其它计数读数记下来（**只记录**，不进正文、不判红）。
func numericSoftNotes(content string) []string {
	var out []string
	seen := make(map[string]bool, 8)
	add := func(s string) {
		if s == "" || seen[s] || len(out) >= numericMaxSoftNotes {
			return
		}
		seen[s] = true
		out = append(out, fmt.Sprintf("%s（软核：未与回执对拍，可能引自账/上下文）", s))
	}
	for _, m := range numericSoftUnit.FindAllString(content, -1) {
		add(m)
	}
	for _, m := range numericSoftLabel.FindAllString(content, -1) {
		add(m)
	}
	return out
}
