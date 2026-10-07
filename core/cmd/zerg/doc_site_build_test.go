// doc_site_build_test.go —— A14 薄壳（`zerg doc site build` · `cmdDocSiteBuild`）的**两态对拍**与**哑化负控**。
//
// 为什么这么测（照 M2 口径 · 与 `build_wall_test.go` 同型）：进程内直调 `cmdDocSiteBuild`（不引真二进制、
// 不依赖构建），用**仓外合成夹具**给一个可控退码/可控输出的 stub 脚本，验三面：
//
//	① **退码只转发不翻译**：脚本退 0/2 ⇒ 命令面退 0/2（不许吞成 0，也不许翻成 1）；
//	② **stdout 逐字同**（两态）：走命令面拿到的 stdout == 脚本直跑同一参数的 stdout（逐字节）；
//	③ **递不到脚本 ⇒ 8**（不给结论），**不拿脚本的 2 冒充**。
//
// 哑化负控（"牙"）：把 `cmdDocSiteBuild` 改成「一律 return 0 / 吞 stdout」⇒ 上面 ①② 当场 FAIL 并给出
// 本件行号（负控只在**仓外副本**做，✗ 不在落仓件上做 —— 见 `zerg-m2-parallel-go-migration` 技能）。
//
// 另有一条**真件两态**（`docs/site/export-and-build.sh` 现读在位才跑，否则 Skip）：`--help`（rc=0）
// 与未知参数（rc=2）各跑「脚本直跑 ⟷ 走 `cmdDocSiteBuild`」，断言 stdout/rc 逐字同、stderr 只多命令面
// 那行**转发宣言**（banner 固定前缀 + 尾部「退 N」行 ⇒ 剥掉后与脚本 stderr 逐字同 ⇒ 证明「不改原件行为」）。
//
// 再加一条**包装入口栅格**（`docSiteBuildGateMain` · inproc 同签名）：两态三面逐格对拍 ≥8 格。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// dsbSyntheticRepo 建最小合成仓根：仓根判据件 ＋ 一个可控退码/可控 stdout 的 stub `export-and-build.sh`。
func dsbSyntheticRepo(t *testing.T, rc int, stdoutBody string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, docSiteBuildScriptRel)
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/usr/bin/env bash\nset -u\n" + stdoutBody + "exit " + strconv.Itoa(rc) + "\n"
	if err := os.WriteFile(scriptPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// dsbRun 走正门（进程内直调 `cmdDocSiteBuild`）：给合成仓根，返回 rc / stdout / stderr。
func dsbRun(t *testing.T, root string, tail ...string) (int, string, string) {
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

// TestDocSiteBuildPassthroughCarriesScriptExitCode —— **只转发、不翻译**：脚本退 0/2 ⇒ 命令面退 0/2（不是 1）。
func TestDocSiteBuildPassthroughCarriesScriptExitCode(t *testing.T) {
	for _, want := range []int{0, 2} {
		root := dsbSyntheticRepo(t, want, "echo 'stub 文档站：导了'\n")
		rc, _, errb := dsbRun(t, root)
		if rc != want {
			t.Fatalf("脚本退 %d ⇒ 命令面退 %d（薄壳只转发、不翻译：两值必须相等）· stderr=%s", want, rc, errb)
		}
	}
}

// TestDocSiteBuildStdoutVerbatimTwoStates —— **两态 stdout 逐字同**：命令面的 stdout **就是**脚本直跑的 stdout。
func TestDocSiteBuildStdoutVerbatimTwoStates(t *testing.T) {
	root := dsbSyntheticRepo(t, 0, "echo '── 文档站导出+构建 ──'\necho '   ✅ site/ 128 页'\n")
	_, out, _ := dsbRun(t, root)
	direct, err := exec.Command("bash", filepath.Join(root, docSiteBuildScriptRel)).Output()
	if err != nil {
		t.Fatal(err)
	}
	if out != string(direct) {
		t.Errorf("`cmdDocSiteBuild` 与脚本直跑 stdout **逐字节不同**：\n命令面=%q\n脚本  =%q", out, string(direct))
	}
}

// TestDocSiteBuildScriptAbsentIsBlocked8 —— 递不到脚本 ⇒ 退 8（不给结论）· ✗ 不拿脚本的 2 冒充。
func TestDocSiteBuildScriptAbsentIsBlocked8(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, errb := dsbRun(t, root)
	if rc != exitBlocked {
		t.Fatalf("脚本不在 ⇒ 命令面退 %d（要 %d）· stderr=%s", rc, exitBlocked, errb)
	}
}

// TestDocSiteBuildRealScriptTwoStatesVerbatim —— **真件两态**（`export-and-build.sh` 现读在位才跑，否则 Skip）：
// `--help`（rc=0）与未知参数（rc=2）各跑「脚本直跑 ⟷ 走 `cmdDocSiteBuild`」，断言 rc/stdout 逐字同、
// stderr 剥掉命令面那行转发宣言后逐字同（证明命令面**不改原件行为**）。
func TestDocSiteBuildRealScriptTwoStatesVerbatim(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Skip("解析不到仓根（合成环境）⇒ 跳真件两态（合成夹具已覆盖三面）")
	}
	script := filepath.Join(root, docSiteBuildScriptRel)
	if _, err := os.Stat(script); err != nil {
		t.Skipf("真件不在（仅公开快照/桩树会走到这里）：%v", err)
	}
	// 命令面固定加在 stderr 顶部的那行转发宣言（banner）—— 只在对比时剥掉，不改本件行为。
	banner := progName + ": 底层 = " + docSiteBuildScriptRel + "（**唯一真源** · 命令面只转发、不翻译）\n"
	for _, tc := range []struct {
		name string
		tail []string
	}{
		{"help", []string{"--help"}},
		{"unknown-flag", []string{"--nope-not-a-flag"}},
	} {
		// 态1：脚本直跑。
		var o1, e1 bytes.Buffer
		c1 := exec.Command("bash", append([]string{script}, tc.tail...)...)
		c1.Stdin = strings.NewReader("")
		c1.Stdout, c1.Stderr = &o1, &e1
		rc1 := 0
		if err := c1.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				rc1 = ee.ExitCode()
			} else {
				t.Fatalf("[%s] 脚本直跑起不来：%v", tc.name, err)
			}
		}
		// 态2：走命令面。
		rc2, out2, err2 := dsbRun(t, root, tc.tail...)
		if rc1 != rc2 {
			t.Errorf("[%s] 两态 rc 不同：脚本=%d 命令面=%d", tc.name, rc1, rc2)
		}
		if o1.String() != out2 {
			t.Errorf("[%s] 两态 stdout 不同：\n脚本  =%q\n命令面=%q", tc.name, o1.String(), out2)
		}
		if !strings.HasPrefix(err2, banner) {
			t.Errorf("[%s] 命令面 stderr 缺转发宣言（期待前缀 %q）：%q", tc.name, banner, err2)
		}
		got := strings.TrimPrefix(err2, banner)
		got = strings.TrimSuffix(got, progName+": 文档站导出+构建脚本退 "+strconv.Itoa(rc2)+"（原样转发，不翻译）\n")
		if e1.String() != got {
			t.Errorf("[%s] 剥掉命令面宣言后 stderr 与脚本不同（改了原件行为）：\n脚本  =%q\n命令面=%q", tc.name, e1.String(), got)
		}
	}
}

// dsbGateMainRun 走**包装入口**（inproc 同签名 `docSiteBuildGateMain`）：给合成仓根，返回 rc / stdout / stderr。
func dsbGateMainRun(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	rc := docSiteBuildGateMain(&out, &errb, tail)
	return rc, out.String(), errb.String()
}

// dsbRunScriptDirect 直跑底层脚本（「仓外候选 ⟷ 直跑原 .sh」里的对照态）：返回 rc / stdout / stderr。
func dsbRunScriptDirect(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	script := filepath.Join(root, docSiteBuildScriptRel)
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

// TestDocSiteBuildGateMainGridTwoStatesThreeFaces —— **包装入口栅格**（`docSiteBuildGateMain` · inproc 同签名）：
// 态1 = 仓外候选包装入口 `docSiteBuildGateMain(&out,&errb,tail)`；态2 = 直跑原脚本 `bash <script> <tail>`。
// 三面 = rc / stdout / stderr（stderr 剥命令面 banner ＋ 退码行 ⇒ **噪声归一**）。逐格对拍 ≥8 格（旗标面 × 退码面）。
// 另断言包装入口与 `cmdDocSiteBuild` 三面**逐字同**（= 包装入口只转发、零新增语义）。
func TestDocSiteBuildGateMainGridTwoStatesThreeFaces(t *testing.T) {
	cells := []struct {
		name string
		tail []string
		rc   int
	}{
		{"help", []string{"--help"}, 0},
		{"unknown", []string{"--nope-not-a-flag"}, 2},
		{"strict", []string{"--strict"}, 1},
		{"serve", []string{"--serve"}, 0},
		{"strict-serve", []string{"--strict", "--serve"}, 2},
		{"out-dir", []string{"--out", "/tmp/site"}, 0},
		{"strict-out", []string{"--strict", "--out", "/tmp/site"}, 1},
		{"two-unknown", []string{"--aaa", "--bbb"}, 2},
	}
	banner := progName + ": 底层 = " + docSiteBuildScriptRel + "（**唯一真源** · 命令面只转发、不翻译）\n"
	for _, c := range cells {
		// stub 把 `$*`（argv 逐字）打进 stdout ⇒ 旗标面 passthrough 可观测。
		root := dsbSyntheticRepo(t, c.rc, "printf 'ARGS:%s\\n' \"$*\"\n")
		rcS, outS, errS := dsbRunScriptDirect(t, root, c.tail...)
		rcW, outW, errW := dsbGateMainRun(t, root, c.tail...)
		rcC, outC, errC := dsbRun(t, root, c.tail...)
		if rcW != rcS || rcC != rcS {
			t.Errorf("[%s] rc 三态不同：脚本=%d 包装=%d cmd=%d", c.name, rcS, rcW, rcC)
		}
		if outW != outS || outC != outS {
			t.Errorf("[%s] stdout 三态不同：\n脚本=%q\n包装=%q\ncmd =%q", c.name, outS, outW, outC)
		}
		if !strings.HasPrefix(errW, banner) {
			t.Errorf("[%s] 包装 stderr 缺 banner：%q", c.name, errW)
		}
		got := strings.TrimPrefix(errW, banner)
		if rcW != 0 {
			got = strings.TrimSuffix(got, progName+": 文档站导出+构建脚本退 "+strconv.Itoa(rcW)+"（原样转发，不翻译）\n")
		}
		if got != errS {
			t.Errorf("[%s] 剥噪声后 stderr 与脚本不同：\n脚本=%q\n包装=%q", c.name, errS, got)
		}
		if errW != errC {
			t.Errorf("[%s] 包装 stderr 与 cmd 不同：\n包装=%q\ncmd =%q", c.name, errW, errC)
		}
	}
}
