package gateway

import (
	"fmt"
	"testing"
	"time"
)

// O-7 绑定表硬上限 + LRU（正 2 负 2）与 Q-222 读路径刷新。

func TestSessionBindingCapAndLRU(t *testing.T) {
	g := NewGateway("test-token", nil, nil, nil)
	total := maxSessionBindings + 88
	for i := 0; i < total; i++ {
		g.bindSession(fmt.Sprintf("k%04d", i), "x3", "m")
	}
	if n := len(g.sessions); n > maxSessionBindings {
		t.Fatalf("超出硬上限：%d > %d", n, maxSessionBindings)
	}
	if _, ok := g.sessions["k0000"]; ok {
		t.Fatal("LRU 未淘汰最久未用者 k0000")
	}
	if _, ok := g.sessions[fmt.Sprintf("k%04d", total-1)]; !ok {
		t.Fatal("最新写入者被误淘汰")
	}
}

func TestBoundSessionRefreshKeepsAlive(t *testing.T) {
	g := NewGateway("test-token", nil, nil, nil)
	g.bindSession("s1", "x3", "m")
	g.sessions["s1"] = sessionBinding{host: "x3", model: "m", lastUsed: time.Now().Add(-sessionTTL + 2*time.Minute)}
	if got := g.boundSession("s1", "m"); got != "x3" {
		t.Fatalf("未命中：%q", got)
	}
	if d := time.Since(g.sessions["s1"].lastUsed); d > time.Minute {
		t.Fatalf("命中未刷新 lastUsed（Q-222）：距上次使用 %v", d)
	}
}

func TestBoundSessionExpiredAndModelMismatch(t *testing.T) {
	g := NewGateway("test-token", nil, nil, nil)
	g.sessions["s2"] = sessionBinding{host: "x3", model: "m", lastUsed: time.Now().Add(-2 * sessionTTL)}
	if got := g.boundSession("s2", "m"); got != "" {
		t.Fatalf("过期项应返回空，得到 %q", got)
	}
	if _, ok := g.sessions["s2"]; ok {
		t.Fatal("过期项应被删除")
	}
	g.bindSession("s3", "x3", "m1")
	if got := g.boundSession("s3", "m2"); got != "" {
		t.Fatalf("换模型应重新路由，得到 %q", got)
	}
}
