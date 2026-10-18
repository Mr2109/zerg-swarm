package main

// family_fusion_served_test.go —— 缺口 GAP-20261019-01 的 `zerg ask` 可分辨面。
//
// 钉两件事：
//  ① 机器面（`--json`）必须给出 served_* / pinned 五格（调用方据此判「真答话的是谁」）；
//  ② 读头是**纯抄**：有则抄、无则空（✗ 拿路由选中的 model/host 顶替）。

import (
	"net/http"
	"testing"
)

func TestAskFields_CarryServedIdentity(t *testing.T) {
	want := map[string]bool{
		"served_model": false, "served_host": false, "served_port": false,
		"served_pid": false, "pinned": false,
	}
	for _, f := range askFields {
		if _, ok := want[f]; ok {
			want[f] = true
		}
	}
	for f, hit := range want {
		if !hit {
			t.Fatalf("askFields 缺字段 %q —— --json 面就分不出「谁在应答」", f)
		}
	}
}

func TestAskServedFromHeader(t *testing.T) {
	h := http.Header{}
	h.Set("X-Zerg-Served-Model", "Qwen3.8-Flash-Next")
	h.Set("X-Zerg-Served-Host", "edge-node")
	h.Set("X-Zerg-Served-Port", "9400")
	h.Set("X-Zerg-Served-Pid", "1913944")
	h.Set("X-Zerg-Served-Pinned", "default")
	got := askServedFromHeader(h)
	if got.Model != "Qwen3.8-Flash-Next" || got.Host != "edge-node" || got.Port != "9400" ||
		got.Pid != "1913944" || got.Pinned != "default" {
		t.Fatalf("五格抄错: %#v", got)
	}

	empty := askServedFromHeader(http.Header{})
	if empty != (askServed{}) {
		t.Fatalf("上游没给头时应全空（✗ 拿别处的值顶替），实得 %#v", empty)
	}
}
