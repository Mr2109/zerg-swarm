// ledger_family.go —— 「多件族」账本登记口径的**唯一登记处**（GAP-20260927-323）。
// 门与生成器不得再各写一套件名规则；新账本名/新族名只进这张表。
//
// 收编面 v0：check_face_parity.go 的族名关键词、check_gap_axis_reconcile.go 的账本名。
// 其余散写 zerg-cli-gaps.jsonl 的件后续逐件收编（另立缺口，勿在本件扩大）。

package main

// ledgerFamilyNames —— 账本文件名（多件族成员名）登记表。
var ledgerFamilyNames = map[string]string{
	"gaps": "zerg-cli-gaps.jsonl",
}

// ledgerFamilyHints —— 门件名里判定「账本族」的关键词（族名口径）。
var ledgerFamilyHints = []string{"ledger", "judge"}
