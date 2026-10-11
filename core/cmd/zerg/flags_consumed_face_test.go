// flags_consumed_face_test.go —— `zerg help flags-consumed` 的**对账牙**（缺口 `GAP-20260928-37` ·
// 待拍项 #20「「接受即忽略」的可见面出口」）。
//
// 病（照账读出来）：解析器层登记了一批**全体命令都收下**的旗标名（如 `--machine`），而多数命令实现件
// **从不读它** —— 被吃掉时**任何可见面都没有痕迹**（rc=0 · stdout 与不带时**逐字节相同**）。这不是 bug
// （是**签过字的口径**），问题是**用户看不见**：他以为 `zerg port ls --machine edge-node` 是查 edge-node，实际查的是本机。
//
// 治法（待拍项 #20 的推荐形态）：加一个**只读帮助主题** `zerg help flags-consumed`，把「登记面 × 消费面」
// 的对账口径落成一处可查的自描述面（照 `zerg help dangerous` / `zerg help skill` 同形）。
//
// 本件的牙（四档 · **纯函数判定口** + 正控/负控 · 照 `cli_danger_opened_test.go` 同一族形状）：
//
//	① 正控：`renderHelpTopic("flags-consumed")` ⇒ rc=0，正文含口径句 + 旗标块；判定口报空。
//	② **对账**：旗标块逐条 ⟷ **唯一真源** `fcDispatchConsumedFlags`（同源 · ✗ 不许两处各抄一份）；
//	   负控：把正文里任一枚旗标挪走 ⇒ 判定口**必红**（证明它不是恒绿空转）。
//	③ 负控：认不出的主题仍走「未知帮助主题」rc=2（本面**没把出口放宽**）。
//	④ 对界：既有主题 `dangerous` 一字不动（本面只**加一个主题**）。
//
// ★ 本件**不记任何绝对条数**（危险档总条数 / 已开放 / 未开放一律现读，见 `cli_danger_opened_test.go`
// 的同一口径 —— 缺口 `GAP-20260927-357`）。
package main

import (
	"strconv"
	"strings"
	"testing"
)

// fcBlockHeader —— 旗标块的表头（正文里那一条「派发层替全体消费的只有：」）。
const fcBlockHeader = "派发层替全体消费的只有"

// fcBlockFlags 从主题正文里抽旗标块那一段**逐条**的旗标
// （紧跟表头之后、以「两个空格 + `--`」开头的连续行，遇到第一条不以此开头的行即停）。
func fcBlockFlags(out string) []string {
	lines := strings.Split(out, "\n")
	start := -1
	for i, ln := range lines {
		if strings.Contains(ln, fcBlockHeader) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}
	var got []string
	for _, ln := range lines[start:] {
		if !strings.HasPrefix(ln, "  --") {
			break
		}
		got = append(got, strings.TrimSpace(ln))
	}
	return got
}

// fcProblems —— 判定口（**纯函数**）：正文 ⟷ 真源 `fcDispatchConsumedFlags` 的对账。
// 报空 ⇒ 相符（「可得且相符」）；报非空 ⇒ 逐条点名（「对账不符」）。
func fcProblems(out string) []string {
	var bad []string
	if !strings.Contains(out, fcBlockHeader) {
		return append(bad, "正文里没有旗标块表头「"+fcBlockHeader+"」（口径面被删？）")
	}
	got := fcBlockFlags(out)
	if len(got) != len(fcDispatchConsumedFlags) {
		bad = append(bad, "旗标块条数 "+strconv.Itoa(len(got))+" ≠ 真源 "+strconv.Itoa(len(fcDispatchConsumedFlags)))
	}
	for _, want := range fcDispatchConsumedFlags {
		if !fcHas(got, want) {
			bad = append(bad, "旗标块漏了真源里的 "+want)
		}
	}
	for _, g := range got {
		if !fcHas(fcDispatchConsumedFlags, g) {
			bad = append(bad, "旗标块多了真源里没有的 "+g)
		}
	}
	return bad
}

func fcHas(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// fcFirstLine 取首行（判「首行逐字」用）。
func fcFirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// ① 正控 + ② 对账：正文可得（rc=0）且与唯一真源相符（判定口报空）。
func TestFlagsConsumedFace_PositiveControlAndReconcile(t *testing.T) {
	var out, errb strings.Builder
	rc := renderHelpTopic("flags-consumed", &out, &errb)
	if rc != 0 {
		t.Fatalf("`zerg help flags-consumed` 要 rc=0（改前 rc=2「未知帮助主题 flags-consumed」），得到 rc=%d · stderr=%q", rc, errb.String())
	}
	body := out.String()
	for _, want := range []string{"接受即忽略", "谁用谁读", "恒退 0", fcBlockHeader} {
		if !strings.Contains(body, want) {
			t.Errorf("正文缺口径句 %q：\n%s", want, body)
		}
	}
	if bad := fcProblems(body); len(bad) > 0 {
		t.Errorf("对账不符（正文 ⟷ 真源 `fcDispatchConsumedFlags`）：%v", bad)
	}
}

// ② 负控：挪走正文里任一枚真源旗标 ⇒ 判定口**必红**（证明它不是恒绿空转）。
func TestFlagsConsumedFace_ReconcileHasTeeth(t *testing.T) {
	var out, errb strings.Builder
	if rc := renderHelpTopic("flags-consumed", &out, &errb); rc != 0 {
		t.Fatalf("前置不成立：rc=%d（stderr=%q）", rc, errb.String())
	}
	body := out.String()
	// 前提：原样必须相符（否则负控无意义）。
	if bad := fcProblems(body); len(bad) > 0 {
		t.Fatalf("负控前置不成立：原样就对账不符 %v", bad)
	}
	first := fcDispatchConsumedFlags[0]
	dropped := strings.Replace(body, "  "+first+"\n", "", 1)
	if dropped == body {
		t.Fatalf("夹具没挪到任何旗标行（真源第一枚 %q）", first)
	}
	if len(fcProblems(dropped)) == 0 {
		t.Errorf("负控失败：挪走真源旗标 %q 后判定口仍报空 ⇒ 判据是恒绿空转", first)
	}
}

// ③ 负控：认不出的主题仍走「未知帮助主题」rc=2（出口没放宽）。
func TestFlagsConsumedFace_UnknownTopicStillRefused(t *testing.T) {
	var out, errb strings.Builder
	rc := renderHelpTopic("flags-consumed-没有这个主题-zz", &out, &errb)
	if rc != 2 || !strings.HasPrefix(errb.String(), "zerg: 未知帮助主题") {
		t.Errorf("认不出的主题仍要 rc=2 + 「未知帮助主题」（出口没放宽），得到 rc=%d · stderr=%q", rc, errb.String())
	}
}

// ④ 对界：既有主题 `dangerous` 一字不动（本面只加一个主题）。
func TestFlagsConsumedFace_ExistingTopicsUntouched(t *testing.T) {
	var out, errb strings.Builder
	rc := renderHelpTopic("dangerous", &out, &errb)
	if rc != 0 || !strings.Contains(out.String(), "危险动作清单") {
		t.Errorf("`zerg help dangerous` 应一字不动：rc=%d · stderr=%q · out首行=%q", rc, errb.String(), fcFirstLine(out.String()))
	}
}
