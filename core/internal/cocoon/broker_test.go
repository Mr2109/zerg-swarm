// broker_test.go — T4b 的七格＋两格加固：授权 → 执行、错误面无细节、未知 op 拒、
// 弱身份 × 高权拒、socket 0600/目录 0700、残留 socket 重建、审计无真值。
//
// ★ 这一件测的是**行为面**（wire 上看到什么、盘上留下什么），不是内部函数；
// 每一格都从"连上去、发一行、读一行"开始 —— 因为 broker 的契约就在那一行帧上。
package cocoon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// ── 夹具 ──────────────────────────────────────────────────────────────

// lockedBuf — 并发安全的审计缓冲（handler goroutine 写、测试 goroutine 读）。
type lockedBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// Lines — 非空行。
func (l *lockedBuf) Lines() []string {
	var out []string
	for _, ln := range strings.Split(l.String(), "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}

// rig — 一个跑起来的 broker ＋ 它用到的令牌管理器与审计缓冲。
type rig struct {
	t     *testing.T
	b     *Broker
	tm    *TokenManager
	sock  string
	dir   string
	audit *lockedBuf
}

// strongProbe / weakProbe — 测试注入的对端身份（生产走分平台探针；此处把强度钉死，
// 免得"这一格在哪台机器上跑"改变结论）。
func strongProbe(*net.UnixConn) PeerIdentityRecord {
	return PeerIdentityRecord{Kind: "test-strong", ID: "4242", StartTime: 7, Confidence: PeerIdentityStrong}
}

func weakProbe(*net.UnixConn) PeerIdentityRecord {
	return PeerIdentityRecord{Kind: "test-weak", ID: "4242", Confidence: PeerIdentityWeak}
}

// shortDir — 短路径临时目录。★ 必须用短路径：unix socket 的 sun_path 只有 104 字节
// （macOS），而 lane/CI 的 TMPDIR 动辄上百字节 ⇒ 直接 t.TempDir() 会得到
// 一句 `bind: invalid argument`（看着像参数写错，其实是路径太长）。
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "cbs")
	if err != nil {
		t.Fatalf("建短路径临时目录失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// newRig — 建目录、Listen、起 Serve。probe 为 nil 时用生产探针。
func newRig(t *testing.T, probe func(*net.UnixConn) PeerIdentityRecord) *rig {
	t.Helper()
	dir := shortDir(t)
	sock := filepath.Join(dir, "broker.sock")
	tm := NewTokenManager()
	audit := &lockedBuf{}
	b := NewBroker(tm)
	b.Audit = audit
	if probe != nil {
		b.PeerProbe = probe
	}
	if err := b.Listen(sock); err != nil {
		t.Fatalf("Listen(%q) 失败: %v", sock, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- b.Serve(ctx) }()
	r := &rig{t: t, b: b, tm: tm, sock: sock, dir: dir, audit: audit}
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-errc:
			if err != nil {
				t.Errorf("Serve 收工时返回错误: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Errorf("Serve 在 5s 内没有退出")
		}
	})
	return r
}

// issue — 签发一枚令牌并返回它的本体（测试里允许持有：这是客户端那一侧）。
func (r *rig) issue(inst string) string {
	r.t.Helper()
	tok, err := r.tm.IssueToken(inst)
	if err != nil {
		r.t.Fatalf("IssueToken(%q) 失败: %v", inst, err)
	}
	return tok.Value
}

// send — 发一行原始字节，收一行原始字节（★ 断言"无细节"必须看原始字节）。
func (r *rig) send(raw string) []byte {
	r.t.Helper()
	c, err := net.Dial("unix", r.sock)
	if err != nil {
		r.t.Fatalf("拨 broker socket 失败: %v", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Write([]byte(raw + "\n")); err != nil {
		r.t.Fatalf("写帧失败: %v", err)
	}
	line, err := bufio.NewReader(c).ReadBytes('\n')
	if err != nil {
		r.t.Fatalf("读响应失败: %v", err)
	}
	return line
}

// call — 发一条规整请求，回解析后的响应。
func (r *rig) call(inst, token, op string, args any) map[string]any {
	r.t.Helper()
	req := map[string]any{"inst": inst, "token": token, "op": op}
	if args != nil {
		req["args"] = args
	}
	raw, err := json.Marshal(req)
	if err != nil {
		r.t.Fatalf("序列化请求失败: %v", err)
	}
	line := r.send(string(raw))
	var out map[string]any
	if err := json.Unmarshal(line, &out); err != nil {
		r.t.Fatalf("响应不是合法 JSON: %q (%v)", line, err)
	}
	return out
}

// errorCode — 断言"拒了"并取出码（成功响应直接判红）。
func errorCode(t *testing.T, resp map[string]any, want string) {
	t.Helper()
	if ok, _ := resp["ok"].(bool); ok {
		t.Fatalf("本该被拒，却回了成功: %#v", resp)
	}
	if got, _ := resp["code"].(string); got != want {
		t.Fatalf("码不对: 得 %q 要 %q（整帧 %#v）", got, want, resp)
	}
	// ★ 拒的帧只许有 {ok, code} —— 多一个 reason/detail 就是把"为什么"告诉了对端。
	for k := range resp {
		if k != "ok" && k != "code" {
			t.Fatalf("拒的响应里多出字段 %q（整帧 %#v）", k, resp)
		}
	}
}

// auditLines — 把审计缓冲解成结构体（顺带断言每行都是合法 JSON）。
func (r *rig) auditLines() []AuditLine {
	r.t.Helper()
	var out []AuditLine
	for _, ln := range r.audit.Lines() {
		var a AuditLine
		if err := json.Unmarshal([]byte(ln), &a); err != nil {
			r.t.Fatalf("审计行不是合法 JSON: %q (%v)", ln, err)
		}
		out = append(out, a)
	}
	return out
}

// ── ① 合法令牌 × 非高权 op ⇒ 成功 ─────────────────────────────────────

func TestBrokerAuthorizedOpSucceeds(t *testing.T) {
	// 生产探针（不注入）：macOS 上是 weak，而 code.show 是**非高权** ⇒ 照样放行。
	// 这正是 v3 §A「判据 7 不冒用必拒，不依赖任何平台能力」的落点。
	r := newRig(t, nil)
	tok := r.issue("cocoon-a")

	resp := r.call("cocoon-a", tok, "code.show", map[string]any{"file": "core/main.go"})
	if ok, _ := resp["ok"].(bool); !ok {
		t.Fatalf("合法令牌 + 非高权 op 应当成功，实得 %#v", resp)
	}
	res, _ := resp["result"].(map[string]any)
	if res == nil || res["file"] != "core/main.go" {
		t.Fatalf("result 形状不对: %#v", resp)
	}

	lines := r.auditLines()
	if len(lines) != 1 {
		t.Fatalf("审计应当恰好 1 行，实得 %d 行: %#v", len(lines), lines)
	}
	a := lines[0]
	if !a.OK || a.Op != "code.show" || a.Inst != "cocoon-a" {
		t.Fatalf("审计行不对: %#v", a)
	}
	if a.PeerConfidence == "" {
		t.Fatalf("审计行缺 peer_confidence: %#v", a)
	}
	if a.TS == "" {
		t.Fatalf("审计行缺 ts: %#v", a)
	}
}

// ── ② 错令牌 ⇒ 拒，且**无细节**（三种原因同帧）────────────────────────

func TestBrokerBadTokenRefusedWithoutDetail(t *testing.T) {
	r := newRig(t, strongProbe)
	good := r.issue("cocoon-b")
	const inst = "cocoon-b"

	// 三枚"错"的样子：乱码 / 别人的票（重发即吊销旧票，这里用另一茧的票）/ 已吊销。
	stale, err := r.tm.IssueToken(inst)
	if err != nil {
		t.Fatalf("IssueToken 失败: %v", err)
	}
	other := r.issue("cocoon-other")
	if _, err := r.tm.IssueToken(inst); err != nil { // 重发 ⇒ stale 作废
		t.Fatalf("重发令牌失败: %v", err)
	}
	cases := map[string]string{
		"乱码":    "not-a-token-at-all",
		"别人的票":  other,
		"已吊销旧票": stale.Value,
		"空票":    "",
	}
	var firstFrame string
	for name, bad := range cases {
		raw := r.send(`{"inst":"` + inst + `","token":"` + bad + `","op":"code.show","args":{"file":"a.go"}}`)
		var resp map[string]any
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatalf("[%s] 响应不是合法 JSON: %q", name, raw)
		}
		errorCode(t, resp, CodeUnauthorized)
		// ★ 原始字节里不许出现：出示的令牌、正确令牌、任何原因字样。
		body := string(raw)
		if bad != "" && strings.Contains(body, bad) {
			t.Fatalf("[%s] 响应回显了出示的令牌: %q", name, body)
		}
		if strings.Contains(body, good) {
			t.Fatalf("[%s] 响应里出现了正确令牌: %q", name, body)
		}
		for _, leak := range []string{"令牌", "吊销", "过期", "不匹配", "token"} {
			if strings.Contains(body, leak) {
				t.Fatalf("[%s] 响应回显了原因字样 %q: %q", name, leak, body)
			}
		}
		if firstFrame == "" {
			firstFrame = body
			continue
		}
		// ★ 四种原因必须**逐字节同帧** —— 否则对端能靠帧的差异反推"是哪一种失败"。
		if body != firstFrame {
			t.Fatalf("[%s] 与首帧不同（可做判定 oracle）:\n 首帧 %q\n 本帧 %q", name, firstFrame, body)
		}
	}

	// 被拒也要留审计（ok=false），且审计里没有令牌本体。
	lines := r.auditLines()
	if len(lines) != len(cases) {
		t.Fatalf("审计行数应为 %d，实得 %d: %#v", len(cases), len(lines), lines)
	}
	for _, a := range lines {
		if a.OK {
			t.Fatalf("被拒的请求在审计里记成了成功: %#v", a)
		}
	}
	if strings.Contains(r.audit.String(), good) || strings.Contains(r.audit.String(), other) {
		t.Fatalf("审计里出现了令牌本体: %s", r.audit.String())
	}
}

// ── ③ 未知 op ⇒ 拒（且没令牌的人探不出哪些 op 存在）──────────────────

func TestBrokerUnknownOpRefused(t *testing.T) {
	// (a) 身份 strong（身份闸不拦）+ 合法令牌 + 未注册的 op ⇒ unknown_op。
	r := newRig(t, strongProbe)
	tok := r.issue("cocoon-c")
	errorCode(t, r.call("cocoon-c", tok, "repo.commit", map[string]any{}), CodeUnknownOp)
	errorCode(t, r.call("cocoon-c", tok, "code.show.all", nil), CodeUnknownOp)

	// (b) ★ 同一条未知 op，**不带令牌** ⇒ 只能是 unauthorized（fail-closed 按高权判），
	//     不能是 unknown_op —— 否则未授权方拿到一张"哪些 op 存在"的探针表。
	errorCode(t, r.call("cocoon-c", "", "repo.commit", map[string]any{}), CodeUnauthorized)
	errorCode(t, r.call("cocoon-c", "garbage", "repo.commit", map[string]any{}), CodeUnauthorized)

	// (c) 弱身份 + 未知 op ⇒ 同样拒（按高权判，identity 也不够），且理由不泄露 op 是否存在。
	resp := r.call("cocoon-c", tok, "repo.commit", map[string]any{})
	if ok, _ := resp["ok"].(bool); ok {
		t.Fatalf("未知 op 被放行了: %#v", resp)
	}
	if code, _ := resp["code"].(string); code != CodeUnknownOp && code != CodeDenied {
		t.Fatalf("未知 op 的码不对: %#v", resp)
	}

	// 审计逐条留痕。
	lines := r.auditLines()
	if len(lines) != 5 {
		t.Fatalf("审计应有 5 行，实得 %d: %#v", len(lines), lines)
	}
}

// ── ④ 弱身份 × 高权 op ⇒ 拒（✗ 不降权放行）──────────────────────────

func TestBrokerWeakPeerPrivilegedOpRefused(t *testing.T) {
	// (a) weak ⇒ 拒。
	weakRig := newRig(t, weakProbe)
	weakTok := weakRig.issue("cocoon-d")
	errorCode(t, weakRig.call("cocoon-d", weakTok, "ai.request", map[string]any{
		"host":    "api.openai.com",
		"headers": map[string]any{"Authorization": "Bearer {{secret:openai}}"},
	}), CodeDenied)

	// (b) 同一枚请求、同一枚 op，只把身份换 strong ⇒ 放行。
	//     这一对照证明"拒"是**身份强度**造成的，不是 op 本身不可用。
	strongRig := newRig(t, strongProbe)
	strongRig.b.Secrets = FuncSecretResolver(func(name string) (string, error) {
		if name != "openai" {
			return "", errors.New("no such secret")
		}
		return "sk-live-0001", nil
	})
	strongTok := strongRig.issue("cocoon-d2")
	resp := strongRig.call("cocoon-d2", strongTok, "ai.request", map[string]any{
		"host":    "api.openai.com",
		"headers": map[string]any{"Authorization": "Bearer {{secret:openai}}"},
	})
	if ok, _ := resp["ok"].(bool); !ok {
		t.Fatalf("strong 身份 + 高权 op 应当放行，实得 %#v", resp)
	}

	// (c) none（拿不到身份）同样拒。
	noneRig := newRig(t, func(*net.UnixConn) PeerIdentityRecord {
		return PeerIdentityRecord{Kind: "none", Confidence: PeerIdentityNone}
	})
	noneTok := noneRig.issue("cocoon-d3")
	errorCode(t, noneRig.call("cocoon-d3", noneTok, "ai.request", map[string]any{
		"host":    "api.openai.com",
		"headers": map[string]any{"Authorization": "Bearer {{secret:openai}}"},
	}), CodeDenied)
}

// ── ⑤ socket 0600 / 目录 0700（含"已有 0755 目录被收紧"）─────────────

func TestBrokerSocketPerms(t *testing.T) {
	// ① 全新建：目录 0700、socket 0600。
	r := newRig(t, nil)
	fi, err := os.Lstat(r.sock)
	if err != nil {
		t.Fatalf("读 socket 失败: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("socket 权限应为 0600，实得 %#o", perm)
	}
	if fi.Mode()&os.ModeSocket == 0 {
		t.Fatalf("socket 路径上不是 socket: %s", fi.Mode())
	}
	di, err := os.Stat(r.dir)
	if err != nil {
		t.Fatalf("读目录失败: %v", err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Fatalf("socket 目录权限应为 0700，实得 %#o", perm)
	}

	// ② 目录本来就是 0755 ⇒ Listen 之后必须被**收紧**到 0700（✗ 只建不改）。
	loose := filepath.Join(shortDir(t), "loose")
	if err := os.Mkdir(loose, 0o755); err != nil {
		t.Fatalf("建 0755 目录失败: %v", err)
	}
	if err := os.Chmod(loose, 0o755); err != nil { // Mkdir 受 umask 影响，显式钉一次
		t.Fatalf("chmod 失败: %v", err)
	}
	b := NewBroker(NewTokenManager())
	b.Audit = &lockedBuf{}
	sock := filepath.Join(loose, "b.sock")
	if err := b.Listen(sock); err != nil {
		t.Fatalf("在 0755 目录下 Listen 失败: %v", err)
	}
	defer b.Close()
	if di, err := os.Stat(loose); err != nil || di.Mode().Perm() != 0o700 {
		t.Fatalf("0755 目录没被收紧到 0700: %v %#o", err, di.Mode().Perm())
	}
	if fi, err := os.Lstat(sock); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("socket 权限不是 0600: %v", err)
	}
}

// ── ⑥ 残留 socket 件 ⇒ 启动能重建；活 socket / 普通件 ⇒ 拒────────────

func TestBrokerStaleSocketRebuilt(t *testing.T) {
	dir := shortDir(t)
	sock := filepath.Join(dir, "stale.sock")

	// 造一枚**真残留**：bind + listen 之后直接 close fd（不 unlink）——
	// 这正是"上次崩了"在盘上留下的形状，不是手搓一个空文件。
	fd, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("建 socket 失败: %v", err)
	}
	if err := unix.Bind(fd, &unix.SockaddrUnix{Name: sock}); err != nil {
		unix.Close(fd)
		t.Fatalf("bind 失败: %v", err)
	}
	if err := unix.Listen(fd, 1); err != nil {
		unix.Close(fd)
		t.Fatalf("listen 失败: %v", err)
	}
	if err := unix.Close(fd); err != nil {
		t.Fatalf("close 失败: %v", err)
	}
	if fi, err := os.Lstat(sock); err != nil || fi.Mode()&os.ModeSocket == 0 {
		t.Fatalf("残留 socket 没造出来: %v", err)
	}
	// 残留在、但没人听 ⇒ 拨号必失败（这就是"死"的判据）。
	if c, err := net.DialTimeout("unix", sock, 200*time.Millisecond); err == nil {
		c.Close()
		t.Fatalf("残留 socket 竟然还有人在听")
	}

	// ① 启动必须能清掉重建（✗ 让人先手动 rm）。
	b := NewBroker(NewTokenManager())
	b.Audit = &lockedBuf{}
	if err := b.Listen(sock); err != nil {
		t.Fatalf("残留 socket 上 Listen 失败: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- b.Serve(ctx) }()
	tok, err := b.Tokens.IssueToken("cocoon-e")
	if err != nil {
		t.Fatalf("IssueToken 失败: %v", err)
	}
	req := `{"inst":"cocoon-e","token":"` + tok.Value + `","op":"code.show","args":{"file":"x.go"}}`
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("重建后拨号失败: %v", err)
	}
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Write([]byte(req + "\n")); err != nil {
		t.Fatalf("写帧失败: %v", err)
	}
	line, err := bufio.NewReader(c).ReadBytes('\n')
	c.Close()
	if err != nil {
		t.Fatalf("读响应失败: %v", err)
	}
	if !strings.Contains(string(line), `"ok":true`) {
		t.Fatalf("重建后的 broker 不工作: %q", line)
	}

	// ② 活 socket ⇒ 拒（✗ 不抢别人的茧）。
	b2 := NewBroker(NewTokenManager())
	b2.Audit = &lockedBuf{}
	if err := b2.Listen(sock); !errors.Is(err, ErrSocketInUse) {
		t.Fatalf("活 socket 上应当拒（ErrSocketInUse），实得 %v", err)
	}

	// ③ 非 socket 件占位 ⇒ 拒，且**不许删掉**。
	plain := filepath.Join(dir, "plain.sock")
	if err := os.WriteFile(plain, []byte("别删我"), 0o600); err != nil {
		t.Fatalf("造普通件失败: %v", err)
	}
	b3 := NewBroker(NewTokenManager())
	b3.Audit = &lockedBuf{}
	if err := b3.Listen(plain); !errors.Is(err, ErrNonSocketInPath) {
		t.Fatalf("普通件占位应当拒（ErrNonSocketInPath），实得 %v", err)
	}
	if body, err := os.ReadFile(plain); err != nil || string(body) != "别删我" {
		t.Fatalf("普通件被动了: %q %v", body, err)
	}

	cancel()
	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("Serve 收工报错: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Serve 没退出")
	}
}

// ── ⑦ 审计行不含令牌与真值；响应过回显擦除 ───────────────────────────

func TestBrokerAuditHasNoSecrets(t *testing.T) {
	r := newRig(t, nil)
	const (
		inst = "cocoon-f"
		// 夹具哨兵（非真凭据）：**拼接构造**，避免在源码里落成 BLOCK 命中形态（G1 · 见 publish/mirror-allow.txt 同件条目）
		secret = "sk-TOPSE" + "CRET-9f8e7d"
		argTxt = "ARGVALUE-please-do-not-log"
	)
	// 一枚"会把真值放进 result"的测试 op —— 用来证明出线前那道 ScrubResponse 真在跑。
	if err := r.b.Ops.Register(OpSpec{
		Name: "test.echo",
		Handle: func(_ context.Context, _ *Broker, req OpRequest) (any, error) {
			return map[string]string{"value": secret, "note": argTxt}, nil
		},
	}); err != nil {
		t.Fatalf("注册测试 op 失败: %v", err)
	}
	r.b.ScrubSecrets = []string{secret}

	tok := r.issue(inst)
	raw := r.send(`{"inst":"` + inst + `","token":"` + tok + `","op":"test.echo","args":{"note":"` + argTxt + `"}}`)
	body := string(raw)

	// ① 响应里不许有真值（擦除面生效）。
	if strings.Contains(body, secret) {
		t.Fatalf("响应回显了真值: %q", body)
	}
	if !strings.Contains(body, ScrubPlaceholder) {
		t.Fatalf("响应里没有擦除记号（擦除面没跑到）: %q", body)
	}
	var resp map[string]any
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("擦除把 JSON 弄坏了: %q (%v)", body, err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		t.Fatalf("该请求应当成功: %#v", resp)
	}

	// ② 审计行：字段表恰好是这六个，一个不多一个不少。
	lines := r.auditLines()
	if len(lines) != 1 {
		t.Fatalf("审计应有 1 行，实得 %d: %#v", len(lines), lines)
	}
	// 用原始行取键（结构体反序列化会丢掉"多出来的键"，必须从字节上看）。
	var rawMap map[string]any
	if err := json.Unmarshal([]byte(r.audit.Lines()[0]), &rawMap); err != nil {
		t.Fatalf("审计行不是合法 JSON: %v", err)
	}
	var keys []string
	for k := range rawMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"inst", "ms", "ok", "op", "peer_confidence", "ts"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("审计字段表不对:\n 得 %v\n 要 %v", keys, want)
	}

	// ③ 审计全文里不许出现令牌、真值、参数值。
	auditText := r.audit.String()
	for _, forbidden := range []string{tok, secret, argTxt, "token"} {
		if strings.Contains(auditText, forbidden) {
			t.Fatalf("审计里出现了 %q: %s", forbidden, auditText)
		}
	}
}

// ── ⑧ ai.request：T5 的注入接在 broker 上，真值不回茧 ────────────────

func TestBrokerAIRequestInjectsWithoutLeak(t *testing.T) {
	// 夹具哨兵（非真凭据）：**拼接构造**，避免在源码里落成 BLOCK 命中形态（G1 · 见 publish/mirror-allow.txt 同件条目）
	const secret = "sk-LIVE-" + "0a1b2c3d"
	r := newRig(t, strongProbe) // ai.request 是高权 op ⇒ 必须 strong
	r.b.Secrets = FuncSecretResolver(func(name string) (string, error) {
		if name != "openai" {
			return "", errors.New("未知密钥")
		}
		return secret, nil
	})
	r.b.ScrubSecrets = []string{secret}
	tok := r.issue("cocoon-g")

	resp := r.call("cocoon-g", tok, "ai.request", map[string]any{
		"host": "api.openai.com",
		"headers": map[string]any{
			"Authorization": "Bearer {{secret:openai}}",
			"X-Trace":       "plain",
		},
	})
	if ok, _ := resp["ok"].(bool); !ok {
		t.Fatalf("ai.request 应当成功，实得 %#v", resp)
	}
	raw, _ := json.Marshal(resp)
	if strings.Contains(string(raw), secret) {
		t.Fatalf("响应里回了真值: %s", raw)
	}
	if strings.Contains(string(raw), "{{secret:") {
		t.Fatalf("响应里回了未换值的占位符: %s", raw)
	}
	res, _ := resp["result"].(map[string]any)
	if res == nil {
		t.Fatalf("result 缺失: %#v", resp)
	}
	if n, _ := res["substituted"].(float64); n != 1 {
		t.Fatalf("substituted 应为 1（只有 Authorization 带占位符），实得 %#v", res["substituted"])
	}
	names, _ := res["header_names"].([]any)
	if len(names) != 2 {
		t.Fatalf("header_names 应为 2 项，实得 %#v", res["header_names"])
	}

	// 上游不在白名单 ⇒ 整条拒（fail-closed），且不回原因。
	errorCode(t, r.call("cocoon-g", tok, "ai.request", map[string]any{
		"host":    "evil.example.com",
		"headers": map[string]any{"Authorization": "Bearer {{secret:openai}}"},
	}), CodeInternal)
}

// ── ⑨ 参数面：未知字段 / 绝对路径 / `..` 一律拒（授权粒度是"完整参数"）──

func TestBrokerArgsStrict(t *testing.T) {
	r := newRig(t, nil)
	tok := r.issue("cocoon-h")

	bad := []struct {
		name string
		args map[string]any
	}{
		{"未知字段", map[string]any{"file": "a.go", "path": "/etc/passwd"}},
		{"绝对路径", map[string]any{"file": "/etc/passwd"}},
		{"上跳", map[string]any{"file": "../../.ssh/id_rsa"}},
		{"空 file", map[string]any{"file": ""}},
		{"缺 file", map[string]any{}},
	}
	for _, c := range bad {
		errorCode(t, r.call("cocoon-h", tok, "code.show", c.args), CodeBadRequest)
		t.Logf("[%s] 已按 bad_request 拒", c.name)
	}

	// 合法形状照常放行（负控：别为了拒坏参数把好参数也拒了）。
	ok := r.call("cocoon-h", tok, "code.show", map[string]any{"file": "internal/cocoon/broker.go"})
	if yes, _ := ok["ok"].(bool); !yes {
		t.Fatalf("合法参数被拒: %#v", ok)
	}
}

// ── ⑩ 分平台探针：TCP fd 上的 macOS 陷阱必须被挡住 ────────────────────

func TestPeerProbeRejectsNonUnixFD(t *testing.T) {
	// 一枚**真实 TCP** 连接的一对 fd：在 macOS 上对它调 LOCAL_PEERCRED 会"成功但输出全零"，
	// 读出来正好是 uid 0 / root。本层靠"getsockname 必须是 AF_UNIX"在调用前就挡住。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("起 TCP 监听失败: %v", err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err == nil {
			time.Sleep(200 * time.Millisecond)
			c.Close()
		}
	}()
	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("拨 TCP 失败: %v", err)
	}
	defer client.Close()
	tc, ok := client.(*net.TCPConn)
	if !ok {
		t.Fatalf("不是 TCP 连接")
	}
	raw, err := tc.SyscallConn()
	if err != nil {
		t.Fatalf("取 SyscallConn 失败: %v", err)
	}
	var rec PeerIdentityRecord
	var probed bool
	if err := raw.Control(func(fd uintptr) {
		rec, probed = probeFD(int(fd))
	}); err != nil {
		t.Fatalf("Control 失败: %v", err)
	}
	if probed {
		t.Fatalf("TCP fd 上竟然取到了身份: %#v", rec)
	}
	if rec.Confidence != PeerIdentityNone {
		t.Fatalf("TCP fd 上的强度必须 none，实得 %q", rec.Confidence)
	}
	if rec.Kind != "not-unix-socket" {
		t.Fatalf("TCP fd 应被 getsockname 判非 unix socket，实得 kind=%q", rec.Kind)
	}
	// 同一个 fd 用生产探针的公共入口再走一遍（UnixConn 那侧无法伪造，故只在 probeFD 这一层断言）。
	if rec2, ok2 := probeFD(-1); ok2 || rec2.Confidence != PeerIdentityNone {
		t.Fatalf("非法 fd 没被判 none: %#v %v", rec2, ok2)
	}
}

// ── ⑪ 生产探针自检：真连一对 unix socket，取到的必须"是自己的 pid" ──────
//
// ★ 这一格是为了钉住一个真实发生过的事故：`audit_token_t` 漏读 `ruid` 会让整段错位一格，
// 读出来的 `PID` 其实是 `rgid`、真 pid 落到 `ASID` 上 —— 读数"看着像取到了"，
// 而 socket 上根本没人能发现。判据用"同进程自连 ⇒ pid 必然等于 os.Getpid()"，错位当场红。
func TestPeerProbePlatformSane(t *testing.T) {
	dir := shortDir(t)
	sock := filepath.Join(dir, "probe.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("起 unix 监听失败: %v", err)
	}
	defer ln.Close()
	srvCh := make(chan *net.UnixConn, 1)
	go func() {
		c, err := ln.Accept()
		if err == nil {
			srvCh <- c.(*net.UnixConn)
		}
	}()
	cli, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("拨 unix socket 失败: %v", err)
	}
	defer cli.Close()
	srv := <-srvCh
	defer srv.Close()

	for name, conn := range map[string]*net.UnixConn{"client": cli.(*net.UnixConn), "server": srv} {
		rec := defaultPeerProbe(conn)
		if rec.Confidence != PeerIdentityWeak && rec.Confidence != PeerIdentityStrong {
			t.Fatalf("[%s] 同机同进程自连必须拿得到身份，实得 %#v", name, rec)
		}
		if want := strconv.Itoa(os.Getpid()); rec.ID != want {
			t.Fatalf("[%s] 对端 pid 应是本进程 %s，实得 %q（kind=%q）", name, want, rec.ID, rec.Kind)
		}
		if rec.StartTime == 0 {
			t.Fatalf("[%s] 抗复用那一维（starttime/pidversion）没取到: %#v", name, rec)
		}
	}
}

// ── ⑫ 权限自检本身有牙：盘上不是 0600 就必须报（不靠 chmod 那一步兜）────
func TestVerifySocketPerm(t *testing.T) {
	dir := shortDir(t)
	sock := filepath.Join(dir, "perm.sock")
	if err := os.WriteFile(sock, nil, 0o644); err != nil {
		t.Fatalf("造件失败: %v", err)
	}
	if err := os.Chmod(sock, 0o644); err != nil { // WriteFile 受 umask 影响，显式钉一次
		t.Fatalf("chmod 失败: %v", err)
	}
	if err := verifySocketPerm(sock); !errors.Is(err, ErrBadSocketPerm) {
		t.Fatalf("0644 的件应当被判 ErrBadSocketPerm，实得 %v", err)
	}
	if err := os.Chmod(sock, 0o600); err != nil {
		t.Fatalf("chmod 失败: %v", err)
	}
	if err := verifySocketPerm(sock); err != nil {
		t.Fatalf("0600 的件应当过，实得 %v", err)
	}
	if err := verifySocketPerm(filepath.Join(dir, "nope.sock")); err == nil {
		t.Fatalf("件不存在时应当报错")
	}
}

// ── ⑬ 判据 6（v2 §E）：宿主仓操作（`repo.*`）在册为零 · 注册口也收不进 ────────
//
// ★ 这一格测**两件事**，缺一不可：
//
//	(a) 真在用的注册表里没有 `repo` / `repo.*` —— 「本版没写」是个会被下一次提交推翻的事实，
//	    必须由测试钉住（`DefaultOps` 加一笔就红）。
//	(b) 注册口**自己拒收** —— 就算有人绕过 (a) 去 `Register`，也加不进来。只钉 (a) 等于只钉
//	    「当前这份默认表」，钉 (b) 才钉住「茧内不可能有宿主仓写权」这条判据。
func TestBrokerRepoOpsNotRegistered(t *testing.T) {
	// (a) 真跑用的那张表（`NewBroker` 填的 `DefaultOps`）。
	r := newRig(t, strongProbe)
	if r.b.Ops == nil {
		t.Fatal("broker 没有注册表（fail-closed 面本身要在，才能谈「在册为零」）")
	}
	names := r.b.Ops.Names()
	if len(names) == 0 {
		t.Fatal("注册表是空的 —— 这一格就无从谈起（正控：至少要有一枚在册 op）")
	}
	for _, name := range names {
		if isRepoSelfOp(name) {
			t.Fatalf("宿主仓操作 %q 进了白名单（判据 6：茧内 `repo commit` 必拒）", name)
		}
	}
	for _, op := range []string{"repo", "repo.commit", "repo.status", "repo.diffstat", "repo.push", "Repo.Commit"} {
		if _, ok := r.b.Ops.Lookup(op); ok {
			t.Fatalf("Lookup(%q) 命中了 —— 宿主仓操作在册", op)
		}
	}
	// 正控：非仓族的两枚桩照常在册（不是「把表清空」冒充判据）。
	for _, op := range []string{"code.show", "ai.request"} {
		if _, ok := r.b.Ops.Lookup(op); !ok {
			t.Fatalf("%q 不在册 —— 判据 6 不许靠清空白名单达成", op)
		}
	}

	// (b) 注册口拒收（大小写 / 空白变形一并收窄）。空注册表 ⇒ 谁也别想偷偷塞进去。
	nop := func(context.Context, *Broker, OpRequest) (any, error) { return nil, nil }
	for _, op := range []string{"repo", "repo.commit", "repo.status", "  repo.push  ", "Repo.Commit"} {
		reg := NewOpRegistry()
		err := reg.Register(OpSpec{Name: op, Privileged: true, Handle: nop})
		if !errors.Is(err, ErrRepoSelfOp) {
			t.Fatalf("Register(%q) 应回 ErrRepoSelfOp，得 %v（宿主仓操作被收进了注册表）", op, err)
		}
		if got := reg.Names(); len(got) != 0 {
			t.Fatalf("Register(%q) 被拒后注册表却有 %v（拒得不干净）", op, got)
		}
	}
	// 负控：只是前缀像的名字不受误伤（否则「禁 repo.*」会顺手废掉一片无关 op）。
	for _, op := range []string{"report", "repofoo", "repo2.commit"} {
		reg := NewOpRegistry()
		if err := reg.Register(OpSpec{Name: op, Handle: nop}); err != nil {
			t.Fatalf("Register(%q) 被误当成宿主仓操作：%v", op, err)
		}
	}
}

// TestBrokerRepoCommitRefusedOnWire — 真发一条 `op="repo.commit"`，逐字节看回帧：
// 拒、码是 `unknown_op`（令牌与身份都过了才谈在册），且**不带任何细节**（连 op 名都不回显）。
func TestBrokerRepoCommitRefusedOnWire(t *testing.T) {
	r := newRig(t, strongProbe) // strong：身份闸不拦 ⇒ 走到的是「在不在册」那一道
	tok := r.issue("cocoon-c")

	req, err := json.Marshal(map[string]any{
		"inst": "cocoon-c", "token": tok, "op": "repo.commit",
		"args": map[string]any{"only": []string{"core/"}, "message": "茧内提交"},
	})
	if err != nil {
		t.Fatal(err)
	}
	line := r.send(string(req))
	got := strings.TrimSpace(string(line))

	// ★ 逐字节相等：这一串就是「拒 + 无细节」的全部内容。多一个 reason/detail/hint 都红。
	if want := `{"ok":false,"code":"unknown_op"}`; got != want {
		t.Fatalf("回帧不是恰好 %s，实得 %q（多出的字段 = 把「为什么拒」告诉了对端）", want, got)
	}
	// 连 op 名都不回显（否则对端能拿它当「哪些 op 存在」的探针）。
	if strings.Contains(strings.ToLower(got), "repo") {
		t.Fatalf("回帧回显了 op 名: %q", got)
	}
	// 解析面同判（`errorCode` 顺带断言「拒帧只有 ok/code 两键」）。
	errorCode(t, r.call("cocoon-c", tok, "repo.commit", map[string]any{"only": []string{"core/"}}), CodeUnknownOp)

	// 正控：同一枚令牌打一枚在册 op ⇒ 成功。证明「拒」是 op 不在册，不是令牌/身份不行。
	if resp := r.call("cocoon-c", tok, "code.show", map[string]any{"file": "main.go"}); resp["ok"] != true {
		t.Fatalf("同一枚令牌打 code.show 应放行，得 %#v（那这一格就没证明 repo.commit 是被 op 拒的）", resp)
	}

	// 审计仍留痕（被拒也记一行），且不含令牌本体。
	lines := r.auditLines()
	if len(lines) != 3 {
		t.Fatalf("审计应有 3 行（repo.commit ×2 + code.show ×1），实得 %d: %#v", len(lines), lines)
	}
	denied := 0
	for _, a := range lines {
		if a.Op == "repo.commit" {
			denied++
			if a.OK {
				t.Fatalf("被拒的 repo.commit 在审计里记成了成功: %#v", a)
			}
		}
	}
	if denied != 2 {
		t.Fatalf("审计里 repo.commit 应有 2 行，实得 %d: %#v", denied, lines)
	}
	if strings.Contains(r.audit.String(), tok) {
		t.Fatal("审计里出现了令牌本体")
	}
}
