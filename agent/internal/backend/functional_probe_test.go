// functional_probe_test.go —— M3 功能预检的回归。
//
// 四类形态都取自真机经验，不是假想：
//
//	① 正常出字（200 + 有 choice）        ⇒ 必须判通过
//	② 端口活着但出不了字（200 + choices 为空）⇒ **必须判失败**（这正是"装好了却不能用"的形态）
//	③ 引擎自报 5xx（500 + rocm prefill failed）⇒ 必须判失败（2026-09-14 事故原文）
//	④ 响应不是 JSON                       ⇒ 必须判失败（不许把解析失败当成功）
package backend

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func portOfURL(t *testing.T, u string) int {
	t.Helper()
	i := strings.LastIndex(u, ":")
	if i < 0 {
		t.Fatalf("无法从 %q 解析端口", u)
	}
	p, err := strconv.Atoi(u[i+1:])
	if err != nil {
		t.Fatalf("无法从 %q 解析端口: %v", u, err)
	}
	return p
}

func TestProbeInference(t *testing.T) {
	cases := []struct {
		name    string
		code    int
		body    string
		wantErr bool
	}{
		{"正常出字", http.StatusOK, `{"choices":[{"message":{"role":"assistant","content":"p"}}]}`, false},
		{"端口活着但无 choice", http.StatusOK, `{"choices":[]}`, true},
		{"引擎自报 500（2026-09-14 事故原文）", http.StatusInternalServerError,
			`{"error":{"message":"rocm prefill failed"}}`, true},
		{"响不是 JSON", http.StatusOK, `<html>oops</html>`, true},
		{"错误对象但 200（引擎把错包在 200 里）", http.StatusOK, `{"error":{"message":"prefill failed"}}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(c.code)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()

			err := probeInference(portOfURL(t, srv.URL), "test-model", 5*time.Second)
			if c.wantErr && err == nil {
				t.Fatalf("期望预检失败，却判通过了（这正是「加载成功≠可用」漏网的形态）")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("期望预检通过，却报错：%v", err)
			}
		})
	}
}

func TestProbeInference_ConnectionRefused(t *testing.T) {
	// 没有任何东西监听 ⇒ 必须报错（fail-closed，绝不把"连不上"当就绪）
	if err := probeInference(59997, "test-model", 2*time.Second); err == nil {
		t.Fatal("连不上端口时应报错，却判通过了")
	}
}

func TestDowngradeArgs(t *testing.T) {
	cases := []struct {
		name        string
		in          []string
		wantCtx     string
		wantCache   string
		wantChanged bool
	}{
		{
			name:        "1M→512k，16GB→8GB，512→256（设计举的例子）",
			in:          []string{"--ctx", "1048576", "--ssd-streaming-cache-experts", "16GB", "--ssd-streaming-preload-experts", "512"},
			wantCtx:     "524288",
			wantCache:   "8GB",
			wantChanged: true,
		},
		{
			name:        "已经很小 ⇒ 不动（不许把 ctx 降到 4096 以下）",
			in:          []string{"--ctx", "4096"},
			wantCtx:     "4096",
			wantChanged: false,
		},
		{
			name:        "认不出的形状一律不动",
			in:          []string{"--model", "/data/x.gguf", "--port", "9401"},
			wantChanged: false,
		},
		{
			name:        "缓存单位也认（4GB→2GB）",
			in:          []string{"--ssd-streaming-cache-experts", "4GB"},
			wantCache:   "2GB",
			wantChanged: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, changed := downgradeArgs(c.in)
			if changed != c.wantChanged {
				t.Fatalf("changed=%v，期望 %v", changed, c.wantChanged)
			}
			if c.wantCtx != "" {
				if v := argValue(got, "--ctx"); v != c.wantCtx {
					t.Fatalf("--ctx=%q，期望 %q", v, c.wantCtx)
				}
			}
			if c.wantCache != "" {
				if v := argValue(got, "--ssd-streaming-cache-experts"); v != c.wantCache {
					t.Fatalf("--ssd-streaming-cache-experts=%q，期望 %q", v, c.wantCache)
				}
			}
			// 原切片不得被就地改写（调用方可能还要用原参数）
			if c.wantChanged && &got[0] == &c.in[0] {
				t.Fatal("降档应返回新切片，不得就地改写原参数")
			}
		})
	}
}

func argValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
