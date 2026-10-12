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

// ── 入口面：自检 20 条必须全过（自回收 delta=0 由 os.RemoveAll 的 defer 保）──────────
func TestCpfsSelfTest_AllPass(t *testing.T) {
	var out bytes.Buffer
	rc := cpfsSelfTest(&out)
	if rc != 0 {
		t.Fatalf("自检 rc = %d，期望 0\n%s", rc, out.String())
	}
	if !strings.Contains(out.String(), "自检：20 过 / 0 红（20 条用例）") {
		t.Errorf("自检汇总行不符：\n%s", out.String())
	}
}

// ── ★ C20（GAP-20261013-08）双向判据的反向腿：`public=yes` 的件写进 excludes ⇒ 判据⑨ 必须红（rc=1）──
// 改前：判据① 单向（镜像器丢弃 ⊆ 旧器排除）+ 判据⑥ 只扫 `public=no` 行 ⇒ **三道门全绿**而镜像器会真剔（假绿）。
func TestCpfsEvaluate_PublicYesInExcludesIsRed(t *testing.T) {
	root := t.TempDir()
	cpfsMakeFixture(root, cpfsFx{
		name: "rev-public-yes", wl: []string{"scripts/", "wall/"},
		priv: []string{"scripts/mech.py"}, excl: []string{"scripts/mech.py", "scripts/keep.py"},
		drop: []string{"scripts/mech.py"}, want: 1,
	})
	rc, lines := cpfsEvaluate(root, cpfsPathsFor(root))
	joined := strings.Join(lines, "\n")
	if rc != 1 {
		t.Fatalf("`public=yes` 落进 excludes：rc = %d，期望 1（判据⑨ 反向腿无牙）\n%s", rc, joined)
	}
	if !strings.Contains(joined, "判据⑨ 排除面 ∩ 应公开面 = 1 条") {
		t.Errorf("判据⑨ 未点名命中数：\n%s", joined)
	}
	// 「点出行与件」两样都要有：件名（左）＋ 命中它的排除行所属表与条目（右）。
	if !strings.Contains(joined, "← 排除行（EXCLUDES）：scripts/keep.py") ||
		!strings.Contains(joined, "scripts/keep.py  ") {
		t.Errorf("判据⑨ 未同时点出件与行：\n%s", joined)
	}
}

// 正控（判据⑨ 的反面）：**同一张排除表**、只把该件的标记表改 `no` ⇒ 判据⑨ 不得红（rc=0）。
// 这一格证明判据键在「标记表 public=yes」，✗ 不是「凡出现在 excludes 即红」（否则会误伤合法排除）。
func TestCpfsEvaluate_SameExcludesMarkedNoIsGreen(t *testing.T) {
	root := t.TempDir()
	cpfsMakeFixture(root, cpfsFx{
		name: "rev-marked-no", wl: []string{"scripts/", "wall/"},
		priv: []string{"scripts/mech.py"}, excl: []string{"scripts/mech.py", "scripts/keep.py"},
		drop: []string{"scripts/mech.py"}, want: 0,
		marksText: "文件	public	理由\n" +
			"scripts/mech.py	no	夹具：B 类（合成 · 判据⑥ 的被判对象）\n" +
			"scripts/keep.py	no	夹具：同一排除表 · 只改标 no ⇒ 判据⑨ 不得红\n",
	})
	rc, lines := cpfsEvaluate(root, cpfsPathsFor(root))
	joined := strings.Join(lines, "\n")
	if rc != 0 {
		t.Fatalf("同一排除表 · 标记表说 no：rc = %d，期望 0（判据⑨ 误伤合法排除）\n%s", rc, joined)
	}
	if !strings.Contains(joined, "✅ 判据⑨ 排除面 ∩ 应公开面：0 条") {
		t.Errorf("判据⑨ 正控行不符：\n%s", joined)
	}
}

// ── 负控（降档有牙）：面外件（未入索引）一入库即红 ⇒ 判据⑦ 降档 rc=2（只降档不报红）────────
func TestCpfsEvaluate_OuterViolDowngrades(t *testing.T) {
	root := t.TempDir()
	cpfsMakeFixture(root, cpfsFx{
		name: "outer", wl: []string{"scripts/", "wall/"},
		priv: []string{"scripts/mech.py"}, excl: []string{"scripts/mech.py"},
		drop: []string{"scripts/mech.py"}, want: 2,
		marksText: "文件\tpublic\t理由\n" +
			"scripts/mech.py\tno\t夹具：B 类\n" +
			"scripts/keep.py\tyes\t夹具：A 类\n" +
			"scripts/outer_viol.py\tno\t夹具：面外件\n",
	})
	cpfsWrite(cpfsPyJoin(root, "scripts/outer_viol.py"), "x\n") // 写完不 add ⇒ 面外（未入索引）
	rc, lines := cpfsEvaluate(root, cpfsPathsFor(root))
	if rc != 2 {
		t.Fatalf("面外违规件 rc = %d，期望 2（降档失效）\n%s", rc, strings.Join(lines, "\n"))
	}
	if !strings.Contains(strings.Join(lines, "\n"), "判据⑦ 面外件一入库即红 1 条") {
		t.Errorf("判据⑦ 未点名面外违规件：\n%s", strings.Join(lines, "\n"))
	}
}

// ── 正控：面外件全绿（新码）⇒ 判据⑦ 不降档（rc=0 · 不误伤他路未跟踪新码）────────────────
func TestCpfsEvaluate_OuterCleanNoDowngrade(t *testing.T) {
	root := t.TempDir()
	cpfsMakeFixture(root, cpfsFx{
		name: "outerclean", wl: []string{"scripts/", "wall/"},
		priv: []string{"scripts/mech.py"}, excl: []string{"scripts/mech.py"},
		drop: []string{"scripts/mech.py"}, want: 0,
	})
	cpfsWrite(cpfsPyJoin(root, "scripts/outer_new.py"), "x\n")
	rc, lines := cpfsEvaluate(root, cpfsPathsFor(root))
	if rc != 0 {
		t.Fatalf("面外干净件 rc = %d，期望 0\n%s", rc, strings.Join(lines, "\n"))
	}
	if !strings.Contains(strings.Join(lines, "\n"), "判据⑦ 面外件（未入索引）1 条") {
		t.Errorf("判据⑦ 关系式行不符：\n%s", strings.Join(lines, "\n"))
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

// ── B1 有牙（判据⑧）：只存于 HEAD（索引已删）的白名单件 ⇒ 必报 + 降档 rc=2 ──────────────
// 合成桩：件已提交进 HEAD，再从索引 `git rm --cached` 并删工作树件 ⇒ `ls-files` 与 `ls-files --others`
// 都看不见它，而镜像器真源 `ls-tree -r HEAD` 仍看得见它。改前（无判据⑧）本门 **rc=0 假绿**。
func TestCpfsEvaluate_HeadOnlyReported(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@e")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@e")
	root := t.TempDir()
	cpfsMakeFixture(root, cpfsFx{
		name: "headonly", wl: []string{"scripts/", "wall/"},
		priv: []string{"scripts/mech.py"}, excl: []string{"scripts/mech.py"},
		drop: []string{"scripts/mech.py"}, want: 2,
		trackedExtra: []string{"scripts/head_only.py"},
		marksText: "文件\tpublic\t理由\n" +
			"scripts/mech.py\tno\t夹具：B 类（合成）\n" +
			"scripts/keep.py\tyes\t夹具：A 类（合成）\n" +
			"scripts/head_only.py\tno\t夹具：只存于 HEAD（索引已删）· 镜像器仍发 · 判据⑥ 看不见\n",
	})
	// 提交成 HEAD 后，把 head_only.py 从索引删 + 删工作树件 ⇒ 只存于 HEAD（两旧面都看不见它）
	cpfsGit("-C", root, "commit", "-q", "--no-gpg-sign", "-m", "fx")
	cpfsGit("-C", root, "rm", "-q", "--cached", "--", "scripts/head_only.py")
	_ = os.Remove(cpfsPyJoin(root, "scripts/head_only.py"))
	rc, lines := cpfsEvaluate(root, cpfsPathsFor(root))
	joined := strings.Join(lines, "\n")
	if rc != 2 {
		t.Fatalf("只存于 HEAD 件 rc = %d，期望 2（判据⑧ 无牙 · 假绿）\n%s", rc, joined)
	}
	if !strings.Contains(joined, "判据⑧") {
		t.Errorf("判据⑧ 未出声：\n%s", joined)
	}
	if !strings.Contains(joined, "只存于 HEAD") || !strings.Contains(joined, "scripts/head_only.py") {
		t.Errorf("判据⑧ 未报出「只存于 HEAD」的件：\n%s", joined)
	}
}

// ── B1 正控：件集 HEAD == 索引 ⇒ 判据⑧ 报 0 条 · rc 不变（不误伤正常件）──────────────
func TestCpfsEvaluate_HeadSameAsIndexGreen(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@e")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@e")
	root := t.TempDir()
	cpfsMakeFixture(root, cpfsFx{
		name: "headsame", wl: []string{"scripts/", "wall/"},
		priv: []string{"scripts/mech.py"}, excl: []string{"scripts/mech.py"},
		drop: []string{"scripts/mech.py"}, want: 0,
	})
	cpfsGit("-C", root, "commit", "-q", "--no-gpg-sign", "-m", "fx")
	rc, lines := cpfsEvaluate(root, cpfsPathsFor(root))
	joined := strings.Join(lines, "\n")
	if rc != 0 {
		t.Fatalf("正常件（HEAD==索引）rc = %d，期望 0\n%s", rc, joined)
	}
	if !strings.Contains(joined, "判据⑧ 只存于 HEAD 的件 0 条") {
		t.Errorf("判据⑧ 正控行不符：\n%s", joined)
	}
}
