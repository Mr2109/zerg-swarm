package api

// resources_ledger_test.go —— GET /api/resources/ledger 的反例优先测试（httptest + 真实 chi 路由 + AuthMiddleware）。
//
// 覆盖：鉴权（401/403）· 静态段优先于 /api/resources/{type} · 缺就缺（不出现假值、不 500）·
// 严格只读（请求前后文件清单逐字相同）· estimated（有真值=false / 缺 KV=true 且 basis 非空）。

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/go-chi/chi/v5"
)

// ── 假 GGUF 构造（照 GGUF v3 规范；与 modelreg 测试同形，独立一份避免跨包依赖）──

const (
	ledgerGGUFStr = 8 // GGUF 值类型 STRING
	ledgerGGUFU32 = 4 // GGUF 值类型 UINT32
)

func ledgerLE32(v uint32) []byte { b := make([]byte, 4); binary.LittleEndian.PutUint32(b, v); return b }
func ledgerLE64(v uint64) []byte { b := make([]byte, 8); binary.LittleEndian.PutUint64(b, v); return b }
func ledgerStr(s string) []byte  { return append(ledgerLE64(uint64(len(s))), []byte(s)...) }

func ledgerKV(key string, vt uint32, val []byte) []byte {
	out := ledgerStr(key)
	out = append(out, ledgerLE32(vt)...)
	return append(out, val...)
}

func ledgerGGUF(kvs ...[]byte) []byte {
	out := []byte("GGUF")
	out = append(out, ledgerLE32(3)...)                // version
	out = append(out, ledgerLE64(0)...)                // tensor_count
	out = append(out, ledgerLE64(uint64(len(kvs)))...) // kv_count
	for _, x := range kvs {
		out = append(out, x...)
	}
	return out
}

// writeLedgerGGUF 造一个 GGUF：withKV=true 带 KV 真值键（head_count_kv/key_length），
// false 则只有层数（模拟"缺 KV 参数"）。
func writeLedgerGGUF(t *testing.T, dir, name string, withKV bool) string {
	t.Helper()
	kvs := [][]byte{
		ledgerKV("general.architecture", ledgerGGUFStr, ledgerStr("llama")),
		ledgerKV("llama.context_length", ledgerGGUFU32, ledgerLE32(4096)),
		ledgerKV("llama.block_count", ledgerGGUFU32, ledgerLE32(32)),
		ledgerKV("llama.embedding_length", ledgerGGUFU32, ledgerLE32(4096)),
	}
	if withKV {
		kvs = append(kvs,
			ledgerKV("llama.attention.head_count", ledgerGGUFU32, ledgerLE32(32)),
			ledgerKV("llama.attention.head_count_kv", ledgerGGUFU32, ledgerLE32(8)),
			ledgerKV("llama.attention.key_length", ledgerGGUFU32, ledgerLE32(128)),
		)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, ledgerGGUF(kvs...), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// writeLedgerRecord 在 <root>/manifests/<id>/<ver>.json 写一条最简登记记录。
func writeLedgerRecord(t *testing.T, root, id, ver, fileName string, size int64) {
	t.Helper()
	dir := filepath.Join(root, "manifests", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	rec := map[string]interface{}{
		"schema":         "zerg.model.v1",
		"id":             id,
		"digest":         "sha256:" + ver,
		"name":           "TestModel",
		"context_window": 4096,
		"files": []map[string]interface{}{
			{"role": "weights", "name": fileName, "sha256": "deadbeef", "size": size},
		},
	}
	b, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(dir, ver+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// newLedgerHandlers 造一个模型目录=空临时目录的 Handlers（避免读到真实的 ~/.zerg/models）。
func newLedgerHandlers(t *testing.T) (*Handlers, string) {
	t.Helper()
	dir := t.TempDir()
	h := newTestHandlers()
	h.ModelsDir = dir
	h.Config = &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{},
		Fleet:  map[string]config.FleetNode{},
	}
	return h, dir
}

// newResourcesTestRouter 与 main.go 注册方式一致（同一套 AuthMiddleware），
// 并额外注册既有的 GET /api/resources/{type}（桩占位），验证静态段优先。
func newResourcesTestRouter(h *Handlers) *chi.Mux {
	r := chi.NewRouter()
	r.Use(AuthMiddleware("test-token"))
	r.Get("/api/resources/ledger", h.ResourceLedgerHandler)
	r.Post("/api/resources/pin", h.ResourcePinHandler)
	r.Post("/api/resources/unpin", h.ResourceUnpinHandler)
	r.Get("/api/resources/{type}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"type": "stub"})
	})
	return r
}

func doResReq(t *testing.T, r *chi.Mux, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	if token != "" {
		req.Header.Set("X-Auth-Token", token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeLedger(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("响应不是 JSON: %q (%v)", w.Body.String(), err)
	}
	return m
}

// resTree 捕获目录树的 相对路径|大小|权限——比只列名字更严的只读断言。
func resTree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		size, mode := int64(-1), os.FileMode(0)
		if info != nil {
			size, mode = info.Size(), info.Mode()
		}
		out = append(out, rel+"|"+itoa(size)+"|"+mode.String())
		return nil
	})
	sort.Strings(out)
	return out
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [24]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// machinesOf 把响应里的 machines 数组按机器名索引。
func machinesOf(t *testing.T, m map[string]interface{}) map[string]map[string]interface{} {
	t.Helper()
	raw, _ := m["machines"].([]interface{})
	out := map[string]map[string]interface{}{}
	for _, x := range raw {
		obj, ok := x.(map[string]interface{})
		if !ok {
			t.Fatalf("machines 元素不是对象: %T", x)
		}
		out[obj["machine"].(string)] = obj
	}
	return out
}

// firstFit 取某台机器 fit 数组的第一项。
func firstFit(t *testing.T, machine map[string]interface{}) map[string]interface{} {
	t.Helper()
	arr, ok := machine["fit"].([]interface{})
	if !ok || len(arr) == 0 {
		t.Fatalf("机器 %v 的 fit 应为非空数组，实得 %v", machine["machine"], machine["fit"])
	}
	fit, ok := arr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("fit 元素不是对象: %T", arr[0])
	}
	return fit
}

// ── 鉴权：无令牌 401 / 错令牌 403（三个接口都不是免鉴权旁路） ──────────────────

func TestResourceLedger_RequiresAuth(t *testing.T) {
	h := newTestHandlers()
	r := newResourcesTestRouter(h)

	if w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("GET 无令牌应 401，实得 %d body=%s", w.Code, w.Body.String())
	}
	if w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "wrong", ""); w.Code != http.StatusForbidden {
		t.Fatalf("GET 错令牌应 403，实得 %d body=%s", w.Code, w.Body.String())
	}
	body := `{"host":"x3","model":"m","ttl_s":60}`
	if w := doResReq(t, r, http.MethodPost, "/api/resources/pin", "", body); w.Code != http.StatusUnauthorized {
		t.Fatalf("pin 无令牌应 401，实得 %d", w.Code)
	}
	if w := doResReq(t, r, http.MethodPost, "/api/resources/pin", "wrong", body); w.Code != http.StatusForbidden {
		t.Fatalf("pin 错令牌应 403，实得 %d", w.Code)
	}
	if w := doResReq(t, r, http.MethodPost, "/api/resources/unpin", "", `{"host":"x3","model":"m"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("unpin 无令牌应 401，实得 %d", w.Code)
	}
}

// ── 路由：静态段 /api/resources/ledger 必须赢过既有的参数段 /api/resources/{type} ──

func TestResourceLedger_RouteBeatsTypeParam(t *testing.T) {
	h := newTestHandlers()
	r := newResourcesTestRouter(h)

	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实得 %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"machines"`) {
		t.Fatalf("应命中账本处理器（响应含 machines），实得 %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"stub"`) {
		t.Fatalf("被 /api/resources/{type} 参数段抢走了，实得 %s", w.Body.String())
	}
}

// ── 缺就缺：无心跳字段/无驻留/无显存 → 对应键整键不出现，且不 500 ──────────────

func TestResourceLedger_MissingDataKeepsKeysAbsent(t *testing.T) {
	h, _ := newLedgerHandlers(t)
	// x3：内存已知、显存未知、无驻留、无未托管
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "x3", MemTotalGb: 128, MemAvailableGb: 120})
	// mini1：连内存数据都没有
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "mini1"})

	r := newResourcesTestRouter(h)
	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("缺数据必须 200（不许 500），实得 %d body=%s", w.Code, w.Body.String())
	}
	byName := machinesOf(t, decodeLedger(t, w))
	if len(byName) != 2 {
		t.Fatalf("应有 2 台机器，实得 %d", len(byName))
	}

	x3 := byName["x3"]
	if x3["mem_known"] != true {
		t.Errorf("x3 有内存数据，mem_known 应为 true，实得 %v", x3["mem_known"])
	}
	if x3["vram_known"] != false {
		t.Errorf("x3 未提供显存，vram_known 应为 false，实得 %v", x3["vram_known"])
	}
	for _, k := range []string{"resident", "unmanaged", "vram_total_gb", "vram_used_gb", "vram_free_gb", "fit"} {
		if v, has := x3[k]; has {
			t.Errorf("x3 未提供 %s，该键必须整键不出现（不造值），实得 %v", k, v)
		}
	}

	mini := byName["mini1"]
	if mini["mem_known"] != false {
		t.Errorf("mini1 无内存数据，mem_known 应为 false，实得 %v", mini["mem_known"])
	}
	for _, k := range []string{"mem_total_gb", "mem_available_gb", "resident"} {
		if v, has := mini[k]; has {
			t.Errorf("mini1 未提供 %s，该键必须整键不出现，实得 %v", k, v)
		}
	}
}

// ── 严格只读：请求前后目录/文件清单逐字相同 ────────────────────────────────

func TestResourceLedger_StrictlyReadOnly(t *testing.T) {
	h, dir := newLedgerHandlers(t)
	writeLedgerGGUF(t, dir, "kvtruth.gguf", true)
	writeLedgerRecord(t, dir, "testmodel", "sha256-abc", "kvtruth.gguf", int64(4)<<30)
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "local", MemTotalGb: 64, MemAvailableGb: 32})

	before := resTree(t, dir)
	r := newResourcesTestRouter(h)
	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实得 %d body=%s", w.Code, w.Body.String())
	}
	after := resTree(t, dir)
	if strings.Join(before, "|") != strings.Join(after, "|") {
		t.Fatalf("GET 前后文件清单变化（只读被破坏）:\n before=%v\n after =%v", before, after)
	}
}

// ── estimated：全真值 → false；缺 KV → true 且 basis 非空 ────────────────────

func TestResourceLedger_EstimatedFalseWhenAllInputsReal(t *testing.T) {
	h, dir := newLedgerHandlers(t)
	gguf := writeLedgerGGUF(t, dir, "kvtruth.gguf", true)
	writeLedgerRecord(t, dir, "testmodel", "sha256-abc", "kvtruth.gguf", int64(4)<<30)
	h.Config = &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"TestModel": {{Host: "local", File: gguf, Architecture: "llama"}},
		},
		Fleet: map[string]config.FleetNode{},
	}
	h.KvCacheBytesPerElem = 2.0 // 引擎 KV dtype 真值（fp16）
	h.EngineOverheadGb = 2.0    // 引擎开销真值
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "local", MemTotalGb: 128, MemAvailableGb: 120})

	r := newResourcesTestRouter(h)
	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实得 %d body=%s", w.Code, w.Body.String())
	}
	fit := firstFit(t, machinesOf(t, decodeLedger(t, w))["local"])
	if fit["estimated"] != false {
		t.Fatalf("全部真值（KV 头数/头维度/dtype/开销齐备）应 estimated=false，实得 %v basis=%v",
			fit["estimated"], fit["basis"])
	}
	if fit["verdict"] != "fit" {
		t.Fatalf("内存充裕应判 fit，实得 %v basis=%v", fit["verdict"], fit["basis"])
	}
	if s, _ := fit["basis"].(string); strings.TrimSpace(s) == "" {
		t.Fatalf("basis 必须非空（可解释性）")
	}
	t.Logf("estimated=false 用例响应 fit=%v", fit)
}

func TestResourceLedger_EstimatedTrueWhenKVMissing(t *testing.T) {
	h, dir := newLedgerHandlers(t)
	gguf := writeLedgerGGUF(t, dir, "nokv.gguf", false) // 无 head_count_kv / key_length
	writeLedgerRecord(t, dir, "testmodel", "sha256-def", "nokv.gguf", int64(4)<<30)
	h.Config = &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"TestModel": {{Host: "local", File: gguf, Architecture: "llama"}},
		},
		Fleet: map[string]config.FleetNode{},
	}
	h.KvCacheBytesPerElem = 2.0
	h.EngineOverheadGb = 2.0
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "local", MemTotalGb: 128, MemAvailableGb: 120})

	r := newResourcesTestRouter(h)
	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实得 %d body=%s", w.Code, w.Body.String())
	}
	fit := firstFit(t, machinesOf(t, decodeLedger(t, w))["local"])
	if fit["estimated"] != true {
		t.Fatalf("缺 KV 参数必须 estimated=true（不许冒充实测），实得 %v", fit["estimated"])
	}
	basis, _ := fit["basis"].(string)
	if strings.TrimSpace(basis) == "" {
		t.Fatalf("estimated=true 时 basis 必须非空")
	}
	if !strings.Contains(basis, "回退") {
		t.Errorf("basis 应写明用了架构族回退，实得 %q", basis)
	}
	t.Logf("estimated=true 用例响应 fit=%v", fit)
}

// ── 反例：坏记录不能让整个账本挂掉（不 500），且坏记录不产生估算项 ──────────────

func TestResourceLedger_BadRecordDoesNotFailRequest(t *testing.T) {
	h, dir := newLedgerHandlers(t)
	gguf := writeLedgerGGUF(t, dir, "good.gguf", true)
	writeLedgerRecord(t, dir, "goodmodel", "sha256-aaa", "good.gguf", int64(4)<<30)
	// 坏记录：不是合法 JSON
	badDir := filepath.Join(dir, "manifests", "badmodel")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "sha256-bad.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.Config = &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"TestModel": {{Host: "local", File: gguf, Architecture: "llama"}},
		},
		Fleet: map[string]config.FleetNode{},
	}
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{Machine: "local", MemTotalGb: 64, MemAvailableGb: 32})

	r := newResourcesTestRouter(h)
	w := doResReq(t, r, http.MethodGet, "/api/resources/ledger", "test-token", "")
	if w.Code != http.StatusOK {
		t.Fatalf("坏记录必须 200（不许 500），实得 %d body=%s", w.Code, w.Body.String())
	}
	m := decodeLedger(t, w)
	// 只有好记录进入估算（坏记录被跳过）
	if got := m["registered_models"]; got != float64(1) {
		t.Errorf("坏记录应被跳过，registered_models 应为 1，实得 %v", got)
	}
}
