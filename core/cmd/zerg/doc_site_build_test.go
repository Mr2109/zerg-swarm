// doc_site_build_test.go —— 真搬后（`zerg doc site build` · 行为落 Go）的**两态三面**逐字对拍 + 「不 exec 原脚本」的牙。
//
// 本件对应 `doc_site_build.go` 已从「壳转账」改成**真搬**：命令面**不再 exec** `docs/site/export-and-build.sh`。
//
// 测试口径：态1 = 原脚本直跑（`bash <脚本> <tail>`）⟷ 态2 = 走 Go 正门（`cmdDocSiteBuild` / `docSiteBuildGateMain`）；
// 三面 = rc / stdout / stderr **逐字**比。只在**不需要 venv/pip/mkdocs/联网**的可跑面（`--help` / `-h` / 未知参数 /
// 缺件）做逐字 —— 重活面（真建站）要外部工具链与网络，不属单测面（见件头「未覆盖面」）。
//
// 「不 exec」的牙：合成仓放一枚**毒脚本**（打印 POISON、退 7）⇒ 走 Go 正门必须**看都不看它**（照真搬行为走）；
// 谁若把实现退回壳转账，这几条用例当场红。
//
// 两态「脚本在盘 ⟷ 移出仓」：合成仓里「有脚本」与「无脚本」两态，Go 侧三面**必须逐字同**（✗ 不得 rc=2）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const dsbScriptRel = "docs/site/export-and-build.sh"

// dsbRunGo —— 走正门（进程内直调 `cmdDocSiteBuild`）：给仓根，返回 rc / stdout / stderr。
func dsbRunGo(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"doc", "site", "build"},
		orig: append([]string{"doc", "site", "build"}, tail...),
	}
	rc := cmdDocSiteBuild(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// dsbRunGoGate —— 走**包装入口**（inproc 同签名 `docSiteBuildGateMain`）：给仓根，返回 rc / stdout / stderr。
func dsbRunGoGate(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	rc := docSiteBuildGateMain(&out, &errb, tail)
	return rc, out.String(), errb.String()
}

// dsbRunSh —— 直跑原脚本（态1）。
func dsbRunSh(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	script := filepath.Join(root, dsbScriptRel)
	var o, e bytes.Buffer
	c := exec.Command("bash", append([]string{script}, tail...)...)
	c.Dir = root
	c.Stdin = strings.NewReader("")
	c.Stdout, c.Stderr = &o, &e
	rc := 0
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			t.Fatalf("脚本直跑起不来：%v", err)
		}
	}
	return rc, o.String(), e.String()
}

// dsbRealRoot —— 真仓根（合成环境解析不到就跳过）。
func dsbRealRoot(t *testing.T) string {
	t.Helper()
	root := repoRoot()
	if root == "" {
		t.Skip("解析不到仓根（合成环境）⇒ 跳真件两态")
	}
	if _, err := os.Stat(filepath.Join(root, dsbScriptRel)); err != nil {
		t.Skipf("真脚本不在（仅公开快照/桩树会走到这里）：%v", err)
	}
	return root
}

// ── 1) 真件两态三面栅格（只含「参数面就退出」的格 · 不触 venv/pip/mkdocs/联网）────────────────────
func TestDocSiteBuildArgGridTwoStatesThreeFaces(t *testing.T) {
	root := dsbRealRoot(t)
	cells := []struct {
		name string
		tail []string
	}{
		{"help", []string{"--help"}},
		{"help-short", []string{"-h"}},
		{"unknown", []string{"--nope-not-a-flag"}},
		{"strict-then-unknown", []string{"--strict", "--nope"}},
		{"unknown-first", []string{"--nope", "--strict"}},
		{"two-unknown", []string{"--aaa", "--bbb"}},
		{"serve-then-unknown", []string{"--serve", "--nope"}},
		{"strict-serve-unknown", []string{"--strict", "--serve", "--nope"}},
	}
	for _, c := range cells {
		rcS, outS, errS := dsbRunSh(t, root, c.tail...)
		rcC, outC, errC := dsbRunGo(t, root, c.tail...)
		rcW, outW, errW := dsbRunGoGate(t, root, c.tail...)
		if rcS != rcC || rcC != rcW {
			t.Errorf("[%s] rc 三态不同：脚本=%d cmd=%d 包装=%d", c.name, rcS, rcC, rcW)
		}
		if outS != outC || outC != outW {
			t.Errorf("[%s] stdout 三态不同：\n脚本=%q\ncmd =%q\n包装=%q", c.name, outS, outC, outW)
		}
		if errS != errC || errC != errW {
			t.Errorf("[%s] stderr 三态不同（真搬后不许再有「我转发自…」宣言）：\n脚本=%q\ncmd =%q\n包装=%q",
				c.name, errS, errC, errW)
		}
	}
}

// ── 2) 帮助面常量 == 原脚本第 2–20 行（漂移牙：谁改原脚本那几行而没同批改常量，这里当场红）────────
func TestDocSiteBuildHelpConstantMatchesLiveScript(t *testing.T) {
	root := dsbRealRoot(t)
	b, err := os.ReadFile(filepath.Join(root, dsbScriptRel))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) < 20 {
		t.Fatalf("原脚本不足 20 行（%d）", len(lines))
	}
	want := strings.Join(lines[1:20], "\n") + "\n"
	_, out, _ := dsbRunGo(t, root, "--help")
	if out != want {
		t.Errorf("`--help` 正文与原脚本第 2–20 行**逐字不同**：\n常量 =%q\n脚本 =%q", out, want)
	}
}

// ── 3) 「不 exec 原脚本」的牙：合成仓放毒脚本（POISON · 退 7）⇒ 走 Go 正门必须看都不看它 ─────────────
func TestDocSiteBuildDoesNotExecScript(t *testing.T) {
	root := dsbPoisonRepo(t, "#!/usr/bin/env bash\necho POISON\nexit 7\n")
	// 缺 mkdocs.yml ⇒ 真搬行为退 2「缺配置」，✗ 不是毒脚本的 7、✗ 不打印 POISON。
	rc, out, errb := dsbRunGo(t, root)
	if rc != exitUsage {
		t.Fatalf("缺配置应退 %d，得 %d（若退 7 = 又走回了壳转账）· stderr=%s", exitUsage, rc, errb)
	}
	if strings.Contains(out, "POISON") || strings.Contains(errb, "POISON") {
		t.Fatalf("命令面 exec 了原脚本（出现 POISON）：out=%q err=%q", out, errb)
	}
	if !strings.Contains(errb, "✗ 缺配置：") {
		t.Fatalf("缺配置的 stderr 正文不对：%q", errb)
	}
	// --help 同样不许碰毒脚本（退 0 · 打帮助常量）。
	rc, out, _ = dsbRunGo(t, root, "--help")
	if rc != exitOK {
		t.Fatalf("--help 应退 0，得 %d（若退 7 = 又走回了壳转账）", rc)
	}
	if !strings.HasPrefix(out, "# =============================================================================\n") || strings.Contains(out, "POISON") {
		t.Fatalf("--help 正文不对：%q", out)
	}
}

// ── 4) 两态「脚本在盘 ⟷ 移出仓」：Go 侧三面逐字同（✗ 不得 rc=2）───────────────────────────────
func TestDocSiteBuildScriptAbsentSameAsPresent(t *testing.T) {
	withSh := dsbRepoWithRealScript(t) // 有脚本（真件副本）
	noSh := dsbVersionOnlyRepo(t)      // 无脚本
	cells := [][]string{{"--help"}, {"-h"}, {"--nope"}}
	for _, tail := range cells {
		// 只用「参数面就退出」的格（不打印任何路径）⇒ 两态输出逐字应同，与仓根字符串无关。
		rcP, outP, errP := dsbRunGoZ(t, withSh, tail...)
		rcA, outA, errA := dsbRunGoZ(t, noSh, tail...)
		if rcP != rcA || outP != outA || errP != errA {
			t.Errorf("[%v] 「脚本在盘 ⟷ 移出仓」三面不同：\n在盘 rc=%d out=%q err=%q\n无脚本 rc=%d out=%q err=%q",
				tail, rcP, outP, errP, rcA, outA, errA)
		}
		if rcA == exitUsage && strings.Contains(errA, "脚本不在") {
			t.Errorf("[%v] 移出仓后走了「脚本不在」的旧壳口径（真搬后不该再有这一档）", tail)
		}
	}
}

// ── 5) 缺配置两态逐字（真脚本副本的合成仓 · 无 mkdocs.yml）：脚本直跑 ⟷ 走 Go 正门 ───────────────
func TestDocSiteBuildMissingConfigTwoStates(t *testing.T) {
	withSh := dsbRepoWithRealScript(t)
	zr := dsbShDerivedRoot(t, filepath.Join(withSh, dsbScriptRel))
	rcS, outS, errS := dsbRunSh(t, withSh)
	rcC, outC, errC := dsbRunGoZ(t, zr)
	if rcS != rcC {
		t.Fatalf("缺配置两态 rc 不同：脚本=%d cmd=%d", rcS, rcC)
	}
	if rcS != exitUsage {
		t.Fatalf("缺配置应退 2，得 %d", rcS)
	}
	if outS != outC {
		t.Errorf("缺配置两态 stdout 不同：脚本=%q cmd=%q", outS, outC)
	}
	if errS != errC {
		t.Errorf("缺配置两态 stderr 不同：\n脚本=%q\ncmd =%q", errS, errC)
	}
}

// ── 夹具 ────────────────────────────────────────────────────────────────────────────────────────

// dsbVersionOnlyRepo —— 只放仓根判据件的合成仓（无脚本 · 无 mkdocs.yml）。
func dsbVersionOnlyRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// dsbPoisonRepo —— 合成仓 + 一枚自带行为的「毒脚本」。
func dsbPoisonRepo(t *testing.T, body string) string {
	t.Helper()
	root := dsbVersionOnlyRepo(t)
	sp := filepath.Join(root, dsbScriptRel)
	if err := os.MkdirAll(filepath.Dir(sp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sp, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// dsbRepoWithRealScript —— 合成仓 + **真脚本副本**（无 mkdocs.yml ⇒ 两侧都停在「缺配置」档）。
func dsbRepoWithRealScript(t *testing.T) string {
	t.Helper()
	real := dsbRealRoot(t)
	root := dsbVersionOnlyRepo(t)
	b, err := os.ReadFile(filepath.Join(real, dsbScriptRel))
	if err != nil {
		t.Fatal(err)
	}
	sp := filepath.Join(root, dsbScriptRel)
	if err := os.MkdirAll(filepath.Dir(sp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sp, b, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// dsbShDerivedRoot —— 用 bash 现读脚本推导出的仓根（= 原脚本的 REPO_ROOT），供 ZERG_REPO 对齐用。
func dsbShDerivedRoot(t *testing.T, scriptPath string) string {
	t.Helper()
	out, err := exec.Command("bash", "-c", `cd "$(dirname "$1")/../.." && pwd`, "_", scriptPath).Output()
	if err != nil {
		t.Fatalf("bash 推导仓根失败：%v", err)
	}
	return strings.TrimSpace(string(out))
}

// dsbRunGoZ —— 走正门，但 ZERG_REPO 显式给「脚本推导仓根」（两态对齐路径文案用）。
func dsbRunGoZ(t *testing.T, zergRepo string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", zergRepo)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"doc", "site", "build"},
		orig: append([]string{"doc", "site", "build"}, tail...),
	}
	rc := cmdDocSiteBuild(inv, &out, &errb)
	return rc, out.String(), errb.String()
}
