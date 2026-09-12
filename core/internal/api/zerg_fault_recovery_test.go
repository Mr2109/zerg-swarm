package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ========== v2.5.6 故障自愈测试（Mr2109 2026-08-28） ==========

// TestClassifyGatewayError_JSON 网关返回 JSON 错误（带 type code）→ 正确解析
func TestClassifyGatewayError_JSON(t *testing.T) {
	// 熔断——环境故障（可等）
	body := `{"error":{"message":"模型 Qwen3.8-27B 无可用候选","type":"circuit_open"}}`
	err := classifyGatewayError(503, body)
	if !isCircuitOpen(err) {
		t.Fatalf("circuit_open 应识别为熔断: %v", err)
	}
	if !isEnvFault(err) {
		t.Fatalf("circuit_open 应算环境故障: %v", err)
	}
	if isRetryable(err) {
		t.Fatalf("circuit_open 不应算可重试（等恢复——不消耗重试次数）: %v", err)
	}

	// 转发失败——可重试
	body2 := `{"error":{"message":"backend timeout","type":"upstream_fail"}}`
	err2 := classifyGatewayError(502, body2)
	if isCircuitOpen(err2) {
		t.Fatalf("upstream_fail 不应算熔断: %v", err2)
	}
	if !isEnvFault(err2) {
		t.Fatalf("upstream_fail 应算环境故障: %v", err2)
	}
	if !isRetryable(err2) {
		t.Fatalf("upstream_fail 应算可重试: %v", err2)
	}

	// 参数错——不可重试
	body3 := `{"error":{"message":"bad request","type":"bad_request"}}`
	err3 := classifyGatewayError(400, body3)
	if isEnvFault(err3) {
		t.Fatalf("bad_request 不应算环境故障: %v", err3)
	}
	if isRetryable(err3) {
		t.Fatalf("bad_request 不应算可重试: %v", err3)
	}
}

// TestClassifyGatewayError_StatusFallback 无 JSON code——按 HTTP 状态码兜底
func TestClassifyGatewayError_StatusFallback(t *testing.T) {
	// 503 → circuit_open（熔断）
	err := classifyGatewayError(http.StatusServiceUnavailable, "service unavailable")
	if !isCircuitOpen(err) {
		t.Fatalf("503 应兜底为 circuit_open: %v", err)
	}
	// 429 → busy（排队——环境故障）
	err2 := classifyGatewayError(http.StatusTooManyRequests, "busy")
	if !isEnvFault(err2) {
		t.Fatalf("429 应算环境故障: %v", err2)
	}
	// 404 → model_not_found（不可重试）
	err3 := classifyGatewayError(http.StatusNotFound, "not found")
	if isEnvFault(err3) {
		t.Fatalf("404 不应算环境故障: %v", err3)
	}
	// 500 → upstream_fail（可重试）
	err4 := classifyGatewayError(http.StatusInternalServerError, "server error")
	if !isRetryable(err4) {
		t.Fatalf("500 应兜底为 upstream_fail（可重试）: %v", err4)
	}
}

// TestClassifyGatewayError_LegacyCode 旧 code 归一化（not_found_error/api_error）
func TestClassifyGatewayError_LegacyCode(t *testing.T) {
	// 旧 code: not_found_error → model_not_found（不可重试）
	body := `{"error":{"message":"路由选择失败: 模型 xx 未在路由表中找到","type":"not_found_error"}}`
	err := classifyGatewayError(404, body)
	if isEnvFault(err) {
		t.Fatalf("旧 not_found_error 应归一化为 model_not_found（不可重试）: %v", err)
	}
	// 旧 code: api_error → upstream_fail（可重试）
	body2 := `{"error":{"message":"转发失败","type":"api_error"}}`
	err2 := classifyGatewayError(502, body2)
	if !isRetryable(err2) {
		t.Fatalf("旧 api_error 应归一化为 upstream_fail（可重试）: %v", err2)
	}
}

// TestClassifyGatewayError_Retryable 可重试判定（只认 upstream_fail/busy）
func TestClassifyGatewayError_Retryable(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{fmt.Errorf("[circuit_open] 熔断"), false},
		{fmt.Errorf("[upstream_fail] 转发失败"), true},
		{fmt.Errorf("[busy] 排队"), true},
		{fmt.Errorf("[bad_request] 参数错"), false},
		{fmt.Errorf("[model_not_found] 模型不存在"), false},
		{fmt.Errorf("普通错误"), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := isRetryable(c.err); got != c.want {
			t.Fatalf("isRetryable(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

// TestIsEnvFault 环境故障判定（可等/可重试——不消耗任务重试次数）
func TestIsEnvFault(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{fmt.Errorf("[circuit_open] 熔断"), true},
		{fmt.Errorf("[upstream_fail] 转发失败"), true},
		{fmt.Errorf("[busy] 排队"), true},
		{fmt.Errorf("[bad_request] 参数错"), false},
		{fmt.Errorf("[model_not_found] 模型不存在"), false},
		{fmt.Errorf("REBUILD: 循环超限"), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := isEnvFault(c.err); got != c.want {
			t.Fatalf("isEnvFault(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

// TestPingModel 探活请求构造（模型名为空=放行；有模型=发请求——本地网关可达时 ok）
func TestPingModel(t *testing.T) {
	// 待修补 #35: 切临时 tasksFile——构造期不读真机 /tmp/zerg-tasks.json
	isolateTasksFile(t)
	s := NewMasterScheduler("", 1)
	// 无模型——放行
	ok, err := s.pingModel("")
	if !ok || err != nil {
		t.Fatalf("空模型应放行: ok=%v err=%v", ok, err)
	}
	// 有模型——发探活（网关可能不可达——但必须返回分类错误而非 panic）
	ok2, err2 := s.pingModel("Qwen3.8-27B")
	_ = ok2
	if err2 != nil && !strings.Contains(err2.Error(), "[") {
		t.Fatalf("探活错误应带分类前缀: %v", err2)
	}
	// 缓存生效（第二次不重复请求——直接命中缓存）
	s.pingModel("Qwen3.8-27B")
	if _, ok := s.pingCache["Qwen3.8-27B"]; !ok {
		t.Fatalf("探活结果应缓存")
	}
}

// ========== v2.5.6 ping 三级漏斗测试（Mr2109 2026-08-28 效率优化） ==========

// mockStoreReader 模拟快照读取（测试第1级快照优先）
type mockStoreReader struct {
	snapshots map[string]*FleetSnapshotLite
}

func (m *mockStoreReader) MachineSnapshot(machine string) *FleetSnapshotLite {
	if m == nil {
		return nil
	}
	return m.snapshots[machine]
}

// TestPingModel_SnapshotFastPath 第1级: 快照 healthy + 已加载模型 → 0ms 通过（不发请求）
func TestPingModel_SnapshotFastPath(t *testing.T) {
	// 待修补 #35: 切临时 tasksFile——构造期不读真机 /tmp/zerg-tasks.json
	isolateTasksFile(t)
	mock := &mockStoreReader{snapshots: map[string]*FleetSnapshotLite{
		"local": {Healthy: true, Model: "Qwen3.8-27B-Q4_K_M-vcruz305"},
	}}
	s := NewMasterScheduler("", 1, mock)
	// local 已加载 Qwen3.8——直接通过（不发网络请求——快）
	ok, err := s.pingModel("Qwen3.8-27B")
	if !ok || err != nil {
		t.Fatalf("快照优先应通过（0ms）: ok=%v err=%v", ok, err)
	}
}

// TestPingModel_SnapshotUnhealthy 快照 unhealthy → 不走快照——发请求探测（走到网络层）
func TestPingModel_SnapshotUnhealthy(t *testing.T) {
	// 待修补 #35: 切临时 tasksFile——构造期不读真机 /tmp/zerg-tasks.json
	isolateTasksFile(t)
	mock := &mockStoreReader{snapshots: map[string]*FleetSnapshotLite{
		"local": {Healthy: false, Model: "Qwen3.8-27B"},
		"x3":    {Healthy: true, Model: "example-35b"},
	}}
	s := NewMasterScheduler("", 1, mock)
	// local unhealthy 且 x3 未加载目标——快照不通过——走网络探测（网关可能不可达——分类错误）
	ok, err := s.pingModel("Qwen3.8-27B")
	_ = ok
	if err != nil && !strings.Contains(err.Error(), "[") {
		t.Fatalf("网络探测错误应带分类前缀: %v", err)
	}
}

// TestPingModel_SnapshotNilStore 无 store → 跳过快照——直接网络探测（不 panic）
func TestPingModel_SnapshotNilStore(t *testing.T) {
	// 待修补 #35: 切临时 tasksFile——构造期不读真机 /tmp/zerg-tasks.json
	isolateTasksFile(t)
	s := NewMasterScheduler("", 1) // 不注入 store
	ok, err := s.pingModel("Qwen3.8-27B")
	_ = ok
	if err != nil && !strings.Contains(err.Error(), "[") {
		t.Fatalf("无 store 应正常走网络探测: %v", err)
	}
}

// TestModelFileLoadedLite 快照精简版匹配（文件名形式/逻辑名/负例）
func TestModelFileLoadedLite(t *testing.T) {
	// 文件名形式（Qwen3.8-27B-Q4_K_M-vcruz305 → 含 Qwen3.8-27B）
	lite := &FleetSnapshotLite{Model: "Qwen3.8-27B-Q4_K_M-vcruz305"}
	if !modelFileLoadedLite(lite, "Qwen3.8-27B") {
		t.Fatalf("文件名形式应匹配逻辑名")
	}
	// 带 .gguf + 路径
	lite2 := &FleetSnapshotLite{Model: "/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf"}
	if !modelFileLoadedLite(lite2, "Qwen3.8-27B") {
		t.Fatalf("带路径带 .gguf 应匹配")
	}
	// 逻辑名精确
	lite3 := &FleetSnapshotLite{Model: "Qwen3.8-27B"}
	if !modelFileLoadedLite(lite3, "Qwen3.8-27B") {
		t.Fatalf("逻辑名应匹配")
	}
	// 负例（不同模型）
	lite4 := &FleetSnapshotLite{Model: "example-35b"}
	if modelFileLoadedLite(lite4, "Qwen3.8-27B") {
		t.Fatalf("不同模型不应匹配")
	}
	// 空快照
	if modelFileLoadedLite(nil, "Qwen3.8-27B") {
		t.Fatalf("nil 快照不应匹配")
	}
}

// TestFinishTaskReviewFlow zerg 流程完成 → 三层复查（2026-08-29 Mr2109）
// 合格报告 → reviewing + 派复查任务；空壳报告 → 确定性验证不过 → failed
func TestFinishTaskReviewFlow(t *testing.T) {
	// 待修补 #35: 切临时 tasksFile——构造期不读真机 /tmp/zerg-tasks.json，落盘不写真机文件
	isolateTasksFile(t)
	s := NewMasterScheduler("", 1)
	// 任务目录（临时——避免真实 /tmp/zerg-tasks 污染）
	task := &Task{
		ID:          "task-review-test-1",
		Description: "测试任务",
		Type:        "external",
		Model:       "Qwen3.8-27B",
		Workdir:     "/tmp/zerg-test-flow",
		Status:      "running",
		Flow:        "zerg",
	}
	taskDir := taskDirForTask(task)
	_ = os.RemoveAll(taskDir)
	_ = os.MkdirAll(taskDir, 0o755)
	// 模拟 git 仓库（VerifyTaskOutput 查 git 改动——zerg 流程目录有 .git 才算）——没有 .git 时 gitHasChanges 行为？
	// 先测空壳报告: 报告存在但 <100 字节 → 确定性验证不过 → failed
	_ = os.MkdirAll(taskDir, 0o755)
	_ = os.WriteFile(filepath.Join(taskDir, "internal-task-report.md"), []byte("# 空壳报告"), 0o644)
	s.running[task.ID] = task
	s.finishTask(task, nil)
	if task.Status != "failed" {
		t.Fatalf("空壳报告应 failed（确定性验证拦截）——实际 %s: %s", task.Status, task.FailReason)
	}
	if !strings.Contains(task.FailReason, "确定性验证不过") {
		t.Fatalf("失败原因应为确定性验证: %s", task.FailReason)
	}
	_ = os.RemoveAll(taskDir)
}
