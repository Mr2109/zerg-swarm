package backend

// manager_load_reject_test.go —— T2 / T3（《设计-资源管理器》§七）故障注入式验收。
//
// 与既有用例的分工（不重复）：manager_evict_test.go 覆盖的是**裁决与预检函式**本身
// （evictIfNeededLocked / evictForMemoryLocked / ensureMemoryForLocked / rejectInsufficientMemory）。
// 本文件补的是"**装载入口 doStart 的接线**"——
//   T3：内存不足 → 装载返回 507，且目标模型**不进入驻留清单**（账本不出现它为 ready）；
//   T2：在飞的 A 被请求 B 时**绝不被驱逐**，B 的装载被**明确拒绝**（507），不是静默抢。
// 既有用例只验到函式级；若 doStart 里 `if !ok { return reject }` 的接线被删掉，
// 既有用例不会红——本文件会（见交付物里的变异验证）。
//
// 安全：测试条目把 Cmd 指到不存在的二进制——即便接线被破坏、装载流程继续走下去，
// 也只会快速失败（500），不会真的 spawn 出 llama-server 进程。

import (
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// noopEntry 造一个"需求大到内存不够、且即便往下走也不会真起进程"的后端条目。
func noopEntry(memGB float64) *registry.ModelEntry {
	return &registry.ModelEntry{
		MemGB: memGB,
		Cmd:   registry.CmdString("/nonexistent/zerg-acceptance-noop-binary"),
	}
}

// T3：注入「可用内存 + 可腾退 < 需求」→ 装载返回 507（insufficient memory），目标不进 resident。
func TestT3_DoStartRejects507AndTargetNotResident(t *testing.T) {
	// 唯一的可腾退项只有 4G；需求取一个远超真机可用内存的值 → 可用+可腾退 < 需求。
	m := newEvictTestManager(3, map[string]*subproc{
		"small": {model: "small", state: StateReady, entry: &registry.ModelEntry{MemGB: 4}, lastUsed: time.Now().Add(-time.Hour)},
	})

	resp, err := m.doStart("huge", noopEntry(1e9))
	if err != nil {
		t.Fatalf("doStart 不应返回 error（拒装走响应体）: %v", err)
	}
	if resp["status"] != 507 {
		t.Fatalf("内存不足必须拒装 507，实得 %+v", resp)
	}
	if resp["code"] != "insufficient memory" {
		t.Fatalf("拒装语义必须沿用 manager.go 的 insufficient memory，实得 %+v", resp)
	}
	if resp["ok"] == true {
		t.Fatalf("拒装必须是明确错误，不能报成功: %+v", resp)
	}
	// 账本不出现该模型为 ready：resident 明细里不得有 huge。
	for _, d := range m.ResidentDetail() {
		if d.Alias == "huge" {
			t.Fatalf("被拒装的模型不得进入驻留清单，实得 %+v", m.ResidentDetail())
		}
	}
	if _, has := m.procs["huge"]; has {
		t.Fatalf("被拒装的模型不得进入 procs，实得 %v", keysOf(m.procs))
	}
}

// T2：在飞请求的 A 绝不被驱逐；请求 B → B 的装载被明确拒绝（507），非静默抢。
func TestT2_InflightResident_NewLoadExplicitlyRejected(t *testing.T) {
	// 单槽 + 最旧 + 最大的 A 有在飞请求（reqCount>0）——干扰条件给满，仍不许动它。
	m := newEvictTestManager(1, map[string]*subproc{
		"A": {
			model:    "A",
			state:    StateReady,
			entry:    &registry.ModelEntry{MemGB: 86},
			lastUsed: time.Now().Add(-24 * time.Hour),
			reqCount: 1,
		},
	})

	resp, err := m.doStart("B", noopEntry(1e9))
	if err != nil {
		t.Fatalf("doStart 不应返回 error: %v", err)
	}
	if _, still := m.procs["A"]; !still {
		t.Fatal("红线①被破：有在飞请求的 A 被装载流程驱逐了")
	}
	if _, has := m.procs["B"]; has {
		t.Fatalf("B 的装载应被明确拒绝（非静默抢），却进入了 resident: %v", keysOf(m.procs))
	}
	if resp["status"] != 507 || resp["ok"] == true {
		t.Fatalf("在飞时新装载必须返回明确错误（507），不得静默成功: %+v", resp)
	}
}
