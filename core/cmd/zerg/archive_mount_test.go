// archive_mount_test.go —— A11 薄壳（`zerg archive mount` · `cmdArchiveMount`）的**两态对拍**与**哑化负控**。
//
// 为什么这么测（照 M2 口径 · 与 `build_wall_test.go` 同型）：进程内直调 `cmdArchiveMount`（不引真二进制、
// 不依赖构建），用**仓外合成夹具**给一个可控退码/可控输出/可回声 argv 的 stub 脚本，验四面：
//
//	① **退码只转发不翻译**：脚本退 0/1/2/64 ⇒ 命令面退同一码（不许吞成 0，也不许翻成别的）；
//	② **stdout 逐字同**（两态）：走命令面拿到的 stdout == 脚本直跑同一参数的 stdout（逐字节）；
//	③ **stderr 只多转发宣言**：命令面 stderr == 转发宣言 ＋ 脚本 stderr（rc=0 档无多余行）⇒ 不改原件行为；
//	④ **递不到脚本 ⇒ 8**（不给结论），**不拿脚本的某个码冒充**。
//
// 哑化负控（"牙"）：把 `cmdArchiveMount` 改成「一律 return 0 / 吞 stdout」⇒ 上面 ①② 当场 FAIL 并给出
// 本件行号（负控只在**仓外副本**做，✗ 不在落仓件上做 —— 见 `zerg-m2-parallel-go-migration` 技能）。
//
// 另有一条**真件两态**（`archive-mount.sh` 现读在位才跑，否则 Skip）：`--help`（rc=0）与未知参数（rc=64）
// 各跑「脚本直跑 ⟷ 走 `cmdArchiveMount`」，断言 rc/stdout 逐字同、stderr 剥掉命令面那两行后逐字同
// （证明命令面**不改原件行为**）。
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

// amSyntheticRepo 建最小合成仓根：仓根判据件 ＋ 一个可控退码/可控输出/可回声 argv 的 stub `archive-mount.sh`。
func amSyntheticRepo(t *testing.T, rc int, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, archiveMountScriptRel)
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	full := "#!/usr/bin/env bash\nset -u\n" + body + "exit " + strconv.Itoa(rc) + "\n"
	if err := os.WriteFile(scriptPath, []byte(full), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// amEmptyRepo 只建仓根判据件、**不建**脚本（核「递不到 ⇒ 8」）。
func amEmptyRepo(t *testing.T) string {
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

// amRun 走正门（进程内直调 `cmdArchiveMount`）：给合成仓根，返回 rc / stdout / stderr。
func amRun(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"archive", "mount"},
		orig: append([]string{"archive", "mount"}, tail...),
	}
	rc := cmdArchiveMount(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// amBanner —— 命令面固定加在 stderr 顶部的那行转发宣言（对比时剥掉，不改本件行为）。
func amBanner() string {
	return progName + ": 底层 = " + archiveMountScriptRel + "（**唯一真源** · 命令面只转发、不翻译）\n"
}

// amRcLine —— 非 0 档命令面补的那行「原样转发」（rc=0 档不出现）。
func amRcLine(rc int) string {
	return progName + ": 归档挂载脚本退 " + strconv.Itoa(rc) + "（原样转发，不翻译）\n"
}

// TestArchiveMountPassthroughCarriesScriptExitCode —— **只转发、不翻译**：逐档原样转出（0/1/2/64）。
func TestArchiveMountPassthroughCarriesScriptExitCode(t *testing.T) {
	for _, want := range []int{0, 1, 2, 64} {
		root := amSyntheticRepo(t, want, "echo 'stub 归档：跑了'\n")
		rc, _, errb := amRun(t, root, "status")
		if rc != want {
			t.Fatalf("脚本退 %d ⇒ 命令面退 %d（薄壳只转发、不翻译：两值必须相等）· stderr=%s", want, rc, errb)
		}
	}
}

// TestArchiveMountStdoutVerbatimTwoStates —— **两态 stdout 逐字同**：命令面的 stdout **就是**脚本直跑的 stdout。
func TestArchiveMountStdoutVerbatimTwoStates(t *testing.T) {
	root := amSyntheticRepo(t, 0, "echo '✅ 已挂：/Volumes/ZERG-ARCHIVE（只读）'\necho '  挂载行 : dev on /Volumes/ZERG-ARCHIVE'\n")
	_, out, _ := amRun(t, root, "status")
	direct, err := exec.Command("bash", filepath.Join(root, archiveMountScriptRel), "status").Output()
	if err != nil {
		t.Fatal(err)
	}
	if out != string(direct) {
		t.Errorf("`cmdArchiveMount` 与脚本直跑 stdout **逐字节不同**：\n命令面=%q\n脚本  =%q", out, string(direct))
	}
}

// TestArchiveMountStderrPassthroughVerbatim —— 命令面 stderr == 转发宣言 ＋ 脚本 stderr（rc=0 档无多余行）。
func TestArchiveMountStderrPassthroughVerbatim(t *testing.T) {
	root := amSyntheticRepo(t, 0, "echo 'stub：只剩宣言这行不同' >&2\n")
	_, _, errb := amRun(t, root, "status")
	direct, err := exec.Command("bash", filepath.Join(root, archiveMountScriptRel), "status").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	want := amBanner() + string(direct)
	if errb != want {
		t.Errorf("命令面 stderr 与「转发宣言 ＋ 脚本 stderr」**逐字节不同**：\n命令面=%q\n期望  =%q", errb, want)
	}
}

// TestArchiveMountForwardsTailArgsVerbatim —— **尾部实参原样转出**：`on`/`off`/`status` 之后的一切都在。
func TestArchiveMountForwardsTailArgsVerbatim(t *testing.T) {
	root := amSyntheticRepo(t, 0, "printf '%s\\n' \"$@\"\n")
	rc, out, _ := amRun(t, root, "on", "status", "--x")
	if rc != 0 {
		t.Fatalf("rc=%d（要 0）", rc)
	}
	if out != "on\nstatus\n--x\n" {
		t.Errorf("尾部实参未逐字转出：%q（要 %q）", out, "on\nstatus\n--x\n")
	}
}

// TestArchiveMountScriptAbsentIsBlocked8 —— 递不到脚本 ⇒ 退 8（不给结论）· ✗ 不拿脚本的某个码冒充。
func TestArchiveMountScriptAbsentIsBlocked8(t *testing.T) {
	root := amEmptyRepo(t)
	rc, _, errb := amRun(t, root, "status")
	if rc != exitBlocked {
		t.Fatalf("脚本不在 ⇒ 命令面退 %d（要 %d）· stderr=%s", rc, exitBlocked, errb)
	}
}

// TestArchiveMountRealScriptTwoStatesVerbatim —— **真件两态**（`archive-mount.sh` 现读在位才跑，否则 Skip）：
// `--help`（rc=0）与未知参数（rc=64）各跑「脚本直跑 ⟷ 走 `cmdArchiveMount`」，断言 rc/stdout 逐字同、
// stderr 剥掉命令面那两行后逐字同（证明命令面**不改原件行为**）。
func TestArchiveMountRealScriptTwoStatesVerbatim(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Skip("解析不到仓根（合成环境）⇒ 跳真件两态（合成夹具已覆盖四面）")
	}
	script := filepath.Join(root, archiveMountScriptRel)
	if _, err := os.Stat(script); err != nil {
		t.Skipf("真件不在（仅公开快照/桩树会走到这里）：%v", err)
	}
	for _, tc := range []struct {
		name string
		tail []string
	}{
		{"help", []string{"--help"}},
		{"unknown-arg", []string{"--nope-not-a-flag"}},
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
		rc2, out2, err2 := amRun(t, root, tc.tail...)
		if rc1 != rc2 {
			t.Errorf("[%s] 两态 rc 不同：脚本=%d 命令面=%d", tc.name, rc1, rc2)
		}
		if o1.String() != out2 {
			t.Errorf("[%s] 两态 stdout 不同：\n脚本  =%q\n命令面=%q", tc.name, o1.String(), out2)
		}
		if !strings.HasPrefix(err2, amBanner()) {
			t.Errorf("[%s] 命令面 stderr 缺转发宣言（期待前缀 %q）：%q", tc.name, amBanner(), err2)
		}
		got := strings.TrimPrefix(err2, amBanner())
		got = strings.TrimSuffix(got, amRcLine(rc2))
		if e1.String() != got {
			t.Errorf("[%s] 剥掉命令面宣言后 stderr 与脚本不同（改了原件行为）：\n脚本  =%q\n命令面=%q", tc.name, e1.String(), got)
		}
	}
}
