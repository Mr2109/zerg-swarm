// family_gap_diff_test.go —— `zerg gap diff`（新命令 · **尚未接进命令树**）的**四态 ＋ 负控**机检。
//
// 落点：`package main` ＋ **直调** `cmdGapDiff`（该命令还不在命令树里 ⇒ 走不了 `RunForTest`；
// 接线那一单才改走命令树）。夹具全在 `t.TempDir()` 的合成盘面上 —— 不碰真状态目录、不碰任何仓件。
//
// 四态（逐条读数）：
//
//	① 两份相同 ⇒ rc=0 且人面逐字出「无差异」；
//	② 人为差异 ⇒ rc=1 且**逐条点名**（仅左 / 仅右 / 同 id 不同体各一条 · id ＋ 格名 ＋ 两值）；
//	③ 输入取不到 / 格式不对 ⇒ rc=2 且 **stdout 0 字节**（不给结论 · ✗ 不假绿）；
//	④ `--json`（包封 items 逐格）与 `--dry-run`（**档不得真写**）各真跑一次。
//
// 负控（三条）：
//
//	① 非可比格（只一侧带的格）**不得**误报为差异（点名 `unit` 但差异数不动）；
//	② 一份账里塞一行半截 JSON ⇒ rc=2 且 stdout 0 字节；
//	③ `--dry-run --out` ⇒ 报告件**不存在**且两输入 sha256 逐字不变；真写（`--yes`）⇒ 件在盘。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gapDiffTestRec —— 造一条合法账行（闭集内的 state/impact/prio · 带 `unit`）。
func gapDiffTestRec(id, state, unit string) map[string]any {
	return map[string]any{
		"id": id, "fp": "fp-" + id, "symptom": "夹具症状", "handmade": "夹具",
		"impact": "命令面", "prio": "P1", "state": state,
		"want_family": "gap", "want_action": "ls", "repro_cmd": "zerg gap ls",
		"verify_cmd": "zerg gap ls", "found_at": "2026-10-01T00:00:00+08:00", "unit": unit,
	}
}

// gapDiffTestWriteLedger —— 把若干账行写成一份账（jsonl）。
func gapDiffTestWriteLedger(t *testing.T, path string, recs []map[string]any) {
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
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("写夹具账失败：%v", err)
	}
}

// gapDiffTestRun —— 直调**未接线**的命令入口（合成 `invocation`；`kv` 空表由本函数补）。
func gapDiffTestRun(t *testing.T, inv *invocation) (int, string, string) {
	t.Helper()
	if inv.kv == nil {
		inv.kv = map[string][]string{}
	}
	var out, errb bytes.Buffer
	rc := cmdGapDiff(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// gapDiffTestInv —— 一枚合成 invocation（path 走 `gap diff` 那一格）。
func gapDiffTestInv(args ...string) *invocation {
	return &invocation{path: []string{"gap", "diff"}, args: args, kv: map[string][]string{}}
}

// gapDiffTestSHA —— 件 sha256（十六进制全串）。
func gapDiffTestSHA(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读件失败（%s）：%v", p, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// gapDiffTestItems —— 从包封里取 `items` 与 `meta.count`（`--json` 面）。
func gapDiffTestItems(t *testing.T, env string) ([]map[string]any, int) {
	t.Helper()
	var e struct {
		Items []map[string]any `json:"items"`
		Meta  struct {
			Count int `json:"count"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(env), &e); err != nil {
		t.Fatalf("解不了包封：%v · %q", err, env)
	}
	return e.Items, e.Meta.Count
}

// TestGapDiffFourStates —— ①/②/③ 三态 ＋ ④ `--json`。
func TestGapDiffFourStates(t *testing.T) {
	dir := t.TempDir()
	unit := "core/cmd/zerg"
	left := filepath.Join(dir, "left.jsonl")
	right := filepath.Join(dir, "right.jsonl")

	// 左：A(仍缺) B(仍缺) C(已解)；右：A(**已解**) B(仍缺) D(仅右)
	gapDiffTestWriteLedger(t, left, []map[string]any{
		gapDiffTestRec("GAP-20260101-0001", "仍缺", unit),
		gapDiffTestRec("GAP-20260101-0002", "仍缺", unit),
		gapDiffTestRec("GAP-20260101-0003", "已解", unit),
	})
	gapDiffTestWriteLedger(t, right, []map[string]any{
		gapDiffTestRec("GAP-20260101-0001", "已解", unit),
		gapDiffTestRec("GAP-20260101-0002", "仍缺", unit),
		gapDiffTestRec("GAP-20260101-0004", "仍缺", unit),
	})

	// ① 两份相同 ⇒ rc=0 + 逐字「无差异」（`same` = `left` 的逐字拷贝：三条同 id 同格）
	same := filepath.Join(dir, "same.jsonl")
	gapDiffTestWriteLedger(t, same, []map[string]any{
		gapDiffTestRec("GAP-20260101-0001", "仍缺", unit),
		gapDiffTestRec("GAP-20260101-0002", "仍缺", unit),
		gapDiffTestRec("GAP-20260101-0003", "已解", unit),
	})
	rc, out, errb := gapDiffTestRun(t, gapDiffTestInv(left, same))
	t.Logf("态① 相同：rc=%d out=%q", rc, out)
	if rc != 0 {
		t.Fatalf("态①（两份相同）要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "无差异") {
		t.Fatalf("态① 人面须逐字出「无差异」，得到 %q", out)
	}

	// ② 有人为差异 ⇒ rc=1 + 逐条点名（仅左 / 仅右 / 同 id 不同体各一条）
	rc, out, errb = gapDiffTestRun(t, gapDiffTestInv(left, right))
	t.Logf("态② 有差：rc=%d out=%q", rc, out)
	if rc != 1 {
		t.Fatalf("态②（有人为差异）要 rc=1，得到 %d · stderr=%q", rc, errb)
	}
	for _, want := range []string{
		"仅左 1", "仅右 1", "同 id 不同体 1",
		"[仅左] GAP-20260101-0003", "[仅右] GAP-20260101-0004",
		"[同 id 不同体] GAP-20260101-0001", "格 `state`",
		"左 `仍缺`", "右 `已解`",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("态② 人面缺逐字片段 %q · 现得 %q", want, out)
		}
	}

	// ③ 输入取不到 ⇒ rc=2 且 stdout **0 字节**（不给结论 · ✗ 不假绿）
	rc, out, errb = gapDiffTestRun(t, gapDiffTestInv(left, filepath.Join(dir, "nope.jsonl")))
	t.Logf("态③ 取不到：rc=%d stdout=%q stderr=%q", rc, out, errb)
	if rc != 2 {
		t.Fatalf("态③（取不到）要 rc=2，得到 %d", rc)
	}
	if out != "" {
		t.Fatalf("态③ stdout 须 0 字节（不给结论），得到 %q", out)
	}
	if !strings.Contains(errb, "不给结论") {
		t.Fatalf("态③ stderr 须逐字含「不给结论」，得到 %q", errb)
	}

	// ③b 格式不对（账里一行半截 JSON）⇒ rc=2 且 stdout 0 字节
	bad := filepath.Join(dir, "bad.jsonl")
	if err := os.WriteFile(bad, []byte("{\"id\":\"GAP-1\"}\n{不是JSON\n"), 0o644); err != nil {
		t.Fatalf("写坏夹具失败：%v", err)
	}
	rc, out, errb = gapDiffTestRun(t, gapDiffTestInv(left, bad))
	t.Logf("态③b 坏格式：rc=%d stdout=%q", rc, out)
	if rc != 2 {
		t.Fatalf("态③b（坏格式）要 rc=2，得到 %d · stderr=%q", rc, errb)
	}
	if out != "" {
		t.Fatalf("态③b stdout 须 0 字节，得到 %q", out)
	}

	// ④ `--json` ⇒ 包封 items 逐格（三类差共 3 格 · meta.count 同源）
	inv := gapDiffTestInv(left, right)
	inv.jsonGiven = true
	inv.fields = gapDiffFields
	rc, out, errb = gapDiffTestRun(t, inv)
	t.Logf("态④ --json：rc=%d out=%q", rc, out)
	if rc != 1 {
		t.Fatalf("态④（--json 有差）要 rc=1，得到 %d · stderr=%q", rc, errb)
	}
	items, count := gapDiffTestItems(t, out)
	if len(items) != 3 || count != 3 {
		t.Fatalf("态④ 期望 items/count = 3/3（真下界 3），得到 %d/%d · %q", len(items), count, out)
	}
	kinds := map[string]bool{}
	for _, it := range items {
		k, _ := it["kind"].(string)
		kinds[k] = true
		if _, ok := it["id"]; !ok {
			t.Fatalf("态④ 每格须带 `id`，得到 %v", it)
		}
	}
	if !kinds[gapDiffKindOnlyLeft] || !kinds[gapDiffKindOnlyRight] || !kinds[gapDiffKindBodyDiff] {
		t.Fatalf("态④ 三类差都须出现，得到 kind 集 %v", kinds)
	}
}

// TestGapDiffNegativeControls —— 三条负控（非可比格不误报 / 坏格式不假绿 / dry-run 不真写）。
func TestGapDiffNegativeControls(t *testing.T) {
	dir := t.TempDir()
	unit := "core/cmd/zerg"

	// 负控① 账（带 unit/fp/handmade…）⟷ 导出件（只承载 prio/state/symptom/want/verify_cmd）
	ledger := filepath.Join(dir, "side.jsonl")
	gapDiffTestWriteLedger(t, ledger, []map[string]any{
		gapDiffTestRec("GAP-20260101-0001", "仍缺", unit),
	})
	exp := filepath.Join(dir, "side.md")
	expBody := "## 缺口真源导出（**只列未闭**） · 2026-10-15\n" +
		"> 生成件：由 `zerg gap export` 从真源生成\n\n" +
		"### 命令面（1 条）\n\n" +
		"- `GAP-20260101-0001` P1 · **仍缺** · 夹具症状 · 想要 `gap ls` · 判据 `zerg gap ls`\n"
	if err := os.WriteFile(exp, []byte(expBody), 0o644); err != nil {
		t.Fatalf("写导出夹具失败：%v", err)
	}
	rc, out, errb := gapDiffTestRun(t, gapDiffTestInv(ledger, exp))
	t.Logf("负控① 账⟷导出：rc=%d out=%q", rc, out)
	if rc != 0 {
		t.Fatalf("负控①（非可比格不计差异）要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "无差异") || !strings.Contains(out, "非可比格") || !strings.Contains(out, "unit") {
		t.Fatalf("负控① 须报无差异 + 点名非可比格（含 unit），得到 %q", out)
	}

	// 负控② 一份账塞一行半截 JSON ⇒ rc=2 且 stdout 0 字节（与态③b 同型 · 换一侧再验一次）
	bad := filepath.Join(dir, "bad2.jsonl")
	if err := os.WriteFile(bad, []byte("[1,2,3]\n"), 0o644); err != nil {
		t.Fatalf("写坏夹具失败：%v", err)
	}
	rc, out, errb = gapDiffTestRun(t, gapDiffTestInv(bad, ledger))
	t.Logf("负控② 坏账：rc=%d stdout=%q", rc, out)
	if rc != 2 {
		t.Fatalf("负控②（坏账）要 rc=2，得到 %d · stderr=%q", rc, errb)
	}
	if out != "" {
		t.Fatalf("负控② stdout 须 0 字节，得到 %q", out)
	}

	// 负控③ `--dry-run --out` ⇒ rc 照真跑(1) 且**报告件不存在** + 两输入 sha 逐字不变；真写 ⇒ 件在盘
	left := filepath.Join(dir, "l.jsonl")
	right := filepath.Join(dir, "r.jsonl")
	gapDiffTestWriteLedger(t, left, []map[string]any{gapDiffTestRec("GAP-20260101-0001", "仍缺", unit)})
	gapDiffTestWriteLedger(t, right, []map[string]any{gapDiffTestRec("GAP-20260101-0001", "已解", unit)})
	shaL0, shaR0 := gapDiffTestSHA(t, left), gapDiffTestSHA(t, right)
	report := filepath.Join(dir, "报告.md")

	inv := gapDiffTestInv(left, right)
	inv.kv["--out"] = []string{report}
	inv.dryRun = true
	rc, out, errb = gapDiffTestRun(t, inv)
	t.Logf("负控③ dry-run：rc=%d out=%q", rc, out)
	if rc != 1 {
		t.Fatalf("负控③（dry-run · 判定照真跑）要 rc=1，得到 %d · stderr=%q", rc, errb)
	}
	if _, err := os.Stat(report); !os.IsNotExist(err) {
		t.Fatalf("负控③ `--dry-run` 档**不得真写**：报告件竟然在盘（err=%v）", err)
	}
	if gapDiffTestSHA(t, left) != shaL0 || gapDiffTestSHA(t, right) != shaR0 {
		t.Fatalf("负控③ `--dry-run` 档两输入 sha256 须逐字不变")
	}

	inv = gapDiffTestInv(left, right)
	inv.kv["--out"] = []string{report}
	inv.yes = true
	rc, _, errb = gapDiffTestRun(t, inv)
	if rc != 1 {
		t.Fatalf("负控③b（真写）要 rc=1，得到 %d · stderr=%q", rc, errb)
	}
	b, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("负控③b 真写后报告件须在盘：%v", err)
	}
	if !strings.Contains(string(b), "有差异") {
		t.Fatalf("负控③b 报告件须含判定，得到 %q", string(b))
	}

	// 负控③c 缺 `--yes`（D2 档）⇒ rc=2 且报告件仍不在盘
	report2 := filepath.Join(dir, "报告2.md")
	inv = gapDiffTestInv(left, right)
	inv.kv["--out"] = []string{report2}
	rc, out, errb = gapDiffTestRun(t, inv)
	t.Logf("负控③c 缺 yes：rc=%d stdout=%q", rc, out)
	if rc != 2 {
		t.Fatalf("负控③c（缺 --yes）要 rc=2，得到 %d · stderr=%q", rc, errb)
	}
	if _, err := os.Stat(report2); !os.IsNotExist(err) {
		t.Fatalf("负控③c 缺 `--yes` 不得落件（err=%v）", err)
	}
}
