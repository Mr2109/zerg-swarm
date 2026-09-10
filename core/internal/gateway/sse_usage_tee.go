// sse_usage_tee.go — 丙批 N4 补齐（2026-09-10）：流式响应旁路采样，提取缓存计量
//
// 背景：对话路径全走**流式**（SSE），而网关侧的缓存计量原先只在非流式响应上解析
// （流式直通适配器，不破坏 SSE）→ 命中率统计对真实对话永远是 0 样本。
//
// 实测（本机 llama-server 8082，stream=true）：**末块**带
//   "timings":{"cache_n":0,"prompt_n":12,...}   ← cache_n=命中(复用 KV)，prompt_n=本次评估(未命中)
// 做法：把响应体包一层旁路 tee，只保留**尾部窗口**（16KB），响应结束后从中取最后一条含
// timings/usage 的 `data:` JSON，交给既有 recordPrefixCache 解析（该解析器已兼容
// openai cached_tokens / llama.cpp timings / DeepSeek prompt_cache_hit_tokens 三种形态）。

package gateway

import (
	"encoding/json"
	"io"
	"strings"
	"sync"
)

const sseTeeLimit = 16 << 10 // 尾窗 16KB（足够容纳末块 timings）

type sseUsageTee struct {
	r     io.ReadCloser
	mu    sync.Mutex
	tail  []byte
	limit int
}

// newSSEUsageTee — 包装流式响应体（透传读取 + 尾部留痕）
func newSSEUsageTee(r io.ReadCloser) *sseUsageTee {
	return &sseUsageTee{r: r, limit: sseTeeLimit}
}

func (t *sseUsageTee) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 {
		t.mu.Lock()
		t.tail = append(t.tail, p[:n]...)
		if len(t.tail) > t.limit {
			t.tail = t.tail[len(t.tail)-t.limit:]
		}
		t.mu.Unlock()
	}
	return n, err
}

func (t *sseUsageTee) Close() error { return t.r.Close() }

// UsageJSON — 从尾窗里取最后一条含缓存计量的 data: JSON（无则 nil）
// 只认最后一条：流式响应里 usage/timings 出现在末块（llama.cpp 行为，已实测）。
func (t *sseUsageTee) UsageJSON() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.tail) == 0 {
		return nil
	}
	lines := strings.Split(string(t.tail), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(l, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(l, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		if !strings.Contains(payload, `"timings"`) && !strings.Contains(payload, `"usage"`) {
			continue
		}
		if json.Valid([]byte(payload)) {
			return []byte(payload)
		}
	}
	return nil
}
