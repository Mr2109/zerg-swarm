package main

// check_gap_bucket_zero_test.go —— 门133「桶内未闭归零」的两态与红线（合成夹具 · 不读真账）。
//
// 为什么不用真账做断言：真账在 `~/.zerg/state`（每台机器/每条 lane 不同）⇒ 拿它断言 = 把测试绑在
// 本机存量上；真账只做**观测**（见本单回执的两态读数），判据面用合成账。
//
// 覆盖：
//   ① 态A 目标桶 1 条未闭 ⇒ rc=1 且逐条点名 GAP-id（不是只报「有 1 条」）
//   ② 态B 同账只把该条 state 改 `已解` ⇒ rc=0（两态差 = 一行 state）
//   ③ 白名单在效期内放行 ⇒ rc=0；过期 ⇒ rc=1 且点到到期日
//   ④ 白名单空转（id 已闭 / 指向清单外桶）⇒ rc=1
//   ⑤ 取数面拿不到（清单件不在盘 / schema 不符 / 账本空转）⇒ rc=2
//   ⑥ 清单件（拟落仓那件）自己必须能在盘读取并验形制通过

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const bztRegistry = `{
  "schema": "gap-bucket-zero/1",
  "targets": [
    {"axis": "module", "bucket": "core/cmd/zerg", "why": "命令树族落点"},
    {"axis": "unit", "bucket": "core/cmd/zerg/main.go", "why": "按件面再判一次"}
  ],
  "whitelist": []
}`

func bztLedger(open bool) string {
	st := "已解"
	if open {
		st = "仍缺"
	}
	return strings.Join([]string{
		`{"id":"GAP-20261011-03","state":"` + st + `","unit":"core/cmd/zerg/main.go","module":"core/cmd/zerg","impact":"命令面","prio":"P1"}`,
		`{"id":"GAP-20260925-03","state":"仍缺","unit":"无件(命令面)","module":"无件","impact":"命令面","prio":"P2"}`,
		`{"id":"GAP-20260926-92","state":"已解","unit":"deploy/scripts/zerg-deploy-agent.sh","module":"deploy/scripts","impact":"换件面","prio":"P2"}`,
	}, "\n") + "\n"
}

// bztFixture —— 造 (账本, 清单件) 夹具，返回两路径。
func bztFixture(t *testing.T, ledger, registry string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	lp := filepath.Join(dir, "zerg-cli-gaps.jsonl")
	rp := filepath.Join(dir, "bucket-zero-close.json")
	if err := os.WriteFile(lp, []byte(ledger), 0o644); err != nil {
		t.Fatalf("写夹具账失败：%v", err)
	}
	if err := os.WriteFile(rp, []byte(registry), 0o644); err != nil {
		t.Fatalf("写夹具清单失败：%v", err)
	}
	return lp, rp
}

func bztRun(t *testing.T, argv ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkGapBucketZeroGateMain(&out, &errb, argv)
	return rc, out.String()
}

// ①② 两态：同账同清单，只差一行 state。
func TestGapBucketZeroTwoStates(t *testing.T) {
	lpA, rp := bztFixture(t, bztLedger(true), bztRegistry)
	rcA, outA := bztRun(t, "--ledger", lpA, "--registry", rp)
	if rcA != gapBzRCFail {
		t.Fatalf("态A 期望 rc=1，实得 rc=%d\n%s", rcA, outA)
	}
	if !strings.Contains(outA, "· GAP-20261011-03") {
		t.Fatalf("态A 必须逐条点名 GAP-20261011-03\n%s", outA)
	}
	// 两个轴都要点名同一条（件面 / 模块面两轴同判）
	if got := strings.Count(outA, "· GAP-20261011-03"); got != 2 {
		t.Fatalf("两轴各点一次，期望 2 处，实得 %d\n%s", got, outA)
	}

	lpB, _ := bztFixture(t, bztLedger(false), bztRegistry)
	rcB, outB := bztRun(t, "--ledger", lpB, "--registry", rp)
	if rcB != gapBzRCOK {
		t.Fatalf("态B 期望 rc=0，实得 rc=%d\n%s", rcB, outB)
	}
	if strings.Contains(outB, "✗") || strings.Contains(outB, "· GAP-20261011-03") {
		t.Fatalf("态B 不该留红行/点名\n%s", outB)
	}
	if !strings.Contains(outB, "2 个目标桶全清") {
		t.Fatalf("态B 应报「2 个目标桶全清」\n%s", outB)
	}
}

// ③ 白名单：效期内放行 ≠ 已闭；过期即红并点到到期日。
func TestGapBucketZeroWhitelist(t *testing.T) {
	live := `{"schema":"gap-bucket-zero/1","targets":[{"axis":"module","bucket":"core/cmd/zerg","why":"x"}],"whitelist":[{"axis":"module","bucket":"core/cmd/zerg","id":"GAP-20261011-03","reason":"单件拆族跨两版，随拆件批一起销","until":"2026-12-31"}]}`
	lp, rp := bztFixture(t, bztLedger(true), live)
	rc, out := bztRun(t, "--ledger", lp, "--registry", rp)
	if rc != gapBzRCOK {
		t.Fatalf("白名单在效期内 ⇒ rc=0，实得 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "放行：GAP-20261011-03") {
		t.Fatalf("必须把放行的那条写进人面（放行 ≠ 已闭）\n%s", out)
	}
	if !strings.Contains(out, "必须归零 0") {
		t.Fatalf("放行后必须归零应为 0\n%s", out)
	}

	// 过期：把到期日挪到过去
	past := strings.Replace(live, "2026-12-31", "2020-01-01", 1)
	lp2, rp2 := bztFixture(t, bztLedger(true), past)
	rc2, out2 := bztRun(t, "--ledger", lp2, "--registry", rp2)
	if rc2 != gapBzRCFail {
		t.Fatalf("白名单过期 ⇒ rc=1，实得 rc=%d\n%s", rc2, out2)
	}
	if !strings.Contains(out2, "过期") || !strings.Contains(out2, "2020-01-01") {
		t.Fatalf("过期必须点到到期日\n%s", out2)
	}
}

// ④ 白名单空转：id 已闭 / 指向清单外桶 —— 都不许放行空气。
func TestGapBucketZeroWhitelistStale(t *testing.T) {
	reg := `{"schema":"gap-bucket-zero/1","targets":[{"axis":"module","bucket":"core/cmd/zerg","why":"x"}],"whitelist":[{"axis":"module","bucket":"core/cmd/zerg","id":"GAP-20261011-03","reason":"r","until":"2099-12-31"}]}`
	lp, rp := bztFixture(t, bztLedger(false), reg) // 该 id 已 已解
	rc, out := bztRun(t, "--ledger", lp, "--registry", rp)
	if rc != gapBzRCFail || !strings.Contains(out, "空转") {
		t.Fatalf("id 已闭 ⇒ 白名单空转 ⇒ rc=1，实得 rc=%d\n%s", rc, out)
	}

	outside := `{"schema":"gap-bucket-zero/1","targets":[{"axis":"module","bucket":"core/cmd/zerg","why":"x"}],"whitelist":[{"axis":"module","bucket":"无件","id":"*","reason":"无件桶永远有存量","until":"2099-12-31"}]}`
	lp2, rp2 := bztFixture(t, bztLedger(true), outside)
	rc2, out2 := bztRun(t, "--ledger", lp2, "--registry", rp2)
	if rc2 != gapBzRCFail || !strings.Contains(out2, "空转") {
		t.Fatalf("白名单指向清单外桶 ⇒ 空转 rc=1，实得 rc=%d\n%s", rc2, out2)
	}
}

// ⑤ 取数面拿不到 ⇒ rc=2（不绿）。
func TestGapBucketZeroBlocked(t *testing.T) {
	lp, rp := bztFixture(t, bztLedger(true), bztRegistry)

	rc, _ := bztRun(t, "--ledger", lp, "--registry", filepath.Join(t.TempDir(), "nope.json"))
	if rc != gapBzRCBlock {
		t.Fatalf("清单件不在盘 ⇒ rc=2，实得 rc=%d", rc)
	}
	_, rpBad := bztFixture(t, bztLedger(true), `{"schema":"gap-bucket-zero/9","targets":[],"whitelist":[]}`)
	rc2, _ := bztRun(t, "--ledger", lp, "--registry", rpBad)
	if rc2 != gapBzRCBlock {
		t.Fatalf("schema/targets 不符 ⇒ rc=2，实得 rc=%d", rc2)
	}
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(empty, []byte("\n"), 0o644); err != nil {
		t.Fatalf("写空账失败：%v", err)
	}
	rc3, _ := bztRun(t, "--ledger", empty, "--registry", rp)
	if rc3 != gapBzRCBlock {
		t.Fatalf("账本空转 ⇒ rc=2，实得 rc=%d", rc3)
	}
	if rc4, _ := bztRun(t, "--ledger", lp, "--registry", rp, "--bogus"); rc4 != gapBzRCBlock {
		t.Fatalf("未知旗标 ⇒ rc=2，实得 rc=%d", rc4)
	}
}

// ⑥ 拟落仓的清单件自己必须读得动、形制验得过（只验结构，不断言它列了哪几桶）。
func TestGapBucketZeroShippedRegistryLoads(t *testing.T) {
	root := ""
	here, err := os.Getwd()
	if err != nil {
		t.Fatalf("取 cwd 失败：%v", err)
	}
	for p := here; ; p = filepath.Dir(p) {
		if gapAxisRecIsFile(filepath.Join(p, "core", "go.mod")) {
			root = p
			break
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	if root == "" {
		t.Skip("没找到仓根（core/go.mod 不在上溯链上）")
	}
	path := filepath.Join(root, gapBzRegistryRel)
	if !gapAxisRecIsFile(path) {
		t.Skipf("清单件尚未落仓（本单按「只写 scratch」执行）：%s", path)
	}
	reg, why := gapBzLoadRegistry(path)
	if reg == nil {
		t.Fatalf("清单件验形制不过：%s", why)
	}
	if len(reg.Targets) == 0 {
		t.Fatalf("清单件 targets 空 ⇒ 本门没有判据面")
	}
	for _, a := range reg.Allow {
		if strings.TrimSpace(a.Reason) == "" || strings.TrimSpace(a.Until) == "" {
			t.Fatalf("白名单条目缺 reason/until：%+v", a)
		}
	}
}

// 自检 17 条全绿。
func TestGapBucketZeroSelfTest(t *testing.T) {
	rc, out := bztRun(t, "--self-test")
	if rc != gapBzRCOK || !strings.Contains(out, "失败 0 条") {
		t.Fatalf("--self-test 未全绿 rc=%d\n%s", rc, out)
	}
}
