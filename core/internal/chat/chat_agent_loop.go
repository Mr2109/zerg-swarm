// chat_agent_loop.go — v2.5.7 P4-38 AgentLoop v3（引导式防循环——本地小模型特化）
// 核心（Mr2109调）: 重复不是失败就停——是引导模型换招式/换工具继续解决问题
// 参考: Hermes issue #481 Tool-Call Loop Guard（SHA-256 指纹 + 滑动窗口 + 先警告后升级）
//       26 本地模型测试（压力下放弃工具是主失败模式）——上下文轻量 + 系统提示强化

package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// LoopExit — 循环退出原因
type LoopExit int

const (
	ExitNatural      LoopExit = iota // 模型无工具调用——最终回复
	ExitBudgetRounds                 // 30 轮保护
	ExitBudgetWallClock              // 600s 墙钟
	ExitNoProgress                   // 引导 3 次仍无进展——升级收尾
	ExitCancelled                    // 用户断开
)

func (e LoopExit) String() string {
	switch e {
	case ExitNatural:
		return "自然终止（模型无工具调用）"
	case ExitBudgetRounds:
		return "轮数上限（30）"
	case ExitBudgetWallClock:
		return "墙钟上限（600s）"
	case ExitNoProgress:
		return "多次引导仍无进展——升级收尾"
	case ExitCancelled:
		return "用户断开"
	}
	return "未知"
}

// LoopConfig — 循环保护参数（Claude "dozens of turns"——30 轮 + 600s 墙钟 + 引导 3 次）
type LoopConfig struct {
	MaxRounds        int     // 30
	MaxWallSeconds   float64 // 600
	MaxGuideAttempts int     // 引导次数 3（先警告后升级——Hermes 481）
	WindowSize       int     // 指纹滑动窗口 8
}

// DefaultLoopConfig — 默认配置
func DefaultLoopConfig() LoopConfig {
	return LoopConfig{MaxRounds: 30, MaxWallSeconds: 600, MaxGuideAttempts: 3, WindowSize: 8}
}

// LoopGuard — 循环守卫（SHA-256 指纹滑动窗口——检测相同调用重复 + A/B 交替卡死）
// 反应: 分级引导（换策略→列工具→升级收尾）——不是失败就停（Mr2109要求）
type LoopGuard struct {
	fingerprints []string // 滑动窗口（工具名+参数哈希）
	guideCount   int      // 已引导次数
	cfg          LoopConfig
}

// NewLoopGuard — 创建守卫
func NewLoopGuard(cfg LoopConfig) *LoopGuard {
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = 8
	}
	if cfg.MaxGuideAttempts <= 0 {
		cfg.MaxGuideAttempts = 3
	}
	return &LoopGuard{cfg: cfg}
}

// Record — 记录一次工具调用（工具名 + 参数 JSON 序列化 → SHA-256）
// P4-38: 指纹含结果哈希？——不——结果在执行后才知道——指纹只含工具名+参数
//（结果变化不算循环——同工具同参数不同结果 = 正常推进——轮询/验证场景）
func (g *LoopGuard) Record(name string, args map[string]any) string {
	raw := name
	if b, err := json.Marshal(args); err == nil {
		raw += "|" + string(b)
	}
	h := sha256.Sum256([]byte(raw))
	fp := hex.EncodeToString(h[:8]) // 8 字节足够
	g.fingerprints = append(g.fingerprints, fp)
	if len(g.fingerprints) > g.cfg.WindowSize {
		g.fingerprints = g.fingerprints[len(g.fingerprints)-g.cfg.WindowSize:]
	}
	return fp
}

// Detect — 检测循环模式（返回: 是否检测到——原因）
// 模式1: 相同指纹 ≥3 次（窗口内）
// 模式2: A/B 交替（ABAB 出现 ≥2 对）
func (g *LoopGuard) Detect() (bool, string) {
	n := len(g.fingerprints)
	if n < 3 {
		return false, ""
	}
	// 模式1: 最近 3 次相同
	last := g.fingerprints[n-1]
	if n >= 3 && g.fingerprints[n-2] == last && g.fingerprints[n-3] == last {
		return true, "相同调用重复 3 次"
	}
	// 模式2: A/B 交替（检查最后 4 项 ABAB 或 6 项 ABABAB）
	if n >= 4 {
		a, b := g.fingerprints[n-4], g.fingerprints[n-3]
		if a != b && g.fingerprints[n-2] == a && g.fingerprints[n-1] == b {
			return true, "两个调用交替卡死（A/B 循环）"
		}
	}
	return false, ""
}

// BuildGuide — 分级引导消息（第 1 次: 换策略——第 2 次: 列工具——第 3 次: 升级收尾）
// 返回: (引导消息, 是否升级收尾)
func (g *LoopGuard) BuildGuide(reason string, availableTools []string) (string, bool) {
	g.guideCount++
	switch {
	case g.guideCount == 1:
		return fmt.Sprintf("（注意：检测到 %s——你已经重复同样的操作。请换一种方法或换一个工具来推进任务——不要重复刚才的调用）", reason), false
	case g.guideCount == 2:
		// 第 2 次仍重复——直接升级（同指纹重复浪费轮次——小模型空参数循环无意义）
		return "（已经尽力尝试了多种方式，请把到目前为止获得的信息整理成最终回答。如果信息不足或没找到答案，就直接说明没找到——绝不编造。）", true
	default:
		return "（已经尽力尝试了多种方式，请把到目前为止获得的信息整理成最终回答。如果信息不足或没找到答案，就直接说明没找到——绝不编造。）", true
	}
}

// GuideCount — 已引导次数
func (g *LoopGuard) GuideCount() int {
	return g.guideCount
}

// ShouldUpgrade — 是否达到升级阈值（引导 ≥ MaxGuideAttempts）
func (g *LoopGuard) ShouldUpgrade() bool {
	return g.guideCount >= g.cfg.MaxGuideAttempts
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
