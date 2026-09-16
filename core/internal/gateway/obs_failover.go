// obs_failover.go — 丁（超时留痕）：把「回落给谁、为什么」写进观测文件，不再只活在日志里。
//
// 为什么要写：实测一轮里 `upstream_timeout` 连发 28 次、每次都回落 ⇒ 但观测面看不到「回落」这件事，
// 只能靠翻 /tmp/zerg-core.log。落盘后：一条 failover 记录 = 谁→谁、原因原文、时间。
//
// 独立性：本文件不 import chat 包（避免与 chat→gateway 的方向形成环），自己按同一份 JSONL 格式追加一行；
// 写不进去不影响转发（best-effort，与 chat 侧 obsWrite 同一纪律）。
package gateway

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// obsFailoverPath — 与 chat 观测同一份文件（<state>/chat_obs.jsonl；ZERG_STATE_DIR 可覆盖）。
func obsFailoverPath() string {
	dir := os.Getenv("ZERG_STATE_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".zerg", "state")
	}
	return filepath.Join(dir, "chat_obs.jsonl")
}

// ObsFailover — 记一条回落（kind=failover）。best-effort：失败只返回错误，不影响转发。
func ObsFailover(from, to, reason, model string) {
	p := obsFailoverPath()
	if p == "" {
		return
	}
	rec := map[string]any{
		"ts":              time.Now().Format(time.RFC3339),
		"kind":            "failover",
		"failover_from":   from,
		"failover_to":     to,
		"failover_reason": reason,
	}
	if model != "" {
		rec["model"] = model
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}

// ObsQueued — 乙-2：记一条「子端侧排队时长」（kind=queued）。
// 数据来自子端响应头 X-Zerg-Queued-Ms（子端自报「接到 → 开始干活」的耗时）。
// 语义纪律：排队 ≠ 推理 ⇒ 这条记录**只用于观测**，不参与任何时限判定（业界口径：waiting 是容量信号）。
func ObsQueued(host, model string, queuedMS int64) {
	p := obsFailoverPath()
	if p == "" || queuedMS < 0 {
		return
	}
	rec := map[string]any{
		"ts":        time.Now().Format(time.RFC3339),
		"kind":      "queued",
		"host":      host,
		"queued_ms": queuedMS,
	}
	if model != "" {
		rec["model"] = model
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}
