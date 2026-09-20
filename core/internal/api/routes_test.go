package api

// routes_test.go — 真源表与「三面同源」的机器判据（2026-09-20 批 B · ②）
//
// 本文件回答三个问题，每个都能**独立变红**：
//  ① 表里的每个端点，主控**真的注册过**吗（扫 `core/cmd/zerg-core/main.go` 的真实注册行）——
//     防「幻影端点」（设计稿承诺过、代码没有的那种）。
//  ② 能力清单与 OpenAPI 路径表**是不是同一套路径** —— 防两面各写各的（今天实测 17 vs 8）。
//  ③ 自描述三面**列不列自己** + 命令面（`zerg api ls|openapi|help`）投影的那几条在不在表里。

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// reRouteReg 匹配主控里的一条注册：`r.Get("/api/…", …)`。
// 为什么扫文本而不是反射运行期 mux：主控的注册面全在 main.go 的字面量里（实测 56 条、**无**变量/循环式注册），
// 扫文本能在**单测**里跑，不需要起服务、不占端口、不碰生产。
var reRouteReg = regexp.MustCompile(`r\.(Get|Post|Delete|Put|Patch)\("([^"]+)"`)

// mainGoPath 主控 main.go 的路径（本包目录 = core/internal/api）。
func mainGoPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "cmd", "zerg-core", "main.go")
	if _, err := os.Stat(p); err != nil {
		t.Skipf("找不到主控 main.go（%s）——公开快照形态下跳过；%v", p, err)
	}
	return p
}

// realRoutes 真实注册表的集合，元素形如 `GET /api/tasks`。
func realRoutes(t *testing.T) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(mainGoPath(t))
	if err != nil {
		t.Fatalf("读主控 main.go 失败: %v", err)
	}
	out := map[string]bool{}
	for _, m := range reRouteReg.FindAllStringSubmatch(string(b), -1) {
		out[strings.ToUpper(m[1])+" "+m[2]] = true
	}
	if len(out) == 0 {
		t.Fatal("主控 main.go 里一条路由注册都没扫到（空转 = 假覆盖）⇒ 判据不成立")
	}
	return out
}

// TestRouteTableMatchesRealRoutes —— 真源表 ⊆ 主控真实路由表（逐条）。
func TestRouteTableMatchesRealRoutes(t *testing.T) {
	real := realRoutes(t)
	missing := []string{}
	for _, e := range routeTable {
		if !real[e.Method+" "+e.Path] {
			missing = append(missing, e.Method+" "+e.Path)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("真源表里有 %d 条端点主控**没有注册**（幻影端点）：\n  %s\n注册面实际有 %d 条",
			len(missing), strings.Join(missing, "\n  "), len(real))
	}
	t.Logf("真源表 %d 条端点逐条命中主控真实路由表（注册面共 %d 条）", len(routeTable), len(real))
}

// TestCapabilitiesAndOpenAPIPathsAreAligned —— 两面**由同一张表派生**，键集必须相等。
func TestCapabilitiesAndOpenAPIPathsAreAligned(t *testing.T) {
	caps := capabilityItems()
	paths := openapiPaths()

	// ① 能力条数 == 表条数（派生，不是各写各的）
	if len(caps) != len(routeTable) {
		t.Fatalf("能力条数 %d ≠ 真源表条数 %d", len(caps), len(routeTable))
	}
	// ② 能力端点集（去重路径）== OpenAPI 路径集
	want := map[string]bool{}
	for _, p := range routePaths() {
		want[p] = true
	}
	got := map[string]bool{}
	for p := range paths {
		got[p] = true
	}
	var onlyWant, onlyGot []string
	for p := range want {
		if !got[p] {
			onlyWant = append(onlyWant, p)
		}
	}
	for p := range got {
		if !want[p] {
			onlyGot = append(onlyGot, p)
		}
	}
	if len(onlyWant) > 0 || len(onlyGot) > 0 {
		t.Fatalf("两面路径差集非空：只在能力侧 %v · 只在 OpenAPI 侧 %v", onlyWant, onlyGot)
	}
	// ③ 每条能力条目的 endpoint 写法 = 「方法 路径」（人面/机器面同一个串）
	for _, c := range caps {
		ep, _ := c["endpoint"].(string)
		name, _ := c["name"].(string)
		found := false
		for _, e := range routeTable {
			if ep == e.Method+" "+e.Path && name == e.Name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("能力条目 %q 的 endpoint %q 与真源表不符", name, ep)
		}
	}
	t.Logf("能力 %d 条 · OpenAPI 路径 %d 条 · 差集空（两条能力共享同一端点 /api/resources/{type} 是设计如此：ledger 就是那条路由）",
		len(caps), len(paths))
}

// TestSelfDescribingFacesAreListedByThemselves —— 自描述三面必须**列出自己**（差集 2 条的病根）。
func TestSelfDescribingFacesAreListedByThemselves(t *testing.T) {
	self := []string{"/api/capabilities", "/api/openapi.json", "/api/help"}
	inCaps := map[string]bool{}
	for _, e := range routeTable {
		inCaps[e.Path] = true
	}
	paths := openapiPaths()
	for _, p := range self {
		if !inCaps[p] {
			t.Errorf("自描述端点 %s 不在能力清单里（清单不列自己 ⇒ 外部 agent 顺着清单走会断在这里）", p)
		}
		if _, ok := paths[p]; !ok {
			t.Errorf("自描述端点 %s 不在 OpenAPI paths 里", p)
		}
	}
}

// TestCommandFaceProjectionsCovered —— 命令面投影的那几条远端端点必须在表里。
// （`zerg api ls` / `zerg api openapi` / `zerg agent ls` / `zerg task ls` / `zerg model ls`）
// 出处：批 A · T-08 的对账面判据①（当时 3/5 —— 缺的两条正是两个自描述面自身）。
func TestCommandFaceProjectionsCovered(t *testing.T) {
	want := []string{"/api/capabilities", "/api/openapi.json", "/api/fleet/models", "/api/fleet/status", "/api/tasks"}
	inCaps := map[string]bool{}
	for _, e := range routeTable {
		inCaps[e.Path] = true
	}
	miss := []string{}
	for _, p := range want {
		if !inCaps[p] {
			miss = append(miss, p)
		}
	}
	if len(miss) > 0 {
		t.Fatalf("命令面投影的端点有 %d 条不在能力清单里：%v", len(miss), miss)
	}
	t.Logf("命令面投影 5 条逐条命中（5/5）")
}

// TestNoDuplicateNamesOrEndpoints —— 能力名不许重；端点 (方法,路径) 允许**多条视图共用**（打印可见）。
//
// 为什么共用是允许的（而不是判红）：模板路由 `/api/resources/{type}` 一条就服务了三个视图
// （`list_resources` 全量 · `resource_fit` 装不装得下 · `resource_residency` 驻留），
// 硬要它们各占一条路径就只能在文档里造幻影端点（本表的第一条边界正好禁止那件事）。
// 但**能力名**必须唯一 —— 机器面按名索引，重名就是两义。
func TestNoDuplicateNamesOrEndpoints(t *testing.T) {
	names := map[string]int{}
	for _, e := range routeTable {
		names[e.Name]++
	}
	for n, c := range names {
		if c > 1 {
			t.Errorf("能力名重复 %d 次：%s", c, n)
		}
	}
	shared := []string{}
	for _, p := range routePaths() {
		ns := []string{}
		for _, e := range routeTable {
			if e.Path == p {
				ns = append(ns, e.Name)
			}
		}
		if len(ns) > 1 {
			shared = append(shared, p+" ← "+strings.Join(ns, ","))
		}
	}
	sort.Strings(shared)
	if len(shared) > 0 {
		t.Logf("被多条能力共用的端点（模板路由的多个视图，不判红）：%v", shared)
	}
}

// TestOpenAPIDetailHasNoPhantomPath —— 详表不许长出真源表没有的路径。
func TestOpenAPIDetailHasNoPhantomPath(t *testing.T) {
	inTable := map[string]bool{}
	for _, p := range routePaths() {
		inTable[p] = true
	}
	for p := range openapiPathDetail {
		if !inTable[p] {
			t.Errorf("OpenAPI 详表里的路径 %s 不在真源表里（幻影端点）", p)
		}
	}
}
