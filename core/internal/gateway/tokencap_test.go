// tokencap_test.go —— 2026-09-19 ④：ctx_window / max_tokens 没进"声明袋" ⇒ ctx/2 上限无人守。
//
// 真机现象：`gateway/fleet.yaml` 该条已声明 `ctx_window: 524288, max_tokens: 262144`；主控日志只有
// `temperature override 0.60` 与 `timeout override 300s`，**从来没有** `max_tokens override 262144`
// ⇒ 声明是死字段（解析侧连这一格都没有，yaml 里写了直接丢）。
//
// 本文件钉住（三条用例 + 上限优先级 + 声明袋装填，全部纯函数/无网络）：
//
//	① 调用方给了且 ≤ 上限 ⇒ **原样放行**（不再被无条件覆盖）；
//	② 调用方没给 ⇒ 用**默认**（现有动态额度，outbudget.DynamicMaxTokens）；
//	③ 调用方给了但 > 上限 ⇒ **钳到上限** + 日志 `max_tokens clamped: want=… cap=…`；
//	④ 上限来源优先级：档案（事实）> 声明（fleet）> 默认；
//	⑤ fleet 的 ctx_window / max_tokens 真的被装进声明袋（解析侧有字段、装填处有赋值）——
//	   这两格是 ④ 的"药引"，缺任何一半都会让它退回死字段。
package gateway

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// fleetWithDecl 造一份只带一条声明的 fleet 配置（单 dict 形态，与 fleet.yaml 里的写法同构）。
func fleetWithDecl(t *testing.T, model string, ctx, maxTokens int) *config.FleetConfig {
	t.Helper()
	yaml := "models:\n" +
		"  " + model + ":\n" +
		"    host: x3\n" +
		"    backend: ds4-server\n" +
		"    file: /data/models/x.gguf\n" +
		"    mem_gb: 86\n" +
		"    ctx_window: " + intYAML(ctx) + "\n" +
		"    max_tokens: " + intYAML(maxTokens) + "\n"
	cfg, err := config.ParseFleetConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("解析测试 fleet 配置失败: %v", err)
	}
	return cfg
}

func intYAML(v int) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ① 三条用例的纯逻辑面：原样 / 默认 / 钳位。
func TestClampMaxTokens_ThreeCases(t *testing.T) {
	cases := []struct {
		name    string
		want    int
		cap     int
		def     int
		value   int
		source  string
		clamped bool
	}{
		{"调用方给了且 ≤ 上限 ⇒ 原样放行", 4096, 262144, 32768, 4096, "调用方(原样)", false},
		{"调用方给了正好等于上限 ⇒ 原样", 262144, 262144, 32768, 262144, "调用方(原样)", false},
		{"调用方没给 ⇒ 用默认（动态额度）", 0, 262144, 32768, 32768, "默认(动态额度)", false},
		{"调用方给了但超上限 ⇒ 钳到上限", 999999, 262144, 32768, 262144, "钳位(上限)", true},
		{"谁都没给 ⇒ 默认上限兜底（cap=0）", 0, 0, 0, 0, "默认(动态额度)", false},
	}
	for _, c := range cases {
		got := ClampMaxTokens(c.want, c.cap, c.def)
		if got.Value != c.value || got.Source != c.source || got.Clamped != c.clamped {
			t.Errorf("%s：应 value=%d source=%s clamped=%v，实得 %+v",
				c.name, c.value, c.source, c.clamped, got)
		}
		if c.want > 0 && got.Want != c.want {
			t.Errorf("%s：应保留调用方原值 want=%d，实得 %d", c.name, c.want, got.Want)
		}
	}
	// 负额度不许出现（不编造一个负的 max_tokens）
	if got := ClampMaxTokens(0, 100, -5); got.Value != 0 {
		t.Errorf("默认额度为负时应落 0，实得 %d", got.Value)
	}
}

// ② 端到端（请求体层面）：三种形态都写回 body，且钳位必须留痕（日志里出现 clamped 行）。
func TestApplyTokenBudget_WritesBackAndLogsClamp(t *testing.T) {
	// ③ 超上限 ⇒ 钳位 + 日志
	var buf bytes.Buffer
	oldOut := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldOut)

	body := []byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}],"max_tokens":999999}`)
	res := ApplyTokenBudget(TokenBudgetInput{
		Model: "deepseek-v4-flash", ForwardBody: body,
		CtxWindow: 524288, CtxSource: "声明(意图)", Cap: 262144, CapSource: "声明(fleet)",
	})
	if !res.Limit.Clamped || res.Limit.Value != 262144 {
		t.Fatalf("超上限必须钳到 262144，实得 %+v", res.Limit)
	}
	var out map[string]any
	if err := json.Unmarshal(res.Body, &out); err != nil {
		t.Fatal(err)
	}
	if v, _ := out["max_tokens"].(float64); int(v) != 262144 {
		t.Errorf("body.max_tokens 应为 262144，实得 %v", out["max_tokens"])
	}
	if v, _ := out["max_output_tokens"].(float64); int(v) != 262144 {
		t.Errorf("body.max_output_tokens 也必须写（两种格式都要，v2.5.6 的教训）：%v", out["max_output_tokens"])
	}
	if logs := buf.String(); !strings.Contains(logs, "max_tokens clamped: want=999999 cap=262144") {
		t.Errorf("钳位必须留痕（真机症状就是「上限没生效却看不出来」）：日志=%q", logs)
	}

	// ① 给了且 ≤ 上限 ⇒ 原样（不被动态额度覆盖）
	body2 := []byte(`{"model":"deepseek-v4-flash","max_tokens":4096}`)
	res2 := ApplyTokenBudget(TokenBudgetInput{
		Model: "deepseek-v4-flash", ForwardBody: body2,
		CtxWindow: 524288, CtxSource: "声明(意图)", Cap: 262144, CapSource: "声明(fleet)",
	})
	if res2.Limit.Clamped || res2.Limit.Value != 4096 {
		t.Fatalf("调用方的 4096 ≤ 上限必须原样放行，实得 %+v", res2.Limit)
	}
	var out2 map[string]any
	if err := json.Unmarshal(res2.Body, &out2); err != nil {
		t.Fatal(err)
	}
	if v, _ := out2["max_tokens"].(float64); int(v) != 4096 {
		t.Errorf("原样放行后 body.max_tokens 应仍是 4096，实得 %v（旧行为：被无条件覆盖成动态额度）", out2["max_tokens"])
	}

	// ② 没给 ⇒ 用默认（动态额度；此处 ctx=524288 远大于 prompt ⇒ 应有正额度）
	body3 := []byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}]}`)
	res3 := ApplyTokenBudget(TokenBudgetInput{
		Model: "deepseek-v4-flash", ForwardBody: body3,
		CtxWindow: 524288, CtxSource: "声明(意图)", Cap: 262144, CapSource: "声明(fleet)",
	})
	if res3.Dyn <= 0 {
		t.Fatalf("ctx=524288 + 极小 prompt 应有正动态额度，实得 dyn=%d", res3.Dyn)
	}
	if res3.Limit.Value != res3.Dyn || res3.Limit.Source != "默认(动态额度)" {
		t.Fatalf("没给时应落默认（动态额度 %d），实得 %+v", res3.Dyn, res3.Limit)
	}
	var out3 map[string]any
	if err := json.Unmarshal(res3.Body, &out3); err != nil {
		t.Fatal(err)
	}
	if v, _ := out3["max_tokens"].(float64); int(v) != res3.Dyn {
		t.Errorf("body.max_tokens 应为动态额度 %d，实得 %v", res3.Dyn, out3["max_tokens"])
	}
}

// ④ 上限来源优先级：档案（事实）> 声明（fleet）> 默认。
func TestResolveTokenCap_Priority(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir)

	// 默认档（既无档案也无声明）
	if cap, src := resolveTokenCap("m", 0); cap != maxTokensCapDefault || src != "默认" {
		t.Fatalf("无档案无声明应落默认上限 %d，实得 %d[%s]", maxTokensCapDefault, cap, src)
	}
	// 声明档
	if cap, src := resolveTokenCap("m", 262144); cap != 262144 || src != "声明(fleet)" {
		t.Fatalf("有 fleet 声明时应取 262144[声明(fleet)]，实得 %d[%s]", cap, src)
	}
	// 档案档（事实优先于声明）：档案里写了 max_tokens ⇒ 它压过 fleet 声明
	if err := os.WriteFile(filepath.Join(dir, "m.yaml"), []byte("ctx_window: 262144\nmax_tokens: 65536\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if cap, src := resolveTokenCap("m", 262144); cap != 65536 || src != "档案(事实)" {
		t.Fatalf("档案（事实）必须压过声明（意图）：应取 65536[档案(事实)]，实得 %d[%s]", cap, src)
	}
}

// ⑤ 声明袋：fleet 的 ctx_window / max_tokens 真的进了袋子，且参与裁决。
func TestFleetDeclared_ParsesBothFields(t *testing.T) {
	cfg := fleetWithDecl(t, "deepseek-v4-flash", 524288, 262144)
	ctx, mt := FleetDeclared(cfg, "deepseek-v4-flash")
	if ctx != 524288 || mt != 262144 {
		t.Fatalf("fleet 声明应解析出 ctx=524288 max_tokens=262144（缺 max_tokens 字段 ⇒ 声明就是死字段），实得 ctx=%d mt=%d", ctx, mt)
	}
	if ctx, mt := FleetDeclared(nil, "x"); ctx != 0 || mt != 0 {
		t.Fatalf("nil 配置应如实回 (0,0)，实得 %d/%d", ctx, mt)
	}
	if ctx, mt := FleetDeclared(cfg, "不存在"); ctx != 0 || mt != 0 {
		t.Fatalf("未声明的模型应如实回 (0,0)，实得 %d/%d", ctx, mt)
	}
}

// ⑥ 整块覆盖（use mockAdapter from adapter_override_test.go）：声明进袋 → 上限钳位生效。
func TestApplyAdapterOverrides_DeclarationBagAndClamp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir) // 无档案 ⇒ 上限取 fleet 声明
	g := &Gateway{
		adapterRegistry: map[string]plugin.Plugin{"deepseek-v4-flash": &mockAdapter{}},
		config:          fleetWithDecl(t, "deepseek-v4-flash", 524288, 262144),
	}
	// 调用方给了 999999（> 声明上限 262144）⇒ 钳到 262144（注意 mockAdapter 自己只声明 32768，
	// 若 fleet 的声明没进袋，上限会退化成默认 32768 —— 用例据此分辨"声明有没有生效"）
	body := []byte(`{"model":"deepseek-v4-flash","max_tokens":999999,"messages":[{"role":"user","content":"hi"}]}`)
	out := g.ApplyAdapterOverrides("deepseek-v4-flash", body, &mockAdapter{})
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatal(err)
	}
	if v, _ := obj["max_tokens"].(float64); int(v) != 262144 {
		t.Fatalf("fleet 声明的上限必须生效（钳到 262144），实得 %v —— 声明进袋这一半没做的话会退化成默认上限", obj["max_tokens"])
	}
	if v, _ := obj["temperature"].(float64); v != 0.7 {
		t.Errorf("温度覆盖是既有行为，不许被 ④ 弄坏：实得 %v", obj["temperature"])
	}
	// 调用方没给 ⇒ 动态额度（默认档），而不是适配器写死值 32768 或声明值 262144
	body2 := []byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}]}`)
	out2 := g.ApplyAdapterOverrides("deepseek-v4-flash", body2, &mockAdapter{})
	var obj2 map[string]any
	if err := json.Unmarshal(out2, &obj2); err != nil {
		t.Fatal(err)
	}
	if v, _ := obj2["max_tokens"].(float64); int(v) == 262144 || v <= 0 {
		t.Fatalf("调用方没给时应落**动态额度**（不是声明值、不是写死值），实得 %v", obj2["max_tokens"])
	}
	// 超时覆盖未被破坏
	if g.timeoutOverride["deepseek-v4-flash"] != 120 {
		t.Errorf("超时覆盖是既有行为，不许被 ④ 弄坏：timeoutOverride=%v", g.timeoutOverride)
	}
}

// ⑦ 反例（真机形态）：**适配器什么也不声明**、只有 fleet 声明 max_tokens ⇒ 声明只能靠"装进声明袋"
// 才进得来（预算分支的入口就是袋子里那格），钳位才有依据。
//
// 这不是杜撰的形态：deepseek-v4-flash 的适配器（core/internal/plugin/adapters/ds4.go）里**没有**
// ctx_window / max_tokens 两格（真机日志"从来没有 max_tokens override 262144"正是这么来的）。
func TestApplyAdapterOverrides_FleetOnlyDeclarationStillClamps(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir) // 无档案 ⇒ 上限只能来自 fleet 声明
	ada := &bareAdapter{}
	g := &Gateway{
		adapterRegistry: map[string]plugin.Plugin{"deepseek-v4-flash": ada},
		config:          fleetWithDecl(t, "deepseek-v4-flash", 524288, 262144),
	}
	body := []byte(`{"model":"deepseek-v4-flash","max_tokens":999999,"messages":[{"role":"user","content":"hi"}]}`)
	out := g.ApplyAdapterOverrides("deepseek-v4-flash", body, ada)
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatal(err)
	}
	if v, _ := obj["max_tokens"].(float64); int(v) != 262144 {
		t.Fatalf("适配器不声明时，fleet 的 max_tokens=262144 必须经**声明袋**生效（钳到 262144）；"+
			"实得 %v —— 装袋那一半缺了的话这里会原样放行 999999（声明又变回死字段）", obj["max_tokens"])
	}
	if v, _ := obj["max_output_tokens"].(float64); int(v) != 262144 {
		t.Fatalf("两种格式都要写：max_output_tokens 实得 %v", obj["max_output_tokens"])
	}
}

// bareAdapter 只回温度、**不**声明 ctx_window / max_tokens（真机 ds4 适配器的形态）。
type bareAdapter struct{}

func (b *bareAdapter) Name() string                  { return "bare" }
func (b *bareAdapter) Type() plugin.PluginType       { return plugin.PluginTypeModelAdapter }
func (b *bareAdapter) Version() string               { return "1.0" }
func (b *bareAdapter) Capabilities() []string        { return []string{"model"} }
func (b *bareAdapter) Init(cfg map[string]any) error { return nil }
func (b *bareAdapter) Start() error                  { return nil }
func (b *bareAdapter) Stop() error                   { return nil }
func (b *bareAdapter) Close() error                  { return nil }
func (b *bareAdapter) GetDescriptions() string       { return "bare" }
func (b *bareAdapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	return plugin.PluginOutput{Result: map[string]any{"temperature": 0.6}}, nil
}
