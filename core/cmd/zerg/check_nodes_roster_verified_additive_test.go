// check_nodes_roster_verified_additive_test.go —— 门 G（`cnrCheckModelsVerified`）对**加性键**的判定。
//
// 背景（GAP-20261003-21 · 设计《设计-verified三态与锚-v0.1-20261010》§三乙案 / §2.2）：核实面已从
// 裸 `verified`（bool）迁到**加性键** `verified_state`（三态 true|false|reported）＋ `verified_where`
// （reported 时必填 · 自报机名 ∈ 名册）。门 G 若只读裸 `verified` ⇒ 加性形态被当「无此键」静默跳过
// ⇒ 失明（缺 `verified_where` 也 rc=0）。本条给「同时认加性键 + state 非空为准」一颗牙。
//
// 成对负控（哑化「读加性键」那一句 ⇒ 副本 A/B/C 全 rc=0 ⇒ 本用例必红）：
//
//	A：`verified_state: reported` 缺 `verified_where` ⇒ rc=1 并点名
//	B：`verified_state: reported` ＋ `verified_where: ghost`（∉ 名册）⇒ rc=1
//	C：`verified_state: maybe`（闭集外）⇒ rc=1
//	OK：`verified_state: reported` ＋ `verified_where: edge-node`（∈ 名册）⇒ rc=0
//	旧态：`verified: true`（裸 bool · 无加性键）⇒ rc=0（向后兼容 · 既有条不受影响）
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cnrAdditiveFleet —— 取 G 夹具 fleet，替换 host/file/核实行，落到 dir 下。
func cnrAdditiveFleet(t *testing.T, dir, name, vline string) string {
	t.Helper()
	s := cnrReplaceAll(cnrFleetVerifiedFix, "__HOST__", "Mr2109")
	s = cnrReplaceAll(s, "__FILE__", "/nonexistent/none.gguf")
	s = cnrReplaceAll(s, "__VERIFIED__", vline)
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatalf("写夹具失败 %s: %v", p, err)
	}
	return p
}

// TestCheckNodesRoster_VerifiedAdditiveState —— 加性键三发 + OK + 旧态向后兼容。
func TestCheckNodesRoster_VerifiedAdditiveState(t *testing.T) {
	d := t.TempDir()
	roster := cnrWriteFixture(t, d, "roster.yaml", cnrRosterFix)
	plist := cnrWriteFixture(t, d, "a.plist", cnrPlistFix)

	cases := []struct {
		name  string
		vline string
		want  int
	}{
		{"OK：reported + verified_where:edge-node ∈ 名册", "verified_state: reported, verified_where: edge-node", 0},
		{"A：reported 缺 verified_where", "verified_state: reported", 1},
		{"B：reported + verified_where:ghost ∉ 名册", "verified_state: reported, verified_where: ghost", 1},
		{"C：verified_state:maybe 闭集外", "verified_state: maybe", 1},
		{"旧态：verified:true（裸 bool · 无加性键）向后兼容", "verified: true", 0},
	}
	for i, c := range cases {
		fleet := cnrAdditiveFleet(t, d, fmt.Sprintf("gadd_%d.yaml", i), c.vline)
		rc, out, _ := cnrRunMain("--roster", roster, "--fleet", fleet, "--plist", plist)
		if rc != c.want {
			t.Errorf("档 %q：期望 rc=%d 实得 rc=%d\n%s", c.name, c.want, rc, out)
			continue
		}
		// 点名：加性 A 档失败判词须点到该条（tag = mG（host=Mr2109））。
		if c.want == 1 && !strings.Contains(out, "G `mG（host=Mr2109）`") {
			t.Errorf("档 %q：失败判词未点名该条：\n%s", c.name, out)
		}
		// 加性 OK 档须真按「声明级（自报机=edge-node）」判，而非跳过。
		if c.want == 0 && i == 0 && !strings.Contains(out, "声明级（自报机=edge-node）") {
			t.Errorf("档 %q：加性 OK 未被判为声明级：\n%s", c.name, out)
		}
	}
}
