// family_gap_show_axis_test.go —— `gap show <件路径>` 机器面「轴面数 N / 未闭数 M」两格的**成对自检**
// （缺口 `GAP-20260928-173`）。
//
// 口径（与 `gap status --by unit` **同源**）：
//
//	· 轴面数 N = `unit` 键面**精确等值**该件的条数；
//	· 未闭数 M = 其中 `state ∈ gapOpenStates`（仍缺/已派/已立项/回归）的条数。
//
//	正控：同一件，`gap show <件> --json` 的 `meta.axis` / `meta.axis_open` 与
//	      `gap status --by unit --json` 里该件桶的 `count` / `open` **逐数相符**（两命令两读数相等）。
//	负控：件不在仓 ⇒ 仍走**既有档**（退码 2 · `file_not_in_repo`），✗ 不许因新增两格而换码 / 破坏旧退码。
//
// 落点：`package main`（够得着包级桥 `RunForTest = run` —— 与 `main()` 同一个 `run`）。
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

// gapShowAxisRec —— 造一条合法账行（闭集内 state/impact/prio）。`unit` 是件轴键；
// `symptom` 里也带上件路径 ⇒ 该条同时落「逐字子串面」（两面对拍才有意义）。
func gapShowAxisRec(id, unit, state string) map[string]any {
	return map[string]any{
		"id": id, "fp": "fp-" + id, "symptom": "夹具症状（" + unit + "）", "handmade": "夹具",
		"impact": "命令面", "prio": "P1", "state": state,
		"want_family": "gap", "want_action": "show", "repro_cmd": "zerg gap show x", "verify_cmd": "zerg gap show x",
		"found_at": "2026-10-01T00:00:00+08:00", "unit": unit,
	}
}

func gapShowAxisWrite(t *testing.T, dir string, recs []map[string]any) {
	t.Helper()
	var b strings.Builder
	for _, r := range recs {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("夹具行序列化失败：%v", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "zerg-cli-gaps.jsonl"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("写夹具真源失败：%v", err)
	}
}

func gapShowAxisRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

func gapShowAxisMeta(t *testing.T, env string) map[string]any {
	t.Helper()
	var e struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(env), &e); err != nil {
		t.Fatalf("解不了包封：%v · %q", err, env)
	}
	if e.Meta == nil {
		t.Fatalf("包封没有 meta：%q", env)
	}
	return e.Meta
}

func TestGapShowFileAxisCountsMatchStatusUnit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	target := "core/internal/version/version.go" // 真在仓里的件（`gap show` 会 stat 它）
	other := "core/cmd/zerg/family_gap.go"
	recs := []map[string]any{
		gapShowAxisRec("GAP-20990101-A", target, "仍缺"),
		gapShowAxisRec("GAP-20990101-B", target, "已派"),
		gapShowAxisRec("GAP-20990101-C", target, "已解"),
		gapShowAxisRec("GAP-20990101-D", target, "不做"),
		gapShowAxisRec("GAP-20990101-E", other, "仍缺"),
	}
	gapShowAxisWrite(t, dir, recs)

	// 正控 ①：`gap show <件路径> --json`
	rc, out, errb := gapShowAxisRun(t, "gap", "show", target, "--json", "id")
	if rc != 0 {
		t.Fatalf("gap show 要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	meta := gapShowAxisMeta(t, out)
	axisV, okA := meta["axis"]
	openV, okO := meta["axis_open"]
	if !okA || !okO {
		t.Fatalf("机器面缺 axis / axis_open 两格（现得：%v）", meta)
	}
	axis, okA2 := axisV.(float64)
	open, okO2 := openV.(float64)
	if !okA2 || !okO2 {
		t.Fatalf("axis / axis_open 要是数字：axis=%v axis_open=%v", axisV, openV)
	}

	// 正控 ②：同件在 `gap status --by unit` 的桶（count / open）
	rc, sout, serrb := gapShowAxisRun(t, "gap", "status", "--by", "unit", "--json", "bucket,count,open")
	if rc != 0 {
		t.Fatalf("gap status 要 rc=0，得到 %d · stderr=%q", rc, serrb)
	}
	var senv struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(sout), &senv); err != nil {
		t.Fatalf("解不了 status 包封：%v · %q", err, sout)
	}
	bCount, bOpen := -1.0, -1.0
	for _, it := range senv.Items {
		if it["bucket"] != target {
			continue
		}
		// `gap status` 的 items 值是**字符串**（`rows` 是 map[string]string ⇒ 出口按字符串渲染）。
		cs, _ := it["count"].(string)
		osv, _ := it["open"].(string)
		c, e1 := strconv.ParseFloat(cs, 64)
		o, e2 := strconv.ParseFloat(osv, 64)
		if e1 != nil || e2 != nil {
			t.Fatalf("status 桶 %q 的 count/open 解析失败：count=%v open=%v", target, it["count"], it["open"])
		}
		bCount, bOpen = c, o
	}
	if bCount < 0 {
		t.Fatalf("status 里没有该件桶 %q：%s", target, sout)
	}

	// 逐数相符：两命令两读数相等（同源牙 —— 任一处改口径，本格必红）
	if axis != bCount {
		t.Errorf("轴面数 N 不符：gap show axis=%v · status count=%v", axis, bCount)
	}
	if open != bOpen {
		t.Errorf("未闭数 M 不符：gap show axis_open=%v · status open=%v", open, bOpen)
	}
	// 夹具脱数自校：该件 4 条轴面（仍缺/已派/已解/不做）· 其中未闭 2（仍缺 + 已派）
	if axis != 4 {
		t.Errorf("轴面数应为 4（该件 4 条），得到 %v", axis)
	}
	if open != 2 {
		t.Errorf("未闭数应为 2（仍缺 + 已派），得到 %v", open)
	}
	// `items[]` 仍是**逐字子串全集**（不动）：夹具 5 条 symptom 各带自己的 unit ⇒ 含 target 的 4 条
	var env struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("解不了 show 包封：%v", err)
	}
	if len(env.Items) != 4 {
		t.Errorf("items 应是逐字子串全集 4 条（不动），得到 %d", len(env.Items))
	}

	// 负控：件不在仓 ⇒ 既有档（退码 2），✗ 不许因新增两格换码 / 越位出新格
	missing := "core/cmd/zerg/__no_such_file__.go"
	rc2, out2, errb2 := gapShowAxisRun(t, "gap", "show", missing, "--json", "id")
	if rc2 != 2 {
		t.Errorf("件不在仓要仍退 2（既有档），得到 %d · out=%q err=%q", rc2, out2, errb2)
	}
	if !strings.Contains(errb2, "仓里没有这个件") {
		t.Errorf("件不在仓的报话应仍是既有那句：%q", errb2)
	}
	if strings.Contains(out2, "\"axis\"") || strings.Contains(out2, "axis_open") {
		t.Errorf("件不在仓那一态 ✗ 不许出 axis 两格：%q", out2)
	}
}
