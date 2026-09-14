// service_control_test.go —— 判忙探针的回归测试（纯函数）。
//
// 原属本文件的进程树解析（parsePsTree / parsePsPGID / stopProcessTree N7 用例）与
// 归还 env 还原（envFromFiltered）用例随借用/归还机制一起退场（§10.1 service_control.go 行）。
// parseSlotBusy 的语义被 p2_inflight_test.go 的源码断言钉住：只作交叉校验，不进判定路径。
package backend

import "testing"

// ── 空闲检定（§9.2；P4 起仅作交叉校验）────────────────────────────────────

func TestParseSlotBusy(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{
			"四槽全闲（llama.cpp 多槽形态）",
			`[{"id":0,"n_ctx":262144,"speculative":false,"is_processing":false},{"id":1,"n_ctx":262144,"speculative":false,"is_processing":false},{"id":2,"n_ctx":262144,"speculative":false,"is_processing":false},{"id":3,"n_ctx":262144,"speculative":false,"is_processing":false}]`,
			false,
		},
		{
			"单槽闲（llama.cpp 单槽形态）",
			`[{"id":0,"n_ctx":32768,"speculative":false,"is_processing":false}]`,
			false,
		},
		{"有槽在处理 ⇒ 忙", `[{"id":0,"is_processing":false},{"id":1,"is_processing":true}]`, true},
		{"带空格的 JSON 也算忙", `[{"id":0,"is_processing": true}]`, true},
		{"空响应 ⇒ 按忙（保守）", "", true},
		{"无该字段 ⇒ 按忙（保守）", `[{"id":0,"n_ctx":32768}]`, true},
		{"非法 JSON ⇒ 按忙（保守）", "not json", true},
	}
	for _, c := range cases {
		got, detail := parseSlotBusy([]byte(c.body))
		if got != c.want {
			t.Errorf("%s: busy=%v（%s），期望 %v", c.name, got, detail, c.want)
		}
		if detail == "" {
			t.Errorf("%s: 必须给出判定依据（detail）", c.name)
		}
	}
}
