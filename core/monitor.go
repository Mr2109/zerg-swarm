// core/monitor.go —— 虫族主控监看面板（B4：状态灯 + 模型矩阵 + 任务 + 告警）
// 读 8580 控制面 API（fleet status/models），单页仪表盘展示
package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Monitor 监看面板服务（读 fleet 状态展示）
type Monitor struct {
	apiBase string // 8580 控制面地址（http://127.0.0.1:8580）
	token   string // X-Auth-Token
}

func NewMonitor(apiBase, token string) *Monitor {
	return &Monitor{apiBase: apiBase, token: token}
}

// fleetStatus 从 8580 拉取集群状态
func (m *Monitor) fetchFleet() (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", m.apiBase+"/api/fleet/status", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", m.token)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var d map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, err
	}
	return d, nil
}

func (m *Monitor) handler(w http.ResponseWriter, r *http.Request) {
	fleet, err := m.fetchFleet()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Zerg Monitor</title>
<style>
body{font-family:-apple-system,sans-serif;margin:20px;background:#0d1117;color:#c9d1d9}
h1{color:#58a6ff;font-size:20px} h2{color:#58a6ff;font-size:15px;margin-top:20px}
.card{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:12px;margin:8px 0}
.dot{display:inline-block;width:12px;height:12px;border-radius:50%%;margin-right:6px}
.green{background:#3fb950} .red{background:#f85149} .yellow{background:#d29922}
table{border-collapse:collapse;width:100%%} th,td{border:1px solid #30363d;padding:6px 10px;font-size:13px}
th{background:#21262d} .num{color:#79c0ff} .warn{color:#f85149}
</style></head><body>
<h1>🐜 Zerg Monitor</h1>`)
	if err != nil {
		fmt.Fprintf(w, `<div class="card"><span class="dot red"></span>控制面连接失败: %v</div>`, err)
		fmt.Fprintf(w, `</body></html>`)
		return
	}
	// 机器状态灯
	machines, _ := fleet["machines"].(map[string]interface{})
	fmt.Fprintf(w, `<h2>机器状态</h2><div class="card"><table><tr><th>机器</th><th>健康</th><th>内存可用</th><th>负载</th><th>模型</th><th>请求</th></tr>`)
	for name, mv := range machines {
		mm := mv.(map[string]interface{})
		healthy, _ := mm["healthy"].(bool)
		cls := "red"
		if healthy {
			cls = "green"
		}
		memAvail, _ := mm["mem_available_gb"].(float64)
		load, _ := mm["load"].(float64)
		models, _ := mm["models"].([]interface{})
		active, _ := mm["active_requests"].(float64)
		fmt.Fprintf(w, `<tr><td>%s</td><td><span class="dot %s"></span>%v</td><td class="num">%.0fG</td><td class="num">%.1f</td><td>%d</td><td class="num">%.0f</td></tr>`,
			name, cls, healthy, memAvail, load, len(models), active)
	}
	fmt.Fprintf(w, `</table></div>`)
	// 模型矩阵
	models, _ := fleet["available_models"].([]interface{})
	fmt.Fprintf(w, `<h2>可用模型（%d）</h2><div class="card">`, len(models))
	for _, md := range models {
		fmt.Fprintf(w, `<span class="dot green"></span>%v&nbsp;&nbsp;`, md)
	}
	fmt.Fprintf(w, `</div>`)
	// 汇总
	healthyCount, _ := fleet["healthy_count"].(float64)
	totalMachines, _ := fleet["total_machines"].(float64)
	activeReqs, _ := fleet["total_active_requests"].(float64)
	fmt.Fprintf(w, `<div class="card">健康 %v/%v | 活跃请求 <span class="num">%.0f</span> | 更新时间 %v</div>`,
		healthyCount, totalMachines, activeReqs, time.Now().Format("15:04:05"))
	fmt.Fprintf(w, `</body></html>`)
}

// Start 启动监看面板
func (m *Monitor) Start(port string) error {
	http.HandleFunc("/", m.handler)
	addr := "127.0.0.1:" + port
	fmt.Printf("👁️  Monitor: http://%s\n", addr)
	return http.ListenAndServe(addr, nil)
}
