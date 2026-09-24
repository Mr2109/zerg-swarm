package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/routepin"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// route_pin_test.go —— 「显式指定机器」那道门的判据件（**全离线**：不碰真机群、不写 ~/.zerg、
// 不发网络请求；每一条都自带隔离 —— `ZERG_ROUTE_PINS` 指到 `t.TempDir()`）。
//
// 它判的是设计稿（`Zerg-内部文档/项目文档/v2.5.12/设计-指定机器路由-v1.0-20260924.md`）四条约束的
// **可验证形式**，逐条对号：
//
//	① 优先级写死：显式指定 > 默认路由 —— `TestRoutePin_TableHit` / `..._HeaderHit` / `..._HeaderBeatsTable`
//	② 不永久改变默认（一次性 + TTL）—— `..._Expired` / `..._HeaderBeatsTable` / `..._BadExpiryIsNotHit`
//	③ 零落盘的一次性路径 —— `..._HeaderNeverTouchesDisk`（盘面 sha256+尺寸+mtime 逐字不变）
//	④ 未命中 ⇒ 与今天逐字一致 —— 见 `route_parity_test.go`（改前/改后同一批输入的金标准对拍）
//
// 两条**负控**（「模型写不到覆盖表」这一条的正身）：
//
//	· `..._RequestCannotPersist`：请求侧**任何**写法都写不进覆盖表（含「写意图头」）；
//	· `..._GatewayHasNoWritePath`：**源码级**断言 —— 那道门那一件里 0 个写盘调用。

// pinTestConfig —— 一份最小配置：`dual` = 本机 + x3 两个候选（现场那条正是这种形状），
// `only-remote` = 只有 x3 一个候选（用来测「点名一台不是候选的机器」的负控）。
// 别名一条：钉的是**标准名**、请求可以用别名。
func pinTestConfig() *config.FleetConfig {
	return &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"dual": {
				{Host: "Mr2109", Backend: "llama-server", File: "/m/dual.gguf", MemGb: 21},
				{Host: "x3", Backend: "llama-server", File: "/m/dual.gguf", MemGb: 21},
			},
			"only-remote": {
				{Host: "x3", Backend: "llama-server", File: "/m/only.gguf", MemGb: 5},
			},
		},
		Fleet: map[string]config.FleetNode{
			"Mr2109": {Host: "127.0.0.1", Port: 8100},
			"x3":   {Host: "<worker-ip>", Port: 8100},
		},
		Aliases: map[string]string{"双候选": "dual"},
	}
}

// pinTestGateway —— 造一个只带路由面的 Gateway（与 `pickroute_test.go` 同一种构造法：直接字面量，
// 不经 `NewGateway` ⇒ 不起网关、不读规则表、不碰任何外部件）。
func pinTestGateway(t *testing.T, pins ...routepin.Pin) (*Gateway, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "route_pins.json")
	t.Setenv(routepin.EnvPath, path)
	if len(pins) > 0 {
		tbl := &routepin.Table{ID: routepin.TableID}
		for _, p := range pins {
			tbl.Upsert(p)
		}
		if err := tbl.Save(path); err != nil {
			t.Fatalf("夹具：覆盖表写不进去：%v", err)
		}
	}
	g := &Gateway{
		config:     pinTestConfig(),
		roundRobin: map[string]int{},
		failCounts: map[string]int{},
		failSince:  map[string]time.Time{},
		store:      &store.Store{},
	}
	return g, path
}

// pinRow —— 一条钉（`model` 缺省 = `dual`）。
func pinRow(model, machine string, ttl time.Duration) routepin.Pin {
	now := time.Now()
	if model == "" {
		model = "dual"
	}
	return routepin.Pin{
		Model:      model,
		Machine:    machine,
		CreatedAt:  now.Format(time.RFC3339),
		ExpiresAt:  now.Add(ttl).Format(time.RFC3339),
		TTLSeconds: int64(ttl / time.Second),
		By:         "判据件",
	}
}

// defaultHostOf —— 「今天的默认择优」会挑哪台（供「回默认」那几条断言用**同一个真源**）。
func defaultHostOf(t *testing.T, g *Gateway, model string) string {
	t.Helper()
	r, err := g.pickRoute(model, "", "")
	if err != nil {
		t.Fatalf("默认择优本该选出一台：%v", err)
	}
	return r.Host
}

// defaultHostFresh —— 同上，但**另起一枚干净 Gateway**（临时目录随之另开 —— 与「已钉住的
// 那一枚」互不污染，专供「钉住/过期/撒掉之后是不是回到了默认那台」的对照）。
func defaultHostFresh(t *testing.T, model string) string {
	t.Helper()
	g, _ := pinTestGateway(t)
	return defaultHostOf(t, g, model)
}

// ── ① 优先级：显式指定 > 默认路由 ────────────────────────────────────────────────

// TestRoutePin_TableHit —— 覆盖表命中 ⇒ **命中即用**（落点名那台；URL/端口按 fleet 真源组装）。
func TestRoutePin_TableHit(t *testing.T) {
	g, _ := pinTestGateway(t, pinRow("", "x3", 2*time.Hour))
	r, err := g.pickRoute("dual", "", "")
	if err != nil {
		t.Fatalf("钉住后本该直接落 x3：%v", err)
	}
	if r.Host != "x3" {
		t.Fatalf("覆盖表命中没生效：期望 x3，实得 %s", r.Host)
	}
	if r.URL != "http://<worker-ip>:8100/infer" || r.Port != 8100 {
		t.Fatalf("URL/端口该按 fleet 真源组装，实得 %q / %d", r.URL, r.Port)
	}
}

// TestRoutePin_TableHit_OnLocal —— 反向也要成立（钉回本机那台 ⇒ 就落本机）：
// 「命中即用」不许被择优的方向绑住（否则「钉住」会变成「钉到远程」的同义词）。
func TestRoutePin_TableHit_OnLocal(t *testing.T) {
	g, _ := pinTestGateway(t, pinRow("", "Mr2109", 2*time.Hour))
	r, err := g.pickRoute("dual", "", "")
	if err != nil {
		t.Fatalf("钉住后本该直接落 Mr2109：%v", err)
	}
	if r.Host != "Mr2109" {
		t.Fatalf("覆盖表命中没生效：期望 Mr2109，实得 %s", r.Host)
	}
}

// TestRoutePin_HeaderHit —— 请求头那一档（零落盘的一次性路径）同样命中即用。
func TestRoutePin_HeaderHit(t *testing.T) {
	g, _ := pinTestGateway(t)
	r, err := g.pickRouteForRequest("dual", "x3", "", "")
	if err != nil {
		t.Fatalf("带头请求本该落 x3：%v", err)
	}
	if r.Host != "x3" {
		t.Fatalf("请求头命中没生效：期望 x3，实得 %s", r.Host)
	}
}

// TestRoutePin_HeaderBeatsTable —— 两档同时给**且指向不同机器** ⇒ **头赢**
// （「本次请求的意图」比「表里的长期意图」更具体 —— 写死的口径，**不靠遍历顺序说话**）。
func TestRoutePin_HeaderBeatsTable(t *testing.T) {
	g, _ := pinTestGateway(t, pinRow("", "Mr2109", 2*time.Hour))
	r, err := g.pickRouteForRequest("dual", "x3", "", "")
	if err != nil {
		t.Fatalf("两档同给本该落头点名那台：%v", err)
	}
	if r.Host != "x3" {
		t.Fatalf("优先级写反了：头点 x3、表钉 Mr2109 ⇒ 该落 x3，实得 %s", r.Host)
	}
}

// TestRoutePin_AliasHit —— 钉的是**标准名**、请求用**别名** ⇒ 照样命中（先解析别名、再查表）。
func TestRoutePin_AliasHit(t *testing.T) {
	g, _ := pinTestGateway(t, pinRow("", "x3", 2*time.Hour))
	r, err := g.pickRoute("双候选", "", "") // 别名 → dual（配置里那一条）
	if err != nil {
		t.Fatalf("别名请求本该命中覆盖表：%v", err)
	}
	if r.Host != "x3" {
		t.Fatalf("别名没走到钉住那台：期望 x3，实得 %s", r.Host)
	}
}

// TestRoutePin_ModelNotInTable —— 请求的模型不在路由表里 ⇒ 与今天同路（报「不在路由表里」）。
func TestRoutePin_ModelNotInTable(t *testing.T) {
	g, _ := pinTestGateway(t, pinRow("", "x3", 2*time.Hour))
	_, err := g.pickRoute("压根没有这个模型", "", "")
	if err == nil || !strings.Contains(err.Error(), "not found in the routing table") {
		t.Fatalf("期望「不在路由表里」那条原有报错，实得：%v", err)
	}
}

// TestRoutePin_NotACandidateIsFailClosed —— 点名一台**不是该模型候选**的机器 ⇒ fail-closed
// （**绝不悄悄回默认择优** —— 那正好是「钉住等于没钉」的静默降级）。头/表两条路各判一次。
func TestRoutePin_NotACandidateIsFailClosed(t *testing.T) {
	g, _ := pinTestGateway(t)
	_, err := g.pickRouteForRequest("only-remote", "Mr2109", "", "")
	if err == nil {
		t.Fatal("头点名 Mr2109、而 only-remote 在 Mr2109 上没有候选 ⇒ 本该 fail-closed，却放行了")
	}
	if !strings.Contains(err.Error(), "没有候选") || !strings.Contains(err.Error(), "不悄悄回默认择优") {
		t.Fatalf("报错文案该点名「没有候选」+「不悄悄回默认择优」，实得：%v", err)
	}

	g2, _ := pinTestGateway(t, pinRow("only-remote", "Mr2109", 2*time.Hour))
	_, err2 := g2.pickRoute("only-remote", "", "")
	if err2 == nil || !strings.Contains(err2.Error(), "没有候选") {
		t.Fatalf("覆盖表点了一台不是候选的机器 ⇒ 本该 fail-closed，实得：%v", err2)
	}
}

// ── ② TTL：到期自动回默认 ──────────────────────────────────────────────────────

// TestRoutePin_Expired —— `expires_at` 已过 ⇒ **不命中** ⇒ 回默认择优（与不钉时同一台）；
// 且过期行**仍留在盘上**（读者永不删行 —— 「钉过什么、什么时候过期」要看得见）。
func TestRoutePin_Expired(t *testing.T) {
	now := time.Now()
	expired := routepin.Pin{
		Model: "dual", Machine: "x3",
		CreatedAt:  now.Add(-3 * time.Hour).Format(time.RFC3339),
		ExpiresAt:  now.Add(-1 * time.Minute).Format(time.RFC3339), // 已过期 1 分钟
		TTLSeconds: 7200,
	}
	g, path := pinTestGateway(t, expired)
	if tbl, err := routepin.Load(path); err != nil || len(tbl.Pins) != 1 {
		t.Fatalf("读者不该删过期行（盘面要能自证），实得：%+v / %v", tbl, err)
	}
	if h, _ := g.pinnedHostFor("dual"); h != "" {
		t.Fatalf("过期行不许命中，实得 %q", h)
	}
	base := defaultHostFresh(t, "dual")
	r, err := g.pickRoute("dual", "", "")
	if err != nil {
		t.Fatalf("过期后本该回默认择优：%v", err)
	}
	if r.Host != base {
		t.Fatalf("过期后没回默认择优：期望 %s，实得 %s", base, r.Host)
	}
}

// TestRoutePin_BadExpiryIsNotHit —— `expires_at` 读不出来（坏形状）⇒ **不命中**：
// 宁可不钉，也不许把「时刻读不出来」当「永不过期」（后者正好是「永久改变默认」那条路）。
func TestRoutePin_BadExpiryIsNotHit(t *testing.T) {
	g, _ := pinTestGateway(t, routepin.Pin{Model: "dual", Machine: "x3", ExpiresAt: "不是时刻"})
	if h, _ := g.pinnedHostFor("dual"); h != "" {
		t.Fatalf("坏时刻的行不许命中，实得 %q", h)
	}
}

// TestRoutePin_BrokenTableIsMiss —— 覆盖表整件坏掉（不是合法 JSON）⇒ **不命中**（回默认），
// 且**不把整条路打死**：请求照走默认择优（读者侧 fail-soft，写者侧才 fail-closed）。
func TestRoutePin_BrokenTableIsMiss(t *testing.T) {
	g, path := pinTestGateway(t)
	if err := os.WriteFile(path, []byte("{ 这不是 JSON"), 0o644); err != nil {
		t.Fatal(err)
	}
	if h, _ := g.pinnedHostFor("dual"); h != "" {
		t.Fatalf("坏表不许命中，实得 %q", h)
	}
	if _, err := g.pickRoute("dual", "", ""); err != nil {
		t.Fatalf("坏表时本该照走默认择优：%v", err)
	}
}

// TestRoutePin_HotReload —— 钉/撒**不必重启主控**：同一枚 Gateway，盘面一变 ⇒ 下一次请求即变。
func TestRoutePin_HotReload(t *testing.T) {
	g, path := pinTestGateway(t)
	base := defaultHostOf(t, g, "dual")
	tbl := &routepin.Table{ID: routepin.TableID}
	tbl.Upsert(pinRow("", "x3", time.Hour))
	if err := tbl.Save(path); err != nil {
		t.Fatal(err)
	}
	touchAhead(t, path, 2*time.Second) // mtime 粒度兜底：明确推到「刚才之后」
	if h, _ := g.pinnedHostFor("dual"); h != "x3" {
		t.Fatalf("钉下去之后该落 x3（热读没生效）：实得 %q", h)
	}
	if err := (&routepin.Table{ID: routepin.TableID}).Save(path); err != nil { // 撒
		t.Fatal(err)
	}
	touchAhead(t, path, 4*time.Second)
	if h, _ := g.pinnedHostFor("dual"); h != "" {
		t.Fatalf("撒掉之后不该还命中：实得 %q", h)
	}
	if got := defaultHostOf(t, g, "dual"); got != base {
		t.Fatalf("撒掉之后该回默认择优 %s，实得 %s", base, got)
	}
}

// ── ③ 零落盘 + 负控：模型（请求面）写不到覆盖表 ────────────────────────────────

// TestRoutePin_HeaderNeverTouchesDisk —— 带头跑一批请求（多种写法）⇒ 覆盖表
// **sha256 + 尺寸 + mtime 三者逐字不变**（「零落盘」那条约束的可验证形式）。
func TestRoutePin_HeaderNeverTouchesDisk(t *testing.T) {
	g, path := pinTestGateway(t, pinRow("", "Mr2109", time.Hour))
	before := fileDigest(t, path)
	for _, hdr := range []string{"x3", "Mr2109", "不存在的机器", "", "x3 "} {
		_, _ = g.pickRouteForRequest("dual", hdr, "", "")
	}
	if after := fileDigest(t, path); after != before {
		t.Fatalf("请求面动了覆盖表：写前 %s ≠ 写后 %s", before, after)
	}
}

// TestRoutePin_RequestCannotPersist —— **负控正身**：请求侧想「写意图」也写不进表 ——
// 覆盖表既没有那一行，盘面也逐字未变（写口只有一个：命令面 `zerg route pin|unpin`）。
func TestRoutePin_RequestCannotPersist(t *testing.T) {
	g, path := pinTestGateway(t)
	before := fileDigest(t, path)
	// 请求侧「写意图」的三种写法（真实现里**一个都不认**）：写意图头 / 头 + TTL / 另一枚头名。
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"dual"}`))
	req.Header.Set(machineHeaderName, "x3")
	req.Header.Set(machineHeaderName+"-TTL", "2h")
	req.Header.Set("X-Zerg-Route-Pin", "2h")
	if h := machineHeaderOf(req); h != "x3" {
		t.Fatalf("头解析该逐字取 x3，实得 %q", h)
	}
	if _, err := g.pickRouteForRequest("dual", machineHeaderOf(req), "", ""); err != nil {
		t.Fatalf("带头请求本该落 x3：%v", err)
	}
	if after := fileDigest(t, path); after != before {
		t.Fatalf("请求面写进了覆盖表（负控失效）：写前 %s ≠ 写后 %s", before, after)
	}
	if tbl, err := routepin.Load(path); err != nil || len(tbl.Pins) != 0 {
		t.Fatalf("覆盖表里不许出现请求侧写下的行：%+v / %v", tbl, err)
	}
	// 一次性：下一次**不带**头的请求不许还落 x3（头不是钉）。
	if h, _ := g.pinnedHostFor("dual"); h != "" {
		t.Fatalf("头不许变成持久钉：实得 %q", h)
	}
	base := defaultHostFresh(t, "dual")
	r, err := g.pickRouteForRequest("dual", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Host != base {
		t.Fatalf("不带头的下一次请求该回默认择优 %s，实得 %s", base, r.Host)
	}
}

// TestRoutePin_GatewayHasNoWritePath —— **源码级**断言：那道门那一件里 0 个写盘调用。
//
// 为什么用源码级：行为级只能证「这一批请求没写」，源码级证的是「**没有写这条路**」——
// 后者才是「模型写不到覆盖表」的正身（写口只有命令面那一侧）。
//
// ★ 判据只看**代码**（先剥注释）：本件头部那几行散文里逐字写着「0 个 `os.WriteFile` …」
// —— 不剥注释就会拿说明文字当真隐患（本仓「检测与就地修同口径跳过整行注释」那条纪律）。
func TestRoutePin_GatewayHasNoWritePath(t *testing.T) {
	src, err := os.ReadFile("route_pin.go")
	if err != nil {
		t.Fatalf("读不到 route_pin.go（判据件必须能自读）：%v", err)
	}
	var code []string
	inBlock := false
	for _, ln := range strings.Split(string(src), "\n") {
		s := strings.TrimSpace(ln)
		if inBlock {
			if strings.Contains(s, "*/") {
				inBlock = false
			}
			continue
		}
		if strings.HasPrefix(s, "/*") {
			if !strings.Contains(s, "*/") {
				inBlock = true
			}
			continue
		}
		if strings.HasPrefix(s, "//") {
			continue // 整行注释：说明文字，不当隐患
		}
		if i := strings.Index(s, "//"); i >= 0 {
			s = s[:i] // 行末注释同口径剥掉
		}
		code = append(code, s)
	}
	text := strings.Join(code, "\n")
	for _, w := range []string{
		"os.WriteFile", "os.Create", "os.OpenFile", "os.Rename", "os.Remove",
		"os.MkdirAll", "ioutil.WriteFile", ".Save(", "os.Truncate", "os.Chmod",
	} {
		if strings.Contains(text, w) {
			t.Fatalf("那道门那一件里出现了写盘调用 %q —— 「网关从不写覆盖表」这条被破了", w)
		}
	}
}

// ── 夹具 ──────────────────────────────────────────────────────────────────────

// fileDigest —— 「盘面逐字不变」的取法（sha256 + 尺寸 + mtime 三者一起）。
func fileDigest(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "absent"
		}
		t.Fatalf("读不到 %s：%v", path, err)
	}
	sum := sha256.Sum256(b)
	mt := int64(0)
	if st, err := os.Stat(path); err == nil {
		mt = st.ModTime().UnixNano()
	}
	return fmt.Sprintf("%s|%d|%d", hex.EncodeToString(sum[:]), len(b), mt)
}

// touchAhead —— 把件的 mtime 明确推到一个「刚才之后」的时刻（同秒内两次写盘时，
// mtime 可能相同 ⇒ 热读会判「没变」。夹具侧推时间，**不改实现**）。
func touchAhead(t *testing.T, path string, d time.Duration) {
	t.Helper()
	ts := time.Now().Add(d)
	if err := os.Chtimes(path, ts, ts); err != nil {
		t.Fatalf("夹具：推 mtime 失败：%v", err)
	}
}
