// tokencap_audit_test.go —— GAP-20261021-05：**审计行本身**的端到端判据（clamp / reject / 不越限 三态）。
//
// 为什么另起一件：tokencap_test.go ⑥ 只验到「ApplyAdapterOverrides 把 Limit.Clamped/Want/Value/Cap
// 交回调用方」——那是**数据源**；真正要保的是网关在**请求处理链上**把这两行落下来：
//
//	✂️ outbudget audit: … decision=clamp want=… value=… cap=…
//	⛔ outbudget fail-closed: … decision=reject … | <reason>
//
// 数据源对了、接线断了（例如 tokencap.go 末尾 `return forwardBody, bud` 回成零值），
// 上面两行就会**静默消失**而所有纯函数用例照旧全绿。故本件的判据是**日志原文**，并且走
// handleRequest（真实入站处理函数 + 假子端），不是只调 ApplyAdapterOverrides。
package gateway

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

const auditTestModel = "tokencap-audit-test"

// auditUpstream —— 假子端：记下收到的请求体（证明"这发真的转发了"），回一个最小 OpenAI 兼容响应。
type auditUpstream struct {
	srv *httptest.Server

	mu     sync.Mutex
	bodies [][]byte
}

func newAuditUpstream(t *testing.T) *auditUpstream {
	t.Helper()
	u := &auditUpstream{}
	u.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		u.mu.Lock()
		u.bodies = append(u.bodies, buf.Bytes())
		u.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_audit","object":"chat.completion","created":1,` +
			`"model":"` + auditTestModel + `","choices":[{"index":0,` +
			`"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],` +
			`"usage":{"total_tokens":3}}`))
	}))
	t.Cleanup(u.srv.Close)
	return u
}

func (u *auditUpstream) calls() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.bodies)
}

// auditGateway —— 该模型带 fleet 声明（ctx_window / max_tokens），适配器声明 max_tokens=32768
// （mockAdapter）⇒ 进预算分支；上限取 fleet 的声明档。
func auditGateway(t *testing.T, u *auditUpstream, ctxWindow, fleetMax int) *Gateway {
	t.Helper()
	raw := u.srv.URL
	host := strings.TrimPrefix(raw, "http://")
	host, portStr, _ := strings.Cut(host, ":")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("假子端端口不是数字：%v", err)
	}
	cfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			auditTestModel: {{
				Host:      "node-a",
				Backend:   "ds4-server",
				File:      "/data/tokencap-audit-test.gguf",
				CtxWindow: ctxWindow,
				MaxTokens: fleetMax,
			}},
		},
		Fleet: map[string]config.FleetNode{
			"node-a": {Host: host, Port: port},
		},
	}
	return NewGateway("tok", cfg, store.NewStore(), map[string]plugin.Plugin{auditTestModel: &mockAdapter{}})
}

// auditBody —— 长度**恰为** want 字节的 chat 请求体（content 全是 ASCII 'x' ⇒ 长度线性可控）。
// callerMax > 0 ⇒ 带上调用方给的那一格 max_tokens。
func auditBody(t *testing.T, want int, session string, callerMax int) []byte {
	t.Helper()
	field := ""
	if callerMax > 0 {
		field = `"max_tokens":` + strconv.Itoa(callerMax) + `,`
	}
	pre := `{"model":"` + auditTestModel + `","session_id":"` + session + `",` + field +
		`"messages":[{"role":"user","content":"`
	post := `"}]}`
	n := want - len(pre) - len(post)
	if n < 0 {
		t.Fatalf("夹具 want=%d 比骨架（%d 字节）还短", want, len(pre)+len(post))
	}
	b := []byte(pre + strings.Repeat("x", n) + post)
	if len(b) != want {
		t.Fatalf("夹具长度没对齐：want=%d 实得=%d", want, len(b))
	}
	return b
}

// postAudit —— 走**真实处理链**（handleRequest），返回响应与本次请求落下的全部日志原文。
func postAudit(t *testing.T, g *Gateway, session string, body []byte) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var buf bytes.Buffer
	oldOut := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldOut)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "tok")
	req.Header.Set(sessionHeaderName, session) // 会话号确定 ⇒ 审计行里的 session= 可对拍
	rec := httptest.NewRecorder()
	g.handleRequest(rec, req)
	return rec, buf.String()
}

// 正控甲：钳位路径 ⇒ 必须**真落一行** decision=clamp，且 want/value/cap 三格都是非零真实值。
func TestTokencapAudit_ClampLineLands(t *testing.T) {
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir()) // 无档案 ⇒ 上限取 fleet 声明
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	up := newAuditUpstream(t)
	// ctx=524288 ⇒ 额度充足（不会走拒发），fleet 声明上限 262144
	g := auditGateway(t, up, 524288, 262144)

	const sess = "audit-clamp-sess"
	body := auditBody(t, 600, sess, 999999) // 调用方要 999999 > 上限 262144 ⇒ 钳位
	rec, logs := postAudit(t, g, sess, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("钳位不是拒发：应照常转发得 200，实得 %d（body=%s）\n日志:\n%s", rec.Code, rec.Body.String(), logs)
	}
	if up.calls() != 1 {
		t.Fatalf("钳位那发应照常转发到子端一次，实得 %d 次", up.calls())
	}
	// 真值必须进到子端收到的请求体（审计行不是纸上数字）
	var sent map[string]any
	if err := json.Unmarshal(up.bodies[0], &sent); err != nil {
		t.Fatalf("子端收到的请求体不是合法 JSON：%v", err)
	}
	if v, _ := sent["max_tokens"].(float64); int(v) != 262144 {
		t.Fatalf("子端收到的 max_tokens 应为钳位值 262144，实得 %v", sent["max_tokens"])
	}

	m := regexp.MustCompile(`decision=clamp want=(\d+) value=(\d+) cap=(\d+)`).FindStringSubmatch(logs)
	if m == nil {
		t.Fatalf("钳位必须落一行 decision=clamp（want/value/cap 三格齐）；实际日志:\n%s", logs)
	}
	want, _ := strconv.Atoi(m[1])
	value, _ := strconv.Atoi(m[2])
	capv, _ := strconv.Atoi(m[3])
	// ★ 三格必须都是**非零真实值**（回零值的接线缺陷正是本用例要咬住的）
	if want <= 0 || value <= 0 || capv <= 0 {
		t.Fatalf("审计行三格不许是零值：want=%d value=%d cap=%d（日志:%s）", want, value, capv, m[0])
	}
	if want != 999999 || value != 262144 || capv != 262144 {
		t.Fatalf("审计行读数不对：want=%d value=%d cap=%d（要 999999/262144/262144）", want, value, capv)
	}
	// 会话面必须可对拍（审计行的意义就是"这一发被截断了"能落到具体会话上）
	if !strings.Contains(logs, "outbudget audit: model="+auditTestModel+" session="+sess) {
		t.Fatalf("审计行缺 model/session 两格（或值不对）:\n%s", logs)
	}
	if !strings.Contains(logs, "threshold=") {
		t.Fatalf("审计行缺 threshold 格:\n%s", logs)
	}
}

// 正控乙：拒发路径 ⇒ 必须**真落一行** decision=reject，且带可读 reason（与错误码）。
func TestTokencapAudit_RejectLineLands(t *testing.T) {
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir())
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	up := newAuditUpstream(t)
	// ctx=8192：reserve=max(819,2048)=2048 ⇒ prompt≈6500 即已超上下文 ⇒ 拒发
	g := auditGateway(t, up, 8192, 32768)

	const sess = "audit-reject-sess"
	body := auditBody(t, 26000, sess, 22438)
	rec, logs := postAudit(t, g, sess, body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("拒发应回 400，实得 %d（body=%s）\n日志:\n%s", rec.Code, rec.Body.String(), logs)
	}
	if up.calls() != 0 {
		t.Fatalf("拒发的那发**一个字节都不许转发**，子端却收到 %d 次", up.calls())
	}
	if !strings.Contains(logs, "decision=reject") {
		t.Fatalf("拒发必须落一行 decision=reject；实际日志:\n%s", logs)
	}
	m := regexp.MustCompile(`outbudget fail-closed: model=(\S+) code=(\S+) session=(\S+) tokens=\d+ threshold=(\d+) decision=reject \| (.+)`).
		FindStringSubmatch(logs)
	if m == nil {
		t.Fatalf("拒发行的形状不对（要 model/code/session/tokens/threshold/decision + reason）:\n%s", logs)
	}
	if m[1] != auditTestModel || m[2] != outBudgetRejectCodeOverCtx || m[3] != sess {
		t.Fatalf("拒发行的 model/code/session 不对：%q/%q/%q", m[1], m[2], m[3])
	}
	if thr, _ := strconv.Atoi(m[4]); thr <= 0 {
		t.Fatalf("拒发行的 threshold 必须是真实值（>0），实得 %q", m[4])
	}
	reason := strings.TrimSpace(m[5])
	if reason == "" {
		t.Fatalf("拒发行必须带 reason（✗ 空尾巴）:\n%s", logs)
	}
	if !strings.Contains(reason, "拒发") || !strings.Contains(reason, "prompt≈") {
		t.Fatalf("拒发理由必须可读（含「拒发」与 prompt≈）：%q", reason)
	}
	// 拒发码必须真的回给调用方（不只是写日志）
	if !strings.Contains(rec.Body.String(), outBudgetRejectCodeOverCtx) {
		t.Fatalf("拒发错误码必须回给调用方：%s", rec.Body.String())
	}
}

// 负控：不越限（调用方 4096 ≤ 上限 262144，额度充足）⇒ **两行都不许出现**。
//
// 这条同时防两种假绿：① 总线/日志里别的用例带的 decision= 串进来；
// ② 预算分支根本没进（那样"没有审计行"是白捡的）—— 故先钉"预算分支真跑过"。
func TestTokencapAudit_NoLineWhenWithinLimit(t *testing.T) {
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir())
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	up := newAuditUpstream(t)
	g := auditGateway(t, up, 524288, 262144)

	const sess = "audit-normal-sess"
	body := auditBody(t, 600, sess, 4096)
	rec, logs := postAudit(t, g, sess, body)

	if rec.Code != http.StatusOK || up.calls() != 1 {
		t.Fatalf("不越限应照常转发 200 ×1，实得 code=%d calls=%d\n日志:\n%s", rec.Code, up.calls(), logs)
	}
	// 前置：这一发**确实**进了预算分支（否则"没有审计行"不算证据）
	if !strings.Contains(logs, "max_tokens 声明值") {
		t.Fatalf("负控失效：这一发根本没进预算分支，日志:\n%s", logs)
	}
	if !strings.Contains(logs, "max_tokens = 4096（来源=调用方(原样)") {
		t.Fatalf("负控失效：调用方的 4096 应按原样写回，日志:\n%s", logs)
	}
	if strings.Contains(logs, "decision=clamp") {
		t.Fatalf("不越限不许落 clamp 审计行:\n%s", logs)
	}
	if strings.Contains(logs, "decision=reject") {
		t.Fatalf("不越限不许落 reject 审计行:\n%s", logs)
	}
}
