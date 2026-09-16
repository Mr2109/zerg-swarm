package gateway

import (
	"os"
	"path/filepath"
	"testing"
)

// 首 token 闸「只放宽、不收窄」：思考型模型不得被适配器的 120s 掐死；档案实测值优先且更大时生效。
func TestThinkingModelFirstTokenMin(t *testing.T) {
	if got := thinkingModelFirstTokenMin("Qwen3.8-27B"); got < 300 {
		t.Errorf("思考型模型下限过低：%d", got)
	}
	if got := thinkingModelFirstTokenMin("gemma-4-26B"); got != 0 {
		t.Errorf("非思考型不应被强行下限：%d", got)
	}
}

func TestGetRequestTimeoutOnlyWidens(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir)
	g := &Gateway{timeoutOverride: map[string]int{"Qwen3.8-27B": 120, "gemma-4-26B": 90}}

	// ① 思考型：适配器 120s ⇒ 应被放宽（绝不低于 300）
	if got := g.getRequestTimeout("Qwen3.8-27B"); got < 300 {
		t.Errorf("思考型未被放宽：%d", got)
	}
	// ② 非思考型：不放松也不收窄 ⇒ 原值
	if got := g.getRequestTimeout("gemma-4-26B"); got != 90 {
		t.Errorf("非思考型被改动：%d", got)
	}
	// ③ 档案实测值更大 ⇒ 以档案为准
	if err := os.WriteFile(filepath.Join(dir, "Qwen3.8-27B.yaml"), []byte("first_token_sec: 900\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := g.getRequestTimeout("Qwen3.8-27B"); got != 900 {
		t.Errorf("档案值未生效：%d", got)
	}
	// ④ 档案值更小 ⇒ 不得收窄（取最大）
	if err := os.WriteFile(filepath.Join(dir, "gemma-4-26B.yaml"), []byte("first_token_sec: 10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := g.getRequestTimeout("gemma-4-26B"); got != 90 {
		t.Errorf("被档案收窄了（不允许）：%d", got)
	}
}
