// chat_agent_loop.go — v2.5.7 P4-38 AgentLoop v3（引导式防循环——本地小模型特化）
// 核心（Mr2109调）: 重复不是失败就停——是引导模型换招式/换工具继续解决问题
// 参考: Hermes issue #481 Tool-Call Loop Guard（SHA-256 指纹 + 滑动窗口 + 先警告后升级）
//       26 本地模型测试（压力下放弃工具是主失败模式）——上下文轻量 + 系统提示强化

package chat

// 2026-09-05: LoopGuard 抽公共包 internal/loopguard（CA/对话共用同一防循环内核）——
// 本文件保留类型别名转发（兼容现有调用点）+ CompactToolResults（chat 特有）

import (
	"fmt"

	"github.com/Mr2109/zerg-swarm/core/internal/loopguard"
)

// LoopGuard/LoopConfig — 公共包别名（渐进迁移——现有调用点不改）
type LoopGuard = loopguard.Guard

// LoopConfig — 守卫配置别名
type LoopConfig = loopguard.Config

// DefaultLoopConfig — 默认配置
func DefaultLoopConfig() LoopConfig {
	return LoopConfig{}
}

// NewLoopGuard — 创建守卫（转发公共包）
func NewLoopGuard(cfg LoopConfig) *LoopGuard {
	return loopguard.New(cfg)
}

// CompactToolResults — P4-38 T5: 上下文轻量化（loop 中旧工具结果压缩）
// 本地小模型上下文金贵——超过 N 轮的旧工具结果 → 一行摘要（防上下文膨胀放弃工具）
// P4-50 改进: keepRecent 按"工具结果条数"计（非消息条数）——最近 N 个工具结果保留完整——
// 防关键结果（port_services 全量清单）被压成摘要 → 模型反复核验（"被省略"感知——B 场景 151s 根因之一）
// 返回: 压缩后的消息切片（新切片——不修改原）
func CompactToolResults(msgs []map[string]any, keepRecent int) []map[string]any {
	if keepRecent <= 0 {
		keepRecent = 3
	}
	// 统计 tool 消息总数
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
			// 剩余 tool 数 < keepRecent → 最近的关键结果——保留完整
			if toolCount <= keepRecent {
				out = append(out, m)
				toolCount--
				continue
			}
			toolCount--
			content, _ := m["content"].(string)
			if len([]rune(content)) > 100 {
				// 旧工具结果 → 一行摘要（保留 call_id 关联——模型知道可重调工具拿全量）
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
