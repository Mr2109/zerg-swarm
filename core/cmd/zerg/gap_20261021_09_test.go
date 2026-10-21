// gap_20261021_09_test.go —— 缺口 `GAP-20261021-09` 的自检（`gap status` 的帮助声明 ⟷ 实现脱节）。
//
// 病（两处，同因：**旗标在册、帮助不列、实现不读**）：
//
//	① 帮助用法串（`commands[]` 里 `gap status` 的 `usage`）**不列** `--unit` / `--module` / `--grep`
//	   三枚 —— 而 `--unit` / `--module` 其实能用（`GAP-20260929-39` 已修），`--grep` 完全没实现。
//	② `--grep` **假过滤**：`zerg gap status --grep zzzz` 照样报全账「桶 322 个 · 全部 1518 条」
//	   ⇒ 传了等于没传，且**没有任何自报**（「带 grep / 不带 grep」两份读数在机器面上不可分辨）。
//
// 本件断言（收口后）：
//
//	① 用法串真列三枚（逐字 `[--unit <件路径|目录前缀>] [--module <模块前缀>] [--grep <关键词>]`）；
//	② `--grep` 真过滤：命中 2 条 / 2 桶（全量 5 条 / 3 桶）· 大小写不敏感 · 一条多处命中只算一条；
//	③ 与 `--unit` **AND** 叠加（不是互相覆盖）；
//	④ 负控：不存在的串 ⇒ 零命中（rc=1 · 「与筛选条件相符 0 条」· 机器面 `items` 为空、
//	   `meta.total`/`meta.hits` 都为 0）—— 「零命中」不是全量、也不是绿；
//	⑤ 空值闸：`--grep` 给空值 ⇒ 用法错 2（空值不许当「不过滤」静默放行）；
//	⑥ 收窄回显：`meta.query` 带 `"grep"`（带 / 不带 grep 的两份读数可分辨）；
//	⑦ 未给 `--grep` ⇒ `meta.query` 里没有 `"grep"` 这一格（不给没要的收窄背书）。
//
// ★ 本件名 `TestGapStatusHelpDeclaresFilters` 正是缺口登记的 `verify_cmd` 点名的那个测试
// （`zerg code find TestGapStatusHelpDeclaresFilters --path core/cmd/zerg`）⇒ 本笔落地即能按登记口径消案。
//
// 落点：`package main`（够得着包级桥 `RunForTest = run`）；夹具复用 `family_gap_show_axis_test.go`
// 的 `gapShowAxisRec` / `gapShowAxisWrite` / `gapShowAxisRun` / `gapShowAxisMeta`（不另抄第二套）。
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// gapStatusGrepHeader —— 人面表头里的两个数（本筛选后条数 / 桶数）—— 现从真输出里取，
// ✗ 不从 `meta` 抄（`meta.total` 是**桶数**，条数只在人面表头与 `meta.count` 之外出现 ⇒ 此处现读文字）。
func gapStatusGrepHeader(t *testing.T, out string) (rows, buckets int) {
	t.Helper()
	var okR, okB bool
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "zerg gap status --by ") {
			continue
		}
		fields := strings.Split(line, " · ")
		for _, f := range fields {
			if strings.HasPrefix(f, "本筛选后 ") && strings.HasSuffix(f, " 条") {
				rows, okR = atoiOrPanic(strings.TrimSuffix(strings.TrimPrefix(f, "本筛选后 "), " 条")), true
			}
			if strings.HasPrefix(f, "桶 ") {
				head := strings.TrimPrefix(f, "桶 ")
				if i := strings.Index(head, " 个"); i > 0 {
					buckets, okB = atoiOrPanic(head[:i]), true
				}
			}
		}
	}
	if !okR || !okB {
		t.Fatalf("人面表头缺「本筛选后 N 条」/「桶 N 个」（rows=%v buckets=%v）：%q", okR, okB, out)
	}
	return rows, buckets
}

// gapStatusCountOf —— 取桶行 `count`（机器面是**字符串**形态）· 取不出 ⇒ 当场炸。
func gapStatusCountOf(t *testing.T, it map[string]any) int {
	t.Helper()
	switch v := it["count"].(type) {
	case string:
		return atoiOrPanic(v)
	case float64:
		return int(v)
	}
	t.Fatalf("桶行缺 `count`：%v", it)
	return 0
}

// gapStatusQueryOf —— `meta.query` 是**对象**（不是串）：解成 map 再逐格判。
func gapStatusQueryOf(t *testing.T, meta map[string]any) map[string]any {
	t.Helper()
	q, ok := meta["query"].(map[string]any)
	if !ok {
		t.Fatalf("`meta.query` 要是对象，得到 %T(%v)", meta["query"], meta["query"])
	}
	return q
}

// atoiOrPanic —— 夹具自用：把表头里的十进制数字取出来（取不出 ⇒ 测试当场炸，✗ 不静默 0）。
func atoiOrPanic(s string) int {
	n := 0
	if s == "" {
		panic("空串不是数")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			panic("非法数字：" + s)
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// TestGapStatusHelpDeclaresFilters —— ① 帮助用法串真列 `--unit` / `--module` / `--grep`。
func TestGapStatusHelpDeclaresFilters(t *testing.T) {
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
	for _, want := range []string{
		"[--unit <件路径|目录前缀>]",
		"[--module <模块前缀>]",
		"[--grep <关键词>]",
	} {
		if !strings.Contains(usage, want) {
			t.Errorf("`gap status` 的用法串没宣告 %q：%s", want, usage)
		}
	}
}

// TestGapStatusGrepReallyFilters —— ②③④⑤⑥⑦ 真过滤 / AND / 负控 / 空值闸 / 回显。
func TestGapStatusGrepReallyFilters(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	u1, u2, u3 := "core/cmd/zerg/main.go", "core/cmd/zerg/family_gap.go", "core/internal/version/version.go"
	mk := func(id, unit, symptom string) map[string]any {
		r := gapShowAxisRec(id, unit, "仍缺")
		r["symptom"] = symptom
		return r
	}
	recs := []map[string]any{
		mk("GAP-20990701-A", u1, "斑马 Zebra 只中一条（大写入账）"),
		mk("GAP-20990701-B", u1, "与关键词无关"),
		mk("GAP-20990701-C", u2, "小写 zebra 也要中（大小写不敏感）"),
		mk("GAP-20990701-D", u2, "无关"),
		mk("GAP-20990701-E", u3, "无关"),
	}
	gapShowAxisWrite(t, dir, recs)

	// 全量基准（不带 --grep）：5 条 / 3 桶。
	rc, out, errb := gapShowAxisRun(t, "gap", "status")
	if rc != 0 {
		t.Fatalf("全量 gap status 要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	if rows, buckets := gapStatusGrepHeader(t, out); rows != 5 || buckets != 3 {
		t.Fatalf("全量基准应为 5 条 / 3 桶，得到 %d 条 / %d 桶", rows, buckets)
	}

	// ② `--grep zebra`（小写 · 大小写不敏感）：A、C 两条进 ⇒ 2 条 / 2 桶（< 全量 ⇒ 真过滤）。
	rc, out, errb = gapShowAxisRun(t, "gap", "status", "--grep", "zebra", "--json", "bucket,count")
	if rc != 0 {
		t.Fatalf("`--grep zebra` 要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	var env struct {
		Items []map[string]any `json:"items"`
		Meta  map[string]any   `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("解不了包封：%v · %q", err, out)
	}
	if want := 2.0; env.Meta["total"] != want || env.Meta["hits"] != want {
		t.Fatalf("`--grep zebra` 应为 2 桶（total/hits 都是 2），得到 %v / %v", env.Meta["total"], env.Meta["hits"])
	}
	sum := 0
	for _, it := range env.Items {
		sum += gapStatusCountOf(t, it)
	}
	if sum != 2 {
		t.Errorf("`--grep zebra` 的桶内条数之和应为 2（A/C 各一条），得到 %d · items=%v", sum, env.Items)
	}
	// ⑥ 收窄回显带 grep。
	if q := gapStatusQueryOf(t, env.Meta); q["grep"] != "zebra" {
		t.Errorf("`meta.query.grep` 要回显 `zebra`，得到 %v（query=%v）", q["grep"], q)
	}

	// 人面同一个数（表头 2 条 / 2 桶）—— 人机两面对得上。
	_, out, _ = gapShowAxisRun(t, "gap", "status", "--grep", "zebra")
	if rows, buckets := gapStatusGrepHeader(t, out); rows != 2 || buckets != 2 {
		t.Errorf("`--grep zebra` 人面应为 2 条 / 2 桶，得到 %d 条 / %d 桶", rows, buckets)
	}

	// ③ AND 叠加：`--grep zebra --unit <u1>` ⇒ 只剩 A ⇒ 1 条 / 1 桶。
	_, out, _ = gapShowAxisRun(t, "gap", "status", "--grep", "zebra", "--unit", u1)
	if rows, buckets := gapStatusGrepHeader(t, out); rows != 1 || buckets != 1 {
		t.Errorf("`--grep zebra --unit %s`（AND）应为 1 条 / 1 桶，得到 %d 条 / %d 桶", u1, rows, buckets)
	}

	// ④ 负控：不存在的串 ⇒ 零命中（rc=1 · 0 条 · 机器面空）。
	rc, out, errb = gapShowAxisRun(t, "gap", "status", "--grep", "zzzz")
	if rc != 1 {
		t.Fatalf("`--grep zzzz` 要 rc=1（零命中不是绿），得到 %d · stderr=%q", rc, errb)
	}
	if !strings.Contains(errb, "与筛选条件相符 0 条") {
		t.Errorf("零命中判词要报「与筛选条件相符 0 条」，stderr=%q", errb)
	}
	rc, out, _ = gapShowAxisRun(t, "gap", "status", "--grep", "zzzz", "--json", "bucket,count")
	if rc != 1 {
		t.Fatalf("`--grep zzzz --json` 也要 rc=1，得到 %d", rc)
	}
	env.Items, env.Meta = nil, nil
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("解不了包封：%v · %q", err, out)
	}
	if len(env.Items) != 0 {
		t.Errorf("零命中 `items` 要为空（桶 0 个），得到 %v", env.Items)
	}
	if env.Meta["total"] != 0.0 || env.Meta["hits"] != 0.0 {
		t.Errorf("零命中 `meta.total`/`meta.hits` 都要 0，得到 %v / %v", env.Meta["total"], env.Meta["hits"])
	}

	// ⑤ 空值闸：`--grep ""` ⇒ 用法错 2（空值不是「不过滤」）。
	if rc, _, _ = gapShowAxisRun(t, "gap", "status", "--grep", ""); rc != 2 {
		t.Errorf("`--grep` 给空值要 rc=2（用法错），得到 %d", rc)
	}

	// ⑧ `--by tier` 面：`--grep` 也算**收窄**（进 `hasNarrow`）—— 否则守恒式拿「三桶之和（收窄后）」
	//    去对「总账（1518）」⇒ 当场判红。实测哑化 `hasNarrow` 那一项：`--by tier --grep usage` 得
	//    「三桶之和 97 ≠ 总账 1518 ⇒ 判红 1」（本条断言就是那颗牙）。
	rc, out, errb = gapShowAxisRun(t, "gap", "status", "--by", "tier", "--grep", "zebra")
	if rc != 0 {
		t.Fatalf("`--by tier --grep zebra` 要 rc=0（守恒式不许因 grep 而判红），得到 %d · stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "守恒式：三桶之和 2 == 现读总数 2 ✓") {
		t.Errorf("`--by tier --grep zebra` 的守恒式要点名「三桶之和 2 == 现读总数 2 ✓」，得到：%s", out)
	}

	// ⑦ 不带 `--grep` ⇒ `meta.query` 里没有 `"grep"` 这一格。
	_, out, _ = gapShowAxisRun(t, "gap", "status", "--json", "bucket")
	env.Items, env.Meta = nil, nil
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("解不了包封：%v · %q", err, out)
	}
	if q := gapStatusQueryOf(t, env.Meta); q["grep"] != nil {
		t.Errorf("没给 `--grep` 时 `meta.query` 不该有 grep 格，得到 %v", q)
	}
}
