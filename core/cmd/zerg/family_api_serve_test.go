// family_api_serve_test.go —— `zerg api serve`（外部接入 REST API 归入命令面 · 批 B4）的判据。
//
// 落点 = `package main`（**内部**）：常驻档的一条 argv **跑不返回**（矩阵一格 = 一条 argv = 一跑），
// 所以「起 → 停」那一趟走实现的同一入口 `apiServeRun`（`cmdAPIServe` 只多一层信号 ctx）；
// HTTP 面走同一条实现造出来的 mux（`httptest` 直打 —— 不真占端口、不进真网络）；
// 用法面 / 命令树面走**外部**入口 `run`。
//
// 全部合成盘面（`t.TempDir()` 当工作目录）：不碰真 `~/.zerg/state`、不起 `zerg-api` 二进制、
// 不派任何真任务（`POST /task` 只用**校验必败**的载荷 —— `TaskSpec.Validate` 在第一道就退，
// 落到 `statepath.IssuesDir()` 之前 ⇒ 真任务单目录一个字节都不动）。
//
// 本件钉住的四格（任务单 §三 B4 的「判据件至少」）：
//
//	① **登记面**：命令树里真有 `api serve`（旗标逐字三枚 = 源件 `-port`/`-workdir`/`-insecure` ·
//	   `arity=none`），且**既有三条 `api` 动作一条不少**（`ls`/`openapi`/`help` —— 归入是只增一格）；
//	② **正常路径**：六条路由逐条打（`/capabilities` · `/status` · `/task/{id}` · `/` · `/panel`
//	   + 缺单 404 · 坏 JSON 400 · 坏 spec 400），Bearer 鉴权两态（有令牌 ⇒ 401 / 给对了 ⇒ 200），
//	   以及「起 → 停」一整趟退 0（启动行逐字）；
//	③ **用法面**：未知旗标 / 位置参数 / `--port` 裸给 / `--port` 非整数 / `--port` 越界 /
//	   `--workdir` 裸给 / 不认的契约主号 ⇒ 一律 2 + stdout 零字节；且这三枚旗标**不泄漏**给别族；
//	   缺令牌拒启 ⇒ 1（源件 `log.Fatalf` 同档）、`--insecure` ⇒ 横幅 + 放行；
//	④ **只读面**：跑前/跑后工作目录**逐件 sha256 不变**（`GET /status` `/task/{id}` 只读任务单目录）。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// apiServeInvoke —— 走**外部**入口跑一次（与 `package main_test` 的 `RunForTest` 同一件）。
func apiServeInvoke(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := run(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// apiServeMustInvoke —— 造一枚已解析的 invocation（配置档直取，便于单测解析面）。
func apiServeMustInvoke(t *testing.T, argv ...string) *invocation {
	t.Helper()
	inv, err := parseInvocation(append([]string{"api"}, argv...))
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	return inv
}

// apiServeDo —— 打一发 httptest 请求（返回状态码 + 正文）。
func apiServeDo(t *testing.T, h http.Handler, method, path, body string, hdr map[string]string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// ① 登记面：path / 旗标逐字 / arity / 既有三条动作仍在。
func TestApiServeCommandRegistered(t *testing.T) {
	info, ok := CommandInfoOfForTest("api serve")
	if !ok {
		t.Fatalf("命令树里没有 `zerg api serve`（批 B4 的核心那一条）")
	}
	for _, f := range []string{"--port", "--workdir", "--insecure"} {
		if !strings.Contains(info.Usage, f) {
			t.Errorf("用法串漏了旗标 %s（源件三枚必须逐字保留）：%q", f, info.Usage)
		}
	}
	if !strings.HasPrefix(info.Usage, "zerg api serve") {
		t.Errorf("用法串没以 `zerg api serve` 开头：%q", info.Usage)
	}
	if info.Passthrough {
		t.Errorf("`api serve` 不该是透传型（它自己判旗标语义）")
	}
	if !info.OpenForRun {
		t.Errorf("`api serve` 要标成**已开放**（真跑起进程 —— 不是占位动作）")
	}

	// arity：不收位置参数（同族 `api ls/openapi/help` 都是 none）。
	var arity string
	for _, c := range commands {
		if joinPath(c.path) == "api serve" {
			arity = c.arity
		}
	}
	if arity != arityNone {
		t.Errorf("`api serve` 的 arity = %q（要 %q —— 位置参数一律拒）", arity, arityNone)
	}

	// 既有三条只读投影**一条不少**（归入是「只增一格」，✗ 不许挤掉客户端面那三条）。
	for _, p := range []string{"api ls", "api openapi", "api help"} {
		if _, ok := CommandInfoOfForTest(p); !ok {
			t.Errorf("`zerg %s` 不见了 —— 本批只许新增 `api serve`，不许动客户端面三条", p)
		}
	}
}

// ② 正常路径：六条路由 + 鉴权两态 + 起停一趟。
func TestApiServeNormalPath(t *testing.T) {
	t.Setenv("ZERG_API_TOKEN", "")
	wd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wd, "docs", "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wd, "docs", "issues", "20260814-123456-abcd.md"),
		[]byte("# 任务\n- **任务**: 优化某函数\n- **状态**: queued\n- **失败类型**: \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := apiServeCfg{port: 0, workDir: wd}
	h := apiServeMux(cfg)

	// `/capabilities`：机器可读清单逐键在（名字与源件同 —— 发现面靠它）。
	if code, body := apiServeDo(t, h, "GET", "/capabilities", "", nil); code != 200 ||
		!strings.Contains(body, `"name":"zerg-api"`) || !strings.Contains(body, `"/task"`) {
		t.Errorf("GET /capabilities：code=%d body=%q（要 200 + 源件那份清单）", code, body)
	}

	// `/status`：任务统计读到合成任务单（状态分布 + 任务列表）。
	code, body := apiServeDo(t, h, "GET", "/status", "", nil)
	if code != 200 {
		t.Fatalf("GET /status：code=%d（要 200）", code)
	}
	var st struct {
		Tasks  int                 `json:"tasks"`
		Status map[string]int      `json:"status"`
		List   []map[string]string `json:"task_list"`
	}
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		t.Fatalf("GET /status 的正文不是 JSON：%v body=%q", err, body)
	}
	if st.Tasks != 1 || st.Status["queued"] != 1 || len(st.List) != 1 {
		t.Errorf("GET /status 读数不对：%+v（合成盘面是 1 件 queued）", st)
	}

	// `/task/{id}`：读得到 + 读不到 404（源件同档）。
	if code, body := apiServeDo(t, h, "GET", "/task/20260814-123456-abcd", "", nil); code != 200 ||
		!strings.Contains(body, "优化某函数") {
		t.Errorf("GET /task/{id}：code=%d body=%q", code, body)
	}
	if code, _ := apiServeDo(t, h, "GET", "/task/没有这一单", "", nil); code != 404 {
		t.Errorf("GET /task/<缺单>：code=%d（要 404 —— 源件同档）", code)
	}

	// `/` 与 `/panel`：人面说明 + 面板 HTML（判词逐字）。
	if code, body := apiServeDo(t, h, "GET", "/", "", nil); code != 200 || !strings.Contains(body, "虫族任务执行体 API") {
		t.Errorf("GET /：code=%d body=%q", code, body)
	}
	if code, body := apiServeDo(t, h, "GET", "/panel", "", nil); code != 200 || !strings.Contains(body, "虫族任务执行体监控") {
		t.Errorf("GET /panel：code=%d body=%q", code, body)
	}

	// `POST /task`：坏 JSON ⇒ 400；坏 spec（类型不在闭集）⇒ 400 ⇒ **真任务单目录一个字节不动**。
	if code, body := apiServeDo(t, h, "POST", "/task", "{不是 JSON", nil); code != 400 ||
		!strings.Contains(body, "invalid JSON") {
		t.Errorf("POST /task 坏 JSON：code=%d body=%q（要 400 + 源件那句）", code, body)
	}
	if code, body := apiServeDo(t, h, "POST", "/task",
		`{"task":"x","type":"bogus","priority":"normal","source":"api"}`, nil); code != 400 {
		t.Errorf("POST /task 坏 spec：code=%d body=%q（要 400 —— Validate 第一道就退）", code, body)
	}

	// 鉴权两态：有令牌 ⇒ 没带/带错 401 · 带对 200（源件 `authOK` 逐字）。
	authCfg := apiServeCfg{port: 0, workDir: wd, token: "tok-123"}
	ha := apiServeMux(authCfg)
	if code, body := apiServeDo(t, ha, "GET", "/capabilities", "", nil); code != 401 ||
		!strings.Contains(body, "unauthorized") {
		t.Errorf("有令牌但没带：code=%d body=%q（要 401 + 源件那句）", code, body)
	}
	if code, _ := apiServeDo(t, ha, "GET", "/capabilities", "", map[string]string{"Authorization": "Bearer 错"}); code != 401 {
		t.Errorf("有令牌但带错：code=%d（要 401）", code)
	}
	if code, _ := apiServeDo(t, ha, "GET", "/capabilities", "", map[string]string{"Authorization": "Bearer tok-123"}); code != 200 {
		t.Errorf("有令牌且带对：code=%d（要 200）", code)
	}

	// 起 → 停一整趟：ctx 已结束 ⇒ 起监听后立刻优雅停，退 0（同一实现，只是 ctx 不同源）。
	// 无令牌档必须显式 `--insecure`（源件 P-007）；横幅与 `（无鉴权）` 注记同批判。
	var errb bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	noTokCfg := cfg
	noTokCfg.insecure = true
	if rc := apiServeRun(ctx, noTokCfg, &errb); rc != exitOK {
		t.Fatalf("常驻档起停一趟 rc=%d（要 0）· stderr=\n%s", rc, errb.String())
	}
	got := errb.String()
	for _, want := range []string{
		"⚠️  横幅：zerg-api 以 --insecure 启动",
		"🚀 zerg-api started :0 (workdir=" + wd,
		"（无鉴权）",
		"📡 stop signal received, shutting down gracefully...",
		"🛑 api server stopped",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("起停面缺 %q\n—— 实得：\n%s", want, got)
		}
	}
	// 有令牌 ⇒ 启动行末尾那句注记是 Bearer 档。
	var tb bytes.Buffer
	tctx, tcancel := context.WithCancel(context.Background())
	tcancel()
	if rc := apiServeRun(tctx, authCfg, &tb); rc != exitOK {
		t.Fatalf("带令牌起停 rc=%d（要 0）", rc)
	}
	if !strings.Contains(tb.String(), "（Bearer 鉴权）") {
		t.Errorf("带令牌的启动行没写 `（Bearer 鉴权）`：\n%s", tb.String())
	}
}

// ③ 用法面：七格必须 2 + stdout 零字节；缺令牌拒启 1 / `--insecure` 放行；旗标不泄漏。
func TestApiServeUsageFace(t *testing.T) {
	wd := t.TempDir()
	cases := []struct {
		name string
		argv []string
		want string // stderr 必含
	}{
		{"未知旗标", []string{"api", "serve", "--nosuchflag-zz"}, "未知旗标"},
		{"多余位置参数", []string{"api", "serve", "extra"}, "不收位置参数"},
		{"--port 裸给", []string{"api", "serve", "--port"}, "--port"},
		{"--port 非整数", []string{"api", "serve", "--port", "abc"}, "0..65535"},
		{"--port 越界", []string{"api", "serve", "--port", "70000"}, "0..65535"},
		{"--workdir 裸给", []string{"api", "serve", "--workdir"}, "--workdir"},
		{"不认的契约主号", []string{"api", "serve", "--schema", "zerg/v9"}, "契约主号"},
	}
	for _, c := range cases {
		rc, out, errb := apiServeInvoke(t, c.argv...)
		if rc != exitUsage {
			t.Errorf("%s：rc=%d（要 %d · 用法错）", c.name, rc, exitUsage)
		}
		if out != "" {
			t.Errorf("%s：stdout 要零字节（用法类错误不吐包封 · K2 甲档）：%q", c.name, out)
		}
		if !strings.Contains(errb, c.want) {
			t.Errorf("%s：stderr 没说到 %q：%q", c.name, c.want, errb)
		}
	}

	// 解析面：缺省值逐字（端口 8083 · 工作目录 = 当前目录）。
	t.Setenv("ZERG_API_TOKEN", "t")
	inv := apiServeMustInvoke(t, "serve", "--workdir", wd)
	cfg, rc := apiServeParse(inv, &bytes.Buffer{})
	if rc != exitOK {
		t.Fatalf("`api serve --workdir <dir>` 的配置档 rc=%d（要 0）", rc)
	}
	if cfg.port != apiServeDefaultPort {
		t.Errorf("`--port` 缺省 = %d（要源件的 %d）", cfg.port, apiServeDefaultPort)
	}
	if cfg.workDir != wd || !filepath.IsAbs(cfg.workDir) {
		t.Errorf("工作目录 = %q（要绝对路径 %q）", cfg.workDir, wd)
	}
	if cfg.insecure {
		t.Errorf("没给 `--insecure` 时 it 该是 false：%+v", cfg)
	}
	// `--port 0` 收（= 源件 `:0` · 系统分配）；`--insecure` 收。
	inv = apiServeMustInvoke(t, "serve", "--workdir", wd, "--port", "0", "--insecure")
	if cfg, rc = apiServeParse(inv, &bytes.Buffer{}); rc != exitOK || !cfg.insecure || cfg.port != 0 {
		t.Errorf("`--port 0 --insecure` 解析不对：rc=%d cfg=%+v", rc, cfg)
	}

	// 缺令牌 ⇒ **拒启**（源件 `log.Fatalf` 同档 · 退 1），且判词逐字含 §十二 `P-007`。
	t.Setenv("ZERG_API_TOKEN", "")
	var errb bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rc := apiServeRun(ctx, apiServeCfg{port: 0, workDir: wd}, &errb); rc != exitFail {
		t.Errorf("缺令牌拒启：rc=%d（要 %d）", rc, exitFail)
	}
	if !strings.Contains(errb.String(), "ZERG_API_TOKEN 未设置 ⇒ 拒绝启动") ||
		!strings.Contains(errb.String(), "--insecure") {
		t.Errorf("缺令牌判词不对：%q", errb.String())
	}
	// `--insecure` ⇒ 放行并**打横幅**（不许静默裸奔）。
	var ib bytes.Buffer
	ictx, icancel := context.WithCancel(context.Background())
	icancel()
	if rc := apiServeRun(ictx, apiServeCfg{port: 0, workDir: wd, insecure: true}, &ib); rc != exitOK {
		t.Errorf("`--insecure` 放行：rc=%d（要 0）· stderr=%q", rc, ib.String())
	}
	if !strings.Contains(ib.String(), "横幅") || !strings.Contains(ib.String(), "无鉴权") {
		t.Errorf("`--insecure` 没打横幅（§十二 P-007 要求显式告警）：%q", ib.String())
	}

	// **不泄漏**：这三枚旗标在别族的命令上一律仍 2（数据驱动收口那一排）。
	for _, argv := range [][]string{
		{"trace", "--insecure"},
		{"api", "ls", "--port", "9"},
		{"api", "help", "--insecure"},
		{"sched", "show", "--port", "9"},
	} {
		rc, out, _ := apiServeInvoke(t, argv...)
		if rc != exitUsage || out != "" {
			t.Errorf("%v：rc=%d out=%q（别族给新旗标要仍 2 且 stdout 零字节）", argv, rc, out)
		}
	}
}

// ④ 只读面：起停一趟 + 六个只读端点打完，工作目录逐件 sha256 不变。
func TestApiServeReadOnly(t *testing.T) {
	t.Setenv("ZERG_API_TOKEN", "")
	wd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wd, "docs", "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wd, "docs", "issues", "T-9.md"),
		[]byte("# 任务九\n- **任务**: 读面\n- **状态**: running\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := schedRunTreeHash(t, wd)

	cfg := apiServeCfg{port: 0, workDir: wd}
	h := apiServeMux(cfg)
	for _, p := range []string{"/capabilities", "/status", "/task/T-9", "/", "/panel"} {
		if code, _ := apiServeDo(t, h, "GET", p, "", nil); code != 200 {
			t.Errorf("GET %s 没回 200", p)
		}
	}
	// 校验必败的 POST 也不许落盘（Validate 在 statepath 之前就退）。
	if code, _ := apiServeDo(t, h, "POST", "/task", `{"task":"","type":"code","priority":"normal","source":"api"}`, nil); code != 400 {
		t.Errorf("空任务 POST 该 400（现 %d）", code)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var errb bytes.Buffer
	roCfg := cfg
	roCfg.insecure = true // 无令牌档（P-007）：显式放行才起得来
	if rc := apiServeRun(ctx, roCfg, &errb); rc != exitOK {
		t.Fatalf("起停一趟 rc=%d（要 0）· stderr=\n%s", rc, errb.String())
	}
	if after := schedRunTreeHash(t, wd); after != before {
		t.Errorf("`zerg api serve` 动了工作目录（只读端点只该读任务单）：\n改前：\n%s\n改后：\n%s", before, after)
	}
}
