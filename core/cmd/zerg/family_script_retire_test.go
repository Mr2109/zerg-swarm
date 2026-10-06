// family_script_retire_test.go —— `zerg script retire` 退场正门的**机检**（设计稿 §四 两面判 · §五 负控三条）。
//
// 落点：`package main_test` + `zerg.RunForTest`（与 `cli_gap_test.go` / `cli_matrix_test.go` 同一条路 ——
// 跑的是**当前源码**的行为，不是盘上旧制品）。全部**合成盘面**（`t.TempDir()` 当仓根与状态目录）：
// **不碰真 `core/internal/contract/adopt-retire.json`**、不碰真 `~/.zerg/state`、不删任何件。
//
// 本件钉住的格（逐格对设计稿）：
//
//	① 四态：`--dry-run`=0（零副作用）· 缺 `--yes`=2（拒在 D2 确认闸那一层）· `--yes`=0（真写 +1 条目）
//	② 负控三条：缺 `--yes` ⇒ 2 且登记面 sha256 逐字不变（对照正控 `--yes` ⇒ +1）·
//	   例外类件 ⇒ 1（同命令对普通候选件 ⇒ 0 · 成对，证明判的是例外面而非恒红）·
//	   目标不在盘 / 不在闭集 ⇒ 不给结论（fail-closed · ✗ 不静默绿）
//	③ 冲突：已有同 `object` 条目 ⇒ 14
//	④ ✗ 不代管物理删除：真跑 `--yes` 后**目标件仍在盘**（一个件都没被删）
//	⑤ 机器面：`--json` 六键包封 + 字段表（result/object/state/…）
package main_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// srFixtureJSON —— 合成退场真源（两枚 ledger + 一条例外类名册）。
const srFixtureJSON = `{
  "schema": "zerg-adopt-retire/1",
  "retire_ledger": [
    {
      "object": "scripts/old1.py",
      "state": "已删（2026-01-01）",
      "evidence": "夹具"
    },
    {
      "object": "scripts/keep2.py",
      "state": "已删（2026-01-02）",
      "evidence": "夹具"
    }
  ],
  "retire_exceptions": [
    {
      "object": "scripts/ext-tool.sh",
      "category": "外部依赖（退场例外）",
      "reason": "能力依赖外部工具（联网 / 本机物件）",
      "keep_cost": "保留代价：每版随外部工具协议同步核对",
      "review_date": "2026-10-07"
    }
  ]
}
`

// srEnv —— 合成仓根 + 状态目录；返回真源路径。
func srEnv(t *testing.T) (root, truth, state string) {
	t.Helper()
	root = t.TempDir()
	state = t.TempDir()
	for _, d := range []string{
		filepath.Join(root, "core", "internal", "contract"),
		filepath.Join(root, "scripts"),
		filepath.Join(root, "vendor"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("建合成目录失败：%v", err)
		}
	}
	truth = filepath.Join(root, "core", "internal", "contract", "adopt-retire.json")
	if err := os.WriteFile(truth, []byte(srFixtureJSON), 0o644); err != nil {
		t.Fatalf("写合成真源失败：%v", err)
	}
	// 目标件（普通候选 · 在盘）
	srWrite(t, filepath.Join(root, "scripts", "keep.py"), "#!/usr/bin/env python3\nprint('x')\n")
	// 冲突靶（在盘 + 已在 ledger）
	srWrite(t, filepath.Join(root, "scripts", "keep2.py"), "#!/usr/bin/env python3\nprint('y')\n")
	// 例外类靶（在盘 + 在例外名册）
	srWrite(t, filepath.Join(root, "scripts", "ext-tool.sh"), "#!/bin/sh\necho hi\n")
	// 闭集外靶
	srWrite(t, filepath.Join(root, "vendor", "x.py"), "print('v')\n")

	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_STATE_DIR", state)
	t.Setenv("ZERG_EDIT_AUDIT", filepath.Join(state, "edit_audit.jsonl"))
	return root, truth, state
}

func srWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatalf("写合成件失败（%s）：%v", p, err)
	}
}

func srRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

func srSHA(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读不了 %s：%v", p, err)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func srLedger(t *testing.T, truth string) []map[string]string {
	t.Helper()
	b, err := os.ReadFile(truth)
	if err != nil {
		t.Fatalf("读不了真源：%v", err)
	}
	var doc struct {
		Ledger []map[string]string `json:"retire_ledger"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("真源解不动（写坏了？）：%v", err)
	}
	return doc.Ledger
}

// ── ① 四态 ────────────────────────────────────────────────────────────────────

func TestScriptRetireFourStates(t *testing.T) {
	_, truth, state := srEnv(t)
	before := srSHA(t, truth)

	// 态一：--dry-run ⇒ 0 · 零副作用（真源 sha 逐字不变 · 审计件不出现）
	rc, out, _ := srRun(t, "script", "retire", "scripts/keep.py", "--dry-run")
	if rc != 0 {
		t.Fatalf("--dry-run 期望 rc=0，实得 %d（out=%q）", rc, out)
	}
	if got := srSHA(t, truth); got != before {
		t.Fatalf("--dry-run 有副作用：真源 sha 变了（%s → %s）", before, got)
	}
	if _, err := os.Stat(filepath.Join(state, "edit_audit.jsonl")); err == nil {
		t.Fatalf("--dry-run 落了审计件（应零副作用）")
	}

	// 态二：缺 --yes 且缺 --dry-run ⇒ 2（拒在 D2 确认闸那一层）
	rc, _, errb := srRun(t, "script", "retire", "scripts/keep.py", "--evidence", "对拍三面同")
	if rc != 2 {
		t.Fatalf("缺 --yes 期望 rc=2，实得 %d（err=%q）", rc, errb)
	}
	if !strings.Contains(errb, "--yes") {
		t.Fatalf("缺 --yes 的拒绝理由没点到 --yes：%q", errb)
	}
	if got := srSHA(t, truth); got != before {
		t.Fatalf("缺 --yes 那一档有写面：真源 sha 变了")
	}

	// 态三：--yes 真写（带 --evidence）⇒ 0 且条目 +1
	rc, out, errb = srRun(t, "script", "retire", "scripts/keep.py", "--yes", "--evidence", "对拍三面同 · Go 后继件 sha256=deadbeef")
	if rc != 0 {
		t.Fatalf("--yes 真写期望 rc=0，实得 %d（out=%q err=%q）", rc, out, errb)
	}
	after := srSHA(t, truth)
	if after == before {
		t.Fatalf("--yes 真写后真源 sha 未变（没写？）")
	}
	led := srLedger(t, truth)
	if len(led) != 3 {
		t.Fatalf("真写后条目应 3 条，实得 %d", len(led))
	}
	last := led[2]
	if last["object"] != "scripts/keep.py" {
		t.Fatalf("追加条目的 object 不对：%q", last["object"])
	}
	if !strings.Contains(last["state"], "已登记退场") {
		t.Fatalf("追加条目的 state 应为「已登记退场」，实得 %q", last["state"])
	}

	// ④ ✗ 不代管物理删除：目标件仍在盘（一个件都没被删）
	if _, err := os.Stat(filepath.Join(os.Getenv("ZERG_REPO"), "scripts", "keep.py")); err != nil {
		t.Fatalf("--yes 之后目标件不在盘了（本正门 ✗ 不代管 git rm）：%v", err)
	}

	// 态四：目标不在盘 ⇒ 2（不给结论）；不在闭集 ⇒ 1（拒执）
	rc, _, _ = srRun(t, "script", "retire", "scripts/notarget.py", "--dry-run")
	if rc != 2 {
		t.Fatalf("目标不在盘期望 rc=2，实得 %d", rc)
	}
	rc, _, errb = srRun(t, "script", "retire", "vendor/x.py", "--dry-run")
	if rc != 1 {
		t.Fatalf("目标不在闭集（vendor/）期望 rc=1，实得 %d（err=%q）", rc, errb)
	}
}

// ── ② 负控三条（成对：证明判的是那一面而非恒绿/恒红）─────────────────────────

func TestScriptRetireNegativeControls(t *testing.T) {
	_, truth, _ := srEnv(t)

	// 负控 1：缺 --yes ⇒ 2 且零写面（已由上面钉住 · 这里再单钉一次契约）
	before := srSHA(t, truth)
	if rc, _, _ := srRun(t, "script", "retire", "scripts/keep.py"); rc != 2 {
		t.Fatalf("负控1 期望缺 --yes ⇒ 2")
	}
	if srSHA(t, truth) != before {
		t.Fatalf("负控1 破了：缺 --yes 也有写面")
	}

	// 负控 2（成对）：例外类件 ⇒ 1；同命令对普通候选件 ⇒ 0（干跑）
	rcExc, _, errExc := srRun(t, "script", "retire", "scripts/ext-tool.sh", "--dry-run")
	if rcExc != 1 {
		t.Fatalf("负控2 例外类件期望 rc=1，实得 %d（err=%q）", rcExc, errExc)
	}
	if !strings.Contains(errExc, "例外类") {
		t.Fatalf("负控2 没点名「例外类」：%q", errExc)
	}
	rcNorm, _, _ := srRun(t, "script", "retire", "scripts/keep.py", "--dry-run")
	if rcNorm != 0 {
		t.Fatalf("负控2 成对那一半：普通候选件期望 rc=0，实得 %d（= 判的是例外面还是恒红？）", rcNorm)
	}

	// 负控 3：目标不在盘 / 不在闭集 ⇒ 不给结论（fail-closed · ✗ 不静默绿）
	if rc, _, _ := srRun(t, "script", "retire", "scripts/ghost.py", "--dry-run"); rc == 0 {
		t.Fatalf("负控3 破了：目标不在盘却退 0（静默绿）")
	}
	if rc, _, _ := srRun(t, "script", "retire", "vendor/x.py", "--dry-run"); rc == 0 {
		t.Fatalf("负控3 破了：目标不在闭集却退 0")
	}
}

// ── ③ 冲突（14）· 用法错（2）· 机器面 ─────────────────────────────────────────

func TestScriptRetireConflictAndUsage(t *testing.T) {
	_, truth, _ := srEnv(t)

	// 已有同 object 条目 ⇒ 14
	if rc, _, errb := srRun(t, "script", "retire", "scripts/keep2.py", "--dry-run"); rc != 14 {
		t.Fatalf("冲突期望 rc=14，实得 %d（err=%q）", rc, errb)
	}
	// --dry-run 与 --yes 同给 ⇒ 2
	if rc, _, _ := srRun(t, "script", "retire", "scripts/keep.py", "--dry-run", "--yes"); rc != 2 {
		t.Fatalf("两档同给期望 rc=2")
	}
	// --yes 缺 --evidence ⇒ 2（防无凭退场）
	before := srSHA(t, truth)
	if rc, _, _ := srRun(t, "script", "retire", "scripts/keep.py", "--yes"); rc != 2 {
		t.Fatalf("--yes 缺 --evidence 期望 rc=2")
	}
	if srSHA(t, truth) != before {
		t.Fatalf("--yes 缺 --evidence 也有写面")
	}
	// 多余位置参数 ⇒ 2
	if rc, _, _ := srRun(t, "script", "retire", "scripts/keep.py", "scripts/keep2.py", "--dry-run"); rc != 2 {
		t.Fatalf("多余位置参数期望 rc=2")
	}
	// --json 机器面
	rc, out, _ := srRun(t, "script", "retire", "scripts/keep.py", "--dry-run", "--json", "result,object,state,closed_set")
	if rc != 0 {
		t.Fatalf("--json 干跑期望 rc=0，实得 %d", rc)
	}
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil {
		t.Fatalf("--json 包封解不动（out=%q）：%v", out, err)
	}
	if len(env.Items) != 1 || env.Items[0]["object"] != "scripts/keep.py" || env.Items[0]["result"] != "dry_run" {
		t.Fatalf("--json 条目面不对：%+v", env.Items)
	}
}

// ── ⑤ 例外类名册**逐条机检**（GAP-20261007-03）：四列齐 ＋ `object` 在决-12 闭集 ＋ `review_date` 可解析 ──
//
// 名册条目此前**无任何门核**（`adopt_retire_test.go` 的 arDoc 不解析 `retire_exceptions`）
// ⇒ 四列缺一 / 闭集外件 / 坏日期都无人拦。这里对 `scriptRetireExceptionErrors` 做**成对**验收：
// 正控（形态合 ⇒ 0 处）＋ 三条负控（各造一处坏 ⇒ 必报，且**带 `件:行`**）；再拿**真源**现跑一遍。

const (
	// 正控：形态合的名册。
	srRosterGood = `{
  "retire_exceptions": [
    {
      "object": "scripts/ext-tool.sh",
      "category": "外部依赖（退场例外）",
      "reason": "能力依赖外部工具（联网 / 本机物件）",
      "keep_cost": "保留代价：每版随外部工具协议同步核对",
      "review_date": "2026-10-07"
    }
  ]
}
`
	// 负控①：四列缺一（缺 `review_date`）—— 元素 `{` 落在第 3 行。
	srRosterMissingCol = `{
  "retire_exceptions": [
    {
      "object": "scripts/ext-tool.sh",
      "category": "外部依赖（退场例外）",
      "reason": "能力依赖外部工具",
      "keep_cost": "保留代价：每版核对"
    }
  ]
}
`
	// 负控②：`object` 不在决-12 闭集 —— 元素 `{` 落在第 3 行。
	srRosterOutOfSet = `{
  "retire_exceptions": [
    {
      "object": ".githooks/pre-commit",
      "category": "门跑基建·结构性例外",
      "reason": "提交链总开关",
      "keep_cost": "每版同步",
      "review_date": "2026-10-07"
    }
  ]
}
`
	// 负控③：`review_date` 解不动 —— 元素 `{` 落在第 3 行。
	srRosterBadDate = `{
  "retire_exceptions": [
    {
      "object": "scripts/ext-tool.sh",
      "category": "外部依赖（退场例外）",
      "reason": "能力依赖外部工具",
      "keep_cost": "每版核对",
      "review_date": "2026/10/07"
    }
  ]
}
`
)

func srHasLine(errs []string, want string) bool {
	for _, e := range errs {
		if strings.Contains(e, want) {
			return true
		}
	}
	return false
}

func TestScriptRetireRosterExceptionsShape(t *testing.T) {
	const rel = "core/internal/contract/adopt-retire.json"

	// 正控（成对的一半）：形态合的名册 ⇒ 0 处。
	if errs := zerg.ScriptRetireExceptionErrorsForTest(rel, srRosterGood); len(errs) != 0 {
		t.Fatalf("正控破：形态合的名册报了 %d 处：%v", len(errs), errs)
	}

	// 负控①（四列缺一）：缺 `review_date` ⇒ 必报，且**带行号**（`件:3`）。
	errs := zerg.ScriptRetireExceptionErrorsForTest(rel, srRosterMissingCol)
	if len(errs) == 0 {
		t.Fatalf("负控① 破：四列缺一没报（假绿）")
	}
	if !srHasLine(errs, rel+":3:") {
		t.Fatalf("负控① 破：报错没带 `件:行`：%v", errs)
	}
	if !srHasLine(errs, "review_date 列是空的") {
		t.Fatalf("负控① 破：没点名缺的那一列：%v", errs)
	}
	t.Logf("负控①（四列缺一）读数：%v", errs)

	// 负控②（不在闭集）：`object=.githooks/pre-commit` ⇒ 必报，带行号。
	errs = zerg.ScriptRetireExceptionErrorsForTest(rel, srRosterOutOfSet)
	if len(errs) == 0 {
		t.Fatalf("负控② 破：闭集外件没报（假绿）")
	}
	if !srHasLine(errs, rel+":3:") || !srHasLine(errs, "不在决-12 闭集") {
		t.Fatalf("负控② 破：报错没带 `件:行` 或没点名闭集：%v", errs)
	}
	// 负控② 成对：同一条换成 `vendor/` 前缀也不该被当闭集内。
	if e2 := zerg.ScriptRetireExceptionErrorsForTest(rel, strings.Replace(srRosterOutOfSet, ".githooks/pre-commit", "vendor/x.sh", 1)); len(e2) == 0 {
		t.Fatalf("负控② 成对破：`vendor/` 件没被判出闭集")
	}
	t.Logf("负控②（不在闭集）读数：%v", errs)

	// 负控③（`review_date` 解不动）：`2026/10/07` ⇒ 必报，带行号。
	errs = zerg.ScriptRetireExceptionErrorsForTest(rel, srRosterBadDate)
	if len(errs) == 0 {
		t.Fatalf("负控③ 破：`review_date` 解不动没报（假绿）")
	}
	if !srHasLine(errs, rel+":3:") {
		t.Fatalf("负控③ 破：报错没带 `件:行`：%v", errs)
	}
	t.Logf("负控③（review_date 解不动）读数：%v", errs)

	// 真源现跑：真名册必须形态合（✗ 不是只测合成件 ⇒ 门要对真对象有牙）。
	root := repoRootFromCLI(t)
	b, err := os.ReadFile(filepath.Join(root, "core", "internal", "contract", "adopt-retire.json"))
	if err != nil {
		t.Fatalf("真源读不到：%v", err)
	}
	e := zerg.ScriptRetireExceptionErrorsForTest(rel, string(b))
	if len(e) > 0 {
		t.Errorf("真源例外类名册形态坏（%d 处）：\n  %s", len(e), strings.Join(e, "\n  "))
	} else {
		// ★ 条数**现算**（✗ 不写死 —— 写死会随名册增长静默过期）。
		var d struct {
			RetireExceptions []json.RawMessage `json:"retire_exceptions"`
		}
		_ = json.Unmarshal(b, &d)
		t.Logf("真源现跑读数：例外类名册形态合（0 处 · %d 条）", len(d.RetireExceptions))
	}
}

// TestScriptRetireRosterMalformedBlocksCommand —— 端到端：名册形态坏 ⇒ 命令层**不给结论**（退码 2）；
// 成对：同一目标 + 形态合的名册 ⇒ 干跑 0（证明判的是名册面，不是恒红）。
func TestScriptRetireRosterMalformedBlocksCommand(t *testing.T) {
	_, truth, _ := srEnv(t)

	// 正控：夹具名册形态合 ⇒ 干跑 rc=0。
	if rc, _, errb := srRun(t, "script", "retire", "scripts/keep.py", "--dry-run"); rc != 0 {
		t.Fatalf("正控破：形态合的名册下干跑期望 0，实得 %d（err=%q）", rc, errb)
	}

	// 负控：把真源名册改成「四列缺一」⇒ 命令层必报非零，且 stderr 带行号（元素 `{` 在第 5 行）。
	bad := `{
  "schema": "zerg-adopt-retire/1",
  "retire_ledger": [],
  "retire_exceptions": [
    {
      "object": "scripts/ext-tool.sh",
      "reason": "缺 category 与 review_date",
      "keep_cost": "保留代价"
    }
  ]
}
`
	if err := os.WriteFile(truth, []byte(bad), 0o644); err != nil {
		t.Fatalf("写负控真源失败：%v", err)
	}
	rc, _, errb := srRun(t, "script", "retire", "scripts/keep.py", "--dry-run")
	if rc == 0 {
		t.Fatalf("负控破：名册形态坏却退 0（静默绿）—— err=%q", errb)
	}
	if !strings.Contains(errb, "名册形态坏") {
		t.Fatalf("负控破：stderr 没点名「名册形态坏」：%q", errb)
	}
	if !strings.Contains(errb, "adopt-retire.json:5:") {
		t.Fatalf("负控破：stderr 没带 `件:行`（行号）：%q", errb)
	}
}

// ── ⑥ 闭集重定义（决-12）：闭集 = 自研面（全仓 − `vendor/**` − `tools/ocr/venv/**`）────────────
//
// 成对验收（对**两处**消费者同时钉：命令闸 `cmdScriptRetire` ＋ 名册机检 `scriptRetireExceptionErrors`）：
//
//	正控 —— 新纳入面的一件（`publish/**` · `mcp/**` · `ui/scripts/**` · `tools/**` · 顶层）能登进名册
//	  （机检 0 处）＋ 命令层同件干跑 rc=0；
//	负控 —— `vendor/**` 与 `tools/ocr/venv/**` 下的件仍**被拒**（名册报「不在决-12 闭集」带 `件:行`；
//	  命令层 rc=1）。
func TestScriptRetireClosedSetIsSelfDevelopedFace(t *testing.T) {
	const rel = "core/internal/contract/adopt-retire.json"
	root, _, _ := srEnv(t)

	mk := func(p string) {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("建目录失败（%s）：%v", p, err)
		}
		srWrite(t, full, "# probe\n")
	}
	// 新纳入面（决-12 新增的闭集成员）：各面各取一件。
	newFace := []string{
		"publish/mirror-public.sh",
		"mcp/mcp_zerg_server.py",
		"ui/scripts/check-i18n.py",
		"tools/doc_index_gen.py",
		"start-zerg-ui.sh",
	}
	for _, p := range newFace {
		mk(p)
	}
	// 仍被排除面：`tools/ocr/venv/**` 下的一件。
	venvItem := "tools/ocr/venv/lib/python3.9/site-packages/x.py"
	mk(venvItem)

	// —— 正控①：新纳入面的件登进名册 ⇒ 机检 0 处 ——
	rosterNew := `{
  "retire_exceptions": [
    {
      "object": "publish/mirror-public.sh",
      "category": "外部依赖/联网",
      "reason": "现役逐提交镜像器（git 远端）",
      "keep_cost": "随公开面规则逐版核对",
      "review_date": "2026-10-07"
    },
    {
      "object": "ui/scripts/check-i18n.py",
      "category": "工具/生成器（非 gates 面判据件）",
      "reason": "真身在 ui/scripts · 不接为判据",
      "keep_cost": "保留例外登记一行",
      "review_date": "2026-10-07"
    }
  ]
}
`
	if errs := zerg.ScriptRetireExceptionErrorsForTest(rel, rosterNew); len(errs) != 0 {
		t.Fatalf("正控① 破：决-12 新纳入面的件登不进名册（报了 %d 处）：%v", len(errs), errs)
	}

	// —— 正控②：命令层同件干跑 rc=0（闭集闸放行）——
	for _, p := range newFace {
		rc, _, errb := srRun(t, "script", "retire", p, "--dry-run")
		if rc != 0 {
			t.Fatalf("正控② 破：决-12 新纳入面的件 %q 干跑被拒（期望 0，实得 %d · err=%q）", p, rc, errb)
		}
	}

	// —— 负控①：vendor/ 件仍被拒（命令层 rc=1）——
	if rc, _, errb := srRun(t, "script", "retire", "vendor/x.py", "--dry-run"); rc != 1 {
		t.Fatalf("负控① 破：vendor/ 件应仍被拒 rc=1，实得 %d（err=%q）", rc, errb)
	}
	// —— 负控②：tools/ocr/venv/ 件仍被拒（命令层 rc=1）——
	if rc, _, errb := srRun(t, "script", "retire", venvItem, "--dry-run"); rc != 1 {
		t.Fatalf("负控② 破：%s 应仍被拒 rc=1，实得 %d（err=%q）", venvItem, rc, errb)
	}
	// —— 负控③（成对另一半）：名册机检对排除面件必报，带 `件:行` ——
	for _, bad := range []string{"vendor/x.py", venvItem} {
		text := `{
  "retire_exceptions": [
    {
      "object": "` + bad + `",
      "category": "c",
      "reason": "r",
      "keep_cost": "k",
      "review_date": "2026-10-07"
    }
  ]
}
`
		got := zerg.ScriptRetireExceptionErrorsForTest(rel, text)
		if len(got) == 0 {
			t.Fatalf("负控③ 破：排除面件 %q 登进名册没报（假绿）", bad)
		}
		if !srHasLine(got, rel+":3:") || !srHasLine(got, "不在决-12 闭集") {
			t.Fatalf("负控③ 破：%q 的报错没带 `件:行` 或没点名闭集：%v", bad, got)
		}
	}
}

// TestScriptRetireRealRosterMissingKeepCostReddens —— ★ 负控（**真名册**派生）：把真真源里**首条**的
// `keep_cost` 抹掉 ⇒ 机检必红、且带 `件:行`（门要对**真对象**有牙，✗ 不是只测合成件）。
func TestScriptRetireRealRosterMissingKeepCostReddens(t *testing.T) {
	const rel = "core/internal/contract/adopt-retire.json"
	root := repoRootFromCLI(t)
	b, err := os.ReadFile(filepath.Join(root, "core", "internal", "contract", "adopt-retire.json"))
	if err != nil {
		t.Fatalf("真源读不到：%v", err)
	}
	raw := string(b)

	// 正控（成对的一半）：真名册现跑形态合（0 处）。
	if errs := zerg.ScriptRetireExceptionErrorsForTest(rel, raw); len(errs) != 0 {
		t.Fatalf("正控破：真名册现跑报了 %d 处：%v", len(errs), errs)
	}

	// 负控：删掉真名册里**首条** `keep_cost` 那一行（逐字节派生 · ✗ 不写真件）。
	lines := strings.Split(raw, "\n")
	idx := -1
	for i, ln := range lines {
		if strings.Contains(ln, `"keep_cost":`) {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("真名册里找不到 keep_cost 行（形态变了？）")
	}
	mutated := strings.Join(append(append([]string{}, lines[:idx]...), lines[idx+1:]...), "\n")
	errs := zerg.ScriptRetireExceptionErrorsForTest(rel, mutated)
	if len(errs) == 0 {
		t.Fatal("负控破：真名册抹掉一条 keep_cost 后没报（假绿）")
	}
	if !srHasLine(errs, rel+":") || !srHasLine(errs, "keep_cost 列是空的") {
		t.Fatalf("负控破：抹 keep_cost 的报错没带 `件:行` 或没点名该列：%v", errs)
	}
	t.Logf("真名册派生负控读数（抹首条 keep_cost）：%v", errs)
}
