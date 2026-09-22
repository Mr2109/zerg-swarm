package subtask

// plan_cache_statepath_test.go — 计划模板目录硬编码治理（批 B-1，2026-09-18）用例。
//
// 缺陷（口径同 api/tasks_persist.go 上一批 / agent 批 A）:
//
//	plan_cache.go: `const planTemplateDir = "/tmp/zerg-plan-templates"`（目录类落点）
//	/tmp = macOS 重启即清 + tmp_cleaner 3 天未访问即删（同类任务攒下的计划模板丢）；多实例共用同一目录。
//
// 修后契约（本文件逐条钉死）:
//  ① 写只写统一状态目录派生路径（statepath → ZERG_STATE_DIR → ~/.zerg/state/zerg-plan-templates），
//     新目录（含父目录）首次写入自动建；
//  ② 读入口新目录优先；旧 /tmp 目录存在 ⇒ 追加为**只读**入口（目录类口径：只读保留不搬）——
//     新旧同签名时命中新路径那份（新优先）；
//  ③ 新无旧有 ⇒ Find 仍能读到旧模板（迁移兼容），旧文件/目录不删、不改（sha256 + mtime 双证）；
//  ④ 两都无 ⇒ 空启动不报错、读不建目录、不凭空创建旧目录；
//  ⑤ 显式覆盖（测试隔离）⇒ 只走覆盖路径，不复旧目录（防测试读真机 /tmp）。
//
// 隔离纪律: 一律切包级变量（t.Cleanup 严格恢复）+ ZERG_STATE_DIR 指 temp——
// 绝不碰真机 /tmp/zerg-plan-templates。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// TestMain — 包级测试隔离（批 B-1 加）。
// 计划模板目录默认由 statepath 派生（ZERG_STATE_DIR → ~/.zerg/state/zerg-plan-templates）；
// 既有用例（TestSchedulerFullRun 等跑 done 任务会真入库）若不隔离，就会写真机状态目录，
// 而修前它们写的是真机 /tmp/zerg-plan-templates。统一把状态目录指到进程级临时目录，
// 保证**任何**用例都不碰真机 ~/.zerg/state 与 /tmp/zerg-plan-templates。
func TestMain(m *testing.M) {
	tmpState, err := os.MkdirTemp("", "zerg-subtask-test-state-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "建临时状态目录失败: %v\n", err)
		os.Exit(1)
	}
	_ = os.Setenv("ZERG_STATE_DIR", tmpState)
	code := m.Run()
	_ = os.RemoveAll(tmpState)
	os.Exit(code)
}

// planPastTime 旧文件内容的固定 mtime（2020-01-01T00:00:00Z）。
// 固定过去时间：若代码改写了旧物，mtime 必变（纳秒粒度，不会漏判）。
var planPastTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// ───────── 通用断言工具 ─────────

// setPlanTemplateDirs 切包级写/读路径（override, legacy），测完严格恢复。
func setPlanTemplateDirs(t *testing.T, override, legacy string) {
	t.Helper()
	oldOverride, oldLegacy := planTemplateDirOverride, legacyPlanTemplateDir
	planTemplateDirOverride, legacyPlanTemplateDir = override, legacy
	t.Cleanup(func() { planTemplateDirOverride, legacyPlanTemplateDir = oldOverride, oldLegacy })
}

func planFileSHA256(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读文件失败 %s: %v", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func planFileMtime(t *testing.T, path string) time.Time {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat 失败 %s: %v", path, err)
	}
	return st.ModTime()
}

// planDirNames 目录内的文件名（排序——旧目录没被写/删的硬证据）。
func planDirNames(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("列目录失败 %s: %v", dir, err)
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// planMustNotExist 断言路径不存在（只读不建目录/文件）。
func planMustNotExist(t *testing.T, path, why string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s：%s 不该存在", why, path)
	}
}

// planWithStepIDs 造一个最小可用计划（步骤 ID 可辨认——用于断言命中的是新还是旧）。
func planWithStepIDs(id string) *Plan {
	return &Plan{Steps: []Step{{ID: id, Goal: "写 " + id + ".txt", Produces: []string{id + ".txt"}}}}
}

// mintLegacyPlanTemplate 造一个「旧版本留下的计划模板目录」（真入库格式的一份模板，mtime 固定过去时刻）。
// 通过临时切覆盖路径 + 真实 SavePlanTemplate 写出——保证与线上旧文件同格式（不是手搓 JSON）。
func mintLegacyPlanTemplate(t *testing.T, desc string, p *Plan) (dir, file string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), planTemplateDirName)
	prev := planTemplateDirOverride
	planTemplateDirOverride = dir
	err := SavePlanTemplate(desc, p)
	planTemplateDirOverride = prev
	if err != nil {
		t.Fatalf("造旧计划模板失败: %v", err)
	}
	ents, rerr := os.ReadDir(dir)
	if rerr != nil || len(ents) != 1 {
		t.Fatalf("旧模板目录应恰好 1 份模板: %v %v", ents, rerr)
	}
	file = filepath.Join(dir, ents[0].Name())
	if err := os.Chtimes(file, planPastTime, planPastTime); err != nil {
		t.Fatalf("设旧模板 mtime 失败: %v", err)
	}
	return dir, file
}

// readTemplateStepID 读出目录下唯一模板文件里的首个步骤 ID（断言新写那份到底写了什么）。
func readTemplateStepID(t *testing.T, dir string) string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("列目录失败 %s: %v", dir, err)
	}
	jsons := 0
	var tpl PlanTemplate
	for _, e := range ents {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		jsons++
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("读模板失败: %v", err)
		}
		if err := json.Unmarshal(b, &tpl); err != nil {
			t.Fatalf("模板 JSON 非法: %v", err)
		}
	}
	if jsons != 1 {
		t.Fatalf("预期恰好 1 份模板，实得 %d", jsons)
	}
	var p Plan
	if err := json.Unmarshal([]byte(tpl.PlanJSON), &p); err != nil || len(p.Steps) == 0 {
		t.Fatalf("模板内计划不可解析: %v", err)
	}
	return p.Steps[0].ID
}

// ───────── ① 默认落点派生自状态目录（不再 /tmp）─────────

// TestPlanTemplateDir_DefaultDerivedFromStateDir_NotTmp
func TestPlanTemplateDir_DefaultDerivedFromStateDir_NotTmp(t *testing.T) {
	if planTemplateLegacyDefaultDir != "/tmp/zerg-plan-templates" {
		t.Fatalf("旧目录字面量应保持 /tmp/zerg-plan-templates（只读来源），实得 %q", planTemplateLegacyDefaultDir)
	}
	stateDir := filepath.Join(t.TempDir(), "custom-state")
	t.Setenv("ZERG_STATE_DIR", stateDir)
	setPlanTemplateDirs(t, "", filepath.Join(t.TempDir(), "legacy-plan-templates-missing"))

	want := filepath.Join(stateDir, planTemplateDirName)
	if got := planTemplateWriteDir(); got != want {
		t.Fatalf("写目录应由状态目录派生：want %s, got %s", want, got)
	}
	// ★ 2026-09-23 波B（设计-CI适配-v1.1 §五 第二批 #2 · §八 待拍 2 裁定）：删掉旧断言
	//   「不以 /tmp/ 开头」—— 它在 TMPDIR=/tmp（Linux runner）下恒假，是自伤而非判据；
	//   真命题已由上面那条 `got != want`（= 状态目录派生值）判过 ⇒ 不留第二条。
	if dirs := planTemplateReadDirs(); len(dirs) != 1 || dirs[0] != want {
		t.Fatalf("无旧目录时读入口只应有派生路径 %s，实得 %v", want, dirs)
	}
}

// ───────── ② 写只落新路径，旧目录一字不动 ─────────

// TestPlanTemplateDir_WriteOnlyToStateDir_LegacyUntouched
func TestPlanTemplateDir_WriteOnlyToStateDir_LegacyUntouched(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "nested", "state") // 目录不存在——验证首次自动建
	t.Setenv("ZERG_STATE_DIR", stateDir)
	desc := "在工作区创建 calc.go 实现 Add 函数。然后创建 calc_test.go 测试 Add。最后写报告 internal-task-report.md。"
	legacyDir, legacyFile := mintLegacyPlanTemplate(t, desc, planWithStepIDs("OLD"))
	setPlanTemplateDirs(t, "", legacyDir)
	sumBefore, mtimeBefore := planFileSHA256(t, legacyFile), planFileMtime(t, legacyFile)
	namesBefore := planDirNames(t, legacyDir)

	if err := SavePlanTemplate(desc, planWithStepIDs("NEW")); err != nil {
		t.Fatalf("入库失败: %v", err)
	}

	wantDir := filepath.Join(stateDir, planTemplateDirName)
	if st, err := os.Stat(wantDir); err != nil || !st.IsDir() {
		t.Fatalf("新目录（含父目录）应首次自动创建: %v", err)
	}
	if got := readTemplateStepID(t, wantDir); got != "NEW" {
		t.Fatalf("新写模板应落新目录且内容为新计划：stepID=%s", got)
	}
	// 旧目录：内容 sha256 + mtime + 名单三证不变（只读保留不搬）
	if got := planFileSHA256(t, legacyFile); got != sumBefore {
		t.Fatalf("旧模板文件内容被改了！%s → %s", sumBefore, got)
	}
	if got := planFileMtime(t, legacyFile); !got.Equal(mtimeBefore) {
		t.Fatalf("旧模板文件被改写了！mtime %v → %v", mtimeBefore, got)
	}
	if got := planDirNames(t, legacyDir); len(got) != len(namesBefore) || got[0] != namesBefore[0] {
		t.Fatalf("旧模板目录被写了（必须只读、不删不改）：%v → %v", namesBefore, got)
	}
	// 读入口: 新在前、旧在后（目录类只读保留口径）
	if dirs := planTemplateReadDirs(); len(dirs) != 2 || dirs[0] != wantDir || dirs[1] != legacyDir {
		t.Fatalf("读入口应为 [新 %s, 旧只读 %s]，实得 %v", wantDir, legacyDir, dirs)
	}
}

// ───────── ③ 新无旧有 ⇒ 读到旧（一次），旧不被改 ─────────

// TestPlanTemplate_NewMissing_LegacyReadOnce_Untouched
func TestPlanTemplate_NewMissing_LegacyReadOnce_Untouched(t *testing.T) {
	stateDir := t.TempDir() // 新目录尚不存在（首次）
	t.Setenv("ZERG_STATE_DIR", stateDir)
	desc := "在工作区创建 calc.go 实现 Add 函数。然后创建 calc_test.go 测试 Add。最后写报告 internal-task-report.md。"
	legacyDir, legacyFile := mintLegacyPlanTemplate(t, desc, planWithStepIDs("OLD"))
	setPlanTemplateDirs(t, "", legacyDir)
	sumBefore, mtimeBefore := planFileSHA256(t, legacyFile), planFileMtime(t, legacyFile)
	namesBefore := planDirNames(t, legacyDir)

	got, sim := FindPlanTemplate(desc)

	if got == nil || sim < 0.99 {
		t.Fatalf("新无旧有 ⇒ 应从旧只读入口命中：sim=%.2f plan=%+v", sim, got)
	}
	if got.Steps[0].ID != "OLD" {
		t.Fatalf("命中的应是旧模板里的计划：stepID=%s", got.Steps[0].ID)
	}
	planMustNotExist(t, filepath.Join(stateDir, planTemplateDirName), "只读检索不该建新目录")
	if got := planFileSHA256(t, legacyFile); got != sumBefore {
		t.Fatalf("旧模板文件内容被改了！%s → %s", sumBefore, got)
	}
	if got := planFileMtime(t, legacyFile); !got.Equal(mtimeBefore) {
		t.Fatalf("旧模板文件被改写了！mtime %v → %v", mtimeBefore, got)
	}
	if got := planDirNames(t, legacyDir); len(got) != len(namesBefore) || got[0] != namesBefore[0] {
		t.Fatalf("旧模板目录被写了（只读保留不搬）：%v → %v", namesBefore, got)
	}
}

// ───────── ④ 新旧同签名 ⇒ 新路径优先 ─────────

// TestPlanTemplate_NewPreferredOverLegacy_SameSignature
func TestPlanTemplate_NewPreferredOverLegacy_SameSignature(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	desc := "在工作区创建 calc.go 实现 Add 函数。然后创建 calc_test.go 测试 Add。最后写报告 internal-task-report.md。"
	legacyDir, legacyFile := mintLegacyPlanTemplate(t, desc, planWithStepIDs("OLD"))
	sumBefore, mtimeBefore := planFileSHA256(t, legacyFile), planFileMtime(t, legacyFile)
	namesBefore := planDirNames(t, legacyDir)

	// 新目录放同签名的另一份（同一 desc ⇒ 同关键词集合 ⇒ 同分），必须赢
	newDir := filepath.Join(stateDir, planTemplateDirName)
	setPlanTemplateDirs(t, newDir, legacyDir)
	if err := SavePlanTemplate(desc, planWithStepIDs("NEW")); err != nil {
		t.Fatalf("入库失败: %v", err)
	}
	setPlanTemplateDirs(t, "", legacyDir) // 回到生产口径（无显式覆盖）

	got, sim := FindPlanTemplate(desc)

	if got == nil || sim < 0.99 {
		t.Fatalf("新旧同签名应命中：sim=%.2f plan=%+v", sim, got)
	}
	if got.Steps[0].ID != "NEW" {
		t.Fatalf("新路径优先：应命中新目录那份（NEW），实得 stepID=%s", got.Steps[0].ID)
	}
	if got := planFileSHA256(t, legacyFile); got != sumBefore {
		t.Fatalf("旧模板文件内容被改了！%s → %s", sumBefore, got)
	}
	if got := planFileMtime(t, legacyFile); !got.Equal(mtimeBefore) {
		t.Fatalf("旧模板文件被改写了！mtime %v → %v", mtimeBefore, got)
	}
	if got := planDirNames(t, legacyDir); len(got) != len(namesBefore) || got[0] != namesBefore[0] {
		t.Fatalf("旧模板目录被写了：%v → %v", namesBefore, got)
	}
}

// ───────── ⑤ 两都无 ⇒ 空启动不报错，首次写入自动建（旧目录不凭空建）─────────

// TestPlanTemplate_NeitherExists_EmptyStartNoError_NewDirAutoCreated
func TestPlanTemplate_NeitherExists_EmptyStartNoError_NewDirAutoCreated(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "nested", "state") // 不存在
	t.Setenv("ZERG_STATE_DIR", stateDir)
	legacyDir := filepath.Join(t.TempDir(), "legacy-plan-templates") // 不存在
	setPlanTemplateDirs(t, "", legacyDir)

	if got, sim := FindPlanTemplate("配置 nginx 服务器反向代理并启用 https 证书"); got != nil || sim != 0 {
		t.Fatalf("两都无 ⇒ 空启动应返回 nil,0（不报错）：plan=%+v sim=%.2f", got, sim)
	}
	planMustNotExist(t, filepath.Join(stateDir, planTemplateDirName), "只读检索不该建目录")
	planMustNotExist(t, legacyDir, "无旧目录时不得凭空创建旧 /tmp 目录")

	desc := "在工作区创建 calc.go 实现 Add 函数。然后创建 calc_test.go 测试 Add。最后写报告 internal-task-report.md。"
	if err := SavePlanTemplate(desc, planWithStepIDs("A")); err != nil {
		t.Fatalf("空启动后首次入库失败: %v", err)
	}
	wantDir := filepath.Join(stateDir, planTemplateDirName)
	if got := readTemplateStepID(t, wantDir); got != "A" {
		t.Fatalf("首次写入应落派生新目录：stepID=%s", got)
	}
	planMustNotExist(t, legacyDir, "写只写新路径——旧目录永不写也不建")
}

// ───────── ⑥ 显式覆盖（测试隔离）⇒ 不复旧目录 ─────────

// TestPlanTemplate_ExplicitOverrideSkipsLegacy
func TestPlanTemplate_ExplicitOverrideSkipsLegacy(t *testing.T) {
	override := t.TempDir()
	desc := "在工作区创建 calc.go 实现 Add 函数。然后创建 calc_test.go 测试 Add。最后写报告 internal-task-report.md。"
	legacyDir, legacyFile := mintLegacyPlanTemplate(t, desc, planWithStepIDs("OLD"))
	setPlanTemplateDirs(t, override, legacyDir)
	sumBefore := planFileSHA256(t, legacyFile)
	namesBefore := planDirNames(t, legacyDir)

	if got := planTemplateWriteDir(); got != override {
		t.Fatalf("显式覆盖应生效：want %s, got %s", override, got)
	}
	if dirs := planTemplateReadDirs(); len(dirs) != 1 || dirs[0] != override {
		t.Fatalf("覆盖态读入口只应有覆盖路径，实得 %v", dirs)
	}
	if got, sim := FindPlanTemplate(desc); got != nil || sim != 0 {
		t.Fatalf("覆盖态不得复旧目录（防测试读真机 /tmp）：plan=%+v sim=%.2f", got, sim)
	}
	if err := SavePlanTemplate(desc, planWithStepIDs("OVR")); err != nil {
		t.Fatalf("覆盖态入库失败: %v", err)
	}
	if got := readTemplateStepID(t, override); got != "OVR" {
		t.Fatalf("覆盖态写入应落覆盖路径：stepID=%s", got)
	}
	if got := planFileSHA256(t, legacyFile); got != sumBefore {
		t.Fatalf("覆盖态旧模板文件不该被动：%s → %s", sumBefore, got)
	}
	if got := planDirNames(t, legacyDir); len(got) != len(namesBefore) || got[0] != namesBefore[0] {
		t.Fatalf("覆盖态旧模板目录被写了：%v → %v", namesBefore, got)
	}
}
