// eval_run_all_test.go —— A7 薄壳（`zerg eval run-all` · `cmdEvalRunAll`）的**两态对拍**与**哑化负控**。
//
// 为什么这么测（照 M2 口径 · 与 `build_wall_test.go` / `archive_mount_test.go` 同型）：进程内直调
// `cmdEvalRunAll`（不引真二进制、不依赖构建），用**仓外合成夹具**给一个可控退码/可控输出的 stub 脚本，
// 验四面：
//
//	① **退码只转发不翻译**：脚本退 2 ⇒ 命令面退 2（不许吞成 0，也不许翻成 1）；
//	② **stdout 逐字同**（两态）：走命令面拿到的 stdout == 脚本直跑同一参数的 stdout（逐字节）；
//	③ **位置参数逐字转发**：`<模型> [版本标签]` 原样递给脚本（覆盖 `inv.orig[len(inv.path):]` 取尾）；
//	④ **递不到脚本 ⇒ 8**（不给结论），**不拿脚本的某个码冒充**。
//
// ★ **本件不设「真件两态」用例（与 `build_wall_test.go` 不同）** —— 真源 `scripts/build/run_all.sh`
// **没有用法/帮助分支**：它把第一个位置参数当 `<模型>`，任何一次调用都会**真跑三层**（层1 `go test` ＋
// 层2/3 评测任务集 ＋ 写 `zerg-evals/` 存档）。在测试里调用它 ＝ 真起重活（违「✗ 禁跑门/重编」）。
// ⇒ 真件行为面只在**只读合成桩**上验，真源的实际执行留给人手动跑（`run-all --help` 不是安全档）。
//
// 哑化负控（\"牙\"）：把 `cmdEvalRunAll` 改成「一律 return 0 / 吞 stdout」⇒ 上面 ①②③ 当场 FAIL 并给出
// 本件行号（负控只在**仓外副本**做，✗ 不在落仓件上做 —— 见 `zerg-m2-parallel-go-migration` 技能）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// erallSyntheticRepo 建最小合成仓根：仓根判据件 ＋ 一个可控退码/可控输出/会回显参数 的
// stub `run_all.sh`。stub 会把收到的位置参数逐行回显（供「位置参数逐字转发」用）。
func erallSyntheticRepo(t *testing.T, rc int, stdoutBody string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, evalRunAllScriptRel)
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// `$@` 逐行回显：把命令面递进来的位置参数原样打出来（供 ③ 断言）。
	body := "#!/usr/bin/env bash\nset -u\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done\n" + stdoutBody + "exit " + strconv.Itoa(rc) + "\n"
	if err := os.WriteFile(scriptPath, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// erallRun 走正门（进程内直调 `cmdEvalRunAll`）：给合成仓根，返回 rc / stdout / stderr。
func erallRun(t *testing.T, root string, tail ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO", root)
	var out, errb bytes.Buffer
	inv := &invocation{
		path: []string{"eval", "run-all"},
		orig: append([]string{"eval", "run-all"}, tail...),
	}
	rc := cmdEvalRunAll(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// TestEvalRunAllPassthroughCarriesScriptExitCode —— **只转发、不翻译**：脚本退 2 ⇒ 命令面退 2（不是 0/1）。
func TestEvalRunAllPassthroughCarriesScriptExitCode(t *testing.T) {
	for _, want := range []int{0, 2} {
		root := erallSyntheticRepo(t, want, "echo 'stub 评测批跑：跑了'\n")
		rc, _, errb := erallRun(t, root)
		if rc != want {
			t.Fatalf("脚本退 %d ⇒ 命令面退 %d（薄壳只转发、不翻译：两值必须相等）· stderr=%s", want, rc, errb)
		}
	}
}

// TestEvalRunAllStdoutVerbatimTwoStates —— **两态 stdout 逐字同**：命令面的 stdout **就是**脚本直跑的 stdout。
func TestEvalRunAllStdoutVerbatimTwoStates(t *testing.T) {
	root := erallSyntheticRepo(t, 0, "echo '── 评测批跑 run_all ──'\necho '   ✅ 存档: zerg-evals/results/agent/runs/example-35b-v2.5'\n")
	args := []string{"example-35b", "v2.5"}
	_, out, _ := erallRun(t, root, args...)
	direct, err := exec.Command("bash", append([]string{filepath.Join(root, evalRunAllScriptRel)}, args...)...).Output()
	if err != nil {
		t.Fatal(err)
	}
	if out != string(direct) {
		t.Errorf("`cmdEvalRunAll` 与脚本直跑 stdout **逐字节不同**：\n命令面=%q\n脚本  =%q", out, string(direct))
	}
}

// TestEvalRunAllForwardsPositionalArgsVerbatim —— **位置参数逐字转发**：`<模型> [版本标签]` 原样到脚本。
func TestEvalRunAllForwardsPositionalArgsVerbatim(t *testing.T) {
	root := erallSyntheticRepo(t, 0, "")
	args := []string{"example-35b", "v2.5-20261007"}
	_, out, _ := erallRun(t, root, args...)
	direct, err := exec.Command("bash", append([]string{filepath.Join(root, evalRunAllScriptRel)}, args...)...).Output()
	if err != nil {
		t.Fatal(err)
	}
	if out != string(direct) {
		t.Errorf("位置参数转发与脚本直跑不同：\n命令面=%q\n脚本  =%q", out, string(direct))
	}
	if out != args[0]+"\n"+args[1]+"\n" {
		t.Errorf("命令面未把位置参数逐字转发（取尾 `inv.orig[len(inv.path):]` 出错）：got=%q want=%q",
			out, args[0]+"\n"+args[1]+"\n")
	}
}

// TestEvalRunAllScriptAbsentIsBlocked8 —— 递不到脚本 ⇒ 退 8（不给结论）· ✗ 不拿脚本的某个码冒充。
func TestEvalRunAllScriptAbsentIsBlocked8(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, errb := erallRun(t, root)
	if rc != exitBlocked {
		t.Fatalf("脚本不在 ⇒ 命令面退 %d（要 %d）· stderr=%s", rc, exitBlocked, errb)
	}
}
