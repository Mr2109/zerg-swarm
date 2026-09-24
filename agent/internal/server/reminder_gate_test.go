package server

import (
	"bytes"
	"testing"
)

// 判据件（v2.5.12 · 批三 · H1 修法）：卵侧「重提示重跑」默认关 + 无工具不注入。
//
// 病象（排查稿 §③H1 / 本单改前实测）：经网关发一发**没有工具**的最小请求，卵会把一句英文
// reminder 追加进发给模型的请求体、**再跑一次推理**，把**第二次**的回答回给客户端 ⇒
// ① 客户端拿不到自己那一问的答案 ② 引擎侧 prompt_tokens 45（干净直连 21）③ 正文与 think 同字。
// 修法两道闸：① `ZERG_TOOL_REMINDER` 未点名 ⇒ 不注入（默认关）② 请求体没声明工具 ⇒ 不注入。
//
// 变异自检口径：删掉 maybeRemind 里的闸① ⇒ TestMaybeRemind_DefaultOff_NoInjection 变红；
// 删掉闸② ⇒ TestMaybeRemind_EnabledButNoTools_NoInjection 变红（两条都已实测，见回执）。

// remindBodyNoTools 复刻本单探针的请求体：单条 user、**没有 tools**。
const remindBodyNoTools = `{"model":"example-35b-v2","messages":[{"role":"user","content":"只回一行：ZERGB1-枫-在线"}],"stream":false,"max_tokens":40}`

// remindBodyWithTools 同形但**声明了一只工具**（旧机制的适用场景）。
const remindBodyWithTools = `{"model":"example-35b-v2","messages":[{"role":"user","content":"查一下今天的日志"}],"tools":[{"type":"function","function":{"name":"read_file","parameters":{"type":"object"}}}],"stream":false}`

// remindRespIntent 复刻「该调不调」的短答：含意图词 let me（触发旧机制的条件 3）。
const remindRespIntent = `{"choices":[{"finish_reason":"stop","index":0,"message":{"role":"assistant","content":"Let me first read the log file."}}],"usage":{"prompt_tokens":21}}`

// TestMaybeRemind_DefaultOff_NoInjection —— 闸①：没有点名开 ⇒ 一律不注入（**默认路径**）。
func TestMaybeRemind_DefaultOff_NoInjection(t *testing.T) {
	t.Setenv("ZERG_TOOL_REMINDER", "") // 显式清空 = 默认态；即便机器上设过也不影响本用例
	s := &Server{}
	got, should := s.maybeRemind("example-35b-v2", []byte(remindBodyWithTools), []byte(remindRespIntent))
	if should {
		t.Fatalf("闸①失守：默认关却仍注入了重提示 ⇒ 请求体被改写\nbody=%s", got)
	}
	if got != nil {
		t.Fatalf("闸①失守：should=false 却回带了改写后的体：%s", got)
	}
}

// TestMaybeRemind_EnabledButNoTools_NoInjection —— 闸②：点名开了，但客户端没声明工具 ⇒ 仍不注入。
func TestMaybeRemind_EnabledButNoTools_NoInjection(t *testing.T) {
	t.Setenv("ZERG_TOOL_REMINDER", "1")
	s := &Server{}
	got, should := s.maybeRemind("example-35b-v2", []byte(remindBodyNoTools), []byte(remindRespIntent))
	if should {
		t.Fatalf("闸②失守：请求体没有 tools 却注入了重提示 ⇒ 请求体被改写\nbody=%s", got)
	}
	if got != nil {
		t.Fatalf("闸②失守：should=false 却回带了改写后的体：%s", got)
	}
}

// TestMaybeRemind_EnabledWithTools_Injects —— 两道闸都过 ⇒ 逐字恢复旧行为（可关可开里的「可开」）。
func TestMaybeRemind_EnabledWithTools_Injects(t *testing.T) {
	t.Setenv("ZERG_TOOL_REMINDER", "1")
	s := &Server{}
	got, should := s.maybeRemind("example-35b-v2", []byte(remindBodyWithTools), []byte(remindRespIntent))
	if !should {
		t.Fatal("点名开 + 有工具 + 短答含意图 ⇒ 应注入，实得 should=false")
	}
	if !bytes.Contains(got, []byte("Always call tools directly")) {
		t.Fatalf("注入体里没有 reminder 原文：%s", got)
	}
	// 原消息必须**逐字保留**（只追加，不改写既有内容）
	if !bytes.Contains(got, []byte("查一下今天的日志")) {
		t.Fatalf("注入体把原消息改掉了：%s", got)
	}
	if !bytes.Contains(got, []byte(`"role":"user"`)) {
		t.Fatalf("注入体里没有追加的 user 消息：%s", got)
	}
}

// TestMaybeRemind_PerModelOptIn —— 「按模型逐枚开」：点名别的模型 ⇒ 本模型仍关。
func TestMaybeRemind_PerModelOptIn(t *testing.T) {
	s := &Server{}
	for _, tc := range []struct {
		env   string
		model string
		want  bool
	}{
		{"qwen3.6", "example-35b-v2", false}, // 点名了别人 ⇒ 关
		{"example-35b-v2", "example-35b-v2", true},   // 点名了本模型（前缀口径）⇒ 开
		{"qwen3.6, example-35b-v2", "example-35b-v2", true},
		{"*", "example-35b-v2", true},
		{"all", "example-35b-v2", true},
		{"", "example-35b-v2", false},
	} {
		t.Setenv("ZERG_TOOL_REMINDER", tc.env)
		if _, should := s.maybeRemind(tc.model, []byte(remindBodyWithTools), []byte(remindRespIntent)); should != tc.want {
			t.Fatalf("ZERG_TOOL_REMINDER=%q model=%s ⇒ 实得 should=%v，要 %v", tc.env, tc.model, should, tc.want)
		}
	}
}

// TestMaybeRemind_NonReminderModel_Unchanged —— 别的模型（ds4：NeedsToolReminder()==false）逐字不变。
func TestMaybeRemind_NonReminderModel_Unchanged(t *testing.T) {
	t.Setenv("ZERG_TOOL_REMINDER", "1")
	s := &Server{}
	body := []byte(`{"model":"ds4","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"x"}}]}`)
	if _, should := s.maybeRemind("ds4", body, []byte(remindRespIntent)); should {
		t.Fatal("ds4 的 NeedsToolReminder()==false ⇒ 永不注入，实得 should=true")
	}
}

// TestBodyDefinesTools —— 工具声明的读法（chat 的 tools / functions；读不出 ⇒ fail-closed）。
func TestBodyDefinesTools(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"无 tools", remindBodyNoTools, false},
		{"空 tools 数组", `{"messages":[],"tools":[]}`, false},
		{"有 tools", remindBodyWithTools, true},
		{"有 functions（旧字段）", `{"messages":[],"functions":[{"name":"x"}]}`, true},
		{"tools 不是数组", `{"messages":[],"tools":"x"}`, false},
		{"体读不出（非 JSON）", `not json`, false},
	} {
		if got := bodyDefinesTools([]byte(tc.body)); got != tc.want {
			t.Fatalf("%s：实得 %v，要 %v", tc.name, got, tc.want)
		}
	}
}
