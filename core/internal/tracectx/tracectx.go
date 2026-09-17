// Package tracectx —— T1.6 传播载体：W3C Trace Context（traceparent）+ baggage 的**纯函数**实现。
//
// 依据：设计稿 v1.2 §〇 B16「跨进程/跨节点（LLM worker、工具子进程、多机）必须能拼上」；任务表 T1.6。
// 定位：**叶子包** —— 只 import 标准库，零内部依赖。gateway 与 chat 之间刻意不互相 import
// （见 internal/gateway/obs_failover.go 文件头），传播载体必须落在两边都能 import 的叶子上，否则成环。
//
// 三条口径（改这里先读它）：
//  1. **往返一致**：Parse(Format(x)) == x；Format 只产出规范形（四段、全小写十六进制、无空白），
//     Parse 只接受规范形 —— 不做"尽力而为"的修复（修复会把坏值悄悄变成看着合法的另一个值）。
//  2. **非法输入一律拒绝**（不是宽容解析）：段数 / 版本 / 长度 / 大小写 / 全零 id / flags 位数，
//     任一项不符即报错（哨兵 err，用 errors.Is 判定）。调用方**不得**把报错读成"拿到空 trace_id"。
//  3. **绝不静默拿空串当 trace_id**：上游没给 ⇒ 本侧生成 root（Root=true 如实标记）；
//     上游给了但非法 ⇒ 拒绝 + 重新生成（TraceErr 带原因，调用方落事件）；随机源不可用 ⇒ 返回 Err，
//     宁可不写头，也不用空串/全零顶替。
//
// 线宽口径（重要，勿混）：trace-id 线上 16 字节 / 32 位十六进制，与本仓 T1.1 的内部 trace_id 同为 32 位
// ⇒ 两侧可直接对齐（这就是"跨进程能拼上"的钥匙）。span-id 线上是 **8 字节 / 16 位**，而本仓内部
// span_id 是 32 位：两者**不是**同一个 id 空间，禁止互相截断/填充来"凑一个"（截断可能撞成全零、
// 填充则是编造）。因此传播事件里凡来自线上的 span 一律注明是 wire span（见 gateway 侧事件字段）。
package tracectx

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// 头名（W3C Trace Context L2 / W3C Baggage）
const (
	HeaderTraceparent = "traceparent"
	HeaderBaggage     = "baggage"
)

// 字段宽度（W3C：version 2 位十六进制、trace-id 32 位、span-id 16 位、trace-flags 2 位）
const (
	TraceIDHexLen = 32
	SpanIDHexLen  = 16
	FlagsHexLen   = 2

	version00 = "00"
)

// FlagSampled —— trace-flags 的采样位（bit0）。W3C L2：版本 00 只有这一位有定义，
// 因此其余位一律照原样透传、不解释（我们首版全量落盘 ⇒ 自己生成时置 1）。
const FlagSampled byte = 0x01

// 上游三态（落事件用；**不要**用空串表达"没给"——空串与"给了个空值"必须可分）
const (
	UpstreamPresent = "present" // 上游给了合法 traceparent ⇒ 沿用它的 trace-id
	UpstreamAbsent  = "absent"  // 上游没给 ⇒ 本侧新生成 root
	UpstreamInvalid = "invalid" // 上游给了但非法 ⇒ 拒绝（不沿用），本侧新生成
)

// 拒绝原因（哨兵错误；调用方用 errors.Is 判定，不要比字符串）
var (
	ErrEmpty   = errors.New("tracectx: traceparent 为空")
	ErrShape   = errors.New("tracectx: traceparent 段数不是 4")
	ErrVersion = errors.New("tracectx: traceparent 版本不是 00")
	ErrLength  = errors.New("tracectx: traceparent 字段长度不符")
	ErrNotHex  = errors.New("tracectx: traceparent 含非小写十六进制字符")
	ErrZeroID  = errors.New("tracectx: trace-id/span-id 全零（W3C 明确禁止）")
	ErrRandom  = errors.New("tracectx: 随机源不可用")
)

// TraceContext —— 一次传播的上下文（traceparent 四段的语义形态）。
type TraceContext struct {
	TraceID string // 32 位小写十六进制、非全零
	SpanID  string // 16 位小写十六进制、非全零
	Flags   byte   // trace-flags（bit0=sampled）
}

// Sampled —— 采样位（bit0）。
func (t TraceContext) Sampled() bool { return t.Flags&FlagSampled != 0 }

// IsZero —— 未拿到上下文（零值）。调用方据此走"没拿到 ⇒ 不写"这条路，不得拿零值当 id 用。
func (t TraceContext) IsZero() bool { return t.TraceID == "" && t.SpanID == "" }

// Validate —— 逐字段校验（拒绝而非修复）。返回的 err 必是上面某个哨兵（errors.Is 可判）。
func (t TraceContext) Validate() error {
	if err := checkLowerHex(t.TraceID, TraceIDHexLen, "trace-id"); err != nil {
		return err
	}
	return checkLowerHex(t.SpanID, SpanIDHexLen, "span-id")
}

// String —— 规范形；**仅供日志/调试**。拼请求头请用 Format（它会把错误交回来——
// String 拿不到错误，用它拼头就等于把"可能写出坏值"藏起来）。
func (t TraceContext) String() string {
	s, err := FormatTraceparent(t)
	if err != nil {
		return "<非法 tracectx>"
	}
	return s
}

// FormatTraceparent —— **生成**纯函数：TraceContext ⇒ 规范形 `00-<32hex>-<16hex>-<2hex>`。
// 非法上下文一律报错（绝不"顺手修一下"）。
func FormatTraceparent(t TraceContext) (string, error) {
	if err := t.Validate(); err != nil {
		return "", err
	}
	// flags 恒为 2 位小写十六进制（byte ⇒ 0x00–0xff，%02x 正好 2 位）⇒ 规范形可逐字节复原
	return version00 + "-" + t.TraceID + "-" + t.SpanID + "-" + fmt.Sprintf("%02x", t.Flags), nil
}

// ParseTraceparent —— **解析**纯函数：规范形 ⇒ TraceContext。非法输入一律拒绝。
//
// 拒绝清单（每条都有对应用例）：空串/仅空白、段数 != 4、版本 != 00、trace-id 长度 != 32、
// span-id 长度 != 16、flags 长度 != 2、含非小写十六进制（大写也算非法——W3C 要求小写）、
// trace-id 或 span-id 全零。
func ParseTraceparent(s string) (TraceContext, error) {
	if strings.TrimSpace(s) == "" {
		return TraceContext{}, fmt.Errorf("%w：%q", ErrEmpty, s)
	}
	parts := strings.Split(s, "-")
	if len(parts) != 4 {
		return TraceContext{}, fmt.Errorf("%w：得到 %d 段（%q）", ErrShape, len(parts), s)
	}
	if parts[0] != version00 {
		return TraceContext{}, fmt.Errorf("%w：%q（只认 00）", ErrVersion, parts[0])
	}
	if err := checkLowerHex(parts[1], TraceIDHexLen, "trace-id"); err != nil {
		return TraceContext{}, err
	}
	if err := checkLowerHex(parts[2], SpanIDHexLen, "span-id"); err != nil {
		return TraceContext{}, err
	}
	f, err := parseFlags(parts[3])
	if err != nil {
		return TraceContext{}, err
	}
	return TraceContext{TraceID: parts[1], SpanID: parts[2], Flags: f}, nil
}

// parseFlags —— flags 段：必须恰好 2 位小写十六进制。
func parseFlags(s string) (byte, error) {
	if len(s) != FlagsHexLen {
		return 0, fmt.Errorf("%w：flags 应 %d 位，实际 %d 位（%q）", ErrLength, FlagsHexLen, len(s), s)
	}
	if !isLowerHexStr(s) {
		return 0, fmt.Errorf("%w：flags=%q", ErrNotHex, s)
	}
	v, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		return 0, fmt.Errorf("%w：flags=%q（%v）", ErrNotHex, s, err)
	}
	return byte(v), nil
}

// checkLowerHex —— 长度 + 小写十六进制 + 非全零（W3C：全零 id 非法）。
func checkLowerHex(s string, n int, what string) error {
	if len(s) != n {
		return fmt.Errorf("%w：%s 应 %d 位，实际 %d 位（%q）", ErrLength, what, n, len(s), s)
	}
	if !isLowerHexStr(s) {
		return fmt.Errorf("%w：%s=%q", ErrNotHex, what, s)
	}
	if allZero(s) {
		return fmt.Errorf("%w：%s=%q", ErrZeroID, what, s)
	}
	return nil
}

func isLowerHexStr(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isLowerHexByte(s[i]) {
			return false
		}
	}
	return true
}

func isLowerHexByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
}

func allZero(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '0' {
			return false
		}
	}
	return true
}

// newID —— crypto/rand ⇒ n 字节小写十六进制。失败返回错误：**不**退化到伪随机、更**不**用空串顶替。
func newID(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("%w：%v", ErrRandom, err)
	}
	return hex.EncodeToString(b), nil
}

// Root —— **本侧新生成**一条链路根（上游未提供 traceparent 时走这条；采样位默认置 1）。
func Root() (TraceContext, error) {
	tid, err := newID(TraceIDHexLen / 2)
	if err != nil {
		return TraceContext{}, err
	}
	sid, err := newID(SpanIDHexLen / 2)
	if err != nil {
		return TraceContext{}, err
	}
	t := TraceContext{TraceID: tid, SpanID: sid, Flags: FlagSampled}
	if err := t.Validate(); err != nil {
		// 只可能因随机源给出全零（概率 ~0）⇒ 老实报错，不重试掩盖、更不编造。
		return TraceContext{}, err
	}
	return t, nil
}

// Child —— 同一条 trace 上开一个新 span（**每出一个跳**用）：trace-id 与 flags 不变、span-id 新生成。
func (t TraceContext) Child() (TraceContext, error) {
	if err := t.Validate(); err != nil {
		return TraceContext{}, err
	}
	sid, err := newID(SpanIDHexLen / 2)
	if err != nil {
		return TraceContext{}, err
	}
	return TraceContext{TraceID: t.TraceID, SpanID: sid, Flags: t.Flags}, nil
}

// ── 入站 ──

// Inbound —— 入站判定结果（有则沿用、无则本侧生成；非法**拒绝**后同样走生成）。
type Inbound struct {
	Trace      TraceContext // 采纳的上下文（生成失败时为零值 ⇒ 调用方必须当"没拿到"处理）
	Baggage    Baggage      // 上游 baggage（解析失败 ⇒ 空 + BaggageErr）
	Root       bool         // true = 本侧新生成（上游没给，或给了但被拒）
	Raw        string       // 上游 traceparent 原文（空 = 没给）
	Upstream   string       // present | absent | invalid
	TraceErr   error        // 非 nil = 上游给了但非法（原因）/ 本侧生成失败
	BaggageErr error        // 非 nil = 上游 baggage 非法（已丢弃，不影响 trace 采纳）
}

// Accept —— 入站**纯函数**：吃上游 traceparent 原文 ⇒ 有则沿用，无则本侧新生成（标记 root）。
// 非法上游值：拒绝（不沿用）+ 本侧生成，原因留在 TraceErr —— 绝不"宽容地"拿它当 trace 用。
func Accept(raw string) Inbound {
	in := Inbound{Raw: raw}
	switch tc, err := ParseTraceparent(raw); {
	case err == nil:
		in.Trace, in.Upstream = tc, UpstreamPresent
		return in
	case strings.TrimSpace(raw) != "":
		in.Upstream, in.TraceErr = UpstreamInvalid, err
	default:
		in.Upstream = UpstreamAbsent
	}
	tc, err := Root()
	if err != nil {
		// 生成失败：Root 保持 false、Trace 为零值 —— 调用方据此不落传播事件（不写空 trace_id）。
		in.TraceErr = err
		return in
	}
	in.Trace, in.Root = tc, true
	return in
}

// AcceptRequest —— 入站：从请求头读 traceparent（+baggage）。
// 无 traceparent ⇒ 本侧生成 root（绝不用空串顶替）；有但非法 ⇒ 拒绝 + 生成（TraceErr 说明原因）。
// 只读头，不写任何东西 ⇒ 可作为中间件/处理函数第一步调用。
func AcceptRequest(h http.Header) Inbound {
	if h == nil {
		return Accept("")
	}
	in := Accept(headerGet(h, HeaderTraceparent))
	rawBG := headerGet(h, HeaderBaggage)
	if strings.TrimSpace(rawBG) == "" {
		return in
	}
	bg, err := ParseBaggage(rawBG)
	if err != nil {
		in.BaggageErr = err
		return in
	}
	in.Baggage = bg
	return in
}

// headerGet —— 大小写不敏感地取一个请求头。
//
// 为什么不能只用 http.Header.Get：Get 是按**规范化键**查找（查 "Traceparent"），
// 而由非 Go 侧拼进来的小写键（traceparent）在 map 里就是字面小写 ⇒ Get 读不到 ⇒ 我们会"以为上游没给"
// 而另起一条 trace —— 一次静默失链（正是本任务要防的失败模式）。Go 服务器解析请求时会规范化键，
// 所以两条路都得通：先走标准 Get，未命中再逐键等值比较。
func headerGet(h http.Header, name string) string {
	if h == nil {
		return ""
	}
	if v := h.Get(name); v != "" {
		return v
	}
	for k, vs := range h {
		if len(vs) > 0 && strings.EqualFold(k, name) {
			return vs[0]
		}
	}
	return ""
}

// HeaderValue —— headerGet 的导出形态（供各调用点读传播头用；理由同上：不宽容会静默失链）。
func HeaderValue(h http.Header, name string) string { return headerGet(h, name) }

// ── 出站 ──

// Outbound —— 出站注头结果（头的原文 + 判定，供调用方落事件用）。
type Outbound struct {
	Trace         TraceContext
	Baggage       Baggage
	Root          bool   // true = 本侧新生成 root（上游与本会话都没有可用的 trace）
	Upstream      string // present | absent | invalid
	ParentSpanID  string // 本跳的父 span（wire 域 16 位）：上游头里的 span-id，或本会话已绑定的入站跳 span
	ParentSource  string // header | session | ""（无父 ⇒ 本跳是根）
	Traceparent   string // 实际写进请求头的 traceparent 原文（空 = 没写）
	BaggageHeader string // 实际写进请求头的 baggage 原文（空 = 没写）
	Err           error  // 非 nil ⇒ **一个字都没写**（拿不到随机源 / 会话 id 不是合法 baggage 值等）
	BaggageErr    error  // 非 nil ⇒ 只有 baggage 没写（traceparent 照写）——不静默
}

// 父 span 的来源（落事件用；"父是谁"必须可查，否则父子对不上账）
const (
	ParentFromHeader  = "header"  // 来自上游请求头（真·上游 span）
	ParentFromSession = "session" // 来自本会话已绑定的入站跳 span（同一次调用的上一跳）
)

// SetOutbound —— 出站**唯一入口**：先定 trace（①上游原文 ②本会话已绑定的上游 trace ③本侧新生成），
// 再把 traceparent / baggage 写进 dst。src 通常是被转发的入站请求头（保留上游链路）；可为 nil。
//
// 语义（写死）：
//   - 上游给了合法 traceparent ⇒ 沿用其 trace-id，本跳新开 span（Child）——不新造一条 trace。
//   - 上游没给/非法，但本会话已绑定（入站 Accept 过）⇒ 同样沿用该 trace、本跳新开 span。
//   - 都没有 ⇒ 本侧新生成 root（Root=true 如实标记）。
//   - 生成失败 ⇒ 一个字都不写（宁缺勿假）；会话 id 不是合法 baggage 值 ⇒ 只丢 baggage（BaggageErr 记因），
//     traceparent 照写 —— 因为"链路能不能拼上"比"会话标记"更关键，两者不互相拖累。
func SetOutbound(dst, src http.Header, session string, replay bool) Outbound {
	out := Outbound{}
	raw := ""
	if src != nil {
		raw = headerGet(src, HeaderTraceparent)
	}
	var base TraceContext
	switch tc, err := ParseTraceparent(raw); {
	case err == nil:
		base, out.Upstream, out.ParentSpanID = tc, UpstreamPresent, tc.SpanID
		out.ParentSource = ParentFromHeader
	case strings.TrimSpace(raw) != "":
		out.Upstream, out.Err = UpstreamInvalid, err
	default:
		out.Upstream = UpstreamAbsent
	}
	if out.Upstream != UpstreamPresent && session != "" {
		if tc, ok := SessionTrace(session); ok {
			// 本会话入站已沿用上游 trace ⇒ 出站必须仍在这条链上（否则一次调用会被劈成两条 trace）。
			base, out.Upstream = tc, UpstreamPresent
			out.ParentSpanID, out.ParentSource = tc.SpanID, ParentFromSession
		}
	}
	if out.Upstream == UpstreamPresent {
		hop, err := base.Child()
		if err != nil {
			out.Err = err
			return out
		}
		out.Trace = hop
	} else {
		root, err := Root()
		if err != nil {
			out.Err = err
			return out
		}
		out.Trace, out.Root = root, true
	}

	tp, err := FormatTraceparent(out.Trace)
	if err != nil {
		out.Err = err
		return out
	}

	// baggage：继承上游成员 → 覆盖/追加我们自己的键（会话 + 回放标记）
	bg := Baggage{}
	if src != nil {
		if up, perr := ParseBaggage(headerGet(src, HeaderBaggage)); perr == nil {
			bg = up
		}
	}
	if session != "" {
		bg = bg.Set(KeySession, session)
	}
	if replay {
		bg = bg.Set(KeyReplay, "1")
	}
	bgStr, berr := FormatBaggage(bg)
	if berr != nil {
		bgStr = "" // 会话 id 等不是合法 baggage 值 ⇒ 丢 baggage（记账），不写坏头
		out.BaggageErr = berr
	}

	if dst != nil {
		dst.Set(HeaderTraceparent, tp)
		if bgStr != "" {
			dst.Set(HeaderBaggage, bgStr)
		}
	}
	out.Traceparent, out.BaggageHeader, out.Baggage = tp, bgStr, bg
	return out
}

// Propagate —— 出站**便捷入口**（控制面调用点用：/load、/unload、/status、/pin…）：
// 只把两个头写进请求，不关心结果。拿不到随机源 ⇒ 不写（请求照发——观测/传播失败绝不拦业务）。
func Propagate(dst, src http.Header, session string, replay bool) {
	_ = SetOutbound(dst, src, session, replay)
}
