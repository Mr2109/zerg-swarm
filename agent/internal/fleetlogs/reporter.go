// Package fleetlogs 提供子端到主控的**机群日志批量上报**（缺口 `GAP-20260927-202` ① · 段2）。
//
// 定案真源 `gateway/protocol.md:57-65`：
//   - 每 5s 一批增量 POST `{controller}/api/fleet/logs`
//   - 头 `X-Auth-Token`
//   - body `{"machine":"node-a","logs":[{"ts","level","component","model","request_id","message"}]}`
//   - level ∈ INFO/WARN/ERROR/DEBUG；component ∈ agent/backend/health/heartbeat/fleet
//
// 诚实纪律（与 `heartbeat` 同族）：读**本机日志文件**（logx 的 `--log-file` 源）的**增量**，
// 逐行解析成协议条目批量上报；**上报失败不静默吞** —— 出声（logx.Errorf）+ 计数，
// 且**只在上报成功后才推进读取游标**（失败 ⇒ 下轮重读重发、条目不丢）。
// 无本机日志源（`--log-file` 空）⇒ 启动时**点名一次**，不上报（不静默当无事）。
package fleetlogs

import (
	"bytes"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/logx"
)

// Entry 一条机群日志（字段名逐字照 `protocol.md:64` 定案；`machine` 在顶层，不在这儿）。
type Entry struct {
	TS        string `json:"ts"`
	Level     string `json:"level"`
	Component string `json:"component"`
	Model     string `json:"model"`
	RequestID string `json:"request_id"`
	Message   string `json:"message"`
	// EventID —— **幂等键**（§三.5 附 H③ · Q6）：节点侧生成的 ULID（时间戳 48b ＋ 随机 80b）。
	//   生成位点 = **入队（上报）之前、每批生成一次 ＋ 重投复用**。
	//   ★ 必须跨重投**稳定**：主控靠 `UNIQUE(machine, event_id)` ＋ `ON CONFLICT DO NOTHING` 去重；
	//     若每轮重投都重生成 ⇒ 去重永不触发、重投全成新行（去重失效）。
	//   `omitempty`：老路（`ParseLogLine`）不填 ⇒ 不上线（主控对空 event_id **不**去重 —— 是写明的时效边界）。
	EventID string `json:"event_id,omitempty"`
}

// reportBody 上报体（顶层 machine + logs[]）。
type reportBody struct {
	Machine string  `json:"machine"`
	Logs    []Entry `json:"logs"`
}

const (
	defaultInterval = 5 * time.Second // 稳态上报间隔（`protocol.md:62` 定案 · **不变**）
	// ── T5b-d ② 退避 ＋ ④ 批上限（照 §三.5 附 H① · §六 Q7）──
	backoffBase = 1 * time.Second  // 退避基数（局域网 ⇒ 比公网 2s 更激进）
	backoffCap  = 60 * time.Second // 退避上限（✗ **不封顶**：日志是审计料、丢不起 ⇒ 一直退避重试）
	maxBatch    = 500              // 单批条数上限（超出 ⇒ 切多批连续投 · ✗ 不攒巨 payload）
)

// Reporter 机群日志上报器。
type Reporter struct {
	controller string
	token      string
	machine    string
	logPath    string
	interval   time.Duration
	httpClient *http.Client
	stopCh     chan struct{}

	mu     sync.Mutex
	offset int64 // 已上报到的文件字节游标（只在 post 成功后前移）
	fails  int   // 累计上报失败次数（「不静默吞」的可核证据）

	// ── T5b-d 四格（**不换队列模型** · ✗ 不引入队列 / spool / 内存上限）──
	gaveUp    bool          // ③ 错误分类：401/403 = 令牌错 ⇒ 立即放弃（✗ 不再重试）
	drops     int           // ③ 错误分类：400/413 = 永久性失败 ⇒ 不重试、丢该批（计数 ＋ 出声）
	backoff   time.Duration // ② 最近一次失败后的退避时长（0 = 上次成功；供诊断读）
	pendStart int64         // ① 上一批的起始游标（**重投复用** event_id 的锚）
	pendEnd   int64         // ① 上一批的结束游标
	pendIDs   []string      // ① 上一批逐条 event_id（重投复用 · ✗ 不重生成）
}

// NewReporter 造上报器。interval 固定 5s（`protocol.md:62` 定案）；logPath 空 = 无本机日志源。
func NewReporter(controller, token, machine, logPath string) *Reporter {
	return &Reporter{
		controller: strings.TrimRight(controller, "/"),
		token:      token,
		machine:    machine,
		logPath:    logPath,
		interval:   defaultInterval,
		stopCh:     make(chan struct{}),
		// 禁用代理：子端到主控是局域网直连，系统代理（如 mihomo TUN）会劫持导致 no route
		// （与 heartbeat 同因）。
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: &http.Transport{Proxy: nil},
		},
	}
}

// Start 启动上报循环。无本机日志源 ⇒ 点名一次后返回（不空转）。
func (r *Reporter) Start() {
	if strings.TrimSpace(r.logPath) == "" {
		logx.Warnf("fleet", "日志上报**无本机日志源**（`--log-file` 未设）⇒ 不上报（不静默当无事）", "machine", r.machine)
		return
	}
	// 起始游标 = 当前文件末尾（增量：只报启动之后新写的行，不重放历史）。
	r.mu.Lock()
	if fi, err := os.Stat(r.logPath); err == nil {
		r.offset = fi.Size()
	}
	r.mu.Unlock()

	go func() {
		// 稳态间隔 = `interval`（5s · 不变）；**失败 ⇒ 指数退避** base 1s ×2 / cap 60s（✗ 不封顶）。
		timer := time.NewTimer(r.interval)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				r.RunOnce()
				wait := r.interval
				if d := r.Backoff(); d > 0 {
					wait = d
				}
				timer.Reset(wait)
			case <-r.stopCh:
				return
			}
		}
	}()
}

// Stop 停止上报循环（可重复调用安全）。
func (r *Reporter) Stop() {
	select {
	case <-r.stopCh:
	default:
		close(r.stopCh)
	}
}

// FailCount 累计上报失败次数（供自检 / 诊断读）。
func (r *Reporter) FailCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fails
}

// Offset 当前读取游标（供自检 / 诊断读）。
func (r *Reporter) Offset() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.offset
}

// Drops 累计「永久性失败丢批」数（HTTP 400/413 ⇒ 不重试）——供自检 / 诊断读。
func (r *Reporter) Drops() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.drops
}

// GaveUp 是否已放弃上报（HTTP 401/403 = 令牌错 ⇒ 立即放弃）。
func (r *Reporter) GaveUp() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gaveUp
}

// Backoff 最近一次失败后的退避时长（0 = 上次成功）；供诊断 / 自检读。
func (r *Reporter) Backoff() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.backoff
}

// RunOnce 读一次增量并上报（循环与自检共用）。
//
// ★ 失败语义（硬约束 · **一字不动**）：**可重试的失败 ⇒ 出声 ＋ 计数 ＋ 游标不推进**
// （下轮重读重发、条目不丢 · 重投**复用同批 event_id**）。T5b-d 只**新增**两支分类：
//   - 400/413 = 永久性失败 ⇒ ✗ 不重试（重投同一批永远不会成 = 毒丸）⇒ 丢该批 ＋ 出声 ＋ 计数 ＋ 游标前移；
//   - 401/403 = 令牌错 ⇒ **立即放弃**（`gaveUp`）＋ 点名（✗ 再试无意义）。
func (r *Reporter) RunOnce() {
	r.mu.Lock()
	if r.gaveUp {
		r.mu.Unlock()
		return // 已放弃（401/403）⇒ ✗ 不再试（点名已在放弃那一次做过）
	}
	start := r.offset
	r.mu.Unlock()

	entries, next, err := r.scan()
	if err != nil {
		logx.Errorf("fleet", "读本机日志源失败", "machine", r.machine, "path", r.logPath, "error", err)
		return
	}
	if len(entries) == 0 {
		return
	}
	r.assignEventIDs(start, next, entries) // ① 每批生成一次 ＋ **重投复用**

	perr := r.postBatch(entries) // ④ 批 ≤500 切多批
	switch classifyPostErr(perr) {
	case postOK:
		r.mu.Lock()
		r.offset = next
		r.pendIDs, r.pendStart, r.pendEnd = nil, 0, 0
		r.backoff = 0
		r.mu.Unlock()
		logx.Infof("fleet", "日志上报成功", "machine", r.machine, "count", len(entries))
	case postRetry:
		// **不静默吞**：出声 + 计数；② 退避 base 1s ×2 / cap 60s（✗ 不封顶）；游标**不**前移
		// ⇒ 下轮重读重发、条目不丢（重投**复用同批 event_id**）。
		r.mu.Lock()
		r.fails++
		n := r.fails
		r.backoff = nextBackoff(n)
		wait := r.backoff
		r.mu.Unlock()
		logx.Errorf("fleet", "日志上报失败", "machine", r.machine, "url", r.endpoint(),
			"count", len(entries), "fail_total", n, "backoff", wait.String(), "error", perr)
	case postDrop:
		// 400/413 = 永久性失败 ⇒ ✗ 不重试（毒丸）⇒ 丢该批 ＋ 出声 ＋ 计数 ＋ 游标前移。
		r.mu.Lock()
		r.drops++
		d := r.drops
		r.pendIDs, r.pendStart, r.pendEnd = nil, 0, 0
		r.offset = next
		r.mu.Unlock()
		logx.Errorf("fleet", "日志上报**永久性失败**（HTTP 400/413）⇒ 不重试、丢该批", "machine", r.machine,
			"count", len(entries), "drop_total", d, "error", perr)
	case postGiveUp:
		// 401/403 = 令牌错 ⇒ **立即放弃 ＋ 点名**（✗ 不再重试 · ✗ 不静默）。
		r.mu.Lock()
		r.gaveUp = true
		r.mu.Unlock()
		logx.Errorf("fleet", "日志上报**认证失败**（HTTP 401/403 · 令牌缺失或不合法）⇒ 立即放弃、不再重试",
			"machine", r.machine, "url", r.endpoint(), "error", perr)
	}
}

func (r *Reporter) endpoint() string { return r.controller + "/api/fleet/logs" }

// scan 从当前游标起读**完整行**（只吃以 `\n` 结尾的行），解析成条目并给出新游标。
// 不修改 r.offset（推进由 RunOnce 在 post 成功后做）。解析不出的行跳过（不入上报体）。
func (r *Reporter) scan() ([]Entry, int64, error) {
	r.mu.Lock()
	start := r.offset
	r.mu.Unlock()

	f, err := os.Open(r.logPath)
	if err != nil {
		return nil, start, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, start, err
	}
	size := fi.Size()
	if start > size {
		start = 0 // 文件被截断 / 轮转 ⇒ 从头读
	}
	if start == size {
		return nil, start, nil
	}
	buf := make([]byte, size-start)
	n, err := f.ReadAt(buf, start)
	if err != nil && n == 0 {
		return nil, start, err
	}
	buf = buf[:n]
	lastNL := bytes.LastIndexByte(buf, '\n')
	if lastNL < 0 {
		return nil, start, nil // 没有完整行（尾巴不完整）⇒ 留到下一轮
	}
	complete := buf[:lastNL+1]
	entries := make([]Entry, 0, bytes.Count(complete, []byte("\n")))
	for _, line := range strings.Split(string(complete), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if e, ok := ParseLogLine(line); ok {
			entries = append(entries, e)
		}
	}
	return entries, start + int64(lastNL+1), nil
}

// post 把一批条目 POST 到主控 `/api/fleet/logs`。
func (r *Reporter) post(entries []Entry) error {
	body, err := json.Marshal(reportBody{Machine: r.machine, Logs: entries})
	if err != nil {
		return fmt.Errorf("序列化上报体失败: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, r.endpoint(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", r.token)
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &postStatusError{code: resp.StatusCode} // 带码 ⇒ 错误分类的唯一依据（文案与旧逐字同）
	}
	return nil
}

// ParseLogLine 把一行 logx 日志解析成协议条目。
//
// logx 格式（`agent/internal/logx/logx.go` 的 `"%s %-5s [%-10s] %s %s\n"`）：
// `<ts> <LEVEL(5宽)> [<component(10宽)>] <message> <k=v...>`
// ⇒ ts / level / component 逐字取自该行（component 去右填充）；model / request_id
// 从行内 `model=` / `request_id=` 字段取。
func ParseLogLine(line string) (Entry, bool) {
	line = strings.TrimRight(line, "\r")
	sp1 := strings.IndexByte(line, ' ')
	if sp1 <= 0 {
		return Entry{}, false
	}
	ts := line[:sp1]
	rest := line[sp1+1:]
	sp2 := strings.IndexByte(rest, ' ')
	if sp2 <= 0 {
		return Entry{}, false
	}
	level := strings.TrimSpace(rest[:sp2])
	rest = strings.TrimSpace(rest[sp2+1:])
	component := "agent"
	if strings.HasPrefix(rest, "[") {
		if end := strings.IndexByte(rest, ']'); end > 0 {
			component = strings.TrimSpace(rest[1:end])
			rest = strings.TrimSpace(rest[end+1:])
		}
	}
	e := Entry{TS: ts, Level: strings.ToUpper(level), Component: component, Message: rest}
	for _, tok := range strings.Fields(rest) {
		switch {
		case strings.HasPrefix(tok, "model="):
			e.Model = strings.TrimPrefix(tok, "model=")
		case strings.HasPrefix(tok, "request_id="):
			e.RequestID = strings.TrimPrefix(tok, "request_id=")
		}
	}
	return e, true
}

// ─────────────────────── T5b-d 四格（不换队列模型 · ✗ 不引入队列/spool/内存上限） ───────────────────────

// postBatch —— ④ 批 ≤ `maxBatch`（500）：超出 ⇒ **切多批连续投**（✗ 不攒巨 payload · §三.5 附 H①）。
// ✗ 不引入队列 / spool：本件只把**一次 scan 的增量**按 500 切段、逐段 POST。
func (r *Reporter) postBatch(entries []Entry) error {
	for start := 0; start < len(entries); start += maxBatch {
		end := start + maxBatch
		if end > len(entries) {
			end = len(entries)
		}
		if err := r.post(entries[start:end]); err != nil {
			return err // 某段失败即返（后续段不投 ⇒ 游标不前进，下轮从同一批头重来、id 复用）
		}
	}
	return nil
}

// postStatusError —— 上报被 HTTP 非 2xx 挡下（带状态码 ⇒ **错误分类的唯一依据**）。
type postStatusError struct{ code int }

func (e *postStatusError) Error() string { return fmt.Sprintf("HTTP %d", e.code) }

// postOutcome —— 一次上报的归属（③ 错误分类三态 ＋ 成功）。
type postOutcome int

const (
	postOK     postOutcome = iota // 成功
	postRetry                     // 可重试（网络 / 404 / 429 / 5xx 一类）⇒ 游标**不**前进
	postDrop                      // 400 / 413 ⇒ 永久性失败、不重试、丢该批
	postGiveUp                    // 401 / 403 ⇒ 令牌错、立即放弃 ＋ 点名
)

// classifyPostErr —— ③ 错误分类（照 §三.5 附 H① · §六 Q7 逐字）：
//
//	400 / 413                  ⇒ **不重试**（毒丸：重投同一批永远不会成）
//	401 / 403                  ⇒ **不重试 ＋ 立即放弃 ＋ 点名**（令牌错）
//	其余（网络 / 404 / 429 / 5xx）⇒ **重试**（游标不前进 · 「失败⇒游标不前进」语义**不动**）
func classifyPostErr(err error) postOutcome {
	if err == nil {
		return postOK
	}
	var se *postStatusError
	if errors.As(err, &se) {
		switch se.code {
		case 400, 413:
			return postDrop
		case 401, 403:
			return postGiveUp
		}
	}
	return postRetry
}

// nextBackoff —— ② 指数退避 base `backoffBase`(1s) ×2 / cap `backoffCap`(60s) · **✗ 不封顶**。
// 序列（fails = 1,2,3,…）：1s · 2s · 4s · 8s · 16s · 32s · 60s · 60s · …（照 §三.5 附 H① 的 Datadog 配方）。
func nextBackoff(fails int) time.Duration {
	if fails <= 0 {
		return 0
	}
	d := backoffBase
	for i := 1; i < fails; i++ {
		if d >= backoffCap {
			return backoffCap
		}
		d *= 2
	}
	if d > backoffCap {
		return backoffCap
	}
	return d
}

// assignEventIDs —— ① `event_id`（ULID）：**入队（上报）前生成一次、重投复用**（§三.5 附 H③）。
// ★ 重投复用是**硬要求**（同 `Entry.EventID` 注）。复用锚 = 「这一批的字节区间」：
// 失败 ⇒ 游标不前进 ⇒ 下轮扫到**同一区间** ⇒ 复用同批 id；区间一变（期间又来了新完整行）⇒ 重生成。
// ✗ 本件不引入队列 / spool：只记**当前这一批**（O(批) 内存）。
func (r *Reporter) assignEventIDs(start, end int64, entries []Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pendIDs != nil && len(r.pendIDs) == len(entries) && r.pendStart == start && r.pendEnd == end {
		for i := range entries {
			entries[i].EventID = r.pendIDs[i]
		}
		return
	}
	ids := make([]string, len(entries))
	for i := range entries {
		ids[i] = newULID()
		entries[i].EventID = ids[i]
	}
	r.pendStart, r.pendEnd, r.pendIDs = start, end, ids
}

// ulidAlphabet —— Crockford Base32（ULID 规范字母表：✗ 不含 I / L / O / U）。
const ulidAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ulidFallbackSeq —— 随机源不可用时的兜底自增（✗ 绝不退化成空串 —— 空 event_id 会让去重失效）。
var ulidFallbackSeq uint64

// newULID —— 造一枚 ULID = 时间戳 48b ＋ 随机 80b（26 字符 · §三.5 附 H③）。
// ★ 随机段取 `crypto/rand`；万一取不到 ⇒ 用纳秒时间 ＋ 进程内自增兜底（仍保「同进程不重复」最低要求）。
func newULID() string {
	ms := uint64(time.Now().UnixMilli()) & 0xffffffffffff // 48 bit
	var rnd [10]byte
	if _, err := crand.Read(rnd[:]); err != nil {
		n := uint64(time.Now().UnixNano())
		s := atomic.AddUint64(&ulidFallbackSeq, 1)
		for i := 0; i < 8; i++ {
			rnd[i] = byte(n >> uint(56-8*i))
		}
		rnd[8] = byte(s >> 8)
		rnd[9] = byte(s)
	}
	return encodeULID(ms, rnd)
}

// encodeULID —— 128 bit → 26 字符（前 10 字符 = 48b 时间戳 MSB-first（首字符只用高 3 位）；
// 后 16 字符 = 80b 随机段，每字符恰 5 位）—— 逐字照 ULID 规范的字符布局。
func encodeULID(ms uint64, rnd [10]byte) string {
	var out [26]byte
	for i := 0; i < 10; i++ {
		out[i] = ulidAlphabet[(ms>>uint(45-5*i))&0x1f]
	}
	for i := 0; i < 16; i++ {
		v := 0
		for k := 0; k < 5; k++ {
			v <<= 1
			pos := 5*i + k
			if rnd[pos/8]&(1<<uint(7-pos%8)) != 0 {
				v |= 1
			}
		}
		out[10+i] = ulidAlphabet[v]
	}
	return string(out[:])
}
