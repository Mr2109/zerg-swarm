package modelreg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 待修补 #12：probe.license.v1（只认结构化来源；绝不默认 yes；不靠文案匹配猜）──────
//
// 反例优先：每个"能拿到"的用例旁边都有一个"必须如实说不知道"的反例。

// 许可证正文夹具：用各许可证的**法定固定措辞**（不是"看起来像"）。
const (
	mitLicenseText = `MIT License

Copyright (c) 2026 Nobody

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software.
`
	apacheLicenseText = `                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION
`
	gpl3LicenseText = `                    GNU GENERAL PUBLIC LICENSE
                       Version 3, 29 June 2007

 Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
`
	// 认不出的正文：既不是法定措辞，也不是任何标准许可证。
	unrecognizedLicenseText = `Do whatever you want with this model, we are nice people.
`
)

// writeGGUFIn 在 dir 里写一个自定义 KV 的 GGUF（dir 为空则用 t.TempDir()）。
func writeGGUFIn(t *testing.T, dir, name string, kvs ...[]byte) string {
	t.Helper()
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, makeGGUF(kvs...), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// writeFileIn 在 dir 里写一个附属文件（README / LICENSE 之类），返回路径。
func writeFileIn(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// baseKVs 是最小可用的 GGUF 键集（不含任何许可键）。
func baseKVs() [][]byte {
	return [][]byte{
		kv("general.architecture", ggufTypeString, gstr("llama")),
		kv("general.name", ggufTypeString, gstr("LicTest-8B")),
		kv("llama.context_length", ggufTypeUint32, le32(4096)),
	}
}

// ① GGUF 有许可键 → 填入，且 evidence 指向**键名**；commercial 仍恒为 unknown。
func TestLicenseProbeGGUFKeys(t *testing.T) {
	dir := t.TempDir()
	kvs := append(baseKVs(),
		kv("general.license", ggufTypeString, gstr("apache-2.0")),
		kv("general.license.name", ggufTypeString, gstr("Apache License 2.0")),
		kv("general.license.link", ggufTypeString, gstr("https://www.apache.org/licenses/LICENSE-2.0")),
	)
	p := writeGGUFIn(t, dir, "LicTest-8B-Q4_K_M.gguf", kvs...)

	meta, _, err := ProbeMetaGGUFFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lp := ProbeLicense(meta, dir)
	if lp.SPDX != "Apache-2.0" {
		t.Fatalf("应从 GGUF general.license 读到 Apache-2.0，实际 %q", lp.SPDX)
	}
	if lp.Source != LicenseSourceGGUFKey {
		t.Fatalf("来源应为 %s，实际 %s", LicenseSourceGGUFKey, lp.Source)
	}
	if lp.Name != "Apache License 2.0" || lp.Link != "https://www.apache.org/licenses/LICENSE-2.0" {
		t.Fatalf("名字/链接应如实照抄 GGUF：name=%q link=%q", lp.Name, lp.Link)
	}
	if lp.Commercial != "unknown" {
		t.Fatalf("商业态恒 unknown（绝不默认 yes），实际 %q", lp.Commercial)
	}
	for _, want := range []string{
		EvidenceLicense,
		`gguf_key: general.license="apache-2.0"`,
		`gguf_key: general.license.name="Apache License 2.0"`,
		`gguf_key: general.license.link="https://www.apache.org/licenses/LICENSE-2.0"`,
	} {
		if !strings.Contains(lp.Evidence, want) {
			t.Fatalf("evidence 必须指向键名：缺 %q\n%s", want, lp.Evidence)
		}
	}

	// 接进整条记录：license.evidence 落正文，verify 零 error。
	rec, _, err := Probe(ProbeOptions{Target: p})
	if err != nil {
		t.Fatal(err)
	}
	if rec.License.SPDX != "Apache-2.0" || !strings.Contains(rec.License.Evidence, "gguf_key: general.license") {
		t.Fatalf("记录里的许可块不对：%+v", rec.License)
	}
	if rec.License.Commercial != "unknown" {
		t.Fatalf("记录 commercial 必须 unknown：%q", rec.License.Commercial)
	}
	if n := CountErrors(Verify(rec, false)); n != 0 {
		t.Fatalf("探针产物不该有 error，实际 %d 条：%+v", n, Verify(rec, false))
	}
}

// ② 没有任何结构化来源 → unknown，且**不得**出现 yes；evidence 列出尝试过的来源。
func TestLicenseProbeNoStructuredSource(t *testing.T) {
	dir := t.TempDir()
	p := writeGGUFIn(t, dir, "NoLic-8B-Q4_K_M.gguf", baseKVs()...)

	rec, _, err := Probe(ProbeOptions{Target: p})
	if err != nil {
		t.Fatal(err)
	}
	if rec.License.SPDX != "unknown" {
		t.Fatalf("读不到结构化来源必须 spdx=unknown，实际 %q", rec.License.SPDX)
	}
	if rec.License.Commercial != "unknown" {
		t.Fatalf("commercial 必须 unknown，实际 %q", rec.License.Commercial)
	}
	if rec.License.Name != "" || rec.License.Link != "" {
		t.Fatalf("没有结构化来源时不许造名字/链接：%+v", rec.License)
	}
	ev := rec.License.Evidence
	if !strings.Contains(ev, FailNoLicenseSource) {
		t.Fatalf("evidence 应标 no_license_source，实际 %q", ev)
	}
	for _, want := range []string{"general.license", "general.license.name", "general.license.link", "LICENSE"} {
		if !strings.Contains(ev, want) {
			t.Fatalf("evidence 必须列出尝试过的来源（缺 %q）：%s", want, ev)
		}
	}
	// 硬规则：记录正文里**绝不**出现 commercial=yes（唯一合法 yes 的来源是人工/声明）。
	body := string(recordBody(t, rec))
	if strings.Contains(body, `"commercial": "yes"`) {
		t.Fatalf("探针产物绝不许出现 commercial=yes：\n%s", body)
	}
	if n := CountErrors(Verify(rec, false)); n != 0 {
		t.Fatalf("spdx=unknown 是合法取值，不该报 error：%+v", Verify(rec, false))
	}
	t.Logf("证据串（无结构化来源）：%s", ev)
}

// ③ 诱饵用例：同目录放一段"这看起来像 MIT"的自由文本 → 不得判成 MIT、不得填 spdx。
func TestLicenseProbeFreeTextDecoyIsNotEvidence(t *testing.T) {
	dir := t.TempDir()
	p := writeGGUFIn(t, dir, "Decoy-8B-Q4_K_M.gguf", baseKVs()...)
	// 两份诱饵：README 与模型卡。它们都**不是**明确命名的许可证文件，更不是结构化信号。
	writeFileIn(t, dir, "README.md", `# Decoy-8B

License: MIT
这看起来像 MIT（自由度很高，可以商用），随你怎么用。
`)
	writeFileIn(t, dir, "MODEL_CARD.md", `---
license: mit
---
This model card says the license is MIT and it looks very permissive.
`)

	rec, _, err := Probe(ProbeOptions{Target: p})
	if err != nil {
		t.Fatal(err)
	}
	if rec.License.SPDX != "unknown" {
		t.Fatalf("自由文本不是证据：必须 unknown，实际 %q", rec.License.SPDX)
	}
	if rec.License.Commercial != "unknown" {
		t.Fatalf("自由文本不是证据：commercial 必须 unknown，实际 %q", rec.License.Commercial)
	}
	body := string(recordBody(t, rec))
	if strings.Contains(body, "MIT") {
		t.Fatalf("诱饵文案不得渗进记录（不得出现 MIT）：\n%s", body)
	}
	if strings.Contains(strings.ToLower(body), `"commercial": "yes"`) {
		t.Fatalf("诱饵不得promote 成商用 yes：\n%s", body)
	}
	if !strings.Contains(body, `"spdx": "unknown"`) {
		t.Fatalf("记录里应明写 spdx=unknown（缺 = 未知，不许省成空白）：\n%s", body)
	}
}

// ④ 同目录有**明确命名**的 LICENSE 文件 → 按文件内容（正文法定措辞）判定并留证据。
func TestLicenseProbeSameDirLicenseFileByContent(t *testing.T) {
	cases := []struct {
		file    string
		content string
		want    string
	}{
		{"LICENSE", apacheLicenseText, "Apache-2.0"},
		{"LICENSE.txt", mitLicenseText, "MIT"},
		{"LICENCE", gpl3LicenseText, "GPL-3.0"},
	}
	for _, tc := range cases {
		t.Run(tc.file+"→"+tc.want, func(t *testing.T) {
			dir := t.TempDir()
			p := writeGGUFIn(t, dir, "FileLic-8B-Q4_K_M.gguf", baseKVs()...)
			writeFileIn(t, dir, tc.file, tc.content)

			rec, _, err := Probe(ProbeOptions{Target: p})
			if err != nil {
				t.Fatal(err)
			}
			if rec.License.SPDX != tc.want {
				t.Fatalf("应按 LICENSE 正文判为 %s，实际 %q（evidence=%s）", tc.want, rec.License.SPDX, rec.License.Evidence)
			}
			if !strings.Contains(rec.License.Evidence, LicenseSourceSameDirFile+": "+tc.file) {
				t.Fatalf("evidence 必须指向该文件：%s", rec.License.Evidence)
			}
			if !strings.Contains(rec.License.Evidence, "sha256=") {
				t.Fatalf("evidence 必须带内容摘要（内容锚）：%s", rec.License.Evidence)
			}
			if rec.License.Commercial != "unknown" {
				t.Fatalf("commercial 恒 unknown（绝不默认 yes），实际 %q", rec.License.Commercial)
			}
		})
	}
}

// ④反例 1：文件存在但正文认不出 → unknown（绝不许凑一个 SPDX），evidence 写明"未识别"。
func TestLicenseProbeUnrecognizedLicenseFileStaysUnknown(t *testing.T) {
	dir := t.TempDir()
	p := writeGGUFIn(t, dir, "WeirdLic-8B-Q4_K_M.gguf", baseKVs()...)
	writeFileIn(t, dir, "LICENSE", unrecognizedLicenseText)

	rec, _, err := Probe(ProbeOptions{Target: p})
	if err != nil {
		t.Fatal(err)
	}
	if rec.License.SPDX != "unknown" {
		t.Fatalf("认不出的 LICENSE 必须 unknown（不许猜），实际 %q", rec.License.SPDX)
	}
	if !strings.Contains(rec.License.Evidence, "spdx=unrecognized") {
		t.Fatalf("evidence 必须写明文件在但没认出：%s", rec.License.Evidence)
	}
}

// ④ 反例 2：文件名里明写的 SPDX（LICENSE-MIT）→ 按**窄口径**文件名规则判定。
// 窄口径 = 只认整 token，避免 permit/unlicensed 之类误命中。
func TestLicenseProbeSameDirLicenseFileByName(t *testing.T) {
	cases := []struct {
		file string
		want string
	}{
		{"LICENSE-MIT", "MIT"},
		{"LICENSE-APACHE-2.0", "Apache-2.0"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			dir := t.TempDir()
			p := writeGGUFIn(t, dir, "NameLic-8B-Q4_K_M.gguf", baseKVs()...)
			writeFileIn(t, dir, tc.file, unrecognizedLicenseText)
			rec, _, err := Probe(ProbeOptions{Target: p})
			if err != nil {
				t.Fatal(err)
			}
			if rec.License.SPDX != tc.want {
				t.Fatalf("按文件名应判为 %s，实际 %q", tc.want, rec.License.SPDX)
			}
		})
	}
}

// ④ 反例 3：**不是**明确命名的许可证文件（LICENSE-MIT 之外的花名）不参与判定。
// permitted-uses.txt 含 "mit" 子串 —— 窄口径必须不被它骗到。
func TestLicenseProbeNonLicenseFileNameIgnored(t *testing.T) {
	dir := t.TempDir()
	p := writeGGUFIn(t, dir, "Substr-8B-Q4_K_M.gguf", baseKVs()...)
	writeFileIn(t, dir, "permit-mit-like.txt", mitLicenseText)

	rec, _, err := Probe(ProbeOptions{Target: p})
	if err != nil {
		t.Fatal(err)
	}
	if rec.License.SPDX != "unknown" {
		t.Fatalf("非明确命名的文件不得参与判定，实际 %q", rec.License.SPDX)
	}
}

// ⑤ 两个结构化来源互相矛盾 → 如实标冲突（不静默挑一个），权重自带声明优先。
func TestLicenseProbeConflictingSourcesFlagged(t *testing.T) {
	dir := t.TempDir()
	kvs := append(baseKVs(), kv("general.license", ggufTypeString, gstr("apache-2.0")))
	p := writeGGUFIn(t, dir, "Conflict-8B-Q4_K_M.gguf", kvs...)
	writeFileIn(t, dir, "LICENSE", mitLicenseText)

	rec, _, err := Probe(ProbeOptions{Target: p})
	if err != nil {
		t.Fatal(err)
	}
	if rec.License.SPDX != "Apache-2.0" {
		t.Fatalf("权重自带声明优先，应 Apache-2.0，实际 %q", rec.License.SPDX)
	}
	if !strings.Contains(rec.License.Evidence, "冲突") || !strings.Contains(rec.License.Evidence, "MIT") {
		t.Fatalf("矛盾必须留痕（不能静默挑一个）：%s", rec.License.Evidence)
	}
}

// ⑤ 商业性：即便 GGUF 明写 MIT（最宽松的许可之一），探测也**不下**商业结论（绝不默认 yes）。
func TestLicenseProbeNeverRendersCommercialVerdict(t *testing.T) {
	dir := t.TempDir()
	kvs := append(baseKVs(), kv("general.license", ggufTypeString, gstr("MIT")))
	p := writeGGUFIn(t, dir, "Mit-8B-Q4_K_M.gguf", kvs...)
	writeFileIn(t, dir, "LICENSE", mitLicenseText)

	rec, _, err := Probe(ProbeOptions{Target: p})
	if err != nil {
		t.Fatal(err)
	}
	if rec.License.SPDX != "MIT" {
		t.Fatalf("应读到 MIT，实际 %q", rec.License.SPDX)
	}
	if rec.License.Commercial != "unknown" {
		t.Fatalf("商业态是法律判断，探测不得下结论（必须 unknown），实际 %q", rec.License.Commercial)
	}
}

// ⑥ 确定性：同一批建材换个目录（LICENSE 内容/文件名相同）→ 正文逐字节相同。
// 目录属"存放位置"，不得渗进正文（证据只写文件名 + 内容摘要）。
func TestLicenseProbeEvidenceIsNotPathDependent(t *testing.T) {
	kvs := append(baseKVs(), kv("general.license", ggufTypeString, gstr("apache-2.0")))
	build := func(dir string) *Record {
		t.Helper()
		p := writeGGUFIn(t, dir, "SameMat-8B-Q4_K_M.gguf", kvs...)
		writeFileIn(t, dir, "LICENSE", apacheLicenseText)
		rec, _, err := Probe(ProbeOptions{Target: p})
		if err != nil {
			t.Fatal(err)
		}
		return rec
	}
	a := build(t.TempDir())
	b := build(filepath.Join(t.TempDir(), "deep", "nested", "place"))
	bA, bB := recordBody(t, a), recordBody(t, b)
	if bodyHash(bA) != bodyHash(bB) {
		t.Fatalf("同一批建材换个目录，正文必须逐字节相同\n  A=%s\n  B=%s\n  A正文=%s\n  B正文=%s",
			bodyHash(bA), bodyHash(bB), bA, bB)
	}
	if a.Digest != b.Digest {
		t.Fatalf("digest 也不该随目录变：%s vs %s", a.Digest, b.Digest)
	}
}

// ⑦ 记录里的 license 块必须能被序列化/反序列化往返（旧读者忽略新字段也不报错）。
func TestLicenseEvidenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	kvs := append(baseKVs(), kv("general.license", ggufTypeString, gstr("apache-2.0")))
	p := writeGGUFIn(t, dir, "RT-8B-Q4_K_M.gguf", kvs...)
	rec, _, err := Probe(ProbeOptions{Target: p})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	var back Record
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.License != rec.License {
		t.Fatalf("license 块往返不一致：%+v vs %+v", back.License, rec.License)
	}
	// 未知字段（旧读者视角）不许报错：加一个未来字段再解一遍。
	var withFuture struct {
		License map[string]interface{} `json:"license"`
	}
	if err := json.Unmarshal([]byte(`{"license":{"spdx":"MIT","commercial":"unknown","future_field":1}}`), &withFuture); err != nil {
		t.Fatalf("未知字段必须被忽略而不报错（标准 §十）：%v", err)
	}
}
