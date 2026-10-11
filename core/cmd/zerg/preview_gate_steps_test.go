// preview_gate_steps_test.go —— `pgsGateMain`（scripts/gates/preview-gate-steps.py 的 Go 真搬）的**成对负控**单测。
//
// 用法：cd core && go test -mod=mod ./cmd/zerg -run 'Pgs' -v
//
// ★ 全部私有符号带 `pgs` / `pgsTest` 前缀（多路并行落件防撞名 · 已在仓件同查）。
// ★ 成对负控（✗ 禁恒绿）：每条关键判据都**正反两向**各钉一格 —— 空步表 ≠ 取不到步表 ·
//
//	全角数字认 ⟷ 非数字不认 · 写意图三档全 rc=2 ⟷ 只读档 rc=0 · 自检自回收 delta=0。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const pgsTestList = `自检结论: 全过（断言 3 条 · 失败 0 条）
── 步骤清单（scope 模式 名称）──
   go    empty  gofmt -l core
   gates tri    门① 覆盖（阻断）
   docs  tri-report docs: glossary 术语表 zh+en（只报告）
共 3 步
`

func pgsTestMkFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败 %s: %v", name, err)
	}
	return p
}

func pgsTestRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := pgsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// ── ① parse_list：空步表 ≠ 取不到步表（成对·血证 GAP-20261001-13 的 nil 面）────────
func TestPgsParseListEmptyVsMissing(t *testing.T) {
	// 正向：有「步骤清单」段 + `共 0 步`（件在盘、表为空）⇒ **不当取不到**，退 err="" 且 steps 非 nil。
	empty := "── 步骤清单（scope 模式 名称）──\n共 0 步\n"
	steps, total, hasTotal, err := pgsParseList(empty)
	if err != "" {
		t.Fatalf("空步表被误判成「取不到」：err=%q（应为空）", err)
	}
	if steps == nil {
		t.Fatalf("空步表返回 nil slice ⇒ 下游无法与「取不到」区分（应返回 []）")
	}
	if !hasTotal || total != 0 || len(steps) != 0 {
		t.Fatalf("空步表读数错：hasTotal=%v total=%d len=%d", hasTotal, total, len(steps))
	}
	// 反向：无「步骤清单」段（真取不到）⇒ err 非空。
	_, _, _, err2 := pgsParseList("nothing here\n")
	if err2 == "" {
		t.Fatalf("缺「步骤清单」段却未判「取不到」⇒ 负控失灵")
	}
	// 另一反向：段在但缺 `共 N 步` ⇒ 计数自证做不了 ⇒ err 非空。
	_, _, _, err3 := pgsParseList("── 步骤清单（scope 模式 名称）──\n   go empty a\n")
	if err3 == "" {
		t.Fatalf("缺「共 N 步」却未判「计数自证做不了」⇒ 负控失灵")
	}
}

func TestPgsParseListCountAndParts(t *testing.T) {
	steps, total, hasTotal, err := pgsParseList(pgsTestList)
	if err != "" || !hasTotal || total != 3 || len(steps) != 3 {
		t.Fatalf("解析错：err=%q total=%d hasTotal=%v n=%d", err, total, hasTotal, len(steps))
	}
	if steps[1] != (pgsStep{Scope: "gates", Mode: "tri", Name: "门① 覆盖（阻断）"}) {
		t.Fatalf("三格解错：%+v", steps[1])
	}
	// 计数自证不过 ⇒ 判不了
	if _, _, _, e := pgsParseList(strings.Replace(pgsTestList, "共 3 步", "共 5 步", 1)); e == "" {
		t.Fatalf("步数不符却未判不了 ⇒ 负控失灵")
	}
	// 行不足三格 ⇒ 判不了
	if _, _, _, e := pgsParseList("── 步骤清单（scope 模式 名称）──\n   gates tri\n"); e == "" {
		t.Fatalf("两格行却未判不了 ⇒ 负控失灵")
	}
	// 总步数行必须带 `步` 收尾：`共 1x` **不是**总数行 ⇒ 判不了（防「丢掉 `步` 要求」的回归）
	if _, _, _, e := pgsParseList("── 步骤清单（scope 模式 名称）──\n   go empty a\n共 1x\n"); e == "" {
		t.Fatalf("`共 1x` 被误当总数行 ⇒ `步` 收尾牙失灵")
	}
}

// py json.JSONDecodeError 正文的逐字牙（本件 pgsPyJSONErr 映射）。
func TestPgsPyJSONErrStrings(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"{\"a\" 1}": "提案 JSON 解不动：Expecting ':' delimiter: line 1 column 6 (char 5)",
		"x":         "提案 JSON 解不动：Expecting value: line 1 column 1 (char 0)",
		"{不是 json":  "提案 JSON 解不动：Expecting property name enclosed in double quotes: line 1 column 2 (char 1)",
		"[1 2]":     "提案 JSON 解不动：Expecting ',' delimiter: line 1 column 4 (char 3)",
	}
	i := 0
	for body, want := range cases {
		i++
		p := pgsTestMkFile(t, dir, "j"+strconv.Itoa(i)+".json", body)
		if _, got := pgsParseProposal(p); got != want {
			t.Errorf("JSON 错字正文不符：\n got=%q\nwant=%q", got, want)
		}
	}
}

// 全角数字：py `\d` + `int()` 认 Unicode Nd ⇒ `共 ３ 步` 亦作 3（成对：非数字不认）。
func TestPgsParseListUnicodeDigits(t *testing.T) {
	txt := "── 步骤清单（scope 模式 名称）──\n   go empty a\n   go empty b\n   go empty c\n共 ３ 步\n"
	_, total, hasTotal, err := pgsParseList(txt)
	if err != "" || !hasTotal || total != 3 {
		t.Fatalf("全角「３」未按 3 解：err=%q hasTotal=%v total=%d", err, hasTotal, total)
	}
	bad := strings.Replace(txt, "共 ３ 步", "共 x 步", 1)
	// 首行三格仍成立 ⇒ 应落到「缺共 N 步」；无论如何 err 非空。
	if _, _, _, e := pgsParseList(bad); e == "" {
		t.Fatalf("非数字却未判不了 ⇒ 负控失灵")
	}
}

// ── ② parse_proposal：两形 + 闭集 + 重名 + 空表 ───────────────────────────────
func TestPgsParseProposalForms(t *testing.T) {
	dir := t.TempDir()
	tsv := pgsTestMkFile(t, dir, "p.tsv",
		"# 注释整行\n"+
			"go\tempty\tgofmt -l core\n"+
			"gates\ttri-report\t门① 覆盖（阻断）\n")
	prop, err := pgsParseProposal(tsv)
	if err != "" || len(prop) != 2 {
		t.Fatalf("TSV 提案解错：err=%q n=%d", err, len(prop))
	}
	if prop[1].Mode != "tri-report" {
		t.Fatalf("模式解错：%q", prop[1].Mode)
	}
	// 名称里的 `#` 不被当注释截断
	h := pgsTestMkFile(t, dir, "h.tsv", "gates\ttri\tscripts/*（无后缀 + 首行 #!）按 shebang 语法\n")
	ph, e2 := pgsParseProposal(h)
	if e2 != "" || len(ph) != 1 || ph[0].Name != "scripts/*（无后缀 + 首行 #!）按 shebang 语法" {
		t.Fatalf("`#` 被截断：err=%q %+v", e2, ph)
	}
	// JSON 形
	js := pgsTestMkFile(t, dir, "p.json", `{"id": "proposal.v1", "steps": [{"scope": "gates", "mode": "tri", "name": "门①"}]}`)
	pj, e3 := pgsParseProposal(js)
	if e3 != "" || len(pj) != 1 || pj[0].Name != "门①" {
		t.Fatalf("JSON 提案解错：err=%q %+v", e3, pj)
	}
}

func TestPgsParseProposalRejections(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"缺格":    "gates\ttri\n",
		"闭集外":   "gates\t阻断\t门①\n",
		"重名":    "gates\ttri\t门①\ngates\ttri\t门①\n",
		"空表":    "",
		"坏JSON": "{不是 json",
	}
	for name, body := range cases {
		p := pgsTestMkFile(t, dir, strings.ReplaceAll(name, "/", "_")+".tsv", body)
		if name == "坏JSON" {
			p = pgsTestMkFile(t, dir, "bad.json", body)
		}
		if _, e := pgsParseProposal(p); e == "" {
			t.Errorf("负控失灵：%s 未被判不了", name)
		}
	}
	// 反向正控：合法两行必过（成对）
	good := pgsTestMkFile(t, dir, "good.tsv", "gates\ttri\t门①\n")
	if _, e := pgsParseProposal(good); e != "" {
		t.Errorf("合法提案被误拒：%q", e)
	}
	// 提案件不在盘 ⇒ rc 面文案
	if _, e := pgsParseProposal(filepath.Join(dir, "nope.tsv")); !strings.Contains(e, "提案读不到：") {
		t.Errorf("缺件文案错：%q", e)
	}
}

// ── ③ diff_steps：序（提案序 / 现表序）+ 注记排序 ──────────────────────────────
func TestPgsDiffStepsOrderAndNotes(t *testing.T) {
	cur := []pgsStep{{"gates", "tri", "甲", false}, {"docs", "tri-report", "乙", false}}
	// 提案序：乙（先）改 scope、甲（后）改模式、丙 新增。
	prop := []pgsStep{{"release", "tri-report", "乙", false}, {"gates", "tri-report", "甲", false}, {"gates", "tri", "丙", false}}
	added, removed, changed, notes := pgsDiffSteps(cur, prop)
	// added 依**提案序**（只丙）
	if len(added) != 1 || added[0].Name != "丙" {
		t.Fatalf("added 错：%+v", added)
	}
	if len(removed) != 0 {
		t.Fatalf("removed 应为 0：%+v", removed)
	}
	// changed 依**提案序**：乙 在前（scope 变）、甲 在后（模式变）
	if len(changed) != 2 {
		t.Fatalf("changed 应 2 处：%+v", changed)
	}
	if changed[0].A.Name != "乙" || changed[0].Why != "scope 变化（同一道门归到别的 scope）" {
		t.Fatalf("changed[0] 错：%+v", changed[0])
	}
	if changed[1].A.Name != "甲" || changed[1].Why != "模式变化" {
		t.Fatalf("changed[1] 错：%+v", changed[1])
	}
	// 新 scope 注记（去重 + sorted）：release 不在现表
	if len(notes) != 1 || !strings.Contains(notes[0], "`release`") {
		t.Fatalf("scope 注记错：%+v", notes)
	}
	// 成对负控：模式变化**不算**增删
	if _, _, c2, _ := pgsDiffSteps(cur, []pgsStep{{"gates", "tri-report", "甲", false}, {"docs", "tri-report", "乙", false}}); len(c2) != 1 {
		t.Fatalf("只改一档应 1 处变化：%+v", c2)
	}
}

// ── ④ 写意图三档：一律 rc=2 且一字不写 ───────────────────────────────────────
func TestPgsWriteIntentRefusalTiers(t *testing.T) {
	dir := t.TempDir()
	tgt := pgsTestMkFile(t, dir, "t.sh", "#!/bin/sh\ntrue\n")
	pre, _ := pgsSHA256File(tgt)
	if rc, _ := pgsWriteIntentRefusal(&pgsArgs{target: "t.sh", apply: true}, "t.sh"); rc != 2 {
		t.Fatalf("档① 应 rc=2，实 %d", rc)
	}
	if rc, _ := pgsWriteIntentRefusal(&pgsArgs{target: "t.sh", apply: true, yes: true}, "t.sh"); rc != 2 {
		t.Fatalf("档② 应 rc=2，实 %d", rc)
	}
	ap := pgsTestMkFile(t, dir, "a.json", "{}\n")
	rc, lines := pgsWriteIntentRefusal(&pgsArgs{target: "t.sh", apply: true, yes: true, approval: ap}, "t.sh")
	if rc != 2 {
		t.Fatalf("档③ 应 rc=2，实 %d", rc)
	}
	if !pgsAnyContains(lines, "写面本波不做") {
		t.Fatalf("档③ 判词缺「写面本波不做」：%+v", lines)
	}
	post, _ := pgsSHA256File(tgt)
	if post != pre {
		t.Fatalf("写意图竟然改了件：%s → %s", pre, post)
	}
}

// ── ⑤ argparse 面：-h / 未知旗标 / 缺值 ──────────────────────────────────────
func TestPgsArgparseHelpAndErrors(t *testing.T) {
	rc, out, errb := pgsTestRun("--help")
	if rc != 0 || out != pgsHelp || errb != "" {
		t.Fatalf("-h 档错：rc=%d out==help?%v err=%q", rc, out == pgsHelp, errb)
	}
	if !strings.Contains(out, "optional arguments:") {
		t.Fatalf("-h 正文缺 argparse 3.9 标题")
	}
	rc2, out2, errb2 := pgsTestRun("--nope")
	if rc2 != 2 || out2 != "" {
		t.Fatalf("未知旗标应 rc=2 且 stdout 空：rc=%d out=%q", rc2, out2)
	}
	if !strings.Contains(errb2, "unrecognized arguments: --nope") || !strings.HasPrefix(errb2, "usage: preview-gate-steps.py") {
		t.Fatalf("未知旗标 stderr 错：%q", errb2)
	}
	rc3, _, errb3 := pgsTestRun("--proposal")
	if rc3 != 2 || !strings.Contains(errb3, "argument --proposal: expected one argument") {
		t.Fatalf("缺值档错：rc=%d err=%q", rc3, errb3)
	}
	// --opt=值 等号形
	rc4, out4, _ := pgsTestRun("--root", t.TempDir(), "--target=missing.sh")
	if rc4 != 2 || !strings.Contains(out4, "⛔ 目标件不在：missing.sh") {
		t.Fatalf("等号形/缺件档错：rc=%d out=%q", rc4, out4)
	}
}

// ── ⑥ 只读预览端到端 + JSON 空集合（[] 非 null）────────────────────────────
func TestPgsGateMainReadOnlyPreviewAndJSON(t *testing.T) {
	dir := t.TempDir()
	cache := pgsTestMkFile(t, dir, "list.txt", pgsTestList)
	prop := pgsTestMkFile(t, dir, "p.tsv",
		"go\tempty\tgofmt -l core\n"+
			"gates\ttri\t门① 覆盖（阻断）\n"+
			"gates\ttri-report\t门㉑ 新门\n")
	tgt := pgsTestMkFile(t, dir, "gate.sh", "#!/bin/sh\n")
	pre, _ := pgsSHA256File(tgt)

	rc, out, errb := pgsTestRun("--root", dir, "--list-cache", cache, "--proposal", prop, "--target", "gate.sh")
	if rc != 0 || errb != "" {
		t.Fatalf("只读预览应 rc=0：rc=%d err=%q", rc, errb)
	}
	for _, want := range []string{"── 门禁步骤表只读预览（`R-02` · **不写任何件**）──", "   +1 步（**将新增**）", "     + gates    tri-report  门㉑ 新门", "   -1 步（**将删**）", "   只读性自证：目标件 `gate.sh`"} {
		if !strings.Contains(out, want) {
			t.Errorf("预览输出缺：%q", want)
		}
	}
	post, _ := pgsSHA256File(tgt)
	if post != pre {
		t.Fatalf("只读预览改了目标件")
	}

	// JSON 档：一步不差 ⇒ added/removed 必为 `[]` 而非 `null`（空集合显式置空）
	same := pgsTestMkFile(t, dir, "same.tsv",
		"go\tempty\tgofmt -l core\n"+
			"gates\ttri\t门① 覆盖（阻断）\n"+
			"docs\ttri-report\tdocs: glossary 术语表 zh+en（只报告）\n")
	rc2, out2, _ := pgsTestRun("--root", dir, "--list-cache", cache, "--proposal", same, "--target", "gate.sh", "--json")
	if rc2 != 0 {
		t.Fatalf("JSON 档应 rc=0：%d", rc2)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(pgsTestFirstJSONBlob(out2)), &doc); err != nil {
		t.Fatalf("JSON 输出解不动：%v\n%s", err, out2)
	}
	if doc["id"] != "gate-steps-diff.v1" {
		t.Errorf("id 错：%v", doc["id"])
	}
	// added 必须存在且为空数组（不是 null、也不缺键）
	added, ok := doc["added"].([]any)
	if !ok || len(added) != 0 {
		t.Errorf("空 added 应为 []（非 null）：%v", doc["added"])
	}
	if !strings.Contains(out2, "\"added\": []") {
		t.Errorf("JSON 正文未显式打空数组：\n%s", out2)
	}
}

// pgsTestFirstJSONBlob —— 从 stdout 里取出那段缩进 JSON（后随只读性自证行）。
func pgsTestFirstJSONBlob(out string) string {
	i := strings.Index(out, "{")
	if i < 0 {
		return out
	}
	depth, inStr, esc := 0, false, false
	for j := i; j < len(out); j++ {
		c := out[j]
		if inStr {
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return out[i : j+1]
			}
		}
	}
	return out[i:]
}

// ── ⑦ 写意图端到端：rc=2 且目标件 sha 不变 ─────────────────────────────────
func TestPgsGateMainWriteIntentRefuses(t *testing.T) {
	dir := t.TempDir()
	tgt := pgsTestMkFile(t, dir, "gate.sh", "#!/bin/sh\n")
	pre, _ := pgsSHA256File(tgt)
	rc, out, _ := pgsTestRun("--root", dir, "--target", "gate.sh", "--apply")
	if rc != 2 || !strings.Contains(out, "⇒ **不写**（rc=2）") {
		t.Fatalf("写意图应 rc=2：rc=%d out=%q", rc, out)
	}
	post, _ := pgsSHA256File(tgt)
	if post != pre {
		t.Fatalf("写意图改了目标件")
	}
}

// ── ⑧ --template：从快照打骨架（只印不写）────────────────────────────────────
func TestPgsTemplate(t *testing.T) {
	dir := t.TempDir()
	cache := pgsTestMkFile(t, dir, "list.txt", pgsTestList)
	tgt := pgsTestMkFile(t, dir, "gate.sh", "#!/bin/sh\n")
	pre, _ := pgsSHA256File(tgt)
	rc, out, _ := pgsTestRun("--root", dir, "--list-cache", cache, "--target", "gate.sh", "--template")
	if rc != 0 {
		t.Fatalf("--template 应 rc=0：%d", rc)
	}
	if !strings.Contains(out, "# 门禁步骤表提案骨架") || !strings.Contains(out, "# 共 3 步") {
		t.Fatalf("骨架输出错：%q", out)
	}
	if !strings.Contains(out, "gates\ttri\t门① 覆盖（阻断）") {
		t.Fatalf("骨架行错：%q", out)
	}
	if post, _ := pgsSHA256File(tgt); post != pre {
		t.Fatalf("--template 改了目标件")
	}
}

// ── ⑨ 自检：全过 + 自回收（A 类 tempdir delta=0 牙）────────────────────────
func TestPgsSelfTestSucceedsAndCleansTmp(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
	pre := pgsTestCountTmpDirs(t)
	var buf bytes.Buffer
	rc := pgsSelfTest(&buf)
	if rc != 0 {
		t.Fatalf("自检应全过（rc=0），实 %d\n%s", rc, buf.String())
	}
	if !strings.Contains(buf.String(), "自检结论: 全过（失败 0 条）") {
		t.Fatalf("自检结论行错：%s", buf.String())
	}
	if !strings.Contains(buf.String(), "   ✓ ① `--list` 解出 4 步") {
		t.Fatalf("自检 ① 未 ✓：%s", buf.String())
	}
	post := pgsTestCountTmpDirs(t)
	if post != pre {
		t.Fatalf("自检 tempdir 未自回收：前 %d 后 %d（应 delta=0）", pre, post)
	}
}

// pgsTestCountTmpDirs —— 数 TMPDIR 下本件自检前缀的目录（A 类泄漏探针）。
func pgsTestCountTmpDirs(t *testing.T) int {
	t.Helper()
	dir := os.TempDir()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读 TMPDIR 失败：%v", err)
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() && strings.HasPrefix(e.Name(), "zerg-preview-steps-selftest-") {
			n++
		}
	}
	return n
}

// ── ⑩ GAP-20261005-30 残面：现状真源走**机读面**（`--list --json`），人面只作退化路 ──────────
// 机读面单对象 `{"count":N,"steps":[{"scope":..,"mode":..,"name":..},…]}`（`precommit-gates.sh --list --json`）。
const pgsTestListJSON = `{"count":3,"steps":[` +
	`{"scope":"go","mode":"empty","name":"gofmt -l core"},` +
	`{"scope":"gates","mode":"tri","name":"门① 覆盖（阻断）"},` +
	`{"scope":"docs","mode":"tri-report","name":"docs: glossary 术语表 zh+en（只报告）"}]}`

func TestPgsParseListJSONMachineFace(t *testing.T) {
	// 正控：机读面单对象 ⇒ 解成（含「共 N 步」文案**一字未参与** —— 文案改不影响读数）
	steps, total, isM, err := pgsParseListJSON(pgsTestListJSON)
	if !isM || err != "" || total != 3 || len(steps) != 3 {
		t.Fatalf("机读面解错：isM=%v err=%q total=%d n=%d", isM, err, total, len(steps))
	}
	if steps[1] != (pgsStep{Scope: "gates", Mode: "tri", Name: "门① 覆盖（阻断）"}) {
		t.Fatalf("机读面三格解错：%+v", steps[1])
	}
	// 负控①：人面文本 ⇒ **无机读面**（应交人面退化路，不许假装读出）
	if _, _, isM2, _ := pgsParseListJSON(pgsTestList); isM2 {
		t.Fatalf("人面文本被误当机读面 ⇒ 退化路走不到")
	}
	// 负控②：别的 JSON 对象 ⇒ 不是步表机读面
	if _, _, isM3, _ := pgsParseListJSON(`{"a": 1}` + "\n"); isM3 {
		t.Fatalf("非步表 JSON 被误当机读面")
	}
	// 负控③：机读面在场但计数自证不过 ⇒ isMachine=true 且 err 非空（**响亮失败**，✗ 静默退回人面）
	if _, _, isM4, e4 := pgsParseListJSON(`{"count":5,"steps":[{"scope":"go","mode":"empty","name":"a"}]}` + "\n"); !isM4 || e4 == "" {
		t.Fatalf("机读面计数不符却未响亮失败：isM=%v err=%q", isM4, e4)
	}
	// 负控④：机读面在场但某条缺 `name` ⇒ 响亮失败
	if _, _, isM5, e5 := pgsParseListJSON(`{"count":1,"steps":[{"scope":"go","mode":"empty"}]}` + "\n"); !isM5 || e5 == "" {
		t.Fatalf("机读面缺 name 却未响亮失败：isM=%v err=%q", isM5, e5)
	}
	// 负控⑤：机读面文案变体不可能误伤 —— 同一条机读面，附一段**改了字**的人面，仍读机读面
	mutated := "── 步骤清单（改了表头文案）──\n   go    empty  gofmt -l core\n共 9 步\n" + pgsTestListJSON + "\n"
	s2, t2, isM6, e6 := pgsParseListJSON(mutated)
	if !isM6 || e6 != "" || t2 != 3 || len(s2) != 3 {
		t.Fatalf("人面文案改后机读面未优先：isM=%v err=%q total=%d n=%d", isM6, e6, t2, len(s2))
	}
}

// pgsLoadCurrent 走机读面快照（`--list-cache` 存的是 `--list --json` 单对象）⇒ src 点明机读面；
// 人面快照仍走人面退化路（成对负控：src 不含「机读面」）。
func TestPgsLoadCurrentMachineFaceCache(t *testing.T) {
	dir := t.TempDir()
	mcache := pgsTestMkFile(t, dir, "list.json", pgsTestListJSON+"\n")
	steps, src, err := pgsLoadCurrent(dir, &pgsArgs{listCache: mcache})
	if err != "" || len(steps) != 3 {
		t.Fatalf("机读面快照解错：err=%q n=%d", err, len(steps))
	}
	if !strings.Contains(src, "机读面") || !strings.Contains(src, "共 3 步") {
		t.Fatalf("机读面真源串错：%q", src)
	}
	// 成对负控：人面快照仍走人面退化路
	hc := pgsTestMkFile(t, dir, "list.txt", pgsTestList)
	hsteps, hsrc, herr := pgsLoadCurrent(dir, &pgsArgs{listCache: hc})
	if herr != "" || len(hsteps) != 3 || strings.Contains(hsrc, "机读面") {
		t.Fatalf("人面快照真源串错（应走退化路）：err=%q n=%d src=%q", herr, len(hsteps), hsrc)
	}
}

// 负控（真红牙）：机读面在场而形状坏 ⇒ `pgsLoadCurrent` **响亮失败**（err 非空），✗ 不许退回人面当绿。
func TestPgsLoadCurrentMachineFaceLoudFail(t *testing.T) {
	dir := t.TempDir()
	bad := pgsTestMkFile(t, dir, "bad.json", `{"count":7,"steps":[{"scope":"go","mode":"empty","name":"a"}]}`+"\n")
	if _, _, err := pgsLoadCurrent(dir, &pgsArgs{listCache: bad}); err == "" {
		t.Fatalf("机读面计数不符却未响亮失败（静默当绿 ⇒ 负控失灵）")
	}
}
