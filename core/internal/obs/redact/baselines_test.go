package redact

import (
	"regexp"
	"strings"
)

// ── 基线对照：两个「合理的第一版实现」───────────────────────────────────────
//
// 它们**只活在测试里**（生产路径没有这两个函数）。存在的唯一理由：探针有效性靠
// 「基线必须泄漏」来证明 —— 一条谁都不命中的探针等于没有探针（设计附件 §四）。

// baselineKeyDenylist：方法 A —— 按键名遮蔽，从不看值。最便宜，但只覆盖「想得到的键名」。
var keyDeny = map[string]bool{
	"password": true, "api_key": true, "token": true, "authorization": true,
	"cookie": true, "secret": true, "path": true, "file": true,
}

func baselineKeyDenylist(v any, key string) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, vv := range t {
			if keyDeny[strings.ToLower(k)] {
				m[k] = PlaceholderRedacted
				continue
			}
			m[k] = baselineKeyDenylist(vv, k)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i := range t {
			s[i] = baselineKeyDenylist(t[i], key)
		}
		return s
	}
	return v
}

var (
	reAsciiPath  = regexp.MustCompile(`/(?:Users|home)/[A-Za-z0-9._-]+`)
	reAsciiEmail = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	reAsciiIPv4  = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	reAsciiTok   = regexp.MustCompile(`\bsk-[A-Za-z0-9\-_]{16,}\b`)
)

// asciiLine 是「纯 ASCII 正则」基线的值管线。**故意**保留它的两个真实缺陷：
//   - ReplaceAllString（${HOME} 被当命名捕获组 ⇒ 路径被删而非打码，D14）；
//   - 只认 ASCII 形态：`\/`、`\u002f`、全角、零宽、percent、base64、hf_ 一律漏。
func asciiLine(s string) string {
	s = reAsciiTok.ReplaceAllString(s, PlaceholderRedacted)
	s = reAsciiPath.ReplaceAllString(s, PlaceholderPath)
	s = reAsciiEmail.ReplaceAllString(s, PlaceholderRedacted)
	s = reAsciiIPv4.ReplaceAllString(s, PlaceholderRedacted)
	return s
}

// baselineASCIITopLevel：方法 B —— 值级正则，但只处理**顶层字符串**、不折叠、不解码。
func baselineASCIITopLevel(ev map[string]any) map[string]any {
	out := make(map[string]any, len(ev))
	for k, v := range ev {
		if s, ok := v.(string); ok {
			out[k] = asciiLine(s)
			continue
		}
		out[k] = v
	}
	return out
}
