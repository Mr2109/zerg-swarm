// check_verdict_record_test.go —— Go 真搬的**成对负控**定向单测（M2 · laneB475）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckVerdictRecordGatePort（定向：go test -run TestCheckVerdictRecordGatePort）。
//
// 夹具全部造在 `t.TempDir()` 里（不碰真仓 / ✗ 不碰真状态目录）；`/tmp` 形态按自指纪律拆开拼。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func vrtRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkVerdictRecordGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

const vrtGate = `#!/usr/bin/env bash
run_suite() {
  case "$1" in
    gates)
      add_step gates "门甲 夹具一" tri "${REPO_ROOT}" "python3 scripts/gates/check-a.py"
      add_step gates "门乙 夹具二" tri-report "${REPO_ROOT}" "python3 scripts/gates/check-b.py"
      ;;
  esac
}
`

// vrtMk 造一枚仓外夹具（门表 + 两件基线 + 判据件面），返 (仓根, 状态目录)。
func vrtMk(t *testing.T, gate string, withMatrix bool, judgeName, judgeBody string) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "core", "cmd", "zerg", "testdata"), 0o777); err != nil {
		t.Fatal(err)
	}
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(body), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	write("scripts/gates/precommit-gates.sh", gate)
	write("scripts/gates/cli-contract-baseline.json", `{"commands": 3, "mustfail_cases": 5, "blanks": 0}`)
	if withMatrix {
		write("core/cmd/zerg/testdata/cli-matrix.json", `{"cases": [{"command": "zerg a", "want_rc": 2}]}`)
	}
	write(filepath.ToSlash(filepath.Join("scripts", "gates", judgeName)), judgeBody)
	return root, filepath.Join(root, "st")
}

func TestCheckVerdictRecordGatePort(t *testing.T) {
	tmp := "/" + "tmp" + "/"

	// ── 正控①：全绿夹具 ⇒ rc=0（落点面 0 命中 + 两遍现算逐字相同）────────────
	root, st := vrtMk(t, vrtGate, true, "check-a.py", "print('ok')\n")
	if rc, out, _ := vrtRun(t, "--root", root, "--state-dir", st); rc != 0 {
		t.Fatalf("正控①该绿没绿：rc=%d\n%s", rc, out)
	} else if !strings.Contains(out, "落点面 0 命中") {
		t.Fatalf("正控①缺绿档逐字：%s", out)
	}

	// ── 负控②：判据件把结论写 `/tmp`（命令位）⇒ rc=1 + 点名「件:行」──────────
	root2, st2 := vrtMk(t, vrtGate, true, "check-a.py", "import subprocess\nsubprocess.run(\"git status > "+tmp+"st.txt\", shell=True)\n")
	rc2, out2, _ := vrtRun(t, "--root", root2, "--state-dir", st2)
	if rc2 != 1 {
		t.Fatalf("负控②该红没红：rc=%d\n%s", rc2, out2)
	}
	if !strings.Contains(out2, "scripts/gates/check-a.py:2") {
		t.Fatalf("负控②没点名到件:行：%s", out2)
	}

	// ── 真话负控③：同一处落点挪进仓内 ⇒ 回绿（证红红在那一处）──────────────
	root3, st3 := vrtMk(t, vrtGate, true, "check-a.py", "import subprocess\nsubprocess.run(\"git status > scripts/gates/x.txt\", shell=True)\n")
	if rc, _, _ := vrtRun(t, "--root", root3, "--state-dir", st3); rc != 0 {
		t.Fatalf("真话负控③该绿没绿：rc=%d", rc)
	}

	// ── 负控④：空转（0 条 add_step）⇒ rc=2 不给结论 ──────────────────────────
	root4, st4 := vrtMk(t, "#!/usr/bin/env bash\n# 空\n", true, "check-a.py", "print(1)\n")
	if rc, _, _ := vrtRun(t, "--root", root4, "--state-dir", st4); rc != 2 {
		t.Fatalf("负控④空转该 rc=2：rc=%d", rc)
	}

	// ── 负控⑤：基线件不在盘 ⇒ rc=2（不拿「读不到」当「没问题」）────────────
	root5, st5 := vrtMk(t, vrtGate, false, "check-a.py", "print(1)\n")
	if rc, _, _ := vrtRun(t, "--root", root5, "--state-dir", st5); rc != 2 {
		t.Fatalf("负控⑤缺基线该 rc=2：rc=%d", rc)
	}

	// ── 真话负控⑥：自豁免（同名件=本件）里的形态字面量不进扫面 ⇒ rc=0 ────────
	root6, st6 := vrtMk(t, vrtGate, true, "check-a.py", "print(1)\n")
	_ = os.WriteFile(filepath.Join(root6, "scripts", "gates", "check-verdict-record.py"),
		[]byte("PAT = r\"> "+tmp+"x\"\n"), 0o666)
	rc6, out6, _ := vrtRun(t, "--root", root6, "--state-dir", st6)
	if rc6 != 0 || !strings.Contains(out6, "自豁免") {
		t.Fatalf("真话负控⑥自豁免没起效：rc=%d\n%s", rc6, out6)
	}

	// ── 正控⑦：夹具合成标本串 ⇒ 走登记豁免 ⇒ rc=0（逐条打印理由 + 悬空不判红）──
	spec := "NEG_CASES = [\n    (\"b\", \"x\", \"n\", \"deploy.sh\",\n     '#!/usr/bin/env bash\\ncat > " + tmp + "x" + "3.service << UNIT_EOF\\n', \"UNIT-1\"),\n]\n"
	root7, st7 := vrtMk(t, vrtGate, true, "check-token-argv.py", spec)
	rc7, out7, _ := vrtRun(t, "--root", root7, "--state-dir", st7)
	if rc7 != 0 || !strings.Contains(out7, "夹具登记豁免") || !strings.Contains(out7, "悬空登记") {
		t.Fatalf("正控⑦登记豁免没起效：rc=%d\n%s", rc7, out7)
	}

	// ── 负控⑧：**同一登记件**里的真写 ⇒ 登记盖不住 ⇒ 照旧红（证未放宽真判据）──
	root8, st8 := vrtMk(t, vrtGate, true, "check-token-argv.py", "import subprocess\nsubprocess.run(\"git status > "+tmp+"st.txt\", shell=True)\n")
	if rc, _, _ := vrtRun(t, "--root", root8, "--state-dir", st8); rc != 1 {
		t.Fatalf("负控⑧登记件真写该红：rc=%d", rc)
	}

	// ── 指纹面：只改排版 ⇒ 不变；改内容 ⇒ 变 ─────────────────────────────────
	itA, _ := vrSubjectItems(root)
	itB, _ := vrSubjectItems(root3)
	if vrFingerprint(itA, vrSubject) != vrFingerprint(itB, vrSubject) {
		t.Fatalf("指纹面对同内容（不同落点排版）不该变")
	}
	root9, _ := vrtMk(t, strings.Replace(vrtGate, "门甲 夹具一", "门甲 夹具一改", 1), true, "check-a.py", "print(1)\n")
	it9, _ := vrSubjectItems(root9)
	if vrFingerprint(it9, vrSubject) == vrFingerprint(itA, vrSubject) {
		t.Fatalf("指纹面：改内容（步名）却不变 ⇒ 判据被写死")
	}

	// ── 记录/可比性：同一夹具两跑 ⇒ compare rc=0；不同内容两跑 ⇒ rc=2；一跑 ⇒ rc=2 ─
	rcA, _ := vrtMk(t, vrtGate, true, "check-a.py", "print(1)\n")
	rcB, _ := vrtMk(t, strings.Replace(vrtGate, "门甲 夹具一", "门甲 夹具一改", 1), true, "check-a.py", "print(1)\n")
	sts := filepath.Join(t.TempDir(), "state")
	if rc, _, _ := vrtRun(t, "--root", rcA, "--state-dir", sts, "--record"); rc != 0 {
		t.Fatalf("--record 首跑该 rc=0：rc=%d", rc)
	}
	if rc, _, _ := vrtRun(t, "--root", rcA, "--state-dir", sts, "--record"); rc != 0 {
		t.Fatalf("--record 二跑该 rc=0：rc=%d", rc)
	}
	if rc, out, _ := vrtRun(t, "--state-dir", sts, "--compare"); rc != 0 || !strings.Contains(out, "指纹相同") {
		t.Fatalf("compare 同一改动该 rc=0：rc=%d\n%s", rc, out)
	}
	stD := filepath.Join(t.TempDir(), "state")
	_, _, _ = vrtRun(t, "--root", rcA, "--state-dir", stD, "--record")
	_, _, _ = vrtRun(t, "--root", rcB, "--state-dir", stD, "--record")
	if rc, out, _ := vrtRun(t, "--state-dir", stD, "--compare"); rc != 2 || !strings.Contains(out, "仓变了，结果不可比") {
		t.Fatalf("compare 不同指纹该 rc=2：rc=%d\n%s", rc, out)
	}
	st1 := filepath.Join(t.TempDir(), "state")
	_, _, _ = vrtRun(t, "--root", rcA, "--state-dir", st1, "--record")
	if rc, out, _ := vrtRun(t, "--state-dir", st1, "--compare"); rc != 2 || !strings.Contains(out, "不足两跑") {
		t.Fatalf("compare 不足两跑该 rc=2：rc=%d\n%s", rc, out)
	}

	// ── JSON null 面：缺键 ⟷ 显式 null 在 py `.get` 语义下同归 None ⇒ 两态逐字相同 ─
	rootN, _ := vrtMk(t, vrtGate, true, "check-a.py", "print(1)\n")
	stM := filepath.Join(t.TempDir(), "stm")
	stU := filepath.Join(t.TempDir(), "stu")
	_ = os.MkdirAll(stM, 0o777)
	_ = os.MkdirAll(stU, 0o777)
	_ = os.WriteFile(filepath.Join(stM, "verdict_records.jsonl"), []byte(`{"subject": "gate-face"}`+"\n"), 0o666)
	_ = os.WriteFile(filepath.Join(stU, "verdict_records.jsonl"), []byte(`{"subject": "gate-face", "ts": null, "fingerprint": null, "verdict": null}`+"\n"), 0o666)
	_, outM, _ := vrtRun(t, "--root", rootN, "--state-dir", stM)
	_, outU, _ := vrtRun(t, "--root", rootN, "--state-dir", stU)
	if strings.ReplaceAll(outM, stM, "<ST>") != strings.ReplaceAll(outU, stU, "<ST>") {
		t.Fatalf("JSON null 面：缺键 ⟷ 显式 null 两态不该分叉\n缺键=%q\nnull=%q", outM, outU)
	}

	// ── --json 机读面：形状自证（供机器消费者）───────────────────────────────
	if rc, out, _ := vrtRun(t, "--root", root, "--state-dir", st, "--json"); rc != 0 ||
		!strings.Contains(out, `"schema": "zerg/v1"`) || !strings.Contains(out, `"landing_hit_count": 0`) {
		t.Fatalf("--json 形状不合：rc=%d\n%s", rc, out)
	}

	// ── 自检档：进程内复用入口 · 断言「全过」+ rc=0（自检自回收见文件系统面）──
	rcST, outST, _ := vrtRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检结论: 全过") {
		t.Fatalf("--self-test 该全过：rc=%d\n%s", rcST, outST)
	}
}
