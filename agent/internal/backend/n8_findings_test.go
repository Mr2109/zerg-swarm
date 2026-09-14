// n8_findings_test.go —— 2026-09-14 N8 真机演练抓出的两个缺陷的回归。
//
// 这两个缺陷**任何单测都没覆盖到**，只有把整条链路放到真机上跑才现形：
//
//	缺陷A：probeServiceIdle 把 busy 当 idle 返回 ⇒ 借用 100% 不可用，
//	       且日志自相矛盾（「目标服务正忙：4 个槽均空闲」）。
//	缺陷B：M8 准入在统一内存平台上双重扣减 ⇒ 可用内存被打到 4.3GB，任何真实模型都装不下。
package backend

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// 缺陷A：方向必须是 idle（而不是把 parseSlotBusy 的 busy 直接透传）。
func TestProbeServiceIdle_DirectionIsIdleNotBusy(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantIdle bool
	}{
		{
			name:     "四个槽全空闲（X3 真机形态）",
			body:     `[{"is_processing":false},{"is_processing":false},{"is_processing":false},{"is_processing":false}]`,
			wantIdle: true,
		},
		{
			name:     "有槽在处理 ⇒ 不空闲",
			body:     `[{"is_processing":true},{"is_processing":false}]`,
			wantIdle: false,
		},
		{name: "空响应 ⇒ 按忙（fail-safe）", body: ``, wantIdle: false},
		{name: "无 is_processing 字段 ⇒ 按忙", body: `{"foo":1}`, wantIdle: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/slots" {
					t.Errorf("探测路径应为 /slots，实得 %s", r.URL.Path)
				}
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()

			idle, detail := probeServiceIdle(portOfURL(t, srv.URL), 3*time.Second)
			if idle != c.wantIdle {
				t.Fatalf("idle=%v（依据：%s），期望 %v", idle, detail, c.wantIdle)
			}
			if idle && detail == "" {
				t.Fatal("判定为空闲时必须给出依据（否则真机上没法解释为什么借/不借）")
			}
		})
	}
}

// 连不上时必须按"不空闲"处理（绝不能在探不到的时候去停别人的服务）。
func TestProbeServiceIdle_UnreachableMeansNotIdle(t *testing.T) {
	if idle, _ := probeServiceIdle(59998, 2*time.Second); idle {
		t.Fatal("连不上时必须按忙处理（fail-closed），否则会去停一个探不到状态的服务")
	}
}

// 缺陷B：占用扣减默认关闭（统一内存平台扣一次就是双重扣减）。
func TestDeductOccupancyFromAvail_DefaultOff(t *testing.T) {
	t.Setenv(EnvDeductOccupancy, "")
	if deductOccupancyFromAvail() {
		t.Fatal("默认必须为 false —— X3 实测 MemAvailable 61.0GB 里已含基线 56.68GB，" +
			"再扣一次只剩 4.3GB，任何真实模型都装不下")
	}
	t.Setenv(EnvDeductOccupancy, "1")
	if !deductOccupancyFromAvail() {
		t.Fatal("显式设 1 时应为 true（离散 GPU：采样看不见显存）")
	}
	t.Setenv(EnvDeductOccupancy, "yes")
	if !deductOccupancyFromAvail() {
		t.Fatal("yes 也应视为开启")
	}
	t.Setenv(EnvDeductOccupancy, "0")
	if deductOccupancyFromAvail() {
		t.Fatal("0 应视为关闭")
	}
}
