package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// wopRunEnv —— 在受控 env 下跑入口，返回 (rc, stdout, stderr)；退出时逐键还原。
// ★ 不用 init()/TestMain：包级初始化会与同包他件相撞（`go vet` 才判得出来）。
func wopRunEnv(t *testing.T, env map[string]string, unset []string) (int, string, string) {
	t.Helper()
	type saved struct {
		v   string
		had bool
	}
	old := map[string]saved{}
	touch := func(k string) {
		if _, done := old[k]; done {
			return
		}
		v, ok := os.LookupEnv(k)
		old[k] = saved{v, ok}
	}
	for k := range env {
		touch(k)
	}
	for _, k := range unset {
		touch(k)
	}
	for k := range old {
		os.Unsetenv(k)
	}
	for k, v := range env {
		os.Setenv(k, v)
	}
	defer func() {
		for k, s := range old {
			if s.had {
				os.Setenv(k, s.v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()
	var out, errb bytes.Buffer
	rc := writeoutProbeGateMain(&out, &errb, nil)
	return rc, out.String(), errb.String()
}

// wopRunEnvArgv —— 同 wopRunEnv，但把 argv 显式喂给入口（钉「本件忽略 argv」= py 裸脚本口径）。
func wopRunEnvArgv(t *testing.T, env map[string]string, unset []string, argv []string) (int, string, string) {
	t.Helper()
	type saved struct {
		v   string
		had bool
	}
	old := map[string]saved{}
	touch := func(k string) {
		if _, done := old[k]; done {
			return
		}
		v, ok := os.LookupEnv(k)
		old[k] = saved{v, ok}
	}
	for k := range env {
		touch(k)
	}
	for _, k := range unset {
		touch(k)
	}
	for k := range old {
		os.Unsetenv(k)
	}
	for k, v := range env {
		os.Setenv(k, v)
	}
	defer func() {
		for k, s := range old {
			if s.had {
				os.Setenv(k, s.v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()
	var out, errb bytes.Buffer
	rc := writeoutProbeGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// ★ 口径差③（见件头）：py 的回溯正文含**解释器面的帧**（`<frozen os>` / `os.py`）、随解释器版本与
// 安装路径漂 ⇒ 本件**只对齐 rc ＋ stdout**，stderr 走具名文案。下面两条 = 「防波门探的那一面」
// （argv=`--self-test` · env 两枚皆缺）的逐字期望值 —— 谁把 rc 改回 2 或换了文案，格8 立刻带行号红。
const wopWantKeyErrDir = "KeyError: 'PROBE_DIR'（py 真身裸回溯 rc=1 · 回溯帧落在解释器面（<frozen os> / os.py）⇒ 正文不复刻 · 见件头口径差③）\n"
const wopWantKeyErrOutside = "KeyError: 'PROBE_OUTSIDE'（py 真身裸回溯 rc=1 · 回溯帧落在解释器面（<frozen os> / os.py）⇒ 正文不复刻 · 见件头口径差③）\n"
const wopWantMakedirsTail = "（py 真身裸回溯 rc=1 · 回溯帧落在解释器面（<frozen os> / os.py）⇒ 正文不复刻 · 见件头口径差③）\n"

func wopASCIIPath(t *testing.T, p string) string {
	t.Helper()
	for i := 0; i < len(p); i++ {
		if p[i] < 0x20 || p[i] > 0x7e || p[i] == '"' || p[i] == '\\' {
			t.Fatalf("夹具路径含需转义字符，断言式会失真: %q", p)
		}
	}
	return p
}

// 格1：两枚皆可写 ⇒ ok / OPEN；且 inside.txt 真落。
func TestWriteoutProbeInsideOkOutsideOpen(t *testing.T) {
	td := t.TempDir()
	pd := filepath.Join(td, "probe")
	po := filepath.Join(td, "outside.txt")
	rc, out, errs := wopRunEnv(t, map[string]string{"PROBE_DIR": pd, "PROBE_OUTSIDE": po}, nil)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, errs)
	}
	want := "{\"write_inside\": \"ok\", \"write_outside\": \"OPEN\", \"write_outside_target\": \"" + wopASCIIPath(t, po) + "\"}\n"
	if out != want {
		t.Fatalf("stdout 不符\n got=%q\nwant=%q", out, want)
	}
	if _, err := os.Stat(filepath.Join(pd, "inside.txt")); err != nil {
		t.Fatalf("inside.txt 未落: %v", err)
	}
}

// 格2：outside 指向一个**已存在的目录** ⇒ py `open(dir,"w")` = IsADirectoryError。
func TestWriteoutProbeOutsideIsDir(t *testing.T) {
	td := t.TempDir()
	pd := filepath.Join(td, "probe")
	po := filepath.Join(td, "outside_dir")
	if err := os.MkdirAll(po, 0o755); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := wopRunEnv(t, map[string]string{"PROBE_DIR": pd, "PROBE_OUTSIDE": po}, nil)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	want := "{\"write_inside\": \"ok\", \"write_outside\": \"BLOCKED:IsADirectoryError\", \"write_outside_target\": \"" + po + "\"}\n"
	if out != want {
		t.Fatalf("stdout 不符\n got=%q\nwant=%q", out, want)
	}
}

// 格3：outside 的父目录不存在 ⇒ FileNotFoundError。
func TestWriteoutProbeOutsideParentMissing(t *testing.T) {
	td := t.TempDir()
	pd := filepath.Join(td, "probe")
	po := filepath.Join(td, "no_such_dir", "x.txt")
	rc, out, _ := wopRunEnv(t, map[string]string{"PROBE_DIR": pd, "PROBE_OUTSIDE": po}, nil)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	want := "{\"write_inside\": \"ok\", \"write_outside\": \"BLOCKED:FileNotFoundError\", \"write_outside_target\": \"" + po + "\"}\n"
	if out != want {
		t.Fatalf("stdout 不符\n got=%q\nwant=%q", out, want)
	}
}

// 格4：w/inside.txt 位置是个**目录** ⇒ write_inside = FAIL:IsADirectoryError（outside 仍探）。
func TestWriteoutProbeInsideIsDir(t *testing.T) {
	td := t.TempDir()
	pd := filepath.Join(td, "probe")
	if err := os.MkdirAll(filepath.Join(pd, "inside.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	po := filepath.Join(td, "outside.txt")
	rc, out, _ := wopRunEnv(t, map[string]string{"PROBE_DIR": pd, "PROBE_OUTSIDE": po}, nil)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	want := "{\"write_inside\": \"FAIL:IsADirectoryError\", \"write_outside\": \"OPEN\", \"write_outside_target\": \"" + po + "\"}\n"
	if out != want {
		t.Fatalf("stdout 不符\n got=%q\nwant=%q", out, want)
	}
}

// 格5：PROBE_DIR 已是一枚**普通文件** ⇒ py `os.makedirs(exist_ok=True)` 抛 FileExistsError（裸回溯
// rc=1）⇒ 本件对齐 rc=1、stdout 空；且错误类名按 py 语义纠回（✗ 不是 Go 的 NotADirectoryError）。
func TestWriteoutProbeDirIsFile(t *testing.T) {
	td := t.TempDir()
	pd := filepath.Join(td, "probe")
	if err := os.WriteFile(pd, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	po := filepath.Join(td, "outside.txt")
	rc, out, errs := wopRunEnv(t, map[string]string{"PROBE_DIR": pd, "PROBE_OUTSIDE": po}, nil)
	if rc != 1 {
		t.Fatalf("rc=%d（应 1 —— 逐字沿用 py 裸回溯档）stderr=%q", rc, errs)
	}
	if out != "" {
		t.Fatalf("stdout 应空，得 %q", out)
	}
	want := "FileExistsError: '" + pd + "'" + wopWantMakedirsTail
	if errs != want {
		t.Fatalf("stderr 不符\n got=%q\nwant=%q", errs, want)
	}
}

// 格6：PROBE_DIR 未设 ⇒ py `os.environ["PROBE_DIR"]` KeyError（裸回溯 rc=1）⇒ 本件对齐 rc=1 ·
// stdout 空 · stderr 恰为具名文案（回溯正文不复刻 · 见件头口径差③）。
func TestWriteoutProbeDirEnvUnset(t *testing.T) {
	td := t.TempDir()
	rc, out, errs := wopRunEnv(t, map[string]string{"PROBE_OUTSIDE": filepath.Join(td, "o.txt")}, []string{"PROBE_DIR"})
	if rc != 1 || out != "" {
		t.Fatalf("rc=%d out=%q（应 1/空）", rc, out)
	}
	if errs != wopWantKeyErrDir {
		t.Fatalf("stderr 不符\n got=%q\nwant=%q", errs, wopWantKeyErrDir)
	}
}

// 格7：PROBE_OUTSIDE 未设 ⇒ 同上（KeyError: 'PROBE_OUTSIDE' · rc=1 · stdout 空）。
func TestWriteoutProbeOutsideEnvUnset(t *testing.T) {
	td := t.TempDir()
	rc, out, errs := wopRunEnv(t, map[string]string{"PROBE_DIR": filepath.Join(td, "p")}, []string{"PROBE_OUTSIDE"})
	if rc != 1 || out != "" {
		t.Fatalf("rc=%d out=%q（应 1/空）", rc, out)
	}
	if errs != wopWantKeyErrOutside {
		t.Fatalf("stderr 不符\n got=%q\nwant=%q", errs, wopWantKeyErrOutside)
	}
}

// 格8（本单新钉的**那一面**）：`check-py-go-pairing-drift` 探该键时给的 argv = `--self-test`、
// 而 py 是裸脚本不读 argv ⇒ 本件必须**同样忽略 argv**：同一 env 下 `--self-test` 与无参**逐字同**。
// ★ 负控（哑化）：把入口的 rc 改回 2、或改成 re-parses argv ⇒ 本格必带行号红。
func TestWriteoutProbeDriftProbeFaceArgvSelfTest(t *testing.T) {
	td := t.TempDir()
	po := filepath.Join(td, "o.txt")
	rc, out, errs := wopRunEnv(t, map[string]string{"PROBE_OUTSIDE": po}, []string{"PROBE_DIR"})
	if rc != 1 || out != "" || errs != wopWantKeyErrDir {
		t.Fatalf("无参档：rc=%d out=%q err=%q（应 1/空/具名）", rc, out, errs)
	}
	// 同一 env，改喂 --self-test：argv 被忽略 ⇒ 三面逐字同（= py 裸脚本口径）。
	rc2, out2, errs2 := wopRunEnvArgv(t, map[string]string{"PROBE_OUTSIDE": po}, []string{"PROBE_DIR"}, []string{"--self-test"})
	if rc2 != rc || out2 != out || errs2 != errs {
		t.Fatalf("argv 面分叉：--self-test ⇒ rc=%d out=%q err=%q ⟷ 无参 rc=%d out=%q err=%q",
			rc2, out2, errs2, rc, out, errs)
	}
}

// 格9：w 目录非可写（0555）⇒ inside 写 = PermissionError（非 root 才有牙）。
func TestWriteoutProbeInsidePermission(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 不受目录权限约束 ⇒ 本档不适用")
	}
	td := t.TempDir()
	pd := filepath.Join(td, "probe")
	if err := os.MkdirAll(pd, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(pd, 0o755) })
	po := filepath.Join(td, "outside.txt")
	rc, out, _ := wopRunEnv(t, map[string]string{"PROBE_DIR": pd, "PROBE_OUTSIDE": po}, nil)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	want := "{\"write_inside\": \"FAIL:PermissionError\", \"write_outside\": \"OPEN\", \"write_outside_target\": \"" + po + "\"}\n"
	if out != want {
		t.Fatalf("stdout 不符\n got=%q\nwant=%q", out, want)
	}
}

// 格10：outside 的父目录非可写（0555）⇒ BLOCKED:PermissionError。
func TestWriteoutProbeOutsidePermission(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 不受目录权限约束 ⇒ 本档不适用")
	}
	td := t.TempDir()
	pd := filepath.Join(td, "probe")
	ro := filepath.Join(td, "ro")
	if err := os.MkdirAll(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
	po := filepath.Join(ro, "x.txt")
	rc, out, _ := wopRunEnv(t, map[string]string{"PROBE_DIR": pd, "PROBE_OUTSIDE": po}, nil)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	want := "{\"write_inside\": \"ok\", \"write_outside\": \"BLOCKED:PermissionError\", \"write_outside_target\": \"" + po + "\"}\n"
	if out != want {
		t.Fatalf("stdout 不符\n got=%q\nwant=%q", out, want)
	}
}

// wopPyJoin 语义三态（含空段/绝对后段/尾斜杠）—— 恒同构钉死。
func TestWriteoutProbePyJoinSemantics(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"a", "b", "a/b"},
		{"a/", "b", "a/b"},
		{"a", "", "a/"},
		{"", "a", "a"},
		{"a", "/abs", "/abs"},
		{"w", "inside.txt", "w/inside.txt"},
	}
	for _, c := range cases {
		if got := wopPyJoin(c.a, c.b); got != c.want {
			t.Fatalf("wopPyJoin(%q,%q)=%q want %q", c.a, c.b, got, c.want)
		}
	}
}
