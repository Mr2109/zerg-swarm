// eggmax_override_test.go —— `GAP-20260926-201`（全通路 · 单卵时限）**子端侧**自检。
//
// 判据（本单口径 ④ · 子端 half）：
//
//	正控 —— 逐枚覆盖（flag）> env `ZERG_EGG_MAX_S` > 现默认 0（三级优先级逐格）；
//	负控 —— `ParseEggMaxS` 对非法值**报错**（调用方据此 400 明报拒收 · ✗ 不静默回落默认）。
//
// 白盒 · `package backend`（判据要直接驱动那一格）。不真起引擎、不真联网。
package backend

import (
	"testing"
	"time"
)

// TestParseEggMaxS —— 秒口径逐格：空⇒0 · 数字⇒秒 · 非法/非正⇒报错。
func TestParseEggMaxS(t *testing.T) {
	ok := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"   ", 0},
		{"300", 300 * time.Second},
		{"1.5", 1500 * time.Millisecond},
	}
	for _, c := range ok {
		got, err := ParseEggMaxS(c.in)
		if err != nil || got != c.want {
			t.Fatalf("ParseEggMaxS(%q) = %v,%v 要 %v,nil", c.in, got, err, c.want)
		}
	}
	// 负控：非法 / 非正 ⇒ 报错（✗ 不静默回落）
	for _, bad := range []string{"abc", "0", "-5", "5x", "1e"} {
		if _, err := ParseEggMaxS(bad); err == nil {
			t.Fatalf("ParseEggMaxS(%q) 该报错（✗ 不静默回落默认）", bad)
		}
	}
}

// TestResolveEggMaxPriority —— 优先级逐格：逐枚覆盖 > env > 0；覆盖>0 时自动开看门狗（否则无牙）。
func TestResolveEggMaxPriority(t *testing.T) {
	env := WatchdogConfig{Enabled: false, EggMax: 120 * time.Second}
	// 无覆盖 ⇒ 保持 env（且不擅自开看门狗）
	if c := resolveEggMax(env, 0); c.EggMax != 120*time.Second || c.Enabled {
		t.Fatalf("无覆盖该保持 env EggMax=120s Enabled=false，得 %+v", c)
	}
	// 覆盖 > env ⇒ 覆盖胜 + 自动开看门狗
	c := resolveEggMax(env, 5*time.Second)
	if c.EggMax != 5*time.Second || !c.Enabled {
		t.Fatalf("覆盖该胜 env 且 Enabled=true，得 %+v", c)
	}
	// env=0（未设）＋ 覆盖 ⇒ 覆盖胜（穿默认 0）
	c2 := resolveEggMax(WatchdogConfig{}, 7*time.Second)
	if c2.EggMax != 7*time.Second || !c2.Enabled {
		t.Fatalf("覆盖该穿默认 0，得 %+v", c2)
	}
}

// TestManagerEggMaxOverride —— Manager 的逐枚覆盖：设 / 读 / 清 / 缺省（nil map 也安全）。
func TestManagerEggMaxOverride(t *testing.T) {
	m := &Manager{} // 零值可用（eggMaxOverride 为 nil ⇒ 读安全、写自建）
	if got := m.eggMaxOverrideFor("m1"); got != 0 {
		t.Fatalf("未设该为 0，得 %v", got)
	}
	m.SetEggMaxOverride("m1", 5*time.Second)
	if got := m.eggMaxOverrideFor("m1"); got != 5*time.Second {
		t.Fatalf("设后该为 5s，得 %v", got)
	}
	if got := m.eggMaxOverrideFor("m2"); got != 0 {
		t.Fatalf("别的模型该为 0，得 %v", got)
	}
	m.SetEggMaxOverride("m1", 0) // 清除 ⇒ 回落 env
	if got := m.eggMaxOverrideFor("m1"); got != 0 {
		t.Fatalf("清除后该为 0，得 %v", got)
	}
}
