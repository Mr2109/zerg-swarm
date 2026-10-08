// fleet_logs_test.go —— `fleet_logs.go`（`gateway/scripts/fleet_logs.py` 的 Go 真搬）的定向单测。
//
// 覆盖：帮助面 / 用法错面（退码 2 + 逐字文案）/ URL 构造（`?` 恒附体 · quote 语义）/
// 行渲染（query · trace 缩进）/ stats 表头与空档 / HTTP 5xx 档 / 令牌文件读不了档 /
// 未识别参数档。**成对负控**两枚：① `--self-test` 必须仍被当**未识别参数**（证没给 CLI 加过
// 新旗标 ⇒ 接口同形）；② 无参 `query` 的 URL 仍带尾 `?`（证没把 py 的引号/拼接口径放宽）。
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func flgTRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := fleetLogsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// flgTServe 起一个合成主控：按路径路由回据，并把收到的 request URI 记进 seen。
func flgTServe(t *testing.T, routes map[string]string) (*httptest.Server, *[]string) {
	t.Helper()
	seen := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.URL.RequestURI())
		body, ok := routes[r.URL.Path]
		if !ok {
			http.Error(w, "no route", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, seen
}

func TestFleetLogsHelpTop(t *testing.T) {
	rc, out, errs := flgTRun(t, "-h")
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d stderr=%q（要 0/空）", rc, errs)
	}
	head := "usage: fleet_logs.py [-h] [--controller CONTROLLER] [--token-file TOKEN_FILE]\n" +
		"                     {query,trace,errors,stats} ...\n"
	if !strings.HasPrefix(out, head) {
		t.Fatalf("帮助首两行不逐字：%q", out)
	}
	for _, must := range []string{"Fleet 日志查询工具（调主控 :8580）", "options:", "  --controller CONTROLLER", "~/.zerg/token"} {
		if !strings.Contains(out, must) {
			t.Errorf("帮助缺 %q", must)
		}
	}
}

func TestFleetLogsHelpSubcommands(t *testing.T) {
	cases := map[string]string{
		"query":  "usage: fleet_logs.py query [-h] [--machine MACHINE] [--component COMPONENT]\n",
		"trace":  "usage: fleet_logs.py trace [-h] request_id\n",
		"errors": "usage: fleet_logs.py errors [-h] [--since SINCE]\n",
		"stats":  "usage: fleet_logs.py stats [-h] --model MODEL\n",
	}
	for sub, want := range cases {
		rc, out, errs := flgTRun(t, sub, "-h")
		if rc != 0 || errs != "" {
			t.Fatalf("%s -h: rc=%d stderr=%q", sub, rc, errs)
		}
		if !strings.HasPrefix(out, want) {
			t.Errorf("%s -h 用法首行不逐字：%q", sub, strings.SplitN(out, "\n", 2)[0])
		}
	}
}

func TestFleetLogsUsageErrors(t *testing.T) {
	top := "usage: fleet_logs.py [-h] [--controller CONTROLLER] [--token-file TOKEN_FILE]\n" +
		"                     {query,trace,errors,stats} ...\n"
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"no_subcmd", nil, top + "fleet_logs.py: error: the following arguments are required: command\n"},
		{"bad_subcmd", []string{"frobnicate"}, top + "fleet_logs.py: error: argument command: invalid choice: 'frobnicate' (choose from 'query', 'trace', 'errors', 'stats')\n"},
		{"unrecognized", []string{"query", "--nope", "1"}, top + "fleet_logs.py: error: unrecognized arguments: --nope 1\n"},
		{"extra_positionals", []string{"query", "a", "b"}, top + "fleet_logs.py: error: unrecognized arguments: a b\n"},
		{"missing_controller_value", []string{"--controller"}, top + "fleet_logs.py: error: argument --controller: expected one argument\n"},
		{"missing_model", []string{"stats"}, "usage: fleet_logs.py stats [-h] --model MODEL\nfleet_logs.py stats: error: the following arguments are required: --model\n"},
		{"trace_noarg", []string{"trace"}, "usage: fleet_logs.py trace [-h] request_id\nfleet_logs.py trace: error: the following arguments are required: request_id\n"},
		{"flag_no_value", []string{"query", "--m"}, "usage: fleet_logs.py query [-h] [--machine MACHINE] [--component COMPONENT]\n                           [--since SINCE] [--level LEVEL]\nfleet_logs.py query: error: argument --machine: expected one argument\n"},
	}
	for _, c := range cases {
		rc, out, errs := flgTRun(t, c.argv...)
		if rc != 2 {
			t.Errorf("%s: rc=%d（要 2）", c.name, rc)
		}
		if out != "" {
			t.Errorf("%s: stdout 应为空，得 %q", c.name, out)
		}
		if errs != c.want {
			t.Errorf("%s: stderr 不逐字\n got=%q\nwant=%q", c.name, errs, c.want)
		}
	}
}

// 负控①：CLI 面上**不许**多出 `--self-test` 一类新旗标（接口同形）。
func TestFleetLogsNoExtraSelfTestFlag(t *testing.T) {
	rc, out, errs := flgTRun(t, "query", "--self-test")
	top := "usage: fleet_logs.py [-h] [--controller CONTROLLER] [--token-file TOKEN_FILE]\n" +
		"                     {query,trace,errors,stats} ...\n"
	if rc != 2 || out != "" || errs != top+"fleet_logs.py: error: unrecognized arguments: --self-test\n" {
		t.Fatalf("`--self-test` 必须仍是未识别参数：rc=%d out=%q err=%q", rc, out, errs)
	}
}

func TestFleetLogsQueryParamsAndQuote(t *testing.T) {
	srv, seen := flgTServe(t, map[string]string{
		"/api/fleet/logs/query": `[{"ts":"T1","level":"ERROR","machine":"edge-node","component":"agent","model":"M","request_id":"r1","message":"m"}]`,
	})
	rc, out, errs := flgTRun(t, "--controller", srv.URL, "query", "--machine", "x 3&y=z/7", "--since", "1h")
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d stderr=%q", rc, errs)
	}
	if got := (*seen)[0]; got != "/api/fleet/logs/query?machine=x%203%26y%3Dz/7&since=1h" {
		t.Errorf("URL 不逐字：%q", got)
	}
	want := "T1 [ERROR] [x3/agent] model=M request_id=r1 m\n"
	if out != want {
		t.Errorf("行渲染不逐字：got=%q want=%q", out, want)
	}
}

// 负控②：无参 query 的 URL 仍带尾 `?`（py `if params:` 恒真且 join 为空串）。
func TestFleetLogsQueryAlwaysAppendsQuestion(t *testing.T) {
	srv, seen := flgTServe(t, map[string]string{"/api/fleet/logs/query": `[]`})
	rc, out, errs := flgTRun(t, "--controller", srv.URL, "query")
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d stderr=%q", rc, errs)
	}
	if got := (*seen)[0]; got != "/api/fleet/logs/query?" {
		t.Errorf("尾 `?` 丢失：%q", got)
	}
	if out != "（无匹配日志）\n" {
		t.Errorf("空集渲染不逐字：%q", out)
	}
}

func TestFleetLogsTraceChainIndent(t *testing.T) {
	srv, _ := flgTServe(t, map[string]string{
		"/api/fleet/logs/trace": `[{"ts":"T","level":"INFO","machine":"m","component":"c","model":null,"request_id":null,"message":"x"}]`,
	})
	rc, out, errs := flgTRun(t, "--controller", srv.URL, "trace", "abc-123")
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d stderr=%q", rc, errs)
	}
	want := "  T [INFO] [m/c] model=- request_id=- x\n"
	if out != want {
		t.Errorf("trace 缩进/空值渲染不逐字：got=%q want=%q", out, want)
	}
}

func TestFleetLogsStatsShape(t *testing.T) {
	srv, _ := flgTServe(t, map[string]string{
		"/api/fleet/logs/stats": `{"INFO": 5, "ERROR": 2, "TRACE": 9}`,
	})
	rc, out, errs := flgTRun(t, "--controller", srv.URL, "stats", "--model", "GLM-5.2")
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d stderr=%q", rc, errs)
	}
	// 只出 DEBUG/INFO/WARN/ERROR 四词里在场的，次序固定；TRACE 不列。
	want := "模型 GLM-5.2 日志统计:\n  INFO: 5\n  ERROR: 2\n"
	if out != want {
		t.Errorf("stats 渲染不逐字：got=%q want=%q", out, want)
	}
}

func TestFleetLogsStatsEmpty(t *testing.T) {
	srv, _ := flgTServe(t, map[string]string{"/api/fleet/logs/stats": `{}`})
	rc, out, _ := flgTRun(t, "--controller", srv.URL, "stats", "--model", "Nope")
	if rc != 0 || out != "模型 Nope 日志统计:\n  （无日志）\n" {
		t.Errorf("空 stats 渲染不逐字：rc=%d out=%q", rc, out)
	}
}

func TestFleetLogsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom\n"))
	}))
	t.Cleanup(srv.Close)
	rc, out, errs := flgTRun(t, "--controller", srv.URL, "query")
	if rc != 1 || out != "" || errs != "API 错误 500: boom\n\n" {
		t.Fatalf("5xx 档不逐字：rc=%d out=%q err=%q", rc, out, errs)
	}
}

func TestFleetLogsTokenFileMissing(t *testing.T) {
	rc, out, errs := flgTRun(t, "--token-file", "/no/such/flg-token", "query")
	if rc != 2 || out != "" {
		t.Fatalf("rc=%d out=%q（要 2/空）", rc, out)
	}
	want := "读令牌文件失败：/no/such/flg-token: [Errno 2] No such file or directory: '/no/such/flg-token'\n"
	if errs != want {
		t.Errorf("令牌文件错不逐字：got=%q want=%q", errs, want)
	}
}

func TestFleetLogsTokenFileRead(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tok")
	if err := os.WriteFile(p, []byte("tok-xyz\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv, _ := flgTServe(t, map[string]string{"/api/fleet/logs/errors": `[]`})
	rc, _, errs := flgTRun(t, "--token-file", p, "--controller", srv.URL, "errors")
	if rc != 0 || errs != "" {
		t.Fatalf("读得到令牌却失败：rc=%d err=%q", rc, errs)
	}
}

// errors 子命令的 `--since` 默认 1h（py set_defaults）。
func TestFleetLogsErrorsDefaultSince(t *testing.T) {
	srv, seen := flgTServe(t, map[string]string{"/api/fleet/logs/errors": `[]`})
	rc, _, errs := flgTRun(t, "--controller", srv.URL, "errors")
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if got := (*seen)[0]; got != "/api/fleet/logs/errors?since=1h" {
		t.Errorf("errors 默认 since 不逐字：%q", got)
	}
}
