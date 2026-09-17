package agent

// sysmetrics.go — v2.5.4.9 机器级指标采样（CA 全量追踪——设计文档 docs/设计-CA跟踪程序.md）
// 每轮采样: 本机 CPU/内存/磁盘读写 + X3 GPU/CPU/内存（agent 状态接口）
// 记录到 sysmetrics.jsonl（zerg-trace 时间线视图联动分析瓶颈）

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/tracectx"
)

// SysMetric 单次采样（一轮）
type SysMetric struct {
	Timestamp time.Time `json:"ts"`
	Step      int       `json:"step"`
	// 本机
	LocalCPU  float64 `json:"local_cpu,omitempty"`       // 本机 CPU %（总）
	LocalMem  float64 `json:"local_mem,omitempty"`       // 本机内存 %
	DiskRead  float64 `json:"disk_read_mbps,omitempty"`  // 磁盘读 MB/s
	DiskWrite float64 `json:"disk_write_mbps,omitempty"` // 磁盘写 MB/s
	// X3（agent 状态——如有）
	X3GPU      float64 `json:"x3_gpu,omitempty"`          // X3 GPU %
	X3CPU      float64 `json:"x3_cpu,omitempty"`          // X3 CPU %
	X3Active   int     `json:"x3_active,omitempty"`       // X3 活跃请求
	X3MemAvail float64 `json:"x3_mem_avail_gb,omitempty"` // X3 可用内存 GB
}

// SysMetricsCollector 机器指标采集器（写 sysmetrics.jsonl）
type SysMetricsCollector struct {
	file  *os.File
	x3URL string             // X3 agent 状态地址（如 http://<worker-ip>:8100/status）——空=不采 X3
	token string             // X3 认证 token
	last  map[string]float64 // 上次磁盘读数（算速率）
	lastT time.Time
}

// NewSysMetricsCollector 创建采集器
func NewSysMetricsCollector(logDir, x3URL, token string) (*SysMetricsCollector, error) {
	path := filepath.Join(logDir, "sysmetrics.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &SysMetricsCollector{
		file:  f,
		x3URL: x3URL,
		token: token,
		last:  map[string]float64{},
	}, nil
}

// Close 关闭
func (c *SysMetricsCollector) Close() {
	if c.file != nil {
		c.file.Close()
	}
}

// Sample 采样一次（step 轮次）——轻量（<50ms）
func (c *SysMetricsCollector) Sample(step int) {
	m := SysMetric{Timestamp: time.Now(), Step: step}
	// 本机 CPU/内存（ps 总负载）
	c.sampleLocal(&m)
	// 磁盘读写（iostat——macOS）
	c.sampleDisk(&m)
	// X3 状态（HTTP——轻量）
	c.sampleX3(&m)
	// 写入
	data, _ := json.Marshal(m)
	c.file.Write(append(data, '\n'))
}

// sampleLocal 本机 CPU/内存（ps 汇总）
func (c *SysMetricsCollector) sampleLocal(m *SysMetric) {
	out, err := exec.Command("ps", "-A", "-o", "%cpu,%mem").Output()
	if err != nil {
		return
	}
	var cpuTotal, memTotal float64
	var n int
	for _, line := range strings.Split(string(out), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		cpu, _ := strconv.ParseFloat(fields[0], 64)
		mem, _ := strconv.ParseFloat(fields[1], 64)
		cpuTotal += cpu
		memTotal += mem
		n++
	}
	if n > 0 {
		m.LocalCPU = cpuTotal
		m.LocalMem = memTotal
	}
}

// sampleDisk 磁盘读写（macOS iostat——读/写 KB/s → MB/s）
func (c *SysMetricsCollector) sampleDisk(m *SysMetric) {
	out, err := exec.Command("iostat", "-d", "-c", "2").Output()
	if err != nil {
		return
	}
	// iostat 输出最后一行: 磁盘名 KB/t tps MB/s KB/s 等——取读/写
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 3 {
		return
	}
	fields := strings.Fields(lines[len(lines)-1])
	// macOS iostat: device KB/t tps MB/s KB/s
	if len(fields) >= 5 {
		if mbps, err := strconv.ParseFloat(fields[3], 64); err == nil {
			m.DiskRead = mbps
		}
		if kbs, err := strconv.ParseFloat(fields[4], 64); err == nil {
			m.DiskWrite = kbs / 1024
		}
	}
}

// sampleX3 X3 agent 状态（HTTP——超时短——失败跳过）
func (c *SysMetricsCollector) sampleX3(m *SysMetric) {
	if c.x3URL == "" {
		return
	}
	req, err := http.NewRequest("GET", c.x3URL, nil)
	if err != nil {
		return
	}
	if c.token != "" {
		req.Header.Set("X-Auth-Token", c.token)
	}
	// T1.6 传播（出站到子端）：X3 /status 采集也是"主控→子端"的 HTTP 调用 ⇒ 带 traceparent，
	// 让这条采样的链在两侧也对得上（无会话 ⇒ 本侧 root，如实）。
	tracectx.Propagate(req.Header, nil, "", tracectx.ReplayMarked())
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var st struct {
		GpuPct       float64 `json:"gpu_pct"`
		CpuPct       float64 `json:"cpu_pct"`
		Active       int     `json:"active_requests"`
		MemAvailable float64 `json:"mem_available_gb"`
	}
	if json.NewDecoder(resp.Body).Decode(&st) == nil {
		m.X3GPU = st.GpuPct
		m.X3CPU = st.CpuPct
		m.X3Active = st.Active
		m.X3MemAvail = st.MemAvailable
	}
}
