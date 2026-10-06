// check_doc_name_test.go —— Go 真搬的**成对负控定向单测 + 逐格对拍**（M2 · check_doc_name.go）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态，并对 py 原件逐格对拍（rc / stdout 两面）。
// 用例名：TestCheckDocNameGatePort_Directed（定向）· TestCheckDocNameGatePort_Parity（对拍）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func cdnRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "scripts", "gates", "check-doc-name.py")); err != nil {
		t.Skipf("仓根未定位到 py 原件（%s）", root)
	}
	return root
}

func cdnWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cdnMakeGoodTree —— 造一棵好件树（全绿，含允许对 edit-assert）。
func cdnMakeGoodTree(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "good")
	files := map[string]string{
		"docs/01-设计/设计-文档体系-v1.0.md":  "# x\n",
		"docs/zh/模块-主控.md":            "# x\n",
		"README.md":                   "# x\n",
		"scripts/docs/edit-assert":    "#!/usr/bin/env python3\n",
		"scripts/docs/edit-assert.md": "# 说明书\n",
	}
	for rel, txt := range files {
		cdnWrite(t, filepath.Join(root, rel), txt)
	}
	return root
}

// cdnMakeBadTree —— 造一棵坏件树（含 N6 同基名对 + N1/N2/N3/N4 命中）。
func cdnMakeBadTree(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "bad")
	bad := []string{
		"docs/INDEX.md",
		"docs/研究 - 自我纠错.md",
		"docs/01-设计/设计-文档体系-20260918.md",
		"docs/01-设计/设计-v2.5.10-卵功能与对话机制.md",
		"docs/01-设计/设计-文档体系.md",
		"docs/01-设计/设计-文档体系.py",
	}
	for _, rel := range bad {
		cdnWrite(t, filepath.Join(root, rel), "# x\n")
	}
	cdnWrite(t, filepath.Join(root, "docs/混用目录/01-a.md"), "# x\n")
	cdnWrite(t, filepath.Join(root, "docs/混用目录/02-b.md"), "# x\n")
	cdnWrite(t, filepath.Join(root, "docs/混用目录/c.md"), "# x\n")
	cdnWrite(t, filepath.Join(root, "docs/混用目录/d.md"), "# x\n")
	return root
}

func cdnRunGate(argv []string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkDocNameGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckDocNameGatePort_Directed(t *testing.T) {
	good := cdnMakeGoodTree(t)
	bad := cdnMakeBadTree(t)

	if rc, out, _ := cdnRunGate([]string{"--repo-root", good, "--target", good, "--no-self-test"}); rc != 0 || !strings.Contains(out, "OK") || !strings.Contains(out, "A06") {
		t.Fatalf("好件必绿失败：rc=%d\n%s", rc, out)
	}
	if rc, out, _ := cdnRunGate([]string{"--repo-root", bad, "--target", bad, "--no-self-test"}); rc != 1 || !strings.Contains(out, "N6") {
		t.Fatalf("坏件必红失败：rc=%d\n%s", rc, out)
	}
	if rc, out, _ := cdnRunGate([]string{"--repo-root", bad, "--target", filepath.Join(bad, "docs/INDEX.md"), "--no-self-test"}); rc != 1 || !strings.Contains(out, "N1") {
		t.Fatalf("N1 坏件应红：rc=%d\n%s", rc, out)
	}
	if rc, out, _ := cdnRunGate([]string{"--repo-root", bad, "--target", filepath.Join(bad, "no-such"), "--no-self-test"}); rc != 2 || !strings.Contains(out, "不给结论") {
		t.Fatalf("缺件应 rc=2：rc=%d\n%s", rc, out)
	}
	if rc, out, _ := cdnRunGate([]string{"--list-exempt"}); rc != 0 || !strings.Contains(out, "A 档 · 《清单-命名规范化》已登记的「不改」条目：15 条（期望 15）") {
		t.Fatalf("--list-exempt 失败：rc=%d\n%s", rc, out)
	}
	if rc, out, _ := cdnRunGate([]string{"--list-rules"}); rc != 0 || !strings.Contains(out, "N1") {
		t.Fatalf("--list-rules 失败：rc=%d\n%s", rc, out)
	}
}

type cdnParityCell struct {
	name     string
	argv     []string
	envExtra []string // KEY=VALUE 覆盖；空串值表示 unset
}

func cdnPyEnv(base []string, extra []string) []string {
	drop := map[string]bool{}
	for _, e := range extra {
		if k, _, ok := strings.Cut(e, "="); ok {
			drop[k] = true
		}
	}
	out := []string{}
	for _, e := range base {
		k, _, ok := strings.Cut(e, "=")
		if ok && drop[k] {
			continue
		}
		out = append(out, e)
	}
	for _, e := range extra {
		if e == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}

func TestCheckDocNameGatePort_Parity(t *testing.T) {
	repo := cdnRepoRoot(t)
	pyPath := filepath.Join(repo, "scripts", "gates", "check-doc-name.py")
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("无 python3，跳过对拍")
	}
	good := cdnMakeGoodTree(t)
	bad := cdnMakeBadTree(t)
	fx := filepath.Dir(bad)
	fxempty := t.TempDir()

	// N7 形态分类夹具（GAP-20260926-166/213d）：词形态照告警 + single + 版本标记（收窄不告警）
	n7 := filepath.Join(fx, "n7")
	cdnWrite(t, filepath.Join(n7, "docs", "模块Agent说明.md"), "# x\n") // (file, CJK→LAT, word)
	cdnWrite(t, filepath.Join(n7, "docs", "Agent模块.md"), "# x\n")   // (file, LAT→CJK, word)
	cdnWrite(t, filepath.Join(n7, "docs", "模块A说明.md"), "# x\n")     // (file, CJK→LAT, single)
	cdnWrite(t, filepath.Join(n7, "docs", "设计-模块v2.1.md"), "# x\n") // version ⇒ 收窄，无 N7

	// 为 devdocs 默认根负控准备：unset 三个换根 env ⇒ 同级根不存在 ⇒ rc=2
	devUnset := []string{"ZERG_DEVDOCS_ROOT=", "ZERG_DOCS_ALT=", "ZERG_DOCS="}

	cells := []cdnParityCell{
		{"真仓·scope repo", []string{"--repo-root", repo, "--scope", "repo", "--no-self-test"}, nil},
		{"真仓·json", []string{"--repo-root", repo, "--scope", "repo", "--json", "--no-self-test"}, nil},
		{"真仓·max-examples 1", []string{"--repo-root", repo, "--scope", "repo", "--max-examples", "1", "--no-self-test"}, nil},
		{"list-rules", []string{"--list-rules"}, nil},
		{"list-exempt", []string{"--list-exempt"}, nil},
		{"夹具好件必绿", []string{"--repo-root", good, "--target", good, "--no-self-test"}, nil},
		{"夹具坏件必红", []string{"--repo-root", fx, "--target", bad, "--no-self-test"}, nil},
		{"夹具前缀混用告警", []string{"--repo-root", fx, "--target", filepath.Join(bad, "docs/混用目录"), "--no-self-test"}, nil},
		{"单件目标", []string{"--repo-root", fx, "--target", filepath.Join(bad, "docs/INDEX.md"), "--no-self-test"}, nil},
		{"缺件目标", []string{"--repo-root", fx, "--target", filepath.Join(fx, "no-such"), "--no-self-test"}, nil},
		{"scope docs 缺面", []string{"--repo-root", fxempty, "--scope", "docs", "--no-self-test"}, nil},
		{"devdocs 根不在", []string{"--repo-root", fx, "--scope", "devdocs", "--no-self-test"}, devUnset},
		{"json 夹具坏件", []string{"--repo-root", fx, "--target", bad, "--json", "--no-self-test"}, nil},
		{"夹具好件 --list-exempt --hits", []string{"--repo-root", good, "--target", good, "--list-exempt", "--hits", "--no-self-test"}, nil},
		{"夹具坏件 --summary", []string{"--repo-root", fx, "--target", bad, "--summary", "--no-self-test"}, nil},
		{"夹具坏件 --n7-shapes", []string{"--repo-root", fx, "--target", bad, "--n7-shapes", "--no-self-test"}, nil},
		{"夹具坏件 --exclude-prefix", []string{"--repo-root", fx, "--target", bad, "--exclude-prefix", "bad/docs/01-设计/", "--no-self-test"}, nil},
		{"N7 形态分类 + 收窄", []string{"--repo-root", fx, "--target", n7, "--n7-shapes", "--no-self-test"}, nil},
		{"真仓 --summary", []string{"--repo-root", repo, "--scope", "repo", "--summary", "--no-self-test"}, nil},
	}

	baseEnv := os.Environ()
	mismatches := 0
	for _, c := range cells {
		var goOut, goErr bytes.Buffer
		goRC := checkDocNameGateMain(&goOut, &goErr, c.argv)

		cmd := exec.Command(py, append([]string{pyPath}, c.argv...)...)
		cmd.Env = cdnPyEnv(baseEnv, c.envExtra)
		var pyOut, pyErr bytes.Buffer
		cmd.Stdout = &pyOut
		cmd.Stderr = &pyErr
		runErr := cmd.Run()
		pyRC := 0
		if runErr != nil {
			if ee, ok := runErr.(*exec.ExitError); ok {
				pyRC = ee.ExitCode()
			} else {
				t.Fatalf("起 py 失败：%v", runErr)
			}
		}
		if goRC != pyRC || goOut.String() != pyOut.String() {
			mismatches++
			t.Errorf("[%s] DIFF: rc go=%d py=%d · stdout 同=%v\n--go--\n%s\n--py--\n%s",
				c.name, goRC, pyRC, goOut.String() == pyOut.String(), goOut.String(), pyOut.String())
		} else {
			t.Logf("[%s] SAME (rc=%d, stdout %d B)", c.name, goRC, goOut.Len())
		}
	}
	t.Logf("对拍格数=%d 失配=%d", len(cells), mismatches)
}

// TestCheckDocNameGatePort_SelfTestSameRC —— 自检 rc 面（✗ 不比 stdout：元自检格已声明未搬）。
func TestCheckDocNameGatePort_SelfTestSameRC(t *testing.T) {
	repo := cdnRepoRoot(t)
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("无 python3")
	}
	goRC, _, _ := cdnRunGate([]string{"--self-test"})
	cmd := exec.Command(py, filepath.Join(repo, "scripts", "gates", "check-doc-name.py"), "--self-test")
	out, _ := cmd.CombinedOutput()
	pyRC := cmd.ProcessState.ExitCode()
	if goRC != pyRC {
		t.Errorf("--self-test rc 不一致：go=%d py=%d\npy 输出尾部：%s", goRC, pyRC, tailStr(string(out), 8))
	}
}

func tailStr(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// TestCheckDocName_MetaSelfCheckRefuses —— 回归牙（M2 战线①对齐批-3）：
// ⑫ 元自检 —— 自检未过 ⇒ main 必须**拒扫真目标**（rc=2 · 首行=拒扫抬头 · 末行非结论行）。
// 哑化 main 的注入分支 / 去掉拒扫抬头 ⇒ 本用例 FAIL（带行号）。
func TestCheckDocName_MetaSelfCheckRefuses(t *testing.T) {
	t.Setenv("CHECK_DOC_NAME_META_FORCE_FAIL", "1")
	d := t.TempDir()
	var out, errb bytes.Buffer
	rc := checkDocNameGateMain(&out, &errb, []string{"--repo-root", d, "--target", d})
	if rc != 2 {
		t.Fatalf("自检未过 ⇒ 该退 2（拒扫真目标），实得 %d\nout=%q", rc, out.String())
	}
	var ol []string
	for _, l := range strings.Split(out.String(), "\n") {
		if strings.TrimSpace(l) != "" {
			ol = append(ol, l)
		}
	}
	if len(ol) == 0 || ol[0] != "自检未过 ⇒ 拒绝扫真目标（rc=2 不给结论）" {
		t.Errorf("首行该是拒扫抬头，实得 %q", ol)
	}
	if n := len(ol); n > 0 && (strings.HasPrefix(ol[n-1], "OK：") || strings.HasPrefix(ol[n-1], "FAIL：")) {
		t.Errorf("末行不该是结论行：%q", ol[n-1])
	}

	// 同牙另一半（对齐 py 现件）：`--self-test` 清单必须含 ⑫ 元自检格、断言数 30（哑化该格 ⇒ FAIL）。
	var o2, e2 bytes.Buffer
	if rc2 := checkDocNameGateMain(&o2, &e2, []string{"--self-test"}); rc2 != 0 {
		t.Fatalf("--self-test 该退 0，实得 %d\n%s", rc2, o2.String())
	}
	if !strings.Contains(o2.String(), "✓ 自检未过 ⇒ rc=2 且拒绝扫真目标（元自检 · 注入副本）") ||
		!strings.Contains(o2.String(), "自检用例：期望 30 条 · 实跑 30 条") {
		t.Errorf("自检清单缺 ⑫ 元自检格 / 断言数不为 30：\n%s", o2.String())
	}
}
