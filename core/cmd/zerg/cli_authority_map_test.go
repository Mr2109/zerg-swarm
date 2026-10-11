// cli_authority_map_test.go —— `Q-021` 命令面三态 ↔ 控制层三态**映射表**（波① `T3`）的判据机检。
//
// 判据（`任务清单-缺口收口-20260923.md` §`T3` 原样三条 · 逐条落成断言）：
//
//	① 映射表落到登记件（机器可读）—— `core/internal/contract/authority-map.json`
//	② **干跑与真跑授权判据 identical**（同一命令，加/不加写旗标，授权判定结果逐字同）
//	③ 表里任一枚举值在两集合外 ⇒ 判红（**成对负控**：喂一份被改坏的表，判定口必须报错）
//
// 闭集判定的实现就落在本件的 `checkAuthorityMap` 上（**唯一一处**判定口）：真表 ⇒ 0 问题；
// 三份被改坏的表 ⇒ 各自必须报错（集合外的值 / 命令面旗标不在闭集 / 控制层值没人用上）。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func authorityMapPath() string {
	return filepath.Join("..", "..", "internal", "contract", "authority-map.json")
}

func loadAuthorityMap(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(authorityMapPath())
	if err != nil {
		t.Fatalf("映射表读不到（%s）：%v", authorityMapPath(), err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("映射表不是 JSON：%v", err)
	}
	return m
}

func setOf(t *testing.T, m map[string]any, key string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	rows, ok := m[key].([]any)
	if !ok || len(rows) == 0 {
		t.Fatalf("`%s` 不是非空数组（映射表缺一格）", key)
	}
	for _, r := range rows {
		row, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("`%s` 里有不是对象的一行", key)
		}
		v, _ := row["值"].(string)
		if v == "" {
			t.Fatalf("`%s` 里有空值行", key)
		}
		out[v] = true
	}
	return out
}

// checkAuthorityMap —— 「两侧各自闭集 + 映射只许用集合里的值 + 每个值都得有人用」的判定口。
// 返回问题清单（空 = 判绿）。**唯一一处**判定逻辑：正控跑真表、负控跑被改坏的表。
func checkAuthorityMap(m map[string]any) []string {
	problems := []string{}
	if m["schema"] != "zerg/authority-map/v1" {
		problems = append(problems, "schema 不是 zerg/authority-map/v1")
	}
	cli := map[string]bool{}
	cliBare := map[string]bool{}
	for _, r := range m["命令面闭集"].([]any) {
		v := r.(map[string]any)["值"].(string)
		cli[v] = true
		cliBare[bareFlag(v)] = true
	}
	ctl := map[string]bool{}
	for _, r := range m["控制层闭集"].([]any) {
		v := r.(map[string]any)["值"].(string)
		ctl[v] = true
	}
	usedCtl := map[string]bool{}
	rows, ok := m["映射"].([]any)
	if !ok || len(rows) == 0 {
		return append(problems, "`映射` 不是非空数组")
	}
	for i, r := range rows {
		row := r.(map[string]any)
		tag := "映射[第 " + itoa(i+1) + " 行]"
		vals, ok := row["命令面"].([]any)
		if !ok {
			problems = append(problems, tag+" 的 `命令面` 不是数组")
			continue
		}
		for _, v := range vals {
			s, _ := v.(string)
			if !cli[s] && !cliBare[bareFlag(s)] {
				problems = append(problems, tag+" 的命令面值 "+s+" **不在命令面闭集里**")
			}
		}
		c, _ := row["控制层"].(string)
		if !ctl[c] {
			problems = append(problems, tag+" 的控制层值 "+c+" **不在控制层闭集里**")
		} else {
			usedCtl[c] = true
		}
	}
	for v := range ctl {
		if !usedCtl[v] {
			problems = append(problems, "控制层闭集里的 "+v+" **没有任何映射行用它**（闭集里挂着一个死值 = 表没写死）")
		}
	}
	return problems
}

// bareFlag —— 旗标名归一：`--confirm=<目标>` → `--confirm`（映射行写旗标名或带占位值都认）。
func bareFlag(s string) string {
	if j := strings.IndexByte(s, '='); j >= 0 {
		return s[:j]
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}
	return out
}

// 判据 ① + ③：真表判绿；三份「改坏的表」各自必须判红（成对负控 —— 探针没牙即红）。
func TestAuthorityMapClosedSetsAndTeeth(t *testing.T) {
	real := loadAuthorityMap(t)
	if p := checkAuthorityMap(real); len(p) != 0 {
		t.Fatalf("真表被判红（要 0 问题）：%v", p)
	}
	// 两侧闭集都得有内容，且得覆盖到三个控制层词（allow / block / require_approval）
	if got := len(setOf(t, real, "命令面闭集")); got != 3 {
		t.Errorf("命令面闭集 = %d 枚（要 3：干跑 / 确认 / 写旗标）", got)
	}
	if got := len(setOf(t, real, "控制层闭集")); got != 3 {
		t.Errorf("控制层闭集 = %d 枚（要 3：allow / block / require_approval）", got)
	}

	// 负控一：控制层写一个集合外的词
	b1 := loadAuthorityMap(t)
	b1["映射"].([]any)[0].(map[string]any)["控制层"] = "allow_all"
	if len(checkAuthorityMap(b1)) == 0 {
		t.Fatalf("负控一没牙：控制层给了集合外的值还判绿")
	}
	// 负控二：命令面写一枚不在闭集里的旗标
	b2 := loadAuthorityMap(t)
	b2["映射"].([]any)[0].(map[string]any)["命令面"] = []any{"--sudo"}
	if len(checkAuthorityMap(b2)) == 0 {
		t.Fatalf("负控二没牙：命令面给了集合外的旗标还判绿")
	}
	// 负控三：把「block」那一行整行抹掉 ⇒ 控制层闭集里 block 没人用 ⇒ 判红
	b3 := loadAuthorityMap(t)
	kept := []any{}
	for _, r := range b3["映射"].([]any) {
		if r.(map[string]any)["控制层"] == "block" {
			continue
		}
		kept = append(kept, r)
	}
	b3["映射"] = kept
	if len(checkAuthorityMap(b3)) == 0 {
		t.Fatalf("负控三没牙：闭集里的 block 没人用上还判绿（= 表没写死）")
	}
}

// 判据 ② **干跑与真跑授权判据 identical**：同一命令、同一组旗标（只差写旗标），两档必须**点名同一个词**。
// 例表就写在登记件里（`同判据例`）⇒ 加一条命令要同批加一行例。
func TestAuthorityMapDryRunAndRealShareOneVerdict(t *testing.T) {
	m := loadAuthorityMap(t)
	rows, ok := m["同判据例"].([]any)
	if !ok || len(rows) == 0 {
		t.Fatalf("登记件里没有 `同判据例`（干跑/真跑同判据要逐条现跑）")
	}
	_, fleet := fleetFixtureAt(t) // 复用 `T1a` 的名册夹具（同包）
	for _, r := range rows {
		c := r.(map[string]any)
		name, _ := c["命令"].(string)
		want, _ := c["要同的词"].(string)
		dry := substCase(c["干跑"].([]any), fleet)
		real := substCase(c["真跑"].([]any), fleet)
		wantRC := int(c["真跑退码"].(float64))
		rcDry, outDry, errDry := runCapture(dry...)
		if rcDry != 0 {
			t.Fatalf("%s：干跑 rc=%d（要 0）· stderr=%s", name, rcDry, errDry)
		}
		if !strings.Contains(outDry+errDry, want) {
			t.Fatalf("%s：干跑那一档没点名 %q ⇒ 真跑判据没一并给（两档不是同一条判据）\n%s%s", name, want, outDry, errDry)
		}
		rcReal, outReal, errReal := runCapture(real...)
		if rcReal != wantRC {
			t.Fatalf("%s：真跑（缺写旗标）rc=%d（要 %d）· stderr=%s", name, rcReal, wantRC, errReal)
		}
		if outReal != "" {
			t.Fatalf("%s：真跑被拒时 stdout 该空，得到 %q", name, outReal)
		}
		if !strings.Contains(errReal, want) {
			t.Fatalf("%s：真跑那一档没说 %q ⇒ 两档判据分叉\n%s", name, want, errReal)
		}
	}
}

// substCase 把例表里的占位（`__MODEL__` / `__FLEET__`）换成现跑的夹具值。
func substCase(argv []any, fleet string) []string {
	out := []string{}
	for _, a := range argv {
		s, _ := a.(string)
		s = strings.ReplaceAll(s, "__MODEL__", "m1")
		s = strings.ReplaceAll(s, "__FLEET__", fleet)
		out = append(out, s)
	}
	return out
}

// ── 判据 ④ 全格齐 + 「无对应」一格**显式写出**（§一 序96 · 组4 `W-11` · 波5 补强）──────────────
//
// 为什么单独一条判据：`checkAuthorityMap` 判的是「表里用到的值都在闭集里」；本条判的是**反过来的
// 那一面** —— 命令面旗标的**每一个子集**都得有格：要么在 `映射` 里给出对应，要么在 `无对应` 里
// **逐字写「无对应」**。少了任何一条 = 表上留了一个**空格**（没写死 ⇒ 下一个人自己发明）。
//
// 八子集（3 枚旗标）= ∅ · `--dry-run` · `--confirm` · `--yes` · `--dry-run+--yes` ·
// `--dry-run+--confirm` · `--confirm+--yes` · `--dry-run+--confirm+--yes`。
func authorityFlagSubsets(flags []string) map[string]bool {
	out := map[string]bool{}
	n := len(flags)
	for mask := 0; mask < (1 << n); mask++ {
		key := []string{}
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				key = append(key, flags[i])
			}
		}
		sort.Strings(key)
		out[strings.Join(key, "+")] = true
	}
	return out
}

// authorityRowKey —— 一行 `命令面` 归一成子集键（旗标名去 `=<值>`、排序、`+` 连接；空集 ⇒ ""）。
func authorityRowKey(vals []any) string {
	key := []string{}
	for _, v := range vals {
		s, _ := v.(string)
		key = append(key, bareFlag(s))
	}
	sort.Strings(key)
	return strings.Join(key, "+")
}

// authorityGridProblems —— 全格齐判定口（**唯一一处**：正控跑真表、三份负控跑被改坏的表）。
func authorityGridProblems(m map[string]any) []string {
	problems := []string{}
	flags := []string{}
	for _, r := range m["命令面闭集"].([]any) {
		if v, _ := r.(map[string]any)["值"].(string); v != "" {
			flags = append(flags, bareFlag(v))
		}
	}
	sort.Strings(flags)
	want := authorityFlagSubsets(flags)
	got := map[string]bool{}
	for _, r := range m["映射"].([]any) {
		got[authorityRowKey(r.(map[string]any)["命令面"].([]any))] = true
	}
	noMap, ok := m["无对应"].([]any)
	if !ok {
		problems = append(problems, "`无对应` 一格**没写出来**（缺这一键 ⇒ 有空还是有对应无从判别）")
	}
	for i, r := range noMap {
		tag := "`无对应`[第 " + itoa(i+1) + " 行]"
		row, ok := r.(map[string]any)
		if !ok {
			problems = append(problems, tag+" 不是对象")
			continue
		}
		if vals, ok := row["命令面"].([]any); ok {
			got[authorityRowKey(vals)] = true
		} else {
			problems = append(problems, tag+" 缺 `命令面`（子集没点名 ⇒ 判不出它填的是哪一格）")
		}
		if reason, _ := row["理由"].(string); !strings.Contains(reason, "无对应") {
			problems = append(problems, tag+" 的 `理由` 没逐字写「无对应」（序96 判据：无对应的格子必须**显式**写）")
		}
		if ctl, _ := row["控制层"].(string); ctl == "" {
			problems = append(problems, tag+" 缺 `控制层`")
		}
	}
	for k := range want {
		if !got[k] {
			name := k
			if name == "" {
				name = "∅（一个旗标都不给）"
			}
			problems = append(problems, "命令面子集 "+name+" **一个格都没有**（既不在 `映射`、也不在 `无对应` ⇒ 表上是个空格）")
		}
	}
	return problems
}

func TestAuthorityMapGridCompleteness(t *testing.T) {
	real := loadAuthorityMap(t)
	if p := authorityGridProblems(real); len(p) != 0 {
		t.Fatalf("真表被判红（要 0 问题）：%v", p)
	}
	// 负控一：抹掉一条**有对应**的子集行（`--dry-run` + `--yes`）⇒ 那一格成空格 ⇒ 必须红
	b1 := loadAuthorityMap(t)
	kept := []any{}
	for _, r := range b1["映射"].([]any) {
		if authorityRowKey(r.(map[string]any)["命令面"].([]any)) == "--dry-run+--yes" {
			continue
		}
		kept = append(kept, r)
	}
	b1["映射"] = kept
	if len(authorityGridProblems(b1)) == 0 {
		t.Fatalf("负控一没牙：抹掉一条映射行（`--dry-run` + `--yes`）还判绿")
	}
	// 负控二：`无对应` 行不写「无对应」⇒ 必须红
	b2 := loadAuthorityMap(t)
	b2["无对应"] = []any{map[string]any{
		"命令面": []any{"--dry-run"}, "控制层": "require_approval", "理由": "干跑不执行 ⇒ 没有这一态",
	}}
	if len(authorityGridProblems(b2)) == 0 {
		t.Fatalf("负控二没牙：`无对应` 行不逐字写「无对应」还判绿")
	}
	// 负控三：把 `无对应` 整键抹掉 ⇒ 必须红（缺这一键 = 有没有空格无从判别）
	b3 := loadAuthorityMap(t)
	delete(b3, "无对应")
	if len(authorityGridProblems(b3)) == 0 {
		t.Fatalf("负控三没牙：把 `无对应` 整键抹掉还判绿")
	}
}

// ── 见证命令面（待拍 #22 落地 · `GAP-20260928-18`）────────────────────────────────────
//
// 缺口：`authority-map.json` 的 `映射` 每行只有 {命令面, 控制层, 判据}，**行内不带见证命令**
// ⇒ 表写 allow 而实现已收紧成 block 也能**长期无人红**（`GAP-20260928-18` 复现面）。
//
// 本单只钉**三个不写盘子集**当见证命令（真跑 · 只读形态）：∅ / `--dry-run` / `--dry-run --yes`。
// 其余（写盘类：`--yes` 真跑 / `--confirm <真目标>` …）⇒ **✗ 绝不真跑** ⇒ 显式登记
// 「无见证命令」＋理由（照仓内既有 `无对应` 一格体例，✗ 不留空）。
//
// 对账判口 = **rc ＋ stderr 点名/detail**：`block` 与 `require_approval` **同为 rc=2**，
// 光看 rc 分不开 —— 那正是本缺口的病灶 ⇒ 判口必须落到 stderr 的 `detail`/缺失旗标上。
type authorityWitness struct {
	argv   []string // 见证命令 argv（空 = 无见证命令 ⇒ 必写 reason）
	rc     int      // 现跑退码
	needle string   // stderr 必须点名的词 / detail（区分 allow / block / require_approval）
	ctl    string   // 见证命令应证实的控制层（要与该行 `控制层` 逐字同）
	reason string   // 无见证命令时：理由，须逐字含「无见证命令」
}

// authorityRowRawKey —— 一行 `命令面` **逐字**归一（含 `=<值>`；排序后 `+` 连接）。
// 与 `authorityRowKey` 的区别：后者**剥** `=<值>`（把 `--confirm=<目标>` 与 `--confirm=<别的值>`
// 并成同一格）。见证表面积按**控制层取值**分（一行 allow 一行 block）⇒ 必须用逐字形态。
func authorityRowRawKey(vals []any) string {
	key := []string{}
	for _, v := range vals {
		s, _ := v.(string)
		key = append(key, s)
	}
	sort.Strings(key)
	return strings.Join(key, "+")
}

// authorityWitnessPinned —— 本批钉死的三个**不写盘**子集（父代理 2026-10-10 口径）。
var authorityWitnessPinned = []string{"", "--dry-run", "--dry-run+--yes", "--confirm=<别的值>"}

// authorityWitnesses —— 见证命令面，键 = `authorityRowRawKey`。
var authorityWitnesses = map[string]authorityWitness{
	// 三个不写盘子集：给**真跑**见证命令（只读形态）。
	"": {
		argv:   []string{"model", "opts", "set", "m1", "--set", "temperature=0.8"},
		rc:     2,
		needle: "缺 --yes",
		ctl:    "require_approval",
	},
	"--dry-run": {
		argv:   []string{"model", "opts", "set", "m1", "--set", "temperature=0.8", "--dry-run"},
		rc:     0,
		needle: "--dry-run",
		ctl:    "allow",
	},
	"--dry-run+--yes": {
		argv:   []string{"model", "opts", "set", "m1", "--set", "temperature=0.8", "--dry-run", "--yes"},
		rc:     2,
		needle: "dry_run_yes_conflict",
		ctl:    "block",
	},

	// 写盘类子集：✗ 绝不真跑 ⇒ 显式登记「无见证命令」＋理由（照 `无对应` 体例）。
	"--confirm=<目标>": {
		reason: "无见证命令 —— `--confirm <真目标>` 属写盘形态（真跑会读口令、碰目标）；本单 ✗ 绝不真跑写盘形态 ⇒ 只登记、不给见证命令",
	},
	"--yes": {
		reason: "无见证命令 —— `--yes` 真跑**会写盘**（D2 实时生效）；本单 ✗ 绝不真跑写盘形态 ⇒ 只登记、不给见证命令",
	},
	"--confirm=<目标>+--yes": {
		reason: "无见证命令 —— 含 `--yes` 真跑会写盘，且带 `--confirm <真目标>`；本单 ✗ 绝不真跑写盘形态 ⇒ 只登记、不给见证命令",
	},
	"--confirm=<别的值>": {
		argv:   []string{"task", "pause", "T-zzz", "--confirm=T-zzz-other"},
		rc:     2,
		needle: "确认值不匹配",
		ctl:    "block",
	},
	"--confirm=<目标>+--dry-run": {
		reason: "无见证命令 —— 带 `--confirm <真目标>`（写盘形态的确认旗标）；本单 ✗ 绝不真跑写盘形态 ⇒ 只登记、不给见证命令",
	},
	"--confirm=<目标>+--dry-run+--yes": {
		reason: "无见证命令 —— 带 `--confirm <真目标>` 与 `--yes`（写盘形态）；本单 ✗ 绝不真跑写盘形态 ⇒ 只登记、不给见证命令",
	},
}

// TestAuthorityMapWitness —— 映射 9 行**逐行**要有见证命令登记（可见证 ⇒ 真跑比 rc ＋ stderr；
// 不可见证 ⇒ 显式「无见证命令」＋理由），且**可见证行的控制层要与现跑逐字对上**（映射漂移即红）。
//
// 牙口：改**某行控制层**（如把 `--dry-run` 行从 allow 改 block）⇒ 该行见证命令现跑仍给 allow
// ⇒ 本用例 FAIL（改前：无人红）。见本单两态 / 负控读数。
func TestAuthorityMapWitness(t *testing.T) {
	m := loadAuthorityMap(t)
	rows, ok := m["映射"].([]any)
	if !ok || len(rows) == 0 {
		t.Fatalf("`映射` 不是非空数组")
	}
	seen := map[string]bool{}
	ran := map[string]string{}
	for i, r := range rows {
		row := r.(map[string]any)
		vals, ok := row["命令面"].([]any)
		if !ok {
			t.Fatalf("映射[第 %d 行] 的 `命令面` 不是数组", i+1)
		}
		k := authorityRowRawKey(vals)
		ctl, _ := row["控制层"].(string)
		w, ok := authorityWitnesses[k]
		if !ok {
			t.Fatalf("映射[第 %d 行]（命令面 %v · 控制层 %s）**没有见证命令登记** —— 可见证 ⇒ 给 `argv`；不可见证 ⇒ 显式写「无见证命令」＋理由（✗ 别留空）", i+1, vals, ctl)
		}
		if seen[k] {
			t.Fatalf("映射[第 %d 行] 的命令面键 %q 与另一行重了", i+1, k)
		}
		seen[k] = true
		if len(w.argv) == 0 {
			if w.reason == "" || !strings.Contains(w.reason, "无见证命令") {
				t.Errorf("映射[第 %d 行]（%q）无见证命令，但 `reason` 没逐字写「无见证命令」（照 `无对应` 体例 ✗ 不许留空）", i+1, k)
			}
			continue
		}
		// 可见证支：控制层与表里逐字对 + 现跑比 rc ＋ stderr。
		if w.ctl != ctl {
			t.Errorf("映射[第 %d 行]（%q）表里写控制层 %q，而见证命令按 %q 判 ⇒ 表/实现分家（映射漂移）", i+1, k, ctl, w.ctl)
			continue
		}
		rc, _, errOut := runCapture(w.argv...)
		if rc != w.rc {
			t.Errorf("映射[第 %d 行]（%q · %s）见证命令现跑 rc=%d（要 %d）· stderr=%s", i+1, k, w.ctl, rc, w.rc, errOut)
			continue
		}
		if w.needle != "" && !strings.Contains(errOut, w.needle) {
			t.Errorf("映射[第 %d 行]（%q · %s）见证命令 stderr 没点名 %q（rc 分不开 block / require_approval ⇒ 判口要看 stderr）\nstderr=%s", i+1, k, w.ctl, w.needle, errOut)
			continue
		}
		ran[k] = errOut
	}
	// 反向：表面积里不许有对不上任何映射行的死条目。
	for k := range authorityWitnesses {
		if !seen[k] {
			t.Errorf("见证表面积 %q 对不上任何映射行（死条目 —— 表老了或键写歪了）", k)
		}
	}
	// 口径钉死：本批钉四个子集（三个不写盘子集 + `--confirm=<别的值>` 这一格：用不带干跑且不带 --yes 的安全形态取证）。
	for _, k := range authorityWitnessPinned {
		if w, ok := authorityWitnesses[k]; !ok || len(w.argv) == 0 {
			t.Errorf("钉死的见证子集 %q 没给 `argv`（本批三个不写盘子集必须可见证）", k)
		}
	}
	for k, w := range authorityWitnesses {
		pinned := false
		for _, p := range authorityWitnessPinned {
			if k == p {
				pinned = true
			}
		}
		if !pinned && len(w.argv) != 0 {
			t.Errorf("非钉死子集 %q 也给了见证命令（本批只钉三个不写盘子集 · ✗ 不扩到三子集外）", k)
		}
	}
	// 病灶交叉核：`block` 与 `require_approval` 同为 rc=2，靠 stderr 分开 ——
	// 两档各自的 detail 词**不许**出现在对方那档上（否则判口仍是「rc 一刀切」= 没牙）。
	if b, ok := ran["--dry-run+--yes"]; ok {
		if !strings.Contains(b, "dry_run_yes_conflict") {
			t.Errorf("block 档 stderr 没带 detail=`dry_run_yes_conflict`：%s", b)
		}
		if strings.Contains(b, "缺 --yes") {
			t.Errorf("block 档 stderr 也带 require_approval 的「缺 --yes」⇒ 两档 stderr 分不开：%s", b)
		}
	}
	if a, ok := ran[""]; ok {
		if strings.Contains(a, "dry_run_yes_conflict") {
			t.Errorf("require_approval 档 stderr 也带 block 的 detail ⇒ 两档 stderr 分不开：%s", a)
		}
	}
}
