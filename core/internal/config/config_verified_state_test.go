package config

import "testing"

// GAP-20261003-21（加性档）：消费者 schema 承载 verified 三态与内容锚 —— 本用例给「加性」一颗牙。
//
// 两态语义（设计稿 §三乙案 · 加性）：旧 `verified: true/false` 一字不改（零破坏）；第三态
// `reported` 与自报机名 `verified_where` 走**加性新键**，不再逼着消费者把新值塞进旧 `verified`
// 字段（那会让整份 fleet.yaml 解析失败 ⇒ 既有已 verified 的全翻红）。
// ★ 哑化加性（去掉 VerifiedState / VerifiedWhere 的 yaml 绑定）⇒ 本用例红。
func TestModelCandidateCarriesVerifiedStateAdditive(t *testing.T) {
	const doc = `
models:
  M-Old-True:
    - { host: node-a, backend: ds4-server, file: /x.gguf, mem_gb: 1, verified: true }
  M-Old-False:
    - { host: node-a, backend: ds4-server, file: /y.gguf, mem_gb: 1, verified: false }
  M-Reported:
    - { host: node-b, backend: ds4-server, file: /z.gguf, mem_gb: 1, verified_state: reported, verified_where: edge-node }
`
	cfg, err := ParseFleetConfig([]byte(doc))
	if err != nil {
		t.Fatalf("解析失败（加性键不该让整份表红）: %v", err)
	}
	// 旧态零破坏：verified: true/false 仍按 bool 读。
	if c := cfg.Models["M-Old-True"][0]; !c.Verified {
		t.Fatalf("旧态破：verified: true 未按 bool 读（Verified=%v）", c.Verified)
	}
	if c := cfg.Models["M-Old-False"][0]; c.Verified {
		t.Fatalf("旧态破：verified: false 未按 bool 读（Verified=%v）", c.Verified)
	}
	// 新态：加性键必须被承载（哑化即在此红）。
	c := cfg.Models["M-Reported"][0]
	if c.VerifiedState != "reported" {
		t.Fatalf("加性字段未承载：VerifiedState=%q（要 reported）—— 加性档被哑化？", c.VerifiedState)
	}
	if c.VerifiedWhere != "edge-node" {
		t.Fatalf("加性字段未承载：VerifiedWhere=%q（要 edge-node）—— 加性档被哑化？", c.VerifiedWhere)
	}
}
