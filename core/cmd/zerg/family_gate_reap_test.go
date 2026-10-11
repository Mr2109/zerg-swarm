// family_gate_reap_test.go —— `gate reap` 判活口径的机检（层①进程内 · 同包 `package main`）。
//
// 判据（缺口 `GAP-20261002-14` 逐字）：`gate reap` 的判活依据 = **整组/根的 start_time**
// （根的 `etime` 在门限内 ⇒ 整组刚起）与 **组内是否有真在推进的成员**（任一成员 `pcpu > 0`）；
// **循环换壳的子成员不是「在飞」证据**（`precommit-gates.sh` 循环起子壳 ⇒ 子成员 `etime` 恒几秒，
// 不得据此判活 —— 否则该组永远判活、永远收不掉）。
package main

import "testing"

// grStat —— 造一条判活读数（etime 用 `ps` 的 `[[dd-]hh:]mm:ss` 形态）。
func grStat(pcpu float64, etime string) gateReapStat {
	return gateReapStat{pcpu: pcpu, etime: etime}
}

// TestGateReapLiveInTreeRespawnedChildIsNotInFlight —— ★ 负控主靶（GAP-20261002-14 病灶形态）：
// 根已老（10 分钟 ≫ 门限 300s）且无主；子成员循环换壳 ⇒ `etime` 恒几秒、0% CPU。
// 旧口径看「谱系内任意成员的 etime」⇒ 判活、永远收不掉；新口径只认**根**的 start_time ＋ `pcpu`
// ⇒ 无活证据 ⇒ **不判活**（可回收）。把这条判据哑化（改回按后代 etime 判）⇒ 本格必红。
func TestGateReapLiveInTreeRespawnedChildIsNotInFlight(t *testing.T) {
	root := 49942
	allPPID := map[int]int{root: 1, 70001: root}
	stat := map[int]gateReapStat{root: grStat(0, "10:00"), 70001: grStat(0, "00:03")}
	if live, pid, why := gateReapLiveInTree(root, allPPID, stat); live {
		t.Fatalf("循环换壳的年轻子成员被当成了在飞证据：live=true pid=%d why=%q（应判**不活** · GAP-20261002-14）", pid, why)
	}
}

// TestGateReapLiveInTreeYoungRootIsLive —— GAP-20261001-17 形态（回归保护）：根刚起 ⇒ 整组刚起
// ⇒ 判活、绝不碰。（活门跑永不回收。）
func TestGateReapLiveInTreeYoungRootIsLive(t *testing.T) {
	root := 69881
	allPPID := map[int]int{root: 1, 85695: root}
	stat := map[int]gateReapStat{root: grStat(0, "00:05"), 85695: grStat(0, "00:03")}
	live, pid, _ := gateReapLiveInTree(root, allPPID, stat)
	if !live || pid != root {
		t.Fatalf("根刚起（etime<门限）应判活：live=%v pid=%d（GAP-20261001-17：活门跑永不回收）", live, pid)
	}
}

// TestGateReapLiveInTreeCPUBurnIsProgress —— 「真在推进」= 组内有成员在烧 CPU：根已老，但后代 pcpu>0 ⇒ 判活。
func TestGateReapLiveInTreeCPUBurnIsProgress(t *testing.T) {
	root := 1234
	allPPID := map[int]int{root: 1, 1235: root}
	stat := map[int]gateReapStat{root: grStat(0, "20:00"), 1235: grStat(37.5, "12:00")}
	live, pid, _ := gateReapLiveInTree(root, allPPID, stat)
	if !live || pid != 1235 {
		t.Fatalf("组内在烧 CPU 的成员应判活：live=%v pid=%d", live, pid)
	}
}

// TestGateReapLiveInTreeNoEvidenceIsNotLive —— 根老 + 后代也老 + 0% CPU ⇒ 无活证据 ⇒ 不判活。
func TestGateReapLiveInTreeNoEvidenceIsNotLive(t *testing.T) {
	root := 4321
	allPPID := map[int]int{root: 1, 4322: root}
	stat := map[int]gateReapStat{root: grStat(0, "30:00"), 4322: grStat(0, "25:00")}
	if live, pid, _ := gateReapLiveInTree(root, allPPID, stat); live {
		t.Fatalf("无任何活证据却判活：live=true pid=%d", pid)
	}
}
