// check_artifact_source_parity_test.go —— 判据件 `check-artifact-source-parity.py` Go 真搬（门74）的
// **成对负控**测试件。✗ 禁恒绿：每条正控都配一条会真分叉的负控/反误报，且专测把 py 的 B 类真口子钉住。
//
// 用例面（逐条）：
//
//	· TestAsp_SelfTest          —— 入口自检档 19 断言 0 失败（= py 自检逐字同）。
//	· TestAsp_Flags             —— 全部声明旗标各跑一格（--list-rules / -h / --self-test / 未知旗标 / 缺值）。
//	· TestAsp_Fixtures          —— 正控 0 · 负控（真落后）1 · 反误报（dirty+落后）0 · 空转 2 · 缺制品 2。
//	· TestAsp_JSON              —— --json 面：键序 / `behind: null` / `lagging: []` 三形态（✗ 禁 omitempty 吞键）。
//	· TestAsp_PyPathSemantics   —— py join/normpath/abspath/relpath/fnmatch/glob 的已知语义差各钉一格。
//	· TestAsp_BClassSilentSkip  —— B 类真口子：读不了的件/目录**静默**少判（现状钉死，修了口子立刻红）。
//	· TestAsp_SelfRefNoEmbedHit —— 自指：本件＋测试件**零物理行**匹配自门嵌入口径（命中数 0）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// aspRunEntry —— 进程内复用入口（argv 不含程序名 · 与 py sys.argv[1:] 同形）。
func aspRunEntry(argv ...string) (int, string, string) {
	var so, se bytes.Buffer
	rc := checkArtifactSourceParityGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

func aspGitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var so bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &so
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v 失败：%v（%s）", args, err, so.String())
	}
	return strings.TrimSpace(so.String())
}

// aspNewRepo —— 合成一棵真 git 仓：core/a.go 一筆提交 · 返回 (仓根, HEAD)。
func aspNewRepo(t *testing.T, name string) (string, string) {
	t.Helper()
	r := aspPyJoin(t.TempDir(), name)
	if err := os.MkdirAll(aspPyJoin(r, "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aspPyJoin(r, "core", "a.go"), []byte("package main\n// v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aspGitIn(t, r, "init", "-q")
	aspGitIn(t, r, "-c", "user.email=s@x", "-c", "user.name=s", "add", "-A")
	aspGitIn(t, r, "-c", "user.email=s@x", "-c", "user.name=s", "commit", "-qm", "x")
	return r, aspGitIn(t, r, "rev-parse", "HEAD")
}

func TestAsp_SelfTest(t *testing.T) {
	rc, out, _ := aspRunEntry("--self-test")
	if rc != 0 {
		t.Fatalf("自检 rc=%d（期望 0）\n%s", rc, out)
	}
	if !strings.Contains(out, "断言 19 条 · 失败 0 条") {
		t.Fatalf("自检结论行不符（py 为 19/0）：\n%s", out)
	}
}

func TestAsp_Flags(t *testing.T) {
	rc, out, _ := aspRunEntry("--list-rules")
	if rc != 0 || !strings.Contains(out, "门74 制品↔源码两代分离 · 规则表") {
		t.Fatalf("--list-rules rc=%d：\n%s", rc, out)
	}
	rc, out, _ = aspRunEntry("-h")
	if rc != 0 || !strings.HasPrefix(out, "usage: check-artifact-source-parity.py") {
		t.Fatalf("-h rc=%d：\n%q", rc, out)
	}
	rc, _, _ = aspRunEntry("--self-test")
	if rc != 0 {
		t.Fatalf("--self-test rc=%d（期望 0）", rc)
	}
	rc, _, se := aspRunEntry("--no-such-flag")
	if rc != 2 || !strings.Contains(se, "unrecognized arguments: --no-such-flag") {
		t.Fatalf("未知旗标 rc=%d stderr=%q（期望 rc=2）", rc, se)
	}
	rc, _, se = aspRunEntry("--root")
	if rc != 2 || !strings.Contains(se, "argument --root: expected one argument") {
		t.Fatalf("--root 缺值 rc=%d stderr=%q（期望 rc=2）", rc, se)
	}
}

func TestAsp_Fixtures(t *testing.T) {
	// ① 正控：制品 sha = HEAD 短 sha · 建构晚于输入面 ⇒ rc=0
	r1, h1 := aspNewRepo(t, "same")
	a1 := aspFakeArtifact(r1, "bin-art", aspFirst16(h1), "2026-09-28T10:00:00Z", false)
	aspSetMtime(aspPyJoin(r1, "core", "a.go"), "2026-09-28T09:00:00Z")
	rc, out, _ := aspRunEntry("--root", r1, "--artifact", a1)
	if rc != 0 {
		t.Fatalf("① 正控 rc=%d（期望 0）\n%s", rc, out)
	}
	if !strings.Contains(out, "面A 提交面：同一笔") {
		t.Fatalf("① 面A 未判同笔：\n%s", out)
	}

	// ② 负控（真落后）：制品建构早于输入面 ⇒ rc=1 且逐条点名
	r2, h2 := aspNewRepo(t, "lag")
	a2 := aspFakeArtifact(r2, "bin-art", aspFirst16(h2), "2026-09-27T00:00:00Z", false)
	aspSetMtime(aspPyJoin(r2, "core", "a.go"), "2026-09-28T09:00:00Z")
	rc, out, _ = aspRunEntry("--root", r2, "--artifact", a2)
	if rc != 1 {
		t.Fatalf("② 负控 rc=%d（期望 1 · 若恒 0 说明判据哑了）\n%s", rc, out)
	}
	if n := aspCountContains(aspSplitLines(out), "· core/a.go（源码侧"); n != 1 {
		t.Fatalf("② 未逐条点名 core/a.go（命中 %d）", n)
	}

	// ③ 反误报：dirty 常态 + 提交面落后 1 笔 + 建构不早于输入面 ⇒ 必须 rc=0
	r3, _ := aspNewRepo(t, "dirty")
	aspGitIn(t, r3, "-c", "user.email=s@x", "-c", "user.name=s", "commit", "-q", "--allow-empty", "-m", "y")
	old := aspGitIn(t, r3, "rev-list", "--max-count=1", "HEAD~1")
	a3 := aspFakeArtifact(r3, "bin-art", aspFirst16(old), "2026-09-28T10:00:00Z", true)
	aspSetMtime(aspPyJoin(r3, "core", "a.go"), "2026-09-28T09:00:00Z")
	rc, out, _ = aspRunEntry("--root", r3, "--artifact", a3)
	if rc != 0 {
		t.Fatalf("③ 反误报失败：dirty+落后被误判 rc=%d（应 0）\n%s", rc, out)
	}
	if !strings.Contains(out, "面C 脏标记：制品自报 +dirty") {
		t.Fatalf("③ 面C 未如实登记 dirty")
	}

	// ④ 空转 ≠ 绿：仓在但制品不在盘 ⇒ rc=2
	rc, out, _ = aspRunEntry("--root", r1, "--artifact", aspPyJoin(t.TempDir(), "没有这一件", "zerg"))
	if rc != 2 || !strings.Contains(out, "制品不在盘") {
		t.Fatalf("④ 缺制品 rc=%d（期望 2）\n%s", rc, out)
	}

	// ⑤ 自报面认不出 ⇒ rc=2
	bad := aspPyJoin(r1, "art-bad")
	if err := os.WriteFile(bad, []byte("#!/bin/sh\necho 我不知道\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	rc, out, _ = aspRunEntry("--root", r1, "--artifact", bad)
	if rc != 2 || !strings.Contains(out, "自报面认不出") {
		t.Fatalf("⑤ 认不出 rc=%d（期望 2）\n%s", rc, out)
	}

	// ⑥ 制品那一代不在本仓对象库 ⇒ rc=2
	a6 := aspFakeArtifact(r1, "art6", "deadbee1234", "2026-09-28T10:00:00Z", false)
	rc, _, _ = aspRunEntry("--root", r1, "--artifact", a6)
	if rc != 2 {
		t.Fatalf("⑥ 非本仓 sha rc=%d（期望 2 · 判不了不得当绿）", rc)
	}

	// ⑦ *_test.go 不算输入面 ⇒ rc=0
	r7, h7 := aspNewRepo(t, "testonly")
	a7 := aspFakeArtifact(r7, "bin-art", aspFirst16(h7), "2026-09-28T10:00:00Z", false)
	aspSetMtime(aspPyJoin(r7, "core", "a.go"), "2026-09-28T09:00:00Z")
	tp := aspPyJoin(r7, "core", "a_test.go")
	if err := os.WriteFile(tp, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aspSetMtime(tp, "2026-09-28T11:00:00Z")
	rc, out, _ = aspRunEntry("--root", r7, "--artifact", a7)
	if rc != 0 {
		t.Fatalf("⑦ 只改 *_test.go 被误判 rc=%d（应 0）\n%s", rc, out)
	}
}

func TestAsp_JSON(t *testing.T) {
	r, h := aspNewRepo(t, "json")
	a := aspFakeArtifact(r, "bin-art", aspFirst16(h), "2026-09-28T10:00:00Z", false)
	aspSetMtime(aspPyJoin(r, "core", "a.go"), "2026-09-28T09:00:00Z")
	rc, out, _ := aspRunEntry("--json", "--root", r, "--artifact", a)
	if rc != 0 {
		t.Fatalf("--json rc=%d", rc)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("--json 不可解析：%v\n%s", err, out)
	}
	for _, k := range []string{"rc", "artifact_commit", "head_full", "lagging", "behind"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("--json 缺键 %s：\n%s", k, out)
		}
	}
	// 空值三形态：`behind: null` 与 `lagging: []` 必须**键在场**（✗ 禁被 omitempty 吞掉）。
	if !strings.Contains(out, "\"behind\": null") {
		t.Fatalf("behind 应为显式 null（键在场）：\n%s", out)
	}
	if !strings.Contains(out, "\"lagging\": []") {
		t.Fatalf("lagging 应为显式空数组（键在场）：\n%s", out)
	}
	// 键序（py 插入序 · 头两键固定为 root / artifact）。
	if !strings.HasPrefix(out, "{\n  \"root\": ") {
		t.Fatalf("--json 键序不符（首键应为 root）：\n%s", out[:min(len(out), 80)])
	}
}

func TestAsp_PyPathSemantics(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"join 后段保 `..`", aspPyJoin("a", "../b"), "a/../b"},
		{"join 后段绝对 ⇒ 丢弃前段", aspPyJoin("repo", "/abs"), "/abs"},
		{"join 空段保分隔符 a/", aspPyJoin("a", ""), "a/"},
		{"join 空段 a/b", aspPyJoin("a", "", "b"), "a/b"},
		{"join 空首段", aspPyJoin("", "a"), "a"},
		{"normpath 保留恰两个前导斜杠", aspPyNormpath("//a/b"), "//a/b"},
		{"normpath 三个斜杠折成一个", aspPyNormpath("///a/b"), "/a/b"},
		{"normpath 空 ⇒ .", aspPyNormpath(""), "."},
		{"abspath 词法 normpath 一次（不解析符号链接）", aspPyAbspath("/w", "a/../b"), "/w/b"},
		{"dirname 剥尾斜杠", aspPyDirname("/a/b/"), "/a/b"},
		{"dirname 根", aspPyDirname("/a"), "/"},
		{"relpath 同路径 ⇒ .", aspPyRelpath("/w", "/w/x", "/w/x"), "."},
		{"relpath 下钻", aspPyRelpath("/w", "/w/x/y", "/w"), "x/y"},
		{"relpath 上溯", aspPyRelpath("/w", "/w", "/w/x"), ".."},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Fatalf("%s：得到 %q 期望 %q", c.name, c.got, c.want)
		}
	}
	// fnmatch：`*` 必须**跨 `/`**（Go path.Match/filepath.Match 不跨 ⇒ 用它会整批假失配）。
	if !aspFnmatchCase("a/b/c.json", "*.json") {
		t.Fatal("fnmatch `*` 未跨 `/`（应用 path.Match 就会这样）")
	}
	if !aspFnmatchCase("x.txt", "x.[t]xt") {
		t.Fatal("fnmatch `[seq]` 未实现")
	}
	// glob：无 magic 且不存在 ⇒ 空；字面存在 ⇒ 自身。
	if got := aspGlob("/definitely/not/here/x.json"); len(got) != 0 {
		t.Fatalf("glob 不存在的字面量应为空，得到 %v", got)
	}
	d := t.TempDir()
	_ = os.WriteFile(aspPyJoin(d, "a.json"), []byte("{}"), 0o644)
	_ = os.WriteFile(aspPyJoin(d, "b.txt"), []byte(""), 0o644)
	_ = os.WriteFile(aspPyJoin(d, ".hidden.json"), []byte("{}"), 0o644)
	got := aspGlob(aspPyJoin(d, "*.json"))
	if len(got) != 1 || !strings.HasSuffix(got[0], "a.json") {
		t.Fatalf("glob *.json 应只命中 a.json（隐藏项不参与），得到 %v", got)
	}
}

// TestAsp_BClassSilentSkip —— ★ 钉住 py 的 B 类真口子（读不了 ⇒ **静默**少判 · 逐字照搬）：
// `.go` 件读不了时**跳过 embed 扫描**（`except OSError: pass`）⇒ 它点到的资产被静默漏判，
// 分母不守恒于「判不了」。日后谁把该口子修成红，本用例立刻 FAIL（＝给口子留了把牙）。
func TestAsp_BClassSilentSkip(t *testing.T) {
	r, h := aspNewRepo(t, "bclass")
	a := aspFakeArtifact(r, "bin-art", aspFirst16(h), "2026-09-28T10:00:00Z", false)
	aspSetMtime(aspPyJoin(r, "core", "a.go"), "2026-09-28T09:00:00Z")
	// core/emb.go 现读一条自门嵌入口径 ⇒ 它点到的资产会进输入面。
	emb := aspPyJoin(r, "core", "emb.go")
	if err := os.WriteFile(emb, []byte("package main\n\n//go:embed asset.json\nvar raw string\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	asset := aspPyJoin(r, "core", "asset.json")
	if err := os.WriteFile(asset, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aspSetMtime(emb, "2026-09-28T09:00:00Z")
	aspSetMtime(asset, "2026-09-28T11:00:00Z") // 建构之后改过 ⇒ 该资产落后
	rcBefore, outBefore, _ := aspRunEntry("--root", r, "--artifact", a)
	if rcBefore != 1 {
		t.Fatalf("前置：emb.go 可读时 embed 面应认出资产 ⇒ rc=1，得到 %d（夹具失灵）\n%s", rcBefore, outBefore)
	}
	if err := os.Chmod(emb, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(emb, 0o644) }()
	rcAfter, _, _ := aspRunEntry("--root", r, "--artifact", a)
	if rcAfter != 0 {
		t.Fatalf("B 类口子形态变了：读不了的 `.go` 件本应**静默**跳过 embed 扫描（资产随之漏判 · rc 1→0），得到 %d", rcAfter)
	}
}

// TestAsp_SelfRefNoEmbedHit —— 自指：本件与测试件落在自门扫描面（core/**/*.go）内 ⇒
// 全文**零物理行**可匹配自门嵌入口径（命中数 0）⇒ 自门现读本件不新增 embed 资产。
func TestAsp_SelfRefNoEmbedHit(t *testing.T) {
	for _, f := range []string{"check_artifact_source_parity.go", "check_artifact_source_parity_test.go"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读本件失败（测试须在 core/cmd/zerg 下跑）：%v", err)
		}
		hits := 0
		for _, line := range aspIterLines(string(data)) {
			if aspEmbedRE.MatchString(line) {
				hits++
			}
		}
		if hits != 0 {
			t.Fatalf("%s 有 %d 行匹配自门嵌入口径 ⇒ 会让自门凭空多出 embed 资产（自指形态①）", f, hits)
		}
	}
}
