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
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
	"github.com/Mr2109/zerg-swarm/core/internal/plugin/adapters"
)

// fleetWithDecl 造一份只带一条声明的 fleet 配置（单 dict 形态，与 fleet.yaml 里的写法同构）。
func fleetWithDecl(t *testing.T, model string, ctx, maxTokens int) *config.FleetConfig {
	t.Helper()
	yaml := "models:\n" +
		"  " + model + ":\n" +
		"    host: node-a\n" +
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
	out, _ := g.ApplyAdapterOverrides("deepseek-v4-flash", body, &mockAdapter{})
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
	out2, _ := g.ApplyAdapterOverrides("deepseek-v4-flash", body2, &mockAdapter{})
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
	out, _ := g.ApplyAdapterOverrides("deepseek-v4-flash", body, ada)
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

// ── ⑧ v2.5.12（2026-09-24 · 排查稿 `排查-卵子代理回声与会话串话-v2.5.12-20260924.md` §③ H4）─────────
//
// 病象（逐字）：真机**每一发** example-35b-v2 请求都打
//
//	`⚠️ adapter example-35b-v2 execution failed (using defaults): example-35b-v2: not started`
//
// ⇒ `ApplyAdapterOverrides` 走 `aerr != nil` 那条路**原样返回 forwardBody** ⇒ 温度/max_tokens 一个都不下发
// （`gateway/fleet.yaml` 里那句「适配器 max_tokens/thinking 已控」是死声明）。
//
// 两处修正（都在本批）：① 装配链补 `Start()`（`core/cmd/zerg-core/main.go`，判据在 adapters 包的
// `TestAdapterAssembly_InitsAndStarts`）② 声明袋调用要带 `prompt`（本函数上方那一段）。
// 本用例是它们在**网关这一侧**的离线判据 —— 真端到端（主控日志里那行告警归零）需重启主控，本单禁重启。
func TestApplyAdapterOverrides_OrnithStartedDeclarationReachesBody(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir) // 无档案 ⇒ 上限只用声明，不受本机档案干扰
	ada := adapters.NewExampleAdapter()    // 与 main.go 注册表同一个构造器
	if err := ada.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	g := &Gateway{
		adapterRegistry: map[string]plugin.Plugin{"example-35b-v2": ada},
		config:          fleetWithDecl(t, "example-35b-v2", 262144, 0),
	}
	body := []byte(`{"model":"example-35b-v2","messages":[{"role":"user","content":"只回一行：甲-在线"}],"stream":false}`)

	// 反例（**修复前的真机形态**：注册表只 Init 没 Start）⇒ 一个字段都不许进体。
	out0, _ := g.ApplyAdapterOverrides("example-35b-v2", body, ada)
	var obj0 map[string]any
	if err := json.Unmarshal(out0, &obj0); err != nil {
		t.Fatalf("反例输出不是合法 JSON：%v", err)
	}
	if _, has := obj0["temperature"]; has {
		t.Fatalf("没 Start 时适配器必须报 not started、原样返回 body —— 现在温度却进体了：%s", out0)
	}
	if _, has := obj0["max_tokens"]; has {
		t.Fatalf("没 Start 时不许写 max_tokens（真机那行 'using defaults' 就是这个分支）：%s", out0)
	}

	// 正例（装配链补上 Start 之后的口径）⇒ 温度 0.8 与 max_tokens 预算必须真进体。
	if err := ada.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	out1, _ := g.ApplyAdapterOverrides("example-35b-v2", body, ada)
	var obj1 map[string]any
	if err := json.Unmarshal(out1, &obj1); err != nil {
		t.Fatalf("正例输出不是合法 JSON：%v", err)
	}
	if v, _ := obj1["temperature"].(float64); v != 0.8 {
		t.Errorf("声明袋里的温度没落进请求体（want 0.8）：实得 %v —— body=%s", obj1["temperature"], out1)
	}
	if v, _ := obj1["max_tokens"].(float64); v <= 0 {
		t.Errorf("max_tokens 预算没进体（want > 0）：实得 %v —— body=%s", obj1["max_tokens"], out1)
	}
	// 调用方没给 max_tokens ⇒ 取动态额度；给 32768 是适配器声明、不是写死 —— 不许原样留空
	if _, has := obj1["max_tokens"]; !has {
		t.Errorf("max_tokens 这一格必须出现（预算分支的产出）：%s", out1)
	}
}

// promptStrictAdapter 精确模拟 example-35b-v2 的**形态门**：`Context` 里没 prompt 就报错。
// 用途：钉住「声明袋调用（ApplyAdapterOverrides）必须把 prompt 给进去」——把上游那行 Context 去掉 ⇒ 本用例红。
type promptStrictAdapter struct{}

func (p *promptStrictAdapter) Name() string                  { return "prompt-strict" }
func (p *promptStrictAdapter) Type() plugin.PluginType       { return plugin.PluginTypeModelAdapter }
func (p *promptStrictAdapter) Version() string               { return "1.0" }
func (p *promptStrictAdapter) Capabilities() []string        { return []string{"model"} }
func (p *promptStrictAdapter) Init(cfg map[string]any) error { return nil }
func (p *promptStrictAdapter) Start() error                  { return nil }
func (p *promptStrictAdapter) Stop() error                   { return nil }
func (p *promptStrictAdapter) Close() error                  { return nil }
func (p *promptStrictAdapter) GetDescriptions() string       { return "prompt-strict" }
func (p *promptStrictAdapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	if s, _ := input.Context["prompt"].(string); s == "" {
		return plugin.PluginOutput{}, errPromptStrict
	}
	return plugin.PluginOutput{Result: map[string]any{"temperature": 0.8, "max_tokens": 32768}}, nil
}

var errPromptStrict = errors.New("prompt-strict: prompt is empty")

func TestApplyAdapterOverrides_BagCallCarriesPrompt(t *testing.T) {
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir())
	ada := &promptStrictAdapter{}
	g := &Gateway{
		adapterRegistry: map[string]plugin.Plugin{"prompt-strict": ada},
		config:          fleetWithDecl(t, "prompt-strict", 262144, 0),
	}
	body := []byte(`{"model":"prompt-strict","messages":[{"role":"user","content":"只回一行：甲-在线"}]}`)
	out, _ := g.ApplyAdapterOverrides("prompt-strict", body, ada)
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("输出不是合法 JSON：%v", err)
	}
	if v, _ := obj["temperature"].(float64); v != 0.8 {
		t.Fatalf("声明袋调用没带 prompt ⇒ 适配器报错 ⇒ 声明一个都不下发（学 example-35b-v2 的形态门）：实得 %v", obj["temperature"])
	}
	// 空 messages 的 body：tokenize 不出 prompt ⇒ 适配器照旧失败、body 原样 —— 与修复前同形（不静默改行为）
	out2, _ := g.ApplyAdapterOverrides("prompt-strict", []byte(`{"model":"prompt-strict","messages":[]}`), ada)
	if strings.Contains(string(out2), `"temperature"`) {
		t.Fatalf("取不到 prompt 时不许凭空造一个（应原样返回 body）：%s", out2)
	}
}

// ── ⑨ 输出预算 fail-closed（一号修法 · 2026-10-16）────────────────────────────────────
//
// 病象（真机逐字）：本地卵 Qwen3.8-Flash-Next@edge-node（引擎 `-c 32768`）干「读 1448 行缺口账」的活时，
// 第 2–14 发网关只打一行「⚠️ 输出预算：提示已超上下文（ctx=32768…）——本次不给输出额度（调用方原值
// 仍按上限口径放行）」⇒ **那 13 发照发**、唯一上限＝引擎 `-c`；末发 usage 恰撞顶
// （completion=111 / prompt=32657 / total=32768）且正文为空（111 token 极可能全是思考）⇒ 模型在
// 思考段被 ctx 硬切 ⇒ 子代理循环把那截断文本当「无工具调用的最终答复」⇒ 判 completed ⇒
// **状态假成功、产物 0 字节**。
//
// 本组用例钉**网关侧**那一半（fail-closed）：① 超上下文 ⇒ 拒发；② 剩余不足 ⇒ 拒发；
// ③ 额度充足 ⇒ 与改前逐字同；④ 开关 = `open` ⇒ 旧行为逐字同（④ 用本波新落的常量，见文件末）。
//
// ★ 拒发的**可观测面就是 Body**：拒发 ⇒ 一个字节都不改（✗ 不进任何输出额度、✗ 不改写调用方原值）
// —— 旧档的特征恰恰是「改写 + 补 max_output_tokens 那一格」，两态据此可分辨。

// outbudgetBody —— 造一个长度**恰为** want 字节的请求体（content 全是 ASCII 'x' ⇒ 无转义，长度线性可控）。
// callerMax > 0 ⇒ 带上调用方给的那一格 max_tokens（「调用方给了原值」的形态 —— 真机首发就是它）。
func outbudgetBody(t *testing.T, want, callerMax int) []byte {
	t.Helper()
	field := ""
	if callerMax > 0 {
		field = `"max_tokens":` + intYAML(callerMax) + `,`
	}
	pre := `{"model":"m",` + field + `"messages":[{"role":"user","content":"`
	post := `"}]}`
	n := want - len(pre) - len(post)
	if n < 0 {
		t.Fatalf("夹具 want=%d 比骨架（%d 字节）还短", want, len(pre)+len(post))
	}
	b := []byte(pre + strings.Repeat("x", n) + post)
	if len(b) != want {
		t.Fatalf("夹具长度没对齐：want=%d 实得=%d", want, len(b))
	}
	return b
}

// ① 提示已超上下文 ⇒ **拒发**：Body 逐字节不动（旧档：调用方原值 22438 原样放行 + 补 max_output_tokens）。
func TestOutbudgetFailClosed_OverCtxNoPassThrough(t *testing.T) {
	// ctx=8192：reserve=max(819,2048)=2048 ⇒ 只要 prompt≈>6144 就已超上下文。
	body := outbudgetBody(t, 26000, 22438) // prompt≈6500 ⇒ avail = 8192−6500−2048 < 0
	pe := EstimatePromptTokens(len(body))
	if pe <= 8192-2048 {
		t.Fatalf("夹具没造出「已超上下文」：prompt≈%d（要 > %d）", pe, 8192-2048)
	}
	res := ApplyTokenBudget(TokenBudgetInput{
		Model: "m", ForwardBody: body, CtxWindow: 8192, CtxSource: "档案(事实)",
		Cap: 32768, CapSource: "默认",
	})
	if !bytes.Equal(res.Body, body) {
		t.Fatalf("提示已超上下文 ⇒ 必须拒发（Body 一个字节都不许改）；实得被改写成：%s…", res.Body[:min(80, len(res.Body))])
	}
	if bytes.Contains(res.Body, []byte("max_output_tokens")) {
		t.Fatalf("拒发不许补任何输出额度字段（旧档正是在这里补上 max_output_tokens）：%s", res.Body)
	}
}

// ② 剩余输出空间不足（0 < avail < outBudgetMinAvail）⇒ **拒发**：Body 逐字节不动
// （旧档会给一份被 minOutTokens 兜底成 1024 的额度 —— 比真实剩余空间大得多）。
func TestOutbudgetFailClosed_RemainderBelowMinNoPassThrough(t *testing.T) {
	// ctx=4096：reserve=max(409,2048)=2048；prompt≈1750 ⇒ avail ≈ 298（>0 且远小于 512 的量级）。
	body := outbudgetBody(t, 7000, 0)
	pe := EstimatePromptTokens(len(body))
	if pe <= 4096-2048-512 || pe >= 4096-2048 {
		t.Fatalf("夹具没落进「剩余不足」那一档：prompt≈%d（要在 %d~%d 之间）", pe, 4096-2048-512, 4096-2048)
	}
	res := ApplyTokenBudget(TokenBudgetInput{
		Model: "m", ForwardBody: body, CtxWindow: 4096, CtxSource: "档案(事实)",
		Cap: 32768, CapSource: "默认",
	})
	if !bytes.Equal(res.Body, body) {
		t.Fatalf("剩余空间不足 ⇒ 必须拒发（Body 一个字节都不许改）；实得被改写成：%s…", res.Body[:min(80, len(res.Body))])
	}
}

// ③ 额度充足 ⇒ 与改前**逐字同**（本波不许动正常档：调用方原样 / 没给用动态额度 / 超了钳位）。
func TestOutbudgetFailClosed_NormalBudgetUnchanged(t *testing.T) {
	body := outbudgetBody(t, 400, 4096) // ctx=131072 ⇒ 额度充足
	res := ApplyTokenBudget(TokenBudgetInput{
		Model: "m", ForwardBody: body, CtxWindow: 131072, CtxSource: "声明(意图)",
		Cap: maxTokensCapDefault, CapSource: "默认",
	})
	var obj map[string]any
	if err := json.Unmarshal(res.Body, &obj); err != nil {
		t.Fatal(err)
	}
	if v, _ := obj["max_tokens"].(float64); int(v) != 4096 {
		t.Fatalf("正常档：调用方的 4096 ≤ 上限必须原样放行，实得 %v", obj["max_tokens"])
	}
	if v, _ := obj["max_output_tokens"].(float64); int(v) != 4096 {
		t.Fatalf("正常档：max_output_tokens 也必须写（两种格式都要），实得 %v", obj["max_output_tokens"])
	}
	if res.Limit.Clamped || res.Limit.Source != "调用方(原样)" {
		t.Fatalf("正常档的裁决不许变：%+v", res.Limit)
	}
	if want := ClampMaxTokens(4096, maxTokensCapDefault, res.Dyn); res.Limit != want {
		t.Fatalf("正常档须与 ClampMaxTokens 逐字同源：want=%+v got=%+v", want, res.Limit)
	}
}

// ④ 开关两态：默认（未设置）/ 任何**不是恰 `open`** 的取值 ⇒ closed（拒发）；
//
//	恰 `open` ⇒ 旧行为**逐字同**（放行 ＋ 那行 ⚠️ 日志 ＋ 调用方原值按上限口径进体）。
func TestOutbudgetFailClosed_SwitchTwoStates(t *testing.T) {
	// 真机那一发的形态：ctx=8192、prompt≈6500（已超上下文）、调用方给了 22438。
	body := outbudgetBody(t, 26000, 22438)
	in := TokenBudgetInput{Model: "m", ForwardBody: body, CtxWindow: 8192, CtxSource: "档案(事实)",
		Cap: 32768, CapSource: "默认"}

	// ① 未设置（本机默认档）⇒ closed ⇒ 拒发
	if got := ApplyTokenBudget(in); !got.Rejected {
		t.Fatalf("默认档必须 fail-closed（拒发），实得 Rejected=%v", got.Rejected)
	}
	// ② 非「恰 open」的各种写法 ⇒ 仍是 closed（判不了就按拒发 —— fail-closed 方向偏安全）
	for _, v := range []string{"", "closed", "closed ", "OPEN", "Open", "open ", " open", "1", "yes"} {
		t.Setenv(OutbudgetFailModeEnv, v)
		if got := ApplyTokenBudget(in); !got.Rejected {
			t.Fatalf("开关=%q 不是「恰 open」⇒ 必须仍拒发，实得 Rejected=false", v)
		}
	}
	// ③ **恰 open** ⇒ 旧行为逐字同
	t.Setenv(OutbudgetFailModeEnv, "open")
	var buf bytes.Buffer
	oldOut := log.Writer()
	log.SetOutput(&buf)
	got := ApplyTokenBudget(in)
	log.SetOutput(oldOut)
	if got.Rejected || got.RejectCode != "" || got.RejectReason != "" {
		t.Fatalf("开关=open 必须退回旧档（不拒发、不带码），实得 %+v", got)
	}
	var obj map[string]any
	if err := json.Unmarshal(got.Body, &obj); err != nil {
		t.Fatal(err)
	}
	if v, _ := obj["max_tokens"].(float64); int(v) != 22438 {
		t.Fatalf("旧档：调用方原值 22438 按上限口径原样放行，实得 %v", obj["max_tokens"])
	}
	if v, _ := obj["max_output_tokens"].(float64); int(v) != 22438 {
		t.Fatalf("旧档：两种格式都要写，max_output_tokens 实得 %v", obj["max_output_tokens"])
	}
	want := "⚠️ 输出预算：提示已超上下文（ctx=8192 来源=档案(事实) prompt≈6500）——本次不给输出额度" +
		"（调用方原值仍按上限口径放行；不再无条件覆盖）"
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("旧档那行日志必须逐字不变：\nwant=%s\ngot=%s", want, buf.String())
	}
}

// ⑤ 拒发的**回报面**：Rejected ＋ 机器可识别错误码 ＋ 可读原因（✗ 不是只写一行日志）。
func TestOutbudgetFailClosed_RejectCodeAndReason(t *testing.T) {
	// 甲：提示已超上下文（ctx=8192 / prompt≈6500）
	bodyA := outbudgetBody(t, 26000, 22438)
	gotA := ApplyTokenBudget(TokenBudgetInput{Model: "m", ForwardBody: bodyA, CtxWindow: 8192,
		CtxSource: "档案(事实)", Cap: 32768, CapSource: "默认"})
	if !gotA.Rejected || gotA.RejectCode != outBudgetRejectCodeOverCtx {
		t.Fatalf("超上下文 ⇒ Rejected=true 且 code=%q，实得 %+v", outBudgetRejectCodeOverCtx, gotA)
	}
	for _, seg := range []string{"输出预算", "提示已超上下文", "ctx=8192", "prompt≈6500", "本次拒发", "fail-closed"} {
		if !strings.Contains(gotA.RejectReason, seg) {
			t.Fatalf("拒发原因必须逐字可读（缺 %q）：%s", seg, gotA.RejectReason)
		}
	}
	if !bytes.Equal(gotA.Body, bodyA) {
		t.Fatalf("拒发 ⇒ Body 一个字节都不许动：%s", gotA.Body[:min(80, len(gotA.Body))])
	}

	// 乙：剩余输出空间不足（0 < avail < outBudgetMinAvail，报话里带阈值）
	bodyB := outbudgetBody(t, 7000, 0)
	gotB := ApplyTokenBudget(TokenBudgetInput{Model: "m", ForwardBody: bodyB, CtxWindow: 4096,
		CtxSource: "档案(事实)", Cap: 32768, CapSource: "默认"})
	if !gotB.Rejected || gotB.RejectCode != outBudgetRejectCodeRemainder {
		t.Fatalf("剩余不足 ⇒ Rejected=true 且 code=%q，实得 %+v", outBudgetRejectCodeRemainder, gotB)
	}
	for _, seg := range []string{"剩余输出额度不足", "剩余≈298", "< 512", "本次拒发", "fail-closed"} {
		if !strings.Contains(gotB.RejectReason, seg) {
			t.Fatalf("拒发原因必须逐字可读（缺 %q）：%s", seg, gotB.RejectReason)
		}
	}
	if !bytes.Equal(gotB.Body, bodyB) {
		t.Fatalf("拒发 ⇒ Body 一个字节都不许动：%s", gotB.Body[:min(80, len(gotB.Body))])
	}
}

// ⑥ 端到端（声明袋 → 裁决 → 回报）：ApplyAdapterOverrides 必须把拒发整枚**回传**给调用方。
// 这正是「机制在、接线不在」那一格的判据：只改 ApplyTokenBudget 而不把这个返回值交出去 ⇒ 本用例红
// （旧签名 `[]byte` 拿不到它 ⇒ 网关只能照发，就是本修要治的病）。
func TestApplyAdapterOverrides_RejectPropagatesToCaller(t *testing.T) {
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir()) // 无档案 ⇒ ctx 与上限都走声明袋
	ada := &bareAdapter{}
	g := &Gateway{
		adapterRegistry: map[string]plugin.Plugin{"deepseek-v4-flash": ada},
		config:          fleetWithDecl(t, "deepseek-v4-flash", 8192, 32768),
	}
	body := outbudgetBody(t, 26000, 22438)
	out, bud := g.ApplyAdapterOverrides("deepseek-v4-flash", body, ada)
	if !bud.Rejected {
		t.Fatalf("声明袋里的 ctx=8192 + prompt≈6500 ⇒ 必须拒发并回传，实得 %+v", bud)
	}
	if bud.RejectCode == "" || bud.RejectReason == "" {
		t.Fatalf("拒发必须带回错误码与可读原因（✗ 只写日志）：%+v", bud)
	}
	// 拒发**不进任何输出额度**：调用方原值照旧、也不补 max_output_tokens 那一格。
	// （声明袋里那几处与额度无关的覆盖 —— 如温度 —— 本来就已经落进 body，不在此列。）
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("拒发时交回的请求体仍是合法 JSON：%v", err)
	}
	if _, has := obj["max_output_tokens"]; has {
		t.Fatalf("拒发不许补任何输出额度字段：%s", out[:min(80, len(out))])
	}
	if v, _ := obj["max_tokens"].(float64); int(v) != 22438 {
		t.Fatalf("拒发不许改写调用方原值（应仍是 22438），实得 %v", obj["max_tokens"])
	}
}
