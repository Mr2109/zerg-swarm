// 控制端点：POST /api/control/load | unload | stop
// 让 UI/客户端可以通过主控对子端与本机后端做加载/卸载/退出操作。
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/localback"
)

// ControlHandlers 控制端点处理器。
type ControlHandlers struct {
	Token     string
	Fleet     map[string]config.FleetNode
	Models    map[string][]config.ModelCandidate
	LocalBack *localback.LocalBackend
}

// NewControlHandlers 创建控制处理器。
func NewControlHandlers(token string, cfg *config.FleetConfig, lb *localback.LocalBackend) *ControlHandlers {
	return &ControlHandlers{
		Token:     token,
		Fleet:     cfg.Fleet,
		Models:    cfg.Models,
		LocalBack: lb,
	}
}

// LoadHandler 处理 POST /api/control/load
// body: {"machine": "x3"|"local"|"mini1", "model": "deepseek-v4-flash"}
func (h *ControlHandlers) LoadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req struct {
		Machine string `json:"machine"`
		Model   string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体 JSON 解析失败: "+err.Error())
		return
	}
	if req.Machine == "" || req.Model == "" {
		writeError(w, http.StatusBadRequest, "machine 和 model 字段不能为空")
		return
	}

	// 从路由表找该模型在该机器的候选
	candidates, ok := h.Models[req.Model]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("未知模型: %s", req.Model))
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
		writeError(w, http.StatusNotFound, fmt.Sprintf("模型 %s 在机器 %s 无部署", req.Model, req.Machine))
		return
	}

	// 本机：走 localback
	if req.Machine == "local" {
		if err := h.LocalBack.LoadModel(candidate.File, int(candidate.MemGb)); err != nil {
			writeError(w, http.StatusInternalServerError, "本机加载失败: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "machine": "local", "model": req.Model})
		return
	}

	// 远程子端：转发 /load
	node, ok := h.Fleet[req.Machine]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("未知机器: %s", req.Machine))
		return
	}
	body, _ := json.Marshal(map[string]string{"model": req.Model})
	status, respBody, err := h.forward(node, "/load", body)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("转发到 %s 失败: %v", req.Machine, err))
		return
	}
	writeJSON(w, status, json.RawMessage(respBody))
}

// UnloadHandler 处理 POST /api/control/unload
// body: {"machine": "x3"|"local"|"mini1"}
func (h *ControlHandlers) UnloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req struct {
		Machine string `json:"machine"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体 JSON 解析失败: "+err.Error())
		return
	}
	if req.Machine == "" {
		writeError(w, http.StatusBadRequest, "machine 字段不能为空")
		return
	}

	// 本机：走 localback
	if req.Machine == "local" {
		h.LocalBack.Stop()
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "machine": "local"})
		return
	}

	node, ok := h.Fleet[req.Machine]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("未知机器: %s", req.Machine))
		return
	}
	status, respBody, err := h.forward(node, "/unload", nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("转发到 %s 失败: %v", req.Machine, err))
		return
	}
	writeJSON(w, status, json.RawMessage(respBody))
}

// StopHandler 处理 POST /api/control/stop —— 主控自身优雅退出。
// 延迟 300ms 退出，保证响应先送达。
func (h *ControlHandlers) StopHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "主控即将退出"})
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()
}

// forward 转发请求到子端 agent（带认证头）。
func (h *ControlHandlers) forward(node config.FleetNode, path string, body []byte) (int, []byte, error) {
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
	req.Header.Set("X-Auth-Token", h.Token)

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

// CoreStatusHandler 处理 GET /api/core/status——主控自身状态。
// 返回主控进程 PID、启动时间、内存占用（供 UI 主控模块显示）。
func (h *ControlHandlers) CoreStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}
	pid := os.Getpid()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":            true,
		"pid":           pid,
		"started_at":    time.Now().Format(time.RFC3339),
		"version":       "zerg-core v2",
		"local_backend": h.LocalBack.State(),
	})
}

// CoreLogsHandler 处理 GET /api/core/logs——主控最近日志。
// 从本地日志文件 /tmp/zerg-core.log 读取尾部 N 行（不存在则返回空）。
func (h *ControlHandlers) CoreLogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}
	const logPath = "/tmp/zerg-core.log"
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
