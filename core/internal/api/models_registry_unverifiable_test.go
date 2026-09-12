package api

// models_registry_unverifiable_test.go — GET /api/models/registry 的**不可判定能力**专项测试
// （待修补 #28）。
//
// 背景：待修补 #27 给能力快照（<version>.capabilities.json，schema v1）加了可选字段
// `unverifiable[]`——形状 {"name","reason","evidence"}，语义是"本轮**没探出结论**"
// （预算耗尽 / 超时），与 capabilities[].value=false（端点明确表态"没有"）严格区分：
// 缺 = 未知，绝不 = 没有。本接口原先完全不带出这个字段，UI 因而看不到"为什么某项能力
// 既不是有也不是没有"。本文件逐条钉死修好后的行为（反例优先）。
//
// 取舍（已用断言钉住）：unverifiable 与三个 snapshot_* 出处字段**解耦**——出处字段沿用既有
// 出现条件（快照读到了且 len(capabilities) > 0），本次改动**不**改变它。故"只有 unverifiable、
// 没有 capabilities"时：unverifiable 照常出现，而 snapshot_endpoint/generated_at/online_probed
// 仍整键不出现（那三个字段标注的是"上面这些能力在哪探出来的"，纯不可判定场景没有实测能力
// 可溯源）。见 TestModelRegistryAPI_UnverifiableOnlyNoCapabilities。
//
// 既有 4 个能力/许可用例一字不动（不得改弱），这里只做加法。写盘一律 t.TempDir()；
// **绝不碰 ~/.zerg**。

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
)

// 以下两条证据串照抄 modelreg.unverifiableOf 的真实产出格式
// （probe.<能力>.v1 budget=<预算> (<失败分类>: <原始响应摘要>)）。
const (
	uvEvidenceEmbedding = "probe.embedding.v1 budget=2048 (budget_exhausted: response truncated at max_tokens)"
	uvEvidenceTools     = "probe.tools.v1 budget=1024 (timeout: deadline exceeded after 5s)"
)

// registryUnverifiableByName 从响应里按 name 取一条不可判定记录（找不到返回 nil）。
func registryUnverifiableByName(uv []ModelRegistryUnverifiable, name string) *ModelRegistryUnverifiable {
	for i := range uv {
		if uv[i].Name == name {
			return &uv[i]
		}
	}
	return nil
}

// TestModelRegistryAPI_UnverifiableFromSnapshot 正例：
// 记录正文无能力 + 快照同时有 capabilities 与 unverifiable →
// 两者都带出；unverifiable 的 name/reason/evidence 逐字照抄、顺序保持；
// unverifiable 的名字**绝不**混进 capabilities（false≠未知）；出处三字段仍按既有规则出现。
func TestModelRegistryAPI_UnverifiableFromSnapshot(t *testing.T) {
	root := t.TempDir()
	// 记录正文完全没有能力（与 probe 产出一致）
	rec := validRegistryRecord("uvboth", "sha256:"+strings.Repeat("f", 64), "unknown", 8192)
	recPath := putRecord(t, root, rec)

	const endpoint = "http://127.0.0.1:18081/v1"
	genAt := time.Date(2026, 9, 12, 4, 5, 6, 0, time.UTC)
	rep := &modelreg.ProbeReport{
		Endpoint:     endpoint,
		GeneratedAt:  genAt,
		OnlineProbed: true,
		Capabilities: []modelreg.Capability{
			{Name: "text", Value: true, Source: "probed", Evidence: modelreg.EvidenceText},
			// 确定不支持：value=false，**不**进 unverifiable（区分点）
			{Name: "vision", Value: false, Source: "probed", Evidence: modelreg.EvidenceVision + " (no_vision: http 400)"},
		},
		Unverifiable: []modelreg.Unverifiable{
			{Name: "embedding", Reason: "budget_exhausted", Evidence: uvEvidenceEmbedding},
			{Name: "tools", Reason: "timeout", Evidence: uvEvidenceTools},
		},
	}
	sibPath := writeSnapshotForRecord(t, recPath, rec, rep)

	before := snapshotTree(t, root)
	h := newTestHandlers()
	h.ModelsDir = root
	raw, resp := getRegistryRaw(t, h)

	if resp.Count != 1 || resp.BadRecords != 0 {
		t.Fatalf("预期 count=1 bad_records=0——实际 count=%d bad=%d", resp.Count, resp.BadRecords)
	}
	got := resp.Records[0]

	// —— capabilities 来自快照（正文一个都没有）——
	if len(got.Capabilities) != 2 {
		t.Fatalf("能力应来自快照（2 条）——实际 %s", mustJSON(t, got.Capabilities))
	}
	// 红线：不可判定的名字绝不能出现在 capabilities 里
	for _, c := range got.Capabilities {
		if c.Name == "embedding" || c.Name == "tools" {
			t.Errorf("不可判定能力 %q 不得混进 capabilities（false≠未知）——实际 %s",
				c.Name, mustJSON(t, got.Capabilities))
		}
	}

	// —— unverifiable 逐字照抄 + 顺序保持 ——
	wantUV := []ModelRegistryUnverifiable{
		{Name: "embedding", Reason: "budget_exhausted", Evidence: uvEvidenceEmbedding},
		{Name: "tools", Reason: "timeout", Evidence: uvEvidenceTools},
	}
	if !reflect.DeepEqual(got.Unverifiable, wantUV) {
		t.Fatalf("unverifiable 应与快照逐字一致（含顺序）\n got=%s\nwant=%s",
			mustJSON(t, got.Unverifiable), mustJSON(t, wantUV))
	}
	if uv := registryUnverifiableByName(got.Unverifiable, "embedding"); uv == nil || uv.Reason != "budget_exhausted" {
		t.Errorf("embedding 的 reason 应照抄 budget_exhausted——实际 %+v", uv)
	}
	if uv := registryUnverifiableByName(got.Unverifiable, "tools"); uv == nil || uv.Reason != "timeout" {
		t.Errorf("tools 的 reason 应照抄 timeout——实际 %+v", uv)
	}
	// 不可判定的名字必须也在 capabilities 之外（已在上面查过），且都带 evidence
	for _, uv := range got.Unverifiable {
		if uv.Reason == "" || uv.Evidence == "" {
			t.Errorf("不可判定记录 %q 缺 reason/evidence（必须可追责）——%+v", uv.Name, uv)
		}
	}

	// —— 出处三字段仍按既有规则出现（快照读到了且有实测能力）——
	if got.SnapshotEndpoint != endpoint {
		t.Errorf("snapshot_endpoint 应为 %q——实际 %q", endpoint, got.SnapshotEndpoint)
	}
	if got.SnapshotGeneratedAt != "2026-09-12T04:05:06Z" {
		t.Errorf("snapshot_generated_at 应为 2026-09-12T04:05:06Z——实际 %q", got.SnapshotGeneratedAt)
	}
	if got.SnapshotOnlineProbed == nil || !*got.SnapshotOnlineProbed {
		t.Errorf("snapshot_online_probed 应为 true——实际 %v", got.SnapshotOnlineProbed)
	}

	// —— 原始 JSON 层面确认逐字写出去（不是靠结构体字段骗过断言）——
	body := string(raw)
	wantJSON := `"unverifiable":[` +
		`{"name":"embedding","reason":"budget_exhausted","evidence":"` + uvEvidenceEmbedding + `"},` +
		`{"name":"tools","reason":"timeout","evidence":"` + uvEvidenceTools + `"}]`
	if !strings.Contains(body, wantJSON) {
		t.Errorf("响应里 unverifiable 未逐字照抄\n want → %s\n got  → %s", wantJSON, body)
	}
	for _, want := range []string{
		`"snapshot_endpoint":"` + endpoint + `"`,
		`"evidence":"` + modelreg.EvidenceText + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("响应里缺少真实快照值 %s——body=%s", want, body)
		}
	}

	// 只读：快照文件请求后仍在、临时根内容不变
	if _, err := os.Stat(sibPath); err != nil {
		t.Errorf("只读被破坏：请求后快照文件应仍存在——err=%v", err)
	}
	assertSameTree(t, root, before)
	t.Logf("unverifiable + capabilities → %s", mustJSON(t, got))
}

// TestModelRegistryAPI_UnverifiableOnlyNoCapabilities 反例优先：
// 快照**只有** unverifiable、没有 capabilities（预算耗尽把整轮探测拖垮）→
// unverifiable 照常带出；三个 snapshot_* 出处字段按本任务的取舍**仍整键不出现**
// （不改变它们的既有出现条件）。这条用断言把取舍钉死。
func TestModelRegistryAPI_UnverifiableOnlyNoCapabilities(t *testing.T) {
	root := t.TempDir()
	rec := validRegistryRecord("uvonly", "sha256:"+strings.Repeat("a", 64), "unknown", 4096)
	recPath := putRecord(t, root, rec)

	rep := &modelreg.ProbeReport{
		// 故意给足出处信息：即便快照带端点与在线探测，只要没有实测能力，出处就不该出现
		Endpoint:     "http://127.0.0.1:18082/v1",
		GeneratedAt:  time.Date(2026, 9, 12, 5, 6, 7, 0, time.UTC),
		OnlineProbed: true,
		Capabilities: nil, // 没有能力：全都不可判定
		Unverifiable: []modelreg.Unverifiable{
			{Name: "text", Reason: "budget_exhausted", Evidence: uvEvidenceEmbedding},
		},
	}
	writeSnapshotForRecord(t, recPath, rec, rep)

	before := snapshotTree(t, root)
	h := newTestHandlers()
	h.ModelsDir = root
	raw, resp := getRegistryRaw(t, h)

	if resp.Count != 1 || resp.BadRecords != 0 {
		t.Fatalf("预期 count=1 bad_records=0——实际 count=%d bad=%d", resp.Count, resp.BadRecords)
	}
	got := resp.Records[0]

	// unverifiable 照常出现（这是本任务的核心承诺）
	wantUV := []ModelRegistryUnverifiable{
		{Name: "text", Reason: "budget_exhausted", Evidence: uvEvidenceEmbedding},
	}
	if !reflect.DeepEqual(got.Unverifiable, wantUV) {
		t.Fatalf("只有 unverifiable 时也必须带出\n got=%s\nwant=%s",
			mustJSON(t, got.Unverifiable), mustJSON(t, wantUV))
	}
	// capabilities 仍为 []（没有实测能力）
	if len(got.Capabilities) != 0 {
		t.Errorf("快照无 capabilities 时应为空——实际 %s", mustJSON(t, got.Capabilities))
	}
	// 取舍红线：出处字段仍整键不出现（本次改动不改变它们的出现条件）
	if got.SnapshotEndpoint != "" || got.SnapshotGeneratedAt != "" || got.SnapshotOnlineProbed != nil {
		t.Errorf("无实测能力时出处字段必须全空（既有规则不变）——实际 %+v", got)
	}
	body := string(raw)
	if !strings.Contains(body, `"unverifiable":[{"name":"text","reason":"budget_exhausted","evidence":"`+uvEvidenceEmbedding+`"}]`) {
		t.Errorf("unverifiable 未逐字出现——body=%s", body)
	}
	for _, key := range []string{`"snapshot_endpoint"`, `"snapshot_generated_at"`, `"snapshot_online_probed"`} {
		if strings.Contains(body, key) {
			t.Errorf("按取舍，无实测能力时出处字段 %s 不应出现——body=%s", key, body)
		}
	}

	assertSameTree(t, root, before)
	t.Logf("unverifiable-only(no caps) → %s", mustJSON(t, got))
}

// TestModelRegistryAPI_UnverifiableAbsent 反例：
// 快照不存在 / 是坏 JSON / 有 capabilities 但无 unverifiable（或缺省、或显式空数组）→
// 响应里 **`unverifiable` 整键不出现**、不 500、不造值；坏快照与坏记录策略一致。
func TestModelRegistryAPI_UnverifiableAbsent(t *testing.T) {
	cases := []struct {
		name string
		// setup 在临时根里造好记录，返回 root；可选造快照
		setup func(t *testing.T) string
		// wantCaps 期望该条带出的能力数（顺带确认别的字段没受影响）
		wantCaps int
	}{
		{
			name: "反例-无快照文件：整键不出现",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				putRecord(t, root, validRegistryRecord("nosnap", "sha256:"+strings.Repeat("b", 64), "unknown", 4096, "text"))
				return root
			},
			wantCaps: 1, // 正文声明能力照常给出
		},
		{
			name: "反例-坏 JSON 快照：整键不出现、不 500",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				rec := validRegistryRecord("badsnap", "sha256:"+strings.Repeat("c", 64), "unknown", 4096)
				recPath := putRecord(t, root, rec)
				sib := modelreg.CapabilitySnapshotPath(recPath)
				if err := os.WriteFile(sib, []byte(`{ "unverifiable": [ {"name":"text"} ,`), 0o644); err != nil {
					t.Fatalf("造坏快照失败: %v", err)
				}
				return root
			},
			wantCaps: 0,
		},
		{
			name: "反例-合法快照但无 unverifiable 字段：整键不出现",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				rec := validRegistryRecord("nocvu", "sha256:"+strings.Repeat("d", 64), "unknown", 4096)
				recPath := putRecord(t, root, rec)
				// 真实写入器在 unverifiable 为空时会省略该字段
				rep := &modelreg.ProbeReport{
					Endpoint:     "http://127.0.0.1:18083/v1",
					GeneratedAt:  time.Date(2026, 9, 12, 6, 7, 8, 0, time.UTC),
					OnlineProbed: true,
					Capabilities: []modelreg.Capability{
						{Name: "text", Value: true, Source: "probed", Evidence: modelreg.EvidenceText},
					},
				}
				writeSnapshotForRecord(t, recPath, rec, rep)
				return root
			},
			wantCaps: 1,
		},
		{
			name: "反例-快照显式空数组 unverifiable:[]：整键不出现（不是空数组）",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				rec := validRegistryRecord("emptyarr", "sha256:"+strings.Repeat("e", 64), "unknown", 4096)
				recPath := putRecord(t, root, rec)
				sib := modelreg.CapabilitySnapshotPath(recPath)
				// 手写一份显式空数组的快照（真实写入器不会产出这种，但旧数据/外部工具可能）
				raw := `{"schema":"zerg.model.capability_snapshot.v1","id":"emptyarr",` +
					`"endpoint":"http://127.0.0.1:18084/v1","online_probed":true,` +
					`"capabilities":[{"name":"text","value":true,"source":"probed","evidence":"probe.text.v1 ok"}],` +
					`"unverifiable":[]}`
				if err := os.WriteFile(sib, []byte(raw), 0o644); err != nil {
					t.Fatalf("造空数组快照失败: %v", err)
				}
				return root
			},
			wantCaps: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.setup(t)
			before := snapshotTree(t, root)
			h := newTestHandlers()
			h.ModelsDir = root
			// getRegistryRaw 内部要求 200：这里顺带证明"坏快照/缺快照不 500"
			raw, resp := getRegistryRaw(t, h)

			if resp.Count != 1 || resp.BadRecords != 0 {
				t.Fatalf("预期 count=1 bad_records=0——实际 count=%d bad=%d", resp.Count, resp.BadRecords)
			}
			got := resp.Records[0]
			if len(got.Unverifiable) != 0 {
				t.Errorf("unverifiable 应为空——实际 %s", mustJSON(t, got.Unverifiable))
			}
			if len(got.Capabilities) != tc.wantCaps {
				t.Errorf("capabilities 应为 %d 条——实际 %s", tc.wantCaps, mustJSON(t, got.Capabilities))
			}
			// 整键不出现（原文级）——绝不能是 "unverifiable":null / []
			if strings.Contains(string(raw), `"unverifiable"`) {
				t.Errorf("unverifiable 应整键不出现——body=%s", raw)
			}
			assertSameTree(t, root, before)
			t.Logf("%s → %s", tc.name, mustJSON(t, got))
		})
	}
}
