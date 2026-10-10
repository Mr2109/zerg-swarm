// 控制端点：POST /api/control/load | unload | stop
// 让 UI/客户端可以通过主控对子端与本机后端做加载/卸载/退出操作。
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/tracectx"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// coreProcessStartedAt —— 主控进程**真启动时刻**（进程级：包初始化时取一次，此后不随请求变）。
// 缺口 GAP-20260928-47：本字段原先写成 time.Now()（= 本次请求处理时刻）⇒ 「拿 started_at 判
// 有没有重启过」恒判不了（每次读都在变）。改成进程启动时刻后，重启 ⇒ 该值与 pid 一起翻新。
var coreProcessStartedAt = time.Now()

// ControlHandlers 控制端点处理器。
type ControlHandlers struct {
	Token  string
	Fleet  map[string]config.FleetNode
	Models map[string][]config.ModelCandidate

	// cfgSource —— 缺口 GAP-20260925-42：构造时 Fleet/Models 是**值拷贝**，而
	//   `POST /api/config/reload` 只把主控的 `Handlers.Config` 指向**新对象**
	//   （handlers.go ReloadConfigHandler: `h.Config = newCfg`）⇒ 本处理器永远看不到
	//   reload 后的新条目（`model add` 写册 + reload 报 ok，孵卵面照样 404）。
	//   非 nil 时每次请求**现读**当前配置（与其余 API 面同源）；nil 时退回构造时快照（向后兼容）。
	cfgSource func() *config.FleetConfig

	// loadMu / loadTasks / loadOrder —— 装载任务登记表（缺口 GAP-20260925-44）。
	//   `POST /api/control/load` 是**异步受理**：请求一受理即回 `accepted + task_id`，真正的装载在
	//   后台 goroutine 里跑；本表就是那张**只读查询面**的底账（`GET /api/core/status` 的
	//   `load_tasks`）。phase 三态：accepted（已受理·后台装载中）/ ok（装载返回成功）/
	//   failed（装载失败，带原因）⇒ 调用方能区分「还在冷装」与「起不来」（「**已受理 ≠ 已完成**」）。
	loadMu    sync.Mutex
	loadTasks map[string]*loadTaskRec
	loadOrder []string
}

// NewControlHandlers 创建控制处理器。
func NewControlHandlers(token string, cfg *config.FleetConfig) *ControlHandlers {
	return &ControlHandlers{
		Token:  token,
		Fleet:  cfg.Fleet,
		Models: cfg.Models,
	}
}

// SetConfigSource 注入口（缺口 GAP-20260925-42）：让本处理器与主控的 `Handlers.Config`
// 同源 —— 装配处传入 `func() *config.FleetConfig { return handlers.Config }`。此后
// `config reload` 换掉 `Handlers.Config` 的**同一刻**，本处理器即看到新条目。
func (h *ControlHandlers) SetConfigSource(fn func() *config.FleetConfig) {
	h.cfgSource = fn
}

// currentConfig 现读当前配置：有注入口用注入口（reload 后同步），否则用构造时快照。
func (h *ControlHandlers) currentConfig() *config.FleetConfig {
	if h.cfgSource != nil {
		if c := h.cfgSource(); c != nil {
			return c
		}
	}
	return &config.FleetConfig{
		Auth:   config.AuthConfig{Token: h.Token},
		Fleet:  h.Fleet,
		Models: h.Models,
	}
}

// ── 装载任务（缺口 GAP-20260925-44）─────────────────────────────────────────────────────────
//
// `POST /api/control/load` 是**异步受理**：请求一受理即回 `accepted + task_id`，真正的装载在
// 后台 goroutine 里跑。本区就是那张**只读查询面**的底账 —— `GET /api/core/status` 的
// `load_tasks`（最近 N 枚）读出 `phase`：accepted（已受理·后台装载中）/ ok（装载返回成功）/
// failed（装载失败，带原因）⇒ 调用方能区分「还在冷装」与「起不来」。
const (
	phaseAccepted = "accepted"
	phaseOK       = "ok"
	phaseFailed   = "failed"
	// loadTasksKept —— 只读面保留的最近任务枚数（够复盘，不无限涨）。
	loadTasksKept = 64
	// loadDispatchTimeout —— 后台装载转发的超时：**必须 >= 子端装载截止（120s）+ 余量**，
	//   否则又回到改前「主控先放弃（60s）、子端仍在装（120s）」的错配（本账父代理补记）。
	loadDispatchTimeout = 300 * time.Second
)

// loadTaskRec —— 一枚装载任务（内部态 · `startedAt` 用 time.Time 好算 elapsed）。
type loadTaskRec struct {
	id         string
	machine    string
	model      string
	phase      string
	startedAt  time.Time
	finishedAt time.Time
	backend    string
	port       int
	errMsg     string
}

// loadTaskSeq —— 任务号序号（进程级原子计数，与纳秒时间一起保证唯一）。
var loadTaskSeq uint64

// newLoadTaskID —— 造一枚任务号（形状：`load-<纳秒>-<序号>-<机>-<模型>`）。
func newLoadTaskID(machine, model string) string {
	n := atomic.AddUint64(&loadTaskSeq, 1)
	return fmt.Sprintf("load-%d-%d-%s-%s", time.Now().UnixNano(), n, machine, model)
}

// addLoadTask 登记一枚新任务（**受理**那一刻调用）。
func (h *ControlHandlers) addLoadTask(t *loadTaskRec) {
	h.loadMu.Lock()
	defer h.loadMu.Unlock()
	if h.loadTasks == nil {
		h.loadTasks = map[string]*loadTaskRec{}
	}
	h.loadTasks[t.id] = t
	h.loadOrder = append(h.loadOrder, t.id)
	if len(h.loadOrder) > loadTasksKept {
		dropped := h.loadOrder[0]
		h.loadOrder = h.loadOrder[1:]
		delete(h.loadTasks, dropped)
	}
}

// finishLoadTask 结算一枚任务（**后台装载返回**时调用）。
func (h *ControlHandlers) finishLoadTask(id, phase, errMsg, backend string, port int) {
	h.loadMu.Lock()
	defer h.loadMu.Unlock()
	t := h.loadTasks[id]
	if t == nil {
		return
	}
	t.phase = phase
	t.errMsg = errMsg
	t.backend = backend
	t.port = port
	t.finishedAt = time.Now()
}

// snapshotLoadTasks 只读快照（最近 loadTasksKept 枚 · 供 `GET /api/core/status`）。
func (h *ControlHandlers) snapshotLoadTasks() []map[string]interface{} {
	h.loadMu.Lock()
	defer h.loadMu.Unlock()
	out := make([]map[string]interface{}, 0, len(h.loadOrder))
	now := time.Now()
	for _, id := range h.loadOrder {
		t := h.loadTasks[id]
		if t == nil {
			continue
		}
		end := now
		if !t.finishedAt.IsZero() {
			end = t.finishedAt
		}
		row := map[string]interface{}{
			"task_id":    t.id,
			"machine":    t.machine,
			"model":      t.model,
			"phase":      t.phase,
			"started_at": t.startedAt.Format(time.RFC3339),
			"elapsed_s":  end.Sub(t.startedAt).Seconds(),
		}
		if !t.finishedAt.IsZero() {
			row["finished_at"] = t.finishedAt.Format(time.RFC3339)
		}
		if t.backend != "" {
			row["backend"] = t.backend
		}
		if t.port != 0 {
			row["port"] = t.port
		}
		if t.errMsg != "" {
			row["error"] = t.errMsg
		}
		out = append(out, row)
	}
	return out
}

// loadAcceptWindow —— 「短窗」：请求受理后，主控在这个窗口内等一次**已可判定**的结果 ——
//
//	结果到了就**当场据实回**（成功只增不改地补 accepted…；失败原样透传且**不**回 accepted ⇒ 负控）；
//	窗口内没到（冷装/慢装）⇒ 回「已受理 + 任务号」，结果留后台结算 —— 这才是「保持异步」。
//	取 2s：够覆盖「未知/拒装/已驻留复用」这类**同步可判定**的返回，又远短于冷装（实测 >45s）。
var loadAcceptWindow = 2 * time.Second

// loadResult —— 一次后台装载转发的结果。
type loadResult struct {
	status   int
	respBody []byte
	err      error
}

// loadBodyFailed —— 子端响应体自称失败（`ok:false`）。
func loadBodyFailed(b []byte) bool {
	var m map[string]interface{}
	if json.Unmarshal(b, &m) != nil {
		return false
	}
	okv, has := m["ok"]
	return has && okv == false
}

// loadAcceptedBody —— 受理面的响应体（**只增不改**）：子端原体（若已有）逐键保留，再补
//
//	ok / accepted / task_id / started_at / machine / model / note。
func loadAcceptedBody(machine, model, taskID string, started time.Time, subend map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range subend { // 子端原体逐键保留（零破坏）
		out[k] = v
	}
	out["ok"] = true // `ok` 原样保留（语义收紧为「已受理」）
	out["accepted"] = true
	out["task_id"] = taskID
	out["started_at"] = started.Format(time.RFC3339)
	out["machine"] = machine
	out["model"] = model
	out["note"] = "已受理 ≠ 已完成：装载在后台进行，用 GET /api/core/status 的 load_tasks 查 phase（accepted|ok|failed）"
	return out
}

// dispatchLoad —— 后台装载：把 `/load` 转发给子端，并把**真结果**结算进任务表（供只读面查）。
// 它的整段都可能慢（冷装 >45s）⇒ 主控只在「短窗」内等它，超窗即走「已受理 + 任务号」那条路。
func (h *ControlHandlers) dispatchLoad(node config.FleetNode, machine, model string, body []byte, taskID string) loadResult {
	status, respBody, err := forwardToNodeTimeout(h.currentConfig().Auth.Token, node, "/load", body, loadDispatchTimeout)
	r := loadResult{status: status, respBody: respBody, err: err}
	phase, errMsg, backend, port := phaseOK, "", "", 0
	switch {
	case err != nil:
		phase, errMsg = phaseFailed, err.Error()
	case status >= 400:
		phase, errMsg = phaseFailed, fmt.Sprintf("子端拒装（http_%d）：%s", status, strings.TrimSpace(string(respBody)))
	case loadBodyFailed(respBody):
		phase, errMsg = phaseFailed, strings.TrimSpace(string(respBody))
	default:
		var m map[string]interface{}
		if json.Unmarshal(respBody, &m) == nil {
			if b, ok := m["backend"].(string); ok {
				backend = b
			}
			if p, ok := m["port"].(float64); ok {
				port = int(p)
			}
		}
	}
	h.finishLoadTask(taskID, phase, errMsg, backend, port)
	return r
}

// LoadHandler 处理 POST /api/control/load
// body: {"machine": "node-a"|"node-local"|"node-b", "model": "deepseek-v4-flash"}
func (h *ControlHandlers) LoadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 POST 方法")
		return
	}

	var req struct {
		Machine string `json:"machine"`
		Model   string `json:"model"`
		// 单卵时限（GAP-20260926-201 全通路）：CLI `--timeout` ⇒ 本字段 ⇒ **透传**进子端
		// /load 体（✗ 不许在下面重造体时丢掉）。单位 = **秒**（与子端 env `ZERG_EGG_MAX_S` 同口径）。
		EggMaxS string `json:"egg_max_s"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "请求体 JSON 解析失败: "+err.Error())
		return
	}
	if req.Machine == "" || req.Model == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_MACHINE_OR_MODEL", "machine 和 model 字段不能为空")
		return
	}

	// 从路由表找该模型在该机器的候选（现读配置：reload 后与主控同源）
	cfg := h.currentConfig()
	candidates, ok := cfg.Models[req.Model]
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "MODEL_NOT_FOUND", fmt.Sprintf("未知模型: %s", req.Model))
		return
	}
	var candidate *config.ModelCandidate
	for i := range candidates {
		if candidates[i].Host == req.Machine {
			candidate = &candidates[i]
			break
		}
	}
	if candidate == nil {
		// 缺口 GAP-20260925-48：把「机器名不存在」与「该机没部署此模型」分成**两档**报 —— 此前二者
		//   同回 MODEL_NOT_DEPLOYED + 原样回填请求里的 machine 串（调用方传来未展开的 `${USER}` 时，
		//   报错里机器名就是字面量 `${USER}`，既看不出是哪台机、也分不清「机器名写错」与「该机确无
		//   部署」）；后者再**列出该模型实际有部署的机器**，让调用方能一眼改对。code/status 不变。
		if _, ok := cfg.Fleet[req.Machine]; !ok && req.Machine != "local" {
			writeErrorCode(w, http.StatusNotFound, "MACHINE_NOT_FOUND",
				fmt.Sprintf("机器 %s 不存在（名册里没有这台机）—— 这**不是**「该机没部署此模型」", req.Machine))
			return
		}
		hosts := make([]string, 0, len(candidates))
		for i := range candidates {
			hosts = append(hosts, candidates[i].Host)
		}
		writeErrorCode(w, http.StatusNotFound, "MODEL_NOT_DEPLOYED",
			fmt.Sprintf("模型 %s 在机器 %s 无部署；该模型实际有部署的机器：%s", req.Model, req.Machine, strings.Join(hosts, " / ")))
		return
	}

	// 本机角色（localback）已于 2026-09-16 退役（Mr2109 拍：所有可推理的计算机都是子端）。
	// ⚠ 必须**显式拒绝**：三处注入已改传 nil ⇒ 若还走进老分支（h.LocalBack.LoadModel）就是
	// 空指针 panic（崩溃比报错糟得多）。本机 = 名为 node-local 的普通子端，照远程子端走。
	if req.Machine == "local" {
		writeErrorCode(w, http.StatusGone, "MACHINE_RETIRED",
			"本机角色已退役：本机 = 名为 Mr2109 的普通子端，请用 machine=Mr2109")
		return
	}

	// 远程子端：**受理**装载请求（缺口 GAP-20260925-44 —— 保持异步，回「已受理 + 任务号」）。
	node, ok := cfg.Fleet[req.Machine]
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "MACHINE_NOT_FOUND", fmt.Sprintf("未知机器: %s", req.Machine))
		return
	}
	// 改前：主控**同步**转发 `/load` 并把子端响应体原样回出去 —— 调用方看到 `{"ok":true,…}`
	//   无从区分「引擎还在冷装」与「引擎起不来」（本账症状）；且主控转发超时 60s 短于子端装载
	//   截止 120s ⇒ 冷装 >60s 时主控先回 502 FORWARD_FAILED，而子端仍在装。
	// 改后（Mr2109 拍：**保持异步**）：请求一受理即回「已受理 + 任务号」，真正的装载在后台跑
	//   （超时放宽到 >= 子端截止）；只读查询面 = `GET /api/core/status` 的 `load_tasks`。
	// ★ 响应**只增不改**：`ok` 保留（语义收紧为「已受理」），新增 accepted / task_id / started_at /
	//   machine / model / note；子端原体（backend/port…）异步下即时不可得，装载完成后由
	//   `load_tasks` 那一枚任务补齐。
	// ★ 负控：**受理前**的前置拒执一字未动（未知模型 404 MODEL_NOT_FOUND · 未部署
	//   MODEL_NOT_DEPLOYED · 机器不存在 MACHINE_NOT_FOUND · 本机已退役 410 · 体坏 400）⇒
	//   这些路径**不**出 `accepted`（装载失败 = 未受理，不得回 accepted）。
	loadBody := map[string]string{"model": req.Model}
	if req.EggMaxS != "" {
		// 透传（GAP-20260926-201）：CLI 给的单卵时限**原样**带给子端 —— 重造体时不许丢。
		loadBody["egg_max_s"] = req.EggMaxS
	}
	body, _ := json.Marshal(loadBody)
	taskID := newLoadTaskID(req.Machine, req.Model)
	started := time.Now()
	h.addLoadTask(&loadTaskRec{
		id: taskID, machine: req.Machine, model: req.Model,
		phase: phaseAccepted, startedAt: started,
	})
	// 后台装载（**保持异步**）：请求一受理即开跑；主控只在「短窗」内等一次**已可判定**的结果 ——
	// 结果到了就当场据实回（成功补 accepted…；失败原样透传且**不**回 accepted ⇒ 负控）。
	resCh := make(chan loadResult, 1)
	go func() { resCh <- h.dispatchLoad(node, req.Machine, req.Model, body, taskID) }()
	var res loadResult
	select {
	case res = <-resCh:
		// 短窗内已有结果 ⇒ 走下面据实回（异步只在「结果没到」时才需要）。
	case <-time.After(loadAcceptWindow):
		// 装载仍在跑（冷装/慢装）⇒ 「已受理 + 任务号」；结果留后台结算，用只读面查。
		writeJSON(w, http.StatusOK, loadAcceptedBody(req.Machine, req.Model, taskID, started, nil))
		return
	}
	if res.err != nil {
		// 负控：打不到子端 = 装载**未受理** ⇒ **不**回 accepted（原样报 502 FORWARD_FAILED）。
		writeErrorCode(w, http.StatusBadGateway, "FORWARD_FAILED", fmt.Sprintf("转发到 %s 失败: %v", req.Machine, res.err))
		return
	}
	if res.status >= 400 || loadBodyFailed(res.respBody) {
		// 负控：子端**同步拒装**（http ≥400 / ok:false）⇒ 原样透传，**不**回 accepted。
		writeJSON(w, res.status, json.RawMessage(res.respBody))
		return
	}
	var subend map[string]interface{}
	_ = json.Unmarshal(res.respBody, &subend)
	writeJSON(w, http.StatusOK, loadAcceptedBody(req.Machine, req.Model, taskID, started, subend))
}

// UnloadHandler 处理 POST /api/control/unload
// body: {"machine": "node-a"|"node-local"|"node-b"}（"local" 已退役 ⇒ 410 MACHINE_RETIRED）
func (h *ControlHandlers) UnloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 POST 方法")
		return
	}

	var req struct {
		Machine string `json:"machine"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "请求体 JSON 解析失败: "+err.Error())
		return
	}
	if req.Machine == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_MACHINE", "machine 字段不能为空")
		return
	}

	// 本机角色已退役（同上）：显式拒绝，绝不 deref nil 的 LocalBack。
	if req.Machine == "local" {
		writeErrorCode(w, http.StatusGone, "MACHINE_RETIRED",
			"本机角色已退役：本机 = 名为 Mr2109 的普通子端，请用 machine=Mr2109")
		return
	}

	cfg := h.currentConfig()
	node, ok := cfg.Fleet[req.Machine]
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "MACHINE_NOT_FOUND", fmt.Sprintf("未知机器: %s", req.Machine))
		return
	}
	status, respBody, err := h.forward(node, "/unload", nil)
	if err != nil {
		writeErrorCode(w, http.StatusBadGateway, "FORWARD_FAILED", fmt.Sprintf("转发到 %s 失败: %v", req.Machine, err))
		return
	}
	writeJSON(w, status, json.RawMessage(respBody))
}

// StopHandler 处理 POST /api/control/stop —— 主控自身优雅退出。
// 延迟 300ms 退出，保证响应先送达。
func (h *ControlHandlers) StopHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 POST 方法")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "主控即将退出"})
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()
}

// forward 转发请求到子端 agent（带认证头）。
// forwardToNode 向一台子端发一条控制请求（/load、/unload 等）并回传其响应。
//
// 3c（2026-09-16）：从 ControlHandlers.forward 抽出为**包级函数**，供 Handlers 复用
// （步2：handlers 的"手动启动/停止模型"要从"主控自己起引擎"改成"转发到目标子端" ⇒ 同一实现，不复制）。
func forwardToNode(token string, node config.FleetNode, path string, body []byte) (int, []byte, error) {
	return forwardToNodeTimeout(token, node, path, body, 60*time.Second)
}

// forwardToNodeTimeout —— forwardToNode 的可配超时版（缺口 GAP-20260925-44：后台装载那条路
// 要用 >= 子端装载截止（120s）的超时，不能再吃 60s 的默认档 —— 否则主控先放弃、子端仍在装）。
func forwardToNodeTimeout(token string, node config.FleetNode, path string, body []byte, timeout time.Duration) (int, []byte, error) {
	url := fmt.Sprintf("http://%s:%d%s", node.Host, node.Port, path)
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(http.MethodPost, url, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", token)
	// T1.6 传播（控制面出站）：/load、/unload、/stop 也是"主控→子端"，一并带 traceparent。
	// 控制面没有会话上下文（不是某轮对话的一部分）⇒ 本侧新生成一条 root trace 是**如实**的：
	// 这次控制操作就是一条独立的链，不该硬塞进某个会话的链里。
	tracectx.Propagate(req.Header, nil, "", tracectx.ReplayMarked())

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, respBody, nil
}

// forward 控制面自用：委托给包级 forwardToNode（保持既有调用点不变）。
func (h *ControlHandlers) forward(node config.FleetNode, path string, body []byte) (int, []byte, error) {
	return forwardToNode(h.currentConfig().Auth.Token, node, path, body)
}

// CoreStatusHandler 处理 GET /api/core/status——主控自身状态。
// 返回主控进程 PID、启动时间、内存占用（供 UI 主控模块显示）。
func (h *ControlHandlers) CoreStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 GET 方法")
		return
	}
	pid := os.Getpid()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"pid":        pid,
		"started_at": coreProcessStartedAt.Format(time.RFC3339),
		// 版本**同源同值**（§九 M15 `T1`/`T2` · §十二 `P-078` 选 (i)）：与 `/api/capabilities`
		// 的 `version` 用同**一个**真源（`version.Tag`）——原来这里硬编码过一枚第二版本号
		// （一枚「组件名 + 版本主号」的硬编码字面量），那是「同一件东西两处口径」的典型
		// （已红第 2 条）；本版把它换成真源之后，本文件里**不再有**任何版本字面量
		// （判据就是 `grep -n` 那枚字符串 **0 命中** —— 连注释里都不留，否则判据假红）。
		// ★ 旧字段**不删**（键名一个没改，旧消费者零改动）。
		"version": version.Tag,
		// ★ 缺口 GAP-20260925-44：只读查询面 —— 装载任务登记表（最近 N 枚 · phase
		//   accepted|ok|failed）。复用**既有**面 `GET /api/core/status`（不新开路由、不碰任何
		//   `main.go`）；`POST /api/control/load` 回的 `task_id` 可在这里查到它的进展。
		"load_tasks": h.snapshotLoadTasks(),
		// 3c：原 "local_backend" 字段（LocalBackend 状态）已删 —— 本机角色退役，引擎由子端托管。
	})
}

// CoreLogsHandler 处理 GET /api/core/logs——主控最近日志。
// 从本地日志文件 /tmp/zerg-core.log 读取尾部 N 行（不存在则返回空）。
func (h *ControlHandlers) CoreLogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 GET 方法")
		return
	}
	logPath := filepath.Join(statepath.RuntimeLogDir(), "zerg-core.log")
	lines, err := tailFile(logPath, 200)
	if err != nil {
		// 日志文件不存在不算错误，返回空
		writeJSON(w, http.StatusOK, map[string]interface{}{"logs": []string{}, "path": logPath})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"logs": lines, "path": logPath})
}

// tailFile 读取文件末尾 n 行。
func tailFile(path string, n int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	all := strings.Split(string(data), "\n")
	if len(all) > n {
		all = all[len(all)-n:]
	}
	// 去掉末尾空行
	for len(all) > 0 && strings.TrimSpace(all[len(all)-1]) == "" {
		all = all[:len(all)-1]
	}
	return all, nil
}
