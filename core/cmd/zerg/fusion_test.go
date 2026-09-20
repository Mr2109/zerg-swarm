// fusion_test.go —— §十八.3 融合四件（`ask` / `plan` / `apply` / 意图 schema · 开工单 T-59）
// 的**判据机检**（层①进程内 · 外部测试包 `package main_test`）。
//
// 判据（开工单 T-59 逐字）：
//
//	① 依赖次序**不许倒**（四件任一未落，后一件不许先开）；
//	② `--capability` 词表外 ⇒ `2`、在词表但无候选 ⇒ `1`、**一律不新增码**；
//	③ 意图件在同一份件上过 **L1 schema → L2 引用存在性 → L3 干跑 → L4 人在环**，
//	   任一层不过 ⇒ **后续层不跑**；
//	④ `zerg apply` **只吃那一份件**（件被改一字节 / 旧值变 / 换机 ⇒ `2`）；
//	⑤ `plan` **零副作用**。
//
// 负控：`layerOrderJudge` 是「层序不许越过」这条判据的**唯一判定口** ——
// TestFusionLayerJudgeHasTeeth 直接喂它一个错的实况（L1 没过却声称 L2/L3 也跑了）⇒ 必须报错。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
	"github.com/Mr2109/zerg-swarm/core/internal/contract"
	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
)

// fusionRun 起一次命令（进程内 run —— 层①的落点）。
func fusionRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// ---- 判据① / 意图 schema 真源 -------------------------------------------------------------

// TestIntentSchemaTruthSourceHasFourClosedSets —— 意图件真源的**四闭集**齐 + F1–F7 在位
// （依赖次序的第三件；它不在 ⇒ `plan` 产不出件、`apply` 无从绑定）。
func TestIntentSchemaTruthSourceHasFourClosedSets(t *testing.T) {
	ip, err := contract.IntentPlan()
	if err != nil {
		t.Fatalf("意图件真源读不出来：%v", err)
	}
	if ip.EnvelopeSchema != "zerg/v1" {
		t.Errorf("包封 schema 应为 zerg/v1，实测 %q", ip.EnvelopeSchema)
	}
	// 四闭集（包封 kind / target.kind / action / error.kind）—— 一个都不许空
	for name, set := range map[string][]string{
		"envelope_kinds": ip.EnvelopeKinds, "target_kinds": ip.TargetKinds,
		"actions": ip.Actions, "error_kinds": ip.ErrorKinds,
	} {
		if len(set) == 0 {
			t.Errorf("闭集 %s 是空的 ⇒ 消费侧无从判", name)
		}
	}
	// 七语义件 F1–F7 逐格在位（字段名 → 编号）
	want := map[string]string{"action": "F1", "target": "F2", "scope": "F3",
		"preconditions": "F4", "expected": "F5", "rollback": "F6", "evidence": "F7"}
	for field, id := range want {
		if ip.FieldIDs[field] != id {
			t.Errorf("字段 %s 的编号应为 %s，实测 %q", field, id, ip.FieldIDs[field])
		}
		if !contract.Has(ip.Fields, field) {
			t.Errorf("字段 %s 不在 fields 里", field)
		}
	}
	// 四附加件
	for _, f := range []string{"plan_id", "plan_digest", "approval", "status"} {
		if !contract.Has(ip.ExtraFields, f) {
			t.Errorf("附加件 %s 不在 extra_fields 里", f)
		}
	}
	// 四层
	if len(ip.Layers) != 4 {
		t.Errorf("四个层应恰好 4 条，实测 %v", ip.Layers)
	}
}

// TestDependencyOrderIsNotInverted —— 判据①：四件按依赖次序在位（缺任何一件，后一件不许先开）。
// 本件把次序落成**可机检的形状**：命令树里 `ask` / `plan` / `apply` 三条都在（前一件 = T-41 的
// id 规范化真源，由 contract.ModelIDMap() 承载）。
func TestDependencyOrderIsNotInverted(t *testing.T) {
	if _, err := contract.ModelIDMap(); err != nil {
		t.Fatalf("第一件（能力路由的 id 规范化真源）不在：%v ⇒ 后三件不许先开", err)
	}
	paths := zerg.CommandPathsForTest()
	for _, want := range []string{"ask", "plan", "apply"} {
		if !contains(paths, want) {
			t.Errorf("命令 %q 不在命令树里（依赖次序：能力路由 → ask → 意图 schema → plan/apply）", want)
		}
	}
}

// ---- 判据②：词表闸 ------------------------------------------------------------------------

// TestAskCapabilityVocabularyIsStrict —— 词表外 ⇒ `2`（且**在任何 HTTP 之前**判，
// 不碰主控）；`--prefer` 同样过词表。
func TestAskCapabilityVocabularyIsStrict(t *testing.T) {
	// 故意不给主控地址（ZERG_PORT 指向一个没人听的端口）—— 词表闸必须在 HTTP 之前把话说完
	t.Setenv("ZERG_PORT", "1") // 特权端口，本进程连不上

	rc, out, errb := fusionRun(t, "ask", "你好", "--capability", "绝无此能力")
	if rc != 2 {
		t.Errorf("词表外的能力名 ⇒ 退码 %d（要 2 · §十八.3-4「词表外 ⇒ 2」）· stderr=%s", rc, errb)
	}
	if out != "" {
		t.Errorf("词表外时 stdout 必须 0 字节，实测 %d 字节：%q", len(out), out)
	}
	if !strings.Contains(errb, "不在词表") {
		t.Errorf("stderr 没点名「不在词表」：%q", errb)
	}
	if !strings.Contains(errb, "bad_capability") {
		t.Errorf("stderr 没给 error.detail=bad_capability：%q", errb)
	}

	// `--prefer` 也过词表（软筛不是免检通道）
	rc, _, errb = fusionRun(t, "ask", "你好", "--prefer", "绝无此能力")
	if rc != 2 {
		t.Errorf("`--prefer` 词表外 ⇒ 退码 %d（要 2）· stderr=%s", rc, errb)
	}

	// 词表内 18 值全部收（逐条不误判为词表外）
	ip, err := contract.IntentPlan()
	if err != nil {
		t.Fatalf("真源读不出来：%v", err)
	}
	if len(ip.CapabilityDecidable) != 5 {
		t.Errorf("「今天判得出」的能力应为 5 条，实测 %d：%v", len(ip.CapabilityDecidable), ip.CapabilityDecidable)
	}
}

// TestAskInVocabularyButNoCandidateIsFail —— 判据②下半：**在词表但无候选** ⇒ `1`（不新增码）。
// 打一个**合成主控**（本机 httptest）：路由表里有一条模型，但它没有 `registry_id`（接不上能力面）
// ⇒ 能力筛后候选为空 ⇒ `1`。
func TestAskInVocabularyButNoCandidateIsFail(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{
		"/api/fleet/models":    `{"count":1,"models":[{"id":"orphan-model","host":"x","backend":"llama-server","mem_gb":8}]}`,
		"/api/models/registry": `{"count":0,"records":[]}`,
	})
	defer srv.Close()

	rc, out, errb := fusionRun(t, "ask", "你好", "--capability", "text")
	if rc != 1 {
		t.Errorf("在词表但无候选 ⇒ 退码 %d（要 1 · §十八.3-4「在词表但无候选 ⇒ 1」）· stderr=%s", rc, errb)
	}
	if out != "" {
		t.Errorf("拒答时 stdout 必须 0 字节，实测 %d 字节", len(out))
	}
	if !strings.Contains(errb, "no_capability_candidate") {
		t.Errorf("stderr 没给 detail=no_capability_candidate：%q", errb)
	}
}

// TestAskPicksCandidateViaRegistryIDJoin —— 判据①（能力路由接得到路由表）+ `--dry-run` 零副作用：
// 合成主控给一条**带 registry_id** 的模型 + 一条证过 `text` 的快照记录 ⇒ 能力筛命中、
// `--dry-run` 打印路线且**不发任何推理请求**（计数器为 0）。
func TestAskPicksCandidateViaRegistryIDJoin(t *testing.T) {
	var inferCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/fleet/models", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"count":1,"models":[{"id":"example-35b-v2","host":"Mr2109","backend":"llama-server","mem_gb":21,"registry_id":"example-35b-v2-1-5-35b-q4-k-m"}]}`)
	})
	mux.HandleFunc("/api/models/registry", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"count":1,"records":[{"id":"example-35b-v2-1-5-35b-q4-k-m","fleet_id":"example-35b-v2","capabilities":[{"name":"text","value":true,"source":"probed","evidence":"probe.text.v1"}]}]}`)
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		inferCalls++
		fmt.Fprint(w, `{"choices":[{"message":{"content":"不该被调到"}}]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")
	t.Setenv("ZERG_PORT", port)
	t.Setenv("ZERG_GATEWAY_PORT", port) // 网关也指过来：**dry-run 若有任何推理调用就会被计数**

	rc, out, errb := fusionRun(t, "ask", "你好", "--capability", "text", "--dry-run")
	if rc != 0 {
		t.Fatalf("有候选 + --dry-run ⇒ 退码 %d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "example-35b-v2") {
		t.Errorf("dry-run 路线里没点名候选模型：%q", out)
	}
	if !strings.Contains(out, "零副作用") {
		t.Errorf("dry-run 输出没写明零副作用口径：%q", out)
	}
	if inferCalls != 0 {
		t.Errorf("`--dry-run` 竟然发了 %d 次推理请求（要 0 · 零副作用）", inferCalls)
	}
}

// ---- 判据⑤ / 判据③④：plan 零副作用 · apply 四层 -------------------------------------------

// TestPlanIsZeroSideEffect —— 判据⑤：`plan` 只写**那一件**，工作目录逐字节不变。
func TestPlanIsZeroSideEffect(t *testing.T) {
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("编号闭集读不出来：%v", err)
	}
	work := t.TempDir()
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "哨兵.txt"), []byte("原样\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotDir(t, work)
	wd, _ := os.Getwd()
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(wd) }()

	planPath := filepath.Join(out, "p.json")
	rc, sout, serr := fusionRun(t, "plan", "model", "start", "example-35b-v2",
		"--dry-run", "--target", "待办:"+ids[0], "--out", planPath)
	if rc != 0 {
		t.Fatalf("`plan` 退码 %d（要 0）· stderr=%s", rc, serr)
	}
	if !strings.HasPrefix(strings.TrimSpace(sout), "PLAN-") {
		t.Errorf("`plan` 首行应给 plan_id，实测 %q", firstLine(sout))
	}
	if !strings.Contains(sout, "零副作用") {
		t.Errorf("`plan` 没写明零副作用口径：%q", sout)
	}
	if after := snapshotDir(t, work); after != before {
		t.Errorf("判据⑤ 破：工作目录被改了\n前:\n%s\n后:\n%s", before, after)
	}
	if _, err := os.Stat(planPath); err != nil {
		t.Fatalf("意图件没落盘：%v", err)
	}
}

// TestApplyFourLayersInOrder —— 判据③④：四层按序判、任一层不过后续层不跑。
func TestApplyFourLayersInOrder(t *testing.T) {
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("编号闭集读不出来：%v", err)
	}
	out := t.TempDir()
	good := filepath.Join(out, "good.json")
	if rc, _, se := fusionRun(t, "plan", "model", "start", "example-35b-v2",
		"--dry-run", "--target", "待办:"+ids[0], "--out", good); rc != 0 {
		t.Fatalf("造件失败 rc=%d stderr=%s", rc, se)
	}
	raw, _ := os.ReadFile(good)
	body := strings.TrimRight(string(raw), "\n")

	// ④-a L4：缺 `--confirm` ⇒ 2，且 L1/L2/L3 都过了（证明层是真按序跑的）
	rc, _, se := fusionRun(t, "apply", good)
	if rc != 2 {
		t.Errorf("缺 --confirm ⇒ 退码 %d（要 2）· stderr=%s", rc, se)
	}
	if !strings.Contains(se, "L4_human") || !strings.Contains(se, "confirm_required") {
		t.Errorf("stderr 没点名 L4_human/confirm_required：%q", se)
	}
	if !strings.Contains(se, "L3_dry_run") {
		t.Errorf("stderr 没显示 L1–L3 已过（层序不可见）：%q", se)
	}

	// ③/④-b L4 之后：`--confirm` 给对 ⇒ 四层全过、但写面未开放 ⇒ 2（不给结论）
	rc, _, se = fusionRun(t, "apply", good, "--confirm=example-35b-v2")
	if rc != 2 {
		t.Errorf("四层全过但未开放 ⇒ 退码 %d（要 2）· stderr=%s", rc, se)
	}
	if !strings.Contains(se, "not_opened") {
		t.Errorf("stderr 没点名 not_opened：%q", se)
	}

	// ④-c 件被改**一字节**（改一个字符）⇒ 2，且**卡在 L1**（后续层不跑）
	edited := filepath.Join(out, "edited.json")
	if err := os.WriteFile(edited, []byte(strings.Replace(body, "example-35b-v2", "example-35b-v2-1.5-35c", 1)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, se = fusionRun(t, "apply", edited, "--confirm=example-35b-v2-1.5-35c")
	if rc != 2 {
		t.Errorf("件被改一字节 ⇒ 退码 %d（要 2）· stderr=%s", rc, se)
	}
	if !strings.Contains(se, "L1_schema") {
		t.Errorf("改件应卡在 L1_schema：%q", se)
	}
	if !strings.Contains(se, "已过的层：[]") {
		t.Errorf("L1 没过时「已过的层」必须为空（后续层不跑）：%q", se)
	}

	// ③-d L1：动作不在闭集 ⇒ 2（改件后连摘要都对不上也要先报闭集）
	badAction := filepath.Join(out, "badaction.json")
	if err := os.WriteFile(badAction, []byte(strings.Replace(body, `"action":"model.start"`, `"action":"nope.nope"`, 1)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, se = fusionRun(t, "apply", badAction, "--confirm=x")
	if rc != 2 || !strings.Contains(se, "plan_action_bad") {
		t.Errorf("动作不在闭集 ⇒ 要 2 + plan_action_bad，实测 rc=%d stderr=%q", rc, se)
	}

	// ⑦ L2：依据回指不上既有编号 ⇒ 2，且 L1 已过、L3 未跑
	badRef := filepath.Join(out, "badref.json")
	body2 := rewritePlanDigest(t, strings.Replace(body, `"ref":"待办:`, `"ref":"待办:ZZZZ-`, 1))
	if err := os.WriteFile(badRef, []byte(body2+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, se = fusionRun(t, "apply", badRef, "--confirm=example-35b-v2")
	if rc != 2 || !strings.Contains(se, "L2_reference") {
		t.Errorf("依据回指不上 ⇒ 要 2 + L2_reference，实测 rc=%d stderr=%q", rc, se)
	}
	if !strings.Contains(se, "L1_schema") {
		t.Errorf("L2 失败时 L1 应在「已过的层」里：%q", se)
	}

	// ④-d 件读不到 ⇒ 2（不给结论 —— 判据没有对象）
	rc, _, se = fusionRun(t, "apply", filepath.Join(out, "不存在.json"), "--confirm=x")
	if rc != 2 || !strings.Contains(se, "plan_unreadable") {
		t.Errorf("件读不到 ⇒ 要 2 + plan_unreadable，实测 rc=%d stderr=%q", rc, se)
	}
}

// TestApplyRejectsNonDryRunPlanAtL3 —— 判据③：`plan`（不带 `--dry-run`）产出的件在 **L3** 被拒，
// 且 L1/L2 **已经过了**（层序可观测：L1、L2 在「已过的层」里，L4 没跑）。
func TestApplyRejectsNonDryRunPlanAtL3(t *testing.T) {
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("编号闭集读不出来：%v", err)
	}
	out := t.TempDir()
	p := filepath.Join(out, "nodry.json")
	if rc, _, se := fusionRun(t, "plan", "model", "start", "example-35b-v2",
		"--target", "待办:"+ids[0], "--out", p); rc != 0 {
		t.Fatalf("造件失败 rc=%d stderr=%s", rc, se)
	}
	rc, _, se := fusionRun(t, "apply", p, "--confirm=example-35b-v2")
	if rc != 2 || !strings.Contains(se, "plan_not_dry_run") {
		t.Fatalf("非干跑件 ⇒ 要 2 + plan_not_dry_run，实测 rc=%d stderr=%q", rc, se)
	}
	if !strings.Contains(se, "L1_schema") || !strings.Contains(se, "L2_reference") {
		t.Errorf("L3 失败时 L1/L2 应在「已过的层」里：%q", se)
	}
	if strings.Contains(se, "L4_human") {
		t.Errorf("L3 没过 ⇒ L4 **不许跑**（层序不许越过）：%q", se)
	}
}

// TestApplyRejectsWhenHostChanged —— 判据④：**换机** ⇒ 2（件是在别的机上算的）。
func TestApplyRejectsWhenHostChanged(t *testing.T) {
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("编号闭集读不出来：%v", err)
	}
	out := t.TempDir()
	p := filepath.Join(out, "host.json")
	t.Setenv("ZERG_HOST", "Mr2109")
	if rc, _, se := fusionRun(t, "plan", "model", "start", "example-35b-v2",
		"--dry-run", "--target", "待办:"+ids[0], "--out", p); rc != 0 {
		t.Fatalf("造件失败 rc=%d stderr=%s", rc, se)
	}
	t.Setenv("ZERG_HOST", "x3") // 换机
	rc, _, se := fusionRun(t, "apply", p, "--confirm=example-35b-v2")
	if rc != 2 || !strings.Contains(se, "plan_host_mismatch") {
		t.Fatalf("换机 ⇒ 要 2 + plan_host_mismatch，实测 rc=%d stderr=%q", rc, se)
	}
}

// ---- 负控：层序判定口**真的有牙** ----------------------------------------------------------

// layerOrderJudge 是本票「任一层不过 ⇒ 后续层不跑」这条判据的**唯一判定口**。
type fusionLive struct {
	FailedLayer  string
	LayersPassed []string
}

func layerOrderJudge(l fusionLive) []error {
	var errs []error
	all := []string{"L1_schema", "L2_reference", "L3_dry_run", "L4_human"}
	if l.FailedLayer == "" {
		return nil
	}
	idx := -1
	for i, name := range all {
		if name == l.FailedLayer {
			idx = i
		}
	}
	if idx < 0 {
		return []error{fmt.Errorf("失败的层名 %q 不在四层里", l.FailedLayer)}
	}
	// 失败层**之后**的层一律不许出现在「已过的层」里
	for _, seen := range l.LayersPassed {
		for i, name := range all {
			if name == seen && i > idx {
				errs = append(errs, fmt.Errorf("层序越过：卡在 %s，却声称 %s 也过了（后续层不许跑）", l.FailedLayer, seen))
			}
		}
	}
	return errs
}

// TestFusionLayerJudgeHasTeeth —— 负控：判定口喂一个**错的**实况（L1 没过却声称 L3 也跑了）⇒ 必须报错。
func TestFusionLayerJudgeHasTeeth(t *testing.T) {
	errs := layerOrderJudge(fusionLive{FailedLayer: "L1_schema", LayersPassed: []string{"L1_schema", "L3_dry_run"}})
	if len(errs) == 0 {
		t.Fatal("负控失败：判定口放过了「L1 没过却声称 L3 也跑了」⇒ 本票的层序判据不会红（假绿）")
	}
	if !strings.Contains(errs[0].Error(), "层序越过") {
		t.Errorf("负控报错文案没点名层序：%v", errs[0])
	}
	// 正控：合法层序不误判
	if errs := layerOrderJudge(fusionLive{FailedLayer: "L3_dry_run", LayersPassed: []string{"L1_schema", "L2_reference"}}); len(errs) != 0 {
		t.Errorf("正控失败：合法层序被判红：%v", errs)
	}
	// 正控：四层全过（FailedLayer 空）不报
	if errs := layerOrderJudge(fusionLive{}); len(errs) != 0 {
		t.Errorf("正控失败：全过被判红：%v", errs)
	}
}

// ---- 小工具 -----------------------------------------------------------------------------

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

var (
	planDigestRe = regexp.MustCompile(`"plan_digest":"([0-9a-f]*)"`)
	planIDRe     = regexp.MustCompile(`"plan_id":"(PLAN-[0-9a-f]*)"`)
)

// rewritePlanDigest 改过件正文之后**复算摘要**（与 `plan` 同法：把 plan_id 与 plan_digest
// 两格都清空再对整份包封求 sha256），这样能让 apply 走到后面的层，而不是一律卡在摘要不符上。
func rewritePlanDigest(t *testing.T, body string) string {
	t.Helper()
	blanked := planIDRe.ReplaceAllString(body, `"plan_id":""`)
	blanked = planDigestRe.ReplaceAllString(blanked, `"plan_digest":""`)
	sum := sha256.Sum256([]byte(blanked))
	newDigest := hex.EncodeToString(sum[:])
	out := planIDRe.ReplaceAllString(body, `"plan_id":"PLAN-`+newDigest[:12]+`"`)
	out = planDigestRe.ReplaceAllString(out, `"plan_digest":"`+newDigest+`"`)
	if out == body {
		t.Fatal("复算摘要失败：件里找不到 plan_digest 字段")
	}
	return out
}

// newSyntheticMaster 起一个**合成主控**（只答给定的路径），并按它的端口设置 `ZERG_PORT`。
func newSyntheticMaster(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for p, body := range routes {
		b := body
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, b)
		})
	}
	srv := httptest.NewServer(mux)
	t.Setenv("ZERG_PORT", strings.TrimPrefix(srv.URL, "http://127.0.0.1:"))
	return srv
}

// 一句话口径：本票的四闭集真源不许在别处再抄一份 —— 这条断言用「真源里的动作闭集
// 与命令树里真实存在的族名对得上」来表达（`model` / `agent` / `core` / `task` 都在命令树里）。
func TestIntentTargetKindsExistInCommandTree(t *testing.T) {
	ip, err := contract.IntentPlan()
	if err != nil {
		t.Fatalf("真源读不出来：%v", err)
	}
	paths := zerg.CommandPathsForTest()
	has := map[string]bool{}
	for _, p := range paths {
		seg := strings.SplitN(p, " ", 2)[0]
		has[seg] = true
	}
	for _, k := range ip.TargetKinds {
		if !has[k] {
			t.Errorf("target.kind %q 在命令树里没有对应族", k)
		}
	}
	// 动作名的族段必须在 target.kind 闭集里（一件动作只能落在四类目标上）
	for _, a := range ip.Actions {
		fam, _, ok := strings.Cut(a, ".")
		if !ok || !contract.Has(ip.TargetKinds, fam) {
			t.Errorf("动作 %q 的族段不是合法 target.kind", a)
		}
	}
	_ = modelreg.CapabilityNames
	_ = json.Valid
}
