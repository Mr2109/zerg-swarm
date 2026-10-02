// check_judge_registry_test.go —— `checkJudgeRegistryGateMain` 的成对负控（正控绿 / 各造 violation 红）。
//
// ★ 每格都自成一对：先证「干净夹具 ⇒ 绿」（正控），再证「同一夹具加一处扰动 ⇒ rc=1/2」（负控）。
// 只证「绿」或只证「红」的用例没有鉴别力，一律不收。
// ★ DEV-1142「拆两半」后新增：派生半**不再出**四字段（needs_env/serial/hazard/dry_run）；
//
//	人签半（judge-adoption/1）三类决定 adopt/exempt/declare · enforce 迁移期与 enforce_sig 上牙机检。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func jrRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkJudgeRegistryGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func jrMkdirs(t *testing.T, root string, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func jrFix(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	jrMkdirs(t, root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a')\n")
	jrMkdirs(t, root, "scripts/b.sh", "#!/usr/bin/env bash\necho b\n")
	jrMkdirs(t, root, "scripts/gates/__pycache__/ignored.py", "# 缓存件，不进名录\n")
	return root
}

// jrWriteReg 现跑 --emit --out - 并把产物落成 reg.json ⇒ 返回其路径（供 --check 读）。
func jrWriteReg(t *testing.T, root string) string {
	t.Helper()
	rc, out, errb := jrRun(t, "--root", root, "--emit", "--out", "-")
	if rc != 0 || errb != "" {
		t.Fatalf("前置 emit：rc=%d stderr=%q", rc, errb)
	}
	regPath := filepath.Join(root, "reg.json")
	if err := os.WriteFile(regPath, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	return regPath
}

// jrWriteDec 把人签半字面落盘 ⇒ 返回其路径。
func jrWriteDec(t *testing.T, root, body string) string {
	t.Helper()
	decPath := filepath.Join(root, "dec.json")
	if err := os.WriteFile(decPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return decPath
}

// ── 正控 ＋ 「--self-test 逐字」 ──

func TestCheckJudgeRegistrySelfTest(t *testing.T) {
	rc, out, errb := jrRun(t, "--self-test")
	want := "✓ 正控：干净夹具判绿（条目 3）\n" +
		"✓ 格A（甲/乙 分离）：`--emit` 产物无人签半字段（含四字段已搬走）\n" +
		"✓ 格B（人签半未建）：乙半面不判红（迁移期只报告）\n" +
		"✓ 格C（人签半全覆盖 adopt）：判绿\n" +
		"✓ 格D（enforce=true · 在册缺一条决定）：现红\n" +
		"✓ 格E（悬空决定）：现红\n" +
		"✓ 格F（闭集 / 未人签）：现红\n" +
		"✓ 格G（exempt 可见）：备注逐条打印（非静默放过）\n" +
		"✓ 格G2（exempt ✗ 不判红）：豁免只报告不判红\n" +
		"✓ 格H（`--emit` ✗ 不碰人签半）：`build_registry` 与人签半无入参耦合\n" +
		"✓ 格I（declare 合法 + enforce=false）：判绿（b 仅声明未采纳 ⇒ 采纳面迁移期不判红）\n" +
		"✓ 格I2（declare 可见）：备注逐条打印四字段（非静默）\n" +
		"✓ 格J（declare 越界/缺键）：现红（非布尔 / 值越闭集 / 缺载荷 各自点名）\n" +
		"✓ 格J2（declare 缺字段键）：现红（缺 `dry_run` 点名）\n" +
		"✓ 格K1（enforce=false）：未决定不判红（迁移期只报告）\n" +
		"✓ 格K2（enforce=true）：未决定判红（现 3 条）\n" +
		"✓ 格K3（enforce=false 脚注）：脚注报「未决定 3 条」\n" +
		"✓ 格L（上牙未人签）：现红（enforce=true 缺 enforce_sig）\n" +
		"✓ 格M（enforce 非布尔）：现红（并落回迁移期读）\n" +
		"✓ 负控①（盘上有件表里无）：现红\n" +
		"✓ 负控②（表里点名盘上无）：现红\n" +
		"✓ 负控③（缓存键过期）：现红\n" +
		"✓ 负控④（同名多件）：现红\n" +
		"✓ 负控⑤（越出前缀）：现红\n"
	if rc != 0 {
		t.Fatalf("--self-test rc=%d, want 0（stdout=%q stderr=%q）", rc, out, errb)
	}
	if out != want {
		t.Fatalf("--self-test stdout 非逐字：\n got %q\nwant %q", out, want)
	}
	if errb != "" {
		t.Fatalf("--self-test stderr=%q, want empty", errb)
	}
}

// ── 正控：干净夹具 --emit 结构对 ＋ 往返 --check 绿 ──

func TestCheckJudgeRegistryEmitCheckRoundTrip(t *testing.T) {
	root := jrFix(t)
	rc, out, errb := jrRun(t, "--root", root, "--emit", "--out", "-")
	if rc != 0 || errb != "" {
		t.Fatalf("--emit rc=%d stderr=%q", rc, errb)
	}
	for _, sub := range []string{
		`"schema": "judge-registry/1"`,
		`"roster_dir": "scripts"`,
		`"extensions": [`,
		`"count": 2`,
		`"dupes": []`,
		`"cache_key": `,
		`"interp": "python3"`,
		`"interp": "bash"`,
	} {
		if !strings.Contains(out, sub) {
			t.Errorf("--emit stdout 缺 %q\n---\n%s", sub, out)
		}
	}
	if strings.Contains(out, "ignored") {
		t.Errorf("`__pycache__` 下的件不该进名录，却出现 ignored")
	}
	if !strings.HasSuffix(out, "}\n") {
		t.Errorf("--emit stdout 应以 JSON ＋ 尾换行收束")
	}

	regPath := filepath.Join(root, "reg.json")
	if err := os.WriteFile(regPath, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out2, errb2 := jrRun(t, "--root", root, "--registry", regPath, "--check")
	if rc != 0 {
		t.Fatalf("往返 --check 应绿：rc=%d stderr=%q", rc, errb2)
	}
	// 人签半未建 ⇒ 乙半迁移期脚注走 stderr（正常档 · ✗ 不改退码）。
	if !strings.Contains(errb2, "未建") || !strings.Contains(errb2, "未决定 2 条") {
		t.Fatalf("往返 --check stderr 未带乙半迁移期脚注：%q", errb2)
	}
	if !strings.Contains(out2, "✓ 判据注册表对拍绿（rc=0）：条目 2") {
		t.Fatalf("--check 绿行非逐字：%q", out2)
	}
}

// ── ★ DEV-1142：派生半 ✗ 不得出四字段（needs_env/serial/hazard/dry_run）＋ ✗ 不得出人签半字段 ──

func TestCheckJudgeRegistryEmitOmitsPersonFields(t *testing.T) {
	root := jrFix(t)
	_, out, _ := jrRun(t, "--root", root, "--emit", "--out", "-")
	for _, k := range []string{"needs_env", "serial", "hazard", "dry_run",
		"\"decision\"", "\"adopt\"", "\"exempt\"", "\"declare\"", "\"sig\"", "\"adopted\""} {
		if strings.Contains(out, k) {
			t.Errorf("派生半（--emit）不该含 %q：\n%s", k, out)
		}
	}
}

// ── 负控①：盘上有件、表里没点名 ⇒ rc=1 ──

func TestCheckJudgeRegistryNewFileUnregistered(t *testing.T) {
	root := jrFix(t)
	regPath := jrWriteReg(t, root)

	// 正控：此时绿
	if rc, _, _ := jrRun(t, "--root", root, "--registry", regPath, "--check"); rc != 0 {
		t.Fatalf("前置正控该绿，得 rc=%d", rc)
	}
	// 负控：加一件未登记
	jrMkdirs(t, root, "scripts/c.py", "#!/usr/bin/env python3\nprint('c')\n")
	rc, _, errb := jrRun(t, "--root", root, "--registry", regPath, "--check")
	if rc != 1 {
		t.Fatalf("新件未登记应 rc=1，得 %d（stderr=%q）", rc, errb)
	}
	if !strings.Contains(errb, "没点名") || !strings.Contains(errb, "c（scripts/c.py）") {
		t.Errorf("stderr 未逐字点名缺登记件：%q", errb)
	}
}

// ── 负控②：表里点名、盘上无件 ⇒ rc=1（悬空登记） ──

func TestCheckJudgeRegistryDanglingEntry(t *testing.T) {
	root := jrFix(t)
	regPath := jrWriteReg(t, root)
	if rc, _, _ := jrRun(t, "--root", root, "--registry", regPath, "--check"); rc != 0 {
		t.Fatalf("前置正控该绿")
	}
	data, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	m["entries"] = append(m["entries"].([]any), map[string]any{
		"id": "ghost", "name": "ghost", "path": "scripts/ghost.py",
		"sha256": strings.Repeat("0", 64),
	})
	b, _ := json.Marshal(m)
	ghost := filepath.Join(root, "ghost.json")
	_ = os.WriteFile(ghost, b, 0o644)
	rc, _, errb := jrRun(t, "--root", root, "--registry", ghost, "--check")
	if rc != 1 {
		t.Fatalf("悬空登记应 rc=1，得 %d（stderr=%q）", rc, errb)
	}
	if !strings.Contains(errb, "悬空登记") || !strings.Contains(errb, "ghost（scripts/ghost.py）") {
		t.Errorf("stderr 未逐字报悬空登记：%q", errb)
	}
}

// ── 负控③：件被改 ⇒ 缓存键过期 ⇒ rc=1 ──

func TestCheckJudgeRegistryStaleCacheKey(t *testing.T) {
	root := jrFix(t)
	regPath := jrWriteReg(t, root)
	if rc, _, _ := jrRun(t, "--root", root, "--registry", regPath, "--check"); rc != 0 {
		t.Fatalf("前置正控该绿")
	}
	jrMkdirs(t, root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a-changed')\n")
	rc, _, errb := jrRun(t, "--root", root, "--registry", regPath, "--check")
	if rc != 1 || !strings.Contains(errb, "缓存键过期") {
		t.Fatalf("缓存键过期应 rc=1＋具名，得 rc=%d stderr=%q", rc, errb)
	}
}

// ── 负控④：同名多件 ⇒ --emit 报 dupes 且不进 entries ⇒ rc=0 但 stderr 有警 ──

func TestCheckJudgeRegistrySameNameMultiple(t *testing.T) {
	root := jrFix(t)
	rc, out, errb := jrRun(t, "--root", root, "--emit", "--out", "-")
	if rc != 0 || errb != "" || !strings.Contains(out, `"dupes": []`) {
		t.Fatalf("前置正控：rc=%d stderr=%q", rc, errb)
	}
	jrMkdirs(t, root, "scripts/sub/a.py", "#!/usr/bin/env python3\nprint('a2')\n")
	rc, out2, errb2 := jrRun(t, "--root", root, "--emit", "--out", "-")
	if rc != 0 {
		t.Fatalf("同名多件时 --emit 仍应 rc=0，得 %d", rc)
	}
	if !strings.Contains(out2, `"dupes": [`) || !strings.Contains(out2, `"a"`) {
		t.Errorf("stdout 未记 dupes：%s", out2)
	}
	if !strings.Contains(out2, `"count": 1`) {
		t.Errorf("同名条目应从 entries 剔除（count 1）：%s", out2)
	}
	if !strings.Contains(errb2, "盘上有同名多件") {
		t.Errorf("stderr 未报同名多件：%q", errb2)
	}
}

// ── 负控⑤：schema 不符 ⇒ rc=2（加载期硬错） ──

func TestCheckJudgeRegistrySchemaMismatch(t *testing.T) {
	root := jrFix(t)
	bad := filepath.Join(root, "bad.json")
	_ = os.WriteFile(bad, []byte(`{"schema":"wrong/9","count":0,"entries":[]}`), 0o644)
	rc, out, errb := jrRun(t, "--root", root, "--registry", bad, "--check")
	if rc != 2 {
		t.Fatalf("schema 不符应 rc=2，得 %d（stdout=%q stderr=%q）", rc, out, errb)
	}
	if !strings.Contains(errb, "加载期硬错") || !strings.Contains(errb, "schema 不符") {
		t.Errorf("stderr 未报加载期硬错：%q", errb)
	}
	if !strings.Contains(errb, "wrong/9") {
		t.Errorf("stderr 未带现读 schema：%q", errb)
	}
}

// ── 负控⑥：注册表读不到 ⇒ rc=2（不给结论 · 静默绿是大忌） ──

func TestCheckJudgeRegistryMissingRegistry(t *testing.T) {
	root := jrFix(t)
	rc, _, errb := jrRun(t, "--root", root, "--registry", filepath.Join(root, "nope.json"), "--check")
	if rc != 2 {
		t.Fatalf("注册表读不到应 rc=2，得 %d", rc)
	}
	if !strings.Contains(errb, "注册表读不到") {
		t.Errorf("stderr=%q", errb)
	}
}

// ── 负控⑦：三选一都不给 / 未知旗标 / 互斥 ⇒ rc=2 ＋ 具名文案 ──

func TestCheckJudgeRegistryUsageErrors(t *testing.T) {
	rc, _, errb := jrRun(t)
	if rc != 2 || errb != "用法错：`--emit` / `--check` / `--self-test` 三选一必给一个（都不给 ⇒ rc=2）\n" {
		t.Fatalf("无旗标应 rc=2＋逐字用法行，得 rc=%d stderr=%q", rc, errb)
	}
	rc, _, errb = jrRun(t, "--bogus")
	if rc != 2 || !strings.Contains(errb, "未知旗标") {
		t.Fatalf("未知旗标应 rc=2＋具名，得 rc=%d stderr=%q", rc, errb)
	}
	rc, _, errb = jrRun(t, "--emit", "--check")
	if rc != 2 || !strings.Contains(errb, "互斥") {
		t.Fatalf("互斥应 rc=2＋具名，得 rc=%d stderr=%q", rc, errb)
	}
}

// ── 负控⑧：--emit 落盘（非 `-`）时打确认行（★ DEV-1142：作「派生半」） ──

func TestCheckJudgeRegistryEmitToFile(t *testing.T) {
	root := jrFix(t)
	outPath := filepath.Join(root, "sub", "reg.json")
	rc, out, errb := jrRun(t, "--root", root, "--emit", "--out", outPath)
	if rc != 0 || errb != "" {
		t.Fatalf("--emit 落盘 rc=%d stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "判据注册表（派生半）已生成：") || !strings.Contains(out, "（条目 2 · 同名歧义 0）") {
		t.Errorf("确认行非逐字：%q", out)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("落盘件不存在：%v", err)
	}
}

// ── 鉴别力自证：把「缺 name」条目喂进去 —— 该档必须现红（防「恒绿」假件） ──

func TestCheckJudgeRegistryEntryMissingName(t *testing.T) {
	root := jrFix(t)
	bad := filepath.Join(root, "bad2.json")
	_ = os.WriteFile(bad, []byte(`{"schema":"judge-registry/1","count":1,"dupes":[],"entries":[{"id":"x"}]}`), 0o644)
	rc, _, errb := jrRun(t, "--root", root, "--registry", bad, "--check")
	if rc != 1 {
		t.Fatalf("缺 name 条目应 rc=1，得 %d（stderr=%q）", rc, errb)
	}
	if !strings.Contains(errb, "注册表条目缺 `name`") {
		t.Errorf("stderr 未报缺 name：%q", errb)
	}
	if !strings.Contains(errb, "{'id': 'x'}") {
		t.Errorf("stderr 未带 Python repr 形态：%q", errb)
	}
}

// ── ★ DEV-1142 人签半（乙）· 三类决定 / enforce 两态 / declare 闭集 ──

// 乙半未建 ⇒ 迁移期：rc=0 ＋ 脚注「未决定 N 条」。
func TestCheckJudgeRegistryDecisionsAbsent(t *testing.T) {
	root := jrFix(t)
	regPath := jrWriteReg(t, root)
	rc, _, errb := jrRun(t, "--root", root, "--registry", regPath, "--check")
	if rc != 0 {
		t.Fatalf("人签半未建应 rc=0（迁移期），得 %d（stderr=%q）", rc, errb)
	}
	if !strings.Contains(errb, "未建") || !strings.Contains(errb, "未决定 2 条") {
		t.Errorf("stderr 未报迁移期脚注：%q", errb)
	}
}

// enforce=false ⇒ 在册无人签采纳/豁免决定**不判红** · 脚注报「未决定 2 条」。
func TestCheckJudgeRegistryEnforceFalseUndecided(t *testing.T) {
	root := jrFix(t)
	regPath := jrWriteReg(t, root)
	decPath := jrWriteDec(t, root, `{"schema":"judge-adoption/1","enforce":false,"decisions":[]}`)
	rc, out, errb := jrRun(t, "--root", root, "--registry", regPath, "--decisions", decPath, "--check")
	if rc != 0 {
		t.Fatalf("enforce=false 应 rc=0，得 %d（stdout=%q stderr=%q）", rc, out, errb)
	}
	if !strings.Contains(errb, "`enforce`=False") || !strings.Contains(errb, "未决定 2 条") {
		t.Errorf("stderr 未报 enforce=false ＋ 未决定计数：%q", errb)
	}
	if !strings.Contains(out, "甲⇄乙 对拍过（enforce=false 迁移期）") {
		t.Errorf("绿行未带甲⇄乙 迁移期态：%q", out)
	}
}

// enforce=true ＋ enforce_sig 齐 ⇒ 在册无人签采纳/豁免决定判红 rc=1。
func TestCheckJudgeRegistryEnforceTrueSigned(t *testing.T) {
	root := jrFix(t)
	regPath := jrWriteReg(t, root)
	decPath := jrWriteDec(t, root,
		`{"schema":"judge-adoption/1","enforce":true,"enforce_sig":{"by":"tester","at":"2026-10-03","sig":"s-enforce"},"decisions":[]}`)
	rc, _, errb := jrRun(t, "--root", root, "--registry", regPath, "--decisions", decPath, "--check")
	if rc != 1 {
		t.Fatalf("enforce=true 未决定应 rc=1，得 %d（stderr=%q）", rc, errb)
	}
	if !strings.Contains(errb, "无人签采纳/豁免决定") {
		t.Errorf("stderr 未报未决定（上牙）：%q", errb)
	}
	if strings.Contains(errb, "上牙未人签") || strings.Contains(errb, "**未人签**") {
		t.Errorf("enforce_sig 齐时不该报上牙未人签：%q", errb)
	}
}

// enforce=true 缺 enforce_sig ⇒ rc=1（上牙本身必须人签）。
func TestCheckJudgeRegistryEnforceTrueUnsigned(t *testing.T) {
	root := jrFix(t)
	regPath := jrWriteReg(t, root)
	decPath := jrWriteDec(t, root, `{"schema":"judge-adoption/1","enforce":true,"decisions":[]}`)
	rc, _, errb := jrRun(t, "--root", root, "--registry", regPath, "--decisions", decPath, "--check")
	if rc != 1 {
		t.Fatalf("enforce=true 缺 enforce_sig 应 rc=1，得 %d（stderr=%q）", rc, errb)
	}
	if !strings.Contains(errb, "上牙") || !strings.Contains(errb, "未人签") {
		t.Errorf("stderr 未报「上牙未人签」：%q", errb)
	}
}

// declare 载荷值越出闭集 ⇒ rc=1 且逐字段点名。
func TestCheckJudgeRegistryDeclareOutOfClosedSet(t *testing.T) {
	root := jrFix(t)
	regPath := jrWriteReg(t, root)
	decPath := jrWriteDec(t, root,
		`{"schema":"judge-adoption/1","enforce":false,"decisions":[{"id":"a","decision":"declare",`+
			`"by":"tester","at":"2026-10-03","sig":"sig-a",`+
			`"fields":{"needs_env":"yes","serial":false,"hazard":"boom","dry_run":"yes"}}]}`)
	rc, _, errb := jrRun(t, "--root", root, "--registry", regPath, "--decisions", decPath, "--check")
	if rc != 1 {
		t.Fatalf("declare 越界应 rc=1，得 %d（stderr=%q）", rc, errb)
	}
	if !strings.Contains(errb, "`needs_env` 非布尔") ||
		!strings.Contains(errb, "`hazard` 值不在闭集") ||
		!strings.Contains(errb, "`dry_run` 值不在闭集") {
		t.Errorf("stderr 未逐字段点名越界：%q", errb)
	}
}

// 人签半 schema 不符 ⇒ rc=2（加载期硬错 · 不给结论）。
func TestCheckJudgeRegistryDecisionsSchemaMismatch(t *testing.T) {
	root := jrFix(t)
	regPath := jrWriteReg(t, root)
	decPath := jrWriteDec(t, root, `{"schema":"wrong/1","enforce":false,"decisions":[]}`)
	rc, _, errb := jrRun(t, "--root", root, "--registry", regPath, "--decisions", decPath, "--check")
	if rc != 2 {
		t.Fatalf("人签半 schema 不符应 rc=2，得 %d（stderr=%q）", rc, errb)
	}
	if !strings.Contains(errb, "人签半加载期硬错") {
		t.Errorf("stderr 未报人签半加载期硬错：%q", errb)
	}
}
