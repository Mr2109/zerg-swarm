package config

import "testing"

func TestLoadFleetConfig(t *testing.T) {
	cfg, err := LoadFleetConfig("<repo>/gateway/fleet.yaml")
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if len(cfg.Models) == 0 {
		t.Fatal("无模型")
	}
	// 检查 example-35b 模块字段
	if cands, ok := cfg.Models["example-35b"]; ok {
		c := cands[0]
		if c.ToolSupport == nil || !*c.ToolSupport {
			t.Errorf("example-35b tool_support 应为 true")
		}
		if !c.Verified {
			t.Errorf("example-35b verified 应为 true")
		}
		t.Logf("example-35b: desc=%s", c.Description)
	} else {
		t.Error("example-35b 未找到")
	}
}
