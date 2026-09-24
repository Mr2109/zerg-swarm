package adapters

// adapter_start_wiring_test.go —— v2.5.12 装配链判据（2026-09-24）。
//
// 病象（逐字见 `Zerg-内部文档/项目文档/v2.5.12/排查-卵子代理回声与会话串话-v2.5.12-20260924.md` §1.5 与 §③ H4）：
// 真机**每一发** example-35b-v2 请求都打
//
//	`⚠️ adapter example-35b-v2 execution failed (using defaults): example-35b-v2: not started`
//
// —— 适配器**只被 Init、从来没被 Start**（`core/cmd/zerg-core/main.go` 的注册循环），
// 而 `ApplyAdapterOverrides` 在 `aerr != nil` 时**原样返回 forwardBody**
// ⇒ 温度 / max_tokens / thinking 这些**声明一个都不下发**（fleet.yaml 里「适配器 max_tokens/thinking 已控」是死声明）。
//
// 本文件钉两件事（两件都不是「改适配器」能收口的）：
//
//	① 【源码级】主控装配链必须 **Init 与 Start 同现**（删掉那一行 ⇒ 本判据红）；
//	② 【行为级】Init + Start 之后，`Execute` 必须回得出声明袋，且三个键的**类型**正是网关认的那三种
//	   （`ApplyAdapterOverrides` 只读 float64 temperature / int max_tokens，thinking 只做声明回显）。
//
// 为什么①非用源码级不可：行为级判据只能覆盖「本包里的适配器」，证不了「装配链真的把它们推到了 started」
// —— 本次的根因**不在**适配器里，在装配链上（只 Init 不 Start）。

import (
	"os"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// ① 源码级：`core/cmd/zerg-core/main.go` 的注册循环里 Init 与 Start 必须同现（先剥注释再扫）。
func TestAdapterAssembly_InitsAndStarts(t *testing.T) {
	src, err := os.ReadFile("../../../cmd/zerg-core/main.go")
	if err != nil {
		t.Fatalf("读不到 core/cmd/zerg-core/main.go（判据件必须能自读）：%v", err)
	}
	var code []string
	inBlock := false
	for _, ln := range strings.Split(string(src), "\n") {
		s := strings.TrimSpace(ln)
		if inBlock {
			if strings.Contains(s, "*/") {
				inBlock = false
			}
			continue
		}
		if strings.HasPrefix(s, "/*") {
			if !strings.Contains(s, "*/") {
				inBlock = true
			}
			continue
		}
		if strings.HasPrefix(s, "//") {
			continue // 整行注释：本判据的说明文字里逐字写着 adp.Start()，不剥就会自己骗自己
		}
		if i := strings.Index(s, "//"); i >= 0 {
			s = s[:i] // 行末注释同口径剥掉
		}
		code = append(code, s)
	}
	text := strings.Join(code, "\n")

	iInit := strings.Index(text, "adp.Init(")
	iStart := strings.Index(text, "adp.Start()")
	if iInit < 0 {
		t.Fatal("main.go 里找不到适配器 Init 调用 —— 装配链被改过，本判据要跟着更新")
	}
	if iStart < 0 {
		t.Fatal("装配链里没有 adp.Start() —— 适配器恒 not started，example-35b-v2 的声明一个都不下发（本判据红）")
	}
	if iStart < iInit {
		t.Fatal("adp.Start() 出现在 adp.Init 之前 —— 顺序反了（Start 会报 not initialized）")
	}
	// 必须落在**同一段注册循环**里（Init 之后、注册表那条日志之前）；别处调 Start 不算。
	tail := text[iInit:]
	if i := strings.Index(tail, "Model adapter registry"); i >= 0 && !strings.Contains(tail[:i], "adp.Start()") {
		t.Fatal("adp.Start() 不在同一段注册循环里（Init … 注册表日志之间）—— 换个地方调 Start 不算把装配链补齐")
	}
}

// ② 行为级：Init + Start 之后，声明袋必须出得来，且三个键的类型正是网关认的那三种。
func TestOrnithAdapter_InitThenStart_DeclarationBagDelivered(t *testing.T) {
	// main.go 注册表里挂的正是这两枚 example-35b-v2（example-35b / example-35b-v2，同一个构造器）。
	for _, model := range []string{"example-35b", "example-35b-v2"} {
		t.Run(model, func(t *testing.T) {
			a := NewOrnithAdapter()
			if err := a.Init(nil); err != nil { // main.go：只 Init 过它
				t.Fatalf("Init: %v", err)
			}
			if err := a.Start(); err != nil { // ← 本批补上的那一步
				t.Fatalf("Start: %v", err)
			}
			// 网关的调用形态（tokencap.go ApplyAdapterOverrides）：Data{model} + Context{prompt}。
			out, err := a.Execute(plugin.PluginInput{
				Data:    map[string]any{"model": model},
				Context: map[string]any{"prompt": "只回一行：甲-在线"},
			})
			if err != nil {
				t.Fatalf("Init+Start 后 Execute 必须成功 —— 报错就会走网关的 'using defaults' 分支、声明不下发：%v", err)
			}
			bag, ok := out.Result.(map[string]any)
			if !ok {
				t.Fatalf("声明袋类型错误：%T", out.Result)
			}
			if v, ok := bag["temperature"].(float64); !ok || v != 0.8 {
				t.Errorf("声明袋 temperature 必须是 float64 0.8（网关只认 float64）：实得 %#v", bag["temperature"])
			}
			if v, ok := bag["max_tokens"].(int); !ok || v <= 0 {
				t.Errorf("声明袋 max_tokens 必须是正 int（网关预算分支的入口）：实得 %#v", bag["max_tokens"])
			}
			if _, ok := bag["thinking"].(bool); !ok {
				t.Errorf("声明袋 thinking 必须是 bool：实得 %#v", bag["thinking"])
			}
		})
	}
}
