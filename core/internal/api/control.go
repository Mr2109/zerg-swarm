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
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/tracectx"

	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

// ControlHandlers 控制端点处理器。
type ControlHandlers struct {
	Token  string
	Fleet  map[string]config.FleetNode
	Models map[string][]config.ModelCandidate
}

// NewControlHandlers 创建控制处理器。
func NewControlHandlers(token string, cfg *config.FleetConfig) *ControlHandlers {
	return &ControlHandlers{
		Token:  token,
		Fleet:  cfg.Fleet,
		Models: cfg.Models,
	}
}

// LoadHandler 处理 POST /api/control/load
// body: {"machine": "x3"|"Mr2109"|"mini1", "model": "deepseek-v4-flash"}
func (h *ControlHandlers) LoadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 POST 方法")
		return
	}

	var req struct {
		Machine string `json:"machine"`
		Model   string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "请求体 JSON 解析失败: "+err.Error())
		return
	}
	if req.Machine == "" || req.Model == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_MACHINE_OR_MODEL", "machine 和 model 字段不能为空")
		return
	}

	// 从路由表找该模型在该机器的候选
	candidates, ok := h.Models[req.Model]
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
		writeErrorCode(w, http.StatusNotFound, "MODEL_NOT_DEPLOYED", fmt.Sprintf("模型 %s 在机器 %s 无部署", req.Model, req.Machine))
		return
	}

	// 本机角色（localback）已于 2026-09-16 退役（Mr2109 拍：所有可推理的计算机都是子端）。
	// ⚠ 必须**显式拒绝**：三处注入已改传 nil ⇒ 若还走进老分支（h.LocalBack.LoadModel）就是
	// 空指针 panic（崩溃比报错糟得多）。本机 = 名为 Mr2109 的普通子端，照远程子端走。
	if req.Machine == "local" {
		writeErrorCode(w, http.StatusGone, "MACHINE_RETIRED",
			"本机角色已退役：本机 = 名为 Mr2109 的普通子端，请用 machine=Mr2109")
		return
	}

	// 远程子端：转发 /load
	node, ok := h.Fleet[req.Machine]
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "MACHINE_NOT_FOUND", fmt.Sprintf("未知机器: %s", req.Machine))
		return
	}
	body, _ := json.Marshal(map[string]string{"model": req.Model})
	status, respBody, err := h.forward(node, "/load", body)
	if err != nil {
		writeErrorCode(w, http.StatusBadGateway, "FORWARD_FAILED", fmt.Sprintf("转发到 %s 失败: %v", req.Machine, err))
		return
	}
	writeJSON(w, status, json.RawMessage(respBody))
}

// UnloadHandler 处理 POST /api/control/unload
// body: {"machine": "x3"|"Mr2109"|"mini1"}（"local" 已退役 ⇒ 410 MACHINE_RETIRED）
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

	node, ok := h.Fleet[req.Machine]
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

	client := &http.Client{Timeout: 60 * time.Second}
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
	return forwardToNode(h.Token, node, path, body)
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
		"started_at": time.Now().Format(time.RFC3339),
		// 版本**同源同值**（§九 M15 `T1`/`T2` · §十二 `P-078` 选 (i)）：与 `/api/capabilities`
		// 的 `version` 用同**一个**真源（`version.Tag`）——原来这里硬编码过一枚第二版本号
		// （一枚「组件名 + 版本主号」的硬编码字面量），那是「同一件东西两处口径」的典型
		// （已红第 2 条）；本版把它换成真源之后，本文件里**不再有**任何版本字面量
		// （判据就是 `grep -n` 那枚字符串 **0 命中** —— 连注释里都不留，否则判据假红）。
		// ★ 旧字段**不删**（键名一个没改，旧消费者零改动）。
		"version": version.Tag,
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
