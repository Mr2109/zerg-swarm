package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// egg_warmup.go — 缺口 ⑤：冷卵自动孵（2026-09-19 Mr2109 批「开工」）。
//
// 现象：卵空闲 `idle_unload_s`（DS4 卵=600s）卸载后，主控路由不会把它孵回来 ⇒ 请求直接吃
// 子端 `503 {"error":"model not available"}` ⇒ `model_fail_count` 累加 ⇒ 熔断。
//
// 设计（第一版"转发前预探测"被既有 trace 用例当场证伪 —— 它对每次转发都多打一跳，
// 见 Zerg-内部文档/项目文档/v2.5.10/修复报告-子端五条-20260919.md 的 ⑨ 实测定论）：
//   **只在目标机真的回了 5xx「model not available」之后**才孵，然后**原地重发一次**。
// 零侵入热路径：正常 2xx 响应一个字节都不多读。

// ---- 重发标记（context 携带；只重发一次 ⇒ 不掩盖真故障）----

type eggRetryKey struct{}

func withEggRetry(ctx context.Context) context.Context {
	return context.WithValue(ctx, eggRetryKey{}, true)
}

func eggRetryDone(ctx context.Context) bool {
	v, _ := ctx.Value(eggRetryKey{}).(bool)
	return v
}

const (
	eggLoadTimeout = 180 * time.Second // 孵蛋等待（DS4 实测装载 6.3s，留足余量）
	eggPeekLimit   = 4 << 10           // 只读这么多去认错（响应体不能全读——要还原给下游）
)

// eggErrorClient 孵蛋专用客户端（不共用长连接：孵化期间连接会被占住）
func eggErrorClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DisableKeepAlives: true},
	}
}

// peekModelNotAvailable 判断这个 5xx 响应是不是"模型没装载"（读体后**原样还原**，下游照常能读）。
func peekModelNotAvailable(resp *http.Response) bool {
	if resp == nil || resp.Body == nil {
		return false
	}
	if resp.StatusCode != http.StatusServiceUnavailable && resp.StatusCode != http.StatusBadGateway {
		return false
	}
	head, err := io.ReadAll(io.LimitReader(resp.Body, eggPeekLimit))
	if err != nil && len(head) == 0 {
		return false
	}
	// 还原：已读部分 + 剩余部分拼回去
	rest := resp.Body
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(strings.NewReader(string(head)), rest), rest}
	if len(head) == 0 {
		return false
	}
	low := strings.ToLower(string(head))
	return strings.Contains(low, "model not available") ||
		strings.Contains(low, "not loaded") ||
		strings.Contains(low, "no such model") ||
		strings.Contains(string(head), "模型不可用") ||
		strings.Contains(string(head), "未装载")
}

// eggStateOn 查询目标机 /eggs，返回该模型的状态；该机没把它当卵 ⇒ ("", false)。
func (g *Gateway) eggStateOn(model string, host string) (string, bool) {
	ip, port := g.fleetNode(host)
	if ip == "" {
		return "", false
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:%d/eggs", ip, port), nil)
	if err != nil {
		return "", false
	}
	if g.authToken != "" {
		req.Header.Set("X-Auth-Token", g.authToken)
	}
	resp, err := eggErrorClient(3 * time.Second).Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	var body struct {
		Eggs []struct {
			Model string `json:"model"`
			State string `json:"state"`
		} `json:"eggs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", false
	}
	for _, e := range body.Eggs {
		if e.Model == model {
			return e.State, true
		}
	}
	return "", false
}

// hatchEgg 向目标机发一次孵蛋（幂等：已装载时子端不会重起引擎），成功且复核 ready 才返回 true。
func (g *Gateway) hatchEgg(model, host string) bool {
	state, isEgg := g.eggStateOn(model, host)
	if !isEgg {
		log.Printf("⚠️ %s on %s reported 5xx but is not a managed egg — not hatching", model, host)
		return false
	}
	if state == "ready" {
		// 已是 ready 却回 5xx ⇒ 不是"冷"的问题，别硬孵（保持原失败语义，不掩盖真故障）
		log.Printf("⚠️ %s on %s is already %q yet returned 5xx — not hatching", model, host, state)
		return false
	}
	log.Printf("🥚 egg %s on %s is %q — hatching, then one retry", model, host, state)
	ip, port := g.fleetNode(host)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s:%d/load", ip, port),
		strings.NewReader(fmt.Sprintf(`{"model":%q}`, model)))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	if g.authToken != "" {
		req.Header.Set("X-Auth-Token", g.authToken)
	}
	resp, err := eggErrorClient(eggLoadTimeout).Do(req)
	if err != nil {
		log.Printf("⚠️ hatch request failed for %s on %s: %v — continuing (原失败语义不变)", model, host, err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ hatch for %s on %s returned %d — continuing", model, host, resp.StatusCode)
		return false
	}
	// 复核（不看响应码下结论 = 假绿）
	if st, ok := g.eggStateOn(model, host); ok && st == "ready" {
		log.Printf("🥚 egg %s on %s hatched → ready", model, host)
		return true
	}
	log.Printf("⚠️ hatch for %s on %s returned 200 但复核未 ready — continuing", model, host)
	return false
}
