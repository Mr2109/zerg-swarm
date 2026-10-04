package store

import (
	"encoding/json"
	"strings"
	"testing"
)

// ── 引擎实时态（GAP-20260925-45）──────────────────────────────────
// 反例优先：缺省 ≠ 空数组撑场；上报的值必须原样落库并出现在 /api/fleet/status 的载荷里。

// TestHeartbeat_DecodeAndCarryEngines —— 子端心跳带 engines[] ⇒ 落库 + 快照可查 + JSON 可出。
func TestHeartbeat_DecodeAndCarryEngines(t *testing.T) {
	payload := []byte(`{
		"machine":"edge-node","backend_state":"ready","model":"example-35b","active_requests":2,"healthy":true,
		"engines":[
			{"egg_id":"example-35b","state":"ready","pid":4242,"inflight":2,"fail_n":1,
			 "verdict":"extended","reason":"窗口无输出但工时在涨","last_output_ago_s":12.5,"cpu_usage_usec_delta":900000},
			{"egg_id":"gemma-4-12B","state":"ready","pid":4243,"inflight":0,"verdict":"ok"}
		],
		"future_unknown_field":{"x":1}
	}`)
	var req HeartbeatRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		t.Fatalf("心跳体解析失败: %v", err)
	}
	if len(req.Engines) != 2 {
		t.Fatalf("engines 应解出 2 项，实得 %d", len(req.Engines))
	}
	if req.Engines[0].EggID != "example-35b" || req.Engines[0].Inflight != 2 ||
		req.Engines[0].Verdict != "extended" || req.Engines[0].LastOutputAgoS != 12.5 ||
		req.Engines[0].CPUUsecDelta != 900000 || req.Engines[0].FailN != 1 {
		t.Fatalf("engines[0] 字段未如实解出: %+v", req.Engines[0])
	}

	s := NewStore()
	s.ReceiveHeartbeat(req)
	snap := s.GetSnapshot("edge-node")
	if snap == nil {
		t.Fatal("快照未创建")
	}
	if len(snap.Engines) != 2 || snap.Engines[0].EggID != "example-35b" {
		t.Fatalf("快照未带引擎实时态: %+v", snap.Engines)
	}

	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("快照序列化失败: %v", err)
	}
	if !strings.Contains(string(raw), `"verdict":"extended"`) {
		t.Fatalf("序列化后的快照缺 engines 判词: %s", raw)
	}
}

// TestHeartbeat_EnginesAbsentWhenNotReported —— 反例：没报 engines ⇒ 快照里该键**缺席**
// （缺省 ≠ 空数组撑场；与 active_requests「没来源就不出现」同纪律）。
func TestHeartbeat_EnginesAbsentWhenNotReported(t *testing.T) {
	s := NewStore()
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "mini-node-1", Healthy: true})
	snap := s.GetSnapshot("mini-node-1")
	if snap == nil {
		t.Fatal("快照未创建")
	}
	if snap.Engines != nil {
		t.Fatalf("没报 engines 时快照该键应为 nil，实得 %+v", snap.Engines)
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if strings.Contains(string(raw), `"engines"`) {
		t.Fatalf("没报 engines 时序列化不该出现该键（omitempty 纪律）: %s", raw)
	}
}

// TestHeartbeat_EnginesOverwrittenOnNextBeat —— 心跳覆盖：上一拍的 engines 不许残留。
func TestHeartbeat_EnginesOverwrittenOnNextBeat(t *testing.T) {
	s := NewStore()
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "edge-node", Engines: []EngineLive{{EggID: "a", Inflight: 1}}})
	s.ReceiveHeartbeat(HeartbeatRequest{Machine: "edge-node"}) // 第二拍没带
	snap := s.GetSnapshot("edge-node")
	if snap.Engines != nil {
		t.Fatalf("第二拍未带 engines ⇒ 应被覆盖为 nil，实得 %+v", snap.Engines)
	}
}
