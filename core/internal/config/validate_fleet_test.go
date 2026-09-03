package config

import (
	"os"
	"testing"
)

// TestValidateRealFleet 对真实 fleet.yaml 跑校验（M0 验证——27 模型）
func TestValidateRealFleet(t *testing.T) {
	data, err := os.ReadFile("<repo>/gateway/fleet.yaml")
	if err != nil {
		t.Skip("fleet.yaml 不存在")
	}
	cfg, err := ParseFleetConfig(data)
	if err != nil {
		t.Fatalf("fleet.yaml 解析失败: %v", err)
	}
	total, fatal := 0, 0
	for name, candidates := range cfg.Models {
		total++
		if len(candidates) == 0 {
			continue
		}
		res := Validate(name, candidates[0])
		if res.HasFatal() {
			fatal++
			t.Logf("模型 %s: Fatal 错误 %v", name, collectFields(res.Errors))
		}
	}
	t.Logf("fleet.yaml 模型数: %d, Fatal: %d", total, fatal)
	if fatal > 0 {
		t.Errorf("%d 个模型校验失败", fatal)
	}
}
