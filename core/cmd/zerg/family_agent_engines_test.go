// family_agent_engines_test.go —— `zerg agent engines`（缺口 `GAP-20260927-04`）与
// `agent slots` 回据投影（缺口 `GAP-20260927-202` ②）的回归牙。
//
// 为什么要有它：这两条命令的「读懂回据」这半是全命令里唯一可用**回据原文**钉死的判据 ——
// 真打子端/引擎由 `agentStubChildForTest`（family_agent_bench_test.go 既有桩）替代，
// 断言落在「在跑的才算数、pid/端口/状态如实透出、空回据是**空结论**不是取不到、非数组不编字段」。
package main

import (
	"io"
	"net"
	"reflect"
	"strconv"
	"testing"
)

// r6TwoEggsBody —— 两枚在跑（port>0）+ 一枚登记但没跑（port=0）+ 每枚带 pid/unit_state。
const r6TwoEggsBody = `{"eggs":[
 {"egg_id":"gemma-4-26B","model":"gemma-4-26B","state":"idle_armed","unit_state":"active","port":9401,"pid":31337,"inflight":0,"managed":true},
 {"egg_id":"example-35b","model":"example-35b","state":"ready","unit_state":"active","port":9400,"pid":4242,"inflight":1,"managed":true,"argv":["llama-server","-m","/w/example-35b-v2.gguf","-c","131072","--reasoning-budget","256","--jinja"]},
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

// ⑤ GAP-20260926-01：/eggs 回据里的 argv **逐字**进 fact；缺格（没写 argv）⇒ 缺席（nil），渲染时**点名**。
func TestAgentEngineFacts_ArgvVerbatim(t *testing.T) {
	child := agentStubChildForTest(t, r6TwoEggsBody, statusTwoModels)
	facts, rc := agentEngineFacts(&invocation{}, child, io.Discard)
	if rc != exitOK {
		t.Fatalf("应退 0，实得 %d", rc)
	}
	byModel := map[string]agentEngineFact{}
	for _, f := range facts {
		byModel[f.Model] = f
	}
	orn := byModel["example-35b"]
	want := []string{"llama-server", "-m", "/w/example-35b-v2.gguf", "-c", "131072", "--reasoning-budget", "256", "--jinja"}
	if !reflect.DeepEqual(orn.Argv, want) {
		t.Fatalf("argv 应逐字透出：\n want=%#v\n got =%#v", want, orn.Argv)
	}
	if got, exp := agentRenderArgv(orn.Argv), `["llama-server","-m","/w/example-35b-v2.gguf","-c","131072","--reasoning-budget","256","--jinja"]`; got != exp {
		t.Fatalf("机读渲染应逐字：\n want=%q\n got =%q", exp, got)
	}
	// 负控①：gemma 那枚回据里**没有** argv 这一格 ⇒ 缺席（nil）+ 渲染**点名**（不静默当空）。
	gm := byModel["gemma-4-26B"]
	if gm.Argv != nil {
		t.Fatalf("缺格应缺席（nil），实得 %#v", gm.Argv)
	}
	if got := agentRenderArgv(gm.Argv); got != agentArgvAbsent {
		t.Fatalf("缺格渲染应点名 %q，实得 %q", agentArgvAbsent, got)
	}
}

// ⑤a 非法形状（argv 不是字符串数组）⇒ 认不出即缺席（不混编、不当空）。
func TestAgentEngineFacts_ArgvMalformedIsAbsent(t *testing.T) {
	body := `{"eggs":[{"egg_id":"m","model":"m","state":"ready","port":9400,"pid":1,"managed":true,"argv":[1,2,3]}]}`
	child := agentStubChildForTest(t, body, statusTwoModels)
	facts, rc := agentEngineFacts(&invocation{}, child, io.Discard)
	if rc != exitOK || len(facts) != 1 {
		t.Fatalf("应 1 枚（退 0），实得 rc=%d %+v", rc, facts)
	}
	if facts[0].Argv != nil {
		t.Fatalf("非法形状应缺席（nil），实得 %#v", facts[0].Argv)
	}
}

// ⑤b 负控②：子端**取不到** ⇒ blocked 退 8（不给结论，✗ 不许静默当空）。
func TestAgentEngineFacts_UnreachableIsBlocked(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占位监听失败：%v", err)
	}
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	p, _ := strconv.Atoi(portStr)
	_ = ln.Close() // 立刻放掉 ⇒ 这个端口上没人应答（子端取不到）
	inv := &invocation{}
	facts, rc := agentEngineFacts(inv, agentAddr{Host: "127.0.0.1", Port: p}, io.Discard)
	if rc != exitBlocked {
		t.Fatalf("取不到应退 8（blocked），实得 %d（facts=%+v）", rc, facts)
	}
	if facts != nil {
		t.Fatalf("取不到不该给结论，实得 %+v", facts)
	}
}

// ⑤c `--json` 字段闭集含 argv（合法）；闭集外仍按既有档退 2（点名由 reportBadField 负责）。
func TestAgentEnginesFields_ArgvLegal(t *testing.T) {
	if rc := agentCheckFields(&invocation{fields: []string{"argv"}}, io.Discard, agentEnginesFields); rc != exitOK {
		t.Fatalf("argv 应是合法机器面字段，实得 rc=%d", rc)
	}
	if rc := agentCheckFields(&invocation{fields: []string{"nosuch-zz"}}, io.Discard, agentEnginesFields); rc != exitUsage {
		t.Fatalf("闭集外字段应退 2（既有档），实得 rc=%d", rc)
	}
}

// ---- GAP-20260925-45：`agent engines` 的「引擎实时态」三格（inflight / last_output_ago_s / watchdog_verdict）----
//
// 病（改前现读）：子端 `/eggs` 已经透出 `inflight` 与 `watchdog` 子对象，而 `agent engines` **把它们丢掉**
// ⇒ 「这台机的引擎此刻在不在算、多久没出声、看门狗怎么判的」在机面上看不见（区分不了「挂住」与「只是慢」）。
//
// 三条判据（都在**合成**面上跑 · ✗ 不连真集群 · ✗ 不改生产状态）：
//
//	正控：正常载荷 ⇒ 三格读得到、数值**逐字相符**（在算 inflight=1 / 出过 last_output_ago_s=12.5 / 判词 extended）。
//	负控一：喂 `verdict:"stuck_no_progress"` / `last_output_ago_s:813.6` ⇒ 须**如实报该值**（不是恒 `ok`/`0`）。
//	负控二：子端取不到（打不到）⇒ **不给结论**（退码 8 · facts=nil），✗ 不许静默当空。

// realtimeTwoEggsBody —— 两枚在跑：一枚正常有输出（在算/出过），一枚看门狗判「无进展卡住」。
const realtimeTwoEggsBody = `{"eggs":[
 {"egg_id":"example-35b","model":"example-35b","state":"ready","port":9400,"pid":4242,"inflight":1,"managed":true,"watchdog":{"window_idx":1,"extensions":0,"last_output_ago_s":12.5,"cpu_usage_usec_delta":900000,"verdict":"extended","reason":"合成载荷"}},
 {"egg_id":"gemma-4-26B","model":"gemma-4-26B","state":"ready","port":9401,"pid":31337,"inflight":1,"managed":true,"watchdog":{"window_idx":3,"extensions":0,"last_output_ago_s":813.6,"cpu_usage_usec_delta":0,"verdict":"stuck_no_progress","reason":"合成载荷"}}
]}`

// 正控：`inflight` / `last_output_ago_s` / `watchdog_verdict` 三格逐字，且在算/出过/判词三类各自可辨。
func TestAgentEngineRealtime_Verbatim(t *testing.T) {
	child := agentStubChildForTest(t, realtimeTwoEggsBody, statusTwoModels)
	facts, rc := agentEngineFacts(&invocation{}, child, io.Discard)
	if rc != exitOK {
		t.Fatalf("应退 0，实得 %d", rc)
	}
	byModel := map[string]agentEngineFact{}
	for _, f := range facts {
		byModel[f.Model] = f
	}
	orn := byModel["example-35b"]
	if orn.Inflight == nil || *orn.Inflight != 1 {
		t.Fatalf("在算与否（inflight）要逐字 1，实得 %v", orn.Inflight)
	}
	if orn.Watchdog == nil || orn.Watchdog.LastOutputAgoS != "12.5" || orn.Watchdog.Verdict != "extended" {
		t.Fatalf("出过首 token 与判词两格要逐字 12.5/extended，实得 %+v", orn.Watchdog)
	}
	// 行面（--json 所投影的那一行）同样逐字 —— 三格都在。
	row := agentEngineRow("Mr2109", orn)
	if row[agentEngineFieldInflight] != "1" || row[agentEngineFieldLastOutputAgoS] != "12.5" || row[agentEngineFieldWatchdogVerdict] != "extended" {
		t.Fatalf("行面三格应逐字 inflight=1/last_output_ago_s=12.5/watchdog_verdict=extended，实得 %+v", row)
	}
	if got := marshalObjectRegistered([]string{"machine", "inflight", "last_output_ago_s", "watchdog_verdict"}, row); got != `{"machine":"Mr2109","inflight":"1","last_output_ago_s":"12.5","watchdog_verdict":"extended"}` {
		t.Fatalf("机器面 JSON 应逐字，实得 %s", got)
	}
}

// 负控一：看门狗判「卡住」⇒ 须如实报 `stuck_no_progress` / `813.6`（不是恒 ok / 恒 0）。
func TestAgentEngineRealtime_StuckMustReport(t *testing.T) {
	child := agentStubChildForTest(t, realtimeTwoEggsBody, statusTwoModels)
	facts, rc := agentEngineFacts(&invocation{}, child, io.Discard)
	if rc != exitOK {
		t.Fatalf("应退 0，实得 %d", rc)
	}
	var stuck *agentEngineFact
	for i := range facts {
		if facts[i].Model == "gemma-4-26B" {
			stuck = &facts[i]
		}
	}
	if stuck == nil || stuck.Watchdog == nil {
		t.Fatalf("挂住那一枚没带上 watchdog：%+v", facts)
	}
	if stuck.Watchdog.Verdict != "stuck_no_progress" {
		t.Errorf("负控一：判词要如实报 `stuck_no_progress`（不许恒 ok），实得 %q", stuck.Watchdog.Verdict)
	}
	if stuck.Watchdog.LastOutputAgoS != "813.6" {
		t.Errorf("负控一：静默读数要逐字 `813.6`，实得 %q", stuck.Watchdog.LastOutputAgoS)
	}
	if stuck.Watchdog.Verdict == "ok" || stuck.Watchdog.LastOutputAgoS == "0" {
		t.Errorf("负控一：把卡住读成 ok/0 = 本缺口的原现象：%+v", stuck.Watchdog)
	}
}

// 负控二：子端取不到 ⇒ 不给结论（退码 8 · facts=nil），✗ 不许静默当空。
func TestAgentEngineRealtime_UnreachableIsBlocked(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占位监听失败：%v", err)
	}
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	p, _ := strconv.Atoi(portStr)
	_ = ln.Close() // 立刻放掉 ⇒ 这个端口上没人应答（子端取不到）
	facts, rc := agentEngineFacts(&invocation{}, agentAddr{Host: "127.0.0.1", Port: p}, io.Discard)
	if rc != exitBlocked {
		t.Fatalf("取不到应退 8（blocked），实得 %d（facts=%+v）", rc, facts)
	}
	if facts != nil {
		t.Fatalf("取不到不该给结论，实得 %+v", facts)
	}
}

// 负控三（缺观测不编）：回据里没有 `watchdog` / 没有 `inflight` ⇒ 对应格**缺席**（键不出现），✗ 不许编 ok/0。
func TestAgentEngineRealtime_AbsentMustOmit(t *testing.T) {
	// (a) 有 inflight 但没 watchdog ⇒ watchdog 两格缺席；inflight=0 是**真读数**要逐字照报。
	body := `{"eggs":[{"egg_id":"m","model":"m","state":"ready","port":9400,"pid":1,"inflight":0,"managed":true}]}`
	facts, rc := agentEngineFacts(&invocation{}, agentStubChildForTest(t, body, statusTwoModels), io.Discard)
	if rc != exitOK || len(facts) != 1 {
		t.Fatalf("应 1 枚（退 0），实得 rc=%d %+v", rc, facts)
	}
	row := agentEngineRow("Mr2109", facts[0])
	if _, ok := row[agentEngineFieldWatchdogVerdict]; ok {
		t.Errorf("回据没 watchdog ⇒ watchdog_verdict 键不该出现（不许编 ok），实得 %v", row)
	}
	if _, ok := row[agentEngineFieldLastOutputAgoS]; ok {
		t.Errorf("回据没 watchdog ⇒ last_output_ago_s 键不该出现（不许编 0），实得 %v", row)
	}
	if v, ok := row[agentEngineFieldInflight]; !ok || v != "0" {
		t.Errorf("回据报了 inflight=0 ⇒ 要逐字 `0`（真读数，不并进「缺席」），实得 %q ok=%v", v, ok)
	}
	// (b) `/status` 兜底那一枚回据里**没有** `inflight` ⇒ 该格缺席（✗ 不许编 0）。
	child := agentStubChildForTest(t, `{"eggs":[]}`,
		`{"machine":"Mr2109","model":"m","port":9400,"backend_state":"ready","pid":4242}`)
	facts, rc = agentEngineFacts(&invocation{}, child, io.Discard)
	if rc != exitOK || len(facts) != 1 || facts[0].Via != "/status" {
		t.Fatalf("/status 兜底应出 1 枚（退 0），实得 rc=%d %+v", rc, facts)
	}
	row = agentEngineRow("Mr2109", facts[0])
	if _, ok := row[agentEngineFieldInflight]; ok {
		t.Errorf("/status 兜底没有 inflight ⇒ 键不该出现（不许编 0），实得 %v", row)
	}
}

// 可选字段层（optFields）：三格走**可选层**（出口白名单收下、冻结 `agentEnginesFields` 不含）。
func TestAgentEnginesRealtime_OptionalFieldsLayer(t *testing.T) {
	for _, f := range []string{agentEngineFieldInflight, agentEngineFieldWatchdogVerdict, agentEngineFieldLastOutputAgoS} {
		if rc := agentCheckFields(&invocation{fields: []string{f}}, io.Discard, agentEnginesJSONFields); rc != exitOK {
			t.Errorf("可选层字段 %q 应被收下，实得 rc=%d", f, rc)
		}
		if rc := agentCheckFields(&invocation{fields: []string{f}}, io.Discard, agentEnginesFields); rc != exitUsage {
			t.Errorf("可选层字段 %q 不该进冻结表 `agentEnginesFields`，实得 rc=%d", f, rc)
		}
	}
}
