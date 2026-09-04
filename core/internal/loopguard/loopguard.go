// Package loopguard — 工具循环守卫（2026-09-05 从 chat 包抽出公共化——CA/对话共用同一防循环内核）
// SHA-256 指纹滑动窗口: 相同调用重复 3 次 / A-B 交替卡死 → 分级引导（换招→收尾）
// Mr2109调: 重复不是失败就停——是引导模型换招式/换工具继续解决问题
package loopguard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Config — 守卫配置
type Config struct {
	WindowSize       int // 指纹滑动窗口（默认 8）
	MaxGuideAttempts int // 引导升级阈值（默认 3）
}

// Guard — 循环守卫
type Guard struct {
	fingerprints []string
	guideCount   int
	cfg          Config
}

// New — 创建守卫（默认参数: WindowSize=8 MaxGuideAttempts=3）
func New(cfg Config) *Guard {
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = 8
	}
	if cfg.MaxGuideAttempts <= 0 {
		cfg.MaxGuideAttempts = 3
	}
	return &Guard{cfg: cfg}
}

// Record — 记录一次工具调用（工具名+参数 → SHA-256 8 字节指纹）
// 指纹不含结果——同工具同参数不同结果=正常推进（轮询/验证场景）
func (g *Guard) Record(name string, args map[string]any) string {
	raw := name
	if b, err := json.Marshal(args); err == nil {
		raw += "|" + string(b)
	}
	h := sha256.Sum256([]byte(raw))
	fp := hex.EncodeToString(h[:8])
	g.fingerprints = append(g.fingerprints, fp)
	if len(g.fingerprints) > g.cfg.WindowSize {
		g.fingerprints = g.fingerprints[len(g.fingerprints)-g.cfg.WindowSize:]
	}
	return fp
}

// Detect — 检测循环模式（模式1: 最近 3 次相同 / 模式2: ABAB 交替）
func (g *Guard) Detect() (bool, string) {
	n := len(g.fingerprints)
	if n < 3 {
		return false, ""
	}
	last := g.fingerprints[n-1]
	if g.fingerprints[n-2] == last && g.fingerprints[n-3] == last {
		return true, "相同调用重复 3 次"
	}
	if n >= 4 {
		a, b := g.fingerprints[n-4], g.fingerprints[n-3]
		if a != b && g.fingerprints[n-2] == a && g.fingerprints[n-1] == b {
			return true, "两个调用交替卡死（A/B 循环）"
		}
	}
	return false, ""
}

// BuildGuide — 分级引导消息（第 1 次: 换策略——第 2 次: 升级收尾）
// 返回: (引导消息, 是否升级收尾)
func (g *Guard) BuildGuide(reason string, availableTools []string) (string, bool) {
	g.guideCount++
	switch {
	case g.guideCount == 1:
		return fmt.Sprintf("（注意：检测到 %s——你已经重复同样的操作。请换一种方法或换一个工具来推进任务——不要重复刚才的调用）", reason), false
	default:
		return "（已经尽力尝试了多种方式，请把到目前为止获得的信息整理成最终回答。如果信息不足或没找到答案，就直接说明没找到——绝不编造。）", true
	}
}

// GuideCount — 已引导次数
func (g *Guard) GuideCount() int {
	return g.guideCount
}
