// cli_metrics_test.go —— `zerg metrics`（度量与排序面 · 一条只读命令）的判据 ·
// 组1 序12（源件 `承接自-v2.5.11/承接-度量与排序面-20260921.md:40-42`）。
//
// 落点：`package main_test` + `zerg.RunForTest` —— 跑的是**当前源码**的行为（不是盘上旧制品），
// 与 `cli_matrix_test.go` 的层①同一条路。全部合成盘面（`t.TempDir()` 当读数档落点）：
// **不碰真 `~/.zerg/state`**、不跑 `jscpd` / `staticcheck` / 门禁任何一条。
//
// 本件钉住的七格（逐格对 §一 序53 的验收判据）：
//
//	① 一个可比排序（四类读数各一格）：序按 `score` 降序 · 档 A/B/C/D 逐条对 ·
//	   `--json` 的 `items` **逐条值不同**（负控：专治「一坨同一个值」那种假机读面）；
//	② **空输入 ⇒ rc=2 不给结论**（本条点名的负控）：件不在盘 · 空件 · 零读数 三态各一次，
//	   且 **stdout 0 字节**（「不给结论」不许顺手吐半张表）；
//	③ 读不通 ⇒ **rc=8**（JSON 坏 / 没有 `blocks`）—— 判不了不许当绿；
//	④ 全部不可比 ⇒ rc=2（给不出可比排序，就不给结论）· 逐条点名；
//	⑤ 用法面三态：`--json` 不给字段（码取自退码表）· 多余位置参数 · 未知旗标 ⇒ 各 2；
//	⑥ **只读**：跑前/跑后读数档 sha256 逐字不变（本命令不写不删不改任何件）；
//	⑦ 不可比条目**不静默丢**：`rank` 记 `-` 且仍在 `items` 里。
package main_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// metricsRun 跑一次 `metrics`（合成盘面）。
func metricsRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// metricsFixture 造一个读数档：四类各一条 + 一条**不可比**（`budget = 0`）。
// 归一（逐条手算，写在这里以便断言可核）：
//
//	dup   core/cmd/zerg  119 / 40  = 2.975   ⇒ D（`dir = le`：越小越好 ⇒ value/budget）
//	dead  core/cmd/zerg   65 / 20  = 3.25    ⇒ D
//	gates docs             4 / 1   = 4       ⇒ D
//	cover core/cmd/zerg   12 / 60  = 5.00    ⇒ D（`dir = ge`：越大越好 ⇒ budget/value = 60/12）
//	dup   core/internal   10 / 0   ⇒ **不可比**（`budget ≤ 0` ⇒ 不给分、不进排序）
func metricsFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "metrics-readings.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const metricsFixtureJSON = `{
  "generated_at": "2026-09-24T04:00:00Z",
  "blocks": {
    "dup":   {"dir": "le", "unit": "克隆处",   "source_cmd": "npx jscpd@4 core --format go --min-tokens 50",
              "readings": [{"target": "core/cmd/zerg", "value": 119, "budget": 40},
                           {"target": "core/internal", "value": 10, "budget": 0}]},
    "dead":  {"dir": "le", "unit": "未用符号", "source_cmd": "staticcheck -checks=U1000 ./...",
              "readings": [{"target": "core/cmd/zerg", "value": 65, "budget": 20}]},
    "cover": {"dir": "ge", "unit": "%",        "source_cmd": "go test -cover ./...",
              "readings": [{"target": "core/cmd/zerg", "value": 12, "budget": 60}]},
    "gates": {"dir": "le", "unit": "步",       "source_cmd": "zerg gate results --last",
              "readings": [{"target": "docs", "value": 4, "budget": 1}]}
  }
}`

// metricsItems 跑 `--json` 全字段并把 `items` 解回来（判据走机器面，不走人面字符串）。
func metricsItems(t *testing.T, p string, fields string) (int, []map[string]string) {
	t.Helper()
	rc, out, errb := metricsRun(t, "metrics", "--input", p, "--json", fields)
	if rc != 0 {
		t.Fatalf("`metrics --json %s` ⇒ 要 0，得到 %d · stderr=%s", fields, rc, errb)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("包封不是 JSON：%v · out=%s", err, out)
	}
	var items []map[string]string
	if err := json.Unmarshal(doc["items"], &items); err != nil {
		t.Fatalf("`items` 解不回来：%v", err)
	}
	return rc, items
}

// ---- ① 一个可比排序 + 档位 + 逐条值不同 ------------------------------------------------------

func TestMetricsRankingOrderAndGrades(t *testing.T) {
	p := metricsFixture(t, metricsFixtureJSON)
	_, items := metricsItems(t, p, strings.Join([]string{"rank", "grade", "category", "target", "value", "budget", "unit", "score", "source"}, ","))
	if len(items) != 5 {
		t.Fatalf("五条读数（四可比 + 一不可比）⇒ items 要 5 条，得到 %d：%v", len(items), items)
	}
	// 排序：score 降序 ⇒ gates 4.00 · dead 3.25 · dup 2.98 · cover 5.00
	//   ★ 注意 cover 是 `dir=ge`（越大越好）：12 / 60 ⇒ budget/value = 5.00 ⇒ 排在第一位。
	wantOrder := []struct{ cat, target, grade string }{
		{"cover", "core/cmd/zerg", "D"},
		{"gates", "docs", "D"},
		{"dead", "core/cmd/zerg", "D"},
		{"dup", "core/cmd/zerg", "D"},
	}
	for i, w := range wantOrder {
		got := items[i]
		if got["rank"] != string(rune('1'+i)) {
			t.Errorf("第 %d 条的 rank 要 %d，得到 %q", i+1, i+1, got["rank"])
		}
		if got["category"] != w.cat || got["target"] != w.target {
			t.Errorf("第 %d 条要 %s/%s，得到 %s/%s（排序口径：score 降序 · 并列按类序再按 target）",
				i+1, w.cat, w.target, got["category"], got["target"])
		}
		if got["grade"] != w.grade {
			t.Errorf("第 %d 条（score=%s）的档要 %s，得到 %s", i+1, got["score"], w.grade, got["grade"])
		}
	}
	// 逐条值不同（负控：同一坨值重复出现 = 假机读面 —— `help export --json` 的老病）。
	seen := map[string]int{}
	for _, it := range items {
		seen[it["category"]+"/"+it["target"]]++
	}
	if len(seen) != len(items) {
		t.Errorf("items 里有重复的 %s/%s：机读面必须逐条可区分", "category", "target")
	}
	// 不可比那条：rank 记 `-`（**不静默丢**），且 budget=0 逐字照实。
	last := items[len(items)-1]
	if last["rank"] != "-" || last["target"] != "core/internal" || last["budget"] != "0" {
		t.Errorf("不可比条目要排在最后、`rank` 记 `-`、`budget` 照实记 0，得到 %v", last)
	}
	if last["score"] != "-" || last["grade"] != "-" {
		t.Errorf("不可比条目不许给分/给档（照实记 `-`），得到 score=%q grade=%q", last["score"], last["grade"])
	}
}

// 档位边界（A/B/C/D 三条阈值 —— 本面自定的一条尺，逐条钉住；三条阈值都是**闭区间下界**）。
func TestMetricsGradeBoundaries(t *testing.T) {
	body := `{"blocks": {
	  "dup": {"dir": "le", "unit": "处", "source_cmd": "x",
	    "readings": [
	      {"target": "a-a", "value": 1, "budget": 4},
	      {"target": "b-b", "value": 2, "budget": 4},
	      {"target": "c-c", "value": 3, "budget": 4},
	      {"target": "d-d", "value": 1, "budget": 1},
	      {"target": "e-e", "value": 2, "budget": 1}
	    ]}}}`
	p := metricsFixture(t, body)
	_, items := metricsItems(t, p, "rank,grade,target,score")
	// 0.25 ⇒ A · 0.50 ⇒ B（下界闭）· 0.75 ⇒ B · 1.00 ⇒ C（下界闭）· 2.00 ⇒ D（下界闭）
	want := map[string]string{"a-a": "A", "b-b": "B", "c-c": "B", "d-d": "C", "e-e": "D"}
	for _, it := range items {
		if it["grade"] != want[it["target"]] {
			t.Errorf("%s（score=%s）的档要 %s，得到 %s", it["target"], it["score"], want[it["target"]], it["grade"])
		}
	}
}

// ---- ② 空输入 ⇒ rc=2 不给结论（本条点名的负控）· stdout 0 字节 --------------------------------

func TestMetricsEmptyInputIsNotAConclusion(t *testing.T) {
	usage := usageCodeFromTable(t)

	// (a) 件不在盘
	missing := filepath.Join(t.TempDir(), "no-such-readings.json")
	rc, out, errb := metricsRun(t, "metrics", "--input", missing)
	if rc != usage {
		t.Errorf("读数档不在盘 ⇒ 要退码表里 `usage`(%d)，得到 %d · stderr=%s", usage, rc, errb)
	}
	if len(out) != 0 {
		t.Errorf("「不给结论」不许吐半张表：stdout 要 0 字节，得到 %q", out)
	}
	for _, w := range []string{"空输入", "不给结论"} {
		if !strings.Contains(errb, w) {
			t.Errorf("stderr 要点名 %q：%s", w, errb)
		}
	}

	// (b) 空件
	empty := metricsFixture(t, "   \n")
	rc, out, _ = metricsRun(t, "metrics", "--input", empty)
	if rc != usage || len(out) != 0 {
		t.Errorf("空件 ⇒ 要 rc=%d 且 stdout 0 字节，得到 rc=%d out=%q", usage, rc, out)
	}

	// (c) 零读数（四类都在、readings 全空）
	zero := metricsFixture(t, `{"blocks": {"dup": {"dir": "le", "unit": "处", "source_cmd": "x"}}}`)
	rc, out, errb = metricsRun(t, "metrics", "--input", zero)
	if rc != usage || len(out) != 0 {
		t.Errorf("零读数 ⇒ 要 rc=%d 且 stdout 0 字节，得到 rc=%d out=%q", usage, rc, out)
	}
	if !strings.Contains(errb, "零条读数") {
		t.Errorf("零读数要在 stderr 点名：%s", errb)
	}

	// (d) 默认落点（`<状态目录>`）不在盘 ⇒ 同样是不给结论（不是 0、也不是 1）
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	rc, out, errb = metricsRun(t, "metrics")
	if rc != usage || len(out) != 0 {
		t.Errorf("默认落点缺件 ⇒ 要 rc=%d 且 stdout 0 字节，得到 rc=%d out=%q · stderr=%s",
			usage, rc, out, errb)
	}
}

// ---- ③ 读不通 ⇒ rc=8（判不了不许当绿）-------------------------------------------------------

func TestMetricsUnreadableIsBlockedNotGreen(t *testing.T) {
	// (a) JSON 坏
	bad := metricsFixture(t, "{ 这不是 JSON")
	rc, out, errb := metricsRun(t, "metrics", "--input", bad)
	if rc != 8 {
		t.Errorf("JSON 坏 ⇒ 要 8（读不通 · 不许当绿），得到 %d · stderr=%s", rc, errb)
	}
	if len(out) != 0 {
		t.Errorf("读不通 ⇒ stdout 要 0 字节，得到 %q", out)
	}
	// (b) 没有 blocks
	noblk := metricsFixture(t, `{"generated_at": "2026-09-24T04:00:00Z"}`)
	rc, _, errb = metricsRun(t, "metrics", "--input", noblk)
	if rc != 8 {
		t.Errorf("没有 `blocks` ⇒ 要 8，得到 %d · stderr=%s", rc, errb)
	}
}

// ---- ④ 全部不可比 ⇒ rc=2 -------------------------------------------------------------------

func TestMetricsAllUncomparableIsNotAConclusion(t *testing.T) {
	body := `{"blocks": {"dup": {"dir": "le", "unit": "处", "source_cmd": "x",
	  "readings": [{"target": "a", "value": 9, "budget": 0}]},
	  "dead": {"dir": "多大都行", "unit": "处", "source_cmd": "y",
	  "readings": [{"target": "b", "value": 9, "budget": 3}]}}}`
	p := metricsFixture(t, body)
	rc, out, errb := metricsRun(t, "metrics", "--input", p)
	if rc != usageCodeFromTable(t) {
		t.Errorf("全部不可比 ⇒ 给不出可比排序 ⇒ 要 2，得到 %d · stderr=%s", rc, errb)
	}
	if len(out) != 0 {
		t.Errorf("给不出排序 ⇒ stdout 要 0 字节，得到 %q", out)
	}
	for _, w := range []string{"全部不可比", "不可比："} {
		if !strings.Contains(errb, w) {
			t.Errorf("要逐条点名不可比的原因（%q）：%s", w, errb)
		}
	}
}

// ---- ⑤ 用法面三态 --------------------------------------------------------------------------

func TestMetricsUsageErrors(t *testing.T) {
	usage := usageCodeFromTable(t)
	p := metricsFixture(t, metricsFixtureJSON)

	// `--json` 不给字段 ⇒ 码取自退码表 + stdout 0 字节 + stderr 列字段表。
	rc, out, errb := metricsRun(t, "metrics", "--input", p, "--json")
	if rc != usage {
		t.Errorf("`--json` 不给字段 ⇒ 要 %d，得到 %d · stderr=%s", usage, rc, errb)
	}
	if len(out) != 0 {
		t.Errorf("`--json` 不给字段 ⇒ stdout 要 0 字节，得到 %q", out)
	}
	for _, f := range []string{"rank", "grade", "category", "score", "source"} {
		if !strings.Contains(errb, f) {
			t.Errorf("stderr 的字段清单里缺 %q：%s", f, errb)
		}
	}

	// 多余位置参数 ⇒ 2（本命令无位置参数）。
	if rc, _, _ = metricsRun(t, "metrics", "多余"); rc != usage {
		t.Errorf("多余位置参数 ⇒ 要 %d，得到 %d", usage, rc)
	}
	// 未知旗标 ⇒ 2。
	if rc, _, _ = metricsRun(t, "metrics", "--nosuchflag-zz"); rc != usage {
		t.Errorf("未知旗标 ⇒ 要 %d，得到 %d", usage, rc)
	}
}

// ---- ⑥ 只读：跑前/跑后读数档 sha256 逐字不变 ------------------------------------------------

func TestMetricsIsReadOnly(t *testing.T) {
	p := metricsFixture(t, metricsFixtureJSON)
	before, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	sum := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
	for _, argv := range [][]string{
		{"metrics", "--input", p},
		{"metrics", "--input", p, "--json", "rank,score"},
		{"metrics", "--input", p, "--json"}, // 用法错那条路也不许写盘
	} {
		metricsRun(t, argv...)
		after, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%v 之后读数档不见了：%v", argv, err)
		}
		if sum(before) != sum(after) {
			t.Fatalf("%v 改动了读数档（sha256 变了）—— 本命令是**只读**面", argv)
		}
	}
}
