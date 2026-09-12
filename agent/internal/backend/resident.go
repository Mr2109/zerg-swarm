// 驻留明细与未托管进程上报（《设计-资源管理器》§3.1 驻留明细 / §八 Q6）。
//
// 两条纪律：
//  1. 托管项：如实报状态/最后使用/在飞请求/实测 RSS/是否托管(managed=true)。
//  2. 未托管项（实测 E2：手工 screen 起的服务）：只做**只读端口探测**如实标注
//     managed=false；**绝不接管、绝不杀**（Q6 拍板）。
package backend

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// ResidentDetail 一台机器上一个驻留模型的上报明细（字段与形状对齐 core 侧账本)。
type ResidentDetail struct {
	Digest       string  `json:"digest,omitempty"`     // 模型身份（内容摘要）；本端拿不到则缺席，不编造
	Alias        string  `json:"alias,omitempty"`      // 名字仅作别名
	File         string  `json:"file,omitempty"`       // 权重路径
	State        string  `json:"state"`                // loading|ready|crashed|sleeping
	LastUsedAgoS float64 `json:"last_used_ago_s"`      // 距最后使用的秒数（LRU 判据）
	ReqCount     int     `json:"req_count"`            // 该模型在飞请求数
	RssGb        float64 `json:"rss_gb,omitempty"`     // 进程实测内存（RSS）
	Managed      bool    `json:"managed"`              // 是否在子端托管清单
	MemGb        float64 `json:"mem_gb,omitempty"`     // 模型声明内存需求
	CtxWindow    int     `json:"ctx_window,omitempty"` // 上下文上限
	Source       string  `json:"source,omitempty"`     // 由谁装载：managed|manual|external
}

// UnmanagedProcess 未托管但占着端口的进程（如实呈现；绝不接管、绝不杀）。
type UnmanagedProcess struct {
	PID     int     `json:"pid,omitempty"`
	Port    int     `json:"port,omitempty"`
	Command string  `json:"command,omitempty"`
	Model   string  `json:"model,omitempty"`
	RssGb   float64 `json:"rss_gb,omitempty"`
	Note    string  `json:"note,omitempty"`
	Managed bool    `json:"managed"` // 恒 false——未托管项如实标注
}

// ResidentDetail 返回当前托管驻留模型的明细列表（按别名排序，结果可复现）。
// last_used_ago_s 与 req_count 来自 subproc（manager.go 的 lastUsed/reqCount）。
func (m *Manager) ResidentDetail() []ResidentDetail {
	m.mu.Lock()
	now := time.Now()
	out := make([]ResidentDetail, 0, len(m.procs))
	for name, sp := range m.procs {
		d := ResidentDetail{
			Alias:    name,
			State:    sp.state,
			ReqCount: sp.reqCount,
			Managed:  true,
			Source:   "managed",
		}
		if !sp.lastUsed.IsZero() {
			d.LastUsedAgoS = now.Sub(sp.lastUsed).Seconds()
		}
		if sp.entry != nil {
			d.File = sp.entry.File
			d.MemGb = sp.entry.MemGB
			d.CtxWindow = entryCtxWindow(sp.entry)
			d.Digest = entryDigest(sp.entry)
		}
		if sp.proc != nil && sp.proc.Process != nil {
			d.RssGb = readProcessRssGb(sp.proc.Process.Pid)
		}
		out = append(out, d)
	}
	m.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Alias < out[j].Alias })
	return out
}

// UnmanagedListeners 对给定端口做**只读** TCP 探测，返回"有人在听但不归本管理器管"的项。
// 覆盖实测 E2（手工 screen 起的服务）；绝不接管、绝不 kill、不发任何模型指令。
func (m *Manager) UnmanagedListeners(ports []int) []UnmanagedProcess {
	m.mu.Lock()
	owned := make(map[int]bool, len(m.procs))
	for _, sp := range m.procs {
		if sp.port > 0 {
			owned[sp.port] = true
		}
	}
	m.mu.Unlock()

	hits := probeListeners(ports)
	out := make([]UnmanagedProcess, 0, len(hits))
	for _, port := range hits {
		if owned[port] {
			continue // 本管理器自己的进程——托管项，不在未托管清单里
		}
		out = append(out, UnmanagedProcess{
			Port:    port,
			Note:    "listening on 127.0.0.1, not managed by agent (managed=false); read-only report, never taken over",
			Managed: false,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

// probeListeners 并发探测 127.0.0.1 上哪些端口在监听（只做 TCP 连接，不发送数据）。
func probeListeners(ports []int) []int {
	const workers = 128
	const dialTimeout = 150 * time.Millisecond

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var hits []int

	for _, p := range ports {
		if p <= 0 || p > 65535 {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(port int) {
			defer wg.Done()
			defer func() { <-sem }()
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), dialTimeout)
			if err != nil {
				return
			}
			conn.Close()
			mu.Lock()
			hits = append(hits, port)
			mu.Unlock()
		}(p)
	}
	wg.Wait()
	return hits
}

// ParsePortSpec 解析端口清单（如 "9000-9999" 或 "8100,8101,9000-9002"）。
// 非法片段跳过；最多 4096 个端口（防止意外铺满整段）。空串返回 nil。
func ParsePortSpec(spec string) []int {
	const maxPorts = 4096
	var out []int
	seen := make(map[int]bool)
	add := func(p int) bool {
		if p <= 0 || p > 65535 || seen[p] {
			return true
		}
		seen[p] = true
		out = append(out, p)
		return len(out) < maxPorts
	}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			loN, err1 := strconv.Atoi(strings.TrimSpace(lo))
			hiN, err2 := strconv.Atoi(strings.TrimSpace(hi))
			if err1 != nil || err2 != nil || loN > hiN {
				continue
			}
			for p := loN; p <= hiN; p++ {
				if !add(p) {
					return out
				}
			}
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		if !add(n) {
			return out
		}
	}
	return out
}

// entryCtxWindow 从注册条目的额外字段里取上下文上限；取不到返回 0（缺席，不编造）。
func entryCtxWindow(e *registry.ModelEntry) int {
	if e == nil || e.Custom == nil {
		return 0
	}
	for _, k := range []string{"ctx_window", "context_window", "context_length", "n_ctx", "ctx"} {
		if v, ok := e.Custom[k]; ok {
			switch n := v.(type) {
			case int:
				return n
			case int64:
				return int(n)
			case float64:
				return int(n)
			}
		}
	}
	return 0
}

// entryDigest 从注册条目的额外字段里取内容摘要；取不到返回空串（缺席，不编造）。
func entryDigest(e *registry.ModelEntry) string {
	if e == nil || e.Custom == nil {
		return ""
	}
	for _, k := range []string{"digest", "sha256", "model_digest"} {
		if v, ok := e.Custom[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// readProcessRssGb 读进程实测内存（RSS，GB）。
// Linux 走 /proc/<pid>/status 的 VmRSS；其余（含 macOS）走 ps -o rss=。
// 读不到返回 0（如实缺席，不编造）。
func readProcessRssGb(pid int) float64 {
	if pid <= 0 {
		return 0
	}
	// Linux：/proc/<pid>/status -> VmRSS: <kB> kB
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "VmRSS:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if kb, err := strconv.ParseFloat(fields[1], 64); err == nil {
						return kb / 1024 / 1024
					}
				}
			}
		}
	}
	// 其余平台（含 macOS）：ps -o rss= 输出 kB
	return readMacOSRss(pid)
}
