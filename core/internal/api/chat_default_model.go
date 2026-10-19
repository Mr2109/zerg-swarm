package api

import (
	"os"
	"strings"
)

// defaultChatModel 对话默认模型。
//
// Mr2109 2026-10-19 定：默认改用 Nemotron-3.5-Super-VL-120B-A12B（1M 上下文，
// 实测 17.77 t/s · GTT 峰值 77.96 GiB · 装载 22–30 s）；此前写死 Qwen3.8-Flash-Next。
//
// 为什么改成读环境变量：换默认模型不该再动代码——写死一处就要重编一次、还得
// 同批过门74（制品↔源码）。这里给 ZERG_CHAT_DEFAULT_MODEL 一个口子，空值回落内置名。
func defaultChatModel() string {
	if v := strings.TrimSpace(os.Getenv("ZERG_CHAT_DEFAULT_MODEL")); v != "" {
		return v
	}
	return "Nemotron-3.5-Super-VL-120B-A12B"
}
