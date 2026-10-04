package config

import "testing"

// GAP-20260926-71：取址口径必须是**跨包共享的 config 纯函数** NodeAddr（此前只是 gateway 包私有方法）。
// 改前：config 包无 NodeAddr ⇒ 引用处编译不过（undefined: NodeAddr）＝共享函数不存在。
func TestGAP71_NodeAddrSharedSource(t *testing.T) {
	fleet := map[string]FleetNode{
		"edge-node":    {Host: "10.9.9.9", Port: 9001},
		"mini-node-1": {Host: "10.9.9.8"}, // 未给 port ⇒ 回退 DefaultAgentPort
	}
	cases := []struct {
		host     string
		wantIP   string
		wantPort int
	}{
		{"edge-node", "10.9.9.9", 9001},
		{"mini-node-1", "10.9.9.8", DefaultAgentPort},
		{"notinmap", "notinmap", DefaultAgentPort},
	}
	for _, c := range cases {
		ip, port := NodeAddr(fleet, c.host)
		if ip != c.wantIP || port != c.wantPort {
			t.Errorf("NodeAddr(%q)=(%q,%d)，want (%q,%d)", c.host, ip, port, c.wantIP, c.wantPort)
		}
	}
	if ip, port := NodeAddr(nil, "edge-node"); ip != "edge-node" || port != DefaultAgentPort {
		t.Errorf("NodeAddr(nil,edge-node)=(%q,%d)，want (edge-node,%d)", ip, port, DefaultAgentPort)
	}
}
