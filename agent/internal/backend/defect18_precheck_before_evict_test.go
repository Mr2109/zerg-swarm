package backend

import (
	"testing"
	"time"
)

// defect18_precheck_before_evict_test.go —— 真机缺陷 18（2026-09-15）的回归判据。
//
// 现象（生产事故后半段 + 沙箱各复现一次）：
//
//	异模型 /load → 「提前收卵 当前卵（无在飞）」→ 收卵完成 → 然后才 ✗ 拒孵（无实测档案）
//	⇒ 正在服务的卵被白收、新模型也没上 ⇒ 服务归零。
//
// 根因：`Start` 里「收卵」在 `hatchGateLocked`（声明/档案/资源判据）**之前** ⇒
// 一个注定被 507 的请求会先把服务收掉，再自己失败。
//
// 判据（改回旧顺序即红）：静态预检未过时 ——
//
//	① 返回拒绝（507/502），② 卵仍在账本里，③ 卵状态**一丝不变**（不得停在 draining）。
func TestDefect18_PrecheckBeforeEvict_KeepsServingEgg(t *testing.T) {
	t.Setenv("ZERG_HATCH", "1") // 开孵化：静态预检只在这条路径上生效
	m := startLockTestManager(t, "eggA", "eggB")

	m.mu.Lock()
	m.procs["eggA"] = &subproc{
		model: "eggA", Unit: "zerg-eggA", port: 9411,
		state: StateReady, lastUsed: time.Now(),
		entry: inflightEntry(),
	}
	m.mu.Unlock()

	before := m.StateOf("eggA")
	resp, err := m.Start("eggB") // eggB 无实测档案 ⇒ 预检必须挡下
	if err != nil {
		t.Fatalf("预检拒绝不应带 err（应返回结构化响应）: %v", err)
	}
	if resp == nil {
		t.Fatal("eggB 无实测档案：必须被拒（返回结构化响应）")
	}
	if st := resp["status"]; st != 507 && st != 502 {
		t.Fatalf("应 507（无档案）/502（声明不过），实得 %v（%v）", st, resp)
	}
	if _, ok := m.procs["eggA"]; !ok {
		t.Fatal("正在服务的卵被收掉了 —— 服务归零（缺陷 18 回归）")
	}
	if st := m.StateOf("eggA"); st != before {
		t.Fatalf("预检未过却动了卵：状态 %s → %s（不得停在 draining）", before, st)
	}
}
