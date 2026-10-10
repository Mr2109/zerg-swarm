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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
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
}

// reportBody 上报体（顶层 machine + logs[]）。
type reportBody struct {
	Machine string  `json:"machine"`
	Logs    []Entry `json:"logs"`
}

const defaultInterval = 5 * time.Second

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
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.RunOnce()
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

// RunOnce 读一次增量并上报（循环与自检共用）。上报失败 ⇒ 出声 + 计数 + **游标不推进**。
func (r *Reporter) RunOnce() {
	entries, next, err := r.scan()
	if err != nil {
		logx.Errorf("fleet", "读本机日志源失败", "machine", r.machine, "path", r.logPath, "error", err)
		return
	}
	if len(entries) == 0 {
		return
	}
	if perr := r.post(entries); perr != nil {
		// **不静默吞**：出声 + 计数；游标**不**前移 ⇒ 下轮重读重发、条目不丢。
		r.mu.Lock()
		r.fails++
		n := r.fails
		r.mu.Unlock()
		logx.Errorf("fleet", "日志上报失败", "machine", r.machine, "url", r.endpoint(),
			"count", len(entries), "fail_total", n, "error", perr)
		return
	}
	r.mu.Lock()
	r.offset = next
	r.mu.Unlock()
	logx.Infof("fleet", "日志上报成功", "machine", r.machine, "count", len(entries))
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
		return fmt.Errorf("HTTP %d", resp.StatusCode)
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
