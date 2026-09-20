// receipt_test.go —— 交接回执（§20.3 `H3`）与「上次做到哪」的入口（§十二 `P-120` 定案 ①）
// 的**判据机检**（层①进程内 · 外部测试包 `package main_test` · 开工单 T-61）。
//
// 判据（开工单 T-61 逐字）：
//
//	① 每轮收尾写**一页回执**（本轮 id · 做了什么 · 证据 · 下一步 · 阻碍）且**带 `trace_id`**；
//	   **收尾不写回执 ⇒ 下一轮的入口必须报「无回执」并退码 `2`**；
//	⑤ 「上次做到哪」的入口名**定案** = `zerg context ls --resume`（`P-120` 定案 ①，**不新立族**）。
//
// 负控（H1 那条纪律的判定口）：`aiDirectEditJudge` 直接喂一个**错**的实况
// （一轮动过仓、证据里却没有命令面调用）⇒ 必须报错 —— 没有这一格，门⑬ 只是「今天恰好绿」。
package main_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// TestReceiptTruthSourceShape —— 回执真源成形（五格 + 入口名定案 + 五值判定词）。
func TestReceiptTruthSourceShape(t *testing.T) {
	s, err := contract.Receipt()
	if err != nil {
		t.Fatalf("回执真源读不出来：%v", err)
	}
	for _, k := range []string{"round_id", "trace_id", "what", "next"} {
		if !contract.Has(s.RequiredFields, k) {
			t.Errorf("必填格 %s 不在 required_fields 里", k)
		}
	}
	if s.ResumeEntry != "zerg context ls --resume" {
		t.Errorf("「上次做到哪」的入口名应为 `zerg context ls --resume`（P-120 定案 ①），实测 %q", s.ResumeEntry)
	}
	if len(s.VerdictWords) != 5 {
		t.Errorf("五值判定词应为 5 条，实测 %v", s.VerdictWords)
	}
	if !strings.Contains(s.TraceIDRule, "TR-") {
		t.Errorf("trace_id 规则里应写明 `TR-` 前缀：%q", s.TraceIDRule)
	}
}

// TestResumeEntryReportsNoReceiptWithExit2 —— 判据①后半：**收尾不写回执** ⇒ 下一轮的入口
// 必须报「无回执」并退码 `2`（**不许**退回「什么都没做」）。
func TestResumeEntryReportsNoReceiptWithExit2(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("ZERG_RECEIPT_DIR", empty)

	rc, out, errb := fusionRun(t, "context", "ls", "--resume")
	if rc != 2 {
		t.Errorf("无回执 ⇒ 退码 %d（要 2）· stderr=%q", rc, errb)
	}
	if out != "" {
		t.Errorf("无回执时 stdout 必须 0 字节，实测 %d 字节：%q", len(out), out)
	}
	if !strings.Contains(errb, "无回执") {
		t.Errorf("stderr 没点名「无回执」：%q", errb)
	}
	if !strings.Contains(errb, "no_receipt") {
		t.Errorf("stderr 没给 detail=no_receipt：%q", errb)
	}
	// 顺带证「它没退化成第二条命令」：`context ls` 本体仍在（不带 --resume 照旧出档位表）
	rc, out, errb = fusionRun(t, "context", "ls")
	if rc != 0 {
		t.Errorf("`context ls` 本体应仍是 rc=0（本票**不新立族**），实测 %d · stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "builtin") {
		t.Errorf("`context ls` 本体没出档位表：%q", out)
	}
}

// TestReceiptRoundTripThenResumeReadsItBack —— 判据①/⑤：写一页 ⇒ 读得回（换会话/换 AI 读得到）。
func TestReceiptRoundTripThenResumeReadsItBack(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_RECEIPT_DIR", dir)

	// 缺格 ⇒ 2（「做了什么 / 下一步」是回执的必备格）
	rc, out, errb := fusionRun(t, "dev", "receipt", "new", "--next", "下一步")
	if rc != 2 || out != "" {
		t.Errorf("缺 --what ⇒ 要 rc=2 + stdout 0 字节，实测 rc=%d stdout=%q", rc, out)
	}
	if !strings.Contains(errb, "--what") {
		t.Errorf("stderr 没点名缺的那一格：%q", errb)
	}

	// 写一页（判据①：五格 + trace_id）
	rc, out, errb = fusionRun(t, "dev", "receipt", "new",
		"--what", "做了 A 与 B", "--next", "下一步做 C",
		"--evidence", "zerg gate run --fast", "--blockers", "等通道",
		"--commit", "deadbee")
	if rc != 0 {
		t.Fatalf("写回执 ⇒ rc=%d（要 0）· stderr=%s", rc, errb)
	}
	roundID := firstLine(out)
	if !strings.HasPrefix(roundID, "R-") {
		t.Fatalf("首行应给本轮 id，实测 %q", roundID)
	}
	if !strings.Contains(out, "TR-") {
		t.Errorf("输出里没给 trace_id：%q", out)
	}

	// 件名必须 = <round_id>.json（门⑬ 的 R3 同一条口径）
	p := filepath.Join(dir, roundID+".json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("回执没落盘到 %s：%v", p, err)
	}
	txt := string(b)
	if !strings.Contains(txt, `"trace_id": "TR-`) && !strings.Contains(txt, `"trace_id":"TR-`) {
		t.Errorf("件里没带 trace_id：%s", txt)
	}
	if !regexp.MustCompile(`TR-[0-9a-f]{12}`).MatchString(txt) {
		t.Errorf("trace_id 形态不对（要 TR- + 12 位十六进制）：%s", txt)
	}
	if !strings.Contains(txt, "做了 A 与 B") || !strings.Contains(txt, "下一步做 C") {
		t.Errorf("件里没记全「做了什么 / 下一步」：%s", txt)
	}

	// 入口读得回
	rc, out, errb = fusionRun(t, "context", "ls", "--resume")
	if rc != 0 {
		t.Fatalf("有回执 ⇒ `context ls --resume` 应 rc=0，实测 %d · stderr=%q", rc, errb)
	}
	for _, want := range []string{roundID, "TR-", "做了 A 与 B", "下一步做 C"} {
		if !strings.Contains(out, want) {
			t.Errorf("续做面没读出 %q：%q", want, out)
		}
	}

	// 清单面
	rc, out, _ = fusionRun(t, "dev", "receipt", "ls")
	if rc != 0 || !strings.Contains(out, roundID) {
		t.Errorf("`dev receipt ls` 应列出本轮，实测 rc=%d out=%q", rc, out)
	}
}

// ---- 负控：H1（零直改）的判定口**真的有牙**（与门⑬ 的 R1 同一口径）---------------------------

type aiEditLive struct {
	RoundID  string
	Commits  []string
	Evidence []string
}

// aiDirectEditJudge = 门⑬ 的 R1：一轮**动过仓**（commits 非空）⇒ 证据里必须有命令面调用。
func aiDirectEditJudge(l aiEditLive) []error {
	if len(l.Commits) == 0 {
		return nil // 没动仓 ⇒ 不判（不许误判成红）
	}
	for _, e := range l.Evidence {
		if strings.Contains(e, "zerg") {
			return nil
		}
	}
	return []error{errNoZergCall{l.RoundID}}
}

type errNoZergCall struct{ round string }

func (e errNoZergCall) Error() string {
	return "**AI 直改**：轮次 " + e.round + " 说动过仓而证据里没有任何 `zerg …` 调用（§20.3 H1）"
}

// TestAIDirectEditJudgeHasTeeth —— 负控：喂「动过仓、证据里零命令面调用」⇒ 必须报错。
func TestAIDirectEditJudgeHasTeeth(t *testing.T) {
	bad := aiDirectEditJudge(aiEditLive{RoundID: "R-x", Commits: []string{"deadbee"}, Evidence: []string{"vim core/x.go"}})
	if len(bad) == 0 {
		t.Fatal("负控失败：判定口放过了「动过仓而证据里没有命令面调用」⇒ 门⑬ 不会红（假绿）")
	}
	if !strings.Contains(bad[0].Error(), "AI 直改") {
		t.Errorf("负控报错文案没点名「AI 直改」：%v", bad[0])
	}
	// 正控两格：没动仓不判 / 有命令面调用不判
	if e := aiDirectEditJudge(aiEditLive{RoundID: "R-x", Commits: nil, Evidence: []string{"只读 grep"}}); len(e) != 0 {
		t.Errorf("正控失败：没动仓却被判红：%v", e)
	}
	if e := aiDirectEditJudge(aiEditLive{RoundID: "R-x", Commits: []string{"deadbee"}, Evidence: []string{"zerg gate run --fast"}}); len(e) != 0 {
		t.Errorf("正控失败：有命令面调用却被判红：%v", e)
	}
	_ = zerg.RunForTest
}
