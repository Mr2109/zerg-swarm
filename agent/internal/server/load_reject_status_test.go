// load_reject_status_test.go —— 反例用例：拒孵的 HTTP 状态码必须与 body 自称的码一致
// （2026-09-15 第一枚卵真机实测缺陷 7）。
//
// 缺陷原文（报告 §12 缺陷 7）：`backend.errResponse` 用 **int** 装 status
// （`map[string]interface{}{"status": 507}`），而 server 侧原先只断言 `.(float64)`
// （那是 JSON **反序列化**后的形态，不是内进程 map 的形态）⇒ 断言恒失败 ⇒
// **HTTP 恒为 500**，body 里却写着 `"status":507` / `"status":502`
// （真机负例原文：`HTTP=500 {"code":"no measured profile",...,"status":507}`）。
//
// 本文件钉住：
//
//	① responseStatus 的映射（int / float64 / int64 / 字符串 / 取不到）；
//	② 端到端：孵化路径的两种拒孵（507 无实测档案 / 502 声明认不得）在 **HTTP 状态行**上
//	   就是 507 / 502，不再是 500。
package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// ① 映射表：四种形态都认，取不到才回退。
func TestResponseStatus_MapsIntAndFloat(t *testing.T) {
	cases := []struct {
		name     string
		body     map[string]interface{}
		fallback int
		want     int
	}{
		{"int（backend.errResponse 的真实形态）", map[string]interface{}{"status": 507}, 500, 507},
		{"float64（JSON 反序列化形态）", map[string]interface{}{"status": float64(502)}, 500, 502},
		{"int64", map[string]interface{}{"status": int64(507)}, 500, 507},
		{"数字字符串", map[string]interface{}{"status": "507"}, 500, 507},
		{"无 status 字段", map[string]interface{}{"ok": false}, 500, 500},
		{"status 不是数", map[string]interface{}{"status": "nope"}, 500, 500},
		{"nil 响应体", nil, 500, 500},
	}
	for _, c := range cases {
		if got := responseStatus(c.body, c.fallback); got != c.want {
			t.Errorf("%s：应为 %d，实得 %d", c.name, c.want, got)
		}
	}
}

// registryFromYAML 落一份真注册表（YAML 到底的链路，不手工拼 ModelEntry）。
func registryFromYAML(t *testing.T, body string) *registry.Registry {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "agent_models.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := registry.New(p)
	if err != nil {
		t.Fatalf("加载测试注册表失败: %v", err)
	}
	return reg
}

// postLoadRaw 直接打 handleLoad（与既有 /unload 用例同一手法：不启监听端口）。
func postLoadRaw(t *testing.T, s *Server, token, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest("POST", "/load", strings.NewReader(body))
	req.Header.Set("X-Auth-Token", token)
	rec := httptest.NewRecorder()
	s.handleLoad(rec, req)
	var out map[string]interface{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("响应不是 JSON: %q（%v）", rec.Body.String(), err)
		}
	}
	return rec.Code, out
}

// ②-a 无实测档案 ⇒ 拒孵：HTTP 状态行必须是 507（不是 500），且与 body 一致。
func TestLoadReject_HTTPStatusIs507(t *testing.T) {
	t.Setenv(backend.EnvHatch, "1")
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir()) // 空档案目录 ⇒ 无实测档案

	// 不声明 mem_gb：跳过内存预检，把这条用例钉在「孵化闸门」那一条 507 上
	// （内存账随机器浮动，断言跟它走就成了偶发红）。
	reg := registryFromYAML(t, "egg-a:\n"+
		"  backend: llama-server\n"+
		"  file: /nonexistent/egg-a.gguf\n"+
		"  schema_version: 1\n"+
		"  idle_unload_s: 600\n"+
		"  cmd: /nonexistent/zerg-noop -m {file} --port {port}\n")
	s := NewServer(NewAgent("x3", "tok", reg, backend.NewManager(reg, "x3"), ""))

	code, out := postLoadRaw(t, s, "tok", `{"model":"egg-a"}`)
	if code != 507 {
		t.Fatalf("拒孵的 HTTP 状态行必须是 507（真机缺陷：恒 500，body 却写 507），实得 %d（body=%+v）", code, out)
	}
	if got, _ := out["status"].(float64); int(got) != code {
		t.Fatalf("HTTP 状态行（%d）必须与 body 自称的 status（%v）一致", code, out["status"])
	}
	if c, _ := out["code"].(string); c != "no measured profile" {
		t.Fatalf("本用例打的是「无实测档案」那条 507，实得 code=%q body=%+v", c, out)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "实测档案") {
		t.Fatalf("理由要写清「无实测档案」，实得 %v", out["error"])
	}
}

// ②-b 声明认不得（schema_version=99）⇒ 拒孵：HTTP 状态行必须是 502（不是 500）。
func TestLoadReject_HTTPStatusIs502(t *testing.T) {
	t.Setenv(backend.EnvHatch, "1")
	t.Setenv("ZERG_EGG_PROFILE_DIR", t.TempDir())

	reg := registryFromYAML(t, "egg-v99:\n"+
		"  backend: llama-server\n"+
		"  file: /nonexistent/egg-v99.gguf\n"+
		"  mem_gb: 1\n"+
		"  schema_version: 99\n"+
		"  cmd: /nonexistent/zerg-noop -m {file} --port {port}\n")
	s := NewServer(NewAgent("x3", "tok", reg, backend.NewManager(reg, "x3"), ""))

	code, out := postLoadRaw(t, s, "tok", `{"model":"egg-v99"}`)
	if code != 502 {
		t.Fatalf("声明认不得的拒孵必须是 502（真机缺陷：恒 500，body 却写 502），实得 %d（body=%+v）", code, out)
	}
	if got, _ := out["status"].(float64); int(got) != code {
		t.Fatalf("HTTP 状态行（%d）必须与 body 自称的 status（%v）一致", code, out["status"])
	}
}
