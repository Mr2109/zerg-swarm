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
	const fakePath = "/tmp/导出-命令面帮助-20260920.md"
	row := helpExportRow(fakePath)

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
}

// TestExportSchemaKeyJudgeHasTeeth —— ⓔ 负控：判据不是恒绿。
func TestExportSchemaKeyJudgeHasTeeth(t *testing.T) {
	s := loadExportSchema(t)
	good := helpExportRow("/tmp/x.md")

	if probs := keyProblems(s, good); len(probs) != 0 {
		t.Fatalf("正控都不绿（好行应无问题）：%v", probs)
	}

	// 负控①：真值行**多一个** schema 没声明的键 ⇒ 必报。
	extra := map[string]string{}
	for k, v := range good {
		extra[k] = v
	}
	extra["surprise"] = "1"
	if probs := keyProblems(s, extra); len(probs) == 0 {
		t.Fatal("负控① 未报：多出一个未声明键竟然通过了（判据是假牙）")
	}

	// 负控②：抽掉一个 required 键 ⇒ 必报。
	for _, k := range s.Required {
		short := map[string]string{}
		for kk, vv := range good {
			if kk != k {
				short[kk] = vv
			}
		}
		if probs := keyProblems(s, short); len(probs) == 0 {
			t.Fatalf("负控② 未报：抽掉 required 键 %q 竟然通过了", k)
		}
	}
}
