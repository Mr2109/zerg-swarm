// family_gap_writeface_action_test.go —— 缺口 `GAP-20261011-02`（体系级）的**回归牙**（本单新增件）。
//
// 面：写面判定（`gapWriteFaceWhy`）此前**按族判**（只看 `cmd.danger != nil`）—— 而危险档登记在
// 「**整条命令**」上的**多动作节点**（如 `dev proposal`：`new` 写 · `list`/`show`/`check` 只读）里，
// 同族的**只读动作**也被一并判成写面 ⇒ 只读判据立不了（现读实测：`zerg gap add --verify-cmd
// 'zerg dev proposal list --json id'` ⇒ 退 2，而拒词自己写着「`new` 写、`list`/`show`/`check` 只读」
// —— 判词自相矛盾即「按族判、不看动作」的病）。
//
// 治（本件钉住）：判定**下沉到动作级** —— 只读动作名**只**取自现读的危险档数据（`cmd.danger.Effect`），
// 本族**不另立动作名单**（✗ 别硬编码动作名表 · ✗ 别自造第二份真源）。★ 只认动作位（`rest[0]`），
// 写动作的旗标值里混着只读动作名不许误放；未声明只读的动作照旧拦（fail-closed 不松）。
//
// 负控（哑化新档 = 删掉 `gapWriteFaceWhy` 里那段动作级放行 ⇒ 本件〔甲〕/〔丙〕应红）在件外以
// `go test -overlay` 跑（同 `family_gap_judgefields_test.go` 的负控跑法 —— 不改仓内件）。
package main

import (
	"bytes"
	"testing"
)

// ── 〔甲〕只读动作放行：`dev proposal list`/`show`/`check` ⇒ 不是写面（`gapWriteFaceWhy` 空串）。 ──
func TestGapWriteFaceActionLevelReadOnlyAllowed(t *testing.T) {
	for _, c := range []string{
		"zerg dev proposal list --json id",
		"zerg dev proposal show DEV-0001",
		"zerg dev proposal check --deep",
	} {
		if why := gapWriteFaceWhy(c); why != "" {
			t.Errorf("只读动作应放行（写面判定当空串）\n  cmd = %q\n  why = %q", c, why)
		}
	}
}

// ── 〔乙〕写动作仍必拦：同族的写动作（`new`/`revise`/`--retract`/`--void`）不是只读 ⇒ 仍判写面（非空）。 ──
func TestGapWriteFaceActionLevelWriteStillBlocked(t *testing.T) {
	for _, c := range []string{
		"zerg dev proposal new --title t",
		"zerg dev proposal revise DEV-0001 --criterion 'zerg gap ls'",
		"zerg dev proposal --retract DEV-0001 --reason r",
		"zerg dev proposal --void DEV-0001 --reason r",
		// ★ 动作位只认 `rest[0]`：写动作的旗标值里混着只读动作名 ⇒ 不许误放（这是关键安全格）。
		"zerg dev proposal new --title list",
	} {
		if why := gapWriteFaceWhy(c); why == "" {
			t.Errorf("写动作必须仍判写面（why 非空）\n  cmd = %q", c)
		}
	}
}

// ── 〔丙〕端到端（进程内同一 `run` 入口）：`gap add` 拿同族只读动作当判据 ⇒ rc=0 收下；写动作 ⇒ rc=2 拒。 ──
func TestGapAddVerifyCmdActionLevel(t *testing.T) {
	add := func(v string) int {
		dir := t.TempDir()
		t.Setenv("ZERG_STATE_DIR", dir)
		t.Setenv("ZERG_EDIT_AUDIT", dir+"/edit_audit.jsonl")
		argv := []string{"gap", "add", "--symptom", "x", "--handmade", "y", "--impact", "命令面",
			"--want-family", "dev", "--want-action", "z", "--repro-cmd", "zerg",
			"--verify-cmd", v, "--dry-run"}
		var out, errb bytes.Buffer
		return run(argv, &out, &errb)
	}
	// 两态：改前 rc=2（「落在写面」）· 改后 rc=0（收下）。
	if rc := add("zerg dev proposal list --json id"); rc != 0 {
		t.Errorf("同族只读动作当判据应收下：rc=%d（要 0）", rc)
	}
	// 负控逐条：同族**写**动作仍必拒。
	for _, v := range []string{"zerg dev proposal new --title t", "zerg dev proposal revise DEV-0001 --criterion 'zerg gap ls'"} {
		if rc := add(v); rc != 2 {
			t.Errorf("同族写动作当判据应拒：rc=%d（要 2）· cmd=%q", rc, v)
		}
	}
	// 负控：**其它族**的只读动作未被波及（本就是一等公民）。
	if rc := add("zerg model ls --json name"); rc != 0 {
		t.Errorf("其它族只读动作应照旧收下：rc=%d（要 0）", rc)
	}
}

// ── 〔丁〕取词器：只读动作名**只**从危险档数据（`Effect`）里取 —— 逐格钉 `dev proposal` 那一句。 ──
func TestGapDangerReadOnlyActionsFromDangerData(t *testing.T) {
	got := gapDangerReadOnlyActions("落一件提案件（状态目录）—— `new` 写、`list`/`show`/`check` 只读")
	for _, a := range []string{"list", "show", "check"} {
		if !got[a] {
			t.Errorf("只读动作 %q 应从危险档 Effect 里取到（got=%v）", a, got)
		}
	}
	for _, w := range []string{"new", "revise"} {
		if got[w] {
			t.Errorf("写动作 %q 不许进只读闭集（got=%v）", w, got)
		}
	}
	// 无关 Effect（无「只读」子句）⇒ 空集（调用方照旧按写面拦）。
	if got := gapDangerReadOnlyActions("改那台子端盘上的注册表件（`add` 先校验）"); len(got) != 0 {
		t.Errorf("无「只读」子句 ⇒ 空集，得到 %v", got)
	}
}
