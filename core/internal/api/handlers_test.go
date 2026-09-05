package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"zerg/core/internal/config"
	"zerg/core/internal/store"
)

// ===== 辅助函数 =====

func newTestHandlers() *Handlers {
	return &Handlers{
		Config: &config.FleetConfig{
			Models: map[string][]config.ModelCandidate{
				"test-model": {
					{Host: "127.0.0.1", Backend: "llama-server", MemGb: 8},
				},
			},
			Fleet: map[string]config.FleetNode{},
		},
		Store:           store.NewStore(),
		ConfigPath:      "",
		HeartbeatLogger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func newTestHandlersWithConfigPath(path string) *Handlers {
	return &Handlers{
		Config: &config.FleetConfig{
			Models: map[string][]config.ModelCandidate{},
			Fleet:  map[string]config.FleetNode{},
		},
		Store:           store.NewStore(),
		ConfigPath:      path,
		HeartbeatLogger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// ===== 测试 1: HeartbeatHandler =====

func TestHeartbeatHandler_ValidHeartbeat(t *testing.T) {
	h := newTestHandlers()

	reqBody := store.HeartbeatRequest{
		Machine:        "node-1",
		Model:          strPtr("llama-7b"),
		Backend:        strPtr("llama-server"),
		Port:           intPtr(8080),
		MemAvailableGb: 16.0,
		MemTotalGb:     32.0,
		Load:           0.5,
		Healthy:        true,
		BackendState:   "running",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/fleet/heartbeat", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HeartbeatHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp store.HeartbeatResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.OK != true {
		t.Errorf("expected OK true, got %v", resp.OK)
	}

	// 验证 store 中确实存储了快照
	snapshots := h.Store.GetAllSnapshots()
	snap, ok := snapshots["node-1"]
	if !ok {
		t.Fatal("expected snapshot for 'node-1' in store")
	}
	if snap.MemAvailableGb != 16.0 {
		t.Errorf("expected MemAvailableGb 16.0, got %f", snap.MemAvailableGb)
	}
	if !snap.Healthy {
		t.Error("expected snapshot to be healthy")
	}
}

func TestHeartbeatHandler_MissingMachine(t *testing.T) {
	h := newTestHandlers()

	reqBody := store.HeartbeatRequest{
		Machine:        "",
		MemAvailableGb: 8.0,
		MemTotalGb:     16.0,
		Load:           0.1,
		Healthy:        true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/fleet/heartbeat", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HeartbeatHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "machine") {
		t.Errorf("expected error message about 'machine', got '%s'", resp["error"])
	}
}

func TestHeartbeatHandler_InvalidMethod(t *testing.T) {
	h := newTestHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/heartbeat", nil)
	w := httptest.NewRecorder()

	h.HeartbeatHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHeartbeatHandler_InvalidJSON(t *testing.T) {
	h := newTestHandlers()

	req := httptest.NewRequest(http.MethodPost, "/api/fleet/heartbeat", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	h.HeartbeatHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// ===== 测试 2: AuthMiddleware =====

func TestAuthMiddleware_ValidToken(t *testing.T) {
	mw := AuthMiddleware("secret-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/status", nil)
	req.Header.Set("X-Auth-Token", "secret-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	mw := AuthMiddleware("secret-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/status", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "X-Auth-Token") {
		t.Errorf("expected error about X-Auth-Token, got '%s'", resp["error"])
	}
}

func TestAuthMiddleware_WrongToken(t *testing.T) {
	mw := AuthMiddleware("secret-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/status", nil)
	req.Header.Set("X-Auth-Token", "wrong-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "无效") {
		t.Errorf("expected error about invalid token, got '%s'", resp["error"])
	}
}

func TestAuthMiddleware_NonAPIPath(t *testing.T) {
	mw := AuthMiddleware("secret-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d (pass-through), got %d", http.StatusOK, w.Code)
	}
}

// ===== 测试 3: StatusHandler =====

func TestStatusHandler_EmptyFleet(t *testing.T) {
	h := newTestHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/status", nil)
	w := httptest.NewRecorder()

	h.StatusHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	totalMachines, ok := resp["total_machines"].(float64)
	if !ok || totalMachines != 0 {
		t.Errorf("expected 0 total machines, got %v", resp["total_machines"])
	}

	healthyCount, ok := resp["healthy_count"].(float64)
	if !ok || healthyCount != 0 {
		t.Errorf("expected 0 healthy count, got %v", resp["healthy_count"])
	}
}

func TestStatusHandler_WithHeartbeatData(t *testing.T) {
	h := newTestHandlers()

	// 先发送心跳
	reqBody := store.HeartbeatRequest{
		Machine:        "node-2",
		MemAvailableGb: 32.0,
		MemTotalGb:     64.0,
		Load:           0.3,
		Healthy:        true,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/fleet/heartbeat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HeartbeatHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("heartbeat should succeed, got %d", w.Code)
	}

	// 查询状态
	statusReq := httptest.NewRequest(http.MethodGet, "/api/fleet/status", nil)
	statusW := httptest.NewRecorder()
	h.StatusHandler(statusW, statusReq)

	if statusW.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, statusW.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(statusW.Body).Decode(&resp)

	totalMachines := resp["total_machines"].(float64)
	if totalMachines != 1 {
		t.Errorf("expected 1 total machine, got %f", totalMachines)
	}

	healthyCount := resp["healthy_count"].(float64)
	if healthyCount != 1 {
		t.Errorf("expected 1 healthy machine, got %f", healthyCount)
	}
}

func TestStatusHandler_InvalidMethod(t *testing.T) {
	h := newTestHandlers()

	req := httptest.NewRequest(http.MethodPost, "/api/fleet/status", nil)
	w := httptest.NewRecorder()

	h.StatusHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// ===== 测试 4: ModelsHandler =====

func TestModelsHandler_ReturnsModels(t *testing.T) {
	h := newTestHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/models", nil)
	w := httptest.NewRecorder()

	h.ModelsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	count := resp["count"].(float64)
	if count != 1 {
		t.Errorf("expected 1 model, got %f", count)
	}

	models := resp["models"].([]interface{})
	if len(models) != 1 {
		t.Fatalf("expected 1 model entry, got %d", len(models))
	}

	model := models[0].(map[string]interface{})
	if model["id"] != "test-model" {
		t.Errorf("expected model id 'test-model', got '%v'", model["id"])
	}
}

func TestModelsHandler_InvalidMethod(t *testing.T) {
	h := newTestHandlers()

	req := httptest.NewRequest(http.MethodPost, "/api/fleet/models", nil)
	w := httptest.NewRecorder()

	h.ModelsHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// ===== 测试 5: TasksHandler =====

func TestTasksHandler_EmptyTasks(t *testing.T) {
	h := newTestHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/tasks", nil)
	w := httptest.NewRecorder()

	h.TasksHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	count := resp["count"].(float64)
	if count != 0 {
		t.Errorf("expected 0 tasks, got %f", count)
	}
}

func TestTasksHandler_WithTask(t *testing.T) {
	h := newTestHandlers()

	h.Store.CreateTask("llama-7b", "node-1")

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/tasks", nil)
	w := httptest.NewRecorder()

	h.TasksHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	count := resp["count"].(float64)
	if count != 1 {
		t.Errorf("expected 1 task, got %f", count)
	}
}

// ===== 测试 6: LogsHandler =====

func TestLogsHandler_ValidLog(t *testing.T) {
	h := newTestHandlers()

	logEntry := map[string]interface{}{
		"machine": "node-1",
		"level":   "INFO",
		"message": "test log message",
	}
	body, _ := json.Marshal(logEntry)

	req := httptest.NewRequest(http.MethodPost, "/api/fleet/logs", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.LogsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestLogsHandler_NonJSONBody(t *testing.T) {
	h := newTestHandlers()

	req := httptest.NewRequest(http.MethodPost, "/api/fleet/logs", strings.NewReader("plain text log"))
	w := httptest.NewRecorder()

	h.LogsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestLogsHandler_InvalidMethod(t *testing.T) {
	h := newTestHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/logs", nil)
	w := httptest.NewRecorder()

	h.LogsHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// ===== 测试 7: ReloadConfigHandler =====

func TestReloadConfigHandler_NoConfigPath(t *testing.T) {
	h := newTestHandlers()

	req := httptest.NewRequest(http.MethodPost, "/api/config/reload", nil)
	w := httptest.NewRecorder()

	h.ReloadConfigHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestReloadConfigHandler_InvalidPath(t *testing.T) {
	h := newTestHandlersWithConfigPath("/nonexistent/fleet.yaml")

	req := httptest.NewRequest(http.MethodPost, "/api/config/reload", nil)
	w := httptest.NewRecorder()

	h.ReloadConfigHandler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// ===== 测试 8: 多节点心跳 =====

func TestHeartbeatHandler_MultipleMachines(t *testing.T) {
	h := newTestHandlers()

	machines := []string{"node-a", "node-b", "node-c"}
	for _, machine := range machines {
		reqBody := store.HeartbeatRequest{
			Machine:        machine,
			MemAvailableGb: 8.0,
			MemTotalGb:     16.0,
			Load:           0.2,
			Healthy:        true,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/fleet/heartbeat", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.HeartbeatHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("heartbeat for %s: expected %d, got %d", machine, http.StatusOK, w.Code)
		}
	}

	snapshots := h.Store.GetAllSnapshots()
	for _, machine := range machines {
		if _, ok := snapshots[machine]; !ok {
			t.Errorf("expected snapshot for %s in store", machine)
		}
	}
}

// ===== 测试 9: AuthMiddleware 完整保护测试 =====

func TestAuthMiddleware_ProtectsAPI(t *testing.T) {
	mw := AuthMiddleware("correct-token")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("protected content"))
	}))

	// 无 token → 401
	req1 := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Errorf("no token: expected 401, got %d", w1.Code)
	}

	// 正确 token → 200
	req2 := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req2.Header.Set("X-Auth-Token", "correct-token")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("valid token: expected 200, got %d", w2.Code)
	}
	if w2.Body.String() != "protected content" {
		t.Errorf("expected 'protected content', got '%s'", w2.Body.String())
	}
}

// ===== 辅助函数 =====

func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func init() {
	_ = time.Now
}
