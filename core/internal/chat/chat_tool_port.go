package chat

// chat_tool_port.go — port_services 本机端口→服务映射（P4-50——bb32ae 教训）
// 背景: 模型测速/诊断时 bash 盲扫端口（8770=sharingd 误判 2 轮/8104 扫 8 端点全 404）——无本机服务地图
// 实现: lsof 监听端口 → 进程 → 已知服务标注（模型不用猜）

import (
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// knownPortServices — 已知服务标注（本机——扩展随部署）
var knownPortServices = map[int]string{
	8580: "zerg-core 主控 HTTP",
	8790: "OCR 服务（image_ocr 工具——RapidOCR）",
	8001: "DS4 模型（provider ds4——base_url 127.0.0.1:8001）",
	5432: "Postgres",
	8104: "python3 推理服务（推理/模型——用前先验端口存活）",
}

// portServices — 本机监听端口清单（lsof -iTCP -sTCP:LISTEN——进程名/服务标注）
func portServices(args map[string]any) (string, error) {
	cmd := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-nP")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(out), "\n")
	type portInfo struct {
		port   int
		proc   string
		pid    string
		proto  string
		marked bool
	}
	seen := map[string]portInfo{} // key: proc@port
	var order []string
	for _, l := range lines[1:] { // 跳表头
		f := strings.Fields(l)
		if len(f) < 9 {
			continue
		}
		proc := f[0]
		pid := f[1]
		// 找端口（IPv4 127.0.0.1:8580 或 *:8580 或 [::1]:8580）
		port := -1
		for _, token := range f[8:] {
			if strings.Contains(token, ":") {
				parts := strings.Split(token, ":")
				last := parts[len(parts)-1]
				if n, err := strconv.Atoi(last); err == nil && n > 0 {
					port = n
					break
				}
			}
		}
		if port <= 0 {
			continue
		}
		key := proc + "@" + strconv.Itoa(port)
		if _, ok := seen[key]; !ok {
			seen[key] = portInfo{port: port, proc: proc, pid: pid}
			order = append(order, key)
		}
	}
	// 排序（按端口）
	sort.Slice(order, func(i, j int) bool {
		return seen[order[i]].port < seen[order[j]].port
	})
	var sb strings.Builder
	sb.WriteString("# 本机监听端口→服务\n")
	sb.WriteString("（未知进程可 ps -p <pid> 看详情——推理服务常见 python3/llama-server）\n\n")
	for _, key := range order {
		pi := seen[key]
		label, ok := knownPortServices[pi.port]
		if ok {
			pi.marked = true
			sb.WriteString(strconv.Itoa(pi.port) + ": " + label + "（进程 " + pi.proc + " pid " + pi.pid + "）\n")
		} else {
			sb.WriteString(strconv.Itoa(pi.port) + ": " + pi.proc + "（pid " + pi.pid + "——未标注——看进程判断）\n")
		}
	}
	return sb.String(), nil
}
