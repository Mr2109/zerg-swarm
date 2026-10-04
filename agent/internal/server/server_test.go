package server

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/heartbeat"
)

// 编译期断言：Agent 满足心跳的 ActiveCounter 接口（active_requests 接真值的前提）。
var _ heartbeat.ActiveCounter = (*Agent)(nil)

// TestAgentActiveRequests_ReflectsCounter —— active_requests 来源=activeReqs 真值，随增减。
func TestAgentActiveRequests_ReflectsCounter(t *testing.T) {
	a := NewAgent("edge-node", "tok", nil, nil, "")
	if got := a.ActiveRequests(); got != 0 {
		t.Fatalf("初始应为 0，实得 %d", got)
	}
	a.mu.Lock()
	a.activeReqs = 4
	a.mu.Unlock()
	if got := a.ActiveRequests(); got != 4 {
		t.Fatalf("应为 4，实得 %d", got)
	}
}

// TestAgentActiveRequests_ConcurrentIncrementDecrement —— 并发增减后计数守恒（无丢计数/无竞态）。
func TestAgentActiveRequests_ConcurrentIncrementDecrement(t *testing.T) {
	a := NewAgent("edge-node", "tok", nil, nil, "")
	a.mu.Lock()
	a.activeReqs = 4
	a.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.mu.Lock()
			a.activeReqs++
			a.mu.Unlock()
			a.mu.Lock()
			a.activeReqs--
			a.mu.Unlock()
		}()
	}
	wg.Wait()
	if got := a.ActiveRequests(); got != 4 {
		t.Fatalf("并发增减后应仍为 4（守恒），实得 %d", got)
	}
}

// TestDetectCrossFamilyPortConflict —— GAP-20260925-28：端口跨协议族冲突检测的回归牙。
//
// 夹具（照账里 mini01 :8100 的实况）：同一端口先被「外部」监听者**只占住 IPv4**（账里那台
// 是外部 Python 服务），此刻本端若以 net.Listen("tcp", …) 去绑，内核会静默把它降级到
// IPv6-only ⇒ 两边各占一族、互不知情（探活会连上「别人」却报 reachable=true）。
// 断言：① 这种双族夹具必须被检测到并**点名 tcp4**；② 端口全空（本端可独占双栈）时**不误报**。
func TestDetectCrossFamilyPortConflict(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("取空闲端口失败: %v", err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	probe.Close()

	// 负控：端口全空 ⇒ 无冲突（不误报）
	if msg := detectCrossFamilyPortConflict("0.0.0.0", port); msg != "" {
		t.Fatalf("端口全空不应报冲突，实得 %q", msg)
	}

	// 正控：外部监听者只占 IPv4 ⇒ 本端会被静默降级到 IPv6 ⇒ 必须报冲突并点名 tcp4
	foreign, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		t.Fatalf("夹具（外部 IPv4 监听者）起不来: %v", err)
	}
	defer foreign.Close()

	msg := detectCrossFamilyPortConflict("0.0.0.0", port)
	if msg == "" {
		t.Fatal("IPv4 被外部占住时必须报「跨协议族冲突」（这正是 GAP-20260925-28 的假阳性根因）")
	}
	if !strings.Contains(msg, "tcp4") {
		t.Fatalf("报错必须点名被占的协议族 tcp4，实得 %q", msg)
	}
}
