package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidateRealFleet 对真实 fleet.yaml 跑校验（M0 验证——27 模型）。
// 公开快照里没有私有 fleet.yaml（只留 fleet.example.yaml）：此时只验证示例表**可解析**，
// 不跑严格字段校验（示例表是模板，字段可能故意留空）。
func TestValidateRealFleet(t *testing.T) {
	p := fleetPath(t)
	if p == "" {
		t.Skip("未找到 fleet.yaml / fleet.example.yaml（公开快照形态）——跳过")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("读不到 %s: %v", p, err)
	}
	cfg, err := ParseFleetConfig(data)
	if err != nil {
		t.Fatalf("%s 解析失败: %v", filepath.Base(p), err)
	}
	if filepath.Base(p) != "fleet.yaml" {
		t.Logf("公开快照形态：%s 解析通过（%d 个模型组），跳过严格字段校验", filepath.Base(p), len(cfg.Models))
		return
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
