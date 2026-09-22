// export_schema_test.go —— 帮助导出物 ↔ `*.schema.json` 的对拍（§二十一 已红第 17 条 · 开工单 T-39）。
//
// 已红的病：**全仓零份 `*.schema.json`**（判据 `git ls-files | grep -c "\.schema\.json"` = 0）
// ⇒ 「契约真源 = 代码内 schema」（§九 M16 · §十二 `P-084`/`P-085`/`P-095`）在**导出物这一面**
// 没有任何机器可读落点：导出物的形状只活在渲染函数里，谁改一处都没有东西会红。
//
// 本件把「schema ↔ 导出物」钉成机检（每次 `go test` 都跑，不碰网络、不写文件）：
//
//	ⓐ **键面**：`required ⊂ 真值行 ⊂ properties` —— **两个方向的差集都为空**；
//	ⓑ **取值面**：真值行的 `schema` == schema 里写死的 `const` == 代码里的 `contractSchema`；
//	ⓒ **计数面**：`commands`/`dangerous` 与**导出物 markdown 的表行数**逐数相等（不是各算各的）；
//	ⓓ **层级面**：`layers` 的算术和 = 命令树条数，且 `space`（若出现）必须 = 0（茧壁铁律）；
//	ⓔ **负控**：键差集非空（多加键 / 少 required 键）⇒ ⓐ 的判据**会红**（不是恒绿）。
//
// 真值只有一个来源：`helpExportRow()`（命令面 `zerg help export --json` 也调它）—— 测试不另拼一份。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// exportSchemaRel —— 相对本包目录（`go test` 的 cwd 就是包目录 `core/cmd/zerg`）。
const exportSchemaRel = "../../internal/contract/export.schema.json"

type exportJSONSchema struct {
	Schema               string                     `json:"$schema"`
	ID                   string                     `json:"$id"`
	Title                string                     `json:"title"`
	Type                 string                     `json:"type"`
	AdditionalProperties bool                       `json:"additionalProperties"`
	Required             []string                   `json:"required"`
	Properties           map[string]json.RawMessage `json:"properties"`
}

func loadExportSchema(t *testing.T) *exportJSONSchema {
	t.Helper()
	b, err := os.ReadFile(exportSchemaRel)
	if err != nil {
		t.Fatalf("读不到 schema %s：%v（T-39 的判据① 就落在这个文件上）", exportSchemaRel, err)
	}
	var s exportJSONSchema
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("schema 不是合法 JSON：%v", err)
	}
	return &s
}

// keyProblems —— **键面**判据：required ⊂ 实键 ⊂ properties。返回问题列表（空 = 对拍得上）。
func keyProblems(s *exportJSONSchema, row map[string]string) []string {
	have := map[string]bool{}
	for k := range row {
		have[k] = true
	}
	var probs []string
	for _, k := range s.Required {
		if !have[k] {
			probs = append(probs, "缺 required 键 "+k)
		}
	}
	for k := range row {
		if _, ok := s.Properties[k]; !ok {
			probs = append(probs, "真值行有、schema 未声明的键 "+k)
		}
	}
	for k := range s.Properties {
		if !have[k] {
			probs = append(probs, "schema 声明了、真值行没有的键 "+k)
		}
	}
	sort.Strings(probs)
	return probs
}

// sectionRows 取导出物某两节之间的命令表行（表行一律以 "| `zerg " 开头 ⇒ 计数不靠肉眼看）。
func sectionRows(md, from, to string) []string {
	i := strings.Index(md, from)
	if i < 0 {
		return nil
	}
	rest := md[i+len(from):]
	if j := strings.Index(rest, to); j >= 0 {
		rest = rest[:j]
	}
	var rows []string
	for _, ln := range strings.Split(rest, "\n") {
		if strings.HasPrefix(ln, "| `zerg ") {
			rows = append(rows, ln)
		}
	}
	return rows
}

// TestExportSchemaAgreesWithHelpExport —— ⓐⓑⓒⓓ 四条对拍（T-39 判据②）。
func TestExportSchemaAgreesWithHelpExport(t *testing.T) {
	s := loadExportSchema(t)

	// 前置三件：schema 自己是真 schema（不是个空壳），且**关闭**了对象（否则「未声明键」那条判据是假牙）。
	if s.Schema == "" || s.ID == "" || s.Title == "" {
		t.Fatalf("schema 自身不齐：$schema/%q $id/%q title/%q", s.Schema, s.ID, s.Title)
	}
	if s.Type != "object" {
		t.Fatalf("schema.type 应为 object，实为 %q", s.Type)
	}
	if s.AdditionalProperties {
		t.Fatal("schema 必须 additionalProperties=false —— 否则「真值行多一个键」这条判据没有牙")
	}

	// 真值行：**同一份来源**（命令面也调它），不给测试自己拼 map 的机会。
	// ★ `G-17` ② 起：机器面 = **一行一条命令**（逐条清单）；抽第一行做键面/取值面，整份做逐条清单面。
	const fakePath = "/tmp/导出-命令面帮助-20260920.md"
	facts := helpExportFacts{
		Path:        fakePath,
		Root:        "/tmp/Zerg-内部文档/项目文档",
		DocsVersion: "v9.9.9",
		GeneratedAt: "2026-09-22T00:00:00+08:00",
		Written:     false,
		DryRun:      true,
	}
	rows := helpExportRows(facts)
	if len(rows) != len(catalog()) {
		t.Fatalf("逐条清单行数 %d ≠ 命令树条数 %d（清单与命令树不同源）", len(rows), len(catalog()))
	}
	row := rows[0]

	// ⓐ 键面。
	if probs := keyProblems(s, row); len(probs) > 0 {
		t.Fatalf("ⓐ 键面差集非空（schema 与真值行对不上）：%v", probs)
	}

	// ⓑ 取值面：const 与实际取值、与代码常量三处同值。
	var schemaProp struct {
		Const string `json:"const"`
	}
	if err := json.Unmarshal(s.Properties["schema"], &schemaProp); err != nil {
		t.Fatalf("schema.properties.schema 解析失败：%v", err)
	}
	if schemaProp.Const != contractSchema {
		t.Fatalf("ⓑ schema 写死的 const=%q，代码里的 contractSchema=%q（两处真源漂了）", schemaProp.Const, contractSchema)
	}
	if row["schema"] != contractSchema {
		t.Fatalf("ⓑ 真值行的 schema=%q，应为 %q", row["schema"], contractSchema)
	}

	// ⓒ 计数面：`--json` 的数与**导出物 markdown 的表行数**逐数相等。
	md := renderHelpMarkdown()
	openRows := sectionRows(md, "## 一、命令清单", "## 二、危险动作")
	dangerRows := sectionRows(md, "## 二、危险动作", "## 三、退码表")
	if len(openRows) == 0 || len(dangerRows) == 0 {
		t.Fatalf("导出物节标题没找到（渲染函数改了标题？）· 命令 %d 行 · 危险 %d 行", len(openRows), len(dangerRows))
	}
	if got := strconv.Itoa(len(openRows)); got != row["commands"] {
		t.Fatalf("ⓒ 命令清单表行数 %d ≠ --json 的 commands=%s", len(openRows), row["commands"])
	}
	if got := strconv.Itoa(len(dangerRows)); got != row["dangerous"] {
		t.Fatalf("ⓒ 危险动作表行数 %d ≠ --json 的 dangerous=%s", len(dangerRows), row["dangerous"])
	}
	if n := len(openRows) + len(dangerRows); n != len(catalog()) {
		t.Fatalf("ⓒ 两张表的行数和 %d ≠ 命令树条数 %d（有命令没进导出物）", n, len(catalog()))
	}

	// ⓓ 层级面：形状合规 + 算术和 = 命令树条数 + space 恒为 0。
	var layersProp struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(s.Properties["layers"], &layersProp); err != nil {
		t.Fatalf("schema.properties.layers 解析失败：%v", err)
	}
	if layersProp.Pattern == "" {
		t.Fatal("ⓓ schema 必须给 layers 的 pattern（否则形状没人管）")
	}
	if ok, err := regexp.MatchString(layersProp.Pattern, row["layers"]); err != nil || !ok {
		t.Fatalf("ⓓ layers=%q 不合 schema 的 pattern %q（err=%v）", row["layers"], layersProp.Pattern, err)
	}
	sum := 0
	for _, kv := range strings.Split(row["layers"], ",") {
		p := strings.SplitN(kv, "=", 2)
		if len(p) != 2 {
			t.Fatalf("ⓓ layers 片段解不动：%q", kv)
		}
		n, err := strconv.Atoi(p[1])
		if err != nil {
			t.Fatalf("ⓓ layers 片段不是整数：%q", kv)
		}
		if p[0] == "space" && n != 0 {
			t.Fatalf("ⓓ 茧壁铁律破了：空间（space）内零命令面，实为 %d", n)
		}
		sum += n
	}
	if sum != len(catalog()) {
		t.Fatalf("ⓓ layers 三档之和 %d ≠ 命令树条数 %d", sum, len(catalog()))
	}

	// ⓕ 逐条清单面 + ⓖ 两式同源面（缺口 `G-17` ②：机器面要给**逐条清单 + 危险档档位**，且与人面同源）。
	if probs := catalogProblems(md, rows); len(probs) > 0 {
		t.Fatalf("ⓕ/ⓖ 逐条清单面或两式同源面破：%v", probs)
	}
}

// catalogProblems —— 逐条清单面（ⓕ）+ 两式同源面（ⓖ）的**判定口**（正控与负控都调它 ⇒ 判据不是恒绿）。
//
//	ⓕ 每行：`command` 逐字属于命令树；`is_dangerous`/`opened`/`danger_level`/`confirm_target` 与命令树一致；
//	   `dangerous` 那一格 = 危险行数（逐条数出来的，不是另算一遍）；
//	ⓖ 两式同源：机器面的每一条命令名都能在导出物 markdown 里找到同名表行，**且反方向也成立**
//	   （markdown 的每一行都在机器面里）—— 一个方向的差集非空即报。
func catalogProblems(md string, rows []map[string]string) []string {
	var probs []string
	dangerSeen := 0
	machine := map[string]bool{}
	for _, r := range rows {
		name := r["command"]
		if name == "" {
			probs = append(probs, "有行的 command 是空串")
			continue
		}
		if machine[name] {
			probs = append(probs, "机器面里命令重复："+name)
		}
		machine[name] = true
		c := find(strings.Fields(strings.TrimPrefix(name, "zerg ")))
		if c == nil {
			probs = append(probs, "机器面里有命令树里没有的命令："+name)
			continue
		}
		if c.danger == nil {
			if r["is_dangerous"] != "false" || r["danger_level"] != "—" || r["opened"] != "true" {
				probs = append(probs, fmt.Sprintf("非危险档 %s 的逐条格不对：is_dangerous=%q danger_level=%q opened=%q",
					name, r["is_dangerous"], r["danger_level"], r["opened"]))
			}
			continue
		}
		dangerSeen++
		if r["is_dangerous"] != "true" || r["danger_level"] != c.danger.Level {
			probs = append(probs, fmt.Sprintf("危险档 %s 档位不对：is_dangerous=%q danger_level=%q（命令树 %q）",
				name, r["is_dangerous"], r["danger_level"], c.danger.Level))
		}
		if r["confirm_target"] != c.danger.Target {
			probs = append(probs, fmt.Sprintf("危险档 %s 的 confirm_target=%q ≠ 命令树 %q",
				name, r["confirm_target"], c.danger.Target))
		}
		if want := strconv.FormatBool(c.opened); r["opened"] != want {
			probs = append(probs, fmt.Sprintf("危险档 %s 的 opened=%q ≠ 命令树 %q", name, r["opened"], want))
		}
	}
	if len(rows) > 0 {
		if want := strconv.Itoa(dangerSeen); rows[0]["dangerous"] != want {
			probs = append(probs, fmt.Sprintf("逐条数出来的危险档 %d 条 ≠ 摘要格 dangerous=%q", dangerSeen, rows[0]["dangerous"]))
		}
	}
	// ⓖ 两式同源（两个方向的差集都要空）。
	mdRows := append(sectionRows(md, "## 一、命令清单", "## 二、危险动作"),
		sectionRows(md, "## 二、危险动作", "## 三、退码表")...)
	mdSet := map[string]bool{}
	for _, ln := range mdRows {
		if i := strings.Index(ln, "`zerg "); i >= 0 {
			rest := ln[i+1:]
			if j := strings.Index(rest[1:], "`"); j >= 0 {
				mdSet[rest[:j+1]] = true
			}
		}
	}
	for name := range machine {
		if !mdSet[name] {
			probs = append(probs, "机器面有、导出物 markdown 没有的命令："+name)
		}
	}
	for name := range mdSet {
		if !machine[name] {
			probs = append(probs, "导出物 markdown 有、机器面没有的命令："+name)
		}
	}
	sort.Strings(probs)
	return probs
}

// TestExportSchemaKeyJudgeHasTeeth —— ⓔ 负控：判据不是恒绿。
func TestExportSchemaKeyJudgeHasTeeth(t *testing.T) {
	s := loadExportSchema(t)
	good := helpExportRows(helpExportFacts{
		Path: "/tmp/x.md", Root: "/tmp/Zerg-内部文档/项目文档", DocsVersion: "v9.9.9",
		GeneratedAt: "2026-09-22T00:00:00+08:00", Written: true,
	})
	if len(good) == 0 {
		t.Fatal("真值行一份都没有（命令树空？）")
	}

	// 正控：好行 + 好清单 ⇒ 两个判定口都无问题。
	if probs := keyProblems(s, good[0]); len(probs) != 0 {
		t.Fatalf("正控都不绿（好行应无问题）：%v", probs)
	}
	if probs := catalogProblems(renderHelpMarkdown(), good); len(probs) != 0 {
		t.Fatalf("正控都不绿（逐条清单面/同源面应无问题）：%v", probs)
	}

	// 负控①：真值行**多一个** schema 没声明的键 ⇒ 必报。
	extra := map[string]string{}
	for k, v := range good[0] {
		extra[k] = v
	}
	extra["surprise"] = "1"
	if probs := keyProblems(s, extra); len(probs) == 0 {
		t.Fatal("负控① 未报：多出一个未声明键竟然通过了（判据是假牙）")
	}

	// 负控②：抽掉一个 required 键 ⇒ 必报。
	for _, k := range s.Required {
		short := map[string]string{}
		for kk, vv := range good[0] {
			if kk != k {
				short[kk] = vv
			}
		}
		if probs := keyProblems(s, short); len(probs) == 0 {
			t.Fatalf("负控② 未报：抽掉 required 键 %q 竟然通过了", k)
		}
	}

	// 负控③：清单**少一行**（抽掉末条）⇒ 逐条清单面必报（`G-17` ② 的那条判据不是恒绿）。
	if probs := catalogProblems(renderHelpMarkdown(), good[:len(good)-1]); len(probs) == 0 {
		t.Fatal("负控③ 未报：清单少一行竟然通过了")
	}

	// 负控④：把末条的档位改一个字 ⇒ 逐条档位面必报。
	bad := make([]map[string]string, len(good))
	for i, r := range good {
		cp := map[string]string{}
		for k, v := range r {
			cp[k] = v
		}
		bad[i] = cp
	}
	last := bad[len(bad)-1]
	if last["is_dangerous"] == "true" {
		last["danger_level"] = "D9"
	} else {
		last["danger_level"] = "D3"
	}
	if probs := catalogProblems(renderHelpMarkdown(), bad); len(probs) == 0 {
		t.Fatal("负控④ 未报：档位改错竟然通过了")
	}
}
