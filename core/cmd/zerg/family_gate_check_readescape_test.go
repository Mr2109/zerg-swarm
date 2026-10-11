// family_gate_check_readescape_test.go —— 批A（P1 丁）的**回归牙**：公共小口
// `gateCheckReadJudgeEscape` 与判据链的同口径放行（本单新增件 · 提案 DEV-2515）。
//
// 面：`zerg gate check <判据名>` 此前**一律**被判「写面」（校验器只读命令树**壳节点**的 danger
// `dangerD3`）⇒ 账里以只读判据为 verify_cmd 的条目改不动、`dev proposal --criterion` 也立不了判据
// （现读实测拒词：「判据里的 `gate check` 在本版是危险档、真跑未开放」）。
//
// 治（本件钉住）：`gate check <名>` 若该名在**中央分类表**（`gate_inproc_table.go:221`）里是
// **只读档** ⇒ 判据本体零副作用 ⇒ 放行；✗ 不放宽其余档。窄面靠两条负控钉死：
//   ① 非 `gate check` 命令（`dev release …`）不得放行 —— 既有负控 `dev_proposal_test.go:143-148`
//      断言它 rc=2 且 stderr 点名「危险档」；
//   ② `gate check` 但**写档**判据名（`build-all` 等）不得放行。
//
// 同形先例：`family_gap_writeface_action_test.go`（同包 · 直调包内函数 + 断言 why）。

package main

import "testing"

// 〔正控〕只读档：共口放行 · 且三处判据链同口径。
func TestGateCheckReadJudgeEscapeAllowsReadJudge(t *testing.T) {
	if !gateCheckReadJudgeEscape([]string{"gate", "check"}, []string{"check-doc-name"}) {
		t.Error("只读判据名应放行（共口）")
	}
	if why := gapWriteFaceWhy("zerg gate check check-doc-name"); why != "" {
		t.Errorf("写面闸应放行只读判据；why = %q", why)
	}
	if why := gapJudgeUnknownFlag([]string{"gate", "check", "gen-cli-reference", "--check"}); why != "" {
		t.Errorf("透传命令（gate check 自带 passthrough）的子件旗标不该判未知旗标；why = %q", why)
	}
}

// 〔负控①〕非 `gate check` 命令：一律不得放行（钉住「✗ 别放宽成闸一律放行」）。
func TestGateCheckReadJudgeEscapeRefusesOtherCommands(t *testing.T) {
	if gateCheckReadJudgeEscape([]string{"dev", "release"}, []string{"--candidate", "DEV-0001"}) {
		t.Error("非 gate check 命令不得放行")
	}
	if why := gapWriteFaceWhy("zerg dev release --candidate DEV-0001"); why == "" {
		t.Error("dev release 仍须判写面（既有负控要求其 stderr 点名「危险档」）")
	}
}

// 〔负控②〕`gate check` 但**写档**判据名 / 无名：不得放行。
func TestGateCheckReadJudgeEscapeRefusesWriteJudge(t *testing.T) {
	if gateCheckReadJudgeEscape([]string{"gate", "check"}, []string{"build-all"}) {
		t.Error("写档判据名（中央表 write）不得放行")
	}
	if gateCheckReadJudgeEscape([]string{"gate", "check"}, nil) {
		t.Error("无判据名不得放行")
	}
}
