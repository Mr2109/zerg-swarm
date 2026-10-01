// check_private_path_forms_test.go —— `check_private_path_forms.go` 的成对负控单测（M2 · 路B续4）。
//
// 口径：**成对负控**（✗ 禁恒绿）—— 同一套判据，正控（夹具全被格认领）⇒ rc=0；
// 负控（注入一条未被任何形态格认领的规则 / 把一处出处句弄成 0 命中 / 抽掉一处门禁侧覆盖）
// ⇒ rc=1；再把负控那一处补回 ⇒ 回绿 rc=0。**只是**去掉判据本身会 FAIL（哑化验证见交付读数）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 合成夹具：与 `scripts/gates/check-private-path-forms.py` 的 self_test 夹具**同形** ──
var ppfRulesBase = []string{
	"# 合成替换规则（单测夹具）",
	"Mr2109\tMr2109\t称谓",
	"<host>\t<host>",
	"regex:<repo>\t<repo>",
	"regex:/Volumes/(AHZ|BXC)\t<volume-path>",
	"~\t~",
	"<controller-host>\t<worker-host>",
	"regex:192\\.168\\.\\d{1,3}\\.\\d{1,3}\t<worker-ip>",
	"regex:\\b10\\.0\\.\\d{1,3}\\.\\d{1,3}\\b\t<cluster-ip>",
	"regex:Copyright \\d{4} The Zerg Swarm Authors\tCopyright 2026 The Zerg Swarm Authors",
	"示例虫茧\t示例虫茧",
	"example-8b-quant\texample-8b-quant",
	"Mr2109\tMr2109\t裸用户名",
	"example-moe-36b\texample-moe-36b",
	"example-35b-v2\texample-35b-v2",
	"example-35b\texample-35b",
	"ExampleAdapter\tExampleAdapter",
	"EXAMPLE_TEMPLATE\tEXAMPLE_TEMPLATE",
	"example-35b-v2-\texample-35b-v2-",
	"example-35b-v2\texample-35b-v2",
	"L1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=\tL1VzZXJzL2Z1enowMS8uY29uZmlnL3plcmcvdG9rZW4=",
	"Zerg-内部文档\tZerg-内部文档",
	"!夹具\tcore/internal/obs/redact/*_test.go",
}

var ppfRedactBase = []string{
	"package redact",
	"func hasHomePath(s string) bool {",
	"\treturn strings.Contains(s, \"/Users/\") || strings.Contains(s, \"/home/\") || strings.Contains(s, \"~/\")",
	"}",
	"// 分流，是为了**不留第二份 reHomeDir 的副本**（两份规则 = 下一次漂移的种子）；它不做 `$` 展开，",
	"var (\treHomeDir = regexp.MustCompile(`~/|/(?:Users|home)/[A-Za-z0-9._-]+`)\n)",
}

func ppfWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ppfBuild(t *testing.T, dir string, rules []string) {
	t.Helper()
	ppfWrite(t, filepath.Join(dir, "publish", "replace-rules.tsv"), strings.Join(rules, "\n")+"\n")
	ppfWrite(t, filepath.Join(dir, "core/internal/obs/redact", "redact.go"), strings.Join(ppfRedactBase, "\n")+"\n")
	stubs := map[string][]string{}
	for _, f := range pvForms {
		if f.GateFile != "" && f.GateProbe != "" {
			stubs[f.GateFile] = append(stubs[f.GateFile], f.GateProbe)
		}
	}
	for gf, gps := range stubs {
		var b strings.Builder
		b.WriteString("# 合成门禁侧件（单测夹具 · 只为 E3 的探针命中）\n")
		for _, gp := range gps {
			b.WriteString("NEEDLE = '" + gp + "'\n")
		}
		ppfWrite(t, filepath.Join(dir, gf), b.String())
	}
}

func ppfRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := privatePathFormsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestPrivatePathFormsGatePort(t *testing.T) {
	// ① 正控：夹具齐 ⇒ rc=0（形态全被格认领 · 存量债逐条打印）
	green := t.TempDir()
	ppfBuild(t, green, ppfRulesBase)
	if rc, out, _ := ppfRun(green); rc != 0 {
		t.Fatalf("正控失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
	} else if !strings.Contains(out, "形态枚举全过（rc=0）") {
		t.Fatalf("正控输出缺结论行：%q", out)
	}

	// ② 负控：注入一条真形态、表里任何格都不认领 ⇒ rc=1 且 E2a 点名该行原文
	bad := t.TempDir()
	ppfBuild(t, bad, append(append([]string{}, ppfRulesBase...), "A72109\tMr2109\t镜像账号（真形态 · 表里无格）"))
	if rc, out, _ := ppfRun(bad); rc != 1 || !strings.Contains(out, "[E2a]") || !strings.Contains(out, "A72109") {
		t.Fatalf("负控①（未认领规则）失败：期望 rc=1 且点名 A72109，实得 rc=%d out=%q", rc, out)
	}

	// ③ 成对回绿：同一夹具把那条规则去掉 ⇒ rc=0（证明 ② 的判据不是恒红）
	regreen := t.TempDir()
	ppfBuild(t, regreen, ppfRulesBase)
	if rc, _, _ := ppfRun(regreen); rc != 0 {
		t.Fatalf("成对回绿失败：期望 rc=0，实得 rc=%d", rc)
	}

	// ④ 负控：一处出处句（F-01 的 redact.go 探针）出现 2 次 ⇒ E1 rc=1
	dup := t.TempDir()
	ppfBuild(t, dup, ppfRulesBase)
	redact := filepath.Join(dup, "core/internal/obs/redact", "redact.go")
	body, _ := os.ReadFile(redact)
	ppfWrite(t, redact, string(body)+"// 第二处 strings.Contains(s, \"/Users/\") 复制\n")
	if rc, out, _ := ppfRun(dup); rc != 1 || !strings.Contains(out, "[E1]") {
		t.Fatalf("负控②（出处句多命中）失败：期望 rc=1 且 [E1]，实得 rc=%d out=%q", rc, out)
	}

	// ⑤ 负控：抽掉一处门禁侧覆盖件（F-06 的 check-public-marks.py 不在盘）⇒ E3 空指 rc=1
	gap := t.TempDir()
	ppfBuild(t, gap, ppfRulesBase)
	if err := os.Remove(filepath.Join(gap, "scripts/gates/check-public-marks.py")); err != nil {
		t.Fatal(err)
	}
	if rc, out, _ := ppfRun(gap); rc != 1 || !strings.Contains(out, "[E3]") {
		t.Fatalf("负控③（门禁侧空指）失败：期望 rc=1 且 [E3]，实得 rc=%d out=%q", rc, out)
	}

	// ⑥ 缺真源件（空目录）⇒ rc=2 不给结论（缺件 ≠ 绿）
	if rc, out, _ := ppfRun(t.TempDir()); rc != 2 || !strings.Contains(out, "不给结论") {
		t.Fatalf("负控④（缺真源）失败：期望 rc=2，实得 rc=%d out=%q", rc, out)
	}

	// ⑦ --list-forms 列形态表（含 F-01 与 F-16）⇒ rc=0
	if rc, out, _ := ppfRun("--list-forms"); rc != 0 || !strings.Contains(out, "F-01") || !strings.Contains(out, "F-16") {
		t.Fatalf("--list-forms 失败：期望 rc=0 且含 F-01/F-16，实得 rc=%d", rc)
	}

	// ⑧ 真仓现读（显式传根 · 主仓根）⇒ rc=0（与 py 原件同读数）
	repo := filepath.Dir(filepath.Dir(filepath.Dir(mustAbs(t, "."))))
	if st, err := os.Stat(filepath.Join(repo, "publish", "replace-rules.tsv")); err == nil && !st.IsDir() {
		if rc, out, _ := ppfRun(repo); rc != 0 {
			t.Fatalf("真仓现读失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
		}
	}
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
