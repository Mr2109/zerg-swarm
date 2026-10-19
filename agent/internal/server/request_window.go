// request_window.go —— 子端 `/status` 的「最近请求数」与「在飞请求来源」两格
// （缺口 `GAP-20261010-16` 的剩余面）。
//
// 改前的病：CLI 面（`zerg agent show <机> --json recent_requests,request_source`）两格早就在，
// 且判据件 `core/cmd/zerg/cli_agent_request_columns_test.go` 已在，但子端 `/status`
// **从不回这两个键** ⇒ 两格永远落「未知」。本件补齐子端这一半，且
// **✗ 不动 `active_requests` 的语义与数值**（它仍由 handleInferRequest 里
// `activeReqs++ / --` 给出，本件一个字不碰那次增减）。
//
// 两条口径（与 CLI 侧 `core/cmd/zerg/family_agent.go::agentRequestFacts` 对齐）：
//
//	recent_requests : 数字 —— 窗口内（默认 300s，`?window_s=` 可覆盖）本端**收到**的
//	                 `/infer` 次数。口径是「收到的请求数」，✗ 不是 `active_requests`
//	                 （后者只数「正被 worker 处理」的那一个）。
//	request_source  : 字符串 —— 此刻在飞请求的来源（`X-Forwarded-For` 首段，否则
//	                 `RemoteAddr` 的 IP 部分）；**无在飞 ⇒ 明写 `无`**。
//	                 ✗ 不回落空串：空串在 CLI 面等于「子端没报」= 未知，与「真的没人在打」分不开。
//
// 两格的记账点（保证与 `active_requests` 同真源、同时刻）：
//   - `note()` 在 `handleInfer` 进门处（认证通过后、读体前）记一次到达；
//   - `enter()` / `leave()` 与 `activeReqs++/--` **在同一个 defer 里**增减 ⇒
//     「来源为空 ⟺ active_requests == 0」在任一时刻都成立。
//
// 内存有界（✗ 不引新依赖、✗ 不随请求数增长）：时间戳只留最近 `requestStampRetention` 条，
// 且记/取时按窗口裁掉过期的；在飞表按请求序号增删，长度 ≤ 在飞数。
package server

import (
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// defaultRequestWindow 默认窗口 5 分钟（与 CLI `--window-min` 默认值同源；`?window_s=` 可覆盖）。
	defaultRequestWindow = 5 * time.Minute
	// maxRequestWindow 窗口上限 24h —— 与 CLI `--window-min` 的上界 1440 分钟逐字对齐；
	// 超界**明报拒收**（✗ 静默夹取：那会让读数与请求的窗口不是一回事）。
	maxRequestWindow = 24 * time.Hour
	// requestStampRetention 时间戳保留条数上限：超出丢最旧的（窗口再长也只占这么多内存）。
	requestStampRetention = 4096
	// requestSourceNone 无在飞请求时的来源读数（✗ 空串 —— 见件头）。
	requestSourceNone = "无"
)

// requestWindow 请求窗口记账（窗口计数 ＋ 在飞来源）。
// 零值不可用，由 `newRequestWindow` 造。方法对 nil 接收者是空操作 ——
// 有些用例直接造 `&Server{}` / `&Agent{}`（不走 NewAgent），那时不该 panic。
type requestWindow struct {
	mu       sync.Mutex
	stamps   []time.Time       // 到达时刻（只保留窗口内 + 最近 retention 条）
	inFlight map[uint64]string // 请求序号 → 来源（与 activeReqs 同生命周期）
	seq      uint64
	now      func() time.Time // 测试可注入；nil ⇒ time.Now
}

func newRequestWindow() *requestWindow {
	return &requestWindow{inFlight: make(map[uint64]string), now: time.Now}
}

func (w *requestWindow) clock() time.Time {
	if w.now != nil {
		return w.now()
	}
	return time.Now()
}

// note 记一次「收到 /infer」。
func (w *requestWindow) note() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stamps = append(w.stamps, w.clock())
	// 超出保留量 ⇒ 丢最旧的（环形语义；窗口再长也不会把切片撑大）
	if n := len(w.stamps); n > requestStampRetention {
		w.stamps = append(w.stamps[:0], w.stamps[n-requestStampRetention:]...)
	}
}

// enter 记一条在飞请求，返回它的序号（供 leave 用）。
func (w *requestWindow) enter(src string) uint64 {
	if w == nil {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.inFlight == nil {
		w.inFlight = make(map[uint64]string)
	}
	w.seq++
	w.inFlight[w.seq] = strings.TrimSpace(src)
	return w.seq
}

// leave 摘掉一条在飞请求（序号来自 enter）。
func (w *requestWindow) leave(id uint64) {
	if w == nil {
		return
	}
	w.mu.Lock()
	delete(w.inFlight, id)
	w.mu.Unlock()
}

// snapshot 返回（窗口内到达数, 在飞来源去重排序后的切片）。
// ★ 只裁**任何窗口都用不到**的那批（早于 `maxRequestWindow`）；✗ 不能按「本次要的窗口」裁 ——
// 先问 1 秒窗口、再问 5 分钟时必须仍数得到（按短窗口裁会把读数永久吃掉）。
func (w *requestWindow) snapshot(window time.Duration) (int, []string) {
	if w == nil {
		return 0, nil
	}
	if window <= 0 || window > maxRequestWindow {
		window = defaultRequestWindow
	}
	now := w.clock()
	w.mu.Lock()
	defer w.mu.Unlock()
	hardCut := now.Add(-maxRequestWindow)
	drop := 0
	for drop < len(w.stamps) && w.stamps[drop].Before(hardCut) {
		drop++
	}
	if drop > 0 {
		w.stamps = append(w.stamps[:0], w.stamps[drop:]...)
	}
	cut := now.Add(-window)
	recent := 0
	for _, ts := range w.stamps {
		if !ts.Before(cut) {
			recent++
		}
	}
	seen := make(map[string]struct{}, len(w.inFlight))
	for _, s := range w.inFlight {
		if s = strings.TrimSpace(s); s != "" {
			seen[s] = struct{}{}
		}
	}
	sources := make([]string, 0, len(seen))
	for s := range seen {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	return recent, sources
}

// requestWindowParam 解析 `?window_s=`（缺省 = defaultRequestWindow）。
// 给了但读不出 / ≤0 / 超上限 ⇒ **明报拒收**（✗ 静默回落默认：那会让「窗口 1s」的读数
// 其实是 5 分钟的，读数与请求对不上）。
func requestWindowParam(r *http.Request) (time.Duration, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("window_s"))
	if raw == "" {
		return defaultRequestWindow, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("window_s 只收整数秒，实得 %q", raw)
	}
	if n <= 0 {
		return 0, fmt.Errorf("window_s 只收正整数秒，实得 %d", n)
	}
	d := time.Duration(n) * time.Second
	if d > maxRequestWindow {
		return 0, fmt.Errorf("window_s 上限 %d 秒（24h，与 CLI `--window-min` 1..1440 同界），实得 %d",
			int(maxRequestWindow/time.Second), n)
	}
	return d, nil
}

// requestSourceOf 取一条请求的来源：网关转发优先 `X-Forwarded-For` 首段，
// 否则 `RemoteAddr` 的 IP 部分。只读请求头，零副作用。
func requestSourceOf(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = strings.TrimSpace(xff[:i])
		}
		if xff != "" {
			return xff
		}
	}
	if host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
