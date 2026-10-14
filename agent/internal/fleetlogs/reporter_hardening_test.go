// reporter_hardening_test.go —— T5b-d「子端四格（不换队列模型）」的定向单测。
//
// 四格：① `event_id` = ULID（每批生成一次 ＋ **重投复用**）；② 退避 base 1s ×2 / cap 60s（✗ 不封顶）；
// ③ 错误分类（400/401/403/413 **不重试** · 401/403 立即放弃 ＋ 点名）；④ 批 ≤500（超出切多批）。
// ★ 本件 ✗ 不动「失败 ⇒ 游标不前进」语义（那是 `reporter_test.go` 逐字钉死的可重试那一支）。
package fleetlogs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// capturedBody —— 上报体（顶层 machine ＋ logs[]）。
type capturedBody struct {
	Machine string  `json:"machine"`
	Logs    []Entry `json:"logs"`
}

// newCaptureServer —— 假主控：按 codes **依次**回状态码（用尽 ⇒ 停在最后一个），并把每次 body 收进 got。
func newCaptureServer(t *testing.T, codes ...int) (*httptest.Server, *[]capturedBody) {
	t.Helper()
	got := &[]capturedBody{}
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var cb capturedBody
		_ = json.Unmarshal(b, &cb)
		*got = append(*got, cb)
		code := http.StatusOK
		if len(codes) > 0 {
			code = codes[i]
			if i < len(codes)-1 {
				i++
			}
		}
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"ok":true,"stored":0}`))
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

// reporterAt —— 造一台「从 0 起读整份夹具」的上报器（`Start` 默认把游标置到末尾 ⇒ 测试手动归零）。
func reporterAt(t *testing.T, url, logPath string) *Reporter {
	t.Helper()
	r := NewReporter(url, "tok", "node-a", logPath)
	r.mu.Lock()
	r.offset = 0
	r.mu.Unlock()
	return r
}

// writeNLogLines —— 写 n 行 logx 形态日志（供批上限那一格）。
func writeNLogLines(t *testing.T, path string, n int) {
	t.Helper()
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString("2026-10-10T13:00:00.000+08:00 INFO  [agent     ] 夹具行\n")
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("写日志源失败: %v", err)
	}
}

func ids(ls []Entry) []string {
	out := make([]string, len(ls))
	for i := range ls {
		out[i] = ls[i].EventID
	}
	return out
}

// ── ① event_id = ULID（26 字符 · Crockford 字母表）＋ **重投复用** ──────────────────────────

func TestReporter_EventIDIsULIDAndReusedOnRetry(t *testing.T) {
	srv, got := newCaptureServer(t, http.StatusInternalServerError, http.StatusOK) // 第 1 轮 500 ⇒ 可重试
	logPath := filepath.Join(t.TempDir(), "zerg-agent.log")
	writeLog(t, logPath,
		"2026-10-10T13:00:00.000+08:00 INFO  [agent     ] 一行",
		"2026-10-10T13:00:01.000+08:00 INFO  [agent     ] 两行")
	r := reporterAt(t, srv.URL, logPath)

	r.RunOnce() // 第 1 轮：主控 500 ⇒ 可重试 ⇒ 游标**不**前进
	if r.FailCount() != 1 {
		t.Fatalf("第 1 轮 500 应计 1 次失败，实得 %d", r.FailCount())
	}
	if r.Offset() != 0 {
		t.Fatalf("可重试失败后游标不该推进（条目不丢），实得 offset=%d", r.Offset())
	}
	r.RunOnce() // 第 2 轮：主控 200 ⇒ 成功

	if len(*got) != 2 {
		t.Fatalf("应发生 2 次 POST（失败 1 ＋ 成功 1），实得 %d", len(*got))
	}
	if len((*got)[0].Logs) != 2 || len((*got)[1].Logs) != 2 {
		t.Fatalf("两轮各应带 2 条，实得 %d / %d", len((*got)[0].Logs), len((*got)[1].Logs))
	}
	for _, id := range ids((*got)[0].Logs) {
		if len(id) != 26 {
			t.Fatalf("event_id 应是 26 字符的 ULID，实得 %q（%d 字符）", id, len(id))
		}
		for _, c := range id {
			if !strings.ContainsRune(ulidAlphabet, c) {
				t.Fatalf("event_id 含 ULID 字母表外的字符 %q（%q）", string(c), id)
			}
		}
	}
	if a, b := strings.Join(ids((*got)[0].Logs), ","), strings.Join(ids((*got)[1].Logs), ","); a != b {
		t.Fatalf("★ 重投**必须复用同批 event_id**（否则主控 UNIQUE(machine,event_id) 去重永不触发）：\n 第1轮=%s\n 第2轮=%s", a, b)
	}
	if r.Offset() == 0 {
		t.Fatal("成功上报后游标应推进，实得 0")
	}
	// 两枚 id 互不相同（每批逐条各一枚）。
	if ids0 := ids((*got)[0].Logs); ids0[0] == ids0[1] {
		t.Fatalf("同批两条不该共用同一 event_id：%q", ids0[0])
	}
}

// ── ④ 批 ≤500（超出切多批）─────────────────────────────────────────────────────────────

func TestReporter_BatchCapSplitsAt500(t *testing.T) {
	if maxBatch != 500 {
		t.Fatalf("批上限应为 500（§三.5 附 H①），实得 %d", maxBatch)
	}
	srv, got := newCaptureServer(t) // 恒 200
	logPath := filepath.Join(t.TempDir(), "zerg-agent.log")
	writeNLogLines(t, logPath, 501)
	r := reporterAt(t, srv.URL, logPath)

	r.RunOnce()

	if len(*got) != 2 {
		t.Fatalf("501 条一批应**切 2 批**投，实得 %d 批", len(*got))
	}
	if n0, n1 := len((*got)[0].Logs), len((*got)[1].Logs); n0 != 500 || n1 != 1 {
		t.Fatalf("切批应为 500 ＋ 1，实得 %d ＋ %d", n0, n1)
	}
	if r.Offset() == 0 {
		t.Fatal("两批都成 ⇒ 游标应推进，实得 0")
	}
}

// ── ③ 错误分类（400/401/403/413 不重试 · 401/403 立即放弃 ＋ 点名 · 其余重试）──────────────

func TestReporter_PostErrorClassification_Classifier(t *testing.T) {
	cases := []struct {
		code int
		want postOutcome
	}{
		{400, postDrop}, {413, postDrop}, // 永久性失败 ⇒ 不重试
		{401, postGiveUp}, {403, postGiveUp}, // 令牌错 ⇒ 立即放弃
		{404, postRetry}, {429, postRetry}, {500, postRetry}, {503, postRetry}, // 其余重试
	}
	for _, c := range cases {
		if got := classifyPostErr(&postStatusError{code: c.code}); got != c.want {
			t.Errorf("classifyPostErr(HTTP %d) 应 %v，实得 %v", c.code, c.want, got)
		}
	}
	if got := classifyPostErr(nil); got != postOK {
		t.Errorf("无错应 postOK，实得 %v", got)
	}
	// 非状态型错（网络一类）⇒ 重试（✗ 不许把网络错误判成永久性）。
	if got := classifyPostErr(io.ErrUnexpectedEOF); got != postRetry {
		t.Errorf("非状态型错应 postRetry，实得 %v", got)
	}
}

func TestReporter_PostErrorClassification_Behavior(t *testing.T) {
	cases := []struct {
		code        int
		wantDrops   int
		wantFails   int
		wantGiveUp  bool
		wantAdvance bool
	}{
		{400, 1, 0, false, true},  // 永久性失败 ⇒ 丢该批（游标前移）
		{413, 1, 0, false, true},  //
		{401, 0, 0, true, false},  // 令牌错 ⇒ 立即放弃（游标不动）
		{403, 0, 0, true, false},  //
		{404, 0, 1, false, false}, // 其余 ⇒ 重试（游标不前进）
		{429, 0, 1, false, false},
		{500, 0, 1, false, false},
	}
	for _, c := range cases {
		srv, _ := newCaptureServer(t, c.code)
		logPath := filepath.Join(t.TempDir(), "zerg-agent.log")
		writeLog(t, logPath, "2026-10-10T13:00:00.000+08:00 INFO  [agent     ] 一行")
		r := reporterAt(t, srv.URL, logPath)

		r.RunOnce()

		if r.Drops() != c.wantDrops {
			t.Errorf("HTTP %d：Drops 应 %d，实得 %d", c.code, c.wantDrops, r.Drops())
		}
		if r.FailCount() != c.wantFails {
			t.Errorf("HTTP %d：FailCount 应 %d，实得 %d", c.code, c.wantFails, r.FailCount())
		}
		if r.GaveUp() != c.wantGiveUp {
			t.Errorf("HTTP %d：GaveUp 应 %v，实得 %v", c.code, c.wantGiveUp, r.GaveUp())
		}
		if adv := r.Offset() != 0; adv != c.wantAdvance {
			t.Errorf("HTTP %d：游标前移应 %v，实得 %v（offset=%d）", c.code, c.wantAdvance, adv, r.Offset())
		}
	}
}

// 401/403 放弃后**不再试**（第二次 RunOnce 一次 POST 都不发生）。
func TestReporter_GiveUpStopsRetrying(t *testing.T) {
	srv, got := newCaptureServer(t, http.StatusUnauthorized)
	logPath := filepath.Join(t.TempDir(), "zerg-agent.log")
	writeLog(t, logPath, "2026-10-10T13:00:00.000+08:00 INFO  [agent     ] 一行")
	r := reporterAt(t, srv.URL, logPath)

	r.RunOnce()
	if !r.GaveUp() {
		t.Fatal("401 应触发立即放弃")
	}
	r.RunOnce() // 放弃后再调：✗ 不该再发
	if len(*got) != 1 {
		t.Fatalf("放弃后不该再上报（应恒 1 次 POST），实得 %d", len(*got))
	}
}

// ── ② 退避序列 base 1s ×2 / cap 60s（✗ 不封顶）＋ 与 RunOnce/Start 的接线 ─────────────────

func TestReporter_BackoffSequence(t *testing.T) {
	want := []time.Duration{1, 2, 4, 8, 16, 32, 60, 60, 60, 60}
	for i, w := range want {
		if got := nextBackoff(i + 1); got != w*time.Second {
			t.Errorf("nextBackoff(%d) 应 %v，实得 %v", i+1, w*time.Second, got)
		}
	}
	if got := nextBackoff(0); got != 0 {
		t.Errorf("nextBackoff(0) 应 0，实得 %v", got)
	}
	// 接线：可重试失败 ⇒ Backoff() 非 0（Start 循环据此改等待时长）；成功 ⇒ 归 0。
	srv, _ := newCaptureServer(t, http.StatusInternalServerError, http.StatusOK)
	logPath := filepath.Join(t.TempDir(), "zerg-agent.log")
	writeLog(t, logPath, "2026-10-10T13:00:00.000+08:00 INFO  [agent     ] 一行")
	r := reporterAt(t, srv.URL, logPath)

	r.RunOnce() // 500 ⇒ 失败 ⇒ 退避 1s
	if r.Backoff() != 1*time.Second {
		t.Fatalf("首次失败后退避应为 1s，实得 %v", r.Backoff())
	}
	r.RunOnce() // 200 ⇒ 成功 ⇒ 退避归 0
	if r.Backoff() != 0 {
		t.Fatalf("成功后退避应归 0，实得 %v", r.Backoff())
	}
}
