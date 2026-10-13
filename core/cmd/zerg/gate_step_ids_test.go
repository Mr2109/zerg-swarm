// gate_step_ids_test.go —— 步的**稳定短 id**（缺口 `GAP-20260928-68`）的成对判据 ＋ 仓外夹具入口。
//
// 病（改前 · 现读）：门步骤名只有**人读串**（含空格 / 冒号 / CJK / 反引号），没有一个**稳定短 id**
// ⇒ `gate run --step` / `--fast` / 脚本的 `filter_step` 一律**按名逐字精确匹配**，工具与脚本没有
// 可稳定引用的句柄（本夜两次抄错步名）；而拿**行号 / 序号**当 id 必漂（步骤表常插常删）。
//
// 治法（本片 · 判形态）：短 id = `s` ＋ `sha256(normStepName(步名))` 前 4 字节 hex（`gate.go` 的
// `gateStepShortID`）—— **步名派生**（✗ 不含行号/序号/表内位置）⇒ 插一行、删一行、重排都不动其余步的
// id；「步名 ↦ 短 id」因此是一张**映射**（实时面 = `zerg gate ls --json id`），并**落件**
// `testdata/gate-step-ids.json`（真步名的冻结向量）。本文件钉四件：
//
//	① 冻结向量**自洽**：每条 `id == gateStepShortID(名)` ⇒ 改了派生算法 / 手改冻结件**当场红**
//	   （短 id 不许漂 —— 这正是「落件」的牙）；
//	② 冻结向量里**仍在步骤表里**的名，其 id 必须**逐字相符**；表里已没有的名只**报告不阻断**
//	   （✗ 不把夹具钉在会被正常开发改动的真源件上：加步 / 删步不该让本用例红）；
//	③ **插 / 删一行后短 id 不变**：同一批步名，在「原表 / 插一行（插在中间）/ 删一行」三张表下逐字相同；
//	   并配**负控**：把 id 换成行号派生的 `n<行号>` 同一条比对照样红 ⇒ 证明本夹具的比对**有牙**；
//	④ **同名步 ⇒ 报冲突**（✗ 不静默取首）+ 成对负控（不重复的表 ⇒ 不报冲突）。
//
// 重生成冻结向量：go test ./cmd/zerg -run TestGateStepIDs_FrozenVectors -count=1 -update-step-ids
//
//	仓外夹具（拿仓外步骤表现算映射）：go test ./cmd/zerg -run TestGateStepIDs_TableFixture -count=1 \
//	                                         -step-table <仓外步骤表> -step-ids-out <落点.json>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var (
	gateStepIDsUpdate = flag.Bool("update-step-ids", false, "重生成 testdata/gate-step-ids.json（真步名的冻结向量）")
	gateStepIDsTable  = flag.String("step-table", "", "拿它当步骤表真源（仓外夹具用 · 缺省 = 仓内真源）")
	gateStepIDsOut    = flag.String("step-ids-out", "", "把现算的「步名 ↦ 短 id」映射写到这个件（仓外夹具用）")
)

// gateStepIDFrozenRel —— 冻结向量件的**包内相对**路径（真源 = 本包 `testdata/`）。
const gateStepIDFrozenRel = "testdata/gate-step-ids.json"

// gateStepIDFrozenNote —— 冻结向量件头部的口径自述（**手改会被现算值对拍判红**）。
const gateStepIDFrozenNote = "步的**稳定短 id**（缺口 GAP-20260928-68）真步名**冻结向量**。id = `s` ＋ " +
	"sha256(normStepName(步名)) 前 4 字节 hex（派生真源 = core/cmd/zerg/gate.go 的 gateStepShortID）——" +
	"步名派生 ⇒ **插/删一行不影响其余步的 id**（✗ 不用行号/序号当 id）。本件由 `go test ./cmd/zerg " +
	"-run TestGateStepIDs_FrozenVectors -count=1 -update-step-ids` 现算生成（✗ 不许手改）：机检两条 —— " +
	"① 每条 id == 现算值（改了算法即红）② 本表里仍在步骤表里的名，其 id 必须逐字相符。"

// gateStepIDFrozenDoc —— 冻结向量件形状（`steps` 按步名序）。
type gateStepIDFrozenDoc struct {
	Schema        string               `json:"schema"`
	Note          string               `json:"note"`
	GeneratedFrom string               `json:"generated_from"`
	Steps         []gateStepIDFrozenKV `json:"steps"`
}

// gateStepIDFrozenKV —— 一条向量：`name` = **归一后步名**（`normStepName` · 与脚本 `filter_step` 同一串空间）。
type gateStepIDFrozenKV struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// gateStepIDsReadFrozen —— 读冻结向量件（读不到 ⇒ 判不了，不许当绿）。
func gateStepIDsReadFrozen(t *testing.T, path string) gateStepIDFrozenDoc {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("冻结向量件读不到（%s）：%v ⇒ 判不了，不许当绿", path, err)
	}
	var doc gateStepIDFrozenDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("冻结向量件解不动（%s）：%v", path, err)
	}
	if doc.Schema == "" || len(doc.Steps) == 0 {
		t.Fatalf("冻结向量件形状不对（%s）：schema=%q steps=%d", path, doc.Schema, len(doc.Steps))
	}
	return doc
}

// gateStepIDFrozenJSON —— 现算结果 ⇒ 冻结向量件正文（人读缩进 + 稳定序）。
func gateStepIDFrozenJSON(from string, rows []gateStepIDFrozenKV) string {
	rows = append([]gateStepIDFrozenKV{}, rows...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	b, _ := json.MarshalIndent(gateStepIDFrozenDoc{
		Schema:        "gate-step-ids/1",
		Note:          gateStepIDFrozenNote,
		GeneratedFrom: from,
		Steps:         rows,
	}, "", " ")
	return string(b) + "\n"
}

// gateStepIDTableSource —— 步骤表真源文本：缺省 = 仓内真源（`repoRoot()` ＋ `gateScriptRel`，与
// `gate ls` / `gate find` / `gate explain` / `gate run --step` **同一处**）；仓外夹具可用 `-step-table` 指别的件。
// 读不到 / 解析不到仓根 ⇒ **判不了**（fail-closed），✗ 不拿空表当读数。
func gateStepIDTableSource(t *testing.T) (string, string) {
	t.Helper()
	if *gateStepIDsTable != "" {
		b, err := os.ReadFile(*gateStepIDsTable)
		if err != nil {
			t.Fatalf("-step-table %s 读不到：%v ⇒ 判不了，不许当绿", *gateStepIDsTable, err)
		}
		return string(b), *gateStepIDsTable
	}
	root := repoRoot()
	if root == "" {
		t.Fatalf("解析不到仓根（找 core/internal/version/version.go 失败）⇒ 判不了，不许当绿")
	}
	p := filepath.Join(root, gateScriptRel)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("门禁步骤表真源读不到（%s）：%v ⇒ 判不了，不许当绿", gateScriptRel, err)
	}
	return string(b), gateScriptRel
}

// gateStepIDsFromSource —— 步骤表文本 ⇒ 现算的「步名 ↦ 短 id」行（每条 `add_step` 一行 · 按步名序）。
//
// 解析**复用**仓内既有那一枚（`parseAddStepsStrict` —— 与 `gate find`/`gate explain`/`gate run-step`
// 同一处 · ✗ 不另造第二套解析器）；「读不准」的声明照同族口径**上报**（✗ 不静默丢弃）；
// 名一律走 `normStepName`（与脚本 `filter_step` 去反斜杠同口径）⇒ 源文形态与 `--list` 形态同键。
func gateStepIDsFromSource(t *testing.T, src string) []gateStepIDFrozenKV {
	t.Helper()
	decls, unc := parseAddStepsStrict(src)
	reportUncertainSteps(os.Stderr, unc)
	out := make([]gateStepIDFrozenKV, 0, len(decls))
	for _, d := range decls {
		out = append(out, gateStepIDFrozenKV{ID: gateStepShortID(d.Name), Name: normStepName(d.Name)})
	}
	return out
}

// gateStepIDsByID —— 名 ⇒ 短 id（同名步 ⇒ 同 id · 这里只用来对账，撞车另判）。
func gateStepIDsByID(rows []gateStepIDFrozenKV) map[string]string {
	m := map[string]string{}
	for _, r := range rows {
		m[r.Name] = r.ID
	}
	return m
}

// ① 冻结向量：自洽（每条 id == 现算值）+ 仍在步骤表里的名逐字相符（表里已没有的名只报告不阻断）。
func TestGateStepIDs_FrozenVectors(t *testing.T) {
	src, from := gateStepIDTableSource(t)
	live := gateStepIDsFromSource(t, src)
	if *gateStepIDsUpdate {
		if err := os.WriteFile(gateStepIDFrozenRel, []byte(gateStepIDFrozenJSON(from, live)), 0o644); err != nil {
			t.Fatalf("重生成 %s 失败：%v", gateStepIDFrozenRel, err)
		}
		t.Logf("已重生成冻结向量：%s（%d 步 · 真源 %s）", gateStepIDFrozenRel, len(live), from)
		return
	}
	frozen := gateStepIDsReadFrozen(t, gateStepIDFrozenRel)
	// ① 自洽：每条 id 必须等于**现算值**（改算法 / 手改冻结件 ⇒ 当场红）。
	byID := map[string]bool{}
	for _, kv := range frozen.Steps {
		if got := gateStepShortID(kv.Name); got != kv.ID {
			t.Errorf("冻结向量自洽破：名 %q 登记 id=%s，现算=%s（派生算法被改或冻结件被手改 ⇒ 短 id 漂了）",
				kv.Name, kv.ID, got)
		}
		if byID[kv.ID] {
			t.Errorf("冻结向量自洽破：id %s 登记了不止一条（同一 id 不许落在两个名上）", kv.ID)
		}
		byID[kv.ID] = true
	}
	// ② 真步名的 id 不许漂；表里已没有的名**只报告**（加步/删步属正常开发 ⇒ ✗ 不据此阻断）。
	liveByName := gateStepIDsByID(live)
	covered, stale := 0, 0
	for _, kv := range frozen.Steps {
		if id, ok := liveByName[kv.Name]; ok {
			covered++
			if id != kv.ID {
				t.Errorf("真步名的短 id 漂了：%q 现算=%s，冻结件登记=%s", kv.Name, id, kv.ID)
			}
			continue
		}
		stale++
		t.Logf("只报告：冻结键 %q 现读步骤表里已没有（加步/删步属正常开发 ⇒ 本用例不据此阻断）", kv.Name)
	}
	t.Logf("冻结向量 %d 条 · 仍在步骤表 %d · 表里已没有 %d · 真源 %s（现读 add_step 行 %d 条）",
		len(frozen.Steps), covered, stale, from, len(live))
}

// ② 插 / 删一行后短 id 不变（+ 负控：行号派生的 id 同一条比对照样红 ⇒ 比对有牙）。
//
// 合成表只判「位置无关」这一件性质（✗ 不当真源、✗ 不代表真表 —— 真表在门脚本里）；步名取**真表的形态**
// （空格 / 冒号 / CJK / 括号 / `/` 全套），因为 id 的输入就是这串。
const gateStepIDFixtureBase = `add_step gates "门① 覆盖：构建清单目录 + 脚本接线（阻断）" tri "${R}" "x.py"
add_step core "core: go test ./... -count=1" rc "${R}" "go test"
add_step docs "docs: 未代入占位符 = 0（产物句面 · 两值 · 阻断）" tri "${R}" "y.py"
add_step rust "cargo fmt --check" rc "${R}" "cargo fmt"
add_step public "scripts/*.sh + scripts/*/*.sh 语法（bash -n）" rc "${R}" "bash -n"
`

// gateStepIDFixtureInserted —— 原表**中间**插一行（行号派生的 id 必然在这里翻车）。
const gateStepIDFixtureInserted = `add_step gates "门① 覆盖：构建清单目录 + 脚本接线（阻断）" tri "${R}" "x.py"
add_step core "core: go test ./... -count=1" rc "${R}" "go test"
add_step extra "新插的一步（插在中间）" rc "${R}" "true"
add_step docs "docs: 未代入占位符 = 0（产物句面 · 两值 · 阻断）" tri "${R}" "y.py"
add_step rust "cargo fmt --check" rc "${R}" "cargo fmt"
add_step public "scripts/*.sh + scripts/*/*.sh 语法（bash -n）" rc "${R}" "bash -n"
`

// gateStepIDFixtureDeleted —— 原表删一行（docs 那一步）。
const gateStepIDFixtureDeleted = `add_step gates "门① 覆盖：构建清单目录 + 脚本接线（阻断）" tri "${R}" "x.py"
add_step core "core: go test ./... -count=1" rc "${R}" "go test"
add_step rust "cargo fmt --check" rc "${R}" "cargo fmt"
add_step public "scripts/*.sh + scripts/*/*.sh 语法（bash -n）" rc "${R}" "bash -n"
`

func TestGateStepShortID_StableUnderRowInsertDelete(t *testing.T) {
	base := gateStepIDsFromSource(t, gateStepIDFixtureBase)
	ins := gateStepIDsFromSource(t, gateStepIDFixtureInserted)
	del := gateStepIDsFromSource(t, gateStepIDFixtureDeleted)
	if len(base) != 5 || len(ins) != 6 || len(del) != 4 {
		t.Fatalf("合成表解析条数不对：原 %d（要 5）· 插 %d（要 6）· 删 %d（要 4）—— 解析器没吃下全部声明",
			len(base), len(ins), len(del))
	}
	// 负控（行号当 id —— 改前的错形态）：同一条比对**必须红**，否则本夹具的比对没有牙。
	lineID := func(rows []gateStepIDFrozenKV, name string) string {
		for i, r := range rows {
			if r.Name == name {
				return fmt.Sprintf("n%d", i+1)
			}
		}
		return "（表里没有）"
	}
	negCtlRed := false
	for _, r := range base {
		if lineID(base, r.Name) != lineID(ins, r.Name) {
			negCtlRed = true
			break
		}
	}
	if !negCtlRed {
		t.Fatalf("负控不红：行号派生的 id 在「原表 ⟷ 插一行的表」之间没变化 ⇒ 本夹具的比对无牙（✗ 不许静默通过）")
	}
	// 正控：三条表里**共有的**步名，派生短 id 必须逐字相同。
	insByName := gateStepIDsByID(ins)
	delByName := gateStepIDsByID(del)
	common := 0
	for _, r := range base {
		id := gateStepShortID(r.Name)
		if got, ok := insByName[r.Name]; ok {
			common++
			if got != id {
				t.Errorf("插一行后短 id 变了：%q 原=%s 插后=%s", r.Name, id, got)
			}
		}
		if got, ok := delByName[r.Name]; ok {
			if got != id {
				t.Errorf("删一行后短 id 变了：%q 原=%s 删后=%s", r.Name, id, got)
			}
		}
	}
	if common == 0 {
		t.Fatalf("合成表之间没有共有步名 ⇒ 这条判据空转")
	}
	t.Logf("插/删一行后短 id 逐字不变：共有步名 %d 个（负控「行号当 id」同一条比对照样红 ⇒ 有牙）", common)
}

// ③ 同名步 ⇒ 响亮判冲突（✗ 不静默取首）；成对负控：不重复的表 ⇒ 不报冲突。
func TestGateStepIDs_AmbiguousSameNameConflicts(t *testing.T) {
	dupRows := []gateStepIDRow{
		{Scope: "go", Mode: "empty", Name: "gofmt -l core"},
		{Scope: "gates", Mode: "tri", Name: "gofmt -l core"},
	}
	conf := gateStepIDConflictReports(dupRows)
	if len(conf) == 0 {
		t.Fatalf("同名步 ⇒ 必须报冲突（✗ 不静默取首）")
	}
	if !strings.Contains(conf[0], "gofmt -l core") || !strings.Contains(conf[0], "2 步") {
		t.Errorf("撞车那条要点名「同一个短 id 上有哪几步」：%q", conf[0])
	}
	if !strings.Contains(conf[0], gateStepShortID("gofmt -l core")) {
		t.Errorf("撞车那条要打出那个撞了的短 id：%q", conf[0])
	}
	// 负控：不重复的表 ⇒ 零冲突（哑化「撞车检测」为恒报，本负控必红）。
	uniq := []gateStepIDRow{
		{Scope: "go", Mode: "empty", Name: "gofmt -l core"},
		{Scope: "gates", Mode: "tri", Name: "门① 覆盖（阻断）"},
	}
	if c2 := gateStepIDConflictReports(uniq); len(c2) != 0 {
		t.Errorf("不重复的表 ⇒ 不该报冲突：%q", c2)
	}
	// 反例的**接缝**：异名同 id（哈希撞车）也走同一条判据 —— 这里用「同名归一后同名」的等价形态钉住
	// （转义形态与裸形态经 `normStepName` 归一后同名 ⇒ 同 id）。
	esc := gateStepIDRow{Scope: "go", Mode: "empty", Name: "core: a \\`b\\`"}
	bare := gateStepIDRow{Scope: "go", Mode: "empty", Name: "core: a `b`"}
	if gateStepShortID(esc.Name) != gateStepShortID(bare.Name) {
		t.Errorf("归一后同名的两种写法（转义 ⟷ 裸）必须派生出同一个短 id：%s ⟷ %s",
			gateStepShortID(esc.Name), gateStepShortID(bare.Name))
	}
	if c3 := gateStepIDConflictReports([]gateStepIDRow{esc, bare}); len(c3) == 0 {
		t.Errorf("转义形态与裸形态撞在同一个 id 上 ⇒ 同一条判据也要报（✗ 不静默取首）")
	}
}

// ④ 仓外夹具入口：拿 `-step-table <仓外步骤表>` 现算「步名 ↦ 短 id」映射，写到 `-step-ids-out <件>`。
// 两个旗标都不给 ⇒ 只做一次现读烟测（打出行数与真源），✗ 不写任何件。
func TestGateStepIDs_TableFixture(t *testing.T) {
	src, from := gateStepIDTableSource(t)
	rows := gateStepIDsFromSource(t, src)
	if *gateStepIDsOut == "" {
		t.Logf("烟测（未给 -step-ids-out ⇒ 不写件）：现读 %s 得 %d 步（短 id 形如 %s）",
			from, len(rows), gateStepShortID(rows[0].Name))
		return
	}
	if err := os.WriteFile(*gateStepIDsOut, []byte(gateStepIDFrozenJSON(from, rows)), 0o644); err != nil {
		t.Fatalf("写 %s 失败：%v", *gateStepIDsOut, err)
	}
	t.Logf("仓外夹具：%d 步的「步名 ↦ 短 id」映射已写到 %s（真源 %s）", len(rows), *gateStepIDsOut, from)
}
