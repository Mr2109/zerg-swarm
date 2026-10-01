package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 定向单测：cpfsPyJoin 三语义差（本批标准①）─────────────────────────────────────
func TestCpfsPyJoin_SemanticsThreeDifferences(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"吃 `..` 原样留串（✗ 不归一）", cpfsPyJoin("a", "../b"), "a/../b"},
		{"后段绝对 ⇒ 丢弃前面所有段", cpfsPyJoin("repo", "/abs"), "/abs"},
		{"空段：join('a','')=='a/'", cpfsPyJoin("a", ""), "a/"},
		{"空段：join('','a')=='a'", cpfsPyJoin("", "a"), "a"},
		{"空段：join('a','','b')=='a/b'", cpfsPyJoin("a", "", "b"), "a/b"},
		{"常规：repo+publish+whitelist.txt", cpfsPyJoin("repo", "publish", "whitelist.txt"), "repo/publish/whitelist.txt"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s：cpfsPyJoin = %q，期望 %q", c.name, c.got, c.want)
		}
	}
}

func TestCpfsPyNormpath_Tails(t *testing.T) {
	if got := cpfsPyNormpath(""); got != "." {
		t.Errorf(`cpfsPyNormpath("") = %q，期望 "."`, got)
	}
	if got := cpfsPyNormpath("a/b/../c"); got != "a/c" {
		t.Errorf(`cpfsPyNormpath("a/b/../c") = %q，期望 "a/c"`, got)
	}
	if got := cpfsPyNormpath("/a/b/"); got != "/a/b" {
		t.Errorf(`cpfsPyNormpath("/a/b/") = %q，期望 "/a/b"`, got)
	}
}

// ── 定向单测：判据⑤ 清单语法（负控：坏清单必须逐条报）──────────────────────────────
func TestCpfsListSyntaxProblems_ReportsThreeShapes(t *testing.T) {
	dir := t.TempDir()
	pub := filepath.Join(dir, "publish")
	if err := os.MkdirAll(pub, 0o755); err != nil {
		t.Fatal(err)
	}
	wl := filepath.Join(pub, "whitelist.txt")
	_ = os.WriteFile(wl, []byte("scripts/\nwall/\n# 残件（未闭合；"), 0o644)
	_ = os.WriteFile(filepath.Join(pub, "private-paths.txt"), []byte("scripts/mech.py\n"), 0o644)
	_ = os.WriteFile(filepath.Join(pub, "mirror-allow.txt"), []byte("scripts/mech.py 无TAB理由\n"), 0o644)
	_ = os.WriteFile(filepath.Join(pub, "replace-rules.tsv"), []byte("foo\tbar\n"), 0o644)
	probs := cpfsListSyntaxProblems(cpfsPaths{whitelist: wl})
	joined := ""
	for _, p := range probs {
		joined += p[0] + ":" + p[1] + ":" + p[2] + "\n"
	}
	for _, want := range []string{"末行没有结尾换行", "末条内容行以 ； 结尾", "整件全角括号不配平", "理由空"} {
		if !strings.Contains(joined, want) {
			t.Errorf("判据⑤ 缺一类问题：%q 不在\n%s", want, joined)
		}
	}
	if len(probs) < 4 {
		t.Errorf("判据⑤ 问题数 = %d，期望 ≥4", len(probs))
	}
}

// ── 负控（有牙）：镜像器丢而旧器不排 ⇒ 判据① 必须红（rc=1）───────────────────────────
func TestCpfsEvaluate_DriftIsRed(t *testing.T) {
	root := t.TempDir()
	cpfsMakeFixture(root, cpfsFx{
		name: "drift", wl: []string{"scripts/", "wall/"},
		priv: []string{"scripts/mech.py"}, excl: []string{"scripts/unrelated-but-real.py"},
		drop: []string{"scripts/mech.py"}, want: 1,
	})
	rc, lines := cpfsEvaluate(root, cpfsPathsFor(root))
	if rc != 1 {
		t.Fatalf("漂移格 rc = %d，期望 1（负控失效）\n%s", rc, strings.Join(lines, "\n"))
	}
	if !strings.Contains(strings.Join(lines, "\n"), "判据① 漂移 1 条") {
		t.Errorf("判据① 未点名漂移：\n%s", strings.Join(lines, "\n"))
	}
}

// ── 正控：干净夹具 ⇒ rc=0（证明负控不是恒红）───────────────────────────────────────
func TestCpfsEvaluate_CleanIsGreen(t *testing.T) {
	root := t.TempDir()
	cpfsMakeFixture(root, cpfsFx{
		name: "clean", wl: []string{"scripts/", "wall/"},
		priv: []string{"scripts/mech.py"}, excl: []string{"scripts/mech.py"},
		drop: []string{"scripts/mech.py"}, want: 0,
	})
	rc, lines := cpfsEvaluate(root, cpfsPathsFor(root))
	if rc != 0 {
		t.Fatalf("干净格 rc = %d，期望 0\n%s", rc, strings.Join(lines, "\n"))
	}
}

// ── 入口面：自检 16 条必须全过（自回收 delta=0 由 os.RemoveAll 的 defer 保）──────────
func TestCpfsSelfTest_AllPass(t *testing.T) {
	var out bytes.Buffer
	rc := cpfsSelfTest(&out)
	if rc != 0 {
		t.Fatalf("自检 rc = %d，期望 0\n%s", rc, out.String())
	}
	if !strings.Contains(out.String(), "自检：16 过 / 0 红（16 条用例）") {
		t.Errorf("自检汇总行不符：\n%s", out.String())
	}
}

// ── 入口面：--json 必须拒（Q-176 · 机器面缺 ≠ 零命中）──────────────────────────────
func TestCpfsGateMain_JsonRefused(t *testing.T) {
	var out, errb bytes.Buffer
	rc := checkPublishFaceSyncGateMain(&out, &errb, []string{"check-publish-face-sync.py", "--json"})
	if rc != 2 {
		t.Fatalf("--json rc = %d，期望 2", rc)
	}
	if !strings.Contains(errb.String(), "本门无机器读面") {
		t.Errorf("--json 未逐字说明：%s", errb.String())
	}
}

// ── 入口面：-h 帮助面逐字（usage 续行缩进 34）──────────────────────────────────────
func TestCpfsGateMain_HelpFace(t *testing.T) {
	var out, errb bytes.Buffer
	rc := checkPublishFaceSyncGateMain(&out, &errb, []string{"check-publish-face-sync.py", "-h"})
	if rc != 0 {
		t.Fatalf("-h rc = %d，期望 0", rc)
	}
	if !strings.Contains(out.String(), "usage: check-publish-face-sync.py [-h] [--repo REPO] [--self-test]") {
		t.Errorf("-h usage 首行不符：\n%s", out.String())
	}
	if !strings.Contains(out.String(), "                                  [--whitelist WHITELIST]") {
		t.Errorf("-h usage 续行缩进不符：\n%s", out.String())
	}
}
