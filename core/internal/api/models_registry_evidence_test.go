package api

// models_registry_evidence_test.go — GET /api/models/registry 的**证据链三样**专项测试
// （待修补 #39）。
//
// 背景：模型登记库详情要回答Mr2109的三句「这个结论是从哪来的」——
//  ① 能力断言的**引擎维度** `capabilities[].engines`（数据由 #11 加入能力快照）；
//  ② **无法判定**的能力 `unverifiable[]`（数据 #27/#28 已在快照里，本文件顺带核对）；
//  ③ 许可证结论的**来源锚** `license.evidence`（数据由 #12 加入记录）。
// 本文件逐条钉死接口只读侧的带出行为（反例优先）。
//
// 硬口径（与既有 snapshot_* 字段完全一致）：**缺就截缺**——字段不存在 / 空数组 /
// 全是空白串 / 坏 JSON 快照 → 该键**整键不出现**（omitempty），绝不填占位值、
// 绝不不 500、绝不造空数组。写盘一律 t.TempDir()；**绝不碰 ~/.zerg**。

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
)

// TestModelRegistryAPI_CapabilityEnginesFromSnapshot 正例：
// 快照里的能力条带 engines[]（含多引擎、重复、需去空白的写法）→
// 响应逐条带出，只做「去空白 + 丢空串 + 去重」，**保持原顺序**；原始 JSON 里逐字出现。
func TestModelRegistryAPI_CapabilityEnginesFromSnapshot(t *testing.T) {
	root := t.TempDir()
	rec := validRegistryRecord("engmodel", "sha256:"+strings.Repeat("1", 64), "unknown", 8192)
	recPath := putRecord(t, root, rec)

	rep := &modelreg.ProbeReport{
		Endpoint:     "http://127.0.0.1:18085/v1",
		GeneratedAt:  time.Date(2026, 9, 13, 1, 2, 3, 0, time.UTC),
		OnlineProbed: true,
		Capabilities: []modelreg.Capability{
			// 多引擎 + 重复 + 首尾空白 + 空串：读侧只收敛，不改写引擎名
			{Name: "vision", Value: true, Source: "probed", Evidence: modelreg.EvidenceVision,
				Engines: []string{" llama.cpp ", "llama.cpp", "", "vllm"}},
			// 单引擎
			{Name: "text", Value: true, Source: "probed", Evidence: modelreg.EvidenceText,
				Engines: []string{"llama.cpp"}},
			// 无引擎维度（旧快照/未给 --engine）：不得凭空补一个
			{Name: "tools", Value: true, Source: "probed", Evidence: modelreg.EvidenceTools},
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

	vision := registryCapByName(got.Capabilities, "vision")
	if vision == nil {
		t.Fatalf("应有 vision 能力——实际 %s", mustJSON(t, got.Capabilities))
	}
	wantVisionEngines := []string{"llama.cpp", "vllm"} // 去空白 + 丢空串 + 去重 + 保序
	if !reflect.DeepEqual(vision.Engines, wantVisionEngines) {
		t.Errorf("vision.engines 应为 %v——实际 %v", wantVisionEngines, vision.Engines)
	}
	if text := registryCapByName(got.Capabilities, "text"); text == nil || !reflect.DeepEqual(text.Engines, []string{"llama.cpp"}) {
		t.Errorf("text.engines 应为 [llama.cpp]——实际 %+v", text)
	}
	// 无引擎维度的断言：Engines 必须为空（缺 = 未知，绝不补默认引擎）
	if tools := registryCapByName(got.Capabilities, "tools"); tools == nil || len(tools.Engines) != 0 {
		t.Errorf("无引擎维度的 tools 不得带 engines——实际 %+v", tools)
	}

	// 原始 JSON 层面：真实引擎值被写出去；无引擎的那条**不带 engines 键**
	body := string(raw)
	if !strings.Contains(body, `"engines":["llama.cpp","vllm"]`) {
		t.Errorf("响应里 vision.engines 未逐字写出去——body=%s", body)
	}
	if strings.Count(body, `"engines"`) != 2 {
		t.Errorf("应恰好只有 2 条能力带 engines —— 实际出现 %d 次（body=%s）",
			strings.Count(body, `"engines"`), body)
	}
	// 只读：请求不写盘
	if _, err := os.Stat(modelreg.CapabilitySnapshotPath(recPath)); err != nil {
		t.Errorf("只读被破坏：快照文件应仍存在——err=%v", err)
	}
	assertSameTree(t, root, before)
	t.Logf("engines 贯通 → %s", mustJSON(t, got))
}

// TestModelRegistryAPI_CapabilityEnginesAbsent 反例优先：
// 无快照 / 坏 JSON 快照 / 缺 engines 字段 / 显式空数组 / 全是空白串 →
// 响应里能力条**不带 engines 键**（整键不出现，不是 null / [] / 占位串），且不 500。
func TestModelRegistryAPI_CapabilityEnginesAbsent(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) string
	}{
		{
			name: "反例-无快照文件：engines 整键不出现",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				putRecord(t, root, validRegistryRecord("noeng", "sha256:"+strings.Repeat("2", 64), "unknown", 4096, "text"))
				return root
			},
		},
		{
			name: "反例-坏 JSON 快照：engines 整键不出现、不 500",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				rec := validRegistryRecord("badeng", "sha256:"+strings.Repeat("3", 64), "unknown", 4096)
				recPath := putRecord(t, root, rec)
				sib := modelreg.CapabilitySnapshotPath(recPath)
				if err := os.WriteFile(sib, []byte(`{ "capabilities": [ {"name":"text","engines":[`), 0o644); err != nil {
					t.Fatalf("造坏快照失败: %v", err)
				}
				return root
			},
		},
		{
			name: "反例-快照显式空数组 engines:[]：整键不出现（不是空数组）",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				rec := validRegistryRecord("emptyeng", "sha256:"+strings.Repeat("4", 64), "unknown", 4096)
				recPath := putRecord(t, root, rec)
				sib := modelreg.CapabilitySnapshotPath(recPath)
				raw := `{"schema":"zerg.model.capability_snapshot.v1","id":"emptyeng",` +
					`"capabilities":[{"name":"text","value":true,"source":"probed",` +
					`"evidence":"probe.text.v1 ok","engines":[]}]}`
				if err := os.WriteFile(sib, []byte(raw), 0o644); err != nil {
					t.Fatalf("造空数组快照失败: %v", err)
				}
				return root
			},
		},
		{
			name: "反例-engines 全是空白串：整键不出现、不填占位",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				rec := validRegistryRecord("blankeng", "sha256:"+strings.Repeat("5", 64), "unknown", 4096)
				recPath := putRecord(t, root, rec)
				// 真实的写盘器：断言里只给空白引擎
				rep := &modelreg.ProbeReport{
					Endpoint:     "http://127.0.0.1:18086/v1",
					GeneratedAt:  time.Date(2026, 9, 13, 2, 3, 4, 0, time.UTC),
					OnlineProbed: true,
					Capabilities: []modelreg.Capability{
						{Name: "text", Value: true, Source: "probed", Evidence: modelreg.EvidenceText,
							Engines: []string{"  ", ""}},
					},
				}
				writeSnapshotForRecord(t, recPath, rec, rep)
				return root
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.setup(t)
			before := snapshotTree(t, root)
			h := newTestHandlers()
			h.ModelsDir = root
			raw, resp := getRegistryRaw(t, h) // 内部要求 200：顺带证明坏快照不 500

			if resp.Count != 1 || resp.BadRecords != 0 {
				t.Fatalf("预期 count=1 bad_records=0——实际 count=%d bad=%d", resp.Count, resp.BadRecords)
			}
			got := resp.Records[0]
			for _, c := range got.Capabilities {
				if len(c.Engines) != 0 {
					t.Errorf("能力 %q 不该带 engines——实际 %v", c.Name, c.Engines)
				}
			}
			// 整键不出现（原文级）——绝不能是 "engines":null / []
			if strings.Contains(string(raw), `"engines"`) {
				t.Errorf("engines 应整键不出现——body=%s", raw)
			}
			assertSameTree(t, root, before)
			t.Logf("%s → %s", tc.name, mustJSON(t, got))
		})
	}
}

// TestModelRegistryAPI_LicenseEvidence 正例 + 反例：
//   - 记录带 license.evidence → 响应逐字带出（真实值贯通）；
//   - 记录没有 evidence → `evidence` 键整键不出现（不是空串、更不是占位串）。
func TestModelRegistryAPI_LicenseEvidence(t *testing.T) {
	const anchor = `probe.license.v1 (gguf_key: general.license="apache-2.0")`

	t.Run("正例-带来源锚：逐字贯通", func(t *testing.T) {
		root := t.TempDir()
		rec := licensingRecord("evlic", "sha256:"+strings.Repeat("6", 64), modelreg.License{
			SPDX: "apache-2.0", Commercial: "unknown", Evidence: anchor,
		})
		putRecord(t, root, rec)

		before := snapshotTree(t, root)
		h := newTestHandlers()
		h.ModelsDir = root
		raw, resp := getRegistryRaw(t, h)

		if resp.Count != 1 || resp.BadRecords != 0 {
			t.Fatalf("预期 count=1 bad_records=0——实际 count=%d bad=%d", resp.Count, resp.BadRecords)
		}
		got := resp.Records[0]
		if got.License == nil {
			t.Fatalf("应有 license 块——实际 %s", mustJSON(t, got))
		}
		if got.License.Evidence != anchor {
			t.Errorf("license.evidence 应逐字照抄——got=%q want=%q", got.License.Evidence, anchor)
		}
		body := string(raw)
		if !strings.Contains(body, `"evidence":"probe.license.v1 (gguf_key: general.license=\"apache-2.0\")"`) {
			t.Errorf("响应里 license.evidence 未逐字写出去——body=%s", body)
		}
		assertNoTracePlaceholders(t, raw)
		assertSameTree(t, root, before)
		t.Logf("license.evidence → %s", mustJSON(t, got))
	})

	t.Run("反例-无来源锚：evidence 整键不出现", func(t *testing.T) {
		root := t.TempDir()
		// 只有 spdx + commercial：无 evidence（照 probe 拿不到结构化来源之外的旧记录形状）
		putRecord(t, root, licensingRecord("noev", "sha256:"+strings.Repeat("7", 64),
			modelreg.License{SPDX: "mit", Commercial: "yes"}))

		before := snapshotTree(t, root)
		h := newTestHandlers()
		h.ModelsDir = root
		raw, resp := getRegistryRaw(t, h)

		if resp.Count != 1 || resp.BadRecords != 0 {
			t.Fatalf("预期 count=1 bad_records=0——实际 count=%d bad=%d", resp.Count, resp.BadRecords)
		}
		got := resp.Records[0]
		if got.License == nil {
			t.Fatalf("应有 license 块——实际 %s", mustJSON(t, got))
		}
		if got.License.Evidence != "" {
			t.Errorf("无来源锚时 evidence 应为空——实际 %q", got.License.Evidence)
		}
		if strings.Contains(string(raw), `"evidence":"probe.license.v1`) {
			t.Errorf("无来源锚时不该出现 license.evidence——body=%s", raw)
		}
		assertSameTree(t, root, before)
		t.Logf("无 license.evidence → %s", mustJSON(t, got))
	})
}

// TestLicenseBlockEmpty_CountsEvidence 纯单元：evidence 有值时 license 块不算空
// （否则会把唯一的一环证据链丢掉）；真正全空才算空。
func TestLicenseBlockEmpty_CountsEvidence(t *testing.T) {
	if !licenseBlockEmpty(modelreg.License{}) {
		t.Error("全零值 license 应判为空块")
	}
	if licenseBlockEmpty(modelreg.License{Evidence: "probe.license.v1 (x)"}) {
		t.Error("只带 evidence 的 license 不是空块——证据链不能整块丢掉")
	}
}

// TestModelRegistryAPI_EvidenceChainCoexist 证据链三样共存：
// 同一条记录同时带 engines[] + unverifiable[] + license.evidence →
// 三者都在同一次响应里如实出现，且不出现任何禁用占位词；混一条坏记录也不影响。
func TestModelRegistryAPI_EvidenceChainCoexist(t *testing.T) {
	root := t.TempDir()
	rec := validRegistryRecord("chain", "sha256:"+strings.Repeat("8", 64), "unknown", 8192)
	rec.License = modelreg.License{SPDX: "apache-2.0", Commercial: "unknown",
		Evidence: `probe.license.v1 (license_file: LICENSE)`}
	recPath := putRecord(t, root, rec)

	// 同目录混一条坏记录（证明证据链带出与坏记录容错互不干扰）
	badDir := root + "/manifests/badchain"
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("造坏记录目录失败: %v", err)
	}
	if err := os.WriteFile(badDir+"/sha256-deadbeef0009.json", []byte("{ not json"), 0o644); err != nil {
		t.Fatalf("造坏记录失败: %v", err)
	}

	rep := &modelreg.ProbeReport{
		Endpoint:     "http://127.0.0.1:18087/v1",
		GeneratedAt:  time.Date(2026, 9, 13, 3, 4, 5, 0, time.UTC),
		OnlineProbed: true,
		Capabilities: []modelreg.Capability{
			{Name: "text", Value: true, Source: "probed", Evidence: modelreg.EvidenceText,
				Engines: []string{"llama.cpp"}},
		},
		Unverifiable: []modelreg.Unverifiable{
			{Name: "embedding", Reason: "budget_exhausted", Evidence: uvEvidenceEmbedding},
		},
	}
	writeSnapshotForRecord(t, recPath, rec, rep)

	before := snapshotTree(t, root)
	h := newTestHandlers()
	h.ModelsDir = root
	raw, resp := getRegistryRaw(t, h)

	if resp.Count != 2 || resp.BadRecords != 1 {
		t.Fatalf("预期 count=2 bad_records=1——实际 count=%d bad=%d", resp.Count, resp.BadRecords)
	}
	var got *ModelRegistryRecord
	for i := range resp.Records {
		if resp.Records[i].ID == "chain" {
			got = &resp.Records[i]
		}
	}
	if got == nil {
		t.Fatalf("应有 chain 记录——实际 %s", mustJSON(t, resp.Records))
	}

	// ① 引擎维度
	if c := registryCapByName(got.Capabilities, "text"); c == nil || !reflect.DeepEqual(c.Engines, []string{"llama.cpp"}) {
		t.Errorf("① engines 未带出——实际 %+v", c)
	}
	// ② 无法判定（核对 #28 口径）
	if uv := registryUnverifiableByName(got.Unverifiable, "embedding"); uv == nil || uv.Reason != "budget_exhausted" {
		t.Errorf("② unverifiable 未带出——实际 %s", mustJSON(t, got.Unverifiable))
	}
	// ③ 许可证来源锚
	if got.License == nil || got.License.Evidence != `probe.license.v1 (license_file: LICENSE)` {
		t.Errorf("③ license.evidence 未带出——实际 %+v", got.License)
	}

	// 原始 JSON：三样键都在
	body := string(raw)
	for _, key := range []string{`"engines":["llama.cpp"]`, `"unverifiable":[`, `"evidence":"probe.license.v1 (license_file: LICENSE)"`} {
		if !strings.Contains(body, key) {
			t.Errorf("响应里缺少 %s——body=%s", key, body)
		}
	}
	assertNoTracePlaceholders(t, raw)
	assertSameTree(t, root, before)
	t.Logf("证据链三样共存 → %s", mustJSON(t, got))
}
