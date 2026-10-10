// check_doc_freshness_test.go —— Go 真搬的**成对负控**定向单测（M2 · 文档新鲜度族）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckDocFreshnessGatePort（定向：go test -run TestCheckDocFreshnessGatePort）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cdfBuild —— 造一棵夹具仓（docs/ 语料 + 配置 + 可选附加件）。
func cdfBuild(t *testing.T, docs map[string]string, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range docs {
		write(filepath.Join("docs", name), body)
	}
	for rel, body := range extra {
		write(rel, body)
	}
	write(filepath.Join("scripts", "docs", "doc-freshness.config.json"),
		`{"corpus":["docs"],"base_roots":["","core","docs"],"hard_exclude":[".git"],
"mirror_paths":[],"frozen_paths":[],"third_party_paths":[],"engine_data_paths":[],
"fixture_paths":[],"external_domain_paths":[],"d2":{"resolve_package_relative":true},
"d3":{"slug_profile":"github","spec_doc":"docs/设计.md","include_wiki_links":false},
"d4":{"generators":[]},"allowlist":[],"refuse_domain":[]}`)
	return root
}

// cdfRunIn —— 把 cwd 切到 root 跑入口（Go 侧仓根起于 cwd · 见件头 (a)）。
func cdfRunIn(t *testing.T, root string, argv ...string) (int, string, string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	rc := checkDocFreshnessGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// cdfWithSibling —— 让「仓外同级文档仓」在盘（ext_ok=true · 走原判据），
// 用于夹具格；self-test 格必须在**默认**取根下跑（见件头 (a)·(b)）。
func cdfWithSibling(t *testing.T, root string) {
	t.Helper()
	v := filepath.Join(root, "Zerg-内部文档", "项目文档", "v2.5.10")
	if err := os.MkdirAll(v, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_DEVDOCS_ROOT", v)
}

func TestCheckDocFreshnessGatePort(t *testing.T) {
	// ── ① 正控（好件 ⇒ 0）：引用目标在盘 ───────────────────────────────────
	good := cdfBuild(t, map[string]string{
		"a.md": "[好](b.md)\n", "b.md": "# b\n",
	}, nil)
	cdfWithSibling(t, good)
	rc, out, _ := cdfRunIn(t, good, "d1")
	if rc != 0 || !strings.Contains(out, "D1 · 引用路径存在") || !strings.Contains(out, "rc=0 OK（全绿）") {
		t.Fatalf("正控失败：好件应 rc=0，得 rc=%d out=%q", rc, out)
	}

	// ── ② 负控（坏件 ⇒ 1）：断链被点名 ────────────────────────────────────
	bad := cdfBuild(t, map[string]string{"a.md": "[坏](missing.md)\n"}, nil)
	cdfWithSibling(t, bad)
	rcBad, outBad, _ := cdfRunIn(t, bad, "d1")
	if rcBad != 1 || !strings.Contains(outBad, "missing.md") || !strings.Contains(outBad, "红·断链") {
		t.Fatalf("负控失败：坏件应 rc=1 且点名 missing.md，得 rc=%d out=%q", rcBad, outBad)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rc == rcBad {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rc, rcBad)
	}

	// ── ③ 不给结论（≠ 绿）：语料为空 + 缺语料根 ⇒ rc=2 且点名路径 ──────────
	empty := t.TempDir()
	if err := os.MkdirAll(filepath.Join(empty, "scripts", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(empty, "scripts", "docs", "doc-freshness.config.json"),
		[]byte(`{"corpus":["docs"],"base_roots":[""],"hard_exclude":[],"d4":{"generators":[]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cdfWithSibling(t, empty)
	rcMiss, outMiss, _ := cdfRunIn(t, empty, "d1")
	if rcMiss != 2 || !strings.Contains(outMiss, "缺件") || !strings.Contains(outMiss, "docs") {
		t.Fatalf("缺件失败：应 rc=2 且点名 docs，得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── ④ 负控（D2）：行号越界 + 文件不存在 ⇒ rc=1、两条红 ─────────────────
	d2bad := cdfBuild(t, map[string]string{"a.md": "见 `core/x.py:99` 与 `core/nope.py:1`。\n"},
		map[string]string{"core/x.py": "l1\nl2\nl3\n"})
	cdfWithSibling(t, d2bad)
	rcD2, outD2, _ := cdfRunIn(t, d2bad, "d2")
	if rcD2 != 1 || !strings.Contains(outD2, "红·行号越界") || !strings.Contains(outD2, "红·文件不存在") {
		t.Fatalf("D2 负控失败：应 rc=1 且两类红，得 rc=%d out=%q", rcD2, outD2)
	}

	// ── ⑤ 正控（D3）：锚命中 ⇒ 0；负控：断锚 ⇒ 1（成对）───────────────────
	d3good := cdfBuild(t, map[string]string{"设计.md": "# 设计\n", "a.md": "[好](设计.md#设计)\n"}, nil)
	cdfWithSibling(t, d3good)
	rc3, out3, _ := cdfRunIn(t, d3good, "d3")
	if rc3 != 0 || !strings.Contains(out3, "rc=0 OK（全绿）") {
		t.Fatalf("D3 正控失败：得 rc=%d out=%q", rc3, out3)
	}
	d3bad := cdfBuild(t, map[string]string{"设计.md": "# 设计\n", "a.md": "[坏](设计.md#不存在)\n"}, nil)
	cdfWithSibling(t, d3bad)
	rc3b, out3b, _ := cdfRunIn(t, d3bad, "d3")
	if rc3b != 1 || !strings.Contains(out3b, "红·断锚") {
		t.Fatalf("D3 负控失败：得 rc=%d out=%q", rc3b, out3b)
	}
	if rc3 == rc3b {
		t.Fatalf("D3 成对断言失败：rc=%d == rc=%d", rc3, rc3b)
	}

	os.Unsetenv("ZERG_DEVDOCS_ROOT")
	// ── ⑥ 缺件：配置文件不在 ⇒ rc=2（判据不可判，不许静默绿）──────────────
	noCfg := t.TempDir()
	rcCfg, _, errCfg := cdfRunIn(t, noCfg, "d1")
	if rcCfg != 2 || !strings.Contains(errCfg, "缺配置文件") {
		t.Fatalf("缺配置失败：得 rc=%d stderr=%q", rcCfg, errCfg)
	}

	// ── ⑦ Q-176：本门无 `--json` 读面 ⇒ 拒收 rc=2（stderr 明说 + 指向 --list-items）──
	rcJSON, _, errJSON := cdfRunIn(t, good, "--json")
	if rcJSON != 2 || !strings.Contains(errJSON, "本门无 `--json` 读面") ||
		!strings.Contains(errJSON, "字段表：无") || !strings.Contains(errJSON, "`--list-items`") {
		t.Fatalf("--json 拒收失败：得 rc=%d stderr=%q", rcJSON, errJSON)
	}

	// ── ⑧ 未知旗标：argparse 式 usage + error（stderr）⇒ rc=2 ──────────────
	rcBogus, outBogus, errBogus := cdfRunIn(t, good, "--bogus")
	if rcBogus != 2 || outBogus != "" || !strings.Contains(errBogus, "unrecognized arguments: --bogus") {
		t.Fatalf("未知旗标失败：得 rc=%d out=%q stderr=%q", rcBogus, outBogus, errBogus)
	}

	// ── ⑨ --scope 坏取值 ⇒ rc=2（invalid choice）──────────────────────────
	rcCh, _, errCh := cdfRunIn(t, good, "--scope", "bad")
	if rcCh != 2 || !strings.Contains(errCh, "invalid choice: 'bad'") {
		t.Fatalf("坏 scope 失败：得 rc=%d stderr=%q", rcCh, errCh)
	}

	// ── ⑩ 未知门禁 ⇒ rc=2（用法错）────────────────────────────────────────
	rcG, _, errG := cdfRunIn(t, good, "nosuchgate")
	if rcG != 2 || !strings.Contains(errG, "未知门禁 'nosuchgate'") {
		t.Fatalf("未知门禁失败：得 rc=%d stderr=%q", rcG, errG)
	}

	// ── ⑪ -h 帮助正文逐字（argparse 形）⇒ rc=0 ────────────────────────────
	rcH, outH, _ := cdfRunIn(t, good, "-h")
	if rcH != 0 || !strings.HasPrefix(outH, cdfUsageLine) || !strings.Contains(outH, "optional arguments:") ||
		!strings.Contains(outH, "换到开发文档面") {
		t.Fatalf("-h 失败：得 rc=%d out=%q", rcH, outH)
	}

	// ── ⑫ --list-rules ⇒ rc=0 + 规则表头 ──────────────────────────────────
	rcL, outL, _ := cdfRunIn(t, good, "--list-rules")
	if rcL != 0 || !strings.Contains(outL, "check-doc-freshness.py v1.0.0 —— 规则表") ||
		!strings.Contains(outL, "── D4") {
		t.Fatalf("--list-rules 失败：得 rc=%d out=%q", rcL, outL)
	}

	// ── ⑬ --self-test 自身必须全过（防恒绿/恒红）⇒ rc=0 + 末行自印组数 ───────
	rcST, outST, _ := cdfRunIn(t, good, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "self-test：48 条通过 / 0 条失败") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}

// ── `--repo` 与 `--config` 配对（py `GAP-20260926-184` · 本笔 Go 新行为）──
// 显式给 `--repo <仓外副本>` 而未给 `--config` ⇒ config 按该 repo 推同一相对落点。
// 改前：config 仍按 cwd 解析（cwd=裸目录 ⇒ 「缺配置文件 rc=2」）。有牙：见下两格。
func TestCheckDocFreshnessRepoConfigPairing(t *testing.T) {
	fx := cdfBuild(t, map[string]string{"a.md": "[好](b.md)\n", "b.md": "# b\n"}, nil)
	cdfWithSibling(t, fx) // 让仓外同级文档仓在盘（d1 正常判）
	defer os.Unsetenv("ZERG_DEVDOCS_ROOT")
	bare := t.TempDir() // cwd = 裸目录（无 scripts/docs/配置）
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(bare); err != nil {
		t.Fatal(err)
	}

	// ① 正控：--repo 生效 ⇒ config 按该 repo 推出 ⇒ 语料在场 ⇒ rc=0（改前会「缺配置文件 rc=2」）
	var out, errb bytes.Buffer
	rc := checkDocFreshnessGateMain(&out, &errb, []string{"--repo", fx, "d1"})
	if rc != 0 || !strings.Contains(out.String(), "rc=0 OK（全绿）") {
		t.Fatalf("--repo/--config 配对失败：应 rc=0（config 按 --repo 解析），得 rc=%d out=%q err=%q",
			rc, out.String(), errb.String())
	}
	// ② 成对负控：显式 `--config` 指向不存在的件 ⇒ 仍 rc=2「缺配置文件」（配对不掩盖真缺件）
	var out2, err2 bytes.Buffer
	rc2 := checkDocFreshnessGateMain(&out2, &err2, []string{"--repo", fx, "--config", filepath.Join(bare, "nope.json"), "d1"})
	if rc2 != 2 || !strings.Contains(err2.String(), "缺配置文件") {
		t.Fatalf("成对负控失败：显式缺 --config 应 rc=2「缺配置文件」，得 rc=%d err=%q", rc2, err2.String())
	}
}

// ── GAP-20260926-178：`--list-items` 逐条目机器面（TSV · 与人读面同源 · 与 --json 分开）
// ── GAP-20260926-136：豁免表「登记过期（dead entry）」告警（只报 · 不进退码 · 逐条点名）
func TestCheckDocFreshnessListItemsAndDeadEntry(t *testing.T) {
	// 夹具：D2 两条红（行号越界 + 文件不存在）⇒ findings == 2
	fx := cdfBuild(t, map[string]string{"a.md": "见 `core/x.py:99` 与 `core/nope.py:1`。\n"},
		map[string]string{"core/x.py": "l1\nl2\nl3\n"})
	cdfWithSibling(t, fx)
	defer os.Unsetenv("ZERG_DEVDOCS_ROOT")

	// ① --list-items：只出 TSV（表头 + 逐条）；人读面须被抑制
	rcLI, outLI, _ := cdfRunIn(t, fx, "d2", "--list-items")
	lines := strings.Split(strings.TrimSuffix(outLI, "\n"), "\n")
	if rcLI != 1 || len(lines) == 0 || lines[0] != "gate	file	line	target	kind	domain	detail" {
		t.Fatalf("--list-items 失败：应 rc=1 且首行 TSV 表头，得 rc=%d out=%q", rcLI, outLI)
	}
	if len(lines) != 3 {
		t.Fatalf("--list-items 应出 1 表头 + 2 findings 行，得 %d 行：%q", len(lines), outLI)
	}
	if strings.Contains(outLI, "语料 ") || strings.Contains(outLI, "总耗时 ") {
		t.Fatalf("--list-items 应只出 TSV（人读面须抑制），得 out=%q", outLI)
	}
	// 成对：不给该档时人读面照出（证明「抑制」是给档才做，不是恒抑制）
	_, outPlain, _ := cdfRunIn(t, fx, "d2")
	if !strings.Contains(outPlain, "语料 ") || !strings.Contains(outPlain, "总耗时 ") {
		t.Fatalf("不带 --list-items 应出人读面（成对负控），得 out=%q", outPlain)
	}

	// ② 豁免表登记过期：两条死豁免（宿主不在盘 + 键 0 次）⇒ 逐条点名，且 rc 不受它影响
	al := cdfBuild(t, map[string]string{"a.md": "[好](b.md)\n", "b.md": "# b\n"}, nil)
	cdfWithSibling(t, al)
	// 覆盖配置：加两条死豁免（idx0 宿主不在盘 · idx1 键 0 次）
	if err := os.WriteFile(filepath.Join(al, "scripts", "docs", "doc-freshness.config.json"),
		[]byte(`{"corpus":["docs"],"base_roots":["","docs"],"hard_exclude":[".git"],"mirror_paths":[],`+
			`"frozen_paths":[],"third_party_paths":[],"engine_data_paths":[],"fixture_paths":[],`+
			`"external_domain_paths":[],"d2":{"resolve_package_relative":true},`+
			`"d3":{"slug_profile":"github","spec_doc":"docs/设计.md"},"d4":{"generators":[]},"refuse_domain":[],`+
			`"allowlist":[{"gate":"d1","file_glob":"docs/nope.md","target":"x","reason":"r","date":"2026-01-01"},`+
			`{"gate":"d1","file_glob":"docs/a.md","target":"core/none.py:1","reason":"r","date":"2026-01-01"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rcDE, outDE, _ := cdfRunIn(t, al, "d1")
	if !strings.Contains(outDE, "⚠ 豁免表登记过期（dead entry）= 2 条") ||
		!strings.Contains(outDE, "allowlist[0]") || !strings.Contains(outDE, "allowlist[1]") {
		t.Fatalf("dead entry 告警失败（应 2 条逐条点名）：得 rc=%d out=%q", rcDE, outDE)
	}
	if rcDE != 0 {
		t.Fatalf("dead entry 只报 · 不进退码：应 rc=0，得 rc=%d out=%q", rcDE, outDE)
	}
}

// ── GAP-20261001-50：`--report` 真落盘（改前 cdfWriteReport2 恒 `return nil` ⇒ 静默不落盘）。
// 成对：正控（真写盘 + py 同形键序/空形）↔ 负控（写盘目录不存在 ⇒ 与 py 同 rc=1 · 不落盘 · 不打「总耗时」）。
func TestCheckDocFreshnessReport(t *testing.T) {
	fx := cdfBuild(t, map[string]string{"a.md": "[好](b.md)\n", "b.md": "# b\n"}, nil)
	cdfWithSibling(t, fx)
	defer os.Unsetenv("ZERG_DEVDOCS_ROOT")

	rep := filepath.Join(t.TempDir(), "rep.json")
	rc, out, errs := cdfRunIn(t, fx, "d1", "--report", rep)
	if rc != 0 || !strings.Contains(out, "报告落盘：") {
		t.Fatalf("--report 正控失败：应 rc=0 且打「报告落盘」，得 rc=%d out=%q err=%q", rc, out, errs)
	}
	blob, err := os.ReadFile(rep)
	if err != nil {
		t.Fatalf("--report 应落盘，读回失败：%v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(blob, &doc); err != nil {
		t.Fatalf("--report 产物不是合法 JSON：%v\n%s", err, blob)
	}
	for _, k := range []string{"script", "version", "generated_at", "repo", "scope", "elapsed_ms_total", "index", "gates"} {
		if _, ok := doc[k]; !ok {
			t.Fatalf("--report 缺顶层键 %q：%s", k, blob)
		}
	}
	// 键序 = py dict 插入序（顶层 `script` 必为首键）；缩进 1 空格；无尾随换行
	if !strings.HasPrefix(string(blob), "{\n \"script\": \"check-doc-freshness.py\",\n") {
		t.Fatalf("--report 顶层键序/缩进不符 py：%s", blob)
	}
	if blob[len(blob)-1] != '}' {
		t.Fatalf("--report 结尾应为 '}'（py json.dump 无尾随换行）：%q", blob[len(blob)-4:])
	}
	if !strings.Contains(string(blob), "\"counts\": {\n    \"语料篇数\": 2,") ||
		!strings.Contains(string(blob), "\"errors\": [],") ||
		!strings.Contains(string(blob), "\"findings\": []") {
		t.Fatalf("--report 同形/键序不符 py：%s", blob)
	}

	// 成对负控：写盘目录不存在 ⇒ 与 py 同（rc=1 · 不落盘 · **不**打「总耗时」）
	badPath := filepath.Join(t.TempDir(), "nodir", "x.json")
	rcBad, outBad, errBad := cdfRunIn(t, fx, "d1", "--report", badPath)
	if rcBad != 1 {
		t.Fatalf("--report 负控应 rc=1（对齐 py 未捕获异常退出码），得 rc=%d out=%q err=%q", rcBad, outBad, errBad)
	}
	if strings.Contains(outBad, "总耗时") {
		t.Fatalf("--report 落盘失败应**不**打「总耗时」（对齐 py），得 out=%q", outBad)
	}
	if _, err := os.Stat(badPath); err == nil {
		t.Fatalf("--report 负控不应落盘：%s", badPath)
	}
	if !strings.Contains(errBad, "报告落盘失败") {
		t.Fatalf("--report 负控应给失败档：err=%q", errBad)
	}
}
