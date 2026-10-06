// swap_ui_test.go —— A3 薄壳（`zerg swap ui` · `cmdSwapUI`）的**两态对拍**与**哑化负控**。
//
// 为什么这么测（照 M2 口径 · 与 `build_wall_test.go` / `swap_agent_test.go` 同型）：进程内直调 `cmdSwapUI`
// （不引真二进制、不依赖构建/服务/启动器），用**仓外合成夹具**给一个可控退码/可控输出的 stub 脚本，验三面：
//
//	① **退码只转发不翻译**：脚本退 0 / 1 / 2 ⇒ 命令面退 0 / 1 / 2（不许吞成 0，也不许翻成别的）；
//	② **stdout 逐字同**（两态）：走命令面拿到的 stdout == 脚本直跑同一参数的 stdout（逐字节）；
//	③ **递不到脚本 ⇒ 8**（不给结论），**不拿脚本的 2 冒充**。
//
// 哑化负控（"牙"）：把 `cmdSwapUI` 改成「一律 return 0 / 吞 stdout」⇒ 上面 ①② 当场 FAIL 并给出
// 本件行号（负控只在**仓外副本**做，✗ 不在落仓件上做 —— 见 `zerg-m2-parallel-go-migration` 技能）。
//
// 另有一条**真件两态**（`zerg-swap-ui.sh` 现读在位才跑，否则 Skip）：`--help`（rc=0）与未知参数（rc=2）
// 各跑「脚本直跑 ⟷ 走 `cmdSwapUI`」，断言 stdout/rc 逐字同、stderr 只多命令面那行**转发宣言**
// （banner 固定前缀 ⇒ 剥掉后与脚本 stderr 逐字同 ⇒ 证明「不改原件行为」）。
//
// ★ 安全线（换件面 · 与 A1/A2 同）：本测件**只用不产生副作用**的档（`--help` / 未知参数）——**绝不**真跑换件、
// 绝不带 `--swap`（真换件档虽未开放 ⇒ 也一律不碰）。
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

// suSyntheticRepo 建最小合成仓根：仓根判据件 ＋ 一个可控退码/可控 stdout 的 stub `zerg-swap-ui.sh`。
func suSyntheticRepo(t *testing.T, rc int, stdoutBody string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, swapUIScriptRel)
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/usr/bin/env bash\nset -u\n" + stdoutBody + "exit " + strconv.Itoa(rc) + "\n"
	if err := os.WriteFile(scriptPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// suRun 走正门（进程内直调 `cmdSwapUI`）：给合成仓根，返回 rc / stdout / stderr。
func suRun(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"swap", "ui"},
		orig: append([]string{"swap", "ui"}, tail...),
	}
	rc := cmdSwapUI(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// TestSwapUIPassthroughCarriesScriptExitCode —— **只转发、不翻译**：脚本退 0/1/2 ⇒ 命令面同码（不吞不翻）。
func TestSwapUIPassthroughCarriesScriptExitCode(t *testing.T) {
	for _, want := range []int{0, 1, 2} {
		root := suSyntheticRepo(t, want, "echo 'stub UI 换件：闸跑完了'\n")
		rc, _, errb := suRun(t, root)
		if rc != want {
			t.Fatalf("脚本退 %d ⇒ 命令面退 %d（薄壳只转发、不翻译：两值必须相等）· stderr=%s", want, rc, errb)
		}
	}
}

// TestSwapUIStdoutVerbatimTwoStates —— **两态 stdout 逐字同**：命令面的 stdout **就是**脚本直跑的 stdout。
func TestSwapUIStdoutVerbatimTwoStates(t *testing.T) {
	root := suSyntheticRepo(t, 0, "echo '── UI 换件 zerg-ui ──'\necho '   [闸] A 工作树半成品 ⇒ PASS'\n")
	_, out, _ := suRun(t, root)
	direct, err := exec.Command("bash", filepath.Join(root, swapUIScriptRel)).Output()
	if err != nil {
		t.Fatal(err)
	}
	if out != string(direct) {
		t.Errorf("`cmdSwapUI` 与脚本直跑 stdout **逐字节不同**：\n命令面=%q\n脚本  =%q", out, string(direct))
	}
}

// TestSwapUIScriptAbsentIsBlocked8 —— 递不到脚本 ⇒ 退 8（不给结论）· ✗ 不拿脚本的 2 冒充。
func TestSwapUIScriptAbsentIsBlocked8(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, errb := suRun(t, root)
	if rc != exitBlocked {
		t.Fatalf("脚本不在 ⇒ 命令面退 %d（要 %d）· stderr=%s", rc, exitBlocked, errb)
	}
}

// TestSwapUIRealScriptTwoStatesVerbatim —— **真件两态**（`zerg-swap-ui.sh` 现读在位才跑，否则 Skip）：
// `--help`（rc=0）与未知参数（rc=2）各跑「脚本直跑 ⟷ 走 `cmdSwapUI`」，断言 rc/stdout 逐字同、
// stderr 剥掉命令面那行转发宣言后逐字同（证明命令面**不改原件行为**）。
//
// ★ 安全线：只走这两枚**不产生副作用**的档；✗ 不带 `--swap`（真换件档未开放，也一律不碰）。
func TestSwapUIRealScriptTwoStatesVerbatim(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Skip("解析不到仓根（合成环境）⇒ 跳真件两态（合成夹具已覆盖三面）")
	}
	script := filepath.Join(root, swapUIScriptRel)
	if _, err := os.Stat(script); err != nil {
		t.Skipf("真件不在（仅公开快照/桩树会走到这里）：%v", err)
	}
	// 命令面固定加在 stderr 顶部的那行转发宣言（banner）—— 只在对比时剥掉，不改本件行为。
	banner := progName + ": 底层 = " + swapUIScriptRel + "（**唯一真源** · 命令面只转发、不翻译）\n"
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
		rc2, out2, err2 := suRun(t, root, tc.tail...)
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
		got = strings.TrimSuffix(got, progName+": UI 换件脚本退 "+strconv.Itoa(rc2)+"（原样转发，不翻译）\n")
		if e1.String() != got {
			t.Errorf("[%s] 剥掉命令面宣言后 stderr 与脚本不同（改了原件行为）：\n脚本  =%q\n命令面=%q", tc.name, e1.String(), got)
		}
	}
}
