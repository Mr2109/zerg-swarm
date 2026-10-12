// validator_verified_state_test.go —— 启动校验对**加性键** `verified_state` / `verified_where` 的判定。
//
// 背景（GAP-20261003-21 · 设计《设计-verified三态与锚-v0.1-20261010》§二 2.2 / §六）：核实面已按
// §三乙案迁到加性键 `verified_state`（闭集 true|false|reported）＋ `verified_where`（reported 时必填 ·
// 自报机名 ∈ 名册）。启动校验器 `Validate`（validator.go）此前**不查**这两枚键 ⇒ 「并存期分叉」：
// 同一份 yaml 判据 G（check_nodes_roster.go 的 cnrCheckModelsVerified）判红、启动校验判过。
// 本条给「启动校验与判据 G 同口径」一颗牙。
//
// 成对负控（把 validator.go 的 V017 那一句哑化 ⇒ A/B/C 三格 HasFatal 全变 false ⇒ 本用例必红 ·
// 两态做法见报告 -overlay）：
//
//	正控：旧态 `verified: true`（裸 bool · 无加性键）⇒ 维持原判（不判红）
//	OK  ：`verified_state: reported` ＋ `verified_where: edge-node`（∈ 名册）⇒ 不判红
//	A   ：`verified_state: reported` 缺 `verified_where` ⇒ Fatal 并点名 verified_where
//	B   ：`verified_state: reported` ＋ `verified_where: ghost`（∉ 名册）⇒ Fatal
//	C   ：`verified_state: maybe`（闭集外）⇒ Fatal 并点名 verified_state
package config

import (
	"os"
	"testing"
)

// vsFleet —— 造一份最小 fleet（host 取非本机 edge-node ⇒ 不触发 V006 文件存在性），把 vline 塞进条目。
func vsFleet(vline string) string {
	return "models:\n  mV:\n    - { host: edge-node, backend: ds4-server, file: /x.gguf, mem_gb: 1, " + vline + " }\n"
}

// TestValidate_VerifiedStateAdditive —— 四态（正控 / OK / A / B / C）。
func TestValidate_VerifiedStateAdditive(t *testing.T) {
	// ★ 公开面（`ZERG_PUBLIC_TREE_ROOT=1`，如公开仓 CI）跳过：本用例断言「机型名 ∈ 名册」，
	//   用的是**私有机器型名**（`edge-node`，夹具与名册两侧同源）；脱敏后两侧被改成不同值 ⇒ 必红。
	//   前提（私有机型名册）在公开面不存在，不是判据失效。私有面行为一字不改。
	if os.Getenv("ZERG_PUBLIC_TREE_ROOT") == "1" {
		t.Skip("公开面：用例依赖私有机型名与名册一致 ⇒ 脱敏后前提不成立")
	}
	cases := []struct {
		name      string
		vline     string
		wantFatal bool
		wantField string
	}{
		{"正控：旧态 verified:true（裸 bool · 无加性键）维持原判", "verified: true", false, ""},
		{"OK：reported + verified_where:edge-node ∈ 名册", "verified_state: reported, verified_where: edge-node", false, ""},
		{"A：reported 缺 verified_where", "verified_state: reported", true, "verified_where"},
		{"B：reported + verified_where:ghost ∉ 名册", "verified_state: reported, verified_where: ghost", true, "verified_where"},
		{"C：verified_state:maybe 闭集外", "verified_state: maybe", true, "verified_state"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseFleetConfig([]byte(vsFleet(tc.vline)))
			if err != nil {
				t.Fatalf("解析失败（加性键不该让整份表红）: %v", err)
			}
			cands := cfg.Models["mV"]
			if len(cands) == 0 {
				t.Fatal("没解析出候选")
			}
			res := Validate("mV", cands[0])
			if got := res.HasFatal(); got != tc.wantFatal {
				t.Errorf("HasFatal=%v（要 %v）· errors=%v", got, tc.wantFatal, collectFields(res.Errors))
			}
			if tc.wantField != "" {
				found := false
				for _, e := range res.Errors {
					if e.Level == FatalLevel && e.Field == tc.wantField {
						found = true
						if e.Message == "" {
							t.Errorf("Fatal 字段 %q 报话为空", tc.wantField)
						}
					}
				}
				if !found {
					t.Errorf("期望 %q 的 Fatal（点名），得到: %v", tc.wantField, collectFields(res.Errors))
				}
			}
		})
	}
}
