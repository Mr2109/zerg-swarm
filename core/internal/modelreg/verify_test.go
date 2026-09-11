package modelreg

import (
	"os"
	"path/filepath"
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

// V4 非商用但无留痕 → error
func TestV4_NoLicenseTrace(t *testing.T) {
	r := good()
	r.License.Commercial = "no"
	if n := CountErrors(Verify(r, false)); n == 0 {
		t.Fatal("非商用无 accepted_by/at 必须报 error")
	}
	r.License.AcceptedBy, r.License.AcceptedAt = "laodao", "2026-09-11"
	if n := CountErrors(Verify(r, false)); n != 0 {
		t.Fatal("补上留痕后应合格")
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
