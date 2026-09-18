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
		if !ev.OK || ev.Code != "" || !ev.AcceptanceDeclared || ev.Event != sliceCreatedEventName ||
			ev.Alias != sliceMountEventName {
			t.Fatalf("allow 事件字段不符（B 项⑤: event=slice_created，旧名归并为 alias=slice_mount）: %+v", ev)
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
	// B 项 B5: 三个新片字段键也一并不许出现（未声明 ⇒ 零字节）
	for _, k := range []string{"slice_id", "depends_on", "owner", "acceptance",
		"lead", "members", "plan_approved"} {
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

// ============ B 项 B5（2026-09-18）: 黑板三字段 —— Lead / 分工方案 / 等确认闸 —— 用例 ============
//
// 覆盖（新字段缺失 / 存在 / 非法 各一 + 等确认闸两态 + HTTP 面）:
//
//	 8. Lead 字段: 缺失 ⇒ 放（不拒）· 存在 ⇒ 放 + 落进 Task + JSON 往返 · 非法（空白）⇒ 拒
//	 9. 分工方案: 缺失 ⇒ 放 · 存在 ⇒ 放 + 往返 · 非法（缺成员 id / 缺「干什么」）⇒ 拒
//	10. 等确认闸: 未确认 ⇒ 拒 · 显式 false ⇒ 拒 · 已确认 ⇒ 放 · 没提分工方案 ⇒ 闸不适用（零回归面）
//	11. HTTP 面: 三个新码过 POST /api/tasks（lead 空白 / 成员缺项 / 未确认）+ 已确认 ⇒ 202
//
// 取证明细（同既有 7 条 —— 一律真读真跑，不 mock 判定）:
//   - 「拒」的判据 = 调度器队列长度不长（maxConcurrent=0 ⇒ 放行只排队、不起进程）
//   - 观测面的判据 = 真读 SliceMountEventsFile() 落盘的那一行（Code/Field 逐字对）
//   - 「落进 Task」的判据 = 真 json.Marshal 出来的字节里能看到该键（指针 ⇒「没写」与「写了空」可分）
//
// 出处（逐字，只引用）: docs/01-设计/设计-协作骨架-v2.1-20260918.md:190（Lead 角色）/ :203（提分工方案
// ⇒ 等 Mr2109 确认 ⇒ 挂板）/ :69（s13「提出分工 ✓ 等确认 ✓」+ `plan approval`）/:211（黑板 = 归属 Lead/Member）；
// 设计-任务模块骨架-v1.0-20260918.md:59（须由 Lead/人声明）；§4.6-2（成员是带 id 的实例）。

// ---- 用例 8: Lead 字段（缺失 ⇒ 放 · 存在 ⇒ 放 + 落盘 · 非法 ⇒ 拒）----

func TestSliceMount_LeadFieldMissingPresentIllegal(t *testing.T) {
	empty := []string{}

	// ① 缺失: 没写 lead ⇒ **不拒**（稿面把 Lead 定为角色之一「由 Mr2109 指定」，未写「缺 Lead 即拒」⇒ 不自造必填）
	if serr := ValidateSliceMount(
		SliceInput{SliceID: "S1", Acceptance: &empty}, SliceBoard{}); serr != nil {
		t.Fatalf("没写 lead 的片不该被拒（lead 本轮不参与必填判定）: %s", serr.Message)
	}

	// ② 存在: lead 非空 ⇒ 放行 + 真落进 Task（序列化能看见、往返不失真）
	lead := "qwen3-coder"
	s := newSliceTestScheduler(t)
	task := &Task{ID: "t-lead", Description: "带 Lead 的片", Priority: PriorityInternal}
	SliceInput{SliceID: "S2", Acceptance: &empty, Lead: &lead}.ApplyTo(task)
	if err := s.SubmitSlice(task); err != nil {
		t.Fatalf("带 lead 的片应放行入队，实际被拒: %v", err)
	}
	if got := s.QueueLen(); got != 1 {
		t.Fatalf("带 lead 的片应入队——队列应为 1，实际 %d", got)
	}
	raw, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if !strings.Contains(string(raw), `"lead":"qwen3-coder"`) {
		t.Fatalf("lead 应出现在任务 JSON 里: %s", raw)
	}
	var back Task
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if back.Lead == nil || *back.Lead != lead {
		t.Fatalf("lead 往返失真: %v", back.Lead)
	}
	if !SliceInputFromTask(&back).Declared() {
		t.Fatal("只声明 lead 也算「声明了片字段」（挂板校验的总开关要认它）")
	}
	t.Logf("② lead 落进任务 = %s", raw)

	// ③ 非法: 显式声明了 lead 却给空白 ⇒ 拒（SLICE_LEAD_INVALID，可行动）
	blank := "   "
	serr := ValidateSliceMount(
		SliceInput{SliceID: "S3", Acceptance: &empty, Lead: &blank}, SliceBoard{})
	assertActionable(t, serr, SliceErrLeadInvalid, "lead", "怎么改", "没写 = 未指定")
	t.Logf("③ 非法 lead 原文: %s", serr.Message)
	if got := SliceInputFromTask(&Task{SliceID: "S3", Acceptance: &empty, Lead: &blank}).Lead; got == nil || *got != blank {
		t.Fatalf("空白 lead 也应原样带进入参（判定在闸里，不在搬运里）: %v", got)
	}
	// 入队面: 空白 lead 的片**不入队**（队列仍 1）+ 观测面恰一条 deny
	s.Submit(&Task{ID: "t-lead-blank", SliceID: "S3", Acceptance: &empty, Lead: &blank})
	if got := s.QueueLen(); got != 1 {
		t.Fatalf("空白 lead 的片必须拒绝入队——队列应仍为 1，实际 %d", got)
	}
	evs := readSliceMountEvents(t)
	if len(evs) != 2 || evs[1].OK || evs[1].Code != SliceErrLeadInvalid || evs[1].Field != "lead" {
		t.Fatalf("观测面应为 1 allow + 1 deny(lead)，实际 %+v", evs)
	}
	t.Logf("③ 观测事件（deny）= event=%s code=%s field=%s task=%s",
		evs[1].Event, evs[1].Code, evs[1].Field, evs[1].TaskID)
}

// ---- 用例 9: 分工方案（缺失 ⇒ 放 · 存在 ⇒ 放 + 落盘 · 非法 ⇒ 拒）----

func TestSliceMount_DivisionOfLaborMissingPresentIllegal(t *testing.T) {
	empty := []string{}

	// ① 缺失: 没写 members ⇒ 放（且**不触发**等确认闸——没提分工方案就没有要确认的东西）
	if serr := ValidateSliceMount(
		SliceInput{SliceID: "S1", Acceptance: &empty}, SliceBoard{}); serr != nil {
		t.Fatalf("没写分工方案的片不该被拒: %s", serr.Message)
	}

	// ② 存在: members 每条的 id/duty 都齐全 + 已确认 ⇒ 放 + 落进 Task + 往返
	members := []SliceMember{
		{ID: "egg-alpha", Duty: "写用例并跑 go test ./internal/api/ -count=1"},
		{ID: "egg-beta", Duty: "接 HTTP 面（POST /api/tasks）"},
	}
	yes := true
	s := newSliceTestScheduler(t)
	task := &Task{ID: "t-div", Description: "带分工方案的片", Priority: PriorityInternal}
	SliceInput{SliceID: "S2", Acceptance: &empty, Members: members, PlanApproved: &yes}.ApplyTo(task)
	if err := s.SubmitSlice(task); err != nil {
		t.Fatalf("带分工方案（已确认）的片应放行入队，实际被拒: %v", err)
	}
	if got := s.QueueLen(); got != 1 {
		t.Fatalf("应入队——队列应为 1，实际 %d", got)
	}
	raw, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	for _, want := range []string{`"members":[`, `"id":"egg-alpha"`, `"duty":"写用例并跑 go test ./internal/api/ -count=1"`, `"plan_approved":true`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("分工方案应出现在任务 JSON 里（缺 %s）: %s", want, raw)
		}
	}
	var back Task
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if len(back.Members) != 2 || back.Members[0].ID != "egg-alpha" || back.Members[1].Duty != "接 HTTP 面（POST /api/tasks）" {
		t.Fatalf("分工方案往返失真: %+v", back.Members)
	}
	t.Logf("② 分工方案落进任务 = %s", raw)

	// ③ 非法: 成员缺 id ⇒ 拒
	noID := []SliceMember{{Duty: "写用例"}}
	serr := ValidateSliceMount(
		SliceInput{SliceID: "S3", Acceptance: &empty, Members: noID, PlanApproved: &yes}, SliceBoard{})
	assertActionable(t, serr, SliceErrMemberInvalid, "members", "怎么改", "带 id 的实例")
	t.Logf("③ 非法成员原文（缺 id）: %s", serr.Message)

	// ③b 非法: 成员有 id 但没写「干什么」⇒ 同一码、同一字段
	noDuty := []SliceMember{{ID: "egg-gamma"}}
	serr2 := ValidateSliceMount(
		SliceInput{SliceID: "S3", Acceptance: &empty, Members: noDuty, PlanApproved: &yes}, SliceBoard{})
	assertActionable(t, serr2, SliceErrMemberInvalid, "members", "怎么改", "没写「干什么」")
	t.Logf("③b 非法成员原文（缺 duty）: %s", serr2.Message)

	// 入队面: 缺项的片**不入队**（队列仍 1）+ 观测面恰一条 deny
	s.Submit(&Task{ID: "t-div-bad", SliceID: "S4", Acceptance: &empty, Members: noDuty})
	if got := s.QueueLen(); got != 1 {
		t.Fatalf("分工方案缺项的片必须拒绝入队——队列应仍为 1，实际 %d", got)
	}
	evs := readSliceMountEvents(t)
	if len(evs) != 2 || evs[1].OK || evs[1].Code != SliceErrMemberInvalid || evs[1].Field != "members" {
		t.Fatalf("观测面应为 1 allow + 1 deny(members)，实际 %+v", evs)
	}
	t.Logf("③ 观测事件（deny）= event=%s code=%s field=%s", evs[1].Event, evs[1].Code, evs[1].Field)
}

// ---- 用例 10: 等确认闸（未确认 ⇒ 拒 · 显式 false ⇒ 拒 · 已确认 ⇒ 放 · 未提分工 ⇒ 不适用）----

func TestSliceMount_PlanApprovalGateUnconfirmedRootCause(t *testing.T) {
	empty := []string{}
	members := []SliceMember{{ID: "egg-alpha", Duty: "写用例并跑 -count=1"}}

	// ① 未确认（声明了分工方案、没写 plan_approved）⇒ **拒**，且不入队
	s := newSliceTestScheduler(t)
	err := s.SubmitSlice(&Task{
		ID: "t-unconfirmed", SliceID: "S1", Acceptance: &empty, Members: members,
		Lead: strPtr("qwen3-coder"),
	})
	if err == nil {
		t.Fatal("提了分工方案但未确认的片必须被拒（v2.1 §4.3: 提分工方案 ⇒ 等确认 ⇒ 挂板），实际放行")
	}
	serr, ok := err.(*SliceValidationError)
	if !ok {
		t.Fatalf("应返回 *SliceValidationError，实际 %T", err)
	}
	assertActionable(t, serr, SliceErrPlanUnconfirmed, "plan_approved", "等 Mr2109 确认", "怎么改")
	t.Logf("① 未确认原文: %s", serr.Message)
	if got := s.QueueLen(); got != 0 {
		t.Fatalf("未确认的片不该入队——队列应为 0，实际 %d", got)
	}
	evs := readSliceMountEvents(t)
	if len(evs) != 1 || evs[0].OK || evs[0].Code != SliceErrPlanUnconfirmed ||
		evs[0].Field != "plan_approved" || evs[0].SliceID != "S1" {
		t.Fatalf("观测面应恰有一条 deny(等确认闸)，实际 %+v", evs)
	}

	// ② 显式 false ⇒ 同样拒（fail closed —— 显式「没确认」不许静默通过）
	no := false
	s2 := newSliceTestScheduler(t)
	err2 := s2.SubmitSlice(&Task{
		ID: "t-declined", SliceID: "S2", Acceptance: &empty, Members: members, PlanApproved: &no,
	})
	serr2, ok2 := err2.(*SliceValidationError)
	if !ok2 || serr2.Code != SliceErrPlanUnconfirmed {
		t.Fatalf("显式 plan_approved=false 应判未确认并被拒，实际 %v", err2)
	}
	if got := s2.QueueLen(); got != 0 {
		t.Fatalf("显式 false 的片不该入队——队列应为 0，实际 %d", got)
	}
	t.Logf("② 显式 false 原文: %s", serr2.Message)

	// ③ 已确认（显式 true）⇒ **放**
	yes := true
	s3 := newSliceTestScheduler(t)
	if err := s3.SubmitSlice(&Task{
		ID: "t-approved", SliceID: "S3", Acceptance: &empty, Members: members, PlanApproved: &yes,
	}); err != nil {
		t.Fatalf("已确认（plan_approved=true）的片应放行，实际被拒: %v", err)
	}
	if got := s3.QueueLen(); got != 1 {
		t.Fatalf("已确认的片应入队——队列应为 1，实际 %d", got)
	}
	evs3 := readSliceMountEvents(t)
	if len(evs3) != 1 || !evs3[0].OK || evs3[0].Code != "" {
		t.Fatalf("应恰有一条 allow 事件，实际 %+v", evs3)
	}
	t.Logf("③ 已确认 ⇒ 放（队列=%d，allow 事件 1 条）", s3.QueueLen())

	// ④ 闸**不适用**: 没提分工方案（members 空）且没写确认键 ⇒ 放（= 现有形态的片零回归面）
	s4 := newSliceTestScheduler(t)
	if err := s4.SubmitSlice(&Task{
		ID: "t-no-plan", SliceID: "S4", Acceptance: &empty, Lead: strPtr("qwen3-coder"),
	}); err != nil {
		t.Fatalf("没提分工方案的片不该被等确认闸拦（闸只管「提了分工方案」的片）: %v", err)
	}
	if got := s4.QueueLen(); got != 1 {
		t.Fatalf("没提分工方案的片应照常入队——队列应为 1，实际 %d", got)
	}
	t.Logf("④ 未提分工方案 ⇒ 闸不适用（队列=1）")
}

// ---- 用例 11: HTTP 面（POST /api/tasks）三个新码 + 已确认放行 ----

func TestSliceMountHandler_PlanApprovalGateOverHTTP(t *testing.T) {
	s := newSliceTestScheduler(t)
	h := &Handlers{Scheduler: s}
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(body))
		w := httptest.NewRecorder()
		h.SubmitTaskHandler(w, req)
		return w
	}
	errType := func(w *httptest.ResponseRecorder) string {
		var e struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &e)
		if !strings.Contains(e.Error.Message, "怎么改") {
			t.Fatalf("HTTP 错误必须可行动（含「怎么改」）: %s", w.Body.String())
		}
		return e.Error.Type
	}

	// ① lead 空白 ⇒ 400 SLICE_LEAD_INVALID
	w := post(`{"description":"片","slice_id":"S1","acceptance":["echo ok"],"lead":"  "}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("空白 lead 应回 400，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	if got := errType(w); got != SliceErrLeadInvalid {
		t.Fatalf("错误码应为 %s，实际 %s", SliceErrLeadInvalid, got)
	}
	t.Logf("① HTTP %d %s", w.Code, w.Body.String())

	// ② 分工方案里成员缺 id ⇒ 400 SLICE_MEMBER_INVALID
	w = post(`{"description":"片","slice_id":"S2","acceptance":["echo ok"],"members":[{"duty":"写用例"}],"plan_approved":true}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("成员缺 id 应回 400，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	if got := errType(w); got != SliceErrMemberInvalid {
		t.Fatalf("错误码应为 %s，实际 %s", SliceErrMemberInvalid, got)
	}
	t.Logf("② HTTP %d %s", w.Code, w.Body.String())

	// ③ 声明了分工方案但未确认 ⇒ 400 SLICE_PLAN_UNCONFIRMED（等确认闸 = 挂板前拒）
	w = post(`{"description":"片","slice_id":"S3","acceptance":["echo ok"],"members":[{"id":"egg-alpha","duty":"写用例"}]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未确认应回 400，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	if got := errType(w); got != SliceErrPlanUnconfirmed {
		t.Fatalf("错误码应为 %s，实际 %s", SliceErrPlanUnconfirmed, got)
	}
	if s.QueueLen() != 0 {
		t.Fatalf("三次拒绝后队列必须仍为空，实际 %d", s.QueueLen())
	}
	t.Logf("③ HTTP %d %s", w.Code, w.Body.String())

	// ④ 已确认 + Lead + 分工方案齐全 ⇒ 202 入队（证明过了闸真进板）
	w = post(`{"description":"片4","slice_id":"S4","lead":"qwen3-coder","acceptance":["echo ok"],` +
		`"members":[{"id":"egg-alpha","duty":"写用例"}],"plan_approved":true}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("已确认的片应回 202，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	if got := s.QueueLen(); got != 1 {
		t.Fatalf("已确认的片应入队——队列应为 1，实际 %d", got)
	}
	if got := s.SliceBoard(); len(got) != 1 || got["S4"] == nil && len(got["S4"]) != 0 {
		t.Fatalf("已确认的片应真的进板（挂板视图）: %+v", got)
	}
	// 依赖它的片也能挂（证明进板的是真片）
	w = post(`{"description":"片5","slice_id":"S5","depends_on":["S4"],"acceptance":["echo ok"]}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("依赖已挂片的片应回 202，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	if got := s.QueueLen(); got != 2 {
		t.Fatalf("两片应入队——队列应为 2，实际 %d", got)
	}
	t.Logf("④ HTTP %d 队列=%d", w.Code, s.QueueLen())

	// ⑤ 非片任务（既有形态）⇒ 202，原路不变
	w = post(`{"description":"普通任务","model":"m","priority":3}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("非片任务应回 202，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	if got := s.QueueLen(); got != 3 {
		t.Fatalf("非片任务应照常入队——队列应为 3，实际 %d", got)
	}
	t.Logf("⑤ 队列=%d 事件=%d", s.QueueLen(), len(readSliceMountEvents(t)))
}

// 注: 「显式写了这个键」用的字符串指针走包内既有 helper strPtr（handlers_test.go:579），本文件不另造一份。
