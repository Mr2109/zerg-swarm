// 驻留明细上报（《设计-资源管理器》§3.1 驻留明细 / §八 Q6）。
//
// 纪律：托管项如实报状态/最后使用/在飞请求/实测 RSS/是否托管(managed=true)。
// （未托管端口探测层已随 P4 退场清理删除——卵之外无引擎，附录 C·C7；
//
//	"非引擎 GPU 使用者"的只读上报归 §8.7 / monitor 逐进程 GTT 归因。）
package backend

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
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
	// 批 3（Q2 规则④ / Q5）新增：驱逐排序键与 pin 状态。
	// 两者都只在确有其事时出现（取不到就缺席，不写假值）。
	WeightsBytes int64   `json:"weights_bytes,omitempty"` // 权重（量化后）字节数——同档"腾得多"者先
	Pinned       bool    `json:"pinned,omitempty"`        // Q5：pin 中且 TTL 未到期
	PinTtlS      float64 `json:"pin_ttl_s,omitempty"`     // Q5：pin 剩余 TTL 秒
}

// ResidentDetail 返回当前托管驻留模型的明细列表（按别名排序，结果可复现）。
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
			d.WeightsBytes = entryWeightsBytes(sp.entry)
		}
		if pinned, remain := pinState(sp, now); pinned {
			d.Pinned = true
			d.PinTtlS = remain
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
