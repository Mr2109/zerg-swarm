package api

// resource_trust_ascii_test.go —— 信任级别 ASCII 化（2026-09-23 波F · 设计-CI适配-v1.1 §九）。
//
// 病（现读 · 不是推断）：三个信任级别（正式/未知/空串）在**数据面**里是中文 ——
//   `ui/src/app.rs` 拿中文字面量直比服务端状态串；服务端发出去的 JSON 也是中文。
// 本波（破坏性接口变更 · 提案 DEV-0053 · 服务端与 UI 同批）：
//   ① 服务端发 **ASCII 机器码**（闭集 new/official/unknown）——API JSON 与状态文件都是；
//   ② 旧中文件读入时翻译一次（statusCode），写侧只写机器码，旧文件不删不改；
//   ③ **端点级**断言：把「服务端真发的字节里 trust 是 ASCII」钉在 httptest
//      （真 chi 路由 + AuthMiddleware）上 —— 不靠读代码相信，靠现跑读数。
//
// 本文件只加不改：resource_trust_migrate_test.go 的路径/迁移判据一字未动
// （唯一随动 = ⑦ 的期望值由中文改成机器码 official —— 契约改了，期望值必须跟着改，不是放宽）。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// asciiCodeRe 机器码硬形状：纯 ASCII 小写字母（§九 铁律「面向机器的一律 ASCII」）。
var asciiCodeRe = regexp.MustCompile(`^[a-z]+$`)

// trustFieldRe 从**真发出去的字节**里抠 trust 字段（只看这一格，别的字段不归本波管）。
var trustFieldRe = regexp.MustCompile(`"trust":"([^"]*)"`)

// trustClosedSet 三个机器码的闭集（加新码 = 破坏性接口变更 ⇒ 走提案）。
func trustClosedSet() map[string]bool {
	return map[string]bool{TrustNew: true, TrustOfficial: true, TrustUnknown: true}
}

// assertASCIIMachineCode 判一个串是闭集里的机器码（端点读数逐格用）。
func assertASCIIMachineCode(t *testing.T, where, got string) {
	t.Helper()
	if !asciiCodeRe.MatchString(got) {
		t.Fatalf("%s：信任级别必须是纯 ASCII 机器码（现读 %q）—— 数据面夹中文/空串都算破（§九）", where, got)
	}
	if !trustClosedSet()[got] {
		t.Fatalf("%s：机器码 %q 不在闭集 new/official/unknown 里（闭集外的值不许过界）", where, got)
	}
}

// ── ① 常量与翻译面 ──────────────────────────────────────────────────────────────

func TestTrustStatusMachineCodesAreASCII(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range []string{TrustNew, TrustOfficial, TrustUnknown} {
		if !asciiCodeRe.MatchString(c) {
			t.Fatalf("信任级别机器码必须是纯 ASCII 小写字母，实得 %q", c)
		}
		for _, r := range c {
			if r > 127 {
				t.Fatalf("机器码含非 ASCII 字符：%q", c)
			}
		}
		if seen[c] {
			t.Fatalf("机器码重复：%q（三个级别必须可分）", c)
		}
		seen[c] = true
	}
	// 旧中文 → 机器码（幂等：机器码再过一次还是它自己）
	cases := []struct{ in, want string }{
		{legacyStatusNew, TrustNew},
		{legacyStatusOfficial, TrustOfficial},
		{legacyStatusUnknown, TrustUnknown},
		{"", TrustUnknown}, // 空串不是码（闭集里没有空串）⇒ 落 unknown
		{TrustNew, TrustNew},
		{TrustOfficial, TrustOfficial},
		{TrustUnknown, TrustUnknown},
	}
	for _, c := range cases {
		if got := statusCode(c.in); got != c.want {
			t.Fatalf("statusCode(%q) = %q，want %q", c.in, got, c.want)
		}
	}
	// 认不出的值原样返回（缺就缺 —— 不猜、不硬塞成 unknown）
	if got := statusCode("experimental"); got != "experimental" {
		t.Fatalf("认不出的状态应原样返回（不猜），实得 %q", got)
	}
}

// ── ② 旧中文件 → 读入翻译 → 写侧只写机器码 ──────────────────────────────────────

func TestResourceTrust_LegacyChineseStatusTranslatedOnLoad(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	setResTrustPaths(t, "", filepath.Join(t.TempDir(), "no-legacy.json"))
	rt := resetResourceTrust(t)

	// 造一份 **2026-09-23 之前** 落盘的状态文件：四个桶里都是中文状态。
	legacyDoc := &ResourceTrust{
		Models: map[string]ResTrustEntry{"m-official": {Status: legacyStatusOfficial, Uses: 100}},
		Tools:  map[string]ResTrustEntry{"t-new": {Status: legacyStatusNew, Uses: 3}},
		Skills: map[string]ResTrustEntry{"s-unknown": {Status: legacyStatusUnknown}},
		Mcps:   map[string]ResTrustEntry{"x-empty": {Status: ""}},
	}
	buf, err := json.MarshalIndent(legacyDoc, "", "  ")
	if err != nil {
		t.Fatalf("序列化旧中文件失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, resTrustStateFileName), buf, 0o644); err != nil {
		t.Fatalf("写旧中文件失败: %v", err)
	}

	LoadResourceTrust()

	for _, c := range []struct{ kind, name, want string }{
		{"models", "m-official", TrustOfficial},
		{"tools", "t-new", TrustNew},
		{"skills", "s-unknown", TrustUnknown},
		{"mcp", "x-empty", TrustUnknown}, // 空串 → unknown（不是空串过界）
	} {
		if got := rt.GetResourceStatus(c.kind, c.name); got != c.want {
			t.Fatalf("旧中文件读入后 GetResourceStatus(%s,%s) = %q，want %q", c.kind, c.name, got, c.want)
		}
	}
	// 未注册 ⇒ unknown（不是空串、不是中文）
	assertASCIIMachineCode(t, "未注册资源", rt.GetResourceStatus("models", "never-seen"))
	// 桶都不认 ⇒ unknown
	if got := rt.GetResourceStatus("no-such-kind", "x"); got != TrustUnknown {
		t.Fatalf("不认的资源类型应回 %q，实得 %q", TrustUnknown, got)
	}

	// 写侧：一次写盘后，落盘文件里**不许**再出现中文状态（旧文件本身不动 —— 那是另一处判据）。
	rt.UseResource("tools", "t-new")
	raw, err := os.ReadFile(filepath.Join(stateDir, resTrustStateFileName))
	if err != nil {
		t.Fatalf("写盘后读新文件失败: %v", err)
	}
	for _, zh := range []string{legacyStatusNew, legacyStatusOfficial, legacyStatusUnknown} {
		if strings.Contains(string(raw), zh) {
			t.Fatalf("新落盘的文件里仍含中文状态 %q ⇒ 写侧漏了（§九：数据面一律 ASCII）", zh)
		}
	}
	var doc ResourceTrust
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("新落盘文件非法 JSON: %v", err)
	}
	for k, e := range doc.Tools {
		assertASCIIMachineCode(t, "写盘文件 tools["+k+"]", statusCode(e.Status))
	}
}

// ── ③ 端点级：服务端真发的字节里 trust 是 ASCII 机器码 ──────────────────────────

// newTrustRouter 与 main.go 注册方式一致（同一套 AuthMiddleware + 同一个 ResourcesHandler）。
func newTrustRouter(h *Handlers) *chi.Mux {
	r := chi.NewRouter()
	r.Use(AuthMiddleware("test-token"))
	r.Get("/api/resources/{type}", h.ResourcesHandler)
	return r
}

// getResources 取一个资源桶：真走 HTTP（路由 + 鉴权 + JSON 序列化），返回 (items, 原始响应体)。
func getResources(t *testing.T, r *chi.Mux, resType string) ([]map[string]interface{}, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/resources/"+resType, nil)
	req.Header.Set("X-Auth-Token", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/resources/%s ⇒ HTTP %d，body=%s", resType, w.Code, w.Body.String())
	}
	raw := w.Body.String()
	var doc struct {
		Type  string                   `json:"type"`
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("GET /api/resources/%s 响应非法 JSON: %v", resType, err)
	}
	return doc.Items, raw
}

// trustValuesIn 从真发出去的字节里逐格抠 trust（判的是**响应体本身**，不是内存里的 map）。
func trustValuesIn(t *testing.T, resType, raw string) []string {
	t.Helper()
	vals := []string{}
	for _, m := range trustFieldRe.FindAllStringSubmatch(raw, -1) {
		vals = append(vals, m[1])
	}
	if len(vals) == 0 {
		t.Fatalf("GET /api/resources/%s 的响应体里一个 trust 字段都没有 ⇒ 空转不算绿（body=%s）", resType, raw)
	}
	return vals
}

func TestResourcesHandler_TrustIsASCIIMachineCode(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", stateDir)
	skillsDir := t.TempDir()
	t.Setenv("ZERG_SKILLS_DIR", skillsDir)
	setResTrustPaths(t, "", filepath.Join(t.TempDir(), "no-legacy.json"))
	_ = resetResourceTrust(t)

	// 造一个真技能目录（ResourcesHandler 只认含 SKILL.md 的目录）+ 一份**旧中文**信任表：
	// 这样端点上的 trust 既走过「读表」又走过「翻译」，读数最硬。
	if err := os.MkdirAll(filepath.Join(skillsDir, "zerg-skill-demo"), 0o755); err != nil {
		t.Fatalf("造技能目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "zerg-skill-demo", "SKILL.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatalf("造 SKILL.md 失败: %v", err)
	}
	legacyDoc := &ResourceTrust{
		Models: map[string]ResTrustEntry{},
		Tools:  map[string]ResTrustEntry{},
		Skills: map[string]ResTrustEntry{"zerg-skill-demo": {Status: legacyStatusOfficial, Uses: 100}},
		Mcps:   map[string]ResTrustEntry{},
	}
	buf, _ := json.MarshalIndent(legacyDoc, "", "  ")
	if err := os.WriteFile(filepath.Join(stateDir, resTrustStateFileName), buf, 0o644); err != nil {
		t.Fatalf("写旧中文信任表失败: %v", err)
	}
	LoadResourceTrust()

	h := newTestHandlers()
	h.ModelsDir = t.TempDir() // 不读真机 ~/.zerg/models
	r := newTrustRouter(h)

	// (a) tools 面：trust 由 trustOf 算（发码件 = handlers.go）
	tools, rawTools := getResources(t, r, "tools")
	if len(tools) == 0 {
		t.Fatalf("tools 面 0 项 ⇒ 端点没跑到（空转不算绿）")
	}
	for _, v := range trustValuesIn(t, "tools", rawTools) {
		assertASCIIMachineCode(t, "tools 面 trust 字段", v)
	}

	// (b) skills 面：trust 来自信任表 —— 表里那份是**旧中文**，端点必须发机器码 official
	skills, rawSkills := getResources(t, r, "skills")
	var found bool
	for _, it := range skills {
		if name, _ := it["name"].(string); name == "zerg-skill-demo" {
			found = true
		}
	}
	if !found {
		t.Fatalf("skills 面没列到夹具技能 ⇒ 端点没跑到（空转不算绿）: %s", rawSkills)
	}
	var sawOfficial bool
	for _, v := range trustValuesIn(t, "skills", rawSkills) {
		assertASCIIMachineCode(t, "skills 面 trust 字段", v)
		if v == TrustOfficial {
			sawOfficial = true
		}
	}
	if !sawOfficial {
		t.Fatalf("旧中文「正式」进表 ⇒ 端点应发 %q，响应里没看到: %s", TrustOfficial, rawSkills)
	}
	if strings.Contains(rawSkills, legacyStatusOfficial) {
		t.Fatalf("skills 面响应里出现中文状态 ⇒ 服务端还在发中文: %s", rawSkills)
	}

	// (c) models 面：空注册表也不许发出空串/中文（发出去的 trust 逐格过闭集）
	_, rawModels := getResources(t, r, "models")
	if vals := trustFieldRe.FindAllStringSubmatch(rawModels, -1); len(vals) > 0 {
		for _, m := range vals {
			assertASCIIMachineCode(t, "models 面 trust 字段", m[1])
		}
	}
}
