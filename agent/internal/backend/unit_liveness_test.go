// unit_liveness_test.go —— 2026-09-19 ②：卵单元活性核验（backend 侧只读探针）。
//
// 消费侧（server /eggs、/services）靠它把「单元已死」的卵从 ready 里摘出来；本文件钉住核验本身：
//
//	① failed（OOM 形态）⇒ 判死 + 理由带 systemd 原文（Result=oom-kill）；
//	② inactive ⇒ 判死；activating/reloading/deactivating（过渡态）⇒ **算活着**（不误杀慢启动的卵）；
//	③ 核不到（没有属性行 / 探针报错）⇒ missing（「不知道」不许当「活着」）；
//	④ 没有单元名（裸 exec 路径）⇒ 不核、State 空（如实缺席，不编造）；
//	⑤ 判词档位 dead_unit 与 stuck_* 分开（前者是「单元没了」，后者是「单元还在但不推进」）。
package backend

import (
	"errors"
	"strings"
	"testing"
)

// ① OOM 之后单元留在 failed ⇒ 明确判死，理由必须能复盘。
func TestUnitLiveness_FailedUnitIsDead(t *testing.T) {
	withUnitStateProbe(t, func(unit string) (unitState, error) {
		if unit != "zerg-deepseek-v4-flash" {
			t.Errorf("探针收到意外单元名 %q", unit)
		}
		return unitState{ActiveState: "failed", SubState: "failed", Result: "oom-kill", ExecMainStatus: 0}, nil
	})
	m := NewManager(nil, "x3")
	lv := m.UnitLiveness("zerg-deepseek-v4-flash")
	if lv.Alive || lv.State != "failed" || !lv.Dead() {
		t.Fatalf("failed 单元必须判死：%+v", lv)
	}
	for _, want := range []string{"ActiveState=failed", "Result=oom-kill"} {
		if !strings.Contains(lv.Detail, want) {
			t.Errorf("理由里应带 %q（复盘要用）：%q", want, lv.Detail)
		}
	}
}

// ② 各态口径：inactive 判死；三个过渡态算活着（真机教训：慢启动的正确卵不许被误杀）。
func TestUnitLiveness_StateMatrix(t *testing.T) {
	cases := []struct {
		active string
		dead   bool
	}{
		{"active", false},
		{"activating", false},
		{"reloading", false},
		{"deactivating", false},
		{"inactive", true},
		{"failed", true},
		{"ACTIVE", false}, // systemd 给的是小写，但大小写不该改变结论
	}
	for _, c := range cases {
		withUnitStateProbe(t, func(string) (unitState, error) {
			return unitState{ActiveState: c.active, SubState: "running"}, nil
		})
		m := NewManager(nil, "x3")
		lv := m.UnitLiveness("zerg-x")
		if lv.Dead() != c.dead || lv.Alive == c.dead {
			t.Errorf("ActiveState=%q：应 dead=%v，实得 %+v", c.active, c.dead, lv)
		}
	}
}

// ③ 核不到 ⇒ missing（不是「活着」；也不是把卵悄悄删掉——那是消费侧的事）。
func TestUnitLiveness_MissingWhenUnreadable(t *testing.T) {
	withUnitStateProbe(t, func(string) (unitState, error) {
		return unitState{}, errors.New("本机没有 systemctl（非 Linux）")
	})
	m := NewManager(nil, "x3")
	lv := m.UnitLiveness("zerg-x")
	if lv.State != "missing" || lv.Alive || !lv.Dead() {
		t.Fatalf("核不到单元时应落 missing（不许当活着）：%+v", lv)
	}
	if !strings.Contains(lv.Detail, "systemctl") {
		t.Errorf("理由应带探针原文（为什么核不到）：%q", lv.Detail)
	}
}

// ④ 没有单元名（裸 exec 路径）⇒ 不调探针、State 空（如实缺席，不编造一个 active）。
func TestUnitLiveness_NoUnitNameSkipsProbe(t *testing.T) {
	called := false
	withUnitStateProbe(t, func(string) (unitState, error) {
		called = true
		return unitState{ActiveState: "active"}, nil
	})
	m := NewManager(nil, "x3")
	lv := m.UnitLiveness("")
	if called {
		t.Fatal("没有单元名时不该去问 systemd")
	}
	if lv.State != "" || lv.Alive || lv.Dead() {
		t.Fatalf("裸 exec 路径应如实缺席：%+v", lv)
	}
}

// ⑤ 判词档位：dead_unit 是**新档**，与 stuck_* 同族但不同义（不许复用 stuck_*）。
func TestWatchdogVerdict_DeadUnitIsItsOwnSlot(t *testing.T) {
	if WatchdogDeadUnit != "dead_unit" {
		t.Fatalf("判词字面量必须是 dead_unit（主控/文档按它分档）：%q", WatchdogDeadUnit)
	}
	if WatchdogDeadUnit == WatchdogStuckCPUStalled || WatchdogDeadUnit == WatchdogStuckNoProgress {
		t.Fatal("「单元没了」与「单元活着但不推进」是两回事，不许共用一个档")
	}
}
