// family_agent_engines_test.go —— `zerg agent engines`（缺口 `GAP-20260927-04`）与
// `agent slots` 回据投影（缺口 `GAP-20260927-202` ②）的回归牙。
//
// 为什么要有它：这两条命令的「读懂回据」这半是全命令里唯一可用**回据原文**钉死的判据 ——
// 真打子端/引擎由 `agentStubChildForTest`（family_agent_bench_test.go 既有桩）替代，
// 断言落在「在跑的才算数、pid/端口/状态如实透出、空回据是**空结论**不是取不到、非数组不编字段」。
package main

import (
	"io"
	"testing"
)

// r6TwoEggsBody —— 两枚在跑（port>0）+ 一枚登记但没跑（port=0）+ 每枚带 pid/unit_state。
const r6TwoEggsBody = `{"eggs":[
 {"egg_id":"gemma-4-26B","model":"gemma-4-26B","state":"idle_armed","unit_state":"active","port":9401,"pid":31337,"inflight":0,"managed":true},
 {"egg_id":"example-35b","model":"example-35b","state":"ready","unit_state":"active","port":9400,"pid":4242,"inflight":1,"managed":true},
 {"egg_id":"Qwen3.8-27B","model":"Qwen3.8-27B","state":"idle","port":0,"managed":true}
]}`

// ① 在跑的（port>0）逐枚列出；pid/端口/状态/unit_state/managed 如实透出；port=0 的不进清单。
func TestAgentEngineFacts_ListsRunningWithPid(t *testing.T) {
	child := agentStubChildForTest(t, r6TwoEggsBody, statusTwoModels)
	inv := &invocation{}
	facts, rc := agentEngineFacts(inv, child, io.Discard)
	if rc != exitOK {
		t.Fatalf("应退 0，实得 %d", rc)
	}
	if len(facts) != 2 {
		t.Fatalf("回据里 2 枚在跑（port>0），认到 %d 枚：%+v", len(facts), facts)
	}
	byModel := map[string]agentEngineFact{}
	for _, f := range facts {
		byModel[f.Model] = f
	}
	if _, ok := byModel["Qwen3.8-27B"]; ok {
		t.Fatalf("`port=0` 的（没在跑）不该进清单")
	}
	orn, ok := byModel["example-35b"]
	if !ok {
		t.Fatalf("在跑的 example-35b 没被认出，实得 %+v", facts)
	}
	if orn.Port != 9400 || orn.PID != 4242 || orn.State != "ready" || orn.UnitState != "active" || !orn.Managed || orn.Via != "/eggs" {
		t.Fatalf("在跑引擎的 pid/端口/状态/unit_state/managed 应如实透出，实得 %+v", orn)
	}
}

// ② 没有在跑的引擎 ⇒ **空结论**退 0（不是 blocked）：`/eggs` 空 + `/status.port=0` 两处一致。
func TestAgentEngineFacts_EmptyIsConclusion(t *testing.T) {
	child := agentStubChildForTest(t, `{"eggs":[]}`, `{"machine":"Mr2109","port":0,"model":""}`)
	inv := &invocation{}
	facts, rc := agentEngineFacts(inv, child, io.Discard)
	if rc != exitOK {
		t.Fatalf("没有在跑的引擎是**空结论**（退 0），实得 %d", rc)
	}
	if len(facts) != 0 {
		t.Fatalf("应为空，实得 %+v", facts)
	}
}

// ③ `/eggs` 没报在跑的 ⇒ `/status.port` 兜底（与 `agent bench` 同一通道），带 pid 与 via=/status。
func TestAgentEngineFacts_StatusFallback(t *testing.T) {
	child := agentStubChildForTest(t, `{"eggs":[]}`,
		`{"machine":"Mr2109","model":"example-35b","port":9400,"backend_state":"ready","pid":4242}`)
	inv := &invocation{}
	facts, rc := agentEngineFacts(inv, child, io.Discard)
	if rc != exitOK || len(facts) != 1 {
		t.Fatalf("/status 兜底应出 1 枚（退 0），实得 rc=%d facts=%+v", rc, facts)
	}
	if facts[0].Port != 9400 || facts[0].Via != "/status" || facts[0].PID != 4242 {
		t.Fatalf("兜底那一枚应带端口/via=/status/pid，实得 %+v", facts[0])
	}
}

// ④ `agent slots` 回据投影：JSON 数组 → 逐槽行（字段名逐字取自回据）；非数组 ⇒ 退 8（不编字段）。
func TestAgentSlotRowsFromPayload(t *testing.T) {
	rows, rc := agentSlotRowsFromPayload("Mr2109", "example-35b", 9400,
		`[{"id":0,"state":"idle","n_ctx":4096,"prompt":""},{"id":1,"prompt":"虫族"}]`, "2", "/eggs")
	if rc != exitOK {
		t.Fatalf("合法数组应退 0，实得 %d", rc)
	}
	if len(rows) != 2 {
		t.Fatalf("应 2 槽，实得 %d", len(rows))
	}
	if rows[0]["slot_id"] != "0" || rows[0]["slot_state"] != "idle" || rows[0]["n_ctx"] != "4096" || rows[0]["slots_total"] != "2" {
		t.Fatalf("第 0 槽字段应逐字取自回据，实得 %+v", rows[0])
	}
	if rows[1]["slot_id"] != "1" || rows[1]["prompt_chars"] != "2" {
		t.Fatalf("第 1 槽 id/prompt_chars（按字符数）不对，实得 %+v", rows[1])
	}
	if _, rc := agentSlotRowsFromPayload("Mr2109", "m", 9400, `{"slots":1}`, "", "/eggs"); rc != exitBlocked {
		t.Fatalf("非数组形状应退 8（不给结论、不编字段），实得 %d", rc)
	}
}
