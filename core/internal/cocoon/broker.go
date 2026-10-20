// broker.go — 茧内 CLI 的宿主侧 broker：本地 Unix socket ＋ 令牌 ＋ **操作级**白名单 ＋ 一行审计。
//
// ★ 这一层把 T4（令牌 / 身份强度闸，auth.go）与 T5（凭据注入 / 回显擦除，inject.go）接在
// 一个**只走本机回环**的服务面上；不做任何真网络拨号（AI 上游的拨号属于后续件，本件只到
// 「把出站请求装配好、把真值留在宿主」为止）。
//
// 五条不变量（测试逐条打，改这里先读 v3 §A 与 v1 §5.2）：
//
//	① **先授权、后执行**：`Authorize` 在**任何** op 处理之前跑，且判定输入只有
//	  （inst, 出示的令牌, op 的权限档, 对端身份）—— 不看 args 内容决定放不放。
//	② **操作级授权**（v1 §5.2「`code.show` 而非 `zerg`」）：闸门认的是**一个具体操作**，
//	  不是命令名、也不是"这一类命令"。参数在授权**之后**按**该操作自己的** schema 校验
//	  （未知字段一律拒 ⇒ 偷加 flag 打不进 handler）。
//	③ **未知 op 按高权处理**（fail-closed）：注册表里没有的 op，授权时按 Privileged=true 判 ——
//	  这样"没令牌的人"探不出哪个 op 存在（一律 unauthorized），只有真过了授权才回 unknown_op。
//	④ **错误面只有码、没有细节**：响应帧只有 {ok, code, result}；令牌不匹配 / 已吊销 / 已过期
//	  一律同一个 `unauthorized`，不回原因、不回实例内幕、不回令牌的任何片段。
//	⑤ **审计一行、无真值**：{ts, inst, op, ok, ms, peer_confidence}，**没有** token 字段，
//	  也不含 args / 结果 / 密钥值；inst 与 op 是**客户端可控**字串 ⇒ 落盘前先收窄（去控制字符、
//	  截断），免得一行审计被塞进多行或撑爆。
//
// ★ socket 面（v3 §A「每茧独立 socket ＋ 独立 uid；否则 0600 形同虚设」）：
//
//	socket 0600 · 所在目录 0700（只收紧、不放松 —— 已有的 0755 目录会被减到 0700）
//	  ⇒ 同机别的用户连不上，连"试一下这个路径存不存在"都做不到（目录不可穿越）
//	启动时路径上已有 socket 件 ⇒ 先**探活**：有人在听 ⇒ 拒（不去抢别人的茧）；
//	  无人听（上次崩了留下的死 socket）⇒ 清掉重建 —— 否则每次重启都要人先手动 rm。
//	非 socket 件占位 ⇒ 拒（✗ 不悄悄删别人的普通文件）。
//
// ★ 帧协议：**每连接一行请求、一行响应**（JSON，UTF-8，≤ MaxFrameBytes）：
//
//	请求  {"inst":"<茧 id>","token":"<43 字符>","op":"code.show","args":{…}}
//	响应  {"ok":true,"result":{…}}  /  {"ok":false,"code":"unauthorized"}
//
//	一连接一请求是**故意的**：多请求复用一条连接会让"这条连接是谁"的判定只做一次，
//	而令牌可能在连接存活期间被重发（重发即吊销旧票，见 auth.go）⇒ 每条请求都重判一次。
package cocoon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	// MaxFrameBytes — 单帧上限（64 KiB）。一行超限即拒（不读到内存里再判）。
	MaxFrameBytes = 64 << 10

	// socketPerm / socketDirPerm — v3 §A 的 socket 面：socket 0600、目录 0700。
	socketPerm    = 0o600
	socketDirPerm = 0o700

	// connReadTimeout / connWriteTimeout — 单请求的读写死线。
	// 防的是"连上来不说话"占满 goroutine（不是防慢客户端，broker 只服务本机茧）。
	connReadTimeout  = 30 * time.Second
	connWriteTimeout = 10 * time.Second

	// staleDialTimeout — 判"路径上那枚 socket 还有没有人在听"的短拨号死线。
	staleDialTimeout = 300 * time.Millisecond

	// auditFieldMaxRunes — 审计里客户端可控字段（inst / op）的截断长度。
	auditFieldMaxRunes = 64

	// maxSocketPathBytes — socket 路径的字节上限（sun_path 取两个平台的最小值再留余量：
	// macOS 104、Linux 108，含结尾 NUL）。超了内核只回一个 `bind: invalid argument`，
	// 看不出是"路径太长" ⇒ 本层先判、先报。
	maxSocketPathBytes = 100

	// opPrefix — 授权动作名的前缀：`op:code.show`。审计与 AuthzEvent 都用它，
	// 这样"授权粒度 = 一个操作"在日志面上就是字面事实，不会被读成命令名。
	opPrefix = "op:"
)

// 响应码（wire 面只有这几个字符串，**不带任何细节**）。
const (
	// CodeUnauthorized — 令牌校验未过（未签发 / 不匹配 / 已吊销 / 已过期；四种同码）。
	CodeUnauthorized = "unauthorized"
	// CodeDenied — 令牌过了但身份强度不足（高权操作 + weak/none）。
	CodeDenied = "denied"
	// CodeUnknownOp — 令牌与身份都过了，但注册表里没有这个 op。
	CodeUnknownOp = "unknown_op"
	// CodeBadRequest — 帧形态 / 参数不合 schema。
	CodeBadRequest = "bad_request"
	// CodeInternal — handler 报错（原因不回给对端，只进 ErrorHook）。
	CodeInternal = "internal"
)

var (
	// ErrBadSocketPath — socket 路径为空。
	ErrBadSocketPath = errors.New("cocoon: broker socket 路径为空")
	// ErrSocketInUse — 路径上的 socket 有人在听（另一个 broker 活着）。✗ 不抢。
	ErrSocketInUse = errors.New("cocoon: broker socket 已被占用（有进程在听，拒绝抢占）")
	// ErrNonSocketInPath — 路径上有个**非 socket** 件。✗ 不删别人的件。
	ErrNonSocketInPath = errors.New("cocoon: socket 路径上已有非 socket 件（拒绝删除）")
	// ErrBadSocketPerm — 建完自检：socket 权限不是 0600（chmod 没成 / 件被换掉）。
	ErrBadSocketPerm = errors.New("cocoon: broker socket 权限不是 0600")
	// ErrNotListening — Serve 时还没有 Listen，或已经 Close。
	ErrNotListening = errors.New("cocoon: broker 未在监听（先 Listen）")
	// ErrBrokerAlreadyListening — 重复 Listen。
	ErrBrokerAlreadyListening = errors.New("cocoon: broker 已在监听")
	// ErrBrokerNilTokens — 没给令牌管理器。fail-closed：没有令牌面就没有授权面。
	ErrBrokerNilTokens = errors.New("cocoon: broker 没有令牌管理器（用 NewBroker）")
	// ErrOpExists — 重复注册同一个 op。
	ErrOpExists = errors.New("cocoon: 操作已注册")
	// ErrOpEmptyName — 注册空名字的 op（空名会让"未知 op"和"空 op"混成一格）。
	ErrOpEmptyName = errors.New("cocoon: 操作名为空")
	// ErrFrameTooLong — 单帧超过 MaxFrameBytes。
	ErrFrameTooLong = errors.New("cocoon: 帧超过上限")
	// ErrSocketPathTooLong — socket 路径超过内核 sun_path 容量（超了只会得到
	// 一句 `bind: invalid argument`，看不出真因）。
	ErrSocketPathTooLong = errors.New("cocoon: broker socket 路径过长")
)

// ── 帧 ────────────────────────────────────────────────────────────────

// brokerRequest — 请求帧。★ 字段顺序即协议：inst 定位茧、token 是唯一凭据、op 是授权粒度、
// args 在授权**之后**才被解读。
type brokerRequest struct {
	Inst  string          `json:"inst"`
	Token string          `json:"token"`
	Op    string          `json:"op"`
	Args  json.RawMessage `json:"args"`
}

// String — 人面（★ 只出脱敏形；这个类型永远不该被 `%v` 打到日志里，但真打了也漏不出去）。
func (r brokerRequest) String() string {
	return fmt.Sprintf("brokerReq{inst=%q op=%q token=%q}", r.Inst, r.Op, RedactToken(r.Token))
}

// GoString — `%#v` 同口径。
func (r brokerRequest) GoString() string { return r.String() }

// brokerResponse — 响应帧。**故意只有三个字段**：状态、码、结果。
// 加字段（reason / detail / hint）之前先想清楚：那等于把"为什么拒"告诉对端。
type brokerResponse struct {
	OK     bool            `json:"ok"`
	Code   string          `json:"code,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

// ── 操作白名单（授权粒度 = 一个操作）────────────────────────────────────

// OpRequest — 交给 handler 的一次调用（已过授权与参数校验）。
type OpRequest struct {
	// Inst — 茧 id（授权过的那个，不是客户端自报的第二份）。
	Inst string
	// Args — 原始参数。handler 自己按需解析（本层已按 schema 校验过一遍）。
	Args json.RawMessage
}

// OpSpec — 注册表里的一项：一个**操作**（不是命令族）的权限档、参数 schema、执行体。
//
// ★ Privileged 是**唯一的权限来源**：授权判定只读这一位（外加令牌与身份强度）。
// 用 `code.show` 而不是 `zerg` / `code.*` 当键，正是 v1 §5.2「授权粒度是完整操作」的落点 ——
// 命令名级的白名单在事故里被打穿过（`git show --output` 就是按命令名判的洞）。
type OpSpec struct {
	// Name — 操作名（`code.show`）。点分、小写、无空白。
	Name string
	// Privileged — 高权操作：除令牌外**还**要求对端身份 strong（weak ⇒ 拒）。
	Privileged bool
	// Validate — 参数 schema 校验（nil = 只要求是合法 JSON 对象）。
	// ★ 它跑在**授权之后**：授权不看参数内容，参数只是"这次授权要执行的那个具体形状"。
	Validate func(args json.RawMessage) error
	// Handle — 执行体。返回的 result 会被 JSON 化（并且**过一遍回显擦除**，见 Broker.respond）。
	Handle func(ctx context.Context, b *Broker, req OpRequest) (any, error)
}

// OpRegistry — 白名单注册表。**没有注册的 op 一律不存在**（不是"默认放行"）。
type OpRegistry struct {
	mu  sync.RWMutex
	ops map[string]OpSpec
}

// NewOpRegistry — 空注册表。
func NewOpRegistry() *OpRegistry {
	return &OpRegistry{ops: make(map[string]OpSpec)}
}

// Register — 注册一个操作。空名 / 重名一律拒（重名会让"谁的权限档生效"取决于注册顺序）。
func (r *OpRegistry) Register(spec OpSpec) error {
	if r == nil {
		return errors.New("cocoon: 操作注册表未初始化")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return ErrOpEmptyName
	}
	if spec.Handle == nil {
		return fmt.Errorf("cocoon: 操作 %q 没有执行体", spec.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.ops[spec.Name]; dup {
		return fmt.Errorf("%w: %q", ErrOpExists, spec.Name)
	}
	if r.ops == nil {
		r.ops = make(map[string]OpSpec)
	}
	r.ops[spec.Name] = spec
	return nil
}

// Lookup — 查一个操作（不在册 ⇒ false）。
func (r *OpRegistry) Lookup(name string) (OpSpec, bool) {
	if r == nil {
		return OpSpec{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := r.ops[name]
	return spec, ok
}

// Names — 在册操作名（排序后）。只给人面/自检用，**不进 wire**。
func (r *OpRegistry) Names() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.ops))
	for name := range r.ops {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ── 审计行 ────────────────────────────────────────────────────────────

// AuditLine — 每请求一行。★ 字段表就是"不含真值"的证明：没有 token、没有 args、
// 没有结果、没有密钥值。客户端可控的两个字串（inst / op）已收窄。
type AuditLine struct {
	// TS — 请求开始时刻（UTC · RFC3339Nano）。
	TS string `json:"ts"`
	// Inst — 茧 id（收窄后）。空串 = 帧没解出来 / 没给。
	Inst string `json:"inst"`
	// Op — 请求的操作名（收窄后）。空串 = 没给。
	Op string `json:"op"`
	// OK — 是否成功执行完（授权过 + 参数合 schema + handler 不报错）。
	OK bool `json:"ok"`
	// MS — 处理耗时（毫秒）。
	MS int64 `json:"ms"`
	// PeerConfidence — 对端身份强度（strong|weak|none）。
	PeerConfidence string `json:"peer_confidence"`
}

// ── Broker ────────────────────────────────────────────────────────────

// Broker — 一个茧（或一组共用同一 uid 的茧）的宿主侧 socket 服务。
//
// ★ 零值不可用：至少要有 Tokens（NewBroker 会填 Ops=DefaultOps()）。
type Broker struct {
	// Tokens — T4 的令牌管理器（唯一授权入口）。
	Tokens *TokenManager
	// Ops — 操作白名单。nil ⇒ 什么都不许执行（fail-closed，不是"全放行"）。
	Ops *OpRegistry
	// Secrets — T5 的密钥解析器（`ai.request` 用它把占位符换成真值）。
	Secrets SecretResolver
	// ScrubSecrets — 出站响应要擦的真值清单（`ScrubResponse` 用）。
	// ★ nil 表示"没有配擦除面"——生产必须配；本层不替调用方猜真值。
	ScrubSecrets []string
	// Audit — 审计落点（一行一条 JSON）。nil ⇒ os.Stderr（**不静默丢弃**）。
	Audit io.Writer
	// Now — 时钟（测试注入；nil ⇒ time.Now）。
	Now func() time.Time
	// PeerProbe — 对端身份探测（测试注入；nil ⇒ 分平台探测，见 peer_probe*.go）。
	PeerProbe func(*net.UnixConn) PeerIdentityRecord
	// ErrorHook — handler 报错的服务端出口（**细节只走这里**，不进 wire）。
	ErrorHook func(inst, op string, err error)

	mu       sync.Mutex
	ln       *net.UnixListener
	path     string
	closed   bool
	closeErr error
	wg       sync.WaitGroup

	auditMu sync.Mutex
}

// NewBroker — 用 T4 的令牌管理器构造（操作白名单默认 = DefaultOps()）。
func NewBroker(tokens *TokenManager) *Broker {
	return &Broker{Tokens: tokens, Ops: DefaultOps()}
}

// ── 生命周期 ──────────────────────────────────────────────────────────

// Listen — 建目录（0700）→ 处理残留 socket → 绑定 → chmod 0600 → 自检权限。
//
// ★ 自检那一步不是形式主义：`net.ListenUnix` 建出来的 socket 会**继承 umask**（常见 0755），
// chmod 之后再 Lstat 现读一遍，才能在"chmod 被拦 / 件被别的进程换掉"时当场红，
// 而不是带着 0755 的 socket 跑一整夜。
func (b *Broker) Listen(socketPath string) error {
	if b == nil {
		return ErrBrokerNilTokens
	}
	if b.Tokens == nil {
		return ErrBrokerNilTokens
	}
	if strings.TrimSpace(socketPath) == "" {
		return ErrBadSocketPath
	}
	if len(socketPath) > maxSocketPathBytes {
		return fmt.Errorf("%w: %d 字节 > %d", ErrSocketPathTooLong, len(socketPath), maxSocketPathBytes)
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return ErrNotListening
	}
	if b.ln != nil {
		b.mu.Unlock()
		return ErrBrokerAlreadyListening
	}
	b.mu.Unlock()

	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, socketDirPerm); err != nil {
		return fmt.Errorf("cocoon: 建 broker socket 目录 %q 失败: %w", dir, err)
	}
	// 目录 0700：**只收紧、不放松**（对方给的 0755 只去掉组/其他位，不夺走属主的黏滞位一类）。
	if fi, err := os.Stat(dir); err != nil {
		return fmt.Errorf("cocoon: 读 broker socket 目录 %q 失败: %w", dir, err)
	} else if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		if err := os.Chmod(dir, perm&^0o077); err != nil {
			return fmt.Errorf("cocoon: 收紧 broker socket 目录 %q 到 0700 失败: %w", dir, err)
		}
	}

	if err := clearStaleSocket(socketPath); err != nil {
		return err
	}

	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return fmt.Errorf("cocoon: 绑定 broker socket %q 失败: %w", socketPath, err)
	}
	if err := os.Chmod(socketPath, socketPerm); err != nil {
		ln.Close()
		os.Remove(socketPath)
		return fmt.Errorf("cocoon: 把 broker socket %q 置为 0600 失败: %w", socketPath, err)
	}
	if perr := verifySocketPerm(socketPath); perr != nil {
		ln.Close()
		os.Remove(socketPath)
		return perr
	}

	b.mu.Lock()
	b.ln = ln
	b.path = socketPath
	b.mu.Unlock()
	return nil
}

// verifySocketPerm — 建完**现读**一遍权限。抽成独立函数是为了让这条自检能被单独测到：
// `chmod` 成功不代表"盘上现在就是 0600"（件可能在两步之间被换掉），而这一条是最后一道闸。
func verifySocketPerm(socketPath string) error {
	fi, err := os.Lstat(socketPath)
	if err != nil {
		return fmt.Errorf("cocoon: 自检读 broker socket %q 失败: %w", socketPath, err)
	}
	if perm := fi.Mode().Perm(); perm != socketPerm {
		return fmt.Errorf("%w: %q 现读 %#o", ErrBadSocketPerm, socketPath, perm)
	}
	return nil
}

// clearStaleSocket — 路径上已有件时的三态：无件 ⇒ 放过；死 socket ⇒ 清掉；
// 活 socket ⇒ 拒（有人在听就是别人的茧，✗ 不抢）；非 socket 件 ⇒ 拒（✗ 不删别人的件）。
//
// ★ "死 socket" 只有一种判法：**拨一下**。不能靠"文件在不在"（重启后它必然还在），
// 也不能靠 socket 的 mtime/pid 字段（Linux 上根本没有，BSD/macOS 上也不可信）。
func clearStaleSocket(socketPath string) error {
	fi, err := os.Lstat(socketPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cocoon: 读 broker socket 路径 %q 失败: %w", socketPath, err)
	}
	if fi.Mode()&fs.ModeSocket == 0 {
		return fmt.Errorf("%w: %q（%s）", ErrNonSocketInPath, socketPath, fi.Mode().Type())
	}
	if c, derr := net.DialTimeout("unix", socketPath, staleDialTimeout); derr == nil {
		c.Close()
		return fmt.Errorf("%w: %q", ErrSocketInUse, socketPath)
	}
	if err := os.Remove(socketPath); err != nil {
		return fmt.Errorf("cocoon: 清理残留 broker socket %q 失败: %w", socketPath, err)
	}
	return nil
}

// SocketPath — 当前监听的 socket 路径（未监听 ⇒ 空串）。
func (b *Broker) SocketPath() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.path
}

// Serve — 接客循环，直到 ctx 结束或 Close。返回 nil 表示"正常收工"。
//
// ★ 每条连接一个 goroutine、**一条请求**：一连接一请求让令牌/身份在**每次**请求上重判
// （令牌重发即吊销旧票 ⇒ 长连接会把"已吊销"变成"还能用"）。
func (b *Broker) Serve(ctx context.Context) error {
	if b == nil {
		return ErrBrokerNilTokens
	}
	b.mu.Lock()
	ln := b.ln
	b.mu.Unlock()
	if ln == nil {
		return ErrNotListening
	}
	if ctx == nil {
		ctx = context.Background()
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			b.Close()
		case <-done:
		}
	}()

	for {
		conn, err := ln.AcceptUnix()
		if err != nil {
			if b.isClosed() || ctx.Err() != nil {
				b.wg.Wait()
				return nil
			}
			// 单次 accept 失败不放手（本机 socket 上 EINTR/EMFILE 都可恢复），但连续错没有意义 ——
			// 交给调用方看见（Serve 返回错误即"这一层不工作了"，别静默空转）。
			b.wg.Wait()
			return fmt.Errorf("cocoon: broker accept 失败: %w", err)
		}
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			b.serveConn(ctx, conn)
		}()
	}
}

// Close — 停接客、等在手请求收工。可重复调用。
func (b *Broker) Close() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	ln := b.ln
	b.ln = nil
	b.path = ""
	b.closed = true
	err := b.closeErr
	b.closeErr = ErrNotListening
	b.mu.Unlock()

	if ln != nil {
		if cerr := ln.Close(); cerr != nil && !errors.Is(cerr, net.ErrClosed) {
			err = cerr
		}
	}
	b.wg.Wait()
	if errors.Is(err, ErrNotListening) {
		return nil
	}
	return err
}

func (b *Broker) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

// ── 单请求处理 ────────────────────────────────────────────────────────

// serveConn — 读一行 → 授权 → 校验参数 → 执行 → 写一行。
//
// ★ 次序是**审计先写、响应后写**：客户端拿到响应的那一刻，这一行的审计一定已经在盘上。
// 反过来（先回响应、后写审计）会留一个窗口 —— 响应到了、审计丢了，事后查不出这次调用。
func (b *Broker) serveConn(ctx context.Context, conn *net.UnixConn) {
	defer conn.Close()

	peer := b.peerOf(conn)
	start := b.now()
	resp, inst, op, ok := b.dispatch(ctx, conn, peer)
	b.audit(start, inst, op, ok, peer)
	b.respond(conn, resp)
}

// dispatch — 一次请求的全部判定，返回（响应, 审计用的 inst/op, 是否成功）。
// 拆出来是为了让"审计先于响应"这条次序在代码形状上就成立（单一出口）。
func (b *Broker) dispatch(ctx context.Context, conn *net.UnixConn, peer PeerIdentityRecord) (brokerResponse, string, string, bool) {
	fail := func(code string) (brokerResponse, string, string, bool) {
		return brokerResponse{OK: false, Code: code}, "", "", false
	}

	_ = conn.SetReadDeadline(b.now().Add(connReadTimeout))
	line, err := readFrame(conn)
	if err != nil {
		return fail(CodeBadRequest)
	}
	var req brokerRequest
	if err := json.Unmarshal(line, &req); err != nil {
		return fail(CodeBadRequest)
	}
	inst, op := req.Inst, req.Op
	if strings.TrimSpace(inst) == "" || strings.TrimSpace(op) == "" {
		return brokerResponse{OK: false, Code: CodeBadRequest}, inst, op, false
	}

	spec, known := b.lookupOp(op)
	// ★ fail-closed：认不出的 op 按**高权**判 —— 没令牌的人一律 unauthorized，
	//   拿不到"哪个 op 存在"这张探针表（否则未授权方也能枚举命令面）。
	privileged := true
	if known {
		privileged = spec.Privileged
	}

	// ① 先授权（唯一入口；令牌 → 身份强度）。
	if aerr := b.Tokens.Authorize(inst, req.Token, AuthzRequest{
		Action:     opPrefix + op,
		Privileged: privileged,
		Peer:       peer,
	}); aerr != nil {
		code := CodeDenied
		if errors.Is(aerr, ErrUnauthorized) || errors.Is(aerr, ErrTokenManagerNil) || errors.Is(aerr, ErrEmptyInstanceID) {
			code = CodeUnauthorized
		}
		return brokerResponse{OK: false, Code: code}, inst, op, false
	}

	// ② 授权过了才谈"这个 op 存不存在"。
	if !known {
		return brokerResponse{OK: false, Code: CodeUnknownOp}, inst, op, false
	}

	// ③ 参数按**该操作自己的** schema 校验（未知字段拒 ⇒ 偷加 flag 进不来）。
	if spec.Validate != nil {
		if verr := spec.Validate(req.Args); verr != nil {
			return brokerResponse{OK: false, Code: CodeBadRequest}, inst, op, false
		}
	}

	// ④ 执行。
	res, herr := spec.Handle(ctx, b, OpRequest{Inst: inst, Args: req.Args})
	if herr != nil {
		if b.ErrorHook != nil {
			b.ErrorHook(inst, op, herr)
		}
		return brokerResponse{OK: false, Code: CodeInternal}, inst, op, false
	}
	raw, merr := json.Marshal(res)
	if merr != nil {
		if b.ErrorHook != nil {
			b.ErrorHook(inst, op, merr)
		}
		return brokerResponse{OK: false, Code: CodeInternal}, inst, op, false
	}
	return brokerResponse{OK: true, Result: raw}, inst, op, true
}

// respond — 写一行响应。★ 结果先过 `ScrubResponse`（T5 的回显擦除）：
// 即便某个 handler 手滑把真值放进了 result，出线前也会被打成 `[REDACTED]`。
func (b *Broker) respond(conn *net.UnixConn, resp brokerResponse) {
	if len(resp.Result) > 0 && len(b.ScrubSecrets) > 0 {
		resp.Result = ScrubResponse(resp.Result, b.ScrubSecrets)
	}
	// ★ 擦除后再统一 percent/转义：Result 是 json.RawMessage，ScrubResponse 只换字节，
	//   只要替身是纯 ASCII（`[REDACTED]`）就不会把 JSON 结构弄坏；真弄坏了下面 marshal 会报错 ——
	//   那种情况宁可不回，也不回一份半截 JSON。
	if len(resp.Result) > 0 && !json.Valid(resp.Result) {
		resp = brokerResponse{OK: false, Code: CodeInternal}
	}
	out, err := json.Marshal(resp)
	if err != nil {
		out = []byte(`{"ok":false,"code":"internal"}`)
	}
	out = append(out, '\n')
	_ = conn.SetWriteDeadline(time.Now().Add(connWriteTimeout))
	_, _ = conn.Write(out)
}

// audit — 写一行审计（无令牌、无 args、无真值）。
func (b *Broker) audit(start time.Time, inst, op string, ok bool, peer PeerIdentityRecord) {
	line := AuditLine{
		TS:             start.UTC().Format(time.RFC3339Nano),
		Inst:           auditField(inst),
		Op:             auditField(op),
		OK:             ok,
		MS:             b.now().Sub(start).Milliseconds(),
		PeerConfidence: string(peer.ConfidenceOrNone()),
	}
	body, err := json.Marshal(line)
	if err != nil {
		return
	}
	body = append(body, '\n')

	w := b.Audit
	if w == nil {
		w = os.Stderr
	}
	b.auditMu.Lock()
	defer b.auditMu.Unlock()
	_, _ = w.Write(body)
}

// lookupOp — 注册表查询（nil 注册表 ⇒ 什么都不在册）。
func (b *Broker) lookupOp(name string) (OpSpec, bool) {
	if b == nil {
		return OpSpec{}, false
	}
	if b.Ops == nil {
		return OpSpec{}, false
	}
	return b.Ops.Lookup(name)
}

func (b *Broker) peerOf(conn *net.UnixConn) PeerIdentityRecord {
	probe := b.PeerProbe
	if probe == nil {
		probe = defaultPeerProbe
	}
	rec := probe(conn)
	if err := rec.Validate(); err != nil {
		// 认不出的强度按 none（✗ 不许把"认不出"当 strong）。
		return PeerIdentityRecord{Kind: rec.Kind, ID: rec.ID, StartTime: rec.StartTime, Confidence: PeerIdentityNone}
	}
	return rec
}

func (b *Broker) now() time.Time {
	if b != nil && b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

// readFrame — 读一行（≤ MaxFrameBytes）。超限 ⇒ ErrFrameTooLong（不继续读）。
func readFrame(conn *net.UnixConn) ([]byte, error) {
	br := bufio.NewReaderSize(conn, MaxFrameBytes)
	line, err := br.ReadSlice('\n')
	if err != nil {
		if errors.Is(err, bufio.ErrBufferFull) {
			return nil, ErrFrameTooLong
		}
		return nil, err
	}
	line = []byte(strings.TrimRight(string(line), "\r\n"))
	if len(line) == 0 {
		return nil, errors.New("cocoon: 空帧")
	}
	return line, nil
}

// auditField — 收窄客户端可控字串：去掉控制字符（防一行变多行）、限长（防撑爆审计）。
func auditField(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= auditFieldMaxRunes {
			break
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// ── 内置操作（两枚桩）────────────────────────────────────────────────

// codeShowArgs — `code.show` 的参数 schema。
type codeShowArgs struct {
	// File — 要看的件（相对茧根）。本桩只做形态校验：非空、不带 `..` 段、不是绝对路径。
	File string `json:"file"`
}

// codeShowResult — `code.show` 的结果形状（桩：不回内容，只回"这次要看哪个件"）。
type codeShowResult struct {
	File string `json:"file"`
	Stub bool   `json:"stub"`
}

// aiRequestArgs — `ai.request` 的参数 schema：目标 host ＋ 带占位符的请求头。
type aiRequestArgs struct {
	Host    string            `json:"host"`
	Headers map[string]string `json:"headers"`
}

// aiRequestResult — ★ 出去的**只有计划**：header 名、以及"换掉了几个占位符"。
// 真值一个字都不回 —— broker 自己拿真值去拨上游（拨号属后续件），茧内永远只见占位符。
type aiRequestResult struct {
	Host        string   `json:"host"`
	HeaderNames []string `json:"header_names"`
	Substituted int      `json:"substituted"`
}

// DefaultOps — 内置白名单：两枚桩。权限档是刻意的：
//
//	code.show   非高权（只读代码视图；weak 身份也放行 —— v3 §A「判据 7 不依赖平台能力」）
//	ai.request  高权（它会把真值注入出站请求；weak ⇒ 拒，fail-closed）
func DefaultOps() *OpRegistry {
	r := NewOpRegistry()
	_ = r.Register(OpSpec{
		Name:       "code.show",
		Privileged: false,
		Validate:   validateCodeShow,
		Handle:     handleCodeShow,
	})
	_ = r.Register(OpSpec{
		Name:       "ai.request",
		Privileged: true,
		Validate:   validateAIRequest,
		Handle:     handleAIRequest,
	})
	return r
}

func validateCodeShow(args json.RawMessage) error {
	var a codeShowArgs
	if err := strictUnmarshal(args, &a); err != nil {
		return err
	}
	if a.File == "" {
		return errors.New("cocoon: code.show 缺 file")
	}
	if filepath.IsAbs(a.File) {
		return errors.New("cocoon: code.show 的 file 不许是绝对路径")
	}
	for _, seg := range strings.Split(filepath.ToSlash(a.File), "/") {
		if seg == ".." {
			return errors.New("cocoon: code.show 的 file 不许含 ..")
		}
	}
	return nil
}

func handleCodeShow(_ context.Context, _ *Broker, req OpRequest) (any, error) {
	var a codeShowArgs
	if err := strictUnmarshal(req.Args, &a); err != nil {
		return nil, err
	}
	// 桩：本件不读盘（真读要接茧根沙箱件）。形状与真实现一致，换实现时不改协议。
	return codeShowResult{File: a.File, Stub: true}, nil
}

func validateAIRequest(args json.RawMessage) error {
	var a aiRequestArgs
	if err := strictUnmarshal(args, &a); err != nil {
		return err
	}
	if strings.TrimSpace(a.Host) == "" {
		return errors.New("cocoon: ai.request 缺 host")
	}
	if len(a.Headers) == 0 {
		return errors.New("cocoon: ai.request 缺 headers")
	}
	return nil
}

func handleAIRequest(_ context.Context, b *Broker, req OpRequest) (any, error) {
	var a aiRequestArgs
	if err := strictUnmarshal(req.Args, &a); err != nil {
		return nil, err
	}
	h := make(http.Header, len(a.Headers))
	for k, v := range a.Headers {
		h.Set(k, v)
	}
	// T5 的注入：host 白名单 + 占位符换值（fail-closed：任一步不过 ⇒ 整条拒）。
	// ★ 这里**只装配、不回传**：把 out 里的真值留在宿主进程里，交给后续的拨号件。
	out, err := RewriteRequest(h, a.Host, b.Secrets)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out))
	substituted := 0
	for k, v := range out {
		names = append(names, k)
		if len(v) > 0 && v[0] != h.Get(k) {
			substituted++
		}
	}
	sort.Strings(names)
	return aiRequestResult{Host: a.Host, HeaderNames: names, Substituted: substituted}, nil
}

// strictUnmarshal — 严格解析：未知字段一律拒（偷加的 flag 打不进 handler）。
func strictUnmarshal(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return errors.New("cocoon: 缺 args")
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("cocoon: args 后面还有多余内容")
	}
	return nil
}
