// gap_20260929_41_test.go —— 缺口 `GAP-20260929-41` 的自检（gap 命令族三条）。
//
// 缺口原文（三条）：
//
//	① `--by unit` 不支持按桶名收窄（中段桶取不到，只能直读真源）；
//	② 探针状态目录无「只读借用真源」档（要读账必须先把真源拷进沙箱）；
//	③ 用法类错误一律不吐包封（stdout 零字节）。
//
// 本件断言（逐条对号）：
//
//	① TestGapStatusBucketNarrow —— `--bucket <桶名>` 真按**渲染出来的桶键**收窄：
//	   · 中段桶（排在 `--top` 页外、任何记录级旗标都取不到的那个）点得到，且 count/open
//	     与「全量页里同一桶」逐数相符（两读数相等 —— ✗ 不自算第二份）；
//	   · 五个桶轴（unit / module / tier / day / family）同一把尺（走同一个 `gapStatusBucketPage`）；
//	   · 点不中 ⇒ rc=1 且人面点名「桶名不存在」（✗ 与「桶内 0 条」混为一谈）+ 给最近邻候选；
//	   · 部分点中 ⇒ rc=0，但未点中的桶名**逐条警告**（`meta.bucket_unmatched` 计数，✗ 静默少列）；
//	   · 空值闸：`--bucket ""` ⇒ 用法错 2（空值不是「不过滤」）；
//	   · 未点名 ⇒ `meta.query` 里没有 `bucket` 格（不给没要的收窄背书）。
//	   ★ 旗标登记面（`main.go` 的 `valueFlagName` + `gap status` usage 串）**不在本单允许面**
//	     ⇒ 本件用 `cmdGapStatus` 直接驱动（够得着包级函数）；另设 `TestGapStatusBucketFlagRegistered`
//	     在登记落地后自动转为硬断言（现在 skip：跳过本身就是「待插行」的机读记号）。
//
//	② TestGapBorrowReadOnly —— 只读借用档 `ZERG_GAP_LEDGER_BORROW`：
//	   · 未开档（沙箱无账）⇒ 照旧 rc=8（既有行为一字不变，负控）；
//	   · 绝对目录 / 绝对件 / `real` 三形态取值判据正确；
//	   · 开档 ⇒ 读面**直接读真源**（不拷）：条数与真源相符 + stderr 自报「只读借用档」；
//	   · 错值（相对路径）⇒ 用法错 2（✗ 不静默回落成「未开档」= 静默换真源）；
//	   · 写面 ⇒ **拒执**且借来的真源**逐字节不变**（sha256 对拍）。
//
//	③ TestGapFamilyUsageErrorsEmitNoEnvelope —— 用法类错误（rc=2）一律 stdout **零字节**：
//	   全族逐动作跑一遍；并配一条**正控**（rc=8 的 blocked 读数**必须**吐包封）——
//	   否则「零字节」这条断言在「命令根本不输出」时也会假绿（判据要有牙）。
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

// ── 夹具 ────────────────────────────────────────────────────────────────────────────

// gap41Rec —— 造一条合法账行（闭集内 state/impact/prio）。
func gap41Rec(id, unit, module, fam, state, prio, foundAt string) map[string]any {
	return map[string]any{
		"id": id, "fp": "fp-" + id, "symptom": "夹具（" + unit + "）", "handmade": "夹具",
		"impact": "命令面", "prio": prio, "state": state,
		"want_family": fam, "want_action": "status",
		"repro_cmd": "zerg gap status", "verify_cmd": "zerg doctor",
		"found_at": foundAt, "unit": unit, "module": module,
	}
}

// gap41Write —— 把账写成 `<dir>/zerg-cli-gaps.jsonl`（返回该件路径）。
func gap41Write(t *testing.T, dir string, recs []map[string]any) string {
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
	path := filepath.Join(dir, gapLedgerFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建夹具目录失败：%v", err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("写夹具真源失败：%v", err)
	}
	return path
}

// gap41StatusFlags —— `cmdGapStatus` 的入参（直接驱动：旗标解析面在 `main.go`，本单禁改）。
type gap41StatusFlags struct {
	by      string
	top     string
	buckets []string // 逐枚 `--bucket`（空串也是合法入参：空值闸要测）
	units   []string
	modules []string
	grep    string
	fields  []string // 非 nil ⇒ `--json`
}

// gap41RunStatus —— 直接调 `cmdGapStatus`（够得着包级函数，绕过 `main.go` 的旗标名字表）。
func gap41RunStatus(t *testing.T, f gap41StatusFlags) (int, string, string) {
	t.Helper()
	inv := &invocation{path: []string{"gap", "status"}}
	if f.by != "" {
		inv.kvSet("--by", f.by)
	}
	if f.top != "" {
		inv.kvSet("--top", f.top)
	}
	for _, b := range f.buckets {
		inv.kvSet("--bucket", b)
	}
	for _, u := range f.units {
		inv.kvSet("--unit", u)
	}
	for _, m := range f.modules {
		inv.kvSet("--module", m)
	}
	if f.grep != "" {
		inv.kvSet("--grep", f.grep)
	}
	if f.fields != nil {
		inv.jsonGiven = true
		inv.fields = f.fields
	}
	var out, errb bytes.Buffer
	rc := cmdGapStatus(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// gap41Rows —— 从 `--json` 包封里取 `items`（桶行）。
func gap41Rows(t *testing.T, out string) ([]map[string]any, map[string]any) {
	t.Helper()
	var env struct {
		Items []map[string]any `json:"items"`
		Meta  map[string]any   `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("解不了包封：%v · %q", err, out)
	}
	return env.Items, env.Meta
}

// gap41BucketCount —— 取一行的 count（机器面是字符串形态）· 取不出 ⇒ 当场炸（✗ 静默 0）。
func gap41BucketCount(t *testing.T, row map[string]any) int {
	t.Helper()
	switch v := row["count"].(type) {
	case string:
		return atoiOrPanic(v)
	case float64:
		return int(v)
	}
	t.Fatalf("桶行缺 `count`：%v", row)
	return 0
}

// gap41Twelve —— 12 个件桶的夹具（每个件 1 条 · 全 `仍缺`）：
// 排序键下（未闭同分 ⇒ 桶键升序）页序恰是 `u01…u12` ⇒ `--top 3` 时 `u07` 就是**中段桶**
// （排在页外、且没有任何记录级旗标能按桶键点到它）。
func gap41Twelve() []map[string]any {
	recs := make([]map[string]any, 0, 12)
	for i := 1; i <= 12; i++ {
		u := "u" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		recs = append(recs, gap41Rec("GAP-20990001-"+string(rune('A'+i-1)), u, "mod"+string(rune('0'+i%3)), "fam"+string(rune('0'+i%2)), "仍缺", "P2", "2026-09-01T00:00:00+08:00"))
	}
	return recs
}

// ── ① 桶面收窄 ──────────────────────────────────────────────────────────────────────

// TestGapStatusBucketNarrow —— ① 的主判据（中段桶取得到 · 两读数相等 · 五轴承同一把尺）。
func TestGapStatusBucketNarrow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	gap41Write(t, dir, gap41Twelve())

	// 全量页（--top 200 = 一页硬顶）⇒ 桶键 → count 的真值表（**唯一口径**，后面的读数都要跟它相等）。
	rc, out, errb := gap41RunStatus(t, gap41StatusFlags{top: "200", fields: []string{"bucket", "count", "open"}})
	if rc != exitOK {
		t.Fatalf("全量 gap status 要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	full := map[string]int{}
	for _, row := range func() []map[string]any { r, _ := gap41Rows(t, out); return r }() {
		full[row["bucket"].(string)] = gap41BucketCount(t, row)
	}
	if len(full) != 12 {
		t.Fatalf("全量页应有 12 个件桶，得到 %d：%v", len(full), full)
	}

	// ①a 中段桶：`--top 3` 的页里没有它（这是「取不到」的现读凭证）。
	rc, out, _ = gap41RunStatus(t, gap41StatusFlags{top: "3", fields: []string{"bucket", "count"}})
	if rc != exitOK {
		t.Fatalf("`--top 3` 要 rc=0，得到 %d", rc)
	}
	page, _ := gap41Rows(t, out)
	for _, row := range page {
		if row["bucket"] == "u07" {
			t.Fatalf("夹具前提不成立：中段桶 u07 竟在 --top 3 的页里（%v）", page)
		}
	}
	// ①b 按桶名点名它 ⇒ 恰好一行，且 count 与全量页逐数相等。
	rc, out, errb = gap41RunStatus(t, gap41StatusFlags{top: "3", buckets: []string{"u07"}, fields: []string{"bucket", "count", "open"}})
	if rc != exitOK {
		t.Fatalf("`--bucket u07` 要 rc=0（中段桶必须取得到），得到 %d · stderr=%q", rc, errb)
	}
	rows, meta := gap41Rows(t, out)
	if len(rows) != 1 || rows[0]["bucket"] != "u07" {
		t.Fatalf("`--bucket u07` 应只列 u07 一行，得到 %v", rows)
	}
	if got, want := gap41BucketCount(t, rows[0]), full["u07"]; got != want {
		t.Errorf("`--bucket u07` 的 count=%d 与全量页同一桶的 %d 不符（两读数必须相等）", got, want)
	}
	if meta["bucket_unmatched"] != 0.0 || meta["bucket_all"] != 12.0 || meta["bucket_dropped"] != 11.0 {
		t.Errorf("机器面三格不对：all=%v dropped=%v unmatched=%v（应 12 / 11 / 0）",
			meta["bucket_all"], meta["bucket_dropped"], meta["bucket_unmatched"])
	}
	// ①c 收窄回显：`meta.query.bucket` 要点名（带/不带两份读数可分辨）。
	if q, ok := meta["query"].(map[string]any); !ok {
		t.Fatalf("`meta.query` 要是对象，得到 %T", meta["query"])
	} else if arr, ok := q["bucket"].([]any); !ok || len(arr) != 1 || arr[0] != "u07" {
		t.Errorf("`meta.query.bucket` 要回显 [u07]，得到 %v", q["bucket"])
	}

	// ①d 多枚点名 ⇒ 多行（顺序仍按既有排序键：桶键升序）。
	rc, out, _ = gap41RunStatus(t, gap41StatusFlags{top: "3", buckets: []string{"u09", "u05"}, fields: []string{"bucket"}})
	if rc != exitOK {
		t.Fatalf("`--bucket u09 --bucket u05` 要 rc=0，得到 %d", rc)
	}
	rows, _ = gap41Rows(t, out)
	if len(rows) != 2 || rows[0]["bucket"] != "u05" || rows[1]["bucket"] != "u09" {
		t.Errorf("点名两桶应得 [u05 u09]（既有排序键仍生效），得到 %v", rows)
	}

	// ①e 点不中 ⇒ rc=1 + 人面点名「桶名不存在」+ 最近邻候选（✗ 与「桶内 0 条」混为一谈）。
	rc, _, errb = gap41RunStatus(t, gap41StatusFlags{buckets: []string{"u99"}, fields: []string{"bucket"}})
	if rc != exitFail {
		t.Fatalf("点不中的桶名要 rc=1，得到 %d", rc)
	}
	for _, want := range []string{"点不中", "不是「桶内 0 条」", "最近邻"} {
		if !strings.Contains(errb, want) {
			t.Errorf("点不中的人面判词要含 %q，stderr=%q", want, errb)
		}
	}

	// ①f 部分点中 ⇒ rc=0，但未点中的桶名逐条警告（✗ 静默少列）。
	rc, out, errb = gap41RunStatus(t, gap41StatusFlags{buckets: []string{"u07", "u99"}, fields: []string{"bucket"}})
	if rc != exitOK {
		t.Fatalf("部分点中要 rc=0（点中的桶是真读数），得到 %d · stderr=%q", rc, errb)
	}
	rows, meta = gap41Rows(t, out)
	if len(rows) != 1 || rows[0]["bucket"] != "u07" {
		t.Errorf("部分点中应只列 u07，得到 %v", rows)
	}
	if !strings.Contains(errb, "u99") || !strings.Contains(errb, "点不中") {
		t.Errorf("部分点中时未点中的桶名必须逐条警告（含 u99），stderr=%q", errb)
	}
	if meta["bucket_unmatched"] != 1.0 {
		t.Errorf("`meta.bucket_unmatched` 要 1，得到 %v", meta["bucket_unmatched"])
	}

	// ①g 空值闸：空值不是「不过滤」。
	if rc, _, errb = gap41RunStatus(t, gap41StatusFlags{buckets: []string{""}}); rc != exitUsage {
		t.Errorf("`--bucket \"\"` 要 rc=2（用法错），得到 %d · stderr=%q", rc, errb)
	}

	// ①h 未点名 ⇒ `meta.query` 里没有 bucket 格（不给没要的收窄背书）。
	_, out, _ = gap41RunStatus(t, gap41StatusFlags{fields: []string{"bucket"}})
	_, meta = gap41Rows(t, out)
	if q, ok := meta["query"].(map[string]any); ok {
		if q["bucket"] != nil {
			t.Errorf("没给 `--bucket` 时 `meta.query` 不该有 bucket 格，得到 %v", q["bucket"])
		}
	} else {
		t.Fatalf("`meta.query` 要是对象，得到 %T", meta["query"])
	}
}

// TestGapStatusBucketNarrowAllAxes —— ① 的五轴承同一把尺（unit / module / tier / day / family）。
func TestGapStatusBucketNarrowAllAxes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	recs := []map[string]any{
		gap41Rec("GAP-20990002-A", "core/a.go", "modA", "famA", "仍缺", "P0", "2026-09-01T00:00:00+08:00"),
		gap41Rec("GAP-20990002-B", "core/b.go", "modB", "famB", "已解", "P1", "2026-09-02T00:00:00+08:00"),
		gap41Rec("GAP-20990002-C", "core/c.go", "modA", "famA", "仍缺", "P2", "2026-09-03T00:00:00+08:00"),
	}
	gap41Write(t, dir, recs)

	cases := []struct{ name, by, bucket string }{
		{"unit", "", "core/b.go"},
		{"module", gapStatusByModule, "modB"},
		{"family", gapStatusByFamily, "famB"},
		{"day", gapStatusByDay, "2026-09-02"},
		{"tier", gapStatusByTier, gapVerifyTierNone}, // 夹具的 verify_cmd 解析不到 ⇒ 落「占位」……见下
	}
	for _, c := range cases {
		recsOut, _ := func() ([]map[string]any, map[string]any) {
			rc, out, errb := gap41RunStatus(t, gap41StatusFlags{by: c.by, buckets: []string{c.bucket}, top: "200", fields: []string{"bucket", "count"}})
			if rc != exitOK {
				t.Fatalf("[%s] `--bucket %s` 要 rc=0，得到 %d · stderr=%q", c.name, c.bucket, rc, errb)
			}
			return gap41Rows(t, out)
		}()
		if len(recsOut) == 0 {
			t.Errorf("[%s] `--bucket %s` 一行都没列（收窄把桶点丢了）", c.name, c.bucket)
			continue
		}
		for _, row := range recsOut {
			if row["bucket"] != c.bucket {
				t.Errorf("[%s] `--bucket %s` 列了别的桶 %v", c.name, c.bucket, row["bucket"])
			}
		}
	}
}

// TestGapStatusBucketFlagRegistered —— 旗标登记面（`main.go` 两处待插行）的机读记号。
//
// ★ 本单允许面 = `family_gap.go` + 测试件（`main.go` 禁改）⇒ 登记落地前本用例 **skip**；
// 登记一落地（`valueFlagName` 收 `--bucket` + `gap status` 的 usage 串宣告它）就自动变成硬断言：
// 届时若实现没接上（`cmdGapStatus` 不认这枚旗标），本用例当场红。
func TestGapStatusBucketFlagRegistered(t *testing.T) {
	var usage string
	for _, c := range catalog() {
		if strings.Join(c.path, " ") == "gap status" {
			usage = c.usage
			break
		}
	}
	if usage == "" {
		t.Fatalf("命令树里找不到 `gap status`")
	}
	if !strings.Contains(usage, "--bucket") {
		t.Skip("待插行（main.go 不在本单允许面）：① `valueFlagName` 的取值旗标 case 里加 `\"--bucket\",`；" +
			"② `gap status` 的 usage 串加 `[--bucket <桶名>…]`")
	}
	// 登记落地后：真跑一次（走完整 dispatch），旗标必须被认（✗ 不许退「未知旗标」）。
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	gap41Write(t, dir, gap41Twelve())
	if rc, out, errb := gapShowAxisRun(t, "gap", "status", "--bucket", "u07", "--json", "bucket"); rc != exitOK {
		t.Fatalf("`gap status --bucket u07` 要 rc=0（登记 + 实现都到位），得到 %d · stdout=%q · stderr=%q", rc, out, errb)
	}
}

// ── ② 只读借用真源档 ────────────────────────────────────────────────────────────────

// TestGapBorrowResolve —— ② 的取值判据（四态 · 逐形态）。
func TestGapBorrowResolve(t *testing.T) {
	t.Run("未设", func(t *testing.T) {
		t.Setenv(gapBorrowEnv, "")
		if p, armed, err := gapBorrowResolve(); armed || err != nil || p != "" {
			t.Fatalf("未设要 (\\\"\\\", false, nil)，得到 (%q, %v, %v)", p, armed, err)
		}
	})
	t.Run("real", func(t *testing.T) {
		t.Setenv(gapBorrowEnv, gapBorrowReal)
		p, armed, err := gapBorrowResolve()
		if !armed || err != nil {
			t.Fatalf("`real` 要 armed 且无错，得到 (%q, %v, %v)", p, armed, err)
		}
		if !strings.HasSuffix(p, filepath.Join(".zerg", "state", gapLedgerFile)) {
			t.Errorf("`real` 应落 `~/.zerg/state/%s`，得到 %q", gapLedgerFile, p)
		}
	})
	t.Run("绝对目录", func(t *testing.T) {
		d := t.TempDir()
		t.Setenv(gapBorrowEnv, d)
		if p, armed, err := gapBorrowResolve(); !armed || err != nil || p != filepath.Join(d, gapLedgerFile) {
			t.Fatalf("绝对目录要展开成 <dir>/%s，得到 (%q, %v, %v)", gapLedgerFile, p, armed, err)
		}
	})
	t.Run("绝对件", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "x.jsonl")
		t.Setenv(gapBorrowEnv, f)
		if p, armed, err := gapBorrowResolve(); !armed || err != nil || p != f {
			t.Fatalf("绝对件要原样，得到 (%q, %v, %v)", p, armed, err)
		}
	})
	t.Run("相对路径是错值", func(t *testing.T) {
		t.Setenv(gapBorrowEnv, "relative/dir")
		if _, armed, err := gapBorrowResolve(); !armed || err == nil {
			t.Fatalf("相对路径要 armed 且报错（✗ 静默回落），得到 armed=%v err=%v", armed, err)
		}
	})
}

// TestGapBorrowReadOnly —— ② 的主判据（读面借真源 · 写面拒执 · 真源逐字节不变）。
func TestGapBorrowReadOnly(t *testing.T) {
	sandbox := t.TempDir()   // 探针状态目录（**里面没有账** —— 正是「要读账得先拷」的那个困境）
	borrowDir := t.TempDir() // 借来的真源（真账的替身）
	recs := []map[string]any{
		gap41Rec("GAP-20990003-A", "core/a.go", "modA", "famA", "仍缺", "P1", "2026-09-01T00:00:00+08:00"),
		gap41Rec("GAP-20990003-B", "core/b.go", "modB", "famB", "仍缺", "P2", "2026-09-01T00:00:00+08:00"),
	}
	ledger := gap41Write(t, borrowDir, recs)
	before := gap41SHA(t, ledger)

	t.Setenv("ZERG_STATE_DIR", sandbox)

	// 负控①：未开档 ⇒ 照旧「真源不在盘上」rc=8（既有行为一字不变）。
	rc, out, errb := gapShowAxisRun(t, "gap", "ls", "--json", "id")
	if rc != exitBlocked {
		t.Fatalf("未开档时沙箱无账应 rc=8，得到 %d · stdout=%q · stderr=%q", rc, out, errb)
	}
	if strings.Contains(errb, "只读借用档") {
		t.Errorf("未开档时不该出现借用档自报，stderr=%q", errb)
	}

	// 正控：开档 ⇒ 读面直接读真源（不拷），条数与真源相符 + stderr 自报。
	t.Setenv(gapBorrowEnv, borrowDir)
	rc, out, errb = gapShowAxisRun(t, "gap", "ls", "--json", "id")
	if rc != exitOK {
		t.Fatalf("借用档下 `gap ls` 要 rc=0（读得到真源），得到 %d · stdout=%q · stderr=%q", rc, out, errb)
	}
	var env struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("解不了包封：%v · %q", err, out)
	}
	if len(env.Items) != len(recs) {
		t.Errorf("借用档读到的条数应 = 真源 %d 条，得到 %d", len(recs), len(env.Items))
	}
	if !strings.Contains(errb, "只读借用档") {
		t.Errorf("借用档必须自报（✗ 静默换真源），stderr=%q", errb)
	}
	// 沙箱里不许被「顺手拷一份」：读面零写盘。
	if _, err := os.Stat(filepath.Join(sandbox, gapLedgerFile)); err == nil {
		t.Errorf("借用档下读面**不许**往沙箱里拷账（%s 出现了）", filepath.Join(sandbox, gapLedgerFile))
	}

	// 负控②：错值 ⇒ 用法错 2（✗ 落成「真源读不到 8」= 把旋钮写错误报成账坏了）。
	t.Setenv(gapBorrowEnv, "relative/dir")
	if rc, out, errb = gapShowAxisRun(t, "gap", "ls", "--json", "id"); rc != exitUsage {
		t.Fatalf("借用档错值要 rc=2（用法错），得到 %d · stdout=%q", rc, out)
	}
	if !strings.Contains(errb, "不是绝对路径") {
		t.Errorf("错值判词要点名「不是绝对路径」，stderr=%q", errb)
	}

	// 写面①：写命令入口的**早拒**（rc=2 · 在任何盘面动作之前 —— 锁 / 审计一个字都不落）。
	t.Setenv(gapBorrowEnv, borrowDir)
	rc, out, errb = gapShowAxisRun(t, "gap", "add", "--symptom", "借用档下不该入账",
		"--handmade", "手搓", "--impact", "命令面", "--want-family", "gap", "--want-action", "status",
		"--repro-cmd", "zerg gap status", "--verify-cmd", "zerg doctor", "--yes")
	if rc != exitUsage {
		t.Fatalf("借用档下 `gap add` 要 rc=2（只读档拒写），得到 %d · stdout=%q · stderr=%q", rc, out, errb)
	}
	if !strings.Contains(errb, "只读借用档") {
		t.Errorf("早拒判词要点名只读借用档，stderr=%q", errb)
	}

	// 写面②：兜底硬闸（`gapWriteLedger`）—— 走另一条写路（`gap note`：不经早拒面），
	// 也不许把借来的真源改掉；且拒绝文案必须点名「拒写真源 / 一字未动」。
	rc, out, errb = gapShowAxisRun(t, "gap", "note", "GAP-20990003-A", "--text", "借用档下不该写得进去", "--yes")
	if rc == exitOK {
		t.Fatalf("借用档下 `gap note` 必须拒执，却 rc=0 · stdout=%q", out)
	}
	if !strings.Contains(errb, "拒写真源") && !strings.Contains(errb, "只读借用档") {
		t.Errorf("兜底硬闸的判词要点名借用档 / 拒写真源，stderr=%q", errb)
	}
	if after := gap41SHA(t, ledger); after != before {
		t.Errorf("借来的真源被改了！before=%s after=%s", before, after)
	}
	if b, err := os.ReadFile(ledger); err != nil || !strings.Contains(string(b), `"state":"仍缺"`) {
		t.Errorf("真源内容应逐字节不变（A 仍 `仍缺`），读回=%v err=%v", string(b), err)
	}
}

// gap41SHA —— 一件的 sha256（对拍「逐字节不变」用）。
func gap41SHA(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读不了 %s：%v", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ── ③ 用法类错误不许吐包封（stdout 零字节）──────────────────────────────────────────

// TestGapFamilyUsageErrorsEmitNoEnvelope —— ③ 全族逐动作跑：rc=2 ⇒ stdout **零字节**。
func TestGapFamilyUsageErrorsEmitNoEnvelope(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	gap41Write(t, dir, gap41Twelve())

	cases := [][]string{
		{"gap", "ls", "--json", "zzz"},
		{"gap", "ls", "--json"},
		{"gap", "ls", "--state", "bogus"},
		{"gap", "ls", "--unit", ""},
		{"gap", "status", "--json", "zzz"},
		{"gap", "status", "--json"},
		{"gap", "status", "--by", "bogus"},
		{"gap", "status", "--top", "0"},
		{"gap", "status", "--state", "bogus"},
		{"gap", "status", "--unit", ""},
		{"gap", "status", "--grep", ""},
		{"gap", "show", "--json", "zzz"},
		{"gap", "show", "GAP-NOPE", "--json", "zzz"},
		{"gap", "plan"},
		{"gap", "plan", "--json", "zzz"},
		{"gap", "hints", "--json", "zzz"},
		{"gap", "idea", "ls", "--json", "zzz"},
		{"gap", "idea", "ls", "--source", "bogus"},
		{"gap", "idea", "stats", "--json", "zzz"},
		{"gap", "assign", "ls", "--json", "zzz"},
		{"gap", "add"},
		{"gap", "add", "--json", "zzz"},
		{"gap", "note", "--json", "zzz"},
		{"gap", "set-state", "--json", "zzz"},
		{"gap", "set-state"},
		{"gap", "set-unit", "--json", "zzz"},
		{"gap", "judge", "set", "--json", "zzz"},
		{"gap", "project", "--json", "zzz"},
		{"gap", "unproject", "--json", "zzz"},
		{"gap", "backfill-unit", "--json", "zzz"},
		{"gap", "bulk", "set-state", "--json", "zzz"},
		{"gap", "diff", "--json", "zzz"},
		{"gap", "export", "--json", "zzz"},
		{"gap", "regress", "--json", "zzz"},
		{"gap", "reread"},
		{"gap", "reread", "--json", "zzz"},
		{"gap", "verify", "--json", "zzz"},
		{"gap", "verify-one", "--json", "zzz"},
		{"gap", "receipt", "check"},
		{"gap", "receipt", "new"},
		{"gap", "receipt", "new", "--json", "zzz"},
		{"gap", "receipt", "apply", "--json", "zzz"},
		{"gap", "idea", "add", "--json", "zzz"},
		{"gap", "idea", "promote", "--json", "zzz"},
		{"gap", "idea", "sweep", "--json", "zzz"},
		{"gap", "idea", "discard", "--json", "zzz"},
		{"gap", "ls", "--no-such-flag", "1"},
	}
	for _, argv := range cases {
		rc, out, errb := gapShowAxisRun(t, argv...)
		if rc != exitUsage {
			t.Errorf("`zerg %s` 要 rc=2（用法错），得到 %d · stdout=%q · stderr=%q",
				strings.Join(argv, " "), rc, out, errb)
			continue
		}
		if out != "" {
			t.Errorf("用法错（rc=2）stdout 必须**零字节**，`zerg %s` 却吐了 %d 字节：%q",
				strings.Join(argv, " "), len(out), out)
		}
	}

	// ★ 正控（判据的牙）：rc=8 的 blocked 读数**该**吐包封 —— 否则上面那批「零字节」
	//   在「命令根本不输出」时也会假绿。用「沙箱里没有账」造这一档。
	empty := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", empty)
	rc, out, errb := gapShowAxisRun(t, "gap", "ls", "--json", "id")
	if rc != exitBlocked {
		t.Fatalf("正控：沙箱无账时 `gap ls --json id` 要 rc=8，得到 %d · stderr=%q", rc, errb)
	}
	if out == "" {
		t.Errorf("正控失败：rc=8 的 blocked 读数应吐包封（items=null），却零字节 —— 上面那批断言会假绿")
	}
}
