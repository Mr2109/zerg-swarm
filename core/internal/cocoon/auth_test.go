// auth_test.go — 令牌层 ＋ 身份闸的判据格（v3 §A）。
//
//	① TestIssueThenVerify              签发 → 校验过；形状/熵/epoch/TTL 都对
//	② TestVerifyRejectsBadPresentation  错票 / 空票 / 别茧的票 / 未签发的茧 / 只对上前缀的票
//	③ TestReissueRevokesOldImmediately  重发后旧票**立刻**失效（不等 TTL），epoch 递增
//	④ TestRevokeThenReject             吊销后拒；重复吊销幂等；再签发 epoch 继续 +1
//	⑤ TestTokenExpiry                  过期即拒（含恰好到期那一刻）；非高权也拒（令牌是第一道）
//	⑥ TestWeakIdentityPrivilegedDenied  弱/无身份 + 高权 ⇒ ErrPrivilegeNotAllowed（fail-closed）
//	⑦ TestTokenNeverInLogStrings        脱敏形 `abcd…wxyz`；任何 fmt 面都不含完整值
//	⑧ TestManagerHoldsNoPlaintext       反射扫管理器全图：零处等于令牌本体（带回正控）
//	⑨ TestConcurrentIssueVerifyRevoke   并发签发/校验/吊销（-race 有牙）
//	⑩ TestIssueRejectsBadInput          空 id / TTL≤0 / 坏随机源 / nil 管理器 一律拒
package cocoon

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// ── 夹具 ──────────────────────────────────────────────────────────────

// fakeClock — 可控时钟（过期用例要确定性，✗ 不靠 sleep）。
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// newAuthTestManager — 注入假时钟的管理器（ttl 由调用方给）。
func newAuthTestManager(t *testing.T, ttl time.Duration) (*TokenManager, *fakeClock) {
	t.Helper()
	m, err := NewTokenManagerWithTTL(ttl)
	if err != nil {
		t.Fatalf("NewTokenManagerWithTTL(%v) 失败: %v", ttl, err)
	}
	clk := &fakeClock{t: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
	m.now = clk.Now
	return m, clk
}

// eventSink — 审计事件收集器（-race 下多 goroutine 共用）。
type eventSink struct {
	mu  sync.Mutex
	evs []AuthzEvent
}

func (s *eventSink) Hook(ev AuthzEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evs = append(s.evs, ev)
}

func (s *eventSink) Snapshot() []AuthzEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]AuthzEvent(nil), s.evs...)
}

// errReader — 随机源故障的注入件（永远读不出东西）。
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

// tokenShape — 32 字节 base64url（raw，无填充）的严格形状。
var tokenShape = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func strongPeer() PeerIdentityRecord {
	return PeerIdentityRecord{Kind: "linux-peerpidfd", ID: "pidfd:4242", StartTime: 771234, Confidence: PeerIdentityStrong}
}

func weakPeer() PeerIdentityRecord {
	return PeerIdentityRecord{Kind: "macos-vm", ID: "vm:1", StartTime: 0, Confidence: PeerIdentityWeak}
}

// ── ① 签发 → 校验 ────────────────────────────────────────────────────

func TestIssueThenVerify(t *testing.T) {
	m, clk := newAuthTestManager(t, DefaultTokenTTL)

	tok, err := m.IssueToken("c-0001")
	if err != nil {
		t.Fatalf("IssueToken 失败: %v", err)
	}

	// 形状：43 字符 raw base64url，解码回来必须正是 32 字节（= 256 bit 熵）。
	if !tokenShape.MatchString(tok.Value) {
		t.Fatalf("令牌形状不合（要 43 字符 base64url）：%q", tok.Redacted())
	}
	raw, err := base64.RawURLEncoding.DecodeString(tok.Value)
	if err != nil {
		t.Fatalf("令牌不是 raw base64url: %v", err)
	}
	if len(raw) != TokenBytes {
		t.Fatalf("令牌熵 %d 字节 ≠ 要的 %d", len(raw), TokenBytes)
	}

	// TTL / epoch。
	if want := clk.Now().Add(DefaultTokenTTL); !tok.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %s，期望 %s", tok.ExpiresAt, want)
	}
	if tok.Epoch != 1 {
		t.Fatalf("首发 epoch = %d，期望 1", tok.Epoch)
	}

	if !m.VerifyToken("c-0001", tok.Value) {
		t.Fatal("刚签发的令牌校验不过")
	}
	// 同一枚在同一 TTL 内可反复校验（判定不消耗，见 auth.go 头注）。
	if !m.VerifyToken("c-0001", tok.Value) {
		t.Fatal("第二次校验不过 —— 校验不该消耗令牌")
	}

	// 两茧两票，互不串台。
	other, err := m.IssueToken("c-0002")
	if err != nil {
		t.Fatalf("IssueToken(c-0002) 失败: %v", err)
	}
	if other.Value == tok.Value {
		t.Fatal("两个实例拿到了同一枚令牌（随机源没起作用）")
	}
	if !m.VerifyToken("c-0002", other.Value) {
		t.Fatal("c-0002 的令牌在自己名下校验不过")
	}
}

// ── ② 错票一律拒 ──────────────────────────────────────────────────────

func TestVerifyRejectsBadPresentation(t *testing.T) {
	m, _ := newAuthTestManager(t, DefaultTokenTTL)
	tok, err := m.IssueToken("c-0001")
	if err != nil {
		t.Fatalf("IssueToken 失败: %v", err)
	}
	other, err := m.IssueToken("c-0002")
	if err != nil {
		t.Fatalf("IssueToken(c-0002) 失败: %v", err)
	}

	// 三个只差一格的票（首 / 中 / 末）：防「比前缀 / 比后缀 / 比头几位」这类退化实现。
	// 常量时间比较的常见写错法就是只对上开头就放行 —— 单靠「末位改一格」抓不到它。
	first := alterAt(tok.Value, 0)
	middle := alterAt(tok.Value, len(tok.Value)/2)
	last := alterAt(tok.Value, len(tok.Value)-1)

	cases := []struct {
		name     string
		instID   string
		present  string
		wantPass bool
	}{
		{"正确票", "c-0001", tok.Value, true},
		{"空票", "c-0001", "", false},
		{"随机串", "c-0001", strings.Repeat("A", 43), false},
		{"别茧的票", "c-0001", other.Value, false},
		{"首位改一格", "c-0001", first, false},
		{"中段改一格", "c-0001", middle, false},
		{"末位改一格", "c-0001", last, false},
		{"未签发的茧", "c-9999", tok.Value, false},
		{"空实例 id", "", tok.Value, false},
	}
	for _, c := range cases {
		if got := m.VerifyToken(c.instID, c.present); got != c.wantPass {
			t.Errorf("%s: VerifyToken = %t，期望 %t", c.name, got, c.wantPass)
		}
	}
}

// alterAt — 把第 i 个 base64url 字符换成另一个（保证「只差一格」且不改变长度）。
func alterAt(s string, i int) string {
	b := []byte(s)
	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}
	return string(b)
}

// ── ③ 重发即吊销旧票 ──────────────────────────────────────────────────

func TestReissueRevokesOldImmediately(t *testing.T) {
	m, clk := newAuthTestManager(t, DefaultTokenTTL)

	first, err := m.IssueToken("c-0001")
	if err != nil {
		t.Fatalf("首次签发失败: %v", err)
	}
	if !m.VerifyToken("c-0001", first.Value) {
		t.Fatal("首票应当有效")
	}

	// 时间不动 ⇒ 旧票没过期，失效只能是「重发吊销」这一条因。
	second, err := m.IssueToken("c-0001")
	if err != nil {
		t.Fatalf("重发失败: %v", err)
	}
	if !m.nowFn().Equal(clk.Now()) {
		t.Fatal("夹具时钟漂了（本用例要求同一时刻）")
	}
	if second.Epoch != first.Epoch+1 {
		t.Fatalf("重发 epoch = %d，期望 %d", second.Epoch, first.Epoch+1)
	}
	if m.VerifyToken("c-0001", first.Value) {
		t.Fatal("重发后旧票仍然有效 —— 旧票必须立刻失效")
	}
	if !m.VerifyToken("c-0001", second.Value) {
		t.Fatal("新票无效")
	}

	// 旧票不能因「再发一次」复活。
	third, err := m.IssueToken("c-0001")
	if err != nil {
		t.Fatalf("第三次签发失败: %v", err)
	}
	if m.VerifyToken("c-0001", first.Value) || m.VerifyToken("c-0001", second.Value) {
		t.Fatal("第三次签发后仍有旧票有效（应只剩最新那一枚）")
	}
	if third.Epoch != 3 {
		t.Fatalf("第三次签发 epoch = %d，期望 3", third.Epoch)
	}
}

// ── ④ 吊销 ────────────────────────────────────────────────────────────

func TestRevokeThenReject(t *testing.T) {
	m, _ := newAuthTestManager(t, DefaultTokenTTL)
	tok, err := m.IssueToken("c-0001")
	if err != nil {
		t.Fatalf("IssueToken 失败: %v", err)
	}
	if !m.VerifyToken("c-0001", tok.Value) {
		t.Fatal("吊销前应当有效")
	}

	m.RevokeToken("c-0001")
	if m.VerifyToken("c-0001", tok.Value) {
		t.Fatal("吊销后仍然有效")
	}

	// 幂等：重复吊销不炸、也不再改变判定。
	m.RevokeToken("c-0001")
	m.RevokeToken("c-0001")
	if m.VerifyToken("c-0001", tok.Value) {
		t.Fatal("重复吊销后令牌复活了")
	}
	// 吊销未签发的实例：幂等空操作，不许 panic。
	m.RevokeToken("c-never-issued")

	// 撤销后的再签发：epoch 继续 +1（代数不回退），且旧票照样不复活。
	again, err := m.IssueToken("c-0001")
	if err != nil {
		t.Fatalf("吊销后再签发失败: %v", err)
	}
	if again.Epoch != tok.Epoch+1 {
		t.Fatalf("吊销后 epoch = %d，期望 %d", again.Epoch, tok.Epoch+1)
	}
	if m.VerifyToken("c-0001", tok.Value) {
		t.Fatal("吊销后再签发，旧票复活了")
	}
	if !m.VerifyToken("c-0001", again.Value) {
		t.Fatal("吊销后签发的新票无效")
	}

	// Info 面：已吊销记录仍在册（epoch 计数器），且只给脱敏形。
	info, ok := m.Info("c-0001")
	if !ok {
		t.Fatal("Info 说实例不在册（刚签发过）")
	}
	if info.Token != again.Redacted() || strings.Contains(info.Token, again.Value) {
		t.Fatalf("Info 的令牌形态不合格：%q", info.Token)
	}
}

// ── ⑤ 过期 ────────────────────────────────────────────────────────────

func TestTokenExpiry(t *testing.T) {
	ttl := 90 * time.Minute
	m, clk := newAuthTestManager(t, ttl)
	tok, err := m.IssueToken("c-0001")
	if err != nil {
		t.Fatalf("IssueToken 失败: %v", err)
	}

	// TTL 内（差 1ns 到期）：有效。
	clk.Advance(ttl - time.Nanosecond)
	if !m.VerifyToken("c-0001", tok.Value) {
		t.Fatal("TTL 内却判过期")
	}
	if tok.Expired(clk.Now()) {
		t.Fatal("Token.Expired 在 TTL 内返回 true")
	}

	// 恰好到期那一刻：归「过期」（fail-closed，边界不留给运气）。
	clk.Advance(time.Nanosecond)
	if !tok.Expired(clk.Now()) {
		t.Fatal("恰好到期却不算过期（边界要 fail-closed）")
	}
	if m.VerifyToken("c-0001", tok.Value) {
		t.Fatal("已过期令牌仍然有效")
	}
	if err := m.Authorize("c-0001", tok.Value, AuthzRequest{Action: "read"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("过期令牌的非高权请求 err = %v，期望 ErrUnauthorized", err)
	}

	// 过期记录可被清扫；清完 epoch 从 1 重来（实例 id 不复用 ⇒ 不产生歧义）。
	if n := m.PurgeExpired(); n != 1 {
		t.Fatalf("PurgeExpired 清了 %d 条，期望 1 条", n)
	}
	if _, ok := m.Info("c-0001"); ok {
		t.Fatal("清扫后实例仍在册")
	}
	fresh, err := m.IssueToken("c-0001")
	if err != nil {
		t.Fatalf("清扫后签发失败: %v", err)
	}
	if fresh.Epoch != 1 {
		t.Fatalf("清扫后 epoch = %d，期望 1（记录已清）", fresh.Epoch)
	}
}

// ── ⑥ 弱身份 + 高权 ⇒ 拒 ──────────────────────────────────────────────

func TestWeakIdentityPrivilegedDenied(t *testing.T) {
	m, _ := newAuthTestManager(t, DefaultTokenTTL)
	sink := &eventSink{}
	m.SetAuditHook(sink.Hook)

	tok, err := m.IssueToken("c-0001")
	if err != nil {
		t.Fatalf("IssueToken 失败: %v", err)
	}

	// 令牌有效 ＋ weak ＋ 高权 ⇒ 拒，且错误可 errors.Is 判。
	err = m.Authorize("c-0001", tok.Value, AuthzRequest{Action: "destroy", Privileged: true, Peer: weakPeer()})
	if !errors.Is(err, ErrPrivilegeNotAllowed) {
		t.Fatalf("weak+高权 err = %v，期望 ErrPrivilegeNotAllowed", err)
	}
	// 同一件事不许同时报成「令牌无效」——两类原因要能分开定位。
	if errors.Is(err, ErrUnauthorized) {
		t.Fatal("weak+高权被误报成令牌问题")
	}

	// none ＋ 高权 ⇒ 同样拒。
	none := PeerIdentityRecord{Kind: "", ID: "", StartTime: 0, Confidence: PeerIdentityNone}
	if err := m.Authorize("c-0001", tok.Value, AuthzRequest{Action: "destroy", Privileged: true, Peer: none}); !errors.Is(err, ErrPrivilegeNotAllowed) {
		t.Fatalf("none+高权 err = %v，期望 ErrPrivilegeNotAllowed", err)
	}

	// 认不出的强度 ⇒ 归 none ⇒ 高权照样拒（fail-closed，✗ 不把「认不出」当 strong）。
	bogus := PeerIdentityRecord{Kind: "who-knows", ID: "x", Confidence: PeerIdentity("plausible")}
	if err := m.Authorize("c-0001", tok.Value, AuthzRequest{Action: "destroy", Privileged: true, Peer: bogus}); !errors.Is(err, ErrPrivilegeNotAllowed) {
		t.Fatalf("未知强度+高权 err = %v，期望 ErrPrivilegeNotAllowed", err)
	}

	// 弱身份 ＋ 非高权 ⇒ 放行（授权只认令牌，身份只闸高权）。
	if err := m.Authorize("c-0001", tok.Value, AuthzRequest{Action: "read", Peer: weakPeer()}); err != nil {
		t.Fatalf("weak+非高权被拒: %v（v3 §A：授权只认令牌）", err)
	}

	// strong ＋ 高权 ⇒ 唯一放行的组合。
	if err := m.Authorize("c-0001", tok.Value, AuthzRequest{Action: "destroy", Privileged: true, Peer: strongPeer()}); err != nil {
		t.Fatalf("strong+高权被拒: %v", err)
	}

	// 令牌无效时，身份再强也不放行（先认证后授权）。
	if err := m.Authorize("c-0001", "not-the-token", AuthzRequest{Action: "destroy", Privileged: true, Peer: strongPeer()}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("错票+strong err = %v，期望 ErrUnauthorized", err)
	}

	// 审计面：四次拒 + 两次放行，全部记下，且 reason 能区分原因。
	evs := sink.Snapshot()
	if len(evs) != 6 {
		t.Fatalf("审计事件 %d 条，期望 6 条", len(evs))
	}
	denied := 0
	for _, ev := range evs {
		if !ev.Allowed {
			denied++
			if !strings.Contains(ev.Reason, "privileged_needs_strong") && !strings.Contains(ev.Reason, "mismatch") {
				t.Errorf("拒绝事件 reason = %q，认不出原因", ev.Reason)
			}
		}
	}
	if denied != 4 {
		t.Fatalf("拒绝事件 %d 条，期望 4 条", denied)
	}

	// 记录层面的同一判定（PrivilegedAllowed 与 Authorize 必须同口径）。
	if weakPeer().PrivilegedAllowed() || none.PrivilegedAllowed() || bogus.PrivilegedAllowed() {
		t.Fatal("PrivilegedAllowed 对 weak/none/未知返回了 true")
	}
	if !strongPeer().PrivilegedAllowed() {
		t.Fatal("PrivilegedAllowed 对 strong 返回了 false")
	}
	// 身份记录自检：未知强度必须 Validate 报错（而不是静默通过）。
	if err := bogus.Validate(); err == nil {
		t.Fatal("未知强度的 Validate 没报错")
	}
}

// ── ⑦ 令牌不进任何日志串 ──────────────────────────────────────────────

func TestTokenNeverInLogStrings(t *testing.T) {
	m, _ := newAuthTestManager(t, DefaultTokenTTL)
	sink := &eventSink{}
	m.SetAuditHook(sink.Hook)

	tok, err := m.IssueToken("c-0001")
	if err != nil {
		t.Fatalf("IssueToken 失败: %v", err)
	}
	bad := strings.Repeat("Z", 43)

	// 跑一遍全流程，把各类事件都凑齐（放行/弱身份拒/错票拒）。
	_ = m.Authorize("c-0001", tok.Value, AuthzRequest{Action: "read", Peer: weakPeer()})
	_ = m.Authorize("c-0001", tok.Value, AuthzRequest{Action: "destroy", Privileged: true, Peer: weakPeer()})
	_ = m.Authorize("c-0001", bad, AuthzRequest{Action: "destroy", Privileged: true, Peer: strongPeer()})

	// 脱敏形：`abcd…wxyz`（首 4 + 省略号 + 末 4），长度 9。
	red := tok.Redacted()
	wantRed := tok.Value[:4] + redactedEllipsis + tok.Value[len(tok.Value)-4:]
	if red != wantRed {
		t.Fatalf("Redacted = %q，期望 %q", red, wantRed)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{4}…[A-Za-z0-9_-]{4}$`).MatchString(red) {
		t.Fatalf("Redacted 形状不合 `abcd…wxyz`：%q", red)
	}
	if strings.Contains(red, tok.Value) || utf8.RuneCountInString(red) != 9 {
		t.Fatalf("Redacted 含完整值或长度不对：%q（%d rune）", red, utf8.RuneCountInString(red))
	}
	// 短值不许露首尾（≤ 8 字符露首尾等于露全部）。
	if got := RedactToken("abc"); got != redactedEllipsis {
		t.Fatalf("短值脱敏 = %q，期望只回省略号", got)
	}

	// 收集所有「会进日志」的串：Token / AuthzEvent / TokenInfo 的每个 fmt 面。
	var logs []string
	logs = append(logs, tok.Redacted())
	logs = append(logs,
		fmt.Sprintf("%v", tok), fmt.Sprintf("%+v", tok),
		fmt.Sprintf("%s", tok), fmt.Sprintf("%#v", tok),
		tok.String(), tok.GoString(),
	)
	info, ok := m.Info("c-0001")
	if !ok {
		t.Fatal("Info 说实例不在册")
	}
	logs = append(logs,
		fmt.Sprintf("%v", info), fmt.Sprintf("%+v", info),
		fmt.Sprintf("%s", info), fmt.Sprintf("%#v", info),
		info.String(), info.GoString(),
	)
	for _, ev := range sink.Snapshot() {
		logs = append(logs,
			fmt.Sprintf("%v", ev), fmt.Sprintf("%+v", ev),
			fmt.Sprintf("%s", ev), fmt.Sprintf("%#v", ev),
			ev.String(), ev.GoString(),
		)
	}

	if len(logs) < 20 {
		t.Fatalf("日志面样本只凑到 %d 条（下界 20）—— 样本不足，本用例的结论不成立", len(logs))
	}
	for i, s := range logs {
		if strings.Contains(s, tok.Value) {
			t.Fatalf("第 %d 条日志串含令牌本体：%q", i, s)
		}
		if strings.Contains(s, bad) {
			t.Fatalf("第 %d 条日志串含被拒的令牌本体：%q", i, s)
		}
	}
	// 正控：脱敏形确实出现了（否则「不含完整值」可能只是因为什么都没打）。
	hits := 0
	for _, s := range logs {
		if strings.Contains(s, red) {
			hits++
		}
	}
	if hits < 3 {
		t.Fatalf("脱敏形只出现在 %d 条日志串里（下界 3）—— 审计面没接上", hits)
	}
}

// ── ⑧ 管理器里没有令牌本体 ────────────────────────────────────────────

func TestManagerHoldsNoPlaintext(t *testing.T) {
	m, _ := newAuthTestManager(t, DefaultTokenTTL)
	tok, err := m.IssueToken("c-0001")
	if err != nil {
		t.Fatalf("IssueToken 失败: %v", err)
	}
	if _, err := m.IssueToken("c-0002"); err != nil {
		t.Fatalf("IssueToken(c-0002) 失败: %v", err)
	}

	// 扫管理器全图（含 map 里的 *tokenEntry），找等于令牌本体的字符串。
	// ★ 双向证：节点数有下界 ⇒ 真走到底了；找脱敏形必须正好中 2 处（两个实例）
	// ⇒ 扫描器不是空转的 0 命中。
	nRaw, nodes, strs := scanForRaw(t, m, tok.Value)
	if nodes < 20 || strs < 2 {
		t.Fatalf("扫描只走到 %d 个节点 / %d 个字符串（下界 20 / 2）—— 没走到底，结论不成立", nodes, strs)
	}
	if nRaw != 0 {
		t.Fatalf("管理器里 %d 处等于令牌本体（只许留摘要 + 脱敏形）", nRaw)
	}
	// 正控：同一个扫描器找脱敏形必须中 1 处（c-0001 在册那一枚；c-0002 是另一枚票）。
	nRed, _, _ := scanForRaw(t, m, tok.Redacted())
	if nRed != 1 {
		t.Fatalf("正控失败：扫描器找到脱敏形 %d 处，期望 1 处 ⇒ 扫描器坏了", nRed)
	}
	// 负控：令牌本体换个末位也不该命中（证明比较是真逐字比，不是前缀比）。
	nFake, _, _ := scanForRaw(t, m, alterAt(tok.Value, len(tok.Value)-1))
	if nFake != 0 {
		t.Fatalf("改了末位的假票命中了 %d 处（比较退化成前缀比）", nFake)
	}
}

// scanForRaw — 深度遍历任意值，统计「等于 raw 的字符串」个数、走过的节点数、见到的字符串数。
func scanForRaw(t *testing.T, root any, raw string) (matched, nodes, strs int) {
	t.Helper()
	seen := map[uintptr]bool{}
	var walk func(v reflect.Value, depth int)
	walk = func(v reflect.Value, depth int) {
		if depth > 12 || !v.IsValid() {
			return
		}
		nodes++
		switch v.Kind() {
		case reflect.String:
			strs++
			if v.String() == raw {
				matched++
			}
		case reflect.Ptr, reflect.Interface:
			if v.IsNil() {
				return
			}
			if v.Kind() == reflect.Ptr {
				p := v.Pointer()
				if seen[p] {
					return
				}
				seen[p] = true
			}
			walk(v.Elem(), depth+1)
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				walk(v.Field(i), depth+1)
			}
		case reflect.Map:
			iter := v.MapRange()
			for iter.Next() {
				walk(iter.Key(), depth+1)
				walk(iter.Value(), depth+1)
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i), depth+1)
			}
		}
	}
	walk(reflect.ValueOf(root), 0)
	return matched, nodes, strs
}

// ── ⑨ 并发 ────────────────────────────────────────────────────────────

func TestConcurrentIssueVerifyRevoke(t *testing.T) {
	m, _ := newAuthTestManager(t, DefaultTokenTTL)
	sink := &eventSink{}
	m.SetAuditHook(sink.Hook)

	const workers, rounds = 8, 40
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer wg.Done()
			inst := fmt.Sprintf("c-%04d", w%4)
			for i := 0; i < rounds; i++ {
				tok, err := m.IssueToken(inst)
				if err != nil {
					t.Errorf("worker %d IssueToken 失败: %v", w, err)
					return
				}
				_ = m.VerifyToken(inst, tok.Value)
				// 刚自己签的票至少在被别人覆盖之前要么有效、要么被覆盖 —— 两种都合法，
				// 这里只要求「不 panic、不出现第三种读数」。
				_ = m.Authorize(inst, tok.Value, AuthzRequest{Action: "read", Peer: weakPeer()})
				if i%7 == 0 {
					m.RevokeToken(inst)
				}
				_, _ = m.Info(inst)
			}
		}(w)
	}
	wg.Wait()

	for _, ev := range sink.Snapshot() {
		if strings.Contains(ev.String(), "…") && strings.Count(ev.Token, "…") > 1 {
			t.Fatalf("审计事件的令牌形态异常：%q", ev.Token)
		}
	}
	// 收尾：每个实例都能重新拿到一枚可用的票。
	for w := 0; w < 4; w++ {
		inst := fmt.Sprintf("c-%04d", w)
		tok, err := m.IssueToken(inst)
		if err != nil {
			t.Fatalf("收尾签发 %s 失败: %v", inst, err)
		}
		if !m.VerifyToken(inst, tok.Value) {
			t.Fatalf("收尾时 %s 的票无效", inst)
		}
	}
}

// ── ⑩ 输入闸 ──────────────────────────────────────────────────────────

func TestIssueRejectsBadInput(t *testing.T) {
	m, _ := newAuthTestManager(t, DefaultTokenTTL)

	for _, id := range []string{"", "   ", "\t\n"} {
		if _, err := m.IssueToken(id); !errors.Is(err, ErrEmptyInstanceID) {
			t.Errorf("IssueToken(%q) err = %v，期望 ErrEmptyInstanceID", id, err)
		}
		if m.VerifyToken(id, strings.Repeat("A", 43)) {
			t.Errorf("空实例 id %q 居然校验通过", id)
		}
	}

	// TTL ≤ 0 一律拒（没有「0 = 永久」这一档）。
	for _, ttl := range []time.Duration{0, -1 * time.Second} {
		if _, err := NewTokenManagerWithTTL(ttl); !errors.Is(err, ErrBadTokenTTL) {
			t.Errorf("NewTokenManagerWithTTL(%v) err = %v，期望 ErrBadTokenTTL", ttl, err)
		}
	}

	// 随机源坏 ⇒ 拒发（弱随机令牌等于没有令牌）。
	bad := NewTokenManager()
	bad.rand = errReader{}
	if _, err := bad.IssueToken("c-0001"); !errors.Is(err, ErrTokenRand) {
		t.Errorf("坏随机源 err = %v，期望 ErrTokenRand", err)
	}

	// nil 管理器：签发/判定一律 fail-closed。
	var nilM *TokenManager
	if _, err := nilM.IssueToken("c-0001"); !errors.Is(err, ErrTokenManagerNil) {
		t.Errorf("nil 管理器 IssueToken err = %v，期望 ErrTokenManagerNil", err)
	}
	if err := nilM.Authorize("c-0001", strings.Repeat("A", 43), AuthzRequest{Action: "read"}); !errors.Is(err, ErrTokenManagerNil) {
		t.Errorf("nil 管理器 Authorize err = %v，期望 ErrTokenManagerNil", err)
	}
	if nilM.VerifyToken("c-0001", strings.Repeat("A", 43)) {
		t.Error("nil 管理器的 VerifyToken 返回了 true")
	}
	nilM.RevokeToken("c-0001") // 幂等空操作，不许 panic
	if n := nilM.PurgeExpired(); n != 0 {
		t.Errorf("nil 管理器 PurgeExpired = %d，期望 0", n)
	}

	// 零值管理器（没走构造器）：ttl=0 ⇒ 拒发（fail-closed，✗ 不许静默当永久票）。
	var zero TokenManager
	if _, err := zero.IssueToken("c-0001"); !errors.Is(err, ErrBadTokenTTL) {
		t.Errorf("零值管理器 IssueToken err = %v，期望 ErrBadTokenTTL", err)
	}
}
