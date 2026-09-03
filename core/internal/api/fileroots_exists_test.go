package api

// fileroots_exists_test.go — 文件/目录浏览器 阶段 1 后端小尾巴（2026-09-13）：
//   ① FileRoot.Exists —— 根不存在也照常返回该项（exists=false），供 UI 显示「根不存在」空态；
//   ② /api/capabilities、/api/openapi.json、/api/help 三条可发现性端点补录
//      GET /api/fileroots、POST /api/fileroots/open、POST /api/fileroots/reveal。
//
// 纪律：
//   - 写盘一律 t.TempDir()（绝不碰 ~/.zerg、绝不碰仓库 docs；open/reveal 用桩，不真开窗口）；
//   - 断言钉**字面值**（不做「引用产品常量」式断言——那会改常量两边同变、恒绿）；
//   - 原始 JSON 层面也要断言一次：结构体字段缺键时会退化成零值，只看结构体会把「没写出去」
//     错判成「写出去且为 false」。
//
// 用例 → 需求映射：
//   TestFileRoots_ExistsTrueWhenAllPresent      → ① 五项都在 ⇒ exists=true（且字段真在响应里）
//   TestFileRoots_ExistsFalseForMissingRoot     → ① 指向不存在目录的根仍在列表里且 exists=false
//   TestDiscoverability_FileRootsEndpoints       → ② 三端点出现在 capabilities / openapi / help
//   TestDiscoverability_NoAuthBypass             → ② 三个可发现性端点仍走鉴权（不是旁路）

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// newPhase1Router 组一个与 main.go 注册方式一致的路由（fileroots 三端点 + 三个可发现性端点）。
func newPhase1Router(h *Handlers) *chi.Mux {
	r := chi.NewRouter()
	r.Use(AuthMiddleware(fbToken))
	r.Get("/api/fileroots", h.FileRootsHandler)
	r.Post("/api/fileroots/open", h.FileOpenHandler)
	r.Post("/api/fileroots/reveal", h.FileRevealHandler)
	r.Get("/api/docs", h.DocsHandler) // 供「根不存在时列目录仍 200 + 空列表」这条断言用
	r.Get("/api/capabilities", h.CapabilitiesHandler)
	r.Get("/api/openapi.json", h.OpenAPIHandler)
	r.Get("/api/help", h.HelpHandler)
	return r
}

// fetchRoots 取 /api/fileroots 的 roots 段 + 原始响应体。
func fetchRoots(t *testing.T, r *chi.Mux) ([]FileRoot, string) {
	t.Helper()
	w := doFB(t, r, http.MethodGet, "/api/fileroots", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200（body=%s）", w.Code, w.Body.String())
	}
	var body struct {
		Roots []FileRoot `json:"roots"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是 JSON: %v", err)
	}
	return body.Roots, w.Body.String()
}

// ── ① exists：五项都在 ──────────────────────────────────────────────────────

func TestFileRoots_ExistsTrueWhenAllPresent(t *testing.T) {
	setupFB(t) // 五个根目录都真实建出来
	h, _ := newFBHandlers(t)
	r := newPhase1Router(h)

	roots, raw := fetchRoots(t, r)
	if len(roots) != 5 {
		t.Fatalf("根数 = %d，期望 5: %+v", len(roots), roots)
	}
	for _, root := range roots {
		if !root.Exists {
			t.Errorf("根 %s 目录存在，exists 应为 true（path=%s）", root.ID, root.Path)
		}
	}
	// 原始 JSON 层面：字段必须真被写出去（结构体缺键会退化成 false，只看字段会假绿）
	if !strings.Contains(raw, `"exists":true`) {
		t.Errorf("响应里没有 \"exists\":true —— 字段没写出去？body=%s", raw)
	}
	if strings.Contains(raw, `"exists":false`) {
		t.Errorf("五项都在时不该出现 exists:false —— body=%s", raw)
	}
}

// ── ① exists：根不存在（用 t.Setenv 把两个根指到不存在的目录）─────────────

func TestFileRoots_ExistsFalseForMissingRoot(t *testing.T) {
	setupFB(t)
	gone := filepath.Join(t.TempDir(), "不存在的根") // 只取路径，**不创建**
	t.Setenv("ZERG_WEIGHTS_DIR", gone)
	t.Setenv("ZERG_MODELS_DIR", gone)

	// 前提校验：该目录确实不存在（否则本用例证明不了任何事）
	if _, err := os.Stat(gone); !os.IsNotExist(err) {
		t.Fatalf("前提不成立：%s 竟已存在（err=%v）", gone, err)
	}

	h, _ := newFBHandlers(t)
	r := newPhase1Router(h)
	roots, raw := fetchRoots(t, r)

	// 不跳过不存在的根：仍是五项、顺序不变
	wantIDs := []string{"docs", "repo", "models", "tasks", "weights"}
	if len(roots) != 5 {
		t.Fatalf("根数 = %d，期望仍是 5（不跳过不存在的根）: %+v", len(roots), roots)
	}
	for i, root := range roots {
		if root.ID != wantIDs[i] {
			t.Fatalf("第 %d 个根 id = %q，期望 %q", i, root.ID, wantIDs[i])
		}
		switch root.ID {
		case "weights", "models":
			if root.Path != gone {
				t.Errorf("根 %s 路径 = %q，期望 %q（尊重环境变量）", root.ID, root.Path, gone)
			}
			if root.Exists {
				t.Errorf("根 %s 指向不存在的目录，exists 应为 false（path=%s）", root.ID, root.Path)
			}
		default:
			// docs / repo / tasks 由 setupFB 真实建出 ⇒ 仍然存在
			if !root.Exists {
				t.Errorf("根 %s 目录存在，exists 应为 true（path=%s）", root.ID, root.Path)
			}
		}
	}
	if !strings.Contains(raw, `"exists":false`) {
		t.Errorf("响应里应有 \"exists\":false —— body=%s", raw)
	}

	// 「根不存在」不等于「接口出错」：列目录仍是 200 + 空列表（既有语义不变）
	w := doFB(t, r, http.MethodGet, docsQuery("weights", ""), nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"files":[]`) {
		t.Errorf("不存在的根列表应 200 + files:[]，实际 %d（body=%s）", w.Code, w.Body.String())
	}

	// rootExists 单元反例：空串一律 false（没有路径谈不上存在——不猜、不假装）
	if rootExists("") {
		t.Error("rootExists(\"\") 应为 false")
	}
}

// ── ② 可发现性三端点：capabilities / openapi.json / help ───────────────────

func TestDiscoverability_FileRootsEndpoints(t *testing.T) {
	h, _ := newFBHandlers(t)
	r := newPhase1Router(h)

	t.Run("capabilities 清单含三条", func(t *testing.T) {
		w := doFB(t, r, http.MethodGet, "/api/capabilities", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("状态码 = %d，期望 200（body=%s）", w.Code, w.Body.String())
		}
		body := w.Body.String()
		// 钉字面值：只认这三条 endpoint 字符串，不认「模糊包含 fileroots 就算」
		for _, want := range []string{
			`"endpoint":"GET /api/fileroots"`,
			`"endpoint":"POST /api/fileroots/open"`,
			`"endpoint":"POST /api/fileroots/reveal"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("capabilities 缺少 %s —— body=%s", want, body)
			}
		}
	})

	t.Run("openapi.json 含三条路径与方法", func(t *testing.T) {
		w := doFB(t, r, http.MethodGet, "/api/openapi.json", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("状态码 = %d，期望 200（body=%s）", w.Code, w.Body.String())
		}
		var spec struct {
			Paths map[string]map[string]json.RawMessage `json:"paths"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
			t.Fatalf("OpenAPI 不是 JSON: %v", err)
		}
		want := map[string]string{
			"/api/fileroots":        "get",
			"/api/fileroots/open":   "post",
			"/api/fileroots/reveal": "post",
		}
		for path, method := range want {
			ops, ok := spec.Paths[path]
			if !ok {
				t.Errorf("openapi paths 缺少 %s —— 现有键=%v", path, keysOf(spec.Paths))
				continue
			}
			if _, ok := ops[method]; !ok {
				t.Errorf("openapi %s 缺少 %s —— 实际方法=%v", path, method, keysOf(ops))
			}
		}
	})

	t.Run("help 文本含三条", func(t *testing.T) {
		w := doFB(t, r, http.MethodGet, "/api/help", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("状态码 = %d，期望 200", w.Code)
		}
		var body struct {
			Help string `json:"help"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("help 不是 JSON: %v", err)
		}
		for _, want := range []string{
			"/api/fileroots -H",
			"/api/fileroots/open",
			"/api/fileroots/reveal",
		} {
			if !strings.Contains(body.Help, want) {
				t.Errorf("help 缺少 %q —— help=%s", want, body.Help)
			}
		}
		// help 里给出的三条 curl 必须与实现的方法一致（GET 根清单、POST 两个动作）
		if !strings.Contains(body.Help, "curl http://127.0.0.1:8580/api/fileroots -H") {
			t.Errorf("help 的根清单示例应为 GET：%s", body.Help)
		}
		if !strings.Contains(body.Help, "curl -X POST http://127.0.0.1:8580/api/fileroots/open") ||
			!strings.Contains(body.Help, "curl -X POST http://127.0.0.1:8580/api/fileroots/reveal") {
			t.Errorf("help 的 open/reveal 示例应为 POST：%s", body.Help)
		}
	})
}

// TestDiscoverability_NoAuthBypass：三个可发现性端点仍走 AuthMiddleware（老规矩：文档端点不是旁路）。
func TestDiscoverability_NoAuthBypass(t *testing.T) {
	h, _ := newFBHandlers(t)
	r := newPhase1Router(h)
	for _, target := range []string{"/api/capabilities", "/api/openapi.json", "/api/help"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s 无令牌状态码 = %d，期望 401", target, w.Code)
		}
	}
}

// keysOf 打印 map 的键（只在断言失败信息里用，帮定位「到底有哪些路径」）。
func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
