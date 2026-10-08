// build_wall_test.go —— A4 薄壳（`zerg build wall` · `cmdBuildWall`）的**委派形态**与**哑化负控**。
//
// 为什么这么测（照 M2 口径 · 与 `ui_start_test.go` 同型）：进程内直调 `cmdBuildWall`（不引真二进制、
// 不依赖构建），用**仓外合成夹具**（一个假 `go` + 一份占位真身源码）给一个可控退码/可控输出、
// 并把 stub 收到的 **name/argv/cwd** 记回来，验五面：
//
//	① **委派形态**：命令面起的进程 = `go run ./cmd/zerg-build …`，工作目录 = `<仓根>/scripts/build`
//	   —— 本单（2026-10-09）= 把旧壳 `bash scripts/build/build-wall.sh` 改指 **Go 真身**，**差异仅限命令串**；
//	② **退码只转发不翻译**：真身退 2 ⇒ 命令面退 2（不许吞成 0，也不许翻成 1）；
//	③ **stdout 逐字同**（两态）：走命令面拿到的 stdout == stub 直跑同一参数的 stdout（逐字节）；
//	④ **递不到真身 ⇒ 8**（不给结论），**不拿真身的 2 冒充**（真身源码不在 / go 工具链不可用，两格）；
//
// 哑化负控（"牙"）：把 `cmdBuildWall` 改成「一律 return 0 / 吞 stdout」⇒ 上面 ①②③④ 当场 FAIL 并给出
// 本件行号（负控只在**仓外副本**做，✗ 不在落仓件上做 —— 见 `zerg-m2-parallel-go-migration` 技能）。
//
// 另有一条**真件两态**（真仓根 + 真 `go` 都在位才跑，否则 Skip）：`--help`（rc=0）与未知参数（rc=2）
// 各跑「真身直跑（`(cd scripts/build && go run ./cmd/zerg-build …)`）⟷ 走 `cmdBuildWall`」，
// 断言 rc/stdout 逐字同、stderr 只多命令面那行**转发宣言**（banner 固定前缀 ⇒ 剥掉后与真身 stderr 逐字同）。
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

// bwFakeGo —— 造一个只在 PATH 里出现的**假 `go`**：把 stub 收到的 **cwd 与 argv** 记到 `$BW_LOG`，
// 打 `$BW_OUT`，退 `$BW_RC`。用途 = 在合成仓根里替掉真 `go run`（真身不真编，只验命令串/转发）。
func bwFakeGo(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	body := `#!/usr/bin/env bash
set -u
if [ -n "${BW_LOG:-}" ]; then
  { printf 'cwd\t%s\n' "$PWD"; for a in "$@"; do printf 'arg\t%s\n' "$a"; done; } > "$BW_LOG"
fi
printf '%s' "${BW_OUT:-}"
exit "${BW_RC:-0}"
`
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// bwSyntheticRepo 建最小合成仓根：仓根判据件 ＋ **真身源码占位**（递达性闸读它）。
func bwSyntheticRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	realBody := filepath.Join(root, filepath.FromSlash(buildWallGoMain))
	if err := os.MkdirAll(filepath.Dir(realBody), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(realBody, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// bwRun 走正门（进程内直调 `cmdBuildWall`）：给合成仓根与假 `go` 目录，返回 rc / stdout / stderr。
// stub 记下的 cwd/argv 另经 `$BW_LOG` 读回（见 bwFakeGo）。
func bwRun(t *testing.T, root, fakeBin string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"build", "wall"},
		orig: append([]string{"build", "wall"}, tail...),
	}
	rc := cmdBuildWall(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// TestBuildWallDelegatesToGoRealBody —— **委派形态**：起的进程 = `go run ./cmd/zerg-build …`，
// cwd = `<仓根>/scripts/build`。这条就是「差异仅限命令串」的机器判据（旧 `bash <壳>` ⇒ 新 `go run <真身>`）。
func TestBuildWallDelegatesToGoRealBody(t *testing.T) {
	root := bwSyntheticRepo(t)
	fakeBin := bwFakeGo(t)
	logPath := filepath.Join(t.TempDir(), "argv.log")
	t.Setenv("BW_LOG", logPath)
	t.Setenv("BW_RC", "0")
	t.Setenv("BW_OUT", "stub 茧壁：编了\n")
	rc, _, errb := bwRun(t, root, fakeBin, "--debug")
	if rc != 0 {
		t.Fatalf("rc=%d（要 0）· stderr=%s", rc, errb)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("假 `go` 没被起（真身没被委派）：%v", err)
	}
	want := []string{
		"cwd\t" + filepath.Join(root, filepath.FromSlash(buildWallGoDir)),
		"arg\trun",
		"arg\t" + buildWallGoPkg,
		"arg\t--debug",
	}
	got := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("委派形态不符（name/args/dir）：\n现读=%q\n要  =%q\n（本单只换命令串：旧 `bash <build-wall.sh>` ⇒ 新 `go run %s`；cwd=<仓根>/%s）",
			got, want, buildWallGoPkg, buildWallGoDir)
	}
}

// TestBuildWallPassthroughCarriesRealBodyExitCode —— **只转发、不翻译**：真身退 2 ⇒ 命令面退 2（不是 0/1）。
func TestBuildWallPassthroughCarriesRealBodyExitCode(t *testing.T) {
	fakeBin := bwFakeGo(t)
	for _, want := range []int{0, 2} {
		root := bwSyntheticRepo(t)
		t.Setenv("BW_RC", strconv.Itoa(want))
		t.Setenv("BW_OUT", "stub 茧壁：编了\n")
		rc, _, errb := bwRun(t, root, fakeBin)
		if rc != want {
			t.Fatalf("真身退 %d ⇒ 命令面退 %d（薄壳只转发、不翻译：两值必须相等）· stderr=%s", want, rc, errb)
		}
	}
}

// TestBuildWallStdoutVerbatimTwoStates —— **两态 stdout 逐字同**：命令面的 stdout **就是**真身直跑的 stdout。
func TestBuildWallStdoutVerbatimTwoStates(t *testing.T) {
	root := bwSyntheticRepo(t)
	fakeBin := bwFakeGo(t)
	body := "── 茧壁 zerg-wall ──\n   ✅ bin/zerg-wall  598208 字节\n"
	t.Setenv("BW_RC", "0")
	t.Setenv("BW_OUT", body)
	_, out, _ := bwRun(t, root, fakeBin)
	direct, err := exec.Command(filepath.Join(fakeBin, "go")).Output()
	if err != nil {
		t.Fatal(err)
	}
	if out != string(direct) {
		t.Errorf("`cmdBuildWall` 与真身直跑 stdout **逐字节不同**：\n命令面=%q\n真身  =%q", out, string(direct))
	}
}

// TestBuildWallRealBodyAbsentIsBlocked8 —— 递不到真身（真身源码不在）⇒ 退 8（不给结论）· ✗ 不拿真身的 2 冒充。
func TestBuildWallRealBodyAbsentIsBlocked8(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeBin := bwFakeGo(t)
	t.Setenv("BW_RC", "2")
	rc, _, errb := bwRun(t, root, fakeBin)
	if rc != exitBlocked {
		t.Fatalf("真身不在 ⇒ 命令面退 %d（要 %d）· stderr=%s", rc, exitBlocked, errb)
	}
}

// TestBuildWallGoToolchainAbsentIsBlocked8 —— go 工具链不可用 ⇒ 退 8（不给结论 · 与上格成对）。
func TestBuildWallGoToolchainAbsentIsBlocked8(t *testing.T) {
	root := bwSyntheticRepo(t)
	empty := t.TempDir() // 空 PATH ⇒ LookPath("go") 失败，且不带任何真身可跑的工具
	t.Setenv("ZERG_REPO", root)
	t.Setenv("PATH", empty)
	var out, errb bytes.Buffer
	inv := &invocation{path: []string{"build", "wall"}, orig: []string{"build", "wall"}}
	if rc := cmdBuildWall(inv, &out, &errb); rc != exitBlocked {
		t.Fatalf("无 go ⇒ 命令面退 %d（要 %d）· stderr=%s", rc, exitBlocked, errb.String())
	}
}

// TestBuildWallRealBodyTwoStatesVerbatim —— **真件两态**（真仓根 + 真 `go` 都在位才跑，否则 Skip）：
// `--help`（rc=0）与未知参数（rc=2）各跑「真身直跑 ⟷ 走 `cmdBuildWall`」，断言 rc/stdout 逐字同、
// stderr 剥掉命令面那行转发宣言后逐字同（证明命令面**不改真身行为**）。
func TestBuildWallRealBodyTwoStatesVerbatim(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Skip("解析不到仓根（合成环境）⇒ 跳真件两态（合成夹具已覆盖五面）")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(buildWallGoMain))); err != nil {
		t.Skipf("真身不在（仅桩树/公开快照会走到这里）：%v", err)
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("无 go 工具链 ⇒ 跳真件两态：%v", err)
	}
	// 命令面固定加在 stderr 顶部的那行转发宣言（banner）—— 只在对比时剥掉，不改本件行为。
	banner := progName + ": 底层 = " + buildWallGoCmd + "（**唯一真源** · 命令面只转发、不翻译）\n"
	for _, tc := range []struct {
		name string
		tail []string
	}{
		{"help", []string{"--help"}},
		{"unknown-flag", []string{"--nope-not-a-flag"}},
	} {
		// 态1：真身直跑（`(cd scripts/build && go run ./cmd/zerg-build …)`）。
		var o1, e1 bytes.Buffer
		c1 := exec.Command(goBin, append([]string{"run", buildWallGoPkg}, tc.tail...)...)
		c1.Dir = filepath.Join(root, filepath.FromSlash(buildWallGoDir))
		c1.Stdin = strings.NewReader("")
		c1.Stdout, c1.Stderr = &o1, &e1
		rc1 := 0
		if err := c1.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				rc1 = ee.ExitCode()
			} else {
				t.Fatalf("[%s] 真身直跑起不来：%v", tc.name, err)
			}
		}
		// 态2：走命令面（PATH 用真 `go` 所在目录 ⇒ 不替掉真身）。
		rc2, out2, err2 := bwRun(t, root, filepath.Dir(goBin), tc.tail...)
		if rc1 != rc2 {
			t.Errorf("[%s] 两态 rc 不同：真身=%d 命令面=%d", tc.name, rc1, rc2)
		}
		if o1.String() != out2 {
			t.Errorf("[%s] 两态 stdout 不同：\n真身  =%q\n命令面=%q", tc.name, o1.String(), out2)
		}
		if !strings.HasPrefix(err2, banner) {
			t.Errorf("[%s] 命令面 stderr 缺转发宣言（期待前缀 %q）：%q", tc.name, banner, err2)
		}
		got := strings.TrimPrefix(err2, banner)
		got = strings.TrimSuffix(got, progName+": 茧壁构建真身退 "+strconv.Itoa(rc2)+"（原样转发，不翻译）\n")
		if e1.String() != got {
			t.Errorf("[%s] 剥掉命令面宣言后 stderr 与真身不同（改了真身行为）：\n真身  =%q\n命令面=%q", tc.name, e1.String(), got)
		}
	}
}
