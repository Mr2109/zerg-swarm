// baseline.go —— P2：基线服务（未托管的手工服务）声明与只读检查。
//
// 设计依据：docs/01-设计/设计-子端服务切换与基线服务声明-20260914.md §7（S3/S6）、
// §9.1（L1 端口→身份 / L2 进程→归属与可停性 / L3 托管方式分类）、§11 M8（配额显式化）。
//
// 铁律（与 resident.go 一致）：本文件**只读**——绝不接管、绝不杀未托管进程（红线②）。
// 借用/归还的写动作在 P3 落地，且必须走 baseline 声明 + 租约存档。
package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// EnvBaselinePorts 声明的基线服务端口清单（如 "9000,9001" 或 "9000-9001"）。
// 这些端口属于 Mr2109 手工起的常驻服务：子端只检查、只报告；启用 borrow 时按租约借还（P3）。
const EnvBaselinePorts = "ZERG_BASELINE_PORTS"

// EnvMachineBudgetGB 机型配额（GB）：准入时与系统可用内存取小者（M8 显式化）。
const EnvMachineBudgetGB = "ZERG_MACHINE_BUDGET_GB"

// EnvBudgetReserveGB 预留给系统与其它进程的余量（GB，默认 8）。
const EnvBudgetReserveGB = "ZERG_BUDGET_RESERVE_GB"

// BaselineService 一个基线服务（未托管的手工服务）的检查结果。
type BaselineService struct {
	Port      int      `json:"port"`
	PID       int      `json:"pid,omitempty"`
	Kind      string   `json:"kind,omitempty"`     // llama | ds4（按可执行文件判定）
	Class     string   `json:"class,omitempty"`    // bare | screen | systemd | unknown（决定"怎么停"）
	Unit      string   `json:"unit,omitempty"`     // systemd 单元名（class=systemd 时有值）
	Identity  string   `json:"identity,omitempty"` // /v1/models 报出的模型标识（L1：端口活≠身份对）
	ApproxGB  float64  `json:"approx_gb,omitempty"`
	Argv      []string `json:"argv,omitempty"`       // 停前存档用（P3）
	Cwd       string   `json:"cwd,omitempty"`        // 同上
	StartTime uint64   `json:"start_time,omitempty"` // PID 复用守卫用（M1/R1）
	Note      string   `json:"note,omitempty"`
	Listening bool     `json:"listening"`
	Managed   bool     `json:"managed"` // 恒 false：基线服务始终是"未托管"
}

// baselinePorts 解析声明的基线端口（未声明则返回空 ⇒ 本机制完全不介入）。
func baselinePorts() []int {
	return ParsePortSpec(strings.TrimSpace(os.Getenv(EnvBaselinePorts)))
}

// baselinePortsConfigured 是否声明了基线（用于日志与"不介入"的快速判定）。
func baselinePortsConfigured() bool { return len(baselinePorts()) > 0 }

// isInferenceArgv 判断一个进程是否"看起来是推理服务"（llama.cpp 系 / ds4 系）。
func isInferenceArgv(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	b := strings.ToLower(argv[0])
	if i := strings.LastIndexByte(b, '/'); i >= 0 {
		b = b[i+1:]
	}
	return strings.Contains(b, "llama-server") || strings.Contains(b, "ds4-server")
}

// parsePortFromArgv 从命令行取监听端口（--port N / -p N / --port=N）；取不到返回 0。
func parsePortFromArgv(argv []string) int {
	ok := func(s string) int {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n <= 0 || n > 65535 {
			return 0
		}
		return n
	}
	for i, a := range argv {
		switch {
		case a == "--port" || a == "-p":
			if i+1 < len(argv) {
				if n := ok(argv[i+1]); n > 0 {
					return n
				}
			}
		case strings.HasPrefix(a, "--port="):
			if n := ok(strings.TrimPrefix(a, "--port=")); n > 0 {
				return n
			}
		}
	}
	return 0
}

// serviceClassFrom 判定托管方式（L3），决定"怎么停"。
//
//   - systemd：cgroup 形如 "/system.slice/ds4-server.service" ⇒ 取单元名（最可靠，优先判定）
//   - screen：自身或祖先进程里有 screen 会话（如 "SCREEN -dmS k2p9001 …" 或 "screen -S k2p9001"）
//   - bare：其余（nohup / 直接起的裸进程）⇒ 发信号即可
//
// ancestry 为「自身 argv」+「各级父进程 argv」（由近及远，顺序不限）；纯函数，便于测试。
func serviceClassFrom(cgroup string, ancestry [][]string) (class, unit string, screenName string) {
	// ① systemd：cgroup 里出现 <单元名>.service
	for _, line := range strings.Split(cgroup, "\n") {
		if i := strings.Index(line, ".service"); i > 0 {
			seg := line[:i+len(".service")]
			if j := strings.LastIndexByte(seg, '/'); j >= 0 {
				seg = seg[j+1:]
			}
			if seg != "" && seg != "-" {
				return "systemd", seg, ""
			}
		}
	}
	// ② screen：回溯祖先链找会话名（-S <名> / -dmS <名>）
	for _, argv := range ancestry {
		for i, a := range argv {
			if a != "-S" && a != "-dmS" && !strings.HasPrefix(a, "-dmS") {
				continue
			}
			if a == "-dmS" && i+1 < len(argv) {
				return "screen", "", argv[i+1]
			}
			if strings.HasPrefix(a, "-dmS") && len(a) > len("-dmS") {
				return "screen", "", strings.TrimPrefix(a, "-dmS")
			}
			if a == "-S" && i+1 < len(argv) {
				return "screen", "", argv[i+1]
			}
		}
	}
	return "bare", "", ""
}

// parseModelIdentity 从 /v1/models 的响应体里取模型标识（L1：端口活 ≠ 身份对）。
//
// 两种形态都认：
//   - llama.cpp 系：{"models":[{"name":"/data/models/…gguf"}]}        ⇒ 取 name（权重路径）
//   - OpenAI 系（ds4）：{"data":[{"id":"deepseek-v4-flash",…}]}          ⇒ 取 id
func parseModelIdentity(body []byte) string {
	var shaped struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &shaped); err != nil {
		return ""
	}
	for _, m := range shaped.Models {
		if m.Name != "" {
			return m.Name
		}
		if m.Model != "" {
			return m.Model
		}
	}
	for _, d := range shaped.Data {
		if d.ID != "" {
			return d.ID
		}
	}
	return ""
}

// probeIdentity 对端口发一条只读 /v1/models，返回模型标识（失败返回空串）。
func probeIdentity(port int, timeout time.Duration) string {
	client := &http.Client{Timeout: timeout}
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/models", port)
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}
	return parseModelIdentity(body)
}

// BaselineServices 只读检查声明的基线服务（对外入口；内部转调持锁版本）。
func (m *Manager) BaselineServices() []BaselineService {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.baselineServicesLocked()
}

// baselineServicesLocked 的实际实现——**调用方必须已持 m.mu**。
// （doStart 持锁时直接调它；若这里再 Lock 会与不可重入的 sync.Mutex 死锁——已由全包回归抓到过。）
func (m *Manager) baselineServicesLocked() []BaselineService {
	ports := baselinePorts()
	if len(ports) == 0 {
		return nil
	}
	// 本端自己占的端口不算基线（避免把托管项误报成未托管）。
	owned := make(map[int]bool, len(m.procs))
	for _, sp := range m.procs {
		if sp.port > 0 {
			owned[sp.port] = true
		}
	}

	listening := make(map[int]bool)
	for _, p := range probeListeners(ports) {
		listening[p] = true
	}

	out := make([]BaselineService, 0, len(ports))
	for _, port := range ports {
		if owned[port] {
			continue
		}
		svc := BaselineService{
			Port:      port,
			Listening: listening[port],
			Managed:   false,
			Note:      "baseline：未托管的手工服务，只读检查；写动作须经 baseline 声明与租约（P3）",
		}
		if !svc.Listening {
			svc.Note = "baseline：端口未监听（服务未运行或已停）"
			out = append(out, svc)
			continue
		}
		if info, ok := findListenerProcess(port); ok {
			svc.PID = info.PID
			svc.Argv = info.Argv
			svc.Cwd = info.Cwd
			svc.StartTime = info.StartTime
			svc.ApproxGB = info.MemGB
			svc.Kind = kindFromExecutable(firstOr(info.Argv, ""))
			class, unit, _ := serviceClassFrom(info.Cgroup, info.Ancestry)
			svc.Class = class
			svc.Unit = unit
			if class == "" {
				svc.Class = "unknown"
			}
		} else {
			svc.Class = "unknown"
			svc.Note = "baseline：端口在监听但未定位到进程（可能是别的用户/容器）——仍只报告，不动作"
		}
		svc.Identity = probeIdentity(port, 2*time.Second)
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

func firstOr(s []string, def string) string {
	if len(s) > 0 {
		return s[0]
	}
	return def
}

// baselineOccupiedGb 汇总基线服务实测占用（GB）——用于准入扣减（M8）。
func baselineOccupiedGb(svcs []BaselineService) float64 {
	var sum float64
	for _, s := range svcs {
		if s.Listening {
			sum += s.ApproxGB
		}
	}
	return sum
}

// EffectiveAvailableGb 对外入口（加锁）；内部调用请用 effectiveAvailableGbLocked。
func (m *Manager) EffectiveAvailableGb(systemAvailable float64) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.effectiveAvailableGbLocked(systemAvailable)
}

// effectiveAvailableGbLocked 计算"真正可以给新模型用"的可用内存（M8 显式化）：
//
//	min(系统可用, 机型配额) − 预留 − 基线服务实测占用 − 已驻留托管项
//
// 三个入参都可由环境变量覆盖（ZERG_MACHINE_BUDGET_GB / ZERG_BUDGET_RESERVE_GB /
// ZERG_BASELINE_PORTS）；未配置时退化为"只看系统可用"，与既有行为一致。
// **调用方必须已持 m.mu**（它读 m.procs 并调 baselineServicesLocked——再 Lock 会死锁）。
func (m *Manager) effectiveAvailableGbLocked(systemAvailable float64) float64 {
	avail := systemAvailable
	if budget := envFloat(EnvMachineBudgetGB, 0); budget > 0 && budget < avail {
		avail = budget
	}
	avail -= envFloat(EnvBudgetReserveGB, 0)
	if avail < 0 {
		avail = 0
	}
	// 只在"采样看不见显存"的平台才扣（默认不扣 —— 统一内存上扣一次就是双重扣减，
	// 会把可用内存打到 4.3GB 这种数值，导致任何真实模型都装不下。见 deductOccupancyFromAvail）。
	if deductOccupancyFromAvail() {
		avail -= baselineOccupiedGb(m.baselineServicesLocked())
		for _, sp := range m.procs {
			avail -= residentMemGbOf(sp)
		}
	}
	if avail < 0 {
		avail = 0
	}
	return avail
}

// deductOccupancyFromAvail 决定"基线 + 托管驻留项的占用"是否要从**系统可用内存**里再扣一次。
//
// 默认 **false**，理由（2026-09-14 X3 真机实测）：
//
//	MemTotal 122.2 GB、MemAvailable 61.0 GB，而两条基线服务实测占用 56.68 GB ——
//	61.0 正是"122.2 减去（含这 56.68 在内的）已用"的结果 ⇒ **采样口径已经把 GTT/权重算进去了**。
//	再按 M8 原文扣一次就是**双重扣减**：61.0 − 56.68 = 4.3 GB，
//	结果任何真实模型（≥5GB）都被判"内存不足"⇒ 机器实际上装不了东西。
//
// 什么时候该设 true（ZERG_DEDUCT_OCCUPANCY_FROM_AVAIL=1）：
//
//	采样**看不见**显存的平台（离散 GPU：权重在独立 VRAM 里，MemAvailable 里没有它）。
//	统一内存（AMD APU / Apple 统一内存）一律用默认 false。
func deductOccupancyFromAvail() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(EnvDeductOccupancy)))
	return v == "1" || v == "true" || v == "yes"
}

// EnvDeductOccupancy 见 deductOccupancyFromAvail。
const EnvDeductOccupancy = "ZERG_DEDUCT_OCCUPANCY_FROM_AVAIL"

// residentMemGbOf 取一个托管驻留项的实测占用（GB）：GPU 宿主走 drm fdinfo（gtt+vram），
// 取不到才回落 RSS；取不到任何值返回 0（缺席，不编造）。
// 注意：与基线服务同一口径 —— 否则"托管项"和"基线项"两笔扣减会一个准一个离谱（M15）。
func residentMemGbOf(sp *subproc) float64 {
	if sp == nil {
		return 0
	}
	// M10：外部复用项没有进程句柄，现场读不到 ⇒ 用登记时记下的基线实测占用。
	// 少了这条，复用会在内存核算里"凭空消失"23.98GB 这类大块（X3 实测）。
	if sp.external {
		return sp.baselineGB
	}
	if sp.proc == nil || sp.proc.Process == nil {
		return 0
	}
	return readProcessMemGb(sp.proc.Process.Pid)
}

func envFloat(name string, def float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 {
		return def
	}
	return v
}
