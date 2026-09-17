package api

// ============ B 项③ 片（单子）最小 schema 与挂板校验 —— 用例（2026-09-18）============
//
// 覆盖（任务书要求四条 + 两条守卫）:
//  1. 无 id ⇒ 拒                     TestSliceMount_RejectsMissingSliceID
//  2. 有环 ⇒ 拒                     TestSliceMount_RejectsDependencyCycle
//  3. 悬空依赖 ⇒ 拒                 TestSliceMount_RejectsDanglingDependency
//  4. 合法 ⇒ 通过（含拓扑可排序）    TestSliceMount_AcceptsLegalSliceAndTopoSorts
//  5. 缺 acceptance 声明 ≠ 显式空    TestSliceMount_AcceptanceMissingVsExplicitlyEmpty
//  6. 未声明片字段 ⇒ 提交行为不变    TestSliceMount_UndeclaredFieldsLeaveSubmitUntouched（零回归守卫）
//  7. HTTP 面 ⇒ 400 + 可行动错误     TestSliceMountHandler_RejectsAndAcceptsOverHTTP
//
// 取证明细（一律真读真跑，不 mock 判定）:
//   - 「拒绝入队」的判据 = 调度器队列长度仍为 0（maxConcurrent=0 ⇒ 放行只排队、不起进程）
//   - 观测面的判据 = 真读 SliceMountEventsFile() 落盘的那一行（不是看返回值）
//   - 错误可行动的判据 = 错误码 + 原文里必须出现「缺什么」与「怎么改」

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// newSliceTestScheduler 建一个**不派发**的调度器（maxConcurrent=0 ⇒ 入队只排队、不起进程、
// 不 spawn 任何东西 ⇒ 用例确定且快），并把任务持久化文件 / 任务目录 / 事件目录全切到 t.TempDir()
// （不碰真机 /tmp/zerg-tasks.json 与 ~/.zerg/state —— 与仓库既有测试隔离习惯同规）。
func newSliceTestScheduler(t *testing.T) *MasterScheduler {
	t.Helper()
	isolateTasksFile(t)
	isolateTaskRoot(t)
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	return NewMasterScheduler("/bin/echo", 0)
}

// readSliceMountEvents 真读回挂板事件（观测面取证——不 mock，落盘文件不存在 ⇒ 返回 nil）
func readSliceMountEvents(t *testing.T) []SliceMountEvent {
	t.Helper()
	data, err := os.ReadFile(SliceMountEventsFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("读挂板事件失败: %v", err)
	}
	var out []SliceMountEvent
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var ev SliceMountEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("挂板事件不是合法 JSON 行: %q (%v)", line, err)
		}
		out = append(out, ev)
	}
	return out
}

// assertActionable 错误必须**可行动**: 有机器可判错误码 + 原文点明缺什么 + 给出怎么写对
func assertActionable(t *testing.T, serr *SliceValidationError, wantCode, wantField string, wantWords ...string) {
	t.Helper()
	if serr == nil {
		t.Fatalf("期望被拒（%s），实际放行", wantCode)
	}
	if serr.Code != wantCode {
		t.Fatalf("错误码不符: 期望 %s，实际 %s（原文: %s）", wantCode, serr.Code, serr.Message)
	}
	if serr.Field != wantField {
		t.Fatalf("出错字段不符: 期望 %s，实际 %s（原文: %s）", wantField, serr.Field, serr.Message)
	}
	for _, w := range wantWords {
		if !strings.Contains(serr.Message, w) {
			t.Fatalf("错误信息不可行动: 缺字样 %q（原文: %s）", w, serr.Message)
		}
	}
}

// ---- 用例 1: 无 id ⇒ 拒 ----

func TestSliceMount_RejectsMissingSliceID(t *testing.T) {
	// ① 纯函数面: 只要声明了任何一个片字段，就必须给出 slice_id
	empty := []string{}
	in := SliceInput{DependsOn: []string{"S0"}, Acceptance: &empty}
	serr := ValidateSliceMount(in, SliceBoard{"S0": nil})
	assertActionable(t, serr, SliceErrMissingID, "slice_id", "缺 slice_id", "怎么改")
	t.Logf("① 纯函数面原文: %s", serr.Message)

	// 只声明 owner 也算「声明了片字段」——同样要 id（否则无法被 depends_on 引用）
	if serr2 := ValidateSliceMount(SliceInput{Owner: "Mr2109"}, SliceBoard{}); serr2 == nil ||
		serr2.Code != SliceErrMissingID {
		t.Fatalf("只声明 owner 时也应因缺 slice_id 被拒，实际: %v", serr2)
	}

	// ② 入队面: 拒绝 = **不入队**（队空）+ 观测面恰有一条 deny 事件
	s := newSliceTestScheduler(t)
	task := &Task{ID: "t-missing-id", Description: "片无 id", SliceID: "", DependsOn: []string{"S0"}, Acceptance: &empty}
	s.Submit(task)
	if got := s.QueueLen(); got != 0 {
		t.Fatalf("无 slice_id 的片必须拒绝入队——队列应为 0，实际 %d", got)
	}
	evs := readSliceMountEvents(t)
	if len(evs) != 1 {
		t.Fatalf("观测面应有且仅有一条挂板事件，实际 %d 条: %+v", len(evs), evs)
	}
	if evs[0].OK || evs[0].Code != SliceErrMissingID || evs[0].TaskID != "t-missing-id" {
		t.Fatalf("deny 事件字段不符: %+v", evs[0])
	}
	t.Logf("② 观测事件原文: event=%s ok=%v task=%s code=%s field=%s",
		evs[0].Event, evs[0].OK, evs[0].TaskID, evs[0].Code, evs[0].Field)
}

// ---- 用例 2: 有环 ⇒ 拒 ----

func TestSliceMount_RejectsDependencyCycle(t *testing.T) {
	empty := []string{}
	// ① 经真实入队路径造环: 板上已有 S2（S2 依赖 S1），再挂 S1（S1 依赖 S2）⇒ S1 → S2 → S1
	s := newSliceTestScheduler(t)
	s.mu.Lock()
	s.history["h-s2"] = &Task{ID: "h-s2", SliceID: "S2", DependsOn: []string{"S1"}}
	s.mu.Unlock()
	err := s.SubmitSlice(&Task{ID: "t-s1", SliceID: "S1", DependsOn: []string{"S2"}, Acceptance: &empty})
	if err == nil {
		t.Fatal("互相依赖的片（S1→S2→S1）必须拒绝入队，实际放行")
	}
	serr, ok := err.(*SliceValidationError)
	if !ok {
		t.Fatalf("应返回 *SliceValidationError，实际 %T", err)
	}
	assertActionable(t, serr, SliceErrDependsCycle, "depends_on", "成环", "怎么改", "S1", "S2")
	t.Logf("① 环: %s", serr.Message)
	if got := s.QueueLen(); got != 0 {
		t.Fatalf("成环的片必须拒绝入队——队列应为 0，实际 %d", got)
	}

	// ② 自依赖也是环（且不该被误报成「悬空」——自依赖在板上的定义里算存在）
	selfErr := ValidateSliceMount(
		SliceInput{SliceID: "S7", DependsOn: []string{"S7"}, Acceptance: &empty}, SliceBoard{})
	assertActionable(t, selfErr, SliceErrDependsCycle, "depends_on", "S7")
	t.Logf("② 自依赖: %s", selfErr.Message)

	// ③ 观测面: 两条 deny 事件（真实入队路径 1 条 + 纯函数不落事件 ⇒ 断言只落了入队那条）
	evs := readSliceMountEvents(t)
	if len(evs) != 1 || evs[0].OK || evs[0].Code != SliceErrDependsCycle {
		t.Fatalf("观测面应恰有一条 deny(环) 事件，实际 %+v", evs)
	}
}

// ---- 用例 3: 悬空依赖 ⇒ 拒 ----

func TestSliceMount_RejectsDanglingDependency(t *testing.T) {
	empty := []string{}
	// ① 纯函数面: 板上只有 S1，本片依赖不存在的 S99
	serr := ValidateSliceMount(
		SliceInput{SliceID: "S2", DependsOn: []string{"S1", "S99"}, Acceptance: &empty},
		SliceBoard{"S1": nil})
	assertActionable(t, serr, SliceErrDependsDangling, "depends_on", "不存在的片", "S99", "怎么改")
	if !strings.Contains(serr.Message, "S1") {
		t.Fatalf("错误信息应给出板上可用的片 id（S1）: %s", serr.Message)
	}
	t.Logf("① 纯函数面原文: %s", serr.Message)

	// ② 入队面: 板上为空 ⇒ 任何依赖都悬空 —— 拒绝且不入队
	s := newSliceTestScheduler(t)
	s.Submit(&Task{ID: "t-dangling", SliceID: "S3", DependsOn: []string{"S1"}, Acceptance: &empty})
	if got := s.QueueLen(); got != 0 {
		t.Fatalf("悬空依赖的片必须拒绝入队——队列应为 0，实际 %d", got)
	}
	evs := readSliceMountEvents(t)
	if len(evs) != 1 || evs[0].OK || evs[0].Code != SliceErrDependsDangling {
		t.Fatalf("观测面应恰有一条 deny(悬空) 事件，实际 %+v", evs)
	}
	if !strings.Contains(evs[0].Detail, "空——板上还没有任何片") {
		t.Fatalf("空板的错误信息应明说板上无片，实际: %s", evs[0].Detail)
	}
}

// ---- 用例 4: 合法 ⇒ 通过（含 depends_on 拓扑可排序）----

func TestSliceMount_AcceptsLegalSliceAndTopoSorts(t *testing.T) {
	acc := []string{"go test ./internal/api/ -run TestSliceMount -count=1"}
	// ① 纯函数面: S1 ← S2 ← S3（依赖先于被依赖者）
	board := SliceBoard{"S1": nil, "S2": []string{"S1"}, "S3": []string{"S1", "S2"}}
	if serr := ValidateSliceMount(
		SliceInput{SliceID: "S4", DependsOn: []string{"S2"}, Acceptance: &acc}, board); serr != nil {
		t.Fatalf("合法的片不该被拒: %s", serr.Message)
	}

	// ② 拓扑可排序: 顺序确定（同图同序）+ 每条依赖都在被依赖者之前
	order, err := TopoSortSlices(SliceBoard{"S1": nil, "S2": []string{"S1"}, "S3": []string{"S1", "S2"}})
	if err != nil {
		t.Fatalf("合法依赖图必须可拓扑排序，实际报错: %v", err)
	}
	if fmt.Sprint(order) != "[S1 S2 S3]" {
		t.Fatalf("拓扑序应为 [S1 S2 S3]（确定性），实际 %v", order)
	}
	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	for id, deps := range board {
		for _, d := range deps {
			if pos[d] >= pos[id] {
				t.Fatalf("拓扑序违反依赖: %s 依赖 %s，但 %s 排在 %s 之后（%v）", id, d, d, id, order)
			}
		}
	}
	t.Logf("② 拓扑序 = %v", order)

	// ③ 入队面: 按拓扑序依次挂三片，全部放行入队
	s := newSliceTestScheduler(t)
	for _, tc := range []struct {
		id   string
		deps []string
	}{{"S1", nil}, {"S2", []string{"S1"}}, {"S3", []string{"S2"}}} {
		if err := s.SubmitSlice(&Task{
			ID: "t-" + tc.id, SliceID: tc.id, DependsOn: tc.deps, Acceptance: &acc,
		}); err != nil {
			t.Fatalf("片 %s 应放行入队，实际被拒: %v", tc.id, err)
		}
	}
	if got := s.QueueLen(); got != 3 {
		t.Fatalf("三片都应入队——队列应为 3，实际 %d", got)
	}
	got := s.SliceBoard()
	if len(got) != 3 || len(got["S3"]) != 1 || got["S3"][0] != "S2" {
		t.Fatalf("挂板视图不符: %+v", got)
	}
	t.Logf("③ 挂板视图 = %+v", got)

	// ④ 观测面: 三片各一条 allow 事件（**恰好一条**——不允许重复上报）
	evs := readSliceMountEvents(t)
	if len(evs) != 3 {
		t.Fatalf("三片应恰有 3 条 allow 事件，实际 %d 条: %+v", len(evs), evs)
	}
	for _, ev := range evs {
		if !ev.OK || ev.Code != "" || !ev.AcceptanceDeclared || ev.Event != sliceMountEventName {
			t.Fatalf("allow 事件字段不符: %+v", ev)
		}
	}
	t.Logf("④ allow 事件（首条）= %+v", evs[0])
}

// ---- 用例 5: 缺 acceptance 声明 ≠ 显式空 -----

func TestSliceMount_AcceptanceMissingVsExplicitlyEmpty(t *testing.T) {
	// ① 线上真实解码语义: **键缺失** 与 **显式 []** 必须能分开（这是用指针承载的原因）
	var missing SliceInput
	if err := json.Unmarshal([]byte(`{"slice_id":"S1","owner":"Mr2109"}`), &missing); err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if missing.Acceptance != nil {
		t.Fatalf("键缺失时 Acceptance 应为 nil（缺失），实际 %v", *missing.Acceptance)
	}
	if !missing.Declared() {
		t.Fatal("声明了 slice_id/owner 就是「声明了片字段」")
	}
	var explicitEmpty SliceInput
	if err := json.Unmarshal([]byte(`{"slice_id":"S1","acceptance":[]}`), &explicitEmpty); err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if explicitEmpty.Acceptance == nil {
		t.Fatal("显式 [] 时 Acceptance 必须非 nil（显式声明为空）——缺失与显式空不可混为一谈")
	}
	if len(*explicitEmpty.Acceptance) != 0 {
		t.Fatalf("显式 [] 应是空判据列表，实际 %v", *explicitEmpty.Acceptance)
	}
	var withItems SliceInput
	if err := json.Unmarshal([]byte(`{"slice_id":"S1","acceptance":["go vet ./internal/api/"]}`), &withItems); err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	t.Logf("① 解码: 缺失=%v 显式空=%v(len=%d) 有判据=%v", missing.Acceptance == nil,
		explicitEmpty.Acceptance != nil, len(*explicitEmpty.Acceptance), *withItems.Acceptance)

	// ② 校验: 缺失 ⇒ 拒；显式空 ⇒ 放行
	assertActionable(t, ValidateSliceMount(missing, SliceBoard{}),
		SliceErrMissingAcceptance, "acceptance", "缺 acceptance 声明", "怎么改", "区分开")
	if serr := ValidateSliceMount(explicitEmpty, SliceBoard{}); serr != nil {
		t.Fatalf("显式空数组（已声明、暂无判据）应放行，实际: %s", serr.Message)
	}
	if serr := ValidateSliceMount(withItems, SliceBoard{}); serr != nil {
		t.Fatalf("显式判据应放行，实际: %s", serr.Message)
	}

	// ③ 观测面: 拒绝时 acceptance_declared=false，放行时 true——两个形态在观测面上可分
	s := newSliceTestScheduler(t)
	s.Submit(&Task{ID: "t-no-acc", SliceID: "S1", Acceptance: nil})
	s.Submit(&Task{ID: "t-acc", SliceID: "S2", Acceptance: &[]string{}})
	evs := readSliceMountEvents(t)
	if len(evs) != 2 {
		t.Fatalf("应有 2 条事件（1 deny 1 allow），实际 %d: %+v", len(evs), evs)
	}
	if evs[0].OK || evs[0].AcceptanceDeclared || evs[0].Code != SliceErrMissingAcceptance {
		t.Fatalf("缺声明的 deny 事件不符: %+v", evs[0])
	}
	if !evs[1].OK || !evs[1].AcceptanceDeclared {
		t.Fatalf("显式空的 allow 事件不符: %+v", evs[1])
	}
	if got := s.QueueLen(); got != 1 {
		t.Fatalf("只有第二片该入队——队列应为 1，实际 %d", got)
	}
}

// ---- 用例 6（零回归守卫）: 未声明片字段 ⇒ 提交行为与观测面都不变 ----

func TestSliceMount_UndeclaredFieldsLeaveSubmitUntouched(t *testing.T) {
	s := newSliceTestScheduler(t)
	// 现有形态的任务（完全不带片字段）——原样提交
	t1 := &Task{ID: "t-plain-1", Description: "普通任务", Priority: PriorityInternal}
	t2 := &Task{ID: "t-plain-2", Description: "普通任务2", Priority: PriorityExternal}
	s.Submit(t1)
	s.Submit(t2)
	if got := s.QueueLen(); got != 2 {
		t.Fatalf("未声明片字段的任务不该被拦——队列应为 2，实际 %d", got)
	}
	// 观测面: 一个字节都不该落（现有任务在观测面上零变化）
	if evs := readSliceMountEvents(t); len(evs) != 0 {
		t.Fatalf("非片任务不该产生挂板事件，实际 %+v", evs)
	}
	if _, err := os.Stat(SliceMountEventsFile()); !os.IsNotExist(err) {
		t.Fatalf("非片任务不该创建事件文件（err=%v）", err)
	}

	// 序列化形态: 现有任务的 JSON 里不得出现片字段（omitempty ⇒ 输出与改动前逐字节一致）
	raw, err := json.Marshal(&Task{ID: "t-plain-1", Description: "普通任务", Priority: PriorityInternal})
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	for _, k := range []string{"slice_id", "depends_on", "owner", "acceptance"} {
		if strings.Contains(string(raw), `"`+k+`"`) {
			t.Fatalf("现有任务的 JSON 不该含片字段 %q: %s", k, raw)
		}
	}
	t.Logf("未声明片字段: 队列=2 事件=0 JSON=%s", raw)
}

// ---- 用例 7: HTTP 面（/api/tasks）----

func TestSliceMountHandler_RejectsAndAcceptsOverHTTP(t *testing.T) {
	s := newSliceTestScheduler(t)
	h := &Handlers{Scheduler: s}

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.SubmitTaskHandler(w, req)
		return w
	}
	type errResp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	// ① 缺 slice_id（声明了 depends_on）⇒ 400 SLICE_MISSING_ID，且不入队
	w := post(`{"description":"片","depends_on":["S1"],"acceptance":["echo ok"]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 slice_id 应回 400，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	var e1 errResp
	_ = json.Unmarshal(w.Body.Bytes(), &e1)
	if e1.Error.Type != SliceErrMissingID || !strings.Contains(e1.Error.Message, "怎么改") {
		t.Fatalf("HTTP 错误必须带机器可判码 + 可行动建议，实际: %s", w.Body.String())
	}
	t.Logf("① HTTP %d %s", w.Code, w.Body.String())

	// ② 缺 acceptance 声明 ⇒ 400 SLICE_MISSING_ACCEPTANCE
	w = post(`{"description":"片","slice_id":"S9"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 acceptance 声明应回 400，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	var e2 errResp
	_ = json.Unmarshal(w.Body.Bytes(), &e2)
	if e2.Error.Type != SliceErrMissingAcceptance {
		t.Fatalf("错误码应为 %s，实际 %s", SliceErrMissingAcceptance, e2.Error.Type)
	}
	t.Logf("② HTTP %d %s", w.Code, w.Body.String())

	// ③ 悬空依赖 ⇒ 400 SLICE_DEPENDS_DANGLING（板上无 S1）
	w = post(`{"description":"片","slice_id":"S10","depends_on":["S1"],"acceptance":["echo ok"]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("悬空依赖应回 400，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	var e3 errResp
	_ = json.Unmarshal(w.Body.Bytes(), &e3)
	if e3.Error.Type != SliceErrDependsDangling {
		t.Fatalf("错误码应为 %s，实际 %s", SliceErrDependsDangling, e3.Error.Type)
	}
	if s.QueueLen() != 0 {
		t.Fatalf("三次拒绝后队列必须仍为空，实际 %d", s.QueueLen())
	}

	// ④ 合法片 ⇒ 202 入队；随后依赖它的片也能挂（证明通过 HTTP 面真的进板）
	w = post(`{"description":"片1","slice_id":"S1","acceptance":["echo ok"]}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("合法片应回 202，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	w = post(`{"description":"片2","slice_id":"S2","depends_on":["S1"],"owner":"Mr2109","acceptance":["echo ok"]}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("依赖已挂片的片应回 202，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	if got := s.QueueLen(); got != 2 {
		t.Fatalf("两片应入队——队列应为 2，实际 %d", got)
	}

	// ⑤ 非片任务（既有形态）⇒ 202，原路不变
	w = post(`{"description":"普通任务","model":"m","priority":3}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("非片任务应回 202，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	if got := s.QueueLen(); got != 3 {
		t.Fatalf("非片任务应照常入队——队列应为 3，实际 %d", got)
	}
	t.Logf("④⑤ 队列=%d 事件=%d", s.QueueLen(), len(readSliceMountEvents(t)))
}
