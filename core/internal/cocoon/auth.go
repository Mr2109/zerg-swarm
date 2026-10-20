// auth.go — 每茧一次性令牌 ＋ 对端身份强度闸（v3 §A 的**可执行形态**）
//
// ★ 三条不变量（测试必须逐条打，改这里先读 v3 §A）：
//
//	① **授权只认令牌**：32 字节密码学随机 → base64url（43 字符）。每茧**一票**，
//	   重发即吊销旧票（epoch 单调 +1）。身份缺失/偏弱**不**影响「非高权」请求的放行 ——
//	   这正是 §A「判据 7（冒用必拒）不依赖任何平台能力」的落点。
//	② **身份只做闸门**：{kind,id,start_time,confidence}，confidence 只有 strong 许高权；
//	   weak / none / 未知强度 ⇒ 高权一律拒（fail-closed，**✗ 不降权放行**）、只记审计。
//	③ **令牌不进日志**：管理器只存**摘要**（sha256）与**脱敏形**（`abcd…wxyz`），
//	   进程里根本没有第二份完整值可漏；Token / AuthzEvent / TokenInfo 的 fmt 面
//	   一律走脱敏形（`%v`/`%+v`/`%s`/`%#v` 全覆盖 —— 见各自 String/GoString）。
//
// ★ 「一次性」的口径（写死，免得后人读歪）：**每茧同时只存在一枚有效令牌**，
// 重发即吊销旧枚；**校验不消耗**令牌（同一枚在 TTL 内可反复校验）。
// 若做成「校验即焚」，第二个请求就得重新下发令牌 —— 与 §A「授权只认令牌」自相矛盾，
// 且会把令牌变成每次请求都要搬运的东西（暴露面反而变大）。
//
// ★ 本层是**进程内状态、不落盘**：控制面重启 ⇒ 全部令牌作废，客户端须从
// `<茧目录>/token`（0600，宿主侧）重取。作废落在安全侧，不是故障。
//
// ★ 名字说明：本包的 `PeerIdentity`（backend.go）已经是**强度枚举**（strong/weak/none），
// 故「身份记录」这个结构体叫 `PeerIdentityRecord`，其 Confidence 字段**复用**该枚举
// （底层 string，`Confidence: "weak"` 这类字面量写法照样成立）。
package cocoon

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const (
	// TokenBytes — 令牌熵（字节）。32 字节 = 256 bit，base64url 后 43 字符。
	TokenBytes = 32
	// DefaultTokenTTL — 缺省有效期（24h）。TTL 只挡「旧票长期有效」，不承担吊销职责。
	DefaultTokenTTL = 24 * time.Hour

	// redactedPrefixLen / redactedSuffixLen — 脱敏形保留的首尾各 4 字符（`abcd…wxyz`）。
	// 43 字符里露 8 个 ⇒ 仍有 ≥200 bit 未知，够「人眼对账」而不够「反推」。
	redactedPrefixLen = 4
	redactedSuffixLen = 4
)

// redactedEllipsis — 脱敏形的中间记号（单个省略号字符，不是三个点）。
const redactedEllipsis = "…"

var (
	// ErrUnauthorized — 令牌校验未过（未签发 / 不匹配 / 已吊销 / 已过期 / 未出示）。
	// ★ 对外**不区分**这四种原因（少给口子做判定 oracle）；原因只进审计事件。
	ErrUnauthorized = errors.New("cocoon: 令牌校验未过")
	// ErrPrivilegeNotAllowed — 身份强度不足，高权操作被拒（v3 §A：weak ⇒ fail-closed）。
	ErrPrivilegeNotAllowed = errors.New("cocoon: 身份强度不足，拒绝高权操作")
	// ErrEmptyInstanceID — 签发/校验时实例 id 为空（空 id 会让「每茧一票」退化成共用一票）。
	ErrEmptyInstanceID = errors.New("cocoon: 令牌实例 id 为空")
	// ErrBadTokenTTL — TTL 非正。**不许**用「0 = 永久」这种约定，永久票 = 吊销失效。
	ErrBadTokenTTL = errors.New("cocoon: 令牌 TTL 必须为正")
	// ErrTokenRand — 随机源读失败。此时**必须**拒发（弱随机令牌等于没有令牌）。
	ErrTokenRand = errors.New("cocoon: 令牌随机源不可用")
	// ErrTokenManagerNil — 管理器未初始化（零值 / nil 接收者）。fail-closed。
	ErrTokenManagerNil = errors.New("cocoon: 令牌管理器未初始化（用 NewTokenManager）")
)

// ── 令牌 ──────────────────────────────────────────────────────────────

// Token — 一枚签发出去的令牌。**只在签发那一刻**把 Value 交给调用方，
// 管理器自己不留第二份（只留摘要 + 脱敏形）。
//
// ★ 这个值**不要**直接进日志/错误串/审计 —— 它的 fmt 面已经是脱敏形，
// 但 `%#v` 之外的手段（自己拼字段、json 序列化）照样能漏，故仍按「凭据」对待。
type Token struct {
	// Value — 令牌本体（base64url，43 字符）。只在交给客户端时存在。
	Value string
	// ExpiresAt — 到期时刻（含：`!now.Before(ExpiresAt)` 即过期，边界归「过期」）。
	ExpiresAt time.Time
	// Epoch — 该实例的签发代数（首发 = 1，每次重发 +1）。审计靠它对齐「哪一票」。
	Epoch uint64
}

// Expired — 在 now 这一刻是否已过期（边界取闭区间：恰好到期即过期，fail-closed）。
func (t Token) Expired(now time.Time) bool { return !now.Before(t.ExpiresAt) }

// Redacted — 脱敏形：`abcd…wxyz`（首 4 + 省略号 + 末 4）。
// 太短的值（≤ 8 字符）一位都不露，只回省略号 —— 短值露首尾等于露全部。
func (t Token) Redacted() string { return RedactToken(t.Value) }

// String — fmt 面默认脱敏（`%v` / `%+v` / `%s`）⇒ 手滑 `log.Printf("%v", tok)` 不漏值。
func (t Token) String() string { return t.Redacted() }

// GoString — `%#v` 也脱敏（否则 `%#v` 会把 Value 逐字打出来）。
func (t Token) GoString() string { return t.Redacted() }

// RedactToken — 脱敏任意令牌串（审计/日志唯一许可的令牌形态）。
func RedactToken(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= redactedPrefixLen+redactedSuffixLen {
		return redactedEllipsis
	}
	return v[:redactedPrefixLen] + redactedEllipsis + v[len(v)-redactedSuffixLen:]
}

// ── 对端身份 ──────────────────────────────────────────────────────────

// PeerIdentityRecord — 对端身份的**记录**（v3 §A 契约：{kind,id,start_time|pidversion,confidence}）。
//
//	Kind      — 身份来源机制（例：`linux-peerpidfd` / `linux-peersec` / `macos-vm` / `windows-sockaddr-hv`）
//	ID        — 该机制给出的标识（pid / 内核标签 / VmId:ServiceId）
//	StartTime — 抗复用的那一维（Linux 进程 starttime / macOS audit_token 的 pidversion）
//	Confidence— 强度，见 PeerIdentity 枚举。**只有 strong 许高权**。
//
// ★ 本结构**只用于归属 + 二次校验**，不是凭据：把它当授权依据 = 把 §A 的结论反过来读。
type PeerIdentityRecord struct {
	Kind       string
	ID         string
	StartTime  int64
	Confidence PeerIdentity
}

// PrivilegedAllowed — 只有 strong 许高权（weak/none/未知一律 false，fail-closed）。
func (p PeerIdentityRecord) PrivilegedAllowed() bool { return p.Confidence == PeerIdentityStrong }

// ConfidenceOrNone — 未知强度按 none 处理（✗ 不许把「认不出」当 strong）。
func (p PeerIdentityRecord) ConfidenceOrNone() PeerIdentity {
	if p.Confidence.Valid() {
		return p.Confidence
	}
	return PeerIdentityNone
}

// Validate — 强度必须落在 known 集合里（认不出 ⇒ 调用方该按 none 处理并记审计）。
func (p PeerIdentityRecord) Validate() error {
	if !p.Confidence.Valid() {
		return fmt.Errorf("cocoon: 对端身份强度非法 %q（strong|weak|none）", string(p.Confidence))
	}
	return nil
}

// String — 身份的人面（不含任何凭据；审计用）。
func (p PeerIdentityRecord) String() string {
	return fmt.Sprintf("peer{kind=%q id=%q start=%d conf=%q}",
		p.Kind, p.ID, p.StartTime, string(p.ConfidenceOrNone()))
}

// ── 授权请求 / 审计事件 ───────────────────────────────────────────────

// AuthzRequest — 一次授权判定要的**全部**输入（不多不少，免得判定里再摸外部状态）。
type AuthzRequest struct {
	// Action — 动作名（审计用；例：`exec` / `write` / `destroy`）。
	Action string
	// Privileged — 是否高权动作。true ⇒ 除令牌外**还**要求 identity=strong。
	Privileged bool
	// Peer — 对端身份（weak/none 也照样传进来 —— 拒/放由本层判，调用方不做决定）。
	Peer PeerIdentityRecord
}

// AuthzEvent — 一次判定的事实（**令牌只以脱敏形出现**，这是审计面唯一的令牌形态）。
type AuthzEvent struct {
	At         time.Time
	InstanceID string
	Action     string
	Privileged bool
	Allowed    bool
	Confidence PeerIdentity
	PeerKind   string
	PeerID     string
	Token      string // 脱敏形（RedactToken 的产物），**绝不**是令牌本体
	Reason     string
	Epoch      uint64
}

// String — 日志面。★ 唯一出现令牌的位置是 Token 字段，而它进构造时就已经是脱敏形。
func (e AuthzEvent) String() string {
	return fmt.Sprintf("[cocoon-auth] %s inst=%q action=%q allowed=%t privileged=%t conf=%q peer=%q/%q token=%q epoch=%d reason=%q",
		e.At.UTC().Format(time.RFC3339Nano), e.InstanceID, e.Action, e.Allowed, e.Privileged,
		string(e.ConfidenceOrNone()), e.PeerKind, e.PeerID, e.Token, e.Epoch, e.Reason)
}

// GoString — `%#v` 也走日志面（不露字段裸值）。
func (e AuthzEvent) GoString() string { return e.String() }

// ConfidenceOrNone — 未知强度归 none（与 PeerIdentityRecord 同口径）。
func (e AuthzEvent) ConfidenceOrNone() PeerIdentity {
	if e.Confidence.Valid() {
		return e.Confidence
	}
	return PeerIdentityNone
}

// ── 管理器 ────────────────────────────────────────────────────────────

// tokenEntry — 一枚在册令牌的**全部**留存量：摘要 + 脱敏形 + 元数据。
// ★ 这里没有 Value 字段，是「令牌不进日志/内存泄漏面」的结构性保证（不是纪律）。
type tokenEntry struct {
	digest    [sha256.Size]byte
	redacted  string
	issuedAt  time.Time
	expiresAt time.Time
	epoch     uint64
	revoked   bool
}

// TokenManager — 每茧一票的令牌真源（进程内）。
//
// 并发：一把互斥锁护 entries（签发/校验/吊销都是 O(1)，不做分片 —— 先简单）。
// 锁内**不调**审计钩子（钩子可能回写存储/再进来，见 emit）。
type TokenManager struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]*tokenEntry

	// rand — 随机源（缺省 crypto/rand.Reader）。测试可注入坏源验「拒发」这一支。
	rand io.Reader
	// now — 时钟（缺省 time.Now）。测试注入以验过期边界，**生产路径不设**。
	now func() time.Time
	// audit — 审计钩子（缺省 nil = 不记）。★ 只收脱敏形，拿不到令牌本体。
	audit func(AuthzEvent)
}

// NewTokenManager — 缺省 TTL（24h）的管理器。
func NewTokenManager() *TokenManager {
	return &TokenManager{
		ttl:     DefaultTokenTTL,
		entries: make(map[string]*tokenEntry),
		rand:    rand.Reader,
		now:     time.Now,
	}
}

// NewTokenManagerWithTTL — 自定义 TTL。ttl ≤ 0 ⇒ 拒（**没有「0 = 永久」这一档**）。
func NewTokenManagerWithTTL(ttl time.Duration) (*TokenManager, error) {
	if ttl <= 0 {
		return nil, fmt.Errorf("%w: %v", ErrBadTokenTTL, ttl)
	}
	m := NewTokenManager()
	m.ttl = ttl
	return m, nil
}

// TTL — 现用有效期。
func (m *TokenManager) TTL() time.Duration {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ttl
}

// SetAuditHook — 挂审计钩子（nil = 不记）。钩子在**锁外**调用。
func (m *TokenManager) SetAuditHook(hook func(AuthzEvent)) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audit = hook
}

// nowFn / randReader — 缺省值兜底（零值管理器也能拿到「安全缺省」）。
func (m *TokenManager) nowFn() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

func (m *TokenManager) randReader() io.Reader {
	if m.rand != nil {
		return m.rand
	}
	return rand.Reader
}

// IssueToken — 给实例签发一枚新令牌（32 字节随机 → base64url）。
//
// ★ 重发即吊销：旧票在这一步**立刻**失效（旧摘要被覆盖），与 TTL 无关。
// 返回的 Token.Epoch 为该实例的签发代数（首发 1，逐次 +1）。
func (m *TokenManager) IssueToken(instID string) (Token, error) {
	if m == nil {
		return Token{}, ErrTokenManagerNil
	}
	if strings.TrimSpace(instID) == "" {
		return Token{}, ErrEmptyInstanceID
	}
	m.mu.Lock()
	ttl := m.ttl
	m.mu.Unlock()
	if ttl <= 0 {
		return Token{}, fmt.Errorf("%w: %v", ErrBadTokenTTL, ttl)
	}

	buf := make([]byte, TokenBytes)
	if _, err := io.ReadFull(m.randReader(), buf); err != nil {
		return Token{}, fmt.Errorf("%w: %v", ErrTokenRand, err)
	}
	value := base64.RawURLEncoding.EncodeToString(buf)
	now := m.nowFn()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.entries == nil {
		m.entries = make(map[string]*tokenEntry)
	}
	var epoch uint64 = 1
	if old, ok := m.entries[instID]; ok {
		epoch = old.epoch + 1
	}
	tok := Token{Value: value, ExpiresAt: now.Add(ttl), Epoch: epoch}
	// 覆盖写法 = 旧票当场失效（不留第二枚摘要 ⇒ 不存在「两票并存」的窗口）。
	m.entries[instID] = &tokenEntry{
		digest:    sha256.Sum256([]byte(value)),
		redacted:  RedactToken(value),
		issuedAt:  now,
		expiresAt: tok.ExpiresAt,
		epoch:     epoch,
	}
	return tok, nil
}

// checkResult — 内部判定结果（原因只进审计，不进对外错误）。
type checkResult struct {
	ok       bool
	reason   string
	redacted string
	epoch    uint64
}

// check — 令牌校验本体：摘要 + subtle.ConstantTimeCompare（**不**比原串）。
//
// 比摘要而非原串的两个理由：① 长度差也走同一条常量时间路径（原串比会先比长度）；
// ② 摘要比较让「部分匹配」无从计时（sha256 的雪崩效应）。
func (m *TokenManager) check(instID, presented string) checkResult {
	if m == nil {
		return checkResult{reason: "no_manager"}
	}
	if presented == "" {
		return checkResult{reason: "empty_presented"}
	}
	got := sha256.Sum256([]byte(presented))

	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[instID]
	if !ok {
		return checkResult{reason: "not_issued"}
	}
	res := checkResult{redacted: e.redacted, epoch: e.epoch}
	switch {
	case e.revoked:
		res.reason = "revoked"
	case !m.nowFn().Before(e.expiresAt):
		res.reason = "expired"
	case subtle.ConstantTimeCompare(got[:], e.digest[:]) != 1:
		res.reason = "mismatch"
	default:
		res.ok = true
		res.reason = "ok"
	}
	return res
}

// VerifyToken — 令牌是否有效（未过期、未吊销、与在册摘要一致）。判定不消耗令牌。
func (m *TokenManager) VerifyToken(instID, presented string) bool {
	return m.check(instID, presented).ok
}

// RevokeToken — 吊销某实例的令牌（幂等：没签发过也算成功）。
// ★ 连摘要一起清零：吊销后再拿原票来比，比的是全零摘要，永远不中。
func (m *TokenManager) RevokeToken(instID string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[instID]
	if !ok {
		return
	}
	e.revoked = true
	e.digest = [sha256.Size]byte{}
}

// TokenInfo — 某实例令牌的**元数据**（不含凭据本体；审计/排障用）。
type TokenInfo struct {
	InstanceID string
	// Token — 脱敏形（`abcd…wxyz`）。
	Token     string
	IssuedAt  time.Time
	ExpiresAt time.Time
	Epoch     uint64
	Revoked   bool
	Expired   bool
}

// String — fmt 面（Token 已是脱敏形）。
func (i TokenInfo) String() string {
	return fmt.Sprintf("token{inst=%q token=%q epoch=%d issued=%s expires=%s revoked=%t expired=%t}",
		i.InstanceID, i.Token, i.Epoch, i.IssuedAt.UTC().Format(time.RFC3339),
		i.ExpiresAt.UTC().Format(time.RFC3339), i.Revoked, i.Expired)
}

// GoString — `%#v` 也走 String。
func (i TokenInfo) GoString() string { return i.String() }

// Info — 取元数据（第二个返回值 = 该实例在册过）。
func (m *TokenManager) Info(instID string) (TokenInfo, bool) {
	if m == nil {
		return TokenInfo{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[instID]
	if !ok {
		return TokenInfo{}, false
	}
	return TokenInfo{
		InstanceID: instID,
		Token:      e.redacted,
		IssuedAt:   e.issuedAt,
		ExpiresAt:  e.expiresAt,
		Epoch:      e.epoch,
		Revoked:    e.revoked,
		Expired:    !m.nowFn().Before(e.expiresAt),
	}, true
}

// PurgeExpired — 清掉「已过期且未吊销」的在册记录，返回清掉的条数。
//
// ★ 已吊销的记录**留着**：它只是代数计数器（epoch 单调），本身不含别名给的可用凭据。
// 清掉过期记录会让该实例下次签发的 epoch 从 1 重来 —— 实例 id 由后端分配、不复用，
// 故不产生歧义；若要「代数永续」，别清（或把 epoch 挪进 meta）。
func (m *TokenManager) PurgeExpired() int {
	if m == nil {
		return 0
	}
	now := m.nowFn()
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, e := range m.entries {
		if !e.revoked && !now.Before(e.expiresAt) {
			delete(m.entries, id)
			n++
		}
	}
	return n
}

// ── 授权判定 ──────────────────────────────────────────────────────────

// Authorize — 唯一授权入口（v3 §A）。判定顺序：**先认证（令牌）再授权（身份）**。
//
//	令牌无效（未签发 / 不匹配 / 已吊销 / 已过期）⇒ ErrUnauthorized（wrapped，可 errors.Is）
//	令牌有效 且 req.Privileged 且 身份 ≠ strong ⇒ ErrPrivilegeNotAllowed（wrapped）
//	其余 ⇒ nil（弱身份的非高权请求**放行**，但记审计）
//
// ★ fail-closed：认不出的 confidence 归 none ⇒ 高权必拒；nil 管理器 ⇒ 直接 ErrTokenManagerNil。
// ★ 错误串里**只有实例 id 与动作名**，绝不含 presented（那是凭据）。
func (m *TokenManager) Authorize(instID, presented string, req AuthzRequest) error {
	if m == nil {
		return ErrTokenManagerNil
	}
	res := m.check(instID, presented)
	conf := req.Peer.ConfidenceOrNone()

	ev := AuthzEvent{
		At:         m.nowFn(),
		InstanceID: instID,
		Action:     req.Action,
		Privileged: req.Privileged,
		Confidence: conf,
		PeerKind:   req.Peer.Kind,
		PeerID:     req.Peer.ID,
		Token:      res.redacted, // 只有脱敏形能进事件
		Epoch:      res.epoch,
	}

	if !res.ok {
		ev.Allowed = false
		ev.Reason = "token:" + res.reason
		m.emit(ev)
		return fmt.Errorf("%w: 实例 %q 动作 %q", ErrUnauthorized, instID, req.Action)
	}
	if req.Privileged && conf != PeerIdentityStrong {
		ev.Allowed = false
		ev.Reason = "privileged_needs_strong:" + string(conf)
		m.emit(ev)
		return fmt.Errorf("%w: 实例 %q 身份强度 %q 不得执行高权动作 %q",
			ErrPrivilegeNotAllowed, instID, string(conf), req.Action)
	}
	ev.Allowed = true
	if req.Privileged {
		ev.Reason = "ok:privileged_strong"
	} else {
		ev.Reason = "ok:token_only"
	}
	m.emit(ev)
	return nil
}

// emit — 锁外调钩子（钩子里若再调本管理器，不会自锁）。
func (m *TokenManager) emit(ev AuthzEvent) {
	if m == nil {
		return
	}
	m.mu.Lock()
	hook := m.audit
	m.mu.Unlock()
	if hook == nil {
		return
	}
	hook(ev)
}
