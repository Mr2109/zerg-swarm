// idle_selfsleep.go —— P6（R6）：受管模型的「空闲自退」透传。
//
// 设计依据：《设计-子端服务切换与基线服务声明》§10 R6 + §11 P6 ——
//
//	受管侧的空闲自退可作为轻量替代：让**引擎自己**在空闲后收敛，而不是子端硬杀。
//
// ⚠️ 真机核实（2026-09-14，X3 上 `llama-server --help`，不靠记忆）：
//
//	-to, --timeout N              只是 server **read/write timeout**（默认 3600），**不是**空闲退出；
//	--sleep-idle-seconds SECONDS  "number of seconds of idleness after which the server will sleep" ✅ 这才是真开关。
//
// 设计稿 R6 原文写的 `--timeout` **是错的**——拿它当空闲自退会是一个静默的错误开关
// （既不报错、也永不会按空闲退出）。此处以真机为准，文档已回填（代码为准）。
//
// 生效范围：只对 llama 家族；ds4-server 没有这个开关（传了会启动失败，宁可不动）。
// 未配置 / 配 0 ⇒ 完全不介入，行为与旧版逐字一致。
package backend

import (
	"os"
	"strconv"
	"strings"
)

// EnvIdleSelfSleepS 引擎空闲自退的秒数。未设或 0 ⇒ 不透传（默认关闭，绝不改变既有行为）。
const EnvIdleSelfSleepS = "ZERG_IDLE_SLEEP_S"

// 引擎自睡的下限：太小的值会让模型在两次请求之间反复入睡/唤醒，反而更慢。
const idleSleepFloorS = 60

// idleSelfSleepSeconds 读配置（纯函数式读取，便于测试用 t.Setenv 注入）。
func idleSelfSleepSeconds() int {
	v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(EnvIdleSelfSleepS)))
	if err != nil || v <= 0 {
		return 0
	}
	if v < idleSleepFloorS {
		return idleSleepFloorS
	}
	return v
}

// applyIdleSelfSleep 追加引擎空闲自睡参数（P6）。
//
// 幂等：参数里已有 `--sleep-idle-seconds` 就不再追加（尊重显式配置）。
// 不支持的后端（ds4-server）原样返回 —— 宁可不动，也不要塞一个它不认的开关把启动搞失败。
func applyIdleSelfSleep(args []string, backend string) []string {
	if strings.Contains(strings.ToLower(backend), "ds4") {
		return args
	}
	for _, a := range args {
		if a == "--sleep-idle-seconds" {
			return args
		}
	}
	secs := idleSelfSleepSeconds()
	if secs <= 0 {
		return args
	}
	out := make([]string, 0, len(args)+2)
	out = append(out, args...)
	out = append(out, "--sleep-idle-seconds", strconv.Itoa(secs))
	return out
}
