// check_gate_coverage_test.go —— check_gate_coverage.go 的成对负控单测件（M2 · 甲案）。
//
// 设计口径：每条用例都**对着一处真判据**，且**哑化那处判据 ⇒ 用例必 FAIL（带行号）**——
// 不是恒绿。核心负控 = 「漏一个目录必红」「命中超基线必红」「空转/白名单缺理由 ⇒ rc=2」
// 「计数自证被置假 ⇒ rc=2」「--json stdout 必须纯」。
//
// 跑法：go test ./cmd/zerg -run TestCheckGateCoverage -count=1
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func cgcTWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(cgcPyDirname(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cgcTGate(t *testing.T, root string, steps [][2]string, extra string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\nset -u\n")
	b.WriteString("REPO_ROOT=\"$(cd \"$(dirname \"${BASH_SOURCE[0]}\")/../..\" && pwd)\"\n")
	b.WriteString("STEP_SCOPE=(); STEP_NAME=(); STEP_MODE=(); STEP_DIR=(); STEP_CMD=()\n")
	b.WriteString("add_step() { STEP_SCOPE+=(\"$1\"); STEP_NAME+=(\"$2\"); STEP_MODE+=(\"$3\"); STEP_DIR+=(\"$4\"); STEP_CMD+=(\"$5\"); }\n")
	b.WriteString("clear_steps() { STEP_SCOPE=(); STEP_NAME=(); STEP_MODE=(); STEP_DIR=(); STEP_CMD=(); }\n")
	b.WriteString("build_steps() {\n")
	for _, s := range steps {
		n := s[0]
		if n == "" {
			n = "root"
		}
		b.WriteString(fmt.Sprintf("  add_step fixture \"步骤 %s\" rc \"${REPO_ROOT}/%s\" \"%s\"\n", n, s[0], s[1]))
	}
	b.WriteString("}\nmain() { clear_steps; build_steps; }\nmain \"$@\"\n")
	b.WriteString(extra)
	cgcTWrite(t, cgcPyJoin(root, "scripts", "gates", "precommit-gates.sh"), b.String())
}

// cgcTFixture 造一个最小仓；cfgRaw 非空时原样落 config（用于造缺 reason / 缺 date / 未知键等）。
func cgcTFixture(t *testing.T, name string, steps [][2]string, manifests, scripts []string, baseline int, cfgRaw string) string {
	t.Helper()
	root := cgcPyJoin(t.TempDir(), name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if manifests == nil {
		manifests = []string{"core/go.mod"}
	}
	if scripts == nil {
		scripts = []string{"scripts/ok.py"}
	}
	for _, m := range manifests {
		c := "[package]\n"
		if strings.HasSuffix(m, "go.mod") {
			c = "module fixture\n"
		}
		cgcTWrite(t, cgcPyJoin(root, m), c)
	}
	for _, s := range scripts {
		cgcTWrite(t, cgcPyJoin(root, s), "#!/usr/bin/env python3\nprint('fixture')\n")
	}
	cgcTGate(t, root, steps, "")
	if cfgRaw == "" {
		doc := map[string]any{"schema": "gate-coverage/1", "baseline": baseline,
			"whitelist": map[string]any{"build_dirs": []any{}, "scripts": []any{}}}
		b, _ := json.Marshal(doc)
		cgcTWrite(t, cgcPyJoin(root, "scripts", "gates", "gate-coverage.config.json"), string(b))
	} else {
		cgcTWrite(t, cgcPyJoin(root, "scripts", "gates", "gate-coverage.config.json"), cfgRaw)
	}
	return root
}

func cgcTRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var so, se bytes.Buffer
	rc := cgcRunMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// cgcTGitify：给夹具建真 git 索引（不提交）—— ④「新件 ⟷ 存量债」判据的前提。
func cgcTGitify(t *testing.T, root string) {
	t.Helper()
	for _, a := range [][]string{{"git", "init", "-q"}, {"git", "add", "-A"}} {
		p := exec.Command(a[0], a[1:]...)
		p.Dir = root
		if err := p.Run(); err != nil {
			t.Fatalf("git %v: %v", a, err)
		}
	}
}

// 正控：目录被收进 + 脚本在表里 + 命中 ≤ 基线 ⇒ rc=0。
func TestCheckGateCoverageGreenFixture(t *testing.T) {
	root := cgcTFixture(t, "good",
		[][2]string{{"core", "go build ./..."}, {"", "python3 scripts/ok.py"}},
		[]string{"core/go.mod"}, []string{"scripts/ok.py"}, 0, "")
	rc, so, se := cgcTRun(t, "--no-self-test", "--root", root)
	if rc != 0 {
		t.Fatalf("rc=%d want 0\nstdout=%s\nstderr=%s", rc, so, se)
	}
	if !strings.Contains(so, "未收进且不在白名单 = 红：0") {
		t.Fatalf("want A red 0 line in stdout, got:\n%s", so)
	}
}

// ★ 核心负控：抽掉 core 那一步 ⇒ 必红且逐字点名 core（哑化 RED 判据 ⇒ 本用例 FAIL 带行号）。
func TestCheckGateCoverageMissingDirIsRed(t *testing.T) {
	root := cgcTFixture(t, "miss", [][2]string{{"", "python3 scripts/ok.py"}},
		[]string{"core/go.mod"}, []string{"scripts/ok.py"}, 0, "")
	rc, so, se := cgcTRun(t, "--no-self-test", "--root", root)
	if rc != 1 {
		t.Fatalf("rc=%d want 1（A 漏掉一个目录必红）\nstdout=%s\nstderr=%s", rc, so, se)
	}
	if !strings.Contains(so, "未收进且不在白名单 = 红：1") || !strings.Contains(so, "core") {
		t.Fatalf("must name core as red, got:\n%s", so)
	}
}

// ★ 负控：命中数 > 基线 ⇒ 必红（哑化棘轮判据 ⇒ FAIL）。
func TestCheckGateCoverageRatchetExceededIsRed(t *testing.T) {
	// orphan.py 未点名且不在白名单 ⇒ 命中 1；ok.py 走白名单豁免；baseline=0 ⇒ 红。
	cfg := `{"schema":"gate-coverage/1","baseline":0,"whitelist":{"build_dirs":[],"scripts":[{"path":"scripts/ok.py","reason":"夹具占位","date":"2026-09-18"}]}}`
	root := cgcTFixture(t, "rat", [][2]string{{"core", "go build ./..."}},
		[]string{"core/go.mod"}, []string{"scripts/ok.py", "scripts/orphan.py"}, 0, cfg)
	cgcTGitify(t, root)
	rc, so, se := cgcTRun(t, "--no-self-test", "--root", root)
	if rc != 1 {
		t.Fatalf("rc=%d want 1（命中 1 > 基线 0）\nstdout=%s\nstderr=%s", rc, so, se)
	}
	if !strings.Contains(so, "超过基线 1 条") {
		t.Fatalf("want 「超过基线 1 条」, got:\n%s", so)
	}
}

// ★ 负控：A 空转（0 个构建清单目录）⇒ rc=2，绝不报绿（哑化空转判据 ⇒ FAIL）。
func TestCheckGateCoverageEmptyScanBlocks(t *testing.T) {
	root := cgcTFixture(t, "empty", [][2]string{{"", "python3 scripts/ok.py"}},
		[]string{}, []string{"scripts/ok.py"}, 0, "")
	rc, so, se := cgcTRun(t, "--no-self-test", "--root", root)
	if rc != 2 {
		t.Fatalf("rc=%d want 2（空转 ⇒ 不给结论）", rc)
	}
	if !strings.Contains(so+se, "空转") {
		t.Fatalf("want 「空转」 in stdout+stderr, got:\n%s", so+se)
	}
}

// ★ 负控：白名单条目缺 reason ⇒ rc=2（不是 rc=1）。
func TestCheckGateCoverageWhitelistMissingReasonBlocks(t *testing.T) {
	cfg := `{"schema":"gate-coverage/1","baseline":0,"whitelist":{"build_dirs":[{"path":"core","date":"2026-09-18"}],"scripts":[]}}`
	root := cgcTFixture(t, "wl-no-reason", [][2]string{{"", "python3 scripts/ok.py"}},
		[]string{"core/go.mod"}, []string{"scripts/ok.py"}, 0, cfg)
	rc, _, se := cgcTRun(t, "--no-self-test", "--root", root)
	if rc != 2 {
		t.Fatalf("rc=%d want 2（白名单缺 reason）", rc)
	}
	if !strings.Contains(se, "缺 reason") {
		t.Fatalf("want 「缺 reason」 on stderr, got:\n%s", se)
	}
}

// ★ 负控：计数自证两条恒等式**各自有牙**（哑化成恒真 ⇒ 后两断言 FAIL 带行号）。
func TestCheckGateCoverageCountsCertifyHasTeeth(t *testing.T) {
	ok3 := map[string]int{"in_steps": 1, "mentioned": 1, "unmentioned": 1}
	if !cgcCountsCertify(1, 1, 1, 3, ok3, 3) {
		t.Fatal("正控：三列表 1+1+1 == 3 且 三档 1+1+1 == 3 ⇒ 必 true")
	}
	if cgcCountsCertify(1, 1, 0, 3, ok3, 3) {
		t.Fatal("负控：三列表 1+1+0 == 2 ≠ 3 ⇒ 必 false（三列表那一半有牙）")
	}
	if cgcCountsCertify(1, 1, 1, 3, map[string]int{"in_steps": 1, "mentioned": 1, "unmentioned": 0}, 3) {
		t.Fatal("负控：三档 1+1+0 == 2 ≠ 3 ⇒ 必 false（三档那一半有牙）")
	}
}

// ★ 负控：计数自证被置假 ⇒ 端到端 rc=2（证明那条路不是死码）。
func TestCheckGateCoverageEndToEndCertifyForceBlocks(t *testing.T) {
	root := cgcTFixture(t, "good2",
		[][2]string{{"core", "go build ./..."}, {"", "python3 scripts/ok.py"}},
		[]string{"core/go.mod"}, []string{"scripts/ok.py"}, 0, "")
	f := false
	cgcCertifyForce = &f
	var so, se bytes.Buffer
	rc := cgcRunMain(&so, &se, []string{"--no-self-test", "--root", root})
	cgcCertifyForce = nil
	if rc != 2 {
		t.Fatalf("rc=%d want 2（计数自证不过 ⇒ 不给结论）", rc)
	}
}

// ★ `--json`：stdout 必须纯 JSON，自检文本改走 stderr（哑化分流 ⇒ FAIL）。
func TestCheckGateCoverageJSONPurity(t *testing.T) {
	root := cgcTFixture(t, "good3",
		[][2]string{{"core", "go build ./..."}, {"", "python3 scripts/ok.py"}},
		[]string{"core/go.mod"}, []string{"scripts/ok.py"}, 0, "")
	rc, so, se := cgcTRun(t, "--json", "--root", root)
	if rc != 0 {
		t.Fatalf("rc=%d want 0", rc)
	}
	var v any
	if err := json.Unmarshal([]byte(so), &v); err != nil {
		t.Fatalf("stdout 不是纯 JSON: %v\nstdout 首 200 字：%s", err, cgcFirstN(so, 200))
	}
	if strings.Contains(so, "自检") {
		t.Fatal("自检文本混进 stdout（--json 契约破）")
	}
	if !strings.Contains(se, "── 自检（成对负控）──") {
		t.Fatalf("自检文本必须改走 stderr，got stderr:\n%s", se)
	}
}

// ★ `--json` 与 `--self-test` 互斥 ⇒ 用法错 rc=2。
func TestCheckGateCoverageJSONSelfTestMutex(t *testing.T) {
	var so, se bytes.Buffer
	rc := cgcRunMain(&so, &se, []string{"--json", "--self-test"})
	if rc != 2 {
		t.Fatalf("rc=%d want 2（互斥 ⇒ 用法错）", rc)
	}
	if !strings.Contains(se.String(), "互斥") {
		t.Fatalf("stderr must name 互斥, got:\n%s", se.String())
	}
}

// ★ 成对自检本体必须全过（哑化任一判据 ⇒ 本用例 FAIL）。
func TestCheckGateCoverageSelfTestPasses(t *testing.T) {
	var buf bytes.Buffer
	if !cgcSelfTest(&buf, true) {
		t.Fatalf("自检未全过：\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "自检结论: 全过") {
		t.Fatalf("want 「自检结论: 全过」, got:\n%s", buf.String())
	}
}

func cgcFirstN(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}
