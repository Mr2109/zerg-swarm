package api

// fileroots_test.go — 文件/目录浏览器 阶段 1 后端测试（《设计-文件浏览器虫茧-20260913》§七 后端 1–9 条）
//
// 纪律：
//   - 写盘一律 t.TempDir()：绝不碰 ~/.zerg、绝不碰仓库 docs（审计落注入目录；open/reveal 用桩，
//     否则测试会真的弹出访达窗口）。
//   - 反例优先：穿越、符号链接逃逸、非白名单类型、不存在路径都当正例来测。
//   - 逐字节断言（不是"解析后相等"）：缺省 /api/docs 的老行为要求字节级不变。
//
// 用例 → 需求映射：
//   TestFileroots_DefaultDocsByteIdentical      → §七 1（缺省输出逐字节一致）
//   TestFileroots_RootIsolation                 → §七 2（各根只列自己 + 跨根读 400）
//   TestFileroots_TraversalAndSymlinkRejected   → §七 3（穿越/绝对路径/符号链接逃逸）
//   TestFileroots_WriteEndpointsDocsOnly        → §七 4（只读根写操作被拒）
//   TestFileroots_ActionsOpenReveal             → §七 5（目录/文本/非白名单/不存在）
//   TestFileroots_AuditLog                      → §七 6（一行一动作、失败留痕、不记内容、轮转、30 天）
//   TestFileroots_ReadNoSizeLimit               → §七 7（3 MiB 完整返回）
//   TestFileroots_Contract                      → §七 8/9（五项顺序 + 环境变量 + 类型放开可配置）

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/modelreg"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/go-chi/chi/v5"
)

const fbToken = "test-token"

// ── 测试脚手架 ──────────────────────────────────────────────────────────────

// fbFixtures 是四个可注入根（每根一个 t.TempDir() 子目录；docs 挂在 repo 下）。
type fbFixtures struct {
	repo    string
	models  string
	tasks   string
	weights string
}

// setupFB 造固定根集合，并用环境变量把各根指过去（与生产同一套解析器，不改代码只改配置）。
func setupFB(t *testing.T) fbFixtures {
	t.Helper()
	base := t.TempDir()
	f := fbFixtures{
		repo:    filepath.Join(base, "repo"),
		models:  filepath.Join(base, "models"),
		tasks:   filepath.Join(base, "tasks"),
		weights: filepath.Join(base, "weights"),
	}
	for _, d := range []string{filepath.Join(f.repo, "docs"), f.models, f.tasks, f.weights} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("建夹具目录失败 %s: %v", d, err)
		}
	}
	t.Setenv("ZERG_WORKSPACE", f.repo)
	t.Setenv("ZERG_MODELS_DIR", f.models)
	t.Setenv("ZERG_TASK_ROOT", f.tasks)
	t.Setenv("ZERG_WEIGHTS_DIR", f.weights)
	return f
}

// fbAuditStub 是 open/reveal 的执行桩：只记录调用，绝不真开窗口。
type fbAuditStub struct {
	mu    sync.Mutex
	calls [][2]string
}

func (s *fbAuditStub) run(action, abs string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, [2]string{action, abs})
	return nil
}

func (s *fbAuditStub) snapshot() [][2]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][2]string, len(s.calls))
	copy(out, s.calls)
	return out
}

// newFBHandlers 造 Handlers：审计落 t.TempDir()、open/reveal 走桩。
func newFBHandlers(t *testing.T) (*Handlers, *fbAuditStub) {
	t.Helper()
	h := newTestHandlers()
	stub := &fbAuditStub{}
	h.FileBrowserAudit = &fileBrowserAuditParams{Dir: t.TempDir()}
	h.FileBrowserRun = stub.run
	return h, stub
}

// newFBTestRouter 组一个与 main.go 注册方式一致的路由（同一套 AuthMiddleware）。
func newFBTestRouter(h *Handlers) *chi.Mux {
	r := chi.NewRouter()
	r.Use(AuthMiddleware(fbToken))
	r.Get("/api/fileroots", h.FileRootsHandler)
	r.Post("/api/fileroots/open", h.FileOpenHandler)
	r.Post("/api/fileroots/reveal", h.FileRevealHandler)
	r.Get("/api/docs", h.DocsHandler)
	r.Get("/api/docs/*", h.DocsHandler)
	r.Post("/api/docs/mkdir", h.DocMkdirHandler)
	r.Post("/api/docs/rename", h.DocRenameHandler)
	r.Post("/api/docs/delete", h.DocDeleteHandler)
	r.Post("/api/docs/copy", h.DocCopyHandler)
	r.Post("/api/docs/save", h.DocSaveHandler)
	return r
}

// doFB 发一次请求（带令牌）；body 为 nil 表示无体。
func doFB(t *testing.T, r *chi.Mux, method, target string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("序列化请求体失败: %v", err)
		}
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, rd)
	req.Header.Set("X-Auth-Token", fbToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// docsQuery 拼参数化读取的 URL（url.Values.Encode 会把 / 变成 %2F——正好覆盖 "..%2f" 这一形态）。
func docsQuery(rootID, rel string) string {
	q := url.Values{}
	if rootID != "" {
		q.Set("root", rootID)
	}
	if rel != "" {
		q.Set("path", rel)
	}
	return "/api/docs?" + q.Encode()
}

// respErrorCode 取错误响应的 {"error":{"type":...}}。
func respErrorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("错误响应不是合法 JSON: %v (body=%s)", err, w.Body.String())
	}
	return body.Error.Type
}

// readUIActionLog 读注入目录里的审计行（只读主文件）。
func readUIActionLog(t *testing.T, dir string) []uiActionEntry {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, uiActionsLogName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("读审计日志失败: %v", err)
	}
	var out []uiActionEntry
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if line == "" {
			continue
		}
		var e uiActionEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("审计行不是合法 JSON（说明被写坏/截断）: %q: %v", line, err)
		}
		out = append(out, e)
	}
	return out
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写文件失败 %s: %v", path, err)
	}
}

// ── §七 1：缺省 GET /api/docs 输出与改造前逐字节一致 ────────────────────────

func TestFileroots_DefaultDocsByteIdentical(t *testing.T) {
	f := setupFB(t)
	docs := filepath.Join(f.repo, "docs")
	mustWriteFile(t, filepath.Join(docs, "01-设计", "a.md"), "# a\n")
	mustWriteFile(t, filepath.Join(docs, "INDEX.md"), "# 索引\n")
	mustWriteFile(t, filepath.Join(docs, "notes.txt"), "忽略我\n")          // 非 md：老行为不收
	mustWriteFile(t, filepath.Join(docs, "issues", "old.md"), "噪音\n")    // issues：老行为排除
	mustWriteFile(t, filepath.Join(docs, "thunderbolt", "x.md"), "噪音\n") // thunderbolt：排除
	mustWriteFile(t, filepath.Join(docs, "子目录", "notes.txt"), "忽略我\n")   // 目录收、文件不收

	// 老行为的目录列表（walk 字典序、只收 .md、排除 issues/thunderbolt）——手写的黄金字节，
	// 不是"解析后相等"：改过滤规则/改字段名/改排序都会立刻失败。
	const golden = "{\"dirs\":[\"01-设计\",\"子目录\"],\"files\":[\"01-设计/a.md\",\"INDEX.md\"]}\n"

	h, _ := newFBHandlers(t)
	r := newFBTestRouter(h)

	t.Run("无查询参数=老行为", func(t *testing.T) {
		w := doFB(t, r, http.MethodGet, "/api/docs", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("状态码 = %d，期望 200（body=%s）", w.Code, w.Body.String())
		}
		if got := w.Body.String(); got != golden {
			t.Fatalf("缺省 /api/docs 输出与改造前不一致：\n got=%q\nwant=%q", got, golden)
		}
	})

	t.Run("root=docs 与老行为逐字节相同", func(t *testing.T) {
		w := doFB(t, r, http.MethodGet, docsQuery("docs", ""), nil)
		if got := w.Body.String(); got != golden {
			t.Fatalf("root=docs 输出与老行为不一致：\n got=%q\nwant=%q", got, golden)
		}
	})

	t.Run("老的按路径读仍是{path,content}两字段", func(t *testing.T) {
		w := doFB(t, r, http.MethodGet, "/api/docs/INDEX.md", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("状态码 = %d，期望 200", w.Code)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatalf("响应不是 JSON: %v", err)
		}
		if len(m) != 2 || m["path"] != "INDEX.md" || m["content"] != "# 索引\n" {
			t.Fatalf("老读接口字段变了: %v（必须仍是 path+content 两字段）", m)
		}
	})

	t.Run("docs 根与写操作使用的 docsRoot 同源", func(t *testing.T) {
		if got := fileRoots()[0].Path; got != filepath.Join(statepath.WorkspaceRoot(), "docs") {
			t.Fatalf("docs 根 = %q，期望 <仓库根>/docs", got)
		}
	})
}

// ── §七 2：各根只列自己 + 跨根读 400 ────────────────────────────────────────

func TestFileroots_RootIsolation(t *testing.T) {
	f := setupFB(t)
	mustWriteFile(t, filepath.Join(f.repo, "docs", "only-docs.md"), "d\n")
	mustWriteFile(t, filepath.Join(f.models, "manifests", "rec.json"), "{}\n")
	mustWriteFile(t, filepath.Join(f.models, "only-models.gguf"), "M\n")

	h, _ := newFBHandlers(t)
	r := newFBTestRouter(h)

	listOf := func(rootID string) (files, dirs []string) {
		t.Helper()
		w := doFB(t, r, http.MethodGet, docsQuery(rootID, ""), nil)
		if w.Code != http.StatusOK {
			t.Fatalf("列 %s 根状态码 = %d，期望 200（body=%s）", rootID, w.Code, w.Body.String())
		}
		var body struct {
			Files []string `json:"files"`
			Dirs  []string `json:"dirs"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("列 %s 根响应不是 JSON: %v", rootID, err)
		}
		return body.Files, body.Dirs
	}

	docsFiles, _ := listOf("docs")
	if strings.Join(docsFiles, ",") != "only-docs.md" {
		t.Fatalf("docs 根应只列自己：got=%v", docsFiles)
	}
	// 非 docs 根不做 .md 过滤（权重根的意义就是看见 .gguf/mmproj/LICENSE）
	modelFiles, modelDirs := listOf("models")
	if strings.Join(modelFiles, ",") != "manifests/rec.json,only-models.gguf" {
		t.Fatalf("models 根文件列表 = %v", modelFiles)
	}
	if strings.Join(modelDirs, ",") != "manifests" {
		t.Fatalf("models 根目录列表 = %v", modelDirs)
	}
	for _, x := range append(modelFiles, modelDirs...) {
		if strings.Contains(x, "only-docs") {
			t.Fatalf("models 根泄露了 docs 根条目: %s", x)
		}
	}

	// 跨根读（反例）：以 models 根为起点往上走
	for _, rel := range []string{"..", "../..", "../../docs/only-docs.md"} {
		w := doFB(t, r, http.MethodGet, docsQuery("models", rel), nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("跨根读 rel=%q 状态码 = %d，期望 400（body=%s）", rel, w.Code, w.Body.String())
		}
		if got := respErrorCode(t, w); got != "INVALID_PATH" {
			t.Fatalf("跨根读 rel=%q 错误码 = %q，期望 INVALID_PATH", rel, got)
		}
	}
}

// ── §七 3：穿越 / 绝对路径 / 符号链接逃逸 ──────────────────────────────────

func TestFileroots_TraversalAndSymlinkRejected(t *testing.T) {
	f := setupFB(t)
	mustWriteFile(t, filepath.Join(f.weights, "sub", "a.md"), "# a\n")
	mustWriteFile(t, filepath.Join(f.weights, "inside-target", "ok.md"), "# ok\n")
	if err := os.Symlink("/etc", filepath.Join(f.weights, "esc")); err != nil {
		t.Fatalf("造逃逸符号链接失败: %v", err)
	}
	// 根内链接必须**放行**（否则就是把"用符号链接"本身当罪）——它是逃逸检查的阳性对照
	if err := os.Symlink(filepath.Join(f.weights, "inside-target"), filepath.Join(f.weights, "inside")); err != nil {
		t.Fatalf("造根内符号链接失败: %v", err)
	}

	h, _ := newFBHandlers(t)
	r := newFBTestRouter(h)

	traversal := []struct {
		name string
		rel  string // 走 url.Values 编码（/ 会变成 %2F）；与 raw 二选一
		raw  string // 原样查询串（用来看"..%2f"这种手写形态解码后是什么）
	}{
		{name: "两点", rel: ".."},
		{name: "两点斜杠", rel: "../"},
		{name: "两级上溯", rel: "../.."},
		{name: "上溯到兄弟根", rel: "../models/x.json"},
		{name: "中段上溯", rel: "sub/../../esc"},
		{name: "手写的 ..%2f", raw: "/api/docs?root=weights&path=..%2f.."},
		{name: "手写的编码点加斜杠", raw: "/api/docs?root=weights&path=%2e%2e%2f%2e%2e"},
		{name: "绝对路径", rel: "/etc/passwd"},
		{name: "绝对根", rel: "/"},
	}
	for _, tc := range traversal {
		t.Run("穿越_"+tc.name, func(t *testing.T) {
			target := tc.raw
			if target == "" {
				target = docsQuery("weights", tc.rel)
			}
			w := doFB(t, r, http.MethodGet, target, nil)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("请求 %q 状态码 = %d，期望 400（body=%s）", target, w.Code, w.Body.String())
			}
			if got := respErrorCode(t, w); got != "INVALID_PATH" {
				t.Fatalf("请求 %q 错误码 = %q，期望 INVALID_PATH", target, got)
			}
		})
	}

	t.Run("符号链接逃逸被拒", func(t *testing.T) {
		for _, rel := range []string{"esc", "esc/passwd"} {
			w := doFB(t, r, http.MethodGet, docsQuery("weights", rel), nil)
			if w.Code != http.StatusBadRequest || respErrorCode(t, w) != "INVALID_PATH" {
				t.Fatalf("rel=%q 应 400/INVALID_PATH，实际 %d/%s", rel, w.Code, w.Body.String())
			}
		}
	})

	t.Run("根内符号链接放行", func(t *testing.T) {
		w := doFB(t, r, http.MethodGet, docsQuery("weights", "inside/ok.md"), nil)
		if w.Code != http.StatusOK {
			t.Fatalf("根内链接应放行，实际 %d（body=%s）", w.Code, w.Body.String())
		}
	})

	// 直接测解析器：空串 / NUL / 未知根（HTTP 层到不了的两条）
	t.Run("safePathIn 单元反例", func(t *testing.T) {
		cases := []struct {
			name, root, rel, wantCode string
		}{
			{"空串", "weights", "", "INVALID_PATH"},
			{"NUL", "weights", "a\x00b", "INVALID_PATH"},
			{"绝对路径", "weights", "/abs", "INVALID_PATH"},
			{"未知根", "nope", "a.md", "INVALID_ROOT"},
			{"未知根+穿越", "nope", "../..", "INVALID_ROOT"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if _, code := safePathIn(tc.root, tc.rel); code != tc.wantCode {
					t.Fatalf("safePathIn(%q,%q) = %q，期望 %q", tc.root, tc.rel, code, tc.wantCode)
				}
			})
		}
		if abs, code := safePathIn("weights", "sub/a.md"); code != "" || abs != filepath.Join(f.weights, "sub", "a.md") {
			t.Fatalf("合法路径被拒或解析错: abs=%q code=%q", abs, code)
		}
	})
}

// ── §七 4：只读根上写操作被拒 ──────────────────────────────────────────────

func TestFileroots_WriteEndpointsDocsOnly(t *testing.T) {
	f := setupFB(t)
	mustWriteFile(t, filepath.Join(f.repo, "docs", "INDEX.md"), "# 索引\n")
	mustWriteFile(t, filepath.Join(f.weights, "w.md"), "w\n")
	mustWriteFile(t, filepath.Join(f.models, "m.json"), "{}\n")
	mustWriteFile(t, filepath.Join(f.tasks, "t.md"), "t\n")

	snapshot := func(dir string) string {
		t.Helper()
		var names []string
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(dir, p)
			names = append(names, rel)
			return nil
		})
		return strings.Join(names, ",")
	}
	before := map[string]string{
		f.weights: snapshot(f.weights),
		f.models:  snapshot(f.models),
		f.tasks:   snapshot(f.tasks),
	}

	h, _ := newFBHandlers(t)
	r := newFBTestRouter(h)

	// 既有五个写端点都不接受 root 参数、且只认 docs 根内的相对路径——
	// 所以"在只读根上写"的每条可达路径都必须是 400（且不落盘）。
	cases := []struct {
		name, endpoint string
		body           interface{}
	}{
		{"mkdir 穿到权重根", "/api/docs/mkdir", map[string]interface{}{"dir": "../weights/新建"}},
		{"mkdir 带 root 字段", "/api/docs/mkdir", map[string]interface{}{"root": "weights", "dir": "../weights/x"}},
		{"rename 目标穿出", "/api/docs/rename", map[string]interface{}{"old": "INDEX.md", "new": "../../weights/x.md"}},
		{"rename 带 root 字段", "/api/docs/rename", map[string]interface{}{"root": "weights", "old": "INDEX.md", "new": "../weights/x.md"}},
		{"delete 穿到权重根", "/api/docs/delete", map[string]interface{}{"path": "../weights/w.md"}},
		{"delete 带 root 字段", "/api/docs/delete", map[string]interface{}{"root": "weights", "path": "../weights/w.md"}},
		{"copy 目标穿出", "/api/docs/copy", map[string]interface{}{"from": "INDEX.md", "to": "../weights/x.md"}},
		{"copy 带 root 字段", "/api/docs/copy", map[string]interface{}{"root": "weights", "from": "INDEX.md", "to": "../weights/x.md"}},
		{"save 绝对路径", "/api/docs/save", map[string]interface{}{"path": "/tmp/不该写", "content": "x"}},
		{"save 穿到任务根", "/api/docs/save", map[string]interface{}{"path": "../tasks/x.md", "content": "x"}},
		{"save 空路径(带 root)", "/api/docs/save", map[string]interface{}{"root": "weights", "path": "", "content": "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doFB(t, r, http.MethodPost, tc.endpoint, tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("状态码 = %d，期望 400（body=%s）", w.Code, w.Body.String())
			}
			if got := respErrorCode(t, w); got != "INVALID_PATH" {
				t.Fatalf("错误码 = %q，期望 INVALID_PATH", got)
			}
		})
	}

	// 只读根的文件树必须一字未动
	for dir, want := range before {
		if got := snapshot(dir); got != want {
			t.Fatalf("只读根被写脏：%s\n got=%q\nwant=%q", dir, got, want)
		}
	}

	// 声明侧：只有 docs 可写
	wf := doFB(t, r, http.MethodGet, "/api/fileroots", nil)
	var roots struct {
		Roots []FileRoot `json:"roots"`
	}
	if err := json.Unmarshal(wf.Body.Bytes(), &roots); err != nil {
		t.Fatalf("fileroots 响应不是 JSON: %v", err)
	}
	for _, root := range roots.Roots {
		if want := root.ID == "docs"; root.Writable != want {
			t.Fatalf("根 %s 的 writable = %v，期望 %v（只有 docs 可写）", root.ID, root.Writable, want)
		}
	}
}

// ── §七 5：open / reveal 三条路径判定 ───────────────────────────────────────

func TestFileroots_ActionsOpenReveal(t *testing.T) {
	f := setupFB(t)
	mustWriteFile(t, filepath.Join(f.weights, "sub", "a.md"), "# a\n")
	mustWriteFile(t, filepath.Join(f.weights, "m.gguf"), "GGUF")
	mustWriteFile(t, filepath.Join(f.weights, "无扩展名"), "x")

	h, stub := newFBHandlers(t)
	r := newFBTestRouter(h)

	type actionCase struct {
		name             string
		endpoint, action string
		body             map[string]interface{}
		wantOK           bool
		wantCode         string // 非 200 时的期望错误码
		wantAbs          string
	}
	sub := filepath.Join(f.weights, "sub")
	cases := []actionCase{
		{"open 目录", "/api/fileroots/open", "open", map[string]interface{}{"root": "weights", "path": "sub", "mode": "dir"}, true, "", sub},
		{"open 文本", "/api/fileroots/open", "open", map[string]interface{}{"root": "weights", "path": "sub/a.md", "mode": "file"}, true, "", filepath.Join(sub, "a.md")},
		{"open 根本身", "/api/fileroots/open", "open", map[string]interface{}{"root": "weights", "path": ""}, true, "", f.weights},
		{"open 非白名单类型", "/api/fileroots/open", "open", map[string]interface{}{"root": "weights", "path": "m.gguf"}, false, "NOT_ALLOWED", ""},
		{"open 无扩展名", "/api/fileroots/open", "open", map[string]interface{}{"root": "weights", "path": "无扩展名"}, false, "NOT_ALLOWED", ""},
		{"open 不存在", "/api/fileroots/open", "open", map[string]interface{}{"root": "weights", "path": "nope.md"}, false, "NOT_FOUND", ""},
		{"open 未知根", "/api/fileroots/open", "open", map[string]interface{}{"root": "nope", "path": "a.md"}, false, "INVALID_ROOT", ""},
		{"open 穿越", "/api/fileroots/open", "open", map[string]interface{}{"root": "weights", "path": "../../etc/passwd"}, false, "INVALID_PATH", ""},
		{"reveal 目录", "/api/fileroots/reveal", "reveal", map[string]interface{}{"root": "weights", "path": "sub"}, true, "", sub},
		{"reveal 文本", "/api/fileroots/reveal", "reveal", map[string]interface{}{"root": "weights", "path": "sub/a.md"}, true, "", filepath.Join(sub, "a.md")},
		{"reveal 非白名单类型（设计 §4.4：reveal 对任意类型放行）", "/api/fileroots/reveal", "reveal", map[string]interface{}{"root": "weights", "path": "m.gguf"}, true, "", filepath.Join(f.weights, "m.gguf")},
		{"reveal 不存在", "/api/fileroots/reveal", "reveal", map[string]interface{}{"root": "weights", "path": "nope.md"}, false, "NOT_FOUND", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doFB(t, r, http.MethodPost, tc.endpoint, tc.body)
			if tc.wantOK {
				if w.Code != http.StatusOK {
					t.Fatalf("状态码 = %d，期望 200（body=%s）", w.Code, w.Body.String())
				}
				var body struct {
					OK  bool   `json:"ok"`
					Abs string `json:"abs"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatalf("响应不是 JSON: %v", err)
				}
				if !body.OK || body.Abs != tc.wantAbs {
					t.Fatalf("响应 = %+v，期望 ok=true abs=%q", body, tc.wantAbs)
				}
				return
			}
			if w.Code != errorStatus(tc.wantCode) {
				t.Fatalf("状态码 = %d，期望 %d（body=%s）", w.Code, errorStatus(tc.wantCode), w.Body.String())
			}
			if got := respErrorCode(t, w); got != tc.wantCode {
				t.Fatalf("错误码 = %q，期望 %q", got, tc.wantCode)
			}
		})
	}

	// 桩必须被调到，且参数就是"后端算出来的绝对路径"（证明绝对路径不经前端传入）
	calls := stub.snapshot()
	if len(calls) != 6 {
		t.Fatalf("执行桩调用次数 = %d，期望 6（成功的 6 条：3 open + 3 reveal）: %v", len(calls), calls)
	}
	if calls[0][0] != "open" || calls[1][0] != "open" || calls[2][0] != "open" {
		t.Fatalf("前三个应是 open: %v", calls)
	}
	if calls[3][0] != "reveal" || calls[4][0] != "reveal" || calls[5][0] != "reveal" {
		t.Fatalf("后三个应是 reveal: %v", calls)
	}
	if calls[4][1] != filepath.Join(sub, "a.md") {
		t.Fatalf("reveal 传的绝对路径 = %q，期望 %q", calls[4][1], filepath.Join(sub, "a.md"))
	}
	// 最后一条是「reveal 非白名单类型」——它必须真的走到执行桩（证明确实放行而非被闸门挡下）
	if calls[5][1] != filepath.Join(f.weights, "m.gguf") {
		t.Fatalf("reveal(.gguf) 传的绝对路径 = %q，期望 %q", calls[5][1], filepath.Join(f.weights, "m.gguf"))
	}

	t.Run("ALLOW_ALL_TYPES=1 放开非白名单类型", func(t *testing.T) {
		t.Setenv(fileBrowserAllowAllEnv, "1")
		w := doFB(t, r, http.MethodPost, "/api/fileroots/open", map[string]interface{}{"root": "weights", "path": "m.gguf"})
		if w.Code != http.StatusOK {
			t.Fatalf("放开后应 200，实际 %d（body=%s）", w.Code, w.Body.String())
		}
		w = doFB(t, r, http.MethodPost, "/api/fileroots/reveal", map[string]interface{}{"root": "weights", "path": "无扩展名"})
		if w.Code != http.StatusOK {
			t.Fatalf("放开后无扩展名应 200，实际 %d（body=%s）", w.Code, w.Body.String())
		}
	})
}

// ── §七 6：审计日志 ────────────────────────────────────────────────────────

func TestFileroots_AuditLog(t *testing.T) {
	f := setupFB(t)
	mustWriteFile(t, filepath.Join(f.weights, "a.md"), "TOPSECRET-内容-不该进日志\n")
	mustWriteFile(t, filepath.Join(f.weights, "m.gguf"), "G")

	fixed := time.Date(2026, 9, 13, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))

	t.Run("一行一动作_字段完整_失败留痕_不记内容", func(t *testing.T) {
		dir := t.TempDir()
		h, _ := newFBHandlers(t)
		h.FileBrowserAudit = &fileBrowserAuditParams{Dir: dir, Now: func() time.Time { return fixed }}
		r := newFBTestRouter(h)

		// 动作1：成功 open 文本（内容含标记）
		if w := doFB(t, r, http.MethodPost, "/api/fileroots/open", map[string]interface{}{"root": "weights", "path": "a.md"}); w.Code != 200 {
			t.Fatalf("open 文本应 200，实际 %d", w.Code)
		}
		// 动作2：失败 open（非白名单类型）
		if w := doFB(t, r, http.MethodPost, "/api/fileroots/open", map[string]interface{}{"root": "weights", "path": "m.gguf"}); w.Code != 400 {
			t.Fatalf("open gguf 应 400，实际 %d", w.Code)
		}
		// 动作3：成功 reveal
		if w := doFB(t, r, http.MethodPost, "/api/fileroots/reveal", map[string]interface{}{"root": "weights", "path": "a.md"}); w.Code != 200 {
			t.Fatalf("reveal 应 200，实际 %d", w.Code)
		}
		// 动作4：不存在的路径也留痕
		if w := doFB(t, r, http.MethodPost, "/api/fileroots/open", map[string]interface{}{"root": "weights", "path": "nope.md"}); w.Code != 404 {
			t.Fatalf("open 不存在应 404，实际 %d", w.Code)
		}

		entries := readUIActionLog(t, dir)
		if len(entries) != 4 {
			t.Fatalf("审计行数 = %d，期望 4（每动作恰好一行）: %+v", len(entries), entries)
		}
		wantResults := []string{"ok", "NOT_ALLOWED", "ok", "NOT_FOUND"}
		wantActions := []string{"open", "open", "reveal", "open"}
		for i, e := range entries {
			if e.Time != "2026-09-13T10:00:00+08:00" {
				t.Fatalf("第 %d 行时间 = %q，期望 ISO8601 的 2026-09-13T10:00:00+08:00", i, e.Time)
			}
			if e.Action != wantActions[i] || e.Root != "weights" || e.Result != wantResults[i] {
				t.Fatalf("第 %d 行字段不符: %+v（期望 action=%s result=%s root=weights）", i, e, wantActions[i], wantResults[i])
			}
			if e.Path == "" {
				t.Fatalf("第 %d 行缺相对路径: %+v", i, e)
			}
		}

		// 内容绝不入日志：先把文件内容真正读出来（GET），再确认日志里没有那个标记
		w := doFB(t, r, http.MethodGet, docsQuery("weights", "a.md"), nil)
		if !strings.Contains(w.Body.String(), "TOPSECRET") {
			t.Fatalf("前置条件失败：读取接口没返回文件内容")
		}
		raw, err := os.ReadFile(filepath.Join(dir, uiActionsLogName))
		if err != nil {
			t.Fatalf("读审计日志失败: %v", err)
		}
		if strings.Contains(string(raw), "TOPSECRET") {
			t.Fatalf("审计日志里出现了文件内容:\n%s", raw)
		}
		// 字段只有那五个（多一个都不行）
		var probe map[string]interface{}
		if err := json.Unmarshal([]byte(strings.Split(strings.TrimSpace(string(raw)), "\n")[0]), &probe); err != nil {
			t.Fatalf("审计行不是 JSON: %v", err)
		}
		if len(probe) != 5 {
			t.Fatalf("审计字段数 = %d，期望 5（time/action/root/path/result）: %v", len(probe), probe)
		}
	})

	t.Run("满上限轮转_留 N 份", func(t *testing.T) {
		dir := t.TempDir()
		h, _ := newFBHandlers(t)
		h.FileBrowserAudit = &fileBrowserAuditParams{Dir: dir, MaxBytes: 300, Keep: 2, Now: func() time.Time { return fixed }}
		r := newFBTestRouter(h)

		for i := 0; i < 7; i++ {
			p := filepath.Join(f.weights, "sub", "rot"+string(rune('a'+i))+".md")
			mustWriteFile(t, p, "x\n")
			rel := filepath.ToSlash(filepath.Join("sub", "rot"+string(rune('a'+i))+".md"))
			if w := doFB(t, r, http.MethodPost, "/api/fileroots/open", map[string]interface{}{"root": "weights", "path": rel}); w.Code != 200 {
				t.Fatalf("第 %d 次 open 应 200，实际 %d（body=%s）", i, w.Code, w.Body.String())
			}
		}
		logPath := filepath.Join(dir, uiActionsLogName)
		if _, err := os.Stat(logPath); err != nil {
			t.Fatalf("主日志不存在: %v", err)
		}
		for i := 1; i <= 2; i++ {
			if _, err := os.Stat(logPath + "." + string(rune('0'+i))); err != nil {
				t.Fatalf("轮转文件 .%d 不存在: %v", i, err)
			}
		}
		if _, err := os.Stat(logPath + ".3"); !os.IsNotExist(err) {
			t.Fatalf("轮转文件 .3 不该存在（只留 2 份），err=%v", err)
		}
		// 每个文件里的每一行都必须是完整 JSON（轮转不得把一行劈开）
		total := 0
		for _, p := range []string{logPath, logPath + ".1", logPath + ".2"} {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("读 %s 失败: %v", p, err)
			}
			for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
				if line == "" {
					continue
				}
				var e uiActionEntry
				if err := json.Unmarshal([]byte(line), &e); err != nil {
					t.Fatalf("%s 里有被劈开/写坏的行: %q", p, line)
				}
				if e.Action != "open" || e.Result != "ok" {
					t.Fatalf("%s 行内容不符: %+v", p, e)
				}
				total++
			}
		}
		if total < 3 || total > 7 {
			t.Fatalf("轮转后总行数 = %d，期望落在 [3,7]（保留份数内的留存）", total)
		}
	})

	t.Run("30 天前的轮转文件被清理_近期的留下", func(t *testing.T) {
		dir := t.TempDir()
		oldPath := filepath.Join(dir, uiActionsLogName+".3")
		recentPath := filepath.Join(dir, uiActionsLogName+".4")
		mustWriteFile(t, oldPath, "{}\n")
		mustWriteFile(t, recentPath, "{}\n")
		if err := os.Chtimes(oldPath, fixed.Add(-40*24*time.Hour), fixed.Add(-40*24*time.Hour)); err != nil {
			t.Fatalf("回拨旧文件时间失败: %v", err)
		}
		if err := os.Chtimes(recentPath, fixed.Add(-5*24*time.Hour), fixed.Add(-5*24*time.Hour)); err != nil {
			t.Fatalf("设置近期文件时间失败: %v", err)
		}

		h, _ := newFBHandlers(t)
		h.FileBrowserAudit = &fileBrowserAuditParams{Dir: dir, Now: func() time.Time { return fixed }}
		r := newFBTestRouter(h)
		if w := doFB(t, r, http.MethodPost, "/api/fileroots/open", map[string]interface{}{"root": "weights", "path": "a.md"}); w.Code != 200 {
			t.Fatalf("open 应 200，实际 %d", w.Code)
		}
		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Fatalf("40 天前的轮转文件应被清理，err=%v", err)
		}
		if _, err := os.Stat(recentPath); err != nil {
			t.Fatalf("5 天前的轮转文件不该被清理: %v", err)
		}
		if len(readUIActionLog(t, dir)) != 1 {
			t.Fatalf("主日志应恰好 1 行")
		}
	})

	t.Run("默认位置与默认参数", func(t *testing.T) {
		p := (fileBrowserAuditParams{}).resolved()
		if p.MaxBytes != 1<<20 || p.Keep != 5 || p.AgeDays != 30 {
			t.Fatalf("默认轮转参数 = %d/%d/%d，期望 1048576/5/30", p.MaxBytes, p.Keep, p.AgeDays)
		}
		stateDir := filepath.Join(t.TempDir(), "state")
		t.Setenv("ZERG_STATE_DIR", stateDir)
		if got := uiActionsLogDir(); got != filepath.Join(filepath.Dir(stateDir), "logs") {
			t.Fatalf("审计目录 = %q，期望与状态根同级的 logs", got)
		}
		t.Setenv("ZERG_STATE_DIR", "")
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("取不到 HOME: %v", err)
		}
		if got := uiActionsLogDir(); got != filepath.Join(home, ".zerg", "logs") {
			t.Fatalf("缺省审计目录 = %q，期望 <HOME>/.zerg/logs", got)
		}
	})
}

// ── §七 7：读取不设上限（3 MiB 完整返回）──────────────────────────────────

func TestFileroots_ReadNoSizeLimit(t *testing.T) {
	f := setupFB(t)
	const size = 3 * 1024 * 1024 // 3 MiB
	big := make([]byte, size)
	for i := range big {
		big[i] = byte('a' + i%26)
	}
	bigPath := filepath.Join(f.weights, "big.md")
	if err := os.WriteFile(bigPath, big, 0o644); err != nil {
		t.Fatalf("写 3 MiB 文件失败: %v", err)
	}

	h, _ := newFBHandlers(t)
	r := newFBTestRouter(h)
	w := doFB(t, r, http.MethodGet, docsQuery("weights", "big.md"), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200（body 前 200 字节=%s）", w.Code, head(w.Body.String(), 200))
	}
	var body struct {
		Path, Content, Root string
		Size                int
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是 JSON: %v", err)
	}
	if body.Path != "big.md" || body.Root != "weights" {
		t.Fatalf("path/root 字段 = %q/%q", body.Path, body.Root)
	}
	if len(body.Content) != size || body.Size != size {
		t.Fatalf("返回字节数 = %d（size 字段 %d），期望 %d —— 不得截断/不得拒读", len(body.Content), body.Size, size)
	}
	onDisk, err := os.ReadFile(bigPath)
	if err != nil {
		t.Fatalf("回读磁盘文件失败: %v", err)
	}
	if len(onDisk) != size {
		t.Fatalf("磁盘文件字节数 = %d，期望 %d", len(onDisk), size)
	}
	if !bytes.Equal([]byte(body.Content), onDisk) {
		t.Fatalf("返回内容与磁盘不一致（或尾部被截断）")
	}
}

func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ── §七 8/9：契约（五项顺序 + 环境变量 + 类型放开可配置）───────────────────

func TestFileroots_Contract(t *testing.T) {
	f := setupFB(t)
	h, _ := newFBHandlers(t)
	r := newFBTestRouter(h)

	fetch := func(t *testing.T) ([]FileRoot, FileBrowserConfig) {
		t.Helper()
		w := doFB(t, r, http.MethodGet, "/api/fileroots", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("状态码 = %d，期望 200（body=%s）", w.Code, w.Body.String())
		}
		var body struct {
			Roots  []FileRoot        `json:"roots"`
			Config FileBrowserConfig `json:"config"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("响应不是 JSON: %v", err)
		}
		return body.Roots, body.Config
	}

	t.Run("恰好五项_顺序与标记", func(t *testing.T) {
		roots, cfg := fetch(t)
		wantIDs := []string{"docs", "repo", "models", "tasks", "weights"}
		wantLabels := []string{"虫族文档", "虫族仓库", "模型登记库", "任务目录", "模型权重"}
		if len(roots) != 5 {
			t.Fatalf("根数 = %d，期望 5: %+v", len(roots), roots)
		}
		for i, root := range roots {
			if root.ID != wantIDs[i] {
				t.Fatalf("第 %d 个根 id = %q，期望 %q", i, root.ID, wantIDs[i])
			}
			if root.Label != wantLabels[i] {
				t.Fatalf("第 %d 个根 label = %q，期望 %q", i, root.Label, wantLabels[i])
			}
			if want := i == 0; root.Default != want {
				t.Fatalf("根 %s 的 default = %v，期望 %v", root.ID, root.Default, want)
			}
			if want := i == 0; root.Writable != want {
				t.Fatalf("根 %s 的 writable = %v，期望 %v", root.ID, root.Writable, want)
			}
		}
		if cfg.DisplayMax != 1048576 || cfg.AllowAllTypes {
			t.Fatalf("缺省 config = %+v，期望 display_max=1048576 / allow_all_types=false", cfg)
		}
		if strings.Join(cfg.TextExts, ",") != strings.Join(defaultTextExts, ",") {
			t.Fatalf("缺省 text_exts = %v，期望 %v", cfg.TextExts, defaultTextExts)
		}
	})

	t.Run("路径尊重各环境变量", func(t *testing.T) {
		roots, _ := fetch(t)
		want := map[string]string{
			"docs":    filepath.Join(f.repo, "docs"),
			"repo":    f.repo,
			"models":  f.models,
			"tasks":   f.tasks,
			"weights": f.weights,
		}
		for _, root := range roots {
			if root.Path != want[root.ID] {
				t.Fatalf("根 %s 路径 = %q，期望 %q", root.ID, root.Path, want[root.ID])
			}
		}
		if got := modelreg.DefaultModelsDir(); got != f.models {
			t.Fatalf("modelreg 解析器与本接口不同源: %q != %q", got, f.models)
		}
	})

	t.Run("不存在的根也照常返回", func(t *testing.T) {
		gone := filepath.Join(t.TempDir(), "不存在")
		t.Setenv("ZERG_WEIGHTS_DIR", gone)
		t.Setenv("ZERG_MODELS_DIR", gone)
		roots, _ := fetch(t)
		if len(roots) != 5 {
			t.Fatalf("根数 = %d，期望仍是 5（不跳过不存在的根）", len(roots))
		}
		for _, root := range roots {
			if (root.ID == "weights" || root.ID == "models") && root.Path != gone {
				t.Fatalf("根 %s 路径 = %q，期望 %q", root.ID, root.Path, gone)
			}
		}
		// 列表接口对不存在的根返回空列表而非报错
		w := doFB(t, r, http.MethodGet, docsQuery("weights", ""), nil)
		if w.Code != http.StatusOK {
			t.Fatalf("不存在的根列表状态码 = %d，期望 200（body=%s）", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "\"files\":[]") {
			t.Fatalf("不存在的根应返回空 files: %s", w.Body.String())
		}
		// 读文件才报 NOT_FOUND
		w = doFB(t, r, http.MethodGet, docsQuery("weights", "x.md"), nil)
		if w.Code != http.StatusNotFound || respErrorCode(t, w) != "NOT_FOUND" {
			t.Fatalf("不存在根里读文件应 404/NOT_FOUND，实际 %d/%s", w.Code, w.Body.String())
		}
	})

	t.Run("类型放开是可配置的", func(t *testing.T) {
		t.Setenv(fileBrowserTextExtsEnv, " md , .GGUF , .foo ")
		t.Setenv(fileBrowserAllowAllEnv, "1")
		t.Setenv(fileBrowserDisplayMaxEnv, "2048")
		_, cfg := fetch(t)
		if cfg.DisplayMax != 2048 {
			t.Fatalf("display_max = %d，期望 2048（改配置即生效）", cfg.DisplayMax)
		}
		if !cfg.AllowAllTypes {
			t.Fatalf("allow_all_types 应为 true")
		}
		if strings.Join(cfg.TextExts, ",") != ".md,.gguf,.foo" {
			t.Fatalf("text_exts = %v，期望 [.md .gguf .foo]（补前导点 + 小写 + 去空格）", cfg.TextExts)
		}
	})

	t.Run("类型清单与放开开关直接影响读接口", func(t *testing.T) {
		mustWriteFile(t, filepath.Join(f.weights, "b.gguf"), "G")
		// 默认：.gguf 不读入 UI
		w := doFB(t, r, http.MethodGet, docsQuery("weights", "b.gguf"), nil)
		if w.Code != http.StatusBadRequest || respErrorCode(t, w) != "NOT_ALLOWED" {
			t.Fatalf("默认读 .gguf 应 400/NOT_ALLOWED，实际 %d/%s", w.Code, w.Body.String())
		}
		// 往清单里加 .gguf（不改代码）
		t.Setenv(fileBrowserTextExtsEnv, ".gguf")
		w = doFB(t, r, http.MethodGet, docsQuery("weights", "b.gguf"), nil)
		if w.Code != http.StatusOK {
			t.Fatalf("清单加 .gguf 后应 200，实际 %d（body=%s）", w.Code, w.Body.String())
		}
	})

	t.Run("新接口不是免鉴权旁路", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/fileroots", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("无令牌状态码 = %d，期望 401", w.Code)
		}
	})
}
