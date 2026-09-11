package api

// models_registry_license_test.go — GET /api/models/registry 的 **license 块** 专项测试。
//
// 为什么单独一个文件：既有 models_registry_test.go 的四个用例一字不改（不得改弱），
// 这里只做加法。
//
// 覆盖（反例优先）：
//   - 反例「缺值就缺」：只给 spdx + commercial 的记录 → 响应里 license 块存在但
//     name/link/source_url/accepted_by/accepted_at **整键不出现**（不许用空串/占位串充数）；
//     同一次请求里混一条坏记录 → 坏记录**没有** license 块（不许凭空造一个许可出来）。
//   - 正例「真实值贯通」：完整 license（含 accepted_by/accepted_at）走真实的
//     Store.Put 落盘 → 响应里逐字段等于落盘值，且原始 JSON 里能看到真实留痕。
//   - 两条用例都断言响应里**不出现**标准 §五 的禁用占位词（项目硬规则：留痕不许占位）。
//
// 写盘一律 t.TempDir()——绝不碰 ~/.zerg。

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
)

// licensingRecord 造一条带指定 license 的合法记录（其余字段复用 validRegistryRecord）。
func licensingRecord(id, digest string, lic modelreg.License) *modelreg.Record {
	rec := validRegistryRecord(id, digest, lic.Commercial, 8192, "text", "tools")
	rec.License = lic
	return rec
}

// getRegistryRaw 与 getRegistry 同语义，但把**原始响应体**也交出来（要断键的存在与否）。
func getRegistryRaw(t *testing.T, h *Handlers) ([]byte, registryResp) {
	t.Helper()
	w := doRegistryReq(t, newRegistryTestRouter(h), "test-token")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200——实际 %d body=%s", w.Code, w.Body.String())
	}
	raw := append([]byte(nil), w.Body.Bytes()...)
	var resp registryResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("响应 JSON 解析失败: %v body=%s", err, raw)
	}
	return raw, resp
}

// licensePlaceholderWords 与 modelreg 门禁的禁用占位词同表（标准 §五）。
// 这里独立再列一遍：测的是「响应里不出现」，不该依赖被测包自身的表。
var licensePlaceholderWords = []string{
	"unset", "pending", "tbd", "todo", "n/a", "na", "placeholder",
	"unknown", "待定", "未定", "none", "null", "-",
}

// assertNoTracePlaceholders 断言响应里没有任何占位串当值出现（不含 unknown：
// commercial 的合法四态里本来就有 "unknown"，那是真实取值不是占位）。
func assertNoTracePlaceholders(t *testing.T, raw []byte) {
	t.Helper()
	body := strings.ToLower(string(raw))
	for _, w := range licensePlaceholderWords {
		if w == "unknown" { // commercial=unknown 是标准 §五 的合法状态
			continue
		}
		if strings.Contains(body, `"`+w+`"`) {
			t.Errorf("响应里出现禁用占位值 %q（标准 §五：留痕不许占位）—— body=%s", w, raw)
		}
	}
}

// TestModelRegistryAPI_LicenseSparseAndBadRecord 反例优先：
// 缺值就缺（optional 键整键不出现），坏记录不凭空生成 license 块。
func TestModelRegistryAPI_LicenseSparseAndBadRecord(t *testing.T) {
	root := t.TempDir()
	// 只有 spdx + commercial：无 name/link/source_url，也无任何人工留痕
	putRecord(t, root, licensingRecord("sparse",
		"sha256:"+strings.Repeat("4", 64),
		modelreg.License{SPDX: "apache-2.0", Commercial: "yes"}))

	// 同目录混一条坏记录（故意写坏 JSON）
	badDir := filepath.Join(root, "manifests", "badmodel")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("造坏记录目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "sha256-deadbeef0002.json"), []byte("{ not json at all"), 0o644); err != nil {
		t.Fatalf("造坏记录失败: %v", err)
	}

	before := snapshotTree(t, root)
	h := newTestHandlers()
	h.ModelsDir = root
	raw, resp := getRegistryRaw(t, h)

	if resp.Count != 2 || resp.BadRecords != 1 {
		t.Fatalf("预期 count=2 bad_records=1——实际 count=%d bad=%d", resp.Count, resp.BadRecords)
	}

	var sparse, bad *ModelRegistryRecord
	for i := range resp.Records {
		switch resp.Records[i].ID {
		case "sparse":
			sparse = &resp.Records[i]
		case "badmodel":
			bad = &resp.Records[i]
		}
	}
	if sparse == nil || bad == nil {
		t.Fatalf("应同时拿到 sparse 与 badmodel——实际 %s", mustJSON(t, resp.Records))
	}

	// —— 反例 1：坏记录不得出现 license 块（读取失败 ≠ 有一条空许可）——
	if bad.License != nil {
		t.Errorf("坏记录不应有 license 块——实际 %+v", *bad.License)
	}

	// —— 反例 2：只有 spdx+commercial 时，license 块在，缺的字段整键不出现 ——
	if sparse.License == nil {
		t.Fatalf("sparse 记录应有 license 块——实际 %s", mustJSON(t, sparse))
	}
	if sparse.License.SPDX != "apache-2.0" {
		t.Errorf("spdx 应为 apache-2.0——实际 %q", sparse.License.SPDX)
	}
	if sparse.License.Commercial != "yes" {
		t.Errorf("license.commercial 应为 yes——实际 %q", sparse.License.Commercial)
	}
	if sparse.License.Gated {
		t.Errorf("未声明的 gated 应为 false——实际 true")
	}
	if sparse.License.Name != "" || sparse.License.Link != "" || sparse.License.SourceURL != "" ||
		sparse.License.AcceptedBy != "" || sparse.License.AcceptedAt != "" {
		t.Errorf("无留痕记录的许可细节字段应全空——实际 %+v", *sparse.License)
	}
	// 顶层兼容字段保留（UI 现依赖）
	if sparse.Commercial != "yes" {
		t.Errorf("顶层兼容字段 commercial 应为 yes——实际 %q", sparse.Commercial)
	}

	// 原始 JSON 层面：缺的键**必须整键不出现**（不是空串、更不是占位串）
	body := string(raw)
	for _, key := range []string{`"accepted_by"`, `"accepted_at"`, `"source_url"`, `"link"`} {
		if strings.Contains(body, key) {
			t.Errorf("缺值字段 %s 不应出现在响应里——body=%s", key, body)
		}
	}
	if !strings.Contains(body, `"license":{"spdx":"apache-2.0","commercial":"yes","gated":false}`) {
		t.Errorf("license 块形状不符（应为 spdx/commercial/gated 三项）——body=%s", body)
	}
	assertNoTracePlaceholders(t, raw)

	assertSameTree(t, root, before) // 只读
	t.Logf("sparse+bad → %s", mustJSON(t, resp.Records))
}

// TestModelRegistryAPI_LicenseFullTrace 正例（真实值贯通）：
// 完整 license（含受限许可的人工留痕）→ 响应里逐字段等于落盘值，原始 JSON 能看到真实留痕。
func TestModelRegistryAPI_LicenseFullTrace(t *testing.T) {
	root := t.TempDir()
	lic := modelreg.License{
		SPDX:       "other",
		Name:       "通义千问自定义许可",
		Link:       "https://example.invalid/qwen-custom-license",
		Commercial: "revenue_gated",
		Gated:      true,
		SourceURL:  "https://example.invalid/qwen-weights",
		AcceptedBy: "Mr2109",
		AcceptedAt: "2026-09-12",
	}
	putRecord(t, root, licensingRecord("qwenlic", "sha256:"+strings.Repeat("5", 64), lic))

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
	if *got.License != (ModelRegistryLicense{
		SPDX: lic.SPDX, Name: lic.Name, Link: lic.Link, Commercial: lic.Commercial,
		Gated: lic.Gated, SourceURL: lic.SourceURL, AcceptedBy: lic.AcceptedBy, AcceptedAt: lic.AcceptedAt,
	}) {
		t.Errorf("license 块与落盘值不符:\n got=%+v\n want=%+v", *got.License, lic)
	}
	// 顶层兼容字段保留不动（值来自 Store.List 的裁剪行）
	if got.Commercial != "revenue_gated" {
		t.Errorf("顶层兼容字段 commercial 应为 revenue_gated——实际 %q", got.Commercial)
	}
	// 红线：受限许可不得作默认项
	if got.DefaultEligible {
		t.Errorf("commercial=revenue_gated 时 default_eligible 必须为 false")
	}

	// 原始 JSON 里留痕与来源必须是真的写进去的（不是靠结构体字段名骗过断言）
	body := string(raw)
	for _, want := range []string{
		`"accepted_by":"Mr2109"`,
		`"accepted_at":"2026-09-12"`,
		`"source_url":"https://example.invalid/qwen-weights"`,
		`"link":"https://example.invalid/qwen-custom-license"`,
		`"name":"通义千问自定义许可"`,
		`"spdx":"other"`,
		`"gated":true`,
		`"commercial":"revenue_gated"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("响应里缺少真实值 %s——body=%s", want, body)
		}
	}
	assertNoTracePlaceholders(t, raw)

	assertSameTree(t, root, before) // 只读
	t.Logf("full license → %s", mustJSON(t, resp.Records))
}
