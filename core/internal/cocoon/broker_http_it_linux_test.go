//go:build linux

// broker_http_it_linux_test.go — T4b 的**真机三态验收**（真 nsjail · 真 Linux 容器）。
//
// ★ 这一件验的是 T4 留下的那个洞：茧内拿到的 `ZERG_AI_BASE` 必须是 http(s) 回环 URL，
// 而 broker 原来只听 unix socket ⇒ 茧内 curl 到不了（实测 ECONNREFUSED）。三态缺一不可：
//
//	① 宿主 broker 起着 ⇒ 茧内 `curl $ZERG_AI_BASE/health` = **200**，body 里有后端名与模型名；
//	② 茧内 `POST /v1/chat/completions`（一句短提示）⇒ 拿到**上游回复**；且上游那一侧看到的是
//	   **真令牌**、从没见过占位符（判据落在假上游的记账上，不靠茧内自述）；
//	③ **负控**：broker 未起 ⇒ 茧内 curl **连接失败**（拿不到 200，也拿不到任何 HTTP 状态行）
//	   —— 这一条是「① 的 200 不是假绿」的唯一证据。
//
// 前置（缺一样就 Skip 并说清缺什么，✗ 不假绿）：linux ＋ root ＋ 有 nsjail ＋ 茧内镜像里有 curl
// （容器镜像 `zerg-cocoon-it` 有；最小 rootfs 没有 ⇒ 那一路显式 Skip）。
//
// 环境变量：
//
//	ZERG_COCOON_IT_BASE   可选。茧目录父目录（缺省 t.TempDir()）。
//	                       ★ overlay 档要求该盘能当 upper（tmpfs/ext4 才行）；本件不换根，
//	                       所以普通目录就够。
//	ZERG_AI_IT_BROKER_PORT 可选。broker 的 HTTP 端口（缺省 0 = 内核挑）。★ 缺省不用 8082：
//	                       本机/容器里 8082 常被 zerg-core 网关占着，硬绑会「Address already in use」。
package cocoon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// itRealToken — 三态验收里的「真令牌」：只许出现在宿主 broker 与假上游之间。
// ★ 茧内那一侧（stdout / 令牌件 / env）一处都不许有 —— 下面的断言逐条查。
const itRealToken = "sk-t4b-it-REAL-7c1d"

// itUpstream — 假上游（记账：命中次数、收到的 Authorization、最后一条 body）。
type itUpstream struct {
	mu    sync.Mutex
	srv   *httptest.Server
	hits  int
	auth  string
	body  string
	reply string
}

func newITUpstream(t *testing.T) *itUpstream {
	t.Helper()
	u := &itUpstream{reply: `{"id":"chatcmpl-it","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"pong-from-fake-upstream"}}]}`}
	u.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		u.mu.Lock()
		u.hits++
		u.auth = r.Header.Get("Authorization")
		u.body = string(raw)
		u.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(u.reply))
	}))
	t.Cleanup(u.srv.Close)
	return u
}

func (u *itUpstream) snapshot() (int, string, string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.hits, u.auth, u.body
}

// itJailScript — 茧内那一侧的唯一动作：读占位符件 → curl /health → curl chat。
//
// ★ 刻意只用 `$ZERG_AI_BASE` 与 `$ZERG_AI_TOKEN_FILE` 两个 env（茧内能拿到的全部事实），
// 不硬编码地址 —— 这一格验的就是「注入的两个值够不够用」。
const itJailScript = `
set -u
tok=$(cat "$ZERG_AI_TOKEN_FILE")
echo "TOKEN_PLACEHOLDER=$tok"
echo "BASE=$ZERG_AI_BASE"
echo "PIDNS=$(readlink /proc/self/ns/pid)"
# 茧内的 / 在无 rootfs 档是 nsjail 的只读绑定（实测写 /tmp 得 EROFS）⇒ 一律不落临时件：
# 状态码与 body 都从 curl 的 stdout 里取（-w 把码追加到最后一行）。
h=$(curl -sS --max-time 10 -w '\n%{http_code}' "$ZERG_AI_BASE/health" 2>&1); hrc=$?
hcode=$(printf '%s\n' "$h" | tail -n 1); hbody=$(printf '%s\n' "$h" | sed '$d')
[ "$hrc" -eq 0 ] || hcode="curl-fail"
echo "HEALTH_CODE=$hcode"
echo "HEALTH_BODY=$hbody"
payload='{"model":"it","messages":[{"role":"user","content":"ping"}]}'
c=$(curl -sS --max-time 20 -w '\n%{http_code}' \
  -H "Authorization: Bearer $tok" \
  -H 'Content-Type: application/json' -d "$payload" \
  "$ZERG_AI_BASE/v1/chat/completions" 2>&1); crc=$?
ccode=$(printf '%s\n' "$c" | tail -n 1); cbody=$(printf '%s\n' "$c" | sed '$d')
[ "$crc" -eq 0 ] || ccode="curl-fail"
echo "CHAT_CODE=$ccode"
echo "CHAT_BODY=$cbody"
echo "DONE=1"
`

// itRun — 在茧里跑一段 bash，回整份 stdout（判据全从 stdout 的标记行读）。
func itRun(t *testing.T, be *NsjailBackend, inst Instance, env map[string]string, script string) (string, ExecResult) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := be.Exec(ctx, inst, []string{"/bin/bash", "-c", script}, ExecOpts{
		Env:     env,
		Timeout: 45 * time.Second,
	})
	if err != nil {
		t.Fatalf("茧内 Exec 失败（不是判据失败，是这一格没跑起来）: %v", err)
	}
	return string(res.Stdout), res
}

// hostPIDNS — 宿主的 pid namespace（判「茧内真的换了 pidns」的对照值）。
func hostPIDNS() string {
	ns, err := os.Readlink("/proc/self/ns/pid")
	if err != nil {
		return "?"
	}
	return ns
}

// itMarker — 从标记行里取值（`KEY=value`）。
func itMarker(out, key string) (string, bool) {
	prefix := key + "="
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, prefix) {
			return strings.TrimPrefix(ln, prefix), true
		}
	}
	return "", false
}

func TestBrokerHTTPCocoonReachIntegration(t *testing.T) {
	if _, err := exec.LookPath("nsjail"); err != nil {
		t.Skipf("找不到 nsjail（%v）：本格只在真 linux 特权容器里跑", err)
	}
	if os.Geteuid() != 0 {
		t.Skip("需要 root：nsjail 的 namespace/cgroup 要它")
	}
	if _, err := os.Stat("/usr/bin/curl"); err != nil {
		t.Skip("宿主/茧内镜像里没有 /usr/bin/curl：本格的三态判据靠茧内 curl")
	}

	// ── 宿主侧：假上游 ＋（真）broker 的 HTTP 面 ───────────────────────
	up := newITUpstream(t)

	sockDir, err := os.MkdirTemp("/tmp", "t4b")
	if err != nil {
		t.Fatalf("建短路径临时目录失败: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })

	tm := NewTokenManager()
	b := NewBroker(tm)
	b.Secrets = FuncSecretResolver(func(name string) (string, error) {
		if name != DefaultAIUpstreamSecret {
			return "", fmt.Errorf("unexpected secret name %q", name)
		}
		return itRealToken, nil
	})
	auditPath := filepath.Join(sockDir, "audit.jsonl")
	auditFile, err := os.Create(auditPath)
	if err != nil {
		t.Fatalf("建审计件失败: %v", err)
	}
	t.Cleanup(func() { _ = auditFile.Close() })
	b.Audit = auditFile
	if err := b.Listen(filepath.Join(sockDir, "broker.sock")); err != nil {
		t.Fatalf("Listen(unix) 失败: %v", err)
	}
	// ★ 只绑回环；端口缺省由内核挑（8082 常被宿主网关占着）。
	addr, err := b.ListenHTTP(BrokerHTTPOptions{
		Addr:     "127.0.0.1:0",
		Upstream: up.srv.URL,
		Backend:  nsjailBackendName,
		Model:    "it-model",
	})
	if err != nil {
		t.Fatalf("ListenHTTP 失败: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serveErr := make(chan error, 1)
	go func() { serveErr <- b.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-serveErr:
		case <-time.After(10 * time.Second):
			t.Errorf("Serve 10s 内没退出")
		}
	})
	defer func() { _ = b.Close() }()

	// ── 茧内注入面：与命令面同一套算法（BuildAIEnvChecked ＋ WithTokenFile）──
	cocoonDir := filepath.Join(sockDir, "cocoon")
	tokenHost, err := WriteAITokenPlaceholder(cocoonDir)
	if err != nil {
		t.Fatalf("写占位符件失败: %v", err)
	}
	plan, err := BuildAIEnvChecked("it-inst", "it-model", "http://"+addr)
	if err != nil {
		t.Fatalf("BuildAIEnvChecked: %v", err)
	}
	plan, err = plan.WithTokenFile(AITokenInCocoonPath(false, tokenHost))
	if err != nil {
		t.Fatalf("WithTokenFile: %v", err)
	}
	if got := plan.Env[EnvAIBase]; got != "http://"+addr {
		t.Fatalf("注入的 base = %q，want http://%s", got, addr)
	}

	// ── 真 nsjail 茧（不换根档：与 T3 的首选档同形，茧内看得到宿主的 curl）──
	be := NewNsjailBackend()
	be.BaseDir = os.Getenv("ZERG_COCOON_IT_BASE")
	if be.BaseDir == "" {
		be.BaseDir = t.TempDir()
	}
	// 契约面显式要 host（T5）：茧内 127.0.0.1 就是宿主 127.0.0.1 ⇒ 茧内直连宿主 broker 的
	// 回环端口才成立。★ 默认是 none（断网档）：茧内只有自己的回环，宿主 broker 够不着。
	spec := &Spec{File: SpecFileName, Cocoon: CocoonSpec{Name: "t4b-http-it", Image: "debian:bookworm", Net: NetHost}}
	inst, err := be.Provision(context.Background(), spec)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	t.Cleanup(func() { _ = be.Destroy(context.Background(), inst) })

	// ── ① ＋ ②：broker 起着 ────────────────────────────────────────────
	out, res := itRun(t, be, inst, plan.Env, itJailScript)
	if !res.Success() {
		t.Fatalf("茧内脚本 rc=%d（signal=%s timedout=%v）\nstdout=%s\nstderr=%s",
			res.RC, res.Signal, res.TimedOut, out, res.Stderr)
	}
	if !strings.Contains(out, "DONE=1") {
		t.Fatalf("茧内脚本没跑到底：\n%s\nstderr=%s", out, res.Stderr)
	}
	// 真的是茧（不是宿主进程）：pid namespace 与宿主不同。
	if hostNS, herr := os.Readlink("/proc/self/ns/pid"); herr == nil {
		if jailNS, ok := itMarker(out, "PIDNS"); ok && jailNS == hostNS {
			t.Fatalf("茧内 pidns == 宿主 pidns（%s）⇒ 这一格没进 namespace", jailNS)
		}
	}
	// 茧内只拿到占位符。
	if tok, _ := itMarker(out, "TOKEN_PLACEHOLDER"); tok != AITokenPlaceholder {
		t.Fatalf("茧内令牌件内容 = %q，want %q", tok, AITokenPlaceholder)
	}
	if strings.Contains(out, itRealToken) {
		t.Fatalf("真令牌出现在了茧内输出里：\n%s", out)
	}
	// ★ 证据行（报告引的是这些读数，不是「PASS」这三个字母）。
	hcodeEv, _ := itMarker(out, "HEALTH_CODE")
	hbodyEv, _ := itMarker(out, "HEALTH_BODY")
	pidnsEv, _ := itMarker(out, "PIDNS")
	t.Logf("① 茧内 curl %s/health ⇒ %s · body=%s · 茧内 pidns=%s（宿主 %s）",
		plan.Base, hcodeEv, hbodyEv, pidnsEv, hostPIDNS())

	// ① /health = 200 ＋ 一行 JSON（后端名 / 模型名 / 上游是否接着）
	if code, _ := itMarker(out, "HEALTH_CODE"); code != "200" {
		t.Fatalf("① 茧内 curl $ZERG_AI_BASE/health 状态 = %q，want 200\nstdout=%s\nstderr=%s",
			code, out, res.Stderr)
	}
	hb, _ := itMarker(out, "HEALTH_BODY")
	var health map[string]any
	if err := json.Unmarshal([]byte(hb), &health); err != nil {
		t.Fatalf("① /health body 不是 JSON: %q (%v)", hb, err)
	}
	if health["backend"] != nsjailBackendName || health["model"] != "it-model" {
		t.Fatalf("① /health 该报后端名与模型名，实得 %#v", health)
	}
	if uc, _ := health["upstream_configured"].(bool); !uc {
		t.Fatalf("① /health 的 upstream_configured 该为 true，实得 %#v", health["upstream_configured"])
	}
	if strings.Contains(hb, itRealToken) {
		t.Fatalf("① /health 泄漏真令牌: %s", hb)
	}

	chatcodeEv, _ := itMarker(out, "CHAT_CODE")
	chatbodyEv, _ := itMarker(out, "CHAT_BODY")
	upHits, upAuth, upBody := up.snapshot()
	t.Logf("② 茧内 POST /v1/chat/completions ⇒ %s · body=%s", chatcodeEv, chatbodyEv)
	t.Logf("② 假上游（★ 假上游，不是真模型）命中=%d · Authorization=%q（占位符 %q 未到达上游）· body=%s",
		upHits, upAuth, AITokenPlaceholder, upBody)

	// ② chat completions 拿到上游回复；上游收到的是真令牌
	if code, _ := itMarker(out, "CHAT_CODE"); code != "200" {
		t.Fatalf("② 茧内 POST /v1/chat/completions 状态 = %q，want 200\nstdout=%s\nstderr=%s",
			code, out, res.Stderr)
	}
	cb, _ := itMarker(out, "CHAT_BODY")
	if !strings.Contains(cb, "pong-from-fake-upstream") {
		t.Fatalf("② 没拿到上游回复：%q", cb)
	}
	hits, auth, body := up.snapshot()
	if hits != 1 {
		t.Fatalf("② 假上游该被命中恰 1 次，实得 %d", hits)
	}
	if auth != "Bearer "+itRealToken {
		t.Fatalf("② 上游收到的 Authorization = %q，want 真令牌（broker 换值这一跳）", auth)
	}
	if strings.Contains(auth, AITokenPlaceholder) {
		t.Fatalf("② 占位符被发到了上游（%q）", auth)
	}
	if !strings.Contains(body, "ping") {
		t.Fatalf("② 上游没收到那句短提示：%q", body)
	}
	if strings.Contains(cb, itRealToken) {
		t.Fatalf("② 回给茧内的 body 泄漏真令牌：%s", cb)
	}
	// 审计：有 agent 行，且不含真值。
	if raw, rerr := os.ReadFile(auditPath); rerr == nil {
		if strings.Contains(string(raw), itRealToken) {
			t.Fatalf("审计件里出现了真令牌:%s", raw)
		}
	}

	// ── ③ 负控：broker 关掉 ⇒ 茧内 curl 连接失败 ──────────────────────
	if cerr := b.Close(); cerr != nil {
		t.Fatalf("③ 前置：Close broker 失败: %v", cerr)
	}
	out2, res2 := itRun(t, be, inst, plan.Env, itJailScript)
	if !strings.Contains(out2, "DONE=1") {
		t.Fatalf("③ 负控脚本没跑到底：\n%s\nstderr=%s", out2, res2.Stderr)
	}
	h2, _ := itMarker(out2, "HEALTH_CODE")
	c2, _ := itMarker(out2, "CHAT_CODE")
	hb2, _ := itMarker(out2, "HEALTH_BODY")
	t.Logf("③ 负控（broker 已关）：health=%s · chat=%s · curl 的连接错误=%q", h2, c2, hb2)
	if h2 == "200" || c2 == "200" {
		t.Fatalf("③ 负控失败：broker 已关，茧内仍拿到 200（health=%q chat=%q）—— 这就是「假 200」", h2, c2)
	}
	if h2 != "curl-fail" && h2 != "000" {
		t.Fatalf("③ 负控该是**连接失败**（curl-fail/000），实得 health=%q", h2)
	}
	// 连接失败的真凭据：curl 把「连不上」写在 stderr，脚本里 2>&1 并进了 $h ⇒ 落在 HEALTH_BODY。
	if !strings.Contains(out2, "Failed to connect") && !strings.Contains(out2, "Connection refused") {
		t.Fatalf("③ 负控该有连接失败的证据（curl 的 Failed to connect / Connection refused），stdout=%s stderr=%s",
			out2, res2.Stderr)
	}
}
