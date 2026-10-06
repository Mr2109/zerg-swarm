package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// crneWrite 造一枚夹具件（含父目录）。
func crneWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func crneRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkRecordNotEvidenceGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// TestCrneSelfTest 自检档：合成夹具成对负控 ⇒ rc=0 且四条断言全过。
func TestCrneSelfTest(t *testing.T) {
	rc, out, _ := crneRun(t, "--self-test")
	if rc != 0 {
		t.Fatalf("自检 rc=%d，要 0\n%s", rc, out)
	}
	if !strings.Contains(out, "自检结论: 全过（断言 4 条 · 失败 0 条）") {
		t.Fatalf("自检结论行不符：\n%s", out)
	}
}

// TestCrneMissingRoot 取不到仓根 ⇒ rc=2 不给结论。
func TestCrneMissingRoot(t *testing.T) {
	rc, out, _ := crneRun(t, "--root", "/no/such/root-xyz-请勿存在")
	if rc != 2 {
		t.Fatalf("缺根 rc=%d，要 2\n%s", rc, out)
	}
	if !strings.Contains(out, "仓根不在") {
		t.Fatalf("缺根文案不符：\n%s", out)
	}
}

// TestCrneEmptyDomain 空域（扫描面 0 件）⇒ rc=2 BLOCKED，✗ 不许当绿。
func TestCrneEmptyDomain(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	rc, _, errb := crneRun(t, "--root", root, "--scan-dir", "scripts")
	if rc != 2 {
		t.Fatalf("空域 rc=%d，要 2", rc)
	}
	if !strings.Contains(errb, "空域") || !strings.Contains(errb, "rc=2") {
		t.Fatalf("空域文案不符：\n%s", errb)
	}
}

// TestCrneR1Hit R1 成员即存在（成员不在盘上）⇒ rc=1 且 stdout 出现 【R1】。
func TestCrneR1Hit(t *testing.T) {
	root := t.TempDir()
	crneWrite(t, root, "scripts/名册.tsv",
		"件\t说明\nscripts/ghost0.py\tx\nscripts/ghost1.py\tx\nscripts/ghost2.py\tx\nscripts/ghost3.py\tx\nscripts/ghost4.py\tx\nscripts/ghost5.py\tx\n")
	rc, out, _ := crneRun(t, "--root", root, "--scan-dir", "scripts")
	if rc != 1 {
		t.Fatalf("R1 rc=%d，要 1\n%s", rc, out)
	}
	if !strings.Contains(out, "【R1】命中 ") {
		t.Fatalf("无 R1 段落：\n%s", out)
	}
	if strings.Contains(out, "【R1】命中 0 条") {
		t.Fatalf("R1 未命中：\n%s", out)
	}
}

// TestCrneR3Hit R3 记录即已修（断言完成 · 无对盘字段）⇒ rc=1。
func TestCrneR3Hit(t *testing.T) {
	root := t.TempDir()
	crneWrite(t, root, "scripts/台账.tsv",
		"件\t状态\nscripts/ghost0.py\t已修\nscripts/ghost1.py\t已修\nscripts/ghost2.py\t已修\nscripts/ghost3.py\t已修\nscripts/ghost4.py\t已修\nscripts/ghost5.py\t已修\n")
	rc, out, _ := crneRun(t, "--root", root, "--scan-dir", "scripts")
	if rc != 1 {
		t.Fatalf("R3 rc=%d，要 1\n%s", rc, out)
	}
	if strings.Contains(out, "【R3】命中 0 条") {
		t.Fatalf("R3 未命中：\n%s", out)
	}
}

// TestCrneR3NoFalseFire 成对负控：带对盘字段（sha/commit）的「已修」行 ⇒ 不报 R3。
func TestCrneR3NoFalseFire(t *testing.T) {
	root := t.TempDir()
	crneWrite(t, root, "scripts/台账.tsv",
		"件\t状态\t对盘\nscripts/ghost0.py\t已修\tsha256:abcd\nscripts/ghost1.py\t已修\tsha256:abcd\nscripts/ghost2.py\t已修\tsha256:abcd\nscripts/ghost3.py\t已修\tsha256:abcd\nscripts/ghost4.py\t已修\tsha256:abcd\nscripts/ghost5.py\t已修\tsha256:abcd\n")
	_, out, _ := crneRun(t, "--root", root, "--scan-dir", "scripts")
	if !strings.Contains(out, "【R3】命中 0 条") {
		t.Fatalf("带对盘字段仍报 R3（假红）：\n%s", out)
	}
}

// TestCrneClean 对拍豁免 + 真源豁免 ⇒ 0 命中 rc=0。
func TestCrneClean(t *testing.T) {
	root := t.TempDir()
	crneWrite(t, root, "scripts/清单.tsv",
		"# 唯一真源：本表即 scripts/ 的单一来源；漂移由 check-x.py 双向差集管\n件\tsha256\nscripts/real0.py\tabcd\nscripts/real1.py\tabcd\nscripts/real2.py\tabcd\nscripts/real3.py\tabcd\nscripts/real4.py\tabcd\n")
	// 让每条成员都在盘上且带 sha256 逐条对上（此处不写 sha 值 ⇒ pair_exempt 只需在盘上）
	crneWrite(t, root, "scripts/real0.py", "# 0\n")
	crneWrite(t, root, "scripts/real1.py", "# 1\n")
	crneWrite(t, root, "scripts/real2.py", "# 2\n")
	crneWrite(t, root, "scripts/real3.py", "# 3\n")
	crneWrite(t, root, "scripts/real4.py", "# 4\n")
	rc, out, _ := crneRun(t, "--root", root, "--scan-dir", "scripts")
	if rc != 0 {
		t.Fatalf("干净夹具 rc=%d，要 0\n%s", rc, out)
	}
	if !strings.Contains(out, "⇒ 未命中（rc=0）") {
		t.Fatalf("干净夹具文案不符：\n%s", out)
	}
}

// TestCrneJSON --json 档：机器可读 · findings 空集合必须是 [] 而非 null。
func TestCrneJSON(t *testing.T) {
	root := t.TempDir()
	crneWrite(t, root, "scripts/清单.tsv",
		"# 唯一真源：本表即 scripts/ 的单一来源；漂移由 check-x.py 双向差集管\n件\tsha256\nscripts/real0.py\tabcd\nscripts/real1.py\tabcd\nscripts/real2.py\tabcd\nscripts/real3.py\tabcd\nscripts/real4.py\tabcd\n")
	for i := 0; i < 5; i++ {
		crneWrite(t, root, "scripts/real"+string(rune('0'+i))+".py", "#\n")
	}
	rc, out, _ := crneRun(t, "--root", root, "--scan-dir", "scripts", "--json")
	if rc != 0 {
		t.Fatalf("json rc=%d，要 0\n%s", rc, out)
	}
	if !strings.Contains(out, "\"findings\": []") {
		t.Fatalf("空 findings 未显式置空（可能落成 null）：\n%s", out)
	}
	var decoded struct {
		Meta struct {
			Scanned  int      `json:"scanned"`
			Surfaces []string `json:"surfaces"`
		} `json:"meta"`
		Findings []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("--json 输出不是合法 JSON：%v\n%s", err, out)
	}
	if decoded.Meta.Scanned == 0 {
		t.Fatalf("scanned=0 不符（应扫到夹具件）")
	}
}

// TestCrneTablesExplicit --tables 显式点名（给了就不自动发现）：不存在的记录面 ⇒ R? 命中。
func TestCrneTablesExplicit(t *testing.T) {
	root := t.TempDir()
	crneWrite(t, root, "scripts/real0.py", "#\n")
	rc, out, _ := crneRun(t, "--root", root, "--scan-dir", "scripts", "--tables", "scripts/nope.tsv")
	if rc != 1 {
		t.Fatalf("显式表不在盘 rc=%d，要 1\n%s", rc, out)
	}
	// R? 不在 R1/R2/R3 文本循环里打印，但总数应反映为 rc=1
	if !strings.Contains(out, "rc=1") {
		t.Fatalf("未反映命中：\n%s", out)
	}
}
