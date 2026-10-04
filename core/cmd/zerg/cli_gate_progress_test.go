// cli_gate_progress_test.go —— `zerg gate progress` / `zerg gate wait` 的判据（设计-门进度可查-v0.3）。
//
// 落进程内 `RunForTest`（读当前源码 · 本批不重建 `bin/`），**照 `cli_gate_results_test.go` 的风格**。
//
// 三态**负控必须有牙**（设计稿 §六「成对负控三态」）：
//
//	① 在跑趟（合成 outdir：results.tsv 有内容 ＋ 一个**真活**的门进程引用它）⇒ 退 0、判「还在跑」；
//	   且**门禁的红不渗进 progress 自己的退码**（表里有 FAIL 仍退 0 —— §3.6 / gh 的 watch 陷阱反面）。
//	② 已收工趟（完整 results.tsv ＋ 无活进程 ＋ mtime 压旧）⇒ 判「已收工」；有 FAIL ⇒ 退 1。
//	③ 判不出趟（空目录 / 不可读）⇒ **退 8**（断言**不落绿**：rc != 0）。
//	  ＋ 多趟活 ⇒ **打印候选清单 ＋ 非零退 2**（§3.7.1 / §十.4 · 绝不弹提示、绝不读 stdin）。
//	  ＋ `--json` 首行是版本对象（§十.5）。
//	  ＋ 只读：跑前/跑后趟目录逐件指纹相同。
package main_test

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gpWriteTSV —— 造一趟合成产物：`状态\t步名\trc\t耗时\t日志路径`（照脚本 `run_step` 落盘式）。
func gpWriteTSV(t *testing.T, dir string, rows [][5]string) {
	t.Helper()
	var b strings.Builder
	for i, r := range rows {
		log := filepath.Join(dir, "01-step.log")
		if err := os.WriteFile(log, []byte("合成日志\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = i
		b.WriteString(strings.Join([]string{r[0], r[1], r[2], r[3], log}, "\t") + "\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "results.tsv"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gpStartFakeGate —— 起一个**真活**的门进程：`bash <repo>/scripts/gates/precommit-gates.sh --outdir <dir>`。
// 它 cmd 含 `precommit-gates.sh`（⇒ `gateReapScan` 认它）与 `--outdir <dir>`（⇒ progress 认它属这一趟）。
func gpStartFakeGate(t *testing.T, repo, dir string) *exec.Cmd {
	t.Helper()
	script := filepath.Join(repo, "scripts", "gates", "precommit-gates.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\nset -u\nsleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, "--outdir", dir)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatalf("起假门进程失败：%v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	// 给 ps 一点时间看见它（进程表快照）。
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		out, _ := exec.Command("ps", "-o", "pid=", "-p", gpItoa(cmd.Process.Pid)).Output()
		if strings.TrimSpace(string(out)) != "" {
			return cmd
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("假门进程 pid=%d 在进程表里看不到", cmd.Process.Pid)
	return nil
}

func gpItoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// gpIsolateEnv —— 把 holder 锁与 TMPDIR 都指到合成位置（不碰真机产物、不碰真锁）。
func gpIsolateEnv(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("TMPDIR", base)
	t.Setenv("ZERG_GATE_RUN_ROOTS", "") // 隔离：只扫本用例这一根，别去碰真机 /var/folders（GAP-20261004-05）
	t.Setenv("ZERG_GATE_LOCK_DIR", filepath.Join(base, "lock"))
	return base
}

// ① 在跑趟：真活进程引用它 ⇒ 退 0、判「还在跑」；★ FAIL 不渗进 progress 自己的退码。
func TestGateProgressInFlight(t *testing.T) {
	gpIsolateEnv(t)
	repo := syntheticRepo(t, "sleep 60\n")
	dir := t.TempDir()
	gpWriteTSV(t, dir, [][5]string{
		{"PASS", "a", "0", "1s", ""},
		{"FAIL", "b", "1", "2s", ""}, // 有红，但这一趟**还在跑** ⇒ progress 自身退 0（§3.6）
	})
	gpStartFakeGate(t, repo, dir)

	rc, out, errb := runZergRepo(t, repo, "gate", "progress", "--dir", dir)
	if rc != 0 {
		t.Fatalf("在跑趟 ⇒ 读成功退 0（门禁的红不渗进 progress 的退码 · §3.6），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "还在跑") {
		t.Errorf("要判出「还在跑」：\n%s", out)
	}
	if !strings.Contains(out, "PASS 1") || !strings.Contains(out, "FAIL 1") {
		t.Errorf("四数要照结果表数出来：\n%s", out)
	}
}

// ② 已收工趟：无活进程 ⇒ 判「已收工」；有 FAIL ⇒ 退 1（与那趟自身退码一致 · §3.6）。
func TestGateProgressFinished(t *testing.T) {
	gpIsolateEnv(t)
	repo := syntheticRepo(t, "sleep 60\n")
	dir := t.TempDir()
	gpWriteTSV(t, dir, [][5]string{
		{"PASS", "a", "0", "1s", ""},
		{"FAIL", "b", "1", "2s", ""},
	})
	// 压旧 mtime：证明「静态产物 + 无活进程」= 已收工，而不是靠「文件新」蒙混。
	old := time.Now().Add(-30 * time.Minute)
	if err := os.Chtimes(filepath.Join(dir, "results.tsv"), old, old); err != nil {
		t.Fatal(err)
	}
	rc, out, errb := runZergRepo(t, repo, "gate", "progress", "--dir", dir)
	if rc != 1 {
		t.Fatalf("已收工且那一趟有红（FAIL）⇒ 退 1（§3.6：与那趟自己一致 —— 门禁的红**只**在已收工时透进退码），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "已收工") {
		t.Errorf("要判出「已收工」：\n%s", out)
	}
	// 全 PASS 的已收工趟 ⇒ 退 0。
	dir2 := t.TempDir()
	gpWriteTSV(t, dir2, [][5]string{{"PASS", "a", "0", "1s", ""}})
	if err := os.Chtimes(filepath.Join(dir2, "results.tsv"), old, old); err != nil {
		t.Fatal(err)
	}
	if rc2, _, _ := runZergRepo(t, repo, "gate", "progress", "--dir", dir2); rc2 != 0 {
		t.Errorf("已收工且全绿 ⇒ 退 0，得到 %d", rc2)
	}
}

// ③ 判不出趟：空目录 / 无 results.tsv ⇒ **退 8**（负控有牙：必须不落绿）。
func TestGateProgressUnknownNotGreen(t *testing.T) {
	gpIsolateEnv(t)
	repo := syntheticRepo(t, "sleep 60\n")
	empty := t.TempDir() // 目录在、results.tsv 不在

	rc, _, errb := runZergRepo(t, repo, "gate", "progress", "--dir", empty)
	if rc != 8 {
		t.Fatalf("空趟 ⇒ 判不出退 8（不是 0）· §3.8，得到 %d · stderr=%s", rc, errb)
	}
	if rc == 0 {
		t.Fatal("负控失败：判不出竟落绿（rc=0）")
	}
	// 指到不存在的目录 ⇒ 同样 8。
	if rc2, _, _ := runZergRepo(t, repo, "gate", "progress", "--dir", "/nonexistent-zerg-zz"); rc2 != 8 {
		t.Errorf("不存在的那一趟 ⇒ 退 8，得到 %d", rc2)
	}
	// `--last` 在一枚候选都没有时也判不出（TMPDIR 已隔离为空）。
	if rc3, _, _ := runZergRepo(t, repo, "gate", "progress", "--last"); rc3 != 8 {
		t.Errorf("TMPDIR 隔离后 `--last` 无候选 ⇒ 退 8，得到 %d", rc3)
	}
}

// ＋ 多趟活 ⇒ **打印候选清单 ＋ 非零退 2**（§3.7.1 / §十.4：绝不弹提示、绝不读 stdin）。
func TestGateProgressMultipleLiveExitsNonZero(t *testing.T) {
	gpIsolateEnv(t)
	repo := syntheticRepo(t, "sleep 60\n")
	d1, d2 := t.TempDir(), t.TempDir()
	gpWriteTSV(t, d1, [][5]string{{"PASS", "a", "0", "1s", ""}})
	gpWriteTSV(t, d2, [][5]string{{"PASS", "a", "0", "1s", ""}})
	gpStartFakeGate(t, repo, d1)
	gpStartFakeGate(t, repo, d2)

	rc, out, errb := runZergRepo(t, repo, "gate", "progress")
	if rc != 2 {
		t.Fatalf("多趟活且没给 --dir ⇒ 非零退 2（绝不弹提示 · §十.4），得到 %d · out=%s · stderr=%s", rc, out, errb)
	}
	// 不卡死某串汉字：只判「点名了这一档的拒绝理由」（多趟活 ＋ **逐个**候选趟 ＋ 请用 --dir）。
	if strings.TrimSpace(errb) == "" || !strings.Contains(errb, d1) || !strings.Contains(errb, d2) {
		t.Errorf("要明写「多趟活」并**逐个**打印候选趟（含 `--dir` 提示），得到：\n%s", errb)
	}
	if rc == 0 {
		t.Fatal("负控失败：多趟活竟落绿")
	}
}

// ＋ `--json` 首行是版本对象（§十.5）；字段 snake_case；未知值「省略键 + <字段>_known:false」。
func TestGateProgressJSONHasSchemaFirstLine(t *testing.T) {
	gpIsolateEnv(t)
	repo := syntheticRepo(t, "sleep 60\n")
	dir := t.TempDir()
	gpWriteTSV(t, dir, [][5]string{{"PASS", "a", "0", "1s", ""}})
	if err := os.Chtimes(filepath.Join(dir, "results.tsv"), time.Now().Add(-time.Minute), time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	rc, out, errb := runZergRepo(t, repo, "gate", "progress", "--dir", dir, "--json", "run_dir,steps_done,alive,deepest_pid")
	if rc != 0 {
		t.Fatalf("退 0，得到 %d · stderr=%s", rc, errb)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("`--json` 至少两行（首行 schema + 一条记录）：%q", out)
	}
	var head struct {
		Type    string `json:"type"`
		Schema  string `json:"schema"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &head); err != nil {
		t.Fatalf("首行不是 JSON：%v · %q", err, lines[0])
	}
	if head.Type != "schema" || head.Schema != "zerg.gate.progress" || head.Version != "1.1" {
		t.Errorf("首行版本对象不对：type=%q schema=%q version=%q", head.Type, head.Schema, head.Version)
	}
	var rec map[string]json.RawMessage
	if err := json.Unmarshal([]byte(lines[1]), &rec); err != nil {
		t.Fatalf("第二条不是 JSON：%v · %q", err, lines[1])
	}
	if string(rec["type"]) != `"progress"` {
		t.Errorf("第二条 type 要是 progress：%s", lines[1])
	}
	if _, ok := rec["run_dir"]; !ok {
		t.Errorf("点名的 run_dir 要给：%s", lines[1])
	}
	// 已收工趟无活进程 ⇒ deepest_pid 未知 ⇒ 省略键 + deepest_pid_known:false（§十.5）。
	if _, ok := rec["deepest_pid"]; ok {
		t.Errorf("未知值不许用 null 冒充：%s", lines[1])
	}
	if string(rec["deepest_pid_known"]) != "false" {
		t.Errorf("未知值要写 <字段>_known:false：%s", lines[1])
	}
	// GAP-20261002-19 新增两格（1.0 → 1.1 的向后兼容增补）：整趟已耗时 ＋ 最慢步清单。
	rc1, out1, errb1 := runZergRepo(t, repo, "gate", "progress", "--dir", dir, "--json", "elapsed_secs,slowest_steps")
	if rc1 != 0 {
		t.Fatalf("新字段可点（退 0），得到 %d · stderr=%s", rc1, errb1)
	}
	ls := strings.Split(strings.TrimRight(out1, "\n"), "\n")
	if len(ls) < 2 {
		t.Fatalf("`--json` 至少两行：%q", out1)
	}
	var rec2 map[string]json.RawMessage
	if err := json.Unmarshal([]byte(ls[1]), &rec2); err != nil {
		t.Fatalf("第二条不是 JSON：%v · %q", err, ls[1])
	}
	if _, ok := rec2["elapsed_secs"]; !ok {
		t.Errorf("已收工趟要报 elapsed_secs（整趟已耗时）：%s", ls[1])
	}
	if raw := string(rec2["slowest_steps"]); raw == "" || raw[0] != '[' {
		t.Errorf("slowest_steps 要是 JSON 数组（恒 `[]`，绝不 null）：%s", ls[1])
	}
	// 负控：字段表外的名字 ⇒ 退 2 + stderr 列字段表（与仓内 `gate results --json` 同规）。
	if rc, _, e := runZergRepo(t, repo, "gate", "progress", "--dir", dir, "--json", "nosuch_field"); rc != 2 || !strings.Contains(e, "run_dir") {
		t.Errorf("字段表外 ⇒ 退 2 + stderr 列字段表，得到 %d · stderr=%q", rc, e)
	}
}

// ＋ 只读：跑前/跑后整个趟目录逐件指纹相同（不删不改任何日志）。
func TestGateProgressIsReadOnly(t *testing.T) {
	gpIsolateEnv(t)
	repo := syntheticRepo(t, "sleep 60\n")
	dir := t.TempDir()
	gpWriteTSV(t, dir, [][5]string{{"PASS", "a", "0", "1s", ""}, {"FAIL", "b", "1", "2s", ""}})
	before := treeFingerprint(t, dir)
	for _, argv := range [][]string{
		{"gate", "progress", "--dir", dir},
		{"gate", "progress", "--dir", dir, "--json", "run_dir"},
		{"gate", "progress", "--dir", "/nonexistent-zerg-zz"},
	} {
		_, _, _ = runZergRepo(t, repo, argv...)
	}
	if after := treeFingerprint(t, dir); after != before {
		t.Errorf("只读面跑完那一趟变了（不删不改那一条破了）：\n前=%s\n后=%s", before, after)
	}
}

// ＋ `gate wait`：已收工趟 ⇒ 立即返回；`--timeout 0` ⇒ 拒执退 2（不许 0=无穷 · §十.6）。
func TestGateWaitBoundedAndExitStatus(t *testing.T) {
	gpIsolateEnv(t)
	repo := syntheticRepo(t, "sleep 60\n")
	dir := t.TempDir()
	gpWriteTSV(t, dir, [][5]string{{"PASS", "a", "0", "1s", ""}, {"FAIL", "b", "1", "2s", ""}})
	old := time.Now().Add(-30 * time.Minute)
	if err := os.Chtimes(filepath.Join(dir, "results.tsv"), old, old); err != nil {
		t.Fatal(err)
	}
	// 已收工：不带 --exit-status ⇒ 退 0。
	if rc, _, errb := runZergRepo(t, repo, "gate", "wait", "--dir", dir, "--timeout", "5"); rc != 0 {
		t.Fatalf("已收工 ⇒ 退 0（读成功），得到 %d · stderr=%s", rc, errb)
	}
	// 带 --exit-status ⇒ 把门禁的红透成退码（FAIL ⇒ 1）。
	if rc, _, _ := runZergRepo(t, repo, "gate", "wait", "--dir", dir, "--timeout", "5", "--exit-status"); rc != 1 {
		t.Errorf("--exit-status 下 FAIL ⇒ 退 1（§十.6），得到 %d", rc)
	}
	// `--timeout 0` ⇒ 拒执 2（不学 kubectl 的 0=无穷）。
	if rc, _, errb := runZergRepo(t, repo, "gate", "wait", "--dir", dir, "--timeout", "0"); rc != 2 || strings.TrimSpace(errb) == "" || !strings.Contains(errb, "--timeout") {
		t.Errorf("`--timeout 0` ⇒ 退 2 ＋ stderr 点名该档拒绝理由（--timeout 不许 0=无穷），得到 %d · stderr=%q", rc, errb)
	}
}

// ＋ 非 TTY 下 `--pick` 拒执（§3.1 增补 G4 · 绝不弹提示、绝不读 stdin）。
func TestGateProgressPickRejectedNonTTY(t *testing.T) {
	gpIsolateEnv(t)
	repo := syntheticRepo(t, "sleep 60\n")
	dir := t.TempDir()
	gpWriteTSV(t, dir, [][5]string{{"PASS", "a", "0", "1s", ""}})
	// RunForTest 的 stdout 是 bytes.Buffer ⇒ 非 TTY。
	if rc, _, errb := runZergRepo(t, repo, "gate", "progress", "--dir", dir, "--pick"); rc != 2 || strings.TrimSpace(errb) == "" || !strings.Contains(errb, "--pick") {
		t.Errorf("非 TTY 下 `--pick` ⇒ 退 2 ＋ stderr 点名该档拒绝理由（--pick 需 TTY），得到 %d · stderr=%q", rc, errb)
	}
}

// ＋ 跨趟根（GAP-20261004-05）：`gate progress --last` 也要扫全部趟根（与 `gate results --last` 同源）。
// 负控有牙：回落单根 ⇒ base 为空 ⇒ 一枚候选都没有 ⇒ 退 8；本用例断言取到 other 那趟。
func TestGateProgressLastScansAllRunRoots(t *testing.T) {
	base, other := t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", base)
	t.Setenv("ZERG_GATE_RUN_ROOTS", other)
	repo := syntheticRepo(t, "sleep 60\n")
	d := filepath.Join(other, "zerg-gates-20260303-000000")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	gpWriteTSV(t, d, [][5]string{{"PASS", "a", "0", "1s", ""}})
	// 收工态（无活进程 · 压旧 mtime）⇒ 「已收工」+ 退 0。
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(d, "results.tsv"), old, old); err != nil {
		t.Fatal(err)
	}
	rc, out, errb := runZergRepo(t, repo, "gate", "progress", "--last")
	if rc != 0 {
		t.Fatalf("`--last` ⇒ 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "zerg-gates-20260303-000000") {
		t.Errorf("`gate progress --last` 要扫到「加宽趟根」里那趟 —— 只扫一个根就会漏：\n%s", out)
	}
}

// ＋ 活体样本：门进程命令行**不带 `--outdir`**（脚本默认命名档）时，`--dir <活趟>` 不许误判
// 「已收工 · 根 pid 0」（GAP-20261004-05 · 追加症状）。判活靠**根级持锁者**：
// 趟目录的根里有活的单实例持锁者、且这一趟是该根下最新候选 ⇒ 这一趟「还在跑」，root_pid = 持锁者 pid。
func TestGateProgressDirLiveViaRootLock(t *testing.T) {
	base := t.TempDir()
	t.Setenv("TMPDIR", base) // 私有根：别去读真机持锁者
	t.Setenv("ZERG_GATE_RUN_ROOTS", "")
	t.Setenv("ZERG_GATE_LOCK_DIR", filepath.Join(base, "lock"))
	repo := syntheticRepo(t, "sleep 60\n") // 假门脚本
	root := t.TempDir()
	dir := filepath.Join(root, "zerg-gates-20261004-172712")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gpWriteTSV(t, dir, [][5]string{{"PASS", "a", "0", "1s", ""}, {"PASS", "b", "0", "2s", ""}})
	// 起假门：命令行**不带 --outdir**（复刻脚本默认命名档的形态）。
	script := filepath.Join(repo, "scripts", "gates", "precommit-gates.sh")
	cmd := exec.Command("bash", script)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatalf("起假门进程失败：%v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	// 写「这一趟所在的根」里的单实例持锁者（门脚本的默认落点）。
	lockDir := filepath.Join(root, "zerg-gate-instance")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lockDir, "gate-run.lock"),
		[]byte("pid: "+gpItoa(cmd.Process.Pid)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errb := runZergRepo(t, repo, "gate", "progress", "--dir", dir)
	if rc != 0 {
		t.Fatalf("活趟读成功 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "还在跑") {
		t.Errorf("命令行不带 --outdir 的活趟要判「还在跑」（根里有活持锁者）—— 不许误判「已收工」：\n%s", out)
	}
	if !strings.Contains(out, "根 pid "+gpItoa(cmd.Process.Pid)) {
		t.Errorf("root_pid 要指出活门进程（%d）：\n%s", cmd.Process.Pid, out)
	}
}

// ★ 缺口 `GAP-20261004-07`：进度分母要数**本趟真会跑的步**（＝脚本 `--list` 的真步数）——
// `self_test()` 里那批 `add_step self "自检-…"` 只在 `--self-test` 档才 add，**不算进分母**。
// 负控有牙：夹具脚本三条声明（2 真步 + 1 self）⇒ `steps_total` 必须是 2（改前会数成 3）。
func TestGateProgressStepsTotalExcludesSelfFixtures(t *testing.T) {
	gpIsolateEnv(t)
	body := "add_step go \"门甲\" tri \"${t}\" \"true\"\n" +
		"add_step go \"门乙\" rc \"${t}\" \"true\"\n" +
		"add_step self \"自检-必过-1\" rc \"${t}\" \"true\"\n"
	repo := syntheticRepo(t, body)
	dir := t.TempDir()
	gpWriteTSV(t, dir, [][5]string{{"PASS", "a", "0", "1s", ""}})
	rc, out, errb := runZergRepo(t, repo, "gate", "progress", "--dir", dir, "--json", "steps_total,steps_done")
	if rc != 0 {
		t.Fatalf("退 0，得到 %d · stderr=%s", rc, errb)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("`--json` 至少两行：%q", out)
	}
	var rec map[string]json.RawMessage
	if err := json.Unmarshal([]byte(lines[1]), &rec); err != nil {
		t.Fatalf("第二条不是 JSON：%v · %q", err, lines[1])
	}
	if got := string(rec["steps_total"]); got != "2" {
		t.Errorf("分母要数非 self 的真步（2），实得 %s（`add_step self` 的自检夹具步不算）", got)
	}
}
