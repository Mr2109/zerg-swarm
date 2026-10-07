// deploy_launch_test.go —— A13 薄壳（`zerg deploy launch` · `cmdDeployLaunch`）的**两态对拍**与**哑化负控**。
//
// 为什么这么测（照 M2 口径 · 与 `build_wall_test.go` 同型）：进程内直调 `cmdDeployLaunch`（不引真二进制、
// 不依赖构建），用**仓外合成夹具**给一个可控退码/可控输出的 stub 脚本，验三面：
//
//	① **退码只转发不翻译**：脚本退 0/2/3/4/5 ⇒ 命令面退同一个码（不许吞成 0，也不许翻成 1）；
//	② **stdout 逐字同**（两态）：走命令面拿到的 stdout == 脚本直跑同一参数的 stdout（逐字节）；
//	③ **递不到脚本 ⇒ 8**（不给结论），**不拿脚本的 2 冒充**。
//
// 哑化负控（"牙"）：把 `cmdDeployLaunch` 改成「一律 return 0 / 吞 stdout」⇒ 上面 ①② 当场 FAIL 并给出
// 本件行号（负控只在**仓外副本**做，✗ 不在落仓件上做 —— 见 `zerg-m2-parallel-go-migration` 技能）。
//
// 另有一条**真件两态**（`deploy/scripts/deploy-launch.sh` 现读在位才跑，否则 Skip）：`--help`（rc=0）与
// 未知参数（rc=2）各跑「脚本直跑 ⟷ 走 `cmdDeployLaunch`」，断言 stdout/rc 逐字同、stderr 只多命令面那行
// **转发宣言**（banner 固定前缀 ⇒ 剥掉后与脚本 stderr 逐字同 ⇒ 证明「不改原件行为」）。
//
// 再加一条**包装入口栅格**（`deployLaunchGateMain` · inproc 同签名）：两态三面逐格对拍 ≥8 格。
//
// ★ 两个 `--nope`/`--help` 档都不 ssh、不写盘（`--help` 走 awk 打脚本头；未知参数走 `die` ⇒ 2）
// ⇒ 真件两态档**零副作用**（不触 `~/.zerg/deploy_launch/`）。
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

// dplSyntheticRepo 建最小合成仓根：仓根判据件 ＋ 一个可控退码/可控 stdout 的 stub `deploy-launch.sh`。
func dplSyntheticRepo(t *testing.T, rc int, stdoutBody string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, deployLaunchScriptRel)
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/usr/bin/env bash\nset -u\n" + stdoutBody + "exit " + strconv.Itoa(rc) + "\n"
	if err := os.WriteFile(scriptPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// dplRun 走正门（进程内直调 `cmdDeployLaunch`）：给合成仓根，返回 rc / stdout / stderr。
func dplRun(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"deploy", "launch"},
		orig: append([]string{"deploy", "launch"}, tail...),
	}
	rc := cmdDeployLaunch(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// TestDeployLaunchPassthroughCarriesScriptExitCode —— **只转发、不翻译**：脚本退 N ⇒ 命令面退 N。
func TestDeployLaunchPassthroughCarriesScriptExitCode(t *testing.T) {
	for _, want := range []int{0, 2, 3, 4, 5} {
		root := dplSyntheticRepo(t, want, "echo 'stub 发布：意图件 + 回读'\n")
		rc, _, errb := dplRun(t, root)
		if rc != want {
			t.Fatalf("脚本退 %d ⇒ 命令面退 %d（薄壳只转发、不翻译：两值必须相等）· stderr=%s", want, rc, errb)
		}
	}
}

// TestDeployLaunchStdoutVerbatimTwoStates —— **两态 stdout 逐字同**：命令面的 stdout **就是**脚本直跑的 stdout。
func TestDeployLaunchStdoutVerbatimTwoStates(t *testing.T) {
	root := dplSyntheticRepo(t, 0, "echo '🧾 意图件: /tmp/deploy_launch/intent-20261007T000000Z-abcdef12.json'\necho '📥 回读件: /tmp/deploy_launch/latest-deploy.json.readback'\n")
	_, out, _ := dplRun(t, root)
	direct, err := exec.Command("bash", filepath.Join(root, deployLaunchScriptRel)).Output()
	if err != nil {
		t.Fatal(err)
	}
	if out != string(direct) {
		t.Errorf("`cmdDeployLaunch` 与脚本直跑 stdout **逐字节不同**：\n命令面=%q\n脚本  =%q", out, string(direct))
	}
}

// TestDeployLaunchScriptAbsentIsBlocked8 —— 递不到脚本 ⇒ 退 8（不给结论）· ✗ 不拿脚本的 2 冒充。
func TestDeployLaunchScriptAbsentIsBlocked8(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, errb := dplRun(t, root)
	if rc != exitBlocked {
		t.Fatalf("脚本不在 ⇒ 命令面退 %d（要 %d）· stderr=%s", rc, exitBlocked, errb)
	}
}

// TestDeployLaunchRealScriptTwoStatesVerbatim —— **真件两态**（`deploy-launch.sh` 现读在位才跑，否则 Skip）：
// `--help`（rc=0）与未知参数（rc=2）各跑「脚本直跑 ⟷ 走 `cmdDeployLaunch`」，断言 rc/stdout 逐字同、
// stderr 剥掉命令面那行转发宣言后逐字同（证明命令面**不改原件行为**）。
func TestDeployLaunchRealScriptTwoStatesVerbatim(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Skip("解析不到仓根（合成环境）⇒ 跳真件两态（合成夹具已覆盖三面）")
	}
	script := filepath.Join(root, deployLaunchScriptRel)
	if _, err := os.Stat(script); err != nil {
		t.Skipf("真件不在（仅公开快照/桩树会走到这里）：%v", err)
	}
	// 命令面固定加在 stderr 顶部的那行转发宣言（banner）—— 只在对比时剥掉，不改本件行为。
	banner := progName + ": 底层 = " + deployLaunchScriptRel + "（**唯一真源** · 命令面只转发、不翻译）\n"
	for _, tc := range []struct {
		name string
		tail []string
	}{
		{"help", []string{"--help"}},
		{"unknown-flag", []string{"--nope-not-a-flag"}},
		{"missing-required", []string{"--host", "edge-node"}},
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
		rc2, out2, err2 := dplRun(t, root, tc.tail...)
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
		got = strings.TrimSuffix(got, progName+": 发布封装脚本退 "+strconv.Itoa(rc2)+"（原样转发，不翻译）\n")
		if e1.String() != got {
			t.Errorf("[%s] 剥掉命令面宣言后 stderr 与脚本不同（改了原件行为）：\n脚本  =%q\n命令面=%q", tc.name, e1.String(), got)
		}
	}
}

// dplGateMainRun 走**包装入口**（inproc 同签名 `deployLaunchGateMain`）：给合成仓根，返回 rc / stdout / stderr。
func dplGateMainRun(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	rc := deployLaunchGateMain(&out, &errb, tail)
	return rc, out.String(), errb.String()
}

// dplRunScriptDirect 直跑底层脚本（「仓外候选 ⟷ 直跑原 .sh」里的对照态）：返回 rc / stdout / stderr。
func dplRunScriptDirect(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	script := filepath.Join(root, deployLaunchScriptRel)
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

// TestDeployLaunchGateMainGridTwoStatesThreeFaces —— **包装入口栅格**（`deployLaunchGateMain` · inproc 同签名）：
// 态1 = 仓外候选包装入口 `deployLaunchGateMain(&out,&errb,tail)`；态2 = 直跑原脚本 `bash <script> <tail>`。
// 三面 = rc / stdout / stderr（stderr 剥命令面 banner ＋ 退码行 ⇒ **噪声归一**）。逐格对拍 ≥8 格（旗标面 × 退码面）。
// 另断言包装入口与 `cmdDeployLaunch` 三面**逐字同**（= 包装入口只转发、零新增语义）。
func TestDeployLaunchGateMainGridTwoStatesThreeFaces(t *testing.T) {
	cells := []struct {
		name string
		tail []string
		rc   int
	}{
		{"help", []string{"--help"}, 0},
		{"unknown", []string{"--nope-not-a-flag"}, 2},
		{"host-file", []string{"--host", "edge-node", "--file", "/tmp/x.json"}, 0},
		{"host-file-sha", []string{"--host", "edge-node", "--file", "/tmp/x.json", "--expect-sha256", "abc"}, 3},
		{"dry-run", []string{"--dry-run"}, 0},
		{"agent-unit", []string{"--agent", "a", "--unit", "u"}, 3},
		{"target-version", []string{"--target", "t", "--expect-version", "1.2.3"}, 4},
		{"probe-launchdir", []string{"--probe-url", "http://127.0.0.1:1", "--launch-dir", "/tmp/d"}, 5},
	}
	banner := progName + ": 底层 = " + deployLaunchScriptRel + "（**唯一真源** · 命令面只转发、不翻译）\n"
	for _, c := range cells {
		// stub 把 `$*`（argv 逐字）打进 stdout ⇒ 旗标面 passthrough 可观测。
		root := dplSyntheticRepo(t, c.rc, "printf 'ARGS:%s\\n' \"$*\"\n")
		rcS, outS, errS := dplRunScriptDirect(t, root, c.tail...)
		rcW, outW, errW := dplGateMainRun(t, root, c.tail...)
		rcC, outC, errC := dplRun(t, root, c.tail...)
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
			got = strings.TrimSuffix(got, progName+": 发布封装脚本退 "+strconv.Itoa(rcW)+"（原样转发，不翻译）\n")
		}
		if got != errS {
			t.Errorf("[%s] 剥噪声后 stderr 与脚本不同：\n脚本=%q\n包装=%q", c.name, errS, got)
		}
		if errW != errC {
			t.Errorf("[%s] 包装 stderr 与 cmd 不同：\n包装=%q\ncmd =%q", c.name, errW, errC)
		}
	}
}
