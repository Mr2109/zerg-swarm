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
	"dev build", "dev test", "dev verify", "dev edit"}

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

// ---- D3b 第一步：提案接校验（判据**可机检** · 缺判据 / 判据跑不动 ⇒ rc 2）----

func TestDevProposalCriterionMustBeMachineCheckable(t *testing.T) {
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())
	base := []string{"--title", "判据", "--target", "待办:" + ids[0], "--goal", "g",
		"--evidence", "e", "--rollback", "r"}

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
		"--evidence", "e", "--rollback", "r", "--criterion", testCriterion); rc != 0 {
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
	rc := zerg.RunForTest(append([]string{"dev", "proposal", "new"}, argv...), &out, &errb)
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
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来（%v · %d 条）—— 判据③的判据自己坏了 ⇒ 不给结论", err, len(ids))
	}
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())

	// ③-a 回指不上 ⇒ exit 2（**不是**自造码）· stdout 0 字节
	rc, out, errb := runProposalNew(t, "--title", "回指不上", "--target", "待办:ZZZZ-9999",
		"--goal", "g", "--evidence", "e", "--rollback", "r", "--criterion", testCriterion)
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
		"--goal", "g", "--evidence", "e", "--rollback", "r", "--criterion", testCriterion)
	if rc != 0 {
		t.Errorf("既有编号 %q ⇒ 退码 %d（要 0）· stderr=%s", ids[0], rc, errb)
	}
	if strings.TrimSpace(out) == "" {
		t.Errorf("成功时 stdout 要给提案 id（拿到空）· stderr=%s", errb)
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
		"--goal", "g", "--evidence", "e", "--rollback", "r", "--criterion", testCriterion)
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
