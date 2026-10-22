// family_gap_setwant_test.go —— 缺口 `GAP-20261008-07`（want 无写面）的**回归牙**（本单新增件）。
//
// 面（现读实测）：缺口账的 `want_family` / `want_action` 两格**没有 CLI 写面** —— 同族五写面
// （`set-state` / `note` / `set-unit` / `judge set` / `assign`）都不碰这两格 ⇒ 拍板换了方向只能手改
// 盘上真源（`<状态目录>/zerg-cli-gaps.jsonl`）：绕审计、绕账写锁、绕一切校验。
//
// 治（本件钉住）：`gap set-want`（`family_gap_state.go:cmdGapSetWant`）—— 形制照同族 `gap set-unit`：
// 只改 want 三格 · `--dry-run` 恒 0 · **缺 `--yes` fail-closed 2** · `--evidence` 必填（落成 `notes`
// 末尾一条注）· 写前审计先落盘（`gapRewriteOne`）· 只重写目标那一行 · 写后读回自证 · 同值 ⇒ 幂等 0。
// 取值判据 = 族名**闭集**（`family_gap.go:gapWantFamilyClosed`：命令树顶层族 ∪ 账内现用原值）＋
// 动作名**形态闭集**（单一 token：非空 / 不以 `-` 开头 / 不含空白 / ≤128 rune）。
//
// ★ 本件的两态是**同一条 argv** 的两发：① 命令树里**没有**这条动作（= 改前的真态）⇒ rc=2 且账一个
// 字节不动；② 装上任之后 ⇒ rc=0 且账内那两格**逐字**变成要的值（其余键与其余行逐字节不变）。
// 装/卸只在件内（`gapSetWantTestInstall` · `t.Cleanup` 复原），✗ 不改仓内 `main.go`。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gapSetWantTestNode —— 生产节点件的**同一份**声明（`main.go` 的 `commands` 里那一格的字面量）。
// 之所以在测试件里装：本单边界划到「✗ 不动 `main.go`」（另几路正在改它）⇒ 生产入口那一行待落；
// 语义件（handler / 判据 / 审计 / 收口）全在 `family_gap*.go`，本件走**同一条** `run` 入口真跑。
func gapSetWantTestNode() *command {
	return &command{
		path:    []string{"gap", "set-want"},
		kind:    "GapSetWant",
		summary: "改一条缺口的 want 三格（写面 · `--dry-run` 恒 0 · 缺 `--yes` fail-closed 2）",
		usage:   "zerg gap set-want <GAP id> --want-family <族> --want-action <动作> [--want-argv <段>…] --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]",
		arity:   "any",
		args:    []string{"缺口 id（**恰好一条** —— 本动作不做批量）"},
		fields:  gapSetWantFields,
		danger: &dangerSpec{dangerD2, "缺口 id",
			"改真源里的 `want_family` / `want_action` / `want_argv` 三格 + 往 `notes` 追加一条改口径留痕注 + 审计一行（可逆：照审计那一格回写 · 留痕注删掉即回原状；`state` 逐字不动 · 其余行逐字节不变）；族名越界 / 动作名形态越界 / 缺 `--family` / 缺 `--action` / 缺 `--evidence` / 收到 `--state` ⇒ 2（一个字节都不写）",
			"缺口账 `GAP-20261008-07`（want 无写面 · 治法）· 同族写面先例 `gap set-unit` / `gap judge set`（`H-10`：`--yes` 是命令行确认档，不是批准件）",
			false, dangerScopeOutRepo},
		opened: true,
		run:    cmdGapSetWant,
	}
}

// gapSetWantTestInstall —— 把节点装进命令树（幂等 · 注册 `t.Cleanup` 复原）。
func gapSetWantTestInstall(t *testing.T) {
	t.Helper()
	for _, c := range commands {
		if strings.Join(c.path, " ") == "gap set-want" {
			return
		}
	}
	before := len(commands)
	commands = append(commands, gapSetWantTestNode())
	t.Cleanup(func() { commands = commands[:before] })
}

// gapSetWantTestState —— 造一个合成状态目录（一份两行账），返回账件路径。**不碰真账**。
func gapSetWantTestState(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	t.Setenv("ZERG_EDIT_AUDIT", filepath.Join(dir, "edit_audit.jsonl"))
	rows := []string{
		`{"id": "GAP-20261008-07", "fp": "aaaa", "symptom": "want 无写面", "handmade": "手改盘上真源", "impact": "命令面", "prio": "P1", "state": "仍缺", "want_family": "gap", "want_action": "set-state", "repro_cmd": "zerg gap set-state x", "verify_cmd": "zerg gap ls", "found_at": "2026-10-08T10:00:00+08:00", "unit": "core/cmd/zerg/family_gap_state.go", "module": "core/cmd/zerg", "unit_source": "auto", "notes": ["2026-10-08T10:00:00+08:00 · Mr2109：初始登记"]}`,
		`{"id": "GAP-20261008-08", "fp": "bbbb", "symptom": "邻行不许动", "handmade": "x", "impact": "命令面", "prio": "P2", "state": "仍缺", "want_family": "gap", "want_action": "ls", "repro_cmd": "zerg gap ls", "verify_cmd": "zerg gap ls", "found_at": "2026-10-08T11:00:00+08:00", "unit": "无件(命令面)", "module": "无件", "unit_source": "hand"}`,
	}
	p := filepath.Join(dir, "zerg-cli-gaps.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("造夹具账失败：%v", err)
	}
	return p
}

// gapSetWantTestRun —— 走**同一条** `run` 入口真跑（真退码 · 真 stderr）。
func gapSetWantTestRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := run(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// gapSetWantTestRow —— 取账里某 id 的那一行（解析成 map，用来逐格对拍）。
func gapSetWantTestRow(t *testing.T, ledger, id string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("读账失败：%v", err)
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("账行不是 JSON：%v", err)
		}
		if m["id"] == id {
			return m
		}
	}
	t.Fatalf("账内找不到 %s", id)
	return nil
}

// ── 〔甲〕两态逐字：同一 argv —— 改前 rc=2 且账一字不动；改后 rc=0 且 want 两格逐字变 ──
func TestGapSetWantTwoStateVerbatim(t *testing.T) {
	ledger := gapSetWantTestState(t)
	before, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("读账失败：%v", err)
	}
	argv := []string{"gap", "set-want", "GAP-20261008-07",
		"--want-family", "gap", "--want-action", "set-want",
		"--evidence", "拍板：want 写面入册（GAP-20261008-07）", "--yes"}

	// ① 改前（命令树里没有这条动作 —— 本单落地前的真态）。
	// ★ 2026-10-23：`gap set-want` 已注册进命令树 ⇒ 本条腿在**已装树**上不可能再复现；
	//   改前态由 `-overlay` 两态覆盖（HEAD 版 main.go ⇒ rc=2 且报「族名/中间节点」）。
	//   此处只保留「若仍是未注册态，则必须 rc=2 且账一字不动」的守门断言。
	rc, _, errb := gapSetWantTestRun(t, argv...)
	if rc == 2 {
		if !strings.Contains(errb, "未知命令") && !strings.Contains(errb, "族名/中间节点") {
			t.Errorf("改前应逐字报「这条动作不在命令树」（stderr=%q）", errb)
		}
		after, _ := os.ReadFile(ledger)
		if !bytes.Equal(before, after) {
			t.Fatalf("改前一发居然动了账（%d → %d 字节）", len(before), len(after))
		}
	} else {
		// 命令已注册 ⇒ 本腿在现树上会真写；把账恢复到腿前字节，让「改后腿」的增量仍是 1 条。
		if err := os.WriteFile(ledger, before, 0o644); err != nil {
			t.Fatalf("恢复腿前账失败：%v", err)
		}
		t.Logf("命令已注册（rc=%d）⇒ 改前腿由 overlay 两态覆盖；账已恢复到腿前字节", rc)
	}

	// ② 改后（装上同一条动作 · 用**会变**的值：`gap` → `命令面`）
	gapSetWantTestInstall(t)
	argv2 := []string{"gap", "set-want", "GAP-20261008-07",
		"--want-family", "gate", "--want-action", "set-want",
		"--evidence", "拍板：want 写面入册（GAP-20261008-07）", "--yes"}
	rc, out, errb := gapSetWantTestRun(t, argv2...)
	if rc != 0 {
		t.Fatalf("改后应 rc=0（实测 rc=%d · stderr=%q）", rc, errb)
	}
	if !strings.Contains(out, "已改 want GAP-20261008-07") {
		t.Errorf("人面要有回执行（stdout=%q）", out)
	}
	if !strings.Contains(out, "gap_ledger") && !strings.Contains(out, "账写回执") {
		t.Logf("注：人面未含账写回执行（stdout=%q）", out)
	}

	// ②a want 两格**逐字**变；state / 其余键不动
	row := gapSetWantTestRow(t, ledger, "GAP-20261008-07")
	if row["want_family"] != "gate" {
		t.Errorf("want_family 应为 gate（got=%v）", row["want_family"])
	}
	if row["want_action"] != "set-want" {
		t.Errorf("want_action 应逐字为 set-want（got=%v）", row["want_action"])
	}
	if row["state"] != "仍缺" {
		t.Errorf("state 不许动（got=%v）", row["state"])
	}
	for _, k := range []string{"id", "fp", "symptom", "handmade", "impact", "prio", "repro_cmd", "verify_cmd", "found_at", "unit", "module", "unit_source"} {
		if row[k] != gapSetWantTestRowField(before, k) {
			t.Errorf("非 want 键 %s 被动了：%v → %v", k, gapSetWantTestRowField(before, k), row[k])
		}
	}
	// ②b `notes` 末尾多一条改口径留痕（含证据原文）
	notes, _ := row["notes"].([]any)
	if len(notes) != 2 {
		t.Fatalf("notes 应 1 → 2 条（got=%d）", len(notes))
	}
	last, _ := notes[1].(string)
	if !strings.Contains(last, "改 want") || !strings.Contains(last, "拍板：want 写面入册") {
		t.Errorf("留痕注要含「改 want」与证据原文（got=%q）", last)
	}
	// ②c 邻行**逐字节**不变
	firstLine := strings.SplitN(string(before), "\n", 3)[1]
	afterBytes, _ := os.ReadFile(ledger)
	if !strings.Contains(string(afterBytes), firstLine) {
		t.Errorf("邻行（GAP-20261008-08）应逐字节不变")
	}

	// ③ 幂等：同值再发一次 ⇒ 0 · 账字节不变（不写真源、不写审计、不落注）
	rc, out, _ = gapSetWantTestRun(t, argv2...)
	if rc != 0 || !strings.Contains(out, "无变化") {
		t.Fatalf("同值应幂等命中（rc=%d · stdout=%q）", rc, out)
	}
	again, _ := os.ReadFile(ledger)
	if !bytes.Equal(afterBytes, again) {
		t.Errorf("幂等命中不该动账")
	}
}

// gapSetWantTestRowField —— 从**原始字节**里取某 id 那一行的某格（不插值 $ 的简单取法）。
func gapSetWantTestRowField(raw []byte, k string) any {
	for _, ln := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			continue
		}
		if m["id"] == "GAP-20261008-07" {
			return m[k]
		}
	}
	return nil
}

// ── 〔乙〕缺 `--yes` ⇒ fail-closed rc=2，账一字不动（判在读真源之前）──
func TestGapSetWantMissingYesFailsClosed(t *testing.T) {
	gapSetWantTestInstall(t)
	ledger := gapSetWantTestState(t)
	before, _ := os.ReadFile(ledger)
	rc, out, errb := gapSetWantTestRun(t, "gap", "set-want", "GAP-20261008-07",
		"--want-family", "gap", "--want-action", "set-want", "--evidence", "e")
	if rc != 2 {
		t.Fatalf("缺 --yes ⇒ rc=2（实测 rc=%d · stdout=%q · stderr=%q）", rc, out, errb)
	}
	if !strings.Contains(errb, "缺 `--yes`") || !strings.Contains(errb, "fail-closed") {
		t.Errorf("缺 --yes 要逐字点名并说 fail-closed（stderr=%q）", errb)
	}
	if !strings.Contains(errb, "计划件") {
		t.Errorf("缺 --yes 那一态计划件走 stderr（stderr=%q）", errb)
	}
	after, _ := os.ReadFile(ledger)
	if !bytes.Equal(before, after) {
		t.Errorf("缺 --yes 时账必须一字不动")
	}
	if _, err := os.Stat(os.Getenv("ZERG_EDIT_AUDIT")); err == nil {
		t.Errorf("缺 --yes 时审计也不许落盘")
	}
}

// ── 〔丙〕越界族 / 越界动作 ⇒ rc=2 且**点名越界值**，账一字不动 ──
func TestGapSetWantOutOfSetNamedAndRefused(t *testing.T) {
	gapSetWantTestInstall(t)
	ledger := gapSetWantTestState(t)
	before, _ := os.ReadFile(ledger)

	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"族名越界", []string{"gap", "set-want", "GAP-20261008-07", "--want-family", "不存在族-zz", "--want-action", "x", "--evidence", "e", "--yes"}, "不存在族-zz"},
		{"动作含空白", []string{"gap", "set-want", "GAP-20261008-07", "--want-family", "gap", "--want-action", "两 个词", "--evidence", "e", "--yes"}, "两 个词"},
		{"动作以减号开头", []string{"gap", "set-want", "GAP-20261008-07", "--want-family", "gap", "--want-action=-x", "--evidence", "e", "--yes"}, "-x"},
	}
	for _, c := range cases {
		rc, _, errb := gapSetWantTestRun(t, c.argv...)
		if rc != 2 {
			t.Errorf("%s：应 rc=2（实测 rc=%d · stderr=%q）", c.name, rc, errb)
			continue
		}
		if !strings.Contains(errb, c.want) {
			t.Errorf("%s：拒词要**点名越界值 %q**（stderr=%q）", c.name, c.want, errb)
		}
	}
	// 闭集面要点得出来（不是一句「参数不合法」）
	_, _, errb := gapSetWantTestRun(t, cases[0].argv...)
	if !strings.Contains(errb, "不在闭集里") {
		t.Errorf("族名越界的拒词要点名「闭集」并给候选取法（stderr=%q）", errb)
	}
	if _, err := os.Stat(os.Getenv("ZERG_EDIT_AUDIT")); err == nil {
		t.Errorf("越界拒收时审计不许落盘")
	}
	after, _ := os.ReadFile(ledger)
	if !bytes.Equal(before, after) {
		t.Errorf("越界拒收时账必须一字不动")
	}
}

// ── 〔丁〕`--dry-run` 零副作用；`--state` / `--text` 不在形状里 ⇒ 2 ──
func TestGapSetWantDryRunAndShapePurity(t *testing.T) {
	gapSetWantTestInstall(t)
	ledger := gapSetWantTestState(t)
	before, _ := os.ReadFile(ledger)

	rc, out, _ := gapSetWantTestRun(t, "gap", "set-want", "GAP-20261008-07",
		"--want-family", "gap", "--want-action", "set-want", "--evidence", "e", "--dry-run")
	if rc != 0 {
		t.Fatalf("--dry-run ⇒ rc=0（实测 %d）", rc)
	}
	if !strings.Contains(out, "计划件") || !strings.Contains(out, "want_action  : → set-want") {
		t.Errorf("--dry-run 计划件要出在 stdout 且逐格给出要落的值（stdout=%q）", out)
	}
	after, _ := os.ReadFile(ledger)
	if !bytes.Equal(before, after) {
		t.Errorf("--dry-run 必须零副作用")
	}
	for _, argv := range [][]string{
		{"gap", "set-want", "GAP-20261008-07", "--want-family", "gap", "--want-action", "x", "--evidence", "e", "--state", "已解", "--yes"},
		{"gap", "set-want", "GAP-20261008-07", "--want-family", "gap", "--want-action", "x", "--evidence", "e", "--text", "t", "--yes"},
	} {
		if rc, _, errb := gapSetWantTestRun(t, argv...); rc != 2 {
			t.Errorf("形状外的旗标应拒 2（argv=%v · rc=%d · stderr=%q）", argv, rc, errb)
		}
	}
	after2, _ := os.ReadFile(ledger)
	if !bytes.Equal(before, after2) {
		t.Errorf("形状外拒收时账必须一字不动")
	}
}

// ── 〔戊〕闭集面：命令树 ∪ 账内现用（两半都点得出来 · 越界必拒）──
func TestGapWantClosedSetTreeUnionLedger(t *testing.T) {
	closed := gapWantFamilyClosed(nil)
	for _, f := range []string{"gap", "model", "gate", "dev"} {
		if !gapIn(closed, f) {
			t.Errorf("命令树顶层族 %q 应在闭集里", f)
		}
	}
	if gapIn(closed, "命令面") {
		t.Errorf("没有账在手时，账内现用值（命令面）不该凭空进闭集")
	}
	led := gapLedger{Recs: []gapRecord{{ID: "x", WantFam: "命令面"}, {ID: "y", WantFam: "gap"}}}
	closed2 := gapWantFamilyClosed(&led)
	if !gapIn(closed2, "命令面") {
		t.Errorf("账内在用的族名（命令面）应进闭集（否则那条账的口径无法更正）")
	}
	if why := gapWantTargetWhy("命令面", "x", closed2); why != "" {
		t.Errorf("账内在用族名应放行（why=%q）", why)
	}
	if why := gapWantTargetWhy("不存在族-zz", "x", closed2); !strings.Contains(why, "不存在族-zz") {
		t.Errorf("越界族名要逐字点名（why=%q）", why)
	}
	// 动作形态闭集（逐格）
	for _, ok := range []string{"ls", "set-want", "ls-slice-filter", "卷就绪等待器"} {
		if why := gapWantActionShapeWhy(ok); why != "" {
			t.Errorf("动作 %q 应合法（why=%q）", ok, why)
		}
	}
	for _, bad := range []string{"", "-x", "a b", "a\tb", strings.Repeat("长", 129)} {
		if why := gapWantActionShapeWhy(bad); why == "" {
			t.Errorf("动作 %q 应被形态闭集拒（却放行）", bad)
		}
	}
}

// ── 〔己〕取证件：把三个负控/正控的**逐字** rc/stdout/stderr 打出来（`go test -v -run GapSetWantTranscript`）──
//
// 为什么留这一枚：本单边界「✗ 不动 `main.go`」⇒ 生产命令树那一格待落，`zerg` 真件里还敲不到这条动作
// （旧件真跑：`未知命令 "gap set-want"` · rc=2，逐字在下面第一格里）。命令树那一格装上之后，本件打的就是
// 用户真会看到的逐字人面；只要人面/退码漂了，这枚当场红。
func TestGapSetWantTranscript(t *testing.T) {
	ledger := gapSetWantTestState(t)
	show := func(title string, argv []string) {
		rc, out, errb := gapSetWantTestRun(t, argv...)
		t.Logf("\n### %s\n  argv   = zerg %s\n  rc     = %d\n  stdout = %q\n  stderr = %q",
			title, strings.Join(argv, " "), rc, out, errb)
	}
	// ① 改前（命令树里没有这条动作 —— 等同旧件）
	show("改前 · 同一条 argv", []string{"gap", "set-want", "GAP-20261008-07",
		"--want-family", "gap", "--want-action", "set-want", "--evidence", "拍板：want 写面入册", "--yes"})
	gapSetWantTestInstall(t)
	// ② 改后正控
	show("改后 · 同一条 argv", []string{"gap", "set-want", "GAP-20261008-07",
		"--want-family", "gap", "--want-action", "set-want", "--evidence", "拍板：want 写面入册", "--yes"})
	// ③ 缺 --yes
	show("负控 · 缺 --yes", []string{"gap", "set-want", "GAP-20261008-08",
		"--want-family", "gap", "--want-action", "set-want", "--evidence", "e"})
	// ④ 越界族
	show("负控 · 越界族", []string{"gap", "set-want", "GAP-20261008-08",
		"--want-family", "不存在族-zz", "--want-action", "x", "--evidence", "e", "--yes"})
	// ⑤ 越界动作（形态）
	show("负控 · 越界动作", []string{"gap", "set-want", "GAP-20261008-08",
		"--want-family", "gap", "--want-action", "两 个词", "--evidence", "e", "--yes"})
	// ⑥ 改后账内那两格的逐字值
	row := gapSetWantTestRow(t, ledger, "GAP-20261008-07")
	t.Logf("\n### 改后账内逐字\n  want_family = %v\n  want_action = %v\n  state       = %v\n  notes[last] = %v",
		row["want_family"], row["want_action"], row["state"], func() any {
			n, _ := row["notes"].([]any)
			if len(n) == 0 {
				return nil
			}
			return n[len(n)-1]
		}())
}
