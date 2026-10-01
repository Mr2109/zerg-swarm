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
