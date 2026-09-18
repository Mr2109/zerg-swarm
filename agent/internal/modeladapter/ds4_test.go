package modeladapter

import (
	"reflect"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// 2026-09-14 新增：ds4 适配器的 --ctx / --vision 透传
// 设计依据：docs/01-设计/设计-ds4适配器接视觉.md

func mkDS4(file string, custom map[string]interface{}) *registry.ModelEntry {
	return &registry.ModelEntry{File: file, Custom: custom}
}

// 无 vision / 无 ctx ⇒ 参数与改动前逐元素相同（防回归）
func TestDS4NoVisionUnchanged(t *testing.T) {
	e := mkDS4("/data/models/v4.gguf", map[string]interface{}{})
	got := (&DS4{}).BuildArgs(e, 9400)
	want := []string{"--model", "/data/models/v4.gguf", "--port", "9400", "--host", "127.0.0.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("无 vision/ctx 时参数应与基线一致\n got=%v\nwant=%v", got, want)
	}
}

// ssd 两个键的老行为不变
func TestDS4SSDArgsUnchanged(t *testing.T) {
	e := mkDS4("/m.gguf", map[string]interface{}{
		"ssd":                         true,
		"ssd_streaming_cache_experts": "4GB",
	})
	got := (&DS4{}).BuildArgs(e, 9400)
	want := []string{"--model", "/m.gguf", "--port", "9400", "--host", "127.0.0.1",
		"--ssd-streaming", "--ssd-streaming-cache-experts", "4GB"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ssd 参数应与基线一致\n got=%v\nwant=%v", got, want)
	}
}

// vision 有值 ⇒ 含 --vision 且值紧随其后
func TestDS4VisionArgAppended(t *testing.T) {
	e := mkDS4("/m.gguf", map[string]interface{}{"vision": "/enc.gguf"})
	got := (&DS4{}).BuildArgs(e, 9400)
	if !hasPair(got, "--vision", "/enc.gguf") {
		t.Fatalf("应含 --vision /enc.gguf，实得 %v", got)
	}
	for i, a := range got {
		if a == "--vision" && (i+1 >= len(got) || got[i+1] != "/enc.gguf") {
			t.Fatalf("--vision 与其值必须相邻，实得 %v", got)
		}
	}
}

// vision 为空串 ⇒ 不加参数
func TestDS4VisionEmptyIgnored(t *testing.T) {
	e := mkDS4("/m.gguf", map[string]interface{}{"vision": ""})
	got := (&DS4{}).BuildArgs(e, 9400)
	for _, a := range got {
		if a == "--vision" {
			t.Fatalf("空 vision 不应产生 --vision，实得 %v", got)
		}
	}
}

// ctx 各类型都认（YAML 可能给 int / int64 / float64 / string）
func TestDS4CtxArgAllTypes(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
	}{
		{"int", 1048576},
		{"int64", int64(1048576)},
		{"float64", float64(1048576)},
		{"string", "1048576"},
	}
	for _, c := range cases {
		e := mkDS4("/m.gguf", map[string]interface{}{"ctx": c.value})
		got := (&DS4{}).BuildArgs(e, 9400)
		if !hasPair(got, "--ctx", "1048576") {
			t.Fatalf("[%s] 应含 --ctx 1048576，实得 %v", c.name, got)
		}
	}
}

// ctx 非法/缺失/零 ⇒ 不加参数（不能把 0 传给引擎）
func TestDS4CtxInvalidIgnored(t *testing.T) {
	for _, v := range []interface{}{0, -1, "abc", "", nil} {
		e := mkDS4("/m.gguf", map[string]interface{}{"ctx": v})
		got := (&DS4{}).BuildArgs(e, 9400)
		for _, a := range got {
			if a == "--ctx" {
				t.Fatalf("非法 ctx=%v 不应产生 --ctx，实得 %v", v, got)
			}
		}
	}
}

// 视觉模型名必须派发到 DS4 适配器（防派发回归）
func TestDispatchDeepSeekVisionExp(t *testing.T) {
	a := Dispatch("DeepSeek-V4-Flash-Vision-Exp")
	if _, ok := a.(*DS4); !ok {
		t.Fatalf("DeepSeek-V4-Flash-Vision-Exp 应命中 DS4，实得 %T (%s)", a, a.Name())
	}
	a2 := Dispatch("deepseek-v4-flash")
	if _, ok := a2.(*DS4); !ok {
		t.Fatalf("deepseek-v4-flash 应命中 DS4，实得 %T", a2)
	}
}

func hasPair(args []string, key, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == key && args[i+1] == val {
			return true
		}
	}
	return false
}
