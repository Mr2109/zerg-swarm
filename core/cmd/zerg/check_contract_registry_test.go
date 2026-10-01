// check_contract_registry_test.go —— Go 真搬的**成对负控**定向单测（M2-0 · §4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckContractRegistryGatePort（定向：go test -run TestCheckContractRegistryGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Registry fixture：与真件同形的最小合成登记表（八字段齐 · 两条 entry）。
const ccrTestSelfRel = "scripts/gates/check-contract-registry.py"

func ccrTestGoodDoc() string {
	a := `{"id":"S-a","name":"甲件","truth":"core/internal/contract/a.json","truth_shape":"x",` +
		`"version_field":"schema","gate":"` + ccrTestSelfRel + `","change_class":"A","change_note":"x"}`
	b := `{"id":"S-b","name":"乙件","truth":"core/internal/contract/b.json","truth_shape":"x",` +
		`"version_field":"schema","gate":"由 core/agent 的单测覆盖（go test）","change_class":"B","change_note":"x"}`
	return `{"schema":"zerg-contract-registry/1","note":"真源见 ` + ccrTestSelfRel + `（本门）",` +
		`"authority":"夹具","entries":[` + a + `,` + b + `]}`
}

// ccrTestRoot —— 造一棵含锚点/表/Go 声明/门件的合成仓根（仓外 · t.TempDir()）。
func ccrTestRoot(t *testing.T, registry, declConst string) string {
	t.Helper()
	root := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk(filepath.Join("core", "internal", "version", "version.go"), "package version\n")
	mk(filepath.Join("core", "internal", "contract", "a.json"), "{}\n")
	mk(filepath.Join("core", "internal", "contract", "b.json"), "{}\n")
	mk(ccrTestSelfRel, "# 夹具门件\n")
	mk(filepath.Join("core", "cmd", "zerg", "family_impact_contract.go"),
		"package main\n\nconst impactContractRegistryGateRel = \""+declConst+"\"\n")
	mk(filepath.Join("core", "internal", "contract", "registry.json"), registry)
	return root
}

func ccrTestWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ccrTestRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := contractRegistryGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckContractRegistryGatePort(t *testing.T) {
	good := ccrTestGoodDoc()

	// ── 正控（必然判绿）：真件形状 ⇒ rc=0 ─────────────────────────────────
	root := ccrTestRoot(t, good, ccrTestSelfRel)
	rcGreen, outGreen, _ := ccrTestRun(t, "--root", root)
	if rcGreen != 0 || !strings.Contains(outGreen, "表 ↔ 件一致（rc=0）") {
		t.Fatalf("正控失败：好件应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控（必然判红）：R6 加一条**无门**的 entry ⇒ rc=1 ────────────────
	bad := strings.Replace(good, `]}`, `,{"id":"S-z","name":"无门的条目","truth":"core/internal/contract/a.json",`+
		`"truth_shape":"x","version_field":"schema","gate":"","change_class":"B","change_note":"x"}]}`, 1)
	rootNoGate := ccrTestRoot(t, bad, ccrTestSelfRel)
	rcRed, outRed, _ := ccrTestRun(t, "--root", rootNoGate)
	if rcRed != 1 || !strings.Contains(outRed, "S-z") {
		t.Fatalf("负控-R6 无门 失败：得 rc=%d out=%q", rcRed, outRed)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcRed {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcRed)
	}

	// ── 负控 · R5 truth 件不在盘 ─────────────────────────────────────────
	ghostTruth := strings.Replace(good, "core/internal/contract/a.json", "core/internal/contract/不在盘.json", 1)
	rcGT, outGT, _ := ccrTestRun(t, "--root", ccrTestRoot(t, ghostTruth, ccrTestSelfRel))
	if rcGT != 1 || !strings.Contains(outGT, "truth 件不在盘") {
		t.Fatalf("负控-R5 失败：得 rc=%d out=%q", rcGT, outGT)
	}

	// ── 负控 · R8 Go 侧声明漂 ────────────────────────────────────────────
	rcGo, outGo, _ := ccrTestRun(t, "--root", ccrTestRoot(t, good, "scripts/gates/other-thing.py"))
	if rcGo != 1 || !strings.Contains(outGo, "两处漂了") {
		t.Fatalf("负控-R8 失败：得 rc=%d out=%q", rcGo, outGo)
	}

	// ── 负控 · R2 缺字段 ─────────────────────────────────────────────────
	rcMiss, outMiss, _ := ccrTestRun(t, "--root",
		ccrTestRoot(t, strings.Replace(good, `,"change_note":"x"}`, "}", 1), ccrTestSelfRel))
	if rcMiss != 1 || !strings.Contains(outMiss, "缺字段") {
		t.Fatalf("负控-R2 失败：得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── 空转（≠ 绿）：entries 空 ⇒ rc=1（不当绿） ────────────────────────
	rcEmpty, outEmpty, _ := ccrTestRun(t, "--root",
		ccrTestRoot(t, `{"schema":"zerg-contract-registry/1","note":"x","authority":"x","entries":[]}`, ccrTestSelfRel))
	if rcEmpty != 1 || !strings.Contains(outEmpty, "空转") {
		t.Fatalf("空转失败：得 rc=%d out=%q", rcEmpty, outEmpty)
	}

	// ── 不给结论（≠ 绿）：取不到输入（根解析不到）⇒ rc=2 ─────────────────
	rcNo, outNo, _ := ccrTestRun(t, "--root", filepath.Join(t.TempDir(), "nope"))
	if rcNo != 2 || !strings.Contains(outNo, "解析不到仓根") {
		t.Fatalf("取不到输入失败：得 rc=%d out=%q", rcNo, outNo)
	}

	// ── 不给结论：表读不到 ⇒ rc=2 ────────────────────────────────────────
	rcNoReg, outNoReg, _ := ccrTestRun(t, "--root", root, "--registry", filepath.Join(t.TempDir(), "缺.json"))
	if rcNoReg != 2 || !strings.Contains(outNoReg, "读不到") {
		t.Fatalf("表读不到失败：得 rc=%d out=%q", rcNoReg, outNoReg)
	}

	// ── 不给结论：`--json` 字段表外 ⇒ rc=2（stderr 明说） ────────────────
	rcJSON, outJSON, errJSON := ccrTestRun(t, "--root", root, "--json", "zzz")
	if rcJSON != 2 || !strings.Contains(errJSON, "字段表外") || outJSON != "" {
		t.Fatalf("--json 拒收失败：得 rc=%d out=%q stderr=%q", rcJSON, outJSON, errJSON)
	}

	// ── 机器面：`--json entries,reds` ⇒ rc=0 且键按字母序（py json.dumps 形态） ──
	rcJOK, outJOK, _ := ccrTestRun(t, "--root", root, "--json", "entries,reds")
	if rcJOK != 0 || !strings.Contains(outJOK, `{"entries": 2, "reds": 0}`) {
		t.Fatalf("--json 机器面失败：得 rc=%d out=%q", rcJOK, outJOK)
	}

	// ── --self-test 十组自身必须全过（防恒绿/恒红） ──────────────────────
	rcST, outST, _ := ccrTestRun(t, "--root", root, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检全过（10 组") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}
