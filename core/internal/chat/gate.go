// gate.go — 首 token 闸的生效值（丙 的判据需要它；2026-09-17 补最后一根线）。
//
// 口径与网关一致（甲：按卵可配、**只放宽不收窄**）：档案实测值（事实）优先 ⇒ 思考型离线下限 ⇒ 无则 0（未知，不猜）。
// ⚠ 已知重复：网关侧 gateway.thinkingModelFirstTokenMin / ProfileFirstTokenSec 是同一套口径。
//
//	本版为"不引新包、不造环"先在 chat 侧独立实现（纯读文件、无副作用）；
//	承接项：v2.5.11 抽到中性包（internal/gate）单一真源 —— 见 变更-v2.5.11 待办。
package chat

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// eggProfileDir — 卵档案目录（ZERG_EGG_PROFILE_DIR 可覆盖，测试用）。
func eggProfileDir() string {
	if d := os.Getenv("ZERG_EGG_PROFILE_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zerg", "egg-profiles")
}

// EffectiveGateSec — 该卵本轮生效的首 token 闸（秒）；0 = 未知（不编造）。
func EffectiveGateSec(model string) int {
	eff := thinkingFloorSec(model)
	if v, ok := profileFirstTokenSec(model); ok && v > eff {
		eff = v
	}
	return eff
}

// thinkingFloorSec — 思考型/大参数模型的保守下限（与网关侧同表）。
func thinkingFloorSec(model string) int {
	m := strings.ToLower(model)
	for _, k := range []string{"qwen3.8-27b", "qwen3.8", "nemotron", "deepseek", "qwen3"} {
		if strings.Contains(m, k) {
			return 600
		}
	}
	return 0
}

// profileFirstTokenSec — 读档案里的 first_token_sec（事实）；无档案/无字段/坏值 ⇒ false。
func profileFirstTokenSec(model string) (int, bool) {
	dir := eggProfileDir()
	if dir == "" || model == "" {
		return 0, false
	}
	b, err := os.ReadFile(filepath.Join(dir, model+".yaml"))
	if err != nil {
		ents, derr := os.ReadDir(dir)
		if derr != nil {
			return 0, false
		}
		for _, e := range ents {
			if strings.EqualFold(strings.TrimSuffix(e.Name(), ".yaml"), model) {
				b, err = os.ReadFile(filepath.Join(dir, e.Name()))
				break
			}
		}
		if err != nil || b == nil {
			return 0, false
		}
	}
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") || !strings.HasPrefix(t, "first_token_sec:") {
			continue
		}
		if n, perr := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(t, "first_token_sec:"))); perr == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

// RoundTimeoutFor — 单轮推理限时（原写死 120s ✗ ⇒ 实测每批 4 次 upstream_timeout，真凶即此）。
// 口径与首 token 闸一致（甲：按卵可配、**只放宽不收窄**）：取 max(120s, 首 token 闸 × 2, 思考型下限 600s)。
// 为什么 ≥ 首 token 闸：闸是"等到第一个字"的时间 ⇒ 单轮限时若比它短，等于闸还没到就被掐（自相矛盾）✓
func RoundTimeoutFor(model string) time.Duration {
	base := 120 * time.Second
	g := EffectiveGateSec(model)
	if g > 0 {
		if d := time.Duration(g) * 2 * time.Second; d > base {
			base = d
		}
	}
	if thinkingFloorSec(model) > 0 && base < 600*time.Second {
		base = 600 * time.Second
	}
	return base
}

// WallClockFor — 整任务墙钟（原写死 600s ✗ 思考型长任务会被整体掐断）。
// 只放宽：至少 600s；思考型给 1800s（30 分钟）—— 且**始终 ≥ 单轮限时**。
func WallClockFor(model string) time.Duration {
	base := 600 * time.Second
	if thinkingFloorSec(model) > 0 {
		base = 1800 * time.Second
	}
	if rt := RoundTimeoutFor(model); rt > base {
		base = rt
	}
	return base
}
