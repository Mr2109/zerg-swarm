package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// affected_steps_test.go —— `affectedStepsGateMain` 的**成对负控**用例（M2 路 A 续5）。
//
// 纪律：正控（声明齐 + 有命中 ⇒ rc=0 + 点名 ▶）+ 负控（少声明 ⇒ rc=2「静默漏选」·
// 缺指纹/基线 ⇒ rc=2「取不到输入基线」· 步表件缺 ⇒ rc=2 · 集合不等 ⇒ rc=2）
// 一律落 `t.TempDir()` 夹具，**不碰真目标**；负控有牙（哑化对应判据分支 ⇒ 用例 FAIL）。

const asTestList = "── 步骤清单（scope 模式 名称）──\n" +
	"   go     rc     a-step\n" +
	"   rust   empty  b-step\n" +
	"   docs   tri    c-step\n" +
	"共 3 步\n"

func asTestWrite(t *testing.T, p, body string) string {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败 %s：%v", p, err)
	}
	return p
}

func asTestRun(argv []string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := affectedStepsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestAffectedStepsGatePort(t *testing.T) {
	dir := t.TempDir()
	fp := strings.Repeat("f", 64)

	list := asTestWrite(t, filepath.Join(dir, "list.txt"), asTestList)
	chg := asTestWrite(t, filepath.Join(dir, "chg.txt"), "core/cmd/x.go\n")
	chg0 := asTestWrite(t, filepath.Join(dir, "chg0.txt"), "")
	chg7 := asTestWrite(t, filepath.Join(dir, "chg7.txt"), "deploy/unknown.yaml\n")

	m1 := asTestWrite(t, filepath.Join(dir, "m1.json"),
		`{"input_fingerprint":"`+fp+`","source_head":"deadbeef",`+
			`"faces":{"core":["a-step"],"agent":["b-step"],"docs":["c-step"]},`+
			`"files":[{"pattern":"core/**","kind":"prefix","steps":["a-step"]},`+
			`{"pattern":"ui/**","kind":"prefix","steps":["b-step"]},`+
			`{"pattern":"docs/**","kind":"prefix","steps":["c-step"]}]}`)
	// 负控夹具：少声明一步（D=2, N=3）
	m2 := asTestWrite(t, filepath.Join(dir, "m2.json"),
		`{"input_fingerprint":"`+fp+`","source_head":"deadbeef",`+
			`"faces":{"core":["a-step"],"agent":["b-step"]},`+
			`"files":[{"pattern":"core/**","kind":"prefix","steps":["a-step"]}]}`)
	// 负控夹具：D==N 但集合不等（步名改名）
	m3 := asTestWrite(t, filepath.Join(dir, "m3.json"),
		`{"input_fingerprint":"`+fp+`","source_head":"deadbeef",`+
			`"faces":{"core":["a-step"],"agent":["b-step"],"docs":["zzz"]},"files":[]}`)
	// 负控夹具：缺指纹/基线
	m4 := asTestWrite(t, filepath.Join(dir, "m4.json"),
		`{"faces":{"core":["a-step"],"agent":["b-step"],"docs":["c-step"]},"files":[]}`)

	// ① 正控：声明齐 + 有命中 ⇒ rc=0 且点名受影响步
	rc, out, errs := asTestRun([]string{"--list", list, "--map", m1, "--changed", chg})
	if rc != 0 {
		t.Fatalf("① 正控期望 rc=0，实得 rc=%d（stderr=%s）", rc, errs)
	}
	if !strings.Contains(out, "▶ a-step") {
		t.Fatalf("① 正控未点名受影响步：%s", out)
	}
	if !strings.Contains(out, "受影响的步集：1 步") {
		t.Fatalf("① 正控步集计数不对：%s", out)
	}

	// ② 负控：少声明一步 ⇒ rc=2 且点名「静默漏选」
	rc, out, _ = asTestRun([]string{"--list", list, "--map", m2, "--changed", chg})
	if rc != 2 {
		t.Fatalf("② 负控（少声明）期望 rc=2，实得 rc=%d", rc)
	}
	if !strings.Contains(out, "静默漏选") {
		t.Fatalf("② 负控未点名「静默漏选」：%s", out)
	}

	// ③ 负控：D==N 但集合不等 ⇒ rc=2
	rc, out, _ = asTestRun([]string{"--list", list, "--map", m3, "--changed", chg})
	if rc != 2 {
		t.Fatalf("③ 负控（集合不等）期望 rc=2，实得 rc=%d", rc)
	}
	if !strings.Contains(out, "命名集合 ≠ 表集合") {
		t.Fatalf("③ 负控未点名集合不等：%s", out)
	}

	// ④ 负控：缺指纹/基线 ⇒ rc=2 且点名「取不到输入基线」
	rc, out, _ = asTestRun([]string{"--list", list, "--map", m4, "--changed", chg})
	if rc != 2 {
		t.Fatalf("④ 负控（缺指纹）期望 rc=2，实得 rc=%d", rc)
	}
	if !strings.Contains(out, "取不到输入基线") {
		t.Fatalf("④ 负控未点名「取不到输入基线」：%s", out)
	}

	// ⑤ 负控：未声明件 ⇒ 报全量（rc=0 · 受影响 = N = 3）
	rc, out, _ = asTestRun([]string{"--list", list, "--map", m1, "--changed", chg7})
	if rc != 0 {
		t.Fatalf("⑤ 未声明件期望 rc=0，实得 rc=%d", rc)
	}
	if !strings.Contains(out, "未声明件 1 件") || !strings.Contains(out, "受影响的步集：3 步") {
		t.Fatalf("⑤ 未声明件未按全量报：%s", out)
	}

	// ⑥ 负控：空改动 ⇒ rc=0 且「受影响 0 步」
	rc, out, _ = asTestRun([]string{"--list", list, "--map", m1, "--changed", chg0})
	if rc != 0 || !strings.Contains(out, "受影响 0 步") {
		t.Fatalf("⑥ 空改动期望 rc=0 + 「受影响 0 步」，实得 rc=%d：%s", rc, out)
	}

	// ⑦ 负控：步表件缺 / 声明表缺 / 无参 ⇒ rc=2
	if rc, _, _ = asTestRun([]string{"--list", filepath.Join(dir, "nope.txt"), "--map", m1}); rc != 2 {
		t.Fatalf("⑦a 步表件缺期望 rc=2，实得 rc=%d", rc)
	}
	if rc, _, _ = asTestRun([]string{"--list", list, "--map", filepath.Join(dir, "nope.json")}); rc != 2 {
		t.Fatalf("⑦b 声明表缺期望 rc=2，实得 rc=%d", rc)
	}
	if rc, _, errs = asTestRun(nil); rc != 2 || !strings.Contains(errs, "用法：affected-steps.py") {
		t.Fatalf("⑦c 无参期望 rc=2 + 用法行，实得 rc=%d stderr=%s", rc, errs)
	}

	// ⑧ 正控：--json 合法面（rc=0 · 机器面 kind=AffectedSteps）
	rc, out, _ = asTestRun([]string{"--list", list, "--map", m1, "--changed", chg, "--json"})
	if rc != 0 || !strings.Contains(out, `"kind": "AffectedSteps"`) || !strings.Contains(out, `"rc": 0`) {
		t.Fatalf("⑧ --json 面不对 rc=%d out=%s", rc, out)
	}

	// ⑨ 自检档 ⇒ rc=0（7 控全过）
	if rc, out, _ = asTestRun([]string{"--self-test"}); rc != 0 || !strings.Contains(out, "自检通过") {
		t.Fatalf("⑨ --self-test 期望 rc=0 + 「自检通过」，实得 rc=%d out=%s", rc, out)
	}
}
