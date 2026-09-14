// functional_probe.go —— M3：功能预检（"加载成功 ≠ 可用"）。
//
// 设计依据：《设计-子端服务切换与基线服务声明》§11 M3（今天 502 的直接成因）：
//
//	准入判据是内存估算，而**引擎自己还有守卫**——`rocm prefill failed` 正是在"装好了"之后才发生。
//	处置：①装载后加**功能预检**（一条 max_tokens=1 的极小请求）；②不通过则降档重试一次；
//	③仍不通过 ⇒ 明确报错 + 归还，**绝不对外声称已就绪**。
//
// 本文件实现 ①与③；②（降档重试）见 downgradeArgs（纯函数，已单测，钩子在下一笔接）。
// 设计上刻意放在**无平台标签**的文件里：探针与参数改写都不依赖 Linux，
// 放在 _linux.go 会让开发机（darwin）测不到（教训见 skill §8.8）。
package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// EnvProbeTimeout 功能预检的超时（秒）。默认 120s：
// 1M 上下文的首字很慢（实测 GLM 预热约 5s、DS4 长提示更久），给足但绝不无限等。
const EnvProbeTimeout = "ZERG_PROBE_TIMEOUT_S"

func probeTimeout() time.Duration {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(EnvProbeTimeout))); err == nil && v > 0 {
		return time.Duration(v) * time.Second
	}
	return 120 * time.Second
}

// probeInference 功能预检：向已就绪的端口发一条 max_tokens=1 的极小请求，验证**真的能出字**。
//
// 为什么不能只看 /health 与 /v1/models：那两个只证明"HTTP 活着"，
// 而今天的事故恰恰是"模型装好了、端口也应答、但一生成就 rocm prefill failed"。
// 所以判据必须是**一次真实生成**：HTTP 200 **且** 至少带回一个 choice。
func probeInference(port int, model string, timeout time.Duration) error {
	payload := map[string]interface{}{
		"model":       model,
		"max_tokens":  1,
		"temperature": 0,
		"messages":    []map[string]string{{"role": "user", "content": "ping"}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("构造预检请求失败: %w", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", port)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("构造预检请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("预检请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("预检 HTTP %d: %s", resp.StatusCode, truncateForLog(string(raw), 200))
	}
	var parsed struct {
		Choices []json.RawMessage `json:"choices"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("预检响应不是合法 JSON: %s", truncateForLog(string(raw), 200))
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return fmt.Errorf("引擎自报错误: %s", truncateForLog(parsed.Error.Message, 200))
	}
	if len(parsed.Choices) == 0 {
		// 这一条正是"端口活着但出不了字"的形态——必须当失败，不能当就绪。
		return fmt.Errorf("预检无任何 choice（端口应活着但不能出字）: %s", truncateForLog(string(raw), 200))
	}
	return nil
}

// downgradeArgs 把启动参数里的上下文与专家缓存各降一档（M3 的 ②）。
//
// 保守原则：**只认已知形状**，认不出的一律不动；若一个都没降到，返回 changed=false
// —— 调用方据此判断"降档无意义"，不要拿同一套参数白试一遍。
//
//	--ctx 1048576            → 524288        （下限 4096，绝不低于）
//	--ssd-streaming-cache-experts 16GB → 8GB （下限 1GB）
//	--ssd-streaming-preload-experts 512  → 256（下限 64）
func downgradeArgs(args []string) ([]string, bool) {
	out := make([]string, len(args))
	copy(out, args)
	changed := false

	for i := 0; i < len(out); i++ {
		switch out[i] {
		case "--ctx":
			if i+1 < len(out) {
				if v, err := strconv.Atoi(out[i+1]); err == nil && v > 4096 {
					out[i+1] = strconv.Itoa(maxInt(v/2, 4096))
					changed = true
				}
			}
		case "--ssd-streaming-cache-experts":
			if i+1 < len(out) {
				if n, ok := halveSizeToken(out[i+1]); ok {
					out[i+1] = n
					changed = true
				}
			}
		case "--ssd-streaming-preload-experts":
			if i+1 < len(out) {
				if v, err := strconv.Atoi(out[i+1]); err == nil && v > 64 {
					out[i+1] = strconv.Itoa(maxInt(v/2, 64))
					changed = true
				}
			}
		}
	}
	return out, changed
}

// halveSizeToken 把 "16GB" 这类尺寸令牌减半，保持单位；认不出则不返回。
func halveSizeToken(tok string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(tok))
	for _, unit := range []string{"GB", "MB"} {
		if strings.HasSuffix(upper, unit) {
			numStr := strings.TrimSuffix(upper, unit)
			v, err := strconv.Atoi(numStr)
			if err != nil || v <= 1 {
				return "", false
			}
			halved := v / 2
			if halved < 1 {
				halved = 1
			}
			return fmt.Sprintf("%d%s", halved, unit), true
		}
	}
	return "", false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
