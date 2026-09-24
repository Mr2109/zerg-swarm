// cli_truncated_detail_test.go —— `meta.truncated_detail`（块D `K-1` · `O-10` · 缺口 `G-81`）的判据。
//
// 本批做什么（一句话）：`truncated=true` 时**同时**给计数 —— 走 `meta` 的**按需子键**
// `meta.truncated_detail`（顶层**仍是那六键**，一个不多一个不少 ✗），形状照块D `K-1` 写死：
//
//	{"cut_from":"head|middle|tail","kept_items":<int>,"dropped_items":<int>,"total_items":<int>}
//
// 单位**一律「条目」** · **不报 token 估算** ✗（我们有真值，不必估）· **不给下标 / 偏移** ✗
// · **不含续读入口** ✗（取回是 `meta.how_to_restore` 那一格的事）· **三数自校**
// （`kept_items + dropped_items == total_items`）。
//
// 判据（**每条配正控 + 成对负控** · 判定口 = `truncatedDetailJudge` 一处）：
//
//	判据① 与 `truncated` 成对    正控 = 真裁 ⇒ 四键在 · 负控 = 没裁 ⇒ 缺席 / 只给布尔 ⇒ 红
//	判据② 四键齐、一个不多不少    负控 = 缺一键 / 多一键 ⇒ 红
//	判据③ 三数自校 + 真裁非零     负控 = `kept+dropped ≠ total` ⇒ 红 · `dropped=0` ⇒ 红
//	判据④ `cut_from` 三值闭集     负控 = 闭集外的值（`side`）⇒ 红
//	判据⑤ 同源同值（块D `K-6`）   正控 = `dropped_items` 与 `warnings[]`「已裁 N 条」的 N 同值
//	                              · `kept_items` 与 `items[]` 条数同值（同一处取值 ⇒ 不漂）
//
// ★ 落点：`package main_test`（外部测试包）—— `RunForTest` 跑**当前源码**，不是盘上旧制品。
package main_test

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// truncatedDetailForTest 从包封文本里取出 `meta.truncated_detail` 四格（缺席 ⇒ ok=false）。
func truncatedDetailForTest(t *testing.T, envJSON string) (map[string]json.RawMessage, bool) {
	t.Helper()
	var env struct {
		Meta map[string]json.RawMessage `json:"meta"`
	}
	if err := json.Unmarshal([]byte(envJSON), &env); err != nil {
		t.Fatalf("包封不是 JSON：%v · %q", err, tail(envJSON, 300))
	}
	raw, ok := env.Meta["truncated_detail"]
	if !ok {
		return nil, false
	}
	var d map[string]json.RawMessage
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("`meta.truncated_detail` 不是对象：%v · %s", err, string(raw))
	}
	return d, true
}

// cutWarningCount 「已裁 N 条」里的 N（找不到 ⇒ -1）。
var cutWarningRe = regexp.MustCompile(`^已裁 ([0-9]+) 条$`)

func cutWarningCount(warns []string) int {
	for _, w := range warns {
		if m := cutWarningRe.FindStringSubmatch(strings.TrimSpace(w)); m != nil {
			n, _ := strconv.Atoi(m[1])
			return n
		}
	}
	return -1
}

// synthDetail 造一份合成 detail（负控用 · 键序无关）。
func synthDetail(cutFrom string, kept, dropped, total int) string {
	return `{"cut_from":"` + cutFrom + `","kept_items":` + strconv.Itoa(kept) +
		`,"dropped_items":` + strconv.Itoa(dropped) + `,"total_items":` + strconv.Itoa(total) + `}`
}

func TestTruncatedDetail_JudgePairsWithBoolean(t *testing.T) {
	// ---- 合成面：判定口的正控与**五条负控**（不碰仓 · 纯内存）-------------------------------
	okEnv := zerg.EnvelopeRenderForTest("[{\"a\":\"1\"}]", 1, []string{"已裁 2 条"}, true,
		[][2]string{{"truncated_detail", synthDetail("tail", 1, 2, 3)}})
	if err := zerg.TruncatedDetailJudgeForTest(okEnv); err != nil {
		t.Errorf("正控失败（合成 · 四键齐 + 三数自校）：%v", err)
	}

	// 负控①（本批的正面靶子）：**只给布尔** ⇒ 必红。
	onlyBool := zerg.EnvelopeRenderForTest("[]", 0, []string{"已裁 1 条"}, true, nil)
	if err := zerg.TruncatedDetailJudgeForTest(onlyBool); err == nil {
		t.Error("负控①失败：`truncated=true` 却只给布尔，判定口竟判过（块D `K-1` 要的就是这一格红）")
	} else {
		t.Logf("负控①判红（原样）：%v", err)
	}

	// 负控②：缺一键（抽掉 `total_items`）。
	fewer := zerg.EnvelopeRenderForTest("[]", 0, nil, true, [][2]string{
		{"truncated_detail", `{"cut_from":"tail","kept_items":1,"dropped_items":2}`}})
	if err := zerg.TruncatedDetailJudgeForTest(fewer); err == nil {
		t.Error("负控②失败：四键缺一竟判过")
	}
	// 负控②b：多一键。
	more := zerg.EnvelopeRenderForTest("[]", 0, nil, true, [][2]string{
		{"truncated_detail", `{"cut_from":"tail","kept_items":1,"dropped_items":2,"total_items":3,"tokens":9}`}})
	if err := zerg.TruncatedDetailJudgeForTest(more); err == nil {
		t.Error("负控②b 失败：多了一键竟判过（四键一个不多一个不少）")
	}
	// 负控③：三数不自校。
	badSum := zerg.EnvelopeRenderForTest("[]", 0, nil, true,
		[][2]string{{"truncated_detail", synthDetail("tail", 1, 2, 9)}})
	if err := zerg.TruncatedDetailJudgeForTest(badSum); err == nil {
		t.Error("负控③失败：`kept+dropped ≠ total` 竟判过")
	}
	// 负控③b：真裁却 `dropped_items=0`（裁了就得说裁了几条）。
	zeroDrop := zerg.EnvelopeRenderForTest("[]", 0, nil, true,
		[][2]string{{"truncated_detail", synthDetail("tail", 3, 0, 3)}})
	if err := zerg.TruncatedDetailJudgeForTest(zeroDrop); err == nil {
		t.Error("负控③b 失败：`dropped_items=0` 竟判过")
	}
	// 负控④：`cut_from` 不在三值闭集内。
	offSet := zerg.EnvelopeRenderForTest("[]", 0, nil, true,
		[][2]string{{"truncated_detail", synthDetail("side", 1, 2, 3)}})
	if err := zerg.TruncatedDetailJudgeForTest(offSet); err == nil {
		t.Error("负控④失败：`cut_from=\"side\"` 竟判过（闭集外必须红）")
	}
	// 负控⑤：没裁却给计数（余量不是「裁了」）。
	notCut := zerg.EnvelopeRenderForTest("[]", 0, nil, false,
		[][2]string{{"truncated_detail", synthDetail("tail", 1, 2, 3)}})
	if err := zerg.TruncatedDetailJudgeForTest(notCut); err == nil {
		t.Error("负控⑤失败：`truncated=false` 却写了计数竟判过（缺席 ≠ 假值）")
	}

	// ---- 静态自检：第七键 / 保留名单 / 不报 token -------------------------------------------
	keys, cutSet := zerg.TruncatedDetailKeysForTest()
	if len(keys) != 4 {
		t.Errorf("四键真源条数 = %d（要 4）：%v", len(keys), keys)
	}
	if strings.Join(cutSet, "|") != "head|middle|tail" {
		t.Errorf("`cut_from` 三值闭集 = %v（要 head|middle|tail）", cutSet)
	}
	for _, k := range zerg.EnvelopeKeysForTest() {
		if k == "truncated_detail" {
			t.Error("红线破：`truncated_detail` 跑到**顶层**去了（顶层仍是那六键 · 一个不多）")
		}
	}
	for _, r := range zerg.EnvelopeMetaReservedForTest() {
		if r == "truncated_detail" {
			t.Errorf("`truncated_detail` 被旧子键保留名单挡住（%v）⇒ 写不进去", zerg.EnvelopeMetaReservedForTest())
		}
	}
	for _, k := range keys {
		if strings.Contains(strings.ToLower(k), "token") {
			t.Errorf("四键里有 token 面（`%s`）—— 块D `K-1` 逐字：**不报 token 估算** ✗", k)
		}
	}
}

func TestTruncatedDetail_RealRunPairsWithCutCount(t *testing.T) {
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	// 正控：真被裁的件（六层全量 > 卡片上限 ⇒ 卡片块自己报真裁）。
	rc, out, errb := runCapture("impact", "core/cmd/zerg/main.go", "--json", "what,why")
	if rc != 0 && rc != 1 {
		t.Fatalf("准备态不对：`impact core/cmd/zerg/main.go --json what,why` 退码 = %d（要 0/1 档真跑）· stderr=%s",
			rc, tail(errb, 400))
	}
	if !strings.Contains(errb, "本跑 truncated=true") {
		t.Fatalf("准备态不对：卡片块没报真裁（stderr 里找不到「本跑 truncated=true」）—— 换一个更大的件再判：%s",
			tail(errb, 400))
	}
	if err := zerg.TruncatedDetailJudgeForTest(out); err != nil {
		t.Fatalf("正控①失败：真裁了这一跑却判不过：%v · 包封=%s", err, tail(out, 400))
	}
	d, ok := truncatedDetailForTest(t, out)
	if !ok {
		t.Fatal("正控①失败：真裁了却没有 `meta.truncated_detail`（三数没落进包封）")
	}
	var env struct {
		Items    []map[string]string `json:"items"`
		Warnings []string            `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("包封解析失败：%v", err)
	}
	var kept, dropped, total int
	_ = json.Unmarshal(d["kept_items"], &kept)
	_ = json.Unmarshal(d["dropped_items"], &dropped)
	_ = json.Unmarshal(d["total_items"], &total)

	// 判据⑤（块D `K-6` · **同源同值**）：`kept_items` ⇄ `items[]` 条数 · `dropped_items` ⇄「已裁 N 条」的 N。
	if kept != len(env.Items) {
		t.Errorf("判据⑤失败：`kept_items` = %d 而包封 `items[]` 有 %d 条（同一处取值 ⇒ 必须同值）",
			kept, len(env.Items))
	}
	if n := cutWarningCount(env.Warnings); n != dropped {
		t.Errorf("判据⑤失败：`dropped_items` = %d 而 `warnings[]` 里「已裁 N 条」的 N = %d（同源 ⇒ 必须同值）· warnings=%v",
			dropped, n, env.Warnings)
	}
	if kept+dropped != total {
		t.Errorf("判据③失败：三数不自校 %d + %d ≠ %d", kept, dropped, total)
	}
	t.Logf("真跑四格逐字：cut_from=%s kept_items=%d dropped_items=%d total_items=%d（items[]=%d 条 · 已裁 N 条 N=%d）",
		strings.Trim(string(d["cut_from"]), `"`), kept, dropped, total, len(env.Items), cutWarningCount(env.Warnings))

	// 成对负控（**没裁**）：零命中件 ⇒ `truncated=false` 且本格**缺席**（缺席 ≠ 假值）。
	rc, out, _ = runCapture("impact", impactZeroHitTarget(), "--json", "what")
	if rc != 1 {
		t.Fatalf("准备态不对：零命中件退码 = %d（要 1）", rc)
	}
	if _, ok := truncatedDetailForTest(t, out); ok {
		t.Errorf("负控①失败：一条都没裁却写了 `meta.truncated_detail`（余量不是「裁了」）· 包封=%s", tail(out, 300))
	}
	if err := zerg.TruncatedDetailJudgeForTest(out); err != nil {
		t.Errorf("负控①失败：没裁那一跑判不过：%v", err)
	}
	// 成对负控（**另一条命令**）：非裁命令一句都不许有本格。
	_, out, _ = runCapture("version", "--json", "name")
	if _, ok := truncatedDetailForTest(t, out); ok {
		t.Errorf("负控②失败：`version` 这跑了 `meta.truncated_detail`（本格只归真裁的那条命令）· 包封=%s", tail(out, 300))
	}
}

func TestTruncatedDetail_CardLevelSameSource(t *testing.T) {
	// 卡片面正控（纯函数 · 不碰仓）：13 条合法条目 ⇒ 条数上限 12 ⇒ 必裁 1 条；
	// `dropped_items` 与「已裁 N 条」那份取值同源（同一处 `impactCardBudget`）。
	rows := []map[string]string{}
	for i := 0; i < 13; i++ {
		rows = append(rows, map[string]string{
			"what": "core/cmd/zerg/main.go",
			"why":  "词法",
			"how":  "第 " + strconv.Itoa(i) + " 条",
			"red":  "",
		})
	}
	card := zerg.ImpactCardOfForTest(rows, "", "core/cmd/zerg/main.go")
	if !card.Truncated {
		t.Fatal("准备态不对：13 条 > 上限 12 却不裁")
	}
	detail, ok := zerg.ImpactCardBudgetDetailForTest(card)
	if !ok {
		t.Fatal("卡片面失败：裁了却给不出 `truncated_detail`（ok=false）")
	}
	var d map[string]json.RawMessage
	if err := json.Unmarshal([]byte(detail), &d); err != nil {
		t.Fatalf("卡片面 `truncated_detail` 不是对象：%v · %s", err, detail)
	}
	var kept, dropped, total int
	_ = json.Unmarshal(d["kept_items"], &kept)
	_ = json.Unmarshal(d["dropped_items"], &dropped)
	_ = json.Unmarshal(d["total_items"], &total)
	if kept != len(card.Items) {
		t.Errorf("卡片面：`kept_items` = %d 而卡片留下 %d 条", kept, len(card.Items))
	}
	if n := cutWarningCount(card.Warnings); n != dropped {
		t.Errorf("卡片面（块D `K-6` 同源）：`dropped_items` = %d 而「已裁 N 条」N = %d · warnings=%v",
			dropped, n, card.Warnings)
	}
	if kept+dropped != total {
		t.Errorf("卡片面：三数不自校 %d + %d ≠ %d", kept, dropped, total)
	}
	var cutFrom string
	_ = json.Unmarshal(d["cut_from"], &cutFrom)
	_, cutSet := zerg.TruncatedDetailKeysForTest()
	inSet := false
	for _, v := range cutSet {
		if cutFrom == v {
			inSet = true
		}
	}
	if !inSet {
		t.Errorf("卡片面：`cut_from` = %q 不在三值闭集 %v 内", cutFrom, cutSet)
	}
	t.Logf("卡片面逐字：%s（留下 %d 条 · 卡片警告 %v）", detail, len(card.Items), card.Warnings)

	// 卡片面负控：没裁的卡片 ⇒ ok=false（缺席 · 不许造一格空值）。
	small := zerg.ImpactCardOfForTest(rows[:3], "", "core/cmd/zerg/main.go")
	if small.Truncated {
		t.Fatal("准备态不对：3 条不该裁")
	}
	if _, ok := zerg.ImpactCardBudgetDetailForTest(small); ok {
		t.Error("卡片面负控失败：没裁却给了 `truncated_detail`")
	}
}
