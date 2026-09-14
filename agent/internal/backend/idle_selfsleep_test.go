// idle_selfsleep_test.go —— P6（R6）空闲自退透传的回归。
//
// 重点：默认**完全不介入**（不配置就不改变任何既有行为）；ds4 不塞它不认的开关。
package backend

import "testing"

func TestApplyIdleSelfSleep(t *testing.T) {
	base := []string{"--model", "/data/models/k2/k2horizon-q4_k_m.gguf", "--port", "9401"}

	t.Run("未配置 ⇒ 原样返回（默认关闭，绝不改变既有行为）", func(t *testing.T) {
		t.Setenv(EnvIdleSelfSleepS, "")
		got := applyIdleSelfSleep(base, "llama.cpp")
		if len(got) != len(base) {
			t.Fatalf("未配置时不该追加参数，实得 %v", got)
		}
	})

	t.Run("配了 ⇒ 追加真开关名 --sleep-idle-seconds", func(t *testing.T) {
		t.Setenv(EnvIdleSelfSleepS, "600")
		got := applyIdleSelfSleep(base, "llama.cpp")
		if v := argValue(got, "--sleep-idle-seconds"); v != "600" {
			t.Fatalf("应追加 --sleep-idle-seconds 600，实得 %q（参数：%v）", v, got)
		}
		// 反面：**绝不能**写成 --timeout（真机核实过：那只是 read/write 超时）
		if v := argValue(got, "--timeout"); v != "" {
			t.Fatalf("不得使用 --timeout（它只是 read/write 超时，不是空闲自退），实得 %q", v)
		}
	})

	t.Run("低于下限 ⇒ 抬到 60 秒（防反复入睡唤醒）", func(t *testing.T) {
		t.Setenv(EnvIdleSelfSleepS, "5")
		got := applyIdleSelfSleep(base, "llama.cpp")
		if v := argValue(got, "--sleep-idle-seconds"); v != "60" {
			t.Fatalf("下限应为 60，实得 %q", v)
		}
	})

	t.Run("ds4-server ⇒ 不动（它没有这个开关）", func(t *testing.T) {
		t.Setenv(EnvIdleSelfSleepS, "600")
		got := applyIdleSelfSleep(base, "ds4-server")
		if len(got) != len(base) {
			t.Fatalf("ds4 不该被追加参数，实得 %v", got)
		}
	})

	t.Run("幂等：已有该开关 ⇒ 不重复追加", func(t *testing.T) {
		t.Setenv(EnvIdleSelfSleepS, "600")
		with := append(append([]string{}, base...), "--sleep-idle-seconds", "300")
		got := applyIdleSelfSleep(with, "llama.cpp")
		if n := countFlag(got, "--sleep-idle-seconds"); n != 1 {
			t.Fatalf("不该重复追加，实得 %d 次：%v", n, got)
		}
		if v := argValue(got, "--sleep-idle-seconds"); v != "300" {
			t.Fatalf("应尊重已有配置 300，实得 %q", v)
		}
	})

	t.Run("不就地改写调用方的切片", func(t *testing.T) {
		t.Setenv(EnvIdleSelfSleepS, "600")
		in := append([]string{}, base...)
		_ = applyIdleSelfSleep(in, "llama.cpp")
		if len(in) != len(base) {
			t.Fatalf("原切片被就地改写：%v", in)
		}
	})
}

func countFlag(args []string, flag string) int {
	n := 0
	for _, a := range args {
		if a == flag {
			n++
		}
	}
	return n
}
