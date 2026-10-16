// cli_code_show_test.go —— `zerg code show <件:行>` 的**真二进制**判据（任务单 `波7` 序71 · `Q-014`）。
//
// 为什么在 `main_test` 这一层（§九 M17「三层测试落点」的第三层）：退码是 `os.Exit` 之后的终值，
// 而且本命令的输入面是**文件树 + 行号** —— 合成一个临时仓根比动真仓干净（与 `cli_code_find_test.go` 同法）。
//
// 判据（任务单逐字）：`code show <件:行>` ⇒ rc=0 且出**该行邻域**；负控：**行号越界 ⇒ rc=2**。
package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCodeShow_NeighborhoodAndUsage —— 正控（出邻域）+ 六格负控（各档退码）成对钉住。
func TestCodeShow_NeighborhoodAndUsage(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n")
	// 一件十行的文本：行号 = 行号，方便逐条断言。
	var sb strings.Builder
	for i := 1; i <= 10; i++ {
		sb.WriteString("LINE_")
		sb.WriteString(strings.Repeat("x", i))
		sb.WriteString("\n")
	}
	mustWrite(t, filepath.Join(root, "aaa", "ten.txt"), sb.String())

	// ── 正控①：第 5 行 ⇒ rc=0 · 邻域 = 第 2…8 行（默认 `±3`）· 该行原文在输出里。
	rc, out, errb := execCase(t, bin, root, "code", "show", "aaa/ten.txt:5")
	if rc != 0 {
		t.Fatalf("在范围内 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "LINE_xxxxx") {
		t.Errorf("被点的那一行没出来：%q", out)
	}
	if !strings.Contains(out, "LINE_xx") || !strings.Contains(out, "LINE_xxxxxxxx") {
		t.Errorf("邻域边界没出来（应含第 2 与第 8 行）：%q", out)
	}
	if !strings.Contains(errb, "共 10 行") || !strings.Contains(errb, "窗口 ±3") {
		t.Errorf("人面抬头没报「共 N 行 / 窗口」：%q", errb)
	}

	// ── 正控②：`--ctx 0` ⇒ 只剩那一行；机器面 `target=yes` 恰好一条且指向它。
	rc, out, errb = execCase(t, bin, root, "code", "show", "aaa/ten.txt:7", "--ctx", "0", "--json", "path,line,text,target")
	if rc != 0 {
		t.Fatalf("--ctx 0 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("机器面不是 JSON：%v · out=%q", err, out)
	}
	if len(env.Items) != 1 {
		t.Fatalf("--ctx 0 应只出 1 条，得到 %d 条：%v", len(env.Items), env.Items)
	}
	if env.Items[0]["line"] != "7" || env.Items[0]["target"] != "yes" || env.Items[0]["path"] != "aaa/ten.txt" {
		t.Errorf("机器面那一行的三格不对：%v", env.Items[0])
	}
	if env.Items[0]["text"] != "LINE_xxxxxxx" {
		t.Errorf("机器面的 text 不是原文：%q", env.Items[0]["text"])
	}

	// ── 负控①（任务单点名）：**行号越界 ⇒ rc=2**（不是 1 —— 1 在本族里是「真没有」）。
	rc, _, errb = execCase(t, bin, root, "code", "show", "aaa/ten.txt:11")
	if rc != 2 {
		t.Errorf("行号越界 ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "行号越界") {
		t.Errorf("行号越界要逐字说清：%q", errb)
	}

	// ── 负控②：件不在 ⇒ 1（「没有」不是错 —— 与 `code find` 零命中同族）。
	rc, _, errb = execCase(t, bin, root, "code", "show", "aaa/nope.txt:1")
	if rc != 1 {
		t.Errorf("件不在 ⇒ 退 1，得到 %d · stderr=%s", rc, errb)
	}

	// ── 负控③：缺参 / 行号非正整数 / `--ctx` 坏 ⇒ 各退 2。
	//   ★ 2026-10-04（缺口 `GAP-20261001-59`）：**「没冒号 ⇒ 2」这一格已废** —— 接受面扩到
	//   「纯件名 = 整件读」（原 `{"code","show","aaa/ten.txt"}` 落此列 ⇒ 现在 rc=0）⇒ 改成正控⑦。
	for _, argv := range [][]string{
		{"code", "show"},
		{"code", "show", "aaa/ten.txt:0"},
		{"code", "show", "aaa/ten.txt:abc"},
		{"code", "show", "aaa/ten.txt:5", "--ctx", "-1"},
	} {
		rc, _, errb = execCase(t, bin, root, argv...)
		if rc != 2 {
			t.Errorf("%v ⇒ 退 2，得到 %d · stderr=%s", argv, rc, errb)
		}
	}

	// ── 正控⑦（缺口 `GAP-20261001-59` · 2026-10-04）：**纯件名 = 整件读**（Hermes 侧 `read_file` 的 CLI 正门）。
	//   接受面前移那一格（上一块移除的 `aaa/ten.txt`）改由这里**正控 + 逐行**钉住新契约。
	rc, out, errb = execCase(t, bin, root, "code", "show", "aaa/ten.txt")
	if rc != 0 {
		t.Errorf("纯件名整件读 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "整件读") {
		t.Errorf("整件读要在 stderr 抬头点明：%q", errb)
	}
	rc, out, errb = execCase(t, bin, root, "code", "show", "aaa/ten.txt", "--json", "path,line,text,target")
	if rc != 0 {
		t.Errorf("整件读 --json ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var envFile struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &envFile); err != nil {
		t.Fatalf("整件读机器面不是 JSON：%v · out=%q", err, out)
	}
	if len(envFile.Items) != 10 {
		t.Errorf("整件读应出 10 行，得到 %d 条", len(envFile.Items))
	} else if envFile.Items[0]["line"] != "1" || envFile.Items[9]["line"] != "10" || envFile.Items[0]["target"] != "no" {
		t.Errorf("整件读逐行三格不对：首=%v 末=%v", envFile.Items[0], envFile.Items[9])
	}

	// ── 负控④：件出仓 / 点的是目录 ⇒ 2（出仓按**路径段**判，不是字符串前缀）。
	for _, argv := range [][]string{
		{"code", "show", "../outside.md:1"},
		{"code", "show", "scripts/gates:1"},
	} {
		rc, _, errb = execCase(t, bin, root, argv...)
		if rc != 2 {
			t.Errorf("%v ⇒ 退 2，得到 %d · stderr=%s", argv, rc, errb)
		}
	}

	// ── 负控⑤：**不是文本**（非 UTF-8）⇒ 8（不给结论 · 不猜它的行号）。
	mustWrite(t, filepath.Join(root, "aaa", "bin.dat"), "\xff\xfe\x00nope\n")
	rc, _, errb = execCase(t, bin, root, "code", "show", "aaa/bin.dat:1")
	if rc != 8 {
		t.Errorf("非 UTF-8 ⇒ 退 8（不给结论），得到 %d · stderr=%s", rc, errb)
	}

	// ── 负控⑥：`--json` 不给字段 ⇒ 2（§4.1 `K2` 甲档 · 退码取自退码表）；★ 缺口 `GAP-20260929-41`
	//   （本批）：`kind=usage` 的失败**不吐包封** ⇒ stdout **零字节**、提示面走 stderr 的字段清单
	//   （旧断言「stdout 必是 error 包封且 items 空」已过期；本断言在旧行为下会红 ⇒ 有牙）。
	rc, out, errb = execCase(t, bin, root, "code", "show", "aaa/ten.txt:5", "--json")
	if rc != 2 {
		t.Errorf("--json 不给字段 ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if out != "" {
		t.Errorf("K2 这一格（用法错）stdout 必须零字节（GAP-20260929-41），得到 %q", out)
	}
}

// TestCodeShow_WindowClampsAtFileEdges —— 窗口在**文件两端**收边：不会越出第 1 行 / 末行。
func TestCodeShow_WindowClampsAtFileEdges(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n")
	mustWrite(t, filepath.Join(root, "aaa", "three.txt"), "one\ntwo\nthree\n")

	rc, out, errb := execCase(t, bin, root, "code", "show", "aaa/three.txt:1", "--ctx", "9",
		"--json", "path,line,text,target")
	if rc != 0 {
		t.Fatalf("文件头那一行 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("机器面不是 JSON：%v · out=%q", err, out)
	}
	if len(env.Items) != 3 {
		t.Fatalf("三行的件 · 窗口再大也只出 3 条，得到 %d：%v", len(env.Items), env.Items)
	}
	if env.Items[0]["line"] != "1" || env.Items[0]["target"] != "yes" {
		t.Errorf("收边后第一条应是第 1 行且是 target：%v", env.Items[0])
	}
	if env.Items[2]["line"] != "3" || env.Items[2]["text"] != "three" {
		t.Errorf("收边后末条应是第 3 行原文：%v", env.Items[2])
	}

	// 末行（第 3 行 = 件真有的最后一行）仍是合法目标（越界只从 4 起）。
	if rc, _, errb = execCase(t, bin, root, "code", "show", "aaa/three.txt:3"); rc != 0 {
		t.Errorf("末行 ⇒ 退 0，得到 %d · stderr=%s", rc, errb)
	}
	if rc, _, _ = execCase(t, bin, root, "code", "show", "aaa/three.txt:4"); rc != 2 {
		t.Errorf("末行 +1 ⇒ 退 2，得到 %d", rc)
	}
}

// execCaseAt —— 起真二进制、**指定 cwd**、且显式剔掉 `ZERG_REPO` / `ZERG_REPO_ROOT`
// （探「根解析认 cwd」那条路；`execCase` 钉的是显式优先那一档，两者成对）。
func execCaseAt(t *testing.T, bin, dir string, argv ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, argv...)
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "ZERG_REPO=") || strings.HasPrefix(kv, "ZERG_REPO_ROOT=") {
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = env
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("")
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	rc := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			t.Fatalf("起不了 bin/zerg：%v", err)
		}
	}
	return rc, out.String(), errb.String()
}

// TestCodeShow_FindSiblingRepoCwd —— 缺口 `GAP-20260928-05` 的真二进制判据：`code show` / `code find`
// 的根解析**认 cwd 的并排仓**（守卫按 cwd 拦手搓形态、把正门当替身 ⇒ 替身必须跟着 cwd 走）。
//
// 成对三态（② 是正控 —— 只有「换根」真生效才可能绿）：
//
//	① cwd = 一棵**没有 `.git`** 的临时目录 ⇒ 不是并排仓 ⇒ 根回落主仓 ⇒ 退 1「件不在」；
//	② 同目录放上 `.git` ⇒ 根切到它 ⇒ 同一条命令退 0 且读到那一行；
//	③ 同一条 cwd 下 `code find … --path <相对目录>` 也退 0（缺口点名的另一条替身同样认 cwd）。
//
// ⇒ 去掉修复（`codeRepoRoot` 落回 `repoRoot()`）时 ② 必塌回 ① 的退 1（这就是这颗牙）。
func TestCodeShow_FindSiblingRepoCwd(t *testing.T) {
	bin := zergBinary(t)
	sib := t.TempDir()
	mustWrite(t, filepath.Join(sib, "aaa", "ten.txt"), "LINE_a\nLINE_b\nneedle_c\nLINE_d\nLINE_e\n")

	// ① 无 `.git` ⇒ 根回落主仓 ⇒ 件不在（退 1）。
	rc, _, errb := execCaseAt(t, bin, sib, "code", "show", "aaa/ten.txt:3")
	if rc != 1 {
		t.Fatalf("cwd 无 `.git` ⇒ 应回落主仓退 1，得到 %d · stderr=%s", rc, errb)
	}

	// ② 放上 `.git`（目录形态即够 —— 本判据只认「最近的含 .git 的祖先」）⇒ 根切到并排仓。
	if err := os.Mkdir(filepath.Join(sib, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	rc, out, errb := execCaseAt(t, bin, sib, "code", "show", "aaa/ten.txt:3")
	if rc != 0 {
		t.Fatalf("并排仓 cwd（有 `.git`）⇒ 根应切到它并退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "needle_c") {
		t.Errorf("没从并排仓读到那一行：%q", out)
	}

	// ③ 缺口点名的另一条替身 `code find` 在同一条 cwd 下也认它。
	rc, out, errb = execCaseAt(t, bin, sib, "code", "find", "needle", "--path", "aaa", "--json", "path")
	if rc != 0 {
		t.Fatalf("`code find` 在同一条 cwd 下应退 0，得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "aaa/ten.txt") {
		t.Errorf("`code find` 没扫到并排仓那件：%q", out)
	}
}
