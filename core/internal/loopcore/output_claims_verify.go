// output_claims_verify.go — GAP-20261018-01 第二半（续）：正文硬断言的**分级核**收口到
// 「引文是否真在所指的那一行」＋「软核读数的取证分级」。
//
// ── 前情（现读，逐条）──
//
//	· `claims_numbers.go` 的 `VerifyNumericClaims` 已落三档：硬 `rc=N`（紧跟命令的那个数与该命令真实回执
//	  的 rc 逐字比）、硬 `文件:行`（件是否存在 ∧ 行号是否超界）、软 其它计数（只记 `SoftNotes`，不判红）。
//	  它把「该行内容对不对」**明确留给了另一层**（该件文件头逐字：「后者是语义判断…本批不做（留在语义
//	  核查那一层）」）—— 本文件就是那一层。
//
// ── 本文件补的三格（✗ 不新开第五态、✗ 不另立判据）──
//
//	① B 类 ③「引文要在那一行」：一句话里用**反引号**点了符号名，且这处引文**紧贴**一处 `件:行` 引用
//	   ⇒ 该符号必须出现在**那一行**的正文里；不在 ⇒ send_back。
//	   判据落到「行证据」上，而不是新造状态：把那一行的原文当作一份回执（ID = `line:<件>:<行>`），
//	   主张片段 = 那个引文 ⇒ **既有** `VerifyClaims` 的判据「片段必须能在指定回执原文里找到」自然给出
//	   keep（行上有这句）/ send_back（行在、但行上没这句）。★ 没有第五条判据，也没有第五个状态名。
//
//	② A 类补同义形态 `退出码 N`：第一版只认 `rc=N`；「退出码 0」这类同义写法同样是**硬**断言 ——
//	   紧跟命令的那个数必须与该命令真实回执的 rc 逐字相同，不符 ⇒ 引用该回执 ⇒ send_back。
//
//	③ C 类取证分级：其它计数（件数 / 行数 / 条数 / 字节 / 百分比）在**本轮回执原文**里找得到 ⇒ 标
//	   「已取证」；找不到 ⇒ 标「未取证」并进 warning（`output_claim_soft` 事件）。**恒不判红、恒不进正文**：
//	   这类数常引自缺口账 / 上下文 / 门的历史读数，一律判红会逼模型不敢报数（比漏判更坏）。
//
// ── 保守边界（全部「拿不准就不判」；误杀的代价比漏判严重得多）──
//
//	· 引文只认**标识符形态**：`DeriveClaims` / `pkg.Fn` / `Fn(args)`。反引号里是命令（`zerg …`）、
//	  路径、URL、文件名的 ⇒ 不判；含中文 / 空白的散文 ⇒ 不判（那是行文强调，判它必误杀真话）。
//	· 引文必须**紧贴**引用（同一句内，与 `件:行` 的间距 ≤ `numericQuoteWindow` 个字符）：
//	  一句话里另有远处的反引号（「…，它还调用了 `splitClaimSentences`」）不算对这一行的断言 ⇒ 不判。
//	· 同句带否定 / 疑问 / 假设词（`numericNonAssertiveWords`，沿用 GAP-20261018-04 口径）⇒ 不判。
//	· 件不存在 / 行号超界 ⇒ **本文件不判**（归 `claims_numbers.go` 的假引用那一格，免得同一条出两票）。
//	· 件读不了 / 不是普通件 / `root` 空 ⇒ 不判（判不出来就不猜）。
//	· 软核**恒不碰产出正文**：只进结构体与事件载荷，谁都不许把它写进 `res.Content`。
package loopcore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// NumericGranularCheck — 分级核的**增量**结果（硬核判红 ＋ 软核取证分级）。
//
//	HardFailed   — 硬核判红的条目（与 `NumericClaimCheck.HardFailed` 同一闭集 {retract, send_back}，
//	               同一结果类型 `ClaimVerdictResult`；调用方用既有 `mergeNumericClaims` 并进同一批）。
//	SoftBacked   — 软核：在本轮回执原文里**找得到**的读数（「已取证」）。
//	SoftUnbacked — 软核：回执里**找不到**的读数（「未取证」⇒ 只进 warning，✗ 不判红）。
type NumericGranularCheck struct {
	HardFailed   []ClaimVerdictResult
	SoftBacked   []string
	SoftUnbacked []string
}

const (
	// claimLineEvidencePrefix — 「行证据回执」的合成 ID 前缀。
	// 真实回执 ID 来自 `Trace.CallID`（短号），不可能以此前缀开头 ⇒ 两者不会撞号。
	claimLineEvidencePrefix = "line:"
	// numericQuoteWindow — 引文与 `件:行` 的**最大间距**（字节；ASCII 引文 ≈ 字符）。
	// 只判紧贴的那处引文：一句里远处的另一个反引号不是对这一行的断言。
	numericQuoteWindow = 40
	// numericQuotedMaxRunes — 单处引文长度上限（超过就不像符号名，像整段散文）。
	numericQuotedMaxRunes = 80
	// numericSoftWarnTokens — warning 载荷里每类最多带几个读数（低基数：warning 是观察面，不是判据面）。
	numericSoftWarnTokens = 8
	// numericSoftMaxTokens — 软核扫出的读数条数上限（防随正文长度膨胀；与 numericMaxSoftNotes 同量级）。
	numericSoftMaxTokens = 32
)

var (
	// numericExitWordAssert — A 类的同义形态：`退出码 0`（`rc=N` 由 claims_numbers.go 那一腿管）。
	numericExitWordAssert = regexp.MustCompile(`退出码\s*([0-9]+)`)
	// numericQuotedSymbol — 引文形态一：标识符 / 点号限定名（`DeriveClaims` / `pkg.Fn`）。
	numericQuotedSymbol = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
	// numericQuotedCall — 引文形态二：调用形态（`Fn(args)`）；只核括号前的名字。
	numericQuotedCall = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*\([^()]*\)$`)
	// numericSoftToken — 软核：`<数字> + 计数单位`（含字节 / 百分比，补齐第一版单位表）。
	numericSoftToken = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?\s*(?:字节|断言|件|条|行|个|项|处|次|张|份|步|门|秒|KB|MB|GB|%)`)
	// numericSoftLabelToken — 软核：`<计数标签> + 数字`（`陈旧 7` / `剩余 12`）。
	numericSoftLabelToken = regexp.MustCompile(`(?:陈旧|新增|命中|失败|覆盖|缺口|待办|剩余|未登记|已登记|已解)\s*[0-9]+`)
	// numericDigits — 从读数里取数字部分（回执里常写成 `136`、`136件`、`136 件` 三种形态）。
	numericDigits = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)
)

// VerifyNumericGranular — 分级核的增量入口（纯函数：不写盘、不改 traces、不改正文）。
//
// 硬核两腿都只**造主张**，裁决一律交既有 `VerifyClaims`；软核只分类取证状态。全空 ⇒ 调用方行为与接线前
// 逐字节一致（run.go 只把 `HardFailed` 并进派生那一批、把软核写进 warning 事件）。
func VerifyNumericGranular(content string, traces []Trace, root string) NumericGranularCheck {
	var out NumericGranularCheck
	if strings.TrimSpace(content) == "" {
		return out
	}
	claims := exitWordClaims(content, traces)
	quoted, lineReceipts := quotedLineClaims(content, root)
	claims = append(claims, quoted...)
	if len(claims) > 0 {
		receipts := MemberReceipts(traces)
		receipts = append(receipts, lineReceipts...)
		for _, r := range VerifyClaims(claims, receipts) {
			switch r.Verdict {
			case VerdictRetract, VerdictSendBack:
				out.HardFailed = append(out.HardFailed, r)
			}
		}
	}
	out.SoftBacked, out.SoftUnbacked = classifySoftTokens(content, MemberReceipts(traces))
	return out
}

// ── ① A 类同义形态 `退出码 N` ────────────────────────────────────────────────

// exitWordClaims — 逐句核 `退出码 N`（只认**紧跟**在一条 `zerg …` 命令之后的那个数）。
func exitWordClaims(content string, traces []Trace) []Claim {
	if !strings.Contains(content, "退出码") {
		return nil
	}
	cs := make([]claimCand, 0, len(traces))
	for _, t := range traces {
		if cmd := traceZergCommand(t); cmd != "" {
			cs = append(cs, claimCand{cmd: cmd, t: t})
		}
	}
	if len(cs) == 0 {
		return nil
	}
	var out []Claim
	seen := make(map[string]bool, 4)
	for _, raw := range splitClaimSentences(content) {
		s := strings.TrimSpace(raw)
		if s == "" || !strings.Contains(s, "退出码") {
			continue
		}
		c, ok := exitWordClaim(s, cs)
		if !ok || seen[c.ReceiptID+"|"+c.Quote] {
			continue
		}
		seen[c.ReceiptID+"|"+c.Quote] = true
		out = append(out, c)
	}
	return out
}

// exitWordClaim — 单句核 `退出码 N`：返回 (主张, 是否判红)。
//
// 真话（N == 真实 rc）/ 判不出来 / 命令本轮没跑过 / 同句另有 `rc=N`（那一腿已判，免得同一条出两票）
// ⇒ false（不判）。
func exitWordClaim(sentence string, cs []claimCand) (Claim, bool) {
	if strings.Contains(sentence, "rc=") {
		return Claim{}, false // `rc=N` 由 claims_numbers.go 那一腿管
	}
	named := zergCommandInSentence(sentence)
	if named == "" {
		return Claim{}, false
	}
	idx := strings.Index(sentence, named)
	if idx < 0 {
		return Claim{}, false
	}
	rest := sentence[idx+len(named):]
	m := numericExitWordAssert.FindStringSubmatchIndex(rest)
	if m == nil {
		return Claim{}, false
	}
	// 「紧跟」：命令与断言之间只许排版符 / 标点（夹了实词就不是「这条命令的读数」）。
	for _, r := range rest[:m[0]] {
		if !strings.ContainsRune(numericRCGapAllowed, r) {
			return Claim{}, false
		}
	}
	want, err := strconv.Atoi(rest[m[2]:m[3]])
	if err != nil {
		return Claim{}, false
	}
	ns, nn := normalizeForMatch(sentence), normalizeForMatch(named)
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
	return Claim{Text: sentence, ReceiptID: hitID, Quote: rest[m[0]:m[1]]}, true
}

// ── ② B 类 ③ 引文必须在所指的那一行 ──────────────────────────────────────────

// quotedLineClaims — 逐处 `件:行` 判「同句紧贴的引文是否真在那一行」。
//
// 返回 (主张集, 行证据回执集)：回执集只含**真被判红的那几行**的原文（不在行上才需要它去对片段）。
func quotedLineClaims(content, root string) ([]Claim, []Receipt) {
	if strings.TrimSpace(root) == "" || !strings.Contains(content, "`") {
		return nil, nil
	}
	var claims []Claim
	var receipts []Receipt
	seenClaim := make(map[string]bool, 4)
	seenReceipt := make(map[string]bool, 4)
	for _, hit := range scanFileLineRefs(content) {
		c, rec, ok := judgeLineQuote(hit, root)
		if !ok {
			continue
		}
		key := c.ReceiptID + "|" + c.Quote
		if seenClaim[key] {
			continue
		}
		seenClaim[key] = true
		claims = append(claims, c)
		if !seenReceipt[rec.ID] {
			seenReceipt[rec.ID] = true
			receipts = append(receipts, rec)
		}
	}
	return claims, receipts
}

// judgeLineQuote — 判一处引用上紧贴的引文：行上真有 ⇒ 不判；行上没有 ⇒ 造一条 send_back 主张。
//
// 件不存在 / 行号超界 / 读不了 / 同句非肯定引用 / 没有紧贴的标识符引文 ⇒ false（不判）。
func judgeLineQuote(hit fileRefHit, root string) (Claim, Receipt, bool) {
	if !numericRefIsAssertion(hit.sent, hit.ref) {
		return Claim{}, Receipt{}, false
	}
	i := strings.LastIndex(hit.ref, ":")
	if i <= 0 {
		return Claim{}, Receipt{}, false
	}
	path, lineStr := hit.ref[:i], hit.ref[i+1:]
	lineNo, err := strconv.Atoi(lineStr)
	if err != nil || lineNo < 1 {
		return Claim{}, Receipt{}, false
	}
	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") || filepath.IsAbs(path) {
		return Claim{}, Receipt{}, false
	}
	line, ok := readLineAt(filepath.Join(root, filepath.FromSlash(path)), lineNo)
	if !ok {
		return Claim{}, Receipt{}, false // 件不在 / 行超界 ⇒ 归 claims_numbers.go 的假引用那格
	}
	rec := Receipt{
		ID:     claimLineEvidencePrefix + path + ":" + strconv.Itoa(lineNo),
		Tool:   "file:line",
		Output: line,
	}
	for _, frag := range quotedCodeFragments(hit.sent, hit.ref) {
		target := symbolTarget(frag)
		if target == "" || strings.Contains(line, target) {
			continue // 行上真有这个符号 ⇒ 有据，不判
		}
		return Claim{Text: hit.ctx, ReceiptID: rec.ID, Quote: frag}, rec, true
	}
	return Claim{}, Receipt{}, false
}

// readLineAt — 读该件的第 lineNo 行（1 起）；件不在 / 行超界 / 读不了 ⇒ false。
//
// 行数口径与 `lineCountOf` 一致（`a\nb\n` = 2 行）：`lineNo > 换行符数` 即超界 —— 两者必须同口径，
// 否则同一条引用会被本文件与 claims_numbers.go 各判一票。
func readLineAt(path string, lineNo int) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	total := strings.Count(string(b), "\n")
	if len(b) > 0 && b[len(b)-1] != '\n' {
		total++
	}
	if lineNo < 1 || lineNo > total {
		return "", false
	}
	lines := strings.Split(string(b), "\n")
	if lineNo > len(lines) {
		return "", false
	}
	return lines[lineNo-1], true
}

// quotedCodeFragments — 从**同一句**里取「紧贴这处引用、且像符号名」的反引号引文（保序、去重）。
//
// 跳过：空串 / 含引用串本身的（引的就是这个路径）/ 不紧贴的 / 不像符号名的（命令 / 路径 / URL / 散文）。
func quotedCodeFragments(sentence, ref string) []string {
	var out []string
	seen := make(map[string]bool, 4)
	refAt := -1
	if ref != "" {
		refAt = strings.Index(sentence, ref)
	}
	for i := 0; i < len(sentence); {
		j := strings.IndexByte(sentence[i:], '`')
		if j < 0 {
			break
		}
		open := i + j
		k := strings.IndexByte(sentence[open+1:], '`')
		if k < 0 {
			break
		}
		closeAt := open + 1 + k
		span := strings.TrimSpace(sentence[open+1 : closeAt])
		i = closeAt + 1
		if span == "" || seen[span] {
			continue
		}
		if ref != "" && strings.Contains(span, ref) {
			continue // 引文就是这处引用本身
		}
		if !quotedSpanNearRef(sentence, open, closeAt, refAt, len(ref)) {
			continue // 远处的反引号不是对这一行的断言
		}
		if !isCitableSymbol(span) {
			continue
		}
		seen[span] = true
		out = append(out, span)
	}
	return out
}

// quotedSpanNearRef — 这处引文是否紧贴引用（间距 ≤ numericQuoteWindow **个字符**；相交/包住也算紧贴）。
//
// 间距按 rune 数算，不按字节：一句中文里「引用 与 引文 之间」常只隔十来个字，按字节算会把中文正文
// 整片判成「不紧贴」⇒ 该判的全漏（这里宁可按字算，语义上「紧贴」本来就是字符距离）。
func quotedSpanNearRef(sentence string, open, closeAt, refAt, refLen int) bool {
	if refAt < 0 {
		return false
	}
	switch {
	case closeAt < refAt:
		return utf8.RuneCountInString(sentence[closeAt+1:refAt]) <= numericQuoteWindow
	case open > refAt+refLen:
		return utf8.RuneCountInString(sentence[refAt+refLen:open]) <= numericQuoteWindow
	}
	return true
}

// isCitableSymbol — 这处引文是不是「可核的符号名」。
//
// 只认标识符形态（`DeriveClaims` / `pkg.Fn` / `Fn(args)`）：
// 命令（`zerg …`，总带空白）、路径（含 `/`）、URL、文件名（带已知后缀）、含中文或空白的散文一律不认 ——
// 那些是行文强调或命令本身，拿它们去核「这一行」必误杀真话。
func isCitableSymbol(span string) bool {
	if span == "" || utf8.RuneCountInString(span) > numericQuotedMaxRunes {
		return false
	}
	if strings.Contains(span, "://") || strings.Contains(span, "/") {
		return false
	}
	for _, suf := range []string{".go", ".md", ".json", ".yaml", ".yml", ".sh", ".tsv"} {
		if strings.HasSuffix(span, suf) {
			return false
		}
	}
	if numericQuotedSymbol.MatchString(span) {
		return true
	}
	return numericQuotedCall.MatchString(span)
}

// symbolTarget — 引文里真正要出现的那一段：调用形态只核括号前的名字（`Fn(a, b)` ⇒ `Fn`）。
func symbolTarget(span string) string {
	if i := strings.IndexByte(span, '('); i > 0 {
		return span[:i]
	}
	return span
}

// ── ③ C 类：软核取证分级 ────────────────────────────────────────────────────

// classifySoftTokens — 把正文里的其它计数分成「回执已取证」与「未取证（进 warning）」两档。
func classifySoftTokens(content string, receipts []Receipt) (backed, unbacked []string) {
	for _, tok := range softTokens(content) {
		if softTokenBacked(tok, receipts) {
			backed = append(backed, tok)
			continue
		}
		unbacked = append(unbacked, tok)
	}
	return backed, unbacked
}

// softTokens — 扫出软核读数（保序、去重、封顶）。
func softTokens(content string) []string {
	var out []string
	seen := make(map[string]bool, 8)
	add := func(s string) {
		if s == "" || seen[s] || len(out) >= numericSoftMaxTokens {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, m := range numericSoftToken.FindAllString(content, -1) {
		add(m)
	}
	for _, m := range numericSoftLabelToken.FindAllString(content, -1) {
		add(m)
	}
	return out
}

// softTokenBacked — 这个读数在本轮回执原文里找得到吗（「已取证」= 有据可对）。
//
// 两级匹配：① 归一化后的整串（`136 件` 对 `136 件` / `136\n件`）；② 数字本体 + 数字边界
// （回执写成 `136` 而正文写 `136 件` 也算 —— 单位是正文的叙述，不是第二个事实）。
func softTokenBacked(tok string, receipts []Receipt) bool {
	num := numericDigits.FindString(tok)
	nt := normalizeForMatch(tok)
	for _, r := range receipts {
		out := normalizeForMatch(r.Output)
		if nt != "" && strings.Contains(out, nt) {
			return true
		}
		if num != "" && numberBoundaryContains(out, num) {
			return true
		}
	}
	return false
}

// numberBoundaryContains — `num` 是否以**独立数字**形态出现在 `s` 里（`136` 不匹配 `1360` / `3.14`）。
func numberBoundaryContains(s, num string) bool {
	if num == "" {
		return false
	}
	for off := 0; off <= len(s); {
		i := strings.Index(s[off:], num)
		if i < 0 {
			return false
		}
		i += off
		end := i + len(num)
		leftOK := i == 0 || (!isASCIIDigit(s[i-1]) && s[i-1] != '.')
		rightOK := end >= len(s) || (!isASCIIDigit(s[end]) && s[end] != '.')
		if leftOK && rightOK {
			return true
		}
		off = i + 1
	}
	return false
}

// isASCIIDigit — ASCII 数字（不走 unicode：这里判的是回执原文里的数字边界）。
func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }

// numericSoftWarning — `output_claim_soft` 事件的载荷（低基数：计数 + 少量读数串）。
type numericSoftWarning struct {
	Backed         int      `json:"soft_backed"`
	Unbacked       int      `json:"soft_unbacked"`
	BackedTokens   []string `json:"backed_tokens,omitempty"`
	UnbackedTokens []string `json:"unbacked_tokens,omitempty"`
}

// WarningPayload — 软核 warning 的事件载荷（**没有未取证读数 ⇒ 空串** ⇒ 调用方不发事件）。
//
// 只在「回执里对不上」时报警：已取证的读数不必惊动人，真话成稿因此**一个事件都不发**
// （低基数 · 也免得每轮对话的机器面多出一格）。载荷带两类计数与少量读数串（都是 `数字+单位`，无自由文本）。
func (c NumericGranularCheck) WarningPayload() string {
	if len(c.SoftUnbacked) == 0 {
		return ""
	}
	w := numericSoftWarning{
		Backed:         len(c.SoftBacked),
		Unbacked:       len(c.SoftUnbacked),
		BackedTokens:   capTokens(c.SoftBacked),
		UnbackedTokens: capTokens(c.SoftUnbacked),
	}
	b, err := json.Marshal(w)
	if err != nil {
		return ""
	}
	return string(b)
}

// capTokens — 载荷里每类最多带 numericSoftWarnTokens 个读数。
func capTokens(in []string) []string {
	if len(in) <= numericSoftWarnTokens {
		return in
	}
	return in[:numericSoftWarnTokens]
}
