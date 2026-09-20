// family_reap_test.go —— 回收与残留（M19）判据的机检（层①进程内 · 外部测试包 `package main_test` ·
// 开工单 T-54）。
//
// 判据（开工单逐字）：
//
//	① `zerg agent reap --dry-run` 的清单带 `plan_id`、逐件带证据；
//	② **入库件（git-tracked）零命中**（`RC11` 红线）；
//	③ 回滚件到期前**一律档 ③**；
//	④ 三档回收权 × 五类差集逐条有判词；
//	⑤ 退役的「标记」放哪定案（`P-111`）。
package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// mkReapTree 造一棵合成的回收现场（旧件 / 安全网 / 新件三样齐）。
func mkReapTree(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	for _, sub := range []string{"bin/_history", "dist"} {
		if err := os.MkdirAll(filepath.Join(d, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"bin/zerg-core.bak":           "old",
		"bin/_history/zerg-core.prev": "safety-net",
		"dist/zerg-2.5.9.tar.gz.bak":  "olddist",
	}
	for rel, body := range files {
		if err := os.WriteFile(filepath.Join(d, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(d, "bin", "fresh.bak"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	return d
}

// ---- 判据①：干跑单带 plan_id + 逐件带证据；指纹稳定（同集两次一样）----

func TestReapDryRunHasPlanIDAndPerItemEvidence(t *testing.T) {
	root := mkReapTree(t)
	plan, err := zerg.BuildReapPlanForTest(root, 0, nil)
	if err != nil {
		t.Fatalf("造干跑单失败：%v", err)
	}
	if !strings.HasPrefix(plan.PlanID, "PLAN-") || len(plan.PlanID) != len("PLAN-")+12 {
		t.Errorf("plan_id 形态不对：%q（要 PLAN- + 12 位）", plan.PlanID)
	}
	if len(plan.Items) == 0 {
		t.Fatal("干跑单一件都没有（空转 = 假覆盖）")
	}
	for _, it := range plan.Items {
		for _, k := range []string{"tracked", "age_days", "bytes", "tier"} {
			if strings.TrimSpace(it.Evidence[k]) == "" {
				t.Errorf("%s 的证据缺 %s（判据①：逐件带证据）", it.Rel, k)
			}
		}
		if strings.TrimSpace(it.Judge) == "" {
			t.Errorf("%s 没有判词（判据④）", it.Rel)
		}
	}
	// 指纹稳定：同一集两次一样（`RC12` 的「对不上即拒」靠的就是这个稳定性）。
	a, b, err := zerg.ReapPlanIDOfSameSetForTest(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("同一集两次的 plan_id 不同：%s ≠ %s（那「对不上即拒」就没法用了）", a, b)
	}
	// 指纹有区分度：加一件 ⇒ 变（负控成对）。
	if err := os.WriteFile(filepath.Join(root, "bin", "extra.bak"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan2, _ := zerg.BuildReapPlanForTest(root, 0, nil)
	if plan2.PlanID == plan.PlanID {
		t.Error("负控失败：加了一件之后 plan_id 没变（指纹没区分度）")
	}
}

// ---- 判据②：`RC11` 红线 —— 入库件零命中（带 git 真夹具的端到端）----

func TestReapRedLineTrackedIsZero(t *testing.T) {
	root := mkReapTree(t)
	// ① 判定口：把 `bin/_history/zerg-core.prev` 声明成入库件 ⇒ 干跑单里它必须出现在 Tracked
	//    （**不是**「悄悄跳过」—— 判据②要的是零命中，而命中就要**整单拒**）。
	plan, err := zerg.BuildReapPlanForTest(root, 0, []string{"bin/_history/zerg-core.prev"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tracked) != 1 || plan.Tracked[0] != "bin/_history/zerg-core.prev" {
		t.Fatalf("红线命中 = %v（要那一件）", plan.Tracked)
	}
	for _, it := range plan.Items {
		if it.Rel == "bin/_history/zerg-core.prev" {
			t.Error("入库件出现在了候选（Items）里 —— 红线是「永不进候选」")
		}
	}
	// ② 端到端：真起一个小 git 仓（`git init` + `git add`），把那件真变成 tracked ⇒ 真跑 `ZERG_REPO`。
	repo := mkReapTree(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("本机没有 git ⇒ 这一格不给结论（不假装跑过）")
	}
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v 失败：%v / %s", args, err, out)
		}
	}
	run("init", "-q")
	run("add", "bin/_history/zerg-core.prev")
	t.Setenv("ZERG_REPO", repo)
	rc, out, errb := runCapture("agent", "reap", "--dry-run", "--min-age-days", "0")
	if rc != 2 {
		t.Fatalf("有入库件在候选里时 rc=%d（要 2 = 整单拒）· stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "bin/_history/zerg-core.prev") || !strings.Contains(errb, "永不进候选") {
		t.Errorf("拒单理由没点名那件 / 没引红线：stdout=%s stderr=%s", out, errb)
	}
	if out != "" {
		t.Errorf("整单拒时 stdout 必须 0 字节：%q", out)
	}
	// ③ 成对负控：把那一件从索引里拿掉 ⇒ 同一棵树就不再命中（证明判据真的在看 git，不是恒红）。
	run("rm", "--cached", "-q", "bin/_history/zerg-core.prev")
	rc, out, errb = runCapture("agent", "reap", "--dry-run", "--min-age-days", "0")
	if rc == 2 && strings.Contains(errb, "永不进候选") {
		t.Errorf("拿掉入库件后仍然按红线拒：stderr=%s", errb)
	}
	if !strings.Contains(out, "PLAN-") {
		t.Errorf("拿掉入库件后没出干跑单：%s", out)
	}
}

// ---- 判据③：回滚件到期前一律档 ③ ----

func TestReapSafetyNetIsAlwaysTierThree(t *testing.T) {
	root := mkReapTree(t)
	plan, err := zerg.BuildReapPlanForTest(root, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range plan.Items {
		if it.Rel != "bin/_history/zerg-core.prev" {
			continue
		}
		found = true
		if it.Tier != "3" {
			t.Errorf("安全网里的回滚件落到了档 %s（要档 3 —— P-105：回滚件到期前一律档 ③）", it.Tier)
		}
		if !strings.Contains(it.Judge, "永不自动") {
			t.Errorf("回滚件的判词没写「永不自动」：%s", it.Judge)
		}
	}
	if !found {
		t.Error("干跑单里没有安全网那件（判据③没被判到）")
	}
	// 负控成对：一般旧件（`bin/*.bak`）**不是**档 ③（否则「一律档③」这句话没有区分度）。
	for _, it := range plan.Items {
		if it.Rel == "bin/zerg-core.bak" && it.Tier != "2" {
			t.Errorf("一般换件残片落到了档 %s（要档 2 · 点名可动）", it.Tier)
		}
	}
}

// ---- 判据④：三档 × 五类差集逐条有判词；`RC1`–`RC14` 十四条齐 ----

func TestReapContractClosedSets(t *testing.T) {
	spec, err := zerg.ReapSpecOfForTest()
	if err != nil {
		t.Fatalf("回收真源读不出来：%v", err)
	}
	if len(spec.Tiers) != 3 {
		t.Errorf("档数 = %d（要三档）· %v", len(spec.Tiers), spec.Tiers)
	}
	if len(spec.DiffJudges) != 5 {
		t.Errorf("差集类数 = %d（要五类 `D-A`–`D-E` · `RC4` 不许新增第六个值）", len(spec.DiffJudges))
	}
	for _, id := range []string{"D-A", "D-B", "D-C", "D-D", "D-E"} {
		if strings.TrimSpace(spec.DiffJudges[id]) == "" {
			t.Errorf("%s 没有判词（判据④：逐条有判词）", id)
		}
	}
	if len(spec.RCRules) != 14 {
		t.Errorf("`RC*` 条数 = %d（要 14）", len(spec.RCRules))
	}
	if !strings.Contains(spec.RedLine, "永不进候选") {
		t.Errorf("红线那句话不在真源里：%q", spec.RedLine)
	}
	if strings.TrimSpace(spec.PlanIDRule) == "" {
		t.Error("`plan_id` 的口径不在真源里（`RC12`）")
	}
	// 判据⑤（`P-111`）：退役标记的落点**定案**必须在真源里，且点明「不建第二套账本」。
	if !strings.Contains(spec.RetiredPath, "retired.json") || !strings.Contains(spec.RetiredPath, "不建第二套账本") {
		t.Errorf("`P-111` 的定案没落进真源：%q", spec.RetiredPath)
	}
	// 清册类逐条有 tier + 判词（`RC3`：清册是闭集）。
	if len(spec.ObjectClass) == 0 {
		t.Error("对象清册是空的（`RC3`：清册是闭集）")
	}
}

// ---- `RC6`：龄阈值显式、不许魔数 ----

func TestReapAgeThresholdIsExplicit(t *testing.T) {
	root := mkReapTree(t)
	t.Setenv("ZERG_REPO", root)
	// 缺 `--min-age-days` ⇒ 2（detail 里点明「不许魔数」）。
	rc, out, errb := runCapture("agent", "reap", "--dry-run")
	if rc != 2 {
		t.Fatalf("缺龄阈值 rc=%d（要 2）· stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "不许写魔数") {
		t.Errorf("拒因没引 `RC6`：%s", errb)
	}
	if out != "" {
		t.Errorf("拒时 stdout 必须 0 字节：%q", out)
	}
	// 给了非整数 ⇒ 2。
	if rc, _, _ := runCapture("agent", "reap", "--dry-run", "--min-age-days", "三十"); rc != 2 {
		t.Errorf("坏龄阈值 rc=%d（要 2）", rc)
	}
	// 给足龄阈值 ⇒ 出单（新件落「跳过」，并按 `RC14` 配计数与体积）。
	rc, out, errb = runCapture("agent", "reap", "--dry-run", "--min-age-days", "30")
	if !strings.Contains(out, "PLAN-") {
		t.Fatalf("给了龄阈值却没出单：rc=%d stdout=%s stderr=%s", rc, out, errb)
	}
	if !strings.Contains(out, "跳过") || !strings.Contains(out, "RC14") {
		t.Errorf("跳过面没有配计数与体积（`RC14`）：%s", out)
	}
}
