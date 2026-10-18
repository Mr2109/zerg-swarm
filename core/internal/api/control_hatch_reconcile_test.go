// control_hatch_reconcile_test.go —— 主控「孵化容忍档」的两态验收（GAP-20261018-15）。
//
// 背景：首次展开 113 GB 的蛋在 edge-node 实测 471 s + 就绪 34 s；主控的前台转发窗是 300 s ⇒
// 旧行为在这里把 `load_tasks` 判成 failed（模型其实起来了）。改后：
//
//	① 转发超时 ⇒ 不判红，转「孵化容忍」：按子端只读面 `/eggs` 的 `expand` 进度继续核，
//	   就绪即 phase=ok（并在孵化期把 `expand` 投到 `load_tasks[].expand`）；
//	② 进度停滞 ⇒ failed（报话「展开停滞」）；
//	③ 等满容忍窗 ⇒ failed，且**三种形态分别报**（实例已起 / 只在展开 / 只读面什么都没有）。
package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
)

// hatchFakeNode —— 假子端：`/load` 慢于主控前台窗（模拟首次展开），`/eggs` 由 case 决定。
func hatchFakeNode(t *testing.T, loadDelay time.Duration, eggs func(n int32) string) (string, int) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/load", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(loadDelay)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"backend":"llama-server","port":9400}`))
	})
	var n int32
	mux.HandleFunc("/eggs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		k := atomic.AddInt32(&n, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(eggs(k)))
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return zzSplitHostPort(t, s.URL)
}

// shortenHatchTimings 把三枚时长收窄到秒级（生产默认值不动；判据要能在几秒内验完）。
func shortenHatchTimings(t *testing.T, dispatch, every, stall time.Duration) {
	t.Helper()
	od, oe, os := loadDispatchTimeout, loadHatchReconcileEvery, loadHatchStallWindow
	loadDispatchTimeout, loadHatchReconcileEvery, loadHatchStallWindow = dispatch, every, stall
	t.Cleanup(func() { loadDispatchTimeout, loadHatchReconcileEvery, loadHatchStallWindow = od, oe, os })
}

// hatchTaskRow 取该任务在只读面里的那一行。
func hatchTaskRow(t *testing.T, h *ControlHandlers, taskID string) map[string]interface{} {
	t.Helper()
	for _, row := range h.snapshotLoadTasks() {
		if s, _ := row["task_id"].(string); s == taskID {
			return row
		}
	}
	t.Fatalf("只读面 `load_tasks` 里查不到任务 %s", taskID)
	return nil
}

// TestLoadHatchReconcile_ForwardTimeoutTurnsOK —— 正控：转发超时后按 /eggs 核到就绪 ⇒ phase=ok。
func TestLoadHatchReconcile_ForwardTimeoutTurnsOK(t *testing.T) {
	shortenHatchTimings(t, 150*time.Millisecond, 20*time.Millisecond, 5*time.Second)
	host, port := hatchFakeNode(t, 600*time.Millisecond, func(n int32) string {
		if n <= 3 {
			return fmt.Sprintf(`{"eggs":[{"egg_id":"m1","model":"m1","state":"loading","port":9400,`+
				`"expand":{"state":"expanding","bytes_done":%d,"bytes_total":1000,"segments_done":%d,"segments_total":10}}]}`,
				n*10, n)
		}
		return `{"eggs":[{"egg_id":"m1","model":"m1","state":"ready","port":9400}]}`
	})
	h := NewControlHandlers("tok", zzCfg(host, port))
	taskID := newLoadTaskID("fake", "m1")
	h.addLoadTask(&loadTaskRec{id: taskID, machine: "fake", model: "m1", phase: phaseAccepted, startedAt: time.Now()})

	// 孵化期的只读投影：必须能看见 `expand`（主控的装载判据有依据，不是「探不到就超时」）
	var sawExpand atomic.Bool
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			for _, row := range h.snapshotLoadTasks() {
				if s, _ := row["task_id"].(string); s == taskID {
					if _, ok := row["expand"]; ok {
						sawExpand.Store(true)
					}
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	res := h.dispatchLoad(config.FleetNode{Host: host, Port: port}, "fake", "m1",
		[]byte(`{"model":"m1"}`), taskID, 5*time.Second)
	close(stop)
	if res.err != nil {
		t.Fatalf("转超时 + 只读面已就绪 ⇒ 不该把错误透给调用方：%v", res.err)
	}
	row := hatchTaskRow(t, h, taskID)
	if got := row["phase"]; got != phaseOK {
		t.Fatalf("只读面已报 ready ⇒ phase 必须是 ok（改前是 failed），实得 %v · row=%v", got, row)
	}
	if got, _ := row["port"].(int); got != 9400 {
		t.Fatalf("phase=ok 时应带出端口 9400，实得 %v", row["port"])
	}
	if !sawExpand.Load() {
		t.Error("孵化期 `load_tasks[].expand` 一次都没出现（主控的进度面没接线）")
	}
}

// TestLoadHatchReconcile_StallFailsDistinctly —— 负控①：进度卡住 ⇒ failed 且报话可分辨。
func TestLoadHatchReconcile_StallFailsDistinctly(t *testing.T) {
	shortenHatchTimings(t, 120*time.Millisecond, 20*time.Millisecond, 200*time.Millisecond)
	host, port := hatchFakeNode(t, 500*time.Millisecond, func(n int32) string {
		return `{"eggs":[{"egg_id":"m1","model":"m1","state":"loading",` +
			`"expand":{"state":"expanding","bytes_done":5,"bytes_total":1000,"segments_done":1,"segments_total":10}}]}`
	})
	h := NewControlHandlers("tok", zzCfg(host, port))
	taskID := newLoadTaskID("fake", "m1")
	h.addLoadTask(&loadTaskRec{id: taskID, machine: "fake", model: "m1", phase: phaseAccepted, startedAt: time.Now()})

	h.dispatchLoad(config.FleetNode{Host: host, Port: port}, "fake", "m1", []byte(`{"model":"m1"}`), taskID, 10*time.Second)
	row := hatchTaskRow(t, h, taskID)
	if got := row["phase"]; got != phaseFailed {
		t.Fatalf("进度停滞必须判 failed，实得 %v", got)
	}
	msg, _ := row["error"].(string)
	if !strings.Contains(msg, "展开停滞") {
		t.Fatalf("失败原因必须点明「展开停滞」（可分辨，不是笼统 failed）：%q", msg)
	}
}

// TestLoadHatchReconcile_TimeoutReasonsDistinct —— 负控②③：等满上界时「实例已起」与
// 「只读面什么都没有」必须报得出来（这正是本缺口要的「可分辨」）。
func TestLoadHatchReconcile_TimeoutReasonsDistinct(t *testing.T) {
	t.Run("实例已起但未就绪", func(t *testing.T) {
		shortenHatchTimings(t, 100*time.Millisecond, 20*time.Millisecond, 10*time.Second)
		host, port := hatchFakeNode(t, 400*time.Millisecond, func(n int32) string {
			return `{"eggs":[{"egg_id":"m1","model":"m1","state":"loading","port":9400,"pid":4242}]}`
		})
		h := NewControlHandlers("tok", zzCfg(host, port))
		taskID := newLoadTaskID("fake", "m1")
		h.addLoadTask(&loadTaskRec{id: taskID, machine: "fake", model: "m1", phase: phaseAccepted, startedAt: time.Now()})
		h.dispatchLoad(config.FleetNode{Host: host, Port: port}, "fake", "m1", []byte(`{"model":"m1"}`), taskID, 400*time.Millisecond)
		row := hatchTaskRow(t, h, taskID)
		if row["phase"] != phaseFailed {
			t.Fatalf("等满上界仍未就绪 ⇒ failed，实得 %v", row["phase"])
		}
		msg, _ := row["error"].(string)
		for _, frag := range []string{"实例已起", "port=9400", "pid=4242"} {
			if !strings.Contains(msg, frag) {
				t.Fatalf("「孵化超时但实例已起」这类形态必须可分辨，报话缺 %q：%q", frag, msg)
			}
		}
	})

	t.Run("只读面什么都没有", func(t *testing.T) {
		shortenHatchTimings(t, 100*time.Millisecond, 20*time.Millisecond, 10*time.Second)
		host, port := hatchFakeNode(t, 400*time.Millisecond, func(n int32) string { return `{"eggs":[]}` })
		h := NewControlHandlers("tok", zzCfg(host, port))
		taskID := newLoadTaskID("fake", "m1")
		h.addLoadTask(&loadTaskRec{id: taskID, machine: "fake", model: "m1", phase: phaseAccepted, startedAt: time.Now()})
		h.dispatchLoad(config.FleetNode{Host: host, Port: port}, "fake", "m1", []byte(`{"model":"m1"}`), taskID, 300*time.Millisecond)
		row := hatchTaskRow(t, h, taskID)
		if row["phase"] != phaseFailed {
			t.Fatalf("只读面探不到 ⇒ failed，实得 %v", row["phase"])
		}
		msg, _ := row["error"].(string)
		if !strings.Contains(msg, "既没有该卵的条目") {
			t.Fatalf("「探不到」必须与「实例已起」分开报：%q", msg)
		}
	})
}
