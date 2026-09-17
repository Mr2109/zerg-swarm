// tracectx_test.go —— T1.6 传播载体实证。
//
// 分三层，缺一不可：
//
//	① **往返一致**：Parse(Format(x)) == x（含 W3C 规范里的那个公开例子 + 200 次随机）。
//	② **非法一律拒绝**：长度/段数/版本/大小写/全零/flags 位数/flags 非法 —— 每条一个用例（负例 ≥ 5）。
//	③ **绝不静默拿空串当 trace_id**：上游没给 ⇒ 本侧生成 root（并如实标记）；给了非法 ⇒ 拒绝 + 生成。
package tracectx

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

// W3C Trace Context 规范里的公开示例（L2 §3.2.2.1 节选）：往返必须逐字节复原。
const w3cSample = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func mustParse(t *testing.T, s string) TraceContext {
	t.Helper()
	tc, err := ParseTraceparent(s)
	if err != nil {
		t.Fatalf("解析 %q 失败：%v", s, err)
	}
	return tc
}

// ① 往返一致：parse(format(x)) == x，且 format(parse(s)) == 规范形 s
func TestTraceparentRoundTrip(t *testing.T) {
	// ①-1 规范示例：逐字节复原
	tc := mustParse(t, w3cSample)
	if got, err := FormatTraceparent(tc); err != nil || got != w3cSample {
		t.Errorf("规范示例往返不一致：got=%q err=%v want=%q", got, err, w3cSample)
	}
	// ①-2 flags 全值域里挑几个代表（00 / 01 / ff）都要能往返
	for _, f := range []byte{0x00, 0x01, 0x0f, 0x10, 0xff} {
		in := TraceContext{TraceID: tc.TraceID, SpanID: tc.SpanID, Flags: f}
		s, err := FormatTraceparent(in)
		if err != nil {
			t.Fatalf("flags=%#x 生成失败：%v", f, err)
		}
		out, err := ParseTraceparent(s)
		if err != nil {
			t.Fatalf("flags=%#x 生成后再解析失败：%v（原文 %q）", f, err, s)
		}
		if out != in {
			t.Errorf("parse(format(x)) != x：in=%+v out=%+v（原文 %q）", in, out, s)
		}
	}
	// ①-3 本侧生成 200 条：生成→解析必须回到同一个上下文（随机位宽也要对得上）
	for i := 0; i < 200; i++ {
		root, err := Root()
		if err != nil {
			t.Fatalf("第 %d 次生成 root 失败：%v", i, err)
		}
		s, err := FormatTraceparent(root)
		if err != nil {
			t.Fatalf("第 %d 次格式化失败：%v", i, err)
		}
		if len(s) != len(w3cSample) {
			t.Fatalf("规范形长度应 %d，实际 %d（%q）", len(w3cSample), len(s), s)
		}
		back, err := ParseTraceparent(s)
		if err != nil {
			t.Fatalf("第 %d 次回解失败：%v（%q）", i, err, s)
		}
		if back != root {
			t.Fatalf("往返不一致：%+v → %q → %+v", root, s, back)
		}
		// 子 span：trace-id 不变、span-id 必须换新（每跳一个）
		child, err := root.Child()
		if err != nil {
			t.Fatalf("第 %d 次开子 span 失败：%v", i, err)
		}
		if child.TraceID != root.TraceID || child.SpanID == root.SpanID || child.Flags != root.Flags {
			t.Fatalf("子 span 口径不符：root=%+v child=%+v", root, child)
		}
	}
}

// ② 非法输入一律拒绝（每条：必须报错 + 报对原因）
func TestTraceparentRejectsIllegal(t *testing.T) {
	const valid = "4bf92f3577b34da6a3ce929d0e0e4736"
	const span = "00f067aa0ba902b7"
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"空串", "", ErrEmpty},
		{"仅空白", "   ", ErrEmpty},
		{"段数不足（3 段）", "00-" + valid + "-" + span, ErrShape},
		{"段数过多（5 段）", "00-" + valid + "-" + span + "-01-x", ErrShape},
		{"版本不是 00", "01-" + valid + "-" + span + "-01", ErrVersion},
		{"版本是 ff", "ff-" + valid + "-" + span + "-01", ErrVersion},
		{"trace-id 太短", "00-abc-" + span + "-01", ErrLength},
		{"trace-id 太长（超过 32）", "00-" + valid + "00-" + span + "-01", ErrLength},
		{"span-id 太长（超过 16）", "00-" + valid + "-" + span + "00-01", ErrLength},
		{"span-id 太短", "00-" + valid + "-0f067aa-01", ErrLength},
		{"trace-id 含大写", "00-" + strings.ToUpper(valid) + "-" + span + "-01", ErrNotHex},
		{"trace-id 含非十六进制", "00-4bf92f3577b34da6a3ce929d0e0e473z-" + span + "-01", ErrNotHex},
		{"trace-id 全零（W3C 禁止）", "00-" + strings.Repeat("0", 32) + "-" + span + "-01", ErrZeroID},
		{"span-id 全零（W3C 禁止）", "00-" + valid + "-" + strings.Repeat("0", 16) + "-01", ErrZeroID},
		{"flags 只 1 位", "00-" + valid + "-" + span + "-1", ErrLength},
		{"flags 3 位", "00-" + valid + "-" + span + "-010", ErrLength},
		{"flags 非十六进制", "00-" + valid + "-" + span + "-zz", ErrNotHex},
		{"flags 大写", "00-" + valid + "-" + span + "-0A", ErrNotHex},
	}
	if len(cases) < 5 {
		t.Fatalf("负例不足：只有 %d 条（要求 ≥ 5）", len(cases))
	}
	for _, c := range cases {
		tc, err := ParseTraceparent(c.in)
		if err == nil {
			t.Errorf("【%s】非法输入被接受：%q → %+v", c.name, c.in, tc)
			continue
		}
		if !errors.Is(err, c.want) {
			t.Errorf("【%s】拒绝原因不对：%v（应 %v）", c.name, err, c.want)
		}
		// 拒绝 ⇒ 绝不返回"半个上下文"（调用方若拿零值当 id 用会写出坏头）
		if !tc.IsZero() {
			t.Errorf("【%s】拒绝后必须返回零值上下文，实际 %+v", c.name, tc)
		}
	}
}

// ②-补 生成侧同样拒绝非法上下文（不"顺手修一下"）
func TestFormatRejectsIllegal(t *testing.T) {
	const valid = "4bf92f3577b34da6a3ce929d0e0e4736"
	cases := []struct {
		name string
		in   TraceContext
		want error
	}{
		{"空上下文", TraceContext{}, ErrLength},
		{"trace-id 太短", TraceContext{TraceID: "abc", SpanID: "00f067aa0ba902b7"}, ErrLength},
		{"trace-id 含大写", TraceContext{TraceID: strings.ToUpper(valid), SpanID: "00f067aa0ba902b7"}, ErrNotHex},
		{"span-id 全零", TraceContext{TraceID: valid, SpanID: strings.Repeat("0", 16)}, ErrZeroID},
	}
	for _, c := range cases {
		s, err := FormatTraceparent(c.in)
		if err == nil || s != "" {
			t.Errorf("【%s】非法上下文必须报错且不产出字符串：s=%q err=%v", c.name, s, err)
			continue
		}
		if !errors.Is(err, c.want) {
			t.Errorf("【%s】原因不对：%v（应 %v）", c.name, err, c.want)
		}
	}
}

// ③ 上游没给 ⇒ 本侧生成并标记 root（**不**静默拿空串当 trace_id）
func TestAcceptGeneratesWhenAbsent(t *testing.T) {
	in := Accept("")
	if in.Upstream != UpstreamAbsent || !in.Root {
		t.Errorf("上游没给 ⇒ 应 absent + root=true，实际 upstream=%q root=%v", in.Upstream, in.Root)
	}
	if in.TraceErr != nil {
		t.Errorf("上游没给不是错误：%v", in.TraceErr)
	}
	if err := in.Trace.Validate(); err != nil {
		t.Fatalf("本侧生成必须是合法上下文（空串/全零都算没生成）：%v（%+v）", err, in.Trace)
	}
	if s, err := FormatTraceparent(in.Trace); err != nil || !strings.HasPrefix(s, "00-") {
		t.Fatalf("本侧生成必须能写成头：%q %v", s, err)
	}
	// 两次生成必须是两条不同的链（否则等于把空串换成了常量）
	other := Accept("")
	if other.Trace.TraceID == in.Trace.TraceID {
		t.Errorf("两次本侧生成撞了同一条 trace：%q", in.Trace.TraceID)
	}
}

// ③-补 上游给了合法值 ⇒ 沿用（root=false，trace-id 一字不改）
func TestAcceptAdoptsUpstream(t *testing.T) {
	in := Accept(w3cSample)
	if in.Upstream != UpstreamPresent || in.Root {
		t.Errorf("上游给了合法值 ⇒ present + root=false，实际 %q/%v", in.Upstream, in.Root)
	}
	if in.Trace.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("必须沿用上游 trace-id，实际 %q", in.Trace.TraceID)
	}
	if in.Raw != w3cSample || in.TraceErr != nil {
		t.Errorf("原文/原因字段不符：raw=%q err=%v", in.Raw, in.TraceErr)
	}
	// 请求头入口同样口径
	h := http.Header{HeaderTraceparent: []string{w3cSample}}
	if got := AcceptRequest(h); got.Trace.TraceID != in.Trace.TraceID || got.Root {
		t.Errorf("AcceptRequest 口径与 Accept 不一致：%+v", got)
	}
}

// ③-补 上游给了非法值 ⇒ 拒绝（不沿用）+ 本侧生成 + 原因可读
func TestAcceptRejectsUpstreamInvalid(t *testing.T) {
	in := Accept("00-ABC-def-01")
	if in.Upstream != UpstreamInvalid || !in.Root {
		t.Errorf("非法上游 ⇒ invalid + root=true，实际 %q/%v", in.Upstream, in.Root)
	}
	if in.TraceErr == nil {
		t.Error("非法上游必须留下原因（否则运维只能看到“凭空换了条链”）")
	}
	if err := in.Trace.Validate(); err != nil {
		t.Errorf("拒绝后必须给出合法的新上下文：%v", err)
	}
	if strings.Contains(in.Trace.TraceID, "ABC") {
		t.Errorf("拒绝的含义就是不沿用：%+v", in.Trace)
	}
}

// ④ 出站注头：上游有 ⇒ 同一条 trace、本跳新 span；上游无但会话已绑定 ⇒ 仍同一条 trace
func TestSetOutboundKeepsOneTrace(t *testing.T) {
	// ④-1 上游有：trace-id 必须一模一样，span-id 必须换新（每跳一个 span）
	src := http.Header{HeaderTraceparent: []string{w3cSample}}
	dst := http.Header{}
	out := SetOutbound(dst, src, "sess-out-a", false)
	if out.Err != nil || out.Upstream != UpstreamPresent || out.Root {
		t.Fatalf("上游有时出站判定不符：err=%v upstream=%q root=%v", out.Err, out.Upstream, out.Root)
	}
	got := dst.Get(HeaderTraceparent)
	parsed, err := ParseTraceparent(got)
	if err != nil {
		t.Fatalf("写进请求头的 traceparent 不合法：%q %v", got, err)
	}
	if parsed.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("出站必须沿用上游 trace-id：%q", parsed.TraceID)
	}
	if parsed.SpanID == "00f067aa0ba902b7" {
		t.Errorf("出站必须新开本跳 span（不能照抄上游 span）：%q", parsed.SpanID)
	}
	if bg := dst.Get(HeaderBaggage); bg != "zerg.session=sess-out-a" {
		t.Errorf("baggage 应带会话标记，实际 %q", bg)
	}

	// ④-2 上游无、会话已绑定（入站 Accept 过）⇒ 仍必须落在同一条 trace 上
	accepted := Accept("")
	if !accepted.Root {
		t.Fatal("空上游应生成本侧 root")
	}
	if !BindSession("sess-out-b", accepted.Trace) {
		t.Fatal("绑定会话失败")
	}
	dst2 := http.Header{}
	out2 := SetOutbound(dst2, http.Header{}, "sess-out-b", true)
	if out2.Root {
		t.Error("会话已绑定 ⇒ 出站不该另起一条 trace")
	}
	p2, err := ParseTraceparent(dst2.Get(HeaderTraceparent))
	if err != nil {
		t.Fatalf("非法头：%v", err)
	}
	if p2.TraceID != accepted.Trace.TraceID {
		t.Errorf("出站与入站被劈成两条 trace：出站 %q vs 入站 %q", p2.TraceID, accepted.Trace.TraceID)
	}
	if bg := dst2.Get(HeaderBaggage); bg != "zerg.session=sess-out-b,zerg.replay=1" {
		t.Errorf("baggage 应含会话 + 回放标记，实际 %q", bg)
	}
	if !out2.Baggage.Replay() {
		t.Error("回放标记未生效")
	}

	// ④-3 上游无、会话也没绑 ⇒ 本侧新生成 root（root=true 如实标记）
	dst3 := http.Header{}
	out3 := SetOutbound(dst3, http.Header{}, "sess-out-c", false)
	if !out3.Root || out3.Upstream != UpstreamAbsent {
		t.Errorf("都没有 ⇒ 应 absent + root=true，实际 %q/%v", out3.Upstream, out3.Root)
	}
	if _, err := ParseTraceparent(dst3.Get(HeaderTraceparent)); err != nil {
		t.Errorf("新生成的头必须合法：%v", err)
	}
	if dst3.Get(HeaderBaggage) != "zerg.session=sess-out-c" {
		t.Errorf("baggage 应带会话标记，实际 %q", dst3.Get(HeaderBaggage))
	}
}

// ④-补 会话 ID 不是合法 baggage 值 ⇒ 只丢 baggage + 记账，traceparent 照写（不互相拖累）
func TestSetOutboundDropsBadBaggageOnly(t *testing.T) {
	dst := http.Header{}
	out := SetOutbound(dst, http.Header{}, "bad session id", false)
	if out.BaggageErr == nil {
		t.Error("非法会话 id 应记 baggage 失败原因（不静默）")
	}
	if dst.Get(HeaderBaggage) != "" {
		t.Errorf("非法 baggage 一个字节都不该写：%q", dst.Get(HeaderBaggage))
	}
	if _, err := ParseTraceparent(dst.Get(HeaderTraceparent)); err != nil {
		t.Errorf("baggage 失败不得连带拖累 traceparent：%v", err)
	}
}

// ④-补 上游 baggage 必须被继承（子端拿得到会话/回放），我们自己的键覆盖同名成员
func TestSetOutboundInheritsUpstreamBaggage(t *testing.T) {
	src := http.Header{HeaderBaggage: []string{"zerg.tenant=t1,zerg.session=upstream-sess"}}
	dst := http.Header{}
	out := SetOutbound(dst, src, "ours", true)
	if got := dst.Get(HeaderBaggage); got != "zerg.tenant=t1,zerg.session=ours,zerg.replay=1" {
		t.Errorf("继承+覆盖口径不符：%q", got)
	}
	if out.Baggage.Get("zerg.tenant") != "t1" {
		t.Errorf("上游成员丢了：%+v", out.Baggage.Pairs)
	}
}

// ④-补 Propagate 是"只写不报"的便捷口：nil 头不得 panic
func TestPropagateNilSafe(t *testing.T) {
	Propagate(nil, nil, "s", false) // 不得 panic
	h := http.Header{}
	Propagate(h, nil, "", false)
	if _, err := ParseTraceparent(h.Get(HeaderTraceparent)); err != nil {
		t.Errorf("空会话也要有合法 traceparent：%v", err)
	}
	if h.Get(HeaderBaggage) != "" {
		t.Errorf("空会话不写会话成员：%q", h.Get(HeaderBaggage))
	}
}

// ⑤ baggage 往返一致 + 非法拒绝（第二载体的完整口径）
func TestBaggageRoundTripAndRejects(t *testing.T) {
	cases := []string{
		"",
		"zerg.session=s1",
		"zerg.session=s1,zerg.replay=1",
		"zerg.tenant=t1,zerg.session=s1,zerg.replay=1",
		"a=",       // 空值是合法的
		"sess=1:2", // ':' 在值里允许
	}
	for _, s := range cases {
		bg, err := ParseBaggage(s)
		if err != nil {
			t.Fatalf("合法 baggage 被拒：%q → %v", s, err)
		}
		back, err := FormatBaggage(bg)
		if err != nil {
			t.Fatalf("回写失败：%q → %v", s, err)
		}
		if back != s {
			t.Errorf("往返不一致：%q → %q", s, back)
		}
		again, err := ParseBaggage(back)
		if err != nil || !again.Equal(bg) {
			t.Errorf("parse(format(x)) != x：%q → %+v → %v / %+v", s, bg, err, again)
		}
	}
	// 允许成员两侧 OWS（W3C 语法），但规范形不带空格 ⇒ 往返仍逐字节一致
	bg, err := ParseBaggage("zerg.session=a , zerg.replay=1")
	if err != nil {
		t.Fatalf("带 OWS 的合法 baggage 被拒：%v", err)
	}
	if s, _ := FormatBaggage(bg); s != "zerg.session=a,zerg.replay=1" {
		t.Errorf("规范形不该带空格：%q", s)
	}

	rejects := []struct {
		name string
		in   string
		want error
	}{
		{"空成员", "a=b,,c=d", ErrBaggageEmptyMember},
		{"尾随逗号", "a=b,", ErrBaggageEmptyMember},
		{"成员缺等号", "zerg.session", ErrBaggageShape},
		{"键含空格", "a b=c", ErrBaggageKey},
		{"键为空", "=v", ErrBaggageKey},
		{"值含空格", "a=b c", ErrBaggageValue},
		{"值含等号", "a=b=c", ErrBaggageValue},
		{"超长", strings.Repeat("k=v,", MaxBaggageBytes/4+1), ErrBaggageTooLong},
	}
	for _, c := range rejects {
		b, err := ParseBaggage(c.in)
		if err == nil {
			t.Errorf("【%s】非法 baggage 被接受：%q → %+v", c.name, c.in, b)
			continue
		}
		if !errors.Is(err, c.want) {
			t.Errorf("【%s】原因不对：%v（应 %v）", c.name, err, c.want)
		}
	}
	// 值里带分隔符/空格只有**生成侧**能触发（解析侧会先被逗号切成两个成员）⇒ 生成侧也必须拒
	fmtRejects := []struct {
		name string
		in   Baggage
		want error
	}{
		{"值含逗号", Baggage{Pairs: []Pair{{Key: "a", Value: "b,c"}}}, ErrBaggageValue},
		{"值含空格", Baggage{Pairs: []Pair{{Key: "a", Value: "b c"}}}, ErrBaggageValue},
		{"值含分号", Baggage{Pairs: []Pair{{Key: "a", Value: "b;c"}}}, ErrBaggageValue},
		{"键含空格", Baggage{Pairs: []Pair{{Key: "a b", Value: "c"}}}, ErrBaggageKey},
		{"键为空", Baggage{Pairs: []Pair{{Key: "", Value: "c"}}}, ErrBaggageKey},
		{"超长", Baggage{Pairs: []Pair{{Key: "k", Value: strings.Repeat("v", MaxBaggageBytes)}}}, ErrBaggageTooLong},
	}
	for _, c := range fmtRejects {
		s, err := FormatBaggage(c.in)
		if err == nil || s != "" {
			t.Errorf("【%s】非法成员必须报错且不产出字符串：%q %v", c.name, s, err)
			continue
		}
		if !errors.Is(err, c.want) {
			t.Errorf("【%s】原因不对：%v（应 %v）", c.name, err, c.want)
		}
	}
}

// ⑤-补 会话绑定表：非法不绑、覆盖以后到者为准、取值刷新 LRU
func TestSessionBinding(t *testing.T) {
	resetSessionBindings()
	if _, ok := SessionTrace("nobody"); ok {
		t.Error("没绑过就该 ok=false（调用方据此生成 root）")
	}
	if BindSession("", TraceContext{TraceID: strings.Repeat("a", 32), SpanID: strings.Repeat("b", 16)}) {
		t.Error("空会话不该绑")
	}
	if BindSession("sess-bind", TraceContext{TraceID: "XYZ", SpanID: "abc"}) {
		t.Error("非法上下文不该绑（绑上去就等于把垃圾传播出去）")
	}
	first := mustParse(t, w3cSample)
	if !BindSession("sess-bind", first) {
		t.Fatal("合法绑定失败")
	}
	got, ok := SessionTrace("sess-bind")
	if !ok || got != first {
		t.Errorf("取回不符：%+v / %v", got, ok)
	}
	second, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	BindSession("sess-bind", second)
	got2, _ := SessionTrace("sess-bind")
	if got2 != second {
		t.Errorf("覆盖语义应为“后到者为准”（否则本次入站/出站会自相矛盾）：%+v", got2)
	}
}
