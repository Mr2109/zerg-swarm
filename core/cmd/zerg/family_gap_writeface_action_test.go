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
	"strings"
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
// gapAddDryRunRC —— 进程内跑一条 `gap add … --verify-cmd <v> --dry-run`（合成状态目录 · 零副作用 ·
// 走与命令面同一条 `run` 入口 ⇒ 判据闸逐字生效）。
func gapAddDryRunRC(t *testing.T, v string) int {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	t.Setenv("ZERG_EDIT_AUDIT", dir+"/edit_audit.jsonl")
	argv := []string{"gap", "add", "--symptom", "x", "--handmade", "y", "--impact", "命令面",
		"--want-family", "dev", "--want-action", "z", "--repro-cmd", "zerg",
		"--verify-cmd", v, "--dry-run"}
	var out, errb bytes.Buffer
	return run(argv, &out, &errb)
}

func TestGapAddVerifyCmdActionLevel(t *testing.T) {
	add := func(v string) int { return gapAddDryRunRC(t, v) }
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
	if rc := add("zerg model ls --json id"); rc != 0 {
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

// ── 〔戊〕缺口 `GAP-20261005-34`（本枚）：**原样透传型**判据必被拒跑；**成对负控** = 纯只读判据仍放行。 ──
//
// 面：同族两道只读闸（`gapWriteFaceWhy` / `gapRegressRefuse`）此前只认危险档，对**原样透传型**
// （`cmd.passthrough` —— 参数逐字交给被包脚本）无分支 ⇒ 一条 `zerg gate run --step … --only-step`
// 型判据被判「可跑」收下，真跑一次就把整条 `precommit-gates.sh` 拉起来（含 shard go-test）。
// 治：共口 `gapPassthroughWhy`（口径与 `gapRereadWhy` 同一条）；**✗ 不放宽**：写面档与命令树档
// 判词一字不动、`gate check <只读判据名>` 那条既有共口放行照旧（本件也一并钉住）。
//
// 负控（哑化新支 = 删掉 `gapPassthroughWhy` 的两处调用 ⇒ 本件必 FAIL）在件外以 `go test -overlay` 跑。
func TestGapPassthroughJudgeRefused(t *testing.T) {
	// 正控（逐条）：透传型 ⇒ 写面闸**逐条点名「原样透传型」**；regress 闸**一概拒跑**（非空 why）。
	for _, c := range []string{
		"zerg gate run --step 'gofmt -l core' --only-step",
		"zerg gate run --fast",
		"zerg gate ls --json scope",
		"zerg sandbox test",
		"zerg cocoon pack",
	} {
		why := gapWriteFaceWhy(c)
		if !strings.Contains(why, "原样透传型") {
			t.Errorf("透传型判据必须被写面闸拒跑并点名「原样透传型」\n  cmd = %q\n  why = %q", c, why)
		}
		// regress 闸：本组都是「真判据」分级（非通用样板）⇒ 拒词就是「原样透传型」这一档。
		if w := gapRegressRefuse(gapRecord{VerifyCmd: c}); !strings.Contains(w, "原样透传型") {
			t.Errorf("透传型判据必须被 regress 闸拒跑并点名「原样透传型」\n  cmd = %q\n  why = %q", c, w)
		}
	}
	// ★ 通用样板那一条（`zerg gate ls` · 分级=占位）仍**拒跑**（拒词哪一档都对，✗ 只要别放行）。
	for _, c := range []string{"zerg gate ls", "zerg gate ls --json id"} {
		if why := gapWriteFaceWhy(c); !strings.Contains(why, "原样透传型") {
			t.Errorf("透传型判据（写面闸那一路）必拒并点名：cmd=%q why=%q", c, why)
		}
		if w := gapRegressRefuse(gapRecord{VerifyCmd: c}); w == "" {
			t.Errorf("透传型判据经 regress 闸不许放行：cmd=%q", c)
		}
	}
	// 负控（成对）：**纯只读判据仍放行** —— 两道闸都空串（✗ 别把只读面一起拦掉）。
	for _, c := range []string{
		"zerg gap ls --json id",
		"zerg code show core/cmd/zerg/family_gap.go:544",
		"zerg dev proposal list --json id",
		"zerg gate check check-doc-name",
	} {
		if why := gapWriteFaceWhy(c); why != "" {
			t.Errorf("纯只读判据仍须放行（写面闸当空串）\n  cmd = %q\n  why = %q", c, why)
		}
		if w := gapRegressRefuse(gapRecord{VerifyCmd: c}); w != "" {
			t.Errorf("纯只读判据仍须放行（regress 闸当空串）\n  cmd = %q\n  why = %q", c, w)
		}
	}
	// 端到端（进程内同一 `run` 入口）：透传型 ⇒ rc=2 拒收；纯只读 ⇒ rc=0 收下（成对负控）。
	if rc := gapAddDryRunRC(t, "zerg gate run --step 'gofmt -l core' --only-step"); rc != 2 {
		t.Errorf("透传型判据经 `gap add` 应拒收：rc=%d（要 2 · 改前为 0）", rc)
	}
	if rc := gapAddDryRunRC(t, "zerg code show core/cmd/zerg/family_gap.go:544"); rc != 0 {
		t.Errorf("纯只读判据经 `gap add` 应照旧收下：rc=%d（要 0）", rc)
	}
	// ★ 窄面负控：`gate check <写档判据名>` 的拒词仍走**写面**那一支（透传支不许顶替它）。
	if why := gapWriteFaceWhy("zerg gate check build-all"); !strings.Contains(why, "写面") {
		t.Errorf("写档判据名的拒词仍须点名「写面」：why = %q", why)
	}
}
