// ui_start_test.go —— A6 薄壳（`zerg ui start` · `cmdUIStart`）的**两态对拍**与**哑化负控**。
//
// 为什么这么测（照 M2 口径 · 与 `build_wall_test.go` 同型）：进程内直调 `cmdUIStart`（不引真二进制、
// 不依赖构建），用**仓外合成夹具**给一个可控退码/可控输出的 stub 脚本，验三面：
//
//	① **退码只转发不翻译**：脚本退 N ⇒ 命令面退 N（不许吞成 0，也不许翻成别的）；
//	② **stdout 逐字同**（两态）：走命令面拿到的 stdout == 脚本直跑同一参数的 stdout（逐字节）；
//	③ **递不到脚本 ⇒ 8**（不给结论），**不拿脚本的 1 冒充**。
//
// 哑化负控（"牙"）：把 `cmdUIStart` 改成「一律 return 0 / 吞 stdout」⇒ 上面 ①② 当场 FAIL 并给出
// 本件行号（负控只在**仓外副本**做，✗ 不在落仓件上做 —— 见 `zerg-m2-parallel-go-migration` 技能）。
//
// 另有一条**真件安全态**（`start-zerg-ui.sh` 现读在位才跑，否则 Skip）：把**真脚本**拷进仓外骨架仓
// （无 `bin/zerg-ui`）跑两态 —— 脚本在「二进制不可执行」那一档早退（rc=1），**不会真起 UI**；
// 断言「脚本直跑 ⟷ 走 `cmdUIStart`」的 rc/stdout 逐字同、stderr 剥掉命令面那两行（转发宣言 +
// 退码行）后逐字同 ⇒ 证明命令面**不改原件行为**。★ 之所以只取这一档：本机 `bin/zerg-ui` 在位且可执行，
// 直接跑真脚本会**真起 UI** ⇒ 那是不可接受的副作用（不在用例里做）。
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

// uisSkeletonRepo 建最小合成仓根（只放仓根判据件），返回其路径。
func uisSkeletonRepo(t *testing.T) string {
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

// uisSyntheticRepo 建最小合成仓根 ＋ 一个可控退码/可控 stdout 的 stub `start-zerg-ui.sh`。
func uisSyntheticRepo(t *testing.T, rc int, stdoutBody string) string {
	t.Helper()
	root := uisSkeletonRepo(t)
	scriptPath := filepath.Join(root, uiStartScriptRel)
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/usr/bin/env bash\nset -u\n" + stdoutBody + "exit " + strconv.Itoa(rc) + "\n"
	if err := os.WriteFile(scriptPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// uisRun 走正门（进程内直调 `cmdUIStart`）：给合成仓根，返回 rc / stdout / stderr。
func uisRun(t *testing.T, root string, tail ...string) (int, string, string) {
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

// TestUIStartPassthroughCarriesScriptExitCode —— **只转发、不翻译**：脚本退 N ⇒ 命令面退 N（逐档原样）。
func TestUIStartPassthroughCarriesScriptExitCode(t *testing.T) {
	for _, want := range []int{0, 1, 2} {
		root := uisSyntheticRepo(t, want, "echo 'stub UI：起了一趟'\n")
		rc, _, errb := uisRun(t, root)
		if rc != want {
			t.Fatalf("脚本退 %d ⇒ 命令面退 %d（薄壳只转发、不翻译：两值必须相等）· stderr=%s", want, rc, errb)
		}
	}
}

// TestUIStartStdoutVerbatimTwoStates —— **两态 stdout 逐字同**：命令面的 stdout **就是**脚本直跑的 stdout。
func TestUIStartStdoutVerbatimTwoStates(t *testing.T) {
	for _, st := range []struct {
		name string
		body string
	}{
		{"stA", "echo '✅ UI 已启动（PID 4242 · PPID 1 · 日志 /tmp/zerg-ui.log）'\n"},
		{"stB", "echo 'ℹ️ UI 已在运行（PID 4242）——不重复启动'\n"},
	} {
		root := uisSyntheticRepo(t, 0, st.body)
		_, out, _ := uisRun(t, root)
		direct, err := exec.Command("bash", filepath.Join(root, uiStartScriptRel)).Output()
		if err != nil {
			t.Fatalf("[%s] 脚本直跑起不来：%v", st.name, err)
		}
		if out != string(direct) {
			t.Errorf("[%s] `cmdUIStart` 与脚本直跑 stdout **逐字节不同**：\n命令面=%q\n脚本  =%q", st.name, out, string(direct))
		}
	}
}

// TestUIStartScriptAbsentIsBlocked8 —— 递不到脚本 ⇒ 退 8（不给结论）· ✗ 不拿脚本的 1 冒充。
func TestUIStartScriptAbsentIsBlocked8(t *testing.T) {
	root := uisSkeletonRepo(t) // 骨架仓里**没有** scripts/build/start-zerg-ui.sh
	rc, _, errb := uisRun(t, root)
	if rc != exitBlocked {
		t.Fatalf("脚本不在 ⇒ 命令面退 %d（要 %d）· stderr=%s", rc, exitBlocked, errb)
	}
}

// TestUIStartRealScriptSafeStateVerbatim —— **真件安全态**（`start-zerg-ui.sh` 现读在位才跑，否则 Skip）：
// 把真脚本拷进仓外骨架仓（**无 `bin/zerg-ui`**）⇒ 脚本在「二进制不可执行」那一档早退（rc=1），
// **不会真起 UI**；断言「脚本直跑 ⟷ 走 `cmdUIStart`」rc/stdout 逐字同、stderr 剥掉命令面两行后逐字同。
func TestUIStartRealScriptSafeStateVerbatim(t *testing.T) {
	src := repoRoot()
	if src == "" {
		t.Skip("解析不到仓根（合成环境）⇒ 跳真件安全态（合成夹具已覆盖三面）")
	}
	realScript := filepath.Join(src, uiStartScriptRel)
	real, err := os.ReadFile(realScript)
	if err != nil {
		t.Skipf("真件不在（仅公开快照/桩树会走到这里）：%v", err)
	}
	root := uisSkeletonRepo(t) // 骨架仓：无 bin/zerg-ui ⇒ 脚本走「不可执行」早退档
	scriptPath := filepath.Join(root, uiStartScriptRel)
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptPath, real, 0o755); err != nil {
		t.Fatal(err)
	}
	// 命令面固定加在 stderr 顶部的那行转发宣言（banner）与退码行 —— 只在对比时剥掉，不改本件行为。
	banner := progName + ": 底层 = " + uiStartScriptRel + "（**唯一真源** · 命令面只转发、不翻译）\n"
	// 态1：脚本直跑。
	var o1, e1 bytes.Buffer
	c1 := exec.Command("bash", scriptPath)
	c1.Dir = root
	c1.Stdin = strings.NewReader("")
	c1.Stdout, c1.Stderr = &o1, &e1
	rc1 := 0
	if err := c1.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc1 = ee.ExitCode()
		} else {
			t.Fatalf("脚本直跑起不来：%v", err)
		}
	}
	// 态2：走命令面。
	rc2, out2, err2 := uisRun(t, root)
	if rc1 != rc2 {
		t.Errorf("两态 rc 不同：脚本=%d 命令面=%d", rc1, rc2)
	}
	if rc1 != 1 {
		t.Fatalf("真件安全态期望脚本 rc=1（UI 二进制不可执行早退档），实得 %d（夹具前提不成立）· stderr=%s", rc1, e1.String())
	}
	if o1.String() != out2 {
		t.Errorf("两态 stdout 不同：\n脚本  =%q\n命令面=%q", o1.String(), out2)
	}
	if !strings.HasPrefix(err2, banner) {
		t.Errorf("命令面 stderr 缺转发宣言（期待前缀 %q）：%q", banner, err2)
	}
	got := strings.TrimPrefix(err2, banner)
	got = strings.TrimSuffix(got, progName+": UI 启动脚本退 "+strconv.Itoa(rc2)+"（原样转发，不翻译）\n")
	if e1.String() != got {
		t.Errorf("剥掉命令面两行后 stderr 与脚本不同（改了原件行为）：\n脚本  =%q\n命令面=%q", e1.String(), got)
	}
}
