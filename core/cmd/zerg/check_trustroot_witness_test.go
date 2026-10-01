// check_trustroot_witness_test.go —— `check_trustroot_witness.go` 的成对负控测试件（M2 · 信任根见证族）。
//
// ★ 自指安全（与件头标准声明 (b) 同口径）：本件**不写**审计副本落点的连续原串，一律走 `twAuditCopyRel()` 拼串；
// 否则本件（`core/**/*.go` · 正文含落点 ⇒ 落在 W5 扫面内）会让 py 原件的真仓读数由 0 翻 1。
//
// 测试面（成对 · 逐条有牙）：
//
//	① `--self-test` 全绿（rc=0 · 24 格过 0 格不过）
//	② W1 闭集负控（多一键 ⇒ 判红）
//	③ W2 未落地 ⇒ 不给结论（不是绿）
//	④ W4 已落地而两器都没排 ⇒ 判红两条
//	⑤ W5 表元素行 ⇒ 豁免；表外真读 ⇒ 判红（窄豁免）
//	⑥ 真取数面 `twScanReaders` ⇒ 命中真判红并逐字点名（正控）+ 干净件零命中（反控）
//	⑦ W1b 缺件 ⇒ 未取数（rc=2 档），不是绿
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ① --self-test 全绿
func TestCtwMainSelfTestGreen(t *testing.T) {
	var out, errb bytes.Buffer
	rc := checkTrustrootWitnessGateMain(&out, &errb, []string{"--self-test"})
	if rc != 0 {
		t.Fatalf("--self-test rc=%d（要 0）；stdout 尾=%q", rc, lastLines(out.String(), 3))
	}
	if !strings.Contains(out.String(), "自检：24 格过 · 0 格不过") {
		t.Fatalf("自检结论格不符（要 24 格过 0 格不过）")
	}
}

// ② W1 闭集负控：多一键 ⇒ 判红
func TestCtwParsePubExtraKeyReds(t *testing.T) {
	good := []byte(`{"alg":"ed25519","pub":"AAAA","key_id":"8b69ad95939df5cf","at":"2026-09-21T12:43:11+08:00"}`)
	if _, p := twParsePub(good); len(p) != 0 {
		t.Fatalf("正控：好件被判红 %v", p)
	}
	bad := []byte(`{"alg":"ed25519","pub":"AAAA","key_id":"8b69ad95939df5cf","at":"2026-09-21T12:43:11+08:00","extra":1}`)
	if _, p := twParsePub(bad); len(p) == 0 {
		t.Fatalf("负控没牙：多一键（五键）仍判绿")
	}
	// 负控·坏 key_id
	badKid := bytes.Replace(good, []byte("8b69ad95"), []byte("ZZ69ad95"), 1)
	if _, p := twParsePub(badKid); len(p) == 0 {
		t.Fatalf("负控没牙：坏 key_id 仍判绿")
	}
}

// ③ W2 未落地 ⇒ 不给结论（rc=2 档），不是绿
func TestCtwJudgePairUnlandedIsNoConclusion(t *testing.T) {
	run := []byte(`{"alg":"ed25519","pub":"AAAA","key_id":"8b69ad95939df5cf","at":"2026-09-21T12:43:11+08:00"}`)
	reds, nocalls, st := twJudgePair(run, nil, "", nil, "8b69ad95939df5cf")
	if len(reds) != 0 || len(nocalls) == 0 || st != "not_landed" {
		t.Fatalf("缺副本应落不给结论：reds=%d nocalls=%d st=%s", len(reds), len(nocalls), st)
	}
	// 逐字相同 ⇒ 绿
	reds2, nocalls2, st2 := twJudgePair(run, run, "", nil, "8b69ad95939df5cf")
	if len(reds2) != 0 || len(nocalls2) != 0 || st2 != "landed" {
		t.Fatalf("两份逐字相同应绿：reds=%d nocalls=%d st=%s", len(reds2), len(nocalls2), st2)
	}
}

// ④ W4 已落地而两器都没排 ⇒ 判红两条
func TestCtwJudgePublicFaceUnlandedVsMissing(t *testing.T) {
	if p, w := twJudgePublicFace(false, "", "", ""); len(p) != 0 || !strings.Contains(w, "未落地") {
		t.Fatalf("未落地应只交代（不判红）")
	}
	if p, _ := twJudgePublicFace(true, "x\n", "y\n", ""); len(p) != 2 {
		t.Fatalf("已落地而两器都没排应判红 2 条，得 %d", len(p))
	}
	lit := strings.ReplaceAll(twAuditCopyRel(), string(os.PathSeparator), "/")
	if p, _ := twJudgePublicFace(true, lit+"\n", lit+"\n", ""); len(p) != 0 {
		t.Fatalf("两器都显式排除不应判红")
	}
}

// ⑤ W5 窄豁免：表元素行豁免 · 表外真读判红
func TestCtwJudgeReadersNarrowExcuse(t *testing.T) {
	lit := strings.ReplaceAll(twAuditCopyRel(), string(os.PathSeparator), "/")
	exRel := strings.ReplaceAll(twPublishExcludesSh, string(os.PathSeparator), "/")
	syn := "EXCLUDES=(\n  \"a\"\n  \"" + lit + "\"\n)\ncat \"" + lit + "\"\n"
	ss, se, ok := twFindTableSpan(syn, "EXCLUDES=(", ")")
	if !ok {
		t.Fatalf("定不出表区间（夹具自坏）")
	}
	anchors := map[string]twSpan{exRel: {ss, se}}
	lines := twSplitLines(syn)
	in := []twHit{{exRel, 3, lines[2]}}
	out := []twHit{{exRel, 5, lines[4]}}
	if r, e := twJudgeReaders(in, anchors, "", nil, ""); len(r) != 0 || len(e) != 1 {
		t.Fatalf("表元素行应豁免：reds=%d excused=%d", len(r), len(e))
	}
	if r, e := twJudgeReaders(out, anchors, "", nil, ""); len(r) != 1 || len(e) != 0 {
		t.Fatalf("表外真读应判红：reds=%d excused=%d", len(r), len(e))
	}
	// 无锚 ⇒ 一条都不豁免
	if r, e := twJudgeReaders(in, nil, "", nil, ""); len(r) != 1 || len(e) != 0 {
		t.Fatalf("无锚应一条都不豁免：reds=%d excused=%d", len(r), len(e))
	}
}

// ⑥ 真取数面：命中真判红并逐字点名（正控）+ 干净件零命中（反控）
func TestCtwScanReadersFindsLiteral(t *testing.T) {
	root := t.TempDir()
	lit := strings.ReplaceAll(twAuditCopyRel(), string(os.PathSeparator), "/")
	hitRel := "core/cmd/zerg/reader.go"
	cleanRel := "core/cmd/zerg/clean.go"
	if err := os.MkdirAll(filepath.Join(root, "core", "cmd", "zerg"), 0o755); err != nil {
		t.Fatalf("建夹具失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, hitRel), []byte("package main\nvar p = \""+lit+"\"\n"), 0o644); err != nil {
		t.Fatalf("写夹具失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, cleanRel), []byte("package main\n// 干净件\n"), 0o644); err != nil {
		t.Fatalf("写夹具失败：%v", err)
	}
	hits := twScanReaders(root, []string{hitRel, cleanRel}, twAuditCopyRel())
	found := false
	for _, h := range hits {
		if h.path == hitRel && h.line == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("真取数面没判出命中（假安全）：hits=%v", hits)
	}
	if clean := twScanReaders(root, []string{cleanRel}, twAuditCopyRel()); len(clean) != 0 {
		t.Fatalf("干净件应零命中（假阳）：%v", clean)
	}
}

// ⑥b 读不了的件 ⇒ 静默跳过（真口子 · 逐字照搬：既不进可判也不进判不了）
func TestCtwScanReadersUnreadableSkipped(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 下 chmod 000 仍可读 ⇒ 该格无鉴别力")
	}
	root := t.TempDir()
	lit := strings.ReplaceAll(twAuditCopyRel(), string(os.PathSeparator), "/")
	rel := "core/cmd/zerg/blocked.go"
	if err := os.MkdirAll(filepath.Join(root, "core", "cmd", "zerg"), 0o755); err != nil {
		t.Fatalf("建夹具失败：%v", err)
	}
	p := filepath.Join(root, rel)
	if err := os.WriteFile(p, []byte("package main\nvar p = \""+lit+"\"\n"), 0o644); err != nil {
		t.Fatalf("写夹具失败：%v", err)
	}
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatalf("chmod 失败：%v", err)
	}
	defer os.Chmod(p, 0o644)
	if hits := twScanReaders(root, []string{rel}, twAuditCopyRel()); len(hits) != 0 {
		t.Fatalf("读不了的件应静默跳过（真口子现状）：hits=%v", hits)
	}
}

// ⑥c 读失败不再静默（仓外牙 · 回归）：待扫批件不可读 ⇒ 登记 `readFails` ⇒ rc=2，
//
//	BLOCKED 头落**本件实际通道 stderr**（`twScan` 处 `fmt.Fprint(stderr, …)`）+ 逐件点名那一件。
func TestCtwReadFailBlockedStderr(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 下 chmod 000 仍可读 ⇒ 该格无鉴别力")
	}
	repo := t.TempDir()
	if o, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init 夹具失败：%v %s", err, o)
	}
	state := t.TempDir()
	appr := filepath.Join(state, "approvals")
	if err := os.MkdirAll(appr, 0o755); err != nil {
		t.Fatalf("建夹具失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(appr, "operator.pub"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("写运行真源失败：%v", err)
	}
	blocked := filepath.Join(appr, "bad.json")
	if err := os.WriteFile(blocked, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("写批件失败：%v", err)
	}
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatalf("chmod 000 失败：%v", err)
	}
	defer os.Chmod(blocked, 0o644)
	var out, errb bytes.Buffer
	rc := checkTrustrootWitnessGateMain(&out, &errb,
		[]string{"--root", repo, "--state", state, "--no-self-test"})
	if rc != 2 {
		t.Fatalf("读失败应 rc=2（不给结论）：得 rc=%d out=%q err=%q", rc, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "⛔ BLOCKED：以下件读失败，覆盖面不完整 —— 不给结论（原 py 此处 `continue` 静默）\n") {
		t.Fatalf("BLOCKED 头未落 stderr（本件实际通道）：%q", errb.String())
	}
	if !strings.Contains(errb.String(), "   · "+blocked+"（") {
		t.Fatalf("未逐件点名读失败件：%q", errb.String())
	}
	if strings.Contains(out.String(), "BLOCKED") {
		t.Fatalf("BLOCKED 头不该落 stdout（本件通道是 stderr）：%q", out.String())
	}
}

// ⑦ W1b 缺件 ⇒ 未取数（rc=2 档），不是绿
func TestCtwJudgeTicketsEmptyIsNoConclusion(t *testing.T) {
	reds, nocalls, _ := twJudgeTickets(nil, "8b69ad95939df5cf")
	if len(reds) != 0 || len(nocalls) == 0 {
		t.Fatalf("缺件应落未取数：reds=%d nocalls=%d", len(reds), len(nocalls))
	}
	reds2, nocalls2, _ := twJudgeTickets([]twTicket{{"old.json", strings.Repeat("0", 16), "2026-01-01T00:00:00+08:00"}}, "8b69ad95939df5cf")
	if len(reds2) == 0 || len(nocalls2) != 0 {
		t.Fatalf("有件但一枚都不认应判红：reds=%d nocalls=%d", len(reds2), len(nocalls2))
	}
}

func lastLines(s string, n int) string {
	ls := twSplitLines(s)
	if len(ls) > n {
		ls = ls[len(ls)-n:]
	}
	return strings.Join(ls, "\n")
}
