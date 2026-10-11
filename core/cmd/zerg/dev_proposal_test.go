// dev_proposal_test.go —— 提案件通道（`zerg dev proposal new|list|show` · §17.2 ② 改环 · 开工单 T-57）
// 的**判据机检**（层①进程内 · 外部测试包 `package main_test`）。
//
// 本票的四条判据逐条落成断言：
//
//	① **读环一条命令都不新增**（§17.2.1 ① 逐字「一条都不新增 —— 复用 §三 既有族」）：
//	   `dev` 族的命令路径是**闭集** `{dev proposal, dev release, dev rollback}` —— 读事实一律走
//	   既有族（`zerg context ls` / `zerg task show` / `zerg doctor` …），本族**不自建读命令**。
//	② **产出后系统状态逐字不变**（§17.3 铁律③ 判据）：`new` 只在 `ZERG_PROPOSAL_DIR` 下新增一件，
//	   工作目录（cwd 快照）**逐字节不变** —— 不写仓、不触主控。
//	③ **`--target` 必须回指既有编号**（§17.6 `SD1`）：回指不上 ⇒ `exit 2`、stdout **0 字节**，
//	   且**不新立退码**（用的是主表里的 2，不是自造码）。
//	④ **没退点 / 没出处的件不许提**（判据④）：缺 `--rollback` 或 `--evidence` ⇒ `exit 2`。
//
// 负控（本票的「判据会红」证明）：testProposalJudge 是三格判据的**唯一判定口**，
// TestDevProposalHarnessHasTeeth 直接喂它一个**错的**实况（坏输入却 rc=0）⇒ 必须报错。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// devFamilyClosure —— 判据①的闭集（**一条读命令都不新增**；`dev` 族 = §17.4 全表那七条）。
// ★ 2026-09-20 批 E · T-61 补入 `dev receipt`：它是**交接回执**（一轮一页 · 落状态目录），
//
//	不是「读事实」的命令（读事实一律走既有族）⇒ 判据①逐字「读环**一条都不新增**」仍然成立。
//
// ★ 2026-09-20 批 E · T-58 补入 `dev build` / `dev test` / `dev verify`（§17.4 第 2/3/5 条）：
//
//	三条都不是「读事实」的新读命令 —— 它们是**候选区**的建 / 测 / 收证据：
//	· `build` / `test` 本版只到**干跑档**出计划件（真跑拒执 ⇒ 2），且**判据（证据单）在前**（§17.7）；
//	· `verify` 收的是**门禁结果表**（一步一日志的既有产物）—— 判据真源仍是门禁步骤表，它**不新立判据**（§17.4 接缝第 5 行逐字）。
//	⇒ 「读事实走既有族」这条没收；闭集本身随 §17.4 的条目**同批**刷新（棘轮，不许悄悄长大）。
var devFamilyClosure = []string{"dev proposal", "dev release", "dev rollback", "dev receipt",
	"dev build", "dev test", "dev verify", "dev edit",
	// ★ GAP-20260927-377（P0 · 本枚随动）：`dev check --syntax`（类型面只读正门）落地 ⇒ 闭集**同批**长大
	//   （棘轮：闭集本身随 §17.4 的条目/SD 同批刷新 —— 这里手工补，不许悄悄漏）。
	"dev check",
	// ★ GAP-20261010-30（本枚随动）：`dev reclaim`（直写补正门 · D3 **写面**：调既有 `dev edit` 净零重落
	//   只补审计链 · 盘上内容逐字节不变）落地 ⇒ 闭集**同批**长大。它是**写命令**，不是「读事实」的新读命令
	//   ⇒ 判据①「读环**一条都不新增**」仍成立；棘轮：闭集随新命令同批刷新，不许悄悄漏。
	"dev reclaim"}

// snapshotDir 给一棵目录树拍逐文件 sha256 快照（相对路径排序 ⇒ 可比对「逐字节不变」）。
func snapshotDir(t *testing.T, root string) string {
	t.Helper()
	type ent struct{ rel, sum string }
	var ents []ent
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		sum := sha256.Sum256(b)
		ents = append(ents, ent{rel, hex.EncodeToString(sum[:])})
		return nil
	})
	if err != nil {
		t.Fatalf("快照 %s 失败：%v", root, err)
	}
	sort.Slice(ents, func(i, j int) bool { return ents[i].rel < ents[j].rel })
	var b strings.Builder
	for _, e := range ents {
		fmt.Fprintf(&b, "%s\t%s\n", e.rel, e.sum)
	}
	return b.String()
}

// testCriterion —— 测试用的**合法判据**（一条本版跑得动、零副作用的命令面命令）。
// 为什么用 `--dry-run` 那一态：它永远跑得动且零副作用（D3b 第一步的判据口径）。
const testCriterion = "zerg build all --only cli --dry-run"

// proposalEvidenceDir —— 本仓「出处必须在册」的闸（`GAP-20260927-254`）读的是状态目录里的缺口账
// （`$ZERG_STATE_DIR/zerg-cli-gaps.jsonl` · 默认 `~/.zerg/state`）。夹具**自带一份合成账**，把出处指到
// 里面真在册的那一条 ⇒ 与「本机账在不在 / 环境里有没有 ZERG_STATE_DIR」脱钩（判据不变，只换夹具）。
const proposalEvidenceInLedger = "GAP-20260927-254"

func proposalEvidenceDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	line := `{"id":"` + proposalEvidenceInLedger + `"}`
	if err := os.WriteFile(filepath.Join(dir, "zerg-cli-gaps.jsonl"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_STATE_DIR", dir)
}

// ---- D3b 第一步：提案接校验（判据**可机检** · 缺判据 / 判据跑不动 ⇒ rc 2）----

func TestDevProposalCriterionMustBeMachineCheckable(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())
	base := []string{"--title", "判据", "--target", "待办:" + ids[0], "--goal", "g",
		"--evidence", "GAP-20260927-254", "--rollback", "r"}

	// 负控①：**缺判据** ⇒ 2 · stdout 0 字节 · 点名「判据」
	rc, out, errb := runProposalNew(t, base...)
	if rc != 2 {
		t.Errorf("缺 --criterion ⇒ 退码 %d（要 2 · D3b「提案缺判据 ⇒ 退码 2」）", rc)
	}
	if out != "" {
		t.Errorf("拒收时 stdout 必须 0 字节，实测 %d 字节", len(out))
	}
	if !strings.Contains(errb, "判据") {
		t.Errorf("stderr 没点名「判据」：%q", errb)
	}

	// 负控②：判据**不是命令面**（人话 / 别的工具）⇒ 2
	rc2, _, errb2 := runProposalNew(t, append(append([]string{}, base...), "--criterion", "看门禁全绿就行")...)
	if rc2 != 2 || !strings.Contains(errb2, "首词") {
		t.Errorf("人话判据 ⇒ rc=%d（要 2）、stderr 要点名首词：%q", rc2, errb2)
	}

	// 负控③：判据的命令**不在命令树里**（幽灵命令）⇒ 2（与门⑫ 同一条口径）
	rc3, _, errb3 := runProposalNew(t, append(append([]string{}, base...), "--criterion", "zerg nosuchfamily nosuchaction")...)
	if rc3 != 2 || !strings.Contains(errb3, "不在命令树里") {
		t.Errorf("幽灵命令判据 ⇒ rc=%d（要 2）、stderr 要点名「不在命令树里」：%q", rc3, errb3)
	}

	// 负控④：判据落在**本版未开放的危险档**上（真跑跑不到一个绿）⇒ 2
	rc4, _, errb4 := runProposalNew(t, append(append([]string{}, base...),
		"--criterion", "zerg dev release --candidate DEV-0001")...)
	if rc4 != 2 || !strings.Contains(errb4, "危险档") {
		t.Errorf("未开放危险档当判据 ⇒ rc=%d（要 2）、stderr 要点名「危险档」：%q", rc4, errb4)
	}

	// 正控：可机检的判据 ⇒ 0，且件里记着它（判据字段只增不改）
	rc5, out5, errb5 := runProposalNew(t, append(append([]string{}, base...), "--criterion", testCriterion)...)
	if rc5 != 0 {
		t.Fatalf("可机检判据 ⇒ rc=%d（要 0）· stderr=%s", rc5, errb5)
	}
	if strings.TrimSpace(out5) == "" {
		t.Errorf("成功时 stdout 要给提案 id")
	}
	// `--file` 声明（`dev edit` 的作用域真源）
	rc6, _, errb6 := runProposalNew(t, append(append([]string{}, base...),
		"--criterion", testCriterion, "--file", "core/cmd/zerg/main.go")...)
	if rc6 != 0 {
		t.Errorf("带 --file ⇒ rc=%d（要 0）· stderr=%s", rc6, errb6)
	}
	// 负控⑤：`--file` 绝对路径 ⇒ 2（作用域只认仓内相对路径）
	rc7, _, errb7 := runProposalNew(t, append(append([]string{}, base...),
		"--criterion", testCriterion, "--file", "/etc/passwd")...)
	if rc7 != 2 || !strings.Contains(errb7, "绝对路径") {
		t.Errorf("绝对路径的 --file ⇒ rc=%d（要 2）、stderr 要点名「绝对路径」：%q", rc7, errb7)
	}
}

func TestDevProposalCheckFailsClosed(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	dir := t.TempDir()
	t.Setenv("ZERG_PROPOSAL_DIR", dir)
	var out, errb strings.Builder

	// 空转（一件都没有）⇒ 2（不给结论 · 不静默放绿）
	rc := zerg.RunForTest([]string{"dev", "proposal", "check"}, &out, &errb)
	if rc != 2 {
		t.Errorf("空目录 check ⇒ rc=%d（要 2 · 空转不给结论）", rc)
	}

	// 有件（判据可跑）⇒ 0
	out.Reset()
	errb.Reset()
	if rc, _, e6 := runProposalNew(t, "--title", "正控", "--target", "待办:"+ids[0], "--goal", "g",
		"--evidence", "GAP-20260927-254", "--rollback", "r", "--criterion", testCriterion); rc != 0 {
		t.Fatalf("正控件提不出来：rc=%d · stderr=%s", rc, e6)
	}
	if rc := zerg.RunForTest([]string{"dev", "proposal", "check"}, &out, &errb); rc != 0 {
		t.Errorf("判据可跑的件 check ⇒ rc=%d（要 0）· stderr=%s", rc, errb.String())
	}

	// 塞一件**手写的坏件**（判据是空串 —— 绕过 new 的校验才可能的形态）⇒ 2，且逐件点名
	bad, _ := json.Marshal(map[string]any{
		"id": "DEV-9001", "title": "手写坏件", "target": ids[0], "goal": "g",
		"evidence": []string{"e"}, "rollback_ref": "r", "criterion": "", "state": "未决",
	})
	if err := os.WriteFile(filepath.Join(dir, "DEV-9001.json"), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	rc = zerg.RunForTest([]string{"dev", "proposal", "check", "DEV-9001"}, &out, &errb)
	if rc != 2 {
		t.Errorf("判据为空的件 check ⇒ rc=%d（要 2 · fail-closed）· stderr=%s", rc, errb.String())
	}
	if !strings.Contains(out.String(), "缺") {
		t.Errorf("check 的行里要给判据态（缺/跑不动/可跑）：%q", out.String())
	}
}

// runProposalNew 起一次 `dev proposal new`（**只走进程内 run**，不起二进制 —— 层①的落点）。
func runProposalNew(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(append([]string{"dev", "proposal", "new", "--yes"}, argv...), &out, &errb)
	return rc, out.String(), errb.String()
}

// proposalJudge 是本票①②③④四格判据的**唯一判定口**。
// 抽成函数的目的同上层的 judgeCase：负控要能直接喂一个错的实况进来（见本文件末）。
type proposalLive struct {
	RC        int
	Stdout    string
	StderrTxt string
	// CwdBefore / CwdAfter 是**工作目录**（不含提案件目录）的前后快照 —— 判据②的取值来源。
	CwdBefore, CwdAfter string
	// ProposalFiles 是提案件目录里的件名（判据②的「只在这里落件」）。
	ProposalFiles []string
	// LoggedCommands 是**真台账**里记到的命令面调用（T-61 的 H1 留痕面；本票先留接口，空 = 未接）。
	LoggedCommands []string
}

func proposalJudge(l proposalLive) []error {
	var errs []error
	// 判据②：工作目录逐字节不变（含「不写仓」）
	if l.CwdBefore != l.CwdAfter {
		errs = append(errs, fmt.Errorf("判据② 破：工作目录被改了（快照不等）\n前:\n%s\n后:\n%s", l.CwdBefore, l.CwdAfter))
	}
	return errs
}

// ---- 判据① 读环不新增命令 ----

func TestDevFamilyIsACHangedOnlyRingNoNewReadCommands(t *testing.T) {
	got := []string{}
	for _, p := range zerg.CommandPathsForTest() {
		if p == "dev" || strings.HasPrefix(p, "dev ") {
			got = append(got, p)
		}
	}
	sort.Strings(got)
	want := append([]string{}, devFamilyClosure...)
	sort.Strings(want)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("`dev` 族的命令路径 = %v（要闭集 %v）—— 判据①逐字「读环**一条都不新增**」："+
			"读事实走既有族（context ls / task show / doctor / gate ls），本族不许自建读命令", got, want)
	}
}

// ---- 判据③①④：回指 / 拒收 / 退码不新立 ----

func TestDevProposalTargetMustReferenceExistingID(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来（%v · %d 条）—— 判据③的判据自己坏了 ⇒ 不给结论", err, len(ids))
	}
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())

	// ③-a 回指不上 ⇒ exit 2（**不是**自造码）· stdout 0 字节
	rc, out, errb := runProposalNew(t, "--title", "回指不上", "--target", "待办:ZZZZ-9999",
		"--goal", "g", "--evidence", "GAP-20260927-254", "--rollback", "r", "--criterion", testCriterion)
	if rc != 2 {
		t.Errorf("回指不上的目标 ⇒ 退码 %d（要 2 · §17.6 SD1「回指不上 ⇒ exit=2 · 不新立码」）", rc)
	}
	if out != "" {
		t.Errorf("回指不上时 stdout 必须 0 字节，实测 %d 字节：%q", len(out), out)
	}
	if !strings.Contains(errb, "回指不上") {
		t.Errorf("stderr 没点名「回指不上」这个判词：%q", errb)
	}

	// ③-b 回指得上（真台账里的既有编号）⇒ exit 0，且件落在提案件目录
	rc, out, errb = runProposalNew(t, "--title", "回指得上", "--target", "待办:"+ids[0],
		"--goal", "g", "--evidence", "GAP-20260927-254", "--rollback", "r", "--criterion", testCriterion)
	if rc != 0 {
		t.Errorf("既有编号 %q ⇒ 退码 %d（要 0）· stderr=%s", ids[0], rc, errb)
	}
	if strings.TrimSpace(out) == "" {
		t.Errorf("成功时 stdout 要给提案 id（拿到空）· stderr=%s", errb)
	}
}

// TestDevProposalRollbackRepeatedIsUsageError —— 重复给 `--rollback` ⇒ 拒执并点名（缺口 Q-154 同族 ·
// 同名旗标只许给一次；照 `ci green --run` 先例）。改前 `--rollback A --rollback B` 静默收成末值 ⇒ rc=0。
func TestDevProposalRollbackRepeatedIsUsageError(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())
	base := []string{"--title", "t", "--target", "待办:" + ids[0], "--goal", "g",
		"--evidence", proposalEvidenceInLedger, "--criterion", testCriterion}

	rc, _, errb := runProposalNew(t, append(append([]string{}, base...), "--rollback", "r", "--rollback", "r2")...)
	if rc != 2 {
		t.Errorf("`--rollback` 给两次 ⇒ 退码 %d（要 2 · 同名旗标重复 = 用法错）", rc)
	}
	if !strings.Contains(errb, "只许给一次") || !strings.Contains(errb, "--rollback") {
		t.Errorf("拒执要点名 `--rollback` 与「只许给一次」：%q", errb)
	}
	rc2, _, errb2 := runProposalNew(t, append(append([]string{}, base...), "--rollback", "r")...)
	if strings.Contains(errb2, "只许给一次") {
		t.Errorf("只给一次 `--rollback` 不该触发重复拒执（rc=%d）：%q", rc2, errb2)
	}
}

// TestDevProposalSingleValFlagsRepeatedIsUsageError —— 重复给**单值语义**旗标 ⇒ 同型拒执并点名
// （照 `--rollback` 那一处已落的形态 · 缺口 Q-154 同族）。逐旗标两态：
//
//	① 重复给 ⇒ rc=2 · stderr 点名该旗标与「只许给一次」；
//	② 只给一次 ⇒ 不触发该重复拒执。
//
// ★ **多值语义**旗标（`--evidence` / `--file`）**不在本闸** —— 见下一个用例。
func TestDevProposalSingleValFlagsRepeatedIsUsageError(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())
	base := []string{"--title", "t", "--target", "待办:" + ids[0], "--goal", "g",
		"--evidence", proposalEvidenceInLedger, "--rollback", "r", "--criterion", testCriterion}

	// base 里已有一枚的（title/target/goal/criterion）只补一枚即成「两次」；其余补两枚。
	singleValInBase := map[string]bool{"--title": true, "--target": true, "--goal": true, "--criterion": true}
	cases := []struct {
		flag  string
		extra []string
	}{
		{"--title", []string{"--title", "t2"}},
		{"--target", []string{"--target", "待办:" + ids[0]}},
		{"--goal", []string{"--goal", "g2"}},
		{"--criterion", []string{"--criterion", testCriterion}},
		{"--criterion-new", []string{"--criterion-new", "zerg future cmd"}},
		{"--by", []string{"--by", "ai"}},
		{"--subject", []string{"--subject", "s"}},
		{"--subject-kind", []string{"--subject-kind", "ai"}},
		{"--egg-id", []string{"--egg-id", "EGG-1"}},
		{"--approver", []string{"--approver", "Mr2109"}},
		{"--approver-kind", []string{"--approver-kind", "human"}},
	}
	for _, c := range cases {
		// ① 重复给 ⇒ rc=2 · 点名该旗标与「只许给一次」
		rep := append(append([]string{}, base...), c.extra...)
		if !singleValInBase[c.flag] {
			rep = append(rep, c.extra...) // 不在 base 里的：补两枚
		}
		rc, _, errb := runProposalNew(t, rep...)
		if rc != 2 {
			t.Errorf("%s 给两次 ⇒ rc=%d（要 2 · 单值旗标重复 = 用法错 2）· stderr=%s", c.flag, rc, errb)
		}
		if !strings.Contains(errb, "只许给一次") || !strings.Contains(errb, c.flag) {
			t.Errorf("%s 重复拒执要点名该旗标与「只许给一次」：%q", c.flag, errb)
		}
		// ② 只给一次 ⇒ 不触发该重复拒执
		var once []string
		if singleValInBase[c.flag] {
			once = base
		} else {
			once = append(append([]string{}, base...), c.extra...)
		}
		rc1, _, errb1 := runProposalNew(t, once...)
		if strings.Contains(errb1, "只许给一次") {
			t.Errorf("%s 只给一次不该触发重复拒执（rc=%d）：%q", c.flag, rc1, errb1)
		}
	}
	// `--batch` 也是单值语义：重复给同型拒执（位置在最前 · 任何盘面动作之前）
	rcB, _, errB := runProposalNew(t, "--batch", "a.json", "--batch", "b.json")
	if rcB != 2 || !strings.Contains(errB, "只许给一次") || !strings.Contains(errB, "--batch") {
		t.Errorf("`--batch` 给两次 ⇒ rc=%d（要 2 · 点名 `--batch` 与「只许给一次」）：%q", rcB, errB)
	}
}

// TestDevProposalMultiValFlagsMayRepeat —— ★ **多值语义**旗标（`--evidence` / `--file`）**不在重复拒执闸**内
// （它们本就该能重复给 · 逐枚 append）；重复给不触发「只许给一次」且照旧可提。
func TestDevProposalMultiValFlagsMayRepeat(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())
	base := []string{"--title", "t", "--target", "待办:" + ids[0], "--goal", "g",
		"--evidence", proposalEvidenceInLedger, "--criterion", testCriterion, "--rollback", "r"}
	rc, _, errb := runProposalNew(t, append(append([]string{}, base...),
		"--evidence", proposalEvidenceInLedger,
		"--file", "core/cmd/zerg/family_dev.go",
		"--file", "core/cmd/zerg/main.go")...)
	if strings.Contains(errb, "只许给一次") {
		t.Errorf("多值旗标（--evidence/--file）重复给不该触发单值重复拒执（rc=%d）：%q", rc, errb)
	}
	if rc != 0 {
		t.Errorf("多值旗标重复给应照旧可提（rc=%d 要 0）· stderr=%s", rc, errb)
	}
}

func TestDevProposalRefusesWithoutRollbackOrEvidence(t *testing.T) {
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())
	base := []string{"--title", "t", "--target", "待办:" + ids[0], "--goal", "g", "--criterion", testCriterion}

	// ④-a 没退点 ⇒ 2
	rc, out, errb := runProposalNew(t, append(append([]string{}, base...), "--evidence", "e")...)
	if rc != 2 {
		t.Errorf("缺 --rollback ⇒ 退码 %d（要 2 · 「没退点的件不许提」）", rc)
	}
	if out != "" {
		t.Errorf("拒收时 stdout 必须 0 字节，实测 %d 字节", len(out))
	}
	if !strings.Contains(errb, "退点") {
		t.Errorf("stderr 没点名「退点」：%q", errb)
	}

	// ④-b 没出处 ⇒ 2
	rc, out, errb = runProposalNew(t, append(append([]string{}, base...), "--rollback", "r")...)
	if rc != 2 {
		t.Errorf("缺 --evidence ⇒ 退码 %d（要 2 · 「证据为空 ⇒ 不给结论」）", rc)
	}
	if out != "" {
		t.Errorf("拒收时 stdout 必须 0 字节，实测 %d 字节", len(out))
	}
	if !strings.Contains(errb, "出处") {
		t.Errorf("stderr 没点名「出处」：%q", errb)
	}
}

// ---- 判据②：产出后系统状态逐字不变 ----

func TestDevProposalNewHasZeroSideEffectOnTheWorkspace(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	work := t.TempDir()
	prop := t.TempDir()
	// 工作目录里先放一件「哨兵」：它必须一个字节都不动（连 mtime 之外的字节都不动）
	if err := os.WriteFile(filepath.Join(work, "哨兵.txt"), []byte("原样\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotDir(t, work)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(wd) }()
	t.Setenv("ZERG_PROPOSAL_DIR", prop)

	rc, _, errb := runProposalNew(t, "--title", "零副作用", "--target", "待办:"+ids[0],
		"--goal", "g", "--evidence", "GAP-20260927-254", "--rollback", "r", "--criterion", testCriterion)
	if rc != 0 {
		t.Fatalf("`new` 退码 %d（要 0）· stderr=%s", rc, errb)
	}
	after := snapshotDir(t, work)
	files, err := os.ReadDir(prop)
	if err != nil {
		t.Fatalf("提案件目录读不出来：%v（判据②要它**至少**落一件）", err)
	}
	names := []string{}
	for _, f := range files {
		names = append(names, f.Name())
	}
	if len(names) != 1 {
		t.Errorf("提案件目录里应有且只有 1 件，实测 %d 件：%v", len(names), names)
	}
	for _, e := range proposalJudge(proposalLive{
		RC: rc, StderrTxt: errb, CwdBefore: before, CwdAfter: after, ProposalFiles: names,
	}) {
		t.Error(e)
	}
	if before != after {
		t.Errorf("判据② 破：工作目录快照不等\n前:\n%s\n后:\n%s", before, after)
	}
}

// ---- 负控：判定口**真的有牙** ----

// TestDevProposalHarnessHasTeeth —— 负控（本票「判据会红」的证明）：
// 把**同一个**判定口喂一个错的实况（工作目录被改了）⇒ 它必须报错。
// 没有这一格，上面那条断言只是「今天恰好相等」，不是「判据会红」。
func TestDevProposalHarnessHasTeeth(t *testing.T) {
	errs := proposalJudge(proposalLive{CwdBefore: "a\t1\n", CwdAfter: "a\t2\n"})
	if len(errs) == 0 {
		t.Fatal("负控失败：判定口放过了「工作目录被改」这件事 ⇒ 本票的判据不会红（假绿）")
	}
	if !strings.Contains(errs[0].Error(), "判据②") {
		t.Errorf("负控报错文案没点名判据②：%v", errs[0])
	}
	// 正控：相同快照 ⇒ 不报（不误判）
	if errs := proposalJudge(proposalLive{CwdBefore: "a\t1\n", CwdAfter: "a\t1\n"}); len(errs) != 0 {
		t.Errorf("正控失败：相同快照被判红：%v", errs)
	}
}

// ---- ★ GAP-20260927-266：判据的**行断言形态** `<件>:<行> 含 <串>`（本枚）----

// proposalLineOf 现读某件里含 needle 的那一行的行号（测试自锚 —— 不写死行号，改代码后仍成立）。
func proposalLineOf(t *testing.T, rel, needle string) int {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// 上溯找仓根（判据：有 core/internal/version/version.go）—— 不赌测试 cwd 恰好是包目录。
	root := wd
	for {
		if _, err := os.Stat(filepath.Join(root, "core", "internal", "version", "version.go")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatalf("从 %s 上溯找不到仓根", wd)
		}
		root = parent
	}
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("读 %s 失败：%v", rel, err)
	}
	for i, ln := range strings.Split(string(b), "\n") {
		if strings.Contains(ln, needle) {
			return i + 1
		}
	}
	t.Fatalf("%s 里找不到 %q（测试自锚失败）", rel, needle)
	return 0
}

// TestDevProposalCriterionAcceptsLineAssertion —— 判据可以写成**行断言** `<件>:<行> 含 <串>`：
// 钉到具体一行才有牙（目录级搜索**改前也命中** = 无牙 · GAP-20260927-266）。
// 三态都判：成立 ⇒ new/check 均 0；不成立 ⇒ check 必须红（这一格就是「判据有牙」的证明）。
func TestDevProposalCriterionAcceptsLineAssertion(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())
	base := []string{"--title", "行断言", "--target", "待办:" + ids[0], "--goal", "g",
		"--evidence", "GAP-20260927-254", "--rollback", "r"}

	target := "core/cmd/zerg/family_dev.go"
	ln := proposalLineOf(t, target, "func criterionRunnable")

	// 正控：**成立**的行断言 ⇒ new 0，check 0。
	critOK := fmt.Sprintf("%s:%d 含 criterionRunnable", target, ln)
	if rc, out, errb := runProposalNew(t, append(append([]string{}, base...), "--criterion", critOK)...); rc != 0 {
		t.Fatalf("行断言判据 ⇒ new rc=%d（要 0）· stderr=%s", rc, errb)
	} else if strings.TrimSpace(out) == "" {
		t.Errorf("成功时 stdout 要给提案 id")
	}
	var out, errb strings.Builder
	if rc := zerg.RunForTest([]string{"dev", "proposal", "check"}, &out, &errb); rc != 0 {
		t.Errorf("成立的行断言 ⇒ check rc=%d（要 0）· stderr=%s", rc, errb.String())
	}

	// 负控：**不成立**的行断言 ⇒ new 仍 0（形态合法即可提），但 check 必须红（判据未达 · 无牙即红）。
	critBad := fmt.Sprintf("%s:%d 含 zzz_not_present_zzz_266", target, ln)
	if rc, _, errb2 := runProposalNew(t, append(append([]string{}, base...), "--criterion", critBad)...); rc != 0 {
		t.Fatalf("不成立的行断言 ⇒ new rc=%d（要 0 · 形态合法即可提）· stderr=%s", rc, errb2)
	}
	out.Reset()
	errb.Reset()
	if rc := zerg.RunForTest([]string{"dev", "proposal", "check"}, &out, &errb); rc == 0 {
		t.Errorf("不成立的行断言 ⇒ check rc=%d（要 2 · 不给结论）· stderr=%s", rc, errb.String())
	}

	// 负控②：绝对路径的件名 ⇒ 拒（形态只认仓内相对路径）。
	critAbs := "/etc/passwd:1 含 root"
	if rc, _, errb3 := runProposalNew(t, append(append([]string{}, base...), "--criterion", critAbs)...); rc != 2 || !strings.Contains(errb3, "绝对路径") {
		t.Errorf("绝对路径的行断言 ⇒ rc=%d（要 2 · 点名「绝对路径」）· stderr=%s", rc, errb3)
	}
}

// ---- ★ GAP-20261009-09：`--evidence` 收「描述性证据」（无 `GAP-`/`DEV-` 前缀的自由文本）----

// TestDevProposalEvidenceAcceptsDescriptiveText —— 证据面认两类：
//
//	① **id 形状**（`GAP-…` / `DEV-…`）⇒ 仍须**在册**（不放宽对真 id 的校验）；
//	② **描述性证据**（无前缀自由文本）⇒ 直接收（两态：改前 rc=2、改后 rc=0）。
//
// 负控：**不存在的 id 形状**仍 rc=2（不让不存在的 id 蒙混）。
func TestDevProposalEvidenceAcceptsDescriptiveText(t *testing.T) {
	proposalEvidenceDir(t) // 合成账里只有在册的 proposalEvidenceInLedger
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())
	base := []string{"--title", "证据面", "--target", "待办:" + ids[0], "--goal", "g",
		"--rollback", "r", "--criterion", testCriterion}

	// ② 描述性证据（无前缀自由文本）⇒ rc=0（收下 · 改前是 rc=2）
	rc, out, errb := runProposalNew(t, append(append([]string{}, base...), "--evidence", "这是一段自由文本证据")...)
	if rc != 0 {
		t.Fatalf("描述性证据 ⇒ rc=%d（要 0 · 收下）· stderr=%s", rc, errb)
	}
	if strings.TrimSpace(out) == "" {
		t.Errorf("成功时 stdout 要给提案 id")
	}

	// ① 在册 id 形状 ⇒ rc=0（不变）
	if rc, _, errb := runProposalNew(t, append(append([]string{}, base...), "--evidence", "GAP-20260927-254")...); rc != 0 {
		t.Errorf("在册 id ⇒ rc=%d（要 0）· stderr=%s", rc, errb)
	}

	// 负控：**不存在的 GAP id** ⇒ 仍 rc=2（仍在册校验 · 不让不存在的 id 蒙混）
	rc3, out3, errb3 := runProposalNew(t, append(append([]string{}, base...), "--evidence", "GAP-29990101-99")...)
	if rc3 != 2 {
		t.Errorf("不存在的 GAP id ⇒ rc=%d（要 2 · 仍在册校验）· stderr=%s", rc3, errb3)
	}
	if out3 != "" {
		t.Errorf("拒收时 stdout 必须 0 字节，实测 %d 字节", len(out3))
	}
	if !strings.Contains(errb3, "查不到") {
		t.Errorf("stderr 要点名「查不到」：%q", errb3)
	}
}

// ---- ★ GAP-20260928-161（方案甲）：前瞻判据档 `--criterion-new`（本版**还跑不动**的判据）----

// TestDevProposalCriterionNewForwardSlot —— 前瞻判据档三格：
//
//	正控：前瞻判据（本版**跑不动**的命令形态）被收下，且**分栏落盘**（`criterion_new` + 它的态=前瞻）。
//	负控①：强档 `--criterion` 收到**前瞻形态** ⇒ 拒（强档一字未动 · 两档不许互相冒充）。
//	负控②：前瞻档 `--criterion-new` 收到**本版真能跑**的命令 ⇒ 拒（不许冒充前瞻）。
func TestDevProposalCriterionNewForwardSlot(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	dir := t.TempDir()
	t.Setenv("ZERG_PROPOSAL_DIR", dir)

	// 前瞻形态 = 一条首词是 zerg、但**本版命令树里解析不到**的命令（将来才有的命令形态）。
	const forward = "zerg gap backfill-future --dry-run"

	// ---- 负控①：强档 `--criterion` 收前瞻形态 ⇒ 红（强档的判定口一字未动：解析不到 ⇒ 不收）----
	rcS, outS, errS := runProposalNew(t, "--title", "强档收前瞻", "--target", "待办:"+ids[0], "--goal", "g",
		"--evidence", "GAP-20260927-254", "--rollback", "r", "--criterion", forward)
	if rcS != 2 || !strings.Contains(errS, "不在命令树里") {
		t.Errorf("强档 `--criterion` 收前瞻形态 ⇒ rc=%d（要 2 · 点名「不在命令树里」）· stderr=%s", rcS, errS)
	}
	if outS != "" {
		t.Errorf("拒收时 stdout 必须 0 字节，实测 %d 字节", len(outS))
	}

	// ---- 负控②：前瞻档 `--criterion-new` 收本版**真能跑**的命令 ⇒ 红（不许冒充前瞻）----
	rcN, outN, errN := runProposalNew(t, "--title", "前瞻档收能真跑", "--target", "待办:"+ids[0], "--goal", "g",
		"--evidence", "GAP-20260927-254", "--rollback", "r", "--criterion", testCriterion, "--criterion-new", testCriterion)
	if rcN != 2 || !strings.Contains(errN, "能解析到") {
		t.Errorf("前瞻档 `--criterion-new` 收本版真能跑的命令 ⇒ rc=%d（要 2 · 点名「能解析到」）· stderr=%s", rcN, errN)
	}
	if outN != "" {
		t.Errorf("拒收时 stdout 必须 0 字节，实测 %d 字节", len(outN))
	}

	// ---- 正控：前瞻判据入档 ⇒ rc=0；落件**显式标前瞻**（分栏：criterion_new 值 + criterion_new_state=前瞻）----
	rc, out, errb := runProposalNew(t, "--title", "前瞻档", "--target", "待办:"+ids[0], "--goal", "g",
		"--evidence", "GAP-20260927-254", "--rollback", "r", "--criterion", testCriterion, "--criterion-new", forward)
	if rc != 0 {
		t.Fatalf("前瞻判据入档 ⇒ rc=%d（要 0）· stderr=%s", rc, errb)
	}
	id := strings.TrimSpace(out)
	if id == "" {
		t.Fatalf("成功时 stdout 要给提案 id")
	}
	b, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		t.Fatalf("读落件 %s 失败：%v", id, err)
	}
	var rec map[string]any
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("落件 JSON 解不开：%v", err)
	}
	if got, _ := rec["criterion_new"].(string); got != forward {
		t.Errorf("落件 `criterion_new` = %q（要 %q）", got, forward)
	}
	if got, _ := rec["criterion"].(string); got != testCriterion {
		t.Errorf("落件 `criterion`（强档）被动了 = %q（要 %q · 强档一字未动）", got, testCriterion)
	}
	// 机器面同一格：`criterion_new_state` 必须**显式标前瞻**（分栏落盘的「它的态」）。
	var jout, jerr strings.Builder
	if rc := zerg.RunForTest([]string{"dev", "proposal", "show", id, "--json", "criterion,criterion_state,criterion_new,criterion_new_state"}, &jout, &jerr); rc != 0 {
		t.Fatalf("show --json 取分栏 ⇒ rc=%d · stderr=%s", rc, jerr.String())
	}
	if !strings.Contains(jout.String(), `"criterion_new_state":"前瞻"`) {
		t.Errorf("机器面 `criterion_new_state` 要显式标「前瞻」：%s", jout.String())
	}
}

// ---- ★ GAP-20260927-265：判据「现树校验面」的**出口提示**（本枚）----

// TestDevProposalCriterionLiveTreeExitHint —— 强档 `--criterion` 的「现役命令树」校验面：
//
//	正控：合规判据（本版真跑得动）⇒ rc=0，且**逐字**落件（判据值不改写）。
//	负控①：本版**跑不动**的判据（新命令形态）⇒ 按既有档拒（rc=2）· 点名命令 · **指向前瞻档 `--criterion-new`**。
//	负控②：族级判据（`zerg dev` 是族名、不是可跑命令）⇒ 仍按既有档拒（rc=2）。
//
// 证明「拒执不再只点名、给出出口」这一格：负控① 的 stderr 必须含 `--criterion-new`。
func TestDevProposalCriterionLiveTreeExitHint(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	dir := t.TempDir()
	t.Setenv("ZERG_PROPOSAL_DIR", dir)

	// ---- 正控：合规判据 ⇒ rc=0，且逐字落件 ----
	rc, out, errb := runProposalNew(t, "--title", "现树校验正控", "--target", "待办:"+ids[0], "--goal", "g",
		"--evidence", proposalEvidenceInLedger, "--rollback", "r", "--criterion", testCriterion)
	if rc != 0 {
		t.Fatalf("合规判据 ⇒ rc=%d（要 0）· stderr=%s", rc, errb)
	}
	id := strings.TrimSpace(out)
	b, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		t.Fatalf("读落件 %s 失败：%v", id, err)
	}
	var rec map[string]any
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("落件 JSON 解不开：%v", err)
	}
	if got, _ := rec["criterion"].(string); got != testCriterion {
		t.Errorf("落件 criterion = %q（要 %q · 逐字）", got, testCriterion)
	}

	// ---- 负控①：本版跑不动的判据 ⇒ 按既有档拒 · 点名 · 指向前瞻档 ----
	const notInTree = "zerg gap backfill-future --dry-run"
	rc1, out1, err1 := runProposalNew(t, "--title", "现树校验负控一", "--target", "待办:"+ids[0], "--goal", "g",
		"--evidence", proposalEvidenceInLedger, "--rollback", "r", "--criterion", notInTree)
	if rc1 != 2 {
		t.Fatalf("不在命令树里的判据 ⇒ rc=%d（要 2 · 按既有档拒）· stderr=%s", rc1, err1)
	}
	if out1 != "" {
		t.Errorf("拒收时 stdout 必须 0 字节，实测 %d 字节", len(out1))
	}
	if !strings.Contains(err1, "不在命令树里") || !strings.Contains(err1, "backfill-future") {
		t.Errorf("负控① 要点名「不在命令树里」+ 逐字点名命令：%q", err1)
	}
	if !strings.Contains(err1, "--criterion-new") {
		t.Errorf("负控① 要**指向前瞻档** `--criterion-new`（GAP-20260927-265 的出口）：%q", err1)
	}

	// ---- 负控②：族级判据（族名不是可跑命令）⇒ 仍按既有档拒 ----
	rc2, out2, err2 := runProposalNew(t, "--title", "现树校验负控二", "--target", "待办:"+ids[0], "--goal", "g",
		"--evidence", proposalEvidenceInLedger, "--rollback", "r", "--criterion", "zerg dev")
	if rc2 != 2 {
		t.Fatalf("族级判据 ⇒ rc=%d（要 2 · 族名跑不动）· stderr=%s", rc2, err2)
	}
	if out2 != "" {
		t.Errorf("拒收时 stdout 必须 0 字节，实测 %d 字节", len(out2))
	}
	if !strings.Contains(err2, "不在命令树里") {
		t.Errorf("负控② 要点名「不在命令树里」：%q", err2)
	}
}

// ---- ★ GAP-20260927-267：`dev proposal revise <提案 id>` —— 提案的「改判据」动作（本枚）----

// runProposalRevise 起一次 `dev proposal revise`（**只走进程内 run**）。
func runProposalRevise(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(append([]string{"dev", "proposal", "revise"}, argv...), &out, &errb)
	return rc, out.String(), errb.String()
}

// readProposalCriterion 读提案件里的 criterion（「拒执零写 / 逐字落件」的取值面）。
func readProposalCriterion(t *testing.T, dir, id string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		t.Fatalf("读落件 %s 失败：%v", id, err)
	}
	var rec map[string]any
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("落件 JSON 解不开：%v", err)
	}
	s, _ := rec["criterion"].(string)
	return s
}

// TestDevProposalReviseCriterion —— 改判据动作（GAP-20260927-267）：
//
//	正控：合规判据就地改 ⇒ rc=0，且**逐字落件**（其余字段一字不动）。
//	负控①（非法/越界形态 ⇒ 按既有档拒并点名）：判据不在命令树 / 人话 / 一格都不给 / 提案不存在 —— 四条 rc=2，逐条点名。
//	负控②（既有档行为逐字不变）：`new` 正控仍 rc=0 · `show` 仍只读并读到改后的判据 · `check` 判据可跑仍 rc=0。
func TestDevProposalReviseCriterion(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	dir := t.TempDir()
	t.Setenv("ZERG_PROPOSAL_DIR", dir)

	// 先提一件（强档判据 = testCriterion），拿到 id。
	rc, out, errb := runProposalNew(t, "--title", "改判据", "--target", "待办:"+ids[0], "--goal", "g",
		"--evidence", proposalEvidenceInLedger, "--rollback", "r", "--criterion", testCriterion)
	if rc != 0 {
		t.Fatalf("正控件提不出来：rc=%d · stderr=%s", rc, errb)
	}
	id := strings.TrimSpace(out)
	if id == "" {
		t.Fatalf("成功时 stdout 要给提案 id")
	}

	// ---- 负控①a：非法形态（判据不在命令树）⇒ rc=2 · 按既有档拒 · 点名「不在命令树里」----
	const notInTree = "zerg gap backfill-future --dry-run"
	rc1, out1, err1 := runProposalRevise(t, id, "--criterion", notInTree, "--yes")
	if rc1 != 2 {
		t.Errorf("非法判据（不在命令树）⇒ rc=%d（要 2 · 按既有档拒）· stderr=%s", rc1, err1)
	}
	if out1 != "" {
		t.Errorf("拒收时 stdout 必须 0 字节，实测 %d 字节", len(out1))
	}
	if !strings.Contains(err1, "不在命令树里") {
		t.Errorf("负控①a 要点名「不在命令树里」：%q", err1)
	}
	if got := readProposalCriterion(t, dir, id); got != testCriterion {
		t.Errorf("拒执后盘上判据被改了 = %q（要 %q · 拒执零写）", got, testCriterion)
	}

	// ---- 负控①b：人话判据（越界形态）⇒ rc=2 · 点名「首词」----
	rc2, _, err2 := runProposalRevise(t, id, "--criterion", "看门禁全绿就行", "--yes")
	if rc2 != 2 || !strings.Contains(err2, "首词") {
		t.Errorf("人话判据 ⇒ rc=%d（要 2）、stderr 要点名首词：%q", rc2, err2)
	}

	// ---- 负控①c：一格都不给 ⇒ rc=2（没改任何东西）----
	rc3, _, err3 := runProposalRevise(t, id, "--yes")
	if rc3 != 2 || !strings.Contains(err3, "没给") {
		t.Errorf("不给判据 ⇒ rc=%d（要 2）、stderr 要点名「没给」：%q", rc3, err3)
	}

	// ---- 负控①d：提案不存在 ⇒ rc=2（同族写面 `dev edit` 同口径）----
	rc4, _, err4 := runProposalRevise(t, "DEV-9999-nope", "--criterion", testCriterion, "--yes")
	if rc4 != 2 || !strings.Contains(err4, "没有") {
		t.Errorf("提案不存在 ⇒ rc=%d（要 2）、stderr 要点名「没有」：%q", rc4, err4)
	}

	// ---- 正控：合规判据就地改 ⇒ rc=0，且逐字落件（其余字段一字不动）----
	const newCrit = "zerg gate run --fast"
	rc5, _, err5 := runProposalRevise(t, id, "--criterion", newCrit, "--yes")
	if rc5 != 0 {
		t.Fatalf("合规改判据 ⇒ rc=%d（要 0）· stderr=%s", rc5, err5)
	}
	if got := readProposalCriterion(t, dir, id); got != newCrit {
		t.Errorf("改后盘上判据 = %q（要 %q · 逐字）", got, newCrit)
	}
	b, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		t.Fatalf("读落件失败：%v", err)
	}
	var rec map[string]any
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("落件 JSON 解不开：%v", err)
	}
	if rec["id"] != id || rec["target"] != ids[0] || rec["goal"] != "g" || rec["rollback_ref"] != "r" {
		t.Errorf("改判据动了别的字段：id=%v target=%v goal=%v rollback=%v", rec["id"], rec["target"], rec["goal"], rec["rollback_ref"])
	}

	// ---- 负控②：既有档行为逐字不变 ----
	var so, se strings.Builder
	if rc := zerg.RunForTest([]string{"dev", "proposal", "show", id, "--json", "criterion"}, &so, &se); rc != 0 {
		t.Errorf("show 既有档 ⇒ rc=%d（要 0）· stderr=%s", rc, se.String())
	}
	if !strings.Contains(so.String(), newCrit) {
		t.Errorf("show 应读到改后的判据 %q：%s", newCrit, so.String())
	}
	var co, ce strings.Builder
	if rc := zerg.RunForTest([]string{"dev", "proposal", "check", id}, &co, &ce); rc != 0 {
		t.Errorf("check（判据可跑）⇒ rc=%d（要 0）· stderr=%s", rc, ce.String())
	}
}
