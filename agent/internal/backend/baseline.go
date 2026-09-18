// baseline.go —— 模型级身份探针 + 内存扣减教训（P4 退场清理后的残余保留面）。
//
// 设计依据：设计-子端沙箱化.md §10.1 baseline.go 行——
// 基线服务声明/检查/占用扣减那一整套（声明端口 env、BaselineService、
// serviceClassFrom、baselineServicesLocked、baselineOccupiedGb、机型配额/预留两 env）
// 随「卵之外无引擎」（§1.3）**整体退场**（附录 C·C8）。
//
// 保留两样：
//  1. probeIdentity / parseModelIdentity——模型级就绪探针仍需要读 /v1/models 的身份，
//     且 llama 系与 OpenAI 系两种形态都要认；
//  2. deductOccupancyFromAvail 的**教训**（统一内存上别双重扣减）——原样保留在代码里，
//     给 §8 双闸门的接线当判例。
package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// parseModelIdentity 从 /v1/models 的响应体里取模型标识（L1：端口活 ≠ 身份对）。
//
// 两种形态都认：
//   - llama.cpp 系：{"models":[{"name":"/data/models/…gguf"}]}        ⇒ 取 name（权重路径）
//   - OpenAI 系（ds4）：{"data":[{"id":"deepseek-v4-flash",…}]}          ⇒ 取 id
func parseModelIdentity(body []byte) string {
	var shaped struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &shaped); err != nil {
		return ""
	}
	for _, m := range shaped.Models {
		if m.Name != "" {
			return m.Name
		}
		if m.Model != "" {
			return m.Model
		}
	}
	for _, d := range shaped.Data {
		if d.ID != "" {
			return d.ID
		}
	}
	return ""
}

// probeIdentity 对端口发一条只读 /v1/models，返回模型标识（失败返回空串）。
func probeIdentity(port int, timeout time.Duration) string {
	client := &http.Client{Timeout: timeout}
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/models", port)
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}
	return parseModelIdentity(body)
}

// deductOccupancyFromAvail 判定"驻留项的占用"是否要从**系统可用内存**里再扣一次。
//
// 默认 **false**，理由（2026-09-14 X3 真机实测，教训原样保留）：
//
//	MemTotal 122.2 GB、MemAvailable 61.0 GB，而两条外部推理服务实测占用 56.68 GB ——
//	61.0 正是"122.2 减去（含这 56.68 在内的）已用"的结果 ⇒ **采样口径已经把 GTT/权重算进去了**。
//	再扣一次就是**双重扣减**：61.0 − 56.68 = 4.3 GB，
//	结果任何真实模型（≥5GB）都被判"内存不足"⇒ 机器实际上装不了东西。
//
// 什么时候该设 true（ZERG_DEDUCT_OCCUPANCY_FROM_AVAIL=1）：
//
//	采样**看不见**显存的平台（离散 GPU：权重在独立 VRAM 里，MemAvailable 里没有它）。
//	统一内存（AMD APU / Apple 统一内存）一律用默认 false。
func deductOccupancyFromAvail() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(EnvDeductOccupancy)))
	return v == "1" || v == "true" || v == "yes"
}

// EnvDeductOccupancy 见 deductOccupancyFromAvail。
const EnvDeductOccupancy = "ZERG_DEDUCT_OCCUPANCY_FROM_AVAIL"
