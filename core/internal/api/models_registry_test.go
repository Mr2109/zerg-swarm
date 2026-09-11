package api

// models_registry_test.go — GET /api/models/registry 只读接口测试
// （httptest + 真实 chi 路由 + AuthMiddleware，写法与 gateway_breakers_test.go 一致）。
//
// 覆盖（反例优先）：
//   - 根目录不存在 → 200 + count=0，且**请求后目录仍不存在**（证明只读，没建目录）；
//   - 根目录存在但空 → 200 + count=0；
//   - 临时根里放合法记录（走 modelreg.Store.Put 真实落盘）→ count/字段正确，
//     且 commercial=unknown → default_eligible=false、commercial=yes → true；
//   - 坏记录（故意写坏 JSON）→ 整个请求不 500，如实计入该条 errors + bad_records；
//   - .trace.json 兄弟文件不被当成记录（Store.List 语义）；
//   - 无 token → 401、错 token → 403（新接口不是免鉴权旁路）；
//   - 每次请求前后临时根的文件清单不变（严格只读，不写任何文件）。
//
// 写盘一律 t.TempDir()——绝不碰 ~/.zerg。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
	"github.com/go-chi/chi/v5"
)

// registryResp 是接口响应的解析视图（用与 handler 相同的字段名）。
type registryResp struct {
	Root         string                `json:"root"`
	ManifestsDir string                `json:"manifests_dir"`
	Count        int                   `json:"count"`
	BadRecords   int                   `json:"bad_records"`
	Records      []ModelRegistryRecord `json:"records"`
}

// newRegistryTestRouter 建一个与 main.go 注册方式一致的测试路由（同一套 AuthMiddleware）。
func newRegistryTestRouter(h *Handlers) *chi.Mux {
	r := chi.NewRouter()
	r.Use(AuthMiddleware("test-token"))
	r.Get("/api/models/registry", h.ModelRegistryHandler)
	return r
}

// doRegistryReq 发一次 GET /api/models/registry（token 为空则不带头）。
func doRegistryReq(t *testing.T, r *chi.Mux, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/models/registry", nil)
	if token != "" {
		req.Header.Set("X-Auth-Token", token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// getRegistry 带令牌请求一次并解析响应（非 200 直接失败）。
func getRegistry(t *testing.T, h *Handlers) registryResp {
	t.Helper()
	w := doRegistryReq(t, newRegistryTestRouter(h), "test-token")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200——实际 %d body=%s", w.Code, w.Body.String())
	}
	var resp registryResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应 JSON 解析失败: %v body=%s", err, w.Body.String())
	}
	return resp
}

// validRegistryRecord 造一条能过 Verify（errors=0）的记录。
func validRegistryRecord(id, digest, commercial string, ctx int, caps ...string) *modelreg.Record {
	rec := &modelreg.Record{
		Schema:        modelreg.SchemaV1,
		ID:            id,
		Digest:        digest,
		Name:          "测试模型-" + id,
		ContextWindow: ctx,
		Files: []modelreg.File{
			{Role: "weights", Name: id + ".gguf", SHA256: strings.Repeat("a", 64), Size: 4096},
		},
		EngineRecipes: map[string]modelreg.EngineRecipe{
			"llama-server": {Args: []string{"-ngl", "99"}},
		},
		License:   modelreg.License{SPDX: "apache-2.0", Commercial: commercial},
		SourceURL: "https://example.invalid/" + id,
	}
	for _, c := range caps {
		// source=declared 必须带来源锚，否则 Verify 会给 warn（这里保持 errors/warns 均为 0）
		rec.Capabilities = append(rec.Capabilities, modelreg.Capability{
			Name: c, Value: true, Source: "declared", Evidence: "model card / GGUF key",
		})
	}
	return rec
}

// putRecord 用真实 Store.Put 把记录落进临时根（strict=false，与 zerg-model 入目录同路径）。
func putRecord(t *testing.T, root string, rec *modelreg.Record) string {
	t.Helper()
	res, err := modelreg.NewStore(root).Put(rec, false)
	if err != nil {
		t.Fatalf("Store.Put 失败（造数据）: %v", err)
	}
	return res.Path
}

// snapshotTree 记录临时根下所有文件/目录的相对路径（证明"请求不写盘"用）。
func snapshotTree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, rel)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("遍历临时根失败: %v", err)
	}
	sort.Strings(out)
	return out
}

// assertSameTree 请求前后文件清单必须一致（严格只读）。
func assertSameTree(t *testing.T, root string, before []string) {
	t.Helper()
	after := snapshotTree(t, root)
	if strings.Join(before, "|") != strings.Join(after, "|") {
		t.Errorf("请求前后临时根内容变化（只读被破坏）:\n before=%v\n after =%v", before, after)
	}
}

// TestModelRegistryAPI_Cases 表驱动覆盖：缺失根 / 空根 / 有记录 / 混坏记录。
func TestModelRegistryAPI_Cases(t *testing.T) {
	cases := []struct {
		name string
		// setup 返回模型目录根（一律 t.TempDir() 下）
		setup func(t *testing.T) string
		// check 收到响应后做断言（root 是 setup 给的那个根）
		check func(t *testing.T, root string, resp registryResp)
	}{
		{
			name: "反例-根目录不存在：200 且 count=0",
			setup: func(t *testing.T) string {
				// 故意不创建：root 是 t.TempDir() 下一个不存在的子目录
				return filepath.Join(t.TempDir(), "absent-models")
			},
			check: func(t *testing.T, root string, resp registryResp) {
				if resp.Count != 0 {
					t.Errorf("count 应为 0——实际 %d records=%+v", resp.Count, resp.Records)
				}
				if resp.BadRecords != 0 {
					t.Errorf("bad_records 应为 0——实际 %d", resp.BadRecords)
				}
				if len(resp.Records) != 0 {
					t.Errorf("records 应为空——实际 %+v", resp.Records)
				}
				// 只读硬证据：请求后根目录仍不存在（接口没建目录）
				if _, err := os.Stat(root); !os.IsNotExist(err) {
					t.Fatalf("只读被破坏：请求后根目录应仍不存在，实际 err=%v", err)
				}
				if resp.Root != root {
					t.Errorf("root 回显应为 %s——实际 %s", root, resp.Root)
				}
			},
		},
		{
			name: "反例-根目录存在但空：200 且 count=0",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			check: func(t *testing.T, root string, resp registryResp) {
				if resp.Count != 0 {
					t.Errorf("count 应为 0——实际 %d", resp.Count)
				}
				if resp.ManifestsDir != filepath.Join(root, "manifests") {
					t.Errorf("manifests_dir 应为 %s——实际 %s", filepath.Join(root, "manifests"), resp.ManifestsDir)
				}
				// 空根请求也不得凭空造出 manifests/ 目录
				if _, err := os.Stat(filepath.Join(root, "manifests")); !os.IsNotExist(err) {
					t.Fatalf("只读被破坏：空根请求后不应出现 manifests/ 目录，实际 err=%v", err)
				}
			},
		},
		{
			name: "正例-两条合法记录：count/字段/default_eligible 正确",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				putRecord(t, root, validRegistryRecord("alpha", "sha256:"+strings.Repeat("1", 64), "unknown", 8192, "text", "tools"))
				putRecord(t, root, validRegistryRecord("beta", "sha256:"+strings.Repeat("2", 64), "yes", 32768, "vision"))
				// 兄弟留痕文件：.trace.json 不是记录，不得计入
				manifests := filepath.Join(root, "manifests", "alpha")
				if err := os.WriteFile(filepath.Join(manifests, "sha256-notarecord.trace.json"), []byte("{}"), 0o644); err != nil {
					t.Fatalf("造 .trace.json 失败: %v", err)
				}
				return root
			},
			check: func(t *testing.T, root string, resp registryResp) {
				if resp.Count != 2 {
					t.Fatalf("count 应为 2（.trace.json 不计）——实际 %d records=%+v", resp.Count, resp.Records)
				}
				if resp.BadRecords != 0 {
					t.Errorf("bad_records 应为 0——实际 %d", resp.BadRecords)
				}
				// 排序：(id, version) → alpha 在前
				if resp.Records[0].ID != "alpha" || resp.Records[1].ID != "beta" {
					t.Fatalf("应按 id 排序 [alpha beta]——实际 [%s %s]", resp.Records[0].ID, resp.Records[1].ID)
				}
				alpha := resp.Records[0]
				if alpha.Digest != "sha256:"+strings.Repeat("1", 64) {
					t.Errorf("digest 不符——实际 %s", alpha.Digest)
				}
				if alpha.Version != "sha256-"+strings.Repeat("1", 12) {
					t.Errorf("version 应为 sha256-<digest前12>——实际 %s", alpha.Version)
				}
				if alpha.Name != "测试模型-alpha" {
					t.Errorf("name 不符——实际 %s", alpha.Name)
				}
				if alpha.Commercial != "unknown" {
					t.Errorf("commercial 应为 unknown——实际 %s", alpha.Commercial)
				}
				// 红线：commercial 非 yes 不得作默认项
				if alpha.DefaultEligible {
					t.Errorf("commercial=unknown 时 default_eligible 必须为 false")
				}
				if !resp.Records[1].DefaultEligible {
					t.Errorf("commercial=yes 时 default_eligible 必须为 true")
				}
				if alpha.Errors != 0 || alpha.Warns != 0 {
					t.Errorf("合法记录 errors/warns 应均为 0——实际 %d/%d", alpha.Errors, alpha.Warns)
				}
				if alpha.ContextWindow != 8192 {
					t.Errorf("context_window 应为 8192——实际 %d", alpha.ContextWindow)
				}
				if len(alpha.Files) != 1 || alpha.Files[0].Role != "weights" || alpha.Files[0].SHA256 != strings.Repeat("a", 64) || alpha.Files[0].Size != 4096 {
					t.Errorf("files 明细不符——实际 %+v", alpha.Files)
				}
				if len(alpha.Capabilities) != 2 || alpha.Capabilities[0].Name != "text" || !alpha.Capabilities[0].Value || alpha.Capabilities[0].Source != "declared" {
					t.Errorf("capabilities 明细不符——实际 %+v", alpha.Capabilities)
				}
				if len(alpha.EngineRecipes) != 1 || alpha.EngineRecipes[0] != "llama-server" {
					t.Errorf("engine_recipes 不符——实际 %+v", alpha.EngineRecipes)
				}
				if alpha.Path == "" {
					t.Errorf("path 不应为空")
				}
			},
		},
		{
			name: "反例-坏记录：不 500、如实计入 errors",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				putRecord(t, root, validRegistryRecord("good", "sha256:"+strings.Repeat("3", 64), "unknown", 4096, "text"))
				badDir := filepath.Join(root, "manifests", "badmodel")
				if err := os.MkdirAll(badDir, 0o755); err != nil {
					t.Fatalf("造坏记录目录失败: %v", err)
				}
				if err := os.WriteFile(filepath.Join(badDir, "sha256-deadbeef0000.json"), []byte("{ this is not json"), 0o644); err != nil {
					t.Fatalf("造坏记录失败: %v", err)
				}
				return root
			},
			check: func(t *testing.T, root string, resp registryResp) {
				if resp.Count != 2 {
					t.Fatalf("count 应为 2（1 好 + 1 坏）——实际 %d records=%+v", resp.Count, resp.Records)
				}
				if resp.BadRecords != 1 {
					t.Errorf("bad_records 应为 1——实际 %d", resp.BadRecords)
				}
				var bad *ModelRegistryRecord
				for i := range resp.Records {
					if resp.Records[i].ID == "badmodel" {
						bad = &resp.Records[i]
					}
				}
				if bad == nil {
					t.Fatalf("坏记录应出现在 records 里（不得被静默丢掉）——实际 %+v", resp.Records)
				}
				if bad.Errors < 1 {
					t.Errorf("坏记录 errors 应 >= 1——实际 %d", bad.Errors)
				}
				if strings.TrimSpace(bad.Error) == "" {
					t.Errorf("坏记录 error 说明不应为空")
				}
				// 好记录不受影响
				for _, rec := range resp.Records {
					if rec.ID == "good" && rec.Errors != 0 {
						t.Errorf("好记录 errors 应为 0——实际 %d", rec.Errors)
					}
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.setup(t)
			before := snapshotTree(t, root)
			h := newTestHandlers()
			h.ModelsDir = root
			resp := getRegistry(t, h)
			tc.check(t, root, resp)
			assertSameTree(t, root, before)
			t.Logf("%s → count=%d bad_records=%d records=%s", tc.name, resp.Count, resp.BadRecords, mustJSON(t, resp.Records))
		})
	}
}

// TestModelRegistryAPI_RequiresAuth 无 token → 401；错 token → 403（鉴权中间件确实生效）。
func TestModelRegistryAPI_RequiresAuth(t *testing.T) {
	h := newTestHandlers()
	h.ModelsDir = t.TempDir()
	r := newRegistryTestRouter(h)

	if w := doRegistryReq(t, r, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("无令牌应 401——实际 %d body=%s", w.Code, w.Body.String())
	}
	if w := doRegistryReq(t, r, "wrong-token"); w.Code != http.StatusForbidden {
		t.Fatalf("错误令牌应 403——实际 %d body=%s", w.Code, w.Body.String())
	}
	if w := doRegistryReq(t, r, "test-token"); w.Code != http.StatusOK {
		t.Fatalf("正确令牌应 200——实际 %d body=%s", w.Code, w.Body.String())
	}
	t.Logf("无 token→401, 错 token→403, 对 token→200")
}

// mustJSON 打印用（失败不影响断言）。
func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		return "<marshal-error>"
	}
	return string(b)
}
