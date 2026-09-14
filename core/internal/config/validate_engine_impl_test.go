package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ═══ P1：卵清单必须承载「引擎实现/变体」（设计-子端沙箱化-20260914 §1.2 / §4.7 / 附录 C·C1）═══
//
// 本文件钉住的就是**那一句核心价值**：卵清单里的「引擎实现/变体」字段缺失时，
// 必须**报错（Fatal）**，**不得静默退回主线引擎**。
// 失败形态（附录 C·C1 的实测描述）：第二台设备要孵 K2 时落到 detectLlamaServerPath() 的
// 主线 llama-server ⇒ 起不来（unknown model architecture: k2-horizon）或误链，**且不报错**。

// engineImplCandidate 造一枚「需要非主线引擎实现」的卵（host 用远端 ⇒ 跳过本地文件存在性检查，
// 让本用例的判据只落在 cmd: 上，不受本机文件系统影响）。
func engineImplCandidate() ModelCandidate {
	return ModelCandidate{
		Name:         "example-moe-36b",
		Family:       "llama",
		Host:         "x3",
		Backend:      "llama-server",
		File:         "/data/models/k2/k2horizon-q4_k_m.gguf",
		MemGb:        23,
		CtxWindow:    32768,
		Modality:     "text",
		ToolSupport:  boolPtr(false),
		Architecture: "k2-horizon",
	}
}

// TestEngineImpl_RealFleetK2CarriesImpl 真实 fleet.yaml：凡需要非主线实现的卵都必须有 cmd:。
// 这是**回归测试**——K2 那条一旦被改回「只在 description 里写 run-k2.sh」就会在这里红。
func TestEngineImpl_RealFleetK2CarriesImpl(t *testing.T) {
	p := fleetPath(t)
	if p == "" {
		t.Skip("未找到 fleet.yaml（公开快照形态）——跳过")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("读不到 %s: %v", p, err)
	}
	cfg, err := ParseFleetConfig(data)
	if err != nil {
		t.Fatalf("%s 解析失败: %v", filepath.Base(p), err)
	}
	if filepath.Base(p) != "fleet.yaml" {
		t.Skipf("公开快照形态：%s 是模板，跳过严格判据", filepath.Base(p))
	}

	// 判据要能失败，也要能命中：样本为空 ⇒ 本用例已失效，必须红。
	need := 0
	for name, cands := range cfg.Models {
		for i, c := range cands {
			if !NeedsEngineImpl(c) {
				continue
			}
			need++
			if len(c.Cmd) == 0 {
				t.Errorf("卵 %s 候选#%d（架构 %s）缺 cmd: —— 会静默退回主线 llama-server", name, i, eggArch(c))
			}
		}
	}
	if need == 0 {
		t.Fatalf("%s 里没有任何「需要非主线引擎实现」的卵——判据样本为空，"+
			"说明架构字段或 NeedsEngineImpl 的清单被改动了（本用例已失效）", filepath.Base(p))
	}

	// K2 这一条逐项核对（设计稿附录 A.4 的真机形态照录）。
	cands, ok := cfg.Models["example-moe-36b"]
	if !ok || len(cands) == 0 {
		t.Fatal("fleet.yaml 里没有 example-moe-36b 条目")
	}
	k2 := cands[0]
	if !NeedsEngineImpl(k2) {
		t.Fatalf("K2 条目（架构 %s / backend %s）未被判为「需要非主线引擎实现」", eggArch(k2), k2.Backend)
	}
	if len(k2.Cmd) == 0 {
		t.Fatal("K2 条目缺 cmd: —— 引擎实现/变体没有任何字段承载")
	}
	if base := filepath.Base(k2.Cmd[0]); base != "run-k2.sh" {
		t.Fatalf("K2 的引擎实现应是包装脚本 run-k2.sh（清 LD_LIBRARY_PATH），实际 argv0=%s", k2.Cmd[0])
	}
	argv := strings.Join(k2.Cmd, " ")
	for _, want := range []string{"{file}", "{port}"} {
		if !strings.Contains(argv, want) {
			t.Fatalf("K2 的 cmd: 缺占位符 %s（manager 侧替换后才是真命令）: %s", want, argv)
		}
	}
	if !strings.Contains(argv, "-ngl") {
		t.Fatalf("K2 的 cmd: 缺 GPU 层数参数 -ngl（真机形态照录，缺了会退回 CPU）: %s", argv)
	}
	// 有 cmd: 之后 V016 不该再拦它（卵清单整体 Fatal 为 0 由 TestValidateRealFleet 把守）。
	if res := Validate("example-moe-36b", k2); res.HasFatal() {
		t.Fatalf("K2 已补 cmd: 却仍有 Fatal: %v", collectFields(res.Errors))
	}
}

// TestEngineImpl_MissingCmdIsFatal 缺 cmd: ⇒ Fatal（这是本项的核心判据，不许静默）。
func TestEngineImpl_MissingCmdIsFatal(t *testing.T) {
	res := Validate("example-moe-36b", engineImplCandidate())
	if !res.HasFatal() {
		t.Fatal("需要非主线引擎实现的卵缺 cmd: 必须 Fatal（拒孵），不许静默退回主线引擎")
	}
	fields := collectFields(res.Errors)
	if len(fields) != 1 || fields[0] != "cmd" {
		t.Fatalf("期望唯一一条 Fatal 落在 cmd 字段，实际: %v", fields)
	}
	msg := res.Errors[0].Message
	for _, want := range []string{"静默", "主线", "k2-horizon", "cmd"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("Fatal 文案必须点明 %q（否则读日志的人不知道后果）: %s", want, msg)
		}
	}
	if res.Errors[0].Level != FatalLevel {
		t.Fatalf("该判据必须是 Fatal 级别（拒孵），实际 %v", res.Errors[0].Level)
	}
}

// TestEngineImpl_CmdPresentPasses 补上 cmd: ⇒ 放行（判据不能反过来误伤）。
func TestEngineImpl_CmdPresentPasses(t *testing.T) {
	c := engineImplCandidate()
	c.Cmd = []string{"/home/g01/agent/run-k2.sh", "-m", "{file}", "--port", "{port}"}
	if res := Validate("example-moe-36b", c); res.HasFatal() {
		t.Fatalf("补了 cmd: 仍报 Fatal: %v", collectFields(res.Errors))
	}
}

// TestEngineImpl_PlainArchNotRequired 走主线的普通架构不该被这条判据波及。
func TestEngineImpl_PlainArchNotRequired(t *testing.T) {
	c := engineImplCandidate()
	c.Architecture = "qwen4exp"
	if NeedsEngineImpl(c) {
		t.Fatal("普通架构（主线支持）不该被判为需要非主线引擎实现")
	}
	if res := Validate("Qwen3.8-Flash-Next", c); res.HasFatal() {
		t.Fatalf("普通架构缺 cmd: 不该 Fatal: %v", collectFields(res.Errors))
	}
}

// TestEngineImpl_NonLlamaBackendNotRequired backend 已是非 llama 家族时，
// 「引擎实现」由 backend 字段承载（manager 直接取 ds4 可执行文件），不重复要求 cmd:。
func TestEngineImpl_NonLlamaBackendNotRequired(t *testing.T) {
	c := engineImplCandidate()
	c.Backend = "ds4-server"
	c.Architecture = ""
	if NeedsEngineImpl(c) {
		t.Fatal("backend 已是 ds4-server 时不该再要求 cmd:")
	}
	if res := Validate("deepseek-v4-flash", c); res.HasFatal() {
		t.Fatalf("ds4 条目缺 cmd: 不该 Fatal: %v", collectFields(res.Errors))
	}
}

// TestEngineImpl_ExplicitMarkerTriggers 架构还没进清单时的通用开关：engine_impl_required。
func TestEngineImpl_ExplicitMarkerTriggers(t *testing.T) {
	c := engineImplCandidate()
	c.Architecture = "some-brand-new-arch"
	if NeedsEngineImpl(c) {
		t.Fatal("未打标的新架构不该自动要求 cmd:")
	}
	c.EngineImplRequired = true
	if !NeedsEngineImpl(c) {
		t.Fatal("engine_impl_required: true 必须要求 cmd:")
	}
	if res := Validate("新引擎的卵", c); !res.HasFatal() {
		t.Fatal("打了 engine_impl_required 却缺 cmd: 必须 Fatal")
	}
	// 补上 cmd: ⇒ 放行。
	c.Cmd = []string{"/opt/fork/bin/llama-server", "-m", "{file}", "--port", "{port}"}
	if res := Validate("新引擎的卵", c); res.HasFatal() {
		t.Fatalf("补了 cmd: 仍报 Fatal: %v", collectFields(res.Errors))
	}
}

// TestEngineImpl_YAMLFlowAndBlockForm 两种写法都要能承载 cmd:（清单里既有 flow 也有 block 条目）。
func TestEngineImpl_YAMLFlowAndBlockForm(t *testing.T) {
	const doc = `
models:
  example-moe-36b:
    - { host: x3, backend: llama-server, file: /data/models/k2/x.gguf, mem_gb: 23, architecture: k2-horizon, cmd: ["/home/g01/agent/run-k2.sh", "-m", "{file}", "--port", "{port}"] }
  Block-Form-Fork:
    - host: x3
      backend: llama-server
      file: /data/models/fork/y.gguf
      mem_gb: 20
      architecture: some-fork-only
      engine_impl_required: true
      cmd:
        - /opt/fork/bin/llama-server
        - -m
        - "{file}"
        - --port
        - "{port}"
`
	cfg, err := ParseFleetConfig([]byte(doc))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	flow := cfg.Models["example-moe-36b"][0]
	if len(flow.Cmd) != 5 || filepath.Base(flow.Cmd[0]) != "run-k2.sh" {
		t.Fatalf("flow 形式的 cmd 解析错: %v", flow.Cmd)
	}
	block := cfg.Models["Block-Form-Fork"][0]
	if len(block.Cmd) != 5 || block.Cmd[0] != "/opt/fork/bin/llama-server" {
		t.Fatalf("block 形式的 cmd 解析错: %v", block.Cmd)
	}
	if !block.EngineImplRequired {
		t.Fatal("engine_impl_required 未解析（布尔开关掉了会导致判据静默失效）")
	}
	for _, c := range []ModelCandidate{flow, block} {
		if res := Validate("卵", c); res.HasFatal() {
			t.Fatalf("两种写法都带了 cmd:，不该 Fatal: %v", collectFields(res.Errors))
		}
	}
}
