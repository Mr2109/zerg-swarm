// cli_gap_show_absent_test.go —— 缺口 `GAP-20261021-06`（**假零命中**）的机检。
//
// 病（缺口逐字）：`ZERG_STATE_DIR` 指到不存在 / 错路目录时，`zerg gap show <id> --json symptom`
// 退 8，**包里仍带 `items: []`** ⇒ 读方把「读不到真源」当「正文为空」（今天两路据此误判）。
// 要的是：「读不到」与「读到了、只是 0 条」在机器面**不同形**，而**退码不动**（仍 8）。
//
// 判据（三档，合成状态目录 —— **不碰真 `~/.zerg/state`**，复用 `cli_gap_test.go` 的 `gapEnv` / `gapRun`）：
//
//	① 缺档（真源件不在盘上）⇒ rc **仍 8**（fail-closed 不动）· 包封 `items` **不是 `[]`**
//	    （`null` 或整键不出）· `meta.readable` = **布尔** `false` · `meta.reason=ledger_absent` 照旧
//	    · `meta.ledger_path` 照旧 · `error.detail=ledger_absent` 照旧（`Q-138` 的两因分档不许被吃掉）
//	② 读到了但零命中 ⇒ `gap ls` 收窄档 rc=1 · `items` **仍是 `[]`** · `meta.readable` **缺席**
//	    （`[]` 从此只归这一档 —— 这才是「正文为空」）
//	③ 有账但 id 不在 ⇒ rc=2（用法面 · 既有口径逐字不动）· stdout **零字节**
//	    （`GAP-20260929-41`：`kind=usage` 不补 stdout 包封 ⇒ 该档根本没有 `items` 可言）
//
// 落点：`package main_test`（与 `cli_gap_test.go` / `cli_gap_reason_test.go` 同包同路：`zerg.RunForTest`）。
// 负控（本件的牙）：把 `gapLedgerErr` 里那行 `inv.metaAddJSON("readable", "false")` 拿掉，
// 或把 `errEnvelope` 的 `itemsJSON = "null"` 支哑掉 ⇒ ① 当场红（实测：修前裸跑即红在 ①）。
package main_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// gap06Envelope —— 解包封成 `map`（要判「键在不在」与「值是 null 还是 []」，结构体解不了这个差）。
func gap06Envelope(t *testing.T, out string) map[string]any {
	t.Helper()
	var top map[string]any
	if err := json.Unmarshal([]byte(out), &top); err != nil {
		t.Fatalf("包封解析不了：%v · %q", err, out)
	}
	if top == nil {
		t.Fatalf("包封不是对象：%q", out)
	}
	return top
}

func TestGapShowLedgerAbsentNotEmpty(t *testing.T) {
	_, ledger, _ := gapEnv(t) // 合成状态目录；真源件**不建** ⇒ ①

	// ── ① 缺档档：`ZERG_STATE_DIR` 指着，但真源件不在盘上 ─────────────────────────────
	if _, err := os.Stat(ledger); !os.IsNotExist(err) {
		t.Fatalf("夹具前提不成立：真源件竟然在盘上（%s）", ledger)
	}
	rc, out, errb := gapRun(t, "gap", "show", "GAP-20261021-06", "--json", "symptom")
	if rc != 8 {
		t.Fatalf("缺档仍要退 8（fail-closed 一个字节不动），得到 %d · stderr=%s", rc, errb)
	}
	top := gap06Envelope(t, out)
	if items, has := top["items"]; has && items != nil {
		t.Fatalf("缺档档的 `items` 必须**不是** `[]`（`null` 或整键不出）—— 得到 %v\n"+
			"  ⇒ 读方仍会把「读不到真源」当「正文为空」（假零命中）\n  包封=%s", items, out)
	}
	meta, _ := top["meta"].(map[string]any)
	if meta == nil {
		t.Fatalf("包封没有 meta：%q", out)
	}
	if got, ok := meta["readable"]; !ok || got != false {
		t.Fatalf("缺档档 `meta.readable` 要是**布尔** false，得到 %v（在册=%t）· 包封=%s", got, ok, out)
	}
	if got, _ := meta["reason"].(string); got != "ledger_absent" {
		t.Fatalf("缺档档 `meta.reason` 仍要 ledger_absent（`Q-138` 那两因分档不许被本改法吃掉），得到 %q", got)
	}
	if got, _ := meta["ledger_path"].(string); got != ledger {
		t.Fatalf("缺档档 `meta.ledger_path` 仍要逐字 %q，得到 %q", ledger, got)
	}
	if eb, _ := top["error"].(map[string]any); eb == nil || eb["detail"] != "ledger_absent" {
		t.Fatalf("缺档档 `error.detail` 仍要 ledger_absent，得到 %v", top["error"])
	}

	// ── ③ 有账但 id 不在 ⇒ 用法面（rc=2 · stdout 零字节 · 既有口径逐字不动）───────────
	line := `{"id":"GAP-20990101-01","fp":"fp-fixture","symptom":"夹具","handmade":"夹具",` +
		`"impact":"命令面","prio":"P1","state":"仍缺","want_family":"gap","want_action":"show",` +
		`"repro_cmd":"zerg gap show GAP-20990101-01","verify_cmd":"zerg gap show GAP-20990101-01",` +
		`"found_at":"2026-10-01T00:00:00+08:00"}`
	if err := os.WriteFile(ledger, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("夹具写真源失败：%v", err)
	}
	rc3, out3, errb3 := gapRun(t, "gap", "show", "GAP-20261021-06", "--json", "symptom")
	if rc3 != 2 {
		t.Fatalf("有账但 id 不在 ⇒ 仍要退 2（用法错），得到 %d · stderr=%s", rc3, errb3)
	}
	if strings.TrimSpace(out3) != "" {
		t.Fatalf("有账但 id 不在 ⇒ stdout 仍要零字节（`GAP-20260929-41`：kind=usage 不补包封），得到 %q", out3)
	}

	// ── ② 读到了但零命中 ⇒ `items` **仍是 `[]`**（「正文为空」只归这一档）──────────────
	rc2, out2, errb2 := gapRun(t, "gap", "ls", "--unit", "本件不存在的件.go", "--json", "id")
	if rc2 != 1 {
		t.Fatalf("`gap ls` 收窄档零命中仍要退 1，得到 %d · stderr=%s", rc2, errb2)
	}
	top2 := gap06Envelope(t, out2)
	items2, has2 := top2["items"]
	arr2, isArr := items2.([]any)
	if !has2 || !isArr || len(arr2) != 0 {
		t.Fatalf("读到真源的零命中档 `items` 仍要逐字 `[]`（不许被本改法连坐成 null/缺键），得到 %v（在册=%t）· 包封=%s",
			items2, has2, out2)
	}
	meta2, _ := top2["meta"].(map[string]any)
	if meta2 == nil {
		t.Fatalf("零命中档包封没有 meta：%q", out2)
	}
	if v, ok := meta2["readable"]; ok {
		t.Fatalf("读到真源的档**不许**出现 `meta.readable`（那一格只归「读不到」档），得到 %v", v)
	}
}
