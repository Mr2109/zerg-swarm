// errors_envmeta_failure_test.go —— 缺口 `GAP-20260928-164` 的机检牙：**失败包封要吃 envMeta**。
//
// 口径（本枚立的唯一一条）：
//
//	`emitErrEnvelope` 收了调用方攒下的按需子键（`invocation.envMeta`）⇒ 命令先把查询上下文
//	写进 `envMeta`、随后在出口面失败时，**失败包封**也带得出 `query` / `ledger_sha16` 一类格
//	（改前只渲染 `cliError.Meta`、`envMeta` 一格不带 ⇒ 查询面在错误档拿不到查询上下文）。
//	★ 缺口 `GAP-20260929-41`（本批）：`kind=usage` 的失败**不吐包封** ⇒ 本件的失败载体取
//	`kind=failed` 的**零命中**支（见下），用法错那一档 stdout 恒零字节、读不到失败包封。
//
// 负控（哑化）：把 `errors.go` 的 `envMetaSubkeysJSON(envMeta)` 从失败 `meta` 那行撤掉 ⇒ 本牙必红
// （靠 `go test -overlay` 的哑化两态验证：不带 overlay 应绿、带哑化 overlay 应红）。
//
// 落点：`package main`（够得着 `RunForTest` ⇒ 与 `main()` 同一个 `run`）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// errEnvMetaRec —— 一条合法账行（闭集内 state/impact/prio · `unit` 供 `--unit` 收窄）。
func errEnvMetaRec(id, unit string) map[string]any {
	return map[string]any{
		"id": id, "fp": "fp-" + id, "symptom": "夹具症状（失败包封 envMeta 对拍）", "handmade": "夹具",
		"impact": "命令面", "prio": "P1", "state": "仍缺",
		"want_family": "gap", "want_action": "ls", "repro_cmd": "zerg gap ls", "verify_cmd": "zerg gap ls",
		"found_at": "2026-10-01T00:00:00+08:00", "unit": unit,
	}
}

// TestErrEnvelopeCarriesEnvMeta —— 查询面在**错误档**的失败包封要带出 `envMeta` 那几格。
func TestErrEnvelopeCarriesEnvMeta(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	unit := "core/cmd/zerg"
	// 写夹具真源（一条 · `unit` == 收窄值 ⇒ 命中 ≥1 ⇒ 走出口面那一支，不落零命中自出包封）。
	var b strings.Builder
	line, err := json.Marshal(errEnvMetaRec("GAP-20990101-164", unit))
	if err != nil {
		t.Fatalf("夹具行序列化失败：%v", err)
	}
	b.Write(line)
	b.WriteByte('\n')
	if err := os.WriteFile(filepath.Join(dir, "zerg-cli-gaps.jsonl"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("写夹具真源失败：%v", err)
	}

	// ★ 缺口 `GAP-20260929-41`（本批）：`kind=usage` 的失败**不再补 stdout 包封**（用法错一律
	//   零字节 · 报错全在 stderr）⇒ 本件不能再拿「`--json` 点闭集外字段」当载体（那一路 stdout
	//   恒 0 字节，失败包封拿不到了）。改用**零命中**那一支（`--unit` 收窄到不含夹具行的值）：
	//   它是 `kind=failed`（退码 1），仍由本命令自己走 `emitEnvelopeWith` 出失败包封 ⇒ 本件钉的
	//   `envMeta` 那几格照旧可读。判据**一字未放宽**：退码精确 + 既有两格 + 本枚那五格逐格在。
	//   ★ 件 `Q-150`（2026-10-22）：字段面改指**在册**的 `id` —— 原先那五个名字
	//   （`total,hits,query,query_ts,ledger_sha16`）都是 `meta` 子键、**不在** `gap ls` 的
	//   字段表里：本件过去只在「零命中自出包封」那条路上取数，而那条路当时**不核字段名** ⇒
	//   它靠的正是 `Q-150` 要治的那处静默吞。字段名前置校验接上之后，本件钉的**主题
	//   （失败包封吃 envMeta）一字不动**，只把投影名换成合法的一格（`items` 恒为空数组，与
	//   本件的断言无关；五格 meta 子键由 `gapLsMetaAdd` 照旧给出）。
	var out, errb bytes.Buffer
	rc := RunForTest([]string{"gap", "ls", "--unit", unit + "/零命中-zz", "--json",
		"id"}, &out, &errb)
	if rc != 1 {
		t.Fatalf("零命中要退 1（`kind=failed` · 「没有」不是「失败」也不是绿），得到 %d · stdout=%q stderr=%q",
			rc, out.String(), errb.String())
	}
	var env struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("解不了失败包封：%v · %q", err, out.String())
	}
	if env.Meta == nil {
		t.Fatalf("失败包封没有 meta：%q", out.String())
	}
	// ① 既有子键语义不变（count/source 仍在 —— 本枚只**加格**）
	if _, ok := env.Meta["count"]; !ok {
		t.Errorf("失败包封 meta 缺既有格 count（本枚只加格、✗ 不动旧格）：%v", env.Meta)
	}
	if _, ok := env.Meta["source"]; !ok {
		t.Errorf("失败包封 meta 缺既有格 source（本枚只加格、✗ 不动旧格）：%v", env.Meta)
	}
	// ② 本枚的正题：envMeta 那几格（查询上下文 + 账本身份）要并进失败包封
	for _, k := range []string{"total", "hits", "query", "query_ts", "ledger_sha16"} {
		if _, ok := env.Meta[k]; !ok {
			t.Errorf("失败包封 meta 缺格 %q —— `envMeta` 没并进失败包封（缺口 GAP-20260928-164）：%v", k, env.Meta)
		}
	}
}
