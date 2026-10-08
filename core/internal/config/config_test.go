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

// TestNodeAddr —— GAP-20260926-71：取址口径提为 config 纯函数后，机器名→(IP,端口) 的
// 真源行为（含缺配置 / 缺 host 的回退与默认端口）必须逐条成立；私有真表在盘上时再与
// fleet.yaml 的 fleet 段逐字对拍。哑化 NodeAddr（忽略 fleet）即本用例红。
func TestNodeAddr(t *testing.T) {
	fleet := map[string]FleetNode{
		"node-a": {Host: "<cluster-ip>", Port: 8100},
		"node-b": {Host: "<cluster-ip>"}, // 未给端口 → 回退默认端口
		"bare":   {},                     // 未给 host → 回退机器名
	}
	cases := []struct {
		host     string
		wantIP   string
		wantPort int
	}{
		{"node-a", "<cluster-ip>", 8100},
		{"node-b", "<cluster-ip>", DefaultAgentPort},
		{"bare", "bare", DefaultAgentPort},
		{"missing", "missing", DefaultAgentPort},
	}
	for _, c := range cases {
		ip, port := NodeAddr(fleet, c.host)
		if ip != c.wantIP || port != c.wantPort {
			t.Errorf("NodeAddr(%q) = (%q,%d)，want (%q,%d)", c.host, ip, port, c.wantIP, c.wantPort)
		}
	}
	// nil map：不得 panic，走回退支。
	if ip, port := NodeAddr(nil, "node-a"); ip != "node-a" || port != DefaultAgentPort {
		t.Errorf("NodeAddr(nil,node-a) = (%q,%d)，want (node-a,%d)", ip, port, DefaultAgentPort)
	}
	// 真源对拍（仅私有真表形态）。
	if p := fleetPath(t); p != "" && filepath.Base(p) == "fleet.yaml" {
		cfg, err := LoadFleetConfig(p)
		if err != nil {
			t.Fatalf("加载 %s: %v", p, err)
		}
		if node, ok := cfg.Fleet["node-a"]; ok && node.Host != "" {
			if ip, port := NodeAddr(cfg.Fleet, "node-a"); ip != node.Host || port != node.Port {
				t.Errorf("node-a 与真源不符：NodeAddr=(%q,%d) fleet.yaml=(%q,%d)", ip, port, node.Host, node.Port)
			}
		}
	}
}
