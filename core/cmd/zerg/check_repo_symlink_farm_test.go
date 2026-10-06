package main

// check_repo_symlink_farm_test.go —— 成对负控（✗ 禁恒绿）：正控（干净 ⇒ rc=0）· 各造 violation（⇒ rc=1）
// · 取不到输入（⇒ rc=2）· JSON 面 · --self-test · 帮助与 argparse 面 · 遍历/边界语义。
// 全部断言以 py 原件现读读数为基准（见回执对拍表）。

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func crsfTRun(argv []string) (int, string, string) {
	var o, e bytes.Buffer
	rc := checkRepoSymlinkFarmGateMain(&o, &e, argv)
	return rc, o.String(), e.String()
}

func crsfTSetup(t *testing.T) (home, repo string) {
	t.Helper()
	base := t.TempDir()
	home = filepath.Join(base, "home")
	repo = filepath.Join(base, "repo")
	if err := os.MkdirAll(filepath.Join(repo, "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	return home, repo
}

// ④ 正控：干净家目录 ⇒ rc=0 · 正文「全绿」。
func TestCrsfCleanHomeIsGreen(t *testing.T) {
	home, repo := crsfTSetup(t)
	rc, out, errS := crsfTRun([]string{"--home", home, "--repo", repo})
	if rc != 0 {
		t.Fatalf("干净家目录应 rc=0，得 %d (stdout=%q stderr=%q)", rc, out, errS)
	}
	if !strings.Contains(out, "✓ 全绿（rc=0）") {
		t.Fatalf("正文缺「全绿」：%q (第 %d 行)", out, 1)
	}
}

// ① must-fail：浅层直挂「指向本仓」链接 ⇒ rc=1 · 逐条打印「链接 → 目标」。
func TestCrsfHitIsRed(t *testing.T) {
	home, repo := crsfTSetup(t)
	link := filepath.Join(home, "zerg-repo-symlink")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := crsfTRun([]string{"--home", home, "--repo", repo})
	if rc != 1 {
		t.Fatalf("指向本仓的链接应 rc=1，得 %d (第 %d 行)", rc, 1)
	}
	if !strings.Contains(out, "✗ 链接 → 目标："+link+" → ") {
		t.Fatalf("未逐条打印链接→目标：%q", out)
	}
}

// ② 指向别处的链接 ⇒ 不判红。
func TestCrsfLinkElsewhereNotRed(t *testing.T) {
	home, repo := crsfTSetup(t)
	outside := filepath.Join(filepath.Dir(home), "elsewhere")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, "x-link")); err != nil {
		t.Fatal(err)
	}
	rc, _, _ := crsfTRun([]string{"--home", home, "--repo", repo})
	if rc != 0 {
		t.Fatalf("指向别处的链接不该判红，得 rc=%d (第 %d 行)", rc, 1)
	}
}

// ③ 二层子目录里的链接 ⇒ 判红（depth≤2 覆盖）。
func TestCrsfDepth2Covered(t *testing.T) {
	home, repo := crsfTSetup(t)
	sub := filepath.Join(home, "work")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(repo, filepath.Join(sub, "repo0")); err != nil {
		t.Fatal(err)
	}
	rc, _, _ := crsfTRun([]string{"--home", home, "--repo", repo})
	if rc != 1 {
		t.Fatalf("二层子目录里的指向本仓链接应 rc=1，得 %d (第 %d 行)", rc, 1)
	}
}

// ③b 三层（超 depth）⇒ 不判 —— 抳住 MAX_DEPTH 屏（抹掉深度限制会误命中、本用例红）。
func TestCrsfDepthBeyondLimitIgnored(t *testing.T) {
	home, repo := crsfTSetup(t)
	deep := filepath.Join(home, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(repo, filepath.Join(deep, "repo0")); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := crsfTRun([]string{"--home", home, "--repo", repo})
	if rc != 0 {
		t.Fatalf("超 depth 的链接不该判红（第 %d 行），得 rc=%d (stdout=%q)", 1, rc, out)
	}
}

// ③c 符号链接目录不下钻（followlinks=False）⇒ 其内「指向本仓」的链接不计。
func TestCrsfSymlinkDirNotDescended(t *testing.T) {
	home, repo := crsfTSetup(t)
	outside := filepath.Join(filepath.Dir(home), "elsewhere")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(repo, filepath.Join(outside, "inner")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, "dirlink")); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := crsfTRun([]string{"--home", home, "--repo", repo})
	if rc != 0 {
		t.Fatalf("别处目录内的链接不该被扫（不下钻符号链接目录），得 rc=%d (stdout=%q) (第 %d 行)", rc, out, 1)
	}
}

// ⑤ 取不到扫描根 ⇒ rc=2（不给结论）。
func TestCrsfMissingHomeBlocked(t *testing.T) {
	home, repo := crsfTSetup(t)
	rc, out, _ := crsfTRun([]string{"--home", filepath.Join(home, "nope"), "--repo", repo})
	if rc != 2 {
		t.Fatalf("扫描根不存在应 rc=2（第 %d 行），得 %d", 1, rc)
	}
	if !strings.Contains(out, "扫描根读不到") {
		t.Fatalf("缺根档正文不符：%q", out)
	}
}

// ⑥ JSON 面：干净 ⇒ hits/problems 为空数组（不是 null）。
func TestCrsfJSONClean(t *testing.T) {
	home, repo := crsfTSetup(t)
	rc, out, _ := crsfTRun([]string{"--home", home, "--repo", repo, "--json"})
	if rc != 0 {
		t.Fatalf("JSON 干净档应 rc=0，得 %d", rc)
	}
	if !strings.Contains(out, "\"hits\": []") || !strings.Contains(out, "\"problems\": []") {
		t.Fatalf("空集合应显式 []：%q (第 %d 行)", out, 1)
	}
	if !strings.Contains(out, "\"home\": ") || !strings.Contains(out, "扫描根=") {
		t.Fatalf("JSON 键/注缺失：%q", out)
	}
}

// ⑥b JSON 命中面：hits 是对象数组，键序 link→target。
func TestCrsfJSONHit(t *testing.T) {
	home, repo := crsfTSetup(t)
	link := filepath.Join(home, "l0")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := crsfTRun([]string{"--home", home, "--repo", repo, "--json"})
	if rc != 1 {
		t.Fatalf("JSON 命中档应 rc=1，得 %d", rc)
	}
	if !strings.Contains(out, "\"link\": ") || !strings.Contains(out, "\"target\": ") {
		t.Fatalf("hits 对象键缺失：%q (第 %d 行)", out, 1)
	}
	if !strings.Contains(out, "\"hits\": [\n    {") {
		t.Fatalf("hits 应为对象数组：%q", out)
	}
}

// ⑦ 自检档：rc=0 · 五格 ✓ 齐（含负控④ rc=2）。
func TestCrsfSelfTestGreen(t *testing.T) {
	rc, out, _ := crsfTRun([]string{"--self-test"})
	if rc != 0 {
		t.Fatalf("--self-test 应 rc=0，得 %d (stdout=%q)", rc, out)
	}
	if !strings.Contains(out, "✓ 负控④：扫描根不存在 ⇒ rc=2（不给结论）") {
		t.Fatalf("自检缺负控④：%q (第 %d 行)", out, 1)
	}
}

// ⑧ 帮助/argparse：-h（rc=0）· 未知旗标（rc=2 stderr）· 前缀缩写（--ho）。
func TestCrsfHelpAndArgparse(t *testing.T) {
	rc, out, errS := crsfTRun([]string{"-h"})
	if rc != 0 || errS != "" {
		t.Fatalf("-h 应 rc=0 且 stderr 空，得 rc=%d err=%q", rc, errS)
	}
	if !strings.HasPrefix(out, "usage: check-repo-symlink-farm.py [-h] [--home HOME] [--repo REPO] [--json]\n"+
		strings.Repeat(" ", 34)+"[--self-test]\n\n") {
		t.Fatalf("帮助 usage 头不符：%q (第 %d 行)", out, 1)
	}

	rc, out, errS = crsfTRun([]string{"--bogus"})
	if rc != 2 || out != "" {
		t.Fatalf("未知旗标应 rc=2 且 stdout 空，得 rc=%d out=%q", rc, out)
	}
	if errS != crsfUsageText+"check-repo-symlink-farm.py: error: unrecognized arguments: --bogus\n" {
		t.Fatalf("未知旗标 stderr 不符：%q (第 %d 行)", errS, 1)
	}

	home, repo := crsfTSetup(t)
	rc, _, _ = crsfTRun([]string{"--ho", home, "--re", repo})
	if rc != 0 {
		t.Fatalf("前缀缩写 --ho/--re 应等价 --home/--repo，得 rc=%d (第 %d 行)", rc, 1)
	}
}
