// check_model_size_anchor_test.go —— `check_model_size_anchor.go` 的定向单测（成对负控 · ✗ 禁恒绿）。
//
// 覆盖面（与件头「已对拍面 / 未覆盖面」对齐）：
//
//	① 自检十四档全过（正控 rc=0）· ② 锚判据六档（good/bad/badd/skip/none/block）逐档退码 ·
//	③ 入口绿件（17 行 = 棘轮）rc=0 · ④ 漂移**点名到字段与两侧值**（负控锚点：哑化后必红）·
//	⑤ `--json` 空值三态（SKIP ⇒ anchor/actual 数值 vs null · findings `[]` 不吞键 · kind 在场）·
//	⑥ CLI 面（-h rc=0 · --list-rules rc=0 · 未知旗标 rc=2 · 缺值 rc=2 · 缩写唯一前缀）·
//	⑦ 判据 E 成对负控（假 registry_id ⇒ 1 · 棘轮短 ⇒ 1 · 记录目录不在 ⇒ 2 · dangling ⇒ 1）·
//	⑧ A 类对齐：自检后临时目录同前缀计数 delta=0（哑化 `defer os.RemoveAll` ⇒ 本条必红）。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cmsaWriteFixture —— 写一枚夹具件，返回其路径。
func cmsaWriteFixture(t *testing.T, dir, name, text string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatalf("写夹具失败 %s: %v", p, err)
	}
	return p
}

// cmsaRunMain —— 进程内复用入口，取 (rc, stdout, stderr)。
func cmsaRunMain(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := checkModelSizeAnchorGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// cmsaGreenFixture —— 造「17 行都映射 R1 且锚都对」的绿件 + 真源 + 记录目录。
func cmsaGreenFixture(t *testing.T, n int) (fleet, idmap, regdir string) {
	t.Helper()
	d := t.TempDir()
	real := cmsaWriteFixture(t, d, "w.gguf", strings.Repeat("x", 1234))
	var b strings.Builder
	b.WriteString("models:\n  M:\n")
	for i := 0; i < n; i++ {
		b.WriteString("    - { host: Mr2109, file: " + real + ", mem_gb: 1, size_bytes: 1234 }\n")
	}
	fleet = cmsaWriteFixture(t, d, "fleet.yaml", b.String())
	idmap = cmsaWriteFixture(t, d, "idmap.json",
		`{"schema":"z/t","entries":[{"registry_id":"R1","fleet_ids":["M"]}]}`)
	manifests := filepath.Join(d, "manifests")
	if err := os.MkdirAll(filepath.Join(manifests, "R1"), 0o755); err != nil {
		t.Fatal(err)
	}
	return fleet, idmap, manifests
}

// TestCheckModelSizeAnchor_SelfTestAllPass —— ① 自检十四档（正控：全过 ⇒ rc=0）。
func TestCheckModelSizeAnchor_SelfTestAllPass(t *testing.T) {
	var so, se strings.Builder
	rc := cmsaSelfTest(&so, &se)
	out := so.String()
	if rc != 0 {
		t.Fatalf("自检应 rc=0，实得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "自检 全绿") {
		t.Fatalf("自检结论行不符，实得尾部：\n%s", out)
	}
	for _, want := range []string{
		"wiring/good", "wiring/false-registry-id", "wiring/ratchet-bump",
		"wiring/dangling-fleet-id", "wiring/no-registry-dir",
		"proc/good(17行=棘轮)", "proc/false-registry-id", "proc/ratchet-short(16<17)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("自检输出缺档 %q\n%s", want, out)
		}
	}
}

// TestCheckModelSizeAnchor_AnchorVerdict —— ② 锚判据六档逐档退码（同 py self_test cases）。
func TestCheckModelSizeAnchor_AnchorVerdict(t *testing.T) {
	d := t.TempDir()
	real := cmsaWriteFixture(t, d, "w.gguf", strings.Repeat("x", 1234))
	base := "models:\n  m1:\n    - { host: Mr2109, file: " + real + ", mem_gb: 1, %s }\n"
	with := func(anchor string) string { return strings.Replace(base, "%s", anchor, 1) }
	cases := []struct {
		name  string
		fleet string
		want  int
	}{
		{"good", with("size_bytes: 1234"), 0},
		{"bad", with("size_bytes: 9999"), 1},
		{"badd", with("size_bytes: abc"), 1},
		{"skip", "models:\n  m1:\n    - { host: node-a, file: /nope/x.gguf, mem_gb: 1, size_bytes: 42 }\n", 2},
		{"none", with("ctx_window: 8"), 2},
		{"block", "models:\n  m2:\n    - host: Mr2109\n      file: " + real + "\n      size_bytes: 1234\n", 0},
	}
	for _, c := range cases {
		rc := cmsaVerdict(cmsaCheck(cmsaParseModels(c.fleet), ""))
		if rc != c.want {
			t.Errorf("档 %q：期望 rc=%d 实得 rc=%d", c.name, c.want, rc)
		}
	}
}

// TestCheckModelSizeAnchor_GreenFixtureEntry —— ③ 入口绿件（17 行 = 棘轮）rc=0。
func TestCheckModelSizeAnchor_GreenFixtureEntry(t *testing.T) {
	fleet, idmap, regdir := cmsaGreenFixture(t, 17)
	rc, out, _ := cmsaRunMain("--fleet", fleet, "--idmap", idmap, "--registry-dir", regdir)
	if rc != 0 {
		t.Fatalf("绿件应 rc=0，实得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "PASS：本机可读的锚全对 ＋ 接线全过（verdict_scope=本机）") {
		t.Fatalf("判词缺 PASS 行：\n%s", out)
	}
}

// TestCheckModelSizeAnchor_DriftNamesField —— ④ 漂移必须**点名到字段与两侧值**（负控锚点）。
// 哑化 `actual == anchor` 的分支（把相等判成漂移、或把 `!=` 抹平）⇒ 本条**必红**。
func TestCheckModelSizeAnchor_DriftNamesField(t *testing.T) {
	fleet, idmap, regdir := cmsaGreenFixture(t, 17)
	raw, err := os.ReadFile(fleet)
	if err != nil {
		t.Fatal(err)
	}
	drift := strings.Replace(string(raw), "size_bytes: 1234", "size_bytes: 9999", 1)
	driftFleet := cmsaWriteFixture(t, filepath.Dir(fleet), "drift.yaml", drift)
	rc, out, _ := cmsaRunMain("--fleet", driftFleet, "--idmap", idmap, "--registry-dir", regdir)
	if rc != 1 {
		t.Fatalf("漂移应 rc=1，实得 %d\n%s", rc, out)
	}
	want := "锚=9999 ≠ 现读 1234"
	if !strings.Contains(out, want) {
		t.Fatalf("漂移明细未点名到字段：缺 %q\n实得：\n%s", want, out)
	}
}

// TestCheckModelSizeAnchor_JSONNilVsEmpty —— ⑤ `--json` 空值三态：SKIP ⇒ actual null ·
// findings `[]`（不吞键）· kind 在场。哑化「空集合显式置空」⇒ 本条**必红**。
func TestCheckModelSizeAnchor_JSONNilVsEmpty(t *testing.T) {
	fleet, idmap, regdir := cmsaGreenFixture(t, 17)
	raw, err := os.ReadFile(fleet)
	if err != nil {
		t.Fatal(err)
	}
	extra := string(raw) + "    - { host: node-a, file: /nope/x.gguf, mem_gb: 1, size_bytes: 42 }\n"
	_ = cmsaWriteFixture(t, filepath.Dir(fleet), "mixed.yaml", extra)
	rc, out, _ := cmsaRunMain("--fleet", filepath.Join(filepath.Dir(fleet), "mixed.yaml"),
		"--idmap", idmap, "--registry-dir", regdir, "--json")
	if rc != 0 {
		t.Fatalf("混合件应 rc=0，实得 %d\n%s", rc, out)
	}
	for _, want := range []string{`"actual": null`, `"findings": []`, `"kind": "SKIP"`, `"verdict_scope": "本机"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("JSON 缺 %q\n实得：\n%s", want, out)
		}
	}
}

// TestCheckModelSizeAnchor_CLIFaces —— ⑥ CLI 面。
func TestCheckModelSizeAnchor_CLIFaces(t *testing.T) {
	rc, out, _ := cmsaRunMain("-h")
	if rc != 0 || !strings.Contains(out, "optional arguments:") {
		t.Fatalf("-h 应 rc=0 且含帮助正文，实得 rc=%d\n%s", rc, out)
	}
	rc, out, _ = cmsaRunMain("--list-rules")
	if rc != 0 || !strings.Contains(out, "退码：0 本机层全对＋接线全过") {
		t.Fatalf("--list-rules 应 rc=0 且含正文，实得 rc=%d\n%s", rc, out)
	}
	rc, _, errOut := cmsaRunMain("--nope")
	if rc != 2 || !strings.Contains(errOut, "unrecognized arguments: --nope") {
		t.Fatalf("未知旗标应 rc=2，实得 rc=%d stderr=%q", rc, errOut)
	}
	rc, _, errOut = cmsaRunMain("--fleet")
	if rc != 2 || !strings.Contains(errOut, "argument --fleet: expected one argument") {
		t.Fatalf("缺值应 rc=2，实得 rc=%d stderr=%q", rc, errOut)
	}
	rc, _, _ = cmsaRunMain("--self-test")
	if rc != 0 {
		t.Fatalf("--self-test 应 rc=0，实得 %d", rc)
	}
}

// TestCheckModelSizeAnchor_WiringNegative —— ⑦ 判据 E 成对负控（内部分析函数层 + 记录目录不在）。
func TestCheckModelSizeAnchor_WiringNegative(t *testing.T) {
	models := cmsaParseModels("models:\n  M:\n    - { host: Mr2109, file: /x.gguf }\n    - { host: node-a, file: /y.gguf }\n")
	regIDs := map[string]bool{"R1": true}
	okF2r := map[string]string{"M": "R1"}

	rc, _, n := cmsaCheckRegistryWiring(models, okF2r, regIDs, true, 2, "/reg")
	if rc != 0 || n != 2 {
		t.Fatalf("正控：期望 rc=0 n_with=2，实得 rc=%d n=%d", rc, n)
	}
	// 负控①：假 registry_id（R2 ∉ 记录集 {R1}）⇒ 必红
	if rc, _, _ = cmsaCheckRegistryWiring(models, map[string]string{"M": "R2"}, regIDs, true, 2, "/reg"); rc != 1 {
		t.Fatalf("假 registry_id 应 rc=1，实得 %d", rc)
	}
	// 负控②：棘轮 +1（行数不足）⇒ 必红
	if rc, _, _ = cmsaCheckRegistryWiring(models, okF2r, regIDs, true, 3, "/reg"); rc != 1 {
		t.Fatalf("棘轮短应 rc=1，实得 %d", rc)
	}
	// 负控③：记录目录不在 ⇒ 判不了 rc=2（findings 点名读不到）
	rc, f, _ := cmsaCheckRegistryWiring(models, okF2r, nil, false, 2, "/reg")
	if rc != 2 || len(f) == 0 {
		t.Fatalf("记录目录不在应 rc=2 且有 findings，实得 rc=%d findings=%v", rc, f)
	}
	// 负控④：E①' dangling fleet_id（GHOST ∉ 模型册）⇒ 必红
	if rc, _, _ = cmsaCheckRegistryWiring(models, map[string]string{"GHOST": "R1"}, regIDs, true, 0, "/reg"); rc != 1 {
		t.Fatalf("dangling fleet_id 应 rc=1，实得 %d", rc)
	}
}

// TestCheckModelSizeAnchor_TempRecycle —— ⑧ A 类对齐：自检后临时目录 delta=0。
// 哑化（删掉 cmsaSelfTest / cmsaWiringSelfTest 里的 `defer os.RemoveAll`）⇒ 本条**必红**。
func TestCheckModelSizeAnchor_TempRecycle(t *testing.T) {
	d := t.TempDir()
	t.Setenv("TMPDIR", d)
	var so, se strings.Builder
	if rc := cmsaSelfTest(&so, &se); rc != 0 {
		t.Fatalf("自检应 rc=0，实得 %d\n%s", rc, so.String())
	}
	ents, err := os.ReadDir(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("A 类回收失败：临时目录残留 %d 项", len(ents))
	}
}
