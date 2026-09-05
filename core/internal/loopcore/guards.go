package loopcore

// guards.go — 上下文轻量化 + 指纹守卫（从 chat 包迁入——唯一实现）

import (
	"fmt"

	"zerg/core/internal/loopguard"
)

// CompactToolResults — 旧工具结果压缩（keepRecent: 最近 N 个保留完整——其余一行摘要）
// 本地小模型上下文金贵——防膨胀到放弃工具（P4-38 T5 实证）
func CompactToolResults(msgs []map[string]any, keepRecent int) []map[string]any {
	if keepRecent <= 0 {
		keepRecent = 3
	}
	toolCount := 0
	for _, m := range msgs {
		if role, _ := m["role"].(string); role == "tool" {
			toolCount++
		}
	}
	if toolCount <= keepRecent || len(msgs) <= 8 {
		return msgs
	}
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		role, _ := m["role"].(string)
		if role == "tool" {
			if toolCount <= keepRecent {
				out = append(out, m)
				toolCount--
				continue
			}
			toolCount--
			content, _ := m["content"].(string)
			if len([]rune(content)) > 100 {
				short := string([]rune(content)[:80]) + "…"
				out = append(out, map[string]any{
					"role":         "tool",
					"tool_call_id": m["tool_call_id"],
					"content":      fmt.Sprintf("[工具结果已压缩——如需完整结果请重新调用该工具] %s", short),
				})
				continue
			}
		}
		out = append(out, m)
	}
	return out
}

// loopguardNew — 指纹守卫（公共包封装）
func loopguardNew() *loopguard.Guard {
	return loopguard.New(loopguard.Config{})
}
