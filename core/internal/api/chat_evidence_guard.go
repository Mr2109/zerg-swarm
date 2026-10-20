package api

// chat_evidence_guard.go —— 「声称实测但本轮无工具调用」标注（A07）。
//
// 病：模型会在正文里写「我实测过了 / 跑了一遍 / 验证通过 / verified」，但这一轮**一次工具都没调** ——
// 没有执行痕迹的「实测」是不可核的断言，调用方却容易当成已核证据接受。
//
// 与退化标注（chat_degenerate_guard.go）同一条纪律：**只标不拦** —— 不改结果、不影响退码，
// 只把「这条断言没有可核证据」写进正文，让调用方自己复跑。
//
// 判据刻意保守：**本轮工具调用次数 == 0** 且正文出现实测类措辞才标；还额外避开否定语境
// （「未实测」「unverified」不算声称）。

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	// evidenceClaimMarker — 本标注的可识别前缀（与退化标注前缀互不相同，谁先谁后都能认出来）。
	evidenceClaimMarker = "⚠ **声称实测但本轮无任何工具调用**"
	// degenerateGuardMarker — 退化标注的前缀（见 chat_degenerate_guard.go），用于剥离已追加的标注块。
	degenerateGuardMarker = "⚠ **疑似退化输出**"
)

// evidenceClaimTerms — 触发标注的实测类措辞。中文与英文都收；「实测过」是「实测」的超集，故只留「实测」。
var evidenceClaimTerms = []string{
	"实测", "跑了一遍", "跑过", "验证通过",
	"verified", "measured",
}

// evidenceNegationRunes — 紧邻其前的否定字 ⇒ 不是声称实测（保守起见不标）。
var evidenceNegationRunes = map[rune]bool{'未': true, '没': true, '不': true, '无': true, '非': true}

// stripGuardAnnotations —— 剥掉正文尾部已追加的守卫标注块（本文件的 + 退化标注的）。
//
// 为什么需要：两条标注都是**追加**到正文尾的，且标注文案里本身带实测类措辞
// （退化标注里有「跑过」）。判词/测长前先剥掉已追加的标注，两条才**互不吞没**、且先标谁后标谁结果一致。
// 只认已知前缀，正文里模型自己写的「⚠ **…」不会被误剥。
func stripGuardAnnotations(content string) string {
	cut := -1
	for _, marker := range []string{evidenceClaimMarker, degenerateGuardMarker} {
		if i := strings.Index(content, "\n\n"+marker); i >= 0 && (cut < 0 || i < cut) {
			cut = i
		}
	}
	if cut < 0 {
		return content
	}
	return content[:cut]
}

// annotateUnverifiedClaim —— 零工具调用却声称实测 ⇒ 在正文尾追加「无可核证据」标记。
//
// 返回新正文；不满足条件时**原样返回**（逐字不改，避免污染既有金标）。
func annotateUnverifiedClaim(content string, toolCalls int) string {
	if toolCalls != 0 {
		return content
	}
	body := stripGuardAnnotations(content)
	if strings.TrimSpace(body) == "" {
		return content
	}
	term, ok := findClaimTerm(body)
	if !ok {
		return content
	}
	return content + fmt.Sprintf(
		"\n\n%s：正文提到「%s」，但这一轮一次工具都没调 —— 真跑过就一定有执行痕迹。"+
			"请以自己复跑的 rc / sha 为准（这不是说正文一定错，是它**没有可核的证据**）。\n",
		evidenceClaimMarker, term)
}

// findClaimTerm —— 取正文里最早出现、且未被否定的实测类措辞（同位置取更长的那个）。
func findClaimTerm(body string) (string, bool) {
	bestIdx, bestTerm := -1, ""
	for _, term := range evidenceClaimTerms {
		for off := 0; off <= len(body); {
			i := indexFold(body[off:], term)
			if i < 0 {
				break
			}
			at := off + i
			if !negatedAt(body, at) {
				if bestIdx < 0 || at < bestIdx || (at == bestIdx && len(term) > len(bestTerm)) {
					bestIdx, bestTerm = at, term
				}
				break // 该词的首个非否定出现即够用
			}
			off = at + len(term)
		}
	}
	return bestTerm, bestIdx >= 0
}

// negatedAt —— at 处的词是否处于否定语境（左侧紧邻「未/没/不/无/非」，或英文的 un-/not /never）。
func negatedAt(body string, at int) bool {
	prefix := body[:at]
	if r, size := utf8.DecodeLastRuneInString(prefix); size > 0 && evidenceNegationRunes[r] {
		return true
	}
	lower := strings.ToLower(prefix)
	if len(lower) != len(prefix) { // 小写化改变字节长度 ⇒ 不做英文前缀判断，宁可不否定
		return false
	}
	if strings.HasSuffix(lower, "un") {
		return true
	}
	for _, lead := range []string{"not ", "never ", "no "} {
		if strings.HasSuffix(lower, lead) {
			return true
		}
	}
	return false
}

// indexFold —— 大小写不敏感的子串下标；大小写折叠改变字节长度时退回逐字面匹配（保证下标可回指原串）。
func indexFold(haystack, needle string) int {
	lowerHay, lowerNeedle := strings.ToLower(haystack), strings.ToLower(needle)
	if len(lowerHay) != len(haystack) {
		return strings.Index(haystack, needle)
	}
	return strings.Index(lowerHay, lowerNeedle)
}
