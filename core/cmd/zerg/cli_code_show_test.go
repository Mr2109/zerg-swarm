// cli_code_show_test.go —— `zerg code show <件:行>` 的**真二进制**判据（任务单 `波7` 序71 · `Q-014`）。
//
// 为什么在 `main_test` 这一层（§九 M17「三层测试落点」的第三层）：退码是 `os.Exit` 之后的终值，
// 而且本命令的输入面是**文件树 + 行号** —— 合成一个临时仓根比动真仓干净（与 `cli_code_find_test.go` 同法）。
//
// 判据（任务单逐字）：`code show <件:行>` ⇒ rc=0 且出**该行邻域**；负控：**行号越界 ⇒ rc=2**。
package main_test

import (
	"encoding/json"
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

	// ── 负控③：缺参 / 没冒号 / 行号非正整数 / `--ctx` 坏 ⇒ 各退 2。
	for _, argv := range [][]string{
		{"code", "show"},
		{"code", "show", "aaa/ten.txt"},
		{"code", "show", "aaa/ten.txt:0"},
		{"code", "show", "aaa/ten.txt:abc"},
		{"code", "show", "aaa/ten.txt:5", "--ctx", "-1"},
	} {
		rc, _, errb = execCase(t, bin, root, argv...)
		if rc != 2 {
			t.Errorf("%v ⇒ 退 2，得到 %d · stderr=%s", argv, rc, errb)
		}
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

	// ── 负控⑥：`--json` 不给字段 ⇒ 2（§4.1 `K2` 甲档 · 退码取自退码表 · stdout 0 字节）。
	rc, out, errb = execCase(t, bin, root, "code", "show", "aaa/ten.txt:5", "--json")
	if rc != 2 {
		t.Errorf("--json 不给字段 ⇒ 退 2，得到 %d · stderr=%s", rc, errb)
	}
	if out != "" {
		t.Errorf("K2 这一格 stdout 必须是 0 字节，得到 %q", out)
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
