// family_model_verify_test.go —— `zerg model verify <模型@机>`（缺口 GAP-20260926-04）的判据机检。
//
// 本件是这条新命令**自己的自检**（口径：新命令要配一格自检 · 正控 + 至少两条负控，且负控要能真红）。
// 判面 = 四方的**纯核心**（`mvVerdict` / `mvExit` / `mvCtxOfCmd`）—— 不碰真盘、不联网（把真源读数
// 与判定分开 ⇒ 判据能用合成输入钉死，负控红即真红）。
//
// 三档闭集 + 三态分离（拍板口径）：一致 / 不一致 / 缺席；读不到(8) ≠ 缺席 ≠ 不一致(1)。
package main

import "testing"

// TestModelVerifyConsistent —— **正控**：三方 ctx 全等（且都在）⇒ verdict=一致 · 退码 0。
func TestModelVerifyConsistent(t *testing.T) {
	vals := []int{65536, 65536, 65536}
	present := []bool{true, true, true}
	if v := mvVerdict(vals, present); v != mvAgree {
		t.Fatalf("正控：期望 verdict=%q，实得 %q", mvAgree, v)
	}
	if rc := mvExit(mvAgree, false); rc != exitOK {
		t.Fatalf("正控：期望退码 %d，实得 %d", exitOK, rc)
	}
}

// TestModelVerifyDisagreement —— **负控①（不一致档 · 真红）**：两方 ctx 不等 ⇒ verdict=不一致 · 退码 1。
func TestModelVerifyDisagreement(t *testing.T) {
	vals := []int{32768, 131072, 131072}
	present := []bool{true, true, true}
	if v := mvVerdict(vals, present); v != mvDisagree {
		t.Fatalf("负控①：期望 verdict=%q，实得 %q", mvDisagree, v)
	}
	rc := mvExit(mvDisagree, false)
	if rc != exitFail {
		t.Fatalf("负控①：期望退码 %d，实得 %d —— 判不一致必须退 1", exitFail, rc)
	}
	if rc == exitOK {
		t.Fatal("负控①：判不一致却退 0 —— 判据无牙（正是要拦的假绿）")
	}
}

// TestModelVerifyAbsent —— **负控②（缺席档 · 真红）**：有 ctx 读数的方不足两方 ⇒ verdict=缺席 · 退码 1。
// 缺席**必须**与「不一致」分成两档（口径：缺席 ≠ 不一致）—— 用只有一方在的输入钉它。
func TestModelVerifyAbsent(t *testing.T) {
	vals := []int{65536, 0, 0}
	present := []bool{true, false, false}
	v := mvVerdict(vals, present)
	if v != mvAbsent {
		t.Fatalf("负控②：期望 verdict=%q（只有一方在 ⇒ 没得比），实得 %q", mvAbsent, v)
	}
	if v == mvDisagree {
		t.Fatal("负控②：把「缺席」误判成「不一致」—— 三态未分离")
	}
	if rc := mvExit(v, false); rc != exitFail {
		t.Fatalf("负控②：期望退码 %d，实得 %d", exitFail, rc)
	}
}

// TestModelVerifyUnreadable —— **负控③（读不到档 · 真红）**：必需件读不到 ⇒ 退码 8（**不给结论**）。
// 关键：读不到**不得**被当成「一致」（0）或「不一致」（1）—— 三态分离的第三条边。
func TestModelVerifyUnreadable(t *testing.T) {
	if rc := mvExit(mvAgree, true); rc != exitBlocked {
		t.Fatalf("负控③：读不到时即使 ctx 一致也必须退 %d，实得 %d", exitBlocked, rc)
	}
	if rc := mvExit(mvDisagree, true); rc != exitBlocked {
		t.Fatalf("负控③：读不到优先于不一致 ⇒ 期望退 %d，实得 %d", exitBlocked, rc)
	}
	if rc := mvExit(mvAgree, true); rc == exitOK {
		t.Fatal("负控③：读不到却退 0 —— 「读不到不许当健康」被破")
	}
}

// TestModelVerifyCtxParse —— 从 `cmd` 两种名册写法里取 ctx（数组形 `"-c", "65536"` 与字符串形 `-c 65536`）。
func TestModelVerifyCtxParse(t *testing.T) {
	cases := []struct {
		cmd  string
		want int
		ok   bool
	}{
		{`["/opt/homebrew/bin/llama-server", "-m", "{file}", "-c", "131072", "-ngl", "999"]`, 131072, true},
		{`/opt/homebrew/bin/llama-server -m {file} --host 127.0.0.1 --port {port} -c 65536 -ngl 999`, 65536, true},
		{`/opt/homebrew/bin/llama-server -m {file} --port {port}`, 0, false},
		{``, 0, false},
	}
	for i, c := range cases {
		got, ok := mvCtxOfCmd(c.cmd)
		if ok != c.ok || (ok && got != c.want) {
			t.Fatalf("第 %d 例：cmd=%q 期望 (%d,%t)，实得 (%d,%t)", i+1, c.cmd, c.want, c.ok, got, ok)
		}
	}
}
