// check_doc_cmds_test.go —— Go 真搬的**成对负控**定向单测（M2 · 新口径靶：文仓/外部树依赖类）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckDocCmdsGatePort（定向：go test -run TestCheckDocCmdsGatePort）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dcmdFixture —— 在 t.TempDir() 下造一棵命令树 + baseline（+ 可选 docs/ 语料）的仓外夹具。
func dcmdFixture(t *testing.T, docs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{filepath.Join("core", "cmd", "zerg"), filepath.Join("scripts", "gates")} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	tree := "var commands []*command\n" +
		"func init() {\n\tcommands = []*command{\n" +
		"\t\t{path: []string{\"version\"}, run: cmdVersion},\n" +
		"\t\t{path: []string{\"dev\", \"proposal\"}, run: cmdDevProposal},\n" +
		"		{path: []string{\"help\"}, run: cmdHelp},\n" +
		"		{path: []string{\"isolate-ro\"}, run: cmdIsolateRO},\n" +
		"	}\n}\n"
	if err := os.WriteFile(filepath.Join(root, "core", "cmd", "zerg", "main.go"), []byte(tree), 0o644); err != nil {
		t.Fatal(err)
	}
	bl, _ := json.Marshal(map[string]interface{}{"devdocs_report_hits": 0, "note": "test"})
	if err := os.WriteFile(filepath.Join(root, "scripts", "gates", "doc-cmds-baseline.json"), bl, 0o644); err != nil {
		t.Fatal(err)
	}
	if docs != nil {
		base := filepath.Join(root, "docs")
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, body := range docs {
			if err := os.WriteFile(filepath.Join(base, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

// dcmdRunIn —— 把 cwd 切到 root 跑入口（Go 侧仓根起于 cwd 上溯 · 见件头 (a)）。
func dcmdRunIn(t *testing.T, root string, argv ...string) (int, string, string) {
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
	rc := checkDocCmdsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckDocCmdsGatePort(t *testing.T) {
	// ── ① 正控（好件 ⇒ 0）：围栏里的真命令 + 行内反引号 ─────────────────────
	good := dcmdFixture(t, map[string]string{
		"good.md": "跑这个：\n```sh\nzerg dev proposal list\n```\n行内 `zerg version` 也过。\n",
	})
	rc, out, _ := dcmdRunIn(t, good, "--scope", "formal")
	if rc != 0 || !strings.Contains(out, "formal 命中 0（阻断档）") || !strings.Contains(out, "⇒ 全过（rc=0）") {
		t.Fatalf("正控失败：好件应 rc=0，得 rc=%d out=%q", rc, out)
	}

	// ── ② 负控（坏件 ⇒ 1）：命令树里没有的坏命令名被点名 ────────────────────
	bad := dcmdFixture(t, map[string]string{"bad.md": "```sh\nzerg dev propozal list\n```\n"})
	rcBad, outBad, _ := dcmdRunIn(t, bad, "--scope", "formal")
	if rcBad != 1 || !strings.Contains(outBad, "propozal") || !strings.Contains(outBad, "不在命令树") {
		t.Fatalf("负控失败：坏件应 rc=1 且点名 propozal，得 rc=%d out=%q", rcBad, outBad)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rc == rcBad {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rc, rcBad)
	}

	// ── ③ 负控：族名不许当前缀停靠（`dev` 真源里不是完整命令）───────────────
	rcFam, outFam, _ := dcmdRunIn(t, dcmdFixture(t, map[string]string{
		"bad.md": "```sh\nzerg dev propozal\n```\n",
	}), "--scope", "formal")
	if rcFam != 1 || !strings.Contains(outFam, "propozal") {
		t.Fatalf("族名放行失败：得 rc=%d out=%q", rcFam, outFam)
	}

	// ── ④ 正控：顶层单段命令 + 子动作（`isolate-ro new`）⇒ 0 ─────────────────
	rcTop, outTop, _ := dcmdRunIn(t, dcmdFixture(t, map[string]string{
		"good.md": "```sh\nzerg isolate-ro new <真仓>\n```\n",
	}), "--scope", "formal")
	if rcTop != 0 {
		t.Fatalf("顶层命令+子动作失败：应 rc=0（本夹具树无 isolate-ro 命令 ⇒ 仍判红说明判据正常）out=%q", outTop)
	}

	// ── ⑤ 正控：help <主题>（主题不在命令树）⇒ 只判族名 `help` ⇒ 0 ──────────
	rcHelp, outHelp, _ := dcmdRunIn(t, dcmdFixture(t, map[string]string{
		"good.md": "退码表见 `zerg help exit-codes`。\n",
	}), "--scope", "formal")
	if rcHelp != 0 {
		t.Fatalf("help 主题失败：应 rc=0，得 rc=%d out=%q", rcHelp, outHelp)
	}

	// ── ⑥ 不给结论（≠ 绿）：formal 语料整棵缺席 ⇒ rc=2 ────────────────────────
	rcMiss, outMiss, _ := dcmdRunIn(t, dcmdFixture(t, nil), "--scope", "formal")
	if rcMiss != 2 || !strings.Contains(outMiss, "docs/ 整棵缺席") || !strings.Contains(outMiss, "不给结论（rc=2）") {
		t.Fatalf("缺件失败：应 rc=2，得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── ⑦ 不给结论（≠ 绿）：devdocs 根整棵不在盘 ⇒ rc=2 ──────────────────────
	noDocs := dcmdFixture(t, nil)
	os.Setenv("ZERG_DEVDOCS_ROOT", filepath.Join(noDocs, "no-such-Zerg-内部文档"))
	defer os.Unsetenv("ZERG_DEVDOCS_ROOT")
	rcDD, outDD, _ := dcmdRunIn(t, noDocs, "--scope", "devdocs")
	if rcDD != 2 || !strings.Contains(outDD, "整棵不在盘") {
		t.Fatalf("devdocs 缺根失败：应 rc=2，得 rc=%d out=%q", rcDD, outDD)
	}

	// ── ⑧ 负控：baseline 缺件 ⇒ rc=2（棘轮没法判 ⇒ 不给结论）────────────────
	noBL := dcmdFixture(t, map[string]string{"good.md": "行内 `zerg version`\n"})
	if err := os.Remove(filepath.Join(noBL, "scripts", "gates", "doc-cmds-baseline.json")); err != nil {
		t.Fatal(err)
	}
	rcBL, outBL, _ := dcmdRunIn(t, noBL, "--scope", "formal")
	if rcBL != 2 || !strings.Contains(outBL, "baseline 缺件或解不动") {
		t.Fatalf("baseline 缺件失败：应 rc=2，得 rc=%d out=%q", rcBL, outBL)
	}

	// ── ⑨ Q-176：本门无机器读面 ⇒ --json 拒收 rc=2（stderr 明说）─────────────
	rcJSON, _, errJSON := dcmdRunIn(t, good, "--json")
	if rcJSON != 2 || !strings.Contains(errJSON, "本门无机器读面") || !strings.Contains(errJSON, "字段表：无") {
		t.Fatalf("--json 拒收失败：得 rc=%d stderr=%q", rcJSON, errJSON)
	}

	// ── ⑩ 未知旗标：argparse 式 usage + error（stderr）⇒ rc=2 ───────────────
	rcBogus, outBogus, errBogus := dcmdRunIn(t, good, "--bogus")
	if rcBogus != 2 || outBogus != "" || !strings.Contains(errBogus, "unrecognized arguments: --bogus") {
		t.Fatalf("未知旗标失败：得 rc=%d out=%q stderr=%q", rcBogus, outBogus, errBogus)
	}

	// ── ⑪ --scope 坏取值 ⇒ rc=2（invalid choice）────────────────────────────
	rcCh, _, errCh := dcmdRunIn(t, good, "--scope", "bad")
	if rcCh != 2 || !strings.Contains(errCh, "invalid choice: 'bad'") {
		t.Fatalf("坏 scope 失败：得 rc=%d stderr=%q", rcCh, errCh)
	}

	// ── ⑫ -h 帮助正文逐字（argparse 形）⇒ rc=0 ──────────────────────────────
	rcH, outH, _ := dcmdRunIn(t, good, "-h")
	if rcH != 0 || !strings.HasPrefix(outH, "usage: check-doc-cmds.py [-h] [--scope {formal,devdocs}] [--self-test]") ||
		!strings.Contains(outH, "optional arguments:") || !strings.Contains(outH, "语料域（可重复；缺省 = 两棵都判）") {
		t.Fatalf("-h 失败：得 rc=%d out=%q", rcH, outH)
	}

	// ── ⑬ --self-test 自身必须全过（防恒绿/恒红）⇒ rc=0 + 末行自印组数 ────────
	rcST, outST, _ := dcmdRunIn(t, good, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检全过（28 组") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}

// ── B 类（读失败静默）回归牙：语料域里一件 .md 锁成 chmod 000 ⇒
// 主件 dcmdJudge 的 os.ReadFile 失败登记面（dcmdReadFails）必响：
// rc=2 + stderr BLOCKED 头 + 逐件点名那份锁死件。登记/判红若删回静默，本用例必红。
func TestCheckDocCmdsReadFailTeeth(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 绕过权限位 ⇒ chmod 000 不挡读，本用例无鉴别力")
	}
	root := dcmdFixture(t, map[string]string{
		"good.md":   "行内 `zerg version` 也过。\n",
		"locked.md": "跑这个：\n```sh\nzerg version\n```\n",
	})
	locked := filepath.Join(root, "docs", "locked.md")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o644)
	rc, _, errb := dcmdRunIn(t, root, "--scope", "formal")
	if rc != 2 {
		t.Fatalf("读失败回归牙：语料域锁死件 ⇒ 应 rc=2（登记 + 判红），得 rc=%d stderr=%q", rc, errb)
	}
	if !strings.Contains(errb, "BLOCKED") {
		t.Fatalf("读失败回归牙：stderr 缺 BLOCKED 头，得 stderr=%q", errb)
	}
	if !strings.Contains(errb, "locked.md") {
		t.Fatalf("读失败回归牙：未逐件点名锁死件 locked.md，得 stderr=%q", errb)
	}
}

// ── GAP-20261001-32③ 回归牙（本单同步引入的新行为 · 照 py 候件 P3B 同步）：语料域一件 .md 读不了 ⇒
// ①进 rc 通道（rc=2）且 stdout 带「N 篇读不了」的 blocked 文案；
// ②读不了 ≠ 零命中：同域的真坏明细照旧点名；
// ③成对反控：去掉锁死件 ⇒ 好件 rc=0 / 坏件 rc=1 且无 blocked 文案（✗ 禁恒红）。
// 哑化主件 dcmdJudge 的 `nUnread` 计数 / 调用点的 blocked 合并 ⇒ 本用例必红。
func TestCheckDocCmdsUnreadableBlocked(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 绕过权限位 ⇒ chmod 000 不挡读，本用例无鉴别力")
	}
	const blocked = "⛔ 不给结论：formal 语料有 1 篇读不了（读不了 ≠ 零命中）—— 判据不可判"

	// ① 只有读不了件 ⇒ rc=2 + blocked 文案
	root := dcmdFixture(t, map[string]string{
		"good.md":   "跑这个：\n```sh\nzerg version\n```\n",
		"locked.md": "跑这个：\n```sh\nzerg version\n```\n",
	})
	if err := os.Chmod(filepath.Join(root, "docs", "locked.md"), 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(root, "docs", "locked.md"), 0o644)
	rc, out, _ := dcmdRunIn(t, root, "--scope", "formal")
	if rc != 2 || !strings.Contains(out, blocked) {
		t.Fatalf("读不了回归牙：应 rc=2 且 stdout 带 blocked 文案，得 rc=%d out=%q", rc, out)
	}

	// ② 读不了 + 真坏：明细仍在（读不了 ≠ 零命中）
	root2 := dcmdFixture(t, map[string]string{
		"bad.md":    "```sh\nzerg dev propozal list\n```\n",
		"locked.md": "x\n",
	})
	if err := os.Chmod(filepath.Join(root2, "docs", "locked.md"), 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(root2, "docs", "locked.md"), 0o644)
	rc2, out2, _ := dcmdRunIn(t, root2, "--scope", "formal")
	if rc2 != 2 || !strings.Contains(out2, "propozal") || !strings.Contains(out2, blocked) {
		t.Fatalf("读不了回归牙：读不了 ≠ 零命中 ⇒ 真坏明细须仍在且带 blocked，得 rc=%d out=%q", rc2, out2)
	}

	// ③ 成对反控（不得恒红）：无锁死件 ⇒ 好件 0 / 坏件 1，且无「读不了」/blocked 文案
	good := dcmdFixture(t, map[string]string{"good.md": "跑这个：\n```sh\nzerg version\n```\n"})
	rcG, outG, _ := dcmdRunIn(t, good, "--scope", "formal")
	if rcG != 0 || strings.Contains(outG, "读不了") || strings.Contains(outG, blocked) {
		t.Fatalf("成对反控失败：好件应 rc=0 且无「读不了」，得 rc=%d out=%q", rcG, outG)
	}
	bad := dcmdFixture(t, map[string]string{"bad.md": "```sh\nzerg dev propozal list\n```\n"})
	rcB, outB, _ := dcmdRunIn(t, bad, "--scope", "formal")
	if rcB != 1 || strings.Contains(outB, "读不了") || strings.Contains(outB, blocked) {
		t.Fatalf("成对反控失败：坏件应 rc=1 且无「读不了」，得 rc=%d out=%q", rcB, outB)
	}
}

// ── 未建提案书写体例回归牙（本单同步 · 照 py `bde233fb` 的 ⑳㉑㉒ · 识别跳过第三类）──
// 判语引出的 `zerg <名>`（账里的提案名）⇒ **不计**（rc=0）且第三类计数认出；
// 同一条命令写成**普通示例**（无判语）⇒ **必计**（rc=1 并点名）。哑化主件 `dcmdExtract` 的
// proposal 分支 / `dcmdProposalMark` ⇒ ① 必红（rc 翻 1 或第三类计数为 0）。
func TestCheckDocCmdsProposalMarkTeeth(t *testing.T) {
	// ① 判语（想要）引出 ⇒ 不计 ⇒ rc=0，且「识别跳过」行**认出来了**（第三类计数 = 1）
	r1 := dcmdFixture(t, map[string]string{"good.md": "未建提案：想要 `zerg pipelinedefect` 收成正门。\n"})
	rc1, out1, _ := dcmdRunIn(t, r1, "--scope", "formal")
	if rc1 != 0 {
		t.Fatalf("提案体例回归牙：判语引出的 `zerg <名>` 应不计 ⇒ rc=0，得 rc=%d out=%q", rc1, out1)
	}
	if !strings.Contains(out1, "未建提案体例（判语引出的 `zerg <名>`）1 段") {
		t.Fatalf("提案体例回归牙：第三类计数未认出（应见「…）1 段」），得 out=%q", out1)
	}

	// ② 成对反控（✗ 禁恒绿）：同一条命令**无判语** ⇒ 必计 ⇒ rc=1 且点名
	r2 := dcmdFixture(t, map[string]string{"good.md": "用法：`zerg pipelinedefect` 可直接跑。\n"})
	rc2, out2, _ := dcmdRunIn(t, r2, "--scope", "formal")
	if rc2 != 1 || !strings.Contains(out2, "pipelinedefect") {
		t.Fatalf("提案体例回归牙（成对）：无判语应必计 ⇒ rc=1 且点名，得 rc=%d out=%q", rc2, out2)
	}
}

// ── devdocs 默认根按「唯一真源 version.go」现算（本笔 Go 新行为 · py `GAP-20260926-209`）──
// 改前 Go 钉死 `v2.5.13`（死版本号）；本笔改为读真源现算。有牙：钉 9.9.9 + 语料在 v9.9.9
// ⇒ 走旧口径取不到根 ⇒ rc=2；读不到真源 ⇒ 归 blocked（rc=2 具名，不静默绿）。
func TestCheckDocCmdsDevdocsVersionSource(t *testing.T) {
	fx := dcmdFixture(t, nil)
	verDir := filepath.Join(fx, "core", "internal", "version")
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		t.Fatal(err)
	}
	verFile := filepath.Join(verDir, "version.go")
	if err := os.WriteFile(verFile, []byte("package version\n\nconst Version = \"9.9.9\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	alt := t.TempDir()
	d := filepath.Join(alt, "项目文档", "v9.9.9")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "good.md"), []byte("行内 `zerg version` 也过。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_DOCS_ALT", alt)
	os.Unsetenv("ZERG_DEVDOCS_ROOT")

	// ① 正控：真源 9.9.9 ⇒ 默认根算成 v9.9.9 ⇒ 语料在场 ⇒ rc=0（旧口径会去 v2.5.13 ⇒ rc=2）
	rc, out, _ := dcmdRunIn(t, fx, "--scope", "devdocs")
	if rc != 0 || !strings.Contains(out, "devdocs（只报告档") {
		t.Fatalf("版本真源现算失败：应把默认根算成 v9.9.9 ⇒ rc=0，得 rc=%d out=%q", rc, out)
	}
	// ② 成对（真源读不到）：删 version.go ⇒ 归 blocked（rc=2 具名「版本真源读不到」，不静默绿）
	if err := os.Remove(verFile); err != nil {
		t.Fatal(err)
	}
	rc2, out2, _ := dcmdRunIn(t, fx, "--scope", "devdocs")
	if rc2 != 2 || !strings.Contains(out2, "版本真源读不到") {
		t.Fatalf("真源缺失应 rc=2 且点名版本真源，得 rc=%d out=%q", rc2, out2)
	}
}
