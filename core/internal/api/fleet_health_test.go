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
		"node-local": {Machine: "node-local", Healthy: true, CodeVersion: "2.5.9", CodeSHA: "fefca220+dirty"},
		"node-a":     {Machine: "node-a", Healthy: true, CodeVersion: "2.5.10", CodeSHA: "213a67c4"},
	}
	fh = CountFleetHealth(master, mixed)
	if fh.Unhealthy < 1 {
		t.Fatalf("混版必须判红：unhealthy_count=%d（真值应 ≥1）", fh.Unhealthy)
	}
	if !fh.Mixed() || len(fh.MixedMachines) != 1 || fh.MixedMachines[0] != "node-local" {
		t.Errorf("混版机器名单应为 [node-local]，得到 %v", fh.MixedMachines)
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
	postHeartbeat(t, h, store.HeartbeatRequest{Machine: "node-local", Healthy: true, CodeVersion: "2.5.9", CodeSHA: "fefca220+dirty"})
	postHeartbeat(t, h, store.HeartbeatRequest{Machine: "node-a", Healthy: true, CodeVersion: "2.5.10", CodeSHA: "213a67c4"})

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
	nodeLocal, _ := machines["node-local"].(map[string]interface{})
	if vm, _ := nodeLocal["version_mismatch"].(bool); !vm {
		t.Errorf("node-local（2.5.9）的 version_mismatch 应为 true，得到 %v", nodeLocal["version_mismatch"])
	}
	// 旧字段不删（旧消费者零改动）：`healthy` 仍是子端自报值
	if hv, _ := nodeLocal["healthy"].(bool); !hv {
		t.Errorf("旧字段 healthy 必须原样保留（子端自报 true），得到 %v", nodeLocal["healthy"])
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

// ── 「双源矩阵同源判定」三态（GAP-20261013-09 · 候选 c）──────────────────────────
//
// 病灶：主控那行报**私有提交 sha**、节点那行报**镜像 tip sha** —— 两套 git 命名空间的字符串，
// 矩阵里没有可比键 ⇒ 三台其实同源也看不出（「全绿也可能是混版」）。
// 治法：比**源提交**（`origin_sha`）并把判决印在三态格上；★ 缺映射/trailer 缺 ⇒ `unknown`（缺 ≠ 一致）。

// TestMachineOriginVerdict_ThreeStates —— 纯函数三态 + 缺项各格（含好件那一格，免得只留反例）。
func TestMachineOriginVerdict_ThreeStates(t *testing.T) {
	cases := []struct {
		name    string
		master  string
		machine string
		want    string
	}{
		{"好件：同一笔源提交 ⇒ same", "3b04f6ab", "3b04f6ab", OriginSame},
		{"反例：源提交不同 ⇒ mixed（真混版）", "3b04f6ab", "1a6b02fc", OriginMixed},
		// ✗ 下面四格就是「防假绿」那颗牙：判不了必须报 unknown，绝不折叠成 same。
		{"缺映射：该机没带 origin_sha ⇒ unknown（缺 ≠ 一致）", "3b04f6ab", "", OriginUnknown},
		{"缺映射：trailer 缺的旧哨兵字面量 ⇒ unknown", "3b04f6ab", "unknown", OriginUnknown},
		{"判不了：主控自己没带源提交 ⇒ unknown（没有基准就没有判决）", "", "3b04f6ab", OriginUnknown},
		{"判不了：两侧都没带 ⇒ unknown（✗ 绝不因两个空串相等而判 same）", "", "", OriginUnknown},
	}
	for _, c := range cases {
		if got := MachineOriginVerdict(c.master, c.machine); got != c.want {
			t.Errorf("%s：MachineOriginVerdict(%q,%q) = %q，want %q", c.name, c.master, c.machine, got, c.want)
		}
	}
}

// TestMachineOriginVerdictWithCode_FourthState —— 第四态「本机脏」三例（GAP-20261013-09 附④）：
// 干净且同源 ⇒ 仍 `same`（反例：脏标记只认 `+dirty` 尾标）；同源但 `code_sha` 带 `+dirty` ⇒
// `local_dirty`（✗ 不折叠成 `same`）；真缺 / 真混版 / 没基准 ⇒ **仍回三态**（第四态只换 `same` 那一格，
// 不许盖掉更强的结论）。★ 干净的 `same` 与脏的 `local_dirty` 并列 ⇒「可与 same 区分」可机检。
func TestMachineOriginVerdictWithCode_FourthState(t *testing.T) {
	cases := []struct {
		name    string
		master  string
		machine string
		codeSHA string
		want    string
	}{
		{"干净且同源 ⇒ 仍 same", "3b04f6ab", "3b04f6ab", "3b04f6ab", OriginSame},
		{"本机脏：同源但 code_sha 带 +dirty ⇒ local_dirty（✗ 不判 same）", "3b04f6ab", "3b04f6ab", "3b04f6ab+dirty", OriginLocalDirty},
		{"真缺：该机没带 origin_sha ⇒ 仍 unknown（第四态不盖 unknown）", "3b04f6ab", "", "3b04f6ab+dirty", OriginUnknown},
		{"真混版：源提交不同 ⇒ 仍 mixed（第四态不盖 mixed）", "3b04f6ab", "1a6b02fc", "3b04f6ab+dirty", OriginMixed},
		{"判不了：主控没带源提交 ⇒ 仍 unknown（没有基准就没有判决）", "", "3b04f6ab", "3b04f6ab+dirty", OriginUnknown},
		{"干净但真混版 ⇒ mixed 不被脏标记干扰", "3b04f6ab", "1a6b02fc", "1a6b02fc", OriginMixed},
	}
	for _, c := range cases {
		if got := MachineOriginVerdictWithCode(c.master, c.machine, c.codeSHA); got != c.want {
			t.Errorf("%s：MachineOriginVerdictWithCode(%q,%q,%q) = %q，want %q",
				c.name, c.master, c.machine, c.codeSHA, got, c.want)
		}
	}
}

// TestStatusHandler_SameOriginThreeStates —— HTTP 层同形状复跑：三台一次打出三态 ＋ 第四态「本机脏」+ 基准随响应。
//
// 三台 `code_version` 全都相同（= 现有版本裁决一律判「不混版」），差别只在**源提交**与**本机脏** ——
// 这正是本缺口的形态：旧字段全绿，而『到底同不同源』这一句没人答。本用例要它答出来。
// ★ 附④ 后 Mr2109（主控本机 · `code_sha` 带 `+dirty`）判 `local_dirty` —— ✗ 不再折叠成 `same`
// （用例名里的「三态」指既有三态；本行的第四态由 `OriginLocalDirty` 承接，改名留收口笔 · 本节不删符号）。
func TestStatusHandler_SameOriginThreeStates(t *testing.T) {
	// 主控基准的真源是 `version.Origin`（构建期注入）；未注入时是空串 ⇒ 全表 unknown。
	// 本用例要证的是**判决**与**下发**，故把基准钉住（用完即还原，不留包级残留）。
	old := version.Origin
	version.Origin = "3b04f6abcd"
	t.Cleanup(func() { version.Origin = old })

	h := newTestHandlers()
	// ① 同源但本机脏：源提交与主控基准逐字相同，而 code_sha 带 +dirty（附④ ⇒ 第四态 local_dirty）
	postHeartbeat(t, h, store.HeartbeatRequest{Machine: "Mr2109", Healthy: true, CodeVersion: version.Version,
		CodeSHA: "3b04f6abcd+dirty", OriginSHA: "3b04f6abcd"})
	// ② 真混版：拿的是另一笔源提交
	postHeartbeat(t, h, store.HeartbeatRequest{Machine: "edge-node", Healthy: true, CodeVersion: version.Version,
		CodeSHA: "ca565e92706196efb9d61910aa3fdfbdf9c8a775", OriginSHA: "1a6b02fc"})
	// ③ 判不了：该机没带源提交（trailer 缺 / 未注入）—— 版本号相同也✗ 不许判 same
	postHeartbeat(t, h, store.HeartbeatRequest{Machine: "mini-node-2", Healthy: true, CodeVersion: version.Version,
		CodeSHA: "ca565e92706196efb9d61910aa3fdfbdf9c8a775"})

	resp := statusOf(t, h)
	if got := resp["master_origin_sha"]; got != "3b04f6abcd" {
		t.Fatalf("master_origin_sha 应随响应报出判决基准（否则「同源」不可复核），实得 %v", got)
	}
	machines, _ := resp["machines"].(map[string]interface{})
	// Mr2109 = 主控本机（源提交同、但 code_sha 带 +dirty）⇒ 第四态 local_dirty（附④ · ✗ 不判 same）；
	// edge-node = 真混版（另一笔源提交）；mini-node-2 = 判不了（没带源提交）。
	want := map[string]string{"Mr2109": OriginLocalDirty, "edge-node": OriginMixed, "mini-node-2": OriginUnknown}
	for m, w := range want {
		row, _ := machines[m].(map[string]interface{})
		if row == nil {
			t.Fatalf("machines 里没有 %s", m)
		}
		if got, _ := row["same_origin"].(string); got != w {
			t.Errorf("%s 的 same_origin = %q，want %q（三台 code_version 全相同，差别只在源提交）", m, got, w)
		}
	}
	// 旧字段不删（旧消费者零改动）：三台同版本号 ⇒ 版本裁决照旧「不混版」
	if mv, _ := resp["mixed_version"].(bool); mv {
		t.Errorf("三台 code_version 全同 ⇒ mixed_version 必须仍为 false，实得 %v", resp["mixed_machines"])
	}
	if hc, _ := resp["healthy_count"].(float64); hc != 3 {
		t.Errorf("healthy_count 应仍为 3（同源判定不得动健康计数），实得 %v", hc)
	}
}
