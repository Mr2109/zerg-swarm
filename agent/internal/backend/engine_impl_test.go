// engine_impl_test.go —— 「卵要体现出用的是什么引擎」（Mr2109 2026-09-15 拍）。
//
// 背景：`EngineImplOf` 原先只取 cmd 首词的**基名** ⇒ X3 上的
// `/home/g01/llama.cpp-src/build-hip-flash/bin/llama-server` 与主线 llama-server 同名，
// 两枚**不同的卵**在清单里看起来一样，"同一模型、不同引擎版本"（§6.8）表达不出来。
// 现在：声明 `engine_impl` 优先；否则按「实际会执行的可执行文件」推**可辨识短名**
// （跳过 bin/sbin/usr/local/opt 这类通用目录，取最近的携带信息的那一层）。
package backend

import (
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

func TestShortEngineName_DistinguishesBuilds(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/home/g01/llama.cpp-src/build-hip-flash/bin/llama-server", "build-hip-flash/llama-server"},
		{"/home/g01/llama-k2/build-k2/bin/llama-server", "build-k2/llama-server"},
		{"/home/g01/agent/run-k2.sh", "agent/run-k2.sh"},
		{"/opt/homebrew/bin/llama-server", "llama-server"},
		{"/usr/local/bin/llama-server", "llama-server"},
		{"/usr/bin/llama-server", "llama-server"},
		{"llama-server", "llama-server"},
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		if got := shortEngineName(c.in); got != c.want {
			t.Errorf("shortEngineName(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
	// 关键断言：两个 fork 的短名**必须不同**（否则卵的引擎字段形同虚设）
	a := shortEngineName("/home/g01/llama.cpp-src/build-hip-flash/bin/llama-server")
	b := shortEngineName("/home/g01/llama-k2/build-k2/bin/llama-server")
	if a == b {
		t.Fatalf("hip-flash 与 k2 两个 fork 的短名相同（%q）—— 卵区分不出引擎实现", a)
	}
}

func TestEngineImplOf_DeclaredWins(t *testing.T) {
	e := &registry.ModelEntry{EngineImpl: "build-k2/llama-server", Backend: "llama-server"}
	if got := EngineImplOf(e); got != "build-k2/llama-server" {
		t.Fatalf("声明的 engine_impl 应原样采用，实得 %q", got)
	}
}

func TestEngineImplOf_FromCmdAndBackend(t *testing.T) {
	// cmd 首词 → 可辨识短名（不是裸基名）
	e := &registry.ModelEntry{Cmd: registry.CmdString("/home/g01/llama-k2/build-k2/bin/llama-server -m {file} --port {port}")}
	if got := EngineImplOf(e); got != "build-k2/llama-server" {
		t.Fatalf("cmd 首词应推成 build-k2/llama-server，实得 %q", got)
	}
	// 非 llama 家族的 backend 原样返回（那是引擎家族名）
	d := &registry.ModelEntry{Backend: "ds4-server"}
	if got := EngineImplOf(d); got != "ds4-server" {
		t.Fatalf("ds4-server 应原样返回，实得 %q", got)
	}
	// nil 与「什么都没声明」都不许 panic，且给非空真值
	if got := EngineImplOf(nil); got == "" {
		t.Error("nil 条目应给出非空的引擎实现名（探测不到时退回 llama-server）")
	}
	if got := EngineImplOf(&registry.ModelEntry{Backend: "llama-server"}); got == "" {
		t.Error("llama 家族条目应给出非空的引擎实现名")
	}
}

func TestCheckEngineImplConsistency(t *testing.T) {
	const engine = "/home/g01/llama.cpp-src/build-hip-flash/bin/llama-server"

	// 相符：可辨识短名 / 纯基名 / 未声明（不校验）⇒ 放行
	for _, ok := range []string{"build-hip-flash/llama-server", "llama-server", ""} {
		if _, err := checkEngineImplConsistency(engine, ok); err != nil {
			t.Errorf("声明 %q 应放行，实得错误：%v", ok, err)
		}
	}
	// 不相符：声明了**另一个 build** ⇒ 必须拒孵（不许静默按其一跑）
	_, err := checkEngineImplConsistency(engine, "build-k2/llama-server")
	if err == nil {
		t.Fatal("声明 build-k2 而实际执行 hip-flash —— 必须拒孵")
	}
	if !strings.Contains(err.Error(), "不一致") {
		t.Errorf("拒孵理由应说明不一致，实得：%v", err)
	}
}
