package modelreg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func good() *Record {
	return &Record{
		Schema: SchemaV1,
		ID:     "qwen3-vl-8b-instruct",
		Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Files: []File{
			{Role: "weights", Name: "model-Q4_K_M.gguf", SHA256: "aaaa", Size: 5},
			{Role: "mmproj", Name: "mmproj-f16.gguf", SHA256: "bbbb", Size: 8},
		},
		Capabilities: []Capability{
			{Name: "vision", Value: true, Source: "probed", Evidence: "probe.vision.1x1.v1"},
		},
		License:   License{SPDX: "Apache-2.0", Commercial: "yes", SourceURL: "https://example.org"},
		SourceURL: "https://example.org",
	}
}

// V1 合法记录 → 无 error
func TestV1_Valid(t *testing.T) {
	if n := CountErrors(Verify(good(), false)); n != 0 {
		t.Fatalf("合法记录不该有 error，实际 %d 条", n)
	}
}

// V2 缺 digest → error
func TestV2_MissingDigest(t *testing.T) {
	r := good()
	r.Digest = ""
	if n := CountErrors(Verify(r, false)); n == 0 {
		t.Fatal("缺 digest 必须报 error")
	}
}

// V3 source=probed 但无 evidence → error
func TestV3_ProbedWithoutEvidence(t *testing.T) {
	r := good()
	r.Capabilities[0].Evidence = ""
	if n := CountErrors(Verify(r, false)); n == 0 {
		t.Fatal("probed 无 evidence 必须报 error")
	}
}

// V4 受限许可（no）但无留痕 → error（待修补 #21 修改后：no/revenue_gated 仍必填）
func TestV4_NoLicenseTrace(t *testing.T) {
	r := good()
	r.License.Commercial = "no"
	findings := Verify(r, false)
	if n := CountErrors(findings); n == 0 {
		t.Fatal("受限许可(no)无 accepted_by/at 必须报 error")
	}
	found := false
	for _, f := range findings {
		if f.Level == "error" && f.Field == "license" && strings.Contains(f.Detail, "留痕") {
			found = true
		}
	}
	if !found {
		t.Fatalf("应报一条 license 留痕 error，实际：%+v", findings)
	}
	r.License.AcceptedBy, r.License.AcceptedAt = "laodao", "2026-09-11T00:00:00Z"
	if n := CountErrors(Verify(r, false)); n != 0 {
		t.Fatal("补上真实留痕后应合格")
	}
}

// 待修补 #21 的五条反例。规则：
//
//	no / revenue_gated → accepted_by/accepted_at 都必须非空（留名留时间）；
//	unknown           → 允许留痕为空，且不报 error、不加 warn；
//	任何状态          → 留痕只要非空，就不许是占位词（否则等于骗过门禁）。
func TestV8_Rule21_NoCommercialRequiresTrace(t *testing.T) {
	r := good()
	r.License.Commercial = "no"
	r.License.AcceptedBy, r.License.AcceptedAt = "", ""
	findings := Verify(r, false)
	if n := CountErrors(findings); n != 1 {
		t.Fatalf("no 且留痕空应恰好 1 条 error，实际 %d 条：%+v", n, findings)
	}
	if findings[0].Field != "license" || !strings.Contains(findings[0].Detail, "留痕") {
		t.Fatalf("error 应落在 license 且说明留痕：%+v", findings[0])
	}
}

func TestV9_Rule21_NoCommercialPlaceholderRejected(t *testing.T) {
	r := good()
	r.License.Commercial = "no"
	r.License.AcceptedBy = "unset" // 占位值
	r.License.AcceptedAt = "2026-09-11T00:00:00Z"
	findings := Verify(r, false)
	if n := CountErrors(findings); n != 1 {
		t.Fatalf("no + accepted_by=unset 应恰好 1 条 error（占位），实际 %d 条：%+v", n, findings)
	}
	if !strings.Contains(findings[0].Detail, "占位") {
		t.Fatalf("错误文案必须提到占位：%+v", findings[0])
	}
}

func TestV10_Rule21_UnknownAllowsEmptyTrace(t *testing.T) {
	// 本次修改的核心：unknown 允许留痕为空，一条 error 都不许有。
	r := good()
	r.License.Commercial = "unknown"
	r.License.AcceptedBy, r.License.AcceptedAt = "", ""
	findings := Verify(r, false)
	if n := CountErrors(findings); n != 0 {
		t.Fatalf("unknown 且留痕空必须无 error（待修补 #21 核心），实际 %d 条：%+v", n, findings)
	}
	for _, f := range findings {
		if f.Level == "warn" && strings.Contains(f.Field, "accepted") {
			t.Fatalf("不要求给 unknown 加 warn（保持最小改动）：%+v", f)
		}
	}
}

func TestV11_Rule21_RevenueGatedRequiresTrace(t *testing.T) {
	r := good()
	r.License.Commercial = "revenue_gated"
	r.License.AcceptedBy, r.License.AcceptedAt = "", ""
	if n := CountErrors(Verify(r, false)); n != 1 {
		t.Fatalf("revenue_gated 且留痕空应恰好 1 条 error，实际 %d 条：%+v", n, Verify(r, false))
	}
	r.License.AcceptedBy, r.License.AcceptedAt = "laodao", "2026-09-11T00:00:00Z"
	if n := CountErrors(Verify(r, false)); n != 0 {
		t.Fatalf("补上真实留痕后应合格，实际 %d 条：%+v", n, Verify(r, false))
	}
}

func TestV12_Rule21_ChinesePlaceholderRejected(t *testing.T) {
	// 占位检查对任何状态都生效：commercial=yes 也不许用占位值。
	r := good()
	r.License.Commercial = "yes"
	r.License.AcceptedBy = "待定"
	r.License.AcceptedAt = "2026-09-11T00:00:00Z"
	findings := Verify(r, false)
	if n := CountErrors(findings); n != 1 {
		t.Fatalf("accepted_by=待定 应恰好 1 条 error，实际 %d 条：%+v", n, findings)
	}
	if !strings.Contains(findings[0].Detail, "占位") {
		t.Fatalf("错误文案必须提到占位：%+v", findings[0])
	}
}

// 占位词全集：大小写不敏感、两侧空白先 trim；真实值必须放行。
func TestV13_Rule21_PlaceholderValueTable(t *testing.T) {
	bad := []string{
		"unset", "UNSET", " pending ", "tbd", "TODO", "n/a", "NA",
		"placeholder", "unknown", "待定", "未定", "none", "null", "-",
	}
	for _, v := range bad {
		r := good()
		r.License.AcceptedBy = v
		r.License.AcceptedAt = "2026-09-11T00:00:00Z"
		if n := CountErrors(Verify(r, false)); n == 0 {
			t.Fatalf("占位值 %q 必须报 error（不许骗过门禁）", v)
		}
	}
	for _, v := range []string{"张三", "laodao", "alice@example.org"} {
		r := good()
		r.License.AcceptedBy = v
		r.License.AcceptedAt = "2026-09-11T00:00:00Z"
		if n := CountErrors(Verify(r, false)); n != 0 {
			t.Fatalf("真实留痕 %q 不该报 error，实际 %d 条：%+v", v, n, Verify(r, false))
		}
	}
}

// V5 自创能力标签 → error
func TestV5_UnknownCapability(t *testing.T) {
	r := good()
	r.Capabilities[0].Name = "super_vision"
	if n := CountErrors(Verify(r, false)); n == 0 {
		t.Fatal("取值表外的能力标签必须报 error")
	}
}

// V6 模板取值越界 → error
func TestV6_BadChatTemplate(t *testing.T) {
	r := good()
	r.EngineRecipes = map[string]EngineRecipe{"llama.cpp": {ChatTemplate: "guess"}}
	if n := CountErrors(Verify(r, false)); n == 0 {
		t.Fatal("chat_template 越界必须报 error")
	}
	// 三种合法写法都要过
	for _, ok := range []string{"from_gguf", "from_tokenizer", "inline:{{ .Prompt }}"} {
		r.EngineRecipes["llama.cpp"] = EngineRecipe{ChatTemplate: ok}
		if n := CountErrors(Verify(r, false)); n != 0 {
			t.Fatalf("合法模板 %q 不该报错", ok)
		}
	}
}

// V7 含未知字段 → 必须忽略而不报错（向后兼容，标准 §十）
func TestV7_UnknownFieldsIgnored(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rec.json")
	body := `{
	  "schema": "zerg.model.v1",
	  "id": "x",
	  "digest": "sha256:abcd",
	  "files": [{"role":"weights","name":"f.gguf","sha256":"ee","size":1}],
	  "license": {"spdx":"MIT","commercial":"yes"},
	  "some_future_field": {"nested": [1,2,3]},
	  "another_unknown": "hello"
	}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := Load(p)
	if err != nil {
		t.Fatalf("含未知字段必须能被读取，实际报错：%v", err)
	}
	if n := CountErrors(Verify(r, false)); n != 0 {
		t.Fatalf("含未知字段的合法记录不该报 error，实际 %d 条", n)
	}
}

// 额外：id 必须小写无空格；spdx=other 必须带 name+link；私有开关必须 ZERG_ 前缀
func TestExtraRules(t *testing.T) {
	r := good()
	r.ID = "Bad ID"
	if CountErrors(Verify(r, false)) == 0 {
		t.Fatal("id 含大写/空格必须报 error")
	}
	r = good()
	r.License.SPDX = "other"
	if CountErrors(Verify(r, false)) == 0 {
		t.Fatal("spdx=other 缺 license_name/link 必须报 error")
	}
	r = good()
	r.EngineRecipes = map[string]EngineRecipe{"vllm": {ExtraEnv: map[string]string{"MY_FLAG": "1"}}}
	if CountErrors(Verify(r, false)) == 0 {
		t.Fatal("私有开关未加 ZERG_ 前缀必须报 error")
	}
}
