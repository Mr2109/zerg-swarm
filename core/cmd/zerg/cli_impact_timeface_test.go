// cli_impact_timeface_test.go —— `B2` 时间面（**预测侧**）的判据（任务单-影响面实施-20260922 §三 `B2`）。
//
// 判的是六件（**每件都配一枚反面** · 防自欺）：
//
//	① **预测口径写清**：预测集里每一条都带「怎么估的」（现算 / 投影 / 没跑 三态）与出处层 ——
//	  反面：没有闭集条目的那一跑，`not_run[]` 必须**逐条点名**（「预测为空」≠「没跑」）；
//	② **标定现跑重测 + 旧值并留**：真件上现跑 ⇒ 四格齐（探针条数 / 假绿 / 假红）；**在册旧值照留**
//	  （带出处与标定时刻）—— 反面：合成尺**四格不齐** ⇒ 必须「**取不到**」，**绝不许**出「假绿 0」；
//	③ **阈值来自现成槽 · 不自设**：输出里那一行必须写出处（`check-slice.py` 第 7 行 §3.5），
//	  且写死「波纹自己不设任何阈值」—— 反面：实现源码里**不许**出现本件自设的门槛赋值（源件自检）；
//	④ **探针自证成对**：`--selftest` 的**前置缺件闸正反两半**（`(e)` 正 / `(f)` 反）缺一 ⇒ 取不到
//	  —— 反面：合成尺只给半边 ⇒ 必须「取不到」（只跑半边不算成对）；
//	⑤ **不抢 `B3` 的实测回填面**：输出里**不许**出现实测三个整数 / `impact_actual` / 回填件；
//	  状态只有「预测」—— 反面：跑完状态目录里**不许**多出回填/审计件；
//	⑥ **与 `B1` 同一预算**（§4.4）：默认档**一次子进程都不起**（`ProbeRan=false` · 明写「未机检 + 怎么拉」），
//	  按需档才起 —— 反面：默认档输出里**不许**出现任何「【现跑】」读数（不许把没跑的数编出来）。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// timefaceFixtureScript —— **合成尺**（只为判据用：`--probe` 与 `--selftest` 两条只读入口）。
// `ZERG_B2_FIXTURE_MODE` 换出三种局面：齐（正控）/ 不齐（四格缺一）/ 半边成对（只给 `(e)`）。
// 它与真件**同形**（同一批行首），但条数与数值都是夹具自己的 ⇒ 「写常量」在这里会露出来。
const timefaceFixtureScript = `import os, sys
mode = os.environ.get("ZERG_B2_FIXTURE_MODE", "ok")
if sys.argv[1:] == ["--probe"]:
    print("── 混淆矩阵（夹具）──")
    if mode == "unbalanced":
        print("合计：探针 7 条（期望红 4 · 期望绿 3 · 期望需递归 0 · 期望错误 0）")
        print("真阳 TP（期望红·判红）= 3 ｜ 假阴 FN/假绿（期望红·未判红）= 0")
        print("真阴 TN（期望绿·判绿）= 3 ｜ 假阳 FP/假红（期望绿·判红）= 0")
        print("假绿率 = 0 ✓")
        print("假红率 = 0.0% ✓")
    else:
        print("合计：探针 7 条（期望红 4 · 期望绿 3 · 期望需递归 0 · 期望错误 0）")
        print("真阳 TP（期望红·判红）= 4 ｜ 假阴 FN/假绿（期望红·未判红）= 0")
        print("真阴 TN（期望绿·判绿）= 3 ｜ 假阳 FP/假红（期望绿·判红）= 0")
        print("假绿率 = 0 ✓")
        print("假红率 = 0.0% ✓")
    print("结论：探针集 过门槛（假绿 0 / 假红在限内）；rc = 0")
elif sys.argv[1:] == ["--selftest"]:
    print("✓ (e) 前置缺件闸·正：夹具正半（实得 rc=0）")
    if mode != "halfpair":
        print("✓ (f) 前置缺件闸·反：夹具反半（实得 rc=2）")
    print("自证结论：全过（用例 2 条，失败 0 条）")
else:
    sys.stderr.write("夹具只认 --probe / --selftest\n")
    sys.exit(2)
`

// timefaceFixtureRoot 造一棵**合成仓**（只放一把合成尺 ⇒ 判据全在这一件上）。
func timefaceFixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		t.Fatalf("合成目录造不出来：%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "gates", "check-slice.py"),
		[]byte(timefaceFixtureScript), 0o644); err != nil {
		t.Fatalf("合成尺写不出来：%v", err)
	}
	return root
}

// TestImpactTimeface_CalibrationFixturePairedControls —— **合成尺**上把判据②③④ 的正反面判掉。
//
// 正控：四格齐 + 成对闸齐 ⇒ **取值**，且报出来的条数是**夹具自己的 7**（不是真件那个 36）。
// 三枚负控（一一对应三条禁令）：
//
//	· 四格不齐（真阳 3 + 假阴 1 ≠ 期望红 4 —— 夹具故意留一处）⇒ 必须**取不到**；
//	· `--selftest` 只给半边 ⇒ 必须**取不到**（**只跑半边不算成对**）；
//	· 默认档 ⇒ 必须**未机检**，且 `ProbeRan=false`（**一次子进程都不起** —— 与 `B1` 同一预算）。
func TestImpactTimeface_CalibrationFixturePairedControls(t *testing.T) {
	root := timefaceFixtureRoot(t)

	// 正控：齐 ⇒ 取值 + 条数随**夹具**（7 条 ⇒ 「写常量」会在这里露出来）。
	t.Setenv("ZERG_B2_FIXTURE_MODE", "ok")
	ok := zerg.ImpactCalibrationOfForTest(root, true)
	if ok.Status != "取值" {
		t.Fatalf("合成尺齐的时候该「取值」，实得 status=%s · reason=%s", ok.Status, ok.Reason)
	}
	if !ok.ProbeRan || len(ok.Runs) != 2 {
		t.Errorf("按需档该真跑两条探针，实得 ProbeRan=%t · runs=%v", ok.ProbeRan, ok.Runs)
	}
	joined := strings.Join(ok.Live, "｜")
	if !strings.Contains(joined, "7 条") {
		t.Errorf("判据②：探针条数该**随尺自报**（夹具是 7 条），实得 %q —— 写常量就会在这里露出来", joined)
	}
	if !strings.Contains(joined, "假绿（期望红 · 未判红）=0（真阳 4 / 假阴 0）") {
		t.Errorf("判据②：四格该按现跑摘出来，实得 %q", joined)
	}
	if len(ok.Paired) != 2 {
		t.Errorf("判据④：成对闸该摘到两半（(e) 正 / (f) 反），实得 %v", ok.Paired)
	}
	if len(ok.Old) == 0 {
		t.Error("判据②：「在册旧值并留」那一份**一条都没有** ⇒ 新旧没法并排比")
	}

	// 负控一：四格不齐 ⇒ **取不到**（不许拿部分当全部 · 更不许出「假绿 0」）。
	t.Setenv("ZERG_B2_FIXTURE_MODE", "unbalanced")
	bad := zerg.ImpactCalibrationOfForTest(root, true)
	if bad.Status != "取不到" {
		t.Fatalf("四格不齐该「取不到」，实得 status=%s · live=%v", bad.Status, bad.Live)
	}
	if len(bad.Live) != 0 {
		t.Errorf("取不到的那一跑**一条现跑数都不许出**，实得 %v", bad.Live)
	}
	if !strings.Contains(bad.Reason, "不齐") {
		t.Errorf("取不到的理由该点名「不齐」，实得 %q", bad.Reason)
	}

	// 负控二：成对闸只给半边 ⇒ **取不到**。
	t.Setenv("ZERG_B2_FIXTURE_MODE", "halfpair")
	half := zerg.ImpactCalibrationOfForTest(root, true)
	if half.Status != "取不到" || !strings.Contains(half.Reason, "成对") {
		t.Errorf("只给半边的成对闸该「取不到」且理由点名「成对」，实得 status=%s · reason=%q", half.Status, half.Reason)
	}

	// 负控三：默认档 ⇒ 未机检 + 不起子进程（**与 `B1` 同一预算**）。
	t.Setenv("ZERG_B2_FIXTURE_MODE", "ok")
	cheap := zerg.ImpactCalibrationOfForTest(root, false)
	if cheap.Status != "未机检" {
		t.Errorf("默认档该「未机检」，实得 %s", cheap.Status)
	}
	if cheap.ProbeRan {
		t.Error("默认档**一次子进程都不许起**（§4.4 与 `B1` 同一预算）—— 本跑却起了")
	}
	if !strings.Contains(cheap.Reason, "--all") {
		t.Errorf("默认档该给出怎么拉（`--all`），实得 %q", cheap.Reason)
	}
}

// TestImpactTimeface_LiveCalibrationAndPrediction —— 真件上的现跑重测 + 预测面（人面与 stderr 块）。
//
// 三跑（判不同的格）：
//
//	① 默认档 `impact S-g`：**不起子进程** ⇒ 时间面块里尺那一格写「未机检」，且**不许**出现任何
//	  「【现跑】」读数（负控：不许把没跑的数编出来）；
//	② 按需档 `impact S-g --all`：预测集里出现契约 `S-g`（③ 层现读）+ 门步（`B1` 的投影）；
//	  尺那一格出**现跑读数**（探针条数由脚本自报）+ 自证成对两半；
//	③ 同一口令两跑：**机读那一行逐字相同**（`M8`：定序 · 无时钟无耗时）+ 状态目录不多件。
func TestImpactTimeface_LiveCalibrationAndPrediction(t *testing.T) {
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)

	// ① 默认档。
	rc, out, errb := runCapture("impact", "S-g")
	if rc != 0 {
		t.Fatalf("`impact S-g`（默认档）该退 0，实得 %d · stderr=%s", rc, errb)
	}
	if err := judgeImpactHuman(out, impactHeads); err != nil {
		t.Errorf("人面仍须恒三行：%v · stdout=%q", err, out)
	}
	if !strings.Contains(errb, "时间面") || !strings.Contains(errb, "预测侧") {
		t.Errorf("stderr 该有时间面（预测侧）那一块：%s", errb)
	}
	if !strings.Contains(errb, "未机检") {
		t.Errorf("默认档该写「未机检」（尺的标定属按需档项）：%s", errb)
	}
	if strings.Contains(errb, "【现跑】") {
		t.Errorf("判据⑥（负控）：默认档**不许**出现任何「【现跑】」读数：%s", errb)
	}
	if !strings.Contains(errb, "本批不做") || !strings.Contains(errb, "B3") {
		t.Errorf("判据⑤：实测侧必须明写「本批不做 · 属 B3」：%s", errb)
	}

	// ② 按需档。
	rc, out, errb = runCapture("impact", "S-g", "--all")
	if rc != 0 {
		t.Fatalf("`impact S-g --all` 该退 0，实得 %d · stderr=%s", rc, errb)
	}
	for _, want := range []string{
		"时间面", "预测集 `predict_red[]`", "怎么估的", "估法口径",
		"现跑重测", "【现跑】", "自证成对", "前置缺件闸·正", "前置缺件闸·反",
		"不许小额度探针", "阈值（**谁设的**）", "波纹自己不设任何阈值",
		"实测侧", "本批不做", "零副作用",
	} {
		if !strings.Contains(errb, want) {
			t.Errorf("时间面块里没有 %q", want)
		}
	}
	if !strings.Contains(errb, "[契约] S-g") {
		t.Errorf("判据①：预测集里该出现契约 S-g（③ 层现读 · `change_class=B`）—— 逐条带「怎么估的」：%s", errb)
	}
	if strings.Contains(errb, "impact_actual\":") || strings.Contains(errb, "hit / miss / false_alarm") {
		t.Errorf("判据⑤：本件**不许**落实测值，实得输出里出现了实测面：%s", errb)
	}

	// ③ 两跑：机读那一行逐字相同（定序 · 无时钟无耗时）。
	_, _, errb2 := runCapture("impact", "S-g", "--all")
	j1, j2 := predictJSONLine(errb), predictJSONLine(errb2)
	if j1 == "" || j2 == "" {
		t.Fatalf("stderr 里该有机读那一行（`\"signal\":\"impact.predict\"`）：%q", errb2)
	}
	if j1 != j2 {
		t.Errorf("同一目标两跑机读行不逐字相同（`M8`）：\n① %s\n② %s", j1, j2)
	}
	if strings.Contains(j1, "2026-") || strings.Contains(j1, "耗时") {
		t.Errorf("机读行里**不许**有时钟/耗时（否则两跑逐字相同这条判据就是假的）：%s", j1)
	}
	assertNoAuditWrites(t, state)
}

// predictJSONLine 从 stderr 里摘那一行机读（前缀固定 ⇒ 摘法不用猜）。
func predictJSONLine(errb string) string {
	for _, ln := range strings.Split(errb, "\n") {
		if i := strings.Index(ln, "{\"signal\":\"impact.predict\""); i >= 0 {
			return ln[i:]
		}
	}
	return ""
}

// TestImpactTimeface_PredictionCaliberPairedControls —— 预测面本身：口径三态 + **空预测 ≠ 没跑**。
//
//	· 合成件/真件上「有闭集条目」⇒ 逐条带 `how`（怎么估的）与出处层；
//	· **零命中那一跑**（已知无关件）⇒ 预测集空，但 `not_run[]` 必须**逐条点名**（不许只留一个空）——
//	  这一对正是判据① 的反面：**「预测为空」与「没跑」是两件事**。
func TestImpactTimeface_PredictionCaliberPairedControls(t *testing.T) {
	root := repoRootFromCLI(t)
	t.Setenv("ZERG_REPO", root)

	// 正控：契约态目标 ⇒ 至少一条契约 id + 口径三态齐。
	v, err := zerg.ImpactTimefaceOfForTest(root, "S-g", true)
	if err != nil {
		t.Fatalf("预测面算不出来：%v", err)
	}
	if len(v.Items) == 0 {
		t.Fatalf("`S-g` 该有闭集条目（③ 层 `change_class=B`），实得 0 条 · not_run=%v", v.NotRun)
	}
	if !strings.Contains(strings.Join(v.Items, "\n"), "[契约] S-g") {
		t.Errorf("契约 id 那一条该在预测集里，实得 %v", v.Items)
	}
	est := strings.Join(v.EstCalib, "\n")
	for _, want := range []string{"**现算**", "**投影**", "**没跑**"} {
		if !strings.Contains(est, want) {
			t.Errorf("判据①：估法口径三态缺 %s：%s", want, est)
		}
	}
	if v.HeadSHA == "" {
		t.Error("口径三件套缺 `head_sha` ⇒ 这一块只许当参考（§7.4）")
	}
	if !strings.Contains(v.JSONLine, "\"actual_side\":\"B3（本件不落实测值）\"") {
		t.Errorf("机读行该写明实测侧归属 `B3`：%s", v.JSONLine)
	}
	if !strings.Contains(v.JSONLine, "\"threshold_from\"") {
		t.Errorf("机读行该带阈值出处那一格（**自设阈值**在这里会被看穿）：%s", v.JSONLine)
	}

	// 反面：零命中的件 ⇒ 空预测 + **逐条点名的 not_run[]**（「预测为空」≠「没跑」）。
	empty, err := zerg.ImpactTimefaceOfForTest(root, "docs/README.md", true)
	if err != nil {
		t.Fatalf("零命中目标算不出来：%v", err)
	}
	if len(empty.Items) != 0 {
		t.Errorf("这一件不该有闭集条目，实得 %v", empty.Items)
	}
	if len(empty.NotRun) == 0 {
		t.Error("判据①（反面）：预测为空时 `not_run[]` **必须逐条点名**（不许留空 —— 那就是「把没跑写成没有」）")
	}

	// 默认档：预测面照样出（口径与出处不依赖档位），但门步那一格是「没跑」。
	cheap, err := zerg.ImpactTimefaceOfForTest(root, "S-g", false)
	if err != nil {
		t.Fatalf("默认档预测面算不出来：%v", err)
	}
	if !strings.Contains(strings.Join(cheap.NotRun, "\n"), "没跑") &&
		!strings.Contains(strings.Join(cheap.NotRun, "\n"), "未机检") {
		t.Errorf("默认档该在 `not_run[]` 里点名「没跑/未机检」的面，实得 %v", cheap.NotRun)
	}
	if cheap.Calib.Status != "未机检" || cheap.Calib.ProbeRan {
		t.Errorf("默认档的尺那一格该「未机检」且不起子进程，实得 %s · ProbeRan=%t", cheap.Calib.Status, cheap.Calib.ProbeRan)
	}
}

// TestImpactTimeface_NoThresholdAndNoBackfillInSource —— 源件自检（两条**禁令**的可判形态）：
//
//	① **不设阈值**：本件的门槛那一格**只引用**尺自陈的口径 ⇒ 源码里不许出现「本件自己判红绿」的赋值
//	  （判法：源码里所有 `threshold`/阈值相关的**赋值**为零 · 门槛只以字符串出处出现）；
//	② **不抢 `B3`**：源码里不许出现回填/实测面的写入（`impact-backfill` / `jsonl` 写入 / `impact_actual` 赋值）。
func TestImpactTimeface_NoThresholdAndNoBackfillInSource(t *testing.T) {
	src, err := zerg.ImpactTimefaceSourceForTest()
	if err != nil {
		t.Fatalf("读实现件源码失败：%v", err)
	}
	for _, banned := range []string{
		"impact-backfill", "impact_actual\"", "WriteFile(", "Create(", "os.MkdirAll",
		"impactTimefaceThreshold", "hit :=", "miss :=", "false_alarm",
	} {
		if strings.Contains(src, banned) {
			t.Errorf("判据③/⑤：实现件里不该出现 %q（本件不设阈值、不落实测、不写任何件）", banned)
		}
	}
	if !strings.Contains(src, "check-slice.py") {
		t.Error("判据③：阈值那一格必须**指名出处**（`scripts/gates/check-slice.py` 第 7 行 §3.5）")
	}
	if !strings.Contains(src, "假绿 = 0 · 假红 ≤ 10%") {
		t.Error("判据③：现成槽自陈的门槛那一条要逐字出现（引它，不是另立一条）")
	}
}
