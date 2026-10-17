// output_claims_derive.go — GAP-20261018-01（P0）：把正文里「声称成功」的句子保守派生成引用主张。
//
// 要治的病（本单前提，现读核实过）：run.go 顶部那道防伪回执 defer 早已完整接线
// （`outputClaims()` 记忆化调 `VerifyOutputClaims` ✗ 命中失败 ⇒ `ApplyOutputClaimFacts` ＋ `ChooseTerminal`
// ＋ `AppendTerminalNote`），但 `Deps.OutputClaims` **恒空**
// （`grep -rn 'OutputClaims:' core --include='*.go' | grep -v _test` ⇒ 0 命中：从没人填）
// ⇒ 四态裁决永远拿不到输入 ⇒「模型声称成功、工具回执却失败 / 那条命令根本没跑过」从来不会判红。
// 本文件就是那个缺口的那一环：**从正文派生**主张，判定仍全部交既有 `VerifyClaims`（一行不改）。
//
// 口径（**保守**：宁可漏判，不可误杀正常交付）：
//
//	① 只抽**声称成功**的句子（标记闭集见 `claimSuccessMarkers`），且该句**点名了一条 `zerg …` 命令**；
//	   没有点名命令的泛称（「全部通过」「已修复」）不抽 —— 没有回执可对，抽了只能判 revise（缺引用），白增噪声。
//	② 否定句整句不抽（「未通过」「没成功」「失败」…）：它们不是「声称成功」。
//	③ 逐条去本轮 `traces` 里按**命令串**找那次真实调用（归一化 = 去首尾空白 + 折叠内部空白，见
//	   `normalizeForMatch`；不改动命令内容）：
//	   · 找到、且该回执 rc ≠ 0 ⇒ `Claim{ReceiptID: 该回执ID, Quote: 该句}`
//	     ⇒ `VerifyClaims` 判 **send_back**（「片段与回执原文不符」——那条回执里根本没有这句成功宣言）；
//	   · 找不到（模型报了个本轮根本没跑过的命令）⇒ `Claim{ReceiptID: "nonexistent:"+命令串, Quote: 该句}`
//	     ⇒ 判 **retract**（「引用了不存在的回执」）；
//	   · 找到且 rc=0 ⇒ **不生成 Claim**（真话不判）。
//	     rc 判不出来（回执里既没有 `rc=N` 也没有 `[exit_code] N`）⇒ 也不生成（不猜）。
//	④ 正文里没有「声称成功且点名命令」的句子 ⇒ 返回 nil ⇒ `VerifyOutputClaims` 的 `Declared=false`
//	   ⇒ 产出口行为与接线前**逐字节一致**（零回归；既有 output_claims_test 的用例钉着这一条）。
//
// 为什么不判别的形态（本批**不**扩张判据面）：只有 {retract, send_back} 会被 `VerifyOutputClaims` 收作
// 「命中失败」，而这两态**只能**由「有回执ID」或「ID 不存在」产出；`revise`（缺引用）按既有口径不算失败
// （见 output_claims.go 文件头 ②）⇒ 泛称主张抽出来也改不了任何行为，故不抽。
package loopcore

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ClaimNonexistentPrefix — 「这条命令本轮**根本没跑过**」的合成回执号前缀。
//
// 只做标记用：真实回执 ID 来自 `Trace.CallID`（工具生成的短号），不可能以此前缀开头；
// 而 `VerifyClaims` 只看「这个 ID 在不在回执集里」⇒ 合成号必然判 retract（假回执）。
const ClaimNonexistentPrefix = "nonexistent:"

var (
	// claimSuccessMarkers — 声称成功类标记（闭集）。只认这几条；
	// 「完成」「好了」「搞定」这类不算 —— 本单只治「假成功」，判宽了会误杀正常交付。
	claimSuccessMarkers = []string{"rc=0", "已解", "已修复", "已通过", "全部通过", "成功"}
	// claimNegationMarkers — 否定 / 失败词：句内出现任一条 ⇒ **整句不抽**（否定句不是「声称成功」）。
	claimNegationMarkers = []string{"未", "没", "失败", "无法", "不通过", "不成功", "不能"}

	// claimZergRC — `zerg` 工具回执的首行形态（chat_tool_zergcmd.go 逐字 `fmt.Fprintf(&b, "rc=%d\n", rc)`）。
	claimZergRC = regexp.MustCompile(`(?m)^rc=(\d+)\s*$`)
	// claimBashExitRC — `bash` 工具回执的元数据尾注形态（agent/bash_v101.go 逐字 `[exit_code] %d`）。
	claimBashExitRC = regexp.MustCompile(`\[exit_code\]\s+(\d+)`)
)

// commandStopPunct — token 里出现即视为「命令行到此结束」的字符（句读 / 引号 / 管道 / 重定向）。
// 反引号也在此：正文里常用反引号包命令，Trim 掉后 token 才是干净的命令词。
const commandStopPunct = "，。、；：（）()【】「」『』“”‘’\"'`|><&"

// splitClaimSentences — 把正文切成句子（换行与中英句读）。
//
// 只按**明确的句子边界**切：不按 ASCII 句点切（`xxx.md` / `--format json` 里都有点），
// 否则一条命令会被切成两半、命令串再也对不上回执。
func splitClaimSentences(content string) []string {
	return strings.FieldsFunc(content, func(r rune) bool {
		switch r {
		case '\n', '\r', '。', '！', '？', '；', ';', '!', '?':
			return true
		}
		return false
	})
}

// claimsSuccess — 该句是否在**声称成功**：含闭集标记 ∧ 不含任何否定/失败词。
func claimsSuccess(sentence string) bool {
	has := false
	for _, m := range claimSuccessMarkers {
		if strings.Contains(sentence, m) {
			has = true
			break
		}
	}
	if !has {
		return false
	}
	for _, m := range claimNegationMarkers {
		if strings.Contains(sentence, m) {
			return false
		}
	}
	return true
}

// isCommandToken — 该 token 像不像命令词（命令名 / 旗标 / 路径）。
//
// 不像就说明命令行已经结束、后面是叙述：含中文的、含句读/引号/管道/重定向的、以及
// 「结论等式」形态（`rc=0` —— 含 `=` 且不是 `-` 开头的旗标）都不是命令词。
func isCommandToken(tok string) bool {
	if tok == "" {
		return false
	}
	for _, r := range tok {
		if unicode.Is(unicode.Han, r) || unicode.IsSpace(r) {
			return false
		}
		if strings.ContainsRune(commandStopPunct, r) {
			return false
		}
	}
	if strings.Contains(tok, "=") && !strings.HasPrefix(tok, "-") {
		return false
	}
	return true
}

// zergCommandInSentence — 取该句**点名**的第一条 `zerg …` 命令串（没有 ⇒ 空串）。
//
// 从正文里第一个独立的 `zerg` 词起逐词收，遇「不像命令词的 token」即止 ⇒ 得到的是**命令本体**，
// 不含后面的叙述（例：`zerg doc xxx --format json` 在「…rc=0 且扫了 136 件」里会在 `rc=0` 前止住）。
// 不用 `strings.Fields` 从头找 `zerg` 词：中文正文里命令常紧跟在标点后（「结论：zerg …」），
// 整 token 是 `结论：zerg` ⇒ 按 token 相等判会漏掉（本单初版就栽在这里，实测派生恒 0 条）。
// 同一条命令串口径也用于读**回执侧**（trace 的 `command` 参数），两侧才能对上。
func zergCommandInSentence(s string) string {
	low := strings.ToLower(s)
	const kw = "zerg"
	for off := 0; off < len(low); {
		i := strings.Index(low[off:], kw)
		if i < 0 {
			return ""
		}
		i += off
		off = i + len(kw)
		// 词边界：前一个 rune 不许是 ASCII 字母/数字（`zzzerg` / `2zerg` 不是命令名；
		// 中文/空白/标点都可以是句子的一部分 —— 中文后直接写命令是常态）。
		if i > 0 {
			if r, _ := utf8.DecodeLastRuneInString(s[:i]); r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
				continue
			}
		}
		rest := s[i+len(kw):]
		if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
			continue // 「zerg」后面必须紧跟空白才是命令形态（zerg_xxx / zerg.foo 不是）
		}
		out := []string{kw}
		for _, n := range strings.Fields(rest) {
			n = strings.Trim(n, commandStopPunct)
			if n == "" || !isCommandToken(n) {
				break
			}
			out = append(out, n)
		}
		if len(out) >= 2 {
			return strings.Join(out, " ")
		}
	}
	return ""
}

// traceZergCommand — 从一条工具轨迹里取出它**那次调用**点名的 `zerg …` 命令串。
//
// 参数口径：`zerg` 工具与 `bash` 工具的命令都在 `{"command": "…"}` 里（chat_tool_zergcmd.go、
// agent/exec.go 同此键名）⇒ 只认这一个键，键不在就是「这次调用没点名 zerg 命令」。
func traceZergCommand(t Trace) string {
	if strings.TrimSpace(t.Args) == "" {
		return ""
	}
	var args map[string]any
	if json.Unmarshal([]byte(t.Args), &args) != nil {
		return ""
	}
	raw, _ := args["command"].(string)
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return zergCommandInSentence(raw)
}

// traceExitCode — 该回执的退出码（`ok=false` = 判不出来，**不猜**）。
//
// `Error` 非空 ⇒ 执行侧自己报了错（该次调用不是成功）⇒ 当非零处理。
// 否则读回执原文里的机器可读锚点：`zerg` 工具的 `rc=N` / `bash` 工具的 `[exit_code] N`。
func traceExitCode(t Trace) (int, bool) {
	if strings.TrimSpace(t.Error) != "" {
		return 1, true
	}
	if m := claimZergRC.FindStringSubmatch(t.Result); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n, true
		}
	}
	if m := claimBashExitRC.FindStringSubmatch(t.Result); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n, true
		}
	}
	return 0, false
}

// traceCommandHit — 该回执是否就是正文点名的那条命令。
//
// 两种形态都算命中（都要求**词边界**，免得 `zerg doc` 撞上 `zerg docker`）：
//
//	· 正文里出现了这条回执的完整命令串（正文照抄了命令）；
//	· 正文点名的命令是这条回执命令串的**前缀**（正文把 `--fast` 这类尾巴省了）。
func traceCommandHit(normSentence, normNamed, normTraceCmd string) bool {
	if normTraceCmd == "" {
		return false
	}
	if strings.Contains(normSentence, normTraceCmd) {
		return true
	}
	return normTraceCmd == normNamed || strings.HasPrefix(normTraceCmd, normNamed+" ")
}

// claimCand — 一条可匹配的回执候选（命令串 + 轨迹本体）。
type claimCand struct {
	cmd string
	t   Trace
}

// DeriveClaims — 从**定稿正文**里保守派生「声称成功」类主张（返回 nil = 一条都没有 ⇒ 不判）。
//
// 纯函数：不读写任何全局状态、不改 traces。判定一律交既有 `VerifyClaims`（本函数只造三条入参字段）。
func DeriveClaims(content string, traces []Trace) []Claim {
	if strings.TrimSpace(content) == "" || !strings.Contains(content, "zerg") {
		return nil // 连命令名都没提 ⇒ 不可能有「点名命令的声称成功句」（快路也是零回归的保证）
	}
	cs := make([]claimCand, 0, len(traces))
	for _, t := range traces {
		if cmd := traceZergCommand(t); cmd != "" {
			cs = append(cs, claimCand{cmd: cmd, t: t})
		}
	}
	seen := make(map[string]bool, 8)
	var out []Claim
	for _, raw := range splitClaimSentences(content) {
		s := strings.TrimSpace(raw)
		if s == "" || seen[s] || !claimsSuccess(s) {
			continue
		}
		named := zergCommandInSentence(s)
		if named == "" {
			continue // 没点名命令的泛称 ⇒ 不抽（没有回执可对）
		}
		seen[s] = true
		if c, ok := deriveOneClaim(s, named, cs); ok {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// deriveOneClaim — 单句派生：命中失败才生成主张，真话/判不出来一律不生成。
func deriveOneClaim(sentence, named string, cs []claimCand) (Claim, bool) {
	ns := normalizeForMatch(sentence)
	nn := normalizeForMatch(named)
	matched := -1
	seenOK := false
	for i, c := range cs {
		if !traceCommandHit(ns, nn, normalizeForMatch(c.cmd)) {
			continue
		}
		if matched < 0 {
			matched = i
		}
		// rc=0（或 rc 判不出来 ⇒ 不猜）⇒ 这句话有据 ⇒ 不判
		if rc, ok := traceExitCode(c.t); !ok || rc == 0 {
			seenOK = true
		}
	}
	if matched < 0 {
		// 本轮**没跑过**这条命令 ⇒ 合成一个不可能存在的回执号 ⇒ VerifyClaims 判 retract（假回执）
		return Claim{Text: sentence, ReceiptID: ClaimNonexistentPrefix + named, Quote: sentence}, true
	}
	if seenOK {
		return Claim{}, false // 真话不判
	}
	c := cs[matched]
	if strings.TrimSpace(c.t.CallID) == "" {
		// 无 ID 的轨迹进不了回执集（MemberReceipts 不收）⇒ 引用不到 ⇒ 判不了，不猜。
		return Claim{}, false
	}
	// 回执存在但 rc≠0 ⇒ 那句「成功」宣言不可能出现在该回执原文里 ⇒ VerifyClaims 判 send_back（无据）
	return Claim{Text: sentence, ReceiptID: c.t.CallID, Quote: sentence}, true
}
