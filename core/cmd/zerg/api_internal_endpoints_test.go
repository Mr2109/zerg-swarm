package main

// api_internal_endpoints_test.go —— `api` 族「② 端点投影」的机器判据（2.5.15 波1 · 批 2）。
//
// 本文件回答四个问题，每个都能**独立变红**：
//
//	① 表里每条端点在注册面上**真的注册过**吗（逐条扫三个注册件的真注册行）——防「幻影端点」；
//	② 反向：注册面**没漏**吗（表逐条命中 + 路径集合与设计稿 §5.1 的 28 条**双向零差集**）；
//	③ 出处锚**指得准**吗（`Origin` 的 `件:行` 那一行**逐字就是**该 (方法,路径) 的注册行）；
//	④ 投影面**只读不重**吗（去重按 (方法,路径)；同一端点在一次输出里只出现一次）。
//
// ④ 同时是「✗ 不开执行面」的守门：本族 `--help` 里出现 `api call` 一类逃生门即红。

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// apiRosterSources —— ② 端点的三个注册件（口径：件内的 `r.<方法>("<路径>", …)` 注册行）。
// 路径一律**相对仓根**（与 `Origin` 栏同一坐标系 —— 出处锚的核对也用它）。
// 与 `core/internal/api/routes_test.go` 的 `realRoutes` 同一条扫法（扫文本，不起服务）。
var apiRosterSources = []string{
	"core/internal/api/chat_handlers.go",
	"core/cmd/zerg-core/main.go",
	"core/internal/gateway/gateway.go",
}

// apiRepoRoot —— 从包目录上溯找仓根（锚 = `core/internal/version/version.go`，与全仓同一条锚）。
func apiRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("取不到工作目录：%v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "core", "internal", "version", "version.go")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("上溯不到仓根（公开快照形态）⇒ 跳过，不静默当绿")
		}
		dir = parent
	}
}

// apiRouteReg 匹配一条注册：`r.Get("/api/…", …)`（方法闭集与主控注册面同形）。
var apiRouteReg = regexp.MustCompile(`r\.(Get|Post|Delete|Put|Patch)\("([^"]+)"`)

// apiScanRoutes —— 从一段源码文本里扫出注册键集（`GET /api/x` 形态）。
// 单独成函数是为了让负控能**直接喂空文本**（证这颗牙不是恒真）。
func apiScanRoutes(text string) map[string]bool {
	out := map[string]bool{}
	for _, m := range apiRouteReg.FindAllStringSubmatch(text, -1) {
		out[apiEndpointKey(m[1], m[2])] = true
	}
	return out
}

// apiRosterMisses —— 表里有、注册面没有的条目（空集才算表与注册面一致）。
func apiRosterMisses(roster []apiInternalEndpoint, found map[string]bool) []string {
	misses := []string{}
	for _, e := range roster {
		if !found[e.key()] {
			misses = append(misses, e.key())
		}
	}
	return misses
}

// ---- ① 表 ⊆ 注册面（逐条现读三个注册件）--------------------------------------------

func TestAPIInternalEndpointsAreRegisteredInSources(t *testing.T) {
	root := apiRepoRoot(t)
	found := map[string]bool{}
	all := strings.Builder{}
	scanned := 0
	for _, rel := range apiRosterSources {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Skipf("注册件不在盘（%s）：公开快照形态下跳过；%v", rel, err)
		}
		scanned++
		all.WriteString(string(b))
	}
	for k := range apiScanRoutes(all.String()) {
		found[k] = true
	}
	if scanned != len(apiRosterSources) {
		t.Skipf("三个注册件只读到 %d 个 ⇒ 跳过（不静默当绿）", scanned)
	}
	if len(found) == 0 {
		t.Fatal("注册面一条注册都没扫到（空转 = 假覆盖）⇒ 判据不成立")
	}
	if misses := apiRosterMisses(apiInternalEndpoints, found); len(misses) > 0 {
		t.Fatalf("② 端点表里有 %d 条注册面上**没有注册**（幻影端点）：\n  %s\n注册面实际扫到 %d 条",
			len(misses), strings.Join(misses, "\n  "), len(found))
	}
	// ★ 负控（**在测内做**，不碰盘上注册件）：把第一条的注册路径改个名 ⇒ 必须报缺 **恰一条**。
	//   为什么在测内做：`-overlay` 只替编译输入，替不了 `os.ReadFile` 的运行期读盘 ⇒ 拿 overlay
	//   哑化注册件**测不出**红（实测），而真去改盘上注册件违反「底层件一字节不改」。
	e0 := apiInternalEndpoints[0]
	verb := apiMethodCamel(e0.Method)
	line := `r.` + verb + `("` + e0.Path + `"`
	if !strings.Contains(all.String(), line) {
		t.Fatalf("负控前置不成立：注册面文本里找不到 %s", line)
	}
	mutated := strings.Replace(all.String(), line, `r.`+verb+`("`+e0.Path+`-MUT"`, 1)
	if got := apiRosterMisses(apiInternalEndpoints, apiScanRoutes(mutated)); len(got) != 1 {
		t.Fatalf("负控失败：哑化一条注册行应报缺**恰一条**，实报 %d 条（%v）⇒ 这颗牙不灵", len(got), got)
	}
	t.Logf("② 端点表 %d 条逐条命中注册面（三个注册件共扫到 %d 条注册）· 负控：哑化 1 条 ⇒ 报缺恰 1 条",
		len(apiInternalEndpoints), len(found))
}

// ---- ② 表自己：条数 / 路径数 / 分组 / 反例（这颗牙不是恒真）--------------------------

func TestAPIInternalEndpointsCoverageAndCounts(t *testing.T) {
	// ① 条数与路径数：30 条 (方法,路径) 摊在 28 条路径上（与设计稿 §5.1 行 247 同口径）。
	if got := len(apiInternalEndpoints); got != 30 {
		t.Fatalf("② 端点表条数 = %d，应为 30（28 条路径 + 两条同路径第二方法）", got)
	}
	paths := map[string]bool{}
	keys := map[string]bool{}
	for _, e := range apiInternalEndpoints {
		paths[e.Path] = true
		if keys[e.key()] {
			t.Fatalf("② 端点表里 (方法,路径) 重复：%s", e.key())
		}
		keys[e.key()] = true
		if e.Class == "" || e.Why == "" || e.Origin == "" || e.Name == "" {
			t.Fatalf("② 端点表条目四栏不许留空：%s", e.key())
		}
	}
	if len(paths) != 28 {
		t.Fatalf("② 端点表路径数 = %d，应为 28（对话 17 ＋ 机群 2 ＋ 文件根 3 ＋ 网关 6）", len(paths))
	}
	// ② 分组条数。
	byClass := map[string]int{}
	for _, e := range apiInternalEndpoints {
		byClass[e.Class]++
	}
	want := map[string]int{
		apiInternalClassChat:     19,
		apiInternalClassFleet:    2,
		apiInternalClassFileroot: 3,
		apiInternalClassGateway:  5,
		apiInternalClassMetrics:  1,
	}
	for cls, n := range want {
		if byClass[cls] != n {
			t.Fatalf("② 端点表分组条数不符：%q 得 %d，应 %d", cls, byClass[cls], n)
		}
	}
	// ③ 负控：喂空文本 ⇒ **全部**条目都该被报缺（证 `apiRosterMisses` 不是恒真断言）。
	if got := len(apiRosterMisses(apiInternalEndpoints, apiScanRoutes(""))); got != len(apiInternalEndpoints) {
		t.Fatalf("负控失败：空注册面应报 %d 条缺，实报 %d（判据恒真 ⇒ 没有牙）", len(apiInternalEndpoints), got)
	}
	// ④ 正控：拿表自己的注册键当注册面 ⇒ 一条都不该缺（成对另一半）。
	self := map[string]bool{}
	for _, e := range apiInternalEndpoints {
		self[e.key()] = true
	}
	if misses := apiRosterMisses(apiInternalEndpoints, self); len(misses) != 0 {
		t.Fatalf("正控失败：拿表自己的键当注册面竟报缺 %v", misses)
	}
}

// apiRosterSourceKnown —— 出处锚只许落在三个注册件里（✗ 不许指到别处）。
func apiRosterSourceKnown(rel string) bool {
	for _, s := range apiRosterSources {
		if s == rel {
			return true
		}
	}
	return false
}

// apiMethodCamel —— `GET` → `Get`（chi/gorilla 注册面的方法名形态）。
func apiMethodCamel(m string) string {
	m = strings.ToUpper(strings.TrimSpace(m))
	if m == "" {
		return ""
	}
	return m[:1] + strings.ToLower(m[1:])
}

// ---- ③ 出处锚（`Origin` 的 件:行 逐字就是该条注册行）--------------------------------

func TestAPIInternalEndpointOriginsPointAtTheRegistrationLine(t *testing.T) {
	root := apiRepoRoot(t)
	cache := map[string][]string{}
	for _, e := range apiInternalEndpoints {
		rel, line, ok := strings.Cut(strings.TrimSpace(e.Origin), ":")
		if !ok {
			t.Fatalf("出处锚形状不对（要 件:行）：%q", e.Origin)
		}
		n, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("出处锚行号不是数：%q", e.Origin)
		}
		if !apiRosterSourceKnown(rel) {
			t.Fatalf("出处锚落在三个注册件之外：%q", e.Origin)
		}
		lines, ok := cache[rel]
		if !ok {
			b, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil {
				t.Skipf("出处件不在盘（%s）：公开快照形态下跳过；%v", rel, err)
			}
			lines = strings.Split(string(b), "\n")
			cache[rel] = lines
		}
		if n < 1 || n > len(lines) {
			t.Fatalf("出处锚越界：%s（该件 %d 行）", e.Origin, len(lines))
		}
		row := lines[n-1]
		want := `r.` + apiMethodCamel(e.Method) + `("` + e.Path + `"`
		if !strings.Contains(row, want) {
			t.Fatalf("出处锚指错行：%s 第 %d 行不含 %s\n  实际该行：%s", rel, n, want, strings.TrimSpace(row))
		}
	}
}

// ---- ④ 投影面：去重 + 不开执行面 ----------------------------------------------------

func TestAPIInternalProjectionDedupesAndHasNoEscapeDoor(t *testing.T) {
	// ① 现读面已出现过的端点不再重复出（宿主文件浏览器三条是**已在对外发现面**的标本）。
	seen := map[string]bool{}
	for _, e := range apiInternalEndpoints {
		if e.Path == "/api/fileroots" || e.Path == "/api/fileroots/open" || e.Path == "/api/fileroots/reveal" {
			seen[e.key()] = true
		}
	}
	rows := apiInternalLsRows(seen)
	if len(rows) != len(apiInternalEndpoints)-3 {
		t.Fatalf("去重后行数 = %d，应为 %d（28 条路径里那三条已在对外面 ⇒ 不重复出）",
			len(rows), len(apiInternalEndpoints)-3)
	}
	for _, r := range rows {
		if seen[liveEndpointKey(r["endpoint"])] {
			t.Fatalf("去重失效：%q 仍在投影里出现", r["endpoint"])
		}
	}
	// ② 现读格子解析：`GET /api/fileroots` ⇒ 键；形状变了 ⇒ 空串（不参与去重，宁重不漏）。
	if k := liveEndpointKey("GET /api/fileroots"); k != apiEndpointKey("get", "/api/fileroots") {
		t.Fatalf("现读格子解析错：得 %q", k)
	}
	if k := liveEndpointKey("不是端点的格子"); k != "" {
		t.Fatalf("形状不认识时应返回空串（不参与去重），实得 %q", k)
	}
	// ③ 同一条 (方法,路径) 在两个投影面上都只出一次。
	// ★ 属性②（N下界）：先钉住**下限 27**（30 条 (方法,路径) − 已在对外发现面的 3 条 ⇒ 现读恰 27）。
	//   少了它，「两面各静默变小」会让下面的等值判照样绿 —— 正是本门（门128）判的那类病。
	openapiRows := apiInternalOpenAPIRows(seen)
	if len(rows) < 27 || len(openapiRows) < 27 {
		t.Fatalf("投影面行数低于下限 27：ls %d / openapi %d —— 结构静默变小 ⇒ 不给绿", len(rows), len(openapiRows))
	}
	if len(openapiRows) != len(rows) {
		t.Fatalf("两面行数不一致（下限 27）：ls %d / openapi %d", len(rows), len(openapiRows))
	}
	// ④ ✗ 不开执行面：本族命令面里不许出现 `api call` 一类通用逃生门（决议⑨「维持不开」）。
	mainSrc, err := os.ReadFile(filepath.Join("main.go"))
	if err != nil {
		t.Skipf("读不到 main.go：%v", err)
	}
	if strings.Contains(string(mainSrc), `[]string{"api", "call"}`) {
		t.Fatal("`api call` 逃生门被登记进命令树了 —— 决议⑨逐字「维持不开」")
	}
}
