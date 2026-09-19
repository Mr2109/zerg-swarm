// reload_profile_refresh_test.go —— 2026-09-19 ①：`/infer/reload` 只重载注册表、**不重载卵档案**。
//
// 真机现象：盘上档案已更新为 `peak_gtt_gb: 74.567`，子端仍按旧值（14.5/15.2）放行与展示。
// 本文件钉住：reload 之后的返回值与**缓存内容**都必须是盘上的新事实（档案目录走
// ZERG_EGG_PROFILE_DIR = t.TempDir()，**绝不碰真实 ~/.zerg**）。
package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
)

// postReloadRaw 直接打 handleReload（与既有 postLoadRaw 同风格：不启监听端口）。
func postReloadRaw(t *testing.T, s *Server, token string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest("POST", "/infer/reload", nil)
	if token != "" {
		req.Header.Set("X-Auth-Token", token)
	}
	rec := httptest.NewRecorder()
	s.handleReload(rec, req)
	var out map[string]interface{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("响应不是 JSON: %q（%v）", rec.Body.String(), err)
		}
	}
	return rec.Code, out
}

// writeProfileForServer 写一份能过校验的 v1 实测档案（字段口径与 monitor.EggProfile 一致）。
func writeProfileForServer(t *testing.T, dir, eggID string, peakGtt, peakMem float64) {
	t.Helper()
	body := "weight_size_gb: 90\n" +
		"peak_gtt_gb: " + strconv.FormatFloat(peakGtt, 'f', -1, 64) + "\n" +
		"peak_mem_gb: " + strconv.FormatFloat(peakMem, 'f', -1, 64) + "\n" +
		"load_seconds: 120\nthroughput_tok_s: 4.8\nsuggested_idle_unload_s: 600\n" +
		"schema_version: 1\nmeasured_at: 2026-09-15T10:00:00+08:00\nmachine: x3\ncalib_runs: 3\n"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, eggID+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ① reload 必须把磁盘上的新档案吃进缓存，并在响应里如实回报刷新了几枚。
func TestInferReload_RefreshesEggProfileCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir)
	if got := monitor.EggProfileDir(); got != dir {
		t.Fatalf("档案目录没被环境变量接管（用例会写到真实 home）：%q", got)
	}
	reg := registryFromYAML(t, "egg-a:\n"+
		"  backend: llama-server\n"+
		"  file: /nonexistent/egg-a.gguf\n"+
		"  schema_version: 1\n"+
		"  idle_unload_s: 600\n"+
		"  cmd: /nonexistent/zerg-noop -m {file} --port {port}\n")
	mgr := backend.NewManager(reg, "x3")
	s := NewServer(NewAgent("x3", "tok", reg, mgr, ""))

	// 盘上先放旧档案（真机里的 14.5/15.2）并让它进缓存
	writeProfileForServer(t, dir, "egg-a", 14.5, 15.2)
	if !mgr.EggProfileAvailable("egg-a") {
		t.Fatal("档案已落盘 ⇒ has_profile 应为 true")
	}

	// 重标定：盘上改成 74.567 / 74.882
	writeProfileForServer(t, dir, "egg-a", 74.567, 74.882)

	code, out := postReloadRaw(t, s, "tok")
	if code != 200 {
		t.Fatalf("reload 应 200，实得 %d（%v）", code, out)
	}
	n, _ := out["egg_profiles"].(float64)
	if n < 1 {
		t.Fatalf("reload 响应必须回报刷新了几枚卵档案（egg_profiles>=1），实得 %v（%+v）", out["egg_profiles"], out)
	}
	// 缓存里必须是**新值**（这就是「闸门/观测面按新事实走」的凭据）
	p, ok := mgr.CachedEggProfile("egg-a")
	if !ok {
		t.Fatal("刷新后 egg-a 应在缓存里")
	}
	if p.PeakGttGb != 74.567 || p.PeakMemGb != 74.882 {
		t.Fatalf("reload 后缓存必须是盘上新值 74.567/74.882，实得 %v/%v（真机缺陷就是这里留着旧值）",
			p.PeakGttGb, p.PeakMemGb)
	}
}

// ② 反例：档案读不出来时 reload **不许**说刷新成功——要带问题清单（留痕，不静默）。
func TestInferReload_ReportsUnreadableProfiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_EGG_PROFILE_DIR", dir)
	reg := registryFromYAML(t, "egg-a:\n"+
		"  backend: llama-server\n"+
		"  file: /nonexistent/egg-a.gguf\n"+
		"  schema_version: 1\n"+
		"  idle_unload_s: 600\n"+
		"  cmd: /nonexistent/zerg-noop -m {file} --port {port}\n")
	s := NewServer(NewAgent("x3", "tok", reg, backend.NewManager(reg, "x3"), ""))

	code, out := postReloadRaw(t, s, "tok")
	if code != 200 {
		t.Fatalf("reload 应 200（注册表重载成功），实得 %d（%v）", code, out)
	}
	if n, _ := out["egg_profiles"].(float64); n != 0 {
		t.Fatalf("没有档案例应回报 0 枚，实得 %v", out["egg_profiles"])
	}
	errs, _ := out["egg_profile_errors"].([]interface{})
	if len(errs) == 0 {
		t.Fatalf("档案读不出来必须如实上报问题清单（不许把「读不到」说成刷新完成）：%+v", out)
	}
	if !strings.Contains(errs[0].(string), "egg-a") {
		t.Fatalf("问题清单应点名是哪枚卵：%v", errs)
	}
}
