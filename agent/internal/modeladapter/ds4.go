package modeladapter

import (
	"fmt"
	"os"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// DS4 DeepSeek-V4-Flash 适配器（ds4-server 后端，不走 llama-server）。
// 特性：ds4-server 原生支持 /v1/responses（RESPPROTO），非思考模型（ThinkingFormat: none）。
//
// 参数归属（P5，设计-子端沙箱化-20260914 §9.2，写死）：
//   - --ssd-streaming / --ssd-streaming-preload-experts / --ssd-streaming-cache-experts
//     / --kv-disk-dir / --kv-disk-space-mb 全部是 **ds4（DwarfStar）的参数**，不是 llama.cpp 的；
//   - llama.cpp 的对应物是 --moe-stream*（机制同源、page cache 策略相反）。
//
// ⇒ 本适配器是这些参数的**唯一发出点**；llama 系适配器（generic/example-35b-v2/qwen/k2/gemma）
// 一律不得发出（测试钉住：TestParameterOwnership_DS4ArgsNeverInLlamaLine）。
type DS4 struct{}

func (a *DS4) Name() string { return "deepseek" }

func (a *DS4) BuildArgs(entry *registry.ModelEntry, port int) []string {
	args := []string{
		"--model", entry.File,
		"--port", fmt.Sprintf("%d", port),
		"--host", "127.0.0.1",
	}
	// ssd 流式加载
	if ssd, ok := entry.Custom["ssd"].(bool); ok && ssd {
		args = append(args, "--ssd-streaming")
	}
	if cache, ok := entry.Custom["ssd_streaming_cache_experts"].(string); ok && cache != "" {
		args = append(args, "--ssd-streaming-cache-experts", cache)
	}
	// 预热专家数（P5，设计 §9.4 / F6）：声明了才发（值必须来自实测档案——P5 验收③的
	// 512/1024/2048 扫描表；无声明 = 无档案 ⇒ **不发**，「无档案不预热」，
	// §8.4 标定铁律：凡数字必实测、禁估值、禁硬编码）。
	if p := entry.SsdStreamingPreloadExperts; p > 0 {
		args = append(args, "--ssd-streaming-preload-experts", fmt.Sprintf("%d", p))
	}
	// KV 盘（P5，设计 §9.4 / §9.7）：默认关闭；声明了才发 --kv-disk-dir + --kv-disk-space-mb。
	// 目录按卵分目录（§9.7④）：显式声明优先，缺省 ~/.zerg/kvdisk/<卵名>/——落「跨孵化保留」侧，
	// 收卵 / GC 不碰它（§6.6）；上界 space_mb 必填（§9.7①，校验在 ValidateEggDeclaration）。
	if kv := entry.KVDisk; kv != nil {
		if dir := kvDiskDir(entry, kv); dir != "" {
			args = append(args, "--kv-disk-dir", dir)
		}
		if kv.SpaceMB > 0 {
			args = append(args, "--kv-disk-space-mb", fmt.Sprintf("%d", kv.SpaceMB))
		}
	}
	// ctx 透传（2026-09-14）：ds4-server 的上下文窗口由 --ctx 决定；
	// 实测依据：已归档至 Zerg-归档/v2.5.10/01-设计/设计-ds4适配器接视觉.md
	if ctx := customInt(entry, "ctx"); ctx > 0 {
		args = append(args, "--ctx", fmt.Sprintf("%d", ctx))
	}
	// 视觉编码器（2026-09-14）：ds4 的 --vision 与 llama-server 的 mmproj 语义不同，故单列键。
	if vision, ok := entry.Custom["vision"].(string); ok && vision != "" {
		args = append(args, "--vision", vision)
	}
	return args
}

// kvDiskDir 解析 KV 盘目录（按卵分目录，设计 §9.7④）。
// 优先级：① 卵声明 kv_disk.dir（显式覆盖，支持 ~ 前缀）② 缺省 ~/.zerg/kvdisk/<卵名>/。
// 两条都落不了地（既无显式目录又无卵名）⇒ 返回空串、不发 --kv-disk-dir（不许猜路径）。
func kvDiskDir(entry *registry.ModelEntry, kv *registry.KVDiskDecl) string {
	if kv.Dir != "" {
		return expandHome(kv.Dir)
	}
	if name := entry.EggName(); name != "" {
		base := "~/.zerg/kvdisk"
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			base = home + "/.zerg/kvdisk"
		}
		return base + "/" + name
	}
	return ""
}

// customInt 读取 inline 字段里的整数（YAML 可能给 int/int64/float64/string，逐类型兜）。
func customInt(entry *registry.ModelEntry, key string) int {
	switch v := entry.Custom[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n := 0
		for _, r := range v {
			if r < '0' || r > '9' {
				return 0
			}
			n = n*10 + int(r-'0')
		}
		return n
	}
	return 0
}

func (a *DS4) ToolCallStyle() string { return "json" }

func (a *DS4) NeedsToolReminder() bool { return false }

func (a *DS4) ReminderPrompt() string { return "" }

func (a *DS4) ThinkingFormat() string { return "none" }

func (a *DS4) ContextWindow() int { return 524288 }
