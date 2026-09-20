package api

// model_id_map_test.go — T-41（§二十一 第 19 条 · §十二 `R2-P1` · §20.7 `OM9` · §20.3 `H2`）
// 能力快照 id ↔ 路由表 id 的**规范化真源**判据机检。
//
// 判据（开工单 T-41 逐字）：
//   ① `/api/models/registry` 与 `/api/fleet/models` 的 id **逐条对得上**；
//   ② 交集**非空**；
//   ③ 「命令有什么能力」这个自描述面**接得到**路由表。
//
// 本件的口径：真源 = `contract.ModelIDMap()`（`core/internal/contract/model-id-map.json`，
// **显式条目**）；两个只读面各带一个**精确**映射键（快照记录带 `fleet_id`、路由表行带
// `registry_id`）⇒ 两侧可在**机器面**上直接连起来（不再靠人眼对名字）。
//
// 反例优先（负控，每条都真的红）：
//   - 近名不作数：`example-35b-v2-1-5-35b-q4-k-m-extra`（多一段）⇒ 不映射（不模糊匹配兜）；
//   - 大小写不作数：`example-35b-v2` ⇒ 不映射；
//   - 未知 id（真仓里的 `test-model`）⇒ 整键**不出现**（不是空串，是键缺席）。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/contract"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// TestModelIDMap_TruthSourceShape —— 真源自身成形（两侧齐 + 反向一致 + schema 在位）。
func TestModelIDMap_TruthSourceShape(t *testing.T) {
	m, err := contract.ModelIDMap()
	if err != nil {
		t.Fatalf("id 规范化真源解不动: %v", err)
	}
	if m.Schema == "" {
		t.Error("真源缺 schema")
	}
	if len(m.Entries) == 0 {
		t.Fatal("真源 entries 为空 —— 空表 = 判据没有对象")
	}
	for _, e := range m.Entries {
		if e.RegistryID == "" || len(e.FleetIDs) == 0 {
			t.Errorf("条目不成形（两侧必须齐）: %+v", e)
		}
		if e.Evidence == "" {
			t.Errorf("条目缺证据（%s）—— 不许有不知从哪来的映射", e.RegistryID)
		}
		// 正向：registry_id ⇒ fleet_ids
		got, ok := m.FleetIDForRegistry(e.RegistryID)
		if !ok || len(got) != len(e.FleetIDs) {
			t.Errorf("正向查表不一致: %s ⇒ %v (ok=%v)", e.RegistryID, got, ok)
		}
		// 反向：每个 fleet_id ⇒ 同一个 registry_id
		for _, f := range e.FleetIDs {
			rid, ok := m.RegistryIDForFleet(f)
			if !ok || rid != e.RegistryID {
				t.Errorf("反向查表不一致: %s ⇒ %q (ok=%v)，应为 %q", f, rid, ok, e.RegistryID)
			}
		}
	}
}

// TestModelIDMap_NoFuzzyFallback —— 负控：近名 / 大小写 / 多一段都**不作数**。
func TestModelIDMap_NoFuzzyFallback(t *testing.T) {
	m, err := contract.ModelIDMap()
	if err != nil {
		t.Fatalf("id 规范化真源解不动: %v", err)
	}
	unknownRegistry := []string{
		"example-35b-v2-1-5-35b-q4-k-m-extra", // 多一段
		"example-35b-v2-1-5-35b",              // 少一段（路由表的短名当快照名来查）
		"ORNITH-1-5-35B-Q4-K-M",       // 大写
		"",                            // 空
	}
	for _, id := range unknownRegistry {
		if got, ok := m.FleetIDForRegistry(id); ok {
			t.Errorf("负控失败：%q 竟然映射到 %v（不许模糊匹配兜）", id, got)
		}
	}
	unknownFleet := []string{
		"example-35b-v2",    // 大小写
		"example-35b-v2.md", // 后缀
		"example-35b-v2X",   // 多一字
		"",
	}
	for _, id := range unknownFleet {
		if got, ok := m.RegistryIDForFleet(id); ok {
			t.Errorf("负控失败：%q 竟然反向映射到 %q（不许模糊匹配兜）", id, got)
		}
	}
}

// TestModelRegistryHandler_FleetIDJoin —— 判据①/②：快照记录带上 `fleet_id`，
// 且它与路由表 id 的**交集非空**。
func TestModelRegistryHandler_FleetIDJoin(t *testing.T) {
	root := t.TempDir()
	known := []string{"gemma-4-26b-a4b-it-ud-q4-k-m", "example-35b-v2-1-5-35b-q4-k-m"}
	unknown := "no-such-manifest-id"
	for i, id := range append(append([]string{}, known...), unknown) {
		putRecord(t, root, validRegistryRecord(id, "sha256:"+filepath.Base(root)+"-"+string(rune('a'+i)), "yes", 4096))
	}
	h := &Handlers{ModelsDir: root}
	resp := getRegistry(t, h)

	got := map[string]string{}
	for _, r := range resp.Records {
		got[r.ID] = r.FleetID
	}
	for _, id := range known {
		if got[id] == "" {
			t.Errorf("已知 id %q 应带 fleet_id，实际为空", id)
		}
	}
	if v, ok := got[unknown]; !ok {
		t.Fatalf("未知 id 的记录应在结果里（只是没有 fleet_id），实际没有")
	} else if v != "" {
		t.Errorf("未知 id %q 不该有 fleet_id，实际 %q", unknown, v)
	}

	// 判据②：快照侧的 fleet_id 集合与真源路由表 id 集合**交集非空**。
	m, err := contract.ModelIDMap()
	if err != nil {
		t.Fatalf("真源解不动: %v", err)
	}
	fleetSet := map[string]bool{}
	for _, e := range m.Entries {
		for _, f := range e.FleetIDs {
			fleetSet[f] = true
		}
	}
	inter := 0
	for _, r := range resp.Records {
		if r.FleetID == "" {
			continue
		}
		for _, f := range splitComma(r.FleetID) {
			if fleetSet[f] {
				inter++
			}
		}
	}
	if inter == 0 {
		t.Error("判据②失败：快照侧映射出的路由表 id 与真源交集为空")
	}
}

// TestModelsHandler_RegistryIDJoin —— 判据①/③：路由表行带 `registry_id`，
// 未知行**整键不出现**（负控）。
func TestModelsHandler_RegistryIDJoin(t *testing.T) {
	h := &Handlers{
		Config: &config.FleetConfig{
			Models: map[string][]config.ModelCandidate{
				"example-35b-v2": {{Host: "Mr2109", Backend: "llama-server"}},
				"gemma-4-26B":    {{Host: "x3", Backend: "llama-server"}},
				"test-model":     {{Host: "127.0.0.1", Backend: "llama-server"}},
			},
			Fleet: map[string]config.FleetNode{},
		},
		Store: store.NewStore(),
	}
	w := httptest.NewRecorder()
	h.ModelsHandler(w, httptest.NewRequest(http.MethodGet, "/api/fleet/models", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200——实际 %d", w.Code)
	}
	var resp struct {
		Models []map[string]interface{} `json:"models"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应解析失败: %v", err)
	}
	seen := map[string]string{}
	for _, r := range resp.Models {
		id, _ := r["id"].(string)
		rid, ok := r["registry_id"].(string)
		if !ok {
			seen[id] = "<absent>"
			continue
		}
		seen[id] = rid
	}
	if seen["example-35b-v2"] != "example-35b-v2-1-5-35b-q4-k-m" {
		t.Errorf("example-35b-v2 的 registry_id 错: %q", seen["example-35b-v2"])
	}
	if seen["gemma-4-26B"] != "gemma-4-26b-a4b-it-ud-q4-k-m" {
		t.Errorf("gemma-4-26B 的 registry_id 错: %q", seen["gemma-4-26B"])
	}
	if seen["test-model"] != "<absent>" {
		t.Errorf("未知模型 test-model 不该有 registry_id（要「键缺席」而不是空串）: %q", seen["test-model"])
	}
}

// splitComma 拆真源里用逗号连接的多候选（与 handler 的 Join 口径一致）。
func splitComma(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
