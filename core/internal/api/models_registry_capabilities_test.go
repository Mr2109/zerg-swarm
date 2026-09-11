package api

// models_registry_capabilities_test.go — GET /api/models/registry 的**能力快照**专项测试
// （待修补 #26）。
//
// 背景：批 4c-后端2 之后，记录正文 `~/.zerg/models/manifests/<id>/<version>.json` 只装身份，
// **不再装 capabilities**；实测能力 + 证据 + 出处（endpoint/generated_at/online_probed）
// 放在记录旁的兄弟文件 `<version>.capabilities.json`。本接口原先仍只读 rec.Capabilities，
// 导致 UI 能力芯片永远为空——本文件逐条钉死修好后的行为。
//
// 既有 models_registry_test.go / models_registry_license_test.go 的用例一字不动（不得改弱），
// 这里只做加法。写盘一律 t.TempDir()（真实 Store.Put + 真实 WriteCapabilitySnapshot）；
// **绝不碰 ~/.zerg**。

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
)

// registryCapByName 从响应里按 name 取一条能力（找不到返回 nil）。
func registryCapByName(caps []ModelRegistryCapability, name string) *ModelRegistryCapability {
	for i := range caps {
		if caps[i].Name == name {
			return &caps[i]
		}
	}
	return nil
}

// writeSnapshotForRecord 用**真实**的 modelreg.WriteCapabilitySnapshot 落一份快照兄弟文件。
// recPath 必须是记录的实际落盘路径（putRecord 的返回值）。
func writeSnapshotForRecord(t *testing.T, recPath string, rec *modelreg.Record, rep *modelreg.ProbeReport) string {
	t.Helper()
	sib, err := modelreg.WriteCapabilitySnapshot(recPath, rec, rep)
	if err != nil {
		t.Fatalf("写能力快照失败（造数据）: %v", err)
	}
	return sib
}

// TestModelRegistryAPI_CapabilitiesFromSnapshot 正例：
// 记录正文无能力 + 快照有实测能力 → 响应里能拿到能力（带 source/evidence）
// 与三个出处字段（endpoint/generated_at/online_probed 均为真实值）。
func TestModelRegistryAPI_CapabilitiesFromSnapshot(t *testing.T) {
	root := t.TempDir()
	// 记录正文**没有** capabilities（与 probe 产出一致）
	rec := validRegistryRecord("snapmodel", "sha256:"+strings.Repeat("a", 64), "unknown", 8192)
	recPath := putRecord(t, root, rec)

	const endpoint = "http://127.0.0.1:18080/v1"
	genAt := time.Date(2026, 9, 12, 3, 4, 5, 0, time.UTC)
	rep := &modelreg.ProbeReport{
		Endpoint:     endpoint,
		GeneratedAt:  genAt,
		OnlineProbed: true,
		Capabilities: []modelreg.Capability{
			{Name: "text", Value: true, Source: "probed", Evidence: modelreg.EvidenceText},
			{Name: "vision", Value: false, Source: "probed", Evidence: modelreg.EvidenceVision + " (no_vision: http 400)"},
			{Name: "tools", Value: true, Source: "probed", Evidence: modelreg.EvidenceTools},
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

	// —— 能力来自快照（记录正文一个能力都没有，只能是快照给的）——
	if len(got.Capabilities) != 3 {
		t.Fatalf("能力应完全来自快照（3 条）——实际 %s", mustJSON(t, got.Capabilities))
	}
	wantText := ModelRegistryCapability{Name: "text", Value: true, Source: "probed", Evidence: modelreg.EvidenceText}
	if c := registryCapByName(got.Capabilities, "text"); c == nil || *c != wantText {
		t.Errorf("text 断言应与快照逐字段一致——got=%+v want=%+v", c, wantText)
	}
	if c := registryCapByName(got.Capabilities, "vision"); c == nil || c.Value || c.Source != "probed" || !strings.Contains(c.Evidence, modelreg.EvidenceVision) {
		t.Errorf("vision 断言（false+证据）不符——实际 %+v", c)
	}
	// 反例红线：记录正文里没有任何能力，响应里出现的能力**必须带 source 与 evidence**
	for _, c := range got.Capabilities {
		if c.Source == "" || c.Evidence == "" {
			t.Errorf("能力 %q 缺 source/evidence（标准 §四）——%+v", c.Name, c)
		}
	}

	// —— 快照出处三字段＝真实值 ——
	if got.SnapshotEndpoint != endpoint {
		t.Errorf("snapshot_endpoint 应为 %q——实际 %q", endpoint, got.SnapshotEndpoint)
	}
	if got.SnapshotGeneratedAt != "2026-09-12T03:04:05Z" {
		t.Errorf("snapshot_generated_at 应为 2026-09-12T03:04:05Z——实际 %q", got.SnapshotGeneratedAt)
	}
	if got.SnapshotOnlineProbed == nil || !*got.SnapshotOnlineProbed {
		t.Errorf("snapshot_online_probed 应为 true——实际 %v", got.SnapshotOnlineProbed)
	}

	// 原始 JSON 层面确认真值被写出去（不是靠结构体字段骗过断言）
	body := string(raw)
	for _, want := range []string{
		`"snapshot_endpoint":"` + endpoint + `"`,
		`"snapshot_generated_at":"2026-09-12T03:04:05Z"`,
		`"snapshot_online_probed":true`,
		`"evidence":"` + modelreg.EvidenceText + `"`,
		`"source":"probed"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("响应里缺少真实快照值 %s——body=%s", want, body)
		}
	}

	// 只读：快照文件请求后仍在、内容不变（请求不写盘）
	if _, err := os.Stat(sibPath); err != nil {
		t.Errorf("只读被破坏：请求后快照文件应仍存在——err=%v", err)
	}
	assertSameTree(t, root, before)
	t.Logf("snapshot caps+provenance → %s", mustJSON(t, got))
}

// TestModelRegistryAPI_RecordOnlyNoSnapshot 反例：
// 只有记录正文（且正文无能力）、没有快照兄弟文件 → 能力为空、无 500、**不凭空造**，
// 且请求不得创建快照文件（只读）。
func TestModelRegistryAPI_RecordOnlyNoSnapshot(t *testing.T) {
	root := t.TempDir()
	rec := validRegistryRecord("nosnap", "sha256:"+strings.Repeat("b", 64), "unknown", 4096)
	recPath := putRecord(t, root, rec)
	sibPath := modelreg.CapabilitySnapshotPath(recPath)
	// 前提：快照兄弟文件确实不存在
	if _, err := os.Stat(sibPath); !os.IsNotExist(err) {
		t.Fatalf("前提不成立：快照文件本不该存在——err=%v", err)
	}

	before := snapshotTree(t, root)
	h := newTestHandlers()
	h.ModelsDir = root
	raw, resp := getRegistryRaw(t, h)

	if resp.Count != 1 || resp.BadRecords != 0 {
		t.Fatalf("预期 count=1 bad_records=0——实际 count=%d bad=%d", resp.Count, resp.BadRecords)
	}
	got := resp.Records[0]
	if len(got.Capabilities) != 0 {
		t.Errorf("无快照、正文也无能力时应为空——实际 %s", mustJSON(t, got.Capabilities))
	}
	// 出处字段整键不出现（没有快照可溯源，不许造）
	if got.SnapshotEndpoint != "" || got.SnapshotGeneratedAt != "" || got.SnapshotOnlineProbed != nil {
		t.Errorf("无快照时出处字段必须全空——实际 %+v", got)
	}
	body := string(raw)
	for _, key := range []string{`"snapshot_endpoint"`, `"snapshot_generated_at"`, `"snapshot_online_probed"`} {
		if strings.Contains(body, key) {
			t.Errorf("无快照时 %s 不应出现在响应里——body=%s", key, body)
		}
	}
	if !strings.Contains(body, `"capabilities":[]`) {
		t.Errorf("能力应为空数组 []（不是 null/省略）——body=%s", body)
	}

	// 只读硬证据：请求后快照文件仍**不存在**（接口没自己造一份）
	if _, err := os.Stat(sibPath); !os.IsNotExist(err) {
		t.Fatalf("只读被破坏：请求后不应出现快照文件——err=%v", err)
	}
	assertSameTree(t, root, before)
	t.Logf("record-only(no caps,no snapshot) → %s", mustJSON(t, got))
}

// TestModelRegistryAPI_BrokenSnapshot 反例：
// 快照是坏 JSON → 整个请求不 500、bad_records 不因此计数、该条能力为空、不造出处；
// 且坏快照文件请求后仍原样存在（只读，不修不删）。
func TestModelRegistryAPI_BrokenSnapshot(t *testing.T) {
	root := t.TempDir()
	// 一条好记录（带声明能力）——坏快照不得影响它
	good := validRegistryRecord("good", "sha256:"+strings.Repeat("c", 64), "unknown", 8192, "text")
	putRecord(t, root, good)
	// 一条记录 + 一个坏 JSON 快照兄弟文件
	brokenRec := validRegistryRecord("broken", "sha256:"+strings.Repeat("d", 64), "unknown", 4096)
	brokenPath := putRecord(t, root, brokenRec)
	brokenSib := modelreg.CapabilitySnapshotPath(brokenPath)
	brokenBytes := []byte("{ this is definitely not a capability snapshot json")
	if err := os.WriteFile(brokenSib, brokenBytes, 0o644); err != nil {
		t.Fatalf("造坏快照失败: %v", err)
	}

	before := snapshotTree(t, root)
	h := newTestHandlers()
	h.ModelsDir = root
	raw, resp := getRegistryRaw(t, h)

	if resp.Count != 2 || resp.BadRecords != 0 {
		t.Fatalf("坏快照不是记录错误：预期 count=2 bad_records=0——实际 count=%d bad=%d",
			resp.Count, resp.BadRecords)
	}
	var broken *ModelRegistryRecord
	for i := range resp.Records {
		if resp.Records[i].ID == "broken" {
			broken = &resp.Records[i]
		}
	}
	if broken == nil {
		t.Fatalf("broken 记录应在响应里——实际 %s", mustJSON(t, resp.Records))
	}
	if len(broken.Capabilities) != 0 {
		t.Errorf("坏快照 → 该条能力应为空（降级，不 500、不造）——实际 %s", mustJSON(t, broken.Capabilities))
	}
	if broken.SnapshotEndpoint != "" || broken.SnapshotGeneratedAt != "" || broken.SnapshotOnlineProbed != nil {
		t.Errorf("坏快照 → 出处字段必须全空——实际 %+v", broken)
	}
	if broken.Errors != 0 {
		t.Errorf("坏快照不该计入该条 errors——实际 %d", broken.Errors)
	}
	// 好记录不受别人坏快照影响
	for _, rec := range resp.Records {
		if rec.ID == "good" && (len(rec.Capabilities) != 1 || rec.Capabilities[0].Name != "text") {
			t.Errorf("好记录的正文能力应照常给出——实际 %s", mustJSON(t, rec.Capabilities))
		}
	}
	// 整个响应里不该出现任何出处字段（两条记录都没有可用快照）
	body := string(raw)
	for _, key := range []string{`"snapshot_endpoint"`, `"snapshot_generated_at"`, `"snapshot_online_probed"`} {
		if strings.Contains(body, key) {
			t.Errorf("坏快照不得产出出处字段 %s——body=%s", key, body)
		}
	}

	// 只读硬证据：坏快照文件请求后仍原样存在（不修、不删、不重写）
	after, err := os.ReadFile(brokenSib)
	if err != nil {
		t.Fatalf("只读被破坏：请求后坏快照文件应仍存在——err=%v", err)
	}
	if string(after) != string(brokenBytes) {
		t.Fatalf("只读被破坏：坏快照内容被改动\n before=%q\n after =%q", brokenBytes, after)
	}
	assertSameTree(t, root, before)
	t.Logf("broken snapshot tolerated → %s", mustJSON(t, broken))
}

// TestModelRegistryAPI_SnapshotOverridesRecord 反例优先（两者都有）：
// 正文有声明能力 + 快照有实测能力 → **同名以快照为准**（值/source/evidence 均来自快照），
// 正文独有、快照没有的能力照常保留；且快照 false 与“缺值”语义分明
// （online_probed=false 如实出现，空 endpoint/generated_at 整键不出现）。
func TestModelRegistryAPI_SnapshotOverridesRecord(t *testing.T) {
	root := t.TempDir()
	// 正文声明 text + reasoning（source=declared）
	rec := validRegistryRecord("both", "sha256:"+strings.Repeat("e", 64), "unknown", 8192, "text", "reasoning")
	recPath := putRecord(t, root, rec)

	// 快照实测 text（覆盖正文那条）+ vision + tools
	rep := &modelreg.ProbeReport{
		// 故意留空 endpoint/generated_at：验证“缺就缺”——整键不出现
		Endpoint:     "",
		GeneratedAt:  time.Time{},
		OnlineProbed: false, // 真实取值 false：必须如实出现，不能与“没有快照”混同
		Capabilities: []modelreg.Capability{
			{Name: "text", Value: true, Source: "probed", Evidence: modelreg.EvidenceText},
			{Name: "vision", Value: false, Source: "probed", Evidence: modelreg.EvidenceVision},
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

	// 顺序：正文顺序优先（text 换成快照值、reasoning 保留），随后补快照独有（vision、tools）
	gotNames := make([]string, 0, len(got.Capabilities))
	for _, c := range got.Capabilities {
		gotNames = append(gotNames, c.Name)
	}
	if strings.Join(gotNames, ",") != "text,reasoning,vision,tools" {
		t.Fatalf("合并顺序应为 text,reasoning,vision,tools——实际 %v", gotNames)
	}
	// 同名 text：以快照为准（probed；正文那条 declared 被换掉）
	if c := registryCapByName(got.Capabilities, "text"); c == nil || c.Source != "probed" || c.Evidence != modelreg.EvidenceText {
		t.Errorf("同名 text 必须以快照为准（probed + 探测器证据）——实际 %+v", c)
	}
	// 正文独有 reasoning：保留（source 仍是 declared）
	if c := registryCapByName(got.Capabilities, "reasoning"); c == nil || c.Source != "declared" {
		t.Errorf("正文独有的 reasoning 应保留为 declared——实际 %+v", c)
	}
	for _, name := range []string{"vision", "tools"} {
		if c := registryCapByName(got.Capabilities, name); c == nil || c.Source != "probed" {
			t.Errorf("快照独有的 %s 应补进来（probed）——实际 %+v", name, c)
		}
	}

	// 出处：online_probed=false 如实出现；空 endpoint/generated_at 整键不出现（缺就缺）
	if got.SnapshotOnlineProbed == nil || *got.SnapshotOnlineProbed {
		t.Errorf("snapshot_online_probed 应为 false（真实取值须如实给出）——实际 %v", got.SnapshotOnlineProbed)
	}
	body := string(raw)
	if !strings.Contains(body, `"snapshot_online_probed":false`) {
		t.Errorf("响应应含 snapshot_online_probed=false——body=%s", body)
	}
	for _, key := range []string{`"snapshot_endpoint"`, `"snapshot_generated_at"`} {
		if strings.Contains(body, key) {
			t.Errorf("空值字段 %s 应整键不出现（缺就缺，不造值）——body=%s", key, body)
		}
	}

	assertSameTree(t, root, before)
	t.Logf("snapshot overrides record → %s", mustJSON(t, got))
}
