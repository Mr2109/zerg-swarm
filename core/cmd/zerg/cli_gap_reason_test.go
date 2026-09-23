// cli_gap_reason_test.go —— 缺口 `Q-138`（`zerg gap ls` 本机恒退 8）的**机检**。
//
// **本条治什么**（设计 `设计-CLI机器读面-v1.0-20260924.md` §1.3 坑 3 · §3.2 坑 3 · §4.2 行 19）：
// 「真源**不在盘上**」与「真源**在盘上但读不出来**」两因今天**同码同档**（都 `rc=8`、只有一句人面
// 文案不同）⇒ 机器面**分不出是哪一种**（与 `Q-144`「两态同码」**同形** · §1.2 归**乙类**）。
//
// **判据**（全在本件现跑 · 合成状态目录，**不碰真 `~/.zerg/state`**）：
//
//	① 真源缺         ⇒ `rc=8`（**仍非 0** · fail-closed 不变）· `meta.reason=ledger_absent`
//	② 真源在但读不出 ⇒ `rc=8`（同上）· `meta.reason=precondition_missing`
//	③ 两者**必须不同**（这就是本条缺口要的判别力）；而 `rc` **仍可同为 8**（只加字段、不动退码）
//	④ `error.kind` **仍是闭集里的 `blocked`**（**不扩 kind 闭集** ⇒ 门⑤ 现跑的「闭集 18 个 kind」不动）
//	⑤ `meta.ledger_path` 逐字 = 真源落点；stderr **首行**带固定前缀短语（机器可 grep）
//	⑥ 负控（fail-closed）：真源删掉 ⇒ **仍非 0**（「没有」不许当零命中）
package main_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// gapErrEnvelope —— `--json` 失败路径的包封（只取本件要判的格）。
type gapErrEnvelope struct {
	Kind  string         `json:"kind"`
	Items []any          `json:"items"`
	Meta  map[string]any `json:"meta"`
	Err   struct {
		Kind      string `json:"kind"`
		Detail    string `json:"detail"`
		Retryable bool   `json:"retryable"`
		Remedy    string `json:"remedy"`
		ExitCode  int    `json:"exit_code"`
		Message   string `json:"message"`
	} `json:"error"`
}

func gapErrEnvelopeOf(t *testing.T, argv ...string) (int, gapErrEnvelope, string) {
	t.Helper()
	rc, out, errb := gapRun(t, argv...)
	var env gapErrEnvelope
	if strings.TrimSpace(out) != "" {
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("包封解析不了：%v · %q", err, out)
		}
	}
	return rc, env, errb
}

func gapReasonOf(env gapErrEnvelope) string {
	if v, ok := env.Meta["reason"].(string); ok {
		return v
	}
	return ""
}

// TestGapQ138_TwoReasonsMachineSeparable —— 判据 ①–⑥（`gap ls` 这一面）。
func TestGapQ138_TwoReasonsMachineSeparable(t *testing.T) {
	_, ledger, _ := gapEnv(t)

	// ① 真源**不在盘上** ⇒ 8 · ledger_absent · kind 仍 blocked · ledger_path 逐字
	rc, env, errb := gapErrEnvelopeOf(t, "gap", "ls", "--json", "id")
	if rc != 8 {
		t.Fatalf("真源缺 ⇒ rc 仍要 8（fail-closed），得到 %d · stderr=%s", rc, errb)
	}
	if env.Err.Kind != "blocked" {
		t.Errorf("error.kind 要仍是闭集里的 blocked（不扩 kind 闭集），得到 %q", env.Err.Kind)
	}
	if env.Err.Detail != "ledger_absent" {
		t.Errorf("error.detail 要 ledger_absent，得到 %q", env.Err.Detail)
	}
	if got := gapReasonOf(env); got != "ledger_absent" {
		t.Errorf("meta.reason 要 ledger_absent，得到 %q", got)
	}
	if got, _ := env.Meta["ledger_path"].(string); got != ledger {
		t.Errorf("meta.ledger_path 要逐字 %q，得到 %q", ledger, got)
	}
	if env.Err.ExitCode != 8 {
		t.Errorf("error.exit_code 要 8，得到 %d", env.Err.ExitCode)
	}
	if !strings.HasPrefix(errb, "zerg: error.kind=blocked · detail=ledger_absent · reason=ledger_absent") {
		t.Errorf("stderr 首行要带固定前缀短语（error.kind=blocked · detail=ledger_absent · reason=ledger_absent），得到 %q", gapFirstLine(errb))
	}
	// 「去哪找 / 怎么补」两件都要在（路径 + 最小修复命令）
	if !strings.Contains(errb, ledger) {
		t.Errorf("真源缺那一态的人面要给路径，得到 %q", errb)
	}
	if !strings.Contains(errb, "zerg gap add") {
		t.Errorf("真源缺那一态的人面要给最小修复命令（`zerg gap add …`），得到 %q", errb)
	}
	reasonAbsent := gapReasonOf(env)

	// ② 真源**在盘上但读不出来**（某行不是 JSON）⇒ rc 仍 8 · precondition_missing
	if err := os.WriteFile(ledger, []byte("{这不是 JSON}\n"), 0o644); err != nil {
		t.Fatalf("夹具写坏真源失败：%v", err)
	}
	rc2, env2, errb2 := gapErrEnvelopeOf(t, "gap", "ls", "--json", "id")
	if rc2 != 8 {
		t.Errorf("真源在但读不出 ⇒ rc 仍要 8，得到 %d · stderr=%s", rc2, errb2)
	}
	if env2.Err.Kind != "blocked" {
		t.Errorf("error.kind 要仍是 blocked，得到 %q", env2.Err.Kind)
	}
	if env2.Err.Detail != "precondition_missing" {
		t.Errorf("error.detail 要 precondition_missing，得到 %q", env2.Err.Detail)
	}
	if got := gapReasonOf(env2); got != "precondition_missing" {
		t.Errorf("meta.reason 要 precondition_missing，得到 %q", got)
	}
	if !strings.HasPrefix(errb2, "zerg: error.kind=blocked · detail=precondition_missing · reason=precondition_missing") {
		t.Errorf("stderr 首行要带固定前缀短语（… detail=precondition_missing …），得到 %q", gapFirstLine(errb2))
	}

	// ③ 这就是本条缺口的判别力：两因**必须不同**，而 rc **仍可同为 8**（只加字段、不动退码）
	if reasonAbsent == gapReasonOf(env2) {
		t.Errorf("两因分不开了（都 %q）—— `Q-138` 没治住", reasonAbsent)
	}
	if rc != rc2 {
		t.Errorf("退码不许动：真源缺 %d ⇄ 真源在但读不出 %d", rc, rc2)
	}
	// 两值逐字 = 设计稿 §4.2 行 19 给的那一枚两值闭集（不另造词）
	for _, r := range []string{reasonAbsent, gapReasonOf(env2)} {
		if r != "ledger_absent" && r != "precondition_missing" {
			t.Errorf("reason %q 不在设计稿的两值闭集 {ledger_absent, precondition_missing} 里", r)
		}
	}

	// ④ 真源在盘、空 ⇒ 1（零命中）—— 与「读不到」**不同区分**（机器面能分开「读不到」与「一条都没有」）
	if err := os.WriteFile(ledger, nil, 0o644); err != nil {
		t.Fatalf("夹具建空真源失败：%v", err)
	}
	if rc, _, errb := gapRun(t, "gap", "ls"); rc != 1 {
		t.Errorf("真源在但 0 行 ⇒ 1（零命中），得到 %d · %s", rc, errb)
	}

	// ⑤ 真源在盘、有内容 ⇒ 0（有面可读 —— 这正是缺口账那条命令面要的东西）
	if rc, _, errb := gapRun(t, append(gapAddArgv("h-q138", "Q-138 夹具"), "--yes")...); rc != 0 {
		t.Fatalf("打底 add --yes 要 0：rc=%d · %s", rc, errb)
	}
	if rc, _, errb := gapRun(t, "gap", "ls"); rc != 0 {
		t.Errorf("真源有内容 ⇒ 0，得到 %d · %s", rc, errb)
	}

	// ⑥ 负控（fail-closed）：真源删掉 ⇒ **仍非 0**（「没有」不许当零命中）
	if err := os.Remove(ledger); err != nil {
		t.Fatalf("夹具删真源失败：%v", err)
	}
	if rc, _, errb := gapRun(t, "gap", "ls"); rc == 0 {
		t.Errorf("负控：真源删掉 ⇒ 不许绿，得到 %d · %s", rc, errb)
	}
}

// TestGapQ138_VerifySameTwoReasons —— 同一条真源、同一枚两值（`verify` 面**不许**各判一套）。
func TestGapQ138_VerifySameTwoReasons(t *testing.T) {
	_, ledger, _ := gapEnv(t)

	rc, env, errb := gapErrEnvelopeOf(t, "gap", "verify", "--all", "--yes", "--json", "id")
	if rc != 8 {
		t.Fatalf("真源缺 ⇒ verify 要 8，得到 %d · %s", rc, errb)
	}
	if gapReasonOf(env) != "ledger_absent" || env.Err.Kind != "blocked" {
		t.Errorf("verify 那一面要同为 kind=blocked / reason=ledger_absent，得到 kind=%q reason=%q", env.Err.Kind, gapReasonOf(env))
	}

	if err := os.WriteFile(ledger, []byte("{这不是 JSON}\n"), 0o644); err != nil {
		t.Fatalf("夹具写坏真源失败：%v", err)
	}
	rc2, env2, errb2 := gapErrEnvelopeOf(t, "gap", "verify", "--all", "--yes", "--json", "id")
	if rc2 != 8 {
		t.Errorf("真源在但读不出 ⇒ verify 仍要 8，得到 %d · %s", rc2, errb2)
	}
	if gapReasonOf(env2) != "precondition_missing" {
		t.Errorf("verify 那一面要 reason=precondition_missing，得到 %q", gapReasonOf(env2))
	}
	if gapReasonOf(env) == gapReasonOf(env2) {
		t.Errorf("verify 两面分不开（都 %q）", gapReasonOf(env))
	}
}
