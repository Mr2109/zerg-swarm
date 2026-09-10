package chat

import "testing"

// 丙批 C2 补（2026-09-10）：暂时性 4xx（408/425/499）不得直接硬熔断
func TestCompactTransient4xxIsRetryable(t *testing.T) {
	for _, code := range []int{408, 425, 499} {
		c := CompactErrClass{HTTPStatus: code}
		if c.nonRetryable() {
			t.Fatalf("status %d 被归为不可重试——超时/过早/断开应当可重试", code)
		}
		if c.kind() != compactClass5xx {
			t.Fatalf("status %d kind=%s，期望按 5xx 递进", code, c.kind())
		}
	}
	// 真·请求错误仍直接熔断
	for _, code := range []int{400, 401, 403, 404, 422} {
		if !(CompactErrClass{HTTPStatus: code}).nonRetryable() {
			t.Fatalf("status %d 应不可重试（直接熔断）", code)
		}
	}
	// 429 走冷却下界（既有行为不变）
	if (CompactErrClass{HTTPStatus: 429}).nonRetryable() {
		t.Fatal("429 不应直接熔断（走冷却下界）")
	}
}
