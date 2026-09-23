// cli_impact_backfill_test.go —— `B3` 实测回填闭环的判据（任务单-影响面实施-20260922 §三 `B3` 判据①–⑤ ·
// 拍板清单 §三 冲突⑧ 的拍死句 · 设计-变更影响面-v1.6 §5.2/§5.3/§5.5）。
//
// 落点在 `package main_test`（§九 M17「三层测试落点」的层①②）：`runCapture` 直接跑一条命令
// （`zerg.RunForTest` = 与 `main()` 同一个 `run` ⇒ 读的是**当前源码**的运行期行为，不是盘上旧制品），
// 判退码 + 机读行 + 件字节。
//
// 逐条判据落在哪一格（**每条都带成对负控** —— 先证明这台机器判得出红，再谈「全过」）：
//
//	判据① 数字可复算 ：同一份结果表 + 同 `head_sha` 两跑 ⇒ 三数逐字相同（E2E A 与 B 对拍）
//	判据② BLOCKED    ：`BLOCKED` 那条步**不进漏报**（判定口 + E2E D 双面）；`REPORT` 同族
//	判据③ 追加式      ：连跑两次 ⇒ 历史行**逐字节不变**（E2E B/C 真跑 + 件级判定口）
//	判据④ 十键        ：缺任一 ⇒ **该行不许写**（逐格负控）；键表从**契约件现读**，不另抄一份
//	判据⑤ 不可机检    ：本文件**不判**「这三个数好不好」；只判「口径/出处/边界」都在（照实标）
//	冲突⑧ 拍死句      ：静态 0 处 + 运行期改回填值 ⇒ 预测/排序/退码逐字不变 —— 两半都真跑
//	零副作用边界      ：干跑一个字节都不落 · 写失败**不改退码**（把落点造成目录 ⇒ 追加必失败）
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// ---- 夹具与判定口（合成件都在 t.TempDir() 里；**不碰真状态目录**）------------------------------

// gateRow 合成结果表的一行（脚本 `run_step` 的原形：`STATUS<TAB>步名<TAB>rc<TAB>耗时<TAB>日志`）。
type gateRow struct {
	Status string
	Name   string
}

// writeGateTable 把一行行写成一个 `results.tsv`（**5 列** —— 少一列就该走「取不到」那一档）。
func writeGateTable(t *testing.T, text string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "results.tsv")
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatalf("写合成结果表失败：%v", err)
	}
	return p
}

// tableText 把若干行拼成结果表原文（每行 5 列；rc/耗时/日志用占位值 —— 本件只读状态列与步名列）。
func tableText(rows []gateRow) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(r.Status + "\t" + r.Name + "\t0\t0s\t/tmp/" + strings.ReplaceAll(r.Name, " ", "_") + ".log\n")
	}
	return b.String()
}

// backfillSignalOf 从 stderr 里摘那枚机读行（`{"signal":"impact.backfill"…}`）—— 摘不到 ⇒ 直接 Fatal。
func backfillSignalOf(t *testing.T, errb string) map[string]any {
	t.Helper()
	for _, ln := range strings.Split(errb, "\n") {
		i := strings.Index(ln, `{"signal":"impact.backfill"`)
		if i < 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(ln[i:])), &m); err != nil {
			t.Fatalf("机读行不是合法 JSON：%v · 原文=%q", err, ln)
		}
		return m
	}
	t.Fatalf("stderr 里没有 `impact.backfill` 机读行（本判据的落点）：%s", tailOf(errb, 12))
	return nil
}

func tailOf(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// intOf 机读行里的一个整数格（JSON 里是 float64）。
func intOf(t *testing.T, m map[string]any, k string) int {
	t.Helper()
	v, ok := m[k]
	if !ok {
		t.Fatalf("机读行缺键 %q", k)
	}
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("机读行 %q 不是数：%#v", k, v)
	}
	return int(f)
}

func strsOf(t *testing.T, m map[string]any, k string) []string {
	t.Helper()
	v, ok := m[k]
	if !ok {
		t.Fatalf("机读行缺键 %q", k)
	}
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("机读行 %q 不是数组：%#v", k, v)
	}
	out := []string{}
	for _, x := range arr {
		s, _ := x.(string)
		out = append(out, s)
	}
	return out
}

// backfillState 每枚 E2E 用例一套**合成**状态目录（`impact-cache` 也落那儿 ⇒ 不碰真状态目录）。
func backfillState(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", d)
	t.Setenv("ZERG_REPO", repoRootFromCLI(t))
	return d
}

// backfillFilePath 落点（读契约件里的 `file` —— 测试不另写第二份文件名）。
func backfillFilePath(t *testing.T, root string) string {
	t.Helper()
	p, err := zerg.ImpactBackfillPathForTest(root)
	if err != nil {
		t.Fatalf("落点解析不出来（契约件读不到？）：%v", err)
	}
	return p
}

const backfillTarget = "core/cmd/zerg/main.go"

// ---- 判据②：三个数与 `BLOCKED` 规则（**纯判定口** · 不 LaTeX 起子进程）------------------------

// TestImpactBackfill_CountsAndBlockedRule —— 判据② 的判定口 + 三组成对负控：
//
//	① 正控：命中=预测红∩真红 · 漏报=真红\预测红 · 虚报=预测红\(真红∪BLOCKED∪REPORT)；
//	② `BLOCKED` **不进漏报**（把 `blocked` 清空 ⇒ 漏报一个字不变 —— 这一格就是判据② 的牙）；
//	③ `REPORT` 同族：预测了而落 REPORT ⇒ **不计虚报**（把它当「判绿」会凭空多出一枚虚报）；
//	④ 负控：喂一组「真红没被预测」⇒ 漏报必须 ≥1（否则这台机器判不出漏报）。
func TestImpactBackfill_CountsAndBlockedRule(t *testing.T) {
	predict := []string{"步甲", "步乙", "步丙", "步丁"}
	fail := []string{"步甲", "步戊"}
	blocked := []string{"步乙"}
	report := []string{"步丙"}

	hit, miss, fa, exclB, exclR := zerg.ImpactBackfillCountsOfForTest(predict, fail, blocked, report)
	if hit != 1 || miss != 1 || fa != 1 {
		t.Errorf("正控：要 命中=1 漏报=1 虚报=1，实得 hit=%d miss=%d fa=%d（步甲=真红且被预测；步戊=真红但没被预测 ⇒ 漏报；步丁=预测了但**判绿** ⇒ 虚报；步乙=预测了而落 BLOCKED · 步丙=预测了而落 REPORT ⇒ 两档都**不进三数**）",
			hit, miss, fa)
	}
	// 三数**两两**对得上两个集合：命中+漏报 = 真红条数 · 命中+虚报 = 预测条数 − 两个排除面。
	if hit+miss != len(fail) {
		t.Errorf("命中+漏报 要 = 真红条数 %d，实得 %d", len(fail), hit+miss)
	}
	if hit+fa != len(predict)-len(exclB)-len(exclR) {
		t.Errorf("命中+虚报 要 = 预测条数 − 排除面 %d，实得 %d", len(predict)-len(exclB)-len(exclR), hit+fa)
	}
	if len(exclB) != 1 || exclB[0] != "步乙" {
		t.Errorf("预测了而落 `BLOCKED` 的步要**逐条单列**（不进三数），实得 %v", exclB)
	}
	if len(exclR) != 1 || exclR[0] != "步丙" {
		t.Errorf("预测了而落 `REPORT` 的步要**逐条单列**，实得 %v", exclR)
	}

	// ② `BLOCKED` 不进漏报：把 blocked 清空 ⇒ 漏报**一个字不变**。
	_, miss2, _, _, _ := zerg.ImpactBackfillCountsOfForTest(predict, fail, nil, report)
	if miss2 != miss {
		t.Errorf("判据② 破：`BLOCKED` 一档影响到了漏报（%d → %d）—— 「`BLOCKED` 不计入漏报」是逐字判据", miss, miss2)
	}

	// ③ `REPORT` 同族：预测了而落 `REPORT` ⇒ 不计虚报（当「判绿」会凭空多一枚）。
	_, _, faR, _, _ := zerg.ImpactBackfillCountsOfForTest(predict, fail, blocked, nil)
	if faR != fa+1 {
		t.Errorf("`REPORT` 那一档没牙：把它从「不当判绿」改成「当判绿」应当**恰好多一枚虚报**（%d → %d），实得 %d", fa, fa+1, faR)
	}

	// ④ 负控：真红没被预测 ⇒ 漏报 ≥ 1（判据① 的「漏报=假阴」这一格必须判得出来）。
	_, miss3, _, _, _ := zerg.ImpactBackfillCountsOfForTest(nil, fail, blocked, report)
	if miss3 != len(fail) {
		t.Errorf("负控④：预测集为空 ⇒ 漏报要 = 真红条数 %d，实得 %d", len(fail), miss3)
	}
}

// ---- 判据④：十键（键表现读契约件）· 缺任一不写 · 追加式 ---------------------------------------

// TestImpactBackfill_TenKeysRefuseAndAppend —— 判据④ + 判据③ 的**件级**判定口：
//
//	① 键表从契约件现读，**至少八键**（§5.5 点名的十条逐字在）；
//	② 齐 ⇒ 写得进；**缺任一 ⇒ 一个字节都不写**（三条负控：`predict_red` / `head_sha` / `actual_fail`）；
//	③ 连追加两次 ⇒ **历史行逐字节不变**（`head -n <旧行数>` 的件级等价物）。
func TestImpactBackfill_TenKeysRefuseAndAppend(t *testing.T) {
	root := repoRootFromCLI(t)
	keys, err := zerg.ImpactBackfillKeysForTest(root)
	if err != nil {
		t.Fatalf("十键读不到（契约件 `%s`）：%v", "core/internal/contract/impact-backfill.json", err)
	}
	if len(keys) < 8 {
		t.Fatalf("§5.5 判据④ 要求**至少八键**，契约件只给了 %d 条：%v", len(keys), keys)
	}
	for _, want := range []string{"head_sha", "target", "predict_red", "actual_fail", "actual_blocked",
		"hit", "miss", "false_alarm", "gate_run_id", "at"} {
		found := false
		for _, k := range keys {
			if k == want {
				found = true
			}
		}
		if !found {
			t.Errorf("§5.5 逐字点名的键 %q 不在契约件的 `required_keys` 里：%v", want, keys)
		}
	}

	p := filepath.Join(t.TempDir(), "impact-backfill.jsonl")
	good, err := zerg.ImpactBackfillRecordJSONForTest("head123", backfillTarget,
		[]string{"步甲"}, []string{"步甲"}, []string{}, []string{}, "run-1@abcdef0123456789", "2026-09-22T00:00:00+08:00")
	if err != nil {
		t.Fatalf("拼记录失败：%v", err)
	}
	if missing, err := zerg.ImpactBackfillAppendForTest(p, good); err != nil || len(missing) != 0 {
		t.Fatalf("正控：十键齐却写不进去（missing=%v · err=%v）", missing, err)
	}
	first, err := os.ReadFile(p)
	if err != nil || len(first) == 0 {
		t.Fatalf("正控：写完读不回来（%v）", err)
	}

	// ② 三条负控：**缺任一 ⇒ 一个字节都不写**（且要点名缺的是哪一格）。
	for _, c := range []struct {
		name, rec string
		want      string
	}{
		{"predict_red 为 null", `{"head_sha":"h","target":"t","predict_red":null,"actual_fail":[],"actual_blocked":[],"gate_run_id":"g","at":"a"}`, "predict_red"},
		{"head_sha 空", `{"head_sha":"","target":"t","predict_red":[],"actual_fail":[],"actual_blocked":[],"gate_run_id":"g","at":"a"}`, "head_sha"},
		{"actual_fail 为 null", `{"head_sha":"h","target":"t","predict_red":[],"actual_fail":null,"actual_blocked":[],"gate_run_id":"g","at":"a"}`, "actual_fail"},
		{"gate_run_id 空", `{"head_sha":"h","target":"t","predict_red":[],"actual_fail":[],"actual_blocked":[],"gate_run_id":"","at":"a"}`, "gate_run_id"},
	} {
		missing, err := zerg.ImpactBackfillAppendForTest(p, c.rec)
		if err != nil {
			t.Errorf("负控（%s）：报错而不是「拒写」：%v", c.name, err)
			continue
		}
		if len(missing) == 0 {
			t.Errorf("负控（%s）：**缺 %s 竟写进去了** —— 判据④「缺任一 ⇒ 该行不许写」没牙", c.name, c.want)
			continue
		}
		got := false
		for _, m := range missing {
			if strings.HasPrefix(m, c.want) {
				got = true
			}
		}
		if !got {
			t.Errorf("负控（%s）：缺失清单要点名 %q，实得 %v", c.name, c.want, missing)
		}
	}
	after, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读不回来：%v", err)
	}
	if string(after) != string(first) {
		t.Errorf("负控面**改动了历史**：拒写的三次竟然动了件（前后字节不同）—— 「缺任一不许写」+「不改历史行」两条都要守")
	}

	// ③ 追加式：再写一枚 ⇒ 旧行**逐字节不变**（只多一行）。
	if missing, err := zerg.ImpactBackfillAppendForTest(p, good); err != nil || len(missing) != 0 {
		t.Fatalf("第二枚写不进去（missing=%v · err=%v）", missing, err)
	}
	two, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读不回来：%v", err)
	}
	if !strings.HasPrefix(string(two), string(first)) {
		t.Errorf("判据③ 破：第二枚追加之后**历史行变了**（`head -n <旧行数>` 要逐字节相同）")
	}
	if strings.Count(string(two), "\n") != 2 {
		t.Errorf("追加式：两次 ⇒ 两行，实得 %d 行", strings.Count(string(two), "\n"))
	}
	// 每行都要**键齐**（十键逐格在）—— 追加的行不许少键。
	for i, ln := range strings.Split(strings.TrimRight(string(two), "\n"), "\n") {
		var doc map[string]any
		if err := json.Unmarshal([]byte(ln), &doc); err != nil {
			t.Fatalf("第 %d 行不是 JSON：%v", i+1, err)
		}
		for _, k := range keys {
			if _, ok := doc[k]; !ok {
				t.Errorf("第 %d 行缺键 %q（§5.5 判据④）", i+1, k)
			}
		}
	}
}

// ---- 判据② 的读侧：结果表解析与「取不到」五档（不拿半份表当结论）--------------------------------

// TestImpactBackfill_GateTableReads —— 结果表读侧：
//
//	① 四档逐档归位（`FAIL` / `BLOCKED` / `REPORT` / `PASS`）；
//	② 五条「取不到」：没给 / 找不到 / 空表 / 少列 / 状态不在四档内；
//	③ `gate_run_id` = 目录基名@结果表 sha256 前 16（**同一份表 ⇒ 同一枚 id** ⇒ 判据① 的复算锚）。
func TestImpactBackfill_GateTableReads(t *testing.T) {
	root := repoRootFromCLI(t)
	text := tableText([]gateRow{
		{"FAIL", "步一"}, {"BLOCKED", "步二"}, {"REPORT", "步三"}, {"PASS", "步四"},
	})
	p := writeGateTable(t, text)
	status, reason, runID, sha, fail, blocked, report, pass := zerg.ImpactBackfillGateReadForTest(root, p)
	if status != "取值" {
		t.Fatalf("四档合成的表竟取不到：%s（%s）", status, reason)
	}
	if len(fail) != 1 || fail[0] != "步一" || len(blocked) != 1 || blocked[0] != "步二" ||
		len(report) != 1 || report[0] != "步三" || len(pass) != 1 || pass[0] != "步四" {
		t.Errorf("四档归位不对：fail=%v blocked=%v report=%v pass=%v", fail, blocked, report, pass)
	}
	if runID == "" || !strings.Contains(runID, "@") || len(sha) != 64 {
		t.Errorf("`gate_run_id` / 结果表 sha256 不对：runID=%q sha=%q", runID, sha)
	}
	// 同一份表 ⇒ 同一枚 id（第三者用同一件可复算）。
	_, _, runID2, _, _, _, _, _ := zerg.ImpactBackfillGateReadForTest(root, p)
	if runID2 != runID {
		t.Errorf("同一份结果表给了两个 `gate_run_id`（%q / %q）—— 判据① 的「可复算」就靠它", runID, runID2)
	}

	// ② 五条「取不到」—— **一条都不许读成「没有红」**。
	for _, c := range []struct{ name, arg, text string }{
		{"没给", "", ""},
		{"找不到", filepath.Join(t.TempDir(), "nope", "results.tsv"), ""},
		{"空表", "", "   \n"},
		{"少列（3 列）", "", "PASS\t步一\t0\n"},
		{"状态不在四档内", "", "GREEN\t步一\t0\t0s\t/tmp/x.log\n"},
	} {
		arg := c.arg
		if c.text != "" {
			arg = writeGateTable(t, c.text)
		}
		st, rs, _, _, _, _, _, _ := zerg.ImpactBackfillGateReadForTest(root, arg)
		if st != "取不到" {
			t.Errorf("「%s」要落**取不到**，实得 %s（理由：%s）", c.name, st, rs)
		}
		if strings.TrimSpace(rs) == "" {
			t.Errorf("「%s」取不到却**没写原因**（「读不到」必须点名）", c.name)
		}
	}
}

// ---- 红线自证：静态那一半（0 处）+ 判定口有牙的负控 --------------------------------------------

// TestImpactBackfill_StaticReadOnlyProof —— 冲突⑧ 的静态一半：
//
//	① 正控：预测/排序/裁条/退码四条路径的 7 件源件里，对回填件名与那两个碰件的函数名的引用 **0 处**；
//	② 负控：造一棵合成树，把那个名字塞进「退码路径」那一件 ⇒ 判定口**必须**报出来
//	  （否则「0 处」就是一句恒绿的话）。
func TestImpactBackfill_StaticReadOnlyProof(t *testing.T) {
	root := repoRootFromCLI(t)
	bf, err := zerg.ImpactBackfillPathForTest(root)
	if err != nil {
		t.Fatalf("落点读不出来：%v", err)
	}
	file := filepath.Base(bf)
	files, hits, unread := zerg.ImpactBackfillScanForTest(root, file)
	if files == 0 {
		t.Fatal("静态自证一件源件都没扫到（空转 = 假覆盖）")
	}
	if len(unread) > 0 {
		t.Errorf("有源件读不到 ⇒ 这一半**不给结论**（不是「0 处」）：%v", unread)
	}
	if len(hits) > 0 {
		t.Errorf("★ 冲突⑧ 静态一半破：预测/排序/裁条/退码四条路径里出现了对回填件的引用：%v", hits)
	}

	// ② 负控：合成树里塞一处引用 ⇒ 判定口必须报（合成树只借「件名」这一条判据面）。
	fake := t.TempDir()
	dir := filepath.Join(fake, "core", "cmd", "zerg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建合成树失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "family_impact.go"),
		[]byte("package main\n\n// 故意塞一处：读 "+file+"\n"), 0o644); err != nil {
		t.Fatalf("写合成件失败：%v", err)
	}
	_, hits2, _ := zerg.ImpactBackfillScanForTest(fake, file)
	if len(hits2) == 0 {
		t.Error("负控失败：合成树里**明明塞了引用**，静态判定口却说 0 处 —— 这台机器判不出越线（恒绿装置）")
	}
}

// ---- 源件自检：口径四条（不设阈值 / 敲门禁 / 改包封 / 追加式写口）--------------------------------

// TestImpactBackfill_SourceSelfCheck —— 源件自检（**判据⑤ 的照实标** 与四条红线的落点）：
//
//	① **不自设阈值**：实现里不许出现现成槽那条门槛的**数值**（要引就引契约件里的那一格）；
//	② **不进门禁**：实现里不许有往门禁脚本里挂步的写法（`add_step`）；
//	③ **不改包封**：实现里不许出现 `emitEnvelope*`；
//	④ **只追加**：写口是 `O_APPEND`，件里**没有** `O_TRUNC`（截断 = 改历史）；
//	⑤ **判据⑤ 照实标**：契约件原文里明写「不可机检 / 这三个数好不好判不了」与「不自设阈值」。
func TestImpactBackfill_SourceSelfCheck(t *testing.T) {
	root := repoRootFromCLI(t)
	src, err := zerg.ImpactBackfillSourceForTest()
	if err != nil {
		t.Fatalf("读实现件失败：%v", err)
	}
	if !strings.Contains(src, "O_APPEND") {
		t.Error("写口不是 `O_APPEND`（追加式是判据③ 的全部）")
	}
	if strings.Contains(src, "O_TRUNC") {
		t.Error("实现里有 `O_TRUNC` —— 截断就是改历史（红线：不许改历史行）")
	}
	// 判据面只看**代码那一半**：把注释行剥掉再判（本件的注释里要**逐字引**红线原话 —— 那是记录，不是实现）。
	code := stripComments(src)
	if strings.Contains(code, "emitEnvelope") {
		t.Error("实现（代码行）里出现了 `emitEnvelope` —— 六键包封是一个键都不许加（红线）")
	}
	if strings.Contains(code, "add_step") {
		t.Error("实现（代码行）里出现了 `add_step` —— 回填**不许进门禁**（红线）")
	}
	// ① 门槛的数值只许从契约件读：实现（代码行）里不许把 `10%` 写死（引现成槽 = 引那一格，不是抄它的数）。
	if strings.Contains(code, "10%") {
		t.Error("实现（代码行）里出现了 `10%` —— 阈值只许**引现成槽**（契约件 `slot.threshold`），不许抄进实现（红线：不许自设阈值）")
	}
	if !strings.Contains(code, "Slot") {
		t.Error("实现里没有引用契约件的 `slot` 那一格（「引现成槽」这一条就没有落点）")
	}
	ctext, err := zerg.ImpactBackfillContractTextForTest(root)
	if err != nil {
		t.Fatalf("读契约件失败：%v", err)
	}
	for _, want := range []string{"判不了", "不自设", "只被读", "不作任何算法的输入", "追加", "BLOCKED"} {
		if !strings.Contains(ctext, want) {
			t.Errorf("契约件原文里缺 %q（判据⑤ 与冲突⑧ 的边界要落在原文上）", want)
		}
	}
	// 现成槽那条门槛**逐字**在契约件里（引的是它，不是新造的度量）。
	slot, err := zerg.ImpactSlotLineForTest(root)
	if err != nil {
		t.Fatalf("读现成槽那一行失败：%v", err)
	}
	if !strings.Contains(slot, "假绿 = 0 · 假红 ≤ 10%") {
		t.Errorf("现成槽（`scripts/gates/check-slice.py` 第 7 行）逐字变了：%q —— 本件引的就是这一行", slot)
	}
	if !strings.Contains(ctext, "假绿 = 0 · 假红 ≤ 10%") {
		t.Error("契约件里没把现成槽的门槛**逐字**引下来（引出处要能对拍）")
	}
}

// ---- E2E：判据①③ + 干跑/真写边界 + 齐不齐（真跑 `zerg impact --gate-results`）------------------

// TestImpactBackfill_EndToEnd —— 判据①③ + 零副作用边界，逐条真跑（**每枚都是合成结果表 + 合成状态目录**）：
//
//	A 干跑（不给确认档）⇒ rc=0 · 三数出得来 · **回填件不在盘上**；
//	B 真写 ⇒ 落一行；三数与 A **逐字相同**（判据① 的「可复算」：同一份表 + 同 `head_sha`）；
//	C 再真写一次 ⇒ 两行，且**第一行逐字节不变**（判据③）；
//	D 换一份「真红/BLOCKED/REPORT 都不在预测集里」的表 ⇒ 漏报=1（只数 `FAIL`）· `BLOCKED` 那条**不在漏报里**；
//	E 不齐表（3 行）⇒ **退 8 不给结论**，且**不许写件**；
//	F 写失败不改退码（把落点造成目录）⇒ rc 仍 0。
func TestImpactBackfill_EndToEnd(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	root := repoRootFromCLI(t)
	state := backfillState(t)
	_ = state
	bf := backfillFilePath(t, root)

	fullTotal, fastTotal, fullSteps, fastSteps, st, rs := zerg.ImpactBackfillTruthForTest(root)
	if st != "取值" {
		t.Fatalf("真源现跑取不到（%s）⇒ 本用例的夹具造不出来", rs)
	}
	if fullTotal != len(fullSteps) || fastTotal != len(fastSteps) || fullTotal == 0 {
		t.Fatalf("真源两档自相矛盾：全量 %d/%d · 快速 %d/%d", fullTotal, len(fullSteps), fastTotal, len(fastSteps))
	}

	// 夹具一：全量档的表，**全 PASS**（⇒ 预测全落空：漏报 0 · 虚报 = 预测条数）。
	rows := make([]gateRow, 0, len(fullSteps))
	for _, nm := range fullSteps {
		rows = append(rows, gateRow{"PASS", nm})
	}
	allPass := writeGateTable(t, tableText(rows))

	// ---- A 干跑 ----
	rc, _, errb := runCapture("impact", backfillTarget, "--gate-results", allPass)
	if rc != 0 {
		t.Fatalf("A 干跑：要退 0（对拍跑通），实得 %d · stderr=%s", rc, tailOf(errb, 8))
	}
	sigA := backfillSignalOf(t, errb)
	if got := intOf(t, sigA, "steps_total"); got != fullTotal {
		t.Errorf("A：`steps_total` 要 = 真源全量档 %d，实得 %d", fullTotal, got)
	}
	if intOf(t, sigA, "hit") != 0 {
		t.Errorf("A：真红 0 条 ⇒ 命中要 0，实得 %d", intOf(t, sigA, "hit"))
	}
	if intOf(t, sigA, "miss") != 0 {
		t.Errorf("A：无 `FAIL` ⇒ 漏报要 0，实得 %d", intOf(t, sigA, "miss"))
	}
	predictA := strsOf(t, sigA, "predict_red")
	if intOf(t, sigA, "false_alarm") != len(predictA) {
		t.Errorf("A：全绿 ⇒ 虚报要 = 预测条数 %d，实得 %d", len(predictA), intOf(t, sigA, "false_alarm"))
	}
	if sigA["read_only_proof_ok"] != true || len(strsOf(t, sigA, "read_only_proof_hits")) != 0 {
		t.Errorf("A：静态自证要 ✓ 0 处，实得 ok=%v hits=%v", sigA["read_only_proof_ok"], sigA["read_only_proof_hits"])
	}
	if sigA["wrote"] != false {
		t.Error("A：**干跑不许写**（`wrote` 要 false）")
	}
	if _, err := os.Stat(bf); err == nil {
		t.Errorf("A：干跑之后回填件**竟然在盘上**（%s）—— 「干跑不写回填」这一条破", bf)
	}

	// ---- B 真写（三数与 A 逐字相同）----
	rcB, _, errbB := runCapture("impact", backfillTarget, "--gate-results", allPass,
		"--confirm="+hostForTest(), "--yes")
	if rcB != 0 {
		t.Fatalf("B 真写：要退 0，实得 %d · stderr=%s", rcB, tailOf(errbB, 8))
	}
	sigB := backfillSignalOf(t, errbB)
	if sigB["wrote"] != true {
		t.Errorf("B：真写要给 `wrote=true`，实得 %v（stderr 末 8 行：%s）", sigB["wrote"], tailOf(errbB, 8))
	}
	for _, k := range []string{"hit", "miss", "false_alarm"} {
		if intOf(t, sigB, k) != intOf(t, sigA, k) {
			t.Errorf("判据① 破（可复算）：同一份结果表 + 同 `head_sha`，两次跑的 %s 不同（%d vs %d）",
				k, intOf(t, sigA, k), intOf(t, sigB, k))
		}
	}
	first, err := os.ReadFile(bf)
	if err != nil || strings.Count(string(first), "\n") != 1 {
		t.Fatalf("B：真写之后回填件要恰一行（%v）", err)
	}

	// ---- C 再真写 ⇒ 历史行逐字节不变（判据③）----
	if rc, _, errb := runCapture("impact", backfillTarget, "--gate-results", allPass,
		"--confirm="+hostForTest(), "--yes"); rc != 0 {
		t.Fatalf("C：第二枚真写要退 0，实得 %d · %s", rc, tailOf(errb, 6))
	}
	two, err := os.ReadFile(bf)
	if err != nil {
		t.Fatalf("C：读不回来：%v", err)
	}
	if !strings.HasPrefix(string(two), string(first)) {
		t.Error("判据③ 破：连跑两次之后**历史行不是逐字节不变**（`head -n <旧行数>` 要相同）")
	}
	if strings.Count(string(two), "\n") != 2 {
		t.Errorf("C：两次 ⇒ 两行，实得 %d 行", strings.Count(string(two), "\n"))
	}

	// ---- D 换一份表：`FAIL` / `BLOCKED` / `REPORT` 都挑**不在预测集里**的步 ----
	inPredict := map[string]bool{}
	for _, s := range predictA {
		inPredict[s] = true
	}
	pick := []string{}
	for _, nm := range fullSteps {
		if !inPredict[nm] {
			pick = append(pick, nm)
		}
	}
	if len(pick) < 3 {
		t.Fatalf("D：可挑的「不在预测集里」的步不足 3 条（%d）—— 夹具不成立", len(pick))
	}
	odd := map[string]string{pick[0]: "FAIL", pick[1]: "BLOCKED", pick[2]: "REPORT"}
	rows2 := make([]gateRow, 0, len(fullSteps))
	for _, nm := range fullSteps {
		if s, ok := odd[nm]; ok {
			rows2 = append(rows2, gateRow{s, nm})
			continue
		}
		rows2 = append(rows2, gateRow{"PASS", nm})
	}
	oddTable := writeGateTable(t, tableText(rows2))
	rcD, _, errbD := runCapture("impact", backfillTarget, "--gate-results", oddTable)
	if rcD != 0 {
		t.Fatalf("D：要退 0，实得 %d · %s", rcD, tailOf(errbD, 8))
	}
	sigD := backfillSignalOf(t, errbD)
	if intOf(t, sigD, "miss") != 1 {
		t.Errorf("D：真红 1 条（且不在预测集里）⇒ 漏报要 1，实得 %d", intOf(t, sigD, "miss"))
	}
	if intOf(t, sigD, "hit") != 0 {
		t.Errorf("D：真红那条没被预测 ⇒ 命中要 0，实得 %d", intOf(t, sigD, "hit"))
	}
	blk := strsOf(t, sigD, "actual_blocked")
	if len(blk) != 1 || blk[0] != pick[1] {
		t.Errorf("D：`BLOCKED` 那条要**逐条单列**在 `actual_blocked` 里（实得 %v）", blk)
	}
	// 判据② 的硬判：`BLOCKED` 那条**不在漏报里** —— 漏报只数 `FAIL`（=1，不是 2）。
	if intOf(t, sigD, "miss") == 2 {
		t.Error("★ 判据② 破：`BLOCKED` 被混进漏报了（漏报 2 —— 只该数那 1 条 `FAIL`）")
	}
	if intOf(t, sigD, "false_alarm") != intOf(t, sigA, "false_alarm") {
		t.Errorf("D：预测侧没变 ⇒ 虚报不该变（%d → %d）", intOf(t, sigA, "false_alarm"), intOf(t, sigD, "false_alarm"))
	}
	if rc := intOf(t, sigD, "history_lines"); rc != 2 {
		t.Errorf("D：累积行数要看到前两枚（2），实得 %d", rc)
	}

	// ---- E 不齐表（3 行）⇒ 退 8 不给结论 + 不许写 ----
	subset := writeGateTable(t, tableText([]gateRow{{"PASS", fullSteps[0]}, {"PASS", fullSteps[1]}, {"PASS", fullSteps[2]}}))
	before, _ := os.ReadFile(bf)
	rcE, _, errbE := runCapture("impact", backfillTarget, "--gate-results", subset,
		"--confirm="+hostForTest(), "--yes")
	if rcE != 8 {
		t.Errorf("E：不齐（3 行 vs 全量 %d / 快速 %d）⇒ 要退 8 不给结论，实得 %d", fullTotal, fastTotal, rcE)
	}
	if !strings.Contains(errbE, "不齐") {
		t.Errorf("E：退 8 要**点名理由**（不齐），stderr 末 8 行：%s", tailOf(errbE, 8))
	}
	afterE, _ := os.ReadFile(bf)
	if string(afterE) != string(before) {
		t.Error("E：取不到的那一档**竟然写了件**（判据：取不到 ⇒ 不许写；宁缺不猜）")
	}

	// ---- F 写失败不改退码：把落点造成**目录** ⇒ 追加必失败，但 rc 仍 0 ----
	if err := os.Remove(bf); err != nil && !os.IsNotExist(err) {
		t.Fatalf("F：清不掉上一枚件：%v", err)
	}
	if err := os.MkdirAll(bf, 0o755); err != nil {
		t.Fatalf("F：造目录失败：%v", err)
	}
	rcF, _, errbF := runCapture("impact", backfillTarget, "--gate-results", allPass,
		"--confirm="+hostForTest(), "--yes")
	if rcF != 0 {
		t.Errorf("F：**写回填失败不得影响主流程退码**（要 0），实得 %d · %s", rcF, tailOf(errbF, 8))
	}
	if !strings.Contains(errbF, "追加失败") {
		t.Errorf("F：写失败要点名（「追加失败」），stderr 末 8 行：%s", tailOf(errbF, 8))
	}
}

// ---- 冲突⑧ 的运行期那一半：改回填值 ⇒ 预测 / 排序 / 退码逐字不变 ------------------------------

// TestImpactBackfill_NotAnInput —— 「回填件只被读，**不作任何算法的输入**」（冲突⑧）的运行期判据：
//
//	① 同一目标、同一仓：回填件**不在盘上** vs **塞一枚毒行**（命中=999/漏报=999/虚报=999）⇒
//	   预测面（`--json` 的四字段条目）与**退码**逐字不变；
//	② 回填块自己那三个数也**不受历史影响**（同一份结果表两跑，毒行前后逐字相同）——
//	   「累积率」只是报表，**不是输入**。
func TestImpactBackfill_NotAnInput(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	root := repoRootFromCLI(t)
	_ = backfillState(t)
	bf := backfillFilePath(t, root)

	rc1, out1, _ := runCapture("impact", backfillTarget, "--json", "what,why,how,red")
	if rc1 != 0 && rc1 != 1 {
		t.Fatalf("正控准备失败：`impact --json` 退码 %d（要 0/1）", rc1)
	}
	poison := `{"head_sha":"poison","target":"` + backfillTarget + `","predict_red":[],"actual_fail":[],` +
		`"actual_blocked":[],"hit":999,"miss":999,"false_alarm":999,"gate_run_id":"poison@0000000000000000","at":"x"}` + "\n"
	if err := os.WriteFile(bf, []byte(poison), 0o644); err != nil {
		t.Fatalf("塞毒行失败：%v", err)
	}
	rc2, out2, _ := runCapture("impact", backfillTarget, "--json", "what,why,how,red")
	if rc1 != rc2 || out1 != out2 {
		t.Errorf("★ 冲突⑧ 破：改了回填值之后**预测面/退码变了**（rc %d→%d · stdout 逐字相同=%v）—— "+
			"回填件只许被读、不许当输入", rc1, rc2, out1 == out2)
	}

	// ② 回填块自己的三数不受历史影响：同一份结果表、毒行前后逐字相同。
	fullTotal, _, fullSteps, _, st, rs := zerg.ImpactBackfillTruthForTest(root)
	if st != "取值" {
		t.Fatalf("真源现跑取不到（%s）", rs)
	}
	rows := make([]gateRow, 0, fullTotal)
	for _, nm := range fullSteps {
		rows = append(rows, gateRow{"PASS", nm})
	}
	tbl := writeGateTable(t, tableText(rows))
	_, _, errbP := runCapture("impact", backfillTarget, "--gate-results", tbl)
	sigPoison := backfillSignalOf(t, errbP)
	if err := os.WriteFile(bf, []byte(""), 0o644); err != nil {
		t.Fatalf("清毒行失败：%v", err)
	}
	_, _, errbC := runCapture("impact", backfillTarget, "--gate-results", tbl)
	sigClean := backfillSignalOf(t, errbC)
	for _, k := range []string{"hit", "miss", "false_alarm", "predict_red", "actual_fail", "actual_blocked"} {
		if !sameJSON(sigPoison[k], sigClean[k]) {
			t.Errorf("★ 冲突⑧ 破：历史值影响到了 %s（毒行 %v vs 空 %v）", k, sigPoison[k], sigClean[k])
		}
	}
	if intOf(t, sigClean, "history_lines") != 0 {
		t.Errorf("清空之后累积行数要 0，实得 %d", intOf(t, sigClean, "history_lines"))
	}
}

// sameJSON 两枚 `any` 的**逐字**比较（先规范化 JSON 文本 ⇒ 顺序无关的键序不影响判据）。
func sameJSON(a, b any) bool {
	ba, err1 := json.Marshal(normalizeJSON(a))
	bb, err2 := json.Marshal(normalizeJSON(b))
	return err1 == nil && err2 == nil && string(ba) == string(bb)
}

func normalizeJSON(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, 0, len(x))
		for _, e := range x {
			out = append(out, normalizeJSON(e))
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			out[k] = normalizeJSON(e)
		}
		return out
	default:
		return v
	}
}

// ---- 用法面：`--gate-results` 的收尾格（给了旗标没给值 ⇒ 退 2 · stdout 0 字节）------------------

// TestImpactBackfill_UsageFaces —— 新旗标面的**执行前判**（两条 must-fail 格，见 cli-matrix 的 `impact#…`）：
//
//	① 收尾给旗标不给值 ⇒ 退 2 · stdout **0 字节**（`flagValue` 看不见这一格 ⇒ 实现里另有一只判口）；
//	② 确认档值与主机名不符 ⇒ 退 2（与 `dev edit` 同一口径）；
//	③ 负控：值给了 ⇒ 这两条**不许**误伤（`impact <目标> --json what` 仍照常）。
func TestImpactBackfill_UsageFaces(t *testing.T) {
	requireDeep(t) // 贵档闸 · 现读见本件头：ZERG_DEEP=1 才跑
	_ = backfillState(t)
	rc, out, errb := runCapture("impact", backfillTarget, "--gate-results")
	if rc != 2 || len(out) != 0 {
		t.Errorf("① 收尾给旗标不给值 ⇒ 要退 2 + stdout 0 字节，实得 rc=%d · %d 字节 · stderr=%s",
			rc, len(out), tailOf(errb, 4))
	}
	if !strings.Contains(errb, "--gate-results") {
		t.Errorf("① 拒因文案要点名这一枚旗标，stderr=%s", tailOf(errb, 4))
	}
	rc, out, errb = runCapture("impact", backfillTarget, "--gate-results", "x", "--confirm=not-this-host", "--yes")
	if rc != 2 || len(out) != 0 {
		t.Errorf("② 确认值不匹配 ⇒ 要退 2 + stdout 0 字节，实得 rc=%d · %d 字节", rc, len(out))
	}

	// ③ 负控：不给这一族旗标 ⇒ 行为一字不变（`--json` 面照常出包封）。
	rc, out, _ = runCapture("impact", backfillTarget, "--json", "what")
	if rc != 0 || !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("③ 负控：没给 `--gate-results` 时 `impact --json` 应照常（rc=0 + 包封），实得 rc=%d · out=%q", rc, out)
	}

	// 旗标「给没给」的判定口本身也判一格（收尾那一格靠它）。
	if !zerg.ImpactBackfillFlagGivenForTest([]string{"impact", "x", "--gate-results"}) {
		t.Error("`--gate-results` 收尾那一格：判定口说「没给」—— 这一格就是②⑪ 那条必须判出来的用法错")
	}
	if zerg.ImpactBackfillFlagGivenForTest([]string{"impact", "x", "--json", "what"}) {
		t.Error("没给这一族旗标时判定口说「给了」—— 会把正常调用判成用法错")
	}
}

// stripComments 把 Go 源码的注释剥掉（**判据只看代码那一半**：注释里要逐字引红线原话 —— 那是记录，
// 不是实现）。做法：逐行砍掉 `//` 之后的内容（本件实现里没有字符串里含 `//` 的写法 ⇒ 够用、且不引解析库）。
func stripComments(src string) string {
	var b strings.Builder
	for _, ln := range strings.Split(src, "\n") {
		if i := strings.Index(ln, "//"); i >= 0 {
			ln = ln[:i]
		}
		b.WriteString(ln + "\n")
	}
	return b.String()
}

// hostForTest 本机主机名（`--confirm` 的值必须与它逐字相同 —— 与 `planHost()` 同一份真源）。
func hostForTest() string {
	if h := strings.TrimSpace(os.Getenv("ZERG_HOST")); h != "" {
		return h
	}
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}
