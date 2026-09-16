// receipt.go — 第①步：引用命中校验（设计稿 v1.2 §第1步）
//
// 要解决的问题（实测）：执行模型会"宣布完成"甚至自己招认"我从未真正读过那些文件……我编造了很多内容"。
// 行业做法（见设计稿调研）：不用词表 ✗，而是把主张**落到证据**上逐条判（ClaimVer / SciLens / GroundCheck）——
// GroundCheck 同构：抽出可验证主张 ⇒ 逐条落到真实证据 ⇒ 每条给带引用的裁决 + 动作 keep/revise/retract/send_back。
//
// 本文件只做**纯函数**部分（好测、零副作用）：
//
//	输入：一组"主张 + 引用（回执ID + 片段）"、以及本轮**可用的工具回执原文**
//	输出：每条主张的四态裁决（keep / revise / retract / send_back）+ 理由
//
// 判据（可判定，不靠猜）：片段必须能在**指定回执**的原文里被找到（去空白后包含）。
//
//	· 命中 ⇒ keep（有据）
//	· 引用指向不存在的回执 ⇒ retract（假回执）
//	· 回执存在但片段对不上 ⇒ send_back（无据 ⇒ 判为**永久错误**，不重试，改走"拆小步"）
//	· 主张无引用 ⇒ revise（缺引用 ⇒ 要求补）
package loopcore

import (
	"strings"
	"unicode"
)

// ClaimVerdict — 四态裁决（对齐 GroundCheck：keep / revise / retract / send_back）
type ClaimVerdict string

const (
	VerdictKeep     ClaimVerdict = "keep"      // 有据：片段命中所指回执原文
	VerdictRevise   ClaimVerdict = "revise"    // 缺引用：要求补（不是无据）
	VerdictRetract  ClaimVerdict = "retract"   // 假回执：引用了不存在的回执
	VerdictSendBack ClaimVerdict = "send_back" // 无据：片段对不上（**永久错误** ⇒ 不重试，走拆小步）
)

// Receipt — 一条工具回执（本轮真实发生过的工具调用）
type Receipt struct {
	ID     string // 回执 ID（如 "17" 或 "read#3"）
	Tool   string // 工具名
	Output string // 该次调用返回的原文（用于命中校验）
}

// Claim — 一条主张及其引用
type Claim struct {
	Text      string // 主张原文（如"我已改了 exec.go 的 TrimSpace"）
	ReceiptID string // 引用的回执 ID（空 = 未引用）
	Quote     string // 引用片段（应来自该回执原文）
}

// ClaimVerdictResult — 单条裁决结果
type ClaimVerdictResult struct {
	Claim   Claim
	Verdict ClaimVerdict
	Reason  string
}

// normalizeForMatch — 归一化：去掉首尾空白、折叠内部空白（换行/多空格视作一个空格）。
// 目的：不因排版差异误判"无据"（宁可放过排版，也不放过"编"——但排版差异属于合法差异）。
func normalizeForMatch(s string) string {
	return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, s)), " ")
}

// VerifyClaims — 逐条裁决；返回顺序与入参一致（便于逐条打印与落观测）。
func VerifyClaims(claims []Claim, receipts []Receipt) []ClaimVerdictResult {
	byID := make(map[string]Receipt, len(receipts))
	for _, r := range receipts {
		byID[r.ID] = r
	}
	out := make([]ClaimVerdictResult, 0, len(claims))
	for _, c := range claims {
		res := ClaimVerdictResult{Claim: c}
		switch {
		case strings.TrimSpace(c.ReceiptID) == "":
			res.Verdict = VerdictRevise
			res.Reason = "缺引用：请给出依据（回执ID + 片段）"
		case func() bool { _, ok := byID[c.ReceiptID]; return !ok }():
			res.Verdict = VerdictRetract
			res.Reason = "引用了不存在的回执 → 撤回该主张"
		default:
			rec := byID[c.ReceiptID]
			// 片段为空 ⇒ 视为未引用（补引用），不算"无据"，避免误杀
			if strings.TrimSpace(c.Quote) == "" {
				res.Verdict = VerdictRevise
				res.Reason = "有回执但无片段：请贴出原文摘录（≤200 字）"
				break
			}
			if strings.Contains(normalizeForMatch(rec.Output), normalizeForMatch(c.Quote)) {
				res.Verdict = VerdictKeep
				res.Reason = "片段命中回执原文（有据）"
			} else {
				res.Verdict = VerdictSendBack
				res.Reason = "片段与回执原文不符（无据；属永久错误 ⇒ 不重试，改为拆小步）"
			}
		}
		out = append(out, res)
	}
	return out
}

// CountByVerdict — 汇总（落观测用：keep/revise/retract/send_back 各几条）
func CountByVerdict(results []ClaimVerdictResult) map[string]int {
	m := map[string]int{}
	for _, r := range results {
		m[string(r.Verdict)]++
	}
	return m
}
