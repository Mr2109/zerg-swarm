// check_contract_describes_truth_test.go —— 门·契约描述 ↔ 真源键集对拍（缺口 `GAP-20260927-228`）的判据面。
//
// 机检口（三面 · 逐面正控 + 负控）：
//
//	① 自检档自成一体：`--self-test` 的 10 格（正控 3 + 负控 7）必须全过（rc=0）。
//	② 盘上两态（真夹具 · `--root` 指到 `t.TempDir()`）：
//	   负控 —— 真源 5 键 / 描述只点 4 ⇒ rc=1 且**点名漏的那一格**；
//	   正控 —— 同一件补上第 5 格 ⇒ rc=0（与负控成对）。
//	③ 真树现读：真仓跑一次 ⇒ rc ∈ {0,1}（**不许 8**：8 = 读不到，说明根定位或输入面坏了），
//	   且输出里必须印「判据」行（描述的字面必须进报告 —— 否则报告与判据脱钩）。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// cdtFixture 造一个最小仓根：`core/internal/contract/{registry.json,t-five.json}`。
func cdtFixture(t *testing.T, shape, truth string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "core", "internal", "contract")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	reg := map[string]interface{}{"entries": []map[string]string{{
		"id":          "S-t",
		"name":        "夹具",
		"truth":       "core/internal/contract/t-five.json",
		"truth_shape": shape,
	}}}
	b, err := json.Marshal(reg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "registry.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "t-five.json"), []byte(truth), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const cdtTruth5 = `{"schema":"zerg/t/1","_出处":"夹具","甲":{"一":"1","二":"2","三":"3","四":"4","五":"5"}}`

// TestContractDescribesTruth_SelfTest ① 自检档（10 格成对）。
func TestContractDescribesTruth_SelfTest(t *testing.T) {
	rc, out := zerg.ContractDescribesTruthMainForTest([]string{"--self-test"})
	if rc != 0 {
		t.Fatalf("自检档 rc=%d（期望 0）：\n%s", rc, out)
	}
	if !strings.Contains(out, "自检全过") {
		t.Fatalf("自检档没报「自检全过」：\n%s", out)
	}
}

// TestContractDescribesTruth_OnDiskTwoStates ② 盘上两态：真源 5 键 ⟷ 描述 4 / 描述 5。
func TestContractDescribesTruth_OnDiskTwoStates(t *testing.T) {
	// 负控：漏 `五` 这一格。
	root := cdtFixture(t, "`甲`（一 / 二 / 三 / 四）", cdtTruth5)
	rc, out := zerg.ContractDescribesTruthMainForTest([]string{"--root", root})
	if rc != 1 {
		t.Fatalf("负控失败：真源 5 键 / 描述 4 键 ⇒ 期望 rc=1，实得 %d：\n%s", rc, out)
	}
	if !strings.Contains(out, "漏列 1/5") || !strings.Contains(out, "`五`") {
		t.Fatalf("负控失败：没点名漏的那一格（`五`）：\n%s", out)
	}
	// 正控：把第 5 格补上 ⇒ rc=0（与负控成对 · 只改描述一格）。
	root2 := cdtFixture(t, "`甲`（一 / 二 / 三 / 四 / 五）", cdtTruth5)
	rc2, out2 := zerg.ContractDescribesTruthMainForTest([]string{"--root", root2})
	if rc2 != 0 {
		t.Fatalf("正控失败：真源 5 键 / 描述 5 键 ⇒ 期望 rc=0，实得 %d：\n%s", rc2, out2)
	}
	// 判不了：真源件不在盘 ⇒ 8（✗ 不许当健康）。
	root3 := cdtFixture(t, "`甲`（一）", cdtTruth5)
	if err := os.Remove(filepath.Join(root3, "core", "internal", "contract", "t-five.json")); err != nil {
		t.Fatal(err)
	}
	if rc3, out3 := zerg.ContractDescribesTruthMainForTest([]string{"--root", root3}); rc3 != 8 {
		t.Fatalf("判不了档失败：真源不在盘 ⇒ 期望 rc=8，实得 %d：\n%s", rc3, out3)
	}
}

// TestContractDescribesTruth_RealTree ③ 真树现读（rc ∈ {0,1} · ✗ 不许 8）。
func TestContractDescribesTruth_RealTree(t *testing.T) {
	root := repoRootFromCLI(t)
	rc, out := zerg.ContractDescribesTruthMainForTest([]string{"--root", root})
	if rc != 0 && rc != 1 {
		t.Fatalf("真树 rc=%d（只许 0 或 1；8 = 读不到）：\n%s", rc, out)
	}
	if !strings.Contains(out, "判据：") {
		t.Fatalf("真树输出里没印「判据：」行（报告与判据脱钩）：\n%s", out)
	}
	// 机器面也必须能给（结构合法 · 键齐）。
	rcj, outj := zerg.ContractDescribesTruthMainForTest([]string{"--root", root, "--json"})
	if rcj != rc {
		t.Fatalf("`--json` 与文本面退码不一致（%d ⟷ %d）", rcj, rc)
	}
	var rep struct {
		Gate    string `json:"gate"`
		Entries int    `json:"entries"`
		Rc      int    `json:"rc"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(outj)), &rep); err != nil {
		t.Fatalf("`--json` 不是合法 JSON：%v\n%s", err, outj)
	}
	if rep.Gate != "check-contract-describes-truth" || rep.Entries == 0 || rep.Rc != rc {
		t.Fatalf("`--json` 机器面不成立：%+v", rep)
	}
}
