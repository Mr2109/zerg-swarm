// ui_start_test.go —— A6 正门（`zerg ui start` · `cmdUIStart`）**真搬**后的**两态三面**回归牙。
//
// 为什么这么测（照 M2 口径 · 与 `build_wall_test.go` 同型）：进程内直调 `cmdUIStart`（不引真二进制、
// 不依赖构建），用**仓外合成夹具**给合成仓根，断言：
//
//	① **真搬不 exec 壳**：仓根里放一枚**毒壳**（打印 POISON / 退 42）⇒ 正门 stderr 若出现 POISON
//	   即证明实现仍在 exec 脚本（壳转账），本用例当红；
//	② **两态三面逐字同**：`scripts/build/start-zerg-ui.sh` 在盘 ⟷ 移出仓（退役态），正门
//	   rc/stdout/stderr 三面逐字同，且退役态**不得**退 8/2（旧壳转账正是靠 `os.Stat(脚本)` 退 8）；
//	③ **与脚本直跑逐字同**（真件在盘才跑，否则 Skip）：把**真脚本**拷进仓外合成仓根，跑三个**确定性**档
//	   （不可执行 / 已在运行 / 起不来）⇒ 「脚本直跑 ⟷ 正门」rc/stdout/stderr 逐字同（三档 × 三面 = 9 格）。
//	   ★ 不取「真起 UI」那一档：本机 `bin/zerg-ui` 在位且可执行，直接跑真脚本会**真起 UI**（不可接受的副作用）。
//
// 哑化负控（"牙"）：把 `cmdUIStart` 改回「exec 脚本」或「一律 return 0 / 吞 stdout」⇒ ①② 当场 FAIL
// 并给出本件行号（负控只在**仓外副本**做，✗ 不在落仓件上做 —— 见 `zerg-m2-parallel-go-migration` 技能）。
//
// 另有一条**包装入口栅格**（`uiStartGateMain` · inproc 同签名）：与 `cmdUIStart` 三面逐字同（只转发、零新增语义）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// uisSkeletonRepo —— 建最小合成仓根（带仓根判据件），返回其路径（已解符号链接 ⇒ 与脚本侧 `pwd` 同形）。
func uisSkeletonRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// uisInstallShell —— 把 body 当 `scripts/build/start-zerg-ui.sh` 落进合成仓根（0755）。
func uisInstallShell(t *testing.T, root, body string) string {
	t.Helper()
	p := filepath.Join(root, "scripts", "build", "start-zerg-ui.sh")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// uisRunCmd —— 进程内直调 `cmdUIStart`（给合成仓根）⇒ rc / stdout / stderr。
func uisRunCmd(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"ui", "start"},
		orig: append([]string{"ui", "start"}, tail...),
	}
	rc := cmdUIStart(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// uisRunGate —— 进程内直调包装入口 `uiStartGateMain` ⇒ rc / stdout / stderr。
func uisRunGate(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	rc := uiStartGateMain(&out, &errb, tail)
	return rc, out.String(), errb.String()
}

// uisRunShellDirect —— 直跑合成仓根里的脚本（对照态 · 单参数 · 无附旗标）。
func uisRunShellDirect(t *testing.T, root string) (int, string, string) {
	t.Helper()
	script := filepath.Join(root, "scripts", "build", "start-zerg-ui.sh")
	var o, e bytes.Buffer
	c := exec.Command("bash", script)
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

// ① 真搬不 exec 壳 ＋ 退役态两态三面逐字同（★ 退役态不得退 8/2）。
func TestUiStartNotExecutableRetiredStateByteIdentical(t *testing.T) {
	root := uisSkeletonRepo(t)
	// 放一枚**毒壳**：实现若仍 exec 脚本 ⇒ stderr 出现 POISON、rc=42 ⇒ 本用例当红（= 仍是壳转账）。
	uisInstallShell(t, root, "#!/usr/bin/env bash\necho POISON-SHELL-RAN >&2\nexit 42\n")

	wantErr := "❌ UI 二进制不可执行: " + filepath.Join(root, uiStartBinRel) +
		"（先 cargo build --release -p zerg-ui → 拷到 bin/ → codesign -s - --force）\n"

	rc1, out1, err1 := uisRunCmd(t, root)
	if strings.Contains(err1, "POISON") {
		t.Fatalf("实现仍在 exec 脚本（stderr 出现 POISON ⇒ 仍是壳转账）：%q", err1)
	}
	if rc1 != exitFail || out1 != "" || err1 != wantErr {
		t.Fatalf("态①（壳在盘）期望 rc=%d · stdout 空 · stderr=%q；实得 rc=%d out=%q err=%q",
			exitFail, wantErr, rc1, out1, err1)
	}

	// 退役态：把壳移出仓 ⇒ 三面必须逐字同。
	if err := os.Remove(filepath.Join(root, "scripts", "build", "start-zerg-ui.sh")); err != nil {
		t.Fatal(err)
	}
	rc2, out2, err2 := uisRunCmd(t, root)
	if rc1 != rc2 || out1 != out2 || err1 != err2 {
		t.Fatalf("两态三面不逐字同：\n态① rc=%d out=%q err=%q\n态② rc=%d out=%q err=%q",
			rc1, out1, err1, rc2, out2, err2)
	}
	if rc2 == exitBlocked || rc2 == exitUsage {
		t.Fatalf("退役态**不得**退 8/2（旧壳转账靠 os.Stat(脚本) 退 8 的病）：rc=%d", rc2)
	}
}

// uisPrepAlreadyRunning —— 造「已经在运行」：真二进制（`/bin/sleep` 的**符号链接** —— 拷贝二进制在 macOS
// 上会被签名策略杀掉），argv[0] = `<root>/bin/zerg-ui` ⇒ 逐 PID 点名命中（同族 `.bak` 不会命中）。
func uisPrepAlreadyRunning(t *testing.T, root string) func() {
	t.Helper()
	if _, err := os.Stat("/bin/sleep"); err != nil {
		t.Skip("无 /bin/sleep ⇒ 跳 already-running 场景")
	}
	bin := filepath.Join(root, uiStartBinRel)
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/bin/sleep", bin); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "60")
	if err := cmd.Start(); err != nil {
		t.Skipf("起 sleep 夹具失败：%v", err)
	}
	time.Sleep(300 * time.Millisecond) // 等它进进程表
	return func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

// uisPrepLaunchFailure —— 造「起得来又立刻退」：一枚立刻退的脚本；其 ps 名（`/usr/bin/env bash …`）≠ `$BIN`
// ⇒ 回读必落空 ⇒ 走失败支（睡 4 秒 + 日志尾部）。
func uisPrepLaunchFailure(t *testing.T, root string) func() {
	t.Helper()
	bin := filepath.Join(root, uiStartBinRel)
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/usr/bin/env bash\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return func() {}
}

// ③ 与真脚本直跑逐字同（三档 × 三面 = 9 格）＋ 每档退役态逐字同。真件不在（退役后）⇒ Skip。
func TestUiStartParityWithRealShellTwoStatesThreeFaces(t *testing.T) {
	src := repoRoot()
	if src == "" {
		t.Skip("解析不到仓根 ⇒ 跳真件对拍（合成夹具 ①② 已覆盖三面）")
	}
	real, err := os.ReadFile(filepath.Join(src, "scripts", "build", "start-zerg-ui.sh"))
	if err != nil {
		t.Skipf("真件不在（退役后即走到这里 —— 两态不变由 TestUiStartNotExecutableRetiredStateByteIdentical 担保）：%v", err)
	}
	shBody := string(real)

	scen := []struct {
		name string
		prep func(t *testing.T, root string) func()
	}{
		{"not-executable", func(t *testing.T, root string) func() { return func() {} }},
		{"already-running", uisPrepAlreadyRunning},
		{"launch-failure", uisPrepLaunchFailure},
	}
	for _, sc := range scen {
		t.Run(sc.name, func(t *testing.T) {
			root := uisSkeletonRepo(t)
			uisInstallShell(t, root, shBody)
			t.Setenv("ZERG_UI_LOG", filepath.Join(root, "ui.log"))
			cleanup := sc.prep(t, root)
			defer cleanup()

			rcS, outS, errS := uisRunShellDirect(t, root)
			rcG, outG, errG := uisRunCmd(t, root)
			if rcS != rcG || outS != outG || errS != errG {
				t.Fatalf("[%s] 两态三面不逐字同：\n脚本 rc=%d out=%q err=%q\n正门 rc=%d out=%q err=%q",
					sc.name, rcS, outS, errS, rcG, outG, errG)
			}

			// 退役态：把壳移出仓 ⇒ 正门三面逐字同（✗ 不得 rc=8/2）。
			if err := os.Remove(filepath.Join(root, "scripts", "build", "start-zerg-ui.sh")); err != nil {
				t.Fatal(err)
			}
			rcR, outR, errR := uisRunCmd(t, root)
			if rcR != rcG || outR != outG || errR != errG {
				t.Fatalf("[%s] 退役态不逐字同：\n在盘 rc=%d out=%q err=%q\n退役 rc=%d out=%q err=%q",
					sc.name, rcG, outG, errG, rcR, outR, errR)
			}
			if rcR == exitBlocked || rcR == exitUsage {
				t.Fatalf("[%s] 退役态**不得**退 8/2：rc=%d", sc.name, rcR)
			}
		})
	}
}

// ④ 包装入口栅格：`uiStartGateMain` 与 `cmdUIStart` 三面逐字同（只转发 · 零新增语义）。
func TestUiStartGateMainForwardsCmdUIStart(t *testing.T) {
	root := uisSkeletonRepo(t)
	rcC, outC, errC := uisRunCmd(t, root)
	rcW, outW, errW := uisRunGate(t, root, "--not-a-flag", "x")
	if rcC != rcW || outC != outW || errC != errW {
		t.Fatalf("包装入口与 cmdUIStart 三面不同：\ncmd  rc=%d out=%q err=%q\ngate rc=%d out=%q err=%q",
			rcC, outC, errC, rcW, outW, errW)
	}
}
