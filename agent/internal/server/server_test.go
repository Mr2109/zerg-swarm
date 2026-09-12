package server

import (
	"sync"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/heartbeat"
)

// 编译期断言：Agent 满足心跳的 ActiveCounter 接口（active_requests 接真值的前提）。
var _ heartbeat.ActiveCounter = (*Agent)(nil)

// TestAgentActiveRequests_ReflectsCounter —— active_requests 来源=activeReqs 真值，随增减。
func TestAgentActiveRequests_ReflectsCounter(t *testing.T) {
	a := NewAgent("x3", "tok", nil, nil, "")
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
	a := NewAgent("x3", "tok", nil, nil, "")
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
