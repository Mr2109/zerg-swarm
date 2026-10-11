// cli_doctor_hostload_test.go —— 「本机过载体检」格（缺口 `GAP-20260926-202` 候选 c1 的只读落点）的回归牙。
//
// 判据：① 本机负载读得到 ⇒ 该格出现，`verdict ∈ {PASS, REPORT}`（**都 rc 中性** —— FAIL 会把「机器忙」红成
// 故障、SKIP 才算读不到），`detail` 含**明确「过载/不过载」判定**与 load1/ncpu 读数；② 端到端：`doctor --quick
// --json name,verdict,detail` 里能点到这一格（rc 不吃「未知字段」=2）。缺 loadavg/ncpu ⇒ SKIP（读不到不当健康）。
package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestDoctorHostLoadItemVerdict —— 正控：本机负载可读 ⇒ 该格给出明确「过载/不过载」判定，verdict rc 中性。
func TestDoctorHostLoadItemVerdict(t *testing.T) {
	it := doctorHostLoadItem(nil, false)
	if it["name"] != "本机过载体检" {
		t.Fatalf("项名 = %q（要「本机过载体检」）", it["name"])
	}
	switch it["verdict"] {
	case "PASS", "REPORT":
		// rc 中性两态，正是本项要的（过载只报告、不把机器忙红成故障）。
	case "SKIP":
		t.Fatalf("本机读不到 loadavg/ncpu（本格 SKIP）—— 判定面未成立：%s", it["detail"])
	default:
		t.Fatalf("verdict = %q（只许 PASS/REPORT：FAIL 会把「机器忙」红成故障）", it["verdict"])
	}
	d := it["detail"]
	if !strings.Contains(d, "过载") { // 「不过载」含「过载」子串 ⇒ 两态都须命中，哑化去掉判词即红。
		t.Errorf("detail 没有明确「过载/不过载」判定（只印了数字？）：%q", d)
	}
	if !strings.Contains(d, "load1=") || !strings.Contains(d, "ncpu=") {
		t.Errorf("detail 缺 load1/ncpu 读数：%q", d)
	}
}

// TestDoctorHostLoadItemEndToEnd —— 端到端：`doctor --quick` 机读面里点到「本机过载体检」这一格（rc 不吃 2）。
func TestDoctorHostLoadItemEndToEnd(t *testing.T) {
	var out, errb bytes.Buffer
	rc := RunForTest([]string{"doctor", "--quick", "--json", "name,verdict,detail"}, &out, &errb)
	if rc == 2 {
		t.Fatalf("`doctor --quick --json name,verdict,detail` rc=2（机读面被判未知字段）：%s", errb.String())
	}
	if !strings.Contains(out.String(), "本机过载体检") {
		t.Fatalf("机读面里没有「本机过载体检」这一格：%s", out.String())
	}
}
