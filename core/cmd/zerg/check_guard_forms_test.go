// check_guard_forms_test.go —— check_guard_forms.go（缺口 `GAP-20260927-37` · 候选 A 仓内侧判据）的
// **两形态对照**成对负控测试。前缀 `TestGuardForms`（定向跑：`go test -run GuardForms`）。
//
// 两态自证（本单验收）：
//
//	改后 ＝ 带 argv 分支（数组元素直喂）：4 条 argv 负控真命中、2 条只读 argv 正控不误判 ⇒ 全过。
//	改前 ＝ argv 分支去掉（数组先渲染成文本再切词）：argv 负控零命中 ⇒ 本测试必红（证出缺口病症）。
//
// 「改前」两态读数用 `go test -overlay` 把 check_guard_forms.go 映射到一份 argv 分支被哑化的副本。
package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestGuardFormsArgvNegativesHit —— 4 条 argv 负控（照守卫 4 条逐条对映）**必逐条命中对应规则**。
// 改后绿；把 argv 分支去掉（guardFormsTokensArgv 退化成「渲染成文本再切词」）⇒ 本条必红。
func TestGuardFormsArgvNegativesHit(t *testing.T) {
	neg := []struct {
		name string
		argv []string
		want string
	}{
		{"git-grep", []string{"git", "grep", "-n", "x", "--", "."}, "git-grep"},
		{"sed-n", []string{"sed", "-n", "1,10p", "f"}, "sed-n"},
		{"grep-recursive", []string{"grep", "-r", "x", "."}, "grep-recursive"},
		{"direct-gate-script", []string{"python3", "scripts/gates/precommit-gates.sh"}, "direct-gate-script"},
	}
	for _, c := range neg {
		got, hit := guardFormsCheckArgv(c.argv)
		if !hit {
			t.Fatalf("argv 负控 %q 未命中（argv=%q）—— argv 形态零命中＝本缺口病症", c.name, c.argv)
		}
		if got.Key != c.want {
			t.Fatalf("argv 负控 %q 命中规则 %q（要 %q）", c.name, got.Key, c.want)
		}
	}
}

// TestGuardFormsArgvPositivesClean —— 2 条只读 argv 正控**必不误判**。
func TestGuardFormsArgvPositivesClean(t *testing.T) {
	pos := []struct {
		name string
		argv []string
	}{
		{"git-status", []string{"git", "status", "--porcelain"}},
		{"zerg-code-show", []string{"zerg", "code", "show", "f:1"}},
	}
	for _, c := range pos {
		if got, hit := guardFormsCheckArgv(c.argv); hit {
			t.Fatalf("argv 正控 %q 被误判为危险（规则 %q · argv=%q）", c.name, got.Key, c.argv)
		}
	}
}

// TestGuardFormsTwoFormEquivalence —— 每组靶子 shell 文本形态与 argv 形态判定**逐字同读**。
func TestGuardFormsTwoFormEquivalence(t *testing.T) {
	if len(guardFormsPairs) < 6 {
		t.Fatalf("canonical 靶子要 >= 6（4 负控 + 2 正控），得 %d", len(guardFormsPairs))
	}
	for _, p := range guardFormsPairs {
		gs, hs := guardFormsCheckShell(p.Shell)
		ga, ha := guardFormsCheckArgv(p.Argv)
		if hs != ha || gs.Key != ga.Key {
			t.Fatalf("两形态分歧 %q：shell=%v/%q argv=%v/%q", p.Name, hs, gs.Key, ha, ga.Key)
		}
	}
}

// TestGuardFormsDiseaseLegacyPath —— 证出病症：**改前那条错路**（argv 渲染成文本再按 shell 切词）
// 对 4 条负控零命中。这是本缺口病症的进程内直证（不依赖 overlay）。
func TestGuardFormsDiseaseLegacyPath(t *testing.T) {
	for _, p := range guardFormsPairs {
		if p.WantDanger == "" {
			continue
		}
		if got, hit := guardFormsCheckArgvAsShell(p.Argv); hit {
			t.Fatalf("改前错路 %q 竟命中 %q —— 该路本应零命中（缺口病症）", p.Name, got.Key)
		}
	}
}

// TestGuardFormsSelfTestGreen —— `--self-test` rc=0 ＋ 结语逐字 ＋ 无红格。
func TestGuardFormsSelfTestGreen(t *testing.T) {
	rc, out, errs := gftRun("--self-test")
	if rc != 0 {
		t.Fatalf("--self-test 应 rc=0，得 rc=%d\n%s\nstderr=%q", rc, out, errs)
	}
	if !strings.Contains(out, "自检 PASS") || strings.Contains(out, "✗") {
		t.Fatalf("自检结语/红格不符：%q", out)
	}
}

// TestGuardFormsEntryFaces —— `--argv` JSON 喂料（命中退 1 / 干净退 0）＋ `--check` 文本喂料
// ＋ 未知旗标 rc=2 ＋ `--argv` 缺值 rc=2 ＋ `--argv` 非 JSON rc=2。
func TestGuardFormsEntryFaces(t *testing.T) {
	if rc, out, _ := gftRun("--argv", `["git","grep","-n","x","--","."]`); rc != 1 || !strings.Contains(out, "git-grep") {
		t.Fatalf("--argv 危险形态应 rc=1 且点名 git-grep：rc=%d out=%q", rc, out)
	}
	if rc, out, _ := gftRun("--argv", `["git","status","--porcelain"]`); rc != 0 || !strings.Contains(out, "干净") {
		t.Fatalf("--argv 只读形态应 rc=0 且干净：rc=%d out=%q", rc, out)
	}
	if rc, out, _ := gftRun("--check", "sed -n 1,10p f"); rc != 1 || !strings.Contains(out, "sed-n") {
		t.Fatalf("--check 文本危险形态应 rc=1 且点名 sed-n：rc=%d out=%q", rc, out)
	}
	if rc, _, errs := gftRun("--argv", "not-json"); rc != 2 || !strings.Contains(errs, "rc=2") {
		t.Fatalf("--argv 非 JSON 应 rc=2：rc=%d err=%q", rc, errs)
	}
	if rc, _, _ := gftRun("--bogus"); rc != 2 {
		t.Fatalf("未知旗标应 rc=2，得 %d", rc)
	}
	if rc, _, _ := gftRun("--argv"); rc != 2 {
		t.Fatalf("--argv 缺值应 rc=2，得 %d", rc)
	}
}

// TestGuardFormsDefaultReport —— 默认档：两形态处处同读 ⇒ rc=0；危险规则表齐 4 条。
func TestGuardFormsDefaultReport(t *testing.T) {
	rc, out, _ := gftRun()
	if rc != 0 {
		t.Fatalf("默认档应 rc=0（两形态同读），得 %d\n%s", rc, out)
	}
	for _, k := range []string{"git-grep", "sed-n", "grep-recursive", "direct-gate-script"} {
		if !strings.Contains(out, k) {
			t.Fatalf("默认档应含规则 %q：%q", k, out)
		}
	}
	if len(guardFormsRules) != 4 {
		t.Fatalf("危险规则应恰 4 条，得 %d", len(guardFormsRules))
	}
}

// gftRun —— 调入口（三面：rc/stdout/stderr）。
func gftRun(args ...string) (int, string, string) {
	var o, e bytes.Buffer
	rc := checkGuardFormsGateMain(&o, &e, args)
	return rc, o.String(), e.String()
}
