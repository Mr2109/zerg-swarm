// cli_gate_check_test.go —— M1 批B：`zerg gate check` 判据转发壳的**进程内真用例**。
//
// 为什么这层（缺口 `GAP-20261001-10`）：清清单 M1-01/M1-03 引的 `go test -run TestGateCheck`
// **全仓 0 命中** ⇒ 那几条验收命令一直**空跑**（假绿）。本件把 TestGateCheck 补在盘上：
//
//	① 未知判据名 ⇒ 退 2（保守拒 · 给可用名）；
//	② 子件失败**逐字透传** ⇒ 壳退子件的码（此例 7），且台账落一行 `raw_rc=7`（六字段齐）；
//	③ `ZERG_GATE_FORWARD=off` ⇒ 退 8（`detail=judge_broken`）· 子件**没跑**（成对负控：`on` ⇒ 子件真跑）；
//	④ 壳级 `--dry-run --json` 全表档 ⇒ 退 0 · `count` == 名录件数 · **零副作用**（成对负控：真跑有副作用）；
//	⑤ 件级锁竞争 ⇒ 退 2（**不给结论** · 不 exec ⇒ 不落台账）。
package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gctTestPyExe —— 起判据件子进程用的解释器**钉死**（缺口 `GAP-20261001-61`：✗ 不用 PATH 上的裸
//
//	`python3`；同 `cicgTestPython` / `ccatTestPyExe` / `tsmPython` 一律 `/usr/bin/python3`）。
const gctTestPyExe = "/usr/bin/python3"

// gctWrite 往合成仓根写一件（自动建目录）。
func gctWrite(t *testing.T, root, rel, body string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// gctInv 造一条 `zerg gate check …` 的 invocation。`orig` = `os.Args[1:]`（**不含 argv0**）⇒
// `orig[2:]` 恰是壳身后的逐字参数（首枚 = 判据名）。
func gctInv(args ...string) *invocation {
	return &invocation{orig: append([]string{"gate", "check"}, args...)}
}

// gctRun 进程内跑转发壳一次 ⇒ (rc, stdout, stderr)。
func gctRun(inv *invocation, root string) (int, string, string) {
	var out, errb strings.Builder
	rc := gateCheck(inv, &out, &errb, root)
	return rc, out.String(), errb.String()
}

// ① 名字不在册 ⇒ **保守拒**（退 2 · 给可用名 · ✗ 不模糊匹配）。
func TestGateCheckUnknownNameIsUsageError(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/known-good.py", "print('ok')\n")
	rc, _, errb := gctRun(gctInv("no-such-judge"), root)
	if rc != 2 {
		t.Fatalf("未知判据名 ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "不在册") || !strings.Contains(errb, "known-good") {
		t.Errorf("要明说「不在册」并**给可用名列表**：%q", errb)
	}
	if !strings.Contains(errb, "judge_resolved=false") {
		t.Errorf("壳自身错 ⇒ judge_resolved=false：%q", errb)
	}
}

// ② 子件退非 0 ⇒ 壳**原样透传**该码（不是「非 0 一律 1」），并落一行台账（六字段齐 · `raw_rc` = 子件码）。
func TestGateCheckChildExitPassthroughAndLedger(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/boom.py", "import sys\nprint('boom-out')\nsys.exit(7)\n")
	rc, out, errb := gctRun(gctInv("boom"), root)
	if rc != 7 {
		t.Fatalf("子件退 7 ⇒ 壳退 7（逐字透传），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "boom-out") {
		t.Errorf("子件 stdout 要逐字对接：%q", out)
	}
	led := filepath.Join(root, "scripts", "gates", "judge-run-ledger.jsonl")
	b, err := os.ReadFile(led)
	if err != nil {
		t.Fatalf("失败支要落一行台账（append-only）：%v", err)
	}
	line := strings.TrimSpace(string(b))
	if line == "" {
		t.Fatal("台账行是空的")
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(line), &row); err != nil {
		t.Fatalf("台账行不是合法 JSON：%v · %q", err, line)
	}
	for _, k := range []string{"raw_rc", "detail", "sha256", "head", "judge_resolved", "dry_run"} {
		if _, ok := row[k]; !ok {
			t.Errorf("台账行缺六字段之一的 `%s`：%q", k, line)
		}
	}
	if got, ok := row["raw_rc"].(float64); !ok || got != 7 {
		t.Errorf("`raw_rc` 要装子件原样退码 7：%q", line)
	}
	if row["detail"] == nil || row["detail"] == "" {
		t.Errorf("`detail` 不许空：%q", line)
	}
}

// ③ 转发开关：`off` ⇒ **不 exec** ⇒ 退 8（`detail=judge_broken`）；成对负控 `on` ⇒ 子件真跑。
func TestGateCheckForwardOffIsBlocked(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "RAN")
	gctWrite(t, root, "scripts/writer.py",
		"import os\nopen(os.environ['GCT_MARKER'],'w').write('x')\n")
	t.Setenv("GCT_MARKER", marker)

	// 负控（off）：子件不许跑。
	t.Setenv("ZERG_GATE_FORWARD", "off")
	rc, _, errb := gctRun(gctInv("writer"), root)
	if rc != 8 {
		t.Fatalf("ZERG_GATE_FORWARD=off ⇒ 退 8（不给结论 · judge_broken），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "detail=judge_broken") {
		t.Errorf("要逐字带 `detail=judge_broken`：%q", errb)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("off ⇒ **不 exec**，子件不该跑（marker 却在：%v）", err)
	}

	// 正控（on）：非 `off` ⇒ 转发开，子件真跑。
	t.Setenv("ZERG_GATE_FORWARD", "on")
	rc2, _, errb2 := gctRun(gctInv("writer"), root)
	if rc2 != 0 {
		t.Fatalf("ZERG_GATE_FORWARD=on ⇒ 照转发（子件退 0 ⇒ 壳退 0），得到 %d · stderr=%s", rc2, errb2)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("on ⇒ 子件应当真跑并产生副作用：%v", err)
	}
}

// ④ 壳级 `--dry-run --json` 全表档：退 0 · `count` == 名录件数 · **零副作用**（成对负控：真跑有副作用）。
func TestGateCheckDryRunFullTableZeroSideEffect(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "RAN")
	gctWrite(t, root, "scripts/one.py",
		"import os\nopen(os.environ['GCT_MARKER'],'w').write('x')\n")
	gctWrite(t, root, "scripts/two.sh", "#!/usr/bin/env bash\nexit 0\n")
	t.Setenv("GCT_MARKER", marker)
	t.Setenv("ZERG_GATE_FORWARD", "on")

	rc, out, errb := gctRun(gctInv("--dry-run", "--json"), root)
	if rc != 0 {
		t.Fatalf("全表干跑 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Count    int  `json:"count"`
		Resolved int  `json:"resolved"`
		DryRun   bool `json:"dry_run"`
		Entries  []struct {
			Name          string `json:"name"`
			JudgeResolved bool   `json:"judge_resolved"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("--json 不是合法包封：%v · %q", err, out)
	}
	if env.Count != 2 || env.Resolved != 2 || len(env.Entries) != 2 {
		t.Fatalf("count/resolved/entries 要 == 名录件数 2：%+v", env)
	}
	if !env.DryRun {
		t.Errorf("--dry-run ⇒ dry_run 标记为真：%+v", env)
	}
	for _, e := range env.Entries {
		if !e.JudgeResolved {
			t.Errorf("逐件 judge_resolved 应为真：%+v", e)
		}
	}
	// 零副作用：干跑不许 exec（marker 不在）。
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("干跑**零副作用**，子件不该跑（marker 却在：%v）", err)
	}
	// 成对负控：真跑会产生副作用（否则本用例是恒绿）。
	if rc2, _, errb2 := gctRun(gctInv("one"), root); rc2 != 0 {
		t.Fatalf("负控：真跑 one 应退 0，得到 %d · stderr=%s", rc2, errb2)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("负控失败：真跑应当产生副作用（marker 缺）⇒ 本用例可能恒绿：%v", err)
	}
}

// ⑤ 件级锁竞争 ⇒ 退 2（**不给结论** · 不 exec ⇒ 不落台账）。调**现成的** `judge-lock.py` 造真竞争。
func TestGateCheckLockContentionIsBlocked(t *testing.T) {
	repoRel := filepath.Join("..", "..", "..")
	realTool := filepath.Join(repoRel, "scripts", "gates", "judge-lock.py")
	if _, err := os.Stat(realTool); err != nil {
		t.Skipf("现成 judge-lock.py 不在（%v）", err)
	}
	root := t.TempDir()
	tool := gctWrite(t, root, "scripts/gates/judge-lock.py", "")
	src, err := os.ReadFile(realTool)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tool, src, 0o644); err != nil {
		t.Fatal(err)
	}
	gctWrite(t, root, "scripts/lockprobe.py", "print('lockprobe')\n")
	gctWrite(t, root, "scripts/other.py", "print('other')\n")

	lockDir := t.TempDir()
	t.Setenv("ZERG_GATE_LOCK_DIR", lockDir)

	// 起**真第二个进程**持同名锁（内部 `--_selftest-hold` 真持锁数秒）。
	// 解释器钉死（缺口 `GAP-20261001-61`）：✗ 不用 PATH 上的裸 python3；不在盘则跳过（前置不成立）。
	if _, err := os.Stat(gctTestPyExe); err != nil {
		t.Skipf("%s 不在盘 ⇒ 跳过（GAP-20261001-61：钉解释器）", gctTestPyExe)
	}
	holder := exec.Command(gctTestPyExe, tool, "--name", "lockprobe", "--slots", "2", "--wait", "0",
		"--_selftest-hold", "8")
	holder.Env = os.Environ()
	pipe, err := holder.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	holder.Stderr = os.Stderr
	if err := holder.Start(); err != nil {
		t.Fatalf("起持锁子进程：%v", err)
	}
	defer func() { _ = holder.Process.Kill(); _, _ = holder.Process.Wait() }()

	held := make(chan bool, 1)
	go func() {
		sc := bufio.NewScanner(pipe)
		for sc.Scan() {
			if strings.TrimSpace(sc.Text()) == "HELD" {
				held <- true
				return
			}
		}
		held <- false
	}()
	select {
	case ok := <-held:
		if !ok {
			t.Fatal("持锁子进程没报 HELD（前置不成立）")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("等 HELD 超时（前置不成立）")
	}

	rc, _, errb := gctRun(gctInv("lockprobe"), root)
	if rc != 2 {
		t.Fatalf("同名件级锁被占 ⇒ 退 2（不给结论），得到 %d · stderr=%s", rc, errb)
	}
	// 不 exec ⇒ 不落台账（锁竞争在 exec 之前拦下）。
	if _, err := os.Stat(filepath.Join(root, "scripts", "gates", "judge-run-ledger.jsonl")); !os.IsNotExist(err) {
		t.Errorf("锁竞争不 exec ⇒ 不该落台账：%v", err)
	}

	// 成对负控：**不同名**不互阻 ⇒ 照跑（退 0）。
	rc2, _, errb2 := gctRun(gctInv("other"), root)
	if rc2 != 0 {
		t.Fatalf("不同名不互阻 ⇒ 照跑退 0，得到 %d · stderr=%s", rc2, errb2)
	}
}

// ⑥ 进程内 Go 表完整性：项数 == 108，且**每项 key 都能在现读名录里找到**（名字 ⇒ 件 不断裂）。
// ★ 棘轮口径：本数 == 全包 *GateMain 入口数（现读两者同为 108）。门111 check-fleet-loadable 的 Go 真搬件（check_fleet_loadable.go）入 gate_inproc_table.go 后 107→108 ⇒ 期望随动（✗ 非凑绿）。
func TestGateCheckGoEntryTableComplete(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	byName, dupes, err := gateCheckRoster(root)
	if err != nil {
		t.Skipf("现读名录读不到（%v）", err)
	}
	if len(dupes) != 0 {
		t.Fatalf("真实名录有同名多件 ⇒ 前置不成立：%v", dupes)
	}
	if got := len(gateCheckGoEntry); got != 108 {
		t.Fatalf("进程内 Go 表项数要 == 108，得到 %d", got)
	}
	missing := []string{}
	for name := range gateCheckGoEntry {
		if _, ok := byName[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		t.Fatalf("表里 %d 项在名录里找不到（名字 ⇒ 件 断裂）：%q", len(missing), missing)
	}
}

// ⑦ 进程内 Go 表 key 形态：key == 名录件的**基名去扩展名**（与名录扫描面同形 · 现读交叉 · ✗ 不抄字面量）。
func TestGateCheckGoEntryKeysMirrorRoster(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	byName, _, err := gateCheckRoster(root)
	if err != nil {
		t.Skipf("现读名录读不到（%v）", err)
	}
	for name := range gateCheckGoEntry {
		full, ok := byName[name]
		if !ok {
			t.Errorf("表 key %q 在名录里找不到", name)
			continue
		}
		base := filepath.Base(full)
		got := strings.TrimSuffix(base, filepath.Ext(base))
		if got != name {
			t.Errorf("表 key %q 与名录件基名去扩展名 %q 不同形（件 %s）", name, got, full)
		}
	}
}

// ⑧ 开关**在进程内分派之前**：off ⇒ 已在 Go 表里的判据名也退 8，且**不落台账**
// （进程内分派点的台账 append 无条件 ⇒ 无行即证明没走到分派）。
func TestGateCheckForwardOffPrecedesInproc(t *testing.T) {
	name := ""
	for k := range gateCheckGoEntry {
		name = k
		break
	}
	if name == "" {
		t.Fatal("进程内 Go 表为空 ⇒ 前置不成立")
	}
	// 成对前提：该名**确实**在表里 —— 不设开关时它就会走进程内分派。
	if _, hit := gateCheckGoEntry[name]; !hit {
		t.Fatalf("前置不成立：%q 不在 Go 表里", name)
	}
	root := t.TempDir()
	gctWrite(t, root, "scripts/"+name+".py", "print('unused')\n")
	led := filepath.Join(root, "scripts", "gates", "judge-run-ledger.jsonl")

	t.Setenv("ZERG_GATE_FORWARD", "off")
	rc, _, errb := gctRun(gctInv(name), root)
	if rc != exitBlocked {
		t.Fatalf("ZERG_GATE_FORWARD=off ⇒ 退 %d（不给结论），得到 %d · stderr=%s", exitBlocked, rc, errb)
	}
	if !strings.Contains(errb, "转发已关") || !strings.Contains(errb, "detail=judge_broken") {
		t.Errorf("要明说「转发已关」并带 detail=judge_broken：%q", errb)
	}
	if _, err := os.Stat(led); !os.IsNotExist(err) {
		t.Errorf("off ⇒ 在进程内分派（其 append 无条件）**之前**就返回 ⇒ 不该落台账：%v", err)
	}
}

// ⑨ gateCheckRunInproc：cwd **无残留**；两枚 env 也跟着恢复（含**原本不存在**的档 ⇒ 调后回到「不存在」）。
func TestGateCheckRunInprocRestoresCwdAndEnv(t *testing.T) {
	root := t.TempDir()
	rootReal := root
	if r, err := filepath.EvalSymlinks(root); err == nil {
		rootReal = r
	}
	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// 档一：两枚 env **原本不存在**。
	oldRoot, hadRoot := os.LookupEnv("ZERG_REPO_ROOT")
	oldRepo, hadRepo := os.LookupEnv("ZERG_REPO")
	_ = os.Unsetenv("ZERG_REPO_ROOT")
	_ = os.Unsetenv("ZERG_REPO")
	defer func() {
		if hadRoot {
			_ = os.Setenv("ZERG_REPO_ROOT", oldRoot)
		} else {
			_ = os.Unsetenv("ZERG_REPO_ROOT")
		}
		if hadRepo {
			_ = os.Setenv("ZERG_REPO", oldRepo)
		} else {
			_ = os.Unsetenv("ZERG_REPO")
		}
	}()

	var inWd, inRoot, inRepo string
	fn := func(stdout, stderr io.Writer, argv []string) int {
		inWd, _ = os.Getwd()
		inRoot, _ = os.LookupEnv("ZERG_REPO_ROOT")
		inRepo, _ = os.LookupEnv("ZERG_REPO")
		return 0
	}

	if rc := gateCheckRunInproc(fn, root, io.Discard, io.Discard, nil); rc != 0 {
		t.Fatalf("哑件退 0 应原样转出，得到 %d", rc)
	}
	if inWd != rootReal {
		t.Errorf("域内 cwd 应切到 root（%s），得到 %s", rootReal, inWd)
	}
	if inRoot != root || inRepo != root {
		t.Errorf("域内 ZERG_REPO_ROOT/ZERG_REPO 都应设为 root（%s）：%q / %q", root, inRoot, inRepo)
	}
	if after, _ := os.Getwd(); after != before {
		t.Errorf("cwd 有残留：调前 %s · 调后 %s", before, after)
	}
	if v, ok := os.LookupEnv("ZERG_REPO_ROOT"); ok {
		t.Errorf("原本不存在 ⇒ 调后应回到「不存在」，却 = %q", v)
	}
	if v, ok := os.LookupEnv("ZERG_REPO"); ok {
		t.Errorf("原本不存在 ⇒ 调后应回到「不存在」，却 = %q", v)
	}

	// 档二：两枚 env **原本有值** ⇒ 调后回到原值。
	_ = os.Setenv("ZERG_REPO_ROOT", "/sentinel/root")
	_ = os.Setenv("ZERG_REPO", "/sentinel/repo")
	if rc := gateCheckRunInproc(fn, root, io.Discard, io.Discard, nil); rc != 0 {
		t.Fatalf("哑件退 0 应原样转出（档二），得到 %d", rc)
	}
	if v, _ := os.LookupEnv("ZERG_REPO_ROOT"); v != "/sentinel/root" {
		t.Errorf("原本有值 ⇒ 调后应回原值 /sentinel/root，却 = %q", v)
	}
	if v, _ := os.LookupEnv("ZERG_REPO"); v != "/sentinel/repo" {
		t.Errorf("原本有值 ⇒ 调后应回原值 /sentinel/repo，却 = %q", v)
	}
	if after, _ := os.Getwd(); after != before {
		t.Errorf("cwd 有残留（档二）：调前 %s · 调后 %s", before, after)
	}
}

// ⑩ 未命中 ⇒ 原路：不在 Go 表里的判据名**不走进程内**（真起解释器 exec 件）；连名录都不在 ⇒ 走「不在册」退 2。
func TestGateCheckNameMissFallsThroughToExec(t *testing.T) {
	const name = "gct-not-in-go-table" // 自造名：现读断言它不在进程内 Go 表里。
	if _, hit := gateCheckGoEntry[name]; hit {
		t.Fatalf("前置不成立：%q 竟在 Go 表里", name)
	}
	root := t.TempDir()
	gctWrite(t, root, "scripts/"+name+".py", "print('gct-exec-path')\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")

	rc, out, errb := gctRun(gctInv(name), root)
	if rc != 0 {
		t.Fatalf("不在 Go 表 ⇒ 走 exec 原路（合成 py 退 0 ⇒ 壳退 0），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "gct-exec-path") {
		t.Errorf("未命中表 ⇒ 应 exec 真件（透传其 stdout），却 %q", out)
	}
	b, err := os.ReadFile(filepath.Join(root, "scripts", "gates", "judge-run-ledger.jsonl"))
	if err != nil {
		t.Fatalf("exec 支应落台账：%v", err)
	}
	if !strings.Contains(string(b), "\"detail\":\"ok\"") {
		t.Errorf("exec 成功支台账该落 detail=ok：%q", string(b))
	}

	// 反面对照：名字**连名录都不在** ⇒ 走「不在册」那条错路（退 2 · 不 exec）。
	rc2, _, errb2 := gctRun(gctInv("gct-nowhere-at-all"), root)
	if rc2 != exitUsage {
		t.Fatalf("不在册 ⇒ 退 %d（保守拒），得到 %d · stderr=%s", exitUsage, rc2, errb2)
	}
	if !strings.Contains(errb2, "不在册") {
		t.Errorf("要逐字说「不在册」：%q", errb2)
	}
}

// ⑪ ★ GAP-20261005-20 · **重编类护栏**（属性判）：落在构建/发布入口目录 或 非注释行写 repo bin/ 的件
//
//	⇒ 与写工具同档，缺 `--yes` 拒执（退 2 · **不 exec**）；给 `--yes` ⇒ 照跑（成对负控，证护栏是唯一拦点）。
func TestGateCheckRebuildClassGuard(t *testing.T) {
	root := t.TempDir()
	// 重编类夹具：落在构建/发布入口目录，且**非注释行**含「写 repo bin/」写位形态（在 `if false` 里 ⇒ 不真跑）。
	gctWrite(t, root, "scripts/build/build-all.sh",
		"#!/usr/bin/env bash\n"+
			"echo gct-rebuild-fixture\n"+
			"if false; then\n"+
			"  go build -o \"${REPO_ROOT}/bin/gct-artifact\" ./cmd/x\n"+
			"fi\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")

	// 成对负控（护栏的另一半）：给 `--yes` ⇒ 重编类件**照跑**（证它不是「永远拒执」）。
	invYes := gctInv("build-all", "--yes")
	invYes.yes = true // 进程内测试里 `--yes` 由全局解析置位（✗ 不靠 orig 反解）
	rcYes, outYes, errbYes := gctRun(invYes, root)
	if rcYes != 0 || !strings.Contains(outYes, "gct-rebuild-fixture") {
		t.Fatalf("给 --yes ⇒ 重编类件照跑（成对负控）：rc=%d out=%q stderr=%q", rcYes, outYes, errbYes)
	}

	// 改后语义（有护栏）：缺 `--yes` ⇒ 拒执（退 2）· **不 exec**（夹具的 echo 不出现）。
	rc, out, errb := gctRun(gctInv("build-all"), root)
	if rc != exitUsage {
		t.Fatalf("重编类缺 --yes ⇒ 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
	if strings.Contains(out, "gct-rebuild-fixture") {
		t.Errorf("缺 --yes ⇒ **不 exec**，夹具不该跑：out=%q", out)
	}
	if !strings.Contains(errb, "重编类") || !strings.Contains(errb, "--yes") {
		t.Errorf("要明说「重编类」并指 --yes：%q", errb)
	}
}

// ⑫ ★ 属性判**不是靠名**：件既不在构建目录、名字也不含 build/all —— 仅凭「非注释行写 repo bin/」就该判危险档。
func TestGateCheckRebuildClassByContentNotName(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/gates/whatever.sh",
		"#!/usr/bin/env bash\n"+
			"echo gct-content-fixture\n"+
			"if false; then printf x > bin/zerg-core; fi\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")
	rc, out, errb := gctRun(gctInv("whatever"), root)
	if rc != exitUsage {
		t.Fatalf("非注释行写 bin/ ⇒ 判重编类缺 --yes 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
	if strings.Contains(out, "gct-content-fixture") {
		t.Errorf("缺 --yes ⇒ 不 exec：out=%q", out)
	}
}

// ⑬ ★ 不回归：**只读判据**（读 bin/、注释里提构建目录）⇒ 不受影响，照跑退 0（✗ 不把日常只读判据全变危险档）。
func TestGateCheckReadOnlyJudgeUnaffected(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/readonly-judge.py",
		"import sys\n"+
			"# 只是**读**：下面这两处都不是写位形态\n"+
			"#   open('bin/zerg','r')  · grep scripts/build/build-all.sh\n"+
			"print('readonly-ok')\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")
	rc, out, errb := gctRun(gctInv("readonly-judge"), root) // ✗ 不给 --yes
	if rc != 0 {
		t.Fatalf("只读判据 ⇒ 照跑退 0（不受护栏影响），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "readonly-ok") {
		t.Errorf("只读判据 stdout 应透传：%q", out)
	}
}

// ⑭ ★ 目录角色属性：落在构建/发布入口目录下的件（即便正文没有写位形态）⇒ 仍判重编类。
func TestGateCheckRebuildDirRole(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/build/plain-helper.sh", "#!/usr/bin/env bash\necho plain\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")
	rc, _, errb := gctRun(gctInv("plain-helper"), root)
	if rc != exitUsage {
		t.Fatalf("构建目录角色 ⇒ 判重编类缺 --yes 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
}

// ⑮ ★ GAP-20261005-20 另一半 · **整套门类护栏**（属性判）：会**真跑整套门/步**的件（遍历名册逐个真跑）
//
//	⇒ 与写工具同档，缺 `--yes` 拒执（退 2 · **不 exec**）；给 `--yes` ⇒ 照跑（成对负控，证护栏是唯一拦点）。
func TestGateCheckSuiteClassGuard(t *testing.T) {
	root := t.TempDir()
	// 整套门夹具：现读名册目录（`${GATEDIR}`）并**命令位**起解释器跑一个变量执行体（在 `if false` 里 ⇒ 不真跑）。
	gctWrite(t, root, "scripts/gates/suite-runner.sh",
		"#!/usr/bin/env bash\n"+
			"echo gct-suite-fixture\n"+
			"GATEDIR=\"${REPO_ROOT}/scripts/gates\"\n"+
			"if false; then\n"+
			"  bash \"${GATEDIR}/some-gate.py\"\n"+
			"fi\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")

	// 成对负控：给 `--yes` ⇒ 整套门类件**照跑**（证它不是「永远拒执」）。
	invYes := gctInv("suite-runner", "--yes")
	invYes.yes = true
	rcYes, outYes, errbYes := gctRun(invYes, root)
	if rcYes != 0 || !strings.Contains(outYes, "gct-suite-fixture") {
		t.Fatalf("给 --yes ⇒ 整套门类件照跑（成对负控）：rc=%d out=%q stderr=%q", rcYes, outYes, errbYes)
	}

	// 改后语义（有护栏）：缺 `--yes` ⇒ 拒执（退 2）· **不 exec**（夹具的 echo 不出现）。
	rc, out, errb := gctRun(gctInv("suite-runner"), root)
	if rc != exitUsage {
		t.Fatalf("整套门类缺 --yes ⇒ 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
	if strings.Contains(out, "gct-suite-fixture") {
		t.Errorf("缺 --yes ⇒ **不 exec**，夹具不该跑：out=%q", out)
	}
	if !strings.Contains(errb, "整套门") || !strings.Contains(errb, "--yes") {
		t.Errorf("要明说「整套门类」并指 --yes：%q", errb)
	}
}

// ⑯ ★ 属性判**不是靠名**：件名字既不含 all 也不含 real-gates —— 仅凭「现读名册 + 命令位跑变量执行体」就该判危险档。
func TestGateCheckSuiteClassByContentNotName(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/gates/zzz-whatever.sh",
		"#!/usr/bin/env bash\n"+
			"echo gct-suite-content\n"+
			"GATEDIR=\"${REPO_ROOT}/scripts/gates\"\n"+
			"if false; then exec \"${GATEDIR}/g.py\"; fi\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")
	rc, out, errb := gctRun(gctInv("zzz-whatever"), root)
	if rc != exitUsage {
		t.Fatalf("名册执行属性 ⇒ 判整套门类缺 --yes 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
	if strings.Contains(out, "gct-suite-content") {
		t.Errorf("缺 --yes ⇒ 不 exec：out=%q", out)
	}
}

// ⑰ ★ 步命令执行属性（shell 起 `-c`）：整套门**执行器**本体 ⇒ 同档拒执。
func TestGateCheckSuiteStepCmdGuard(t *testing.T) {
	root := t.TempDir()
	gctWrite(t, root, "scripts/gates/exec-body.sh",
		"#!/usr/bin/env bash\n"+
			"echo gct-stepcmd-fixture\n"+
			"if false; then bash -c \"${cmd}\"; fi\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")
	rc, out, errb := gctRun(gctInv("exec-body"), root)
	if rc != exitUsage {
		t.Fatalf("步命令执行属性 ⇒ 判整套门类缺 --yes 退 %d，得到 %d · stderr=%s", exitUsage, rc, errb)
	}
	if strings.Contains(out, "gct-stepcmd-fixture") {
		t.Errorf("缺 --yes ⇒ 不 exec：out=%q", out)
	}
}

// ⑱ ★ 不回归：「只读读步骤表」的判据（把步骤表当文本读 · 带只读档起名册件）⇒ 不受影响，照跑退 0。
func TestGateCheckStepTableReadersUnaffected(t *testing.T) {
	root := t.TempDir()
	// 读步骤表（合成步表 + open 解析）—— 从不起命令位解释器 ⇒ 两条属性都不命中。
	gctWrite(t, root, "scripts/gates/reader-judge.py",
		"# 只读：把步表当文本读（不 exec）\n"+
			"txt = open('scripts/gates/step-table.sh').read()\n"+
			"print('reader-ok')\n")
	gctWrite(t, root, "scripts/gates/step-table.sh",
		"#!/usr/bin/env bash\n# add_step go \"x\" tri \"${REPO_ROOT}\" \"echo x\"\n")
	// 名册件**带只读档**（`--self-test`）⇒ 同行有只读档 ⇒ 不判真跑。
	gctWrite(t, root, "scripts/gates/selfonly.sh",
		"#!/usr/bin/env bash\n"+
			"GATEDIR=\"${REPO_ROOT}/scripts/gates\"\n"+
			"if false; then bash \"${GATEDIR}/g.py\" --self-test; fi\n"+
			"echo selfonly-ok\n")
	t.Setenv("ZERG_GATE_FORWARD", "on")
	for _, name := range []string{"reader-judge", "selfonly"} {
		rc, _, errb := gctRun(gctInv(name), root)
		if rc != 0 {
			t.Fatalf("只读判据 %q ⇒ 照跑退 0（不受护栏影响），得到 %d · stderr=%s", name, rc, errb)
		}
	}
}
