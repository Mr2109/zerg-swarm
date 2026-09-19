// eggs_unit_liveness_test.go —— 2026-09-19 ②：OOM 后卵状态陈旧 + 看门狗判 ok。
//
// 现象（真机）：`GET /eggs` 返回 `state=ready · gtt_gb=78.2`，而同一时刻机器上 `GTT 用 0.0 GiB ·
// ds4 进程 0`；`watchdog.verdict = "ok"` ⇒ 主控按「有活儿能派」路由过去 ⇒ 客户端 503/0 token。
// 病灶：卵表项直接来自 backend 自报的状态机字段，**没有单位活体核验**。
//
// 本文件钉住（liveFor 注入假核验；不碰真 systemd）：
//
//	① 伪造「unit 已 failed」⇒ state **不得** ready（落 dead）+ `unit_state=failed` + `dead_unit` 判词；
//	② 伪造「unit 核不到」⇒ state 落 missing（「不知道」也不许报 ready）；
//	③ unit 还在活动 ⇒ 原样 ready（不许把活卵误报死——那是另一种害）；
//	④ 裸 exec（没有 unit）⇒ unit_state 缺席（不编造），且**核验器一次都不被调**；
//	⑤ 原有看门狗计数不许被覆盖丢（窗口/次数保留，原判词写进理由）；
//	⑥ 记录**不删**：条数不变（证据要留；清理归卸载/GC 路径）。
package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
)

// obsEgg 造一枚「孵化路径」的只读观测（有 unit，状态机自称 ready）。
func obsEgg(id, unit string) backend.EggObservation {
	return backend.EggObservation{EggID: id, Model: id, Unit: unit, State: backend.StateReady, Inflight: 0}
}

// liveOf 注入式核验器：按单元名给结论，同时记录被问过哪些单元。
func liveOf(m map[string]backend.UnitLiveness) (func(string) backend.UnitLiveness, *[]string) {
	var asked []string
	return func(unit string) backend.UnitLiveness {
		asked = append(asked, unit)
		if lv, ok := m[unit]; ok {
			return lv
		}
		return backend.UnitLiveness{State: "missing", Detail: "测试没有喂这条单元"}
	}, &asked
}

func eggJSON(t *testing.T, e servicesEgg) map[string]interface{} {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// ① 单元 failed（OOM 形态）⇒ 不得 ready，落 dead + unit_state=failed + dead_unit 判词。
func TestEggs_DeadUnitIsNeverReady(t *testing.T) {
	obs := []backend.EggObservation{obsEgg("deepseek-v4-flash", "zerg-deepseek-v4-flash")}
	eggs := eggEntries(obs, nil, nil)
	live, asked := liveOf(map[string]backend.UnitLiveness{
		"zerg-deepseek-v4-flash": {State: "failed", Alive: false,
			Detail: "单元 zerg-deepseek-v4-flash：ActiveState=failed SubState=failed Result=oom-kill ExecMainStatus=0"},
	})
	var logs []string
	n := applyUnitLiveness(obs, eggs, live, func(eggID, unitState, state, reason string) {
		logs = append(logs, eggID+"|"+state+"|"+unitState)
	})

	if n != 1 {
		t.Fatalf("应判 1 枚死卵，实得 %d", n)
	}
	if len(*asked) != 1 || (*asked)[0] != "zerg-deepseek-v4-flash" {
		t.Fatalf("核验器应被问过该单元名，实得 %v", *asked)
	}
	if eggs[0].State == backend.StateReady || eggs[0].State != "dead" {
		t.Fatalf("单元 failed 时 state 必须落 dead（真机缺陷就是这里报了 ready），实得 %q", eggs[0].State)
	}
	if eggs[0].UnitState != "failed" {
		t.Fatalf("unit_state 应为 failed，实得 %q", eggs[0].UnitState)
	}
	if eggs[0].Watchdog == nil || eggs[0].Watchdog.Verdict != backend.WatchdogDeadUnit {
		t.Fatalf("看门狗判词应补 dead_unit 档，实得 %+v", eggs[0].Watchdog)
	}
	if eggs[0].Watchdog.Reason == "" || !strings.Contains(eggs[0].Watchdog.Reason, "oom-kill") {
		t.Fatalf("判词理由必须带单元现场证据（Result=oom-kill），实得 %q", eggs[0].Watchdog.Reason)
	}
	if len(logs) != 1 || logs[0] != "deepseek-v4-flash|dead|failed" {
		t.Fatalf("必须留痕一条（egg|state|unit_state），实得 %v", logs)
	}
	// ⑥ 记录不删：条数不变。
	if len(eggs) != len(obs) {
		t.Fatalf("记录条数必须不变（不静默删记录）：obs=%d eggs=%d", len(obs), len(eggs))
	}
	// JSON 面上也不许出现 ready（主控读的就是这个）。
	j := eggJSON(t, eggs[0])
	if j["state"] == backend.StateReady {
		t.Fatalf("JSON 里 state 仍是 ready：%v", j)
	}
	if j["unit_state"] != "failed" {
		t.Fatalf("JSON 应带 unit_state=failed，实得 %v", j["unit_state"])
	}
}

// ② 核不到单元（非 Linux / 没有 systemctl / 单元不存在）⇒ state 落 missing，仍不报 ready。
func TestEggs_MissingUnitFallsToMissing(t *testing.T) {
	obs := []backend.EggObservation{obsEgg("egg-a", "zerg-egg-a")}
	eggs := eggEntries(obs, nil, nil)
	live, _ := liveOf(map[string]backend.UnitLiveness{
		"zerg-egg-a": {State: "missing", Detail: "核不到单元 zerg-egg-a 的 ActiveState"},
	})
	applyUnitLiveness(obs, eggs, live, nil)
	if eggs[0].State != "missing" {
		t.Fatalf("核不到单元时 state 应落 missing（「不知道」不许当「活着」），实得 %q", eggs[0].State)
	}
	if eggs[0].UnitState != "missing" {
		t.Fatalf("unit_state 应为 missing，实得 %q", eggs[0].UnitState)
	}
}

// ③ 活卵不许被误报：unit active ⇒ state 原样 ready、unit_state=active、不塞 dead_unit 判词。
func TestEggs_AliveUnitKeepsReady(t *testing.T) {
	obs := []backend.EggObservation{obsEgg("egg-a", "zerg-egg-a")}
	eggs := eggEntries(obs, nil, nil)
	live, _ := liveOf(map[string]backend.UnitLiveness{
		"zerg-egg-a": {State: "active", Alive: true, Detail: "单元 zerg-egg-a：ActiveState=active"},
	})
	if n := applyUnitLiveness(obs, eggs, live, nil); n != 0 {
		t.Fatalf("活卵不该被判死，实得 %d 枚", n)
	}
	if eggs[0].State != backend.StateReady || eggs[0].UnitState != "active" {
		t.Fatalf("活卵应原样 ready + unit_state=active，实得 state=%q unit_state=%q", eggs[0].State, eggs[0].UnitState)
	}
	if eggs[0].Watchdog != nil {
		t.Fatalf("活卵不该被塞 dead_unit 判词，实得 %+v", eggs[0].Watchdog)
	}
}

// ④ 裸 exec（没有 unit）：unit_state 缺席，且核验器一次都不被调（没有单元可核，不编造）。
func TestEggs_NoUnitSkipsLivenessProbe(t *testing.T) {
	obs := []backend.EggObservation{obsEgg("bare-egg", "")}
	eggs := eggEntries(obs, nil, nil)
	live, asked := liveOf(nil)
	applyUnitLiveness(obs, eggs, live, nil)
	if len(*asked) != 0 {
		t.Fatalf("没有单元名时不该去核（会被核成 missing ⇒ 把裸 exec 卵误报死）：%v", *asked)
	}
	if eggs[0].UnitState != "" || eggs[0].State != backend.StateReady {
		t.Fatalf("裸 exec 路径 unit_state 应缺席、state 原样，实得 state=%q unit_state=%q", eggs[0].State, eggs[0].UnitState)
	}
	j := eggJSON(t, eggs[0])
	if _, has := j["unit_state"]; has {
		t.Fatalf("unit_state 应缺席（不编造）：%v", j)
	}
}

// ⑤ 原有看门狗计数不丢：窗口/次数/工时保留，原判词写进理由。
func TestEggs_DeadUnitKeepsPreviousWatchdogCounters(t *testing.T) {
	prev := &backend.WatchdogObservation{
		WindowIdx: 3, Extensions: 2, LastOutputAgoS: 91.5, CPUUsecDelta: 0,
		Verdict: backend.WatchdogStuckCPUStalled, Reason: "第 3 个窗口无输出，且本单元 CPU 工时一点不动",
	}
	obs := []backend.EggObservation{{EggID: "egg-a", Model: "egg-a", Unit: "zerg-egg-a", State: backend.StateReady, Watchdog: prev}}
	eggs := eggEntries(obs, nil, nil)
	live, _ := liveOf(map[string]backend.UnitLiveness{"zerg-egg-a": {State: "inactive", Detail: "单元 zerg-egg-a：ActiveState=inactive"}})
	applyUnitLiveness(obs, eggs, live, nil)

	got := eggs[0].Watchdog
	if got.Verdict != backend.WatchdogDeadUnit {
		t.Fatalf("判词应为 dead_unit，实得 %q", got.Verdict)
	}
	if got.WindowIdx != 3 || got.Extensions != 2 || got.LastOutputAgoS != 91.5 {
		t.Fatalf("原看门狗计数必须保留（不许因为换了个判词就把现场证据清掉）：%+v", got)
	}
	if !strings.Contains(got.Reason, string(backend.WatchdogStuckCPUStalled)) {
		t.Fatalf("原判词应写进理由（复盘要用）：%q", got.Reason)
	}
	if prev.Verdict != backend.WatchdogStuckCPUStalled {
		t.Fatalf("不许改动 backend 账本里的共享观测对象：%+v", prev)
	}
}

// ⑥ 结构性断言：两个观测面端点都必须真的调用核验（handler 层的接线不许被悄悄摘掉）。
//
// 为什么用源码断言：handler 级的「死卵」端到端用例需要把一个**真单元**弄死（真 systemd 用户实例），
// 开发机（macOS）上不可得；而「核验被接进 /eggs 与 /services」这件事是纯接线，源码层面可钉死
// （同 backend/unit_probe_test.go 对 waitForReady 的结构性断言）。
func TestEggs_EndpointsWireUnitLiveness(t *testing.T) {
	src, err := readSourceForAssertion("eggs_endpoint.go")
	if err != nil {
		t.Fatalf("读 eggs_endpoint.go 失败: %v", err)
	}
	code := stripLineComments(src)
	if n := strings.Count(code, "s.applyEggUnitLiveness(obs, eggs)"); n != 2 {
		t.Fatalf("/eggs 与 /services 两处都必须调用 applyEggUnitLiveness（实得 %d 处）——"+
			"少一处就会有一条观测面继续把死卵报成 ready", n)
	}
	if !strings.Contains(code, "len(obs) != len(eggs)") {
		t.Fatal("applyUnitLiveness 必须拒绝长度不等的输入（不许猜对齐后越界/错位改状态）")
	}
	if !strings.Contains(code, "applyUnitLiveness(obs, eggs, s.agent.backends.UnitLiveness") {
		t.Fatal("applyEggUnitLiveness 必须真的把核验接到 applyUnitLiveness（核验器 = backend.Manager 的只读探针）——" +
			"留一个恒返回 0 的壳子（或换成一个假核验器）都会让观测面继续把死卵报成 ready")
	}
}

// readSourceForAssertion 读本包源码（结构性断言用；CWD = 包目录）。
func readSourceForAssertion(name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(".", name))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// stripLineComments 去掉整行注释（免得注释里提到的调用被判成真的接线）。
func stripLineComments(src string) string {
	var sb strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}
