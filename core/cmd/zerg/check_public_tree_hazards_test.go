// check_public_tree_hazards_test.go —— `check_public_tree_hazards.go` 的**成对定向单测**。
//
// 覆盖面：
//
//	· 纯函数 tree_class 五格（发布档前置判据）
//	· 分类面 PATH_RES/FORM_RES 十五格（与 py `cases_path`/`cases_form` 同表）
//	· 真自检面 `cphSelftest`（含仓外临时 git 仓 · 断言「全过（30 格）」）
//	· **负控有牙**：真扫描面在含危险物的树**真判红**、在干净树**真判绿**（一侧哑化即 FAIL）
//	· 主流程三退码（0/1/2）与 JSON 拒收面
package main

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestCheckPublicTreeHazardsTreeClass(t *testing.T) {
	for i, c := range cphCasesTree() {
		got, why := cphTreeClass(c.cnt, c.subj, c.body)
		if got != c.want {
			t.Fatalf("case %d (%s): got=%v want=%v (why=%s)", i, c.why, got, c.want, why)
		}
	}
}

func TestCheckPublicTreeHazardsClassifierTables(t *testing.T) {
	for i, c := range cphCasesPath() {
		got := false
		for _, rx := range cphDangerPath {
			if rx.rx.MatchString(c.text) {
				got = true
				break
			}
		}
		if got != c.want {
			t.Fatalf("path case %d (%s): got=%v want=%v", i, c.why, got, c.want)
		}
	}
	for i, c := range cphCasesForm() {
		got := false
		for _, rx := range cphTokenForm {
			if rx.rx.MatchString(c.text) {
				got = true
				break
			}
		}
		if got != c.want {
			t.Fatalf("form case %d (%s): got=%v want=%v", i, c.why, got, c.want)
		}
	}
}

// cphTestRepo —— 建一枚仓外临时 git 仓（hazard=true 放危险路径 + 令牌形态）。
func cphTestRepo(t *testing.T, hazard bool) string {
	t.Helper()
	repo := cphPyJoin(t.TempDir(), "repo")
	env := cphGitEnv()
	if err := os.MkdirAll(cphPyJoin(repo, ".zerg", "logs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if hazard {
		if err := os.WriteFile(cphPyJoin(repo, ".zerg/logs/events.jsonl"),
			[]byte("note: "+cphT+"shared0123456789\n"), 0o644); err != nil {
			t.Fatalf("write hazard: %v", err)
		}
	} else {
		if err := os.WriteFile(cphPyJoin(repo, "docs.md"), []byte("clean\n"), 0o644); err != nil {
			t.Fatalf("write clean: %v", err)
		}
	}
	for _, args := range [][]string{
		{"git", "init", "-q", "-b", "main", repo},
		{"git", "-C", repo, "add", "-A"},
		{"git", "-C", repo, "commit", "-q", "-m", "fixture"},
	} {
		if err := cphGitFixture(args, env); err != nil {
			t.Fatalf("fixture git %v: %v", args, err)
		}
	}
	return repo
}

// cphTestProductRepo —— 建一枚「发布档产物树」夹具（单提交 + 快照提交信息）。
func cphTestProductRepo(t *testing.T, hazard bool) string {
	t.Helper()
	repo := cphPyJoin(t.TempDir(), "repo")
	env := cphGitEnv()
	if err := os.MkdirAll(cphPyJoin(repo, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if hazard {
		if err := os.MkdirAll(cphPyJoin(repo, ".zerg", "logs"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(cphPyJoin(repo, ".zerg/logs/events.jsonl"), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write hazard: %v", err)
		}
	}
	if err := os.WriteFile(cphPyJoin(repo, "docs/guide.md"), []byte("# g\n"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	for _, args := range [][]string{
		{"git", "init", "-q", "-b", "main", repo},
		{"git", "-C", repo, "add", "-A"},
		{"git", "-C", repo, "commit", "-q", "-m", cphSnapshotSubject,
			"-m", "源码快照，非开发历史。" + cphSnapshotBodyMark + " abc"},
	} {
		if err := cphGitFixture(args, env); err != nil {
			t.Fatalf("fixture git %v: %v", args, err)
		}
	}
	return repo
}

// TestCheckPublicTreeHazardsSelftestPasses —— 真自检面（含仓外 git 夹具）必须「全过（30 格）」。
// 谁把 scan_paths/scan_forms 哑化、或丢了 `-c core.quotepath=false`，本用例立刻 FAIL 带行号。
func TestCheckPublicTreeHazardsSelftestPasses(t *testing.T) {
	var buf bytes.Buffer
	if !cphSelftest(&buf) {
		t.Fatalf("cphSelftest 未过：\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "自测: 全过（41 格）") {
		t.Fatalf("自检格数不符（want 41）：\n%s", buf.String())
	}
}

// TestCheckPublicTreeHazardsScanDetectorTeeth —— **负控有牙**：真扫描面两侧都验。
// 含危险物 ⇒ 必判红且逐字点名；干净树 ⇒ 必判绿（另一侧哑化即失牙）。
func TestCheckPublicTreeHazardsScanDetectorTeeth(t *testing.T) {
	repo := cphTestRepo(t, true)
	ph, _ := cphScanPaths(repo, false)
	if len(ph) == 0 {
		t.Fatalf("负控失牙：cphScanPaths 未抓到危险路径（疑被哑化）")
	}
	named := false
	for _, h := range ph {
		if h[0] == ".zerg/logs/events.jsonl" {
			named = true
		}
	}
	if !named {
		t.Fatalf("负控失牙：cphScanPaths 未逐字点名危险件，实测 %v", ph)
	}
	fh, _ := cphScanForms(repo, "HEAD", false)
	if len(fh) == 0 {
		t.Fatalf("负控失牙：cphScanForms 未抓到令牌形态（疑被哑化）")
	}
	clean := cphTestRepo(t, false)
	if got, _ := cphScanPaths(clean, false); len(got) != 0 {
		t.Fatalf("干净树不应命中路径，实测 %v", got)
	}
	if got, _ := cphScanForms(clean, "HEAD", false); len(got) != 0 {
		t.Fatalf("干净树不应命中形态，实测 %v", got)
	}
}

// TestCheckPublicTreeHazardsMainFixtureCodes —— 主流程发布档三面（0 干净 / 1 命中 / 2 非产物树）。
func TestCheckPublicTreeHazardsMainFixtureCodes(t *testing.T) {
	clean := cphTestProductRepo(t, false)
	var out, errb bytes.Buffer
	if rc := checkPublicTreeHazardsGateMain(&out, &errb, []string{clean, "HEAD"}); rc != 0 {
		t.Fatalf("干净产物树 rc=%d want 0\nstdout=%s\nstderr=%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "结果: 通过——") {
		t.Fatalf("干净产物树缺「通过」正文：\n%s", out.String())
	}
	danger := cphTestProductRepo(t, true)
	out.Reset()
	errb.Reset()
	if rc := checkPublicTreeHazardsGateMain(&out, &errb, []string{danger, "HEAD"}); rc != 1 {
		t.Fatalf("含危险物产物树 rc=%d want 1\nstdout=%s\nstderr=%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "结果: 不通过——危险路径 3 项") {
		t.Fatalf("含危险物产物树缺「不通过」正文：\n%s", out.String())
	}
	// 非 git 目录 ⇒ 拒绝出结论 rc=2（缺语料铁律）
	out.Reset()
	errb.Reset()
	plain := cphPyJoin(t.TempDir(), "no-git")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if rc := checkPublicTreeHazardsGateMain(&out, &errb, []string{plain, "HEAD"}); rc != 2 {
		t.Fatalf("非 git 目录 rc=%d want 2", rc)
	}
	if !strings.Contains(errb.String(), "拒绝出结论（rc=2）") {
		t.Fatalf("非 git 目录缺拒绝正文：\n%s", errb.String())
	}
}

// TestCheckPublicTreeHazardsJSONRefusal —— 本门无机器读面 ⇒ 给 `--json` 必须 rc=2 且不沉默。
func TestCheckPublicTreeHazardsJSONRefusal(t *testing.T) {
	var out, errb bytes.Buffer
	rc := checkPublicTreeHazardsGateMain(&out, &errb, []string{"--json"})
	if rc != 2 {
		t.Fatalf("--json rc=%d want 2", rc)
	}
	if out.Len() != 0 {
		t.Fatalf("--json 不应有 stdout，实测 %q", out.String())
	}
	if !strings.Contains(errb.String(), "本门无机器读面") ||
		!strings.Contains(errb.String(), "字段表：无（本门只出人读面）") ||
		!strings.Contains(errb.String(), "机器面缺 ≠ 零命中") {
		t.Fatalf("--json 拒收正文不符：\n%s", errb.String())
	}
}

// TestCheckPublicTreeHazardsRegistryLedger —— 登记 / 豁免面（GAP-20260926-155）成对负控。
// 有牙：登记表真命中 ⇒ 拆进在册；未登记不豁免；默认空表 ⇒ 0 豁免；**发布档绝不因表放行**。
func TestCheckPublicTreeHazardsRegistryLedger(t *testing.T) {
	// ① 分类面（镜像 py cases_reg：登记命中 / 未登记 / 锚定负控）
	regFx := cphRegFixture()
	for i, c := range cphCasesReg() {
		gotWhy, gotHit := cphRegistryHit(c.path, regFx)
		if gotHit != c.wantHit {
			t.Fatalf("reg case %d (%s): hit got=%v want=%v", i, c.path, gotHit, c.wantHit)
		}
		if c.wantWhy != nil && gotWhy != *c.wantWhy {
			t.Fatalf("reg case %d (%s): why got=%q want=%q", i, c.path, gotWhy, *c.wantWhy)
		}
	}
	// ② 空表不变式：默认空登记表 ⇒ 一个都不豁免（与旧行为逐字同形）
	emptyInreg, emptyReal := cphPartitionRegistered(
		[][2]string{{"a/b.jsonl", "日志"}, {"x.log", "日志"}}, []cphRule{})
	if len(emptyInreg) != 0 || len(emptyReal) != 2 {
		t.Fatalf("空登记表必须 0 豁免：inreg=%v real=%v", emptyInreg, emptyReal)
	}
	// ③ 真扫描输出上，本件真表（默认空）⇒ 0 豁免
	repo := cphTestRepo(t, true)
	ph, _ := cphScanPaths(repo, false)
	inreg, real := cphPartitionRegistered(ph, cphExcludeRegistry)
	if len(inreg) != 0 || len(real) != len(ph) {
		t.Fatalf("默认空登记表必须 0 豁免：inreg=%d real=%d ph=%d", len(inreg), len(real), len(ph))
	}
	// ③b **有牙正控**：非空登记表 ⇒ 该件真从判红面分出（哑化 cphPartitionRegistered ⇒ 本格必红）
	regHit := []cphRule{{regexp.MustCompile(`^\.zerg/logs/events\.jsonl$`), "夹具：判为发布面排除"}}
	inreg2, real2 := cphPartitionRegistered(ph, regHit)
	named := 0
	for _, h := range inreg2 {
		if h[0] == ".zerg/logs/events.jsonl" {
			named++
		}
	}
	if named == 0 || len(real2) >= len(ph) {
		t.Fatalf("登记面正控失牙：非空登记表未把命中拆进在册（inreg=%d real=%d ph=%d）",
			len(inreg2), len(real2), len(ph))
	}
	// ④ **发布档绝不因表放行**：临时喂一张能命中危险件的登记表，publish 档仍须 rc=1（不通过）。
	saved := cphExcludeRegistry
	cphExcludeRegistry = []cphRule{{regexp.MustCompile(`^\.zerg/logs/events\.jsonl$`), "测试用（发布面排除）"}}
	defer func() { cphExcludeRegistry = saved }()
	prod := cphTestProductRepo(t, true)
	var out, errb bytes.Buffer
	if rc := checkPublicTreeHazardsGateMain(&out, &errb, []string{prod, "HEAD"}); rc != 1 {
		t.Fatalf("发布档不得因登记表放行：rc=%d want 1\n%s", rc, out.String())
	}
	if !strings.Contains(out.String(), "不通过") {
		t.Fatalf("发布档应判不通过：\n%s", out.String())
	}
}
