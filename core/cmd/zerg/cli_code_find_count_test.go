// cli_code_find_count_test.go —— 缺口 `GAP-20260928-142`（命令面 · P1）的**真二进制**判据：
// `zerg code find … --json count` 的命中计数直读面。
//
// 病（改前现读）：`--json` 的合法字段集里没有 `count` —— 默认面给 `--json count` 报
// `未知字段 "count"`（rc=2 · detail=`json_field_unknown:count`）；聚合面（`--count`）的**真产出集**
// 里明明有 `count`（`{path,count}` 按件命中数），却因**出口白名单**（`cmd.fields`）没登记它而在
// `emitSelected` 那一关被拒 ⇒ 想要「命中多少条」的人只能把整表拉回来自己数。
//
// 治法（**只增 · 向后兼容**）：把 `count` 登进 `code find` 的**可选字段层**（`optFields`，与
// `approveOptionalFields` 同一体例：登在出口白名单、点得出来，但**不进**文档/帮助导出面的**冻结表**
// ⇒ 不造「表列着、面不产出」的反例，也无需重生成命令页）；并在默认面 / `--files-only` 面逐行落
// 一格 `count` = **本跑命中条数**（跑级真值，与 `truncated`/`face` 同一形态）。
//
// ★ 本件从源码现编（`zergBinaryFromSource`）—— 本单硬约束禁重编 `bin/zerg`（行为面待重编）；
// 拿旧制品会把「本面未落制品」误读成「本面不存在」（与 `cli_code_find_test.go` 的 ① 组同法）。
//
// 成对四档：
//
//	① 正控：默认面 `--json count` 的读数 == 同命令人面报的「命中 M 条」（逐数相符）；
//	② 正控（聚合面修复 · 缺口本体）：`--count --json path,count` 不再被出口白名单拒（改前 rc=2）；
//	③ 负控：非法字段（`--json bogus`）仍按既有档拒（rc=2 · 同 detail `json_field_unknown:`）；
//	④ 负控：空命中 ⇒ 报「计 0」（`meta.count=0` 在 · 而非整支不见）—— 退码仍是「零命中」档 1。
package main_test

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var codeFindHitLineRE = regexp.MustCompile(`命中 (\d+) 条`)

// codeFindCountEnvelope —— `--json` 包封按需取 `items[].count` 与 `meta.count`。
type codeFindCountEnvelope struct {
	Items []struct {
		Count string `json:"count"`
		Path  string `json:"path"`
	} `json:"items"`
	Meta struct {
		Count int `json:"count"`
	} `json:"meta"`
}

func TestCodeFind_JSONCountHitCount(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := syntheticRepo(t, "exit 0\n")
	mustWrite(t, filepath.Join(root, "aaa", "one.go"), "package aaa\n// needle here\n// needle again\n")
	mustWrite(t, filepath.Join(root, "bbb", "two.go"), "package bbb\nneedle()\n")

	// 人面读数：本跑「命中 M 条」（人面 stderr 那一行）；並取同命令 `--json path` 的 items 数
	//（= 人面**列出的行数** —— 未截断时与 M 相等）。
	rcH, _, errH := execCase(t, bin, root, "code", "find", "needle")
	if rcH != 0 {
		t.Fatalf("人面那一跑要 rc=0，得到 %d · stderr=%s", rcH, errH)
	}
	m := codeFindHitLineRE.FindStringSubmatch(errH)
	if m == nil {
		t.Fatalf("人面必须报「命中 M 条」：stderr=%q", errH)
	}
	humanHits, _ := strconv.Atoi(m[1])
	if humanHits != 3 {
		t.Fatalf("夹具应有 3 处命中，人面报 %d · stderr=%q", humanHits, errH)
	}
	rcP, outP, _ := execCase(t, bin, root, "code", "find", "needle", "--json", "path")
	if rcP != 0 {
		t.Fatalf("`--json path` 那一跑要 rc=0，得到 %d", rcP)
	}
	var envP struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(outP), &envP); err != nil {
		t.Fatalf("`--json path` 不是合法包封：%v · %q", err, outP)
	}
	humanRows := len(envP.Items)

	// ① 正控：`--json count` 收下（改前 rc=2）且读数与「命中 M 条」逐数相符。
	rc, out, errb := execCase(t, bin, root, "code", "find", "needle", "--json", "count")
	if rc != 0 {
		t.Fatalf("`--json count` 要 rc=0（改前：未知字段 count ⇒ rc=2）；得到 %d · stderr=%s", rc, errb)
	}
	var env codeFindCountEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("`--json count` 不是合法包封：%v · %q", err, out)
	}
	// ★ ② 冻结下限 = 3（棘轮 · 只许增不许减）：夹具 3 处命中 ⇒ `--json count` 条目现读 3
	//   —— 结构静默变小（抽到的条目 < 3）⇒ 本格判红，✗ 不许静默绿。
	cfN := len(env.Items)
	if cfN < 3 {
		t.Fatalf("② 冻结下限 3 被破：`--json count` 只抽到 %d 条（棘轮 · 只许增不许减）", cfN)
	}
	if len(env.Items) != humanRows {
		t.Errorf("`--json count` 的条目数 %d ≠ 人面列出的行数 %d", len(env.Items), humanRows)
	}
	for i, it := range env.Items {
		if it.Count != strconv.Itoa(humanHits) {
			t.Errorf("第 %d 条的 count=%q 要 == 人面命中数 %d（逐数相符）", i, it.Count, humanHits)
		}
	}

	// ② 正控（聚合面修复 · 缺口本体）：`--count --json path,count` 不再被出口白名单拒。
	rcC, outC, errC := execCase(t, bin, root, "code", "find", "needle", "--count", "--json", "path,count")
	if rcC != 0 {
		t.Fatalf("`--count --json path,count` 要 rc=0（改前：未知字段 count ⇒ rc=2）；得到 %d · stderr=%s", rcC, errC)
	}
	if !strings.Contains(outC, `"count"`) {
		t.Errorf("聚合面 `--json path,count` 没出按件 count 格：%q", outC)
	}

	// ③ 负控：非法字段仍按既有档拒（rc=2）· ★ 缺口 `GAP-20260929-41`（本批）：用法错**不吐包封**
	//   ⇒ stdout **零字节**，坏字段名与合法字段表全在 stderr（旧断言「包封里 error.detail =
	//   json_field_unknown:bogus_zz」已过期；本断言在旧行为下会红 ⇒ 有牙）。
	rcB, outB, errB := execCase(t, bin, root, "code", "find", "needle", "--json", "bogus_zz")
	if rcB != 2 {
		t.Errorf("非法字段 `--json bogus_zz` 要 rc=2（既有档），得到 %d", rcB)
	}
	if outB != "" {
		t.Errorf("非法字段（用法错）⇒ stdout 必须零字节（GAP-20260929-41），得到 %q", outB)
	}
	if !strings.Contains(errB, `未知字段 "bogus_zz"`) || !strings.Contains(errB, "合法字段:") {
		t.Errorf("非法字段要在 stderr 点名 `bogus_zz` 并列合法字段：%q", errB)
	}

	// ④ 负控：空命中 ⇒ 报「计 0」（`meta.count=0` 在 · 而非整支不见）；退码仍为零命中档 1。
	rcZ, outZ, errZ := execCase(t, bin, root, "code", "find", "zzz_nomatch_zzz", "--json", "count")
	if rcZ != 1 {
		t.Errorf("空命中要给「零命中」档 rc=1（不是用法错），得到 %d · stderr=%s", rcZ, errZ)
	}
	var envZ codeFindCountEnvelope
	if err := json.Unmarshal([]byte(outZ), &envZ); err != nil {
		t.Fatalf("空命中 `--json count` 不是合法包封：%v · %q", err, outZ)
	}
	if envZ.Meta.Count != 0 {
		t.Errorf("空命中 ⇒ 计 0（meta.count=0 必须在 · 而非缺格），得到 %d", envZ.Meta.Count)
	}
	if len(envZ.Items) != 0 {
		t.Errorf("空命中 ⇒ items 应为空集，得到 %d 条", len(envZ.Items))
	}
}
