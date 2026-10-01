// check_doc_meta_test.go —— Go 真搬的**成对负控**定向单测（M2 · 新口径靶：文仓/外部树依赖类）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckDocMetaGatePort（定向：go test -run TestCheckDocMetaGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cdmFixtureSchema —— 夹具里的最小 schema（关键字全在本脚本实现子集内；✗ 不引未实现关键字）。
const cdmFixtureSchema = `{
  "type": "object",
  "required": ["title", "type", "status", "source_of_truth", "owner", "updated_at"],
  "additionalProperties": false,
  "properties": {
    "title": {"type": "string", "minLength": 1, "maxLength": 60},
    "type": {"enum": ["tutorial", "how-to", "reference", "explanation", "adr", "record"]},
    "status": {"enum": ["authoritative", "draft", "historical", "speculation"]},
    "source_of_truth": {"oneOf": [
      {"type": "string", "minLength": 1},
      {"type": "array", "minItems": 1, "uniqueItems": true, "items": {"type": "string", "minLength": 1}}
    ]},
    "owner": {"type": "string", "minLength": 1},
    "updated_at": {"type": "string", "format": "date", "pattern": "^\\d{4}-\\d{2}-\\d{2}$"},
    "reviewed_at": {"type": "string", "format": "date", "pattern": "^\\d{4}-\\d{2}-\\d{2}$"},
    "ai_assisted": {"enum": ["human", "ai", "mixed"]}
  },
  "allOf": [
    {"if": {"required": ["status"], "properties": {"status": {"const": "authoritative"}}},
     "then": {"required": ["reviewed_at"]}},
    {"if": {"required": ["type"], "properties": {"type": {"const": "adr"}}},
     "then": {"required": ["status", "updated_at"]}}
  ]
}`

// cdmGoodBody —— 一件必然判绿的文档（与 py 自检 GOOD_DOC 逐字同构）。
const cdmGoodBody = "---\ntitle: 好件\ntype: reference\nstatus: draft\n" +
	"source_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n\n# 好件\n"

// cdmFixture —— 在 t.TempDir() 下造一棵「仓根 + schema + 若干文档」的仓外夹具，返回根。
func cdmFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "site"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "site", "frontmatter-schema.json"),
		[]byte(cdmFixtureSchema), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// cdmPut —— 往夹具里写一件（相对路径 + 正文）。
func cdmPut(t *testing.T, root, rel, body string) string {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// cdmRunGate —— 入口直调（仓根/schema 显式传 ⇒ cwd 无关）。
func cdmRunGate(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkDocMetaGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// cdmBaseArgs —— 显式仓根 + 夹具 schema + 关自检（自检夹具不落在本用例的 TempDir 里）。
func cdmBaseArgs(root string) []string {
	return []string{"--repo-root", root, "--schema",
		filepath.Join(root, "docs", "site", "frontmatter-schema.json"), "--no-self-test"}
}

func TestCheckDocMetaGatePort(t *testing.T) {
	root := cdmFixture(t)

	// ── ① 正控（好件 ⇒ rc=0 且计数为「合规 1」）────────────────────────────
	good := cdmPut(t, root, filepath.Join("docs", "good", "a-good.md"), cdmGoodBody)
	rc, out, _ := cdmRunGate(t, append(cdmBaseArgs(root), "--target", good)...)
	if rc != 0 || !strings.Contains(out, "合规 1") || !strings.Contains(out, "OK：合规 1 篇") {
		t.Fatalf("正控失败：好件应 rc=0 且合规 1，得 rc=%d out=%q", rc, out)
	}

	// ── ② 负控（坏件 ⇒ rc=1 且点名它自己的规则号）──────────────────────────
	bad := cdmPut(t, root, filepath.Join("docs", "bad", "bad-type.md"),
		"---\ntitle: 坏 type\ntype: guide\nstatus: draft\nsource_of_truth: x\nowner: o\n"+
			"updated_at: 2026-09-18\n---\n")
	rcBad, outBad, _ := cdmRunGate(t, append(cdmBaseArgs(root), "--target", bad)...)
	if rcBad != 1 || !strings.Contains(outBad, "M3") || !strings.Contains(outBad, "'guide' 不在封闭值域") {
		t.Fatalf("负控失败：坏件应 rc=1 且点名 M3/guide，得 rc=%d out=%q", rcBad, outBad)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rc == rcBad {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rc, rcBad)
	}

	// ── ③ 空值三形态：`[]` / `null` / 缺键（判据各不同，✗ 不许混成一档）──────
	emptyList := cdmPut(t, root, filepath.Join("docs", "shapes", "s-empty-list.md"),
		"---\ntitle: 空列表\ntype: reference\nstatus: draft\nsource_of_truth: []\nowner: o\n"+
			"updated_at: 2026-09-18\n---\n")
	rcEL, outEL, _ := cdmRunGate(t, append(cdmBaseArgs(root), "--target", emptyList)...)
	if rcEL != 1 || !strings.Contains(outEL, "oneOf 的 2 个分支都不满足") {
		t.Fatalf("`[]` 形态失败：应 rc=1 且报 oneOf 两分支皆不满足，得 rc=%d out=%q", rcEL, outEL)
	}
	nullVal := cdmPut(t, root, filepath.Join("docs", "shapes", "s-null.md"),
		"---\ntitle: 空值\ntype: reference\nstatus: draft\nsource_of_truth: null\nowner: o\n"+
			"updated_at: 2026-09-18\n---\n")
	rcNL, outNL, _ := cdmRunGate(t, append(cdmBaseArgs(root), "--target", nullVal)...)
	if rcNL != 1 || !strings.Contains(outNL, "oneOf 的 2 个分支都不满足") {
		t.Fatalf("`null` 形态失败：应 rc=1 且报 oneOf 两分支皆不满足，得 rc=%d out=%q", rcNL, outNL)
	}
	missing := cdmPut(t, root, filepath.Join("docs", "shapes", "s-missing.md"),
		"---\ntitle: 缺键\ntype: reference\nsource_of_truth: x\nowner: o\n"+
			"updated_at: 2026-09-18\n---\n")
	rcMS, outMS, _ := cdmRunGate(t, append(cdmBaseArgs(root), "--target", missing)...)
	if rcMS != 1 || !strings.Contains(outMS, "M2") || !strings.Contains(outMS, "status: 必填项缺失") {
		t.Fatalf("缺键形态失败：应 rc=1 且报 M2/status，得 rc=%d out=%q", rcMS, outMS)
	}

	// ── ④ 不给结论（≠ 绿）：目标不存在 ⇒ rc=2 ─────────────────────────────
	rcMiss, outMiss, _ := cdmRunGate(t, append(cdmBaseArgs(root), "--target",
		filepath.Join(root, "no-such-dir"))...)
	if rcMiss != 2 || !strings.Contains(outMiss, "BLOCKED：目标不存在 ⇒ 不给结论") {
		t.Fatalf("缺件失败：应 rc=2，得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── ⑤ 不给结论（≠ 绿）：扫描域为空 ⇒ rc=2（空转不许读成绿）─────────────
	emptyDir := filepath.Join(root, "docs", "empty")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rcEmp, outEmp, _ := cdmRunGate(t, append(cdmBaseArgs(root), "--target", emptyDir)...)
	if rcEmp != 2 || !strings.Contains(outEmp, "扫描域为空") {
		t.Fatalf("空转失败：应 rc=2 且报扫描域为空，得 rc=%d out=%q", rcEmp, outEmp)
	}

	// ── ⑥ 目录默认值不一致 = 告警（rc=0，不是 rc=1）──────────────────────
	sk := cdmPut(t, root, filepath.Join("docs", "skills", "warn.md"), cdmGoodBody)
	rcW, outW, _ := cdmRunGate(t, append(cdmBaseArgs(root), "--target", sk)...)
	if rcW != 0 || !strings.Contains(outW, "M16") || !strings.Contains(outW, "该目录默认值 = how-to") {
		t.Fatalf("M16 失败：应 rc=0 告警，得 rc=%d out=%q", rcW, outW)
	}

	// ── ⑦ 不可判定（写法未建模）⇒ rc=2，且规则号是 M14 ─────────────────────
	blk := cdmPut(t, root, filepath.Join("docs", "blk", "b.md"),
		"---\ntitle: 嵌套\nmeta:\n  a: 1\ntype: reference\nstatus: draft\n"+
			"source_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n")
	rcBlk, outBlk, _ := cdmRunGate(t, append(cdmBaseArgs(root), "--target", blk)...)
	if rcBlk != 2 || !strings.Contains(outBlk, "M14") {
		t.Fatalf("不可判定失败：应 rc=2 且报 M14，得 rc=%d out=%q", rcBlk, outBlk)
	}

	// ── ⑧ --missing=fail：同一件（无 frontmatter）由告警升红（两档结论必须不同）
	nfm := cdmPut(t, root, filepath.Join("docs", "nofm", "x.md"), "# 没有 frontmatter\n")
	rcWarn, outWarn, _ := cdmRunGate(t, append(cdmBaseArgs(root), "--target", nfm)...)
	if rcWarn != 0 || !strings.Contains(outWarn, "M1") || !strings.Contains(outWarn, "告警 1") {
		t.Fatalf("M1 默认为告警失败：应 rc=0，得 rc=%d out=%q", rcWarn, outWarn)
	}
	rcFail, outFail, _ := cdmRunGate(t, append(cdmBaseArgs(root), "--target", nfm, "--missing=fail")...)
	if rcFail != 1 || !strings.Contains(outFail, "不合规 1") {
		t.Fatalf("M1 --missing=fail 失败：应 rc=1，得 rc=%d out=%q", rcFail, outFail)
	}
	if rcWarn == rcFail {
		t.Fatalf("成对断言失败：M1 两档 rc 同码（%d）⇒ 档位没接上", rcWarn)
	}

	// ── ⑨ --json 机器面：空集合必须是 `[]`/`{}`（✗ 不是 null，✗ 不许吞键）────
	rcJ, outJ, _ := cdmRunGate(t, append(cdmBaseArgs(root), "--target", good, "--json")...)
	if rcJ != 0 || !strings.Contains(outJ, "\"rc\": 0") ||
		!strings.Contains(outJ, "\"per_rule\": {}") || !strings.Contains(outJ, "\"skipped\": {}") {
		t.Fatalf("--json 失败：空集合应为 {}（✗ null / ✗ 吞键），得 rc=%d out=%q", rcJ, outJ)
	}

	// ── ⑩ -h 帮助正文逐字（argparse 形）⇒ rc=0 ───────────────────────────
	rcH, outH, _ := cdmRunGate(t, "-h")
	if rcH != 0 || !strings.HasPrefix(outH, "usage: check-doc-meta.py [-h] [--target TARGET]") ||
		!strings.Contains(outH, "optional arguments:") {
		t.Fatalf("-h 失败：得 rc=%d out=%q", rcH, outH)
	}

	// ── ⑪ 未知旗标：argparse 式 usage + error（stderr）⇒ rc=2 ─────────────
	rcB, outB, errB := cdmRunGate(t, "--bogus", "--no-self-test", "--repo-root", root)
	if rcB != 2 || outB != "" || !strings.Contains(errB, "unrecognized arguments: --bogus") {
		t.Fatalf("未知旗标失败：得 rc=%d out=%q stderr=%q", rcB, outB, errB)
	}

	// ── ⑫ --list-rules ⇒ rc=0 且 M1..M16 皆在表 ──────────────────────────
	rcL, outL, _ := cdmRunGate(t, "--list-rules")
	if rcL != 0 || !strings.Contains(outL, "check-doc-meta.py 规则表") ||
		!strings.Contains(outL, "M16  [WARN ] type 与目录默认值不一致") {
		t.Fatalf("--list-rules 失败：得 rc=%d out=%q", rcL, outL)
	}
}
