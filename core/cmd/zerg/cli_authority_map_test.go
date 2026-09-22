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
