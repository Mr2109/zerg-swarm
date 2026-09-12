package modelreg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 待修补 #16：血缘声明 parent / base_model ────────────────────────────────────
//
// 反例优先：合法值 → 通过；占位值/形状不对 → error；旧记录（无字段）→ 仍通过；
// 同一批建材同一命令跑两次 → 正文逐字节相同；且**绝不**由 store 按写入顺序推断。

const (
	validParentVersion = "sha256-0123456789ab"
	validParentDigest  = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	validBaseModel     = "gemma-4-26b-a4b-it"
)

// ① 合法 parent/base → verify 零 error；字段真的进正文。
func TestLineageExplicitParentAndBaseAccepted(t *testing.T) {
	dir := t.TempDir()
	p := writeGGUFIn(t, dir, "Lineage-8B-Q4_K_M.gguf", baseKVs()...)

	rec, _, err := Probe(ProbeOptions{Target: p, Parent: validParentVersion, BaseModel: validBaseModel})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Parent != validParentVersion {
		t.Fatalf("parent 应照抄显式入参，实际 %q", rec.Parent)
	}
	if rec.BaseModel != validBaseModel {
		t.Fatalf("base_model 应照抄显式入参，实际 %q", rec.BaseModel)
	}
	if n := CountErrors(Verify(rec, false)); n != 0 {
		t.Fatalf("合法血缘不该报 error：%+v", Verify(rec, false))
	}
	body := string(recordBody(t, rec))
	if !strings.Contains(body, `"parent": "`+validParentVersion+`"`) ||
		!strings.Contains(body, `"base_model": "`+validBaseModel+`"`) {
		t.Fatalf("血缘必须进记录正文：\n%s", body)
	}
	if !strings.Contains(rec.Notes, "血缘声明") {
		t.Fatalf("notes 应说明血缘由调用方显式传入：%s", rec.Notes)
	}
	// digest 形态的 parent 也合法（两种形状都能引用"上一版"）。
	rec2, _, err := Probe(ProbeOptions{Target: p, Parent: validParentDigest})
	if err != nil {
		t.Fatal(err)
	}
	if n := CountErrors(Verify(rec2, false)); n != 0 {
		t.Fatalf("parent=digest 形态也应合法：%+v", Verify(rec2, false))
	}
}

// ② 占位值 → error（parent 与 base_model 都算）。
func TestLineagePlaceholdersRejected(t *testing.T) {
	parentCases := []string{"unknown", "unset", "pending", "tbd", "todo", "n/a", "na", "placeholder", "none", "null", "-", "待定", "未定", "self", "base", "parent", "latest", "previous", "SELF"}
	for _, v := range parentCases {
		rec := goodRecord("ph-model", "a")
		rec.Parent = v
		findings := Verify(rec, false)
		if CountErrors(findings) == 0 {
			t.Fatalf("parent 占位值 %q 必须报 error", v)
		}
		if !hasFindingField(findings, "parent") {
			t.Fatalf("parent 占位值 %q 的 error 应挂在 parent 字段：%+v", v, findings)
		}
	}
	baseCases := []string{"unknown", "tbd", "none", "-", "self", "parent", "UNKNOWN"}
	for _, v := range baseCases {
		rec := goodRecord("ph-model", "a")
		rec.BaseModel = v
		findings := Verify(rec, false)
		if CountErrors(findings) == 0 {
			t.Fatalf("base_model 占位值 %q 必须报 error", v)
		}
		if !hasFindingField(findings, "base_model") {
			t.Fatalf("base_model 占位值 %q 的 error 应挂在 base_model 字段：%+v", v, findings)
		}
	}
}

// ③ parent 形状不对 / base_model 形状不对 → error。
func TestLineageBadShapesRejected(t *testing.T) {
	badParents := []string{
		"sha256-",                           // 空摘要
		"sha256:0123",                       // digest 位数不足
		"sha256:" + strings.Repeat("0", 63), // 少一位
		"sha256:" + strings.Repeat("0", 65), // 多一位
		"sha256-" + strings.Repeat("z", 12), // 非十六进制
		"v2",                                // 既不是 version 也不是 digest
		"sha1-0123456789ab",                 // 前缀不对
		"0123456789ab",                      // 缺前缀
		"sha256-0123456789ab / v3",          // 带修饰
		"sha256:x",                          // 非十六进制
	}
	for _, v := range badParents {
		rec := goodRecord("shape-model", "a")
		rec.Parent = v
		findings := Verify(rec, false)
		if CountErrors(findings) == 0 {
			t.Fatalf("parent 形状错误 %q 必须报 error", v)
		}
		if !hasFindingField(findings, "parent") {
			t.Fatalf("parent 形状错误的 error 应挂在 parent：%+v", findings)
		}
	}
	// 合法形状（大写十六进制也算形状合法——形状规则只看"是不是摘要/版本"）。
	okParents := []string{validParentVersion, validParentDigest, "sha256-0123456789AB"}
	for _, v := range okParents {
		rec := goodRecord("shape-model", "a")
		rec.Parent = v
		if n := CountErrors(Verify(rec, false)); n != 0 {
			t.Fatalf("合法 parent 形状 %q 不该报 error：%+v", v, Verify(rec, false))
		}
	}

	// base_model 与 id 用**同一条形状规则**（标准 §三：小写、不含空格）——不额外收紧，
	// 避免出现"id 能过、base_model 不能过"这种自相矛盾的门禁。
	badBases := []string{"Gemma 4 26B", "UPPER-CASE", "has	tab", "a b c"}
	for _, v := range badBases {
		rec := goodRecord("shape-model", "a")
		rec.BaseModel = v
		findings := Verify(rec, false)
		if CountErrors(findings) == 0 {
			t.Fatalf("base_model 形状错误 %q 必须报 error", v)
		}
		if !hasFindingField(findings, "base_model") {
			t.Fatalf("base_model 形状错误的 error 应挂在 base_model：%+v", findings)
		}
	}
	okBases := []string{validBaseModel, "gemma-4-26b-a4b-it", "example-35b-v2"}
	for _, v := range okBases {
		rec := goodRecord("shape-model", "a")
		rec.BaseModel = v
		if n := CountErrors(Verify(rec, false)); n != 0 {
			t.Fatalf("合法 base_model 形状 %q 不该报 error：%+v", v, Verify(rec, false))
		}
	}
}

// ④ parent 自指（指向本记录的 version/digest）→ error：填了等于没填。
func TestLineageSelfReferenceRejected(t *testing.T) {
	rec := goodRecord("self-model", "a")
	rec.Parent = rec.Digest
	if CountErrors(Verify(rec, false)) == 0 {
		t.Fatal("parent 指向自己的 digest 必须报 error")
	}
	rec2 := goodRecord("self-model", "a")
	rec2.Parent = VersionOf(rec2)
	if CountErrors(Verify(rec2, false)) == 0 {
		t.Fatal("parent 指向自己的 version 必须报 error")
	}
}

// ⑤ 向后兼容：旧记录（完全没有这两个字段）照样合法，且序列化后不凭空长出键。
func TestLineageBackwardCompatibleOldRecord(t *testing.T) {
	// 一份"加字段之前"的记录正文（手写 JSON，模拟目录里已有的旧记录）。
	old := []byte(`{
  "schema": "zerg.model.v1",
  "id": "old-model-8b",
  "digest": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "files": [{"role": "weights", "name": "old-Q4_K_M.gguf", "sha256": "aaaa", "size": 5}],
  "license": {"spdx": "Apache-2.0", "commercial": "unknown"},
  "source_url": "https://example.org/old",
  "state": "known"
}
`)
	p := filepath.Join(t.TempDir(), "old-model-8b.json")
	if err := os.WriteFile(p, old, 0o600); err != nil {
		t.Fatal(err)
	}
	rec, err := Load(p)
	if err != nil {
		t.Fatalf("旧记录必须能被读进来（未知/缺字段不报错）：%v", err)
	}
	if rec.Parent != "" || rec.BaseModel != "" {
		t.Fatalf("旧的记录不该凭空有血缘：%q/%q", rec.Parent, rec.BaseModel)
	}
	if n := CountErrors(Verify(rec, false)); n != 0 {
		t.Fatalf("旧记录必须依然通过（可选字段，向后兼容）：%+v", Verify(rec, false))
	}
	body, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "parent") || strings.Contains(string(body), "base_model") {
		t.Fatalf("没有血缘时不得凭空长出键（omitempty）：%s", body)
	}
}

// ⑥ 确定性（本条目最关键的保证）：同一批建材 + 同一条命令跑两次 → 正文逐字节相同；
// 换个 parent → 正文才不同（证明字段真的生效，不是被吞掉）。
func TestLineageDeterministicBodyBytes(t *testing.T) {
	dir := t.TempDir()
	p := writeGGUFIn(t, dir, "Det-8B-Q4_K_M.gguf", baseKVs()...)

	run := func(parent, base string) *Record {
		t.Helper()
		rec, _, err := Probe(ProbeOptions{Target: p, Parent: parent, BaseModel: base})
		if err != nil {
			t.Fatal(err)
		}
		return rec
	}
	a1 := run(validParentVersion, validBaseModel)
	a2 := run(validParentVersion, validBaseModel)
	b1, b2 := recordBody(t, a1), recordBody(t, a2)
	if bodyHash(b1) != bodyHash(b2) {
		t.Fatalf("同一条命令两次探测，正文必须逐字节相同\n  1=%s\n  2=%s", bodyHash(b1), bodyHash(b2))
	}
	// 反例：换了 parent → 正文必须不同（否则说明字段没进正文）
	other := run("sha256-ffffffffffff", validBaseModel)
	if bodyHash(recordBody(t, other)) == bodyHash(b1) {
		t.Fatal("换了 parent，正文必须随之不同（说明血缘确实进了正文）")
	}
	// 反例：不给血缘 → 正文里没有 parent/base_model（可选字段）
	none := run("", "")
	noBody := string(recordBody(t, none))
	if strings.Contains(noBody, `"parent"`) || strings.Contains(noBody, `"base_model"`) {
		t.Fatalf("没传血缘时不得凭空写进正文：\n%s", noBody)
	}
}

// ⑦ 反例：**不许**从 GGUF 里现成的 general.base_model.* 自动推断（那是显示名/URL，不是注册 id；
// 自动填 = 把显示名当 id 猜，正是"禁造假值"要堵的洞）。
func TestLineageNotAutoInferredFromGGUF(t *testing.T) {
	dir := t.TempDir()
	kvs := append(baseKVs(),
		kv("general.base_model.count", ggufTypeUint32, le32(1)),
		kv("general.base_model.0.name", ggufTypeString, gstr("Gemma 4 26B A4B It")),
		kv("general.base_model.0.organization", ggufTypeString, gstr("Google")),
		kv("general.base_model.0.repo_url", ggufTypeString, gstr("https://huggingface.co/google/gemma-4-26B-A4B-it")),
	)
	p := writeGGUFIn(t, dir, "BaseHint-8B-Q4_K_M.gguf", kvs...)

	rec, _, err := Probe(ProbeOptions{Target: p})
	if err != nil {
		t.Fatal(err)
	}
	if rec.BaseModel != "" || rec.Parent != "" {
		t.Fatalf("血缘只能由调用方显式传入，不得自动推断：parent=%q base_model=%q", rec.Parent, rec.BaseModel)
	}
	body := string(recordBody(t, rec))
	if strings.Contains(body, "base_model") || strings.Contains(body, "gemma-4-26B-A4B-it") {
		t.Fatalf("GGUF 里的 general.base_model.* 不得渗进正文：\n%s", body)
	}
}

// ⑧ 反例：**store 绝不按写入顺序推断 parent**。
// 同一 id 连写两条（建材不同 = 不同版本），两条落盘正文里都不许出现 parent——
// 若由 store 推断，第二条就会被写成"parent=第一条"，而那条值不是建材的一部分：
// 换个入库顺序/换台机器就会得到不同的字节，破坏「同一批建材 → 正文逐字节相同」。
func TestStoreNeverInfersParent(t *testing.T) {
	manifests := t.TempDir()
	st := NewStoreAtManifests(manifests)

	v1 := goodRecord("ordered-model", "a")
	v2 := goodRecord("ordered-model", "b")
	if v1.Digest == v2.Digest {
		t.Fatal("夹具应当是两个不同版本（digest 不同）")
	}
	for _, rec := range []*Record{v1, v2} {
		res, err := st.Put(rec, false)
		if err != nil {
			t.Fatalf("合法记录应能入目录：%v（%+v）", err, res.Findings)
		}
	}
	// 反序再写一遍（模拟"另一台机器/另一种顺序"）：两次结果必须一致——store 不因顺序改内容。
	manifests2 := t.TempDir()
	st2 := NewStoreAtManifests(manifests2)
	for _, rec := range []*Record{v2, v1} {
		if _, err := st2.Put(rec, false); err != nil {
			t.Fatalf("反序写入也应通过：%v", err)
		}
	}
	for _, pair := range []struct{ a, b string }{{manifests, manifests2}} {
		for _, rec := range []*Record{v1, v2} {
			pA := filepath.Join(pair.a, rec.ID, VersionOf(rec)+".json")
			pB := filepath.Join(pair.b, rec.ID, VersionOf(rec)+".json")
			bA, err := os.ReadFile(pA)
			if err != nil {
				t.Fatal(err)
			}
			bB, err := os.ReadFile(pB)
			if err != nil {
				t.Fatal(err)
			}
			if bodyHash(bA) != bodyHash(bB) {
				t.Fatalf("同一建材、不同写入顺序，落盘正文必须逐字节相同\n  顺序A=%s\n  顺序B=%s", bA, bB)
			}
			if strings.Contains(string(bA), `"parent"`) || strings.Contains(string(bA), `"base_model"`) {
				t.Fatalf("store 不许推断血缘，落盘正文里不得出现 parent/base_model：\n%s", bA)
			}
		}
	}
}

// hasFindingField 判定 findings 里是否有挂在某字段上的结论。
func hasFindingField(fs []Finding, field string) bool {
	for _, f := range fs {
		if f.Field == field {
			return true
		}
	}
	return false
}
