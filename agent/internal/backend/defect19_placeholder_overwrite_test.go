package backend

import (
	"testing"
	"time"
)

// defect19_placeholder_overwrite_test.go —— 真机缺陷 19（2026-09-15 沙箱复现）的回归判据。
//
// 真机形态（沙箱子端日志逐行）：
//
//	18:57:12 提前收卵 phi-mini（无在飞，锁内置 draining）      ← 赢了权、状态置 draining
//	          …但收卵动作没执行（并发下扫描已扫不到）⇒ 卵卡在 draining
//	18:57:12 孵化失败: Unit zerg-phi-mini.service was already loaded  ← 同名 /load 去孵同名单元
//	18:57:12 五档淘汰(tier=1…): 卸载 phi-mini (释放 ≈2.6GB)      ← 删掉的是**幻影条目**（Unit 空）
//	⇒ 真单元失去记账 ⇒ 孤儿（占 6.5 GB 显存、/eggs 失真、同名 /load 永久失败）
//
// 两条不变量（改回旧写法即红）：
//
//	① 同名占位**不得覆盖**在途条目：必须先把它停干净（真单元要被 collectUnit 停掉）；
//	② 账本里**不得**留下 Unit 为空的幻影条目（孵化失败必须删条目）。
func TestDefect19_PlaceholderMustNotOverwriteEntry(t *testing.T) {
	t.Setenv("ZERG_HATCH", "1")
	m := startLockTestManager(t, "eggX")

	old := &subproc{
		model: "eggX", Unit: "zerg-eggX", port: 9421,
		state: StateDraining, lastUsed: time.Now(), entry: inflightEntry(),
	}
	m.mu.Lock()
	m.procs["eggX"] = old
	// 驻留上限放宽：本用例要验的是「占位守卫」，不是五档淘汰（后者会先把条目摘走 ⇒ 验不到）
	m.maxResident = 3
	stopped := make(chan string, 4)
	m.stopHook = func(sp *subproc) { stopped <- sp.Unit }
	m.mu.Unlock()

	resp, err := m.Start("eggX") // 走到 doStart 的占位守卫
	if err != nil {
		t.Fatalf("自愈路径不应带 err: %v", err)
	}
	if resp == nil || resp["ok"] != true {
		t.Fatalf("卡死 draining 且卵还活着 ⇒ 应放回复用（ok=true），实得 %+v", resp)
	}
	select {
	case u := <-stopped:
		t.Fatalf("自愈路径**不该**停掉还活着的真单元（停了 %q）——那是可避免的服务中断", u)
	default:
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	sp, ok := m.procs["eggX"]
	if !ok {
		t.Fatal("账本条目被丢了 —— 真单元会成孤儿（缺陷 19）")
	}
	if sp.Unit != "zerg-eggX" {
		t.Fatalf("真单元的记账被顶掉了：unit=%q（幻影覆盖 ⇒ 孤儿）", sp.Unit)
	}
	if sp.state != StateReady {
		t.Fatalf("放回后应为 ready，实得 %s", sp.state)
	}
}
