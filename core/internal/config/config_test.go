package config

import (
	"os"
	"path/filepath"
	"testing"
)

// fleetPath 定位 fleet.yaml：
//  1. 环境变量 ZERG_FLEET_YAML（显式指定，测试可注入假表）
//  2. 从当前工作目录向上找 gateway/fleet.yaml，再找 gateway/fleet.example.yaml
//
// 为什么要这样：公开快照**有意排除**私有 gateway/fleet.yaml（只留 fleet.example.yaml），
// 所以测试绝不能硬编码私有绝对路径——那会让公开仓的 CI 必失败（2026-09-11 CI 实测踩到）。
// 两种形态都要成立：私有仓用真表做严格断言；公开快照用示例表验证"可加载"；都没有则跳过。
func fleetPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("ZERG_FLEET_YAML"); p != "" {
		return p
	}
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for i := 0; i < 8; i++ {
		for _, name := range []string{"gateway/fleet.yaml", "gateway/fleet.example.yaml"} {
			p := filepath.Join(dir, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func TestLoadFleetConfig(t *testing.T) {
	p := fleetPath(t)
	if p == "" {
		t.Skip("未找到 gateway/fleet.yaml 或 gateway/fleet.example.yaml（公开快照形态）——跳过")
	}

	cfg, err := LoadFleetConfig(p)
	if err != nil {
		t.Fatalf("加载失败(%s): %v", p, err)
	}
	if len(cfg.Models) == 0 {
		t.Fatalf("无模型：%s", p)
	}

	// 示例表只验证"可加载"（公开快照形态）；真表才做下面的字段断言。
	if filepath.Base(p) != "fleet.yaml" {
		t.Logf("公开快照形态：用 %s 验证可加载性（%d 个模型组）", filepath.Base(p), len(cfg.Models))
		return
	}

	// 检查 example-35b 模块字段（仅私有真表）
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
