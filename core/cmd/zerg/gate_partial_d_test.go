// gate_partial_d_test.go —— 表乙大簇 D 项（D2/D3/D5/D6/D8/D10/D12/D16/D19）定向单测。
// ⚠ 候件（仓外）：拟落路径 `core/cmd/zerg/gate_partial_d_test.go`。只测**纯函数 + 临时目录读写**，
// 不跑任何真门、不碰 `bin/`、不重编。
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// D5：转全量抬头必须把 N（现读默认集步数）印出。
func TestGatePartialFullHeaderPrintsN(t *testing.T) {
	p := &gatePartialPlan{
		Full: true, N: 170, Baseline: "/tmp/zerg-gates-x", BaselineHead: "abc123",
		Blocked: 15, Report: 20, Changed: []string{"a"},
		Reasons: []string{"五真源命中 1 件 ⇒ 转全量"},
	}
	var b strings.Builder
	gatePartialPrintFullHeader(&b, p)
	out := b.String()
	if !strings.Contains(out, "N = 170 步") {
		t.Fatalf("D5：转全量抬头未印 N：\n%s", out)
	}
	if !strings.Contains(out, "五真源命中") {
		t.Fatalf("D16：转全量原因未并列印出：\n%s", out)
	}
}

// D16：多命中 ⇒ Reasons 逐条并列（不合并成一句「全局原因」）。
func TestGatePartialDecideMultiCauseParallel(t *testing.T) {
	full := []string{"s0", "s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9"}
	p := &gatePartialPlan{N: len(full), Undeclared: []string{"docs/x"}, FiveTruths: []string{"scripts/gates/affected-map.json"}}
	gatePartialDecide(p, nil, nil, full)
	if !p.Full || len(p.Reasons) != 2 {
		t.Fatalf("D16：undeclared + 五真源 应全量且两条并列 cause，实得 full=%v reasons=%v", p.Full, p.Reasons)
	}
	if !strings.Contains(p.Reasons[0], "未声明件") || !strings.Contains(p.Reasons[1], "五真源") {
		t.Fatalf("D16：并列次序错：%v", p.Reasons)
	}
}

// D8：五真源命中 ⇒ partial.json 含 triggers.five_truths。
func TestGatePartialJSONTriggers(t *testing.T) {
	dir := t.TempDir()
	p := &gatePartialPlan{
		N: 10, FiveTruths: []string{"scripts/gates/judge-registry.json"},
		Undeclared: []string{"docs/a"}, Changed: []string{"docs/a", "scripts/gates/judge-registry.json"},
		Baseline: "/tmp/zerg-gates-b", BaselineHead: "h", BaselineFP: "f", ResultsMT: "t",
	}
	if err := writePartialJSON(dir, p, 3); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, gatePartialJSONFile))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Triggers struct {
			FiveTruths []string `json:"five_truths"`
		} `json:"triggers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Triggers.FiveTruths) != 1 || doc.Triggers.FiveTruths[0] != "scripts/gates/judge-registry.json" {
		t.Fatalf("D8：triggers.five_truths 缺失/不对：%s", raw)
	}
}

// D6：`git diff --name-status` 解析 —— 重命名**两侧**都入改动集。
func TestGatePartialParseNameStatusRenameBothSides(t *testing.T) {
	got := gatePartialParseNameStatus("M\ta.go\nR100\told.go\tnew.go\nD\tgone.go\nA\tadd.go\n")
	want := []string{"a.go", "add.go", "gone.go", "new.go", "old.go"}
	if len(got) != len(want) {
		t.Fatalf("D6：元素数不对 got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("D6：两面对称性不对 got=%v want=%v", got, want)
		}
	}
}

// D10：freshness-partial.json 往返 + 硬前提放宽（认 results-partial.tsv）。
func TestGateFreshnessPartialRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, gateResultsPartialTSV), []byte("PASS\tx\t0\t1s\t/x.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	on := gatePartialBasedOn{Dir: "/base", Head: "bh", WorktreeFP: "bfp", ResultsMT: "rmt"}
	if err := writeGateFreshnessPartial(dir, "/root", "FP", "HEAD", on); err != nil {
		t.Fatalf("D10：部分趟随件应可写（认 results-partial.tsv），实得 %v", err)
	}
	rec, err := readGateFreshnessPartial(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Fingerprint != "FP" || rec.Head != "HEAD" || rec.BasedOn.Dir != "/base" || rec.BasedOn.WorktreeFP != "bfp" {
		t.Fatalf("D10：往返值不对：%+v", rec)
	}
	empty := t.TempDir()
	if err := writeGateFreshnessPartial(empty, "/root", "FP", "HEAD", on); err == nil {
		t.Fatal("D10：非趟目录必须拒写")
	}
}

// D12：全绿口径 = FAIL==0 && BLOCKED==0 && REPORT==0；last-green 往返；落点常量。
func TestGateRunAllGreenAndLastGreen(t *testing.T) {
	if !gateRunAllGreen(map[string]int{"PASS": 5}) {
		t.Fatal("D12：全 PASS 应判全绿")
	}
	if gateRunAllGreen(map[string]int{"PASS": 5, "REPORT": 1}) {
		t.Fatal("D12：有 REPORT 即非全绿（本机制口径 vs 旧退码口径）")
	}
	if gateRunAllGreen(map[string]int{"PASS": 5, "BLOCKED": 1}) {
		t.Fatal("D12：有 BLOCKED 即非全绿")
	}
	root := t.TempDir()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, gateResultsTSV), []byte("PASS\tx\t0\t1s\t/x.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeGateLastGreen(root, dir, "HEAD", "FP", 1, 0); err != nil {
		t.Fatal(err)
	}
	rec, err := readGateLastGreen(root)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Schema != "gate-last-green/1" || rec.Dir != dir || rec.Head != "HEAD" || rec.Fingerprint != "FP" {
		t.Fatalf("D12：last-green 往返不对：%+v", rec)
	}
	if lastGreenRel != "scripts/gates/seal/.runtime/last-green.json" {
		t.Fatalf("D12：落点常量（治法② git-ignored 子目录）不对：%s", lastGreenRel)
	}
}

// D12：`last-green.json` 必须被 `worktreeFingerprint` 精确排除（固定点治法）——真仓小夹具。
func TestGateFingerprintExcludesLastGreen(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git 不可用/失败（跳过）：%v · %s", err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@example.invalid")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("commit", "-qm", "init")
	fp0, _, err := worktreeFingerprint(root)
	if err != nil {
		t.Skipf("指纹算不出（跳过）：%v", err)
	}
	// 普通未跟踪件 ⇒ 指纹变。
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fp1, _, _ := worktreeFingerprint(root)
	if fp1 == fp0 {
		t.Fatal("普通未跟踪件未使指纹变 —— 夹具失效")
	}
	// last-green.json 落点 ⇒ **被精确排除** ⇒ 指纹不变。
	lp := filepath.Join(root, filepath.FromSlash(lastGreenRel))
	if err := os.MkdirAll(filepath.Dir(lp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lp, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fp2, _, _ := worktreeFingerprint(root)
	if fp2 != fp1 {
		t.Fatal("D12：last-green.json 进了指纹（固定点未治）")
	}
}

// D19：同一基线内取 results-partial.tsv mtime 最新的一趟；跨基线不取。
func TestGatePartialNewestSnapshot(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	base := filepath.Join(tmp, "zerg-gates-base")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	mk := func(name, basedOn string, age time.Duration) string {
		d := filepath.Join(tmp, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, gateResultsPartialTSV), []byte("PASS\tx\t0\t1s\t/x.log\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rec := gateFreshnessPartialRecord{Head: "h", Fingerprint: "f", At: "a", BasedOn: gatePartialBasedOn{Dir: basedOn}}
		b, _ := json.Marshal(rec)
		if err := os.WriteFile(filepath.Join(d, gateFreshnessPartialFile), b, 0o644); err != nil {
			t.Fatal(err)
		}
		mt := time.Now().Add(-age)
		_ = os.Chtimes(filepath.Join(d, gateResultsPartialTSV), mt, mt)
		return d
	}
	oldD := mk("zerg-gates-snap-old", base, 2*time.Hour)
	newD := mk("zerg-gates-snap-new", base, 1*time.Minute)
	_ = mk("zerg-gates-snap-other", "/some/other/base", time.Second) // 跨基线：不许取
	got, err := gatePartialNewestSnapshot(base)
	if err != nil {
		t.Fatal(err)
	}
	if got != newD {
		t.Fatalf("D19：应取同基线最新快照 %s，实得 %s（old=%s）", newD, got, oldD)
	}
}
