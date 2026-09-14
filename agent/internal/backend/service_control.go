// service_control.go —— 引擎侧判忙探针（P4 退场清理后的残余保留面）。
//
// 设计依据：设计-子端沙箱化-20260914.md §10.1 service_control.go 行、§7.7 修补 3：
// `probeServiceIdle` / `parseSlotBusy`（/slots 的 is_processing 判忙）**降为交叉校验**——
// draining / 卸载 / 切换的唯一依据是子端的在飞引用计数（§7.7 修补 3），
// /slots 是**引擎侧信号**，ds4 类引擎未必提供 ⇒ 正确性不挂在引擎特性上。
//
// 随「卵之外无引擎」退场的部分（P4 已删）：
//   - stopProcessTree / stopScreenSession（手工服务不在管辖内；停进程树没有对象）；
//   - restoreBaselineService + envFromFiltered（不再有需要归还的东西——F2 的
//     「归还进程仍在 agent cgroup」缺陷随之整条消失）；
//   - stopSystemdUnit 的借用用途（收卵由孵化器按 §6.2 授权做）。
//
// 安全铁律（不变）：探不到就按忙；本文件只读，绝不对探针目标发任何写动作。
package backend

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// httpGetBody 发一条只读 GET，返回响应体（上限 1MiB）。仅用于本机探测。
func httpGetBody(url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// ── 空闲检定（§9.2；P4 起仅作交叉校验）──────────────────────────────────────

// parseSlotBusy 解析 /slots 响应判断是否有请求在飞（纯函数，便于测试）。
//
// llama.cpp 的 /slots 返回 [{"id":0,"is_processing":false},…]；任一槽 true 即判忙。
// 解析失败 ⇒ 返回 (true, "无法判定") —— **探不到就按忙**（保守优先）。
func parseSlotBusy(body []byte) (busy bool, detail string) {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return true, "空响应 ⇒ 按忙处理"
	}
	if !strings.Contains(s, "is_processing") {
		return true, "无 is_processing 字段 ⇒ 按忙处理"
	}
	// 不引入 JSON 依赖：直接数 true/false 出现次数（结构固定，字段名唯一）。
	nTrue := strings.Count(s, `"is_processing":true`)
	nTrue += strings.Count(s, `"is_processing": true`)
	nFalse := strings.Count(s, `"is_processing":false`)
	nFalse += strings.Count(s, `"is_processing": false`)
	if nTrue == 0 && nFalse == 0 {
		return true, "解析不到 is_processing 取值 ⇒ 按忙处理"
	}
	if nTrue > 0 {
		return true, fmt.Sprintf("有 %d 个槽在处理请求", nTrue)
	}
	return false, fmt.Sprintf("%d 个槽均空闲", nFalse)
}

// probeServiceIdle 对端口探 /slots 判忙闲（探不到 ⇒ 按忙，绝不打断在飞请求）。
// ⚠ P4 起只作**交叉校验**：卸载/切换判据是子端的在飞引用计数（p2_lifecycle.go），
// 不得把本函数接进任何判定路径（p2_inflight_test 用源码断言钉住）。
func probeServiceIdle(port int, timeout time.Duration) (bool, string) {
	body, err := httpGetBody(fmt.Sprintf("http://127.0.0.1:%d/slots", port), timeout)
	if err != nil {
		return false, "无法连接（" + err.Error() + "）⇒ 按忙处理"
	}
	// ⚠️ 注意方向：parseSlotBusy 返回的是 **busy**，而本函数声明返回的是 **idle**。
	// 2026-09-14 真机 N8 演练抓到的缺陷：这里曾直接 `return parseSlotBusy(body)`
	// ⇒ 空闲（busy=false）时返回 idle=false ⇒ 判定被拒，且日志自相矛盾：
	// 「目标服务正忙：4 个槽均空闲」。
	busy, detail := parseSlotBusy(body)
	return !busy, detail
}
