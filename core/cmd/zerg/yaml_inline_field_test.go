// yaml_inline_field_test.go —— GAP-20261019-12 的判据件。
//
// 病（实测）：`agent deploy Mr2109` 拼出 `open "/Volumes/…/bin"/zerg-agentd.new: no such file or
// directory` —— 因为 yamlInlineField 是手搓按行解析，把 YAML 行内映射里的引号当成取值。
// 两态：① 双/单引号成对 ⇒ 剥掉；② 不带引号 / 不成对 ⇒ 原样（✗ 不猜）。
package main

import "testing"

func TestYAMLInlineFieldUnquotes(t *testing.T) {
	cases := []struct{ body, key, want string }{
		{`{ name: Mr2109, prefix: "<repo>/bin", api: "http://127.0.0.1:8580" }`, "prefix", "<repo>/bin"},
		{`{ name: edge-node, ssh: g01@<worker-ip>, prefix: /home/<user>/zerg/bin }`, "prefix", "/home/<user>/zerg/bin"},
		{`{ name: m, prefix: '/opt/zerg/bin' }`, "prefix", "/opt/zerg/bin"},
		{`{ name: m, prefix: "/opt/a }`, "prefix", `"/opt/a`}, // 不成对 ⇒ 原样
		{`{ name: Mr2109 }`, "prefix", ""},
	}
	for _, c := range cases {
		if got := yamlInlineField(c.body, c.key); got != c.want {
			t.Errorf("yamlInlineField(%q, %q) = %q，期望 %q", c.body, c.key, got, c.want)
		}
	}
}
