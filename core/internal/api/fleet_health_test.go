// fleet_health_test.go — §二十一 已红第 1 条（T-23）的**成对负控**：
// 「混版机群被判绿」这一条要在**两个层面**都钉住：
//
//	① 计数函数层（纯函数）：好件绿 / 混版红 / 未报不判 / 自报不健康红 —— 四格逐条断言；
//	② HTTP 处理器层（真路由 + 真包封）：两枚真心跳（不同 code_version）打进真 Store，
//	   再跑真 `StatusHandler`，断言 `unhealthy_count ≥ 1` —— 这条才是开工单判据的
//	   **同形状**复跑（判据要的 `curl … /api/fleet/status` 就是它；生产面那一次要重启主控 ⇒ 待拍）。
//
// 为什么必须有「好件」那一格：只有反例的测试会在任何实现下都红 —— 判不出「改坏了」与
// 「本来就坏」的差别（同门⑤/门⑥/门⑦ 的成对负控口径）。
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/Mr2109/zerg-swarm/core/internal/version"
)

func TestMachineVersionVerdict_PairedControls(t *testing.T) {
	cases := []struct {
		name    string
		master  string
		machine string
		want    string
	}{
		{"好件：同版本号 ⇒ same", "2.5.10", "2.5.10", VersionSame},
		{"反例：混版（旧一档）⇒ mixed", "2.5.10", "2.5.9", VersionMixed},
		{"反例：混版（新一档）⇒ mixed", "2.5.10", "2.5.11", VersionMixed},
		{"缺报：空串 ⇒ unknown（没报 ≠ 一致）", "2.5.10", "", VersionUnknown},
	}
	for _, c := range cases {
		if got := MachineVersionVerdict(c.master, c.machine); got != c.want {
			t.Errorf("%s：MachineVersionVerdict(%q,%q) = %q，want %q", c.name, c.master, c.machine, got, c.want)
		}
	}
}

func TestCountFleetHealth_PairedControls(t *testing.T) {
	master := "2.5.10"
	// ① 好件：两台同版 + 自报健康 ⇒ 全绿、无混版
	good := map[string]*store.FleetSnapshot{
		"a": {Machine: "a", Healthy: true, CodeVersion: master},
		"b": {Machine: "b", Healthy: true, CodeVersion: master},
	}
	fh := CountFleetHealth(master, good)
	if fh.Healthy != 2 || fh.Unhealthy != 0 || fh.Mixed() {
		t.Errorf("好件应全绿：healthy=%d unhealthy=%d mixed=%v", fh.Healthy, fh.Unhealthy, fh.Mixed())
	}

	// ② 反例（本条病灶）：2.5.9 / 2.5.10 并存、两台都自报 healthy=true ⇒ 必须至少一台不健康
	mixed := map[string]*store.FleetSnapshot{
		"Mr2109": {Machine: "Mr2109", Healthy: true, CodeVersion: "2.5.9", CodeSHA: "fefca220+dirty"},
		"x3":   {Machine: "x3", Healthy: true, CodeVersion: "2.5.10", CodeSHA: "213a67c4"},
	}
	fh = CountFleetHealth(master, mixed)
	if fh.Unhealthy < 1 {
		t.Fatalf("混版必须判红：unhealthy_count=%d（真值应 ≥1）", fh.Unhealthy)
	}
	if !fh.Mixed() || len(fh.MixedMachines) != 1 || fh.MixedMachines[0] != "Mr2109" {
		t.Errorf("混版机器名单应为 [Mr2109]，得到 %v", fh.MixedMachines)
	}
	if fh.Healthy != 1 {
		t.Errorf("同版那台仍应计健康：healthy=%d", fh.Healthy)
	}

	// ③ 缺报：没带 code_version ⇒ 不判（既不 healthy 也不 unhealthy），单列 unknown
	unknown := map[string]*store.FleetSnapshot{
		"c": {Machine: "c", Healthy: true},
	}
	fh = CountFleetHealth(master, unknown)
	if fh.VersionUnknown != 1 || fh.Mixed() || fh.Unhealthy != 0 {
		t.Errorf("缺报应记 unknown 且不判混版：unknown=%d mixed=%v unhealthy=%d",
			fh.VersionUnknown, fh.Mixed(), fh.Unhealthy)
	}

	// ④ 自报不健康 + 同版 ⇒ 仍计不健康（旧语义一格不动）
	sick := map[string]*store.FleetSnapshot{
		"d": {Machine: "d", Healthy: false, CodeVersion: master},
	}
	fh = CountFleetHealth(master, sick)
	if fh.Unhealthy != 1 || fh.Mixed() {
		t.Errorf("同版但自报不健康 ⇒ unhealthy=1 且不混版，得到 unhealthy=%d mixed=%v", fh.Unhealthy, fh.Mixed())
	}
}

// postHeartbeat 打一枚真心跳进真 Store（与生产同一条 HTTP 处理器）。
func postHeartbeat(t *testing.T, h *Handlers, body store.HeartbeatRequest) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal heartbeat: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/fleet/heartbeat", bytes.NewReader(raw))
	w := httptest.NewRecorder()
	h.HeartbeatHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("heartbeat %s 应 200，得到 %d（body=%s）", body.Machine, w.Code, w.Body.String())
	}
}

// statusOf 跑真 StatusHandler 并把响应解成 map。
func statusOf(t *testing.T, h *Handlers) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/fleet/status", nil)
	w := httptest.NewRecorder()
	h.StatusHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("fleet/status 应 200，得到 %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

// TestStatusHandler_MixedFleetJudgedUnhealthy —— 开工单 T-23 判据的同形状复跑（HTTP 层）。
func TestStatusHandler_MixedFleetJudgedUnhealthy(t *testing.T) {
	h := newTestHandlers()
	postHeartbeat(t, h, store.HeartbeatRequest{Machine: "Mr2109", Healthy: true, CodeVersion: "2.5.9", CodeSHA: "fefca220+dirty"})
	postHeartbeat(t, h, store.HeartbeatRequest{Machine: "x3", Healthy: true, CodeVersion: "2.5.10", CodeSHA: "213a67c4"})

	resp := statusOf(t, h)
	unhealthy, _ := resp["unhealthy_count"].(float64)
	if unhealthy < 1 {
		t.Fatalf("混版机群必须判红：unhealthy_count=%v（want ≥1）", resp["unhealthy_count"])
	}
	if mv, _ := resp["mixed_version"].(bool); !mv {
		t.Errorf("mixed_version 应为 true，得到 %v", resp["mixed_version"])
	}
	if mcv, _ := resp["master_code_version"].(string); mcv != version.Version {
		t.Errorf("master_code_version 应为主控自己那一版 %q，得到 %v", version.Version, resp["master_code_version"])
	}
	machines, _ := resp["machines"].(map[string]interface{})
	Mr2109, _ := machines["Mr2109"].(map[string]interface{})
	if vm, _ := Mr2109["version_mismatch"].(bool); !vm {
		t.Errorf("Mr2109（2.5.9）的 version_mismatch 应为 true，得到 %v", Mr2109["version_mismatch"])
	}
	// 旧字段不删（旧消费者零改动）：`healthy` 仍是子端自报值
	if hv, _ := Mr2109["healthy"].(bool); !hv {
		t.Errorf("旧字段 healthy 必须原样保留（子端自报 true），得到 %v", Mr2109["healthy"])
	}
}

// TestStatusHandler_SameVersionNotMixed —— 成对的另一格：全员同版时**不许**被判混版。
func TestStatusHandler_SameVersionNotMixed(t *testing.T) {
	h := newTestHandlers()
	postHeartbeat(t, h, store.HeartbeatRequest{Machine: "m1", Healthy: true, CodeVersion: version.Version})
	postHeartbeat(t, h, store.HeartbeatRequest{Machine: "m2", Healthy: true, CodeVersion: version.Version})

	resp := statusOf(t, h)
	if mv, _ := resp["mixed_version"].(bool); mv {
		t.Errorf("全员同版不许判混版：mixed_machines=%v", resp["mixed_machines"])
	}
	if hc, _ := resp["healthy_count"].(float64); hc != 2 {
		t.Errorf("全员同版且健康 ⇒ healthy_count=2，得到 %v", resp["healthy_count"])
	}
	if uc, _ := resp["unhealthy_count"].(float64); uc != 0 {
		t.Errorf("全员同版且健康 ⇒ unhealthy_count=0，得到 %v", resp["unhealthy_count"])
	}
}
