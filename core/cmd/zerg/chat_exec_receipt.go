package main

// chat_exec_receipt.go —— 对话回答的**执行凭据核对**（缺口 GAP-20261018-25 第二半 · P0）。
//
// 病灶（实测 · 已亲核）：本地卵在 chat 会话里编造了完整假回执 —— 提案号、rc=0、
// after_sha256、hunk 数四处全假，格式与真回执一模一样。第一半已在每次工具执行时
// 发 `x-<8hex>` 凭据（见 internal/chat 的 zergNewExecID），并在输出里要求模型引用
// 数字时带上它。本文件做第二半：**回答里出现数字类断言却一个凭据都没提 ⇒ 标未取证**。
//
// 口径（刻意保守，只标不拦）：
//   - 只认「回答里出现过 `x-` + 8 位 hex」为「带了凭据」；不提任何凭据却报 rc/sha/hunk/字节
//     ⇒ 高置信度未取证（真跑过就一定有凭据可带）。
//   - 只做尾注，不改正文、不改退码（正文是模型成稿，改动会污染审计）。
//   - 纯文本类回答（没提任何数字）一字不加。

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	execIDPattern = regexp.MustCompile(`x-[0-9a-f]{8}`)
	// ★ 每条都必须**带值**才算断言：只提到 `after_sha256` 这个词（例如「我不知道 after_sha256」）
	// 不是报数字 —— 首版只匹配词本身，实测把模型的诚实回答误标成未取证（假阳性）。
	claimPatterns = []*regexp.Regexp{
		regexp.MustCompile(`rc\s*=\s*-?\d+`),
		regexp.MustCompile(`(?i)after_sha256\s*[=:]\s*[0-9a-f]{8,}`),
		regexp.MustCompile(`(?i)sha256\s*[=:]\s*[0-9a-f]{8,}`),
		regexp.MustCompile(`hunk[s]?\s*[=:]?\s*\d+`),
		regexp.MustCompile(`\d+\s*处[^。；\n]{0,6}(改|替换|命中|变动)`),
		regexp.MustCompile(`\d+\s*字节`),
	}
)

// annotateUnbackedClaims —— 回答里报数字却没有任何执行凭据 ⇒ 末尾追加一行未取证标记。
func annotateUnbackedClaims(answer string) string {
	if strings.TrimSpace(answer) == "" {
		return answer
	}
	if execIDPattern.MatchString(answer) {
		return answer // 带了至少一枚凭据 ⇒ 按已取证处理（细粒度比对留给服务端）
	}
	var hits []string
	for _, re := range claimPatterns {
		if m := re.FindString(answer); m != "" {
			hits = append(hits, m)
		}
	}
	if len(hits) == 0 {
		return answer
	}
	uniq := make([]string, 0, len(hits))
	seen := map[string]bool{}
	for _, h := range hits {
		if !seen[h] {
			seen[h] = true
			uniq = append(uniq, h)
		}
	}
	return answer + fmt.Sprintf(
		"\n\n⚠ **未取证**：本轮回答里报了 %s，但**没有带任何执行凭据**（形如 `x-1a2b3c4d`）。"+
			"真跑过工具就一定有凭据可带 ⇒ 这些数字**可能是编造的**，请以自己复跑的 rc / sha 为准（缺口 GAP-20261018-25）。\n",
		strings.Join(uniq, "、"))
}
